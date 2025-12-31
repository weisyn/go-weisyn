// Package candidatepool GetCandidates覆盖率测试
package candidatepool

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/mempool/testutil"
)

// TestGetCandidatesForHeight_WithExistingCandidates_ReturnsCandidates 测试获取存在的候选区块
func TestGetCandidatesForHeight_WithExistingCandidates_ReturnsCandidates(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	height := uint64(100)
	block1 := testutil.CreateSimpleTestBlock(height)
	block2 := testutil.CreateSimpleTestBlock(height)

	// 添加候选区块
	_, err1 := pool.AddCandidate(block1, "peer1")
	require.NoError(t, err1)
	_, err2 := pool.AddCandidate(block2, "peer2")
	require.NoError(t, err2)

	// Act
	candidates, err := pool.GetCandidatesForHeight(height, 100*time.Millisecond)

	// Assert
	assert.NoError(t, err, "应该成功获取候选区块")
	assert.GreaterOrEqual(t, len(candidates), 2, "应该返回至少2个候选区块")
}

// TestGetCandidatesForHeight_WithNoCandidates_WaitsAndReturnsEmpty 测试没有候选区块时等待
func TestGetCandidatesForHeight_WithNoCandidates_WaitsAndReturnsEmpty(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	height := uint64(999)

	// Act - 使用很短的超时时间
	candidates, err := pool.GetCandidatesForHeight(height, 50*time.Millisecond)

	// Assert
	// 根据实现，可能返回超时错误或空列表
	if err != nil {
		assert.Equal(t, ErrTimeout, err, "应该返回超时错误")
	} else {
		assert.Empty(t, candidates, "应该返回空列表")
	}
}

// TestGetAllCandidates_WithEmptyPool_ReturnsEmpty 测试空池返回空列表
func TestGetAllCandidates_WithEmptyPool_ReturnsEmpty(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)

	// Act
	candidates, err := pool.GetAllCandidates()

	// Assert
	assert.NoError(t, err, "应该成功获取候选区块")
	assert.Empty(t, candidates, "应该返回空列表")
}

// TestGetAllCandidates_WithMultipleCandidates_ReturnsAll 测试获取所有候选区块
func TestGetAllCandidates_WithMultipleCandidates_ReturnsAll(t *testing.T) {
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
	candidates, err := pool.GetAllCandidates()

	// Assert
	assert.NoError(t, err, "应该成功获取候选区块")
	assert.Equal(t, numBlocks, len(candidates), "应该返回所有候选区块")
}

// TestWaitForCandidates_WithEnoughCandidates_ReturnsImmediately 测试已有足够候选区块时立即返回
func TestWaitForCandidates_WithEnoughCandidates_ReturnsImmediately(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	minCount := 3

	// 添加足够的候选区块
	for i := 0; i < minCount; i++ {
		block := testutil.CreateSimpleTestBlock(uint64(100 + i))
		_, err := pool.AddCandidate(block, "peer1")
		require.NoError(t, err)
	}

	// Act
	candidates, err := pool.WaitForCandidates(minCount, 100*time.Millisecond)

	// Assert
	assert.NoError(t, err, "应该成功获取候选区块")
	assert.GreaterOrEqual(t, len(candidates), minCount, "应该返回至少minCount个候选区块")
}

// TestWaitForCandidates_WithInsufficientCandidates_ReturnsTimeout 测试候选区块不足时超时
func TestWaitForCandidates_WithInsufficientCandidates_ReturnsTimeout(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	minCount := 10

	// 添加少量候选区块
	for i := 0; i < 2; i++ {
		block := testutil.CreateSimpleTestBlock(uint64(100 + i))
		_, err := pool.AddCandidate(block, "peer1")
		require.NoError(t, err)
	}

	// Act - 使用很短的超时时间
	candidates, err := pool.WaitForCandidates(minCount, 50*time.Millisecond)

	// Assert
	// 根据实现，可能返回超时错误或部分候选区块
	if err != nil {
		assert.Equal(t, ErrTimeout, err, "应该返回超时错误")
	} else {
		assert.Less(t, len(candidates), minCount, "返回的候选区块数量应该少于minCount")
	}
}

// TestWaitForCandidates_WithPoolClosed_ReturnsError 测试池关闭后等待
func TestWaitForCandidates_WithPoolClosed_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	minCount := 5

	// 启动池
	err := pool.Start()
	require.NoError(t, err)

	// 关闭池
	err = pool.Stop()
	require.NoError(t, err)

	// Act
	candidates, err := pool.WaitForCandidates(minCount, 100*time.Millisecond)

	// Assert
	assert.Error(t, err, "池关闭后应该返回错误")
	assert.Nil(t, candidates, "候选区块列表应为nil")
	assert.Equal(t, ErrPoolClosed, err, "应该返回池关闭错误")
}

