// Package event 提供WES系统的事件总线接口定义
//
// 🎯 **事件总线系统 (Event Bus System)**
//
// 本文件定义了WES系统的事件总线接口，支持：
// - 标准事件订阅和发布
// - WES消息事件的特殊处理
// - 事件过滤和路由
// - 异步事件处理
// - 事件历史和监控
package event

import (
	"context"
	"time"

	"github.com/weisyn/v1/pkg/types"
)

// 兼容别名
type EventType = types.EventType

// 兼容别名
type ProtocolType = types.ProtocolType

// Event 事件接口
type Event interface {
	// Type 返回事件类型
	Type() EventType
	// Data 返回事件数据
	Data() interface{}
}

// EventBus 事件总线接口
//
// 🎯 **增强的事件总线**：
// - 保持现有接口的完全兼容
// - 新增WES消息事件的特殊支持
// - 增加事件过滤和监控能力
// - 支持上下文控制和超时管理
type EventBus interface {
	// ================== 标准事件接口 (保持兼容) ==================
	// 注意：事件总线由DI容器自动管理生命周期

	// Subscribe 订阅事件
	Subscribe(eventType EventType, handler interface{}) error
	// SubscribeAsync 异步订阅事件
	SubscribeAsync(eventType EventType, handler interface{}, transactional bool) error
	// SubscribeOnce 一次性订阅事件
	SubscribeOnce(eventType EventType, handler interface{}) error
	// SubscribeOnceAsync 异步一次性订阅事件
	SubscribeOnceAsync(eventType EventType, handler interface{}, transactional bool) error
	// Publish 发布事件
	Publish(eventType EventType, args ...interface{})
	// PublishEvent 发布Event接口类型事件
	PublishEvent(event Event)
	// Unsubscribe 取消订阅
	Unsubscribe(eventType EventType, handler interface{}) error
	// WaitAsync 等待所有异步处理完成
	WaitAsync()
	// HasCallback 检查是否有回调函数
	HasCallback(eventType EventType) bool
	// GetEventHistory 获取指定事件类型的历史记录
	// 如果历史功能未启用或没有历史记录，返回nil
	GetEventHistory(eventType EventType) []interface{}

	// ================== WES增强接口 ==================

	// PublishWESEvent 发布WES事件
	// 支持基于 Envelope（二进制） 的事件发布
	PublishWESEvent(event *types.WESEvent) error

	// SubscribeWithFilter 带过滤器的订阅
	// 支持复杂的事件过滤逻辑
	SubscribeWithFilter(eventType EventType, filter EventFilter, handler EventHandler) (types.SubscriptionID, error)

	// SubscribeWESEvents 订阅WES消息事件（按协议/Topic 过滤）
	SubscribeWESEvents(protocols []ProtocolType, handler WESEventHandler) (types.SubscriptionID, error)

	// UnsubscribeByID 通过订阅ID取消订阅
	UnsubscribeByID(id types.SubscriptionID) error

	// ================== 事件监控和指标 ==================

	// ❌ **已删除：GetEventMetrics() - 事件指标查询接口**
	//
	// 🚨 **删除原因**：
	// GetEventMetrics试图返回EventMetrics结构体，但该结构体已被删除。
	// 这个接口的删除再次确认了事件监控在自运行系统中的错误性：
	//   • 事件系统的作用是传递消息，不是收集统计数据
	//   • TotalEvents/EventsByType等指标没有任何决策价值
	//   • 事件处理性能应该由内部算法优化，不依赖外部监控
	//
	// 🎯 **事件系统的正确职责**：
	// 事件总线应该专注于：
	//   • 高效可靠的事件传递
	//   • 订阅者管理和事件路由
	//   • 异步事件处理和错误恢复
	//   • 不应该暴露事件处理统计信息

	// EnableEventHistory 启用事件历史记录
	EnableEventHistory(eventType EventType, maxSize int) error

	// DisableEventHistory 禁用事件历史记录
	DisableEventHistory(eventType EventType) error

	// GetActiveSubscriptions 获取活跃订阅列表
	GetActiveSubscriptions() ([]*types.SubscriptionInfo, error)

	// ================== 配置和管理 ==================

	// UpdateConfig 更新事件总线配置
	UpdateConfig(config *types.EventBusConfig) error

	// GetConfig 获取当前配置
	GetConfig() (*types.EventBusConfig, error)

	// RegisterEventInterceptor 注册事件拦截器
	RegisterEventInterceptor(interceptor EventInterceptor) error

	// UnregisterEventInterceptor 注销事件拦截器
	UnregisterEventInterceptor(interceptorID string) error
}

// ==================== 增强事件系统接口 ====================

// EnhancedEventBus 增强事件总线接口
//
// 🚀 **增强功能总览**：
// 包含所有基础EventBus功能，并新增：
// - 动态域注册和管理
// - 智能事件路由
// - 事件验证和过滤
// - 统一协调和生命周期管理
type EnhancedEventBus interface {
	EventBus // 继承所有基础功能

	// ================== 生命周期管理 ==================

	// Start 启动增强事件总线
	Start(ctx context.Context) error

	// Stop 停止增强事件总线
	Stop(ctx context.Context) error

	// IsRunning 检查是否正在运行
	IsRunning() bool

	// ================== 域注册管理 ==================

	// RegisterDomain 注册事件域
	RegisterDomain(domain string, info DomainInfo) error

	// UnregisterDomain 注销事件域
	UnregisterDomain(domain string) error

	// IsDomainRegistered 检查域是否已注册
	IsDomainRegistered(domain string) bool

	// ListDomains 列出所有已注册域
	ListDomains() []string

	// GetDomainInfo 获取域信息
	GetDomainInfo(domain string) (*DomainInfo, error)

	// ValidateEventName 验证事件名称格式
	ValidateEventName(eventName string) error

	// ================== 智能路由管理 ==================

	// SetRouteStrategy 设置事件类型的路由策略
	SetRouteStrategy(eventType string, strategy RouteStrategy) error

	// GetRouteStrategy 获取事件类型的路由策略
	GetRouteStrategy(eventType string) RouteStrategy

	// AddSubscriptionWithOptions 添加带选项的订阅
	AddSubscriptionWithOptions(eventType string, handler EventHandler, options ...SubscriptionOption) (string, error)

	// RemoveSubscription 移除订阅
	RemoveSubscription(subscriptionID string) error

	// SetSubscriptionPriority 设置订阅优先级
	SetSubscriptionPriority(subscriptionID string, priority Priority) error

	// ================== 事件验证管理 ==================

	// AddValidationRule 添加验证规则
	AddValidationRule(rule ValidationRule) error

	// RemoveValidationRule 移除验证规则
	RemoveValidationRule(ruleID string) error

	// ListValidationRules 列出所有验证规则
	ListValidationRules() []ValidationRule

	// ValidateEvent 验证事件
	ValidateEvent(event Event) error

	// ValidateEventWithContext 带上下文验证事件
	ValidateEventWithContext(ctx context.Context, event Event) error

	// ================== 批量操作 ==================

	// PublishEvents 批量发布事件
	PublishEvents(events []Event) error

	// ValidateEvents 批量验证事件
	ValidateEvents(events []Event) []error

	// ❌ **已删除：统计和监控方法**
	//
	// 🚨 **删除原因**：
	// 删除了3个监控相关的接口方法：
	// - GetStatistics() *EventStatistics - 获取事件统计信息
	// - GetHealthStatus() *HealthStatus - 获取健康状态
	// - ResetStatistics() error - 重置统计信息
	//
	// 🎯 **符合项目偏好**：
	// 公共接口不暴露监控结构，避免在自治系统中暴露无意义运行状态
}

// DomainRegistry 域注册中心接口
type DomainRegistry interface {
	// RegisterDomain 注册事件域
	RegisterDomain(domain string, info DomainInfo) error

	// UnregisterDomain 注销事件域
	UnregisterDomain(domain string) error

	// IsDomainRegistered 检查域是否已注册
	IsDomainRegistered(domain string) bool

	// ListDomains 列出所有已注册域
	ListDomains() []string

	// GetDomainInfo 获取域信息
	GetDomainInfo(domain string) (*DomainInfo, error)

	// ValidateEventName 验证事件名称是否符合已注册域
	ValidateEventName(eventName string) error

	// AddEventRoute 添加事件路由信息
	AddEventRoute(eventType string, subscriber string) error

	// RemoveEventRoute 移除事件路由信息
	RemoveEventRoute(eventType string, subscriber string) error

	// GetEventRoutes 获取事件路由信息
	GetEventRoutes(eventType string) []string

	// ❌ **已删除：GetStatistics() - 注册中心统计方法**
	// 删除原因：返回RegistryStatistics结构体（已删除），符合"接口不暴露指标"偏好
}

// EventRouter 事件路由器接口
type EventRouter interface {
	// Start 启动路由器
	Start() error

	// Stop 停止路由器
	Stop() error

	// IsRunning 检查是否正在运行
	IsRunning() bool

	// AddSubscription 添加订阅
	AddSubscription(eventType string, handler EventHandler, options ...SubscriptionOption) (string, error)

	// RemoveSubscription 移除订阅
	RemoveSubscription(subscriptionID string) error

	// SetRouteStrategy 设置路由策略
	SetRouteStrategy(eventType string, strategy RouteStrategy) error

	// GetRouteStrategy 获取路由策略
	GetRouteStrategy(eventType string) RouteStrategy

	// RouteEvent 路由事件到订阅者
	RouteEvent(eventType string, event Event) error

	// SetSubscriptionPriority 设置订阅优先级
	SetSubscriptionPriority(subscriptionID string, priority Priority) error

	// GetActiveSubscriptions 获取活跃订阅
	GetActiveSubscriptions() []SubscriptionInfo

	// ❌ **已删除：GetStatistics() - 路由器统计方法**
	// 删除原因：返回RouterStatistics结构体（已删除），符合"接口不暴露指标"偏好
}

// EventValidator 事件验证器接口
type EventValidator interface {
	// ValidateEvent 验证事件
	ValidateEvent(event Event) error

	// ValidateEventWithContext 带上下文验证事件
	ValidateEventWithContext(ctx context.Context, event Event) error

	// AddValidationRule 添加验证规则
	AddValidationRule(rule ValidationRule) error

	// RemoveValidationRule 移除验证规则
	RemoveValidationRule(ruleID string) error

	// ListValidationRules 列出所有验证规则
	ListValidationRules() []ValidationRule

	// ValidateEvents 批量验证事件
	ValidateEvents(events []Event) []error

	// ❌ **已删除：GetStatistics() - 验证器统计方法**
	// 删除原因：返回ValidatorStatistics结构体（已删除），符合"接口不暴露指标"偏好
}

// EventCoordinator 事件协调器接口
type EventCoordinator interface {
	// Start 启动协调器
	Start(ctx context.Context) error

	// Stop 停止协调器
	Stop(ctx context.Context) error

	// IsRunning 检查是否正在运行
	IsRunning() bool

	// PublishEvent 发布事件
	PublishEvent(eventType string, data interface{}, opts ...PublishOption) error

	// SubscribeEvent 订阅事件
	SubscribeEvent(eventType string, handler EventHandler) (string, error)

	// SubscribeEventWithOptions 带选项订阅事件
	SubscribeEventWithOptions(eventType string, handler EventHandler, options ...SubscriptionOption) (string, error)

	// UnsubscribeEvent 取消订阅事件
	UnsubscribeEvent(subscriptionID string) error

	// RegisterDomain 注册域
	RegisterDomain(domain string, info DomainInfo) error

	// UnregisterDomain 注销域
	UnregisterDomain(domain string) error

	// AddValidationRule 添加验证规则
	AddValidationRule(rule ValidationRule) error

	// RemoveValidationRule 移除验证规则
	RemoveValidationRule(ruleID string) error

	// PublishEvents 批量发布事件
	PublishEvents(events []EventData) error

	// ValidateEvents 批量验证事件
	ValidateEvents(events []Event) []error

	// ❌ **已删除：GetStatistics() - 协调器统计方法**
	// 删除原因：返回CoordinatorStatistics结构体（已删除），符合"接口不暴露指标"偏好

	// ❌ **已删除：GetHealthStatus() - 协调器健康状态方法**
	// 删除原因：返回HealthStatus结构体（已删除），自治系统不需要暴露健康状态

	// UpdateConfiguration 更新配置
	UpdateConfiguration(config interface{}) error
}

// ==================== WES事件相关接口 ====================

// 兼容别名
type WESEvent = types.WESEvent

// EventFilter 事件过滤器接口
// 用于实现复杂的事件过滤逻辑
type EventFilter interface {
	// Match 检查事件是否匹配过滤条件
	Match(event Event) bool

	// MatchWES 检查WES事件是否匹配
	MatchWES(event *WESEvent) bool

	// GetFilterInfo 获取过滤器信息
	GetFilterInfo() *types.FilterInfo
}

// EventHandler 标准事件处理器
type EventHandler func(event Event) error

// WESEventHandler WES事件处理器
type WESEventHandler = types.WESEventHandler

// EventInterceptor 事件拦截器接口
// 用于在事件发布前后进行处理
type EventInterceptor interface {
	// PrePublish 发布前拦截
	PrePublish(event Event) (Event, error)

	// PostPublish 发布后拦截
	PostPublish(event Event, result error) error

	// GetInterceptorInfo 获取拦截器信息
	GetInterceptorInfo() *types.InterceptorInfo
}

// ==================== 数据结构定义 ====================

// 兼容别名：将已迁移到 pkg/types 的数据结构在本包中提供别名，避免大范围改动
type SubscriptionID = types.SubscriptionID
type SubscriptionInfo = types.SubscriptionInfo
type FilterInfo = types.FilterInfo
type InterceptorInfo = types.InterceptorInfo
type EventBusConfig = types.EventBusConfig
type Priority = types.Priority

// ==================== 增强功能数据类型 ====================

// DomainInfo 事件域信息
type DomainInfo struct {
	Name         string    `json:"name"`          // 域名
	Component    string    `json:"component"`     // 所属组件
	Description  string    `json:"description"`   // 描述信息
	EventTypes   []string  `json:"event_types"`   // 该域支持的事件类型（可选）
	RegisteredAt time.Time `json:"registered_at"` // 注册时间
	Active       bool      `json:"active"`        // 是否活跃
}

// RouteStrategy 路由策略类型
type RouteStrategy string

const (
	RouteStrategyDirect     RouteStrategy = "direct"      // 直接路由
	RouteStrategyBroadcast  RouteStrategy = "broadcast"   // 广播路由
	RouteStrategyRoundRobin RouteStrategy = "round_robin" // 轮询路由
	RouteStrategyPriority   RouteStrategy = "priority"    // 优先级路由
	RouteStrategyFilter     RouteStrategy = "filter"      // 过滤路由
)

// SubscriptionOption 订阅选项
type SubscriptionOption func(*SubscriptionConfig)

// SubscriptionConfig 订阅配置
type SubscriptionConfig struct {
	Priority  Priority               `json:"priority"`  // 优先级
	Component string                 `json:"component"` // 组件标识
	Metadata  map[string]interface{} `json:"metadata"`  // 元数据
}

// ValidationRule 验证规则接口
type ValidationRule interface {
	// GetID 获取规则ID
	GetID() string

	// GetName 获取规则名称
	GetName() string

	// Validate 执行验证
	Validate(event Event) error

	// GetDescription 获取规则描述
	GetDescription() string

	// IsEnabled 是否启用
	IsEnabled() bool
}

// PublishOption 发布选项
type PublishOption func(*PublishConfig)

// PublishConfig 发布配置
type PublishConfig struct {
	Priority   Priority               `json:"priority"`    // 事件优先级
	Component  string                 `json:"component"`   // 发布组件
	Metadata   map[string]interface{} `json:"metadata"`    // 事件元数据
	Async      bool                   `json:"async"`       // 是否异步发布
	Timeout    time.Duration          `json:"timeout"`     // 发布超时时间
	RetryCount int                    `json:"retry_count"` // 重试次数
}

// EventData 事件数据结构
type EventData struct {
	Type     string                 `json:"type"`     // 事件类型
	Data     interface{}            `json:"data"`     // 事件数据
	Metadata map[string]interface{} `json:"metadata"` // 元数据
}

// ❌ **已删除：EventStatistics - 事件统计信息**
//
// 🚨 **删除原因**：
// 事件统计结构体包含9个监控字段，违反了项目"接口不暴露指标"偏好：
// - TotalEvents/SuccessfulEvents/FailedEvents - 事件数量统计没有明确消费者
// - AverageLatency/EventsPerSecond - 性能统计在自治系统中无实际用途
// - EventsByType/EventsByDomain - 分类统计增加复杂度而无决策价值
// - LastEventTime/StatisticsStartTime - 时间信息管理过于细致
//
// ❌ **已删除：HealthStatus/HealthLevel - 健康状态监控**
//
// 🚨 **删除原因**：
// 健康状态监控违反自治原则：
// - Overall/Components/Issues/LastCheck - 健康监控的消费者是谁？
// - 自治系统应该内部处理健康问题，不需要对外暴露健康状态
// - 健康级别(healthy/warning/critical/unknown)属于传统IT运维思维
//
// ❌ **已删除：RegistryStatistics - 注册中心统计信息**
//
// 🚨 **删除原因**：
// 注册中心统计包含6个统计字段：
// - TotalDomains/ActiveDomains/TotalEventTypes - 数量统计无实际价值
// - DomainsPerComponent/EventTypesPerDomain - 分布统计过于细致
// - LastRegistrationTime - 时间追踪不必要
//
// ❌ **已删除：RouterStatistics - 路由器统计信息**
//
// 🚨 **删除原因**：
// 路由器统计包含8个详细统计：
// - TotalSubscriptions/ActiveSubscriptions - 订阅数量统计无决策意义
// - RoutedEvents/FailedRoutes/AverageRouteTime - 路由性能统计没有消费者
// - SubscriptionsByType/RoutesByStrategy - 过细的分类统计
// - LastRouteTime - 不必要的时间追踪
//
// ❌ **已删除：ValidatorStatistics - 验证器统计信息**
//
// 🚨 **删除原因**：
// 验证器统计包含7个统计字段：
// - TotalValidations/SuccessValidations/FailedValidations - 验证数量统计
// - AverageLatency - 平均延迟监控
// - ValidationsByRule/RuleStatistics - 规则级别的详细统计
// - LastValidationTime - 时间追踪
//
// ❌ **已删除：CoordinatorStatistics - 协调器统计信息**
//
// 🚨 **删除原因**：
// 协调器统计是最复杂的监控结构体，包含9个字段：
// - TotalOperations/SuccessfulOperations/FailedOperations - 操作统计
// - AverageResponseTime - 响应时间监控
// - DomainRegistryStats/EventRouterStats/EventValidatorStats - 嵌套统计
// - ComponentHealth - 组件健康状态（已删除的HealthLevel类型）
// - LastOperationTime - 时间追踪
//
// 🎯 **删除总结**：
// 所有这些监控结构体都违反了项目核心原则：
// 1. **接口不暴露指标** - 公共接口不应包含监控数据
// 2. **自治系统** - 组件应该内部处理问题，不需要外部监控
// 3. **无明确消费者** - 这些统计数据没有明确的使用场景
// 4. **增加复杂度** - 监控逻辑比业务逻辑还复杂

// ❌ **已删除：EventMetrics - 事件总线的过度监控结构**
//
// 🚨 **删除原因**：
// EventMetrics又是一个"事无巨细监控"的产物，包含13个统计字段：
//
// **🔥 基础统计组（3个字段）**：
//   • TotalEvents/EventsByType/EventsByProtocol - 事件数量统计给谁看？用于什么决策？
//   问题：事件系统应该专注于事件传递，而非事件统计
//
// **🔥 处理统计组（3个字段）**：
//   • SuccessfulEvents/FailedEvents/AvgProcessingTime - 事件处理统计的实际价值何在？
//   问题：事件处理失败应该通过重试机制解决，不需要外部统计
//
// **🔥 订阅统计组（2个字段）**：
//   • ActiveSubscriptions/SubscriptionsByType - 订阅数量的监控有什么决策意义？
//   问题：订阅管理是内部功能，不需要暴露订阅统计
//
// **🔥 性能指标组（3个字段）**：
//   • EventsPerSecond/MemoryUsage/QueueLength - 事件系统的性能监控给谁用？
//   问题：事件系统应该自动优化性能，不需要外部性能监控
//
// **🔥 时间信息组（2个字段）**：
//   • MeasurementPeriod/LastUpdated - 监控数据的时间戳管理
//   问题：连监控数据本身都需要时间戳管理，过度复杂
//
// 🎯 **事件指标的设计错误**：
//
// **1. 监控成本罪** - 每个事件都要更新统计数据，影响事件处理性能
//   问题：为了监控事件性能，反而降低了事件性能
//   现实：统计数据的维护开销比事件处理本身还大
//
// **2. 职责混乱罪** - 事件系统变成了监控分析系统
//   问题：事件总线的核心职责是事件传递，不是数据分析
//   现实：监控逻辑比事件逻辑还复杂
//
// **3. 无价值收集罪** - 收集大量数据但没有明确的使用场景
//   问题：EventsByType/EventsByProtocol等统计数据的商业价值何在？
//   现实：纯粹的"数据收集强迫症"
//
// 🎯 **正确的事件系统设计应该**：
// 1. 专注于高效可靠的事件传递
// 2. 自动处理事件传递失败和重试
// 3. 内部优化事件处理性能
// 4. 不暴露事件处理过程的统计细节

// EventBusConfig 事件总线配置已迁移至 pkg/types.EventBusConfig

// ==================== 预定义事件类型 ====================

const (
	// 系统事件
	EventTypeSystemStartup  EventType = "system.startup"
	EventTypeSystemShutdown EventType = "system.shutdown"
	EventTypeSystemError    EventType = "system.error"

	// 节点事件
	EventTypeHostStarted EventType = "host.started" // Host启动事件
	EventTypeHostStopped EventType = "host.stopped" // Host停止事件
	EventTypeHostError   EventType = "host.error"   // Host错误事件

	// 网络事件
	EventTypeNetworkPeerConnected    EventType = "network.peer.connected"
	EventTypeNetworkPeerDisconnected EventType = "network.peer.disconnected"
	EventTypeNetworkMessageReceived  EventType = "network.message.received"
	EventTypeNetworkMessageSent      EventType = "network.message.sent"
	EventTypeNetworkQualityChanged   EventType = "network.quality.changed"

	// K桶事件（诊断用）
	EventTypeKBucketSummaryUpdated EventType = "kbucket.summary.updated"

	// 自愈/损坏事件（生产自运行：不依赖人工介入）
	EventTypeCorruptionDetected EventType = "corruption.detected"
	EventTypeCorruptionRepaired EventType = "corruption.repaired"
	EventTypeCorruptionRepairFailed EventType = "corruption.repair_failed"

	// 共识事件
	EventTypeConsensusBlockMined     EventType = "consensus.block.mined"
	EventTypeConsensusBlockReceived  EventType = "consensus.block.received"
	EventTypeConsensusVoteReceived   EventType = "consensus.vote.received"
	EventTypeConsensusRoundCompleted EventType = "consensus.round.completed"
	EventTypeConsensusTimeout        EventType = "consensus.timeout"

	// 同步事件
	EventTypeSyncStarted   EventType = "sync.started"
	EventTypeSyncCompleted EventType = "sync.completed"
	EventTypeSyncFailed    EventType = "sync.failed"
	EventTypeSyncProgress  EventType = "sync.progress"
	EventTypeSyncConflict  EventType = "sync.conflict"

	// 分发事件
	EventTypeDistributionStarted   EventType = "distribution.started"
	EventTypeDistributionCompleted EventType = "distribution.completed"
	EventTypeDistributionFailed    EventType = "distribution.failed"

	// 状态事件
	EventTypeStateChanged            EventType = "state.changed"
	EventTypeStateCoordinationNeeded EventType = "state.coordination.needed"

	// 决策事件
	EventTypeDecisionRequired EventType = "decision.required"
	EventTypeDecisionMade     EventType = "decision.made"
	EventTypeDecisionExecuted EventType = "decision.executed"

	// 区块链事件
	EventTypeBlockProduced  EventType = "blockchain.block.produced"  // 区块生产完成
	EventTypeBlockValidated EventType = "blockchain.block.validated" // 区块验证完成
	EventTypeBlockProcessed EventType = "blockchain.block.processed" // 区块处理完成
	EventTypeBlockConfirmed EventType = "blockchain.block.confirmed" // 区块确认
	EventTypeBlockReverted  EventType = "blockchain.block.reverted"  // 区块回滚
	EventTypeBlockFinalized EventType = "blockchain.block.finalized" // 区块最终确认

	// 链状态事件
	EventTypeChainHeightChanged EventType = "blockchain.chain.height_changed" // 链高度变化
	EventTypeChainStateUpdated  EventType = "blockchain.chain.state_updated"  // 链状态更新
	EventTypeChainReorganized   EventType = "blockchain.chain.reorganized"    // 链重组

	// 分叉处理事件 - 分叉检测和处理流程
	EventTypeForkDetected   EventType = "blockchain.fork.detected"   // 分叉检测
	EventTypeForkProcessing EventType = "blockchain.fork.processing" // 分叉处理中
	EventTypeForkCompleted  EventType = "blockchain.fork.completed"  // 分叉处理完成
	EventTypeForkFailed     EventType = "blockchain.fork.failed"     // 分叉处理失败

	// 细粒度 REORG 阶段事件
	EventTypeReorgPrepareStarted     EventType = "blockchain.reorg.prepare.started"
	EventTypeReorgPrepareCompleted   EventType = "blockchain.reorg.prepare.completed"
	EventTypeReorgRollbackStarted    EventType = "blockchain.reorg.rollback.started"
	EventTypeReorgRollbackCompleted  EventType = "blockchain.reorg.rollback.completed"
	EventTypeReorgReplayStarted      EventType = "blockchain.reorg.replay.started"
	EventTypeReorgReplayCompleted    EventType = "blockchain.reorg.replay.completed"
	EventTypeReorgVerifyStarted      EventType = "blockchain.reorg.verify.started"
	EventTypeReorgVerifyCompleted    EventType = "blockchain.reorg.verify.completed"
	EventTypeReorgCommitStarted      EventType = "blockchain.reorg.commit.started"
	EventTypeReorgCommitCompleted    EventType = "blockchain.reorg.commit.completed"
	EventTypeReorgAborted            EventType = "blockchain.reorg.aborted"
	EventTypeReorgCompensation       EventType = "blockchain.reorg.compensation"
)

// ==================== 优先级常量 ====================

const (
	PriorityCritical Priority = 4 // 关键优先级
	PriorityHigh     Priority = 3 // 高优先级
	PriorityNormal   Priority = 2 // 普通优先级
	PriorityLow      Priority = 1 // 低优先级
)

// ==================== 便利函数 ====================

// WithPriority 设置订阅优先级
func WithPriority(priority Priority) SubscriptionOption {
	return func(config *SubscriptionConfig) {
		config.Priority = priority
	}
}

// WithComponent 设置订阅组件标识
func WithComponent(component string) SubscriptionOption {
	return func(config *SubscriptionConfig) {
		config.Component = component
	}
}

// WithMetadata 设置订阅元数据
func WithMetadata(metadata map[string]interface{}) SubscriptionOption {
	return func(config *SubscriptionConfig) {
		config.Metadata = metadata
	}
}

// WithPublishPriority 设置发布优先级
func WithPublishPriority(priority Priority) PublishOption {
	return func(config *PublishConfig) {
		config.Priority = priority
	}
}

// WithPublishComponent 设置发布组件标识
func WithPublishComponent(component string) PublishOption {
	return func(config *PublishConfig) {
		config.Component = component
	}
}

// WithPublishMetadata 设置发布元数据
func WithPublishMetadata(metadata map[string]interface{}) PublishOption {
	return func(config *PublishConfig) {
		config.Metadata = metadata
	}
}

// WithAsync 设置异步发布
func WithAsync(async bool) PublishOption {
	return func(config *PublishConfig) {
		config.Async = async
	}
}

// WithTimeout 设置发布超时
func WithTimeout(timeout time.Duration) PublishOption {
	return func(config *PublishConfig) {
		config.Timeout = timeout
	}
}

// WithRetry 设置重试次数
func WithRetry(retryCount int) PublishOption {
	return func(config *PublishConfig) {
		config.RetryCount = retryCount
	}
}

// NewDomainInfo 创建域信息
func NewDomainInfo(name, component, description string) DomainInfo {
	return DomainInfo{
		Name:         name,
		Component:    component,
		Description:  description,
		EventTypes:   make([]string, 0),
		RegisteredAt: time.Now(),
		Active:       true,
	}
}

// NewEventData 创建事件数据
func NewEventData(eventType string, data interface{}) EventData {
	return EventData{
		Type:     eventType,
		Data:     data,
		Metadata: make(map[string]interface{}),
	}
}

// NewEventDataWithMetadata 创建带元数据的事件数据
func NewEventDataWithMetadata(eventType string, data interface{}, metadata map[string]interface{}) EventData {
	return EventData{
		Type:     eventType,
		Data:     data,
		Metadata: metadata,
	}
}
