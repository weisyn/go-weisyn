package interfaces

import (
	"context"

	"github.com/libp2p/go-libp2p/core/metrics"
	libpeer "github.com/libp2p/go-libp2p/core/peer"

	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
)

// InternalP2P 内部 P2P 接口
//
// - 嵌入公共接口 p2p.Service
// - 未来如有内部控制/调试方法，可在此添加，不暴露到 pkg 层
type InternalP2P interface {
	p2pi.Service
}

// BandwidthProvider 提供带宽计数器的内部接口
//
// 用于解耦子模块对 host 包的直接依赖，通过接口获取带宽统计能力
type BandwidthProvider interface {
	// BandwidthReporter 返回带宽统计 Reporter
	BandwidthReporter() metrics.Reporter
}

// ResourceManagerInspector 提供 ResourceManager 限额视图的内部接口
//
// 用于解耦子模块对 host 包的直接依赖，通过接口获取资源管理限额信息
type ResourceManagerInspector interface {
	// ResourceManagerLimits 返回 ResourceManager 限额信息（可直接序列化为 JSON 的 map）
	ResourceManagerLimits() map[string]interface{}
}

// RendezvousRouting 提供基于 DHT 的 Rendezvous 发现和状态观察能力的内部接口
//
// 用于解耦 discovery / diagnostics 对具体 DHT 实现（kad-dht）的依赖，由 routing.Service 实现
type RendezvousRouting interface {
	// AdvertiseAndFindPeers 在指定命名空间下执行广告与发现，返回对端 AddrInfo channel
	AdvertiseAndFindPeers(ctx context.Context, ns string) (<-chan libpeer.AddrInfo, error)

	// FindPeer 通过DHT查找指定peer的地址信息
	FindPeer(ctx context.Context, id libpeer.ID) (libpeer.AddrInfo, error)

	// RoutingTableSize 返回当前路由表大小（不可用时返回 0）
	RoutingTableSize() int

	// Offline 返回当前 Routing 是否处于离线模式（例如未启用 DHT 或初始化失败）
	Offline() bool
}

// WESPeerValidator 提供 WES 业务节点验证能力的内部接口
//
// 🆕 用于解耦连接管理、DHT 路由过滤等模块对具体验证逻辑的依赖
//
// 背景：
// - 阿里云节点 Goroutine 峰值 34,832（19x 本地），大量非 WES 节点涌入
// - 需要统一的 WES 节点验证接口，用于：
//   1. 连接管理器权重设置（WESConnNotifee）
//   2. DHT 路由表过滤（RoutingTableFilter）
//   3. K 桶节点验证（validateWESPeer）
//
// 参考：LIBP2P_GOROUTINE_ANALYSIS.md
type WESPeerValidator interface {
	// IsWESPeer 判断指定 peer 是否是 WES 业务节点
	//
	// 判断标准：协议列表中包含 "/weisyn/" 前缀的协议
	//
	// 返回值：
	//   - bool: 是否是 WES 节点
	//   - error: 验证过程中的错误（如 peerstore 不可用）
	IsWESPeer(ctx context.Context, peerID libpeer.ID) (bool, error)
}
