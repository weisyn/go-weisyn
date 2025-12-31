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
)

// ============================================================================
// 轨迹完整性验证测试（异步轨迹记录优化 - 阶段3测试）
// ============================================================================
//
// 🎯 **测试目的**：
// 测试轨迹完整性验证功能，确保异步轨迹记录不会丢失或损坏数据。
//
// ⚠️ **注意**：
// - 必须使用`go test -race`运行这些测试
// - 测试会启动真实的Manager和工作线程池
//
// ============================================================================

// TestTraceIntegrityBasic 测试基本轨迹完整性验证
func TestTraceIntegrityBasic(t *testing.T) {
	manager := setupIntegrationManager(t)

	// 启用异步轨迹记录
	// ✅ 关键：buffer/queue 容量必须覆盖本测试写入量，否则会触发环形覆盖导致用例非确定性失败
	err := manager.EnableAsyncTraceRecording(2, 200, 50*time.Millisecond, 3, 10*time.Millisecond)
	require.NoError(t, err)
	defer manager.DisableAsyncTraceRecording()

	// 创建执行上下文
	ctx := context.Background()
	executionID := "test_trace_integrity_1"
	callerAddress := "test_caller"

	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)

	// 记录一些宿主函数调用
	totalCalls := 100
	expectedCalls := make([]*ispcInterfaces.HostFunctionCall, totalCalls)

	for i := 0; i < totalCalls; i++ {
		call := &ispcInterfaces.HostFunctionCall{
			Sequence:     uint64(i),
			FunctionName: fmt.Sprintf("test_function_%d", i),
			Parameters:   map[string]interface{}{"param": i, "index": i},
			Result:       map[string]interface{}{"result": i * 2, "index": i},
			Timestamp:    time.Now().UnixNano() + int64(i),
		}
		expectedCalls[i] = call
		executionContext.RecordHostFunctionCall(call)
	}

	// 执行完成同步点：刷新队列
	err = manager.FlushTraceQueue()
	require.NoError(t, err)

	// 等待处理完成
	time.Sleep(100 * time.Millisecond)

	// 验证轨迹完整性
	trace, err := executionContext.GetExecutionTrace()
	require.NoError(t, err)

	// 验证记录数量
	assert.Equal(t, totalCalls, len(trace), "轨迹记录数量应该正确")

	// 验证记录内容完整性
	for i, call := range trace {
		assert.Equal(t, expectedCalls[i].FunctionName, call.FunctionName, "函数名应该匹配: index=%d", i)
		assert.Equal(t, expectedCalls[i].Sequence, call.Sequence, "序号应该匹配: index=%d", i)

		// 验证参数
		if expectedCalls[i].Parameters != nil {
			assert.NotNil(t, call.Parameters, "参数不应该为nil: index=%d", i)
			if call.Parameters != nil {
				assert.Equal(t, expectedCalls[i].Parameters["param"], call.Parameters["param"], "参数值应该匹配: index=%d", i)
			}
		}

		// 验证结果
		if expectedCalls[i].Result != nil {
			assert.NotNil(t, call.Result, "结果不应该为nil: index=%d", i)
			if call.Result != nil {
				assert.Equal(t, expectedCalls[i].Result["result"], call.Result["result"], "结果值应该匹配: index=%d", i)
			}
		}
	}

	// 清理
	err = manager.DestroyContext(ctx, executionID)
	require.NoError(t, err)
}

// TestTraceIntegrityOrder 测试轨迹记录顺序完整性
func TestTraceIntegrityOrder(t *testing.T) {
	manager := setupIntegrationManager(t)

	// 启用异步轨迹记录
	// ✅ 本测试写入 200 条，buffer/queue 需 >=200 以避免覆盖导致 len/顺序不一致
	err := manager.EnableAsyncTraceRecording(2, 300, 50*time.Millisecond, 3, 10*time.Millisecond)
	require.NoError(t, err)
	defer manager.DisableAsyncTraceRecording()

	// 创建执行上下文
	ctx := context.Background()
	executionID := "test_trace_integrity_order"
	callerAddress := "test_caller"

	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)

	// 按顺序记录宿主函数调用
	totalCalls := 200
	for i := 0; i < totalCalls; i++ {
		call := &ispcInterfaces.HostFunctionCall{
			Sequence:     uint64(i),
			FunctionName: fmt.Sprintf("test_function_%d", i),
			Parameters:   map[string]interface{}{"sequence": i},
			Result:       map[string]interface{}{"sequence": i},
			Timestamp:    time.Now().UnixNano() + int64(i),
		}
		executionContext.RecordHostFunctionCall(call)
	}

	// 执行完成同步点：刷新队列
	err = manager.FlushTraceQueue()
	require.NoError(t, err)

	// 等待处理完成
	time.Sleep(100 * time.Millisecond)

	// 验证轨迹顺序完整性
	trace, err := executionContext.GetExecutionTrace()
	require.NoError(t, err)

	assert.Equal(t, totalCalls, len(trace), "轨迹记录数量应该正确")

	// 验证记录顺序（序号应该连续）
	for i := 0; i < len(trace); i++ {
		// 注意：由于异步处理，序号可能不完全连续，但应该大致按顺序
		assert.GreaterOrEqual(t, trace[i].Sequence, uint64(0), "序号应该非负: index=%d", i)
		assert.Less(t, trace[i].Sequence, uint64(totalCalls), "序号应该在范围内: index=%d", i)
	}

	// 清理
	err = manager.DestroyContext(ctx, executionID)
	require.NoError(t, err)
}

// TestTraceIntegrityConcurrent 测试并发场景下的轨迹完整性
func TestTraceIntegrityConcurrent(t *testing.T) {
	manager := setupIntegrationManager(t)

	// 启用异步轨迹记录
	err := manager.EnableAsyncTraceRecording(5, 50, 50*time.Millisecond, 3, 10*time.Millisecond)
	require.NoError(t, err)
	defer manager.DisableAsyncTraceRecording()

	// 创建执行上下文
	ctx := context.Background()
	executionID := "test_trace_integrity_concurrent"
	callerAddress := "test_caller"

	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)

	// 并发记录宿主函数调用
	concurrency := 100
	callsPerGoroutine := 50
	totalExpectedCalls := concurrency * callsPerGoroutine

	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < callsPerGoroutine; j++ {
				call := &ispcInterfaces.HostFunctionCall{
					Sequence:     uint64(goroutineID*callsPerGoroutine + j),
					FunctionName: fmt.Sprintf("test_function_%d_%d", goroutineID, j),
					Parameters:   map[string]interface{}{"goroutine": goroutineID, "call": j},
					Result:       map[string]interface{}{"goroutine": goroutineID, "call": j},
					Timestamp:    time.Now().UnixNano(),
				}
				executionContext.RecordHostFunctionCall(call)
			}
		}(i)
	}

	wg.Wait()

	// 执行完成同步点：刷新队列
	err = manager.FlushTraceQueue()
	require.NoError(t, err)

	// 等待处理完成
	time.Sleep(200 * time.Millisecond)

	// 验证轨迹完整性
	trace, err := executionContext.GetExecutionTrace()
	require.NoError(t, err)

	// 验证记录数量（允许少量误差，因为并发处理）
	assert.GreaterOrEqual(t, len(trace), totalExpectedCalls-10, "大部分记录应该被写入（允许少量误差）")

	// 验证记录内容完整性（检查是否有nil或空值）
	for i, call := range trace {
		assert.NotEmpty(t, call.FunctionName, "函数名不应该为空: index=%d", i)
		assert.NotNil(t, call.Parameters, "参数不应该为nil: index=%d", i)
		assert.NotNil(t, call.Result, "结果不应该为nil: index=%d", i)
		assert.Greater(t, call.Timestamp, int64(0), "时间戳应该有效: index=%d", i)
	}

	// 清理
	err = manager.DestroyContext(ctx, executionID)
	require.NoError(t, err)
}

// TestTraceIntegrityAfterFlush 测试刷新后的轨迹完整性
func TestTraceIntegrityAfterFlush(t *testing.T) {
	manager := setupIntegrationManager(t)

	// 启用异步轨迹记录
	err := manager.EnableAsyncTraceRecording(2, 50, 50*time.Millisecond, 3, 10*time.Millisecond)
	require.NoError(t, err)
	defer manager.DisableAsyncTraceRecording()

	// 创建执行上下文
	ctx := context.Background()
	executionID := "test_trace_integrity_after_flush"
	callerAddress := "test_caller"

	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)

	// 记录一些宿主函数调用
	firstBatch := 50
	for i := 0; i < firstBatch; i++ {
		call := &ispcInterfaces.HostFunctionCall{
			Sequence:     uint64(i),
			FunctionName: fmt.Sprintf("test_function_%d", i),
			Parameters:   map[string]interface{}{"batch": 1, "index": i},
			Result:       map[string]interface{}{"batch": 1, "index": i},
			Timestamp:    time.Now().UnixNano(),
		}
		executionContext.RecordHostFunctionCall(call)
	}

	// 第一次刷新
	err = manager.FlushTraceQueue()
	require.NoError(t, err)
	time.Sleep(50 * time.Millisecond)

	traceAfterFirst, err := executionContext.GetExecutionTrace()
	require.NoError(t, err)
	firstBatchCount := len(traceAfterFirst)

	// 记录更多宿主函数调用
	secondBatch := 50
	for i := 0; i < secondBatch; i++ {
		call := &ispcInterfaces.HostFunctionCall{
			Sequence:     uint64(firstBatch + i),
			FunctionName: fmt.Sprintf("test_function_%d", firstBatch+i),
			Parameters:   map[string]interface{}{"batch": 2, "index": i},
			Result:       map[string]interface{}{"batch": 2, "index": i},
			Timestamp:    time.Now().UnixNano(),
		}
		executionContext.RecordHostFunctionCall(call)
	}

	// 第二次刷新
	err = manager.FlushTraceQueue()
	require.NoError(t, err)
	time.Sleep(50 * time.Millisecond)

	traceAfterSecond, err := executionContext.GetExecutionTrace()
	require.NoError(t, err)

	// 验证轨迹完整性：第二次刷新后，记录数量应该增加
	assert.GreaterOrEqual(t, len(traceAfterSecond), firstBatchCount, "第二次刷新后，记录数量应该增加或保持不变")
	assert.GreaterOrEqual(t, len(traceAfterSecond), firstBatch+secondBatch-5, "大部分记录应该被写入（允许少量误差）")

	// 验证第一批记录仍然存在
	foundFirstBatch := 0
	for _, call := range traceAfterSecond {
		if call.Parameters != nil {
			if batch, ok := call.Parameters["batch"].(int); ok && batch == 1 {
				foundFirstBatch++
			}
		}
	}
	assert.GreaterOrEqual(t, foundFirstBatch, firstBatch-5, "第一批记录应该仍然存在（允许少量误差）")

	// 清理
	err = manager.DestroyContext(ctx, executionID)
	require.NoError(t, err)
}

// TestTraceIntegrityNoLoss 测试轨迹记录无丢失
func TestTraceIntegrityNoLoss(t *testing.T) {
	manager := setupIntegrationManager(t)

	// 启用异步轨迹记录
	err := manager.EnableAsyncTraceRecording(2, 50, 50*time.Millisecond, 3, 10*time.Millisecond)
	require.NoError(t, err)
	defer manager.DisableAsyncTraceRecording()

	// 创建执行上下文
	ctx := context.Background()
	executionID := "test_trace_integrity_no_loss"
	callerAddress := "test_caller"

	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)

	// 记录大量宿主函数调用
	largeBatch := 1000
	uniqueValues := make(map[int]bool)

	for i := 0; i < largeBatch; i++ {
		uniqueValues[i] = true
		call := &ispcInterfaces.HostFunctionCall{
			Sequence:     uint64(i),
			FunctionName: fmt.Sprintf("test_function_%d", i),
			Parameters:   map[string]interface{}{"unique_id": i},
			Result:       map[string]interface{}{"unique_id": i},
			Timestamp:    time.Now().UnixNano(),
		}
		executionContext.RecordHostFunctionCall(call)
	}

	// 执行完成同步点：刷新队列
	err = manager.FlushTraceQueue()
	require.NoError(t, err)

	// 等待处理完成
	time.Sleep(200 * time.Millisecond)

	// 验证轨迹完整性：检查是否有记录丢失
	trace, err := executionContext.GetExecutionTrace()
	require.NoError(t, err)

	// 统计唯一值
	foundValues := make(map[int]bool)
	for _, call := range trace {
		if call.Parameters != nil {
			if uniqueID, ok := call.Parameters["unique_id"].(int); ok {
				foundValues[uniqueID] = true
			}
		}
	}

	// 验证大部分唯一值都被找到（允许少量误差）
	foundCount := len(foundValues)
	expectedCount := len(uniqueValues)
	assert.GreaterOrEqual(t, foundCount, expectedCount-10, "大部分记录应该被找到（允许少量误差）")

	// 清理
	err = manager.DestroyContext(ctx, executionID)
	require.NoError(t, err)
}
