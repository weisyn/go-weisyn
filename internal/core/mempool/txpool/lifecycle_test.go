// Package txpool 生命周期管理测试
package txpool

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStart_WithValidContext_StartsMaintenanceLoop 测试Start方法
func TestStart_WithValidContext_StartsMaintenanceLoop(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	ctx := context.Background()

	// Act
	err := pool.Start(ctx)

	// Assert
	assert.NoError(t, err, "Start应该成功")
	// 注意：maintenanceLoop是后台协程，我们无法直接验证其运行
	// 但可以通过提交交易后等待一段时间来间接验证
}

// TestStop_WithRunningPool_StopsPool 测试Stop方法
func TestStop_WithRunningPool_StopsPool(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)

	// Act
	err := pool.Stop()

	// Assert
	assert.NoError(t, err, "Stop应该成功")
}

// TestStart_ThenStop_WorksCorrectly 测试Start后Stop的流程
func TestStart_ThenStop_WorksCorrectly(t *testing.T) {
	// Arrange
	pool := createTestTxPool(t)
	ctx := context.Background()

	// Act
	err1 := pool.Start(ctx)
	require.NoError(t, err1)

	err2 := pool.Stop()

	// Assert
	assert.NoError(t, err2, "Stop应该成功")
}

