// Package tx provides transaction verifier interfaces.
package tx

import (
	"context"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// TxVerifier 交易验证器接口（验证微内核）
//
// 🎯 **核心职责**：三阶段验证（AuthZ + Conservation + Condition）
//
// 💡 **设计理念**：
// 采用"微内核 + 插件"架构，将验证逻辑模块化：
// - 微内核：提供三个验证钩子（AuthZ、Conservation、Condition），协调验证流程
// - 插件：具体的验证逻辑（7 种 AuthZ 插件、多种 Conservation 插件、Condition 插件）
//
// 🏗️ **验证流程**：
// 1. **AuthZ Hook（权限验证）**：最核心，验证 UnlockingProof 匹配 LockingCondition
// 2. **Conservation Hook（价值守恒）**：验证 Σ输入 ≥ Σ输出 + Fee
// 3. **Condition Hook（条件检查）**：验证时间锁、高度锁、nonce 等
//
// ⚠️ **核心约束**：
// - ❌ 验证无副作用：不能修改交易、不能消费 UTXO
// - ❌ 插件无状态：插件不能存储验证结果
// - ✅ 插件可并行：AuthZ 插件之间可以并行验证
//
// 📞 **调用方**：
// - TxProcessor: 提交前验证
// - SignedTx.Verify(): 用户主动验证
//
// 📝 **典型用法**：
//
//	// 1. 创建 Verifier 并注册插件
//	verifier := NewVerifier(utxoManager)
//	verifier.RegisterAuthZPlugin(singleKeyPlugin)
//	verifier.RegisterAuthZPlugin(multiKeyPlugin)
//	// ... 注册其他插件
//
//	// 2. 验证交易
//	err := verifier.Verify(ctx, tx)
//	if err != nil {
//	    // 验证失败
//	    return err
//	}
type TxVerifier interface {
	// Verify 三阶段验证
	//
	// 🎯 **验证流程**：
	//
	// 阶段 1：权限验证（AuthZ Hook）
	// - 对于每个 input，获取其引用的 UTXO
	// - 提取 UTXO 的 LockingCondition 和 input 的 UnlockingProof
	// - 遍历注册的 AuthZ 插件，找到匹配的插件进行验证
	// - 7 种插件：SingleKey、MultiKey、Contract、Delegation、Threshold、Time、Height
	// - ⚠️ 只要有一个 input 验证失败，整个交易失败
	//
	// 阶段 2：价值守恒（Conservation Hook）
	// - 计算输入总额：Σ(inputs.amount)（排除 is_reference_only 的 input）
	// - 计算输出总额：Σ(outputs.amount)
	// - 验证：输入总额 ≥ 输出总额 + Fee
	// - 支持多种费用模式：UTXO 差额、MinimumFee、ProportionalFee、ContractFee、PriorityFee
	//
	// 阶段 3：条件检查（Condition Hook）
	// - 验证时间锁：如果有 time_window，检查当前时间是否在窗口内
	// - 验证高度锁：如果有 height_window，检查当前高度是否在窗口内
	// - 验证 nonce：检查 nonce 是否正确（防重放）
	// - 验证链 ID：检查 chain_id 是否匹配（防跨链重放）
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - tx: 待验证的交易
	//
	// 返回：
	//   - error: 验证失败的原因
	//     • AuthZ 失败：权限验证失败
	//     • Conservation 失败：价值守恒失败
	//     • Condition 失败：条件检查失败
	//
	// ⚠️ 约束：
	// - 验证过程不能修改交易
	// - 验证过程不能消费 UTXO（UTXO 消费由区块确认后处理）
	// - 验证过程只能读取 UTXO 状态（通过 repository.UTXOManager）
	Verify(ctx context.Context, tx *transaction.Transaction) error

	// ==================== 🔌 插件注册接口 ====================

	// RegisterAuthZPlugin 注册权限验证插件
	//
	// 🎯 **用途**：注册 7 种权限验证插件
	//
	// 参数：
	//   - plugin: 权限验证插件（实现 AuthZPlugin 接口）
	//
	// 📝 **7 种插件**：
	// - SingleKeyPlugin: 单密钥验证
	// - MultiKeyPlugin: 多重签名验证
	// - ContractPlugin: 智能合约验证
	// - DelegationPlugin: 委托授权验证
	// - ThresholdPlugin: 门限签名验证
	// - TimePlugin: 时间锁验证（递归验证基础锁）
	// - HeightPlugin: 高度锁验证（递归验证基础锁）
	RegisterAuthZPlugin(plugin AuthZPlugin)

	// RegisterConservationPlugin 注册价值守恒插件
	//
	// 🎯 **用途**：注册价值守恒验证插件
	//
	// 参数：
	//   - plugin: 价值守恒插件（实现 ConservationPlugin 接口）
	//
	// 📝 **典型插件**：
	// - BasicConservationPlugin: 基础价值守恒（Σ输入 ≥ Σ输出）
	// - MinFeeConservationPlugin: 最低费用检查
	// - ProportionalFeePlugin: 比例费用检查
	RegisterConservationPlugin(plugin ConservationPlugin)

	// RegisterConditionPlugin 注册条件检查插件
	//
	// 🎯 **用途**：注册条件检查插件
	//
	// 参数：
	//   - plugin: 条件检查插件（实现 ConditionPlugin 接口）
	//
	// 📝 **典型插件**：
	// - TimeWindowPlugin: 时间窗口检查
	// - HeightWindowPlugin: 高度窗口检查
	// - NoncePlugin: nonce 检查
	// - ChainIDPlugin: 链 ID 检查
	RegisterConditionPlugin(plugin ConditionPlugin)
}

// ================================================================================================
// 🎯 接口设计说明
// ================================================================================================

// 设计权衡 1: 微内核 vs 单一验证函数
//
// 背景：交易验证包含多种规则，如何组织验证逻辑
//
// 备选方案：
// 1. 微内核 + 插件：内核提供钩子，逻辑在插件中 - 优势：可扩展 - 劣势：实现复杂
// 2. 单一验证函数：所有逻辑写在一起 - 优势：简单 - 劣势：难以扩展
//
// 选择：微内核 + 插件
//
// 理由：
// - 7 种权限验证 + 多种费用模式 + 条件检查，逻辑复杂
// - 插件化设计支持未来新增验证方式（如新的锁定机制）
// - 符合开闭原则（Open-Closed Principle）
// - 内核稳定，验证逻辑灵活扩展
//
// 代价：
// - 实现复杂度增加（需要设计插件接口和注册机制）
// - 但长期收益远大于短期成本

// 设计权衡 2: 三阶段验证顺序
//
// 背景：验证顺序是否重要
//
// 备选方案：
// 1. AuthZ → Conservation → Condition - 优势：权限优先 - 劣势：无
// 2. Conservation → AuthZ → Condition - 优势：快速失败 - 劣势：权限不优先
//
// 选择：AuthZ → Conservation → Condition
//
// 理由：
// - 权限验证是核心，必须优先检查（符合顶层设计"TX = 权限验证 + 状态转换"）
// - 没有权限就不应该执行任何操作，包括价值守恒检查
// - 符合安全优先原则
//
// 代价：
// - 即使余额不足，也要先验证权限（略慢）
// - 但安全性比性能更重要

// 设计权衡 3: 是否支持插件优先级
//
// 背景：多个插件时，是否需要控制执行顺序
//
// 备选方案：
// 1. 不支持：按注册顺序执行 - 优势：简单 - 劣势：不灵活
// 2. 支持：插件带优先级字段 - 优势：灵活 - 劣势：复杂
//
// 选择：不支持（v1.0）
//
// 理由：
// - 大部分场景不需要控制优先级
// - 插件应该无状态、可并行，执行顺序不影响结果
// - 保持 v1.0 简单，未来需要时再添加
//
// 代价：
// - 无法控制插件执行顺序
// - 但实际上也不需要

// ================================================================================================
// 🎯 使用示例
// ================================================================================================

// Example_Verification 展示如何使用 TxVerifier 验证交易
//
// 说明：此函数只是示例，不会被编译运行
func Example_Verification() {
	// var (
	// 	ctx       context.Context
	// 	verifier  TxVerifier
	// 	tx        *transaction.Transaction
	// )
	//
	// // 步骤 1：注册插件（通常在系统启动时完成）
	// verifier.RegisterAuthZPlugin(NewSingleKeyPlugin())
	// verifier.RegisterAuthZPlugin(NewMultiKeyPlugin())
	// verifier.RegisterAuthZPlugin(NewContractPlugin())
	// // ... 注册其他插件
	//
	// verifier.RegisterConservationPlugin(NewBasicConservationPlugin())
	// verifier.RegisterConservationPlugin(NewMinFeePlugin())
	//
	// verifier.RegisterConditionPlugin(NewTimeWindowPlugin())
	// verifier.RegisterConditionPlugin(NewNoncePlugin())
	//
	// // 步骤 2：验证交易
	// err := verifier.Verify(ctx, tx)
	// if err != nil {
	// 	// 验证失败
	// 	// 错误信息包含失败原因（AuthZ/Conservation/Condition）
	// 	return err
	// }
	//
	// // 验证通过，可以提交
}

// Example_PluginRegistration 展示插件注册的典型模式
//
// 说明：此函数只是示例，不会被编译运行
func Example_PluginRegistration() {
	// // 使用 fx 依赖注入框架注册插件
	// fx.Options(
	// 	// 提供 Verifier
	// 	fx.Provide(NewVerifier),
	//
	// 	// 提供所有 AuthZ 插件
	// 	fx.Provide(
	// 		fx.Annotate(
	// 			NewSingleKeyPlugin,
	// 			fx.As(new(AuthZPlugin)),
	// 			fx.ResultTags(`group:"authz_plugins"`),
	// 		),
	// 	),
	// 	fx.Provide(
	// 		fx.Annotate(
	// 			NewMultiKeyPlugin,
	// 			fx.As(new(AuthZPlugin)),
	// 			fx.ResultTags(`group:"authz_plugins"`),
	// 		),
	// 	),
	// 	// ... 其他插件
	//
	// 	// 注册插件到 Verifier
	// 	fx.Invoke(func(
	// 		verifier TxVerifier,
	// 		authzPlugins []AuthZPlugin `group:"authz_plugins"`,
	// 		conservationPlugins []ConservationPlugin `group:"conservation_plugins"`,
	// 		conditionPlugins []ConditionPlugin `group:"condition_plugins"`,
	// 	) {
	// 		// 注册所有插件
	// 		for _, plugin := range authzPlugins {
	// 			verifier.RegisterAuthZPlugin(plugin)
	// 		}
	// 		for _, plugin := range conservationPlugins {
	// 			verifier.RegisterConservationPlugin(plugin)
	// 		}
	// 		for _, plugin := range conditionPlugins {
	// 			verifier.RegisterConditionPlugin(plugin)
	// 		}
	// 	}),
	// )
}
