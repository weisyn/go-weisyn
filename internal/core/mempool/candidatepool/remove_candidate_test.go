// Package candidatepool RemoveCandidate覆盖率测试
package candidatepool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/mempool/testutil"
)

// TestRemoveCandidate_WithExistingCandidate_RemovesSuccessfully 测试移除存在的候选区块
func TestRemoveCandidate_WithExistingCandidate_RemovesSuccessfully(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	block := testutil.CreateSimpleTestBlock(100)

	// 添加候选区块
	blockHash, err := pool.AddCandidate(block, "peer1")
	require.NoError(t, err)
	require.NotNil(t, blockHash)

	// Act
	err = pool.RemoveCandidate(blockHash)

	// Assert
	assert.NoError(t, err, "应该成功移除候选区块")

	// 验证候选区块已被移除
	candidate, err := pool.GetCandidateByHash(blockHash)
	assert.Error(t, err, "应该返回未找到错误")
	assert.Nil(t, candidate, "候选区块应为nil")
	assert.Equal(t, ErrCandidateNotFound, err, "应该返回未找到错误")
}

// TestRemoveCandidate_WithNonExistentHash_ReturnsError 测试移除不存在的候选区块
func TestRemoveCandidate_WithNonExistentHash_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	nonExistentHash := []byte("non_existent_hash_32_bytes_12345678")

	// Act
	err := pool.RemoveCandidate(nonExistentHash)

	// Assert
	assert.Error(t, err, "应该返回错误")
	assert.Equal(t, ErrCandidateNotFound, err, "应该返回未找到错误")
}

// TestRemoveCandidate_WithNilHash_ReturnsError 测试nil哈希
func TestRemoveCandidate_WithNilHash_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)

	// Act
	err := pool.RemoveCandidate(nil)

	// Assert
	assert.Error(t, err, "nil哈希应该返回错误")
}

// TestRemoveCandidate_WithMultipleCandidates_RemovesOnlyOne 测试移除多个候选区块中的一个
func TestRemoveCandidate_WithMultipleCandidates_RemovesOnlyOne(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	block1 := testutil.CreateSimpleTestBlock(100)
	block2 := testutil.CreateSimpleTestBlock(200)

	// 添加两个候选区块
	hash1, err1 := pool.AddCandidate(block1, "peer1")
	require.NoError(t, err1)
	hash2, err2 := pool.AddCandidate(block2, "peer2")
	require.NoError(t, err2)

	// Act - 移除第一个
	err := pool.RemoveCandidate(hash1)

	// Assert
	assert.NoError(t, err, "应该成功移除候选区块")

	// 验证第一个已被移除
	_, err = pool.GetCandidateByHash(hash1)
	assert.Error(t, err, "第一个候选区块应该已被移除")

	// 验证第二个仍然存在
	candidate2, err := pool.GetCandidateByHash(hash2)
	assert.NoError(t, err, "第二个候选区块应该仍然存在")
	assert.NotNil(t, candidate2, "第二个候选区块不应为nil")
}

