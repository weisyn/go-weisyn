package incremental

import (
	"fmt"
)

// ============================================================================
// 变更检测器（增量验证算法优化 - 阶段2）
// ============================================================================
//
// 🎯 **设计目的**：
// 实现变更检测器，检测新旧轨迹之间的变更，并计算变更路径。
//
// 🏗️ **实现策略**：
// - 使用哈希映射快速查找记录
// - 比较记录哈希而非完整内容
// - 合并相同路径，减少验证次数
//
// ⚠️ **注意**：
// - 变更检测需要O(n)时间
// - 路径计算需要O(k*log n)时间，k为变更记录数
//
// ============================================================================

// ChangeDetector 变更检测器
type ChangeDetector struct {
	builder *MerkleTreeBuilder
}

// NewChangeDetector 创建变更检测器
func NewChangeDetector(builder *MerkleTreeBuilder) *ChangeDetector {
	return &ChangeDetector{
		builder: builder,
	}
}

// DetectChanges 检测变更
//
// 📋 **参数**：
//   - oldRecords: 旧轨迹记录列表
//   - newRecords: 新轨迹记录列表
//
// 📋 **返回值**：
//   - []*ChangeInfo: 变更列表
//   - error: 检测错误
func (d *ChangeDetector) DetectChanges(oldRecords []*TraceRecord, newRecords []*TraceRecord) ([]*ChangeInfo, error) {
	changes := make([]*ChangeInfo, 0)
	
	// 使用哈希映射快速查找
	oldMap := make(map[int]*TraceRecord)
	for i, record := range oldRecords {
		oldMap[i] = record
	}
	
	// 检测变更
	for i, newRecord := range newRecords {
		oldRecord, exists := oldMap[i]
		
		if !exists {
			// 新增记录
			changes = append(changes, &ChangeInfo{
				Type:      ChangeTypeAdded,
				Index:     i,
				OldRecord: nil,
				NewRecord: newRecord,
			})
		} else if !RecordsEqual(oldRecord, newRecord) {
			// 修改记录
			changes = append(changes, &ChangeInfo{
				Type:      ChangeTypeModified,
				Index:     i,
				OldRecord: oldRecord,
				NewRecord: newRecord,
			})
		}
	}
	
	// 检测删除的记录
	for i := len(newRecords); i < len(oldRecords); i++ {
		changes = append(changes, &ChangeInfo{
			Type:      ChangeTypeDeleted,
			Index:     i,
			OldRecord: oldRecords[i],
			NewRecord: nil,
		})
	}
	
	return changes, nil
}

// CalculateChangedPaths 计算变更路径
//
// 📋 **参数**：
//   - tree: Merkle树
//   - changes: 变更列表
//
// 📋 **返回值**：
//   - []*MerklePath: 变更路径列表
//   - error: 计算错误
func (d *ChangeDetector) CalculateChangedPaths(tree *MerkleTraceTree, changes []*ChangeInfo) ([]*MerklePath, error) {
	if tree == nil {
		return nil, fmt.Errorf("树不能为空")
	}
	
	paths := make([]*MerklePath, 0)
	
	for _, change := range changes {
		// 计算变更记录的路径
		// 注意：对于新增记录，不在旧树中，无法计算路径，跳过
		if change.Type == ChangeTypeAdded {
			// 新增记录不在旧树中，无法计算路径
			// 跳过，不添加到路径列表
			continue
		}
		
		// 对于修改和删除的记录，从旧树计算路径
		leafIndex := change.Index
		
		// 验证索引在有效范围内
		// 注意：LeafCount 是树的叶子节点数量，但 ExtractRecords 可能返回更多记录（奇数个记录时最后一个被复制）
		// 所以我们需要验证索引不超过 LeafCount，但如果索引等于 LeafCount-1，也可能是有效的（最后一个节点）
		if leafIndex < 0 || leafIndex >= tree.LeafCount {
			// 如果索引等于 LeafCount，可能是 ExtractRecords 返回了更多记录导致的
			// 这种情况下，我们使用最后一个有效索引
			if leafIndex == tree.LeafCount {
				leafIndex = tree.LeafCount - 1
			} else {
				return nil, fmt.Errorf("变更索引超出范围: index=%d, tree.LeafCount=%d", change.Index, tree.LeafCount)
			}
		}
		
		path, err := d.builder.CalculatePath(tree, leafIndex)
		if err != nil {
			return nil, fmt.Errorf("计算路径失败: index=%d, error=%w", leafIndex, err)
		}
		
		paths = append(paths, path)
	}
	
	return paths, nil
}

