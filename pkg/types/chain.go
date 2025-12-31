// Package types provides blockchain type definitions.
package types

// ChainInfo 区块链综合信息（跨模块数据结构）
//
// 用于一次性返回链的关键状态数据，便于上层聚合查询与展示。
type ChainInfo struct {
	// 基础状态
	Height        uint64 `json:"height"`          // 当前区块高度
	BestBlockHash []byte `json:"best_block_hash"` // 最佳区块哈希

	// 系统状态
	IsReady bool   `json:"is_ready"` // 系统是否就绪可用
	Status  string `json:"status"`   // 链状态详细描述
	// Status可能的值：
	// - "normal": 正常运行状态，可处理交易和区块
	// - "syncing": 正在同步中，暂不可用
	// - "fork_processing": 正在处理分叉，系统锁定中
	// - "error": 系统错误状态，需人工干预
	// - "maintenance": 维护状态，计划性停机

	// 网络信息
	NetworkHeight uint64 `json:"network_height"` // 网络高度（可选）
	PeerCount     int    `json:"peer_count"`     // 连接的节点数（可选）

	// 时间信息
	LastBlockTime int64 `json:"last_block_time"` // 最后区块时间戳
	Uptime        int64 `json:"uptime"`          // 系统运行时间（秒）

	// 节点模式（Light/Full）
	NodeMode NodeMode `json:"node_mode"`
}

// NodeMode 节点模式（全局统一枚举）
//
// 说明：
// - Light：轻节点，仅同步并保存区块头；不持久化区块体
// - Full：全节点，同步并保存完整区块（头+体）
type NodeMode string

const (
	// NodeModeLight 轻节点模式
	NodeModeLight NodeMode = "light"
	// NodeModeFull 全节点模式
	NodeModeFull NodeMode = "full"
)

// IsValidNodeMode 校验节点模式是否合法
func IsValidNodeMode(mode NodeMode) bool {
	return mode == NodeModeLight || mode == NodeModeFull
}
