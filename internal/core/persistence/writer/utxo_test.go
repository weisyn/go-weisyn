// Package writer 提供 UTXO 变更写入逻辑的测试
//
// 🧪 **测试文件**
//
// 本文件测试 UTXO 变更写入的核心功能，遵循测试规范：
// - docs/system/standards/principles/testing-standards.md
//
// 🎯 **测试覆盖**：
// - UTXO创建
// - UTXO删除
// - 地址索引更新
package writer

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/persistence/testutil"
	txtestutil "github.com/weisyn/v1/internal/core/tx/testutil"
	"github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// ==================== UTXO创建测试 ====================

// TestCreateUTXOInTransaction_WithValidUTXO_CreatesSuccessfully 测试创建UTXO
func TestCreateUTXOInTransaction_WithValidUTXO_CreatesSuccessfully(t *testing.T) {
	// Arrange
	ctx := context.Background()
	badgerStore := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()
	service := NewService(badgerStore, fileStore, blockHashClient, txHashClient, nil)

	outpoint := txtestutil.CreateOutPoint(nil, 0)
	output := txtestutil.CreateNativeCoinOutput(nil, "1000", nil)
	utxoObj := txtestutil.CreateUTXO(outpoint, output, utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	// 类型断言为 *Service 以访问内部方法
	serviceImpl := service.(*Service)

	// Act
	err := badgerStore.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		return serviceImpl.createUTXOInTransaction(ctx, tx, utxoObj)
	})
	require.NoError(t, err)

	// Assert - 验证UTXO已创建
	utxoKey := fmt.Sprintf("utxo:set:%x:%d", outpoint.TxId, outpoint.OutputIndex)
	utxoData, err := badgerStore.Get(ctx, []byte(utxoKey))
	assert.NoError(t, err)
	assert.NotNil(t, utxoData)
}

// TestCreateUTXOInTransaction_WithNilUTXO_ReturnsError 测试 nil UTXO 时返回错误
func TestCreateUTXOInTransaction_WithNilUTXO_ReturnsError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	badgerStore := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()
	service := NewService(badgerStore, fileStore, blockHashClient, txHashClient, nil)

	// 类型断言为 *Service 以访问内部方法
	serviceImpl := service.(*Service)

	// Act
	err := badgerStore.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		return serviceImpl.createUTXOInTransaction(ctx, tx, nil)
	})

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无效的 UTXO 对象")
}

// ==================== UTXO删除测试 ====================

// TestDeleteUTXOInTransaction_WithValidOutPoint_DeletesSuccessfully 测试删除UTXO
func TestDeleteUTXOInTransaction_WithValidOutPoint_DeletesSuccessfully(t *testing.T) {
	// Arrange
	ctx := context.Background()
	badgerStore := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()
	service := NewService(badgerStore, fileStore, blockHashClient, txHashClient, nil)

	outpoint := txtestutil.CreateOutPoint(nil, 0)
	output := txtestutil.CreateNativeCoinOutput(nil, "1000", nil)
	utxoObj := txtestutil.CreateUTXO(outpoint, output, utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	// 类型断言为 *Service 以访问内部方法
	serviceImpl := service.(*Service)

	// 先创建UTXO
	err := badgerStore.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		return serviceImpl.createUTXOInTransaction(ctx, tx, utxoObj)
	})
	require.NoError(t, err)

	// Act - 删除UTXO
	err = badgerStore.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		return serviceImpl.deleteUTXOInTransaction(ctx, tx, outpoint)
	})
	require.NoError(t, err)

	// Assert - 验证UTXO已删除
	utxoKey := fmt.Sprintf("utxo:set:%x:%d", outpoint.TxId, outpoint.OutputIndex)
	utxoData, err := badgerStore.Get(ctx, []byte(utxoKey))
	assert.NoError(t, err)
	assert.Nil(t, utxoData, "UTXO应该已被删除")
}

// TestDeleteUTXOInTransaction_WithNilOutPoint_ReturnsError 测试 nil OutPoint 时返回错误
func TestDeleteUTXOInTransaction_WithNilOutPoint_ReturnsError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	badgerStore := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()
	service := NewService(badgerStore, fileStore, blockHashClient, txHashClient, nil)

	// 类型断言为 *Service 以访问内部方法
	serviceImpl := service.(*Service)

	// Act
	err := badgerStore.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		return serviceImpl.deleteUTXOInTransaction(ctx, tx, nil)
	})

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无效的 OutPoint")
}

