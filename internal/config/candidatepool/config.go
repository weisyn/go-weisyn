package candidatepool

import "time"

// CandidatePoolOptions 候选池配置选项
// 整个候选池模块的统一配置入口，包含所有候选区块管理相关的配置参数
type CandidatePoolOptions struct {
	// === 基础池配置 ===
	MaxCandidates int           `json:"max_candidates"` // 候选区块池最大容量
	MaxAge        time.Duration `json:"max_age"`        // 候选区块最大生存时间
	MemoryLimit   uint64        `json:"memory_limit"`   // 内存使用限制(字节)

	// === 清理和维护配置 ===
	CleanupInterval        time.Duration `json:"cleanup_interval"`         // 清理任务执行间隔
	MemoryWarningThreshold float64       `json:"memory_warning_threshold"` // 内存预警阈值(0-1)
	GCInterval             time.Duration `json:"gc_interval"`              // 垃圾回收间隔
	
	// === 高度清理配置 ===
	HeightCleanupEnabled bool   `json:"height_cleanup_enabled"` // 是否启用基于高度的清理
	KeepHeightDepth      uint64 `json:"keep_height_depth"`      // 保留多少个高度深度的候选区块
	AggressiveCleanup    bool   `json:"aggressive_cleanup"`     // 池满时是否启用激进清理

	// === 验证和处理配置 ===
	VerificationTimeout   time.Duration `json:"verification_timeout"`   // 验证超时时间
	ValidationConcurrency int           `json:"validation_concurrency"` // 验证并发数
	MaxValidationQueue    int           `json:"max_validation_queue"`   // 最大验证队列大小

	// === 优先级和排序配置 ===
	PriorityEnabled bool   `json:"priority_enabled"` // 是否启用优先级排序
	MaxBlockSize    uint64 `json:"max_block_size"`   // 最大区块大小限制(字节)
	MinBlockSize    uint64 `json:"min_block_size"`   // 最小区块大小限制(字节)

	// === 聚合和打包配置 ===
	AggregationTimeout      time.Duration `json:"aggregation_timeout"`        // 聚合等待超时时间
	MaxTransactionsPerBlock int           `json:"max_transactions_per_block"` // 每个区块最大交易数
	MinTransactionsPerBlock int           `json:"min_transactions_per_block"` // 每个区块最小交易数

	// === 性能和监控配置 ===
	MetricsEnabled      bool          `json:"metrics_enabled"`      // 是否启用性能指标收集
	MetricsInterval     time.Duration `json:"metrics_interval"`     // 指标收集间隔
	StatisticsRetention time.Duration `json:"statistics_retention"` // 统计数据保留时间

	// === 缓存和索引配置 ===
	IndexCacheSize           int `json:"index_cache_size"`            // 索引缓存大小
	BloomFilterSize          int `json:"bloom_filter_size"`           // 布隆过滤器大小
	BloomFilterHashFunctions int `json:"bloom_filter_hash_functions"` // 布隆过滤器哈希函数数量

	// === 网络和同步配置 ===
	SyncTimeout       time.Duration `json:"sync_timeout"`        // 同步超时时间
	MaxSyncBatch      int           `json:"max_sync_batch"`      // 最大同步批次大小
	SyncRetryAttempts int           `json:"sync_retry_attempts"` // 同步重试次数

	// === 内部配置（不对外暴露） ===
	PriorityWeights       map[string]float64     `json:"-"` // 优先级权重配置
	ValidationLevels      map[string]bool        `json:"-"` // 验证级别配置
	CleanupStrategies     []string               `json:"-"` // 清理策略配置
	PerformanceThresholds map[string]interface{} `json:"-"` // 性能阈值配置
}

// Config 候选池配置实现
type Config struct {
	options *CandidatePoolOptions
}

// New 创建候选池配置实现
func New(userConfig interface{}) *Config {
	// 1. 先创建完整的默认配置
	defaultOptions := createDefaultCandidatePoolOptions()

	// 2. 暂时不处理用户配置，后续添加
	// TODO: 当有用户配置类型时，在这里进行转换和合并

	return &Config{
		options: defaultOptions,
	}
}

// createDefaultCandidatePoolOptions 创建默认候选池配置
func createDefaultCandidatePoolOptions() *CandidatePoolOptions {
	return &CandidatePoolOptions{
		// 基础池配置
		MaxCandidates: defaultMaxCandidates,
		MaxAge:        defaultMaxAge,
		MemoryLimit:   defaultMemoryLimit,

		// 清理和维护配置
		CleanupInterval:        defaultCleanupInterval,
		MemoryWarningThreshold: defaultMemoryWarningThreshold,
		GCInterval:             defaultGCInterval,
		
		// 高度清理配置
		HeightCleanupEnabled: defaultHeightCleanupEnabled,
		KeepHeightDepth:      defaultKeepHeightDepth,
		AggressiveCleanup:    defaultAggressiveCleanup,

		// 验证和处理配置
		VerificationTimeout:   defaultVerificationTimeout,
		ValidationConcurrency: defaultValidationConcurrency,
		MaxValidationQueue:    defaultMaxValidationQueue,

		// 优先级和排序配置
		PriorityEnabled: defaultPriorityEnabled,
		MaxBlockSize:    defaultMaxBlockSize,
		MinBlockSize:    defaultMinBlockSize,

		// 聚合和打包配置
		AggregationTimeout:      defaultAggregationTimeout,
		MaxTransactionsPerBlock: defaultMaxTransactionsPerBlock,
		MinTransactionsPerBlock: defaultMinTransactionsPerBlock,

		// 性能和监控配置
		MetricsEnabled:      defaultMetricsEnabled,
		MetricsInterval:     defaultMetricsInterval,
		StatisticsRetention: defaultStatisticsRetention,

		// 缓存和索引配置
		IndexCacheSize:           defaultIndexCacheSize,
		BloomFilterSize:          defaultBloomFilterSize,
		BloomFilterHashFunctions: defaultBloomFilterHashFunctions,

		// 网络和同步配置
		SyncTimeout:       defaultSyncTimeout,
		MaxSyncBatch:      defaultMaxSyncBatch,
		SyncRetryAttempts: defaultSyncRetryAttempts,

		// 内部配置
		PriorityWeights:       copyFloat64Map(defaultPriorityWeights),          // 复制映射
		ValidationLevels:      copyBoolMap(defaultValidationLevels),            // 复制映射
		CleanupStrategies:     append([]string{}, defaultCleanupStrategies...), // 复制切片
		PerformanceThresholds: copyInterfaceMap(defaultPerformanceThresholds),  // 复制映射
	}
}

// copyFloat64Map 复制float64映射
func copyFloat64Map(src map[string]float64) map[string]float64 {
	dst := make(map[string]float64, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// copyBoolMap 复制bool映射
func copyBoolMap(src map[string]bool) map[string]bool {
	dst := make(map[string]bool, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// copyInterfaceMap 复制interface{}映射
func copyInterfaceMap(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// GetOptions 获取完整的候选池配置选项
func (c *Config) GetOptions() *CandidatePoolOptions {
	return c.options
}

// === 基础池配置访问方法 ===

// GetMaxCandidates 获取候选区块池最大容量
func (c *Config) GetMaxCandidates() int {
	return c.options.MaxCandidates
}

// GetMaxAge 获取候选区块最大生存时间
func (c *Config) GetMaxAge() time.Duration {
	return c.options.MaxAge
}

// GetMemoryLimit 获取内存使用限制
func (c *Config) GetMemoryLimit() uint64 {
	return c.options.MemoryLimit
}

// === 清理和维护配置访问方法 ===

// GetCleanupInterval 获取清理任务执行间隔
func (c *Config) GetCleanupInterval() time.Duration {
	return c.options.CleanupInterval
}

// GetMemoryWarningThreshold 获取内存预警阈值
func (c *Config) GetMemoryWarningThreshold() float64 {
	return c.options.MemoryWarningThreshold
}

// GetGCInterval 获取垃圾回收间隔
func (c *Config) GetGCInterval() time.Duration {
	return c.options.GCInterval
}

// === 高度清理配置访问方法 ===

// IsHeightCleanupEnabled 是否启用基于高度的清理
func (c *Config) IsHeightCleanupEnabled() bool {
	return c.options.HeightCleanupEnabled
}

// GetKeepHeightDepth 获取保留的高度深度
func (c *Config) GetKeepHeightDepth() uint64 {
	return c.options.KeepHeightDepth
}

// IsAggressiveCleanupEnabled 是否启用激进清理
func (c *Config) IsAggressiveCleanupEnabled() bool {
	return c.options.AggressiveCleanup
}

// === 验证和处理配置访问方法 ===

// GetVerificationTimeout 获取验证超时时间
func (c *Config) GetVerificationTimeout() time.Duration {
	return c.options.VerificationTimeout
}

// GetValidationConcurrency 获取验证并发数
func (c *Config) GetValidationConcurrency() int {
	return c.options.ValidationConcurrency
}

// GetMaxValidationQueue 获取最大验证队列大小
func (c *Config) GetMaxValidationQueue() int {
	return c.options.MaxValidationQueue
}

// === 优先级和排序配置访问方法 ===

// IsPriorityEnabled 是否启用优先级排序
func (c *Config) IsPriorityEnabled() bool {
	return c.options.PriorityEnabled
}

// GetMaxBlockSize 获取最大区块大小限制
func (c *Config) GetMaxBlockSize() uint64 {
	return c.options.MaxBlockSize
}

// GetMinBlockSize 获取最小区块大小限制
func (c *Config) GetMinBlockSize() uint64 {
	return c.options.MinBlockSize
}

// === 聚合和打包配置访问方法 ===

// GetAggregationTimeout 获取聚合等待超时时间
func (c *Config) GetAggregationTimeout() time.Duration {
	return c.options.AggregationTimeout
}

// GetMaxTransactionsPerBlock 获取每个区块最大交易数
func (c *Config) GetMaxTransactionsPerBlock() int {
	return c.options.MaxTransactionsPerBlock
}

// GetMinTransactionsPerBlock 获取每个区块最小交易数
func (c *Config) GetMinTransactionsPerBlock() int {
	return c.options.MinTransactionsPerBlock
}

// === 性能和监控配置访问方法 ===

// IsMetricsEnabled 是否启用性能指标收集
func (c *Config) IsMetricsEnabled() bool {
	return c.options.MetricsEnabled
}

// GetMetricsInterval 获取指标收集间隔
func (c *Config) GetMetricsInterval() time.Duration {
	return c.options.MetricsInterval
}

// GetStatisticsRetention 获取统计数据保留时间
func (c *Config) GetStatisticsRetention() time.Duration {
	return c.options.StatisticsRetention
}

// === 缓存和索引配置访问方法 ===

// GetIndexCacheSize 获取索引缓存大小
func (c *Config) GetIndexCacheSize() int {
	return c.options.IndexCacheSize
}

// GetBloomFilterSize 获取布隆过滤器大小
func (c *Config) GetBloomFilterSize() int {
	return c.options.BloomFilterSize
}

// GetBloomFilterHashFunctions 获取布隆过滤器哈希函数数量
func (c *Config) GetBloomFilterHashFunctions() int {
	return c.options.BloomFilterHashFunctions
}

// === 网络和同步配置访问方法 ===

// GetSyncTimeout 获取同步超时时间
func (c *Config) GetSyncTimeout() time.Duration {
	return c.options.SyncTimeout
}

// GetMaxSyncBatch 获取最大同步批次大小
func (c *Config) GetMaxSyncBatch() int {
	return c.options.MaxSyncBatch
}

// GetSyncRetryAttempts 获取同步重试次数
func (c *Config) GetSyncRetryAttempts() int {
	return c.options.SyncRetryAttempts
}

// === 优先级权重管理方法 ===

// GetPriorityWeight 获取指定因子的优先级权重
func (c *Config) GetPriorityWeight(factor string) float64 {
	if weight, exists := c.options.PriorityWeights[factor]; exists {
		return weight
	}
	return 0.0
}

// GetAllPriorityWeights 获取所有优先级权重配置
func (c *Config) GetAllPriorityWeights() map[string]float64 {
	return copyFloat64Map(c.options.PriorityWeights) // 返回副本
}

// === 验证级别管理方法 ===

// IsValidationLevelEnabled 检查验证级别是否启用
func (c *Config) IsValidationLevelEnabled(level string) bool {
	if enabled, exists := c.options.ValidationLevels[level]; exists {
		return enabled
	}
	return false
}

// GetAllValidationLevels 获取所有验证级别配置
func (c *Config) GetAllValidationLevels() map[string]bool {
	return copyBoolMap(c.options.ValidationLevels) // 返回副本
}

// === 清理策略管理方法 ===

// GetCleanupStrategies 获取清理策略列表
func (c *Config) GetCleanupStrategies() []string {
	return append([]string{}, c.options.CleanupStrategies...) // 返回副本
}

// IsCleanupStrategyEnabled 检查清理策略是否启用
func (c *Config) IsCleanupStrategyEnabled(strategy string) bool {
	for _, enabledStrategy := range c.options.CleanupStrategies {
		if enabledStrategy == strategy {
			return true
		}
	}
	return false
}

// === 性能阈值管理方法 ===

// GetPerformanceThreshold 获取性能阈值
func (c *Config) GetPerformanceThreshold(metric string) interface{} {
	return c.options.PerformanceThresholds[metric]
}

// GetAllPerformanceThresholds 获取所有性能阈值配置
func (c *Config) GetAllPerformanceThresholds() map[string]interface{} {
	return copyInterfaceMap(c.options.PerformanceThresholds) // 返回副本
}
