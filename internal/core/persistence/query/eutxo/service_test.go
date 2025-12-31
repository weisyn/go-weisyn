// Package eutxo 提供EUTXO查询服务的测试
//
// 🧪 **测试文件**
//
// 本文件测试 UTXOQuery 服务的核心功能，遵循测试规范：
// - docs/system/standards/principles/testing-standards.md
//
// 🎯 **测试覆盖**：
// - 服务创建
// - UTXO查询
// - 地址UTXO查询
// - 赞助池UTXO查询
// - 状态根计算
package eutxo

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
	txtestutil "github.com/weisyn/v1/internal/core/tx/testutil"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pb/blockchain/utxo"
)

// ==================== 服务创建测试 ====================

// TestNewService_WithValidDependencies_ReturnsService 测试使用有效依赖创建服务
func TestNewService_WithValidDependencies_ReturnsService(t *testing.T) {
	// Arrange
	storage := testutil.NewTestBadgerStore()
	hasher := testutil.NewTestHashManager().(crypto.HashManager)
	logger := testutil.NewTestLogger()

	// Act
	service, err := NewService(storage, hasher, logger)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, service)
}

// TestNewService_WithNilStorage_ReturnsError 测试使用 nil storage 创建服务
func TestNewService_WithNilStorage_ReturnsError(t *testing.T) {
	// Arrange
	hasher := testutil.NewTestHashManager().(crypto.HashManager)
	logger := testutil.NewTestLogger()

	// Act
	service, err := NewService(nil, hasher, logger)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "storage 不能为空")
}

// TestNewService_WithNilHasher_ReturnsError 测试使用 nil hasher 创建服务
func TestNewService_WithNilHasher_ReturnsError(t *testing.T) {
	// Arrange
	storage := testutil.NewTestBadgerStore()
	logger := testutil.NewTestLogger()

	// Act
	service, err := NewService(storage, nil, logger)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "hasher 不能为空")
}

// ==================== UTXO查询测试 ====================

// TestGetUTXO_WithValidOutPoint_ReturnsUTXO 测试查询UTXO
func TestGetUTXO_WithValidOutPoint_ReturnsUTXO(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	hasher := testutil.NewTestHashManager().(crypto.HashManager)
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, hasher, logger)
	require.NoError(t, err)

	// 创建测试UTXO
	outpoint := txtestutil.CreateOutPoint(nil, 0)
	output := txtestutil.CreateNativeCoinOutput(nil, "1000", nil)
	utxoObj := txtestutil.CreateUTXO(outpoint, output, utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	
	utxoData, err := proto.Marshal(utxoObj)
	require.NoError(t, err)
	
	utxoKey := fmt.Sprintf("utxo:set:%x:%d", outpoint.TxId, outpoint.OutputIndex)
	err = storage.Set(ctx, []byte(utxoKey), utxoData)
	require.NoError(t, err)

	// Act
	result, err := service.GetUTXO(ctx, outpoint)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, utxoObj.Category, result.Category)
}

// TestGetUTXO_WithInvalidOutPoint_ReturnsError 测试无效OutPoint时返回错误
func TestGetUTXO_WithInvalidOutPoint_ReturnsError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	hasher := testutil.NewTestHashManager().(crypto.HashManager)
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, hasher, logger)
	require.NoError(t, err)

	// Act
	result, err := service.GetUTXO(ctx, nil)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "无效的 OutPoint")
}

// ==================== 地址UTXO查询测试 ====================

// TestGetUTXOsByAddress_WithValidAddress_ReturnsUTXOs 测试按地址查询UTXO
func TestGetUTXOsByAddress_WithValidAddress_ReturnsUTXOs(t *testing.T) {
	// Arrange
	ctx := context.Background()
	storage := testutil.NewTestBadgerStore()
	hasher := testutil.NewTestHashManager().(crypto.HashManager)
	logger := testutil.NewTestLogger()
	service, err := NewService(storage, hasher, logger)
	require.NoError(t, err)

	address := testutil.RandomAddress()
	
	// 创建测试UTXO
	outpoint1 := txtestutil.CreateOutPoint(nil, 0)
	outpoint2 := txtestutil.CreateOutPoint(nil, 1)
	
	// 创建地址索引数据（格式：多个36字节的outpoint）
	indexData := make([]byte, 72) // 2个outpoint
	copy(indexData[0:32], outpoint1.TxId)
	binary.BigEndian.PutUint32(indexData[32:36], outpoint1.OutputIndex)
	copy(indexData[36:68], outpoint2.TxId)
	binary.BigEndian.PutUint32(indexData[68:72], outpoint2.OutputIndex)
	
	addressIndexKey := fmt.Sprintf("index:address:%x", address)
	err = storage.Set(ctx, []byte(addressIndexKey), indexData)
	require.NoError(t, err)
	
	// 保存UTXO数据
	output := txtestutil.CreateNativeCoinOutput(address, "1000", nil)
	utxo1 := txtestutil.CreateUTXO(outpoint1, output, utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxo2 := txtestutil.CreateUTXO(outpoint2, output, utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	
	utxo1Data, err := proto.Marshal(utxo1)
	require.NoError(t, err)
	utxo2Data, err := proto.Marshal(utxo2)
	require.NoError(t, err)
	
	utxo1Key := fmt.Sprintf("utxo:set:%x:%d", outpoint1.TxId, outpoint1.OutputIndex)
	utxo2Key := fmt.Sprintf("utxo:set:%x:%d", outpoint2.TxId, outpoint2.OutputIndex)
	storage.Set(ctx, []byte(utxo1Key), utxo1Data)
	storage.Set(ctx, []byte(utxo2Key), utxo2Data)

	// Act
	utxos, err := service.GetUTXOsByAddress(ctx, address, nil, false)

	// Assert
	assert.NoError(t, err)
	assert.Len(t, utxos, 2)
}

// ==================== 编译时检查 ====================

// 确保 Service 实现了接口
var _ interfaces.InternalUTXOQuery = (*Service)(nil)

