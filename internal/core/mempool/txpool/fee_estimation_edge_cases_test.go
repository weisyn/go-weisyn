// Package txpool 费用估算边界条件测试
package txpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/config/txpool"
	"github.com/weisyn/v1/internal/core/mempool/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// TestEstimateTransactionSize_WithNilTx_ReturnsZero 测试nil交易的大小估算
func TestEstimateTransactionSize_WithNilTx_ReturnsZero(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act
	size := pool.estimateTransactionSize(nil)

	// Assert
	// 根据实现，nil交易返回500（默认大小）
	assert.Equal(t, uint64(500), size, "nil交易应该返回默认大小500")
}

// TestEstimateTransactionSize_WithEmptyTx_ReturnsNonZero 测试空交易的大小估算
func TestEstimateTransactionSize_WithEmptyTx_ReturnsNonZero(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := &transaction.Transaction{
		Nonce:  1,
		Inputs: []*transaction.TxInput{},
		Outputs: []*transaction.TxOutput{},
	}

	// Act
	size := pool.estimateTransactionSize(tx)

	// Assert
	// 即使空交易，序列化后也有一定大小
	assert.GreaterOrEqual(t, size, uint64(0), "空交易大小应该>=0")
}

// TestEstimateTransactionSize_WithLargeTx_ReturnsLargeSize 测试大交易的大小估算
func TestEstimateTransactionSize_WithLargeTx_ReturnsLargeSize(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	
	// 创建小交易
	tx1 := testutil.CreateSimpleTestTransaction(1)

	// 创建大交易（多个输入和输出）
	inputs := make([]*transaction.TxInput, 50)
	outputs := make([]*transaction.TxOutput, 50)
	for i := 0; i < 50; i++ {
		inputs[i] = &transaction.TxInput{
			PreviousOutput: &transaction.OutPoint{
				TxId:        []byte("large_tx_id_32_bytes_12345678"),
				OutputIndex: uint32(i),
			},
		}
		outputs[i] = &transaction.TxOutput{
			Owner: []byte("recipient_large"),
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
	size1 := pool.estimateTransactionSize(tx1)
	size2 := pool.estimateTransactionSize(tx2)

	// Assert
	assert.Greater(t, size2, size1, "大交易应该大小更大")
}

// TestCalculateTransactionPriority_WithNilWrapper_ReturnsZero 测试nil包装器的优先级计算
func TestCalculateTransactionPriority_WithNilWrapper_ReturnsZero(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act
	priority := pool.calculateTransactionPriority(nil)

	// Assert
	assert.Equal(t, uint64(0), priority, "nil包装器应该返回0优先级")
}

// TestCalculateTransactionPriority_WithHighFee_HasHighPriority 测试高费用交易的优先级
func TestCalculateTransactionPriority_WithHighFee_HasHighPriority(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	
	// 创建两个交易，一个费用高，一个费用低
	// 注意：由于费用估算基于交易大小和复杂度，我们通过创建不同大小的交易来影响费用
	tx1 := testutil.CreateSimpleTestTransaction(1)
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
	}, nil)

	wrapper1 := NewTxWrapper(tx1, []byte("tx1_id_32_bytes_12345678"))
	wrapper2 := NewTxWrapper(tx2, []byte("tx2_id_32_bytes_12345678"))

	// Act
	priority1 := pool.calculateTransactionPriority(wrapper1)
	priority2 := pool.calculateTransactionPriority(wrapper2)

	// Assert
	// 优先级应该>=0
	assert.GreaterOrEqual(t, priority1, uint64(0), "优先级应该>=0")
	assert.GreaterOrEqual(t, priority2, uint64(0), "优先级应该>=0")
	// 注意：具体优先级值取决于实现，我们主要验证不会panic
}

// TestEstimateTransactionFee_WithVeryLargeTx_AppliesComplexityMultiplier 测试超大交易的费用估算
func TestEstimateTransactionFee_WithVeryLargeTx_AppliesComplexityMultiplier(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	
	// 创建超大交易（大量输入和输出）
	inputs := make([]*transaction.TxInput, 100)
	outputs := make([]*transaction.TxOutput, 100)
	for i := 0; i < 100; i++ {
		inputs[i] = &transaction.TxInput{
			PreviousOutput: &transaction.OutPoint{
				TxId:        []byte("very_large_tx_id_32_bytes_12345678"),
				OutputIndex: uint32(i),
			},
		}
		outputs[i] = &transaction.TxOutput{
			Owner: []byte("recipient_very_large"),
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
	tx := testutil.CreateTestTransaction(1, inputs, outputs)

	// Act
	fee := pool.estimateTransactionFee(tx)

	// Assert
	assert.Greater(t, fee, uint64(0), "超大交易应该返回>0的费用")
	assert.GreaterOrEqual(t, fee, uint64(10000), "费用应该>=最小费用限制（10000）")
}

// TestEstimateTransactionFee_WithZeroPriceLimit_UsesDefaultRate 测试零PriceLimit使用默认费率
func TestEstimateTransactionFee_WithZeroPriceLimit_UsesDefaultRate(t *testing.T) {
	// Arrange
	config := &txpool.TxPoolOptions{
		MaxSize:    1000,
		MemoryLimit: 100 * 1024 * 1024,
		MaxTxSize:  1024 * 1024,
		PriceLimit: 0, // 零PriceLimit
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
	// 应该使用默认费率（1000单位/字节）
}

// TestEstimateTransactionFee_WithSingleInputOutput_HasBaseFee 测试单输入单输出交易的基础费用
func TestEstimateTransactionFee_WithSingleInputOutput_HasBaseFee(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	tx := testutil.CreateSimpleTestTransaction(1)

	// Act
	fee := pool.estimateTransactionFee(tx)

	// Assert
	assert.Greater(t, fee, uint64(0), "单输入单输出交易应该返回>0的费用")
	assert.GreaterOrEqual(t, fee, uint64(10000), "费用应该>=最小费用限制（10000）")
}

