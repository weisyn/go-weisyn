// Package txpool hasUTXOConflict覆盖率提升测试
package txpool

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/core/mempool/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// TestHasUTXOConflict_WithSameOutPoint_ReturnsTrue 测试相同OutPoint冲突
func TestHasUTXOConflict_WithSameOutPoint_ReturnsTrue(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	
	outPoint := &transaction.OutPoint{
		TxId:        []byte("parent_tx_id_32_bytes_12345678"),
		OutputIndex: 0,
	}
	
	tx1 := testutil.CreateTestTransaction(1, []*transaction.TxInput{
		{PreviousOutput: outPoint},
	}, nil)
	tx2 := testutil.CreateTestTransaction(2, []*transaction.TxInput{
		{PreviousOutput: outPoint},
	}, nil)
	
	// Act
	hasConflict := pool.hasUTXOConflict(tx1, tx2)
	
	// Assert
	assert.True(t, hasConflict, "相同OutPoint应该冲突")
}

// TestHasUTXOConflict_WithDifferentOutPoint_ReturnsFalse 测试不同OutPoint不冲突
func TestHasUTXOConflict_WithDifferentOutPoint_ReturnsFalse(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	
	outPoint1 := &transaction.OutPoint{
		TxId:        []byte("parent_tx_id_32_bytes_12345678"),
		OutputIndex: 0,
	}
	outPoint2 := &transaction.OutPoint{
		TxId:        []byte("parent_tx_id_32_bytes_87654321"),
		OutputIndex: 1,
	}
	
	tx1 := testutil.CreateTestTransaction(1, []*transaction.TxInput{
		{PreviousOutput: outPoint1},
	}, nil)
	tx2 := testutil.CreateTestTransaction(2, []*transaction.TxInput{
		{PreviousOutput: outPoint2},
	}, nil)
	
	// Act
	hasConflict := pool.hasUTXOConflict(tx1, tx2)
	
	// Assert
	assert.False(t, hasConflict, "不同OutPoint不应该冲突")
}

// TestHasUTXOConflict_WithNilPreviousOutput_ReturnsFalse 测试nil PreviousOutput
func TestHasUTXOConflict_WithNilPreviousOutput_ReturnsFalse(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	
	tx1 := testutil.CreateTestTransaction(1, []*transaction.TxInput{
		{PreviousOutput: nil},
	}, nil)
	tx2 := testutil.CreateTestTransaction(2, []*transaction.TxInput{
		{PreviousOutput: nil},
	}, nil)
	
	// Act
	hasConflict := pool.hasUTXOConflict(tx1, tx2)
	
	// Assert
	assert.False(t, hasConflict, "nil PreviousOutput不应该冲突")
}

// TestHasUTXOConflict_WithMixedNilAndValid_ReturnsFalse 测试混合nil和有效OutPoint
func TestHasUTXOConflict_WithMixedNilAndValid_ReturnsFalse(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	
	outPoint := &transaction.OutPoint{
		TxId:        []byte("parent_tx_id_32_bytes_12345678"),
		OutputIndex: 0,
	}
	
	tx1 := testutil.CreateTestTransaction(1, []*transaction.TxInput{
		{PreviousOutput: nil},
	}, nil)
	tx2 := testutil.CreateTestTransaction(2, []*transaction.TxInput{
		{PreviousOutput: outPoint},
	}, nil)
	
	// Act
	hasConflict := pool.hasUTXOConflict(tx1, tx2)
	
	// Assert
	assert.False(t, hasConflict, "nil和有效OutPoint不应该冲突")
}

// TestHasUTXOConflict_WithSameTxIdDifferentIndex_ReturnsFalse 测试相同TxId不同Index
func TestHasUTXOConflict_WithSameTxIdDifferentIndex_ReturnsFalse(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	
	outPoint1 := &transaction.OutPoint{
		TxId:        []byte("parent_tx_id_32_bytes_12345678"),
		OutputIndex: 0,
	}
	outPoint2 := &transaction.OutPoint{
		TxId:        []byte("parent_tx_id_32_bytes_12345678"),
		OutputIndex: 1,
	}
	
	tx1 := testutil.CreateTestTransaction(1, []*transaction.TxInput{
		{PreviousOutput: outPoint1},
	}, nil)
	tx2 := testutil.CreateTestTransaction(2, []*transaction.TxInput{
		{PreviousOutput: outPoint2},
	}, nil)
	
	// Act
	hasConflict := pool.hasUTXOConflict(tx1, tx2)
	
	// Assert
	assert.False(t, hasConflict, "相同TxId不同Index不应该冲突")
}

// TestHasUTXOConflict_WithMultipleInputs_DetectsConflict 测试多输入中的冲突
func TestHasUTXOConflict_WithMultipleInputs_DetectsConflict(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	
	outPoint1 := &transaction.OutPoint{
		TxId:        []byte("parent_tx_id_32_bytes_12345678"),
		OutputIndex: 0,
	}
	outPoint2 := &transaction.OutPoint{
		TxId:        []byte("parent_tx_id_32_bytes_87654321"),
		OutputIndex: 1,
	}
	outPoint3 := &transaction.OutPoint{
		TxId:        []byte("parent_tx_id_32_bytes_12345678"),
		OutputIndex: 0, // 与tx1的第一个输入冲突
	}
	
	tx1 := testutil.CreateTestTransaction(1, []*transaction.TxInput{
		{PreviousOutput: outPoint1},
		{PreviousOutput: outPoint2},
	}, nil)
	tx2 := testutil.CreateTestTransaction(2, []*transaction.TxInput{
		{PreviousOutput: outPoint3},
	}, nil)
	
	// Act
	hasConflict := pool.hasUTXOConflict(tx1, tx2)
	
	// Assert
	assert.True(t, hasConflict, "多输入中应该检测到冲突")
}

