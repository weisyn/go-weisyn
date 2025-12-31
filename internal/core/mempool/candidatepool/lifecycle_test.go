// Package candidatepool 生命周期方法覆盖率测试
package candidatepool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStart_WithValidPool_StartsSuccessfully 测试启动有效的池
func TestStart_WithValidPool_StartsSuccessfully(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)

	// Act
	err := pool.Start()

	// Assert
	assert.NoError(t, err, "应该成功启动")
	assert.True(t, pool.IsRunning(), "池应该正在运行")

	// 清理
	_ = pool.Stop()
}

// TestStart_WithAlreadyRunning_ReturnsError 测试重复启动
func TestStart_WithAlreadyRunning_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)

	// 第一次启动
	err := pool.Start()
	require.NoError(t, err)

	// Act - 第二次启动
	err = pool.Start()

	// Assert
	assert.Error(t, err, "重复启动应该返回错误")
	assert.Contains(t, err.Error(), "已在运行", "错误信息应该包含'已在运行'")

	// 清理
	_ = pool.Stop()
}

// TestStop_WithRunningPool_StopsSuccessfully 测试停止运行的池
func TestStop_WithRunningPool_StopsSuccessfully(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)

	// 启动池
	err := pool.Start()
	require.NoError(t, err)
	require.True(t, pool.IsRunning(), "池应该正在运行")

	// Act
	err = pool.Stop()

	// Assert
	assert.NoError(t, err, "应该成功停止")
	assert.False(t, pool.IsRunning(), "池应该已停止")
}

// TestStop_WithNotRunning_ReturnsError 测试停止未运行的池
func TestStop_WithNotRunning_ReturnsError(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)

	// Act
	err := pool.Stop()

	// Assert
	assert.Error(t, err, "停止未运行的池应该返回错误")
	assert.Contains(t, err.Error(), "未运行", "错误信息应该包含'未运行'")
}

// TestIsRunning_WithNewPool_ReturnsFalse 测试新池未运行
func TestIsRunning_WithNewPool_ReturnsFalse(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)

	// Act
	isRunning := pool.IsRunning()

	// Assert
	assert.False(t, isRunning, "新池应该未运行")
}

// TestIsRunning_WithStartedPool_ReturnsTrue 测试启动后的池正在运行
func TestIsRunning_WithStartedPool_ReturnsTrue(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)

	// 启动池
	err := pool.Start()
	require.NoError(t, err)

	// Act
	isRunning := pool.IsRunning()

	// Assert
	assert.True(t, isRunning, "启动后的池应该正在运行")

	// 清理
	_ = pool.Stop()
}

// TestIsRunning_WithStoppedPool_ReturnsFalse 测试停止后的池未运行
func TestIsRunning_WithStoppedPool_ReturnsFalse(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)

	// 启动并停止池
	err := pool.Start()
	require.NoError(t, err)
	err = pool.Stop()
	require.NoError(t, err)

	// Act
	isRunning := pool.IsRunning()

	// Assert
	assert.False(t, isRunning, "停止后的池应该未运行")
}

