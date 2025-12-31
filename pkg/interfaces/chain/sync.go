// Package chain 提供链管理同步服务的公共接口定义
//
// 🔄 **系统同步服务接口 (System Sync Service)**
//
// 本包定义 WES 系统的链同步服务接口，遵循代码组织规范，
// 同步是链管理的职责，接口定义在chain包内。
//
// 🎯 **核心职责**：
// - 手动同步控制
// - 同步状态查询（委托给ChainQuery）
// - 网络协议处理（通过integration层）
// - 事件订阅处理（通过integration层）
//
// ⚠️ **架构定位**：
// - 同步是链管理的职责，接口定义在chain包内
// - 实现放在 internal/core/chain/sync/
//
// 详细使用说明请参考：pkg/interfaces/chain/README.md
package chain

import (
	"context"

	"github.com/weisyn/v1/pkg/types"
)

// SystemSyncService 系统同步服务接口
//
// 🎯 **同步协调服务**
//
// 负责协调区块同步过程，包括：
// - 手动同步控制
// - 同步状态查询（委托给ChainQuery）
// - 网络协议处理（通过integration层）
// - 事件订阅处理（通过integration层）
//
// ⚠️ **架构定位**：
// - 同步是链管理的职责，接口定义在chain包内
// - 实现放在 internal/core/chain/sync/
//
// 📞 **调用方**：
// - API服务：提供同步控制接口
// - CLI工具：提供同步命令
// - 监控系统：查询同步状态
//
// ⚠️ **核心约束**：
// - 协调服务：不直接操作存储，只读操作通过ChainQuery
// - 委托模式：具体业务逻辑委托给BlockValidator和BlockProcessor
// - 网络适配：网络协议处理通过integration层
// - 状态查询：同步状态实时计算，不持久化（符合区块链特性）
type SystemSyncService interface {
	// TriggerSync 手动触发同步
	//
	// 手动触发区块同步操作，从网络获取缺失的区块。
	//
	// 参数：
	//   - ctx: 上下文对象
	//
	// 返回：
	//   - error: 同步错误，nil表示成功
	//
	// 使用场景：
	//   - 节点启动后检查是否需要同步
	//   - 用户手动触发同步
	//   - 监控系统检测到同步延迟时触发
	TriggerSync(ctx context.Context) error

	// CancelSync 取消当前同步
	//
	// 取消正在进行的同步操作。
	//
	// 参数：
	//   - ctx: 上下文对象
	//
	// 返回：
	//   - error: 取消错误，nil表示成功
	//
	// 使用场景：
	//   - 用户手动取消同步
	//   - 系统关闭时取消同步
	CancelSync(ctx context.Context) error

	// CheckSync 检查同步状态（委托给ChainQuery）
	//
	// 查询当前同步状态，包括：
	// - 本地链高度
	// - 网络高度
	// - 同步进度
	// - 同步状态（idle/syncing/synced/error）
	//
	// 参数：
	//   - ctx: 上下文对象
	//
	// 返回：
	//   - *types.SystemSyncStatus: 同步状态信息
	//   - error: 查询错误
	//
	// 使用场景：
	//   - API服务：提供同步状态查询接口
	//   - 监控系统：监控同步进度
	//   - 自动同步：检查是否需要触发同步
	CheckSync(ctx context.Context) (*types.SystemSyncStatus, error)
}

