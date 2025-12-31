// Package tx 提供交易查询服务的测试
//
// 🧪 **测试文件**
//
// 本文件测试 TxQuery 服务的核心功能，遵循测试规范：
// - docs/system/standards/principles/testing-standards.md
//
// 🎯 **测试覆盖**：
// - 服务创建
// - 交易查询
// - 交易区块高度查询
// - 账户nonce查询
package tx

import (
	"context"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/persistence/query/interfaces"
	"github.com/weisyn/v1/internal/core/persistence/testutil"
	core "github.com/weisyn/v1/pb/blockchain/block"
	"google.golang.org/protobuf/proto"
)

// ==================== 服务创建测试 ====================

// TestNewService_WithValidDependencies_ReturnsService 测试使用有效依赖创建服务
func TestNewService_WithValidDependencies_ReturnsService(t *testing.T) {
	// Arrange
	storage := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	txHashClient := testutil.NewTestTransactionHashClient()
	logger := testutil.NewTestLogger()

	// Act
	service, err := NewService(storage, fileStore, txHashClient, nil, logger)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, service)
}

// TestNewService_WithNilStorage_ReturnsError 测试使用 nil storage 创建服务
func TestNewService_WithNilStorage_ReturnsError(t *testing.T) {
	// Arrange
	fileStore := testutil.NewTestFileStore()
	txHashClient := testutil.NewTestTransactionHashClient()
	logger := testutil.NewTestLogger()

	// Act
	service, err := NewService(nil, fileStore, txHashClient, nil, logger)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "storage 不能为空")
}

// ==================== 交易区块高度查询测试 ====================

// TestGetTxBlockHeight_WithValidTxHash_ReturnsHeight 测试获取交易区块高度
func TestGetTxBlockHeight_WithValidTxHash_ReturnsHeight(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	txHashClient := testutil.NewTestTransactionHashClient()
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, fileStore, txHashClient, nil, logger)
	require.NoError(t, err)

	txHash := testutil.RandomHash()
	blockHeight := uint64(100)
	blockHash := testutil.RandomHash()
	txIndex := uint32(0)

	// 创建交易位置数据（格式：blockHeight(8) + blockHash(32) + txIndex(4)）
	locationData := make([]byte, 44)
	binary.BigEndian.PutUint64(locationData[0:8], blockHeight)
	copy(locationData[8:40], blockHash)
	binary.BigEndian.PutUint32(locationData[40:44], txIndex)

	txKey := []byte(fmt.Sprintf("indices:tx:%x", txHash))
	err = storage.Set(ctx, txKey, locationData)
	require.NoError(t, err)

	// Act
	height, err := service.GetTxBlockHeight(ctx, txHash)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, blockHeight, height)
}

// ==================== 账户nonce查询测试 ====================

// TestGetAccountNonce_WithValidAddress_ReturnsNonce 测试获取账户nonce
func TestGetAccountNonce_WithValidAddress_ReturnsNonce(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	txHashClient := testutil.NewTestTransactionHashClient()
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, fileStore, txHashClient, nil, logger)
	require.NoError(t, err)

	address := testutil.RandomAddress()
	nonce := uint64(42)

	nonceBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(nonceBytes, nonce)

	nonceKey := []byte(fmt.Sprintf("indices:nonce:%x", address))
	err = storage.Set(ctx, nonceKey, nonceBytes)
	require.NoError(t, err)

	// Act
	result, err := service.GetAccountNonce(ctx, address)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, nonce, result)
}

// TestGetAccountNonce_WithMissingNonce_ReturnsZero 测试缺失nonce时返回0
func TestGetAccountNonce_WithMissingNonce_ReturnsZero(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	txHashClient := testutil.NewTestTransactionHashClient()
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, fileStore, txHashClient, nil, logger)
	require.NoError(t, err)

	address := testutil.RandomAddress()

	// Act
	result, err := service.GetAccountNonce(ctx, address)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, uint64(0), result)
}

// ==================== 区块时间戳查询测试 ====================

// TestGetBlockTimestamp_NewIndexFormat_LoadsFromFileStore
// ✅ 彻底迭代验收：indices:height 必须为新格式，且从 FileStore 读取区块返回 Header.Timestamp。
func TestGetBlockTimestamp_NewIndexFormat_LoadsFromFileStore(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	txHashClient := testutil.NewTestTransactionHashClient()
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, fileStore, txHashClient, nil, logger)
	require.NoError(t, err)

	height := uint64(7)
	ts := int64(1700000000)
	filePath := "blocks/0000000000/0000000007.bin"

	block := &core.Block{
		Header: &core.BlockHeader{
			Height:    height,
			Timestamp: uint64(ts),
		},
		Body: &core.BlockBody{
			Transactions: nil,
		},
	}
	blockBytes, err := proto.Marshal(block)
	require.NoError(t, err)

	// 写入文件系统（MockFileStore）
	require.NoError(t, fileStore.Save(ctx, filePath, blockBytes))

	// 写入高度索引（新格式：hash32 + pathLen + path + fileSize）
	indexValue := make([]byte, 32+1+len(filePath)+8)
	copy(indexValue[0:32], testutil.RandomHash())
	indexValue[32] = byte(len(filePath))
	copy(indexValue[33:33+len(filePath)], []byte(filePath))
	binary.BigEndian.PutUint64(indexValue[33+len(filePath):33+len(filePath)+8], uint64(len(blockBytes)))

	heightKey := []byte(fmt.Sprintf("indices:height:%d", height))
	require.NoError(t, storage.Set(ctx, heightKey, indexValue))

	// Act
	got, err := service.GetBlockTimestamp(ctx, height)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, ts, got)
}

// ==================== 编译时检查 ====================

// 确保 Service 实现了接口
var _ interfaces.InternalTxQuery = (*Service)(nil)
