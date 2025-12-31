// Package merkle provides Merkle tree implementation with caching and parallel processing.
package merkle

import (
	"crypto/sha256"
	"errors"
	"sync"
)

// 错误定义
var (
	ErrEmptyHashList = errors.New("空哈希列表")
	ErrNilHash       = errors.New("存在空哈希")
)

// Node 默克尔树节点
type Node struct {
	Hash  []byte
	Left  *Node
	Right *Node
}

// TreeCache 默克尔树缓存
type TreeCache struct {
	// 使用根哈希作为键，节点作为值
	nodeCache map[string]*Node
	// 叶子哈希列表的哈希作为键，根哈希作为值
	rootCache map[string][]byte
	mu        sync.RWMutex
}

// NewTreeCache 创建新的默克尔树缓存
func NewTreeCache() *TreeCache {
	return &TreeCache{
		nodeCache: make(map[string]*Node),
		rootCache: make(map[string][]byte),
	}
}

// GetNode 从缓存获取节点
func (c *TreeCache) GetNode(hash string) (*Node, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	node, ok := c.nodeCache[hash]
	return node, ok
}

// SetNode 将节点添加到缓存
func (c *TreeCache) SetNode(hash string, node *Node) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.nodeCache[hash] = node
}

// GetRoot 从缓存获取根哈希
func (c *TreeCache) GetRoot(hashesKey string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	root, ok := c.rootCache[hashesKey]
	if ok {
		// 返回副本而非引用
		result := make([]byte, len(root))
		copy(result, root)
		return result, true
	}
	return nil, false
}

// SetRoot 将根哈希添加到缓存
func (c *TreeCache) SetRoot(hashesKey string, root []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 存储副本而非引用
	rootCopy := make([]byte, len(root))
	copy(rootCopy, root)
	c.rootCache[hashesKey] = rootCopy
}

// 全局树缓存
var globalTreeCache = NewTreeCache()

// hashesFingerprint 计算哈希列表的指纹，用于缓存
func hashesFingerprint(hashes [][]byte) []byte {
	// 连接所有哈希的前4字节
	data := make([]byte, 0, len(hashes)*4)
	for _, h := range hashes {
		switch { //nolint:gocritic // ifElseChain: 使用 switch 更清晰
		case len(h) == 0:
			// 包含空哈希时返回特殊指纹
			return []byte("empty_hash_error")
		case len(h) >= 4:
			data = append(data, h[:4]...)
		default:
			data = append(data, h...)
		}
	}

	// 计算指纹
	hash := sha256.Sum256(data)
	return hash[:]
}

// nodePool 节点对象池，减少GC压力
var nodePool = sync.Pool{
	New: func() interface{} {
		return &Node{}
	},
}

// getNode 从池中获取一个节点
func getNode() *Node {
	return nodePool.Get().(*Node)
}

// ComputeRoot 计算一组哈希的默克尔根
// 此实现使用缓存提高重复计算的性能
//
// 参数:
//   - hashes: 要计算的哈希列表
//
// 返回:
//   - []byte: 计算得到的默克尔根哈希
//   - error: 计算过程中的错误，成功时为nil
func ComputeRoot(hashes [][]byte) ([]byte, error) {
	if len(hashes) == 0 {
		return nil, ErrEmptyHashList
	}

	// 首先验证所有哈希
	for _, hash := range hashes {
		if len(hash) == 0 {
			return nil, ErrNilHash
		}
	}

	// 如果只有一个哈希，直接返回
	if len(hashes) == 1 {
		return hashes[0], nil
	}

	// 检查缓存
	fingerprint := hashesFingerprint(hashes)
	fingerprintKey := string(fingerprint)

	if cachedRoot, ok := globalTreeCache.GetRoot(fingerprintKey); ok {
		return cachedRoot, nil
	}

	// 使用并行构建优化性能
	root := parallelBuildTree(hashes)

	// 存入缓存
	globalTreeCache.SetRoot(fingerprintKey, root.Hash)

	return root.Hash, nil
}

// parallelBuildTree 并行构建默克尔树
func parallelBuildTree(hashes [][]byte) *Node {
	// 构建叶子节点
	leaves := make([]*Node, len(hashes))
	for i, hash := range hashes {
		node := getNode()
		node.Hash = hash
		leaves[i] = node
	}

	// 构建树
	root := buildTreeParallel(leaves)
	return root
}

// buildTreeParallel 并行构建树的上层，针对大型树提高性能
func buildTreeParallel(nodes []*Node) *Node {
	if len(nodes) == 0 {
		return nil
	}

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

	// 计算下一层父节点数量
	numParents := len(nodesList) / 2

	// 对于大型树使用并行处理
	if numParents > 32 {
		return buildTreeParallelLarge(nodesList)
	}

	// 两两组合，构建上一层节点
	parents := make([]*Node, 0, numParents)
	for i := 0; i < len(nodesList); i += 2 {
		left := nodesList[i]
		right := nodesList[i+1]

		// 检查缓存
		combinedHash := string(append(left.Hash, right.Hash...))
		if cachedNode, ok := globalTreeCache.GetNode(combinedHash); ok {
			parents = append(parents, cachedNode)
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

		parents = append(parents, parent)
	}

	// 递归构建树的上层
	return buildTreeParallel(parents)
}

// buildTreeParallelLarge 使用goroutine并行处理大型树
func buildTreeParallelLarge(nodes []*Node) *Node {
	numParents := len(nodes) / 2
	parents := make([]*Node, numParents)

	// 使用多个goroutine并行处理
	var wg sync.WaitGroup
	numGoroutines := 4 // 可调整为CPU核心数

	// 计算每个goroutine处理的节点数
	nodesPerGoroutine := (len(nodes)/2 + numGoroutines - 1) / numGoroutines

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)

		go func(goroutineID int) {
			defer wg.Done()

			// 计算此goroutine处理的范围
			startIdx := goroutineID * nodesPerGoroutine * 2
			endIdx := (goroutineID + 1) * nodesPerGoroutine * 2
			if endIdx > len(nodes) {
				endIdx = len(nodes)
			}

			// 处理分配的节点
			for i := startIdx; i < endIdx; i += 2 {
				if i+1 >= len(nodes) {
					break
				}

				left := nodes[i]
				right := nodes[i+1]

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
		}(g)
	}

	wg.Wait()

	// 递归构建树的上层
	return buildTreeParallel(parents)
}

// combineHashes 组合两个哈希值，计算新的哈希
//
// 参数:
//   - left: 左侧哈希
//   - right: 右侧哈希
//
// 返回:
//   - []byte: 组合后的哈希值
func combineHashes(left, right []byte) []byte {
	// 将两个哈希连接起来
	combined := make([]byte, len(left)+len(right))
	copy(combined, left)
	copy(combined[len(left):], right)

	// 计算SHA-256哈希
	hasher := sha256.New()
	hasher.Write(combined)
	return hasher.Sum(nil)
}
