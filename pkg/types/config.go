// Package types provides configuration type definitions.
package types

// AppConfig 应用程序根配置
// 只包含JSON配置文件解析所需的结构，不包含任何内部字段
// 默认值和完整配置结构在 internal/config/*/defaults.go 和 internal/config/*/config.go 中定义
type AppConfig struct {
	// 应用程序基本信息
	AppName *string `json:"app_name,omitempty"` // 应用名称
	DataDir *string `json:"data_dir,omitempty"` // 数据目录路径
	Version *string `json:"version,omitempty"`  // 应用版本

	// === 运行环境与网络模式配置 ===
	// Environment 运行环境：dev | test | prod
	// 描述部署的生命周期阶段，只影响日志级别、指标上报、默认端口等运维属性
	Environment *string `json:"environment,omitempty"` // 运行环境：dev | test | prod

	// NetworkProfile 网络配置档案名称（可选）
	// 用于标识特定的 (Environment, ChainMode, NetworkID) 组合，如 "prod-public-mainnet"
	NetworkProfile *string `json:"network_profile,omitempty"` // 网络配置档案名称

	// NodeRole 节点角色（可选，v1 预设模板）
	// 用于区分节点在网络中的职责：
	// - miner:     出块节点，通常需要 from_genesis 或受信任快照 + 完整同步
	// - validator: 共识验证节点，参与投票/验证但不直接挖矿
	// - full:      普通全节点，仅同步与转发，不参与出块/投票
	// - light:     轻节点，仅维护头部与部分状态
	// 默认留空时，由各模块按 Environment/ChainMode 推导合适的行为。
	NodeRole *string `json:"node_role,omitempty"`

	// === 新统一配置结构 ===
	// 网络身份配置 - 对应配置文件中的 network 字段
	Network *UserNetworkConfig `json:"network,omitempty"`

	// 创世配置 - 对应配置文件中的 genesis 字段
	Genesis *UserGenesisConfig `json:"genesis,omitempty"`

	// 挖矿配置 - 对应配置文件中的 mining 字段
	Mining *UserMiningConfig `json:"mining,omitempty"`

	// 节点网络配置
	Node *UserNodeConfig `json:"node,omitempty"`

	// API服务配置
	API *UserAPIConfig `json:"api,omitempty"`

	// 安全配置 - 对应配置文件中的 security 字段
	Security *UserSecurityConfig `json:"security,omitempty"`

	// === 保持向后兼容的字段 ===
	// 区块链配置
	Blockchain interface{} `json:"blockchain,omitempty"`

	// 共识配置
	Consensus interface{} `json:"consensus,omitempty"`

	// 存储配置
	Storage *UserStorageConfig `json:"storage,omitempty"`

	// 日志配置
	Log *UserLogConfig `json:"log,omitempty"`

	// 同步配置 - 对应配置文件中的 sync 字段
	// 用于控制节点启动时的同步策略等高级行为
	Sync *UserSyncConfig `json:"sync,omitempty"`

	// 内存监控配置
	MemoryMonitoring *UserMemoryMonitoringConfig `json:"memory_monitoring,omitempty"`

	// 签名器配置（内部配置，不暴露给用户）
	Signer *UserSignerConfig `json:"signer,omitempty"`

	// EUTXO配置 - 对应配置文件中的 eutxo 字段
	EUTXO *UserEUTXOConfig `json:"eutxo,omitempty"`
}

// UserNetworkConfig 用户网络身份配置
// 对应配置文件中的 network 字段
type UserNetworkConfig struct {
	ChainID          *uint64 `json:"chain_id,omitempty"`          // 链ID
	NetworkName      *string `json:"network_name,omitempty"`      // 网络名称
	NetworkID        *string `json:"network_id,omitempty"`        // 网络标识符（如"WES_mainnet_2025"）
	NetworkNamespace *string `json:"network_namespace,omitempty"` // 网络命名空间（如"mainnet-public", "test-consortium", "dev-private"）
	ChainMode        *string `json:"chain_mode,omitempty"`        // 链模式：public | consortium | private
}

// UserSyncConfig 用户同步配置
// 对应配置文件中的 sync 字段
type UserSyncConfig struct {
	// StartupMode 启动同步模式：from_genesis | from_network | snapshot
	// - from_genesis: 节点可以从本地创世高度开始（典型 dev/单节点挖矿场景）
	// - from_network: 节点应从网络获取已有区块高度再参与出块/业务（典型 test/prod follower）
	// - snapshot:     节点从快照导入后再追同步（预留，当前实现视为 from_network 的变体）
	StartupMode *string `json:"startup_mode,omitempty"`

	// RequireTrustedCheckpoint 是否强制要求配置受信任检查点：
	// - true  且 startup_mode=from_network 时，如果未配置 trusted_checkpoint 或配置不完整，将视为配置错误/拒绝同步；
	// - false 或未设置时，不强制要求检查点（默认行为，便于现网平滑过渡）。
	//
	// 典型用法：
	// - prod/test 共识/全节点：建议显式设置 require_trusted_checkpoint=true，并提供受信任高度+区块哈希；
	// - dev 本地单节点：保持默认（false），不要求检查点。
	RequireTrustedCheckpoint *bool `json:"require_trusted_checkpoint,omitempty"`

	// TrustedCheckpoint 受信任检查点配置：
	// - height: 受信任区块高度（>=0），通常为较新的已充分确认高度；
	// - block_hash: 对应高度区块的哈希，使用 0x 前缀的十六进制字符串或纯 hex 字符串。
	//
	// 后续同步策略可以基于该检查点：
	// - from_network 模式下，仅接受与该检查点一致的远端历史；
	// - 在快照/归档恢复场景中，用于锚定状态正确性。
	TrustedCheckpoint *UserTrustedCheckpointConfig `json:"trusted_checkpoint,omitempty"`
}

// UserTrustedCheckpointConfig 受信任检查点配置
type UserTrustedCheckpointConfig struct {
	// Height 受信任区块高度：
	// - 对于从创世完整同步的节点，可为 0（表示不使用中间检查点）；
	// - 对于从网络/快照恢复的节点，通常配置为一个最近但已充分确认的高度。
	Height *uint64 `json:"height,omitempty"`

	// BlockHash 对应高度区块的哈希（十六进制字符串，大小写不敏感，可带 0x 前缀）。
	// 当 Height > 0 且 RequireTrustedCheckpoint=true 时，建议必须提供。
	BlockHash *string `json:"block_hash,omitempty"`
}

// UserGenesisConfig 用户创世配置
// 对应配置文件中的 genesis 字段
type UserGenesisConfig struct {
	Timestamp           int64                `json:"timestamp,omitempty"`             // 创世时间戳（固定值，确保所有节点一致）
	Accounts            []UserGenesisAccount `json:"accounts,omitempty"`              // 创世账户列表
	ExpectedGenesisHash *string              `json:"expected_genesis_hash,omitempty"` // 预期创世哈希（十六进制字符串），用于强制校验链身份
}

// UserGenesisAccount 用户创世账户配置
// 只包含JSON配置文件中实际出现的字段
type UserGenesisAccount struct {
	Name           string `json:"name,omitempty"`            // 账户名称
	PrivateKey     string `json:"private_key,omitempty"`     // 私钥（仅用于开发/测试环境）
	PublicKey      string `json:"public_key,omitempty"`      // 公钥（生产环境推荐只提供公钥）
	Address        string `json:"address,omitempty"`         // 地址
	InitialBalance string `json:"initial_balance,omitempty"` // 初始余额（字符串形式支持大数）
}

// UserMiningConfig 用户挖矿配置
// 对应配置文件中的 mining 字段
type UserMiningConfig struct {
	TargetBlockTime  *string `json:"target_block_time,omitempty"`  // 目标出块时间（如："5s", "10s"）
	EnableAggregator *bool   `json:"enable_aggregator,omitempty"`  // 是否启用聚合器
	MaxMiningThreads *int    `json:"max_mining_threads,omitempty"` // 最大挖矿线程数
	MiningTimeout    *string `json:"mining_timeout,omitempty"`     // 单轮挖矿超时（如："5m"）
	PoWSlice         *string `json:"pow_slice,omitempty"`          // 单次PoW尝试窗口（如："30s"；过小会导致频繁重建候选、有效算力损失）

	// ========== 挖矿稳定性门闸（V2） ==========
	// MinNetworkQuorumTotal 最小网络法定人数（含本机）。
	// - dev 默认 2（至少 2 个节点互相发现并完成握手）
	// - prod 默认 max(3, consensus.aggregator.min_peer_threshold)
	MinNetworkQuorumTotal *int `json:"min_network_quorum_total,omitempty"`

	// AllowSingleNodeMining 是否允许单节点挖矿（仅 dev 环境，且仅允许 from_genesis 启动模式）。
	AllowSingleNodeMining *bool `json:"allow_single_node_mining,omitempty"`

	// NetworkDiscoveryTimeoutSeconds 网络发现超时（秒）。
	NetworkDiscoveryTimeoutSeconds *int `json:"network_discovery_timeout_seconds,omitempty"`

	// QuorumRecoveryTimeoutSeconds 法定人数恢复超时（秒）。
	QuorumRecoveryTimeoutSeconds *int `json:"quorum_recovery_timeout_seconds,omitempty"`

	// MaxHeightSkew 最大高度偏差阈值（区块数）。
	// ⚠️ 彻底简化：不区分 initial/runtime，统一使用一个阈值。
	MaxHeightSkew *uint64 `json:"max_height_skew,omitempty"`

	// MaxTipStalenessSeconds 链尖时效性阈值（秒）。
	MaxTipStalenessSeconds *uint64 `json:"max_tip_staleness_seconds,omitempty"`

	// EnableTipFreshnessCheck 是否启用链尖新鲜度检查。
	EnableTipFreshnessCheck *bool `json:"enable_tip_freshness_check,omitempty"`

	// EnableNetworkAlignmentCheck 是否启用网络对齐检查（V2 挖矿门闸）。
	// 默认 true，允许关闭以在生产环境逐步启用。
	EnableNetworkAlignmentCheck *bool `json:"enable_network_alignment_check,omitempty"`
}

// UserNodeConfig 用户节点网络配置
// 只包含JSON配置文件中实际出现的字段
type UserNodeConfig struct {
	ListenAddresses []string `json:"listen_addresses,omitempty"` // P2P监听地址列表
	BootstrapPeers  []string `json:"bootstrap_peers,omitempty"`  // 引导节点列表

	EnableMDNS      *bool `json:"enable_mdns,omitempty"`           // 启用mDNS发现
	EnableDHT       *bool `json:"enable_dht,omitempty"`            // 启用DHT
	EnableNATPort   *bool `json:"enable_nat_port,omitempty"`       // 启用NAT端口映射
	EnableAutoRelay *bool `json:"enable_auto_relay,omitempty"`     // 启用自动中继
	EnableDCUtR     *bool `json:"enable_dcutr,omitempty"`          // 启用打洞
	EnableAutoNAT   *bool `json:"enable_autonat_client,omitempty"` // 启用 AutoNAT 客户端（自检测可达性）

	// DHT 发现高级配置
	// - expected_min_peers: 期望的最小 DHT peers 数量，用于 DHT 发现状态机从 Bootstrap 过渡到 Steady 的阈值；
	//   典型公网环境建议为 3；单节点/极小网络可设置为 0。
	// - single_node_mode: 单节点/孤立网络模式开关，为 true 时可以显式关闭 DHT rendezvous 循环。
	ExpectedMinPeers *int  `json:"expected_min_peers,omitempty"`
	SingleNodeMode   *bool `json:"single_node_mode,omitempty"`

	// P2P身份与地址公告配置
	Host *UserHostConfig `json:"host,omitempty"` // 主机配置
}

// UserHostConfig 用户主机配置
// 只包含JSON配置文件中实际出现的字段
type UserHostConfig struct {
	Identity              *UserIdentityConfig `json:"identity,omitempty"`                // 身份配置
	Gater                 *UserGaterConfig    `json:"gater,omitempty"`                   // 连接门禁配置
	AdvertisePrivateAddrs *bool               `json:"advertise_private_addrs,omitempty"` // 是否公告私网地址（影响地址过滤）

	// 诊断配置（可选）
	// - diagnostics_enabled: 是否启用诊断 HTTP 服务（pprof / P2P diagnostics 等）
	// - diagnostics_port: 诊断 HTTP 端口，默认 28686，对应 internal/config/node/defaults.go 中的 defaultDiagnosticsPort
	DiagnosticsEnabled *bool `json:"diagnostics_enabled,omitempty"`
	DiagnosticsPort    *int  `json:"diagnostics_port,omitempty"`
}

// UserIdentityConfig 用户身份配置
// 只包含JSON配置文件中实际出现的字段
type UserIdentityConfig struct {
	PrivateKey *string `json:"private_key,omitempty"` // base64编码的libp2p私钥
	KeyFile    *string `json:"key_file,omitempty"`    // 私钥文件路径
}

// UserGaterConfig 用户连接门禁配置
// 用于控制P2P节点的连接准入策略
type UserGaterConfig struct {
	Mode          *string  `json:"mode,omitempty"`           // 门禁模式：open | allowlist | denylist
	AllowCIDRs    []string `json:"allow_cidrs,omitempty"`    // 允许的CIDR网段列表（mode=allowlist时生效）
	AllowPrefixes []string `json:"allow_prefixes,omitempty"` // 允许的地址前缀列表（mode=allowlist时生效）
	DenyCIDRs     []string `json:"deny_cidrs,omitempty"`     // 拒绝的CIDR网段列表（mode=denylist时生效）
	DenyPrefixes  []string `json:"deny_prefixes,omitempty"`  // 拒绝的地址前缀列表（mode=denylist时生效）
}

// UserAPIConfig 用户API配置
// 只包含JSON配置文件中实际出现的字段
type UserAPIConfig struct {
	// HTTP 服务总开关（包含 REST/JSON-RPC/WebSocket）
	HTTPEnabled *bool `json:"http_enabled,omitempty"` // 是否启用HTTP服务（默认true）
	HTTPPort    *int  `json:"http_port,omitempty"`    // HTTP监听端口

	// HTTP 协议细粒度开关（v0.0.2+）
	HTTPEnableREST      *bool `json:"http_enable_rest,omitempty"`      // 是否启用REST端点（默认true）
	HTTPEnableJSONRPC   *bool `json:"http_enable_jsonrpc,omitempty"`   // 是否启用JSON-RPC（默认true，主协议）
	HTTPEnableWebSocket *bool `json:"http_enable_websocket,omitempty"` // 是否启用WebSocket（默认true）

	// HTTP CORS 配置
	HTTPCorsEnabled *bool    `json:"http_cors_enabled,omitempty"` // 是否启用CORS（默认true）
	HTTPCorsOrigins []string `json:"http_cors_origins,omitempty"` // 允许的CORS源（默认["*"]）

	// gRPC 配置
	GRPCEnabled *bool `json:"grpc_enabled,omitempty"` // 是否启用gRPC API（默认true）
	GRPCPort    *int  `json:"grpc_port,omitempty"`    // gRPC监听端口

	// 兼容性字段（已废弃，使用 http_enable_websocket）
	WebSocketEnabled *bool `json:"websocket_enabled,omitempty"` // [废弃] 使用 http_enable_websocket
	WebSocketPort    *int  `json:"websocket_port,omitempty"`    // [废弃] WebSocket 使用 HTTP 端口

	// 功能开关
	EnableMiningAPI *bool `json:"enable_mining_api,omitempty"` // 是否启用挖矿API（默认false）
}

// UserSecurityConfig 用户安全配置
// 对应配置文件中的 security 字段
// 定义链的安全模型和访问控制策略
type UserSecurityConfig struct {
	// 接入控制配置
	AccessControl *UserAccessControlConfig `json:"access_control,omitempty"`

	// 证书管理配置（仅联盟链）
	CertificateManagement *UserCertificateManagementConfig `json:"certificate_management,omitempty"`

	// PSK 配置（仅私有链）
	PSK *UserPSKConfig `json:"psk,omitempty"`

	// 权限模型：public | consortium | private
	PermissionModel *string `json:"permission_model,omitempty"`
}

// UserAccessControlConfig 用户接入控制配置
// 定义网络接入控制策略
type UserAccessControlConfig struct {
	// 接入控制模式：open | allowlist | psk
	// - public: "open" - 开放接入，只做黑名单/行为过滤
	// - consortium: "allowlist" - 证书许可 + IP 白名单
	// - private: "psk" - PSK + 内网限制
	Mode *string `json:"mode,omitempty"`
}

// UserCertificateManagementConfig 用户证书管理配置
// 仅用于联盟链，定义证书管理相关配置
type UserCertificateManagementConfig struct {
	// CA Bundle 文件路径
	// 包含联盟根 CA / 中间 CA 的证书包
	CABundlePath *string `json:"ca_bundle_path,omitempty"`

	// 信任的根 CA 文件路径列表（可选，多 CA 支持）
	// 如果提供，将使用这些路径的 CA 证书，而不是单一的 ca_bundle_path
	TrustedRoots []string `json:"trusted_roots,omitempty"`

	// 是否允许中间 CA（Intermediate CA）
	// true：允许中间 CA 签发的证书
	// false：只接受根 CA 直接签发的证书
	IntermediateAllowed *bool `json:"intermediate_allowed,omitempty"`

	// 允许的证书 Subject 白名单（可选）
	// 格式：["CN=node1.example.com", "CN=node2.example.com"]
	// 如果配置，只有 Subject 匹配的证书才能通过验证
	AllowedSubjects []string `json:"allowed_subjects,omitempty"`

	// 允许的组织（Organization）白名单（可选）
	// 格式：["Bank A", "Bank B"]
	// 如果配置，只有 Organization 匹配的证书才能通过验证
	AllowedOrgs []string `json:"allowed_orgs,omitempty"`

	// CRL URLs（证书吊销列表 URL，后续可选）
	// 用于检查证书是否已被吊销
	CRLURLs []string `json:"crl_urls,omitempty"`

	// OCSP URLs（在线证书状态协议 URL，后续可选）
	// 用于实时检查证书状态
	OCSPURLs []string `json:"ocsp_urls,omitempty"`
}

// UserPSKConfig 用户 PSK 配置
// 仅用于私有链，定义预共享密钥配置
type UserPSKConfig struct {
	// PSK 文件路径
	// 由工具或运维生成，不建议手工编辑明文密钥
	File *string `json:"file,omitempty"`
}

// UserStorageConfig 用户存储配置
// 只包含 JSON 配置文件中实际出现的字段。
// 在 v1 之后，统一使用 data_root 作为“数据根目录（data_root）”，
// 实际链实例数据目录由 data_root + Environment + 链实例信息组合得到。
type UserStorageConfig struct {
	DataRoot *string `json:"data_root,omitempty"` // 数据根目录（data_root）
}

// UserLogConfig 用户日志配置
// 只包含JSON配置文件中实际出现的字段
type UserLogConfig struct {
	Level    *string `json:"level,omitempty"`     // 日志级别：debug, info, warn, error, fatal
	FilePath *string `json:"file_path,omitempty"` // 日志文件路径
}

// UserMemoryMonitoringConfig 用户内存监控配置
// 只包含JSON配置文件中实际出现的字段
type UserMemoryMonitoringConfig struct {
	// Mode 内存监控模式：minimal | heuristic | accurate
	// - minimal: 只统计对象数，ApproxBytes 一律为 0（适合 dev 环境，减少开销）
	// - heuristic: 对能获取真实统计的模块计算 ApproxBytes（如 proto.Size），其他为 0（默认，适合大多数场景）
	// - accurate: 所有模块尽可能计算 ApproxBytes（包括基于配置的估算，适合 prod 环境）
	Mode *string `json:"mode,omitempty"`

	// 🆕 内存保护配置
	MemoryGuard *UserMemoryGuardConfig `json:"memory_guard,omitempty"`
}

// UserMemoryGuardConfig 内存保护守护程序配置
type UserMemoryGuardConfig struct {
	// Enabled 是否启用内存保护（默认 true）
	Enabled *bool `json:"enabled,omitempty"`

	// SoftLimitMB 软限制（MB）
	// 超过此限制时触发 GC
	// 默认 3072（3GB）
	SoftLimitMB *uint64 `json:"soft_limit_mb,omitempty"`

	// HardLimitMB 硬限制（MB）
	// 超过此限制时强制清理缓存 + GC
	// 默认 4096（4GB）
	HardLimitMB *uint64 `json:"hard_limit_mb,omitempty"`

	// AutoProfile 是否自动保存 heap profile（当 RSS 超过 HardLimit 时）
	// 默认 true
	AutoProfile *bool `json:"auto_profile,omitempty"`

	// ProfileOutputDir heap profile 输出目录
	// 默认 "data/pprof"
	ProfileOutputDir *string `json:"profile_output_dir,omitempty"`

	// CheckIntervalSeconds 检查间隔（秒）
	// 默认 30
	CheckIntervalSeconds *int `json:"check_interval_seconds,omitempty"`
}

// 配置辅助函数
// 这些函数帮助创建指针类型的配置值，区分"未设置"和"设置为零值"

// BoolPtr 创建bool指针，用于明确表示用户设置了该值
func BoolPtr(v bool) *bool {
	return &v
}

// IntPtr 创建int指针，用于明确表示用户设置了该值
func IntPtr(v int) *int {
	return &v
}

// StringPtr 创建string指针，用于明确表示用户设置了该值
func StringPtr(v string) *string {
	return &v
}

// UInt64Ptr 创建uint64指针，用于明确表示用户设置了该值
func UInt64Ptr(v uint64) *uint64 {
	return &v
}

// UserSignerConfig 用户签名器配置
// 只包含JSON配置文件中实际出现的字段
// ⚠️ 注意：这是内部配置，通常不暴露给用户，但允许通过环境变量或配置文件提供
type UserSignerConfig struct {
	// 签名器类型（local, kms, hsm）
	Type string `json:"type,omitempty"`

	// 本地签名器配置
	Local *UserLocalSignerConfig `json:"local,omitempty"`

	// KMS签名器配置
	KMS *UserKMSSignerConfig `json:"kms,omitempty"`

	// HSM签名器配置
	HSM *UserHSMSignerConfig `json:"hsm,omitempty"`
}

// GetStartupMode 返回启动同步模式（带默认值推导）
//
// 如果配置中未设置 startup_mode，则根据环境推导：
// - dev: 默认 from_genesis（便于本地开发）
// - test/prod: 默认 from_network（生产安全默认值）
func (c *AppConfig) GetStartupMode() StartupMode {
	if c.Sync == nil || c.Sync.StartupMode == nil || *c.Sync.StartupMode == "" {
		// 按环境给默认值
		env := ""
		if c.Environment != nil {
			env = *c.Environment
		}
		if env == "dev" {
			return StartupModeFromGenesis
		}
		return StartupModeFromNetwork
	}
	return StartupMode(*c.Sync.StartupMode)
}

// GetEnvironment 返回运行环境
func (c *AppConfig) GetEnvironment() Environment {
	if c.Environment == nil || *c.Environment == "" {
		return EnvDev // 默认 dev
	}
	return Environment(*c.Environment)
}

// UserLocalSignerConfig 用户本地签名器配置
type UserLocalSignerConfig struct {
	PrivateKeyHex string `json:"private_key_hex,omitempty"` // 私钥（Hex编码）
	Algorithm     string `json:"algorithm,omitempty"`       // 签名算法
	Environment   string `json:"environment,omitempty"`     // 环境标识
}

// UserKMSSignerConfig 用户KMS签名器配置
type UserKMSSignerConfig struct {
	KeyID         string `json:"key_id,omitempty"`
	Algorithm     string `json:"algorithm,omitempty"`
	RetryCount    int    `json:"retry_count,omitempty"`
	RetryDelayMs  int    `json:"retry_delay_ms,omitempty"`
	SignTimeoutMs int    `json:"sign_timeout_ms,omitempty"`
	Environment   string `json:"environment,omitempty"`
}

// UserHSMSignerConfig 用户HSM签名器配置
type UserHSMSignerConfig struct {
	KeyID           string `json:"key_id,omitempty"`
	KeyLabel        string `json:"key_label,omitempty"`
	Algorithm       string `json:"algorithm,omitempty"`
	LibraryPath     string `json:"library_path,omitempty"`
	EncryptedPIN    string `json:"encrypted_pin,omitempty"`
	KMSKeyID        string `json:"kms_key_id,omitempty"`
	KMSType         string `json:"kms_type,omitempty"`
	VaultAddr       string `json:"vault_addr,omitempty"`
	VaultToken      string `json:"vault_token,omitempty"`
	VaultSecretPath string `json:"vault_secret_path,omitempty"`
	SessionPoolSize int    `json:"session_pool_size,omitempty"`
	Endpoint        string `json:"endpoint,omitempty"`
	Username        string `json:"username,omitempty"`
	Password        string `json:"password,omitempty"`
	Environment     string `json:"environment,omitempty"`
}

// UserEUTXOConfig EUTXO配置
// 对应配置文件中的 eutxo 字段
type UserEUTXOConfig struct {
	// StartupHealthCheck 启动时健康检查配置
	StartupHealthCheck *UserStartupHealthCheckConfig `json:"startup_health_check,omitempty"`

	// Snapshot 快照配置
	Snapshot *UserSnapshotConfig `json:"snapshot,omitempty"`
}

// UserStartupHealthCheckConfig 启动时健康检查配置
type UserStartupHealthCheckConfig struct {
	// Enabled 是否启用启动时健康检查
	Enabled *bool `json:"enabled,omitempty"`

	// AutoRepair 是否自动修复损坏的UTXO
	AutoRepair *bool `json:"auto_repair,omitempty"`
}

// UserSnapshotConfig 快照配置
type UserSnapshotConfig struct {
	// CorruptUTXOPolicy 损坏UTXO处理策略
	// - "reject": 严格模式，拒绝创建快照
	// - "repair": 修复模式，自动修复并继续
	// - "warn": 告警模式，记录日志但继续
	CorruptUTXOPolicy *string `json:"corrupt_utxo_policy,omitempty"`

	// MaxRepairableCount 最多自动修复的UTXO数量
	MaxRepairableCount *int `json:"max_repairable_count,omitempty"`
}
