// collect_candidates.go
// 候选收集和窗口管理核心（固定收集窗口策略）
//
// 🎯 **固定收集窗口设计理念**：
//
// **核心目标**：给足够时间收集候选区块进行选择，确保聚合器能收到各矿工的候选
//
// **设计原则**：
// 1. **固定时间窗口**：从接收第一个候选区块开始，启动固定时长的收集窗口
// 2. **被动收集模式**：聚合器被动等待候选区块提交，不主动拉取
// 3. **窗口结束即选择**：收集窗口结束后立即进行选择，不等待更多候选
// 4. **时间确定性**：窗口时长固定，给矿工明确的提交时间预期
//
// **与矿工难度调整的配合**：
// - 矿工侧：通过难度系数控制出块速度，让矿工有足够时间收集更多交易
// - 聚合器侧：通过固定收集窗口，给足够时间让各矿工的候选区块到达
// - 分离关注点：矿工专注交易收集，聚合器专注候选收集
//
// **时间戳完整性保护**：
// - 绝不基于区块时间戳调整收集窗口或等待时间
// - 区块时间戳必须反映真实创建时间
// - 收集窗口基于聚合器接收时间，与区块时间戳无关
//
// 主要功能：
// 1. 固定时间收集窗口的启动、管理和停止
// 2. 被动接收其他节点提交的候选区块
// 3. 去重检测避免重复存储
// 4. 存储有效候选到候选池
// 5. 追踪收集进度和统计
//
// 作者：WES开发团队
// 创建时间：2025-09-13

package candidate_collector

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	"github.com/weisyn/v1/pkg/types"
)

// collectionWindow 收集窗口状态
type collectionWindow struct {
	height              uint64                           // 目标高度
	startTime           time.Time                        // 窗口启动时间
	duration            time.Duration                    // 窗口持续时间
	endTime             time.Time                        // 窗口结束时间
	isActive            bool                             // 窗口是否活跃
	candidatesCollected int                              // 已收集候选数量
	candidatesValidated int                              // 已验证候选数量
	candidatesRejected  int                              // 已拒绝候选数量
	duplicatesDetected  int                              // 检测到的重复数量
	collectedCandidates map[string]*types.CandidateBlock // 收集的候选区块（以哈希为key）
	receiveDelays       []time.Duration                  // 接收延迟记录
	mutex               sync.RWMutex                     // 读写锁
	cancelFunc          context.CancelFunc               // 取消函数
}

// collectionManager 收集管理器
type collectionManager struct {
	logger        log.Logger
	candidatePool mempool.CandidatePool
	activeWindows map[uint64]*collectionWindow // 活跃的收集窗口
	windowMutex   sync.RWMutex                 // 窗口操作锁
	validator     *candidateValidator          // 候选验证器
}

// newCollectionManager 创建收集管理器
func newCollectionManager(
	logger log.Logger,
	candidatePool mempool.CandidatePool,
	validator *candidateValidator,
) *collectionManager {
	return &collectionManager{
		logger:        logger,
		candidatePool: candidatePool,
		activeWindows: make(map[uint64]*collectionWindow),
		validator:     validator,
	}
}

// startCollectionWindow 启动收集窗口
func (m *collectionManager) startCollectionWindow(height uint64, duration time.Duration) error {
	m.windowMutex.Lock()
	defer m.windowMutex.Unlock()

	// 检查是否已存在该高度的窗口
	if _, exists := m.activeWindows[height]; exists {
		return errors.New("collection window already exists for height")
	}

	// 创建窗口上下文和取消函数
	ctx, cancelFunc := context.WithCancel(context.Background())

	// 创建新的收集窗口
	now := time.Now()
	window := &collectionWindow{
		height:              height,
		startTime:           now,
		duration:            duration,
		endTime:             now.Add(duration),
		isActive:            true,
		collectedCandidates: make(map[string]*types.CandidateBlock),
		receiveDelays:       make([]time.Duration, 0),
		cancelFunc:          cancelFunc,
	}

	m.activeWindows[height] = window

	// 启动窗口超时处理
	go m.handleWindowTimeout(ctx, height, duration)

	m.logger.Info("启动候选收集窗口")
	return nil
}

// closeCollectionWindow 关闭收集窗口
func (m *collectionManager) closeCollectionWindow(height uint64) ([]types.CandidateBlock, error) {
	m.windowMutex.Lock()
	defer m.windowMutex.Unlock()

	window, exists := m.activeWindows[height]
	if !exists {
		return nil, errors.New("collection window not found")
	}

	window.mutex.Lock()
	defer window.mutex.Unlock()

	// 标记窗口为非活跃状态
	window.isActive = false
	if window.cancelFunc != nil {
		window.cancelFunc()
	}

	// 提取收集到的候选区块
	candidates := make([]types.CandidateBlock, 0, len(window.collectedCandidates))
	for _, candidate := range window.collectedCandidates {
		candidates = append(candidates, *candidate)
	}

	// 从活跃窗口列表中移除
	delete(m.activeWindows, height)

	m.logger.Info("关闭候选收集窗口")
	return candidates, nil
}

// isCollectionActive 检查收集窗口是否活跃
func (m *collectionManager) isCollectionActive(height uint64) bool {
	m.windowMutex.RLock()
	defer m.windowMutex.RUnlock()

	window, exists := m.activeWindows[height]
	if !exists {
		return false
	}

	window.mutex.RLock()
	defer window.mutex.RUnlock()
	return window.isActive && time.Now().Before(window.endTime)
}

// getCollectionProgress 获取收集进度
func (m *collectionManager) getCollectionProgress(height uint64) (*types.CollectionProgress, error) {
	m.windowMutex.RLock()
	defer m.windowMutex.RUnlock()

	window, exists := m.activeWindows[height]
	if !exists {
		return nil, errors.New("collection window not found")
	}

	window.mutex.RLock()
	defer window.mutex.RUnlock()

	// 计算平均接收延迟
	var avgDelay time.Duration
	if len(window.receiveDelays) > 0 {
		totalDelay := time.Duration(0)
		for _, delay := range window.receiveDelays {
			totalDelay += delay
		}
		avgDelay = totalDelay / time.Duration(len(window.receiveDelays))
	}

	// 计算进度百分比
	elapsed := time.Since(window.startTime)
	progress := float64(elapsed) / float64(window.duration)
	if progress > 1.0 {
		progress = 1.0
	}

	return &types.CollectionProgress{
		Height:              window.height,
		WindowStartTime:     window.startTime,
		WindowDuration:      window.duration,
		WindowEndTime:       window.endTime,
		IsActive:            window.isActive && time.Now().Before(window.endTime),
		CandidatesCollected: window.candidatesCollected,
		CandidatesValidated: window.candidatesValidated,
		CandidatesRejected:  window.candidatesRejected,
		DuplicatesDetected:  window.duplicatesDetected,
		AverageReceiveDelay: avgDelay,
		ProgressPercentage:  progress,
	}, nil
}

// handleWindowTimeout 处理窗口超时
func (m *collectionManager) handleWindowTimeout(ctx context.Context, height uint64, duration time.Duration) {
	select {
	case <-time.After(duration):
		m.logger.Info("收集窗口超时，自动关闭")
		_, err := m.closeCollectionWindow(height)
		if err != nil {
			m.logger.Info("自动关闭窗口失败")
		}
	case <-ctx.Done():
		// 窗口被手动关闭
		return
	}
}

// collectCandidateFromMempool 从候选池收集指定高度的候选区块
func (m *collectionManager) collectCandidateFromMempool(height uint64) error {
	m.windowMutex.RLock()
	window, exists := m.activeWindows[height]
	m.windowMutex.RUnlock()

	if !exists {
		return errors.New("no active collection window")
	}

	// 进一步加锁保护窗口状态，避免与 closeCollectionWindow 并发修改产生数据竞争
	window.mutex.RLock()
	isActive := window.isActive && time.Now().Before(window.endTime)
	window.mutex.RUnlock()

	if !isActive {
		return errors.New("no active collection window")
	}

	// 从候选池获取指定高度的候选区块
	candidates, err := m.candidatePool.GetCandidatesForHeight(height, 100*time.Millisecond)
	if err != nil {
		return err
	}

	// 处理每个候选区块
	for i := range candidates {
		candidate := candidates[i]
		if err := m.processCandidateBlock(height, window, candidate); err != nil {
			var hashBytes []byte
			if candidate != nil {
				hashBytes = candidate.BlockHash
			}
			m.logger.Warnf("处理候选区块失败: height=%d, hash=%x, err=%v", height, hashBytes, err)
			continue
		}
	}

	return nil
}

// processCandidateBlock 处理单个候选区块
//
// 注意：window 由调用方在持有 windowMutex 的前提下安全获取，避免在此函数中直接访问 activeWindows map。
func (m *collectionManager) processCandidateBlock(height uint64, window *collectionWindow, candidate *types.CandidateBlock) error {
	window.mutex.Lock()
	defer window.mutex.Unlock()

	// 检查窗口是否仍然活跃
	if !window.isActive || time.Now().After(window.endTime) {
		return errors.New("collection window expired")
	}

	// 生成候选区块的唯一标识
	blockKey := string(candidate.BlockHash)

	// 检查是否已收集过该候选区块
	if _, exists := window.collectedCandidates[blockKey]; exists {
		window.duplicatesDetected++
		return errors.New("duplicate candidate block")
	}

	// 验证候选区块
	if err := m.validator.validateCandidate(candidate); err != nil {
		window.candidatesRejected++
		return err
	}

	// 记录接收延迟
	receiveDelay := time.Since(candidate.ProducedAt)
	window.receiveDelays = append(window.receiveDelays, receiveDelay)

	// 将候选区块添加到收集窗口
	window.collectedCandidates[blockKey] = candidate
	window.candidatesCollected++
	window.candidatesValidated++

	return nil
}
