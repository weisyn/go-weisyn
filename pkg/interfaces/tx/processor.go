// Package tx 提供交易处理的公共接口定义
//
// 🎯 **接口定位**
//
// 本包定义 WES 交易处理系统的公共接口，供 ISPC、BLOCKCHAIN、CLI 等上层组件调用。
// 遵循"接口隔离原则"和"依赖反转原则"，让核心域依赖接口而非具体实现。
//
// 📋 **核心接口**：
// - TxProcessor: 对外统一入口（协调 Builder + Verifier + TxPool）
// - TxBuilder: Type-state 构建器（纯装配器）
// - TxVerifier: 验证微内核（插件化验证）
// - AuthZPlugin/ConservationPlugin/ConditionPlugin: 验证插件
// - Signer/FeeEstimator/ProofProvider: 端口接口
//
// ⚠️ **架构约束**：
// - ❌ 接口层不定义数据结构（数据结构在 pkg/types）
// - ❌ 接口层不定义错误类型（错误类型在 pkg/types 或直接返回 error）
// - ✅ 接口层只定义方法签名和职责约定
package tx

import (
	"context"

	"github.com/weisyn/v1/pkg/types"
)

// TxProcessor 交易处理协调器接口
//
// 🎯 **核心职责**：对外统一入口，协调所有交易处理组件
//
// 💡 **设计理念**：
// 将复杂的交易处理流程（构建、验证、提交、广播）封装为简单的对外接口。
// 使用方（ISPC、BLOCKCHAIN、CLI）只需要调用 SubmitTx，无需关心内部细节。
//
// 📞 **调用方**：
// - ISPC: 智能合约执行后提交交易
// - BLOCKCHAIN: 区块处理时提交 Coinbase 等特殊交易
// - CLI/API: 用户通过钱包/命令行提交交易
//
// 🔄 **内部协调**（对使用方透明）：
// 1. 调用 TxVerifier 进行三阶段验证
// 2. 验证通过后调用 mempool.TxPool.SubmitTx
// 3. TxPool 内部自动广播到网络
//
// ⚠️ **核心约束**：
// - 必须先验证后提交（不允许跳过验证）
// - 验证失败不入池（立即返回错误）
// - 验证通过后入池，TxPool 内部处理广播
//
// 📝 **典型用法**：
//
//	// 1. 构建交易
//	composed := builder.AddInput(...).AddOutput(...).Build()
//	proven := composed.WithProofs(ctx, proofProvider)
//	signed := proven.Sign(ctx, signer)
//
//	// 2. 提交交易
//	submitted, err := processor.SubmitTx(ctx, signed)
//	if err != nil {
//	    // 验证失败或入池失败
//	    return err
//	}
//
//	// 3. 查询状态
//	status, err := processor.GetTxStatus(ctx, submitted.GetTxHash())
type TxProcessor interface {
	// SubmitTx 提交交易到系统（验证 + 入池）
	//
	// 🎯 **核心流程**：
	// 1. 验证交易（AuthZ + Conservation + Condition）
	// 2. 验证通过后提交到 TxPool
	// 3. TxPool 内部自动广播到网络
	//
	// 参数：
	//   - ctx: 上下文对象（支持超时和取消）
	//   - signedTx: 已签名的交易（Type-state 保证）
	//
	// 返回：
	//   - *types.SubmittedTx: 已提交的交易（包含 txHash）
	//   - error: 验证失败或入池失败
	//
	// ⚠️ 前置条件：
	// - signedTx 必须是通过 Type-state 构建的（保证有 proof 和 signature）
	// - signedTx 内容不能为空
	//
	// ⚠️ 后置保证：
	// - 返回成功表示交易已在 TxPool 中
	// - TxPool 会自动广播到网络（无需调用方关心）
	//
	// 📝 **错误类型**：
	// - 验证失败：权限验证失败、余额不足、条件不满足等
	// - 入池失败：TxPool 已满、重复交易、nonce 错误等
	// - 网络错误：上下文超时、取消等
	SubmitTx(ctx context.Context, signedTx *types.SignedTx) (*types.SubmittedTx, error)

	// GetTxStatus 查询交易状态
	//
	// 🎯 **用途**：查询交易的广播和确认状态
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - txHash: 交易哈希（全局唯一标识）
	//
	// 返回：
	//   - *types.TxBroadcastState: 交易广播和确认状态
	//   - error: 交易不存在或查询失败
	//
	// 📝 **状态说明**：
	// - LocalSubmitted: 已入池，等待广播
	// - Broadcasted: 已广播到网络
	// - Confirmed: 已被区块收录
	// - BroadcastFailed: 广播失败
	// - Expired: 已过期
	GetTxStatus(ctx context.Context, txHash []byte) (*types.TxBroadcastState, error)
}

// ================================================================================================
// 🎯 接口设计说明
// ================================================================================================

// 设计权衡 1: TxProcessor 接口粒度
//
// 背景：需要为上层组件提供交易处理接口
//
// 备选方案：
// 1. 粗粒度接口（SubmitTx）：一个方法完成所有事情 - 优势：简单易用 - 劣势：灵活性低
// 2. 细粒度接口（Validate/Submit/Broadcast）：分步骤 - 优势：灵活 - 劣势：复杂
//
// 选择：粗粒度接口
//
// 理由：
// - 上层组件（ISPC、BLOCKCHAIN）只需要"提交交易"这一个核心功能
// - 验证、入池、广播是内部实现细节，无需暴露
// - 简化使用方的代码，降低出错概率
//
// 代价：
// - 无法单独调用验证或广播（但实际上也不需要）
// - 如果需要更细粒度的控制，可以通过 TxVerifier 接口实现

// 设计权衡 2: 是否暴露 TxPool 接口
//
// 背景：TxProcessor 内部需要调用 mempool.TxPool
//
// 备选方案：
// 1. 不暴露：TxProcessor 通过依赖注入使用 TxPool - 优势：接口简洁 - 劣势：无法直接操作池
// 2. 暴露：将 TxPool 接口嵌入 TxProcessor - 优势：灵活 - 劣势：接口复杂
//
// 选择：不暴露
//
// 理由：
// - TxPool 是实现细节，上层组件不应该直接操作
// - mempool.TxPool 已有完整的接口定义，不需要在 TX 层重复
// - 保持接口简洁，遵循"接口隔离原则"
//
// 代价：
// - 如果需要直接操作 TxPool（如查询所有待处理交易），需要直接依赖 mempool 包
// - 但这种场景很少，通常只在系统管理和监控时才需要

// ================================================================================================
// 🎯 使用示例
// ================================================================================================

// Example_SubmitTransfer 展示如何使用 TxProcessor 提交转账交易
//
// 说明：此函数只是示例，不会被编译运行
func Example_SubmitTransfer() {
	// var (
	// 	ctx       context.Context
	// 	processor TxProcessor
	// 	builder   TxBuilder
	// 	signer    Signer
	// 	provider  ProofProvider
	// )
	//
	// // 步骤 1：构建交易
	// composed := builder.
	// 	AddInput(outpoint1, false).
	// 	AddAssetOutput(bob, 100, nil, lock).
	// 	Build()
	//
	// // 步骤 2：添加证明
	// proven, err := composed.WithProofs(ctx, provider)
	// if err != nil {
	// 	panic(err)
	// }
	//
	// // 步骤 3：签名
	// signed, err := proven.Sign(ctx, signer)
	// if err != nil {
	// 	panic(err)
	// }
	//
	// // 步骤 4：提交（验证 + 入池）
	// submitted, err := processor.SubmitTx(ctx, signed)
	// if err != nil {
	// 	// 验证失败或入池失败
	// 	panic(err)
	// }
	//
	// // 步骤 5：查询状态
	// status, err := processor.GetTxStatus(ctx, submitted.GetTxHash())
	// if err != nil {
	// 	panic(err)
	// }
	// _ = status
}
