package incremental

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// ============================================================================
// detector.go 测试
// ============================================================================

// TestNewChangeDetector 测试创建变更检测器
func TestNewChangeDetector(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	detector := NewChangeDetector(builder)
	require.NotNil(t, detector)
	require.Equal(t, builder, detector.builder)
}

// TestDetectChanges_NoChanges 测试无变更
func TestDetectChanges_NoChanges(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	detector := NewChangeDetector(builder)
	
	records := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
	}
	
	changes, err := detector.DetectChanges(records, records)
	require.NoError(t, err)
	require.Equal(t, 0, len(changes))
}

// TestDetectChanges_Modified 测试修改记录
func TestDetectChanges_Modified(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	detector := NewChangeDetector(builder)
	
	oldRecords := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
	}
	
	newRecords := []*TraceRecord{
		NewTraceRecord([]byte("modified record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
	}
	
	changes, err := detector.DetectChanges(oldRecords, newRecords)
	require.NoError(t, err)
	require.Equal(t, 1, len(changes))
	require.Equal(t, ChangeTypeModified, changes[0].Type)
	require.Equal(t, 0, changes[0].Index)
	require.True(t, RecordsEqual(oldRecords[0], changes[0].OldRecord))
	require.True(t, RecordsEqual(newRecords[0], changes[0].NewRecord))
}

// TestDetectChanges_Added 测试新增记录
func TestDetectChanges_Added(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	detector := NewChangeDetector(builder)
	
	oldRecords := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
	}
	
	newRecords := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
		NewTraceRecord([]byte("record3"), nil),
	}
	
	changes, err := detector.DetectChanges(oldRecords, newRecords)
	require.NoError(t, err)
	require.Equal(t, 1, len(changes))
	require.Equal(t, ChangeTypeAdded, changes[0].Type)
	require.Equal(t, 2, changes[0].Index)
	require.Nil(t, changes[0].OldRecord)
	require.True(t, RecordsEqual(newRecords[2], changes[0].NewRecord))
}

// TestDetectChanges_Deleted 测试删除记录
func TestDetectChanges_Deleted(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	detector := NewChangeDetector(builder)
	
	oldRecords := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
		NewTraceRecord([]byte("record3"), nil),
	}
	
	newRecords := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
	}
	
	changes, err := detector.DetectChanges(oldRecords, newRecords)
	require.NoError(t, err)
	require.Equal(t, 1, len(changes))
	require.Equal(t, ChangeTypeDeleted, changes[0].Type)
	require.Equal(t, 2, changes[0].Index)
	require.True(t, RecordsEqual(oldRecords[2], changes[0].OldRecord))
	require.Nil(t, changes[0].NewRecord)
}

// TestDetectChanges_MultipleChanges 测试多个变更
func TestDetectChanges_MultipleChanges(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	detector := NewChangeDetector(builder)
	
	oldRecords := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
		NewTraceRecord([]byte("record3"), nil),
	}
	
	newRecords := []*TraceRecord{
		NewTraceRecord([]byte("modified record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
		NewTraceRecord([]byte("record3"), nil),
		NewTraceRecord([]byte("record4"), nil),
	}
	
	changes, err := detector.DetectChanges(oldRecords, newRecords)
	require.NoError(t, err)
	require.Equal(t, 2, len(changes))
	
	// 第一个变更：修改
	require.Equal(t, ChangeTypeModified, changes[0].Type)
	require.Equal(t, 0, changes[0].Index)
	
	// 第二个变更：新增
	require.Equal(t, ChangeTypeAdded, changes[1].Type)
	require.Equal(t, 3, changes[1].Index)
}

// TestDetectChanges_EmptyOldRecords 测试空旧记录
func TestDetectChanges_EmptyOldRecords(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	detector := NewChangeDetector(builder)
	
	newRecords := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
	}
	
	changes, err := detector.DetectChanges([]*TraceRecord{}, newRecords)
	require.NoError(t, err)
	require.Equal(t, 2, len(changes))
	for i, change := range changes {
		require.Equal(t, ChangeTypeAdded, change.Type)
		require.Equal(t, i, change.Index)
	}
}

// TestDetectChanges_EmptyNewRecords 测试空新记录
func TestDetectChanges_EmptyNewRecords(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	detector := NewChangeDetector(builder)
	
	oldRecords := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
	}
	
	changes, err := detector.DetectChanges(oldRecords, []*TraceRecord{})
	require.NoError(t, err)
	require.Equal(t, 2, len(changes))
	for i, change := range changes {
		require.Equal(t, ChangeTypeDeleted, change.Type)
		require.Equal(t, i, change.Index)
	}
}

// TestCalculateChangedPaths 测试计算变更路径
func TestCalculateChangedPaths(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	detector := NewChangeDetector(builder)
	
	records := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
		NewTraceRecord([]byte("record3"), nil),
		NewTraceRecord([]byte("record4"), nil),
	}
	
	tree, err := builder.BuildTree(records)
	require.NoError(t, err)
	
	changes := []*ChangeInfo{
		{
			Type:      ChangeTypeModified,
			Index:     0,
			OldRecord: records[0],
			NewRecord: NewTraceRecord([]byte("modified record1"), nil),
		},
		{
			Type:      ChangeTypeModified,
			Index:     2,
			OldRecord: records[2],
			NewRecord: NewTraceRecord([]byte("modified record3"), nil),
		},
	}
	
	paths, err := detector.CalculateChangedPaths(tree, changes)
	require.NoError(t, err)
	require.Equal(t, 2, len(paths))
	
	// 验证路径正确性
	for i, path := range paths {
		require.Equal(t, changes[i].Index, path.LeafIndex)
		require.NotNil(t, path.LeafHash)
		require.NotNil(t, path.RootHash)
		require.Equal(t, tree.Root.Hash, path.RootHash)
	}
}

// TestCalculateChangedPaths_NilTree 测试nil树
func TestCalculateChangedPaths_NilTree(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	detector := NewChangeDetector(builder)
	
	changes := []*ChangeInfo{
		{
			Type:      ChangeTypeModified,
			Index:     0,
			OldRecord: nil,
			NewRecord: nil,
		},
	}
	
	paths, err := detector.CalculateChangedPaths(nil, changes)
	require.Error(t, err)
	require.Nil(t, paths)
	require.Contains(t, err.Error(), "树不能为空")
}

// TestCalculateChangedPaths_InvalidIndex 测试无效索引
func TestCalculateChangedPaths_InvalidIndex(t *testing.T) {
	builder := NewMerkleTreeBuilder(nil)
	detector := NewChangeDetector(builder)
	
	records := []*TraceRecord{
		NewTraceRecord([]byte("record1"), nil),
		NewTraceRecord([]byte("record2"), nil),
	}
	
	tree, err := builder.BuildTree(records)
	require.NoError(t, err)
	
	changes := []*ChangeInfo{
		{
			Type:      ChangeTypeModified,
			Index:     100, // 无效索引
			OldRecord: nil,
			NewRecord: nil,
		},
	}
	
	paths, err := detector.CalculateChangedPaths(tree, changes)
	require.Error(t, err)
	require.Nil(t, paths)
	require.Contains(t, err.Error(), "变更索引超出范围")
}

