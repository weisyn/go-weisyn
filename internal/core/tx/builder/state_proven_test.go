// Package builder_test 提供 Builder ProvenTx 状态的单元测试
//
// 🧪 **测试覆盖**：
// - ProvenTx → SignedTx 转换测试
// - 签名验证测试
// - 错误场景测试
package builder

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/tx/testutil"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== ProvenTx → SignedTx 转换测试 ====================

// TestProvenTx_Sign_Success 测试成功签名
func TestProvenTx_Sign_Success(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建完整的 ProvenTx
	_, provenTx := buildProvenTx(t, builder)

	// 签名
	signer := testutil.NewMockSigner(nil)
	signed, err := provenTx.Sign(context.Background(), signer)

	assert.NoError(t, err)
	assert.NotNil(t, signed)
	assert.NotNil(t, signed.Tx)
	assert.True(t, provenTx.Sealed) // ProvenTx 应该被封闭
}

// TestProvenTx_Sign_AlreadySealed 测试重复签名
func TestProvenTx_Sign_AlreadySealed(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建 ProvenTx
	_, provenTx := buildProvenTx(t, builder)

	// 第一次签名
	signer := testutil.NewMockSigner(nil)
	_, err := provenTx.Sign(context.Background(), signer)
	require.NoError(t, err)

	// 第二次签名应该失败
	_, err = provenTx.Sign(context.Background(), signer)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already sealed")
}

// TestProvenTx_Sign_MissingProof_Duplicate 测试缺少证明（避免与 service_test.go 重复）
func TestProvenTx_Sign_MissingProof_Duplicate(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建 ComposedTx（不添加证明）
	outpoint := testutil.CreateOutPoint(nil, 0)
	builder.AddInput(outpoint, false)
	owner := testutil.RandomAddress()
	builder.AddAssetOutput(owner, "1000", nil, testutil.CreateSingleKeyLock(nil))
	composedTx, err := builder.Build()
	require.NoError(t, err)

	// 创建 ProvenTx（但没有证明）
	provenTx := &ProvenTx{
		ProvenTx: &types.ProvenTx{
			Tx:     composedTx.Tx,
			Sealed: false,
		},
		builder: builder,
	}

	// 签名应该失败（缺少 UnlockingProof）
	signer := testutil.NewMockSigner(nil)
	_, err = provenTx.Sign(context.Background(), signer)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "缺少 UnlockingProof")
}

// TestProvenTx_Sign_EmptyInputs 测试空输入（Coinbase）
func TestProvenTx_Sign_EmptyInputs(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建 Coinbase 交易
	owner := testutil.RandomAddress()
	builder.AddAssetOutput(owner, "1000", nil, testutil.CreateSingleKeyLock(nil))
	composedTx, err := builder.Build()
	require.NoError(t, err)

	// 创建 ProvenTx（无输入）
	provenTx := &ProvenTx{
		ProvenTx: &types.ProvenTx{
			Tx:     composedTx.Tx,
			Sealed: false,
		},
		builder: builder,
	}

	// 签名应该成功（无输入不需要证明）
	signer := testutil.NewMockSigner(nil)
	signed, err := provenTx.Sign(context.Background(), signer)

	assert.NoError(t, err)
	assert.NotNil(t, signed)
	assert.Len(t, signed.Tx.Inputs, 0)
}

// TestProvenTx_Sign_MultipleInputs 测试多个输入
func TestProvenTx_Sign_MultipleInputs(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建多个输入的 ProvenTx
	for i := 0; i < 3; i++ {
		outpoint := testutil.CreateOutPoint(nil, uint32(i))
		builder.AddInput(outpoint, false)
	}
	owner := testutil.RandomAddress()
	builder.AddAssetOutput(owner, "1000", nil, testutil.CreateSingleKeyLock(nil))
	composedTx, err := builder.Build()
	require.NoError(t, err)

	// 添加证明
	composed := &ComposedTx{
		ComposedTx: composedTx,
		builder:    builder,
	}

	proofProvider := testutil.NewMockProofProvider()
	for i := 0; i < 3; i++ {
		proof := testutil.CreateSingleKeyProof(nil, nil)
		proofProvider.SetProof(i, proof)
	}
	provenTx, err := composed.WithProofs(context.Background(), proofProvider)
	require.NoError(t, err)

	// 签名应该成功
	signer := testutil.NewMockSigner(nil)
	signed, err := provenTx.Sign(context.Background(), signer)

	assert.NoError(t, err)
	assert.NotNil(t, signed)
	assert.Len(t, signed.Tx.Inputs, 3)
	// 验证所有输入都有证明
	for i := 0; i < 3; i++ {
		assert.NotNil(t, signed.Tx.Inputs[i].UnlockingProof, "输入 %d 应该有证明", i)
	}
}

// TestProvenTx_Sign_NilSigner 测试 nil signer
func TestProvenTx_Sign_NilSigner(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建 ProvenTx
	_, provenTx := buildProvenTx(t, builder)

	// 使用 nil signer（P1 MVP 阶段 signer 未使用，所以应该成功）
	signed, err := provenTx.Sign(context.Background(), nil)

	// P1 MVP 阶段 signer 参数未使用，所以 nil 也应该成功
	assert.NoError(t, err)
	assert.NotNil(t, signed)
}

// TestProvenTx_Sign_ContextCanceled 测试上下文取消
func TestProvenTx_Sign_ContextCanceled(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建 ProvenTx
	_, provenTx := buildProvenTx(t, builder)

	// 创建已取消的上下文
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// 签名（P1 MVP 阶段不检查上下文，所以应该成功）
	signer := testutil.NewMockSigner(nil)
	signed, err := provenTx.Sign(ctx, signer)

	// P1 MVP 阶段不检查上下文，所以应该成功
	assert.NoError(t, err)
	assert.NotNil(t, signed)
}

// ==================== 辅助函数 ====================

// buildProvenTx 构建一个完整的 ProvenTx（用于测试）
func buildProvenTx(t *testing.T, builder *Service) (*types.ComposedTx, *ProvenTx) {
	// 构建 ComposedTx
	outpoint := testutil.CreateOutPoint(nil, 0)
	builder.AddInput(outpoint, false)
	owner := testutil.RandomAddress()
	builder.AddAssetOutput(owner, "1000", nil, testutil.CreateSingleKeyLock(nil))
	composedTx, err := builder.Build()
	require.NoError(t, err)

	// 添加证明
	composed := &ComposedTx{
		ComposedTx: composedTx,
		builder:    builder,
	}

	proofProvider := testutil.NewMockProofProvider()
	proof := testutil.CreateSingleKeyProof(nil, nil)
	proofProvider.SetProof(0, proof)
	provenTx, err := composed.WithProofs(context.Background(), proofProvider)
	require.NoError(t, err)

	return composedTx, provenTx
}

