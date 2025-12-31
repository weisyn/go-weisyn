# CLI 代码迁移通知

> **迁移日期**: 2025-01-XX  
> **状态**: ✅ 已完成

## 📋 迁移说明

`client/cli/` 目录下的所有 CLI 命令实现已迁移到 `cmd/cli/` 目录。

### 迁移内容

- ✅ 所有命令文件（17个）已从 `client/cli/` 迁移到 `cmd/cli/`
- ✅ CLI 入口点现在位于 `cmd/cli/main.go`
- ✅ 包名从 `package cmd` 改为 `package main`（统一在同一包中）

### 目录变更

**旧位置**:
```
client/cli/
├── root.go
├── account.go
├── chain.go
└── ... (其他命令文件)
```

**新位置**:
```
cmd/cli/
├── main.go          # 新增：CLI 入口点
├── root.go
├── account.go
├── chain.go
└── ... (其他命令文件)
```

### 依赖关系

- ✅ `cmd/cli/` 继续使用 `client/core/` 作为业务层支持库
- ✅ `cmd/cli/` 继续使用 `client/pkg/` 作为公共库
- ✅ `client/core/` 和 `client/pkg/` 保持不变，继续提供服务

### 构建方式

**旧方式**（已废弃）:
```bash
# 不再支持
go build -o bin/wes ./client/cli
```

**新方式**:
```bash
# 推荐方式
make build-cli
# 或
go build -o bin/weisyn-cli ./cmd/cli
```

### 影响范围

- ✅ **无代码影响**: 没有其他代码引用 `client/cli` 包
- ✅ **无功能影响**: 所有功能保持不变
- ✅ **向后兼容**: CLI 使用方式不变，只是构建路径改变

### 清理说明

`client/cli/` 目录已删除，避免代码重复。

如果需要在历史提交中查看旧代码，可以通过 Git 历史访问：
```bash
git log --all --full-history -- client/cli/
```

## 📚 相关文档

- **[cmd/cli/README.md](../cmd/cli/README.md)** - CLI 使用文档
- **[cmd/CLI_MIGRATION_SUMMARY.md](../cmd/CLI_MIGRATION_SUMMARY.md)** - 迁移详细总结

