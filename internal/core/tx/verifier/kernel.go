// Package verifier 提供交易验证微内核实现
//
// kernel.go: 验证微内核（Verifier Kernel）
package verifier

import (
	"context"
	"fmt"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	txiface "github.com/weisyn/v1/pkg/interfaces/tx"
)

// Kernel 验证微内核
//
// 🎯 **核心职责**：协调三个验证钩子（AuthZ、Conservation、Condition）按顺序执行
//
// 💡 **设计理念**：
// Verifier Kernel 是验证微内核的核心组件，负责按照固定顺序调用三个验证钩子：
// 1. AuthZ Hook：权限验证（UnlockingProof 是否匹配 LockingCondition）
// 2. Conservation Hook：价值守恒验证（Σ输入 ≥ Σ输出）
// 3. Condition Hook：条件检查（时间锁、高度锁等）
//
// ⚠️ **核心约束**：
// - 三个钩子必须按顺序执行（AuthZ → Conservation → Condition）
// - 任何一个钩子验证失败，整个验证失败
// - 验证过程无副作用（不修改交易、不消费 UTXO）
//
// 📞 **调用方**：Processor（通过 interfaces.Verifier 接口）
type Kernel struct {
	authzHook        *AuthZHook
	conservationHook *ConservationHook
	conditionHook    *ConditionHook
}

// NewKernel 创建新的 Verifier Kernel
//
// 参数：
//   - eutxoQuery: UTXO 管理器（用于查询输入引用的 UTXO）
//
// 返回：
//   - *Kernel: 新创建的实例
func NewKernel(eutxoQuery persistence.UTXOQuery) *Kernel {
	return &Kernel{
		authzHook:        NewAuthZHook(eutxoQuery),
		conservationHook: NewConservationHook(eutxoQuery),
		conditionHook:    NewConditionHook(),
	}
}

// Verify 验证交易
//
// 实现 interfaces.Verifier 接口
//
// 🎯 **核心逻辑**：
// 1. AuthZ 验证：权限验证
// 2. Conservation 验证：价值守恒验证
// 3. Condition 验证：条件检查
//
// 参数：
//   - ctx: 上下文对象
//   - tx: 待验证的交易
//
// 返回：
//   - error: 验证失败的原因
//   - nil: 验证通过
//   - non-nil: 验证失败，描述失败原因
func (k *Kernel) Verify(ctx context.Context, tx *transaction.Transaction) error {
	// 1. 权限验证（AuthZ）
	if err := k.authzHook.Verify(ctx, tx); err != nil {
		return fmt.Errorf("权限验证失败: %w", err)
	}

	// 2. 价值守恒验证（Conservation）
	if err := k.conservationHook.Verify(ctx, tx); err != nil {
		return fmt.Errorf("价值守恒验证失败: %w", err)
	}

	// 3. 条件检查（Condition）
	// 注意：P1 阶段暂时使用 0 作为 blockHeight 和 blockTime
	// 后续阶段将从区块链状态获取实际值
	if err := k.conditionHook.Verify(ctx, tx, 0, 0); err != nil {
		return fmt.Errorf("条件检查失败: %w", err)
	}

	return nil
}

// RegisterAuthZPlugin 注册 AuthZ 插件
//
// 实现 interfaces.Verifier 接口
//
// 参数：
//   - plugin: 待注册的 AuthZ 插件
func (k *Kernel) RegisterAuthZPlugin(plugin txiface.AuthZPlugin) {
	k.authzHook.Register(plugin)
}

// RegisterConservationPlugin 注册 Conservation 插件
//
// 实现 interfaces.Verifier 接口
//
// 参数：
//   - plugin: 待注册的 Conservation 插件
func (k *Kernel) RegisterConservationPlugin(plugin txiface.ConservationPlugin) {
	k.conservationHook.Register(plugin)
}

// RegisterConditionPlugin 注册 Condition 插件
//
// 实现 interfaces.Verifier 接口
//
// 参数：
//   - plugin: 待注册的 Condition 插件
func (k *Kernel) RegisterConditionPlugin(plugin txiface.ConditionPlugin) {
	k.conditionHook.Register(plugin)
}

// VerifyAuthZLock 验证单个锁定条件（用于递归验证）
//
// 🎯 **核心职责**：供 TimeLock/HeightLock 插件递归验证 base_lock
//
// 💡 **设计理念**：
// TimeLock 和 HeightLock 包含 base_lock 字段，验证时需要递归验证 base_lock。
// 本方法提供独立的 lock + proof 验证能力，避免重复实现验证逻辑。
//
// 🎯 **核心逻辑**：
// 1. 遍历所有已注册的 AuthZ 插件
// 2. 找到匹配 lock 类型的插件
// 3. 调用插件的 Match 方法验证
//
// 参数：
//   - ctx: 上下文对象
//   - lock: 锁定条件（通常是 base_lock）
//   - proof: 解锁证明（通常是 base_proof）
//   - tx: 完整的交易对象
//
// 返回：
//   - error: 验证失败的原因
//   - nil: 验证通过
//
// 📝 **使用场景**：
//
//	// TimeLockPlugin 递归验证 base_lock
//	err := verifier.VerifyAuthZLock(ctx, timeLock.BaseLock, timeProof.BaseProof, tx)
func (k *Kernel) VerifyAuthZLock(
	ctx context.Context,
	lock *transaction.LockingCondition,
	proof *transaction.UnlockingProof,
	tx *transaction.Transaction,
) error {
	// 遍历所有已注册的 AuthZ 插件，尝试匹配和验证
	matched := false
	var lastErr error

	for _, plugin := range k.authzHook.plugins {
		ok, err := plugin.Match(ctx, lock, proof, tx)
		if err != nil {
			// 插件匹配但验证失败
			lastErr = fmt.Errorf("插件 %s 验证失败: %w", plugin.Name(), err)
			if ok {
				// 如果匹配但失败，直接返回错误（不再尝试其他插件）
				return lastErr
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
			return fmt.Errorf("所有 AuthZ 插件都未匹配或验证失败: %w", lastErr)
		}
		return fmt.Errorf("没有 AuthZ 插件匹配此锁定条件类型")
	}

	return nil
}

// VerifyBatch 批量验证多个交易
//
// 实现 interfaces.Verifier 接口
//
// 🎯 **用途**：区块验证时批量验证交易列表
//
// 参数：
//   - ctx: 上下文对象
//   - txs: 待验证的交易列表
//
// 返回：
//   - []error: 每个交易的验证结果（nil表示通过）
//   - error: 批量验证过程的整体错误
func (k *Kernel) VerifyBatch(ctx context.Context, txs []*transaction.Transaction) ([]error, error) {
	results := make([]error, len(txs))
	for i, tx := range txs {
		results[i] = k.Verify(ctx, tx)
	}
	return results, nil
}

// VerifyWithContext 带环境的验证
//
// 实现 interfaces.Verifier 接口
//
// 🎯 **用途**：在特定环境下验证交易（提供区块高度、时间等环境信息）
//
// 💡 **设计理念**：
// validationCtx 应该是 txiface.VerifierEnvironment 类型，提供验证所需的环境信息：
// - 区块高度（用于 HeightLock 验证）
// - 区块时间（用于 TimeLock 验证）
// - 链ID（用于防跨链重放攻击）
// - Nonce查询（用于防重放攻击）
//
// 参数：
//   - ctx: 上下文对象
//   - tx: 待验证的交易
//   - validationCtx: 验证环境（应为 txiface.VerifierEnvironment 类型）
//
// 返回：
//   - error: 验证失败的原因
//
// 📝 **使用示例**：
//
//	env := txiface.NewStaticVerifierEnvironment(blockHeight, blockTime, chainID)
//	err := verifier.VerifyWithContext(ctx, tx, env)
func (k *Kernel) VerifyWithContext(
	ctx context.Context,
	tx *transaction.Transaction,
	validationCtx interface{},
) error {
	// 1. 权限验证（AuthZ）- 不需要环境信息
	if err := k.authzHook.Verify(ctx, tx); err != nil {
		return fmt.Errorf("权限验证失败: %w", err)
	}

	// 2. 价值守恒验证（Conservation）- 不需要环境信息
	if err := k.conservationHook.Verify(ctx, tx); err != nil {
		return fmt.Errorf("价值守恒验证失败: %w", err)
	}

	// 3. 条件检查（Condition）- 需要环境信息
	//   将 validationCtx 转换为 VerifierEnvironment 并注入context
	var blockHeight, blockTime uint64 = 0, 0
	if env, ok := validationCtx.(txiface.VerifierEnvironment); ok && env != nil {
		// 将 VerifierEnvironment 注入 context，供所有插件使用
		ctx = txiface.WithVerifierEnvironment(ctx, env)
		
		// 提取区块高度和时间（用于Condition Hook）
		blockHeight = env.GetBlockHeight()
		blockTime = env.GetBlockTime()
	}

	if err := k.conditionHook.Verify(ctx, tx, blockHeight, blockTime); err != nil {
		return fmt.Errorf("条件检查失败: %w", err)
	}

	return nil
}
