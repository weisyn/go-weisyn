package hostabi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/tx/selector"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/tx"
	"github.com/weisyn/v1/pkg/types"
)

// ============================================================================
// txAdapterImpl 测试
// ============================================================================
//
// 🎯 **测试目的**：发现 txAdapterImpl 的缺陷和BUG
//
// ============================================================================

// TestNewTxAdapter 测试创建TxAdapter
func TestNewTxAdapter(t *testing.T) {
	mockDraftService := &mockDraftServiceForTxAdapter{}
	mockVerifier := &mockTxVerifier{}
	mockSelector := &selector.Service{} // 需要真实的selector，但可以传入nil的依赖

	adapter := NewTxAdapter(mockDraftService, mockVerifier, mockSelector)

	assert.NotNil(t, adapter, "应该成功创建TxAdapter")
}

// TestTxAdapterImpl_BeginTransaction 测试开始构建交易
func TestTxAdapterImpl_BeginTransaction(t *testing.T) {
	adapter := createTestTxAdapter(t)

	ctx := context.Background()
	blockHeight := uint64(100)
	blockTimestamp := uint64(1234567890)

	handle, err := adapter.BeginTransaction(ctx, blockHeight, blockTimestamp)

	assert.NoError(t, err, "应该成功")
	assert.Greater(t, handle, int32(0), "应该返回有效的handle")
}

// TestTxAdapterImpl_GetDraft 测试获取Draft对象
func TestTxAdapterImpl_GetDraft(t *testing.T) {
	adapter := createTestTxAdapter(t)

	ctx := context.Background()
	blockHeight := uint64(100)
	blockTimestamp := uint64(1234567890)

	// 先创建Draft
	handle, err := adapter.BeginTransaction(ctx, blockHeight, blockTimestamp)
	require.NoError(t, err, "应该成功创建Draft")

	// 获取Draft
	draft, err := adapter.GetDraft(ctx, handle)

	assert.NoError(t, err, "应该成功")
	assert.NotNil(t, draft, "应该返回Draft对象")
}

// TestTxAdapterImpl_GetDraft_NotFound 测试获取不存在的Draft
func TestTxAdapterImpl_GetDraft_NotFound(t *testing.T) {
	adapter := createTestTxAdapter(t)

	ctx := context.Background()
	invalidHandle := int32(999)

	draft, err := adapter.GetDraft(ctx, invalidHandle)

	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, draft, "Draft应该为nil")
	assert.Contains(t, err.Error(), "draft 不存在", "错误信息应该正确")
}

// TestTxAdapterImpl_AddCustomOutput 测试添加自定义输出
func TestTxAdapterImpl_AddCustomOutput(t *testing.T) {
	adapter := createTestTxAdapter(t)

	ctx := context.Background()
	blockHeight := uint64(100)
	blockTimestamp := uint64(1234567890)

	// 先创建Draft
	handle, err := adapter.BeginTransaction(ctx, blockHeight, blockTimestamp)
	require.NoError(t, err, "应该成功创建Draft")

	// 添加自定义输出
	output := &transaction.TxOutput{
		Owner: make([]byte, 20),
		OutputContent: &transaction.TxOutput_Asset{
			Asset: &transaction.AssetOutput{
				AssetContent: &transaction.AssetOutput_NativeCoin{
					NativeCoin: &transaction.NativeCoinAsset{
						Amount: "100",
					},
				},
			},
		},
	}

	outputIndex, err := adapter.AddCustomOutput(ctx, handle, output)

	assert.NoError(t, err, "应该成功")
	assert.Equal(t, int32(0), outputIndex, "应该返回输出索引0")
}

// TestTxAdapterImpl_AddCustomOutput_NotFound 测试为不存在的Draft添加输出
func TestTxAdapterImpl_AddCustomOutput_NotFound(t *testing.T) {
	adapter := createTestTxAdapter(t)

	ctx := context.Background()
	invalidHandle := int32(999)

	output := &transaction.TxOutput{}

	_, err := adapter.AddCustomOutput(ctx, invalidHandle, output)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "获取 Draft 失败", "错误信息应该正确")
}

// TestTxAdapterImpl_FinalizeTransaction 测试完成交易构建
func TestTxAdapterImpl_FinalizeTransaction(t *testing.T) {
	adapter := createTestTxAdapter(t)

	ctx := context.Background()
	blockHeight := uint64(100)
	blockTimestamp := uint64(1234567890)

	// 先创建Draft
	handle, err := adapter.BeginTransaction(ctx, blockHeight, blockTimestamp)
	require.NoError(t, err, "应该成功创建Draft")

	// 添加输入和输出
	outpoint := &transaction.OutPoint{
		TxId:        make([]byte, 32),
		OutputIndex: 0,
	}
	_, err = adapter.AddCustomInput(ctx, handle, outpoint, false)
	require.NoError(t, err, "应该成功添加输入")

	output := &transaction.TxOutput{
		Owner: make([]byte, 20),
		OutputContent: &transaction.TxOutput_Asset{
			Asset: &transaction.AssetOutput{
				AssetContent: &transaction.AssetOutput_NativeCoin{
					NativeCoin: &transaction.NativeCoinAsset{
						Amount: "100",
					},
				},
			},
		},
	}
	_, err = adapter.AddCustomOutput(ctx, handle, output)
	require.NoError(t, err, "应该成功添加输出")

	// 完成交易构建
	tx, err := adapter.FinalizeTransaction(ctx, handle)

	assert.NoError(t, err, "应该成功")
	assert.NotNil(t, tx, "应该返回交易对象")
	assert.Len(t, tx.Inputs, 1, "应该有1个输入")
	assert.Len(t, tx.Outputs, 1, "应该有1个输出")
}

// TestTxAdapterImpl_FinalizeTransaction_EmptyTransaction 测试空交易
func TestTxAdapterImpl_FinalizeTransaction_EmptyTransaction(t *testing.T) {
	adapter := createTestTxAdapter(t)

	ctx := context.Background()
	blockHeight := uint64(100)
	blockTimestamp := uint64(1234567890)

	// 先创建Draft（不添加任何输入或输出）
	handle, err := adapter.BeginTransaction(ctx, blockHeight, blockTimestamp)
	require.NoError(t, err, "应该成功创建Draft")

	// 完成交易构建（应该失败，因为交易为空）
	tx, err := adapter.FinalizeTransaction(ctx, handle)

	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, tx, "交易应该为nil")
	assert.Contains(t, err.Error(), "交易为空", "错误信息应该正确")
}

// TestTxAdapterImpl_FinalizeTransaction_NotFound 测试完成不存在的Draft
func TestTxAdapterImpl_FinalizeTransaction_NotFound(t *testing.T) {
	adapter := createTestTxAdapter(t)

	ctx := context.Background()
	invalidHandle := int32(999)

	tx, err := adapter.FinalizeTransaction(ctx, invalidHandle)

	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, tx, "交易应该为nil")
	assert.Contains(t, err.Error(), "获取 Draft 失败", "错误信息应该正确")
}

// TestTxAdapterImpl_CleanupDraft 测试清理Draft
func TestTxAdapterImpl_CleanupDraft(t *testing.T) {
	adapter := createTestTxAdapter(t)

	ctx := context.Background()
	blockHeight := uint64(100)
	blockTimestamp := uint64(1234567890)

	// 先创建Draft
	handle, err := adapter.BeginTransaction(ctx, blockHeight, blockTimestamp)
	require.NoError(t, err, "应该成功创建Draft")

	// 清理Draft
	err = adapter.CleanupDraft(ctx, handle)

	assert.NoError(t, err, "应该成功")

	// 验证Draft已被清理（再次获取应该失败）
	_, err = adapter.GetDraft(ctx, handle)
	assert.Error(t, err, "应该返回错误（Draft已被清理）")
}

// TestTxAdapterImpl_CleanupDraft_NotFound 测试清理不存在的Draft
func TestTxAdapterImpl_CleanupDraft_NotFound(t *testing.T) {
	adapter := createTestTxAdapter(t)

	ctx := context.Background()
	invalidHandle := int32(999)

	err := adapter.CleanupDraft(ctx, invalidHandle)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "draft 不存在", "错误信息应该正确")
}

// ============================================================================
// 辅助函数
// ============================================================================

// createTestTxAdapter 创建测试用的TxAdapter
func createTestTxAdapter(t *testing.T) TxAdapter {
	t.Helper()

	mockDraftService := &mockDraftServiceForTxAdapter{}
	mockVerifier := &mockTxVerifier{}
	mockSelector := &selector.Service{} // 需要真实的selector，但可以传入nil的依赖

	return NewTxAdapter(mockDraftService, mockVerifier, mockSelector)
}

// ============================================================================
// Mock对象定义
// ============================================================================

// mockDraftServiceForTxAdapter Mock的交易草稿服务（用于TxAdapter测试）
type mockDraftServiceForTxAdapter struct{}

func (m *mockDraftServiceForTxAdapter) CreateDraft(ctx context.Context) (*types.DraftTx, error) {
	return &types.DraftTx{
		DraftID: "draft-123",
		Tx:      &transaction.Transaction{},
	}, nil
}
func (m *mockDraftServiceForTxAdapter) LoadDraft(ctx context.Context, draftID string) (*types.DraftTx, error) {
	return &types.DraftTx{
		DraftID: draftID,
		Tx:      &transaction.Transaction{},
	}, nil
}
func (m *mockDraftServiceForTxAdapter) SaveDraft(ctx context.Context, draft *types.DraftTx) error {
	return nil
}
func (m *mockDraftServiceForTxAdapter) GetDraftByID(ctx context.Context, draftID string) (*types.DraftTx, error) {
	return nil, nil
}
func (m *mockDraftServiceForTxAdapter) ValidateDraft(ctx context.Context, draft *types.DraftTx) error {
	return nil
}
func (m *mockDraftServiceForTxAdapter) SealDraft(ctx context.Context, draft *types.DraftTx) (*types.ComposedTx, error) {
	return nil, nil
}
func (m *mockDraftServiceForTxAdapter) DeleteDraft(ctx context.Context, draftID string) error {
	return nil
}
func (m *mockDraftServiceForTxAdapter) AddInput(ctx context.Context, draft *types.DraftTx, outpoint *transaction.OutPoint, isReferenceOnly bool, unlockingProof *transaction.UnlockingProof) (uint32, error) {
	draft.Tx.Inputs = append(draft.Tx.Inputs, &transaction.TxInput{})
	return uint32(len(draft.Tx.Inputs) - 1), nil
}
func (m *mockDraftServiceForTxAdapter) AddAssetOutput(ctx context.Context, draft *types.DraftTx, owner []byte, amount string, tokenID []byte, lockingConditions []*transaction.LockingCondition) (uint32, error) {
	draft.Tx.Outputs = append(draft.Tx.Outputs, &transaction.TxOutput{})
	return uint32(len(draft.Tx.Outputs) - 1), nil
}
func (m *mockDraftServiceForTxAdapter) AddResourceOutput(ctx context.Context, draft *types.DraftTx, contentHash []byte, category string, owner []byte, lockingConditions []*transaction.LockingCondition, metadata []byte) (uint32, error) {
	draft.Tx.Outputs = append(draft.Tx.Outputs, &transaction.TxOutput{})
	return uint32(len(draft.Tx.Outputs) - 1), nil
}
func (m *mockDraftServiceForTxAdapter) AddStateOutput(ctx context.Context, draft *types.DraftTx, stateID []byte, stateVersion uint64, executionResultHash []byte, publicInputs []byte, parentStateHash []byte) (uint32, error) {
	draft.Tx.Outputs = append(draft.Tx.Outputs, &transaction.TxOutput{})
	return uint32(len(draft.Tx.Outputs) - 1), nil
}

// mockTxVerifier Mock的交易验证器
type mockTxVerifier struct{}

func (m *mockTxVerifier) Verify(ctx context.Context, tx *transaction.Transaction) error {
	return nil
}

func (m *mockTxVerifier) RegisterAuthZPlugin(plugin tx.AuthZPlugin) {
	// 空实现
}

func (m *mockTxVerifier) RegisterConservationPlugin(plugin tx.ConservationPlugin) {
	// 空实现
}

func (m *mockTxVerifier) RegisterConditionPlugin(plugin tx.ConditionPlugin) {
	// 空实现
}

