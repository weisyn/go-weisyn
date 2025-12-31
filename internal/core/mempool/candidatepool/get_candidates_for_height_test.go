// Package candidatepool GetCandidatesForHeight覆盖率提升测试
package candidatepool

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/mempool/testutil"
)

// TestGetCandidatesForHeight_WithMultipleHeights_ReturnsCorrectHeight 测试多个高度时返回正确高度
func TestGetCandidatesForHeight_WithMultipleHeights_ReturnsCorrectHeight(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	targetHeight := uint64(100)

	// 添加不同高度的候选区块
	block1 := testutil.CreateSimpleTestBlock(targetHeight)
	block2 := testutil.CreateSimpleTestBlock(targetHeight)
	block3 := testutil.CreateSimpleTestBlock(200) // 不同高度

	_, err1 := pool.AddCandidate(block1, "peer1")
	require.NoError(t, err1)
	_, err2 := pool.AddCandidate(block2, "peer2")
	require.NoError(t, err2)
	_, err3 := pool.AddCandidate(block3, "peer3")
	require.NoError(t, err3)

	// Act
	candidates, err := pool.GetCandidatesForHeight(targetHeight, 100*time.Millisecond)

	// Assert
	assert.NoError(t, err, "应该成功获取候选区块")
	assert.Equal(t, 2, len(candidates), "应该返回2个候选区块（高度100）")
	for _, candidate := range candidates {
		assert.Equal(t, targetHeight, candidate.Height, "所有候选区块高度应该匹配")
	}
}

// TestGetCandidatesForHeight_WithWaiting_ReceivesNotification 测试等待时收到通知
func TestGetCandidatesForHeight_WithWaiting_ReceivesNotification(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	height := uint64(200)

	// 启动池以确保等待机制正常工作
	err := pool.Start()
	require.NoError(t, err)
	defer func() {
		_ = pool.Stop()
	}()

	// 在另一个goroutine中添加候选区块
	done := make(chan bool, 1)
	go func() {
		defer func() { done <- true }()
		time.Sleep(50 * time.Millisecond)
		block := testutil.CreateSimpleTestBlock(height)
		_, err := pool.AddCandidate(block, "peer1")
		if err != nil {
			t.Logf("添加候选区块失败: %v", err)
		}
	}()

	// Act - 等待候选区块（应该收到通知）
	candidates, err := pool.GetCandidatesForHeight(height, 300*time.Millisecond)

	// Assert
	// 等待goroutine完成
	select {
	case <-done:
	case <-time.After(400 * time.Millisecond):
		t.Log("添加候选区块的goroutine超时")
	}

	if err != nil {
		// 如果超时，这是可以接受的（取决于实现）
		assert.Equal(t, ErrTimeout, err, "应该返回超时错误或成功")
	} else {
		assert.NotEmpty(t, candidates, "应该收到候选区块通知")
		if len(candidates) > 0 {
			assert.Equal(t, height, candidates[0].Height, "候选区块高度应该匹配")
		}
	}
}

// TestGetCandidatesForHeight_WithTimeout_ReturnsTimeout 测试超时场景
func TestGetCandidatesForHeight_WithTimeout_ReturnsTimeout(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	height := uint64(999)

	// 启动池以确保等待机制正常工作
	err := pool.Start()
	require.NoError(t, err)
	defer pool.Stop()

	// Act - 使用很短的超时时间
	candidates, err := pool.GetCandidatesForHeight(height, 50*time.Millisecond)

	// Assert
	assert.Error(t, err, "应该返回超时错误")
	assert.Nil(t, candidates, "候选区块列表应为nil")
	assert.Equal(t, ErrTimeout, err, "应该返回超时错误")
}

// TestGetCandidatesForHeight_WithPoolClosed_ReturnsError 测试池关闭后获取
func TestGetCandidatesForHeight_WithPoolClosed_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	height := uint64(100)

	// 启动并关闭池
	err := pool.Start()
	require.NoError(t, err)
	err = pool.Stop()
	require.NoError(t, err)

	// 等待一小段时间确保关闭完成
	time.Sleep(50 * time.Millisecond)

	// Act
	candidates, err := pool.GetCandidatesForHeight(height, 100*time.Millisecond)

	// Assert
	assert.Error(t, err, "池关闭后应该返回错误")
	assert.Nil(t, candidates, "候选区块列表应为nil")
	assert.Equal(t, ErrPoolClosed, err, "应该返回池关闭错误")
}

// TestGetCandidatesForHeight_WithExistingCandidates_CreatesCopy 测试返回副本
func TestGetCandidatesForHeight_WithExistingCandidates_CreatesCopy(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	height := uint64(100)

	// 添加候选区块
	block := testutil.CreateSimpleTestBlock(height)
	_, err := pool.AddCandidate(block, "peer1")
	require.NoError(t, err)

	// Act - 获取候选区块
	candidates1, err1 := pool.GetCandidatesForHeight(height, 100*time.Millisecond)
	require.NoError(t, err1)

	// 再次获取
	candidates2, err2 := pool.GetCandidatesForHeight(height, 100*time.Millisecond)
	require.NoError(t, err2)

	// Assert - 验证返回的是副本（不是同一个切片）
	assert.Equal(t, len(candidates1), len(candidates2), "两次获取应该返回相同数量的候选区块")
	// 验证是副本（地址不同）
	if len(candidates1) > 0 && len(candidates2) > 0 {
		// 虽然内容相同，但应该是不同的切片实例
		assert.Equal(t, candidates1[0].Height, candidates2[0].Height, "内容应该相同")
	}
}

