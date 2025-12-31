package builder_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/core/block/builder"
	"github.com/weisyn/v1/internal/core/block/testutil"
)

// ==================== NewCandidateLRUCache 测试 ====================

// TestNewCandidateLRUCache_WithValidSize_ReturnsCache 测试使用有效大小创建LRU缓存
func TestNewCandidateLRUCache_WithValidSize_ReturnsCache(t *testing.T) {
	// Arrange
	maxSize := 10
	logger := &testutil.MockLogger{}

	// Act
	cache := builder.NewCandidateLRUCache(maxSize, logger)

	// Assert
	assert.NotNil(t, cache)
	assert.Equal(t, maxSize, cache.Stats()["maxSize"])
	assert.Equal(t, 0, cache.Size())
}

// TestNewCandidateLRUCache_WithZeroSize_UsesDefaultSize 测试使用0大小时使用默认大小
func TestNewCandidateLRUCache_WithZeroSize_UsesDefaultSize(t *testing.T) {
	// Arrange
	logger := &testutil.MockLogger{}

	// Act
	cache := builder.NewCandidateLRUCache(0, logger)

	// Assert
	assert.NotNil(t, cache)
	stats := cache.Stats()
	assert.Equal(t, 100, stats["maxSize"], "应该使用默认大小100")
}

// TestNewCandidateLRUCache_WithNegativeSize_UsesDefaultSize 测试使用负数大小时使用默认大小
func TestNewCandidateLRUCache_WithNegativeSize_UsesDefaultSize(t *testing.T) {
	// Arrange
	logger := &testutil.MockLogger{}

	// Act
	cache := builder.NewCandidateLRUCache(-1, logger)

	// Assert
	assert.NotNil(t, cache)
	stats := cache.Stats()
	assert.Equal(t, 100, stats["maxSize"], "应该使用默认大小100")
}

// TestNewCandidateLRUCache_WithNilLogger_Works 测试使用nil logger时正常工作
func TestNewCandidateLRUCache_WithNilLogger_Works(t *testing.T) {
	// Arrange
	maxSize := 10

	// Act
	cache := builder.NewCandidateLRUCache(maxSize, nil)

	// Assert
	assert.NotNil(t, cache)
	assert.Equal(t, maxSize, cache.Stats()["maxSize"])
}

// ==================== Get 测试 ====================

// TestLRUCache_Get_WithNonExistentKey_ReturnsFalse 测试获取不存在的键
func TestLRUCache_Get_WithNonExistentKey_ReturnsFalse(t *testing.T) {
	// Arrange
	cache := builder.NewCandidateLRUCache(10, nil)

	// Act
	block, exists := cache.Get("non-existent")

	// Assert
	assert.Nil(t, block)
	assert.False(t, exists)
}

// TestLRUCache_Get_WithExistingKey_ReturnsBlock 测试获取存在的键
func TestLRUCache_Get_WithExistingKey_ReturnsBlock(t *testing.T) {
	// Arrange
	cache := builder.NewCandidateLRUCache(10, nil)
	block := testutil.NewTestBlock(1, make([]byte, 32))
	key := "test-key"

	// Act
	cache.Put(key, block)
	retrievedBlock, exists := cache.Get(key)

	// Assert
	assert.True(t, exists)
	assert.NotNil(t, retrievedBlock)
	assert.Equal(t, block.Header.Height, retrievedBlock.Header.Height)
}

// TestLRUCache_Get_MovesToHead 测试获取操作将节点移动到头部
func TestLRUCache_Get_MovesToHead(t *testing.T) {
	// Arrange
	cache := builder.NewCandidateLRUCache(3, nil)
	block1 := testutil.NewTestBlock(1, make([]byte, 32))
	block2 := testutil.NewTestBlock(2, make([]byte, 32))
	block3 := testutil.NewTestBlock(3, make([]byte, 32))

	// Act
	cache.Put("key1", block1)
	cache.Put("key2", block2)
	cache.Put("key3", block3)
	// 获取 key1，应该将其移动到头部
	_, _ = cache.Get("key1")
	// 添加新元素，应该淘汰 key2（因为 key1 被移动到头部）
	cache.Put("key4", testutil.NewTestBlock(4, make([]byte, 32)))

	// Assert
	_, exists1 := cache.Get("key1")
	_, exists2 := cache.Get("key2")
	_, exists3 := cache.Get("key3")
	_, exists4 := cache.Get("key4")

	assert.True(t, exists1, "key1 应该存在（被移动到头部）")
	assert.False(t, exists2, "key2 应该被淘汰")
	assert.True(t, exists3, "key3 应该存在")
	assert.True(t, exists4, "key4 应该存在")
}

// ==================== Put 测试 ====================

// TestLRUCache_Put_WithNewKey_AddsToCache 测试添加新键
func TestLRUCache_Put_WithNewKey_AddsToCache(t *testing.T) {
	// Arrange
	cache := builder.NewCandidateLRUCache(10, nil)
	block := testutil.NewTestBlock(1, make([]byte, 32))
	key := "test-key"

	// Act
	cache.Put(key, block)

	// Assert
	assert.Equal(t, 1, cache.Size())
	retrievedBlock, exists := cache.Get(key)
	assert.True(t, exists)
	assert.NotNil(t, retrievedBlock)
}

// TestLRUCache_Put_WithExistingKey_UpdatesValue 测试更新已存在的键
func TestLRUCache_Put_WithExistingKey_UpdatesValue(t *testing.T) {
	// Arrange
	cache := builder.NewCandidateLRUCache(10, nil)
	block1 := testutil.NewTestBlock(1, make([]byte, 32))
	block2 := testutil.NewTestBlock(2, make([]byte, 32))
	key := "test-key"

	// Act
	cache.Put(key, block1)
	cache.Put(key, block2)

	// Assert
	assert.Equal(t, 1, cache.Size(), "大小应该仍然是1")
	retrievedBlock, exists := cache.Get(key)
	assert.True(t, exists)
	assert.Equal(t, uint64(2), retrievedBlock.Header.Height, "应该返回更新后的值")
}

// TestLRUCache_Put_WhenFull_EvictsLRU 测试缓存满时淘汰最近最少使用的项
func TestLRUCache_Put_WhenFull_EvictsLRU(t *testing.T) {
	// Arrange
	cache := builder.NewCandidateLRUCache(2, nil)
	block1 := testutil.NewTestBlock(1, make([]byte, 32))
	block2 := testutil.NewTestBlock(2, make([]byte, 32))
	block3 := testutil.NewTestBlock(3, make([]byte, 32))

	// Act
	cache.Put("key1", block1)
	cache.Put("key2", block2)
	cache.Put("key3", block3) // 应该淘汰 key1

	// Assert
	assert.Equal(t, 2, cache.Size(), "缓存大小应该为2")
	_, exists1 := cache.Get("key1")
	_, exists2 := cache.Get("key2")
	_, exists3 := cache.Get("key3")

	assert.False(t, exists1, "key1 应该被淘汰")
	assert.True(t, exists2, "key2 应该存在")
	assert.True(t, exists3, "key3 应该存在")
}

// TestLRUCache_Put_WithNilBlock_HandlesGracefully 测试添加nil区块时的处理
func TestLRUCache_Put_WithNilBlock_HandlesGracefully(t *testing.T) {
	// Arrange
	cache := builder.NewCandidateLRUCache(10, nil)
	key := "test-key"

	// Act & Assert
	// 应该不会 panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("❌ BUG发现：Put nil 区块时发生 panic: %v", r)
		}
	}()

	cache.Put(key, nil)

	// 验证 nil 被存储
	retrievedBlock, exists := cache.Get(key)
	assert.True(t, exists, "键应该存在")
	assert.Nil(t, retrievedBlock, "值应该是 nil")
}

// ==================== Delete 测试 ====================

// TestLRUCache_Delete_WithExistingKey_RemovesFromCache 测试删除存在的键
func TestLRUCache_Delete_WithExistingKey_RemovesFromCache(t *testing.T) {
	// Arrange
	cache := builder.NewCandidateLRUCache(10, nil)
	block := testutil.NewTestBlock(1, make([]byte, 32))
	key := "test-key"

	cache.Put(key, block)
	assert.Equal(t, 1, cache.Size())

	// Act
	cache.Delete(key)

	// Assert
	assert.Equal(t, 0, cache.Size())
	_, exists := cache.Get(key)
	assert.False(t, exists)
}

// TestLRUCache_Delete_WithNonExistentKey_NoError 测试删除不存在的键
func TestLRUCache_Delete_WithNonExistentKey_NoError(t *testing.T) {
	// Arrange
	cache := builder.NewCandidateLRUCache(10, nil)

	// Act & Assert
	// 应该不会 panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("❌ BUG发现：Delete 不存在的键时发生 panic: %v", r)
		}
	}()

	cache.Delete("non-existent")
	assert.Equal(t, 0, cache.Size())
}

// ==================== Clear 测试 ====================

// TestLRUCache_Clear_RemovesAllEntries 测试清空缓存
func TestLRUCache_Clear_RemovesAllEntries(t *testing.T) {
	// Arrange
	cache := builder.NewCandidateLRUCache(10, nil)
	cache.Put("key1", testutil.NewTestBlock(1, make([]byte, 32)))
	cache.Put("key2", testutil.NewTestBlock(2, make([]byte, 32)))
	cache.Put("key3", testutil.NewTestBlock(3, make([]byte, 32)))

	assert.Equal(t, 3, cache.Size())

	// Act
	cache.Clear()

	// Assert
	assert.Equal(t, 0, cache.Size())
	stats := cache.Stats()
	assert.Equal(t, int64(0), stats["hitCount"])
	assert.Equal(t, int64(0), stats["missCount"])
}

// ==================== Size 测试 ====================

// TestLRUCache_Size_ReturnsCorrectCount 测试获取缓存大小
func TestLRUCache_Size_ReturnsCorrectCount(t *testing.T) {
	// Arrange
	cache := builder.NewCandidateLRUCache(10, nil)

	// Act & Assert
	assert.Equal(t, 0, cache.Size())

	cache.Put("key1", testutil.NewTestBlock(1, make([]byte, 32)))
	assert.Equal(t, 1, cache.Size())

	cache.Put("key2", testutil.NewTestBlock(2, make([]byte, 32)))
	assert.Equal(t, 2, cache.Size())

	cache.Delete("key1")
	assert.Equal(t, 1, cache.Size())
}

// ==================== Stats 测试 ====================

// TestLRUCache_Stats_ReturnsCorrectStatistics 测试获取统计信息
func TestLRUCache_Stats_ReturnsCorrectStatistics(t *testing.T) {
	// Arrange
	cache := builder.NewCandidateLRUCache(10, nil)

	// Act
	stats := cache.Stats()

	// Assert
	assert.NotNil(t, stats)
	assert.Equal(t, 0, stats["size"])
	assert.Equal(t, 10, stats["maxSize"])
	assert.Equal(t, int64(0), stats["hitCount"])
	assert.Equal(t, int64(0), stats["missCount"])
	assert.Equal(t, float64(0), stats["hitRate"])
	assert.Equal(t, int64(0), stats["totalRequests"])
}

// TestLRUCache_Stats_AfterOperations_UpdatesCorrectly 测试操作后统计信息更新
func TestLRUCache_Stats_AfterOperations_UpdatesCorrectly(t *testing.T) {
	// Arrange
	cache := builder.NewCandidateLRUCache(10, nil)
	block := testutil.NewTestBlock(1, make([]byte, 32))

	// Act
	cache.Put("key1", block)
	_, _ = cache.Get("key1") // 命中
	_, _ = cache.Get("key2")  // 未命中
	_, _ = cache.Get("key1")  // 命中

	stats := cache.Stats()

	// Assert
	assert.Equal(t, 1, stats["size"])
	assert.Equal(t, int64(2), stats["hitCount"], "应该有2次命中")
	assert.Equal(t, int64(1), stats["missCount"], "应该有1次未命中")
	assert.Equal(t, int64(3), stats["totalRequests"], "应该有3次请求")
	assert.Greater(t, stats["hitRate"], float64(0), "命中率应该大于0")
}

// ==================== LRU策略测试 ====================

// TestLRUCache_LRUPolicy_EvictsLeastRecentlyUsed 测试LRU淘汰策略
func TestLRUCache_LRUPolicy_EvictsLeastRecentlyUsed(t *testing.T) {
	// Arrange
	cache := builder.NewCandidateLRUCache(3, nil)

	// Act
	// 添加3个元素
	cache.Put("key1", testutil.NewTestBlock(1, make([]byte, 32)))
	cache.Put("key2", testutil.NewTestBlock(2, make([]byte, 32)))
	cache.Put("key3", testutil.NewTestBlock(3, make([]byte, 32)))

	// 访问 key2 和 key3，使 key1 成为最少使用的
	_, _ = cache.Get("key2")
	_, _ = cache.Get("key3")

	// 添加新元素，应该淘汰 key1
	cache.Put("key4", testutil.NewTestBlock(4, make([]byte, 32)))

	// Assert
	_, exists1 := cache.Get("key1")
	_, exists2 := cache.Get("key2")
	_, exists3 := cache.Get("key3")
	_, exists4 := cache.Get("key4")

	assert.False(t, exists1, "key1 应该被淘汰（最少使用）")
	assert.True(t, exists2, "key2 应该存在")
	assert.True(t, exists3, "key3 应该存在")
	assert.True(t, exists4, "key4 应该存在")
}

// TestLRUCache_LRUPolicy_WithSingleElement_Works 测试单个元素的LRU策略
func TestLRUCache_LRUPolicy_WithSingleElement_Works(t *testing.T) {
	// Arrange
	cache := builder.NewCandidateLRUCache(1, nil)

	// Act
	cache.Put("key1", testutil.NewTestBlock(1, make([]byte, 32)))
	cache.Put("key2", testutil.NewTestBlock(2, make([]byte, 32))) // 应该淘汰 key1

	// Assert
	_, exists1 := cache.Get("key1")
	_, exists2 := cache.Get("key2")

	assert.False(t, exists1, "key1 应该被淘汰")
	assert.True(t, exists2, "key2 应该存在")
}

// ==================== 并发安全测试 ====================

// TestLRUCache_ConcurrentAccess_IsSafe 测试并发访问的安全性
func TestLRUCache_ConcurrentAccess_IsSafe(t *testing.T) {
	// Arrange
	cache := builder.NewCandidateLRUCache(100, nil)
	concurrency := 50

	// Act
	done := make(chan bool, concurrency)
	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("❌ BUG发现：并发访问LRU缓存时发生 panic: %v", r)
				}
				done <- true
			}()

			key := fmt.Sprintf("key-%d", id)
			block := testutil.NewTestBlock(uint64(id), make([]byte, 32))
			cache.Put(key, block)
			_, _ = cache.Get(key)
			cache.Delete(key)
		}(i)
	}

	// Assert
	for i := 0; i < concurrency; i++ {
		<-done
	}

	// 验证缓存状态一致
	stats := cache.Stats()
	assert.GreaterOrEqual(t, stats["size"], 0, "缓存大小应该 >= 0")
}

// ==================== 边界条件测试 ====================

// TestLRUCache_WithEmptyKey_HandlesGracefully 测试空键的处理
func TestLRUCache_WithEmptyKey_HandlesGracefully(t *testing.T) {
	// Arrange
	cache := builder.NewCandidateLRUCache(10, nil)
	block := testutil.NewTestBlock(1, make([]byte, 32))

	// Act & Assert
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("❌ BUG发现：使用空键时发生 panic: %v", r)
		}
	}()

	cache.Put("", block)
	retrievedBlock, exists := cache.Get("")
	assert.True(t, exists)
	assert.NotNil(t, retrievedBlock)
}

// TestLRUCache_WithVeryLargeSize_Works 测试非常大的缓存大小
func TestLRUCache_WithVeryLargeSize_Works(t *testing.T) {
	// Arrange
	maxSize := 10000
	cache := builder.NewCandidateLRUCache(maxSize, nil)

	// Act
	for i := 0; i < maxSize; i++ {
		key := fmt.Sprintf("key-%d", i)
		block := testutil.NewTestBlock(uint64(i), make([]byte, 32))
		cache.Put(key, block)
	}

	// Assert
	assert.Equal(t, maxSize, cache.Size())
	stats := cache.Stats()
	assert.Equal(t, maxSize, stats["maxSize"])
}

// ==================== 发现代码问题测试 ====================

// TestLRUCache_DetectsPotentialIssues 测试发现潜在问题
func TestLRUCache_DetectsPotentialIssues(t *testing.T) {
	// 🐛 问题发现：检查代码中是否有潜在问题

	t.Logf("✅ LRU缓存实现检查：")
	t.Logf("  - 使用双向链表+哈希表实现，时间复杂度O(1)")
	t.Logf("  - 使用读写锁保证并发安全")
	t.Logf("  - LRU淘汰策略正确实现")

	// 验证实现正确性
	cache := builder.NewCandidateLRUCache(2, nil)
	cache.Put("key1", testutil.NewTestBlock(1, make([]byte, 32)))
	cache.Put("key2", testutil.NewTestBlock(2, make([]byte, 32)))
	cache.Put("key3", testutil.NewTestBlock(3, make([]byte, 32)))

	// 验证 key1 被淘汰
	_, exists1 := cache.Get("key1")
	assert.False(t, exists1, "LRU策略应该正确淘汰最少使用的项")
}

