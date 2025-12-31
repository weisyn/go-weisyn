// Package verifier 提供交易验证微内核和钩子实现
//
// authz_hook.go: 权限验证钩子（AuthZ Hook）
package verifier

import (
	"context"
	"fmt"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/interfaces/tx"
)

// AuthZHook 权限验证钩子
//
// 🎯 **核心职责**：管理 AuthZ 插件注册和调用
//
// 💡 **设计理念**：
// AuthZ Hook 遍历所有已注册的 AuthZ 插件，对每个输入的解锁证明进行验证。
// 插件采用"尝试匹配"模式：插件返回 (true, nil) 表示匹配且验证通过，
// 返回 (false, nil) 表示不匹配，让其他插件尝试。
//
// ⚠️ **核心约束**：
// - 对每个输入，至少有一个插件必须匹配并通过验证
// - 插件按注册顺序尝试
// - 一旦某个插件匹配并通过，停止尝试其他插件
//
// 📞 **调用方**：Verifier Kernel
type AuthZHook struct {
	plugins []tx.AuthZPlugin
	eutxoQuery persistence.UTXOQuery
}

// NewAuthZHook 创建新的 AuthZHook
//
// 参数：
//   - eutxoQuery: UTXO 管理器（用于查询输入引用的 UTXO）
//
// 返回：
//   - *AuthZHook: 新创建的实例
func NewAuthZHook(eutxoQuery persistence.UTXOQuery) *AuthZHook {
	return &AuthZHook{
		plugins: make([]tx.AuthZPlugin, 0),
		eutxoQuery: eutxoQuery,
	}
}

// Register 注册 AuthZ 插件
//
// 参数：
//   - plugin: 待注册的 AuthZ 插件
func (h *AuthZHook) Register(plugin tx.AuthZPlugin) {
	h.plugins = append(h.plugins, plugin)
}

// Verify 验证交易的权限
//
// 🎯 **核心逻辑**：
// 1. 遍历交易的所有输入
// 2. 对每个输入，获取其引用的 UTXO
// 3. 提取 UTXO 的 LockingCondition 和输入的 UnlockingProof
// 4. 遍历所有已注册的插件，尝试匹配和验证
// 5. 至少有一个插件必须匹配并通过验证
//
// 参数：
//   - ctx: 上下文对象
//   - tx: 待验证的交易
//
// 返回：
//   - error: 验证失败的原因
//   - nil: 所有输入的权限验证通过
//   - non-nil: 某个输入的权限验证失败
func (h *AuthZHook) Verify(ctx context.Context, tx *transaction.Transaction) error {
	// 1. 遍历交易的所有输入
	for i, input := range tx.Inputs {
		// 2. 获取输入引用的 UTXO
		utxo, err := h.eutxoQuery.GetUTXO(ctx, input.PreviousOutput)
		if err != nil {
			return fmt.Errorf("输入 %d: 获取 UTXO 失败: %w", i, err)
		}

		// 3. 提取 TxOutput（使用 CachedOutput）
		txOutput := utxo.GetCachedOutput()
		if txOutput == nil {
			return fmt.Errorf("输入 %d: UTXO 没有缓存的 TxOutput", i)
		}
		if len(txOutput.LockingConditions) == 0 {
			return fmt.Errorf("输入 %d: TxOutput 没有任何锁定条件", i)
		}

		// 4. 获取第一个锁定条件（P1 只处理单条件）
		lockingCondition := txOutput.LockingConditions[0]

		// 5. 构建 UnlockingProof（从输入的 UnlockingProof 字段）
		// 注意：input.UnlockingProof 是 isTxInput_UnlockingProof 接口类型
		// 需要转换为 *transaction.UnlockingProof 以便插件使用
		unlockingProof := h.buildUnlockingProof(input)

		// 6. 遍历所有插件，尝试匹配和验证
		matched := false
		var lastErr error

		for _, plugin := range h.plugins {
			ok, err := plugin.Match(ctx, lockingCondition, unlockingProof, tx)
			if err != nil {
				// 插件匹配但验证失败
				lastErr = fmt.Errorf("插件 %s 验证失败: %w", plugin.Name(), err)
				if ok {
					// 如果匹配但失败，直接返回错误（不再尝试其他插件）
					return fmt.Errorf("输入 %d: %w", i, lastErr)
				}
				// 如果不匹配，继续尝试下一个插件
				continue
			}
			if ok {
				// 匹配且验证通过
				matched = true
				break
			}
		}

		if !matched {
			if lastErr != nil {
				return fmt.Errorf("输入 %d: 所有 AuthZ 插件都未匹配或验证失败: %w", i, lastErr)
			}
			return fmt.Errorf("输入 %d: 没有 AuthZ 插件匹配此锁定条件类型", i)
		}
	}

	return nil
}

// buildUnlockingProof 从 TxInput 的 UnlockingProof 字段构建 UnlockingProof
//
// 🎯 **核心职责**：将 TxInput.UnlockingProof (oneof) 转换为 UnlockingProof 消息
//
// 💡 **设计理念**：
// Hook 层负责 Proof 提取和类型转换,Plugin 层只负责验证逻辑。
// 这保持了插件接口的纯净和稳定性。
//
// ⚠️ **P2 扩展**：
// 新增支持 TimeProof 和 HeightProof,使 TimeLock/HeightLock 插件可以正常工作。
//
// 参数：
//   - input: TxInput
//
// 返回：
//   - *transaction.UnlockingProof: 构建的 UnlockingProof
func (h *AuthZHook) buildUnlockingProof(input *transaction.TxInput) *transaction.UnlockingProof {
	// 根据 input.UnlockingProof 的类型构建 UnlockingProof
	switch proof := input.UnlockingProof.(type) {
	case *transaction.TxInput_SingleKeyProof:
		return &transaction.UnlockingProof{
			Proof: &transaction.UnlockingProof_SingleKeyProof{
				SingleKeyProof: proof.SingleKeyProof,
			},
		}
	case *transaction.TxInput_MultiKeyProof:
		return &transaction.UnlockingProof{
			Proof: &transaction.UnlockingProof_MultiKeyProof{
				MultiKeyProof: proof.MultiKeyProof,
			},
		}
	case *transaction.TxInput_ExecutionProof:
		return &transaction.UnlockingProof{
			Proof: &transaction.UnlockingProof_ExecutionProof{
				ExecutionProof: proof.ExecutionProof,
			},
		}
	case *transaction.TxInput_DelegationProof:
		return &transaction.UnlockingProof{
			Proof: &transaction.UnlockingProof_DelegationProof{
				DelegationProof: proof.DelegationProof,
			},
		}
	case *transaction.TxInput_ThresholdProof:
		return &transaction.UnlockingProof{
			Proof: &transaction.UnlockingProof_ThresholdProof{
				ThresholdProof: proof.ThresholdProof,
			},
		}
	case *transaction.TxInput_TimeProof:
		// P2 新增：TimeLock 特殊处理
		// TimeProof 包含 base_proof (UnlockingProof),直接返回 base_proof 用于验证
		// TimeLock 插件会从 TxInput 中提取完整的 TimeProof 进行时间验证
		return proof.TimeProof.BaseProof

	case *transaction.TxInput_HeightProof:
		// P2 新增：HeightLock 特殊处理
		// HeightProof 包含 base_proof (UnlockingProof),直接返回 base_proof 用于验证
		// HeightLock 插件会从 TxInput 中提取完整的 HeightProof 进行高度验证
		return proof.HeightProof.BaseProof

	default:
		// 未知类型，返回空的 UnlockingProof
		return &transaction.UnlockingProof{}
	}
}
