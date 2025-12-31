// Package candidatepool SetEventSink覆盖率测试
package candidatepool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/mempool/testutil"
	"github.com/weisyn/v1/pkg/types"
)

// MockCandidateEventSink Mock事件下沉实现
type MockCandidateEventSink struct {
	addedCandidates   []*types.CandidateBlock
	removedCandidates []struct {
		candidate *types.CandidateBlock
		reason    string
	}
	expiredCandidates []*types.CandidateBlock
	clearedCounts     []int
	cleanupCompleted  int
}

func (m *MockCandidateEventSink) OnCandidateAdded(candidate *types.CandidateBlock) {
	m.addedCandidates = append(m.addedCandidates, candidate)
}

func (m *MockCandidateEventSink) OnCandidateRemoved(candidate *types.CandidateBlock, reason string) {
	m.removedCandidates = append(m.removedCandidates, struct {
		candidate *types.CandidateBlock
		reason    string
	}{candidate, reason})
}

func (m *MockCandidateEventSink) OnCandidateExpired(candidate *types.CandidateBlock) {
	m.expiredCandidates = append(m.expiredCandidates, candidate)
}

func (m *MockCandidateEventSink) OnPoolCleared(count int) {
	m.clearedCounts = append(m.clearedCounts, count)
}

func (m *MockCandidateEventSink) OnCleanupCompleted() {
	m.cleanupCompleted++
}

// TestSetEventSink_WithValidSink_SetsSuccessfully 测试设置有效的事件下沉
func TestSetEventSink_WithValidSink_SetsSuccessfully(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	mockSink := &MockCandidateEventSink{}

	// Act
	pool.SetEventSink(mockSink)

	// Assert
	// 验证事件下沉已设置（通过添加候选区块触发事件）
	block := testutil.CreateSimpleTestBlock(100)
	_, err := pool.AddCandidate(block, "peer1")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(mockSink.addedCandidates), "应该触发OnCandidateAdded事件")
}

// TestSetEventSink_WithNilSink_UsesNoop 测试设置nil事件下沉
func TestSetEventSink_WithNilSink_UsesNoop(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)

	// Act - 设置nil事件下沉
	pool.SetEventSink(nil)

	// Assert - 应该使用Noop实现，不会panic
	block := testutil.CreateSimpleTestBlock(100)
	_, err := pool.AddCandidate(block, "peer1")
	assert.NoError(t, err, "使用Noop事件下沉应该不报错")
}

// TestEventSink_OnCandidateRemoved_TriggersEvent 测试移除候选区块触发事件
func TestEventSink_OnCandidateRemoved_TriggersEvent(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	mockSink := &MockCandidateEventSink{}
	pool.SetEventSink(mockSink)

	block := testutil.CreateSimpleTestBlock(100)
	blockHash, err := pool.AddCandidate(block, "peer1")
	require.NoError(t, err)

	// Act
	err = pool.RemoveCandidate(blockHash)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 1, len(mockSink.removedCandidates), "应该触发OnCandidateRemoved事件")
	assert.Equal(t, "manual_removal", mockSink.removedCandidates[0].reason, "移除原因应该是manual_removal")
}

// TestEventSink_OnPoolCleared_TriggersEvent 测试清空池触发事件
func TestEventSink_OnPoolCleared_TriggersEvent(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	mockSink := &MockCandidateEventSink{}
	pool.SetEventSink(mockSink)

	// 添加一些候选区块
	for i := 0; i < 3; i++ {
		block := testutil.CreateSimpleTestBlock(uint64(100 + i))
		_, err := pool.AddCandidate(block, "peer1")
		require.NoError(t, err)
	}

	// Act
	count, err := pool.ClearCandidates()

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 3, count)
	assert.Equal(t, 1, len(mockSink.clearedCounts), "应该触发OnPoolCleared事件")
	assert.Equal(t, 3, mockSink.clearedCounts[0], "清空数量应该匹配")
}

