// Package fee_test 提供 Fee Calculator 的单元测试
//
// 🧪 **测试覆盖**：
// - 费用计算核心功能测试
// - Coinbase 交易处理测试
// - 多资产费用计算测试
// - 负费用检测测试
// - 边界条件和错误场景测试
package fee

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/core/tx/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	txiface "github.com/weisyn/v1/pkg/interfaces/tx"
)

// ==================== Fee Calculator 核心功能测试 ====================

// TestNewCalculator 测试创建费用计算器
func TestNewCalculator(t *testing.T) {
	utxoFetcher := newMockUTXOFetcher()
	calculator := NewCalculator(utxoFetcher)

	assert.NotNil(t, calculator)
	assert.NotNil(t, calculator.utxoFetcher)
}

// TestNewCalculator_NilFetcher 测试 nil fetcher（应该 panic）
func TestNewCalculator_NilFetcher(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("应该 panic")
		}
	}()

	NewCalculator(nil)
}

// TestCalculate_Success 测试计算费用成功
func TestCalculate_Success(t *testing.T) {
	// 创建带状态的 fetcher
	utxos := make(map[string]*transaction.TxOutput)
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "2000", testutil.CreateSingleKeyLock(nil))
	utxos[fmt.Sprintf("%x:%d", outpoint.TxId, outpoint.OutputIndex)] = output

	utxoFetcher := func(ctx context.Context, op *transaction.OutPoint) (*transaction.TxOutput, error) {
		key := fmt.Sprintf("%x:%d", op.TxId, op.OutputIndex)
		if output, ok := utxos[key]; ok {
			return output, nil
		}
		return nil, fmt.Errorf("UTXO not found")
	}

	calculator := NewCalculator(utxoFetcher)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil)),
		},
	)

	fees, err := calculator.Calculate(context.Background(), tx)

	assert.NoError(t, err)
	assert.NotNil(t, fees)
	assert.NotEmpty(t, fees.ByToken)
}

// TestCalculate_Coinbase 测试 Coinbase 交易
func TestCalculate_Coinbase(t *testing.T) {
	utxoFetcher := newMockUTXOFetcher()
	calculator := NewCalculator(utxoFetcher)

	// Coinbase 交易：无输入
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil)),
		},
	)

	fees, err := calculator.Calculate(context.Background(), tx)

	assert.NoError(t, err)
	assert.NotNil(t, fees)
	assert.Empty(t, fees.ByToken) // Coinbase 交易费用为空
}

// TestCalculate_MultiToken 测试多Token费用计算
func TestCalculate_MultiToken(t *testing.T) {
	utxos := make(map[string]*transaction.TxOutput)
	owner1 := testutil.RandomAddress()
	owner2 := testutil.RandomAddress()

	// 创建原生币UTXO
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(owner1, "2000", testutil.CreateSingleKeyLock(nil))
	utxos[fmt.Sprintf("%x:%d", outpoint1.TxId, outpoint1.OutputIndex)] = output1

	// 创建合约Token UTXO
	outpoint2 := testutil.CreateOutPoint(nil, 1)
	contractAddr := testutil.RandomAddress()
	classID := []byte("test-class")
	output2 := testutil.CreateContractTokenOutput(owner1, "1000", contractAddr, classID, nil)
	utxos[fmt.Sprintf("%x:%d", outpoint2.TxId, outpoint2.OutputIndex)] = output2

	utxoFetcher := func(ctx context.Context, op *transaction.OutPoint) (*transaction.TxOutput, error) {
		key := fmt.Sprintf("%x:%d", op.TxId, op.OutputIndex)
		if output, ok := utxos[key]; ok {
			return output, nil
		}
		return nil, fmt.Errorf("UTXO not found")
	}

	calculator := NewCalculator(utxoFetcher)

	// 创建交易：消耗两个UTXO，创建两个输出
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{PreviousOutput: outpoint1, IsReferenceOnly: false},
			{PreviousOutput: outpoint2, IsReferenceOnly: false},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(owner2, "1500", testutil.CreateSingleKeyLock(nil)),                          // 费用500
			testutil.CreateContractTokenOutput(owner2, "800", contractAddr, classID, nil), // 费用200
		},
	)

	fees, err := calculator.Calculate(context.Background(), tx)

	assert.NoError(t, err)
	assert.NotNil(t, fees)
	assert.Len(t, fees.ByToken, 2) // 两种Token都有费用

	// 验证原生币费用
	nativeFee, ok := fees.ByToken[txiface.TokenKey("native")]
	assert.True(t, ok)
	assert.Equal(t, "500", nativeFee.String())

	// 验证合约Token费用
	contractKey := txiface.TokenKey(fmt.Sprintf("contract:%x:%x", contractAddr, classID))
	contractFee, ok := fees.ByToken[contractKey]
	assert.True(t, ok)
	assert.Equal(t, "200", contractFee.String())
}

// TestCalculate_NegativeFee 测试负费用检测
func TestCalculate_NegativeFee(t *testing.T) {
	utxos := make(map[string]*transaction.TxOutput)
	owner := testutil.RandomAddress()

	// 创建UTXO（金额1000）
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(owner, "1000", testutil.CreateSingleKeyLock(nil))
	utxos[fmt.Sprintf("%x:%d", outpoint.TxId, outpoint.OutputIndex)] = output

	utxoFetcher := func(ctx context.Context, op *transaction.OutPoint) (*transaction.TxOutput, error) {
		key := fmt.Sprintf("%x:%d", op.TxId, op.OutputIndex)
		if output, ok := utxos[key]; ok {
			return output, nil
		}
		return nil, fmt.Errorf("UTXO not found")
	}

	calculator := NewCalculator(utxoFetcher)

	// 创建交易：输出金额大于输入（负费用）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{PreviousOutput: outpoint, IsReferenceOnly: false},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(owner, "2000", testutil.CreateSingleKeyLock(nil)), // 输出大于输入
		},
	)

	fees, err := calculator.Calculate(context.Background(), tx)

	assert.Error(t, err)
	assert.Nil(t, fees)
	assert.Contains(t, err.Error(), "负费用检测")
}

// TestCalculate_UTXONotFound 测试UTXO不存在
func TestCalculate_UTXONotFound(t *testing.T) {
	utxoFetcher := func(ctx context.Context, op *transaction.OutPoint) (*transaction.TxOutput, error) {
		return nil, fmt.Errorf("UTXO not found")
	}

	calculator := NewCalculator(utxoFetcher)

	outpoint := testutil.CreateOutPoint(nil, 0)
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{PreviousOutput: outpoint, IsReferenceOnly: false},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil)),
		},
	)

	fees, err := calculator.Calculate(context.Background(), tx)

	assert.Error(t, err)
	assert.Nil(t, fees)
	assert.Contains(t, err.Error(), "查询UTXO失败")
}

// TestCalculate_ReferenceOnlyInput 测试引用型输入（不计入费用）
func TestCalculate_ReferenceOnlyInput(t *testing.T) {
	utxos := make(map[string]*transaction.TxOutput)
	owner := testutil.RandomAddress()

	// 创建UTXO
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(owner, "2000", testutil.CreateSingleKeyLock(nil))
	utxos[fmt.Sprintf("%x:%d", outpoint1.TxId, outpoint1.OutputIndex)] = output1

	// 引用型输入（不计入费用）
	outpoint2 := testutil.CreateOutPoint(nil, 1)
	output2 := testutil.CreateNativeCoinOutput(owner, "1000", testutil.CreateSingleKeyLock(nil))
	utxos[fmt.Sprintf("%x:%d", outpoint2.TxId, outpoint2.OutputIndex)] = output2

	utxoFetcher := func(ctx context.Context, op *transaction.OutPoint) (*transaction.TxOutput, error) {
		key := fmt.Sprintf("%x:%d", op.TxId, op.OutputIndex)
		if output, ok := utxos[key]; ok {
			return output, nil
		}
		return nil, fmt.Errorf("UTXO not found")
	}

	calculator := NewCalculator(utxoFetcher)

	// 创建交易：一个普通输入 + 一个引用型输入
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{PreviousOutput: outpoint1, IsReferenceOnly: false}, // 计入费用
			{PreviousOutput: outpoint2, IsReferenceOnly: true},  // 不计入费用
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(owner, "1500", testutil.CreateSingleKeyLock(nil)), // 费用500
		},
	)

	fees, err := calculator.Calculate(context.Background(), tx)

	assert.NoError(t, err)
	assert.NotNil(t, fees)
	// 只有outpoint1计入费用，outpoint2不计入
	nativeFee, ok := fees.ByToken[txiface.TokenKey("native")]
	assert.True(t, ok)
	assert.Equal(t, "500", nativeFee.String()) // 2000 - 1500 = 500
}

// TestCalculate_NonAssetUTXO 测试非资产UTXO（不计入费用）
func TestCalculate_NonAssetUTXO(t *testing.T) {
	utxoFetcher := func(ctx context.Context, op *transaction.OutPoint) (*transaction.TxOutput, error) {
		// 返回非资产输出（Resource或State）
		// ResourceOutput 需要 Resource 字段，但这里仅用于测试非资产输出的场景
		return &transaction.TxOutput{
			Owner: testutil.RandomAddress(),
			OutputContent: &transaction.TxOutput_Resource{
				Resource: &transaction.ResourceOutput{
					// Resource 字段可以为 nil，仅用于测试非资产输出场景
				},
			},
		}, nil
	}

	calculator := NewCalculator(utxoFetcher)

	outpoint := testutil.CreateOutPoint(nil, 0)
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{PreviousOutput: outpoint, IsReferenceOnly: false},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil)),
		},
	)

	fees, err := calculator.Calculate(context.Background(), tx)

	// 非资产UTXO不计入费用，但输出有资产，应该返回错误（输出Token没有对应输入）
	assert.Error(t, err)
	assert.Nil(t, fees)
	assert.Contains(t, err.Error(), "输出Token没有对应输入")
}

// TestCalculate_ZeroFee 测试零费用（输入=输出）
func TestCalculate_ZeroFee(t *testing.T) {
	utxos := make(map[string]*transaction.TxOutput)
	owner := testutil.RandomAddress()

	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(owner, "1000", testutil.CreateSingleKeyLock(nil))
	utxos[fmt.Sprintf("%x:%d", outpoint.TxId, outpoint.OutputIndex)] = output

	utxoFetcher := func(ctx context.Context, op *transaction.OutPoint) (*transaction.TxOutput, error) {
		key := fmt.Sprintf("%x:%d", op.TxId, op.OutputIndex)
		if output, ok := utxos[key]; ok {
			return output, nil
		}
		return nil, fmt.Errorf("UTXO not found")
	}

	calculator := NewCalculator(utxoFetcher)

	// 输入=输出，费用为0
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{PreviousOutput: outpoint, IsReferenceOnly: false},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(owner, "1000", testutil.CreateSingleKeyLock(nil)),
		},
	)

	fees, err := calculator.Calculate(context.Background(), tx)

	assert.NoError(t, err)
	assert.NotNil(t, fees)
	// 零费用不记录在ByToken中
	assert.Empty(t, fees.ByToken)
}

// ==================== extractTokenInfo 测试用例 ====================

// TestExtractTokenInfo_NativeCoin 测试提取原生币信息
func TestExtractTokenInfo_NativeCoin(t *testing.T) {
	utxoFetcher := newMockUTXOFetcher()
	calculator := NewCalculator(utxoFetcher)

	assetOutput := &transaction.AssetOutput{
		AssetContent: &transaction.AssetOutput_NativeCoin{
			NativeCoin: &transaction.NativeCoinAsset{
				Amount: "1000",
			},
		},
	}

	tokenKey, amount, err := calculator.extractTokenInfo(assetOutput)

	assert.NoError(t, err)
	assert.Equal(t, txiface.TokenKey("native"), tokenKey)
	assert.Equal(t, int64(1000), amount.Int64())
}

// TestExtractTokenInfo_NativeCoin_InvalidAmount 测试无效的原生币金额
func TestExtractTokenInfo_NativeCoin_InvalidAmount(t *testing.T) {
	utxoFetcher := newMockUTXOFetcher()
	calculator := NewCalculator(utxoFetcher)

	assetOutput := &transaction.AssetOutput{
		AssetContent: &transaction.AssetOutput_NativeCoin{
			NativeCoin: &transaction.NativeCoinAsset{
				Amount: "invalid-number",
			},
		},
	}

	_, _, err := calculator.extractTokenInfo(assetOutput)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "原生币金额格式错误")
}

// TestExtractTokenInfo_ContractToken_Fungible 测试提取同质化Token信息
func TestExtractTokenInfo_ContractToken_Fungible(t *testing.T) {
	utxoFetcher := newMockUTXOFetcher()
	calculator := NewCalculator(utxoFetcher)

	contractAddr := testutil.RandomBytes(20)
	classId := testutil.RandomBytes(16)

	assetOutput := &transaction.AssetOutput{
		AssetContent: &transaction.AssetOutput_ContractToken{
			ContractToken: &transaction.ContractTokenAsset{
				ContractAddress: contractAddr,
				TokenIdentifier: &transaction.ContractTokenAsset_FungibleClassId{
					FungibleClassId: classId,
				},
				Amount: "2000",
			},
		},
	}

	tokenKey, amount, err := calculator.extractTokenInfo(assetOutput)

	assert.NoError(t, err)
	expectedKey := txiface.TokenKey(fmt.Sprintf("contract:%x:%x", contractAddr, classId))
	assert.Equal(t, expectedKey, tokenKey)
	assert.Equal(t, int64(2000), amount.Int64())
}

// TestExtractTokenInfo_ContractToken_NFT 测试提取NFT信息
func TestExtractTokenInfo_ContractToken_NFT(t *testing.T) {
	utxoFetcher := newMockUTXOFetcher()
	calculator := NewCalculator(utxoFetcher)

	contractAddr := testutil.RandomBytes(20)
	uniqueId := testutil.RandomBytes(16)

	assetOutput := &transaction.AssetOutput{
		AssetContent: &transaction.AssetOutput_ContractToken{
			ContractToken: &transaction.ContractTokenAsset{
				ContractAddress: contractAddr,
				TokenIdentifier: &transaction.ContractTokenAsset_NftUniqueId{
					NftUniqueId: uniqueId,
				},
				Amount: "1",
			},
		},
	}

	tokenKey, amount, err := calculator.extractTokenInfo(assetOutput)

	assert.NoError(t, err)
	expectedKey := txiface.TokenKey(fmt.Sprintf("contract:%x:nft:%x", contractAddr, uniqueId))
	assert.Equal(t, expectedKey, tokenKey)
	assert.Equal(t, int64(1), amount.Int64())
}

// TestExtractTokenInfo_ContractToken_SFT 测试提取SFT信息
func TestExtractTokenInfo_ContractToken_SFT(t *testing.T) {
	utxoFetcher := newMockUTXOFetcher()
	calculator := NewCalculator(utxoFetcher)

	contractAddr := testutil.RandomBytes(20)
	batchId := testutil.RandomBytes(16)
	instanceId := uint64(12345)

	assetOutput := &transaction.AssetOutput{
		AssetContent: &transaction.AssetOutput_ContractToken{
			ContractToken: &transaction.ContractTokenAsset{
				ContractAddress: contractAddr,
				TokenIdentifier: &transaction.ContractTokenAsset_SemiFungibleId{
					SemiFungibleId: &transaction.SemiFungibleId{
						BatchId:    batchId,
						InstanceId: instanceId,
					},
				},
				Amount: "5000",
			},
		},
	}

	tokenKey, amount, err := calculator.extractTokenInfo(assetOutput)

	assert.NoError(t, err)
	expectedKey := txiface.TokenKey(fmt.Sprintf("contract:%x:sft:%x:%x", contractAddr, batchId, instanceId))
	assert.Equal(t, expectedKey, tokenKey)
	assert.Equal(t, int64(5000), amount.Int64())
}

// TestExtractTokenInfo_ContractToken_InvalidAmount 测试无效的合约Token金额
func TestExtractTokenInfo_ContractToken_InvalidAmount(t *testing.T) {
	utxoFetcher := newMockUTXOFetcher()
	calculator := NewCalculator(utxoFetcher)

	contractAddr := testutil.RandomBytes(20)
	classId := testutil.RandomBytes(16)

	assetOutput := &transaction.AssetOutput{
		AssetContent: &transaction.AssetOutput_ContractToken{
			ContractToken: &transaction.ContractTokenAsset{
				ContractAddress: contractAddr,
				TokenIdentifier: &transaction.ContractTokenAsset_FungibleClassId{
					FungibleClassId: classId,
				},
				Amount: "invalid-number",
			},
		},
	}

	_, _, err := calculator.extractTokenInfo(assetOutput)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "合约Token金额格式错误")
}

// TestExtractTokenInfo_ContractToken_NoIdentifier 测试缺少Token标识符
func TestExtractTokenInfo_ContractToken_NoIdentifier(t *testing.T) {
	utxoFetcher := newMockUTXOFetcher()
	calculator := NewCalculator(utxoFetcher)

	contractAddr := testutil.RandomBytes(20)

	assetOutput := &transaction.AssetOutput{
		AssetContent: &transaction.AssetOutput_ContractToken{
			ContractToken: &transaction.ContractTokenAsset{
				ContractAddress: contractAddr,
				// 缺少 TokenIdentifier
				Amount: "1000",
			},
		},
	}

	_, _, err := calculator.extractTokenInfo(assetOutput)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "合约Token缺少标识符")
}

// TestExtractTokenInfo_UnknownType 测试未知的资产类型
func TestExtractTokenInfo_UnknownType(t *testing.T) {
	utxoFetcher := newMockUTXOFetcher()
	calculator := NewCalculator(utxoFetcher)

	assetOutput := &transaction.AssetOutput{
		// 既不是 NativeCoin 也不是 ContractToken
	}

	_, _, err := calculator.extractTokenInfo(assetOutput)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未知的资产类型")
}

// ==================== Mock 对象 ====================

// mockUTXOFetcher 模拟 UTXO Fetcher
type mockUTXOFetcher struct {
	utxos map[*transaction.OutPoint]*transaction.TxOutput
}

func newMockUTXOFetcher() txiface.UTXOFetcher {
	return func(ctx context.Context, outpoint *transaction.OutPoint) (*transaction.TxOutput, error) {
		// 简化实现：返回固定输出
		return testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil)), nil
	}
}
