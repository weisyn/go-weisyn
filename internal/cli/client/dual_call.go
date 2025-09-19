// Package client 提供CLI客户端的双重调用模式实现
package client

import (
	"context"
	"fmt"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// CallMode 调用模式枚举
type CallMode int

const (
	// DirectCall 直接调用核心服务
	DirectCall CallMode = iota
	// APICall 通过API调用
	APICall
	// AutoCall 自动选择最优调用模式
	AutoCall
)

// String 返回调用模式的字符串表示
func (cm CallMode) String() string {
	switch cm {
	case DirectCall:
		return "DirectCall"
	case APICall:
		return "APICall"
	case AutoCall:
		return "AutoCall"
	default:
		return "Unknown"
	}
}

// CallStrategy 调用策略配置
type CallStrategy struct {
	// 默认调用模式
	DefaultMode CallMode

	// 超时配置
	DirectCallTimeout time.Duration
	APICallTimeout    time.Duration

	// 重试配置
	MaxRetries int
	RetryDelay time.Duration

	// 性能阈值
	PerformanceThreshold time.Duration

	// 降级策略
	FallbackEnabled bool
	FallbackMode    CallMode
}

// CallResult 调用结果
type CallResult struct {
	Data     interface{}   // 调用结果数据
	Mode     CallMode      // 实际使用的调用模式
	Duration time.Duration // 调用耗时
	Success  bool          // 调用是否成功
	Error    error         // 错误信息
	Retries  int           // 重试次数
}

// CallContext 调用上下文
type CallContext struct {
	Operation    string                 // 操作名称
	Parameters   map[string]interface{} // 调用参数
	RequireAuth  bool                   // 是否需要认证
	CacheEnabled bool                   // 是否启用缓存
	UserLevel    bool                   // 是否为用户级操作
}

// DualCallClient 双重调用客户端接口
type DualCallClient interface {
	// 配置管理
	SetStrategy(strategy CallStrategy)
	GetStrategy() CallStrategy

	// 调用执行
	Call(ctx context.Context, callCtx CallContext) (*CallResult, error)
	CallWithMode(ctx context.Context, callCtx CallContext, mode CallMode) (*CallResult, error)

	// 性能监控
	GetCallStats() *CallStats
	ResetStats()

	// 健康检查
	CheckDirectCallHealth(ctx context.Context) error
	CheckAPICallHealth(ctx context.Context) error
}

// CallStats 调用统计信息
type CallStats struct {
	TotalCalls        int64         // 总调用次数
	DirectCalls       int64         // 直接调用次数
	APICalls          int64         // API调用次数
	SuccessfulCalls   int64         // 成功调用次数
	FailedCalls       int64         // 失败调用次数
	AverageLatency    time.Duration // 平均延迟
	DirectCallLatency time.Duration // 直接调用平均延迟
	APICallLatency    time.Duration // API调用平均延迟
	LastUpdateTime    time.Time     // 最后更新时间
}

// DirectCallExecutor 直接调用执行器接口
type DirectCallExecutor interface {
	Execute(ctx context.Context, operation string, params map[string]interface{}) (interface{}, error)
	IsAvailable() bool
	GetLatency() time.Duration
}

// APICallExecutor API调用执行器接口
type APICallExecutor interface {
	Execute(ctx context.Context, operation string, params map[string]interface{}) (interface{}, error)
	IsAvailable() bool
	GetLatency() time.Duration
}

// dualCallClient 双重调用客户端实现
type dualCallClient struct {
	logger     log.Logger
	strategy   CallStrategy
	directExec DirectCallExecutor
	apiExec    APICallExecutor
	stats      *CallStats

	// 性能监控
	recentCalls    []*CallResult
	maxRecentCalls int
}

// NewDualCallClient 创建双重调用客户端
func NewDualCallClient(
	logger log.Logger,
	directExec DirectCallExecutor,
	apiExec APICallExecutor,
) DualCallClient {
	return &dualCallClient{
		logger:         logger,
		directExec:     directExec,
		apiExec:        apiExec,
		stats:          &CallStats{},
		strategy:       getDefaultStrategy(),
		recentCalls:    make([]*CallResult, 0),
		maxRecentCalls: 100,
	}
}

// SetStrategy 设置调用策略
func (dc *dualCallClient) SetStrategy(strategy CallStrategy) {
	dc.strategy = strategy
	dc.logger.Info(fmt.Sprintf("调用策略已更新: mode=%s", strategy.DefaultMode.String()))
}

// GetStrategy 获取当前调用策略
func (dc *dualCallClient) GetStrategy() CallStrategy {
	return dc.strategy
}

// Call 执行调用（自动选择模式）
func (dc *dualCallClient) Call(ctx context.Context, callCtx CallContext) (*CallResult, error) {
	// 根据策略选择调用模式
	mode := dc.selectCallMode(callCtx)
	return dc.CallWithMode(ctx, callCtx, mode)
}

// CallWithMode 使用指定模式执行调用
func (dc *dualCallClient) CallWithMode(ctx context.Context, callCtx CallContext, mode CallMode) (*CallResult, error) {
	startTime := time.Now()

	dc.logger.Info(fmt.Sprintf("开始执行调用: operation=%s, mode=%s", callCtx.Operation, mode.String()))

	var result *CallResult
	var err error

	// 执行调用
	var attempt int
	for attempt = 0; attempt <= dc.strategy.MaxRetries; attempt++ {
		if attempt > 0 {
			// 等待重试延迟
			time.Sleep(dc.strategy.RetryDelay)
			dc.logger.Info(fmt.Sprintf("重试调用: attempt=%d", attempt))
		}

		result, err = dc.executeCall(ctx, callCtx, mode)
		if err == nil && result.Success {
			break // 成功，不需要重试
		}

		// 如果启用降级且主模式失败，尝试降级模式
		if dc.strategy.FallbackEnabled && mode != dc.strategy.FallbackMode && attempt == 0 {
			dc.logger.Info(fmt.Sprintf("主模式失败，尝试降级模式: from=%s, to=%s",
				mode.String(), dc.strategy.FallbackMode.String()))
			mode = dc.strategy.FallbackMode
		}
	}

	// 更新统计信息
	result.Duration = time.Since(startTime)
	if result != nil {
		result.Retries = attempt
	}

	dc.updateStats(result)
	dc.addRecentCall(result)

	if err != nil {
		dc.logger.Error(fmt.Sprintf("调用执行失败: operation=%s, error=%v", callCtx.Operation, err))
		return result, err
	}

	dc.logger.Info(fmt.Sprintf("调用执行完成: operation=%s, mode=%s, duration=%v",
		callCtx.Operation, result.Mode.String(), result.Duration))

	return result, nil
}

// executeCall 执行单次调用
func (dc *dualCallClient) executeCall(ctx context.Context, callCtx CallContext, mode CallMode) (*CallResult, error) {
	result := &CallResult{
		Mode:    mode,
		Success: false,
	}

	var executor interface {
		Execute(context.Context, string, map[string]interface{}) (interface{}, error)
	}
	var timeout time.Duration

	// 选择执行器和超时时间
	switch mode {
	case DirectCall:
		if !dc.directExec.IsAvailable() {
			return result, fmt.Errorf("直接调用执行器不可用")
		}
		executor = dc.directExec
		timeout = dc.strategy.DirectCallTimeout

	case APICall:
		if !dc.apiExec.IsAvailable() {
			return result, fmt.Errorf("API调用执行器不可用")
		}
		executor = dc.apiExec
		timeout = dc.strategy.APICallTimeout

	default:
		return result, fmt.Errorf("不支持的调用模式: %s", mode.String())
	}

	// 创建带超时的上下文
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// 执行调用
	data, err := executor.Execute(ctx, callCtx.Operation, callCtx.Parameters)
	if err != nil {
		result.Error = err
		return result, err
	}

	result.Data = data
	result.Success = true
	return result, nil
}

// selectCallMode 选择调用模式
func (dc *dualCallClient) selectCallMode(callCtx CallContext) CallMode {
	// 如果指定了默认模式且不是自动模式，直接使用
	if dc.strategy.DefaultMode != AutoCall {
		return dc.strategy.DefaultMode
	}

	// 自动选择逻辑

	// 1. 检查认证要求
	if callCtx.RequireAuth && callCtx.UserLevel {
		// 用户级操作优先使用直接调用（性能更好）
		if dc.directExec.IsAvailable() {
			return DirectCall
		}
		return APICall
	}

	// 2. 系统级操作，基于性能选择
	directLatency := dc.directExec.GetLatency()
	apiLatency := dc.apiExec.GetLatency()

	// 如果直接调用延迟明显更低，选择直接调用
	if dc.directExec.IsAvailable() && directLatency > 0 &&
		(apiLatency <= 0 || directLatency < apiLatency/2) {
		return DirectCall
	}

	// 3. 基于近期调用性能选择
	if len(dc.recentCalls) > 10 {
		directSuccessRate := dc.calculateSuccessRate(DirectCall)
		apiSuccessRate := dc.calculateSuccessRate(APICall)

		// 如果直接调用成功率更高，优先选择
		if directSuccessRate > apiSuccessRate+0.1 { // 10%的优势
			return DirectCall
		}
	}

	// 4. 默认选择API调用（更稳定）
	if dc.apiExec.IsAvailable() {
		return APICall
	}

	// 5. 最后选择直接调用
	return DirectCall
}

// calculateSuccessRate 计算指定模式的成功率
func (dc *dualCallClient) calculateSuccessRate(mode CallMode) float64 {
	totalCalls := 0
	successCalls := 0

	for _, call := range dc.recentCalls {
		if call.Mode == mode {
			totalCalls++
			if call.Success {
				successCalls++
			}
		}
	}

	if totalCalls == 0 {
		return 0.0
	}

	return float64(successCalls) / float64(totalCalls)
}

// updateStats 更新统计信息
func (dc *dualCallClient) updateStats(result *CallResult) {
	if result == nil {
		return
	}

	dc.stats.TotalCalls++
	dc.stats.LastUpdateTime = time.Now()

	if result.Success {
		dc.stats.SuccessfulCalls++
	} else {
		dc.stats.FailedCalls++
	}

	switch result.Mode {
	case DirectCall:
		dc.stats.DirectCalls++
		if result.Duration > 0 {
			// 简化的平均延迟计算
			if dc.stats.DirectCallLatency == 0 {
				dc.stats.DirectCallLatency = result.Duration
			} else {
				dc.stats.DirectCallLatency = (dc.stats.DirectCallLatency + result.Duration) / 2
			}
		}

	case APICall:
		dc.stats.APICalls++
		if result.Duration > 0 {
			if dc.stats.APICallLatency == 0 {
				dc.stats.APICallLatency = result.Duration
			} else {
				dc.stats.APICallLatency = (dc.stats.APICallLatency + result.Duration) / 2
			}
		}
	}

	// 更新总体平均延迟
	if result.Duration > 0 {
		if dc.stats.AverageLatency == 0 {
			dc.stats.AverageLatency = result.Duration
		} else {
			dc.stats.AverageLatency = (dc.stats.AverageLatency + result.Duration) / 2
		}
	}
}

// addRecentCall 添加到最近调用记录
func (dc *dualCallClient) addRecentCall(result *CallResult) {
	if result == nil {
		return
	}

	dc.recentCalls = append(dc.recentCalls, result)

	// 保持最大记录数限制
	if len(dc.recentCalls) > dc.maxRecentCalls {
		dc.recentCalls = dc.recentCalls[1:]
	}
}

// GetCallStats 获取调用统计信息
func (dc *dualCallClient) GetCallStats() *CallStats {
	// 返回统计信息的副本
	stats := *dc.stats
	return &stats
}

// ResetStats 重置统计信息
func (dc *dualCallClient) ResetStats() {
	dc.stats = &CallStats{}
	dc.recentCalls = make([]*CallResult, 0)
	dc.logger.Info("调用统计信息已重置")
}

// CheckDirectCallHealth 检查直接调用健康状态
func (dc *dualCallClient) CheckDirectCallHealth(ctx context.Context) error {
	if !dc.directExec.IsAvailable() {
		return fmt.Errorf("直接调用执行器不可用")
	}

	// 执行简单的健康检查调用
	_, err := dc.directExec.Execute(ctx, "health_check", map[string]interface{}{})
	return err
}

// CheckAPICallHealth 检查API调用健康状态
func (dc *dualCallClient) CheckAPICallHealth(ctx context.Context) error {
	if !dc.apiExec.IsAvailable() {
		return fmt.Errorf("API调用执行器不可用")
	}

	// 执行简单的健康检查调用
	_, err := dc.apiExec.Execute(ctx, "health_check", map[string]interface{}{})
	return err
}

// getDefaultStrategy 获取默认调用策略
func getDefaultStrategy() CallStrategy {
	return CallStrategy{
		DefaultMode:          AutoCall,
		DirectCallTimeout:    30 * time.Second,
		APICallTimeout:       60 * time.Second,
		MaxRetries:           2,
		RetryDelay:           1 * time.Second,
		PerformanceThreshold: 5 * time.Second,
		FallbackEnabled:      true,
		FallbackMode:         APICall,
	}
}
