package context

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// LockFreeQueue并发测试（异步轨迹记录优化 - 阶段1测试）
// ============================================================================
//
// 🎯 **测试目的**：
// 测试LockFreeQueue的并发安全性，使用race detector（-race flag）检测数据竞争。
//
// ⚠️ **注意**：
// - 必须使用`go test -race`运行这些测试
// - 测试会启动多个goroutine并发操作队列
// - 验证无数据竞争和ABA问题
//
// ============================================================================

// TestLockFreeQueueConcurrentEnqueue 测试并发入队
func TestLockFreeQueueConcurrentEnqueue(t *testing.T) {
	queue := NewLockFreeQueue()
	
	// 并发入队数量
	concurrency := 100
	recordsPerGoroutine := 100
	
	var wg sync.WaitGroup
	wg.Add(concurrency)
	
	// 启动多个goroutine并发入队
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
				assert.True(t, success, "入队应该成功")
			}
		}(i)
	}
	
	// 等待所有goroutine完成
	wg.Wait()
	
	// 验证统计信息
	stats := queue.GetStats()
	expectedEnqueueCount := int64(concurrency * recordsPerGoroutine)
	assert.Equal(t, expectedEnqueueCount, stats["enqueue_count"], "入队计数应该正确")
	assert.GreaterOrEqual(t, stats["size"], int64(0), "队列大小应该非负")
}

// TestLockFreeQueueConcurrentDequeue 测试并发出队
func TestLockFreeQueueConcurrentDequeue(t *testing.T) {
	queue := NewLockFreeQueue()
	
	// 先入队一些记录
	totalRecords := 1000
	for i := 0; i < totalRecords; i++ {
		record := &TraceRecord{
			RecordType:  "host_function_call",
			ExecutionID: "execution-1",
			HostFunctionCall: &HostFunctionCall{
				FunctionName: "test_function",
				Duration:     time.Duration(i) * time.Millisecond,
			},
		}
		queue.Enqueue(record)
	}
	
	// 并发出队数量
	concurrency := 10
	
	var wg sync.WaitGroup
	wg.Add(concurrency)
	
	dequeuedCount := int64(0)
	var dequeuedMutex sync.Mutex
	
	// 启动多个goroutine并发出队
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			for {
				record := queue.Dequeue()
				if record == nil {
					break
				}
				dequeuedMutex.Lock()
				dequeuedCount++
				dequeuedMutex.Unlock()
			}
		}()
	}
	
	// 等待所有goroutine完成
	wg.Wait()
	
	// 验证出队数量
	assert.Equal(t, int64(totalRecords), dequeuedCount, "所有记录都应该被出队")
	
	// 验证统计信息
	stats := queue.GetStats()
	assert.Equal(t, int64(totalRecords), stats["dequeue_count"], "出队计数应该正确")
	assert.Equal(t, int64(0), stats["size"], "队列应该为空")
}

// TestLockFreeQueueConcurrentEnqueueDequeue 测试并发入队和出队
func TestLockFreeQueueConcurrentEnqueueDequeue(t *testing.T) {
	queue := NewLockFreeQueue()
	
	// 并发入队和出队数量
	concurrency := 50
	recordsPerGoroutine := 100
	
	var wg sync.WaitGroup
	wg.Add(concurrency * 2) // 入队和出队各concurrency个goroutine
	
	// 启动入队goroutine
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
				queue.Enqueue(record)
			}
		}(i)
	}
	
	// 启动出队goroutine
	dequeuedCount := int64(0)
	var dequeuedMutex sync.Mutex
	
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			for {
				record := queue.Dequeue()
				if record == nil {
					// 等待一段时间，确保所有入队操作完成
					time.Sleep(10 * time.Millisecond)
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
	
	// 验证出队数量（应该等于入队数量）
	expectedDequeuedCount := int64(concurrency * recordsPerGoroutine)
	assert.Equal(t, expectedDequeuedCount, dequeuedCount, "所有记录都应该被出队")
	
	// 验证统计信息
	stats := queue.GetStats()
	assert.Equal(t, expectedDequeuedCount, stats["enqueue_count"], "入队计数应该正确")
	assert.Equal(t, expectedDequeuedCount, stats["dequeue_count"], "出队计数应该正确")
}

// TestLockFreeQueueConcurrentBatchDequeue 测试并发批量出队
func TestLockFreeQueueConcurrentBatchDequeue(t *testing.T) {
	queue := NewLockFreeQueue()
	
	// 先入队一些记录
	totalRecords := 1000
	for i := 0; i < totalRecords; i++ {
		record := &TraceRecord{
			RecordType:  "host_function_call",
			ExecutionID: "execution-1",
			HostFunctionCall: &HostFunctionCall{
				FunctionName: "test_function",
				Duration:     time.Duration(i) * time.Millisecond,
			},
		}
		queue.Enqueue(record)
	}
	
	// 并发批量出队数量
	concurrency := 5
	batchSize := 50
	
	var wg sync.WaitGroup
	wg.Add(concurrency)
	
	dequeuedCount := int64(0)
	var dequeuedMutex sync.Mutex
	
	// 启动多个goroutine并发批量出队
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			for {
				batch := queue.DequeueBatch(batchSize)
				if len(batch) == 0 {
					break
				}
				dequeuedMutex.Lock()
				dequeuedCount += int64(len(batch))
				dequeuedMutex.Unlock()
			}
		}()
	}
	
	// 等待所有goroutine完成
	wg.Wait()
	
	// 验证出队数量
	assert.Equal(t, int64(totalRecords), dequeuedCount, "所有记录都应该被出队")
	
	// 验证统计信息
	stats := queue.GetStats()
	assert.Equal(t, int64(totalRecords), stats["dequeue_count"], "出队计数应该正确")
	assert.Equal(t, int64(0), stats["size"], "队列应该为空")
}

// TestLockFreeQueueABAProblem 测试ABA问题防护
func TestLockFreeQueueABAProblem(t *testing.T) {
	queue := NewLockFreeQueue()
	
	// 创建记录
	record1 := &TraceRecord{
		RecordType:  "host_function_call",
		ExecutionID: "execution-1",
		HostFunctionCall: &HostFunctionCall{
			FunctionName: "test_function_1",
		},
	}
	
	record2 := &TraceRecord{
		RecordType:  "host_function_call",
		ExecutionID: "execution-2",
		HostFunctionCall: &HostFunctionCall{
			FunctionName: "test_function_2",
		},
	}
	
	// 入队记录1
	queue.Enqueue(record1)
	
	// 出队记录1
	dequeued1 := queue.Dequeue()
	require.NotNil(t, dequeued1, "应该能出队记录1")
	assert.Equal(t, "test_function_1", dequeued1.HostFunctionCall.FunctionName)
	
	// 再次入队记录1（模拟ABA问题场景）
	queue.Enqueue(record1)
	
	// 入队记录2
	queue.Enqueue(record2)
	
	// 出队应该得到记录1（不是记录2）
	dequeued2 := queue.Dequeue()
	require.NotNil(t, dequeued2, "应该能出队记录")
	assert.Equal(t, "test_function_1", dequeued2.HostFunctionCall.FunctionName)
	
	// 再次出队应该得到记录2
	dequeued3 := queue.Dequeue()
	require.NotNil(t, dequeued3, "应该能出队记录2")
	assert.Equal(t, "test_function_2", dequeued3.HostFunctionCall.FunctionName)
}

// TestLockFreeQueueEmptyQueue 测试空队列操作
func TestLockFreeQueueEmptyQueue(t *testing.T) {
	queue := NewLockFreeQueue()
	
	// 空队列出队应该返回nil
	record := queue.Dequeue()
	assert.Nil(t, record, "空队列出队应该返回nil")
	
	// 空队列批量出队应该返回空切片
	batch := queue.DequeueBatch(10)
	assert.Empty(t, batch, "空队列批量出队应该返回空切片")
	
	// 验证统计信息
	stats := queue.GetStats()
	assert.Equal(t, int64(0), stats["enqueue_count"], "入队计数应该为0")
	assert.Equal(t, int64(0), stats["dequeue_count"], "出队计数应该为0")
	assert.Equal(t, int64(0), stats["size"], "队列大小应该为0")
}

// TestLockFreeQueueFlush 测试队列刷新
func TestLockFreeQueueFlush(t *testing.T) {
	queue := NewLockFreeQueue()
	
	// 入队一些记录
	totalRecords := 100
	for i := 0; i < totalRecords; i++ {
		record := &TraceRecord{
			RecordType:  "host_function_call",
			ExecutionID: "execution-1",
			HostFunctionCall: &HostFunctionCall{
				FunctionName: "test_function",
				Duration:     time.Duration(i) * time.Millisecond,
			},
		}
		queue.Enqueue(record)
	}
	
	// 刷新队列
	flushed := queue.Flush()
	assert.Equal(t, totalRecords, len(flushed), "刷新应该返回所有记录")
	
	// 验证队列为空
	stats := queue.GetStats()
	assert.Equal(t, int64(0), stats["size"], "队列应该为空")
	
	// 再次刷新应该返回空切片
	flushed2 := queue.Flush()
	assert.Empty(t, flushed2, "再次刷新应该返回空切片")
}

// TestLockFreeQueueConcurrentFlush 测试并发刷新
func TestLockFreeQueueConcurrentFlush(t *testing.T) {
	queue := NewLockFreeQueue()
	
	// 入队一些记录
	totalRecords := 1000
	for i := 0; i < totalRecords; i++ {
		record := &TraceRecord{
			RecordType:  "host_function_call",
			ExecutionID: "execution-1",
			HostFunctionCall: &HostFunctionCall{
				FunctionName: "test_function",
				Duration:     time.Duration(i) * time.Millisecond,
			},
		}
		queue.Enqueue(record)
	}
	
	// 并发刷新数量
	concurrency := 10
	
	var wg sync.WaitGroup
	wg.Add(concurrency)
	
	flushedCount := int64(0)
	var flushedMutex sync.Mutex
	
	// 启动多个goroutine并发刷新
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			flushed := queue.Flush()
			flushedMutex.Lock()
			flushedCount += int64(len(flushed))
			flushedMutex.Unlock()
		}()
	}
	
	// 等待所有goroutine完成
	wg.Wait()
	
	// 验证刷新数量（应该等于总记录数，因为只有一个goroutine能真正刷新）
	assert.LessOrEqual(t, flushedCount, int64(totalRecords), "刷新数量不应该超过总记录数")
	
	// 验证队列为空
	stats := queue.GetStats()
	assert.Equal(t, int64(0), stats["size"], "队列应该为空")
}

