// Package writer 提供 DataWriter 服务的测试
//
// 🧪 **测试文件**
//
// 本文件测试 DataWriter 服务的核心功能，遵循测试规范：
// - docs/system/standards/principles/testing-standards.md
//
// 🎯 **测试覆盖**：
// - 服务创建
// - 区块写入（单个和批量）
// - 高度验证
// - 事务原子性
// - 错误处理
package writer

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/persistence/testutil"
	_ "github.com/weisyn/v1/internal/core/infrastructure/writegate" // 注册 WriteGate 默认实现，避免单测中 writegate.Default() panic
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ==================== 服务创建测试 ====================

// TestNewService_WithValidDependencies_ReturnsService 测试使用有效依赖创建服务
func TestNewService_WithValidDependencies_ReturnsService(t *testing.T) {
	// Arrange
	storage := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()
	logger := testutil.NewTestLogger()

	// Act
	service := NewService(storage, fileStore, blockHashClient, txHashClient, logger)

	// Assert
	assert.NotNil(t, service)
}

// TestNewService_WithNilStorage_ReturnsService 测试使用 nil storage 创建服务
// 注意：当前实现不检查 nil，允许创建但会在使用时失败
func TestNewService_WithNilStorage_ReturnsService(t *testing.T) {
	// Arrange
	fileStore := testutil.NewTestFileStore()
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()
	logger := testutil.NewTestLogger()

	// Act
	service := NewService(nil, fileStore, blockHashClient, txHashClient, logger)

	// Assert
	// 注意：当前实现不检查 nil，允许创建
	assert.NotNil(t, service)
}

// ==================== 区块写入测试 ====================

// TestWriteBlock_WithGenesisBlock_WritesSuccessfully 测试写入创世区块
func TestWriteBlock_WithGenesisBlock_WritesSuccessfully(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()
	service := NewService(storage, fileStore, blockHashClient, txHashClient, nil)

	genesisBlock := testutil.CreateBlock(0, nil)

	// Act
	err := service.WriteBlock(ctx, genesisBlock)

	// Assert
	assert.NoError(t, err)

	// 验证链尖已更新
	tipKey := []byte("state:chain:tip")
	tipData, err := storage.Get(ctx, tipKey)
	assert.NoError(t, err)
	assert.NotNil(t, tipData)
	assert.GreaterOrEqual(t, len(tipData), 8, "链尖数据应该至少包含8字节高度")
}

// TestWriteBlock_WithSequentialBlocks_WritesSuccessfully 测试顺序写入区块
func TestWriteBlock_WithSequentialBlocks_WritesSuccessfully(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()
	service := NewService(storage, fileStore, blockHashClient, txHashClient, nil)

	// 先写入创世区块
	genesisBlock := testutil.CreateBlock(0, nil)
	err := service.WriteBlock(ctx, genesisBlock)
	require.NoError(t, err)

	// 计算创世区块哈希（用于下一个区块的 PreviousHash）
	hashResp, err := blockHashClient.ComputeBlockHash(ctx, &core.ComputeBlockHashRequest{Block: genesisBlock}, nil)
	require.NoError(t, err)
	genesisHash := hashResp.Hash

	// Act - 写入高度1的区块
	block1 := testutil.CreateBlock(1, genesisHash)
	err = service.WriteBlock(ctx, block1)

	// Assert
	assert.NoError(t, err)

	// 验证链尖已更新为高度1
	tipKey := []byte("state:chain:tip")
	tipData, err := storage.Get(ctx, tipKey)
	assert.NoError(t, err)
	assert.NotNil(t, tipData)
}

// TestWriteBlock_WithInvalidHeight_ReturnsError 测试写入无效高度的区块
func TestWriteBlock_WithInvalidHeight_ReturnsError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()
	service := NewService(storage, fileStore, blockHashClient, txHashClient, nil)

	// 先写入创世区块
	genesisBlock := testutil.CreateBlock(0, nil)
	err := service.WriteBlock(ctx, genesisBlock)
	require.NoError(t, err)

	// Act - 尝试写入高度3的区块（跳过高度1和2）
	// 注意：BlockHeader 没有 Hash 字段，需要通过 blockHashClient 计算
	hashResp, err := blockHashClient.ComputeBlockHash(ctx, &core.ComputeBlockHashRequest{Block: genesisBlock}, nil)
	require.NoError(t, err)
	genesisHash := hashResp.Hash
	block3 := testutil.CreateBlock(3, genesisHash)
	err = service.WriteBlock(ctx, block3)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "期望")
}

// TestWriteBlock_WithDuplicateGenesisBlock_ReturnsError 测试重复写入创世区块
func TestWriteBlock_WithDuplicateGenesisBlock_ReturnsError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()
	service := NewService(storage, fileStore, blockHashClient, txHashClient, nil)

	genesisBlock := testutil.CreateBlock(0, nil)
	err := service.WriteBlock(ctx, genesisBlock)
	require.NoError(t, err)

	// Act - 再次写入创世区块
	err = service.WriteBlock(ctx, genesisBlock)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "创世区块")
}

// TestWriteBlock_WithoutGenesisBlock_ReturnsError 测试未初始化链时写入非创世区块
func TestWriteBlock_WithoutGenesisBlock_ReturnsError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()
	service := NewService(storage, fileStore, blockHashClient, txHashClient, nil)

	// Act - 尝试写入高度1的区块（链未初始化）
	block1 := testutil.CreateBlock(1, testutil.RandomHash())
	err := service.WriteBlock(ctx, block1)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "创世区块")
}

// ==================== 批量写入测试 ====================

// TestWriteBlocks_WithSequentialBlocks_WritesSuccessfully 测试批量写入顺序区块
func TestWriteBlocks_WithSequentialBlocks_WritesSuccessfully(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()
	service := NewService(storage, fileStore, blockHashClient, txHashClient, nil)

	// 先写入创世区块
	genesisBlock := testutil.CreateBlock(0, nil)
	err := service.WriteBlock(ctx, genesisBlock)
	require.NoError(t, err)

	// 计算创世区块哈希
	hashResp, err := blockHashClient.ComputeBlockHash(ctx, &core.ComputeBlockHashRequest{Block: genesisBlock}, nil)
	require.NoError(t, err)
	genesisHash := hashResp.Hash

	// 创建连续区块
	blocks := []*core.Block{
		testutil.CreateBlock(1, genesisHash),
		testutil.CreateBlock(2, testutil.RandomHash()),
		testutil.CreateBlock(3, testutil.RandomHash()),
	}

	// Act
	err = service.WriteBlocks(ctx, blocks)

	// Assert
	assert.NoError(t, err)
}

// TestWriteBlocks_WithEmptyList_ReturnsError 测试批量写入空列表
func TestWriteBlocks_WithEmptyList_ReturnsError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()
	service := NewService(storage, fileStore, blockHashClient, txHashClient, nil)

	// Act
	err := service.WriteBlocks(ctx, []*core.Block{})

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "为空")
}

// TestWriteBlocks_WithNonSequentialBlocks_ReturnsError 测试批量写入非连续区块
func TestWriteBlocks_WithNonSequentialBlocks_ReturnsError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()
	service := NewService(storage, fileStore, blockHashClient, txHashClient, nil)

	// 先写入创世区块
	genesisBlock := testutil.CreateBlock(0, nil)
	err := service.WriteBlock(ctx, genesisBlock)
	require.NoError(t, err)

	// 计算创世区块哈希
	hashResp, err := blockHashClient.ComputeBlockHash(ctx, &core.ComputeBlockHashRequest{Block: genesisBlock}, nil)
	require.NoError(t, err)
	genesisHash := hashResp.Hash

	// 创建非连续区块（跳过高度2）
	blocks := []*core.Block{
		testutil.CreateBlock(1, genesisHash),
		testutil.CreateBlock(3, testutil.RandomHash()), // 跳过高度2
	}

	// Act
	err = service.WriteBlocks(ctx, blocks)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不连续")
}

// ==================== 高度获取测试 ====================

// TestWriteBlock_WithEmptyChain_HeightIsZero 测试空链时写入创世区块
// 注意：getCurrentHeight 是私有方法，通过 WriteBlock 间接测试
func TestWriteBlock_WithEmptyChain_HeightIsZero(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()
	service := NewService(storage, fileStore, blockHashClient, txHashClient, nil)

	genesisBlock := testutil.CreateBlock(0, nil)

	// Act
	err := service.WriteBlock(ctx, genesisBlock)

	// Assert
	assert.NoError(t, err)

	// 验证链尖已更新（高度为0）
	tipKey := []byte("state:chain:tip")
	tipData, err := storage.Get(ctx, tipKey)
	assert.NoError(t, err)
	assert.NotNil(t, tipData)
	assert.GreaterOrEqual(t, len(tipData), 8, "链尖数据应该至少包含8字节高度")
}

// ==================== 交易索引删除测试 ====================

// TestDeleteBlockTransactionIndices_WithValidBlock_DeletesSuccessfully 测试删除区块交易索引
func TestDeleteBlockTransactionIndices_WithValidBlock_DeletesSuccessfully(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()
	service := NewService(storage, fileStore, blockHashClient, txHashClient, nil)

	// 创建带交易的区块
	block := testutil.CreateBlock(0, nil)
	block.Body = &core.BlockBody{
		Transactions: []*transaction.Transaction{
			testutil.CreateTransaction(),
			testutil.CreateTransaction(),
		},
	}

	// 先写入区块（创建交易索引）
	err := service.WriteBlock(ctx, block)
	require.NoError(t, err)

	// 验证交易索引已创建
	txHash1, err := txHashClient.ComputeHash(ctx, &transaction.ComputeHashRequest{Transaction: block.Body.Transactions[0]}, nil)
	require.NoError(t, err)
	txKey1 := []byte(fmt.Sprintf("indices:tx:%x", txHash1.Hash))
	_, err = storage.Get(ctx, txKey1)
	assert.NoError(t, err, "交易索引应该存在")

	// Act - 删除交易索引
	err = service.DeleteBlockTransactionIndices(ctx, block)

	// Assert
	assert.NoError(t, err)

	// 验证交易索引已删除（MockBadgerStore 在键不存在时可能返回 nil, nil）
	data, err := storage.Get(ctx, txKey1)
	if err == nil {
		assert.Nil(t, data, "交易索引应该已被删除（数据应为 nil）")
	} else {
		assert.Error(t, err, "交易索引应该已被删除（应该返回错误）")
	}
}

// TestDeleteBlockTransactionIndices_WithEmptyBlock_IsIdempotent 测试删除空区块的交易索引
func TestDeleteBlockTransactionIndices_WithEmptyBlock_IsIdempotent(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()
	service := NewService(storage, fileStore, blockHashClient, txHashClient, nil)

	block := testutil.CreateBlock(0, nil)
	block.Body = &core.BlockBody{
		Transactions: []*transaction.Transaction{},
	}

	// Act
	err := service.DeleteBlockTransactionIndices(ctx, block)

	// Assert
	assert.NoError(t, err, "删除空区块的交易索引应该是幂等的")
}

// TestDeleteBlockTransactionIndices_WithNilBody_Panics 测试删除 nil Body 区块的交易索引
// 注意：当前实现不检查 nil Body，会在访问 block.Body.Transactions 时 panic
func TestDeleteBlockTransactionIndices_WithNilBody_Panics(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()
	service := NewService(storage, fileStore, blockHashClient, txHashClient, nil)

	block := testutil.CreateBlock(0, nil)
	block.Body = nil

	// Act & Assert
	// 注意：当前实现不检查 nil Body，会在访问 block.Body.Transactions 时 panic
	assert.Panics(t, func() {
		_ = service.DeleteBlockTransactionIndices(ctx, block)
	}, "删除 nil Body 区块的交易索引应该 panic")
}

// ==================== 边界条件测试 ====================

// TestWriteBlock_WithNilBlock_Panics 测试写入 nil 区块
// 注意：当前实现不检查 nil，会在访问 block.Header 时 panic
func TestWriteBlock_WithNilBlock_Panics(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()
	service := NewService(storage, fileStore, blockHashClient, txHashClient, nil)

	// Act & Assert
	// 注意：当前实现不检查 nil，会在访问 block.Header 时 panic
	// 这里验证会 panic（使用 recover 捕获）
	assert.Panics(t, func() {
		_ = service.WriteBlock(ctx, nil)
	}, "写入 nil 区块应该 panic")
}

// TestWriteBlock_WithNilBlockHeader_Panics 测试写入 nil Header 的区块
// 注意：当前实现不检查 nil Header，会在访问 block.Header.Height 时 panic
func TestWriteBlock_WithNilBlockHeader_Panics(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()
	service := NewService(storage, fileStore, blockHashClient, txHashClient, nil)

	block := &core.Block{
		Header: nil,
		Body:   &core.BlockBody{},
	}

	// Act & Assert
	// 注意：当前实现不检查 nil Header，会在访问 block.Header.Height 时 panic
	assert.Panics(t, func() {
		_ = service.WriteBlock(ctx, block)
	}, "写入 nil Header 的区块应该 panic")
}
