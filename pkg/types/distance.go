// distance.go
// XOR距离选择相关类型定义
//
// 本文件定义距离寻址选择算法相关的数据类型，用于替代复杂的多因子评分系统。
// 基于XOR距离的确定性区块选择，简化共识算法并提高性能。
//
// 设计原则：
// - 确定性：相同输入必产生相同结果
// - 简洁性：最小化数据结构和算法复杂度
// - 可验证性：支持选择过程的验证和证明
// - 高性能：优化大整数计算和内存使用
//
// 作者：WES开发团队
// 创建时间：2025-09-14

package types

import (
	"errors"
	"math/big"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// ==================== XOR距离计算结果 ====================

// DistanceResult XOR距离计算结果
//
// 🎯 **距离结果**: 候选区块与父区块的XOR距离计算结果
// 📋 **选择依据**: 聚合器选择最优候选的唯一依据
type DistanceResult struct {
	Candidate    *CandidateBlock `json:"candidate"`     // 候选区块
	Distance     *big.Int        `json:"distance"`      // XOR距离值
	CalculatedAt time.Time       `json:"calculated_at"` // 计算时间
}

// DistanceStatistics 距离计算统计信息
//
// 🎯 **统计监控**: 距离计算过程的性能统计
type DistanceStatistics struct {
	TotalCalculations uint64        `json:"total_calculations"` // 总计算次数
	AverageTime       time.Duration `json:"average_time"`       // 平均计算时间
	LastCalculatedAt  time.Time     `json:"last_calculated_at"` // 最后计算时间
}

// ==================== 距离选择证明 ====================

// DistanceSelectionProof 距离选择证明
//
// 🎯 **选择证明**: 为基于XOR距离的选择决策生成可验证证明
// 📋 **共识保证**: 确保选择过程的透明性和可验证性
type DistanceSelectionProof struct {
	// 基本信息
	SelectedBlockHash []byte `json:"selected_block_hash"` // 选中区块哈希
	ParentBlockHash   []byte `json:"parent_block_hash"`   // 父区块哈希
	SelectedDistance  string `json:"selected_distance"`   // 选中区块的距离值（big.Int字符串）

	// 证明数据
	TotalCandidates    uint32            `json:"total_candidates"`     // 总候选数量
	DistanceSummary    []byte            `json:"distance_summary"`     // 所有距离计算的摘要哈希
	TieBreakingApplied bool              `json:"tie_breaking_applied"` // 是否应用了tie-breaking
	TieBreakingProof   *TieBreakingProof `json:"tie_breaking_proof"`   // tie-breaking证明

	// 算法元数据
	Algorithm      string        `json:"algorithm"`       // 算法标识 "xor_distance_v1"
	GeneratedAt    time.Time     `json:"generated_at"`    // 证明生成时间
	GenerationTime time.Duration `json:"generation_time"` // 证明生成耗时
	ProofHash      []byte        `json:"proof_hash"`      // 证明哈希
}

// TieBreakingProof Tie-breaking证明
//
// 🎯 **Tie处理**: 当多个候选具有相同最小距离时的tie-breaking证明
type TieBreakingProof struct {
	TiedBlockHashes   [][]byte `json:"tied_block_hashes"`   // 所有tie的区块哈希
	TiedCount         uint32   `json:"tied_count"`          // tie的区块数量
	BreakingStrategy  string   `json:"breaking_strategy"`   // tie-breaking策略（如："lexicographic_hash"）
	SelectedBlockHash []byte   `json:"selected_block_hash"` // tie-breaking选中的区块哈希
}

// ==================== 距离选择结果分发 ====================

// DistanceDistributionMessage 基于距离选择的分发消息
//
// 🎯 **分发载体**: 聚合结果的网络分发消息（距离选择版）
type DistanceDistributionMessage struct {
	// 核心内容
	SelectedBlock  *CandidateBlock         `json:"selected_block"`  // 选中的区块
	SelectionProof *DistanceSelectionProof `json:"selection_proof"` // 距离选择证明

	// 分发信息
	AggregatorID peer.ID       `json:"aggregator_id"` // 聚合器ID
	MessageID    string        `json:"message_id"`    // 消息ID
	Timestamp    time.Time     `json:"timestamp"`     // 分发时间
	TTL          time.Duration `json:"ttl"`           // 消息TTL

	// 网络信息
	Priority    int       `json:"priority"`     // 分发优先级
	TargetPeers []peer.ID `json:"target_peers"` // 目标节点列表
}

// ==================== 距离选择错误类型 ====================

// 定义距离选择相关的错误类型
var (
	// 距离计算错误
	ErrNoDistanceResults        = errors.New("没有距离计算结果")
	ErrDistanceValidationFailed = errors.New("距离验证失败")
	ErrSelectedBlockNotFound    = errors.New("选中区块未找到")
	ErrInvalidSelection         = errors.New("无效的区块选择")
	ErrInvalidTieBreaking       = errors.New("无效的tie-breaking")

	// 证明验证错误
	ErrProofHashMismatch              = errors.New("证明哈希不匹配")
	ErrDistanceValueMismatch          = errors.New("距离值不匹配")
	ErrInvalidProofHash               = errors.New("无效的证明哈希")
	ErrMissingTieBreakingProof        = errors.New("缺少tie-breaking证明")
	ErrUnsupportedTieBreakingStrategy = errors.New("不支持的tie-breaking策略")
	ErrSelectedHashNotInTieList       = errors.New("选中哈希不在tie列表中")
	ErrInvalidLexicographicSelection  = errors.New("无效的字典序选择")

	// 证明结构错误
	ErrEmptySelectedBlockHash = errors.New("选中区块哈希为空")
	ErrEmptyParentBlockHash   = errors.New("父区块哈希为空")
	ErrEmptySelectedDistance  = errors.New("选中距离为空")
	ErrUnsupportedAlgorithm   = errors.New("不支持的算法")
)

// ==================== 兼容性类型别名 ====================

// DistanceBasedSelection 距离选择结果
//
// 🎯 **简化接口**: 提供向后兼容的选择结果接口
type DistanceBasedSelection struct {
	SelectedCandidate *CandidateBlock  `json:"selected_candidate"` // 选中的候选
	MinDistance       string           `json:"min_distance"`       // 最小距离值
	AllResults        []DistanceResult `json:"all_results"`        // 所有计算结果
	SelectionTime     time.Duration    `json:"selection_time"`     // 选择耗时
	TieBreakApplied   bool             `json:"tie_break_applied"`  // 是否应用tie-breaking
}
