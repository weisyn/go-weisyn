// Package conservation_test 提供 MinFeePlugin 的单元测试
//
// 🧪 **测试规范遵循**：
// - 每个源文件对应一个测试文件
// - 遵循测试规范：docs/system/standards/principles/testing-standards.md
package conservation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/core/tx/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxopb "github.com/weisyn/v1/pb/blockchain/utxo"
)

// ==================== MinFeePlugin 测试 ====================

// TestNewMinFeePlugin 测试创建 MinFeePlugin
func TestNewMinFeePlugin(t *testing.T) {
	plugin := NewMinFeePlugin()

	assert.NotNil(t, plugin)
}

// TestMinFeePlugin_Name 测试插件名称
func TestMinFeePlugin_Name(t *testing.T) {
	plugin := NewMinFeePlugin()

	assert.Equal(t, "min_fee", plugin.Name())
}

// TestMinFeePlugin_Check_NoMinimumFee 测试没有设置最低费用
func TestMinFeePlugin_Check_NoMinimumFee(t *testing.T) {
	plugin := NewMinFeePlugin()

	tx := testutil.CreateTransaction(nil, nil)
	// 不设置 minimum_fee

	err := plugin.Check(context.Background(), nil, nil, tx)

	assert.NoError(t, err) // 应该直接通过
}

// TestMinFeePlugin_Check_Success_NativeToken 测试原生代币费用验证成功
func TestMinFeePlugin_Check_Success_NativeToken(t *testing.T) {
	plugin := NewMinFeePlugin()

	// 创建输入 UTXO（1000 原生币）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	// 创建交易（输入 1000，输出 900，费用 100，最低费用 50）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint1,
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "900", testutil.CreateSingleKeyLock(nil)),
		},
	)

	minAmount := "50"
	tx.FeeMechanism = &transaction.Transaction_MinimumFee{
		MinimumFee: &transaction.MinimumFee{
			MinimumAmount: minAmount,
			FeeToken: &transaction.TokenReference{
				TokenType: &transaction.TokenReference_NativeToken{
					NativeToken: true,
				},
			},
		},
	}

	inputs := []*utxopb.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.NoError(t, err) // 实际费用 100 >= 最低费用 50
}

// TestMinFeePlugin_Check_InsufficientFee_NativeToken 测试原生代币费用不足
func TestMinFeePlugin_Check_InsufficientFee_NativeToken(t *testing.T) {
	plugin := NewMinFeePlugin()

	// 创建输入 UTXO（1000 原生币）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	// 创建交易（输入 1000，输出 950，费用 50，最低费用 100）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint1,
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "950", testutil.CreateSingleKeyLock(nil)),
		},
	)

	minAmount := "100"
	tx.FeeMechanism = &transaction.Transaction_MinimumFee{
		MinimumFee: &transaction.MinimumFee{
			MinimumAmount: minAmount,
			FeeToken: &transaction.TokenReference{
				TokenType: &transaction.TokenReference_NativeToken{
					NativeToken: true,
				},
			},
		},
	}

	inputs := []*utxopb.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient fee")
}

// TestMinFeePlugin_Check_InvalidMinimumAmount 测试无效的最低费用金额
func TestMinFeePlugin_Check_InvalidMinimumAmount(t *testing.T) {
	plugin := NewMinFeePlugin()

	tx := testutil.CreateTransaction(nil, nil)
	tx.FeeMechanism = &transaction.Transaction_MinimumFee{
		MinimumFee: &transaction.MinimumFee{
			MinimumAmount: "invalid", // 无效金额
			FeeToken: &transaction.TokenReference{
				TokenType: &transaction.TokenReference_NativeToken{
					NativeToken: true,
				},
			},
		},
	}

	err := plugin.Check(context.Background(), nil, nil, tx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid minimum_fee.minimum_amount")
}

// TestMinFeePlugin_Check_NegativeMinimumAmount 测试负数最低费用
func TestMinFeePlugin_Check_NegativeMinimumAmount(t *testing.T) {
	plugin := NewMinFeePlugin()

	tx := testutil.CreateTransaction(nil, nil)
	tx.FeeMechanism = &transaction.Transaction_MinimumFee{
		MinimumFee: &transaction.MinimumFee{
			MinimumAmount: "-100", // 负数
			FeeToken: &transaction.TokenReference{
				TokenType: &transaction.TokenReference_NativeToken{
					NativeToken: true,
				},
			},
		},
	}

	err := plugin.Check(context.Background(), nil, nil, tx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid minimum_fee.minimum_amount")
}

// TestMinFeePlugin_Check_ContractToken 测试合约代币费用验证
func TestMinFeePlugin_Check_ContractToken(t *testing.T) {
	plugin := NewMinFeePlugin()

	contractAddr := testutil.RandomAddress()
	// 创建输入 UTXO（1000 合约代币）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateContractTokenOutput(testutil.RandomAddress(), "1000", contractAddr, []byte("token"), nil)
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	// 创建交易（输入 1000，输出 900，费用 100，最低费用 50）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint1,
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateContractTokenOutput(testutil.RandomAddress(), "900", contractAddr, []byte("token"), nil),
		},
	)

	minAmount := "50"
	tx.FeeMechanism = &transaction.Transaction_MinimumFee{
		MinimumFee: &transaction.MinimumFee{
			MinimumAmount: minAmount,
			FeeToken: &transaction.TokenReference{
				TokenType: &transaction.TokenReference_ContractAddress{
					ContractAddress: contractAddr,
				},
			},
		},
	}

	inputs := []*utxopb.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.NoError(t, err) // 实际费用 100 >= 最低费用 50
}

// TestMinFeePlugin_Check_ReferenceOnlyInput 测试引用型输入不计入费用
func TestMinFeePlugin_Check_ReferenceOnlyInput(t *testing.T) {
	plugin := NewMinFeePlugin()

	// 创建输入 UTXO（1000 原生币）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	// 创建交易（引用型输入不计入费用）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint1,
				IsReferenceOnly: true, // 引用型输入
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "900", testutil.CreateSingleKeyLock(nil)),
		},
	)

	minAmount := "50"
	tx.FeeMechanism = &transaction.Transaction_MinimumFee{
		MinimumFee: &transaction.MinimumFee{
			MinimumAmount: minAmount,
			FeeToken: &transaction.TokenReference{
				TokenType: &transaction.TokenReference_NativeToken{
					NativeToken: true,
				},
			},
		},
	}

	inputs := []*utxopb.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	// 引用型输入不计入，实际费用 = 0 - 900 = -900（负数，应该失败）
	assert.Error(t, err)
}

// TestMinFeePlugin_Check_ExactMinimumFee 测试正好等于最低费用
func TestMinFeePlugin_Check_ExactMinimumFee(t *testing.T) {
	plugin := NewMinFeePlugin()

	// 创建输入 UTXO（1000 原生币）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	// 创建交易（输入 1000，输出 900，费用 100，最低费用 100）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint1,
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "900", testutil.CreateSingleKeyLock(nil)),
		},
	)

	minAmount := "100"
	tx.FeeMechanism = &transaction.Transaction_MinimumFee{
		MinimumFee: &transaction.MinimumFee{
			MinimumAmount: minAmount,
			FeeToken: &transaction.TokenReference{
				TokenType: &transaction.TokenReference_NativeToken{
					NativeToken: true,
				},
			},
		},
	}

	inputs := []*utxopb.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.NoError(t, err) // 实际费用 100 >= 最低费用 100
}

// TestMinFeePlugin_Check_UnknownFeeTokenType 测试未知的费用代币类型
func TestMinFeePlugin_Check_UnknownFeeTokenType(t *testing.T) {
	plugin := NewMinFeePlugin()

	tx := testutil.CreateTransaction(nil, nil)
	tx.FeeMechanism = &transaction.Transaction_MinimumFee{
		MinimumFee: &transaction.MinimumFee{
			MinimumAmount: "100",
			FeeToken: &transaction.TokenReference{
				// 不设置 TokenType，导致类型断言失败
			},
		},
	}

	err := plugin.Check(context.Background(), nil, nil, tx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown fee_token type")
}

// TestMinFeePlugin_Check_InvalidInputAmount 测试无效的输入金额
func TestMinFeePlugin_Check_InvalidInputAmount(t *testing.T) {
	plugin := NewMinFeePlugin()

	// 创建输入 UTXO（无效的原生币金额）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil))
	output1.GetAsset().AssetContent.(*transaction.AssetOutput_NativeCoin).NativeCoin.Amount = "invalid"
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint1,
				IsReferenceOnly: false,
			},
		},
		nil,
	)

	minAmount := "50"
	tx.FeeMechanism = &transaction.Transaction_MinimumFee{
		MinimumFee: &transaction.MinimumFee{
			MinimumAmount: minAmount,
			FeeToken: &transaction.TokenReference{
				TokenType: &transaction.TokenReference_NativeToken{
					NativeToken: true,
				},
			},
		},
	}

	inputs := []*utxopb.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid input native coin amount")
}

// TestMinFeePlugin_Check_NegativeFee 测试负费用
func TestMinFeePlugin_Check_NegativeFee(t *testing.T) {
	plugin := NewMinFeePlugin()

	// 创建输入 UTXO（500 原生币）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "500", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	// 创建交易（输入 500，输出 600，费用为负）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint1,
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "600", testutil.CreateSingleKeyLock(nil)),
		},
	)

	minAmount := "50"
	tx.FeeMechanism = &transaction.Transaction_MinimumFee{
		MinimumFee: &transaction.MinimumFee{
			MinimumAmount: minAmount,
			FeeToken: &transaction.TokenReference{
				TokenType: &transaction.TokenReference_NativeToken{
					NativeToken: true,
				},
			},
		},
	}

	inputs := []*utxopb.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "negative fee")
}

