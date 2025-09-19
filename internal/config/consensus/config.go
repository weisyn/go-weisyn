package consensus

import "time"

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

	// 奖励和激励配置
	Reward RewardConfig `json:"reward"`

	// 内部配置
	ConsensusTypes        []string               `json:"-"`
	ValidationLevels      map[string]bool        `json:"-"`
	PerformanceThresholds map[string]interface{} `json:"-"`
}

// MinerConfig 矿工角色专属配置
type MinerConfig struct {
	// 挖矿控制参数
	MiningTimeout   time.Duration `json:"mining_timeout"`    // 挖矿超时时间
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

	// 高度门闸配置
	MaxForkDepth uint64 `json:"max_fork_depth"` // 最大允许分叉深度

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
}

// AggregatorConfig 聚合器角色专属配置
type AggregatorConfig struct {
	// 基础配置
	EnableAggregator bool `json:"enable_aggregator"` // 是否启用聚合节点功能
	MaxCandidates    int  `json:"max_candidates"`    // 最大候选区块数量
	MinCandidates    int  `json:"min_candidates"`    // 最小候选区块数量

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
}

// POWConfig POW算法配置
type POWConfig struct {
	InitialDifficulty          uint64  `json:"initial_difficulty"`           // 初始难度
	MinDifficulty              uint64  `json:"min_difficulty"`               // 最小难度
	MaxDifficulty              uint64  `json:"max_difficulty"`               // 最大难度
	DifficultyWindow           uint64  `json:"difficulty_window"`            // 难度调整窗口
	DifficultyAdjustmentFactor float64 `json:"difficulty_adjustment_factor"` // 难度调整因子
	WorkerCount                uint32  `json:"worker_count"`                 // 挖矿线程数
	MaxNonce                   uint64  `json:"max_nonce"`                    // 最大Nonce范围
	EnableParallel             bool    `json:"enable_parallel"`              // 是否启用并行挖矿
	HashRateWindow             uint64  `json:"hash_rate_window"`             // 算力统计窗口
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

// RewardConfig 奖励配置
type RewardConfig struct {
	BlockReward         uint64  `json:"block_reward"`          // 区块奖励
	TransactionFeeRatio float64 `json:"transaction_fee_ratio"` // 交易费用分配比例
	HalvingInterval     uint64  `json:"halving_interval"`      // 奖励减半间隔
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

			// 处理POW配置
			if powMap, exists := configMap["pow"]; exists {
				if powConfig, ok := powMap.(map[string]interface{}); ok {
					// 处理初始难度
					if initialDifficulty, exists := powConfig["initial_difficulty"]; exists {
						if difficultyFloat, ok := initialDifficulty.(float64); ok {
							defaultOptions.POW.InitialDifficulty = uint64(difficultyFloat)
						}
					}
				}
			}
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
			MiningTimeout:             defaultMiningTimeout,
			LoopInterval:              defaultLoopInterval,
			MaxTransactions:           defaultMaxTransactions,
			MinTransactions:           defaultMinTransactions,
			TxSelectionMode:           defaultTxSelectionMode,
			MaxCPUUsage:               defaultMaxCPUUsage,
			MaxMemoryUsage:            defaultMaxMemoryUsage,
			MaxGoroutines:             defaultMaxGoroutines,
			SendRetryCount:            defaultSendRetryCount,
			SendTimeout:               defaultSendTimeout,
			DecisionNodes:             defaultDecisionNodes,
			MaxCandidatesBuffer:       defaultMaxCandidatesBuffer,
			ConfirmationTimeout:       defaultConfirmationTimeout,
			ConfirmationCheckInterval: defaultConfirmationCheckInterval,
			PerformanceReportInterval: defaultPerformanceReportInterval,
			MetricsUpdateInterval:     defaultMetricsUpdateInterval,
			HealthCheckInterval:       defaultHealthCheckInterval,
			EngineStopTimeout:         defaultEngineStopTimeout,
			NeighborFanout:            defaultNeighborFanout,
			RelayHopLimit:             defaultRelayHopLimit,
			MaxForkDepth:              defaultMaxForkDepth,
		},

		// 聚合器角色配置
		Aggregator: AggregatorConfig{
			EnableAggregator:      defaultEnableAggregator,
			MaxCandidates:         defaultMaxCandidates,
			MinCandidates:         defaultMinCandidates,
			PowDifficultyWeight:   defaultPowDifficultyWeight,
			TransactionFeeWeight:  defaultTransactionFeeWeight,
			TimestampWeight:       defaultTimestampWeight,
			MinerReputationWeight: defaultMinerReputationWeight,
			NetworkContribWeight:  defaultNetworkContribWeight,
			AntiSpamWeight:        defaultAntiSpamWeight,
			MinDifficulty:         defaultAggregatorMinDifficulty,
			MaxTimestampOffset:    defaultMaxTimestampOffset,
			MinTransactionCount:   defaultMinTransactionCount,
			MaxBlockSize:          defaultAggregatorMaxBlockSize,
			PreferLocalMiner:      defaultPreferLocalMiner,
			MinPoWQuality:         defaultMinPoWQuality,
			NetworkLatencyFactor:  defaultNetworkLatencyFactor,
			CollectionTimeout:     defaultCollectionTimeout,
			SelectionInterval:     defaultSelectionInterval,
			IdealPropagationDelay: defaultIdealPropagationDelay,
			MaxPropagationDelay:   defaultMaxPropagationDelay,
			MinPeerThreshold:      defaultMinPeerThreshold,

			// 调度器配置
			EnableScheduler:       defaultEnableScheduler,
			SchedulerTickInterval: defaultSchedulerTickInterval,
			WindowCleanupInterval: defaultWindowCleanupInterval,
			MaxWindowAge:          defaultMaxWindowAge,
			StatisticsInterval:    defaultStatisticsInterval,

			// 触发条件配置
			EnableTimeoutTrigger:   defaultEnableTimeoutTrigger,
			EnableThresholdTrigger: defaultEnableThresholdTrigger,
			EnableMaxTrigger:       defaultEnableMaxTrigger,
			ThresholdRatio:         defaultThresholdRatio,

			// 容错配置
			MaxRetryAttempts:    defaultMaxRetryAttempts,
			RetryBackoffFactor:  defaultRetryBackoffFactor,
			SelectionTimeout:    defaultSelectionTimeout,
			DistributionTimeout: defaultDistributionTimeout,

			// 共识算法配置
			ConsensusThreshold:  defaultConsensusThreshold,
			MinConfirmationRate: defaultMinConfirmationRate,
		},

		// POW配置
		POW: POWConfig{
			InitialDifficulty:          defaultInitialDifficulty,
			MinDifficulty:              defaultMinDifficulty,
			MaxDifficulty:              defaultMaxDifficulty,
			DifficultyWindow:           defaultDifficultyWindow,
			DifficultyAdjustmentFactor: defaultDifficultyAdjustmentFactor,
			WorkerCount:                defaultWorkerCount,
			MaxNonce:                   defaultMaxNonce,
			EnableParallel:             defaultEnableParallel,
			HashRateWindow:             defaultHashRateWindow,
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

		// 奖励配置
		Reward: RewardConfig{
			BlockReward:         defaultBlockReward,
			TransactionFeeRatio: defaultTransactionFeeRatio,
			HalvingInterval:     defaultHalvingInterval,
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

// GetBlockReward 获取区块奖励
func (c *Config) GetBlockReward() uint64 {
	return c.options.Reward.BlockReward
}

// IsMetricsEnabled 是否启用性能指标收集
func (c *Config) IsMetricsEnabled() bool {
	return c.options.Performance.MetricsEnabled
}
