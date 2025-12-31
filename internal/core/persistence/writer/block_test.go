// Package writer 提供区块数据写入逻辑的测试
//
// 🧪 **测试文件**
//
// 本文件测试区块数据写入的核心功能，遵循测试规范：
// - docs/system/standards/principles/testing-standards.md
//
// 🎯 **测试覆盖**：
// - 区块数据写入
// - 区块哈希计算
// - 文件存储
// - 索引更新
package writer

import (
	"context"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/persistence/testutil"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// ==================== 区块数据写入测试 ====================

// TestWriteBlockData_WithValidBlock_WritesSuccessfully 测试写入区块数据
func TestWriteBlockData_WithValidBlock_WritesSuccessfully(t *testing.T) {
	// Arrange
	ctx := context.Background()
	badgerStore := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()
	service := NewService(badgerStore, fileStore, blockHashClient, txHashClient, nil)

	block := testutil.CreateBlock(100, testutil.RandomHash())

	// 类型断言为 *Service 以访问内部方法
	serviceImpl := service.(*Service)

	// 创建事务
	err := badgerStore.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		return serviceImpl.writeBlockData(ctx, tx, block)
	})
	require.NoError(t, err)

	// Assert - 验证高度索引已创建
	heightKey := []byte(fmt.Sprintf("indices:height:%d", block.Header.Height))
	indexData, err := badgerStore.Get(ctx, heightKey)
	assert.NoError(t, err)
	assert.NotNil(t, indexData)
	assert.GreaterOrEqual(t, len(indexData), 32+1+8, "索引值应包含 hash+path+size")

	// Assert - 验证区块文件已写入 blocks/
	pathLen := int(indexData[32])
	require.Greater(t, pathLen, 0)
	require.GreaterOrEqual(t, len(indexData), 33+pathLen+8)
	filePath := string(indexData[33 : 33+pathLen])
	blockBytes, err := fileStore.Load(ctx, filePath)
	assert.NoError(t, err)
	assert.NotEmpty(t, blockBytes)

	// 验证哈希索引已创建
	blockHash := indexData[:32]
	hashKey := []byte(fmt.Sprintf("indices:hash:%x", blockHash))
	heightData, err := badgerStore.Get(ctx, hashKey)
	assert.NoError(t, err)
	assert.NotNil(t, heightData)
	assert.Equal(t, block.Header.Height, binary.BigEndian.Uint64(heightData))
}

// TestWriteBlockData_WithNilBlockHashClient_ReturnsError 测试 nil blockHashClient 时返回错误
func TestWriteBlockData_WithNilBlockHashClient_ReturnsError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	badgerStore := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	txHashClient := testutil.NewTestTransactionHashClient()
	service := NewService(badgerStore, fileStore, nil, txHashClient, nil)

	block := testutil.CreateBlock(100, testutil.RandomHash())

	// 类型断言为 *Service 以访问内部方法
	serviceImpl := service.(*Service)

	// Act
	err := badgerStore.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		return serviceImpl.writeBlockData(ctx, tx, block)
	})

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "blockHashClient 未初始化")
}

// TestWriteBlockData_WithNilFileStore_ReturnsError 测试 nil fileStore 时返回错误（blocks/ 为区块原始数据落点）
func TestWriteBlockData_WithNilFileStore_ReturnsError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	badgerStore := testutil.NewTestBadgerStore()
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()
	service := NewService(badgerStore, nil, blockHashClient, txHashClient, nil)

	block := testutil.CreateBlock(100, testutil.RandomHash())

	// 类型断言为 *Service 以访问内部方法
	serviceImpl := service.(*Service)

	// Act
	err := badgerStore.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		return serviceImpl.writeBlockData(ctx, tx, block)
	})

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fileStore 未初始化")
}

// TestWriteBlockData_WithDifferentHeights_WritesCorrectFiles 测试不同高度写入正确的 blocks/ 文件
func TestWriteBlockData_WithDifferentHeights_WritesCorrectFiles(t *testing.T) {
	// Arrange
	ctx := context.Background()
	badgerStore := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()
	service := NewService(badgerStore, fileStore, blockHashClient, txHashClient, nil)

	testCases := []struct {
		height uint64
	}{
		{1},
		{1000},
		{1001},
		{2000},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("height_%d", tc.height), func(t *testing.T) {
			block := testutil.CreateBlock(tc.height, testutil.RandomHash())

			// 类型断言为 *Service 以访问内部方法
			serviceImpl := service.(*Service)

			err := badgerStore.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
				return serviceImpl.writeBlockData(ctx, tx, block)
			})
			require.NoError(t, err)

			// 验证高度索引 & 文件存在
			heightKey := []byte(fmt.Sprintf("indices:height:%d", tc.height))
			indexData, err := badgerStore.Get(ctx, heightKey)
			require.NoError(t, err)
			require.GreaterOrEqual(t, len(indexData), 32+1+8)
			pathLen := int(indexData[32])
			require.Greater(t, pathLen, 0)
			require.GreaterOrEqual(t, len(indexData), 33+pathLen+8)
			filePath := string(indexData[33 : 33+pathLen])
			b, err := fileStore.Load(ctx, filePath)
			require.NoError(t, err)
			assert.NotEmpty(t, b)
		})
	}
}
