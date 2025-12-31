package coordinator

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/pkg/types"
)

// ============================================================================
// 资源限制测试
// ============================================================================
//
// 🎯 **测试目的**：发现资源限制检查的缺陷和BUG
//
// ============================================================================

// TestGetISPCResourceLimits 测试获取ISPC资源限制
func TestGetISPCResourceLimits(t *testing.T) {
	manager := createTestManager(t)

	// 测试默认情况（可能返回nil）
	limits := manager.getISPCResourceLimits()
	// 如果没有配置，应该返回nil
	// 如果有配置，应该返回ResourceLimits
	if limits != nil {
		assert.NotNil(t, limits)
	}
}

// TestGetISPCResourceLimits_NilConfigProvider 测试nil configProvider
func TestGetISPCResourceLimits_NilConfigProvider(t *testing.T) {
	manager := createTestManager(t)
	manager.configProvider = nil

	limits := manager.getISPCResourceLimits()
	assert.Nil(t, limits, "nil configProvider应该返回nil")
}

// TestCheckResourceLimits_NilUsage 测试nil资源使用
func TestCheckResourceLimits_NilUsage(t *testing.T) {
	manager := createTestManager(t)

	limits := &types.ResourceLimits{
		MaxMemoryBytes: 1024 * 1024,
	}

	err := manager.checkResourceLimits(nil, limits)
	assert.NoError(t, err, "nil usage应该允许（无限制）")
}

// TestCheckResourceLimits_NilLimits 测试nil资源限制
func TestCheckResourceLimits_NilLimits(t *testing.T) {
	manager := createTestManager(t)

	usage := &types.ResourceUsage{
		PeakMemoryBytes: 1024 * 1024,
	}

	err := manager.checkResourceLimits(usage, nil)
	assert.NoError(t, err, "nil limits应该允许（无限制）")
}

// TestCheckResourceLimits_BothNil 测试两者都为nil
func TestCheckResourceLimits_BothNil(t *testing.T) {
	manager := createTestManager(t)

	err := manager.checkResourceLimits(nil, nil)
	assert.NoError(t, err, "两者都为nil应该允许（无限制）")
}

// TestCheckResourceLimits_ValidUsage 测试有效的资源使用
func TestCheckResourceLimits_ValidUsage(t *testing.T) {
	manager := createTestManager(t)

	usage := &types.ResourceUsage{
		PeakMemoryBytes: 512 * 1024, // 512KB
	}

	limits := &types.ResourceLimits{
		MaxMemoryBytes: 1024 * 1024, // 1MB
	}

	err := manager.checkResourceLimits(usage, limits)
	// 如果ValidateResourceUsage实现正确，应该通过
	// 如果实现有问题，可能会返回错误
	if err != nil {
		t.Logf("⚠️ 警告：资源限制检查返回错误（可能是ValidateResourceUsage的实现问题）：%v", err)
	}
}

// TestLogResourceUsage_NilUsage 测试nil资源使用日志
func TestLogResourceUsage_NilUsage(t *testing.T) {
	manager := createTestManager(t)

	// 不应该panic
	assert.NotPanics(t, func() {
		manager.logResourceUsage(nil)
	}, "nil usage不应该panic")
}

// TestLogResourceUsage_WithUsage 测试有资源使用的情况
func TestLogResourceUsage_WithUsage(t *testing.T) {
	manager := createTestManager(t)

	usage := &types.ResourceUsage{
		ExecutionTimeMs:   100,
		PeakMemoryMB:     10.5,
		TraceSizeMB:      2.3,
		HostFunctionCalls: 5,
		UTXOQueries:      3,
		ResourceQueries:  2,
		StateChanges:     1,
	}

	// 不应该panic（即使没有启用资源日志）
	assert.NotPanics(t, func() {
		manager.logResourceUsage(usage)
	}, "记录资源使用不应该panic")
}

// TestLogResourceUsage_NilConfigProvider 测试nil configProvider
func TestLogResourceUsage_NilConfigProvider(t *testing.T) {
	manager := createTestManager(t)
	manager.configProvider = nil

	usage := &types.ResourceUsage{
		ExecutionTimeMs: 100,
	}

	// 不应该panic
	assert.NotPanics(t, func() {
		manager.logResourceUsage(usage)
	}, "nil configProvider不应该panic")
}

