// Package types provides resource type definitions.
package types

import "time"

// ==================== ISPC资源消耗统计 ====================

// ResourceUsage 资源使用统计
// 用于记录ISPC执行过程中的资源消耗情况，用于性能分析和问题诊断
// 注意：WES不需要Gas计费，这是本地资源配额管理
type ResourceUsage struct {
	// 执行时间统计
	ExecutionTimeMs    int64         `json:"execution_time_ms"`    // 总执行时间（毫秒）
	ExecutionDuration  time.Duration `json:"execution_duration"`   // 总执行时间（Duration）
	StartTime          time.Time     `json:"start_time"`           // 开始时间
	EndTime            time.Time     `json:"end_time"`             // 结束时间
	
	// 内存使用统计
	PeakMemoryBytes    uint64        `json:"peak_memory_bytes"`    // 峰值内存占用（字节）
	PeakMemoryMB       float64       `json:"peak_memory_mb"`       // 峰值内存占用（MB）
	AverageMemoryBytes uint64        `json:"average_memory_bytes"` // 平均内存占用（字节）
	
	// 存储使用统计
	TraceSizeBytes     uint64        `json:"trace_size_bytes"`    // 执行轨迹大小（字节）
	TraceSizeMB         float64       `json:"trace_size_mb"`       // 执行轨迹大小（MB）
	TempStorageBytes    uint64        `json:"temp_storage_bytes"`  // 临时存储占用（字节）
	
	// 操作统计
	HostFunctionCalls  uint32        `json:"host_function_calls"` // 宿主函数调用次数
	UTXOQueries         uint32        `json:"utxo_queries"`        // UTXO查询次数
	ResourceQueries     uint32        `json:"resource_queries"`   // 资源查询次数
	StateChanges        uint32        `json:"state_changes"`       // 状态变更次数
	
	// CPU使用统计（可选）
	CPUTimeMs           int64         `json:"cpu_time_ms"`         // CPU时间（毫秒，可选）
}

// ResourceLimits ISPC资源限制配置
// 用于定义ISPC执行过程中的资源配额限制
// 注意：WES不需要Gas计费，这是本地资源配额管理
type ResourceLimits struct {
	// 执行时间限制
	ExecutionTimeoutSeconds int `json:"execution_timeout_seconds"` // 执行超时时间（秒）
	
	// 内存限制
	MaxMemoryMB            int `json:"max_memory_mb"`             // 最大内存限制（MB）
	MaxMemoryBytes          uint64 `json:"max_memory_bytes"`      // 最大内存限制（字节）
	
	// 存储限制
	MaxTraceSizeMB          int `json:"max_trace_size_mb"`         // 最大执行轨迹大小（MB）
	MaxTraceSizeBytes       uint64 `json:"max_trace_size_bytes"`  // 最大执行轨迹大小（字节）
	MaxTempStorageMB        int `json:"max_temp_storage_mb"`       // 最大临时存储（MB）
	MaxTempStorageBytes     uint64 `json:"max_temp_storage_bytes"` // 最大临时存储（字节）
	
	// 操作限制
	MaxHostFunctionCalls    uint32 `json:"max_host_function_calls"` // 最大宿主函数调用次数
	MaxUTXOQueries          uint32 `json:"max_utxo_queries"`        // 最大UTXO查询次数
	MaxResourceQueries     uint32 `json:"max_resource_queries"`     // 最大资源查询次数
	
	// 配额管理
	MaxConcurrentExecutions int `json:"max_concurrent_executions"`  // 最大并发执行数
}

// ResourceQuota 资源配额
// 用于管理执行节点的资源配额分配
type ResourceQuota struct {
	// 当前使用情况
	CurrentExecutions      int   `json:"current_executions"`       // 当前并发执行数
	TotalExecutions        int64 `json:"total_executions"`         // 总执行次数
	TotalExecutionTimeMs   int64 `json:"total_execution_time_ms"` // 总执行时间（毫秒）
	
	// 配额限制
	MaxConcurrentExecutions int `json:"max_concurrent_executions"` // 最大并发执行数
	MaxTotalExecutionsPerMin int `json:"max_total_executions_per_min"` // 每分钟最大执行次数
}

// ValidateResourceUsage 验证资源使用是否超出限制
// 返回：是否超出限制，超出限制的资源类型，错误信息
func (usage *ResourceUsage) ValidateResourceUsage(limits *ResourceLimits) (bool, string, error) {
	if limits == nil {
		return true, "", nil // 无限制，允许
	}
	
	// 检查执行时间
	if limits.ExecutionTimeoutSeconds > 0 {
		if usage.ExecutionTimeMs > int64(limits.ExecutionTimeoutSeconds*1000) {
			return false, "execution_time", nil
		}
	}
	
	// 检查内存使用
	if limits.MaxMemoryBytes > 0 {
		if usage.PeakMemoryBytes > limits.MaxMemoryBytes {
			return false, "memory", nil
		}
	}
	
	// 检查执行轨迹大小
	if limits.MaxTraceSizeBytes > 0 {
		if usage.TraceSizeBytes > limits.MaxTraceSizeBytes {
			return false, "trace_size", nil
		}
	}
	
	// 检查临时存储
	if limits.MaxTempStorageBytes > 0 {
		if usage.TempStorageBytes > limits.MaxTempStorageBytes {
			return false, "temp_storage", nil
		}
	}
	
	// 检查操作次数
	if limits.MaxHostFunctionCalls > 0 {
		if usage.HostFunctionCalls > limits.MaxHostFunctionCalls {
			return false, "host_function_calls", nil
		}
	}
	
	if limits.MaxUTXOQueries > 0 {
		if usage.UTXOQueries > limits.MaxUTXOQueries {
			return false, "utxo_queries", nil
		}
	}
	
	if limits.MaxResourceQueries > 0 {
		if usage.ResourceQueries > limits.MaxResourceQueries {
			return false, "resource_queries", nil
		}
	}
	
	return true, "", nil // 未超出限制
}

// ToMB 将字节转换为MB
func (usage *ResourceUsage) ToMB(bytes uint64) float64 {
	return float64(bytes) / (1024 * 1024)
}

// UpdatePeakMemory 更新峰值内存
func (usage *ResourceUsage) UpdatePeakMemory(currentBytes uint64) {
	if currentBytes > usage.PeakMemoryBytes {
		usage.PeakMemoryBytes = currentBytes
		usage.PeakMemoryMB = usage.ToMB(currentBytes)
	}
}

// UpdateTraceSize 更新执行轨迹大小
func (usage *ResourceUsage) UpdateTraceSize(sizeBytes uint64) {
	usage.TraceSizeBytes = sizeBytes
	usage.TraceSizeMB = usage.ToMB(sizeBytes)
}

// Finalize 完成资源使用统计
func (usage *ResourceUsage) Finalize() {
	if !usage.EndTime.IsZero() && !usage.StartTime.IsZero() {
		usage.ExecutionDuration = usage.EndTime.Sub(usage.StartTime)
		usage.ExecutionTimeMs = int64(usage.ExecutionDuration.Milliseconds())
	}
	
	// 更新内存统计
	if usage.PeakMemoryBytes > 0 {
		usage.PeakMemoryMB = usage.ToMB(usage.PeakMemoryBytes)
	}
	
	// 更新轨迹大小统计
	if usage.TraceSizeBytes > 0 {
		usage.TraceSizeMB = usage.ToMB(usage.TraceSizeBytes)
	}
}
