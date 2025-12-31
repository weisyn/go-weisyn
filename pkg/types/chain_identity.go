// Package types 提供链身份相关的类型定义
package types

// ChainMode 链模式
type ChainMode string

const (
	ChainModePublic     ChainMode = "public"
	ChainModeConsortium ChainMode = "consortium"
	ChainModePrivate    ChainMode = "private"
)

// ChainIdentity 链身份标识
//
// 🎯 **链身份统一表示**
//
// 用于在所有跨节点通信中明确标识"这是不是同一条链"。
// 包含链的核心标识信息：chain_id、network_namespace、genesis_hash 等。
//
// 设计原则：
// - 确定性：相同配置产生相同的 ChainIdentity
// - 不可伪造：genesis_hash 确保无法伪造链身份
// - 可验证：所有字段都可以独立验证
type ChainIdentity struct {
	ChainID          string    `json:"chain_id"`          // 链ID（数字字符串或十六进制）
	NetworkNamespace string    `json:"network_namespace"` // 网络命名空间（如 "mainnet-public", "test-consortium"）
	NetworkID        string    `json:"network_id"`        // 网络标识符（如 "WES_mainnet_2025"）
	ChainMode        ChainMode `json:"chain_mode"`        // 链模式：public | consortium | private
	GenesisHash      string    `json:"genesis_hash"`      // 创世区块哈希（十六进制字符串，32字节）
	VersionTag       string    `json:"version_tag"`       // 版本标签（可选，如 "v1", "v1.1-hotfix"）
}

// IsSameChain 判断两个链身份是否代表同一条链
//
// 判断标准：
// - chain_id 必须相同
// - network_namespace 必须相同
// - network_id 必须相同（用于区分同命名空间下的不同网络/部署）
// - genesis_hash 必须相同（核心约束）
// - chain_mode 必须相同
func (c ChainIdentity) IsSameChain(other ChainIdentity) bool {
	return c.ChainID == other.ChainID &&
		c.NetworkNamespace == other.NetworkNamespace &&
		c.NetworkID == other.NetworkID &&
		c.GenesisHash == other.GenesisHash &&
		c.ChainMode == other.ChainMode
}

// String 返回链身份的字符串表示（用于日志）
//
// 格式：{network_namespace}/{chain_mode}/{chain_id}@{genesis_hash[:8]}
func (c ChainIdentity) String() string {
	hashPrefix := ""
	if len(c.GenesisHash) >= 8 {
		hashPrefix = c.GenesisHash[:8]
	} else if len(c.GenesisHash) > 0 {
		hashPrefix = c.GenesisHash
	}
	return c.NetworkNamespace + "/" + string(c.ChainMode) + "/" + c.ChainID + "@" + hashPrefix
}

// IsValid 验证链身份是否有效
//
// 检查所有必填字段是否已设置
func (c ChainIdentity) IsValid() bool {
	return c.ChainID != "" &&
		c.NetworkNamespace != "" &&
		c.NetworkID != "" &&
		c.GenesisHash != "" &&
		c.ChainMode != ""
}
