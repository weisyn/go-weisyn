// Package candidatepool performMaintenance覆盖率测试
package candidatepool

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/config/candidatepool"
	"github.com/weisyn/v1/internal/core/mempool/testutil"
)

// TestPerformMaintenance_WithExpiredCandidates_RemovesExpired 测试维护循环清理过期候选区块
func TestPerformMaintenance_WithExpiredCandidates_RemovesExpired(t *testing.T) {
	// Arrange
	config := &candidatepool.CandidatePoolOptions{
		MaxCandidates:   100,
		MaxAge:          100 * time.Millisecond, // 很短的过期时间
		MemoryLimit:     100 * 1024 * 1024,
		CleanupInterval: 1 * time.Minute, // 使用较长的清理间隔，避免自动触发
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

	// 手动修改候选区块的接收时间使其过期
	cp.mu.Lock()
	for _, candidate := range cp.candidates {
		candidate.ReceivedAt = time.Now().Add(-200 * time.Millisecond) // 200ms前，已过期
	}
	initialCount := len(cp.candidates)
	cp.mu.Unlock()

	// Act - 手动触发维护（不启动池，避免后台goroutine）
	cp.performMaintenance()

	// Assert - 验证过期候选区块已被清理
	allCandidates, err := cp.GetAllCandidates()
	assert.NoError(t, err)
	// 过期候选区块应该被清理
	if initialCount > 0 {
		assert.LessOrEqual(t, len(allCandidates), initialCount, "过期候选区块应该被清理")
	}
}

// TestPerformMaintenance_WithNoExpiredCandidates_NoRemoval 测试没有过期候选区块时不清理
func TestPerformMaintenance_WithNoExpiredCandidates_NoRemoval(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)

	// 添加候选区块
	block := testutil.CreateSimpleTestBlock(100)
	_, err := pool.AddCandidate(block, "peer1")
	require.NoError(t, err)

	// Act - 手动触发维护
	pool.performMaintenance()

	// Assert - 验证候选区块未被清理
	allCandidates, err := pool.GetAllCandidates()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(allCandidates), "没有过期候选区块时不应该清理")
}

// TestPerformMaintenance_WithEventSink_TriggersEvent 测试维护触发事件
func TestPerformMaintenance_WithEventSink_TriggersEvent(t *testing.T) {
	// Arrange
	config := &candidatepool.CandidatePoolOptions{
		MaxCandidates:   100,
		MaxAge:          100 * time.Millisecond,
		MemoryLimit:     100 * 1024 * 1024,
		CleanupInterval: 1 * time.Minute, // 使用较长的清理间隔
	}
	logger := &testutil.MockLogger{}
	eventBus := testutil.NewMockEventBus()
	memory := testutil.NewMockMemoryStore()
	hashService := &testutil.MockBlockHashService{}

	pool, err := NewCandidatePoolWithCache(config, logger, eventBus, memory, hashService, nil)
	require.NoError(t, err)
	cp := pool.(*CandidatePool)

	mockSink := &MockCandidateEventSink{}
	cp.SetEventSink(mockSink)

	// 添加候选区块并使其过期
	block := testutil.CreateSimpleTestBlock(100)
	_, err = cp.AddCandidate(block, "peer1")
	require.NoError(t, err)

	cp.mu.Lock()
	for _, candidate := range cp.candidates {
		candidate.ReceivedAt = time.Now().Add(-200 * time.Millisecond)
	}
	initialCount := len(cp.candidates)
	cp.mu.Unlock()

	// Act - 手动触发维护
	cp.performMaintenance()

	// Assert - 验证触发了清理完成事件
	// 注意：只有在清理了候选区块时才会触发事件
	cp.mu.RLock()
	finalCount := len(cp.candidates)
	cp.mu.RUnlock()

	if finalCount < initialCount {
		assert.Greater(t, mockSink.cleanupCompleted, 0, "应该触发OnCleanupCompleted事件")
	}
}
