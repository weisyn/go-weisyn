package context

import (
	"sync"
	"testing"
	"time"
)

// ============================================================================
// LockFreeQueue性能基准测试（异步轨迹记录优化 - 阶段1测试）
// ============================================================================
//
// 🎯 **测试目的**：
// 测试LockFreeQueue的性能，包括入队、出队、批量出队等操作的性能。
//
// 🔧 **使用方法**：
// - 运行基准测试：`go test -bench=. -benchmem ./internal/core/ispc/context`
// - 运行特定测试：`go test -bench=BenchmarkLockFreeQueueEnqueue ./internal/core/ispc/context`
//
// ============================================================================

// BenchmarkLockFreeQueueEnqueue 基准测试：入队性能
func BenchmarkLockFreeQueueEnqueue(b *testing.B) {
	queue := NewLockFreeQueue()
	
	record := &TraceRecord{
		RecordType:  "host_function_call",
		ExecutionID: "execution-1",
		HostFunctionCall: &HostFunctionCall{
			FunctionName: "test_function",
			Duration:     time.Millisecond,
		},
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		queue.Enqueue(record)
	}
}

// BenchmarkLockFreeQueueDequeue 基准测试：出队性能
func BenchmarkLockFreeQueueDequeue(b *testing.B) {
	queue := NewLockFreeQueue()
	
	// 预先入队一些记录
	record := &TraceRecord{
		RecordType:  "host_function_call",
		ExecutionID: "execution-1",
		HostFunctionCall: &HostFunctionCall{
			FunctionName: "test_function",
			Duration:     time.Millisecond,
		},
	}
	
	for i := 0; i < b.N; i++ {
		queue.Enqueue(record)
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		queue.Dequeue()
	}
}

// BenchmarkLockFreeQueueDequeueBatch 基准测试：批量出队性能
func BenchmarkLockFreeQueueDequeueBatch(b *testing.B) {
	queue := NewLockFreeQueue()
	
	// 预先入队一些记录
	record := &TraceRecord{
		RecordType:  "host_function_call",
		ExecutionID: "execution-1",
		HostFunctionCall: &HostFunctionCall{
			FunctionName: "test_function",
			Duration:     time.Millisecond,
		},
	}
	
	batchSize := 100
	totalRecords := b.N * batchSize
	
	for i := 0; i < totalRecords; i++ {
		queue.Enqueue(record)
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		queue.DequeueBatch(batchSize)
	}
}

// BenchmarkLockFreeQueueConcurrentEnqueue 基准测试：并发入队性能
func BenchmarkLockFreeQueueConcurrentEnqueue(b *testing.B) {
	queue := NewLockFreeQueue()
	
	record := &TraceRecord{
		RecordType:  "host_function_call",
		ExecutionID: "execution-1",
		HostFunctionCall: &HostFunctionCall{
			FunctionName: "test_function",
			Duration:     time.Millisecond,
		},
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			queue.Enqueue(record)
		}
	})
}

// BenchmarkLockFreeQueueConcurrentDequeue 基准测试：并发出队性能
func BenchmarkLockFreeQueueConcurrentDequeue(b *testing.B) {
	queue := NewLockFreeQueue()
	
	// 预先入队一些记录
	record := &TraceRecord{
		RecordType:  "host_function_call",
		ExecutionID: "execution-1",
		HostFunctionCall: &HostFunctionCall{
			FunctionName: "test_function",
			Duration:     time.Millisecond,
		},
	}
	
	for i := 0; i < b.N; i++ {
		queue.Enqueue(record)
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			queue.Dequeue()
		}
	})
}

// BenchmarkLockFreeQueueConcurrentEnqueueDequeue 基准测试：并发入队和出队性能
func BenchmarkLockFreeQueueConcurrentEnqueueDequeue(b *testing.B) {
	queue := NewLockFreeQueue()
	
	record := &TraceRecord{
		RecordType:  "host_function_call",
		ExecutionID: "execution-1",
		HostFunctionCall: &HostFunctionCall{
			FunctionName: "test_function",
			Duration:     time.Millisecond,
		},
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	var wg sync.WaitGroup
	wg.Add(2)
	
	// 入队goroutine
	go func() {
		defer wg.Done()
		for i := 0; i < b.N; i++ {
			queue.Enqueue(record)
		}
	}()
	
	// 出队goroutine
	go func() {
		defer wg.Done()
		for i := 0; i < b.N; i++ {
			queue.Dequeue()
		}
	}()
	
	wg.Wait()
}

// BenchmarkLockFreeQueueFlush 基准测试：队列刷新性能
func BenchmarkLockFreeQueueFlush(b *testing.B) {
	queue := NewLockFreeQueue()
	
	record := &TraceRecord{
		RecordType:  "host_function_call",
		ExecutionID: "execution-1",
		HostFunctionCall: &HostFunctionCall{
			FunctionName: "test_function",
			Duration:     time.Millisecond,
		},
	}
	
	recordsPerIteration := 1000
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		// 入队一些记录
		for j := 0; j < recordsPerIteration; j++ {
			queue.Enqueue(record)
		}
		
		// 刷新队列
		queue.Flush()
	}
}

// BenchmarkLockFreeQueueGetStats 基准测试：统计信息获取性能
func BenchmarkLockFreeQueueGetStats(b *testing.B) {
	queue := NewLockFreeQueue()
	
	record := &TraceRecord{
		RecordType:  "host_function_call",
		ExecutionID: "execution-1",
		HostFunctionCall: &HostFunctionCall{
			FunctionName: "test_function",
			Duration:     time.Millisecond,
		},
	}
	
	// 预先入队一些记录
	for i := 0; i < 1000; i++ {
		queue.Enqueue(record)
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		queue.GetStats()
	}
}

// BenchmarkLockFreeQueueSize 基准测试：队列大小计算性能
func BenchmarkLockFreeQueueSize(b *testing.B) {
	queue := NewLockFreeQueue()
	
	record := &TraceRecord{
		RecordType:  "host_function_call",
		ExecutionID: "execution-1",
		HostFunctionCall: &HostFunctionCall{
			FunctionName: "test_function",
			Duration:     time.Millisecond,
		},
	}
	
	// 预先入队一些记录
	for i := 0; i < 1000; i++ {
		queue.Enqueue(record)
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		queue.Size()
	}
}

