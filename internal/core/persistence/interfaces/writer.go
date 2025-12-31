// Package interfaces 定义 persistence 组件的内部接口
//
// 🔧 **内部接口层 (Internal Interfaces Layer)**
//
// 本包定义 persistence 组件的内部接口，作为公共接口和具体实现之间的桥梁。
//
// 🎯 **核心职责**：
// - 继承公共接口（persistence.DataWriter）
// - 扩展内部专用方法（如需要）
//
// 🏗️ **架构定位**：
// ```
// pkg/interfaces/persistence (公共接口)
//     ↓ 继承
// internal/core/persistence/interfaces (内部接口) ← 本目录
//     ↓ 实现
// internal/core/persistence/writer (服务实现)
// ```
package interfaces

import (
	"github.com/weisyn/v1/pkg/interfaces/persistence"
)

// InternalDataWriter 内部数据写入接口
//
// 🎯 **核心职责**：
// 继承公共接口 persistence.DataWriter，作为实现层与公共接口的桥接。
//
// 💡 **设计理念**：
// - 继承公共接口：嵌入 persistence.DataWriter
// - 内部扩展：目前无额外内部方法（纯继承）
// - 实现约束：所有实现必须实现此内部接口
//
// 📋 **继承关系**：
// - 继承：persistence.DataWriter
//   - WriteBlock(ctx, block) error
//   - WriteBlocks(ctx, blocks) error
//
// ⚠️ **注意事项**：
// - 内部接口仅用于实现层，不对外暴露
// - 通过 module.go 绑定到公共接口
// - 如果未来需要内部协作方法，可在此扩展
type InternalDataWriter interface {
	persistence.DataWriter // 嵌入公共接口（强制继承）

	// 内部专用方法（目前无，如需要可在此添加）
	// 例如：
	// getCurrentHeight() (uint64, error)  // 内部：获取当前高度
	// validateBlockOrder(block *core.Block) error  // 内部：验证区块顺序
}

