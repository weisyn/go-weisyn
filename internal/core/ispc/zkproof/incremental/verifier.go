package incremental

import (
	"fmt"
)

// ============================================================================
// 增量验证器（增量验证算法优化 - 阶段2）
// ============================================================================
//
// 🎯 **设计目的**：
// 实现增量验证器，验证增量验证证明的正确性。
//
// 🏗️ **实现策略**：
// - 验证旧根哈希
// - 验证每个变更路径
// - 重新计算新根哈希
// - 验证新根哈希一致性
//
// ⚠️ **注意**：
// - 验证需要O(k*log n)时间，k为变更记录数
// - 需要确保验证的正确性和安全性
//
// ============================================================================

// IncrementalVerifier 增量验证器
type IncrementalVerifier struct {
	builder *MerkleTreeBuilder
}

// NewIncrementalVerifier 创建增量验证器
func NewIncrementalVerifier(builder *MerkleTreeBuilder) *IncrementalVerifier {
	return &IncrementalVerifier{
		builder: builder,
	}
}

// VerifyProof 验证增量证明
//
// 📋 **参数**：
//   - proof: 增量验证证明
//   - oldRootHash: 旧根哈希（用于验证）
//
// 📋 **返回值**：
//   - bool: 验证结果
//   - error: 验证错误
func (v *IncrementalVerifier) VerifyProof(proof *IncrementalVerificationProof, oldRootHash []byte) (bool, error) {
	if proof == nil {
		return false, fmt.Errorf("证明不能为空")
	}

	// 1. 验证旧根哈希
	if oldRootHash != nil {
		if !bytesEqual(proof.OldRootHash, oldRootHash) {
			return false, fmt.Errorf("旧根哈希不匹配")
		}
	}

	// 2. 验证变更路径（路径来自旧树，应该能验证旧树的状态）
	// 注意：ChangedPaths 只包含修改和删除的路径（新增记录不在旧树中，无路径）
	// ChangedRecords 只包含新增和修改的记录（删除记录无 NewRecord，不在列表中）
	// 所以路径数量和记录数量可能不一致，这是正常的
	for i, path := range proof.ChangedPaths {
		if !v.builder.VerifyPath(path) {
			return false, fmt.Errorf("变更路径验证失败: index=%d", i)
		}

		// 路径验证已经确保路径能验证旧树的状态
		// 路径中的根哈希应该等于 proof.OldRootHash
		if !bytesEqual(path.RootHash, proof.OldRootHash) {
			return false, fmt.Errorf("变更路径的根哈希与旧根哈希不匹配: index=%d", i)
		}
	}

	// 4. 重新计算新根哈希
	newRootHash, err := v.recalculateRootHash(proof)
	if err != nil {
		return false, fmt.Errorf("重新计算新根哈希失败: %w", err)
	}

	// 5. 验证新根哈希
	if !bytesEqual(newRootHash, proof.NewRootHash) {
		return false, fmt.Errorf("新根哈希不匹配")
	}

	return true, nil
}

// VerifyPath 验证Merkle路径
//
// 📋 **参数**：
//   - path: Merkle路径
//
// 📋 **返回值**：
//   - bool: 验证结果
func (v *IncrementalVerifier) VerifyPath(path *MerklePath) bool {
	return v.builder.VerifyPath(path)
}

// recalculateRootHash 重新计算新根哈希
//
// 🎯 **算法**：
// 根据变更路径和变更记录，重新计算新根哈希
//
// 📋 **实现策略**：
// 1. 如果没有变更，新根哈希等于旧根哈希
// 2. 如果有单个变更，使用变更路径重新计算根哈希
// 3. 如果有多个变更，需要合并所有变更路径重新计算根哈希
//
// ⚠️ **注意**：
// - 对于多个变更，需要确保变更路径的根哈希一致
// - 如果变更路径的根哈希不一致，说明变更不在同一棵树中，这是错误情况
func (v *IncrementalVerifier) recalculateRootHash(proof *IncrementalVerificationProof) ([]byte, error) {
	if len(proof.ChangedPaths) == 0 && len(proof.ChangedRecords) == 0 {
		// 没有变更，新根哈希等于旧根哈希
		return proof.OldRootHash, nil
	}

	// 如果没有路径但有记录（只有新增），无法使用路径重新计算根哈希
	if len(proof.ChangedPaths) == 0 && len(proof.ChangedRecords) > 0 {
		return nil, fmt.Errorf("无法重新计算根哈希: 只有新增记录，无变更路径")
	}

	// 如果没有记录但有路径（只有删除），需要特殊处理
	if len(proof.ChangedPaths) > 0 && len(proof.ChangedRecords) == 0 {
		return nil, fmt.Errorf("无法重新计算根哈希: 只有删除记录，需要完整实现")
	}

	// 1. 验证所有变更路径的根哈希是否一致（必须来自同一棵树）
	firstRootHash := proof.ChangedPaths[0].RootHash
	for i := 1; i < len(proof.ChangedPaths); i++ {
		if !bytesEqual(proof.ChangedPaths[i].RootHash, firstRootHash) {
			return nil, fmt.Errorf("变更路径的根哈希不一致: 路径[0]根哈希=%x, 路径[%d]根哈希=%x",
				firstRootHash[:min(8, len(firstRootHash))], i, proof.ChangedPaths[i].RootHash[:min(8, len(proof.ChangedPaths[i].RootHash))])
		}
	}

	// 2. 如果有单个变更，直接使用变更路径重新计算根哈希
	if len(proof.ChangedPaths) == 1 {
		if len(proof.ChangedRecords) == 0 {
			return nil, fmt.Errorf("无法重新计算根哈希: 有路径但无记录")
		}
		return v.recalculateRootHashFromPath(proof.ChangedPaths[0], proof.ChangedRecords[0])
	}

	// 3. 多变更路径：实现多点更新的根哈希重算（Merkle multiproof merge）
	//
	// 约束（当前实现聚焦“修改”场景）：
	// - 需要 ChangedPaths 与 ChangedRecords 一一对应（同序、同数量）
	// - 新增/删除会导致路径与记录数量不一致，属于后续扩展点（需要附带新增叶子/删除叶子的结构证明）
	if len(proof.ChangedPaths) != len(proof.ChangedRecords) {
		return nil, fmt.Errorf("暂不支持多路径与记录数量不一致的重算（changed_paths=%d changed_records=%d）：仅支持纯修改场景",
			len(proof.ChangedPaths), len(proof.ChangedRecords))
	}

	return v.recalculateRootHashFromMultiplePaths(proof.ChangedPaths, proof.ChangedRecords)
}

// recalculateRootHashFromMultiplePaths 合并多条变更路径并重算新根哈希（多点更新）
//
// 实现思路（确定性 + 冲突检测）：
// - 使用每条路径提供的“兄弟节点哈希（旧树快照）”补齐未变更分支
// - 对每个被修改的叶子写入新叶子哈希
// - 自底向上逐层计算父节点哈希；若同一节点被多条路径推导出不同哈希，直接报冲突
func (v *IncrementalVerifier) recalculateRootHashFromMultiplePaths(paths []*MerklePath, records []*TraceRecord) ([]byte, error) {
	if len(paths) == 0 || len(records) == 0 {
		return nil, fmt.Errorf("多路径重算需要 paths 与 records 非空")
	}
	if len(paths) != len(records) {
		return nil, fmt.Errorf("paths 与 records 数量必须一致: paths=%d records=%d", len(paths), len(records))
	}

	// 统一深度（兄弟哈希数量应一致）
	depth := len(paths[0].SiblingHashes)
	if depth == 0 {
		return nil, fmt.Errorf("路径深度为0，无法重算根哈希")
	}

	// levelHashes[level][index] = hash
	levelHashes := make([]map[int][]byte, depth+1)
	for i := 0; i <= depth; i++ {
		levelHashes[i] = make(map[int][]byte)
	}
	// 标记每个节点哈希的来源：
	// - snapshot：来自旧树快照（路径 sibling 提供）
	// - derived：由“新叶子 + 已知兄弟节点”推导得到（优先级更高，可覆盖 snapshot）
	type nodeSource uint8
	const (
		sourceSnapshot nodeSource = iota
		sourceDerived
	)
	levelSources := make([]map[int]nodeSource, depth+1)
	for i := 0; i <= depth; i++ {
		levelSources[i] = make(map[int]nodeSource)
	}

	setNode := func(level int, index int, hash []byte, src nodeSource) error {
		if existing, ok := levelHashes[level][index]; ok {
			existingSrc := levelSources[level][index]
			if bytesEqual(existing, hash) {
				// 一致则升级为 derived（更强语义）
				if src == sourceDerived {
					levelSources[level][index] = sourceDerived
				}
				return nil
			}

			// snapshot 与 derived 冲突：允许 derived 覆盖 snapshot（典型场景：某条路径的 sibling 子树里包含了另一条路径的变更叶子）
			if existingSrc == sourceSnapshot && src == sourceDerived {
				levelHashes[level][index] = hash
				levelSources[level][index] = sourceDerived
				return nil
			}
			// derived 已存在：不允许被 snapshot 覆盖；也不允许两个 derived 互相冲突
			if existingSrc == sourceDerived {
				return fmt.Errorf("节点哈希冲突: level=%d index=%d", level, index)
			}
			// snapshot 已存在、又来了不同 snapshot：说明 proof 自相矛盾
			return fmt.Errorf("节点哈希冲突: level=%d index=%d", level, index)
		}

		levelHashes[level][index] = hash
		levelSources[level][index] = src
		return nil
	}

	// 1) 写入变更叶子 + 填充每条路径的兄弟节点哈希（来自旧树快照）
	for i, path := range paths {
		if path == nil {
			return nil, fmt.Errorf("路径不能为空: index=%d", i)
		}
		if records[i] == nil {
			return nil, fmt.Errorf("变更记录不能为空: index=%d", i)
		}
		if len(path.SiblingHashes) != depth || len(path.PathDirections) != depth {
			return nil, fmt.Errorf("路径深度不一致: index=%d sibling=%d dir=%d depth=%d",
				i, len(path.SiblingHashes), len(path.PathDirections), depth)
		}

		leafIndex := path.LeafIndex
		newLeafHash := v.builder.hashFunc(SerializeRecord(records[i]))
		// derived 叶子可以覆盖 snapshot 叶子（例如：另一条路径把它当作 sibling 叶子带了旧值）
		if err := setNode(0, leafIndex, newLeafHash, sourceDerived); err != nil {
			return nil, fmt.Errorf("同一叶子被多次更新且哈希冲突: leaf_index=%d", leafIndex)
		}

		// 将每一层的 sibling hash 放入对应层级索引（nodeIndexAtLevel ^ 1）
		for l := 0; l < depth; l++ {
			nodeIndex := leafIndex >> l
			siblingIndex := nodeIndex ^ 1
			siblingHash := path.SiblingHashes[l]
			// snapshot 兄弟节点：
			// - 若节点未来会被推导（因为覆盖到其他变更路径子树），derived 会覆盖它；
			// - 若已经是 derived，则不允许 snapshot 覆盖（但也不应报错）。
			if existing, ok := levelHashes[l][siblingIndex]; ok {
				if levelSources[l][siblingIndex] == sourceDerived {
					continue
				}
				if !bytesEqual(existing, siblingHash) {
					return nil, fmt.Errorf("兄弟节点哈希冲突: level=%d index=%d", l, siblingIndex)
				}
				continue
			}
			if err := setNode(l, siblingIndex, siblingHash, sourceSnapshot); err != nil {
				return nil, fmt.Errorf("兄弟节点哈希冲突: level=%d index=%d", l, siblingIndex)
			}
		}
	}

	// 2) 自底向上计算父节点（逐层）
	for l := 0; l < depth; l++ {
		parentsToTry := make(map[int]struct{})
		for childIdx := range levelHashes[l] {
			parentsToTry[childIdx/2] = struct{}{}
		}
		for parentIdx := range parentsToTry {
			leftIdx := parentIdx * 2
			rightIdx := leftIdx + 1
			leftHash, okL := levelHashes[l][leftIdx]
			rightHash, okR := levelHashes[l][rightIdx]
			if !okL || !okR {
				// 信息不足，理论上意味着 paths 集合不完整（或树结构不匹配）
				continue
			}
			parentHash := v.builder.hashFunc(append(leftHash, rightHash...))
			if err := setNode(l+1, parentIdx, parentHash, sourceDerived); err != nil {
				return nil, fmt.Errorf("父节点哈希冲突: level=%d index=%d", l+1, parentIdx)
			}
		}
	}

	rootHash, ok := levelHashes[depth][0]
	if !ok || len(rootHash) == 0 {
		return nil, fmt.Errorf("无法从提供的多路径信息重算根哈希: depth=%d", depth)
	}
	return rootHash, nil
}

// recalculateRootHashFromPath 从单个变更路径重新计算根哈希
//
// 🎯 **算法**：
// 1. 从变更记录的哈希开始（叶子节点）
// 2. 使用路径中的兄弟节点哈希，按照路径方向向上计算
// 3. 最终得到根哈希
func (v *IncrementalVerifier) recalculateRootHashFromPath(path *MerklePath, record *TraceRecord) ([]byte, error) {
	if path == nil {
		return nil, fmt.Errorf("变更路径不能为空")
	}

	if record == nil {
		return nil, fmt.Errorf("变更记录不能为空")
	}

	// 1. 计算变更记录的哈希（叶子节点哈希）
	// 注意：这是新记录的哈希，路径中的 LeafHash 是旧记录的哈希
	recordHash := v.builder.hashFunc(SerializeRecord(record))

	// 2. 从新记录的哈希开始，使用路径中的兄弟节点哈希向上计算
	// 路径中的兄弟节点哈希来自旧树，但用于重新计算新根哈希
	currentHash := recordHash

	// 验证路径长度一致性
	if len(path.SiblingHashes) != len(path.PathDirections) {
		return nil, fmt.Errorf("路径长度不一致: 兄弟节点数=%d, 方向数=%d",
			len(path.SiblingHashes), len(path.PathDirections))
	}

	// 4. 按照路径方向向上计算哈希
	for i := 0; i < len(path.SiblingHashes); i++ {
		siblingHash := path.SiblingHashes[i]
		direction := path.PathDirections[i]

		// 根据方向组合哈希
		if direction == 0 {
			// 左子节点，组合为 [currentHash, siblingHash]
			currentHash = v.builder.hashFunc(append(currentHash, siblingHash...))
		} else {
			// 右子节点，组合为 [siblingHash, currentHash]
			currentHash = v.builder.hashFunc(append(siblingHash, currentHash...))
		}
	}

	// 5. 返回计算出的新根哈希
	// 注意：不需要验证与路径中的根哈希匹配，因为路径是旧树的
	// 计算出的根哈希应该等于 proof.NewRootHash
	return currentHash, nil
}

// min 返回两个整数中的较小值（辅助函数）
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
