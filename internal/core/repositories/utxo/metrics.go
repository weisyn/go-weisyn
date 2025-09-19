// Package utxo UTXO性能监控指标实现
//
// 📊 **UTXO性能监控器 (UTXO Metrics Collector)**
//
// 本文件实现UTXO模块的性能监控指标收集：
// - 查询性能：记录查询延迟、吞吐量等指标
// - 缓存性能：监控缓存命中率和效率
// - 引用操作：统计引用/解引用操作的性能
// - 系统健康：监控UTXO系统的整体健康状况
//
// 🎯 **核心功能**
// - 延迟统计：详细的操作延迟分布统计
// - 吞吐量监控：实时的操作吞吐量监控
// - 错误率统计：操作成功率和错误统计
// - 资源监控：内存和存储资源使用监控
//
// 🏗️ **设计原则**
// - 内部使用：严格遵循项目约束，仅内部使用，不暴露给接口
// - 轻量级：监控开销最小化，不影响主流程性能
// - 实用性：专注于真实有用的监控指标
// - 可选启用：可通过配置控制监控功能的开启
//
// ⚠️ **重要约束**：
// 根据项目memory约束，公共接口不暴露监控数据。
// 本模块仅供内部性能调优和问题诊断使用。
package utxo

import (
	"sync"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ============================================================================
//                              监控器定义
// ============================================================================

// MetricsCollector UTXO性能监控指标收集器
//
// 🎯 **监控核心组件**
//
// 负责收集UTXO模块的各项性能指标，包括查询性能、缓存效率、
// 操作延迟等关键指标。仅供内部使用，不暴露给公共接口。
//
// 架构特点：
// - 非侵入式：监控逻辑不干扰业务流程
// - 高效率：最小化监控开销
// - 全方位：覆盖UTXO操作的关键性能点
// - 可控制：支持动态开启/关闭监控功能
type MetricsCollector struct {
	// 监控配置
	enabled bool // 是否启用监控

	// 查询性能指标
	queryMetrics     *QueryMetrics     // 查询操作指标
	cacheMetrics     *CacheMetrics     // 缓存操作指标
	referenceMetrics *ReferenceMetrics // 引用操作指标
	systemMetrics    *SystemMetrics    // 系统健康指标

	// 内部状态
	logger    log.Logger   // 日志服务
	mutex     sync.RWMutex // 读写锁保护
	startTime time.Time    // 监控开始时间
}

// ============================================================================
//                              监控数据结构
// ============================================================================

// QueryMetrics 查询性能指标
//
// 🎯 **查询性能监控**：
// 记录UTXO查询操作的性能指标。
type QueryMetrics struct {
	// 精确查询指标
	GetUTXOCount        int64         `json:"get_utxo_count"`         // GetUTXO调用次数
	GetUTXOTotalLatency time.Duration `json:"get_utxo_total_latency"` // GetUTXO总延迟
	GetUTXOMaxLatency   time.Duration `json:"get_utxo_max_latency"`   // GetUTXO最大延迟
	GetUTXOMinLatency   time.Duration `json:"get_utxo_min_latency"`   // GetUTXO最小延迟

	// 地址查询指标
	GetByAddressCount        int64         `json:"get_by_address_count"`         // GetUTXOsByAddress调用次数
	GetByAddressTotalLatency time.Duration `json:"get_by_address_total_latency"` // 地址查询总延迟
	GetByAddressMaxLatency   time.Duration `json:"get_by_address_max_latency"`   // 地址查询最大延迟
	GetByAddressMinLatency   time.Duration `json:"get_by_address_min_latency"`   // 地址查询最小延迟

	// 查询结果统计
	EmptyResultCount    int64 `json:"empty_result_count"`    // 空结果查询次数
	SingleResultCount   int64 `json:"single_result_count"`   // 单结果查询次数
	MultipleResultCount int64 `json:"multiple_result_count"` // 多结果查询次数
	TotalUTXOsReturned  int64 `json:"total_utxos_returned"`  // 总返回UTXO数量

	// 错误统计
	QueryErrorCount int64 `json:"query_error_count"` // 查询错误次数
	TimeoutCount    int64 `json:"timeout_count"`     // 查询超时次数
}

// CacheMetrics 缓存性能指标
//
// 🎯 **缓存性能监控**：
// 记录UTXO缓存操作的性能指标。
type CacheMetrics struct {
	// 缓存命中统计
	CacheHitCount      int64 `json:"cache_hit_count"`      // 缓存命中次数
	CacheMissCount     int64 `json:"cache_miss_count"`     // 缓存未命中次数
	CacheTotalRequests int64 `json:"cache_total_requests"` // 缓存总请求次数

	// 缓存操作统计
	CachePutCount          int64 `json:"cache_put_count"`          // 缓存存入次数
	CacheEvictionCount     int64 `json:"cache_eviction_count"`     // 缓存淘汰次数
	CacheInvalidationCount int64 `json:"cache_invalidation_count"` // 缓存失效次数

	// 缓存效率指标
	CurrentCacheSize int     `json:"current_cache_size"` // 当前缓存大小
	MaxCacheSize     int     `json:"max_cache_size"`     // 最大缓存大小
	CacheHitRate     float64 `json:"cache_hit_rate"`     // 缓存命中率
}

// ReferenceMetrics 引用操作指标
//
// 🎯 **引用操作监控**：
// 记录ResourceUTXO引用操作的性能指标。
type ReferenceMetrics struct {
	// 引用操作统计
	ReferenceCount        int64         `json:"reference_count"`         // 引用操作次数
	UnreferenceCount      int64         `json:"unreference_count"`       // 解除引用次数
	ReferenceTotalLatency time.Duration `json:"reference_total_latency"` // 引用操作总延迟
	ReferenceMaxLatency   time.Duration `json:"reference_max_latency"`   // 引用操作最大延迟

	// 引用状态统计
	ConcurrentReferenceCount int64 `json:"concurrent_reference_count"` // 当前并发引用数
	MaxConcurrentReferences  int64 `json:"max_concurrent_references"`  // 历史最大并发引用数
	ReferenceConflictCount   int64 `json:"reference_conflict_count"`   // 引用冲突次数

	// 引用错误统计
	ReferenceErrorCount     int64 `json:"reference_error_count"`      // 引用操作错误次数
	InvalidReferenceCount   int64 `json:"invalid_reference_count"`    // 无效引用次数
	OverLimitReferenceCount int64 `json:"over_limit_reference_count"` // 超限引用次数
}

// SystemMetrics 系统健康指标
//
// 🎯 **系统健康监控**：
// 记录UTXO系统整体健康状况的指标。
type SystemMetrics struct {
	// 系统状态统计
	UptimeSeconds         int64   `json:"uptime_seconds"`          // 系统运行时间（秒）
	TotalOperationCount   int64   `json:"total_operation_count"`   // 总操作次数
	SuccessOperationCount int64   `json:"success_operation_count"` // 成功操作次数
	ErrorOperationCount   int64   `json:"error_operation_count"`   // 错误操作次数
	OperationSuccessRate  float64 `json:"operation_success_rate"`  // 操作成功率

	// 资源使用统计
	EstimatedMemoryUsage int64 `json:"estimated_memory_usage"` // 估计内存使用量（字节）
	ActiveUTXOCount      int64 `json:"active_utxo_count"`      // 活跃UTXO数量
	IndexCount           int64 `json:"index_count"`            // 索引条目数量

	// 性能基准
	AverageQueryLatency time.Duration `json:"average_query_latency"`  // 平均查询延迟
	TotalProcessingTime time.Duration `json:"total_processing_time"`  // 总处理时间
	LastHealthCheckTime time.Time     `json:"last_health_check_time"` // 最后健康检查时间
}

// ============================================================================
//                              构造函数
// ============================================================================

// NewMetricsCollector 创建UTXO性能监控指标收集器实例
//
// 🏗️ **构造器模式**
//
// 参数：
//   - enabled: 是否启用监控
//   - logger: 日志服务
//
// 返回：
//   - *MetricsCollector: 监控指标收集器实例
func NewMetricsCollector(enabled bool, logger log.Logger) *MetricsCollector {
	collector := &MetricsCollector{
		enabled:          enabled,
		logger:           logger,
		startTime:        time.Now(),
		queryMetrics:     &QueryMetrics{},
		cacheMetrics:     &CacheMetrics{},
		referenceMetrics: &ReferenceMetrics{},
		systemMetrics:    &SystemMetrics{},
	}

	if enabled && logger != nil {
		logger.Debug("UTXO性能监控指标收集器已启用")
	}

	return collector
}

// ============================================================================
//                           📈 查询性能监控
// ============================================================================

// RecordGetUTXOLatency 记录GetUTXO操作延迟
//
// 🎯 **查询性能记录**：
// 记录精确UTXO查询操作的延迟和结果统计。
//
// 参数：
//   - latency: 查询延迟
//   - found: 是否找到UTXO
//   - err: 查询错误
func (mc *MetricsCollector) RecordGetUTXOLatency(latency time.Duration, found bool, err error) {
	if !mc.enabled {
		return
	}

	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	mc.queryMetrics.GetUTXOCount++
	mc.queryMetrics.GetUTXOTotalLatency += latency

	// 更新最大最小延迟
	if mc.queryMetrics.GetUTXOMaxLatency < latency || mc.queryMetrics.GetUTXOCount == 1 {
		mc.queryMetrics.GetUTXOMaxLatency = latency
	}
	if mc.queryMetrics.GetUTXOMinLatency > latency || mc.queryMetrics.GetUTXOCount == 1 {
		mc.queryMetrics.GetUTXOMinLatency = latency
	}

	// 记录查询结果
	if err != nil {
		mc.queryMetrics.QueryErrorCount++
		mc.systemMetrics.ErrorOperationCount++
	} else {
		mc.systemMetrics.SuccessOperationCount++
		if found {
			mc.queryMetrics.SingleResultCount++
			mc.queryMetrics.TotalUTXOsReturned++
		} else {
			mc.queryMetrics.EmptyResultCount++
		}
	}

	mc.systemMetrics.TotalOperationCount++
}

// RecordGetUTXOsByAddressLatency 记录GetUTXOsByAddress操作延迟
//
// 🎯 **地址查询性能记录**：
// 记录按地址查询UTXO操作的延迟和结果统计。
//
// 参数：
//   - latency: 查询延迟
//   - resultCount: 返回的UTXO数量
//   - err: 查询错误
func (mc *MetricsCollector) RecordGetUTXOsByAddressLatency(latency time.Duration, resultCount int, err error) {
	if !mc.enabled {
		return
	}

	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	mc.queryMetrics.GetByAddressCount++
	mc.queryMetrics.GetByAddressTotalLatency += latency

	// 更新最大最小延迟
	if mc.queryMetrics.GetByAddressMaxLatency < latency || mc.queryMetrics.GetByAddressCount == 1 {
		mc.queryMetrics.GetByAddressMaxLatency = latency
	}
	if mc.queryMetrics.GetByAddressMinLatency > latency || mc.queryMetrics.GetByAddressCount == 1 {
		mc.queryMetrics.GetByAddressMinLatency = latency
	}

	// 记录查询结果
	if err != nil {
		mc.queryMetrics.QueryErrorCount++
		mc.systemMetrics.ErrorOperationCount++
	} else {
		mc.systemMetrics.SuccessOperationCount++
		mc.queryMetrics.TotalUTXOsReturned += int64(resultCount)

		if resultCount == 0 {
			mc.queryMetrics.EmptyResultCount++
		} else if resultCount == 1 {
			mc.queryMetrics.SingleResultCount++
		} else {
			mc.queryMetrics.MultipleResultCount++
		}
	}

	mc.systemMetrics.TotalOperationCount++
}

// ============================================================================
//                           🧠 缓存性能监控
// ============================================================================

// RecordCacheHit 记录缓存命中
func (mc *MetricsCollector) RecordCacheHit() {
	if !mc.enabled {
		return
	}

	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	mc.cacheMetrics.CacheHitCount++
	mc.cacheMetrics.CacheTotalRequests++
}

// RecordCacheMiss 记录缓存未命中
func (mc *MetricsCollector) RecordCacheMiss() {
	if !mc.enabled {
		return
	}

	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	mc.cacheMetrics.CacheMissCount++
	mc.cacheMetrics.CacheTotalRequests++
}

// RecordCacheEviction 记录缓存淘汰
func (mc *MetricsCollector) RecordCacheEviction() {
	if !mc.enabled {
		return
	}

	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	mc.cacheMetrics.CacheEvictionCount++
}

// UpdateCacheSize 更新缓存大小
func (mc *MetricsCollector) UpdateCacheSize(currentSize, maxSize int) {
	if !mc.enabled {
		return
	}

	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	mc.cacheMetrics.CurrentCacheSize = currentSize
	mc.cacheMetrics.MaxCacheSize = maxSize

	// 计算缓存命中率
	if mc.cacheMetrics.CacheTotalRequests > 0 {
		mc.cacheMetrics.CacheHitRate = float64(mc.cacheMetrics.CacheHitCount) / float64(mc.cacheMetrics.CacheTotalRequests)
	}
}

// ============================================================================
//                           🔄 引用操作监控
// ============================================================================

// RecordReferenceLatency 记录引用操作延迟
//
// 🎯 **引用操作性能记录**：
// 记录ResourceUTXO引用操作的延迟和结果统计。
//
// 参数：
//   - latency: 操作延迟
//   - isReference: true为引用操作，false为解除引用操作
//   - err: 操作错误
func (mc *MetricsCollector) RecordReferenceLatency(latency time.Duration, isReference bool, err error) {
	if !mc.enabled {
		return
	}

	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	if isReference {
		mc.referenceMetrics.ReferenceCount++
		if err == nil {
			mc.referenceMetrics.ConcurrentReferenceCount++
			if mc.referenceMetrics.ConcurrentReferenceCount > mc.referenceMetrics.MaxConcurrentReferences {
				mc.referenceMetrics.MaxConcurrentReferences = mc.referenceMetrics.ConcurrentReferenceCount
			}
		}
	} else {
		mc.referenceMetrics.UnreferenceCount++
		if err == nil && mc.referenceMetrics.ConcurrentReferenceCount > 0 {
			mc.referenceMetrics.ConcurrentReferenceCount--
		}
	}

	mc.referenceMetrics.ReferenceTotalLatency += latency
	if mc.referenceMetrics.ReferenceMaxLatency < latency {
		mc.referenceMetrics.ReferenceMaxLatency = latency
	}

	// 记录错误
	if err != nil {
		mc.referenceMetrics.ReferenceErrorCount++
		mc.systemMetrics.ErrorOperationCount++
	} else {
		mc.systemMetrics.SuccessOperationCount++
	}

	mc.systemMetrics.TotalOperationCount++
}

// ============================================================================
//                           📊 系统健康监控
// ============================================================================

// UpdateSystemHealth 更新系统健康指标
//
// 🎯 **系统健康更新**：
// 定期更新系统健康相关的指标。
//
// 参数：
//   - activeUTXOs: 当前活跃UTXO数量
//   - indexCount: 索引条目数量
//   - estimatedMemory: 估计内存使用量
func (mc *MetricsCollector) UpdateSystemHealth(activeUTXOs, indexCount int64, estimatedMemory int64) {
	if !mc.enabled {
		return
	}

	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	mc.systemMetrics.ActiveUTXOCount = activeUTXOs
	mc.systemMetrics.IndexCount = indexCount
	mc.systemMetrics.EstimatedMemoryUsage = estimatedMemory
	mc.systemMetrics.UptimeSeconds = int64(time.Since(mc.startTime).Seconds())
	mc.systemMetrics.LastHealthCheckTime = time.Now()

	// 计算操作成功率
	if mc.systemMetrics.TotalOperationCount > 0 {
		mc.systemMetrics.OperationSuccessRate = float64(mc.systemMetrics.SuccessOperationCount) / float64(mc.systemMetrics.TotalOperationCount)
	}

	// 计算平均查询延迟
	totalQueries := mc.queryMetrics.GetUTXOCount + mc.queryMetrics.GetByAddressCount
	if totalQueries > 0 {
		totalLatency := mc.queryMetrics.GetUTXOTotalLatency + mc.queryMetrics.GetByAddressTotalLatency
		mc.systemMetrics.AverageQueryLatency = time.Duration(int64(totalLatency) / totalQueries)
	}
}

// ============================================================================
//                           📋 监控数据访问
// ============================================================================

// GetAllMetrics 获取所有监控指标
//
// 🎯 **监控数据访问**：
// 返回所有监控指标的快照，用于内部性能分析。
// 注意：严格遵循项目约束，仅供内部使用。
//
// 返回：
//   - QueryMetrics: 查询性能指标快照
//   - CacheMetrics: 缓存性能指标快照
//   - ReferenceMetrics: 引用操作指标快照
//   - SystemMetrics: 系统健康指标快照
func (mc *MetricsCollector) GetAllMetrics() (QueryMetrics, CacheMetrics, ReferenceMetrics, SystemMetrics) {
	if !mc.enabled {
		return QueryMetrics{}, CacheMetrics{}, ReferenceMetrics{}, SystemMetrics{}
	}

	mc.mutex.RLock()
	defer mc.mutex.RUnlock()

	return *mc.queryMetrics, *mc.cacheMetrics, *mc.referenceMetrics, *mc.systemMetrics
}

// ResetMetrics 重置所有监控指标
//
// 🎯 **监控重置功能**：
// 重置所有监控计数器，用于重新开始监控周期。
func (mc *MetricsCollector) ResetMetrics() {
	if !mc.enabled {
		return
	}

	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	mc.queryMetrics = &QueryMetrics{}
	mc.cacheMetrics = &CacheMetrics{}
	mc.referenceMetrics = &ReferenceMetrics{}
	mc.systemMetrics = &SystemMetrics{}
	mc.startTime = time.Now()

	if mc.logger != nil {
		mc.logger.Debug("所有UTXO监控指标已重置")
	}
}

// IsEnabled 检查监控是否启用
//
// 🎯 **监控状态检查**：
// 返回监控功能是否启用的状态。
//
// 返回：
//   - bool: 监控是否启用
func (mc *MetricsCollector) IsEnabled() bool {
	return mc.enabled
}

// SetEnabled 设置监控启用状态
//
// 🎯 **监控状态控制**：
// 动态控制监控功能的开启和关闭。
//
// 参数：
//   - enabled: 是否启用监控
func (mc *MetricsCollector) SetEnabled(enabled bool) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	mc.enabled = enabled

	if mc.logger != nil {
		if enabled {
			mc.logger.Debug("UTXO性能监控已启用")
		} else {
			mc.logger.Debug("UTXO性能监控已禁用")
		}
	}
}
