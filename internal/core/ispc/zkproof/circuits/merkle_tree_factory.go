package circuits

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
)

// ============================================================================
// Merkle Tree电路工厂函数（解决数组长度问题）
// ============================================================================
//
// 🎯 **设计目的**：
// 提供工厂函数来正确创建Merkle Tree电路实例，确保数组长度在编译时固定。
//
// ⚠️ **关键问题**：
// gnark要求数组长度在编译时固定。使用切片 `[]` 会导致循环不执行的问题。
// 解决方案：使用固定长度数组 `[n]` 或通过工厂函数确保正确初始化。
//
// 📋 **设计决策**：
// 1. 定义最大深度常量（根据实际需求确定）
// 2. 提供工厂函数，根据实际路径长度创建电路
// 3. 如果路径长度超过最大深度，返回错误
//
// ============================================================================

const (
	// MaxMerkleTreeDepth 最大Merkle树深度
	// 根据实际业务需求确定：假设最多支持 2^20 = 1,048,576 个叶子节点
	MaxMerkleTreeDepth = 20

	// DefaultMerkleTreeDepth 默认Merkle树深度
	// 大多数情况下，树深度不会超过10
	DefaultMerkleTreeDepth = 10
)

// NewMerklePathCircuit 创建Merkle路径验证电路
//
// 📋 **参数**：
//   - depth: 路径深度（兄弟节点数量）
//
// 📋 **返回值**：
//   - *MerklePathCircuit: 正确初始化的电路实例
//   - error: 如果深度超过最大深度，返回错误
//
// ⚠️ **关键**：确保数组长度在创建时正确分配
func NewMerklePathCircuit(depth int) (*MerklePathCircuit, error) {
	if depth <= 0 {
		return nil, fmt.Errorf("路径深度必须大于0: %d", depth)
	}
	if depth > MaxMerkleTreeDepth {
		return nil, fmt.Errorf("路径深度超过最大限制: %d > %d", depth, MaxMerkleTreeDepth)
	}

	return &MerklePathCircuit{
		SiblingHashes:  make([]frontend.Variable, depth),
		PathDirections: make([]frontend.Variable, depth),
		MaxDepth:       depth,
	}, nil
}

// NewBatchMerklePathCircuit 创建批量Merkle路径验证电路
//
// 📋 **参数**：
//   - pathCount: 路径数量
//   - depth: 每个路径的深度（兄弟节点数量）
//
// 📋 **返回值**：
//   - *BatchMerklePathCircuit: 正确初始化的电路实例
//   - error: 如果参数无效，返回错误
//
// ⚠️ **关键**：确保每个路径的数组长度在创建时正确分配
func NewBatchMerklePathCircuit(pathCount int, depth int) (*BatchMerklePathCircuit, error) {
	if pathCount <= 0 {
		return nil, fmt.Errorf("路径数量必须大于0: %d", pathCount)
	}
	if depth <= 0 {
		return nil, fmt.Errorf("路径深度必须大于0: %d", depth)
	}
	if depth > MaxMerkleTreeDepth {
		return nil, fmt.Errorf("路径深度超过最大限制: %d > %d", depth, MaxMerkleTreeDepth)
	}

	paths := make([]MerklePathInput, pathCount)
	for i := range paths {
		paths[i] = MerklePathInput{
			SiblingHashes:  make([]frontend.Variable, depth),
			PathDirections: make([]frontend.Variable, depth),
			MaxDepth:       depth,
		}
	}

	return &BatchMerklePathCircuit{
		Paths:    paths,
		MaxPaths: pathCount,
	}, nil
}

// NewIncrementalUpdateCircuit 创建增量更新验证电路
//
// 📋 **参数**：
//   - pathCount: 变更路径数量
//   - depth: 每个路径的深度（兄弟节点数量）
//
// 📋 **返回值**：
//   - *IncrementalUpdateCircuit: 正确初始化的电路实例
//   - error: 如果参数无效，返回错误
//
// ⚠️ **关键**：确保每个路径的数组长度在创建时正确分配
func NewIncrementalUpdateCircuit(pathCount int, depth int) (*IncrementalUpdateCircuit, error) {
	if pathCount <= 0 {
		return nil, fmt.Errorf("路径数量必须大于0: %d", pathCount)
	}
	if depth <= 0 {
		return nil, fmt.Errorf("路径深度必须大于0: %d", depth)
	}
	if depth > MaxMerkleTreeDepth {
		return nil, fmt.Errorf("路径深度超过最大限制: %d > %d", depth, MaxMerkleTreeDepth)
	}

	changedPaths := make([]MerklePathInput, pathCount)
	for i := range changedPaths {
		changedPaths[i] = MerklePathInput{
			SiblingHashes:  make([]frontend.Variable, depth),
			PathDirections: make([]frontend.Variable, depth),
			MaxDepth:       depth,
		}
	}

	return &IncrementalUpdateCircuit{
		ChangedPaths: changedPaths,
		NewLeafData:  make([]frontend.Variable, pathCount),
		MaxPaths:     pathCount,
	}, nil
}

// CreateMerklePathCircuitFromPath 根据实际路径创建电路
//
// 📋 **参数**：
//   - siblingHashesCount: 兄弟节点哈希数量（路径深度）
//
// 📋 **返回值**：
//   - *MerklePathCircuit: 正确初始化的电路实例
//   - error: 如果参数无效，返回错误
//
// 🎯 **用途**：从实际的Merkle路径数据创建电路，确保数组长度匹配
func CreateMerklePathCircuitFromPath(siblingHashesCount int) (*MerklePathCircuit, error) {
	return NewMerklePathCircuit(siblingHashesCount)
}

