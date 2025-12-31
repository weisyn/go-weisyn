package context

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	"github.com/weisyn/v1/internal/core/ispc/testutil"
)

// ============================================================================
// 异步轨迹记录集成测试（异步轨迹记录优化 - 阶段3测试）
// ============================================================================
//
// 🎯 **测试目的**：
// 测试异步轨迹记录的完整集成流程，包括启用、记录、刷新、禁用等。
//
// ⚠️ **注意**：
// - 必须使用`go test -race`运行这些测试
// - 测试会启动真实的Manager和工作线程池
//
// ============================================================================

// ============================================================================
//
// ✅ **重构说明**：使用testutil包中的统一Mock对象，遵循测试规范
//
// ============================================================================

// setupIntegrationManager 创建用于集成测试的Manager
//
// ✅ **重构说明**：使用testutil包中的统一Mock对象，遵循测试规范
func setupIntegrationManager(t *testing.T) *Manager {
	logger := testutil.NewTestLogger()
	clock := testutil.NewTestClock()
	
	manager := &Manager{
		logger: logger,
		clock:  clock,
		config: &ContextManagerConfig{
			DefaultTimeoutMs:      30000,
			MaxContextLifetime:    300000,
			MaxConcurrentContexts: 100,
			MaxMemoryPerContext:   104857600,
			CleanupIntervalMs:     60000,
		},
		contexts: make(map[string]ispcInterfaces.ExecutionContext),
	}
	
	return manager
}

// TestAsyncTraceRecordingIntegration 测试异步轨迹记录集成流程
func TestAsyncTraceRecordingIntegration(t *testing.T) {
	manager := setupIntegrationManager(t)
	
	// 1. 启用异步轨迹记录
	err := manager.EnableAsyncTraceRecording(2, 50, 50*time.Millisecond, 3, 10*time.Millisecond)
	require.NoError(t, err, "启用异步轨迹记录应该成功")
	assert.True(t, manager.IsAsyncTraceRecordingEnabled(), "异步轨迹记录应该已启用")
	
	// 2. 创建执行上下文
	ctx := context.Background()
	executionID := "test_execution_1"
	callerAddress := "test_caller"
	
	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err, "创建执行上下文应该成功")
	assert.NotNil(t, executionContext, "执行上下文不应该为nil")
	
	// 3. 记录宿主函数调用（异步模式）
	totalCalls := 100
	for i := 0; i < totalCalls; i++ {
		call := &ispcInterfaces.HostFunctionCall{
			Sequence:     uint64(i),
			FunctionName: "test_function",
			Parameters:   map[string]interface{}{"param": i},
			Result:       map[string]interface{}{"result": i * 2},
			Timestamp:    time.Now().UnixNano(),
		}
		executionContext.RecordHostFunctionCall(call)
	}
	
	// 4. 等待异步处理完成
	time.Sleep(200 * time.Millisecond)
	
	// 5. 刷新队列（执行完成同步点）
	err = manager.FlushTraceQueue()
	require.NoError(t, err, "刷新队列应该成功")
	
	// 6. 验证轨迹记录已写入
	trace, err := executionContext.GetExecutionTrace()
	require.NoError(t, err, "获取执行轨迹应该成功")
	assert.Equal(t, totalCalls, len(trace), "所有宿主函数调用都应该被记录")
	
	// 7. 验证统计信息
	stats := manager.GetTraceQueueStats()
	assert.NotNil(t, stats, "统计信息不应该为nil")
	
	// 8. 销毁执行上下文
	err = manager.DestroyContext(ctx, executionID)
	require.NoError(t, err, "销毁执行上下文应该成功")
	
	// 9. 禁用异步轨迹记录
	err = manager.DisableAsyncTraceRecording()
	require.NoError(t, err, "禁用异步轨迹记录应该成功")
	assert.False(t, manager.IsAsyncTraceRecordingEnabled(), "异步轨迹记录应该已禁用")
}

// TestAsyncTraceRecordingConcurrentIntegration 测试并发异步轨迹记录集成
func TestAsyncTraceRecordingConcurrentIntegration(t *testing.T) {
	manager := setupIntegrationManager(t)
	
	// 启用异步轨迹记录
	err := manager.EnableAsyncTraceRecording(5, 50, 50*time.Millisecond, 3, 10*time.Millisecond)
	require.NoError(t, err)
	
	// 创建多个执行上下文
	executionCount := 10
	contexts := make(map[string]ispcInterfaces.ExecutionContext)
	ctx := context.Background()
	
	for i := 0; i < executionCount; i++ {
		executionID := fmt.Sprintf("test_execution_%d", i)
		callerAddress := fmt.Sprintf("test_caller_%d", i)
		
		executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
		require.NoError(t, err)
		contexts[executionID] = executionContext
	}
	
	// 并发记录宿主函数调用
	concurrency := 50
	callsPerGoroutine := 20
	var wg sync.WaitGroup
	wg.Add(concurrency)
	
	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer wg.Done()
			executionID := fmt.Sprintf("test_execution_%d", id%executionCount)
			executionContext := contexts[executionID]
			
			for j := 0; j < callsPerGoroutine; j++ {
				call := &ispcInterfaces.HostFunctionCall{
					Sequence:     uint64(j),
					FunctionName: "test_function",
					Parameters:   map[string]interface{}{"param": j},
					Result:       map[string]interface{}{"result": j * 2},
					Timestamp:    time.Now().UnixNano(),
				}
				executionContext.RecordHostFunctionCall(call)
			}
		}(i)
	}
	
	wg.Wait()
	
	// 等待异步处理完成
	time.Sleep(500 * time.Millisecond)
	
	// 刷新队列
	err = manager.FlushTraceQueue()
	require.NoError(t, err)
	
	// 验证所有轨迹记录已写入
	totalExpectedCalls := concurrency * callsPerGoroutine
	totalWrittenCalls := 0
	
	for executionID, executionContext := range contexts {
		trace, err := executionContext.GetExecutionTrace()
		require.NoError(t, err, "获取执行轨迹应该成功: executionID=%s", executionID)
		totalWrittenCalls += len(trace)
		
		// 清理
		err = manager.DestroyContext(ctx, executionID)
		require.NoError(t, err)
	}
	
	assert.Equal(t, totalExpectedCalls, totalWrittenCalls, "所有宿主函数调用都应该被记录")
	
	// 禁用异步轨迹记录
	err = manager.DisableAsyncTraceRecording()
	require.NoError(t, err)
}

// TestAsyncTraceRecordingBackwardCompatibility 测试向后兼容性（同步模式）
func TestAsyncTraceRecordingBackwardCompatibility(t *testing.T) {
	manager := setupIntegrationManager(t)
	
	// 不启用异步轨迹记录（默认同步模式）
	assert.False(t, manager.IsAsyncTraceRecordingEnabled(), "默认应该禁用异步轨迹记录")
	
	// 创建执行上下文
	ctx := context.Background()
	executionID := "test_execution_sync"
	callerAddress := "test_caller"
	
	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)
	
	// 记录宿主函数调用（同步模式）
	totalCalls := 50
	for i := 0; i < totalCalls; i++ {
		call := &ispcInterfaces.HostFunctionCall{
			Sequence:     uint64(i),
			FunctionName: "test_function",
			Parameters:   map[string]interface{}{"param": i},
			Result:       map[string]interface{}{"result": i * 2},
			Timestamp:    time.Now().UnixNano(),
		}
		executionContext.RecordHostFunctionCall(call)
	}
	
	// 同步模式下，记录应该立即写入
	trace, err := executionContext.GetExecutionTrace()
	require.NoError(t, err)
	assert.Equal(t, totalCalls, len(trace), "所有宿主函数调用都应该被记录（同步模式）")
	
	// 清理
	err = manager.DestroyContext(ctx, executionID)
	require.NoError(t, err)
}

// TestAsyncTraceRecordingEnableDisable 测试启用和禁用异步轨迹记录
func TestAsyncTraceRecordingEnableDisable(t *testing.T) {
	manager := setupIntegrationManager(t)
	
	// 初始状态应该是禁用
	assert.False(t, manager.IsAsyncTraceRecordingEnabled(), "初始状态应该禁用异步轨迹记录")
	
	// 启用异步轨迹记录
	err := manager.EnableAsyncTraceRecording(2, 50, 50*time.Millisecond, 3, 10*time.Millisecond)
	require.NoError(t, err)
	assert.True(t, manager.IsAsyncTraceRecordingEnabled(), "异步轨迹记录应该已启用")
	
	// 再次启用应该成功（幂等）
	err = manager.EnableAsyncTraceRecording(2, 50, 50*time.Millisecond, 3, 10*time.Millisecond)
	require.NoError(t, err)
	assert.True(t, manager.IsAsyncTraceRecordingEnabled(), "再次启用应该成功")
	
	// 禁用异步轨迹记录
	err = manager.DisableAsyncTraceRecording()
	require.NoError(t, err)
	assert.False(t, manager.IsAsyncTraceRecordingEnabled(), "异步轨迹记录应该已禁用")
	
	// 再次禁用应该成功（幂等）
	err = manager.DisableAsyncTraceRecording()
	require.NoError(t, err)
	assert.False(t, manager.IsAsyncTraceRecordingEnabled(), "再次禁用应该成功")
}

// TestAsyncTraceRecordingStats 测试统计信息
func TestAsyncTraceRecordingStats(t *testing.T) {
	manager := setupIntegrationManager(t)
	
	// 启用异步轨迹记录
	err := manager.EnableAsyncTraceRecording(2, 50, 50*time.Millisecond, 3, 10*time.Millisecond)
	require.NoError(t, err)
	
	// 创建执行上下文
	ctx := context.Background()
	executionID := "test_execution_stats"
	callerAddress := "test_caller"
	
	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)
	
	// 记录一些宿主函数调用
	totalCalls := 100
	for i := 0; i < totalCalls; i++ {
		call := &ispcInterfaces.HostFunctionCall{
			Sequence:     uint64(i),
			FunctionName: "test_function",
			Parameters:   map[string]interface{}{"param": i},
			Result:       map[string]interface{}{"result": i * 2},
			Timestamp:    time.Now().UnixNano(),
		}
		executionContext.RecordHostFunctionCall(call)
	}
	
	// 等待处理完成
	time.Sleep(200 * time.Millisecond)
	
	// 获取统计信息
	stats := manager.GetTraceQueueStats()
	assert.NotNil(t, stats, "统计信息不应该为nil")
	
	// 刷新队列
	err = manager.FlushTraceQueue()
	require.NoError(t, err)
	
	// 再次获取统计信息
	statsAfterFlush := manager.GetTraceQueueStats()
	assert.NotNil(t, statsAfterFlush, "刷新后的统计信息不应该为nil")
	
	// 清理
	err = manager.DestroyContext(ctx, executionID)
	require.NoError(t, err)
	
	// 禁用异步轨迹记录
	err = manager.DisableAsyncTraceRecording()
	require.NoError(t, err)
}

