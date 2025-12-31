// Package types 提供统一的创世配置类型定义
package types

// GenesisConfig 创世区块配置
//
// 🎯 **统一配置结构**
//
// 本结构体与configs/genesis.json的JSON格式完全对应，
// 用于解析创世配置文件并生成确定性的创世区块。
//
// 设计原则：
// - 完全匹配JSON结构：确保配置文件能正确解析
// - 确定性：相同配置产生相同创世区块
// - 可扩展性：支持未来新增配置字段
type GenesisConfig struct {
	// 网络基础信息
	NetworkID string `json:"network_id"` // 网络标识，如 "WES_testnet"
	ChainID   uint64 `json:"chain_id"`   // 链ID，如 12345

	// 创世账户配置
	GenesisAccounts []GenesisAccount `json:"genesis_accounts"` // 预分配账户列表

	// 时间配置 (可选，如果不提供则使用当前时间)
	Timestamp int64 `json:"timestamp,omitempty"` // 创世时间戳
}

// GenesisAccount 创世账户配置
//
// 🎯 **账户预分配配置**
//
// 定义创世区块中的初始代币分配，每个账户包含：
// - 身份信息：名称、公钥、地址
// - 分配信息：初始余额、地址类型
type GenesisAccount struct {
	// 身份标识
	Name      string `json:"name"`       // 账户名称（用于识别，不影响链状态）
	PublicKey string `json:"public_key"` // 十六进制公钥字符串

	// 资产分配
	InitialBalance string `json:"initial_balance"` // 初始余额（字符串格式，支持大数）

	// 地址信息（用于验证）
	Address     string `json:"address"`      // 期望的地址（用于配置验证）
	AddressType string `json:"address_type"` // 地址类型，如 "bitcoin_style"

	// 私钥（仅用于测试网络，生产网络不应包含）
	PrivateKey string `json:"private_key,omitempty"` // 私钥（测试用）
}

// 注意：以下业务逻辑函数已移除，应移到业务层：
// - ValidateGenesisConfig() - 应移到 internal/core/genesis/validator.go
// - GetTotalSupply() - 应移到 internal/core/genesis/service.go
//
// types 包只应包含数据结构定义，不应包含验证或计算逻辑
