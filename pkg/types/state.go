// Package types provides state type definitions.
package types

import (
	"fmt"
	"math/big"
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

// ChainWeight 链权重信息
//
// 🎯 **用途**：用于分叉选择时比较不同链的权重
//
// 📊 **权重指标**：
// - CumulativeDifficulty: 累积难度（主要指标）
// - BlockCount: 区块数量（次要指标）
// - TipHash: 链尖区块哈希（确定性 tie-break，必须全网一致）
// - LastBlockTime: 最后区块时间（观测指标，不应作为 tie-break）
//
// 📞 **使用者**：internal/core/chain/fork 分叉处理模块
type ChainWeight struct {
	CumulativeDifficulty *big.Int `json:"cumulative_difficulty"` // 累积难度
	BlockCount           uint64   `json:"block_count"`           // 区块数量
	TipHash              []byte   `json:"tip_hash,omitempty"`    // 链尖区块哈希（用于确定性 tie-break）
	LastBlockTime        int64    `json:"last_block_time"`       // 最后区块时间（Unix时间戳）
}

// String 实现 fmt.Stringer 接口（必要的格式化方法，用于日志和调试）
func (cw *ChainWeight) String() string {
	if cw == nil || cw.CumulativeDifficulty == nil {
		return "ChainWeight{nil}"
	}
	hashPrefix := ""
	if len(cw.TipHash) > 0 {
		n := 8
		if len(cw.TipHash) < n {
			n = len(cw.TipHash)
		}
		hashPrefix = fmt.Sprintf("%x", cw.TipHash[:n])
	}
	return fmt.Sprintf("ChainWeight{Difficulty:%s, Blocks:%d, TipHash:%s, Time:%d}",
		cw.CumulativeDifficulty.String(), cw.BlockCount, hashPrefix, cw.LastBlockTime)
}

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
// 注意：StateSnapshot 类型已被移除（未使用），如需使用可从 git 历史中恢复

// ================================================================================================
// 📸 UTXO快照相关类型
// ================================================================================================

// UTXOSnapshotData UTXO快照数据
// 📞 **使用者**: pkg/interfaces/eutxo/snapshot.go 接口使用
type UTXOSnapshotData struct {
	SnapshotID  string            `json:"snapshot_id"`  // 快照ID
	Height      uint64            `json:"height"`       // 快照高度
	BlockHash   *transaction.Hash `json:"block_hash"`   // 区块哈希
	StateRoot   []byte            `json:"state_root"`   // 状态根
	UTXOCount   uint64            `json:"utxo_count"`   // UTXO数量
	CreatedTime time.Time         `json:"created_time"` // 创建时间
}

// ================================================================================================
// ✅ 验证相关类型
// ================================================================================================

// 注意：BlockValidationResult 类型已被移除（未使用）
// 如需使用，可从 git 历史中恢复
