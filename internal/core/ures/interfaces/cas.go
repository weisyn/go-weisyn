// Package interfaces 定义 URES 模块的内部接口
//
// 🎯 **设计目的**：
// - 继承公共接口（pkg/interfaces/ures）
// - 扩展内部方法（性能指标、内部使用）
// - 支持测试和 Mock
package interfaces

import (
	uresif "github.com/weisyn/v1/pkg/interfaces/ures"
)

// InternalCASStorage 内部内容寻址存储接口
//
// 🎯 **核心职责**：
// - 继承公共 CASStorage 接口
//
// 💡 **设计理念**：
// - 接口继承：嵌入公共接口
// - 易于测试：支持 Mock
//
// 📞 **实现方**：
// - cas.Service：CASStorage 服务实现
//
// 📞 **调用方**：
// - writer.Service：ResourceWriter 服务
type InternalCASStorage interface {
	uresif.CASStorage // 嵌入公共接口
}

