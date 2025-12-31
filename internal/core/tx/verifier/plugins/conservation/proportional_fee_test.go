// Package conservation_test 提供 ProportionalFeePlugin 的单元测试
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

// ==================== ProportionalFeePlugin 测试 ====================

// TestNewProportionalFeePlugin 测试创建 ProportionalFeePlugin
func TestNewProportionalFeePlugin(t *testing.T) {
	plugin := NewProportionalFeePlugin()

	assert.NotNil(t, plugin)
}

// TestProportionalFeePlugin_Name 测试插件名称
func TestProportionalFeePlugin_Name(t *testing.T) {
	plugin := NewProportionalFeePlugin()

	assert.Equal(t, "proportional_fee", plugin.Name())
}

// TestProportionalFeePlugin_Check_NoProportionalFee 测试没有设置按比例收费
func TestProportionalFeePlugin_Check_NoProportionalFee(t *testing.T) {
	plugin := NewProportionalFeePlugin()

	tx := testutil.CreateTransaction(nil, nil)
	// 不设置 proportional_fee

	err := plugin.Check(context.Background(), nil, nil, tx)

	assert.NoError(t, err) // 应该直接通过
}

// TestProportionalFeePlugin_Check_Success_NativeToken 测试原生代币按比例收费验证成功
func TestProportionalFeePlugin_Check_Success_NativeToken(t *testing.T) {
	plugin := NewProportionalFeePlugin()

	// 创建输入 UTXO（100000 原生币）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "100000", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	// 创建交易（输入 100000，输出 90000，费用 10000，费率 0.1% = 10/10000，最低费用 = 90000 * 10 / 10000 = 900）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint1,
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "90000", testutil.CreateSingleKeyLock(nil)),
		},
	)

	rateBasisPoints := uint32(10) // 0.1% = 10/10000
	tx.FeeMechanism = &transaction.Transaction_ProportionalFee{
		ProportionalFee: &transaction.ProportionalFee{
			RateBasisPoints: rateBasisPoints,
			FeeToken: &transaction.TokenReference{
				TokenType: &transaction.TokenReference_NativeToken{
					NativeToken: true,
				},
			},
		},
	}

	inputs := []*utxopb.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.NoError(t, err) // 实际费用 10000 >= 最低费用 900
}

// TestProportionalFeePlugin_Check_InsufficientFee_NativeToken 测试原生代币按比例收费不足
func TestProportionalFeePlugin_Check_InsufficientFee_NativeToken(t *testing.T) {
	plugin := NewProportionalFeePlugin()

	// 创建输入 UTXO（100000 原生币）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "100000", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	// 创建交易（输入 100000，输出 99950，费用 50，费率 0.1% = 10/10000，最低费用 = 99950 * 10 / 10000 = 99.95，向下取整为 99）
	// 但实际费用 50 < 最低费用 99，应该失败
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint1,
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "99950", testutil.CreateSingleKeyLock(nil)),
		},
	)

	rateBasisPoints := uint32(10) // 0.1% = 10/10000
	tx.FeeMechanism = &transaction.Transaction_ProportionalFee{
		ProportionalFee: &transaction.ProportionalFee{
			RateBasisPoints: rateBasisPoints,
			FeeToken: &transaction.TokenReference{
				TokenType: &transaction.TokenReference_NativeToken{
					NativeToken: true,
				},
			},
		},
	}

	inputs := []*utxopb.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.Error(t, err) // 实际费用 50 < 最低费用 99
	assert.Contains(t, err.Error(), "insufficient proportional fee")
}

// TestProportionalFeePlugin_Check_ZeroRate 测试费率为 0
func TestProportionalFeePlugin_Check_ZeroRate(t *testing.T) {
	plugin := NewProportionalFeePlugin()

	tx := testutil.CreateTransaction(nil, nil)
	tx.FeeMechanism = &transaction.Transaction_ProportionalFee{
		ProportionalFee: &transaction.ProportionalFee{
			RateBasisPoints: 0, // 费率为 0
			FeeToken: &transaction.TokenReference{
				TokenType: &transaction.TokenReference_NativeToken{
					NativeToken: true,
				},
			},
		},
	}

	err := plugin.Check(context.Background(), nil, nil, tx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid proportional_fee.rate_basis_points")
}

// TestProportionalFeePlugin_Check_MaxFeeAmount 测试最大费用限制
func TestProportionalFeePlugin_Check_MaxFeeAmount(t *testing.T) {
	plugin := NewProportionalFeePlugin()

	// 创建输入 UTXO（1000000 原生币）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000000", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	// 创建交易（输入 1000000，输出 500000，费用 500000，费率 0.1%，最低费用 = 500000 * 10 / 10000 = 5000）
	// 设置最大费用 10000
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint1,
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "500000", testutil.CreateSingleKeyLock(nil)),
		},
	)

	maxFeeAmount := "10000"
	rateBasisPoints := uint32(10) // 0.1% = 10/10000
	tx.FeeMechanism = &transaction.Transaction_ProportionalFee{
		ProportionalFee: &transaction.ProportionalFee{
			RateBasisPoints: rateBasisPoints,
			MaxFeeAmount:    &maxFeeAmount,
			FeeToken: &transaction.TokenReference{
				TokenType: &transaction.TokenReference_NativeToken{
					NativeToken: true,
				},
			},
		},
	}

	inputs := []*utxopb.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.Error(t, err) // 实际费用 500000 > 最大费用 10000
	assert.Contains(t, err.Error(), "excessive proportional fee")
}

// TestProportionalFeePlugin_Check_MaxFeeAmountWithinLimit 测试最大费用限制内
func TestProportionalFeePlugin_Check_MaxFeeAmountWithinLimit(t *testing.T) {
	plugin := NewProportionalFeePlugin()

	// 创建输入 UTXO（100000 原生币）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "100000", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	// 创建交易（输入 100000，输出 90000，费用 10000，费率 0.1%，最低费用 = 90000 * 10 / 10000 = 900）
	// 设置最大费用 20000
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint1,
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "90000", testutil.CreateSingleKeyLock(nil)),
		},
	)

	maxFeeAmount := "20000"
	rateBasisPoints := uint32(10) // 0.1% = 10/10000
	tx.FeeMechanism = &transaction.Transaction_ProportionalFee{
		ProportionalFee: &transaction.ProportionalFee{
			RateBasisPoints: rateBasisPoints,
			MaxFeeAmount:    &maxFeeAmount,
			FeeToken: &transaction.TokenReference{
				TokenType: &transaction.TokenReference_NativeToken{
					NativeToken: true,
				},
			},
		},
	}

	inputs := []*utxopb.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.NoError(t, err) // 实际费用 10000 >= 最低费用 900 且 <= 最大费用 20000
}

// TestProportionalFeePlugin_Check_InvalidMaxFeeAmount 测试无效的最大费用金额
func TestProportionalFeePlugin_Check_InvalidMaxFeeAmount(t *testing.T) {
	plugin := NewProportionalFeePlugin()

	tx := testutil.CreateTransaction(nil, nil)
	maxFeeAmount := "invalid"
	rateBasisPoints := uint32(10)
	tx.FeeMechanism = &transaction.Transaction_ProportionalFee{
		ProportionalFee: &transaction.ProportionalFee{
			RateBasisPoints: rateBasisPoints,
			MaxFeeAmount:    &maxFeeAmount,
			FeeToken: &transaction.TokenReference{
				TokenType: &transaction.TokenReference_NativeToken{
					NativeToken: true,
				},
			},
		},
	}

	err := plugin.Check(context.Background(), nil, nil, tx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid proportional_fee.max_fee_amount")
}

// TestProportionalFeePlugin_Check_ContractToken 测试合约代币按比例收费
func TestProportionalFeePlugin_Check_ContractToken(t *testing.T) {
	plugin := NewProportionalFeePlugin()

	contractAddr := testutil.RandomAddress()
	// 创建输入 UTXO（100000 合约代币）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateContractTokenOutput(testutil.RandomAddress(), "100000", contractAddr, []byte("token"), nil)
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	// 创建交易（输入 100000，输出 90000，费用 10000，费率 0.1%，最低费用 = 90000 * 10 / 10000 = 900）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint1,
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateContractTokenOutput(testutil.RandomAddress(), "90000", contractAddr, []byte("token"), nil),
		},
	)

	rateBasisPoints := uint32(10) // 0.1% = 10/10000
	tx.FeeMechanism = &transaction.Transaction_ProportionalFee{
		ProportionalFee: &transaction.ProportionalFee{
			RateBasisPoints: rateBasisPoints,
			FeeToken: &transaction.TokenReference{
				TokenType: &transaction.TokenReference_ContractAddress{
					ContractAddress: contractAddr,
				},
			},
		},
	}

	inputs := []*utxopb.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.NoError(t, err) // 实际费用 10000 >= 最低费用 900
}

// TestProportionalFeePlugin_Check_ReferenceOnlyInput 测试引用型输入不计入费用
func TestProportionalFeePlugin_Check_ReferenceOnlyInput(t *testing.T) {
	plugin := NewProportionalFeePlugin()

	// 创建输入 UTXO（100000 原生币）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "100000", testutil.CreateSingleKeyLock(nil))
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
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "90000", testutil.CreateSingleKeyLock(nil)),
		},
	)

	rateBasisPoints := uint32(10) // 0.1% = 10/10000
	tx.FeeMechanism = &transaction.Transaction_ProportionalFee{
		ProportionalFee: &transaction.ProportionalFee{
			RateBasisPoints: rateBasisPoints,
			FeeToken: &transaction.TokenReference{
				TokenType: &transaction.TokenReference_NativeToken{
					NativeToken: true,
				},
			},
		},
	}

	inputs := []*utxopb.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	// 引用型输入不计入，实际费用 = 0 - 90000 = -90000（负数，应该失败）
	assert.Error(t, err)
}

// TestProportionalFeePlugin_Check_UnknownFeeTokenType 测试未知的费用代币类型
func TestProportionalFeePlugin_Check_UnknownFeeTokenType(t *testing.T) {
	plugin := NewProportionalFeePlugin()

	tx := testutil.CreateTransaction(nil, nil)
	tx.FeeMechanism = &transaction.Transaction_ProportionalFee{
		ProportionalFee: &transaction.ProportionalFee{
			RateBasisPoints: 10,
			FeeToken: &transaction.TokenReference{
				// 不设置 TokenType，导致类型断言失败
			},
		},
	}

	err := plugin.Check(context.Background(), nil, nil, tx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown fee_token type")
}

// TestProportionalFeePlugin_Check_InvalidInputAmount 测试无效的输入金额
func TestProportionalFeePlugin_Check_InvalidInputAmount(t *testing.T) {
	plugin := NewProportionalFeePlugin()

	// 创建输入 UTXO（无效的原生币金额）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "100000", testutil.CreateSingleKeyLock(nil))
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

	rateBasisPoints := uint32(10)
	tx.FeeMechanism = &transaction.Transaction_ProportionalFee{
		ProportionalFee: &transaction.ProportionalFee{
			RateBasisPoints: rateBasisPoints,
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

// TestProportionalFeePlugin_Check_NegativeFee 测试负费用
func TestProportionalFeePlugin_Check_NegativeFee(t *testing.T) {
	plugin := NewProportionalFeePlugin()

	// 创建输入 UTXO（50000 原生币）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "50000", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	// 创建交易（输入 50000，输出 60000，费用为负）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint1,
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "60000", testutil.CreateSingleKeyLock(nil)),
		},
	)

	rateBasisPoints := uint32(10)
	tx.FeeMechanism = &transaction.Transaction_ProportionalFee{
		ProportionalFee: &transaction.ProportionalFee{
			RateBasisPoints: rateBasisPoints,
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

// TestProportionalFeePlugin_Check_ExactMinimumFee 测试正好等于最低费用
func TestProportionalFeePlugin_Check_ExactMinimumFee(t *testing.T) {
	plugin := NewProportionalFeePlugin()

	// 创建输入 UTXO（100000 原生币）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "100000", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	// 创建交易（输入 100000，输出 90000，费用 10000，费率 0.1%，最低费用 = 90000 * 10 / 10000 = 900）
	// 但实际费用 10000 >= 900，应该通过
	// 改为：输入 100000，输出 99100，费用 900，最低费用 = 99100 * 10 / 10000 = 991（向上取整）
	// 再改为：输入 100000，输出 99000，费用 10000，最低费用 = 99000 * 10 / 10000 = 990
	// 实际费用 10000 >= 990，应该通过
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint1,
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "99000", testutil.CreateSingleKeyLock(nil)),
		},
	)

	rateBasisPoints := uint32(10) // 0.1% = 10/10000
	tx.FeeMechanism = &transaction.Transaction_ProportionalFee{
		ProportionalFee: &transaction.ProportionalFee{
			RateBasisPoints: rateBasisPoints,
			FeeToken: &transaction.TokenReference{
				TokenType: &transaction.TokenReference_NativeToken{
					NativeToken: true,
				},
			},
		},
	}

	inputs := []*utxopb.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.NoError(t, err) // 实际费用 10000 >= 最低费用 990
}

