// Package candidatepool 竞态条件和并发问题测试
package candidatepool

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/mempool/testutil"
	"github.com/weisyn/v1/pkg/types"
)

// TestNotifyWaiters_WithoutLock_RaceCondition 测试notifyWaiters在没有锁的情况下可能存在的竞态条件
func TestNotifyWaiters_WithoutLock_RaceCondition(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	height := uint64(100)

	// 启动多个goroutine同时添加候选区块和等待
	var wg sync.WaitGroup
	errors := make(chan error, 10)

	// 启动多个等待者
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			candidates, err := pool.GetCandidatesForHeight(height, 500*time.Millisecond)
			if err != nil && err != ErrTimeout {
				errors <- err
			} else if err == nil && len(candidates) == 0 {
				errors <- fmt.Errorf("收到空候选区块列表")
			}
		}()
	}

	// 等待一小段时间，确保等待者已经注册
	time.Sleep(50 * time.Millisecond)

	// 添加候选区块（这会触发notifyWaiters）
	block := testutil.CreateSimpleTestBlock(height)
	_, err := pool.AddCandidate(block, "peer1")
	require.NoError(t, err)

	// 等待所有goroutine完成
	done := make(chan bool)
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 检查是否有错误
		close(errors)
		for err := range errors {
			t.Errorf("发现错误: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("测试超时，可能存在死锁或竞态条件")
	}
}

// TestAddCandidate_ConcurrentAccess_RaceCondition 测试并发添加候选区块时的竞态条件
func TestAddCandidate_ConcurrentAccess_RaceCondition(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	numGoroutines := 10
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	// 并发添加候选区块
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			block := testutil.CreateSimpleTestBlock(uint64(100 + index))
			_, err := pool.AddCandidate(block, "peer1")
			if err != nil && err != ErrCandidateAlreadyExists {
				errors <- fmt.Errorf("goroutine %d: %v", index, err)
			}
		}(i)
	}

	// 等待所有goroutine完成
	wg.Wait()
	close(errors)

	// Assert - 检查是否有错误
	errorCount := 0
	for err := range errors {
		t.Errorf("并发访问错误: %v", err)
		errorCount++
	}

	// 验证所有候选区块都已添加
	allCandidates, err := pool.GetAllCandidates()
	assert.NoError(t, err)
	assert.Equal(t, numGoroutines, len(allCandidates), "所有候选区块应该都已添加")
}

// TestNotifyWaiters_WithNilChannel_Panic 测试notifyWaiters在通道为nil时是否会panic
func TestNotifyWaiters_WithNilChannel_Panic(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	height := uint64(100)

	// 手动设置一个nil通道（模拟可能的bug）
	pool.mu.Lock()
	pool.waitChannels["test_nil_channel"] = nil
	pool.mu.Unlock()

	// Act - 添加候选区块，这会触发notifyWaiters
	block := testutil.CreateSimpleTestBlock(height)
	
	// 这不应该panic
	assert.NotPanics(t, func() {
		_, err := pool.AddCandidate(block, "peer1")
		require.NoError(t, err)
	}, "notifyWaiters应该处理nil通道而不panic")
}

// TestWaitForCandidatesAtHeight_WithPoolClosed_DuringWait 测试在等待期间池关闭的情况
func TestWaitForCandidatesAtHeight_WithPoolClosed_DuringWait(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	height := uint64(100)

	// 启动池
	err := pool.Start()
	require.NoError(t, err)

	// 在另一个goroutine中等待
	done := make(chan bool, 1)
	go func() {
		_, err := pool.GetCandidatesForHeight(height, 1*time.Second)
		if err == ErrPoolClosed {
			done <- true
		} else {
			done <- false
		}
	}()

	// 等待一小段时间，确保等待者已经注册
	time.Sleep(50 * time.Millisecond)

	// 关闭池
	err = pool.Stop()
	require.NoError(t, err)

	// Assert - 等待者应该收到池关闭错误
	select {
	case result := <-done:
		assert.True(t, result, "应该收到池关闭错误")
	case <-time.After(2 * time.Second):
		t.Error("测试超时，等待者可能没有正确响应池关闭")
	}
}

// TestNotifyWaiters_WithClosedChannel_Panic 测试notifyWaiters在通道已关闭时是否会panic
func TestNotifyWaiters_WithClosedChannel_Panic(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	height := uint64(100)

	// 创建一个已关闭的通道
	closedCh := make(chan []*types.CandidateBlock)
	close(closedCh)

	pool.mu.Lock()
	pool.waitChannels["test_closed_channel"] = closedCh
	pool.mu.Unlock()

	// Act - 添加候选区块，这会触发notifyWaiters
	block := testutil.CreateSimpleTestBlock(height)
	
	// 这不应该panic（应该优雅地处理关闭的通道）
	assert.NotPanics(t, func() {
		_, err := pool.AddCandidate(block, "peer1")
		require.NoError(t, err)
	}, "notifyWaiters应该处理关闭的通道而不panic")
}

