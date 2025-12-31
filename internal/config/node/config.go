package node

import (
	"time"

	"github.com/weisyn/v1/pkg/types"
)

// NodeOptions 节点网络配置选项
//
// ⚠️ **已废弃**：此配置结构仅用于配置映射（从 YAML 到 p2p.Options），不再用于运行时。
// 运行时请使用 internal/config/p2p.Options。
//
// 整个节点网络组件的统一配置入口，包含三个主要子模块的配置
type NodeOptions struct {
	// 连接管理配置 - 映射到 internal/core/p2p/connectivity
	Connectivity ConnectivityConfig `json:"connectivity"`

	// 节点发现配置 - 映射到 internal/core/p2p/discovery
	Discovery DiscoveryConfig `json:"discovery"`

	// 主机配置 - 映射到 internal/core/p2p/host
	Host HostConfig `json:"host"`
}

// ConnectivityConfig 连接管理配置
type ConnectivityConfig struct {
	// 基础连接参数
	MinPeers    int           `json:"min_peers"`    // 最小连接节点数
	MaxPeers    int           `json:"max_peers"`    // 最大连接节点数
	LowWater    int           `json:"low_water"`    // 连接管理低水位
	HighWater   int           `json:"high_water"`   // 连接管理高水位
	GracePeriod time.Duration `json:"grace_period"` // 连接优雅关闭期

	// NAT和连通性
	EnableNATPort        bool `json:"enable_nat_port"`        // 启用NAT端口映射
	EnableRelayTransport bool `json:"enable_relay_transport"` // 启用中继传输客户端
	EnableRelayService   bool `json:"enable_relay_service"`   // 启用中继服务端
	EnableAutoRelay      bool `json:"enable_auto_relay"`      // 启用自动中继
	EnableDCUtR          bool `json:"enable_dcutr"`           // 启用DCUtR打洞
	EnableAutoNATService bool `json:"enable_autonat_service"` // AutoNAT 服务端开关
	EnableAutoNATClient  bool `json:"enable_autonat_client"`  // AutoNAT 客户端开关

	// 可达性强制策略："", "public", "private"
	ForceReachability string `json:"force_reachability"`

	// 动态AutoRelay候选上限
	AutoRelayDynamicCandidates int `json:"autorelay_dynamic_candidates"`

	// Relay Service 资源配置（仅当 EnableRelayService=true 时生效）
	RelayMaxReservations int `json:"relay_max_reservations"` // 最大预约数（默认 128）
	RelayMaxCircuits     int `json:"relay_max_circuits"`     // 每个 peer 的最大电路数（默认 16）
	RelayBufferSize      int `json:"relay_buffer_size"`      // 中继连接缓冲区大小（默认 2048）

	// 资源限制
	Resources ResourceConfig `json:"resources"` // 资源管理配置
}

// DiscoveryConfig 节点发现配置
type DiscoveryConfig struct {
	// 引导节点
	BootstrapPeers []string `json:"bootstrap_peers"` // 引导节点列表

	// 静态中继节点
	StaticRelayPeers []string `json:"static_relay_peers"`

	// mDNS发现
	MDNS MDNSConfig `json:"mdns"` // mDNS配置

	// DHT发现
	DHT DHTConfig `json:"dht"` // DHT配置

	// 发现时间参数
	DiscoveryInterval time.Duration `json:"discovery_interval"` // 发现间隔
	AdvertiseInterval time.Duration `json:"advertise_interval"` // 广播间隔

	// Rendezvous 命名空间（可选，默认"weisyn"）
	RendezvousNamespace string `json:"rendezvous_namespace"`

	// 高级 DHT 发现行为配置
	// - ExpectedMinPeers: 期望的最小 DHT peers 数量，用于从 Bootstrap 过渡到 Steady 阶段的阈值；
	//   典型公网环境建议保持默认值（例如3），单节点/极小网络可配置为0。
	// - SingleNodeMode: 单节点/孤立网络模式开关，为 true 时可以显式关闭 DHT 发现循环。
	ExpectedMinPeers int  `json:"expected_min_peers"`
	SingleNodeMode   bool `json:"single_node_mode"`

	// ============================
	// ForceConnect（GossipSub 拉活）
	// ============================
	//
	// 说明：
	// - WES 网络中存在大量“非业务的公网 libp2p 节点”，不应对其做全量主动连接；
	// - 这里提供可控的“拉活”机制：业务关键节点优先，其余节点仅抽样辅助公网发现/mesh形成。
	BusinessCriticalPeerIDs []string          `json:"business_critical_peer_ids"` // 业务关键节点 PeerID 列表（个位数）
	ForceConnect            ForceConnectConfig `json:"force_connect"`              // 主动连接控制参数
}

// ForceConnectConfig 主动连接（拉活）配置
type ForceConnectConfig struct {
	Enabled            *bool         `json:"enabled"`              // 是否启用（nil=默认启用，false=禁用）
	Cooldown           time.Duration `json:"cooldown"`             // 触发冷却（默认2m）
	Concurrency        int           `json:"concurrency"`          // 并发上限（默认15）
	BudgetPerRound     int           `json:"budget_per_round"`     // 每轮总预算（默认50）
	Tier2SampleBudget  int           `json:"tier2_sample_budget"`  // Tier2 抽样预算（默认20）
	Timeout            time.Duration `json:"timeout"`              // 单peer拨号超时（默认10s）
}

// HostConfig 主机配置
type HostConfig struct {
	// 监听地址
	ListenAddresses []string `json:"listen_addresses"` // 监听地址列表

	// 地址配置
	AdvertisePrivateAddrs bool `json:"advertise_private_addrs"` // 是否公告私网地址

	// 身份配置（用于固定 PeerID）
	Identity IdentityConfig `json:"identity"`

	// 传输协议配置
	Transport TransportConfig `json:"transport"` // 传输协议配置

	// 多路复用器配置
	Muxer MuxerConfig `json:"muxer"` // 多路复用器配置

	// 安全协议配置
	Security SecurityConfig `json:"security"`

	// 地址过滤
	Gater GaterConfig `json:"gater"` // 地址过滤配置

	// 地址公告策略（可选）：用于覆盖外宣地址集合
	Announce       []string `json:"announce"`        // 完全替换的外宣地址集合
	AppendAnnounce []string `json:"append_announce"` // 追加外宣地址集合
	NoAnnounce     []string `json:"no_announce"`     // 不外宣地址/网段（支持CIDR）

	// 诊断配置
	DiagnosticsEnabled bool `json:"diagnostics_enabled"` // 是否启用诊断
	DiagnosticsPort    int  `json:"diagnostics_port"`    // 诊断端口
}

// IdentityConfig 主机身份配置
// 当未提供私钥且指定的密钥文件不存在时，系统将自动生成并持久化一个缺省密钥
type IdentityConfig struct {
	// PrivateKey 以base64编码的libp2p私钥（crypto.MarshalPrivateKey后的结果）
	// 若提供该字段，将优先生效
	PrivateKey string `json:"private_key"`
	// KeyFile 私钥持久化文件路径（建议位于数据目录）
	// 若为空，将使用内置默认路径
	KeyFile string `json:"key_file"`
}

// SecurityConfig 安全协议配置
type SecurityConfig struct {
	EnableTLS   bool `json:"enable_tls"`
	EnableNoise bool `json:"enable_noise"`
}

// === 子配置结构定义 ===

// ResourceConfig 资源管理配置
type ResourceConfig struct {
	MemoryLimitMB      int `json:"memory_limit_mb"`      // 内存限制(MB)
	MaxFileDescriptors int `json:"max_file_descriptors"` // 最大文件描述符数
}

// MDNSConfig mDNS发现配置
type MDNSConfig struct {
	Enabled        bool          `json:"enabled"`         // 是否启用mDNS
	ServiceName    string        `json:"service_name"`    // 服务名称
	ConnectTimeout time.Duration `json:"connect_timeout"` // 连接超时
	RetryLimit     int           `json:"retry_limit"`     // 重试次数限制
}

// DHTConfig DHT配置
type DHTConfig struct {
	Enabled        bool   `json:"enabled"`         // 是否启用DHT
	Mode           string `json:"mode"`            // DHT模式：client/server/auto
	ProtocolPrefix string `json:"protocol_prefix"` // 协议前缀
	DataStorePath  string `json:"data_store_path"` // 数据存储路径

	// 高级配置
	EnableLANLoopback             bool `json:"enable_lan_loopback"`               // 启用LAN回环
	EnableOptimisticProvide       bool `json:"enable_optimistic_provide"`         // 启用乐观提供
	OptimisticProvideJobsPoolSize int  `json:"optimistic_provide_jobs_pool_size"` // 乐观提供作业池大小
}

// TransportConfig 传输协议配置
type TransportConfig struct {
	EnableTCP       bool `json:"enable_tcp"`       // 是否启用TCP
	EnableQUIC      bool `json:"enable_quic"`      // 是否启用QUIC
	EnableWebSocket bool `json:"enable_websocket"` // 是否启用WebSocket
}

// MuxerConfig 多路复用器配置
type MuxerConfig struct {
	EnableYamux            bool          `json:"enable_yamux"`             // 是否启用Yamux
	YamuxWindowSize        int           `json:"yamux_window_size"`        // Yamux窗口大小
	YamuxMaxStreams        int           `json:"yamux_max_streams"`        // Yamux最大流数
	YamuxConnectionTimeout time.Duration `json:"yamux_connection_timeout"` // Yamux连接超时
}

// GaterConfig 地址过滤配置
type GaterConfig struct {
	AllowedPrefixes []string `json:"allowed_prefixes"` // 允许的地址前缀
	BlockedPrefixes []string `json:"blocked_prefixes"` // 阻止的地址前缀
}

// Config 节点网络配置实现
type Config struct {
	options *NodeOptions
}

// New 创建节点网络配置实现
func New(userConfig *types.UserNodeConfig) *Config {
	// 1. 先创建完整的默认配置
	defaultOptions := createDefaultNodeOptions()

	// 2. 如果有用户配置，则转换并覆盖默认配置
	if userConfig != nil {
		convertAndMergeUserConfig(defaultOptions, userConfig)
	}

	return &Config{
		options: defaultOptions,
	}
}

// createDefaultNodeOptions 创建默认节点配置
func createDefaultNodeOptions() *NodeOptions {
	return &NodeOptions{
		Connectivity: ConnectivityConfig{
			MinPeers:                   defaultMinPeers,
			MaxPeers:                   defaultMaxPeers,
			LowWater:                   defaultLowWater,
			HighWater:                  defaultHighWater,
			GracePeriod:                defaultGracePeriod,
			EnableNATPort:              defaultEnableNATPort,
			EnableRelayTransport:       defaultEnableRelayTransport,
			EnableRelayService:         defaultEnableRelayService,
			EnableAutoRelay:            defaultEnableAutoRelay,
			EnableDCUtR:                defaultEnableDCUtR,
			EnableAutoNATService:       defaultEnableAutoNATService,
			EnableAutoNATClient:        defaultEnableAutoNATClient,
			ForceReachability:          defaultForceReachability,
			AutoRelayDynamicCandidates: defaultAutoRelayDynamicCandidates,
			Resources: ResourceConfig{
				MemoryLimitMB:      defaultMemoryLimitMB,
				MaxFileDescriptors: defaultMaxFileDescriptors,
			},
		},
		Discovery: DiscoveryConfig{
			BootstrapPeers:    append([]string{}, defaultBootstrapPeers...), // 复制切片
			MDNS:              MDNSConfig{Enabled: true, ServiceName: defaultMDNSServiceName, ConnectTimeout: defaultMDNSConnectTimeout, RetryLimit: defaultMDNSRetryLimit},
			DHT:               DHTConfig{Enabled: true, Mode: defaultDHTMode, ProtocolPrefix: defaultDHTProtocolPrefix},
			DiscoveryInterval: defaultDiscoveryInterval,
			AdvertiseInterval: defaultAdvertiseInterval,
			ExpectedMinPeers:  defaultDiscoveryExpectedMinPeers,
			SingleNodeMode:    defaultDiscoverySingleNodeMode,
		},
		Host: HostConfig{
			ListenAddresses: append([]string{}, defaultListenAddresses...), // 复制切片
			Transport:       TransportConfig{EnableTCP: defaultEnableTCP, EnableQUIC: defaultEnableQUIC, EnableWebSocket: defaultEnableWebSocket},
			Muxer:           MuxerConfig{EnableYamux: defaultEnableYamux, YamuxWindowSize: defaultYamuxWindowSize, YamuxMaxStreams: defaultYamuxMaxStreams, YamuxConnectionTimeout: defaultYamuxConnectionTimeout},
			Security:        SecurityConfig{EnableTLS: defaultEnableTLS, EnableNoise: defaultEnableNoise},
			DiagnosticsPort: defaultDiagnosticsPort,
		},
	}
}

// convertAndMergeUserConfig 将用户配置转换并合并到默认配置中
// 只处理JSON配置文件中实际出现的字段，其他字段使用defaults.go中的默认值
func convertAndMergeUserConfig(defaultOpts *NodeOptions, userConfig *types.UserNodeConfig) {
	// === 监听地址 ===
	if userConfig.ListenAddresses != nil {
		defaultOpts.Host.ListenAddresses = append([]string{}, userConfig.ListenAddresses...)
	}

	// === 引导节点 ===
	if userConfig.BootstrapPeers != nil {
		defaultOpts.Discovery.BootstrapPeers = append([]string{}, userConfig.BootstrapPeers...)
	}

	// === 网络发现和功能开关（JSON配置字段） ===
	if userConfig.EnableMDNS != nil {
		defaultOpts.Discovery.MDNS.Enabled = *userConfig.EnableMDNS
	}
	if userConfig.EnableDHT != nil {
		defaultOpts.Discovery.DHT.Enabled = *userConfig.EnableDHT
	}
	if userConfig.ExpectedMinPeers != nil {
		defaultOpts.Discovery.ExpectedMinPeers = *userConfig.ExpectedMinPeers
	}
	if userConfig.SingleNodeMode != nil {
		defaultOpts.Discovery.SingleNodeMode = *userConfig.SingleNodeMode
	}
	if userConfig.EnableNATPort != nil {
		defaultOpts.Connectivity.EnableNATPort = *userConfig.EnableNATPort
	}
	if userConfig.EnableAutoRelay != nil {
		defaultOpts.Connectivity.EnableAutoRelay = *userConfig.EnableAutoRelay
	}
	if userConfig.EnableDCUtR != nil {
		defaultOpts.Connectivity.EnableDCUtR = *userConfig.EnableDCUtR
	}

	// AutoNAT 客户端开关（用于自动检测公网/私网可达性）
	if userConfig.EnableAutoNAT != nil {
		defaultOpts.Connectivity.EnableAutoNATClient = *userConfig.EnableAutoNAT
	}

	// === P2P身份配置 ===
	if userConfig.Host != nil && userConfig.Host.Identity != nil {
		if userConfig.Host.Identity.PrivateKey != nil {
			defaultOpts.Host.Identity.PrivateKey = *userConfig.Host.Identity.PrivateKey
		}
		if userConfig.Host.Identity.KeyFile != nil {
			defaultOpts.Host.Identity.KeyFile = *userConfig.Host.Identity.KeyFile
		}
	}

	// === P2P连接门禁与地址公告配置（Gater / Announce）===
	if userConfig.Host != nil {
		// 诊断配置：是否启用诊断 HTTP 服务及端口
		if userConfig.Host.DiagnosticsEnabled != nil {
			defaultOpts.Host.DiagnosticsEnabled = *userConfig.Host.DiagnosticsEnabled
		}
		if userConfig.Host.DiagnosticsPort != nil {
			defaultOpts.Host.DiagnosticsPort = *userConfig.Host.DiagnosticsPort
		}

		// 地址公告策略：是否允许公告私网地址
		if userConfig.Host.AdvertisePrivateAddrs != nil {
			defaultOpts.Host.AdvertisePrivateAddrs = *userConfig.Host.AdvertisePrivateAddrs
		}

		if userConfig.Host.Gater != nil {
			userGater := userConfig.Host.Gater
			if userGater.Mode != nil {
				// Mode 字段暂不直接映射到 GaterConfig（当前 GaterConfig 只有 AllowedPrefixes/BlockedPrefixes）
				// 未来可以扩展 GaterConfig 以支持 Mode 字段
				// 当前根据 Mode 设置 AllowedPrefixes/BlockedPrefixes 的行为
			}
			if userGater.AllowCIDRs != nil {
				// CIDR 列表转换为 prefix 列表（libp2p 的 gater 使用 prefix）
				defaultOpts.Host.Gater.AllowedPrefixes = append([]string{}, userGater.AllowCIDRs...)
			}
			if userGater.AllowPrefixes != nil {
				defaultOpts.Host.Gater.AllowedPrefixes = append(defaultOpts.Host.Gater.AllowedPrefixes, userGater.AllowPrefixes...)
			}
			if userGater.DenyCIDRs != nil {
				defaultOpts.Host.Gater.BlockedPrefixes = append([]string{}, userGater.DenyCIDRs...)
			}
			if userGater.DenyPrefixes != nil {
				defaultOpts.Host.Gater.BlockedPrefixes = append(defaultOpts.Host.Gater.BlockedPrefixes, userGater.DenyPrefixes...)
			}
		}
	}

	// 其他所有字段（MinPeers, MaxPeers, Transport, Security等）使用defaults.go中的默认值
	// 这确保了配置只依赖JSON中实际定义的字段和系统默认值
}

// GetOptions 获取完整的节点网络配置选项
func (c *Config) GetOptions() *NodeOptions {
	return c.options
}

// GetConnectivityConfig 获取连接管理配置
func (c *Config) GetConnectivityConfig() *ConnectivityConfig {
	return &c.options.Connectivity
}

// GetDiscoveryConfig 获取发现配置
func (c *Config) GetDiscoveryConfig() *DiscoveryConfig {
	return &c.options.Discovery
}

// GetHostConfig 获取主机配置
func (c *Config) GetHostConfig() *HostConfig {
	return &c.options.Host
}
