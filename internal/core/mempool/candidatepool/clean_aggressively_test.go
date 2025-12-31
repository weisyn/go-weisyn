// Package candidatepool cleanAggressively覆盖率测试
package candidatepool

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/config/candidatepool"
	"github.com/weisyn/v1/internal/core/mempool/testutil"
)

// TestCleanAggressively_WithAggressiveCleanupEnabled_RemovesOldest 测试启用激进清理时移除最旧的
func TestCleanAggressively_WithAggressiveCleanupEnabled_RemovesOldest(t *testing.T) {
	// Arrange
	config := &candidatepool.CandidatePoolOptions{
		MaxCandidates:    5, // 很小的限制
		MaxAge:          10 * time.Minute,
		MemoryLimit:     100 * 1024 * 1024,
		CleanupInterval: 1 * time.Minute,
		AggressiveCleanup: true, // 启用激进清理
	}
	logger := &testutil.MockLogger{}
	eventBus := testutil.NewMockEventBus()
	memory := testutil.NewMockMemoryStore()
	hashService := &testutil.MockBlockHashService{}

	pool, err := NewCandidatePoolWithCache(config, logger, eventBus, memory, hashService, nil)
	require.NoError(t, err)
	cp := pool.(*CandidatePool)

	// 添加多个候选区块（超过限制）
	numBlocks := 8
	for i := 0; i < numBlocks; i++ {
		block := testutil.CreateSimpleTestBlock(uint64(100 + i))
		_, err := cp.AddCandidate(block, "peer1")
		require.NoError(t, err)
		// 设置不同的接收时间（第一个最旧）
		cp.mu.Lock()
		for hashStr, candidate := range cp.candidates {
			if candidate.Height == uint64(100+i) {
				candidate.ReceivedAt = time.Now().Add(-time.Duration(i) * time.Minute)
				_ = hashStr
			}
		}
		cp.mu.Unlock()
	}

	// Act - 手动触发激进清理
	cp.mu.Lock()
	removed := cp.cleanAggressively()
	cp.mu.Unlock()

	// Assert
	assert.Greater(t, removed, 0, "应该清理了一些候选区块")
	// 验证池大小减少
	allCandidates, err := cp.GetAllCandidates()
	assert.NoError(t, err)
	assert.Less(t, len(allCandidates), numBlocks, "候选区块数量应该减少")
}

// TestCleanAggressively_WithAggressiveCleanupDisabled_ReturnsZero 测试禁用激进清理时返回0
func TestCleanAggressively_WithAggressiveCleanupDisabled_ReturnsZero(t *testing.T) {
	// Arrange
	config := &candidatepool.CandidatePoolOptions{
		MaxCandidates:    5,
		MaxAge:          10 * time.Minute,
		MemoryLimit:     100 * 1024 * 1024,
		CleanupInterval: 1 * time.Minute,
		AggressiveCleanup: false, // 禁用激进清理
	}
	logger := &testutil.MockLogger{}
	eventBus := testutil.NewMockEventBus()
	memory := testutil.NewMockMemoryStore()
	hashService := &testutil.MockBlockHashService{}

	pool, err := NewCandidatePoolWithCache(config, logger, eventBus, memory, hashService, nil)
	require.NoError(t, err)
	cp := pool.(*CandidatePool)

	// 添加候选区块
	block := testutil.CreateSimpleTestBlock(100)
	_, err = cp.AddCandidate(block, "peer1")
	require.NoError(t, err)

	// Act - 手动触发激进清理
	cp.mu.Lock()
	removed := cp.cleanAggressively()
	cp.mu.Unlock()

	// Assert
	assert.Equal(t, 0, removed, "禁用激进清理时应该返回0")
}

// TestCleanAggressively_WithEmptyPool_ReturnsZero 测试空池时返回0
func TestCleanAggressively_WithEmptyPool_ReturnsZero(t *testing.T) {
	// Arrange
	config := &candidatepool.CandidatePoolOptions{
		MaxCandidates:    5,
		MaxAge:          10 * time.Minute,
		MemoryLimit:     100 * 1024 * 1024,
		CleanupInterval: 1 * time.Minute,
		AggressiveCleanup: true,
	}
	logger := &testutil.MockLogger{}
	eventBus := testutil.NewMockEventBus()
	memory := testutil.NewMockMemoryStore()
	hashService := &testutil.MockBlockHashService{}

	pool, err := NewCandidatePoolWithCache(config, logger, eventBus, memory, hashService, nil)
	require.NoError(t, err)
	cp := pool.(*CandidatePool)

	// Act - 手动触发激进清理
	cp.mu.Lock()
	removed := cp.cleanAggressively()
	cp.mu.Unlock()

	// Assert
	assert.Equal(t, 0, removed, "空池时应该返回0")
}

// TestCleanAggressively_WithFullPool_Removes25Percent 测试池满时清理25%
func TestCleanAggressively_WithFullPool_Removes25Percent(t *testing.T) {
	// Arrange
	config := &candidatepool.CandidatePoolOptions{
		MaxCandidates:    10,
		MaxAge:          10 * time.Minute,
		MemoryLimit:     100 * 1024 * 1024,
		CleanupInterval: 1 * time.Minute,
		AggressiveCleanup: true,
	}
	logger := &testutil.MockLogger{}
	eventBus := testutil.NewMockEventBus()
	memory := testutil.NewMockMemoryStore()
	hashService := &testutil.MockBlockHashService{}

	pool, err := NewCandidatePoolWithCache(config, logger, eventBus, memory, hashService, nil)
	require.NoError(t, err)
	cp := pool.(*CandidatePool)

	// 添加候选区块直到达到限制
	numBlocks := 10
	for i := 0; i < numBlocks; i++ {
		block := testutil.CreateSimpleTestBlock(uint64(100 + i))
		_, err := cp.AddCandidate(block, "peer1")
		require.NoError(t, err)
		// 设置不同的接收时间
		cp.mu.Lock()
		for _, candidate := range cp.candidates {
			if candidate.Height == uint64(100+i) {
				candidate.ReceivedAt = time.Now().Add(-time.Duration(i) * time.Minute)
			}
		}
		cp.mu.Unlock()
	}

	// Act - 手动触发激进清理
	cp.mu.Lock()
	removed := cp.cleanAggressively()
	cp.mu.Unlock()

	// Assert
	// 应该清理约25%的候选区块（10个中的2-3个）
	assert.GreaterOrEqual(t, removed, 2, "应该至少清理2个候选区块（25%）")
	assert.LessOrEqual(t, removed, 3, "应该最多清理3个候选区块（25%）")
}

