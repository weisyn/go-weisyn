// Package node 提供节点配置相关的辅助函数
package node

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/weisyn/v1/pkg/types"
)

// BuildLocalChainIdentity 从 AppConfig 构造本地 ChainIdentity
//
// 🎯 **链身份构建**
//
// 基于应用配置和计算得到的 genesis hash，构建完整的链身份标识。
// 这是节点"认为自己是哪条链"的唯一来源。
//
// 参数：
//   - cfg: 应用配置
//   - genesisHash: 从 GenesisConfig 计算得到的创世区块哈希（十六进制字符串）
//
// 返回：
//   - ChainIdentity: 完整的链身份标识
func BuildLocalChainIdentity(cfg *types.AppConfig, genesisHash string) types.ChainIdentity {
	if cfg == nil || cfg.Network == nil {
		panic("AppConfig 或 Network 配置不能为空")
	}

	chainID := ""
	if cfg.Network.ChainID != nil {
		chainID = fmt.Sprintf("%d", *cfg.Network.ChainID)
	}

	networkNamespace := ""
	if cfg.Network.NetworkNamespace != nil {
		networkNamespace = *cfg.Network.NetworkNamespace
	}

	networkID := ""
	if cfg.Network.NetworkID != nil {
		networkID = *cfg.Network.NetworkID
	}

	chainMode := types.ChainModePublic
	if cfg.Network.ChainMode != nil {
		chainMode = types.ChainMode(*cfg.Network.ChainMode)
	}

	return types.ChainIdentity{
		ChainID:          chainID,
		NetworkNamespace: networkNamespace,
		NetworkID:        networkID,
		ChainMode:        chainMode,
		GenesisHash:      genesisHash,
		VersionTag:       "", // 可选，后续可以从配置中读取
	}
}

// CalculateGenesisHash 从 GenesisConfig 计算确定性的创世区块哈希
//
// 🎯 **确定性哈希计算**
//
// 对 GenesisConfig 的关键字段进行规范化序列化后计算 SHA256 哈希。
// 确保相同配置产生相同的哈希值。
//
// 计算策略：
// 1. 对关键字段进行规范化序列化（JSON with sorted keys）
// 2. 计算 SHA256 哈希
// 3. 返回十六进制字符串
//
// 注意：此函数只基于配置计算哈希，不依赖实际构建的区块。
// 实际区块的哈希可能因为 PoW nonce 而不同，但配置哈希是确定性的。
//
// 参数：
//   - genesis: 创世配置
//
// 返回：
//   - string: 创世配置哈希（十六进制字符串，64字符）
//   - error: 计算错误
func CalculateGenesisHash(genesis *types.GenesisConfig) (string, error) {
	if genesis == nil {
		return "", fmt.Errorf("genesis config 不能为空")
	}

	// 构建用于哈希计算的规范化结构
	// 只包含影响创世区块的关键字段
	hashInput := struct {
		NetworkID       string                      `json:"network_id"`
		ChainID         uint64                      `json:"chain_id"`
		Timestamp       int64                       `json:"timestamp"`
		GenesisAccounts []genesisAccountForHash     `json:"genesis_accounts"`
	}{
		NetworkID: genesis.NetworkID,
		ChainID:   genesis.ChainID,
		Timestamp: genesis.Timestamp,
	}

	// 转换账户列表，只包含影响状态的字段
	for _, acc := range genesis.GenesisAccounts {
		hashInput.GenesisAccounts = append(hashInput.GenesisAccounts, genesisAccountForHash{
			PublicKey:      acc.PublicKey,
			InitialBalance: acc.InitialBalance,
			Address:        acc.Address,
		})
	}

	// 对账户列表按 PublicKey 排序，确保确定性
	sort.Slice(hashInput.GenesisAccounts, func(i, j int) bool {
		return hashInput.GenesisAccounts[i].PublicKey < hashInput.GenesisAccounts[j].PublicKey
	})

	// 序列化为 JSON（使用 sorted keys）
	jsonBytes, err := json.Marshal(hashInput)
	if err != nil {
		return "", fmt.Errorf("序列化 genesis config 失败: %w", err)
	}

	// 计算 SHA256 哈希
	hash := sha256.Sum256(jsonBytes)

	// 返回十六进制字符串
	return hex.EncodeToString(hash[:]), nil
}

// genesisAccountForHash 用于哈希计算的账户结构（只包含影响状态的字段）
type genesisAccountForHash struct {
	PublicKey      string `json:"public_key"`
	InitialBalance string `json:"initial_balance"`
	Address        string `json:"address"`
}

