package context

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ============================================================================
// 后台工作线程（异步轨迹记录优化 - 阶段2）
// ============================================================================
//
// 🎯 **设计目的**：
// 实现后台工作线程池，批量处理无锁队列中的轨迹记录，写入ExecutionContext。
//
// 🏗️ **实现策略**：
// - 使用goroutine pool（避免goroutine泄漏）
// - 实现优雅关闭
// - 添加工作负载均衡
// - 批量处理队列中的记录
//
// ⚠️ **注意**：
// - 工作线程需要与ExecutionContext关联
// - 执行完成时需要确保所有记录都已写入
// - 需要处理队列为空时的等待逻辑
//
// ============================================================================

// TraceWorker 轨迹记录工作线程
//
// 🎯 **核心职责**：
// - 从无锁队列批量出队轨迹记录
// - 批量写入对应的ExecutionContext
// - 处理执行完成同步点
type TraceWorker struct {
	// 工作线程ID
	workerID int
	
	// 无锁队列（共享）
	queue *LockFreeQueue
	
	// ExecutionContext映射（executionID -> ExecutionContext）
	contexts map[string]ispcInterfaces.ExecutionContext
	contextsMutex sync.RWMutex
	
	// 批量大小
	batchSize int
	
	// 批量超时（如果队列为空，等待多久后处理）
	batchTimeout time.Duration
	
	// 批量失败重试配置
	maxRetries int           // 最大重试次数
	retryDelay time.Duration // 重试延迟
	
	// 控制通道
	stopCh chan struct{}
	doneCh chan struct{}
	started atomic.Bool // 标记是否已启动
	
	// 日志记录器
	logger log.Logger
	
	// 统计信息
	processedCount atomic.Int64
	errorCount     atomic.Int64
}

// NewTraceWorker 创建轨迹记录工作线程
//
// 📋 **参数**：
//   - workerID: 工作线程ID
//   - queue: 无锁队列
//   - batchSize: 批量大小
//   - batchTimeout: 批量超时
//   - maxRetries: 最大重试次数（默认3）
//   - retryDelay: 重试延迟（默认10ms）
//   - logger: 日志记录器
//
// 📋 **返回值**：
//   - *TraceWorker: 工作线程实例
func NewTraceWorker(
	workerID int,
	queue *LockFreeQueue,
	batchSize int,
	batchTimeout time.Duration,
	maxRetries int,
	retryDelay time.Duration,
	logger log.Logger,
) *TraceWorker {
	if batchSize <= 0 {
		batchSize = 100 // 默认批量大小
	}
	if batchTimeout <= 0 {
		batchTimeout = 100 * time.Millisecond // 默认超时100ms
	}
	if maxRetries <= 0 {
		maxRetries = 3 // 默认最大重试3次
	}
	if retryDelay <= 0 {
		retryDelay = 10 * time.Millisecond // 默认重试延迟10ms
	}
	
	return &TraceWorker{
		workerID:     workerID,
		queue:        queue,
		contexts:     make(map[string]ispcInterfaces.ExecutionContext),
		batchSize:    batchSize,
		batchTimeout: batchTimeout,
		maxRetries:   maxRetries,
		retryDelay:   retryDelay,
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
		started:      atomic.Bool{},
		logger:       logger,
	}
}

// RegisterContext 注册ExecutionContext
//
// 📋 **参数**：
//   - executionID: 执行上下文ID
//   - ctx: 执行上下文
func (w *TraceWorker) RegisterContext(executionID string, ctx ispcInterfaces.ExecutionContext) {
	w.contextsMutex.Lock()
	defer w.contextsMutex.Unlock()
	
	w.contexts[executionID] = ctx
}

// UnregisterContext 注销ExecutionContext
//
// 📋 **参数**：
//   - executionID: 执行上下文ID
func (w *TraceWorker) UnregisterContext(executionID string) {
	w.contextsMutex.Lock()
	defer w.contextsMutex.Unlock()
	
	delete(w.contexts, executionID)
}

// Start 启动工作线程
//
// 🎯 **工作流程**：
// - 循环批量出队
// - 批量写入ExecutionContext
// - 处理执行完成同步点
func (w *TraceWorker) Start() {
	go w.run()
}

// run 工作线程主循环
func (w *TraceWorker) run() {
	defer close(w.doneCh)
	
	for {
		select {
		case <-w.stopCh:
			// 收到停止信号，处理剩余记录后退出
			w.flush()
			return
		default:
			// 批量处理
			w.processBatch()
		}
	}
}

// processBatch 批量处理队列中的记录
func (w *TraceWorker) processBatch() {
	// 批量出队
	records := w.queue.DequeueBatch(w.batchSize)
	
	if len(records) == 0 {
		// 队列为空，等待一段时间
		select {
		case <-w.stopCh:
			return
		case <-time.After(w.batchTimeout):
			return
		}
	}
	
	// 按executionID分组
	recordsByExecutionID := make(map[string][]*TraceRecord)
	for _, record := range records {
		if record == nil {
			continue
		}
		recordsByExecutionID[record.ExecutionID] = append(recordsByExecutionID[record.ExecutionID], record)
	}
	
	// 批量写入对应的ExecutionContext
	for executionID, records := range recordsByExecutionID {
		if err := w.writeRecordsWithRetry(executionID, records); err != nil {
			w.errorCount.Add(1)
			if w.logger != nil {
				w.logger.Errorf("工作线程%d写入记录失败（已重试%d次）: executionID=%s, error=%v", w.workerID, w.maxRetries, executionID, err)
			}
		} else {
			w.processedCount.Add(int64(len(records)))
		}
	}
}

// writeRecordsWithRetry 批量写入记录到ExecutionContext（带重试机制）
//
// 📋 **参数**：
//   - executionID: 执行上下文ID
//   - records: 轨迹记录列表
//
// 📋 **返回值**：
//   - error: 写入错误（重试后仍失败）
func (w *TraceWorker) writeRecordsWithRetry(executionID string, records []*TraceRecord) error {
	var lastErr error
	for attempt := 0; attempt <= w.maxRetries; attempt++ {
		if attempt > 0 {
			// 重试前等待
			select {
			case <-w.stopCh:
				return fmt.Errorf("工作线程已停止")
			case <-time.After(w.retryDelay):
			}
		}
		
		err := w.writeRecords(executionID, records)
		if err == nil {
			return nil // 成功
		}
		
		lastErr = err
		
		// 如果是ExecutionContext不存在（已销毁），不需要重试
		if err.Error() == "ExecutionContext类型错误" {
			// 这种情况可能是正常的（执行完成后上下文被销毁）
			// 但为了安全起见，我们仍然记录错误
			if w.logger != nil && attempt == 0 {
				w.logger.Debugf("工作线程%d: ExecutionContext不存在（可能已销毁）: executionID=%s", w.workerID, executionID)
			}
			return err
		}
		
		// 其他错误，记录重试日志
		if w.logger != nil && attempt < w.maxRetries {
			w.logger.Warnf("工作线程%d写入记录失败，重试中 (%d/%d): executionID=%s, error=%v", w.workerID, attempt+1, w.maxRetries, executionID, err)
		}
	}
	
	return fmt.Errorf("写入失败（已重试%d次）: %w", w.maxRetries, lastErr)
}

// writeRecords 批量写入记录到ExecutionContext（使用接口方法）
//
// 📋 **参数**：
//   - executionID: 执行上下文ID
//   - records: 轨迹记录列表
//
// 📋 **返回值**：
//   - error: 写入错误
func (w *TraceWorker) writeRecords(executionID string, records []*TraceRecord) error {
	// 获取ExecutionContext
	w.contextsMutex.RLock()
	ctx, exists := w.contexts[executionID]
	w.contextsMutex.RUnlock()
	
	if !exists || ctx == nil {
		// ExecutionContext不存在或为nil，可能是已销毁
		// 这种情况是正常的（执行完成后上下文被销毁），返回nil不报错
		return nil
	}
	
	// 转换为接口类型
	interfaceRecords := make([]ispcInterfaces.TraceRecord, len(records))
	for i, record := range records {
		interfaceRecord := ispcInterfaces.TraceRecord{
			RecordType:  record.RecordType,
			ExecutionID: record.ExecutionID,
		}
		
		// 转换宿主函数调用记录
		if record.HostFunctionCall != nil {
			// 转换Parameters和Result
			var params map[string]interface{}
			var result map[string]interface{}
			if record.HostFunctionCall.Parameters != nil {
				if p, ok := record.HostFunctionCall.Parameters.(map[string]interface{}); ok {
					params = p
				}
			}
			if record.HostFunctionCall.Result != nil {
				if r, ok := record.HostFunctionCall.Result.(map[string]interface{}); ok {
					result = r
				}
			}
			
			interfaceRecord.HostFunctionCall = &ispcInterfaces.HostFunctionCall{
				Sequence:     record.HostFunctionCall.Sequence, // 使用record的Sequence，而不是索引i
				FunctionName: record.HostFunctionCall.FunctionName,
				Parameters:   params,
				Result:       result,
				Timestamp:    record.HostFunctionCall.Timestamp.UnixNano(),
			}
		}
		
		// 转换状态变更记录
		if record.StateChange != nil {
			interfaceRecord.StateChange = &ispcInterfaces.StateChangeRecord{
				Type:      record.StateChange.Type,
				Key:       record.StateChange.Key,
				OldValue:  record.StateChange.OldValue,
				NewValue:  record.StateChange.NewValue,
				Timestamp: record.StateChange.Timestamp.UnixNano(),
			}
		}
		
		// 转换执行事件记录
		if record.ExecutionEvent != nil {
			// 转换Data
			var eventData map[string]interface{}
			if record.ExecutionEvent.Data != nil {
				if d, ok := record.ExecutionEvent.Data.(map[string]interface{}); ok {
					eventData = d
				}
			}
			
			interfaceRecord.ExecutionEvent = &ispcInterfaces.ExecutionEventRecord{
				EventType: record.ExecutionEvent.EventType,
				Data:      eventData,
				Timestamp: record.ExecutionEvent.Timestamp.UnixNano(),
			}
		}
		
		interfaceRecords[i] = interfaceRecord
	}
	
	// 使用接口方法批量写入
	return ctx.RecordTraceRecords(interfaceRecords)
}

// flush 刷新队列（处理所有剩余记录）
//
// 🎯 **用途**：
// - 执行完成同步点时使用
// - 确保所有记录都已写入
func (w *TraceWorker) flush() {
	// 循环处理直到队列为空
	for {
		records := w.queue.DequeueBatch(w.batchSize)
		if len(records) == 0 {
			break
		}
		
		// 按executionID分组
		recordsByExecutionID := make(map[string][]*TraceRecord)
		for _, record := range records {
			if record == nil {
				continue
			}
			recordsByExecutionID[record.ExecutionID] = append(recordsByExecutionID[record.ExecutionID], record)
		}
		
		// 批量写入
		for executionID, records := range recordsByExecutionID {
			if err := w.writeRecordsWithRetry(executionID, records); err != nil {
				w.errorCount.Add(1)
				if w.logger != nil {
					w.logger.Errorf("工作线程%d刷新记录失败（已重试%d次）: executionID=%s, error=%v", w.workerID, w.maxRetries, executionID, err)
				}
			} else {
				w.processedCount.Add(int64(len(records)))
			}
		}
	}
}

// Stop 停止工作线程
//
// 🎯 **优雅关闭**：
// - 发送停止信号
// - 等待工作线程完成
// - 处理剩余记录
func (w *TraceWorker) Stop() {
	// 检查是否已经启动
	if !w.started.Load() {
		return // 未启动，直接返回
	}
	
	// 检查是否已经停止（通过检查doneCh是否已关闭）
	select {
	case <-w.doneCh:
		// 已经停止，直接返回
		return
	default:
		// 未停止，继续执行停止逻辑
	}
	
	// 关闭stopCh（如果已经关闭，会panic，所以需要先检查）
	select {
	case <-w.stopCh:
		// stopCh已经关闭，说明已经在停止过程中
		// 等待doneCh关闭
		<-w.doneCh
		return
	default:
		// stopCh未关闭，关闭它
		close(w.stopCh)
	}
	
	// 等待工作线程完成
	<-w.doneCh
	
	// 标记为未启动
	w.started.Store(false)
}

// GetStats 获取统计信息
//
// 📋 **返回值**：
//   - map[string]int64: 统计信息（processed_count, error_count）
func (w *TraceWorker) GetStats() map[string]int64 {
	return map[string]int64{
		"processed_count": w.processedCount.Load(),
		"error_count":     w.errorCount.Load(),
	}
}

// ============================================================================
// 工作线程池（TraceWorkerPool）
// ============================================================================

// TraceWorkerPool 轨迹记录工作线程池
//
// 🎯 **核心职责**：
// - 管理多个工作线程
// - 负载均衡
// - 优雅关闭
type TraceWorkerPool struct {
	// 工作线程列表
	workers []*TraceWorker
	
	// 无锁队列（共享）
	queue *LockFreeQueue
	
	// 工作线程数量
	workerCount int
	
	// 批量大小
	batchSize int
	
	// 批量超时
	batchTimeout time.Duration
	
	// 批量失败重试配置
	maxRetries int           // 最大重试次数
	retryDelay time.Duration // 重试延迟
	
	// 日志记录器
	logger log.Logger
	
	// 是否已启动
	started bool
	startMutex sync.Mutex
}

// NewTraceWorkerPool 创建轨迹记录工作线程池
//
// 📋 **参数**：
//   - queue: 无锁队列
//   - workerCount: 工作线程数量
//   - batchSize: 批量大小
//   - batchTimeout: 批量超时
//   - maxRetries: 最大重试次数（默认3）
//   - retryDelay: 重试延迟（默认10ms）
//   - logger: 日志记录器
//
// 📋 **返回值**：
//   - *TraceWorkerPool: 工作线程池实例
func NewTraceWorkerPool(
	queue *LockFreeQueue,
	workerCount int,
	batchSize int,
	batchTimeout time.Duration,
	maxRetries int,
	retryDelay time.Duration,
	logger log.Logger,
) *TraceWorkerPool {
	if workerCount <= 0 {
		workerCount = 2 // 默认2个工作线程
	}
	
	return &TraceWorkerPool{
		queue:        queue,
		workerCount:  workerCount,
		batchSize:    batchSize,
		batchTimeout: batchTimeout,
		maxRetries:   maxRetries,
		retryDelay:   retryDelay,
		logger:       logger,
	}
}

// Start 启动工作线程池
func (p *TraceWorkerPool) Start() {
	p.startMutex.Lock()
	defer p.startMutex.Unlock()
	
	if p.started {
		return
	}
	
	// 创建工作线程
	p.workers = make([]*TraceWorker, p.workerCount)
	for i := 0; i < p.workerCount; i++ {
		worker := NewTraceWorker(i, p.queue, p.batchSize, p.batchTimeout, p.maxRetries, p.retryDelay, p.logger)
		p.workers[i] = worker
		worker.Start()
	}
	
	p.started = true
}

// Stop 停止工作线程池
//
// 🎯 **优雅关闭**：
// - 停止所有工作线程
// - 等待所有工作线程完成
func (p *TraceWorkerPool) Stop() {
	p.startMutex.Lock()
	defer p.startMutex.Unlock()
	
	if !p.started {
		return
	}
	
	// 停止所有工作线程
	for _, worker := range p.workers {
		worker.Stop()
	}
	
	p.started = false
}

// RegisterContext 注册ExecutionContext到所有工作线程
//
// 📋 **参数**：
//   - executionID: 执行上下文ID
//   - ctx: 执行上下文
func (p *TraceWorkerPool) RegisterContext(executionID string, ctx ispcInterfaces.ExecutionContext) {
	for _, worker := range p.workers {
		worker.RegisterContext(executionID, ctx)
	}
}

// UnregisterContext 从所有工作线程注销ExecutionContext
//
// 📋 **参数**：
//   - executionID: 执行上下文ID
func (p *TraceWorkerPool) UnregisterContext(executionID string) {
	for _, worker := range p.workers {
		worker.UnregisterContext(executionID)
	}
}

// Flush 刷新队列（所有工作线程处理剩余记录）
//
// 🎯 **用途**：
// - 执行完成同步点时使用
func (p *TraceWorkerPool) Flush() {
	for _, worker := range p.workers {
		worker.flush()
	}
}

// GetStats 获取统计信息
//
// 📋 **返回值**：
//   - map[string]int64: 统计信息（所有工作线程的统计信息汇总）
func (p *TraceWorkerPool) GetStats() map[string]int64 {
	totalProcessed := int64(0)
	totalErrors := int64(0)
	
	for _, worker := range p.workers {
		stats := worker.GetStats()
		totalProcessed += stats["processed_count"]
		totalErrors += stats["error_count"]
	}
	
	return map[string]int64{
		"total_processed": totalProcessed,
		"total_errors":    totalErrors,
		"worker_count":    int64(p.workerCount),
	}
}

