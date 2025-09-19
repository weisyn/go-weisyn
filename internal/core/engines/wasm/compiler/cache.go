package compiler

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// CompiledModuleCache 编译缓存接口
// 负责缓存已编译的 WASM 模块，避免重复编译
type CompiledModuleCache interface {
	// Get 获取缓存项
	Get(key string) (interface{}, bool)
	// Set 设置缓存项
	Set(key string, module interface{}, ttlSeconds int) bool
	// Remove 移除缓存项
	Remove(key string) bool
	// Clear 清空所有缓存
	Clear() int
	// Stats 获取缓存统计信息
	Stats() *CacheStats
}

// CacheStats 缓存统计信息
type CacheStats struct {
	Hits        int64     `json:"hits"`
	Misses      int64     `json:"misses"`
	Entries     int       `json:"entries"`
	MemoryUsage int64     `json:"memory_usage"`
	LastReset   time.Time `json:"last_reset"`
}

// CacheEntry 缓存条目
type CacheEntry struct {
	Module      interface{}
	CreatedAt   time.Time
	LastUsed    time.Time
	AccessCount int64
	TTL         time.Duration
}

// WASMModuleCache WASM 模块缓存实现
type WASMModuleCache struct {
	entries         map[string]*CacheEntry
	mutex           sync.RWMutex
	stats           *CacheStats
	maxSize         int
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
}

// NewWASMModuleCache 创建新的模块缓存
func NewWASMModuleCache(maxSize int, cleanupInterval time.Duration) *WASMModuleCache {
	cache := &WASMModuleCache{
		entries:         make(map[string]*CacheEntry),
		stats:           &CacheStats{LastReset: time.Now()},
		maxSize:         maxSize,
		cleanupInterval: cleanupInterval,
		stopCleanup:     make(chan struct{}),
	}

	// 启动清理 goroutine
	go cache.startCleanupRoutine()

	return cache
}

// Get 获取缓存项
func (c *WASMModuleCache) Get(key string) (interface{}, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		c.stats.Misses++
		return nil, false
	}

	// 检查是否过期
	if entry.TTL > 0 && time.Since(entry.CreatedAt) > entry.TTL {
		c.mutex.RUnlock()
		c.mutex.Lock()
		delete(c.entries, key)
		c.mutex.Unlock()
		c.mutex.RLock()
		c.stats.Misses++
		return nil, false
	}

	// 更新访问统计
	entry.LastUsed = time.Now()
	entry.AccessCount++
	c.stats.Hits++

	return entry.Module, true
}

// Set 设置缓存项
func (c *WASMModuleCache) Set(key string, module interface{}, ttlSeconds int) bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// 检查容量限制
	if c.maxSize > 0 && len(c.entries) >= c.maxSize {
		c.evictLRU()
	}

	now := time.Now()
	entry := &CacheEntry{
		Module:      module,
		CreatedAt:   now,
		LastUsed:    now,
		AccessCount: 0,
	}

	if ttlSeconds > 0 {
		entry.TTL = time.Duration(ttlSeconds) * time.Second
	}

	c.entries[key] = entry
	c.stats.Entries = len(c.entries)

	return true
}

// Remove 移除缓存项
func (c *WASMModuleCache) Remove(key string) bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	_, exists := c.entries[key]
	if exists {
		delete(c.entries, key)
		c.stats.Entries = len(c.entries)
	}

	return exists
}

// Clear 清空所有缓存
func (c *WASMModuleCache) Clear() int {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	count := len(c.entries)
	c.entries = make(map[string]*CacheEntry)
	c.stats.Entries = 0
	c.stats.Hits = 0
	c.stats.Misses = 0
	c.stats.LastReset = time.Now()

	return count
}

// Stats 获取缓存统计信息
func (c *WASMModuleCache) Stats() *CacheStats {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	stats := *c.stats // 复制统计数据
	stats.Entries = len(c.entries)

	// 计算内存使用估算
	stats.MemoryUsage = int64(len(c.entries) * 1024) // 粗略估算

	return &stats
}

// evictLRU 淘汰最近最少使用的条目
func (c *WASMModuleCache) evictLRU() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.entries {
		if oldestKey == "" || entry.LastUsed.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.LastUsed
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

// startCleanupRoutine 启动清理例程
func (c *WASMModuleCache) startCleanupRoutine() {
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

// cleanup 清理过期条目
func (c *WASMModuleCache) cleanup() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now()
	for key, entry := range c.entries {
		if entry.TTL > 0 && now.Sub(entry.CreatedAt) > entry.TTL {
			delete(c.entries, key)
		}
	}

	c.stats.Entries = len(c.entries)
}

// Close 关闭缓存
func (c *WASMModuleCache) Close() {
	close(c.stopCleanup)
	c.Clear()
}

// GenerateModuleKey 生成模块缓存键
func GenerateModuleKey(bytecode []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(bytecode))
}

// Validator 字节码验证器接口
type Validator interface {
	// Validate 验证 WASM 字节码
	Validate(wasmBytes []byte) error
	// ValidateModule 验证已编译模块
	ValidateModule(module interface{}) error
}

// BasicValidator 基础验证器实现
type BasicValidator struct{}

// NewBasicValidator 创建基础验证器
func NewBasicValidator() *BasicValidator {
	return &BasicValidator{}
}

// Validate 验证 WASM 字节码
func (v *BasicValidator) Validate(wasmBytes []byte) error {
	if len(wasmBytes) < 8 {
		return fmt.Errorf("invalid wasm: too short")
	}

	// 检查 WASM 魔数
	if string(wasmBytes[:4]) != "\x00asm" {
		return fmt.Errorf("invalid wasm: bad magic number")
	}

	// 检查版本号
	version := uint32(wasmBytes[4]) | uint32(wasmBytes[5])<<8 |
		uint32(wasmBytes[6])<<16 | uint32(wasmBytes[7])<<24
	if version != 1 {
		return fmt.Errorf("unsupported wasm version: %d", version)
	}

	return nil
}

// ValidateModule 验证已编译模块
func (v *BasicValidator) ValidateModule(module interface{}) error {
	if module == nil {
		return fmt.Errorf("module is nil")
	}
	return nil
}

// Optimizer 优化器接口
type Optimizer interface {
	// Optimize 优化 WASM 字节码
	Optimize(ctx context.Context, wasmBytes []byte) ([]byte, error)
	// GetOptimizationLevel 获取优化级别
	GetOptimizationLevel() int
}

// BasicOptimizer 基础优化器实现
type BasicOptimizer struct {
	level int
}

// NewBasicOptimizer 创建基础优化器
func NewBasicOptimizer(level int) *BasicOptimizer {
	return &BasicOptimizer{level: level}
}

// Optimize 优化 WASM 字节码（当前为透传实现）
func (o *BasicOptimizer) Optimize(ctx context.Context, wasmBytes []byte) ([]byte, error) {
	// TODO: 实现具体的优化逻辑
	// 当前直接返回原始字节码
	return wasmBytes, nil
}

// GetOptimizationLevel 获取优化级别
func (o *BasicOptimizer) GetOptimizationLevel() int {
	return o.level
}
