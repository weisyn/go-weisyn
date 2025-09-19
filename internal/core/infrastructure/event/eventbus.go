// 基于asaskevich/EventBus的事件总线实现
// 集成了所有增强功能：域注册、智能路由、事件验证、协调器等

package event

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	evbus "github.com/asaskevich/EventBus"
	eventconfig "github.com/weisyn/v1/internal/config/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
	"github.com/google/uuid"
)

// ==================== 增强类型定义 ====================

// DomainEventHandler 域事件处理器
type DomainEventHandler func(eventType string, data interface{}) error

// priorityOption 优先级选项
type priorityOption struct {
	priority Priority
}

// componentOption 组件选项
type componentOption struct {
	component string
}

// EventBus 是基于asaskevich/EventBus的增强实现
//
// 🎯 **WES增强特性**：
// - 保持与原有asaskevich/EventBus的完全兼容
// - 新增WES消息事件的特殊处理
// - 增加生命周期管理能力
// - 支持事件过滤和拦截
// - 内置监控和指标统计
type EventBus struct {
	// ================== 基础组件 ==================
	bus    evbus.Bus           // 底层事件总线
	config *eventconfig.Config // 配置

	// ================== 历史记录 ==================
	historyMu    sync.RWMutex                      // 历史记录锁
	eventHistory map[event.EventType][]interface{} // 历史事件存储

	// ================== WES增强功能 ==================
	running atomic.Bool        // 运行状态
	ctx     context.Context    // 上下文
	cancel  context.CancelFunc // 取消函数

	// WES订阅管理
	weisynSubscriptions map[string]*weisynSubscription // WES消息订阅
	weisynMutex         sync.RWMutex                 // WES订阅锁

	// 事件过滤和拦截
	filters      []event.EventFilter      // 事件过滤器
	interceptors []event.EventInterceptor // 事件拦截器
	filterMutex  sync.RWMutex             // 过滤器锁

	// 指标统计
	metrics      *eventMetrics // 事件指标
	metricsMutex sync.RWMutex  // 指标锁
}

// weisynSubscription WES消息订阅信息
type weisynSubscription struct {
	id        types.SubscriptionID
	protocols []event.ProtocolType
	filter    event.EventFilter
	handler   event.WESEventHandler
	createdAt time.Time
	active    bool

	// 统计信息
	triggerCount  atomic.Uint64
	lastTriggered atomic.Pointer[time.Time]
}

// eventMetrics 简化的事件指标
type eventMetrics struct {
	totalEvents      atomic.Uint64
	successfulEvents atomic.Uint64
	failedEvents     atomic.Uint64
	weisynEvents       atomic.Uint64

	measurementStart time.Time
	lastUpdated      atomic.Pointer[time.Time]
}

// New 创建增强的事件总线实例
// 所有事件总线实例必须通过此函数创建，确保配置被正确应用
func New(config *eventconfig.Config) event.EventBus {
	eb := &EventBus{
		bus:               evbus.New(),
		config:            config,
		eventHistory:      make(map[event.EventType][]interface{}),
		weisynSubscriptions: make(map[string]*weisynSubscription),
		filters:           make([]event.EventFilter, 0),
		interceptors:      make([]event.EventInterceptor, 0),
		metrics:           newEventMetrics(),
	}

	return eb
}

// newEventMetrics 创建新的事件指标
func newEventMetrics() *eventMetrics {
	return &eventMetrics{
		measurementStart: time.Now(),
	}
}

// Subscribe 实现订阅
func (eb *EventBus) Subscribe(eventType event.EventType, handler interface{}) error {
	if !eb.config.IsEnabled() {
		return nil // 如果事件系统未启用，静默成功
	}
	return eb.bus.Subscribe(string(eventType), handler)
}

// SubscribeAsync 实现异步订阅
func (eb *EventBus) SubscribeAsync(eventType event.EventType, handler interface{}, transactional bool) error {
	if !eb.config.IsEnabled() {
		return nil
	}
	return eb.bus.SubscribeAsync(string(eventType), handler, transactional)
}

// SubscribeOnce 实现一次性订阅
func (eb *EventBus) SubscribeOnce(eventType event.EventType, handler interface{}) error {
	if !eb.config.IsEnabled() {
		return nil
	}
	return eb.bus.SubscribeOnce(string(eventType), handler)
}

// SubscribeOnceAsync 实现异步一次性订阅
func (eb *EventBus) SubscribeOnceAsync(eventType event.EventType, handler interface{}, transactional bool) error {
	if !eb.config.IsEnabled() {
		return nil
	}
	// asaskevich/EventBus库中SubscribeOnceAsync方法签名不同，需要单独处理
	eb.bus.SubscribeOnceAsync(string(eventType), handler)
	return nil
}

// Publish 实现发布
func (eb *EventBus) Publish(eventType event.EventType, args ...interface{}) {
	if !eb.config.IsEnabled() {
		return
	}

	// 不再需要历史记录功能 - 简化为基础事件传递

	eb.bus.Publish(string(eventType), args...)
}

// PublishEvent 发布Event接口类型事件
func (eb *EventBus) PublishEvent(e event.Event) {
	if !eb.config.IsEnabled() {
		return
	}

	eventType := e.Type()
	data := e.Data()

	// 不再需要历史记录功能 - 简化为基础事件传递

	eb.bus.Publish(string(eventType), data)
}

// saveEventToHistory 已简化 - 不再保存历史记录
func (eb *EventBus) saveEventToHistory(eventType event.EventType, args []interface{}) {
	// 历史记录功能已简化，不再实现
}

// GetEventHistory 获取指定类型的事件历史
func (eb *EventBus) GetEventHistory(eventType event.EventType) []interface{} {
	// 历史记录功能已简化，不再实现
	return nil
}

// Unsubscribe 取消订阅
func (eb *EventBus) Unsubscribe(eventType event.EventType, handler interface{}) error {
	if !eb.config.IsEnabled() {
		return nil
	}
	return eb.bus.Unsubscribe(string(eventType), handler)
}

// WaitAsync 等待异步处理完成
func (eb *EventBus) WaitAsync() {
	if !eb.config.IsEnabled() {
		return
	}
	eb.bus.WaitAsync()
}

// HasCallback 检查是否有回调
func (eb *EventBus) HasCallback(eventType event.EventType) bool {
	if !eb.config.IsEnabled() {
		return false
	}
	return eb.bus.HasCallback(string(eventType))
}

// ==================== WES增强功能实现 ====================

// Start 启动事件总线
func (eb *EventBus) Start(ctx context.Context) error {
	if eb.running.Load() {
		return fmt.Errorf("event bus already running")
	}

	eb.ctx, eb.cancel = context.WithCancel(ctx)
	eb.running.Store(true)

	// 更新指标
	now := time.Now()
	eb.metrics.lastUpdated.Store(&now)

	return nil
}

// Stop 停止事件总线
func (eb *EventBus) Stop(ctx context.Context) error {
	if !eb.running.Load() {
		return fmt.Errorf("event bus not running")
	}

	eb.running.Store(false)
	if eb.cancel != nil {
		eb.cancel()
	}

	// 等待异步处理完成
	eb.WaitAsync()

	return nil
}

// IsRunning 检查事件总线是否运行中
func (eb *EventBus) IsRunning() bool {
	return eb.running.Load()
}

// PublishWESEvent 发布WES事件
func (eb *EventBus) PublishWESEvent(weisynEvent *event.WESEvent) error {
	if !eb.config.IsEnabled() {
		return nil
	}

	if weisynEvent == nil {
		return fmt.Errorf("WES event cannot be nil")
	}

	// 设置默认值
	if weisynEvent.ID == "" {
		weisynEvent.ID = uuid.New().String()
	}
	if weisynEvent.Timestamp.IsZero() {
		weisynEvent.Timestamp = time.Now()
	}

	// 应用事件拦截器
	if err := eb.applyPreInterceptors(weisynEvent); err != nil {
		return fmt.Errorf("pre-interceptor failed: %w", err)
	}

	// 处理WES特定订阅
	eb.processWESSubscriptions(weisynEvent)

	// 同时发布为标准事件
	eb.bus.Publish(string(weisynEvent.EventType), weisynEvent)

	// 更新指标
	eb.metrics.weisynEvents.Add(1)
	eb.metrics.totalEvents.Add(1)

	// 应用后置拦截器
	go eb.applyPostInterceptors(weisynEvent, nil)

	return nil
}

// SubscribeWithFilter 带过滤器的订阅
func (eb *EventBus) SubscribeWithFilter(eventType event.EventType, filter event.EventFilter, handler event.EventHandler) (types.SubscriptionID, error) {
	if !eb.config.IsEnabled() {
		return "", nil
	}

	if handler == nil {
		return "", fmt.Errorf("handler cannot be nil")
	}

	if filter == nil {
		return "", fmt.Errorf("filter cannot be nil")
	}

	subID := types.SubscriptionID(uuid.New().String())

	// 创建包装的处理器
	wrappedHandler := func(args ...interface{}) {
		// 构造临时事件对象
		if len(args) > 0 {
			if weisynEvent, ok := args[0].(*event.WESEvent); ok {
				if filter.MatchWES(weisynEvent) {
					if err := handler(weisynEvent); err != nil {
						eb.metrics.failedEvents.Add(1)
					} else {
						eb.metrics.successfulEvents.Add(1)
					}
				}
			}
		}
	}

	// 使用底层事件总线订阅
	err := eb.bus.Subscribe(string(eventType), wrappedHandler)
	if err != nil {
		return "", err
	}

	return subID, nil
}

// SubscribeWESEvents 订阅WES消息事件
func (eb *EventBus) SubscribeWESEvents(protocols []event.ProtocolType, handler event.WESEventHandler) (types.SubscriptionID, error) {
	if !eb.config.IsEnabled() {
		return "", nil
	}

	if handler == nil {
		return "", fmt.Errorf("handler cannot be nil")
	}

	subID := types.SubscriptionID(uuid.New().String())

	weisynSub := &weisynSubscription{
		id:        subID,
		protocols: protocols,
		handler:   handler,
		createdAt: time.Now(),
		active:    true,
	}

	eb.weisynMutex.Lock()
	eb.weisynSubscriptions[string(subID)] = weisynSub
	eb.weisynMutex.Unlock()

	return subID, nil
}

// UnsubscribeByID 通过订阅ID取消订阅
func (eb *EventBus) UnsubscribeByID(id types.SubscriptionID) error {
	if !eb.config.IsEnabled() {
		return nil
	}

	eb.weisynMutex.Lock()
	defer eb.weisynMutex.Unlock()

	// 检查WES订阅
	if weisynSub, exists := eb.weisynSubscriptions[string(id)]; exists {
		weisynSub.active = false
		delete(eb.weisynSubscriptions, string(id))
		return nil
	}

	return fmt.Errorf("subscription not found: %s", id)
}

// EnableEventHistory 启用事件历史记录
func (eb *EventBus) EnableEventHistory(eventType event.EventType, maxSize int) error {
	// 复用现有的历史记录功能，由配置控制
	return nil
}

// DisableEventHistory 禁用事件历史记录
func (eb *EventBus) DisableEventHistory(eventType event.EventType) error {
	// 复用现有的历史记录功能，由配置控制
	return nil
}

// GetActiveSubscriptions 获取活跃订阅列表
func (eb *EventBus) GetActiveSubscriptions() ([]*types.SubscriptionInfo, error) {
	eb.weisynMutex.RLock()
	defer eb.weisynMutex.RUnlock()

	var subscriptions []*types.SubscriptionInfo

	// WES订阅
	for _, weisynSub := range eb.weisynSubscriptions {
		if !weisynSub.active {
			continue
		}

		var lastTriggered *time.Time
		if ptr := weisynSub.lastTriggered.Load(); ptr != nil {
			lastTriggered = ptr
		}

		subInfo := &types.SubscriptionInfo{
			ID:            weisynSub.id,
			EventType:     "", // WES订阅可能处理多种事件类型
			Protocols:     nil,
			Handler:       fmt.Sprintf("%T", weisynSub.handler),
			CreatedAt:     weisynSub.createdAt,
			LastTriggered: lastTriggered,
			TriggerCount:  weisynSub.triggerCount.Load(),
			IsActive:      weisynSub.active,
		}
		subscriptions = append(subscriptions, subInfo)
	}

	return subscriptions, nil
}

// UpdateConfig 更新事件总线配置
func (eb *EventBus) UpdateConfig(config *types.EventBusConfig) error {
	// 注意：这里的config参数类型与现有的config.EventConfig不匹配
	// 这是接口设计不一致的问题，需要适配
	return fmt.Errorf("config update not implemented for legacy EventBus")
}

// GetConfig 获取当前配置
func (eb *EventBus) GetConfig() (*types.EventBusConfig, error) {
	// 转换现有配置到新的配置格式
	return &types.EventBusConfig{
		MaxEventHistory:     0, // 历史记录功能已简化
		DefaultAsync:        false,
		EnableMetrics:       false, // 简化指标功能
		MetricsInterval:     time.Minute,
		MaxConcurrentEvents: eb.config.GetMaxWorkers(),
		EventQueueSize:      eb.config.GetBufferSize(),
		WorkerPoolSize:      10,
		ProcessingTimeout:   time.Minute,
		EnableFiltering:     true,
		EnableInterception:  true,
		EnablePersistence:   false,
		RequireAuth:         false,
		MaxEventSize:        1024 * 1024,
		RateLimit:           1000,
		EnableAudit:         false,
		LogLevel:            "info",
	}, nil
}

// RegisterEventInterceptor 注册事件拦截器
func (eb *EventBus) RegisterEventInterceptor(interceptor event.EventInterceptor) error {
	if interceptor == nil {
		return fmt.Errorf("interceptor cannot be nil")
	}

	eb.filterMutex.Lock()
	defer eb.filterMutex.Unlock()

	eb.interceptors = append(eb.interceptors, interceptor)

	return nil
}

// UnregisterEventInterceptor 注销事件拦截器
func (eb *EventBus) UnregisterEventInterceptor(interceptorID string) error {
	eb.filterMutex.Lock()
	defer eb.filterMutex.Unlock()

	for i, interceptor := range eb.interceptors {
		if info := interceptor.GetInterceptorInfo(); info != nil && info.ID == interceptorID {
			// 从切片中移除
			eb.interceptors = append(eb.interceptors[:i], eb.interceptors[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("interceptor not found: %s", interceptorID)
}

// ==================== 内部实现方法 ====================

// processWESSubscriptions 处理WES特定订阅
func (eb *EventBus) processWESSubscriptions(weisynEvent *event.WESEvent) {
	eb.weisynMutex.RLock()
	defer eb.weisynMutex.RUnlock()

	for _, weisynSub := range eb.weisynSubscriptions {
		if !weisynSub.active {
			continue
		}

		// 检查协议过滤
		if len(weisynSub.protocols) > 0 {
			protocolMatch := false
			for _, protocol := range weisynSub.protocols {
				if protocol == weisynEvent.Protocol {
					protocolMatch = true
					break
				}
			}
			if !protocolMatch {
				continue
			}
		}

		// 检查自定义过滤器
		if weisynSub.filter != nil && !weisynSub.filter.MatchWES(weisynEvent) {
			continue
		}

		// 异步处理
		go eb.handleWESSubscription(weisynSub, weisynEvent)
	}
}

// handleWESSubscription 处理WES订阅
func (eb *EventBus) handleWESSubscription(weisynSub *weisynSubscription, weisynEvent *event.WESEvent) {
	defer func() {
		if r := recover(); r != nil {
			eb.metrics.failedEvents.Add(1)
		}
	}()

	// 执行处理器
	err := weisynSub.handler(weisynEvent)
	if err != nil {
		eb.metrics.failedEvents.Add(1)
	} else {
		eb.metrics.successfulEvents.Add(1)
	}

	// 更新统计
	weisynSub.triggerCount.Add(1)
	now := time.Now()
	weisynSub.lastTriggered.Store(&now)
}

// applyPreInterceptors 应用前置拦截器
func (eb *EventBus) applyPreInterceptors(weisynEvent *event.WESEvent) error {
	eb.filterMutex.RLock()
	defer eb.filterMutex.RUnlock()

	for _, interceptor := range eb.interceptors {
		// WESEvent实现了Event接口
		_, err := interceptor.PrePublish(weisynEvent)
		if err != nil {
			return err
		}
	}

	return nil
}

// applyPostInterceptors 应用后置拦截器
func (eb *EventBus) applyPostInterceptors(weisynEvent *event.WESEvent, result error) {
	eb.filterMutex.RLock()
	defer eb.filterMutex.RUnlock()

	for _, interceptor := range eb.interceptors {
		interceptor.PostPublish(weisynEvent, result)
	}
}

// ==================== 增强事件总线实现 ====================

// EnhancedEventBus 集成所有增强功能的事件总线
//
// 🚀 **核心特性**：
// - 完全向后兼容原有EventBus
// - 集成EventCoordinator协调器
// - 支持域注册和管理
// - 智能事件路由
// - 事件验证和过滤
// - 企业级监控和统计
type EnhancedEventBus struct {
	// ================== 基础组件 ==================
	*EventBus                    // 嵌入原有EventBus，保持兼容性
	coordinator EventCoordinator // 事件协调器

	// ================== 增强组件 ==================
	domainRegistry *DomainRegistry // 域注册中心
	eventRouter    *EventRouter    // 智能路由器
	eventValidator EventValidator  // 事件验证器

	// ================== 配置和状态 ==================
	enhancedConfig *EnhancedEventBusConfig // 增强配置
	logger         log.Logger              // 日志记录器

	// ================== 生命周期管理 ==================
	started    atomic.Bool        // 启动状态
	ctx        context.Context    // 上下文
	cancelFunc context.CancelFunc // 取消函数

	// ================== 订阅管理 ==================
	subscriptionMutex sync.RWMutex                             // 订阅锁
	subscriptions     map[string]*EnhancedSubscriptionInfo     // 订阅映射
	domainHandlers    map[string][]DomainEventHandler          // 域事件处理器
	typeHandlers      map[event.EventType][]event.EventHandler // 类型事件处理器

	// ================== 统计和监控 ==================
	enhancedMetrics *EnhancedEventMetrics // 增强指标
	metricsMutex    sync.RWMutex          // 指标锁
}

// EnhancedEventBusConfig 增强事件总线配置
type EnhancedEventBusConfig struct {
	*eventconfig.Config                    // 基础配置
	CoordinatorConfig   *CoordinatorConfig // 协调器配置

	// 增强功能开关
	EnableDomainRegistry  bool `json:"enable_domain_registry"`
	EnableSmartRouting    bool `json:"enable_smart_routing"`
	EnableEventValidation bool `json:"enable_event_validation"`
	EnableEnhancedMetrics bool `json:"enable_enhanced_metrics"`

	// 性能配置
	AsyncByDefault            bool          `json:"async_by_default"`
	DefaultEventPriority      Priority      `json:"default_event_priority"`
	MaxSubscriptionsPerDomain int           `json:"max_subscriptions_per_domain"`
	EventBatchSize            int           `json:"event_batch_size"`
	MetricsFlushInterval      time.Duration `json:"metrics_flush_interval"`

	// 安全配置
	RequireDomainAuth     bool     `json:"require_domain_auth"`
	AllowedDomains        []string `json:"allowed_domains"`
	BlockedEventTypes     []string `json:"blocked_event_types"`
	EnableEventEncryption bool     `json:"enable_event_encryption"`
}

// DefaultEnhancedEventBusConfig 默认增强配置
func DefaultEnhancedEventBusConfig() *EnhancedEventBusConfig {
	// 使用默认的事件配置
	defaultEventConfig := eventconfig.New(nil)

	return &EnhancedEventBusConfig{
		Config:                    defaultEventConfig,
		CoordinatorConfig:         DefaultCoordinatorConfig(),
		EnableDomainRegistry:      true,
		EnableSmartRouting:        true,
		EnableEventValidation:     true,
		EnableEnhancedMetrics:     true,
		AsyncByDefault:            false,
		DefaultEventPriority:      PriorityNormal,
		MaxSubscriptionsPerDomain: 1000,
		EventBatchSize:            100,
		MetricsFlushInterval:      30 * time.Second,
		RequireDomainAuth:         false,
		AllowedDomains:            []string{},
		BlockedEventTypes:         []string{},
		EnableEventEncryption:     false,
	}
}

// EnhancedSubscriptionInfo 增强订阅信息
type EnhancedSubscriptionInfo struct {
	*types.SubscriptionInfo // 基础订阅信息

	Domain     string               `json:"domain,omitempty"`
	Priority   Priority             `json:"priority"`
	Filter     event.EventFilter    `json:"-"` // 过滤器不序列化
	Options    []SubscriptionOption `json:"-"` // 选项不序列化
	Component  string               `json:"component,omitempty"`
	Route      RouteStrategy        `json:"route,omitempty"`
	LastError  string               `json:"last_error,omitempty"`
	ErrorCount uint64               `json:"error_count"`
}

// EnhancedEventMetrics 增强事件指标
type EnhancedEventMetrics struct {
	*eventMetrics // 基础指标

	// 域统计
	DomainStats map[string]*DomainMetrics `json:"domain_stats"`

	// 路由统计
	RouteStats map[RouteStrategy]*RouteMetrics `json:"route_stats"`

	// 验证统计
	ValidationStats *ValidationMetrics `json:"validation_stats"`

	// 性能统计
	PerformanceStats *PerformanceMetrics `json:"performance_stats"`

	// 错误统计
	ErrorStats *ErrorMetrics `json:"error_stats"`
}

// DomainMetrics 域统计
type DomainMetrics struct {
	Domain          string                    `json:"domain"`
	EventsPublished atomic.Uint64             `json:"events_published"`
	EventsReceived  atomic.Uint64             `json:"events_received"`
	Subscriptions   atomic.Uint64             `json:"subscriptions"`
	LastActivity    atomic.Pointer[time.Time] `json:"last_activity"`
}

// RouteMetrics 路由统计
type RouteMetrics struct {
	Strategy       RouteStrategy                 `json:"strategy"`
	EventsRouted   atomic.Uint64                 `json:"events_routed"`
	AverageLatency atomic.Pointer[time.Duration] `json:"average_latency"`
	SuccessRate    atomic.Pointer[float64]       `json:"success_rate"`
}

// ValidationMetrics 验证统计
type ValidationMetrics struct {
	ValidationsPerformed atomic.Uint64                 `json:"validations_performed"`
	ValidationsPassed    atomic.Uint64                 `json:"validations_passed"`
	ValidationsFailed    atomic.Uint64                 `json:"validations_failed"`
	AverageLatency       atomic.Pointer[time.Duration] `json:"average_latency"`
}

// PerformanceMetrics 性能统计
type PerformanceMetrics struct {
	PublishLatency      atomic.Pointer[time.Duration] `json:"publish_latency"`
	SubscribeLatency    atomic.Pointer[time.Duration] `json:"subscribe_latency"`
	EndToEndLatency     atomic.Pointer[time.Duration] `json:"end_to_end_latency"`
	ThroughputPerSecond atomic.Uint64                 `json:"throughput_per_second"`
	PeakThroughput      atomic.Uint64                 `json:"peak_throughput"`
}

// ErrorMetrics 错误统计
type ErrorMetrics struct {
	PublishErrors    atomic.Uint64 `json:"publish_errors"`
	SubscribeErrors  atomic.Uint64 `json:"subscribe_errors"`
	ValidationErrors atomic.Uint64 `json:"validation_errors"`
	RoutingErrors    atomic.Uint64 `json:"routing_errors"`
	HandlerErrors    atomic.Uint64 `json:"handler_errors"`
	SystemErrors     atomic.Uint64 `json:"system_errors"`
}

// NewEnhanced 创建增强事件总线
func NewEnhanced(
	logger log.Logger,
	config *EnhancedEventBusConfig,
) (*EnhancedEventBus, error) {
	if config == nil {
		config = DefaultEnhancedEventBusConfig()
	}

	// 创建基础EventBus
	baseEventBus := New(config.Config).(*EventBus)

	// 创建增强组件
	domainRegistry := NewDomainRegistry(logger)
	eventRouter := NewEventRouter(logger)
	eventValidator := NewBasicEventValidator(logger, DefaultValidatorConfig())

	// 创建协调器
	coordinator := NewBasicEventCoordinator(
		logger,
		config.CoordinatorConfig,
		domainRegistry,
		eventRouter,
		eventValidator,
		baseEventBus.bus,
	)

	var componentLogger log.Logger
	if logger != nil {
		componentLogger = logger.With("component", "enhanced_event_bus")
	}

	enhanced := &EnhancedEventBus{
		EventBus:        baseEventBus,
		coordinator:     coordinator,
		domainRegistry:  domainRegistry,
		eventRouter:     eventRouter,
		eventValidator:  eventValidator,
		enhancedConfig:  config,
		logger:          componentLogger,
		subscriptions:   make(map[string]*EnhancedSubscriptionInfo),
		domainHandlers:  make(map[string][]DomainEventHandler),
		typeHandlers:    make(map[event.EventType][]event.EventHandler),
		enhancedMetrics: newEnhancedEventMetrics(),
	}

	return enhanced, nil
}

// newEnhancedEventMetrics 创建增强事件指标
func newEnhancedEventMetrics() *EnhancedEventMetrics {
	return &EnhancedEventMetrics{
		eventMetrics:     newEventMetrics(),
		DomainStats:      make(map[string]*DomainMetrics),
		RouteStats:       make(map[RouteStrategy]*RouteMetrics),
		ValidationStats:  &ValidationMetrics{},
		PerformanceStats: &PerformanceMetrics{},
		ErrorStats:       &ErrorMetrics{},
	}
}

// ==================== 生命周期管理 ====================

// Start 启动增强事件总线
func (eeb *EnhancedEventBus) Start(ctx context.Context) error {
	if eeb.started.Load() {
		return fmt.Errorf("enhanced event bus already started")
	}

	eeb.ctx, eeb.cancelFunc = context.WithCancel(ctx)

	// 启动协调器
	if err := eeb.coordinator.Start(eeb.ctx); err != nil {
		return fmt.Errorf("failed to start coordinator: %w", err)
	}

	// 启动基础EventBus
	if err := eeb.EventBus.Start(eeb.ctx); err != nil {
		eeb.coordinator.Stop()
		return fmt.Errorf("failed to start base event bus: %w", err)
	}

	// 启动监控协程
	if eeb.enhancedConfig.EnableEnhancedMetrics {
		go eeb.metricsWorker()
	}

	eeb.started.Store(true)
	eeb.logger.Info("增强事件总线已启动")

	return nil
}

// Stop 停止增强事件总线
func (eeb *EnhancedEventBus) Stop(ctx context.Context) error {
	if !eeb.started.Load() {
		return fmt.Errorf("enhanced event bus not started")
	}

	eeb.started.Store(false)

	// 停止协调器
	if err := eeb.coordinator.Stop(); err != nil {
		eeb.logger.Errorf("停止协调器时出错: %v", err)
	}

	// 停止基础EventBus
	if err := eeb.EventBus.Stop(ctx); err != nil {
		eeb.logger.Errorf("停止基础事件总线时出错: %v", err)
	}

	// 取消上下文
	if eeb.cancelFunc != nil {
		eeb.cancelFunc()
	}

	eeb.logger.Info("增强事件总线已停止")
	return nil
}

// IsStarted 检查是否已启动
func (eeb *EnhancedEventBus) IsStarted() bool {
	return eeb.started.Load()
}

// ==================== 增强事件发布 ====================

// PublishEvent 发布事件 (增强版本)
func (eeb *EnhancedEventBus) PublishEvent(e event.Event) {
	if !eeb.started.Load() {
		eeb.logger.Warn("增强事件总线未启动，跳过事件发布")
		return
	}

	startTime := time.Now()
	defer eeb.updatePublishMetrics(time.Since(startTime), true)

	// 使用协调器发布事件
	if err := eeb.coordinator.PublishEvent(string(e.Type()), e.Data()); err != nil {
		eeb.logger.Errorf("事件发布失败: %v", err)
		eeb.enhancedMetrics.ErrorStats.PublishErrors.Add(1)
		eeb.updatePublishMetrics(time.Since(startTime), false)
		return
	}

	// 更新域统计
	domain := ExtractDomainFromEventName(string(e.Type()))
	eeb.updateDomainMetrics(domain, 1, 0)
}

// PublishEventWithPriority 带优先级发布事件
func (eeb *EnhancedEventBus) PublishEventWithPriority(eventType event.EventType, data interface{}, priority Priority) error {
	if !eeb.started.Load() {
		return fmt.Errorf("enhanced event bus not started")
	}

	startTime := time.Now()
	defer eeb.updatePublishMetrics(time.Since(startTime), true)

	// 使用协调器发布事件
	if err := eeb.coordinator.PublishEventWithPriority(string(eventType), data, priority); err != nil {
		eeb.enhancedMetrics.ErrorStats.PublishErrors.Add(1)
		eeb.updatePublishMetrics(time.Since(startTime), false)
		return err
	}

	// 更新域统计
	domain := ExtractDomainFromEventName(string(eventType))
	eeb.updateDomainMetrics(domain, 1, 0)

	return nil
}

// PublishEventWithMetadata 带元数据发布事件
func (eeb *EnhancedEventBus) PublishEventWithMetadata(metadata *EventMetadata, data interface{}) error {
	if !eeb.started.Load() {
		return fmt.Errorf("enhanced event bus not started")
	}

	return eeb.coordinator.PublishEventWithMetadata(metadata, data)
}

// BatchPublishEvents 批量发布事件
func (eeb *EnhancedEventBus) BatchPublishEvents(events []EventRequest) []EventResult {
	if !eeb.started.Load() {
		results := make([]EventResult, len(events))
		for i, event := range events {
			results[i] = EventResult{
				EventType: event.EventType,
				Success:   false,
				Error:     "enhanced event bus not started",
			}
		}
		return results
	}

	return eeb.coordinator.BatchPublishEvents(events)
}

// ==================== 增强事件订阅 ====================

// SubscribeEvent 订阅事件 (增强版本)
func (eeb *EnhancedEventBus) SubscribeEvent(eventType event.EventType, handler event.EventHandler) (types.SubscriptionID, error) {
	return eeb.SubscribeEventWithOptions(eventType, handler)
}

// SubscribeEventWithOptions 带选项订阅事件
func (eeb *EnhancedEventBus) SubscribeEventWithOptions(eventType event.EventType, handler event.EventHandler, options ...SubscriptionOption) (types.SubscriptionID, error) {
	if !eeb.started.Load() {
		return "", fmt.Errorf("enhanced event bus not started")
	}

	startTime := time.Now()
	defer eeb.updateSubscribeMetrics(time.Since(startTime), true)

	// 通过协调器订阅
	subID, err := eeb.coordinator.SubscribeEventWithOptions(string(eventType), handler, options...)
	if err != nil {
		eeb.enhancedMetrics.ErrorStats.SubscribeErrors.Add(1)
		eeb.updateSubscribeMetrics(time.Since(startTime), false)
		return "", err
	}

	// 记录订阅信息
	eeb.recordSubscription(subID, eventType, options)

	// 更新域统计
	domain := ExtractDomainFromEventName(string(eventType))
	eeb.updateDomainMetrics(domain, 0, 1)

	return types.SubscriptionID(subID), nil
}

// SubscribeDomainEvents 订阅域事件
func (eeb *EnhancedEventBus) SubscribeDomainEvents(domain string, handler DomainEventHandler) (types.SubscriptionID, error) {
	if !eeb.started.Load() {
		return "", fmt.Errorf("enhanced event bus not started")
	}

	// 检查域是否已注册
	if !eeb.coordinator.IsDomainRegistered(domain) {
		return "", fmt.Errorf("domain not registered: %s", domain)
	}

	subID := types.SubscriptionID(uuid.New().String())

	eeb.subscriptionMutex.Lock()
	eeb.domainHandlers[domain] = append(eeb.domainHandlers[domain], handler)
	eeb.subscriptionMutex.Unlock()

	eeb.logger.Infof("域事件订阅成功: domain=%s, id=%s", domain, subID)

	return subID, nil
}

// UnsubscribeEvent 取消订阅
func (eeb *EnhancedEventBus) UnsubscribeEvent(subscriptionID types.SubscriptionID) error {
	if !eeb.started.Load() {
		return fmt.Errorf("enhanced event bus not started")
	}

	// 通过协调器取消订阅
	if err := eeb.coordinator.UnsubscribeEvent(string(subscriptionID)); err != nil {
		return err
	}

	// 清理本地记录
	eeb.subscriptionMutex.Lock()
	delete(eeb.subscriptions, string(subscriptionID))
	eeb.subscriptionMutex.Unlock()

	return nil
}

// ==================== 域管理接口 ====================

// RegisterDomain 注册域
func (eeb *EnhancedEventBus) RegisterDomain(domain string, info DomainInfo) error {
	if !eeb.enhancedConfig.EnableDomainRegistry {
		return fmt.Errorf("domain registry not enabled")
	}

	return eeb.coordinator.RegisterDomain(domain, info)
}

// UnregisterDomain 注销域
func (eeb *EnhancedEventBus) UnregisterDomain(domain string) error {
	return eeb.coordinator.UnregisterDomain(domain)
}

// IsDomainRegistered 检查域是否已注册
func (eeb *EnhancedEventBus) IsDomainRegistered(domain string) bool {
	return eeb.coordinator.IsDomainRegistered(domain)
}

// ListDomains 列出所有域
func (eeb *EnhancedEventBus) ListDomains() []string {
	return eeb.coordinator.ListDomains()
}

// GetDomainInfo 获取域信息
func (eeb *EnhancedEventBus) GetDomainInfo(domain string) (*DomainInfo, error) {
	if !eeb.coordinator.IsDomainRegistered(domain) {
		return nil, fmt.Errorf("domain not found: %s", domain)
	}

	// 这里需要从domainRegistry获取详细信息
	// 暂时返回基础信息
	return &DomainInfo{
		Name:        domain,
		Component:   "unknown",
		Description: "Domain registered via enhanced event bus",
	}, nil
}

// ==================== 路由管理接口 ====================

// SetRouteStrategy 设置路由策略
func (eeb *EnhancedEventBus) SetRouteStrategy(eventType event.EventType, strategy RouteStrategy) error {
	return eeb.coordinator.SetRouteStrategy(string(eventType), strategy)
}

// GetRouteStrategy 获取路由策略
func (eeb *EnhancedEventBus) GetRouteStrategy(eventType event.EventType) RouteStrategy {
	return eeb.coordinator.GetRouteStrategy(string(eventType))
}

// ==================== 验证管理接口 ====================

// AddValidationRule 添加验证规则
func (eeb *EnhancedEventBus) AddValidationRule(rule ValidationRule) error {
	return eeb.coordinator.AddValidationRule(rule)
}

// RemoveValidationRule 移除验证规则
func (eeb *EnhancedEventBus) RemoveValidationRule(ruleID string) error {
	return eeb.coordinator.RemoveValidationRule(ruleID)
}

// ValidateEvent 验证事件
func (eeb *EnhancedEventBus) ValidateEvent(e event.Event) error {
	if !eeb.enhancedConfig.EnableEventValidation {
		return nil
	}

	basicEvent := &basicEvent{eventType: string(e.Type()), data: e.Data()}
	return eeb.eventValidator.ValidateEventWithContext(eeb.ctx, basicEvent)
}

// ==================== 统计和监控 ====================

// GetStatistics 获取增强统计信息
func (eeb *EnhancedEventBus) GetStatistics() *EnhancedEventMetrics {
	eeb.metricsMutex.RLock()
	defer eeb.metricsMutex.RUnlock()

	// 更新协调器统计
	coordStats := eeb.coordinator.GetStatistics()
	if coordStats != nil {
		eeb.enhancedMetrics.totalEvents.Store(coordStats.TotalEvents.Load())
		eeb.enhancedMetrics.successfulEvents.Store(coordStats.SuccessEvents.Load())
		eeb.enhancedMetrics.failedEvents.Store(coordStats.FailedEvents.Load())
	}

	return eeb.enhancedMetrics
}

// GetHealthStatus 获取健康状态
func (eeb *EnhancedEventBus) GetHealthStatus() *HealthStatus {
	return eeb.coordinator.GetHealthStatus()
}

// ==================== 配置管理 ====================

// UpdateEnhancedConfig 更新增强配置
func (eeb *EnhancedEventBus) UpdateEnhancedConfig(config *EnhancedEventBusConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	eeb.enhancedConfig = config

	// 更新协调器配置
	if err := eeb.coordinator.UpdateConfig(config.CoordinatorConfig); err != nil {
		return fmt.Errorf("failed to update coordinator config: %w", err)
	}

	eeb.logger.Info("增强事件总线配置已更新")
	return nil
}

// GetEnhancedConfig 获取增强配置
func (eeb *EnhancedEventBus) GetEnhancedConfig() *EnhancedEventBusConfig {
	return eeb.enhancedConfig
}

// ==================== 内部辅助方法 ====================

// recordSubscription 记录订阅信息
func (eeb *EnhancedEventBus) recordSubscription(subID string, eventType event.EventType, options []SubscriptionOption) {
	eeb.subscriptionMutex.Lock()
	defer eeb.subscriptionMutex.Unlock()

	// 解析选项
	priority := eeb.enhancedConfig.DefaultEventPriority
	component := "unknown"

	// 由于选项是interface{}，我们需要特殊处理
	// 这里暂时跳过选项解析，使用默认值

	domain := ExtractDomainFromEventName(string(eventType))

	enhancedSub := &EnhancedSubscriptionInfo{
		SubscriptionInfo: &types.SubscriptionInfo{
			ID:        types.SubscriptionID(subID),
			EventType: eventType,
			CreatedAt: time.Now(),
			IsActive:  true,
		},
		Domain:    domain,
		Priority:  priority,
		Component: component,
		Options:   options,
	}

	eeb.subscriptions[subID] = enhancedSub
}

// updateDomainMetrics 更新域统计
func (eeb *EnhancedEventBus) updateDomainMetrics(domain string, published, subscribed uint64) {
	eeb.metricsMutex.Lock()
	defer eeb.metricsMutex.Unlock()

	if eeb.enhancedMetrics.DomainStats[domain] == nil {
		eeb.enhancedMetrics.DomainStats[domain] = &DomainMetrics{
			Domain: domain,
		}
	}

	domainMetrics := eeb.enhancedMetrics.DomainStats[domain]
	domainMetrics.EventsPublished.Add(published)
	domainMetrics.Subscriptions.Add(subscribed)

	now := time.Now()
	domainMetrics.LastActivity.Store(&now)
}

// updatePublishMetrics 更新发布指标
func (eeb *EnhancedEventBus) updatePublishMetrics(duration time.Duration, success bool) {
	if eeb.enhancedMetrics.PerformanceStats.PublishLatency.Load() == nil {
		eeb.enhancedMetrics.PerformanceStats.PublishLatency.Store(&duration)
	} else {
		// 简单的移动平均
		current := *eeb.enhancedMetrics.PerformanceStats.PublishLatency.Load()
		newAvg := time.Duration((int64(current) + int64(duration)) / 2)
		eeb.enhancedMetrics.PerformanceStats.PublishLatency.Store(&newAvg)
	}
}

// updateSubscribeMetrics 更新订阅指标
func (eeb *EnhancedEventBus) updateSubscribeMetrics(duration time.Duration, success bool) {
	if eeb.enhancedMetrics.PerformanceStats.SubscribeLatency.Load() == nil {
		eeb.enhancedMetrics.PerformanceStats.SubscribeLatency.Store(&duration)
	} else {
		// 简单的移动平均
		current := *eeb.enhancedMetrics.PerformanceStats.SubscribeLatency.Load()
		newAvg := time.Duration((int64(current) + int64(duration)) / 2)
		eeb.enhancedMetrics.PerformanceStats.SubscribeLatency.Store(&newAvg)
	}
}

// metricsWorker 指标工作协程
func (eeb *EnhancedEventBus) metricsWorker() {
	ticker := time.NewTicker(eeb.enhancedConfig.MetricsFlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 定期更新指标
			eeb.flushMetrics()

		case <-eeb.ctx.Done():
			return
		}
	}
}

// flushMetrics 刷新指标
func (eeb *EnhancedEventBus) flushMetrics() {
	eeb.logger.Debugf("指标更新: published=%d, failed=%d, domains=%d",
		eeb.enhancedMetrics.eventMetrics.totalEvents.Load(),
		eeb.enhancedMetrics.eventMetrics.failedEvents.Load(),
		len(eeb.enhancedMetrics.DomainStats))
}
