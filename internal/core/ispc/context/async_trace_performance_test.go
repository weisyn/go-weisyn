package context

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	"github.com/weisyn/v1/internal/core/ispc/testutil"
)

// ============================================================================
// 异步轨迹记录性能对比测试（异步轨迹记录优化 - 阶段3测试）
// ============================================================================
//
// 🎯 **测试目的**：
// 对比同步模式和异步模式的性能差异，验证异步轨迹记录的性能提升。
//
// ⚠️ **注意**：
// - 这些是性能基准测试，使用`go test -bench=.`运行
// - 对比同步vs异步的吞吐量和延迟
//
// ============================================================================

// setupBenchmarkManager 创建用于基准测试的Manager
//
// ✅ **重构说明**：使用testutil包中的统一Mock对象，遵循测试规范
func setupBenchmarkManager(b *testing.B) *Manager {
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

// BenchmarkSyncTraceRecording 基准测试：同步轨迹记录性能
func BenchmarkSyncTraceRecording(b *testing.B) {
	manager := setupBenchmarkManager(b)
	
	// 创建执行上下文
	ctx := context.Background()
	executionID := "benchmark_sync"
	callerAddress := "test_caller"
	
	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(b, err)
	
	// 准备测试数据
	call := &ispcInterfaces.HostFunctionCall{
		Sequence:     0,
		FunctionName: "test_function",
		Parameters:   map[string]interface{}{"param": 1},
		Result:       map[string]interface{}{"result": 2},
		Timestamp:    time.Now().UnixNano(),
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		call.Sequence = uint64(i)
		executionContext.RecordHostFunctionCall(call)
	}
	
	// 清理
	_ = manager.DestroyContext(ctx, executionID)
}

// BenchmarkAsyncTraceRecording 基准测试：异步轨迹记录性能
func BenchmarkAsyncTraceRecording(b *testing.B) {
	manager := setupBenchmarkManager(b)
	
	// 启用异步轨迹记录
	err := manager.EnableAsyncTraceRecording(2, 100, 50*time.Millisecond, 3, 10*time.Millisecond)
	require.NoError(b, err)
	defer manager.DisableAsyncTraceRecording()
	
	// 创建执行上下文
	ctx := context.Background()
	executionID := "benchmark_async"
	callerAddress := "test_caller"
	
	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(b, err)
	
	// 准备测试数据
	call := &ispcInterfaces.HostFunctionCall{
		Sequence:     0,
		FunctionName: "test_function",
		Parameters:   map[string]interface{}{"param": 1},
		Result:       map[string]interface{}{"result": 2},
		Timestamp:    time.Now().UnixNano(),
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		call.Sequence = uint64(i)
		executionContext.RecordHostFunctionCall(call)
	}
	
	// 刷新队列，确保所有记录都已写入
	_ = manager.FlushTraceQueue()
	
	// 清理
	_ = manager.DestroyContext(ctx, executionID)
}

// BenchmarkSyncTraceRecordingConcurrent 基准测试：并发同步轨迹记录性能
func BenchmarkSyncTraceRecordingConcurrent(b *testing.B) {
	manager := setupBenchmarkManager(b)
	
	// 创建执行上下文
	ctx := context.Background()
	executionID := "benchmark_sync_concurrent"
	callerAddress := "test_caller"
	
	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(b, err)
	
	// 准备测试数据
	call := &ispcInterfaces.HostFunctionCall{
		Sequence:     0,
		FunctionName: "test_function",
		Parameters:   map[string]interface{}{"param": 1},
		Result:       map[string]interface{}{"result": 2},
		Timestamp:    time.Now().UnixNano(),
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		i := int64(0)
		for pb.Next() {
			call.Sequence = uint64(i)
			executionContext.RecordHostFunctionCall(call)
			i++
		}
	})
	
	// 清理
	_ = manager.DestroyContext(ctx, executionID)
}

// BenchmarkAsyncTraceRecordingConcurrent 基准测试：并发异步轨迹记录性能
func BenchmarkAsyncTraceRecordingConcurrent(b *testing.B) {
	manager := setupBenchmarkManager(b)
	
	// 启用异步轨迹记录
	err := manager.EnableAsyncTraceRecording(5, 100, 50*time.Millisecond, 3, 10*time.Millisecond)
	require.NoError(b, err)
	defer manager.DisableAsyncTraceRecording()
	
	// 创建执行上下文
	ctx := context.Background()
	executionID := "benchmark_async_concurrent"
	callerAddress := "test_caller"
	
	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(b, err)
	
	// 准备测试数据
	call := &ispcInterfaces.HostFunctionCall{
		Sequence:     0,
		FunctionName: "test_function",
		Parameters:   map[string]interface{}{"param": 1},
		Result:       map[string]interface{}{"result": 2},
		Timestamp:    time.Now().UnixNano(),
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		i := int64(0)
		for pb.Next() {
			call.Sequence = uint64(i)
			executionContext.RecordHostFunctionCall(call)
			i++
		}
	})
	
	// 刷新队列，确保所有记录都已写入
	_ = manager.FlushTraceQueue()
	
	// 清理
	_ = manager.DestroyContext(ctx, executionID)
}

// TestSyncVsAsyncPerformanceComparison 测试：同步vs异步性能对比
func TestSyncVsAsyncPerformanceComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过性能对比测试（使用-short标志）")
	}
	
	manager := setupIntegrationManager(t)
	
	// 创建执行上下文
	ctx := context.Background()
	executionIDSync := "test_sync_perf"
	executionIDAsync := "test_async_perf"
	callerAddress := "test_caller"
	
	// 测试同步模式
	executionContextSync, err := manager.CreateContext(ctx, executionIDSync, callerAddress)
	require.NoError(t, err)
	
	callCount := 1000
	startTime := time.Now()
	
	for i := 0; i < callCount; i++ {
		call := &ispcInterfaces.HostFunctionCall{
			Sequence:     uint64(i),
			FunctionName: "test_function",
			Parameters:   map[string]interface{}{"param": i},
			Result:       map[string]interface{}{"result": i * 2},
			Timestamp:    time.Now().UnixNano(),
		}
		executionContextSync.RecordHostFunctionCall(call)
	}
	
	syncDuration := time.Since(startTime)
	
	// 验证同步模式记录
	traceSync, err := executionContextSync.GetExecutionTrace()
	require.NoError(t, err)
	assert.Equal(t, callCount, len(traceSync), "同步模式应该记录所有调用")
	
	// 清理同步上下文
	_ = manager.DestroyContext(ctx, executionIDSync)
	
	// 启用异步轨迹记录
	err = manager.EnableAsyncTraceRecording(2, 100, 50*time.Millisecond, 3, 10*time.Millisecond)
	require.NoError(t, err)
	defer manager.DisableAsyncTraceRecording()
	
	// 测试异步模式
	executionContextAsync, err := manager.CreateContext(ctx, executionIDAsync, callerAddress)
	require.NoError(t, err)
	
	startTime = time.Now()
	
	for i := 0; i < callCount; i++ {
		call := &ispcInterfaces.HostFunctionCall{
			Sequence:     uint64(i),
			FunctionName: "test_function",
			Parameters:   map[string]interface{}{"param": i},
			Result:       map[string]interface{}{"result": i * 2},
			Timestamp:    time.Now().UnixNano(),
		}
		executionContextAsync.RecordHostFunctionCall(call)
	}
	
	asyncDuration := time.Since(startTime)
	
	// 刷新队列
	err = manager.FlushTraceQueue()
	require.NoError(t, err)
	
	// 验证异步模式记录
	traceAsync, err := executionContextAsync.GetExecutionTrace()
	require.NoError(t, err)
	assert.Equal(t, callCount, len(traceAsync), "异步模式应该记录所有调用")
	
	// 清理异步上下文
	_ = manager.DestroyContext(ctx, executionIDAsync)
	
	// 输出性能对比
	t.Logf("性能对比（%d次调用）:", callCount)
	t.Logf("  同步模式耗时: %v", syncDuration)
	t.Logf("  异步模式耗时: %v", asyncDuration)
	t.Logf("  性能提升: %.2f%%", float64(syncDuration-asyncDuration)/float64(syncDuration)*100)
	
	// 验证异步模式应该更快（或至少不慢）
	assert.LessOrEqual(t, asyncDuration, syncDuration*2, "异步模式不应该明显慢于同步模式")
}

