// Package kbucket 提供WES系统的Kademlia DHT路由表接口定义
//
// 🌍 **Kademlia DHT路由表管理 (Kademlia DHT Routing Table Management)**
//
// 本文件定义了WES Kademlia DHT的路由表管理接口，专注于：
// - K桶的组织和管理
// - 节点发现和距离计算
// - 路由表的维护和优化
// - DHT网络的健康监控
//
// 🎯 **设计原则**
// - 高效路由：基于XOR距离的高效路由算法
// - 动态维护：实时更新和维护路由表状态
// - 网络健康：全面的网络健康监控和诊断
// - 扩展性：支持大规模DHT网络的高效管理
// Package kbucket 提供WES系统的Kademlia路由表接口定义
//
// 🗺️ **Kademlia路由表管理 (Kademlia Routing Table Management)**
//
// 本文件定义了WES区块链系统的Kademlia路由表管理接口，专注于：
// - K桶管理：按距离组织节点的K桶结构管理
// - 节点发现：基于XOR距离的节点发现和选择
// - 路由优化：动态路由表维护和优化策略
// - 健康监控：节点健康状态监控和自动清理
//
// 🎯 **核心功能**
// - KBucket：K桶管理器接口，提供完整的路由表管理服务
// - 距离计算：基于XOR距离的节点距离计算和比较
// - 节点管理：节点的添加、删除、更新和查询
// - 路由查找：高效的路由查找和节点选择算法
//
// 🏧 **设计原则**
// - 算法标准：遵循Kademlia DHT的标准算法和协议
// - 性能优先：优化的数据结构和查找算法
// - 可扩展性：支持大规模网络和动态节点管理
// - 容错性：强大的错误处理和网络分区容忍
//
// 🔗 **组件关系**
// - KBucket：被P2P、网络发现、路由等模块使用
// - 与P2PService：为P2P网络提供节点发现和路由能力
// - 与NetworkService：为网络通信提供路由和连接选择
package kademlia

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/weisyn/v1/pkg/types"
)

// KBucketManager Kademlia DHT路由表管理器接口
// 统一管理DHT路由表、节点发现、距离计算和健康监控
type KBucketManager interface {
	// === 组合细粒度接口 ===
	DistanceCalculator
	PeerSelector

	// === Manager特有的统一方法 ===
	// AddPeer 添加节点到路由表
	AddPeer(ctx context.Context, req *AddPeerRequest) error

	// RemovePeer 从路由表移除节点
	RemovePeer(ctx context.Context, peerID string) error

	// UpdatePeer 更新节点信息
	UpdatePeer(ctx context.Context, req *UpdatePeerRequest) error

	// GetPeer 获取节点信息
	GetPeer(ctx context.Context, peerID string) (*PeerInfo, error)

	// ListPeers 列出节点
	ListPeers(ctx context.Context, req *ListPeersRequest) (*ListPeersResponse, error)

	// === 路由查找（扩展方法） ===
	// FindClosestPeers 查找最接近目标的节点（Manager版本）
	FindClosestPeersWithContext(ctx context.Context, req *FindPeersRequest) (*FindPeersResponse, error)

	// CalculateDistance 计算节点距离（Manager版本）
	CalculateDistanceWithContext(ctx context.Context, req *DistanceRequest) (*DistanceResponse, error)

	// === 路由表管理 ===
	// GetRoutingTable 获取路由表快照
	GetRoutingTable(ctx context.Context) (*RoutingTable, error)

	// RefreshBuckets 刷新桶
	RefreshBuckets(ctx context.Context) error

	// ❌ **已删除：GetBucketStats() - 桶统计查询接口**
	//
	// 🚨 **删除原因**：
	// GetBucketStats试图返回BucketStats结构体数组，但该结构体已被删除。
	// 这个接口的删除进一步证明了桶统计在自运行系统中的无价值性：
	//   • 桶统计数据的外部查询没有任何实际用途
	//   • KBucket算法应该内部自治，不需要暴露桶状态
	//   • 路由决策基于距离计算，不依赖统计数据
	//
	// 🎯 **替代方案**：
	// 如果需要路由状态信息，应该通过事件机制通知：
	//   • OnBucketFull 事件：桶满时的处理
	//   • OnPeerEvicted 事件：节点淘汰通知
	//   • OnRoutingOptimized 事件：路由优化完成

	// OptimizeRoutingTable 优化路由表
	OptimizeRoutingTable(ctx context.Context) error

	// === 事件处理 ===
	// RegisterEventHandler 注册事件处理器
	RegisterEventHandler(handler RoutingTableEventHandler) error

	// GetEvents 获取事件流
	GetEvents(ctx context.Context) <-chan *RoutingTableEvent

	// ================== ❌ 已删除：无意义的健康监控和统计接口 ==================
	//
	// 🚨 **为什么删除健康监控和统计接口？**
	//
	// 在自运行区块链系统中，K桶(Kademlia DHT)的健康监控完全没有价值：
	//
	// ❌ **删除的接口及原因**：
	//   • ToggleHealthMonitoring() - 切换健康监控状态
	//     问题：谁会基于什么条件来开启/关闭健康监控？这个决策有什么依据？
	//   • GetHealthStats() - 获取健康统计信息
	//     问题：健康统计给谁看？MonitoredPeers、HealthyPeers数量有什么用？
	//   • CheckPeerHealth() - 检查单个节点健康状态
	//     问题：检查完健康状态然后做什么？系统会自动处理不健康的节点
	//   • GetRoutingMetrics() - 获取路由指标
	//     问题：路由指标给谁看？QuerySuccess率、AvgLatency等统计有什么实际意义？
	//   • GetKBucketStats() - 获取K桶统计
	//     问题：桶的填充率、节点分布等统计数据的消费者是谁？
	//   • RecordQuery() - 记录查询操作
	//     问题：记录查询是为了统计，但统计数据又没有消费者
	//
	// 🎯 **DHT系统的正确设计理念**：
	//   • DHT路由表应该自主维护节点健康状态
	//   • 不健康的节点由内部算法自动替换
	//   • 路由效率通过算法优化，不需要外部监控
	//   • 查询记录只用于内部算法优化，不应暴露给外部
	//
	// ⚠️ **给未来开发者的严重警告**：
	//   K桶监控是典型的过度工程化！在重新添加任何监控接口前，请深思：
	//   1. 这些监控数据的具体消费者是谁？
	//   2. 基于这些数据会触发什么自动化操作？
	//   3. 为什么DHT内部机制不能自动处理这些问题？
	//   4. 外部监控在自治P2P网络中的必要性是什么？

	// === 组件访问 ===
	// GetDistanceCalculator 获取距离计算器
	GetDistanceCalculator() DistanceCalculator

	// GetPeerSelector 获取节点选择器
	GetPeerSelector() PeerSelector

	// GetRoutingTableManager 获取路由表管理器
	GetRoutingTableManager() RoutingTableManager

	// === 生命周期 ===
	// 注意：K桶管理器由DI容器自动管理生命周期
	//
	// ❌ **已删除：GetStatus() - 无意义的状态查询接口**
	//
	// 🚨 **删除原因**：
	// GetStatus()试图暴露管理器的运行状态，但这些信息在自运行系统中没有价值：
	//   • IsRunning - 系统知道自己在运行，无需外部确认
	//   • StartTime/Uptime - 运行时间给谁看？有什么用？
	//   • HealthScore - 健康评分的计算标准是什么？谁会基于此做决策？
	//   • Performance指标 - QueriesPerSecond、MemoryUsage等监控数据的消费者是谁？
	//
	// 🎯 **正确的状态管理**：
	// 在自治系统中，组件状态应该：
	// 1. 由内部机制自动维护
	// 2. 异常时由内部逻辑自动处理
	// 3. 不向外暴露无意义的运行时信息
}

// 兼容别名（数据结构迁至 pkg/types）
type PeerInfo = types.PeerInfo

// 兼容别名
type RoutingTable = types.RoutingTable

// 兼容别名
type Bucket = types.Bucket

// PeerDiversityFilter 节点多样性过滤器接口
type PeerDiversityFilter interface {
	// Allow 判断是否允许添加节点
	Allow(group PeerGroupInfo) bool

	// Increment 增加组计数
	Increment(group PeerGroupInfo)

	// Decrement 减少组计数
	Decrement(group PeerGroupInfo)

	// PeerAddresses 获取节点地址
	PeerAddresses(peerID string) []string
}

// 兼容别名
type PeerGroupInfo = types.PeerGroupInfo

// RoutingStrategy 路由策略接口
type RoutingStrategy interface {
	// CalculateDistance 计算距离
	CalculateDistance(source, target string) []byte

	// SelectClosestPeers 选择最近的节点
	SelectClosestPeers(peers []*PeerInfo, target string, count int) []*PeerInfo

	// GetBucketIndex 获取桶索引
	GetBucketIndex(localID, peerID string) int
}

// === 旧版接口（将被移除） ===
// 这些接口已经整合到KBucketManager中，仅作为过渡使用

// PeerFilter 节点过滤器函数类型（已迁移）
type PeerFilter func(peer.ID) bool

// 兼容别名
type SelectionCriteria = types.SelectionCriteria

// === Manager模式统一类型定义 ===

// 兼容别名
type AddPeerRequest = types.AddPeerRequest

// 兼容别名
type UpdatePeerRequest = types.UpdatePeerRequest

// 兼容别名
type ListPeersRequest = types.ListPeersRequest

// 兼容别名
type ListPeersResponse = types.ListPeersResponse

// 兼容别名
type FindPeersRequest = types.FindPeersRequest

// 兼容别名
type FindPeersResponse = types.FindPeersResponse

// 兼容别名
type DistanceRequest = types.DistanceRequest

// 兼容别名
type DistanceResponse = types.DistanceResponse

// ❌ **已删除：BucketStats - KBucket的微观管理统计结构**
//
// 🚨 **删除原因**：
// BucketStats是"路由表微观管理"的典型例子，包含9个统计字段：
//
// **🔥 容量统计组（4个字段）**：
//   • BucketIndex/PeerCount/MaxSize/HealthyPeers - 桶容量的细分统计有什么决策价值？
//   问题：KBucket算法应该自动管理桶的填充和替换，不需要外部监控桶状态
//
// **🔥 时间统计组（3个字段）**：
//   • LastRefresh/RefreshCount/AverageLatency - 刷新时间和频率的统计给谁用？
//   问题：路由表的刷新策略应该由算法决定，不需要时间统计
//
// **🔥 性能指标组（2个字段）**：
//   • AverageLatency/UtilizationRate - 延迟和利用率的监控意义何在？
//   问题：节点选择应该基于距离算法，不是基于统计数据
//
// 🎯 **KBucket统计的设计错误**：
//
// **1. 算法干扰罪** - 统计监控干扰了KBucket算法的纯粹性
//   问题：KBucket是经典的DHT算法，应该专注于距离计算和节点管理
//   现实：统计逻辑污染了算法的简洁性
//
// **2. 过度优化罪** - 试图通过统计数据优化本来就高效的算法
//   问题：KBucket算法经过大量实践验证，不需要基于统计的优化
//   现实：过度优化可能破坏算法的稳定性
//
// **3. 监控成本罪** - 每次路由操作都要更新统计数据
//   问题：统计数据的维护影响了路由查询性能
//   现实：为了监控路由性能，反而降低了路由性能
//
// 🎯 **正确的KBucket设计应该**：
// 1. 专注于经典的KBucket算法实现
// 2. 基于节点距离而非统计数据做路由决策
// 3. 自动处理节点失效和替换
// 4. 不暴露路由表的内部统计细节

// RoutingTableEventHandler 路由表事件处理器
type RoutingTableEventHandler interface {
	// OnPeerAdded 节点添加事件
	OnPeerAdded(ctx context.Context, event *PeerAddedEvent) error

	// OnPeerRemoved 节点移除事件
	OnPeerRemoved(ctx context.Context, event *PeerRemovedEvent) error

	// OnPeerUpdated 节点更新事件
	OnPeerUpdated(ctx context.Context, event *PeerUpdatedEvent) error

	// OnBucketRefresh 桶刷新事件
	OnBucketRefresh(ctx context.Context, event *BucketRefreshEvent) error

	// GetHandlerName 获取处理器名称
	GetHandlerName() string
}

// 兼容别名
type RoutingTableInfo = types.RoutingTableInfo

// 兼容别名
type NodeHealthInfo = types.NodeHealthInfo

// 兼容别名
type RoutingTableEvent = types.RoutingTableEvent

// 兼容别名
type EventType = types.EventType

const (
	EventTypePeerAdded      = types.EventTypePeerAdded
	EventTypePeerRemoved    = types.EventTypePeerRemoved
	EventTypePeerUpdated    = types.EventTypePeerUpdated
	EventTypeBucketRefresh  = types.EventTypeBucketRefresh
	EventTypeTableOptimized = types.EventTypeTableOptimized
	EventTypeHealthCheck    = types.EventTypeHealthCheck
)

// 兼容别名
type PeerAddedEvent = types.PeerAddedEvent

// 兼容别名
type PeerRemovedEvent = types.PeerRemovedEvent

// 兼容别名
type PeerUpdatedEvent = types.PeerUpdatedEvent

// 兼容别名
type BucketRefreshEvent = types.BucketRefreshEvent

// ❌ **已删除：大批量监控统计结构体 - 过度工程化的监控系统**
//
// 🚨 **批量删除原因**：
// 以下结构体代表了典型的"过度监控"设计错误，在自运行区块链中完全没有价值：
//
// ❌ **HealthMonitorConfig** - 健康监控配置
//   问题：谁来配置监控间隔？基于什么标准调整CheckInterval？MaxFailures阈值如何确定？
//   现实：DHT系统应该有内置的节点健康检查机制，不需要外部配置
//
// ❌ **PeerHealthStatus** - 节点健康状态
//   问题：ResponseTime、FailureCount、HealthScore这些详细信息给谁看？
//   现实：不健康的节点应该由DHT算法自动替换，不需要暴露健康细节
//
// ❌ **KBucketStats** - K桶统计信息
//   问题：TotalBuckets、ActiveBuckets、AverageLatency等统计有什么实际用途？
//   现实：桶的状态是DHT内部实现细节，外部无需关注
//
// ❌ **QueryRecord** - 查询记录
//   问题：记录每个查询的Duration、ResultCount、Success状态有什么意义？
//   现实：查询效率应该由DHT算法内部优化，不需要外部分析
//
// ❌ **ManagerStatus** - 管理器状态
//   问题：IsRunning、StartTime、Uptime、HealthScore给谁看？看了做什么？
//   现实：组件状态应该内部维护，不应向外暴露运行时细节
//
// 🎯 **根本性设计错误**：
// 这些监控结构体反映了"传统IT运维"的思维模式，试图监控每一个细节。
// 但在自运行区块链系统中：
// 1. 系统应该自治，不需要外部监控干预
// 2. 异常应该由内部机制自动处理
// 3. 监控数据没有明确的消费者和使用场景
//
// ⚠️ **严重警告**：
// 不要重新引入这些监控结构体！它们代表着架构设计的根本性错误。
// 在自治系统中，组件应该"做好自己的事"，而不是"报告自己在做什么"。

// ❌ **已删除：PerformanceStats - 无意义的性能监控结构**
//
// 🚨 **删除原因**：
// PerformanceStats代表了性能监控的错误理念：
//   • QueriesPerSecond - 每秒查询数给谁看？达到多少算正常？
//   • AverageQueryTime - 平均查询时间的阈值是什么？谁来基于此优化？
//   • MemoryUsage - 内存使用量由Go运行时管理，不需要业务层关注
//   • GoroutineCount - 协程数量由Go调度器管理，监控它有什么意义？
//
// 🎯 **性能优化的正确方式**：
// 在自运行系统中，性能优化应该：
// 1. 由算法内部自动调整
// 2. 基于系统负载自适应
// 3. 不依赖外部监控数据
//
// ⚠️ **反面教材**：
// 这种性能监控结构体是"监控驱动开发"的错误实践。
// 正确的做法是"算法驱动优化"，而不是"数据驱动监控"。

// === 细粒度接口恢复（独立接口） ===

// DistanceCalculator 距离计算器接口
type DistanceCalculator interface {
	// Distance 计算两个节点之间的XOR距离
	Distance(a, b peer.ID) []byte

	// DistanceToKey 计算节点到密钥的距离
	DistanceToKey(peerID peer.ID, key []byte) []byte

	// Compare 比较两个距离
	Compare(a, b []byte) int

	// CommonPrefixLen 计算公共前缀长度
	CommonPrefixLen(a, b []byte) int
}

// PeerSelector 节点选择器接口
type PeerSelector interface {
	// SelectPeers 选择节点
	SelectPeers(candidates []peer.ID, count int, criteria *SelectionCriteria) []peer.ID

	// RankPeers 对节点进行排序
	RankPeers(peers []peer.ID, targetKey []byte) []peer.ID

	// FilterPeers 过滤节点
	FilterPeers(peers []peer.ID, filter PeerFilter) []peer.ID
}

// RoutingTableManager 路由表管理器接口
type RoutingTableManager interface {
	// GetRoutingTable 获取路由表
	GetRoutingTable() *RoutingTable

	// AddPeer 添加节点
	AddPeer(ctx context.Context, addrInfo peer.AddrInfo) (bool, error)

	// RemovePeer 移除节点
	RemovePeer(peer.ID) error

	// FindClosestPeers 查找最近的节点
	FindClosestPeers(target []byte, count int) []peer.ID

	// RecordPeerSuccess 记录节点成功交互（恢复健康分）
	RecordPeerSuccess(peerID peer.ID)

	// RecordPeerFailure 记录节点失败交互（累计失败分）
	RecordPeerFailure(peerID peer.ID)

	// QuarantineIncompatiblePeer 直接隔离不兼容的节点（不走渐进式降级）
	//
	// 🆕 2025-12-18：用于处理明确不支持 WES 协议的节点
	//
	// 与 RecordPeerFailure 的区别：
	// - RecordPeerFailure: 需要多次失败才会进入隔离状态（渐进式降级）
	// - QuarantineIncompatiblePeer: 直接进入隔离状态（协议不兼容是明确的不兼容）
	//
	// 参数：
	// - peerID: 要隔离的节点 ID
	// - reason: 隔离原因（用于日志）
	QuarantineIncompatiblePeer(peerID peer.ID, reason string)

	// 🆕 IsReady 检查就绪状态（运行中且已初始化）
	IsReady() bool

	// ❌ **已删除：GetHealthMonitorStats() - 遗漏的监控接口**
	//
	// 🚨 **删除原因**：
	// 这个方法在之前的清理中被遗漏了，它和其他健康监控接口一样没有价值：
	//   • 返回HealthMonitorStats结构体（已删除）
	//   • 试图暴露健康监控的内部统计数据
	//   • 在自治P2P网络中完全没有消费者
	//
	// 🎯 **清理遗漏的教训**：
	// 1. 需要更仔细地检查接口定义的完整性
	// 2. 删除结构体时要同时删除所有引用它的方法
	// 3. 监控接口可能分散在不同的接口定义中
}

// === 向后兼容接口别名 ===

// RoutingTableEvents 路由表事件接口别名（向后兼容）
type RoutingTableEvents = RoutingTableEventHandler

// RoutingTableMetrics 路由表指标
// ❌ **已删除：RoutingTableMetrics - 极度复杂的路由监控结构**
//
// 🚨 **删除原因**：
// RoutingTableMetrics是过度监控的典型代表，包含了24个监控字段！
// 每个字段都没有明确的使用场景：
//   • TotalPeers/ConnectedPeers - 这些数量给谁看？基于此做什么决策？
//   • AverageLatency/MaxLatency/MinLatency - 延迟监控后要做什么优化？
//   • SuccessfulQueries/FailedQueries - 查询统计的消费者是谁？
//   • ChurnRate/ConnectivityRatio - 这些复杂指标的实际意义是什么？
//   • BucketUtilization/NetworkSize - 网络规模是DHT自适应的，为什么要监控？
//
// 🎯 **监控过度症**：
// 这种结构体代表了"监控一切"的错误理念。真正的自治系统应该：
// 1. 内部算法自适应网络变化
// 2. 自动处理节点故障和网络分区
// 3. 基于算法而非监控数据做决策
//
// ❌ **已删除：HealthMonitorStats - 健康监控的错误实践**
//
// 🚨 **删除原因**：
// HealthMonitorStats试图监控节点健康的每个细节，但在自治P2P网络中毫无意义：
//   • MonitoredPeers/HealthyPeers/UnhealthyPeers - 健康节点的定义是什么？阈值如何确定？
//   • AverageResponseTime/FailureRate - 这些指标用于什么决策？
//   • TotalPings/SuccessfulPings/FailedPings - Ping统计的目的是什么？
//
// 🎯 **P2P网络的正确理念**：
// 在分布式P2P网络中，节点健康应该：
// 1. 由路由算法自动评估
// 2. 坏节点自动被替换
// 3. 不需要复杂的健康评分体系
//
// ⚠️ **架构教训**：
// 这些庞大的监控结构体说明了一个问题：当你需要监控如此多的指标时，
// 说明系统设计本身可能就有问题。良好的自治系统应该是"黑盒"式的。

// KBucketProvider K桶提供者接口
type KBucketProvider interface {
	// GetDistanceCalculator 获取距离计算器
	GetDistanceCalculator() DistanceCalculator

	// GetPeerSelector 获取节点选择器
	GetPeerSelector() PeerSelector

	// CreateRoutingTableComponents 创建路由表组件
	CreateRoutingTableComponents() (DistanceCalculator, PeerSelector)

	// GetConfig 获取配置
	GetConfig() KBucketConfig
}

// KBucketConfig K桶配置接口
type KBucketConfig interface {
	// GetBucketSize 获取桶大小
	GetBucketSize() int

	// GetMaxLatency 获取最大延迟
	GetMaxLatency() time.Duration

	// GetRefreshInterval 获取刷新间隔
	GetRefreshInterval() time.Duration

	// GetUsefulnessGracePeriod 获取有用性宽限期
	GetUsefulnessGracePeriod() time.Duration

	// IsDiversityFilterEnabled 是否启用多样性过滤
	IsDiversityFilterEnabled() bool

	// GetMaxPeersPerCpl 获取每个CPL的最大节点数
	GetMaxPeersPerCpl() int

	// GetFailureThreshold 获取失败阈值（触发Suspect状态）
	GetFailureThreshold() int

	// GetQuarantineDuration 获取隔离时长
	GetQuarantineDuration() time.Duration

	// GetMinPeersPerBucket 获取每个桶的最小节点数
	GetMinPeersPerBucket() int

	// GetProbeInterval 获取探测间隔
	GetProbeInterval() time.Duration

	// GetHealthDecayHalfLife 获取健康分衰减半衰期
	GetHealthDecayHalfLife() time.Duration

	// GetMaintainInterval 获取维护协程运行间隔
	GetMaintainInterval() time.Duration

	// GetCleanupGracePeriod 获取清理宽限期（断连/长期无用节点进入待清理/待探测前的最小保留时间）
	// P0-010：避免清理过于激进导致 K 桶被逐步掏空。
	GetCleanupGracePeriod() time.Duration

	// GetLowHealthThreshold 获取低健康分阈值（低于该阈值才会被纳入待清理/待探测流程）
	// P0-010：降低误判，避免因少量历史失败而过早标记清理。
	GetLowHealthThreshold() float64

	// GetAddrProtectionGracePeriod 获取地址保护宽限期（仍有地址的 peer 进入待清理/待探测前的最小保留时间）
	// P0-010：为仍有地址的 peer 提供更长的保护窗口（如 30 分钟），避免短期网络故障导致误清理。
	GetAddrProtectionGracePeriod() time.Duration

	// === Phase 2：清理前探测机制配置 ===

	// IsPreCleanupProbeEnabled 是否启用清理前探测（默认true）
	IsPreCleanupProbeEnabled() bool

	// GetProbeTimeout 获取探测超时时间（默认5秒）
	GetProbeTimeout() time.Duration

	// GetProbeFailThreshold 获取探测失败阈值（连续失败多少次才确认清理，默认3次）
	GetProbeFailThreshold() int

	// GetProbeIntervalMin 获取最小探测间隔（避免频繁探测，默认30秒）
	GetProbeIntervalMin() time.Duration

	// GetProbeMaxConcurrent 获取最大并发探测数（避免探测风暴，默认5）
	GetProbeMaxConcurrent() int
}
