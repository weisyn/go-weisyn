# Merkle Tree增量验证模块

---

## 📌 模块说明

本模块实现了Merkle Tree增量验证算法，用于ISPC执行轨迹的增量验证。

**核心功能**：
- ✅ Merkle Tree构建
- ✅ 变更检测
- ✅ 增量证明生成
- ✅ 增量证明验证

**性能提升**：验证时间从O(n)降到O(k*log n)，k为变更记录数

---

## 🏗️ 模块结构

```
incremental/
├── types.go          # 数据结构定义
├── builder.go        # Merkle Tree构建器
├── detector.go       # 变更检测器
├── generator.go      # 增量证明生成器
└── verifier.go       # 增量验证器
```

---

## 📦 核心组件

### 1. MerkleTreeBuilder

**功能**：构建Merkle树、计算路径、验证路径

**使用示例**：

```go
// 创建构建器
builder := incremental.NewMerkleTreeBuilder(nil) // 使用默认SHA256哈希

// 构建树
tree, err := builder.BuildTree(records)

// 计算路径
path, err := builder.CalculatePath(tree, leafIndex)

// 验证路径
isValid := builder.VerifyPath(path)
```

---

### 2. ChangeDetector

**功能**：检测变更、计算变更路径

**使用示例**：

```go
// 创建检测器
detector := incremental.NewChangeDetector(builder)

// 检测变更
changes, err := detector.DetectChanges(oldRecords, newRecords)

// 计算变更路径
paths, err := detector.CalculateChangedPaths(tree, changes)
```

---

### 3. IncrementalProofGenerator

**功能**：生成增量验证证明

**使用示例**：

```go
// 创建生成器
generator := incremental.NewIncrementalProofGenerator(builder, detector)

// 生成证明
proof, err := generator.GenerateProof(oldTree, newRecords, nil) // nil表示自动检测变更
```

---

### 4. IncrementalVerifier

**功能**：验证增量证明

**使用示例**：

```go
// 创建验证器
verifier := incremental.NewIncrementalVerifier(builder)

// 验证证明
isValid, err := verifier.VerifyProof(proof, oldRootHash)
```

---

## 🔧 完整使用流程

```go
// 1. 初始化组件
builder := incremental.NewMerkleTreeBuilder(nil)
detector := incremental.NewChangeDetector(builder)
generator := incremental.NewIncrementalProofGenerator(builder, detector)
verifier := incremental.NewIncrementalVerifier(builder)

// 2. 构建旧轨迹的Merkle树
oldTree, err := builder.BuildTree(oldRecords)

// 3. 生成增量证明
proof, err := generator.GenerateProof(oldTree, newRecords, nil)

// 4. 验证增量证明
isValid, err := verifier.VerifyProof(proof, oldTree.Root.Hash)
```

---

## 🔗 与 coordinator.ExecutionTrace 集成

### TraceRecord 设计

`TraceRecord` 现在直接存储序列化后的轨迹数据（`[]byte`），避免重复定义结构。

**使用方式**：

```go
import (
    "github.com/weisyn/v1/internal/core/ispc/coordinator"
    "github.com/weisyn/v1/internal/core/ispc/zkproof/incremental"
)

// 1. 获取 ExecutionTrace（从 coordinator）
trace := &coordinator.ExecutionTrace{
    TraceID: "trace_123",
    StartTime: startTime,
    EndTime: endTime,
    // ... 其他字段
}

// 2. 序列化 ExecutionTrace（使用 coordinator 的序列化方法）
// 注意：需要访问 coordinator.Manager 的 serializeExecutionTraceForZK 方法
traceBytes, err := coordinatorManager.serializeExecutionTraceForZK(trace)
if err != nil {
    return err
}

// 3. 创建 TraceRecord（使用序列化后的数据）
record := incremental.NewTraceRecord(traceBytes, nil) // nil 使用默认SHA256

// 4. 构建 Merkle 树
records := []*incremental.TraceRecord{record}
tree, err := builder.BuildTree(records)
```

### 序列化方法

**重要**：必须使用 `coordinator.Manager.serializeExecutionTraceForZK()` 方法序列化 `ExecutionTrace`，该方法：
- 使用确定性编码（大端序）
- 确保多次序列化结果一致
- 符合 ZK 证明的确定性要求

---

## ⚠️ 注意事项

1. **TraceRecord 集成**：✅ 已修复 - TraceRecord 现在直接使用序列化后的数据，与 coordinator.ExecutionTrace 完全集成
2. **增量更新优化**：当前 RebuildTree 为完整重建，后续需要优化为真正的增量更新
3. **根哈希重计算**：当前根哈希重计算为简化实现，后续需要实现完整算法
4. **哈希函数**：当前使用 SHA256，后续可替换为 Poseidon（ZK友好）

---

## 📚 相关文档

- **可行性研究**：`docs/components/core/ispc/optimizations/incremental-verification-feasibility-report.md`
- **设计方案**：`docs/components/core/ispc/optimizations/incremental-verification-design.md`
- **电路设计**：`docs/components/core/ispc/optimizations/merkle-tree-circuit.md`

---

**最后更新**：2025-11-24

