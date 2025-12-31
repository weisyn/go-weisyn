// Package aggregator 提供 QueryService 聚合器的测试
//
// 🧪 **测试文件**
//
// 本文件测试 QueryService 聚合器的核心功能，遵循测试规范：
// - docs/system/standards/principles/testing-standards.md
//
// 🎯 **测试覆盖**：
// - 服务创建
// - 方法委托
// - 错误处理
package aggregator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/persistence/testutil"
)

// ==================== 服务创建测试 ====================

// TestNewService_WithValidDependencies_ReturnsService 测试使用有效依赖创建服务
func TestNewService_WithValidDependencies_ReturnsService(t *testing.T) {
	// Arrange
	chainQuery := &testutil.MockInternalChainQuery{}
	blockQuery := &testutil.MockInternalBlockQuery{}
	txQuery := &testutil.MockInternalTxQuery{}
	utxoQuery := &testutil.MockInternalUTXOQuery{}
	resourceQuery := &testutil.MockInternalResourceQuery{}
	accountQuery := &testutil.MockInternalAccountQuery{}
	pricingQuery := &testutil.MockInternalPricingQuery{}
	logger := testutil.NewTestLogger()

	// Act
	service, err := NewService(
		chainQuery,
		blockQuery,
		txQuery,
		utxoQuery,
		resourceQuery,
		accountQuery,
		pricingQuery,
		logger,
	)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, service)
}

// TestNewService_WithNilChainQuery_ReturnsError 测试使用 nil chainQuery 创建服务
func TestNewService_WithNilChainQuery_ReturnsError(t *testing.T) {
	// Arrange
	blockQuery := &testutil.MockInternalBlockQuery{}
	txQuery := &testutil.MockInternalTxQuery{}
	utxoQuery := &testutil.MockInternalUTXOQuery{}
	resourceQuery := &testutil.MockInternalResourceQuery{}
	accountQuery := &testutil.MockInternalAccountQuery{}
	pricingQuery := &testutil.MockInternalPricingQuery{}
	logger := testutil.NewTestLogger()

	// Act
	service, err := NewService(
		nil,
		blockQuery,
		txQuery,
		utxoQuery,
		resourceQuery,
		accountQuery,
		pricingQuery,
		logger,
	)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "chainQuery 不能为空")
}

// TestNewService_WithNilBlockQuery_ReturnsError 测试使用 nil blockQuery 创建服务
func TestNewService_WithNilBlockQuery_ReturnsError(t *testing.T) {
	// Arrange
	chainQuery := &testutil.MockInternalChainQuery{}
	txQuery := &testutil.MockInternalTxQuery{}
	utxoQuery := &testutil.MockInternalUTXOQuery{}
	resourceQuery := &testutil.MockInternalResourceQuery{}
	accountQuery := &testutil.MockInternalAccountQuery{}
	pricingQuery := &testutil.MockInternalPricingQuery{}
	logger := testutil.NewTestLogger()

	// Act
	service, err := NewService(
		chainQuery,
		nil,
		txQuery,
		utxoQuery,
		resourceQuery,
		accountQuery,
		pricingQuery,
		logger,
	)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "blockQuery 不能为空")
}

// ==================== 方法委托测试 ====================

// TestGetChainInfo_DelegatesToChainQuery 测试 GetChainInfo 委托给 ChainQuery
func TestGetChainInfo_DelegatesToChainQuery(t *testing.T) {
	// Arrange
	ctx := context.Background()
	chainQuery := &testutil.MockInternalChainQuery{}
	blockQuery := &testutil.MockInternalBlockQuery{}
	txQuery := &testutil.MockInternalTxQuery{}
	utxoQuery := &testutil.MockInternalUTXOQuery{}
	resourceQuery := &testutil.MockInternalResourceQuery{}
	accountQuery := &testutil.MockInternalAccountQuery{}
	pricingQuery := &testutil.MockInternalPricingQuery{}
	logger := testutil.NewTestLogger()

	service, err := NewService(
		chainQuery,
		blockQuery,
		txQuery,
		utxoQuery,
		resourceQuery,
		accountQuery,
		pricingQuery,
		logger,
	)
	require.NoError(t, err)

	// Act
	chainInfo, err := service.GetChainInfo(ctx)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, chainInfo)
}

// TestGetBlockByHeight_DelegatesToBlockQuery 测试 GetBlockByHeight 委托给 BlockQuery
func TestGetBlockByHeight_DelegatesToBlockQuery(t *testing.T) {
	// Arrange
	ctx := context.Background()
	chainQuery := &testutil.MockInternalChainQuery{}
	blockQuery := &testutil.MockInternalBlockQuery{}
	txQuery := &testutil.MockInternalTxQuery{}
	utxoQuery := &testutil.MockInternalUTXOQuery{}
	resourceQuery := &testutil.MockInternalResourceQuery{}
	accountQuery := &testutil.MockInternalAccountQuery{}
	pricingQuery := &testutil.MockInternalPricingQuery{}
	logger := testutil.NewTestLogger()

	service, err := NewService(
		chainQuery,
		blockQuery,
		txQuery,
		utxoQuery,
		resourceQuery,
		accountQuery,
		pricingQuery,
		logger,
	)
	require.NoError(t, err)

	// Act
	block, err := service.GetBlockByHeight(ctx, 1)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, block)
}

// TestGetUTXO_DelegatesToUTXOQuery 测试 GetUTXO 委托给 UTXOQuery
func TestGetUTXO_DelegatesToUTXOQuery(t *testing.T) {
	// Arrange
	ctx := context.Background()
	chainQuery := &testutil.MockInternalChainQuery{}
	blockQuery := &testutil.MockInternalBlockQuery{}
	txQuery := &testutil.MockInternalTxQuery{}
	utxoQuery := &testutil.MockInternalUTXOQuery{}
	resourceQuery := &testutil.MockInternalResourceQuery{}
	accountQuery := &testutil.MockInternalAccountQuery{}
	pricingQuery := &testutil.MockInternalPricingQuery{}
	logger := testutil.NewTestLogger()

	service, err := NewService(
		chainQuery,
		blockQuery,
		txQuery,
		utxoQuery,
		resourceQuery,
		accountQuery,
		pricingQuery,
		logger,
	)
	require.NoError(t, err)

	outpoint := testutil.CreateOutPoint(nil, 0)

	// Act
	utxo, err := service.GetUTXO(ctx, outpoint)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, utxo)
}

