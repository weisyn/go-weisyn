// Package verifier 提供交易验证微内核和钩子实现
//
// conservation_hook.go: 价值守恒验证钩子（Conservation Hook）
package verifier

import (
	"context"
	"fmt"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxopb "github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/interfaces/tx"
)

// ConservationHook 价值守恒验证钩子
//
// 🎯 **核心职责**：管理 Conservation 插件注册和调用
//
// 💡 **设计理念**：
// Conservation Hook 遍历所有已注册的 Conservation 插件，对交易的价值守恒进行验证。
// 所有插件都必须通过验证，交易才能被认为符合价值守恒规则。
//
// ⚠️ **核心约束**：
// - 所有已注册的插件都必须通过验证
// - 插件按注册顺序执行
// - 任何一个插件验证失败，整个验证失败
//
// 📞 **调用方**：Verifier Kernel
type ConservationHook struct {
	plugins []tx.ConservationPlugin
	eutxoQuery persistence.UTXOQuery
}

// NewConservationHook 创建新的 ConservationHook
//
// 参数：
//   - eutxoQuery: UTXO 管理器（用于查询输入引用的 UTXO）
//
// 返回：
//   - *ConservationHook: 新创建的实例
func NewConservationHook(eutxoQuery persistence.UTXOQuery) *ConservationHook {
	return &ConservationHook{
		plugins: make([]tx.ConservationPlugin, 0),
		eutxoQuery: eutxoQuery,
	}
}

// Register 注册 Conservation 插件
//
// 参数：
//   - plugin: 待注册的 Conservation 插件
func (h *ConservationHook) Register(plugin tx.ConservationPlugin) {
	h.plugins = append(h.plugins, plugin)
}

// Verify 验证交易的价值守恒
//
// 🎯 **核心逻辑**：
// 1. 查询所有输入引用的 UTXO
// 2. 遍历所有已注册的插件
// 3. 每个插件都必须通过验证
//
// 参数：
//   - ctx: 上下文对象
//   - tx: 待验证的交易
//
// 返回：
//   - error: 验证失败的原因
//   - nil: 所有插件的价值守恒验证通过
//   - non-nil: 某个插件的价值守恒验证失败
func (h *ConservationHook) Verify(ctx context.Context, tx *transaction.Transaction) error {
	// 1. 查询所有输入引用的 UTXO
	inputs, err := h.fetchInputUTXOs(ctx, tx)
	if err != nil {
		return fmt.Errorf("查询输入 UTXO 失败: %w", err)
	}

	// 2. 遍历所有插件，每个都必须通过
	for _, plugin := range h.plugins {
		if err := plugin.Check(ctx, inputs, tx.Outputs, tx); err != nil {
			return fmt.Errorf("插件 %s 验证失败: %w", plugin.Name(), err)
		}
	}

	return nil
}

// fetchInputUTXOs 查询所有输入引用的 UTXO
//
// 参数：
//   - ctx: 上下文对象
//   - tx: 交易对象
//
// 返回：
//   - []*utxopb.UTXO: 输入 UTXO 列表
//   - error: 查询失败
func (h *ConservationHook) fetchInputUTXOs(
	ctx context.Context,
	tx *transaction.Transaction,
) ([]*utxopb.UTXO, error) {
	inputs := make([]*utxopb.UTXO, len(tx.Inputs))

	for i, input := range tx.Inputs {
		utxo, err := h.eutxoQuery.GetUTXO(ctx, input.PreviousOutput)
		if err != nil {
			return nil, fmt.Errorf("输入 %d: 获取 UTXO 失败: %w", i, err)
		}
		inputs[i] = utxo
	}

	return inputs, nil
}
