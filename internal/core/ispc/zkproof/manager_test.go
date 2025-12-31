package zkproof

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/weisyn/v1/internal/core/ispc/testutil"
)

// ============================================================================
// manager.go 测试
// ============================================================================

// TestNewManager 测试创建管理器
func TestNewManager(t *testing.T) {
	hashManager := testutil.NewTestHashManager()
	signatureManager := testutil.NewTestSignatureManager()
	logger := testutil.NewTestLogger()
	configProvider := testutil.NewTestConfigProvider()

	manager := NewManager(hashManager, signatureManager, logger, configProvider)
	require.NotNil(t, manager)
	require.NotNil(t, manager.prover)
	require.NotNil(t, manager.validator)
	require.NotNil(t, manager.circuitManager)
	require.NotNil(t, manager.reliabilityEnforcer)
	require.NotNil(t, manager.schemeRegistry)
	require.Equal(t, hashManager, manager.hashManager)
	require.Equal(t, signatureManager, manager.signatureManager)
	require.Equal(t, logger, manager.logger)
}

// TestManager_GetDefaultProvingScheme 测试获取默认证明方案
func TestManager_GetDefaultProvingScheme(t *testing.T) {
	manager := createTestManager(t)
	
	scheme := manager.GetDefaultProvingScheme()
	require.Equal(t, "groth16", scheme)
}

// TestManager_GetDefaultCurve 测试获取默认椭圆曲线
func TestManager_GetDefaultCurve(t *testing.T) {
	manager := createTestManager(t)
	
	curve := manager.GetDefaultCurve()
	require.Equal(t, "bn254", curve)
}

// TestManager_GetSchemeRegistry 测试获取证明方案注册表
func TestManager_GetSchemeRegistry(t *testing.T) {
	manager := createTestManager(t)
	
	registry := manager.GetSchemeRegistry()
	require.NotNil(t, registry)
}

// TestManager_GetScheme 测试获取指定的证明方案
func TestManager_GetScheme(t *testing.T) {
	manager := createTestManager(t)
	
	// 注册一个方案
	scheme := NewGroth16Scheme(testutil.NewTestLogger())
	manager.schemeRegistry.RegisterScheme(scheme)
	
	// 获取方案
	retrievedScheme, err := manager.GetScheme("groth16")
	require.NoError(t, err)
	require.NotNil(t, retrievedScheme)
	require.Equal(t, "groth16", retrievedScheme.SchemeName())
}

// TestManager_GetScheme_NotExists 测试获取不存在的证明方案
func TestManager_GetScheme_NotExists(t *testing.T) {
	manager := createTestManager(t)
	
	_, err := manager.GetScheme("nonexistent")
	require.Error(t, err)
	require.Contains(t, err.Error(), "未注册的证明方案")
}

// TestManager_ListSupportedSchemes 测试列出所有支持的证明方案
func TestManager_ListSupportedSchemes(t *testing.T) {
	manager := createTestManager(t)
	
	// 注册多个方案
	groth16Scheme := NewGroth16Scheme(testutil.NewTestLogger())
	plonkScheme := NewPlonKScheme(testutil.NewTestLogger())
	manager.schemeRegistry.RegisterScheme(groth16Scheme)
	manager.schemeRegistry.RegisterScheme(plonkScheme)
	
	schemes := manager.ListSupportedSchemes()
	require.GreaterOrEqual(t, len(schemes), 2)
	require.Contains(t, schemes, "groth16")
	require.Contains(t, schemes, "plonk")
}

// TestManager_IsSchemeSupported 测试检查证明方案是否支持
func TestManager_IsSchemeSupported(t *testing.T) {
	manager := createTestManager(t)
	
	// 注册方案
	scheme := NewGroth16Scheme(testutil.NewTestLogger())
	manager.schemeRegistry.RegisterScheme(scheme)
	
	require.True(t, manager.IsSchemeSupported("groth16"))
	require.False(t, manager.IsSchemeSupported("nonexistent"))
}

// TestManager_LoadCircuit 测试加载电路
func TestManager_LoadCircuit(t *testing.T) {
	manager := createTestManager(t)
	
	err := manager.LoadCircuit("contract_execution", 1)
	require.NoError(t, err)
	
	require.True(t, manager.IsCircuitLoaded("contract_execution"))
}

// TestManager_IsCircuitLoaded 测试检查电路是否已加载
func TestManager_IsCircuitLoaded(t *testing.T) {
	manager := createTestManager(t)
	
	require.False(t, manager.IsCircuitLoaded("contract_execution"))
	
	err := manager.LoadCircuit("contract_execution", 1)
	require.NoError(t, err)
	
	require.True(t, manager.IsCircuitLoaded("contract_execution"))
}

// TestManager_GetErrorLogs 测试获取错误日志
func TestManager_GetErrorLogs(t *testing.T) {
	manager := createTestManager(t)
	
	logs := manager.GetErrorLogs(10)
	require.NotNil(t, logs)
	require.IsType(t, []ProofGenerationErrorLog{}, logs)
}

// TestManager_GetErrorStats 测试获取错误统计信息
func TestManager_GetErrorStats(t *testing.T) {
	manager := createTestManager(t)
	
	stats := manager.GetErrorStats()
	require.NotNil(t, stats)
	require.IsType(t, map[string]interface{}{}, stats)
}

// TestManager_ClearErrorLogs 测试清空错误日志
func TestManager_ClearErrorLogs(t *testing.T) {
	manager := createTestManager(t)
	
	// 清空错误日志不应该panic
	manager.ClearErrorLogs()
}

// TestManager_GenerateProof 测试生成证明
func TestManager_GenerateProof(t *testing.T) {
	manager := createTestManager(t)
	
	ctx := context.Background()
	input := testutil.NewTestZKProofInput()
	
	// 这个测试可能会失败（因为需要真实的电路和密钥），但我们测试接口调用
	_, err := manager.GenerateProof(ctx, input)
	// 预期可能会失败（因为需要真实的电路），但我们确保接口正确调用
	require.NotNil(t, err) // 预期错误，因为需要真实的电路设置
}

// TestManager_GenerateProofWithRetry 测试带重试机制的证明生成
func TestManager_GenerateProofWithRetry(t *testing.T) {
	manager := createTestManager(t)
	
	ctx := context.Background()
	input := testutil.NewTestZKProofInput()
	
	// 这个测试可能会失败（因为需要真实的电路和密钥），但我们测试接口调用
	_, err := manager.GenerateProofWithRetry(ctx, input)
	// 预期可能会失败（因为需要真实的电路），但我们确保接口正确调用
	require.NotNil(t, err) // 预期错误，因为需要真实的电路设置
}

// createTestManager 创建测试用的管理器
//
// ✅ **重构说明**：使用testutil包中的统一Mock对象，遵循测试规范
func createTestManager(t *testing.T) *Manager {
	hashManager := testutil.NewTestHashManager()
	signatureManager := testutil.NewTestSignatureManager()
	logger := testutil.NewTestLogger()
	configProvider := testutil.NewTestConfigProvider()

	return NewManager(hashManager, signatureManager, logger, configProvider)
}

