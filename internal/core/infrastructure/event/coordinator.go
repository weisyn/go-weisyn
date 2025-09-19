// Package event 事件协调器实现
package event

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	evbus "github.com/asaskevich/EventBus"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// EventCoordinator 事件协调器接口
// 作为所有事件功能的统一协调中心
type EventCoordinator interface {
	// 生命周期管理
	Start(ctx context.Context) error
	Stop() error
	IsRunning() bool

	// 事件发布
	PublishEvent(eventType string, data interface{}) error
	PublishEventWithPriority(eventType string, data interface{}, priority Priority) error
	PublishEventWithMetadata(metadata *EventMetadata, data interface{}) error

	// 事件订阅
	SubscribeEvent(eventType string, handler interface{}) (string, error)
	SubscribeEventWithOptions(eventType string, handler interface{}, options ...SubscriptionOption) (string, error)
	UnsubscribeEvent(subscriptionID string) error

	// 域管理
	RegisterDomain(domain string, info DomainInfo) error
	UnregisterDomain(domain string) error
	IsDomainRegistered(domain string) bool
	ListDomains() []string

	// 验证管理
	AddValidationRule(rule ValidationRule) error
	RemoveValidationRule(ruleID string) error
	ListValidationRules() []ValidationRule

	// 路由管理
	SetRouteStrategy(eventType string, strategy RouteStrategy) error
	GetRouteStrategy(eventType string) RouteStrategy

	// 配置管理
	UpdateConfig(config *CoordinatorConfig) error
	GetConfig() *CoordinatorConfig

	// 统计和监控
	GetStatistics() *CoordinatorStatistics
	GetHealthStatus() *HealthStatus

	// 批量操作
	BatchPublishEvents(events []EventRequest) []EventResult
	BatchValidateEvents(events []Event) []ValidationResult
}

// EventRequest 事件发布请求
type EventRequest struct {
	EventType string                 `json:"event_type"`
	Data      interface{}            `json:"data"`
	Priority  Priority               `json:"priority"`
	Source    string                 `json:"source"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// EventResult 事件发布结果
type EventResult struct {
	EventType string        `json:"event_type"`
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
	EventID   string        `json:"event_id,omitempty"`
}

// CoordinatorConfig 协调器配置
type CoordinatorConfig struct {
	// 功能开关
	EnableDomainRegistry   bool `json:"enable_domain_registry"`
	EnableEventRouter      bool `json:"enable_event_router"`
	EnableEventValidator   bool `json:"enable_event_validator"`
	EnableEnhancedFeatures bool `json:"enable_enhanced_features"`

	// 域管理配置
	StrictDomainCheck      bool `json:"strict_domain_check"`
	AutoCreateDomains      bool `json:"auto_create_domains"`
	RequireDomainExistence bool `json:"require_domain_existence"`

	// 性能配置
	MaxConcurrentEvents    int           `json:"max_concurrent_events"`
	EventProcessingTimeout time.Duration `json:"event_processing_timeout"`
	RetryMaxAttempts       int           `json:"retry_max_attempts"`
	RetryInitialDelay      time.Duration `json:"retry_initial_delay"`
	RetryMaxDelay          time.Duration `json:"retry_max_delay"`

	// 监控配置
	EnableStatistics         bool          `json:"enable_statistics"`
	StatisticsUpdateInterval time.Duration `json:"statistics_update_interval"`
	EnableHealthCheck        bool          `json:"enable_health_check"`
	HealthCheckInterval      time.Duration `json:"health_check_interval"`

	// 错误处理
	EnableErrorRecovery     bool   `json:"enable_error_recovery"`
	ErrorRetryStrategy      string `json:"error_retry_strategy"` // "linear", "exponential", "fixed"
	IgnoreNonCriticalErrors bool   `json:"ignore_non_critical_errors"`
}

// CoordinatorStatistics 协调器统计信息
type CoordinatorStatistics struct {
	// 基础统计
	TotalEvents   atomic.Uint64 `json:"total_events"`
	SuccessEvents atomic.Uint64 `json:"success_events"`
	FailedEvents  atomic.Uint64 `json:"failed_events"`

	// 性能统计
	AverageLatency atomic.Pointer[time.Duration] `json:"average_latency"`
	MaxLatency     atomic.Pointer[time.Duration] `json:"max_latency"`
	MinLatency     atomic.Pointer[time.Duration] `json:"min_latency"`

	// 组件统计
	DomainRegistryStats map[string]interface{} `json:"domain_registry_stats"`
	EventRouterStats    map[string]interface{} `json:"event_router_stats"`
	EventValidatorStats *ValidatorStatistics   `json:"event_validator_stats"`

	// 时间信息
	StartTime      time.Time                 `json:"start_time"`
	LastEventTime  atomic.Pointer[time.Time] `json:"last_event_time"`
	UptimeDuration time.Duration             `json:"uptime_duration"`
}

// HealthStatus 健康状态
type HealthStatus struct {
	Overall    HealthLevel            `json:"overall"`
	Components map[string]HealthLevel `json:"components"`
	Issues     []string               `json:"issues,omitempty"`
	LastCheck  time.Time              `json:"last_check"`
}

// HealthLevel 健康等级
type HealthLevel string

const (
	HealthHealthy  HealthLevel = "healthy"
	HealthWarning  HealthLevel = "warning"
	HealthCritical HealthLevel = "critical"
	HealthUnknown  HealthLevel = "unknown"
)

// DefaultCoordinatorConfig 默认协调器配置
func DefaultCoordinatorConfig() *CoordinatorConfig {
	return &CoordinatorConfig{
		EnableDomainRegistry:     true,
		EnableEventRouter:        true,
		EnableEventValidator:     true,
		EnableEnhancedFeatures:   true,
		StrictDomainCheck:        false,
		AutoCreateDomains:        false,
		RequireDomainExistence:   false,
		MaxConcurrentEvents:      1000,
		EventProcessingTimeout:   30 * time.Second,
		RetryMaxAttempts:         3,
		RetryInitialDelay:        100 * time.Millisecond,
		RetryMaxDelay:            5 * time.Second,
		EnableStatistics:         true,
		StatisticsUpdateInterval: 10 * time.Second,
		EnableHealthCheck:        true,
		HealthCheckInterval:      30 * time.Second,
		EnableErrorRecovery:      true,
		ErrorRetryStrategy:       "exponential",
		IgnoreNonCriticalErrors:  false,
	}
}

// BasicEventCoordinator 基础事件协调器实现
type BasicEventCoordinator struct {
	// 核心组件
	domainRegistry *DomainRegistry
	eventRouter    *EventRouter
	eventValidator EventValidator
	eventBus       evbus.Bus

	// 配置和状态
	config  *CoordinatorConfig
	stats   *CoordinatorStatistics
	health  *HealthStatus
	running atomic.Bool

	// 上下文管理
	ctx    context.Context
	cancel context.CancelFunc

	// 并发控制
	mu        sync.RWMutex
	semaphore chan struct{} // 并发事件限制

	// 依赖项
	logger log.Logger
}

// NewBasicEventCoordinator 创建基础事件协调器
func NewBasicEventCoordinator(
	logger log.Logger,
	config *CoordinatorConfig,
	domainRegistry *DomainRegistry,
	eventRouter *EventRouter,
	eventValidator EventValidator,
	eventBus evbus.Bus,
) *BasicEventCoordinator {
	if config == nil {
		config = DefaultCoordinatorConfig()
	}

	var componentLogger log.Logger
	if logger != nil {
		componentLogger = logger.With("component", "event_coordinator")
	}

	coordinator := &BasicEventCoordinator{
		domainRegistry: domainRegistry,
		eventRouter:    eventRouter,
		eventValidator: eventValidator,
		eventBus:       eventBus,
		config:         config,
		stats:          newCoordinatorStatistics(),
		health:         newHealthStatus(),
		semaphore:      make(chan struct{}, config.MaxConcurrentEvents),
		logger:         componentLogger,
	}

	return coordinator
}

// newCoordinatorStatistics 创建协调器统计信息
func newCoordinatorStatistics() *CoordinatorStatistics {
	stats := &CoordinatorStatistics{
		DomainRegistryStats: make(map[string]interface{}),
		EventRouterStats:    make(map[string]interface{}),
		StartTime:           time.Now(),
	}

	// 初始化原子指针
	zero := time.Duration(0)
	stats.AverageLatency.Store(&zero)
	stats.MaxLatency.Store(&zero)
	stats.MinLatency.Store(&zero)

	return stats
}

// newHealthStatus 创建健康状态
func newHealthStatus() *HealthStatus {
	return &HealthStatus{
		Overall:    HealthUnknown,
		Components: make(map[string]HealthLevel),
		Issues:     make([]string, 0),
		LastCheck:  time.Now(),
	}
}

// Start 启动协调器
func (c *BasicEventCoordinator) Start(ctx context.Context) error {
	if c.running.Load() {
		return fmt.Errorf("event coordinator already running")
	}

	c.ctx, c.cancel = context.WithCancel(ctx)

	// 启动子组件
	if err := c.startComponents(); err != nil {
		return fmt.Errorf("failed to start components: %w", err)
	}

	// 启动监控协程
	if c.config.EnableStatistics {
		go c.statisticsWorker()
	}

	if c.config.EnableHealthCheck {
		go c.healthCheckWorker()
	}

	c.running.Store(true)
	c.stats.StartTime = time.Now()

	c.logger.Info("事件协调器已启动")
	return nil
}

// Stop 停止协调器
func (c *BasicEventCoordinator) Stop() error {
	if !c.running.Load() {
		return fmt.Errorf("event coordinator not running")
	}

	c.running.Store(false)

	// 停止子组件
	if err := c.stopComponents(); err != nil {
		c.logger.Errorf("停止组件时出错: %v", err)
	}

	// 取消上下文
	if c.cancel != nil {
		c.cancel()
	}

	c.logger.Info("事件协调器已停止")
	return nil
}

// IsRunning 检查是否运行中
func (c *BasicEventCoordinator) IsRunning() bool {
	return c.running.Load()
}

// startComponents 启动子组件
func (c *BasicEventCoordinator) startComponents() error {
	// 启动事件路由器
	if c.config.EnableEventRouter && c.eventRouter != nil {
		if err := c.eventRouter.Start(c.ctx); err != nil {
			return fmt.Errorf("failed to start event router: %w", err)
		}
	}

	return nil
}

// stopComponents 停止子组件
func (c *BasicEventCoordinator) stopComponents() error {
	var errors []error

	// 停止事件路由器
	if c.eventRouter != nil {
		if err := c.eventRouter.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop event router: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("multiple errors stopping components: %v", errors)
	}

	return nil
}

// PublishEvent 发布事件
func (c *BasicEventCoordinator) PublishEvent(eventType string, data interface{}) error {
	return c.PublishEventWithPriority(eventType, data, PriorityNormal)
}

// PublishEventWithPriority 带优先级发布事件
func (c *BasicEventCoordinator) PublishEventWithPriority(eventType string, data interface{}, priority Priority) error {
	if !c.running.Load() {
		return fmt.Errorf("event coordinator not running")
	}

	startTime := time.Now()
	defer c.updateEventStatistics(time.Since(startTime), true)

	// 获取并发许可
	select {
	case c.semaphore <- struct{}{}:
		defer func() { <-c.semaphore }()
	case <-c.ctx.Done():
		return fmt.Errorf("event coordinator stopped")
	default:
		return fmt.Errorf("too many concurrent events")
	}

	// 创建事件上下文
	ctx, cancel := context.WithTimeout(c.ctx, c.config.EventProcessingTimeout)
	defer cancel()

	// 1. 事件验证
	if c.config.EnableEventValidator && c.eventValidator != nil {
		event := &basicEvent{eventType: eventType, data: data}

		if c.config.EnableDomainRegistry && c.domainRegistry != nil {
			// 域验证
			if err := c.eventValidator.ValidateEventWithDomain(event, c.domainRegistry, c.config.StrictDomainCheck); err != nil {
				c.updateEventStatistics(time.Since(startTime), false)
				return fmt.Errorf("domain validation failed: %w", err)
			}
		} else {
			// 基础验证
			if err := c.eventValidator.ValidateEventWithContext(ctx, event); err != nil {
				c.updateEventStatistics(time.Since(startTime), false)
				return fmt.Errorf("event validation failed: %w", err)
			}
		}
	}

	// 2. 事件路由
	if c.config.EnableEventRouter && c.eventRouter != nil {
		source := "coordinator"
		if err := c.eventRouter.RouteEvent(eventType, data, priority, source); err != nil {
			c.updateEventStatistics(time.Since(startTime), false)
			return fmt.Errorf("event routing failed: %w", err)
		}
	} else {
		// 直接发布到EventBus
		c.eventBus.Publish(eventType, data)
	}

	c.logger.Debugf("事件发布成功: type=%s, priority=%v", eventType, priority)
	return nil
}

// PublishEventWithMetadata 带元数据发布事件
func (c *BasicEventCoordinator) PublishEventWithMetadata(metadata *EventMetadata, data interface{}) error {
	if metadata == nil {
		return fmt.Errorf("metadata cannot be nil")
	}

	return c.PublishEventWithPriority(metadata.Name, data, metadata.Priority)
}

// SubscribeEvent 订阅事件
func (c *BasicEventCoordinator) SubscribeEvent(eventType string, handler interface{}) (string, error) {
	return c.SubscribeEventWithOptions(eventType, handler)
}

// SubscribeEventWithOptions 带选项订阅事件
func (c *BasicEventCoordinator) SubscribeEventWithOptions(eventType string, handler interface{}, options ...SubscriptionOption) (string, error) {
	if !c.running.Load() {
		return "", fmt.Errorf("event coordinator not running")
	}

	// 验证事件名称
	if c.config.EnableEventValidator {
		if err := ValidateEventName(eventType); err != nil {
			return "", fmt.Errorf("invalid event type: %w", err)
		}
	}

	// 检查域注册
	if c.config.EnableDomainRegistry && c.config.RequireDomainExistence && c.domainRegistry != nil {
		domain := ExtractDomainFromEventName(eventType)
		if !c.domainRegistry.IsDomainRegistered(domain) {
			if c.config.AutoCreateDomains {
				// 自动创建域
				info := DomainInfo{
					Component:   "auto-created",
					Description: fmt.Sprintf("Auto-created domain for %s", domain),
				}
				if err := c.domainRegistry.RegisterDomain(domain, info); err != nil {
					return "", fmt.Errorf("failed to auto-create domain %s: %w", domain, err)
				}
				c.logger.Infof("自动创建域: %s", domain)
			} else {
				return "", fmt.Errorf("domain %s not registered", domain)
			}
		}
	}

	// 使用路由器订阅
	if c.config.EnableEventRouter && c.eventRouter != nil {
		return c.eventRouter.AddSubscription(eventType, handler, options...)
	}

	// 直接订阅EventBus
	return c.subscribeToEventBus(eventType, handler)
}

// UnsubscribeEvent 取消订阅事件
func (c *BasicEventCoordinator) UnsubscribeEvent(subscriptionID string) error {
	if !c.running.Load() {
		return fmt.Errorf("event coordinator not running")
	}

	// 使用路由器取消订阅
	if c.config.EnableEventRouter && c.eventRouter != nil {
		return c.eventRouter.RemoveSubscription(subscriptionID)
	}

	// 从EventBus取消订阅 (EventBus接口限制，需要保存映射)
	c.logger.Warnf("取消订阅功能需要EventBus接口支持: %s", subscriptionID)
	return nil
}

// RegisterDomain 注册域
func (c *BasicEventCoordinator) RegisterDomain(domain string, info DomainInfo) error {
	if !c.config.EnableDomainRegistry || c.domainRegistry == nil {
		return fmt.Errorf("domain registry not enabled")
	}

	return c.domainRegistry.RegisterDomain(domain, info)
}

// UnregisterDomain 注销域
func (c *BasicEventCoordinator) UnregisterDomain(domain string) error {
	if !c.config.EnableDomainRegistry || c.domainRegistry == nil {
		return fmt.Errorf("domain registry not enabled")
	}

	return c.domainRegistry.UnregisterDomain(domain)
}

// IsDomainRegistered 检查域是否已注册
func (c *BasicEventCoordinator) IsDomainRegistered(domain string) bool {
	if !c.config.EnableDomainRegistry || c.domainRegistry == nil {
		return false
	}

	return c.domainRegistry.IsDomainRegistered(domain)
}

// ListDomains 列出所有域
func (c *BasicEventCoordinator) ListDomains() []string {
	if !c.config.EnableDomainRegistry || c.domainRegistry == nil {
		return []string{}
	}

	return c.domainRegistry.ListDomains()
}

// AddValidationRule 添加验证规则
func (c *BasicEventCoordinator) AddValidationRule(rule ValidationRule) error {
	if !c.config.EnableEventValidator || c.eventValidator == nil {
		return fmt.Errorf("event validator not enabled")
	}

	return c.eventValidator.AddRule(rule)
}

// RemoveValidationRule 移除验证规则
func (c *BasicEventCoordinator) RemoveValidationRule(ruleID string) error {
	if !c.config.EnableEventValidator || c.eventValidator == nil {
		return fmt.Errorf("event validator not enabled")
	}

	return c.eventValidator.RemoveRule(ruleID)
}

// ListValidationRules 列出验证规则
func (c *BasicEventCoordinator) ListValidationRules() []ValidationRule {
	if !c.config.EnableEventValidator || c.eventValidator == nil {
		return []ValidationRule{}
	}

	return c.eventValidator.GetRules()
}

// SetRouteStrategy 设置路由策略
func (c *BasicEventCoordinator) SetRouteStrategy(eventType string, strategy RouteStrategy) error {
	if !c.config.EnableEventRouter || c.eventRouter == nil {
		return fmt.Errorf("event router not enabled")
	}

	c.eventRouter.SetRouteStrategy(eventType, strategy)
	return nil
}

// GetRouteStrategy 获取路由策略
func (c *BasicEventCoordinator) GetRouteStrategy(eventType string) RouteStrategy {
	if !c.config.EnableEventRouter || c.eventRouter == nil {
		return RouteBroadcast // 默认策略
	}

	return c.eventRouter.GetRouteStrategy(eventType)
}

// UpdateConfig 更新配置
func (c *BasicEventCoordinator) UpdateConfig(config *CoordinatorConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	c.config = config

	// 更新信号量大小
	if cap(c.semaphore) != config.MaxConcurrentEvents {
		close(c.semaphore)
		c.semaphore = make(chan struct{}, config.MaxConcurrentEvents)
	}

	c.logger.Infof("协调器配置已更新")
	return nil
}

// GetConfig 获取配置
func (c *BasicEventCoordinator) GetConfig() *CoordinatorConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 返回配置副本
	config := *c.config
	return &config
}

// GetStatistics 获取统计信息
func (c *BasicEventCoordinator) GetStatistics() *CoordinatorStatistics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 更新组件统计
	if c.domainRegistry != nil {
		c.stats.DomainRegistryStats = c.domainRegistry.GetStatistics()
	}

	if c.eventRouter != nil {
		c.stats.EventRouterStats = c.eventRouter.GetStatistics()
	}

	if c.eventValidator != nil {
		c.stats.EventValidatorStats = c.eventValidator.GetStatistics()
	}

	// 计算运行时间
	if c.running.Load() {
		c.stats.UptimeDuration = time.Since(c.stats.StartTime)
	}

	return c.stats
}

// GetHealthStatus 获取健康状态
func (c *BasicEventCoordinator) GetHealthStatus() *HealthStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.health
}

// BatchPublishEvents 批量发布事件
func (c *BasicEventCoordinator) BatchPublishEvents(events []EventRequest) []EventResult {
	results := make([]EventResult, len(events))

	for i, event := range events {
		startTime := time.Now()
		err := c.PublishEventWithPriority(event.EventType, event.Data, event.Priority)
		duration := time.Since(startTime)

		results[i] = EventResult{
			EventType: event.EventType,
			Success:   err == nil,
			Duration:  duration,
		}

		if err != nil {
			results[i].Error = err.Error()
		}
	}

	return results
}

// BatchValidateEvents 批量验证事件
func (c *BasicEventCoordinator) BatchValidateEvents(events []Event) []ValidationResult {
	if !c.config.EnableEventValidator || c.eventValidator == nil {
		return nil
	}

	return c.eventValidator.BatchValidate(events)
}

// 内部辅助方法

// subscribeToEventBus 直接订阅EventBus
func (c *BasicEventCoordinator) subscribeToEventBus(eventType string, handler interface{}) (string, error) {
	if err := c.eventBus.Subscribe(eventType, handler); err != nil {
		return "", fmt.Errorf("failed to subscribe to event bus: %w", err)
	}

	// 生成订阅ID（EventBus没有提供）
	subscriptionID := fmt.Sprintf("evbus_%s_%d", eventType, time.Now().UnixNano())
	return subscriptionID, nil
}

// updateEventStatistics 更新事件统计
func (c *BasicEventCoordinator) updateEventStatistics(duration time.Duration, success bool) {
	c.stats.TotalEvents.Add(1)
	if success {
		c.stats.SuccessEvents.Add(1)
	} else {
		c.stats.FailedEvents.Add(1)
	}

	// 更新延迟统计
	c.updateLatencyStats(duration)

	// 更新最后事件时间
	now := time.Now()
	c.stats.LastEventTime.Store(&now)
}

// updateLatencyStats 更新延迟统计
func (c *BasicEventCoordinator) updateLatencyStats(duration time.Duration) {
	// 更新平均延迟
	if avgLatency := c.stats.AverageLatency.Load(); avgLatency != nil {
		alpha := 0.1 // 平滑因子
		newAvg := time.Duration(float64(*avgLatency)*(1-alpha) + float64(duration)*alpha)
		c.stats.AverageLatency.Store(&newAvg)
	} else {
		c.stats.AverageLatency.Store(&duration)
	}

	// 更新最大延迟
	if maxLatency := c.stats.MaxLatency.Load(); maxLatency == nil || duration > *maxLatency {
		c.stats.MaxLatency.Store(&duration)
	}

	// 更新最小延迟
	if minLatency := c.stats.MinLatency.Load(); minLatency == nil || duration < *minLatency {
		c.stats.MinLatency.Store(&duration)
	}
}

// statisticsWorker 统计工作协程
func (c *BasicEventCoordinator) statisticsWorker() {
	ticker := time.NewTicker(c.config.StatisticsUpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 定期更新统计信息（如果需要）
			c.logger.Debugf("统计信息更新: total=%d, success=%d, failed=%d",
				c.stats.TotalEvents.Load(),
				c.stats.SuccessEvents.Load(),
				c.stats.FailedEvents.Load())

		case <-c.ctx.Done():
			return
		}
	}
}

// healthCheckWorker 健康检查工作协程
func (c *BasicEventCoordinator) healthCheckWorker() {
	ticker := time.NewTicker(c.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.performHealthCheck()

		case <-c.ctx.Done():
			return
		}
	}
}

// performHealthCheck 执行健康检查
func (c *BasicEventCoordinator) performHealthCheck() {
	c.mu.Lock()
	defer c.mu.Unlock()

	health := newHealthStatus()

	// 检查各组件健康状态
	if c.domainRegistry != nil {
		health.Components["domain_registry"] = HealthHealthy
	}

	if c.eventRouter != nil {
		if c.eventRouter.IsRunning() {
			health.Components["event_router"] = HealthHealthy
		} else {
			health.Components["event_router"] = HealthCritical
			health.Issues = append(health.Issues, "Event router not running")
		}
	}

	if c.eventValidator != nil {
		health.Components["event_validator"] = HealthHealthy
	}

	// 计算整体健康状态
	health.Overall = c.calculateOverallHealth(health.Components)
	health.LastCheck = time.Now()

	c.health = health
}

// calculateOverallHealth 计算整体健康状态
func (c *BasicEventCoordinator) calculateOverallHealth(components map[string]HealthLevel) HealthLevel {
	if len(components) == 0 {
		return HealthUnknown
	}

	hasCritical := false
	hasWarning := false

	for _, level := range components {
		switch level {
		case HealthCritical:
			hasCritical = true
		case HealthWarning:
			hasWarning = true
		}
	}

	if hasCritical {
		return HealthCritical
	}
	if hasWarning {
		return HealthWarning
	}
	return HealthHealthy
}

// basicEvent 基础事件实现
type basicEvent struct {
	eventType string
	data      interface{}
}

func (e *basicEvent) Type() string {
	return e.eventType
}

func (e *basicEvent) Data() interface{} {
	return e.data
}
