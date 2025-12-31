package circuits

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr/poseidon2"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/test"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Merkle Tree电路测试（Merkle Tree增量验证电路优化 - 阶段2）
// ============================================================================
//
// 🎯 **测试目的**：
// 测试Merkle Tree增量验证电路的功能和性能。
//
// ⚠️ **注意**：
// - 使用真实的gnark测试框架
// - 使用Poseidon2哈希（需要BLS12-377曲线）
// - 使用真实的Merkle Tree数据
//
// ============================================================================

// computePoseidon2Hash 计算Poseidon2哈希（用于测试）
// 使用Merkle-Damgard结构，输入两个field元素（big.Int）
func computePoseidon2Hash(left, right *big.Int) *big.Int {
	hasher := poseidon2.NewMerkleDamgardHasher()
	
	// 将big.Int转换为字节（使用32字节，因为fr.Element是32字节）
	leftBytes := make([]byte, 32)
	rightBytes := make([]byte, 32)
	
	left.FillBytes(leftBytes)
	right.FillBytes(rightBytes)
	
	// 写入数据
	hasher.Write(leftBytes)
	hasher.Write(rightBytes)
	
	// 计算哈希
	result := hasher.Sum(nil)
	
	// 将结果转换为big.Int
	var resultBig big.Int
	resultBig.SetBytes(result)
	return &resultBig
}

// computePoseidon2LeafHash 计算叶子节点的Poseidon2哈希
func computePoseidon2LeafHash(leafData *big.Int) *big.Int {
	zero := big.NewInt(0)
	return computePoseidon2Hash(leafData, zero)
}

// TestMerklePathCircuit 测试Merkle路径验证电路
func TestMerklePathCircuit(t *testing.T) {
	assert := test.NewAssert(t)
	
	// 创建测试数据：构建一个简单的Merkle Tree
	// 叶子节点：0, 1
	leaf0Data := big.NewInt(0)
	leaf1Data := big.NewInt(1)
	
	leaf0Hash := computePoseidon2LeafHash(leaf0Data)
	leaf1Hash := computePoseidon2LeafHash(leaf1Data)
	
	// 计算根哈希：hash(leaf0Hash, leaf1Hash)
	rootHash := computePoseidon2Hash(leaf0Hash, leaf1Hash)
	
	// 创建路径：从leaf0到root
	// leaf0 -> root (left, sibling=leaf1)
	depth := 1
	circuit := &MerklePathCircuit{
		SiblingHashes:  make([]frontend.Variable, depth),
		PathDirections: make([]frontend.Variable, depth),
		MaxDepth:       10,
	}
	
	// 创建有效的witness
	witness := &MerklePathCircuit{
		RootHash:       rootHash,
		LeafData:       leaf0Data,
		LeafIndex:      0,
		SiblingHashes:  []frontend.Variable{leaf1Hash},
		PathDirections: []frontend.Variable{0}, // 左子节点
		MaxDepth:       10,
	}
	
	// 运行测试（使用BLS12-377曲线，因为Poseidon2需要）
	assert.CheckCircuit(
		circuit,
		test.WithValidAssignment(witness),
		test.WithCurves(ecc.BLS12_377),
	)
}

// TestBatchMerklePathCircuit 测试批量Merkle路径验证电路
func TestBatchMerklePathCircuit(t *testing.T) {
	assert := test.NewAssert(t)
	
	// 创建测试数据：构建一个简单的Merkle Tree
	leaf0Data := big.NewInt(0)
	leaf1Data := big.NewInt(1)
	leaf2Data := big.NewInt(2)
	leaf3Data := big.NewInt(3)
	
	leaf0Hash := computePoseidon2LeafHash(leaf0Data)
	leaf1Hash := computePoseidon2LeafHash(leaf1Data)
	leaf2Hash := computePoseidon2LeafHash(leaf2Data)
	leaf3Hash := computePoseidon2LeafHash(leaf3Data)
	
	node01Hash := computePoseidon2Hash(leaf0Hash, leaf1Hash)
	node23Hash := computePoseidon2Hash(leaf2Hash, leaf3Hash)
	rootHash := computePoseidon2Hash(node01Hash, node23Hash)
	
	// 创建批量路径验证电路
	// ⚠️ **关键BUG修复说明**：在 gnark 中，数组长度必须在电路定义时固定
	// 
	// 🐛 **BUG描述**：
	// 如果 `path.SiblingHashes` 在定义时长度为 0，循环 `for j := 0; j < len(path.SiblingHashes); j++` 
	// 不会执行，导致哈希计算被跳过，电路验证失败。
	// 
	// ✅ **修复方法**：
	// 每个路径有 2 个兄弟节点（深度为 2），所以 SiblingHashes 和 PathDirections 长度必须为 2。
	// 必须在创建电路实例时明确指定数组长度，而不是使用 `make([]MerklePathInput, 2)`。
	circuit := &BatchMerklePathCircuit{
		Paths: []MerklePathInput{
			{
				SiblingHashes:  make([]frontend.Variable, 2),
				PathDirections: make([]frontend.Variable, 2),
				MaxDepth:       10,
			},
			{
				SiblingHashes:  make([]frontend.Variable, 2),
				PathDirections: make([]frontend.Variable, 2),
				MaxDepth:       10,
			},
		},
		MaxPaths: 5,
	}
	
	// 创建有效的witness：验证leaf0和leaf2的路径
	witness := &BatchMerklePathCircuit{
		RootHash: rootHash,
		Paths: []MerklePathInput{
			{
				LeafData:       leaf0Data,
				LeafIndex:      0,
				SiblingHashes:  []frontend.Variable{leaf1Hash, node23Hash},
				PathDirections: []frontend.Variable{0, 0},
				MaxDepth:       10,
			},
			{
				LeafData:       leaf2Data,
				LeafIndex:      2,
				SiblingHashes:  []frontend.Variable{leaf3Hash, node01Hash},
				PathDirections: []frontend.Variable{0, 1}, // 第二个路径：左子节点，然后右子节点
				MaxDepth:       10,
			},
		},
		MaxPaths: 5,
	}
	
	// 运行测试
	assert.CheckCircuit(
		circuit,
		test.WithValidAssignment(witness),
		test.WithCurves(ecc.BLS12_377),
	)
}

// TestIncrementalUpdateCircuit 测试增量更新验证电路
func TestIncrementalUpdateCircuit(t *testing.T) {
	assert := test.NewAssert(t)
	
	// 创建旧树的数据
	oldLeaf0Data := big.NewInt(0)
	oldLeaf1Data := big.NewInt(1)
	
	oldLeaf0Hash := computePoseidon2LeafHash(oldLeaf0Data)
	oldLeaf1Hash := computePoseidon2LeafHash(oldLeaf1Data)
	
	oldRootHash := computePoseidon2Hash(oldLeaf0Hash, oldLeaf1Hash)
	
	// 创建新树的数据（更新leaf0）
	newLeaf0Data := big.NewInt(10) // 新叶子数据
	
	newLeaf0Hash := computePoseidon2LeafHash(newLeaf0Data)
	newLeaf1Hash := oldLeaf1Hash // 保持不变
	
	newRootHash := computePoseidon2Hash(newLeaf0Hash, newLeaf1Hash)
	
	// 创建增量更新验证电路
	// ⚠️ **关键BUG修复说明**：在 gnark 中，数组长度必须在电路定义时固定
	// 
	// 🐛 **BUG描述**：
	// 如果 `path.SiblingHashes` 在定义时长度为 0，循环 `for j := 0; j < len(path.SiblingHashes); j++` 
	// 不会执行，导致哈希计算被跳过，电路验证失败。
	// 
	// ✅ **修复方法**：
	// 路径有 1 个兄弟节点（深度为 1），所以 SiblingHashes 和 PathDirections 长度必须为 1。
	// 必须在创建电路实例时明确指定数组长度。
	circuit := &IncrementalUpdateCircuit{
		ChangedPaths: []MerklePathInput{
			{
				SiblingHashes:  make([]frontend.Variable, 1),
				PathDirections: make([]frontend.Variable, 1),
				MaxDepth:       10,
			},
		},
		NewLeafData: make([]frontend.Variable, 1),
		MaxPaths:    5,
	}
	
	// 创建有效的witness
	witness := &IncrementalUpdateCircuit{
		OldRootHash: oldRootHash,
		NewRootHash: newRootHash,
		ChangedPaths: []MerklePathInput{
			{
				LeafData:       oldLeaf0Data, // 旧叶子数据
				LeafIndex:      0,
				SiblingHashes:  []frontend.Variable{oldLeaf1Hash},
				PathDirections: []frontend.Variable{0},
				MaxDepth:       10,
			},
		},
		NewLeafData: []frontend.Variable{newLeaf0Data}, // 新叶子数据
		MaxPaths:    5,
	}
	
	// 运行测试
	assert.CheckCircuit(
		circuit,
		test.WithValidAssignment(witness),
		test.WithCurves(ecc.BLS12_377),
	)
}

// BenchmarkMerklePathCircuit 基准测试Merkle路径验证电路
func BenchmarkMerklePathCircuit(b *testing.B) {
	// 创建测试数据
	leaf0Data := big.NewInt(0)
	leaf1Data := big.NewInt(1)
	
	leaf0Hash := computePoseidon2LeafHash(leaf0Data)
	leaf1Hash := computePoseidon2LeafHash(leaf1Data)
	rootHash := computePoseidon2Hash(leaf0Hash, leaf1Hash)
	
	circuit := &MerklePathCircuit{
		SiblingHashes:  make([]frontend.Variable, 1),
		PathDirections: make([]frontend.Variable, 1),
		MaxDepth:       10,
	}
	
	witness := &MerklePathCircuit{
		RootHash:       rootHash,
		LeafData:       leaf0Data,
		LeafIndex:      0,
		SiblingHashes:  []frontend.Variable{leaf1Hash},
		PathDirections: []frontend.Variable{0},
		MaxDepth:       10,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 编译电路
		_, err := frontend.Compile(ecc.BLS12_377.ScalarField(), nil, circuit)
		require.NoError(b, err)
		
		// 创建witness
		_, err = frontend.NewWitness(witness, ecc.BLS12_377.ScalarField())
		require.NoError(b, err)
	}
}
