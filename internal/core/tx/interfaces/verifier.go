// Package interfaces provides transaction verifier interfaces.
package interfaces

import (
	"context"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/tx"
)

// Verifier 交易验证器内部接口（验证微内核）
//
// 🎯 **职责**：三阶段验证（AuthZ + Conservation + Condition）+ 插件管理
//
// 🔄 **继承关系**：
//   - 继承 tx.TxVerifier 公共接口（包含 Verify() 和三个 Register* 方法）
//   - 扩展内部专用方法（批量验证、带上下文验证等）
//
// 📁 **实现目录**：internal/core/tx/verifier/
//
// 💡 **设计说明**：
//   - 采用"微内核 + 插件"架构
//   - 内核提供三大验证钩子：AuthZ Hook、Conservation Hook、Condition Hook
//   - 验证插件通过 Register* 方法注册到对应钩子（继承自公共接口）
//   - 验证流程：AuthZ(权限) → Conservation(价值守恒) → Condition(条件检查)
//
// ⚠️ **核心约束**：
//   - 验证无副作用：不能修改交易、不能消费 UTXO
//   - 插件无状态：不能存储验证结果
//   - 插件可并行：AuthZ 插件之间可以并行验证
type Verifier interface {
	// ==================== 继承公共接口 ====================

	// 继承公共交易验证器接口
	// 包含：
	// - Verify(ctx, tx) error: 三阶段验证
	// - RegisterAuthZPlugin(plugin): 注册权限验证插件
	// - RegisterConservationPlugin(plugin): 注册价值守恒插件
	// - RegisterConditionPlugin(plugin): 注册条件检查插件
	tx.TxVerifier

	// ==================== 内部扩展方法 ====================

	// VerifyBatch 批量验证多个交易
	//
	// 🎯 **用途**：区块验证时批量验证交易列表
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - txs: 待验证的交易列表
	//
	// 返回：
	//   - []error: 每个交易的验证结果（nil表示通过，非nil表示失败）
	//   - error: 批量验证过程的整体错误（如内部错误）
	//
	// 💡 **优化**：
	//   - 支持并发验证（AuthZ插件之间可并行）
	//   - 提前失败：某个交易验证失败时可选择继续或停止
	VerifyBatch(ctx context.Context, txs []*transaction.Transaction) ([]error, error)

	// VerifyWithContext 带上下文的验证
	//
	// 🎯 **用途**：在特定场景下验证交易（如区块验证、创世验证）
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - tx: 待验证的交易
	//   - validationCtx: 验证上下文（指定场景和选项）
	//
	// 返回：
	//   - error: 验证失败的原因
	//
	// 💡 **使用场景**：
	//   - 区块验证：跳过某些检查（如nonce已由区块验证）
	//   - 创世验证：允许特殊交易（如无输入的Coinbase）
	//   - 缓存验证：跳过已验证的交易
	VerifyWithContext(ctx context.Context, tx *transaction.Transaction, validationCtx interface{}) error

	// VerifyAuthZLock 验证单个锁定条件（用于递归验证）
	//
	// 🎯 **用途**：供 TimeLock/HeightLock 插件递归验证 base_lock
	//
	// 💡 **设计理念**：
	//   - TimeLock 和 HeightLock 包含 base_lock 字段
	//   - 验证时需要递归验证 base_lock 是否满足
	//   - 本方法提供独立的 lock + proof 验证能力
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - lock: 锁定条件（通常是 base_lock）
	//   - proof: 解锁证明（通常是 base_proof）
	//   - tx: 完整的交易对象
	//
	// 返回：
	//   - error: 验证失败的原因，nil表示验证通过
	//
	// 📝 **使用场景**：
	//
	//	// TimeLockPlugin 递归验证 base_lock
	//	if err := verifier.VerifyAuthZLock(ctx, timeLock.BaseLock, timeProof.BaseProof, tx); err != nil {
	//	    return true, fmt.Errorf("base_lock verification failed: %w", err)
	//	}
	VerifyAuthZLock(ctx context.Context, lock *transaction.LockingCondition, proof *transaction.UnlockingProof, tx *transaction.Transaction) error
}
