// Package block 提供区块查询服务的测试
//
// 🧪 **测试文件**
//
// 本文件测试 BlockQuery 服务的核心功能，遵循测试规范：
// - docs/system/standards/principles/testing-standards.md
//
// 🎯 **测试覆盖**：
// - 服务创建
// - 按高度查询区块
// - 按哈希查询区块
// - 区块头查询
// - 区块范围查询
// - 最高区块查询
package block

import (
	"context"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/weisyn/v1/internal/core/persistence/query/interfaces"
	"github.com/weisyn/v1/internal/core/persistence/testutil"
	core "github.com/weisyn/v1/pb/blockchain/block"
)

// ==================== 服务创建测试 ====================

// TestNewService_WithValidDependencies_ReturnsService 测试使用有效依赖创建服务
func TestNewService_WithValidDependencies_ReturnsService(t *testing.T) {
	// Arrange
	storage := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	logger := testutil.NewTestLogger()

	// Act
	service, err := NewService(storage, fileStore, nil, nil, logger)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, service)
}

// TestNewService_WithNilStorage_ReturnsError 测试使用 nil storage 创建服务
func TestNewService_WithNilStorage_ReturnsError(t *testing.T) {
	// Arrange
	logger := testutil.NewTestLogger()
	fileStore := testutil.NewTestFileStore()

	// Act
	service, err := NewService(nil, fileStore, nil, nil, logger)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "storage 不能为空")
}

// ==================== 按高度查询区块测试 ====================

// TestGetBlockByHeight_WithFileBlockData_ReturnsBlock 测试从 blocks/ 文件读取区块数据
func TestGetBlockByHeight_WithFileBlockData_ReturnsBlock(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, fileStore, nil, nil, logger)
	require.NoError(t, err)

	height := uint64(100)

	// 创建测试区块
	block := &core.Block{
		Header: &core.BlockHeader{
			Height: height,
		},
	}
	blockData, err := proto.Marshal(block)
	require.NoError(t, err)

	// 写入区块文件（与 writer/block.go 对齐）
	seg := (height / 1000) * 1000
	blockFilePath := fmt.Sprintf("blocks/%010d/%010d.bin", seg, height)
	err = fileStore.MakeDir(ctx, fmt.Sprintf("blocks/%010d", seg), true)
	require.NoError(t, err)
	err = fileStore.Save(ctx, blockFilePath, blockData)
	require.NoError(t, err)

	// 写入高度索引：blockHash(32) + pathLen(1) + path + size(8)
	blockHash := testutil.RandomHash()
	pathBytes := []byte(blockFilePath)
	indexVal := make([]byte, 32+1+len(pathBytes)+8)
	copy(indexVal[0:32], blockHash)
	indexVal[32] = byte(len(pathBytes))
	copy(indexVal[33:33+len(pathBytes)], pathBytes)
	binary.BigEndian.PutUint64(indexVal[33+len(pathBytes):41+len(pathBytes)], uint64(len(blockData)))
	err = storage.Set(ctx, []byte(fmt.Sprintf("indices:height:%d", height)), indexVal)
	require.NoError(t, err)

	// Act
	result, err := service.GetBlockByHeight(ctx, height)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, height, result.Header.Height)
}

// TestGetBlockByHeight_WithInvalidIndex_ReturnsError 测试无效索引时返回错误
func TestGetBlockByHeight_WithInvalidIndex_ReturnsError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, fileStore, nil, nil, logger)
	require.NoError(t, err)

	height := uint64(200)

	// Act
	result, err := service.GetBlockByHeight(ctx, height)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "区块高度索引")
}

// ==================== 按哈希查询区块测试 ====================

// TestGetBlockByHash_WithValidHash_ReturnsBlock 测试按哈希查询区块
func TestGetBlockByHash_WithValidHash_ReturnsBlock(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, fileStore, nil, nil, logger)
	require.NoError(t, err)

	height := uint64(100)
	blockHash := testutil.RandomHash()

	// 创建测试区块
	block := &core.Block{
		Header: &core.BlockHeader{
			Height: height,
		},
	}
	blockData, err := proto.Marshal(block)
	require.NoError(t, err)

	// 写入区块文件 + 高度索引
	seg := (height / 1000) * 1000
	blockFilePath := fmt.Sprintf("blocks/%010d/%010d.bin", seg, height)
	err = fileStore.MakeDir(ctx, fmt.Sprintf("blocks/%010d", seg), true)
	require.NoError(t, err)
	err = fileStore.Save(ctx, blockFilePath, blockData)
	require.NoError(t, err)

	pathBytes := []byte(blockFilePath)
	indexVal := make([]byte, 32+1+len(pathBytes)+8)
	copy(indexVal[0:32], blockHash)
	indexVal[32] = byte(len(pathBytes))
	copy(indexVal[33:33+len(pathBytes)], pathBytes)
	binary.BigEndian.PutUint64(indexVal[33+len(pathBytes):41+len(pathBytes)], uint64(len(blockData)))
	err = storage.Set(ctx, []byte(fmt.Sprintf("indices:height:%d", height)), indexVal)
	require.NoError(t, err)

	// 设置哈希索引
	hashKey := []byte(fmt.Sprintf("indices:hash:%x", blockHash))
	heightBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(heightBytes, height)
	err = storage.Set(ctx, hashKey, heightBytes)
	require.NoError(t, err)

	// Act
	result, err := service.GetBlockByHash(ctx, blockHash)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, height, result.Header.Height)
}

func TestGetBlockByHash_WithMissingHashIndex_AutoRepairsUsingHeightIndex(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, fileStore, nil, nil, logger)
	require.NoError(t, err)

	height := uint64(926)
	blockHash := testutil.RandomHash()

	// tip 必须存在（repair 依赖 state:chain:tip）
	tipData := make([]byte, 40)
	binary.BigEndian.PutUint64(tipData[:8], height)
	copy(tipData[8:40], blockHash)
	err = storage.Set(ctx, []byte("state:chain:tip"), tipData)
	require.NoError(t, err)

	// 创建测试区块并写入 blocks/ 文件
	block := &core.Block{
		Header: &core.BlockHeader{
			Height: height,
		},
	}
	blockData, err := proto.Marshal(block)
	require.NoError(t, err)

	seg := (height / 1000) * 1000
	blockFilePath := fmt.Sprintf("blocks/%010d/%010d.bin", seg, height)
	err = fileStore.MakeDir(ctx, fmt.Sprintf("blocks/%010d", seg), true)
	require.NoError(t, err)
	err = fileStore.Save(ctx, blockFilePath, blockData)
	require.NoError(t, err)

	// 写入高度索引（但不写哈希索引，模拟历史版本/迁移遗留）
	pathBytes := []byte(blockFilePath)
	indexVal := make([]byte, 32+1+len(pathBytes)+8)
	copy(indexVal[0:32], blockHash)
	indexVal[32] = byte(len(pathBytes))
	copy(indexVal[33:33+len(pathBytes)], pathBytes)
	binary.BigEndian.PutUint64(indexVal[33+len(pathBytes):41+len(pathBytes)], uint64(len(blockData)))
	err = storage.Set(ctx, []byte(fmt.Sprintf("indices:height:%d", height)), indexVal)
	require.NoError(t, err)

	// Act：第一次按 hash 查，应触发自动修复并返回区块
	got, err := service.GetBlockByHash(ctx, blockHash)

	// Assert：返回区块正确
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, height, got.Header.Height)

	// Assert：indices:hash 已被补写为 8 字节高度
	hashKey := []byte(fmt.Sprintf("indices:hash:%x", blockHash))
	hb, err := storage.Get(ctx, hashKey)
	require.NoError(t, err)
	require.Len(t, hb, 8)
	assert.Equal(t, height, binary.BigEndian.Uint64(hb))
}

// ==================== 区块头查询测试 ====================

// TestGetBlockHeader_WithValidHash_ReturnsHeader 测试获取区块头
func TestGetBlockHeader_WithValidHash_ReturnsHeader(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, fileStore, nil, nil, logger)
	require.NoError(t, err)

	height := uint64(100)
	blockHash := testutil.RandomHash()

	// 创建测试区块
	block := &core.Block{
		Header: &core.BlockHeader{
			Height: height,
		},
	}
	blockData, err := proto.Marshal(block)
	require.NoError(t, err)

	// 写入区块文件 + 高度索引
	seg := (height / 1000) * 1000
	blockFilePath := fmt.Sprintf("blocks/%010d/%010d.bin", seg, height)
	err = fileStore.MakeDir(ctx, fmt.Sprintf("blocks/%010d", seg), true)
	require.NoError(t, err)
	err = fileStore.Save(ctx, blockFilePath, blockData)
	require.NoError(t, err)

	pathBytes := []byte(blockFilePath)
	indexVal := make([]byte, 32+1+len(pathBytes)+8)
	copy(indexVal[0:32], blockHash)
	indexVal[32] = byte(len(pathBytes))
	copy(indexVal[33:33+len(pathBytes)], pathBytes)
	binary.BigEndian.PutUint64(indexVal[33+len(pathBytes):41+len(pathBytes)], uint64(len(blockData)))
	err = storage.Set(ctx, []byte(fmt.Sprintf("indices:height:%d", height)), indexVal)
	require.NoError(t, err)

	// 设置哈希索引
	hashKey := []byte(fmt.Sprintf("indices:hash:%x", blockHash))
	heightBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(heightBytes, height)
	err = storage.Set(ctx, hashKey, heightBytes)
	require.NoError(t, err)

	// Act
	header, err := service.GetBlockHeader(ctx, blockHash)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, header)
	assert.Equal(t, height, header.Height)
}

// ==================== 最高区块查询测试 ====================

// TestGetHighestBlock_WithValidTipData_ReturnsHighestBlock 测试获取最高区块
func TestGetHighestBlock_WithValidTipData_ReturnsHighestBlock(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, fileStore, nil, nil, logger)
	require.NoError(t, err)

	height := uint64(1000)
	blockHash := testutil.RandomHash()

	// 设置链尖数据（格式：height(8字节) + blockHash(32字节)）
	tipData := make([]byte, 40)
	binary.BigEndian.PutUint64(tipData[:8], height)
	copy(tipData[8:40], blockHash)

	tipKey := []byte("state:chain:tip")
	err = storage.Set(ctx, tipKey, tipData)
	require.NoError(t, err)

	// Act
	resultHeight, resultHash, err := service.GetHighestBlock(ctx)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, height, resultHeight)
	assert.Equal(t, blockHash, resultHash)
}

// ==================== 编译时检查 ====================

// 确保 Service 实现了接口
var _ interfaces.InternalBlockQuery = (*Service)(nil)
