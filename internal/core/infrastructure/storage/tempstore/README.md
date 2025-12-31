# 临时存储实现 (Temporary Storage Implementation)

　　本模块提供基于文件系统的临时文件存储实现，遵循 `pkg/interfaces/infrastructure/storage/temp.go` 中定义的 TempStore 接口。临时存储专门用于处理短期数据存储需求，具备自动清理和生命周期管理功能。

## 🎯 主要功能

### 临时文件管理
- **CreateTempFile**: 创建临时文件，返回唯一ID和读写流
- **CreateTempFileWithContent**: 创建临时文件并写入指定内容
- **GetTempFile**: 获取临时文件的完整内容
- **OpenTempFile**: 打开临时文件获取读写流
- **RemoveTempFile**: 手动删除临时文件

### 临时目录管理
- **CreateTempDir**: 创建临时目录
- **RemoveTempDir**: 删除临时目录及其内容

### 自动清理机制
- **过期检查**: 自动检查和清理过期文件
- **定时清理**: 后台定时任务清理过期资源
- **生命周期管理**: 可配置的TTL（生存时间）控制
- **SetExpiration**: 动态调整文件/目录过期时间

### 管理功能
- **ListTempFiles**: 列出所有临时文件，支持模式匹配
- **CleanupExpired**: 手动触发过期文件清理
- **文件数量限制**: 防止临时文件过多占用磁盘空间

## ⚙️ 配置选项

```json
{
  "temp_dir": "./data/temp",              // 临时文件目录
  "default_ttl": "24h",                   // 默认生存时间
  "max_temp_file_size": 512,              // 最大临时文件大小(MB)
  "cleanup_interval": "1h",               // 清理任务间隔
  "max_temp_files": 10000,                // 最大临时文件数量
  "auto_cleanup_enabled": true,           // 启用自动清理
  "file_permissions": 644,                // 文件权限
  "directory_permissions": 755            // 目录权限
}
```

## 🔧 使用示例

```go
// 通过依赖注入获取临时存储
var tempStore storage.TempStore

// 创建临时文件
id, file, err := tempStore.CreateTempFile(ctx, "upload", ".tmp")
defer file.Close()

// 创建带内容的临时文件
id, err := tempStore.CreateTempFileWithContent(ctx, "data", ".json", jsonData)

// 读取临时文件
content, err := tempStore.GetTempFile(ctx, id)

// 设置自定义过期时间
err = tempStore.SetExpiration(ctx, id, 2*time.Hour)

// 列出所有临时文件
files, err := tempStore.ListTempFiles(ctx, "upload_*")

// 手动清理过期文件
count, err := tempStore.CleanupExpired(ctx)
```

## 🏗️ 架构特点

### 智能文件管理
- **唯一ID生成**: 使用加密随机数生成唯一标识符
- **文件名策略**: `prefix_id_suffix` 格式，便于识别和管理
- **状态恢复**: 启动时自动恢复已存在的临时文件记录

### 高效清理机制
- **后台协程**: 独立的清理协程，不阻塞主要业务
- **过期检查**: 访问时即时检查，确保不返回过期数据
- **批量清理**: 定时批量清理过期资源，减少磁盘I/O

### 并发安全
- **读写锁**: 保护临时文件记录的并发访问
- **原子操作**: 文件创建和删除操作的原子性
- **错误隔离**: 单个文件操作失败不影响其他操作

## 📁 目录结构

```
internal/core/infrastructure/storage/temp/
├── store.go        # 主要实现文件
└── README.md       # 本文档

internal/config/storage/temporary/
├── config.go       # 配置接口和实现
└── defaults.go     # 默认配置值
```

## 🔄 生命周期管理

### 文件生命周期
1. **创建**: 生成唯一ID，创建物理文件，记录元数据
2. **使用**: 支持多次读写操作，每次访问验证过期时间
3. **过期**: 超过TTL时间后，文件被标记为过期
4. **清理**: 定时清理任务或手动触发删除过期文件

### 自动清理策略
- **即时清理**: 访问过期文件时立即删除
- **定时清理**: 后台定时任务清理所有过期资源
- **启动恢复**: 应用启动时恢复现有临时文件状态
- **优雅关闭**: 关闭时执行最后一次清理

## ⚡ 性能特性

- **内存高效**: 只在内存中维护元数据，文件内容存储在磁盘
- **并发友好**: 支持多线程并发创建和访问临时文件
- **清理优化**: 批量清理减少系统调用开销
- **磁盘管理**: 文件数量和大小限制防止磁盘耗尽

## 🛡️ 安全考虑

- **权限隔离**: 临时文件使用独立的权限设置
- **路径安全**: 所有文件操作限制在配置的临时目录内
- **资源限制**: 文件大小和数量限制防止资源滥用
- **自动清理**: 防止临时文件长期占用磁盘空间
