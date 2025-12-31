// Package candidatepool provides default configuration values for candidate pool.
package candidatepool

import "time"

// 候选池配置默认值
const (
	// === 基础池配置 ===

	// defaultMaxCandidates 默认候选区块池最大容量设为100
	// 原因：100个候选区块能够覆盖多个共识轮次的候选区块
	defaultMaxCandidates = 100

	// defaultMaxAge 默认候选区块最大生存时间设为10分钟
	// 原因：10分钟能覆盖大多数共识周期，过期的候选区块不再有价值
	defaultMaxAge = 10 * time.Minute

	// defaultMemoryLimit 默认内存使用限制设为256MB
	// 原因：256MB能存储大量候选区块，平衡内存使用和容量需求
	defaultMemoryLimit = 256 * 1024 * 1024

	// === 清理和维护配置 ===

	// defaultCleanupInterval 默认清理任务执行间隔设为1分钟
	// 原因：1分钟的清理间隔能及时移除过期候选区块
	defaultCleanupInterval = 1 * time.Minute

	// defaultMemoryWarningThreshold 默认内存预警阈值设为80%
	// 原因：80%的阈值提供足够的缓冲空间，避免内存耗尽
	defaultMemoryWarningThreshold = 0.8

	// defaultGCInterval 默认垃圾回收间隔设为5分钟
	// 原因：5分钟的GC间隔平衡内存回收和性能影响
	defaultGCInterval = 5 * time.Minute

	// === 高度清理配置 ===

	// defaultHeightCleanupEnabled 默认启用基于高度的清理
	// 原因：基于高度的清理能及时清理过时的候选区块
	defaultHeightCleanupEnabled = true

	// defaultKeepHeightDepth 默认保留3个高度深度的候选区块
	// 原因：保留3个高度的候选区块能应对短期分叉情况
	defaultKeepHeightDepth = uint64(3)

	// defaultAggressiveCleanup 默认启用激进清理
	// 原因：池满时激进清理能快速释放空间
	defaultAggressiveCleanup = true

	// === 验证和处理配置 ===

	// defaultVerificationTimeout 默认验证超时时间设为30秒
	// 原因：30秒足够完成候选区块的验证过程
	defaultVerificationTimeout = 30 * time.Second

	// defaultValidationConcurrency 默认验证并发数设为5
	// 原因：5个并发验证任务能提高验证效率，不会过度消耗CPU
	defaultValidationConcurrency = 5

	// defaultMaxValidationQueue 默认最大验证队列大小设为50
	// 原因：50个验证任务的队列能应对突发的验证需求
	defaultMaxValidationQueue = 50

	// === 优先级和排序配置 ===

	// defaultPriorityEnabled 默认启用优先级排序
	// 原因：优先级排序确保高质量候选区块优先处理
	defaultPriorityEnabled = true

	// defaultMaxBlockSize 默认最大区块大小限制设为2MB
	// 原因：2MB的区块大小适合大多数交易负载
	defaultMaxBlockSize = 2 * 1024 * 1024

	// defaultMinBlockSize 默认最小区块大小设为1KB
	// 原因：1KB确保区块包含必要的基础信息
	defaultMinBlockSize = 1024

	// === 聚合和打包配置 ===

	// defaultAggregationTimeout 默认聚合等待超时设为5秒
	// 原因：5秒的聚合时间平衡了区块打包效率和延迟
	defaultAggregationTimeout = 5 * time.Second

	// defaultMaxTransactionsPerBlock 默认每个区块最大交易数设为1000
	// 原因：1000个交易是合理的区块容量，平衡吞吐量和验证时间
	defaultMaxTransactionsPerBlock = 1000

	// defaultMinTransactionsPerBlock 默认每个区块最小交易数设为1
	// 原因：至少包含1个交易确保区块的实用性
	defaultMinTransactionsPerBlock = 1

	// === 性能和监控配置 ===

	// defaultMetricsEnabled 默认启用性能指标收集
	// 原因：性能指标对于候选池优化和问题诊断很重要
	defaultMetricsEnabled = true

	// defaultMetricsInterval 默认指标收集间隔设为30秒
	// 原因：30秒间隔提供足够的监控精度
	defaultMetricsInterval = 30 * time.Second

	// defaultStatisticsRetention 默认统计数据保留时间设为1小时
	// 原因：1小时的历史数据足够进行性能分析
	defaultStatisticsRetention = 1 * time.Hour

	// === 缓存和索引配置 ===

	// defaultIndexCacheSize 默认索引缓存大小设为1000
	// 原因：1000个索引条目能快速定位候选区块
	defaultIndexCacheSize = 1000

	// defaultBloomFilterSize 默认布隆过滤器大小设为10000
	// 原因：10000个元素的布隆过滤器能有效减少重复检查
	defaultBloomFilterSize = 10000

	// defaultBloomFilterHashFunctions 默认布隆过滤器哈希函数数量设为3
	// 原因：3个哈希函数在误判率和计算效率之间取得平衡
	defaultBloomFilterHashFunctions = 3

	// === 网络和同步配置 ===

	// defaultSyncTimeout 默认同步超时时间设为60秒
	// 原因：60秒足够完成候选区块的网络同步
	defaultSyncTimeout = 60 * time.Second

	// defaultMaxSyncBatch 默认最大同步批次大小设为20
	// 原因：20个候选区块的批次平衡网络效率和内存使用
	defaultMaxSyncBatch = 20

	// defaultSyncRetryAttempts 默认同步重试次数设为3
	// 原因：3次重试能应对大多数临时网络问题
	defaultSyncRetryAttempts = 3
)

// 默认优先级权重配置
var defaultPriorityWeights = map[string]float64{
	"transaction_count": 0.3, // 交易数量权重
	"transaction_fees":  0.4, // 交易费用权重
	"block_timestamp":   0.2, // 区块时间戳权重
	"miner_reputation":  0.1, // 矿工声誉权重
}

// 默认验证级别配置
var defaultValidationLevels = map[string]bool{
	"header_validation":      true, // 区块头验证
	"transaction_validation": true, // 交易验证
	"signature_validation":   true, // 签名验证
	"merkle_root_validation": true, // 默克尔根验证
	"执行费用_limit_validation":   true, // 执行费用限制验证
}

// 默认清理策略配置
var defaultCleanupStrategies = []string{
	"age_based",       // 基于年龄的清理
	"height_based",    // 基于高度的清理
	"memory_pressure", // 基于内存压力的清理
	"priority_based",  // 基于优先级的清理
	"size_based",      // 基于大小的清理
}

// 默认性能阈值配置
var defaultPerformanceThresholds = map[string]interface{}{
	"max_validation_time_ms": 5000, // 最大验证时间(毫秒)
	"max_insertion_time_ms":  100,  // 最大插入时间(毫秒)
	"max_query_time_ms":      50,   // 最大查询时间(毫秒)
	"memory_usage_warning":   0.8,  // 内存使用警告阈值
	"memory_usage_critical":  0.95, // 内存使用严重阈值
}
