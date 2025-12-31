// Package difficulty 提供区块难度和时间戳规则验证
package difficulty

import (
	"context"
	"sort"
	"sync"

	core "github.com/weisyn/v1/pb/blockchain/block"
)

const (
	// DefaultMTPCacheCapacity 默认MTP缓存容量
	// 支持约10000个区块高度的MTP缓存，足够覆盖大多数同步场景
	DefaultMTPCacheCapacity = 10000
)

// MTPCache 提供高效的 Median Time Past 缓存服务
//
// 设计目标：
// 1. 避免重复计算：对于相同高度的MTP查询，直接返回缓存值
// 2. 减少IO压力：缓存命中时无需访问数据库
// 3. 线程安全：支持并发读写
// 4. 内存可控：使用LRU策略限制缓存大小
type MTPCache struct {
	// cache 存储 height -> mtp 映射
	// 使用简单的map实现，配合LRU淘汰策略
	cache map[uint64]uint64

	// accessOrder 记录访问顺序，用于LRU淘汰
	accessOrder []uint64

	// capacity 缓存容量
	capacity int

	// mu 保护并发访问
	mu sync.RWMutex

	// hits 和 misses 用于统计缓存命中率
	hits   uint64
	misses uint64
}

// NewMTPCache 创建新的MTP缓存实例
//
// 参数：
//   - capacity: 缓存容量，建议 >= 1000
//
// 返回：
//   - *MTPCache: 缓存实例
func NewMTPCache(capacity int) *MTPCache {
	if capacity <= 0 {
		capacity = DefaultMTPCacheCapacity
	}
	return &MTPCache{
		cache:       make(map[uint64]uint64, capacity),
		accessOrder: make([]uint64, 0, capacity),
		capacity:    capacity,
	}
}

// Get 从缓存中获取MTP值
//
// 参数：
//   - height: 区块高度
//
// 返回：
//   - mtp: MTP时间戳
//   - ok: 是否命中缓存
func (c *MTPCache) Get(height uint64) (uint64, bool) {
	c.mu.RLock()
	mtp, ok := c.cache[height]
	c.mu.RUnlock()

	if ok {
		// 更新访问顺序（需要写锁）
		c.mu.Lock()
		c.hits++
		c.updateAccessOrder(height)
		c.mu.Unlock()
	} else {
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
	}

	return mtp, ok
}

// Set 将MTP值存入缓存
//
// 参数：
//   - height: 区块高度
//   - mtp: MTP时间戳
func (c *MTPCache) Set(height uint64, mtp uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果已存在，只更新访问顺序
	if _, exists := c.cache[height]; exists {
		c.cache[height] = mtp
		c.updateAccessOrder(height)
		return
	}

	// 检查是否需要淘汰
	if len(c.cache) >= c.capacity {
		c.evictLRU()
	}

	// 添加新条目
	c.cache[height] = mtp
	c.accessOrder = append(c.accessOrder, height)
}

// updateAccessOrder 更新访问顺序（将height移到末尾）
// 调用者必须持有写锁
func (c *MTPCache) updateAccessOrder(height uint64) {
	// 找到并移除旧位置
	for i, h := range c.accessOrder {
		if h == height {
			c.accessOrder = append(c.accessOrder[:i], c.accessOrder[i+1:]...)
			break
		}
	}
	// 添加到末尾（最近访问）
	c.accessOrder = append(c.accessOrder, height)
}

// evictLRU 淘汰最久未使用的条目
// 调用者必须持有写锁
func (c *MTPCache) evictLRU() {
	if len(c.accessOrder) == 0 {
		return
	}
	// 淘汰最旧的条目（索引0）
	oldest := c.accessOrder[0]
	c.accessOrder = c.accessOrder[1:]
	delete(c.cache, oldest)
}

// ComputeAndCache 计算MTP并缓存结果
//
// 优先查缓存，未命中时计算并缓存
//
// 参数：
//   - ctx: 上下文
//   - q: 区块高度读取器
//   - height: 目标高度（计算该高度的MTP）
//   - window: MTP窗口大小（通常为11）
//
// 返回：
//   - mtp: MTP时间戳
//   - error: 计算错误
func (c *MTPCache) ComputeAndCache(ctx context.Context, q BlockHeightReader, height uint64, window uint64) (uint64, error) {
	// 1. 先查缓存
	if mtp, ok := c.Get(height); ok {
		return mtp, nil
	}

	// 2. 计算MTP
	mtp, err := computeMTPWithTimestampSlice(ctx, q, height, window)
	if err != nil {
		return 0, err
	}

	// 3. 缓存结果
	c.Set(height, mtp)

	return mtp, nil
}

// ComputeAndCacheBatch 批量计算并缓存MTP
//
// 对于批量同步场景，可以一次性计算多个高度的MTP
//
// 参数：
//   - ctx: 上下文
//   - q: 区块高度读取器或批量读取器
//   - heights: 需要计算MTP的高度列表
//   - window: MTP窗口大小
//
// 返回：
//   - map[uint64]uint64: height -> mtp 映射
//   - error: 计算错误
func (c *MTPCache) ComputeAndCacheBatch(ctx context.Context, q interface{}, heights []uint64, window uint64) (map[uint64]uint64, error) {
	result := make(map[uint64]uint64, len(heights))

	// 分离已缓存和未缓存的高度
	var uncached []uint64
	for _, h := range heights {
		if mtp, ok := c.Get(h); ok {
			result[h] = mtp
		} else {
			uncached = append(uncached, h)
		}
	}

	// 如果全部命中缓存，直接返回
	if len(uncached) == 0 {
		return result, nil
	}

	// 尝试批量读取（如果支持）
	if batchReader, ok := q.(BlockHeaderBatchReader); ok {
		return c.computeBatchWithBatchReader(ctx, batchReader, uncached, window, result)
	}

	// 回退到逐个计算
	if heightReader, ok := q.(BlockHeightReader); ok {
		for _, h := range uncached {
			mtp, err := c.ComputeAndCache(ctx, heightReader, h, window)
			if err != nil {
				return nil, err
			}
			result[h] = mtp
		}
		return result, nil
	}

	return nil, ErrInvalidReader
}

// computeBatchWithBatchReader 使用批量读取器计算MTP
func (c *MTPCache) computeBatchWithBatchReader(ctx context.Context, q BlockHeaderBatchReader, heights []uint64, window uint64, result map[uint64]uint64) (map[uint64]uint64, error) {
	if len(heights) == 0 {
		return result, nil
	}

	// 计算需要读取的区块范围
	sort.Slice(heights, func(i, j int) bool { return heights[i] < heights[j] })
	minHeight := heights[0]
	maxHeight := heights[len(heights)-1]

	// 计算实际需要的起始高度（考虑窗口）
	var start uint64
	if minHeight+1 > window {
		start = minHeight + 1 - window
	} else {
		start = 0
	}

	// 批量读取所有需要的区块头
	headers, err := q.GetBlockHeadersByHeightRange(ctx, start, maxHeight)
	if err != nil {
		return nil, err
	}

	// 构建 height -> timestamp 映射
	timestamps := make(map[uint64]uint64, len(headers))
	for _, h := range headers {
		if h != nil {
			timestamps[h.Height] = h.Timestamp
		}
	}

	// 计算每个高度的MTP
	for _, h := range heights {
		mtp, err := computeMTPFromTimestampMap(timestamps, h, window)
		if err != nil {
			return nil, err
		}
		c.Set(h, mtp)
		result[h] = mtp
	}

	return result, nil
}

// computeMTPFromTimestampMap 从时间戳映射计算MTP
func computeMTPFromTimestampMap(timestamps map[uint64]uint64, endHeight uint64, window uint64) (uint64, error) {
	if window == 0 {
		return 0, ErrInvalidWindow
	}

	var start uint64
	if endHeight+1 > window {
		start = endHeight + 1 - window
	} else {
		start = 0
	}

	tsSlice := make([]uint64, 0, endHeight-start+1)
	for h := start; h <= endHeight; h++ {
		ts, ok := timestamps[h]
		if !ok {
			return 0, ErrMissingTimestamp
		}
		tsSlice = append(tsSlice, ts)
	}

	sort.Slice(tsSlice, func(i, j int) bool { return tsSlice[i] < tsSlice[j] })
	return tsSlice[len(tsSlice)/2], nil
}

// computeMTPWithTimestampSlice 使用逐个读取的方式计算MTP
func computeMTPWithTimestampSlice(ctx context.Context, q BlockHeightReader, endHeight uint64, window uint64) (uint64, error) {
	if window == 0 {
		return 0, ErrInvalidWindow
	}

	var start uint64
	if endHeight+1 > window {
		start = endHeight + 1 - window
	} else {
		start = 0
	}

	timestamps := make([]uint64, 0, endHeight-start+1)
	for h := start; h <= endHeight; h++ {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		b, err := q.GetBlockByHeight(ctx, h)
		if err != nil {
			return 0, err
		}
		if b == nil || b.Header == nil {
			return 0, ErrNilBlock
		}
		timestamps = append(timestamps, b.Header.Timestamp)
	}

	sort.Slice(timestamps, func(i, j int) bool { return timestamps[i] < timestamps[j] })
	return timestamps[len(timestamps)/2], nil
}

// Stats 返回缓存统计信息
func (c *MTPCache) Stats() (size int, capacity int, hits uint64, misses uint64, hitRate float64) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	size = len(c.cache)
	capacity = c.capacity
	hits = c.hits
	misses = c.misses

	total := hits + misses
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}
	return
}

// Clear 清空缓存
func (c *MTPCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[uint64]uint64, c.capacity)
	c.accessOrder = make([]uint64, 0, c.capacity)
	c.hits = 0
	c.misses = 0
}

// Invalidate 使指定高度的缓存失效
//
// 用于链重组时清除受影响的缓存
func (c *MTPCache) Invalidate(height uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.cache, height)
	// 从访问顺序中移除
	for i, h := range c.accessOrder {
		if h == height {
			c.accessOrder = append(c.accessOrder[:i], c.accessOrder[i+1:]...)
			break
		}
	}
}

// InvalidateAbove 使指定高度以上的所有缓存失效
//
// 用于链重组时批量清除受影响的缓存
func (c *MTPCache) InvalidateAbove(height uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 收集需要删除的高度
	var toDelete []uint64
	for h := range c.cache {
		if h > height {
			toDelete = append(toDelete, h)
		}
	}

	// 删除
	for _, h := range toDelete {
		delete(c.cache, h)
	}

	// 重建访问顺序
	newOrder := make([]uint64, 0, len(c.accessOrder))
	for _, h := range c.accessOrder {
		if h <= height {
			newOrder = append(newOrder, h)
		}
	}
	c.accessOrder = newOrder
}

// BlockHeaderBatchReader 批量区块头读取接口
//
// 用于优化MTP计算，减少数据库调用次数
type BlockHeaderBatchReader interface {
	// GetBlockHeadersByHeightRange 批量读取指定高度范围的区块头
	//
	// 参数：
	//   - ctx: 上下文
	//   - start: 起始高度（包含）
	//   - end: 结束高度（包含）
	//
	// 返回：
	//   - []*core.BlockHeader: 区块头列表（按高度排序）
	//   - error: 读取错误
	GetBlockHeadersByHeightRange(ctx context.Context, start, end uint64) ([]*core.BlockHeader, error)
}

// 错误定义
var (
	ErrInvalidWindow    = errorf("window must be >= 1")
	ErrNilBlock         = errorf("block/header is nil")
	ErrMissingTimestamp = errorf("missing timestamp for height")
	ErrInvalidReader    = errorf("invalid reader: must implement BlockHeightReader or BlockHeaderBatchReader")
)

// errorf 创建一个简单的错误
func errorf(msg string) error {
	return &mtpError{msg: msg}
}

type mtpError struct {
	msg string
}

func (e *mtpError) Error() string {
	return e.msg
}

// GlobalMTPCache 全局MTP缓存实例
//
// 供整个节点共享使用，避免重复创建缓存
var GlobalMTPCache = NewMTPCache(DefaultMTPCacheCapacity)

