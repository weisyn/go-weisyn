// Package interfaces 提供 EUTXO 模块的内部接口定义
//
// 📐 **设计理念**：
// - 继承公共接口，确保外部可见性
// - 扩展内部方法，支持系统内部协调
// - 提供指标接口，支持监控和调试
//
// 🎯 **与公共接口的关系**：
// - InternalUTXOWriter 嵌入 pkg/interfaces/eutxo.UTXOWriter
// - 对外暴露为 eutxo.UTXOWriter（通过 fx.As）
// - 内部使用扩展方法（GetWriterMetrics, ValidateUTXO）
//
// 详细设计说明请参考：internal/core/eutxo/TECHNICAL_DESIGN.md
package interfaces

import (
	"context"

	"github.com/weisyn/v1/pb/blockchain/utxo"
	eutxo "github.com/weisyn/v1/pkg/interfaces/eutxo"
)

// InternalUTXOWriter 内部 UTXO 写入接口
//
// 🎯 **核心职责**：
// - 继承公共 UTXOWriter 接口的所有方法
// - 提供内部管理方法（指标、验证）
// - 支持系统内部协调和监控
//
// 💡 **设计理念**：
// - 嵌入式继承：通过嵌入 eutxo.UTXOWriter 继承所有公共方法
// - 内部扩展：添加 GetWriterMetrics、ValidateUTXO 等内部方法
// - 接口分离：内部方法不对外暴露，只供系统内部使用
//
// 📞 **调用方**：
// - Block.Processor - 处理区块时更新 UTXO
// - TX.Processor - 处理交易时更新 UTXO
// - UTXOSnapshot - 快照恢复时使用
// - 监控系统 - 获取性能指标
//
// ⚠️ **核心约束**：
// - 内部方法不通过 fx 导出给外部模块
// - 只在 EUTXO 模块内部和核心模块中使用
type InternalUTXOWriter interface {
	eutxo.UTXOWriter // 嵌入公共接口

	// ==================== 内部管理方法 ====================

	// ValidateUTXO 验证 UTXO 对象的有效性
	//
	// 用途：
	// - 数据验证：确保 UTXO 数据格式正确
	// - 业务规则：验证 UTXO 符合业务规则
	// - 预检查：在创建 UTXO 前验证
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - utxoObj: UTXO 对象
	//
	// 返回：
	//   - error: 验证错误，nil 表示验证通过
	ValidateUTXO(ctx context.Context, utxoObj *utxo.UTXO) error
}

