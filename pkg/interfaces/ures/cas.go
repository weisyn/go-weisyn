// Package ures 提供内容寻址存储的公共接口定义
//
// 📁 **内容寻址存储接口 (Content-Addressable Storage)**
//
// 本包定义 WES 系统的内容寻址存储接口，用于资源文件的存储和读取。
//
// 🎯 **核心职责**：
// - 文件路径构建
// - 文件存储和读取
// - 文件存在性检查
//
// 🏗️ **设计原则**：
// - 内容寻址：基于内容哈希存储文件
// - 职责单一：只负责文件存储操作
//
// 详细使用说明请参考：pkg/interfaces/ures/README.md
package ures

import (
	"context"
)

// CASStorage 内容寻址存储接口（读写）
//
// 🎯 **核心职责**：
// 提供内容寻址存储的文件操作功能。
//
// 💡 **设计理念**：
// - 内容寻址：文件路径基于内容哈希
// - 存储分离：文件存储与资源元数据分离
// - 高效访问：通过哈希快速定位文件
//
// 📞 **调用方**：
// - ResourceWriter：存储资源文件时使用
// - QueryService：查询资源文件时使用
//
// ⚠️ **核心约束**：
// - 内容寻址：文件路径必须基于内容哈希
// - 幂等性：相同内容的文件存储结果一致
type CASStorage interface {
	// BuildFilePath 构建本地文件路径
	//
	// 根据内容哈希构建资源文件的本地存储路径。
	//
	// 参数：
	//   - contentHash: 内容哈希
	//
	// 返回：
	//   - string: 文件路径
	//
	// 使用场景：
	//   - 查询资源文件路径
	//   - 构建文件存储路径
	BuildFilePath(contentHash []byte) string

	// StoreFile 存储文件到内容寻址位置
	//
	// 将文件数据存储到内容寻址位置。
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - contentHash: 内容哈希
	//   - data: 文件数据
	//
	// 返回：
	//   - error: 存储错误，nil表示成功
	//
	// 使用场景：
	//   - 存储资源文件
	//
	// 说明：
	//   - 文件存储在基于内容哈希的路径
	//   - 相同内容的文件会存储在同一位置（幂等性）
	StoreFile(ctx context.Context, contentHash []byte, data []byte) error

	// ReadFile 从内容寻址位置读取文件
	//
	// 根据内容哈希读取文件数据。
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - contentHash: 内容哈希
	//
	// 返回：
	//   - []byte: 文件数据
	//   - error: 读取错误，nil表示成功
	//
	// 使用场景：
	//   - 读取资源文件
	ReadFile(ctx context.Context, contentHash []byte) ([]byte, error)

	// FileExists 检查文件是否存在
	//
	// 检查指定内容哈希的文件是否存在于本地文件系统。
	//
	// 参数：
	//   - contentHash: 内容哈希
	//
	// 返回：
	//   - bool: 文件是否存在
	//
	// 使用场景：
	//   - 检查资源文件是否存在
	FileExists(contentHash []byte) bool
}

