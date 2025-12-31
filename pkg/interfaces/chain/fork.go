// Package chain 提供分叉处理的公共接口定义
//
// 🔀 **分叉处理接口 (Fork Handler)**
//
// 本包定义 WES 系统的分叉处理接口，处理区块链分叉和链重组场景。
//
// 🎯 **核心职责**：
// - 处理分叉情况
// - 获取当前活跃链
//
// 🏗️ **设计原则**：
// - CQRS 写路径：分叉处理涉及状态修改
// - 事务保证：分叉处理必须在事务中执行
// - 原子性：链重组必须原子性执行
//
// 详细使用说明请参考：pkg/interfaces/chain/README.md
package chain

import (
	"context"

	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/types"
)

// ForkHandler 分叉处理接口（写操作）
//
// 🎯 **核心职责**：
// 处理区块链分叉和链重组场景。
//
// 💡 **设计理念**：
// - 分叉处理涉及状态修改，属于写操作
// - 必须在事务中执行，确保原子性
// - 支持最长链原则和链切换
//
// 📞 **调用方**：
// - BlockProcessor：检测到分叉时调用
// - SyncService：同步过程中发现分叉时调用
//
// ⚠️ **核心约束**：
// - 事务保证：所有操作必须在事务中执行
// - 原子性：链重组必须原子性完成
// - 一致性：分叉处理后状态必须一致
type ForkHandler interface {
	// HandleFork 处理分叉情况
	//
	// 当检测到分叉时，处理分叉情况。
	// 根据最长链原则决定是否切换链。
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - forkBlock: 导致分叉的区块
	//
	// 返回：
	//   - error: 处理错误，nil表示成功
	//
	// 使用场景：
	//   - 接收到分叉区块时
	//   - 同步过程中发现分叉时
	HandleFork(ctx context.Context, forkBlock *core.Block) error

	// GetActiveChain 获取当前活跃链
	//
	// 返回当前活跃链的信息。
	//
	// 参数：
	//   - ctx: 上下文对象
	//
	// 返回：
	//   - *types.ChainInfo: 活跃链信息
	//   - error: 查询错误，nil表示成功
	//
	// 使用场景：
	//   - 查询当前活跃链
	//   - 分叉处理后的链信息查询
	GetActiveChain(ctx context.Context) (*types.ChainInfo, error)
}
