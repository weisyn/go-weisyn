// Package txpool 费用估算测试
package txpool

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/config/txpool"
	"github.com/weisyn/v1/internal/core/mempool/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// TestEstimateTransactionFee_WithNilTx_ReturnsZero 测试nil交易
func TestEstimateTransactionFee_WithNilTx_ReturnsZero(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act
	fee := pool.estimateTransactionFee(nil)

	// Assert
	assert.Equal(t, uint64(0), fee, "nil交易应该返回0费用")
}

// TestEstimateTransactionFee_WithCoinbaseTx_ReturnsZero 测试Coinbase交易（无输入）
func TestEstimateTransactionFee_WithCoinbaseTx_ReturnsZero(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := &transaction.Transaction{
		Nonce:  1,
		Inputs: []*transaction.TxInput{}, // 无输入
		Outputs: []*transaction.TxOutput{
			{
				Owner: []byte("recipient_coinbase"),
				OutputContent: &transaction.TxOutput_Asset{
					Asset: &transaction.AssetOutput{
						AssetContent: &transaction.AssetOutput_NativeCoin{
							NativeCoin: &transaction.NativeCoinAsset{
								Amount: "1000000",
							},
						},
					},
				},
			},
		},
	}

	// Act
	fee := pool.estimateTransactionFee(tx)

	// Assert
	assert.Equal(t, uint64(0), fee, "Coinbase交易应该返回0费用")
}

// TestEstimateTransactionFee_WithSimpleTx_ReturnsFee 测试简单交易
func TestEstimateTransactionFee_WithSimpleTx_ReturnsFee(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)

	// Act
	fee := pool.estimateTransactionFee(tx)

	// Assert
	assert.Greater(t, fee, uint64(0), "简单交易应该返回>0的费用")
	assert.GreaterOrEqual(t, fee, uint64(10000), "费用应该>=最小费用限制（10000）")
}

// TestEstimateTransactionFee_WithMultipleInputs_IncreasesFee 测试多输入交易
func TestEstimateTransactionFee_WithMultipleInputs_IncreasesFee(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	
	// 创建单输入交易
	tx1 := testutil.CreateTestTransaction(1, []*transaction.TxInput{
		{
			PreviousOutput: &transaction.OutPoint{
				TxId:        []byte("tx1_id_32_bytes_12345678"),
				OutputIndex: 0,
			},
		},
	}, nil)

	// 创建多输入交易
	tx2 := testutil.CreateTestTransaction(2, []*transaction.TxInput{
		{
			PreviousOutput: &transaction.OutPoint{
				TxId:        []byte("tx2_id_32_bytes_12345678"),
				OutputIndex: 0,
			},
		},
		{
			PreviousOutput: &transaction.OutPoint{
				TxId:        []byte("tx2_id_32_bytes_12345678"),
				OutputIndex: 1,
			},
		},
		{
			PreviousOutput: &transaction.OutPoint{
				TxId:        []byte("tx2_id_32_bytes_12345678"),
				OutputIndex: 2,
			},
		},
	}, nil)

	// Act
	fee1 := pool.estimateTransactionFee(tx1)
	fee2 := pool.estimateTransactionFee(tx2)

	// Assert
	assert.Greater(t, fee2, fee1, "多输入交易应该费用更高")
}

// TestEstimateTransactionFee_WithMultipleOutputs_IncreasesFee 测试多输出交易
func TestEstimateTransactionFee_WithMultipleOutputs_IncreasesFee(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	
	// 创建单输出交易
	tx1 := testutil.CreateTestTransaction(1, nil, []*transaction.TxOutput{
		{
			Owner: []byte("recipient_1"),
			OutputContent: &transaction.TxOutput_Asset{
				Asset: &transaction.AssetOutput{
					AssetContent: &transaction.AssetOutput_NativeCoin{
						NativeCoin: &transaction.NativeCoinAsset{
							Amount: "1000000",
						},
					},
				},
			},
		},
	})

	// 创建多输出交易
	tx2 := testutil.CreateTestTransaction(2, nil, []*transaction.TxOutput{
		{
			Owner: []byte("recipient_2_1"),
			OutputContent: &transaction.TxOutput_Asset{
				Asset: &transaction.AssetOutput{
					AssetContent: &transaction.AssetOutput_NativeCoin{
						NativeCoin: &transaction.NativeCoinAsset{
							Amount: "1000000",
						},
					},
				},
			},
		},
		{
			Owner: []byte("recipient_2_2"),
			OutputContent: &transaction.TxOutput_Asset{
				Asset: &transaction.AssetOutput{
					AssetContent: &transaction.AssetOutput_NativeCoin{
						NativeCoin: &transaction.NativeCoinAsset{
							Amount: "2000000",
						},
					},
				},
			},
		},
		{
			Owner: []byte("recipient_2_3"),
			OutputContent: &transaction.TxOutput_Asset{
				Asset: &transaction.AssetOutput{
					AssetContent: &transaction.AssetOutput_NativeCoin{
						NativeCoin: &transaction.NativeCoinAsset{
							Amount: "3000000",
						},
					},
				},
			},
		},
	})

	// Act
	fee1 := pool.estimateTransactionFee(tx1)
	fee2 := pool.estimateTransactionFee(tx2)

	// Assert
	assert.Greater(t, fee2, fee1, "多输出交易应该费用更高")
}

// TestEstimateTransactionFee_WithLargeTx_IncreasesFee 测试大交易
func TestEstimateTransactionFee_WithLargeTx_IncreasesFee(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	
	// 创建小交易
	tx1 := testutil.CreateSimpleTestTransaction(1)

	// 创建大交易（多个输入和输出）
	inputs := make([]*transaction.TxInput, 10)
	outputs := make([]*transaction.TxOutput, 10)
	for i := 0; i < 10; i++ {
		inputs[i] = &transaction.TxInput{
			PreviousOutput: &transaction.OutPoint{
				TxId:        []byte("large_tx_id_32_bytes_12345678"),
				OutputIndex: uint32(i),
			},
		}
		outputs[i] = &transaction.TxOutput{
			Owner: []byte(fmt.Sprintf("recipient_%d", i)),
			OutputContent: &transaction.TxOutput_Asset{
				Asset: &transaction.AssetOutput{
					AssetContent: &transaction.AssetOutput_NativeCoin{
						NativeCoin: &transaction.NativeCoinAsset{
							Amount: "1000000",
						},
					},
				},
			},
		}
	}
	tx2 := testutil.CreateTestTransaction(2, inputs, outputs)

	// Act
	fee1 := pool.estimateTransactionFee(tx1)
	fee2 := pool.estimateTransactionFee(tx2)

	// Assert
	assert.Greater(t, fee2, fee1, "大交易应该费用更高")
}

// TestEstimateTransactionFee_WithMinFee_AppliesMinimum 测试最小费用限制
func TestEstimateTransactionFee_WithMinFee_AppliesMinimum(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	// 创建一个非常小的交易（可能触发最小费用限制）
	tx := &transaction.Transaction{
		Nonce:  1,
		Inputs: []*transaction.TxInput{
			{
				PreviousOutput: &transaction.OutPoint{
					TxId:        []byte("small_tx"),
					OutputIndex: 0,
				},
			},
		},
		Outputs: []*transaction.TxOutput{
			{
				Owner: []byte("recipient_small"),
				OutputContent: &transaction.TxOutput_Asset{
					Asset: &transaction.AssetOutput{
						AssetContent: &transaction.AssetOutput_NativeCoin{
							NativeCoin: &transaction.NativeCoinAsset{
								Amount: "1000",
							},
						},
					},
				},
			},
		},
	}

	// Act
	fee := pool.estimateTransactionFee(tx)

	// Assert
	assert.GreaterOrEqual(t, fee, uint64(10000), "费用应该>=最小费用限制（10000）")
}

// TestEstimateTransactionFee_WithPriceLimit_UsesConfig 测试使用配置的PriceLimit
func TestEstimateTransactionFee_WithPriceLimit_UsesConfig(t *testing.T) {
	// Arrange
	// 创建带PriceLimit配置的交易池
	config := &txpool.TxPoolOptions{
		MaxSize:    1000,
		MemoryLimit: 100 * 1024 * 1024,
		MaxTxSize:  1024 * 1024,
		PriceLimit: 200000, // 设置PriceLimit
		Mining: txpool.MiningOptions{
			MaxTransactionsForMining: 100,
			MaxBlockSizeForMining:     1024 * 1024,
		},
	}
	logger := &testutil.MockLogger{}
	eventBus := testutil.NewMockEventBus()
	memory := testutil.NewMockMemoryStore()
	hashService := &testutil.MockTransactionHashService{}

	pool, err := NewTxPoolWithCache(config, logger, eventBus, memory, hashService, nil)
	require.NoError(t, err)
	txPool := pool.(*TxPool)

	tx := testutil.CreateSimpleTestTransaction(1)

	// Act
	fee := txPool.estimateTransactionFee(tx)

	// Assert
	assert.Greater(t, fee, uint64(0), "应该返回>0的费用")
	// 注意：PriceLimit会影响费率计算，但具体值取决于实现
}

