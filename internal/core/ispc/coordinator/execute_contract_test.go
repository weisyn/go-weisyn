package coordinator

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	"github.com/weisyn/v1/internal/core/ispc/testutil"
	"github.com/weisyn/v1/internal/core/ispc/zkproof"
	ispcintf "github.com/weisyn/v1/pkg/interfaces/ispc"
)

// ============================================================================
// ExecuteWASMContract 和 ExecuteONNXModel 测试
// ============================================================================
//
// 🎯 **测试目的**：发现执行合约和模型推理的缺陷和BUG
//
// ============================================================================

// TestExecuteWASMContract_InvalidContractHash 测试无效合约哈希
// 🐛 **BUG检测**：空合约哈希应该返回错误
func TestExecuteWASMContract_InvalidContractHash(t *testing.T) {
	manager := createTestManager(t)

	ctx := context.Background()
	contractHash := []byte{} // 空哈希
	methodName := "test_method"
	params := []uint64{1, 2, 3}
	initParams := []byte{}
	callerAddress := "0x1234"

	result, err := manager.ExecuteWASMContract(ctx, contractHash, methodName, params, initParams, callerAddress)
	assert.Error(t, err, "空合约哈希应该返回错误")
	assert.Nil(t, result, "结果应该为nil")
	assert.True(t, errors.Is(err, ErrInvalidContractHash), "应该返回无效合约哈希错误")
}

// TestExecuteWASMContract_InvalidMethodName 测试无效方法名
// 🐛 **BUG检测**：空方法名应该返回错误
func TestExecuteWASMContract_InvalidMethodName(t *testing.T) {
	manager := createTestManager(t)

	ctx := context.Background()
	contractHash := []byte{0x12, 0x34, 0x56}
	methodName := "" // 空方法名
	params := []uint64{1, 2, 3}
	initParams := []byte{}
	callerAddress := "0x1234"

	result, err := manager.ExecuteWASMContract(ctx, contractHash, methodName, params, initParams, callerAddress)
	assert.Error(t, err, "空方法名应该返回错误")
	assert.Nil(t, result, "结果应该为nil")
	assert.True(t, errors.Is(err, ErrInvalidFunctionName), "应该返回无效函数名错误")
}

// TestExecuteWASMContract_MissingCallerAddress 测试缺少调用者地址
// 🐛 **BUG检测**：空调用者地址应该返回错误
func TestExecuteWASMContract_MissingCallerAddress(t *testing.T) {
	manager := createTestManager(t)

	ctx := context.Background()
	contractHash := []byte{0x12, 0x34, 0x56}
	methodName := "test_method"
	params := []uint64{1, 2, 3}
	initParams := []byte{}
	callerAddress := "" // 空调用者地址

	result, err := manager.ExecuteWASMContract(ctx, contractHash, methodName, params, initParams, callerAddress)
	assert.Error(t, err, "空调用者地址应该返回错误")
	assert.Nil(t, result, "结果应该为nil")
	assert.True(t, errors.Is(err, ErrMissingCallerAddress), "应该返回缺少调用者地址错误")
}

// TestExecuteWASMContract_NilContextManager 测试nil contextManager
// 🐛 **BUG检测**：nil contextManager应该返回错误
func TestExecuteWASMContract_NilContextManager(t *testing.T) {
	logger := testutil.NewTestLogger()
	configProvider := testutil.NewTestConfigProvider()

	hashManager := testutil.NewTestHashManager()
	signatureManager := testutil.NewTestSignatureManager()
	zkproofManager := zkproof.NewManager(hashManager, signatureManager, logger, configProvider)
	hostProvider := createMockHostProvider(t, logger)
	engineManager := &mockInternalEngineManager{}

	// 创建Manager，但contextManager为nil
	manager := &Manager{
		engineManager:    engineManager,
		contextManager:   nil, // nil contextManager
		zkproofManager:   zkproofManager,
		hostProvider:     hostProvider,
		logger:           logger,
		configProvider:   configProvider,
		zkProofTaskStore: make(map[string]*zkproof.ZKProofTask),
	}

	ctx := context.Background()
	contractHash := []byte{0x12, 0x34, 0x56}
	methodName := "test_method"
	params := []uint64{1, 2, 3}
	initParams := []byte{}
	callerAddress := "0x1234"

	result, err := manager.ExecuteWASMContract(ctx, contractHash, methodName, params, initParams, callerAddress)
	assert.Error(t, err, "nil contextManager应该返回错误")
	assert.Nil(t, result, "结果应该为nil")
	assert.Contains(t, err.Error(), "contextManager未初始化", "错误信息应该提到contextManager")
}

// TestExecuteWASMContract_EngineExecutionFailed 测试引擎执行失败
// 🐛 **BUG检测**：引擎执行失败应该正确传播错误
func TestExecuteWASMContract_EngineExecutionFailed(t *testing.T) {
	manager := createTestManager(t)

	// 创建会失败的引擎
	failingEngine := &failingMockEngineManager{}
	manager.engineManager = failingEngine

	ctx := context.Background()
	contractHash := []byte{0x12, 0x34, 0x56}
	methodName := "test_method"
	params := []uint64{1, 2, 3}
	initParams := []byte{}
	callerAddress := "0x1234"

	result, err := manager.ExecuteWASMContract(ctx, contractHash, methodName, params, initParams, callerAddress)
	assert.Error(t, err, "引擎执行失败应该返回错误")
	assert.Nil(t, result, "结果应该为nil")
	assert.True(t, errors.Is(err, ErrExecutionFailed), "应该返回执行失败错误")
}

// TestExecuteWASMContract_SetsContractAddress 验证执行上下文会注入合约地址
func TestExecuteWASMContract_SetsContractAddress(t *testing.T) {
	manager := createTestManager(t)
	manager.hashManager = testutil.NewTestHashManager()

	contractHash := make([]byte, 32)
	for i := range contractHash {
		contractHash[i] = byte(i + 1)
	}
	ctx := context.Background()
	methodName := "test_method"
	params := []uint64{}
	initParams := []byte{}
	callerAddress := "00112233445566778899aabbccddeeff00112233" // 20字节十六进制

	result, err := manager.ExecuteWASMContract(ctx, contractHash, methodName, params, initParams, callerAddress)
	require.NoError(t, err, "执行合约不应该失败")
	require.NotNil(t, result, "执行结果不应该为nil")

	addrValue, ok := result.ExecutionContext["contract_address"].([]byte)
	require.True(t, ok, "contract_address 应该是字节数组")
	require.Equal(t, 20, len(addrValue), "合约地址应该是20字节")

	expectedAddr, err := manager.deriveContractAddress(contractHash)
	require.NoError(t, err, "推导合约地址不应该失败")
	assert.Equal(t, expectedAddr, addrValue, "返回的合约地址应该与推导结果一致")
}

// TestExecuteONNXModel_InvalidModelHash 测试无效模型哈希
// 🐛 **BUG检测**：空模型哈希应该返回错误
func TestExecuteONNXModel_InvalidModelHash(t *testing.T) {
	manager := createTestManager(t)

	ctx := context.Background()
	modelHash := []byte{} // 空哈希
	tensorInputs := []ispcintf.TensorInput{
		{Data: []float64{1.0, 2.0}},
	}

	result, err := manager.ExecuteONNXModel(ctx, modelHash, tensorInputs)
	assert.Error(t, err, "空模型哈希应该返回错误")
	assert.Nil(t, result, "结果应该为nil")
	assert.True(t, errors.Is(err, ErrInvalidModelHash), "应该返回无效模型哈希错误")
}

// TestExecuteONNXModel_InvalidInputTensors 测试无效输入张量
// 🐛 **BUG检测**：空输入张量应该返回错误
func TestExecuteONNXModel_InvalidInputTensors(t *testing.T) {
	manager := createTestManager(t)

	ctx := context.Background()
	modelHash := []byte{0x12, 0x34, 0x56}
	tensorInputs := []ispcintf.TensorInput{} // 空输入

	result, err := manager.ExecuteONNXModel(ctx, modelHash, tensorInputs)
	assert.Error(t, err, "空输入张量应该返回错误")
	assert.Nil(t, result, "结果应该为nil")
	assert.True(t, errors.Is(err, ErrInvalidInputTensors), "应该返回无效输入张量错误")
}

// TestExecuteONNXModel_NilContextManager 测试nil contextManager
// 🐛 **BUG检测**：nil contextManager应该返回错误
func TestExecuteONNXModel_NilContextManager(t *testing.T) {
	logger := testutil.NewTestLogger()
	configProvider := testutil.NewTestConfigProvider()

	hashManager := testutil.NewTestHashManager()
	signatureManager := testutil.NewTestSignatureManager()
	zkproofManager := zkproof.NewManager(hashManager, signatureManager, logger, configProvider)
	hostProvider := createMockHostProvider(t, logger)
	engineManager := &mockInternalEngineManager{}

	// 创建Manager，但contextManager为nil
	manager := &Manager{
		engineManager:    engineManager,
		contextManager:   nil, // nil contextManager
		zkproofManager:   zkproofManager,
		hostProvider:     hostProvider,
		logger:           logger,
		configProvider:   configProvider,
		zkProofTaskStore: make(map[string]*zkproof.ZKProofTask),
	}

	ctx := context.Background()
	modelHash := []byte{0x12, 0x34, 0x56}
	tensorInputs := []ispcintf.TensorInput{
		{Data: []float64{1.0, 2.0}},
	}

	result, err := manager.ExecuteONNXModel(ctx, modelHash, tensorInputs)
	assert.Error(t, err, "nil contextManager应该返回错误")
	assert.Nil(t, result, "结果应该为nil")
	assert.Contains(t, err.Error(), "contextManager未初始化", "错误信息应该提到contextManager")
}

// TestExecuteONNXModel_EngineExecutionFailed 测试引擎执行失败
// 🐛 **BUG检测**：引擎执行失败应该正确传播错误
func TestExecuteONNXModel_EngineExecutionFailed(t *testing.T) {
	manager := createTestManager(t)

	// 创建会失败的引擎
	failingEngine := &failingMockEngineManager{}
	manager.engineManager = failingEngine

	ctx := context.Background()
	modelHash := []byte{0x12, 0x34, 0x56}
	tensorInputs := []ispcintf.TensorInput{
		{Data: []float64{1.0, 2.0}},
	}

	result, err := manager.ExecuteONNXModel(ctx, modelHash, tensorInputs)
	assert.Error(t, err, "引擎执行失败应该返回错误")
	assert.Nil(t, result, "结果应该为nil")
	assert.True(t, errors.Is(err, ErrExecutionFailed), "应该返回执行失败错误")
}

// ============================================================================
// Mock对象定义
// ============================================================================

// failingMockEngineManager Mock的失败引擎管理器
type failingMockEngineManager struct{}

func (m *failingMockEngineManager) ExecuteWASM(ctx context.Context, hash []byte, method string, params []uint64) ([]uint64, error) {
	return nil, errors.New("WASM execution failed")
}

func (m *failingMockEngineManager) ExecuteONNX(ctx context.Context, hash []byte, tensorInputs []ispcInterfaces.TensorInput) ([]ispcInterfaces.TensorOutput, error) {
	return nil, errors.New("ONNX execution failed")
}

func (m *failingMockEngineManager) Shutdown(ctx context.Context) error {
	return nil
}
