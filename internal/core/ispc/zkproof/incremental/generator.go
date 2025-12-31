package incremental

import (
	"fmt"
	"time"
)

// ============================================================================
// 增量证明生成器（增量验证算法优化 - 阶段2）
// ============================================================================
//
// 🎯 **设计目的**：
// 实现增量证明生成器，生成增量验证证明。
//
// 🏗️ **实现策略**：
// - 检测变更
// - 计算变更路径
// - 构建新Merkle树
// - 生成增量证明
//
// ⚠️ **注意**：
// - 证明生成需要O(n + k*log n)时间
// - n为轨迹记录数，k为变更记录数
//
// ============================================================================

// IncrementalProofGenerator 增量证明生成器
type IncrementalProofGenerator struct {
	builder  *MerkleTreeBuilder
	detector *ChangeDetector
}

// NewIncrementalProofGenerator 创建增量证明生成器
func NewIncrementalProofGenerator(builder *MerkleTreeBuilder, detector *ChangeDetector) *IncrementalProofGenerator {
	return &IncrementalProofGenerator{
		builder:  builder,
		detector: detector,
	}
}

// GenerateProof 生成增量验证证明
//
// 📋 **参数**：
//   - oldTree: 旧Merkle树
//   - newRecords: 新轨迹记录列表
//   - changes: 变更列表（如果为nil，自动检测）
//
// 📋 **返回值**：
//   - *IncrementalVerificationProof: 增量验证证明
//   - error: 生成错误
func (g *IncrementalProofGenerator) GenerateProof(
	oldTree *MerkleTraceTree,
	newRecords []*TraceRecord,
	changes []*ChangeInfo,
) (*IncrementalVerificationProof, error) {
	if oldTree == nil {
		return nil, fmt.Errorf("旧树不能为空")
	}
	
	// 1. 如果没有提供变更列表，自动检测
	if changes == nil {
		oldRecords := g.builder.ExtractRecords(oldTree)
		var err error
		changes, err = g.detector.DetectChanges(oldRecords, newRecords)
		if err != nil {
			return nil, fmt.Errorf("检测变更失败: %w", err)
		}
	}
	
	// 2. 计算变更路径
	changedPaths, err := g.detector.CalculateChangedPaths(oldTree, changes)
	if err != nil {
		return nil, fmt.Errorf("计算变更路径失败: %w", err)
	}
	
	// 3. 构建新Merkle树
	newTree, err := g.builder.RebuildTree(oldTree, changes)
	if err != nil {
		return nil, fmt.Errorf("重建树失败: %w", err)
	}
	
	// 4. 提取变更记录
	changedRecords := make([]*TraceRecord, 0, len(changes))
	for _, change := range changes {
		if change.NewRecord != nil {
			changedRecords = append(changedRecords, change.NewRecord)
		}
	}
	
	// 5. 构建增量证明
	proof := &IncrementalVerificationProof{
		OldRootHash:    oldTree.Root.Hash,
		ChangedPaths:   changedPaths,
		ChangedRecords: changedRecords,
		NewRootHash:    newTree.Root.Hash,
		CreatedAt:      time.Now(),
	}
	
	return proof, nil
}

