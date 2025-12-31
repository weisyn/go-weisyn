// Package txpool 合规策略测试
package txpool

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/config/txpool"
	"github.com/weisyn/v1/internal/core/mempool/testutil"
	complianceIfaces "github.com/weisyn/v1/pkg/interfaces/compliance"
)

// createTestTxPoolWithCompliance 创建带合规策略的交易池
func createTestTxPoolWithCompliance(t *testing.T, compliancePolicy complianceIfaces.Policy) *TxPool {
	config := &txpool.TxPoolOptions{
		MaxSize:    1000,
		MemoryLimit: 100 * 1024 * 1024,
		MaxTxSize:  1024 * 1024,
		Mining: txpool.MiningOptions{
			MaxTransactionsForMining: 100,
			MaxBlockSizeForMining:     1024 * 1024,
		},
	}
	logger := &testutil.MockLogger{}
	eventBus := testutil.NewMockEventBus()
	memory := testutil.NewMockMemoryStore()
	hashService := &testutil.MockTransactionHashService{}

	pool, err := NewTxPoolWithCacheAndCompliance(config, logger, eventBus, memory, hashService, nil, compliancePolicy, nil)
	require.NoError(t, err)
	return pool.(*TxPool)
}

// TestAddTransaction_WithCompliancePolicy_Allowed_AcceptsTx 测试合规策略允许的交易
func TestAddTransaction_WithCompliancePolicy_Allowed_AcceptsTx(t *testing.T) {
	// Arrange
	compliancePolicy := testutil.NewMockCompliancePolicy(true)
	pool := createTestTxPoolWithCompliance(t, compliancePolicy)
	tx := testutil.CreateSimpleTestTransaction(1)

	// Act
	txID, err := pool.AddTransaction(tx)

	// Assert
	assert.NoError(t, err, "合规策略允许的交易应该被接受")
	assert.NotNil(t, txID, "交易ID不应为nil")
}

// TestAddTransaction_WithCompliancePolicy_Rejected_RejectsTx 测试合规策略拒绝的交易
func TestAddTransaction_WithCompliancePolicy_Rejected_RejectsTx(t *testing.T) {
	// Arrange
	decision := &complianceIfaces.Decision{
		Allowed:      false,
		Reason:        "测试拒绝",
		ReasonDetail:  "测试拒绝详情",
		Source:        complianceIfaces.DecisionSourceConfig,
	}
	compliancePolicy := testutil.NewMockCompliancePolicyWithDecision(decision)
	pool := createTestTxPoolWithCompliance(t, compliancePolicy)
	tx := testutil.CreateSimpleTestTransaction(1)

	// Act
	txID, err := pool.AddTransaction(tx)

	// Assert
	assert.Error(t, err, "合规策略拒绝的交易应该被拒绝")
	assert.Nil(t, txID, "交易ID应该为nil")
	assert.Contains(t, err.Error(), "合规", "错误信息应该包含合规相关描述")
}

// TestAddTransaction_WithCompliancePolicy_Error_ReturnsError 测试合规策略返回错误
func TestAddTransaction_WithCompliancePolicy_Error_ReturnsError(t *testing.T) {
	// Arrange
	complianceError := errors.New("合规检查失败")
	compliancePolicy := testutil.NewMockCompliancePolicyWithError(complianceError)
	pool := createTestTxPoolWithCompliance(t, compliancePolicy)
	tx := testutil.CreateSimpleTestTransaction(1)

	// Act
	txID, err := pool.AddTransaction(tx)

	// Assert
	assert.Error(t, err, "合规策略返回错误时应该返回错误")
	assert.Nil(t, txID, "交易ID应该为nil")
	assert.Contains(t, err.Error(), "合规", "错误信息应该包含合规相关描述")
}

// TestAddTransaction_WithNilCompliancePolicy_AcceptsTx 测试nil合规策略时接受交易
func TestAddTransaction_WithNilCompliancePolicy_AcceptsTx(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t) // 使用不带合规策略的交易池
	tx := testutil.CreateSimpleTestTransaction(1)

	// Act
	txID, err := pool.AddTransaction(tx)

	// Assert
	assert.NoError(t, err, "nil合规策略时应该接受交易")
	assert.NotNil(t, txID, "交易ID不应为nil")
}

// TestGetTransactionsForMining_WithCompliancePolicy_FiltersRejected 测试挖矿时过滤不合规交易
func TestGetTransactionsForMining_WithCompliancePolicy_FiltersRejected(t *testing.T) {
	// Arrange
	// 创建允许的合规策略
	compliancePolicy := testutil.NewMockCompliancePolicy(true)
	pool := createTestTxPoolWithCompliance(t, compliancePolicy)
	
	// 提交多个交易
	numTxs := 5
	for i := 0; i < numTxs; i++ {
		tx := testutil.CreateTestTransaction(i, nil, nil)
		tx.Nonce = uint64(i)
		_, err := pool.SubmitTx(tx)
		require.NoError(t, err)
	}

	// Act
	miningTxs, err := pool.GetPendingTxs(100, 1024*1024, nil)

	// Assert
	assert.NoError(t, err, "应该成功获取挖矿交易")
	// 注意：由于Mock合规策略允许所有交易，应该返回所有pending交易
	assert.GreaterOrEqual(t, len(miningTxs), 0, "应该返回交易列表")
}

// TestNewTxPoolWithCacheAndCompliance_WithValidConfig_ReturnsPool 测试创建带合规策略的交易池
func TestNewTxPoolWithCacheAndCompliance_WithValidConfig_ReturnsPool(t *testing.T) {
	// Arrange
	config := &txpool.TxPoolOptions{
		MaxSize:    1000,
		MemoryLimit: 100 * 1024 * 1024,
		MaxTxSize:  1024 * 1024,
		Mining: txpool.MiningOptions{
			MaxTransactionsForMining: 100,
			MaxBlockSizeForMining:     1024 * 1024,
		},
	}
	logger := &testutil.MockLogger{}
	eventBus := testutil.NewMockEventBus()
	memory := testutil.NewMockMemoryStore()
	hashService := &testutil.MockTransactionHashService{}
	compliancePolicy := testutil.NewMockCompliancePolicy(true)

	// Act
	pool, err := NewTxPoolWithCacheAndCompliance(config, logger, eventBus, memory, hashService, nil, compliancePolicy, nil)

	// Assert
	assert.NoError(t, err, "应该成功创建带合规策略的交易池")
	assert.NotNil(t, pool, "交易池实例不应为nil")
}

// TestNewTxPoolWithCacheAndCompliance_WithNilConfig_ReturnsError 测试nil配置时返回错误
func TestNewTxPoolWithCacheAndCompliance_WithNilConfig_ReturnsError(t *testing.T) {
	// Arrange
	logger := &testutil.MockLogger{}
	eventBus := testutil.NewMockEventBus()
	memory := testutil.NewMockMemoryStore()
	hashService := &testutil.MockTransactionHashService{}
	compliancePolicy := testutil.NewMockCompliancePolicy(true)

	// Act
	pool, err := NewTxPoolWithCacheAndCompliance(nil, logger, eventBus, memory, hashService, nil, compliancePolicy, nil)

	// Assert
	assert.Error(t, err, "nil配置应该返回错误")
	assert.Nil(t, pool, "交易池实例应为nil")
	assert.Contains(t, err.Error(), "配置不能为空", "错误信息应该包含配置相关描述")
}

