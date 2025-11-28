# Consensus 组件能力视图

---

## 🎯 组件定位

Consensus 组件是 WES 系统的共识核心，采用基于距离寻址选择算法的统一 Aggregator 架构，实现 PoW+XOR 混合共识机制。

**在三层模型中的位置**：协调层（Coordination Layer）

> **战略背景**：Consensus 组件定义了 WES 的去中心化共识机制，通过矿工挖矿和聚合器距离选择实现高效、安全的区块共识。详见 [WES 项目总览](../overview.md)

---

## 💡 核心能力

### 1. 矿工服务（Miner）

**能力描述**：
- PoW 工作量证明计算
- 候选区块模板创建
- 挖矿生命周期管理（启动、停止、暂停、恢复）
- 高度门闸管理（防止重复挖矿）
- 挖矿状态管理

**使用约束**：
- 挖矿需要有效的交易池
- 挖矿需要区块链服务支持
- 挖矿结果通过内部接口交给 Aggregator 处理

**典型使用场景**：
- 节点挖矿：启动挖矿服务参与区块生产
- 挖矿控制：动态调整挖矿参数和状态

### 2. 聚合器服务（Aggregator）

**能力描述**：
- 统一网络处理（区块发送、接收、路由决策）
- 动态角色决策（基于 K-bucket 距离判断是否为聚合节点）
- 距离选择引擎（基于 XOR 距离的确定性区块选择）
- 候选区块收集和去重
- 共识结果广播和状态同步

**使用约束**：
- 聚合器角色是动态的，基于距离计算决定
- 距离选择是确定性的，相同输入必产生唯一结果
- 共识结果需要全网广播

**典型使用场景**：
- 区块路由：接收区块并路由到正确的聚合节点
- 共识决策：从候选区块中选择最优区块
- 结果广播：将共识结果广播到全网

---

## 🔧 接口能力

### MinerService（矿工服务）

**能力**：
- `StartMining(ctx)` - 启动挖矿
- `StopMining(ctx)` - 停止挖矿
- `GetMiningStatus()` - 获取挖矿状态
- `SetMiningParameters(params)` - 设置挖矿参数
- `ResumeMining(ctx)` - 恢复挖矿
- `PauseMining(ctx)` - 暂停挖矿

**约束**：
- 挖矿需要有效的交易池和区块链服务
- 挖矿参数可以动态调整

### AggregatorService（聚合器服务）

**能力**：
- `StartAggregation(ctx)` - 启动聚合
- `StopAggregation(ctx)` - 停止聚合
- `GetAggregationStatus()` - 获取聚合状态
- `SetAggregationPolicy(policy)` - 设置聚合策略
- `ProcessCandidateBlock(block)` - 处理候选区块
- `GetDecisionResult(height)` - 获取决策结果

**约束**：
- 聚合器角色是动态的
- 距离选择是确定性的

---

## ⚙️ 配置说明

### 矿工配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `confirmation_timeout` | duration | 30s | 确认等待超时时间 |
| `block_interval` | duration | 10s | 目标出块间隔 |
| `mining_threads` | int | 4 | PoW 计算线程数 |
| `neighbor_fanout` | int | 2 | 首跳扇出数（默认2个近邻） |
| `max_retries` | int | 3 | 发送失败最大重试次数 |

### 聚合器配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `aggregation_interval` | duration | 5s | 聚合轮次间隔 |
| `min_candidates` | int | 1 | 最小候选区块数 |
| `max_candidates` | int | 100 | 最大候选区块数 |
| `selection_timeout` | duration | 0.01s | 距离选择超时（微秒级） |
| `distance_algorithm` | string | "XOR" | 距离计算算法（固定为XOR） |

---

## 📋 使用约束

### 矿工约束

1. **挖矿启动约束**：
   - 需要有效的交易池
   - 需要区块链服务支持
   - 需要网络连接

2. **挖矿参数约束**：
   - 挖矿参数可以动态调整
   - 调整后立即生效

### 聚合器约束

1. **角色决策约束**：
   - 角色是动态的，基于距离计算
   - 不是最近节点时会转发区块

2. **距离选择约束**：
   - 距离选择是确定性的
   - 相同输入必产生唯一结果
   - 选择延迟 < 1ms

---

## 🎯 典型使用场景

### 场景 1：启动挖矿

```go
minerService := consensus.NewMinerService()
err := minerService.StartMining(ctx)
if err != nil {
    return err
}
// 挖矿已启动，自动参与区块生产
```

### 场景 2：处理候选区块

```go
aggregatorService := consensus.NewAggregatorService()
err := aggregatorService.ProcessCandidateBlock(candidateBlock)
if err != nil {
    return err
}
// 候选区块已处理，等待距离选择决策
```

### 场景 3：获取共识结果

```go
aggregatorService := consensus.NewAggregatorService()
result, err := aggregatorService.GetDecisionResult(height)
if err != nil {
    return err
}
// 获取指定高度的共识结果
```

---

## 📚 相关文档

- [架构鸟瞰](../architecture/overview.md) - 了解系统架构
- [Block 能力视图](./block.md) - 了解区块处理能力
- [Mempool 能力视图](./mempool.md) - 了解内存池能力


