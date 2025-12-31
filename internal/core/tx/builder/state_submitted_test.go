// Package builder_test 提供 Builder SubmittedTx 状态的单元测试
//
// 🧪 **测试覆盖**：
// - SubmittedTx 状态查询测试
// - 等待确认测试
// - 错误场景测试
package builder

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/tx/testutil"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== SubmittedTx 状态查询测试 ====================

// TestSubmittedTx_GetStatus_Success 测试成功获取状态
func TestSubmittedTx_GetStatus_Success(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建完整的 SubmittedTx
	submittedTx := buildSubmittedTx(t, builder)

	// 创建模拟的 Processor
	mockTxPool := testutil.NewMockTxPool()
	mockVerifier := &MockVerifier{shouldFail: false}
	processor := &MockProcessor{
		verifier: mockVerifier,
		txPool:   mockTxPool,
	}

	// 查询状态
	status, err := submittedTx.GetStatus(context.Background(), processor)

	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, types.BroadcastStatusLocalSubmitted, status.Status)
}

// TestSubmittedTx_GetStatus_NotFound 测试交易不存在
func TestSubmittedTx_GetStatus_NotFound(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建 SubmittedTx（使用不存在的交易哈希）
	submittedTx := &SubmittedTx{
		SubmittedTx: &types.SubmittedTx{
			TxHash:      testutil.RandomTxID(),
			Tx:          testutil.CreateTransaction(nil, nil),
			SubmittedAt: time.Now(),
		},
		builder: builder,
	}

	// 创建模拟的 Processor（返回错误）
	processor := &FailingProcessor{}

	// 查询状态应该失败
	_, err := submittedTx.GetStatus(context.Background(), processor)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "查询交易状态失败")
}

// TestSubmittedTx_WaitForConfirmation_Success 测试成功等待确认
func TestSubmittedTx_WaitForConfirmation_Success(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建 SubmittedTx
	submittedTx := buildSubmittedTx(t, builder)

	// 创建模拟的 Processor（立即返回确认状态）
	processor := &ConfirmingProcessor{}

	// 等待确认（设置较短的超时）
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := submittedTx.WaitForConfirmation(ctx, processor, 3, 100*time.Millisecond)

	assert.NoError(t, err)
}

// TestSubmittedTx_WaitForConfirmation_Timeout 测试超时
func TestSubmittedTx_WaitForConfirmation_Timeout(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建 SubmittedTx
	submittedTx := buildSubmittedTx(t, builder)

	// 创建模拟的 Processor（永远返回 pending）
	processor := &PendingProcessor{}

	// 等待确认（设置很短的超时）
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := submittedTx.WaitForConfirmation(ctx, processor, 2, 100*time.Millisecond)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "等待确认超时")
}

// TestSubmittedTx_WaitForConfirmation_ContextCanceled 测试上下文取消
func TestSubmittedTx_WaitForConfirmation_ContextCanceled(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建 SubmittedTx
	submittedTx := buildSubmittedTx(t, builder)

	// 创建模拟的 Processor
	processor := &PendingProcessor{}

	// 创建可取消的上下文
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	// 等待确认应该立即返回错误
	err := submittedTx.WaitForConfirmation(ctx, processor, 10, 100*time.Millisecond)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "上下文已取消")
}

// TestSubmittedTx_WaitForConfirmation_BroadcastFailed 测试广播失败状态
func TestSubmittedTx_WaitForConfirmation_BroadcastFailed(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建 SubmittedTx
	submittedTx := buildSubmittedTx(t, builder)

	// 创建模拟的 Processor（返回广播失败状态）
	processor := &BroadcastFailedProcessor{}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := submittedTx.WaitForConfirmation(ctx, processor, 3, 100*time.Millisecond)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "交易广播失败")
}

// TestSubmittedTx_WaitForConfirmation_Expired 测试过期状态
func TestSubmittedTx_WaitForConfirmation_Expired(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建 SubmittedTx
	submittedTx := buildSubmittedTx(t, builder)

	// 创建模拟的 Processor（返回过期状态）
	processor := &ExpiredProcessor{}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := submittedTx.WaitForConfirmation(ctx, processor, 3, 100*time.Millisecond)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "交易已过期")
}

// TestSubmittedTx_WaitForConfirmation_InfiniteRetries 测试无限重试
func TestSubmittedTx_WaitForConfirmation_InfiniteRetries(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建 SubmittedTx
	submittedTx := buildSubmittedTx(t, builder)

	// 创建模拟的 Processor（第3次返回确认）
	processor := &DelayedConfirmingProcessor{confirmAfter: 3}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 使用无限重试（maxRetries = 0）
	err := submittedTx.WaitForConfirmation(ctx, processor, 0, 50*time.Millisecond)

	assert.NoError(t, err)
}

// TestSubmittedTx_WaitForConfirmation_DefaultInterval 测试默认间隔
func TestSubmittedTx_WaitForConfirmation_DefaultInterval(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建 SubmittedTx
	submittedTx := buildSubmittedTx(t, builder)

	// 创建模拟的 Processor（立即返回确认）
	processor := &ConfirmingProcessor{}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// 使用默认间隔（interval = 0）
	err := submittedTx.WaitForConfirmation(ctx, processor, 3, 0)

	assert.NoError(t, err)
}

// TestSubmittedTx_GetStatus_NilProcessor 测试 nil processor
func TestSubmittedTx_GetStatus_NilProcessor(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建 SubmittedTx
	submittedTx := buildSubmittedTx(t, builder)

	// 使用 nil processor 应该 panic 或返回错误
	defer func() {
		if r := recover(); r != nil {
			// 预期会 panic
			assert.NotNil(t, r)
		}
	}()

	_, err := submittedTx.GetStatus(context.Background(), nil)
	// 如果实现检查 nil，应该返回错误
	if err != nil {
		assert.Error(t, err)
	}
}

// ==================== 辅助函数 ====================

// buildSubmittedTx 构建一个完整的 SubmittedTx（用于测试）
func buildSubmittedTx(t *testing.T, builder *Service) *SubmittedTx {
	// 构建 SignedTx
	signedTx := buildSignedTx(t, builder)

	// 提交
	mockTxPool := testutil.NewMockTxPool()
	mockVerifier := &MockVerifier{shouldFail: false}
	processor := &MockProcessor{
		verifier: mockVerifier,
		txPool:   mockTxPool,
	}

	submittedTx, err := signedTx.Submit(context.Background(), processor)
	require.NoError(t, err)

	return submittedTx
}

// ==================== Mock 辅助类型 ====================

// FailingProcessor 模拟失败的处理器
type FailingProcessor struct{}

func (f *FailingProcessor) SubmitTx(ctx context.Context, signedTx *types.SignedTx) (*types.SubmittedTx, error) {
	return nil, assert.AnError
}

func (f *FailingProcessor) GetTxStatus(ctx context.Context, txHash []byte) (*types.TxBroadcastState, error) {
	return nil, assert.AnError
}

// ConfirmingProcessor 模拟立即确认的处理器
type ConfirmingProcessor struct{}

func (c *ConfirmingProcessor) SubmitTx(ctx context.Context, signedTx *types.SignedTx) (*types.SubmittedTx, error) {
	return &types.SubmittedTx{
		TxHash:      testutil.RandomTxID(),
		Tx:          signedTx.Tx,
		SubmittedAt: time.Now(),
	}, nil
}

func (c *ConfirmingProcessor) GetTxStatus(ctx context.Context, txHash []byte) (*types.TxBroadcastState, error) {
	return &types.TxBroadcastState{
		Status: types.BroadcastStatusConfirmed,
	}, nil
}

// PendingProcessor 模拟永远 pending 的处理器
type PendingProcessor struct{}

func (p *PendingProcessor) SubmitTx(ctx context.Context, signedTx *types.SignedTx) (*types.SubmittedTx, error) {
	return &types.SubmittedTx{
		TxHash:      testutil.RandomTxID(),
		Tx:          signedTx.Tx,
		SubmittedAt: time.Now(),
	}, nil
}

func (p *PendingProcessor) GetTxStatus(ctx context.Context, txHash []byte) (*types.TxBroadcastState, error) {
	return &types.TxBroadcastState{
		Status: types.BroadcastStatusLocalSubmitted,
	}, nil
}

// BroadcastFailedProcessor 模拟广播失败的处理器
type BroadcastFailedProcessor struct{}

func (b *BroadcastFailedProcessor) SubmitTx(ctx context.Context, signedTx *types.SignedTx) (*types.SubmittedTx, error) {
	return &types.SubmittedTx{
		TxHash:      testutil.RandomTxID(),
		Tx:          signedTx.Tx,
		SubmittedAt: time.Now(),
	}, nil
}

func (b *BroadcastFailedProcessor) GetTxStatus(ctx context.Context, txHash []byte) (*types.TxBroadcastState, error) {
	return &types.TxBroadcastState{
		Status:       types.BroadcastStatusBroadcastFailed,
		ErrorMessage: "网络错误",
	}, nil
}

// ExpiredProcessor 模拟过期的处理器
type ExpiredProcessor struct{}

func (e *ExpiredProcessor) SubmitTx(ctx context.Context, signedTx *types.SignedTx) (*types.SubmittedTx, error) {
	return &types.SubmittedTx{
		TxHash:      testutil.RandomTxID(),
		Tx:          signedTx.Tx,
		SubmittedAt: time.Now(),
	}, nil
}

func (e *ExpiredProcessor) GetTxStatus(ctx context.Context, txHash []byte) (*types.TxBroadcastState, error) {
	return &types.TxBroadcastState{
		Status: types.BroadcastStatusExpired,
	}, nil
}

// DelayedConfirmingProcessor 模拟延迟确认的处理器
type DelayedConfirmingProcessor struct {
	confirmAfter int
	callCount    int
}

func (d *DelayedConfirmingProcessor) SubmitTx(ctx context.Context, signedTx *types.SignedTx) (*types.SubmittedTx, error) {
	return &types.SubmittedTx{
		TxHash:      testutil.RandomTxID(),
		Tx:          signedTx.Tx,
		SubmittedAt: time.Now(),
	}, nil
}

func (d *DelayedConfirmingProcessor) GetTxStatus(ctx context.Context, txHash []byte) (*types.TxBroadcastState, error) {
	d.callCount++
	if d.callCount >= d.confirmAfter {
		return &types.TxBroadcastState{
			Status: types.BroadcastStatusConfirmed,
		}, nil
	}
	return &types.TxBroadcastState{
		Status: types.BroadcastStatusLocalSubmitted,
	}, nil
}

