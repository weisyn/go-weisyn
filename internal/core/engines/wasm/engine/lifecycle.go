package engine

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// LifecycleState 生命周期状态
type LifecycleState int

const (
	StateUninitialized LifecycleState = iota
	StateInitializing
	StateRunning
	StateStopping
	StateStopped
	StateError
)

// String 返回状态字符串表示
func (s LifecycleState) String() string {
	switch s {
	case StateUninitialized:
		return "uninitialized"
	case StateInitializing:
		return "initializing"
	case StateRunning:
		return "running"
	case StateStopping:
		return "stopping"
	case StateStopped:
		return "stopped"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// LifecycleManager WASM引擎生命周期管理器
type LifecycleManager struct {
	// 状态管理
	state      LifecycleState
	stateMutex sync.RWMutex

	// 配置
	config *LifecycleConfig

	// 组件引用
	vm *VM

	// 生命周期钩子
	startHooks []StartHook
	stopHooks  []StopHook

	// 健康检查
	healthCheckers  []HealthChecker
	lastHealthCheck time.Time
	healthStatus    *HealthStatus

	// 取消上下文
	ctx    context.Context
	cancel context.CancelFunc

	// 等待组
	wg sync.WaitGroup

	// 启动时间
	startTime time.Time

	// 统计信息
	stats *LifecycleStats
}

// LifecycleConfig 生命周期配置
type LifecycleConfig struct {
	// 启动超时
	StartTimeout time.Duration `json:"startTimeout"`

	// 停止超时
	StopTimeout time.Duration `json:"stopTimeout"`

	// 健康检查间隔
	HealthCheckInterval time.Duration `json:"healthCheckInterval"`

	// 是否启用优雅关闭
	GracefulShutdown bool `json:"gracefulShutdown"`

	// 最大启动重试次数
	MaxStartRetries int `json:"maxStartRetries"`

	// 是否自动健康检查
	AutoHealthCheck bool `json:"autoHealthCheck"`
}

// LifecycleStats 生命周期统计
type LifecycleStats struct {
	// 启动次数
	StartCount uint64 `json:"startCount"`

	// 停止次数
	StopCount uint64 `json:"stopCount"`

	// 重启次数
	RestartCount uint64 `json:"restartCount"`

	// 健康检查次数
	HealthCheckCount uint64 `json:"healthCheckCount"`

	// 失败次数
	FailureCount uint64 `json:"failureCount"`

	// 总运行时间
	TotalUptime time.Duration `json:"totalUptime"`

	// 当前运行时间
	CurrentUptime time.Duration `json:"currentUptime"`

	// 最后启动时间
	LastStartTime time.Time `json:"lastStartTime"`

	// 最后停止时间
	LastStopTime time.Time `json:"lastStopTime"`
}

// StartHook 启动钩子函数类型
type StartHook func(ctx context.Context) error

// StopHook 停止钩子函数类型
type StopHook func(ctx context.Context) error

// HealthChecker 健康检查器接口
type HealthChecker interface {
	CheckHealth() *HealthCheckResult
	GetName() string
}

// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	Name      string                 `json:"name"`
	Healthy   bool                   `json:"healthy"`
	Message   string                 `json:"message,omitempty"`
	Latency   time.Duration          `json:"latency"`
	CheckTime time.Time              `json:"checkTime"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// HealthStatus 整体健康状态
type HealthStatus struct {
	Overall   bool                   `json:"overall"`
	Checks    []*HealthCheckResult   `json:"checks"`
	LastCheck time.Time              `json:"lastCheck"`
	Uptime    time.Duration          `json:"uptime"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// NewLifecycleManager 创建生命周期管理器
func NewLifecycleManager(config *LifecycleConfig, vm *VM) *LifecycleManager {
	if config == nil {
		config = defaultLifecycleConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &LifecycleManager{
		state:          StateUninitialized,
		config:         config,
		vm:             vm,
		startHooks:     make([]StartHook, 0),
		stopHooks:      make([]StopHook, 0),
		healthCheckers: make([]HealthChecker, 0),
		ctx:            ctx,
		cancel:         cancel,
		stats:          &LifecycleStats{},
	}
}

// StartEngine 启动引擎
func (lm *LifecycleManager) StartEngine() error {
	lm.stateMutex.Lock()
	defer lm.stateMutex.Unlock()

	if lm.state == StateRunning {
		return fmt.Errorf("引擎已在运行状态")
	}

	if lm.state == StateStopping {
		return fmt.Errorf("引擎正在停止中，无法启动")
	}

	lm.setState(StateInitializing)
	lm.startTime = time.Now()
	lm.stats.StartCount++
	lm.stats.LastStartTime = lm.startTime

	// 使用超时上下文
	ctx, cancel := context.WithTimeout(lm.ctx, lm.config.StartTimeout)
	defer cancel()

	// 执行启动流程
	if err := lm.performStart(ctx); err != nil {
		lm.setState(StateError)
		lm.stats.FailureCount++
		return fmt.Errorf("引擎启动失败: %w", err)
	}

	lm.setState(StateRunning)

	// 启动后台服务
	lm.startBackgroundServices()

	return nil
}

// performStart 执行启动流程
func (lm *LifecycleManager) performStart(ctx context.Context) error {
	// 1. 初始化组件
	if err := lm.initializeComponents(ctx); err != nil {
		return fmt.Errorf("初始化组件失败: %w", err)
	}

	// 2. 执行启动钩子
	for i, hook := range lm.startHooks {
		if err := hook(ctx); err != nil {
			return fmt.Errorf("启动钩子[%d]执行失败: %w", i, err)
		}
	}

	// 3. 预热组件
	if err := lm.warmupComponents(ctx); err != nil {
		return fmt.Errorf("组件预热失败: %w", err)
	}

	return nil
}

// initializeComponents 初始化组件
func (lm *LifecycleManager) initializeComponents(ctx context.Context) error {
	// VM已经在创建管理器时传入，这里可以进行其他初始化
	if lm.vm == nil {
		return fmt.Errorf("VM未初始化")
	}

	// 可以初始化其他组件
	return nil
}

// warmupComponents 预热组件
func (lm *LifecycleManager) warmupComponents(ctx context.Context) error {
	// 预热VM
	if lm.vm != nil {
		// 可以编译一个小的测试模块来预热
		// 这里省略具体实现
	}

	return nil
}

// startBackgroundServices 启动后台服务
func (lm *LifecycleManager) startBackgroundServices() {
	// 启动健康检查
	if lm.config.AutoHealthCheck {
		lm.wg.Add(1)
		go lm.healthCheckLoop()
	}

	// 启动统计更新
	lm.wg.Add(1)
	go lm.statsUpdateLoop()
}

// StopEngine 停止引擎
func (lm *LifecycleManager) StopEngine(timeout time.Duration) error {
	lm.stateMutex.Lock()
	defer lm.stateMutex.Unlock()

	if lm.state != StateRunning {
		return fmt.Errorf("引擎未在运行状态")
	}

	lm.setState(StateStopping)
	lm.stats.StopCount++
	lm.stats.LastStopTime = time.Now()

	// 计算运行时间
	if !lm.startTime.IsZero() {
		uptime := time.Since(lm.startTime)
		lm.stats.TotalUptime += uptime
		lm.stats.CurrentUptime = uptime
	}

	// 使用超时上下文
	if timeout <= 0 {
		timeout = lm.config.StopTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 执行停止流程
	if err := lm.performStop(ctx); err != nil {
		lm.setState(StateError)
		lm.stats.FailureCount++
		return fmt.Errorf("引擎停止失败: %w", err)
	}

	lm.setState(StateStopped)
	return nil
}

// performStop 执行停止流程
func (lm *LifecycleManager) performStop(ctx context.Context) error {
	// 1. 取消后台服务
	lm.cancel()

	// 2. 执行停止钩子
	for i, hook := range lm.stopHooks {
		if err := hook(ctx); err != nil {
			// 记录错误但继续停止流程
			fmt.Printf("停止钩子[%d]执行失败: %v\n", i, err)
		}
	}

	// 3. 停止组件
	if err := lm.shutdownComponents(ctx); err != nil {
		return fmt.Errorf("停止组件失败: %w", err)
	}

	// 4. 等待后台goroutine结束
	done := make(chan struct{})
	go func() {
		lm.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("停止超时")
	}
}

// shutdownComponents 关闭组件
func (lm *LifecycleManager) shutdownComponents(ctx context.Context) error {
	// 关闭VM
	if lm.vm != nil {
		if err := lm.vm.Close(ctx); err != nil {
			return fmt.Errorf("关闭VM失败: %w", err)
		}
	}

	return nil
}

// CheckHealth 健康检查
func (lm *LifecycleManager) CheckHealth() error {
	status := lm.checkHealth()
	if !status.Overall {
		return fmt.Errorf("健康检查失败: %v", status.Details)
	}
	return nil
}

// checkHealth 执行健康检查
func (lm *LifecycleManager) checkHealth() *HealthStatus {
	lm.stateMutex.RLock()
	defer lm.stateMutex.RUnlock()

	status := &HealthStatus{
		Overall:   true,
		Checks:    make([]*HealthCheckResult, 0),
		LastCheck: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// 基本状态检查
	if lm.state != StateRunning {
		status.Overall = false
		status.Details["state"] = lm.state.String()
	}

	// 运行时间
	if !lm.startTime.IsZero() {
		status.Uptime = time.Since(lm.startTime)
	}

	// 执行各个健康检查器
	for _, checker := range lm.healthCheckers {
		result := checker.CheckHealth()
		status.Checks = append(status.Checks, result)

		if !result.Healthy {
			status.Overall = false
		}
	}

	lm.healthStatus = status
	lm.lastHealthCheck = time.Now()
	lm.stats.HealthCheckCount++

	return status
}

// healthCheckLoop 健康检查循环
func (lm *LifecycleManager) healthCheckLoop() {
	defer lm.wg.Done()

	ticker := time.NewTicker(lm.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			lm.checkHealth()
		case <-lm.ctx.Done():
			return
		}
	}
}

// statsUpdateLoop 统计更新循环
func (lm *LifecycleManager) statsUpdateLoop() {
	defer lm.wg.Done()

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			lm.updateStats()
		case <-lm.ctx.Done():
			return
		}
	}
}

// updateStats 更新统计信息
func (lm *LifecycleManager) updateStats() {
	lm.stateMutex.RLock()
	defer lm.stateMutex.RUnlock()

	if lm.state == StateRunning && !lm.startTime.IsZero() {
		lm.stats.CurrentUptime = time.Since(lm.startTime)
	}
}

// RegisterStartHook 注册启动钩子
func (lm *LifecycleManager) RegisterStartHook(hook StartHook) {
	lm.startHooks = append(lm.startHooks, hook)
}

// RegisterStopHook 注册停止钩子
func (lm *LifecycleManager) RegisterStopHook(hook StopHook) {
	lm.stopHooks = append(lm.stopHooks, hook)
}

// RegisterHealthChecker 注册健康检查器
func (lm *LifecycleManager) RegisterHealthChecker(checker HealthChecker) {
	lm.healthCheckers = append(lm.healthCheckers, checker)
}

// GetState 获取当前状态
func (lm *LifecycleManager) GetState() LifecycleState {
	lm.stateMutex.RLock()
	defer lm.stateMutex.RUnlock()
	return lm.state
}

// setState 设置状态（内部使用）
func (lm *LifecycleManager) setState(state LifecycleState) {
	lm.state = state
}

// GetStats 获取统计信息
func (lm *LifecycleManager) GetStats() *LifecycleStats {
	lm.stateMutex.RLock()
	defer lm.stateMutex.RUnlock()

	// 返回副本
	stats := *lm.stats
	return &stats
}

// GetHealthStatus 获取健康状态
func (lm *LifecycleManager) GetHealthStatus() *HealthStatus {
	lm.stateMutex.RLock()
	defer lm.stateMutex.RUnlock()

	if lm.healthStatus == nil {
		return lm.checkHealth()
	}

	return lm.healthStatus
}

// Restart 重启引擎
func (lm *LifecycleManager) Restart() error {
	// 先停止
	if err := lm.StopEngine(lm.config.StopTimeout); err != nil {
		return fmt.Errorf("重启时停止失败: %w", err)
	}

	// 再启动
	if err := lm.StartEngine(); err != nil {
		return fmt.Errorf("重启时启动失败: %w", err)
	}

	lm.stats.RestartCount++
	return nil
}

// defaultLifecycleConfig 默认生命周期配置
func defaultLifecycleConfig() *LifecycleConfig {
	return &LifecycleConfig{
		StartTimeout:        30 * time.Second,
		StopTimeout:         10 * time.Second,
		HealthCheckInterval: 30 * time.Second,
		GracefulShutdown:    true,
		MaxStartRetries:     3,
		AutoHealthCheck:     true,
	}
}

// ==================== 兼容函数 ====================

// Start 启动引擎底层运行时（兼容原接口）
func Start(ctx context.Context, vm *VM) error {
	if vm == nil {
		return fmt.Errorf("VM不能为空")
	}

	manager := NewLifecycleManager(nil, vm)
	return manager.StartEngine()
}

// Health 健康检查（兼容原接口）
func Health(ctx context.Context, vm *VM) error {
	if vm == nil {
		return fmt.Errorf("VM不能为空")
	}

	manager := NewLifecycleManager(nil, vm)
	return manager.CheckHealth()
}

// Stop 停止引擎运行时（兼容原接口）
func Stop(ctx context.Context, vm *VM) error {
	if vm == nil {
		return fmt.Errorf("VM不能为空")
	}

	manager := NewLifecycleManager(nil, vm)
	return manager.StopEngine(10 * time.Second)
}
