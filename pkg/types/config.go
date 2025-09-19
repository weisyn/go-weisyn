package types

// AppConfig 应用程序根配置
// 只包含JSON配置文件解析所需的结构，不包含任何内部字段
// 默认值和完整配置结构在 internal/config/*/defaults.go 和 internal/config/*/config.go 中定义
type AppConfig struct {
	// 应用程序基本信息
	AppName *string `json:"app_name,omitempty"` // 应用名称
	DataDir *string `json:"data_dir,omitempty"` // 数据目录路径
	Version *string `json:"version,omitempty"`  // 应用版本

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

	// === 保持向后兼容的字段 ===
	// 区块链配置
	Blockchain interface{} `json:"blockchain,omitempty"`

	// 共识配置
	Consensus interface{} `json:"consensus,omitempty"`

	// 存储配置
	Storage *UserStorageConfig `json:"storage,omitempty"`

	// 日志配置
	Log *UserLogConfig `json:"log,omitempty"`
}

// UserNetworkConfig 用户网络身份配置
// 对应配置文件中的 network 字段
type UserNetworkConfig struct {
	ChainID          *uint64 `json:"chain_id,omitempty"`          // 链ID
	NetworkName      *string `json:"network_name,omitempty"`      // 网络名称
	NetworkNamespace *string `json:"network_namespace,omitempty"` // 网络命名空间（如"mainnet", "testnet", "dev"）
}

// UserGenesisConfig 用户创世配置
// 对应配置文件中的 genesis 字段
type UserGenesisConfig struct {
	Accounts []UserGenesisAccount `json:"accounts,omitempty"` // 创世账户列表
}

// UserGenesisAccount 用户创世账户配置
// 只包含JSON配置文件中实际出现的字段
type UserGenesisAccount struct {
	Name           string `json:"name,omitempty"`            // 账户名称
	PrivateKey     string `json:"private_key,omitempty"`     // 私钥
	Address        string `json:"address,omitempty"`         // 地址
	InitialBalance string `json:"initial_balance,omitempty"` // 初始余额（字符串形式支持大数）
}

// UserMiningConfig 用户挖矿配置
// 对应配置文件中的 mining 字段
type UserMiningConfig struct {
	TargetBlockTime  *string `json:"target_block_time,omitempty"`  // 目标出块时间（如："5s", "10s"）
	EnableAggregator *bool   `json:"enable_aggregator,omitempty"`  // 是否启用聚合器
	MaxMiningThreads *int    `json:"max_mining_threads,omitempty"` // 最大挖矿线程数
}

// UserNodeConfig 用户节点网络配置
// 只包含JSON配置文件中实际出现的字段
type UserNodeConfig struct {
	ListenAddresses []string `json:"listen_addresses,omitempty"` // P2P监听地址列表
	BootstrapPeers  []string `json:"bootstrap_peers,omitempty"`  // 引导节点列表

	EnableMDNS      *bool `json:"enable_mdns,omitempty"`       // 启用mDNS发现
	EnableDHT       *bool `json:"enable_dht,omitempty"`        // 启用DHT
	EnableNATPort   *bool `json:"enable_nat_port,omitempty"`   // 启用NAT端口映射
	EnableAutoRelay *bool `json:"enable_auto_relay,omitempty"` // 启用自动中继
	EnableDCUtR     *bool `json:"enable_dcutr,omitempty"`      // 启用打洞

	// P2P身份配置
	Host *UserHostConfig `json:"host,omitempty"` // 主机配置
}

// UserHostConfig 用户主机配置
// 只包含JSON配置文件中实际出现的字段
type UserHostConfig struct {
	Identity *UserIdentityConfig `json:"identity,omitempty"` // 身份配置
}

// UserIdentityConfig 用户身份配置
// 只包含JSON配置文件中实际出现的字段
type UserIdentityConfig struct {
	PrivateKey *string `json:"private_key,omitempty"` // base64编码的libp2p私钥
	KeyFile    *string `json:"key_file,omitempty"`    // 私钥文件路径
}

// UserAPIConfig 用户API配置
// 只包含JSON配置文件中实际出现的字段
type UserAPIConfig struct {
	HTTPEnabled *bool `json:"http_enabled,omitempty"` // 是否启用HTTP API
	HTTPPort    *int  `json:"http_port,omitempty"`    // HTTP监听端口

	GRPCEnabled *bool `json:"grpc_enabled,omitempty"` // 是否启用gRPC API
	GRPCPort    *int  `json:"grpc_port,omitempty"`    // gRPC监听端口

	WebSocketEnabled *bool `json:"websocket_enabled,omitempty"` // 是否启用WebSocket
	WebSocketPort    *int  `json:"websocket_port,omitempty"`    // WebSocket监听端口

	EnableMiningAPI *bool `json:"enable_mining_api,omitempty"` // 是否启用挖矿API
}

// UserStorageConfig 用户存储配置
// 只包含JSON配置文件中实际出现的字段
type UserStorageConfig struct {
	DataPath *string `json:"data_path,omitempty"` // 数据存储路径
}

// UserLogConfig 用户日志配置
// 只包含JSON配置文件中实际出现的字段
type UserLogConfig struct {
	Level    *string `json:"level,omitempty"`     // 日志级别：debug, info, warn, error, fatal
	FilePath *string `json:"file_path,omitempty"` // 日志文件路径
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
