package event

import "time"

// EventOptions 事件系统配置选项
// 专注于基础设施核心功能的简化配置
type EventOptions struct {
	// === 基础配置 ===
	Enabled    bool `json:"enabled"`     // 是否启用事件系统
	BufferSize int  `json:"buffer_size"` // 事件缓冲区大小
	MaxWorkers int  `json:"max_workers"` // 最大工作者数量

	// === 基础限制 ===
	MaxSubscribers int `json:"max_subscribers"` // 最大订阅者数量

	// === 增强功能配置 ===
	Enhanced *EnhancedEventOptions `json:"enhanced,omitempty"` // 增强功能配置（可选）
}

// EnhancedEventOptions 增强事件系统配置选项
type EnhancedEventOptions struct {
	// === 总开关 ===
	Enabled bool `json:"enabled"` // 是否启用增强功能

	// === 域注册中心配置 ===
	DomainRegistry *DomainRegistryOptions `json:"domain_registry,omitempty"`

	// === 事件路由器配置 ===
	EventRouter *EventRouterOptions `json:"event_router,omitempty"`

	// === 事件验证器配置 ===
	EventValidator *EventValidatorOptions `json:"event_validator,omitempty"`

	// === 事件协调器配置 ===
	EventCoordinator *EventCoordinatorOptions `json:"event_coordinator,omitempty"`
}

// DomainRegistryOptions 域注册中心配置选项
type DomainRegistryOptions struct {
	Enabled                 bool          `json:"enabled"`                   // 是否启用域注册
	StrictDomainCheck       bool          `json:"strict_domain_check"`       // 严格域检查
	WarnCrossDomain         bool          `json:"warn_cross_domain"`         // 跨域警告
	AllowUnregisteredDomain bool          `json:"allow_unregistered_domain"` // 允许未注册域
	MaxDomains              int           `json:"max_domains"`               // 最大域数量
	DefaultTTL              time.Duration `json:"default_ttl"`               // 域默认TTL
}

// EventRouterOptions 事件路由器配置选项
type EventRouterOptions struct {
	Enabled             bool          `json:"enabled"`               // 是否启用智能路由
	DefaultStrategy     string        `json:"default_strategy"`      // 默认路由策略
	MaxConcurrentRoutes int           `json:"max_concurrent_routes"` // 最大并发路由数
	RouteTimeout        time.Duration `json:"route_timeout"`         // 路由超时时间
	EnablePriorityQueue bool          `json:"enable_priority_queue"` // 启用优先级队列
	MaxQueueSize        int           `json:"max_queue_size"`        // 最大队列大小
	WorkerPoolSize      int           `json:"worker_pool_size"`      // 工作池大小
	EnableMetrics       bool          `json:"enable_metrics"`        // 启用指标收集
}

// EventValidatorOptions 事件验证器配置选项
type EventValidatorOptions struct {
	Enabled                bool          `json:"enabled"`                  // 是否启用事件验证
	StrictMode             bool          `json:"strict_mode"`              // 严格验证模式
	ValidateEventName      bool          `json:"validate_event_name"`      // 验证事件名称
	ValidateEventData      bool          `json:"validate_event_data"`      // 验证事件数据
	ValidationTimeout      time.Duration `json:"validation_timeout"`       // 验证超时时间
	MaxValidationRules     int           `json:"max_validation_rules"`     // 最大验证规则数
	EnableBatchValidation  bool          `json:"enable_batch_validation"`  // 启用批量验证
	CacheValidationResults bool          `json:"cache_validation_results"` // 缓存验证结果
}

// EventCoordinatorOptions 事件协调器配置选项
type EventCoordinatorOptions struct {
	Enabled                 bool          `json:"enabled"`                   // 是否启用事件协调器
	MaxConcurrentEvents     int           `json:"max_concurrent_events"`     // 最大并发事件数
	EventTimeout            time.Duration `json:"event_timeout"`             // 事件处理超时
	HealthCheckInterval     time.Duration `json:"health_check_interval"`     // 健康检查间隔
	MetricsInterval         time.Duration `json:"metrics_interval"`          // 指标收集间隔
	EnableCircuitBreaker    bool          `json:"enable_circuit_breaker"`    // 启用熔断器
	CircuitBreakerThreshold int           `json:"circuit_breaker_threshold"` // 熔断器阈值
	EnableGracefulShutdown  bool          `json:"enable_graceful_shutdown"`  // 启用优雅关闭
	GracefulShutdownTimeout time.Duration `json:"graceful_shutdown_timeout"` // 优雅关闭超时
}

// Config 事件配置实现
type Config struct {
	options *EventOptions
}

// New 创建事件配置实现
func New(userConfig interface{}) *Config {
	// 1. 先创建完整的默认配置
	defaultOptions := createDefaultEventOptions()

	// 2. 暂时不处理用户配置，后续添加
	// TODO: 当有用户配置类型时，在这里进行转换和合并

	return &Config{
		options: defaultOptions,
	}
}

// createDefaultEventOptions 创建默认事件配置
func createDefaultEventOptions() *EventOptions {
	return &EventOptions{
		// 基础配置
		Enabled:    defaultEnabled,
		BufferSize: defaultBufferSize,
		MaxWorkers: defaultMaxWorkers,

		// 基础限制
		MaxSubscribers: defaultMaxSubscribers,

		// 增强功能配置
		Enhanced: createDefaultEnhancedEventOptions(),
	}
}

// createDefaultEnhancedEventOptions 创建默认增强事件配置
func createDefaultEnhancedEventOptions() *EnhancedEventOptions {
	return &EnhancedEventOptions{
		Enabled:          defaultEnhancedEnabled,
		DomainRegistry:   createDefaultDomainRegistryOptions(),
		EventRouter:      createDefaultEventRouterOptions(),
		EventValidator:   createDefaultEventValidatorOptions(),
		EventCoordinator: createDefaultEventCoordinatorOptions(),
	}
}

// createDefaultDomainRegistryOptions 创建默认域注册中心配置
func createDefaultDomainRegistryOptions() *DomainRegistryOptions {
	return &DomainRegistryOptions{
		Enabled:                 defaultDomainRegistryEnabled,
		StrictDomainCheck:       defaultStrictDomainCheck,
		WarnCrossDomain:         defaultWarnCrossDomain,
		AllowUnregisteredDomain: defaultAllowUnregisteredDomain,
		MaxDomains:              defaultMaxDomains,
		DefaultTTL:              defaultDomainTTL,
	}
}

// createDefaultEventRouterOptions 创建默认事件路由器配置
func createDefaultEventRouterOptions() *EventRouterOptions {
	return &EventRouterOptions{
		Enabled:             defaultEventRouterEnabled,
		DefaultStrategy:     defaultRouteStrategy,
		MaxConcurrentRoutes: defaultMaxConcurrentRoutes,
		RouteTimeout:        defaultRouteTimeout,
		EnablePriorityQueue: defaultEnablePriorityQueue,
		MaxQueueSize:        defaultMaxQueueSize,
		WorkerPoolSize:      defaultRouterWorkerPoolSize,
		EnableMetrics:       defaultEnableRouterMetrics,
	}
}

// createDefaultEventValidatorOptions 创建默认事件验证器配置
func createDefaultEventValidatorOptions() *EventValidatorOptions {
	return &EventValidatorOptions{
		Enabled:                defaultEventValidatorEnabled,
		StrictMode:             defaultValidatorStrictMode,
		ValidateEventName:      defaultValidateEventName,
		ValidateEventData:      defaultValidateEventData,
		ValidationTimeout:      defaultValidationTimeout,
		MaxValidationRules:     defaultMaxValidationRules,
		EnableBatchValidation:  defaultEnableBatchValidation,
		CacheValidationResults: defaultCacheValidationResults,
	}
}

// createDefaultEventCoordinatorOptions 创建默认事件协调器配置
func createDefaultEventCoordinatorOptions() *EventCoordinatorOptions {
	return &EventCoordinatorOptions{
		Enabled:                 defaultEventCoordinatorEnabled,
		MaxConcurrentEvents:     defaultMaxConcurrentEvents,
		EventTimeout:            defaultEventTimeout,
		HealthCheckInterval:     defaultHealthCheckInterval,
		MetricsInterval:         defaultMetricsInterval,
		EnableCircuitBreaker:    defaultEnableCircuitBreaker,
		CircuitBreakerThreshold: defaultCircuitBreakerThreshold,
		EnableGracefulShutdown:  defaultEnableGracefulShutdown,
		GracefulShutdownTimeout: defaultGracefulShutdownTimeout,
	}
}

// GetOptions 获取完整的事件配置选项
func (c *Config) GetOptions() *EventOptions {
	return c.options
}

// === 基础配置访问方法 ===

// IsEnabled 是否启用事件系统
func (c *Config) IsEnabled() bool {
	return c.options.Enabled
}

// GetBufferSize 获取事件缓冲区大小
func (c *Config) GetBufferSize() int {
	return c.options.BufferSize
}

// GetMaxWorkers 获取最大工作者数量
func (c *Config) GetMaxWorkers() int {
	return c.options.MaxWorkers
}

// GetMaxSubscribers 获取最大订阅者数量
func (c *Config) GetMaxSubscribers() int {
	return c.options.MaxSubscribers
}

// === 增强配置访问方法 ===

// GetEnhancedOptions 获取增强功能配置选项
func (c *Config) GetEnhancedOptions() *EnhancedEventOptions {
	return c.options.Enhanced
}

// IsEnhancedEnabled 是否启用增强功能
func (c *Config) IsEnhancedEnabled() bool {
	if c.options.Enhanced == nil {
		return false
	}
	return c.options.Enhanced.Enabled
}

// === 域注册中心配置访问方法 ===

// GetDomainRegistryOptions 获取域注册中心配置
func (c *Config) GetDomainRegistryOptions() *DomainRegistryOptions {
	if c.options.Enhanced == nil {
		return nil
	}
	return c.options.Enhanced.DomainRegistry
}

// IsDomainRegistryEnabled 是否启用域注册中心
func (c *Config) IsDomainRegistryEnabled() bool {
	if c.options.Enhanced == nil || c.options.Enhanced.DomainRegistry == nil {
		return false
	}
	return c.options.Enhanced.DomainRegistry.Enabled
}

// === 事件路由器配置访问方法 ===

// GetEventRouterOptions 获取事件路由器配置
func (c *Config) GetEventRouterOptions() *EventRouterOptions {
	if c.options.Enhanced == nil {
		return nil
	}
	return c.options.Enhanced.EventRouter
}

// IsEventRouterEnabled 是否启用事件路由器
func (c *Config) IsEventRouterEnabled() bool {
	if c.options.Enhanced == nil || c.options.Enhanced.EventRouter == nil {
		return false
	}
	return c.options.Enhanced.EventRouter.Enabled
}

// === 事件验证器配置访问方法 ===

// GetEventValidatorOptions 获取事件验证器配置
func (c *Config) GetEventValidatorOptions() *EventValidatorOptions {
	if c.options.Enhanced == nil {
		return nil
	}
	return c.options.Enhanced.EventValidator
}

// IsEventValidatorEnabled 是否启用事件验证器
func (c *Config) IsEventValidatorEnabled() bool {
	if c.options.Enhanced == nil || c.options.Enhanced.EventValidator == nil {
		return false
	}
	return c.options.Enhanced.EventValidator.Enabled
}

// === 事件协调器配置访问方法 ===

// GetEventCoordinatorOptions 获取事件协调器配置
func (c *Config) GetEventCoordinatorOptions() *EventCoordinatorOptions {
	if c.options.Enhanced == nil {
		return nil
	}
	return c.options.Enhanced.EventCoordinator
}

// IsEventCoordinatorEnabled 是否启用事件协调器
func (c *Config) IsEventCoordinatorEnabled() bool {
	if c.options.Enhanced == nil || c.options.Enhanced.EventCoordinator == nil {
		return false
	}
	return c.options.Enhanced.EventCoordinator.Enabled
}
