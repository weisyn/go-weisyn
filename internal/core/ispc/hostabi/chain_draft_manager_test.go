package hostabi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/types"
)

// ============================================================================
// chainDraftManagerImpl 测试
// ============================================================================
//
// 🎯 **测试目的**：发现 chainDraftManagerImpl 的缺陷和BUG
//
// ============================================================================

// TestChainDraftManagerImpl_CreateDraft 测试创建Draft
func TestChainDraftManagerImpl_CreateDraft(t *testing.T) {
	mockDraftService := &mockDraftServiceForChainDraftManager{}
	manager := newChainDraftManager(mockDraftService)

	ctx := context.Background()
	blockHeight := uint64(100)
	blockTimestamp := uint64(1234567890)

	handle, err := manager.CreateDraft(ctx, blockHeight, blockTimestamp)

	assert.NoError(t, err, "应该成功")
	assert.Greater(t, handle, int32(0), "应该返回有效的handle")
}

// TestChainDraftManagerImpl_GetDraft 测试获取Draft
func TestChainDraftManagerImpl_GetDraft(t *testing.T) {
	mockDraftService := &mockDraftServiceForChainDraftManager{}
	manager := newChainDraftManager(mockDraftService)

	ctx := context.Background()
	blockHeight := uint64(100)
	blockTimestamp := uint64(1234567890)

	// 先创建Draft
	handle, err := manager.CreateDraft(ctx, blockHeight, blockTimestamp)
	require.NoError(t, err, "应该成功创建Draft")

	// 获取Draft
	draft, err := manager.GetDraft(ctx, handle)

	assert.NoError(t, err, "应该成功")
	assert.NotNil(t, draft, "应该返回Draft对象")
}

// TestChainDraftManagerImpl_GetDraft_NotFound 测试获取不存在的Draft
func TestChainDraftManagerImpl_GetDraft_NotFound(t *testing.T) {
	mockDraftService := &mockDraftServiceForChainDraftManager{}
	manager := newChainDraftManager(mockDraftService)

	ctx := context.Background()
	invalidHandle := int32(999)

	draft, err := manager.GetDraft(ctx, invalidHandle)

	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, draft, "Draft应该为nil")
	assert.Contains(t, err.Error(), "draft 不存在", "错误信息应该正确")
}

// TestChainDraftManagerImpl_RemoveDraft 测试清理Draft
func TestChainDraftManagerImpl_RemoveDraft(t *testing.T) {
	mockDraftService := &mockDraftServiceForChainDraftManager{}
	manager := newChainDraftManager(mockDraftService)

	ctx := context.Background()
	blockHeight := uint64(100)
	blockTimestamp := uint64(1234567890)

	// 先创建Draft
	handle, err := manager.CreateDraft(ctx, blockHeight, blockTimestamp)
	require.NoError(t, err, "应该成功创建Draft")

	// 清理Draft
	err = manager.RemoveDraft(ctx, handle)

	assert.NoError(t, err, "应该成功")

	// 验证Draft已被清理（再次获取应该失败）
	_, err = manager.GetDraft(ctx, handle)
	assert.Error(t, err, "应该返回错误（Draft已被清理）")
}

// TestChainDraftManagerImpl_RemoveDraft_NotFound 测试清理不存在的Draft
func TestChainDraftManagerImpl_RemoveDraft_NotFound(t *testing.T) {
	mockDraftService := &mockDraftServiceForChainDraftManager{}
	manager := newChainDraftManager(mockDraftService)

	ctx := context.Background()
	invalidHandle := int32(999)

	err := manager.RemoveDraft(ctx, invalidHandle)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "draft 不存在", "错误信息应该正确")
}

// TestChainDraftManagerImpl_CleanupAll 测试清理所有Draft
func TestChainDraftManagerImpl_CleanupAll(t *testing.T) {
	mockDraftService := &mockDraftServiceForChainDraftManager{}
	manager := newChainDraftManager(mockDraftService)

	ctx := context.Background()
	blockHeight := uint64(100)
	blockTimestamp := uint64(1234567890)

	// 创建多个Draft
	handle1, err := manager.CreateDraft(ctx, blockHeight, blockTimestamp)
	require.NoError(t, err, "应该成功创建Draft1")

	handle2, err := manager.CreateDraft(ctx, blockHeight, blockTimestamp)
	require.NoError(t, err, "应该成功创建Draft2")

	// 清理所有Draft
	err = manager.CleanupAll(ctx)

	assert.NoError(t, err, "应该成功")

	// 验证所有Draft已被清理
	_, err = manager.GetDraft(ctx, handle1)
	assert.Error(t, err, "Draft1应该已被清理")

	_, err = manager.GetDraft(ctx, handle2)
	assert.Error(t, err, "Draft2应该已被清理")
}

// TestChainDraftManagerImpl_CreateDraft_CreateDraftFailed 测试CreateDraft失败
func TestChainDraftManagerImpl_CreateDraft_CreateDraftFailed(t *testing.T) {
	mockDraftService := &mockDraftServiceForChainDraftManager{
		createDraftError: assert.AnError,
	}
	manager := newChainDraftManager(mockDraftService)

	ctx := context.Background()
	blockHeight := uint64(100)
	blockTimestamp := uint64(1234567890)

	handle, err := manager.CreateDraft(ctx, blockHeight, blockTimestamp)

	assert.Error(t, err, "应该返回错误")
	assert.Equal(t, int32(0), handle, "handle应该为0")
	assert.Contains(t, err.Error(), "创建 Draft 失败", "错误信息应该正确")
}

// ============================================================================
// Mock对象定义
// ============================================================================

// mockDraftServiceForChainDraftManager Mock的交易草稿服务（用于chainDraftManager测试）
type mockDraftServiceForChainDraftManager struct {
	createDraftError error
}

func (m *mockDraftServiceForChainDraftManager) CreateDraft(ctx context.Context) (*types.DraftTx, error) {
	if m.createDraftError != nil {
		return nil, m.createDraftError
	}
	return &types.DraftTx{
		DraftID: "draft-123",
		Tx:      nil,
	}, nil
}
func (m *mockDraftServiceForChainDraftManager) LoadDraft(ctx context.Context, draftID string) (*types.DraftTx, error) {
	return nil, nil
}
func (m *mockDraftServiceForChainDraftManager) SaveDraft(ctx context.Context, draft *types.DraftTx) error {
	return nil
}
func (m *mockDraftServiceForChainDraftManager) GetDraftByID(ctx context.Context, draftID string) (*types.DraftTx, error) {
	return nil, nil
}
func (m *mockDraftServiceForChainDraftManager) ValidateDraft(ctx context.Context, draft *types.DraftTx) error {
	return nil
}
func (m *mockDraftServiceForChainDraftManager) SealDraft(ctx context.Context, draft *types.DraftTx) (*types.ComposedTx, error) {
	return nil, nil
}
func (m *mockDraftServiceForChainDraftManager) DeleteDraft(ctx context.Context, draftID string) error {
	return nil
}
func (m *mockDraftServiceForChainDraftManager) AddInput(ctx context.Context, draft *types.DraftTx, outpoint *transaction.OutPoint, isReferenceOnly bool, unlockingProof *transaction.UnlockingProof) (uint32, error) {
	return 0, nil
}
func (m *mockDraftServiceForChainDraftManager) AddAssetOutput(ctx context.Context, draft *types.DraftTx, owner []byte, amount string, tokenID []byte, lockingConditions []*transaction.LockingCondition) (uint32, error) {
	return 0, nil
}
func (m *mockDraftServiceForChainDraftManager) AddResourceOutput(ctx context.Context, draft *types.DraftTx, contentHash []byte, category string, owner []byte, lockingConditions []*transaction.LockingCondition, metadata []byte) (uint32, error) {
	return 0, nil
}
func (m *mockDraftServiceForChainDraftManager) AddStateOutput(ctx context.Context, draft *types.DraftTx, stateID []byte, stateVersion uint64, executionResultHash []byte, publicInputs []byte, parentStateHash []byte) (uint32, error) {
	return 0, nil
}

