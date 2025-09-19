package runtime

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// InstancePool 实例池接口
// 为避免重复创建实例，提供对象池功能
// 支持池大小控制与自动清理
type InstancePool interface {
	// Get 从池中获取实例
	Get(moduleID string) (interface{}, bool)
	// Put 归还实例到池中
	Put(moduleID string, instance interface{}) bool
	// Evict 淘汰特定模块的所有实例
	Evict(moduleID string) int
	// Cleanup 清理过期或空闲实例
	Cleanup() int
	// Stats 获取池统计信息
	Stats() *PoolStats
	// Close 关闭实例池
	Close()
}

// PoolStats 实例池统计信息
type PoolStats struct {
	TotalInstances     int       `json:"total_instances"`
	ActiveInstances    int       `json:"active_instances"`
	IdleInstances      int       `json:"idle_instances"`
	Hits               int64     `json:"hits"`
	Misses             int64     `json:"misses"`
	CreatedInstances   int64     `json:"created_instances"`
	DestroyedInstances int64     `json:"destroyed_instances"`
	LastCleanup        time.Time `json:"last_cleanup"`
}

// PoolEntry 池中的实例条目
type PoolEntry struct {
	Instance   interface{}
	CreatedAt  time.Time
	LastUsed   time.Time
	UsageCount int64
	ModuleID   string
}

// WASMInstancePool WASM 实例池实现
type WASMInstancePool struct {
	pools           map[string][]*PoolEntry // 按模块分组的实例池
	mutex           sync.RWMutex
	stats           *PoolStats
	maxPoolSize     int
	maxIdleTime     time.Duration
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
}

// NewWASMInstancePool 创建新的实例池
func NewWASMInstancePool(maxPoolSize int, maxIdleTime, cleanupInterval time.Duration) *WASMInstancePool {
	pool := &WASMInstancePool{
		pools:           make(map[string][]*PoolEntry),
		stats:           &PoolStats{LastCleanup: time.Now()},
		maxPoolSize:     maxPoolSize,
		maxIdleTime:     maxIdleTime,
		cleanupInterval: cleanupInterval,
		stopCleanup:     make(chan struct{}),
	}

	// 启动清理 goroutine
	go pool.startCleanupRoutine()

	return pool
}

// Get 从池中获取实例
func (p *WASMInstancePool) Get(moduleID string) (interface{}, bool) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	poolEntries, exists := p.pools[moduleID]
	if !exists || len(poolEntries) == 0 {
		p.stats.Misses++
		return nil, false
	}

	// 获取最后一个实例（栈式获取）
	entry := poolEntries[len(poolEntries)-1]
	p.pools[moduleID] = poolEntries[:len(poolEntries)-1]

	// 更新统计
	entry.LastUsed = time.Now()
	entry.UsageCount++
	p.stats.Hits++
	p.stats.ActiveInstances++
	p.stats.IdleInstances--

	return entry.Instance, true
}

// Put 归还实例到池中
func (p *WASMInstancePool) Put(moduleID string, instance interface{}) bool {
	if instance == nil {
		return false
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	// 检查池大小限制
	poolEntries := p.pools[moduleID]
	if len(poolEntries) >= p.maxPoolSize {
		// 池已满，销毁实例
		if closer, ok := instance.(interface{ Close(context.Context) error }); ok {
			closer.Close(context.Background())
		}
		p.stats.DestroyedInstances++
		return false
	}

	// 创建池条目
	entry := &PoolEntry{
		Instance:   instance,
		CreatedAt:  time.Now(),
		LastUsed:   time.Now(),
		UsageCount: 0,
		ModuleID:   moduleID,
	}

	// 添加到池中
	p.pools[moduleID] = append(poolEntries, entry)
	p.stats.ActiveInstances--
	p.stats.IdleInstances++

	return true
}

// Evict 淘汰特定模块的所有实例
func (p *WASMInstancePool) Evict(moduleID string) int {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	poolEntries, exists := p.pools[moduleID]
	if !exists {
		return 0
	}

	count := len(poolEntries)
	for _, entry := range poolEntries {
		if closer, ok := entry.Instance.(interface{ Close(context.Context) error }); ok {
			closer.Close(context.Background())
		}
		p.stats.DestroyedInstances++
	}

	delete(p.pools, moduleID)
	p.stats.IdleInstances -= count

	return count
}

// Cleanup 清理过期或空闲实例
func (p *WASMInstancePool) Cleanup() int {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	cleanupCount := 0
	now := time.Now()

	for moduleID, poolEntries := range p.pools {
		var remainingEntries []*PoolEntry

		for _, entry := range poolEntries {
			// 检查是否空闲太久
			if now.Sub(entry.LastUsed) > p.maxIdleTime {
				if closer, ok := entry.Instance.(interface{ Close(context.Context) error }); ok {
					closer.Close(context.Background())
				}
				cleanupCount++
				p.stats.DestroyedInstances++
			} else {
				remainingEntries = append(remainingEntries, entry)
			}
		}

		if len(remainingEntries) == 0 {
			delete(p.pools, moduleID)
		} else {
			p.pools[moduleID] = remainingEntries
		}
	}

	p.stats.IdleInstances -= cleanupCount
	p.stats.LastCleanup = now

	return cleanupCount
}

// Stats 获取池统计信息
func (p *WASMInstancePool) Stats() *PoolStats {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	stats := *p.stats // 复制统计数据

	// 重新计算实例数量
	totalInstances := 0
	for _, poolEntries := range p.pools {
		totalInstances += len(poolEntries)
	}
	stats.TotalInstances = totalInstances
	stats.IdleInstances = totalInstances

	return &stats
}

// Close 关闭实例池
func (p *WASMInstancePool) Close() {
	close(p.stopCleanup)

	p.mutex.Lock()
	defer p.mutex.Unlock()

	// 关闭所有实例
	for moduleID, poolEntries := range p.pools {
		for _, entry := range poolEntries {
			if closer, ok := entry.Instance.(interface{ Close(context.Context) error }); ok {
				closer.Close(context.Background())
			}
		}
		delete(p.pools, moduleID)
	}

	p.stats.TotalInstances = 0
	p.stats.ActiveInstances = 0
	p.stats.IdleInstances = 0
}

// startCleanupRoutine 启动清理例程
func (p *WASMInstancePool) startCleanupRoutine() {
	ticker := time.NewTicker(p.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.Cleanup()
		case <-p.stopCleanup:
			return
		}
	}
}

// CreateInstance 创建新实例（辅助方法）
// 注意：此方法需要具体的引擎类型，当前作为示例保留
func (p *WASMInstancePool) CreateInstance(moduleID string, createFn func() (interface{}, error)) (interface{}, error) {
	instance, err := createFn()
	if err != nil {
		return nil, fmt.Errorf("create instance failed: %w", err)
	}

	p.mutex.Lock()
	p.stats.CreatedInstances++
	p.stats.ActiveInstances++
	p.mutex.Unlock()

	return instance, nil
}
