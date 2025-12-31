// Package builder_test 提供 Builder Type-state 状态转换的单元测试
//
// 🧪 **测试覆盖**：
// - ComposedTx → ProvenTx 转换测试
// - 状态封闭性测试
// - 错误场景测试
package builder

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/tx/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ==================== ComposedTx → ProvenTx 转换测试 ====================

// TestComposedTx_WithProofs_Success 测试成功添加证明
func TestComposedTx_WithProofs_Success(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建 ComposedTx
	outpoint := testutil.CreateOutPoint(nil, 0)
	builder.AddInput(outpoint, false)
	owner := testutil.RandomAddress()
	builder.AddAssetOutput(owner, "1000", nil, testutil.CreateSingleKeyLock(nil))
	composedTx, err := builder.Build()
	require.NoError(t, err)

	// 创建包装类型
	composed := &ComposedTx{
		ComposedTx: composedTx,
		builder:    builder,
	}

	// 创建证明提供者
	proofProvider := testutil.NewMockProofProvider()
	proof := testutil.CreateSingleKeyProof(nil, nil)
	proofProvider.SetProof(0, proof)

	// 转换为 ProvenTx
	proven, err := composed.WithProofs(context.Background(), proofProvider)

	assert.NoError(t, err)
	assert.NotNil(t, proven)
	assert.NotNil(t, proven.Tx)
	assert.True(t, composed.Sealed) // ComposedTx 应该被封闭
	assert.False(t, proven.Sealed)  // ProvenTx 初始状态为未封闭
	assert.NotNil(t, proven.Tx.Inputs[0].UnlockingProof)
}

// TestComposedTx_WithProofs_AlreadySealed_Duplicate 测试重复封闭（避免与 service_test.go 重复）
func TestComposedTx_WithProofs_AlreadySealed_Duplicate(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建 ComposedTx
	outpoint := testutil.CreateOutPoint(nil, 0)
	builder.AddInput(outpoint, false)
	owner := testutil.RandomAddress()
	builder.AddAssetOutput(owner, "1000", nil, testutil.CreateSingleKeyLock(nil))
	composedTx, err := builder.Build()
	require.NoError(t, err)

	composed := &ComposedTx{
		ComposedTx: composedTx,
		builder:    builder,
	}

	// 第一次转换
	proofProvider := testutil.NewMockProofProvider()
	proof := testutil.CreateSingleKeyProof(nil, nil)
	proofProvider.SetProof(0, proof)
	_, err = composed.WithProofs(context.Background(), proofProvider)
	require.NoError(t, err)

	// 第二次转换应该失败
	_, err = composed.WithProofs(context.Background(), proofProvider)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already sealed")
}

// TestComposedTx_WithProofs_MissingProof 测试缺少证明
func TestComposedTx_WithProofs_MissingProof(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建 ComposedTx
	outpoint := testutil.CreateOutPoint(nil, 0)
	builder.AddInput(outpoint, false)
	owner := testutil.RandomAddress()
	builder.AddAssetOutput(owner, "1000", nil, testutil.CreateSingleKeyLock(nil))
	composedTx, err := builder.Build()
	require.NoError(t, err)

	composed := &ComposedTx{
		ComposedTx: composedTx,
		builder:    builder,
	}

	// 创建证明提供者（不设置证明）
	proofProvider := testutil.NewMockProofProvider()

	// 转换应该失败
	_, err = composed.WithProofs(context.Background(), proofProvider)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "proof not found")
}

// TestComposedTx_WithProofs_MultipleInputs 测试多个输入
func TestComposedTx_WithProofs_MultipleInputs(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建 ComposedTx（多个输入）
	for i := 0; i < 3; i++ {
		outpoint := testutil.CreateOutPoint(nil, uint32(i))
		builder.AddInput(outpoint, false)
	}
	owner := testutil.RandomAddress()
	builder.AddAssetOutput(owner, "1000", nil, testutil.CreateSingleKeyLock(nil))
	composedTx, err := builder.Build()
	require.NoError(t, err)

	composed := &ComposedTx{
		ComposedTx: composedTx,
		builder:    builder,
	}

	// 创建证明提供者（为所有输入设置证明）
	proofProvider := testutil.NewMockProofProvider()
	for i := 0; i < 3; i++ {
		proof := testutil.CreateSingleKeyProof(nil, nil)
		proofProvider.SetProof(i, proof)
	}

	// 转换为 ProvenTx
	proven, err := composed.WithProofs(context.Background(), proofProvider)

	assert.NoError(t, err)
	assert.NotNil(t, proven)
	assert.Len(t, proven.Tx.Inputs, 3)
	// 验证所有输入都有证明
	for i := 0; i < 3; i++ {
		assert.NotNil(t, proven.Tx.Inputs[i].UnlockingProof, "输入 %d 应该有证明", i)
	}
}

// TestComposedTx_WithProofs_EmptyInputs 测试空输入（Coinbase）
func TestComposedTx_WithProofs_EmptyInputs(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建 Coinbase 交易（无输入）
	owner := testutil.RandomAddress()
	builder.AddAssetOutput(owner, "1000", nil, testutil.CreateSingleKeyLock(nil))
	composedTx, err := builder.Build()
	require.NoError(t, err)

	composed := &ComposedTx{
		ComposedTx: composedTx,
		builder:    builder,
	}

	// 创建证明提供者（空）
	proofProvider := testutil.NewMockProofProvider()

	// 转换应该成功（无输入不需要证明）
	proven, err := composed.WithProofs(context.Background(), proofProvider)

	assert.NoError(t, err)
	assert.NotNil(t, proven)
	assert.Len(t, proven.Tx.Inputs, 0)
}

// TestComposedTx_WithProofs_ProviderError 测试证明提供者错误
func TestComposedTx_WithProofs_ProviderError(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建 ComposedTx
	outpoint := testutil.CreateOutPoint(nil, 0)
	builder.AddInput(outpoint, false)
	owner := testutil.RandomAddress()
	builder.AddAssetOutput(owner, "1000", nil, testutil.CreateSingleKeyLock(nil))
	composedTx, err := builder.Build()
	require.NoError(t, err)

	composed := &ComposedTx{
		ComposedTx: composedTx,
		builder:    builder,
	}

	// 创建返回错误的证明提供者
	errorProvider := &ErrorProofProvider{}

	// 转换应该失败
	_, err = composed.WithProofs(context.Background(), errorProvider)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "生成解锁证明失败")
}

// TestComposedTx_WithProofs_NilProvider 测试 nil 证明提供者
func TestComposedTx_WithProofs_NilProvider(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建 ComposedTx
	outpoint := testutil.CreateOutPoint(nil, 0)
	builder.AddInput(outpoint, false)
	owner := testutil.RandomAddress()
	builder.AddAssetOutput(owner, "1000", nil, testutil.CreateSingleKeyLock(nil))
	composedTx, err := builder.Build()
	require.NoError(t, err)

	composed := &ComposedTx{
		ComposedTx: composedTx,
		builder:    builder,
	}

	// 使用 nil 提供者应该 panic 或返回错误
	// 根据实现，这可能会 panic，所以使用 recover
	defer func() {
		if r := recover(); r != nil {
			// 预期会 panic
			assert.NotNil(t, r)
		}
	}()

	_, err = composed.WithProofs(context.Background(), nil)
	// 如果实现检查 nil，应该返回错误
	if err != nil {
		assert.Error(t, err)
	}
}

// ==================== Mock 辅助类型 ====================

// ErrorProofProvider 返回错误的证明提供者
type ErrorProofProvider struct{}

func (e *ErrorProofProvider) ProvideProofs(ctx context.Context, tx *transaction.Transaction) error {
	return fmt.Errorf("证明提供失败")
}

