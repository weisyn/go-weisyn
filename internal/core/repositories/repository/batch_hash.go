package repository

import (
	"time"

	repositoryConfig "github.com/weisyn/v1/internal/config/repository"
)

// ============================================================================
//                          📊 性能监控和指标
// ============================================================================

// PerformanceMetrics 性能指标
type PerformanceMetrics struct {
	BlockHeight         uint64        `json:"block_height"`          // 区块高度
	BlockProcessingTime time.Duration `json:"block_processing_time"` // 区块处理总时间
	TransactionCount    int           `json:"transaction_count"`     // 交易数量
	ResourceCount       int           `json:"resource_count"`        // 资源数量
	IndexUpdateTime     time.Duration `json:"index_update_time"`     // 索引更新时间
	HashCalculationTime time.Duration `json:"hash_calculation_time"` // 哈希计算时间
	OutboxEventTime     time.Duration `json:"outbox_event_time"`     // Outbox事件时间
	StorageWriteTime    time.Duration `json:"storage_write_time"`    // 存储写入时间
}

// PerformanceMonitor 性能监控器
type PerformanceMonitor struct {
	recentMetrics []*PerformanceMetrics // 最近的性能指标
	maxHistory    int                   // 最大历史记录数
}

// NewPerformanceMonitor 创建性能监控器（使用默认配置）
func NewPerformanceMonitor() *PerformanceMonitor {
	return &PerformanceMonitor{
		recentMetrics: make([]*PerformanceMetrics, 0),
		maxHistory:    100, // 保留最近100个区块的性能指标
	}
}

// NewPerformanceMonitorWithConfig 创建性能监控器（使用配置）
func NewPerformanceMonitorWithConfig(config *repositoryConfig.PerformanceConfig) *PerformanceMonitor {
	return &PerformanceMonitor{
		recentMetrics: make([]*PerformanceMetrics, 0),
		maxHistory:    config.PerformanceHistorySize,
	}
}

// RecordMetrics 记录性能指标
func (pm *PerformanceMonitor) RecordMetrics(metrics *PerformanceMetrics) {
	pm.recentMetrics = append(pm.recentMetrics, metrics)

	// 保持历史记录在限制内
	if len(pm.recentMetrics) > pm.maxHistory {
		pm.recentMetrics = pm.recentMetrics[1:]
	}
}

// GetAverageMetrics 获取平均性能指标
func (pm *PerformanceMonitor) GetAverageMetrics() *PerformanceMetrics {
	if len(pm.recentMetrics) == 0 {
		return &PerformanceMetrics{}
	}

	var total PerformanceMetrics
	count := len(pm.recentMetrics)

	for _, metrics := range pm.recentMetrics {
		total.BlockProcessingTime += metrics.BlockProcessingTime
		total.TransactionCount += metrics.TransactionCount
		total.ResourceCount += metrics.ResourceCount
		total.IndexUpdateTime += metrics.IndexUpdateTime
		total.HashCalculationTime += metrics.HashCalculationTime
		total.OutboxEventTime += metrics.OutboxEventTime
		total.StorageWriteTime += metrics.StorageWriteTime
	}

	return &PerformanceMetrics{
		BlockProcessingTime: total.BlockProcessingTime / time.Duration(count),
		TransactionCount:    total.TransactionCount / count,
		ResourceCount:       total.ResourceCount / count,
		IndexUpdateTime:     total.IndexUpdateTime / time.Duration(count),
		HashCalculationTime: total.HashCalculationTime / time.Duration(count),
		OutboxEventTime:     total.OutboxEventTime / time.Duration(count),
		StorageWriteTime:    total.StorageWriteTime / time.Duration(count),
	}
}
