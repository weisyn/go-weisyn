// Package builder 提供 Type-state Builder 实现
//
// state_signed.go: SignedTx 状态实现
package builder

import (
	"context"
	"fmt"

	"github.com/weisyn/v1/pkg/interfaces/tx"
	"github.com/weisyn/v1/pkg/types"
)

// SignedTx 已签名的交易（状态3）- 包装类型
//
// 🎯 **设计说明**：
// - 包装 types.SignedTx 以支持流式 API
// - Type-state 转换：SignedTx → SubmittedTx
type SignedTx struct {
	*types.SignedTx
	builder *Service
}

// Submit 提交交易，转换到 SubmittedTx
//
// 🎯 **核心逻辑**：
// 1. 使用 Processor 提交交易到交易池
// 2. Processor 内部先验证，后提交到 TxPool
// 3. TxPool 自动广播到网络
// 4. 返回 SubmittedTx（包含 TxHash、提交时间等）
//
// 参数：
//   - ctx: 上下文对象
//   - processor: 交易处理器（负责验证 + 提交）
//
// 返回：
//   - *SubmittedTx: 已提交的交易
//   - error: 提交失败
//
// ⚠️ **注意**：
// - 验证失败会返回错误
// - 提交失败不会重试（需上层处理）
//
// 💡 **使用示例**：
//
//	submittedTx, err := signedTx.Submit(ctx, processor)
func (s *SignedTx) Submit(
	ctx context.Context,
	processor tx.TxProcessor,
) (*SubmittedTx, error) {
	// 1. 使用 Processor 提交交易
	// processor.SubmitTx 返回 *types.SubmittedTx
	submitted, err := processor.SubmitTx(ctx, s.SignedTx)
	if err != nil {
		return nil, fmt.Errorf("提交交易失败: %w", err)
	}

	// 2. 返回 SubmittedTx（包装类型）
	return &SubmittedTx{
		SubmittedTx: submitted,
		builder:     s.builder,
	}, nil
}
