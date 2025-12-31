package circuits

import (
	"bytes"
	"encoding/binary"
	"math/big"
	"testing"
	"time"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr/poseidon2"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/test"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/ispc/zkproof/incremental"
)

// createPoseidon2HashFunction 创建Poseidon2哈希函数适配器
//
// 🎯 **说明**：
// 将 incremental.HashFunction 接口适配为 Poseidon2 哈希函数。
// 这样 incremental 包可以使用 Poseidon2 构建 Merkle 树，与电路保持一致。
//
// 📋 **实现策略**：
// - 对于叶子节点：输入是整个序列化数据，转换为 field 元素（使用 big.Int），计算 hash(data, 0)
// - 对于内部节点：输入是两个32字节哈希值的拼接（64字节），拆分为两个 field 元素，计算 hash(left, right)
//
// ⚠️ **注意**：
// - Poseidon2 需要两个 field 元素作为输入
// - 使用与 computePoseidon2Hash 相同的实现方式，确保一致性
func createPoseidon2HashFunction() incremental.HashFunction {
	return func(data []byte) []byte {
		hasher := poseidon2.NewMerkleDamgardHasher()

		// 根据数据长度处理：
		// - 如果数据长度 <= 32 字节：作为第一个 field 元素，第二个为 0（叶子节点）
		// - 如果数据长度 > 32 字节：拆分为两个 field 元素（内部节点，64字节）
		var leftBig, rightBig big.Int

		if len(data) <= 32 {
			// 叶子节点：数据 <= 32 字节，作为第一个 field 元素
			leftBig.SetBytes(data)
			// rightBig 保持为 0
		} else {
			// 内部节点：数据是64字节（两个32字节哈希值的拼接）
			if len(data) >= 64 {
				leftBig.SetBytes(data[:32])
				rightBig.SetBytes(data[32:64])
			} else {
				// 数据长度在 32-64 字节之间，前32字节作为left，剩余作为right
				leftBig.SetBytes(data[:32])
				rightBig.SetBytes(data[32:])
			}
		}

		// 将 big.Int 转换为32字节（大端序），与 computePoseidon2Hash 保持一致
		leftBytes := make([]byte, 32)
		rightBytes := make([]byte, 32)
		leftBig.FillBytes(leftBytes)
		rightBig.FillBytes(rightBytes)

		// 计算 Poseidon2 哈希
		hasher.Write(leftBytes)
		hasher.Write(rightBytes)
		result := hasher.Sum(nil)

		// 返回32字节哈希值
		return result
	}
}

// createTestTraceRecord 创建测试用的 TraceRecord
//
// 🎯 **说明**：
// 为了测试目的，创建一个简单的序列化数据格式。
// 实际使用时，应使用 coordinator.Manager.serializeExecutionTraceForZK() 序列化 ExecutionTrace。
//
// 📋 **测试数据格式**：
// ID (string) + Data ([]byte) + Timestamp (int64, 8字节大端序)
func createTestTraceRecord(id string, data []byte, timestamp time.Time) *incremental.TraceRecord {
	var buf bytes.Buffer

	// 写入ID
	buf.WriteString(id)

	// 写入数据
	buf.Write(data)

	// 写入时间戳（Unix时间戳，8字节大端序）
	timestampUnix := uint64(timestamp.Unix())
	binary.Write(&buf, binary.BigEndian, timestampUnix)

	// 使用 NewTraceRecord 创建记录（使用 Poseidon2 哈希函数）
	poseidonHashFunc := createPoseidon2HashFunction()
	return incremental.NewTraceRecord(buf.Bytes(), poseidonHashFunc)
}

// ============================================================================
// Merkle Tree电路集成测试（Merkle Tree增量验证电路优化 - 阶段2）
// ============================================================================
//
// 🎯 **测试目的**：
// 测试Merkle Tree电路与incremental包的集成，使用真实的Merkle路径数据。
//
// ============================================================================

// TestMerklePathCircuitWithIncremental 测试：使用incremental包的真实数据
//
// 🎯 **测试目的**：
// 测试 Merkle Tree 电路与 incremental 包的集成，确保使用相同的 Poseidon2 哈希函数。
func TestMerklePathCircuitWithIncremental(t *testing.T) {
	assert := test.NewAssert(t)

	// 1. 使用incremental包构建Merkle Tree（使用Poseidon2哈希函数）
	poseidonHashFunc := createPoseidon2HashFunction()
	builder := incremental.NewMerkleTreeBuilder(poseidonHashFunc)

	// 创建测试记录（使用新的 TraceRecord 结构）
	now := time.Now()
	records := []*incremental.TraceRecord{
		createTestTraceRecord("record1", []byte("data1"), now),
		createTestTraceRecord("record2", []byte("data2"), now),
		createTestTraceRecord("record3", []byte("data3"), now),
		createTestTraceRecord("record4", []byte("data4"), now),
	}

	// 构建Merkle Tree
	tree, err := builder.BuildTree(records)
	require.NoError(t, err)
	require.NotNil(t, tree)

	// 2. 计算Merkle路径（使用incremental包）
	leafIndex := 0
	path, err := builder.CalculatePath(tree, leafIndex)
	require.NoError(t, err)
	require.NotNil(t, path)

	// 3. 将incremental包的路径转换为电路输入
	// 由于 incremental 包现在使用 Poseidon2 哈希，路径中的哈希值已经是 Poseidon2 哈希

	// 获取叶子数据（序列化后的数据）
	leafDataBytes := records[leafIndex].SerializedData
	// 将序列化数据转换为 field 元素（big.Int）
	// 注意：电路期望的是叶子数据的 field 元素表示，而不是哈希值
	// 我们需要将序列化数据转换为 big.Int
	var leafDataBig big.Int
	if len(leafDataBytes) <= 32 {
		leafDataBig.SetBytes(leafDataBytes)
	} else {
		// 如果数据超过32字节，只取前32字节
		leafDataBig.SetBytes(leafDataBytes[:32])
	}

	// 转换兄弟节点哈希（已经是 Poseidon2 哈希，32字节）
	siblingHashes := make([]frontend.Variable, len(path.SiblingHashes))
	for i, siblingHash := range path.SiblingHashes {
		siblingBig := new(big.Int).SetBytes(siblingHash)
		siblingHashes[i] = siblingBig
	}

	// 转换路径方向
	pathDirections := make([]frontend.Variable, len(path.PathDirections))
	for i, direction := range path.PathDirections {
		pathDirections[i] = direction
	}

	// 根哈希（已经是 Poseidon2 哈希）
	rootHashBig := new(big.Int).SetBytes(path.RootHash)

	// 4. 创建电路和witness
	circuit := &MerklePathCircuit{
		SiblingHashes:  make([]frontend.Variable, len(path.SiblingHashes)),
		PathDirections: make([]frontend.Variable, len(path.PathDirections)),
		MaxDepth:       10,
	}

	witness := &MerklePathCircuit{
		RootHash:       rootHashBig,
		LeafData:       leafDataBig,
		LeafIndex:      frontend.Variable(leafIndex),
		SiblingHashes:  siblingHashes,
		PathDirections: pathDirections,
		MaxDepth:       10,
	}

	// 5. 运行测试
	// 现在 incremental 包和电路都使用 Poseidon2 哈希，应该能够正确验证
	assert.CheckCircuit(
		circuit,
		test.WithValidAssignment(witness),
		test.WithCurves(ecc.BLS12_377),
	)
}

// TestIncrementalUpdateCircuitWithIncremental 测试：使用incremental包的增量更新
//
// 🎯 **测试目的**：
// 测试增量更新电路与 incremental 包的集成，确保使用相同的 Poseidon2 哈希函数。
func TestIncrementalUpdateCircuitWithIncremental(t *testing.T) {
	assert := test.NewAssert(t)

	// 1. 构建旧树（使用Poseidon2哈希函数）
	poseidonHashFunc := createPoseidon2HashFunction()
	builder := incremental.NewMerkleTreeBuilder(poseidonHashFunc)

	now := time.Now()
	oldRecords := []*incremental.TraceRecord{
		createTestTraceRecord("record1", []byte("data1"), now),
		createTestTraceRecord("record2", []byte("data2"), now),
	}

	oldTree, err := builder.BuildTree(oldRecords)
	require.NoError(t, err)

	// 2. 构建新树（更新第一个记录）
	newNow := now.Add(time.Second) // 使用不同的时间戳以确保数据不同
	newRecords := []*incremental.TraceRecord{
		createTestTraceRecord("record1", []byte("new_data1"), newNow), // 更新数据
		createTestTraceRecord("record2", []byte("data2"), now),        // 保持不变
	}

	newTree, err := builder.BuildTree(newRecords)
	require.NoError(t, err)

	// 3. 检测变更
	detector := incremental.NewChangeDetector(builder)
	changes, err := detector.DetectChanges(oldRecords, newRecords)
	require.NoError(t, err)
	require.Greater(t, len(changes), 0)

	// 4. 计算变更路径
	changedPaths, err := detector.CalculateChangedPaths(oldTree, changes)
	require.NoError(t, err)
	require.Greater(t, len(changedPaths), 0)

	// 5. 转换为电路输入
	// 由于 incremental 包现在使用 Poseidon2 哈希，所有哈希值都是 Poseidon2 哈希
	
	// ⚠️ **关键BUG修复说明**：在 gnark 中，数组长度必须在电路定义时固定
	// 
	// 🐛 **BUG描述**：
	// 如果 `path.SiblingHashes` 在定义时长度为 0，循环 `for j := 0; j < len(path.SiblingHashes); j++` 
	// 不会执行，导致哈希计算被跳过，电路验证失败。
	// 
	// ✅ **修复方法**：
	// 需要根据实际路径长度初始化数组。获取第一个路径的长度（所有路径应该有相同的深度），
	// 然后为每个路径的 `SiblingHashes` 和 `PathDirections` 分配正确的长度。
	if len(changedPaths) == 0 {
		t.Fatal("没有变更路径")
	}
	
	// 获取第一个路径的长度（所有路径应该有相同的深度）
	firstPathLen := len(changedPaths[0].SiblingHashes)
	
	circuit := &IncrementalUpdateCircuit{
		ChangedPaths: make([]MerklePathInput, len(changedPaths)),
		NewLeafData:  make([]frontend.Variable, len(changes)),
		MaxPaths:     5,
	}
	
	// 为每个路径初始化数组
	for i := range circuit.ChangedPaths {
		circuit.ChangedPaths[i] = MerklePathInput{
			SiblingHashes:  make([]frontend.Variable, firstPathLen),
			PathDirections: make([]frontend.Variable, firstPathLen),
			MaxDepth:       10,
		}
	}

	witness := &IncrementalUpdateCircuit{
		OldRootHash:  new(big.Int).SetBytes(oldTree.Root.Hash),
		NewRootHash:  new(big.Int).SetBytes(newTree.Root.Hash),
		ChangedPaths: make([]MerklePathInput, len(changedPaths)),
		NewLeafData:  make([]frontend.Variable, len(changes)),
		MaxPaths:     5,
	}

	// 转换变更路径
	for i, path := range changedPaths {
		siblingHashes := make([]frontend.Variable, len(path.SiblingHashes))
		for j, siblingHash := range path.SiblingHashes {
			siblingHashes[j] = new(big.Int).SetBytes(siblingHash)
		}

		pathDirections := make([]frontend.Variable, len(path.PathDirections))
		for j, direction := range path.PathDirections {
			pathDirections[j] = direction
		}

		// 叶子数据（使用原始序列化数据的 field 元素表示）
		// 电路会计算 hash(LeafData, 0) 得到叶子节点哈希
		var leafDataBig big.Int
		if i < len(changes) && changes[i].OldRecord != nil {
			leafDataBytes := changes[i].OldRecord.SerializedData
			if len(leafDataBytes) <= 32 {
				leafDataBig.SetBytes(leafDataBytes)
			} else {
				// 如果数据超过32字节，只取前32字节
				leafDataBig.SetBytes(leafDataBytes[:32])
			}
		} else {
			// 如果没有旧记录，使用路径中的叶子哈希（这种情况不应该发生）
			leafDataBig.SetBytes(path.LeafHash)
		}

		witness.ChangedPaths[i] = MerklePathInput{
			LeafData:       leafDataBig,
			LeafIndex:      frontend.Variable(path.LeafIndex),
			SiblingHashes:  siblingHashes,
			PathDirections: pathDirections,
			MaxDepth:       10,
		}

		// 新叶子数据（使用原始序列化数据的 field 元素表示）
		// 电路会计算 hash(NewLeafData, 0) 得到新叶子节点哈希
		if i < len(changes) && changes[i].NewRecord != nil {
			newLeafDataBytes := changes[i].NewRecord.SerializedData
			var newLeafDataBig big.Int
			if len(newLeafDataBytes) <= 32 {
				newLeafDataBig.SetBytes(newLeafDataBytes)
			} else {
				// 如果数据超过32字节，只取前32字节
				newLeafDataBig.SetBytes(newLeafDataBytes[:32])
			}
			witness.NewLeafData[i] = newLeafDataBig
		}
	}

	// 6. 运行测试
	// 现在 incremental 包和电路都使用 Poseidon2 哈希，应该能够正确验证
	assert.CheckCircuit(
		circuit,
		test.WithValidAssignment(witness),
		test.WithCurves(ecc.BLS12_377),
	)
}
