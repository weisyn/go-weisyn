package memory

import (
	"context"
	"fmt"
	"testing"
	"time"

	memoryconfig "github.com/weisyn/v1/internal/config/storage/memory"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// 测试日志实现，用于测试
type testLogger struct{}

func (l *testLogger) Debug(msg string)                          {}
func (l *testLogger) Debugf(format string, args ...interface{}) {}
func (l *testLogger) Info(msg string)                           {}
func (l *testLogger) Infof(format string, args ...interface{})  {}
func (l *testLogger) Warn(msg string)                           {}
func (l *testLogger) Warnf(format string, args ...interface{})  {}
func (l *testLogger) Error(msg string)                          {}
func (l *testLogger) Errorf(format string, args ...interface{}) {}
func (l *testLogger) Fatal(msg string)                          {}
func (l *testLogger) Fatalf(format string, args ...interface{}) {}
func (l *testLogger) With(args ...interface{}) log.Logger       { return l }
func (l *testLogger) Sync() error                               { return nil }
func (l *testLogger) Close() error                              { return nil }
func (l *testLogger) GetZapLogger() *zap.Logger                 { return nil }

// 测试配置实现
type testMemoryConfig struct{}

func (c *testMemoryConfig) GetLifeWindow() string      { return "5m" }
func (c *testMemoryConfig) GetCleanWindow() string     { return "2m" }
func (c *testMemoryConfig) GetMaxEntriesInWindow() int { return 1000 }
func (c *testMemoryConfig) GetMaxEntrySize() int       { return 1024 }
func (c *testMemoryConfig) GetShards() int             { return 16 }

// setupTestStore 创建测试存储
func setupTestStore(t *testing.T) *Store {
	config := memoryconfig.New(nil) // 使用默认配置
	logger := &testLogger{}
	store := New(config, logger)
	require.NotNil(t, store)
	return store.(*Store)
}

// TestBasicOperations 测试基本操作
func TestBasicOperations(t *testing.T) {
	store := setupTestStore(t)
	// 注意：新的存储接口已移除Close()方法，资源由DI容器自动管理

	ctx := context.Background()

	// 测试设置和获取
	key := "test-key"
	value := []byte("test-value")

	// 测试不存在的键
	_, exists, err := store.Get(ctx, key)
	assert.NoError(t, err)
	assert.False(t, exists)

	exists, err = store.Exists(ctx, key)
	assert.NoError(t, err)
	assert.False(t, exists)

	// 测试设置键值
	err = store.Set(ctx, key, value, 0)
	assert.NoError(t, err)

	// 测试键存在
	exists, err = store.Exists(ctx, key)
	assert.NoError(t, err)
	assert.True(t, exists)

	// 测试获取值
	result, exists, err := store.Get(ctx, key)
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, value, result)

	// 测试删除键
	err = store.Delete(ctx, key)
	assert.NoError(t, err)

	exists, err = store.Exists(ctx, key)
	assert.NoError(t, err)
	assert.False(t, exists)
}

// TestTTL 测试TTL功能
func TestTTL(t *testing.T) {
	store := setupTestStore(t)
	// 注意：新的存储接口已移除Close()方法，资源由DI容器自动管理

	ctx := context.Background()

	// 测试带TTL的设置
	key := "ttl-key"
	value := []byte("ttl-value")
	ttl := 500 * time.Millisecond

	// 设置带TTL的键值
	err := store.Set(ctx, key, value, ttl)
	assert.NoError(t, err)

	// 立即检查，应该存在
	exists, err := store.Exists(ctx, key)
	assert.NoError(t, err)
	assert.True(t, exists)

	// 获取TTL
	remaining, err := store.GetTTL(ctx, key)
	assert.NoError(t, err)
	assert.True(t, remaining > 0)
	assert.True(t, remaining <= ttl)

	// 等待过期
	time.Sleep(ttl + 100*time.Millisecond)

	// 再次检查，应该已过期
	exists, err = store.Exists(ctx, key)
	assert.NoError(t, err)
	assert.False(t, exists)

	// 测试更新TTL
	err = store.Set(ctx, "update-ttl-key", []byte("update-ttl-value"), ttl)
	assert.NoError(t, err)

	// 更新TTL
	newTTL := 2 * ttl
	err = store.UpdateTTL(ctx, "update-ttl-key", newTTL)
	assert.NoError(t, err)

	// 等待原TTL过期
	time.Sleep(ttl + 100*time.Millisecond)

	// 检查键是否仍然存在
	exists, err = store.Exists(ctx, "update-ttl-key")
	assert.NoError(t, err)
	assert.True(t, exists)
}

// TestBatchOperations 测试批量操作
func TestBatchOperations(t *testing.T) {
	store := setupTestStore(t)
	// 注意：新的存储接口已移除Close()方法，资源由DI容器自动管理

	ctx := context.Background()

	// 测试批量设置
	items := map[string][]byte{
		"batch-key1": []byte("batch-value1"),
		"batch-key2": []byte("batch-value2"),
		"batch-key3": []byte("batch-value3"),
	}

	err := store.SetMany(ctx, items, 0)
	assert.NoError(t, err)

	// 测试批量获取
	keys := []string{
		"batch-key1",
		"batch-key2",
		"batch-key3",
		"batch-key4", // 不存在的键
	}

	results, err := store.GetMany(ctx, keys)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(results))
	assert.Equal(t, []byte("batch-value1"), results["batch-key1"])
	assert.Equal(t, []byte("batch-value2"), results["batch-key2"])
	assert.Equal(t, []byte("batch-value3"), results["batch-key3"])
	assert.Nil(t, results["batch-key4"])

	// 测试批量删除
	deleteKeys := []string{
		"batch-key1",
		"batch-key3",
	}

	err = store.DeleteMany(ctx, deleteKeys)
	assert.NoError(t, err)

	// 验证删除结果
	exists, err := store.Exists(ctx, "batch-key1")
	assert.NoError(t, err)
	assert.False(t, exists)

	exists, err = store.Exists(ctx, "batch-key2")
	assert.NoError(t, err)
	assert.True(t, exists)

	exists, err = store.Exists(ctx, "batch-key3")
	assert.NoError(t, err)
	assert.False(t, exists)
}

// TestCount 测试计数功能
func TestCount(t *testing.T) {
	store := setupTestStore(t)
	// 注意：新的存储接口已移除Close()方法，资源由DI容器自动管理

	ctx := context.Background()

	// 初始应该为0
	count, err := store.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// 添加10个键
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("count-key-%d", i)
		value := []byte(fmt.Sprintf("count-value-%d", i))
		err := store.Set(ctx, key, value, 0)
		assert.NoError(t, err)
	}

	// 获取计数
	count, err = store.Count(ctx)
	assert.NoError(t, err)
	assert.True(t, count > 0, "计数应该大于0")

	// 清空缓存
	err = store.Clear(ctx)
	assert.NoError(t, err)

	// 再次获取计数，应该为0
	count, err = store.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

// TestClear 测试清空功能
func TestClear(t *testing.T) {
	store := setupTestStore(t)
	// 注意：新的存储接口已移除Close()方法，资源由DI容器自动管理

	ctx := context.Background()

	// 设置多个键
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("clear-key-%d", i)
		value := []byte(fmt.Sprintf("clear-value-%d", i))
		err := store.Set(ctx, key, value, 0)
		assert.NoError(t, err)
	}

	// 验证键存在
	exists, err := store.Exists(ctx, "clear-key-0")
	assert.NoError(t, err)
	assert.True(t, exists)

	// 清空缓存
	err = store.Clear(ctx)
	assert.NoError(t, err)

	// 验证键不存在
	exists, err = store.Exists(ctx, "clear-key-0")
	assert.NoError(t, err)
	assert.False(t, exists)
}

// TestDeleteByPattern 测试模式删除功能
func TestDeleteByPattern(t *testing.T) {
	store := setupTestStore(t)
	// 注意：新的存储接口已移除Close()方法，资源由DI容器自动管理

	ctx := context.Background()

	// 设置测试数据
	testData := map[string][]byte{
		"user:123":   []byte("user123"),
		"user:456":   []byte("user456"),
		"cache:abc":  []byte("cache_abc"),
		"cache:def":  []byte("cache_def"),
		"temp:xyz":   []byte("temp_xyz"),
		"other:data": []byte("other_data"),
	}

	for key, value := range testData {
		err := store.Set(ctx, key, value, 0)
		assert.NoError(t, err)
	}

	// 测试匹配模式删除
	deletedCount, err := store.DeleteByPattern(ctx, "user:*")
	assert.NoError(t, err)
	assert.Equal(t, int64(2), deletedCount)

	// 验证user:*的键已被删除
	exists, err := store.Exists(ctx, "user:123")
	assert.NoError(t, err)
	assert.False(t, exists)

	exists, err = store.Exists(ctx, "user:456")
	assert.NoError(t, err)
	assert.False(t, exists)

	// 验证其他键仍然存在
	exists, err = store.Exists(ctx, "cache:abc")
	assert.NoError(t, err)
	assert.True(t, exists)

	exists, err = store.Exists(ctx, "temp:xyz")
	assert.NoError(t, err)
	assert.True(t, exists)

	// 测试删除不存在的模式
	deletedCount, err = store.DeleteByPattern(ctx, "nonexistent:*")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), deletedCount)
}

// TestGetKeys 测试获取键功能
func TestGetKeys(t *testing.T) {
	store := setupTestStore(t)
	// 注意：新的存储接口已移除Close()方法，资源由DI容器自动管理

	ctx := context.Background()

	// 设置测试数据
	testData := map[string][]byte{
		"user:123":   []byte("user123"),
		"user:456":   []byte("user456"),
		"cache:abc":  []byte("cache_abc"),
		"cache:def":  []byte("cache_def"),
		"temp:xyz":   []byte("temp_xyz"),
		"other:data": []byte("other_data"),
	}

	for key, value := range testData {
		err := store.Set(ctx, key, value, 0)
		assert.NoError(t, err)
	}

	// 测试获取所有键
	keys, err := store.GetKeys(ctx, "*")
	assert.NoError(t, err)
	assert.Equal(t, 6, len(keys))

	// 测试匹配特定模式的键
	userKeys, err := store.GetKeys(ctx, "user:*")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(userKeys))
	assert.Contains(t, userKeys, "user:123")
	assert.Contains(t, userKeys, "user:456")

	// 测试匹配cache前缀的键
	cacheKeys, err := store.GetKeys(ctx, "cache:*")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(cacheKeys))
	assert.Contains(t, cacheKeys, "cache:abc")
	assert.Contains(t, cacheKeys, "cache:def")

	// 测试不匹配任何键的模式
	noMatchKeys, err := store.GetKeys(ctx, "nonexistent:*")
	assert.NoError(t, err)
	assert.Equal(t, 0, len(noMatchKeys))

	// 测试空模式（应该返回所有键）
	allKeys, err := store.GetKeys(ctx, "")
	assert.NoError(t, err)
	assert.Equal(t, 6, len(allKeys))
}

// TestPatternMatchingWithTTL 测试带TTL的模式匹配
func TestPatternMatchingWithTTL(t *testing.T) {
	store := setupTestStore(t)
	// 注意：新的存储接口已移除Close()方法，资源由DI容器自动管理

	ctx := context.Background()

	// 设置带TTL的键
	err := store.Set(ctx, "temp:short", []byte("short_data"), 100*time.Millisecond)
	assert.NoError(t, err)

	err = store.Set(ctx, "temp:long", []byte("long_data"), 5*time.Second)
	assert.NoError(t, err)

	err = store.Set(ctx, "permanent:data", []byte("permanent"), 0)
	assert.NoError(t, err)

	// 立即检查所有键都存在
	keys, err := store.GetKeys(ctx, "*")
	assert.NoError(t, err)
	assert.Equal(t, 3, len(keys))

	// 等待短TTL过期
	time.Sleep(150 * time.Millisecond)

	// 检查过期键已被自动清理
	keys, err = store.GetKeys(ctx, "*")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(keys))
	assert.NotContains(t, keys, "temp:short")
	assert.Contains(t, keys, "temp:long")
	assert.Contains(t, keys, "permanent:data")

	// 测试删除模式
	deletedCount, err := store.DeleteByPattern(ctx, "temp:*")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), deletedCount) // 只有temp:long还存在

	// 验证结果
	finalKeys, err := store.GetKeys(ctx, "*")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(finalKeys))
	assert.Contains(t, finalKeys, "permanent:data")
}
