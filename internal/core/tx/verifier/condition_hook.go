// Package verifier 提供交易验证微内核和钩子实现
//
// condition_hook.go: 条件检查验证钩子（Condition Hook）
package verifier

import (
	"context"
	"fmt"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/tx"
)

// ConditionHook 条件检查验证钩子
//
// 🎯 **核心职责**：管理 Condition 插件注册和调用
//
// 💡 **设计理念**：
// Condition Hook 遍历所有已注册的 Condition 插件，对交易的条件（时间锁、高度锁、nonce 等）进行检查。
// 所有插件都必须通过验证，交易才能被认为符合条件要求。
//
// ⚠️ **核心约束**：
// - 所有已注册的插件都必须通过验证
// - 插件按注册顺序执行
// - 任何一个插件验证失败，整个验证失败
//
// 📞 **调用方**：Verifier Kernel
type ConditionHook struct {
	plugins []tx.ConditionPlugin
}

// NewConditionHook 创建新的 ConditionHook
//
// 返回：
//   - *ConditionHook: 新创建的实例
func NewConditionHook() *ConditionHook {
	return &ConditionHook{
		plugins: make([]tx.ConditionPlugin, 0),
	}
}

// Register 注册 Condition 插件
//
// 参数：
//   - plugin: 待注册的 Condition 插件
func (h *ConditionHook) Register(plugin tx.ConditionPlugin) {
	h.plugins = append(h.plugins, plugin)
}

// Verify 验证交易的条件
//
// 🎯 **核心逻辑**：
// 1. 遍历所有已注册的插件
// 2. 每个插件都必须通过验证
//
// 参数：
//   - ctx: 上下文对象
//   - tx: 待验证的交易
//   - blockHeight: 当前区块高度（用于高度锁验证）
//   - blockTime: 当前区块时间（用于时间锁验证）
//
// 返回：
//   - error: 验证失败的原因
//   - nil: 所有插件的条件检查通过
//   - non-nil: 某个插件的条件检查失败
func (h *ConditionHook) Verify(
	ctx context.Context,
	tx *transaction.Transaction,
	blockHeight uint64,
	blockTime uint64,
) error {
	// 遍历所有插件，每个都必须通过
	for _, plugin := range h.plugins {
		if err := plugin.Check(ctx, tx, blockHeight, blockTime); err != nil {
			return fmt.Errorf("插件 %s 验证失败: %w", plugin.Name(), err)
		}
	}

	return nil
}
