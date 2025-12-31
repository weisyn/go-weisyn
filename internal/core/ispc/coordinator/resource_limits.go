package coordinator

import (
	"fmt"
	"strconv"
	"strings"

	blockchainconfig "github.com/weisyn/v1/internal/config/blockchain"
	"github.com/weisyn/v1/pkg/types"
)

// convertResourceLimitsConfig 将配置中的ResourceLimitsConfig转换为pkg/types.ResourceLimits
//
// 📋 **参数**：
//   - configLimits: 配置中的资源限制
//
// 🔧 **返回值**：
//   - *types.ResourceLimits: 转换后的资源限制
//
// 🎯 **用途**：将配置层的资源限制转换为执行层的资源限制
func convertResourceLimitsConfig(configLimits *blockchainconfig.ResourceLimitsConfig) *types.ResourceLimits {
	if configLimits == nil {
		return nil
	}

	limits := &types.ResourceLimits{
		ExecutionTimeoutSeconds: configLimits.ExecutionTimeoutSeconds,
		MaxMemoryMB:             configLimits.MaxMemoryMB,
		MaxTraceSizeMB:          configLimits.MaxTraceSizeMB,
		MaxTempStorageMB:        configLimits.MaxTempStorageMB,
		MaxHostFunctionCalls:    configLimits.MaxHostFunctionCalls,
		MaxUTXOQueries:          configLimits.MaxUTXOQueries,
		MaxResourceQueries:      configLimits.MaxResourceQueries,
		MaxConcurrentExecutions: configLimits.MaxConcurrentExecutions,
	}

	// 转换内存限制（字符串格式 -> 字节）
	if configLimits.MaxMemoryMB > 0 {
		limits.MaxMemoryBytes = uint64(configLimits.MaxMemoryMB) * 1024 * 1024
	} else if configLimits.MemoryLimit != "" {
		// 解析字符串格式（如"512MB"）
		if bytes, err := parseMemoryLimit(configLimits.MemoryLimit); err == nil {
			limits.MaxMemoryBytes = bytes
			limits.MaxMemoryMB = int(bytes / (1024 * 1024))
		}
	}

	// 转换执行轨迹大小限制
	if limits.MaxTraceSizeMB > 0 {
		limits.MaxTraceSizeBytes = uint64(limits.MaxTraceSizeMB) * 1024 * 1024
	}

	// 转换临时存储限制
	if limits.MaxTempStorageMB > 0 {
		limits.MaxTempStorageBytes = uint64(limits.MaxTempStorageMB) * 1024 * 1024
	}

	return limits
}

// parseMemoryLimit 解析内存限制字符串（如"512MB"）
func parseMemoryLimit(limitStr string) (uint64, error) {
	limitStr = strings.TrimSpace(strings.ToUpper(limitStr))
	
	var multiplier uint64 = 1
	if strings.HasSuffix(limitStr, "KB") {
		multiplier = 1024
		limitStr = strings.TrimSuffix(limitStr, "KB")
	} else if strings.HasSuffix(limitStr, "MB") {
		multiplier = 1024 * 1024
		limitStr = strings.TrimSuffix(limitStr, "MB")
	} else if strings.HasSuffix(limitStr, "GB") {
		multiplier = 1024 * 1024 * 1024
		limitStr = strings.TrimSuffix(limitStr, "GB")
	}
	
	value, err := strconv.ParseUint(limitStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory limit format: %s", limitStr)
	}
	
	return value * multiplier, nil
}

// getISPCResourceLimits 从配置中获取ISPC资源限制
//
// 📋 **参数**：
//   - configProvider: 配置提供者
//
// 🔧 **返回值**：
//   - *types.ResourceLimits: ISPC资源限制（如果未配置则返回nil）
//
// 🎯 **用途**：从配置中获取ISPC资源限制，用于资源限制检查
func (m *Manager) getISPCResourceLimits() *types.ResourceLimits {
	if m.configProvider == nil {
		return nil
	}

	blockchainConfig := m.configProvider.GetBlockchain()
	if blockchainConfig == nil {
		return nil
	}

	executionConfig := blockchainConfig.Execution
	if executionConfig.ISPC == nil {
		return nil
	}

	ispcConfig := executionConfig.ISPC
	if ispcConfig.ResourceLimits == nil {
		return nil
	}

	return convertResourceLimitsConfig(ispcConfig.ResourceLimits)
}

// checkResourceLimits 检查资源使用是否超出限制
//
// 📋 **参数**：
//   - usage: 资源使用统计
//   - limits: 资源限制
//
// 🔧 **返回值**：
//   - error: 如果超出限制则返回错误
//
// 🎯 **用途**：在执行开始前和执行结束后检查资源限制
func (m *Manager) checkResourceLimits(usage *types.ResourceUsage, limits *types.ResourceLimits) error {
	if usage == nil || limits == nil {
		return nil // 无限制，允许
	}

	valid, resourceType, err := usage.ValidateResourceUsage(limits)
	if err != nil {
		return fmt.Errorf("资源限制验证失败: %w", err)
	}

	if !valid {
		return WrapResourceExhaustedError(resourceType, limits)
	}

	return nil
}

// logResourceUsage 记录资源使用日志（如果启用）
//
// 📋 **参数**：
//   - usage: 资源使用统计
//
// 🔧 **返回值**：无
//
// 🎯 **用途**：在开发/调试模式下记录资源使用日志
func (m *Manager) logResourceUsage(usage *types.ResourceUsage) {
	if usage == nil {
		return
	}

	// 检查是否启用资源日志
	if m.configProvider == nil {
		return
	}

	blockchainConfig := m.configProvider.GetBlockchain()
	if blockchainConfig == nil {
		return
	}

	executionConfig := blockchainConfig.Execution
	if executionConfig.ISPC == nil {
		return
	}

	ispcConfig := executionConfig.ISPC
	if !ispcConfig.EnableResourceLogs {
		return
	}

	// 记录资源使用日志
	m.logger.Infof("📊 资源使用统计: 执行时间=%dms, 峰值内存=%.2fMB, 轨迹大小=%.2fMB, 宿主函数调用=%d, UTXO查询=%d, 资源查询=%d, 状态变更=%d",
		usage.ExecutionTimeMs,
		usage.PeakMemoryMB,
		usage.TraceSizeMB,
		usage.HostFunctionCalls,
		usage.UTXOQueries,
		usage.ResourceQueries,
		usage.StateChanges,
	)
}

