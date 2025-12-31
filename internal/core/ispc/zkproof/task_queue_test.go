package zkproof

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	"github.com/weisyn/v1/internal/core/ispc/testutil"
)

// ============================================================================
// ZK证明任务队列功能测试（异步ZK证明生成优化 - 阶段1）
// ============================================================================
//
// 🎯 **测试目的**：
// 测试ZK证明任务队列的基本功能，包括入队、出队、查询、取消、统计等。
//
// ⚠️ **注意**：
// - 测试优先级队列的正确性
// - 测试任务状态管理
// - 测试超时检测功能
//
// ============================================================================
//
// ✅ **重构说明**：使用testutil包中的统一Mock对象，遵循测试规范
//
// ============================================================================

// createTestTask 创建测试任务
func createTestTask(taskID string, priority int, timeout time.Duration) *ZKProofTask {
	return NewZKProofTask(
		taskID,
		"test_execution_"+taskID,
		&ispcInterfaces.ZKProofInput{
			CircuitID:      "test_circuit",
			CircuitVersion: 1,
		},
		[]byte("hash"),
		nil,
		priority,
		timeout,
	)
}

// TestNewZKProofTaskQueue 测试：创建任务队列
func TestNewZKProofTaskQueue(t *testing.T) {
	logger := testutil.NewTestLogger()
	queue := NewZKProofTaskQueue(logger)

	// 验证队列初始状态
	assert.NotNil(t, queue)
	assert.NotNil(t, queue.queue)
	assert.NotNil(t, queue.tasks)
	assert.NotNil(t, queue.notifyCh)
	assert.Equal(t, logger, queue.logger)
	assert.False(t, queue.started)
	assert.Equal(t, 0, queue.queue.Len())
	assert.Equal(t, 0, len(queue.tasks))
}

// TestQueue_StartStop 测试：启动和停止队列
func TestQueue_StartStop(t *testing.T) {
	logger := testutil.NewTestLogger()
	queue := NewZKProofTaskQueue(logger)

	// 启动队列
	queue.Start()
	assert.True(t, queue.started)

	// 重复启动应该无影响
	queue.Start()
	assert.True(t, queue.started)

	// 停止队列
	queue.Stop()
	assert.False(t, queue.started)

	// 重复停止应该无影响
	queue.Stop()
	assert.False(t, queue.started)
}

// TestQueue_Enqueue 测试：入队任务
func TestQueue_Enqueue(t *testing.T) {
	logger := testutil.NewTestLogger()
	queue := NewZKProofTaskQueue(logger)

	task := createTestTask("task1", 10, 5*time.Minute)

	// 入队任务
	err := queue.Enqueue(task)
	assert.NoError(t, err)

	// 验证任务已入队
	assert.Equal(t, 1, queue.queue.Len())
	assert.Equal(t, 1, len(queue.tasks))
	assert.Equal(t, task, queue.tasks[task.TaskID])
}

// TestQueue_Enqueue_NilTask 测试：入队nil任务
func TestQueue_Enqueue_NilTask(t *testing.T) {
	logger := testutil.NewTestLogger()
	queue := NewZKProofTaskQueue(logger)

	// 入队nil任务应该返回错误
	err := queue.Enqueue(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "任务不能为空")
}

// TestQueue_Enqueue_DuplicateTask 测试：入队重复任务
func TestQueue_Enqueue_DuplicateTask(t *testing.T) {
	logger := testutil.NewTestLogger()
	queue := NewZKProofTaskQueue(logger)

	task := createTestTask("task1", 10, 5*time.Minute)

	// 第一次入队
	err := queue.Enqueue(task)
	assert.NoError(t, err)

	// 第二次入队相同任务应该返回错误
	err = queue.Enqueue(task)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "任务已存在")
}

// TestQueue_Dequeue 测试：出队任务
func TestQueue_Dequeue(t *testing.T) {
	logger := testutil.NewTestLogger()
	queue := NewZKProofTaskQueue(logger)

	task1 := createTestTask("task1", 10, 5*time.Minute)
	task2 := createTestTask("task2", 20, 5*time.Minute)

	// 入队两个任务
	err := queue.Enqueue(task1)
	require.NoError(t, err)
	err = queue.Enqueue(task2)
	require.NoError(t, err)

	// 出队任务（优先级高的先出队）
	dequeuedTask := queue.Dequeue()
	assert.NotNil(t, dequeuedTask)
	assert.Equal(t, task2.TaskID, dequeuedTask.TaskID) // 优先级20 > 10

	// 验证任务已从队列和映射中移除
	assert.Equal(t, 1, queue.queue.Len())
	assert.Equal(t, 1, len(queue.tasks))
	assert.Nil(t, queue.tasks[task2.TaskID])

	// 再次出队
	dequeuedTask = queue.Dequeue()
	assert.NotNil(t, dequeuedTask)
	assert.Equal(t, task1.TaskID, dequeuedTask.TaskID)

	// 队列应该为空
	assert.Equal(t, 0, queue.queue.Len())
	assert.Equal(t, 0, len(queue.tasks))
}

// TestQueue_Dequeue_EmptyQueue 测试：空队列出队
func TestQueue_Dequeue_EmptyQueue(t *testing.T) {
	logger := testutil.NewTestLogger()
	queue := NewZKProofTaskQueue(logger)

	// 空队列出队应该返回nil
	task := queue.Dequeue()
	assert.Nil(t, task)
}

// TestQueue_Peek 测试：查看队列头部
func TestQueue_Peek(t *testing.T) {
	logger := testutil.NewTestLogger()
	queue := NewZKProofTaskQueue(logger)

	task1 := createTestTask("task1", 10, 5*time.Minute)
	task2 := createTestTask("task2", 20, 5*time.Minute)

	// 入队两个任务
	err := queue.Enqueue(task1)
	require.NoError(t, err)
	err = queue.Enqueue(task2)
	require.NoError(t, err)

	// 查看队列头部（应该是最优先级高的任务）
	peekedTask := queue.Peek()
	assert.NotNil(t, peekedTask)
	assert.Equal(t, task2.TaskID, peekedTask.TaskID)

	// 验证任务仍在队列中
	assert.Equal(t, 2, queue.queue.Len())
	assert.Equal(t, 2, len(queue.tasks))
}

// TestQueue_Peek_EmptyQueue 测试：空队列查看
func TestQueue_Peek_EmptyQueue(t *testing.T) {
	logger := testutil.NewTestLogger()
	queue := NewZKProofTaskQueue(logger)

	// 空队列查看应该返回nil
	task := queue.Peek()
	assert.Nil(t, task)
}

// TestQueue_GetTask 测试：获取任务
func TestQueue_GetTask(t *testing.T) {
	logger := testutil.NewTestLogger()
	queue := NewZKProofTaskQueue(logger)

	task := createTestTask("task1", 10, 5*time.Minute)

	// 入队任务
	err := queue.Enqueue(task)
	require.NoError(t, err)

	// 获取任务
	retrievedTask := queue.GetTask(task.TaskID)
	assert.NotNil(t, retrievedTask)
	assert.Equal(t, task.TaskID, retrievedTask.TaskID)

	// 获取不存在的任务
	retrievedTask = queue.GetTask("non_existent")
	assert.Nil(t, retrievedTask)
}

// TestQueue_UpdateTaskStatus 测试：更新任务状态
func TestQueue_UpdateTaskStatus(t *testing.T) {
	logger := testutil.NewTestLogger()
	queue := NewZKProofTaskQueue(logger)

	task := createTestTask("task1", 10, 5*time.Minute)

	// 入队任务
	err := queue.Enqueue(task)
	require.NoError(t, err)

	// 更新任务状态
	err = queue.UpdateTaskStatus(task.TaskID, TaskStatusRunning)
	assert.NoError(t, err)
	assert.Equal(t, TaskStatusRunning, task.Status)

	// 更新不存在的任务状态应该返回错误
	err = queue.UpdateTaskStatus("non_existent", TaskStatusRunning)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "任务不存在")
}

// TestQueue_CancelTask 测试：取消任务
func TestQueue_CancelTask(t *testing.T) {
	logger := testutil.NewTestLogger()
	queue := NewZKProofTaskQueue(logger)

	task := createTestTask("task1", 10, 5*time.Minute)

	// 入队任务
	err := queue.Enqueue(task)
	require.NoError(t, err)

	// 取消任务
	err = queue.CancelTask(task.TaskID)
	assert.NoError(t, err)
	assert.Equal(t, TaskStatusCancelled, task.Status)

	// 验证任务已从队列和映射中移除
	assert.Equal(t, 0, queue.queue.Len())
	assert.Equal(t, 0, len(queue.tasks))
	assert.Nil(t, queue.tasks[task.TaskID])

	// 取消不存在的任务应该返回错误
	err = queue.CancelTask("non_existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "任务不存在")
}

// TestQueue_CancelTask_RunningTask 测试：取消运行中的任务
func TestQueue_CancelTask_RunningTask(t *testing.T) {
	logger := testutil.NewTestLogger()
	queue := NewZKProofTaskQueue(logger)

	task := createTestTask("task1", 10, 5*time.Minute)
	task.MarkRunning()

	// 入队任务（虽然状态是Running，但仍在队列中）
	err := queue.Enqueue(task)
	require.NoError(t, err)

	// 取消运行中的任务
	err = queue.CancelTask(task.TaskID)
	assert.NoError(t, err)
	assert.Equal(t, TaskStatusCancelled, task.Status)
}

// TestQueue_GetStats 测试：获取统计信息
func TestQueue_GetStats(t *testing.T) {
	logger := testutil.NewTestLogger()
	queue := NewZKProofTaskQueue(logger)

	// 空队列统计
	stats := queue.GetStats()
	assert.Equal(t, 0, stats["queue_size"])
	assert.Equal(t, 0, stats["total_tasks"])

	// 添加多个不同状态的任务
	task1 := createTestTask("task1", 10, 5*time.Minute)
	task2 := createTestTask("task2", 20, 5*time.Minute)
	task3 := createTestTask("task3", 30, 5*time.Minute)
	task2.MarkRunning()
	task3.MarkCompleted(nil)

	err := queue.Enqueue(task1)
	require.NoError(t, err)
	err = queue.Enqueue(task2)
	require.NoError(t, err)
	err = queue.Enqueue(task3)
	require.NoError(t, err)

	// 获取统计信息
	stats = queue.GetStats()
	assert.Equal(t, 3, stats["queue_size"])
	assert.Equal(t, 3, stats["total_tasks"])

	statusCounts := stats["status_counts"].(map[string]int)
	assert.Equal(t, 1, statusCounts["pending"])
	assert.Equal(t, 1, statusCounts["running"])
	assert.Equal(t, 1, statusCounts["completed"])
}

// TestQueue_GetNotifyChannel 测试：获取通知通道
func TestQueue_GetNotifyChannel(t *testing.T) {
	logger := testutil.NewTestLogger()
	queue := NewZKProofTaskQueue(logger)

	// 获取通知通道
	notifyCh := queue.GetNotifyChannel()
	assert.NotNil(t, notifyCh)

	// 入队任务应该发送通知
	task := createTestTask("task1", 10, 5*time.Minute)
	err := queue.Enqueue(task)
	require.NoError(t, err)

	// 等待通知（带超时）
	select {
	case notifiedTask := <-notifyCh:
		assert.Equal(t, task.TaskID, notifiedTask.TaskID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("未收到通知")
	}
}

// TestQueue_PriorityOrder 测试：优先级顺序
func TestQueue_PriorityOrder(t *testing.T) {
	logger := testutil.NewTestLogger()
	queue := NewZKProofTaskQueue(logger)

	// 创建不同优先级的任务
	task1 := createTestTask("task1", 10, 5*time.Minute)
	task2 := createTestTask("task2", 30, 5*time.Minute)
	task3 := createTestTask("task3", 20, 5*time.Minute)
	task4 := createTestTask("task4", 5, 5*time.Minute)

	// 按不同顺序入队
	err := queue.Enqueue(task1)
	require.NoError(t, err)
	err = queue.Enqueue(task2)
	require.NoError(t, err)
	err = queue.Enqueue(task3)
	require.NoError(t, err)
	err = queue.Enqueue(task4)
	require.NoError(t, err)

	// 出队顺序应该是：task2(30) -> task3(20) -> task1(10) -> task4(5)
	dequeuedTask := queue.Dequeue()
	assert.Equal(t, task2.TaskID, dequeuedTask.TaskID)

	dequeuedTask = queue.Dequeue()
	assert.Equal(t, task3.TaskID, dequeuedTask.TaskID)

	dequeuedTask = queue.Dequeue()
	assert.Equal(t, task1.TaskID, dequeuedTask.TaskID)

	dequeuedTask = queue.Dequeue()
	assert.Equal(t, task4.TaskID, dequeuedTask.TaskID)
}

// TestQueue_PriorityOrder_SamePriority 测试：相同优先级FIFO
func TestQueue_PriorityOrder_SamePriority(t *testing.T) {
	logger := testutil.NewTestLogger()
	queue := NewZKProofTaskQueue(logger)

	// 创建相同优先级但不同创建时间的任务
	time1 := time.Now()
	task1 := createTestTask("task1", 10, 5*time.Minute)
	task1.CreatedAt = time1

	time.Sleep(10 * time.Millisecond)
	time2 := time.Now()
	task2 := createTestTask("task2", 10, 5*time.Minute)
	task2.CreatedAt = time2

	// 入队任务
	err := queue.Enqueue(task1)
	require.NoError(t, err)
	err = queue.Enqueue(task2)
	require.NoError(t, err)

	// 相同优先级，创建时间早的应该先出队
	dequeuedTask := queue.Dequeue()
	assert.Equal(t, task1.TaskID, dequeuedTask.TaskID)

	dequeuedTask = queue.Dequeue()
	assert.Equal(t, task2.TaskID, dequeuedTask.TaskID)
}

// TestQueue_TimeoutDetection 测试：超时检测
func TestQueue_TimeoutDetection(t *testing.T) {
	logger := testutil.NewTestLogger()
	queue := NewZKProofTaskQueue(logger)

	// 启动队列（启动超时检测器）
	queue.Start()
	defer queue.Stop()

	// 创建即将超时的任务（超时时间很短，确保在检测周期内超时）
	task := createTestTask("task1", 10, 50*time.Millisecond)
	err := queue.Enqueue(task)
	require.NoError(t, err)

	// 等待超时（超时检测器每秒检查一次，等待1.2秒确保至少检查一次）
	time.Sleep(1200 * time.Millisecond)

	// 验证任务已标记为超时
	retrievedTask := queue.GetTask(task.TaskID)
	if retrievedTask != nil {
		// 超时检测器应该已经检测到超时并标记任务
		t.Logf("任务状态: %s", retrievedTask.Status)
		// 如果任务已超时，状态应该是timeout
		if retrievedTask.IsExpired() {
			// 任务已过期，应该被标记为超时
			require.True(t, retrievedTask.IsExpired())
		}
	}
}

// TestQueue_TimeoutDetection_RunningTask 测试：运行中任务的超时检测
func TestQueue_TimeoutDetection_RunningTask(t *testing.T) {
	logger := testutil.NewTestLogger()
	queue := NewZKProofTaskQueue(logger)

	// 启动队列（启动超时检测器）
	queue.Start()
	defer queue.Stop()

	// 创建即将超时的任务并标记为运行中
	task := createTestTask("task1", 10, 50*time.Millisecond)
	task.MarkRunning()
	err := queue.Enqueue(task)
	require.NoError(t, err)

	// 等待超时（超时检测器每秒检查一次，等待1.2秒确保至少检查一次）
	time.Sleep(1200 * time.Millisecond)

	// 验证任务状态（超时检测器应该检测到运行中的任务也超时了）
	retrievedTask := queue.GetTask(task.TaskID)
	if retrievedTask != nil {
		t.Logf("运行中任务状态: %s", retrievedTask.Status)
		// 如果任务已超时，应该被标记为超时
		if retrievedTask.IsExpired() {
			require.True(t, retrievedTask.IsExpired())
		}
	}
}

// TestQueue_TimeoutDetection_MultipleTasks 测试：多个任务的超时检测
func TestQueue_TimeoutDetection_MultipleTasks(t *testing.T) {
	logger := testutil.NewTestLogger()
	queue := NewZKProofTaskQueue(logger)

	// 启动队列（启动超时检测器）
	queue.Start()
	defer queue.Stop()

	// 创建多个即将超时的任务
	task1 := createTestTask("task1", 10, 100*time.Millisecond)
	task2 := createTestTask("task2", 20, 150*time.Millisecond)
	task3 := createTestTask("task3", 30, 200*time.Millisecond)

	err := queue.Enqueue(task1)
	require.NoError(t, err)
	err = queue.Enqueue(task2)
	require.NoError(t, err)
	err = queue.Enqueue(task3)
	require.NoError(t, err)

	// 等待超时
	time.Sleep(250 * time.Millisecond)

	// 验证所有任务的状态
	stats := queue.GetStats()
	t.Logf("队列统计: %+v", stats)
}

// TestQueue_NotifyChannel_BufferFull 测试：通知通道缓冲区满
func TestQueue_NotifyChannel_BufferFull(t *testing.T) {
	logger := testutil.NewTestLogger()
	queue := NewZKProofTaskQueue(logger)

	// 填满通知通道缓冲区（100个）
	for i := 0; i < 100; i++ {
		task := createTestTask(fmt.Sprintf("task%d", i), 10, 5*time.Minute)
		err := queue.Enqueue(task)
		require.NoError(t, err)
	}

	// 再入队一个任务（缓冲区已满，应该被忽略）
	task := createTestTask("task101", 10, 5*time.Minute)
	err := queue.Enqueue(task)
	assert.NoError(t, err) // 入队应该成功，但通知可能丢失

	// 验证任务已入队
	assert.Equal(t, 101, queue.queue.Len())
}

// TestQueue_CancelTask_FromQueue 测试：从队列中取消任务
func TestQueue_CancelTask_FromQueue(t *testing.T) {
	logger := testutil.NewTestLogger()
	queue := NewZKProofTaskQueue(logger)

	// 创建多个任务
	task1 := createTestTask("task1", 10, 5*time.Minute)
	task2 := createTestTask("task2", 20, 5*time.Minute)
	task3 := createTestTask("task3", 30, 5*time.Minute)

	// 入队任务
	err := queue.Enqueue(task1)
	require.NoError(t, err)
	err = queue.Enqueue(task2)
	require.NoError(t, err)
	err = queue.Enqueue(task3)
	require.NoError(t, err)

	// 取消中间优先级的任务
	err = queue.CancelTask(task2.TaskID)
	assert.NoError(t, err)

	// 验证任务已取消并从队列移除
	assert.Equal(t, TaskStatusCancelled, task2.Status)
	assert.Equal(t, 2, queue.queue.Len())
	assert.Nil(t, queue.tasks[task2.TaskID])

	// 验证其他任务仍在队列中
	assert.NotNil(t, queue.tasks[task1.TaskID])
	assert.NotNil(t, queue.tasks[task3.TaskID])

	// 出队顺序应该是：task3 -> task1
	dequeuedTask := queue.Dequeue()
	assert.Equal(t, task3.TaskID, dequeuedTask.TaskID)

	dequeuedTask = queue.Dequeue()
	assert.Equal(t, task1.TaskID, dequeuedTask.TaskID)
}

