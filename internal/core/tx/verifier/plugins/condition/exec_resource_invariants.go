// Package condition 提供条件验证插件实现
//
// exec_resource_invariants.go: 可执行资源交易形态约束插件
package condition

import (
	"context"
	"fmt"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	txiface "github.com/weisyn/v1/pkg/interfaces/tx"
)

// ExecResourceInvariantPlugin
//
// 🎯 **核心职责**：对「带 ZKStateProof 的执行型交易」施加结构性约束，确保：
//   1) 交易至少包含一个输入（排除 0-input 非 coinbase 异常交易）
//   2) 至少存在一个 `is_reference_only = true` 的引用型输入
//   3) （若提供 VerifierEnvironment）该引用输入指向的 UTXO 必须为 ResourceOutput
//
// 💡 **设计背景**：
//   - 对应 protocol 中的 ISPC 执行模型：AssetInput(fee) + ResourceInput(reference-only) + StateOutput(result)
//   - 过去存在仅携带 StateOutput 的 0-input 交易，本插件用于在验证阶段直接拒绝此类违反协议的交易。
//
// 📞 **调用方**：Verifier Kernel（通过 Condition Hook）
type ExecResourceInvariantPlugin struct{}

// NewExecResourceInvariantPlugin 创建插件实例
func NewExecResourceInvariantPlugin() *ExecResourceInvariantPlugin {
	return &ExecResourceInvariantPlugin{}
}

// Name 返回插件名称
func (p *ExecResourceInvariantPlugin) Name() string {
	return "exec_resource_invariants"
}

// Check 执行可执行资源交易的结构性检查
//
// 规则（P1 实现版本）：
//   - 如果交易不存在 StateOutput 或 StateOutput.zk_proof，为非执行型交易 → 直接跳过
//   - 否则：
//       1. 要求 tx.Inputs 非空
//       2. 要求存在至少一个 input.is_reference_only = true
//       3. 若 VerifierEnvironment 可用，则要求该引用输入指向的 UTXO 为 ResourceOutput
func (p *ExecResourceInvariantPlugin) Check(
	ctx context.Context,
	tx *transaction.Transaction,
	blockHeight uint64,
	blockTime uint64,
) error {
	_ = blockHeight
	_ = blockTime

	if tx == nil {
		return fmt.Errorf("transaction is nil")
	}

	// 1. 检测是否为“带 ZKStateProof 的执行型交易”
	hasStateWithProof := false
	for _, out := range tx.Outputs {
		state := out.GetState()
		if state == nil || state.ZkProof == nil {
			continue
		}

		// 🚫 禁止 pending/占位 ZKProof：
		// - Proof 为空（nil/len==0）或 ConstraintCount==0 视为 pending
		// - pending 的执行型交易不得进入 mempool/进块（否则会绕过“必须有可验证证明”的共识语义）
		if len(state.ZkProof.Proof) == 0 || state.ZkProof.ConstraintCount == 0 {
			return fmt.Errorf("execution transaction has pending/empty zk_proof (proof_len=%d constraint_count=%d)",
				len(state.ZkProof.Proof), state.ZkProof.ConstraintCount)
		}

		hasStateWithProof = true
		break
	}
	if !hasStateWithProof {
		// 非执行型交易，不施加额外约束
		return nil
	}

	// 2. 执行型交易必须至少有一个输入（排除 0-input 的非法普通交易）
	if len(tx.Inputs) == 0 {
		return fmt.Errorf("execution transaction with StateOutput.zk_proof must have at least one input")
	}

	// 3. 执行型交易必须至少包含一个引用型输入（is_reference_only = true）
	var (
		hasRefInput bool
		env, _      = txiface.GetVerifierEnvironment(ctx)
	)

	for _, in := range tx.Inputs {
		if !in.IsReferenceOnly {
			continue
		}

		hasRefInput = true

		// 如果环境可用，则进一步检查引用的 UTXO 是否为 ResourceOutput
		if env == nil || in.PreviousOutput == nil {
			continue
		}

		utxo, err := env.GetUTXO(ctx, in.PreviousOutput)
		if err != nil {
			return fmt.Errorf("failed to load referenced UTXO for execution transaction: %w", err)
		}
		if utxo == nil {
			return fmt.Errorf("referenced UTXO for execution transaction is nil")
		}

		output := utxo.GetCachedOutput()
		if output == nil {
			return fmt.Errorf("referenced UTXO for execution transaction has no cached output")
		}
		if output.GetResource() == nil {
			return fmt.Errorf("referenced UTXO for execution transaction must be a ResourceOutput")
		}
	}

	if !hasRefInput {
		return fmt.Errorf("execution transaction with StateOutput.zk_proof must include at least one is_reference_only resource input")
	}

	return nil
}

// 编译期检查：确保 ExecResourceInvariantPlugin 实现了 ConditionPlugin 接口
var _ txiface.ConditionPlugin = (*ExecResourceInvariantPlugin)(nil)


