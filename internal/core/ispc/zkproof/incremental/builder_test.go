package incremental

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// ============================================================================
// builder.go 测试
// ============================================================================

// TestNewMerkleTreeBuilder 测试创建Merkle树构建器
func TestNewMerkleTreeBuilder(t *testing.T) {
	// 测试使用默认哈希函数
	builder := NewMerkleTreeBuilder(nil)
	require.NotNil(t, builder)
	require.NotNil(t, builder.hashFunc)
	
	// 测试使用自定义哈希函数
	customHashFunc := func(data []byte) []byte {
		return data[:min(8, len(data))]
	}
	builder2 := NewMerkleTreeBuilder(customHashFunc)
	require.NotNil(t, builder2)
	require.NotNil(t, builder2.hashFunc)
}

// TestBuildTree_SingleRecord 测试构建单记录树
func TestBuildTree_SingleRecord(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	
	records := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
	}
	
	tree, err := builder.BuildTree(records)
	require.NoError(t, err)
	require.NotNil(t, tree)
	require.Equal(t, 1, tree.LeafCount)
	require.Equal(t, 0, tree.Depth) // 单节点树深度为0
	require.NotNil(t, tree.Root)
	require.True(t, tree.Root.IsLeaf)
	require.Equal(t, records[0], tree.Root.Data)
}

// TestBuildTree_TwoRecords 测试构建两记录树
func TestBuildTree_TwoRecords(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	
	records := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
	}
	
	tree, err := builder.BuildTree(records)
	require.NoError(t, err)
	require.NotNil(t, tree)
	require.Equal(t, 2, tree.LeafCount)
	require.Equal(t, 1, tree.Depth)
	require.NotNil(t, tree.Root)
	require.False(t, tree.Root.IsLeaf)
	require.NotNil(t, tree.Root.Left)
	require.NotNil(t, tree.Root.Right)
	require.True(t, tree.Root.Left.IsLeaf)
	require.True(t, tree.Root.Right.IsLeaf)
}

// TestBuildTree_FourRecords 测试构建四记录树
func TestBuildTree_FourRecords(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	
	records := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
		NewTraceRecord([]byte("record3"), nil),
		NewTraceRecord([]byte("record4"), nil),
	}
	
	tree, err := builder.BuildTree(records)
	require.NoError(t, err)
	require.NotNil(t, tree)
	require.Equal(t, 4, tree.LeafCount)
	require.Equal(t, 2, tree.Depth)
	require.NotNil(t, tree.Root)
}

// TestBuildTree_EmptyRecords 测试空记录列表
func TestBuildTree_EmptyRecords(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	
	tree, err := builder.BuildTree([]*TraceRecord{})
	require.Error(t, err)
	require.Nil(t, tree)
	require.Contains(t, err.Error(), "记录列表不能为空")
}

// TestBuildTree_NilRecord 测试nil记录
func TestBuildTree_NilRecord(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	
	// 创建包含nil的记录
	record := &TraceRecord{
		SerializedData: nil,
		Hash:           nil,
	}
	
	tree, err := builder.BuildTree([]*TraceRecord{record})
	require.Error(t, err)
	require.Nil(t, tree)
	require.Contains(t, err.Error(), "序列化数据为空")
}

// TestBuildTree_OddRecords 测试奇数个记录
func TestBuildTree_OddRecords(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	
	records := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
		NewTraceRecord([]byte("record3"), nil),
	}
	
	tree, err := builder.BuildTree(records)
	require.NoError(t, err)
	require.NotNil(t, tree)
	require.Equal(t, 3, tree.LeafCount)
}

// TestCalculatePath 测试计算路径
func TestCalculatePath(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	
	records := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
		NewTraceRecord([]byte("record3"), nil),
		NewTraceRecord([]byte("record4"), nil),
	}
	
	tree, err := builder.BuildTree(records)
	require.NoError(t, err)
	
	// 计算第一个叶子的路径
	path, err := builder.CalculatePath(tree, 0)
	require.NoError(t, err)
	require.NotNil(t, path)
	require.Equal(t, 0, path.LeafIndex)
	require.NotNil(t, path.LeafHash)
	require.NotNil(t, path.RootHash)
	require.Equal(t, tree.Root.Hash, path.RootHash)
	require.Equal(t, tree.Depth, len(path.SiblingHashes))
	require.Equal(t, tree.Depth, len(path.PathDirections))
}

// TestCalculatePath_InvalidIndex 测试无效索引
func TestCalculatePath_InvalidIndex(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	
	records := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
	}
	
	tree, err := builder.BuildTree(records)
	require.NoError(t, err)
	
	// 测试负索引
	path, err := builder.CalculatePath(tree, -1)
	require.Error(t, err)
	require.Nil(t, path)
	require.Contains(t, err.Error(), "叶子节点索引超出范围")
	
	// 测试超出范围的索引
	path, err = builder.CalculatePath(tree, 10)
	require.Error(t, err)
	require.Nil(t, path)
	require.Contains(t, err.Error(), "叶子节点索引超出范围")
}

// TestCalculatePath_NilTree 测试nil树
func TestCalculatePath_NilTree(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	
	path, err := builder.CalculatePath(nil, 0)
	require.Error(t, err)
	require.Nil(t, path)
	require.Contains(t, err.Error(), "树不能为空")
}

// TestVerifyPath 测试验证路径
func TestVerifyPath(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	
	records := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
	}
	
	tree, err := builder.BuildTree(records)
	require.NoError(t, err)
	
	path, err := builder.CalculatePath(tree, 0)
	require.NoError(t, err)
	
	// 验证路径应该成功
	isValid := builder.VerifyPath(path)
	require.True(t, isValid)
}

// TestVerifyPath_InvalidPath 测试无效路径
func TestVerifyPath_InvalidPath(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	
	records := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
	}
	
	tree, err := builder.BuildTree(records)
	require.NoError(t, err)
	
	path, err := builder.CalculatePath(tree, 0)
	require.NoError(t, err)
	
	// 修改根哈希，使路径无效
	path.RootHash = []byte("invalid hash")
	isValid := builder.VerifyPath(path)
	require.False(t, isValid)
}

// TestVerifyPath_NilPath 测试nil路径
func TestVerifyPath_NilPath(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	
	isValid := builder.VerifyPath(nil)
	require.False(t, isValid)
}

// TestVerifyPath_LengthMismatch 测试长度不匹配
func TestVerifyPath_LengthMismatch(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	
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
	isValid := builder.VerifyPath(path)
	require.False(t, isValid)
}

// TestExtractRecords 测试提取记录
func TestExtractRecords(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	
	records := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
		NewTraceRecord([]byte("record3"), nil),
	}
	
	tree, err := builder.BuildTree(records)
	require.NoError(t, err)
	
	extracted := builder.ExtractRecords(tree)
	// 注意：当有奇数个记录时，BuildTree会复制最后一个节点，所以提取的记录数可能大于原始记录数
	// 但至少应该包含所有原始记录
	require.GreaterOrEqual(t, len(extracted), len(records))
	
	// 验证所有原始记录都在提取结果中
	for _, record := range records {
		found := false
		for _, extractedRecord := range extracted {
			if RecordsEqual(record, extractedRecord) {
				found = true
				break
			}
		}
		require.True(t, found, "记录 %s 未在提取结果中找到", string(record.SerializedData))
	}
}

// TestRebuildTree_NoChanges 测试无变更重建
func TestRebuildTree_NoChanges(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	
	records := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
	}
	
	oldTree, err := builder.BuildTree(records)
	require.NoError(t, err)
	
	// 无变更重建
	newTree, err := builder.RebuildTree(oldTree, []*ChangeInfo{})
	require.NoError(t, err)
	require.NotNil(t, newTree)
	require.Equal(t, oldTree.LeafCount, newTree.LeafCount)
	require.Equal(t, oldTree.Depth, newTree.Depth)
	require.Equal(t, oldTree.Root.Hash, newTree.Root.Hash)
}

// TestRebuildTree_Modify 测试修改记录
func TestRebuildTree_Modify(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	
	oldRecords := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
	}
	
	oldTree, err := builder.BuildTree(oldRecords)
	require.NoError(t, err)
	
	// 修改第一个记录
	newRecord := NewTraceRecord([]byte("modified record1"), nil)
	changes := []*ChangeInfo{
		{
			Type:      ChangeTypeModified,
			Index:     0,
			OldRecord: oldRecords[0],
			NewRecord: newRecord,
		},
	}
	
	newTree, err := builder.RebuildTree(oldTree, changes)
	require.NoError(t, err)
	require.NotNil(t, newTree)
	require.Equal(t, oldTree.LeafCount, newTree.LeafCount)
	require.NotEqual(t, oldTree.Root.Hash, newTree.Root.Hash) // 根哈希应该改变
}

// TestRebuildTree_Add 测试添加记录
func TestRebuildTree_Add(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	
	oldRecords := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
	}
	
	oldTree, err := builder.BuildTree(oldRecords)
	require.NoError(t, err)
	
	// 添加新记录
	newRecord := NewTraceRecord([]byte("record3"), nil)
	changes := []*ChangeInfo{
		{
			Type:      ChangeTypeAdded,
			Index:     2,
			OldRecord: nil,
			NewRecord: newRecord,
		},
	}
	
	newTree, err := builder.RebuildTree(oldTree, changes)
	require.NoError(t, err)
	require.NotNil(t, newTree)
	require.Equal(t, oldTree.LeafCount+1, newTree.LeafCount)
	require.NotEqual(t, oldTree.Root.Hash, newTree.Root.Hash)
}

// TestRebuildTree_Delete 测试删除记录
func TestRebuildTree_Delete(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	
	oldRecords := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
		NewTraceRecord([]byte("record3"), nil),
	}
	
	oldTree, err := builder.BuildTree(oldRecords)
	require.NoError(t, err)
	
	// 删除第一个记录
	changes := []*ChangeInfo{
		{
			Type:      ChangeTypeDeleted,
			Index:     0,
			OldRecord: oldRecords[0],
			NewRecord: nil,
		},
	}
	
	newTree, err := builder.RebuildTree(oldTree, changes)
	require.NoError(t, err)
	require.NotNil(t, newTree)
	
	// 验证新树包含的记录数（删除后应该有2个记录）
	// 注意：由于RebuildTree会重新构建树，所以LeafCount应该反映实际记录数
	newRecords := builder.ExtractRecords(newTree)
	// 去重检查：应该包含剩余的2个记录
	uniqueRecords := make(map[string]bool)
	for _, r := range newRecords {
		uniqueRecords[string(r.SerializedData)] = true
	}
	require.Equal(t, 2, len(uniqueRecords), "删除后应该有2个唯一记录")
	require.NotEqual(t, oldTree.Root.Hash, newTree.Root.Hash)
}

// TestRebuildTree_NilTree 测试nil树
func TestRebuildTree_NilTree(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	
	newTree, err := builder.RebuildTree(nil, []*ChangeInfo{})
	require.Error(t, err)
	require.Nil(t, newTree)
	require.Contains(t, err.Error(), "旧树不能为空")
}

