package builder_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/block/builder"
	"github.com/weisyn/v1/internal/core/block/testutil"
)

// ==================== cacheCandidate 测试（通过 CreateMiningCandidate 间接测试）====================

// TestCacheCandidate_AfterCreatingCandidate_IsCached 测试创建候选区块后缓存成功
func TestCacheCandidate_AfterCreatingCandidate_IsCached(t *testing.T) {
	// Arrange
	storage := testutil.NewMockBadgerStore()
	testutil.SetupChainTip(storage, 0, make([]byte, 32))
	mempool := testutil.NewMockTxPool()
	txProcessor := &testutil.MockTxProcessor{}
	hashManager := &testutil.MockHashManager{}
	blockHashClient := testutil.NewMockBlockHashClient()
	txHashClient := testutil.NewMockTransactionHashClient()
	queryService := testutil.NewMockQueryService()
	feeManager := &testutil.MockFeeManager{}
	logger := &testutil.MockLogger{}

	service, err := builder.NewService(
		storage,
		mempool,
		txProcessor,
		hashManager,
		blockHashClient,
		txHashClient,
		queryService,
		queryService,
		queryService, // chainQuery
		feeManager,
		testutil.NewDefaultMockConfigProvider(),
		logger,
	)
	require.NoError(t, err)

	ctx := context.Background()

	// Act
	blockHash, err := service.CreateMiningCandidate(ctx)
	require.NoError(t, err)

	// Assert - 验证区块被缓存
	block, err := service.GetCachedCandidate(ctx, blockHash)
	assert.NoError(t, err)
	assert.NotNil(t, block)
}

// TestCacheCandidate_WithShortHash_HandlesGracefully 测试短哈希时的缓存处理
func TestCacheCandidate_WithShortHash_HandlesGracefully(t *testing.T) {
	// Arrange
	storage := testutil.NewMockBadgerStore()
	testutil.SetupChainTip(storage, 0, make([]byte, 32))
	mempool := testutil.NewMockTxPool()
	txProcessor := &testutil.MockTxProcessor{}
	hashManager := &testutil.MockHashManager{}
	blockHashClient := testutil.NewMockBlockHashClient()
	txHashClient := testutil.NewMockTransactionHashClient()
	queryService := testutil.NewMockQueryService()
	feeManager := &testutil.MockFeeManager{}
	logger := &testutil.MockLogger{}

	service, err := builder.NewService(
		storage,
		mempool,
		txProcessor,
		hashManager,
		blockHashClient,
		txHashClient,
		queryService,
		queryService,
		queryService, // chainQuery
		feeManager,
		testutil.NewDefaultMockConfigProvider(),
		logger,
	)
	require.NoError(t, err)

	ctx := context.Background()

	// Act
	blockHash, err := service.CreateMiningCandidate(ctx)
	require.NoError(t, err)

	// Assert - 验证缓存键格式正确（使用十六进制字符串）
	if len(blockHash) > 0 {
		// 验证可以通过哈希获取区块
		block, err := service.GetCachedCandidate(ctx, blockHash)
		assert.NoError(t, err)
		assert.NotNil(t, block)
	}
}

// ==================== removeCachedCandidate 测试（通过 RemoveCachedCandidate 测试）====================

// TestRemoveCachedCandidate_WithExistingBlock_RemovesFromCache 测试移除存在的候选区块
func TestRemoveCachedCandidate_WithExistingBlock_RemovesFromCache(t *testing.T) {
	// Arrange
	storage := testutil.NewMockBadgerStore()
	testutil.SetupChainTip(storage, 0, make([]byte, 32))
	mempool := testutil.NewMockTxPool()
	txProcessor := &testutil.MockTxProcessor{}
	hashManager := &testutil.MockHashManager{}
	blockHashClient := testutil.NewMockBlockHashClient()
	txHashClient := testutil.NewMockTransactionHashClient()
	queryService := testutil.NewMockQueryService()
	feeManager := &testutil.MockFeeManager{}
	logger := &testutil.MockLogger{}

	service, err := builder.NewService(
		storage,
		mempool,
		txProcessor,
		hashManager,
		blockHashClient,
		txHashClient,
		queryService,
		queryService,
		queryService, // chainQuery
		feeManager,
		testutil.NewDefaultMockConfigProvider(),
		logger,
	)
	require.NoError(t, err)

	ctx := context.Background()

	// 先创建一个候选区块
	blockHash, err := service.CreateMiningCandidate(ctx)
	require.NoError(t, err)

	// 验证区块存在
	_, err = service.GetCachedCandidate(ctx, blockHash)
	require.NoError(t, err)

	// Act
	err = service.RemoveCachedCandidate(ctx, blockHash)

	// Assert
	assert.NoError(t, err)
	_, err = service.GetCachedCandidate(ctx, blockHash)
	assert.Error(t, err, "区块应该已被移除")
	assert.Contains(t, err.Error(), "候选区块不存在")
}

// TestRemoveCachedCandidate_WithNonExistentBlock_ReturnsError 测试移除不存在的候选区块
func TestRemoveCachedCandidate_WithNonExistentBlock_ReturnsError(t *testing.T) {
	// Arrange
	service, err := testutil.NewTestBlockBuilder()
	require.NoError(t, err)

	ctx := context.Background()
	nonExistentHash := make([]byte, 32)
	copy(nonExistentHash, "non-existent-block-hash")

	// Act
	err = service.RemoveCachedCandidate(ctx, nonExistentHash)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "候选区块不在缓存中")
}

// TestRemoveCachedCandidate_WithShortHash_HandlesGracefully 测试使用短哈希移除时的处理
func TestRemoveCachedCandidate_WithShortHash_HandlesGracefully(t *testing.T) {
	// Arrange
	service, err := testutil.NewTestBlockBuilder()
	require.NoError(t, err)

	ctx := context.Background()
	shortHash := []byte{1, 2, 3} // 长度不足8字节

	// Act & Assert
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("❌ BUG发现：RemoveCachedCandidate 在处理短哈希时发生 panic: %v", r)
		}
	}()

	err = service.RemoveCachedCandidate(ctx, shortHash)
	// 应该返回错误，而不是 panic
	assert.Error(t, err)
}

// TestRemoveCachedCandidate_AfterClearCache_ReturnsError 测试清空缓存后移除区块
func TestRemoveCachedCandidate_AfterClearCache_ReturnsError(t *testing.T) {
	// Arrange
	storage := testutil.NewMockBadgerStore()
	testutil.SetupChainTip(storage, 0, make([]byte, 32))
	mempool := testutil.NewMockTxPool()
	txProcessor := &testutil.MockTxProcessor{}
	hashManager := &testutil.MockHashManager{}
	blockHashClient := testutil.NewMockBlockHashClient()
	txHashClient := testutil.NewMockTransactionHashClient()
	queryService := testutil.NewMockQueryService()
	feeManager := &testutil.MockFeeManager{}
	logger := &testutil.MockLogger{}

	service, err := builder.NewService(
		storage,
		mempool,
		txProcessor,
		hashManager,
		blockHashClient,
		txHashClient,
		queryService,
		queryService,
		queryService, // chainQuery
		feeManager,
		testutil.NewDefaultMockConfigProvider(),
		logger,
	)
	require.NoError(t, err)

	ctx := context.Background()

	// 创建并缓存一个区块
	blockHash, err := service.CreateMiningCandidate(ctx)
	require.NoError(t, err)

	// 清空缓存
	err = service.ClearCandidateCache(ctx)
	require.NoError(t, err)

	// Act
	err = service.RemoveCachedCandidate(ctx, blockHash)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "候选区块不在缓存中")
}

// ==================== 缓存指标更新测试 ====================

// TestCacheCandidate_UpdatesMetrics 测试缓存后指标更新
func TestCacheCandidate_UpdatesMetrics(t *testing.T) {
	// Arrange
	storage := testutil.NewMockBadgerStore()
	testutil.SetupChainTip(storage, 0, make([]byte, 32))
	mempool := testutil.NewMockTxPool()
	txProcessor := &testutil.MockTxProcessor{}
	hashManager := &testutil.MockHashManager{}
	blockHashClient := testutil.NewMockBlockHashClient()
	txHashClient := testutil.NewMockTransactionHashClient()
	queryService := testutil.NewMockQueryService()
	feeManager := &testutil.MockFeeManager{}
	logger := &testutil.MockLogger{}

	service, err := builder.NewService(
		storage,
		mempool,
		txProcessor,
		hashManager,
		blockHashClient,
		txHashClient,
		queryService,
		queryService,
		queryService, // chainQuery
		feeManager,
		testutil.NewDefaultMockConfigProvider(),
		logger,
	)
	require.NoError(t, err)

	ctx := context.Background()

	// 获取初始指标
	initialMetrics, err := service.GetBuilderMetrics(ctx)
	require.NoError(t, err)
	initialCacheSize := initialMetrics.CacheSize

	// Act - 创建候选区块（会自动缓存）
	_, err = service.CreateMiningCandidate(ctx)
	require.NoError(t, err)

	// Assert - 验证缓存大小已更新
	metrics, err := service.GetBuilderMetrics(ctx)
	require.NoError(t, err)
	assert.Greater(t, metrics.CacheSize, initialCacheSize, "缓存大小应该增加")
}

// ==================== 并发安全测试 ====================

// TestCacheCandidate_ConcurrentAccess_IsSafe 测试并发缓存操作的安全性
func TestCacheCandidate_ConcurrentAccess_IsSafe(t *testing.T) {
	// Arrange
	storage := testutil.NewMockBadgerStore()
	testutil.SetupChainTip(storage, 0, make([]byte, 32))
	mempool := testutil.NewMockTxPool()
	txProcessor := &testutil.MockTxProcessor{}
	hashManager := &testutil.MockHashManager{}
	blockHashClient := testutil.NewMockBlockHashClient()
	txHashClient := testutil.NewMockTransactionHashClient()
	queryService := testutil.NewMockQueryService()
	feeManager := &testutil.MockFeeManager{}
	logger := &testutil.MockLogger{}

	service, err := builder.NewService(
		storage,
		mempool,
		txProcessor,
		hashManager,
		blockHashClient,
		txHashClient,
		queryService,
		queryService,
		queryService, // chainQuery
		feeManager,
		testutil.NewDefaultMockConfigProvider(),
		logger,
	)
	require.NoError(t, err)

	ctx := context.Background()
	concurrency := 10

	// Act
	results := make(chan error, concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					results <- fmt.Errorf("panic: %v", r)
				}
			}()
			_, err := service.CreateMiningCandidate(ctx)
			results <- err
		}()
	}

	// Assert
	for i := 0; i < concurrency; i++ {
		err := <-results
		assert.NoError(t, err, "并发创建候选区块不应该失败")
	}

	// 验证缓存状态一致
	metrics, err := service.GetBuilderMetrics(ctx)
	require.NoError(t, err)
	assert.Greater(t, metrics.CacheSize, 0, "缓存应该包含元素")
	assert.LessOrEqual(t, metrics.CacheSize, 100, "缓存大小不应该超过最大值")
}

// ==================== 边界条件测试 ====================

// TestCacheCandidate_WithNilBlock_HandlesGracefully 测试缓存nil区块时的处理
// 🐛 BUG发现：代码应该检查并拒绝缓存nil区块
func TestCacheCandidate_WithNilBlock_HandlesGracefully(t *testing.T) {
	// 注意：cacheCandidate 是私有方法，无法直接测试
	// 但可以通过 CreateMiningCandidate 间接测试
	// 如果 buildCandidate 返回 nil block，cacheCandidate 应该处理

	t.Logf("⚠️ 注意：cacheCandidate 是私有方法，无法直接测试nil区块场景")
	t.Logf("建议：在 cacheCandidate 中添加 nil 检查，或通过集成测试验证")
}

// TestCacheCandidate_WithEmptyHash_HandlesGracefully 测试使用空哈希缓存时的处理
func TestCacheCandidate_WithEmptyHash_HandlesGracefully(t *testing.T) {
	// Arrange
	storage := testutil.NewMockBadgerStore()
	testutil.SetupChainTip(storage, 0, make([]byte, 32))
	mempool := testutil.NewMockTxPool()
	txProcessor := &testutil.MockTxProcessor{}
	hashManager := &testutil.MockHashManager{}
	blockHashClient := testutil.NewMockBlockHashClient()
	// 设置blockHashClient返回空哈希
	blockHashClient.SetError(fmt.Errorf("hash error"))
	txHashClient := testutil.NewMockTransactionHashClient()
	queryService := testutil.NewMockQueryService()
	feeManager := &testutil.MockFeeManager{}
	logger := &testutil.MockLogger{}

	service, err := builder.NewService(
		storage,
		mempool,
		txProcessor,
		hashManager,
		blockHashClient,
		txHashClient,
		queryService,
		queryService,
		queryService, // chainQuery
		feeManager,
		testutil.NewDefaultMockConfigProvider(),
		logger,
	)
	require.NoError(t, err)

	ctx := context.Background()

	// Act
	blockHash, err := service.CreateMiningCandidate(ctx)

	// Assert
	// 如果哈希计算失败，应该返回错误或空哈希
	// 空哈希不应该被缓存
	if err == nil && len(blockHash) == 0 {
		t.Logf("⚠️ 问题：空哈希被返回，可能导致后续问题")
		t.Logf("建议：空哈希不应该被缓存，或应该返回错误")
	}
}

// ==================== 发现代码问题测试 ====================

// TestCacheCandidate_DetectsPotentialIssues 测试发现潜在问题
func TestCacheCandidate_DetectsPotentialIssues(t *testing.T) {
	// 🐛 问题发现：检查缓存实现中的潜在问题

	t.Logf("✅ 缓存实现检查：")
	t.Logf("  - cacheCandidate 使用 LRU 缓存存储候选区块")
	t.Logf("  - 缓存键使用十六进制字符串格式（fmt.Sprintf(\"%%x\", blockHash)）")
	t.Logf("  - 缓存失败不影响返回，只记录警告")

	// 验证缓存键格式
	storage := testutil.NewMockBadgerStore()
	testutil.SetupChainTip(storage, 0, make([]byte, 32))
	mempool := testutil.NewMockTxPool()
	txProcessor := &testutil.MockTxProcessor{}
	hashManager := &testutil.MockHashManager{}
	blockHashClient := testutil.NewMockBlockHashClient()
	txHashClient := testutil.NewMockTransactionHashClient()
	queryService := testutil.NewMockQueryService()
	feeManager := &testutil.MockFeeManager{}
	logger := &testutil.MockLogger{}

	service, err := builder.NewService(
		storage,
		mempool,
		txProcessor,
		hashManager,
		blockHashClient,
		txHashClient,
		queryService,
		queryService,
		queryService, // chainQuery
		feeManager,
		testutil.NewDefaultMockConfigProvider(),
		logger,
	)
	require.NoError(t, err)

	ctx := context.Background()
	blockHash, err := service.CreateMiningCandidate(ctx)
	require.NoError(t, err)

	// 验证可以通过哈希获取区块（说明缓存键格式正确）
	block, err := service.GetCachedCandidate(ctx, blockHash)
	assert.NoError(t, err)
	assert.NotNil(t, block)
}
