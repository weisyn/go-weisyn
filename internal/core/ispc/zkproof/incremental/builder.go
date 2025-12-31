package incremental

import (
	"fmt"
	"time"
)

// ============================================================================
// Merkle Tree构建器（增量验证算法优化 - 阶段2）
// ============================================================================
//
// 🎯 **设计目的**：
// 实现Merkle Tree构建器，支持构建Merkle树、计算路径、验证路径等功能。
//
// 🏗️ **实现策略**：
// - 自底向上构建Merkle树（O(n)时间）
// - 使用递归或迭代方式构建
// - 优化内存使用，避免不必要的节点创建
//
// ⚠️ **注意**：
// - 树构建需要O(n)时间，但只需要构建一次
// - 路径计算需要O(log n)时间
// - 路径验证需要O(log n)时间
//
// ============================================================================

// MerkleTreeBuilder Merkle树构建器
type MerkleTreeBuilder struct {
	hashFunc HashFunction
}

// NewMerkleTreeBuilder 创建Merkle树构建器
func NewMerkleTreeBuilder(hashFunc HashFunction) *MerkleTreeBuilder {
	if hashFunc == nil {
		hashFunc = DefaultHashFunction()
	}
	return &MerkleTreeBuilder{
		hashFunc: hashFunc,
	}
}

// BuildTree 构建Merkle树
//
// 📋 **参数**：
//   - records: 轨迹记录列表
//
// 📋 **返回值**：
//   - *MerkleTraceTree: Merkle树
//   - error: 构建错误
func (b *MerkleTreeBuilder) BuildTree(records []*TraceRecord) (*MerkleTraceTree, error) {
	if len(records) == 0 {
		return nil, fmt.Errorf("记录列表不能为空")
	}
	
	// 1. 创建叶子节点
	leaves := make([]*MerkleTraceNode, len(records))
	for i, record := range records {
		// 计算记录哈希（如果未计算）
		if len(record.Hash) == 0 {
			record.Hash = b.hashFunc(SerializeRecord(record))
		}
		
		// 验证序列化数据不为空
		if len(record.SerializedData) == 0 {
			return nil, fmt.Errorf("记录[%d]的序列化数据为空", i)
		}
		
		leaves[i] = &MerkleTraceNode{
			Hash:   record.Hash,
			IsLeaf: true,
			Data:   record,
			Index:  i,
			Depth:  0,
		}
	}
	
	// 2. 自底向上构建树
	currentLevel := leaves
	depth := 0
	
	for len(currentLevel) > 1 {
		nextLevel := make([]*MerkleTraceNode, 0)
		
		// 两两合并节点
		for i := 0; i < len(currentLevel); i += 2 {
			left := currentLevel[i]
			right := left
			
			if i+1 < len(currentLevel) {
				right = currentLevel[i+1]
			}
			
			// 创建父节点
			parentHash := b.hashFunc(append(left.Hash, right.Hash...))
			parent := &MerkleTraceNode{
				Hash:   parentHash,
				Left:   left,
				Right:  right,
				IsLeaf: false,
				Index:  i / 2,
				Depth:  depth + 1,
			}
			
			nextLevel = append(nextLevel, parent)
		}
		
		currentLevel = nextLevel
		depth++
	}
	
	// 3. 返回树结构
	return &MerkleTraceTree{
		Root:      currentLevel[0],
		LeafCount: len(records),
		Depth:     depth,
		HashFunc:  b.hashFunc,
		CreatedAt: time.Now(),
	}, nil
}

// CalculatePath 计算Merkle路径
//
// 📋 **参数**：
//   - tree: Merkle树
//   - leafIndex: 叶子节点索引
//
// 📋 **返回值**：
//   - *MerklePath: Merkle路径
//   - error: 计算错误
func (b *MerkleTreeBuilder) CalculatePath(tree *MerkleTraceTree, leafIndex int) (*MerklePath, error) {
	if tree == nil || tree.Root == nil {
		return nil, fmt.Errorf("树不能为空")
	}
	
	if leafIndex < 0 || leafIndex >= tree.LeafCount {
		return nil, fmt.Errorf("叶子节点索引超出范围: %d", leafIndex)
	}
	
	// 找到叶子节点
	leafNode := b.findLeafNode(tree.Root, leafIndex, 0, tree.LeafCount-1)
	if leafNode == nil {
		return nil, fmt.Errorf("未找到叶子节点: %d", leafIndex)
	}
	
	// 构建路径
	path := &MerklePath{
		LeafIndex:      leafIndex,
		LeafHash:       leafNode.Hash,
		SiblingHashes:  make([][]byte, 0),
		PathDirections: make([]int, 0),
		RootHash:       tree.Root.Hash,
	}
	
	// 从叶子节点向上遍历到根节点
	currentNode := leafNode
	currentIndex := leafIndex
	
	for currentNode != tree.Root {
		parent := b.findParent(tree.Root, currentNode)
		if parent == nil {
			break
		}
		
		// 确定方向
		if parent.Left == currentNode {
			// 当前节点是左子节点，兄弟节点是右子节点
			path.PathDirections = append(path.PathDirections, 0)
			if parent.Right != nil {
				path.SiblingHashes = append(path.SiblingHashes, parent.Right.Hash)
			} else {
				// 如果没有右子节点，使用当前节点哈希（填充）
				path.SiblingHashes = append(path.SiblingHashes, currentNode.Hash)
			}
		} else {
			// 当前节点是右子节点，兄弟节点是左子节点
			path.PathDirections = append(path.PathDirections, 1)
			if parent.Left != nil {
				path.SiblingHashes = append(path.SiblingHashes, parent.Left.Hash)
			} else {
				// 如果没有左子节点，使用当前节点哈希（填充）
				path.SiblingHashes = append(path.SiblingHashes, currentNode.Hash)
			}
		}
		
		currentNode = parent
		currentIndex = currentIndex / 2
	}
	
	return path, nil
}

// VerifyPath 验证Merkle路径
//
// 📋 **参数**：
//   - path: Merkle路径
//
// 📋 **返回值**：
//   - bool: 验证结果
func (b *MerkleTreeBuilder) VerifyPath(path *MerklePath) bool {
	if path == nil {
		return false
	}
	
	if len(path.SiblingHashes) != len(path.PathDirections) {
		return false
	}
	
	currentHash := path.LeafHash
	
	// 从叶子节点向上验证到根节点
	for i := 0; i < len(path.SiblingHashes); i++ {
		siblingHash := path.SiblingHashes[i]
		direction := path.PathDirections[i]
		
		// 根据方向组合哈希
		if direction == 0 {
			// 左子节点，组合为 [currentHash, siblingHash]
			currentHash = b.hashFunc(append(currentHash, siblingHash...))
		} else {
			// 右子节点，组合为 [siblingHash, currentHash]
			currentHash = b.hashFunc(append(siblingHash, currentHash...))
		}
	}
	
	// 验证最终哈希是否等于根哈希
	return bytesEqual(currentHash, path.RootHash)
}

// findLeafNode 查找叶子节点（递归）
func (b *MerkleTreeBuilder) findLeafNode(node *MerkleTraceNode, targetIndex int, startIndex int, endIndex int) *MerkleTraceNode {
	if node == nil {
		return nil
	}
	
	if node.IsLeaf {
		if node.Index == targetIndex {
			return node
		}
		return nil
	}
	
	// 计算中间索引
	midIndex := (startIndex + endIndex) / 2
	
	if targetIndex <= midIndex {
		// 在左子树
		return b.findLeafNode(node.Left, targetIndex, startIndex, midIndex)
	} else {
		// 在右子树
		return b.findLeafNode(node.Right, targetIndex, midIndex+1, endIndex)
	}
}

// findParent 查找父节点（递归）
func (b *MerkleTreeBuilder) findParent(root *MerkleTraceNode, target *MerkleTraceNode) *MerkleTraceNode {
	if root == nil || target == nil {
		return nil
	}
	
	if root == target {
		return nil // 根节点没有父节点
	}
	
	if root.Left == target || root.Right == target {
		return root
	}
	
	// 递归查找
	if parent := b.findParent(root.Left, target); parent != nil {
		return parent
	}
	
	return b.findParent(root.Right, target)
}

// RebuildTree 重建Merkle树（增量更新）
//
// 📋 **参数**：
//   - oldTree: 旧Merkle树
//   - changes: 变更列表
//
// 📋 **返回值**：
//   - *MerkleTraceTree: 新Merkle树
//   - error: 重建错误
func (b *MerkleTreeBuilder) RebuildTree(oldTree *MerkleTraceTree, changes []*ChangeInfo) (*MerkleTraceTree, error) {
	if oldTree == nil {
		return nil, fmt.Errorf("旧树不能为空")
	}
	
	// 📋 **当前实现**：重新构建整个树
	// - 从旧树提取所有记录
	// - 应用变更（新增/修改/删除）
	// - 重新构建完整的Merkle树
	// - 时间复杂度：O(n)，其中n是记录总数
	//
	// 🔮 **未来优化方向**：增量更新优化
	// - 只更新变更路径，而不是重新构建整个树
	// - 时间复杂度：O(log n)，其中n是记录总数
	// - 优化策略：
	//   1. 识别变更影响的路径（从叶子节点到根节点）
	//   2. 只重新计算变更路径上的节点哈希
	//   3. 保持其他未变更路径不变
	// - 实现复杂度较高，需要维护路径信息和节点引用
	// - 当前实现已满足功能需求，性能优化可在后续版本实现
	
	// 从旧树提取所有记录
	oldRecords := b.extractRecords(oldTree)
	
	// 应用变更
	newRecords := make([]*TraceRecord, len(oldRecords))
	copy(newRecords, oldRecords)
	
	for _, change := range changes {
		switch change.Type {
		case ChangeTypeAdded:
			// 新增：在末尾添加
			newRecords = append(newRecords, change.NewRecord)
		case ChangeTypeModified:
			// 修改：替换记录
			if change.Index < len(newRecords) {
				newRecords[change.Index] = change.NewRecord
			}
		case ChangeTypeDeleted:
			// 删除：移除记录
			if change.Index < len(newRecords) {
				newRecords = append(newRecords[:change.Index], newRecords[change.Index+1:]...)
			}
		}
	}
	
	// 重新构建树
	return b.BuildTree(newRecords)
}

// ExtractRecords 从树中提取所有记录（公开方法）
func (b *MerkleTreeBuilder) ExtractRecords(tree *MerkleTraceTree) []*TraceRecord {
	records := make([]*TraceRecord, 0)
	b.extractRecordsRecursive(tree.Root, &records)
	return records
}

// extractRecords 从树中提取所有记录（内部方法）
func (b *MerkleTreeBuilder) extractRecords(tree *MerkleTraceTree) []*TraceRecord {
	return b.ExtractRecords(tree)
}

// extractRecordsRecursive 递归提取记录
func (b *MerkleTreeBuilder) extractRecordsRecursive(node *MerkleTraceNode, records *[]*TraceRecord) {
	if node == nil {
		return
	}
	
	if node.IsLeaf {
		*records = append(*records, node.Data)
		return
	}
	
	b.extractRecordsRecursive(node.Left, records)
	b.extractRecordsRecursive(node.Right, records)
}

