// 文件说明：
// 本文件实现候选区块池的若干辅助查询/维护方法，包括：
// - 获取候选哈希、按哈希获取候选；
// - 清空候选池、清理过期候选；
// - 等待与通知机制等。
package candidatepool

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/weisyn/v1/pkg/types"
)

// GetCandidateHashes 获取当前所有候选区块的哈希列表。
// 参数：无。
// 返回：
// - [][]byte：候选区块哈希切片；
// - error：恒为 nil（当前实现）。
func (p *CandidatePool) GetCandidateHashes() ([][]byte, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	hashes := make([][]byte, 0, len(p.candidates))
	for _, candidate := range p.candidates {
		hashes = append(hashes, candidate.BlockHash)
	}

	return hashes, nil
}

// GetCandidateByHash 通过哈希获取候选区块。
// 参数：
// - blockHash：候选区块哈希。
// 返回：
// - *types.CandidateBlock：候选区块；
// - error：未找到时返回 ErrCandidateNotFound。
func (p *CandidatePool) GetCandidateByHash(blockHash []byte) (*types.CandidateBlock, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	blockHashStr := hex.EncodeToString(blockHash)
	candidate, exists := p.candidates[blockHashStr]
	if !exists {
		return nil, ErrCandidateNotFound
	}

	return candidate, nil
}

// 注意：GetPoolStatus() 和 GetCandidateStatus() 方法已从公共接口中删除
// 根据"自运行系统"设计原则，不再暴露内部状态监控接口

// ClearCandidates 清空候选区块池。
// 参数：无。
// 返回：
// - int：清空的候选数量；
// - error：清理过程中的错误（当前实现恒为 nil）。
func (p *CandidatePool) ClearCandidates() (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	count := len(p.candidates)

	// 清空所有存储
	p.candidates = make(map[string]*types.CandidateBlock)
	p.candidatesByHeight = make(map[uint64][]*types.CandidateBlock)
	p.pendingCandidates = make(map[string]struct{})
	p.verifiedCandidates = make(map[string]struct{})
	p.expiredCandidates = make(map[string]struct{})

	// 重置内存使用量
	p.memoryUsage = 0

	// 更新统计
	p.totalRemoved += uint64(count)

	// 发布事件
	p.eventSink.OnPoolCleared(count)

	if p.logger != nil {
		p.logger.Infof("清空候选区块池，数量: %d", count)
	}

	return count, nil
}

// ClearExpiredCandidates 清理超时的候选区块。
// 参数：
// - maxAge：最大存活时间。
// 返回：
// - int：被移除的候选数量；
// - error：当前恒为 nil。
func (p *CandidatePool) ClearExpiredCandidates(maxAge time.Duration) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.cleanExpiredCandidatesInternal(), nil
}

// cleanExpiredCandidatesInternal 内部清理过期候选区块（调用方需持锁）。
// 参数：无。
// 返回：
// - int：被移除的候选数量。
func (p *CandidatePool) cleanExpiredCandidatesInternal() int {
	removed := 0
	
	// 1. 基于时间的清理
	removed += p.cleanExpiredByAgeInternal()
	
	// 2. 基于高度的清理（如果启用）
	if p.config.HeightCleanupEnabled {
		removed += p.cleanExpiredByHeightInternal()
	}
	
	return removed
}

// cleanExpiredByAgeInternal 基于时间清理过期候选区块（调用方需持锁）
func (p *CandidatePool) cleanExpiredByAgeInternal() int {
	now := time.Now()
	maxAge := p.config.MaxAge

	var toRemove []string
	var removedSize uint64

	for hashStr, candidate := range p.candidates {
		if now.Sub(candidate.ReceivedAt) > maxAge {
			toRemove = append(toRemove, hashStr)
			removedSize += uint64(candidate.EstimatedSize)

			// 标记为过期
			candidate.Expired = true
			p.expiredCandidates[hashStr] = struct{}{}
		}
	}

	// 执行移除
	removedCount := p.removeCandidatesInternal(toRemove, removedSize, "age_expired")
	
	if p.logger != nil && removedCount > 0 {
		p.logger.Infof("基于时间清理过期候选区块: %d个", removedCount)
	}

	return removedCount
}

// cleanExpiredByHeightInternal 基于高度清理过期候选区块（调用方需持锁）
func (p *CandidatePool) cleanExpiredByHeightInternal() int {
	// 获取当前链高度
	if p.chainStateCache == nil {
		if p.logger != nil {
			p.logger.Debug("链状态缓存未设置，跳过基于高度的清理")
		}
		return 0
	}

	// 使用 Background context，因为这是后台清理协程，不需要取消信号
	ctx := context.Background()
	currentHeight, err := p.chainStateCache.GetCurrentHeight(ctx)
	if err != nil {
		if p.logger != nil {
			p.logger.Warnf("获取当前链高度失败，跳过基于高度的清理: %v", err)
		}
		return 0
	}

	// 计算清理阈值高度：当前高度 - 保留深度
	var cleanupThreshold uint64
	if currentHeight > p.config.KeepHeightDepth {
		cleanupThreshold = currentHeight - p.config.KeepHeightDepth
	} else {
		// 如果当前高度小于等于保留深度，不进行清理
		return 0
	}

	var toRemove []string
	var removedSize uint64

	// 查找需要清理的候选区块
	for hashStr, candidate := range p.candidates {
		if candidate.Height < cleanupThreshold {
			toRemove = append(toRemove, hashStr)
			removedSize += uint64(candidate.EstimatedSize)

			// 标记为过期
			candidate.Expired = true
			p.expiredCandidates[hashStr] = struct{}{}
		}
	}

	// 执行移除
	removedCount := p.removeCandidatesInternal(toRemove, removedSize, "height_expired")
	
	if p.logger != nil && removedCount > 0 {
		p.logger.Infof("基于高度清理过时候选区块: %d个 (清理阈值高度: %d, 当前高度: %d)", 
			removedCount, cleanupThreshold, currentHeight)
	}

	return removedCount
}

// removeCandidatesInternal 内部批量移除候选区块（调用方需持锁）
func (p *CandidatePool) removeCandidatesInternal(toRemove []string, removedSize uint64, reason string) int {
	// 执行移除
	for _, hashStr := range toRemove {
		candidate := p.candidates[hashStr]

		// 从主存储中移除
		delete(p.candidates, hashStr)

		// 从高度索引中移除
		height := candidate.Height
		if heightCandidates := p.candidatesByHeight[height]; len(heightCandidates) > 0 {
			var filtered []*types.CandidateBlock
			for _, c := range heightCandidates {
				if hex.EncodeToString(c.BlockHash) != hashStr {
					filtered = append(filtered, c)
				}
			}
			if len(filtered) == 0 {
				delete(p.candidatesByHeight, height)
			} else {
				p.candidatesByHeight[height] = filtered
			}
		}

		// 从状态映射中移除
		delete(p.pendingCandidates, hashStr)
		delete(p.verifiedCandidates, hashStr)

		// 发布事件
		if reason == "age_expired" {
			p.eventSink.OnCandidateExpired(candidate)
		} else {
			p.eventSink.OnCandidateRemoved(candidate, reason)
		}
	}

	// 更新统计
	p.memoryUsage -= removedSize
	p.totalRemoved += uint64(len(toRemove))

	return len(toRemove)
}

// RemoveCandidate 按哈希移除候选区块。
// 参数：
// - blockHash：候选区块哈希。
// 返回：
// - error：未找到时返回 ErrCandidateNotFound。
func (p *CandidatePool) RemoveCandidate(blockHash []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	blockHashStr := hex.EncodeToString(blockHash)
	candidate, exists := p.candidates[blockHashStr]
	if !exists {
		return ErrCandidateNotFound
	}

	// 从主存储中移除
	delete(p.candidates, blockHashStr)

	// 从高度索引中移除
	height := candidate.Height
	if heightCandidates := p.candidatesByHeight[height]; len(heightCandidates) > 0 {
		var filtered []*types.CandidateBlock
		for _, c := range heightCandidates {
			if hex.EncodeToString(c.BlockHash) != blockHashStr {
				filtered = append(filtered, c)
			}
		}
		if len(filtered) == 0 {
			delete(p.candidatesByHeight, height)
		} else {
			p.candidatesByHeight[height] = filtered
		}
	}

	// 从状态映射中移除
	delete(p.pendingCandidates, blockHashStr)
	delete(p.verifiedCandidates, blockHashStr)
	delete(p.expiredCandidates, blockHashStr)

	// 更新内存使用量
	p.memoryUsage -= uint64(candidate.EstimatedSize)

	// 更新统计
	p.totalRemoved++

	// 发布事件
	p.eventSink.OnCandidateRemoved(candidate, "manual_removal")

	if p.logger != nil {
		p.logger.Infof("移除候选区块，高度: %d, 哈希: %x", height, blockHash[:8])
	}

	return nil
}

// waitForCandidatesAtHeight 等待指定高度的候选区块达到或超时。
// 参数：
// - height：目标高度；
// - timeout：等待时长。
// 返回：
// - []*types.CandidateBlock：符合条件的候选列表；
// - error：超时返回 ErrTimeout，池关闭返回 ErrPoolClosed。
func (p *CandidatePool) waitForCandidatesAtHeight(height uint64, timeout time.Duration) ([]*types.CandidateBlock, error) {
	waitCh := make(chan []*types.CandidateBlock, 1)

	p.mu.Lock()
	waitKey := fmt.Sprintf("height_%d_%d", height, time.Now().UnixNano())
	p.waitChannels[waitKey] = waitCh
	p.mu.Unlock()

	// 清理等待通道
	defer func() {
		p.mu.Lock()
		delete(p.waitChannels, waitKey)
		p.mu.Unlock()
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case candidates := <-waitCh:
		return candidates, nil
	case <-timer.C:
		return nil, ErrTimeout
	case <-p.quit:
		return nil, ErrPoolClosed
	}
}

// notifyWaiters 通知等待中的协程（按高度与数量两种机制）。
// 修复：添加nil通道检查和关闭通道的panic保护
// 参数：
// - height：新到来候选区块的高度。
// 返回：无。
func (p *CandidatePool) notifyWaiters(height uint64) {
	// 通知按高度等待的协程
	heightKey := fmt.Sprintf("height_%d", height)
	for key, ch := range p.waitChannels {
		// 修复：检查通道是否为nil
		if ch == nil {
			continue
		}
		
		if len(key) > len(heightKey) && key[:len(heightKey)] == heightKey {
			candidates := p.candidatesByHeight[height]
			// 修复：使用recover保护，防止向关闭的通道发送数据导致panic
			func() {
				defer func() {
					if r := recover(); r != nil {
						// 通道已关闭，忽略错误
						if p.logger != nil {
							p.logger.Debugf("通知等待者时通道已关闭: %s", key)
						}
					}
				}()
				select {
				case ch <- candidates:
				default:
					// 通道已满，忽略
				}
			}()
		}
	}

	// 通知按数量等待的协程
	countPrefix := "count_"
	totalCount := len(p.candidates)
	for key, ch := range p.waitChannels {
		// 修复：检查通道是否为nil
		if ch == nil {
			continue
		}
		
		if len(key) > len(countPrefix) && key[:len(countPrefix)] == countPrefix {
			// 解析最小数量要求
			var minCount int
			if _, err := fmt.Sscanf(key, "count_%d_", &minCount); err == nil {
				if totalCount >= minCount {
					allCandidates := make([]*types.CandidateBlock, 0, len(p.candidates))
					for _, candidate := range p.candidates {
						allCandidates = append(allCandidates, candidate)
					}
					// 修复：使用recover保护，防止向关闭的通道发送数据导致panic
					func() {
						defer func() {
							if r := recover(); r != nil {
								// 通道已关闭，忽略错误
								if p.logger != nil {
									p.logger.Debugf("通知等待者时通道已关闭: %s", key)
								}
							}
						}()
						select {
						case ch <- allCandidates:
						default:
							// 通道已满，忽略
						}
					}()
				}
			}
		}
	}
}

// clearOutdatedCandidates 清理过时高度的候选区块（已移除高度维度清理，保留时间过期清理）
func (p *CandidatePool) clearOutdatedCandidates() (int, error) {
	// 使用新的基于高度的清理机制
	if p.config.HeightCleanupEnabled {
		return p.cleanExpiredByHeightInternal(), nil
	}
	return 0, nil
}

// ClearOutdatedCandidates 公共接口：清理过时高度的候选区块
func (p *CandidatePool) ClearOutdatedCandidates() (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.clearOutdatedCandidates()
}

// cleanAggressively 激进清理方法，在池满时使用（调用方需持锁）
func (p *CandidatePool) cleanAggressively() int {
	if !p.config.AggressiveCleanup {
		return 0
	}

	removed := 0
	
	// 1. 先进行标准清理
	removed += p.cleanExpiredCandidatesInternal()
	
	// 2. 如果还是满的，按优先级清理最旧的候选区块
	maxCandidates := p.config.MaxCandidates
	if len(p.candidates) >= maxCandidates {
		// 计算需要清理多少个候选区块（清理25%）
		targetRemoval := len(p.candidates) / 4
		if targetRemoval == 0 {
			targetRemoval = 1 // 至少清理1个
		}

		// 收集候选区块并按时间排序（最旧的优先清理）
		type candidateInfo struct {
			hash       string
			candidate  *types.CandidateBlock
			receivedAt time.Time
		}

		var candidates []candidateInfo
		for hashStr, candidate := range p.candidates {
			candidates = append(candidates, candidateInfo{
				hash:       hashStr,
				candidate:  candidate,
				receivedAt: candidate.ReceivedAt,
			})
		}

		// 按接收时间排序（最旧的在前）
		for i := 0; i < len(candidates)-1; i++ {
			for j := i + 1; j < len(candidates); j++ {
				if candidates[i].receivedAt.After(candidates[j].receivedAt) {
					candidates[i], candidates[j] = candidates[j], candidates[i]
				}
			}
		}

		// 清理最旧的候选区块
		var toRemove []string
		var removedSize uint64
		for i := 0; i < targetRemoval && i < len(candidates); i++ {
			toRemove = append(toRemove, candidates[i].hash)
			removedSize += uint64(candidates[i].candidate.EstimatedSize)
			candidates[i].candidate.Expired = true
			p.expiredCandidates[candidates[i].hash] = struct{}{}
		}

		aggressiveRemoved := p.removeCandidatesInternal(toRemove, removedSize, "aggressive_cleanup")
		removed += aggressiveRemoved
		
		if p.logger != nil && aggressiveRemoved > 0 {
			p.logger.Infof("激进清理最旧候选区块: %d个", aggressiveRemoved)
		}
	}

	return removed
}
