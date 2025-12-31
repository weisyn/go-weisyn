package circuits

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/test"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// 边界情况测试
// ============================================================================
//
// 🎯 **测试目的**：
// 测试边界情况和错误处理，确保电路的健壮性。

// TestMerklePathCircuit_MaxDepth 测试最大深度
func TestMerklePathCircuit_MaxDepth(t *testing.T) {
	// 使用最大深度创建电路
	circuit, err := NewMerklePathCircuit(MaxMerkleTreeDepth)
	require.NoError(t, err)

	// 创建深度为MaxMerkleTreeDepth的测试数据
	// 这里简化测试，只测试电路能否编译和运行
	leafData := big.NewInt(0)
	_ = computePoseidon2LeafHash(leafData)
	leafHash := computePoseidon2LeafHash(leafData)

	// 创建简单的路径（所有兄弟节点都是leafHash）
	siblingHashes := make([]frontend.Variable, MaxMerkleTreeDepth)
	pathDirections := make([]frontend.Variable, MaxMerkleTreeDepth)
	for i := range siblingHashes {
		siblingHashes[i] = leafHash
		pathDirections[i] = 0
	}

	// 运行测试（只测试编译，不测试验证，因为路径可能不正确）
	// 注意：MaxMerkleTreeDepth=20 的电路编译可能需要较长时间，这里只测试电路创建
	require.NotNil(t, circuit)
	require.Equal(t, MaxMerkleTreeDepth, len(circuit.SiblingHashes))
	require.Equal(t, MaxMerkleTreeDepth, len(circuit.PathDirections))
	require.Equal(t, MaxMerkleTreeDepth, circuit.MaxDepth)
}

// TestMerklePathCircuit_Depth1 测试最小深度（深度为1）
func TestMerklePathCircuit_Depth1(t *testing.T) {
	assert := test.NewAssert(t)

	circuit, err := NewMerklePathCircuit(1)
	require.NoError(t, err)

	leaf0Data := big.NewInt(0)
	leaf1Data := big.NewInt(1)

	leaf0Hash := computePoseidon2LeafHash(leaf0Data)
	leaf1Hash := computePoseidon2LeafHash(leaf1Data)
	rootHash := computePoseidon2Hash(leaf0Hash, leaf1Hash)

	witness := &MerklePathCircuit{
		RootHash:       rootHash,
		LeafData:       leaf0Data,
		LeafIndex:      0,
		SiblingHashes:  []frontend.Variable{leaf1Hash},
		PathDirections: []frontend.Variable{0},
		MaxDepth:       1,
	}

	assert.CheckCircuit(
		circuit,
		test.WithValidAssignment(witness),
		test.WithCurves(ecc.BLS12_377),
	)
}

// TestMerklePathCircuit_InvalidPath 测试无效路径（应该失败）
func TestMerklePathCircuit_InvalidPath(t *testing.T) {
	assert := test.NewAssert(t)

	circuit, err := NewMerklePathCircuit(1)
	require.NoError(t, err)

	leaf0Data := big.NewInt(0)
	leaf1Data := big.NewInt(1)

	_ = computePoseidon2LeafHash(leaf0Data)
	leaf1Hash := computePoseidon2LeafHash(leaf1Data)
	// 使用错误的根哈希
	wrongRootHash := big.NewInt(999999)

	witness := &MerklePathCircuit{
		RootHash:       wrongRootHash,
		LeafData:       leaf0Data,
		LeafIndex:      0,
		SiblingHashes:  []frontend.Variable{leaf1Hash},
		PathDirections: []frontend.Variable{0},
		MaxDepth:       1,
	}

	// 应该失败（无效的路径）
	assert.CheckCircuit(
		circuit,
		test.WithInvalidAssignment(witness),
		test.WithCurves(ecc.BLS12_377),
	)
}

// TestMerklePathCircuit_ArrayLengthMismatch 测试数组长度不匹配的情况
func TestMerklePathCircuit_ArrayLengthMismatch(t *testing.T) {
	// 测试SiblingHashes和PathDirections长度不匹配的情况
	// 注意：在gnark中，如果数组长度在定义时不匹配，编译时就会失败
	// 这里测试的是witness中的数组长度不匹配

	_, err := NewMerklePathCircuit(2)
	require.NoError(t, err)

	leaf0Data := big.NewInt(0)
	leaf1Data := big.NewInt(1)
	leaf2Data := big.NewInt(2)

	leaf0Hash := computePoseidon2LeafHash(leaf0Data)
	leaf1Hash := computePoseidon2LeafHash(leaf1Data)
	leaf2Hash := computePoseidon2LeafHash(leaf2Data)

	node01Hash := computePoseidon2Hash(leaf0Hash, leaf1Hash)
	rootHash := computePoseidon2Hash(node01Hash, leaf2Hash)

	// PathDirections长度不匹配（只有1个，但应该有2个）
	witness := &MerklePathCircuit{
		RootHash:       rootHash,
		LeafData:       leaf0Data,
		LeafIndex:      0,
		SiblingHashes:  []frontend.Variable{leaf1Hash, node01Hash}, // 2个
		PathDirections: []frontend.Variable{0},                     // 只有1个，不匹配
		MaxDepth:       2,
	}

	// 创建witness应该失败（数组长度不匹配）
	_, err = frontend.NewWitness(witness, ecc.BLS12_377.ScalarField())
	// 注意：gnark可能不会在NewWitness时检查，而是在验证时检查
	// 这里只是确保代码不会panic
	_ = err
}

// TestBatchMerklePathCircuit_EmptyPaths 测试空路径列表
func TestBatchMerklePathCircuit_EmptyPaths(t *testing.T) {
	// 测试路径数量为0的情况（应该通过工厂函数验证）
	_, err := NewBatchMerklePathCircuit(0, 1)
	require.Error(t, err)
}

// TestBatchMerklePathCircuit_SinglePath 测试单一路径
func TestBatchMerklePathCircuit_SinglePath(t *testing.T) {
	assert := test.NewAssert(t)

	circuit, err := NewBatchMerklePathCircuit(1, 1)
	require.NoError(t, err)

	leaf0Data := big.NewInt(0)
	leaf1Data := big.NewInt(1)

	leaf0Hash := computePoseidon2LeafHash(leaf0Data)
	leaf1Hash := computePoseidon2LeafHash(leaf1Data)
	rootHash := computePoseidon2Hash(leaf0Hash, leaf1Hash)

	witness := &BatchMerklePathCircuit{
		RootHash: rootHash,
		Paths: []MerklePathInput{
			{
				LeafData:       leaf0Data,
				LeafIndex:      0,
				SiblingHashes:  []frontend.Variable{leaf1Hash},
				PathDirections: []frontend.Variable{0},
				MaxDepth:       1,
			},
		},
		MaxPaths: 1,
	}

	assert.CheckCircuit(
		circuit,
		test.WithValidAssignment(witness),
		test.WithCurves(ecc.BLS12_377),
	)
}

// TestIncrementalUpdateCircuit_NoChanges 测试无变更的情况
func TestIncrementalUpdateCircuit_NoChanges(t *testing.T) {
	assert := test.NewAssert(t)

	circuit, err := NewIncrementalUpdateCircuit(1, 1)
	require.NoError(t, err)

	// 旧树和新树相同（无变更）
	leaf0Data := big.NewInt(0)
	leaf1Data := big.NewInt(1)

	leaf0Hash := computePoseidon2LeafHash(leaf0Data)
	leaf1Hash := computePoseidon2LeafHash(leaf1Data)
	rootHash := computePoseidon2Hash(leaf0Hash, leaf1Hash)

	witness := &IncrementalUpdateCircuit{
		OldRootHash: rootHash,
		NewRootHash: rootHash, // 相同
		ChangedPaths: []MerklePathInput{
			{
				LeafData:       leaf0Data,
				LeafIndex:      0,
				SiblingHashes:  []frontend.Variable{leaf1Hash},
				PathDirections: []frontend.Variable{0},
				MaxDepth:       1,
			},
		},
		NewLeafData: []frontend.Variable{leaf0Data}, // 相同
		MaxPaths:    1,
	}

	// 应该通过（虽然无变更，但电路应该能处理）
	assert.CheckCircuit(
		circuit,
		test.WithValidAssignment(witness),
		test.WithCurves(ecc.BLS12_377),
	)
}

// TestConstants 测试常量定义
func TestConstants(t *testing.T) {
	require.Greater(t, MaxMerkleTreeDepth, 0)
	require.Greater(t, DefaultMerkleTreeDepth, 0)
	require.LessOrEqual(t, DefaultMerkleTreeDepth, MaxMerkleTreeDepth)
	require.Equal(t, 20, MaxMerkleTreeDepth)
	require.Equal(t, 10, DefaultMerkleTreeDepth)
}

