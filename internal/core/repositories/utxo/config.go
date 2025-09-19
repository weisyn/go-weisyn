// Package utxo UTXO配置管理实现
//
// ⚙️ **UTXO配置管理器 (UTXO Configuration Manager)**
//
// 本文件实现UTXO模块的配置管理：
// - 配置加载：从项目配置体系加载UTXO相关配置
// - 默认值管理：提供合理的默认配置值
// - 配置验证：验证配置参数的有效性
// - 运行时调整：支持配置的动态调整和热更新
//
// 🎯 **核心功能**
// - 配置集成：与internal/config/blockchain/UTXOConfig完全集成
// - 参数管理：管理UTXO模块的所有配置参数
// - 默认策略：提供生产级的默认配置策略
// - 动态更新：支持配置的运行时调整
//
// 🏗️ **设计原则**
// - 配置统一：与项目配置体系保持一致
// - 默认优先：优先使用配置中的值，回退到默认值
// - 验证严格：严格验证配置参数的合理性
// - 性能考虑：配置读取不影响UTXO操作性能
package utxo

import (
	"fmt"
	"time"

	"github.com/weisyn/v1/internal/config/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ============================================================================
//                              配置管理器定义
// ============================================================================

// ConfigManager UTXO配置管理器
//
// 🎯 **配置管理核心**
//
// 负责管理UTXO模块的所有配置参数，包括缓存配置、性能调优参数、
// 索引配置等。与项目配置体系完全集成，确保配置的一致性和可维护性。
//
// 架构特点：
// - 配置继承：继承blockchain.UTXOConfig的所有配置
// - 扩展配置：添加UTXO模块特有的配置项
// - 验证机制：提供配置参数的有效性验证
// - 热更新：支持部分配置的运行时更新
type ConfigManager struct {
	// 基础配置（从blockchain.UTXOConfig继承）
	StateRetentionBlocks int  `json:"state_retention_blocks"` // 状态保留区块数
	PruningEnabled       bool `json:"pruning_enabled"`        // 是否启用修剪
	PruningInterval      int  `json:"pruning_interval"`       // 修剪间隔（区块数）
	CacheSize            int  `json:"cache_size"`             // 状态缓存数量

	// UTXO特有配置
	MaxConcurrentReferences uint64        `json:"max_concurrent_references"` // 默认最大并发引用数
	CacheTTL                time.Duration `json:"cache_ttl"`                 // 缓存生存时间
	IndexBatchSize          int           `json:"index_batch_size"`          // 索引批量操作大小
	QueryTimeout            time.Duration `json:"query_timeout"`             // 查询超时时间

	// 性能调优配置
	BatchProcessingSize int           `json:"batch_processing_size"` // 批处理大小
	PreloadEnabled      bool          `json:"preload_enabled"`       // 是否启用预加载
	CompactionEnabled   bool          `json:"compaction_enabled"`    // 是否启用压缩
	CompactionInterval  time.Duration `json:"compaction_interval"`   // 压缩间隔

	// 监控配置
	MetricsEnabled            bool          `json:"metrics_enabled"`             // 是否启用监控指标
	MetricsCollectionInterval time.Duration `json:"metrics_collection_interval"` // 监控指标收集间隔

	// 内部状态
	logger log.Logger // 日志服务
}

// ============================================================================
//                              构造函数
// ============================================================================

// NewConfigManager 创建UTXO配置管理器实例
//
// 🏗️ **构造器模式**
//
// 从blockchain.UTXOConfig加载基础配置，并应用UTXO模块特有的配置。
// 提供配置验证和默认值回退机制。
//
// 参数：
//   - utxoConfig: 基础UTXO配置
//   - logger: 日志服务
//
// 返回：
//   - *ConfigManager: 配置管理器实例
//   - error: 创建错误
func NewConfigManager(utxoConfig blockchain.UTXOConfig, logger log.Logger) (*ConfigManager, error) {
	manager := &ConfigManager{
		// 1. 继承基础配置
		StateRetentionBlocks: utxoConfig.StateRetentionBlocks,
		PruningEnabled:       utxoConfig.PruningEnabled,
		PruningInterval:      utxoConfig.PruningInterval,
		CacheSize:            utxoConfig.CacheSize,

		// 2. 设置UTXO特有配置默认值
		MaxConcurrentReferences:   defaultMaxConcurrentReferences,
		CacheTTL:                  defaultCacheTTL,
		IndexBatchSize:            defaultIndexBatchSize,
		QueryTimeout:              defaultQueryTimeout,
		BatchProcessingSize:       defaultBatchProcessingSize,
		PreloadEnabled:            defaultPreloadEnabled,
		CompactionEnabled:         defaultCompactionEnabled,
		CompactionInterval:        defaultCompactionInterval,
		MetricsEnabled:            defaultMetricsEnabled,
		MetricsCollectionInterval: defaultMetricsCollectionInterval,

		// 3. 设置内部依赖
		logger: logger,
	}

	// 验证配置参数
	if err := manager.validateConfig(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	if logger != nil {
		logger.Debug("UTXO配置管理器初始化完成")
		manager.logConfigSummary()
	}

	return manager, nil
}

// ============================================================================
//                           📋 配置默认值定义
// ============================================================================

const (
	// 基础功能默认值
	defaultMaxConcurrentReferences = uint64(100)      // 默认最大并发引用数：100个
	defaultCacheTTL                = 5 * time.Minute  // 默认缓存TTL：5分钟
	defaultIndexBatchSize          = 1000             // 默认索引批量大小：1000个
	defaultQueryTimeout            = 10 * time.Second // 默认查询超时：10秒

	// 性能调优默认值
	defaultBatchProcessingSize = 500           // 默认批处理大小：500个
	defaultPreloadEnabled      = true          // 默认启用预加载
	defaultCompactionEnabled   = true          // 默认启用压缩
	defaultCompactionInterval  = 1 * time.Hour // 默认压缩间隔：1小时

	// 监控默认值
	defaultMetricsEnabled            = false            // 默认禁用监控（遵循项目约束）
	defaultMetricsCollectionInterval = 30 * time.Second // 默认监控收集间隔：30秒
)

// ============================================================================
//                           ✅ 配置验证方法
// ============================================================================

// validateConfig 验证配置参数的有效性
//
// 🎯 **配置验证核心**：
// 对所有配置参数进行有效性验证，确保配置参数在合理范围内。
// 防止因配置错误导致的运行时问题。
//
// 返回：
//   - error: 验证错误，nil表示验证通过
func (cm *ConfigManager) validateConfig() error {
	// 验证基础配置
	if cm.StateRetentionBlocks < 0 {
		return fmt.Errorf("状态保留区块数不能为负数: %d", cm.StateRetentionBlocks)
	}

	if cm.PruningInterval <= 0 {
		return fmt.Errorf("修剪间隔必须为正数: %d", cm.PruningInterval)
	}

	if cm.CacheSize < 0 {
		return fmt.Errorf("缓存大小不能为负数: %d", cm.CacheSize)
	}

	// 验证UTXO特有配置
	if cm.MaxConcurrentReferences == 0 {
		return fmt.Errorf("最大并发引用数不能为0")
	}

	if cm.CacheTTL <= 0 {
		return fmt.Errorf("缓存TTL必须为正数: %v", cm.CacheTTL)
	}

	if cm.IndexBatchSize <= 0 {
		return fmt.Errorf("索引批量大小必须为正数: %d", cm.IndexBatchSize)
	}

	if cm.QueryTimeout <= 0 {
		return fmt.Errorf("查询超时时间必须为正数: %v", cm.QueryTimeout)
	}

	// 验证性能调优配置
	if cm.BatchProcessingSize <= 0 {
		return fmt.Errorf("批处理大小必须为正数: %d", cm.BatchProcessingSize)
	}

	if cm.CompactionInterval <= 0 {
		return fmt.Errorf("压缩间隔必须为正数: %v", cm.CompactionInterval)
	}

	if cm.MetricsCollectionInterval <= 0 {
		return fmt.Errorf("监控收集间隔必须为正数: %v", cm.MetricsCollectionInterval)
	}

	return nil
}

// ============================================================================
//                           📊 配置访问方法
// ============================================================================

// GetCacheConfig 获取缓存相关配置
//
// 🎯 **缓存配置访问**：
// 返回UTXO缓存相关的所有配置参数。
//
// 返回：
//   - CacheConfig: 缓存配置结构
func (cm *ConfigManager) GetCacheConfig() CacheConfig {
	return CacheConfig{
		Size:    cm.CacheSize,
		TTL:     cm.CacheTTL,
		Enabled: cm.CacheSize > 0,
		Preload: cm.PreloadEnabled,
	}
}

// GetIndexConfig 获取索引相关配置
//
// 🎯 **索引配置访问**：
// 返回UTXO索引相关的所有配置参数。
//
// 返回：
//   - IndexConfig: 索引配置结构
func (cm *ConfigManager) GetIndexConfig() IndexConfig {
	return IndexConfig{
		BatchSize:          cm.IndexBatchSize,
		CompactionEnabled:  cm.CompactionEnabled,
		CompactionInterval: cm.CompactionInterval,
	}
}

// GetPerformanceConfig 获取性能相关配置
//
// 🎯 **性能配置访问**：
// 返回UTXO性能调优相关的所有配置参数。
//
// 返回：
//   - PerformanceConfig: 性能配置结构
func (cm *ConfigManager) GetPerformanceConfig() PerformanceConfig {
	return PerformanceConfig{
		BatchProcessingSize:     cm.BatchProcessingSize,
		QueryTimeout:            cm.QueryTimeout,
		MaxConcurrentReferences: cm.MaxConcurrentReferences,
		PruningEnabled:          cm.PruningEnabled,
		PruningInterval:         cm.PruningInterval,
		StateRetentionBlocks:    cm.StateRetentionBlocks,
	}
}

// GetMonitoringConfig 获取监控相关配置
//
// 🎯 **监控配置访问**：
// 返回UTXO监控相关的配置参数。
// 注意：遵循项目约束，监控功能默认禁用。
//
// 返回：
//   - MonitoringConfig: 监控配置结构
func (cm *ConfigManager) GetMonitoringConfig() MonitoringConfig {
	return MonitoringConfig{
		Enabled:            cm.MetricsEnabled,
		CollectionInterval: cm.MetricsCollectionInterval,
	}
}

// ============================================================================
//                           🔧 配置更新方法
// ============================================================================

// UpdateCacheConfig 更新缓存配置
//
// 🎯 **缓存配置热更新**：
// 支持运行时更新缓存相关配置，提供配置的动态调整能力。
//
// 参数：
//   - config: 新的缓存配置
//
// 返回：
//   - error: 更新错误
func (cm *ConfigManager) UpdateCacheConfig(config CacheConfig) error {
	// 验证新配置
	if config.Size < 0 {
		return fmt.Errorf("缓存大小不能为负数: %d", config.Size)
	}
	if config.TTL <= 0 {
		return fmt.Errorf("缓存TTL必须为正数: %v", config.TTL)
	}

	// 更新配置
	cm.CacheSize = config.Size
	cm.CacheTTL = config.TTL
	cm.PreloadEnabled = config.Preload

	if cm.logger != nil {
		cm.logger.Infof("缓存配置已更新 - size: %d, ttl: %v, preload: %t",
			cm.CacheSize, cm.CacheTTL, cm.PreloadEnabled)
	}

	return nil
}

// ============================================================================
//                           📝 配置日志方法
// ============================================================================

// logConfigSummary 记录配置摘要
//
// 🎯 **配置可视化**：
// 将当前配置以友好的格式记录到日志，便于调试和监控。
func (cm *ConfigManager) logConfigSummary() {
	if cm.logger == nil {
		return
	}

	cm.logger.Infof("=== UTXO配置摘要 ===")
	cm.logger.Infof("状态管理 - 保留区块: %d, 修剪: %t, 修剪间隔: %d",
		cm.StateRetentionBlocks, cm.PruningEnabled, cm.PruningInterval)
	cm.logger.Infof("缓存配置 - 大小: %d, TTL: %v, 预加载: %t",
		cm.CacheSize, cm.CacheTTL, cm.PreloadEnabled)
	cm.logger.Infof("索引配置 - 批量大小: %d, 压缩: %t, 压缩间隔: %v",
		cm.IndexBatchSize, cm.CompactionEnabled, cm.CompactionInterval)
	cm.logger.Infof("性能配置 - 批处理: %d, 查询超时: %v, 最大引用: %d",
		cm.BatchProcessingSize, cm.QueryTimeout, cm.MaxConcurrentReferences)
	cm.logger.Infof("监控配置 - 启用: %t, 收集间隔: %v",
		cm.MetricsEnabled, cm.MetricsCollectionInterval)
	cm.logger.Infof("====================")
}

// ============================================================================
//                           📋 配置数据结构定义
// ============================================================================

// CacheConfig 缓存配置结构
//
// 🎯 **缓存配置数据**：
// 包含UTXO缓存相关的所有配置参数。
type CacheConfig struct {
	Size    int           `json:"size"`    // 缓存大小（UTXO数量）
	TTL     time.Duration `json:"ttl"`     // 缓存生存时间
	Enabled bool          `json:"enabled"` // 是否启用缓存
	Preload bool          `json:"preload"` // 是否启用预加载
}

// IndexConfig 索引配置结构
//
// 🎯 **索引配置数据**：
// 包含UTXO索引相关的所有配置参数。
type IndexConfig struct {
	BatchSize          int           `json:"batch_size"`          // 批量操作大小
	CompactionEnabled  bool          `json:"compaction_enabled"`  // 是否启用压缩
	CompactionInterval time.Duration `json:"compaction_interval"` // 压缩间隔
}

// PerformanceConfig 性能配置结构
//
// 🎯 **性能配置数据**：
// 包含UTXO性能调优相关的所有配置参数。
type PerformanceConfig struct {
	BatchProcessingSize     int           `json:"batch_processing_size"`     // 批处理大小
	QueryTimeout            time.Duration `json:"query_timeout"`             // 查询超时时间
	MaxConcurrentReferences uint64        `json:"max_concurrent_references"` // 最大并发引用数
	PruningEnabled          bool          `json:"pruning_enabled"`           // 是否启用修剪
	PruningInterval         int           `json:"pruning_interval"`          // 修剪间隔
	StateRetentionBlocks    int           `json:"state_retention_blocks"`    // 状态保留区块数
}

// MonitoringConfig 监控配置结构
//
// 🎯 **监控配置数据**：
// 包含UTXO监控相关的配置参数。
// 注意：遵循项目约束，默认禁用监控功能。
type MonitoringConfig struct {
	Enabled            bool          `json:"enabled"`             // 是否启用监控
	CollectionInterval time.Duration `json:"collection_interval"` // 监控数据收集间隔
}
