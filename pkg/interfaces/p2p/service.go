// Package p2p provides P2P node runtime interfaces.
package p2p

import (
	"context"

	libhost "github.com/libp2p/go-libp2p/core/host"
	libpeer "github.com/libp2p/go-libp2p/core/peer"
)

// Service P2P 节点运行时统一接口（对外暴露的唯一对象）
//
// 设计目标：
// - 一个组件一个接口对象：所有 P2P 能力通过 Service 统一暴露
// - 子系统化：Swarm / Routing / Discovery / Connectivity / Diagnostics 各司其职
// - 对标 Kubo：整体风格类似 IPFS Kubo 的 libp2p + Swarm + DHT + Discovery 子系统
type Service interface {
	Host() libhost.Host
	Swarm() Swarm
	Routing() Routing
	Discovery() Discovery
	Connectivity() Connectivity
	Diagnostics() Diagnostics
}

// ============= Swarm =============

// ConnInfo 连接信息
type ConnInfo struct {
	Peer        libpeer.ID
	Direction   string // "inbound" | "outbound"
	RemoteAddr  string // 远程地址
	LocalAddr   string // 本地地址
	OpenedAt    int64  // 建立时间（Unix 时间戳）
	StreamCount int    // 当前流数量
}

// SwarmStats Swarm 统计信息
type SwarmStats struct {
	NumPeers        int     // 当前连接的 Peer 数量
	NumConns        int     // 当前连接数
	NumStreams      int     // 当前流数量
	InboundRateBps  float64 // 入站带宽速率（字节/秒）
	OutboundRateBps float64 // 出站带宽速率（字节/秒）
	InboundTotal    int64   // 入站总字节数
	OutboundTotal   int64   // 出站总字节数
	InboundConns    int     // 入站连接数
	OutboundConns   int     // 出站连接数
}

// Swarm Swarm 视图 + Dial 能力
//
// 对标 Kubo Swarm：管理所有连接、流、带宽统计
type Swarm interface {
	// Peers 返回当前连接的 Peer 列表
	Peers() []libpeer.AddrInfo

	// Connections 返回当前连接信息
	Connections() []ConnInfo

	// Stats 返回 Swarm 统计信息
	Stats() SwarmStats

	// Dial 连接到指定 Peer
	Dial(ctx context.Context, info libpeer.AddrInfo) error
}

// ============= Routing =============

// DHTMode DHT 模式
type DHTMode string

const (
	DHTModeAuto   DHTMode = "auto"   // 自动模式
	DHTModeServer DHTMode = "server" // 服务器模式（参与路由表存储）
	DHTModeClient DHTMode = "client" // 客户端模式（仅查询，不存储）
	DHTModeLAN    DHTMode = "lan"    // 局域网模式
)

// Routing PeerRouting 能力
//
// 对标 Kubo Routing：基于 DHT 的 Peer 路由与发现
type Routing interface {
	// FindPeer 查找指定 PeerID 的地址信息
	FindPeer(ctx context.Context, id libpeer.ID) (libpeer.AddrInfo, error)

	// FindClosestPeers 查找最接近指定 key 的 Peer 列表
	FindClosestPeers(ctx context.Context, key []byte, count int) (<-chan libpeer.AddrInfo, error)

	// Bootstrap 执行 DHT Bootstrap
	Bootstrap(ctx context.Context) error

	// Mode 返回当前 DHT 模式
	Mode() DHTMode
}

// ============= Discovery =============

// Discovery 发现控制
//
// 统一调度 Bootstrap / mDNS / Rendezvous 等发现插件
type Discovery interface {
	// Start 启动发现服务
	Start(ctx context.Context) error

	// Stop 停止发现服务
	Stop(ctx context.Context) error

	// Trigger 触发一次发现（reason 用于日志）
	Trigger(reason string)
}

// ============= Connectivity =============

// Reachability 可达性状态
type Reachability string

const (
	ReachabilityUnknown Reachability = "unknown" // 未知
	ReachabilityPublic  Reachability = "public"  // 公网可达
	ReachabilityPrivate Reachability = "private" // 私网可达
)

// Profile P2P Profile（运行模式）
type Profile string

const (
	ProfileServer Profile = "server" // 全节点 / 出块节点
	ProfileClient Profile = "client" // 轻节点 / SDK
	ProfileLAN    Profile = "lan"    // 局域网测试
)

// Connectivity 连通性控制与状态
//
// 管理 NAT / AutoNAT / Relay / DCUTR 等连通性增强能力
type Connectivity interface {
	// Reachability 返回当前可达性状态
	Reachability() Reachability

	// Profile 返回当前 P2P Profile
	Profile() Profile
}

// ============= Diagnostics =============

// Diagnostics 诊断与指标
//
// 暴露 HTTP 诊断端点与 Prometheus 指标
type Diagnostics interface {
	// HTTPAddr 返回诊断 HTTP 服务地址
	HTTPAddr() string

	// GetPeersCount 返回当前连接的 peers 数量
	GetPeersCount() int

	// GetConnectionsCount 返回当前活跃连接数
	GetConnectionsCount() int
}
