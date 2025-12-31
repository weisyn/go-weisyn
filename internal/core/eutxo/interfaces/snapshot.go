// Package interfaces 提供 EUTXO 模块的内部接口定义
package interfaces

import (
	"context"

	eutxo "github.com/weisyn/v1/pkg/interfaces/eutxo"
	// persistence "github.com/weisyn/v1/pkg/interfaces/persistence" // ⚠️ 已移除：EUTXO 模块不应依赖 persistence 模块
	"github.com/weisyn/v1/pkg/types"
)

// InternalUTXOSnapshot 内部 UTXO 快照接口
//
// 🎯 **核心职责**：
// - 继承公共 UTXOSnapshot 接口的所有方法
// - 提供内部管理方法（指标、验证）
// - 支持延迟依赖注入（避免循环依赖）
//
// 💡 **设计理念**：
// - 嵌入式继承：通过嵌入 eutxo.UTXOSnapshot 继承所有公共方法
// - 内部扩展：添加 GetSnapshotMetrics、ValidateSnapshot 等内部方法
// - 延迟注入：通过 SetWriter、SetQuery 解决循环依赖
//
// 📞 **调用方**：
// - Chain.ForkHandler - 分叉处理时使用快照
// - Blockchain.SyncService - 同步过程中使用快照
// - 监控系统 - 获取快照指标
//
// ⚠️ **核心约束**：
// - 内部方法不通过 fx 导出给外部模块
// - 快照操作必须在事务中执行
// - 快照恢复必须原子性完成
type InternalUTXOSnapshot interface {
	eutxo.UTXOSnapshot // 嵌入公共接口

	// ==================== 内部管理方法 ====================

	// ValidateSnapshot 验证快照数据的有效性
	//
	// 用途：
	// - 数据验证：确保快照数据完整
	// - 哈希验证：验证快照哈希正确
	// - 预检查：在恢复快照前验证
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - snapshot: 快照数据对象
	//
	// 返回：
	//   - error: 验证错误，nil 表示验证通过
	ValidateSnapshot(ctx context.Context, snapshot *types.UTXOSnapshotData) error

	// ==================== 延迟依赖注入 ====================

	// SetWriter 设置 UTXO 写入器（用于快照恢复）
	//
	// 🎯 **设计目的**：
	// - 避免循环依赖：UTXOSnapshot 依赖 UTXOWriter，但不能在构造时注入
	// - 延迟绑定：在所有服务创建后，通过 fx.Invoke 注入
	// - 运行时配置：支持在运行时动态替换依赖
	//
	// 用途：
	// - 快照恢复：需要调用 UTXOWriter.CreateUTXO
	//
	// 参数：
	//   - writer: UTXO 写入器实例
	SetWriter(writer InternalUTXOWriter)

	// SetQuery 设置 UTXO 查询器（用于快照创建）
	//
	// 🎯 **设计目的**：
	// - 避免循环依赖：UTXOSnapshot 依赖 UTXOQuery，但不能在构造时注入
	// - 延迟绑定：在所有服务创建后，通过 fx.Invoke 注入
	// - 运行时配置：支持在运行时动态替换依赖
	//
	// 用途：
	// - 快照创建：需要调用 UTXOQuery.ListUTXOs
	//
	// 参数：
	//   - query: UTXO 查询器实例
	SetQuery(query InternalUTXOQuery)

	// SetBlockQuery 设置区块查询器（已移除，架构修复）
	//
	// ⚠️ **架构修复**：EUTXO 模块不应依赖 persistence 模块
	// 区块哈希应该由调用方（CHAIN 层的 ForkHandler）提供
	// 此方法已移除，不再需要 BlockQuery 依赖
	// SetBlockQuery(blockQuery persistence.BlockQuery) // 已移除
}

