// Package memory 提供基于BigCache的内存缓存实现
package memory

import (
	"context"
	"fmt"
	"time"

	memoryconfig "github.com/weisyn/v1/internal/config/storage/memory"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// 默认配置实现
type defaultMemoryConfig struct{}

func (c *defaultMemoryConfig) GetLifeWindow() string      { return "10m" }
func (c *defaultMemoryConfig) GetCleanWindow() string     { return "5m" }
func (c *defaultMemoryConfig) GetMaxEntriesInWindow() int { return 10000 }
func (c *defaultMemoryConfig) GetMaxEntrySize() int       { return 500 }
func (c *defaultMemoryConfig) GetShards() int             { return 1024 }

// 自定义配置实现
type customMemoryConfig struct {
	lifeWindow         string
	cleanWindow        string
	maxEntriesInWindow int
	maxEntrySize       int
	shards             int
}

func (c *customMemoryConfig) GetLifeWindow() string      { return c.lifeWindow }
func (c *customMemoryConfig) GetCleanWindow() string     { return c.cleanWindow }
func (c *customMemoryConfig) GetMaxEntriesInWindow() int { return c.maxEntriesInWindow }
func (c *customMemoryConfig) GetMaxEntrySize() int       { return c.maxEntrySize }
func (c *customMemoryConfig) GetShards() int             { return c.shards }

// 创建自定义配置的辅助函数
func newCustomConfig(lifeWindow, cleanWindow string, maxEntries, maxSize, shards int) *customMemoryConfig {
	return &customMemoryConfig{
		lifeWindow:         lifeWindow,
		cleanWindow:        cleanWindow,
		maxEntriesInWindow: maxEntries,
		maxEntrySize:       maxSize,
		shards:             shards,
	}
}

// Example 展示如何使用BigCache内存存储
func Example(logger log.Logger) {
	// 创建默认配置 - 使用新的配置系统
	config := memoryconfig.New(nil)

	// 创建内存存储
	store := New(config, logger)
	// 注意：新的存储接口已移除Close()方法，资源由DI容器自动管理

	// 创建上下文
	ctx := context.Background()

	// 基本操作
	key := "example-key"
	value := []byte("example-value")

	// 设置键值对（无过期时间）
	err := store.Set(ctx, key, value, 0)
	if err != nil {
		logger.Errorf("设置缓存失败: %v", err)
		return
	}

	// 设置带过期时间的键值对
	err = store.Set(ctx, "ttl-key", []byte("ttl-value"), 5*time.Minute)
	if err != nil {
		logger.Errorf("设置带TTL的缓存失败: %v", err)
		return
	}

	// 检查键是否存在
	exists, err := store.Exists(ctx, key)
	if err != nil {
		logger.Errorf("检查键存在失败: %v", err)
		return
	}
	logger.Infof("键 %s 存在: %v", key, exists)

	// 获取值
	result, exists, err := store.Get(ctx, key)
	if err != nil {
		logger.Errorf("获取缓存失败: %v", err)
		return
	}
	if exists {
		logger.Infof("获取到键 %s 的值: %s", key, string(result))
	} else {
		logger.Infof("键 %s 不存在", key)
	}

	// 获取TTL
	ttl, err := store.GetTTL(ctx, "ttl-key")
	if err != nil {
		logger.Errorf("获取TTL失败: %v", err)
		return
	}
	logger.Infof("键 ttl-key 剩余生存时间: %v", ttl)

	// 更新TTL
	err = store.UpdateTTL(ctx, "ttl-key", 10*time.Minute)
	if err != nil {
		logger.Errorf("更新TTL失败: %v", err)
		return
	}
	logger.Info("已更新TTL为10分钟")

	// 批量操作
	items := map[string][]byte{
		"batch-key1": []byte("batch-value1"),
		"batch-key2": []byte("batch-value2"),
		"batch-key3": []byte("batch-value3"),
	}

	// 批量设置
	err = store.SetMany(ctx, items, 0)
	if err != nil {
		logger.Errorf("批量设置失败: %v", err)
		return
	}

	// 批量获取
	keys := []string{"batch-key1", "batch-key2", "batch-key3", "nonexistent-key"}
	results, err := store.GetMany(ctx, keys)
	if err != nil {
		logger.Errorf("批量获取失败: %v", err)
		return
	}
	for k, v := range results {
		logger.Infof("批量获取: %s = %s", k, string(v))
	}

	// 获取缓存中的键数量
	count, err := store.Count(ctx)
	if err != nil {
		logger.Errorf("获取计数失败: %v", err)
		return
	}
	logger.Infof("缓存中键数量: %d", count)

	// 批量删除
	err = store.DeleteMany(ctx, []string{"batch-key1", "batch-key3"})
	if err != nil {
		logger.Errorf("批量删除失败: %v", err)
		return
	}
	logger.Info("已删除batch-key1和batch-key3")

	// 删除单个键
	err = store.Delete(ctx, key)
	if err != nil {
		logger.Errorf("删除键失败: %v", err)
		return
	}
	logger.Infof("已删除键 %s", key)

	// 清空所有缓存
	err = store.Clear(ctx)
	if err != nil {
		logger.Errorf("清空缓存失败: %v", err)
		return
	}
	logger.Info("已清空所有缓存")

	// 验证缓存已清空
	count, _ = store.Count(ctx)
	logger.Infof("清空后缓存中键数量: %d", count)
}

// ExampleBenchmark 展示如何使用BigCache进行大量数据操作的基准测试
func ExampleBenchmark(logger log.Logger) {
	// 创建优化配置，适合大量数据 - 使用新的配置系统
	options := &memoryconfig.MemoryOptions{
		MaxMemory:       100 << 20, // 100MB
		MaxEntries:      1000000,   // 100万条目
		DefaultTTL:      3600,      // 1小时
		CleanupInterval: 1800,      // 30分钟清理
	}
	config := memoryconfig.New(options)

	// 创建内存存储
	store := New(config, logger)
	// 注意：新的存储接口已移除Close()方法，资源由DI容器自动管理

	ctx := context.Background()

	// 基准测试：写入10万个键值对
	startTime := time.Now()
	for i := 0; i < 100000; i++ {
		key := fmt.Sprintf("bench-key-%d", i)
		value := []byte(fmt.Sprintf("bench-value-%d", i))
		if err := store.Set(ctx, key, value, 0); err != nil {
			logger.Errorf("写入失败: %v", err)
			return
		}
	}
	writeTime := time.Since(startTime)
	logger.Infof("写入10万个键值对耗时: %v", writeTime)

	// 统计键数量
	count, _ := store.Count(ctx)
	logger.Infof("缓存中键数量: %d", count)

	// 基准测试：读取10万个键值对
	startTime = time.Now()
	for i := 0; i < 100000; i++ {
		key := fmt.Sprintf("bench-key-%d", i)
		_, exists, err := store.Get(ctx, key)
		if err != nil {
			logger.Errorf("读取失败: %v", err)
			return
		}
		if !exists {
			logger.Warnf("未找到键: %s", key)
		}
	}
	readTime := time.Since(startTime)
	logger.Infof("读取10万个键值对耗时: %v", readTime)

	// 清空缓存
	err := store.Clear(ctx)
	if err != nil {
		logger.Errorf("清空缓存失败: %v", err)
		return
	}
}
