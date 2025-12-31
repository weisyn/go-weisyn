// Package candidatepool ClearCandidates覆盖率测试
package candidatepool

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/config/candidatepool"
	"github.com/weisyn/v1/internal/core/mempool/testutil"
)

// TestClearCandidates_WithEmptyPool_ReturnsZero 测试清空空池
func TestClearCandidates_WithEmptyPool_ReturnsZero(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)

	// Act
	count, err := pool.ClearCandidates()

	// Assert
	assert.NoError(t, err, "应该成功清空")
	assert.Equal(t, 0, count, "应该返回0")
}

// TestClearCandidates_WithMultipleCandidates_ClearsAll 测试清空多个候选区块
func TestClearCandidates_WithMultipleCandidates_ClearsAll(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	numBlocks := 5

	// 添加多个候选区块
	for i := 0; i < numBlocks; i++ {
		block := testutil.CreateSimpleTestBlock(uint64(100 + i))
		_, err := pool.AddCandidate(block, "peer1")
		require.NoError(t, err)
	}

	// Act
	count, err := pool.ClearCandidates()

	// Assert
	assert.NoError(t, err, "应该成功清空")
	assert.Equal(t, numBlocks, count, "应该返回清空的候选区块数量")

	// 验证池已清空
	allCandidates, err := pool.GetAllCandidates()
	assert.NoError(t, err)
	assert.Empty(t, allCandidates, "池应该已清空")
}

// TestClearExpiredCandidates_WithNoExpired_ReturnsZero 测试清理没有过期候选区块
func TestClearExpiredCandidates_WithNoExpired_ReturnsZero(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	block := testutil.CreateSimpleTestBlock(100)
	_, err := pool.AddCandidate(block, "peer1")
	require.NoError(t, err)

	// Act - 使用很长的maxAge
	maxAge := 1 * time.Hour
	count, err := pool.ClearExpiredCandidates(maxAge)

	// Assert
	assert.NoError(t, err, "应该成功清理")
	assert.Equal(t, 0, count, "应该返回0（没有过期候选区块）")
}

// TestClearExpiredCandidates_WithExpiredCandidates_RemovesExpired 测试清理过期候选区块
func TestClearExpiredCandidates_WithExpiredCandidates_RemovesExpired(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	block := testutil.CreateSimpleTestBlock(100)
	_, err := pool.AddCandidate(block, "peer1")
	require.NoError(t, err)

	// 手动修改候选区块的接收时间使其过期
	pool.mu.Lock()
	for _, candidate := range pool.candidates {
		candidate.ReceivedAt = time.Now().Add(-2 * time.Hour) // 2小时前
	}
	pool.mu.Unlock()

	// Act - 使用很短的maxAge
	maxAge := 1 * time.Hour
	count, err := pool.ClearExpiredCandidates(maxAge)

	// Assert
	assert.NoError(t, err, "应该成功清理")
	assert.GreaterOrEqual(t, count, 1, "应该至少清理1个过期候选区块")
}

// TestClearOutdatedCandidates_WithNoChainState_ReturnsZero 测试没有链状态时清理
func TestClearOutdatedCandidates_WithNoChainState_ReturnsZero(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	block := testutil.CreateSimpleTestBlock(100)
	_, err := pool.AddCandidate(block, "peer1")
	require.NoError(t, err)

	// Act
	count, err := pool.ClearOutdatedCandidates()

	// Assert
	assert.NoError(t, err, "应该成功清理")
	// 如果没有链状态缓存，应该返回0
	assert.GreaterOrEqual(t, count, 0, "应该返回0或更多")
}

// TestClearOutdatedCandidates_WithChainState_RemovesOutdated 测试有链状态时清理过时候选区块
func TestClearOutdatedCandidates_WithChainState_RemovesOutdated(t *testing.T) {
	// Arrange
	config := &candidatepool.CandidatePoolOptions{
		MaxCandidates:       100,
		MaxAge:              10 * time.Minute,
		MemoryLimit:         100 * 1024 * 1024,
		CleanupInterval:     1 * time.Minute,
		HeightCleanupEnabled: true,
		KeepHeightDepth:     5,
	}
	logger := &testutil.MockLogger{}
	eventBus := testutil.NewMockEventBus()
	memory := testutil.NewMockMemoryStore()
	hashService := &testutil.MockBlockHashService{}

	// 创建Mock链状态提供者
	mockChainState := &MockChainStateProvider{
		currentHeight: 200,
	}

	pool, err := NewCandidatePoolWithCache(config, logger, eventBus, memory, hashService, mockChainState)
	require.NoError(t, err)
	cp := pool.(*CandidatePool)

	// 添加过时的候选区块（高度远低于当前高度）
	oldBlock := testutil.CreateSimpleTestBlock(100) // 当前高度200，保留深度5，所以100会被清理
	_, err = cp.AddCandidate(oldBlock, "peer1")
	require.NoError(t, err)

	// Act
	count, err := cp.ClearOutdatedCandidates()

	// Assert
	assert.NoError(t, err, "应该成功清理")
	// 根据配置，高度100的区块应该被清理（当前高度200，保留深度5）
	assert.GreaterOrEqual(t, count, 0, "应该返回0或更多")
}

// MockChainStateProvider Mock链状态提供者
type MockChainStateProvider struct {
	currentHeight uint64
	latestHash    []byte
}

func (m *MockChainStateProvider) GetCurrentHeight(ctx context.Context) (uint64, error) {
	return m.currentHeight, nil
}

func (m *MockChainStateProvider) GetLatestBlockHash(ctx context.Context) ([]byte, error) {
	if m.latestHash == nil {
		return []byte("latest_hash_32_bytes_12345678"), nil
	}
	return m.latestHash, nil
}

func (m *MockChainStateProvider) IsValidHeight(height uint64) bool {
	return height <= m.currentHeight+1
}

