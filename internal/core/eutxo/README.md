# EUTXO 模块

---

## 📌 简介

EUTXO（扩展UTXO）模块是 WES 区块链的核心模块之一，负责 UTXO 状态的写入、快照管理。

### 核心价值

- ✅ **UTXO 写入**：创建、删除、引用计数管理
- ✅ **状态根管理**：维护 UTXO 集合的 Merkle 根
- ✅ **快照管理**：快照创建、恢复、删除
- ✅ **CQRS 架构**：读写分离，写操作在此模块

---

## 🏗️ 架构设计

### 架构依赖关系

#### 在依赖链中的位置

EUTXO 模块位于**核心业务层垂直依赖链的第④层**：

```
┌──────────────┐
│  Chain       │ ← ⑦ 最高层（链管理）
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
│  EUTXO       │ ← ④ 状态管理 ← 本模块
└──────┬───────┘
       ↓ 依赖
┌──────────────┐
│  URES        │ ← ③ 资源管理
└──────────────┘
```

#### 允许的依赖

✅ **允许依赖的下层模块**：
- `ures.*` - 资源管理层（③）
- `persistence.QueryService` - 统一查询服务（读操作）

✅ **允许依赖的基础设施**：
- `storage.*` - 存储接口（用于业务逻辑，非持久化）
- `event.*` - 事件总线
- `log.*` - 日志服务
- `crypto.*` - 密码学服务（状态根计算）

#### 禁止的依赖

❌ **禁止依赖的上层模块**：
- `chain.*` - 链管理层（⑦，上层）
- `block.*` - 区块管理层（⑥，上层）
- `tx.*` - 交易处理层（⑤，上层）

❌ **禁止依赖 Persistence 的写入功能**：
- `persistence.DataWriter` - EUTXO 模块不应该直接调用 DataWriter
- 持久化操作应由业务层（如 BlockProcessor）通过 DataWriter 完成

**关键原则**：
- ✅ EUTXO 的 `UTXOWriter` 用于业务逻辑中的 UTXO 操作（如引用计数管理、状态根更新）
- ❌ EUTXO 的 `UTXOWriter` **不应该**用于持久化操作
- ✅ 持久化操作由 `persistence.DataWriter` 完成（在 persistence 组件内部实现）
- ✅ 读操作通过 `persistence.QueryService`

> ⚠️ **重要说明**：`eutxo.UTXOWriter` 和 `persistence/writer/utxo.go` 中的领域 Writer 是两个不同的概念：
> - `eutxo.UTXOWriter`：业务层接口，用于业务逻辑中的 UTXO 操作
> - `persistence/writer/utxo.go`：persistence 组件内部的实现，用于直接操作存储

> 📖 **详细架构分析**：参见 [../ARCHITECTURE_DEPENDENCY_ANALYSIS.md](../ARCHITECTURE_DEPENDENCY_ANALYSIS.md)

### 三层架构

```
pkg/interfaces/eutxo (公共接口)
    ↓ 继承
internal/core/eutxo/interfaces (内部接口)
    ↓ 实现
internal/core/eutxo/{writer,snapshot} (服务实现)
```

### 服务清单

| 服务 | 公共接口 | 内部接口 | 实现 | 状态 |
|-----|---------|---------|------|------|
| **UTXOWriter** | `eutxo.UTXOWriter` | `InternalUTXOWriter` | `writer/` | ✅ 完成 |
| **UTXOSnapshot** | `eutxo.UTXOSnapshot` | `InternalUTXOSnapshot` | `snapshot/` | ✅ 完成 |

---

## 📦 目录结构

```
internal/core/eutxo/
├── MODULE_ASSESSMENT.md           # 模块评估报告
├── IMPLEMENTATION_PLAN.md         # 实施计划
├── TECHNICAL_DESIGN.md            # 技术设计
├── README.md                      # 本文件
│
├── interfaces/                    # 内部接口层
│   ├── writer.go                  # InternalUTXOWriter 接口
│   ├── snapshot.go                # InternalUTXOSnapshot 接口
│   ├── query.go                   # InternalUTXOQuery 接口（内部使用）
│   └── README.md                  # 接口文档
│
├── writer/                        # UTXOWriter 服务实现
│   ├── service.go                 # 服务主文件
│   ├── operations.go              # UTXO 创建/删除
│   ├── reference.go               # 引用计数管理
│   ├── state_root.go              # 状态根管理
│   └── validation.go              # 数据验证
│
├── snapshot/                      # UTXOSnapshot 服务实现
│   ├── service.go                 # 服务主文件
│   ├── create.go                  # 快照创建
│   ├── restore.go                 # 快照恢复
│   └── manage.go                  # 快照管理
│
├── shared/                        # 共享工具
│   ├── cache.go                   # 缓存管理
│   └── index.go                   # 索引管理
│
└── module.go                      # fx 模块定义
```

---

## 🚀 使用示例

### 集成到主应用

```go
import (
    "go.uber.org/fx"
    "github.com/weisyn/v1/internal/core/eutxo"
    eutxoif "github.com/weisyn/v1/pkg/interfaces/eutxo"
)

app := fx.New(
    storage.Module(),        // 提供 Storage
    crypto.Module(),         // 提供 HashManager
    event.Module(),          // 提供 EventBus（可选）
    log.Module(),            // 提供 Logger（可选）
    eutxo.Module(),          // ✅ 添加 EUTXO 模块

    fx.Invoke(func(
        writer eutxoif.UTXOWriter,
        snapshot eutxoif.UTXOSnapshot,
    ) {
        // 使用 UTXOWriter
        // 使用 UTXOSnapshot
    }),
)
```

### 使用 UTXOWriter

```go
// 创建 UTXO
err := utxoWriter.CreateUTXO(ctx, utxoObj)

// 删除 UTXO
err = utxoWriter.DeleteUTXO(ctx, outpoint)

// 引用 UTXO（资源UTXO）
err = utxoWriter.ReferenceUTXO(ctx, outpoint)

// 解除引用
err = utxoWriter.UnreferenceUTXO(ctx, outpoint)

// 更新状态根
err = utxoWriter.UpdateStateRoot(ctx, stateRoot)
```

### 使用 UTXOSnapshot

```go
// 创建快照
snapshot, err := utxoSnapshot.CreateSnapshot(ctx, height)

// 恢复快照
err = utxoSnapshot.RestoreSnapshot(ctx, snapshot)

// 删除快照
err = utxoSnapshot.DeleteSnapshot(ctx, snapshotID)

// 列出快照
snapshots, err := utxoSnapshot.ListSnapshots(ctx)
```

---

## 📊 实施状态

### 已完成阶段

| 阶段 | 内容 | 状态 |
|------|------|------|
| **阶段0** | 规划文档 | ✅ 完成 |
| **阶段1** | 基础目录和接口 | ✅ 完成 |
| **阶段2** | UTXOWriter 服务 | ✅ 完成 |
| **阶段3** | UTXOSnapshot 服务 | ✅ 完成 |
| **阶段4** | fx 依赖注入 | ✅ 完成 |
| **阶段5** | 集成到 Block/Chain | ⏳ 待完成 |
| **阶段6** | 测试与文档 | ⏳ 待完成 |

### 代码质量

- ✅ **零 linter 错误**
- ✅ **完整的中文注释**
- ✅ **符合设计规范**
- ✅ **遵循 CQRS 架构**

### 核心功能

| 功能 | 状态 | 说明 |
|-----|------|------|
| UTXO 创建 | ✅ 完成 | 基本实现，需完善事件发布 |
| UTXO 删除 | ✅ 完成 | 基本实现，需完善事件发布 |
| 引用计数 | ✅ 完成 | 支持引用不消费模式 |
| 状态根更新 | ✅ 完成 | 基本实现，需完善 Merkle 计算 |
| 快照创建 | ✅ 框架 | 核心框架完成，需完善逻辑 |
| 快照恢复 | ✅ 框架 | 核心框架完成，需完善逻辑 |
| 快照管理 | ✅ 框架 | 核心框架完成，需完善逻辑 |

---

## ⚠️ 待完善项

### 高优先级

1. **完善 Query 接口**：实现 `InternalUTXOQuery`，支持快照创建
2. **完善快照逻辑**：实现完整的序列化、压缩、存储逻辑
3. **集成到 Block/Chain**：更新 Block.Processor 和 Chain.ForkHandler

### 中优先级

4. **事件发布**：完善 UTXO 变更事件发布
5. **状态根计算**：实现基于 Merkle 树的状态根计算
6. **单元测试**：编写完整的单元测试

### 低优先级

7. **索引管理**：完善 UTXO 索引维护
8. **性能优化**：优化缓存策略和批量操作
9. **文档完善**：补充使用示例和故障排查指南

---

## 🔧 **实施历史**

### 模块拆分与重构

**背景**：
- EUTXO 模块从 `internal/core/repositories/utxo` 重构为独立的核心模块
- 遵循 CQRS 架构原则，实现读写分离
- 遵循三层架构：公共接口 → 内部接口 → 服务实现

**核心设计决策**：

1. **CQRS 架构**：
   - 写操作：EUTXO 模块（UTXOWriter、UTXOSnapshot）
   - 读操作：Query 模块（UTXOQuery）
   - 持久化：Persistence 模块（DataWriter）

2. **职责边界**：
   - EUTXO 的 `UTXOWriter` 用于业务逻辑中的 UTXO 操作（引用计数管理、状态根更新）
   - EUTXO 的 `UTXOWriter` **不应该**用于持久化操作
   - 持久化操作由 `persistence.DataWriter` 完成

3. **三层输出架构**：
   - AssetOutput：价值载体
   - ResourceOutput：能力载体（与 URES 模块协作）
   - StateOutput：证据载体

**实施成果**：
- ✅ UTXOWriter 服务：UTXO 写入、引用计数、状态根管理
- ✅ UTXOSnapshot 服务：快照创建、恢复、管理
- ✅ 完整的依赖注入配置
- ✅ 符合代码组织规范

> 📖 **详细设计文档**：设计内容已整合到代码实现中，详见各服务实现文件。

---

## 📚 相关文档

- [模块评估报告](./MODULE_ASSESSMENT.md)
- [实施计划](./IMPLEMENTATION_PLAN.md)
- [技术设计文档](./TECHNICAL_DESIGN.md)
- [内部接口文档](./interfaces/README.md)
- [公共接口文档](../../../pkg/interfaces/eutxo/README.md)

---

## 🎊 总结

**EUTXO 模块核心功能已完成！** 🚀

- ✅ 架构清晰，遵循 CQRS 原则
- ✅ 接口定义完整，文档齐全
- ✅ 核心服务实现，可编译运行
- ✅ fx 依赖注入配置，可集成使用

**下一步**：集成到 Block/Chain 模块，编写测试用例。

