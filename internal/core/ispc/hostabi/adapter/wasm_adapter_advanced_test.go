package adapter

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero/api"

	"github.com/weisyn/v1/internal/core/ispc/testutil"
	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxopb "github.com/weisyn/v1/pb/blockchain/utxo"
	blockpb "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/types"
	"google.golang.org/grpc"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ============================================================================
// WASMAdapter高级测试 - 测试更多宿主函数
// ============================================================================
//
// 🎯 **测试目的**：发现更多宿主函数的缺陷和BUG
//
// ============================================================================

// mockEUTXOQuery Mock的EUTXOQuery
type mockEUTXOQuery struct {
	utxos []*utxopb.UTXO
	err   error
}

func (m *mockEUTXOQuery) GetUTXO(ctx context.Context, outpoint *pb.OutPoint) (*utxopb.UTXO, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.utxos) > 0 {
		return m.utxos[0], nil
	}
	return nil, nil
}

func (m *mockEUTXOQuery) GetUTXOsByAddress(ctx context.Context, address []byte, category *utxopb.UTXOCategory, includeSpent bool) ([]*utxopb.UTXO, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.utxos, nil
}

func (m *mockEUTXOQuery) GetCurrentStateRoot(ctx context.Context) ([]byte, error) {
	return []byte("mock-state-root"), nil
}

func (m *mockEUTXOQuery) GetReferenceCount(ctx context.Context, outpoint *pb.OutPoint) (uint32, error) {
	return 0, nil
}

func (m *mockEUTXOQuery) GetSponsorPoolUTXOs(ctx context.Context, includeSpent bool) ([]*utxopb.UTXO, error) {
	return nil, nil
}

// createWASMAdapterWithEUTXOQuery 创建带EUTXOQuery的WASMAdapter
func createWASMAdapterWithEUTXOQuery(t *testing.T) (*WASMAdapter, *mockHostABIForWASM, *mockEUTXOQuery) {
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

	mockEUTXO := &mockEUTXOQuery{
		utxos: []*utxopb.UTXO{
			{
				Outpoint: &pb.OutPoint{
					TxId:        make([]byte, 32),
					OutputIndex: 0,
				},
				Category:     utxopb.UTXOCategory_UTXO_CATEGORY_ASSET,
				OwnerAddress: make([]byte, 20),
				ContentStrategy: &utxopb.UTXO_CachedOutput{
					CachedOutput: &pb.TxOutput{
						OutputContent: &pb.TxOutput_Asset{
							Asset: &pb.AssetOutput{
								AssetContent: &pb.AssetOutput_NativeCoin{
									NativeCoin: &pb.NativeCoinAsset{
										Amount: "1000",
									},
								},
							},
						},
					},
				},
			},
			{
				Outpoint: &pb.OutPoint{
					TxId:        make([]byte, 32),
					OutputIndex: 1,
				},
				Category:     utxopb.UTXOCategory_UTXO_CATEGORY_ASSET,
				OwnerAddress: make([]byte, 20),
				ContentStrategy: &utxopb.UTXO_CachedOutput{
					CachedOutput: &pb.TxOutput{
						Owner: make([]byte, 20),
						LockingConditions: []*pb.LockingCondition{
							{
								Condition: &pb.LockingCondition_ContractLock{
									ContractLock: &pb.ContractLock{
										ContractAddress: append([]byte(nil), mockABI.contractAddr...),
									},
								},
							},
						},
						OutputContent: &pb.TxOutput_Asset{
							Asset: &pb.AssetOutput{
								AssetContent: &pb.AssetOutput_ContractToken{
									ContractToken: &pb.ContractTokenAsset{
										ContractAddress: append([]byte(nil), mockABI.contractAddr...),
										TokenIdentifier: &pb.ContractTokenAsset_FungibleClassId{
											FungibleClassId: []byte("token123"),
										},
										Amount: "1000",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	mockExecCtx := createMockExecutionContext()

	adapter := NewWASMAdapter(
		logger,
		nil, // chainQuery
		nil, // blockQuery
		mockEUTXO, // eutxoQuery
		nil, // uresCAS
		nil, // txQuery
		nil, // resourceQuery
		nil, // txHashClient
		nil, // addressManager
		hashManager,
		nil, // txAdapter
		nil, // draftService
		func(ctx context.Context) ispcInterfaces.ExecutionContext {
			return mockExecCtx
		},
		nil, // buildTxFromDraft
		nil, // encodeTxReceipt
	)

	return adapter, mockABI, mockEUTXO
}

// TestWASMAdapter_QueryUTXOBalance 测试query_utxo_balance函数
func TestWASMAdapter_QueryUTXOBalance(t *testing.T) {
	adapter, mockABI, _ := createWASMAdapterWithEUTXOQuery(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	queryBalance, ok := functions["query_utxo_balance"].(func(context.Context, api.Module, uint32, uint32, uint32) uint64)
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

	// 调用query_utxo_balance（无tokenID）
	result := queryBalance(ctx, module, addrPtr, 0, 0)
	assert.Equal(t, uint64(1000), result, "应该返回余额1000")
}

// TestWASMAdapter_QueryUTXOBalance_WithTokenID 测试带tokenID的query_utxo_balance
func TestWASMAdapter_QueryUTXOBalance_WithTokenID(t *testing.T) {
	adapter, mockABI, _ := createWASMAdapterWithEUTXOQuery(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	queryBalance, ok := functions["query_utxo_balance"].(func(context.Context, api.Module, uint32, uint32, uint32) uint64)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 写入地址和tokenID到内存
	addrPtr := uint32(1024)
	address := make([]byte, 20)
	memory.Write(addrPtr, address)

	tokenIDPtr := uint32(2048)
	tokenID := []byte("token123")
	memory.Write(tokenIDPtr, tokenID)

	result := queryBalance(ctx, module, addrPtr, tokenIDPtr, uint32(len(tokenID)))
	assert.Equal(t, uint64(1000), result, "应该返回余额1000")
}

// TestWASMAdapter_UTXOLookup 测试utxo_lookup函数
func TestWASMAdapter_UTXOLookup(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	utxoLookup, ok := functions["utxo_lookup"].(func(context.Context, api.Module, uint32, uint32, uint32, uint32, uint32) uint32)
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

	// 写入输出缓冲区
	outputPtr := uint32(2048)
	outputSize := uint32(1000)

	// 调用utxo_lookup
	result := utxoLookup(ctx, module, txIDPtr, 32, 0, outputPtr, outputSize)
	assert.Greater(t, result, uint32(0), "应该返回输出字节数")
}

// TestWASMAdapter_UTXOLookup_BufferTooSmall 测试缓冲区太小
func TestWASMAdapter_UTXOLookup_BufferTooSmall(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	utxoLookup, ok := functions["utxo_lookup"].(func(context.Context, api.Module, uint32, uint32, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	txIDPtr := uint32(1024)
	txID := make([]byte, 32)
	memory.Write(txIDPtr, txID)

	// 使用太小的缓冲区
	outputPtr := uint32(2048)
	outputSize := uint32(1) // 太小

	result := utxoLookup(ctx, module, txIDPtr, 32, 0, outputPtr, outputSize)
	assert.Equal(t, uint32(ErrBufferTooSmall), result, "缓冲区太小应该返回错误")
}

// TestWASMAdapter_ResourceLookup 测试resource_lookup函数
func TestWASMAdapter_ResourceLookup(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	resourceLookup, ok := functions["resource_lookup"].(func(context.Context, api.Module, uint32, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 写入contentHash到内存
	hashPtr := uint32(1024)
	contentHash := make([]byte, 32)
	contentHash[0] = 0x12
	memory.Write(hashPtr, contentHash)

	// 写入资源缓冲区
	resourcePtr := uint32(2048)
	resourceSize := uint32(1000)

	result := resourceLookup(ctx, module, hashPtr, 32, resourcePtr, resourceSize)
	assert.Greater(t, result, uint32(0), "应该返回资源字节数")
}

// TestWASMAdapter_ResourceLookup_BufferTooSmall 测试缓冲区太小
func TestWASMAdapter_ResourceLookup_BufferTooSmall(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	resourceLookup, ok := functions["resource_lookup"].(func(context.Context, api.Module, uint32, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	hashPtr := uint32(1024)
	contentHash := make([]byte, 32)
	memory.Write(hashPtr, contentHash)

	resourcePtr := uint32(2048)
	resourceSize := uint32(1) // 太小

	result := resourceLookup(ctx, module, hashPtr, 32, resourcePtr, resourceSize)
	assert.Equal(t, uint32(ErrBufferTooSmall), result, "缓冲区太小应该返回错误")
}

// TestWASMAdapter_SetReturnData 测试set_return_data函数
func TestWASMAdapter_SetReturnData(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	mockExecCtx := createMockExecutionContext()
	adapter.getExecCtxFunc = func(ctx context.Context) ispcInterfaces.ExecutionContext {
		return mockExecCtx
	}

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	setReturnData, ok := functions["set_return_data"].(func(context.Context, api.Module, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 写入返回数据到内存
	dataPtr := uint32(1024)
	data := []byte("test_return_data")
	memory.Write(dataPtr, data)

	// 调用set_return_data
	result := setReturnData(ctx, module, dataPtr, uint32(len(data)))
	assert.Equal(t, uint32(0), result, "应该返回0（成功）")

	// 验证数据被设置
	returnData, err := mockExecCtx.GetReturnData()
	require.NoError(t, err)
	assert.Equal(t, data, returnData, "应该设置正确的返回数据")
}

// TestWASMAdapter_SetReturnData_NilExecutionContext 测试nil ExecutionContext
func TestWASMAdapter_SetReturnData_NilExecutionContext(t *testing.T) {
	adapter := createTestWASMAdapter(t)
	mockABI := &mockHostABIForWASM{}

	adapter.getExecCtxFunc = func(ctx context.Context) ispcInterfaces.ExecutionContext {
		return nil
	}

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)
	setReturnData, ok := functions["set_return_data"].(func(context.Context, api.Module, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	dataPtr := uint32(1024)
	data := []byte("test")
	memory.Write(dataPtr, data)

	result := setReturnData(ctx, module, dataPtr, uint32(len(data)))
	assert.Equal(t, uint32(1), result, "nil ExecutionContext应该返回1（失败）")
}

// TestWASMAdapter_EmitEvent 测试emit_event函数
func TestWASMAdapter_EmitEvent(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	mockExecCtx := createMockExecutionContext()
	adapter.getExecCtxFunc = func(ctx context.Context) ispcInterfaces.ExecutionContext {
		return mockExecCtx
	}

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	emitEvent, ok := functions["emit_event"].(func(context.Context, api.Module, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 写入事件JSON到内存
	eventPtr := uint32(1024)
	eventJSON := []byte(`{"type":"test_event","data":{"key":"value"}}`)
	memory.Write(eventPtr, eventJSON)

	// 调用emit_event
	result := emitEvent(ctx, module, eventPtr, uint32(len(eventJSON)))
	assert.Equal(t, uint32(0), result, "应该返回0（成功）")

	// 验证事件被添加
	// 注意：emit_event的实现会将Event.Type固定为"contract_event"，而不是从JSON中解析
	events, err := mockExecCtx.GetEvents()
	require.NoError(t, err)
	assert.Equal(t, 1, len(events), "应该添加1个事件")
	assert.Equal(t, "contract_event", events[0].Type, "事件类型应该是contract_event")
	assert.NotNil(t, events[0].Data, "事件数据不应该为nil")
}

// TestWASMAdapter_EmitEvent_InvalidJSON 测试无效JSON
// 注意：emit_event的实现不会验证JSON有效性，它只是将JSON字符串存储到Event.Data中
// 因此无效JSON也会成功，这是当前实现的行为
func TestWASMAdapter_EmitEvent_InvalidJSON(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	mockExecCtx := createMockExecutionContext()
	adapter.getExecCtxFunc = func(ctx context.Context) ispcInterfaces.ExecutionContext {
		return mockExecCtx
	}

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	emitEvent, ok := functions["emit_event"].(func(context.Context, api.Module, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 写入无效JSON
	eventPtr := uint32(1024)
	invalidJSON := []byte(`{invalid json}`)
	memory.Write(eventPtr, invalidJSON)

	// 注意：emit_event不会验证JSON，所以即使无效JSON也会成功
	result := emitEvent(ctx, module, eventPtr, uint32(len(invalidJSON)))
	assert.Equal(t, uint32(0), result, "emit_event不会验证JSON有效性，所以无效JSON也会成功")
	
	// 验证事件被添加（即使JSON无效）
	events, err := mockExecCtx.GetEvents()
	require.NoError(t, err)
	assert.Equal(t, 1, len(events), "应该添加1个事件（即使JSON无效）")
}

// createMockBuildTxFromDraft 创建Mock的buildTxFromDraft函数
func createMockBuildTxFromDraft() func(context.Context, interface{}, transaction.TransactionHashServiceClient, persistence.UTXOQuery, []byte, []byte, []byte, uint64, uint64) (*TxReceipt, error) {
	return func(ctx context.Context, txAdapter interface{}, txHashClient transaction.TransactionHashServiceClient, eutxoQuery persistence.UTXOQuery, callerAddress []byte, contractAddress []byte, draftJSON []byte, blockHeight uint64, blockTimestamp uint64) (*TxReceipt, error) {
		return &TxReceipt{
			Mode: "normal",
		}, nil
	}
}

// createMockEncodeTxReceipt 创建Mock的encodeTxReceipt函数
func createMockEncodeTxReceipt() func(*TxReceipt) ([]byte, error) {
	return func(receipt *TxReceipt) ([]byte, error) {
		return json.Marshal(receipt)
	}
}

// createWASMAdapterWithBuildTx 创建带buildTxFromDraft的WASMAdapter
func createWASMAdapterWithBuildTx(t *testing.T) (*WASMAdapter, *mockHostABIForWASM) {
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
		nil, // addressManager
		hashManager,
		&mockTxAdapter{}, // txAdapter
		nil,              // draftService
		func(ctx context.Context) ispcInterfaces.ExecutionContext {
			return mockExecCtx
		},
		createMockBuildTxFromDraft(), // buildTxFromDraft
		createMockEncodeTxReceipt(),  // encodeTxReceipt
	)

	return adapter, mockABI
}

// mockTxAdapter Mock的TxAdapter
type mockTxAdapter struct{}

func (m *mockTxAdapter) FinalizeTransaction(ctx context.Context, draft interface{}) (*TxReceipt, error) {
	return &TxReceipt{Mode: "normal"}, nil
}

func (m *mockTxAdapter) CleanupDraft(ctx context.Context, draftID string) error {
	return nil
}

// TestWASMAdapter_HostBuildTransaction 测试host_build_transaction函数
func TestWASMAdapter_HostBuildTransaction(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithBuildTx(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	buildTx, ok := functions["host_build_transaction"].(func(context.Context, api.Module, uint32, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 写入Draft JSON到内存
	draftPtr := uint32(1024)
	draftJSON := []byte(`{"inputs":[],"outputs":[]}`)
	memory.Write(draftPtr, draftJSON)

	// 写入Receipt缓冲区
	receiptPtr := uint32(2048)
	receiptSize := uint32(1000)

	// 调用host_build_transaction
	result := buildTx(ctx, module, draftPtr, uint32(len(draftJSON)), receiptPtr, receiptSize)
	assert.Equal(t, uint32(0), result, "应该返回0（成功）")

	// 验证Receipt被写入
	receiptBytes, ok := memory.Read(receiptPtr, 100)
	require.True(t, ok)
	assert.Greater(t, len(receiptBytes), 0, "应该写入Receipt JSON")
}

// TestWASMAdapter_HostBuildTransaction_BufferTooSmall 测试缓冲区太小
func TestWASMAdapter_HostBuildTransaction_BufferTooSmall(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithBuildTx(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	buildTx, ok := functions["host_build_transaction"].(func(context.Context, api.Module, uint32, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	draftPtr := uint32(1024)
	draftJSON := []byte(`{"inputs":[],"outputs":[]}`)
	memory.Write(draftPtr, draftJSON)

	receiptPtr := uint32(2048)
	receiptSize := uint32(1) // 太小

	result := buildTx(ctx, module, draftPtr, uint32(len(draftJSON)), receiptPtr, receiptSize)
	assert.Equal(t, uint32(ErrBufferTooSmall), result, "缓冲区太小应该返回错误")
}

// TestWASMAdapter_HostBuildTransaction_NilTxAdapter 测试nil TxAdapter
func TestWASMAdapter_HostBuildTransaction_NilTxAdapter(t *testing.T) {
	adapter := createTestWASMAdapter(t)
	mockABI := &mockHostABIForWASM{}

	adapter.txAdapter = nil

	ctx := context.Background()
	functions := adapter.BuildHostFunctions(ctx, mockABI)
	buildTx, ok := functions["host_build_transaction"].(func(context.Context, api.Module, uint32, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	draftPtr := uint32(1024)
	draftJSON := []byte(`{"inputs":[]}`)
	memory.Write(draftPtr, draftJSON)

	result := buildTx(ctx, module, draftPtr, uint32(len(draftJSON)), 2048, 1000)
	assert.Equal(t, uint32(ErrServiceUnavailable), result, "nil TxAdapter应该返回错误")
}

// TestWASMAdapter_GetBlockHash 测试get_block_hash函数
func TestWASMAdapter_GetBlockHash(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	// 需要设置blockQuery
	adapter.blockQuery = &mockBlockQuery{}

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	getBlockHash, ok := functions["get_block_hash"].(func(context.Context, api.Module, uint64, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	hashPtr := uint32(1024)
	result := getBlockHash(ctx, module, 100, hashPtr)
	assert.Equal(t, uint32(32), result, "应该返回32（区块哈希长度）")

	// 验证哈希被写入
	hashBytes, ok := memory.Read(hashPtr, 32)
	require.True(t, ok)
	assert.Equal(t, 32, len(hashBytes), "应该写入32字节哈希")
}

// mockBlockQuery Mock的BlockQuery
type mockBlockQuery struct{}

func (m *mockBlockQuery) GetBlockByHeight(ctx context.Context, height uint64) (*blockpb.Block, error) {
	return &blockpb.Block{
		Header: &blockpb.BlockHeader{},
	}, nil
}

func (m *mockBlockQuery) GetBlockByHash(ctx context.Context, hash []byte) (*blockpb.Block, error) {
	return &blockpb.Block{
		Header: &blockpb.BlockHeader{},
	}, nil
}

func (m *mockBlockQuery) GetBlockHeader(ctx context.Context, blockHash []byte) (*blockpb.BlockHeader, error) {
	return &blockpb.BlockHeader{}, nil
}

func (m *mockBlockQuery) GetBlockRange(ctx context.Context, startHeight uint64, endHeight uint64) ([]*blockpb.Block, error) {
	return nil, nil
}

func (m *mockBlockQuery) GetHighestBlock(ctx context.Context) (uint64, []byte, error) {
	return 100, make([]byte, 32), nil
}

// TestWASMAdapter_GetTransactionID 测试get_transaction_id函数
func TestWASMAdapter_GetTransactionID(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	// 需要设置draftService和txHashClient
	adapter.draftService = &mockDraftServiceForAdapter{}
	adapter.txHashClient = &mockTxHashServiceClientForAdapter{}
	adapter.hashManager = testutil.NewTestHashManager()

	mockExecCtx := createMockExecutionContext()
	mockExecCtx.draftID = "draft-123"
	adapter.getExecCtxFunc = func(ctx context.Context) ispcInterfaces.ExecutionContext {
		return mockExecCtx
	}

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	getTxID, ok := functions["get_transaction_id"].(func(context.Context, api.Module, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	txIDPtr := uint32(1024)
	result := getTxID(ctx, module, txIDPtr)
	assert.Equal(t, uint32(32), result, "应该返回32（交易ID长度）")

	// 验证交易ID被写入
	txIDBytes, ok := memory.Read(txIDPtr, 32)
	require.True(t, ok)
	assert.Equal(t, 32, len(txIDBytes), "应该写入32字节交易ID")
}

// mockDraftServiceForAdapter Mock的DraftService
type mockDraftServiceForAdapter struct{}

func (m *mockDraftServiceForAdapter) CreateDraft(ctx context.Context) (*types.DraftTx, error) {
	return &types.DraftTx{
		DraftID: "draft-123",
		Tx:      &pb.Transaction{},
	}, nil
}

func (m *mockDraftServiceForAdapter) LoadDraft(ctx context.Context, draftID string) (*types.DraftTx, error) {
	return &types.DraftTx{
		DraftID: draftID,
		Tx:      &pb.Transaction{},
	}, nil
}

func (m *mockDraftServiceForAdapter) SaveDraft(ctx context.Context, draft *types.DraftTx) error {
	return nil
}

func (m *mockDraftServiceForAdapter) GetDraftByID(ctx context.Context, draftID string) (*types.DraftTx, error) {
	return &types.DraftTx{
		DraftID: draftID,
		Tx:      &pb.Transaction{},
	}, nil
}

func (m *mockDraftServiceForAdapter) ValidateDraft(ctx context.Context, draft *types.DraftTx) error {
	return nil
}

func (m *mockDraftServiceForAdapter) SealDraft(ctx context.Context, draft *types.DraftTx) (*types.ComposedTx, error) {
	return nil, nil
}

func (m *mockDraftServiceForAdapter) DeleteDraft(ctx context.Context, draftID string) error {
	return nil
}

func (m *mockDraftServiceForAdapter) AddInput(ctx context.Context, draft *types.DraftTx, outpoint *pb.OutPoint, isReferenceOnly bool, unlockingProof *pb.UnlockingProof) (uint32, error) {
	return 0, nil
}

func (m *mockDraftServiceForAdapter) AddAssetOutput(ctx context.Context, draft *types.DraftTx, owner []byte, amount string, tokenID []byte, lockingConditions []*pb.LockingCondition) (uint32, error) {
	return 0, nil
}

func (m *mockDraftServiceForAdapter) AddResourceOutput(ctx context.Context, draft *types.DraftTx, contentHash []byte, category string, owner []byte, lockingConditions []*pb.LockingCondition, metadata []byte) (uint32, error) {
	return 0, nil
}

func (m *mockDraftServiceForAdapter) AddStateOutput(ctx context.Context, draft *types.DraftTx, stateID []byte, stateVersion uint64, executionResultHash []byte, publicInputs []byte, parentStateHash []byte) (uint32, error) {
	return 0, nil
}

// mockTxHashServiceClientForAdapter Mock的TransactionHashServiceClient
type mockTxHashServiceClientForAdapter struct{}

func (m *mockTxHashServiceClientForAdapter) ComputeHash(ctx context.Context, in *transaction.ComputeHashRequest, opts ...grpc.CallOption) (*transaction.ComputeHashResponse, error) {
	return &transaction.ComputeHashResponse{
		Hash: make([]byte, 32),
	}, nil
}

func (m *mockTxHashServiceClientForAdapter) ValidateHash(ctx context.Context, in *transaction.ValidateHashRequest, opts ...grpc.CallOption) (*transaction.ValidateHashResponse, error) {
	return &transaction.ValidateHashResponse{
		IsValid: true,
	}, nil
}

func (m *mockTxHashServiceClientForAdapter) ComputeSignatureHash(ctx context.Context, in *transaction.ComputeSignatureHashRequest, opts ...grpc.CallOption) (*transaction.ComputeSignatureHashResponse, error) {
	return &transaction.ComputeSignatureHashResponse{
		Hash: make([]byte, 32),
	}, nil
}

func (m *mockTxHashServiceClientForAdapter) ValidateSignatureHash(ctx context.Context, in *transaction.ValidateSignatureHashRequest, opts ...grpc.CallOption) (*transaction.ValidateSignatureHashResponse, error) {
	return &transaction.ValidateSignatureHashResponse{
		IsValid: true,
	}, nil
}

