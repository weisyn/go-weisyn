// Package builder 提供 Type-state Builder 实现
//
// state_composed.go: ComposedTx 状态实现
package builder

import (
	"context"
	"fmt"

	"github.com/weisyn/v1/pkg/interfaces/tx"
	"github.com/weisyn/v1/pkg/types"
)

// ComposedTx 已组合的交易（状态1）- 包装类型
//
// 🎯 **设计说明**：
// - 包装 types.ComposedTx 以支持流式 API
// - 携带 builder 引用，用于访问依赖（如 ProofProvider）
// - Type-state 转换：ComposedTx → ProvenTx → SignedTx → SubmittedTx
type ComposedTx struct {
	*types.ComposedTx
	builder *Service // 回指 Builder（用于访问依赖）
}

// WithProofs 添加证明，转换到 ProvenTx
//
// 🎯 **核心逻辑**：
// 1. 检查是否已封闭
// 2. 使用 ProofProvider 为所有输入生成解锁证明
// 3. 封闭当前状态，返回 ProvenTx
//
// 参数：
//   - ctx: 上下文对象
//   - provider: 证明提供者（用于生成 UnlockingProof）
//
// 返回：
//   - *ProvenTx: 已添加证明的交易
//   - error: 生成证明失败
//
// 💡 **使用示例**：
//
//	composedTx, _ := builder.Build()
//	provenTx, err := composedTx.WithProofs(ctx, proofProvider)
func (c *ComposedTx) WithProofs(
	ctx context.Context,
	provider tx.ProofProvider,
) (*ProvenTx, error) {
	// 1. 检查是否已封闭
	if c.Sealed {
		return nil, fmt.Errorf("ComposedTx already sealed")
	}

	// 2. 使用 ProofProvider 为所有输入生成解锁证明
	if err := provider.ProvideProofs(ctx, c.Tx); err != nil {
		return nil, fmt.Errorf("生成解锁证明失败: %w", err)
	}

	// 3. 封闭当前状态
	c.Sealed = true

	// 4. 返回 ProvenTx（包装类型）
	return &ProvenTx{
		ProvenTx: &types.ProvenTx{
			Tx:     c.Tx,
			Sealed: false, // ProvenTx 初始状态为未封闭（Sign 时才封闭）
		},
		builder: c.builder,
	}, nil
}
