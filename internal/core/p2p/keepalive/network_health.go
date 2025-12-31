// network_health.go - 网络健康检查服务
// 🆕 HIGH-003 修复：提供全面的网络健康监控和自动修复功能
package keepalive

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	libnetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	p2pcfg "github.com/weisyn/v1/internal/config/p2p"
	"github.com/weisyn/v1/pkg/constants/events"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// NetworkHealthStatus 网络健康状态
type NetworkHealthStatus string

const (
	NetworkHealthStatusHealthy   NetworkHealthStatus = "healthy"
	NetworkHealthStatusDegraded  NetworkHealthStatus = "degraded"
	NetworkHealthStatusUnhealthy NetworkHealthStatus = "unhealthy"
)

// NetworkHealthStats 网络健康统计
type NetworkHealthStats struct {
	Status             NetworkHealthStatus
	TotalConnections   int
	ActiveConnections  int
	IdleConnections    int
	TotalTimeouts      uint64
	RecentTimeouts     uint64 // 最近一个周期内的超时数
	TimeoutRatio       float64
	AvgLatencyMs       float64
	LastCheckAt        time.Time
	ConsecutiveFailures int
	ConsecutiveSuccesses int
}

// NetworkHealthChecker 网络健康检查器
type NetworkHealthChecker struct {
	host     host.Host
	logger   log.Logger
	eventBus event.EventBus
	config   p2pcfg.NetworkHealthConfig

	// 状态
	stats     NetworkHealthStats
	statsMu   sync.RWMutex
	
	// 超时计数器
	totalTimeouts  uint64
	periodTimeouts uint64

	// 动态超时管理
	currentTimeout   time.Duration
	timeoutConfig    p2pcfg.NetworkTimeoutConfig
	timeoutMu        sync.RWMutex

	// 修复状态
	healingAttempts  int
	lastHealingAt    time.Time
	healingMu        sync.Mutex

	// 运行控制
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	running   bool
	runningMu sync.RWMutex
}

// NewNetworkHealthChecker 创建网络健康检查器
func NewNetworkHealthChecker(
	host host.Host,
	logger log.Logger,
	eventBus event.EventBus,
	healthConfig p2pcfg.NetworkHealthConfig,
	timeoutConfig p2pcfg.NetworkTimeoutConfig,
) *NetworkHealthChecker {
	ctx, cancel := context.WithCancel(context.Background())

	// 设置默认值
	if healthConfig.CheckInterval <= 0 {
		healthConfig.CheckInterval = 30 * time.Second
	}
	if healthConfig.UnhealthyThreshold <= 0 {
		healthConfig.UnhealthyThreshold = 3
	}
	if healthConfig.HealthyThreshold <= 0 {
		healthConfig.HealthyThreshold = 2
	}
	if healthConfig.TimeoutRatioThreshold <= 0 {
		healthConfig.TimeoutRatioThreshold = 0.3
	}
	if healthConfig.HealingCooldown <= 0 {
		healthConfig.HealingCooldown = time.Minute
	}
	if healthConfig.MaxHealingAttempts <= 0 {
		healthConfig.MaxHealingAttempts = 5
	}

	return &NetworkHealthChecker{
		host:           host,
		logger:         logger,
		eventBus:       eventBus,
		config:         healthConfig,
		timeoutConfig:  timeoutConfig,
		currentTimeout: timeoutConfig.DialTimeout,
		stats: NetworkHealthStats{
			Status: NetworkHealthStatusHealthy,
		},
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start 启动健康检查
func (nhc *NetworkHealthChecker) Start() error {
	nhc.runningMu.Lock()
	defer nhc.runningMu.Unlock()

	if nhc.running {
		return nil
	}

	nhc.running = true

	nhc.wg.Add(1)
	go nhc.checkLoop()

	if nhc.logger != nil {
		nhc.logger.Info("🏥 网络健康检查器已启动")
	}

	return nil
}

// Stop 停止健康检查
func (nhc *NetworkHealthChecker) Stop() {
	nhc.runningMu.Lock()
	if !nhc.running {
		nhc.runningMu.Unlock()
		return
	}
	nhc.running = false
	nhc.runningMu.Unlock()

	nhc.cancel()
	nhc.wg.Wait()

	if nhc.logger != nil {
		nhc.logger.Info("🏥 网络健康检查器已停止")
	}
}

// checkLoop 健康检查循环
func (nhc *NetworkHealthChecker) checkLoop() {
	defer nhc.wg.Done()

	ticker := time.NewTicker(nhc.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-nhc.ctx.Done():
			return
		case <-ticker.C:
			nhc.performHealthCheck()
		}
	}
}

// performHealthCheck 执行健康检查
func (nhc *NetworkHealthChecker) performHealthCheck() {
	nhc.statsMu.Lock()
	defer nhc.statsMu.Unlock()

	// 收集连接统计
	network := nhc.host.Network()
	conns := network.Conns()
	totalConns := len(conns)

	activeConns := 0
	idleConns := 0
	var totalLatency time.Duration
	latencyCount := 0

	for _, conn := range conns {
		stat := conn.Stat()
		if stat.NumStreams > 0 {
			activeConns++
		} else {
			idleConns++
		}
		// 简单的延迟估算（使用连接建立时间）
		if !stat.Opened.IsZero() {
			latencyCount++
			totalLatency += time.Since(stat.Opened)
		}
	}

	// 获取超时统计
	periodTimeouts := atomic.SwapUint64(&nhc.periodTimeouts, 0)
	totalTimeouts := atomic.LoadUint64(&nhc.totalTimeouts)

	// 计算超时比例
	var timeoutRatio float64
	if totalConns > 0 {
		timeoutRatio = float64(periodTimeouts) / float64(totalConns+int(periodTimeouts))
	}

	// 计算平均延迟
	var avgLatencyMs float64
	if latencyCount > 0 {
		avgLatencyMs = float64(totalLatency.Milliseconds()) / float64(latencyCount)
	}

	// 更新统计
	nhc.stats.TotalConnections = totalConns
	nhc.stats.ActiveConnections = activeConns
	nhc.stats.IdleConnections = idleConns
	nhc.stats.TotalTimeouts = totalTimeouts
	nhc.stats.RecentTimeouts = periodTimeouts
	nhc.stats.TimeoutRatio = timeoutRatio
	nhc.stats.AvgLatencyMs = avgLatencyMs
	nhc.stats.LastCheckAt = time.Now()

	// 判断健康状态
	oldStatus := nhc.stats.Status
	if timeoutRatio >= nhc.config.TimeoutRatioThreshold {
		nhc.stats.ConsecutiveFailures++
		nhc.stats.ConsecutiveSuccesses = 0
		if nhc.stats.ConsecutiveFailures >= nhc.config.UnhealthyThreshold {
			nhc.stats.Status = NetworkHealthStatusUnhealthy
		} else {
			nhc.stats.Status = NetworkHealthStatusDegraded
		}
	} else if totalConns < 3 {
		nhc.stats.Status = NetworkHealthStatusDegraded
		nhc.stats.ConsecutiveFailures++
		nhc.stats.ConsecutiveSuccesses = 0
	} else {
		nhc.stats.ConsecutiveSuccesses++
		nhc.stats.ConsecutiveFailures = 0
		if nhc.stats.ConsecutiveSuccesses >= nhc.config.HealthyThreshold {
			nhc.stats.Status = NetworkHealthStatusHealthy
		}
	}

	// 状态变化时记录日志
	if oldStatus != nhc.stats.Status {
		if nhc.logger != nil {
			nhc.logger.Infof("🏥 网络健康状态变化: %s -> %s (conns=%d, timeout_ratio=%.2f%%)",
				oldStatus, nhc.stats.Status, totalConns, timeoutRatio*100)
		}

		// 发布状态变化事件
		if nhc.eventBus != nil {
			nhc.publishHealthEvent()
		}
	}

	// 触发自动修复
	if nhc.config.EnableAutoHealing && nhc.stats.Status == NetworkHealthStatusUnhealthy {
		nhc.tryAutoHealing()
	}

	// 动态调整超时
	if nhc.timeoutConfig.EnableDynamicTimeout {
		nhc.adjustDynamicTimeout(timeoutRatio)
	}

	if nhc.logger != nil {
		nhc.logger.Debugf("🏥 健康检查完成: status=%s conns=%d active=%d idle=%d timeouts=%d ratio=%.2f%%",
			nhc.stats.Status, totalConns, activeConns, idleConns, periodTimeouts, timeoutRatio*100)
	}
}

// adjustDynamicTimeout 动态调整超时时间
func (nhc *NetworkHealthChecker) adjustDynamicTimeout(timeoutRatio float64) {
	nhc.timeoutMu.Lock()
	defer nhc.timeoutMu.Unlock()

	oldTimeout := nhc.currentTimeout

	if timeoutRatio >= nhc.config.TimeoutRatioThreshold {
		// 超时比例高，增加超时时间
		newTimeout := time.Duration(float64(nhc.currentTimeout) * nhc.timeoutConfig.TimeoutIncreaseFactor)
		if newTimeout > nhc.timeoutConfig.MaxTimeout {
			newTimeout = nhc.timeoutConfig.MaxTimeout
		}
		nhc.currentTimeout = newTimeout
	} else if timeoutRatio < nhc.config.TimeoutRatioThreshold/2 {
		// 超时比例低，减少超时时间
		newTimeout := time.Duration(float64(nhc.currentTimeout) * nhc.timeoutConfig.TimeoutDecreaseFactor)
		if newTimeout < nhc.timeoutConfig.MinTimeout {
			newTimeout = nhc.timeoutConfig.MinTimeout
		}
		nhc.currentTimeout = newTimeout
	}

	if oldTimeout != nhc.currentTimeout && nhc.logger != nil {
		nhc.logger.Infof("🕐 动态超时调整: %s -> %s (ratio=%.2f%%)",
			oldTimeout, nhc.currentTimeout, timeoutRatio*100)
	}
}

// tryAutoHealing 尝试自动修复
func (nhc *NetworkHealthChecker) tryAutoHealing() {
	nhc.healingMu.Lock()
	defer nhc.healingMu.Unlock()

	// 检查冷却时间
	if time.Since(nhc.lastHealingAt) < nhc.config.HealingCooldown {
		return
	}

	// 检查最大尝试次数
	if nhc.healingAttempts >= nhc.config.MaxHealingAttempts {
		if nhc.logger != nil {
			nhc.logger.Warnf("🏥 自动修复已达最大尝试次数: %d", nhc.config.MaxHealingAttempts)
		}
		return
	}

	nhc.healingAttempts++
	nhc.lastHealingAt = time.Now()

	if nhc.logger != nil {
		nhc.logger.Infof("🏥 开始自动修复网络 (尝试 %d/%d)",
			nhc.healingAttempts, nhc.config.MaxHealingAttempts)
	}

	// 触发发现加速
	if nhc.eventBus != nil {
		resetData := &types.DiscoveryResetEventData{
			Reason:    "network_unhealthy",
			Trigger:   "network_health_checker",
			Timestamp: time.Now().Unix(),
		}
		nhc.eventBus.Publish(events.EventTypeDiscoveryIntervalReset, resetData)
	}

	// 清理空闲连接
	nhc.cleanupIdleConnections()
}

// cleanupIdleConnections 清理空闲连接
func (nhc *NetworkHealthChecker) cleanupIdleConnections() {
	if !nhc.config.ConnectionCheckEnabled {
		return
	}

	network := nhc.host.Network()
	conns := network.Conns()

	idleCount := 0
	closedCount := 0

	for _, conn := range conns {
		stat := conn.Stat()
		// 检查是否空闲且超时
		if stat.NumStreams == 0 {
			idleDuration := time.Since(stat.Opened)
			if idleDuration > nhc.config.IdleConnectionTimeout {
				if err := conn.Close(); err == nil {
					closedCount++
				}
			} else {
				idleCount++
			}
		}
	}

	if closedCount > 0 && nhc.logger != nil {
		nhc.logger.Infof("🧹 清理空闲连接: closed=%d remaining_idle=%d", closedCount, idleCount)
	}
}

// publishHealthEvent 发布健康事件
func (nhc *NetworkHealthChecker) publishHealthEvent() {
	if nhc.eventBus == nil {
		return
	}

	// 可以定义一个新的事件类型，这里暂时使用日志记录
	if nhc.logger != nil {
		nhc.logger.Infof("📢 网络健康事件: status=%s conns=%d timeouts=%d ratio=%.2f%%",
			nhc.stats.Status, nhc.stats.TotalConnections,
			nhc.stats.RecentTimeouts, nhc.stats.TimeoutRatio*100)
	}
}

// RecordTimeout 记录超时事件
func (nhc *NetworkHealthChecker) RecordTimeout() {
	atomic.AddUint64(&nhc.totalTimeouts, 1)
	atomic.AddUint64(&nhc.periodTimeouts, 1)
}

// GetCurrentTimeout 获取当前动态超时时间
func (nhc *NetworkHealthChecker) GetCurrentTimeout() time.Duration {
	nhc.timeoutMu.RLock()
	defer nhc.timeoutMu.RUnlock()
	return nhc.currentTimeout
}

// GetStats 获取健康统计
func (nhc *NetworkHealthChecker) GetStats() NetworkHealthStats {
	nhc.statsMu.RLock()
	defer nhc.statsMu.RUnlock()
	return nhc.stats
}

// IsHealthy 检查网络是否健康
func (nhc *NetworkHealthChecker) IsHealthy() bool {
	nhc.statsMu.RLock()
	defer nhc.statsMu.RUnlock()
	return nhc.stats.Status == NetworkHealthStatusHealthy
}

// ResetHealingAttempts 重置修复尝试计数
func (nhc *NetworkHealthChecker) ResetHealingAttempts() {
	nhc.healingMu.Lock()
	defer nhc.healingMu.Unlock()
	nhc.healingAttempts = 0
}

// ConnectionHealthChecker 连接健康检查器（用于单个连接）
type ConnectionHealthChecker struct {
	timeout    time.Duration
	maxRetries int
	backoff    *RetryBackoff
}

// RetryBackoff 重试退避策略
type RetryBackoff struct {
	base    time.Duration
	max     time.Duration
	factor  float64
	current time.Duration
	mu      sync.Mutex
}

// NewRetryBackoff 创建重试退避
func NewRetryBackoff(base, max time.Duration, factor float64) *RetryBackoff {
	return &RetryBackoff{
		base:    base,
		max:     max,
		factor:  factor,
		current: base,
	}
}

// Next 获取下一个退避时间
func (rb *RetryBackoff) Next() time.Duration {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	current := rb.current
	rb.current = time.Duration(float64(rb.current) * rb.factor)
	if rb.current > rb.max {
		rb.current = rb.max
	}
	return current
}

// Reset 重置退避
func (rb *RetryBackoff) Reset() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.current = rb.base
}

// NewConnectionHealthChecker 创建连接健康检查器
func NewConnectionHealthChecker(config p2pcfg.NetworkTimeoutConfig) *ConnectionHealthChecker {
	return &ConnectionHealthChecker{
		timeout:    config.DialTimeout,
		maxRetries: config.MaxRetries,
		backoff:    NewRetryBackoff(config.RetryBackoffBase, config.RetryBackoffMax, config.RetryBackoffFactor),
	}
}

// CheckConnection 检查连接健康状态
func (chc *ConnectionHealthChecker) CheckConnection(ctx context.Context, host host.Host, peerID peer.ID) error {
	// 检查连接状态
	connectedness := host.Network().Connectedness(peerID)
	if connectedness == libnetwork.Connected {
		return nil
	}

	// 尝试重连
	chc.backoff.Reset()
	var lastErr error

	for i := 0; i < chc.maxRetries; i++ {
		// 使用动态超时
		dialCtx, cancel := context.WithTimeout(ctx, chc.timeout)

		addrs := host.Peerstore().Addrs(peerID)
		if len(addrs) > 0 {
			addrInfo := peer.AddrInfo{ID: peerID, Addrs: addrs}
			lastErr = host.Connect(dialCtx, addrInfo)
			cancel()

			if lastErr == nil {
				return nil
			}
		} else {
			cancel()
			lastErr = ErrNoAddresses
		}

		// 等待退避时间
		backoffDuration := chc.backoff.Next()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoffDuration):
		}
	}

	return lastErr
}

// ErrNoAddresses 无地址错误
var ErrNoAddresses = &NoAddressesError{}

// NoAddressesError 无地址错误类型
type NoAddressesError struct{}

func (e *NoAddressesError) Error() string {
	return "no addresses available for peer"
}

