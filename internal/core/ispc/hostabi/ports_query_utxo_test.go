package hostabi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/ispc/testutil"
	utxo "github.com/weisyn/v1/pb/blockchain/utxo"
	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ============================================================================
// ports_query_utxo.go 测试
// ============================================================================
//
// 🎯 **测试目的**：发现 GetBalance, GetTransaction 的缺陷和BUG
//
// ============================================================================

// TestGetBalance_NativeCoin 测试查询原生币余额
func TestGetBalance_NativeCoin(t *testing.T) {
	mockUTXOQuery := &mockUTXOQueryForPorts{
		utxos: []*utxo.UTXO{
			{
				Category: utxo.UTXOCategory_UTXO_CATEGORY_ASSET,
				ContentStrategy: &utxo.UTXO_CachedOutput{
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
				Category: utxo.UTXOCategory_UTXO_CATEGORY_ASSET,
				ContentStrategy: &utxo.UTXO_CachedOutput{
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
		},
	}
	hostABI := createHostRuntimePortsWithUTXOQuery(t, mockUTXOQuery)
	ctx := context.Background()
	address := make([]byte, 20)

	balance, err := hostABI.GetBalance(ctx, address, nil)

	assert.NoError(t, err, "应该成功查询余额")
	assert.Equal(t, uint64(3000), balance, "余额应该是3000（1000+2000）")
}

// TestGetBalance_ContractToken 测试查询合约代币余额
func TestGetBalance_ContractToken(t *testing.T) {
	tokenID := make([]byte, 20)
	tokenID[0] = 0x01 // 设置tokenID
	mockUTXOQuery := &mockUTXOQueryForPorts{
		utxos: []*utxo.UTXO{
			{
				Category: utxo.UTXOCategory_UTXO_CATEGORY_ASSET,
				ContentStrategy: &utxo.UTXO_CachedOutput{
					CachedOutput: &pb.TxOutput{
						OutputContent: &pb.TxOutput_Asset{
							Asset: &pb.AssetOutput{
								AssetContent: &pb.AssetOutput_ContractToken{
									ContractToken: &pb.ContractTokenAsset{
										ContractAddress: tokenID,
										Amount:          "5000",
									},
								},
							},
						},
					},
				},
			},
		},
	}
	hostABI := createHostRuntimePortsWithUTXOQuery(t, mockUTXOQuery)
	ctx := context.Background()
	address := make([]byte, 20)

	balance, err := hostABI.GetBalance(ctx, address, tokenID)

	assert.NoError(t, err, "应该成功查询代币余额")
	assert.Equal(t, uint64(5000), balance, "余额应该是5000")
}

// TestGetBalance_NilEUTXOQuery 测试nil EUTXOQuery
func TestGetBalance_NilEUTXOQuery(t *testing.T) {
	// 由于NewHostRuntimePorts不允许nil EUTXOQuery，我们需要直接构造HostRuntimePorts
	// 或者使用反射设置私有字段，但更简单的方法是创建一个空的HostRuntimePorts并手动设置
	logger := testutil.NewTestLogger()
	mockChainQuery := &mockChainQueryForHostABI{}
	mockUTXOQuery := &mockUTXOQueryForHostABI{} // 先创建正常的
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
	require.NoError(t, err, "应该成功创建HostRuntimePorts")

	// 手动设置为nil以测试nil检查
	hostABIPtr := hostABI.(*HostRuntimePorts)
	hostABIPtr.eutxoQuery = nil

	ctx := context.Background()
	address := make([]byte, 20)

	balance, err := hostABIPtr.GetBalance(ctx, address, nil)

	assert.Error(t, err, "应该返回错误")
	assert.Equal(t, uint64(0), balance, "余额应该为0")
	assert.Contains(t, err.Error(), "eutxoQuery 未初始化", "错误信息应该正确")
}

// TestGetBalance_GetUTXOsByAddressFailed 测试GetUTXOsByAddress失败
func TestGetBalance_GetUTXOsByAddressFailed(t *testing.T) {
	mockUTXOQuery := &mockUTXOQueryForPorts{
		getUTXOsByAddressError: assert.AnError,
	}
	hostABI := createHostRuntimePortsWithUTXOQuery(t, mockUTXOQuery)
	ctx := context.Background()
	address := make([]byte, 20)

	balance, err := hostABI.GetBalance(ctx, address, nil)

	assert.Error(t, err, "应该返回错误")
	assert.Equal(t, uint64(0), balance, "余额应该为0")
	assert.Contains(t, err.Error(), "查询余额失败", "错误信息应该正确")
}

// TestGetBalance_EmptyUTXOList 测试空UTXO列表
func TestGetBalance_EmptyUTXOList(t *testing.T) {
	mockUTXOQuery := &mockUTXOQueryForPorts{
		utxos: []*utxo.UTXO{},
	}
	hostABI := createHostRuntimePortsWithUTXOQuery(t, mockUTXOQuery)
	ctx := context.Background()
	address := make([]byte, 20)

	balance, err := hostABI.GetBalance(ctx, address, nil)

	assert.NoError(t, err, "应该成功查询（空列表）")
	assert.Equal(t, uint64(0), balance, "余额应该为0")
}

// TestGetBalance_FilterNonAssetUTXO 测试过滤非Asset类型UTXO
func TestGetBalance_FilterNonAssetUTXO(t *testing.T) {
	mockUTXOQuery := &mockUTXOQueryForPorts{
		utxos: []*utxo.UTXO{
			{
				Category: utxo.UTXOCategory_UTXO_CATEGORY_RESOURCE, // 非Asset类型
				ContentStrategy: &utxo.UTXO_CachedOutput{
					CachedOutput: &pb.TxOutput{},
				},
			},
			{
				Category: utxo.UTXOCategory_UTXO_CATEGORY_ASSET,
				ContentStrategy: &utxo.UTXO_CachedOutput{
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
		},
	}
	hostABI := createHostRuntimePortsWithUTXOQuery(t, mockUTXOQuery)
	ctx := context.Background()
	address := make([]byte, 20)

	balance, err := hostABI.GetBalance(ctx, address, nil)

	assert.NoError(t, err, "应该成功查询")
	assert.Equal(t, uint64(1000), balance, "余额应该是1000（只计算Asset类型）")
}

// TestGetBalance_NilUTXOItem 测试nil UTXO项
func TestGetBalance_NilUTXOItem(t *testing.T) {
	mockUTXOQuery := &mockUTXOQueryForPorts{
		utxos: []*utxo.UTXO{
			nil, // nil UTXO项
			{
				Category: utxo.UTXOCategory_UTXO_CATEGORY_ASSET,
				ContentStrategy: &utxo.UTXO_CachedOutput{
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
		},
	}
	hostABI := createHostRuntimePortsWithUTXOQuery(t, mockUTXOQuery)
	ctx := context.Background()
	address := make([]byte, 20)

	balance, err := hostABI.GetBalance(ctx, address, nil)

	assert.NoError(t, err, "应该成功查询（跳过nil项）")
	assert.Equal(t, uint64(1000), balance, "余额应该是1000")
}

// TestGetBalance_NoCachedOutput 测试没有缓存输出
func TestGetBalance_NoCachedOutput(t *testing.T) {
	mockUTXOQuery := &mockUTXOQueryForPorts{
		utxos: []*utxo.UTXO{
			{
				Category: utxo.UTXOCategory_UTXO_CATEGORY_ASSET,
				ContentStrategy: &utxo.UTXO_ReferenceOnly{
					ReferenceOnly: true, // 只有引用，没有缓存输出
				},
			},
		},
	}
	hostABI := createHostRuntimePortsWithUTXOQuery(t, mockUTXOQuery)
	ctx := context.Background()
	address := make([]byte, 20)

	balance, err := hostABI.GetBalance(ctx, address, nil)

	assert.NoError(t, err, "应该成功查询（跳过无缓存输出）")
	assert.Equal(t, uint64(0), balance, "余额应该为0")
}

// TestGetBalance_NoAssetOutput 测试没有AssetOutput
func TestGetBalance_NoAssetOutput(t *testing.T) {
	mockUTXOQuery := &mockUTXOQueryForPorts{
		utxos: []*utxo.UTXO{
			{
				Category: utxo.UTXOCategory_UTXO_CATEGORY_ASSET,
				ContentStrategy: &utxo.UTXO_CachedOutput{
					CachedOutput: &pb.TxOutput{
						OutputContent: &pb.TxOutput_Resource{
							Resource: &pb.ResourceOutput{}, // 非Asset输出
						},
					},
				},
			},
		},
	}
	hostABI := createHostRuntimePortsWithUTXOQuery(t, mockUTXOQuery)
	ctx := context.Background()
	address := make([]byte, 20)

	balance, err := hostABI.GetBalance(ctx, address, nil)

	assert.NoError(t, err, "应该成功查询（跳过非Asset输出）")
	assert.Equal(t, uint64(0), balance, "余额应该为0")
}

// TestGetBalance_TokenIDMismatch 测试代币ID不匹配
func TestGetBalance_TokenIDMismatch(t *testing.T) {
	tokenID1 := make([]byte, 20)
	tokenID1[0] = 0x01
	tokenID2 := make([]byte, 20)
	tokenID2[0] = 0x02
	mockUTXOQuery := &mockUTXOQueryForPorts{
		utxos: []*utxo.UTXO{
			{
				Category: utxo.UTXOCategory_UTXO_CATEGORY_ASSET,
				ContentStrategy: &utxo.UTXO_CachedOutput{
					CachedOutput: &pb.TxOutput{
						OutputContent: &pb.TxOutput_Asset{
							Asset: &pb.AssetOutput{
								AssetContent: &pb.AssetOutput_ContractToken{
									ContractToken: &pb.ContractTokenAsset{
										ContractAddress: tokenID1, // 不同的tokenID
										Amount:          "5000",
									},
								},
							},
						},
					},
				},
			},
		},
	}
	hostABI := createHostRuntimePortsWithUTXOQuery(t, mockUTXOQuery)
	ctx := context.Background()
	address := make([]byte, 20)

	balance, err := hostABI.GetBalance(ctx, address, tokenID2)

	assert.NoError(t, err, "应该成功查询（跳过不匹配的代币）")
	assert.Equal(t, uint64(0), balance, "余额应该为0")
}

// TestGetBalance_InvalidAmount 测试无效金额字符串
func TestGetBalance_InvalidAmount(t *testing.T) {
	mockUTXOQuery := &mockUTXOQueryForPorts{
		utxos: []*utxo.UTXO{
			{
				Outpoint: &pb.OutPoint{
					TxId:        make([]byte, 32),
					OutputIndex: 0,
				},
				Category: utxo.UTXOCategory_UTXO_CATEGORY_ASSET,
				ContentStrategy: &utxo.UTXO_CachedOutput{
					CachedOutput: &pb.TxOutput{
						OutputContent: &pb.TxOutput_Asset{
							Asset: &pb.AssetOutput{
								AssetContent: &pb.AssetOutput_NativeCoin{
									NativeCoin: &pb.NativeCoinAsset{
										Amount: "invalid_amount", // 无效金额
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
				Category: utxo.UTXOCategory_UTXO_CATEGORY_ASSET,
				ContentStrategy: &utxo.UTXO_CachedOutput{
					CachedOutput: &pb.TxOutput{
						OutputContent: &pb.TxOutput_Asset{
							Asset: &pb.AssetOutput{
								AssetContent: &pb.AssetOutput_NativeCoin{
									NativeCoin: &pb.NativeCoinAsset{
										Amount: "1000", // 有效金额
									},
								},
							},
						},
					},
				},
			},
		},
	}
	hostABI := createHostRuntimePortsWithUTXOQuery(t, mockUTXOQuery)
	ctx := context.Background()
	address := make([]byte, 20)

	balance, err := hostABI.GetBalance(ctx, address, nil)

	assert.NoError(t, err, "应该成功查询（跳过无效金额）")
	assert.Equal(t, uint64(1000), balance, "余额应该是1000（只计算有效金额）")
}

// TestGetBalance_EmptyAmount 测试空金额字符串
func TestGetBalance_EmptyAmount(t *testing.T) {
	mockUTXOQuery := &mockUTXOQueryForPorts{
		utxos: []*utxo.UTXO{
			{
				Category: utxo.UTXOCategory_UTXO_CATEGORY_ASSET,
				ContentStrategy: &utxo.UTXO_CachedOutput{
					CachedOutput: &pb.TxOutput{
						OutputContent: &pb.TxOutput_Asset{
							Asset: &pb.AssetOutput{
								AssetContent: &pb.AssetOutput_NativeCoin{
									NativeCoin: &pb.NativeCoinAsset{
										Amount: "", // 空金额
									},
								},
							},
						},
					},
				},
			},
		},
	}
	hostABI := createHostRuntimePortsWithUTXOQuery(t, mockUTXOQuery)
	ctx := context.Background()
	address := make([]byte, 20)

	balance, err := hostABI.GetBalance(ctx, address, nil)

	assert.NoError(t, err, "应该成功查询（跳过空金额）")
	assert.Equal(t, uint64(0), balance, "余额应该为0")
}

// TestGetBalance_ContractTokenAddressLengthMismatch 测试合约地址长度不匹配
func TestGetBalance_ContractTokenAddressLengthMismatch(t *testing.T) {
	tokenID := make([]byte, 20)
	mockUTXOQuery := &mockUTXOQueryForPorts{
		utxos: []*utxo.UTXO{
			{
				Category: utxo.UTXOCategory_UTXO_CATEGORY_ASSET,
				ContentStrategy: &utxo.UTXO_CachedOutput{
					CachedOutput: &pb.TxOutput{
						OutputContent: &pb.TxOutput_Asset{
							Asset: &pb.AssetOutput{
								AssetContent: &pb.AssetOutput_ContractToken{
									ContractToken: &pb.ContractTokenAsset{
										ContractAddress: make([]byte, 19), // 长度不匹配（19 vs 20）
										Amount:          "5000",
									},
								},
							},
						},
					},
				},
			},
		},
	}
	hostABI := createHostRuntimePortsWithUTXOQuery(t, mockUTXOQuery)
	ctx := context.Background()
	address := make([]byte, 20)

	balance, err := hostABI.GetBalance(ctx, address, tokenID)

	assert.NoError(t, err, "应该成功查询（跳过长度不匹配）")
	assert.Equal(t, uint64(0), balance, "余额应该为0")
}

// TestGetTransaction_Success 测试成功查询交易
func TestGetTransaction_Success(t *testing.T) {
	mockTxQuery := &mockTxQueryForPorts{
		tx: &pb.Transaction{
			Inputs:  []*pb.TxInput{},
			Outputs: []*pb.TxOutput{},
		},
		blockHeight: 100,
	}
	hostABI := createHostRuntimePortsWithTxQuery(t, mockTxQuery)
	ctx := context.Background()
	txID := make([]byte, 32)

	tx, height, confirmed, err := hostABI.GetTransaction(ctx, txID)

	assert.NoError(t, err, "应该成功查询交易")
	assert.NotNil(t, tx, "应该返回交易对象")
	assert.Equal(t, uint64(100), height, "区块高度应该正确")
	assert.True(t, confirmed, "应该已确认")
}

// TestGetTransaction_Unconfirmed 测试未确认交易
func TestGetTransaction_Unconfirmed(t *testing.T) {
	mockTxQuery := &mockTxQueryForPorts{
		tx: &pb.Transaction{
			Inputs:  []*pb.TxInput{},
			Outputs: []*pb.TxOutput{},
		},
		blockHeight: 0, // 未确认
	}
	hostABI := createHostRuntimePortsWithTxQuery(t, mockTxQuery)
	ctx := context.Background()
	txID := make([]byte, 32)

	tx, height, confirmed, err := hostABI.GetTransaction(ctx, txID)

	assert.NoError(t, err, "应该成功查询交易")
	assert.NotNil(t, tx, "应该返回交易对象")
	assert.Equal(t, uint64(0), height, "区块高度应该为0")
	assert.False(t, confirmed, "应该未确认")
}

// TestGetTransaction_NilTxQuery 测试nil TxQuery
func TestGetTransaction_NilTxQuery(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockChainQuery := &mockChainQueryForHostABI{}
	mockUTXOQuery := &mockUTXOQueryForHostABI{}
	mockCASStorage := &mockCASStorageForHostABI{}
	mockTxQuery := &mockTxQueryForHostABI{} // 先创建正常的
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
	require.NoError(t, err, "应该成功创建HostRuntimePorts")

	// 手动设置为nil以测试nil检查
	hostABIPtr := hostABI.(*HostRuntimePorts)
	hostABIPtr.txQuery = nil

	ctx := context.Background()
	txID := make([]byte, 32)

	tx, height, confirmed, err := hostABIPtr.GetTransaction(ctx, txID)

	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, tx, "交易应该为nil")
	assert.Equal(t, uint64(0), height, "区块高度应该为0")
	assert.False(t, confirmed, "应该未确认")
	assert.Contains(t, err.Error(), "txQuery 未初始化", "错误信息应该正确")
}

// TestGetTransaction_GetTransactionFailed 测试GetTransaction失败
func TestGetTransaction_GetTransactionFailed(t *testing.T) {
	mockTxQuery := &mockTxQueryForPorts{
		getTransactionError: assert.AnError,
	}
	hostABI := createHostRuntimePortsWithTxQuery(t, mockTxQuery)
	ctx := context.Background()
	txID := make([]byte, 32)

	tx, height, confirmed, err := hostABI.GetTransaction(ctx, txID)

	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, tx, "交易应该为nil")
	assert.Equal(t, uint64(0), height, "区块高度应该为0")
	assert.False(t, confirmed, "应该未确认")
	assert.Contains(t, err.Error(), "查询交易失败", "错误信息应该正确")
}

// TestGetTransaction_GetTxBlockHeightFailed 测试GetTxBlockHeight失败
func TestGetTransaction_GetTxBlockHeightFailed(t *testing.T) {
	mockTxQuery := &mockTxQueryForPorts{
		tx: &pb.Transaction{
			Inputs:  []*pb.TxInput{},
			Outputs: []*pb.TxOutput{},
		},
		getTxBlockHeightError: assert.AnError, // GetTxBlockHeight失败
	}
	hostABI := createHostRuntimePortsWithTxQuery(t, mockTxQuery)
	ctx := context.Background()
	txID := make([]byte, 32)

	tx, height, confirmed, err := hostABI.GetTransaction(ctx, txID)

	// GetTxBlockHeight失败时，height设为0，confirmed设为false，但不返回错误
	assert.NoError(t, err, "应该成功查询交易（即使GetTxBlockHeight失败）")
	assert.NotNil(t, tx, "应该返回交易对象")
	assert.Equal(t, uint64(0), height, "区块高度应该为0")
	assert.False(t, confirmed, "应该未确认")
}

// ============================================================================
// 辅助函数
// ============================================================================

// createHostRuntimePortsWithUTXOQuery 创建带UTXOQuery的HostRuntimePorts
func createHostRuntimePortsWithUTXOQuery(t *testing.T, mockUTXOQuery *mockUTXOQueryForPorts) *HostRuntimePorts {
	t.Helper()

	logger := testutil.NewTestLogger()
	mockChainQuery := &mockChainQueryForHostABI{}
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
	require.NoError(t, err, "应该成功创建HostRuntimePorts")

	return hostABI.(*HostRuntimePorts)
}

// createHostRuntimePortsWithTxQuery 创建带TxQuery的HostRuntimePorts
func createHostRuntimePortsWithTxQuery(t *testing.T, mockTxQuery *mockTxQueryForPorts) *HostRuntimePorts {
	t.Helper()

	logger := testutil.NewTestLogger()
	mockChainQuery := &mockChainQueryForHostABI{}
	mockUTXOQuery := &mockUTXOQueryForHostABI{}
	mockCASStorage := &mockCASStorageForHostABI{}
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
	require.NoError(t, err, "应该成功创建HostRuntimePorts")

	return hostABI.(*HostRuntimePorts)
}

// mockUTXOQueryForPorts Mock的UTXO查询服务（用于ports测试）
type mockUTXOQueryForPorts struct {
	utxos                  []*utxo.UTXO
	getUTXOsByAddressError error
}

func (m *mockUTXOQueryForPorts) GetUTXO(ctx context.Context, outpoint *pb.OutPoint) (*utxo.UTXO, error) {
	return nil, nil
}

func (m *mockUTXOQueryForPorts) GetUTXOsByAddress(ctx context.Context, address []byte, category *utxo.UTXOCategory, onlyAvailable bool) ([]*utxo.UTXO, error) {
	if m.getUTXOsByAddressError != nil {
		return nil, m.getUTXOsByAddressError
	}
	return m.utxos, nil
}

func (m *mockUTXOQueryForPorts) GetSponsorPoolUTXOs(ctx context.Context, onlyAvailable bool) ([]*utxo.UTXO, error) {
	return nil, nil
}

func (m *mockUTXOQueryForPorts) GetCurrentStateRoot(ctx context.Context) ([]byte, error) {
	return nil, nil
}

// mockTxQueryForPorts Mock的交易查询服务（用于ports测试）
type mockTxQueryForPorts struct {
	tx                    *pb.Transaction
	blockHeight           uint64
	getTransactionError   error
	getTxBlockHeightError error
}

func (m *mockTxQueryForPorts) GetTransaction(ctx context.Context, txHash []byte) (blockHash []byte, txIndex uint32, transaction *pb.Transaction, err error) {
	if m.getTransactionError != nil {
		return nil, 0, nil, m.getTransactionError
	}
	return make([]byte, 32), 0, m.tx, nil
}

func (m *mockTxQueryForPorts) GetTxBlockHeight(ctx context.Context, txHash []byte) (uint64, error) {
	if m.getTxBlockHeightError != nil {
		return 0, m.getTxBlockHeightError
	}
	return m.blockHeight, nil
}

func (m *mockTxQueryForPorts) GetTransactionsByBlock(ctx context.Context, blockHash []byte) ([]*pb.Transaction, error) {
	return nil, nil
}

func (m *mockTxQueryForPorts) GetAccountNonce(ctx context.Context, address []byte) (uint64, error) {
	return 0, nil
}

func (m *mockTxQueryForPorts) GetBlockTimestamp(ctx context.Context, height uint64) (int64, error) {
	return 0, nil
}

