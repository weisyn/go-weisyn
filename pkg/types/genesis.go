// Package types 提供统一的创世配置类型定义
package types

import "fmt"

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

// ValidateGenesisConfig 验证创世配置的完整性
//
// 🎯 **配置完整性验证**
//
// 验证创世配置的基本完整性和一致性：
// 1. 必填字段检查
// 2. 数据格式验证
// 3. 逻辑一致性验证
//
// 参数：
//
//	config: 创世配置
//
// 返回：
//
//	error: 验证错误，nil表示验证通过
func ValidateGenesisConfig(config *GenesisConfig) error {
	if config == nil {
		return fmt.Errorf("创世配置不能为空")
	}

	// 验证基础字段
	if config.NetworkID == "" {
		return fmt.Errorf("网络ID不能为空")
	}

	if config.ChainID == 0 {
		return fmt.Errorf("链ID不能为0")
	}

	// 验证账户配置
	if len(config.GenesisAccounts) == 0 {
		return fmt.Errorf("至少需要一个创世账户")
	}

	// 验证每个账户
	publicKeys := make(map[string]bool)
	addresses := make(map[string]bool)

	for i, account := range config.GenesisAccounts {
		if account.PublicKey == "" {
			return fmt.Errorf("账户[%d]的公钥不能为空", i)
		}

		if account.InitialBalance == "" || account.InitialBalance == "0" {
			return fmt.Errorf("账户[%d]的初始余额不能为空或为0", i)
		}

		// 检查重复
		if publicKeys[account.PublicKey] {
			return fmt.Errorf("发现重复的公钥: %s", account.PublicKey)
		}
		publicKeys[account.PublicKey] = true

		if account.Address != "" && addresses[account.Address] {
			return fmt.Errorf("发现重复的地址: %s", account.Address)
		}
		if account.Address != "" {
			addresses[account.Address] = true
		}
	}

	return nil
}

// GetTotalSupply 计算创世区块的总供应量
//
// 🎯 **总供应量计算**
//
// 计算所有创世账户的初始余额总和，用于：
// 1. 配置验证
// 2. 经济模型验证
// 3. 审计和监控
//
// 参数：
//
//	config: 创世配置
//
// 返回：
//
//	uint64: 总供应量
//	error: 计算错误
func GetTotalSupply(config *GenesisConfig) (uint64, error) {
	total := uint64(0)

	for i, account := range config.GenesisAccounts {
		// 解析余额字符串为数值
		// 注意：这里简化处理，实际应该支持大数解析
		var balance uint64
		if _, err := fmt.Sscanf(account.InitialBalance, "%d", &balance); err != nil {
			return 0, fmt.Errorf("解析账户[%d]余额失败: %w", i, err)
		}

		total += balance
	}

	return total, nil
}
