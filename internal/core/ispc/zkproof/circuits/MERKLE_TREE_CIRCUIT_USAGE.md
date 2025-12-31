# Merkle Tree 电路使用指南

## 📋 问题背景

在 gnark 中，**数组长度必须在电路定义时固定**。如果 `SiblingHashes` 或 `PathDirections` 在定义时长度为 0，循环 `for j := 0; j < len(path.SiblingHashes); j++` 不会执行，导致哈希计算失败。

## ✅ 解决方案

### 1. 使用工厂函数创建电路

**不要直接实例化电路**，必须使用工厂函数：

```go
import "github.com/weisyn/v1/internal/core/ispc/zkproof/circuits"

// ✅ 正确：使用工厂函数
circuit, err := circuits.NewMerklePathCircuit(depth)
if err != nil {
    return err
}

// ❌ 错误：直接实例化（数组长度为0）
circuit := &circuits.MerklePathCircuit{}  // SiblingHashes 长度为 0，循环不会执行
```

### 2. 可用的工厂函数

#### `NewMerklePathCircuit(depth int)`
创建单个路径验证电路。

**参数**：
- `depth`: 路径深度（兄弟节点数量）

**示例**：
```go
circuit, err := circuits.NewMerklePathCircuit(2)  // 深度为2
```

#### `NewBatchMerklePathCircuit(pathCount int, depth int)`
创建批量路径验证电路。

**参数**：
- `pathCount`: 路径数量
- `depth`: 每个路径的深度（兄弟节点数量）

**示例**：
```go
circuit, err := circuits.NewBatchMerklePathCircuit(2, 2)  // 2个路径，每个深度为2
```

#### `NewIncrementalUpdateCircuit(pathCount int, depth int)`
创建增量更新验证电路。

**参数**：
- `pathCount`: 变更路径数量
- `depth`: 每个路径的深度（兄弟节点数量）

**示例**：
```go
circuit, err := circuits.NewIncrementalUpdateCircuit(1, 1)  // 1个路径，深度为1
```

### 3. 最大深度限制

- **MaxMerkleTreeDepth = 20**：最大支持深度（支持最多 2^20 = 1,048,576 个叶子节点）
- **DefaultMerkleTreeDepth = 10**：默认深度（大多数情况下足够）

如果路径深度超过最大限制，工厂函数会返回错误。

## 🔍 在哪里定义和使用？

### 电路定义位置
- **定义文件**：`internal/core/ispc/zkproof/circuits/merkle_tree.go`
- **工厂函数**：`internal/core/ispc/zkproof/circuits/merkle_tree_factory.go`

### 实际使用位置

#### 1. 测试代码
测试代码中已经更新为使用工厂函数：
- `internal/core/ispc/zkproof/circuits/merkle_tree_test.go`
- `internal/core/ispc/zkproof/circuits/merkle_tree_integration_test.go`

#### 2. 实际运行代码（未来）
当需要在 ISPC 运行时使用这些电路时，应该：

```go
// 从 incremental 包获取路径信息
path, err := builder.CalculatePath(tree, leafIndex)
if err != nil {
    return err
}

// 使用工厂函数创建电路
depth := len(path.SiblingHashes)
circuit, err := circuits.NewMerklePathCircuit(depth)
if err != nil {
    return fmt.Errorf("创建电路失败: %w", err)
}

// 创建 witness
witness := &circuits.MerklePathCircuit{
    RootHash:       rootHash,
    LeafData:       leafData,
    LeafIndex:      leafIndex,
    SiblingHashes:  path.SiblingHashes,  // 长度已匹配
    PathDirections: path.PathDirections, // 长度已匹配
    MaxDepth:       depth,
}
```

### 3. CircuitManager 集成

`CircuitManager` 目前不支持 Merkle Tree 电路，因为需要路径深度参数。应该直接使用工厂函数：

```go
// ❌ 不要这样做
circuit, err := circuitManager.GetCircuit("merkle_path", 1)  // 会返回错误

// ✅ 应该这样做
circuit, err := circuits.NewMerklePathCircuit(depth)
```

## 📊 数组类型选择

### 为什么使用切片 `[]` 而不是固定长度数组 `[n]`？

1. **灵活性**：需要支持不同深度的路径（1-20层）
2. **gnark 支持**：gnark 支持切片，只要长度在创建实例时确定
3. **工厂函数保证**：工厂函数确保数组长度正确初始化

### 如果使用固定长度数组 `[n]`？

如果使用固定长度数组，需要为每个可能的深度创建不同的电路类型：
- `MerklePathCircuitDepth1`
- `MerklePathCircuitDepth2`
- ...
- `MerklePathCircuitDepth20`

这会导致代码重复和维护困难。

## ⚠️ 注意事项

1. **必须使用工厂函数**：不要直接实例化电路结构体
2. **路径深度匹配**：确保 witness 中的数组长度与电路定义时一致
3. **最大深度限制**：不要超过 `MaxMerkleTreeDepth = 20`
4. **错误处理**：工厂函数会验证参数，必须检查错误

## 🔗 相关文件

- `internal/core/ispc/zkproof/circuits/merkle_tree.go` - 电路定义
- `internal/core/ispc/zkproof/circuits/merkle_tree_factory.go` - 工厂函数
- `internal/core/ispc/zkproof/circuit_manager.go` - 电路管理器（不支持Merkle Tree电路）

