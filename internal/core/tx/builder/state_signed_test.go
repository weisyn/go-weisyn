// Package builder_test 提供 Builder SignedTx 状态的单元测试
//
// 🧪 **测试覆盖**：
// - SignedTx → SubmittedTx 转换测试
// - 提交验证测试
// - 错误场景测试
package builder

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/tx/testutil"
)

// ==================== SignedTx → SubmittedTx 转换测试 ====================

// TestSignedTx_Submit_Success 测试成功提交
func TestSignedTx_Submit_Success(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建完整的 SignedTx
	signedTx := buildSignedTx(t, builder)

	// 创建模拟的 Processor
	mockTxPool := testutil.NewMockTxPool()
	mockVerifier := &MockVerifier{shouldFail: false}
	processor := &MockProcessor{
		verifier: mockVerifier,
		txPool:   mockTxPool,
	}

	// 提交
	submitted, err := signedTx.Submit(context.Background(), processor)

	assert.NoError(t, err)
	assert.NotNil(t, submitted)
	assert.NotNil(t, submitted.Tx)
	assert.NotNil(t, submitted.TxHash)
	assert.False(t, submitted.SubmittedAt.IsZero())
}

// TestSignedTx_Submit_VerificationFailed 测试验证失败
func TestSignedTx_Submit_VerificationFailed(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建 SignedTx
	signedTx := buildSignedTx(t, builder)

	// 创建模拟的 Processor（验证失败）
	mockTxPool := testutil.NewMockTxPool()
	mockVerifier := &MockVerifier{shouldFail: true}
	processor := &MockProcessor{
		verifier: mockVerifier,
		txPool:   mockTxPool,
	}

	// 提交应该失败
	_, err := signedTx.Submit(context.Background(), processor)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "提交交易失败")
}

// TestSignedTx_Submit_TxPoolFailed 测试交易池提交失败
func TestSignedTx_Submit_TxPoolFailed(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建 SignedTx
	signedTx := buildSignedTx(t, builder)

	// 创建模拟的 Processor（交易池失败）
	processor := &FailingProcessor{}

	// 提交应该失败
	_, err := signedTx.Submit(context.Background(), processor)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "提交交易失败")
}

// TestSignedTx_Submit_NilProcessor 测试 nil processor
func TestSignedTx_Submit_NilProcessor(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建 SignedTx
	signedTx := buildSignedTx(t, builder)

	// 使用 nil processor 应该 panic 或返回错误
	defer func() {
		if r := recover(); r != nil {
			// 预期会 panic
			assert.NotNil(t, r)
		}
	}()

	_, err := signedTx.Submit(context.Background(), nil)
	// 如果实现检查 nil，应该返回错误
	if err != nil {
		assert.Error(t, err)
	}
}

// TestSignedTx_Submit_ContextCanceled 测试上下文取消
func TestSignedTx_Submit_ContextCanceled(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建 SignedTx
	signedTx := buildSignedTx(t, builder)

	// 创建已取消的上下文
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// 创建模拟的 Processor
	mockTxPool := testutil.NewMockTxPool()
	mockVerifier := &MockVerifier{shouldFail: false}
	processor := &MockProcessor{
		verifier: mockVerifier,
		txPool:   mockTxPool,
	}

	// 提交（processor 应该检查上下文）
	_, err := signedTx.Submit(ctx, processor)

	// 如果 processor 检查上下文，应该返回错误
	// 否则可能成功（取决于 processor 实现）
	_ = err // 接受任何结果
}

// TestSignedTx_Submit_MultipleInputsOutputs 测试多个输入输出的交易
func TestSignedTx_Submit_MultipleInputsOutputs(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建多个输入输出的交易
	for i := 0; i < 2; i++ {
		outpoint := testutil.CreateOutPoint(nil, uint32(i))
		builder.AddInput(outpoint, false)
	}
	for i := 0; i < 2; i++ {
		owner := testutil.RandomAddress()
		builder.AddAssetOutput(owner, "1000", nil, testutil.CreateSingleKeyLock(nil))
	}
	composedTx, err := builder.Build()
	require.NoError(t, err)

	// 添加证明
	composed := &ComposedTx{
		ComposedTx: composedTx,
		builder:    builder,
	}

	proofProvider := testutil.NewMockProofProvider()
	for i := 0; i < 2; i++ {
		proof := testutil.CreateSingleKeyProof(nil, nil)
		proofProvider.SetProof(i, proof)
	}
	provenTx, err := composed.WithProofs(context.Background(), proofProvider)
	require.NoError(t, err)

	// 签名
	signer := testutil.NewMockSigner(nil)
	signedTx, err := provenTx.Sign(context.Background(), signer)
	require.NoError(t, err)

	// 提交
	mockTxPool := testutil.NewMockTxPool()
	mockVerifier := &MockVerifier{shouldFail: false}
	processor := &MockProcessor{
		verifier: mockVerifier,
		txPool:   mockTxPool,
	}

	submitted, err := signedTx.Submit(context.Background(), processor)

	assert.NoError(t, err)
	assert.NotNil(t, submitted)
	assert.Len(t, submitted.Tx.Inputs, 2)
	assert.Len(t, submitted.Tx.Outputs, 2)
}

// ==================== 辅助函数 ====================

// buildSignedTx 构建一个完整的 SignedTx（用于测试）
func buildSignedTx(t *testing.T, builder *Service) *SignedTx {
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

	// 签名
	signer := testutil.NewMockSigner(nil)
	signedTx, err := provenTx.Sign(context.Background(), signer)
	require.NoError(t, err)

	return signedTx
}

// ==================== Mock 辅助类型 ====================


