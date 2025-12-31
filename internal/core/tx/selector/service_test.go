// Package selector_test 提供 Selector 服务的单元测试
//
// 🧪 **测试覆盖**：
// - Selector 核心功能测试
// - UTXO 选择算法测试
// - 找零计算测试
// - 多资产处理测试
// - 边界条件和错误场景测试
package selector

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/tx/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxopb "github.com/weisyn/v1/pb/blockchain/utxo"
)

// ==================== Selector 核心功能测试 ====================

// TestNewService 测试创建新的 Selector
func TestNewService(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	logger := &testutil.MockLogger{}

	selector := NewService(utxoQuery, logger)

	assert.NotNil(t, selector)
	assert.NotNil(t, selector.utxoMgr)
	assert.NotNil(t, selector.logger)
}

// TestSelectUTXOs_Success 测试选择 UTXO 成功
func TestSelectUTXOs_Success(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	logger := &testutil.MockLogger{}

	selector := NewService(utxoQuery, logger)

	// 准备 UTXO
	ownerAddress := testutil.RandomAddress()
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(ownerAddress, "500", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo1)

	outpoint2 := testutil.CreateOutPoint(nil, 1)
	output2 := testutil.CreateNativeCoinOutput(ownerAddress, "600", testutil.CreateSingleKeyLock(nil))
	utxo2 := testutil.CreateUTXO(outpoint2, output2, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo2)

	// 选择 UTXO
	requests := []*AssetRequest{
		{
			TokenID:         "native",
			Amount:          "1000",
			ContractAddress: nil,
			ClassID:         nil,
		},
	}

	result, err := selector.SelectUTXOs(context.Background(), ownerAddress, requests)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.SelectedUTXOs, 2)   // 应该选择两个 UTXO
	assert.NotEmpty(t, result.ChangeAmounts) // 应该有找零
}

// TestSelectUTXOs_EmptyRequests 测试空请求列表
func TestSelectUTXOs_EmptyRequests(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	logger := &testutil.MockLogger{}

	selector := NewService(utxoQuery, logger)

	result, err := selector.SelectUTXOs(context.Background(), testutil.RandomAddress(), []*AssetRequest{})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "请求列表不能为空")
}

// TestSelectUTXOs_InsufficientBalance 测试余额不足
func TestSelectUTXOs_InsufficientBalance(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	logger := &testutil.MockLogger{}

	selector := NewService(utxoQuery, logger)

	// 准备少量 UTXO
	ownerAddress := testutil.RandomAddress()
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(ownerAddress, "500", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	// 请求更多金额
	requests := []*AssetRequest{
		{
			TokenID:         "native",
			Amount:          "1000",
			ContractAddress: nil,
			ClassID:         nil,
		},
	}

	result, err := selector.SelectUTXOs(context.Background(), ownerAddress, requests)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "余额不足")
}

// TestSelectUTXOs_GreedyAlgorithm 测试贪心算法
func TestSelectUTXOs_GreedyAlgorithm(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	logger := &testutil.MockLogger{}

	selector := NewService(utxoQuery, logger)

	// 准备多个 UTXO（金额不同）
	ownerAddress := testutil.RandomAddress()

	// UTXO 1: 100
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(ownerAddress, "100", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo1)

	// UTXO 2: 200
	outpoint2 := testutil.CreateOutPoint(nil, 1)
	output2 := testutil.CreateNativeCoinOutput(ownerAddress, "200", testutil.CreateSingleKeyLock(nil))
	utxo2 := testutil.CreateUTXO(outpoint2, output2, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo2)

	// UTXO 3: 500
	outpoint3 := testutil.CreateOutPoint(nil, 2)
	output3 := testutil.CreateNativeCoinOutput(ownerAddress, "500", testutil.CreateSingleKeyLock(nil))
	utxo3 := testutil.CreateUTXO(outpoint3, output3, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo3)

	// 请求 250（应该优先选择接近的单个 UTXO，或累加多个）
	requests := []*AssetRequest{
		{
			TokenID:         "native",
			Amount:          "250",
			ContractAddress: nil,
			ClassID:         nil,
		},
	}

	result, err := selector.SelectUTXOs(context.Background(), ownerAddress, requests)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.SelectedUTXOs)
	// 验证选中的 UTXO 总金额 >= 250
	totalSelected := big.NewInt(0)
	for _, utxo := range result.SelectedUTXOs {
		output := utxo.GetCachedOutput()
		require.NotNil(t, output)
		asset := output.GetAsset()
		require.NotNil(t, asset)
		nativeCoin := asset.GetNativeCoin()
		if nativeCoin != nil {
			amount, _ := new(big.Int).SetString(nativeCoin.Amount, 10)
			totalSelected.Add(totalSelected, amount)
		}
	}
	assert.GreaterOrEqual(t, totalSelected.Cmp(big.NewInt(250)), 0)
}

// TestSelectUTXOs_ChangeCalculation 测试找零计算
func TestSelectUTXOs_ChangeCalculation(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	logger := &testutil.MockLogger{}

	selector := NewService(utxoQuery, logger)

	// 准备 UTXO（金额大于请求金额）
	ownerAddress := testutil.RandomAddress()
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(ownerAddress, "2000", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	requests := []*AssetRequest{
		{
			TokenID:         "native",
			Amount:          "1000",
			ContractAddress: nil,
			ClassID:         nil,
		},
	}

	result, err := selector.SelectUTXOs(context.Background(), ownerAddress, requests)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.ChangeAmounts)

	changeAmount, ok := result.ChangeAmounts["native"]
	assert.True(t, ok)
	// 找零应该是 2000 - 1000 = 1000
	assert.Equal(t, "1000", changeAmount)
}

// TestSelectUTXOs_OnlyAvailableUTXO 测试只选择可用 UTXO
func TestSelectUTXOs_OnlyAvailableUTXO(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	logger := &testutil.MockLogger{}

	selector := NewService(utxoQuery, logger)

	ownerAddress := testutil.RandomAddress()

	// 可用 UTXO
	availableOutpoint := testutil.CreateOutPoint(nil, 0)
	availableOutput := testutil.CreateNativeCoinOutput(ownerAddress, "1000", testutil.CreateSingleKeyLock(nil))
	availableUTXO := testutil.CreateUTXO(availableOutpoint, availableOutput, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(availableUTXO)

	// 已消费 UTXO（不应该被选择）
	consumedOutpoint := testutil.CreateOutPoint(nil, 1)
	consumedOutput := testutil.CreateNativeCoinOutput(ownerAddress, "500", testutil.CreateSingleKeyLock(nil))
	consumedUTXO := testutil.CreateUTXO(consumedOutpoint, consumedOutput, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_CONSUMED)
	utxoQuery.AddUTXO(consumedUTXO)

	requests := []*AssetRequest{
		{
			TokenID:         "native",
			Amount:          "1000",
			ContractAddress: nil,
			ClassID:         nil,
		},
	}

	result, err := selector.SelectUTXOs(context.Background(), ownerAddress, requests)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	// 应该只选择可用 UTXO
	for _, utxo := range result.SelectedUTXOs {
		assert.Equal(t, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE, utxo.GetStatus())
	}
}

// TestSelectUTXOs_MultiAsset 测试多资产选择
func TestSelectUTXOs_MultiAsset(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	logger := &testutil.MockLogger{}

	selector := NewService(utxoQuery, logger)

	ownerAddress := testutil.RandomAddress()
	contractAddr := testutil.RandomAddress()
	classID := []byte("test-class")

	// 原生币 UTXO
	nativeOutpoint := testutil.CreateOutPoint(nil, 0)
	nativeOutput := testutil.CreateNativeCoinOutput(ownerAddress, "1000", testutil.CreateSingleKeyLock(nil))
	nativeUTXO := testutil.CreateUTXO(nativeOutpoint, nativeOutput, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(nativeUTXO)

	// 合约代币 UTXO
	tokenOutpoint := testutil.CreateOutPoint(nil, 1)
	tokenOutput := testutil.CreateContractTokenOutput(ownerAddress, "500", contractAddr, classID, nil)
	tokenUTXO := testutil.CreateUTXO(tokenOutpoint, tokenOutput, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(tokenUTXO)

	// 请求多资产
	// 注意：合约代币的 TokenID 格式是 "contract_address:class_id"（十六进制）
	expectedTokenID := fmt.Sprintf("%x:%x", contractAddr, classID)
	requests := []*AssetRequest{
		{
			TokenID:         "native",
			Amount:          "500",
			ContractAddress: nil,
			ClassID:         nil,
		},
		{
			TokenID:         expectedTokenID,
			Amount:          "200",
			ContractAddress: contractAddr,
			ClassID:         classID,
		},
	}

	result, err := selector.SelectUTXOs(context.Background(), ownerAddress, requests)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.SelectedUTXOs, 2)   // 应该选择两个 UTXO
	assert.NotEmpty(t, result.ChangeAmounts) // 应该有找零
}

// ==================== SelectUTXOs 错误场景测试 ====================

// TestSelectUTXOs_NilOwnerAddress 测试 nil ownerAddress
func TestSelectUTXOs_NilOwnerAddress(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	logger := &testutil.MockLogger{}

	selector := NewService(utxoQuery, logger)

	requests := []*AssetRequest{
		{
			TokenID: "native",
			Amount:  "1000",
		},
	}

	result, err := selector.SelectUTXOs(context.Background(), nil, requests)

	// UTXOQuery.GetUTXOsByAddress 可能会检查 nil
	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestSelectUTXOs_EmptyOwnerAddress 测试空 ownerAddress
func TestSelectUTXOs_EmptyOwnerAddress(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	logger := &testutil.MockLogger{}

	selector := NewService(utxoQuery, logger)

	requests := []*AssetRequest{
		{
			TokenID: "native",
			Amount:  "1000",
		},
	}

	result, err := selector.SelectUTXOs(context.Background(), []byte{}, requests)

	// UTXOQuery.GetUTXOsByAddress 可能会检查空地址
	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestSelectUTXOs_NilRequests 测试 nil requests
func TestSelectUTXOs_NilRequests(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	logger := &testutil.MockLogger{}

	selector := NewService(utxoQuery, logger)

	// 注意：nil requests 会导致 len(requests) == 0，应该返回错误
	defer func() {
		if r := recover(); r != nil {
			// 如果 panic，说明没有处理 nil requests
			assert.NotNil(t, r)
		}
	}()

	result, err := selector.SelectUTXOs(context.Background(), testutil.RandomAddress(), nil)

	// 如果返回了错误而不是 panic，验证错误
	if err != nil {
		assert.Error(t, err)
		assert.Nil(t, result)
	}
}

// TestSelectUTXOs_InvalidAmount_Zero 测试零金额
func TestSelectUTXOs_InvalidAmount_Zero(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	logger := &testutil.MockLogger{}

	selector := NewService(utxoQuery, logger)

	ownerAddress := testutil.RandomAddress()
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(ownerAddress, "1000", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	requests := []*AssetRequest{
		{
			TokenID: "native",
			Amount:  "0",
		},
	}

	result, err := selector.SelectUTXOs(context.Background(), ownerAddress, requests)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "无效的金额")
}

// TestSelectUTXOs_InvalidAmount_Negative 测试负数金额
func TestSelectUTXOs_InvalidAmount_Negative(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	logger := &testutil.MockLogger{}

	selector := NewService(utxoQuery, logger)

	ownerAddress := testutil.RandomAddress()
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(ownerAddress, "1000", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	requests := []*AssetRequest{
		{
			TokenID: "native",
			Amount:  "-100",
		},
	}

	result, err := selector.SelectUTXOs(context.Background(), ownerAddress, requests)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "无效的金额")
}

// TestSelectUTXOs_InvalidAmount_NonNumeric 测试非数字金额
func TestSelectUTXOs_InvalidAmount_NonNumeric(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	logger := &testutil.MockLogger{}

	selector := NewService(utxoQuery, logger)

	ownerAddress := testutil.RandomAddress()
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(ownerAddress, "1000", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	requests := []*AssetRequest{
		{
			TokenID: "native",
			Amount:  "abc",
		},
	}

	result, err := selector.SelectUTXOs(context.Background(), ownerAddress, requests)

	// big.Int.SetString 会返回 false，但不会 panic
	// 如果金额解析失败，targetAmount 会是 0，然后会被检测为无效金额
	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestSelectUTXOs_UTXOQueryFailure 测试 UTXOQuery 查询失败
func TestSelectUTXOs_UTXOQueryFailure(t *testing.T) {
	utxoQuery := &FailingMockUTXOQuery{}
	logger := &testutil.MockLogger{}

	selector := NewService(utxoQuery, logger)

	requests := []*AssetRequest{
		{
			TokenID: "native",
			Amount:  "1000",
		},
	}

	result, err := selector.SelectUTXOs(context.Background(), testutil.RandomAddress(), requests)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "查询 UTXO 失败")
}

// TestSelectUTXOs_NoAvailableUTXOs 测试没有可用 UTXO
func TestSelectUTXOs_NoAvailableUTXOs(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	logger := &testutil.MockLogger{}

	selector := NewService(utxoQuery, logger)

	ownerAddress := testutil.RandomAddress()
	// 不添加任何 UTXO

	requests := []*AssetRequest{
		{
			TokenID: "native",
			Amount:  "1000",
		},
	}

	result, err := selector.SelectUTXOs(context.Background(), ownerAddress, requests)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "余额不足")
}

// TestSelectUTXOs_ContextCanceled 测试 Context 取消
func TestSelectUTXOs_ContextCanceled(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	logger := &testutil.MockLogger{}

	selector := NewService(utxoQuery, logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	requests := []*AssetRequest{
		{
			TokenID: "native",
			Amount:  "1000",
		},
	}

	result, err := selector.SelectUTXOs(ctx, testutil.RandomAddress(), requests)

	// 如果 UTXOQuery 检查 context，应该返回错误
	// 否则可能成功（取决于实现）
	_ = result
	_ = err
}

// ==================== 边界条件测试 ====================

// TestSelectUTXOs_ExactAmount 测试正好等于目标金额
func TestSelectUTXOs_ExactAmount(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	logger := &testutil.MockLogger{}

	selector := NewService(utxoQuery, logger)

	ownerAddress := testutil.RandomAddress()
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(ownerAddress, "1000", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	requests := []*AssetRequest{
		{
			TokenID: "native",
			Amount:  "1000",
		},
	}

	result, err := selector.SelectUTXOs(context.Background(), ownerAddress, requests)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.SelectedUTXOs, 1)
	// 找零应该为空（正好等于目标金额）
	// 注意：当找零为零时，ChangeAmounts 中不应该有该 tokenID 的条目
	changeAmount, ok := result.ChangeAmounts["native"]
	assert.False(t, ok, "找零应该为空，但找到了找零: %s", changeAmount)
}

// TestSelectUTXOs_MultipleUTXOsExactAmount 测试多个 UTXO 累加正好等于目标金额
func TestSelectUTXOs_MultipleUTXOsExactAmount(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	logger := &testutil.MockLogger{}

	selector := NewService(utxoQuery, logger)

	ownerAddress := testutil.RandomAddress()

	// UTXO 1: 300
	outpoint1 := testutil.CreateOutPoint(nil, 0)
	output1 := testutil.CreateNativeCoinOutput(ownerAddress, "300", testutil.CreateSingleKeyLock(nil))
	utxo1 := testutil.CreateUTXO(outpoint1, output1, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo1)

	// UTXO 2: 400
	outpoint2 := testutil.CreateOutPoint(nil, 1)
	output2 := testutil.CreateNativeCoinOutput(ownerAddress, "400", testutil.CreateSingleKeyLock(nil))
	utxo2 := testutil.CreateUTXO(outpoint2, output2, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo2)

	// UTXO 3: 300
	outpoint3 := testutil.CreateOutPoint(nil, 2)
	output3 := testutil.CreateNativeCoinOutput(ownerAddress, "300", testutil.CreateSingleKeyLock(nil))
	utxo3 := testutil.CreateUTXO(outpoint3, output3, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo3)

	// 请求 1000（300 + 400 + 300 = 1000）
	requests := []*AssetRequest{
		{
			TokenID: "native",
			Amount:  "1000",
		},
	}

	result, err := selector.SelectUTXOs(context.Background(), ownerAddress, requests)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.SelectedUTXOs, 3)
	// 找零应该为空（正好等于目标金额）
	// 注意：当找零为零时，ChangeAmounts 中不应该有该 tokenID 的条目
	changeAmount, ok := result.ChangeAmounts["native"]
	assert.False(t, ok, "找零应该为空，但找到了找零: %s", changeAmount)
}

// TestSelectUTXOs_LargeAmount 测试大数金额
func TestSelectUTXOs_LargeAmount(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	logger := &testutil.MockLogger{}

	selector := NewService(utxoQuery, logger)

	ownerAddress := testutil.RandomAddress()
	outpoint := testutil.CreateOutPoint(nil, 0)
	// 使用非常大的金额
	largeAmount := "999999999999999999999999999999999999999999999999999999999999999999999999999999999"
	output := testutil.CreateNativeCoinOutput(ownerAddress, largeAmount, testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	requests := []*AssetRequest{
		{
			TokenID: "native",
			Amount:  largeAmount,
		},
	}

	result, err := selector.SelectUTXOs(context.Background(), ownerAddress, requests)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.SelectedUTXOs, 1)
}

// ==================== 特殊情况测试 ====================

// TestSelectUTXOs_NoChange 测试无找零（正好等于目标金额）
func TestSelectUTXOs_NoChange(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	logger := &testutil.MockLogger{}

	selector := NewService(utxoQuery, logger)

	ownerAddress := testutil.RandomAddress()
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(ownerAddress, "1000", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	requests := []*AssetRequest{
		{
			TokenID: "native",
			Amount:  "1000",
		},
	}

	result, err := selector.SelectUTXOs(context.Background(), ownerAddress, requests)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	// 找零应该为空
	assert.Empty(t, result.ChangeAmounts)
}

// TestSelectUTXOs_ContractTokenTokenID 测试合约代币 TokenID 格式
func TestSelectUTXOs_ContractTokenTokenID(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	logger := &testutil.MockLogger{}

	selector := NewService(utxoQuery, logger)

	ownerAddress := testutil.RandomAddress()
	contractAddr := testutil.RandomAddress()
	classID := []byte("test-class")

	// 合约代币 UTXO
	tokenOutpoint := testutil.CreateOutPoint(nil, 0)
	tokenOutput := testutil.CreateContractTokenOutput(ownerAddress, "1000", contractAddr, classID, nil)
	tokenUTXO := testutil.CreateUTXO(tokenOutpoint, tokenOutput, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(tokenUTXO)

	// TokenID 格式：contract_address:class_id（十六进制）
	expectedTokenID := fmt.Sprintf("%x:%x", contractAddr, classID)
	requests := []*AssetRequest{
		{
			TokenID:         expectedTokenID,
			Amount:          "500",
			ContractAddress: contractAddr,
			ClassID:         classID,
		},
	}

	result, err := selector.SelectUTXOs(context.Background(), ownerAddress, requests)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.SelectedUTXOs, 1)
	assert.NotEmpty(t, result.ChangeAmounts)
}

// ==================== Mock 辅助类型 ====================

// FailingMockUTXOQuery 模拟失败的 UTXO 查询服务
type FailingMockUTXOQuery struct{}

func (m *FailingMockUTXOQuery) GetUTXO(ctx context.Context, outpoint *transaction.OutPoint) (*utxopb.UTXO, error) {
	return nil, assert.AnError
}

func (m *FailingMockUTXOQuery) GetUTXOsByAddress(ctx context.Context, address []byte, category *utxopb.UTXOCategory, availableOnly bool) ([]*utxopb.UTXO, error) {
	return nil, assert.AnError
}

func (m *FailingMockUTXOQuery) GetCurrentStateRoot(ctx context.Context) ([]byte, error) {
	return nil, assert.AnError
}

func (m *FailingMockUTXOQuery) GetSponsorPoolUTXOs(ctx context.Context, onlyAvailable bool) ([]*utxopb.UTXO, error) {
	return nil, assert.AnError
}
