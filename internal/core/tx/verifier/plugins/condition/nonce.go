// Package condition 提供条件验证插件实现
//
// nonce.go: Nonce 验证插件
package condition

import (
	"context"
	"fmt"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	txiface "github.com/weisyn/v1/pkg/interfaces/tx"
)

// NoncePlugin Nonce 验证插件
//
// 🎯 **核心职责**：验证交易的 nonce 是否正确（防重放攻击）
//
// 💡 **设计理念**：
// Nonce（Number used ONCE）用于防止交易重放攻击。每个账户维护一个 nonce 计数器，
// 交易的 nonce 必须等于账户当前 nonce + 1，验证通过后账户 nonce 递增。
//
// ⚠️ **验证规则**：
// 1. 如果交易未设置 nonce（nonce == 0），跳过验证（向后兼容或特殊交易）
// 2. 如果交易设置了 nonce，必须等于账户当前 nonce + 1
// 3. Nonce 验证需要 VerifierEnvironment 提供 nonce 查询能力
//
// 🔒 **核心约束**：
// - 插件无状态：不存储验证结果
// - 插件只读：不修改交易或账户 nonce（验证阶段）
// - 并发安全：多个 goroutine 可以同时调用
//
// 📞 **调用方**：Verifier Kernel（通过 Condition Hook）
type NoncePlugin struct {
	// 注意：Nonce查询需要通过 VerifierEnvironment 提供
	// 本插件不直接依赖账户状态存储，保持端口纯净
}

// NewNoncePlugin 创建新的 NoncePlugin
//
// 返回：
//   - *NoncePlugin: 新创建的实例
func NewNoncePlugin() *NoncePlugin {
	return &NoncePlugin{}
}

// Name 返回插件名称
//
// 实现 tx.ConditionPlugin 接口
//
// 返回：
//   - string: "nonce"
func (p *NoncePlugin) Name() string {
	return "nonce"
}

// Check 检查交易的 nonce
//
// 实现 tx.ConditionPlugin 接口
//
// 🎯 **核心逻辑**：
// 1. 检查交易是否设置了 nonce
// 2. 如果未设置（nonce == 0），跳过验证
// 3. 如果设置了，需要从 VerifierEnvironment 获取账户当前 nonce
// 4. 检查 tx.nonce 是否等于账户 nonce + 1
//
// ⚠️ **重要约束**：
// - 本插件需要 VerifierEnvironment 支持 nonce 查询
// - 如果无法获取账户 nonce，验证失败
// - Nonce 验证仅在验证阶段进行，实际递增由执行层处理
//
// 参数：
//   - ctx: 上下文对象（可能包含 VerifierEnvironment）
//   - tx: 待验证的交易
//   - blockHeight: 当前区块高度（本插件不使用）
//   - blockTime: 当前区块时间（本插件不使用）
//
// 返回：
//   - error: 验证失败的原因
//   - nil: 验证通过
//   - non-nil: nonce 不正确或无法验证
//
// 📝 **使用场景**：
//
//	// 用户首次交易（账户 nonce = 0）
//	tx.Nonce = 1  // 正确
//	err := plugin.Check(ctx, tx, 0, 0)  // nil（验证通过）
//
//	// 用户第二次交易（账户 nonce = 1）
//	tx.Nonce = 2  // 正确
//	err := plugin.Check(ctx, tx, 0, 0)  // nil（验证通过）
//
//	// 用户使用错误的 nonce（账户 nonce = 1）
//	tx.Nonce = 3  // 错误：跳过了 nonce 2
//	err := plugin.Check(ctx, tx, 0, 0)  // error（nonce 不正确）
//
//	// 用户重放旧交易（账户 nonce = 5）
//	tx.Nonce = 3  // 错误：nonce 已使用
//	err := plugin.Check(ctx, tx, 0, 0)  // error（nonce 过期）
func (p *NoncePlugin) Check(
	ctx context.Context,
	tx *transaction.Transaction,
	blockHeight uint64,
	blockTime uint64,
) error {
	// 1. 检查交易是否设置了 nonce
	if tx.Nonce == 0 {
		// 未设置 nonce，跳过验证（向后兼容或特殊交易如 Coinbase）
		return nil
	}

	// 2. 从 context 中获取 VerifierEnvironment
	env, ok := txiface.GetVerifierEnvironment(ctx)
	if !ok || env == nil {
		return fmt.Errorf("nonce 验证需要 VerifierEnvironment，但未提供")
	}

	// 3. 提取交易发起者地址（从第一个输入的 UTXO owner 获取）
	if len(tx.Inputs) == 0 {
		// 没有输入（如Coinbase），跳过nonce验证
		return nil
	}

	// 查询第一个输入的 UTXO
	utxo, err := env.GetUTXO(ctx, tx.Inputs[0].PreviousOutput)
	if err != nil {
		return fmt.Errorf("查询输入 UTXO 失败: %w", err)
	}
	senderAddress := utxo.OwnerAddress

	// 4. 查询账户当前 nonce
	currentNonce, err := env.GetNonce(ctx, senderAddress)
	if err != nil {
		return fmt.Errorf("查询账户 nonce 失败: %w", err)
	}

	// 5. 验证 tx.nonce == currentNonce + 1（严格递增）
	expectedNonce := currentNonce + 1
	if tx.Nonce != expectedNonce {
		return fmt.Errorf(
			"nonce 不正确: tx.nonce=%d, 期望=%d（账户当前nonce=%d）",
			tx.Nonce,
			expectedNonce,
			currentNonce,
		)
	}

	// 验证通过
	return nil
}

// 编译期检查：确保 NoncePlugin 实现了 txiface.ConditionPlugin 接口
var _ txiface.ConditionPlugin = (*NoncePlugin)(nil)
