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

// AggregationState ABS聚合状态枚举
//
// 🎯 **状态定义**：ABS聚合器的8状态流程控制
//
// 状态流程：
// Idle → Listening → Collecting → Evaluating → Selecting → Distributing → Idle
// (错误状态：Error, Paused可从任意状态进入)
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

// ==================== ABS候选区块相关 ====================

// CollectionProgress - 现在定义在 candidate.go 中

// ==================== ABS评分系统 ====================

// ABSScore ABS综合评分结果
//
// 🎯 **智能评分**: ABS架构的多维度评分结果
// 📋 **决策依据**: 聚合器选择最优候选的核心依据
type ABSScore struct {
	// 分项评分
	PoWQualityScore float64 `json:"pow_quality_score"` // PoW质量评分 (40%)
	EconomicScore   float64 `json:"economic_score"`    // 经济价值评分 (30%)
	TimelinesScore  float64 `json:"timeliness_score"`  // 时效性评分 (20%)
	NetworkScore    float64 `json:"network_score"`     // 网络质量评分 (10%)

	// 综合评分
	TotalScore      float64 `json:"total_score"`      // 综合总分
	NormalizedScore float64 `json:"normalized_score"` // 标准化评分 (0-1)

	// 计算信息
	CalculatedAt    time.Time `json:"calculated_at"`    // 计算时间
	CalculationTime int64     `json:"calculation_time"` // 计算耗时(毫秒)
}

// ScoredCandidate 评分后的候选区块
//
// 🎯 **评分结果**: 候选区块与其ABS评分的组合
type ScoredCandidate struct {
	Candidate *CandidateBlock `json:"candidate"` // 候选区块
	Score     *ABSScore       `json:"score"`     // ABS评分
	Rank      int             `json:"rank"`      // 排名（1为最优）
}

// EvaluationStats 评估统计信息
//
// 🎯 **统计监控**: ABS评估过程的统计信息
type EvaluationStats struct {
	TotalCandidates     int           `json:"total_candidates"`       // 总候选数量
	ValidCandidates     int           `json:"valid_candidates"`       // 有效候选数量
	AverageScore        float64       `json:"average_score"`          // 平均评分
	MaxScore            float64       `json:"max_score"`              // 最高评分
	MinScore            float64       `json:"min_score"`              // 最低评分
	EvaluationTime      time.Duration `json:"evaluation_time"`        // 评估总耗时
	AverageTimePerBlock time.Duration `json:"average_time_per_block"` // 平均每个区块评估时间
	LastEvaluationTime  time.Time     `json:"last_evaluation_time"`   // 最后评估时间
}

// ==================== ABS选择证明 ====================

// SelectionProof ABS选择证明
//
// 🎯 **选择证明**: 为聚合器的选择决策生成可验证证明
// 📋 **共识保证**: 确保选择过程的透明性和可验证性
type SelectionProof struct {
	// 选择信息
	SelectedCandidate  *CandidateBlock `json:"selected_candidate"`  // 选中的候选
	SelectionReason    string          `json:"selection_reason"`    // 选择原因
	SelectionTimestamp time.Time       `json:"selection_timestamp"` // 选择时间

	// 证明数据
	AllCandidatesHash   string `json:"all_candidates_hash"`  // 所有候选的哈希
	ScoresHash          string `json:"scores_hash"`          // 评分结果哈希
	AggregatorSignature []byte `json:"aggregator_signature"` // 聚合器签名

	// 验证信息
	AggregatorID peer.ID `json:"aggregator_id"` // 聚合器ID
	BlockHeight  uint64  `json:"block_height"`  // 区块高度
	ProofHash    string  `json:"proof_hash"`    // 证明哈希
}

// ==================== ABS结果分发 ====================

// DistributionMessage ABS分发消息
//
// 🎯 **分发载体**: 聚合结果的网络分发消息
type DistributionMessage struct {
	// 核心内容
	SelectedBlock  *CandidateBlock `json:"selected_block"`  // 选中的区块
	SelectionProof *SelectionProof `json:"selection_proof"` // 选择证明

	// 分发信息
	AggregatorID peer.ID       `json:"aggregator_id"` // 聚合器ID
	MessageID    string        `json:"message_id"`    // 消息ID
	Timestamp    time.Time     `json:"timestamp"`     // 分发时间
	TTL          time.Duration `json:"ttl"`           // 消息TTL

	// 网络信息
	Priority    int       `json:"priority"`     // 分发优先级
	TargetPeers []peer.ID `json:"target_peers"` // 目标节点列表
}

// ConvergenceStatus 共识收敛状态
//
// 🎯 **收敛监控**: 全网对聚合结果的接受状态监控
type ConvergenceStatus struct {
	BlockHash          string        `json:"block_hash"`          // 区块哈希
	TotalNodes         int           `json:"total_nodes"`         // 总节点数
	AcceptingNodes     int           `json:"accepting_nodes"`     // 接受节点数
	RejectingNodes     int           `json:"rejecting_nodes"`     // 拒绝节点数
	UnknownNodes       int           `json:"unknown_nodes"`       // 未知状态节点数
	ConvergenceRatio   float64       `json:"convergence_ratio"`   // 收敛比例
	IsConverged        bool          `json:"is_converged"`        // 是否已收敛
	ConvergedAt        *time.Time    `json:"converged_at"`        // 收敛时间
	MonitoringDuration time.Duration `json:"monitoring_duration"` // 监控持续时间
}

// DistributionStats 分发统计信息
//
// 🎯 **分发监控**: ABS结果分发的统计信息
type DistributionStats struct {
	TotalDistributions   uint64        `json:"total_distributions"`    // 总分发次数
	SuccessfulSends      uint64        `json:"successful_sends"`       // 成功发送次数
	FailedSends          uint64        `json:"failed_sends"`           // 失败发送次数
	AverageLatency       time.Duration `json:"average_latency"`        // 平均延迟
	MaxLatency           time.Duration `json:"max_latency"`            // 最大延迟
	MinLatency           time.Duration `json:"min_latency"`            // 最小延迟
	LastDistributionTime time.Time     `json:"last_distribution_time"` // 最后分发时间
	NetworkCoverage      float64       `json:"network_coverage"`       // 网络覆盖率
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
