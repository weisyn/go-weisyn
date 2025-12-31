// Package interfaces 提供 EUTXO 模块的内部接口定义
package interfaces

import (
	"github.com/weisyn/v1/pkg/interfaces/eutxo"
)

// InternalResourceUTXOQuery 内部资源 UTXO 查询接口
//
// ⚠️ **重要说明**：
// - 此接口继承公共接口 ResourceUTXOQuery，并添加内部扩展方法
// - 符合代码组织规范：内部接口嵌入公共接口
// - 内部方法仅供 EUTXO 模块内部使用
//
// 🎯 **设计目的**：
// - 满足内部查询需求：索引更新器需要查询现有记录
// - 继承公共接口：确保与公共接口一致
// - 扩展内部方法：添加内部需要的额外方法
//
// 💡 **设计理念**：
// - 接口继承：嵌入 eutxo.ResourceUTXOQuery 公共接口
// - 内部扩展：添加批量更新等内部方法
// - 简单实现：基于 Storage 的简单实现
//
// 📞 **调用方**（仅限内部）：
// - ResourceUTXOIndexUpdater - 需要查询现有记录进行更新
// - 内部验证 - 需要查询资源 UTXO 状态
//
// 🔄 **架构说明**：
// - 公共接口：pkg/interfaces/eutxo.ResourceUTXOQuery（对外暴露）
// - 内部接口：InternalResourceUTXOQuery（嵌入公共接口 + 内部方法）
// - 实现：internal/core/eutxo/query/resource.go（实现内部接口）
type InternalResourceUTXOQuery interface {
	// 嵌入公共接口（符合代码组织规范）
	// 继承的方法：GetResourceUTXOByContentHash, ListResourceUTXOs, GetResourceUsageCounters
	eutxo.ResourceUTXOQuery
}

