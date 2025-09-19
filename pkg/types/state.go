package types

import (
	"time"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ================================================================================================
// 🎯 链状态相关类型
// ================================================================================================

// ChainTip 链顶信息
type ChainTip struct {
	Hash       *transaction.Hash `json:"hash"`       // 区块哈希
	Height     uint64            `json:"height"`     // 区块高度
	Timestamp  time.Time         `json:"timestamp"`  // 时间戳
	Difficulty uint64            `json:"difficulty"` // 难度
	TotalWork  uint64            `json:"total_work"` // 总工作量
	ChainWork  *ChainWork        `json:"chain_work"` // 链工作量
}

// ChainWork 链工作量
type ChainWork struct {
	Height    uint64  `json:"height"`     // 高度
	TotalWork uint64  `json:"total_work"` // 总工作量
	Work      []byte  `json:"work"`       // 工作量字节
	Target    []byte  `json:"target"`     // 目标值
	Score     float64 `json:"score"`      // 工作量评分
}

// ChainState 链状态
type ChainState struct {
	BestHeight uint64            `json:"best_height"` // 最佳高度
	BestHash   *transaction.Hash `json:"best_hash"`   // 最佳哈希
	TotalWork  uint64            `json:"total_work"`  // 总工作量
	Difficulty uint64            `json:"difficulty"`  // 当前难度
	LastUpdate time.Time         `json:"last_update"` // 最后更新
}

// ================================================================================================
// 🔀 分叉相关类型
// ================================================================================================

// ForkStatusType 分叉状态枚举
type ForkStatusType string

const (
	ForkStatusActive    ForkStatusType = "active"
	ForkStatusResolved  ForkStatusType = "resolved"
	ForkStatusAbandoned ForkStatusType = "abandoned"
)

// ForkStatus 分叉状态
type ForkStatus struct {
	HasFork    bool              `json:"has_fork"`    // 是否有分叉
	ForkHeight uint64            `json:"fork_height"` // 分叉高度
	ForkHash   *transaction.Hash `json:"fork_hash"`   // 分叉哈希
	MainHash   *transaction.Hash `json:"main_hash"`   // 主链哈希
	ForkLength uint32            `json:"fork_length"` // 分叉长度
}

// ResolutionResult 解决结果
type ResolutionResult struct {
	Success     bool              `json:"success"`       // 是否成功
	NewBestHash *transaction.Hash `json:"new_best_hash"` // 新的最佳哈希
	ReorgDepth  uint32            `json:"reorg_depth"`   // 重组深度
	Message     string            `json:"message"`       // 结果消息
}

// Checkpoint 检查点
type Checkpoint struct {
	Height    uint64            `json:"height"`    // 高度
	Hash      *transaction.Hash `json:"hash"`      // 哈希
	Timestamp time.Time         `json:"timestamp"` // 时间戳
	Verified  bool              `json:"verified"`  // 是否验证
}

// ================================================================================================
// 📸 快照相关类型
// ================================================================================================

// ⚠️ **未使用的快照类型 - 已注释**
// 以下类型未被任何接口使用，按照"只保留被实际使用的类型"原则进行注释

/*
// StateSnapshot 状态快照
// 📞 **使用者**: 此类型未被任何接口使用
type StateSnapshot struct {
	SnapshotHash *transaction.Hash `json:"snapshot_hash"` // 快照哈希
	BlockHeight  uint64     `json:"block_height"`  // 区块高度
	BlockHash    *transaction.Hash `json:"block_hash"`    // 区块哈希
	CreatedTime  time.Time  `json:"created_time"`  // 创建时间
}
*/

// ================================================================================================
// ✅ 验证相关类型
// ================================================================================================

// BlockValidationResult 区块验证结果
// 📞 **使用者**: pkg/interfaces/consensus/consensus.go 接口使用
type BlockValidationResult struct {
	IsValid      bool   `json:"is_valid"`      // 是否有效
	ErrorMessage string `json:"error_message"` // 错误消息
}
