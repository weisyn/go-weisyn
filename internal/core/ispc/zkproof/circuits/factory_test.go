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
// 工厂函数测试
// ============================================================================
//
// 🎯 **测试目的**：
// 测试工厂函数的正确性和错误处理，确保电路实例正确初始化。

// TestNewMerklePathCircuit_Success 测试成功创建Merkle路径电路
func TestNewMerklePathCircuit_Success(t *testing.T) {
	// 测试正常情况
	circuit, err := NewMerklePathCircuit(5)
	require.NoError(t, err)
	require.NotNil(t, circuit)
	require.Equal(t, 5, len(circuit.SiblingHashes))
	require.Equal(t, 5, len(circuit.PathDirections))
	require.Equal(t, 5, circuit.MaxDepth)

	// 测试默认深度
	circuit, err = NewMerklePathCircuit(DefaultMerkleTreeDepth)
	require.NoError(t, err)
	require.NotNil(t, circuit)
	require.Equal(t, DefaultMerkleTreeDepth, len(circuit.SiblingHashes))

	// 测试最大深度
	circuit, err = NewMerklePathCircuit(MaxMerkleTreeDepth)
	require.NoError(t, err)
	require.NotNil(t, circuit)
	require.Equal(t, MaxMerkleTreeDepth, len(circuit.SiblingHashes))
}

// TestNewMerklePathCircuit_Errors 测试错误情况
func TestNewMerklePathCircuit_Errors(t *testing.T) {
	// 测试深度为0
	circuit, err := NewMerklePathCircuit(0)
	require.Error(t, err)
	require.Nil(t, circuit)
	require.Contains(t, err.Error(), "路径深度必须大于0")

	// 测试负深度
	circuit, err = NewMerklePathCircuit(-1)
	require.Error(t, err)
	require.Nil(t, circuit)
	require.Contains(t, err.Error(), "路径深度必须大于0")

	// 测试超过最大深度
	circuit, err = NewMerklePathCircuit(MaxMerkleTreeDepth + 1)
	require.Error(t, err)
	require.Nil(t, circuit)
	require.Contains(t, err.Error(), "路径深度超过最大限制")
}

// TestNewMerklePathCircuit_WithCircuit 测试使用工厂函数创建的电路能否正常工作
func TestNewMerklePathCircuit_WithCircuit(t *testing.T) {
	assert := test.NewAssert(t)

	// 使用工厂函数创建电路
	circuit, err := NewMerklePathCircuit(1)
	require.NoError(t, err)

	// 创建测试数据
	leaf0Data := big.NewInt(0)
	leaf1Data := big.NewInt(1)

	leaf0Hash := computePoseidon2LeafHash(leaf0Data)
	leaf1Hash := computePoseidon2LeafHash(leaf1Data)
	rootHash := computePoseidon2Hash(leaf0Hash, leaf1Hash)

	// 创建witness
	witness := &MerklePathCircuit{
		RootHash:       rootHash,
		LeafData:       leaf0Data,
		LeafIndex:      0,
		SiblingHashes:  []frontend.Variable{leaf1Hash},
		PathDirections: []frontend.Variable{0},
		MaxDepth:       1,
	}

	// 运行测试
	assert.CheckCircuit(
		circuit,
		test.WithValidAssignment(witness),
		test.WithCurves(ecc.BLS12_377),
	)
}

// TestNewBatchMerklePathCircuit_Success 测试成功创建批量路径电路
func TestNewBatchMerklePathCircuit_Success(t *testing.T) {
	// 测试正常情况
	circuit, err := NewBatchMerklePathCircuit(3, 2)
	require.NoError(t, err)
	require.NotNil(t, circuit)
	require.Equal(t, 3, len(circuit.Paths))
	require.Equal(t, 3, circuit.MaxPaths)
	for _, path := range circuit.Paths {
		require.Equal(t, 2, len(path.SiblingHashes))
		require.Equal(t, 2, len(path.PathDirections))
		require.Equal(t, 2, path.MaxDepth)
	}
}

// TestNewBatchMerklePathCircuit_Errors 测试批量路径电路的错误情况
func TestNewBatchMerklePathCircuit_Errors(t *testing.T) {
	// 测试路径数量为0
	circuit, err := NewBatchMerklePathCircuit(0, 2)
	require.Error(t, err)
	require.Nil(t, circuit)
	require.Contains(t, err.Error(), "路径数量必须大于0")

	// 测试路径数量为负
	circuit, err = NewBatchMerklePathCircuit(-1, 2)
	require.Error(t, err)
	require.Nil(t, circuit)

	// 测试深度为0
	circuit, err = NewBatchMerklePathCircuit(2, 0)
	require.Error(t, err)
	require.Nil(t, circuit)
	require.Contains(t, err.Error(), "路径深度必须大于0")

	// 测试超过最大深度
	circuit, err = NewBatchMerklePathCircuit(2, MaxMerkleTreeDepth+1)
	require.Error(t, err)
	require.Nil(t, circuit)
	require.Contains(t, err.Error(), "路径深度超过最大限制")
}

// TestNewBatchMerklePathCircuit_WithCircuit 测试使用工厂函数创建的批量电路能否正常工作
func TestNewBatchMerklePathCircuit_WithCircuit(t *testing.T) {
	assert := test.NewAssert(t)

	// 使用工厂函数创建电路
	circuit, err := NewBatchMerklePathCircuit(2, 2)
	require.NoError(t, err)

	// 创建测试数据
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

	// 创建witness
	witness := &BatchMerklePathCircuit{
		RootHash: rootHash,
		Paths: []MerklePathInput{
			{
				LeafData:       leaf0Data,
				LeafIndex:      0,
				SiblingHashes:  []frontend.Variable{leaf1Hash, node23Hash},
				PathDirections: []frontend.Variable{0, 0},
				MaxDepth:       2,
			},
			{
				LeafData:       leaf2Data,
				LeafIndex:      2,
				SiblingHashes:  []frontend.Variable{leaf3Hash, node01Hash},
				PathDirections: []frontend.Variable{0, 1},
				MaxDepth:       2,
			},
		},
		MaxPaths: 2,
	}

	// 运行测试
	assert.CheckCircuit(
		circuit,
		test.WithValidAssignment(witness),
		test.WithCurves(ecc.BLS12_377),
	)
}

// TestNewIncrementalUpdateCircuit_Success 测试成功创建增量更新电路
func TestNewIncrementalUpdateCircuit_Success(t *testing.T) {
	// 测试正常情况
	circuit, err := NewIncrementalUpdateCircuit(2, 1)
	require.NoError(t, err)
	require.NotNil(t, circuit)
	require.Equal(t, 2, len(circuit.ChangedPaths))
	require.Equal(t, 2, len(circuit.NewLeafData))
	require.Equal(t, 2, circuit.MaxPaths)
	for _, path := range circuit.ChangedPaths {
		require.Equal(t, 1, len(path.SiblingHashes))
		require.Equal(t, 1, len(path.PathDirections))
	}
}

// TestNewIncrementalUpdateCircuit_Errors 测试增量更新电路的错误情况
func TestNewIncrementalUpdateCircuit_Errors(t *testing.T) {
	// 测试路径数量为0
	circuit, err := NewIncrementalUpdateCircuit(0, 1)
	require.Error(t, err)
	require.Nil(t, circuit)
	require.Contains(t, err.Error(), "路径数量必须大于0")

	// 测试深度为0
	circuit, err = NewIncrementalUpdateCircuit(2, 0)
	require.Error(t, err)
	require.Nil(t, circuit)
	require.Contains(t, err.Error(), "路径深度必须大于0")

	// 测试超过最大深度
	circuit, err = NewIncrementalUpdateCircuit(2, MaxMerkleTreeDepth+1)
	require.Error(t, err)
	require.Nil(t, circuit)
	require.Contains(t, err.Error(), "路径深度超过最大限制")
}

// TestNewIncrementalUpdateCircuit_WithCircuit 测试使用工厂函数创建的增量更新电路能否正常工作
func TestNewIncrementalUpdateCircuit_WithCircuit(t *testing.T) {
	assert := test.NewAssert(t)

	// 使用工厂函数创建电路
	circuit, err := NewIncrementalUpdateCircuit(1, 1)
	require.NoError(t, err)

	// 创建测试数据
	oldLeaf0Data := big.NewInt(0)
	oldLeaf1Data := big.NewInt(1)
	newLeaf0Data := big.NewInt(10)

	oldLeaf0Hash := computePoseidon2LeafHash(oldLeaf0Data)
	oldLeaf1Hash := computePoseidon2LeafHash(oldLeaf1Data)
	oldRootHash := computePoseidon2Hash(oldLeaf0Hash, oldLeaf1Hash)

	newLeaf0Hash := computePoseidon2LeafHash(newLeaf0Data)
	newRootHash := computePoseidon2Hash(newLeaf0Hash, oldLeaf1Hash)

	// 创建witness
	witness := &IncrementalUpdateCircuit{
		OldRootHash: oldRootHash,
		NewRootHash: newRootHash,
		ChangedPaths: []MerklePathInput{
			{
				LeafData:       oldLeaf0Data,
				LeafIndex:      0,
				SiblingHashes:  []frontend.Variable{oldLeaf1Hash},
				PathDirections: []frontend.Variable{0},
				MaxDepth:       1,
			},
		},
		NewLeafData: []frontend.Variable{newLeaf0Data},
		MaxPaths:    1,
	}

	// 运行测试
	assert.CheckCircuit(
		circuit,
		test.WithValidAssignment(witness),
		test.WithCurves(ecc.BLS12_377),
	)
}

// TestCreateMerklePathCircuitFromPath 测试CreateMerklePathCircuitFromPath函数
func TestCreateMerklePathCircuitFromPath(t *testing.T) {
	// 测试正常情况
	circuit, err := CreateMerklePathCircuitFromPath(5)
	require.NoError(t, err)
	require.NotNil(t, circuit)
	require.Equal(t, 5, len(circuit.SiblingHashes))

	// 测试错误情况（应该与NewMerklePathCircuit相同）
	circuit, err = CreateMerklePathCircuitFromPath(0)
	require.Error(t, err)
	require.Nil(t, circuit)
}

// TestFactoryFunctions_MaxDepth 测试最大深度边界情况
func TestFactoryFunctions_MaxDepth(t *testing.T) {
	// 测试最大深度
	circuit, err := NewMerklePathCircuit(MaxMerkleTreeDepth)
	require.NoError(t, err)
	require.NotNil(t, circuit)
	require.Equal(t, MaxMerkleTreeDepth, len(circuit.SiblingHashes))

	// 测试超过最大深度
	circuit, err = NewMerklePathCircuit(MaxMerkleTreeDepth + 1)
	require.Error(t, err)
	require.Nil(t, circuit)
}

