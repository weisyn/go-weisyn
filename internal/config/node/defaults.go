package node

import "time"

// P2P网络默认配置值
// 这些默认值基于生产环境的最佳实践和libp2p的推荐配置
const (
	// === 基础网络连接配置 ===

	// defaultMinPeers 最小连接节点数设为8
	// 原因：确保网络有足够的冗余度，防止因个别节点掉线导致的网络分割
	// 8个连接可以提供良好的网络健壮性，同时不会造成过多的资源消耗
	defaultMinPeers = 8

	// defaultMaxPeers 最大连接节点数设为50
	// 原因：平衡网络连通性和资源消耗，50个连接足以维持良好的网络拓扑
	// 过多连接会增加带宽和内存消耗，过少连接会影响网络健壮性
	defaultMaxPeers = 50

	// defaultLowWater 连接管理低水位设为10
	// 原因：当连接数低于此值时开始主动连接新节点，确保网络连通性
	// 设为略高于最小连接数，提供缓冲空间
	defaultLowWater = 10

	// defaultHighWater 连接管理高水位设为25
	// 原因：当连接数超过此值时开始清理低质量连接，避免资源浪费
	// 设为最大连接数的一半，提供足够的连接管理弹性
	defaultHighWater = 25

	// defaultGracePeriod 连接优雅关闭期设为20秒
	// 原因：给正在进行的传输足够时间完成，避免数据丢失
	// 20秒平衡了用户体验和资源占用，符合网络应用的常见实践
	defaultGracePeriod = 20 * time.Second

	// === 资源管理配置 ===

	// defaultMemoryLimitMB 内存限制设为512MB
	// 原因：为P2P模块预留足够内存处理连接、缓存和消息队列
	// 512MB可以支持大量并发连接而不会耗尽系统内存
	defaultMemoryLimitMB = 512

	// defaultMaxFileDescriptors 最大文件描述符数设为4096
	// 原因：每个网络连接需要1个文件描述符，4096支持大量并发连接
	// 符合Linux系统的常见ulimit设置，避免文件描述符耗尽
	defaultMaxFileDescriptors = 4096

	// === 传输协议配置 ===

	// defaultEnableTCP 默认启用TCP传输
	// 原因：TCP是最成熟稳定的传输协议，具有广泛的网络支持
	// 确保与不同网络环境的兼容性，是P2P网络的基础传输
	defaultEnableTCP = true

	// defaultEnableQUIC 默认启用QUIC传输
	// 原因：QUIC提供更好的性能和安全性，支持连接迁移
	// 现代P2P网络的推荐传输协议，可以显著提升用户体验
	defaultEnableQUIC = true

	// defaultEnableWebSocket 默认不启用WebSocket
	// 原因：WebSocket主要用于浏览器环境，服务器节点通常不需要
	// 避免不必要的端口占用和安全风险
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
	// 支持流优先级和流量控制，适合P2P应用场景
	defaultEnableYamux = true

	// defaultYamuxWindowSize Yamux窗口大小设为256KB
	// 原因：平衡内存使用和吞吐量，256KB适合大多数应用场景
	// 足够大以支持高吞吐量传输，又不会占用过多内存
	defaultYamuxWindowSize = 256

	// defaultYamuxMaxStreams Yamux最大流数设为65536
	// 原因：支持大量并发流，满足复杂应用的需求
	// 65536是Yamux协议的理论上限，提供最大灵活性
	defaultYamuxMaxStreams = 65536

	// defaultYamuxConnectionTimeout Yamux连接超时设为30秒
	// 原因：给连接建立足够时间，避免因网络延迟导致的失败
	// 30秒符合网络应用的常见超时设置
	defaultYamuxConnectionTimeout = 30 * time.Second

	// === 诊断配置 ===

	// defaultDiagnosticsPort 诊断端口设为8080
	// 原因：8080是常用的HTTP替代端口，易于记忆和配置
	// 避免与其他常用端口冲突
	defaultDiagnosticsPort = 8080

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
	// 新节点作为客户端，稳定节点自动升级为服务器
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
)

// defaultListenAddresses 默认监听地址
// 同时监听IPv4和IPv6的TCP和QUIC端口4001，确保最大兼容性
// 4001是libp2p的标准端口，避免与其他服务冲突
var defaultListenAddresses = []string{
	"/ip4/0.0.0.0/tcp/4001",         // IPv4 TCP监听，兼容性最好
	"/ip6/::/tcp/4001",              // IPv6 TCP监听，支持双栈网络
	"/ip4/0.0.0.0/udp/4001/quic-v1", // IPv4 QUIC监听，现代高性能协议
	"/ip6/::/udp/4001/quic-v1",      // IPv6 QUIC监听，支持双栈QUIC
}

// defaultBootstrapPeers 默认引导节点列表
// 这些是官方libp2p引导节点，用于新节点初始连接到P2P网络
// 包含DNS地址解析和直连IP地址，提供冗余的连接方式
var defaultBootstrapPeers = []string{
	// libp2p官方引导节点 - 使用DNS地址解析，具有更好的可维护性
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
	"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",

	// 直连IP引导节点 - 提供备用连接方式，在DNS解析失败时使用
	"/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
}
