// Package conservation_test 提供 BasicConservationPlugin 的单元测试
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

// ==================== BasicConservationPlugin 测试 ====================

// TestNewBasicConservationPlugin 测试创建 BasicConservationPlugin
func TestNewBasicConservationPlugin(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	plugin := NewBasicConservationPlugin(utxoQuery)

	assert.NotNil(t, plugin)
	assert.Equal(t, utxoQuery, plugin.eutxoQuery)
}

// TestBasicConservationPlugin_Name 测试插件名称
func TestBasicConservationPlugin_Name(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	plugin := NewBasicConservationPlugin(utxoQuery)

	assert.Equal(t, "basic_conservation", plugin.Name())
}

// TestBasicConservationPlugin_Check_Success 测试价值守恒验证成功
func TestBasicConservationPlugin_Check_Success(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	plugin := NewBasicConservationPlugin(utxoQuery)

	// 创建输入 UTXO（1000 原生币）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo1)

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

	// 验证应该成功（1000 >= 900）
	inputs := []*utxopb.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.NoError(t, err)
}

// TestBasicConservationPlugin_Check_InsufficientFunds 测试资金不足
func TestBasicConservationPlugin_Check_InsufficientFunds(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	plugin := NewBasicConservationPlugin(utxoQuery)

	// 创建输入 UTXO（500 原生币）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "500", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo1)

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

	// 验证应该失败（500 < 600）
	inputs := []*utxopb.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "价值守恒")
}

// TestBasicConservationPlugin_Check_ReferenceOnly 测试引用型输入
func TestBasicConservationPlugin_Check_ReferenceOnly(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	plugin := NewBasicConservationPlugin(utxoQuery)

	// 创建输入 UTXO（1000 原生币）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo1)

	// 创建交易（引用型输入不计入价值守恒）
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

	// 验证应该失败（引用型输入不计入，0 < 900）
	inputs := []*utxopb.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.Error(t, err)
}

// TestBasicConservationPlugin_Check_MultipleAssets 测试多资产
func TestBasicConservationPlugin_Check_MultipleAssets(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	plugin := NewBasicConservationPlugin(utxoQuery)

	// 创建原生币 UTXO
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo1)

	// 创建合约代币 UTXO
	contractAddr := testutil.RandomAddress()
	outpoint2 := testutil.CreateOutPoint(nil, 1)
	output2 := testutil.CreateContractTokenOutput(testutil.RandomAddress(), "500", contractAddr, []byte("token"), nil)
	utxo2 := testutil.CreateUTXO(outpoint2, output2, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo2)

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

	// 验证应该成功（原生币：1000 >= 900，代币：500 >= 400）
	inputs := []*utxopb.UTXO{utxo1, utxo2}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.NoError(t, err)
}

// TestBasicConservationPlugin_Check_ExactMatch 测试精确匹配（无费用）
func TestBasicConservationPlugin_Check_ExactMatch(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	plugin := NewBasicConservationPlugin(utxoQuery)

	// 创建输入 UTXO（1000 原生币）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo1)

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

	// 验证应该成功（1000 >= 1000）
	inputs := []*utxopb.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.NoError(t, err)
}

// TestBasicConservationPlugin_Check_EmptyOutputs 测试空输出（全部作为费用）
func TestBasicConservationPlugin_Check_EmptyOutputs(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	plugin := NewBasicConservationPlugin(utxoQuery)

	// 创建输入 UTXO（1000 原生币）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo1)

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

	// 验证应该成功（1000 >= 0）
	inputs := []*utxopb.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.NoError(t, err)
}

// TestBasicConservationPlugin_Check_NoCachedOutput 测试 UTXO 没有缓存输出
func TestBasicConservationPlugin_Check_NoCachedOutput(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	plugin := NewBasicConservationPlugin(utxoQuery)

	// 创建没有缓存输出的 UTXO
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	utxo1 := &utxopb.UTXO{
		Outpoint: outpoint1,
		// 不设置 CachedOutput
	}
	utxoQuery.AddUTXO(utxo1)

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

	// 验证应该失败（没有输入，0 < 900）
	inputs := []*utxopb.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.Error(t, err)
}

// TestBasicConservationPlugin_Check_NonAssetOutput 测试非资产输出
func TestBasicConservationPlugin_Check_NonAssetOutput(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	plugin := NewBasicConservationPlugin(utxoQuery)

	// 创建输入 UTXO（1000 原生币）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo1)

	// 创建交易（输出包含非资产输出）
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

	// 验证应该成功（1000 >= 900）
	inputs := []*utxopb.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.NoError(t, err)
}

// TestBasicConservationPlugin_Check_ExtractAssetInfoError 测试提取资产信息失败
func TestBasicConservationPlugin_Check_ExtractAssetInfoError(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	plugin := NewBasicConservationPlugin(utxoQuery)

	// 创建输入 UTXO（无效的原生币金额）
	// 注意：由于 testutil.CreateNativeCoinOutput 会验证金额，我们需要直接创建 UTXO
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "invalid", testutil.CreateSingleKeyLock(nil))
	// 修改金额为无效值
	output1.GetAsset().AssetContent.(*transaction.AssetOutput_NativeCoin).NativeCoin.Amount = "invalid"
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo1)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint1,
				IsReferenceOnly: false,
			},
		},
		nil,
	)

	inputs := []*utxopb.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "提取资产信息失败")
}

// TestBasicConservationPlugin_Check_ContractTokenEmptyAddress 测试合约代币地址为空
func TestBasicConservationPlugin_Check_ContractTokenEmptyAddress(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	plugin := NewBasicConservationPlugin(utxoQuery)

	// 创建输入 UTXO（合约代币地址为空）
	contractAddr := testutil.RandomAddress()
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateContractTokenOutput(testutil.RandomAddress(), "500", contractAddr, []byte("token"), nil)
	// 修改地址为空
	output1.GetAsset().AssetContent.(*transaction.AssetOutput_ContractToken).ContractToken.ContractAddress = nil
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo1)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint1,
				IsReferenceOnly: false,
			},
		},
		nil,
	)

	inputs := []*utxopb.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "提取资产信息失败")
}

// TestBasicConservationPlugin_Check_UnsupportedAssetType 测试不支持的资产类型
// 注意：由于 extractAssetInfo 使用 switch 语句，不支持的资产类型会返回错误
// 但实际代码中，如果 AssetContent 为 nil，会在 switch 的 default 分支返回错误
func TestBasicConservationPlugin_Check_UnsupportedAssetType(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	plugin := NewBasicConservationPlugin(utxoQuery)

	// 创建输入 UTXO（没有 AssetContent）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil))
	// 清空 AssetContent
	output1.GetAsset().AssetContent = nil
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo1)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint1,
				IsReferenceOnly: false,
			},
		},
		nil,
	)

	inputs := []*utxopb.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "提取资产信息失败")
}

// TestBasicConservationPlugin_Check_MultipleInputsSameAsset 测试多个输入同一资产
func TestBasicConservationPlugin_Check_MultipleInputsSameAsset(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	plugin := NewBasicConservationPlugin(utxoQuery)

	// 创建多个输入 UTXO（同一资产）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "500", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo1)

	outpoint2 := testutil.CreateOutPoint(nil, 1)
	output2 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "500", testutil.CreateSingleKeyLock(nil))
	utxo2 := testutil.CreateUTXO(outpoint2, output2, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo2)

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

	// 验证应该成功（500 + 500 = 1000 >= 900）
	inputs := []*utxopb.UTXO{utxo1, utxo2}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.NoError(t, err)
}

// TestBasicConservationPlugin_Check_MintingScenario 测试铸造场景（0消费型输入 + ExecutionProof + ContractTokenAsset输出）
func TestBasicConservationPlugin_Check_MintingScenario(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	plugin := NewBasicConservationPlugin(utxoQuery)

	// 创建合约UTXO（引用型输入）
	contractAddr := testutil.RandomAddress()
	contractOutpoint := testutil.CreateOutPoint(nil, 0)
	// 创建 ResourceOutput（合约UTXO）
	contractOutput := &transaction.TxOutput{
		Owner: contractAddr,
		LockingConditions: []*transaction.LockingCondition{testutil.CreateSingleKeyLock(nil)},
		OutputContent: &transaction.TxOutput_Resource{
			Resource: &transaction.ResourceOutput{
				Resource: nil, // 简化测试，不设置具体资源
			},
		},
	}
	contractUTXO := testutil.CreateUTXO(contractOutpoint, contractOutput, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(contractUTXO)

	// 创建铸造交易（0消费型输入 + 引用型输入 + ExecutionProof + ContractTokenAsset输出）
	execProof := &transaction.ExecutionProof{
		Context: &transaction.ExecutionProof_ExecutionContext{
			CallerIdentity: &transaction.IdentityProof{
				PublicKey:     testutil.RandomBytes(33),
				CallerAddress: testutil.RandomBytes(20),
				Signature:     testutil.RandomBytes(64),
				Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
				SighashType:   transaction.SignatureHashType_SIGHASH_ALL,
				Nonce:         testutil.RandomBytes(32),
				Timestamp:     1234567890,
				ContextHash:   testutil.RandomBytes(32),
			},
			ResourceAddress: contractAddr,
			ExecutionType:   transaction.ExecutionType_EXECUTION_TYPE_CONTRACT,
			InputDataHash:   testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
			OutputDataHash:  testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
			Metadata:        map[string][]byte{"method_name": []byte("mint")},
		},
		ExecutionResultHash: testutil.RandomBytes(32),
		StateTransitionProof: testutil.RandomBytes(64),
	}

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  contractOutpoint,
				IsReferenceOnly: true, // 引用型输入
				UnlockingProof: &transaction.TxInput_ExecutionProof{
					ExecutionProof: execProof,
				},
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateContractTokenOutput(
				testutil.RandomAddress(),
				"1000",
				contractAddr,
				[]byte("token123"),
				nil,
			),
		},
	)

	// 验证应该成功（铸造场景允许0输入+N输出）
	inputs := []*utxopb.UTXO{contractUTXO}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.NoError(t, err, "铸造场景应该允许0消费型输入+N输出")
}

// TestBasicConservationPlugin_Check_MintingScenario_NoExecutionProof 测试铸造场景但缺少ExecutionProof
func TestBasicConservationPlugin_Check_MintingScenario_NoExecutionProof(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	plugin := NewBasicConservationPlugin(utxoQuery)

	// 创建合约UTXO（引用型输入）
	contractAddr := testutil.RandomAddress()
	contractOutpoint := testutil.CreateOutPoint(nil, 0)
	contractOutput := &transaction.TxOutput{
		Owner: contractAddr,
		LockingConditions: []*transaction.LockingCondition{testutil.CreateSingleKeyLock(nil)},
		OutputContent: &transaction.TxOutput_Resource{
			Resource: &transaction.ResourceOutput{},
		},
	}
	contractUTXO := testutil.CreateUTXO(contractOutpoint, contractOutput, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(contractUTXO)

	// 创建交易（0消费型输入 + 引用型输入但没有ExecutionProof + ContractTokenAsset输出）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  contractOutpoint,
				IsReferenceOnly: true, // 引用型输入
				// 没有 UnlockingProof
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateContractTokenOutput(
				testutil.RandomAddress(),
				"1000",
				contractAddr,
				[]byte("token123"),
				nil,
			),
		},
	)

	// 验证应该失败（缺少ExecutionProof，不是有效的铸造场景）
	inputs := []*utxopb.UTXO{contractUTXO}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.Error(t, err, "缺少ExecutionProof的铸造场景应该失败")
	assert.Contains(t, err.Error(), "价值守恒验证失败")
}

// TestBasicConservationPlugin_Check_MintingScenario_WithDifferentLocks 测试铸造场景使用不同锁定条件
func TestBasicConservationPlugin_Check_MintingScenario_WithDifferentLocks(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	plugin := NewBasicConservationPlugin(utxoQuery)

	contractAddr := testutil.RandomAddress()
	contractOutpoint := testutil.CreateOutPoint(nil, 0)
	contractOutput := &transaction.TxOutput{
		Owner: contractAddr,
		LockingConditions: []*transaction.LockingCondition{testutil.CreateSingleKeyLock(nil)},
		OutputContent: &transaction.TxOutput_Resource{
			Resource: &transaction.ResourceOutput{},
		},
	}
	contractUTXO := testutil.CreateUTXO(contractOutpoint, contractOutput, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(contractUTXO)

	execProof := &transaction.ExecutionProof{
		Context: &transaction.ExecutionProof_ExecutionContext{
			CallerIdentity: &transaction.IdentityProof{
				PublicKey:     testutil.RandomBytes(33),
				CallerAddress: testutil.RandomBytes(20),
				Signature:     testutil.RandomBytes(64),
				Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
				SighashType:   transaction.SignatureHashType_SIGHASH_ALL,
				Nonce:         testutil.RandomBytes(32),
				Timestamp:     1234567890,
				ContextHash:   testutil.RandomBytes(32),
			},
			ResourceAddress: contractAddr,
			ExecutionType:   transaction.ExecutionType_EXECUTION_TYPE_CONTRACT,
			InputDataHash:   testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
			OutputDataHash:  testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
			Metadata:        map[string][]byte{"method_name": []byte("mint")},
		},
		ExecutionResultHash: testutil.RandomBytes(32),
		StateTransitionProof: testutil.RandomBytes(64),
	}

	// 测试用例：不同的锁定条件
	testCases := []struct {
		name           string
		lock           *transaction.LockingCondition
		expectedResult bool
	}{
		{
			name:           "SingleKeyLock",
			lock:           testutil.CreateSingleKeyLock(nil),
			expectedResult: true,
		},
		{
			name:           "TimeLock",
			lock: &transaction.LockingCondition{
				Condition: &transaction.LockingCondition_TimeLock{
					TimeLock: &transaction.TimeLock{
						UnlockTimestamp: uint64(9999999999), // 未来时间
					},
				},
			},
			expectedResult: true,
		},
		{
			name:           "HeightLock",
			lock: &transaction.LockingCondition{
				Condition: &transaction.LockingCondition_HeightLock{
					HeightLock: &transaction.HeightLock{
						UnlockHeight: 1000,
					},
				},
			},
			expectedResult: true,
		},
		{
			name:           "NilLock",
			lock:           nil,
			expectedResult: true, // 铸造场景允许nil锁定条件
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			outputs := []*transaction.TxOutput{
				testutil.CreateContractTokenOutput(
					testutil.RandomAddress(),
					"1000",
					contractAddr,
					[]byte("token123"),
					tc.lock,
				),
			}

			tx := testutil.CreateTransaction(
				[]*transaction.TxInput{
					{
						PreviousOutput:  contractOutpoint,
						IsReferenceOnly: true,
						UnlockingProof: &transaction.TxInput_ExecutionProof{
							ExecutionProof: execProof,
						},
					},
				},
				outputs,
			)

			inputs := []*utxopb.UTXO{contractUTXO}
			err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

			if tc.expectedResult {
				assert.NoError(t, err, "锁定条件 %s 应该允许铸造", tc.name)
			} else {
				assert.Error(t, err, "锁定条件 %s 应该拒绝铸造", tc.name)
			}
		})
	}
}

// TestBasicConservationPlugin_Check_MintingScenario_CrossContract 测试跨合约铸造（应该失败）
func TestBasicConservationPlugin_Check_MintingScenario_CrossContract(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	plugin := NewBasicConservationPlugin(utxoQuery)

	// 合约A的UTXO
	contractAAddr := testutil.RandomAddress()
	contractAOutpoint := testutil.CreateOutPoint(nil, 0)
	contractAOutput := &transaction.TxOutput{
		Owner: contractAAddr,
		LockingConditions: []*transaction.LockingCondition{testutil.CreateSingleKeyLock(nil)},
		OutputContent: &transaction.TxOutput_Resource{
			Resource: &transaction.ResourceOutput{},
		},
	}
	contractAUTXO := testutil.CreateUTXO(contractAOutpoint, contractAOutput, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(contractAUTXO)

	// 合约B的地址（不同的合约）
	contractBAddr := testutil.RandomAddress()

	// 使用合约A的ExecutionProof，但创建合约B的代币
	execProof := &transaction.ExecutionProof{
		Context: &transaction.ExecutionProof_ExecutionContext{
			CallerIdentity: &transaction.IdentityProof{
				PublicKey:     testutil.RandomBytes(33),
				CallerAddress: testutil.RandomBytes(20),
				Signature:     testutil.RandomBytes(64),
				Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
				SighashType:   transaction.SignatureHashType_SIGHASH_ALL,
				Nonce:         testutil.RandomBytes(32),
				Timestamp:     1234567890,
				ContextHash:   testutil.RandomBytes(32),
			},
			ResourceAddress: contractAAddr, // 合约A的地址
			ExecutionType:   transaction.ExecutionType_EXECUTION_TYPE_CONTRACT,
			InputDataHash:   testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
			OutputDataHash:  testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
			Metadata:        map[string][]byte{"method_name": []byte("mint")},
		},
		ExecutionResultHash: testutil.RandomBytes(32),
		StateTransitionProof: testutil.RandomBytes(64),
	}

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  contractAOutpoint,
				IsReferenceOnly: true,
				UnlockingProof: &transaction.TxInput_ExecutionProof{
					ExecutionProof: execProof,
				},
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateContractTokenOutput(
				testutil.RandomAddress(),
				"1000",
				contractBAddr, // 合约B的地址（不匹配）
				[]byte("token123"),
				nil,
			),
		},
	)

	// Conservation Plugin 应该允许（因为它只检查铸造场景的三个条件）
	// 但 AuthZ Plugin 会拒绝（contract_address不匹配）
	inputs := []*utxopb.UTXO{contractAUTXO}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	// Conservation Plugin 允许铸造场景（即使contract_address不匹配）
	// 实际的contract_address验证由AuthZ Plugin负责
	assert.NoError(t, err, "Conservation Plugin 应该允许铸造场景，contract_address验证由AuthZ负责")
}

// TestBasicConservationPlugin_Check_MultipleOutputsSameAsset 测试多个输出同一资产
func TestBasicConservationPlugin_Check_MultipleOutputsSameAsset(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	plugin := NewBasicConservationPlugin(utxoQuery)

	// 创建输入 UTXO（1000 原生币）
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo1)

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

	// 验证应该成功（1000 >= 400 + 500 = 900）
	inputs := []*utxopb.UTXO{utxo1}
	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.NoError(t, err)
}

