// Package repository 提供资源管理仓库配置管理功能
//
// 🎯 **资源管理仓库配置核心模块 (Repository Management Configuration Core)**
//
// 本文件定义了WES资源管理系统的完整配置选项，包括：
// - 垃圾回收策略配置
// - 查询操作限制配置
// - 一致性检查配置
// - 性能优化参数配置
//
// 通过统一的配置管理，替代硬编码的配置值，提升系统的可维护性和灵活性。
package repository

import (
	"os"
	"time"
)

// ============================================================================
//                           📊 配置选项结构定义
// ============================================================================

// RepositoryOptions 资源管理仓库配置选项
//
// 🎯 **配置模块化设计**：
// - GarbageCollection: 垃圾回收策略配置
// - QueryLimits: 查询操作限制配置
// - Consistency: 一致性检查配置
// - Performance: 性能优化配置
type RepositoryOptions struct {
	// 垃圾回收配置
	GarbageCollection GarbageCollectionConfig `json:"garbage_collection"`

	// 查询限制配置
	QueryLimits QueryLimitsConfig `json:"query_limits"`

	// 一致性检查配置
	Consistency ConsistencyConfig `json:"consistency"`

	// 性能优化配置
	Performance PerformanceConfig `json:"performance"`

	// Outbox模式配置
	Outbox OutboxConfig `json:"outbox"`
}

// GarbageCollectionConfig 垃圾回收配置
//
// 🧹 **垃圾回收策略配置 (Garbage Collection Strategy Configuration)**
//
// 控制资源自动清理的各项参数，确保系统存储空间的有效利用。
type GarbageCollectionConfig struct {
	// 基础清理参数
	DefaultBatchSize int `json:"default_batch_size"` // 默认单次清理数量
	MaxBatchSize     int `json:"max_batch_size"`     // 最大单次清理数量限制

	// 清理触发条件
	AutoTriggerEnabled       bool          `json:"auto_trigger_enabled"`       // 是否启用自动触发清理
	TriggerInterval          time.Duration `json:"trigger_interval"`           // 自动触发间隔
	StoragePressureThreshold float64       `json:"storage_pressure_threshold"` // 存储压力阈值(0.0-1.0)

	// 清理策略
	AggressiveMode bool `json:"aggressive_mode"` // 是否启用激进清理模式
	SafeMode       bool `json:"safe_mode"`       // 是否启用安全清理模式（更多验证）
}

// QueryLimitsConfig 查询限制配置
//
// 🔍 **查询操作限制配置 (Query Operation Limits Configuration)**
//
// 控制各种查询操作的限制参数，防止资源滥用和系统过载。
type QueryLimitsConfig struct {
	// 分页查询限制
	DefaultPageSize int `json:"default_page_size"` // 默认分页大小
	MaxPageSize     int `json:"max_page_size"`     // 最大分页大小限制

	// 批量查询限制
	MaxBatchQuerySize int `json:"max_batch_query_size"` // 最大批量查询数量

	// 查询超时配置
	QueryTimeout        time.Duration `json:"query_timeout"`         // 查询超时时间
	ComplexQueryTimeout time.Duration `json:"complex_query_timeout"` // 复杂查询超时时间

	// 缓存配置
	EnableQueryCache bool          `json:"enable_query_cache"` // 是否启用查询缓存
	CacheTTL         time.Duration `json:"cache_ttl"`          // 缓存生存时间
}

// ConsistencyConfig 一致性检查配置
//
// 🔧 **一致性检查配置 (Consistency Check Configuration)**
//
// 控制系统自愈机制的各项参数，确保数据的长期完整性。
type ConsistencyConfig struct {
	// 检查调度
	AutoCheckEnabled bool          `json:"auto_check_enabled"` // 是否启用自动一致性检查
	CheckInterval    time.Duration `json:"check_interval"`     // 检查间隔
	DeepCheckEnabled bool          `json:"deep_check_enabled"` // 是否启用深度检查

	// 修复策略
	AutoRepairEnabled bool `json:"auto_repair_enabled"` // 是否启用自动修复
	RepairBatchSize   int  `json:"repair_batch_size"`   // 修复批处理大小

	// 健康状态管理
	HealthStatusTTL time.Duration `json:"health_status_ttl"` // 健康状态缓存时间
}

// PerformanceConfig 性能优化配置
//
// ⚡ **性能优化配置 (Performance Optimization Configuration)**
//
// 控制系统性能相关的各项参数，优化资源使用和响应速度。
type PerformanceConfig struct {
	// 索引优化
	EnableIndexV2  bool `json:"enable_index_v2"`  // 是否启用v2索引（并发优化）
	IndexCacheSize int  `json:"index_cache_size"` // 索引缓存大小
	IndexBatchSize int  `json:"index_batch_size"` // 索引批处理大小

	// 流式处理
	EnableStreaming        bool  `json:"enable_streaming"`          // 是否启用流式处理
	StreamBufferSize       int   `json:"stream_buffer_size"`        // 流式缓冲区大小
	LargeFileSizeThreshold int64 `json:"large_file_size_threshold"` // 大文件阈值（字节）

	// 并发控制
	MaxConcurrentOps int `json:"max_concurrent_ops"` // 最大并发操作数
	WorkerPoolSize   int `json:"worker_pool_size"`   // 工作池大小

	// 性能监控
	PerformanceHistorySize int `json:"performance_history_size"` // 性能指标历史记录大小
	ConsistencyCheckRange  int `json:"consistency_check_range"`  // 一致性检查范围
	MaxBlockRangeSize      int `json:"max_block_range_size"`     // 单次查询的最大区块数量
}

// OutboxConfig Outbox模式配置
//
// 📦 **Outbox模式配置 (Outbox Pattern Configuration)**
//
// 控制Outbox模式的重试、处理和清理策略，确保事件的可靠投递。
type OutboxConfig struct {
	// 重试机制
	MaxRetries int           `json:"max_retries"` // 最大重试次数
	RetryDelay time.Duration `json:"retry_delay"` // 重试延迟

	// 处理器配置
	ProcessorInterval time.Duration `json:"processor_interval"` // 处理器运行间隔
	BatchSize         int           `json:"batch_size"`         // 批量处理事件数量

	// 清理配置
	CleanupInterval time.Duration `json:"cleanup_interval"` // 清理已完成事件的间隔
	EventRetention  time.Duration `json:"event_retention"`  // 事件保留时间
}

// ============================================================================
//                           🏗️ 配置实现类
// ============================================================================

// Config 资源管理仓库配置实现
//
// 🎯 **配置管理器 (Configuration Manager)**
//
// 负责管理资源仓库的完整配置，提供统一的配置访问接口。
type Config struct {
	options *RepositoryOptions
}

// New 创建资源管理仓库配置实例
//
// 🏗️ **配置构造函数 (Configuration Constructor)**
//
// 根据用户提供的配置创建配置实例，如果没有用户配置则使用默认值。
//
// 📝 **参数说明**：
//   - userConfig: 用户配置（可以是 *types.UserRepositoryConfig 或 interface{}）
//
// 🔄 **处理流程**：
//  1. 创建默认配置选项
//  2. 如果有用户配置，进行类型转换和配置合并
//  3. 验证配置的合理性
//  4. 返回最终的配置实例
func New(userConfig interface{}) *Config {
	// 1. 创建默认配置
	defaultOptions := createDefaultRepositoryOptions()

	// 2. 处理用户配置
	if userConfig != nil {
		// Repository配置已内部化，不接受用户配置，直接使用默认值
		// 如果将来需要用户配置，可以添加对应的JSON字段到types.AppConfig中
	}

	// 3. 验证和调整配置
	validateAndAdjustConfig(defaultOptions)

	return &Config{
		options: defaultOptions,
	}
}

// GetOptions 获取配置选项
//
// 📊 **配置选项访问器 (Configuration Options Accessor)**
//
// 返回当前的配置选项，供其他模块使用。
func (c *Config) GetOptions() *RepositoryOptions {
	return c.options
}

// ============================================================================
//                           ⚙️ 配置处理辅助函数
// ============================================================================

// createDefaultRepositoryOptions 创建默认的资源管理仓库配置
//
// 🔧 **默认配置生成器 (Default Configuration Generator)**
//
// 根据系统最佳实践和性能测试结果，生成优化的默认配置。
func createDefaultRepositoryOptions() *RepositoryOptions {
	return &RepositoryOptions{
		GarbageCollection: GarbageCollectionConfig{
			DefaultBatchSize:         DefaultGCBatchSize,
			MaxBatchSize:             MaxGCBatchSize,
			AutoTriggerEnabled:       true,
			TriggerInterval:          DefaultGCTriggerInterval,
			StoragePressureThreshold: DefaultStoragePressureThreshold,
			AggressiveMode:           DefaultGCAggressiveMode,
			SafeMode:                 DefaultGCSafeMode,
		},
		QueryLimits: QueryLimitsConfig{
			DefaultPageSize:     DefaultQueryPageSize,
			MaxPageSize:         MaxQueryPageSize,
			MaxBatchQuerySize:   MaxBatchQuerySize,
			QueryTimeout:        DefaultQueryTimeout,
			ComplexQueryTimeout: DefaultComplexQueryTimeout,
			EnableQueryCache:    DefaultEnableQueryCache,
			CacheTTL:            DefaultQueryCacheTTL,
		},
		Consistency: ConsistencyConfig{
			AutoCheckEnabled:  DefaultConsistencyCheckEnabled,
			CheckInterval:     DefaultConsistencyCheckInterval,
			DeepCheckEnabled:  DefaultDeepCheckEnabled,
			AutoRepairEnabled: DefaultAutoRepairEnabled,
			RepairBatchSize:   DefaultRepairBatchSize,
			HealthStatusTTL:   DefaultHealthStatusTTL,
		},
		Performance: PerformanceConfig{
			EnableIndexV2:          DefaultEnableIndexV2,
			IndexCacheSize:         DefaultIndexCacheSize,
			IndexBatchSize:         DefaultIndexBatchSize,
			EnableStreaming:        DefaultEnableStreaming,
			StreamBufferSize:       DefaultStreamBufferSize,
			LargeFileSizeThreshold: DefaultLargeFileSizeThreshold,
			MaxConcurrentOps:       DefaultMaxConcurrentOps,
			WorkerPoolSize:         DefaultWorkerPoolSize,
			PerformanceHistorySize: DefaultPerformanceHistorySize,
			ConsistencyCheckRange:  DefaultConsistencyCheckRange,
			MaxBlockRangeSize:      DefaultMaxBlockRangeSize,
		},
		Outbox: OutboxConfig{
			MaxRetries:        DefaultOutboxMaxRetries,
			RetryDelay:        DefaultOutboxRetryDelay,
			ProcessorInterval: DefaultOutboxProcessorInterval,
			BatchSize:         DefaultOutboxBatchSize,
			CleanupInterval:   DefaultOutboxCleanupInterval,
			EventRetention:    DefaultOutboxEventRetention,
		},
	}
}

// mergeUserConfig 合并用户配置
//
// 注意：mergeUserConfig 函数已删除
// Repository配置现在完全内部化，不接受用户配置
// 如果将来需要用户配置，应该在types.AppConfig中添加对应字段

// 🔄 **映射配置合并器 (Map Configuration Merger)** - 保留用于内部扩展
//
// 将map[string]interface{}格式的配置合并到默认配置中。
// 此函数保留用于内部扩展，但Repository配置现在完全内部化

// mergeMapConfig 合并Map格式的用户配置
//
// 🗂️ **Map配置合并器 (Map Configuration Merger)**
//
// 处理从配置文件加载的Map格式用户配置。
func mergeMapConfig(defaultConfig *RepositoryOptions, configMap map[string]interface{}) {
	// 处理垃圾回收配置
	if gcConfig, ok := configMap["garbage_collection"].(map[string]interface{}); ok {
		mergeGCConfig(&defaultConfig.GarbageCollection, gcConfig)
	}

	// 处理查询限制配置
	if queryConfig, ok := configMap["query_limits"].(map[string]interface{}); ok {
		mergeQueryLimitsConfig(&defaultConfig.QueryLimits, queryConfig)
	}

	// 处理一致性配置
	if consistencyConfig, ok := configMap["consistency"].(map[string]interface{}); ok {
		mergeConsistencyConfig(&defaultConfig.Consistency, consistencyConfig)
	}

	// 处理性能配置
	if perfConfig, ok := configMap["performance"].(map[string]interface{}); ok {
		mergePerformanceConfig(&defaultConfig.Performance, perfConfig)
	}
}

// mergeGCConfig 合并垃圾回收配置
func mergeGCConfig(defaultGC *GarbageCollectionConfig, userGC map[string]interface{}) {
	if val, ok := userGC["default_batch_size"].(float64); ok {
		defaultGC.DefaultBatchSize = int(val)
	}
	if val, ok := userGC["max_batch_size"].(float64); ok {
		defaultGC.MaxBatchSize = int(val)
	}
	if val, ok := userGC["auto_trigger_enabled"].(bool); ok {
		defaultGC.AutoTriggerEnabled = val
	}
	// TODO: 添加更多字段的处理
}

// mergeQueryLimitsConfig 合并查询限制配置
func mergeQueryLimitsConfig(defaultQuery *QueryLimitsConfig, userQuery map[string]interface{}) {
	if val, ok := userQuery["default_page_size"].(float64); ok {
		defaultQuery.DefaultPageSize = int(val)
	}
	if val, ok := userQuery["max_page_size"].(float64); ok {
		defaultQuery.MaxPageSize = int(val)
	}
	if val, ok := userQuery["max_batch_query_size"].(float64); ok {
		defaultQuery.MaxBatchQuerySize = int(val)
	}
	// TODO: 添加更多字段的处理
}

// mergeConsistencyConfig 合并一致性配置
func mergeConsistencyConfig(defaultConsistency *ConsistencyConfig, userConsistency map[string]interface{}) {
	if val, ok := userConsistency["auto_check_enabled"].(bool); ok {
		defaultConsistency.AutoCheckEnabled = val
	}
	if val, ok := userConsistency["auto_repair_enabled"].(bool); ok {
		defaultConsistency.AutoRepairEnabled = val
	}
	// TODO: 添加更多字段的处理
}

// mergePerformanceConfig 合并性能配置
func mergePerformanceConfig(defaultPerf *PerformanceConfig, userPerf map[string]interface{}) {
	if val, ok := userPerf["enable_index_v2"].(bool); ok {
		defaultPerf.EnableIndexV2 = val
	}
	if val, ok := userPerf["enable_streaming"].(bool); ok {
		defaultPerf.EnableStreaming = val
	}
	if val, ok := userPerf["max_concurrent_ops"].(float64); ok {
		defaultPerf.MaxConcurrentOps = int(val)
	}
	// TODO: 添加更多字段的处理
}

// validateAndAdjustConfig 验证并调整配置
//
// 🔧 **配置验证器 (Configuration Validator)**
//
// 确保配置值在合理范围内，并进行必要的调整。
func validateAndAdjustConfig(config *RepositoryOptions) {
	// 验证垃圾回收配置
	if config.GarbageCollection.DefaultBatchSize <= 0 {
		config.GarbageCollection.DefaultBatchSize = 50
	}
	if config.GarbageCollection.MaxBatchSize < config.GarbageCollection.DefaultBatchSize {
		config.GarbageCollection.MaxBatchSize = config.GarbageCollection.DefaultBatchSize * 20
	}

	// 验证查询限制配置
	if config.QueryLimits.DefaultPageSize <= 0 {
		config.QueryLimits.DefaultPageSize = 50
	}
	if config.QueryLimits.MaxPageSize < config.QueryLimits.DefaultPageSize {
		config.QueryLimits.MaxPageSize = config.QueryLimits.DefaultPageSize * 20
	}

	// 验证一致性配置
	if config.Consistency.RepairBatchSize <= 0 {
		config.Consistency.RepairBatchSize = 20
	}

	// 验证性能配置
	if config.Performance.MaxConcurrentOps <= 0 {
		config.Performance.MaxConcurrentOps = 50
	}
	if config.Performance.WorkerPoolSize <= 0 {
		config.Performance.WorkerPoolSize = 10
	}
	if config.Performance.PerformanceHistorySize <= 0 {
		config.Performance.PerformanceHistorySize = 100
	}
	if config.Performance.ConsistencyCheckRange <= 0 {
		config.Performance.ConsistencyCheckRange = 100
	}
	if config.Performance.MaxBlockRangeSize <= 0 {
		config.Performance.MaxBlockRangeSize = 10000
	}

	// 验证Outbox配置
	if config.Outbox.MaxRetries <= 0 {
		config.Outbox.MaxRetries = 3
	}
	if config.Outbox.RetryDelay <= 0 {
		config.Outbox.RetryDelay = time.Second * 2
	}
	if config.Outbox.ProcessorInterval <= 0 {
		config.Outbox.ProcessorInterval = time.Second * 30
	}
	if config.Outbox.BatchSize <= 0 {
		config.Outbox.BatchSize = 50
	}
	if config.Outbox.CleanupInterval <= 0 {
		config.Outbox.CleanupInterval = time.Hour * 24
	}
	if config.Outbox.EventRetention <= 0 {
		config.Outbox.EventRetention = time.Hour * 72
	}

	if os.Getenv("WES_CLI_MODE") != "true" {
		println("🔧 REPOSITORY CONFIG DEBUG: 配置验证完成")
	}
}
