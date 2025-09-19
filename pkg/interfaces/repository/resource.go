package repository

import (
	"context"

	"github.com/weisyn/v1/pkg/types"
)

// ResourceManager 资源管理接口
//
// 🎯 **纯文件操作的资源管理接口**
//
// 基于"文件到文件"的简单理念设计，专注于文件存储和内容寻址。
// 不关心文件内容类型，统一处理所有大小的文件，避免接口复杂化。
//
// 📋 **核心功能**：
// - 文件存储：StoreResourceFile - 从文件路径存储到内容寻址
// - 哈希查询：GetResourceByHash - 基于内容哈希精确查询
// - 类型查询：ListResourcesByType - 按业务类型分页查询
//
// 💡 **设计原则**：
// - 统一操作：所有文件用相同方式处理，无大小区分
// - 内容寻址：基于SHA-256哈希的纯内容寻址
// - 简单高效：避免临时文件、内存加载等复杂操作
// - 确定性：输入文件路径，输出内容哈希
type ResourceManager interface {
	// StoreResourceFile 存储资源文件
	//
	// 🎯 **统一文件存储接口**
	//
	// 从源文件路径读取文件，计算内容哈希，存储到内容寻址位置。
	// 支持任意大小的文件，内部自动优化处理方式。
	//
	// 处理流程：
	// 1. 流式读取源文件并计算SHA-256哈希
	// 2. 检查去重（相同哈希的文件只存储一次）
	// 3. 复制文件到基于哈希的存储路径
	// 4. 建立元数据索引
	//
	// 参数：
	//   - ctx: 上下文对象，用于超时控制和取消操作
	//   - sourceFilePath: 源文件的完整路径
	//   - metadata: 资源元数据信息（类型、创建者等）
	//
	// 返回：
	//   - []byte: 文件内容的SHA-256哈希值（32字节）
	//   - error: 存储操作错误信息
	StoreResourceFile(ctx context.Context, sourceFilePath string, metadata map[string]string) ([]byte, error)

	// GetResourceByHash 基于内容哈希获取资源信息
	//
	// 通过内容哈希获取完整的资源存储信息和元数据。
	// 这是内容寻址架构的核心查询方法。
	//
	// 参数：
	//   - ctx: 上下文对象，用于超时控制和取消操作
	//   - contentHash: 资源内容的SHA-256哈希值（32字节）
	//
	// 返回：
	//   - *types.ResourceStorageInfo: 资源存储信息
	//   - error: 查询错误信息
	GetResourceByHash(ctx context.Context, contentHash []byte) (*types.ResourceStorageInfo, error)

	// ListResourcesByType 按类型列出资源
	//
	// 根据资源类型查询资源列表，支持分页查询。
	// 用于资源浏览和管理功能。
	//
	// 参数：
	//   - ctx: 上下文对象，用于超时控制和取消操作
	//   - resourceType: 资源类型标识符（contract、aimodel、static等）
	//   - offset: 查询偏移量（分页起始位置）
	//   - limit: 查询限制数量（分页大小）
	//
	// 返回：
	//   - []*types.ResourceStorageInfo: 资源存储信息列表
	//   - error: 查询错误信息
	ListResourcesByType(ctx context.Context, resourceType string, offset int, limit int) ([]*types.ResourceStorageInfo, error)
}
