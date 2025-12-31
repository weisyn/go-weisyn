// Package planner_test 提供 Planner 服务的单元测试
//
// 🧪 **测试覆盖**：
// - Planner 核心功能测试
// - UTXO 选择和找零计算测试
// - 多资产处理测试
// - 边界条件和错误场景测试
package planner

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/core/tx/selector"
	"github.com/weisyn/v1/internal/core/tx/testutil"
	utxopb "github.com/weisyn/v1/pb/blockchain/utxo"
)

// ==================== Planner 核心功能测试 ====================

// TestNewService 测试创建新的 Planner
func TestNewService(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	selectorService := selector.NewService(utxoQuery, &testutil.MockLogger{})
	draftService := testutil.NewMockDraftService()
	logger := &testutil.MockLogger{}

	planner := NewService(selectorService, draftService, logger)

	assert.NotNil(t, planner)
	assert.NotNil(t, planner.selector)
	assert.NotNil(t, planner.draftService)
	assert.NotNil(t, planner.logger)
}

// TestPlanAndBuildTransfer_Success 测试规划并构建转账
func TestPlanAndBuildTransfer_Success(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	selectorService := selector.NewService(utxoQuery, &testutil.MockLogger{})
	draftService := testutil.NewMockDraftService()
	logger := &testutil.MockLogger{}

	planner := NewService(selectorService, draftService, logger)

	// 准备 UTXO
	fromAddress := testutil.RandomAddress()
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(fromAddress, "2000", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	// 创建转账请求
	req := &TransferRequest{
		FromAddress:      fromAddress,
		ToAddress:        testutil.RandomAddress(),
		Amount:           "1000",
		ContractAddress:  nil,
		ClassID:          nil,
		LockingCondition: testutil.CreateSingleKeyLock(nil),
		Nonce:            1,
	}

	composed, err := planner.PlanAndBuildTransfer(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, composed)
	assert.NotNil(t, composed.Tx)
	assert.Len(t, composed.Tx.Inputs, 1)
	assert.GreaterOrEqual(t, len(composed.Tx.Outputs), 1) // 至少有一个输出（可能还有找零）
}

// TestPlanAndBuildTransfer_NilRequest 测试 nil 请求
func TestPlanAndBuildTransfer_NilRequest(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	selectorService := selector.NewService(utxoQuery, &testutil.MockLogger{})
	draftService := testutil.NewMockDraftService()
	logger := &testutil.MockLogger{}

	planner := NewService(selectorService, draftService, logger)

	composed, err := planner.PlanAndBuildTransfer(context.Background(), nil)

	assert.Error(t, err)
	assert.Nil(t, composed)
	assert.Contains(t, err.Error(), "转账请求不能为空")
}

// TestPlanAndBuildTransfer_InsufficientBalance 测试余额不足
func TestPlanAndBuildTransfer_InsufficientBalance(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	selectorService := selector.NewService(utxoQuery, &testutil.MockLogger{})
	draftService := testutil.NewMockDraftService()
	logger := &testutil.MockLogger{}

	planner := NewService(selectorService, draftService, logger)

	// 准备少量 UTXO
	fromAddress := testutil.RandomAddress()
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(fromAddress, "500", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	// 请求更多金额
	req := &TransferRequest{
		FromAddress:      fromAddress,
		ToAddress:        testutil.RandomAddress(),
		Amount:           "1000", // 超过余额
		ContractAddress:  nil,
		ClassID:          nil,
		LockingCondition: testutil.CreateSingleKeyLock(nil),
		Nonce:            1,
	}

	composed, err := planner.PlanAndBuildTransfer(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, composed)
	assert.Contains(t, err.Error(), "余额不足")
}

// TestPlanAndBuildTransfer_ChangeCalculation 测试找零计算
func TestPlanAndBuildTransfer_ChangeCalculation(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	selectorService := selector.NewService(utxoQuery, &testutil.MockLogger{})
	draftService := testutil.NewMockDraftService()
	logger := &testutil.MockLogger{}

	planner := NewService(selectorService, draftService, logger)

	// 准备 UTXO（金额大于请求金额）
	fromAddress := testutil.RandomAddress()
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(fromAddress, "2000", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	req := &TransferRequest{
		FromAddress:      fromAddress,
		ToAddress:        testutil.RandomAddress(),
		Amount:           "1000",
		ContractAddress:  nil,
		ClassID:          nil,
		LockingCondition: testutil.CreateSingleKeyLock(nil),
		Nonce:            1,
	}

	composed, err := planner.PlanAndBuildTransfer(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, composed)
	// 应该有找零输出（输出数量 >= 2：转账输出 + 找零输出）
	assert.GreaterOrEqual(t, len(composed.Tx.Outputs), 2)
}

// ==================== 多资产测试 ====================

// TestPlanAndBuildMultiAssetTransfer_Success 测试多资产转账
func TestPlanAndBuildMultiAssetTransfer_Success(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	selectorService := selector.NewService(utxoQuery, &testutil.MockLogger{})
	draftService := testutil.NewMockDraftService()
	logger := &testutil.MockLogger{}

	planner := NewService(selectorService, draftService, logger)

	// 准备多资产 UTXO
	fromAddress := testutil.RandomAddress()

	// 原生币 UTXO
	nativeOutpoint := testutil.CreateOutPoint(nil, 0)
	nativeOutput := testutil.CreateNativeCoinOutput(fromAddress, "1000", testutil.CreateSingleKeyLock(nil))
	nativeUTXO := testutil.CreateUTXO(nativeOutpoint, nativeOutput, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(nativeUTXO)

	// 合约代币 UTXO
	contractAddr := testutil.RandomAddress()
	classID := []byte("test-class")
	tokenOutpoint := testutil.CreateOutPoint(nil, 1)
	tokenOutput := testutil.CreateContractTokenOutput(fromAddress, "500", contractAddr, classID, nil)
	tokenUTXO := testutil.CreateUTXO(tokenOutpoint, tokenOutput, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(tokenUTXO)

	// 创建多资产转账请求
	req := &MultiAssetTransferRequest{
		FromAddress: fromAddress,
		Outputs: []*TransferOutput{
			{
				ToAddress:        testutil.RandomAddress(),
				Amount:           "500",
				ContractAddress:  nil,
				ClassID:          nil,
				LockingCondition: testutil.CreateSingleKeyLock(nil),
			},
			{
				ToAddress:        testutil.RandomAddress(),
				Amount:           "200",
				ContractAddress:  contractAddr,
				ClassID:          classID,
				LockingCondition: testutil.CreateSingleKeyLock(nil),
			},
		},
		DefaultLockingCondition: testutil.CreateSingleKeyLock(nil),
		Nonce:                   1,
	}

	composed, err := planner.PlanAndBuildMultiAssetTransfer(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, composed)
	assert.NotNil(t, composed.Tx)
	assert.Len(t, composed.Tx.Inputs, 2)                  // 两个 UTXO
	assert.GreaterOrEqual(t, len(composed.Tx.Outputs), 2) // 至少两个输出（可能还有找零）
}

// TestPlanAndBuildMultiAssetTransfer_NilRequest 测试 nil 请求
func TestPlanAndBuildMultiAssetTransfer_NilRequest(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	selectorService := selector.NewService(utxoQuery, &testutil.MockLogger{})
	draftService := testutil.NewMockDraftService()
	logger := &testutil.MockLogger{}

	planner := NewService(selectorService, draftService, logger)

	composed, err := planner.PlanAndBuildMultiAssetTransfer(context.Background(), nil)

	assert.Error(t, err)
	assert.Nil(t, composed)
	assert.Contains(t, err.Error(), "转账请求不能为空")
}

// TestPlanAndBuildMultiAssetTransfer_EmptyOutputs 测试空输出列表
func TestPlanAndBuildMultiAssetTransfer_EmptyOutputs(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	selectorService := selector.NewService(utxoQuery, &testutil.MockLogger{})
	draftService := testutil.NewMockDraftService()
	logger := &testutil.MockLogger{}

	planner := NewService(selectorService, draftService, logger)

	req := &MultiAssetTransferRequest{
		FromAddress:            testutil.RandomAddress(),
		Outputs:                []*TransferOutput{}, // 空输出列表
		DefaultLockingCondition: testutil.CreateSingleKeyLock(nil),
		Nonce:                  1,
	}

	composed, err := planner.PlanAndBuildMultiAssetTransfer(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, composed)
	assert.Contains(t, err.Error(), "输出列表不能为空")
}

// TestPlanAndBuildMultiAssetTransfer_InvalidAmountFormat 测试无效金额格式
func TestPlanAndBuildMultiAssetTransfer_InvalidAmountFormat(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	selectorService := selector.NewService(utxoQuery, &testutil.MockLogger{})
	draftService := testutil.NewMockDraftService()
	logger := &testutil.MockLogger{}

	planner := NewService(selectorService, draftService, logger)

	// 准备 UTXO
	fromAddress := testutil.RandomAddress()
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(fromAddress, "1000", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	req := &MultiAssetTransferRequest{
		FromAddress: fromAddress,
		Outputs: []*TransferOutput{
			{
				ToAddress:        testutil.RandomAddress(),
				Amount:           "500",
				ContractAddress:  nil,
				ClassID:          nil,
				LockingCondition: testutil.CreateSingleKeyLock(nil),
			},
			{
				ToAddress:        testutil.RandomAddress(),
				Amount:           "invalid-amount", // 无效金额格式
				ContractAddress:  nil,
				ClassID:          nil,
				LockingCondition: testutil.CreateSingleKeyLock(nil),
			},
		},
		DefaultLockingCondition: testutil.CreateSingleKeyLock(nil),
		Nonce:                   1,
	}

	composed, err := planner.PlanAndBuildMultiAssetTransfer(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, composed)
	assert.Contains(t, err.Error(), "无效的金额格式")
}

// TestPlanAndBuildMultiAssetTransfer_SameAssetAccumulation 测试同一资产累加
func TestPlanAndBuildMultiAssetTransfer_SameAssetAccumulation(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	selectorService := selector.NewService(utxoQuery, &testutil.MockLogger{})
	draftService := testutil.NewMockDraftService()
	logger := &testutil.MockLogger{}

	planner := NewService(selectorService, draftService, logger)

	// 准备 UTXO
	fromAddress := testutil.RandomAddress()
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(fromAddress, "2000", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	// 创建多个相同资产的输出（应该累加）
	req := &MultiAssetTransferRequest{
		FromAddress: fromAddress,
		Outputs: []*TransferOutput{
			{
				ToAddress:        testutil.RandomAddress(),
				Amount:           "500",
				ContractAddress:  nil,
				ClassID:          nil,
				LockingCondition: testutil.CreateSingleKeyLock(nil),
			},
			{
				ToAddress:        testutil.RandomAddress(),
				Amount:           "300",
				ContractAddress:  nil, // 同一资产（原生币）
				ClassID:          nil,
				LockingCondition: testutil.CreateSingleKeyLock(nil),
			},
		},
		DefaultLockingCondition: testutil.CreateSingleKeyLock(nil),
		Nonce:                   1,
	}

	composed, err := planner.PlanAndBuildMultiAssetTransfer(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, composed)
	// 应该成功，因为总需求 800 < 2000
	assert.Len(t, composed.Tx.Inputs, 1)
	assert.GreaterOrEqual(t, len(composed.Tx.Outputs), 2) // 两个转账输出 + 可能的找零
}

// TestPlanAndBuildMultiAssetTransfer_InsufficientBalance 测试余额不足
func TestPlanAndBuildMultiAssetTransfer_InsufficientBalance(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	selectorService := selector.NewService(utxoQuery, &testutil.MockLogger{})
	draftService := testutil.NewMockDraftService()
	logger := &testutil.MockLogger{}

	planner := NewService(selectorService, draftService, logger)

	// 准备少量 UTXO
	fromAddress := testutil.RandomAddress()
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(fromAddress, "500", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	req := &MultiAssetTransferRequest{
		FromAddress: fromAddress,
		Outputs: []*TransferOutput{
			{
				ToAddress:        testutil.RandomAddress(),
				Amount:           "1000", // 超过余额
				ContractAddress:  nil,
				ClassID:          nil,
				LockingCondition: testutil.CreateSingleKeyLock(nil),
			},
		},
		DefaultLockingCondition: testutil.CreateSingleKeyLock(nil),
		Nonce:                   1,
	}

	composed, err := planner.PlanAndBuildMultiAssetTransfer(context.Background(), req)

	assert.Error(t, err)
	assert.Nil(t, composed)
	assert.Contains(t, err.Error(), "余额不足")
}

// TestPlanAndBuildTransfer_ContractToken 测试合约代币转账
func TestPlanAndBuildTransfer_ContractToken(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	selectorService := selector.NewService(utxoQuery, &testutil.MockLogger{})
	draftService := testutil.NewMockDraftService()
	logger := &testutil.MockLogger{}

	planner := NewService(selectorService, draftService, logger)

	// 准备合约代币 UTXO
	fromAddress := testutil.RandomAddress()
	contractAddr := testutil.RandomAddress()
	classID := []byte("test-class")
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateContractTokenOutput(fromAddress, "2000", contractAddr, classID, nil)
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	req := &TransferRequest{
		FromAddress:      fromAddress,
		ToAddress:        testutil.RandomAddress(),
		Amount:           "1000",
		ContractAddress:  contractAddr,
		ClassID:          classID,
		LockingCondition: testutil.CreateSingleKeyLock(nil),
		Nonce:            1,
	}

	composed, err := planner.PlanAndBuildTransfer(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, composed)
	assert.Len(t, composed.Tx.Inputs, 1)
	assert.GreaterOrEqual(t, len(composed.Tx.Outputs), 1)
}

// TestPlanAndBuildTransfer_ChangeLockingCondition 测试自定义找零锁定条件
func TestPlanAndBuildTransfer_ChangeLockingCondition(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	selectorService := selector.NewService(utxoQuery, &testutil.MockLogger{})
	draftService := testutil.NewMockDraftService()
	logger := &testutil.MockLogger{}

	planner := NewService(selectorService, draftService, logger)

	// 准备 UTXO（金额大于请求金额）
	fromAddress := testutil.RandomAddress()
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(fromAddress, "2000", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	// 使用自定义找零锁定条件
	changeLock := testutil.CreateSingleKeyLock(nil)
	req := &TransferRequest{
		FromAddress:            fromAddress,
		ToAddress:              testutil.RandomAddress(),
		Amount:                 "1000",
		ContractAddress:        nil,
		ClassID:                nil,
		LockingCondition:       testutil.CreateSingleKeyLock(nil),
		ChangeLockingCondition: changeLock,
		Nonce:                  1,
	}

	composed, err := planner.PlanAndBuildTransfer(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, composed)
	// 应该有找零输出
	assert.GreaterOrEqual(t, len(composed.Tx.Outputs), 2)
}

// ==================== safeSlicePrefix 测试 ====================

// TestSafeSlicePrefix_Empty 测试空数组
func TestSafeSlicePrefix_Empty(t *testing.T) {
	result := safeSlicePrefix([]byte{}, 8)

	assert.NotNil(t, result)
	assert.Len(t, result, 0)
}

// TestSafeSlicePrefix_ShorterThanMaxLen 测试长度小于 maxLen
func TestSafeSlicePrefix_ShorterThanMaxLen(t *testing.T) {
	data := []byte{1, 2, 3}
	result := safeSlicePrefix(data, 8)

	assert.NotNil(t, result)
	assert.Equal(t, data, result)
	assert.Len(t, result, 3)
}

// TestSafeSlicePrefix_EqualMaxLen 测试长度等于 maxLen
func TestSafeSlicePrefix_EqualMaxLen(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	result := safeSlicePrefix(data, 8)

	assert.NotNil(t, result)
	assert.Equal(t, data, result)
	assert.Len(t, result, 8)
}

// TestSafeSlicePrefix_LongerThanMaxLen 测试长度大于 maxLen
func TestSafeSlicePrefix_LongerThanMaxLen(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	result := safeSlicePrefix(data, 8)

	assert.NotNil(t, result)
	assert.Len(t, result, 8)
	assert.Equal(t, []byte{1, 2, 3, 4, 5, 6, 7, 8}, result)
}

// TestSafeSlicePrefix_ZeroMaxLen 测试 maxLen 为 0
func TestSafeSlicePrefix_ZeroMaxLen(t *testing.T) {
	data := []byte{1, 2, 3}
	result := safeSlicePrefix(data, 0)

	assert.NotNil(t, result)
	assert.Len(t, result, 0)
}
