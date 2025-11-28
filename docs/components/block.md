# Block 组件能力视图

---

## 🎯 组件定位

Block 组件是 WES 系统的区块处理核心，负责区块的完整生命周期管理：构建、验证和处理。

**在三层模型中的位置**：协调层（Coordination Layer）

> **战略背景**：Block 组件位于核心业务层垂直依赖链的第⑥层，依赖 TX（⑤）和 EUTXO（④），被 Chain（⑦）依赖。详见 [WES 项目总览](../overview.md)

---

## 💡 核心能力

### 1. 区块构建（BlockBuilder）

**能力描述**：
- 为矿工创建挖矿候选区块
- 从交易池选择待打包交易
- 构建区块头和区块体
- 支持候选区块缓存和快速检索

**使用约束**：
- 必须从有效的交易池获取交易
- 区块大小和交易数量有限制
- 候选区块有生命周期，过期自动清理

**典型使用场景**：
- 矿工挖矿：创建候选区块模板
- 区块预构建：提前构建候选区块提升挖矿效率

### 2. 区块验证（BlockValidator）

**能力描述**：
- 多层验证机制：基础验证 → 结构验证 → 共识验证 → 交易验证 → 链连接性验证
- 支持快速失败：任一阶段失败立即返回
- 提供详细的验证错误信息

**验证流程**：
```
1. 基础验证（nil检查、空区块检查）
2. 结构验证（区块头、区块体、字段完整性）
3. 共识验证（PoW、难度、时间戳）
4. 交易验证（交易数量、Merkle根、交易有效性）
5. 链连接性验证（父区块、高度连续性）
```

**使用约束**：
- 验证是只读操作，不修改区块
- 验证失败会返回具体错误类型
- 支持并发验证

### 3. 区块处理（BlockProcessor）

**能力描述**：
- 处理验证通过的区块
- 统一通过 DataWriter 写入区块数据
- 验证交易执行结果（StateOutput、ResourceOutput、AssetOutput）
- 处理引用计数和状态根更新
- 清理交易池中的已确认交易

**处理流程**：
```
1. 并发控制检查
2. 验证区块（调用Validator）
3. 通过DataWriter.WriteBlock统一写入区块
   - 存储区块数据
   - 更新交易索引
   - 处理UTXO变更
   - 更新链状态
4. 验证所有交易执行结果
   - StateOutput: 验证ZK证明和执行结果哈希
   - ResourceOutput: 验证资源生命周期
   - AssetOutput: 最终确认交易有效性
   - 引用型输入: 验证引用UTXO的有效性
5. 处理引用计数（processReferenceCounts）
6. 更新状态根（updateStateRoot）
7. 清理交易池
8. 发布BlockProcessed事件
```

**使用约束**：
- 必须先验证，验证通过后才能处理
- 处理是原子操作，失败会回滚
- 支持并发控制，避免重复处理

---

## 🔧 接口能力

### BlockBuilder（区块构建器）

**能力**：
- `CreateMiningCandidate(ctx)` - 创建候选区块
- `GetCandidateBlock(ctx, blockHash)` - 获取缓存的候选区块
- `GetBuilderMetrics(ctx)` - 获取构建性能指标

**约束**：
- 候选区块有缓存机制
- 构建性能受交易池状态影响

### BlockValidator（区块验证器）

**能力**：
- `ValidateBlock(ctx, block)` - 执行完整验证
- `ValidateStructure(ctx, block)` - 验证结构（内部方法）
- `ValidateConsensus(ctx, block)` - 验证共识（内部方法）
- `GetValidatorMetrics(ctx)` - 获取验证性能指标

**约束**：
- 验证是只读操作，不修改区块
- 验证失败会返回具体错误信息

### BlockProcessor（区块处理器）

**能力**：
- `ProcessBlock(ctx, block)` - 处理区块（验证 + 写入 + 执行）
- `GetProcessorMetrics(ctx)` - 获取处理性能指标
- `SetValidator(validator)` - 设置验证器（延迟注入）

**约束**：
- 处理前必须完成验证
- 处理失败会回滚所有变更

---

## ⚙️ 配置说明

### 区块构建配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `max_block_size` | int | 2MB | 最大区块大小 |
| `max_tx_count` | int | 1000 | 最大交易数量 |
| `candidate_cache_size` | int | 100 | 候选区块缓存大小 |
| `candidate_cache_ttl` | duration | 5m | 候选区块缓存TTL |

### 区块验证配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `enable_parallel_validation` | bool | false | 启用并行验证 |
| `validation_timeout` | duration | 30s | 验证超时时间 |

### 区块处理配置

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `enable_concurrent_processing` | bool | false | 启用并发处理 |
| `processing_timeout` | duration | 60s | 处理超时时间 |

---

## 📋 使用约束

### 区块构建约束

1. **交易选择约束**：
   - 必须从有效的交易池获取交易
   - 交易必须已通过验证
   - 交易数量不能超过限制

2. **区块大小约束**：
   - 区块大小不能超过 `max_block_size`
   - 交易数量不能超过 `max_tx_count`

3. **缓存约束**：
   - 候选区块有生命周期限制
   - 过期候选区块会被自动清理

### 区块验证约束

1. **验证顺序约束**：
   - 必须按照验证流程顺序执行
   - 任一阶段失败立即返回

2. **验证结果约束**：
   - 验证失败会返回具体错误类型
   - 验证通过才能进入处理阶段

### 区块处理约束

1. **处理前要求**：
   - 区块必须通过验证
   - 必须提供有效的 DataWriter

2. **原子性约束**：
   - 处理是原子操作
   - 失败会回滚所有变更

3. **并发控制**：
   - 支持并发控制，避免重复处理
   - 同一区块不能同时被多个处理器处理

---

## 🎯 典型使用场景

### 场景 1：矿工创建候选区块

```go
builder := block.NewBlockBuilder()
candidateHash, err := builder.CreateMiningCandidate(ctx)
if err != nil {
    return err
}
// 使用 candidateHash 进行挖矿
```

### 场景 2：验证接收到的区块

```go
validator := block.NewBlockValidator()
valid, err := validator.ValidateBlock(ctx, receivedBlock)
if err != nil {
    return err
}
if !valid {
    return errors.New("区块验证失败")
}
```

### 场景 3：处理验证通过的区块

```go
processor := block.NewBlockProcessor()
err := processor.ProcessBlock(ctx, validatedBlock)
if err != nil {
    return err
}
// 区块已成功处理并持久化
```

---

## 📚 相关文档

- [架构鸟瞰](../architecture/overview.md) - 了解系统架构
- [TX 能力视图](./tx.md) - 了解交易能力
- [Chain 能力视图](./chain.md) - 了解链管理能力
- [EUTXO 能力视图](./eutxo.md) - 了解账本能力


