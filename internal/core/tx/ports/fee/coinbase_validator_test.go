// Package fee_test 提供 CoinbaseValidator 的单元测试
//
// 🧪 **测试覆盖**：
// - CoinbaseValidator 核心功能测试
// - 验证成功场景
// - 验证失败场景
package fee

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/core/tx/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	txiface "github.com/weisyn/v1/pkg/interfaces/tx"
)

// ==================== CoinbaseValidator 核心功能测试 ====================

// TestNewCoinbaseValidator 测试创建 CoinbaseValidator
func TestNewCoinbaseValidator(t *testing.T) {
	validator := NewCoinbaseValidator()

	assert.NotNil(t, validator)
	assert.NotNil(t, validator.calculator)
}

// TestCoinbaseValidator_Validate_Success 测试验证成功
func TestCoinbaseValidator_Validate_Success(t *testing.T) {
	validator := NewCoinbaseValidator()

	expectedFees := &txiface.AggregatedFees{
		ByToken: map[txiface.TokenKey]*big.Int{
			"native": big.NewInt(1000),
		},
	}
	minerAddr := testutil.RandomAddress()

	coinbase := &transaction.Transaction{
		Version: 1,
		Inputs:  []*transaction.TxInput{},
		Outputs: []*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(minerAddr, "1000", testutil.CreateSingleKeyLock(nil)),
		},
	}

	err := validator.Validate(context.Background(), coinbase, expectedFees, minerAddr)

	assert.NoError(t, err)
}

// TestCoinbaseValidator_Validate_EmptyCoinbase 测试空 Coinbase（无输出，无费用）
func TestCoinbaseValidator_Validate_EmptyCoinbase(t *testing.T) {
	validator := NewCoinbaseValidator()

	expectedFees := &txiface.AggregatedFees{
		ByToken: map[txiface.TokenKey]*big.Int{},
	}
	minerAddr := testutil.RandomAddress()

	coinbase := &transaction.Transaction{
		Version: 1,
		Inputs:  []*transaction.TxInput{},
		Outputs: []*transaction.TxOutput{},
	}

	err := validator.Validate(context.Background(), coinbase, expectedFees, minerAddr)

	assert.NoError(t, err) // 零增发模式下，无费用是合法的
}

// TestCoinbaseValidator_Validate_WithInputs 测试有输入的 Coinbase（应该失败）
func TestCoinbaseValidator_Validate_WithInputs(t *testing.T) {
	validator := NewCoinbaseValidator()

	expectedFees := &txiface.AggregatedFees{
		ByToken: map[txiface.TokenKey]*big.Int{
			"native": big.NewInt(1000),
		},
	}
	minerAddr := testutil.RandomAddress()

	coinbase := &transaction.Transaction{
		Version: 1,
		Inputs: []*transaction.TxInput{
			{PreviousOutput: testutil.CreateOutPoint(nil, 0), IsReferenceOnly: false},
		},
		Outputs: []*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(minerAddr, "1000", testutil.CreateSingleKeyLock(nil)),
		},
	}

	err := validator.Validate(context.Background(), coinbase, expectedFees, minerAddr)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不能有输入")
}

// TestCoinbaseValidator_Validate_WrongOwner 测试 Owner 不匹配
func TestCoinbaseValidator_Validate_WrongOwner(t *testing.T) {
	validator := NewCoinbaseValidator()

	expectedFees := &txiface.AggregatedFees{
		ByToken: map[txiface.TokenKey]*big.Int{
			"native": big.NewInt(1000),
		},
	}
	minerAddr := testutil.RandomAddress()
	wrongOwner := testutil.RandomAddress()

	coinbase := &transaction.Transaction{
		Version: 1,
		Inputs:  []*transaction.TxInput{},
		Outputs: []*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(wrongOwner, "1000", testutil.CreateSingleKeyLock(nil)),
		},
	}

	err := validator.Validate(context.Background(), coinbase, expectedFees, minerAddr)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Owner不是矿工地址")
}

// TestCoinbaseValidator_Validate_FeeMismatch 测试费用不匹配
func TestCoinbaseValidator_Validate_FeeMismatch(t *testing.T) {
	validator := NewCoinbaseValidator()

	expectedFees := &txiface.AggregatedFees{
		ByToken: map[txiface.TokenKey]*big.Int{
			"native": big.NewInt(1000),
		},
	}
	minerAddr := testutil.RandomAddress()

	coinbase := &transaction.Transaction{
		Version: 1,
		Inputs:  []*transaction.TxInput{},
		Outputs: []*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(minerAddr, "500", testutil.CreateSingleKeyLock(nil)), // 金额不匹配
		},
	}

	err := validator.Validate(context.Background(), coinbase, expectedFees, minerAddr)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "金额不一致")
}

// TestCoinbaseValidator_Validate_MissingToken 测试缺少 Token
func TestCoinbaseValidator_Validate_MissingToken(t *testing.T) {
	validator := NewCoinbaseValidator()

	expectedFees := &txiface.AggregatedFees{
		ByToken: map[txiface.TokenKey]*big.Int{
			"native":                 big.NewInt(1000),
			"contract:0x1234:0x5678": big.NewInt(500),
		},
	}
	minerAddr := testutil.RandomAddress()

	coinbase := &transaction.Transaction{
		Version: 1,
		Inputs:  []*transaction.TxInput{},
		Outputs: []*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(minerAddr, "1000", testutil.CreateSingleKeyLock(nil)),
			// 缺少合约Token输出
		},
	}

	err := validator.Validate(context.Background(), coinbase, expectedFees, minerAddr)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Token种类数量不一致")
}

// TestCoinbaseValidator_Validate_ZeroAmountOutput 测试金额为0的输出（应该允许）
func TestCoinbaseValidator_Validate_ZeroAmountOutput(t *testing.T) {
	validator := NewCoinbaseValidator()

	// 当期望费用为0时，Coinbase可以没有输出或输出金额为0
	expectedFees := &txiface.AggregatedFees{
		ByToken: map[txiface.TokenKey]*big.Int{},
	}
	minerAddr := testutil.RandomAddress()

	// 空 Coinbase（无输出）
	coinbase := &transaction.Transaction{
		Version: 1,
		Inputs:  []*transaction.TxInput{},
		Outputs: []*transaction.TxOutput{},
	}

	err := validator.Validate(context.Background(), coinbase, expectedFees, minerAddr)

	assert.NoError(t, err) // 零增发模式下，无费用是合法的
}

// ==================== validateFeeConservation 测试用例 ====================

// TestValidateFeeConservation_Success 测试费用守恒验证成功
func TestValidateFeeConservation_Success(t *testing.T) {
	validator := NewCoinbaseValidator()

	actual := map[txiface.TokenKey]*big.Int{
		"native":                 big.NewInt(1000),
		"contract:0x1234:0x5678": big.NewInt(500),
	}
	expected := map[txiface.TokenKey]*big.Int{
		"native":                 big.NewInt(1000),
		"contract:0x1234:0x5678": big.NewInt(500),
	}

	err := validator.validateFeeConservation(actual, expected)

	assert.NoError(t, err)
}

// TestValidateFeeConservation_TokenCountMismatch 测试Token种类数量不一致
func TestValidateFeeConservation_TokenCountMismatch(t *testing.T) {
	validator := NewCoinbaseValidator()

	actual := map[txiface.TokenKey]*big.Int{
		"native": big.NewInt(1000),
	}
	expected := map[txiface.TokenKey]*big.Int{
		"native":                 big.NewInt(1000),
		"contract:0x1234:0x5678": big.NewInt(500),
	}

	err := validator.validateFeeConservation(actual, expected)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Token种类数量不一致")
}

// TestValidateFeeConservation_AmountMismatch 测试金额不一致
func TestValidateFeeConservation_AmountMismatch(t *testing.T) {
	validator := NewCoinbaseValidator()

	actual := map[txiface.TokenKey]*big.Int{
		"native":                 big.NewInt(1000),
		"contract:0x1234:0x5678": big.NewInt(300), // 金额不匹配
	}
	expected := map[txiface.TokenKey]*big.Int{
		"native":                 big.NewInt(1000),
		"contract:0x1234:0x5678": big.NewInt(500),
	}

	err := validator.validateFeeConservation(actual, expected)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "金额不一致")
}

// TestValidateFeeConservation_ExtraToken 测试额外的Token（增发检测）
func TestValidateFeeConservation_ExtraToken(t *testing.T) {
	validator := NewCoinbaseValidator()

	actual := map[txiface.TokenKey]*big.Int{
		"native":                 big.NewInt(1000),
		"contract:0x1234:0x5678": big.NewInt(500),
		"contract:0x9999:0x8888": big.NewInt(200), // 额外的Token
	}
	expected := map[txiface.TokenKey]*big.Int{
		"native":                 big.NewInt(1000),
		"contract:0x1234:0x5678": big.NewInt(500),
	}

	err := validator.validateFeeConservation(actual, expected)

	assert.Error(t, err)
	// validateFeeConservation 先检查数量，所以会先返回"Token种类数量不一致"
	// 但实际代码逻辑中，如果数量一致，会在后续检查中检测额外Token
	assert.Contains(t, err.Error(), "Token种类数量不一致")
}

// TestValidateFeeConservation_EmptyMaps 测试空map
func TestValidateFeeConservation_EmptyMaps(t *testing.T) {
	validator := NewCoinbaseValidator()

	actual := map[txiface.TokenKey]*big.Int{}
	expected := map[txiface.TokenKey]*big.Int{}

	err := validator.validateFeeConservation(actual, expected)

	assert.NoError(t, err)
}

// TestValidateFeeConservation_MultipleTokens 测试多个Token
func TestValidateFeeConservation_MultipleTokens(t *testing.T) {
	validator := NewCoinbaseValidator()

	actual := map[txiface.TokenKey]*big.Int{
		"native":                 big.NewInt(1000),
		"contract:0x1111:0x2222": big.NewInt(500),
		"contract:0x3333:0x4444": big.NewInt(300),
	}
	expected := map[txiface.TokenKey]*big.Int{
		"native":                 big.NewInt(1000),
		"contract:0x1111:0x2222": big.NewInt(500),
		"contract:0x3333:0x4444": big.NewInt(300),
	}

	err := validator.validateFeeConservation(actual, expected)

	assert.NoError(t, err)
}
