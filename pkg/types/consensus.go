// Package types provides consensus type definitions.
package types

import (
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// ==================== 矿工状态管理 ====================

// MinerState 矿工状态枚举
//
// 🎯 **状态定义**: 定义矿工系统的所有可能状态
// 📋 **状态流转**: Idle → Active → Paused/Stopping → Idle，或 Error → Idle
type MinerState int

const (
	MinerStateIdle     MinerState = iota // 空闲状态 - 初始状态和停止后状态
	MinerStateActive                     // 活跃状态 - 正在进行挖矿
	MinerStatePaused                     // 暂停状态 - 临时暂停挖矿
	MinerStateStopping                   // 停止中状态 - 正在停止过程中
	MinerStateError                      // 错误状态 - 遇到不可恢复错误
	MinerStateSyncing                    // 同步状态 - 正在同步区块链
)

// String 返回矿工状态的字符串表示
func (s MinerState) String() string {
	switch s {
	case MinerStateIdle:
		return "Idle"
	case MinerStateActive:
		return "Active"
	case MinerStatePaused:
		return "Paused"
	case MinerStateStopping:
		return "Stopping"
	case MinerStateError:
		return "Error"
	case MinerStateSyncing:
		return "Syncing"
	default:
		return "Unknown"
	}
}

// ==================== 聚合器状态管理 ====================

// AggregationState 聚合状态枚举
//
// 🎯 **状态定义**：聚合器的 8 状态流程控制
//
// 状态流程：
// Idle → Listening → Collecting → Evaluating → Selecting → Distributing → Idle
// （错误状态：Error, Paused 可从任意状态进入）
type AggregationState int

const (
	AggregationStateIdle         AggregationState = iota // 空闲状态，聚合节点生命周期结束
	AggregationStateListening                            // 监听新高度信号
	AggregationStateCollecting                           // 收集候选区块（收集窗口期）
	AggregationStateEvaluating                           // 评估候选区块质量
	AggregationStateSelecting                            // 选择最优候选区块
	AggregationStateDistributing                         // 分发选择结果
	AggregationStatePaused                               // 暂停状态（同步等）
	AggregationStateError                                // 错误状态
)

func (s AggregationState) String() string {
	switch s {
	case AggregationStateIdle:
		return "Idle"
	case AggregationStateListening:
		return "Listening"
	case AggregationStateCollecting:
		return "Collecting"
	case AggregationStateEvaluating:
		return "Evaluating"
	case AggregationStateSelecting:
		return "Selecting"
	case AggregationStateDistributing:
		return "Distributing"
	case AggregationStatePaused:
		return "Paused"
	case AggregationStateError:
		return "Error"
	default:
		return "Unknown"
	}
}

// StateTransition 状态转换记录（通用的）
//
// 🎯 **历史追踪**: 记录每次状态转换的详细信息
// 📋 **审计支持**: 支持状态变更的完整审计
type StateTransition struct {
	FromState    string    `json:"from_state"`    // 源状态（字符串，支持不同状态类型）
	ToState      string    `json:"to_state"`      // 目标状态
	Timestamp    time.Time `json:"timestamp"`     // 转换时间
	Reason       string    `json:"reason"`        // 转换原因
	Success      bool      `json:"success"`       // 转换是否成功
	ErrorMessage string    `json:"error_message"` // 错误信息（如果失败）
}

// ==================== PoW挖矿参数 ====================

// MiningParameters PoW挖矿参数配置
//
// 🎯 **挖矿配置**: 定义PoW挖矿的所有配置参数
// 📋 **性能调优**: 支持根据硬件条件和业务需求调整参数
type MiningParameters struct {
	TargetDifficulty uint64        `json:"target_difficulty"` // 目标难度值
	BlockInterval    time.Duration `json:"block_interval"`    // 目标出块间隔
	MiningTimeout    time.Duration `json:"mining_timeout"`    // 单次挖矿超时时间
	LoopInterval     time.Duration `json:"loop_interval"`     // 挖矿循环间隔
	MaxTransactions  int           `json:"max_transactions"`  // 区块最大交易数
	MinTransactions  int           `json:"min_transactions"`  // 区块最小交易数
	TxSelectionMode  string        `json:"tx_selection_mode"` // 交易选择模式
}

// ==================== 候选区块与验证相关 ====================

// CollectionProgress - 现在定义在 candidate.go 中

// ==================== 候选区块验证与统计 ====================

// CandidateValidationResult 候选区块验证结果
//
// 🎯 **验证结果**: 候选区块的基础验证结果
// 📋 **验证状态**: 记录验证是否通过及验证时间
type CandidateValidationResult struct {
	IsValid        bool      `json:"is_valid"`        // 是否有效
	ValidatedAt    time.Time `json:"validated_at"`    // 验证时间
	ValidationTime int64     `json:"validation_time"` // 验证耗时(毫秒)
}

// EvaluationStats 验证统计信息
//
// 🎯 **统计监控**: 候选区块基础验证过程的统计信息
type EvaluationStats struct {
	TotalCandidates     int           `json:"total_candidates"`       // 总候选数量
	ValidCandidates     int           `json:"valid_candidates"`       // 有效候选数量
	EvaluationTime      time.Duration `json:"evaluation_time"`        // 验证总耗时
	AverageTimePerBlock time.Duration `json:"average_time_per_block"` // 平均每个区块验证时间
	LastEvaluationTime  time.Time     `json:"last_evaluation_time"`   // 最后验证时间
}

// ==================== 挖矿轮次信息 ====================

// MiningRoundInfo 挖矿轮次信息
//
// 🎯 **轮次追踪**: 单次挖矿轮次的详细信息
type MiningRoundInfo struct {
	RoundID    string    `json:"round_id"`              // 轮次ID
	Height     uint64    `json:"height"`                // 挖矿高度
	Difficulty uint32    `json:"difficulty"`            // 挖矿难度
	StartTime  time.Time `json:"start_time"`            // 开始时间
	Status     string    `json:"status"`                // 轮次状态
	BlockHash  string    `json:"block_hash,omitempty"`  // 区块哈希
	SubmitTime time.Time `json:"submit_time,omitempty"` // 提交时间
}

// ==================== 错误处理 ====================

// ProcessingError 区块处理错误
//
// 🎯 **错误封装**: 标准化的处理错误类型
// 📋 **错误分类**: 支持不同类型错误的精确分类和处理
type ProcessingError struct {
	Code      ErrorCode `json:"code"`               // 错误代码
	Message   string    `json:"message"`            // 错误消息
	MinerID   peer.ID   `json:"miner_id,omitempty"` // 相关矿工ID（可选）
	Timestamp time.Time `json:"timestamp"`          // 错误发生时间
}

// Error 实现error接口
func (pe *ProcessingError) Error() string {
	return pe.Message
}

// ErrorCode 错误代码枚举
type ErrorCode int

const (
	ErrCodeUnknown           ErrorCode = iota // 未知错误
	ErrCodeInvalidBlock                       // 无效区块
	ErrCodeInvalidPoW                         // PoW验证失败
	ErrCodeHeightConflict                     // 区块高度冲突
	ErrCodeNetworkFailure                     // 网络操作失败
	ErrCodeProcessingTimeout                  // 处理超时
	ErrCodeInternalError                      // 内部错误
)

// String 返回错误代码的字符串表示
func (ec ErrorCode) String() string {
	switch ec {
	case ErrCodeInvalidBlock:
		return "INVALID_BLOCK"
	case ErrCodeInvalidPoW:
		return "INVALID_POW"
	case ErrCodeHeightConflict:
		return "HEIGHT_CONFLICT"
	case ErrCodeNetworkFailure:
		return "NETWORK_FAILURE"
	case ErrCodeProcessingTimeout:
		return "PROCESSING_TIMEOUT"
	case ErrCodeInternalError:
		return "INTERNAL_ERROR"
	default:
		return "UNKNOWN"
	}
}
