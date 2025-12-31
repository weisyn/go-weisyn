// Package condition 提供条件验证插件实现
//
// chain_id.go: ChainID 验证插件
package condition

import (
	"bytes"
	"context"
	"fmt"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/tx"
)

// ChainIDPlugin ChainID 验证插件
//
// 🎯 **核心职责**：验证交易的 chain_id 是否匹配当前链
//
// 💡 **设计理念**：
// ChainID 用于防止跨链重放攻击。每条链有唯一的 chain_id，
// 交易在创建时必须包含目标链的 chain_id，验证时检查是否匹配。
//
// ⚠️ **验证规则**：
// 1. 如果交易未设置 chain_id，验证通过（向后兼容）
// 2. 如果交易设置了 chain_id，必须与当前链的 chain_id 匹配
// 3. chain_id 匹配使用字节比较（完全相等）
//
// 🔒 **核心约束**：
// - 插件无状态：不存储验证结果
// - 插件只读：不修改交易
// - 并发安全：多个 goroutine 可以同时调用
//
// 📞 **调用方**：Verifier Kernel（通过 Condition Hook）
type ChainIDPlugin struct {
	chainID []byte // 当前链的 chain_id
}

// NewChainIDPlugin 创建新的 ChainIDPlugin
//
// 参数：
//   - chainID: 当前链的 chain_id
//
// 返回：
//   - *ChainIDPlugin: 新创建的实例
func NewChainIDPlugin(chainID []byte) *ChainIDPlugin {
	return &ChainIDPlugin{
		chainID: chainID,
	}
}

// Name 返回插件名称
//
// 实现 tx.ConditionPlugin 接口
//
// 返回：
//   - string: "chain_id"
func (p *ChainIDPlugin) Name() string {
	return "chain_id"
}

// Check 检查交易的 chain_id
//
// 实现 tx.ConditionPlugin 接口
//
// 🎯 **核心逻辑**：
// 1. 检查交易是否设置了 chain_id
// 2. 如果未设置，验证通过（向后兼容）
// 3. 如果设置了，检查是否与当前链的 chain_id 匹配
//
// 参数：
//   - ctx: 上下文对象
//   - tx: 待验证的交易
//   - blockHeight: 当前区块高度（本插件不使用）
//   - blockTime: 当前区块时间（本插件不使用）
//
// 返回：
//   - error: 验证失败的原因
//   - nil: 验证通过
//   - non-nil: chain_id 不匹配
//
// 📝 **使用场景**：
//
//	// 交易设置了正确的 chain_id
//	tx.ChainId = []byte("weisyn-mainnet-v1")
//	// 验证时检查是否与当前链匹配
//	err := plugin.Check(ctx, tx, 0, 0)  // nil（验证通过）
//
//	// 交易设置了错误的 chain_id
//	tx.ChainId = []byte("other-chain-v1")
//	err := plugin.Check(ctx, tx, 0, 0)  // error（chain_id 不匹配）
func (p *ChainIDPlugin) Check(
	ctx context.Context,
	tx *transaction.Transaction,
	blockHeight uint64,
	blockTime uint64,
) error {
	// 1. 检查交易是否设置了 chain_id
	if len(tx.ChainId) == 0 {
		// 未设置 chain_id，验证通过（向后兼容）
		return nil
	}

	// 2. 检查当前链是否配置了 chain_id
	if len(p.chainID) == 0 {
		// 当前链未配置 chain_id，跳过验证
		return nil
	}

	// 3. 检查 chain_id 是否匹配
	if !bytes.Equal(tx.ChainId, p.chainID) {
		return fmt.Errorf(
			"chain_id 不匹配: tx.chain_id=%s, 当前链chain_id=%s",
			string(tx.ChainId),
			string(p.chainID),
		)
	}

	// 4. 验证通过
	return nil
}

// 编译期检查：确保 ChainIDPlugin 实现了 tx.ConditionPlugin 接口
var _ tx.ConditionPlugin = (*ChainIDPlugin)(nil)
