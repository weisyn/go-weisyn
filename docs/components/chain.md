# Chain 组件能力视图

---

## 🎯 组件定位

Chain 组件是 WES 系统的链状态管理核心，负责链尖更新、分叉检测和处理、链同步等链级管理功能。

**在三层模型中的位置**：协调层（Coordination Layer）

> **战略背景**：Chain 组件位于核心业务层垂直依赖链的最高层（⑦），依赖 Block（⑥）、TX（⑤）和 EUTXO（④）。详见 [WES 项目总览](../overview.md)

---

## 💡 核心能力

### 1. 链状态管理（ChainWriter）

**能力描述**：
- 更新同步状态（同步进度、当前高度、网络高度）
- 验证链尖数据一致性
- 提供链状态查询接口

**使用约束**：
- ⚠️ **注意**：链尖更新已移除，由 `persistence.DataWriter.WriteBlock()` 统一管理
- 所有区块相关数据（包括链尖）都通过 DataWriter 写入，确保原子性和一致性
- 同步状态更新是独立的操作，不影响区块处理

**典型使用场景**：
- 链同步：更新同步进度和状态
- 状态监控：查询当前链状态和健康度

### 2. 分叉检测和处理（ForkHandler）

**能力描述**：
- 检测链分叉（基于区块哈希和父区块关系）
- 计算链权重（基于 PoW 难度和区块数量）
- 处理分叉（选择权重更大的链，回滚到分叉点）
- 获取活跃链信息

**使用约束**：
- 分叉检测需要查询链状态
- 分叉处理是原子操作，失败会回滚
- 支持并发分叉检测

**典型使用场景**：
- 分叉检测：检测到分叉时触发处理流程
- 链重组：回滚到分叉点并切换到新链

### 3. 链同步（SystemSyncService）

**能力描述**：
- 管理链同步状态和进度
- 触发同步任务（全量同步、增量同步、快速同步）
- 处理同步事件和状态转换
- 支持同步取消和恢复

**使用约束**：
- 同步是自动的，但可以手动触发
- 同步过程不影响节点运行
- 同步失败会重试

**典型使用场景**：
- 节点启动：自动同步到最新高度
- 网络恢复：恢复中断的同步任务

---

## 🔧 接口能力

### ChainWriter（链状态写入器）

**能力**：
- `UpdateSyncStatus(ctx, status)` - 更新同步状态
- `GetWriterMetrics(ctx)` - 获取写入性能指标
- `ValidateChainTip(ctx)` - 验证链尖数据一致性

**约束**：
- 同步状态更新是独立的操作
- 链尖更新由 DataWriter 统一管理

### ForkHandler（分叉处理器）

**能力**：
- `DetectFork(ctx, newBlock)` - 检测分叉
- `HandleFork(ctx, newBlock)` - 处理分叉
- `GetActiveChain(ctx)` - 获取活跃链信息

**约束**：
- 分叉检测需要查询链状态
- 分叉处理是原子操作

### SystemSyncService（链同步服务）

**能力**：
- `StartSync(ctx)` - 启动同步
- `StopSync(ctx)` - 停止同步
- `GetSyncStatus(ctx)` - 获取同步状态
- `CancelSync(ctx)` - 取消同步

**约束**：
- 同步是自动的，但可以手动控制
- 同步失败会重试

---

## ⚙️ 配置说明

### 链状态管理配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `sync_status_update_interval` | duration | 1s | 同步状态更新间隔 |
| `chain_tip_validation_interval` | duration | 60s | 链尖验证间隔 |

### 分叉处理配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `fork_detection_enabled` | bool | true | 启用分叉检测 |
| `fork_handling_timeout` | duration | 30s | 分叉处理超时时间 |
| `min_chain_weight_diff` | int | 1 | 最小链权重差（用于分叉选择） |

### 链同步配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `sync_mode` | string | "incremental" | 同步模式（full/incremental/fast） |
| `sync_batch_size` | int | 100 | 同步批次大小 |
| `sync_timeout` | duration | 300s | 同步超时时间 |

---

## 📋 使用约束

### 链状态管理约束

1. **同步状态更新约束**：
   - 同步状态更新是独立的操作
   - 不影响区块处理和链尖更新

2. **链尖验证约束**：
   - 链尖验证是只读操作
   - 验证失败可以尝试修复

### 分叉处理约束

1. **分叉检测约束**：
   - 分叉检测需要查询链状态
   - 检测结果可能不准确（网络延迟）

2. **分叉处理约束**：
   - 分叉处理是原子操作
   - 失败会回滚到处理前状态

### 链同步约束

1. **同步启动约束**：
   - 同步是自动的，但可以手动触发
   - 同步过程不影响节点运行

2. **同步取消约束**：
   - 同步取消是异步操作
   - 取消后可以恢复同步

---

## 🎯 典型使用场景

### 场景 1：更新同步状态

```go
writer := chain.NewChainWriter()
syncStatus := &types.SystemSyncStatus{
    Status:        types.SyncStatusSyncing,
    CurrentHeight: 1000,
    NetworkHeight: 1500,
    SyncProgress:  66.67,
}
err := writer.UpdateSyncStatus(ctx, syncStatus)
if err != nil {
    return err
}
```

### 场景 2：检测和处理分叉

```go
handler := chain.NewForkHandler()
isFork, forkHeight, err := handler.DetectFork(ctx, newBlock)
if err != nil {
    return err
}

if isFork {
    err := handler.HandleFork(ctx, newBlock)
    if err != nil {
        return err
    }
}
```

### 场景 3：启动链同步

```go
syncService := chain.NewSystemSyncService()
err := syncService.StartSync(ctx)
if err != nil {
    return err
}
// 同步会自动进行
```

---

## 📚 相关文档

- [架构鸟瞰](../architecture/overview.md) - 了解系统架构
- [Block 能力视图](./block.md) - 了解区块处理能力
- [TX 能力视图](./tx.md) - 了解交易能力


