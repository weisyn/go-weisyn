package txpool

import "time"

// 交易池配置默认值
// 这些默认值基于区块链交易池管理的最佳实践和性能考虑
const (
	// === 基础池配置 ===

	// defaultMaxSize 默认交易池最大容量设为10000
	// 原因：10000个交易能满足大多数网络的交易负载
	// 平衡内存使用和交易处理能力，避免交易池过满影响性能
	defaultMaxSize = 10000

	// defaultPriceLimit 默认最低交易价格设为1
	// 原因：设置最低价格防止垃圾交易和拒绝服务攻击
	// 1单位的最低价格是合理的门槛，不会阻碍正常交易
	defaultPriceLimit = 1

	// defaultPriceBump 默认价格提升百分比设为10%
	// 原因：10%的价格提升确保替换交易有足够的经济激励
	// 防止频繁低价替换，保护网络免受垃圾交易干扰
	defaultPriceBump = 10

	// === 生存周期配置 ===

	// defaultLifetime 默认交易生存时间设为1小时
	// 原因：1小时足够交易被打包，过期交易应被清理以释放空间
	// 平衡交易有效性和池空间管理
	defaultLifetime = 1 * time.Hour

	// defaultCleanupInterval 默认清理间隔设为5分钟
	// 原因：5分钟的清理间隔及时移除过期交易，保持池的健康状态
	// 平衡清理频率和系统开销
	defaultCleanupInterval = 5 * time.Minute

	// === 队列和优先级配置 ===

	// defaultMaxQueued 默认最大排队交易数设为1000
	// 原因：1000个排队交易为未来的打包提供缓冲
	// 控制排队规模，避免内存过度使用
	defaultMaxQueued = 1000

	// defaultMaxNonceGap 默认最大nonce间隔设为16
	// 原因：16的间隔允许合理的nonce跳跃，同时防止攻击
	// 平衡用户便利性和安全性
	defaultMaxNonceGap = 16

	// defaultKeepLocals 默认保留本地交易
	// 原因：本地交易通常是用户发起的重要交易，应优先保留
	// 保护用户交易不被网络拥塞时清理
	defaultKeepLocals = true

	// === 验证和安全配置 ===

	// defaultValidationTimeout 默认验证超时时间设为30秒
	// 原因：30秒足够完成交易验证，避免验证过程阻塞池操作
	// 及时发现无效交易，维护池质量
	defaultValidationTimeout = 30 * time.Second

	// defaultMaxAccountQueue 默认每个账户最大排队交易数设为64
	// 原因：64个交易为单个账户提供充足的排队空间
	// 防止单个账户占用过多池资源
	defaultMaxAccountQueue = 64

	// defaultMaxGlobalQueue 默认全局排队交易数设为4096
	// 原因：4096个全局排队交易支持高并发场景
	// 为网络拥塞时提供足够的交易缓冲
	defaultMaxGlobalQueue = 4096

	// === 性能和监控配置 ===

	// defaultMetricsEnabled 默认启用性能指标收集
	// 原因：性能指标对于交易池优化和问题诊断很重要
	// 监控池的健康状态和处理性能
	defaultMetricsEnabled = true

	// defaultMetricsInterval 默认指标收集间隔设为30秒
	// 原因：30秒间隔提供足够的监控精度
	// 平衡监控详细度和系统开销
	defaultMetricsInterval = 30 * time.Second

	// defaultStatisticsRetention 默认统计数据保留时间设为2小时
	// 原因：2小时的历史数据足够进行性能分析和故障排查
	// 限制历史数据量，避免无限增长
	defaultStatisticsRetention = 2 * time.Hour

	// defaultMaxFeeIncreaseRatio 默认最大费用增长倍数设为10
	// 原因：10倍的限制防止极端费用增长，保护用户
	// 避免意外的高费用支付
	defaultMaxFeeIncreaseRatio = 10.0

	// defaultReconstructionWindow 默认重构时间窗口设为10分钟
	// 原因：10分钟窗口内的交易替换被认为是合理的调整
	// 平衡交易确定性和用户灵活性
	defaultReconstructionWindow = 10 * time.Minute

	// defaultMergeTimeout 默认合并超时时间设为30秒
	// 原因：30秒足够完成交易合并操作
	// 避免合并过程过长影响其他操作
	defaultMergeTimeout = 30 * time.Second

	// defaultMaxConcurrentMerges 默认最大并发合并数设为10
	// 原因：10个并发合并任务平衡处理效率和系统负载
	// 避免过多并发操作影响系统稳定性
	defaultMaxConcurrentMerges = 10

	// defaultMaxMergeTransactions 默认最大合并交易数设为100
	// 原因：100个交易的合并批次在效率和复杂度之间取得平衡
	// 控制合并操作的规模，确保操作可管理
	defaultMaxMergeTransactions = 100

	// === 缓存和索引配置 ===

	// defaultCacheSize 默认缓存大小设为5000
	// 原因：5000个交易的缓存提高查询效率
	// 减少重复计算和查找开销
	defaultCacheSize = 5000

	// defaultIndexCacheSize 默认索引缓存大小设为10000
	// 原因：10000个索引条目加速交易定位
	// 提高交易池操作的响应速度
	defaultIndexCacheSize = 10000

	// defaultBloomFilterSize 默认布隆过滤器大小设为20000
	// 原因：20000个元素的布隆过滤器有效减少重复检查
	// 快速判断交易是否已存在
	defaultBloomFilterSize = 20000

	// === 网络和同步配置 ===

	// defaultSyncTimeout 默认同步超时时间设为60秒
	// 原因：60秒足够完成交易池同步操作
	// 避免同步过程过长影响交易处理
	defaultSyncTimeout = 60 * time.Second

	// defaultMaxSyncBatch 默认最大同步批次大小设为500
	// 原因：500个交易的批次平衡网络效率和内存使用
	// 减少网络往返次数，提高同步效率
	defaultMaxSyncBatch = 500

	// defaultSyncRetryAttempts 默认同步重试次数设为3
	// 原因：3次重试能应对大多数临时网络问题
	// 提高同步可靠性，避免过多重试造成延迟
	defaultSyncRetryAttempts = 3

	// === 挖矿配置 ===

	// defaultMaxTransactionsForMining 默认挖矿时最大交易数量设为1000
	// 原因：1000个交易能够充分利用区块空间，同时保证打包效率
	// 平衡区块大小和处理性能，避免区块过于臃肿
	defaultMaxTransactionsForMining = 1000

	// defaultMaxBlockSizeForMining 默认挖矿时区块大小限制设为1MB
	// 原因：1MB区块大小是经过验证的合理大小，平衡网络传输和容量
	// 兼容主流区块链网络的区块大小限制，确保网络兼容性
	defaultMaxBlockSizeForMining = 1024 * 1024 // 1MB

	// === 资源限制配置 ===

	// defaultMemoryLimit 默认内存限制设为512MB
	// 原因：512MB能存储大量交易，同时控制内存使用
	// 防止交易池消耗过多系统内存
	defaultMemoryLimit = 512 * 1024 * 1024

	// defaultMaxTxSize 默认最大交易大小设为128KB
	// 原因：128KB能容纳复杂的智能合约交易
	// 控制单个交易的大小，防止超大交易影响网络
	defaultMaxTxSize = 128 * 1024

	// defaultMaxTotalSize 默认最大总大小设为100MB
	// 原因：100MB的总大小限制确保交易池大小可控
	// 平衡交易容量和系统资源使用
	defaultMaxTotalSize = 100 * 1024 * 1024
)

// 默认交易排序策略
var defaultSortingStrategies = []string{
	"执行费用_price",    // 按执行费用价格排序
	"fee_per_执行费用",  // 按单位执行费用费用排序
	"total_fee",    // 按总费用排序
	"arrival_time", // 按到达时间排序
}

// 默认验证级别配置
var defaultValidationLevels = map[string]bool{
	"signature_verification":    true, // 签名验证
	"nonce_verification":        true, // Nonce验证
	"balance_verification":      true, // 余额验证
	"执行费用_limit_verification":    true, // 执行费用限制验证
	"smart_contract_validation": true, // 智能合约验证
}

// 默认性能阈值配置
var defaultPerformanceThresholds = map[string]interface{}{
	"max_insertion_time_ms":  100,  // 最大插入时间(毫秒)
	"max_removal_time_ms":    50,   // 最大移除时间(毫秒)
	"max_query_time_ms":      10,   // 最大查询时间(毫秒)
	"memory_usage_warning":   0.8,  // 内存使用警告阈值
	"memory_usage_critical":  0.95, // 内存使用严重阈值
	"pool_fullness_warning":  0.8,  // 池满度警告阈值
	"pool_fullness_critical": 0.95, // 池满度严重阈值
}

// 默认费用策略配置
var defaultFeeStrategies = map[string]interface{}{
	"min_tip_cap":         1,       // 最小小费上限
	"max_fee_cap":         1000000, // 最大费用上限
	"base_fee_multiplier": 1.5,     // 基础费用倍数
	"priority_fee_ratio":  0.1,     // 优先费用比例
}
