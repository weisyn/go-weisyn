# 文件存储实现 (File Storage Implementation)

　　本模块提供基于文件系统的持久化存储实现，遵循 `pkg/interfaces/infrastructure/storage/file.go` 中定义的 FileStore 接口。

## 🎯 主要功能

### 基础文件操作
- **Save/Load**: 文件的保存和读取操作
- **Delete**: 文件删除，支持校验和文件的自动清理
- **Exists**: 文件存在性检查
- **FileInfo**: 文件元数据获取（大小、创建时间、修改时间等）

### 目录管理
- **MakeDir**: 目录创建，支持递归创建
- **DeleteDir**: 目录删除，支持递归删除
- **ListFiles**: 目录文件列表，支持通配符过滤

### 流式操作
- **OpenReadStream**: 文件读取流，适合大文件操作
- **OpenWriteStream**: 文件写入流，适合流式写入
- **Copy**: 文件复制操作
- **Move**: 文件移动/重命名操作

### 数据完整性
- **校验和验证**: 自动生成和验证 SHA-256 校验和
- **文件完整性**: 读取时自动验证文件完整性
- **权限控制**: 可配置的文件和目录权限设置

## ⚙️ 配置选项

```json
{
  "root_path": "./data/files",           // 根目录路径
  "max_file_size": 1024,                 // 最大文件大小(MB)
  "directory_index_enabled": true,       // 启用目录索引
  "file_verification_enabled": true,     // 启用文件校验
  "file_permissions": 644,               // 文件权限
  "directory_permissions": 755           // 目录权限
}
```

## 🔧 使用示例

```go
// 通过依赖注入获取文件存储
var fileStore storage.FileStore

// 保存文件
err := fileStore.Save(ctx, "data/example.txt", []byte("Hello World"))

// 读取文件
data, err := fileStore.Load(ctx, "data/example.txt")

// 使用流式操作处理大文件
reader, err := fileStore.OpenReadStream(ctx, "large_file.dat")
defer reader.Close()
```

## 🏗️ 架构特点

- **线程安全**: 使用读写锁保护并发访问
- **错误处理**: 完善的错误处理和日志记录
- **配置灵活**: 支持多种配置方式和默认值
- **性能优化**: 支持流式操作，适合大文件处理
- **依赖注入**: 完全集成到 fx 依赖注入系统

## 📁 目录结构

```
internal/core/infrastructure/storage/file/
├── store.go        # 主要实现文件
└── README.md       # 本文档

internal/config/storage/file/
├── config.go       # 配置接口和实现
└── defaults.go     # 默认配置值
```

## 🔒 安全特性

- **路径安全**: 自动处理相对路径和绝对路径
- **权限控制**: 可配置的文件和目录权限
- **校验和验证**: 防止数据损坏和篡改
- **错误隔离**: 单个文件操作错误不影响其他操作
