package context

import (
	"fmt"
	"sync"
	"testing"
	"time"

	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	"github.com/weisyn/v1/internal/core/ispc/testutil"
	"github.com/weisyn/v1/pkg/types"
)

// ============================================================================
// 执行轨迹记录性能基准测试
// ============================================================================
//
// 🎯 **目的**：
//   - 用于开发阶段的性能分析和优化
//   - 同步vs异步性能对比测试
//   - 性能回归测试
//
// 📋 **注意**：
//   - 这些是开发工具，不是生产监控
//   - 基准测试需要Mock依赖，避免真实执行
//   - 使用`go test -bench=. -benchmem`运行
//   - 使用`go test -bench=. -cpuprofile=cpu.prof`生成性能分析文件
//
// 🔧 **使用方法**：
//   - 运行所有基准测试：`go test -bench=. ./internal/core/ispc/context`
//   - 运行特定测试：`go test -bench=BenchmarkTraceRecording ./internal/core/ispc/context`
//   - 生成CPU分析：`go test -bench=. -cpuprofile=cpu.prof ./internal/core/ispc/context`
//   - 查看分析结果：`go tool pprof cpu.prof`
//
// ⚠️ **限制**：
//   - 当前仅测试同步记录的性能
//   - 异步记录实现后，将添加异步记录的性能测试
// ============================================================================

// ============================================================================
//
// ✅ **重构说明**：使用testutil包中的统一Mock对象，遵循测试规范
//
// ============================================================================

// setupBenchmarkContext 创建用于基准测试的contextImpl实例
//
// ✅ **重构说明**：使用testutil包中的统一Mock对象，遵循测试规范
func setupBenchmarkContext(b *testing.B) *contextImpl {
	clock := testutil.NewTestClock()
	logger := testutil.NewTestLogger()

	// 创建最小化的Manager（仅用于测试）
	// 注意：这里简化处理，直接创建contextImpl，避免实现所有接口
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
		traceIntegrityChecker: NewTraceIntegrityChecker(nil),
	}

	// 创建contextImpl实例
	execCtx := &contextImpl{
		executionID:   "benchmark_execution_id",
		createdAt:     time.Now(),
		expiresAt:     time.Now().Add(30 * time.Second),
		hasDeadline:   false,
		callerAddress: []byte("benchmark_caller_address"),
		manager:       manager,
		mutex:         sync.RWMutex{},
		resourceUsage: &types.ResourceUsage{
			StartTime: time.Now(),
		},
		deterministicEnforcer: manager.CreateDeterministicEnforcer("benchmark_execution_id", nil, nil),
	}

	return execCtx
}


// createMockHostFunctionCall 创建Mock的宿主函数调用
func createMockHostFunctionCall(sequence uint64, functionName string) *ispcInterfaces.HostFunctionCall {
	return &ispcInterfaces.HostFunctionCall{
		Sequence:     sequence,
		FunctionName: functionName,
		Parameters: map[string]interface{}{
			"param1": "value1",
			"param2": 123,
		},
		Result: map[string]interface{}{
			"result": "success",
		},
		Timestamp: time.Now().UnixNano(),
	}
}

// ============================================================================
// 基准测试：同步轨迹记录
// ============================================================================

// BenchmarkTraceRecording_Sync 基准测试：同步轨迹记录
//
// 🎯 **用途**：测试当前同步记录的性能
func BenchmarkTraceRecording_Sync(b *testing.B) {
	execCtx := setupBenchmarkContext(b)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		call := createMockHostFunctionCall(uint64(i), "test_function")
		execCtx.RecordHostFunctionCall(call)
	}
}

// BenchmarkTraceRecording_Sync_Parallel 并行基准测试：同步轨迹记录
//
// 🎯 **用途**：测试并发场景下同步记录的性能
func BenchmarkTraceRecording_Sync_Parallel(b *testing.B) {
	execCtx := setupBenchmarkContext(b)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		sequence := uint64(0)
		for pb.Next() {
			call := createMockHostFunctionCall(sequence, "test_function")
			execCtx.RecordHostFunctionCall(call)
			sequence++
		}
	})
}

// BenchmarkTraceRecording_Sync_HighFrequency 基准测试：高频同步轨迹记录
//
// 🎯 **用途**：测试高频调用场景下同步记录的性能
func BenchmarkTraceRecording_Sync_HighFrequency(b *testing.B) {
	execCtx := setupBenchmarkContext(b)

	b.ResetTimer()
	b.ReportAllocs()

	// 模拟高频调用：每次迭代记录多个调用
	for i := 0; i < b.N; i++ {
		for j := 0; j < 10; j++ {
			call := createMockHostFunctionCall(uint64(i*10+j), "test_function")
			execCtx.RecordHostFunctionCall(call)
		}
	}
}

// BenchmarkStateChangeRecording_Sync 基准测试：同步状态变更记录
//
// 🎯 **用途**：测试状态变更记录的性能
func BenchmarkStateChangeRecording_Sync(b *testing.B) {
	execCtx := setupBenchmarkContext(b)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		execCtx.RecordStateChange("utxo_create", "key_"+string(rune(i)), nil, "new_value")
	}
}

// BenchmarkGetExecutionTrace_Sync 基准测试：获取执行轨迹（同步记录）
//
// 🎯 **用途**：测试获取执行轨迹的性能
func BenchmarkGetExecutionTrace_Sync(b *testing.B) {
	execCtx := setupBenchmarkContext(b)

	// 预先记录一些调用
	for i := 0; i < 100; i++ {
		call := createMockHostFunctionCall(uint64(i), "test_function")
		execCtx.RecordHostFunctionCall(call)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := execCtx.GetExecutionTrace()
		if err != nil {
			b.Fatalf("获取执行轨迹失败: %v", err)
		}
	}
}

// ============================================================================
// 基准测试：异步轨迹记录（预留接口）
// ============================================================================
//
// ⚠️ **注意**：异步轨迹记录功能尚未实现，这些测试当前会跳过
// 异步记录实现后，将启用这些测试

// BenchmarkTraceRecording_Async 基准测试：异步轨迹记录
//
// 🎯 **用途**：测试异步记录的性能（待实现）
func BenchmarkTraceRecording_Async(b *testing.B) {
	b.Skip("异步轨迹记录功能尚未实现，待异步记录实现后启用")
}

// BenchmarkTraceRecording_Async_Parallel 并行基准测试：异步轨迹记录
//
// 🎯 **用途**：测试并发场景下异步记录的性能（待实现）
func BenchmarkTraceRecording_Async_Parallel(b *testing.B) {
	b.Skip("异步轨迹记录功能尚未实现，待异步记录实现后启用")
}

// BenchmarkTraceRecording_Async_HighFrequency 基准测试：高频异步轨迹记录
//
// 🎯 **用途**：测试高频调用场景下异步记录的性能（待实现）
func BenchmarkTraceRecording_Async_HighFrequency(b *testing.B) {
	b.Skip("异步轨迹记录功能尚未实现，待异步记录实现后启用")
}

// ============================================================================
// 基准测试：性能对比工具
// ============================================================================

// BenchmarkTraceRecording_Comparison 基准测试：同步vs异步性能对比
//
// 🎯 **用途**：对比同步和异步记录的性能差异
func BenchmarkTraceRecording_Comparison(b *testing.B) {
	b.Skip("异步轨迹记录功能尚未实现，待异步记录实现后启用对比测试")
}

// ============================================================================
// 基准测试：轨迹记录各阶段耗时统计
// ============================================================================

// BenchmarkTraceRecording_Timing 基准测试：轨迹记录各阶段耗时统计
//
// 🎯 **用途**：分析轨迹记录各阶段的耗时分布
func BenchmarkTraceRecording_Timing(b *testing.B) {
	execCtx := setupBenchmarkContext(b)

	// 记录各阶段耗时
	var lockTime time.Duration
	var recordTime time.Duration
	var unlockTime time.Duration

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		call := createMockHostFunctionCall(uint64(i), "test_function")

		// 1. 加锁时间
		lockStart := time.Now()
		execCtx.mutex.Lock()
		lockTime += time.Since(lockStart)

		// 2. 记录时间
		recordStart := time.Now()
		execCtx.RecordHostFunctionCall(call)
		recordTime += time.Since(recordStart)

		// 3. 解锁时间
		unlockStart := time.Now()
		execCtx.mutex.Unlock()
		unlockTime += time.Since(unlockStart)
	}

	// 输出各阶段平均耗时
	b.Logf("平均加锁耗时: %v", lockTime/time.Duration(b.N))
	b.Logf("平均记录耗时: %v", recordTime/time.Duration(b.N))
	b.Logf("平均解锁耗时: %v", unlockTime/time.Duration(b.N))
}

// ============================================================================
// 基准测试：内存分配分析
// ============================================================================

// BenchmarkTraceRecording_Memory 基准测试：轨迹记录内存分配分析
//
// 🎯 **用途**：分析轨迹记录过程中的内存分配情况
func BenchmarkTraceRecording_Memory(b *testing.B) {
	execCtx := setupBenchmarkContext(b)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		call := createMockHostFunctionCall(uint64(i), "test_function")
		execCtx.RecordHostFunctionCall(call)
	}
}

// ============================================================================
// 基准测试：不同调用频率的性能对比
// ============================================================================

// BenchmarkTraceRecording_LowFrequency 基准测试：低频调用（每次迭代1次调用）
func BenchmarkTraceRecording_LowFrequency(b *testing.B) {
	execCtx := setupBenchmarkContext(b)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		call := createMockHostFunctionCall(uint64(i), "test_function")
		execCtx.RecordHostFunctionCall(call)
	}
}

// BenchmarkTraceRecording_MediumFrequency 基准测试：中频调用（每次迭代10次调用）
func BenchmarkTraceRecording_MediumFrequency(b *testing.B) {
	execCtx := setupBenchmarkContext(b)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for j := 0; j < 10; j++ {
			call := createMockHostFunctionCall(uint64(i*10+j), "test_function")
			execCtx.RecordHostFunctionCall(call)
		}
	}
}

// BenchmarkTraceRecording_HighFrequency 基准测试：高频调用（每次迭代100次调用）
func BenchmarkTraceRecording_HighFrequency(b *testing.B) {
	execCtx := setupBenchmarkContext(b)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for j := 0; j < 100; j++ {
			call := createMockHostFunctionCall(uint64(i*100+j), "test_function")
			execCtx.RecordHostFunctionCall(call)
		}
	}
}

// ============================================================================
// 基准测试：并发性能对比
// ============================================================================

// BenchmarkTraceRecording_ConcurrencyComparison 基准测试：并发性能对比
//
// 🎯 **用途**：对比不同并发度下的性能
func BenchmarkTraceRecording_ConcurrencyComparison(b *testing.B) {
	execCtx := setupBenchmarkContext(b)

	// 测试不同并发度
	concurrencies := []int{1, 2, 4, 8, 16}

	for _, concurrency := range concurrencies {
		b.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(b *testing.B) {
			var wg sync.WaitGroup
			callsPerGoroutine := b.N / concurrency

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func(goroutineID int) {
					defer wg.Done()
					for j := 0; j < callsPerGoroutine; j++ {
						sequence := uint64(goroutineID*callsPerGoroutine + j)
						call := createMockHostFunctionCall(sequence, "test_function")
						execCtx.RecordHostFunctionCall(call)
					}
				}(i)
			}

			wg.Wait()
		})
	}
}

// ============================================================================
// 基准测试：性能回归测试辅助函数
// ============================================================================

// compareTraceRecordingResults 比较轨迹记录性能结果
//
// 🎯 **用途**：用于性能回归测试，比较当前结果与历史结果
func compareTraceRecordingResults(current, baseline map[string]float64) map[string]float64 {
	comparison := make(map[string]float64)

	for key, currentValue := range current {
		if baselineValue, exists := baseline[key]; exists {
			// 计算性能变化百分比（正值表示变慢，负值表示变快）
			changePercent := ((currentValue - baselineValue) / baselineValue) * 100
			comparison[key] = changePercent
		}
	}

	return comparison
}

// recordTraceRecordingBaseline 记录轨迹记录性能基线
//
// 🎯 **用途**：记录当前性能作为基线，用于后续回归测试
func recordTraceRecordingBaseline(results map[string]float64) {
	// 这里可以将结果保存到文件或数据库中
	// 用于后续的性能回归测试
}

// ============================================================================
// 基准测试：轨迹完整性检查性能
// ============================================================================

// BenchmarkTraceIntegrityCheck 基准测试：轨迹完整性检查性能
//
// 🎯 **用途**：测试轨迹完整性检查的性能开销
func BenchmarkTraceIntegrityCheck(b *testing.B) {
	execCtx := setupBenchmarkContext(b)
	manager := execCtx.manager

	// 预先记录一些调用和状态变更
	for i := 0; i < 100; i++ {
		call := createMockHostFunctionCall(uint64(i), "test_function")
		execCtx.RecordHostFunctionCall(call)
		execCtx.RecordStateChange("utxo_create", "key_"+string(rune(i)), nil, "new_value")
	}

	// 构建执行轨迹
	trace := &ExecutionTrace{
		ExecutionID:       execCtx.executionID,
		StartTime:         execCtx.createdAt,
		EndTime:           execCtx.createdAt.Add(100 * time.Millisecond),
		HostFunctionCalls: execCtx.hostFunctionCalls,
		StateChanges:      execCtx.stateChanges,
		ExecutionEvents:   execCtx.executionEvents,
		TotalDuration:     100 * time.Millisecond,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := manager.CheckTraceIntegrity(trace)
		if err != nil {
			b.Fatalf("轨迹完整性检查失败: %v", err)
		}
	}
}

// BenchmarkTraceValidation 基准测试：轨迹验证性能
//
// 🎯 **用途**：测试轨迹验证的性能开销
func BenchmarkTraceValidation(b *testing.B) {
	execCtx := setupBenchmarkContext(b)
	manager := execCtx.manager

	// 预先记录一些调用和状态变更
	for i := 0; i < 100; i++ {
		call := createMockHostFunctionCall(uint64(i), "test_function")
		execCtx.RecordHostFunctionCall(call)
		execCtx.RecordStateChange("utxo_create", "key_"+string(rune(i)), nil, "new_value")
	}

	// 构建执行轨迹
	trace := &ExecutionTrace{
		ExecutionID:       execCtx.executionID,
		StartTime:         execCtx.createdAt,
		EndTime:           execCtx.createdAt.Add(100 * time.Millisecond),
		HostFunctionCalls: execCtx.hostFunctionCalls,
		StateChanges:      execCtx.stateChanges,
		ExecutionEvents:   execCtx.executionEvents,
		TotalDuration:     100 * time.Millisecond,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = manager.ValidateTrace(trace)
	}
}

