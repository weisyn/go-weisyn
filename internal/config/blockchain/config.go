package blockchain

import (
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/weisyn/v1/pkg/types"
)

// BlockchainOptions 区块链配置选项
// 按域组织的分层配置结构，只包含实际使用的核心配置
type BlockchainOptions struct {
	// === 基础链配置 ===
	ChainID   uint64 `json:"chain_id"`
	NetworkID uint64 `json:"network_id"`

	// === 节点运行模式（全局约束）===
	// Light：仅同步区块头；Full：同步头+体
	NodeMode types.NodeMode `json:"node_mode"`

	// === 区块域配置 ===
	Block BlockConfig `json:"block"`

	// === 交易域配置 ===
	Transaction TransactionConfig `json:"transaction"`

	// === 同步域配置 ===
	Sync SyncConfig `json:"sync"`

	// === UTXO域配置 ===
	UTXO UTXOConfig `json:"utxo"`

	// === 执行域配置 ===
	Execution ExecutionConfig `json:"execution"`

	// === 块文件GC配置 ===
	BlockFileGC *BlockFileGCConfig `json:"block_file_gc,omitempty"`

	// === 临时兼容字段（Genesis和启动流程需要）===
	// 这些字段保持向后兼容，支持现有的startup模块
	GenesisConfig    GenesisConfig `json:"genesis"`
	NetworkType      string        `json:"network_type"`      // "mainnet", "testnet", "devnet"
	GenesisTimestamp int64         `json:"genesis_timestamp"` // 创世时间戳
}

// BlockConfig 区块域配置
type BlockConfig struct {
	MaxBlockSize      uint64        `json:"max_block_size"`     // 最大区块大小
	MaxTransactions   int           `json:"max_transactions"`   // 最大交易数
	BlockTimeTarget   int           `json:"block_time_target"`  // 目标出块时间(秒)
	MinBlockInterval  int           `json:"min_block_interval"` // 最小区块间隔(秒)
	MinDifficulty     uint64        `json:"min_difficulty"`     // 最小难度
	MaxTimeDrift      int           `json:"max_time_drift"`     // 最大时间偏差(秒)
	ValidationTimeout time.Duration `json:"validation_timeout"` // 验证超时
	CacheSize         int           `json:"cache_size"`         // 区块缓存数量
}

// TransactionConfig 交易域配置
type TransactionConfig struct {
	MaxTransactionSize    uint64  `json:"max_transaction_size"`     // 最大交易大小
	BaseFeePerByte        uint64  `json:"base_fee_per_byte"`        // 基础字节费率
	MinimumFee            uint64  `json:"minimum_fee"`              // 最低费用
	MaximumFee            uint64  `json:"maximum_fee"`              // 最高费用
	BaseExecutionFeePrice uint64  `json:"base_execution_fee_price"` // 基础执行费用价格
	CacheSize             int     `json:"cache_size"`               // 交易缓存数量
	CongestionMultiplier  float64 `json:"congestion_multiplier"`    // 拥堵系数
	MaxBatchTransferSize  int     `json:"max_batch_transfer_size"`  // 批量转账最大笔数

	// === 费用相关配置（与transaction.proto fee_mechanism对齐）===
	DustThreshold float64 `json:"dust_threshold"` // 粉尘阈值（最小找零金额，避免粉尘攻击）
	BaseFeeRate   float64 `json:"base_fee_rate"`  // 基础费率参考值（如万三 = 0.0003，仅作参考）
}

// SyncConfig 同步域配置
type SyncConfig struct {
	// === 基础同步配置 ===
	BatchSize     int           `json:"batch_size"`      // 批处理大小
	Concurrency   int           `json:"concurrency"`     // 并发度
	Timeout       time.Duration `json:"timeout"`         // 同步超时
	MinPeerCount  int           `json:"min_peer_count"`  // 最小节点数
	MaxPeerCount  int           `json:"max_peer_count"`  // 最大节点数
	RetryAttempts int           `json:"retry_attempts"`  // 重试次数
	MaxReorgDepth int           `json:"max_reorg_depth"` // 最大重组深度

	// === K桶智能同步配置 ===
	Advanced SyncAdvancedConfig `json:"advanced"` // 高级同步配置
}

// SyncAdvancedConfig 高级同步配置
//
// 🎯 **K桶智能同步配置**
//
// 支持基于Kademlia距离算法的智能节点选择和分页同步机制，
// 提供精细化的同步控制和网络优化配置。
//
// 🔧 **配置分类**：
// - K桶节点选择：控制节点选择策略和数量
// - 智能分页：控制网络传输大小和分页策略
// - 时间检查：控制基于时间的同步触发机制
// - 重试策略：控制重试间隔和故障恢复
type SyncAdvancedConfig struct {
	// === K桶节点选择配置 ===
	KBucketSelectionCount    int           `json:"k_bucket_selection_count"`    // K桶节点选择数量 (默认5)
	KBucketSelectionStrategy string        `json:"k_bucket_selection_strategy"` // K桶节点选择策略 ("distance", "random", "mixed")
	NodeSelectionTimeout     time.Duration `json:"node_selection_timeout"`      // 节点选择超时 (默认3秒)
	MaxConcurrentRequests    int           `json:"max_concurrent_requests"`     // 最大并发请求数 (默认3)

	// === 智能分页配置 ===
	MaxResponseSizeBytes       uint32 `json:"max_response_size_bytes"`      // 网络响应大小限制 (默认5MB)
	MaxBlocksPerRequest        int    `json:"max_blocks_per_request"`       // 每次请求最大区块数 (默认100)
	IntelligentPagingThreshold uint32 `json:"intelligent_paging_threshold"` // 智能分页阈值 (默认2MB)
	MinBlocksGuarantee         int    `json:"min_blocks_guarantee"`         // 最小区块保证数量 (默认1)

	// === 时间检查配置 ===
	TimeCheckEnabled       bool          `json:"time_check_enabled"`        // 是否启用时间检查触发 (默认true)
	TimeCheckThresholdMins int           `json:"time_check_threshold_mins"` // 时间检查阈值分钟 (默认10分钟)
	TimeCheckIntervalMins  int           `json:"time_check_interval_mins"`  // 时间检查间隔分钟 (默认5分钟)
	SyncTriggerTimeout     time.Duration `json:"sync_trigger_timeout"`      // 同步触发超时 (默认30秒)

	// === 节点同步状态缓存配置 ===
	PeerSyncCacheExpiryMins int `json:"peer_sync_cache_expiry_mins"` // 节点同步状态缓存过期时间（分钟）(默认5分钟)

	// === 上游节点记忆（抗抖动）配置 ===
	UpstreamMemoryTTLSeconds          int `json:"upstream_memory_ttl_seconds"`           // 上一次可用上游节点的记忆TTL（秒）(默认600=10分钟)
	UpstreamMaxConsecutiveFailures    int `json:"upstream_max_consecutive_failures"`     // 连续失败达到该阈值时清除记忆上游并快速切换 (默认3)

	// === K桶入桶保障配置（防空桶风险） ===
	KBucketReconcileIntervalSeconds int   `json:"kbucket_reconcile_interval_seconds"` // 周期性reconcile间隔（秒）(默认30)
	KBucketPeerAddRetryBackoffsMs   []int `json:"kbucket_peer_add_retry_backoffs_ms"` // 入桶重试backoff序列（毫秒）(默认[200,1000,3000,8000,15000])

	// === 存储/索引自愈（persistence 内部子能力）配置 ===
	RepairEnabled           bool `json:"repair_enabled"`              // 是否启用在线自愈（默认true）
	RepairMaxConcurrency    int  `json:"repair_max_concurrency"`      // 自愈并发数（默认2）
	RepairThrottleSeconds   int  `json:"repair_throttle_seconds"`     // 同一目标（key/hash）最小修复间隔（秒，默认60）
	RepairHashIndexWindow   int  `json:"repair_hash_index_window"`    // hash->height 索引修复扫描窗口（blocks，默认5000）

	// === fork-aware 自动 reorg（sync 模块）配置 ===
	// AutoReorgMaxDepth 控制同步模块在检测到分叉后，允许自动下载并重组的最大深度：
	// - depth = remote_tip_height - common_ancestor_height
	// - 超过该值将拒绝自动重组（避免极端场景下的巨大回滚/下载成本）
	AutoReorgMaxDepth int `json:"auto_reorg_max_depth"`

	// === 节点熔断（Circuit Breaker）配置 ===
	// CircuitBreakerFailureThreshold 连续失败达到该阈值后触发熔断（默认3次）
	CircuitBreakerFailureThreshold int `json:"circuit_breaker_failure_threshold"`
	// CircuitBreakerRecoverySeconds 熔断后恢复时间（秒，默认300=5分钟）
	CircuitBreakerRecoverySeconds int `json:"circuit_breaker_recovery_seconds"`

	// === 事件去抖与限流配置 ===
	PeerEventDebounceMs        int `json:"peer_event_debounce_ms"`         // 同一节点连接事件去抖时间（毫秒）(默认1000ms)
	GlobalMinTriggerIntervalMs int `json:"global_min_trigger_interval_ms"` // 全局同步触发最小间隔（毫秒）(默认2000ms)
	UpToDateSilenceWindowMins  int `json:"up_to_date_silence_window_mins"` // 同步一致状态静默窗口（分钟）(默认5分钟)

	// === 网络连接配置 ===
	ConnectTimeout time.Duration `json:"connect_timeout"` // 网络连接超时 (默认15秒)
	WriteTimeout   time.Duration `json:"write_timeout"`   // 网络写入超时 (默认10秒)
	ReadTimeout    time.Duration `json:"read_timeout"`    // 网络读取超时 (默认30秒)
	RetryDelay     time.Duration `json:"retry_delay"`     // 重试延迟 (默认2秒)

	// === 重试策略配置 ===
	RetryBackoffIntervals []time.Duration `json:"retry_backoff_intervals"` // 重试间隔序列 (默认[3s,5s,10s,30s])
	MaxRetryAttempts      int             `json:"max_retry_attempts"`      // 最大重试次数 (默认3)
	FailoverNodeCount     int             `json:"failover_node_count"`     // 故障转移节点数 (默认2)
	NodeHealthThreshold   time.Duration   `json:"node_health_threshold"`   // 节点健康度阈值 (默认60秒)

	// === 性能优化配置 ===
	EnableAsyncProcessing  bool          `json:"enable_async_processing"`  // 是否启用异步处理 (默认true)
	BlockValidationTimeout time.Duration `json:"block_validation_timeout"` // 区块验证超时 (默认10秒)
	NetworkLatencyBuffer   time.Duration `json:"network_latency_buffer"`   // 网络延迟缓冲 (默认2秒)
	SyncProgressReportMs   int           `json:"sync_progress_report_ms"`  // 同步进度报告间隔毫秒 (默认5000)

	// === K桶批量处理配置 ===
	MaxBatchSize                         int  `json:"max_batch_size"`                           // K桶批量处理最大批次大小 (默认100)
	MaxConcurrentBlockValidationWorkers  int  `json:"max_concurrent_block_validation_workers"`  // 最大并发区块验证工作协程数 (默认4)
	DefaultBatchProcessingTimeoutSeconds int  `json:"default_batch_processing_timeout_seconds"` // 默认批量处理超时秒数 (默认60)
	EnableIntelligentBatchSizing         bool `json:"enable_intelligent_batch_sizing"`          // 是否启用智能批次大小调整 (默认true)
	BatchProcessingMemoryLimitMB         int  `json:"batch_processing_memory_limit_mb"`         // 批量处理内存限制MB (默认256)
	BatchErrorToleranceLevel             int  `json:"batch_error_tolerance_level"`              // 批量处理错误容忍度级别 (0=无容忍,1=低,2=中,3=高,默认1)
	EnableBatchPipelineProcessing        bool `json:"enable_batch_pipeline_processing"`         // 是否启用批量流水线处理 (默认false)
	BatchValidationMode                  int  `json:"batch_validation_mode"`                    // 批量验证模式 (0=快速,1=标准,2=严格,3=跳过,默认1)
}

// UTXOConfig UTXO域配置
type UTXOConfig struct {
	StateRetentionBlocks int  `json:"state_retention_blocks"` // 状态保留区块数
	PruningEnabled       bool `json:"pruning_enabled"`        // 是否启用修剪
	PruningInterval      int  `json:"pruning_interval"`       // 修剪间隔
	CacheSize            int  `json:"cache_size"`             // 状态缓存数量
}

// ExecutionConfig 执行域配置
type ExecutionConfig struct {
	VMEnabled         bool                  `json:"vm_enabled"`          // 是否启用虚拟机
	ExecutionFeeLimit uint64                `json:"execution_fee_limit"` // 执行费用限制（已废弃，WES不需要Gas）
	CallStackLimit    int                   `json:"call_stack_limit"`    // 调用栈限制
	ResourceLimits    *ResourceLimitsConfig `json:"resource_limits"`     // 资源限制配置（向后兼容）
	WASM              *WASMConfig           `json:"wasm"`                // WASM引擎配置
	ISPC              *ISPCConfig           `json:"ispc"`                // ISPC执行配置（新增）
}

// ResourceLimitsConfig 资源限制配置（ISPC专用）
// 注意：WES不需要Gas计费，这是本地资源配额管理
type ResourceLimitsConfig struct {
	// 执行时间限制
	ExecutionTimeoutSeconds int `json:"execution_timeout_seconds"` // 执行超时时间（秒，默认60）
	
	// 内存限制
	MaxMemoryMB    int    `json:"max_memory_mb"`    // 最大内存限制（MB，默认512）
	MemoryLimit    string `json:"memory_limit"`     // 内存限制（字符串格式，向后兼容，如"512MB"）
	
	// 存储限制
	MaxTraceSizeMB     int `json:"max_trace_size_mb"`     // 最大执行轨迹大小（MB，默认10）
	MaxTempStorageMB   int `json:"max_temp_storage_mb"`   // 最大临时存储（MB，默认100）
	
	// 操作限制
	MaxHostFunctionCalls uint32 `json:"max_host_function_calls"` // 最大宿主函数调用次数（默认10000）
	MaxUTXOQueries       uint32 `json:"max_utxo_queries"`         // 最大UTXO查询次数（默认1000）
	MaxResourceQueries   uint32 `json:"max_resource_queries"`    // 最大资源查询次数（默认1000）
	
	// 配额管理
	MaxConcurrentExecutions int `json:"max_concurrent_executions"` // 最大并发执行数（默认100）
	
	// 向后兼容字段（已废弃，保留用于兼容）
	GlobalQuota       uint64 `json:"global_quota,omitempty"`        // 全局配额（已废弃）
	ExecutionTime     uint64 `json:"execution_time,omitempty"`      // 执行时间限制（已废弃，使用ExecutionTimeoutSeconds）
	ExecutionFeeLimit uint64 `json:"execution_fee_limit,omitempty"` // 执行费用限制（已废弃，WES不需要Gas）
}

// ISPCConfig ISPC执行配置
// 用于配置ISPC执行引擎的资源限制和配额管理
type ISPCConfig struct {
	// 资源限制
	ResourceLimits *ResourceLimitsConfig `json:"resource_limits"` // 资源限制配置
	
	// 资源统计
	EnableResourceStats bool `json:"enable_resource_stats"` // 是否启用资源统计（默认true）
	EnableResourceLogs  bool `json:"enable_resource_logs"`  // 是否启用资源日志（默认false，开发/调试用）
	
	// 异步ZK证明生成配置
	AsyncZKProof *AsyncZKProofConfig `json:"async_zk_proof,omitempty"` // 异步ZK证明生成配置
	
	// 异步轨迹记录配置
	AsyncTrace *AsyncTraceConfig `json:"async_trace,omitempty"` // 异步轨迹记录配置
}

// AsyncZKProofConfig 异步ZK证明生成配置
type AsyncZKProofConfig struct {
	Enabled    bool `json:"enabled"`     // 是否启用异步ZK证明生成（默认false）
	Workers    int  `json:"workers"`     // 工作线程数量（默认2）
	MinWorkers int  `json:"min_workers"` // 最小工作线程数量（默认1）
	MaxWorkers int  `json:"max_workers"` // 最大工作线程数量（默认10）
}

// AsyncTraceConfig 异步轨迹记录配置
type AsyncTraceConfig struct {
	Enabled      bool          `json:"enabled"`        // 是否启用异步轨迹记录（默认false）
	Workers      int           `json:"workers"`       // 工作线程数量（默认2）
	BatchSize    int           `json:"batch_size"`     // 批量大小（默认100）
	BatchTimeout time.Duration `json:"batch_timeout"` // 批量超时时间（默认100ms）
	MaxRetries   int           `json:"max_retries"`   // 最大重试次数（默认3）
	RetryDelay   time.Duration `json:"retry_delay"`   // 重试延迟（默认10ms）
}

// WASMConfig WASM引擎配置
type WASMConfig struct {
	EnableOptimization bool `json:"enable_optimization"` // 是否启用优化
	MaxStackSize       int  `json:"max_stack_size"`      // 最大栈大小
	MaxMemoryPages     int  `json:"max_memory_pages"`    // 最大内存页数
}

// BlockFileGCConfig 块文件GC配置
//
// BlockFileGC 是 chain 模块的后台维护服务，用于清理 blocks/ 目录中的
// 不可达块文件（fork 后的旧链残留）。
//
// 工作原理：
//  1. Mark（标记）：扫描 indices:height 索引，构建可达区块集合
//  2. Sweep（清除）：扫描 blocks/ 目录，删除不在可达集合中的文件
//
// 安全保护：
//  - 保护窗口：最近 ProtectRecentHeight 个区块不会被删除
//  - Dry-run 模式：只检测不删除，用于验证
//  - 限速：避免 I/O 压力
type BlockFileGCConfig struct {
	// 是否启用 GC（默认 false）
	Enabled bool `json:"enabled"`

	// Dry-run 模式：只检测不删除（默认 true）
	DryRun bool `json:"dry_run"`

	// 自动运行间隔（秒，默认 3600 = 1小时）
	IntervalSeconds int `json:"interval_seconds"`

	// 限速：每秒最多处理的文件数（默认 100）
	RateLimitFilesPerSecond int `json:"rate_limit_files_per_sec"`

	// 保护窗口：保护最近 N 个区块（默认 1000）
	ProtectRecentHeight uint64 `json:"protect_recent_height"`
}

// GenesisConfig 创世配置（向后兼容）
type GenesisConfig struct {
	Accounts      []GenesisAccount `json:"accounts"`       // 初始账户分配
	InitialSupply uint64           `json:"initial_supply"` // 初始代币供应量
	Validators    []string         `json:"validators"`     // 初始验证者
	ChainParams   ChainParams      `json:"chain_params"`   // 链参数
}

// GenesisAccount 创世账户
type GenesisAccount struct {
	PublicKey string `json:"public_key"` // 公钥
	Amount    uint64 `json:"amount"`     // 初始余额
}

// ChainParams 链参数（向后兼容）
type ChainParams struct {
	BlockTime         int    `json:"block_time"`          // 出块时间
	Difficulty        uint64 `json:"difficulty"`          // 初始难度
	ExecutionFeeLimit uint64 `json:"execution_fee_limit"` // 执行费用限制
}

// Config 区块链配置实现
type Config struct {
	options               *BlockchainOptions
	externalGenesisConfig *types.GenesisConfig // 外部传入的创世配置（通过provider加载）
}

// 配置解析日志开关：
// - 默认关闭（避免启动/运行期间刷屏）
// - 仅当 WES_CONFIG_DEBUG=true 且非 CLI_MODE 时才输出
func configDebugEnabled() bool {
	return os.Getenv("WES_CONFIG_DEBUG") == "true" && os.Getenv("WES_CLI_MODE") != "true"
}

var printedConfigDebugOnce atomic.Bool

// UserBlockchainConfig 用户区块链配置扩展结构
// 包含创世配置信息，供provider传递
type UserBlockchainConfig struct {
	// 嵌入原有配置
	Genesis interface{} `json:"genesis,omitempty"`
	// 外部创世配置（provider从文件加载后传入）
	ExternalGenesisConfig *types.GenesisConfig `json:"-"` // 不参与JSON序列化
}

// New 创建区块链配置实现
func New(userConfig interface{}) *Config {
	defaultOptions := createDefaultBlockchainOptions()

	config := &Config{
		options: defaultOptions,
	}

	// 处理用户配置
	if userConfig != nil {
		// 只在显式开启调试时输出；并且仅在进程生命周期内打印一次开头提示，避免刷屏
		if configDebugEnabled() && printedConfigDebugOnce.CompareAndSwap(false, true) {
			println("🔧 CONFIG DEBUG: 开始处理用户配置（WES_CONFIG_DEBUG=true）")
		}
		// 检查是否为扩展的用户配置结构
		if extConfig, ok := userConfig.(*UserBlockchainConfig); ok {
			if configDebugEnabled() {
				println("🔧 CONFIG DEBUG: 用户配置是扩展结构")
			}
			// 优先使用外部创世配置（通过provider从文件加载）
			if extConfig.ExternalGenesisConfig != nil && config.externalGenesisConfig == nil {
				if configDebugEnabled() {
					println("🔧 CONFIG DEBUG: 首次设置外部创世配置")
				}
				config.externalGenesisConfig = extConfig.ExternalGenesisConfig
			} else if extConfig.ExternalGenesisConfig != nil {
				if configDebugEnabled() {
					println("🔧 CONFIG DEBUG: 外部创世配置已存在，跳过重复设置")
				}
			}
			// 处理原有的genesis配置
			userConfig = extConfig.Genesis
			if configDebugEnabled() {
				println("🔧 CONFIG DEBUG: 提取内部genesis配置")
			}
		} else {
			if configDebugEnabled() {
				println("🔧 CONFIG DEBUG: 用户配置不是扩展结构，直接处理")
			}
		}

		// 处理内部配置（包括blockchain配置）
		if configDebugEnabled() {
			println("🔧 CONFIG DEBUG: 开始处理内部配置逻辑")
		}
		config.processLegacyConfig(userConfig)

		// 如果外部创世配置存在，优先使用外部配置覆盖创世账户
		if config.externalGenesisConfig != nil {
			if configDebugEnabled() {
				println("🔧 CONFIG DEBUG: 外部创世配置存在，将覆盖内部创世配置")
			}
		}
	} else {
		// 默认不输出
	}

	return config
}

// processLegacyConfig 处理原有的配置逻辑（保持向后兼容）
func (c *Config) processLegacyConfig(userConfig interface{}) {
	// 调试：输出用户配置的详细信息
	if configDebugEnabled() {
		if userConfig != nil {
			println("🔧 DEBUG: userConfig类型:", fmt.Sprintf("%T", userConfig))
			println("🔧 DEBUG: userConfig值:", fmt.Sprintf("%+v", userConfig))
		} else {
			println("🔧 DEBUG: userConfig为nil")
		}
	}

	// 如果提供了用户配置，尝试解析并合并
	if userConfig != nil {
		// 首先尝试直接类型断言为我们期望的结构体
		if structConfig, ok := userConfig.(*struct {
			Genesis *struct {
				GenesisAccounts []struct {
					PublicKey string `json:"public_key"`
					Amount    uint64 `json:"amount"`
				} `json:"genesis_accounts,omitempty"`
			} `json:"genesis,omitempty"`
			Block *struct {
				MinBlockInterval int `json:"min_block_interval,omitempty"`
			} `json:"block,omitempty"`
		}); ok {
			if configDebugEnabled() {
				println("🔧 DEBUG: 成功转换为结构体指针")
			}
			if structConfig.Genesis != nil && len(structConfig.Genesis.GenesisAccounts) > 0 {
				if configDebugEnabled() {
					println("🔧 DEBUG: 找到Genesis配置，账户数:", len(structConfig.Genesis.GenesisAccounts))
				}
				var genesisAccounts []GenesisAccount
				for i, account := range structConfig.Genesis.GenesisAccounts {
					if configDebugEnabled() {
						println("🔧 DEBUG: 处理账户", i, ": PublicKey=", account.PublicKey, ", Amount=", account.Amount)
					}
					if account.PublicKey != "" && account.Amount > 0 {
						genesisAccounts = append(genesisAccounts, GenesisAccount{
							PublicKey: account.PublicKey,
							Amount:    account.Amount,
						})
					}
				}
				if len(genesisAccounts) > 0 {
					if configDebugEnabled() {
						println("🔧 DEBUG: 成功解析配置，账户数:", len(genesisAccounts))
					}
					c.options.GenesisConfig.Accounts = genesisAccounts
					if configDebugEnabled() {
						println("🔧 DEBUG: 已更新默认配置中的创世账户")
					}
				}
			}

			// 处理block配置
			if structConfig.Block != nil {
				if configDebugEnabled() {
					println("🔧 DEBUG: 找到Block配置，MinBlockInterval:", structConfig.Block.MinBlockInterval)
				}
				if structConfig.Block.MinBlockInterval > 0 {
					c.options.Block.MinBlockInterval = structConfig.Block.MinBlockInterval
					if configDebugEnabled() {
						println("🔧 DEBUG: 已更新MinBlockInterval为:", structConfig.Block.MinBlockInterval)
					}
				}
			}
		} else if configMap, ok := userConfig.(map[string]interface{}); ok {
			if configDebugEnabled() {
				println("🔧 DEBUG: 成功转换为map[string]interface{}")
			}

			// 🔧 修复：处理链ID配置 - 统一为uint64类型（遵循pb定义）
			if chainIdVal, exists := configMap["chain_id"]; exists {
				if chainId, ok := chainIdVal.(uint64); ok {
					c.options.ChainID = chainId
					if configDebugEnabled() {
						println("🔧 DEBUG: 设置链ID(uint64):", chainId)
					}
				} else if chainIdInt, ok := chainIdVal.(int); ok {
					c.options.ChainID = uint64(chainIdInt)
					if configDebugEnabled() {
						println("🔧 DEBUG: 设置链ID(int->uint64):", uint64(chainIdInt))
					}
				} else if chainIdFloat, ok := chainIdVal.(float64); ok {
					// JSON解析中数字通常是float64，需要安全转换为uint64
					if chainIdFloat >= 0 && chainIdFloat == float64(uint64(chainIdFloat)) {
						c.options.ChainID = uint64(chainIdFloat)
						if configDebugEnabled() {
							println("🔧 DEBUG: 设置链ID(float64->uint64):", uint64(chainIdFloat))
						}
					} else {
						if configDebugEnabled() {
							println("🔧 ERROR: 无效的链ID值(float64):", chainIdFloat)
						}
					}
				} else {
					if configDebugEnabled() {
						println("🔧 ERROR: 链ID类型转换失败:", fmt.Sprintf("%T", chainIdVal), "值:", chainIdVal)
					}
				}
			}

			// 🔧 修复：处理网络ID配置
			if networkIdVal, exists := configMap["network_id"]; exists {
				if networkId, ok := networkIdVal.(string); ok {
					// 暂时跳过string类型的network_id，因为BlockchainOptions.NetworkID是uint64
					if configDebugEnabled() {
						println("🔧 DEBUG: 跳过string类型的network_id:", networkId)
					}
				}
			}
			// 处理genesis配置
			if genesisMap, exists := configMap["genesis"]; exists {
				if genesisConfig, ok := genesisMap.(map[string]interface{}); ok {
					// 处理genesis_accounts（兼容 "accounts" 和 "genesis_accounts" 两种字段名）
					var accountsInterface interface{}
					var accountsExists bool
					// 优先使用 "accounts"（新格式）
					if accountsInterface, accountsExists = genesisConfig["accounts"]; !accountsExists {
						// 降级到 "genesis_accounts"（旧格式）
						accountsInterface, accountsExists = genesisConfig["genesis_accounts"]
					}

					if accountsExists {
						if accountsList, ok := accountsInterface.([]interface{}); ok {
							var genesisAccounts []GenesisAccount
							for _, accountInterface := range accountsList {
								if accountMap, ok := accountInterface.(map[string]interface{}); ok {
									account := GenesisAccount{}

									// 解析 public_key
									if pubKey, exists := accountMap["public_key"]; exists {
										if pubKeyStr, ok := pubKey.(string); ok {
											account.PublicKey = pubKeyStr
										}
									}

									// 解析金额：支持 "amount" 或 "initial_balance"
									amountParsed := false

									// 1. 尝试 "initial_balance" (新格式，字符串)
									if initialBalance, exists := accountMap["initial_balance"]; exists {
										if balanceStr, ok := initialBalance.(string); ok {
											if balanceInt, err := strconv.ParseUint(balanceStr, 10, 64); err == nil {
												account.Amount = balanceInt
												amountParsed = true
												if configDebugEnabled() {
													println("🔧 DEBUG: 从initial_balance字符串解析金额:", balanceInt)
												}
											}
										}
									}

									// 2. 降级到 "amount" (旧格式，数值)
									if !amountParsed {
										if amount, exists := accountMap["amount"]; exists {
											if amountFloat, ok := amount.(float64); ok {
												account.Amount = uint64(amountFloat)
												amountParsed = true
												if configDebugEnabled() {
													println("🔧 DEBUG: 从amount数值解析金额:", account.Amount)
												}
											} else if amountStr, ok := amount.(string); ok {
												if amountInt, err := strconv.ParseUint(amountStr, 10, 64); err == nil {
													account.Amount = amountInt
													amountParsed = true
													if configDebugEnabled() {
														println("🔧 DEBUG: 从amount字符串解析金额:", amountInt)
													}
												}
											}
										}
									}

									if account.PublicKey != "" && account.Amount > 0 {
										genesisAccounts = append(genesisAccounts, account)
									}
								}
							}
							if len(genesisAccounts) > 0 {
								// 调试日志：打印解析的创世账户信息
								if configDebugEnabled() {
									println("🔧 DEBUG: 解析了创世账户数:", len(genesisAccounts))
									for i, acc := range genesisAccounts {
										println("🔧 DEBUG: 账户", i, ": PublicKey=", acc.PublicKey, ", Amount=", acc.Amount)
									}
									println("🔧 DEBUG: 覆盖前默认配置账户数:", len(c.options.GenesisConfig.Accounts))
									if len(c.options.GenesisConfig.Accounts) > 0 {
										println("🔧 DEBUG: 覆盖前第一个账户金额:", c.options.GenesisConfig.Accounts[0].Amount)
									}
								}
								c.options.GenesisConfig.Accounts = genesisAccounts
								if configDebugEnabled() {
									println("🔧 DEBUG: 覆盖后账户数:", len(c.options.GenesisConfig.Accounts))
									if len(c.options.GenesisConfig.Accounts) > 0 {
										println("🔧 DEBUG: 覆盖后第一个账户金额:", c.options.GenesisConfig.Accounts[0].Amount)
									}
									println("🔧 DEBUG: 已更新默认配置中的创世账户")
								}
							}
						}
					}
				}
			}

			// 处理block配置
			if blockMap, exists := configMap["block"]; exists {
				if blockConfig, ok := blockMap.(map[string]interface{}); ok {
					if minBlockIntervalVal, exists := blockConfig["min_block_interval"]; exists {
						if minBlockInterval, ok := minBlockIntervalVal.(int); ok {
							c.options.Block.MinBlockInterval = minBlockInterval
						} else if minBlockIntervalFloat, ok := minBlockIntervalVal.(float64); ok {
							c.options.Block.MinBlockInterval = int(minBlockIntervalFloat)
						}
					}
				}
			}
		}
	}
}

// GetOptions 获取完整的区块链配置选项
func (c *Config) GetOptions() *BlockchainOptions {
	return c.options
}

// === 基础配置访问方法 ===

// GetChainID 获取链ID
func (c *Config) GetChainID() uint64 {
	return c.options.ChainID
}

// GetNetworkID 获取网络ID
func (c *Config) GetNetworkID() uint64 {
	return c.options.NetworkID
}

// GetNodeMode 获取默认节点模式（Light/Full）
func (c *Config) GetNodeMode() types.NodeMode {
	return c.options.NodeMode
}

// === 区块域配置访问方法 ===

// GetMaxBlockSize 获取最大区块大小
func (c *Config) GetMaxBlockSize() uint64 {
	return c.options.Block.MaxBlockSize
}

// GetMaxTransactions 获取最大交易数
func (c *Config) GetMaxTransactions() int {
	return c.options.Block.MaxTransactions
}

// GetBlockTimeTarget 获取目标出块时间
func (c *Config) GetBlockTimeTarget() int {
	return c.options.Block.BlockTimeTarget
}

// GetMinDifficulty 获取最小难度
func (c *Config) GetMinDifficulty() uint64 {
	return c.options.Block.MinDifficulty
}

// GetBlockCacheSize 获取区块缓存大小
func (c *Config) GetBlockCacheSize() int {
	return c.options.Block.CacheSize
}

// === 交易域配置访问方法 ===

// GetMaxTransactionSize 获取最大交易大小
func (c *Config) GetMaxTransactionSize() uint64 {
	return c.options.Transaction.MaxTransactionSize
}

// GetBaseFeePerByte 获取基础字节费率
func (c *Config) GetBaseFeePerByte() uint64 {
	return c.options.Transaction.BaseFeePerByte
}

// GetMinimumFee 获取最低费用
func (c *Config) GetMinimumFee() uint64 {
	return c.options.Transaction.MinimumFee
}

// GetBaseExecutionFeePrice 获取基础执行费用价格
func (c *Config) GetBaseExecutionFeePrice() uint64 {
	return c.options.Transaction.BaseExecutionFeePrice
}

// GetTransactionCacheSize 获取交易缓存大小
func (c *Config) GetTransactionCacheSize() int {
	return c.options.Transaction.CacheSize
}

// GetMaxBatchTransferSize 获取批量转账最大笔数
func (c *Config) GetMaxBatchTransferSize() int {
	return c.options.Transaction.MaxBatchTransferSize
}

// === 费用相关配置访问方法===

// GetDustThreshold 获取粉尘阈值
//
// 🎯 **用途**：UTXO选择算法中判断是否创建找零输出的门限值
//
// 💡 **设计说明**：
// - 如果找零金额 < 粉尘阈值，则不创建找零输出，避免粉尘攻击
// - 默认值：0.00001 个原生币
// - 与 internal/core/blockchain/transaction/internal/utxo_selector.go 中的逻辑对应
func (c *Config) GetDustThreshold() float64 {
	return c.options.Transaction.DustThreshold
}

// GetBaseFeeRate 获取基础费率参考值
//
// 🎯 **用途**：某些费用计算场景的参考值，不强制使用
//
// 💡 **设计说明**：
// - 仅作为计算参考，实际费用机制由 transaction.proto 的 fee_mechanism 决定
// - 默认值：0.0003（万三费率）
// - 95%的交易使用默认UTXO差额机制：费用 = Σ(输入) - Σ(输出)
func (c *Config) GetBaseFeeRate() float64 {
	return c.options.Transaction.BaseFeeRate
}

// === 同步域配置访问方法 ===

// GetSyncBatchSize 获取同步批次大小
func (c *Config) GetSyncBatchSize() int {
	return c.options.Sync.BatchSize
}

// GetSyncConcurrency 获取同步并发度
func (c *Config) GetSyncConcurrency() int {
	return c.options.Sync.Concurrency
}

// GetSyncTimeout 获取同步超时
func (c *Config) GetSyncTimeout() time.Duration {
	return c.options.Sync.Timeout
}

// GetMaxReorgDepth 获取最大重组深度
func (c *Config) GetMaxReorgDepth() int {
	return c.options.Sync.MaxReorgDepth
}

// === K桶智能同步配置访问方法 ===

// GetSyncAdvancedConfig 获取高级同步配置
func (c *Config) GetSyncAdvancedConfig() SyncAdvancedConfig {
	return c.options.Sync.Advanced
}

// GetKBucketSelectionCount 获取K桶节点选择数量
func (c *Config) GetKBucketSelectionCount() int {
	return c.options.Sync.Advanced.KBucketSelectionCount
}

// GetKBucketSelectionStrategy 获取K桶节点选择策略
func (c *Config) GetKBucketSelectionStrategy() string {
	return c.options.Sync.Advanced.KBucketSelectionStrategy
}

// GetNodeSelectionTimeout 获取节点选择超时
func (c *Config) GetNodeSelectionTimeout() time.Duration {
	return c.options.Sync.Advanced.NodeSelectionTimeout
}

// GetMaxConcurrentRequests 获取最大并发请求数
func (c *Config) GetMaxConcurrentRequests() int {
	return c.options.Sync.Advanced.MaxConcurrentRequests
}

// GetMaxResponseSizeBytes 获取网络响应大小限制
func (c *Config) GetMaxResponseSizeBytes() uint32 {
	return c.options.Sync.Advanced.MaxResponseSizeBytes
}

// GetMaxBlocksPerRequest 获取每次请求最大区块数
func (c *Config) GetMaxBlocksPerRequest() int {
	return c.options.Sync.Advanced.MaxBlocksPerRequest
}

// GetIntelligentPagingThreshold 获取智能分页阈值
func (c *Config) GetIntelligentPagingThreshold() uint32 {
	return c.options.Sync.Advanced.IntelligentPagingThreshold
}

// IsTimeCheckEnabled 是否启用时间检查触发
func (c *Config) IsTimeCheckEnabled() bool {
	return c.options.Sync.Advanced.TimeCheckEnabled
}

// GetTimeCheckThresholdMins 获取时间检查阈值分钟数
func (c *Config) GetTimeCheckThresholdMins() int {
	return c.options.Sync.Advanced.TimeCheckThresholdMins
}

// GetTimeCheckIntervalMins 获取时间检查间隔分钟数
func (c *Config) GetTimeCheckIntervalMins() int {
	return c.options.Sync.Advanced.TimeCheckIntervalMins
}

// GetRetryBackoffIntervals 获取重试间隔序列
func (c *Config) GetRetryBackoffIntervals() []time.Duration {
	return c.options.Sync.Advanced.RetryBackoffIntervals
}

// GetMaxRetryAttempts 获取最大重试次数
func (c *Config) GetMaxRetryAttempts() int {
	return c.options.Sync.Advanced.MaxRetryAttempts
}

// IsAsyncProcessingEnabled 是否启用异步处理
func (c *Config) IsAsyncProcessingEnabled() bool {
	return c.options.Sync.Advanced.EnableAsyncProcessing
}

// === K桶批量处理配置访问方法 ===

// GetMaxBatchSize 获取K桶批量处理最大批次大小
func (c *Config) GetMaxBatchSize() int {
	return c.options.Sync.Advanced.MaxBatchSize
}

// GetMaxConcurrentBlockValidationWorkers 获取最大并发区块验证工作协程数
func (c *Config) GetMaxConcurrentBlockValidationWorkers() int {
	return c.options.Sync.Advanced.MaxConcurrentBlockValidationWorkers
}

// GetDefaultBatchProcessingTimeoutSeconds 获取默认批量处理超时秒数
func (c *Config) GetDefaultBatchProcessingTimeoutSeconds() int {
	return c.options.Sync.Advanced.DefaultBatchProcessingTimeoutSeconds
}

// IsIntelligentBatchSizingEnabled 是否启用智能批次大小调整
func (c *Config) IsIntelligentBatchSizingEnabled() bool {
	return c.options.Sync.Advanced.EnableIntelligentBatchSizing
}

// GetBatchProcessingMemoryLimitMB 获取批量处理内存限制MB
func (c *Config) GetBatchProcessingMemoryLimitMB() int {
	return c.options.Sync.Advanced.BatchProcessingMemoryLimitMB
}

// GetBatchErrorToleranceLevel 获取批量处理错误容忍度级别
func (c *Config) GetBatchErrorToleranceLevel() int {
	return c.options.Sync.Advanced.BatchErrorToleranceLevel
}

// IsBatchPipelineProcessingEnabled 是否启用批量流水线处理
func (c *Config) IsBatchPipelineProcessingEnabled() bool {
	return c.options.Sync.Advanced.EnableBatchPipelineProcessing
}

// GetBatchValidationMode 获取批量验证模式
func (c *Config) GetBatchValidationMode() int {
	return c.options.Sync.Advanced.BatchValidationMode
}

// === UTXO域配置访问方法 ===

// IsPruningEnabled 是否启用状态修剪
func (c *Config) IsPruningEnabled() bool {
	return c.options.UTXO.PruningEnabled
}

// GetStateRetentionBlocks 获取状态保留区块数
func (c *Config) GetStateRetentionBlocks() int {
	return c.options.UTXO.StateRetentionBlocks
}

// GetStateCacheSize 获取状态缓存大小
func (c *Config) GetStateCacheSize() int {
	return c.options.UTXO.CacheSize
}

// === 执行域配置访问方法 ===

// IsVMEnabled 是否启用虚拟机
func (c *Config) IsVMEnabled() bool {
	return c.options.Execution.VMEnabled
}

// GetExecutionFeeLimit 获取执行费用限制
func (c *Config) GetExecutionFeeLimit() uint64 {
	return c.options.Execution.ExecutionFeeLimit
}

// GetCallStackLimit 获取调用栈限制
func (c *Config) GetCallStackLimit() int {
	return c.options.Execution.CallStackLimit
}

// === 向后兼容的配置访问方法（临时保留，支持startup模块）===

// GetGenesisConfig 获取创世配置
func (c *Config) GetGenesisConfig() GenesisConfig {
	return c.options.GenesisConfig
}

// GetNetworkType 获取网络类型
func (c *Config) GetNetworkType() string {
	return c.options.NetworkType
}

// GetGenesisTimestamp 获取创世时间戳
func (c *Config) GetGenesisTimestamp() int64 {
	return c.options.GenesisTimestamp
}

// GetMaxTransactionsPerBlock 获取每个区块最大交易数（兼容方法）
func (c *Config) GetMaxTransactionsPerBlock() int {
	return c.options.Block.MaxTransactions
}

// ============================================================================
//                          创世配置访问接口
// ============================================================================

// GetUnifiedGenesisConfig 获取统一格式的创世配置
//
// 🎯 **统一创世配置获取器**
//
// 优先返回通过provider从文件加载的外部创世配置，
// 如果没有则返回基于内部配置的默认创世配置。
//
// 返回：
//   - *types.GenesisConfig: 统一的创世配置，永不为nil
func (c *Config) GetUnifiedGenesisConfig() *types.GenesisConfig {
	// 🔧 修复：直接优先使用外部创世配置（genesis.json）
	if c.externalGenesisConfig != nil {
		if configDebugEnabled() {
			println("🔧 UNIFIED DEBUG: 使用外部配置, 账户数:", len(c.externalGenesisConfig.GenesisAccounts))
			if len(c.externalGenesisConfig.GenesisAccounts) > 0 {
				println("🔧 UNIFIED DEBUG: 外部配置第一个账户金额:", c.externalGenesisConfig.GenesisAccounts[0].InitialBalance)
			}
		}
		return c.externalGenesisConfig
	}

	// 如果没有外部配置，使用内部配置转换为统一格式
	if configDebugEnabled() {
		println("🔧 UNIFIED DEBUG: 使用内部配置转换")
	}
	internalConfig := c.convertInternalGenesisConfig()
	if configDebugEnabled() && len(internalConfig.GenesisAccounts) > 0 {
		println("🔧 UNIFIED DEBUG: 内部配置第一个账户金额:", internalConfig.GenesisAccounts[0].InitialBalance)
	}
	return internalConfig
}

// convertInternalGenesisConfig 将内部GenesisConfig转换为统一格式
func (c *Config) convertInternalGenesisConfig() *types.GenesisConfig {
	internalConfig := c.options.GenesisConfig

	// 验证创世时间戳必须已配置
	if c.options.GenesisTimestamp == 0 {
		panic("配置错误：GenesisTimestamp 必须指定，不能为0。创世区块时间戳必须是固定值，确保所有节点创建相同的创世区块")
	}

	unifiedConfig := &types.GenesisConfig{
		NetworkID: c.options.NetworkType,
		ChainID:   c.options.ChainID,
		Timestamp: c.options.GenesisTimestamp,
	}

	// 转换创世账户
	for _, internalAccount := range internalConfig.Accounts {
		account := types.GenesisAccount{
			PublicKey:      internalAccount.PublicKey,
			InitialBalance: fmt.Sprintf("%d", internalAccount.Amount),
		}
		unifiedConfig.GenesisAccounts = append(unifiedConfig.GenesisAccounts, account)
	}

	return unifiedConfig
}
