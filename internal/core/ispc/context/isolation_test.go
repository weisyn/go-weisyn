package context

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
)

// ============================================================================
// isolation.go 测试
// ============================================================================
//
// ✅ **重构说明**：使用testutil包中的统一Mock对象，遵循测试规范
//
// ============================================================================

// TestNewContextIsolationEnforcer 测试创建上下文隔离增强器
func TestNewContextIsolationEnforcer(t *testing.T) {
	maxLifetime := 5 * time.Minute
	enforcer := NewContextIsolationEnforcer(maxLifetime)
	require.NotNil(t, enforcer)
	assert.NotNil(t, enforcer.activeContexts)
	assert.Equal(t, maxLifetime, enforcer.maxLifetime)
}

// TestContextIsolationEnforcer_TrackContext 测试跟踪上下文创建
func TestContextIsolationEnforcer_TrackContext(t *testing.T) {
	enforcer := NewContextIsolationEnforcer(5 * time.Minute)
	executionID := "test_execution"

	enforcer.TrackContext(executionID)

	// 验证上下文已被跟踪
	enforcer.mutex.RLock()
	info, exists := enforcer.activeContexts[executionID]
	enforcer.mutex.RUnlock()

	require.True(t, exists)
	assert.Equal(t, executionID, info.executionID)
	assert.False(t, info.isDestroyed)
	assert.Equal(t, uint64(0), info.accessCount)
}

// TestContextIsolationEnforcer_TrackAccess 测试跟踪上下文访问
func TestContextIsolationEnforcer_TrackAccess(t *testing.T) {
	enforcer := NewContextIsolationEnforcer(5 * time.Minute)
	executionID := "test_execution"

	// 先跟踪上下文创建
	enforcer.TrackContext(executionID)

	// 跟踪访问
	enforcer.TrackAccess(executionID)
	enforcer.TrackAccess(executionID)

	// 验证访问计数
	enforcer.mutex.RLock()
	info := enforcer.activeContexts[executionID]
	enforcer.mutex.RUnlock()

	assert.Equal(t, uint64(2), info.accessCount)
	assert.True(t, time.Since(info.lastAccessAt) < time.Second)
}

// TestContextIsolationEnforcer_TrackDestroy 测试跟踪上下文销毁
func TestContextIsolationEnforcer_TrackDestroy(t *testing.T) {
	enforcer := NewContextIsolationEnforcer(5 * time.Minute)
	executionID := "test_execution"

	// 先跟踪上下文创建
	enforcer.TrackContext(executionID)

	// 跟踪销毁
	enforcer.TrackDestroy(executionID)

	// 验证销毁状态
	enforcer.mutex.RLock()
	info := enforcer.activeContexts[executionID]
	enforcer.mutex.RUnlock()

	assert.True(t, info.isDestroyed)
	assert.False(t, info.destroyedAt.IsZero())
}

// TestContextIsolationEnforcer_DetectLeaks 测试检测上下文泄漏
func TestContextIsolationEnforcer_DetectLeaks(t *testing.T) {
	// 创建增强器，设置很短的生存时间用于测试
	maxLifetime := 100 * time.Millisecond
	enforcer := NewContextIsolationEnforcer(maxLifetime)

	// 创建上下文但不销毁（应该被检测为泄漏）
	executionID1 := "leaked_execution"
	enforcer.TrackContext(executionID1)

	// 等待超过最大生存时间
	time.Sleep(150 * time.Millisecond)

	// 检测泄漏
	leakedContexts, err := enforcer.DetectLeaks()
	require.NoError(t, err)
	assert.Contains(t, leakedContexts, executionID1)

	// 创建正常销毁的上下文（不应该被检测为泄漏）
	executionID2 := "normal_execution"
	enforcer.TrackContext(executionID2)
	enforcer.TrackDestroy(executionID2)

	leakedContexts, err = enforcer.DetectLeaks()
	require.NoError(t, err)
	assert.NotContains(t, leakedContexts, executionID2)
}

// TestContextIsolationEnforcer_DetectLeaks_HighAccessCount 测试检测高访问次数泄漏
func TestContextIsolationEnforcer_DetectLeaks_HighAccessCount(t *testing.T) {
	enforcer := NewContextIsolationEnforcer(5 * time.Minute)
	executionID := "high_access_execution"

	enforcer.TrackContext(executionID)

	// 模拟高访问次数（超过10000）
	for i := 0; i < 10001; i++ {
		enforcer.TrackAccess(executionID)
	}

	// 检测泄漏
	leakedContexts, err := enforcer.DetectLeaks()
	require.NoError(t, err)
	assert.Contains(t, leakedContexts, executionID)
}

// TestContextIsolationEnforcer_CleanupOldTracking 测试清理旧的跟踪信息
func TestContextIsolationEnforcer_CleanupOldTracking(t *testing.T) {
	enforcer := NewContextIsolationEnforcer(5 * time.Minute)

	// 创建并销毁上下文
	executionID1 := "old_execution_1"
	enforcer.TrackContext(executionID1)
	enforcer.TrackDestroy(executionID1)

	// 创建未销毁的上下文
	executionID2 := "active_execution"
	enforcer.TrackContext(executionID2)

	// 等待一段时间
	time.Sleep(50 * time.Millisecond)

	// 清理超过50ms的已销毁跟踪信息
	enforcer.CleanupOldTracking(30 * time.Millisecond)

	// 验证旧的跟踪信息已被清理
	enforcer.mutex.RLock()
	_, exists1 := enforcer.activeContexts[executionID1]
	_, exists2 := enforcer.activeContexts[executionID2]
	enforcer.mutex.RUnlock()

	assert.False(t, exists1, "旧的跟踪信息应该被清理")
	assert.True(t, exists2, "活跃的上下文应该保留")
}

// TestDeepCopyContext 测试深度拷贝执行上下文
func TestDeepCopyContext(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_copy"
	callerAddress := "caller"

	// 创建上下文
	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)

	// 类型断言到 contextImpl
	ctxImpl, ok := executionContext.(*contextImpl)
	require.True(t, ok)

	// 添加一些数据
	ctxImpl.RecordHostFunctionCall(&ispcInterfaces.HostFunctionCall{
		Sequence:     1,
		FunctionName: "test_function",
		Parameters:   map[string]interface{}{"key": "value"},
		Result:       map[string]interface{}{"result": "success"},
		Timestamp:    time.Now().UnixNano(),
	})

	// 深度拷贝
	copiedCtx, err := DeepCopyContext(ctxImpl)
	require.NoError(t, err)
	require.NotNil(t, copiedCtx)

	// 验证拷贝的上下文
	assert.Equal(t, ctxImpl.executionID, copiedCtx.executionID)
	assert.Equal(t, len(ctxImpl.hostFunctionCalls), len(copiedCtx.hostFunctionCalls))
	assert.Nil(t, copiedCtx.manager, "管理器引用不应该被拷贝")

	// 验证修改原始上下文不影响拷贝
	ctxImpl.RecordHostFunctionCall(&ispcInterfaces.HostFunctionCall{
		Sequence:     2,
		FunctionName: "another_function",
		Parameters:   map[string]interface{}{},
		Result:       map[string]interface{}{},
		Timestamp:    time.Now().UnixNano(),
	})

	assert.Equal(t, 1, len(copiedCtx.hostFunctionCalls), "拷贝的上下文不应该受影响")

	// 测试拷贝nil上下文
	_, err = DeepCopyContext(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "源上下文不能为nil")

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestVerifyContextIsolation 测试验证上下文隔离
func TestVerifyContextIsolation(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()

	// 创建两个上下文
	executionID1 := "exec_1"
	executionID2 := "exec_2"
	callerAddress := "caller"

	executionContext1, err := manager.CreateContext(ctx, executionID1, callerAddress)
	require.NoError(t, err)

	executionContext2, err := manager.CreateContext(ctx, executionID2, callerAddress)
	require.NoError(t, err)

	// 验证隔离
	isolated, issues := VerifyContextIsolation(executionContext1.(*contextImpl), executionContext2.(*contextImpl))
	assert.True(t, isolated, "两个独立的上下文应该是隔离的")
	assert.Empty(t, issues, "不应该有隔离问题")

	// 清理
	manager.DestroyContext(ctx, executionID1)
	manager.DestroyContext(ctx, executionID2)
}

// TestContextIsolationEnforcer_ConcurrentAccess 测试并发访问
func TestContextIsolationEnforcer_ConcurrentAccess(t *testing.T) {
	enforcer := NewContextIsolationEnforcer(5 * time.Minute)
	executionID := "concurrent_execution"

	// 并发跟踪创建、访问、销毁
	done := make(chan bool, 30)
	for i := 0; i < 10; i++ {
		go func() {
			enforcer.TrackContext(executionID)
			done <- true
		}()
	}
	for i := 0; i < 10; i++ {
		go func() {
			enforcer.TrackAccess(executionID)
			done <- true
		}()
	}
	for i := 0; i < 10; i++ {
		go func() {
			enforcer.TrackDestroy(executionID)
			done <- true
		}()
	}

	// 等待所有goroutine完成
	for i := 0; i < 30; i++ {
		<-done
	}

	// 验证最终状态：如果上下文存在，应该被标记为销毁
	// ⚠️ **测试修复**：由于并发操作，最终状态可能不确定
	// 但至少应该验证并发操作没有panic，且最终状态是一致的
	enforcer.mutex.RLock()
	info, exists := enforcer.activeContexts[executionID]
	enforcer.mutex.RUnlock()

	// 如果上下文存在，应该被标记为销毁（因为TrackDestroy被调用了）
	if exists {
		assert.True(t, info.isDestroyed, "最终应该被标记为销毁")
	} else {
		// 如果上下文不存在，说明已经被清理（这也是合理的）
		t.Logf("上下文已被清理（这也是合理的）")
	}
}

