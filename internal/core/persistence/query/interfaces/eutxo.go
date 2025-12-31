package interfaces

import (
	"context"

	"github.com/weisyn/v1/pkg/interfaces/persistence"
)

// InternalUTXOQuery 内部UTXO查询接口
// 继承公共接口 persistence.UTXOQuery，遵循代码组织规范
type InternalUTXOQuery interface {
	persistence.UTXOQuery // 嵌入公共接口

	// CheckAssetUTXOConsistency 执行资产 UTXO 状态根一致性检查
	//
	// 返回：
	//   - inconsistent: 是否检测到状态根不一致（true 表示不一致）
	//   - error: 检查过程中发生的错误
	CheckAssetUTXOConsistency(ctx context.Context) (inconsistent bool, err error)

	// RunAssetUTXORepair 执行资产 UTXO 自动修复
	//
	// 当前实现：
	// - 重新计算当前 UTXO 状态根
	// - 在非 dryRun 模式下，将该状态根写回持久化存储（utxo_state_root）
	// - 不对 UTXO 集合本身做清空和从区块重放的完整重建（留待后续扩展）
	RunAssetUTXORepair(ctx context.Context, dryRun bool) error
}

