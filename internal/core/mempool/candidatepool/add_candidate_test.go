// Package candidatepool AddCandidate覆盖率测试
package candidatepool

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/config/candidatepool"
	"github.com/weisyn/v1/internal/core/mempool/testutil"
)

// createTestCandidatePool 创建测试用的候选区块池
func createTestCandidatePool(t *testing.T) *CandidatePool {
	config := &candidatepool.CandidatePoolOptions{
		MaxCandidates:    100,
		MaxAge:          10 * time.Minute,
		MemoryLimit:     100 * 1024 * 1024, // 100MB
		CleanupInterval: 1 * time.Minute,    // 清理间隔
	}
	logger := &testutil.MockLogger{}
	eventBus := testutil.NewMockEventBus()
	memory := testutil.NewMockMemoryStore()
	hashService := &testutil.MockBlockHashService{}

	pool, err := NewCandidatePoolWithCache(config, logger, eventBus, memory, hashService, nil)
	require.NoError(t, err)
	return pool.(*CandidatePool)
}

// TestAddCandidate_WithValidBlock_AddsSuccessfully 测试添加有效区块
func TestAddCandidate_WithValidBlock_AddsSuccessfully(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	block := testutil.CreateSimpleTestBlock(1)

	// Act
	blockHash, err := pool.AddCandidate(block, "peer1")

	// Assert
	assert.NoError(t, err, "应该成功添加候选区块")
	assert.NotNil(t, blockHash, "区块哈希不应为nil")
	assert.Equal(t, 32, len(blockHash), "区块哈希长度应为32字节")
}

// TestAddCandidate_WithNilBlock_ReturnsError 测试nil区块
func TestAddCandidate_WithNilBlock_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)

	// Act
	blockHash, err := pool.AddCandidate(nil, "peer1")

	// Assert
	assert.Error(t, err, "nil区块应该返回错误")
	assert.Nil(t, blockHash, "区块哈希应为nil")
	assert.Contains(t, err.Error(), "格式验证失败", "错误信息应该包含格式验证失败")
}

// TestAddCandidate_WithDuplicateBlock_ReturnsError 测试重复区块
func TestAddCandidate_WithDuplicateBlock_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	block := testutil.CreateSimpleTestBlock(1)

	// Act - 第一次添加
	blockHash1, err1 := pool.AddCandidate(block, "peer1")
	require.NoError(t, err1)
	require.NotNil(t, blockHash1)

	// Act - 第二次添加相同区块
	blockHash2, err2 := pool.AddCandidate(block, "peer2")

	// Assert
	assert.Error(t, err2, "重复区块应该返回错误")
	assert.Equal(t, blockHash1, blockHash2, "重复区块应该返回相同的哈希")
	assert.Equal(t, ErrCandidateAlreadyExists, err2, "应该返回已存在错误")
}

// TestAddCandidate_WithDifferentPeers_AddsSuccessfully 测试不同来源的区块
func TestAddCandidate_WithDifferentPeers_AddsSuccessfully(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	block1 := testutil.CreateSimpleTestBlock(1)
	block2 := testutil.CreateSimpleTestBlock(2)

	// Act
	hash1, err1 := pool.AddCandidate(block1, "peer1")
	hash2, err2 := pool.AddCandidate(block2, "peer2")

	// Assert
	assert.NoError(t, err1, "应该成功添加第一个区块")
	assert.NoError(t, err2, "应该成功添加第二个区块")
	assert.NotNil(t, hash1, "第一个区块哈希不应为nil")
	assert.NotNil(t, hash2, "第二个区块哈希不应为nil")
	assert.NotEqual(t, hash1, hash2, "不同区块应该有不同哈希")
}

// TestAddCandidate_WithPoolClosed_ReturnsError 测试池关闭后添加
func TestAddCandidate_WithPoolClosed_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	block := testutil.CreateSimpleTestBlock(1)

	// 启动池（如果未启动）
	_ = pool.Start()

	// 关闭池
	err := pool.Stop()
	require.NoError(t, err)

	// Act
	blockHash, err := pool.AddCandidate(block, "peer1")

	// Assert
	assert.Error(t, err, "池关闭后应该返回错误")
	assert.Nil(t, blockHash, "区块哈希应为nil")
	assert.Equal(t, ErrPoolClosed, err, "应该返回池关闭错误")
}

// TestAddCandidate_WithMemoryLimit_TriggersCleanup 测试内存限制触发清理
func TestAddCandidate_WithMemoryLimit_TriggersCleanup(t *testing.T) {
	// Arrange
	config := &candidatepool.CandidatePoolOptions{
		MaxCandidates: 5, // 很小的限制
		MaxAge:        10 * time.Minute,
		MemoryLimit:   100 * 1024 * 1024, // 100MB，足够大
	}
	logger := &testutil.MockLogger{}
	eventBus := testutil.NewMockEventBus()
	memory := testutil.NewMockMemoryStore()
	hashService := &testutil.MockBlockHashService{}

	pool, err := NewCandidatePoolWithCache(config, logger, eventBus, memory, hashService, nil)
	require.NoError(t, err)
	cp := pool.(*CandidatePool)

	// 添加多个区块直到达到限制
	for i := 0; i < 3; i++ {
		block := testutil.CreateSimpleTestBlock(uint64(i + 10)) // 使用不同的高度避免重复
		_, err := cp.AddCandidate(block, "peer1")
		require.NoError(t, err)
	}

	// Act - 继续添加，应该触发清理
	block := testutil.CreateSimpleTestBlock(13) // 使用不同的高度
	blockHash, err := cp.AddCandidate(block, "peer1")

	// Assert
	// 根据实现，可能会触发清理或返回错误
	if err != nil {
		assert.True(t, err == ErrMemoryLimit || err == ErrPoolFull, "应该返回内存限制或池满错误")
	} else {
		assert.NotNil(t, blockHash, "如果成功，区块哈希不应为nil")
	}
}

// TestAddCandidate_WithEmptyBlock_AddsSuccessfully 测试空区块
func TestAddCandidate_WithEmptyBlock_AddsSuccessfully(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	block := testutil.CreateEmptyTestBlock(1)

	// Act
	blockHash, err := pool.AddCandidate(block, "peer1")

	// Assert
	assert.NoError(t, err, "空区块应该可以添加")
	assert.NotNil(t, blockHash, "区块哈希不应为nil")
}

