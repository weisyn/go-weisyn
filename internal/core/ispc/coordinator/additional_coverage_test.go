package coordinator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/ispc/testutil"
	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ============================================================================
// 额外覆盖率测试
// ============================================================================
//
// 🎯 **测试目的**：覆盖之前未测试的函数和场景
//
// ============================================================================

// TestPrintZKProofResult 测试打印ZK证明结果
func TestPrintZKProofResult(t *testing.T) {
	manager := createTestManager(t)

	circuitID := "test_circuit"
	version := uint32(1)
	proof := &pb.ZKStateProof{
		Proof:               []byte{0x12, 0x34, 0x56},
		PublicInputs:        [][]byte{{0x78}, {0x9a}},
		ProvingScheme:       "groth16",
		Curve:               "bn254",
		VerificationKeyHash: []byte{0xbc, 0xde, 0xf0},
		CircuitId:           circuitID,
		CircuitVersion:      version,
		ConstraintCount:     100,
	}

	// 不应该panic
	assert.NotPanics(t, func() {
		manager.printZKProofResult(circuitID, version, proof)
	}, "打印ZK证明结果不应该panic")
}

// TestPrintZKProofResult_EmptyProof 测试空证明的情况
func TestPrintZKProofResult_EmptyProof(t *testing.T) {
	manager := createTestManager(t)

	circuitID := "test_circuit"
	version := uint32(1)
	proof := &pb.ZKStateProof{
		Proof:               []byte{},
		PublicInputs:        [][]byte{},
		ProvingScheme:       "groth16",
		Curve:               "bn254",
		VerificationKeyHash: []byte{},
		CircuitId:           circuitID,
		CircuitVersion:      version,
		ConstraintCount:     0,
	}

	// 不应该panic
	assert.NotPanics(t, func() {
		manager.printZKProofResult(circuitID, version, proof)
	}, "打印空证明不应该panic")
}

// TestGenerateStateID_WithParamsCount 测试带参数数量的generateStateID
func TestGenerateStateID_WithParamsCount(t *testing.T) {
	manager := createTestManager(t)
	manager.hashManager = testutil.NewTestHashManager()

	ctx := context.Background()
	executionStartTime := time.Now()
	ctx = context.WithValue(ctx, ContextKeyContract, "test_contract")
	ctx = context.WithValue(ctx, ContextKeyFunction, "test_function")
	ctx = context.WithValue(ctx, ContextKeyExecutionStart, executionStartTime)
	ctx = context.WithValue(ctx, ContextKeyParamsCount, 5)

	stateID, err := manager.generateStateID(ctx)
	require.NoError(t, err)
	assert.NotNil(t, stateID)
	assert.Greater(t, len(stateID), 0, "状态ID应该不为空")
}

// TestGenerateStateID_WithAllContextValues 测试包含所有上下文值的情况
func TestGenerateStateID_WithAllContextValues(t *testing.T) {
	manager := createTestManager(t)
	manager.hashManager = testutil.NewTestHashManager()

	ctx := context.Background()
	executionStartTime := time.Now()
	ctx = context.WithValue(ctx, ContextKeyContract, "test_contract_address")
	ctx = context.WithValue(ctx, ContextKeyFunction, "test_function_name")
	ctx = context.WithValue(ctx, ContextKeyExecutionStart, executionStartTime)
	ctx = context.WithValue(ctx, ContextKeyParamsCount, 10)

	stateID1, err1 := manager.generateStateID(ctx)
	require.NoError(t, err1)

	// 相同输入应该产生相同的状态ID（确定性）
	stateID2, err2 := manager.generateStateID(ctx)
	require.NoError(t, err2)
	assert.Equal(t, stateID1, stateID2, "相同输入应该产生相同的状态ID")
}

// TestGenerateStateID_DifferentContracts 测试不同合约产生不同状态ID
func TestGenerateStateID_DifferentContracts(t *testing.T) {
	manager := createTestManager(t)
	manager.hashManager = testutil.NewTestHashManager()

	executionStartTime := time.Now()

	ctx1 := context.Background()
	ctx1 = context.WithValue(ctx1, ContextKeyContract, "contract_a")
	ctx1 = context.WithValue(ctx1, ContextKeyFunction, "test_function")
	ctx1 = context.WithValue(ctx1, ContextKeyExecutionStart, executionStartTime)

	ctx2 := context.Background()
	ctx2 = context.WithValue(ctx2, ContextKeyContract, "contract_b")
	ctx2 = context.WithValue(ctx2, ContextKeyFunction, "test_function")
	ctx2 = context.WithValue(ctx2, ContextKeyExecutionStart, executionStartTime)

	stateID1, err1 := manager.generateStateID(ctx1)
	require.NoError(t, err1)

	stateID2, err2 := manager.generateStateID(ctx2)
	require.NoError(t, err2)

	assert.NotEqual(t, stateID1, stateID2, "不同合约应该产生不同的状态ID")
}

// TestGenerateStateID_DifferentFunctions 测试不同函数产生不同状态ID
func TestGenerateStateID_DifferentFunctions(t *testing.T) {
	manager := createTestManager(t)
	manager.hashManager = testutil.NewTestHashManager()

	executionStartTime := time.Now()

	ctx1 := context.Background()
	ctx1 = context.WithValue(ctx1, ContextKeyContract, "test_contract")
	ctx1 = context.WithValue(ctx1, ContextKeyFunction, "function_a")
	ctx1 = context.WithValue(ctx1, ContextKeyExecutionStart, executionStartTime)

	ctx2 := context.Background()
	ctx2 = context.WithValue(ctx2, ContextKeyContract, "test_contract")
	ctx2 = context.WithValue(ctx2, ContextKeyFunction, "function_b")
	ctx2 = context.WithValue(ctx2, ContextKeyExecutionStart, executionStartTime)

	stateID1, err1 := manager.generateStateID(ctx1)
	require.NoError(t, err1)

	stateID2, err2 := manager.generateStateID(ctx2)
	require.NoError(t, err2)

	assert.NotEqual(t, stateID1, stateID2, "不同函数应该产生不同的状态ID")
}

// TestGenerateStateID_DifferentTimes 测试不同时间产生不同状态ID
func TestGenerateStateID_DifferentTimes(t *testing.T) {
	manager := createTestManager(t)
	manager.hashManager = testutil.NewTestHashManager()

	time1 := time.Now()
	time2 := time1.Add(1 * time.Second)

	ctx1 := context.Background()
	ctx1 = context.WithValue(ctx1, ContextKeyContract, "test_contract")
	ctx1 = context.WithValue(ctx1, ContextKeyFunction, "test_function")
	ctx1 = context.WithValue(ctx1, ContextKeyExecutionStart, time1)

	ctx2 := context.Background()
	ctx2 = context.WithValue(ctx2, ContextKeyContract, "test_contract")
	ctx2 = context.WithValue(ctx2, ContextKeyFunction, "test_function")
	ctx2 = context.WithValue(ctx2, ContextKeyExecutionStart, time2)

	stateID1, err1 := manager.generateStateID(ctx1)
	require.NoError(t, err1)

	stateID2, err2 := manager.generateStateID(ctx2)
	require.NoError(t, err2)

	assert.NotEqual(t, stateID1, stateID2, "不同时间应该产生不同的状态ID")
}

// TestGenerateStateID_DifferentParamsCount 测试不同参数数量产生不同状态ID
func TestGenerateStateID_DifferentParamsCount(t *testing.T) {
	manager := createTestManager(t)
	manager.hashManager = testutil.NewTestHashManager()

	executionStartTime := time.Now()

	ctx1 := context.Background()
	ctx1 = context.WithValue(ctx1, ContextKeyContract, "test_contract")
	ctx1 = context.WithValue(ctx1, ContextKeyFunction, "test_function")
	ctx1 = context.WithValue(ctx1, ContextKeyExecutionStart, executionStartTime)
	ctx1 = context.WithValue(ctx1, ContextKeyParamsCount, 3)

	ctx2 := context.Background()
	ctx2 = context.WithValue(ctx2, ContextKeyContract, "test_contract")
	ctx2 = context.WithValue(ctx2, ContextKeyFunction, "test_function")
	ctx2 = context.WithValue(ctx2, ContextKeyExecutionStart, executionStartTime)
	ctx2 = context.WithValue(ctx2, ContextKeyParamsCount, 5)

	stateID1, err1 := manager.generateStateID(ctx1)
	require.NoError(t, err1)

	stateID2, err2 := manager.generateStateID(ctx2)
	require.NoError(t, err2)

	assert.NotEqual(t, stateID1, stateID2, "不同参数数量应该产生不同的状态ID")
}

