// Package ures 提供资源写入操作的公共接口定义
//
// 📦 **资源写入接口 (Resource Writer)**
//
// 本包定义 WES 系统的资源写入接口，遵循 CQRS 架构原则，
// 专注于资源的写入操作。
//
// 🎯 **核心职责**：
// - 资源文件存储（内容寻址存储）
//
// 🏗️ **设计原则**：
// - CQRS 写路径：只包含写操作，不包含查询
// - 直接操作存储：内部直接操作基础设施层
// - 职责单一：只处理资源的写入
//
// 详细使用说明请参考：pkg/interfaces/ures/README.md
package ures

import (
	"context"
)

// ResourceWriter 资源写入接口（CQRS写路径）
//
// 🎯 **核心职责**：
// 提供资源文件的写入操作，专注于内容寻址存储。
//
// 💡 **设计理念**：
// - 只包含文件存储操作，不包含查询操作（查询由 ResourceQuery 提供）
// - 内部直接操作文件存储层，不通过查询接口
// - 支持内容寻址存储（CAS）
// - 资源索引更新由 DataWriter 统一处理，不在本接口中
//
// 📞 **调用方**：
// - ISPC模块：合约执行后存储资源文件
// - TX模块：交易中包含资源时存储资源文件
//
// ⚠️ **核心约束**：
// - 只负责文件存储：不涉及资源索引更新（由 DataWriter 处理）
// - 幂等性：相同内容的文件只存储一次
type ResourceWriter interface {
	// StoreResourceFile 存储资源文件
	//
	// 将资源文件存储到内容寻址存储系统。
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - sourceFilePath: 源文件路径
	//
	// 返回：
	//   - []byte: 内容哈希（用于后续查询）
	//   - error: 存储错误，nil表示成功
	//
	// 使用场景：
	//   - 部署资源时：在创建 ResourceOutput 之前存储文件
	//   - 合约执行后：存储合约执行产生的资源文件
	//
	// ⚠️ **存储时机**：
	//   - 必须在创建 ResourceOutput 之前调用，因为 ResourceOutput 需要 contentHash
	//   - 文件存储是幂等的，相同内容的文件只存储一次
	//   - 资源索引会在交易确认后由 DataWriter.WriteBlock() 自动更新
	//
	// 说明：
	//   - 文件存储在内容寻址位置（基于内容哈希）
	//   - 返回的内容哈希用于创建 ResourceOutput
	//   - 资源索引更新由 DataWriter.WriteBlock() 统一处理，无需手动调用
	StoreResourceFile(ctx context.Context, sourceFilePath string) ([]byte, error)
}

