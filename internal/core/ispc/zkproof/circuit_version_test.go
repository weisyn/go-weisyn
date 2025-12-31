package zkproof

import (
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/ispc/testutil"
)

// ============================================================================
// circuit_version.go 测试
// ============================================================================

// TestNewCircuitVersionManager 测试创建电路版本管理器
func TestNewCircuitVersionManager(t *testing.T) {
	logger := testutil.NewTestLogger()
	cvm := NewCircuitVersionManager(logger)
	require.NotNil(t, cvm)
	require.NotNil(t, cvm.versionInfo)
	require.NotNil(t, cvm.optimizationReports)
}

// TestRegisterCircuitVersion 测试注册电路版本
func TestRegisterCircuitVersion(t *testing.T) {
	logger := testutil.NewTestLogger()
	cvm := NewCircuitVersionManager(logger)

	info := &CircuitVersionInfo{
		CircuitID:         "test_circuit",
		Version:           1,
		CreatedAt:         time.Now(),
		ConstraintCount:   1000,
		OptimizationLevel: "basic",
		HashFunction:      "sha256",
		Notes:             "Test circuit",
	}

	cvm.RegisterCircuitVersion(info)

	// 验证已注册
	retrieved, exists := cvm.GetCircuitVersionInfo("test_circuit", 1)
	require.True(t, exists)
	require.NotNil(t, retrieved)
	require.Equal(t, info.CircuitID, retrieved.CircuitID)
	require.Equal(t, info.Version, retrieved.Version)
	require.Equal(t, info.ConstraintCount, retrieved.ConstraintCount)
}

// TestRegisterCircuitVersion_Nil 测试注册nil版本信息
func TestRegisterCircuitVersion_Nil(t *testing.T) {
	logger := testutil.NewTestLogger()
	cvm := NewCircuitVersionManager(logger)

	// 应该不会panic
	cvm.RegisterCircuitVersion(nil)

	// 验证没有注册
	retrieved, exists := cvm.GetCircuitVersionInfo("test_circuit", 1)
	require.False(t, exists)
	require.Nil(t, retrieved)
}

// TestCircuitVersionManager_GetCircuitVersionInfo 测试获取电路版本信息
func TestCircuitVersionManager_GetCircuitVersionInfo(t *testing.T) {
	logger := testutil.NewTestLogger()
	cvm := NewCircuitVersionManager(logger)

	info := &CircuitVersionInfo{
		CircuitID:         "test_circuit",
		Version:           1,
		CreatedAt:         time.Now(),
		ConstraintCount:   1000,
		OptimizationLevel: "basic",
		HashFunction:      "sha256",
		Notes:             "Test circuit",
	}

	cvm.RegisterCircuitVersion(info)

	retrieved, exists := cvm.GetCircuitVersionInfo("test_circuit", 1)
	require.True(t, exists)
	require.NotNil(t, retrieved)
	require.Equal(t, info.CircuitID, retrieved.CircuitID)
}

// TestCircuitVersionManager_GetCircuitVersionInfo_NotExists 测试获取不存在的版本信息
func TestCircuitVersionManager_GetCircuitVersionInfo_NotExists(t *testing.T) {
	logger := testutil.NewTestLogger()
	cvm := NewCircuitVersionManager(logger)

	retrieved, exists := cvm.GetCircuitVersionInfo("nonexistent", 1)
	require.False(t, exists)
	require.Nil(t, retrieved)
}

// TestCircuitVersionManager_ListCircuitVersions 测试列出电路版本
func TestCircuitVersionManager_ListCircuitVersions(t *testing.T) {
	logger := testutil.NewTestLogger()
	cvm := NewCircuitVersionManager(logger)

	// 注册多个版本
	info1 := &CircuitVersionInfo{
		CircuitID:         "test_circuit",
		Version:           1,
		CreatedAt:         time.Now(),
		ConstraintCount:   1000,
		OptimizationLevel: "basic",
		HashFunction:      "sha256",
		Notes:             "Version 1",
	}
	info2 := &CircuitVersionInfo{
		CircuitID:         "test_circuit",
		Version:           2,
		CreatedAt:         time.Now(),
		ConstraintCount:   800,
		OptimizationLevel: "optimized",
		HashFunction:      "poseidon",
		Notes:             "Version 2",
	}

	cvm.RegisterCircuitVersion(info1)
	cvm.RegisterCircuitVersion(info2)

	versions := cvm.ListCircuitVersions("test_circuit")
	require.Equal(t, 2, len(versions))
}

// TestListCircuitVersions_Empty 测试列出不存在的电路版本
func TestListCircuitVersions_Empty(t *testing.T) {
	logger := testutil.NewTestLogger()
	cvm := NewCircuitVersionManager(logger)

	versions := cvm.ListCircuitVersions("nonexistent")
	require.Equal(t, 0, len(versions))
}

// TestAnalyzeCircuitConstraints 测试分析电路约束数量
func TestAnalyzeCircuitConstraints(t *testing.T) {
	logger := testutil.NewTestLogger()
	cvm := NewCircuitVersionManager(logger)

	// 创建一个简单的测试电路
	circuit := &ContractExecutionCircuit{
		ExecutionResultHash: big.NewInt(123),
		ExecutionTrace:      big.NewInt(456),
		StateDiff:           big.NewInt(789),
	}

	constraintCount, err := cvm.AnalyzeCircuitConstraints(circuit)
	require.NoError(t, err)
	require.Greater(t, constraintCount, 0)
}

// TestAnalyzeCircuitConstraints_Nil 测试nil电路
func TestAnalyzeCircuitConstraints_Nil(t *testing.T) {
	logger := testutil.NewTestLogger()
	cvm := NewCircuitVersionManager(logger)

	constraintCount, err := cvm.AnalyzeCircuitConstraints(nil)
	require.Error(t, err)
	require.Equal(t, 0, constraintCount)
	require.Contains(t, err.Error(), "电路不能为nil")
}

// TestGenerateOptimizationReport 测试生成优化报告
func TestGenerateOptimizationReport(t *testing.T) {
	logger := testutil.NewTestLogger()
	cvm := NewCircuitVersionManager(logger)

	circuitID := "test_circuit"
	version := uint32(1)
	constraintCount := 10000

	report := cvm.GenerateOptimizationReport(circuitID, version, constraintCount)
	require.NotNil(t, report)
	require.Equal(t, circuitID, report.CircuitID)
	require.Equal(t, version, report.Version)
	require.Equal(t, constraintCount, report.ConstraintCount)
	require.Greater(t, len(report.Optimizations), 0)
	require.GreaterOrEqual(t, report.EstimatedSavings, 0)

	// 验证报告已存储
	retrieved, exists := cvm.GetOptimizationReport(circuitID, version)
	require.True(t, exists)
	require.NotNil(t, retrieved)
	require.Equal(t, report.CircuitID, retrieved.CircuitID)
}

// TestGenerateOptimizationReport_LargeCircuit 测试大型电路的优化报告
func TestGenerateOptimizationReport_LargeCircuit(t *testing.T) {
	logger := testutil.NewTestLogger()
	cvm := NewCircuitVersionManager(logger)

	circuitID := "test_circuit"
	version := uint32(1)
	constraintCount := 50000 // 大型电路

	report := cvm.GenerateOptimizationReport(circuitID, version, constraintCount)
	require.NotNil(t, report)
	require.Greater(t, len(report.Optimizations), 0)
	
	// 大型电路应该有PlonK建议
	hasPlonKSuggestion := false
	for _, opt := range report.Optimizations {
		if testContains(opt, "PlonK") {
			hasPlonKSuggestion = true
			break
		}
	}
	require.True(t, hasPlonKSuggestion, "大型电路应该建议使用PlonK")
}

// TestGenerateOptimizationReport_MediumCircuit 测试中型电路的优化报告
func TestGenerateOptimizationReport_MediumCircuit(t *testing.T) {
	logger := testutil.NewTestLogger()
	cvm := NewCircuitVersionManager(logger)

	circuitID := "test_circuit"
	version := uint32(1)
	constraintCount := 5000 // 中型电路

	report := cvm.GenerateOptimizationReport(circuitID, version, constraintCount)
	require.NotNil(t, report)
	require.Greater(t, len(report.Optimizations), 0)
}

// TestGenerateOptimizationReport_SmallCircuit 测试小型电路的优化报告
func TestGenerateOptimizationReport_SmallCircuit(t *testing.T) {
	logger := testutil.NewTestLogger()
	cvm := NewCircuitVersionManager(logger)

	circuitID := "test_circuit"
	version := uint32(1)
	constraintCount := 500 // 小型电路

	report := cvm.GenerateOptimizationReport(circuitID, version, constraintCount)
	require.NotNil(t, report)
	require.Greater(t, len(report.Optimizations), 0)
}

// TestCircuitVersionManager_GetOptimizationReport 测试获取优化报告
func TestCircuitVersionManager_GetOptimizationReport(t *testing.T) {
	logger := testutil.NewTestLogger()
	cvm := NewCircuitVersionManager(logger)

	circuitID := "test_circuit"
	version := uint32(1)
	constraintCount := 1000

	// 生成报告
	report := cvm.GenerateOptimizationReport(circuitID, version, constraintCount)
	require.NotNil(t, report)

	// 获取报告
	retrieved, exists := cvm.GetOptimizationReport(circuitID, version)
	require.True(t, exists)
	require.NotNil(t, retrieved)
	require.Equal(t, report.CircuitID, retrieved.CircuitID)
	require.Equal(t, report.Version, retrieved.Version)
}

// TestGetOptimizationReport_NotExists 测试获取不存在的优化报告
func TestGetOptimizationReport_NotExists(t *testing.T) {
	logger := testutil.NewTestLogger()
	cvm := NewCircuitVersionManager(logger)

	report, exists := cvm.GetOptimizationReport("nonexistent", 1)
	require.False(t, exists)
	require.Nil(t, report)
}

// TestContainsOptimization 测试包含优化检查
func TestContainsOptimization(t *testing.T) {
	logger := testutil.NewTestLogger()
	cvm := NewCircuitVersionManager(logger)

	// 测试包含
	require.True(t, cvm.containsOptimization("考虑使用Poseidon哈希", "Poseidon"))
	require.True(t, cvm.containsOptimization("使用预计算值", "预计算"))
	require.True(t, cvm.containsOptimization("使用查找表", "查找表"))

	// 测试不包含
	require.False(t, cvm.containsOptimization("考虑使用SHA-256", "Poseidon"))
	require.False(t, cvm.containsOptimization("", "Poseidon"))
}

// TestEstimateConstraintSavings 测试估算约束节省
func TestEstimateConstraintSavings(t *testing.T) {
	logger := testutil.NewTestLogger()
	cvm := NewCircuitVersionManager(logger)

	constraintCount := 10000
	optimizations := []string{
		"考虑使用Poseidon哈希替代SHA-256",
		"使用预计算值减少约束",
		"使用查找表优化复杂运算",
	}

	savings := cvm.estimateConstraintSavings(constraintCount, optimizations)
	require.Greater(t, savings, 0)
	require.LessOrEqual(t, savings, constraintCount/2) // 不应超过50%
}

// TestEstimateConstraintSavings_NoOptimizations 测试无优化建议
func TestEstimateConstraintSavings_NoOptimizations(t *testing.T) {
	logger := testutil.NewTestLogger()
	cvm := NewCircuitVersionManager(logger)

	constraintCount := 10000
	optimizations := []string{}

	savings := cvm.estimateConstraintSavings(constraintCount, optimizations)
	require.Equal(t, 0, savings)
}

// testContains 辅助函数（用于测试，避免与reliability.go中的contains冲突）
func testContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			testContainsInMiddle(s, substr))))
}

func testContainsInMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

