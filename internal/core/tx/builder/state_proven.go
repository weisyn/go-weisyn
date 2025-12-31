// Package builder 提供 Type-state Builder 实现
//
// state_proven.go: ProvenTx 状态实现
package builder

import (
	"context"
	"fmt"

	"github.com/weisyn/v1/pkg/interfaces/tx"
	"github.com/weisyn/v1/pkg/types"
)

// ProvenTx 已添加证明的交易（状态2）- 包装类型
//
// 🎯 **设计说明**：
// - 包装 types.ProvenTx 以支持流式 API
// - Type-state 转换：ProvenTx → SignedTx → SubmittedTx
type ProvenTx struct {
	*types.ProvenTx
	builder *Service
}

// Sign 签名交易，转换到 SignedTx
//
// 🎯 **P1 MVP 简化设计**：
// 在 P1 阶段，签名已经包含在 UnlockingProof 中（由 SimpleProofProvider 生成）。
// 此方法仅执行状态转换，确保 Type-state 的正确性。
//
// 💡 **设计说明**：
// - Transaction 协议层没有单独的 Signature 字段
// - 签名通过 UnlockingProof 存储在每个输入中
// - 本方法主要用于保持 Type-state 的完整性
// - 后续阶段可以在此添加交易级签名验证
//
// 参数：
//   - ctx: 上下文对象
//   - signer: 签名器（P1 阶段未使用，保留接口一致性）
//
// 返回：
//   - *SignedTx: 已完成的交易
//   - error: 状态转换失败
//
// 💡 **使用示例**：
//
//	signedTx, err := provenTx.Sign(ctx, signer)
func (p *ProvenTx) Sign(
	ctx context.Context,
	signer tx.Signer,
) (*SignedTx, error) {
	// 1. 检查是否已封闭
	if p.Sealed {
		return nil, fmt.Errorf("ProvenTx already sealed")
	}

	// 2. P1 MVP: 验证所有输入都有 UnlockingProof
	for i, input := range p.Tx.Inputs {
		if input.UnlockingProof == nil {
			return nil, fmt.Errorf("输入 %d: 缺少 UnlockingProof", i)
		}
	}

	// 3. 封闭当前状态
	p.Sealed = true

	// 4. 返回 SignedTx（包装类型）
	return &SignedTx{
		SignedTx: &types.SignedTx{
			Tx: p.Tx,
		},
		builder: p.builder,
	}, nil
}
