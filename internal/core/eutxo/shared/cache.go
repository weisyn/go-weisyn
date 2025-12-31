// Package shared 提供 EUTXO 模块的共享工具
package shared

import (
	"sync"

	"google.golang.org/protobuf/proto"
)

// Cache UTXO 缓存管理器（P3-15：实现真正的 LRU 淘汰策略）
//
// 🎯 **设计目的**：
// - 缓存热点 UTXO，减少存储访问
// - 提升性能
// - 统计缓存命中率
//
// 💡 **实现**：
// - LRU 实现：使用 map + 双向链表实现真正的 LRU 淘汰策略
// - 并发安全：使用 RWMutex 保护
// - 性能优化：O(1) 的 Get、Put、Delete 操作
type Cache struct {
	capacity       int
	data           map[string]*cacheNode // 键到节点的映射
	hits           uint64
	misses         uint64
	mu             sync.RWMutex
	head           *cacheNode // 双向链表头部（最近访问的）
	tail           *cacheNode // 双向链表尾部（最久未访问的）
	totalSizeBytes int64      // 缓存中所有条目的序列化大小总和（bytes）
}

// cacheNode 双向链表节点
type cacheNode struct {
	key       string
	value     interface{}
	prev      *cacheNode
	next      *cacheNode
	sizeBytes int64 // 该条目的序列化大小（bytes），用于统计
}

// NewCache 创建缓存实例
//
// 参数：
//   - capacity: 缓存容量
func NewCache(capacity int) *Cache {
	// 创建头尾哨兵节点，简化边界处理
	head := &cacheNode{}
	tail := &cacheNode{}
	head.next = tail
	tail.prev = head

	return &Cache{
		capacity: capacity,
		data:     make(map[string]*cacheNode),
		head:     head,
		tail:     tail,
	}
}

// Put 添加到缓存（P3-15：实现真正的 LRU 淘汰策略）
//
// 🎯 **LRU 策略**：
// - 如果键已存在，更新值并移动到头部（标记为最近访问）
// - 如果键不存在，添加到头部
// - 如果缓存满，删除尾部节点（最久未访问的）
func (c *Cache) Put(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 计算新值的序列化大小（如果是 protobuf 消息）
	var newSize int64
	if msg, ok := value.(proto.Message); ok {
		newSize = int64(proto.Size(msg))
	}

	// 检查键是否已存在
	if node, exists := c.data[key]; exists {
		// 更新 totalSizeBytes：减去旧值，加上新值
		c.totalSizeBytes -= node.sizeBytes
		// 更新值并移动到头部（标记为最近访问）
		node.value = value
		node.sizeBytes = newSize
		c.totalSizeBytes += node.sizeBytes
		c.moveToHead(node)
		return
	}

	// 如果缓存满，删除尾部节点（最久未访问的）
	if len(c.data) >= c.capacity {
		c.evictTail()
	}

	// 创建新节点并添加到头部
	node := &cacheNode{
		key:       key,
		value:     value,
		sizeBytes: newSize,
	}
	c.addToHead(node)
	c.data[key] = node
	c.totalSizeBytes += node.sizeBytes
}

// Get 从缓存获取（P3-15：实现真正的 LRU 淘汰策略）
//
// 🎯 **LRU 策略**：
// - 如果命中，将节点移动到头部（标记为最近访问）
// - 如果未命中，更新统计信息
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	node, found := c.data[key]
	if found {
		// 命中：移动到头部（标记为最近访问）
		c.moveToHead(node)
		c.hits++
		return node.value, true
	}

	// 未命中
	c.misses++
	return nil, false
}

// Delete 从缓存删除（P3-15：实现真正的 LRU 淘汰策略）
//
// 🎯 **删除策略**：
// - 从 map 中删除
// - 从双向链表中移除节点
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	node, exists := c.data[key]
	if !exists {
		return
	}

	// 更新 totalSizeBytes
	c.totalSizeBytes -= node.sizeBytes

	// 从 map 中删除
	delete(c.data, key)

	// 从双向链表中移除
	c.removeNode(node)
}

// Size 获取缓存大小
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.data)
}

// Shrink 收缩缓存容量到不超过 targetSize。
// 当前实现采用快速重建策略：当缓存条目数大于 targetSize 时，重置内部 map 和链表结构，
// 释放内存并让热点数据在后续访问中自然重新填充。
func (c *Cache) Shrink(targetSize int) {
	if targetSize <= 0 {
		targetSize = 1
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.data) <= targetSize && c.capacity <= targetSize {
		return
	}

	if targetSize < c.capacity {
		c.capacity = targetSize
	}

	// 重建内部结构
	head := &cacheNode{}
	tail := &cacheNode{}
	head.next = tail
	tail.prev = head

	c.data = make(map[string]*cacheNode)
	c.head = head
	c.tail = tail
	c.totalSizeBytes = 0
}

// HitRate 获取缓存命中率
func (c *Cache) HitRate() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	if total == 0 {
		return 0.0
	}

	return float64(c.hits) / float64(total)
}

// ============================================================================
//                           内部辅助方法（LRU 实现）
// ============================================================================

// addToHead 将节点添加到链表头部
//
// 🎯 **操作**：
// - 将新节点插入到 head 和 head.next 之间
func (c *Cache) addToHead(node *cacheNode) {
	node.prev = c.head
	node.next = c.head.next
	c.head.next.prev = node
	c.head.next = node
}

// removeNode 从链表中移除节点
//
// 🎯 **操作**：
// - 将节点的前后节点连接起来
func (c *Cache) removeNode(node *cacheNode) {
	node.prev.next = node.next
	node.next.prev = node.prev
	node.prev = nil
	node.next = nil
}

// moveToHead 将节点移动到链表头部
//
// 🎯 **操作**：
// - 先移除节点，再添加到头部
func (c *Cache) moveToHead(node *cacheNode) {
	c.removeNode(node)
	c.addToHead(node)
}

// evictTail 淘汰尾部节点（最久未访问的）
//
// 🎯 **操作**：
// - 删除 tail.prev 节点（最久未访问的）
func (c *Cache) evictTail() {
	if len(c.data) == 0 {
		return
	}

	// 获取尾部节点（最久未访问的）
	tailNode := c.tail.prev
	if tailNode == c.head {
		// 链表为空（只有哨兵节点）
		return
	}

	// 从 map 中删除并更新 totalSizeBytes
	delete(c.data, tailNode.key)
	c.totalSizeBytes -= tailNode.sizeBytes

	// 从链表中移除
	c.removeNode(tailNode)
}

// AvgEntrySizeBytes 返回当前缓存中条目的平均序列化大小（bytes）
// 如果缓存为空或未能统计大小，则返回 0。
func (c *Cache) AvgEntrySizeBytes() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.data) == 0 {
		return 0
	}
	return c.totalSizeBytes / int64(len(c.data))
}
