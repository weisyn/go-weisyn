package zkproof

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ============================================================================
// ZK证明工作线程池（异步ZK证明生成优化 - 阶段2）
// ============================================================================
//
// 🎯 **设计目的**：
// 实现工作线程池，管理多个工作线程并发处理ZK证明生成任务。
//
// 🏗️ **实现策略**：
// - 使用goroutine pool（避免goroutine泄漏）
// - 实现动态worker数量调整
// - 添加worker健康检查
// - 实现任务分发策略
//
// ⚠️ **注意**：
// - 工作线程需要从任务队列中获取任务
// - 工作线程需要调用ZK证明生成器生成证明
// - 工作线程需要处理任务失败和重试
//
// ============================================================================

// ProofCallback ZK证明生成完成回调函数
//
// 📋 **参数**：
//   - task: 任务实例
//   - proof: 生成的证明（成功时非nil）
//   - err: 错误信息（失败时非nil）
type ProofCallback func(task *ZKProofTask, proof *transaction.ZKStateProof, err error)

// ZKProofWorker ZK证明工作线程
//
// 🎯 **核心职责**：
// - 从任务队列获取任务
// - 调用ZK证明生成器生成证明
// - 处理任务完成和失败
type ZKProofWorker struct {
	// 工作线程ID
	workerID int
	
	// 任务队列
	taskQueue *ZKProofTaskQueue
	
	// ZK证明管理器
	proofManager *Manager
	
	// 回调函数
	callback ProofCallback
	
	// 控制通道
	stopCh chan struct{}
	doneCh chan struct{}
	
	// 日志记录器
	logger log.Logger
	
	// 统计信息
	processedCount atomic.Int64
	successCount   atomic.Int64
	errorCount     atomic.Int64
	
	// 健康状态
	healthStatus atomic.Value // WorkerHealthStatus
	lastHealthCheck atomic.Value // time.Time
}

// WorkerHealthStatus 工作线程健康状态
type WorkerHealthStatus string

const (
	// WorkerHealthHealthy 健康
	WorkerHealthHealthy WorkerHealthStatus = "healthy"
	
	// WorkerHealthDegraded 降级（处理速度慢）
	WorkerHealthDegraded WorkerHealthStatus = "degraded"
	
	// WorkerHealthUnhealthy 不健康（连续失败）
	WorkerHealthUnhealthy WorkerHealthStatus = "unhealthy"
)

// NewZKProofWorker 创建ZK证明工作线程
//
// 📋 **参数**：
//   - workerID: 工作线程ID
//   - taskQueue: 任务队列
//   - proofManager: ZK证明管理器
//   - callback: 回调函数
//   - logger: 日志记录器
//
// 📋 **返回值**：
//   - *ZKProofWorker: 工作线程实例
func NewZKProofWorker(
	workerID int,
	taskQueue *ZKProofTaskQueue,
	proofManager *Manager,
	callback ProofCallback,
	logger log.Logger,
) *ZKProofWorker {
	worker := &ZKProofWorker{
		workerID:     workerID,
		taskQueue:    taskQueue,
		proofManager: proofManager,
		callback:     callback,
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
		logger:       logger,
	}
	
	worker.healthStatus.Store(WorkerHealthHealthy)
	worker.lastHealthCheck.Store(time.Now())
	
	return worker
}

// Start 启动工作线程
func (w *ZKProofWorker) Start() {
	go w.run()
}

// run 工作线程主循环
func (w *ZKProofWorker) run() {
	defer close(w.doneCh)
	
	for {
		select {
		case <-w.stopCh:
			return
		default:
			// 从队列获取任务
			task := w.taskQueue.Dequeue()
			if task == nil {
				// 队列为空，等待一段时间
				select {
				case <-w.stopCh:
					return
				case <-time.After(100 * time.Millisecond):
					continue
				}
			}
			
			// 处理任务
			w.processTask(task)
		}
	}
}

// processTask 处理任务
func (w *ZKProofWorker) processTask(task *ZKProofTask) {
	// 更新任务状态为运行中
	task.MarkRunning()
	w.taskQueue.UpdateTaskStatus(task.TaskID, TaskStatusRunning)
	
	// 创建上下文（带超时）
	ctx, cancel := context.WithTimeout(context.Background(), time.Until(task.TimeoutAt))
	defer cancel()
	
	// 生成ZK证明
	proof, err := w.generateProof(ctx, task)
	
	// 更新统计信息
	w.processedCount.Add(1)
	
	if err != nil {
		// 生成失败
		w.errorCount.Add(1)
		task.MarkFailed(err)
		w.taskQueue.UpdateTaskStatus(task.TaskID, TaskStatusFailed)
		
		// 检查是否可以重试
		if task.CanRetry() {
			// 重新入队（降低优先级）
			task.Priority -= 10
			if err := w.taskQueue.Enqueue(task); err != nil {
				if w.logger != nil {
					w.logger.Errorf("工作线程%d重试任务入队失败: taskID=%s, error=%v", w.workerID, task.TaskID, err)
				}
			}
		}
		
		// 调用回调
		if w.callback != nil {
			w.callback(task, nil, err)
		}
		
		// 更新健康状态
		w.updateHealthStatus(false)
	} else {
		// 生成成功
		w.successCount.Add(1)
		task.MarkCompleted(proof)
		w.taskQueue.UpdateTaskStatus(task.TaskID, TaskStatusCompleted)
		
		// 调用回调
		if w.callback != nil {
			w.callback(task, proof, nil)
		}
		
		// 更新健康状态
		w.updateHealthStatus(true)
	}
}

// generateProof 生成ZK证明
func (w *ZKProofWorker) generateProof(ctx context.Context, task *ZKProofTask) (*transaction.ZKStateProof, error) {
	// 构建ZK证明输入
	input := task.Input
	
	// 调用ZK证明管理器生成证明
	proof, err := w.proofManager.GenerateStateProof(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("生成ZK证明失败: %w", err)
	}
	
	return proof, nil
}

// updateHealthStatus 更新健康状态
func (w *ZKProofWorker) updateHealthStatus(success bool) {
	now := time.Now()
	w.lastHealthCheck.Store(now)
	
	if success {
		// 成功，设置为健康
		w.healthStatus.Store(WorkerHealthHealthy)
	} else {
		// 失败，检查连续失败次数
		errorCount := w.errorCount.Load()
		successCount := w.successCount.Load()
		
		if errorCount > 0 && successCount > 0 {
			// 有成功也有失败，检查失败率
			failureRate := float64(errorCount) / float64(errorCount+successCount)
			if failureRate > 0.5 {
				w.healthStatus.Store(WorkerHealthDegraded)
			} else {
				w.healthStatus.Store(WorkerHealthHealthy)
			}
		} else if errorCount > 10 {
			// 连续失败超过10次，设置为不健康
			w.healthStatus.Store(WorkerHealthUnhealthy)
		}
	}
}

// Stop 停止工作线程
func (w *ZKProofWorker) Stop() {
	close(w.stopCh)
	<-w.doneCh
}

// GetStats 获取统计信息
func (w *ZKProofWorker) GetStats() map[string]interface{} {
	healthStatus, _ := w.healthStatus.Load().(WorkerHealthStatus)
	lastHealthCheck, _ := w.lastHealthCheck.Load().(time.Time)
	
	return map[string]interface{}{
		"worker_id":        w.workerID,
		"processed_count":  w.processedCount.Load(),
		"success_count":    w.successCount.Load(),
		"error_count":      w.errorCount.Load(),
		"health_status":    string(healthStatus),
		"last_health_check": lastHealthCheck,
	}
}

// GetHealthStatus 获取健康状态
func (w *ZKProofWorker) GetHealthStatus() WorkerHealthStatus {
	status, _ := w.healthStatus.Load().(WorkerHealthStatus)
	return status
}

// ============================================================================
// ZK证明工作线程池（ZKProofWorkerPool）
// ============================================================================

// ZKProofWorkerPool ZK证明工作线程池
//
// 🎯 **核心职责**：
// - 管理多个工作线程
// - 动态调整worker数量
// - 负载均衡
// - 优雅关闭
type ZKProofWorkerPool struct {
	// 工作线程列表
	workers []*ZKProofWorker
	
	// 任务队列
	taskQueue *ZKProofTaskQueue
	
	// ZK证明管理器
	proofManager *Manager
	
	// 回调函数
	callback ProofCallback
	
	// 工作线程数量
	workerCount int
	
	// 最小工作线程数量
	minWorkers int
	
	// 最大工作线程数量
	maxWorkers int
	
	// 日志记录器
	logger log.Logger
	
	// 是否已启动
	started bool
	startMutex sync.Mutex
	
	// 动态调整器（后台goroutine）
	scaler *workerScaler
}

// NewZKProofWorkerPool 创建ZK证明工作线程池
//
// 📋 **参数**：
//   - taskQueue: 任务队列
//   - proofManager: ZK证明管理器
//   - callback: 回调函数
//   - workerCount: 初始工作线程数量
//   - minWorkers: 最小工作线程数量
//   - maxWorkers: 最大工作线程数量
//   - logger: 日志记录器
//
// 📋 **返回值**：
//   - *ZKProofWorkerPool: 工作线程池实例
func NewZKProofWorkerPool(
	taskQueue *ZKProofTaskQueue,
	proofManager *Manager,
	callback ProofCallback,
	workerCount int,
	minWorkers int,
	maxWorkers int,
	logger log.Logger,
) *ZKProofWorkerPool {
	if workerCount <= 0 {
		workerCount = 2 // 默认2个工作线程
	}
	if minWorkers <= 0 {
		minWorkers = 1 // 默认最小1个
	}
	if maxWorkers <= 0 {
		maxWorkers = 10 // 默认最大10个
	}
	if workerCount < minWorkers {
		workerCount = minWorkers
	}
	if workerCount > maxWorkers {
		workerCount = maxWorkers
	}
	
	pool := &ZKProofWorkerPool{
		taskQueue:    taskQueue,
		proofManager: proofManager,
		callback:    callback,
		workerCount:  workerCount,
		minWorkers:  minWorkers,
		maxWorkers:  maxWorkers,
		logger:      logger,
	}
	
	// 创建动态调整器
	pool.scaler = newWorkerScaler(pool, logger)
	
	return pool
}

// Start 启动工作线程池
func (p *ZKProofWorkerPool) Start() {
	p.startMutex.Lock()
	defer p.startMutex.Unlock()
	
	if p.started {
		return
	}
	
	// 创建工作线程
	p.workers = make([]*ZKProofWorker, p.workerCount)
	for i := 0; i < p.workerCount; i++ {
		worker := NewZKProofWorker(i, p.taskQueue, p.proofManager, p.callback, p.logger)
		p.workers[i] = worker
		worker.Start()
	}
	
	// 启动动态调整器
	p.scaler.Start()
	
	p.started = true
	
	if p.logger != nil {
		p.logger.Infof("✅ ZK证明工作线程池已启动: workerCount=%d", p.workerCount)
	}
}

// Stop 停止工作线程池
func (p *ZKProofWorkerPool) Stop() {
	p.startMutex.Lock()
	defer p.startMutex.Unlock()
	
	if !p.started {
		return
	}
	
	// 停止动态调整器
	p.scaler.Stop()
	
	// 停止所有工作线程
	for _, worker := range p.workers {
		worker.Stop()
	}
	
	p.started = false
	
	if p.logger != nil {
		p.logger.Infof("✅ ZK证明工作线程池已停止")
	}
}

// AddWorker 添加工作线程
func (p *ZKProofWorkerPool) AddWorker() error {
	p.startMutex.Lock()
	defer p.startMutex.Unlock()
	
	if len(p.workers) >= p.maxWorkers {
		return fmt.Errorf("已达到最大工作线程数量: %d", p.maxWorkers)
	}
	
	workerID := len(p.workers)
	worker := NewZKProofWorker(workerID, p.taskQueue, p.proofManager, p.callback, p.logger)
	p.workers = append(p.workers, worker)
	worker.Start()
	
	p.workerCount = len(p.workers)
	
	if p.logger != nil {
		p.logger.Infof("✅ 添加工作线程: workerID=%d, total=%d", workerID, p.workerCount)
	}
	
	return nil
}

// RemoveWorker 移除工作线程
func (p *ZKProofWorkerPool) RemoveWorker() error {
	p.startMutex.Lock()
	defer p.startMutex.Unlock()
	
	if len(p.workers) <= p.minWorkers {
		return fmt.Errorf("已达到最小工作线程数量: %d", p.minWorkers)
	}
	
	// 移除最后一个工作线程
	lastIndex := len(p.workers) - 1
	worker := p.workers[lastIndex]
	worker.Stop()
	p.workers = p.workers[:lastIndex]
	
	p.workerCount = len(p.workers)
	
	if p.logger != nil {
		p.logger.Infof("✅ 移除工作线程: workerID=%d, total=%d", lastIndex, p.workerCount)
	}
	
	return nil
}

// GetStats 获取统计信息
func (p *ZKProofWorkerPool) GetStats() map[string]interface{} {
	p.startMutex.Lock()
	defer p.startMutex.Unlock()
	
	totalProcessed := int64(0)
	totalSuccess := int64(0)
	totalErrors := int64(0)
	healthyWorkers := 0
	degradedWorkers := 0
	unhealthyWorkers := 0
	
	for _, worker := range p.workers {
		stats := worker.GetStats()
		totalProcessed += stats["processed_count"].(int64)
		totalSuccess += stats["success_count"].(int64)
		totalErrors += stats["error_count"].(int64)
		
		healthStatus := worker.GetHealthStatus()
		switch healthStatus {
		case WorkerHealthHealthy:
			healthyWorkers++
		case WorkerHealthDegraded:
			degradedWorkers++
		case WorkerHealthUnhealthy:
			unhealthyWorkers++
		}
	}
	
	return map[string]interface{}{
		"worker_count":      p.workerCount,
		"min_workers":       p.minWorkers,
		"max_workers":       p.maxWorkers,
		"total_processed":   totalProcessed,
		"total_success":     totalSuccess,
		"total_errors":      totalErrors,
		"healthy_workers":   healthyWorkers,
		"degraded_workers":  degradedWorkers,
		"unhealthy_workers": unhealthyWorkers,
	}
}

// ============================================================================
// 动态调整器（workerScaler）
// ============================================================================

// workerScaler 工作线程动态调整器
type workerScaler struct {
	pool   *ZKProofWorkerPool
	logger log.Logger
	stopCh chan struct{}
	doneCh chan struct{}
}

// newWorkerScaler 创建动态调整器
func newWorkerScaler(pool *ZKProofWorkerPool, logger log.Logger) *workerScaler {
	return &workerScaler{
		pool:   pool,
		logger: logger,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

// Start 启动动态调整器
func (s *workerScaler) Start() {
	go s.run()
}

// Stop 停止动态调整器
func (s *workerScaler) Stop() {
	close(s.stopCh)
	<-s.doneCh
}

// run 动态调整主循环
func (s *workerScaler) run() {
	defer close(s.doneCh)
	
	ticker := time.NewTicker(30 * time.Second) // 每30秒检查一次
	defer ticker.Stop()
	
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.adjustWorkers()
		}
	}
}

// adjustWorkers 调整工作线程数量
func (s *workerScaler) adjustWorkers() {
	stats := s.pool.GetStats()
	queueStats := s.pool.taskQueue.GetStats()
	
	queueSize, _ := queueStats["queue_size"].(int)
	unhealthyWorkers, _ := stats["unhealthy_workers"].(int)
	
	// 如果队列积压严重，增加工作线程
	if queueSize > 100 && len(s.pool.workers) < s.pool.maxWorkers {
		if err := s.pool.AddWorker(); err == nil {
			if s.logger != nil {
				s.logger.Infof("动态调整：增加工作线程（队列积压: %d）", queueSize)
			}
		} else {
			if s.logger != nil {
				s.logger.Warnf("增加工作线程失败: %v", err)
			}
		}
	}
	
	// 如果队列为空且工作线程过多，减少工作线程
	if queueSize == 0 && len(s.pool.workers) > s.pool.minWorkers {
		if err := s.pool.RemoveWorker(); err == nil {
			if s.logger != nil {
				s.logger.Infof("动态调整：减少工作线程（队列为空）")
			}
		} else {
			if s.logger != nil {
				s.logger.Warnf("减少工作线程失败: %v", err)
			}
		}
	}
	
	// 如果有不健康的工作线程，尝试移除并替换
	if unhealthyWorkers > 0 {
		// 这里可以添加替换不健康工作线程的逻辑
		if s.logger != nil {
			s.logger.Warnf("检测到不健康工作线程: count=%d", unhealthyWorkers)
		}
	}
}

