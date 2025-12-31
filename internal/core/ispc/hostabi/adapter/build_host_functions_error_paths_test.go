package adapter

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	pbresource "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
)

// ============================================================================
// BuildHostFunctions 错误路径测试
// ============================================================================
//
// 🎯 **测试目的**：发现 BuildHostFunctions 中错误处理路径的缺陷和BUG
//
// ============================================================================

// mockHostABIWithErrors Mock的HostABI，返回错误
type mockHostABIWithErrors struct {
	getBlockHeightError    error
	getBlockTimestampError error
	getChainIDError        error
}

func (m *mockHostABIWithErrors) GetBlockHeight(ctx context.Context) (uint64, error) {
	if m.getBlockHeightError != nil {
		return 0, m.getBlockHeightError
	}
	return 100, nil
}

func (m *mockHostABIWithErrors) GetBlockTimestamp(ctx context.Context) (uint64, error) {
	if m.getBlockTimestampError != nil {
		return 0, m.getBlockTimestampError
	}
	return 1234567890, nil
}

func (m *mockHostABIWithErrors) GetChainID(ctx context.Context) ([]byte, error) {
	if m.getChainIDError != nil {
		return nil, m.getChainIDError
	}
	return []byte("test-chain"), nil
}

// 实现其他必需的方法（最小实现）
func (m *mockHostABIWithErrors) GetBlockHash(ctx context.Context, height uint64) ([]byte, error) { return nil, nil }
func (m *mockHostABIWithErrors) GetCaller(ctx context.Context) ([]byte, error)                    { return nil, nil }
func (m *mockHostABIWithErrors) GetContractAddress(ctx context.Context) ([]byte, error)          { return nil, nil }
func (m *mockHostABIWithErrors) GetTransactionID(ctx context.Context) ([]byte, error)            { return nil, nil }
func (m *mockHostABIWithErrors) UTXOLookup(ctx context.Context, outpoint *pb.OutPoint) (*pb.TxOutput, error) {
	return nil, nil
}
func (m *mockHostABIWithErrors) UTXOExists(ctx context.Context, outpoint *pb.OutPoint) (bool, error) { return false, nil }
func (m *mockHostABIWithErrors) ResourceLookup(ctx context.Context, contentHash []byte) (*pbresource.Resource, error) {
	return nil, nil
}
func (m *mockHostABIWithErrors) ResourceExists(ctx context.Context, contentHash []byte) (bool, error) {
	return false, nil
}
func (m *mockHostABIWithErrors) TxAddInput(ctx context.Context, outpoint *pb.OutPoint, isReferenceOnly bool, unlockingProof *pb.UnlockingProof) (uint32, error) {
	return 0, nil
}
func (m *mockHostABIWithErrors) TxAddAssetOutput(ctx context.Context, owner []byte, amount uint64, tokenID []byte, lockingConditions []*pb.LockingCondition) (uint32, error) {
	return 0, nil
}
func (m *mockHostABIWithErrors) TxAddResourceOutput(ctx context.Context, contentHash []byte, category string, owner []byte, lockingConditions []*pb.LockingCondition, metadata []byte) (uint32, error) {
	return 0, nil
}
func (m *mockHostABIWithErrors) TxAddStateOutput(ctx context.Context, stateID []byte, stateVersion uint64, executionResultHash []byte, publicInputs []byte, parentStateHash []byte) (uint32, error) {
	return 0, nil
}
func (m *mockHostABIWithErrors) EmitEvent(ctx context.Context, eventType string, eventData []byte) error { return nil }
func (m *mockHostABIWithErrors) LogDebug(ctx context.Context, message string) error                      { return nil }

// TestBuildHostFunctions_GetBlockHeight_Error 测试 get_block_height 错误处理
func TestBuildHostFunctions_GetBlockHeight_Error(t *testing.T) {
	adapter, _ := createWASMAdapterWithMock(t)
	mockABI := &mockHostABIWithErrors{
		getBlockHeightError: errors.New("获取区块高度失败"),
	}

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	getBlockHeight, ok := functions["get_block_height"].(func() uint64)
	require.True(t, ok, "get_block_height应该是func() uint64类型")

	height := getBlockHeight()
	// 🔧 **修复后**：使用 math.MaxUint64 表示错误
	assert.Equal(t, uint64(math.MaxUint64), height, "错误时应该返回 math.MaxUint64")
}

// TestBuildHostFunctions_GetBlockTimestamp_Error 测试 get_block_timestamp 错误处理
func TestBuildHostFunctions_GetBlockTimestamp_Error(t *testing.T) {
	adapter, _ := createWASMAdapterWithMock(t)
	mockABI := &mockHostABIWithErrors{
		getBlockTimestampError: errors.New("获取区块时间戳失败"),
	}

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	getBlockTimestamp, ok := functions["get_block_timestamp"].(func() uint64)
	require.True(t, ok, "get_block_timestamp应该是func() uint64类型")

	timestamp := getBlockTimestamp()
	// 🔧 **修复后**：使用 math.MaxUint64 表示错误
	assert.Equal(t, uint64(math.MaxUint64), timestamp, "错误时应该返回 math.MaxUint64")
}

// TestBuildHostFunctions_GetChainID_Error 测试 get_chain_id 错误处理
// 注意：get_chain_id 实际上从 ExecutionContext 获取链ID，而不是从 HostABI
// 所以 HostABI 的错误不会影响 get_chain_id 的行为
func TestBuildHostFunctions_GetChainID_Error(t *testing.T) {
	adapter, _ := createWASMAdapterWithMock(t)
	mockABI := &mockHostABIWithErrors{
		getChainIDError: errors.New("获取链ID失败"),
	}

	// 创建mock ExecutionContext返回空链ID（这才是实际测试的错误路径）
	mockExecCtx := &mockExecutionContext{
		chainID: []byte{}, // 空链ID
	}

	adapter.getExecCtxFunc = func(ctx context.Context) ispcInterfaces.ExecutionContext {
		return mockExecCtx
	}

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	getChainID, ok := functions["get_chain_id"].(func(context.Context, api.Module, uint32) uint32)
	require.True(t, ok, "get_chain_id应该是func(context.Context, api.Module, uint32) uint32类型")

	// 创建WASM模块用于测试
	wasmBytes := []byte{
		0x00, 0x61, 0x73, 0x6d,
		0x01, 0x00, 0x00, 0x00,
		0x05, 0x03, 0x01, 0x00, 0x01,
	}

	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	compiled, err := runtime.CompileModule(ctx, wasmBytes)
	require.NoError(t, err)

	module, err := runtime.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithName("test_module"))
	require.NoError(t, err)
	defer module.Close(ctx)

	memory := module.Memory()
	require.NotNil(t, memory)

	// 测试错误路径（空链ID）
	result := getChainID(ctx, module, 0)
	assert.Equal(t, uint32(ErrInternalError), result, "空链ID应该返回ErrInternalError")
}

// TestBuildHostFunctions_GetChainID_EmptyChainID 测试 get_chain_id 空链ID
func TestBuildHostFunctions_GetChainID_EmptyChainID(t *testing.T) {
	adapter, _ := createWASMAdapterWithMock(t)
	mockABI := &mockHostABIWithErrors{} // 不设置错误，但返回空链ID

	// 创建mock ExecutionContext返回空链ID
	mockExecCtx := &mockExecutionContext{
		chainID: []byte{}, // 空链ID
	}

	adapter.getExecCtxFunc = func(ctx context.Context) ispcInterfaces.ExecutionContext {
		return mockExecCtx
	}

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	getChainID, ok := functions["get_chain_id"].(func(context.Context, api.Module, uint32) uint32)
	require.True(t, ok)

	wasmBytes := []byte{
		0x00, 0x61, 0x73, 0x6d,
		0x01, 0x00, 0x00, 0x00,
		0x05, 0x03, 0x01, 0x00, 0x01,
	}

	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	compiled, err := runtime.CompileModule(ctx, wasmBytes)
	require.NoError(t, err)

	module, err := runtime.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithName("test_module"))
	require.NoError(t, err)
	defer module.Close(ctx)

	memory := module.Memory()
	require.NotNil(t, memory)

	// 测试空链ID路径
	result := getChainID(ctx, module, 0)
	assert.Equal(t, uint32(ErrInternalError), result, "空链ID应该返回ErrInternalError")
}

// TestBuildHostFunctions_GetBlockHeight_Success 测试 get_block_height 成功路径
func TestBuildHostFunctions_GetBlockHeight_Success(t *testing.T) {
	adapter, _ := createWASMAdapterWithMock(t)
	mockABI := &mockHostABIWithErrors{} // 不设置错误

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	getBlockHeight, ok := functions["get_block_height"].(func() uint64)
	require.True(t, ok)

	height := getBlockHeight()
	assert.Equal(t, uint64(100), height, "成功时应该返回正确的区块高度")
}

// TestBuildHostFunctions_GetBlockTimestamp_Success 测试 get_block_timestamp 成功路径
func TestBuildHostFunctions_GetBlockTimestamp_Success(t *testing.T) {
	adapter, _ := createWASMAdapterWithMock(t)
	mockABI := &mockHostABIWithErrors{} // 不设置错误

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	getBlockTimestamp, ok := functions["get_block_timestamp"].(func() uint64)
	require.True(t, ok)

	timestamp := getBlockTimestamp()
	assert.Equal(t, uint64(1234567890), timestamp, "成功时应该返回正确的时间戳")
}

// TestBuildHostFunctions_GetChainID_Success 测试 get_chain_id 成功路径
func TestBuildHostFunctions_GetChainID_Success(t *testing.T) {
	adapter, _ := createWASMAdapterWithMock(t)
	mockABI := &mockHostABIWithErrors{} // 不设置错误

	// 创建mock ExecutionContext返回有效链ID
	mockExecCtx := &mockExecutionContext{
		chainID: []byte("test-chain"),
	}

	adapter.getExecCtxFunc = func(ctx context.Context) ispcInterfaces.ExecutionContext {
		return mockExecCtx
	}

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)

	getChainID, ok := functions["get_chain_id"].(func(context.Context, api.Module, uint32) uint32)
	require.True(t, ok)

	wasmBytes := []byte{
		0x00, 0x61, 0x73, 0x6d,
		0x01, 0x00, 0x00, 0x00,
		0x05, 0x03, 0x01, 0x00, 0x01,
	}

	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	compiled, err := runtime.CompileModule(ctx, wasmBytes)
	require.NoError(t, err)

	module, err := runtime.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithName("test_module"))
	require.NoError(t, err)
	defer module.Close(ctx)

	memory := module.Memory()
	require.NotNil(t, memory)

	// 分配内存用于写入链ID
	chainIDPtr := uint32(100)
	result := getChainID(ctx, module, chainIDPtr)
	assert.Equal(t, uint32(len("test-chain")), result, "成功时应该返回链ID长度")

	// 验证内存中写入的数据
	chainIDBytes, ok := memory.Read(chainIDPtr, uint32(len("test-chain")))
	require.True(t, ok, "应该能够读取写入的链ID")
	assert.Equal(t, []byte("test-chain"), chainIDBytes, "内存中的链ID应该正确")
}

