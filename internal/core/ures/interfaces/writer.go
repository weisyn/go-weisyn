// Package interfaces 定义 URES 模块的内部接口
package interfaces

import (
	uresif "github.com/weisyn/v1/pkg/interfaces/ures"
)

// InternalResourceWriter 内部资源写入接口
//
// 🎯 **核心职责**：
// - 继承公共 ResourceWriter 接口
//
// 💡 **设计理念**：
// - 接口继承：嵌入公共接口
// - 易于测试：支持 Mock
// - 职责明确：只负责文件存储，不涉及资源索引更新
//
// 📞 **实现方**：
// - writer.Service：ResourceWriter 服务实现
//
// 📞 **调用方**：
// - ISPC.Runtime：合约执行后存储资源文件
// - TX.Processor：交易中包含资源时存储资源文件
// - DataWriter：在写入区块时可以调用 ResourceWriter 存储文件
type InternalResourceWriter interface {
	uresif.ResourceWriter // 嵌入公共接口
}
