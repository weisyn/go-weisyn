# Block 内部接口（internal/core/block/interfaces）

---

## 📍 **模块定位**

本目录定义 Block 模块的内部接口层，这些接口：
- ✅ 继承公共接口（`pkg/interfaces/block`）
- ✅ 扩展内部管理方法
- ✅ 提供指标和监控接口
- ✅ 支持模块内部协调

**解决什么问题**：
- 接口继承：通过嵌入继承公共接口，确保对外兼容性
- 内部扩展：添加模块内部需要的管理方法
- 指标收集：提供性能监控和调试接口

**不解决什么问题**（边界）：
- 不定义业务逻辑（由 `builder/`, `processor/`, `validator/` 实现）
- 不定义公共接口（由 `pkg/interfaces/block` 定义）

---

## 🏗️ **接口列表**

### 1. InternalBlockBuilder（builder.go）

**职责**：区块构建的内部接口

**继承**：`block.BlockBuilder`

**扩展方法**：
- `GetBuilderMetrics()` - 获取构建指标
- `GetCachedCandidate()` - 获取缓存的候选区块
- `ClearCandidateCache()` - 清理候选区块缓存

**指标类型**：`BuilderMetrics`

### 2. InternalBlockProcessor（processor.go）

**职责**：区块处理的内部接口

**继承**：`block.BlockProcessor`

**扩展方法**：
- `GetProcessorMetrics()` - 获取处理指标
- `SetValidator()` - 设置验证器（延迟注入）

**指标类型**：`ProcessorMetrics`

### 3. InternalBlockValidator（validator.go）

**职责**：区块验证的内部接口

**继承**：`block.BlockValidator`

**扩展方法**：
- `GetValidatorMetrics()` - 获取验证指标
- `ValidateStructure()` - 验证区块结构
- `ValidateConsensus()` - 验证共识规则

**指标类型**：`ValidatorMetrics`

---

## 🔗 **接口关系**

```
pkg/interfaces/block/           （公共接口）
    ├── BlockBuilder
    ├── BlockProcessor
    └── BlockValidator
        ↓ 继承（嵌入）
internal/core/block/interfaces/  （内部接口）
    ├── InternalBlockBuilder
    ├── InternalBlockProcessor
    └── InternalBlockValidator
        ↓ 实现
internal/core/block/            （具体实现）
    ├── builder/Service
    ├── processor/Service
    └── validator/Service
```

---

## 📊 **指标类型说明**

### BuilderMetrics
- 统计指标：CandidatesCreated, CacheHits, CacheMisses
- 时间指标：LastCandidateTime, AvgCreationTime, MaxCreationTime
- 缓存指标：CacheSize, MaxCacheSize
- 状态指标：IsHealthy, ErrorMessage

### ProcessorMetrics
- 统计指标：BlocksProcessed, TransactionsExecuted, SuccessCount, FailureCount
- 时间指标：LastProcessTime, AvgProcessTime, MaxProcessTime
- 数据指标：LastBlockHeight, LastBlockHash
- 状态指标：IsProcessing, IsHealthy, ErrorMessage

### ValidatorMetrics
- 统计指标：BlocksValidated, ValidationsPassed, ValidationsFailed
- 失败分类：StructureErrors, ConsensusErrors, TransactionErrors
- 时间指标：LastValidateTime, AvgValidateTime, MaxValidateTime
- 状态指标：IsHealthy, ErrorMessage

---

## 📚 **参考文档**

- [Block 模块 README](../README.md) - 模块总览（待创建）
- [技术设计文档](../TECHNICAL_DESIGN.md) - 详细设计
- [实施计划](../IMPLEMENTATION_PLAN.md) - 实施步骤
- [公共接口 README](../../../../pkg/interfaces/block/README.md) - 公共接口定义

---

**状态**：✅ 已完成

**维护者**：WES Block 开发组

