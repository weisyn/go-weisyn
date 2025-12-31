package zkproof

import (
	"container/heap"
	"fmt"
	"sync"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ============================================================================
// ZK证明任务队列（异步ZK证明生成优化 - 阶段1）
// ============================================================================
//
// 🎯 **设计目的**：
// 实现优先级队列管理ZK证明生成任务，支持任务提交、查询、取消和超时管理。
//
// 🏗️ **实现策略**：
// - 使用优先级队列（heap）实现任务调度
// - 实现任务状态管理
// - 添加任务超时机制
// - 支持任务查询和取消
//
// ⚠️ **注意**：
// - 优先级队列使用heap实现，优先级高的任务先处理
// - 任务超时需要自动标记为超时状态
// - 任务状态变更需要线程安全
//
// ============================================================================

// ZKProofTaskQueue ZK证明任务队列
//
// 🎯 **核心职责**：
// - 管理ZK证明生成任务
// - 支持优先级调度
// - 支持任务状态管理
// - 支持任务超时检测
type ZKProofTaskQueue struct {
	// 优先级队列（使用heap实现）
	queue *priorityQueue
	
	// 任务映射（taskID -> task）
	tasks map[string]*ZKProofTask
	
	// 任务状态变更通知通道
	notifyCh chan *ZKProofTask
	
	// 同步控制
	mutex sync.RWMutex
	
	// 日志记录器
	logger log.Logger
	
	// 超时检测器（后台goroutine）
	timeoutChecker *timeoutChecker
	
	// 是否已启动
	started bool
}

// NewZKProofTaskQueue 创建ZK证明任务队列
//
// 📋 **参数**：
//   - logger: 日志记录器
//
// 📋 **返回值**：
//   - *ZKProofTaskQueue: 任务队列实例
func NewZKProofTaskQueue(logger log.Logger) *ZKProofTaskQueue {
	q := &ZKProofTaskQueue{
		queue:    newPriorityQueue(),
		tasks:    make(map[string]*ZKProofTask),
		notifyCh: make(chan *ZKProofTask, 100), // 缓冲100个通知
		logger:   logger,
		started:  false,
	}
	
	// 初始化超时检测器
	q.timeoutChecker = newTimeoutChecker(q, logger)
	
	return q
}

// Start 启动任务队列
//
// 🎯 **启动**：
// - 启动超时检测器
func (q *ZKProofTaskQueue) Start() {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	
	if q.started {
		return
	}
	
	q.timeoutChecker.Start()
	q.started = true
	
	if q.logger != nil {
		q.logger.Infof("✅ ZK证明任务队列已启动")
	}
}

// Stop 停止任务队列
//
// 🎯 **停止**：
// - 停止超时检测器
func (q *ZKProofTaskQueue) Stop() {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	
	if !q.started {
		return
	}
	
	q.timeoutChecker.Stop()
	q.started = false
	
	if q.logger != nil {
		q.logger.Infof("✅ ZK证明任务队列已停止")
	}
}

// Enqueue 入队任务
//
// 📋 **参数**：
//   - task: ZK证明生成任务
//
// 📋 **返回值**：
//   - error: 入队错误
func (q *ZKProofTaskQueue) Enqueue(task *ZKProofTask) error {
	if task == nil {
		return fmt.Errorf("任务不能为空")
	}
	
	q.mutex.Lock()
	defer q.mutex.Unlock()
	
	// 检查任务是否已存在
	if _, exists := q.tasks[task.TaskID]; exists {
		return fmt.Errorf("任务已存在: %s", task.TaskID)
	}
	
	// 添加到队列和映射
	heap.Push(q.queue, task)
	q.tasks[task.TaskID] = task
	
	// 发送通知
	select {
	case q.notifyCh <- task:
	default:
		// 通知通道已满，忽略
	}
	
	if q.logger != nil {
		q.logger.Debugf("任务已入队: taskID=%s, priority=%d, executionID=%s", task.TaskID, task.Priority, task.ExecutionID)
	}
	
	return nil
}

// Dequeue 出队任务（优先级最高的任务）
//
// 📋 **返回值**：
//   - *ZKProofTask: 任务实例（如果队列为空返回nil）
func (q *ZKProofTaskQueue) Dequeue() *ZKProofTask {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	
	if q.queue.Len() == 0 {
		return nil
	}
	
	task := heap.Pop(q.queue).(*ZKProofTask)
	delete(q.tasks, task.TaskID)
	
	return task
}

// Peek 查看队列头部任务（不移除）
//
// 📋 **返回值**：
//   - *ZKProofTask: 任务实例（如果队列为空返回nil）
func (q *ZKProofTaskQueue) Peek() *ZKProofTask {
	q.mutex.RLock()
	defer q.mutex.RUnlock()
	
	if q.queue.Len() == 0 {
		return nil
	}
	
	return (*q.queue)[0]
}

// GetTask 获取任务（通过任务ID）
//
// 📋 **参数**：
//   - taskID: 任务ID
//
// 📋 **返回值**：
//   - *ZKProofTask: 任务实例（如果不存在返回nil）
func (q *ZKProofTaskQueue) GetTask(taskID string) *ZKProofTask {
	q.mutex.RLock()
	defer q.mutex.RUnlock()
	
	return q.tasks[taskID]
}

// UpdateTaskStatus 更新任务状态
//
// 📋 **参数**：
//   - taskID: 任务ID
//   - status: 新状态
//
// 📋 **返回值**：
//   - error: 更新错误
func (q *ZKProofTaskQueue) UpdateTaskStatus(taskID string, status TaskStatus) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	
	task, exists := q.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务不存在: %s", taskID)
	}
	
	task.Status = status
	
	// 发送通知
	select {
	case q.notifyCh <- task:
	default:
		// 通知通道已满，忽略
	}
	
	return nil
}

// CancelTask 取消任务
//
// 📋 **参数**：
//   - taskID: 任务ID
//
// 📋 **返回值**：
//   - error: 取消错误
func (q *ZKProofTaskQueue) CancelTask(taskID string) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	
	task, exists := q.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务不存在: %s", taskID)
	}
	
	// 如果任务在队列中，需要移除
	if task.Status == TaskStatusPending {
		// 从队列中移除（需要重建队列）
		newQueue := newPriorityQueue()
		for q.queue.Len() > 0 {
			t := heap.Pop(q.queue).(*ZKProofTask)
			if t.TaskID != taskID {
				heap.Push(newQueue, t)
			}
		}
		q.queue = newQueue
	}
	
	task.MarkCancelled()
	delete(q.tasks, taskID)
	
	if q.logger != nil {
		q.logger.Debugf("任务已取消: taskID=%s", taskID)
	}
	
	return nil
}

// GetStats 获取队列统计信息
//
// 📋 **返回值**：
//   - map[string]interface{}: 统计信息
func (q *ZKProofTaskQueue) GetStats() map[string]interface{} {
	q.mutex.RLock()
	defer q.mutex.RUnlock()
	
	stats := make(map[string]interface{})
	stats["queue_size"] = q.queue.Len()
	stats["total_tasks"] = len(q.tasks)
	
	// 统计各状态任务数量
	statusCounts := make(map[string]int)
	for _, task := range q.tasks {
		statusCounts[string(task.Status)]++
	}
	stats["status_counts"] = statusCounts
	
	return stats
}

// GetNotifyChannel 获取任务状态变更通知通道
//
// 📋 **返回值**：
//   - <-chan *ZKProofTask: 通知通道
func (q *ZKProofTaskQueue) GetNotifyChannel() <-chan *ZKProofTask {
	return q.notifyCh
}

// ============================================================================
// 优先级队列实现（使用heap）
// ============================================================================

// priorityQueue 优先级队列（使用heap实现）
type priorityQueue []*ZKProofTask

// newPriorityQueue 创建优先级队列
func newPriorityQueue() *priorityQueue {
	pq := make(priorityQueue, 0)
	return &pq
}

// Len 返回队列长度
func (pq priorityQueue) Len() int {
	return len(pq)
}

// Less 比较函数（优先级高的在前）
func (pq priorityQueue) Less(i, j int) bool {
	// 优先级高的在前
	if pq[i].Priority != pq[j].Priority {
		return pq[i].Priority > pq[j].Priority
	}
	// 优先级相同，创建时间早的在前
	return pq[i].CreatedAt.Before(pq[j].CreatedAt)
}

// Swap 交换元素
func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

// Push 添加元素
func (pq *priorityQueue) Push(x interface{}) {
	*pq = append(*pq, x.(*ZKProofTask))
}

// Pop 移除并返回最高优先级元素
func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	task := old[n-1]
	*pq = old[0 : n-1]
	return task
}

// ============================================================================
// 超时检测器
// ============================================================================

// timeoutChecker 超时检测器
type timeoutChecker struct {
	queue  *ZKProofTaskQueue
	logger log.Logger
	stopCh chan struct{}
	doneCh chan struct{}
}

// newTimeoutChecker 创建超时检测器
func newTimeoutChecker(queue *ZKProofTaskQueue, logger log.Logger) *timeoutChecker {
	return &timeoutChecker{
		queue:  queue,
		logger: logger,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

// Start 启动超时检测器
func (tc *timeoutChecker) Start() {
	go tc.run()
}

// Stop 停止超时检测器
func (tc *timeoutChecker) Stop() {
	close(tc.stopCh)
	<-tc.doneCh
}

// run 超时检测主循环
func (tc *timeoutChecker) run() {
	defer close(tc.doneCh)
	
	ticker := time.NewTicker(1 * time.Second) // 每秒检查一次
	defer ticker.Stop()
	
	for {
		select {
		case <-tc.stopCh:
			return
		case <-ticker.C:
			tc.checkTimeouts()
		}
	}
}

// checkTimeouts 检查超时任务
func (tc *timeoutChecker) checkTimeouts() {
	tc.queue.mutex.Lock()
	defer tc.queue.mutex.Unlock()
	
	for _, task := range tc.queue.tasks {
		if task.Status == TaskStatusPending || task.Status == TaskStatusRunning {
			if task.IsExpired() {
				task.MarkTimeout()
				if tc.logger != nil {
					tc.logger.Warnf("任务超时: taskID=%s, executionID=%s", task.TaskID, task.ExecutionID)
				}
			}
		}
	}
}

