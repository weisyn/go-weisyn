package hostabi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/ispc/testutil"
	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	utxo "github.com/weisyn/v1/pb/blockchain/utxo"
	core "github.com/weisyn/v1/pb/blockchain/block"
	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	pb_resource "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
	"github.com/weisyn/v1/pkg/types"
)

// ============================================================================
// HostRuntimePorts 测试
// ============================================================================
//
// 🎯 **测试目的**：发现 HostRuntimePorts 的缺陷和BUG
//
// ============================================================================

// TestNewHostRuntimePorts 测试创建HostRuntimePorts
func TestNewHostRuntimePorts(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockChainQuery := &mockChainQueryForHostABI{}
	mockBlockQuery := &mockBlockQueryForHostABI{}
	mockUTXOQuery := &mockUTXOQueryForHostABI{}
	mockCASStorage := &mockCASStorageForHostABI{}
	mockTxQuery := &mockTxQueryForHostABI{}
	mockResourceQuery := &mockResourceQueryForHostABI{}
	mockDraftService := &mockDraftServiceForHostABI{}
	mockHashManager := testutil.NewTestHashManager()
	mockExecCtx := createMockExecutionContextForHostABI()

	hostABI, err := NewHostRuntimePorts(
		logger,
		mockChainQuery,
		mockBlockQuery,
		mockUTXOQuery,
		mockCASStorage,
		mockTxQuery,
		mockResourceQuery,
		mockDraftService,
		mockHashManager,
		mockExecCtx,
	)

	assert.NoError(t, err, "应该成功创建")
	assert.NotNil(t, hostABI, "应该返回HostABI实例")
}

// TestNewHostRuntimePorts_NilDependencies 测试nil依赖
func TestNewHostRuntimePorts_NilDependencies(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockChainQuery := &mockChainQueryForHostABI{}
	mockBlockQuery := &mockBlockQueryForHostABI{}
	mockUTXOQuery := &mockUTXOQueryForHostABI{}
	mockCASStorage := &mockCASStorageForHostABI{}
	mockTxQuery := &mockTxQueryForHostABI{}
	mockResourceQuery := &mockResourceQueryForHostABI{}
	mockDraftService := &mockDraftServiceForHostABI{}
	mockHashManager := testutil.NewTestHashManager()
	mockExecCtx := createMockExecutionContextForHostABI()

	// 测试nil chainQuery
	_, err := NewHostRuntimePorts(logger, nil, mockBlockQuery, mockUTXOQuery, mockCASStorage, mockTxQuery, mockResourceQuery, mockDraftService, mockHashManager, mockExecCtx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "chainQuery 不能为 nil")

	// 测试nil blockQuery
	_, err = NewHostRuntimePorts(logger, mockChainQuery, nil, mockUTXOQuery, mockCASStorage, mockTxQuery, mockResourceQuery, mockDraftService, mockHashManager, mockExecCtx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "blockQuery 不能为 nil")

	// 测试nil eutxoQuery
	_, err = NewHostRuntimePorts(logger, mockChainQuery, mockBlockQuery, nil, mockCASStorage, mockTxQuery, mockResourceQuery, mockDraftService, mockHashManager, mockExecCtx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "eutxoQuery 不能为 nil")

	// 测试nil uresCAS
	_, err = NewHostRuntimePorts(logger, mockChainQuery, mockBlockQuery, mockUTXOQuery, nil, mockTxQuery, mockResourceQuery, mockDraftService, mockHashManager, mockExecCtx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "uresCAS 不能为 nil")

	// 测试nil txQuery
	_, err = NewHostRuntimePorts(logger, mockChainQuery, mockBlockQuery, mockUTXOQuery, mockCASStorage, nil, mockResourceQuery, mockDraftService, mockHashManager, mockExecCtx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "txQuery 不能为 nil")

	// 测试nil resourceQuery
	_, err = NewHostRuntimePorts(logger, mockChainQuery, mockBlockQuery, mockUTXOQuery, mockCASStorage, mockTxQuery, nil, mockDraftService, mockHashManager, mockExecCtx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resourceQuery 不能为 nil")

	// 测试nil draftService
	_, err = NewHostRuntimePorts(logger, mockChainQuery, mockBlockQuery, mockUTXOQuery, mockCASStorage, mockTxQuery, mockResourceQuery, nil, mockHashManager, mockExecCtx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "draftService 不能为 nil")

	// 测试nil hashManager
	_, err = NewHostRuntimePorts(logger, mockChainQuery, mockBlockQuery, mockUTXOQuery, mockCASStorage, mockTxQuery, mockResourceQuery, mockDraftService, nil, mockExecCtx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "hashManager 不能为 nil")

	// 测试nil execCtx
	_, err = NewHostRuntimePorts(logger, mockChainQuery, mockBlockQuery, mockUTXOQuery, mockCASStorage, mockTxQuery, mockResourceQuery, mockDraftService, mockHashManager, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "执行上下文不能为 nil")
}

// TestHostRuntimePorts_GetBlockHeight 测试GetBlockHeight
func TestHostRuntimePorts_GetBlockHeight(t *testing.T) {
	hostABI := createTestHostRuntimePorts(t)
	ctx := context.Background()

	height, err := hostABI.GetBlockHeight(ctx)

	assert.NoError(t, err, "应该成功")
	assert.Equal(t, uint64(100), height, "应该返回正确的区块高度")
}

// TestHostRuntimePorts_GetBlockHeight_ChainQueryError 测试chainQuery错误
func TestHostRuntimePorts_GetBlockHeight_ChainQueryError(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockChainQuery := &mockChainQueryForHostABI{err: assert.AnError}
	mockUTXOQuery := &mockUTXOQueryForHostABI{}
	mockCASStorage := &mockCASStorageForHostABI{}
	mockTxQuery := &mockTxQueryForHostABI{}
	mockResourceQuery := &mockResourceQueryForHostABI{}
	mockDraftService := &mockDraftServiceForHostABI{}
	mockHashManager := testutil.NewTestHashManager()
	mockExecCtx := createMockExecutionContextForHostABI()

	hostABI, err := NewHostRuntimePorts(
		logger,
		mockChainQuery,
		&mockBlockQueryForHostABI{},
		mockUTXOQuery,
		mockCASStorage,
		mockTxQuery,
		mockResourceQuery,
		mockDraftService,
		mockHashManager,
		mockExecCtx,
	)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = hostABI.GetBlockHeight(ctx)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "获取链信息失败", "错误信息应该正确")
}

// TestHostRuntimePorts_GetBlockTimestamp 测试GetBlockTimestamp
func TestHostRuntimePorts_GetBlockTimestamp(t *testing.T) {
	hostABI := createTestHostRuntimePorts(t)
	ctx := context.Background()

	timestamp, err := hostABI.GetBlockTimestamp(ctx)

	assert.NoError(t, err, "应该成功")
	assert.Equal(t, uint64(1234567890), timestamp, "应该返回正确的时间戳")
}

// TestHostRuntimePorts_GetChainID 测试GetChainID
func TestHostRuntimePorts_GetChainID(t *testing.T) {
	hostABI := createTestHostRuntimePorts(t)
	ctx := context.Background()

	chainID, err := hostABI.GetChainID(ctx)

	assert.NoError(t, err, "应该成功")
	assert.Equal(t, []byte("test-chain"), chainID, "应该返回正确的链ID")
}

// TestHostRuntimePorts_GetCaller 测试GetCaller
func TestHostRuntimePorts_GetCaller(t *testing.T) {
	hostABI := createTestHostRuntimePorts(t)
	ctx := context.Background()

	caller, err := hostABI.GetCaller(ctx)

	assert.NoError(t, err, "应该成功")
	assert.Equal(t, 20, len(caller), "应该返回20字节的调用者地址")
}

// TestHostRuntimePorts_GetContractAddress 测试GetContractAddress
func TestHostRuntimePorts_GetContractAddress(t *testing.T) {
	hostABI := createTestHostRuntimePorts(t)
	ctx := context.Background()

	contractAddr, err := hostABI.GetContractAddress(ctx)

	assert.NoError(t, err, "应该成功")
	assert.Equal(t, 20, len(contractAddr), "应该返回20字节的合约地址")
}

// TestHostRuntimePorts_GetTransactionID 测试GetTransactionID
func TestHostRuntimePorts_GetTransactionID(t *testing.T) {
	hostABI := createTestHostRuntimePorts(t)
	ctx := context.Background()

	txID, err := hostABI.GetTransactionID(ctx)

	assert.NoError(t, err, "应该成功")
	assert.Equal(t, 32, len(txID), "应该返回32字节的交易ID")
}

// TestHostRuntimePorts_GetBlockHash_CurrentHeight 测试GetBlockHash（当前高度）
func TestHostRuntimePorts_GetBlockHash_CurrentHeight(t *testing.T) {
	hostABI := createTestHostRuntimePorts(t)
	ctx := context.Background()

	hash, err := hostABI.GetBlockHash(ctx, 100) // 当前高度

	assert.NoError(t, err, "应该成功")
	assert.Equal(t, 32, len(hash), "应该返回32字节的区块哈希")
}

// TestHostRuntimePorts_GetBlockHash_HistoricalBlock 测试GetBlockHash（历史区块）
func TestHostRuntimePorts_GetBlockHash_HistoricalBlock(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockChainQuery := &mockChainQueryForHostABIWithBlockQuery{}
	mockUTXOQuery := &mockUTXOQueryForHostABI{}
	mockCASStorage := &mockCASStorageForHostABI{}
	mockTxQuery := &mockTxQueryForHostABI{}
	mockResourceQuery := &mockResourceQueryForHostABI{}
	mockDraftService := &mockDraftServiceForHostABI{}
	mockHashManager := testutil.NewTestHashManager()
	mockExecCtx := createMockExecutionContextForHostABI()

	hostABI, err := NewHostRuntimePorts(
		logger,
		mockChainQuery,
		&mockBlockQueryForHostABI{},
		mockUTXOQuery,
		mockCASStorage,
		mockTxQuery,
		mockResourceQuery,
		mockDraftService,
		mockHashManager,
		mockExecCtx,
	)
	require.NoError(t, err)

	ctx := context.Background()
	hash, err := hostABI.GetBlockHash(ctx, 50) // 历史区块

	assert.NoError(t, err, "应该成功")
	assert.Equal(t, 32, len(hash), "应该返回32字节的区块哈希")
}

// TestHostRuntimePorts_GetBlockHash_ChainQueryError 测试chainQuery错误
func TestHostRuntimePorts_GetBlockHash_ChainQueryError(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockChainQuery := &mockChainQueryForHostABI{err: assert.AnError}
	mockUTXOQuery := &mockUTXOQueryForHostABI{}
	mockCASStorage := &mockCASStorageForHostABI{}
	mockTxQuery := &mockTxQueryForHostABI{}
	mockResourceQuery := &mockResourceQueryForHostABI{}
	mockDraftService := &mockDraftServiceForHostABI{}
	mockHashManager := testutil.NewTestHashManager()
	mockExecCtx := createMockExecutionContextForHostABI()

	hostABI, err := NewHostRuntimePorts(
		logger,
		mockChainQuery,
		&mockBlockQueryForHostABI{},
		mockUTXOQuery,
		mockCASStorage,
		mockTxQuery,
		mockResourceQuery,
		mockDraftService,
		mockHashManager,
		mockExecCtx,
	)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = hostABI.GetBlockHash(ctx, 50)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "获取链信息失败", "错误信息应该正确")
}

// TestHostRuntimePorts_GetBlockHash_BlockQueryError 测试blockQuery返回错误
func TestHostRuntimePorts_GetBlockHash_BlockQueryError(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockChainQuery := &mockChainQueryForHostABI{}
	mockBlockQuery := &mockBlockQueryForHostABI{err: assert.AnError}
	mockUTXOQuery := &mockUTXOQueryForHostABI{}
	mockCASStorage := &mockCASStorageForHostABI{}
	mockTxQuery := &mockTxQueryForHostABI{}
	mockResourceQuery := &mockResourceQueryForHostABI{}
	mockDraftService := &mockDraftServiceForHostABI{}
	mockHashManager := testutil.NewTestHashManager()
	mockExecCtx := createMockExecutionContextForHostABI()

	hostABI, err := NewHostRuntimePorts(
		logger,
		mockChainQuery,
		mockBlockQuery,
		mockUTXOQuery,
		mockCASStorage,
		mockTxQuery,
		mockResourceQuery,
		mockDraftService,
		mockHashManager,
		mockExecCtx,
	)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = hostABI.GetBlockHash(ctx, 50) // 历史区块，但blockQuery返回错误

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "查询历史区块失败", "错误信息应该正确")
}

// TestHostRuntimePorts_UTXOLookup_NilOutpoint 测试nil outpoint
func TestHostRuntimePorts_UTXOLookup_NilOutpoint(t *testing.T) {
	hostABI := createTestHostRuntimePorts(t)
	ctx := context.Background()

	_, err := hostABI.UTXOLookup(ctx, nil)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "outpoint 不能为 nil", "错误信息应该正确")
}

// TestHostRuntimePorts_UTXOLookup_Success 测试成功查询UTXO
func TestHostRuntimePorts_UTXOLookup_Success(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockChainQuery := &mockChainQueryForHostABI{}
	mockUTXOQuery := &mockUTXOQueryForHostABI{
		utxo: &utxo.UTXO{
			ContentStrategy: &utxo.UTXO_CachedOutput{
				CachedOutput: &pb.TxOutput{
					Owner: make([]byte, 20),
					OutputContent: &pb.TxOutput_Asset{
						Asset: &pb.AssetOutput{
							AssetContent: &pb.AssetOutput_NativeCoin{
								NativeCoin: &pb.NativeCoinAsset{
									Amount: "100",
								},
							},
						},
					},
				},
			},
		},
	}
	mockCASStorage := &mockCASStorageForHostABI{}
	mockTxQuery := &mockTxQueryForHostABI{}
	mockResourceQuery := &mockResourceQueryForHostABI{}
	mockDraftService := &mockDraftServiceForHostABI{}
	mockHashManager := testutil.NewTestHashManager()
	mockExecCtx := createMockExecutionContextForHostABI()

	hostABI, err := NewHostRuntimePorts(
		logger,
		mockChainQuery,
		&mockBlockQueryForHostABI{},
		mockUTXOQuery,
		mockCASStorage,
		mockTxQuery,
		mockResourceQuery,
		mockDraftService,
		mockHashManager,
		mockExecCtx,
	)
	require.NoError(t, err)

	ctx := context.Background()
	outpoint := &pb.OutPoint{
		TxId:        make([]byte, 32),
		OutputIndex: 0,
	}

	txOutput, err := hostABI.UTXOLookup(ctx, outpoint)

	assert.NoError(t, err, "应该成功")
	assert.NotNil(t, txOutput, "应该返回TxOutput")
}

// TestHostRuntimePorts_UTXOLookup_NotFound 测试UTXO不存在
func TestHostRuntimePorts_UTXOLookup_NotFound(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockChainQuery := &mockChainQueryForHostABI{}
	mockUTXOQuery := &mockUTXOQueryForHostABI{utxo: nil, err: assert.AnError}
	mockCASStorage := &mockCASStorageForHostABI{}
	mockTxQuery := &mockTxQueryForHostABI{}
	mockResourceQuery := &mockResourceQueryForHostABI{}
	mockDraftService := &mockDraftServiceForHostABI{}
	mockHashManager := testutil.NewTestHashManager()
	mockExecCtx := createMockExecutionContextForHostABI()

	hostABI, err := NewHostRuntimePorts(
		logger,
		mockChainQuery,
		&mockBlockQueryForHostABI{},
		mockUTXOQuery,
		mockCASStorage,
		mockTxQuery,
		mockResourceQuery,
		mockDraftService,
		mockHashManager,
		mockExecCtx,
	)
	require.NoError(t, err)

	ctx := context.Background()
	outpoint := &pb.OutPoint{
		TxId:        make([]byte, 32),
		OutputIndex: 0,
	}

	_, err = hostABI.UTXOLookup(ctx, outpoint)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "查询 UTXO 失败", "错误信息应该正确")
}

// TestHostRuntimePorts_UTXOLookup_ReferenceOnly 测试引用模式UTXO
func TestHostRuntimePorts_UTXOLookup_ReferenceOnly(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockChainQuery := &mockChainQueryForHostABI{}
	mockUTXOQuery := &mockUTXOQueryForHostABI{
		utxo: &utxo.UTXO{
			ContentStrategy: &utxo.UTXO_ReferenceOnly{
				ReferenceOnly: true,
			},
		},
	}
	mockCASStorage := &mockCASStorageForHostABI{}
	mockTxQuery := &mockTxQueryForHostABI{
		transaction: &pb.Transaction{
			Outputs: []*pb.TxOutput{
				{
					Owner: make([]byte, 20),
					OutputContent: &pb.TxOutput_Asset{
						Asset: &pb.AssetOutput{
							AssetContent: &pb.AssetOutput_NativeCoin{
								NativeCoin: &pb.NativeCoinAsset{
									Amount: "100",
								},
							},
						},
					},
				},
			},
		},
	}
	mockResourceQuery := &mockResourceQueryForHostABI{}
	mockDraftService := &mockDraftServiceForHostABI{}
	mockHashManager := testutil.NewTestHashManager()
	mockExecCtx := createMockExecutionContextForHostABI()

	hostABI, err := NewHostRuntimePorts(
		logger,
		mockChainQuery,
		&mockBlockQueryForHostABI{},
		mockUTXOQuery,
		mockCASStorage,
		mockTxQuery,
		mockResourceQuery,
		mockDraftService,
		mockHashManager,
		mockExecCtx,
	)
	require.NoError(t, err)

	ctx := context.Background()
	outpoint := &pb.OutPoint{
		TxId:        make([]byte, 32),
		OutputIndex: 0,
	}

	txOutput, err := hostABI.UTXOLookup(ctx, outpoint)

	assert.NoError(t, err, "应该成功")
	assert.NotNil(t, txOutput, "应该返回TxOutput")
}

// TestHostRuntimePorts_UTXOLookup_InvalidStorageStrategy 测试无效的存储策略
func TestHostRuntimePorts_UTXOLookup_InvalidStorageStrategy(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockChainQuery := &mockChainQueryForHostABI{}
	mockUTXOQuery := &mockUTXOQueryForHostABI{
		utxo: &utxo.UTXO{
			// 既没有CachedOutput也不是ReferenceOnly
		},
	}
	mockCASStorage := &mockCASStorageForHostABI{}
	mockTxQuery := &mockTxQueryForHostABI{}
	mockResourceQuery := &mockResourceQueryForHostABI{}
	mockDraftService := &mockDraftServiceForHostABI{}
	mockHashManager := testutil.NewTestHashManager()
	mockExecCtx := createMockExecutionContextForHostABI()

	hostABI, err := NewHostRuntimePorts(
		logger,
		mockChainQuery,
		&mockBlockQueryForHostABI{},
		mockUTXOQuery,
		mockCASStorage,
		mockTxQuery,
		mockResourceQuery,
		mockDraftService,
		mockHashManager,
		mockExecCtx,
	)
	require.NoError(t, err)

	ctx := context.Background()
	outpoint := &pb.OutPoint{
		TxId:        make([]byte, 32),
		OutputIndex: 0,
	}

	_, err = hostABI.UTXOLookup(ctx, outpoint)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "UTXO存储策略无效", "错误信息应该正确")
}

// TestHostRuntimePorts_UTXOExists 测试UTXOExists
func TestHostRuntimePorts_UTXOExists(t *testing.T) {
	tests := []struct {
		name     string
		utxo     *utxo.UTXO
		err      error
		expected bool
	}{
		{"存在", &utxo.UTXO{}, nil, true},
		{"不存在", nil, assert.AnError, false},
		{"nil UTXO", nil, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := testutil.NewTestLogger()
			mockChainQuery := &mockChainQueryForHostABI{}
			mockUTXOQuery := &mockUTXOQueryForHostABI{utxo: tt.utxo, err: tt.err}
			mockCASStorage := &mockCASStorageForHostABI{}
			mockTxQuery := &mockTxQueryForHostABI{}
			mockResourceQuery := &mockResourceQueryForHostABI{}
			mockDraftService := &mockDraftServiceForHostABI{}
			mockHashManager := testutil.NewTestHashManager()
			mockExecCtx := createMockExecutionContextForHostABI()

			hostABI, err := NewHostRuntimePorts(
				logger,
				mockChainQuery,
				&mockBlockQueryForHostABI{},
				mockUTXOQuery,
				mockCASStorage,
				mockTxQuery,
				mockResourceQuery,
				mockDraftService,
				mockHashManager,
				mockExecCtx,
			)
			require.NoError(t, err)

			ctx := context.Background()
			outpoint := &pb.OutPoint{
				TxId:        make([]byte, 32),
				OutputIndex: 0,
			}

			exists, err := hostABI.UTXOExists(ctx, outpoint)

			assert.NoError(t, err, "应该成功")
			assert.Equal(t, tt.expected, exists, "应该返回正确的存在状态")
		})
	}
}

// TestHostRuntimePorts_UTXOExists_NilOutpoint 测试nil outpoint
func TestHostRuntimePorts_UTXOExists_NilOutpoint(t *testing.T) {
	hostABI := createTestHostRuntimePorts(t)
	ctx := context.Background()

	_, err := hostABI.UTXOExists(ctx, nil)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "outpoint 不能为 nil", "错误信息应该正确")
}


// TestHostRuntimePorts_GetBlockHash_NilBlock 测试nil区块
func TestHostRuntimePorts_GetBlockHash_NilBlock(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockChainQuery := &mockChainQueryForHostABI{}
	mockBlockQuery := &mockBlockQueryForHostABI{
		returnNilBlock: true,
	}
	mockUTXOQuery := &mockUTXOQueryForHostABI{}
	mockCASStorage := &mockCASStorageForHostABI{}
	mockTxQuery := &mockTxQueryForHostABI{}
	mockResourceQuery := &mockResourceQueryForHostABI{}
	mockDraftService := &mockDraftServiceForHostABI{}
	mockHashManager := testutil.NewTestHashManager()
	mockExecCtx := createMockExecutionContextForHostABI()

	hostABI, err := NewHostRuntimePorts(
		logger,
		mockChainQuery,
		mockBlockQuery,
		mockUTXOQuery,
		mockCASStorage,
		mockTxQuery,
		mockResourceQuery,
		mockDraftService,
		mockHashManager,
		mockExecCtx,
	)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = hostABI.GetBlockHash(ctx, 50)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "区块不存在或区块头为空", "错误信息应该正确")
}

// TestHostRuntimePorts_UTXOLookup_ReferenceOnly_NoTxQuery 测试引用模式但txQuery为nil
// 注意：由于NewHostRuntimePorts不允许nil txQuery，我们需要通过反射或其他方式测试这个场景
// 这里我们创建一个特殊的mock，在UTXOLookup时返回nil txQuery的错误
func TestHostRuntimePorts_UTXOLookup_ReferenceOnly_NoTxQuery(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockChainQuery := &mockChainQueryForHostABI{}
	mockUTXOQuery := &mockUTXOQueryForHostABI{
		utxo: &utxo.UTXO{
			ContentStrategy: &utxo.UTXO_ReferenceOnly{
				ReferenceOnly: true,
			},
		},
	}
	mockCASStorage := &mockCASStorageForHostABI{}
	// 创建一个特殊的HostRuntimePorts，手动设置txQuery为nil（绕过NewHostRuntimePorts的检查）
	// 这需要直接构造HostRuntimePorts结构体
	mockTxQuery := &mockTxQueryForHostABI{}
	mockResourceQuery := &mockResourceQueryForHostABI{}
	mockDraftService := &mockDraftServiceForHostABI{}
	mockHashManager := testutil.NewTestHashManager()
	mockExecCtx := createMockExecutionContextForHostABI()

	hostABI, err := NewHostRuntimePorts(
		logger,
		mockChainQuery,
		&mockBlockQueryForHostABI{},
		mockUTXOQuery,
		mockCASStorage,
		mockTxQuery,
		mockResourceQuery,
		mockDraftService,
		mockHashManager,
		mockExecCtx,
	)
	require.NoError(t, err)

	// 手动设置txQuery为nil（用于测试）
	hostRuntimePorts := hostABI.(*HostRuntimePorts)
	hostRuntimePorts.txQuery = nil

	ctx := context.Background()
	outpoint := &pb.OutPoint{
		TxId:        make([]byte, 32),
		OutputIndex: 0,
	}

	_, err = hostABI.UTXOLookup(ctx, outpoint)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "txQuery 未初始化", "错误信息应该正确")
}

// TestHostRuntimePorts_UTXOLookup_ReferenceOnly_InvalidIndex 测试输出索引越界
func TestHostRuntimePorts_UTXOLookup_ReferenceOnly_InvalidIndex(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockChainQuery := &mockChainQueryForHostABI{}
	mockUTXOQuery := &mockUTXOQueryForHostABI{
		utxo: &utxo.UTXO{
			ContentStrategy: &utxo.UTXO_ReferenceOnly{
				ReferenceOnly: true,
			},
		},
	}
	mockCASStorage := &mockCASStorageForHostABI{}
	mockTxQuery := &mockTxQueryForHostABI{
		transaction: &pb.Transaction{
			Outputs: []*pb.TxOutput{
				{
					Owner: make([]byte, 20),
					OutputContent: &pb.TxOutput_Asset{
						Asset: &pb.AssetOutput{
							AssetContent: &pb.AssetOutput_NativeCoin{
								NativeCoin: &pb.NativeCoinAsset{
									Amount: "100",
								},
							},
						},
					},
				},
			},
		},
	}
	mockResourceQuery := &mockResourceQueryForHostABI{}
	mockDraftService := &mockDraftServiceForHostABI{}
	mockHashManager := testutil.NewTestHashManager()
	mockExecCtx := createMockExecutionContextForHostABI()

	hostABI, err := NewHostRuntimePorts(
		logger,
		mockChainQuery,
		&mockBlockQueryForHostABI{},
		mockUTXOQuery,
		mockCASStorage,
		mockTxQuery,
		mockResourceQuery,
		mockDraftService,
		mockHashManager,
		mockExecCtx,
	)
	require.NoError(t, err)

	ctx := context.Background()
	outpoint := &pb.OutPoint{
		TxId:        make([]byte, 32),
		OutputIndex: 10, // 超出范围
	}

	_, err = hostABI.UTXOLookup(ctx, outpoint)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "输出索引越界", "错误信息应该正确")
}

// TestHostRuntimePorts_ResourceLookup 测试ResourceLookup
func TestHostRuntimePorts_ResourceLookup(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockChainQuery := &mockChainQueryForHostABI{}
	mockUTXOQuery := &mockUTXOQueryForHostABI{}
	mockCASStorage := &mockCASStorageForHostABI{}
	mockTxQuery := &mockTxQueryForHostABI{}
	mockResourceQuery := &mockResourceQueryForHostABI{
		resource: &pb_resource.Resource{
			ContentHash: make([]byte, 32),
		},
	}
	mockDraftService := &mockDraftServiceForHostABI{}
	mockHashManager := testutil.NewTestHashManager()
	mockExecCtx := createMockExecutionContextForHostABI()

	hostABI, err := NewHostRuntimePorts(
		logger,
		mockChainQuery,
		&mockBlockQueryForHostABI{},
		mockUTXOQuery,
		mockCASStorage,
		mockTxQuery,
		mockResourceQuery,
		mockDraftService,
		mockHashManager,
		mockExecCtx,
	)
	require.NoError(t, err)

	ctx := context.Background()
	contentHash := make([]byte, 32)

	resource, err := hostABI.ResourceLookup(ctx, contentHash)

	assert.NoError(t, err, "应该成功")
	assert.NotNil(t, resource, "应该返回Resource")
}

// TestHostRuntimePorts_ResourceLookup_InvalidHashLength 测试无效的哈希长度
func TestHostRuntimePorts_ResourceLookup_InvalidHashLength(t *testing.T) {
	hostABI := createTestHostRuntimePorts(t)
	ctx := context.Background()

	tests := []struct {
		name        string
		contentHash []byte
	}{
		{"空哈希", []byte{}},
		{"短哈希", make([]byte, 20)},
		{"长哈希", make([]byte, 64)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := hostABI.ResourceLookup(ctx, tt.contentHash)

			assert.Error(t, err, "应该返回错误")
			assert.Contains(t, err.Error(), "contentHash 必须是 32 字节", "错误信息应该正确")
		})
	}
}

// TestHostRuntimePorts_ResourceLookup_QueryError 测试查询错误
func TestHostRuntimePorts_ResourceLookup_QueryError(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockChainQuery := &mockChainQueryForHostABI{}
	mockUTXOQuery := &mockUTXOQueryForHostABI{}
	mockCASStorage := &mockCASStorageForHostABI{}
	mockTxQuery := &mockTxQueryForHostABI{}
	mockResourceQuery := &mockResourceQueryForHostABI{
		err: assert.AnError,
	}
	mockDraftService := &mockDraftServiceForHostABI{}
	mockHashManager := testutil.NewTestHashManager()
	mockExecCtx := createMockExecutionContextForHostABI()

	hostABI, err := NewHostRuntimePorts(
		logger,
		mockChainQuery,
		&mockBlockQueryForHostABI{},
		mockUTXOQuery,
		mockCASStorage,
		mockTxQuery,
		mockResourceQuery,
		mockDraftService,
		mockHashManager,
		mockExecCtx,
	)
	require.NoError(t, err)

	ctx := context.Background()
	contentHash := make([]byte, 32)

	_, err = hostABI.ResourceLookup(ctx, contentHash)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "查询资源失败", "错误信息应该正确")
}

// TestHostRuntimePorts_ResourceExists 测试ResourceExists
func TestHostRuntimePorts_ResourceExists(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		err      error
		expected bool
	}{
		{"存在", []byte("test-data"), nil, true},
		{"不存在", nil, assert.AnError, false},
		{"nil数据", nil, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := testutil.NewTestLogger()
			mockChainQuery := &mockChainQueryForHostABI{}
			mockUTXOQuery := &mockUTXOQueryForHostABI{}
			mockCASStorage := &mockCASStorageForHostABI{
				data: tt.data,
				err:  tt.err,
			}
			mockTxQuery := &mockTxQueryForHostABI{}
			mockResourceQuery := &mockResourceQueryForHostABI{}
			mockDraftService := &mockDraftServiceForHostABI{}
			mockHashManager := testutil.NewTestHashManager()
			mockExecCtx := createMockExecutionContextForHostABI()

			hostABI, err := NewHostRuntimePorts(
				logger,
				mockChainQuery,
				&mockBlockQueryForHostABI{},
				mockUTXOQuery,
				mockCASStorage,
				mockTxQuery,
				mockResourceQuery,
				mockDraftService,
				mockHashManager,
				mockExecCtx,
			)
			require.NoError(t, err)

			ctx := context.Background()
			contentHash := make([]byte, 32)

			exists, err := hostABI.ResourceExists(ctx, contentHash)

			assert.NoError(t, err, "应该成功")
			assert.Equal(t, tt.expected, exists, "应该返回正确的存在状态")
		})
	}
}

// TestHostRuntimePorts_ResourceExists_InvalidHashLength 测试无效的哈希长度
func TestHostRuntimePorts_ResourceExists_InvalidHashLength(t *testing.T) {
	hostABI := createTestHostRuntimePorts(t)
	ctx := context.Background()

	tests := []struct {
		name        string
		contentHash []byte
	}{
		{"空哈希", []byte{}},
		{"短哈希", make([]byte, 20)},
		{"长哈希", make([]byte, 64)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := hostABI.ResourceExists(ctx, tt.contentHash)

			assert.Error(t, err, "应该返回错误")
			assert.Contains(t, err.Error(), "contentHash 必须是 32 字节", "错误信息应该正确")
		})
	}
}

// TestHostRuntimePorts_EmitEvent 测试EmitEvent
func TestHostRuntimePorts_EmitEvent(t *testing.T) {
	hostABI := createTestHostRuntimePorts(t)
	ctx := context.Background()

	err := hostABI.EmitEvent(ctx, "test_event", []byte("test-data"))

	assert.NoError(t, err, "应该成功")
}

// TestHostRuntimePorts_EmitEvent_AddEventError 测试AddEvent错误
func TestHostRuntimePorts_EmitEvent_AddEventError(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockChainQuery := &mockChainQueryForHostABI{}
	mockUTXOQuery := &mockUTXOQueryForHostABI{}
	mockCASStorage := &mockCASStorageForHostABI{}
	mockTxQuery := &mockTxQueryForHostABI{}
	mockResourceQuery := &mockResourceQueryForHostABI{}
	mockDraftService := &mockDraftServiceForHostABI{}
	mockHashManager := testutil.NewTestHashManager()
	mockExecCtx := &mockExecutionContextForHostABI{
		executionID:     "exec-123",
		callerAddress:   make([]byte, 20),
		contractAddress: make([]byte, 20),
		txID:            make([]byte, 32),
		chainID:         []byte("test-chain"),
		blockHeight:     100,
		blockTimestamp:  1234567890,
		draftID:         "draft-123",
		addEventErr:     assert.AnError,
	}

	hostABI, err := NewHostRuntimePorts(
		logger,
		mockChainQuery,
		&mockBlockQueryForHostABI{},
		mockUTXOQuery,
		mockCASStorage,
		mockTxQuery,
		mockResourceQuery,
		mockDraftService,
		mockHashManager,
		mockExecCtx,
	)
	require.NoError(t, err)

	ctx := context.Background()
	err = hostABI.EmitEvent(ctx, "test_event", []byte("test-data"))

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "添加事件失败", "错误信息应该正确")
}

// TestHostRuntimePorts_LogDebug 测试LogDebug
func TestHostRuntimePorts_LogDebug(t *testing.T) {
	hostABI := createTestHostRuntimePorts(t)
	ctx := context.Background()

	err := hostABI.LogDebug(ctx, "test debug message")

	assert.NoError(t, err, "应该成功")
}

// ============================================================================
// 类别 C：交易构建（写入）- 4个原语
// ============================================================================

// TestHostRuntimePorts_TxAddInput 测试添加交易输入
func TestHostRuntimePorts_TxAddInput(t *testing.T) {
	hostABI := createTestHostRuntimePorts(t)
	ctx := context.Background()
	outpoint := &pb.OutPoint{
		TxId:        make([]byte, 32),
		OutputIndex: 0,
	}
	unlockingProof := &pb.UnlockingProof{}

	index, err := hostABI.TxAddInput(ctx, outpoint, false, unlockingProof)

	assert.NoError(t, err, "应该成功添加输入")
	assert.Equal(t, uint32(0), index, "应该返回输入索引")
}

// TestHostRuntimePorts_TxAddInput_NilOutpoint 测试nil outpoint
func TestHostRuntimePorts_TxAddInput_NilOutpoint(t *testing.T) {
	hostABI := createTestHostRuntimePorts(t)
	ctx := context.Background()
	unlockingProof := &pb.UnlockingProof{}

	_, err := hostABI.TxAddInput(ctx, nil, false, unlockingProof)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "outpoint 不能为 nil", "错误信息应该正确")
}

// TestHostRuntimePorts_TxAddInput_LoadDraftFailed 测试加载草稿失败
func TestHostRuntimePorts_TxAddInput_LoadDraftFailed(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockChainQuery := &mockChainQueryForHostABI{}
	mockUTXOQuery := &mockUTXOQueryForHostABI{}
	mockCASStorage := &mockCASStorageForHostABI{}
	mockTxQuery := &mockTxQueryForHostABI{}
	mockResourceQuery := &mockResourceQueryForHostABI{}
	mockDraftService := &mockDraftServiceForHostABIWithErrors{
		loadDraftError: assert.AnError,
	}
	mockHashManager := testutil.NewTestHashManager()
	mockExecCtx := createMockExecutionContextForHostABI()

	hostABI, err := NewHostRuntimePorts(
		logger,
		mockChainQuery,
		&mockBlockQueryForHostABI{},
		mockUTXOQuery,
		mockCASStorage,
		mockTxQuery,
		mockResourceQuery,
		mockDraftService,
		mockHashManager,
		mockExecCtx,
	)
	require.NoError(t, err)

	ctx := context.Background()
	outpoint := &pb.OutPoint{
		TxId:        make([]byte, 32),
		OutputIndex: 0,
	}
	unlockingProof := &pb.UnlockingProof{}

	_, err = hostABI.TxAddInput(ctx, outpoint, false, unlockingProof)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "加载草稿失败", "错误信息应该正确")
}

// TestHostRuntimePorts_TxAddAssetOutput 测试添加资产输出
func TestHostRuntimePorts_TxAddAssetOutput(t *testing.T) {
	hostABI := createTestHostRuntimePorts(t)
	ctx := context.Background()
	owner := make([]byte, 20)
	amount := uint64(1000)
	tokenID := []byte(nil)
	lockingConditions := []*pb.LockingCondition{}

	index, err := hostABI.TxAddAssetOutput(ctx, owner, amount, tokenID, lockingConditions)

	assert.NoError(t, err, "应该成功添加资产输出")
	assert.Equal(t, uint32(0), index, "应该返回输出索引")
}

// TestHostRuntimePorts_TxAddAssetOutput_InvalidOwnerLength 测试无效的owner长度
func TestHostRuntimePorts_TxAddAssetOutput_InvalidOwnerLength(t *testing.T) {
	hostABI := createTestHostRuntimePorts(t)
	ctx := context.Background()
	owner := make([]byte, 19) // 无效长度
	amount := uint64(1000)
	tokenID := []byte(nil)
	lockingConditions := []*pb.LockingCondition{}

	_, err := hostABI.TxAddAssetOutput(ctx, owner, amount, tokenID, lockingConditions)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "owner 地址必须是 20 字节", "错误信息应该正确")
}

// TestHostRuntimePorts_TxAddResourceOutput 测试添加资源输出
func TestHostRuntimePorts_TxAddResourceOutput(t *testing.T) {
	hostABI := createTestHostRuntimePorts(t)
	ctx := context.Background()
	contentHash := make([]byte, 32)
	category := "wasm"
	owner := make([]byte, 20)
	lockingConditions := []*pb.LockingCondition{}
	metadata := []byte("test metadata")

	index, err := hostABI.TxAddResourceOutput(ctx, contentHash, category, owner, lockingConditions, metadata)

	assert.NoError(t, err, "应该成功添加资源输出")
	assert.Equal(t, uint32(0), index, "应该返回输出索引")
}

// TestHostRuntimePorts_TxAddResourceOutput_InvalidContentHashLength 测试无效的contentHash长度
func TestHostRuntimePorts_TxAddResourceOutput_InvalidContentHashLength(t *testing.T) {
	hostABI := createTestHostRuntimePorts(t)
	ctx := context.Background()
	contentHash := make([]byte, 31) // 无效长度
	category := "wasm"
	owner := make([]byte, 20)
	lockingConditions := []*pb.LockingCondition{}
	metadata := []byte("test metadata")

	_, err := hostABI.TxAddResourceOutput(ctx, contentHash, category, owner, lockingConditions, metadata)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "contentHash 必须是 32 字节", "错误信息应该正确")
}

// TestHostRuntimePorts_TxAddResourceOutput_InvalidOwnerLength 测试无效的owner长度
func TestHostRuntimePorts_TxAddResourceOutput_InvalidOwnerLength(t *testing.T) {
	hostABI := createTestHostRuntimePorts(t)
	ctx := context.Background()
	contentHash := make([]byte, 32)
	category := "wasm"
	owner := make([]byte, 19) // 无效长度
	lockingConditions := []*pb.LockingCondition{}
	metadata := []byte("test metadata")

	_, err := hostABI.TxAddResourceOutput(ctx, contentHash, category, owner, lockingConditions, metadata)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "owner 地址必须是 20 字节", "错误信息应该正确")
}

// TestHostRuntimePorts_TxAddStateOutput 测试添加状态输出
func TestHostRuntimePorts_TxAddStateOutput(t *testing.T) {
	hostABI := createTestHostRuntimePorts(t)
	ctx := context.Background()
	stateID := []byte("test_state_id")
	stateVersion := uint64(1)
	executionResultHash := make([]byte, 32)
	publicInputs := []byte("public inputs")
	parentStateHash := []byte("parent state hash")

	index, err := hostABI.TxAddStateOutput(ctx, stateID, stateVersion, executionResultHash, publicInputs, parentStateHash)

	assert.NoError(t, err, "应该成功添加状态输出")
	assert.Equal(t, uint32(0), index, "应该返回输出索引")
}

// TestHostRuntimePorts_TxAddStateOutput_EmptyStateID 测试空的stateID
func TestHostRuntimePorts_TxAddStateOutput_EmptyStateID(t *testing.T) {
	hostABI := createTestHostRuntimePorts(t)
	ctx := context.Background()
	stateID := []byte{} // 空stateID
	stateVersion := uint64(1)
	executionResultHash := make([]byte, 32)
	publicInputs := []byte("public inputs")
	parentStateHash := []byte("parent state hash")

	_, err := hostABI.TxAddStateOutput(ctx, stateID, stateVersion, executionResultHash, publicInputs, parentStateHash)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "stateID 不能为空", "错误信息应该正确")
}

// TestHostRuntimePorts_TxAddStateOutput_InvalidExecutionResultHashLength 测试无效的executionResultHash长度
func TestHostRuntimePorts_TxAddStateOutput_InvalidExecutionResultHashLength(t *testing.T) {
	hostABI := createTestHostRuntimePorts(t)
	ctx := context.Background()
	stateID := []byte("test_state_id")
	stateVersion := uint64(1)
	executionResultHash := make([]byte, 31) // 无效长度
	publicInputs := []byte("public inputs")
	parentStateHash := []byte("parent state hash")

	_, err := hostABI.TxAddStateOutput(ctx, stateID, stateVersion, executionResultHash, publicInputs, parentStateHash)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "executionResultHash 必须是 32 字节", "错误信息应该正确")
}

// ============================================================================
// 辅助函数
// ============================================================================

// createTestHostRuntimePorts 创建测试用的HostRuntimePorts
func createTestHostRuntimePorts(t *testing.T) *HostRuntimePorts {
	t.Helper()

	logger := testutil.NewTestLogger()
	mockChainQuery := &mockChainQueryForHostABI{}
	mockBlockQuery := &mockBlockQueryForHostABI{}
	mockUTXOQuery := &mockUTXOQueryForHostABI{}
	mockCASStorage := &mockCASStorageForHostABI{}
	mockTxQuery := &mockTxQueryForHostABI{}
	mockResourceQuery := &mockResourceQueryForHostABI{}
	mockDraftService := &mockDraftServiceForHostABI{}
	mockHashManager := testutil.NewTestHashManager()
	mockExecCtx := createMockExecutionContextForHostABI()

	hostABI, err := NewHostRuntimePorts(
		logger,
		mockChainQuery,
		mockBlockQuery,
		mockUTXOQuery,
		mockCASStorage,
		mockTxQuery,
		mockResourceQuery,
		mockDraftService,
		mockHashManager,
		mockExecCtx,
	)
	require.NoError(t, err)

	return hostABI.(*HostRuntimePorts)
}

// createMockExecutionContextForHostABI 创建Mock的ExecutionContext
func createMockExecutionContextForHostABI() ispcInterfaces.ExecutionContext {
	return &mockExecutionContextForHostABI{
		executionID:      "exec-123",
		callerAddress:    make([]byte, 20),
		contractAddress:  make([]byte, 20),
		txID:             make([]byte, 32),
		chainID:          []byte("test-chain"),
		blockHeight:      100,
		blockTimestamp:   1234567890,
		draftID:          "draft-123",
	}
}

// ============================================================================
// Mock对象定义
// ============================================================================

// mockExecutionContextForHostABI Mock的ExecutionContext
type mockExecutionContextForHostABI struct {
	executionID     string
	callerAddress   []byte
	contractAddress []byte
	txID            []byte
	chainID         []byte
	blockHeight     uint64
	blockTimestamp  uint64
	draftID         string
	addEventErr     error
}

func (m *mockExecutionContextForHostABI) GetExecutionID() string { return m.executionID }
func (m *mockExecutionContextForHostABI) GetDraftID() string { return m.draftID }
func (m *mockExecutionContextForHostABI) GetBlockHeight() uint64 { return m.blockHeight }
func (m *mockExecutionContextForHostABI) GetBlockTimestamp() uint64 { return m.blockTimestamp }
func (m *mockExecutionContextForHostABI) GetChainID() []byte { return m.chainID }
func (m *mockExecutionContextForHostABI) GetTransactionID() []byte { return m.txID }
func (m *mockExecutionContextForHostABI) GetCallerAddress() []byte { return m.callerAddress }
func (m *mockExecutionContextForHostABI) GetContractAddress() []byte { return m.contractAddress }
func (m *mockExecutionContextForHostABI) HostABI() ispcInterfaces.HostABI { return nil }
func (m *mockExecutionContextForHostABI) SetHostABI(hostABI ispcInterfaces.HostABI) error { return nil }
func (m *mockExecutionContextForHostABI) GetTransactionDraft() (*ispcInterfaces.TransactionDraft, error) { return nil, nil }
func (m *mockExecutionContextForHostABI) UpdateTransactionDraft(draft *ispcInterfaces.TransactionDraft) error { return nil }
func (m *mockExecutionContextForHostABI) RecordHostFunctionCall(call *ispcInterfaces.HostFunctionCall) {}
func (m *mockExecutionContextForHostABI) GetExecutionTrace() ([]*ispcInterfaces.HostFunctionCall, error) { return nil, nil }
func (m *mockExecutionContextForHostABI) RecordStateChange(key string, oldValue interface{}, newValue interface{}) error { return nil }
func (m *mockExecutionContextForHostABI) RecordTraceRecords(records []ispcInterfaces.TraceRecord) error { return nil }
func (m *mockExecutionContextForHostABI) GetResourceUsage() *types.ResourceUsage { return &types.ResourceUsage{} }
func (m *mockExecutionContextForHostABI) FinalizeResourceUsage() {}
func (m *mockExecutionContextForHostABI) SetReturnData(data []byte) error { return nil }
func (m *mockExecutionContextForHostABI) GetReturnData() ([]byte, error) { return nil, nil }
func (m *mockExecutionContextForHostABI) AddEvent(event *ispcInterfaces.Event) error {
	if m.addEventErr != nil {
		return m.addEventErr
	}
	return nil
}
func (m *mockExecutionContextForHostABI) GetEvents() ([]*ispcInterfaces.Event, error) { return nil, nil }
func (m *mockExecutionContextForHostABI) SetInitParams(params []byte) error { return nil }
func (m *mockExecutionContextForHostABI) GetInitParams() ([]byte, error) { return nil, nil }

// mockBlockQueryForHostABI Mock的区块查询服务
type mockBlockQueryForHostABI struct {
	err           error
	returnNilBlock bool
}

func (m *mockBlockQueryForHostABI) GetBlockByHeight(ctx context.Context, height uint64) (*core.Block, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.returnNilBlock {
		return nil, nil
	}
	return &core.Block{
		Header: &core.BlockHeader{
			Height: height,
		},
	}, nil
}

func (m *mockBlockQueryForHostABI) GetBlockByHash(ctx context.Context, blockHash []byte) (*core.Block, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &core.Block{
		Header: &core.BlockHeader{
			Height: 100,
		},
	}, nil
}

func (m *mockBlockQueryForHostABI) GetBlockHeader(ctx context.Context, blockHash []byte) (*core.BlockHeader, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &core.BlockHeader{
		Height: 100,
	}, nil
}

func (m *mockBlockQueryForHostABI) GetBlockRange(ctx context.Context, startHeight, endHeight uint64) ([]*core.Block, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []*core.Block{}, nil
}

func (m *mockBlockQueryForHostABI) GetHighestBlock(ctx context.Context) (height uint64, blockHash []byte, err error) {
	if m.err != nil {
		return 0, nil, m.err
	}
	return 100, make([]byte, 32), nil
}

// mockChainQueryForHostABI Mock的链查询服务
type mockChainQueryForHostABI struct {
	err error
}

func (m *mockChainQueryForHostABI) GetChainInfo(ctx context.Context) (*types.ChainInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &types.ChainInfo{
		Height:        100,
		BestBlockHash: make([]byte, 32),
	}, nil
}
func (m *mockChainQueryForHostABI) GetCurrentHeight(ctx context.Context) (uint64, error) { return 100, nil }
func (m *mockChainQueryForHostABI) GetBestBlockHash(ctx context.Context) ([]byte, error) { return make([]byte, 32), nil }
func (m *mockChainQueryForHostABI) GetNodeMode(ctx context.Context) (types.NodeMode, error) { return types.NodeModeFull, nil }
func (m *mockChainQueryForHostABI) IsDataFresh(ctx context.Context) (bool, error) { return true, nil }
func (m *mockChainQueryForHostABI) IsReady(ctx context.Context) (bool, error) { return true, nil }
func (m *mockChainQueryForHostABI) GetSyncStatus(ctx context.Context) (*types.SystemSyncStatus, error) { return nil, nil }

// mockChainQueryForHostABIWithBlockQuery Mock的链查询服务（实现BlockQuery接口）
type mockChainQueryForHostABIWithBlockQuery struct {
	mockChainQueryForHostABI
	err           error
	returnNilBlock bool
}

func (m *mockChainQueryForHostABIWithBlockQuery) GetBlockByHeight(ctx context.Context, height uint64) (*core.Block, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.returnNilBlock {
		return nil, nil
	}
	return &core.Block{
		Header: &core.BlockHeader{
			Height: height,
		},
	}, nil
}
func (m *mockChainQueryForHostABIWithBlockQuery) GetBlockByHash(ctx context.Context, blockHash []byte) (*core.Block, error) { return nil, nil }
func (m *mockChainQueryForHostABIWithBlockQuery) GetBlockHeader(ctx context.Context, blockHash []byte) (*core.BlockHeader, error) { return nil, nil }
func (m *mockChainQueryForHostABIWithBlockQuery) GetBlockRange(ctx context.Context, startHeight, endHeight uint64) ([]*core.Block, error) { return nil, nil }
func (m *mockChainQueryForHostABIWithBlockQuery) GetHighestBlock(ctx context.Context) (height uint64, blockHash []byte, err error) { return 100, make([]byte, 32), nil }

// mockUTXOQueryForHostABI Mock的UTXO查询服务
type mockUTXOQueryForHostABI struct {
	utxo *utxo.UTXO
	err  error
}

func (m *mockUTXOQueryForHostABI) GetUTXO(ctx context.Context, outpoint *pb.OutPoint) (*utxo.UTXO, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.utxo, nil
}
func (m *mockUTXOQueryForHostABI) GetUTXOsByAddress(ctx context.Context, address []byte, category *utxo.UTXOCategory, onlyAvailable bool) ([]*utxo.UTXO, error) { return nil, nil }
func (m *mockUTXOQueryForHostABI) GetSponsorPoolUTXOs(ctx context.Context, onlyAvailable bool) ([]*utxo.UTXO, error) { return nil, nil }
func (m *mockUTXOQueryForHostABI) GetCurrentStateRoot(ctx context.Context) ([]byte, error) { return nil, nil }

// mockCASStorageForHostABI Mock的CAS存储
type mockCASStorageForHostABI struct {
	data []byte
	err  error
}

func (m *mockCASStorageForHostABI) BuildFilePath(contentHash []byte) string { return "" }
func (m *mockCASStorageForHostABI) StoreFile(ctx context.Context, contentHash []byte, data []byte) error { return nil }
func (m *mockCASStorageForHostABI) ReadFile(ctx context.Context, contentHash []byte) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.data, nil
}
func (m *mockCASStorageForHostABI) FileExists(contentHash []byte) bool { return false }

// mockDraftServiceForHostABI Mock的交易草稿服务
type mockDraftServiceForHostABI struct{}

func (m *mockDraftServiceForHostABI) CreateDraft(ctx context.Context) (*types.DraftTx, error) { return nil, nil }
func (m *mockDraftServiceForHostABI) LoadDraft(ctx context.Context, draftID string) (*types.DraftTx, error) {
	return &types.DraftTx{}, nil
}
func (m *mockDraftServiceForHostABI) SaveDraft(ctx context.Context, draft *types.DraftTx) error { return nil }
func (m *mockDraftServiceForHostABI) GetDraftByID(ctx context.Context, draftID string) (*types.DraftTx, error) { return nil, nil }
func (m *mockDraftServiceForHostABI) ValidateDraft(ctx context.Context, draft *types.DraftTx) error { return nil }
func (m *mockDraftServiceForHostABI) SealDraft(ctx context.Context, draft *types.DraftTx) (*types.ComposedTx, error) { return nil, nil }
func (m *mockDraftServiceForHostABI) DeleteDraft(ctx context.Context, draftID string) error { return nil }
func (m *mockDraftServiceForHostABI) AddInput(ctx context.Context, draft *types.DraftTx, outpoint *pb.OutPoint, isReferenceOnly bool, unlockingProof *pb.UnlockingProof) (uint32, error) { return 0, nil }
func (m *mockDraftServiceForHostABI) AddAssetOutput(ctx context.Context, draft *types.DraftTx, owner []byte, amount string, tokenID []byte, lockingConditions []*pb.LockingCondition) (uint32, error) { return 0, nil }
func (m *mockDraftServiceForHostABI) AddResourceOutput(ctx context.Context, draft *types.DraftTx, contentHash []byte, category string, owner []byte, lockingConditions []*pb.LockingCondition, metadata []byte) (uint32, error) { return 0, nil }
func (m *mockDraftServiceForHostABI) AddStateOutput(ctx context.Context, draft *types.DraftTx, stateID []byte, stateVersion uint64, executionResultHash []byte, publicInputs []byte, parentStateHash []byte) (uint32, error) { return 0, nil }

// mockDraftServiceForHostABIWithErrors Mock的交易草稿服务（带错误）
type mockDraftServiceForHostABIWithErrors struct {
	loadDraftError        error
	saveDraftError        error
	addInputError         error
	addAssetOutputError   error
	addResourceOutputError error
	addStateOutputError   error
}

func (m *mockDraftServiceForHostABIWithErrors) CreateDraft(ctx context.Context) (*types.DraftTx, error) { return nil, nil }
func (m *mockDraftServiceForHostABIWithErrors) LoadDraft(ctx context.Context, draftID string) (*types.DraftTx, error) {
	if m.loadDraftError != nil {
		return nil, m.loadDraftError
	}
	return &types.DraftTx{}, nil
}
func (m *mockDraftServiceForHostABIWithErrors) SaveDraft(ctx context.Context, draft *types.DraftTx) error {
	if m.saveDraftError != nil {
		return m.saveDraftError
	}
	return nil
}
func (m *mockDraftServiceForHostABIWithErrors) GetDraftByID(ctx context.Context, draftID string) (*types.DraftTx, error) { return nil, nil }
func (m *mockDraftServiceForHostABIWithErrors) ValidateDraft(ctx context.Context, draft *types.DraftTx) error { return nil }
func (m *mockDraftServiceForHostABIWithErrors) SealDraft(ctx context.Context, draft *types.DraftTx) (*types.ComposedTx, error) { return nil, nil }
func (m *mockDraftServiceForHostABIWithErrors) DeleteDraft(ctx context.Context, draftID string) error { return nil }
func (m *mockDraftServiceForHostABIWithErrors) AddInput(ctx context.Context, draft *types.DraftTx, outpoint *pb.OutPoint, isReferenceOnly bool, unlockingProof *pb.UnlockingProof) (uint32, error) {
	if m.addInputError != nil {
		return 0, m.addInputError
	}
	return 0, nil
}
func (m *mockDraftServiceForHostABIWithErrors) AddAssetOutput(ctx context.Context, draft *types.DraftTx, owner []byte, amount string, tokenID []byte, lockingConditions []*pb.LockingCondition) (uint32, error) {
	if m.addAssetOutputError != nil {
		return 0, m.addAssetOutputError
	}
	return 0, nil
}
func (m *mockDraftServiceForHostABIWithErrors) AddResourceOutput(ctx context.Context, draft *types.DraftTx, contentHash []byte, category string, owner []byte, lockingConditions []*pb.LockingCondition, metadata []byte) (uint32, error) {
	if m.addResourceOutputError != nil {
		return 0, m.addResourceOutputError
	}
	return 0, nil
}
func (m *mockDraftServiceForHostABIWithErrors) AddStateOutput(ctx context.Context, draft *types.DraftTx, stateID []byte, stateVersion uint64, executionResultHash []byte, publicInputs []byte, parentStateHash []byte) (uint32, error) {
	if m.addStateOutputError != nil {
		return 0, m.addStateOutputError
	}
	return 0, nil
}

// mockTxQueryForHostABI Mock的交易查询服务
type mockTxQueryForHostABI struct {
	transaction *pb.Transaction
	err         error
}

func (m *mockTxQueryForHostABI) GetTransaction(ctx context.Context, txHash []byte) (blockHash []byte, txIndex uint32, transaction *pb.Transaction, err error) {
	if m.err != nil {
		return nil, 0, nil, m.err
	}
	return nil, 0, m.transaction, nil
}
func (m *mockTxQueryForHostABI) GetTxBlockHeight(ctx context.Context, txHash []byte) (uint64, error) { return 0, nil }
func (m *mockTxQueryForHostABI) GetBlockTimestamp(ctx context.Context, height uint64) (int64, error) { return 0, nil }
func (m *mockTxQueryForHostABI) GetAccountNonce(ctx context.Context, address []byte) (uint64, error) { return 0, nil }
func (m *mockTxQueryForHostABI) GetTransactionsByBlock(ctx context.Context, blockHash []byte) ([]*pb.Transaction, error) { return nil, nil }

// mockResourceQueryForHostABI Mock的资源查询服务
type mockResourceQueryForHostABI struct {
	resource *pb_resource.Resource
	err      error
}

func (m *mockResourceQueryForHostABI) GetResourceByContentHash(ctx context.Context, contentHash []byte) (*pb_resource.Resource, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.resource, nil
}
func (m *mockResourceQueryForHostABI) GetResourceFromBlockchain(ctx context.Context, contentHash []byte) (*pb_resource.Resource, bool, error) { return nil, false, nil }
func (m *mockResourceQueryForHostABI) GetResourceTransaction(ctx context.Context, contentHash []byte) (txHash, blockHash []byte, blockHeight uint64, err error) { return nil, nil, 0, nil }
func (m *mockResourceQueryForHostABI) CheckFileExists(contentHash []byte) bool { return false }
func (m *mockResourceQueryForHostABI) BuildFilePath(contentHash []byte) string { return "" }
func (m *mockResourceQueryForHostABI) ListResourceHashes(ctx context.Context, offset int, limit int) ([][]byte, error) { return nil, nil }

