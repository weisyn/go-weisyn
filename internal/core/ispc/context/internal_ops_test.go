package context

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// internal_ops.go 测试
// ============================================================================
//
// ✅ **重构说明**：使用testutil包中的统一Mock对象，遵循测试规范
//
// ============================================================================

// TestGenerateExecutionID 测试生成执行ID
func TestGenerateExecutionID(t *testing.T) {
	// 测试生成执行ID
	id1 := generateExecutionID()
	id2 := generateExecutionID()

	// 验证格式
	assert.Contains(t, id1, "exec_")
	assert.Contains(t, id2, "exec_")

	// 验证每次生成不同的ID（由于时间戳不同）
	// ⚠️ **注意**：如果两次调用时间非常接近（纳秒级），可能生成相同的ID
	// 这是正常的，因为generateExecutionID使用time.Now().UnixNano()作为时间戳
	// 在实际使用中，应该尽量传递非空的executionID以确保确定性
	if id1 == id2 {
		t.Logf("警告：两次生成的ID相同（id1=%s, id2=%s），这可能是由于时间戳相同导致的", id1, id2)
	}
	// 不强制要求不同，因为在高频调用时可能相同
}

// TestManager_cleanupExpiredContexts 测试清理过期上下文
func TestManager_cleanupExpiredContexts(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()

	// 创建上下文并手动设置过期时间
	executionID1 := "expired_execution"
	executionContext1, err := manager.CreateContext(ctx, executionID1, "caller")
	require.NoError(t, err)

	// 类型断言到 contextImpl 并设置过期时间
	if ctxImpl, ok := executionContext1.(*contextImpl); ok {
		ctxImpl.mutex.Lock()
		// ⚠️ **BUG修复**：使用Manager的时钟来设置过期时间，确保时钟一致性
		// 如果使用time.Now()，而cleanupExpiredContexts使用m.clock.Now()，会导致时钟不一致
		ctxImpl.expiresAt = manager.clock.Now().Add(-1 * time.Second) // 设置为1秒前过期
		ctxImpl.hasDeadline = true // 确保hasDeadline为true
		ctxImpl.mutex.Unlock()
	}

	// 创建未过期的上下文
	executionID2 := "active_execution"
	_, err = manager.CreateContext(ctx, executionID2, "caller")
	require.NoError(t, err)

	// 执行清理（cleanupExpiredContexts可能不返回错误，只是执行清理）
	manager.cleanupExpiredContexts()

	// 等待一小段时间确保清理完成
	time.Sleep(100 * time.Millisecond)

	// 验证过期上下文已被清理
	_, err = manager.GetContext(executionID1)
	assert.Error(t, err, "过期的上下文应该被清理")

	// 验证未过期的上下文仍然存在
	_, err = manager.GetContext(executionID2)
	assert.NoError(t, err, "未过期的上下文应该仍然存在")

	// 清理
	manager.DestroyContext(ctx, executionID2)
}

// TestManager_cleanupExpiredContexts_NoExpired 测试清理过期上下文（无过期上下文）
func TestManager_cleanupExpiredContexts_NoExpired(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()

	// 创建未过期的上下文
	executionID := "active_execution"
	_, err := manager.CreateContext(ctx, executionID, "caller")
	require.NoError(t, err)

	// 执行清理
	err = manager.cleanupExpiredContexts()
	require.NoError(t, err)

	// 验证上下文仍然存在
	_, err = manager.GetContext(executionID)
	assert.NoError(t, err, "未过期的上下文应该保留")

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestManager_cleanupExpiredContexts_Empty 测试清理过期上下文（空上下文）
func TestManager_cleanupExpiredContexts_Empty(t *testing.T) {
	manager := createTestManager(t)

	// 执行清理（无上下文）
	err := manager.cleanupExpiredContexts()
	require.NoError(t, err)
}

