# Mempool 组件能力视图

---

## 🎯 组件定位

Mempool 组件是 WES 系统的内存池核心，负责交易的临时存储、优先级管理和候选区块的存储检索。

**在三层模型中的位置**：协调层（Coordination Layer）

> **战略背景**：Mempool 组件为矿工提供待打包交易，为共识层提供候选区块管理。详见 [WES 项目总览](../overview.md)

---

## 💡 核心能力

### 1. 交易池（TxPool）

**能力描述**：
- 交易的临时存储和快速访问
- 基于费用的优先级排序
- 交易生命周期管理（提交、确认、过期）
- 交易依赖关系管理
- 安全保护（防止恶意节点填满内存池）

**使用约束**：
- 交易池有容量限制
- 交易有生命周期，过期自动清理
- 重复交易会被拒绝
- 已确认交易会被自动移除

**典型使用场景**：
- 交易提交：用户提交交易到交易池
- 挖矿：矿工从交易池获取待打包交易
- 交易确认：区块确认后清理交易池

### 2. 候选区块池（CandidatePool）

**能力描述**：
- 候选区块的存储和检索
- 按高度索引候选区块
- 候选区块的超时清理
- 候选区块的验证和去重

**使用约束**：
- 候选区块有生命周期限制
- 候选区块按高度索引
- 过期候选区块会被自动清理

**典型使用场景**：
- 共识层：存储和检索候选区块
- 区块选择：从候选池中选择最优区块

---

## 🔧 接口能力

### TxPool（交易池）

**能力**：
- `SubmitTx(tx)` - 提交交易到交易池
- `GetTransactionsForMining()` - 获取挖矿交易列表（按优先级排序）
- `ConfirmTransactions(txIDs)` - 确认交易（清理已确认交易）
- `UpdateTransactionStatus(txID, status)` - 更新交易状态
- `GetTxStatus(txID)` - 查询交易状态

**约束**：
- 提交前必须完成基础验证
- 交易池有容量限制
- 重复交易会被拒绝

### CandidatePool（候选区块池）

**能力**：
- `AddCandidate(candidate)` - 添加候选区块
- `GetCandidatesForHeight(height)` - 获取指定高度的候选区块
- `ClearExpiredCandidates()` - 清理过期候选区块

**约束**：
- 候选区块必须通过基础验证
- 候选区块按高度索引
- 过期候选区块会被自动清理

---

## ⚙️ 配置说明

### 交易池配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `max_pool_size` | int | 10000 | 最大交易池大小 |
| `tx_expire_time` | duration | 1h | 交易过期时间 |
| `enable_priority_queue` | bool | true | 启用优先级队列 |
| `max_tx_size` | int | 1MB | 最大交易大小 |

### 候选区块池配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `max_candidate_pool_size` | int | 1000 | 最大候选池大小 |
| `candidate_expire_time` | duration | 5m | 候选区块过期时间 |
| `max_candidates_per_height` | int | 100 | 每个高度的最大候选数 |

---

## 📋 使用约束

### 交易池约束

1. **提交约束**：
   - 交易必须通过基础验证
   - 交易大小不能超过限制
   - 交易池不能超过容量限制

2. **优先级约束**：
   - 优先级基于交易费用
   - 高费用交易优先打包

3. **生命周期约束**：
   - 交易有过期时间
   - 过期交易会被自动清理
   - 已确认交易会被立即移除

### 候选区块池约束

1. **添加约束**：
   - 候选区块必须通过基础验证
   - 候选区块按高度索引
   - 每个高度的候选数有限制

2. **检索约束**：
   - 支持按高度检索
   - 支持超时等待
   - 过期候选会被自动清理

---

## 🎯 典型使用场景

### 场景 1：提交交易到交易池

```go
txPool := mempool.NewTxPool()
txHash, err := txPool.SubmitTx(tx)
if err != nil {
    return err
}
// 交易已进入交易池，等待打包
```

### 场景 2：获取挖矿交易列表

```go
txPool := mempool.NewTxPool()
txs, err := txPool.GetTransactionsForMining()
if err != nil {
    return err
}
// 按优先级排序的交易列表
```

### 场景 3：确认交易

```go
txPool := mempool.NewTxPool()
err := txPool.ConfirmTransactions(txIDs)
if err != nil {
    return err
}
// 已确认交易已从交易池移除
```

### 场景 4：添加候选区块

```go
candidatePool := mempool.NewCandidatePool()
err := candidatePool.AddCandidate(candidate)
if err != nil {
    return err
}
// 候选区块已添加到候选池
```

---

## 📚 相关文档

- [架构鸟瞰](../architecture/overview.md) - 了解系统架构
- [TX 能力视图](./tx.md) - 了解交易能力
- [Consensus 能力视图](./consensus.md) - 了解共识能力


