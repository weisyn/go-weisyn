// forward_service.go - 区块转发服务
// 🆕 MEDIUM-001 修复：优化区块转发机制
package controller

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	consensusconfig "github.com/weisyn/v1/internal/config/consensus"
	"github.com/weisyn/v1/pkg/constants/protocols"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	netiface "github.com/weisyn/v1/pkg/interfaces/network"
)

// ForwardService 区块转发服务
// 负责管理区块转发的重试、超时和健康分
type ForwardService struct {
	logger         log.Logger
	networkService netiface.Network
	routingManager kademlia.RoutingTableManager
	config         consensusconfig.BlockForwardConfig

	// 动态超时管理
	currentTimeout time.Duration
	timeoutMu      sync.RWMutex

	// 转发统计
	totalForwards     uint64
	successForwards   uint64
	failedForwards    uint64
	timeoutForwards   uint64
	retryForwards     uint64

	// 备用节点缓存
	backupNodes   map[uint64][]peer.ID // height -> backup nodes
	backupNodesMu sync.RWMutex
}

// NewForwardService 创建转发服务
func NewForwardService(
	logger log.Logger,
	networkService netiface.Network,
	routingManager kademlia.RoutingTableManager,
	config consensusconfig.BlockForwardConfig,
) *ForwardService {
	// 设置默认值
	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}
	if config.RetryBackoffBase <= 0 {
		config.RetryBackoffBase = 500 * time.Millisecond
	}
	if config.RetryBackoffMax <= 0 {
		config.RetryBackoffMax = 10 * time.Second
	}
	if config.RetryBackoffFactor <= 0 {
		config.RetryBackoffFactor = 2.0
	}
	if config.CallTimeout <= 0 {
		config.CallTimeout = 15 * time.Second
	}
	if config.MinTimeout <= 0 {
		config.MinTimeout = 5 * time.Second
	}
	if config.MaxTimeout <= 0 {
		config.MaxTimeout = 30 * time.Second
	}
	if config.BackupNodeCount <= 0 {
		config.BackupNodeCount = 2
	}

	return &ForwardService{
		logger:         logger,
		networkService: networkService,
		routingManager: routingManager,
		config:         config,
		currentTimeout: config.CallTimeout,
		backupNodes:    make(map[uint64][]peer.ID),
	}
}

// ForwardResult 转发结果
type ForwardResult struct {
	Success     bool
	Attempts    int
	Duration    time.Duration
	Error       error
	UsedBackup  bool
	FinalTarget peer.ID
}

// ForwardWithRetry 带重试的区块转发
func (fs *ForwardService) ForwardWithRetry(
	ctx context.Context,
	target peer.ID,
	height uint64,
	data []byte,
) (*ForwardResult, error) {
	atomic.AddUint64(&fs.totalForwards, 1)

	startTime := time.Now()
	result := &ForwardResult{
		FinalTarget: target,
	}

	// 获取备用节点列表
	backupNodes := fs.getBackupNodes(height, target)

	// 所有候选节点（主节点 + 备用节点）
	candidates := append([]peer.ID{target}, backupNodes...)

	// 重试退避
	backoff := newForwardBackoff(fs.config.RetryBackoffBase, fs.config.RetryBackoffMax, fs.config.RetryBackoffFactor)

	var lastErr error

	for attempt := 0; attempt < fs.config.MaxRetries; attempt++ {
		for i, candidate := range candidates {
			result.Attempts++

			// 获取当前超时时间
			timeout := fs.getCurrentTimeout()

			// 创建带超时的上下文
			callCtx, cancel := context.WithTimeout(ctx, timeout)

			// 执行网络调用
			_, err := fs.networkService.Call(callCtx, candidate, protocols.ProtocolBlockSubmission, data, nil)
			cancel()

			if err == nil {
				// 转发成功
				result.Success = true
				result.FinalTarget = candidate
				result.UsedBackup = i > 0
				result.Duration = time.Since(startTime)

				atomic.AddUint64(&fs.successForwards, 1)

				// 记录成功到健康系统
				if fs.routingManager != nil {
					fs.routingManager.RecordPeerSuccess(candidate)
				}

				// 动态调整超时（成功时减少）
				if fs.config.EnableDynamicTimeout {
					fs.adjustTimeout(true)
				}

				if fs.logger != nil {
					fs.logger.Infof("✅ 区块转发成功: target=%s, height=%d, attempts=%d, used_backup=%v, duration=%s",
						candidate.String()[:12], height, result.Attempts, result.UsedBackup, result.Duration)
				}

				return result, nil
			}

			// 记录错误
			lastErr = err

			// 检查是否为超时错误
			if errors.Is(err, context.DeadlineExceeded) {
				atomic.AddUint64(&fs.timeoutForwards, 1)
				if fs.config.EnableDynamicTimeout {
					fs.adjustTimeout(false)
				}
			}

			// 记录失败到健康系统
			if fs.routingManager != nil {
				fs.routingManager.RecordPeerFailure(candidate)
			}

			if fs.logger != nil {
				fs.logger.Warnf("⚠️ 区块转发失败: target=%s, height=%d, attempt=%d, error=%v",
					candidate.String()[:12], height, result.Attempts, err)
			}

			// 检查上下文是否已取消
			select {
			case <-ctx.Done():
				result.Error = ctx.Err()
				result.Duration = time.Since(startTime)
				atomic.AddUint64(&fs.failedForwards, 1)
				return result, ctx.Err()
			default:
			}
		}

		// 所有候选都失败，等待退避时间后重试
		if attempt < fs.config.MaxRetries-1 {
			atomic.AddUint64(&fs.retryForwards, 1)
			backoffDuration := backoff.Next()

			if fs.logger != nil {
				fs.logger.Infof("🔄 区块转发重试: height=%d, attempt=%d/%d, backoff=%s",
					height, attempt+2, fs.config.MaxRetries, backoffDuration)
			}

			select {
			case <-ctx.Done():
				result.Error = ctx.Err()
				result.Duration = time.Since(startTime)
				atomic.AddUint64(&fs.failedForwards, 1)
				return result, ctx.Err()
			case <-time.After(backoffDuration):
			}
		}
	}

	// 所有重试都失败
	result.Error = lastErr
	result.Duration = time.Since(startTime)
	atomic.AddUint64(&fs.failedForwards, 1)

	if fs.logger != nil {
		fs.logger.Errorf("🚫 区块转发最终失败: height=%d, attempts=%d, duration=%s, error=%v",
			height, result.Attempts, result.Duration, lastErr)
	}

	return result, lastErr
}

// getBackupNodes 获取备用节点
func (fs *ForwardService) getBackupNodes(height uint64, excludeTarget peer.ID) []peer.ID {
	if !fs.config.EnableBackupNodes {
		return nil
	}

	fs.backupNodesMu.RLock()
	cached, ok := fs.backupNodes[height]
	fs.backupNodesMu.RUnlock()

	if ok && len(cached) > 0 {
		// 过滤掉主目标
		filtered := make([]peer.ID, 0, len(cached))
		for _, p := range cached {
			if p != excludeTarget {
				filtered = append(filtered, p)
			}
		}
		return filtered
	}

	// 从路由表获取备用节点
	if fs.routingManager == nil {
		return nil
	}

	// 获取最近的节点作为备用（使用 target 的字节表示）
	targetBytes := []byte(excludeTarget)
	closestPeers := fs.routingManager.FindClosestPeers(targetBytes, fs.config.BackupNodeCount+1)

	backups := make([]peer.ID, 0, fs.config.BackupNodeCount)
	for _, p := range closestPeers {
		if p != excludeTarget && len(backups) < fs.config.BackupNodeCount {
			backups = append(backups, p)
		}
	}

	// 缓存备用节点
	fs.backupNodesMu.Lock()
	fs.backupNodes[height] = backups
	fs.backupNodesMu.Unlock()

	return backups
}

// ClearBackupCache 清理备用节点缓存
func (fs *ForwardService) ClearBackupCache(height uint64) {
	fs.backupNodesMu.Lock()
	delete(fs.backupNodes, height)
	fs.backupNodesMu.Unlock()
}

// ClearAllBackupCache 清理所有备用节点缓存
func (fs *ForwardService) ClearAllBackupCache() {
	fs.backupNodesMu.Lock()
	fs.backupNodes = make(map[uint64][]peer.ID)
	fs.backupNodesMu.Unlock()
}

// getCurrentTimeout 获取当前超时时间
func (fs *ForwardService) getCurrentTimeout() time.Duration {
	fs.timeoutMu.RLock()
	defer fs.timeoutMu.RUnlock()
	return fs.currentTimeout
}

// adjustTimeout 动态调整超时时间
func (fs *ForwardService) adjustTimeout(success bool) {
	fs.timeoutMu.Lock()
	defer fs.timeoutMu.Unlock()

	if success {
		// 成功时减少超时时间（更激进）
		newTimeout := time.Duration(float64(fs.currentTimeout) * 0.95)
		if newTimeout < fs.config.MinTimeout {
			newTimeout = fs.config.MinTimeout
		}
		fs.currentTimeout = newTimeout
	} else {
		// 失败时增加超时时间
		newTimeout := time.Duration(float64(fs.currentTimeout) * 1.2)
		if newTimeout > fs.config.MaxTimeout {
			newTimeout = fs.config.MaxTimeout
		}
		fs.currentTimeout = newTimeout
	}
}

// GetStats 获取转发统计
func (fs *ForwardService) GetStats() ForwardStats {
	return ForwardStats{
		TotalForwards:   atomic.LoadUint64(&fs.totalForwards),
		SuccessForwards: atomic.LoadUint64(&fs.successForwards),
		FailedForwards:  atomic.LoadUint64(&fs.failedForwards),
		TimeoutForwards: atomic.LoadUint64(&fs.timeoutForwards),
		RetryForwards:   atomic.LoadUint64(&fs.retryForwards),
		CurrentTimeout:  fs.getCurrentTimeout(),
	}
}

// ForwardStats 转发统计信息
type ForwardStats struct {
	TotalForwards   uint64
	SuccessForwards uint64
	FailedForwards  uint64
	TimeoutForwards uint64
	RetryForwards   uint64
	CurrentTimeout  time.Duration
}

// forwardBackoff 转发退避策略
type forwardBackoff struct {
	base    time.Duration
	max     time.Duration
	factor  float64
	current time.Duration
	mu      sync.Mutex
}

// newForwardBackoff 创建转发退避
func newForwardBackoff(base, max time.Duration, factor float64) *forwardBackoff {
	return &forwardBackoff{
		base:    base,
		max:     max,
		factor:  factor,
		current: base,
	}
}

// Next 获取下一个退避时间
func (fb *forwardBackoff) Next() time.Duration {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	current := fb.current
	fb.current = time.Duration(float64(fb.current) * fb.factor)
	if fb.current > fb.max {
		fb.current = fb.max
	}
	return current
}

// Reset 重置退避
func (fb *forwardBackoff) Reset() {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	fb.current = fb.base
}

// HealthScoreRecoveryDaemon 健康分恢复守护进程
// 注：健康分恢复逻辑已整合到 Kademlia 的维护协程中
// 此类型保留用于未来扩展，或可通过路由表事件实现更细粒度的恢复策略
type HealthScoreRecoveryDaemon struct {
	routingManager kademlia.RoutingTableManager
	logger         log.Logger
	config         consensusconfig.BlockForwardConfig

	// 成功的 peer 记录（用于渐进恢复）
	successfulPeers   map[peer.ID]time.Time
	successfulPeersMu sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewHealthScoreRecoveryDaemon 创建健康分恢复守护进程
func NewHealthScoreRecoveryDaemon(
	routingManager kademlia.RoutingTableManager,
	logger log.Logger,
	config consensusconfig.BlockForwardConfig,
) *HealthScoreRecoveryDaemon {
	ctx, cancel := context.WithCancel(context.Background())
	return &HealthScoreRecoveryDaemon{
		routingManager:  routingManager,
		logger:          logger,
		config:          config,
		successfulPeers: make(map[peer.ID]time.Time),
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Start 启动健康分恢复守护进程
func (d *HealthScoreRecoveryDaemon) Start() {
	if d.config.RecoveryInterval <= 0 {
		return
	}

	d.wg.Add(1)
	go d.recoveryLoop()

	if d.logger != nil {
		d.logger.Info("🏥 健康分恢复守护进程已启动")
	}
}

// Stop 停止健康分恢复守护进程
func (d *HealthScoreRecoveryDaemon) Stop() {
	d.cancel()
	d.wg.Wait()

	if d.logger != nil {
		d.logger.Info("🏥 健康分恢复守护进程已停止")
	}
}

// RecordSuccess 记录成功的 peer（供外部调用）
func (d *HealthScoreRecoveryDaemon) RecordSuccess(peerID peer.ID) {
	d.successfulPeersMu.Lock()
	d.successfulPeers[peerID] = time.Now()
	d.successfulPeersMu.Unlock()

	// 同时通知路由管理器
	if d.routingManager != nil {
		d.routingManager.RecordPeerSuccess(peerID)
	}
}

// recoveryLoop 健康分恢复循环
func (d *HealthScoreRecoveryDaemon) recoveryLoop() {
	defer d.wg.Done()

	ticker := time.NewTicker(d.config.RecoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.performRecovery()
		}
	}
}

// performRecovery 执行健康分恢复
// 基于最近成功记录的 peer 列表进行渐进恢复
func (d *HealthScoreRecoveryDaemon) performRecovery() {
	if d.routingManager == nil {
		return
	}

	d.successfulPeersMu.Lock()
	defer d.successfulPeersMu.Unlock()

	recoveredCount := 0
	now := time.Now()
	expireDuration := d.config.RecoveryInterval * 3 // 保留3个周期的记录

	for peerID, lastSuccess := range d.successfulPeers {
		// 清理过期记录
		if now.Sub(lastSuccess) > expireDuration {
			delete(d.successfulPeers, peerID)
			continue
		}

		// 对最近成功的 peer 记录一次成功（渐进恢复健康分）
		d.routingManager.RecordPeerSuccess(peerID)
		recoveredCount++
	}

	if recoveredCount > 0 && d.logger != nil {
		d.logger.Debugf("🏥 健康分恢复: recovered=%d peers", recoveredCount)
	}
}

