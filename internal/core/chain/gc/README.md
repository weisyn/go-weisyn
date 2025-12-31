# Block File GC（块文件垃圾回收）

## 📌 概述

BlockFileGC 是 chain 模块的后台维护服务，用于清理 `blocks/` 目录中的不可达块文件（fork 后的旧链残留）。

## 🎯 设计目标

1. **自动清理**：定期扫描并删除不可达的块文件
2. **安全保护**：保护最近 N 个区块，避免误删
3. **可配置**：支持启用/禁用、间隔、限速等配置
4. **可观测**：提供指标、日志、运维接口

## 📋 工作原理

### Mark-Sweep 算法

1. **Mark（标记）**：扫描 `indices:height` 索引，构建可达区块集合
2. **Sweep（清除）**：扫描 `blocks/` 目录，删除不在可达集合中的文件

### 保护机制

- **保护窗口**：最近 `protect_recent_height` 个区块不会被删除
- **Dry-run 模式**：只检测不删除，用于验证
- **限速**：每秒最多处理 `rate_limit_files_per_sec` 个文件，避免 I/O 压力

## 🔧 配置

### 配置文件

在 `blockchain` 配置中添加 `block_file_gc` 配置段：

```json
{
  "blockchain": {
    "block_file_gc": {
      "enabled": false,
      "dry_run": true,
      "interval_seconds": 3600,
      "rate_limit_files_per_sec": 100,
      "protect_recent_height": 1000
    }
  }
}
```

### 配置说明

| 配置项 | 默认值 | 说明 |
|-------|-------|------|
| `enabled` | `false` | 是否启用 GC |
| `dry_run` | `true` | dry-run 模式（只检测不删除） |
| `interval_seconds` | `3600` | 自动运行间隔（秒） |
| `rate_limit_files_per_sec` | `100` | 限速（文件/秒） |
| `protect_recent_height` | `1000` | 保护最近 N 个区块 |

## 📊 监控指标

BlockFileGC 导出以下 Prometheus 指标：

| 指标 | 类型 | 说明 |
|-----|------|------|
| `weisyn_chain_gc_runs_total` | Counter | GC 运行次数 |
| `weisyn_chain_gc_scanned_files_total` | Counter | 扫描文件总数 |
| `weisyn_chain_gc_deleted_files_total` | Counter | 删除文件总数 |
| `weisyn_chain_gc_reclaimed_bytes_total` | Counter | 回收字节总数 |
| `weisyn_chain_gc_duration_seconds` | Histogram | GC 运行耗时 |
| `weisyn_chain_gc_running` | Gauge | 当前运行状态 |

## 🛠️ 运维操作

### 手动触发 GC

通过代码调用：

```go
result, err := gcService.ManualRun(ctx, &dryRun)
if err != nil {
    log.Errorf("GC failed: %v", err)
}
log.Infof("GC completed: scanned=%d unreachable=%d deleted=%d reclaimed=%d bytes",
    result.ScannedFiles, result.UnreachableFiles, result.DeletedFiles, result.ReclaimedBytes)
```

### 查询 GC 状态

```go
status := gcService.GetStatus()
log.Infof("GC status: enabled=%v running=%v last_run=%v",
    status.Enabled, status.Running, status.LastRunTime)
```

## ⚠️ 注意事项

1. **生产环境建议**：
   - 初次启用时使用 `dry_run: true` 验证
   - 确认无误后再设置 `dry_run: false`
   - 监控 GC 运行日志和指标

2. **磁盘空间**：
   - GC 仅清理不可达文件，不会影响当前链
   - 如果磁盘空间紧张，可以手动触发 GC

3. **I/O 影响**：
   - GC 运行时会扫描大量文件，可能产生 I/O 压力
   - 使用 `rate_limit_files_per_sec` 控制速率

4. **REORG 期间**：
   - GC 会自动保护最近 N 个区块
   - 保护窗口应设置为大于最大 REORG 深度

## 🏗️ 架构集成

BlockFileGC 已集成到 chain 模块的 DI 系统：

```
chain/module.go
  └── ProvideBlockFileGC()
       ├── 读取配置
       ├── 创建 GC 服务
       └── 注册生命周期 Hook
            ├── OnStart: gcService.Start()
            └── OnStop: gcService.Stop()
```

### 依赖关系

- **输入依赖**：
  - `config.Provider`: 获取配置
  - `log.Logger`: 日志记录
  - `storage.BadgerStore`: 读取索引数据
  - `storage.FileStore`: 文件操作

- **生命周期**：
  - 随 chain 模块启动（如果 `enabled: true`）
  - 随 chain 模块停止

## 📚 相关文档

- [10-BlockFileGC架构优化建议.md](/_dev/14-实施任务-implementation-tasks/20251215-16-defect-reports-summary/10-BlockFileGC架构优化建议.md)
- [REORG_IMPLEMENTATION.md](/_dev/03-实现蓝图-implementation/03-区块与链实现-block-and-chain/06-REORG_IMPLEMENTATION.md)
- [REORG_AND_REVERSIBILITY.md](/_dev/02-架构设计-architecture/04-区块与链架构-block-and-chain/05-REORG_AND_REVERSIBILITY.md)

## 🧪 测试

### 单元测试

```bash
go test ./internal/core/chain/gc/... -v
```

### 集成测试

```bash
go test ./internal/core/chain/gc/... -v -tags=integration
```

## 📝 实施记录

- **创建时间**：2024-12
- **架构优化**：集成到 chain 模块 DI 系统
- **监控集成**：Prometheus 指标导出
- **运维接口**：ManualRun, GetStatus

