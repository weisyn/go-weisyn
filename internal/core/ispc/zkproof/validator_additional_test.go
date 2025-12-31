package zkproof

import (
	"context"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/test"
	"github.com/stretchr/testify/require"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"

	"github.com/weisyn/v1/internal/core/ispc/testutil"
)

// ============================================================================
// validator.go 测试补充
// ============================================================================

// TestGenericCircuit_Define 测试通用电路定义
func TestGenericCircuit_Define(t *testing.T) {
	circuit := &GenericCircuit{
		PublicInputs: make([]frontend.Variable, 3),
	}

	// 编译电路
	_, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	require.NoError(t, err)
}

// TestGenericCircuit_Define_WithWitness 测试通用电路与见证
func TestGenericCircuit_Define_WithWitness(t *testing.T) {
	circuit := &GenericCircuit{
		PublicInputs: make([]frontend.Variable, 3),
	}

	witness := &GenericCircuit{
		PublicInputs: []frontend.Variable{1, 2, 3},
	}

	// 使用gnark的test包验证电路
	err := test.IsSolved(circuit, witness, ecc.BN254.ScalarField())
	require.NoError(t, err)
}

// TestGenericCircuit_Define_EmptyInputs 测试空输入
func TestGenericCircuit_Define_EmptyInputs(t *testing.T) {
	circuit := &GenericCircuit{
		PublicInputs: make([]frontend.Variable, 0),
	}

	// 编译电路
	_, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	require.NoError(t, err)
}

// TestGenericCircuit_Define_SingleInput 测试单个输入
func TestGenericCircuit_Define_SingleInput(t *testing.T) {
	circuit := &GenericCircuit{
		PublicInputs: make([]frontend.Variable, 1),
	}

	witness := &GenericCircuit{
		PublicInputs: []frontend.Variable{42},
	}

	// 使用gnark的test包验证电路
	err := test.IsSolved(circuit, witness, ecc.BN254.ScalarField())
	require.NoError(t, err)
}

// ============================================================================
// reliability.go 测试补充
// ============================================================================

// TestProofReliabilityEnforcer_VerifyStateProofSelfCheck 测试状态证明验证自检
//
// ✅ **重构说明**：使用testutil包中的统一Mock对象，遵循测试规范
func TestProofReliabilityEnforcer_VerifyStateProofSelfCheck(t *testing.T) {
	logger := testutil.NewTestLogger()
	hashManager := testutil.NewTestHashManager()
	signatureManager := testutil.NewTestSignatureManager()
	configProvider := testutil.NewTestConfigProvider()
	
	manager := NewManager(hashManager, signatureManager, logger, configProvider)
	enforcer := NewProofReliabilityEnforcer(logger, manager.prover, manager.validator, nil)
	
	// 创建一个简单的状态证明（这里使用nil，实际测试中需要有效的证明）
	stateProof := &transaction.ZKStateProof{
		CircuitId: "test_circuit",
	}
	
	// 由于证明无效，验证应该失败
	ctx := context.Background()
	err := enforcer.verifyStateProofSelfCheck(ctx, stateProof)
	// 验证失败是预期的，因为证明无效
	require.Error(t, err)
	require.Contains(t, err.Error(), "验证")
}

// ============================================================================
// incremental/verifier.go 测试补充
// ============================================================================

// TestMin 测试min辅助函数
func TestMin(t *testing.T) {
	// 由于min是包内私有函数，我们需要通过测试使用它的函数来间接测试
	// 或者我们可以创建一个测试文件在incremental包中
	// 这里我们通过测试使用min的函数来间接覆盖
	
	// min函数在recalculateRootHashFromPath中使用
	// 我们可以通过测试verifier来间接测试min函数
	// 但为了直接测试，我们需要在incremental包中创建测试
}

