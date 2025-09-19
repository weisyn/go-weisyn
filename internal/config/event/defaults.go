package event

import "time"

// 事件系统默认配置值
// 这些默认值基于事件驱动系统的最佳实践和性能考虑
const (
	// === 基础事件配置 ===

	// defaultEnabled 默认启用事件系统
	// 原因：事件系统是区块链的核心组件，用于通知各模块状态变化
	// 几乎所有区块链操作都需要事件通知，默认启用保证系统正常运行
	defaultEnabled = true

	// defaultBufferSize 默认事件缓冲区大小设为1000
	// 原因：1000个事件的缓冲区能处理大多数突发事件场景
	// 平衡内存使用和事件处理能力，避免因缓冲区满而丢失事件
	defaultBufferSize = 1000

	// defaultMaxWorkers 默认事件处理工作者数量设为10
	// 原因：10个工作者能够并行处理多个事件，提高系统响应性
	// 避免单一工作者成为瓶颈，同时控制资源消耗
	defaultMaxWorkers = 10

	// defaultMaxSubscribers 默认最大订阅者数量设为1000
	// 原因：1000个订阅者能满足大多数区块链系统的需求
	// 限制订阅者数量避免事件分发成为性能瓶颈
	defaultMaxSubscribers = 1000

	// === 增强功能总开关 ===

	// defaultEnhancedEnabled 默认启用增强功能
	// 原因：增强功能提供更好的事件管理、路由和验证能力
	// 有助于提升系统的可观测性、可靠性和性能
	defaultEnhancedEnabled = true

	// === 域注册中心默认配置 ===

	// defaultDomainRegistryEnabled 默认启用域注册中心
	// 原因：域注册有助于事件组织和权限控制，提高系统的结构化程度
	defaultDomainRegistryEnabled = true

	// defaultStrictDomainCheck 默认关闭严格域检查
	// 原因：在开发和测试阶段，严格检查可能阻碍开发，生产环境可单独开启
	defaultStrictDomainCheck = false

	// defaultWarnCrossDomain 默认开启跨域警告
	// 原因：跨域事件可能表示架构问题，警告有助于及时发现和解决
	defaultWarnCrossDomain = true

	// defaultAllowUnregisteredDomain 默认允许未注册域
	// 原因：保持向后兼容，避免破坏现有系统，可以后续逐步规范化
	defaultAllowUnregisteredDomain = true

	// defaultMaxDomains 默认最大域数量设为100
	// 原因：100个域足够覆盖大多数系统的组件划分，避免域过多导致管理复杂
	defaultMaxDomains = 100

	// defaultDomainTTL 默认域TTL设为24小时
	// 原因：24小时足够长，避免频繁重新注册，同时允许定期清理不活跃的域
	defaultDomainTTL = 24 * time.Hour

	// === 事件路由器默认配置 ===

	// defaultEventRouterEnabled 默认启用事件路由器
	// 原因：智能路由提高事件分发效率，支持负载均衡和优先级处理
	defaultEventRouterEnabled = true

	// defaultRouteStrategy 默认路由策略为广播
	// 原因：广播策略最安全，确保所有订阅者都能收到事件，避免消息丢失
	defaultRouteStrategy = "broadcast"

	// defaultMaxConcurrentRoutes 默认最大并发路由数设为50
	// 原因：50个并发路由平衡性能和资源消耗，适合中等规模的事件流量
	defaultMaxConcurrentRoutes = 50

	// defaultRouteTimeout 默认路由超时设为30秒
	// 原因：30秒足够处理大多数事件路由，避免长时间阻塞影响系统性能
	defaultRouteTimeout = 30 * time.Second

	// defaultEnablePriorityQueue 默认启用优先级队列
	// 原因：优先级队列确保关键事件优先处理，提高系统响应性
	defaultEnablePriorityQueue = true

	// defaultMaxQueueSize 默认最大队列大小设为10000
	// 原因：10000个事件的队列能缓冲大量突发事件，避免事件丢失
	defaultMaxQueueSize = 10000

	// defaultRouterWorkerPoolSize 默认路由器工作池大小设为20
	// 原因：20个工作者提供充足的并发处理能力，平衡性能和资源消耗
	defaultRouterWorkerPoolSize = 20

	// defaultEnableRouterMetrics 默认启用路由器指标收集
	// 原因：指标收集有助于监控路由性能，及时发现和解决问题
	defaultEnableRouterMetrics = true

	// === 事件验证器默认配置 ===

	// defaultEventValidatorEnabled 默认启用事件验证器
	// 原因：事件验证确保数据质量和系统稳定性，防止无效事件干扰系统
	defaultEventValidatorEnabled = true

	// defaultValidatorStrictMode 默认关闭严格验证模式
	// 原因：严格模式可能过于严苛，影响开发效率，可在生产环境按需开启
	defaultValidatorStrictMode = false

	// defaultValidateEventName 默认启用事件名称验证
	// 原因：事件名称验证确保命名规范，提高系统的可维护性
	defaultValidateEventName = true

	// defaultValidateEventData 默认启用事件数据验证
	// 原因：数据验证防止无效或恶意数据进入系统，确保数据质量
	defaultValidateEventData = true

	// defaultValidationTimeout 默认验证超时设为5秒
	// 原因：5秒足够完成大多数验证操作，避免验证成为性能瓶颈
	defaultValidationTimeout = 5 * time.Second

	// defaultMaxValidationRules 默认最大验证规则数设为50
	// 原因：50个规则能覆盖大多数验证场景，避免规则过多影响性能
	defaultMaxValidationRules = 50

	// defaultEnableBatchValidation 默认启用批量验证
	// 原因：批量验证提高处理效率，特别是在高吞吐量场景下
	defaultEnableBatchValidation = true

	// defaultCacheValidationResults 默认启用验证结果缓存
	// 原因：缓存验证结果避免重复计算，提高验证性能
	defaultCacheValidationResults = true

	// === 事件协调器默认配置 ===

	// defaultEventCoordinatorEnabled 默认启用事件协调器
	// 原因：协调器提供统一的事件管理，简化系统集成和管理
	defaultEventCoordinatorEnabled = true

	// defaultMaxConcurrentEvents 默认最大并发事件数设为100
	// 原因：100个并发事件提供充足的并发处理能力，适合高负载场景
	defaultMaxConcurrentEvents = 100

	// defaultEventTimeout 默认事件处理超时设为60秒
	// 原因：60秒足够处理复杂的事件处理逻辑，避免超时过短导致处理失败
	defaultEventTimeout = 60 * time.Second

	// defaultHealthCheckInterval 默认健康检查间隔设为30秒
	// 原因：30秒间隔平衡监控精度和系统开销，及时发现健康问题
	defaultHealthCheckInterval = 30 * time.Second

	// defaultMetricsInterval 默认指标收集间隔设为10秒
	// 原因：10秒间隔提供足够的指标精度，支持实时监控和分析
	defaultMetricsInterval = 10 * time.Second

	// defaultEnableCircuitBreaker 默认启用熔断器
	// 原因：熔断器防止系统故障扩散，提高系统的容错能力
	defaultEnableCircuitBreaker = true

	// defaultCircuitBreakerThreshold 默认熔断器阈值设为10
	// 原因：10次失败触发熔断合理，平衡容错性和可用性
	defaultCircuitBreakerThreshold = 10

	// defaultEnableGracefulShutdown 默认启用优雅关闭
	// 原因：优雅关闭确保正在处理的事件能够完成，避免数据丢失
	defaultEnableGracefulShutdown = true

	// defaultGracefulShutdownTimeout 默认优雅关闭超时设为30秒
	// 原因：30秒足够完成大多数正在处理的事件，避免关闭时间过长
	defaultGracefulShutdownTimeout = 30 * time.Second
)
