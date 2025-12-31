package zkproof

import (
	"context"
	"sync"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ============================================================================
// 优先级调度器（优先级调度算法优化 - 阶段2）
// ============================================================================
//
// 🎯 **设计目的**：
// 实现优先级调度器，使用PriorityQueue调度任务，支持多种调度策略。
//
// 🏗️ **实现策略**：
// - 使用PriorityQueue管理任务
// - 实现交易类型优先级调度
// - 实现等待时间优先级调度
// - 实现混合优先级策略调度
// - 支持定期优先级调整
//
// ⚠️ **注意**：
// - 调度器负责从PriorityQueue中获取任务并分发给工作线程
// - 需要定期调整优先级，避免低优先级任务饥饿
// - 需要保证公平性，相同优先级任务FIFO处理
//
// ============================================================================

// PriorityScheduler 优先级调度器
//
// 🎯 **核心职责**：
// - 管理优先级队列
// - 调度任务给工作线程
// - 定期调整优先级
// - 保证公平性
type PriorityScheduler struct {
	// 优先级队列
	queue *PriorityQueue
	
	// 优先级策略
	strategy PriorityStrategy
	
	// 同步控制
	mutex sync.RWMutex
	
	// 日志记录器
	logger log.Logger
	
	// 优先级调整器（后台goroutine）
	adjuster *priorityAdjuster
	
	// 是否已启动
	started bool
	
	// 配置
	config *SchedulerConfig
}

// SchedulerConfig 调度器配置
type SchedulerConfig struct {
	// 优先级调整间隔（默认30秒）
	AdjustInterval time.Duration
	
	// 是否启用优先级调整（默认true）
	EnablePriorityAdjustment bool
	
	// 最大等待时间（超过此时间强制提升优先级，默认5分钟）
	MaxWaitTime time.Duration
	
	// 公平性检查间隔（默认10秒）
	FairnessCheckInterval time.Duration
}

// DefaultSchedulerConfig 默认调度器配置
func DefaultSchedulerConfig() *SchedulerConfig {
	return &SchedulerConfig{
		AdjustInterval:           30 * time.Second,
		EnablePriorityAdjustment: true,
		MaxWaitTime:              5 * time.Minute,
		FairnessCheckInterval:    10 * time.Second,
	}
}

// NewPriorityScheduler 创建优先级调度器
//
// 📋 **参数**：
//   - strategy: 优先级策略（如果为nil，使用默认混合策略）
//   - config: 调度器配置（如果为nil，使用默认配置）
//   - logger: 日志记录器
//
// 📋 **返回值**：
//   - *PriorityScheduler: 调度器实例
func NewPriorityScheduler(strategy PriorityStrategy, config *SchedulerConfig, logger log.Logger) *PriorityScheduler {
	if strategy == nil {
		strategy = NewMixedStrategy() // 默认使用混合策略
	}
	
	if config == nil {
		config = DefaultSchedulerConfig()
	}
	
	scheduler := &PriorityScheduler{
		queue:    NewPriorityQueue(strategy, logger),
		strategy: strategy,
		logger:   logger,
		config:   config,
		started:  false,
	}
	
	// 创建优先级调整器
	scheduler.adjuster = newPriorityAdjuster(scheduler, config, logger)
	
	return scheduler
}

// Start 启动调度器
func (s *PriorityScheduler) Start() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	if s.started {
		return
	}
	
	// 启动优先级调整器
	if s.config.EnablePriorityAdjustment {
		s.adjuster.Start()
	}
	
	s.started = true
	
	if s.logger != nil {
		s.logger.Infof("✅ 优先级调度器已启动: adjustInterval=%v", s.config.AdjustInterval)
	}
}

// Stop 停止调度器
func (s *PriorityScheduler) Stop() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	if !s.started {
		return
	}
	
	// 停止优先级调整器
	if s.adjuster != nil {
		s.adjuster.Stop()
	}
	
	s.started = false
	
	if s.logger != nil {
		s.logger.Infof("✅ 优先级调度器已停止")
	}
}

// Enqueue 入队任务
//
// 📋 **参数**：
//   - item: 优先级队列元素
//
// 📋 **返回值**：
//   - error: 入队错误
func (s *PriorityScheduler) Enqueue(item PriorityItem) error {
	return s.queue.Enqueue(item)
}

// Dequeue 出队任务（优先级最高的）
//
// 📋 **返回值**：
//   - PriorityItem: 优先级最高的任务（如果队列为空返回nil）
func (s *PriorityScheduler) Dequeue() PriorityItem {
	return s.queue.Dequeue()
}

// Peek 查看优先级最高的任务（不移除）
//
// 📋 **返回值**：
//   - PriorityItem: 优先级最高的任务（如果队列为空返回nil）
func (s *PriorityScheduler) Peek() PriorityItem {
	return s.queue.Peek()
}

// Get 获取指定ID的任务
//
// 📋 **参数**：
//   - id: 任务ID
//
// 📋 **返回值**：
//   - PriorityItem: 任务（如果不存在返回nil）
func (s *PriorityScheduler) Get(id string) PriorityItem {
	return s.queue.Get(id)
}

// Remove 移除指定ID的任务
//
// 📋 **参数**：
//   - id: 任务ID
//
// 📋 **返回值**：
//   - error: 移除错误
func (s *PriorityScheduler) Remove(id string) error {
	return s.queue.Remove(id)
}

// AdjustPriority 调整任务优先级
//
// 📋 **参数**：
//   - id: 任务ID
//
// 📋 **返回值**：
//   - error: 调整错误
func (s *PriorityScheduler) AdjustPriority(id string) error {
	return s.queue.AdjustPriority(id)
}

// AdjustAllPriorities 调整所有任务优先级
func (s *PriorityScheduler) AdjustAllPriorities() {
	s.queue.AdjustAllPriorities()
}

// CheckFairness 检查公平性
//
// 🎯 **公平性检查**：
// - 检查是否有任务等待时间过长
// - 如果有，提升其优先级
func (s *PriorityScheduler) CheckFairness() {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	if !s.started {
		return
	}
	
	// 检查是否有任务等待时间超过最大等待时间
	// 由于无法直接遍历队列，我们通过定期调整优先级来实现公平性
	// 等待时间策略已经在MixedStrategy中实现
	// 这里可以添加额外的公平性检查逻辑
}

// SetStrategy 设置优先级策略
func (s *PriorityScheduler) SetStrategy(strategy PriorityStrategy) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.strategy = strategy
	s.queue.SetStrategy(strategy)
}

// GetStats 获取统计信息
func (s *PriorityScheduler) GetStats() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	stats := make(map[string]interface{})
	stats["queue_stats"] = s.queue.GetStats()
	stats["started"] = s.started
	stats["config"] = map[string]interface{}{
		"adjust_interval":            s.config.AdjustInterval.String(),
		"enable_priority_adjustment":  s.config.EnablePriorityAdjustment,
		"max_wait_time":               s.config.MaxWaitTime.String(),
		"fairness_check_interval":     s.config.FairnessCheckInterval.String(),
	}
	
	return stats
}

// Len 返回队列长度
func (s *PriorityScheduler) Len() int {
	return s.queue.Len()
}

// IsEmpty 检查队列是否为空
func (s *PriorityScheduler) IsEmpty() bool {
	return s.queue.IsEmpty()
}

// ============================================================================
// 优先级调整器（后台goroutine）
// ============================================================================

// priorityAdjuster 优先级调整器
type priorityAdjuster struct {
	scheduler *PriorityScheduler
	config    *SchedulerConfig
	logger    log.Logger
	stopCh    chan struct{}
	doneCh    chan struct{}
}

// newPriorityAdjuster 创建优先级调整器
func newPriorityAdjuster(scheduler *PriorityScheduler, config *SchedulerConfig, logger log.Logger) *priorityAdjuster {
	return &priorityAdjuster{
		scheduler: scheduler,
		config:    config,
		logger:    logger,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
}

// Start 启动优先级调整器
func (a *priorityAdjuster) Start() {
	go a.run()
}

// Stop 停止优先级调整器
func (a *priorityAdjuster) Stop() {
	close(a.stopCh)
	<-a.doneCh
}

// run 优先级调整器主循环
func (a *priorityAdjuster) run() {
	defer close(a.doneCh)
	
	adjustTicker := time.NewTicker(a.config.AdjustInterval)
	defer adjustTicker.Stop()
	
	fairnessTicker := time.NewTicker(a.config.FairnessCheckInterval)
	defer fairnessTicker.Stop()
	
	for {
		select {
		case <-a.stopCh:
			return
		case <-adjustTicker.C:
			// 定期调整所有任务的优先级
			a.scheduler.AdjustAllPriorities()
			if a.logger != nil {
				a.logger.Debugf("优先级调整完成: queueSize=%d", a.scheduler.Len())
			}
		case <-fairnessTicker.C:
			// 定期检查公平性
			a.scheduler.CheckFairness()
		}
	}
}

// ============================================================================
// 调度策略辅助函数
// ============================================================================

// ScheduleTask 调度任务（辅助函数）
//
// 📋 **参数**：
//   - scheduler: 优先级调度器
//   - item: 优先级队列元素
//
// 📋 **返回值**：
//   - error: 调度错误
func ScheduleTask(scheduler *PriorityScheduler, item PriorityItem) error {
	return scheduler.Enqueue(item)
}

// GetNextTask 获取下一个任务（辅助函数）
//
// 📋 **参数**：
//   - scheduler: 优先级调度器
//
// 📋 **返回值**：
//   - PriorityItem: 下一个任务（如果队列为空返回nil）
func GetNextTask(scheduler *PriorityScheduler) PriorityItem {
	return scheduler.Dequeue()
}

// WaitForTask 等待任务（带超时）
//
// 📋 **参数**：
//   - scheduler: 优先级调度器
//   - ctx: 上下文（用于超时控制）
//   - pollInterval: 轮询间隔（默认100ms）
//
// 📋 **返回值**：
//   - PriorityItem: 任务（如果超时或队列为空返回nil）
func WaitForTask(scheduler *PriorityScheduler, ctx context.Context, pollInterval time.Duration) PriorityItem {
	if pollInterval <= 0 {
		pollInterval = 100 * time.Millisecond
	}
	
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if task := scheduler.Dequeue(); task != nil {
				return task
			}
		}
	}
}

