// Package interfaces 提供 EUTXO 模块的内部接口定义
package interfaces

import (
	"github.com/weisyn/v1/pkg/interfaces/eutxo"
)

// InternalUTXOQuery 内部 UTXO 查询接口
//
// ⚠️ **重要说明**：
// - 此接口继承公共接口 UTXOQuery，并添加内部扩展方法
// - 符合代码组织规范：内部接口嵌入公共接口
// - 内部方法仅供 EUTXO 模块内部使用
//
// 🎯 **设计目的**：
// - 满足内部查询需求：UTXOSnapshot、UTXOWriter 需要查询 UTXO
// - 继承公共接口：确保与公共接口一致
// - 扩展内部方法：当前不再扩展（破坏性调整：ListUTXOs 已提升为公共接口）
//
// 💡 **设计理念**：
// - 接口继承：嵌入 eutxo.UTXOQuery 公共接口
// - 内部扩展：当前不再扩展
// - 简单实现：基于 Storage 的简单实现
//
// 📞 **调用方**（仅限内部）：
// - UTXOSnapshot.CreateSnapshot - 需要查询所有 UTXO
// - UTXOWriter.ReferenceUTXO - 需要查询引用计数
// - 内部验证 - 需要查询 UTXO 状态
//
// 🔄 **架构说明**：
// - 公共接口：pkg/interfaces/eutxo.UTXOQuery（对外暴露）
// - 内部接口：InternalUTXOQuery（嵌入公共接口 + 内部方法）
// - 实现：internal/core/eutxo/query.Service（实现内部接口）
type InternalUTXOQuery interface {
	// 嵌入公共接口（符合代码组织规范）
	// 继承的方法：GetUTXO, GetUTXOsByAddress, GetReferenceCount
	eutxo.UTXOQuery
}

