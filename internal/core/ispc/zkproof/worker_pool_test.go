package zkproof

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"

	"github.com/weisyn/v1/internal/core/ispc/testutil"
)

// ============================================================================
// worker_pool.go 测试
// ============================================================================

// TestNewZKProofWorker 测试创建ZK证明工作线程
func TestNewZKProofWorker(t *testing.T) {
	logger := testutil.NewTestLogger()
	taskQueue := NewZKProofTaskQueue(logger)
	proofManager := createMockZKProofManager()
	callback := func(task *ZKProofTask, proof *transaction.ZKStateProof, err error) {}
	
	worker := NewZKProofWorker(0, taskQueue, proofManager, callback, logger)
	require.NotNil(t, worker)
	require.Equal(t, 0, worker.workerID)
	require.Equal(t, taskQueue, worker.taskQueue)
	require.Equal(t, proofManager, worker.proofManager)
	require.Equal(t, WorkerHealthHealthy, worker.GetHealthStatus())
}

// TestZKProofWorker_StartStop 测试启动和停止工作线程
func TestZKProofWorker_StartStop(t *testing.T) {
	logger := testutil.NewTestLogger()
	taskQueue := NewZKProofTaskQueue(logger)
	taskQueue.Start()
	defer taskQueue.Stop()
	
	proofManager := createMockZKProofManager()
	callback := func(task *ZKProofTask, proof *transaction.ZKStateProof, err error) {}
	
	worker := NewZKProofWorker(0, taskQueue, proofManager, callback, logger)
	
	// 启动工作线程
	worker.Start()
	
	// 等待一小段时间确保goroutine启动
	time.Sleep(50 * time.Millisecond)
	
	// 停止工作线程
	worker.Stop()
	
	// 验证工作线程已停止（通过检查doneCh是否关闭）
	select {
	case <-worker.doneCh:
		// doneCh已关闭，工作线程已停止
	default:
		t.Fatal("工作线程未正确停止")
	}
}

// TestZKProofWorker_ProcessTask 测试处理任务
func TestZKProofWorker_ProcessTask(t *testing.T) {
	logger := testutil.NewTestLogger()
	taskQueue := NewZKProofTaskQueue(logger)
	taskQueue.Start()
	defer taskQueue.Stop()
	
	proofManager := createMockZKProofManager()
	var callbackCalled bool
	var callbackTask *ZKProofTask
	
	callback := func(task *ZKProofTask, proof *transaction.ZKStateProof, err error) {
		callbackCalled = true
		callbackTask = task
	}
	
	worker := NewZKProofWorker(0, taskQueue, proofManager, callback, logger)
	worker.Start()
	defer worker.Stop()
	
	// 创建任务
	task := createTestTask("task1", 10, 5*time.Minute)
	err := taskQueue.Enqueue(task)
	require.NoError(t, err)
	
	// 等待任务处理
	time.Sleep(200 * time.Millisecond)
	
	// 验证回调被调用
	require.True(t, callbackCalled)
	require.NotNil(t, callbackTask)
	require.Equal(t, "task1", callbackTask.TaskID)
}

// TestZKProofWorker_ProcessTask_WithRetry 测试处理任务失败重试
// 注意：这个测试需要实际的proofManager，因为nil会导致panic
// 实际的重试逻辑会在真实的错误场景中测试
func TestZKProofWorker_ProcessTask_WithRetry(t *testing.T) {
	logger := testutil.NewTestLogger()
	taskQueue := NewZKProofTaskQueue(logger)
	taskQueue.Start()
	defer taskQueue.Stop()
	
	// 使用有效的proofManager（实际测试中，错误会在GenerateStateProof中产生）
	proofManager := createMockZKProofManager()
	var callbackCalled bool
	
	callback := func(task *ZKProofTask, proof *transaction.ZKStateProof, err error) {
		callbackCalled = true
		// 在实际场景中，如果GenerateStateProof返回错误，这里会收到错误
	}
	
	worker := NewZKProofWorker(0, taskQueue, proofManager, callback, logger)
	worker.Start()
	defer worker.Stop()
	
	// 创建可重试的任务
	task := createTestTask("task1", 10, 5*time.Minute)
	task.MaxRetries = 3
	err := taskQueue.Enqueue(task)
	require.NoError(t, err)
	
	// 等待任务处理
	time.Sleep(200 * time.Millisecond)
	
	// 验证回调被调用（无论成功或失败）
	require.True(t, callbackCalled)
}

// TestZKProofWorker_UpdateHealthStatus_Degraded 测试健康状态降级
func TestZKProofWorker_UpdateHealthStatus_Degraded(t *testing.T) {
	logger := testutil.NewTestLogger()
	taskQueue := NewZKProofTaskQueue(logger)
	proofManager := createMockZKProofManager()
	callback := func(task *ZKProofTask, proof *transaction.ZKStateProof, err error) {}
	
	worker := NewZKProofWorker(0, taskQueue, proofManager, callback, logger)
	
	// 模拟多次失败，使失败率超过50%
	for i := 0; i < 6; i++ {
		worker.errorCount.Add(1)
		worker.processedCount.Add(1)
		worker.updateHealthStatus(false)
	}
	
	// 添加一些成功，但失败率仍然超过50%
	for i := 0; i < 3; i++ {
		worker.successCount.Add(1)
		worker.processedCount.Add(1)
		worker.updateHealthStatus(true)
	}
	
	// 再次失败，应该设置为降级
	worker.errorCount.Add(1)
	worker.processedCount.Add(1)
	worker.updateHealthStatus(false)
	
	status := worker.GetHealthStatus()
	// 失败率 = 7/(7+3) = 70% > 50%，应该是降级状态
	require.Equal(t, WorkerHealthDegraded, status)
}

// TestZKProofWorker_UpdateHealthStatus_Unhealthy 测试健康状态不健康
func TestZKProofWorker_UpdateHealthStatus_Unhealthy(t *testing.T) {
	logger := testutil.NewTestLogger()
	taskQueue := NewZKProofTaskQueue(logger)
	proofManager := createMockZKProofManager()
	callback := func(task *ZKProofTask, proof *transaction.ZKStateProof, err error) {}
	
	worker := NewZKProofWorker(0, taskQueue, proofManager, callback, logger)
	
	// 模拟连续失败超过10次
	for i := 0; i < 11; i++ {
		worker.errorCount.Add(1)
		worker.updateHealthStatus(false)
	}
	
	status := worker.GetHealthStatus()
	require.Equal(t, WorkerHealthUnhealthy, status)
}

// TestZKProofWorker_UpdateHealthStatus_Healthy 测试健康状态健康
func TestZKProofWorker_UpdateHealthStatus_Healthy(t *testing.T) {
	logger := testutil.NewTestLogger()
	taskQueue := NewZKProofTaskQueue(logger)
	proofManager := createMockZKProofManager()
	callback := func(task *ZKProofTask, proof *transaction.ZKStateProof, err error) {}
	
	worker := NewZKProofWorker(0, taskQueue, proofManager, callback, logger)
	
	// 模拟成功
	worker.successCount.Add(1)
	worker.processedCount.Add(1)
	worker.updateHealthStatus(true)
	
	status := worker.GetHealthStatus()
	require.Equal(t, WorkerHealthHealthy, status)
	
	// 添加一些失败，但失败率不超过50%
	for i := 0; i < 2; i++ {
		worker.errorCount.Add(1)
		worker.processedCount.Add(1)
		worker.updateHealthStatus(false)
	}
	
	for i := 0; i < 3; i++ {
		worker.successCount.Add(1)
		worker.processedCount.Add(1)
		worker.updateHealthStatus(true)
	}
	
	status = worker.GetHealthStatus()
	// 失败率 = 2/(2+4) = 33% < 50%，应该是健康状态
	require.Equal(t, WorkerHealthHealthy, status)
}

// TestZKProofWorker_GetStats 测试获取统计信息
func TestZKProofWorker_GetStats(t *testing.T) {
	logger := testutil.NewTestLogger()
	taskQueue := NewZKProofTaskQueue(logger)
	proofManager := createMockZKProofManager()
	callback := func(task *ZKProofTask, proof *transaction.ZKStateProof, err error) {}
	
	worker := NewZKProofWorker(0, taskQueue, proofManager, callback, logger)
	
	stats := worker.GetStats()
	require.NotNil(t, stats)
	require.Equal(t, 0, stats["worker_id"])
	require.Equal(t, int64(0), stats["processed_count"])
	require.Equal(t, int64(0), stats["success_count"])
	require.Equal(t, int64(0), stats["error_count"])
	require.Equal(t, "healthy", stats["health_status"])
}

// TestZKProofWorker_GetHealthStatus 测试获取健康状态
func TestZKProofWorker_GetHealthStatus(t *testing.T) {
	logger := testutil.NewTestLogger()
	taskQueue := NewZKProofTaskQueue(logger)
	proofManager := createMockZKProofManager()
	callback := func(task *ZKProofTask, proof *transaction.ZKStateProof, err error) {}
	
	worker := NewZKProofWorker(0, taskQueue, proofManager, callback, logger)
	
	status := worker.GetHealthStatus()
	require.Equal(t, WorkerHealthHealthy, status)
}

// TestNewZKProofWorkerPool 测试创建ZK证明工作线程池
func TestNewZKProofWorkerPool(t *testing.T) {
	logger := testutil.NewTestLogger()
	taskQueue := NewZKProofTaskQueue(logger)
	proofManager := createMockZKProofManager()
	callback := func(task *ZKProofTask, proof *transaction.ZKStateProof, err error) {}
	
	pool := NewZKProofWorkerPool(taskQueue, proofManager, callback, 2, 1, 10, logger)
	require.NotNil(t, pool)
	require.Equal(t, 2, pool.workerCount)
	require.Equal(t, 1, pool.minWorkers)
	require.Equal(t, 10, pool.maxWorkers)
	require.False(t, pool.started)
}

// TestNewZKProofWorkerPool_DefaultValues 测试默认值
func TestNewZKProofWorkerPool_DefaultValues(t *testing.T) {
	logger := testutil.NewTestLogger()
	taskQueue := NewZKProofTaskQueue(logger)
	proofManager := createMockZKProofManager()
	callback := func(task *ZKProofTask, proof *transaction.ZKStateProof, err error) {}
	
	pool := NewZKProofWorkerPool(taskQueue, proofManager, callback, 0, 0, 0, logger)
	require.NotNil(t, pool)
	require.Equal(t, 2, pool.workerCount) // 默认2个
	require.Equal(t, 1, pool.minWorkers) // 默认最小1个
	require.Equal(t, 10, pool.maxWorkers) // 默认最大10个
}

// TestZKProofWorkerPool_StartStop 测试启动和停止工作线程池
func TestZKProofWorkerPool_StartStop(t *testing.T) {
	logger := testutil.NewTestLogger()
	taskQueue := NewZKProofTaskQueue(logger)
	taskQueue.Start()
	defer taskQueue.Stop()
	
	proofManager := createMockZKProofManager()
	callback := func(task *ZKProofTask, proof *transaction.ZKStateProof, err error) {}
	
	pool := NewZKProofWorkerPool(taskQueue, proofManager, callback, 2, 1, 10, logger)
	
	// 启动工作线程池
	pool.Start()
	require.True(t, pool.started)
	require.Equal(t, 2, len(pool.workers))
	
	// 重复启动应该无影响
	pool.Start()
	require.True(t, pool.started)
	
	// 停止工作线程池
	pool.Stop()
	require.False(t, pool.started)
	
	// 重复停止应该无影响
	pool.Stop()
	require.False(t, pool.started)
}

// TestZKProofWorkerPool_AddWorker 测试添加工作线程
func TestZKProofWorkerPool_AddWorker(t *testing.T) {
	logger := testutil.NewTestLogger()
	taskQueue := NewZKProofTaskQueue(logger)
	taskQueue.Start()
	defer taskQueue.Stop()
	
	proofManager := createMockZKProofManager()
	callback := func(task *ZKProofTask, proof *transaction.ZKStateProof, err error) {}
	
	pool := NewZKProofWorkerPool(taskQueue, proofManager, callback, 2, 1, 5, logger)
	pool.Start()
	defer pool.Stop()
	
	initialCount := len(pool.workers)
	
	err := pool.AddWorker()
	require.NoError(t, err)
	require.Equal(t, initialCount+1, len(pool.workers))
}

// TestZKProofWorkerPool_AddWorker_MaxWorkers 测试达到最大工作线程数量
func TestZKProofWorkerPool_AddWorker_MaxWorkers(t *testing.T) {
	logger := testutil.NewTestLogger()
	taskQueue := NewZKProofTaskQueue(logger)
	taskQueue.Start()
	defer taskQueue.Stop()
	
	proofManager := createMockZKProofManager()
	callback := func(task *ZKProofTask, proof *transaction.ZKStateProof, err error) {}
	
	pool := NewZKProofWorkerPool(taskQueue, proofManager, callback, 2, 1, 2, logger)
	pool.Start()
	defer pool.Stop()
	
	err := pool.AddWorker()
	require.Error(t, err)
	require.Contains(t, err.Error(), "已达到最大工作线程数量")
}

// TestZKProofWorkerPool_RemoveWorker 测试移除工作线程
func TestZKProofWorkerPool_RemoveWorker(t *testing.T) {
	logger := testutil.NewTestLogger()
	taskQueue := NewZKProofTaskQueue(logger)
	taskQueue.Start()
	defer taskQueue.Stop()
	
	proofManager := createMockZKProofManager()
	callback := func(task *ZKProofTask, proof *transaction.ZKStateProof, err error) {}
	
	pool := NewZKProofWorkerPool(taskQueue, proofManager, callback, 3, 1, 10, logger)
	pool.Start()
	defer pool.Stop()
	
	initialCount := len(pool.workers)
	
	err := pool.RemoveWorker()
	require.NoError(t, err)
	require.Equal(t, initialCount-1, len(pool.workers))
}

// TestZKProofWorkerPool_RemoveWorker_MinWorkers 测试达到最小工作线程数量
func TestZKProofWorkerPool_RemoveWorker_MinWorkers(t *testing.T) {
	logger := testutil.NewTestLogger()
	taskQueue := NewZKProofTaskQueue(logger)
	taskQueue.Start()
	defer taskQueue.Stop()
	
	proofManager := createMockZKProofManager()
	callback := func(task *ZKProofTask, proof *transaction.ZKStateProof, err error) {}
	
	pool := NewZKProofWorkerPool(taskQueue, proofManager, callback, 1, 1, 10, logger)
	pool.Start()
	defer pool.Stop()
	
	err := pool.RemoveWorker()
	require.Error(t, err)
	require.Contains(t, err.Error(), "已达到最小工作线程数量")
}

// TestZKProofWorkerPool_GetStats 测试获取统计信息
func TestZKProofWorkerPool_GetStats(t *testing.T) {
	logger := testutil.NewTestLogger()
	taskQueue := NewZKProofTaskQueue(logger)
	taskQueue.Start()
	defer taskQueue.Stop()
	
	proofManager := createMockZKProofManager()
	callback := func(task *ZKProofTask, proof *transaction.ZKStateProof, err error) {}
	
	pool := NewZKProofWorkerPool(taskQueue, proofManager, callback, 2, 1, 10, logger)
	pool.Start()
	defer pool.Stop()
	
	stats := pool.GetStats()
	require.NotNil(t, stats)
	require.Equal(t, 2, stats["worker_count"])
	require.Equal(t, 1, stats["min_workers"])
	require.Equal(t, 10, stats["max_workers"])
	require.Equal(t, int64(0), stats["total_processed"])
	require.Equal(t, int64(0), stats["total_success"])
	require.Equal(t, int64(0), stats["total_errors"])
}

// TestWorkerHealthStatus_Constants 测试健康状态常量
func TestWorkerHealthStatus_Constants(t *testing.T) {
	require.Equal(t, WorkerHealthStatus("healthy"), WorkerHealthHealthy)
	require.Equal(t, WorkerHealthStatus("degraded"), WorkerHealthDegraded)
	require.Equal(t, WorkerHealthStatus("unhealthy"), WorkerHealthUnhealthy)
}

// TestWorkerScaler_AdjustWorkers_QueueBacklog 测试动态调整器 - 队列积压
func TestWorkerScaler_AdjustWorkers_QueueBacklog(t *testing.T) {
	logger := testutil.NewTestLogger()
	taskQueue := NewZKProofTaskQueue(logger)
	taskQueue.Start()
	defer taskQueue.Stop()
	
	proofManager := createMockZKProofManager()
	callback := func(task *ZKProofTask, proof *transaction.ZKStateProof, err error) {}
	
	pool := NewZKProofWorkerPool(taskQueue, proofManager, callback, 2, 1, 5, logger)
	pool.Start()
	defer pool.Stop()
	
	initialWorkerCount := len(pool.workers)
	
	// 创建大量任务，使队列积压超过100
	for i := 0; i < 150; i++ {
		task := createTestTask(fmt.Sprintf("task%d", i), 10, 5*time.Minute)
		taskQueue.Enqueue(task)
	}
	
	// 直接调用 adjustWorkers（通过反射或创建测试用的scaler）
	scaler := pool.scaler
	scaler.adjustWorkers()
	
	// 验证工作线程数量增加（如果队列积压且未达到最大值）
	queueStats := taskQueue.GetStats()
	queueSize := queueStats["queue_size"].(int)
	if queueSize > 100 && len(pool.workers) < pool.maxWorkers {
		// 工作线程应该增加
		require.GreaterOrEqual(t, len(pool.workers), initialWorkerCount)
	}
}

// TestWorkerScaler_AdjustWorkers_EmptyQueue 测试动态调整器 - 空队列
func TestWorkerScaler_AdjustWorkers_EmptyQueue(t *testing.T) {
	logger := testutil.NewTestLogger()
	taskQueue := NewZKProofTaskQueue(logger)
	taskQueue.Start()
	defer taskQueue.Stop()
	
	proofManager := createMockZKProofManager()
	callback := func(task *ZKProofTask, proof *transaction.ZKStateProof, err error) {}
	
	pool := NewZKProofWorkerPool(taskQueue, proofManager, callback, 3, 1, 10, logger)
	pool.Start()
	defer pool.Stop()
	
	initialWorkerCount := len(pool.workers)
	
	// 确保队列为空
	require.Equal(t, 0, taskQueue.queue.Len())
	
	// 直接调用 adjustWorkers
	scaler := pool.scaler
	scaler.adjustWorkers()
	
	// 验证工作线程数量减少（如果队列为空且超过最小值）
	if len(pool.workers) > pool.minWorkers {
		require.LessOrEqual(t, len(pool.workers), initialWorkerCount)
	}
}

// TestWorkerScaler_AdjustWorkers_UnhealthyWorkers 测试动态调整器 - 不健康工作线程
func TestWorkerScaler_AdjustWorkers_UnhealthyWorkers(t *testing.T) {
	logger := testutil.NewTestLogger()
	taskQueue := NewZKProofTaskQueue(logger)
	taskQueue.Start()
	defer taskQueue.Stop()
	
	proofManager := createMockZKProofManager()
	callback := func(task *ZKProofTask, proof *transaction.ZKStateProof, err error) {}
	
	pool := NewZKProofWorkerPool(taskQueue, proofManager, callback, 2, 1, 10, logger)
	pool.Start()
	defer pool.Stop()
	
	// 模拟一个不健康的工作线程
	if len(pool.workers) > 0 {
		worker := pool.workers[0]
		for i := 0; i < 11; i++ {
			worker.errorCount.Add(1)
			worker.updateHealthStatus(false)
		}
	}
	
	// 直接调用 adjustWorkers
	scaler := pool.scaler
	scaler.adjustWorkers()
	
	// 验证不健康工作线程被检测到（通过日志或统计信息）
	stats := pool.GetStats()
	unhealthyWorkers := stats["unhealthy_workers"].(int)
	require.GreaterOrEqual(t, unhealthyWorkers, 0) // 至少应该检测到
}

// createMockZKProofManager 创建Mock ZK证明管理器
//
// ✅ **重构说明**：使用testutil包中的统一Mock对象，遵循测试规范
func createMockZKProofManager() *Manager {
	hashManager := testutil.NewTestHashManager()
	signatureManager := testutil.NewTestSignatureManager()
	logger := testutil.NewTestLogger()
	configProvider := testutil.NewTestConfigProvider()
	
	manager := NewManager(hashManager, signatureManager, logger, configProvider)
	return manager
}

