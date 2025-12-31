package merkle

import (
	"context"
	"crypto/subtle"
	"errors"
	"sync"
	"time"
)

// 全局互斥锁，用于保护并行处理中的证明路径
var proofMu sync.Mutex

// ProofOptions 证明生成和验证选项
type ProofOptions struct {
	// 使用并行处理
	Parallel bool

	// 验证时使用常量时间比较
	ConstantTimeVerify bool

	// 操作超时
	Timeout time.Duration
}

// DefaultProofOptions 默认证明选项
func DefaultProofOptions() *ProofOptions {
	return &ProofOptions{
		Parallel:           true,
		ConstantTimeVerify: true,
		Timeout:            5 * time.Second, // 默认5秒超时
	}
}

// ProofCache 默克尔证明缓存
type ProofCache struct {
	// 使用指纹(哈希列表+目标索引)作为键，证明路径和方向作为值
	cache map[string]proofCacheEntry
	mu    sync.RWMutex
}

// proofCacheEntry 证明缓存条目
type proofCacheEntry struct {
	proof      [][]byte
	directions []bool
}

// NewProofCache 创建新的证明缓存
func NewProofCache() *ProofCache {
	return &ProofCache{
		cache: make(map[string]proofCacheEntry),
	}
}

// Get 从缓存获取证明
func (c *ProofCache) Get(key string) ([][]byte, []bool, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.cache[key]
	if !ok {
		return nil, nil, false
	}

	// 返回副本而非引用
	proofCopy := make([][]byte, len(entry.proof))
	for i, p := range entry.proof {
		proofCopy[i] = make([]byte, len(p))
		copy(proofCopy[i], p)
	}

	directionsCopy := make([]bool, len(entry.directions))
	copy(directionsCopy, entry.directions)

	return proofCopy, directionsCopy, true
}

// Set 设置证明到缓存
func (c *ProofCache) Set(key string, proof [][]byte, directions []bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 存储副本而非引用
	proofCopy := make([][]byte, len(proof))
	for i, p := range proof {
		proofCopy[i] = make([]byte, len(p))
		copy(proofCopy[i], p)
	}

	directionsCopy := make([]bool, len(directions))
	copy(directionsCopy, directions)

	c.cache[key] = proofCacheEntry{
		proof:      proofCopy,
		directions: directionsCopy,
	}
}

// 全局证明缓存
var globalProofCache = NewProofCache()

// proofCacheKey 生成证明缓存的键
func proofCacheKey(hashes [][]byte, targetIdx int) string {
	// 使用哈希列表的指纹和目标索引作为键
	fingerprint := hashesFingerprint(hashes)

	// 将目标索引添加到指纹末尾
	result := make([]byte, len(fingerprint)+4)
	copy(result, fingerprint)

	// 将目标索引转换为字节
	result[len(fingerprint)] = byte(targetIdx >> 24)
	result[len(fingerprint)+1] = byte(targetIdx >> 16)
	result[len(fingerprint)+2] = byte(targetIdx >> 8)
	result[len(fingerprint)+3] = byte(targetIdx)

	return string(result)
}

// GenerateProof 为给定哈希生成默克尔证明
//
// 参数:
//   - hashes: 所有哈希列表
//   - targetIdx: 目标哈希的索引
//
// 返回:
//   - [][]byte: 证明路径
//   - []bool: 方向指示（true表示右侧，false表示左侧）
//   - error: 生成过程中的错误，成功时为nil
func GenerateProof(hashes [][]byte, targetIdx int) ([][]byte, []bool, error) {
	return GenerateProofWithOptions(context.Background(), hashes, targetIdx, nil)
}

// GenerateProofWithOptions 使用自定义选项生成默克尔证明
//
// 参数:
//   - ctx: 操作上下文，用于取消和超时控制
//   - hashes: 所有哈希列表
//   - targetIdx: 目标哈希的索引
//   - options: 证明选项，如果为nil则使用默认选项
//
// 返回:
//   - [][]byte: 证明路径
//   - []bool: 方向指示（true表示右侧，false表示左侧）
//   - error: 生成过程中的错误，成功时为nil
func GenerateProofWithOptions(ctx context.Context, hashes [][]byte, targetIdx int, options *ProofOptions) ([][]byte, []bool, error) {
	// 使用默认选项
	if options == nil {
		options = DefaultProofOptions()
	}

	// 创建带超时的上下文
	var cancel context.CancelFunc
	if options.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}

	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	default:
	}

	if len(hashes) == 0 {
		return nil, nil, ErrEmptyHashList
	}

	if targetIdx < 0 || targetIdx >= len(hashes) {
		return nil, nil, errors.New("目标索引超出范围")
	}

	// 检查缓存
	cacheKey := proofCacheKey(hashes, targetIdx)
	if cachedProof, cachedDirections, ok := globalProofCache.Get(cacheKey); ok {
		return cachedProof, cachedDirections, nil
	}

	// 构建叶子节点
	leaves := make([]*Node, len(hashes))
	for i, hash := range hashes {
		if len(hash) == 0 {
			return nil, nil, ErrNilHash
		}
		node := getNode()
		node.Hash = hash
		leaves[i] = node
	}

	// 收集证明
	proof := [][]byte{}
	directions := []bool{}

	// 根据选项决定使用并行或串行处理
	var root *Node
	if options.Parallel && len(hashes) > 128 {
		root = collectProofParallel(ctx, leaves, targetIdx, &proof, &directions)
	} else {
		root = collectProof(leaves, targetIdx, &proof, &directions)
	}

	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	default:
	}

	// 存入缓存
	if root != nil {
		globalProofCache.Set(cacheKey, proof, directions)
	}

	return proof, directions, nil
}

// collectProof 在构建树的过程中收集证明路径
// 参数:
//   - nodes: 当前层节点
//   - targetIdx: 目标索引
//   - proof: 证明路径（以指针形式传递，用于收集）
//   - directions: 方向指示（以指针形式传递，用于收集）
//
// 返回:
//   - *Node: 根节点
func collectProof(nodes []*Node, targetIdx int, proof *[][]byte, directions *[]bool) *Node {
	// 基本情况：只有一个节点
	if len(nodes) == 1 {
		return nodes[0]
	}

	// 如果节点数为奇数，复制最后一个节点
	var nodesList []*Node
	if len(nodes)%2 != 0 {
		nodesList = make([]*Node, len(nodes)+1)
		copy(nodesList, nodes)
		nodesList[len(nodes)] = nodes[len(nodes)-1]
	} else {
		nodesList = nodes
	}

	// 计算新的目标索引
	newTargetIdx := targetIdx / 2

	// 两两组合，构建上一层节点
	numParents := len(nodesList) / 2
	parents := make([]*Node, numParents)

	for i := 0; i < len(nodesList); i += 2 {
		left := nodesList[i]
		right := nodesList[i+1]

		// 如果当前节点包含目标节点，添加其兄弟节点到证明
		if i/2 == newTargetIdx {
			if targetIdx%2 == 0 { // 目标在左边
				*proof = append(*proof, right.Hash)
				*directions = append(*directions, true)
			} else { // 目标在右边
				*proof = append(*proof, left.Hash)
				*directions = append(*directions, false)
			}
		}

		// 检查缓存
		combinedHash := string(append(left.Hash, right.Hash...))
		if cachedNode, ok := globalTreeCache.GetNode(combinedHash); ok {
			parents[i/2] = cachedNode
			continue
		}

		// 计算父节点哈希
		parentHash := combineHashes(left.Hash, right.Hash)

		// 创建父节点
		parent := getNode()
		parent.Hash = parentHash
		parent.Left = left
		parent.Right = right

		// 存入缓存
		globalTreeCache.SetNode(combinedHash, parent)

		parents[i/2] = parent
	}

	// 递归构建树的上层
	return collectProof(parents, newTargetIdx, proof, directions)
}

// collectProofParallel 在构建树的过程中并行收集证明路径
func collectProofParallel(ctx context.Context, nodes []*Node, targetIdx int, proof *[][]byte, directions *[]bool) *Node {
	// 基本情况：只有一个节点
	if len(nodes) == 1 {
		return nodes[0]
	}

	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return nil
	default:
	}

	// 如果节点数为奇数，复制最后一个节点
	var nodesList []*Node
	if len(nodes)%2 != 0 {
		nodesList = make([]*Node, len(nodes)+1)
		copy(nodesList, nodes)
		nodesList[len(nodes)] = nodes[len(nodes)-1]
	} else {
		nodesList = nodes
	}

	// 计算新的目标索引
	newTargetIdx := targetIdx / 2

	// 并行处理大节点列表
	if len(nodesList) > 64 {
		return collectProofParallelLarge(ctx, nodesList, targetIdx, newTargetIdx, proof, directions)
	}

	// 两两组合，构建上一层节点
	numParents := len(nodesList) / 2
	parents := make([]*Node, numParents)

	// 并行处理各段
	for i := 0; i < len(nodesList); i += 2 {
		left := nodesList[i]
		right := nodesList[i+1]

		// 如果当前节点包含目标节点，添加其兄弟节点到证明
		if i/2 == newTargetIdx {
			proofMu.Lock()
			if targetIdx%2 == 0 { // 目标在左边
				*proof = append(*proof, right.Hash)
				*directions = append(*directions, true)
			} else { // 目标在右边
				*proof = append(*proof, left.Hash)
				*directions = append(*directions, false)
			}
			proofMu.Unlock()
		}

		// 检查缓存
		combinedHash := string(append(left.Hash, right.Hash...))
		if cachedNode, ok := globalTreeCache.GetNode(combinedHash); ok {
			parents[i/2] = cachedNode
			continue
		}

		// 计算父节点哈希
		parentHash := combineHashes(left.Hash, right.Hash)

		// 创建父节点
		parent := getNode()
		parent.Hash = parentHash
		parent.Left = left
		parent.Right = right

		// 存入缓存
		globalTreeCache.SetNode(combinedHash, parent)

		parents[i/2] = parent
	}

	// 递归构建树的上层
	return collectProofParallel(ctx, parents, newTargetIdx, proof, directions)
}

// collectProofParallelLarge 使用goroutine并行处理大型节点列表
func collectProofParallelLarge(ctx context.Context, nodes []*Node, targetIdx, newTargetIdx int, proof *[][]byte, directions *[]bool) *Node {
	numParents := len(nodes) / 2
	parents := make([]*Node, numParents)

	// 分段处理
	numSegments := 4 // 可调整为CPU核心数
	nodesPerSegment := (len(nodes)/2 + numSegments - 1) / numSegments

	// 创建工作组
	var wg sync.WaitGroup
	wg.Add(numSegments)

	// 并行处理各段
	for seg := 0; seg < numSegments; seg++ {
		go func(segmentID int) {
			defer wg.Done()

			// 计算此段的范围
			startIdx := segmentID * nodesPerSegment * 2
			endIdx := (segmentID + 1) * nodesPerSegment * 2
			if endIdx > len(nodes) {
				endIdx = len(nodes)
			}

			// 对此段的节点进行处理
			for i := startIdx; i < endIdx; i += 2 {
				if i+1 >= len(nodes) {
					break
				}

				left := nodes[i]
				right := nodes[i+1]

				// 如果当前节点包含目标节点，添加其兄弟节点到证明
				if i/2 == newTargetIdx {
					proofMu.Lock()
					if targetIdx%2 == 0 { // 目标在左边
						*proof = append(*proof, right.Hash)
						*directions = append(*directions, true)
					} else { // 目标在右边
						*proof = append(*proof, left.Hash)
						*directions = append(*directions, false)
					}
					proofMu.Unlock()
				}

				// 检查缓存
				combinedHash := string(append(left.Hash, right.Hash...))
				if cachedNode, ok := globalTreeCache.GetNode(combinedHash); ok {
					parents[i/2] = cachedNode
					continue
				}

				// 计算父节点哈希
				parentHash := combineHashes(left.Hash, right.Hash)

				// 创建父节点
				parent := getNode()
				parent.Hash = parentHash
				parent.Left = left
				parent.Right = right

				// 存入缓存
				globalTreeCache.SetNode(combinedHash, parent)

				parents[i/2] = parent
			}
		}(seg)
	}

	// 等待所有段处理完成
	wg.Wait()

	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return nil
	default:
	}

	// 递归构建树的上层
	return collectProofParallel(ctx, parents, newTargetIdx, proof, directions)
}

// VerifyProof 验证默克尔证明
//
// 参数:
//   - targetHash: 目标哈希
//   - proof: 证明路径
//   - directions: 方向指示
//   - root: 默克尔根
//
// 返回:
//   - bool: 验证结果，true表示有效
func VerifyProof(targetHash []byte, proof [][]byte, directions []bool, root []byte) bool {
	return VerifyProofWithOptions(context.Background(), targetHash, proof, directions, root, nil)
}

// VerifyProofWithOptions 使用自定义选项验证默克尔证明
//
// 参数:
//   - ctx: 操作上下文，用于取消和超时控制
//   - targetHash: 目标哈希
//   - proof: 证明路径
//   - directions: 方向指示
//   - root: 默克尔根
//   - options: 证明选项，如果为nil则使用默认选项
//
// 返回:
//   - bool: 验证结果，true表示有效
func VerifyProofWithOptions(ctx context.Context, targetHash []byte, proof [][]byte, directions []bool, root []byte, options *ProofOptions) bool {
	// 使用默认选项
	if options == nil {
		options = DefaultProofOptions()
	}

	// 创建带超时的上下文
	var cancel context.CancelFunc
	if options.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}

	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return false
	default:
	}

	if len(proof) != len(directions) {
		return false
	}

	currentHash := targetHash

	// 沿着证明路径计算哈希
	for i, siblingHash := range proof {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			return false
		default:
		}

		if directions[i] { // 目标在左边，兄弟在右边
			currentHash = combineHashes(currentHash, siblingHash)
		} else { // 目标在右边，兄弟在左边
			currentHash = combineHashes(siblingHash, currentHash)
		}
	}

	// 检查计算的根哈希是否与提供的根哈希匹配
	if options.ConstantTimeVerify {
		return constantTimeCompareHashes(currentHash, root)
	} else {
		return compareHashes(currentHash, root)
	}
}

// compareHashes 比较两个哈希是否相等
//
// 参数:
//   - a: 第一个哈希
//   - b: 第二个哈希
//
// 返回:
//   - bool: 比较结果，true表示相等
func compareHashes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// constantTimeCompareHashes 在常量时间内比较两个哈希是否相等
//
// 参数:
//   - a: 第一个哈希
//   - b: 第二个哈希
//
// 返回:
//   - bool: 比较结果，true表示相等
func constantTimeCompareHashes(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}
