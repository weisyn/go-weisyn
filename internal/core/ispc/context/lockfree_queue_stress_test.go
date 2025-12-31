package context

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// LockFreeQueue压力测试（异步轨迹记录优化 - 阶段1测试）
// ============================================================================
//
// 🎯 **测试目的**：
// 测试LockFreeQueue在高并发场景下的稳定性和性能。
//
// ⚠️ **注意**：
// - 这些测试会创建大量goroutine，需要足够的系统资源
// - 测试时间可能较长
//
// ============================================================================

// TestLockFreeQueueStressHighConcurrency 高并发压力测试
func TestLockFreeQueueStressHighConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过压力测试（使用-short标志）")
	}
	
	queue := NewLockFreeQueue()
	
	// 高并发参数
	concurrency := 1000
	recordsPerGoroutine := 100
	
	var wg sync.WaitGroup
	wg.Add(concurrency)
	
	// 启动大量goroutine并发入队
	startTime := time.Now()
	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < recordsPerGoroutine; j++ {
				record := &TraceRecord{
					RecordType:  "host_function_call",
					ExecutionID: "execution-1",
					HostFunctionCall: &HostFunctionCall{
						FunctionName: "test_function",
						Duration:     time.Duration(j) * time.Millisecond,
					},
				}
				success := queue.Enqueue(record)
				require.True(t, success, "入队应该成功")
			}
		}(i)
	}
	
	// 等待所有goroutine完成
	wg.Wait()
	duration := time.Since(startTime)
	
	// 验证统计信息
	stats := queue.GetStats()
	expectedEnqueueCount := int64(concurrency * recordsPerGoroutine)
	assert.Equal(t, expectedEnqueueCount, stats["enqueue_count"], "入队计数应该正确")
	
	t.Logf("高并发压力测试完成: %d个goroutine, %d条记录/goroutine, 耗时: %v",
		concurrency, recordsPerGoroutine, duration)
}

// TestLockFreeQueueStressMixedOperations 混合操作压力测试
func TestLockFreeQueueStressMixedOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过压力测试（使用-short标志）")
	}
	
	queue := NewLockFreeQueue()
	
	// 混合操作参数
	enqueueGoroutines := 500
	dequeueGoroutines := 500
	recordsPerGoroutine := 200
	
	var wg sync.WaitGroup
	wg.Add(enqueueGoroutines + dequeueGoroutines)
	
	// 启动入队goroutine
	startTime := time.Now()
	for i := 0; i < enqueueGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < recordsPerGoroutine; j++ {
				record := &TraceRecord{
					RecordType:  "host_function_call",
					ExecutionID: "execution-1",
					HostFunctionCall: &HostFunctionCall{
						FunctionName: "test_function",
						Duration:     time.Duration(j) * time.Millisecond,
					},
				}
				queue.Enqueue(record)
			}
		}(i)
	}
	
	// 启动出队goroutine
	dequeuedCount := int64(0)
	var dequeuedMutex sync.Mutex
	
	for i := 0; i < dequeueGoroutines; i++ {
		go func() {
			defer wg.Done()
			for {
				record := queue.Dequeue()
				if record == nil {
					// 等待一段时间，确保所有入队操作完成
					time.Sleep(50 * time.Millisecond)
					record = queue.Dequeue()
					if record == nil {
						break
					}
				}
				dequeuedMutex.Lock()
				dequeuedCount++
				dequeuedMutex.Unlock()
			}
		}()
	}
	
	// 等待所有goroutine完成
	wg.Wait()
	duration := time.Since(startTime)
	
	// 验证出队数量（应该等于入队数量）
	expectedDequeuedCount := int64(enqueueGoroutines * recordsPerGoroutine)
	assert.Equal(t, expectedDequeuedCount, dequeuedCount, "所有记录都应该被出队")
	
	// 验证统计信息
	stats := queue.GetStats()
	assert.Equal(t, expectedDequeuedCount, stats["enqueue_count"], "入队计数应该正确")
	assert.Equal(t, expectedDequeuedCount, stats["dequeue_count"], "出队计数应该正确")
	assert.Equal(t, int64(0), stats["size"], "队列应该为空")
	
	t.Logf("混合操作压力测试完成: %d个入队goroutine, %d个出队goroutine, %d条记录/goroutine, 耗时: %v",
		enqueueGoroutines, dequeueGoroutines, recordsPerGoroutine, duration)
}

// TestLockFreeQueueStressBatchOperations 批量操作压力测试
func TestLockFreeQueueStressBatchOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过压力测试（使用-short标志）")
	}
	
	queue := NewLockFreeQueue()
	
	// 批量操作参数
	enqueueGoroutines := 100
	dequeueGoroutines := 10
	recordsPerGoroutine := 1000
	batchSize := 100
	
	var wg sync.WaitGroup
	wg.Add(enqueueGoroutines + dequeueGoroutines)
	
	// 启动入队goroutine
	startTime := time.Now()
	for i := 0; i < enqueueGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < recordsPerGoroutine; j++ {
				record := &TraceRecord{
					RecordType:  "host_function_call",
					ExecutionID: "execution-1",
					HostFunctionCall: &HostFunctionCall{
						FunctionName: "test_function",
						Duration:     time.Duration(j) * time.Millisecond,
					},
				}
				queue.Enqueue(record)
			}
		}(i)
	}
	
	// 启动批量出队goroutine
	dequeuedCount := int64(0)
	var dequeuedMutex sync.Mutex
	
	for i := 0; i < dequeueGoroutines; i++ {
		go func() {
			defer wg.Done()
			for {
				batch := queue.DequeueBatch(batchSize)
				if len(batch) == 0 {
					// 等待一段时间，确保所有入队操作完成
					time.Sleep(100 * time.Millisecond)
					batch = queue.DequeueBatch(batchSize)
					if len(batch) == 0 {
						break
					}
				}
				dequeuedMutex.Lock()
				dequeuedCount += int64(len(batch))
				dequeuedMutex.Unlock()
			}
		}()
	}
	
	// 等待所有goroutine完成
	wg.Wait()
	duration := time.Since(startTime)
	
	// 验证出队数量（应该等于入队数量）
	expectedDequeuedCount := int64(enqueueGoroutines * recordsPerGoroutine)
	assert.Equal(t, expectedDequeuedCount, dequeuedCount, "所有记录都应该被出队")
	
	// 验证统计信息
	stats := queue.GetStats()
	assert.Equal(t, expectedDequeuedCount, stats["enqueue_count"], "入队计数应该正确")
	assert.Equal(t, expectedDequeuedCount, stats["dequeue_count"], "出队计数应该正确")
	assert.Equal(t, int64(0), stats["size"], "队列应该为空")
	
	t.Logf("批量操作压力测试完成: %d个入队goroutine, %d个出队goroutine, %d条记录/goroutine, 批量大小: %d, 耗时: %v",
		enqueueGoroutines, dequeueGoroutines, recordsPerGoroutine, batchSize, duration)
}

// TestLockFreeQueueStressLongRunning 长时间运行压力测试
func TestLockFreeQueueStressLongRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过压力测试（使用-short标志）")
	}
	
	queue := NewLockFreeQueue()
	
	// 长时间运行参数
	concurrency := 100
	duration := 5 * time.Second
	
	var wg sync.WaitGroup
	wg.Add(concurrency * 2) // 入队和出队各concurrency个goroutine
	
	// 启动入队goroutine
	enqueueCount := int64(0)
	var enqueueMutex sync.Mutex
	
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			startTime := time.Now()
			for time.Since(startTime) < duration {
				record := &TraceRecord{
					RecordType:  "host_function_call",
					ExecutionID: "execution-1",
					HostFunctionCall: &HostFunctionCall{
						FunctionName: "test_function",
						Duration:     time.Millisecond,
					},
				}
				if queue.Enqueue(record) {
					enqueueMutex.Lock()
					enqueueCount++
					enqueueMutex.Unlock()
				}
			}
		}()
	}
	
	// 启动出队goroutine
	dequeuedCount := int64(0)
	var dequeuedMutex sync.Mutex
	
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			startTime := time.Now()
			for time.Since(startTime) < duration {
				record := queue.Dequeue()
				if record != nil {
					dequeuedMutex.Lock()
					dequeuedCount++
					dequeuedMutex.Unlock()
				} else {
					time.Sleep(1 * time.Millisecond) // 避免CPU空转
				}
			}
		}()
	}
	
	// 等待所有goroutine完成
	wg.Wait()
	
	// 验证统计信息
	stats := queue.GetStats()
	assert.Equal(t, enqueueCount, stats["enqueue_count"], "入队计数应该正确")
	assert.Equal(t, dequeuedCount, stats["dequeue_count"], "出队计数应该正确")
	
	t.Logf("长时间运行压力测试完成: %d个goroutine, 运行时间: %v, 入队: %d, 出队: %d",
		concurrency*2, duration, enqueueCount, dequeuedCount)
}

