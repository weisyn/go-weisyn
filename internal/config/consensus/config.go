package consensus

import (
	"fmt"
	"strings"
	"time"
)

// ConsensusOptions 共识配置选项
// 采用分层结构，为不同角色提供专门的配置组
type ConsensusOptions struct {
	// 基础共识配置
	ConsensusType   string        `json:"consensus_type"`
	TargetBlockTime time.Duration `json:"target_block_time"`
	BlockSizeLimit  uint64        `json:"block_size_limit"`

	// 角色特定配置
	Miner      MinerConfig      `json:"miner"`      // 矿工角色配置
	Aggregator AggregatorConfig `json:"aggregator"` // 聚合器角色配置

	// 共享的 POW 配置
	POW POWConfig `json:"pow"`

	// 网络和同步配置
	Network NetworkConfig `json:"network"`

	// 验证和安全配置
	Validation ValidationConfig `json:"validation"`

	// 性能和监控配置
	Performance PerformanceConfig `json:"performance"`

	// 内部配置
	ConsensusTypes        []string               `json:"-"`
	ValidationLevels      map[string]bool        `json:"-"`
	PerformanceThresholds map[string]interface{} `json:"-"`
}

// MinerConfig 矿工角色专属配置
type MinerConfig struct {
	// 挖矿控制参数
	MiningTimeout time.Duration `json:"mining_timeout"` // 挖矿超时时间（0 表示不限制；推荐默认不限制，由外部 ctx/运维策略控制）
	// PoWSlice 旧版“slice mining”参数。
	//
	// 说明：
	// - slice mining 会把 PoW 强行切成固定时间片并频繁重建候选块；
	// - 在高难度/低算力下会显著降低有效算力，表现为卡高度；
	// - 当前实现已不再使用该参数（保留字段仅为兼容配置文件）。
	PoWSlice        time.Duration `json:"pow_slice"`
	LoopInterval    time.Duration `json:"loop_interval"`     // 挖矿循环间隔
	MaxTransactions uint32        `json:"max_transactions"`  // 每个区块最大交易数
	MinTransactions uint32        `json:"min_transactions"`  // 每个区块最小交易数
	TxSelectionMode string        `json:"tx_selection_mode"` // 交易选择模式

	// 资源控制
	MaxCPUUsage    float64 `json:"max_cpu_usage"`    // 最大CPU使用率
	MaxMemoryUsage uint64  `json:"max_memory_usage"` // 最大内存使用量
	MaxGoroutines  int     `json:"max_goroutines"`   // 最大协程数

	// 网络发送参数
	SendRetryCount int           `json:"send_retry_count"` // 发送重试次数
	SendTimeout    time.Duration `json:"send_timeout"`     // 发送超时时间
	DecisionNodes  int           `json:"decision_nodes"`   // 目标决策节点数

	// 区块生产控制
	MaxCandidatesBuffer       int           `json:"max_candidates_buffer"`       // 最大候选区块缓冲数
	ConfirmationTimeout       time.Duration `json:"confirmation_timeout"`        // 确认超时时间
	ConfirmationCheckInterval time.Duration `json:"confirmation_check_interval"` // 确认检查间隔

	// ========== v2：确认门闸退路（非兼容） ==========
	// ⚠️ 系统内不存在“单节点模式”，因此退路只允许：
	// - "sync": 触发一次同步并继续挖矿（默认）
	// - "drop": 丢弃本轮确认跟踪（仅记录诊断）并继续挖矿
	ConfirmationTimeoutFallback     string        `json:"confirmation_timeout_fallback"`
	ConfirmationDiagInterval        time.Duration `json:"confirmation_diag_interval"`
	ConfirmationResubmitMinInterval time.Duration `json:"confirmation_resubmit_min_interval"`

	// ========== V2 新增：聚合器状态查询配置 ==========
	// QueryRetryInterval 状态查询重试间隔（默认15秒）
	QueryRetryInterval time.Duration `json:"query_retry_interval"`
	// MaxQueryAttempts 最大查询尝试次数（默认3次）
	MaxQueryAttempts uint32 `json:"max_query_attempts"`
	// QueryTotalTimeout 查询总超时时间（默认60秒）
	QueryTotalTimeout time.Duration `json:"query_total_timeout"`

	// 高度门闸配置
	MaxForkDepth uint64 `json:"max_fork_depth"` // 最大允许分叉深度

	// ========== 挖矿稳定性门闸（V2） ==========
	// MinNetworkQuorumTotal 最小网络法定人数（含本机）。
	MinNetworkQuorumTotal int `json:"min_network_quorum_total"`
	// AllowSingleNodeMining 是否允许单节点挖矿（仅 dev 环境 + from_genesis）。
	AllowSingleNodeMining bool `json:"allow_single_node_mining"`
	// NetworkDiscoveryTimeoutSeconds 网络发现超时（秒）。
	NetworkDiscoveryTimeoutSeconds int `json:"network_discovery_timeout_seconds"`
	// QuorumRecoveryTimeoutSeconds 法定人数恢复超时（秒）。
	QuorumRecoveryTimeoutSeconds int `json:"quorum_recovery_timeout_seconds"`
	// MaxHeightSkew 最大高度偏差阈值（区块数）。
	// ⚠️ 彻底简化：不区分 initial/runtime，统一使用一个阈值。
	MaxHeightSkew uint64 `json:"max_height_skew"`
	// MaxTipStalenessSeconds 链尖时效性阈值（秒）。
	MaxTipStalenessSeconds uint64 `json:"max_tip_staleness_seconds"`
	// EnableTipFreshnessCheck 是否启用链尖新鲜度检查。
	EnableTipFreshnessCheck bool `json:"enable_tip_freshness_check"`
	// EnableNetworkAlignmentCheck 是否启用网络对齐检查（V2 挖矿门闸）。
	// 默认 true，允许关闭以在生产环境逐步启用。
	EnableNetworkAlignmentCheck bool `json:"enable_network_alignment_check"`

	// ========== PoW引擎性能监控配置 ==========
	PerformanceReportInterval time.Duration `json:"performance_report_interval"` // 性能报告间隔
	MetricsUpdateInterval     time.Duration `json:"metrics_update_interval"`     // 性能指标更新间隔
	HealthCheckInterval       time.Duration `json:"health_check_interval"`       // 健康检查间隔
	EngineStopTimeout         time.Duration `json:"engine_stop_timeout"`         // 引擎停止超时时间

	// ========== 智能等待配置 ==========
	EnableSmartWait     bool          `json:"enable_smart_wait"`     // 启用智能等待机制
	BaseWaitTime        time.Duration `json:"base_wait_time"`        // 基础等待时间
	MaxWaitTime         time.Duration `json:"max_wait_time"`         // 最大等待时间
	AdaptiveWaitEnabled bool          `json:"adaptive_wait_enabled"` // 自适应等待调整

	// ========== 安全内存池配置 ==========
	EnableSafeMempool   bool          `json:"enable_safe_mempool"`   // 启用安全内存池管理
	SafetyTimeoutPeriod time.Duration `json:"safety_timeout_period"` // 安全超时时间
	AutoRollbackEnabled bool          `json:"auto_rollback_enabled"` // 自动回滚启用

	// ========== 冲突处理配置 ==========
	EnableConflictHandling bool   `json:"enable_conflict_handling"` // 启用智能冲突处理
	AutoSyncEnabled        bool   `json:"auto_sync_enabled"`        // 自动同步启用
	QualityComparisonMode  string `json:"quality_comparison_mode"`  // 质量比较模式: "comprehensive", "simple"

	// ========== 发送器策略（K桶扇出与中继相关） ==========
	NeighborFanout            int  `json:"neighbor_fanout"`             // 近邻扇出数（矿工端首跳并行或顺序尝试数）
	RelayHopLimit             int  `json:"relay_hop_limit"`             // 中继跳数上限（接收端默认处理器可中继次数）
	RequirePublicReachable    bool `json:"require_public_reachable"`    // 是否仅选择公网可达节点（预留）
	RequireAggregatorProtocol bool `json:"require_aggregator_protocol"` // 是否仅选择注册提交协议的节点（预留）

	// ========== 赞助激励配置 ==========
	SponsorIncentive SponsorIncentiveConfig `json:"sponsor_incentive"` // 赞助激励策略
}

// ==================== v2 挖矿稳定性门闸配置访问器（供 miner/quorum 使用） ====================
//
// 说明：
// - miner/quorum 作为 miner 的子组件，不应依赖 internal/config/provider；
// - 通过在 MinerConfig 上提供轻量 getter，避免引入配置包循环依赖。
func (c *MinerConfig) GetMinNetworkQuorumTotal() int {
	if c == nil {
		return 0
	}
	return c.MinNetworkQuorumTotal
}

func (c *MinerConfig) GetAllowSingleNodeMining() bool {
	if c == nil {
		return false
	}
	return c.AllowSingleNodeMining
}

func (c *MinerConfig) GetNetworkDiscoveryTimeoutSeconds() int {
	if c == nil {
		return 0
	}
	return c.NetworkDiscoveryTimeoutSeconds
}

func (c *MinerConfig) GetQuorumRecoveryTimeoutSeconds() int {
	if c == nil {
		return 0
	}
	return c.QuorumRecoveryTimeoutSeconds
}

func (c *MinerConfig) GetMaxHeightSkew() uint64 {
	if c == nil {
		return 0
	}
	return c.MaxHeightSkew
}

func (c *MinerConfig) GetMaxTipStalenessSeconds() uint64 {
	if c == nil {
		return 0
	}
	return c.MaxTipStalenessSeconds
}

func (c *MinerConfig) GetEnableTipFreshnessCheck() bool {
	if c == nil {
		return false
	}
	return c.EnableTipFreshnessCheck
}

func (c *MinerConfig) GetEnableNetworkAlignmentCheck() bool {
	if c == nil {
		return true
	}
	return c.EnableNetworkAlignmentCheck
}

// SponsorIncentiveConfig 赞助激励配置
type SponsorIncentiveConfig struct {
	Enabled             bool                `json:"enabled"`                // 是否启用赞助激励
	MaxPerBlock         int                 `json:"max_per_block"`          // 每块最多赞助笔数
	MaxAmountPerSponsor uint64              `json:"max_amount_per_sponsor"` // 单笔最大领取金额
	AcceptedTokens      []TokenFilterConfig `json:"accepted_tokens"`        // 接受的代币白名单
}

// TokenFilterConfig 代币过滤配置
type TokenFilterConfig struct {
	AssetID   string `json:"asset_id"`   // 资产ID："native"(原生币) 或合约地址
	MinAmount uint64 `json:"min_amount"` // 最低接受金额
}

// AggregatorConfig 聚合器角色专属配置
type AggregatorConfig struct {
	// 基础配置
	// EnableAggregator 控制共识运行模式
	// 🎯 **配置语义**：
	//   - true: 分布式聚合器共识模式（生产环境必须使用）
	//     * 多节点通过聚合器达成共识，提供拜占庭容错能力
	//     * 强制要求 MinPeerThreshold >= 3
	//   - false: 单节点开发模式（仅用于开发/测试，⚠️ 禁止用于生产）
	//     * 区块立即本地确认，无网络共识保障
	EnableAggregator bool `json:"enable_aggregator"`
	MaxCandidates    int  `json:"max_candidates"` // 最大候选区块数量
	MinCandidates    int  `json:"min_candidates"` // 最小候选区块数量

	// 决策权重配置（已弃用，距离选择算法不需要权重）
	// ⚠️ 以下字段在距离选择架构中已不再使用，保留仅为配置兼容性
	PowDifficultyWeight   float64 `json:"pow_difficulty_weight"`   // POW难度权重（已弃用）
	TransactionFeeWeight  float64 `json:"transaction_fee_weight"`  // 交易费用权重（已弃用）
	TimestampWeight       float64 `json:"timestamp_weight"`        // 时间戳权重（已弃用）
	MinerReputationWeight float64 `json:"miner_reputation_weight"` // 矿工声誉权重（已弃用）
	NetworkContribWeight  float64 `json:"network_contrib_weight"`  // 网络贡献权重（已弃用）
	AntiSpamWeight        float64 `json:"anti_spam_weight"`        // 反垃圾权重（已弃用）

	// 选择标准配置
	MinDifficulty       uint64        `json:"min_difficulty"`        // 最小难度要求
	MaxTimestampOffset  time.Duration `json:"max_timestamp_offset"`  // 最大时间戳偏移
	MinTransactionCount uint32        `json:"min_transaction_count"` // 最小交易数量
	MaxBlockSize        uint64        `json:"max_block_size"`        // 最大区块大小
	PreferLocalMiner    bool          `json:"prefer_local_miner"`    // 是否优先选择本地矿工
	MinPoWQuality       float64       `json:"min_pow_quality"`       // 最小PoW质量要求

	// 网络参数
	NetworkLatencyFactor     float64       `json:"network_latency_factor"`     // 网络延迟因子
	CollectionTimeout        time.Duration `json:"collection_timeout"`         // 收集超时时间
	CollectionWindowDuration time.Duration `json:"collection_window_duration"` // 候选收集窗口持续时间
	DistributionTimeout      time.Duration `json:"distribution_timeout"`       // 结果分发超时时间
	SelectionInterval        time.Duration `json:"selection_interval"`         // 选择间隔时间
	IdealPropagationDelay    time.Duration `json:"ideal_propagation_delay"`    // 理想传播延迟
	MaxPropagationDelay      time.Duration `json:"max_propagation_delay"`      // 最大传播延迟
	MinPeerThreshold         int           `json:"min_peer_threshold"`         // 最小节点阈值

	// 评分算法参数
	NetworkCacheTTL       time.Duration `json:"network_cache_ttl"`       // 网络状态缓存有效期
	NetworkDelayTolerance time.Duration `json:"network_delay_tolerance"` // 网络延迟容忍度
	DefaultNetworkDelay   time.Duration `json:"default_network_delay"`   // 默认网络延迟基准

	// ========== UTXO冲突解决配置 ==========
	EnableUTXOValidation bool          `json:"enable_utxo_validation"` // 启用UTXO冲突检测
	EnableTxValidation   bool          `json:"enable_tx_validation"`   // 启用交易验证
	EnablePowValidation  bool          `json:"enable_pow_validation"`  // 启用PoW验证
	UTXOValidationMode   string        `json:"utxo_validation_mode"`   // UTXO验证模式: "strict", "fast"
	MaxValidationTime    time.Duration `json:"max_validation_time"`    // 最大验证时间
	ConflictResolution   string        `json:"conflict_resolution"`    // 冲突解决策略: "reject", "queue"

	// ========== 调度器配置 ==========
	EnableScheduler       bool          `json:"enable_scheduler"`        // 是否启用调度器
	SchedulerTickInterval time.Duration `json:"scheduler_tick_interval"` // 调度器检查间隔
	WindowCleanupInterval time.Duration `json:"window_cleanup_interval"` // 窗口清理间隔
	MaxWindowAge          time.Duration `json:"max_window_age"`          // 最大窗口存活时间
	StatisticsInterval    time.Duration `json:"statistics_interval"`     // 统计更新间隔

	// ========== 触发条件配置 ==========
	EnableTimeoutTrigger   bool    `json:"enable_timeout_trigger"`   // 启用超时触发
	EnableThresholdTrigger bool    `json:"enable_threshold_trigger"` // 启用阈值触发
	EnableMaxTrigger       bool    `json:"enable_max_trigger"`       // 启用最大数量触发
	ThresholdRatio         float64 `json:"threshold_ratio"`          // 阈值比例 (相对于max_candidates)

	// ========== 容错配置 ==========
	MaxRetryAttempts   int           `json:"max_retry_attempts"`   // 最大重试次数
	RetryBackoffFactor float64       `json:"retry_backoff_factor"` // 重试退避因子
	SelectionTimeout   time.Duration `json:"selection_timeout"`    // 选择超时时间

	// ========== 共识算法配置 ==========
	ConsensusThreshold  float64 `json:"consensus_threshold"`   // 共识阈值（拜占庭容错阈值）
	MinConfirmationRate float64 `json:"min_confirmation_rate"` // 最小确认率（结果分发确认阈值）

	// ========== 🆕 区块转发配置（MEDIUM-001 修复） ==========
	Forward BlockForwardConfig `json:"forward"` // 区块转发配置
}

// BlockForwardConfig 区块转发配置
// 🆕 MEDIUM-001 修复：优化区块转发机制
type BlockForwardConfig struct {
	// 重试配置
	MaxRetries        int           `json:"max_retries"`         // 最大重试次数（默认3）
	RetryBackoffBase  time.Duration `json:"retry_backoff_base"`  // 重试退避基础时间（默认500ms）
	RetryBackoffMax   time.Duration `json:"retry_backoff_max"`   // 重试退避最大时间（默认10s）
	RetryBackoffFactor float64      `json:"retry_backoff_factor"` // 重试退避增长因子（默认2.0）

	// 超时配置
	CallTimeout         time.Duration `json:"call_timeout"`          // 网络调用超时（默认15s）
	EnableDynamicTimeout bool         `json:"enable_dynamic_timeout"` // 启用动态超时（默认true）
	MinTimeout          time.Duration `json:"min_timeout"`           // 最小超时时间（默认5s）
	MaxTimeout          time.Duration `json:"max_timeout"`           // 最大超时时间（默认30s）

	// 备用节点配置
	EnableBackupNodes   bool `json:"enable_backup_nodes"`    // 启用备用节点（默认true）
	BackupNodeCount     int  `json:"backup_node_count"`      // 备用节点数量（默认2）
	MaxProtocolRetries  int  `json:"max_protocol_retries"`   // 协议不兼容重选次数（默认3）

	// 健康分配置
	FailurePenalty      float64       `json:"failure_penalty"`       // 失败惩罚分（默认10）
	SuccessBonus        float64       `json:"success_bonus"`         // 成功奖励分（默认5）
	RecoveryInterval    time.Duration `json:"recovery_interval"`     // 健康分恢复间隔（默认1m）
	MinHealthScore      float64       `json:"min_health_score"`      // 最小健康分阈值（默认30）
}

// POWConfig POW算法配置
type POWConfig struct {
	// ==================== v2 难度/时间戳共识规则参数（确定性、不可依赖浮点） ====================
	// ⚠️ 重要：以下参数用于“共识有效性规则”（BlockValidator 会强校验），必须在全网一致。
	// - 所有比例参数均使用 PPM（parts-per-million，1.0 = 1_000_000）表示，禁止使用 float 参与共识计算。

	// InitialDifficulty 创世初始难度（用于高度 0 的 Difficulty）
	InitialDifficulty uint64 `json:"initial_difficulty"`

	// MinDifficulty / MaxDifficulty 难度边界（对 nextDifficulty 夹紧）
	MinDifficulty uint64 `json:"min_difficulty"`
	MaxDifficulty uint64 `json:"max_difficulty"`

	// DifficultyWindow 难度统计窗口（区块数，>=2）
	DifficultyWindow uint64 `json:"difficulty_window"`

	// MaxAdjustUpPPM / MaxAdjustDownPPM 每个窗口的最大上/下调比例
	// - MaxAdjustUpPPM: >= 1_000_000
	// - MaxAdjustDownPPM: (0, 1_000_000]
	MaxAdjustUpPPM   uint64 `json:"max_adjust_up_ppm"`
	MaxAdjustDownPPM uint64 `json:"max_adjust_down_ppm"`

	// EMAAlphaPPM 平滑系数（可选）：0 表示禁用 EMA；范围 [0, 1_000_000]
	EMAAlphaPPM uint64 `json:"ema_alpha_ppm"`

	// MTPWindow 中位时间戳窗口大小（默认 11）
	MTPWindow uint64 `json:"mtp_window"`

	// MaxFutureDriftSeconds 允许的未来时间漂移（秒），用于拒绝“未来块”
	MaxFutureDriftSeconds uint64 `json:"max_future_drift_seconds"`

	// ==================== 长间隔紧急降难（确定性、用于防停摆） ====================
	//
	// 语义参考 Bitcoin testnet：
	// - 当 parent->child 的 gap 超过阈值时，允许“下一块”更快地下调难度，避免长时间停摆；
	// - 这两个参数属于共识关键（所有节点必须一致），否则会分叉。
	//
	// EmergencyDownshiftThresholdSeconds 触发紧急降难的时间阈值（秒）。
	// - 0 表示禁用紧急降难。
	EmergencyDownshiftThresholdSeconds uint64 `json:"emergency_downshift_threshold_seconds"`
	// MaxEmergencyDownshiftBits 单块紧急降难的最大 bit 数（>=1）。
	MaxEmergencyDownshiftBits uint64 `json:"max_emergency_downshift_bits"`

	// ==================== PoW 引擎参数（非共识关键，不参与确定性计算） ====================
	WorkerCount    uint32 `json:"worker_count"`     // 挖矿线程数
	MaxNonce       uint64 `json:"max_nonce"`        // 最大Nonce范围
	EnableParallel bool   `json:"enable_parallel"`  // 是否启用并行挖矿
	HashRateWindow uint64 `json:"hash_rate_window"` // 算力统计窗口
}

// NetworkConfig 网络配置
type NetworkConfig struct {
	MaxPendingBlocks  int           `json:"max_pending_blocks"`  // 最大待处理区块数
	SyncTimeout       time.Duration `json:"sync_timeout"`        // 同步超时时间
	MaxReorgDepth     int           `json:"max_reorg_depth"`     // 最大重组深度
	MaxConnectedPeers int           `json:"max_connected_peers"` // 最大连接节点数
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`  // 心跳间隔
	MessageTimeout    time.Duration `json:"message_timeout"`     // 消息超时时间
}

// ValidationConfig 验证配置
type ValidationConfig struct {
	MaxBlockValidationTime       time.Duration `json:"max_block_validation_time"`       // 最大区块验证时间
	MaxTransactionValidationTime time.Duration `json:"max_transaction_validation_time"` // 最大交易验证时间
	EnableFullValidation         bool          `json:"enable_full_validation"`          // 是否启用完整验证
	SkipGenesisValidation        bool          `json:"skip_genesis_validation"`         // 是否跳过创世区块验证
}

// PerformanceConfig 性能配置
type PerformanceConfig struct {
	MetricsEnabled      bool          `json:"metrics_enabled"`       // 是否启用性能指标收集
	MetricsInterval     time.Duration `json:"metrics_interval"`      // 指标收集间隔
	StatisticsRetention time.Duration `json:"statistics_retention"`  // 统计数据保留时间
	MaxCandidateHistory int           `json:"max_candidate_history"` // 最大候选区块历史
	CleanupInterval     time.Duration `json:"cleanup_interval"`      // 清理间隔
	StatisticsInterval  time.Duration `json:"statistics_interval"`   // 统计间隔
}

// Config 共识配置实现
type Config struct {
	options *ConsensusOptions
}

// New 创建共识配置实现
func New(userConfig interface{}) *Config {
	defaultOptions := createDefaultConsensusOptions()

	// 如果提供了用户配置，尝试解析并合并
	if userConfig != nil {
		if configMap, ok := userConfig.(map[string]interface{}); ok {
			// ==================== 顶层共识关键参数 ====================
			// Provider 会把 chainConfig.mining.target_block_time 映射为这里的 "target_block_time"。
			// 这是“统计目标”，会被难度策略/slot 等模块使用，必须确保解析生效。
			var userSetTargetBlockTime bool
			var userSetEmergencyThreshold bool
			var userSetEmergencyMaxBits bool

			if v, exists := configMap["target_block_time"]; exists {
				switch vv := v.(type) {
				case string:
					if d, err := time.ParseDuration(strings.TrimSpace(vv)); err == nil && d > 0 {
						defaultOptions.TargetBlockTime = d
						userSetTargetBlockTime = true
					}
				case float64:
					// 兼容：若上游传的是秒数（JSON number），按 seconds 解析
					if vv > 0 {
						defaultOptions.TargetBlockTime = time.Duration(vv * float64(time.Second))
						userSetTargetBlockTime = true
					}
				case int:
					if vv > 0 {
						defaultOptions.TargetBlockTime = time.Duration(vv) * time.Second
						userSetTargetBlockTime = true
					}
				}
			}

			// 处理聚合器配置
			if aggregatorMap, exists := configMap["aggregator"]; exists {
				if aggregatorConfig, ok := aggregatorMap.(map[string]interface{}); ok {
					// 处理enable_aggregator
					if enableAggregator, exists := aggregatorConfig["enable_aggregator"]; exists {
						if enableBool, ok := enableAggregator.(bool); ok {
							defaultOptions.Aggregator.EnableAggregator = enableBool
						}
					}
					// 处理其他聚合器配置...
					if maxCandidates, exists := aggregatorConfig["max_candidates"]; exists {
						if maxFloat, ok := maxCandidates.(float64); ok {
							defaultOptions.Aggregator.MaxCandidates = int(maxFloat)
						}
					}
					if minCandidates, exists := aggregatorConfig["min_candidates"]; exists {
						if minFloat, ok := minCandidates.(float64); ok {
							defaultOptions.Aggregator.MinCandidates = int(minFloat)
						}
					}
					if collectionTimeout, exists := aggregatorConfig["collection_timeout"]; exists {
						if timeoutStr, ok := collectionTimeout.(string); ok {
							if duration, err := time.ParseDuration(timeoutStr); err == nil {
								defaultOptions.Aggregator.CollectionTimeout = duration
							}
						}
					}
					if selectionInterval, exists := aggregatorConfig["selection_interval"]; exists {
						if intervalStr, ok := selectionInterval.(string); ok {
							if duration, err := time.ParseDuration(intervalStr); err == nil {
								defaultOptions.Aggregator.SelectionInterval = duration
							}
						}
					}
				}
			}

			// 处理Miner配置（v2 确认门闸退路）
			if minerMap, exists := configMap["miner"]; exists {
				if minerCfg, ok := minerMap.(map[string]interface{}); ok {
					// mining_timeout：总体挖矿轮次超时（roundCtx）
					if v, exists := minerCfg["mining_timeout"]; exists {
						switch vv := v.(type) {
						case string:
							if d, err := time.ParseDuration(strings.TrimSpace(vv)); err == nil && d > 0 {
								defaultOptions.Miner.MiningTimeout = d
							}
						case float64:
							// 兼容：若上游传的是秒数（JSON number），按 seconds 解析
							if vv > 0 {
								defaultOptions.Miner.MiningTimeout = time.Duration(vv * float64(time.Second))
							}
						case int:
							if vv > 0 {
								defaultOptions.Miner.MiningTimeout = time.Duration(vv) * time.Second
							}
						}
					}

					// pow_slice：单次PoW尝试窗口（attemptCtx）
					if v, exists := minerCfg["pow_slice"]; exists {
						switch vv := v.(type) {
						case string:
							if d, err := time.ParseDuration(strings.TrimSpace(vv)); err == nil && d > 0 {
								defaultOptions.Miner.PoWSlice = d
							}
						case float64:
							if vv > 0 {
								defaultOptions.Miner.PoWSlice = time.Duration(vv * float64(time.Second))
							}
						case int:
							if vv > 0 {
								defaultOptions.Miner.PoWSlice = time.Duration(vv) * time.Second
							}
						}
					}

					// ========== v2：挖矿稳定性门闸配置 ==========
					if v, exists := minerCfg["min_network_quorum_total"]; exists {
						switch vv := v.(type) {
						case float64:
							defaultOptions.Miner.MinNetworkQuorumTotal = int(vv)
						case int:
							defaultOptions.Miner.MinNetworkQuorumTotal = vv
						}
					}
					if v, exists := minerCfg["allow_single_node_mining"]; exists {
						if b, ok := v.(bool); ok {
							defaultOptions.Miner.AllowSingleNodeMining = b
						}
					}
					if v, exists := minerCfg["network_discovery_timeout_seconds"]; exists {
						switch vv := v.(type) {
						case float64:
							defaultOptions.Miner.NetworkDiscoveryTimeoutSeconds = int(vv)
						case int:
							defaultOptions.Miner.NetworkDiscoveryTimeoutSeconds = vv
						}
					}
					if v, exists := minerCfg["quorum_recovery_timeout_seconds"]; exists {
						switch vv := v.(type) {
						case float64:
							defaultOptions.Miner.QuorumRecoveryTimeoutSeconds = int(vv)
						case int:
							defaultOptions.Miner.QuorumRecoveryTimeoutSeconds = vv
						}
					}
					if v, exists := minerCfg["max_height_skew"]; exists {
						switch vv := v.(type) {
						case float64:
							defaultOptions.Miner.MaxHeightSkew = uint64(vv)
						case int:
							if vv >= 0 {
								defaultOptions.Miner.MaxHeightSkew = uint64(vv)
							}
						case uint64:
							defaultOptions.Miner.MaxHeightSkew = vv
						}
					}
					if v, exists := minerCfg["max_tip_staleness_seconds"]; exists {
						switch vv := v.(type) {
						case float64:
							defaultOptions.Miner.MaxTipStalenessSeconds = uint64(vv)
						case int:
							if vv >= 0 {
								defaultOptions.Miner.MaxTipStalenessSeconds = uint64(vv)
							}
						case uint64:
							defaultOptions.Miner.MaxTipStalenessSeconds = vv
						}
					}
					if v, exists := minerCfg["enable_tip_freshness_check"]; exists {
						if b, ok := v.(bool); ok {
							defaultOptions.Miner.EnableTipFreshnessCheck = b
						}
					}
					if v, exists := minerCfg["enable_network_alignment_check"]; exists {
						if b, ok := v.(bool); ok {
							defaultOptions.Miner.EnableNetworkAlignmentCheck = b
						}
					}

					if v, exists := minerCfg["confirmation_timeout_fallback"]; exists {
						if s, ok := v.(string); ok {
							defaultOptions.Miner.ConfirmationTimeoutFallback = strings.TrimSpace(s)
						}
					}
					if v, exists := minerCfg["confirmation_diag_interval"]; exists {
						if s, ok := v.(string); ok {
							if d, err := time.ParseDuration(s); err == nil {
								defaultOptions.Miner.ConfirmationDiagInterval = d
							}
						}
					}
					if v, exists := minerCfg["confirmation_resubmit_min_interval"]; exists {
						if s, ok := v.(string); ok {
							if d, err := time.ParseDuration(s); err == nil {
								defaultOptions.Miner.ConfirmationResubmitMinInterval = d
							}
						}
					}
				}
			}

			// 处理POW配置
			if powMap, exists := configMap["pow"]; exists {
				if powConfig, ok := powMap.(map[string]interface{}); ok {
					// 处理初始难度
					if initialDifficulty, exists := powConfig["initial_difficulty"]; exists {
						if difficultyFloat, ok := initialDifficulty.(float64); ok {
							defaultOptions.POW.InitialDifficulty = uint64(difficultyFloat)
						}
					}

					// v2：解析确定性难度/时间戳参数（PPM/整数）
					if v, exists := powConfig["min_difficulty"]; exists {
						if f, ok := v.(float64); ok {
							defaultOptions.POW.MinDifficulty = uint64(f)
						}
					}
					if v, exists := powConfig["max_difficulty"]; exists {
						if f, ok := v.(float64); ok {
							defaultOptions.POW.MaxDifficulty = uint64(f)
						}
					}
					if v, exists := powConfig["difficulty_window"]; exists {
						if f, ok := v.(float64); ok {
							defaultOptions.POW.DifficultyWindow = uint64(f)
						}
					}
					if v, exists := powConfig["max_adjust_up_ppm"]; exists {
						if f, ok := v.(float64); ok {
							defaultOptions.POW.MaxAdjustUpPPM = uint64(f)
						}
					}
					if v, exists := powConfig["max_adjust_down_ppm"]; exists {
						if f, ok := v.(float64); ok {
							defaultOptions.POW.MaxAdjustDownPPM = uint64(f)
						}
					}
					if v, exists := powConfig["ema_alpha_ppm"]; exists {
						if f, ok := v.(float64); ok {
							defaultOptions.POW.EMAAlphaPPM = uint64(f)
						}
					}
					if v, exists := powConfig["mtp_window"]; exists {
						if f, ok := v.(float64); ok {
							defaultOptions.POW.MTPWindow = uint64(f)
						}
					}
					if v, exists := powConfig["max_future_drift_seconds"]; exists {
						if f, ok := v.(float64); ok {
							defaultOptions.POW.MaxFutureDriftSeconds = uint64(f)
						}
					}

					// v2：紧急降难参数（共识关键，必须可被 JSON 注入）
					if v, exists := powConfig["emergency_downshift_threshold_seconds"]; exists {
						switch vv := v.(type) {
						case float64:
							defaultOptions.POW.EmergencyDownshiftThresholdSeconds = uint64(vv)
							userSetEmergencyThreshold = true
						case int:
							if vv >= 0 {
								defaultOptions.POW.EmergencyDownshiftThresholdSeconds = uint64(vv)
								userSetEmergencyThreshold = true
							}
						}
					}
					if v, exists := powConfig["max_emergency_downshift_bits"]; exists {
						switch vv := v.(type) {
						case float64:
							defaultOptions.POW.MaxEmergencyDownshiftBits = uint64(vv)
							userSetEmergencyMaxBits = true
						case int:
							if vv >= 0 {
								defaultOptions.POW.MaxEmergencyDownshiftBits = uint64(vv)
								userSetEmergencyMaxBits = true
							}
						}
					}
				}
			}

			// 如果用户显式设置了 target_block_time，但没有显式覆盖 emergency 参数，
			// 则让默认 emergency 阈值随目标时间联动（保持“10 * target”的语义一致）。
			if userSetTargetBlockTime && !userSetEmergencyThreshold {
				if defaultOptions.TargetBlockTime > 0 {
					defaultOptions.POW.EmergencyDownshiftThresholdSeconds = uint64((defaultOptions.TargetBlockTime * 10) / time.Second)
				}
			}
			// MaxEmergencyDownshiftBits：若用户未配置，则保持默认（通常为 8）
			_ = userSetEmergencyMaxBits
		}
	}

	return &Config{
		options: defaultOptions,
	}
}

// createDefaultConsensusOptions 创建默认共识配置
func createDefaultConsensusOptions() *ConsensusOptions {
	return &ConsensusOptions{
		ConsensusType:   defaultConsensusType,
		TargetBlockTime: defaultTargetBlockTime,
		BlockSizeLimit:  defaultBlockSizeLimit,

		// 矿工角色配置
		Miner: MinerConfig{
			MiningTimeout:                   defaultMiningTimeout,
			PoWSlice:                        0,
			LoopInterval:                    defaultLoopInterval,
			MaxTransactions:                 defaultMaxTransactions,
			MinTransactions:                 defaultMinTransactions,
			TxSelectionMode:                 defaultTxSelectionMode,
			MaxCPUUsage:                     defaultMaxCPUUsage,
			MaxMemoryUsage:                  defaultMaxMemoryUsage,
			MaxGoroutines:                   defaultMaxGoroutines,
			SendRetryCount:                  defaultSendRetryCount,
			SendTimeout:                     defaultSendTimeout,
			DecisionNodes:                   defaultDecisionNodes,
			MaxCandidatesBuffer:             defaultMaxCandidatesBuffer,
			ConfirmationTimeout:             defaultConfirmationTimeout,
			ConfirmationCheckInterval:       defaultConfirmationCheckInterval,
			ConfirmationTimeoutFallback:     "sync",
			ConfirmationDiagInterval:        5 * time.Second,
			ConfirmationResubmitMinInterval: 2 * time.Second,
			QueryRetryInterval:              defaultQueryRetryInterval,
			MaxQueryAttempts:                defaultMaxQueryAttempts,
			QueryTotalTimeout:               defaultQueryTotalTimeout,
			PerformanceReportInterval:       defaultPerformanceReportInterval,
			MetricsUpdateInterval:           defaultMetricsUpdateInterval,
			HealthCheckInterval:             defaultHealthCheckInterval,
			EngineStopTimeout:               defaultEngineStopTimeout,
			NeighborFanout:                  defaultNeighborFanout,
			RelayHopLimit:                   defaultRelayHopLimit,
			MaxForkDepth:                    defaultMaxForkDepth,
			// ========== 挖矿稳定性门闸（V2）默认值 ==========
			MinNetworkQuorumTotal:          defaultMinNetworkQuorumTotal, // Provider 会按环境/阈值覆盖
			AllowSingleNodeMining:          defaultAllowSingleNodeMining,
			NetworkDiscoveryTimeoutSeconds: defaultNetworkDiscoveryTimeoutSecs,
			QuorumRecoveryTimeoutSeconds:   defaultQuorumRecoveryTimeoutSecs,
			MaxHeightSkew:                  defaultMaxHeightSkew,
			MaxTipStalenessSeconds:         defaultMaxTipStalenessSeconds,
			EnableTipFreshnessCheck:        defaultEnableTipFreshnessCheck,
			EnableNetworkAlignmentCheck:    defaultEnableNetworkAlignmentCheck,
			// 智能等待配置
			EnableSmartWait:     defaultEnableSmartWait,
			BaseWaitTime:        defaultBaseWaitTime,
			MaxWaitTime:         defaultMaxWaitTime,
			AdaptiveWaitEnabled: defaultAdaptiveWaitEnabled,
			// 安全内存池配置
			EnableSafeMempool:   defaultEnableSafeMempool,
			SafetyTimeoutPeriod: defaultSafetyTimeoutPeriod,
			AutoRollbackEnabled: defaultAutoRollbackEnabled,
			// 冲突处理配置
			EnableConflictHandling: defaultEnableConflictHandling,
			AutoSyncEnabled:        defaultAutoSyncEnabled,
			QualityComparisonMode:  defaultQualityComparisonMode,
			SponsorIncentive: SponsorIncentiveConfig{
				Enabled:             defaultSponsorEnabled,
				MaxPerBlock:         defaultMaxSponsorPerBlock,
				MaxAmountPerSponsor: defaultMaxAmountPerSponsor,
				AcceptedTokens: []TokenFilterConfig{
					{AssetID: "native", MinAmount: 10},
				},
			},
		},

		// 聚合器角色配置
		Aggregator: AggregatorConfig{
			EnableAggregator:         defaultEnableAggregator,
			MaxCandidates:            defaultMaxCandidates,
			MinCandidates:            defaultMinCandidates,
			PowDifficultyWeight:      defaultPowDifficultyWeight,
			TransactionFeeWeight:     defaultTransactionFeeWeight,
			TimestampWeight:          defaultTimestampWeight,
			MinerReputationWeight:    defaultMinerReputationWeight,
			NetworkContribWeight:     defaultNetworkContribWeight,
			AntiSpamWeight:           defaultAntiSpamWeight,
			MinDifficulty:            defaultAggregatorMinDifficulty,
			MaxTimestampOffset:       defaultMaxTimestampOffset,
			MinTransactionCount:      defaultMinTransactionCount,
			MaxBlockSize:             defaultAggregatorMaxBlockSize,
			PreferLocalMiner:         defaultPreferLocalMiner,
			MinPoWQuality:            defaultMinPoWQuality,
			NetworkLatencyFactor:     defaultNetworkLatencyFactor,
			CollectionTimeout:        defaultCollectionTimeout,
			CollectionWindowDuration: defaultCollectionWindowDuration,
			DistributionTimeout:      defaultDistributionTimeoutAggregator,
			SelectionInterval:        defaultSelectionInterval,
			IdealPropagationDelay:    defaultIdealPropagationDelay,
			MaxPropagationDelay:      defaultMaxPropagationDelay,
			MinPeerThreshold:         defaultMinPeerThreshold,

			// 调度器配置
			EnableScheduler:       defaultEnableScheduler,
			SchedulerTickInterval: defaultSchedulerTickInterval,
			WindowCleanupInterval: defaultWindowCleanupInterval,
			MaxWindowAge:          defaultMaxWindowAge,
			StatisticsInterval:    defaultStatisticsIntervalGeneral,

			// 触发条件配置
			EnableTimeoutTrigger:   defaultEnableTimeoutTrigger,
			EnableThresholdTrigger: defaultEnableThresholdTrigger,
			EnableMaxTrigger:       defaultEnableMaxTrigger,
			ThresholdRatio:         defaultThresholdRatio,

			// 容错配置
			MaxRetryAttempts:   defaultMaxRetryAttempts,
			RetryBackoffFactor: defaultRetryBackoffFactor,
			SelectionTimeout:   defaultSelectionTimeout,

			// 共识算法配置
			ConsensusThreshold:  defaultConsensusThreshold,
			MinConfirmationRate: defaultMinConfirmationRate,
			// UTXO冲突解决配置
			EnableUTXOValidation: defaultEnableUTXOValidation,
			EnableTxValidation:   defaultEnableTxValidation,
			EnablePowValidation:  defaultEnablePowValidation,
			UTXOValidationMode:   defaultUTXOValidationMode,
			MaxValidationTime:    defaultMaxValidationTime,
			ConflictResolution:   defaultConflictResolution,

			// 🆕 区块转发配置（MEDIUM-001 修复）
			Forward: BlockForwardConfig{
				MaxRetries:           3,
				RetryBackoffBase:     500 * time.Millisecond,
				RetryBackoffMax:      10 * time.Second,
				RetryBackoffFactor:   2.0,
				CallTimeout:          15 * time.Second,
				EnableDynamicTimeout: true,
				MinTimeout:           5 * time.Second,
				MaxTimeout:           30 * time.Second,
				EnableBackupNodes:    true,
				BackupNodeCount:      2,
				MaxProtocolRetries:   3,
				FailurePenalty:       10,
				SuccessBonus:         5,
				RecoveryInterval:     time.Minute,
				MinHealthScore:       30,
			},
		},

		// POW配置
		POW: POWConfig{
			InitialDifficulty: defaultInitialDifficulty,
			MinDifficulty:     defaultMinDifficulty,
			MaxDifficulty:     defaultMaxDifficulty,
			DifficultyWindow:  defaultDifficultyWindow,

			MaxAdjustUpPPM:                     defaultMaxAdjustUpPPM,
			MaxAdjustDownPPM:                   defaultMaxAdjustDownPPM,
			EMAAlphaPPM:                        defaultEMAAlphaPPM,
			MTPWindow:                          defaultMTPWindow,
			MaxFutureDriftSeconds:              defaultMaxFutureDriftSeconds,
			EmergencyDownshiftThresholdSeconds: defaultEmergencyDownshiftThresholdSeconds,
			MaxEmergencyDownshiftBits:          defaultMaxEmergencyDownshiftBits,

			WorkerCount:    defaultWorkerCount,
			MaxNonce:       defaultMaxNonce,
			EnableParallel: defaultEnableParallel,
			HashRateWindow: defaultHashRateWindow,
		},

		// 网络配置
		Network: NetworkConfig{
			MaxPendingBlocks:  defaultMaxPendingBlocks,
			SyncTimeout:       defaultSyncTimeout,
			MaxReorgDepth:     defaultMaxReorgDepth,
			MaxConnectedPeers: defaultMaxConnectedPeers,
			HeartbeatInterval: defaultHeartbeatInterval,
			MessageTimeout:    defaultMessageTimeout,
		},

		// 验证配置
		Validation: ValidationConfig{
			MaxBlockValidationTime:       defaultMaxBlockValidationTime,
			MaxTransactionValidationTime: defaultMaxTransactionValidationTime,
			EnableFullValidation:         defaultEnableFullValidation,
			SkipGenesisValidation:        defaultSkipGenesisValidation,
		},

		// 性能配置
		Performance: PerformanceConfig{
			MetricsEnabled:      defaultMetricsEnabled,
			MetricsInterval:     defaultMetricsInterval,
			StatisticsRetention: defaultStatisticsRetention,
			MaxCandidateHistory: defaultMaxCandidateHistory,
			CleanupInterval:     defaultCleanupInterval,
			StatisticsInterval:  defaultStatisticsInterval,
		},

		// 内部配置
		ConsensusTypes:        append([]string{}, defaultConsensusTypes...),
		ValidationLevels:      copyBoolMap(defaultValidationLevels),
		PerformanceThresholds: copyInterfaceMap(defaultPerformanceThresholds),
	}
}

func copyBoolMap(src map[string]bool) map[string]bool {
	dst := make(map[string]bool, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func copyInterfaceMap(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// GetOptions 获取完整的共识配置选项
func (c *Config) GetOptions() *ConsensusOptions {
	return c.options
}

// ==================== 配置验证方法 ====================

// ValidateForEnvironment 验证共识配置是否符合指定环境和链模式的要求
//
// 🎯 **环境 + 链模式感知配置验证**：
//   - 生产环境 + 公链 / 联盟链 (env=prod, mode in {public, consortium}):
//   - 强制启用分布式聚合器共识，禁止单节点模式
//   - 要求 min_peer_threshold >= 3
//   - 生产环境 + 私链 (env=prod, mode=private):
//   - 允许单节点模式，但强烈不建议用于高价值场景
//   - 开发 / 测试环境 (env in {dev, test}):
//   - 允许单节点模式
//
// @param environment 运行环境："dev" | "test" | "prod"
// @param chainMode   链模式："public" | "consortium" | "private"
// @return error 验证失败时返回错误信息
func (c *Config) ValidateForEnvironment(environment, chainMode string) error {
	env := strings.ToLower(environment)
	mode := strings.ToLower(chainMode)

	// 只有在生产环境且为公链 / 联盟链时，才强制要求分布式聚合器共识
	if env == "prod" && (mode == "public" || mode == "consortium") {
		if !c.options.Aggregator.EnableAggregator {
			return fmt.Errorf("❌ 生产环境配置错误: enable_aggregator 必须为 true\n" +
				"   原因: 生产环境的公链/联盟链必须使用分布式聚合器共识模式，禁止单节点模式\n" +
				"   风险: 单节点共识可能导致网络分叉和数据不一致\n" +
				"   解决: 请在配置文件中设置 mining.enable_aggregator = true")
		}

		if c.options.Aggregator.MinPeerThreshold < 3 {
			return fmt.Errorf("❌ 生产环境配置错误: min_peer_threshold 必须 >= 3 (当前值: %d)\n"+
				"   原因: 拜占庭容错共识至少需要3个节点\n"+
				"   解决: 请在配置文件中设置 consensus.aggregator.min_peer_threshold >= 3",
				c.options.Aggregator.MinPeerThreshold)
		}
	}

	return nil
}

// GetConsensusType 获取共识类型
func (c *Config) GetConsensusType() string {
	return c.options.ConsensusType
}

// GetTargetBlockTime 获取目标出块时间
func (c *Config) GetTargetBlockTime() time.Duration {
	return c.options.TargetBlockTime
}

// GetInitialDifficulty 获取初始难度
func (c *Config) GetInitialDifficulty() uint64 {
	return c.options.POW.InitialDifficulty
}

// GetMinDifficulty 获取最小难度
func (c *Config) GetMinDifficulty() uint64 {
	return c.options.POW.MinDifficulty
}

// GetMaxDifficulty 获取最大难度
func (c *Config) GetMaxDifficulty() uint64 {
	return c.options.POW.MaxDifficulty
}

// GetWorkerCount 获取挖矿线程数
func (c *Config) GetWorkerCount() uint32 {
	return c.options.POW.WorkerCount
}

// IsParallelEnabled 是否启用并行挖矿
func (c *Config) IsParallelEnabled() bool {
	return c.options.POW.EnableParallel
}

// IsFullValidationEnabled 是否启用完整验证
func (c *Config) IsFullValidationEnabled() bool {
	return c.options.Validation.EnableFullValidation
}

// IsMetricsEnabled 是否启用性能指标收集
func (c *Config) IsMetricsEnabled() bool {
	return c.options.Performance.MetricsEnabled
}
