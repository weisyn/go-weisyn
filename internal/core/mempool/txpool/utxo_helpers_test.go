// Package txpool UTXO辅助方法测试
package txpool

import (
	"testing"

	"github.com/stretchr/testify/assert"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// TestEqualOutPoint_WithEqualOutPoints_ReturnsTrue 测试equalOutPoint方法 - 相等的OutPoint
func TestEqualOutPoint_WithEqualOutPoints_ReturnsTrue(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	txID := []byte("test_tx_id_32_bytes_12345678")
	op1 := &transaction.OutPoint{
		TxId:        txID,
		OutputIndex: 0,
	}
	op2 := &transaction.OutPoint{
		TxId:        txID,
		OutputIndex: 0,
	}

	// Act
	result := pool.equalOutPoint(op1, op2)

	// Assert
	assert.True(t, result, "相等的OutPoint应该返回true")
}

// TestEqualOutPoint_WithDifferentTxID_ReturnsFalse 测试不同的TxID
func TestEqualOutPoint_WithDifferentTxID_ReturnsFalse(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	txID1 := []byte("test_tx_id_1_32_bytes_12345678")
	txID2 := []byte("test_tx_id_2_32_bytes_12345678")
	op1 := &transaction.OutPoint{
		TxId:        txID1,
		OutputIndex: 0,
	}
	op2 := &transaction.OutPoint{
		TxId:        txID2,
		OutputIndex: 0,
	}

	// Act
	result := pool.equalOutPoint(op1, op2)

	// Assert
	assert.False(t, result, "不同TxID的OutPoint应该返回false")
}

// TestEqualOutPoint_WithDifferentOutputIndex_ReturnsFalse 测试不同的OutputIndex
func TestEqualOutPoint_WithDifferentOutputIndex_ReturnsFalse(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	txID := []byte("test_tx_id_32_bytes_12345678")
	op1 := &transaction.OutPoint{
		TxId:        txID,
		OutputIndex: 0,
	}
	op2 := &transaction.OutPoint{
		TxId:        txID,
		OutputIndex: 1,
	}

	// Act
	result := pool.equalOutPoint(op1, op2)

	// Assert
	assert.False(t, result, "不同OutputIndex的OutPoint应该返回false")
}

// TestEqualOutPoint_WithNilOutPoints_ReturnsFalse 测试nil OutPoint
func TestEqualOutPoint_WithNilOutPoints_ReturnsFalse(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	op1 := &transaction.OutPoint{
		TxId:        []byte("test_tx_id_32_bytes_12345678"),
		OutputIndex: 0,
	}

	// Act
	result1 := pool.equalOutPoint(nil, op1)
	result2 := pool.equalOutPoint(op1, nil)
	result3 := pool.equalOutPoint(nil, nil)

	// Assert
	assert.False(t, result1, "nil OutPoint应该返回false")
	assert.False(t, result2, "nil OutPoint应该返回false")
	assert.False(t, result3, "两个nil OutPoint应该返回false")
}

// TestEqualOutPoint_WithDifferentLengthTxID_ReturnsFalse 测试不同长度的TxID
func TestEqualOutPoint_WithDifferentLengthTxID_ReturnsFalse(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	op1 := &transaction.OutPoint{
		TxId:        []byte("short"),
		OutputIndex: 0,
	}
	op2 := &transaction.OutPoint{
		TxId:        []byte("longer_tx_id"),
		OutputIndex: 0,
	}

	// Act
	result := pool.equalOutPoint(op1, op2)

	// Assert
	assert.False(t, result, "不同长度TxID的OutPoint应该返回false")
}

