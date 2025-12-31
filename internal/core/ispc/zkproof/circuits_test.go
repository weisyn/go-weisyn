package zkproof

import (
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/test"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// circuits.go 测试
// ============================================================================

// TestContractExecutionCircuit_Define 测试合约执行电路定义
func TestContractExecutionCircuit_Define(t *testing.T) {
	circuit := &ContractExecutionCircuit{
		ExecutionResultHash: frontend.Variable(0),
		ExecutionTrace:      frontend.Variable(0),
		StateDiff:            frontend.Variable(0),
	}

	// 编译电路
	_, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	require.NoError(t, err)
}

// TestContractExecutionCircuit_WithWitness 测试合约执行电路与见证
func TestContractExecutionCircuit_WithWitness(t *testing.T) {
	circuit := &ContractExecutionCircuit{
		ExecutionResultHash: frontend.Variable(0),
		ExecutionTrace:      frontend.Variable(0),
		StateDiff:            frontend.Variable(0),
	}

	witness := &ContractExecutionCircuit{
		ExecutionResultHash: 12345, // 公开输入
		ExecutionTrace:      100,    // 私有输入
		StateDiff:           200,   // 私有输入
	}

	// 使用gnark的test包验证电路
	err := test.IsSolved(circuit, witness, ecc.BN254.ScalarField())
	require.NoError(t, err)
}

// TestAIModelInferenceCircuit_Define 测试AI模型推理电路定义
func TestAIModelInferenceCircuit_Define(t *testing.T) {
	circuit := &AIModelInferenceCircuit{
		InferenceResultHash: frontend.Variable(0),
		ModelWeights:         frontend.Variable(0),
		InputData:            frontend.Variable(0),
	}

	// 编译电路
	_, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	require.NoError(t, err)
}

// TestAIModelInferenceCircuit_WithWitness 测试AI模型推理电路与见证
func TestAIModelInferenceCircuit_WithWitness(t *testing.T) {
	circuit := &AIModelInferenceCircuit{
		InferenceResultHash: frontend.Variable(0),
		ModelWeights:        frontend.Variable(0),
		InputData:           frontend.Variable(0),
	}

	witness := &AIModelInferenceCircuit{
		InferenceResultHash: 54321, // 公开输入
		ModelWeights:        300,   // 私有输入
		InputData:           400,   // 私有输入
	}

	// 使用gnark的test包验证电路
	err := test.IsSolved(circuit, witness, ecc.BN254.ScalarField())
	require.NoError(t, err)
}

// TestGenericExecutionCircuit_Define 测试通用执行电路定义
func TestGenericExecutionCircuit_Define(t *testing.T) {
	circuit := &GenericExecutionCircuit{
		ResultHash:     frontend.Variable(0),
		ExecutionData:  frontend.Variable(0),
		AuxiliaryData:  frontend.Variable(0),
	}

	// 编译电路
	_, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	require.NoError(t, err)
}

// TestGenericExecutionCircuit_WithWitness 测试通用执行电路与见证
func TestGenericExecutionCircuit_WithWitness(t *testing.T) {
	circuit := &GenericExecutionCircuit{
		ResultHash:     frontend.Variable(0),
		ExecutionData:  frontend.Variable(0),
		AuxiliaryData:  frontend.Variable(0),
	}

	// 创建见证：ResultHash = ExecutionData² + AuxiliaryData²
	// 例如：ExecutionData = 5, AuxiliaryData = 3
	// ResultHash = 5² + 3² = 25 + 9 = 34
	witness := &GenericExecutionCircuit{
		ResultHash:     34, // 公开输入：5² + 3² = 34
		ExecutionData: 5,   // 私有输入
		AuxiliaryData: 3,   // 私有输入
	}

	// 使用gnark的test包验证电路
	err := test.IsSolved(circuit, witness, ecc.BN254.ScalarField())
	require.NoError(t, err)
}

// TestGenericExecutionCircuit_WithWitness_Invalid 测试通用执行电路无效见证
func TestGenericExecutionCircuit_WithWitness_Invalid(t *testing.T) {
	circuit := &GenericExecutionCircuit{
		ResultHash:     frontend.Variable(0),
		ExecutionData:  frontend.Variable(0),
		AuxiliaryData:  frontend.Variable(0),
	}

	// 创建无效见证：ResultHash 不等于 ExecutionData² + AuxiliaryData²
	witness := &GenericExecutionCircuit{
		ResultHash:     100, // 公开输入：不等于 5² + 3² = 34
		ExecutionData:  5,   // 私有输入
		AuxiliaryData:  3,   // 私有输入
	}

	// 使用gnark的test包验证电路（应该失败）
	err := test.IsSolved(circuit, witness, ecc.BN254.ScalarField())
	require.Error(t, err) // 应该返回错误，因为见证无效
}

// TestGenericExecutionCircuit_WithWitness_ZeroValues 测试通用执行电路零值见证
func TestGenericExecutionCircuit_WithWitness_ZeroValues(t *testing.T) {
	circuit := &GenericExecutionCircuit{
		ResultHash:     frontend.Variable(0),
		ExecutionData:  frontend.Variable(0),
		AuxiliaryData:  frontend.Variable(0),
	}

	// 创建零值见证：ResultHash = 0² + 0² = 0
	witness := &GenericExecutionCircuit{
		ResultHash:     0, // 公开输入：0² + 0² = 0
		ExecutionData:  0, // 私有输入
		AuxiliaryData:  0, // 私有输入
	}

	// 使用gnark的test包验证电路
	err := test.IsSolved(circuit, witness, ecc.BN254.ScalarField())
	require.NoError(t, err)
}

// TestContractExecutionCircuit_WithWitness_ZeroValues 测试合约执行电路零值见证
func TestContractExecutionCircuit_WithWitness_ZeroValues(t *testing.T) {
	circuit := &ContractExecutionCircuit{
		ExecutionResultHash: frontend.Variable(0),
		ExecutionTrace:      frontend.Variable(0),
		StateDiff:           frontend.Variable(0),
	}

	witness := &ContractExecutionCircuit{
		ExecutionResultHash: 0, // 公开输入
		ExecutionTrace:      0, // 私有输入
		StateDiff:           0, // 私有输入
	}

	// 使用gnark的test包验证电路
	err := test.IsSolved(circuit, witness, ecc.BN254.ScalarField())
	require.NoError(t, err)
}

// TestAIModelInferenceCircuit_WithWitness_ZeroValues 测试AI模型推理电路零值见证
func TestAIModelInferenceCircuit_WithWitness_ZeroValues(t *testing.T) {
	circuit := &AIModelInferenceCircuit{
		InferenceResultHash: frontend.Variable(0),
		ModelWeights:         frontend.Variable(0),
		InputData:            frontend.Variable(0),
	}

	witness := &AIModelInferenceCircuit{
		InferenceResultHash: 0, // 公开输入
		ModelWeights:        0, // 私有输入
		InputData:           0, // 私有输入
	}

	// 使用gnark的test包验证电路
	err := test.IsSolved(circuit, witness, ecc.BN254.ScalarField())
	require.NoError(t, err)
}

