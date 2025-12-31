# EUTXO 内部接口层

---

## 📌 简介

本目录定义 EUTXO 模块的**内部接口层**，作为公共接口和服务实现之间的桥梁。

### 架构定位

```
pkg/interfaces/eutxo (公共接口)
    ↓ 继承
internal/core/eutxo/interfaces (内部接口) ← 本目录
    ↓ 实现
internal/core/eutxo/{writer,snapshot} (服务实现)
```

---

## 🎯 设计理念

### 1. 嵌入式继承

**原则**：内部接口通过嵌入公共接口，继承所有公共方法

**优势**：
- ✅ 自动继承公共方法，不需要重复声明
- ✅ 对外暴露公共接口，对内使用内部接口
- ✅ 扩展内部方法，不影响公共接口稳定性

**示例**：
```go
type InternalUTXOWriter interface {
    eutxo.UTXOWriter // 嵌入公共接口

    // 内部扩展方法
    GetWriterMetrics(ctx context.Context) (*WriterMetrics, error)
    ValidateUTXO(ctx context.Context, utxoObj *utxo.UTXO) error
}
```

---

### 2. 内部扩展

**原则**：内部接口添加系统内部需要的管理方法

**扩展类型**：
- 📊 **指标接口**：`GetWriterMetrics`、`GetSnapshotMetrics`
- ✅ **验证接口**：`ValidateUTXO`、`ValidateSnapshot`
- 🔗 **延迟注入**：`SetWriter`、`SetQuery`

**用途**：
- 监控系统收集性能指标
- 调试工具分析行为
- 告警系统检测异常
- 内部验证和协调

---

### 3. 接口隔离

**原则**：内部接口不对外暴露，只在系统内部使用

**实现方式**：
- ✅ 通过 `fx.As(new(eutxo.UTXOWriter))` 导出公共接口
- ✅ 通过 `fx.As(new(interfaces.InternalUTXOWriter))` 导出内部接口
- ✅ 内部接口只在核心模块中注入

**好处**：
- 对外稳定：公共接口稳定，不受内部变更影响
- 内部灵活：内部接口可以自由扩展，不影响外部
- 职责分离：公共方法对外，内部方法对内

---

## 📦 接口清单

### 1. InternalUTXOWriter

**文件**：`writer.go`

**职责**：
- 继承 `eutxo.UTXOWriter` 的所有方法
- 提供写入服务指标（`GetWriterMetrics`）
- 提供 UTXO 验证（`ValidateUTXO`）

**使用场景**：
- Block.Processor 更新 UTXO 状态
- TX.Processor 处理交易输出
- UTXOSnapshot 恢复快照
- 监控系统收集指标

**公共方法**（继承自 `eutxo.UTXOWriter`）：
```go
CreateUTXO(ctx, utxoObj) error
DeleteUTXO(ctx, outpoint) error
ReferenceUTXO(ctx, outpoint) error
UnreferenceUTXO(ctx, outpoint) error
UpdateStateRoot(ctx, stateRoot) error
```

**内部方法**：
```go
GetWriterMetrics(ctx) (*WriterMetrics, error)
ValidateUTXO(ctx, utxoObj) error
```

---

### 2. InternalUTXOSnapshot

**文件**：`snapshot.go`

**职责**：
- 继承 `eutxo.UTXOSnapshot` 的所有方法
- 提供快照服务指标（`GetSnapshotMetrics`）
- 提供快照验证（`ValidateSnapshot`）
- 支持延迟依赖注入（`SetWriter`、`SetQuery`）

**使用场景**：
- Chain.ForkHandler 分叉处理
- Blockchain.SyncService 同步过程
- 监控系统收集快照指标

**公共方法**（继承自 `eutxo.UTXOSnapshot`）：
```go
CreateSnapshot(ctx, height) (*types.UTXOSnapshotData, error)
RestoreSnapshot(ctx, snapshot) error
DeleteSnapshot(ctx, snapshotID) error
ListSnapshots(ctx) ([]*types.UTXOSnapshotData, error)
```

**内部方法**：
```go
GetSnapshotMetrics(ctx) (*SnapshotMetrics, error)
ValidateSnapshot(ctx, snapshot) error
SetWriter(writer InternalUTXOWriter)
SetQuery(query InternalUTXOQuery)
```

**延迟注入说明**：
- `SetWriter`：注入 UTXOWriter，用于快照恢复
- `SetQuery`：注入 UTXOQuery，用于快照创建
- 目的：避免循环依赖
- 时机：在 fx.Invoke 中注入，所有服务创建后

---

### 3. InternalUTXOQuery

**文件**：`query.go`

**职责**：
- 提供内部 UTXO 查询方法
- 仅供 EUTXO 模块内部使用
- 不对外暴露，避免与 QueryService 冲突

**使用场景**：
- UTXOSnapshot.CreateSnapshot 查询所有 UTXO
- UTXOWriter.ReferenceUTXO 查询引用计数
- 内部验证和状态查询

**方法**：
```go
GetUTXO(ctx, outpoint) (*utxo.UTXO, error)
ListUTXOs(ctx, height) ([]*utxo.UTXO, error)
GetReferenceCount(ctx, outpoint) (uint64, error)
```

**重要说明**：
- ⚠️ 仅供内部使用，不对外暴露
- ⚠️ 不与 QueryService.UTXOQuery 冲突
- ⚠️ 后续 Query 模块实施时会迁移

---

## 📊 指标数据结构

### WriterMetrics - 写入服务指标

**用途**：监控 UTXOWriter 服务性能

**字段**：
```go
type WriterMetrics struct {
    // 统计指标
    CreateCount      uint64  // 创建次数
    DeleteCount      uint64  // 删除次数
    ReferenceCount   uint64  // 引用次数
    UnreferenceCount uint64  // 解除引用次数
    StateRootUpdates uint64  // 状态根更新次数

    // 性能指标
    AverageCreateTime float64 // 平均创建耗时（秒）
    AverageDeleteTime float64 // 平均删除耗时（秒）

    // 缓存指标
    CacheSize    int     // 当前缓存 UTXO 数量
    CacheHitRate float64 // 缓存命中率

    // 状态指标
    IsHealthy    bool   // 健康状态
    ErrorMessage string // 错误信息
}
```

---

### SnapshotMetrics - 快照服务指标

**用途**：监控 UTXOSnapshot 服务性能

**字段**：
```go
type SnapshotMetrics struct {
    // 统计指标
    CreateCount    uint64 // 创建次数
    RestoreCount   uint64 // 恢复次数
    DeleteCount    uint64 // 删除次数
    TotalSnapshots int    // 总快照数

    // 性能指标
    AverageCreateTime  float64 // 平均创建耗时（秒）
    AverageRestoreTime float64 // 平均恢复耗时（秒）
    TotalSize          int64   // 总大小（字节）

    // 状态指标
    IsHealthy    bool   // 健康状态
    ErrorMessage string // 错误信息
}
```

---

## 🔗 依赖关系

### 接口依赖图

```
InternalUTXOSnapshot
    ├─> InternalUTXOWriter (延迟注入，用于快照恢复)
    └─> InternalUTXOQuery (延迟注入，用于快照创建)

InternalUTXOWriter
    └─> (无依赖)

InternalUTXOQuery
    └─> (无依赖)
```

**关键点**：
- ✅ UTXOSnapshot 依赖 UTXOWriter 和 UTXOQuery（单向）
- ✅ UTXOWriter 和 UTXOQuery 独立，无依赖
- ✅ 通过延迟注入避免循环依赖

---

## 🚀 使用示例

### 示例1：实现 InternalUTXOWriter

```go
package writer

import (
    "github.com/weisyn/v1/internal/core/eutxo/interfaces"
)

type Service struct {
    // ...
}

// 实现公共方法（继承自 eutxo.UTXOWriter）
func (s *Service) CreateUTXO(ctx context.Context, utxoObj *utxo.UTXO) error {
    // ...
}

// 实现内部方法
func (s *Service) GetWriterMetrics(ctx context.Context) (*interfaces.WriterMetrics, error) {
    return s.metrics, nil
}

func (s *Service) ValidateUTXO(ctx context.Context, utxoObj *utxo.UTXO) error {
    // 验证逻辑
}

// 编译时检查接口实现
var _ interfaces.InternalUTXOWriter = (*Service)(nil)
```

---

### 示例2：在 fx 中注册服务

```go
fx.Provide(
    fx.Annotate(
        func(storage storage.Storage, hasher crypto.HashManager) (interfaces.InternalUTXOWriter, error) {
            return writer.NewService(storage, hasher, nil, nil)
        },
        // 导出为公共接口（供外部模块使用）
        fx.As(new(eutxo.UTXOWriter)),
        // 导出为内部接口（供内部模块使用）
        fx.As(new(interfaces.InternalUTXOWriter)),
        fx.ResultTags(`name:"utxo_writer"`),
    ),
)
```

---

### 示例3：延迟注入依赖

```go
fx.Invoke(
    func(
        snapshot interfaces.InternalUTXOSnapshot,
        writer interfaces.InternalUTXOWriter,
        query interfaces.InternalUTXOQuery,
    ) {
        if snapshotService, ok := snapshot.(*snapshot.Service); ok {
            snapshotService.SetWriter(writer)
            snapshotService.SetQuery(query)
        }
    },
)
```

---

## 📚 相关文档

- [EUTXO 模块总览](../README.md)
- [技术设计文档](../TECHNICAL_DESIGN.md)
- [实施计划](../IMPLEMENTATION_PLAN.md)
- [公共接口文档](../../../../pkg/interfaces/eutxo/README.md)

---

## ✅ 验收标准

- ✅ 所有内部接口定义清晰
- ✅ 接口继承关系正确
- ✅ 指标数据结构完整
- ✅ 延迟注入机制设计合理
- ✅ 文档说明详细
- ✅ 无 linter 错误

