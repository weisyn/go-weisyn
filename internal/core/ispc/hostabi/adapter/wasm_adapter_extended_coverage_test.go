package adapter

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero/api"
	"google.golang.org/protobuf/proto"

	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxopb "github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ============================================================================
// WASMAdapter扩展覆盖率测试 - 提高覆盖率到80%+
// ============================================================================
//
// 🎯 **测试目的**：发现更多宿主函数的缺陷和BUG，提高覆盖率
//
// ============================================================================

// TestWASMAdapter_AppendResourceOutput 测试append_resource_output完整流程
func TestWASMAdapter_AppendResourceOutput(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	appendResourceOutput, ok := functions["append_resource_output"].(func(context.Context, api.Module, uint32, uint32, uint32, uint32, uint32, uint32, uint64) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 准备资源JSON数据
	resourceData := map[string]interface{}{
		"content_hash": hex.EncodeToString(make([]byte, 32)),
		"category":     "wasm",
		"metadata":     hex.EncodeToString([]byte("test-metadata")),
	}
	resourceJSON, err := json.Marshal(resourceData)
	require.NoError(t, err)

	// 写入资源JSON到内存
	resourcePtr := uint32(1024)
	memory.Write(resourcePtr, resourceJSON)

	// 写入owner到内存
	ownerPtr := uint32(2048)
	owner := make([]byte, 20)
	owner[0] = 0x12
	memory.Write(ownerPtr, owner)

	// 调用append_resource_output（无lock）
	result := appendResourceOutput(ctx, module, resourcePtr, uint32(len(resourceJSON)), ownerPtr, 20, 0, 0, 1234567890)
	assert.Equal(t, uint32(0), result, "应该返回输出索引（0是有效的第一个输出索引）")
}

// TestWASMAdapter_AppendResourceOutput_WithLock 测试带lock的append_resource_output
func TestWASMAdapter_AppendResourceOutput_WithLock(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	appendResourceOutput, ok := functions["append_resource_output"].(func(context.Context, api.Module, uint32, uint32, uint32, uint32, uint32, uint32, uint64) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 准备资源JSON数据
	resourceData := map[string]interface{}{
		"content_hash": hex.EncodeToString(make([]byte, 32)),
		"category":     "wasm",
	}
	resourceJSON, err := json.Marshal(resourceData)
	require.NoError(t, err)

	resourcePtr := uint32(1024)
	memory.Write(resourcePtr, resourceJSON)

	ownerPtr := uint32(2048)
	owner := make([]byte, 20)
	memory.Write(ownerPtr, owner)

	// 写入lock到内存
	lockPtr := uint32(3072)
	lock := &pb.LockingCondition{}
	lockBytes, err := proto.Marshal(lock)
	require.NoError(t, err)
	memory.Write(lockPtr, lockBytes)

	result := appendResourceOutput(ctx, module, resourcePtr, uint32(len(resourceJSON)), ownerPtr, 20, lockPtr, uint32(len(lockBytes)), 1234567890)
	assert.Equal(t, uint32(0), result, "应该返回输出索引")
}

// TestWASMAdapter_AppendResourceOutput_InvalidJSON 测试无效JSON
func TestWASMAdapter_AppendResourceOutput_InvalidJSON(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	appendResourceOutput, ok := functions["append_resource_output"].(func(context.Context, api.Module, uint32, uint32, uint32, uint32, uint32, uint32, uint64) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	resourcePtr := uint32(1024)
	invalidJSON := []byte(`{invalid json}`)
	memory.Write(resourcePtr, invalidJSON)

	ownerPtr := uint32(2048)
	owner := make([]byte, 20)
	memory.Write(ownerPtr, owner)

	result := appendResourceOutput(ctx, module, resourcePtr, uint32(len(invalidJSON)), ownerPtr, 20, 0, 0, 0)
	assert.Equal(t, uint32(ErrEncodingFailed), result, "无效JSON应该返回错误")
}

// TestWASMAdapter_AppendResourceOutput_InvalidContentHash 测试无效contentHash
func TestWASMAdapter_AppendResourceOutput_InvalidContentHash(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	appendResourceOutput, ok := functions["append_resource_output"].(func(context.Context, api.Module, uint32, uint32, uint32, uint32, uint32, uint32, uint64) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 使用无效的contentHash（长度不是32字节）
	resourceData := map[string]interface{}{
		"content_hash": hex.EncodeToString(make([]byte, 20)), // 20字节，不是32字节
		"category":     "wasm",
	}
	resourceJSON, err := json.Marshal(resourceData)
	require.NoError(t, err)

	resourcePtr := uint32(1024)
	memory.Write(resourcePtr, resourceJSON)

	ownerPtr := uint32(2048)
	owner := make([]byte, 20)
	memory.Write(ownerPtr, owner)

	result := appendResourceOutput(ctx, module, resourcePtr, uint32(len(resourceJSON)), ownerPtr, 20, 0, 0, 0)
	assert.Equal(t, uint32(ErrInvalidHash), result, "无效contentHash应该返回错误")
}

// TestWASMAdapter_AppendResourceOutput_InvalidMetadata 测试无效metadata
func TestWASMAdapter_AppendResourceOutput_InvalidMetadata(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	appendResourceOutput, ok := functions["append_resource_output"].(func(context.Context, api.Module, uint32, uint32, uint32, uint32, uint32, uint32, uint64) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 使用无效的metadata（不是有效的hex字符串）
	resourceData := map[string]interface{}{
		"content_hash": hex.EncodeToString(make([]byte, 32)),
		"category":     "wasm",
		"metadata":     "invalid-hex",
	}
	resourceJSON, err := json.Marshal(resourceData)
	require.NoError(t, err)

	resourcePtr := uint32(1024)
	memory.Write(resourcePtr, resourceJSON)

	ownerPtr := uint32(2048)
	owner := make([]byte, 20)
	memory.Write(ownerPtr, owner)

	result := appendResourceOutput(ctx, module, resourcePtr, uint32(len(resourceJSON)), ownerPtr, 20, 0, 0, 0)
	assert.Equal(t, uint32(ErrEncodingFailed), result, "无效metadata应该返回错误")
}

// TestWASMAdapter_AppendStateOutput 测试append_state_output完整流程
func TestWASMAdapter_AppendStateOutput(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	appendStateOutput, ok := functions["append_state_output"].(func(context.Context, api.Module, uint32, uint32, uint64, uint32, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 写入stateID到内存
	stateIDPtr := uint32(1024)
	stateID := []byte("test_state_id")
	memory.Write(stateIDPtr, stateID)

	// 写入executionResultHash到内存
	resultHashPtr := uint32(2048)
	resultHash := make([]byte, 32)
	resultHash[0] = 0x12
	memory.Write(resultHashPtr, resultHash)

	// 调用append_state_output（无publicInputs和parentStateHash）
	result := appendStateOutput(ctx, module, stateIDPtr, uint32(len(stateID)), 1, resultHashPtr, 0, 0, 0)
	assert.Equal(t, uint32(0), result, "应该返回输出索引（0是有效的第一个输出索引）")
}

// TestWASMAdapter_AppendStateOutput_WithPublicInputs 测试带publicInputs的append_state_output
func TestWASMAdapter_AppendStateOutput_WithPublicInputs(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	appendStateOutput, ok := functions["append_state_output"].(func(context.Context, api.Module, uint32, uint32, uint64, uint32, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	stateIDPtr := uint32(1024)
	stateID := []byte("test_state_id")
	memory.Write(stateIDPtr, stateID)

	resultHashPtr := uint32(2048)
	resultHash := make([]byte, 32)
	memory.Write(resultHashPtr, resultHash)

	// 写入publicInputs到内存
	publicInputsPtr := uint32(3072)
	publicInputs := []byte("public-inputs")
	memory.Write(publicInputsPtr, publicInputs)

	result := appendStateOutput(ctx, module, stateIDPtr, uint32(len(stateID)), 1, resultHashPtr, publicInputsPtr, uint32(len(publicInputs)), 0)
	assert.Equal(t, uint32(0), result, "应该返回输出索引")
}

// TestWASMAdapter_AppendStateOutput_WithParentHash 测试带parentStateHash的append_state_output
func TestWASMAdapter_AppendStateOutput_WithParentHash(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	appendStateOutput, ok := functions["append_state_output"].(func(context.Context, api.Module, uint32, uint32, uint64, uint32, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	stateIDPtr := uint32(1024)
	stateID := []byte("test_state_id")
	memory.Write(stateIDPtr, stateID)

	resultHashPtr := uint32(2048)
	resultHash := make([]byte, 32)
	memory.Write(resultHashPtr, resultHash)

	// 写入parentStateHash到内存
	parentHashPtr := uint32(3072)
	parentHash := make([]byte, 32)
	parentHash[0] = 0x34
	memory.Write(parentHashPtr, parentHash)

	result := appendStateOutput(ctx, module, stateIDPtr, uint32(len(stateID)), 1, resultHashPtr, 0, 0, parentHashPtr)
	assert.Equal(t, uint32(0), result, "应该返回输出索引")
}

// TestWASMAdapter_AppendStateOutput_InvalidResultHashPtr 测试无效resultHashPtr
func TestWASMAdapter_AppendStateOutput_InvalidResultHashPtr(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	appendStateOutput, ok := functions["append_state_output"].(func(context.Context, api.Module, uint32, uint32, uint64, uint32, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	stateIDPtr := uint32(1024)
	stateID := []byte("test_state_id")
	module.Memory().Write(stateIDPtr, stateID)

	// 使用无效的resultHashPtr（0）
	result := appendStateOutput(ctx, module, stateIDPtr, uint32(len(stateID)), 1, 0, 0, 0, 0)
	assert.Equal(t, uint32(ErrInvalidParameter), result, "无效resultHashPtr应该返回错误")
}

// TestWASMAdapter_HostBuildTransaction_BuildTxFailed 测试buildTxFromDraft失败
func TestWASMAdapter_HostBuildTransaction_BuildTxFailed(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithBuildTx(t)
	ctx := context.Background()

	// 设置buildTxFromDraft返回错误
	adapter.buildTxFromDraft = func(ctx context.Context, txAdapter interface{}, txHashClient transaction.TransactionHashServiceClient, eutxoQuery persistence.UTXOQuery, callerAddress []byte, contractAddress []byte, draftJSON []byte, blockHeight uint64, blockTimestamp uint64) (*TxReceipt, error) {
		return nil, assert.AnError
	}

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
	result := buildTx(ctx, module, draftPtr, uint32(len(draftJSON)), receiptPtr, 1000)
	assert.Equal(t, uint32(ErrInternalError), result, "buildTxFromDraft失败应该返回错误")
}

// TestWASMAdapter_HostBuildTransaction_EncodeFailed 测试encodeTxReceipt失败
func TestWASMAdapter_HostBuildTransaction_EncodeFailed(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithBuildTx(t)
	ctx := context.Background()

	// 设置encodeTxReceipt返回错误
	adapter.encodeTxReceipt = func(receipt *TxReceipt) ([]byte, error) {
		return nil, assert.AnError
	}

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
	result := buildTx(ctx, module, draftPtr, uint32(len(draftJSON)), receiptPtr, 1000)
	assert.Equal(t, uint32(ErrEncodingFailed), result, "encodeTxReceipt失败应该返回错误")
}

// TestWASMAdapter_HostBuildTransaction_NilBuildTxFromDraft 测试nil buildTxFromDraft
func TestWASMAdapter_HostBuildTransaction_NilBuildTxFromDraft(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	adapter.txAdapter = &mockTxAdapter{}
	adapter.buildTxFromDraft = nil // 设置为nil

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
	assert.Equal(t, uint32(ErrServiceUnavailable), result, "nil buildTxFromDraft应该返回错误")
}

// TestWASMAdapter_HostBuildTransaction_NilEncodeTxReceipt 测试nil encodeTxReceipt
func TestWASMAdapter_HostBuildTransaction_NilEncodeTxReceipt(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithBuildTx(t)
	ctx := context.Background()

	adapter.encodeTxReceipt = nil // 设置为nil

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

	result := buildTx(ctx, module, draftPtr, uint32(len(draftJSON)), 2048, 1000)
	assert.Equal(t, uint32(ErrServiceUnavailable), result, "nil encodeTxReceipt应该返回错误")
}

// TestWASMAdapter_QueryUTXOBalance_WithTokenIDFilter 测试带tokenID过滤的query_utxo_balance
func TestWASMAdapter_QueryUTXOBalance_WithTokenIDFilter(t *testing.T) {
	adapter, mockABI, mockEUTXO := createWASMAdapterWithEUTXOQuery(t)
	ctx := context.Background()

	// 添加多个UTXO，包括原生币和代币
	mockEUTXO.utxos = []*utxopb.UTXO{
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
					OutputContent: &pb.TxOutput_Asset{
						Asset: &pb.AssetOutput{
							AssetContent: &pb.AssetOutput_NativeCoin{
								NativeCoin: &pb.NativeCoinAsset{
									Amount: "2000",
								},
							},
						},
					},
				},
			},
		},
	}

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	queryBalance, ok := functions["query_utxo_balance"].(func(context.Context, api.Module, uint32, uint32, uint32) uint64)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	addrPtr := uint32(1024)
	address := make([]byte, 20)
	memory.Write(addrPtr, address)

	// 调用query_utxo_balance（无tokenID，应该返回所有原生币余额）
	result := queryBalance(ctx, module, addrPtr, 0, 0)
	assert.Equal(t, uint64(3000), result, "应该返回所有原生币余额（1000+2000）")
}

// TestWASMAdapter_QueryUTXOBalance_QueryError 测试查询错误
func TestWASMAdapter_QueryUTXOBalance_QueryError(t *testing.T) {
	adapter, mockABI, mockEUTXO := createWASMAdapterWithEUTXOQuery(t)
	ctx := context.Background()

	// 设置查询返回错误
	mockEUTXO.err = assert.AnError

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	queryBalance, ok := functions["query_utxo_balance"].(func(context.Context, api.Module, uint32, uint32, uint32) uint64)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	addrPtr := uint32(1024)
	address := make([]byte, 20)
	memory.Write(addrPtr, address)

	result := queryBalance(ctx, module, addrPtr, 0, 0)
	assert.Equal(t, uint64(0), result, "查询错误应该返回0")
}

// TestWASMAdapter_QueryUTXOBalance_InvalidAmount 测试无效金额字符串
func TestWASMAdapter_QueryUTXOBalance_InvalidAmount(t *testing.T) {
	adapter, mockABI, mockEUTXO := createWASMAdapterWithEUTXOQuery(t)
	ctx := context.Background()

	// 添加一个金额无效的UTXO
	mockEUTXO.utxos = []*utxopb.UTXO{
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
									Amount: "invalid-amount", // 无效的金额字符串
								},
							},
						},
					},
				},
			},
		},
	}

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	queryBalance, ok := functions["query_utxo_balance"].(func(context.Context, api.Module, uint32, uint32, uint32) uint64)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	addrPtr := uint32(1024)
	address := make([]byte, 20)
	memory.Write(addrPtr, address)

	result := queryBalance(ctx, module, addrPtr, 0, 0)
	assert.Equal(t, uint64(0), result, "无效金额应该被忽略，返回0")
}

// TestWASMAdapter_QueryUTXOBalance_NoCachedOutput 测试没有缓存输出的UTXO
func TestWASMAdapter_QueryUTXOBalance_NoCachedOutput(t *testing.T) {
	adapter, mockABI, mockEUTXO := createWASMAdapterWithEUTXOQuery(t)
	ctx := context.Background()

	// 添加一个没有缓存输出的UTXO
	mockEUTXO.utxos = []*utxopb.UTXO{
		{
			Outpoint: &pb.OutPoint{
				TxId:        make([]byte, 32),
				OutputIndex: 0,
			},
			Category:     utxopb.UTXOCategory_UTXO_CATEGORY_ASSET,
			OwnerAddress: make([]byte, 20),
			// 没有ContentStrategy，表示没有缓存输出
		},
	}

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	queryBalance, ok := functions["query_utxo_balance"].(func(context.Context, api.Module, uint32, uint32, uint32) uint64)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	addrPtr := uint32(1024)
	address := make([]byte, 20)
	memory.Write(addrPtr, address)

	result := queryBalance(ctx, module, addrPtr, 0, 0)
	assert.Equal(t, uint64(0), result, "没有缓存输出的UTXO应该被忽略，返回0")
}

// TestWASMAdapter_AppendResourceOutput_ZeroResourceLen 测试零长度resource
func TestWASMAdapter_AppendResourceOutput_ZeroResourceLen(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	appendResourceOutput, ok := functions["append_resource_output"].(func(context.Context, api.Module, uint32, uint32, uint32, uint32, uint32, uint32, uint64) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	ownerPtr := uint32(2048)
	owner := make([]byte, 20)
	module.Memory().Write(ownerPtr, owner)

	result := appendResourceOutput(ctx, module, 1024, 0, ownerPtr, 20, 0, 0, 0)
	assert.Equal(t, uint32(ErrInvalidParameter), result, "零长度resource应该返回错误")
}

// TestWASMAdapter_AppendResourceOutput_InvalidOwnerLength 测试无效owner长度
func TestWASMAdapter_AppendResourceOutput_InvalidOwnerLength(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	appendResourceOutput, ok := functions["append_resource_output"].(func(context.Context, api.Module, uint32, uint32, uint32, uint32, uint32, uint32, uint64) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	resourceData := map[string]interface{}{
		"content_hash": hex.EncodeToString(make([]byte, 32)),
		"category":     "wasm",
	}
	resourceJSON, err := json.Marshal(resourceData)
	require.NoError(t, err)

	resourcePtr := uint32(1024)
	memory.Write(resourcePtr, resourceJSON)

	ownerPtr := uint32(2048)
	owner := make([]byte, 19) // 19字节，不是20字节
	memory.Write(ownerPtr, owner)

	result := appendResourceOutput(ctx, module, resourcePtr, uint32(len(resourceJSON)), ownerPtr, 19, 0, 0, 0)
	assert.Equal(t, uint32(ErrInvalidAddress), result, "无效owner长度应该返回错误")
}

// TestWASMAdapter_AppendResourceOutput_InvalidLock 测试无效lock
func TestWASMAdapter_AppendResourceOutput_InvalidLock(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	appendResourceOutput, ok := functions["append_resource_output"].(func(context.Context, api.Module, uint32, uint32, uint32, uint32, uint32, uint32, uint64) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	resourceData := map[string]interface{}{
		"content_hash": hex.EncodeToString(make([]byte, 32)),
		"category":     "wasm",
	}
	resourceJSON, err := json.Marshal(resourceData)
	require.NoError(t, err)

	resourcePtr := uint32(1024)
	memory.Write(resourcePtr, resourceJSON)

	ownerPtr := uint32(2048)
	owner := make([]byte, 20)
	memory.Write(ownerPtr, owner)

	// 写入无效的lock（不是有效的protobuf）
	lockPtr := uint32(3072)
	invalidLock := []byte("invalid-protobuf")
	memory.Write(lockPtr, invalidLock)

	result := appendResourceOutput(ctx, module, resourcePtr, uint32(len(resourceJSON)), ownerPtr, 20, lockPtr, uint32(len(invalidLock)), 0)
	assert.Equal(t, uint32(ErrEncodingFailed), result, "无效lock应该返回错误")
}

// TestWASMAdapter_AppendStateOutput_ZeroStateIDLen 测试零长度stateID
func TestWASMAdapter_AppendStateOutput_ZeroStateIDLen(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	appendStateOutput, ok := functions["append_state_output"].(func(context.Context, api.Module, uint32, uint32, uint64, uint32, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	resultHashPtr := uint32(2048)
	resultHash := make([]byte, 32)
	module.Memory().Write(resultHashPtr, resultHash)

	result := appendStateOutput(ctx, module, 1024, 0, 1, resultHashPtr, 0, 0, 0)
	assert.Equal(t, uint32(ErrInvalidParameter), result, "零长度stateID应该返回错误")
}

// TestWASMAdapter_AppendStateOutput_ReadStateIDFailed 测试读取stateID失败
func TestWASMAdapter_AppendStateOutput_ReadStateIDFailed(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	appendStateOutput, ok := functions["append_state_output"].(func(context.Context, api.Module, uint32, uint32, uint64, uint32, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 使用一个超出内存范围的指针
	memSize := memory.Size()
	stateIDPtr := uint32(memSize + 100) // 超出范围

	resultHashPtr := uint32(2048)
	resultHash := make([]byte, 32)
	memory.Write(resultHashPtr, resultHash)

	result := appendStateOutput(ctx, module, stateIDPtr, 10, 1, resultHashPtr, 0, 0, 0)
	assert.Equal(t, uint32(ErrMemoryAccessFailed), result, "读取stateID失败应该返回错误")
}

// TestWASMAdapter_AppendStateOutput_ReadResultHashFailed 测试读取resultHash失败
func TestWASMAdapter_AppendStateOutput_ReadResultHashFailed(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	appendStateOutput, ok := functions["append_state_output"].(func(context.Context, api.Module, uint32, uint32, uint64, uint32, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	stateIDPtr := uint32(1024)
	stateID := []byte("test_state_id")
	memory.Write(stateIDPtr, stateID)

	// 使用一个超出内存范围的指针
	memSize := memory.Size()
	resultHashPtr := uint32(memSize + 100) // 超出范围

	result := appendStateOutput(ctx, module, stateIDPtr, uint32(len(stateID)), 1, resultHashPtr, 0, 0, 0)
	assert.Equal(t, uint32(ErrMemoryAccessFailed), result, "读取resultHash失败应该返回错误")
}

// TestWASMAdapter_AppendStateOutput_ReadParentHashFailed 测试读取parentHash失败
func TestWASMAdapter_AppendStateOutput_ReadParentHashFailed(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithMock(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	appendStateOutput, ok := functions["append_state_output"].(func(context.Context, api.Module, uint32, uint32, uint64, uint32, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	stateIDPtr := uint32(1024)
	stateID := []byte("test_state_id")
	memory.Write(stateIDPtr, stateID)

	resultHashPtr := uint32(2048)
	resultHash := make([]byte, 32)
	memory.Write(resultHashPtr, resultHash)

	// 使用一个超出内存范围的指针
	memSize := memory.Size()
	parentHashPtr := uint32(memSize + 100) // 超出范围

	result := appendStateOutput(ctx, module, stateIDPtr, uint32(len(stateID)), 1, resultHashPtr, 0, 0, parentHashPtr)
	assert.Equal(t, uint32(ErrMemoryAccessFailed), result, "读取parentHash失败应该返回错误")
}

// TestWASMAdapter_HostBuildTransaction_ReadDraftFailed 测试读取Draft JSON失败
func TestWASMAdapter_HostBuildTransaction_ReadDraftFailed(t *testing.T) {
	adapter, mockABI := createWASMAdapterWithBuildTx(t)
	ctx := context.Background()

	functions := adapter.BuildHostFunctions(ctx, mockABI)
	buildTx, ok := functions["host_build_transaction"].(func(context.Context, api.Module, uint32, uint32, uint32, uint32) uint32)
	require.True(t, ok)

	module, cleanup := createWazeroModule(t, functions)
	defer cleanup()

	memory := module.Memory()
	require.NotNil(t, memory)

	// 使用一个超出内存范围的指针
	memSize := memory.Size()
	draftPtr := uint32(memSize + 100) // 超出范围

	receiptPtr := uint32(2048)
	result := buildTx(ctx, module, draftPtr, 100, receiptPtr, 1000)
	assert.Equal(t, uint32(ErrInvalidParameter), result, "读取Draft JSON失败应该返回错误")
}

// TestWASMAdapter_HostBuildTransaction_WriteReceiptFailed 测试写入Receipt失败
func TestWASMAdapter_HostBuildTransaction_WriteReceiptFailed(t *testing.T) {
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

	// 使用一个超出内存范围的指针
	memSize := memory.Size()
	receiptPtr := uint32(memSize + 100) // 超出范围

	result := buildTx(ctx, module, draftPtr, uint32(len(draftJSON)), receiptPtr, 1000)
	assert.Equal(t, uint32(ErrMemoryAccessFailed), result, "写入Receipt失败应该返回错误")
}

