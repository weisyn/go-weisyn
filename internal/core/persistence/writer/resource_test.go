// Package writer 提供资源索引更新逻辑的测试
//
// 🧪 **测试文件**
//
// 本文件测试资源索引更新的核心功能，遵循测试规范：
// - docs/system/standards/principles/testing-standards.md
//
// 🎯 **测试覆盖**：
// - 资源索引写入
package writer

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/persistence/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	pb_resource "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// ==================== 资源索引写入测试 ====================

// TestWriteResourceIndices_WithResourceOutput_WritesIndex 测试写入资源索引
func TestWriteResourceIndices_WithResourceOutput_WritesIndex(t *testing.T) {
	// Arrange
	ctx := context.Background()
	badgerStore := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()
	service := NewService(badgerStore, fileStore, blockHashClient, txHashClient, nil)

	contentHash := testutil.RandomHash()
	block := testutil.CreateBlock(100, testutil.RandomHash())
	
	// 创建包含资源输出的交易
	resourceOutput := &transaction.TxOutput{
		OutputContent: &transaction.TxOutput_Resource{
			Resource: &transaction.ResourceOutput{
				Resource: &pb_resource.Resource{
					ContentHash: contentHash,
				},
			},
		},
	}
	
	block.Body.Transactions = []*transaction.Transaction{
		{
			Outputs: []*transaction.TxOutput{resourceOutput},
		},
	}

	// 类型断言为 *Service 以访问内部方法
	serviceImpl := service.(*Service)

	// Act
	err := badgerStore.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		return serviceImpl.writeResourceIndices(ctx, tx, block)
	})
	require.NoError(t, err)

	// Assert - 验证资源代码索引已创建（indices:resource-code:{contentHash}）
	codeIndexKey := []byte(fmt.Sprintf("indices:resource-code:%x", contentHash))
	codeIndexData, err := badgerStore.Get(ctx, codeIndexKey)
	assert.NoError(t, err)
	assert.NotNil(t, codeIndexData)
}

// TestWriteResourceIndices_WithNilBlockHashClient_ReturnsError 测试 nil blockHashClient 时返回错误
func TestWriteResourceIndices_WithNilBlockHashClient_ReturnsError(t *testing.T) {
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
		return serviceImpl.writeResourceIndices(ctx, tx, block)
	})

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "blockHashClient 未初始化")
}

// TestWriteResourceIndices_WithEmptyTransactions_ReturnsNoError 测试空交易列表时返回无错误
func TestWriteResourceIndices_WithEmptyTransactions_ReturnsNoError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	badgerStore := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()
	service := NewService(badgerStore, fileStore, blockHashClient, txHashClient, nil)

	block := testutil.CreateBlock(100, testutil.RandomHash())
	block.Body.Transactions = []*transaction.Transaction{}

	// 类型断言为 *Service 以访问内部方法
	serviceImpl := service.(*Service)

	// Act
	err := badgerStore.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		return serviceImpl.writeResourceIndices(ctx, tx, block)
	})

	// Assert
	assert.NoError(t, err)
}

