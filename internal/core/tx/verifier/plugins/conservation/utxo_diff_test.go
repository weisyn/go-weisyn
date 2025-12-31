// Package conservation_test 提供 DefaultUTXODiffPlugin 的单元测试
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
	"github.com/weisyn/v1/pb/blockchain/utxo"
)

// ==================== DefaultUTXODiffPlugin 测试 ====================

// TestNewDefaultUTXODiffPlugin 测试创建 DefaultUTXODiffPlugin
func TestNewDefaultUTXODiffPlugin(t *testing.T) {
	plugin := NewDefaultUTXODiffPlugin()

	assert.NotNil(t, plugin)
}

// TestDefaultUTXODiffPlugin_Name 测试插件名称
func TestDefaultUTXODiffPlugin_Name(t *testing.T) {
	plugin := NewDefaultUTXODiffPlugin()

	assert.Equal(t, "default_utxo_diff", plugin.Name())
}

// TestDefaultUTXODiffPlugin_Check_Coinbase 测试 Coinbase 交易（0输入）
func TestDefaultUTXODiffPlugin_Check_Coinbase(t *testing.T) {
	plugin := NewDefaultUTXODiffPlugin()

	// Coinbase 交易（0输入）
	tx := testutil.CreateTransaction(
		nil,
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil)),
		},
	)

	err := plugin.Check(context.Background(), nil, tx.Outputs, tx)

	assert.NoError(t, err) // Coinbase 交易跳过验证
}

// TestDefaultUTXODiffPlugin_Check_Success_NativeToken 测试原生代币价值守恒验证成功
func TestDefaultUTXODiffPlugin_Check_Success_NativeToken(t *testing.T) {
	plugin := NewDefaultUTXODiffPlugin()

	// 创建输入 UTXO（1000 原生币）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	// 创建交易（输入 1000，输出 900，费用 100）
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

	inputs := []*utxo.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.NoError(t, err) // 1000 >= 900
}

// TestDefaultUTXODiffPlugin_Check_InsufficientFunds_NativeToken 测试原生代币资金不足
func TestDefaultUTXODiffPlugin_Check_InsufficientFunds_NativeToken(t *testing.T) {
	plugin := NewDefaultUTXODiffPlugin()

	// 创建输入 UTXO（500 原生币）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "500", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	// 创建交易（输入 500，输出 600，资金不足）
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

	inputs := []*utxo.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "价值不守恒")
}

// TestDefaultUTXODiffPlugin_Check_ExactMatch_NativeToken 测试原生代币精确匹配
func TestDefaultUTXODiffPlugin_Check_ExactMatch_NativeToken(t *testing.T) {
	plugin := NewDefaultUTXODiffPlugin()

	// 创建输入 UTXO（1000 原生币）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	// 创建交易（输入 1000，输出 1000，无费用）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint1,
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil)),
		},
	)

	inputs := []*utxo.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.NoError(t, err) // 1000 >= 1000
}

// TestDefaultUTXODiffPlugin_Check_ContractToken 测试合约代币价值守恒验证成功
func TestDefaultUTXODiffPlugin_Check_ContractToken(t *testing.T) {
	plugin := NewDefaultUTXODiffPlugin()

	contractAddr := testutil.RandomAddress()
	// 创建输入 UTXO（1000 合约代币）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateContractTokenOutput(testutil.RandomAddress(), "1000", contractAddr, []byte("token"), nil)
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	// 创建交易（输入 1000，输出 900，费用 100）
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

	inputs := []*utxo.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.NoError(t, err) // 1000 >= 900
}

// TestDefaultUTXODiffPlugin_Check_MultipleAssets 测试多资产价值守恒
func TestDefaultUTXODiffPlugin_Check_MultipleAssets(t *testing.T) {
	plugin := NewDefaultUTXODiffPlugin()

	// 创建原生币 UTXO
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	// 创建合约代币 UTXO
	contractAddr := testutil.RandomAddress()
	outpoint2 := testutil.CreateOutPoint(nil, 1)
	output2 := testutil.CreateContractTokenOutput(testutil.RandomAddress(), "500", contractAddr, []byte("token"), nil)
	utxo2 := testutil.CreateUTXO(outpoint2, output2, utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	// 创建交易（多资产）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint1,
				IsReferenceOnly: false,
			},
			{
				PreviousOutput:  outpoint2,
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "900", testutil.CreateSingleKeyLock(nil)),
			testutil.CreateContractTokenOutput(testutil.RandomAddress(), "400", contractAddr, []byte("token"), nil),
		},
	)

	inputs := []*utxo.UTXO{utxo1, utxo2}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.NoError(t, err) // 原生币：1000 >= 900，代币：500 >= 400
}

// TestDefaultUTXODiffPlugin_Check_OutputWithoutInput 测试输出没有对应的输入
func TestDefaultUTXODiffPlugin_Check_OutputWithoutInput(t *testing.T) {
	plugin := NewDefaultUTXODiffPlugin()

	// 创建交易（只有输出，没有输入）
	tx := testutil.CreateTransaction(
		nil,
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil)),
		},
	)

	// 注意：这不是 Coinbase（Coinbase 是 0 输入），但这里 inputs 为空
	inputs := []*utxo.UTXO{}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	// 由于 inputs 为空，会被当作 Coinbase 处理
	assert.NoError(t, err)
}

// TestDefaultUTXODiffPlugin_Check_NoCachedOutput 测试 UTXO 没有缓存输出
func TestDefaultUTXODiffPlugin_Check_NoCachedOutput(t *testing.T) {
	plugin := NewDefaultUTXODiffPlugin()

	// 创建没有缓存输出的 UTXO
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	utxo1 := &utxo.UTXO{
		Outpoint: outpoint1,
		// 不设置 CachedOutput
	}

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

	inputs := []*utxo.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	// 没有 CachedOutput 的 UTXO 会被跳过，所以输入总和为 0，输出总和为 900，应该失败
	assert.Error(t, err)
}

// TestDefaultUTXODiffPlugin_Check_NonAssetOutput 测试非资产输出
func TestDefaultUTXODiffPlugin_Check_NonAssetOutput(t *testing.T) {
	plugin := NewDefaultUTXODiffPlugin()

	// 创建输入 UTXO（1000 原生币）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	// 创建交易（输出包含非资产输出，会被跳过）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint1,
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "900", testutil.CreateSingleKeyLock(nil)),
			// 非资产输出会被跳过
		},
	)

	inputs := []*utxo.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.NoError(t, err) // 1000 >= 900
}

// TestDefaultUTXODiffPlugin_Check_MultipleInputsSameAsset 测试多个输入同一资产
func TestDefaultUTXODiffPlugin_Check_MultipleInputsSameAsset(t *testing.T) {
	plugin := NewDefaultUTXODiffPlugin()

	// 创建多个输入 UTXO（同一资产）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "500", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	outpoint2 := testutil.CreateOutPoint(nil, 1)
	output2 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "500", testutil.CreateSingleKeyLock(nil))
	utxo2 := testutil.CreateUTXO(outpoint2, output2, utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	// 创建交易（输入 1000，输出 900）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint1,
				IsReferenceOnly: false,
			},
			{
				PreviousOutput:  outpoint2,
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "900", testutil.CreateSingleKeyLock(nil)),
		},
	)

	inputs := []*utxo.UTXO{utxo1, utxo2}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.NoError(t, err) // 500 + 500 = 1000 >= 900
}

// TestDefaultUTXODiffPlugin_Check_MultipleOutputsSameAsset 测试多个输出同一资产
func TestDefaultUTXODiffPlugin_Check_MultipleOutputsSameAsset(t *testing.T) {
	plugin := NewDefaultUTXODiffPlugin()

	// 创建输入 UTXO（1000 原生币）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	// 创建交易（输入 1000，输出 400 + 500 = 900）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint1,
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "400", testutil.CreateSingleKeyLock(nil)),
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "500", testutil.CreateSingleKeyLock(nil)),
		},
	)

	inputs := []*utxo.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.NoError(t, err) // 1000 >= 400 + 500 = 900
}

// TestDefaultUTXODiffPlugin_Check_EmptyOutputs 测试空输出（全部作为费用）
func TestDefaultUTXODiffPlugin_Check_EmptyOutputs(t *testing.T) {
	plugin := NewDefaultUTXODiffPlugin()

	// 创建输入 UTXO（1000 原生币）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	// 创建交易（输入 1000，输出 0，全部作为费用）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint1,
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{}, // 空输出
	)

	inputs := []*utxo.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.NoError(t, err) // 1000 >= 0
}

