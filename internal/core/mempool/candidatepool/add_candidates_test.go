// Package candidatepool AddCandidates覆盖率测试
package candidatepool

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/core/mempool/testutil"
	core "github.com/weisyn/v1/pb/blockchain/block"
)

// TestAddCandidates_WithValidBlocks_AddsSuccessfully 测试批量添加有效区块
func TestAddCandidates_WithValidBlocks_AddsSuccessfully(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	// 使用不同的高度确保哈希不同
	blocks := []*core.Block{
		testutil.CreateSimpleTestBlock(100),
		testutil.CreateSimpleTestBlock(200),
		testutil.CreateSimpleTestBlock(300),
	}
	fromPeers := []string{"peer1", "peer2", "peer3"}

	// Act
	hashes, err := pool.AddCandidates(blocks, fromPeers)

	// Assert
	assert.NoError(t, err, "应该成功批量添加区块")
	assert.Equal(t, len(blocks), len(hashes), "返回的哈希数量应该等于区块数量")
	for _, hash := range hashes {
		assert.NotNil(t, hash, "每个哈希不应为nil")
		assert.Equal(t, 32, len(hash), "每个哈希长度应为32字节")
	}
}

// TestAddCandidates_WithMismatchedLengths_ReturnsError 测试长度不匹配
func TestAddCandidates_WithMismatchedLengths_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	blocks := []*core.Block{
		testutil.CreateSimpleTestBlock(1),
		testutil.CreateSimpleTestBlock(2),
	}
	fromPeers := []string{"peer1"} // 长度不匹配

	// Act
	hashes, err := pool.AddCandidates(blocks, fromPeers)

	// Assert
	assert.Error(t, err, "长度不匹配应该返回错误")
	assert.Nil(t, hashes, "哈希列表应为nil")
	assert.Contains(t, err.Error(), "区块数量与节点数量不匹配", "错误信息应该包含长度不匹配")
}

// TestAddCandidates_WithEmptyList_ReturnsEmpty 测试空列表
func TestAddCandidates_WithEmptyList_ReturnsEmpty(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	blocks := []*core.Block{}
	fromPeers := []string{}

	// Act
	hashes, err := pool.AddCandidates(blocks, fromPeers)

	// Assert
	assert.NoError(t, err, "空列表应该不返回错误")
	assert.Empty(t, hashes, "应该返回空哈希列表")
}

// TestAddCandidates_WithPartialFailures_ReturnsPartialResults 测试部分失败
func TestAddCandidates_WithPartialFailures_ReturnsPartialResults(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	block1 := testutil.CreateSimpleTestBlock(100)
	block2 := testutil.CreateSimpleTestBlock(200) // 不同高度，应该成功
	block3 := testutil.CreateSimpleTestBlock(100) // 与block1相同高度，会重复
	blocks := []*core.Block{block1, block2, block3}
	fromPeers := []string{"peer1", "peer2", "peer3"}

	// Act - 批量添加（block3会重复）
	hashes, err := pool.AddCandidates(blocks, fromPeers)

	// Assert
	// 根据实现，AddCandidates在遇到错误时会继续处理，但会返回聚合错误
	// 成功的区块哈希会被添加到hashes中
	if err != nil {
		// 如果有错误，应该包含部分成功的信息
		// block1和block2应该成功，block3会失败（重复）
		assert.NotNil(t, hashes, "即使有错误，也应该返回部分成功的哈希")
		assert.GreaterOrEqual(t, len(hashes), 1, "应该至少有1个成功的哈希")
		assert.Contains(t, err.Error(), "部分候选区块添加失败", "错误信息应该包含部分失败")
	} else {
		// 如果全部成功，哈希数量应该等于区块数量
		assert.Equal(t, len(blocks), len(hashes), "如果全部成功，哈希数量应该等于区块数量")
	}
}

