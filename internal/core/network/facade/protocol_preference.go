// protocol_preference.go - 协议偏好管理
// 🆕 MEDIUM-002 修复：优化协议协商机制，减少不必要的回退
package facade

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// ProtocolPreferenceType 协议偏好类型
type ProtocolPreferenceType int

const (
	// ProtocolPreferenceUnknown 未知（需要探测）
	ProtocolPreferenceUnknown ProtocolPreferenceType = iota
	// ProtocolPreferenceQualified 偏好 qualified 协议（带命名空间）
	ProtocolPreferenceQualified
	// ProtocolPreferenceOriginal 偏好 original 协议（不带命名空间）
	ProtocolPreferenceOriginal
)

// String 返回协议偏好类型的字符串表示
func (p ProtocolPreferenceType) String() string {
	switch p {
	case ProtocolPreferenceQualified:
		return "qualified"
	case ProtocolPreferenceOriginal:
		return "original"
	default:
		return "unknown"
	}
}

// PeerProtocolPreference 节点协议偏好记录
type PeerProtocolPreference struct {
	Preference    ProtocolPreferenceType
	LastUpdated   time.Time
	SuccessCount  int // 使用该偏好成功的次数
	FallbackCount int // 回退次数
}

// ProtocolPreferenceCache 协议偏好缓存
// 记录每个节点的协议偏好，避免每次都尝试 qualified 后回退
type ProtocolPreferenceCache struct {
	preferences   map[peer.ID]*PeerProtocolPreference
	mu            sync.RWMutex
	ttl           time.Duration // 偏好缓存有效期
	maxEntries    int           // 最大缓存条目数
	
	// 统计
	cacheHits      uint64
	cacheMisses    uint64
	fallbackSaved  uint64 // 避免的回退次数
}

// NewProtocolPreferenceCache 创建协议偏好缓存
func NewProtocolPreferenceCache(ttl time.Duration, maxEntries int) *ProtocolPreferenceCache {
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	if maxEntries <= 0 {
		maxEntries = 1000
	}
	return &ProtocolPreferenceCache{
		preferences: make(map[peer.ID]*PeerProtocolPreference),
		ttl:         ttl,
		maxEntries:  maxEntries,
	}
}

// GetPreference 获取节点的协议偏好
func (c *ProtocolPreferenceCache) GetPreference(peerID peer.ID) ProtocolPreferenceType {
	c.mu.RLock()
	defer c.mu.RUnlock()

	pref, ok := c.preferences[peerID]
	if !ok {
		atomic.AddUint64(&c.cacheMisses, 1)
		return ProtocolPreferenceUnknown
	}

	// 检查是否过期
	if time.Since(pref.LastUpdated) > c.ttl {
		atomic.AddUint64(&c.cacheMisses, 1)
		return ProtocolPreferenceUnknown
	}

	atomic.AddUint64(&c.cacheHits, 1)
	return pref.Preference
}

// RecordSuccess 记录协议使用成功
func (c *ProtocolPreferenceCache) RecordSuccess(peerID peer.ID, usedQualified bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	pref, ok := c.preferences[peerID]
	if !ok {
		pref = &PeerProtocolPreference{}
		c.preferences[peerID] = pref
	}

	if usedQualified {
		pref.Preference = ProtocolPreferenceQualified
	} else {
		pref.Preference = ProtocolPreferenceOriginal
	}
	pref.LastUpdated = time.Now()
	pref.SuccessCount++

	// 清理过期条目（如果超过最大条目数）
	if len(c.preferences) > c.maxEntries {
		c.cleanupExpired()
	}
}

// RecordFallback 记录协议回退
func (c *ProtocolPreferenceCache) RecordFallback(peerID peer.ID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	pref, ok := c.preferences[peerID]
	if !ok {
		pref = &PeerProtocolPreference{}
		c.preferences[peerID] = pref
	}

	// 回退意味着该节点不支持 qualified 协议
	pref.Preference = ProtocolPreferenceOriginal
	pref.LastUpdated = time.Now()
	pref.FallbackCount++
}

// RecordFallbackSaved 记录避免的回退
func (c *ProtocolPreferenceCache) RecordFallbackSaved() {
	atomic.AddUint64(&c.fallbackSaved, 1)
}

// cleanupExpired 清理过期条目（需要在持有锁的情况下调用）
func (c *ProtocolPreferenceCache) cleanupExpired() {
	now := time.Now()
	for peerID, pref := range c.preferences {
		if now.Sub(pref.LastUpdated) > c.ttl {
			delete(c.preferences, peerID)
		}
	}
}

// GetStats 获取缓存统计
func (c *ProtocolPreferenceCache) GetStats() ProtocolPreferenceCacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var qualifiedCount, originalCount, unknownCount int
	for _, pref := range c.preferences {
		switch pref.Preference {
		case ProtocolPreferenceQualified:
			qualifiedCount++
		case ProtocolPreferenceOriginal:
			originalCount++
		default:
			unknownCount++
		}
	}

	return ProtocolPreferenceCacheStats{
		TotalEntries:    len(c.preferences),
		QualifiedCount:  qualifiedCount,
		OriginalCount:   originalCount,
		UnknownCount:    unknownCount,
		CacheHits:       atomic.LoadUint64(&c.cacheHits),
		CacheMisses:     atomic.LoadUint64(&c.cacheMisses),
		FallbackSaved:   atomic.LoadUint64(&c.fallbackSaved),
	}
}

// Clear 清空缓存
func (c *ProtocolPreferenceCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.preferences = make(map[peer.ID]*PeerProtocolPreference)
}

// ProtocolPreferenceCacheStats 缓存统计信息
type ProtocolPreferenceCacheStats struct {
	TotalEntries   int
	QualifiedCount int
	OriginalCount  int
	UnknownCount   int
	CacheHits      uint64
	CacheMisses    uint64
	FallbackSaved  uint64
}

// ProtocolNegotiator 协议协商器
// 负责在调用前确定最优协议
type ProtocolNegotiator struct {
	cache           *ProtocolPreferenceCache
	networkNamespace string
}

// NewProtocolNegotiator 创建协议协商器
func NewProtocolNegotiator(namespace string, cacheTTL time.Duration, maxCacheEntries int) *ProtocolNegotiator {
	return &ProtocolNegotiator{
		cache:           NewProtocolPreferenceCache(cacheTTL, maxCacheEntries),
		networkNamespace: namespace,
	}
}

// SelectProtocol 选择最优协议
// 返回值：(推荐协议ID, 是否为qualified, 需要回退尝试)
func (n *ProtocolNegotiator) SelectProtocol(peerID peer.ID, baseProtocol, qualifiedProtocol string) (string, bool, bool) {
	// 如果没有命名空间，只使用原始协议
	if n.networkNamespace == "" || qualifiedProtocol == baseProtocol {
		return baseProtocol, false, false
	}

	// 查询缓存的偏好
	pref := n.cache.GetPreference(peerID)

	switch pref {
	case ProtocolPreferenceQualified:
		// 节点支持 qualified 协议
		n.cache.RecordFallbackSaved()
		return qualifiedProtocol, true, false

	case ProtocolPreferenceOriginal:
		// 节点只支持 original 协议，直接使用
		n.cache.RecordFallbackSaved()
		return baseProtocol, false, false

	default:
		// 未知偏好，需要尝试（先 qualified，可能需要回退）
		return qualifiedProtocol, true, true
	}
}

// RecordResult 记录协商结果
func (n *ProtocolNegotiator) RecordResult(peerID peer.ID, usedQualified, hadFallback bool) {
	if hadFallback {
		n.cache.RecordFallback(peerID)
	} else {
		n.cache.RecordSuccess(peerID, usedQualified)
	}
}

// GetCache 获取缓存（用于统计）
func (n *ProtocolNegotiator) GetCache() *ProtocolPreferenceCache {
	return n.cache
}

