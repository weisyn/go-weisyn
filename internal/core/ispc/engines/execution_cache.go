package engines

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ============================================================================
// 执行结果缓存
// ============================================================================
//
// 🎯 **目的**：
//   - 缓存相同输入的执行结果，避免重复计算
//   - 提升WASM和ONNX引擎的执行性能
//   - 减少资源消耗
//
// 📋 **设计原则**：
//   - 基于输入哈希的缓存键
//   - 支持TTL（生存时间）和最大缓存大小
//   - 线程安全的LRU缓存
//   - 支持缓存统计和监控
//
// ⚠️ **注意事项**：
//   - 仅缓存确定性执行的结果
//   - 不缓存包含随机性或时间依赖的执行结果
//   - 缓存大小和TTL需要根据实际场景调整
//
// ============================================================================

// ExecutionResultCache 执行结果缓存
type ExecutionResultCache struct {
	logger log.Logger

	// 缓存存储
	cache map[string]*CachedExecutionResult
	mu    sync.RWMutex

	// 缓存配置
	maxSize      int           // 最大缓存条目数
	defaultTTL   time.Duration // 默认生存时间
	cleanupInterval time.Duration // 清理间隔

	// 统计信息
	hits   uint64 // 缓存命中次数
	misses uint64 // 缓存未命中次数
	evictions uint64 // 缓存驱逐次数

	// 清理控制
	stopCleanup chan struct{}
	cleanupOnce sync.Once
}

// CachedExecutionResult 缓存的执行结果
type CachedExecutionResult struct {
	Result      interface{}   // 执行结果
	Error       error         // 执行错误（如果有）
	CachedAt    time.Time     // 缓存时间
	ExpiresAt   time.Time     // 过期时间
	AccessCount uint64        // 访问次数
	LastAccess  time.Time     // 最后访问时间
}

// ExecutionCacheKey 执行缓存键
type ExecutionCacheKey struct {
	EngineType string      // 引擎类型（"wasm"或"onnx"）
	ContractID string      // 合约/模型标识符
	Function   string      // 函数名（WASM）或空（ONNX）
	InputHash  string      // 输入哈希（SHA-256）
}

// String 返回缓存键的字符串表示
func (k *ExecutionCacheKey) String() string {
	return fmt.Sprintf("%s:%s:%s:%s", k.EngineType, k.ContractID, k.Function, k.InputHash)
}

// NewExecutionResultCache 创建执行结果缓存
func NewExecutionResultCache(logger log.Logger, maxSize int, defaultTTL time.Duration) *ExecutionResultCache {
	cache := &ExecutionResultCache{
		logger:          logger,
		cache:           make(map[string]*CachedExecutionResult),
		maxSize:         maxSize,
		defaultTTL:      defaultTTL,
		cleanupInterval: defaultTTL / 2, // 清理间隔为TTL的一半
		stopCleanup:     make(chan struct{}),
	}

	// 启动后台清理goroutine
	go cache.cleanupExpiredEntries()

	return cache
}

// Get 获取缓存的执行结果
//
// 📋 **参数**：
//   - key: 缓存键
//
// 🔧 **返回值**：
//   - result: 缓存的执行结果（如果存在且未过期）
//   - error: 缓存的执行错误（如果存在且未过期）
//   - found: 是否找到有效的缓存
func (c *ExecutionResultCache) Get(key *ExecutionCacheKey) (result interface{}, err error, found bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cacheKey := key.String()
	cached, exists := c.cache[cacheKey]

	if !exists {
		c.misses++
		return nil, nil, false
	}

	// 检查是否过期
	if time.Now().After(cached.ExpiresAt) {
		c.misses++
		return nil, nil, false
	}

	// 更新访问统计
	cached.AccessCount++
	cached.LastAccess = time.Now()

	c.hits++
	return cached.Result, cached.Error, true
}

// Set 设置缓存的执行结果
//
// 📋 **参数**：
//   - key: 缓存键
//   - result: 执行结果
//   - err: 执行错误（如果有）
//   - ttl: 生存时间（如果为0则使用默认TTL）
func (c *ExecutionResultCache) Set(key *ExecutionCacheKey, result interface{}, err error, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ttl == 0 {
		ttl = c.defaultTTL
	}

	cacheKey := key.String()
	now := time.Now()

	// 如果缓存已满，执行LRU驱逐
	if len(c.cache) >= c.maxSize {
		c.evictLRU()
	}

	c.cache[cacheKey] = &CachedExecutionResult{
		Result:      result,
		Error:       err,
		CachedAt:    now,
		ExpiresAt:   now.Add(ttl),
		AccessCount: 0,
		LastAccess:  now,
	}
}

// Clear 清空缓存
func (c *ExecutionResultCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]*CachedExecutionResult)
	c.hits = 0
	c.misses = 0
	c.evictions = 0
}

// GetStats 获取缓存统计信息
func (c *ExecutionResultCache) GetStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	totalRequests := c.hits + c.misses
	hitRate := 0.0
	if totalRequests > 0 {
		hitRate = float64(c.hits) / float64(totalRequests) * 100
	}

	return map[string]interface{}{
		"size":          len(c.cache),
		"max_size":      c.maxSize,
		"hits":          c.hits,
		"misses":        c.misses,
		"hit_rate":      hitRate,
		"evictions":     c.evictions,
		"total_requests": totalRequests,
	}
}

// evictLRU 驱逐最近最少使用的缓存条目
func (c *ExecutionResultCache) evictLRU() {
	if len(c.cache) == 0 {
		return
	}

	// 找到最近最少使用的条目
	var lruKey string
	var lruTime time.Time = time.Now()

	for key, entry := range c.cache {
		if entry.LastAccess.Before(lruTime) {
			lruTime = entry.LastAccess
			lruKey = key
		}
	}

	if lruKey != "" {
		delete(c.cache, lruKey)
		c.evictions++
	}
}

// cleanupExpiredEntries 清理过期的缓存条目
func (c *ExecutionResultCache) cleanupExpiredEntries() {
	ticker := time.NewTicker(c.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.stopCleanup:
			return
		}
	}
}

// cleanup 清理过期的缓存条目
func (c *ExecutionResultCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	expiredCount := 0

	for key, entry := range c.cache {
		if now.After(entry.ExpiresAt) {
			delete(c.cache, key)
			expiredCount++
		}
	}

	if expiredCount > 0 && c.logger != nil {
		c.logger.Debugf("清理了 %d 个过期的缓存条目", expiredCount)
	}
}

// Stop 停止缓存清理goroutine
func (c *ExecutionResultCache) Stop() {
	c.cleanupOnce.Do(func() {
		close(c.stopCleanup)
	})
}

// ============================================================================
// 缓存键生成辅助函数
// ============================================================================

// ComputeInputHash 计算输入哈希
//
// 📋 **参数**：
//   - inputs: 输入数据（可以是任意类型）
//
// 🔧 **返回值**：
//   - string: 输入哈希（十六进制字符串）
func ComputeInputHash(inputs interface{}) string {
	// 将输入转换为字节数组
	var inputBytes []byte

	switch v := inputs.(type) {
	case []byte:
		inputBytes = v
	case string:
		inputBytes = []byte(v)
	case []uint64:
		// 将uint64数组转换为字节数组
		inputBytes = make([]byte, len(v)*8)
		for i, val := range v {
			for j := 0; j < 8; j++ {
				inputBytes[i*8+j] = byte(val >> (j * 8))
			}
		}
	case [][]float64:
		// 将float64二维数组转换为字节数组
		// 简化实现：使用fmt.Sprintf序列化
		inputBytes = []byte(fmt.Sprintf("%v", v))
	default:
		// 通用序列化
		inputBytes = []byte(fmt.Sprintf("%v", v))
	}

	// 计算SHA-256哈希
	hash := sha256.Sum256(inputBytes)
	return hex.EncodeToString(hash[:])
}

// BuildCacheKey 构建缓存键
//
// 📋 **参数**：
//   - engineType: 引擎类型（"wasm"或"onnx"）
//   - contractID: 合约/模型标识符
//   - function: 函数名（WASM）或空（ONNX）
//   - inputs: 输入数据
//
// 🔧 **返回值**：
//   - *ExecutionCacheKey: 缓存键
func BuildCacheKey(engineType string, contractID string, function string, inputs interface{}) *ExecutionCacheKey {
	return &ExecutionCacheKey{
		EngineType: engineType,
		ContractID: contractID,
		Function:   function,
		InputHash:  ComputeInputHash(inputs),
	}
}

