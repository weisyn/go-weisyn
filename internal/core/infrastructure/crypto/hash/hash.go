// Package hash provides cryptographic hash functionality.
package hash

import (
	"crypto/sha256"
	"crypto/subtle"
	"hash"
	"sync"
	"time"

	cryptointf "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"golang.org/x/crypto/ripemd160"
	"golang.org/x/crypto/sha3"
)

// 确保HashService实现了cryptointf.HashManager接口
var _ cryptointf.HashManager = (*HashService)(nil)

// HashCache LRU哈希缓存结构（修复内存泄漏）
type HashCache struct {
	maxSize     int                   // 最大缓存容量
	cache       map[string]*cacheNode // 哈希表，O(1)查找
	head        *cacheNode            // 链表头节点（最近使用）
	tail        *cacheNode            // 链表尾节点（最少使用）
	currentSize int                   // 当前缓存大小
	mu          sync.RWMutex          // 读写锁
	totalBytes  int64                 // 缓存总字节数（用于统计）
}

// cacheNode 缓存节点
type cacheNode struct {
	key        string
	value      []byte
	prev       *cacheNode
	next       *cacheNode
	accessTime time.Time
}

// NewHashCache 创建新的哈希缓存（带LRU机制）
// maxSize: 最大缓存条目数（默认10000，约占用320KB-640KB内存）
func NewHashCache(maxSize int) *HashCache {
	if maxSize <= 0 {
		maxSize = 10000 // 默认10000个条目
	}

	cache := &HashCache{
		maxSize:     maxSize,
		cache:       make(map[string]*cacheNode),
		currentSize: 0,
	}

	// 创建虚拟头尾节点
	cache.head = &cacheNode{}
	cache.tail = &cacheNode{}
	cache.head.next = cache.tail
	cache.tail.prev = cache.head

	return cache
}

// Get 从缓存获取哈希值
func (c *HashCache) Get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	node, exists := c.cache[key]
	if !exists {
		return nil, false
	}

	// 缓存命中，移动到链表头部
	c.moveToHead(node)
	result := make([]byte, len(node.value))
	copy(result, node.value) // 返回副本而非引用
	return result, true
}

// Set 设置缓存中的哈希值
func (c *HashCache) Set(key string, value []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果键已存在，更新值
	if node, exists := c.cache[key]; exists {
		// 更新 totalBytes：减去旧值，加上新值
		c.totalBytes -= int64(len(node.value))
		node.value = make([]byte, len(value))
		copy(node.value, value) // 存储副本而非引用
		c.totalBytes += int64(len(node.value))
		node.accessTime = time.Now()
		c.moveToHead(node)
		return
	}

	// 创建新节点
	newNode := &cacheNode{
		key:        key,
		value:      make([]byte, len(value)),
		accessTime: time.Now(),
	}
	copy(newNode.value, value) // 存储副本而非引用

	// 添加到链表头部
	c.addToHead(newNode)
	c.cache[key] = newNode
	c.currentSize++
	c.totalBytes += int64(len(value))

	// 如果缓存已满，淘汰链表尾部的节点
	if c.currentSize > c.maxSize {
		c.evictTail()
	}
}

// Clear 清空缓存（实现 CacheCleaner 接口）
func (c *HashCache) Clear() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	freedBytes := uint64(c.totalBytes)
	c.cache = make(map[string]*cacheNode)
	c.head.next = c.tail
	c.tail.prev = c.head
	c.currentSize = 0
	c.totalBytes = 0

	return freedBytes
}

// Size 获取缓存大小
func (c *HashCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.currentSize
}

// Stats 获取缓存统计信息
func (c *HashCache) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	avgSize := int64(0)
	if c.currentSize > 0 {
		avgSize = c.totalBytes / int64(c.currentSize)
	}

	return map[string]interface{}{
		"size":          c.currentSize,
		"maxSize":       c.maxSize,
		"totalBytes":    c.totalBytes,
		"avgEntryBytes": avgSize,
	}
}

// moveToHead 将节点移动到链表头部
func (c *HashCache) moveToHead(node *cacheNode) {
	c.removeNode(node)
	c.addToHead(node)
	node.accessTime = time.Now()
}

// addToHead 将节点添加到链表头部
func (c *HashCache) addToHead(node *cacheNode) {
	node.prev = c.head
	node.next = c.head.next
	c.head.next.prev = node
	c.head.next = node
}

// removeNode 从链表中删除节点
func (c *HashCache) removeNode(node *cacheNode) {
	node.prev.next = node.next
	node.next.prev = node.prev
}

// evictTail 淘汰链表尾部的节点
func (c *HashCache) evictTail() {
	if c.tail.prev == c.head {
		return // 链表为空
	}

	lastNode := c.tail.prev
	c.totalBytes -= int64(len(lastNode.value))
	c.removeNode(lastNode)
	delete(c.cache, lastNode.key)
	c.currentSize--
}

// HashService 提供哈希计算功能
type HashService struct {
	// 缓存最近的哈希结果，避免重复计算
	sha256Cache       *HashCache
	keccak256Cache    *HashCache
	doubleSHA256Cache *HashCache
	ripemd160Cache    *HashCache // 新增RIPEMD160缓存
}

// Name 返回清理器名称（实现 CacheCleaner 接口）
func (s *HashService) Name() string {
	return "HashService"
}

// ClearCache 清理所有哈希缓存（实现 CacheCleaner 接口）
// 返回释放的估计字节数
func (s *HashService) ClearCache() uint64 {
	var totalFreed uint64
	totalFreed += s.sha256Cache.Clear()
	totalFreed += s.keccak256Cache.Clear()
	totalFreed += s.doubleSHA256Cache.Clear()
	totalFreed += s.ripemd160Cache.Clear()
	return totalFreed
}

// NewHashService 创建新的哈希服务
//
// 返回一个包含优化缓存的哈希服务实例
// 每个缓存默认最大10000个条目（约320KB-640KB内存）
func NewHashService() *HashService {
	return &HashService{
		sha256Cache:       NewHashCache(10000), // 默认10000个条目
		keccak256Cache:    NewHashCache(10000),
		doubleSHA256Cache: NewHashCache(10000),
		ripemd160Cache:    NewHashCache(10000), // 初始化RIPEMD160缓存
	}
}

// cacheKey 根据数据生成缓存键
// 🔥 修复：使用SHA256哈希作为缓存键，确保唯一性
func cacheKey(data []byte) string {
	// 对于任何大小的数据，都使用其SHA256哈希作为缓存键
	// 这确保了缓存键的唯一性，避免因数据截断导致的哈希冲突
	hasher := sha256.New()
	hasher.Write(data)
	keyHash := hasher.Sum(nil)
	return string(keyHash)
}

// SHA256 计算SHA-256哈希
//
// 参数:
//   - data: 要计算哈希的数据
//
// 返回:
//   - []byte: 32字节的SHA-256哈希结果
func (s *HashService) SHA256(data []byte) []byte {
	// 检查缓存
	key := cacheKey(data)
	if cachedHash, ok := s.sha256Cache.Get(key); ok {
		return cachedHash
	}

	// 计算哈希
	hash := sha256.Sum256(data)
	result := hash[:]

	// 存入缓存
	s.sha256Cache.Set(key, result)
	return result
}

// Keccak256 计算Keccak-256哈希
//
// 参数:
//   - data: 要计算哈希的数据
//
// 返回:
//   - []byte: 32字节的Keccak-256哈希结果
func (s *HashService) Keccak256(data []byte) []byte {
	// 检查缓存
	key := cacheKey(data)
	if cachedHash, ok := s.keccak256Cache.Get(key); ok {
		return cachedHash
	}

	// 计算哈希
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(data)
	result := hasher.Sum(nil)

	// 存入缓存
	s.keccak256Cache.Set(key, result)
	return result
}

// RIPEMD160 计算RIPEMD-160哈希
//
// 参数:
//   - data: 要计算哈希的数据
//
// 返回:
//   - []byte: 20字节的RIPEMD-160哈希结果
func (s *HashService) RIPEMD160(data []byte) []byte {
	// 检查缓存
	key := cacheKey(data)
	if cachedHash, ok := s.ripemd160Cache.Get(key); ok {
		return cachedHash
	}

	// 计算哈希
	hasher := ripemd160.New()
	hasher.Write(data)
	result := hasher.Sum(nil)

	// 存入缓存
	s.ripemd160Cache.Set(key, result)
	return result
}

// DoubleSHA256 计算双重SHA-256哈希
//
// 参数:
//   - data: 要计算哈希的数据
//
// 返回:
//   - []byte: 32字节的双重SHA-256哈希结果
func (s *HashService) DoubleSHA256(data []byte) []byte {
	// 检查缓存
	key := cacheKey(data)
	if cachedHash, ok := s.doubleSHA256Cache.Get(key); ok {
		return cachedHash
	}

	// 计算双重哈希
	first := sha256.Sum256(data)
	second := sha256.Sum256(first[:])
	result := second[:]

	// 存入缓存
	s.doubleSHA256Cache.Set(key, result)
	return result
}

// ============================================================================
//                           流式哈希计算实现
// ============================================================================

// NewSHA256Hasher 创建SHA-256流式哈希器
//
// 🎯 **流式哈希计算**
//
// 返回标准 hash.Hash 接口，支持分块写入和流式计算。
// 适用于大文件或流式数据的哈希计算，避免一次性加载全部数据到内存。
//
// 使用示例：
//
//	hasher := hashService.NewSHA256Hasher()
//	io.Copy(hasher, file)  // 流式读取文件
//	hash := hasher.Sum(nil)  // 获取最终哈希
//
// 返回:
//   - hash.Hash: 标准哈希接口，可用于 io.Writer
func (s *HashService) NewSHA256Hasher() hash.Hash {
	return sha256.New()
}

// NewRIPEMD160Hasher 创建RIPEMD-160流式哈希器
//
// 🎯 **流式哈希计算**
//
// 返回标准 hash.Hash 接口，支持分块写入和流式计算。
// 主要用于地址生成等场景的流式哈希计算。
//
// 使用示例：
//
//	hasher := hashService.NewRIPEMD160Hasher()
//	io.Copy(hasher, dataStream)
//	hash := hasher.Sum(nil)
//
// 返回:
//   - hash.Hash: 标准哈希接口，可用于 io.Writer
func (s *HashService) NewRIPEMD160Hasher() hash.Hash {
	return ripemd160.New()
}

// ============================================================================
//                           辅助工具函数
// ============================================================================

// ConstantTimeCompare 在常量时间内比较两个哈希值是否相等
// 用于防止时序攻击，无论何时都会比较整个字节数组
//
// 参数:
//   - a: 第一个哈希值
//   - b: 第二个哈希值
//
// 返回:
//   - bool: 如果两个哈希值相等返回true，否则返回false
func ConstantTimeCompare(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}
