package block

import (
	"sync"

	core "github.com/weisyn/v1/pb/blockchain/block"
)

// blockCache 简单的按高度缓存区块的 LRU 缓存（仅在进程内使用）
//
// 设计目标：
// - O(1) Get / Put
// - 并发安全
// - 避免在高频读取同一高度附近区块时反复打磁盘

type blockCache struct {
	maxSize int

	mu    sync.RWMutex
	cache map[uint64]*blockNode
	head  *blockNode
	tail  *blockNode
	size  int
}

type blockNode struct {
	height uint64
	value  *core.Block
	prev   *blockNode
	next   *blockNode
}

func newBlockCache(maxSize int) *blockCache {
	if maxSize <= 0 {
		maxSize = 1000
	}
	c := &blockCache{
		maxSize: maxSize,
		cache:   make(map[uint64]*blockNode),
	}
	c.head = &blockNode{}
	c.tail = &blockNode{}
	c.head.next = c.tail
	c.tail.prev = c.head
	return c
}

func (c *blockCache) Get(height uint64) (*core.Block, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	node, ok := c.cache[height]
	if !ok {
		return nil, false
	}

	c.moveToHead(node)
	return node.value, true
}

func (c *blockCache) Put(height uint64, block *core.Block) {
	if block == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if node, ok := c.cache[height]; ok {
		node.value = block
		c.moveToHead(node)
		return
	}

	node := &blockNode{
		height: height,
		value:  block,
	}
	c.cache[height] = node
	c.addToHead(node)
	c.size++

	if c.size > c.maxSize {
		c.evictTail()
	}
}

func (c *blockCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.size
}

func (c *blockCache) addToHead(node *blockNode) {
	node.prev = c.head
	node.next = c.head.next
	c.head.next.prev = node
	c.head.next = node
}

func (c *blockCache) removeNode(node *blockNode) {
	node.prev.next = node.next
	node.next.prev = node.prev
}

func (c *blockCache) moveToHead(node *blockNode) {
	c.removeNode(node)
	c.addToHead(node)
}

func (c *blockCache) evictTail() {
	if c.tail.prev == nil || c.tail.prev == c.head {
		return
	}
	evicted := c.tail.prev
	c.removeNode(evicted)
	delete(c.cache, evicted.height)
	c.size--
}


