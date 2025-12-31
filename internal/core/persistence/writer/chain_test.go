// Package writer 提供链状态更新逻辑的测试
//
// 🧪 **测试文件**
//
// 本文件测试链状态更新的核心功能，遵循测试规范：
// - docs/system/standards/principles/testing-standards.md
//
// 🎯 **测试覆盖**：
// - 链尖更新
// - 状态根更新
package writer

import (
	"context"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/persistence/testutil"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// ==================== 链状态更新测试 ====================

// TestWriteChainState_WithValidBlock_UpdatesTip 测试更新链尖
func TestWriteChainState_WithValidBlock_UpdatesTip(t *testing.T) {
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
	
	// Act
	err := badgerStore.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		return serviceImpl.writeChainState(ctx, tx, block)
	})
	require.NoError(t, err)

	// Assert - 验证链尖已更新
	tipKey := []byte("state:chain:tip")
	tipData, err := badgerStore.Get(ctx, tipKey)
	assert.NoError(t, err)
	assert.NotNil(t, tipData)
	assert.Equal(t, 40, len(tipData), "链尖数据应该为40字节（8字节高度 + 32字节哈希）")
	
	height := binary.BigEndian.Uint64(tipData[:8])
	assert.Equal(t, block.Header.Height, height)
}

// TestWriteChainState_WithStateRoot_UpdatesStateRoot 测试更新状态根
func TestWriteChainState_WithStateRoot_UpdatesStateRoot(t *testing.T) {
	// Arrange
	ctx := context.Background()
	badgerStore := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	blockHashClient := testutil.NewTestBlockHashClient()
	txHashClient := testutil.NewTestTransactionHashClient()
	service := NewService(badgerStore, fileStore, blockHashClient, txHashClient, nil)

	stateRoot := testutil.RandomHash()
	block := testutil.CreateBlock(100, testutil.RandomHash())
	block.Header.StateRoot = stateRoot

	// 类型断言为 *Service 以访问内部方法
	serviceImpl := service.(*Service)
	
	// Act
	err := badgerStore.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		return serviceImpl.writeChainState(ctx, tx, block)
	})
	require.NoError(t, err)

	// Assert - 验证状态根已更新
	stateRootKey := []byte("state:chain:root")
	rootData, err := badgerStore.Get(ctx, stateRootKey)
	assert.NoError(t, err)
	assert.NotNil(t, rootData)
	assert.Equal(t, stateRoot, rootData)
}

// TestWriteChainState_WithNilBlockHashClient_ReturnsError 测试 nil blockHashClient 时返回错误
func TestWriteChainState_WithNilBlockHashClient_ReturnsError(t *testing.T) {
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
		return serviceImpl.writeChainState(ctx, tx, block)
	})

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "blockHashClient 未初始化")
}

