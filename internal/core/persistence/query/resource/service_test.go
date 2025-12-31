// Package resource 提供资源查询服务的测试
//
// 🧪 **测试文件**
//
// 本文件测试 ResourceQuery 服务的核心功能，遵循测试规范：
// - docs/system/standards/principles/testing-standards.md
//
// 🎯 **测试覆盖**：
// - 服务创建
// - 资源查询
// - 资源交易信息查询
// - 文件路径构建
// - 资源哈希列表
package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/persistence/query/interfaces"
	"github.com/weisyn/v1/internal/core/persistence/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

type mockTxQuery struct {
	blockHash   []byte
	blockHeight uint64
}

func (m *mockTxQuery) GetTransaction(ctx context.Context, txHash []byte) (blockHash []byte, txIndex uint32, tx *transaction.Transaction, err error) {
	return m.blockHash, 0, &transaction.Transaction{}, nil
}
func (m *mockTxQuery) GetTxBlockHeight(ctx context.Context, txHash []byte) (uint64, error) {
	return m.blockHeight, nil
}
func (m *mockTxQuery) GetBlockTimestamp(ctx context.Context, height uint64) (int64, error) {
	return 0, nil
}
func (m *mockTxQuery) GetAccountNonce(ctx context.Context, address []byte) (uint64, error) {
	return 0, nil
}
func (m *mockTxQuery) GetTransactionsByBlock(ctx context.Context, blockHash []byte) ([]*transaction.Transaction, error) {
	return []*transaction.Transaction{}, nil
}

// ==================== 服务创建测试 ====================

// TestNewService_WithValidDependencies_ReturnsService 测试使用有效依赖创建服务
func TestNewService_WithValidDependencies_ReturnsService(t *testing.T) {
	// Arrange
	badgerStore := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	txQuery := &testutil.MockInternalTxQuery{}
	logger := testutil.NewTestLogger()

	// Act
	service, err := NewService(badgerStore, fileStore, txQuery, logger)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, service)
}

// TestNewService_WithNilBadgerStore_ReturnsError 测试使用 nil badgerStore 创建服务
func TestNewService_WithNilBadgerStore_ReturnsError(t *testing.T) {
	// Arrange
	fileStore := testutil.NewTestFileStore()
	txQuery := &testutil.MockInternalTxQuery{}
	logger := testutil.NewTestLogger()

	// Act
	service, err := NewService(nil, fileStore, txQuery, logger)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "badgerStore 不能为空")
}

// ==================== 资源交易信息查询测试 ====================

// TestGetResourceTransaction_WithValidContentHash_ReturnsTransactionInfo 测试获取资源交易信息
func TestGetResourceTransaction_WithValidContentHash_ReturnsTransactionInfo(t *testing.T) {
	// Arrange
	ctx := context.Background()
	badgerStore := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	logger := testutil.NewTestLogger()
	contentHash := testutil.RandomHash()
	txHash := testutil.RandomHash()
	blockHash := testutil.RandomHash()
	blockHeight := uint64(100)

	txQuery := &mockTxQuery{blockHash: blockHash, blockHeight: blockHeight}
	service, err := NewService(badgerStore, fileStore, txQuery, logger)
	require.NoError(t, err)

	// Phase 4：资源交易信息通过 code→instance 索引获取（indices:resource-code:{contentHash} = JSON数组）
	instanceList := []string{fmt.Sprintf("%x:%d", txHash, 0)}
	indexData, err := json.Marshal(instanceList)
	require.NoError(t, err)
	txIndexKey := []byte(fmt.Sprintf("indices:resource-code:%x", contentHash))
	err = badgerStore.Set(ctx, txIndexKey, indexData)
	require.NoError(t, err)

	// Act
	resultTxHash, resultBlockHash, resultHeight, err := service.GetResourceTransaction(ctx, contentHash)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, txHash, resultTxHash)
	assert.Equal(t, blockHash, resultBlockHash)
	assert.Equal(t, blockHeight, resultHeight)
}

// ==================== 文件路径构建测试 ====================

// TestBuildFilePath_WithValidHash_ReturnsPath 测试构建文件路径
func TestBuildFilePath_WithValidHash_ReturnsPath(t *testing.T) {
	// Arrange
	badgerStore := testutil.NewTestBadgerStore()
	fileStore := testutil.NewTestFileStore()
	txQuery := &testutil.MockInternalTxQuery{}
	logger := testutil.NewTestLogger()
	service, err := NewService(badgerStore, fileStore, txQuery, logger)
	require.NoError(t, err)

	contentHash := testutil.RandomHash()

	// Act
	path := service.BuildFilePath(contentHash)

	// Assert
	assert.NotEmpty(t, path)
	assert.Contains(t, path, fmt.Sprintf("%x", contentHash))
}

// ==================== 编译时检查 ====================

// 确保 Service 实现了接口
var _ interfaces.InternalResourceQuery = (*Service)(nil)
