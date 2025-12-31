package zkproof

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/ispc/testutil"
)

// ============================================================================
// scheduler.go 测试
// ============================================================================

// TestDefaultSchedulerConfig 测试默认调度器配置
func TestDefaultSchedulerConfig(t *testing.T) {
	config := DefaultSchedulerConfig()
	require.NotNil(t, config)
	require.Equal(t, 30*time.Second, config.AdjustInterval)
	require.True(t, config.EnablePriorityAdjustment)
	require.Equal(t, 5*time.Minute, config.MaxWaitTime)
	require.Equal(t, 10*time.Second, config.FairnessCheckInterval)
}

// TestNewPriorityScheduler 测试创建优先级调度器
func TestNewPriorityScheduler(t *testing.T) {
	logger := testutil.NewTestLogger()
	strategy := NewMixedStrategy()
	config := DefaultSchedulerConfig()
	
	scheduler := NewPriorityScheduler(strategy, config, logger)
	require.NotNil(t, scheduler)
	require.NotNil(t, scheduler.queue)
	require.Equal(t, strategy, scheduler.strategy)
	require.Equal(t, config, scheduler.config)
	require.False(t, scheduler.started)
}

// TestNewPriorityScheduler_DefaultStrategy 测试使用默认策略创建调度器
func TestNewPriorityScheduler_DefaultStrategy(t *testing.T) {
	logger := testutil.NewTestLogger()
	
	scheduler := NewPriorityScheduler(nil, nil, logger)
	require.NotNil(t, scheduler)
	require.NotNil(t, scheduler.strategy)
	require.NotNil(t, scheduler.config)
}

// TestPriorityScheduler_StartStop 测试启动和停止调度器
func TestPriorityScheduler_StartStop(t *testing.T) {
	logger := testutil.NewTestLogger()
	scheduler := NewPriorityScheduler(nil, nil, logger)
	
	// 启动调度器
	scheduler.Start()
	require.True(t, scheduler.started)
	
	// 重复启动应该无影响
	scheduler.Start()
	require.True(t, scheduler.started)
	
	// 停止调度器
	scheduler.Stop()
	require.False(t, scheduler.started)
	
	// 重复停止应该无影响
	scheduler.Stop()
	require.False(t, scheduler.started)
}

// TestPriorityScheduler_EnqueueDequeue 测试入队和出队
func TestPriorityScheduler_EnqueueDequeue(t *testing.T) {
	logger := testutil.NewTestLogger()
	scheduler := NewPriorityScheduler(nil, nil, logger)
	
	item1 := &mockPriorityItem{
		id:        "item1",
		priority:  10,
		createdAt: time.Now(),
	}
	item2 := &mockPriorityItem{
		id:        "item2",
		priority:  20,
		createdAt: time.Now(),
	}
	
	err := scheduler.Enqueue(item1)
	require.NoError(t, err)
	err = scheduler.Enqueue(item2)
	require.NoError(t, err)
	
	require.Equal(t, 2, scheduler.Len())
	require.False(t, scheduler.IsEmpty())
	
	// 优先级高的先出队
	dequeued := scheduler.Dequeue()
	require.NotNil(t, dequeued)
	require.Equal(t, "item2", dequeued.GetID())
	
	require.Equal(t, 1, scheduler.Len())
}

// TestPriorityScheduler_Peek 测试查看队首元素
func TestPriorityScheduler_Peek(t *testing.T) {
	logger := testutil.NewTestLogger()
	scheduler := NewPriorityScheduler(nil, nil, logger)
	
	item := &mockPriorityItem{
		id:        "item1",
		priority:  10,
		createdAt: time.Now(),
	}
	
	scheduler.Enqueue(item)
	
	peeked := scheduler.Peek()
	require.NotNil(t, peeked)
	require.Equal(t, "item1", peeked.GetID())
	
	// Peek不应该移除元素
	require.Equal(t, 1, scheduler.Len())
}

// TestPriorityScheduler_Get 测试获取指定ID的任务
func TestPriorityScheduler_Get(t *testing.T) {
	logger := testutil.NewTestLogger()
	scheduler := NewPriorityScheduler(nil, nil, logger)
	
	item := &mockPriorityItem{
		id:        "item1",
		priority:  10,
		createdAt: time.Now(),
	}
	
	scheduler.Enqueue(item)
	
	retrieved := scheduler.Get("item1")
	require.NotNil(t, retrieved)
	require.Equal(t, "item1", retrieved.GetID())
	
	// 不存在的ID
	retrieved = scheduler.Get("nonexistent")
	require.Nil(t, retrieved)
}

// TestPriorityScheduler_Remove 测试移除任务
func TestPriorityScheduler_Remove(t *testing.T) {
	logger := testutil.NewTestLogger()
	scheduler := NewPriorityScheduler(nil, nil, logger)
	
	item := &mockPriorityItem{
		id:        "item1",
		priority:  10,
		createdAt: time.Now(),
	}
	
	scheduler.Enqueue(item)
	require.Equal(t, 1, scheduler.Len())
	
	err := scheduler.Remove("item1")
	require.NoError(t, err)
	require.Equal(t, 0, scheduler.Len())
}

// TestPriorityScheduler_AdjustPriority 测试调整任务优先级
func TestPriorityScheduler_AdjustPriority(t *testing.T) {
	logger := testutil.NewTestLogger()
	scheduler := NewPriorityScheduler(nil, nil, logger)
	
	item := &mockPriorityItem{
		id:        "item1",
		priority:  10,
		createdAt: time.Now().Add(-time.Minute), // 1分钟前创建
	}
	
	scheduler.Enqueue(item)
	
	err := scheduler.AdjustPriority("item1")
	require.NoError(t, err)
	// 等待时间策略会增加优先级
	require.Greater(t, item.GetPriority(), 10)
}

// TestPriorityScheduler_AdjustAllPriorities 测试调整所有任务优先级
func TestPriorityScheduler_AdjustAllPriorities(t *testing.T) {
	logger := testutil.NewTestLogger()
	scheduler := NewPriorityScheduler(nil, nil, logger)
	
	item1 := &mockPriorityItem{
		id:        "item1",
		priority:  10,
		createdAt: time.Now().Add(-time.Minute),
	}
	item2 := &mockPriorityItem{
		id:        "item2",
		priority:  20,
		createdAt: time.Now().Add(-2 * time.Minute),
	}
	
	scheduler.Enqueue(item1)
	scheduler.Enqueue(item2)
	
	scheduler.AdjustAllPriorities()
	
	// 等待时间更长的应该优先级更高
	require.Greater(t, item2.GetPriority(), item1.GetPriority())
}

// TestPriorityScheduler_CheckFairness 测试检查公平性
func TestPriorityScheduler_CheckFairness(t *testing.T) {
	logger := testutil.NewTestLogger()
	scheduler := NewPriorityScheduler(nil, nil, logger)
	
	// 未启动的调度器
	scheduler.CheckFairness()
	require.False(t, scheduler.started)
	
	// 启动后检查公平性
	scheduler.Start()
	defer scheduler.Stop()
	
	scheduler.CheckFairness()
	// 应该正常执行，不报错
}

// TestPriorityScheduler_SetStrategy 测试设置优先级策略
func TestPriorityScheduler_SetStrategy(t *testing.T) {
	logger := testutil.NewTestLogger()
	scheduler := NewPriorityScheduler(nil, nil, logger)
	
	newStrategy := NewTransactionTypeStrategy()
	scheduler.SetStrategy(newStrategy)
	
	require.Equal(t, newStrategy, scheduler.strategy)
}

// TestPriorityScheduler_GetStats 测试获取统计信息
func TestPriorityScheduler_GetStats(t *testing.T) {
	logger := testutil.NewTestLogger()
	scheduler := NewPriorityScheduler(nil, nil, logger)
	
	item := &mockPriorityItem{
		id:        "item1",
		priority:  10,
		createdAt: time.Now(),
	}
	
	scheduler.Enqueue(item)
	
	stats := scheduler.GetStats()
	require.NotNil(t, stats)
	require.False(t, stats["started"].(bool))
	require.NotNil(t, stats["queue_stats"])
	require.NotNil(t, stats["config"])
}

// TestPriorityScheduler_LenIsEmpty 测试队列长度和空检查
func TestPriorityScheduler_LenIsEmpty(t *testing.T) {
	logger := testutil.NewTestLogger()
	scheduler := NewPriorityScheduler(nil, nil, logger)
	
	require.True(t, scheduler.IsEmpty())
	require.Equal(t, 0, scheduler.Len())
	
	item := &mockPriorityItem{
		id:        "item1",
		priority:  10,
		createdAt: time.Now(),
	}
	
	scheduler.Enqueue(item)
	require.False(t, scheduler.IsEmpty())
	require.Equal(t, 1, scheduler.Len())
}

// TestScheduleTask 测试调度任务辅助函数
func TestScheduleTask(t *testing.T) {
	logger := testutil.NewTestLogger()
	scheduler := NewPriorityScheduler(nil, nil, logger)
	
	item := &mockPriorityItem{
		id:        "item1",
		priority:  10,
		createdAt: time.Now(),
	}
	
	err := ScheduleTask(scheduler, item)
	require.NoError(t, err)
	require.Equal(t, 1, scheduler.Len())
}

// TestGetNextTask 测试获取下一个任务辅助函数
func TestGetNextTask(t *testing.T) {
	logger := testutil.NewTestLogger()
	scheduler := NewPriorityScheduler(nil, nil, logger)
	
	item := &mockPriorityItem{
		id:        "item1",
		priority:  10,
		createdAt: time.Now(),
	}
	
	scheduler.Enqueue(item)
	
	task := GetNextTask(scheduler)
	require.NotNil(t, task)
	require.Equal(t, "item1", task.GetID())
}

// TestWaitForTask 测试等待任务（带超时）
func TestWaitForTask(t *testing.T) {
	logger := testutil.NewTestLogger()
	scheduler := NewPriorityScheduler(nil, nil, logger)
	
	// 测试超时情况
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	task := WaitForTask(scheduler, ctx, 50*time.Millisecond)
	require.Nil(t, task) // 超时应该返回nil
	
	// 测试有任务的情况
	item := &mockPriorityItem{
		id:        "item1",
		priority:  10,
		createdAt: time.Now(),
	}
	scheduler.Enqueue(item)
	
	ctx2, cancel2 := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel2()
	
	task = WaitForTask(scheduler, ctx2, 50*time.Millisecond)
	require.NotNil(t, task)
	require.Equal(t, "item1", task.GetID())
}

// TestWaitForTask_ContextCancelled 测试上下文取消
func TestWaitForTask_ContextCancelled(t *testing.T) {
	logger := testutil.NewTestLogger()
	scheduler := NewPriorityScheduler(nil, nil, logger)
	
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消
	
	task := WaitForTask(scheduler, ctx, 50*time.Millisecond)
	require.Nil(t, task)
}

// TestWaitForTask_DefaultPollInterval 测试默认轮询间隔
func TestWaitForTask_DefaultPollInterval(t *testing.T) {
	logger := testutil.NewTestLogger()
	scheduler := NewPriorityScheduler(nil, nil, logger)
	
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	
	// 使用0作为pollInterval，应该使用默认值
	task := WaitForTask(scheduler, ctx, 0)
	require.Nil(t, task) // 超时应该返回nil
}

