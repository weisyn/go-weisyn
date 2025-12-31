// Package block 提供区块处理的公共接口定义
//
// 🔄 **区块处理接口 (Block Processor)**
//
// 本包定义 WES 系统的区块处理接口，遵循 CQRS 架构原则，
// 专注于区块的处理操作。
//
// 🎯 **核心职责**：
// - 处理区块（执行交易、更新状态）
//
// 🏗️ **设计原则**：
// - CQRS 写路径：区块处理属于写操作
// - 事务保证：处理必须在事务中执行
// - 职责单一：只负责区块处理
//
// 详细使用说明请参考：pkg/interfaces/block/README.md
package block

import (
	"context"

	core "github.com/weisyn/v1/pb/blockchain/block"
)

// BlockProcessor 区块处理接口（写操作）
//
// 🎯 **核心职责**：
// 提供区块处理操作，执行区块中的交易并更新状态。
//
// 💡 **设计理念**：
// - 只包含处理操作，不包含验证（由 BlockValidator 提供）
// - 必须在事务中执行，确保原子性
// - 处理完成后更新链状态
//
// 📞 **调用方**：
// - SyncService：同步过程中处理区块
// - ConsensusService：矿工挖出区块后处理
//
// ⚠️ **核心约束**：
// - 事务保证：处理必须在事务中执行
// - 前置条件：区块必须已通过验证
// - 原子性：处理必须原子性完成
type BlockProcessor interface {
	// ProcessBlock 处理区块
	//
	// 执行区块中的交易，更新区块链状态，将区块添加到区块链中。
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - block: 已验证的区块
	//
	// 返回：
	//   - error: 处理错误，nil表示成功
	//
	// 使用场景：
	//   - 处理验证通过的新区块
	//   - 同步过程中应用历史区块
	//   - 分叉解决后重新应用区块
	//
	// 说明：
	//   - 区块必须已通过验证（调用 BlockValidator.ValidateBlock）
	//   - 处理必须在事务中执行
	//   - 处理完成后更新链尖状态
	ProcessBlock(ctx context.Context, block *core.Block) error
}
