// Package types provides candidate block type definitions.
package types

import (
	"time"

	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/libp2p/go-libp2p/core/peer"
)

// PoolOptions 用户可配置的候选区块池选项
type PoolOptions struct {
	MaxCandidates       int           `json:"max_candidates"`       // 最大候选区块数量
	MaxAge              time.Duration `json:"max_age"`              // 候选区块最大生存时间
	MemoryLimit         uint64        `json:"memory_limit"`         // 内存使用限制(字节)
	CleanupInterval     time.Duration `json:"cleanup_interval"`     // 清理任务执行间隔
	VerificationTimeout time.Duration `json:"verification_timeout"` // 验证超时时间
	PriorityEnabled     bool          `json:"priority_enabled"`     // 是否启用优先级排序
	MaxBlockSize        uint64        `json:"max_block_size"`       // 最大区块大小限制
}

// CandidateBlock 候选区块信息（MVP统一结构）
//
// 🎯 **MVP设计原则**：仅包含真实业务流程所需的核心字段
// 📋 **业务需求**：支持候选区块收集、验证、历史记录管理
type CandidateBlock struct {
	// 基础信息
	Block     *core.Block `json:"block"`      // 候选区块
	BlockHash []byte      `json:"block_hash"` // 区块哈希
	Height    uint64      `json:"height"`     // 区块高度

	// 来源信息
	MinerAddress []byte  `json:"miner_address"` // 矿工地址
	Source       peer.ID `json:"source"`        // 发送方节点ID
	FromPeer     string  `json:"from_peer"`     // 来源节点ID字符串
	LocalNode    bool    `json:"local_node"`    // 是否为本地节点产生

	// 时间信息
	ProducedAt time.Time `json:"produced_at"` // 区块生产时间
	ReceivedAt time.Time `json:"received_at"` // 收到时间

	// 验证信息
	Verified     bool      `json:"verified"`      // 是否已验证
	VerifiedAt   time.Time `json:"verified_at"`   // 验证时间
	VerifyErrors []string  `json:"verify_errors"` // 验证错误列表
	Valid        bool      `json:"valid"`         // 是否有效

	// 选择信息
	Selected   bool      `json:"selected"`    // 是否已被选中
	SelectedAt time.Time `json:"selected_at"` // 选中时间
	Expired    bool      `json:"expired"`     // 是否已过期

	// 优先级和质量信息
	Priority         int     `json:"priority"`          // 优先级
	Score            float64 `json:"score,omitempty"`  // [已废弃] 质量分数（PoW+XOR架构中不再使用，保留仅用于向后兼容）
	Difficulty       uint64  `json:"difficulty"`        // 难度值
	TransactionCount int     `json:"transaction_count"` // 交易数量
	EstimatedSize    int     `json:"estimated_size"`    // 预估大小

	// 状态信息
	SendStatus string `json:"send_status"` // 发送状态
}

// CollectionProgress 收集进度信息
type CollectionProgress struct {
	Height              uint64        `json:"height"`                // 目标高度
	WindowStartTime     time.Time     `json:"window_start_time"`     // 窗口启动时间
	WindowDuration      time.Duration `json:"window_duration"`       // 窗口持续时间
	WindowEndTime       time.Time     `json:"window_end_time"`       // 窗口结束时间
	IsActive            bool          `json:"is_active"`             // 窗口是否活跃
	CandidatesCollected int           `json:"candidates_collected"`  // 已收集候选数量
	CandidatesValidated int           `json:"candidates_validated"`  // 已验证候选数量
	CandidatesRejected  int           `json:"candidates_rejected"`   // 已拒绝候选数量
	DuplicatesDetected  int           `json:"duplicates_detected"`   // 检测到的重复数量
	AverageReceiveDelay time.Duration `json:"average_receive_delay"` // 平均接收延迟
	ProgressPercentage  float64       `json:"progress_percentage"`   // 进度百分比
}

// CollectionResult 收集结果
type CollectionResult struct {
	Height              uint64           `json:"height"`                // 目标高度
	TotalCandidates     int              `json:"total_candidates"`      // 总候选数量
	ValidCandidates     int              `json:"valid_candidates"`      // 有效候选数量
	RejectedCandidates  int              `json:"rejected_candidates"`   // 拒绝候选数量
	DuplicateCandidates int              `json:"duplicate_candidates"`  // 重复候选数量
	CollectionStartTime time.Time        `json:"collection_start_time"` // 收集开始时间
	CollectionEndTime   time.Time        `json:"collection_end_time"`   // 收集结束时间
	WindowDuration      time.Duration    `json:"window_duration"`       // 实际窗口持续时间
	AverageReceiveDelay time.Duration    `json:"average_receive_delay"` // 平均接收延迟
	NetworkCoverage     float64          `json:"network_coverage"`      // 网络覆盖率
	QualityScore        float64          `json:"quality_score"`         // 质量评分
	OptimalWindowSize   time.Duration    `json:"optimal_window_size"`   // 优化的窗口大小
	Candidates          []CandidateBlock `json:"candidates"`            // 收集到的候选区块
}
