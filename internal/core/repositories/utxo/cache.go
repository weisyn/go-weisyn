// Package utxo UTXO缓存管理实现
//
// 🧠 **UTXO缓存管理器 (UTXO Cache Manager)**
//
// 本文件实现UTXO的高效缓存管理：
// - 热数据缓存：缓存频繁访问的UTXO数据
// - LRU策略：基于最近最少使用的缓存淘汰策略
// - 失效管理：UTXO状态变更时的缓存失效处理
// - 性能优化：显著提升UTXO查询性能
//
// 🎯 **核心功能**
// - 智能缓存：基于访问模式的智能缓存策略
// - 快速访问：毫秒级的缓存数据访问
// - 一致性保证：确保缓存数据与存储数据的一致性
// - 内存管理：有效控制缓存的内存占用
//
// 🏗️ **设计原则**
// - 性能优先：缓存操作不影响主流程性能
// - 一致性保障：严格保证缓存与存储的一致性
// - 内存高效：合理控制缓存内存占用
// - 简约实现：遵循WES极简设计原则
package utxo

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/utils"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxo "github.com/weisyn/v1/pb/blockchain/utxo"
)

// ============================================================================
//                              缓存管理器定义
// ============================================================================

// CacheManager UTXO缓存管理器
//
// 🎯 **缓存管理核心**
//
// 负责管理UTXO数据的内存缓存，提供高效的数据访问和缓存策略。
// 采用LRU策略管理缓存条目，确保热数据的高速访问。
//
// 架构特点：
// - LRU策略：最近最少使用的缓存淘汰机制
// - 线程安全：支持高并发的缓存访问
// - 自动失效：UTXO状态变更时自动失效相关缓存
// - 统计监控：提供缓存命中率等统计信息
type CacheManager struct {
	// 核心依赖
	logger      log.Logger          // 日志服务
	memoryStore storage.MemoryStore // 内存存储引擎

	// 缓存配置
	maxSize  int           // 最大缓存条目数
	cacheTTL time.Duration // 缓存生存时间
	enabled  bool          // 是否启用缓存

	// 缓存数据
	cache      map[string]*CacheEntry // 缓存数据映射：outpoint_key -> cache_entry
	accessList *AccessList            // LRU访问列表
	mutex      sync.RWMutex           // 读写锁保护

	// 统计信息
	stats *CacheStats // 缓存统计
}

// ============================================================================
//                              缓存数据结构
// ============================================================================

// CacheEntry 缓存条目
//
// 🎯 **缓存条目数据**：
// 包含缓存的UTXO数据及其元数据信息。
type CacheEntry struct {
	UTXO         *utxo.UTXO  // 缓存的UTXO数据
	CachedAt     time.Time   // 缓存时间
	LastAccessed time.Time   // 最后访问时间
	AccessCount  int         // 访问计数
	ListNode     *AccessNode // LRU链表节点
}

// AccessList LRU访问列表
//
// 🎯 **LRU链表实现**：
// 双向链表实现的LRU访问顺序管理。
type AccessList struct {
	head *AccessNode // 链表头（最新访问）
	tail *AccessNode // 链表尾（最旧访问）
	size int         // 链表大小
}

// AccessNode 访问节点
//
// 🎯 **LRU链表节点**：
// LRU双向链表的节点结构。
type AccessNode struct {
	Key  string      // 缓存键
	Prev *AccessNode // 前驱节点
	Next *AccessNode // 后继节点
}

// CacheStats 缓存统计信息
//
// 🎯 **缓存性能统计**：
// 提供缓存命中率、访问统计等监控数据。
type CacheStats struct {
	HitCount      int64     // 缓存命中次数
	MissCount     int64     // 缓存未命中次数
	TotalRequests int64     // 总请求次数
	HitRate       float64   // 缓存命中率
	CurrentSize   int       // 当前缓存条目数
	EvictionCount int64     // 淘汰次数
	LastUpdated   time.Time // 最后更新时间
}

// ============================================================================
//                              构造函数
// ============================================================================

// NewCacheManager 创建UTXO缓存管理器实例
//
// 🏗️ **构造器模式**
//
// 参数：
//   - config: 缓存配置
//   - logger: 日志服务
//   - memoryStore: 内存存储引擎
//
// 返回：
//   - *CacheManager: 缓存管理器实例
//   - error: 创建错误
func NewCacheManager(config CacheConfig, logger log.Logger, memoryStore storage.MemoryStore) (*CacheManager, error) {
	// 参数验证
	if config.Size < 0 {
		return nil, fmt.Errorf("缓存大小不能为负数: %d", config.Size)
	}
	if config.TTL <= 0 {
		return nil, fmt.Errorf("缓存TTL必须为正数: %v", config.TTL)
	}

	manager := &CacheManager{
		logger:      logger,
		memoryStore: memoryStore,
		maxSize:     config.Size,
		cacheTTL:    config.TTL,
		enabled:     config.Enabled,
		cache:       make(map[string]*CacheEntry),
		accessList:  NewAccessList(),
		stats:       &CacheStats{LastUpdated: time.Now()},
	}

	if logger != nil {
		logger.Debugf("UTXO缓存管理器初始化完成 - maxSize: %d, ttl: %v, enabled: %t",
			config.Size, config.TTL, config.Enabled)
	}

	return manager, nil
}

// ============================================================================
//                           🔍 缓存查询操作
// ============================================================================

// Get 从缓存获取UTXO
//
// 🎯 **缓存查询核心**：
// 尝试从缓存获取UTXO数据，如果缓存命中则更新访问统计。
//
// 参数：
//   - ctx: 上下文
//   - outpoint: UTXO位置标识
//
// 返回：
//   - *utxo.UTXO: 缓存的UTXO数据，nil表示缓存未命中
//   - bool: 是否缓存命中
//   - error: 查询错误
func (cm *CacheManager) Get(ctx context.Context, outpoint *transaction.OutPoint) (*utxo.UTXO, bool, error) {
	if !cm.enabled {
		return nil, false, nil // 缓存未启用
	}

	// 构建缓存键
	cacheKey := cm.formatCacheKey(outpoint)

	cm.mutex.RLock()
	entry, exists := cm.cache[cacheKey]
	cm.mutex.RUnlock()

	// 更新统计
	cm.updateStats(exists)

	if !exists {
		return nil, false, nil // 缓存未命中
	}

	// 检查缓存是否过期
	if cm.isCacheExpired(entry) {
		cm.evict(cacheKey)
		cm.updateStats(false) // 视为缓存未命中
		return nil, false, nil
	}

	// 更新访问信息
	cm.mutex.Lock()
	entry.LastAccessed = time.Now()
	entry.AccessCount++
	cm.accessList.MoveToFront(entry.ListNode)
	cm.mutex.Unlock()

	if cm.logger != nil {
		cm.logger.Debugf("缓存命中 - key: %s, accessCount: %d", cacheKey, entry.AccessCount)
	}

	return entry.UTXO, true, nil
}

// ============================================================================
//                           💾 缓存更新操作
// ============================================================================

// Put 将UTXO放入缓存
//
// 🎯 **缓存存储核心**：
// 将UTXO数据存入缓存，如果缓存已满则按LRU策略淘汰旧数据。
//
// 参数：
//   - ctx: 上下文
//   - outpoint: UTXO位置标识
//   - utxoData: UTXO数据
//
// 返回：
//   - error: 存储错误
func (cm *CacheManager) Put(ctx context.Context, outpoint *transaction.OutPoint, utxoData *utxo.UTXO) error {
	if !cm.enabled || utxoData == nil {
		return nil // 缓存未启用或数据为空
	}

	cacheKey := cm.formatCacheKey(outpoint)
	now := time.Now()

	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// 检查缓存是否已存在
	if existingEntry, exists := cm.cache[cacheKey]; exists {
		// 更新现有缓存条目
		existingEntry.UTXO = utxoData
		existingEntry.LastAccessed = now
		existingEntry.AccessCount++
		cm.accessList.MoveToFront(existingEntry.ListNode)

		if cm.logger != nil {
			cm.logger.Debugf("更新缓存条目 - key: %s", cacheKey)
		}
		return nil
	}

	// 检查缓存是否已满
	if len(cm.cache) >= cm.maxSize && cm.maxSize > 0 {
		cm.evictLRU()
	}

	// 创建新缓存条目
	entry := &CacheEntry{
		UTXO:         utxoData,
		CachedAt:     now,
		LastAccessed: now,
		AccessCount:  1,
	}

	// 添加到LRU链表头部
	entry.ListNode = cm.accessList.AddToFront(cacheKey)

	// 存入缓存
	cm.cache[cacheKey] = entry

	if cm.logger != nil {
		cm.logger.Debugf("缓存新条目 - key: %s, cacheSize: %d", cacheKey, len(cm.cache))
	}

	return nil
}

// ============================================================================
//                           🗑️ 缓存失效操作
// ============================================================================

// Invalidate 使缓存失效
//
// 🎯 **缓存失效核心**：
// 当UTXO状态发生变化时，使相关缓存条目失效，保证缓存一致性。
//
// 参数：
//   - ctx: 上下文
//   - outpoint: UTXO位置标识
//
// 返回：
//   - error: 失效处理错误
func (cm *CacheManager) Invalidate(ctx context.Context, outpoint *transaction.OutPoint) error {
	if !cm.enabled {
		return nil // 缓存未启用
	}

	cacheKey := cm.formatCacheKey(outpoint)

	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if entry, exists := cm.cache[cacheKey]; exists {
		// 从LRU链表移除
		cm.accessList.Remove(entry.ListNode)

		// 从缓存映射移除
		delete(cm.cache, cacheKey)

		// 更新统计
		cm.stats.EvictionCount++

		if cm.logger != nil {
			cm.logger.Debugf("缓存失效 - key: %s, remaining: %d", cacheKey, len(cm.cache))
		}
	}

	return nil
}

// InvalidateByAddress 使地址相关的所有缓存失效
//
// 🎯 **批量缓存失效**：
// 当地址的UTXO发生批量变化时，使该地址相关的所有缓存失效。
//
// 参数：
//   - ctx: 上下文
//   - address: 所有者地址
//
// 返回：
//   - int: 失效的缓存条目数
//   - error: 失效处理错误
func (cm *CacheManager) InvalidateByAddress(ctx context.Context, address []byte) (int, error) {
	if !cm.enabled {
		return 0, nil // 缓存未启用
	}

	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	invalidatedCount := 0
	keysToRemove := make([]string, 0)

	// 遍历所有缓存条目，找到属于指定地址的UTXO
	for key, entry := range cm.cache {
		if entry.UTXO != nil && len(entry.UTXO.OwnerAddress) == len(address) {
			// 比较地址
			match := true
			for i, b := range address {
				if entry.UTXO.OwnerAddress[i] != b {
					match = false
					break
				}
			}

			if match {
				keysToRemove = append(keysToRemove, key)
			}
		}
	}

	// 移除匹配的缓存条目
	for _, key := range keysToRemove {
		if entry, exists := cm.cache[key]; exists {
			cm.accessList.Remove(entry.ListNode)
			delete(cm.cache, key)
			invalidatedCount++
		}
	}

	// 更新统计
	cm.stats.EvictionCount += int64(invalidatedCount)

	if cm.logger != nil && invalidatedCount > 0 {
		cm.logger.Debugf("批量缓存失效 - address: %x, invalidated: %d, remaining: %d",
			address, invalidatedCount, len(cm.cache))
	}

	return invalidatedCount, nil
}

// ============================================================================
//                           📊 缓存统计和监控
// ============================================================================

// GetStats 获取缓存统计信息
//
// 🎯 **缓存监控核心**：
// 返回缓存的详细统计信息，用于性能监控和调优。
//
// 返回：
//   - CacheStats: 缓存统计信息的副本
func (cm *CacheManager) GetStats() CacheStats {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	// 计算命中率
	hitRate := 0.0
	if cm.stats.TotalRequests > 0 {
		hitRate = float64(cm.stats.HitCount) / float64(cm.stats.TotalRequests)
	}

	return CacheStats{
		HitCount:      cm.stats.HitCount,
		MissCount:     cm.stats.MissCount,
		TotalRequests: cm.stats.TotalRequests,
		HitRate:       hitRate,
		CurrentSize:   len(cm.cache),
		EvictionCount: cm.stats.EvictionCount,
		LastUpdated:   time.Now(),
	}
}

// ResetStats 重置缓存统计信息
//
// 🎯 **统计重置功能**：
// 重置所有统计计数器，用于重新开始统计监控。
func (cm *CacheManager) ResetStats() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cm.stats = &CacheStats{
		LastUpdated: time.Now(),
	}

	if cm.logger != nil {
		cm.logger.Debug("缓存统计信息已重置")
	}
}

// ============================================================================
//                           🔧 内部辅助方法
// ============================================================================

// formatCacheKey 格式化缓存键
// 使用统一的 utils.OutPointKey 确保格式一致性
func (cm *CacheManager) formatCacheKey(outpoint *transaction.OutPoint) string {
	return utils.OutPointKey(outpoint)
}

// isCacheExpired 检查缓存是否过期
func (cm *CacheManager) isCacheExpired(entry *CacheEntry) bool {
	return time.Since(entry.CachedAt) > cm.cacheTTL
}

// updateStats 更新统计信息
func (cm *CacheManager) updateStats(hit bool) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cm.stats.TotalRequests++
	if hit {
		cm.stats.HitCount++
	} else {
		cm.stats.MissCount++
	}
}

// evict 淘汰指定缓存条目
func (cm *CacheManager) evict(key string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if entry, exists := cm.cache[key]; exists {
		cm.accessList.Remove(entry.ListNode)
		delete(cm.cache, key)
		cm.stats.EvictionCount++
	}
}

// evictLRU 按LRU策略淘汰最旧的缓存条目
func (cm *CacheManager) evictLRU() {
	if cm.accessList.tail != nil {
		key := cm.accessList.tail.Key
		if entry, exists := cm.cache[key]; exists {
			cm.accessList.Remove(entry.ListNode)
			delete(cm.cache, key)
			cm.stats.EvictionCount++

			if cm.logger != nil {
				cm.logger.Debugf("LRU淘汰缓存条目 - key: %s", key)
			}
		}
	}
}

// ============================================================================
//                           🔗 LRU链表实现
// ============================================================================

// NewAccessList 创建新的访问列表
func NewAccessList() *AccessList {
	return &AccessList{}
}

// AddToFront 在链表头部添加节点
func (al *AccessList) AddToFront(key string) *AccessNode {
	node := &AccessNode{Key: key}

	if al.head == nil {
		al.head = node
		al.tail = node
	} else {
		node.Next = al.head
		al.head.Prev = node
		al.head = node
	}

	al.size++
	return node
}

// MoveToFront 将节点移动到链表头部
func (al *AccessList) MoveToFront(node *AccessNode) {
	if node == al.head {
		return // 已经在头部
	}

	// 从当前位置移除
	al.removeNode(node)

	// 添加到头部
	node.Prev = nil
	node.Next = al.head
	if al.head != nil {
		al.head.Prev = node
	}
	al.head = node

	if al.tail == nil {
		al.tail = node
	}
}

// Remove 从链表中移除节点
func (al *AccessList) Remove(node *AccessNode) {
	al.removeNode(node)
	al.size--
}

// removeNode 内部方法：移除节点
func (al *AccessList) removeNode(node *AccessNode) {
	if node.Prev != nil {
		node.Prev.Next = node.Next
	} else {
		al.head = node.Next
	}

	if node.Next != nil {
		node.Next.Prev = node.Prev
	} else {
		al.tail = node.Prev
	}
}
