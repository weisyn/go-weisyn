// Package candidatepool 查询方法覆盖率测试
package candidatepool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/mempool/testutil"
)

// TestGetCandidateHashes_WithEmptyPool_ReturnsEmpty 测试空池返回空哈希列表
func TestGetCandidateHashes_WithEmptyPool_ReturnsEmpty(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)

	// Act
	hashes, err := pool.GetCandidateHashes()

	// Assert
	assert.NoError(t, err, "应该成功获取哈希列表")
	assert.Empty(t, hashes, "应该返回空哈希列表")
}

// TestGetCandidateHashes_WithMultipleCandidates_ReturnsAllHashes 测试获取所有候选区块哈希
func TestGetCandidateHashes_WithMultipleCandidates_ReturnsAllHashes(t *testing.T) {
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
	hashes, err := pool.GetCandidateHashes()

	// Assert
	assert.NoError(t, err, "应该成功获取哈希列表")
	assert.Equal(t, numBlocks, len(hashes), "应该返回所有候选区块的哈希")
	for _, hash := range hashes {
		assert.NotNil(t, hash, "每个哈希不应为nil")
		assert.Equal(t, 32, len(hash), "每个哈希长度应为32字节")
	}
}

// TestGetCandidateByHash_WithExistingCandidate_ReturnsCandidate 测试获取存在的候选区块
func TestGetCandidateByHash_WithExistingCandidate_ReturnsCandidate(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	block := testutil.CreateSimpleTestBlock(100)

	// 添加候选区块
	blockHash, err := pool.AddCandidate(block, "peer1")
	require.NoError(t, err)
	require.NotNil(t, blockHash)

	// Act
	candidate, err := pool.GetCandidateByHash(blockHash)

	// Assert
	assert.NoError(t, err, "应该成功获取候选区块")
	assert.NotNil(t, candidate, "候选区块不应为nil")
	assert.Equal(t, blockHash, candidate.BlockHash, "区块哈希应该匹配")
	assert.Equal(t, uint64(100), candidate.Height, "区块高度应该匹配")
}

// TestGetCandidateByHash_WithNonExistentHash_ReturnsError 测试获取不存在的候选区块
func TestGetCandidateByHash_WithNonExistentHash_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	nonExistentHash := []byte("non_existent_hash_32_bytes_12345678")

	// Act
	candidate, err := pool.GetCandidateByHash(nonExistentHash)

	// Assert
	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, candidate, "候选区块应为nil")
	assert.Equal(t, ErrCandidateNotFound, err, "应该返回未找到错误")
}

// TestGetCandidateByHash_WithNilHash_ReturnsError 测试nil哈希
func TestGetCandidateByHash_WithNilHash_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)

	// Act
	candidate, err := pool.GetCandidateByHash(nil)

	// Assert
	assert.Error(t, err, "nil哈希应该返回错误")
	assert.Nil(t, candidate, "候选区块应为nil")
}

