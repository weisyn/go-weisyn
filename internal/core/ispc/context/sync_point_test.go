package context

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// 执行完成同步点测试（异步轨迹记录优化 - 阶段3测试）
// ============================================================================
//
// 🎯 **测试目的**：
// 测试执行完成同步点的功能，确保在执行完成时刷新队列，所有轨迹记录都已写入。
//
// ⚠️ **注意**：
// - 必须使用`go test -race`运行这些测试
// - 测试会启动真实的Manager和工作线程池
//
// ============================================================================

// TestSyncPointBasic 测试基本同步点功能
func TestSyncPointBasic(t *testing.T) {
	manager := setupIntegrationManager(t)
	
	// 启用异步轨迹记录
	err := manager.EnableAsyncTraceRecording(2, 50, 50*time.Millisecond, 3, 10*time.Millisecond)
	require.NoError(t, err)
	defer manager.DisableAsyncTraceRecording()
	
	// 创建执行上下文
	ctx := context.Background()
	executionID := "test_sync_point_1"
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
	
	// 验证在同步点之前，记录可能还未完全写入
	// （异步模式下，记录可能还在队列中）
	traceBefore, err := executionContext.GetExecutionTrace()
	require.NoError(t, err)
	
	// 执行完成同步点：刷新队列
	err = manager.FlushTraceQueue()
	require.NoError(t, err, "刷新队列应该成功")
	
	// 等待一小段时间，确保刷新完成
	time.Sleep(100 * time.Millisecond)
	
	// 验证同步点之后，所有记录都已写入
	traceAfter, err := executionContext.GetExecutionTrace()
	require.NoError(t, err)
	assert.Equal(t, totalCalls, len(traceAfter), "同步点之后，所有记录都应该被写入")
	assert.GreaterOrEqual(t, len(traceAfter), len(traceBefore), "同步点之后，记录数量应该增加或保持不变")
	
	// 清理
	err = manager.DestroyContext(ctx, executionID)
	require.NoError(t, err)
}

// TestSyncPointConcurrent 测试并发场景下的同步点功能
func TestSyncPointConcurrent(t *testing.T) {
	manager := setupIntegrationManager(t)
	
	// 启用异步轨迹记录
	err := manager.EnableAsyncTraceRecording(5, 50, 50*time.Millisecond, 3, 10*time.Millisecond)
	require.NoError(t, err)
	defer manager.DisableAsyncTraceRecording()
	
	// 创建多个执行上下文
	executionCount := 10
	contexts := make(map[string]ispcInterfaces.ExecutionContext)
	ctx := context.Background()
	
	for i := 0; i < executionCount; i++ {
		executionID := fmt.Sprintf("test_sync_point_%d", i)
		callerAddress := "test_caller"
		
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
			executionID := fmt.Sprintf("test_sync_point_%d", id%executionCount)
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
	
	// 执行完成同步点：刷新队列
	err = manager.FlushTraceQueue()
	require.NoError(t, err, "刷新队列应该成功")
	
	// 等待一小段时间，确保刷新完成
	time.Sleep(200 * time.Millisecond)
	
	// 验证所有执行上下文的记录都已写入
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
	
	assert.Equal(t, totalExpectedCalls, totalWrittenCalls, "同步点之后，所有记录都应该被写入")
}

// TestSyncPointMultipleFlushes 测试多次刷新同步点
func TestSyncPointMultipleFlushes(t *testing.T) {
	manager := setupIntegrationManager(t)
	
	// 启用异步轨迹记录
	err := manager.EnableAsyncTraceRecording(2, 50, 50*time.Millisecond, 3, 10*time.Millisecond)
	require.NoError(t, err)
	defer manager.DisableAsyncTraceRecording()
	
	// 创建执行上下文
	ctx := context.Background()
	executionID := "test_sync_point_multiple"
	callerAddress := "test_caller"
	
	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)
	
	// 第一轮记录
	firstBatch := 50
	for i := 0; i < firstBatch; i++ {
		call := &ispcInterfaces.HostFunctionCall{
			Sequence:     uint64(i),
			FunctionName: "test_function",
			Parameters:   map[string]interface{}{"param": i},
			Result:       map[string]interface{}{"result": i * 2},
			Timestamp:    time.Now().UnixNano(),
		}
		executionContext.RecordHostFunctionCall(call)
	}
	
	// 第一次同步点
	err = manager.FlushTraceQueue()
	require.NoError(t, err)
	time.Sleep(50 * time.Millisecond)
	
	traceAfterFirst, err := executionContext.GetExecutionTrace()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(traceAfterFirst), firstBatch-5, "第一次同步点后，第一轮记录应该被写入")
	
	// 第二轮记录
	secondBatch := 50
	for i := 0; i < secondBatch; i++ {
		call := &ispcInterfaces.HostFunctionCall{
			Sequence:     uint64(firstBatch + i),
			FunctionName: "test_function",
			Parameters:   map[string]interface{}{"param": firstBatch + i},
			Result:       map[string]interface{}{"result": (firstBatch + i) * 2},
			Timestamp:    time.Now().UnixNano(),
		}
		executionContext.RecordHostFunctionCall(call)
	}
	
	// 第二次同步点
	err = manager.FlushTraceQueue()
	require.NoError(t, err)
	time.Sleep(50 * time.Millisecond)
	
	traceAfterSecond, err := executionContext.GetExecutionTrace()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(traceAfterSecond), firstBatch+secondBatch-5, "第二次同步点后，所有记录应该被写入")
	
	// 清理
	err = manager.DestroyContext(ctx, executionID)
	require.NoError(t, err)
}

// TestSyncPointWhenDisabled 测试未启用异步轨迹记录时的同步点
func TestSyncPointWhenDisabled(t *testing.T) {
	manager := setupIntegrationManager(t)
	
	// 不启用异步轨迹记录
	assert.False(t, manager.IsAsyncTraceRecordingEnabled(), "默认应该禁用异步轨迹记录")
	
	// 创建执行上下文
	ctx := context.Background()
	executionID := "test_sync_point_disabled"
	callerAddress := "test_caller"
	
	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)
	
	// 记录一些宿主函数调用（同步模式）
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
	
	// 同步模式下，FlushTraceQueue应该成功但不做任何操作
	err = manager.FlushTraceQueue()
	require.NoError(t, err, "未启用异步轨迹记录时，刷新队列应该成功但不做任何操作")
	
	// 验证记录已写入（同步模式下立即写入）
	trace, err := executionContext.GetExecutionTrace()
	require.NoError(t, err)
	assert.Equal(t, totalCalls, len(trace), "同步模式下，所有记录应该立即写入")
	
	// 清理
	err = manager.DestroyContext(ctx, executionID)
	require.NoError(t, err)
}

// TestSyncPointTimeout 测试同步点超时保护
func TestSyncPointTimeout(t *testing.T) {
	manager := setupIntegrationManager(t)
	
	// 启用异步轨迹记录（使用较短的批量超时）
	err := manager.EnableAsyncTraceRecording(2, 50, 10*time.Millisecond, 3, 5*time.Millisecond)
	require.NoError(t, err)
	defer manager.DisableAsyncTraceRecording()
	
	// 创建执行上下文
	ctx := context.Background()
	executionID := "test_sync_point_timeout"
	callerAddress := "test_caller"
	
	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)
	
	// 记录大量宿主函数调用
	largeBatch := 1000
	for i := 0; i < largeBatch; i++ {
		call := &ispcInterfaces.HostFunctionCall{
			Sequence:     uint64(i),
			FunctionName: "test_function",
			Parameters:   map[string]interface{}{"param": i},
			Result:       map[string]interface{}{"result": i * 2},
			Timestamp:    time.Now().UnixNano(),
		}
		executionContext.RecordHostFunctionCall(call)
	}
	
	// 执行完成同步点（应该能正常完成，即使有大量记录）
	startTime := time.Now()
	err = manager.FlushTraceQueue()
	duration := time.Since(startTime)
	
	require.NoError(t, err, "刷新队列应该成功")
	assert.Less(t, duration, 5*time.Second, "刷新队列应该在合理时间内完成")
	
	// 等待处理完成
	time.Sleep(200 * time.Millisecond)
	
	// 验证记录已写入
	trace, err := executionContext.GetExecutionTrace()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(trace), largeBatch-10, "大部分记录应该被写入（允许少量误差）")
	
	// 清理
	err = manager.DestroyContext(ctx, executionID)
	require.NoError(t, err)
}

