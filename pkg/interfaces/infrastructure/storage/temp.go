// Package storage 提供WES系统的临时存储接口定义
//
// ⏳ **临时存储服务 (Temporary Storage Service)**
//
// 本文件定义了WES区块链系统的临时存储接口，专注于：
// - 临时文件管理：短期文件的创建、存储和自动清理
// - 生命周期控制：基于时间和事件的自动清理机制
// - 流式处理：支持大文件的流式读写和传输
// - 安全隔离：临时数据的安全隔离和访问控制
//
// 🎯 **核心功能**
// - TempService：临时存储服务接口，提供完整的临时数据管理
// - 自动清理：基于TTL和事件的智能清理策略
// - 流式操作：高效的大文件流式读写支持
// - 路径管理：安全的临时文件路径生成和管理
//
// 🏗️ **设计原则**
// - 自动管理：智能的生命周期管理和资源回收
// - 安全可靠：严格的访问控制和数据隔离
// - 性能友好：高效的I/O操作和内存使用
// - 易用性：简洁的API设计和错误处理
//
// 🔗 **组件关系**
// - TempService：被文件上传、数据处理、缓存等模块使用
// - 与StorageProvider：作为临时数据的专用存储层
// - 与FileService：配合提供完整的文件存储解决方案
package storage

import (
	"context"
	"io"
	"time"

	"github.com/weisyn/v1/pkg/types"
)

//=============================================================================
// TempStore 接口定义
//=============================================================================

// TempStore 定义了临时文件存储接口
// 提供临时文件的创建、读写和自动清理功能
// 适用于需要处理临时数据、大型文件上传、离线处理等场景
type TempStore interface {
	//-------------------------------------------------------------------------
	// 生命周期管理
	//-------------------------------------------------------------------------

	// Close 关闭临时存储并释放资源
	// 注意：临时存储资源由DI容器自动管理，但提供Close方法用于手动清理
	Close() error

	//-------------------------------------------------------------------------
	// 临时文件操作
	//-------------------------------------------------------------------------

	// CreateTempFile 创建临时文件
	// prefix: 文件名前缀，用于识别文件来源
	// suffix: 文件后缀，如".txt"、".pdf"等
	// 返回临时文件的唯一标识id和可读写的流
	// 调用者负责在使用完毕后关闭流
	CreateTempFile(ctx context.Context, prefix, suffix string) (id string, file io.ReadWriteCloser, err error)

	// CreateTempFileWithContent 创建临时文件并写入内容
	// prefix: 文件名前缀，用于识别文件来源
	// suffix: 文件后缀，如".txt"、".pdf"等
	// content: 要写入的文件内容
	// 返回临时文件的唯一标识id
	// 创建后文件会自动关闭
	CreateTempFileWithContent(ctx context.Context, prefix, suffix string, content []byte) (id string, err error)

	// GetTempFile 获取临时文件内容
	// id: 临时文件的唯一标识
	// 返回临时文件的完整内容
	// 如果文件不存在或已过期，返回错误
	GetTempFile(ctx context.Context, id string) (content []byte, err error)

	// OpenTempFile 打开临时文件
	// id: 临时文件的唯一标识
	// 返回临时文件的读写流
	// 调用者负责在使用完毕后关闭流
	// 如果文件不存在或已过期，返回错误
	OpenTempFile(ctx context.Context, id string) (file io.ReadWriteCloser, err error)

	// RemoveTempFile 删除临时文件
	// id: 临时文件的唯一标识
	// 永久删除指定的临时文件
	// 如果文件不存在，不会返回错误
	RemoveTempFile(ctx context.Context, id string) error

	//-------------------------------------------------------------------------
	// 临时目录操作
	//-------------------------------------------------------------------------

	// CreateTempDir 创建临时目录
	// prefix: 目录名前缀，用于识别目录来源
	// 返回临时目录的唯一标识id
	// 临时目录可用于存放相关的临时文件
	CreateTempDir(ctx context.Context, prefix string) (id string, err error)

	// RemoveTempDir 删除临时目录
	// id: 临时目录的唯一标识
	// 递归删除指定的临时目录及其内容
	// 如果目录不存在，不会返回错误
	RemoveTempDir(ctx context.Context, id string) error

	//-------------------------------------------------------------------------
	// 管理功能
	//-------------------------------------------------------------------------

	// ListTempFiles 列出所有临时文件
	// pattern: 文件名匹配模式，支持通配符，如"upload_*"
	// pattern为空则列出所有临时文件
	// 返回临时文件的详细信息列表
	ListTempFiles(ctx context.Context, pattern string) ([]types.TempFileInfo, error)

	// CleanupExpired 清理所有过期的临时文件和目录
	// 自动删除已过期的临时文件和目录
	// 返回被清理的文件和目录数量
	// 过期时间由创建时指定或系统默认值决定
	CleanupExpired(ctx context.Context) (int, error)

	// SetExpiration 设置临时文件或目录的过期时间
	// id: 临时文件或目录的唯一标识
	// duration: 从当前时间起计算的生存时间
	// 如果duration为0，则使用系统默认值
	// 可用于延长或缩短临时资源的生存时间
	SetExpiration(ctx context.Context, id string, duration time.Duration) error
}

//=============================================================================
// TempFileInfo 结构体定义
//=============================================================================

// 兼容别名（数据结构迁至 pkg/types）
type TempFileInfo = types.TempFileInfo
