package hostabi

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	pbresource "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	publicispc "github.com/weisyn/v1/pkg/interfaces/ispc"
)

// ============================================================================
// HostABI原语调用缓存
// ============================================================================
//
// 🎯 **目的**：
//   - 缓存只读原语的查询结果，避免重复查询
//   - 提升UTXO和资源查询的性能
//   - 减少对底层服务的调用次数
//
// 📋 **设计原则**：
//   - 仅缓存只读原语（查询类原语）
//   - 不缓存写操作原语（TxAddInput、TxAddAssetOutput等）
//   - 不缓存追踪原语（EmitEvent、LogDebug）
//   - 基于执行上下文的缓存作用域（同一执行上下文内共享缓存）
//
// ⚠️ **注意事项**：
//   - 缓存键需要考虑执行上下文（确保确定性）
//   - 缓存TTL应该较短（避免数据过期）
//   - 写操作应该使相关缓存失效
//
// ============================================================================

// PrimitiveCallCache 原语调用缓存
type PrimitiveCallCache struct {
	logger log.Logger

	// 缓存存储
	cache map[string]*CachedPrimitiveResult
	mu    sync.RWMutex

	// 缓存配置
	maxSize         int           // 最大缓存条目数
	defaultTTL      time.Duration // 默认生存时间
	cleanupInterval time.Duration // 清理间隔

	// 统计信息
	hits      uint64 // 缓存命中次数
	misses    uint64 // 缓存未命中次数
	evictions uint64 // 缓存驱逐次数

	// 清理控制
	stopCleanup chan struct{}
	cleanupOnce sync.Once
}

// CachedPrimitiveResult 缓存的原语调用结果
type CachedPrimitiveResult struct {
	Result      interface{} // 调用结果
	Error       error       // 调用错误（如果有）
	CachedAt    time.Time   // 缓存时间
	ExpiresAt   time.Time   // 过期时间
	AccessCount uint64      // 访问次数
	LastAccess  time.Time   // 最后访问时间
}

// NewPrimitiveCallCache 创建原语调用缓存
func NewPrimitiveCallCache(logger log.Logger, maxSize int, defaultTTL time.Duration) *PrimitiveCallCache {
	cache := &PrimitiveCallCache{
		logger:          logger,
		cache:           make(map[string]*CachedPrimitiveResult),
		maxSize:         maxSize,
		defaultTTL:      defaultTTL,
		cleanupInterval: defaultTTL / 2, // 清理间隔为TTL的一半
		stopCleanup:     make(chan struct{}),
	}

	// 启动后台清理goroutine
	go cache.cleanupExpiredEntries()

	return cache
}

// Get 获取缓存的调用结果
func (c *PrimitiveCallCache) Get(cacheKey string) (result interface{}, err error, found bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

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

// Set 设置缓存的调用结果
func (c *PrimitiveCallCache) Set(cacheKey string, result interface{}, err error, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ttl == 0 {
		ttl = c.defaultTTL
	}

	now := time.Now()

	// 如果缓存已满，执行LRU驱逐
	if len(c.cache) >= c.maxSize {
		c.evictLRU()
	}

	c.cache[cacheKey] = &CachedPrimitiveResult{
		Result:      result,
		Error:       err,
		CachedAt:    now,
		ExpiresAt:   now.Add(ttl),
		AccessCount: 0,
		LastAccess:  now,
	}
}

// Invalidate 使缓存失效（用于写操作后）
func (c *PrimitiveCallCache) Invalidate(pattern string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果pattern为空，清空所有缓存
	if pattern == "" {
		c.cache = make(map[string]*CachedPrimitiveResult)
		return
	}

	// 否则删除匹配的缓存条目
	for key := range c.cache {
		if len(key) >= len(pattern) && key[:len(pattern)] == pattern {
			delete(c.cache, key)
		}
	}
}

// Clear 清空缓存
func (c *PrimitiveCallCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]*CachedPrimitiveResult)
	c.hits = 0
	c.misses = 0
	c.evictions = 0
}

// GetStats 获取缓存统计信息
func (c *PrimitiveCallCache) GetStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	totalRequests := c.hits + c.misses
	hitRate := 0.0
	if totalRequests > 0 {
		hitRate = float64(c.hits) / float64(totalRequests) * 100
	}

	return map[string]interface{}{
		"size":           len(c.cache),
		"max_size":       c.maxSize,
		"hits":           c.hits,
		"misses":         c.misses,
		"hit_rate":       hitRate,
		"evictions":      c.evictions,
		"total_requests": totalRequests,
	}
}

// evictLRU 驱逐最近最少使用的缓存条目
func (c *PrimitiveCallCache) evictLRU() {
	if len(c.cache) == 0 {
		return
	}

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
func (c *PrimitiveCallCache) cleanupExpiredEntries() {
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
func (c *PrimitiveCallCache) cleanup() {
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
		c.logger.Debugf("清理了 %d 个过期的原语调用缓存条目", expiredCount)
	}
}

// Stop 停止缓存清理goroutine
func (c *PrimitiveCallCache) Stop() {
	c.cleanupOnce.Do(func() {
		close(c.stopCleanup)
	})
}

// ============================================================================
// 缓存键生成辅助函数
// ============================================================================

// buildPrimitiveCacheKey 构建原语调用缓存键
//
// 📋 **参数**：
//   - hashManager: 哈希管理器（用于计算参数哈希）
//   - executionID: 执行上下文ID（确保缓存作用域）
//   - primitiveName: 原语名称
//   - params: 调用参数
//
// 🔧 **返回值**：
//   - string: 缓存键
func buildPrimitiveCacheKey(hashManager crypto.HashManager, executionID string, primitiveName string, params interface{}) string {
	// 计算参数哈希
	var paramHash string
	if params != nil {
		paramBytes := []byte(fmt.Sprintf("%v", params))
		hash := hashManager.SHA256(paramBytes)
		paramHash = hex.EncodeToString(hash)
	} else {
		paramHash = "nil"
	}

	return fmt.Sprintf("%s:%s:%s", executionID, primitiveName, paramHash)
}

// ============================================================================
// 带缓存功能的HostABI包装器
// ============================================================================

// HostRuntimePortsWithCache 带缓存功能的HostABI实现包装器
type HostRuntimePortsWithCache struct {
	publicispc.HostABI
	cache       *PrimitiveCallCache
	executionID string
	logger      log.Logger
	hashManager crypto.HashManager // 哈希管理器（用于构建缓存键）
}

// NewHostRuntimePortsWithCache 创建带缓存功能的HostABI包装器
func NewHostRuntimePortsWithCache(
	hostABI publicispc.HostABI,
	cache *PrimitiveCallCache,
	executionID string,
	logger log.Logger,
	hashManager crypto.HashManager,
) *HostRuntimePortsWithCache {
	return &HostRuntimePortsWithCache{
		HostABI:     hostABI,
		cache:       cache,
		executionID: executionID,
		logger:      logger,
		hashManager: hashManager,
	}
}

// GetCacheStats 获取缓存统计信息
func (w *HostRuntimePortsWithCache) GetCacheStats() map[string]interface{} {
	if w.cache == nil {
		return nil
	}
	return w.cache.GetStats()
}

// ClearCache 清空缓存
func (w *HostRuntimePortsWithCache) ClearCache() {
	if w.cache != nil {
		w.cache.Clear()
	}
}

// InvalidateCache 使缓存失效
func (w *HostRuntimePortsWithCache) InvalidateCache(pattern string) {
	if w.cache != nil {
		w.cache.Invalidate(pattern)
	}
}

// 包装只读原语，添加缓存功能

// 类别 A：确定性区块视图（4个）- 只读原语，可以缓存
func (w *HostRuntimePortsWithCache) GetBlockHeight(ctx context.Context) (uint64, error) {
	cacheKey := buildPrimitiveCacheKey(w.hashManager, w.executionID, "GetBlockHeight", nil)

	// 尝试从缓存获取
	if cachedResult, cachedErr, found := w.cache.Get(cacheKey); found {
		if w.logger != nil {
			w.logger.Debug("✅ GetBlockHeight缓存命中")
		}
		if cachedErr != nil {
			return 0, cachedErr
		}
		if result, ok := cachedResult.(uint64); ok {
			return result, nil
		}
	}

	// 调用原始方法
	result, err := w.HostABI.GetBlockHeight(ctx)

	// 缓存结果（仅缓存成功的结果）
	if err == nil {
		w.cache.Set(cacheKey, result, nil, 0) // 使用默认TTL
	}

	return result, err
}

func (w *HostRuntimePortsWithCache) GetBlockTimestamp(ctx context.Context) (uint64, error) {
	cacheKey := buildPrimitiveCacheKey(w.hashManager, w.executionID, "GetBlockTimestamp", nil)

	if cachedResult, cachedErr, found := w.cache.Get(cacheKey); found {
		if w.logger != nil {
			w.logger.Debug("✅ GetBlockTimestamp缓存命中")
		}
		if cachedErr != nil {
			return 0, cachedErr
		}
		if result, ok := cachedResult.(uint64); ok {
			return result, nil
		}
	}

	result, err := w.HostABI.GetBlockTimestamp(ctx)
	if err == nil {
		w.cache.Set(cacheKey, result, nil, 0)
	}

	return result, err
}

func (w *HostRuntimePortsWithCache) GetBlockHash(ctx context.Context, height uint64) ([]byte, error) {
	cacheKey := buildPrimitiveCacheKey(w.hashManager, w.executionID, "GetBlockHash", height)

	if cachedResult, cachedErr, found := w.cache.Get(cacheKey); found {
		if w.logger != nil {
			w.logger.Debugf("✅ GetBlockHash缓存命中: height=%d", height)
		}
		if cachedErr != nil {
			return nil, cachedErr
		}
		if result, ok := cachedResult.([]byte); ok {
			return result, nil
		}
	}

	result, err := w.HostABI.GetBlockHash(ctx, height)
	if err == nil {
		w.cache.Set(cacheKey, result, nil, 0)
	}

	return result, err
}

func (w *HostRuntimePortsWithCache) GetChainID(ctx context.Context) ([]byte, error) {
	cacheKey := buildPrimitiveCacheKey(w.hashManager, w.executionID, "GetChainID", nil)

	if cachedResult, cachedErr, found := w.cache.Get(cacheKey); found {
		if w.logger != nil {
			w.logger.Debug("✅ GetChainID缓存命中")
		}
		if cachedErr != nil {
			return nil, cachedErr
		}
		if result, ok := cachedResult.([]byte); ok {
			return result, nil
		}
	}

	result, err := w.HostABI.GetChainID(ctx)
	if err == nil {
		w.cache.Set(cacheKey, result, nil, 0)
	}

	return result, err
}

// 类别 B：执行上下文（3个）- 只读原语，可以缓存
func (w *HostRuntimePortsWithCache) GetCaller(ctx context.Context) ([]byte, error) {
	cacheKey := buildPrimitiveCacheKey(w.hashManager, w.executionID, "GetCaller", nil)

	if cachedResult, cachedErr, found := w.cache.Get(cacheKey); found {
		if cachedErr != nil {
			return nil, cachedErr
		}
		if result, ok := cachedResult.([]byte); ok {
			return result, nil
		}
	}

	result, err := w.HostABI.GetCaller(ctx)
	if err == nil {
		w.cache.Set(cacheKey, result, nil, 0)
	}

	return result, err
}

func (w *HostRuntimePortsWithCache) GetContractAddress(ctx context.Context) ([]byte, error) {
	cacheKey := buildPrimitiveCacheKey(w.hashManager, w.executionID, "GetContractAddress", nil)

	if cachedResult, cachedErr, found := w.cache.Get(cacheKey); found {
		if cachedErr != nil {
			return nil, cachedErr
		}
		if result, ok := cachedResult.([]byte); ok {
			return result, nil
		}
	}

	result, err := w.HostABI.GetContractAddress(ctx)
	if err == nil {
		w.cache.Set(cacheKey, result, nil, 0)
	}

	return result, err
}

func (w *HostRuntimePortsWithCache) GetTransactionID(ctx context.Context) ([]byte, error) {
	cacheKey := buildPrimitiveCacheKey(w.hashManager, w.executionID, "GetTransactionID", nil)

	if cachedResult, cachedErr, found := w.cache.Get(cacheKey); found {
		if cachedErr != nil {
			return nil, cachedErr
		}
		if result, ok := cachedResult.([]byte); ok {
			return result, nil
		}
	}

	result, err := w.HostABI.GetTransactionID(ctx)
	if err == nil {
		w.cache.Set(cacheKey, result, nil, 0)
	}

	return result, err
}

// 类别 C：UTXO查询（2个）- 只读原语，可以缓存
func (w *HostRuntimePortsWithCache) UTXOLookup(ctx context.Context, outpoint *pb.OutPoint) (*pb.TxOutput, error) {
	if outpoint == nil {
		return nil, fmt.Errorf("outpoint 不能为 nil")
	}

	cacheKey := buildPrimitiveCacheKey(w.hashManager, w.executionID, "UTXOLookup", fmt.Sprintf("%x:%d", outpoint.TxId, outpoint.OutputIndex))

	if cachedResult, cachedErr, found := w.cache.Get(cacheKey); found {
		if w.logger != nil {
			w.logger.Debugf("✅ UTXOLookup缓存命中: txId=%x index=%d", outpoint.TxId[:8], outpoint.OutputIndex)
		}
		if cachedErr != nil {
			return nil, cachedErr
		}
		if result, ok := cachedResult.(*pb.TxOutput); ok {
			return result, nil
		}
	}

	result, err := w.HostABI.UTXOLookup(ctx, outpoint)
	if err == nil {
		w.cache.Set(cacheKey, result, nil, 0)
	}

	return result, err
}

func (w *HostRuntimePortsWithCache) UTXOExists(ctx context.Context, outpoint *pb.OutPoint) (bool, error) {
	if outpoint == nil {
		return false, fmt.Errorf("outpoint 不能为 nil")
	}

	cacheKey := buildPrimitiveCacheKey(w.hashManager, w.executionID, "UTXOExists", fmt.Sprintf("%x:%d", outpoint.TxId, outpoint.OutputIndex))

	if cachedResult, cachedErr, found := w.cache.Get(cacheKey); found {
		if cachedErr != nil {
			return false, cachedErr
		}
		if result, ok := cachedResult.(bool); ok {
			return result, nil
		}
	}

	result, err := w.HostABI.UTXOExists(ctx, outpoint)
	if err == nil {
		w.cache.Set(cacheKey, result, nil, 0)
	}

	return result, err
}

// 类别 D：资源查询（2个）- 只读原语，可以缓存
func (w *HostRuntimePortsWithCache) ResourceLookup(ctx context.Context, contentHash []byte) (*pbresource.Resource, error) {
	if len(contentHash) != 32 {
		return nil, fmt.Errorf("contentHash 必须是 32 字节")
	}

	cacheKey := buildPrimitiveCacheKey(w.hashManager, w.executionID, "ResourceLookup", hex.EncodeToString(contentHash))

	if cachedResult, cachedErr, found := w.cache.Get(cacheKey); found {
		if w.logger != nil {
			w.logger.Debugf("✅ ResourceLookup缓存命中: contentHash=%x", contentHash[:8])
		}
		if cachedErr != nil {
			return nil, cachedErr
		}
		if result, ok := cachedResult.(*pbresource.Resource); ok {
			return result, nil
		}
	}

	result, err := w.HostABI.ResourceLookup(ctx, contentHash)
	if err == nil {
		w.cache.Set(cacheKey, result, nil, 0)
	}

	return result, err
}

func (w *HostRuntimePortsWithCache) ResourceExists(ctx context.Context, contentHash []byte) (bool, error) {
	if len(contentHash) != 32 {
		return false, fmt.Errorf("contentHash 必须是 32 字节")
	}

	cacheKey := buildPrimitiveCacheKey(w.hashManager, w.executionID, "ResourceExists", hex.EncodeToString(contentHash))

	if cachedResult, cachedErr, found := w.cache.Get(cacheKey); found {
		if cachedErr != nil {
			return false, cachedErr
		}
		if result, ok := cachedResult.(bool); ok {
			return result, nil
		}
	}

	result, err := w.HostABI.ResourceExists(ctx, contentHash)
	if err == nil {
		w.cache.Set(cacheKey, result, nil, 0)
	}

	return result, err
}

// 类别 E：交易草稿构建（4个）- 写操作原语，不缓存，但使相关缓存失效
func (w *HostRuntimePortsWithCache) TxAddInput(ctx context.Context, outpoint *pb.OutPoint, isReferenceOnly bool, unlockingProof *pb.UnlockingProof) (uint32, error) {
	result, err := w.HostABI.TxAddInput(ctx, outpoint, isReferenceOnly, unlockingProof)

	// 写操作后使UTXO相关缓存失效
	if err == nil && outpoint != nil {
		w.cache.Invalidate(fmt.Sprintf("%s:UTXO", w.executionID))
	}

	return result, err
}

func (w *HostRuntimePortsWithCache) TxAddAssetOutput(ctx context.Context, owner []byte, amount uint64, tokenID []byte, lockingConditions []*pb.LockingCondition) (uint32, error) {
	return w.HostABI.TxAddAssetOutput(ctx, owner, amount, tokenID, lockingConditions)
}

func (w *HostRuntimePortsWithCache) TxAddResourceOutput(ctx context.Context, contentHash []byte, category string, owner []byte, lockingConditions []*pb.LockingCondition, metadata []byte) (uint32, error) {
	return w.HostABI.TxAddResourceOutput(ctx, contentHash, category, owner, lockingConditions, metadata)
}

func (w *HostRuntimePortsWithCache) TxAddStateOutput(ctx context.Context, stateID []byte, stateVersion uint64, executionResultHash []byte, publicInputs []byte, parentStateHash []byte) (uint32, error) {
	return w.HostABI.TxAddStateOutput(ctx, stateID, stateVersion, executionResultHash, publicInputs, parentStateHash)
}

// 类别 G：执行追踪（2个）- 追踪原语，不缓存
func (w *HostRuntimePortsWithCache) EmitEvent(ctx context.Context, eventType string, eventData []byte) error {
	return w.HostABI.EmitEvent(ctx, eventType, eventData)
}

func (w *HostRuntimePortsWithCache) LogDebug(ctx context.Context, message string) error {
	return w.HostABI.LogDebug(ctx, message)
}

// 确保实现接口
var _ publicispc.HostABI = (*HostRuntimePortsWithCache)(nil)
