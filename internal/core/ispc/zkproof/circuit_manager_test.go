package zkproof

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/ispc/testutil"
)

// ============================================================================
// circuit_manager.go 测试
// ============================================================================
//
// ✅ **重构说明**：使用testutil包中的统一Mock对象，遵循测试规范
//
// ============================================================================

// TestNewCircuitManager 测试创建电路管理器
func TestNewCircuitManager(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := &ZKProofManagerConfig{
		DefaultProvingScheme: "groth16",
		DefaultCurve:         "bn254",
	}

	cm := NewCircuitManager(logger, config)
	require.NotNil(t, cm)
	require.NotNil(t, cm.versionManager)
	require.Equal(t, config, cm.config)
}

// TestGetCircuit_ContractExecution 测试获取合约执行电路
func TestGetCircuit_ContractExecution(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := &ZKProofManagerConfig{
		DefaultProvingScheme: "groth16",
		DefaultCurve:         "bn254",
	}
	cm := NewCircuitManager(logger, config)

	circuit, err := cm.GetCircuit("contract_execution", 1)
	require.NoError(t, err)
	require.NotNil(t, circuit)
	
	// 验证电路类型
	_, ok := circuit.(*ContractExecutionCircuit)
	require.True(t, ok, "应该是ContractExecutionCircuit类型")
}

// TestGetCircuit_AIModelInference 测试获取AI模型推理电路
func TestGetCircuit_AIModelInference(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := &ZKProofManagerConfig{
		DefaultProvingScheme: "groth16",
		DefaultCurve:         "bn254",
	}
	cm := NewCircuitManager(logger, config)

	circuit, err := cm.GetCircuit("aimodel_inference", 1)
	require.NoError(t, err)
	require.NotNil(t, circuit)
	
	// 验证电路类型
	_, ok := circuit.(*AIModelInferenceCircuit)
	require.True(t, ok, "应该是AIModelInferenceCircuit类型")
}

// TestGetCircuit_MerklePath 测试获取Merkle路径电路（应该失败）
func TestGetCircuit_MerklePath(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := &ZKProofManagerConfig{
		DefaultProvingScheme: "groth16",
		DefaultCurve:         "bn254",
	}
	cm := NewCircuitManager(logger, config)

	circuit, err := cm.GetCircuit("merkle_path", 1)
	require.Error(t, err)
	require.Nil(t, circuit)
	require.Contains(t, err.Error(), "需要通过工厂函数创建")
}

// TestGetCircuit_Unsupported 测试不支持的电路ID
func TestGetCircuit_Unsupported(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := &ZKProofManagerConfig{
		DefaultProvingScheme: "groth16",
		DefaultCurve:         "bn254",
	}
	cm := NewCircuitManager(logger, config)

	circuit, err := cm.GetCircuit("unsupported_circuit", 1)
	require.Error(t, err)
	require.Nil(t, circuit)
	require.Contains(t, err.Error(), "不支持的电路ID")
}

// TestGetCircuit_Cache 测试电路缓存
func TestGetCircuit_Cache(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := &ZKProofManagerConfig{
		DefaultProvingScheme: "groth16",
		DefaultCurve:         "bn254",
	}
	cm := NewCircuitManager(logger, config)

	// 第一次获取
	circuit1, err := cm.GetCircuit("contract_execution", 1)
	require.NoError(t, err)
	require.NotNil(t, circuit1)

	// 第二次获取（应该从缓存）
	circuit2, err := cm.GetCircuit("contract_execution", 1)
	require.NoError(t, err)
	require.NotNil(t, circuit2)
	
	// 验证是同一个实例（缓存）
	require.Equal(t, circuit1, circuit2)
}

// TestGetCircuit_DifferentVersions 测试不同版本的电路
func TestGetCircuit_DifferentVersions(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := &ZKProofManagerConfig{
		DefaultProvingScheme: "groth16",
		DefaultCurve:         "bn254",
	}
	cm := NewCircuitManager(logger, config)

	circuit1, err := cm.GetCircuit("contract_execution", 1)
	require.NoError(t, err)
	require.NotNil(t, circuit1)

	// 不同版本应该返回不同的电路（即使版本不存在也应该尝试创建）
	circuit2, err := cm.GetCircuit("contract_execution", 2)
	require.Error(t, err) // 版本2不存在
	require.Nil(t, circuit2)
}

// TestLoadCircuit 测试预加载电路
func TestLoadCircuit(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := &ZKProofManagerConfig{
		DefaultProvingScheme: "groth16",
		DefaultCurve:         "bn254",
	}
	cm := NewCircuitManager(logger, config)

	err := cm.LoadCircuit("contract_execution", 1)
	require.NoError(t, err)
	
	// 验证电路已加载
	require.True(t, cm.IsCircuitLoaded("contract_execution"))
}

// TestLoadCircuit_Error 测试预加载不存在的电路
func TestLoadCircuit_Error(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := &ZKProofManagerConfig{
		DefaultProvingScheme: "groth16",
		DefaultCurve:         "bn254",
	}
	cm := NewCircuitManager(logger, config)

	err := cm.LoadCircuit("unsupported_circuit", 1)
	require.Error(t, err)
}

// TestIsCircuitLoaded 测试检查电路是否已加载
func TestIsCircuitLoaded(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := &ZKProofManagerConfig{
		DefaultProvingScheme: "groth16",
		DefaultCurve:         "bn254",
	}
	cm := NewCircuitManager(logger, config)

	// 初始状态应该未加载
	require.False(t, cm.IsCircuitLoaded("contract_execution"))

	// 加载电路
	err := cm.LoadCircuit("contract_execution", 1)
	require.NoError(t, err)

	// 应该已加载
	require.True(t, cm.IsCircuitLoaded("contract_execution"))
}

// TestGetCircuitVersionInfo 测试获取电路版本信息
func TestGetCircuitVersionInfo(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := &ZKProofManagerConfig{
		DefaultProvingScheme: "groth16",
		DefaultCurve:         "bn254",
	}
	cm := NewCircuitManager(logger, config)

	// 加载电路（会注册版本信息）
	err := cm.LoadCircuit("contract_execution", 1)
	require.NoError(t, err)

	// 获取版本信息
	info, exists := cm.GetCircuitVersionInfo("contract_execution", 1)
	require.True(t, exists)
	require.NotNil(t, info)
	require.Equal(t, "contract_execution", info.CircuitID)
	require.Equal(t, uint32(1), info.Version)
}

// TestGetCircuitVersionInfo_NotExists 测试获取不存在的版本信息
func TestGetCircuitVersionInfo_NotExists(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := &ZKProofManagerConfig{
		DefaultProvingScheme: "groth16",
		DefaultCurve:         "bn254",
	}
	cm := NewCircuitManager(logger, config)

	info, exists := cm.GetCircuitVersionInfo("contract_execution", 1)
	require.False(t, exists)
	require.Nil(t, info)
}

// TestGetOptimizationReport 测试获取优化报告
func TestGetOptimizationReport(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := &ZKProofManagerConfig{
		DefaultProvingScheme: "groth16",
		DefaultCurve:         "bn254",
	}
	cm := NewCircuitManager(logger, config)

	// 获取不存在的优化报告
	report, exists := cm.GetOptimizationReport("contract_execution", 1)
	require.False(t, exists)
	require.Nil(t, report)
}

// TestListCircuitVersions 测试列出电路版本
func TestListCircuitVersions(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := &ZKProofManagerConfig{
		DefaultProvingScheme: "groth16",
		DefaultCurve:         "bn254",
	}
	cm := NewCircuitManager(logger, config)

	// 初始状态应该为空
	versions := cm.ListCircuitVersions("contract_execution")
	require.Equal(t, 0, len(versions))

	// 加载电路
	err := cm.LoadCircuit("contract_execution", 1)
	require.NoError(t, err)

	// 应该有一个版本
	versions = cm.ListCircuitVersions("contract_execution")
	require.Equal(t, 1, len(versions))
	require.Equal(t, uint32(1), versions[0].Version)
}

// TestCreateContractExecutionCircuit_UnsupportedVersion 测试不支持的版本
func TestCreateContractExecutionCircuit_UnsupportedVersion(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := &ZKProofManagerConfig{
		DefaultProvingScheme: "groth16",
		DefaultCurve:         "bn254",
	}
	cm := NewCircuitManager(logger, config)

	circuit, err := cm.GetCircuit("contract_execution", 999)
	require.Error(t, err)
	require.Nil(t, circuit)
	require.Contains(t, err.Error(), "不支持的合约执行电路版本")
}

// TestCreateAIModelInferenceCircuit_UnsupportedVersion 测试不支持的版本
func TestCreateAIModelInferenceCircuit_UnsupportedVersion(t *testing.T) {
	logger := testutil.NewTestLogger()
	config := &ZKProofManagerConfig{
		DefaultProvingScheme: "groth16",
		DefaultCurve:         "bn254",
	}
	cm := NewCircuitManager(logger, config)

	circuit, err := cm.GetCircuit("aimodel_inference", 999)
	require.Error(t, err)
	require.Nil(t, circuit)
	require.Contains(t, err.Error(), "不支持的AI模型推理电路版本")
}

