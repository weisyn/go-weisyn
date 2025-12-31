// Package storage 提供WES系统的文件存储接口定义
//
// 📁 **文件存储服务 (File Storage Service)**
//
// 本文件定义了WES区块链系统的文件存储接口，专注于：
// - 文件管理：完整的文件CRUD操作和元数据管理
// - 多后端支持：可基于本地文件系统、S3、MinIO等实现
// - 流式处理：支持大文件的流式上传和下载
// - 版本控制：文件版本管理和历史记录追踪
//
// 🎯 **核心功能**
// - FileService：文件存储服务接口，提供完整的文件管理能力
// - 多存储后端：灵活的存储后端选择和切换
// - 元数据管理：文件属性、权限和索引的完整管理
// - 流式传输：高效的大文件上传下载和断点续传
//
// 🏗️ **设计原则**
// - 后端无关：抽象存储后端实现，支持多种存储方案
// - 性能优化：流式处理和并发优化提升传输效率
// - 数据安全：文件完整性校验和访问权限控制
// - 易扩展性：支持插件化的存储后端扩展
//
// 🔗 **组件关系**
// - FileService：被媒体、文档、备份等模块使用
// - 与StorageProvider：作为文件数据的专用存储层
// - 与TempService：配合处理临时文件和持久化文件
package storage

import (
	"context"
	"io"

	"github.com/weisyn/v1/pkg/types"
)

//=============================================================================
// FileStore 接口定义
//=============================================================================

// FileStore 定义了通用的文件存储接口
// 适用于需要持久化文件数据的应用场景
// 提供文件的读写、目录管理、流式操作等功能
type FileStore interface {
	//-------------------------------------------------------------------------
	// 生命周期管理
	//-------------------------------------------------------------------------

	//-------------------------------------------------------------------------
	// 基本文件操作
	//-------------------------------------------------------------------------
	// 注意：文件存储资源由DI容器自动管理，无需手动Close()

	// Save 保存数据到指定路径
	// path: 文件路径，可以是相对路径或绝对路径
	// data: 要保存的二进制数据
	// 如果文件已存在，会被覆盖
	Save(ctx context.Context, path string, data []byte) error

	// Load 从指定路径加载数据
	// path: 文件路径，可以是相对路径或绝对路径
	// 返回文件的二进制内容
	// 如果文件不存在，返回错误
	Load(ctx context.Context, path string) ([]byte, error)

	// Delete 删除指定路径的文件
	// path: 文件路径，可以是相对路径或绝对路径
	// 如果文件不存在，返回错误
	// 如果文件正在被使用，可能返回错误
	Delete(ctx context.Context, path string) error

	// Exists 检查指定路径的文件是否存在
	// path: 文件路径，可以是相对路径或绝对路径
	// 返回true表示文件存在，false表示文件不存在
	Exists(ctx context.Context, path string) (bool, error)

	// FileInfo 获取文件信息
	// path: 文件路径，可以是相对路径或绝对路径
	// 返回文件的元数据信息，如大小、创建时间、修改时间等
	// 如果文件不存在，返回错误
	FileInfo(ctx context.Context, path string) (types.FileInfo, error)

	//-------------------------------------------------------------------------
	// 目录操作
	//-------------------------------------------------------------------------

	// ListFiles 列出指定目录下的所有文件
	// dirPath: 目录路径，可以是相对路径或绝对路径
	// pattern: 文件名匹配模式，支持通配符，如"*.txt"，为空则不过滤
	// 返回符合条件的文件路径列表，不包含子目录中的文件
	ListFiles(ctx context.Context, dirPath string, pattern string) ([]string, error)

	// MakeDir 创建目录
	// dirPath: 目录路径，可以是相对路径或绝对路径
	// recursive: 是否递归创建，true表示创建路径中的所有不存在的父目录
	// 如果目录已存在，不会返回错误
	MakeDir(ctx context.Context, dirPath string, recursive bool) error

	// DeleteDir 删除目录
	// dirPath: 目录路径，可以是相对路径或绝对路径
	// recursive: 是否递归删除，true表示删除目录中的所有内容包括子目录
	// 如果目录不存在，返回错误
	// 如果recursive为false且目录不为空，返回错误
	DeleteDir(ctx context.Context, dirPath string, recursive bool) error

	//-------------------------------------------------------------------------
	// 流式操作
	//-------------------------------------------------------------------------

	// OpenReadStream 打开文件的读取流
	// path: 文件路径，可以是相对路径或绝对路径
	// 返回文件的读取流，调用者负责在使用完毕后关闭流
	// 如果文件不存在，返回错误
	OpenReadStream(ctx context.Context, path string) (io.ReadCloser, error)

	// OpenWriteStream 打开文件的写入流
	// path: 文件路径，可以是相对路径或绝对路径
	// 返回文件的写入流，调用者负责在使用完毕后关闭流
	// 如果文件已存在，会被覆盖
	OpenWriteStream(ctx context.Context, path string) (io.WriteCloser, error)

	// Copy 复制文件
	// sourcePath: 源文件路径
	// destPath: 目标文件路径
	// 将源文件复制到目标位置
	// 如果源文件不存在，返回错误
	// 如果目标文件已存在，会被覆盖
	Copy(ctx context.Context, sourcePath, destPath string) error

	// Move 移动文件
	// sourcePath: 源文件路径
	// destPath: 目标文件路径
	// 将源文件移动到目标位置，相当于重命名
	// 如果源文件不存在，返回错误
	// 如果目标文件已存在，会被覆盖
	Move(ctx context.Context, sourcePath, destPath string) error
}

//=============================================================================
// FileInfo 结构体定义
//=============================================================================

// 兼容别名（数据结构迁至 pkg/types）
type FileInfo = types.FileInfo
