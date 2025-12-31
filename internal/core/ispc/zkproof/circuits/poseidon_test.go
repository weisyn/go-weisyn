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
// PoseidonHasher测试
// ============================================================================
//
// 🎯 **测试目的**：
// 测试PoseidonHasher的功能，确保哈希计算正确。

// TestHash2Circuit 测试Hash2的电路
type TestHash2Circuit struct {
	Input1 frontend.Variable
	Input2 frontend.Variable
	Output frontend.Variable `gnark:",public"`
}

func (c *TestHash2Circuit) Define(api frontend.API) error {
	hasher, err := NewPoseidonHasher(api)
	if err != nil {
		return err
	}
	hash := hasher.Hash2(c.Input1, c.Input2)
	api.AssertIsEqual(hash, c.Output)
	return nil
}

// TestNewPoseidonHasher 测试创建PoseidonHasher
func TestNewPoseidonHasher(t *testing.T) {
	assert := test.NewAssert(t)

	circuit := &TestHash2Circuit{}

	// 创建测试数据
	input1 := big.NewInt(123)
	input2 := big.NewInt(456)

	// 计算期望的哈希值（使用链下Poseidon2）
	expectedHash := computePoseidon2Hash(input1, input2)

	// 创建witness
	witness := &TestHash2Circuit{
		Input1: input1,
		Input2: input2,
		Output: expectedHash,
	}

	// 运行测试
	assert.CheckCircuit(
		circuit,
		test.WithValidAssignment(witness),
		test.WithCurves(ecc.BLS12_377),
	)
}

// TestHashLeafCircuit 测试HashLeaf的电路
type TestHashLeafCircuit struct {
	LeafData frontend.Variable
	Output   frontend.Variable `gnark:",public"`
}

func (c *TestHashLeafCircuit) Define(api frontend.API) error {
	hasher, err := NewPoseidonHasher(api)
	if err != nil {
		return err
	}
	hash := hasher.HashLeaf(c.LeafData)
	api.AssertIsEqual(hash, c.Output)
	return nil
}

// TestPoseidonHasher_HashLeaf 测试HashLeaf方法
func TestPoseidonHasher_HashLeaf(t *testing.T) {
	assert := test.NewAssert(t)

	circuit := &TestHashLeafCircuit{}

	// 创建测试数据
	leafData := big.NewInt(789)

	// 计算期望的哈希值
	expectedHash := computePoseidon2LeafHash(leafData)

	// 创建witness
	witness := &TestHashLeafCircuit{
		LeafData: leafData,
		Output:   expectedHash,
	}

	// 运行测试
	assert.CheckCircuit(
		circuit,
		test.WithValidAssignment(witness),
		test.WithCurves(ecc.BLS12_377),
	)
}

// TestHashNodeCircuit 测试HashNode的电路
type TestHashNodeCircuit struct {
	LeftHash  frontend.Variable
	RightHash frontend.Variable
	Output    frontend.Variable `gnark:",public"`
}

func (c *TestHashNodeCircuit) Define(api frontend.API) error {
	hasher, err := NewPoseidonHasher(api)
	if err != nil {
		return err
	}
	hash := hasher.HashNode(c.LeftHash, c.RightHash)
	api.AssertIsEqual(hash, c.Output)
	return nil
}

// TestPoseidonHasher_HashNode 测试HashNode方法
func TestPoseidonHasher_HashNode(t *testing.T) {
	assert := test.NewAssert(t)

	circuit := &TestHashNodeCircuit{}

	// 创建测试数据
	leftHash := big.NewInt(111)
	rightHash := big.NewInt(222)

	// 计算期望的哈希值
	expectedHash := computePoseidon2Hash(leftHash, rightHash)

	// 创建witness
	witness := &TestHashNodeCircuit{
		LeftHash:  leftHash,
		RightHash: rightHash,
		Output:    expectedHash,
	}

	// 运行测试
	assert.CheckCircuit(
		circuit,
		test.WithValidAssignment(witness),
		test.WithCurves(ecc.BLS12_377),
	)
}

// TestConsistencyCircuit 测试一致性电路
type TestConsistencyCircuit struct {
	LeafData frontend.Variable
	Output1  frontend.Variable `gnark:",public"`
	Output2  frontend.Variable `gnark:",public"`
}

func (c *TestConsistencyCircuit) Define(api frontend.API) error {
	hasher, err := NewPoseidonHasher(api)
	if err != nil {
		return err
	}

	// HashLeaf
	hash1 := hasher.HashLeaf(c.LeafData)

	// Hash2(leaf, 0)
	hash2 := hasher.Hash2(c.LeafData, 0)

	// 应该相等
	api.AssertIsEqual(hash1, hash2)
	api.AssertIsEqual(hash1, c.Output1)
	api.AssertIsEqual(hash2, c.Output2)

	return nil
}

// TestPoseidonHasher_Consistency 测试哈希一致性
func TestPoseidonHasher_Consistency(t *testing.T) {
	assert := test.NewAssert(t)

	circuit := &TestConsistencyCircuit{}

	// 创建测试数据
	leafData := big.NewInt(999)
	expectedHash := computePoseidon2LeafHash(leafData)

	// 创建witness
	witness := &TestConsistencyCircuit{
		LeafData: leafData,
		Output1:  expectedHash,
		Output2:  expectedHash,
	}

	// 运行测试
	assert.CheckCircuit(
		circuit,
		test.WithValidAssignment(witness),
		test.WithCurves(ecc.BLS12_377),
	)
}

// BenchmarkPoseidonHasher_Hash2 基准测试Hash2性能
func BenchmarkPoseidonHasher_Hash2(b *testing.B) {
	circuit := &TestHash2Circuit{}

	input1 := big.NewInt(123)
	input2 := big.NewInt(456)
	expectedHash := computePoseidon2Hash(input1, input2)

	witness := &TestHash2Circuit{
		Input1: input1,
		Input2: input2,
		Output: expectedHash,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := frontend.Compile(ecc.BLS12_377.ScalarField(), nil, circuit)
		require.NoError(b, err)

		_, err = frontend.NewWitness(witness, ecc.BLS12_377.ScalarField())
		require.NoError(b, err)
	}
}
