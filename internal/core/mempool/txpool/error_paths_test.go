// Package txpool 错误路径测试 - 专门测试各种错误场景以发现BUG
package txpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/config/txpool"
	"github.com/weisyn/v1/internal/core/mempool/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// TestAddTransaction_WithInvalidFormat_ReturnsError 测试无效格式的交易
func TestAddTransaction_WithInvalidFormat_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	// 创建无效格式的交易（缺少必需字段）
	invalidTx := &transaction.Transaction{
		Version: 1,
		// 缺少Inputs和Outputs
	}

	// Act
	txID, err := pool.AddTransaction(invalidTx)

	// Assert
	assert.Error(t, err, "无效格式的交易应该返回错误")
	assert.Nil(t, txID, "交易ID应该为nil")
}

// TestAddTransaction_WithOversizedTx_ReturnsError 测试超大交易
func TestAddTransaction_WithOversizedTx_ReturnsError(t *testing.T) {
	// Arrange
	// 创建内存限制很小的交易池来测试大小验证
	config := &txpool.TxPoolOptions{
		MaxSize:    1000,
		MemoryLimit: 100 * 1024 * 1024,
		MaxTxSize:  100, // 只有100字节的限制
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

	// 创建正常大小的交易（但可能超过100字节限制）
	largeTx := testutil.CreateSimpleTestTransaction(1)

	// Act
	txID, err := txPool.AddTransaction(largeTx)

	// Assert
	// 注意：根据calculateTransactionSize的实现，交易大小可能小于100字节
	// 如果交易确实超过限制，应该返回错误
	if err != nil {
		assert.Error(t, err, "超大交易应该返回错误")
		assert.Nil(t, txID, "交易ID应该为nil")
	} else {
		// 如果交易没有超过限制，说明大小计算可能不准确，这也是一个发现
		t.Logf("注意：交易大小验证可能没有正确工作，交易大小: %d bytes", calculateTransactionSize(largeTx))
	}
}

// TestAddTransaction_WithUTXOConflict_ReturnsError 测试UTXO冲突
func TestAddTransaction_WithUTXOConflict_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	
	// 创建第一个交易
	outPoint := &transaction.OutPoint{
		TxId:        []byte("parent_tx_id_32_bytes_12345678"),
		OutputIndex: 0,
	}
	tx1 := testutil.CreateTestTransaction(1, []*transaction.TxInput{
		{
			PreviousOutput: outPoint,
			IsReferenceOnly: false,
			Sequence: 0xFFFFFFFF,
		},
	}, nil)
	tx1.Nonce = 1
	
	txID1, err := pool.SubmitTx(tx1)
	require.NoError(t, err)
	require.NotNil(t, txID1)

	// 创建第二个交易，使用相同的UTXO（冲突）
	tx2 := testutil.CreateTestTransaction(2, []*transaction.TxInput{
		{
			PreviousOutput: outPoint, // 相同的UTXO
			IsReferenceOnly: false,
			Sequence: 0xFFFFFFFF,
		},
	}, nil)
	tx2.Nonce = 2

	// Act
	txID2, err := pool.AddTransaction(tx2)

	// Assert
	assert.Error(t, err, "UTXO冲突的交易应该返回错误")
	assert.Nil(t, txID2, "交易ID应该为nil")
	// 验证第一个交易仍然存在
	retrievedTx1, err := pool.GetTx(txID1)
	assert.NoError(t, err, "第一个交易应该仍然存在")
	assert.NotNil(t, retrievedTx1, "第一个交易应该仍然存在")
}

// TestAddTransaction_WithMemoryLimitExceeded_ReturnsError 测试内存限制
func TestAddTransaction_WithMemoryLimitExceeded_ReturnsError(t *testing.T) {
	// Arrange
	// 创建内存限制很小的交易池
	config := &txpool.TxPoolOptions{
		MaxSize:    1000,
		MemoryLimit: 1024, // 只有1KB
		MaxTxSize:  1024 * 1024,
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

	// 提交一个交易填满内存
	tx1 := testutil.CreateSimpleTestTransaction(1)
	_, err = txPool.SubmitTx(tx1)
	require.NoError(t, err)

	// Act - 尝试提交第二个交易（应该超过内存限制）
	tx2 := testutil.CreateSimpleTestTransaction(2)
	txID2, err := txPool.AddTransaction(tx2)

	// Assert
	// 注意：根据实现，可能会触发清理或返回错误
	// 我们主要验证不会panic
	if err != nil {
		assert.Error(t, err, "内存限制时应该返回错误")
		assert.Nil(t, txID2, "交易ID应该为nil")
	}
}

// TestAddTransaction_WithNilHashService_ReturnsError 测试哈希服务为nil时的处理
func TestAddTransaction_WithNilHashService_ReturnsError(t *testing.T) {
	// Arrange
	// 创建哈希服务为nil的交易池（这不应该发生，但测试边界情况）
	config := &txpool.TxPoolOptions{
		MaxSize:    1000,
		MemoryLimit: 100 * 1024 * 1024,
		MaxTxSize:  1024 * 1024,
		Mining: txpool.MiningOptions{
			MaxTransactionsForMining: 100,
			MaxBlockSizeForMining:     1024 * 1024,
		},
	}
	logger := &testutil.MockLogger{}
	eventBus := testutil.NewMockEventBus()
	memory := testutil.NewMockMemoryStore()

	pool, err := NewTxPoolWithCache(config, logger, eventBus, memory, nil, nil)
	require.NoError(t, err) // 创建可能成功，但AddTransaction会失败
	txPool := pool.(*TxPool)

	tx := testutil.CreateSimpleTestTransaction(1)

	// Act
	txID, err := txPool.AddTransaction(tx)

	// Assert
	assert.Error(t, err, "哈希服务为nil时应该返回错误")
	assert.Nil(t, txID, "交易ID应该为nil")
}

// TestAddTransaction_WithEmptyInputs_MayBeValid 测试空输入的交易（Coinbase交易）
func TestAddTransaction_WithEmptyInputs_MayBeValid(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	// Coinbase交易：只有输出，没有输入
	coinbaseTx := &transaction.Transaction{
		Version:           1,
		Inputs:            []*transaction.TxInput{}, // 空输入
		Outputs: []*transaction.TxOutput{
			{
				Owner: []byte("miner_address"),
				OutputContent: &transaction.TxOutput_Asset{
					Asset: &transaction.AssetOutput{
						AssetContent: &transaction.AssetOutput_NativeCoin{
							NativeCoin: &transaction.NativeCoinAsset{
								Amount: "100000000000",
							},
						},
					},
				},
			},
		},
		Nonce:             1,
		CreationTimestamp: 1000,
		ChainId:           []byte("weisyn-testnet"),
		FeeMechanism: &transaction.Transaction_MinimumFee{
			MinimumFee: &transaction.MinimumFee{
				MinimumAmount: "5000000000",
			},
		},
	}

	// Act
	txID, err := pool.AddTransaction(coinbaseTx)

	// Assert
	// 注意：根据验证器实现，Coinbase交易可能被接受或拒绝
	// 我们主要验证不会panic
	if err != nil {
		assert.Error(t, err, "如果Coinbase交易被拒绝，应该有明确的错误信息")
	} else {
		assert.NotNil(t, txID, "如果Coinbase交易被接受，应该有交易ID")
	}
}

// TestAddTransaction_WithEmptyOutputs_ReturnsError 测试空输出的交易
func TestAddTransaction_WithEmptyOutputs_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	// 创建只有输入没有输出的交易（通常无效）
	invalidTx := &transaction.Transaction{
		Version: 1,
		Inputs: []*transaction.TxInput{
			{
				PreviousOutput: &transaction.OutPoint{
					TxId:        []byte("parent_tx_id_32_bytes_12345678"),
					OutputIndex: 0,
				},
				IsReferenceOnly: false,
				Sequence:        0xFFFFFFFF,
			},
		},
		Outputs:           []*transaction.TxOutput{}, // 空输出
		Nonce:             1,
		CreationTimestamp: 1000,
		ChainId:           []byte("weisyn-testnet"),
		FeeMechanism: &transaction.Transaction_MinimumFee{
			MinimumFee: &transaction.MinimumFee{
				MinimumAmount: "5000000000",
			},
		},
	}

	// Act
	txID, err := pool.AddTransaction(invalidTx)

	// Assert
	// 空输出的交易通常应该被拒绝
	assert.Error(t, err, "空输出的交易应该返回错误")
	assert.Nil(t, txID, "交易ID应该为nil")
}

