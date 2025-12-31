// Package eutxo 提供UTXO快照管理的公共接口定义
//
// 📸 **UTXO快照接口 (UTXO Snapshot)**
//
// 本包定义 WES 系统的 UTXO 快照管理接口，用于快照创建和恢复。
//
// 🎯 **核心职责**：
// - UTXO快照创建
// - UTXO快照恢复
// - 快照管理
//
// 🏗️ **设计原则**：
// - CQRS 写路径：快照操作涉及状态修改
// - 事务保证：快照操作必须在事务中执行
// - 原子性：快照恢复必须原子性执行
//
// 详细使用说明请参考：pkg/interfaces/eutxo/README.md
package eutxo

import (
	"context"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/types"
)

// UTXOClearPlan 是“清空当前 UTXO 及其索引/引用关系”的删除计划（事务外预收集、事务内执行）。
//
// 说明：
// - 由于 BadgerTransaction 不提供 scan/iterator，必须通过 BadgerStore.PrefixScan 在事务外收集 keys。
// - 该计划用于“单事务原子快照恢复”与“与索引回滚同事务合并”的高危场景（fork/reorg）。
type UTXOClearPlan struct {
	UTXOKeys         [][]byte // utxo:set:*
	IndexAddressKeys [][]byte // index:address:*
	IndexHeightKeys  [][]byte // index:height:*
	IndexAssetKeys   [][]byte // index:asset:*
	RefKeys          [][]byte // ref:*
}

// UTXOSnapshotPayload 是“快照内容解码后的载荷”，用于在事务外完成 IO/解码，
// 并在事务内仅执行确定性的 Delete/Set（满足严格原子回滚要求）。
type UTXOSnapshotPayload struct {
	Version int
	Utxos   [][]byte
}

// UTXOSnapshot UTXO快照管理接口（写操作）
//
// 🎯 **核心职责**：
// 提供 UTXO 快照的创建、恢复和管理功能。
//
// 💡 **设计理念**：
// - 快照操作涉及状态修改，属于写操作
// - 必须在事务中执行，确保原子性
// - 支持分叉处理和状态回滚
//
// 📞 **调用方**：
// - ForkHandler：分叉处理时使用快照
// - SyncService：同步过程中使用快照
//
// ⚠️ **核心约束**：
// - 事务保证：所有操作必须在事务中执行
// - 原子性：快照恢复必须原子性完成
// - 一致性：快照恢复后状态必须一致
type UTXOSnapshot interface {
	// CreateSnapshot 创建UTXO快照
	//
	// 在指定高度创建 UTXO 集合的快照。
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - height: 快照高度
	//
	// 返回：
	//   - *types.UTXOSnapshotData: 快照数据对象
	//   - error: 创建错误，nil表示成功
	//
	// 使用场景：
	//   - 分叉处理前创建快照
	//   - 区块处理前创建快照（用于回滚）
	CreateSnapshot(ctx context.Context, height uint64) (*types.UTXOSnapshotData, error)

	// BuildClearPlan 构建清空当前 UTXO/索引/引用关系的删除计划（事务外预收集）。
	BuildClearPlan(ctx context.Context) (*UTXOClearPlan, error)

	// LoadSnapshotPayload 加载并解码快照载荷（事务外 IO/解码 + 哈希/版本校验）。
	LoadSnapshotPayload(ctx context.Context, snapshot *types.UTXOSnapshotData) (*UTXOSnapshotPayload, error)

	// RestoreSnapshotInTransaction 在已有 BadgerTransaction 中恢复快照（原子写入）。
	//
	// 关键约束：
	// - 必须在同一事务内完成：清空旧 UTXO/索引/引用 + 写入新 UTXO + 重建索引 + 更新 StateRoot。
	// - 不允许在事务内 scan；clearPlan 必须由 BuildClearPlan 在事务外生成。
	RestoreSnapshotInTransaction(ctx context.Context, tx storage.BadgerTransaction, snapshot *types.UTXOSnapshotData, payload *UTXOSnapshotPayload, clearPlan *UTXOClearPlan) error

	// 🆕 RestoreSnapshotWithBatching 分批恢复快照（解决"Txn is too big"问题）
	//
	// 与 RestoreSnapshotInTransaction 不同，此方法：
	// - 不接收外部事务，自己管理多个小事务
	// - 将大量UTXO分批提交，避免单个事务过大
	// - 适用于Fork回滚等需要恢复大量UTXO的场景
	//
	// 参数：
	// - ctx: 上下文
	// - snapshot: 快照元数据
	// - payload: 快照数据（已加载）
	// - clearPlan: 清空计划（已构建）
	//
	// 返回：
	// - error: 恢复失败时返回错误
	RestoreSnapshotWithBatching(ctx context.Context, snapshot *types.UTXOSnapshotData, payload *UTXOSnapshotPayload, clearPlan *UTXOClearPlan) error

	// RestoreSnapshotAtomic 原子恢复快照（内部自行开启事务）。
	//
	// 说明：
	// - 此方法是新的稳定入口，用于非合并事务的场景（例如自愈/运维恢复）。
	// - 需要与 RestoreSnapshotInTransaction 语义一致，确保恢复过程在事务内完成。
	RestoreSnapshotAtomic(ctx context.Context, snapshot *types.UTXOSnapshotData) error

	// DeleteSnapshot 删除快照
	DeleteSnapshot(ctx context.Context, snapshotID string) error

	// ListSnapshots 列出所有快照
	//
	// 返回所有快照的列表。
	//
	// 参数：
	//   - ctx: 上下文对象
	//
	// 返回：
	//   - []*types.UTXOSnapshotData: 快照数据列表
	//
	// 使用场景：
	//   - 查询所有快照
	//   - 快照管理
	ListSnapshots(ctx context.Context) ([]*types.UTXOSnapshotData, error)
}

