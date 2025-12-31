package incremental

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// ============================================================================
// verifier.go 边界情况测试
// ============================================================================

// TestRecalculateRootHash_NoChanges 测试无变更情况
func TestRecalculateRootHash_NoChanges(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	verifier := NewIncrementalVerifier(builder)
	
	records := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
	}
	tree, err := builder.BuildTree(records)
	require.NoError(t, err)
	
	proof := &IncrementalVerificationProof{
		OldRootHash:    tree.Root.Hash,
		ChangedPaths:   []*MerklePath{},
		ChangedRecords: []*TraceRecord{},
		NewRootHash:    tree.Root.Hash,
	}
	
	// 通过 VerifyProof 间接测试 recalculateRootHash
	isValid, err := verifier.VerifyProof(proof, tree.Root.Hash)
	require.NoError(t, err)
	require.True(t, isValid)
}

// TestRecalculateRootHash_OnlyAddedRecords 测试只有新增记录
func TestRecalculateRootHash_OnlyAddedRecords(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	verifier := NewIncrementalVerifier(builder)
	
	records := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
	}
	tree, err := builder.BuildTree(records)
	require.NoError(t, err)
	
	proof := &IncrementalVerificationProof{
		OldRootHash:    tree.Root.Hash,
		ChangedPaths:   []*MerklePath{}, // 无路径（只有新增）
		ChangedRecords: []*TraceRecord{NewTraceRecord([]byte("record3"), nil)},
		NewRootHash:    tree.Root.Hash,
	}
	
	// 应该失败，因为无法重新计算根哈希
	isValid, err := verifier.VerifyProof(proof, tree.Root.Hash)
	require.Error(t, err)
	require.False(t, isValid)
	require.Contains(t, err.Error(), "无法重新计算根哈希")
}

// TestRecalculateRootHash_OnlyDeletedRecords 测试只有删除记录
func TestRecalculateRootHash_OnlyDeletedRecords(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	verifier := NewIncrementalVerifier(builder)
	
	records := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
	}
	tree, err := builder.BuildTree(records)
	require.NoError(t, err)
	
	// 计算删除记录的路径
	path, err := builder.CalculatePath(tree, 1)
	require.NoError(t, err)
	
	proof := &IncrementalVerificationProof{
		OldRootHash:    tree.Root.Hash,
		ChangedPaths:   []*MerklePath{path}, // 有路径（删除）
		ChangedRecords: []*TraceRecord{},   // 无记录
		NewRootHash:    tree.Root.Hash,
	}
	
	// 应该失败，因为无法重新计算根哈希（只有删除记录）
	isValid, err := verifier.VerifyProof(proof, tree.Root.Hash)
	require.Error(t, err)
	require.False(t, isValid)
	require.Contains(t, err.Error(), "无法重新计算根哈希")
}

// TestRecalculateRootHash_MultiplePaths 测试多个路径
func TestRecalculateRootHash_MultiplePaths(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	verifier := NewIncrementalVerifier(builder)
	
	records := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
		NewTraceRecord([]byte("record3"), nil),
		NewTraceRecord([]byte("record4"), nil),
	}
	tree, err := builder.BuildTree(records)
	require.NoError(t, err)
	
	// 计算两个修改记录的路径
	path1, err := builder.CalculatePath(tree, 0)
	require.NoError(t, err)
	path2, err := builder.CalculatePath(tree, 2)
	require.NoError(t, err)

	// 构造新记录并计算期望的新根哈希（通过重建整棵树）
	newRecords := []*TraceRecord{
		NewTraceRecord([]byte("modified record1"), nil),
		records[1],
		NewTraceRecord([]byte("modified record3"), nil),
		records[3],
	}
	newTree, err := builder.BuildTree(newRecords)
	require.NoError(t, err)
	
	proof := &IncrementalVerificationProof{
		OldRootHash: tree.Root.Hash,
		ChangedPaths: []*MerklePath{path1, path2},
		ChangedRecords: []*TraceRecord{
			newRecords[0],
			newRecords[2],
		},
		NewRootHash: newTree.Root.Hash,
	}
	
	// 多路径合并应能通过验证
	isValid, err := verifier.VerifyProof(proof, tree.Root.Hash)
	require.NoError(t, err)
	require.True(t, isValid)
}

// TestRecalculateRootHashFromPath 测试从路径重新计算根哈希
func TestRecalculateRootHashFromPath(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	verifier := NewIncrementalVerifier(builder)
	
	records := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
	}
	tree, err := builder.BuildTree(records)
	require.NoError(t, err)
	
	// 计算路径
	path, err := builder.CalculatePath(tree, 0)
	require.NoError(t, err)
	
	// 修改记录
	newRecord := NewTraceRecord([]byte("modified record1"), nil)
	
	// 重新计算根哈希
	newRootHash, err := verifier.recalculateRootHashFromPath(path, newRecord)
	require.NoError(t, err)
	require.NotNil(t, newRootHash)
	require.NotEqual(t, tree.Root.Hash, newRootHash)
	
	// 验证新根哈希（通过重新构建树）
	newTree, err := builder.BuildTree([]*TraceRecord{newRecord, records[1]})
	require.NoError(t, err)
	require.Equal(t, newTree.Root.Hash, newRootHash)
}

// TestRecalculateRootHashFromPath_NilPath 测试nil路径
func TestRecalculateRootHashFromPath_NilPath(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	verifier := NewIncrementalVerifier(builder)
	
	record := NewTraceRecord([]byte("record1"), nil)
	
	_, err := verifier.recalculateRootHashFromPath(nil, record)
	require.Error(t, err)
	require.Contains(t, err.Error(), "变更路径不能为空")
}

// TestRecalculateRootHashFromPath_NilRecord 测试nil记录
func TestRecalculateRootHashFromPath_NilRecord(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	verifier := NewIncrementalVerifier(builder)
	
	records := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
	}
	tree, err := builder.BuildTree(records)
	require.NoError(t, err)
	
	path, err := builder.CalculatePath(tree, 0)
	require.NoError(t, err)
	
	_, err = verifier.recalculateRootHashFromPath(path, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "变更记录不能为空")
}

// TestRecalculateRootHashFromPath_LengthMismatch 测试路径长度不匹配
func TestRecalculateRootHashFromPath_LengthMismatch(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	verifier := NewIncrementalVerifier(builder)
	
	records := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
	}
	tree, err := builder.BuildTree(records)
	require.NoError(t, err)
	
	path, err := builder.CalculatePath(tree, 0)
	require.NoError(t, err)
	
	// 修改路径长度，使长度不匹配
	path.SiblingHashes = path.SiblingHashes[:len(path.SiblingHashes)-1]
	
	record := NewTraceRecord([]byte("modified record1"), nil)
	_, err = verifier.recalculateRootHashFromPath(path, record)
	require.Error(t, err)
	require.Contains(t, err.Error(), "路径长度不一致")
}

// TestRecalculateRootHash_InconsistentRootHashes 测试不一致的根哈希
func TestRecalculateRootHash_InconsistentRootHashes(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	verifier := NewIncrementalVerifier(builder)
	
	records1 := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
	}
	tree1, err := builder.BuildTree(records1)
	require.NoError(t, err)
	
	records2 := []*TraceRecord{
		NewTraceRecord([]byte("record3"), nil),
		NewTraceRecord([]byte("record4"), nil),
	}
	tree2, err := builder.BuildTree(records2)
	require.NoError(t, err)
	
	// 创建来自不同树的路径
	path1, err := builder.CalculatePath(tree1, 0)
	require.NoError(t, err)
	path2, err := builder.CalculatePath(tree2, 0)
	require.NoError(t, err)
	
	proof := &IncrementalVerificationProof{
		OldRootHash: tree1.Root.Hash,
		ChangedPaths: []*MerklePath{path1, path2}, // 来自不同的树
		ChangedRecords: []*TraceRecord{
			NewTraceRecord([]byte("modified record1"), nil),
			NewTraceRecord([]byte("modified record3"), nil),
		},
		NewRootHash: tree1.Root.Hash,
	}
	
	// 应该失败，因为路径的根哈希与旧根哈希不匹配（path2来自tree2，根哈希不同）
	isValid, err := verifier.VerifyProof(proof, tree1.Root.Hash)
	require.Error(t, err)
	require.False(t, isValid)
	require.Contains(t, err.Error(), "变更路径的根哈希与旧根哈希不匹配")
}

