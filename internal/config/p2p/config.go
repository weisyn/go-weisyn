package p2p

import (
	"fmt"
	"strings"
	"time"

	libpeer "github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/weisyn/v1/internal/config/node"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/types"
)

// Profile P2P 运行模式
type Profile string

const (
	ProfileServer Profile = "server" // 全节点 / 出块节点
	ProfileClient Profile = "client" // 轻节点 / SDK
	ProfileLAN    Profile = "lan"    // 局域网测试
)

// Options P2P 配置选项
//
// 从 ChainConfig 映射生成，包含所有 P2P 运行时需要的配置项
type Options struct {
	// Profile P2P 运行模式
	Profile Profile

	// 监听地址
	ListenAddrs []string

	// 引导节点
	BootstrapPeers []string

	// DHT 配置
	EnableDHT bool
	DHTMode   string // "auto" / "server" / "client" / "lan"

	// mDNS 配置
	EnableMDNS bool
	// MDNSServiceName mDNS 服务名（必须所有 LAN 节点一致才能互相发现）
	// 由 node.discovery.mdns.service_name 映射而来，通常会按 network namespace 做 qualify（例如 weisyn-node-public-testnet-demo）
	MDNSServiceName string

	// Discovery 调度配置
	DiscoveryNamespace   string        // Rendezvous 命名空间（如 "/weisyn/<networkNamespace>"）
	DiscoveryInterval    time.Duration // 发现轮询间隔
	AdvertiseInterval    time.Duration // DHT 广播间隔
	MaxDiscoveryFailures int           // 连续失败阈值

	// DHT 发现行为高级配置
	// - DiscoveryExpectedMinPeers: 期望的最小 DHT peers 数量，用于 DHT 发现状态机从 Bootstrap 过渡到 Steady 的阈值；
	//   典型公网环境建议为 3；单节点/极小网络可设置为 0。
	// - DiscoverySingleNodeMode: 单节点/孤立网络模式开关，为 true 时可以显式关闭 DHT rendezvous 循环。
	DiscoveryExpectedMinPeers int
	DiscoverySingleNodeMode   bool

	// Phase 3: 发现间隔收敛配置（不向后兼容）
	DiscoveryMaxIntervalCap   time.Duration // bootstrap调度器指数增长上限（默认2m）
	DHTSteadyIntervalCap      time.Duration // DHT steady模式间隔上限（默认2m）
	DiscoveryResetMinInterval time.Duration // 重置后最小间隔（默认30s）
	DiscoveryResetCoolDown    time.Duration // 重置冷却时间（默认10s）

	// Phase 4: 关键peer监控配置（不向后兼容）
	EnableKeyPeerMonitor    bool          // 启用关键peer监控（默认true）
	KeyPeerProbeInterval    time.Duration // 关键peer探测周期（默认60s）
	PerPeerMinProbeInterval time.Duration // 单个peer最小探测间隔（默认30s）
	ProbeTimeout            time.Duration // 探测超时时间（默认5s）
	ProbeFailThreshold      int           // 探测失败阈值（默认3）
	ProbeMaxConcurrent      int           // 最大并发探测数（默认5）
	KeyPeerSetMaxSize       int           // 关键peer集合最大大小（默认128）

	// Phase 5: GossipSub Mesh 拉活（forceConnect）配置（不向后兼容）
	//
	// 背景：
	// - 生产环境中 peerstore 可能包含大量"非业务的公网 libp2p 节点"；
	// - 若对 peerstore 做全量拨号，会造成 goroutine/内存突刺；
	// - 这里引入"业务节点优先 + 抽样辅助公网发现"的可控策略。
	BusinessCriticalPeerIDs       []string      // 业务关键节点 PeerID（个位数）
	ForceConnectEnabled           bool          // 是否启用（默认true）
	ForceConnectCooldown          time.Duration // 冷却时间（默认2m）
	ForceConnectConcurrency       int           // 并发上限（默认15）
	ForceConnectBudgetPerRound    int           // 每轮拨号总预算（默认50）
	ForceConnectTier2SampleBudget int           // Tier2（非业务libp2p节点）抽样预算（默认20）
	ForceConnectTimeout           time.Duration // 单peer拨号超时（默认10s）

	// 🆕 Phase 6: 网络超时和健康检查配置（HIGH-003 修复）
	//
	// 背景：
	// - 大量网络超时（context deadline exceeded）影响 P2P 连接稳定性
	// - 需要更灵活的超时配置和主动健康检查机制
	NetworkTimeoutConfig NetworkTimeoutConfig // 网络超时配置
	NetworkHealthConfig  NetworkHealthConfig  // 网络健康检查配置

	// Relay 配置
	EnableRelay        bool
	EnableRelayService bool

	// Relay Service 资源配置（仅当 EnableRelayService=true 时生效）
	RelayMaxReservations int // 最大预约数（默认 128）
	RelayMaxCircuits     int // 每个 peer 的最大电路数（默认 16）
	RelayBufferSize      int // 中继连接缓冲区大小（默认 2048）

	// AutoRelay 配置
	EnableAutoRelay            bool     // 启用自动中继
	StaticRelayPeers           []string // 静态中继节点列表（优先使用，否则回退到 BootstrapPeers）
	AutoRelayDynamicCandidates int      // 动态 AutoRelay 候选上限（默认 16）

	// DCUTR 配置
	EnableDCUTR bool

	// 私有网络配置
	PrivateNetwork bool
	PSKPath        string // PSK 文件路径（私有链）

	// 证书管理配置（联盟链）
	CertificateManagementCABundlePath string // CA Bundle 文件路径（联盟链）

	// 身份配置（用于固定 PeerID）
	IdentityKeyFile    string // 身份密钥文件路径
	IdentityPrivateKey string // base64编码的libp2p私钥（优先于KeyFile）

	// UserAgent 用户代理字符串（包含链身份信息）
	UserAgent string

	// 连接管理
	MinPeers    int
	MaxPeers    int
	LowWater    int           // 连接管理低水位
	HighWater   int           // 连接管理高水位
	GracePeriod time.Duration // 连接优雅关闭期

	// 传输层配置
	EnableTCP       bool
	EnableQUIC      bool
	EnableWebSocket bool

	// 安全层配置
	EnableTLS   bool
	EnableNoise bool

	// Muxer 配置
	EnableYamux            bool
	YamuxWindowSize        int // KB
	YamuxMaxStreams        int
	YamuxConnectionTimeout time.Duration

	// 地址公告配置
	AdvertisePrivateAddrs bool
	Announce              []string // 完全替换的外宣地址集合
	AppendAnnounce        []string // 追加外宣地址集合
	NoAnnounce            []string // 不外宣地址/网段（支持CIDR）

	// ConnectionGater 配置
	GaterAllowedPrefixes []string // 允许的地址前缀
	GaterBlockedPrefixes []string // 阻止的地址前缀

	// 资源管理配置
	MemoryLimitMB      int // 内存限制(MB)，0表示使用系统默认
	MaxFileDescriptors int // 最大文件描述符数，0表示使用系统默认

	// NAT / 可达性 / AutoNAT 配置
	EnableNATPortMap     bool   // 启用 NAT 端口映射（UPnP/NAT-PMP）
	ForceReachability    string // "", "public", "private" —— 强制可达性策略
	EnableAutoNATClient  bool   // AutoNAT 客户端开关（本节点自测可达性）
	EnableAutoNATService bool   // AutoNAT 服务端开关（为其他节点检测）

	// 诊断配置
	DiagnosticsEnabled bool
	DiagnosticsAddr    string

	// 地址管理器配置
	AddrManager AddrManagerConfig
}

// AddrManagerConfig 地址管理器配置
type AddrManagerConfig struct {
	Enabled              bool          // 启用地址管理器（默认true）
	DHTAddrTTL           time.Duration // DHT发现地址TTL（默认30分钟）
	ConnectedAddrTTL     time.Duration // 连接成功地址TTL（默认24小时）
	FailedAddrTTL        time.Duration // 连接失败地址TTL（默认5分钟）
	RefreshInterval      time.Duration // 地址刷新周期（默认10分钟）
	RefreshThreshold     time.Duration // 地址刷新阈值（默认5分钟）
	MaxConcurrentLookups int           // 最大并发查询数（默认10）
	LookupTimeout        time.Duration // 查询超时时间（默认30秒）

	// 🆕 重发现配置
	RediscoveryInterval    time.Duration // 重发现扫描间隔（默认30s）
	RediscoveryMaxRetries  int           // 最大重试次数（默认10）
	RediscoveryBackoffBase time.Duration // 退避基础时间（默认1m）

	// === peer_addrs 持久化后端配置 ===
	// PersistenceBackend: "badger" | "json"
	// - badger: 专用 BadgerDB（推荐，支持 all_discovered + prune）
	// - json:  文件存储（仅用于调试/迁移）
	PersistenceBackend string        // 默认 "badger"
	BadgerDir          string        // Badger数据目录模板（默认 "data/p2p/<hostID>/badger"）
	NamespacePrefix    string        // key前缀（默认 "peer_addrs/v1/"）
	PruneInterval      time.Duration // 清理周期（默认 1h）
	RecordTTL          time.Duration // 记录TTL（默认 7d）

	EnablePersistence bool   // 启用持久化存储（默认true）
	PersistenceFile   string // 持久化文件路径（相对于数据目录）
}

// NetworkTimeoutConfig 网络超时配置
// 🆕 HIGH-003 修复：提供更灵活的超时配置
type NetworkTimeoutConfig struct {
	// 连接超时配置
	DialTimeout        time.Duration // 拨号超时（默认15s）
	StreamOpenTimeout  time.Duration // 流打开超时（默认10s）
	StreamReadTimeout  time.Duration // 流读取超时（默认30s）
	StreamWriteTimeout time.Duration // 流写入超时（默认30s）

	// 动态超时配置
	EnableDynamicTimeout  bool          // 启用动态超时调整（默认true）
	MinTimeout            time.Duration // 最小超时时间（默认5s）
	MaxTimeout            time.Duration // 最大超时时间（默认60s）
	TimeoutIncreaseFactor float64       // 超时增长因子（默认1.5）
	TimeoutDecreaseFactor float64       // 超时减少因子（默认0.9）

	// 重试配置
	MaxRetries         int           // 最大重试次数（默认3）
	RetryBackoffBase   time.Duration // 重试退避基础时间（默认1s）
	RetryBackoffMax    time.Duration // 重试退避最大时间（默认30s）
	RetryBackoffFactor float64       // 重试退避增长因子（默认2.0）
}

// NetworkHealthConfig 网络健康检查配置
// 🆕 HIGH-003 修复：主动监控网络健康状态
type NetworkHealthConfig struct {
	Enabled               bool          // 启用网络健康检查（默认true）
	CheckInterval         time.Duration // 检查间隔（默认30s）
	UnhealthyThreshold    int           // 不健康阈值（连续失败次数，默认3）
	HealthyThreshold      int           // 健康阈值（连续成功次数，默认2）
	TimeoutRatioThreshold float64       // 超时比例阈值（默认0.3，即30%超时触发告警）

	// 自愈配置
	EnableAutoHealing  bool          // 启用自动修复（默认true）
	HealingCooldown    time.Duration // 修复冷却时间（默认1m）
	MaxHealingAttempts int           // 最大修复尝试次数（默认5）

	// 连接池健康检查
	ConnectionCheckEnabled bool          // 启用连接健康检查（默认true）
	ConnectionCheckTimeout time.Duration // 连接检查超时（默认5s）
	MaxIdleConnections     int           // 最大空闲连接数（默认50）
	IdleConnectionTimeout  time.Duration // 空闲连接超时（默认5m）
}

// GetBootstrapPeers 获取 BootstrapPeers 配置
func (o *Options) GetBootstrapPeers() []string {
	if o == nil {
		return nil
	}
	return o.BootstrapPeers
}

// GetAnnounce 获取 Announce 配置
func (o *Options) GetAnnounce() []string {
	if o == nil {
		return nil
	}
	return o.Announce
}

// GetAppendAnnounce 获取 AppendAnnounce 配置
func (o *Options) GetAppendAnnounce() []string {
	if o == nil {
		return nil
	}
	return o.AppendAnnounce
}

// GetNoAnnounce 获取 NoAnnounce 配置
func (o *Options) GetNoAnnounce() []string {
	if o == nil {
		return nil
	}
	return o.NoAnnounce
}

// GetGaterAllowedPrefixes 获取 GaterAllowedPrefixes 配置
func (o *Options) GetGaterAllowedPrefixes() []string {
	if o == nil {
		return nil
	}
	return o.GaterAllowedPrefixes
}

// GetGaterBlockedPrefixes 获取 GaterBlockedPrefixes 配置
func (o *Options) GetGaterBlockedPrefixes() []string {
	if o == nil {
		return nil
	}
	return o.GaterBlockedPrefixes
}

// NewFromChainConfig 从链配置生成 P2P 配置
//
// 根据链类型（public/consortium/private）和用户配置生成 P2P 运行时配置
func NewFromChainConfig(provider config.Provider) (*Options, error) {
	if provider == nil {
		return nil, fmt.Errorf("config provider is required")
	}

	// 获取链模式
	chainMode := provider.GetChainMode()
	if chainMode == "" {
		return nil, fmt.Errorf("chain mode is required")
	}

	// 获取节点配置（包含 P2P 相关配置）
	nodeCfg := provider.GetNode()
	if nodeCfg == nil {
		return nil, fmt.Errorf("node config is required")
	}

	// 获取网络配置（用于获取网络命名空间等信息）
	networkCfg := provider.GetNetwork()
	if networkCfg == nil {
		return nil, fmt.Errorf("network config is required")
	}

	// 获取网络命名空间（用于 Rendezvous 命名规则等）
	networkNamespace := provider.GetNetworkNamespace()

	// 从节点配置中提取 P2P 相关配置
	opts := &Options{
		ListenAddrs:               nodeCfg.Host.ListenAddresses,
		BootstrapPeers:            nodeCfg.Discovery.BootstrapPeers,
		EnableDHT:                 nodeCfg.Discovery.DHT.Enabled,
		DHTMode:                   nodeCfg.Discovery.DHT.Mode,
		EnableMDNS:                nodeCfg.Discovery.MDNS.Enabled,
		MDNSServiceName:           nodeCfg.Discovery.MDNS.ServiceName,
		DiscoveryNamespace:        nodeCfg.Discovery.RendezvousNamespace,
		DiscoveryInterval:         nodeCfg.Discovery.DiscoveryInterval,
		AdvertiseInterval:         nodeCfg.Discovery.AdvertiseInterval,
		MaxDiscoveryFailures:      5, // 默认值
		DiscoveryExpectedMinPeers: nodeCfg.Discovery.ExpectedMinPeers,
		DiscoverySingleNodeMode:   nodeCfg.Discovery.SingleNodeMode,
		EnableRelay:               nodeCfg.Connectivity.EnableRelayTransport,
		EnableRelayService:        nodeCfg.Connectivity.EnableRelayService,
		EnableDCUTR:               nodeCfg.Connectivity.EnableDCUtR,

		// Relay Service 资源配置（暂时使用默认值，后续可从配置扩展）
		RelayMaxReservations: 128,  // 默认值
		RelayMaxCircuits:     16,   // 默认值
		RelayBufferSize:      2048, // 默认值

		// AutoRelay
		EnableAutoRelay:            nodeCfg.Connectivity.EnableAutoRelay,
		StaticRelayPeers:           nodeCfg.Discovery.StaticRelayPeers,
		AutoRelayDynamicCandidates: nodeCfg.Connectivity.AutoRelayDynamicCandidates,
		MinPeers:                   nodeCfg.Connectivity.MinPeers,
		MaxPeers:                   nodeCfg.Connectivity.MaxPeers,
		LowWater:                   nodeCfg.Connectivity.LowWater,
		HighWater:                  nodeCfg.Connectivity.HighWater,
		GracePeriod:                nodeCfg.Connectivity.GracePeriod,
		EnableTCP:                  nodeCfg.Host.Transport.EnableTCP,
		EnableQUIC:                 nodeCfg.Host.Transport.EnableQUIC,
		EnableWebSocket:            nodeCfg.Host.Transport.EnableWebSocket,
		EnableTLS:                  nodeCfg.Host.Security.EnableTLS,
		EnableNoise:                nodeCfg.Host.Security.EnableNoise,
		EnableYamux:                nodeCfg.Host.Muxer.EnableYamux,
		YamuxWindowSize:            nodeCfg.Host.Muxer.YamuxWindowSize,
		YamuxMaxStreams:            nodeCfg.Host.Muxer.YamuxMaxStreams,
		YamuxConnectionTimeout:     nodeCfg.Host.Muxer.YamuxConnectionTimeout,
		AdvertisePrivateAddrs:      nodeCfg.Host.AdvertisePrivateAddrs,
		Announce:                   nodeCfg.Host.Announce,
		AppendAnnounce:             nodeCfg.Host.AppendAnnounce,
		NoAnnounce:                 nodeCfg.Host.NoAnnounce,
		GaterAllowedPrefixes:       nodeCfg.Host.Gater.AllowedPrefixes,
		GaterBlockedPrefixes:       nodeCfg.Host.Gater.BlockedPrefixes,
		MemoryLimitMB:              nodeCfg.Connectivity.Resources.MemoryLimitMB,
		MaxFileDescriptors:         nodeCfg.Connectivity.Resources.MaxFileDescriptors,
		EnableAutoNATService:       nodeCfg.Connectivity.EnableAutoNATService,

		// NAT / Reachability / AutoNAT
		EnableNATPortMap:    nodeCfg.Connectivity.EnableNATPort,
		ForceReachability:   nodeCfg.Connectivity.ForceReachability,
		EnableAutoNATClient: nodeCfg.Connectivity.EnableAutoNATClient,

		DiagnosticsEnabled: nodeCfg.Host.DiagnosticsEnabled,
		DiagnosticsAddr:    fmt.Sprintf("127.0.0.1:%d", nodeCfg.Host.DiagnosticsPort),

		// 身份配置（用于固定 PeerID）
		// 注意：KeyFile 在 GetNode() 中已经解析为绝对路径（相对于实例数据目录）
		IdentityKeyFile:    nodeCfg.Host.Identity.KeyFile,
		IdentityPrivateKey: nodeCfg.Host.Identity.PrivateKey,

		// Phase 5: forceConnect（GossipSub 拉活）- 从 node.discovery 映射
		BusinessCriticalPeerIDs: append([]string{}, nodeCfg.Discovery.BusinessCriticalPeerIDs...),
		ForceConnectEnabled: func() bool {
			// nil=默认启用；false=显式关闭
			if nodeCfg.Discovery.ForceConnect.Enabled == nil {
				return true
			}
			return *nodeCfg.Discovery.ForceConnect.Enabled
		}(),
		ForceConnectCooldown:          nodeCfg.Discovery.ForceConnect.Cooldown,
		ForceConnectConcurrency:       nodeCfg.Discovery.ForceConnect.Concurrency,
		ForceConnectBudgetPerRound:    nodeCfg.Discovery.ForceConnect.BudgetPerRound,
		ForceConnectTier2SampleBudget: nodeCfg.Discovery.ForceConnect.Tier2SampleBudget,
		ForceConnectTimeout:           nodeCfg.Discovery.ForceConnect.Timeout,
	}

	// === DiscoveryNamespace 命名规则（强约束 + 链身份绑定）===
	//
	// 规则：
	// - 若用户在 NodeOptions 中显式配置了 RendezvousNamespace（非空且非 "weisyn"），则直接复用；
	// - 否则，统一使用 "weisyn-<env>-<chainMode>-<networkNamespace>-<chainID>-<genesisHash8>" 作为默认命名空间。
	//   这样不同链的节点天然不会在同一个 rendezvous namespace 下相遇。
	if opts.DiscoveryNamespace == "" || opts.DiscoveryNamespace == "weisyn" {
		// 获取环境（dev/test/prod）
		env := "dev"
		appCfg := provider.GetAppConfig()
		if appCfg != nil {
			env = string(appCfg.GetEnvironment())
		}

		// 获取 chain_id
		chainID := ""
		if appCfg != nil && appCfg.Network != nil && appCfg.Network.ChainID != nil {
			chainID = fmt.Sprintf("%d", *appCfg.Network.ChainID)
		}

		// 获取 genesis hash（前8位）
		genesisHash8 := ""
		unifiedGenesis := provider.GetUnifiedGenesisConfig()
		if unifiedGenesis != nil {
			// 导入 node 包来计算 genesis hash
			hash, err := node.CalculateGenesisHash(unifiedGenesis)
			if err == nil && len(hash) >= 8 {
				genesisHash8 = hash[:8]
			}
		}

		// 构建包含链身份的 namespace
		if genesisHash8 != "" && chainID != "" {
			opts.DiscoveryNamespace = fmt.Sprintf("weisyn-%s-%s-%s-%s-%s",
				env, chainMode, networkNamespace, chainID, genesisHash8)
		} else {
			// 降级：如果无法获取 genesis hash，使用简化版本
			opts.DiscoveryNamespace = fmt.Sprintf("weisyn-%s-%s-%s-%s",
				env, chainMode, networkNamespace, chainID)
		}
	}

	// 根据链模式设置默认 Profile 和私有网络配置
	switch chainMode {
	case "public":
		// 公有链：默认 server profile，不启用私有网络
		if opts.Profile == "" {
			opts.Profile = ProfileServer
		}
		opts.PrivateNetwork = false

		// 公有链 DHT 模式配置
		//
		// 🆕 libp2p 资源控制说明：
		// - server 模式（默认）：响应其他节点的 DHT 请求，有助于网络健康，但会产生更多入站连接和 Goroutine
		// - client 模式：只主动查询，不响应他人 DHT 请求，减少资源消耗（推荐内存受限环境）
		//
		// 配置方式：设置 node.discovery.dht.mode: "client" 可强制使用 client 模式
		// 参考：LIBP2P_GOROUTINE_ANALYSIS.md
		if opts.EnableDHT {
			if opts.DHTMode == "" || opts.DHTMode == "auto" {
				opts.DHTMode = "server" // 默认 server，可通过配置切换为 client
			}
			// 用户显式配置的 "client" 模式将被保留，不会被覆盖
		}

	case "consortium":
		// 联盟链：默认 server profile，启用私有网络（需要 mTLS，不使用 PSK）
		if opts.Profile == "" {
			opts.Profile = ProfileServer
		}
		// 联盟链使用 mTLS 而不是 PSK，但 PrivateNetwork 标志用于启用证书验证
		opts.PrivateNetwork = true

		// 联盟链：默认使用 client/auto DHT，由运维按需调整
		if opts.EnableDHT && opts.DHTMode == "" {
			opts.DHTMode = "client"
		}
		// 证书管理配置从 security.certificate_management 读取
		certMgmt := provider.GetCertificateManagement()
		if certMgmt != nil && certMgmt.CABundlePath != nil {
			opts.CertificateManagementCABundlePath = *certMgmt.CABundlePath
		}

	case "private":
		// 私有链：默认 lan profile，启用私有网络（使用 PSK）
		if opts.Profile == "" {
			opts.Profile = ProfileLAN
		}
		opts.PrivateNetwork = true

		// 私有链：优先使用 LAN DHT 模式
		if opts.EnableDHT {
			if opts.DHTMode == "" || opts.DHTMode == "auto" {
				opts.DHTMode = "lan"
			}
		}
		// PSK 路径从 security.psk.file 读取
		pskConfig := provider.GetPSK()
		if pskConfig != nil && pskConfig.File != nil && *pskConfig.File != "" {
			opts.PSKPath = *pskConfig.File
		}

	default:
		// 未知链模式，使用默认值
		if opts.Profile == "" {
			opts.Profile = ProfileServer
		}
	}

	// 应用默认值（如果某些字段未设置）
	applyDefaults(opts)

	// 构建 UserAgent（包含链身份信息）
	userAgent := buildUserAgent(provider)
	opts.UserAgent = userAgent

	// === 生产级互联强校验（fail-fast）===
	//
	// 目标：
	// - 避免 test/prod 环境因为“占位符/无效 bootstrap peers / rendezvous 配置缺失”而悄悄进入孤岛；
	// - 明确区分 dev（允许单机/局域网快速启动）与 test/prod（必须可互联）。
	if err := validateConnectivityReadiness(provider, chainMode, opts); err != nil {
		return nil, err
	}

	return opts, nil
}

func validateConnectivityReadiness(provider config.Provider, chainMode string, opts *Options) error {
	if provider == nil || opts == nil {
		return nil
	}

	// 获取环境（dev/test/prod）
	env := "dev"
	appCfg := provider.GetAppConfig()
	if appCfg != nil {
		env = normalizeEnv(string(appCfg.GetEnvironment()))
	}

	// dev 环境允许“单节点/孤岛”启动（用于开发调试）
	if env == "dev" {
		return nil
	}

	// 1) bootstrap peers 必须全部有效且非占位符（test/prod 的强约束）
	valid, invalid, placeholders := validateBootstrapPeers(opts.BootstrapPeers)
	if len(placeholders) > 0 {
		return fmt.Errorf(
			"p2p bootstrap_peers contains placeholder entries (example=%s). "+
				"for %s environment you must configure real peers: /ip4/<ip>/tcp/28683/p2p/<peerId>",
			placeholders[0], env,
		)
	}
	if len(invalid) > 0 {
		return fmt.Errorf(
			"p2p bootstrap_peers contains invalid multiaddr entries (example=%s). "+
				"for %s environment all entries must be valid: /ip4/<ip>/tcp/28683/p2p/<peerId>",
			invalid[0], env,
		)
	}

	// 2) 在非 dev 环境，如果 mDNS 关闭，则必须至少有 1 个有效 bootstrap peer
	//    （否则 DHT/Sync/Consensus 的网络互联不具备任何入口）
	if !opts.EnableMDNS && len(valid) == 0 {
		return fmt.Errorf(
			"p2p connectivity not ready for %s environment: enable_mdns=false and bootstrap_peers is empty. "+
				"please configure at least one bootstrap peer or enable mDNS for LAN deployments",
			env,
		)
	}

	// 3) DHT rendezvous 关键配置检查：启用 DHT 且不处于单节点模式时，必须有 discovery namespace，
	//    且 expected_min_peers 不能为 0（否则 discovery 会显式跳过 rendezvous 循环）。
	if opts.EnableDHT && !opts.DiscoverySingleNodeMode {
		if strings.TrimSpace(opts.DiscoveryNamespace) == "" {
			return fmt.Errorf(
				"p2p discovery not ready for %s environment: enable_dht=true but rendezvous namespace is empty. "+
					"please configure node.discovery.rendezvous_namespace (or ensure it is auto-derived)",
				env,
			)
		}
		// 公有链/联盟链在 test/prod 默认要求 DHT 发现能工作，expected_min_peers=0 会导致跳过 rendezvous。
		if (chainMode == "public" || chainMode == "consortium") && opts.DiscoveryExpectedMinPeers == 0 {
			return fmt.Errorf(
				"p2p discovery not ready for %s environment: expected_min_peers=0 will disable DHT rendezvous loop. "+
					"for %s chain please set node.discovery.expected_min_peers >= 1 (recommended 3)",
				env, chainMode,
			)
		}
	}

	// 4) 公有链在 test/prod 环境必须启用基础连通性增强能力（生产基线）
	// - AutoNATClient：用于自测可达性，决定是否需要 relay/打洞策略
	// - AutoRelay + DCUtR：用于 NAT 环境提升互联成功率
	// - NATPortMap：用于 UPnP/NAT-PMP 端口映射（云/家宽场景常见）
	if chainMode == "public" {
		if !opts.EnableAutoNATClient {
			return fmt.Errorf(
				"p2p connectivity not ready for %s public chain: enable_autonat_client=false. "+
					"for production-grade public internet connectivity please set node.connectivity.enable_autonat_client=true",
				env,
			)
		}
		if !opts.EnableAutoRelay {
			return fmt.Errorf(
				"p2p connectivity not ready for %s public chain: enable_auto_relay=false. "+
					"for production-grade public internet connectivity please set node.connectivity.enable_auto_relay=true",
				env,
			)
		}
		if !opts.EnableDCUTR {
			return fmt.Errorf(
				"p2p connectivity not ready for %s public chain: enable_dcutr=false. "+
					"for production-grade public internet connectivity please set node.connectivity.enable_dcutr=true",
				env,
			)
		}
		if !opts.EnableNATPortMap {
			return fmt.Errorf(
				"p2p connectivity not ready for %s public chain: enable_nat_port=false. "+
					"for production-grade public internet connectivity please set node.connectivity.enable_nat_port=true",
				env,
			)
		}
	}

	return nil
}

func normalizeEnv(env string) string {
	env = strings.TrimSpace(strings.ToLower(env))
	switch env {
	case "", "dev", "development", "local":
		return "dev"
	case "test", "testing", "staging":
		return "test"
	case "prod", "production":
		return "prod"
	default:
		// 未知环境值：保守处理为非 dev（将触发更严格校验），但保留原值以便报错定位
		return env
	}
}

func validateBootstrapPeers(peers []string) (valid []string, invalid []string, placeholders []string) {
	if len(peers) == 0 {
		return nil, nil, nil
	}
	for _, p := range peers {
		if strings.Contains(p, "ExampleBootstrapPeerReplaceMe") {
			placeholders = append(placeholders, p)
			continue
		}
		m, err := ma.NewMultiaddr(p)
		if err != nil {
			invalid = append(invalid, p)
			continue
		}
		if _, err := libpeer.AddrInfoFromP2pAddr(m); err != nil {
			invalid = append(invalid, p)
			continue
		}
		valid = append(valid, p)
	}
	return valid, invalid, placeholders
}

// buildUserAgent 构建包含链身份信息的 UserAgent 字符串
func buildUserAgent(provider config.Provider) string {
	version := "weisyn-node/1.0.0"
	if provider == nil {
		return version
	}

	// 获取链身份
	appCfg := provider.GetAppConfig()
	if appCfg == nil {
		return version
	}

	unifiedGenesis := provider.GetUnifiedGenesisConfig()
	if unifiedGenesis == nil {
		return version
	}

	genesisHash, err := node.CalculateGenesisHash(unifiedGenesis)
	if err != nil {
		return version
	}

	localIdentity := node.BuildLocalChainIdentity(appCfg, genesisHash)
	identityStr := localIdentity.String() // ns/mode/chain@hash8

	return fmt.Sprintf("%s/%s", version, identityStr)
}

// applyDefaults 应用默认值到配置选项
func applyDefaults(opts *Options) {
	// 如果某些关键字段未设置，使用默认值
	if len(opts.ListenAddrs) == 0 {
		opts.ListenAddrs = []string{"/ip4/0.0.0.0/tcp/28683", "/ip4/0.0.0.0/udp/28683/quic-v1"}
	}

	// 🆕 libp2p 资源控制：进一步降低连接水位
	// 背景：阿里云节点 Goroutine 峰值 34,832（19x 本地节点）
	// 目标：降低最大连接数，减少非 WES 节点占用的资源
	// 参考：LIBP2P_GOROUTINE_ANALYSIS.md
	if opts.MinPeers == 0 {
		opts.MinPeers = 8
	}
	if opts.MaxPeers == 0 {
		opts.MaxPeers = 30 // 🆕 40 → 30，进一步减少最大连接数
	}

	if opts.DiagnosticsAddr == "" && opts.DiagnosticsEnabled {
		opts.DiagnosticsAddr = "127.0.0.1:28686"
	}

	// Discovery 调度默认值
	if opts.DiscoveryInterval == 0 {
		opts.DiscoveryInterval = 20 * time.Second
	}
	if opts.AdvertiseInterval == 0 {
		opts.AdvertiseInterval = 300 * time.Second // 5分钟
	}
	if opts.MaxDiscoveryFailures == 0 {
		opts.MaxDiscoveryFailures = 5
	}
	// DiscoveryNamespace 默认值在 NewFromChainConfig 中基于 networkNamespace 统一设置，
	// 这里不再兜底，避免与链模式/网络命名规则冲突。

	// 传输层默认值
	if !opts.EnableTCP && !opts.EnableQUIC && !opts.EnableWebSocket {
		// 如果全部关闭，默认启用 TCP 和 QUIC
		opts.EnableTCP = true
		opts.EnableQUIC = true
	}

	// 安全层默认值
	if !opts.EnableTLS && !opts.EnableNoise {
		// 如果全部关闭，默认启用 Noise
		opts.EnableNoise = true
	}

	// 🆕 libp2p 资源控制：进一步降低连接管理水位
	//
	// 问题：阿里云节点 Goroutine 峰值 34,832 个，与大量非 WES 节点连接有关
	// 解决：HighWater 80 → 50，更激进地淘汰非业务连接
	// 目标：Goroutine 峰值从 34,832 降到 < 15,000
	if opts.LowWater == 0 {
		opts.LowWater = 15
	}
	if opts.HighWater == 0 {
		// 🆕 80 → 50，更激进地控制连接数
		// 配合 WES-aware ConnManager，优先淘汰非 WES 节点
		opts.HighWater = 50
	}
	if opts.GracePeriod == 0 {
		opts.GracePeriod = 20 * time.Second
	}

	// Muxer 默认值
	if opts.YamuxWindowSize == 0 {
		opts.YamuxWindowSize = 1024 // 1MB in KB
	}
	if opts.YamuxMaxStreams == 0 {
		opts.YamuxMaxStreams = 256
	}
	if opts.YamuxConnectionTimeout == 0 {
		opts.YamuxConnectionTimeout = 30 * time.Second
	}

	// NAT / Reachability / AutoNAT 默认值
	// 注意：这些字段在 NewFromChainConfig 中已从 NodeConfig 映射，这里仅作为兜底
	// EnableNATPortMap: 默认 true（连接优先策略，与旧实现一致）
	// ForceReachability: 默认 ""（自动检测）
	// EnableAutoNATClient: 默认 false（需要显式启用）
	// EnableAutoNATService: 默认 false（需要显式启用）

	// AutoRelay 默认值
	if opts.AutoRelayDynamicCandidates == 0 {
		opts.AutoRelayDynamicCandidates = 16 // 与旧实现一致
	}

	// Phase 5: forceConnect（GossipSub 拉活）默认值
	// 默认启用，但通过 cooldown/budget/concurrency 做强约束，避免 goroutine 风暴。
	if opts.ForceConnectCooldown == 0 {
		opts.ForceConnectCooldown = 2 * time.Minute
	}
	if opts.ForceConnectConcurrency == 0 {
		opts.ForceConnectConcurrency = 15
	}
	if opts.ForceConnectBudgetPerRound == 0 {
		opts.ForceConnectBudgetPerRound = 50
	}
	if opts.ForceConnectTier2SampleBudget == 0 {
		opts.ForceConnectTier2SampleBudget = 20
	}
	if opts.ForceConnectTimeout == 0 {
		opts.ForceConnectTimeout = 10 * time.Second
	}

	// Relay Service 资源配置默认值
	if opts.RelayMaxReservations == 0 {
		opts.RelayMaxReservations = 128 // 与 relayv2.DefaultResources() 一致
	}
	if opts.RelayMaxCircuits == 0 {
		opts.RelayMaxCircuits = 16 // 与 relayv2.DefaultResources() 一致
	}
	if opts.RelayBufferSize == 0 {
		opts.RelayBufferSize = 2048 // 与 relayv2.DefaultResources() 一致
	}

	// 资源管理默认值（带宽/FD 限制）
	if opts.MemoryLimitMB == 0 {
		opts.MemoryLimitMB = 512
	}
	if opts.MaxFileDescriptors == 0 {
		opts.MaxFileDescriptors = 4096
	}

	// 地址管理器默认值
	// ⚠️ 关键修复：Enabled字段必须显式设置，否则零值为false导致AddrManager完全失效
	// 默认启用AddrManager（生产级地址生命周期管理）
	if !opts.AddrManager.Enabled {
		opts.AddrManager.Enabled = true
	}
	if opts.AddrManager.DHTAddrTTL == 0 {
		opts.AddrManager.DHTAddrTTL = 30 * time.Minute
	}
	if opts.AddrManager.ConnectedAddrTTL == 0 {
		opts.AddrManager.ConnectedAddrTTL = 24 * time.Hour
	}
	if opts.AddrManager.FailedAddrTTL == 0 {
		opts.AddrManager.FailedAddrTTL = 5 * time.Minute
	}
	if opts.AddrManager.RefreshInterval == 0 {
		opts.AddrManager.RefreshInterval = 10 * time.Minute
	}
	if opts.AddrManager.RefreshThreshold == 0 {
		opts.AddrManager.RefreshThreshold = 5 * time.Minute
	}
	// 🆕 P2 修复：限制最大并发查询数，避免 DHT 风暴
	if opts.AddrManager.MaxConcurrentLookups == 0 {
		opts.AddrManager.MaxConcurrentLookups = 5 // 原 10 → 5
	}
	// 🆕 P2 修复：缩短查询超时，避免网络不稳定时的 Goroutine 堆积
	if opts.AddrManager.LookupTimeout == 0 {
		opts.AddrManager.LookupTimeout = 15 * time.Second // 原 30s → 15s
	}
	if strings.TrimSpace(opts.AddrManager.PersistenceBackend) == "" {
		opts.AddrManager.PersistenceBackend = "badger"
	}
	if strings.TrimSpace(opts.AddrManager.BadgerDir) == "" {
		opts.AddrManager.BadgerDir = "data/p2p/<hostID>/badger"
	}
	if strings.TrimSpace(opts.AddrManager.NamespacePrefix) == "" {
		opts.AddrManager.NamespacePrefix = "peer_addrs/v1/"
	}
	if opts.AddrManager.PruneInterval == 0 {
		opts.AddrManager.PruneInterval = 1 * time.Hour
	}
	if opts.AddrManager.RecordTTL == 0 {
		opts.AddrManager.RecordTTL = 7 * 24 * time.Hour
	}
	if opts.AddrManager.PersistenceFile == "" {
		opts.AddrManager.PersistenceFile = "peer_addrs.json"
	}
	// 🆕 重发现配置默认值（P2 修复：优化重试策略，避免 Goroutine 堆积）
	if opts.AddrManager.RediscoveryInterval == 0 {
		opts.AddrManager.RediscoveryInterval = 30 * time.Second
	}
	if opts.AddrManager.RediscoveryMaxRetries == 0 {
		opts.AddrManager.RediscoveryMaxRetries = 3 // 原 10 → 3，减少重试次数
	}
	if opts.AddrManager.RediscoveryBackoffBase == 0 {
		opts.AddrManager.RediscoveryBackoffBase = 30 * time.Second // 原 1m → 30s
	}
	// EnablePersistence默认启用
	if !opts.AddrManager.EnablePersistence {
		opts.AddrManager.EnablePersistence = true
	}
}

// NewFromAppConfig 从 AppConfig 直接生成 P2P 配置（备用方法）
//
// 当 Provider 接口不完整时，可以直接从 AppConfig 解析
func NewFromAppConfig(appConfig *types.AppConfig) (*Options, error) {
	if appConfig == nil {
		return nil, fmt.Errorf("app config is required")
	}

	opts := &Options{
		Profile: ProfileServer, // 默认值
	}

	// 从 AppConfig 中提取配置
	if appConfig.Network != nil {
		chainMode := ""
		if appConfig.Network.ChainMode != nil {
			chainMode = *appConfig.Network.ChainMode
		}

		switch chainMode {
		case "public":
			opts.Profile = ProfileServer
			opts.PrivateNetwork = false
		case "consortium":
			opts.Profile = ProfileServer
			opts.PrivateNetwork = true
		case "private":
			opts.Profile = ProfileLAN
			opts.PrivateNetwork = true
		}
	}

	if appConfig.Node != nil {
		if appConfig.Node.ListenAddresses != nil {
			opts.ListenAddrs = appConfig.Node.ListenAddresses
		}
		if appConfig.Node.BootstrapPeers != nil {
			opts.BootstrapPeers = appConfig.Node.BootstrapPeers
		}
		if appConfig.Node.EnableMDNS != nil {
			opts.EnableMDNS = *appConfig.Node.EnableMDNS
		}
		if appConfig.Node.EnableDHT != nil {
			opts.EnableDHT = *appConfig.Node.EnableDHT
		}
		if appConfig.Node.EnableDCUtR != nil {
			opts.EnableDCUTR = *appConfig.Node.EnableDCUtR
		}
	}

	return opts, nil
}
