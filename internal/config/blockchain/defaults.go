package blockchain

import (
	"time"

	"github.com/weisyn/v1/pkg/types"
)

// 区块链配置默认值
// 按域组织的分层默认值，只包含实际使用的核心配置
const (
	// === 基础链配置 ===
	defaultChainID   = 2 // 测试网链ID
	defaultNetworkID = 2 // 网络ID与链ID保持一致

	// === 区块域默认值 ===
	defaultMaxBlockSize      = 2 * 1024 * 1024  // 2MB最大区块大小
	defaultMaxTransactions   = 1000             // 最大交易数
	defaultBlockTimeTarget   = 10               // 目标出块时间(秒)
	defaultMinBlockInterval  = 10               // 最小区块间隔(秒)
	defaultMinDifficulty     = 1                // 最小难度
	defaultMaxTimeDrift      = 300              // 最大时间偏差(秒)
	defaultValidationTimeout = 30 * time.Second // 验证超时
	defaultBlockCacheSize    = 1000             // 区块缓存数量

	// === 交易域默认值 ===
	defaultMaxTransactionSize    = 64 * 1024   // 64KB最大交易大小
	defaultBaseFeePerByte        = 10          // 基础字节费率
	defaultMinimumFee            = 1000        // 最低费用
	defaultMaximumFee            = 1000000     // 最高费用
	defaultBaseExecutionFeePrice = 20000000000 // 20 Gwei基础执行费用价格
	defaultTransactionCacheSize  = 10000       // 交易缓存数量
	defaultCongestionMultiplier  = 1.5         // 拥堵系数
	defaultMaxBatchTransferSize  = 100         // 批量转账最大笔数

	// === 费用相关默认值（避免过度配置）===
	defaultDustThreshold = 0.00001 // 粉尘阈值（0.00001个原生币，用于UTXO选择算法）
	defaultBaseFeeRate   = 0.0003  // 基础费率参考（万三 = 0.0003，仅作计算参考）

	// === 同步域默认值 ===
	defaultSyncBatchSize     = 100              // 同步批次大小
	defaultSyncConcurrency   = 4                // 同步并发度
	defaultSyncTimeout       = 60 * time.Second // 同步超时
	defaultSyncMinPeerCount  = 2                // 最小节点数
	defaultSyncMaxPeerCount  = 10               // 最大节点数
	defaultSyncRetryAttempts = 3                // 重试次数
	defaultMaxReorgDepth     = 6                // 最大重组深度
	defaultAutoReorgMaxDepth = 1000             // sync 模块自动 reorg 最大深度（fork-aware）

	// === K桶智能同步默认值 ===
	// K桶节点选择配置
	defaultKBucketSelectionCount    = 5               // K桶节点选择数量
	defaultKBucketSelectionStrategy = "mixed"         // K桶节点选择策略 ("distance", "random", "mixed")
	defaultNodeSelectionTimeout     = 3 * time.Second // 节点选择超时
	defaultMaxConcurrentRequests    = 3               // 最大并发请求数

	// 智能分页配置
	defaultMaxResponseSizeBytes       = uint32(5 * 1024 * 1024) // 5MB网络响应大小限制
	defaultMaxBlocksPerRequest        = 100                     // 每次请求最大区块数
	defaultIntelligentPagingThreshold = uint32(2 * 1024 * 1024) // 2MB智能分页阈值
	defaultMinBlocksGuarantee         = 1                       // 最小区块保证数量

	// 时间检查配置
	defaultTimeCheckEnabled       = true             // 启用时间检查触发
	defaultTimeCheckThresholdMins = 10               // 时间检查阈值分钟数
	defaultTimeCheckIntervalMins  = 5                // 时间检查间隔分钟数
	defaultSyncTriggerTimeout     = 30 * time.Second // 同步触发超时

	// 节点同步状态缓存配置
	defaultPeerSyncCacheExpiryMins = 5 // 节点同步状态缓存过期时间（分钟）

	// 上游节点记忆（抗抖动）配置
	defaultUpstreamMemoryTTLSeconds       = 600 // 10分钟
	defaultUpstreamMaxConsecutiveFailures = 3   // 连续失败3次则清除上游记忆，快速切换

	// K桶入桶保障配置（防空桶风险）
	defaultKBucketReconcileIntervalSeconds = 30 // 30秒一次reconcile

	// 存储/索引自愈（persistence 内部子能力）配置
	defaultRepairEnabled         = true // 启用在线自愈
	defaultRepairMaxConcurrency  = 2    // 并发2
	// 🔧 默认去抖从 60s 降到 10s：
	// - 避免升级/迁移导致 indices:hash 缺失时，节点在同步/验证热路径被长时间卡住
	// - 仍保留去抖保护，防止极端损坏场景下反复触发重扫描造成 I/O 风暴
	defaultRepairThrottleSeconds = 10   // 10秒去抖
	defaultRepairHashIndexWindow = 5000 // 扫描窗口5000

	// 节点熔断（Circuit Breaker）配置
	defaultCircuitBreakerFailureThreshold = 3   // 连续失败3次触发熔断
	defaultCircuitBreakerRecoverySeconds  = 300 // 熔断5分钟后恢复

	// 事件去抖与限流配置
	defaultPeerEventDebounceMs        = 1000 // 同一节点连接事件去抖时间（毫秒）
	defaultGlobalMinTriggerIntervalMs = 2000 // 全局同步触发最小间隔（毫秒）
	defaultUpToDateSilenceWindowMins  = 5    // 同步一致状态静默窗口（分钟）

	// 网络连接配置
	defaultConnectTimeout = 15 * time.Second // 网络连接超时
	defaultWriteTimeout   = 10 * time.Second // 网络写入超时
	defaultReadTimeout    = 30 * time.Second // 网络读取超时
	defaultRetryDelay     = 2 * time.Second  // 重试延迟

	// 重试策略配置
	defaultMaxRetryAttempts    = 3                // 最大重试次数
	defaultFailoverNodeCount   = 2                // 故障转移节点数
	defaultNodeHealthThreshold = 60 * time.Second // 节点健康度阈值

	// 性能优化配置
	defaultEnableAsyncProcessing  = true             // 启用异步处理
	defaultBlockValidationTimeout = 10 * time.Second // 区块验证超时
	defaultNetworkLatencyBuffer   = 2 * time.Second  // 网络延迟缓冲
	defaultSyncProgressReportMs   = 5000             // 同步进度报告间隔毫秒

	// K桶批量处理配置
	defaultMaxBatchSize                         = 100   // K桶批量处理最大批次大小
	defaultMaxConcurrentBlockValidationWorkers  = 4     // 最大并发区块验证工作协程数
	defaultDefaultBatchProcessingTimeoutSeconds = 60    // 默认批量处理超时秒数
	defaultEnableIntelligentBatchSizing         = true  // 启用智能批次大小调整
	defaultBatchProcessingMemoryLimitMB         = 256   // 批量处理内存限制MB
	defaultBatchErrorToleranceLevel             = 1     // 批量处理错误容忍度级别（1=低容忍度）
	defaultEnableBatchPipelineProcessing        = false // 启用批量流水线处理
	defaultBatchValidationMode                  = 1     // 批量验证模式（1=标准验证）

	// === UTXO域默认值 ===
	defaultStateRetentionBlocks = 128  // 状态保留区块数
	defaultPruningEnabled       = true // 启用状态修剪
	defaultPruningInterval      = 1000 // 修剪间隔
	defaultStateCacheSize       = 5000 // 状态缓存数量

	// === 执行域默认值 ===
	defaultVMEnabled         = true    // 启用虚拟机
	defaultExecutionFeeLimit = 8000000 // 执行费用限制（已废弃，WES不需要Gas）
	defaultCallStackLimit    = 1024    // 调用栈限制
	
	// === ISPC资源限制默认值 ===
	defaultExecutionTimeoutSeconds = 60        // 执行超时时间（秒）
	defaultMaxMemoryMB            = 512        // 最大内存限制（MB）
	defaultMaxTraceSizeMB         = 10         // 最大执行轨迹大小（MB）
	defaultMaxTempStorageMB       = 100        // 最大临时存储（MB）
	defaultMaxHostFunctionCalls  = uint32(10000) // 最大宿主函数调用次数
	defaultMaxUTXOQueries        = uint32(1000)  // 最大UTXO查询次数
	defaultMaxResourceQueries    = uint32(1000)  // 最大资源查询次数
	defaultMaxConcurrentExecutions = 100        // 最大并发执行数
	
	// === ISPC资源统计默认值 ===
	defaultEnableResourceStats = true  // 启用资源统计
	defaultEnableResourceLogs  = false // 禁用资源日志（开发/调试用）

	// === 兼容字段默认值 ===
	defaultNetworkType       = "testnet"          // 默认测试网络
	defaultGenesisTimestamp  = int64(1704067200)  // 创世时间戳（2024-01-01 00:00:00 UTC）
	defaultGenesisDifficulty = uint64(1)          // 创世难度
	defaultInitialSupply     = uint64(1000000000) // 初始供应量

	// === 节点模式默认值 ===
	defaultNodeMode = types.NodeModeFull // 默认全节点
)

// 默认创世账户
var defaultGenesisAccounts = []GenesisAccount{
	{
		PublicKey: "02349cb6a770701494eb716d0b430ebcff740a354b2ceaedb4d3a2b4bad2237896",
		Amount:    500000000, // 5亿代币
	},
	{
		PublicKey: "037b9d77205ea12eec387883262ef67e215b71901ff3d3d0d8cc49509077fa2926",
		Amount:    500000000, // 5亿代币
	},
}

// 默认验证者
var defaultValidators = []string{
	"validator_1",
	"validator_2",
	"validator_3",
}

// 默认重试间隔序列
var defaultRetryBackoffIntervals = []time.Duration{
	3 * time.Second,  // 第一次重试：3秒
	5 * time.Second,  // 第二次重试：5秒
	10 * time.Second, // 第三次重试：10秒
	30 * time.Second, // 第四次重试：30秒
}

// K桶入桶重试 backoff（毫秒）
var defaultKBucketPeerAddRetryBackoffsMs = []int{200, 1000, 3000, 8000, 15000}

// createDefaultBlockchainOptions 创建默认的区块链配置选项
func createDefaultBlockchainOptions() *BlockchainOptions {
	return &BlockchainOptions{
		// === 基础链配置 ===
		ChainID:   defaultChainID,
		NetworkID: defaultNetworkID,
		NodeMode:  defaultNodeMode,

		// === 区块域配置 ===
		Block: BlockConfig{
			MaxBlockSize:      defaultMaxBlockSize,
			MaxTransactions:   defaultMaxTransactions,
			BlockTimeTarget:   defaultBlockTimeTarget,
			MinBlockInterval:  defaultMinBlockInterval,
			MinDifficulty:     defaultMinDifficulty,
			MaxTimeDrift:      defaultMaxTimeDrift,
			ValidationTimeout: defaultValidationTimeout,
			CacheSize:         defaultBlockCacheSize,
		},

		// === 交易域配置 ===
		Transaction: TransactionConfig{
			MaxTransactionSize:    defaultMaxTransactionSize,
			BaseFeePerByte:        defaultBaseFeePerByte,
			MinimumFee:            defaultMinimumFee,
			MaximumFee:            defaultMaximumFee,
			BaseExecutionFeePrice: defaultBaseExecutionFeePrice,
			CacheSize:             defaultTransactionCacheSize,
			CongestionMultiplier:  defaultCongestionMultiplier,
			MaxBatchTransferSize:  defaultMaxBatchTransferSize, // 批量转账配置

			// === 费用相关配置（简化设计）===
			DustThreshold: defaultDustThreshold, // 粉尘阈值
			BaseFeeRate:   defaultBaseFeeRate,   // 基础费率参考
		},

		// === 同步域配置 ===
		Sync: SyncConfig{
			BatchSize:     defaultSyncBatchSize,
			Concurrency:   defaultSyncConcurrency,
			Timeout:       defaultSyncTimeout,
			MinPeerCount:  defaultSyncMinPeerCount,
			MaxPeerCount:  defaultSyncMaxPeerCount,
			RetryAttempts: defaultSyncRetryAttempts,
			MaxReorgDepth: defaultMaxReorgDepth,

			// === K桶智能同步高级配置 ===
			Advanced: SyncAdvancedConfig{
				// K桶节点选择配置
				KBucketSelectionCount:    defaultKBucketSelectionCount,
				KBucketSelectionStrategy: defaultKBucketSelectionStrategy,
				NodeSelectionTimeout:     defaultNodeSelectionTimeout,
				MaxConcurrentRequests:    defaultMaxConcurrentRequests,

				// 智能分页配置
				MaxResponseSizeBytes:       defaultMaxResponseSizeBytes,
				MaxBlocksPerRequest:        defaultMaxBlocksPerRequest,
				IntelligentPagingThreshold: defaultIntelligentPagingThreshold,
				MinBlocksGuarantee:         defaultMinBlocksGuarantee,

				// 时间检查配置
				TimeCheckEnabled:       defaultTimeCheckEnabled,
				TimeCheckThresholdMins: defaultTimeCheckThresholdMins,
				TimeCheckIntervalMins:  defaultTimeCheckIntervalMins,
				SyncTriggerTimeout:     defaultSyncTriggerTimeout,

				// 节点同步状态缓存配置
				PeerSyncCacheExpiryMins: defaultPeerSyncCacheExpiryMins,

				// 上游节点记忆（抗抖动）配置
				UpstreamMemoryTTLSeconds:       defaultUpstreamMemoryTTLSeconds,
				UpstreamMaxConsecutiveFailures: defaultUpstreamMaxConsecutiveFailures,

				// K桶入桶保障配置（防空桶风险）
				KBucketReconcileIntervalSeconds: defaultKBucketReconcileIntervalSeconds,
				KBucketPeerAddRetryBackoffsMs:   append([]int{}, defaultKBucketPeerAddRetryBackoffsMs...),

				// 存储/索引自愈（persistence 内部子能力）配置
				RepairEnabled:         defaultRepairEnabled,
				RepairMaxConcurrency:  defaultRepairMaxConcurrency,
				RepairThrottleSeconds: defaultRepairThrottleSeconds,
				RepairHashIndexWindow: defaultRepairHashIndexWindow,

				// fork-aware 自动 reorg（sync 模块）配置
				AutoReorgMaxDepth: defaultAutoReorgMaxDepth,

				// 节点熔断（Circuit Breaker）配置
				CircuitBreakerFailureThreshold: defaultCircuitBreakerFailureThreshold,
				CircuitBreakerRecoverySeconds:  defaultCircuitBreakerRecoverySeconds,

				// 事件去抖与限流配置
				PeerEventDebounceMs:        defaultPeerEventDebounceMs,
				GlobalMinTriggerIntervalMs: defaultGlobalMinTriggerIntervalMs,
				UpToDateSilenceWindowMins:  defaultUpToDateSilenceWindowMins,

				// 网络连接配置
				ConnectTimeout: defaultConnectTimeout,
				WriteTimeout:   defaultWriteTimeout,
				ReadTimeout:    defaultReadTimeout,
				RetryDelay:     defaultRetryDelay,

				// 重试策略配置
				RetryBackoffIntervals: defaultRetryBackoffIntervals,
				MaxRetryAttempts:      defaultMaxRetryAttempts,
				FailoverNodeCount:     defaultFailoverNodeCount,
				NodeHealthThreshold:   defaultNodeHealthThreshold,

				// 性能优化配置
				EnableAsyncProcessing:  defaultEnableAsyncProcessing,
				BlockValidationTimeout: defaultBlockValidationTimeout,
				NetworkLatencyBuffer:   defaultNetworkLatencyBuffer,
				SyncProgressReportMs:   defaultSyncProgressReportMs,

				// K桶批量处理配置
				MaxBatchSize:                         defaultMaxBatchSize,
				MaxConcurrentBlockValidationWorkers:  defaultMaxConcurrentBlockValidationWorkers,
				DefaultBatchProcessingTimeoutSeconds: defaultDefaultBatchProcessingTimeoutSeconds,
				EnableIntelligentBatchSizing:         defaultEnableIntelligentBatchSizing,
				BatchProcessingMemoryLimitMB:         defaultBatchProcessingMemoryLimitMB,
				BatchErrorToleranceLevel:             defaultBatchErrorToleranceLevel,
				EnableBatchPipelineProcessing:        defaultEnableBatchPipelineProcessing,
				BatchValidationMode:                  defaultBatchValidationMode,
			},
		},

		// === UTXO域配置 ===
		UTXO: UTXOConfig{
			StateRetentionBlocks: defaultStateRetentionBlocks,
			PruningEnabled:       defaultPruningEnabled,
			PruningInterval:      defaultPruningInterval,
			CacheSize:            defaultStateCacheSize,
		},

		// === 执行域配置 ===
		Execution: ExecutionConfig{
			VMEnabled:         defaultVMEnabled,
			ExecutionFeeLimit: defaultExecutionFeeLimit,
			CallStackLimit:    defaultCallStackLimit,
			ResourceLimits: &ResourceLimitsConfig{
				// 向后兼容字段（已废弃）
				GlobalQuota:       defaultExecutionFeeLimit,
				ExecutionTime:     30000, // 30秒执行时间限制（已废弃）
				MemoryLimit:       "512MB",
				ExecutionFeeLimit: defaultExecutionFeeLimit,
			},
			WASM: &WASMConfig{
				EnableOptimization: true,
				MaxStackSize:       1024,
				MaxMemoryPages:     256,
			},
			ISPC: &ISPCConfig{
				ResourceLimits: &ResourceLimitsConfig{
					ExecutionTimeoutSeconds: defaultExecutionTimeoutSeconds,
					MaxMemoryMB:            defaultMaxMemoryMB,
					MemoryLimit:            "512MB", // 向后兼容
					MaxTraceSizeMB:         defaultMaxTraceSizeMB,
					MaxTempStorageMB:       defaultMaxTempStorageMB,
					MaxHostFunctionCalls:  defaultMaxHostFunctionCalls,
					MaxUTXOQueries:        defaultMaxUTXOQueries,
					MaxResourceQueries:    defaultMaxResourceQueries,
					MaxConcurrentExecutions: defaultMaxConcurrentExecutions,
				},
				EnableResourceStats: defaultEnableResourceStats,
				EnableResourceLogs:  defaultEnableResourceLogs,
				// 异步功能配置（默认禁用，保持向后兼容）
				AsyncZKProof: &AsyncZKProofConfig{
					Enabled:    false, // 默认禁用，通过配置启用
					Workers:    2,     // 默认2个工作线程
					MinWorkers: 1,     // 最小1个工作线程
					MaxWorkers: 10,    // 最大10个工作线程
				},
				AsyncTrace: &AsyncTraceConfig{
					Enabled:      false,              // 默认禁用，通过配置启用
					Workers:      2,                  // 默认2个工作线程
					BatchSize:    100,                // 默认批量大小100
					BatchTimeout: 100 * time.Millisecond, // 默认批量超时100ms
					MaxRetries:   3,                  // 默认最大重试3次
					RetryDelay:   10 * time.Millisecond, // 默认重试延迟10ms
				},
			},
		},

		// === 向后兼容配置 ===
		GenesisConfig: GenesisConfig{
			Accounts:      defaultGenesisAccounts,
			InitialSupply: defaultInitialSupply,
			Validators:    defaultValidators,
			ChainParams: ChainParams{
				BlockTime:         defaultBlockTimeTarget,
				Difficulty:        defaultGenesisDifficulty,
				ExecutionFeeLimit: defaultExecutionFeeLimit,
			},
		},
		NetworkType:      defaultNetworkType,
		GenesisTimestamp: defaultGenesisTimestamp,
	}
}
