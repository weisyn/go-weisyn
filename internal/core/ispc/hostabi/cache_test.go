package hostabi

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/ispc/testutil"
	utxo "github.com/weisyn/v1/pb/blockchain/utxo"
	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ============================================================================
// PrimitiveCallCache 测试
// ============================================================================
//
// 🎯 **测试目的**：发现 PrimitiveCallCache 的缺陷和BUG
//
// ============================================================================

// TestNewPrimitiveCallCache 测试创建缓存
func TestNewPrimitiveCallCache(t *testing.T) {
	logger := testutil.NewTestLogger()
	maxSize := 100
	defaultTTL := 5 * time.Minute

	cache := NewPrimitiveCallCache(logger, maxSize, defaultTTL)

	assert.NotNil(t, cache, "应该成功创建缓存")
	assert.Equal(t, maxSize, cache.maxSize, "应该设置最大大小")
	assert.Equal(t, defaultTTL, cache.defaultTTL, "应该设置默认TTL")
	assert.NotNil(t, cache.cache, "应该初始化缓存map")
	assert.NotNil(t, cache.stopCleanup, "应该初始化停止通道")

	// 清理
	cache.Stop()
}

// TestPrimitiveCallCache_Get_NotFound 测试获取不存在的缓存
func TestPrimitiveCallCache_Get_NotFound(t *testing.T) {
	logger := testutil.NewTestLogger()
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()

	result, err, found := cache.Get("nonexistent")

	assert.False(t, found, "应该返回未找到")
	assert.Nil(t, result, "结果应该为nil")
	assert.Nil(t, err, "错误应该为nil")
}

// TestPrimitiveCallCache_SetAndGet 测试设置和获取缓存
func TestPrimitiveCallCache_SetAndGet(t *testing.T) {
	logger := testutil.NewTestLogger()
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()

	cacheKey := "test_key"
	testValue := uint64(12345)

	cache.Set(cacheKey, testValue, nil, 0)

	result, err, found := cache.Get(cacheKey)

	assert.True(t, found, "应该找到缓存")
	assert.Nil(t, err, "错误应该为nil")
	assert.Equal(t, testValue, result, "应该返回正确的值")
}

// TestPrimitiveCallCache_SetAndGet_WithError 测试设置和获取带错误的缓存
func TestPrimitiveCallCache_SetAndGet_WithError(t *testing.T) {
	logger := testutil.NewTestLogger()
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()

	cacheKey := "test_key_error"
	testError := assert.AnError

	cache.Set(cacheKey, nil, testError, 0)

	result, err, found := cache.Get(cacheKey)

	assert.True(t, found, "应该找到缓存")
	assert.Equal(t, testError, err, "应该返回缓存的错误")
	assert.Nil(t, result, "结果应该为nil")
}

// TestPrimitiveCallCache_Expired 测试过期缓存
func TestPrimitiveCallCache_Expired(t *testing.T) {
	logger := testutil.NewTestLogger()
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()

	cacheKey := "expired_key"
	testValue := uint64(12345)

	// 设置一个很短的TTL
	cache.Set(cacheKey, testValue, nil, 100*time.Millisecond)

	// 等待过期
	time.Sleep(150 * time.Millisecond)

	result, err, found := cache.Get(cacheKey)

	assert.False(t, found, "应该返回未找到（已过期）")
	assert.Nil(t, result, "结果应该为nil")
	assert.Nil(t, err, "错误应该为nil")
}

// TestPrimitiveCallCache_Invalidate_EmptyPattern 测试清空所有缓存
func TestPrimitiveCallCache_Invalidate_EmptyPattern(t *testing.T) {
	logger := testutil.NewTestLogger()
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()

	// 设置多个缓存
	cache.Set("key1", uint64(1), nil, 0)
	cache.Set("key2", uint64(2), nil, 0)
	cache.Set("key3", uint64(3), nil, 0)

	// 清空所有缓存
	cache.Invalidate("")

	// 验证所有缓存都被清空
	_, _, found1 := cache.Get("key1")
	_, _, found2 := cache.Get("key2")
	_, _, found3 := cache.Get("key3")

	assert.False(t, found1, "key1应该被清空")
	assert.False(t, found2, "key2应该被清空")
	assert.False(t, found3, "key3应该被清空")
}

// TestPrimitiveCallCache_Invalidate_Pattern 测试按模式使缓存失效
func TestPrimitiveCallCache_Invalidate_Pattern(t *testing.T) {
	logger := testutil.NewTestLogger()
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()

	// 设置多个缓存
	cache.Set("exec1:GetBlockHeight:hash1", uint64(1), nil, 0)
	cache.Set("exec1:GetBlockTimestamp:hash1", uint64(2), nil, 0)
	cache.Set("exec2:GetBlockHeight:hash1", uint64(3), nil, 0)

	// 使exec1相关的缓存失效
	cache.Invalidate("exec1")

	// 验证exec1的缓存被清空，exec2的缓存还在
	_, _, found1 := cache.Get("exec1:GetBlockHeight:hash1")
	_, _, found2 := cache.Get("exec1:GetBlockTimestamp:hash1")
	_, _, found3 := cache.Get("exec2:GetBlockHeight:hash1")

	assert.False(t, found1, "exec1:GetBlockHeight应该被清空")
	assert.False(t, found2, "exec1:GetBlockTimestamp应该被清空")
	assert.True(t, found3, "exec2:GetBlockHeight应该还在")
}

// TestPrimitiveCallCache_Clear 测试清空缓存
func TestPrimitiveCallCache_Clear(t *testing.T) {
	logger := testutil.NewTestLogger()
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()

	// 设置缓存并获取（增加命中次数）
	cache.Set("key1", uint64(1), nil, 0)
	cache.Get("key1")
	cache.Get("key1")

	// 清空缓存
	cache.Clear()

	// 验证缓存被清空（在Clear之后调用Get会增加misses，这是正常的）
	_, _, found := cache.Get("key1")
	assert.False(t, found, "缓存应该被清空")

	// 验证统计信息被重置（注意：Clear之后调用Get会增加misses，所以misses应该是1）
	stats := cache.GetStats()
	assert.Equal(t, 0, stats["size"], "大小应该为0")
	assert.Equal(t, uint64(0), stats["hits"], "命中次数应该为0")
	assert.Equal(t, uint64(1), stats["misses"], "未命中次数应该为1（Clear之后调用Get）")
	assert.Equal(t, uint64(0), stats["evictions"], "驱逐次数应该为0")
}

// TestPrimitiveCallCache_GetStats 测试获取统计信息
func TestPrimitiveCallCache_GetStats(t *testing.T) {
	logger := testutil.NewTestLogger()
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()

	// 设置缓存
	cache.Set("key1", uint64(1), nil, 0)
	cache.Set("key2", uint64(2), nil, 0)

	// 获取缓存（命中）
	cache.Get("key1")
	cache.Get("key1")

	// 获取不存在的缓存（未命中）
	cache.Get("nonexistent")

	stats := cache.GetStats()

	assert.Equal(t, 2, stats["size"], "大小应该为2")
	assert.Equal(t, 100, stats["max_size"], "最大大小应该为100")
	assert.Equal(t, uint64(2), stats["hits"], "命中次数应该为2")
	assert.Equal(t, uint64(1), stats["misses"], "未命中次数应该为1")
	assert.Greater(t, stats["hit_rate"], 0.0, "命中率应该大于0")
}

// TestPrimitiveCallCache_EvictLRU 测试LRU驱逐
func TestPrimitiveCallCache_EvictLRU(t *testing.T) {
	logger := testutil.NewTestLogger()
	cache := NewPrimitiveCallCache(logger, 2, 5*time.Minute) // 最大大小为2
	defer cache.Stop()

	// 设置第一个缓存
	cache.Set("key1", uint64(1), nil, 0)
	time.Sleep(10 * time.Millisecond) // 确保时间不同

	// 设置第二个缓存
	cache.Set("key2", uint64(2), nil, 0)
	time.Sleep(10 * time.Millisecond)

	// 访问key2（更新其LastAccess）
	cache.Get("key2")
	time.Sleep(10 * time.Millisecond)

	// 设置第三个缓存（应该驱逐key1）
	cache.Set("key3", uint64(3), nil, 0)

	// 验证key1被驱逐，key2和key3还在
	_, _, found1 := cache.Get("key1")
	_, _, found2 := cache.Get("key2")
	_, _, found3 := cache.Get("key3")

	assert.False(t, found1, "key1应该被驱逐")
	assert.True(t, found2, "key2应该还在")
	assert.True(t, found3, "key3应该还在")

	stats := cache.GetStats()
	assert.Equal(t, uint64(1), stats["evictions"], "驱逐次数应该为1")
}

// TestPrimitiveCallCache_Stop 测试停止缓存清理
func TestPrimitiveCallCache_Stop(t *testing.T) {
	logger := testutil.NewTestLogger()
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)

	// 停止缓存清理
	cache.Stop()

	// 再次停止应该不会panic
	cache.Stop()
}

// TestPrimitiveCallCache_Cleanup 测试清理过期条目
func TestPrimitiveCallCache_Cleanup(t *testing.T) {
	logger := testutil.NewTestLogger()
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()

	// 设置一个过期和一个未过期的缓存
	cache.Set("expired", uint64(1), nil, 100*time.Millisecond)
	cache.Set("valid", uint64(2), nil, 5*time.Minute)

	// 等待过期
	time.Sleep(150 * time.Millisecond)

	// 手动触发清理
	cache.cleanup()

	// 验证过期缓存被清理
	_, _, foundExpired := cache.Get("expired")
	_, _, foundValid := cache.Get("valid")

	assert.False(t, foundExpired, "过期缓存应该被清理")
	assert.True(t, foundValid, "未过期缓存应该还在")
}

// TestBuildPrimitiveCacheKey 测试构建缓存键
func TestBuildPrimitiveCacheKey(t *testing.T) {
	hashManager := testutil.NewTestHashManager()
	executionID := "exec-123"
	primitiveName := "GetBlockHeight"

	// 测试nil参数
	key1 := buildPrimitiveCacheKey(hashManager, executionID, primitiveName, nil)
	assert.Contains(t, key1, executionID, "应该包含executionID")
	assert.Contains(t, key1, primitiveName, "应该包含primitiveName")
	assert.Contains(t, key1, "nil", "应该包含nil参数标记")

	// 测试有参数
	key2 := buildPrimitiveCacheKey(hashManager, executionID, primitiveName, uint64(100))
	assert.Contains(t, key2, executionID, "应该包含executionID")
	assert.Contains(t, key2, primitiveName, "应该包含primitiveName")
	assert.NotContains(t, key2, "nil", "不应该包含nil参数标记")
}

// ============================================================================
// HostRuntimePortsWithCache 测试
// ============================================================================

// TestNewHostRuntimePortsWithCache 测试创建带缓存的HostABI包装器
func TestNewHostRuntimePortsWithCache(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockHostABI := createTestHostRuntimePorts(t)
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()
	executionID := "exec-123"
	hashManager := testutil.NewTestHashManager()

	wrapper := NewHostRuntimePortsWithCache(mockHostABI, cache, executionID, logger, hashManager)

	assert.NotNil(t, wrapper, "应该成功创建包装器")
	assert.Equal(t, mockHostABI, wrapper.HostABI, "应该设置HostABI")
	assert.Equal(t, cache, wrapper.cache, "应该设置缓存")
	assert.Equal(t, executionID, wrapper.executionID, "应该设置executionID")
}

// TestHostRuntimePortsWithCache_GetCacheStats 测试获取缓存统计信息
func TestHostRuntimePortsWithCache_GetCacheStats(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockHostABI := createTestHostRuntimePorts(t)
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()
	executionID := "exec-123"
	hashManager := testutil.NewTestHashManager()

	wrapper := NewHostRuntimePortsWithCache(mockHostABI, cache, executionID, logger, hashManager)

	stats := wrapper.GetCacheStats()
	assert.NotNil(t, stats, "应该返回统计信息")
}

// TestHostRuntimePortsWithCache_GetCacheStats_NilCache 测试nil缓存的统计信息
func TestHostRuntimePortsWithCache_GetCacheStats_NilCache(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockHostABI := createTestHostRuntimePorts(t)
	executionID := "exec-123"
	hashManager := testutil.NewTestHashManager()

	wrapper := &HostRuntimePortsWithCache{
		HostABI:     mockHostABI,
		cache:       nil,
		executionID: executionID,
		logger:      logger,
		hashManager: hashManager,
	}

	stats := wrapper.GetCacheStats()
	assert.Nil(t, stats, "应该返回nil")
}

// TestHostRuntimePortsWithCache_ClearCache 测试清空缓存
func TestHostRuntimePortsWithCache_ClearCache(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockHostABI := createTestHostRuntimePorts(t)
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()
	executionID := "exec-123"
	hashManager := testutil.NewTestHashManager()

	wrapper := NewHostRuntimePortsWithCache(mockHostABI, cache, executionID, logger, hashManager)

	// 设置缓存
	cache.Set("key1", uint64(1), nil, 0)

	// 清空缓存
	wrapper.ClearCache()

	// 验证缓存被清空
	_, _, found := cache.Get("key1")
	assert.False(t, found, "缓存应该被清空")
}

// TestHostRuntimePortsWithCache_InvalidateCache 测试使缓存失效
func TestHostRuntimePortsWithCache_InvalidateCache(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockHostABI := createTestHostRuntimePorts(t)
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()
	executionID := "exec-123"
	hashManager := testutil.NewTestHashManager()

	wrapper := NewHostRuntimePortsWithCache(mockHostABI, cache, executionID, logger, hashManager)

	// 设置缓存
	cache.Set("exec-123:UTXO:hash1", uint64(1), nil, 0)
	cache.Set("exec-123:UTXO:hash2", uint64(2), nil, 0)

	// 使UTXO相关缓存失效
	wrapper.InvalidateCache("exec-123:UTXO")

	// 验证UTXO相关缓存被清空
	_, _, found1 := cache.Get("exec-123:UTXO:hash1")
	_, _, found2 := cache.Get("exec-123:UTXO:hash2")
	assert.False(t, found1, "UTXO缓存应该被清空")
	assert.False(t, found2, "UTXO缓存应该被清空")
}

// TestHostRuntimePortsWithCache_GetBlockHeight 测试GetBlockHeight缓存
func TestHostRuntimePortsWithCache_GetBlockHeight(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockHostABI := createTestHostRuntimePorts(t)
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()
	executionID := "exec-123"
	hashManager := testutil.NewTestHashManager()

	wrapper := NewHostRuntimePortsWithCache(mockHostABI, cache, executionID, logger, hashManager)

	ctx := context.Background()

	// 第一次调用（应该调用原始方法）
	result1, err1 := wrapper.GetBlockHeight(ctx)
	require.NoError(t, err1, "应该成功")

	// 第二次调用（应该从缓存获取）
	result2, err2 := wrapper.GetBlockHeight(ctx)
	require.NoError(t, err2, "应该成功")

	assert.Equal(t, result1, result2, "结果应该相同")

	// 验证缓存统计
	stats := cache.GetStats()
	assert.Greater(t, stats["hits"], uint64(0), "应该有缓存命中")
}

// TestHostRuntimePortsWithCache_GetBlockTimestamp 测试GetBlockTimestamp缓存
func TestHostRuntimePortsWithCache_GetBlockTimestamp(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockHostABI := createTestHostRuntimePorts(t)
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()
	executionID := "exec-123"
	hashManager := testutil.NewTestHashManager()

	wrapper := NewHostRuntimePortsWithCache(mockHostABI, cache, executionID, logger, hashManager)

	ctx := context.Background()

	// 第一次调用
	result1, err1 := wrapper.GetBlockTimestamp(ctx)
	require.NoError(t, err1, "应该成功")

	// 第二次调用（应该从缓存获取）
	result2, err2 := wrapper.GetBlockTimestamp(ctx)
	require.NoError(t, err2, "应该成功")

	assert.Equal(t, result1, result2, "结果应该相同")
}

// TestHostRuntimePortsWithCache_GetBlockHash 测试GetBlockHash缓存
func TestHostRuntimePortsWithCache_GetBlockHash(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockHostABI := createTestHostRuntimePorts(t)
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()
	executionID := "exec-123"
	hashManager := testutil.NewTestHashManager()

	wrapper := NewHostRuntimePortsWithCache(mockHostABI, cache, executionID, logger, hashManager)

	ctx := context.Background()
	height := uint64(100)

	// 第一次调用
	result1, err1 := wrapper.GetBlockHash(ctx, height)
	require.NoError(t, err1, "应该成功")

	// 第二次调用（应该从缓存获取）
	result2, err2 := wrapper.GetBlockHash(ctx, height)
	require.NoError(t, err2, "应该成功")

	assert.Equal(t, result1, result2, "结果应该相同")
}

// TestHostRuntimePortsWithCache_GetChainID 测试GetChainID缓存
func TestHostRuntimePortsWithCache_GetChainID(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockHostABI := createTestHostRuntimePorts(t)
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()
	executionID := "exec-123"
	hashManager := testutil.NewTestHashManager()

	wrapper := NewHostRuntimePortsWithCache(mockHostABI, cache, executionID, logger, hashManager)

	ctx := context.Background()

	// 第一次调用
	result1, err1 := wrapper.GetChainID(ctx)
	require.NoError(t, err1, "应该成功")

	// 第二次调用（应该从缓存获取）
	result2, err2 := wrapper.GetChainID(ctx)
	require.NoError(t, err2, "应该成功")

	assert.Equal(t, result1, result2, "结果应该相同")
}

// TestHostRuntimePortsWithCache_UTXOLookup 测试UTXOLookup缓存
func TestHostRuntimePortsWithCache_UTXOLookup(t *testing.T) {
	logger := testutil.NewTestLogger()
	// 创建一个返回有效UTXO的mockHostABI
	mockHostABI := createTestHostRuntimePortsWithUTXO(t)
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()
	executionID := "exec-123"
	hashManager := testutil.NewTestHashManager()

	wrapper := NewHostRuntimePortsWithCache(mockHostABI, cache, executionID, logger, hashManager)

	ctx := context.Background()
	outpoint := &pb.OutPoint{
		TxId:        make([]byte, 32),
		OutputIndex: 0,
	}

	// 第一次调用
	result1, err1 := wrapper.UTXOLookup(ctx, outpoint)
	require.NoError(t, err1, "应该成功")

	// 第二次调用（应该从缓存获取）
	result2, err2 := wrapper.UTXOLookup(ctx, outpoint)
	require.NoError(t, err2, "应该成功")

	assert.Equal(t, result1, result2, "结果应该相同")
}

// TestHostRuntimePortsWithCache_UTXOExists 测试UTXOExists缓存
func TestHostRuntimePortsWithCache_UTXOExists(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockHostABI := createTestHostRuntimePorts(t)
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()
	executionID := "exec-123"
	hashManager := testutil.NewTestHashManager()

	wrapper := NewHostRuntimePortsWithCache(mockHostABI, cache, executionID, logger, hashManager)

	ctx := context.Background()
	outpoint := &pb.OutPoint{
		TxId:        make([]byte, 32),
		OutputIndex: 0,
	}

	// 第一次调用
	result1, err1 := wrapper.UTXOExists(ctx, outpoint)
	require.NoError(t, err1, "应该成功")

	// 第二次调用（应该从缓存获取）
	result2, err2 := wrapper.UTXOExists(ctx, outpoint)
	require.NoError(t, err2, "应该成功")

	assert.Equal(t, result1, result2, "结果应该相同")
}

// TestHostRuntimePortsWithCache_ResourceLookup 测试ResourceLookup缓存
func TestHostRuntimePortsWithCache_ResourceLookup(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockHostABI := createTestHostRuntimePorts(t)
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()
	executionID := "exec-123"
	hashManager := testutil.NewTestHashManager()

	wrapper := NewHostRuntimePortsWithCache(mockHostABI, cache, executionID, logger, hashManager)

	ctx := context.Background()
	contentHash := make([]byte, 32)

	// 第一次调用
	result1, err1 := wrapper.ResourceLookup(ctx, contentHash)
	require.NoError(t, err1, "应该成功")

	// 第二次调用（应该从缓存获取）
	result2, err2 := wrapper.ResourceLookup(ctx, contentHash)
	require.NoError(t, err2, "应该成功")

	assert.Equal(t, result1, result2, "结果应该相同")
}

// TestHostRuntimePortsWithCache_ResourceExists 测试ResourceExists缓存
func TestHostRuntimePortsWithCache_ResourceExists(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockHostABI := createTestHostRuntimePorts(t)
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()
	executionID := "exec-123"
	hashManager := testutil.NewTestHashManager()

	wrapper := NewHostRuntimePortsWithCache(mockHostABI, cache, executionID, logger, hashManager)

	ctx := context.Background()
	contentHash := make([]byte, 32)

	// 第一次调用
	result1, err1 := wrapper.ResourceExists(ctx, contentHash)
	require.NoError(t, err1, "应该成功")

	// 第二次调用（应该从缓存获取）
	result2, err2 := wrapper.ResourceExists(ctx, contentHash)
	require.NoError(t, err2, "应该成功")

	assert.Equal(t, result1, result2, "结果应该相同")
}

// TestHostRuntimePortsWithCache_TxAddInput_InvalidatesCache 测试TxAddInput使缓存失效
func TestHostRuntimePortsWithCache_TxAddInput_InvalidatesCache(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockHostABI := createTestHostRuntimePorts(t)
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()
	executionID := "exec-123"
	hashManager := testutil.NewTestHashManager()

	wrapper := NewHostRuntimePortsWithCache(mockHostABI, cache, executionID, logger, hashManager)

	ctx := context.Background()

	// 先设置一些UTXO相关缓存
	cache.Set("exec-123:UTXO:hash1", true, nil, 0)
	cache.Set("exec-123:UTXO:hash2", true, nil, 0)

	// 调用TxAddInput（应该使UTXO缓存失效）
	outpoint := &pb.OutPoint{
		TxId:        make([]byte, 32),
		OutputIndex: 0,
	}
	_, err := wrapper.TxAddInput(ctx, outpoint, false, nil)
	require.NoError(t, err, "应该成功")

	// 验证UTXO相关缓存被清空
	_, _, found1 := cache.Get("exec-123:UTXO:hash1")
	_, _, found2 := cache.Get("exec-123:UTXO:hash2")
	assert.False(t, found1, "UTXO缓存应该被清空")
	assert.False(t, found2, "UTXO缓存应该被清空")
}

// TestHostRuntimePortsWithCache_TxAddInput_NilOutpoint 测试TxAddInput的nil outpoint处理
func TestHostRuntimePortsWithCache_TxAddInput_NilOutpoint(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockHostABI := createTestHostRuntimePorts(t)
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()
	executionID := "exec-123"
	hashManager := testutil.NewTestHashManager()

	wrapper := NewHostRuntimePortsWithCache(mockHostABI, cache, executionID, logger, hashManager)

	ctx := context.Background()

	// 调用TxAddInput with nil outpoint（应该返回错误，但不使缓存失效）
	_, err := wrapper.TxAddInput(ctx, nil, false, nil)
	assert.Error(t, err, "应该返回错误")
}

// TestHostRuntimePortsWithCache_GetCaller 测试GetCaller缓存
func TestHostRuntimePortsWithCache_GetCaller(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockHostABI := createTestHostRuntimePorts(t)
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()
	executionID := "exec-123"
	hashManager := testutil.NewTestHashManager()

	wrapper := NewHostRuntimePortsWithCache(mockHostABI, cache, executionID, logger, hashManager)

	ctx := context.Background()

	// 第一次调用
	result1, err1 := wrapper.GetCaller(ctx)
	require.NoError(t, err1, "应该成功")

	// 第二次调用（应该从缓存获取）
	result2, err2 := wrapper.GetCaller(ctx)
	require.NoError(t, err2, "应该成功")

	assert.Equal(t, result1, result2, "结果应该相同")
}

// TestHostRuntimePortsWithCache_GetContractAddress 测试GetContractAddress缓存
func TestHostRuntimePortsWithCache_GetContractAddress(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockHostABI := createTestHostRuntimePorts(t)
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()
	executionID := "exec-123"
	hashManager := testutil.NewTestHashManager()

	wrapper := NewHostRuntimePortsWithCache(mockHostABI, cache, executionID, logger, hashManager)

	ctx := context.Background()

	// 第一次调用
	result1, err1 := wrapper.GetContractAddress(ctx)
	require.NoError(t, err1, "应该成功")

	// 第二次调用（应该从缓存获取）
	result2, err2 := wrapper.GetContractAddress(ctx)
	require.NoError(t, err2, "应该成功")

	assert.Equal(t, result1, result2, "结果应该相同")
}

// TestHostRuntimePortsWithCache_GetTransactionID 测试GetTransactionID缓存
func TestHostRuntimePortsWithCache_GetTransactionID(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockHostABI := createTestHostRuntimePorts(t)
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()
	executionID := "exec-123"
	hashManager := testutil.NewTestHashManager()

	wrapper := NewHostRuntimePortsWithCache(mockHostABI, cache, executionID, logger, hashManager)

	ctx := context.Background()

	// 第一次调用
	result1, err1 := wrapper.GetTransactionID(ctx)
	require.NoError(t, err1, "应该成功")

	// 第二次调用（应该从缓存获取）
	result2, err2 := wrapper.GetTransactionID(ctx)
	require.NoError(t, err2, "应该成功")

	assert.Equal(t, result1, result2, "结果应该相同")
}

// TestHostRuntimePortsWithCache_TxAddAssetOutput 测试TxAddAssetOutput（不缓存）
func TestHostRuntimePortsWithCache_TxAddAssetOutput(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockHostABI := createTestHostRuntimePorts(t)
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()
	executionID := "exec-123"
	hashManager := testutil.NewTestHashManager()

	wrapper := NewHostRuntimePortsWithCache(mockHostABI, cache, executionID, logger, hashManager)

	ctx := context.Background()
	owner := make([]byte, 20)
	amount := uint64(1000)
	tokenID := []byte(nil)
	lockingConditions := []*pb.LockingCondition{}

	index, err := wrapper.TxAddAssetOutput(ctx, owner, amount, tokenID, lockingConditions)

	assert.NoError(t, err, "应该成功")
	assert.Equal(t, uint32(0), index, "应该返回输出索引")
}

// TestHostRuntimePortsWithCache_TxAddResourceOutput 测试TxAddResourceOutput（不缓存）
func TestHostRuntimePortsWithCache_TxAddResourceOutput(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockHostABI := createTestHostRuntimePorts(t)
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()
	executionID := "exec-123"
	hashManager := testutil.NewTestHashManager()

	wrapper := NewHostRuntimePortsWithCache(mockHostABI, cache, executionID, logger, hashManager)

	ctx := context.Background()
	contentHash := make([]byte, 32)
	category := "wasm"
	owner := make([]byte, 20)
	lockingConditions := []*pb.LockingCondition{}
	metadata := []byte("test metadata")

	index, err := wrapper.TxAddResourceOutput(ctx, contentHash, category, owner, lockingConditions, metadata)

	assert.NoError(t, err, "应该成功")
	assert.Equal(t, uint32(0), index, "应该返回输出索引")
}

// TestHostRuntimePortsWithCache_TxAddStateOutput 测试TxAddStateOutput（不缓存）
func TestHostRuntimePortsWithCache_TxAddStateOutput(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockHostABI := createTestHostRuntimePorts(t)
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()
	executionID := "exec-123"
	hashManager := testutil.NewTestHashManager()

	wrapper := NewHostRuntimePortsWithCache(mockHostABI, cache, executionID, logger, hashManager)

	ctx := context.Background()
	stateID := []byte("test_state_id")
	stateVersion := uint64(1)
	executionResultHash := make([]byte, 32)
	publicInputs := []byte("public inputs")
	parentStateHash := []byte("parent state hash")

	index, err := wrapper.TxAddStateOutput(ctx, stateID, stateVersion, executionResultHash, publicInputs, parentStateHash)

	assert.NoError(t, err, "应该成功")
	assert.Equal(t, uint32(0), index, "应该返回输出索引")
}

// TestHostRuntimePortsWithCache_EmitEvent 测试EmitEvent（不缓存）
func TestHostRuntimePortsWithCache_EmitEvent(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockHostABI := createTestHostRuntimePorts(t)
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()
	executionID := "exec-123"
	hashManager := testutil.NewTestHashManager()

	wrapper := NewHostRuntimePortsWithCache(mockHostABI, cache, executionID, logger, hashManager)

	ctx := context.Background()

	err := wrapper.EmitEvent(ctx, "test_event", []byte("test-data"))

	assert.NoError(t, err, "应该成功")
}

// TestHostRuntimePortsWithCache_LogDebug 测试LogDebug（不缓存）
func TestHostRuntimePortsWithCache_LogDebug(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockHostABI := createTestHostRuntimePorts(t)
	cache := NewPrimitiveCallCache(logger, 100, 5*time.Minute)
	defer cache.Stop()
	executionID := "exec-123"
	hashManager := testutil.NewTestHashManager()

	wrapper := NewHostRuntimePortsWithCache(mockHostABI, cache, executionID, logger, hashManager)

	ctx := context.Background()

	err := wrapper.LogDebug(ctx, "test debug message")

	assert.NoError(t, err, "应该成功")
}

// createTestHostRuntimePortsWithUTXO 创建返回有效UTXO的测试HostRuntimePorts
func createTestHostRuntimePortsWithUTXO(t *testing.T) *HostRuntimePorts {
	t.Helper()

	logger := testutil.NewTestLogger()
	mockChainQuery := &mockChainQueryForHostABI{}
	mockUTXOQuery := &mockUTXOQueryForHostABI{
		utxo: &utxo.UTXO{
			ContentStrategy: &utxo.UTXO_CachedOutput{
				CachedOutput: &pb.TxOutput{
					Owner: make([]byte, 20),
					OutputContent: &pb.TxOutput_Asset{
						Asset: &pb.AssetOutput{
							AssetContent: &pb.AssetOutput_NativeCoin{
								NativeCoin: &pb.NativeCoinAsset{
									Amount: "100",
								},
							},
						},
					},
				},
			},
		},
	}
	mockCASStorage := &mockCASStorageForHostABI{}
	mockTxQuery := &mockTxQueryForHostABI{}
	mockResourceQuery := &mockResourceQueryForHostABI{}
	mockDraftService := &mockDraftServiceForHostABI{}
	mockHashManager := testutil.NewTestHashManager()
	mockExecCtx := createMockExecutionContextForHostABI()

	hostABI, err := NewHostRuntimePorts(
		logger,
		mockChainQuery,
		&mockBlockQueryForHostABI{},
		mockUTXOQuery,
		mockCASStorage,
		mockTxQuery,
		mockResourceQuery,
		mockDraftService,
		mockHashManager,
		mockExecCtx,
	)
	require.NoError(t, err)

	return hostABI.(*HostRuntimePorts)
}

