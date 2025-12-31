// Package repository - 默认配置定义
//
// 🔧 **资源管理仓库默认配置 (Repository Default Configuration)**
//
// 本文件定义了资源管理系统的所有默认配置常量，这些值经过性能测试和生产环境验证，
// 可以在大多数场景下提供良好的性能和稳定性。
package repository

import "time"

// ============================================================================
//                         🧹 垃圾回收默认配置
// ============================================================================

const (
	// DefaultGCBatchSize 默认单次清理50个资源（提升批处理效率）
	DefaultGCBatchSize = 50
	// MaxGCBatchSize 最大单次清理1000个资源（避免长时间阻塞）
	MaxGCBatchSize = 1000

	// DefaultGCTriggerInterval 每2小时自动清理一次
	DefaultGCTriggerInterval = time.Hour * 2
	// DefaultStoragePressureThreshold 存储使用率80%时触发清理
	DefaultStoragePressureThreshold = 0.8

	// DefaultGCAggressiveMode 默认不启用激进模式（保持系统稳定）
	DefaultGCAggressiveMode = false
	// DefaultGCSafeMode 默认启用安全模式（更多验证步骤）
	DefaultGCSafeMode = true
)

// ============================================================================
//                         🔍 查询限制默认配置
// ============================================================================

const (
	// DefaultQueryPageSize 默认分页50个资源
	DefaultQueryPageSize = 50
	// MaxQueryPageSize 最大分页1000个资源（避免内存溢出）
	MaxQueryPageSize = 1000

	// MaxBatchQuerySize 最大批量查询100个资源
	MaxBatchQuerySize = 100

	// DefaultQueryTimeout 普通查询超时30秒
	DefaultQueryTimeout = time.Second * 30
	// DefaultComplexQueryTimeout 复杂查询超时5分钟
	DefaultComplexQueryTimeout = time.Minute * 5

	// DefaultEnableQueryCache 默认启用查询缓存
	DefaultEnableQueryCache = true
	// DefaultQueryCacheTTL 查询缓存10分钟有效期
	DefaultQueryCacheTTL = time.Minute * 10
)

// ============================================================================
//                         🔧 一致性检查默认配置
// ============================================================================

const (
	// DefaultConsistencyCheckEnabled 默认启用自动一致性检查
	DefaultConsistencyCheckEnabled = true
	// DefaultConsistencyCheckInterval 每6小时检查一次
	DefaultConsistencyCheckInterval = time.Hour * 6
	// DefaultDeepCheckEnabled 默认不启用深度检查（性能考虑）
	DefaultDeepCheckEnabled = false

	// DefaultAutoRepairEnabled 默认启用自动修复
	DefaultAutoRepairEnabled = true
	// DefaultRepairBatchSize 修复批处理20个资源
	DefaultRepairBatchSize = 20

	// DefaultHealthStatusTTL 健康状态缓存24小时
	DefaultHealthStatusTTL = time.Hour * 24
)

// ============================================================================
//                         ⚡ 性能优化默认配置
// ============================================================================

const (
	// DefaultEnableIndexV2 默认启用v2索引（并发优化）
	DefaultEnableIndexV2 = true
	// DefaultIndexCacheSize 索引缓存10000条记录
	DefaultIndexCacheSize = 10000
	// DefaultIndexBatchSize 索引批处理100个资源
	DefaultIndexBatchSize = 100

	// DefaultEnableStreaming 默认启用流式处理
	DefaultEnableStreaming = true
	// DefaultStreamBufferSize 流式缓冲区64KB
	DefaultStreamBufferSize = 64 * 1024
	// DefaultLargeFileSizeThreshold 大文件阈值100MB
	DefaultLargeFileSizeThreshold = 100 * 1024 * 1024

	// DefaultMaxConcurrentOps 最大50个并发操作
	DefaultMaxConcurrentOps = 50
	// DefaultWorkerPoolSize 工作池10个worker
	DefaultWorkerPoolSize = 10
)

// ============================================================================
//                         📊 性能监控默认配置
// ============================================================================

const (
	// DefaultPerformanceHistorySize 保留最近100个区块的性能指标
	DefaultPerformanceHistorySize = 100

	// DefaultConsistencyCheckRange 验证最近100个区块的一致性
	DefaultConsistencyCheckRange = 100

	// DefaultMaxBlockRangeSize 单次查询的最大区块数量
	DefaultMaxBlockRangeSize = 10000
)

// ============================================================================
//                         📦 Outbox模式默认配置
// ============================================================================

const (
	// DefaultOutboxMaxRetries 最大重试次数
	DefaultOutboxMaxRetries = 3
	// DefaultOutboxRetryDelay 重试延迟
	DefaultOutboxRetryDelay = time.Second * 2

	// DefaultOutboxProcessorInterval 处理器运行间隔
	DefaultOutboxProcessorInterval = time.Second * 30
	// DefaultOutboxBatchSize 批量处理事件数量
	DefaultOutboxBatchSize = 50

	// DefaultOutboxCleanupInterval 清理已完成事件的间隔
	DefaultOutboxCleanupInterval = time.Hour * 24
	// DefaultOutboxEventRetention 事件保留时间（72小时）
	DefaultOutboxEventRetention = time.Hour * 72
)

// ============================================================================
//                         📊 配置建议和说明
// ============================================================================

/*
🎯 **配置建议 (Configuration Recommendations)**

📈 **高并发环境调优**：
- 增加 MaxGCBatchSize 到 2000-5000
- 增加 MaxConcurrentOps 到 100-200
- 增加 WorkerPoolSize 到 20-50
- 减少 GCTriggerInterval 到 30分钟-1小时

💾 **内存受限环境调优**：
- 减少 DefaultQueryPageSize 到 20-30
- 减少 MaxQueryPageSize 到 500
- 减少 IndexCacheSize 到 5000
- 减少 StreamBufferSize 到 32KB

🔒 **高可靠性环境调优**：
- 启用 DeepCheckEnabled = true
- 减少 ConsistencyCheckInterval 到 2-3小时
- 启用 GCSafeMode = true
- 减少 RepairBatchSize 到 10

⚡ **高性能环境调优**：
- 启用 GCAggressiveMode = true
- 增加 LargeFileSizeThreshold 到 500MB-1GB
- 增加 QueryTimeout 到 60-120秒
- 禁用 DeepCheckEnabled = false

🌐 **网络受限环境调优**：
- 增加 QueryTimeout 到 60-300秒
- 增加 ComplexQueryTimeout 到 10-30分钟
- 减少 MaxBatchQuerySize 到 50
- 增加重试机制相关配置
*/
