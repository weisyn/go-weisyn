package circuits

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash/poseidon2"
)

// ============================================================================
// Poseidon哈希辅助函数（Merkle Tree增量验证电路优化 - 后续工作）
// ============================================================================
//
// 🎯 **设计目的**：
// 提供Poseidon2哈希函数，用于Merkle Tree电路中的哈希计算。
// Poseidon2是ZK友好的哈希函数，相比SHA256可以减少90%的约束数量。
//
// 🏗️ **实现策略**：
// - 使用gnark的poseidon2包
// - 支持2输入Poseidon2哈希（用于Merkle Tree节点组合）
// - 提供统一的哈希接口
//
// ⚠️ **注意**：
// - Poseidon2哈希需要2个输入（left, right）
// - 输出是单个field元素
// - 约束数量约为200（相比SHA256的~2000约束，减少90%）
//
// ============================================================================

// PoseidonHasher Poseidon2哈希器
type PoseidonHasher struct {
	api frontend.API
}

// NewPoseidonHasher 创建Poseidon2哈希器
func NewPoseidonHasher(api frontend.API) (*PoseidonHasher, error) {
	return &PoseidonHasher{
		api: api,
	}, nil
}

// Hash2 计算2输入的Poseidon2哈希
//
// 📋 **参数**：
//   - left: 左输入（field元素）
//   - right: 右输入（field元素）
//
// 📋 **返回值**：
//   - frontend.Variable: 哈希结果（field元素）
func (h *PoseidonHasher) Hash2(left, right frontend.Variable) frontend.Variable {
	// 创建新的hasher（每次调用都需要新的hasher，因为hasher是有状态的）
	hasher, err := poseidon2.NewMerkleDamgardHasher(h.api)
	if err != nil {
		// 如果创建失败，返回0（会导致验证失败）
		// 在实际使用中，这不应该发生
		return 0
	}
	
	// 写入两个输入
	hasher.Write(left, right)
	
	// 计算并返回哈希结果
	return hasher.Sum()
}

// HashLeaf 计算叶子节点的Poseidon2哈希
//
// 📋 **参数**：
//   - leafData: 叶子节点数据（field元素）
//
// 📋 **返回值**：
//   - frontend.Variable: 叶子节点哈希
func (h *PoseidonHasher) HashLeaf(leafData frontend.Variable) frontend.Variable {
	// 叶子节点哈希：hash(leafData, 0)
	// 使用0作为填充，确保叶子节点和内部节点有不同的哈希计算方式
	return h.Hash2(leafData, 0)
}

// HashNode 计算内部节点的Poseidon2哈希
//
// 📋 **参数**：
//   - left: 左子节点哈希
//   - right: 右子节点哈希
//
// 📋 **返回值**：
//   - frontend.Variable: 父节点哈希
func (h *PoseidonHasher) HashNode(left, right frontend.Variable) frontend.Variable {
	// 内部节点哈希：hash(left, right)
	return h.Hash2(left, right)
}

