// Package incentive_test 提供 CoinbasePlugin 的单元测试
//
// 🧪 **测试规范遵循**：
// - 每个源文件对应一个测试文件
// - 遵循测试规范：docs/system/standards/principles/testing-standards.md
package incentive

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/core/tx/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	txiface "github.com/weisyn/v1/pkg/interfaces/tx"
)

// ==================== CoinbasePlugin 测试 ====================

// TestNewCoinbasePlugin 测试创建 CoinbasePlugin
func TestNewCoinbasePlugin(t *testing.T) {
	feeManager := &MockFeeManager{}
	plugin := NewCoinbasePlugin(feeManager)

	assert.NotNil(t, plugin)
	assert.Equal(t, feeManager, plugin.feeManager)
	assert.NotNil(t, plugin.coinbaseValidator)
}

// TestCoinbasePlugin_Name 测试插件名称
func TestCoinbasePlugin_Name(t *testing.T) {
	feeManager := &MockFeeManager{}
	plugin := NewCoinbasePlugin(feeManager)

	assert.Equal(t, "CoinbaseValidator", plugin.Name())
}

// TestCoinbasePlugin_Verify_NonCoinbase 测试非 Coinbase 交易（跳过）
func TestCoinbasePlugin_Verify_NonCoinbase(t *testing.T) {
	feeManager := &MockFeeManager{}
	plugin := NewCoinbasePlugin(feeManager)

	// 创建非 Coinbase 交易（有输入）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil)),
		},
	)

	env := &MockVerifierEnvironment{}
	err := plugin.Verify(context.Background(), tx, env)

	assert.NoError(t, err) // 非 Coinbase 交易应该跳过
}

// TestCoinbasePlugin_Verify_Success 测试 Coinbase 验证成功
func TestCoinbasePlugin_Verify_Success(t *testing.T) {
	feeManager := &MockFeeManager{}
	plugin := NewCoinbasePlugin(feeManager)

	minerAddr := testutil.RandomAddress()
	expectedFees := &txiface.AggregatedFees{
		ByToken: map[txiface.TokenKey]*big.Int{
			"native": big.NewInt(1000),
		},
	}

	// 创建 Coinbase 交易（无输入）
	tx := testutil.CreateTransaction(
		nil,
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(minerAddr, "1000", testutil.CreateSingleKeyLock(nil)),
		},
	)

	env := &MockVerifierEnvironment{
		minerAddress: minerAddr,
		expectedFees: expectedFees,
	}

	// Mock ValidateCoinbase 返回成功
	feeManager.validateCoinbaseFunc = func(ctx context.Context, coinbase *transaction.Transaction, expectedFees *txiface.AggregatedFees, minerAddr []byte) error {
		return nil
	}

	err := plugin.Verify(context.Background(), tx, env)

	assert.NoError(t, err)
}

// TestCoinbasePlugin_Verify_InvalidEnvironment 测试无效的验证环境
func TestCoinbasePlugin_Verify_InvalidEnvironment(t *testing.T) {
	feeManager := &MockFeeManager{}
	plugin := NewCoinbasePlugin(feeManager)

	// 创建 Coinbase 交易
	tx := testutil.CreateTransaction(nil, nil)

	// 传入无效的环境类型
	env := "invalid environment"

	err := plugin.Verify(context.Background(), tx, env)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "环境类型错误")
}

// TestCoinbasePlugin_Verify_NilExpectedFees 测试期望费用为 nil
func TestCoinbasePlugin_Verify_NilExpectedFees(t *testing.T) {
	feeManager := &MockFeeManager{}
	plugin := NewCoinbasePlugin(feeManager)

	minerAddr := testutil.RandomAddress()
	tx := testutil.CreateTransaction(nil, nil)

	env := &MockVerifierEnvironment{
		minerAddress: minerAddr,
		expectedFees: nil, // nil 期望费用
	}

	err := plugin.Verify(context.Background(), tx, env)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "期望费用为nil")
}

// TestCoinbasePlugin_Verify_InvalidMinerAddress 测试无效的矿工地址长度
func TestCoinbasePlugin_Verify_InvalidMinerAddress(t *testing.T) {
	feeManager := &MockFeeManager{}
	plugin := NewCoinbasePlugin(feeManager)

	expectedFees := &txiface.AggregatedFees{
		ByToken: map[txiface.TokenKey]*big.Int{
			"native": big.NewInt(1000),
		},
	}

	tx := testutil.CreateTransaction(nil, nil)

	env := &MockVerifierEnvironment{
		minerAddress: []byte{1, 2, 3}, // 长度不是 20 字节
		expectedFees: expectedFees,
	}

	err := plugin.Verify(context.Background(), tx, env)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "矿工地址长度必须为20字节")
}

// TestCoinbasePlugin_Verify_ValidationFailure 测试验证失败
func TestCoinbasePlugin_Verify_ValidationFailure(t *testing.T) {
	feeManager := &MockFeeManager{}
	plugin := NewCoinbasePlugin(feeManager)

	minerAddr := testutil.RandomAddress()
	expectedFees := &txiface.AggregatedFees{
		ByToken: map[txiface.TokenKey]*big.Int{
			"native": big.NewInt(1000),
		},
	}

	tx := testutil.CreateTransaction(nil, nil)

	env := &MockVerifierEnvironment{
		minerAddress: minerAddr,
		expectedFees: expectedFees,
	}

	// Mock ValidateCoinbase 返回失败
	feeManager.validateCoinbaseFunc = func(ctx context.Context, coinbase *transaction.Transaction, expectedFees *txiface.AggregatedFees, minerAddr []byte) error {
		return assert.AnError
	}

	err := plugin.Verify(context.Background(), tx, env)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "验证失败")
}

// TestCoinbasePlugin_Verify_EmptyCoinbase 测试空 Coinbase（无输出）
func TestCoinbasePlugin_Verify_EmptyCoinbase(t *testing.T) {
	feeManager := &MockFeeManager{}
	plugin := NewCoinbasePlugin(feeManager)

	minerAddr := testutil.RandomAddress()
	expectedFees := &txiface.AggregatedFees{
		ByToken: map[txiface.TokenKey]*big.Int{},
	}

	tx := testutil.CreateTransaction(nil, nil) // 无输出

	env := &MockVerifierEnvironment{
		minerAddress: minerAddr,
		expectedFees: expectedFees,
	}

	feeManager.validateCoinbaseFunc = func(ctx context.Context, coinbase *transaction.Transaction, expectedFees *txiface.AggregatedFees, minerAddr []byte) error {
		return nil
	}

	err := plugin.Verify(context.Background(), tx, env)

	assert.NoError(t, err) // 空 Coinbase 应该通过（零增发模式下无费用是合法的）
}

// TestCoinbasePlugin_Verify_MultipleTokens 测试多代币 Coinbase
func TestCoinbasePlugin_Verify_MultipleTokens(t *testing.T) {
	feeManager := &MockFeeManager{}
	plugin := NewCoinbasePlugin(feeManager)

	minerAddr := testutil.RandomAddress()
	expectedFees := &txiface.AggregatedFees{
		ByToken: map[txiface.TokenKey]*big.Int{
			"native": big.NewInt(1000),
		},
	}

	tx := testutil.CreateTransaction(
		nil,
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(minerAddr, "1000", testutil.CreateSingleKeyLock(nil)),
		},
	)

	env := &MockVerifierEnvironment{
		minerAddress: minerAddr,
		expectedFees: expectedFees,
	}

	feeManager.validateCoinbaseFunc = func(ctx context.Context, coinbase *transaction.Transaction, expectedFees *txiface.AggregatedFees, minerAddr []byte) error {
		return nil
	}

	err := plugin.Verify(context.Background(), tx, env)

	assert.NoError(t, err)
}

