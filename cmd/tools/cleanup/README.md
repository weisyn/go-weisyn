# 🧹 数据清理工具 (Cleanup)

> **工具功能**: 清理 WES 区块链数据目录和临时文件

## 📋 快速开始

```bash
# 预览模式（推荐第一步）
go run ./cmd/tools/cleanup --dry-run

# 交互式删除
go run ./cmd/tools/cleanup

# 直接删除（跳过确认）
go run ./cmd/tools/cleanup --yes
```

## 功能说明

`cleanup` 工具用于清理 WES 区块链系统产生的数据文件和临时文件，包括：

- ✅ 区块链数据库文件
- ✅ BadgerDB 存储数据
- ✅ 临时配置文件
- ✅ 日志文件（可选）

### 主要特性

1. **自动查找**: 自动查找所有数据目录和临时文件
2. **预览模式**: 支持 `--dry-run` 模式，仅显示将要删除的文件
3. **交互确认**: 默认需要用户确认，防止误删
4. **显示大小**: 显示每个目录的占用空间
5. **安全删除**: 使用 Go 标准库安全删除文件

## 使用方法

### 基本用法

```bash
# 方式1: 使用 go run（开发环境）
go run ./cmd/tools/cleanup

# 方式2: 编译后运行
go build -o bin/wes-cleanup ./cmd/tools/cleanup
./bin/wes-cleanup
```

### 命令行参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--dry-run` | 预览模式，不实际删除文件 | false |
| `--yes` | 跳过确认提示，直接删除 | false |
| `--help` | 显示帮助信息 | - |

## 清理目标

工具会自动查找并清理以下内容：

- `./data` - 主数据目录
- `./data/badger` - BadgerDB 数据库
- `./internal/core/infrastructure/storage/badger/data` - 存储层数据
- `./config-temp` - 临时配置目录

## 安全提示

### ⚠️ 重要警告

1. **数据不可恢复**: 删除的数据无法恢复，请务必谨慎操作
2. **生产环境**: 在生产环境使用前，**必须先备份数据**
3. **确认环境**: 确保你在正确的工作目录中运行工具
4. **停止服务**: 清理前应该先停止运行的节点服务

## 相关文档

- **[tools/README.md](../README.md)** - 工具集总览
- **[node/README.md](../../node/README.md)** - 节点启动文档

