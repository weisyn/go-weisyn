// Package builder 提供区块构建服务的实现
package builder

import (
	"sync"
	"time"

	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"google.golang.org/protobuf/proto"
)

// ============================================================================
//                              LRU缓存实现
// ============================================================================

// CandidateLRUCache 候选区块LRU缓存实现
//
// 🎯 **高性能候选区块LRU缓存服务**
//
// 使用双向链表+哈希表实现O(1)时间复杂度的LRU缓存。
// 支持并发安全访问，自动淘汰最近最少使用的候选区块。
//
// 💡 **核心价值**：
// - ✅ **高性能**: O(1)时间复杂度的读写操作
// - ✅ **并发安全**: 使用读写锁保证并发安全
// - ✅ **自动淘汰**: LRU策略自动淘汰旧候选区块
// - ✅ **容量控制**: 可配置最大缓存容量
// - ✅ **性能监控**: 实时监控缓存使用情况
type CandidateLRUCache struct {
	maxSize        int                   // 最大缓存容量
	mu             sync.RWMutex          // 读写锁
	cache          map[string]*cacheNode // 哈希表，O(1)查找
	head           *cacheNode            // 链表头节点（最近使用）
	tail           *cacheNode            // 链表尾节点（最少使用）
	currentSize    int                   // 当前缓存大小
	hitCount       int64                 // 缓存命中次数
	missCount      int64                 // 缓存未命中次数
	logger         log.Logger            // 日志记录器（可选）
	totalSizeBytes int64                 // 缓存中所有区块序列化大小总和（bytes）
}

// cacheNode 缓存节点
type cacheNode struct {
	key        string          // 缓存键（区块哈希）
	value      *core.Block     // 缓存值（候选区块）
	prev       *cacheNode      // 前驱节点
	next       *cacheNode      // 后继节点
	accessTime time.Time       // 访问时间
	sizeBytes  int64           // 区块的序列化大小（bytes），用于统计
}

// NewCandidateLRUCache 创建候选区块LRU缓存实例
//
// 🎯 **缓存工厂方法**
//
// 💡 **参数说明**：
//   - maxSize: 最大缓存容量（0表示使用默认值100）
//   - logger: 日志记录器（可选）
//
// 💡 **返回值说明**：
//   - *CandidateLRUCache: 候选区块LRU缓存实例
func NewCandidateLRUCache(maxSize int, logger log.Logger) *CandidateLRUCache {
	if maxSize <= 0 {
		maxSize = 100 // 默认100个候选区块
	}

	cache := &CandidateLRUCache{
		maxSize:     maxSize,
		cache:       make(map[string]*cacheNode),
		currentSize: 0,
		hitCount:    0,
		missCount:   0,
		logger:      logger,
	}

	// 创建虚拟头尾节点
	cache.head = &cacheNode{}
	cache.tail = &cacheNode{}
	cache.head.next = cache.tail
	cache.tail.prev = cache.head

	return cache
}

// Get 获取缓存值
//
// 🎯 **获取缓存的核心方法**
//
// 如果缓存命中，将节点移动到链表头部（标记为最近使用）。
// 如果缓存未命中，返回nil。
//
// 💡 **参数说明**：
//   - key: 缓存键（区块哈希）
//
// 💡 **返回值说明**：
//   - *core.Block: 缓存值，如果不存在返回nil
//   - bool: 是否命中缓存
func (c *CandidateLRUCache) Get(key string) (*core.Block, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 查找缓存
	node, exists := c.cache[key]
	if !exists {
		c.missCount++
		return nil, false
	}

	// 缓存命中，移动到链表头部
	c.moveToHead(node)
	c.hitCount++

	return node.value, true
}

// Put 添加缓存值
//
// 🎯 **添加缓存的核心方法**
//
// 如果键已存在，更新值并移动到链表头部。
// 如果键不存在，创建新节点并添加到链表头部。
// 如果缓存已满，淘汰链表尾部的节点。
//
// 💡 **参数说明**：
//   - key: 缓存键（区块哈希）
//   - value: 缓存值（候选区块）
func (c *CandidateLRUCache) Put(key string, value *core.Block) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 计算新区块的序列化大小（用于后续平均值估算）
	var newSize int64
	if value != nil {
		newSize = int64(proto.Size(value))
	}

	// 如果键已存在，更新值
	if node, exists := c.cache[key]; exists {
		// 更新 totalSizeBytes：减去旧值，加上新值
		c.totalSizeBytes -= node.sizeBytes
		node.value = value
		node.sizeBytes = newSize
		c.totalSizeBytes += node.sizeBytes
		node.accessTime = time.Now()
		c.moveToHead(node)
		return
	}

	// 创建新节点
	newNode := &cacheNode{
		key:        key,
		value:      value,
		accessTime: time.Now(),
		sizeBytes:  newSize,
	}

	// 添加到链表头部
	c.addToHead(newNode)
	c.cache[key] = newNode
	c.currentSize++
	c.totalSizeBytes += newNode.sizeBytes

	// 如果缓存已满，淘汰链表尾部的节点
	if c.currentSize > c.maxSize {
		c.evictTail()
	}
}

// Delete 删除缓存值
//
// 🎯 **删除缓存的核心方法**
//
// 从哈希表和链表中删除指定的缓存项。
//
// 💡 **参数说明**：
//   - key: 缓存键（区块哈希）
func (c *CandidateLRUCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	node, exists := c.cache[key]
	if !exists {
		return
	}

	// 更新 totalSizeBytes
	c.totalSizeBytes -= node.sizeBytes

	// 从链表中删除
	c.removeNode(node)
	delete(c.cache, key)
	c.currentSize--
}

// Clear 清空缓存
//
// 🎯 **清空缓存的核心方法**
//
// 清空所有缓存数据，重置统计信息。
func (c *CandidateLRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]*cacheNode)
	c.head.next = c.tail
	c.tail.prev = c.head
	c.currentSize = 0
	c.totalSizeBytes = 0
	c.hitCount = 0
	c.missCount = 0

	if c.logger != nil {
		c.logger.Infof("[CandidateLRUCache] 缓存已清空")
	}
}

// Size 获取缓存大小
//
// 🎯 **获取缓存大小**
//
// 返回当前缓存的元素数量。
func (c *CandidateLRUCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.currentSize
}

// Stats 获取缓存统计信息
//
// 🎯 **获取缓存统计信息**
//
// 返回缓存的统计信息，包括命中率、命中次数、未命中次数等。
func (c *CandidateLRUCache) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	totalRequests := c.hitCount + c.missCount
	hitRate := float64(0)
	if totalRequests > 0 {
		hitRate = float64(c.hitCount) / float64(totalRequests) * 100
	}

	avgSize := int64(0)
	if c.currentSize > 0 {
		avgSize = c.totalSizeBytes / int64(c.currentSize)
	}

	return map[string]interface{}{
		"size":             c.currentSize,
		"maxSize":          c.maxSize,
		"hitCount":         c.hitCount,
		"missCount":        c.missCount,
		"hitRate":          hitRate,
		"totalRequests":    totalRequests,
		"totalSizeBytes":   c.totalSizeBytes,
		"avgBlockSizeByte": avgSize,
	}
}

// AvgBlockSizeBytes 返回当前缓存中区块的平均序列化大小（bytes）
// 如果缓存为空，返回 0。
func (c *CandidateLRUCache) AvgBlockSizeBytes() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.currentSize == 0 {
		return 0
	}
	return c.totalSizeBytes / int64(c.currentSize)
}

// ============================================================================
//                              内部辅助方法
// ============================================================================

// moveToHead 将节点移动到链表头部
func (c *CandidateLRUCache) moveToHead(node *cacheNode) {
	c.removeNode(node)
	c.addToHead(node)
	node.accessTime = time.Now() // 更新访问时间
}

// addToHead 将节点添加到链表头部
func (c *CandidateLRUCache) addToHead(node *cacheNode) {
	node.prev = c.head
	node.next = c.head.next
	c.head.next.prev = node
	c.head.next = node
}

// removeNode 从链表中删除节点
func (c *CandidateLRUCache) removeNode(node *cacheNode) {
	node.prev.next = node.next
	node.next.prev = node.prev
}

// evictTail 淘汰链表尾部的节点
func (c *CandidateLRUCache) evictTail() {
	if c.tail.prev == c.head {
		return // 链表为空
	}

	lastNode := c.tail.prev
	// 更新 totalSizeBytes
	c.totalSizeBytes -= lastNode.sizeBytes
	c.removeNode(lastNode)
	delete(c.cache, lastNode.key)
	c.currentSize--

	if c.logger != nil {
		c.logger.Debugf("[CandidateLRUCache] 淘汰候选区块 - 哈希: %s", lastNode.key)
	}
}

