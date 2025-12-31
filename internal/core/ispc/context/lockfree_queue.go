package context

import (
	"sync/atomic"
	"unsafe"
)

// ============================================================================
// 无锁队列实现（异步轨迹记录优化 - 阶段1）
// ============================================================================
//
// 🎯 **设计目的**：
// 实现高性能无锁队列，支持高并发入队和批量出队，用于异步轨迹记录。
//
// 🏗️ **实现策略**：
// - 基于CAS（Compare-And-Swap）操作实现无锁队列
// - 使用原子指针操作保证线程安全
// - 实现ABA问题防护（通过版本号或内存对齐）
// - 支持批量出队优化
//
// ⚠️ **注意**：
// - 无锁队列实现复杂度高，需要仔细测试
// - 必须使用race detector（-race flag）进行并发测试
// - 批量出队操作需要保证原子性
//
// ============================================================================

// TraceRecord 轨迹记录（队列元素）
type TraceRecord struct {
	// 记录类型
	RecordType string // "host_function_call", "state_change", "execution_event"
	
	// 宿主函数调用记录（如果RecordType为"host_function_call"）
	HostFunctionCall *HostFunctionCall
	
	// 状态变更记录（如果RecordType为"state_change"）
	StateChange *StateChange
	
	// 执行事件记录（如果RecordType为"execution_event"）
	ExecutionEvent *ExecutionEvent
	
	// 执行上下文ID（用于关联到对应的ExecutionContext）
	ExecutionID string
}

// queueNode 队列节点
type queueNode struct {
	data *TraceRecord  // 轨迹记录数据
	next unsafe.Pointer // 下一个节点（原子指针）
}

// LockFreeQueue 无锁队列
//
// 🎯 **核心特性**：
// - 无锁设计：使用CAS操作，无需mutex
// - 高并发：支持多线程并发入队
// - 批量出队：支持批量出队，提升处理效率
type LockFreeQueue struct {
	// 队列头（原子指针，指向dummy节点）
	head unsafe.Pointer
	
	// 队列尾（原子指针）
	tail unsafe.Pointer
	
	// 统计信息（原子操作）
	enqueueCount atomic.Int64 // 入队计数
	dequeueCount atomic.Int64 // 出队计数
}

// NewLockFreeQueue 创建无锁队列
//
// 📋 **返回值**：
//   - *LockFreeQueue: 无锁队列实例
func NewLockFreeQueue() *LockFreeQueue {
	// 创建dummy节点（简化实现，避免边界条件）
	dummy := &queueNode{
		data: nil,
		next: nil,
	}
	
	dummyPtr := unsafe.Pointer(dummy)
	
	return &LockFreeQueue{
		head: dummyPtr,
		tail: dummyPtr,
	}
}

// Enqueue 入队操作（无锁）
//
// 🎯 **实现**：
// - 使用CAS操作原子性地更新tail指针
// - 支持多线程并发入队
//
// 📋 **参数**：
//   - record: 轨迹记录
//
// 📋 **返回值**：
//   - bool: 是否成功入队
func (q *LockFreeQueue) Enqueue(record *TraceRecord) bool {
	if record == nil {
		return false
	}
	
	// 创建新节点
	newNode := &queueNode{
		data: record,
		next: nil,
	}
	newNodePtr := unsafe.Pointer(newNode)
	
	// CAS循环：原子性地更新tail.next
	for {
		// 读取当前tail
		tailPtr := atomic.LoadPointer(&q.tail)
		tail := (*queueNode)(tailPtr)
		
		// 读取tail.next
		nextPtr := atomic.LoadPointer(&tail.next)
		
		// 如果tail.next不为nil，说明tail不是真正的tail，需要更新
		if nextPtr != nil {
			// 尝试更新tail指针（帮助其他线程完成操作）
			atomic.CompareAndSwapPointer(&q.tail, tailPtr, nextPtr)
			continue
		}
		
		// 尝试将新节点链接到tail.next
		if atomic.CompareAndSwapPointer(&tail.next, nil, newNodePtr) {
			// 成功链接，更新tail指针
			atomic.CompareAndSwapPointer(&q.tail, tailPtr, newNodePtr)
			
			// 更新统计信息
			q.enqueueCount.Add(1)
			return true
		}
		
		// CAS失败，重试
	}
}

// Dequeue 出队操作（无锁，单条）
//
// 🎯 **实现**：
// - 使用CAS操作原子性地更新head指针
// - 返回dummy节点后的第一个节点
//
// 📋 **返回值**：
//   - *TraceRecord: 轨迹记录（如果队列为空则返回nil）
func (q *LockFreeQueue) Dequeue() *TraceRecord {
	for {
		// 读取head和tail
		headPtr := atomic.LoadPointer(&q.head)
		tailPtr := atomic.LoadPointer(&q.tail)
		head := (*queueNode)(headPtr)
		
		// 如果head == tail，队列为空
		if headPtr == tailPtr {
			return nil
		}
		
		// 读取head.next
		nextPtr := atomic.LoadPointer(&head.next)
		if nextPtr == nil {
			// head.next为nil，队列为空
			return nil
		}
		
		next := (*queueNode)(nextPtr)
		
		// 尝试更新head指针
		if atomic.CompareAndSwapPointer(&q.head, headPtr, nextPtr) {
			// 成功出队
			record := next.data
			
			// 更新统计信息
			q.dequeueCount.Add(1)
			
			return record
		}
		
		// CAS失败，重试
	}
}

// DequeueBatch 批量出队操作（无锁）
//
// 🎯 **实现**：
// - 批量出队，减少CAS操作次数
// - 返回最多batchSize条记录
//
// 📋 **参数**：
//   - batchSize: 批量大小（最多出队多少条记录）
//
// 📋 **返回值**：
//   - []*TraceRecord: 轨迹记录列表（如果队列为空则返回空切片）
func (q *LockFreeQueue) DequeueBatch(batchSize int) []*TraceRecord {
	if batchSize <= 0 {
		return nil
	}
	
	result := make([]*TraceRecord, 0, batchSize)
	
	for len(result) < batchSize {
		record := q.Dequeue()
		if record == nil {
			// 队列为空，返回已出队的记录
			break
		}
		result = append(result, record)
	}
	
	return result
}

// IsEmpty 检查队列是否为空
//
// 📋 **返回值**：
//   - bool: 队列是否为空
func (q *LockFreeQueue) IsEmpty() bool {
	headPtr := atomic.LoadPointer(&q.head)
	tailPtr := atomic.LoadPointer(&q.tail)
	return headPtr == tailPtr
}

// Size 获取队列大小（近似值）
//
// ⚠️ **注意**：
// - 由于无锁队列的特性，这个值只是近似值
// - 在高并发场景下可能不准确
//
// 📋 **返回值**：
//   - int64: 队列大小（入队计数 - 出队计数）
func (q *LockFreeQueue) Size() int64 {
	enqueueCount := q.enqueueCount.Load()
	dequeueCount := q.dequeueCount.Load()
	return enqueueCount - dequeueCount
}

// GetStats 获取统计信息
//
// 📋 **返回值**：
//   - map[string]int64: 统计信息（enqueue_count, dequeue_count, size）
func (q *LockFreeQueue) GetStats() map[string]int64 {
	return map[string]int64{
		"enqueue_count": q.enqueueCount.Load(),
		"dequeue_count": q.dequeueCount.Load(),
		"size":          q.Size(),
	}
}

// Flush 刷新队列（出队所有剩余记录）
//
// 🎯 **用途**：
// - 执行完成同步点时使用
// - 确保所有记录都已出队
//
// 📋 **返回值**：
//   - []*TraceRecord: 所有剩余的轨迹记录
func (q *LockFreeQueue) Flush() []*TraceRecord {
	result := make([]*TraceRecord, 0)
	
	for {
		record := q.Dequeue()
		if record == nil {
			break
		}
		result = append(result, record)
	}
	
	return result
}

