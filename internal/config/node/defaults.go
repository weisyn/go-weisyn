// Package node provides default configuration values for P2P network nodes.
package node

import "time"

// P2P网络默认配置值
const (
	// === 基础网络连接配置 ===

	// defaultMinPeers 最小连接节点数设为8
	// 原因：确保网络有足够的冗余度，防止因个别节点掉线导致的网络分割
	defaultMinPeers = 8

	// defaultMaxPeers 最大连接节点数设为50
	// 原因：平衡网络连通性和资源消耗，50个连接足以维持良好的网络拓扑
	defaultMaxPeers = 50

	// defaultLowWater 连接管理低水位设为10
	// 原因：当连接数低于此值时开始主动连接新节点，确保网络连通性
	defaultLowWater = 10

	// defaultHighWater 连接管理高水位设为25
	// 原因：当连接数超过此值时开始清理低质量连接，避免资源浪费
	defaultHighWater = 25

	// defaultGracePeriod 连接优雅关闭期设为20秒
	// 原因：给正在进行的传输足够时间完成，避免数据丢失
	defaultGracePeriod = 20 * time.Second

	// === 资源管理配置 ===

	// defaultMemoryLimitMB 内存限制设为512MB
	// 原因：为P2P模块预留足够内存处理连接、缓存和消息队列
	defaultMemoryLimitMB = 512

	// defaultMaxFileDescriptors 最大文件描述符数设为4096
	// 原因：每个网络连接需要1个文件描述符，4096支持大量并发连接
	defaultMaxFileDescriptors = 4096

	// === 传输协议配置 ===

	// defaultEnableTCP 默认启用TCP传输
	// 原因：TCP是最成熟稳定的传输协议，具有广泛的网络支持
	defaultEnableTCP = true

	// defaultEnableQUIC 默认启用QUIC传输
	// 原因：QUIC提供更好的性能和安全性，支持连接迁移
	defaultEnableQUIC = true

	// defaultEnableWebSocket 默认不启用WebSocket
	// 原因：WebSocket主要用于浏览器环境，服务器节点通常不需要
	defaultEnableWebSocket = false

	// === 连通性默认开关 ===
	// 优先直接连通与打洞；AutoRelay 默认关闭，按需启用
	defaultEnableNATPort              = true
	defaultEnableDCUtR                = true
	defaultEnableAutoRelay            = false
	defaultEnableRelayTransport       = false
	defaultEnableRelayService         = false
	defaultEnableAutoNATClient        = false
	defaultEnableAutoNATService       = false
	defaultAutoRelayDynamicCandidates = 16
	defaultForceReachability          = ""

	// === 安全协议默认开关 ===
	// 默认同时启用 TLS 与 Noise
	defaultEnableTLS   = true
	defaultEnableNoise = true

	// === 多路复用器配置 ===

	// defaultEnableYamux 默认启用Yamux多路复用器
	// 原因：Yamux是libp2p的标准多路复用器，稳定可靠
	defaultEnableYamux = true

	// defaultYamuxWindowSize Yamux窗口大小设为256KB
	// 原因：平衡内存使用和吞吐量，256KB适合大多数应用场景
	defaultYamuxWindowSize = 256

	// defaultYamuxMaxStreams Yamux最大流数设为65536
	// 原因：支持大量并发流，满足复杂应用的需求
	defaultYamuxMaxStreams = 65536

	// defaultYamuxConnectionTimeout Yamux连接超时设为30秒
	// 原因：给连接建立足够时间，避免因网络延迟导致的失败
	defaultYamuxConnectionTimeout = 30 * time.Second

	// === 诊断配置 ===

	// defaultDiagnosticsPort 诊断端口设为28686（WES 端口规范）
	// 原因：统一落在 WES 端口段，避免与常用端口冲突（见 docs/PORTS_SPEC.md）
	defaultDiagnosticsPort = 28686

	// === mDNS发现配置 ===

	// defaultMDNSServiceName 默认mDNS服务名称
	// 原因：使用项目名称作为服务标识，便于网络中的节点相互发现
	defaultMDNSServiceName = "weisyn-node"

	// defaultMDNSConnectTimeout mDNS连接超时增加到20秒
	// 原因：应对网络延迟和连接协商时间，减少连接失败
	defaultMDNSConnectTimeout = 20 * time.Second

	// defaultMDNSRetryLimit mDNS重试次数增加到3
	// 原因：增加重试次数提高连接成功率，特别是在网络不稳定环境下
	defaultMDNSRetryLimit = 3

	// === DHT配置 ===

	// defaultDHTMode DHT模式设为auto
	// 原因：自动模式根据网络环境选择最合适的DHT模式
	defaultDHTMode = "auto"

	// defaultDHTProtocolPrefix DHT协议前缀设为/weisyn
	// 原因：使用项目特定的协议前缀，避免与其他libp2p网络冲突
	defaultDHTProtocolPrefix = "/weisyn"

	// === 发现时间配置 ===

	// defaultDiscoveryInterval 节点发现间隔减少到20秒
	// 原因：更频繁的发现可以改善网络连接质量，特别是启动阶段
	defaultDiscoveryInterval = 20 * time.Second

	// defaultAdvertiseInterval 节点广播间隔设为300秒（5分钟）
	// 原因：定期广播自己的存在，5分钟间隔避免过于频繁的网络广播
	defaultAdvertiseInterval = 300 * time.Second

	// === 发现行为高级配置 ===

	// defaultDiscoveryExpectedMinPeers DHT 期望的最小 peers 数量，用于引导“发现足够节点”判断
	// 原因：在公网上通常希望至少连到若干节点后再进入稳定阶段；单节点/小网络可通过配置覆盖为0。
	defaultDiscoveryExpectedMinPeers = 3

	// defaultDiscoverySingleNodeMode 单节点/孤立网络模式开关
	// 原因：在已知只有一个节点或极小网络时，可以显式关闭 DHT 循环，降低资源消耗。
	defaultDiscoverySingleNodeMode = false
)

// defaultListenAddresses 默认监听地址
// 同时监听IPv4和IPv6的TCP和QUIC端口28683，确保最大兼容性（WES 端口规范）
var defaultListenAddresses = []string{
	"/ip4/0.0.0.0/tcp/28683",         // IPv4 TCP监听，兼容性最好
	"/ip6/::/tcp/28683",              // IPv6 TCP监听，支持双栈网络
	"/ip4/0.0.0.0/udp/28683/quic-v1", // IPv4 QUIC监听，现代高性能协议
	"/ip6/::/udp/28683/quic-v1",      // IPv6 QUIC监听，支持双栈QUIC
}

// defaultBootstrapPeers 默认引导节点列表
//
// ⚠️ WES 不再内置任何公共/第三方引导节点。
// - 原因：端口与网络身份必须由链配置显式定义，避免“误连外部网络/引导到未知节点”的风险；
// - 约束：公链/联盟链/私链都应在链配置中明确配置本链的基础设施节点作为 bootstrap peers。
var defaultBootstrapPeers = []string{}
