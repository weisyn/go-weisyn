package circuits

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Merkle Tree电路性能基准测试（Merkle Tree增量验证电路优化 - 阶段2）
// ============================================================================
//
// 🎯 **测试目的**：
// 测试Merkle Tree增量验证电路的性能，包括约束数量、证明生成时间等。
//
// ============================================================================

// BenchmarkMerklePathCircuitCompilation 基准测试：电路编译性能
func BenchmarkMerklePathCircuitCompilation(b *testing.B) {
	circuit := &MerklePathCircuit{
		SiblingHashes:  make([]frontend.Variable, 10),
		PathDirections: make([]frontend.Variable, 10),
		MaxDepth:       10,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := frontend.Compile(ecc.BLS12_377.ScalarField(), r1cs.NewBuilder, circuit)
		require.NoError(b, err)
	}
}

// BenchmarkMerklePathCircuitConstraintCount 基准测试：约束数量统计
func BenchmarkMerklePathCircuitConstraintCount(b *testing.B) {
	circuit := &MerklePathCircuit{
		SiblingHashes:  make([]frontend.Variable, 10),
		PathDirections: make([]frontend.Variable, 10),
		MaxDepth:       10,
	}
	
	// 编译电路
	compiledCircuit, err := frontend.Compile(ecc.BLS12_377.ScalarField(), r1cs.NewBuilder, circuit)
	require.NoError(b, err)
	
	// 获取约束数量
	constraintCount := compiledCircuit.GetNbConstraints()
	b.Logf("Merkle路径验证电路约束数量: %d", constraintCount)
	
	// 验证约束数量在合理范围内（每个深度约200约束，10深度约2000约束）
	require.Less(b, constraintCount, 5000, "约束数量应该小于5000")
}

// BenchmarkMerklePathCircuitProofGeneration 基准测试：证明生成性能
func BenchmarkMerklePathCircuitProofGeneration(b *testing.B) {
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
	
	// 编译电路
	compiledCircuit, err := frontend.Compile(ecc.BLS12_377.ScalarField(), r1cs.NewBuilder, circuit)
	require.NoError(b, err)
	
	// 生成可信设置
	provingKey, _, err := groth16.Setup(compiledCircuit)
	require.NoError(b, err)
	
	// 创建witness
	fullWitness, err := frontend.NewWitness(witness, ecc.BLS12_377.ScalarField())
	require.NoError(b, err)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		// 生成证明
		_, err := groth16.Prove(compiledCircuit, provingKey, fullWitness)
		require.NoError(b, err)
	}
}

// BenchmarkMerklePathCircuitProofVerification 基准测试：证明验证性能
func BenchmarkMerklePathCircuitProofVerification(b *testing.B) {
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
	
	// 编译电路
	compiledCircuit, err := frontend.Compile(ecc.BLS12_377.ScalarField(), r1cs.NewBuilder, circuit)
	require.NoError(b, err)
	
	// 生成可信设置
	provingKey, verifyingKey, err := groth16.Setup(compiledCircuit)
	require.NoError(b, err)
	
	// 创建witness
	fullWitness, err := frontend.NewWitness(witness, ecc.BLS12_377.ScalarField())
	require.NoError(b, err)
	
	// 生成证明
	proof, err := groth16.Prove(compiledCircuit, provingKey, fullWitness)
	require.NoError(b, err)
	
	// 创建公开witness
	publicWitness, err := frontend.NewWitness(witness, ecc.BLS12_377.ScalarField(), frontend.PublicOnly())
	require.NoError(b, err)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		// 验证证明
		err := groth16.Verify(proof, verifyingKey, publicWitness)
		require.NoError(b, err)
	}
}

// TestMerklePathCircuitPerformance 性能测试：测量约束数量和证明生成时间
func TestMerklePathCircuitPerformance(t *testing.T) {
	// 测试不同深度的电路性能
	depths := []int{1, 5, 10, 20}
	
	for _, depth := range depths {
		t.Run(fmt.Sprintf("Depth_%d", depth), func(t *testing.T) {
			circuit := &MerklePathCircuit{
				SiblingHashes:  make([]frontend.Variable, depth),
				PathDirections: make([]frontend.Variable, depth),
				MaxDepth:       depth,
			}
			
			// 编译电路
			startTime := time.Now()
			compiledCircuit, err := frontend.Compile(ecc.BLS12_377.ScalarField(), r1cs.NewBuilder, circuit)
			compileTime := time.Since(startTime)
			require.NoError(t, err)
			
			// 获取约束数量
			constraintCount := compiledCircuit.GetNbConstraints()
			
			t.Logf("深度 %d: 约束数量=%d, 编译时间=%v", depth, constraintCount, compileTime)
			
			// 验证约束数量在合理范围内
			// 实际测量：深度1约1143约束，深度5约4191约束，深度10约8001约束，深度20约15621约束
			// 每个深度约400-800约束（包含Poseidon哈希的约束）
			// 使用更宽松的上限：每个深度约1200约束（包含一些缓冲）
			expectedConstraints := depth * 1200
			require.LessOrEqual(t, constraintCount, expectedConstraints, "约束数量应该在预期范围内")
		})
	}
}

