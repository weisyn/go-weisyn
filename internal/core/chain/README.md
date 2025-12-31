# Chain 模块实现

---

## 📌 模块信息

- **版本**：1.0.0
- **状态**：🚧 部分完成
- **创建日期**：2025-11-01
- **模块名称**：chain
- **描述**：链状态管理模块，提供链尖更新和分叉处理能力

---

## 🎯 实施状态

### ✅ 已完成

#### 阶段1：接口定义
- ✅ 内部接口 `InternalChainWriter` (interfaces/writer.go)
- ✅ 内部接口 `InternalForkHandler` (interfaces/fork.go)
- ✅ 接口文档 (interfaces/README.md)

#### 阶段2：ChainWriter 服务
- ✅ 服务主文件 (writer/service.go)
- ✅ 同步状态更新 (writer/sync_status.go)
- ✅ 数据验证 (writer/validation.go)
- ⚠️ **注意**：链尖更新已移除，由 `persistence.DataWriter.WriteBlock()` 统一管理

#### 阶段3：ForkHandler 服务
- ✅ 分叉处理服务主文件 (fork/service.go)
- ✅ 分叉处理逻辑 (fork/handler.go)
- ✅ 分叉检测 (fork/detector.go)
- ✅ 链权重计算 (fork/weight.go)

#### 阶段4：依赖注入
- ✅ fx 模块配置 (module.go) - 已启用 ForkHandler

#### 阶段5：事件集成
- ✅ 事件订阅注册器 (integration/event/subscribe_handlers.go)
- ✅ 事件集成文档 (integration/event/README.md)

#### 阶段6：维护服务
- ✅ BlockFileGC 服务实现 (gc/blockfile_gc.go)
- ✅ Prometheus 指标集成 (gc/metrics.go)
- ✅ 运维接口（ManualRun, GetStatus）
- ✅ 配置系统集成
- ✅ DI 生命周期管理
- ✅ 单元测试 (gc/blockfile_gc_test.go)
- ✅ 文档完善 (gc/README.md)

### 🚧 待完成

#### 阶段6：测试
- ⏳ 单元测试
- ⏳ 集成测试
- ⏳ 性能测试

#### 阶段7：文档与清理
- ⏳ 各子目录 README
- ⏳ 使用示例
- ⏳ 清理旧代码

---

## 🔗 架构依赖关系

### 在依赖链中的位置

Chain 模块位于**核心业务层垂直依赖链的最高层（⑦）**：

```
┌──────────────┐
│  Chain       │ ← ⑦ 最高层（链管理）← 本模块
└──────┬───────┘
       ↓ 依赖
┌──────────────┐
│  Block       │ ← ⑥ 区块管理
└──────┬───────┘
       ↓ 依赖
┌──────────────┐
│  TX          │ ← ⑤ 交易处理
└──────┬───────┘
       ↓ 依赖
┌──────────────┐
│  EUTXO       │ ← ④ 状态管理
└──────────────┘
```

### 允许的依赖

✅ **允许依赖的下层模块**：
- `block.*` - 区块处理层（⑥）
- `persistence.QueryService` - 统一查询服务（读操作）
- `persistence.DataWriter` - 统一写入服务（写操作）

✅ **允许依赖的基础设施**：
- `storage.*` - 存储接口
- `event.*` - 事件总线
- `log.*` - 日志服务

### 禁止的依赖

❌ **禁止依赖的上层模块**：
- 无（Chain 是最高层）

❌ **禁止依赖的同层或反向依赖**：
- `tx.*` - 交易处理层（⑤，下层）
- `eutxo.*` - 状态管理层（④，下层）
- 其他业务层模块

**关键原则**：
- ✅ Chain 调用 Block，不直接调用 TX 或 EUTXO
- ✅ 读操作通过 `persistence.QueryService`
- ✅ 写操作通过 `persistence.DataWriter`
- ❌ 禁止反向依赖或循环依赖

> 📖 **详细架构分析**：参见 [../ARCHITECTURE_DEPENDENCY_ANALYSIS.md](../ARCHITECTURE_DEPENDENCY_ANALYSIS.md)

---

## 🏗️ 目录结构

```
internal/core/chain/
├── interfaces/                    # ✅ 内部接口定义
│   ├── writer.go                  # ✅ InternalChainWriter 接口
│   ├── fork.go                    # ✅ InternalForkHandler 接口
│   └── README.md                  # ✅ 接口文档
│
├── writer/                        # ✅ ChainWriter 服务实现
│   ├── service.go                 # ✅ 服务主文件
│   ├── sync_status.go             # ✅ 同步状态更新
│   └── validation.go              # ✅ 数据验证
│
├── fork/                          # ✅ ForkHandler 服务实现
│   ├── service.go                 # ✅ 服务主文件
│   ├── handler.go                 # ✅ 分叉处理逻辑
│   ├── detector.go                # ✅ 分叉检测
│   └── weight.go                  # ✅ 链权重计算
│
├── sync/                          # ✅ SystemSyncService 服务实现
│   ├── manager.go                 # ✅ 同步管理器
│   ├── trigger.go                 # ✅ 同步触发逻辑
│   └── ...                        # ✅ 其他同步相关文件
│
├── startup/                       # ✅ 启动流程包（新增）
│   └── genesis.go                 # ✅ 创世区块初始化逻辑
│
├── gc/                            # ✅ 块文件垃圾回收（维护服务）
│   ├── blockfile_gc.go            # ✅ GC 服务实现
│   ├── metrics.go                 # ✅ Prometheus 指标
│   ├── blockfile_gc_test.go       # ✅ 单元测试
│   └── README.md                  # ✅ GC 文档
│
├── integration/                   # ✅ 集成层
│   ├── event/                     # ✅ 事件集成
│   │   ├── subscribe_handlers.go  # ✅ 事件订阅注册器
│   │   └── README.md              # ✅ 事件集成文档
│   └── network/                   # ✅ 网络集成
│       ├── stream_handlers.go      # ✅ 网络流处理器
│       └── README.md              # ✅ 网络集成文档
│
├── interfaces/                    # ✅ 内部接口定义
│   ├── writer.go                  # ✅ InternalChainWriter 接口
│   ├── fork.go                    # ✅ InternalForkHandler 接口
│   ├── sync.go                    # ✅ InternalSyncService 接口
│   └── README.md                  # ✅ 接口文档
│
├── module.go                      # ✅ fx 模块定义
└── README.md                      # ✅ 本文档
```

---

## 🚀 快速开始

### 示例1：使用 ChainWriter 服务

```go
import (
    "context"
    "github.com/weisyn/v1/internal/core/chain"
    chainif "github.com/weisyn/v1/pkg/interfaces/chain"
    "github.com/weisyn/v1/pkg/types"
)

// 在 fx 应用中注入
func UseChainWriter(writer chainif.ChainWriter) {
    ctx := context.Background()
    
    // ⚠️ 注意：链尖更新已移除，由 persistence.DataWriter.WriteBlock() 统一管理
    // 所有区块相关数据（包括链尖）都通过 DataWriter 写入，确保原子性和一致性
    
    // 更新同步状态
    syncStatus := &types.SystemSyncStatus{
        Status:        types.SyncStatusSyncing,
        CurrentHeight: 1000,
        NetworkHeight: 1500,
        SyncProgress:  66.67,
    }
    err := writer.UpdateSyncStatus(ctx, syncStatus)
    if err != nil {
        log.Fatalf("更新同步状态失败: %v", err)
    }
}
```

### 示例2：使用 ForkHandler 服务

```go
// 在 fx 应用中注入
func UseForkHandler(handler chainif.ForkHandler) {
    ctx := context.Background()
    
    // 检测分叉
    isFork, forkHeight, err := handler.DetectFork(ctx, newBlock)
    if err != nil {
        log.Errorf("分叉检测失败: %v", err)
        return
    }
    
    if isFork {
        log.Infof("检测到分叉，分叉点高度: %d", forkHeight)
        
        // 处理分叉
        if err := handler.HandleFork(ctx, newBlock); err != nil {
            log.Errorf("分叉处理失败: %v", err)
            return
        }
    }
    
    // 获取活跃链
    chainInfo, err := handler.GetActiveChain(ctx)
    if err != nil {
        log.Errorf("获取活跃链失败: %v", err)
        return
    }
    log.Infof("当前活跃链高度: %d", chainInfo.CurrentHeight)
}
```

### 示例3：使用 BlockFileGC 服务

```go
import (
    "context"
    "github.com/weisyn/v1/internal/core/chain/gc"
)

// 手动触发 GC（覆盖配置中的 dry-run 设置）
func TriggerGC(gcService *gc.BlockFileGC) {
    ctx := context.Background()
    
    // 以 dry-run 模式运行（只检测不删除）
    dryRun := true
    result, err := gcService.ManualRun(ctx, &dryRun)
    if err != nil {
        log.Errorf("GC 失败: %v", err)
        return
    }
    
    log.Infof("GC 完成：扫描=%d 不可达=%d 删除=%d 回收=%d bytes",
        result.ScannedFiles, result.UnreachableFiles, 
        result.DeletedFiles, result.ReclaimedBytes)
}

// 查询 GC 状态
func CheckGCStatus(gcService *gc.BlockFileGC) {
    status := gcService.GetStatus()
    log.Infof("GC 状态：enabled=%v running=%v last_run=%v",
        status.Enabled, status.Running, status.LastRunTime)
    
    if status.LastRunResult != nil {
        log.Infof("上次运行：扫描=%d 删除=%d",
            status.LastRunResult.ScannedFiles, 
            status.LastRunResult.DeletedFiles)
    }
}
```

### 监控指标

```go
import (
    "github.com/weisyn/v1/internal/core/chain/interfaces"
)

// 获取写入指标
func MonitorMetrics(writer interfaces.InternalChainWriter) {
    metrics, err := writer.GetWriterMetrics(ctx)
    if err != nil {
        log.Errorf("获取指标失败: %v", err)
        return
    }
    
    // 打印指标
    log.Printf("更新次数: %d", metrics.UpdateCount)
    log.Printf("成功率: %.2f%%", 
        float64(metrics.SuccessCount)/float64(metrics.UpdateCount)*100)
    log.Printf("平均耗时: %.2fms", metrics.AverageUpdateTime*1000)
    log.Printf("最大耗时: %.2fms", metrics.MaxUpdateTime*1000)
    log.Printf("当前高度: %d", metrics.CurrentHeight)
    log.Printf("健康状态: %v", metrics.IsHealthy)
}
```

### 数据验证

```go
// 验证链尖数据一致性
func ValidateData(writer interfaces.InternalChainWriter) {
    err := writer.ValidateChainTip(ctx)
    if err != nil {
        log.Errorf("数据验证失败: %v", err)
        // 尝试修复
        if writerImpl, ok := writer.(*writer.Service); ok {
            if err := writerImpl.RepairChainTip(ctx); err != nil {
                log.Fatalf("修复失败: %v", err)
            }
        }
    }
}
```

---

## 🔧 配置说明

### fx 模块集成

在应用中集成 chain 模块：

```go
package main

import (
    "go.uber.org/fx"
    "github.com/weisyn/v1/internal/core/chain"
    // ... 其他导入
)

func main() {
    app := fx.New(
        // 基础设施模块
        storage.Module(),
        log.Module(),
        
        // Chain 模块
        chain.Module(),
        
        // 其他模块...
    )
    
    app.Run()
}
```

### 依赖要求

Chain 模块需要以下依赖：

| 依赖 | 类型 | 必需 | 说明 |
|-----|------|-----|------|
| `storage.Storage` | 接口 | ✅ | 持久化存储服务（ChainWriter需要） |
| `query.QueryService` | 接口 | ✅ | 查询服务（ForkHandler需要） |
| `log.Logger` | 接口 | ❌ | 日志服务（可选，推荐使用） |
| `event.EventBus` | 接口 | ❌ | 事件总线（可选，启用事件驱动） |
| `storage.BadgerStore` | 接口 | ❌ | BadgerStore（BlockFileGC需要） |
| `storage.FileStore` | 接口 | ❌ | FileStore（BlockFileGC需要） |

### BlockFileGC 配置

BlockFileGC 是 chain 模块的后台维护服务，用于清理不可达的块文件。在 `blockchain` 配置中添加：

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

**配置说明**：

| 配置项 | 默认值 | 说明 |
|-------|-------|------|
| `enabled` | `false` | 是否启用 GC |
| `dry_run` | `true` | dry-run 模式（只检测不删除） |
| `interval_seconds` | `3600` | 自动运行间隔（秒） |
| `rate_limit_files_per_sec` | `100` | 限速（文件/秒） |
| `protect_recent_height` | `1000` | 保护最近 N 个区块 |

**监控指标**：

BlockFileGC 导出以下 Prometheus 指标：
- `weisyn_chain_gc_runs_total` - GC 运行次数
- `weisyn_chain_gc_scanned_files_total` - 扫描文件总数
- `weisyn_chain_gc_deleted_files_total` - 删除文件总数
- `weisyn_chain_gc_reclaimed_bytes_total` - 回收字节总数
- `weisyn_chain_gc_duration_seconds` - GC 运行耗时
- `weisyn_chain_gc_running` - 当前运行状态

详细文档：[gc/README.md](./gc/README.md)

---

## 📊 性能指标

### ChainWriter 性能目标

| 操作 | 目标延迟 | 吞吐量 | 当前状态 |
|-----|---------|--------|---------|
| UpdateSyncStatus | < 5ms | > 200 TPS | ✅ 已实现 |
| GetWriterMetrics | < 1ms | > 1000 QPS | ✅ 已实现 |
| ValidateChainTip | < 20ms | > 50 QPS | ✅ 已实现 |

**注意**：链尖更新已移除，由 `persistence.DataWriter.WriteBlock()` 统一管理，确保原子性和一致性。

### 并发安全

- ✅ 使用读写锁保护状态更新
- ✅ 支持并发读取
- ✅ 原子批量写入
- ✅ 无数据竞争

---

## 🧪 测试

### 运行测试

```bash
# 运行所有测试
go test ./internal/core/chain/...

# 运行特定包的测试
go test ./internal/core/chain/writer

# 运行基准测试
go test -bench=. ./internal/core/chain/writer

# 查看覆盖率
go test -cover ./internal/core/chain/...
```

### 测试状态

| 测试类型 | 状态 | 覆盖率 |
|---------|------|--------|
| 单元测试 | ⏳ 待编写 | - |
| 集成测试 | ⏳ 待编写 | - |
| 性能测试 | ⏳ 待编写 | - |

---

## 📚 相关文档

- [实施计划](./IMPLEMENTATION_PLAN.md) - 详细的分步实施计划
- [技术设计](./TECHNICAL_DESIGN.md) - 技术设计详细文档
- [公共接口设计](../../../docs/system/designs/interfaces/public-interface-design.md) - 接口设计规范
- [Chain 组件文档](../../../docs/components/core/chain/README.md) - 组件级接口文档
- [接口 README](../../pkg/interfaces/chain/README.md) - 公共接口文档

---

## 🔄 下一步建议

Chain 模块的核心功能已经完成！建议的后续工作：

1. **编写测试用例**（重要）
   - ChainWriter 单元测试
   - ForkHandler 单元测试
   - 集成测试
   - 性能基准测试

2. **实际集成测试**（推荐）
   - 将 Chain 模块集成到主应用
   - 测试与其他模块的协作
   - 验证依赖注入配置

3. **可选优化**
   - 实现事件集成（integration/event/）
   - 添加更多指标收集
   - 性能优化

4. **文档完善**（可选）
   - 各子目录详细 README
   - 更多使用示例
   - 故障排查指南

---

## 💬 反馈与贡献

如有问题或建议，请联系：
- **负责人**：WES Chain 开发组
- **状态跟踪**：查看 `IMPLEMENTATION_PLAN.md` 中的时间表

---

**最后更新**：2025-11-01

