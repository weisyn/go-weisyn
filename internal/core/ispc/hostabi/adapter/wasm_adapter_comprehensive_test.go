package adapter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero/api"
	"google.golang.org/protobuf/proto"

	"github.com/weisyn/v1/internal/core/ispc/interfaces"
	"github.com/weisyn/v1/internal/core/ispc/testutil"
	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/types"
	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ============================================================================
// WASMAdapter综合测试 - 测试更多宿主函数
// ============================================================================
//
// 🎯 **测试目的**：发现更多宿主函数的缺陷和BUG
//
// ============================================================================

// mockAddressManager Mock的AddressManager
type mockAddressManager struct {
	bytesToAddressFunc func([]byte) (string, error)
	addressToBytesFunc func(string) ([]byte, error)
}

func (m *mockAddressManager) BytesToAddress(bytes []byte) (string, error) {
	if m.bytesToAddressFunc != nil {
		return m.bytesToAddressFunc(bytes)
	}
	return "test_address_base58", nil
}

func (m *mockAddressManager) AddressToBytes(address string) ([]byte, error) {
	if m.addressToBytesFunc != nil {
		return m.addressToBytesFunc(address)
	}
	return make([]byte, 20), nil
}

func (m *mockAddressManager) PrivateKeyToAddress(privateKey []byte) (string, error) { return "test_address", nil }
func (m *mockAddressManager) PublicKeyToAddress(publicKey []byte) (string, error) { return "test_address", nil }
func (m *mockAddressManager) AddressToHexString(address string) (string, error) { return "", nil }
func (m *mockAddressManager) HexStringToAddress(hex string) (string, error) { return "", nil }
func (m *mockAddressManager) CompareAddresses(addr1, addr2 string) (bool, error) { return true, nil }
func (m *mockAddressManager) GetAddressType(address string) (crypto.AddressType, error) { return crypto.AddressTypeBitcoin, nil }
func (m *mockAddressManager) IsZeroAddress(address string) bool { return false }
func (m *mockAddressManager) StringToAddress(s string) (string, error) { return "", nil }
func (m *mockAddressManager) ValidateAddress(address string) (bool, error) { return true, nil }

// createWASMAdapterWithAddressManager 创建带AddressManager的WASMAdapter
func createWASMAdapterWithAddressManager(t *testing.T) (*WASMAdapter, *mockHostABIForWASM, *mockAddressManager) {
	t.Helper()

	logger := testutil.NewTestLogger()
	hashManager := testutil.NewTestHashManager()
	mockABI := &mockHostABIForWASM{
		blockHeight:    100,
		blockTimestamp: 1234567890,
		chainID:        []byte("test-chain"),
		caller:         make([]byte, 20),
		contractAddr:   make([]byte, 20),
		txID:           make([]byte, 32),
		utxoExists:     true,
		resourceExists: true,
	}

	mockAddressMgr := &mockAddressManager{}
	mockExecCtx := createMockExecutionContext()

	adapter := NewWASMAdapter(
		logger,
		nil, // chainQuery
		nil, // blockQuery
		nil, // eutxoQuery
		nil, // uresCAS
		nil, // txQuery
		nil, // resourceQuery
		nil, // txHashClient
		mockAddressMgr, // addressManager
		hashManager,
		nil, // txAdapter
		nil, // draftService
		func(ctx context.Context) ispcInterfaces.ExecutionContext {
			return mockExecCtx
		},
		nil, // buildTxFromDraft
		nil, // encodeTxReceipt
	)

	return adapter, mockABI, mockAddressMgr
}

// createMockTransactionDraft 创建Mock的TransactionDraft
func createMockTransactionDraft() *ispcInterfaces.TransactionDraft {
	return &ispcInterfaces.TransactionDraft{
		Tx: &pb.Transaction{
			Inputs:  []*pb.TxInput{},
			Outputs: []*pb.TxOutput{},
		},
	}
}

// createMockExecutionContextWithDraft 创建带Draft的Mock ExecutionContext
func createMockExecutionContextWithDraft() *mockExecutionContextWithDraft {
	return &mockExecutionContextWithDraft{
		callerAddress:    make([]byte, 20),
		contractAddress:  make([]byte, 20),
		txID:             make([]byte, 32),
		chainID:          []byte("test-chain"),
		blockHeight:      100,
		blockTimestamp:   1234567890,
		draftID:          "draft-123",
		initParams:       []byte("init-params"),
		draft:            createMockTransactionDraft(),
	}
}

// mockExecutionContextWithDraft Mock的ExecutionContext（带Draft）
type mockExecutionContextWithDraft struct {
	callerAddress   []byte
	contractAddress []byte
	txID            []byte
	chainID         []byte
	blockHeight     uint64
	blockTimestamp  uint64
	draftID         string
	initParams      []byte
	draft           *ispcInterfaces.TransactionDraft
}

func (m *mockExecutionContextWithDraft) GetCallerAddress() []byte { return m.callerAddress }
func (m *mockExecutionContextWithDraft) GetContractAddress() []byte { return m.contractAddress }
func (m *mockExecutionContextWithDraft) GetTransactionID() []byte { return m.txID }
func (m *mockExecutionContextWithDraft) GetChainID() []byte { return m.chainID }
func (m *mockExecutionContextWithDraft) GetBlockHeight() uint64 { return m.blockHeight }
func (m *mockExecutionContextWithDraft) GetBlockTimestamp() uint64 { return m.blockTimestamp }
func (m *mockExecutionContextWithDraft) GetDraftID() string { return m.draftID }
func (m *mockExecutionContextWithDraft) GetInitParams() ([]byte, error) { return m.initParams, nil }
func (m *mockExecutionContextWithDraft) GetTransactionDraft() (*ispcInterfaces.TransactionDraft, error) {
	return m.draft, nil
}
func (m *mockExecutionContextWithDraft) UpdateTransactionDraft(draft *ispcInterfaces.TransactionDraft) error {
	m.draft = draft
	return nil
}

// 实现其他必需的方法（最小实现）
func (m *mockExecutionContextWithDraft) GetExecutionID() string { return "exec-123" }
func (m *mockExecutionContextWithDraft) GetExecutionTrace() ([]*ispcInterfaces.HostFunctionCall, error) { return nil, nil }
func (m *mockExecutionContextWithDraft) RecordHostFunctionCall(call *ispcInterfaces.HostFunctionCall) {}
func (m *mockExecutionContextWithDraft) RecordStateChange(key string, oldValue interface{}, newValue interface{}) error { return nil }
func (m *mockExecutionContextWithDraft) RecordTraceRecords(records []ispcInterfaces.TraceRecord) error { return nil }
func (m *mockExecutionContextWithDraft) GetResourceUsage() *types.ResourceUsage { return nil }
func (m *mockExecutionContextWithDraft) FinalizeResourceUsage() {}
func (m *mockExecutionContextWithDraft) SetReturnData(data []byte) error { return nil }
func (m *mockExecutionContextWithDraft) GetReturnData() ([]byte, error) { return nil, nil }
func (m *mockExecutionContextWithDraft) AddEvent(event *ispcInterfaces.Event) error { return nil }
func (m *mockExecutionContextWithDraft) GetEvents() ([]*ispcInterfaces.Event, error) { return nil, nil }
func (m *mockExecutionContextWithDraft) HostABI() interfaces.HostABI { return nil }
func (m *mockExecutionContextWithDraft) SetHostABI(abi interfaces.HostABI) error { return nil }
func (m *mockExecutionContextWithDraft) SetInitParams(params []byte) error { return nil }

// TestWASMAdapter_AppendTxInput 测试append_tx_input函数
func TestWASMAdapter_AppendTxInput(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	appendTxInput, ok := functions["append_tx_input"].(func(context.Context, api.Module, uint32, uint32, uint32, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 写入txID到内存
	txIDPtr := uint32(1024)
	txID := make([]byte, 32)
	txID[0] = 0x12
	memory.Write(txIDPtr, txID)

	// 调用append_tx_input（无proof）
	// 注意：mockHostABIForWASM的TxAddInput返回0，这是有效的（第一个输入的索引是0）
	result := appendTxInput(ctx, module, txIDPtr, 32, 0, 0, 0, 0)
	assert.Equal(t, uint32(0), result, "应该返回输入索引（0是有效的第一个输入索引）")
}

// TestWASMAdapter_AppendTxInput_WithProof 测试带proof的append_tx_input
func TestWASMAdapter_AppendTxInput_WithProof(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	appendTxInput, ok := functions["append_tx_input"].(func(context.Context, api.Module, uint32, uint32, uint32, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 写入txID到内存
	txIDPtr := uint32(1024)
	txID := make([]byte, 32)
	txID[0] = 0x12
	memory.Write(txIDPtr, txID)

	// 写入proof到内存
	proofPtr := uint32(2048)
	proof := &pb.UnlockingProof{}
	proofBytes, err := proto.Marshal(proof)
	require.NoError(t, err)
	memory.Write(proofPtr, proofBytes)

	// 调用append_tx_input（带proof）
	result := appendTxInput(ctx, module, txIDPtr, 32, 0, 0, proofPtr, uint32(len(proofBytes)))
	assert.Equal(t, uint32(0), result, "应该返回输入索引（0是有效的第一个输入索引）")
}

// TestWASMAdapter_AppendTxInput_InvalidLength 测试无效txID长度
func TestWASMAdapter_AppendTxInput_InvalidLength(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	appendTxInput, ok := functions["append_tx_input"].(func(context.Context, api.Module, uint32, uint32, uint32, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	// 使用无效的txID长度
	result := appendTxInput(ctx, module, 1024, 20, 0, 0, 0, 0) // 长度应该是32
	assert.Equal(t, uint32(ErrInvalidParameter), result, "无效长度应该返回错误")
}

// TestWASMAdapter_AppendAssetOutput 测试append_asset_output函数
func TestWASMAdapter_AppendAssetOutput(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	appendAssetOutput, ok := functions["append_asset_output"].(func(context.Context, api.Module, uint32, uint32, uint64, uint32, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 写入owner到内存
	ownerPtr := uint32(1024)
	owner := make([]byte, 20)
	owner[0] = 0x12
	memory.Write(ownerPtr, owner)

	// 调用append_asset_output（无tokenID和lock）
	result := appendAssetOutput(ctx, module, ownerPtr, 20, 1000, 0, 0, 0, 0)
	assert.Equal(t, uint32(0), result, "应该返回输出索引（0是有效的第一个输出索引）")
}

// TestWASMAdapter_AppendAssetOutput_InvalidOwnerLength 测试无效owner长度
func TestWASMAdapter_AppendAssetOutput_InvalidOwnerLength(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	appendAssetOutput, ok := functions["append_asset_output"].(func(context.Context, api.Module, uint32, uint32, uint64, uint32, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	// 使用无效的owner长度
	result := appendAssetOutput(ctx, module, 1024, 19, 1000, 0, 0, 0, 0) // 长度应该是20
	assert.Equal(t, uint32(ErrInvalidAddress), result, "无效长度应该返回错误")
}

// TestWASMAdapter_StateGet 测试state_get函数
func TestWASMAdapter_StateGet(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	// 创建带Draft的ExecutionContext
	mockExecCtx := createMockExecutionContextWithDraft()
	adapter.getExecCtxFunc = func(ctx context.Context) ispcInterfaces.ExecutionContext {
		return mockExecCtx
	}

	// 添加一个StateOutput到draft
	stateOutput := &pb.TxOutput{
		OutputContent: &pb.TxOutput_State{
			State: &pb.StateOutput{
				StateId:             []byte("test_key"),
				StateVersion:        1,
				ExecutionResultHash: []byte("test_value"),
			},
		},
	}
	mockExecCtx.draft.Tx.Outputs = append(mockExecCtx.draft.Tx.Outputs, stateOutput)

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	stateGet, ok := functions["state_get"].(func(context.Context, api.Module, uint32, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 写入key到内存
	keyPtr := uint32(1024)
	key := []byte("test_key")
	memory.Write(keyPtr, key)

	// 写入value缓冲区
	valuePtr := uint32(2048)
	valueLen := uint32(100)

	// 调用state_get
	result := stateGet(ctx, module, keyPtr, uint32(len(key)), valuePtr, valueLen)
	assert.Equal(t, uint32(0), result, "应该返回0（成功）")

	// 验证value被写入
	// 注意：memory.Read可能返回对齐后的数据，只读取实际写入的长度
	valueBytes, ok := memory.Read(valuePtr, 10) // "test_value"的实际长度
	require.True(t, ok)
	assert.Equal(t, []byte("test_value"), valueBytes[:10], "应该写入正确的value")
}

// TestWASMAdapter_StateGet_NotFound 测试state_get未找到
func TestWASMAdapter_StateGet_NotFound(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	mockExecCtx := createMockExecutionContextWithDraft()
	adapter.getExecCtxFunc = func(ctx context.Context) ispcInterfaces.ExecutionContext {
		return mockExecCtx
	}

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	stateGet, ok := functions["state_get"].(func(context.Context, api.Module, uint32, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 写入不存在的key
	keyPtr := uint32(1024)
	key := []byte("nonexistent_key")
	memory.Write(keyPtr, key)

	valuePtr := uint32(2048)
	result := stateGet(ctx, module, keyPtr, uint32(len(key)), valuePtr, 100)
	assert.Equal(t, uint32(1), result, "未找到应该返回1（失败）")
}

// TestWASMAdapter_StateSet 测试state_set函数
func TestWASMAdapter_StateSet(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	mockExecCtx := createMockExecutionContextWithDraft()
	adapter.getExecCtxFunc = func(ctx context.Context) ispcInterfaces.ExecutionContext {
		return mockExecCtx
	}

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	stateSet, ok := functions["state_set"].(func(context.Context, api.Module, uint32, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 写入key和value到内存
	keyPtr := uint32(1024)
	key := []byte("test_key")
	memory.Write(keyPtr, key)

	valuePtr := uint32(2048)
	value := []byte("test_value")
	memory.Write(valuePtr, value)

	// 调用state_set
	result := stateSet(ctx, module, keyPtr, uint32(len(key)), valuePtr, uint32(len(value)))
	assert.Equal(t, uint32(0), result, "应该返回0（成功）")

	// 验证draft被更新
	draft, err := mockExecCtx.GetTransactionDraft()
	require.NoError(t, err)
	assert.NotNil(t, draft.Tx, "draft.Tx不应该为nil")
	assert.Equal(t, 1, len(draft.Tx.Outputs), "应该有1个输出")
}

// TestWASMAdapter_StateExists 测试state_exists函数
func TestWASMAdapter_StateExists(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	mockExecCtx := createMockExecutionContextWithDraft()
	// 添加一个StateOutput
	stateOutput := &pb.TxOutput{
		OutputContent: &pb.TxOutput_State{
			State: &pb.StateOutput{
				StateId: []byte("test_key"),
			},
		},
	}
	mockExecCtx.draft.Tx.Outputs = append(mockExecCtx.draft.Tx.Outputs, stateOutput)

	adapter.getExecCtxFunc = func(ctx context.Context) ispcInterfaces.ExecutionContext {
		return mockExecCtx
	}

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	stateExists, ok := functions["state_exists"].(func(context.Context, api.Module, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 写入key到内存
	keyPtr := uint32(1024)
	key := []byte("test_key")
	memory.Write(keyPtr, key)

	// 调用state_exists
	result := stateExists(ctx, module, keyPtr, uint32(len(key)))
	assert.Equal(t, uint32(1), result, "应该返回1（存在）")
}

// TestWASMAdapter_StateExists_NotFound 测试state_exists未找到
func TestWASMAdapter_StateExists_NotFound(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	mockExecCtx := createMockExecutionContextWithDraft()
	adapter.getExecCtxFunc = func(ctx context.Context) ispcInterfaces.ExecutionContext {
		return mockExecCtx
	}

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	stateExists, ok := functions["state_exists"].(func(context.Context, api.Module, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 写入不存在的key
	keyPtr := uint32(1024)
	key := []byte("nonexistent_key")
	memory.Write(keyPtr, key)

	result := stateExists(ctx, module, keyPtr, uint32(len(key)))
	assert.Equal(t, uint32(0), result, "应该返回0（不存在）")
}

// TestWASMAdapter_AddressBytesToBase58 测试address_bytes_to_base58函数
func TestWASMAdapter_AddressBytesToBase58(t *testing.T) {
	adapter, mockABI, _ := createWASMAdapterWithAddressManager(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	bytesToBase58, ok := functions["address_bytes_to_base58"].(func(context.Context, api.Module, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 写入地址到内存
	addrPtr := uint32(1024)
	address := make([]byte, 20)
	address[0] = 0x12
	memory.Write(addrPtr, address)

	// 写入结果缓冲区
	resultPtr := uint32(2048)
	maxLen := uint32(100)

	// 调用address_bytes_to_base58
	result := bytesToBase58(ctx, module, addrPtr, resultPtr, maxLen)
	assert.Greater(t, result, uint32(0), "应该返回Base58字符串长度")

	// 验证结果被写入
	resultBytes, ok := memory.Read(resultPtr, result)
	require.True(t, ok)
	assert.Equal(t, int(result), len(resultBytes), "应该写入Base58字符串")
}

// TestWASMAdapter_AddressBytesToBase58_NilAddressManager 测试nil AddressManager
func TestWASMAdapter_AddressBytesToBase58_NilAddressManager(t *testing.T) {
	adapter := createTestWASMAdapter(t)
	mockABI := &mockHostABIForWASM{}

	// 设置nil AddressManager
	adapter.addressManager = nil

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)
	bytesToBase58, ok := functions["address_bytes_to_base58"].(func(context.Context, api.Module, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	addrPtr := uint32(1024)
	address := make([]byte, 20)
	memory.Write(addrPtr, address)

	resultPtr := uint32(2048)
	result := bytesToBase58(ctx, module, addrPtr, resultPtr, 100)
	assert.Equal(t, uint32(0), result, "nil AddressManager应该返回0")
}

// TestWASMAdapter_AddressBytesToBase58_BufferTooSmall 测试缓冲区太小
func TestWASMAdapter_AddressBytesToBase58_BufferTooSmall(t *testing.T) {
	adapter, mockABI, _ := createWASMAdapterWithAddressManager(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	bytesToBase58, ok := functions["address_bytes_to_base58"].(func(context.Context, api.Module, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	addrPtr := uint32(1024)
	address := make([]byte, 20)
	memory.Write(addrPtr, address)

	resultPtr := uint32(2048)
	// 使用太小的缓冲区
	result := bytesToBase58(ctx, module, addrPtr, resultPtr, 5)
	assert.Equal(t, uint32(0), result, "缓冲区太小应该返回0")
}

// TestWASMAdapter_AddressBase58ToBytes 测试address_base58_to_bytes函数
func TestWASMAdapter_AddressBase58ToBytes(t *testing.T) {
	adapter, mockABI, _ := createWASMAdapterWithAddressManager(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	base58ToBytes, ok := functions["address_base58_to_bytes"].(func(context.Context, api.Module, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 写入Base58字符串到内存
	base58Ptr := uint32(1024)
	base58Str := "test_address_base58"
	memory.Write(base58Ptr, []byte(base58Str))

	// 写入结果缓冲区
	resultPtr := uint32(2048)

	// 调用address_base58_to_bytes
	// 注意：根据实现，address_base58_to_bytes返回1表示成功，而不是字节数
	result := base58ToBytes(ctx, module, base58Ptr, uint32(len(base58Str)), resultPtr)
	assert.Equal(t, uint32(1), result, "应该返回1（成功标志）")

	// 验证结果被写入
	addressBytes, ok := memory.Read(resultPtr, 20)
	require.True(t, ok)
	assert.Equal(t, 20, len(addressBytes), "应该写入20字节地址")
}

// TestWASMAdapter_AddressBase58ToBytes_InvalidAddress 测试无效地址
func TestWASMAdapter_AddressBase58ToBytes_InvalidAddress(t *testing.T) {
	adapter, mockABI, _ := createWASMAdapterWithAddressManager(t)
	ctx := context.Background()

	// 创建返回错误的AddressManager
	mockAddrMgr := &mockAddressManager{
		addressToBytesFunc: func(address string) ([]byte, error) {
			return nil, assert.AnError
		},
	}
	adapter.addressManager = mockAddrMgr

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	base58ToBytes, ok := functions["address_base58_to_bytes"].(func(context.Context, api.Module, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	base58Ptr := uint32(1024)
	base58Str := "invalid_address"
	memory.Write(base58Ptr, []byte(base58Str))

	resultPtr := uint32(2048)
	result := base58ToBytes(ctx, module, base58Ptr, uint32(len(base58Str)), resultPtr)
	assert.Equal(t, uint32(0), result, "无效地址应该返回0")
}

