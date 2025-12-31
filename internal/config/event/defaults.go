// Package event provides default configuration values for event system.
package event

import "time"

// 事件系统配置默认值
const (
	// defaultEnabled 默认启用事件系统
	// 原因：事件系统是基础设施的核心组件
	defaultEnabled = true

	// defaultBufferSize 默认事件缓冲区大小设为1000
	// 原因：合理的缓冲区大小平衡内存使用和事件处理能力
	defaultBufferSize = 1000

	// defaultMaxWorkers 默认最大工作者数量设为10
	// 原因：10个工作者能处理大多数事件负载
	defaultMaxWorkers = 10

	// defaultMaxSubscribers 默认最大订阅者数量设为100
	// 原因：100个订阅者能满足大多数应用场景
	defaultMaxSubscribers = 100

	// defaultEnhancedEnabled 默认启用增强功能
	// 原因：增强功能提供更强大的事件处理能力
	defaultEnhancedEnabled = false

	// defaultDomainRegistryEnabled 默认启用域注册中心
	// 原因：域注册中心提供事件域管理功能
	defaultDomainRegistryEnabled = false

	// defaultStrictDomainCheck 默认严格域检查
	// 原因：严格检查确保事件域的正确性
	defaultStrictDomainCheck = false

	// defaultWarnCrossDomain 默认跨域警告
	// 原因：跨域警告帮助发现潜在问题
	defaultWarnCrossDomain = false

	// defaultAllowUnregisteredDomain 默认允许未注册域
	// 原因：允许未注册域提供灵活性
	defaultAllowUnregisteredDomain = true

	// defaultMaxDomains 默认最大域数量设为100
	// 原因：100个域能满足大多数应用场景
	defaultMaxDomains = 100

	// defaultDomainTTL 默认域TTL设为24小时
	// 原因：24小时的TTL平衡域管理的灵活性和资源使用
	defaultDomainTTL = 24 * time.Hour

	// defaultEventRouterEnabled 默认启用事件路由器
	// 原因：事件路由器提供智能路由功能
	defaultEventRouterEnabled = false

	// defaultRouteStrategy 默认路由策略设为"broadcast"
	// 原因：广播策略是最简单可靠的路由方式
	defaultRouteStrategy = "broadcast"

	// defaultMaxConcurrentRoutes 默认最大并发路由数设为10
	// 原因：10个并发路由能处理大多数路由负载
	defaultMaxConcurrentRoutes = 10

	// defaultRouteTimeout 默认路由超时时间设为5秒
	// 原因：5秒足够完成路由操作
	defaultRouteTimeout = 5 * time.Second

	// defaultEnablePriorityQueue 默认启用优先级队列
	// 原因：优先级队列提供更好的事件处理顺序
	defaultEnablePriorityQueue = false

	// defaultMaxQueueSize 默认最大队列大小设为1000
	// 原因：1000个事件的队列能满足大多数场景
	defaultMaxQueueSize = 1000

	// defaultRouterWorkerPoolSize 默认路由器工作池大小设为5
	// 原因：5个工作线程能处理大多数路由负载
	defaultRouterWorkerPoolSize = 5

	// defaultEnableRouterMetrics 默认启用路由器指标收集
	// 原因：指标收集有助于路由性能优化
	defaultEnableRouterMetrics = false

	// defaultEventValidatorEnabled 默认启用事件验证器
	// 原因：事件验证器确保事件的有效性
	defaultEventValidatorEnabled = false

	// defaultValidatorStrictMode 默认严格验证模式
	// 原因：严格模式提供更高的安全性
	defaultValidatorStrictMode = false

	// defaultValidateEventName 默认验证事件名称
	// 原因：验证事件名称确保事件格式正确
	defaultValidateEventName = true

	// defaultValidateEventData 默认验证事件数据
	// 原因：验证事件数据确保数据完整性
	defaultValidateEventData = false

	// defaultValidationTimeout 默认验证超时设为5秒
	// 原因：5秒足够完成事件验证
	defaultValidationTimeout = 5 * time.Second

	// defaultMaxValidationRules 默认最大验证规则数设为100
	// 原因：100个规则能满足大多数验证需求
	defaultMaxValidationRules = 100

	// defaultEnableBatchValidation 默认启用批量验证
	// 原因：批量验证提高验证效率
	defaultEnableBatchValidation = false

	// defaultCacheValidationResults 默认缓存验证结果
	// 原因：缓存结果提高验证性能
	defaultCacheValidationResults = false

	// defaultEventCoordinatorEnabled 默认启用事件协调器
	// 原因：事件协调器提供事件协调功能
	defaultEventCoordinatorEnabled = false

	// defaultMaxConcurrentEvents 默认最大并发事件数设为100
	// 原因：100个并发事件能满足大多数场景
	defaultMaxConcurrentEvents = 100

	// defaultEventTimeout 默认事件处理超时设为10秒
	// 原因：10秒足够完成事件处理
	defaultEventTimeout = 10 * time.Second

	// defaultHealthCheckInterval 默认健康检查间隔设为30秒
	// 原因：30秒间隔提供及时的健康状态监控
	defaultHealthCheckInterval = 30 * time.Second

	// defaultMetricsInterval 默认指标收集间隔设为10秒
	// 原因：10秒间隔提供足够的监控精度
	defaultMetricsInterval = 10 * time.Second

	// defaultEnableCircuitBreaker 默认启用熔断器
	// 原因：熔断器防止系统过载
	defaultEnableCircuitBreaker = false

	// defaultCircuitBreakerThreshold 默认熔断器阈值设为10
	// 原因：10个失败触发熔断是合理的阈值
	defaultCircuitBreakerThreshold = 10

	// defaultEnableGracefulShutdown 默认启用优雅关闭
	// 原因：优雅关闭确保事件不丢失
	defaultEnableGracefulShutdown = true

	// defaultGracefulShutdownTimeout 默认优雅关闭超时设为30秒
	// 原因：30秒足够完成优雅关闭
	defaultGracefulShutdownTimeout = 30 * time.Second
)
