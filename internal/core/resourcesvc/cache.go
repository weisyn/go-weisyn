package resourcesvc

import (
	"sync"
	"time"
)

// resourceViewCache ResourceView 的简单 LRU 缓存实现
//
// 设计目标：
// - O(1) 的 Get / Put（哈希表 + 双向链表）
// - 并发安全（读写锁）
// - 仅缓存按 contentHash 聚合的 ResourceView（代码级视图）

type resourceViewCache struct {
	maxSize int

	mu    sync.RWMutex
	cache map[string]*resourceViewNode
	head  *resourceViewNode
	tail  *resourceViewNode
	size  int
}

type resourceViewNode struct {
	key        string
	value      *ResourceView
	prev, next *resourceViewNode
	accessTime time.Time
}

func newResourceViewCache(maxSize int) *resourceViewCache {
	if maxSize <= 0 {
		maxSize = 1000
	}

	c := &resourceViewCache{
		maxSize: maxSize,
		cache:   make(map[string]*resourceViewNode),
	}
	// 虚拟头尾节点
	c.head = &resourceViewNode{}
	c.tail = &resourceViewNode{}
	c.head.next = c.tail
	c.tail.prev = c.head

	return c
}

func (c *resourceViewCache) Get(key string) (*ResourceView, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	node, ok := c.cache[key]
	if !ok {
		return nil, false
	}

	// 命中，移动到头部
	c.moveToHead(node)
	node.accessTime = time.Now()

	return node.value, true
}

func (c *resourceViewCache) Put(key string, value *ResourceView) {
	if value == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// 已存在则更新并移到头部
	if node, ok := c.cache[key]; ok {
		node.value = value
		node.accessTime = time.Now()
		c.moveToHead(node)
		return
	}

	// 新节点
	node := &resourceViewNode{
		key:        key,
		value:      value,
		accessTime: time.Now(),
	}
	c.cache[key] = node
	c.addToHead(node)
	c.size++

	// 超出容量则淘汰尾部
	if c.size > c.maxSize {
		c.evictTail()
	}
}

func (c *resourceViewCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.size
}

// Shrink 将缓存容量收缩到不超过 targetSize。
// 当前实现选择“快速收缩”：当当前 Size 大于 targetSize 时，直接重建一个更小容量的缓存，
// 让热点数据在后续访问中自然重新填充，避免在这里做复杂的尾部遍历和搬迁。
func (c *resourceViewCache) Shrink(targetSize int) {
	if targetSize <= 0 {
		targetSize = 1
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.size <= targetSize && c.maxSize <= targetSize {
		return
	}

	// 重建一个更小容量的缓存，丢弃旧数据以快速释放内存
	newMax := targetSize
	if newMax <= 0 {
		newMax = 1
	}

	c.maxSize = newMax
	c.cache = make(map[string]*resourceViewNode)
	c.head = &resourceViewNode{}
	c.tail = &resourceViewNode{}
	c.head.next = c.tail
	c.tail.prev = c.head
	c.size = 0
}

// -------- 链表操作 --------

func (c *resourceViewCache) addToHead(node *resourceViewNode) {
	node.prev = c.head
	node.next = c.head.next
	c.head.next.prev = node
	c.head.next = node
}

func (c *resourceViewCache) removeNode(node *resourceViewNode) {
	node.prev.next = node.next
	node.next.prev = node.prev
}

func (c *resourceViewCache) moveToHead(node *resourceViewNode) {
	c.removeNode(node)
	c.addToHead(node)
}

func (c *resourceViewCache) evictTail() {
	// tail.prev 是最久未使用的节点
	if c.tail.prev == nil || c.tail.prev == c.head {
		return
	}
	evicted := c.tail.prev
	c.removeNode(evicted)
	delete(c.cache, evicted.key)
	c.size--
}


