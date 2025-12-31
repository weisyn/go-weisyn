// Package stream 提供背压控制的测试
//
// 🧪 **测试文件**
//
// 本文件测试 Semaphore 的核心功能，遵循测试规范：
// - docs/system/standards/principles/testing-standards.md
//
// 🎯 **测试覆盖**：
// - 信号量创建
// - 信号量获取
// - 信号量释放
// - 超时获取
// - 非阻塞获取
// - 容量查询
// - 并发安全测试
package stream

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ==================== 信号量创建测试 ====================

// TestNewSemaphore_WithValidCapacity_ReturnsSemaphore 测试创建有效容量的信号量
func TestNewSemaphore_WithValidCapacity_ReturnsSemaphore(t *testing.T) {
	// Arrange & Act
	sem := NewSemaphore(10)

	// Assert
	assert.NotNil(t, sem)
	assert.Equal(t, 10, sem.Capacity())
	assert.Equal(t, 10, sem.Available())
}

// TestNewSemaphore_WithZeroCapacity_UsesDefaultCapacity 测试零容量时使用默认容量
func TestNewSemaphore_WithZeroCapacity_UsesDefaultCapacity(t *testing.T) {
	// Arrange & Act
	sem := NewSemaphore(0)

	// Assert
	assert.NotNil(t, sem)
	assert.Equal(t, 1, sem.Capacity(), "零容量应该使用默认容量 1")
}

// TestNewSemaphore_WithNegativeCapacity_UsesDefaultCapacity 测试负容量时使用默认容量
func TestNewSemaphore_WithNegativeCapacity_UsesDefaultCapacity(t *testing.T) {
	// Arrange & Act
	sem := NewSemaphore(-1)

	// Assert
	assert.NotNil(t, sem)
	assert.Equal(t, 1, sem.Capacity(), "负容量应该使用默认容量 1")
}

// ==================== 信号量获取测试 ====================

// TestSemaphore_Acquire_WithAvailableResource_ReturnsNoError 测试获取可用资源
func TestSemaphore_Acquire_WithAvailableResource_ReturnsNoError(t *testing.T) {
	// Arrange
	sem := NewSemaphore(10)
	ctx := context.Background()

	// Act
	err := sem.Acquire(ctx)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 9, sem.Available(), "可用资源应该减少")
}

// TestSemaphore_Acquire_WithCancelledContext_ReturnsError 测试已取消的上下文
func TestSemaphore_Acquire_WithCancelledContext_ReturnsError(t *testing.T) {
	// Arrange
	sem := NewSemaphore(1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	// Act
	err := sem.Acquire(ctx)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

// ==================== 信号量释放测试 ====================

// TestSemaphore_Release_AfterAcquire_IncreasesAvailable 测试释放后增加可用资源
func TestSemaphore_Release_AfterAcquire_IncreasesAvailable(t *testing.T) {
	// Arrange
	sem := NewSemaphore(10)
	ctx := context.Background()
	
	sem.Acquire(ctx)
	availableBefore := sem.Available()

	// Act
	sem.Release()

	// Assert
	assert.Equal(t, availableBefore+1, sem.Available(), "释放后可用资源应该增加")
}

// TestSemaphore_Release_WithoutAcquire_NoPanic 测试未获取就释放不会 panic
func TestSemaphore_Release_WithoutAcquire_NoPanic(t *testing.T) {
	// Arrange
	sem := NewSemaphore(10)

	// Act & Assert - 不应该 panic
	assert.NotPanics(t, func() {
		sem.Release()
	})
}

// ==================== 超时获取测试 ====================

// TestSemaphore_AcquireWithTimeout_WithAvailableResource_ReturnsNoError 测试超时获取可用资源
func TestSemaphore_AcquireWithTimeout_WithAvailableResource_ReturnsNoError(t *testing.T) {
	// Arrange
	sem := NewSemaphore(10)
	timeout := 100 * time.Millisecond

	// Act
	err := sem.AcquireWithTimeout(timeout)

	// Assert
	assert.NoError(t, err)
}

// TestSemaphore_AcquireWithTimeout_WithFullCapacity_TimesOut 测试容量满时超时
func TestSemaphore_AcquireWithTimeout_WithFullCapacity_TimesOut(t *testing.T) {
	// Arrange
	sem := NewSemaphore(1)
	timeout := 50 * time.Millisecond
	
	// 占满容量
	sem.Acquire(context.Background())

	// Act
	start := time.Now()
	err := sem.AcquireWithTimeout(timeout)
	duration := time.Since(start)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
	assert.WithinDuration(t, start.Add(timeout), start.Add(duration), 20*time.Millisecond, "应该在超时时间内返回")
}

// ==================== 非阻塞获取测试 ====================

// TestSemaphore_TryAcquire_WithAvailableResource_ReturnsTrue 测试非阻塞获取可用资源
func TestSemaphore_TryAcquire_WithAvailableResource_ReturnsTrue(t *testing.T) {
	// Arrange
	sem := NewSemaphore(10)

	// Act
	success := sem.TryAcquire()

	// Assert
	assert.True(t, success)
	assert.Equal(t, 9, sem.Available())
}

// TestSemaphore_TryAcquire_WithFullCapacity_ReturnsFalse 测试容量满时非阻塞获取失败
func TestSemaphore_TryAcquire_WithFullCapacity_ReturnsFalse(t *testing.T) {
	// Arrange
	sem := NewSemaphore(1)
	sem.TryAcquire() // 占满容量

	// Act
	success := sem.TryAcquire()

	// Assert
	assert.False(t, success)
	assert.Equal(t, 0, sem.Available())
}

// ==================== 容量查询测试 ====================

// TestSemaphore_Capacity_ReturnsCorrectCapacity 测试返回正确容量
func TestSemaphore_Capacity_ReturnsCorrectCapacity(t *testing.T) {
	testCases := []int{1, 10, 100, 1000}
	
	for _, capacity := range testCases {
		t.Run("", func(t *testing.T) {
			// Arrange
			sem := NewSemaphore(capacity)

			// Act
			actualCapacity := sem.Capacity()

			// Assert
			assert.Equal(t, capacity, actualCapacity)
		})
	}
}

// TestSemaphore_Available_ReturnsCorrectAvailable 测试返回正确可用资源数
func TestSemaphore_Available_ReturnsCorrectAvailable(t *testing.T) {
	// Arrange
	sem := NewSemaphore(10)
	ctx := context.Background()

	// Act & Assert
	assert.Equal(t, 10, sem.Available(), "初始可用资源应该等于容量")
	
	sem.Acquire(ctx)
	assert.Equal(t, 9, sem.Available(), "获取后可用资源应该减少")
	
	sem.Acquire(ctx)
	assert.Equal(t, 8, sem.Available(), "再次获取后可用资源应该继续减少")
	
	sem.Release()
	assert.Equal(t, 9, sem.Available(), "释放后可用资源应该增加")
}

// ==================== 并发安全测试 ====================

// TestSemaphore_ConcurrentAcquireRelease_IsThreadSafe 测试并发获取释放的线程安全性
func TestSemaphore_ConcurrentAcquireRelease_IsThreadSafe(t *testing.T) {
	// Arrange
	sem := NewSemaphore(10)
	goroutines := 20
	iterations := 10
	done := make(chan bool, goroutines)

	// Act - 并发获取和释放
	for i := 0; i < goroutines; i++ {
		go func() {
			defer func() { done <- true }()
			ctx := context.Background()
			for j := 0; j < iterations; j++ {
				err := sem.Acquire(ctx)
				if err == nil {
					sem.Release()
				}
			}
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < goroutines; i++ {
		<-done
	}

	// Assert - 最终可用资源应该等于容量
	assert.Equal(t, 10, sem.Available(), "所有操作完成后，可用资源应该等于容量")
}

