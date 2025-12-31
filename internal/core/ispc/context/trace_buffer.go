package context

import (
	"sync"
)

// ============================================================================
// 执行上下文性能优化
// ============================================================================
//
// 🎯 **目的**：
//   - 优化轨迹记录的性能
//   - 减少锁竞争
//   - 优化内存分配
//
// 📋 **优化策略**：
//   - 预分配切片容量，减少内存重新分配
//   - 使用更细粒度的锁（读写分离）
//   - 批量操作优化
//
// ============================================================================

// TraceBuffer 轨迹记录缓冲区
//
// 🎯 **性能优化**：
//   - 预分配容量，减少内存重新分配
//   - 批量追加，减少锁竞争
//   - 支持快速清空和重置
type TraceBuffer struct {
	// 宿主函数调用记录（预分配容量）
	hostFunctionCalls []HostFunctionCall
	hostCallsMutex    sync.RWMutex
	
	// 状态变更记录（预分配容量）
	stateChanges   []StateChange
	stateMutex     sync.RWMutex
	
	// 执行事件记录（预分配容量）
	executionEvents []ExecutionEvent
	eventsMutex     sync.RWMutex
	
	// 初始容量配置
	initialHostCallsCapacity int
	initialStateCapacity     int
	initialEventsCapacity    int
}

// NewTraceBuffer 创建轨迹记录缓冲区
//
// 📋 **参数**：
//   - initialHostCallsCapacity: 宿主函数调用记录的初始容量
//   - initialStateCapacity: 状态变更记录的初始容量
//   - initialEventsCapacity: 执行事件记录的初始容量
//
// 🔧 **返回值**：
//   - *TraceBuffer: 轨迹记录缓冲区实例
func NewTraceBuffer(initialHostCallsCapacity, initialStateCapacity, initialEventsCapacity int) *TraceBuffer {
	if initialHostCallsCapacity <= 0 {
		initialHostCallsCapacity = 100 // 默认容量
	}
	if initialStateCapacity <= 0 {
		initialStateCapacity = 50
	}
	if initialEventsCapacity <= 0 {
		initialEventsCapacity = 50
	}

	return &TraceBuffer{
		hostFunctionCalls:        make([]HostFunctionCall, 0, initialHostCallsCapacity),
		stateChanges:             make([]StateChange, 0, initialStateCapacity),
		executionEvents:          make([]ExecutionEvent, 0, initialEventsCapacity),
		initialHostCallsCapacity: initialHostCallsCapacity,
		initialStateCapacity:     initialStateCapacity,
		initialEventsCapacity:    initialEventsCapacity,
	}
}

// AddHostFunctionCall 添加宿主函数调用记录（线程安全）
func (b *TraceBuffer) AddHostFunctionCall(call HostFunctionCall) {
	b.hostCallsMutex.Lock()
	defer b.hostCallsMutex.Unlock()
	
	b.hostFunctionCalls = append(b.hostFunctionCalls, call)
}

// AddHostFunctionCalls 批量添加宿主函数调用记录（线程安全）
func (b *TraceBuffer) AddHostFunctionCalls(calls []HostFunctionCall) {
	if len(calls) == 0 {
		return
	}
	
	b.hostCallsMutex.Lock()
	defer b.hostCallsMutex.Unlock()
	
	b.hostFunctionCalls = append(b.hostFunctionCalls, calls...)
}

// GetHostFunctionCalls 获取宿主函数调用记录（线程安全，返回副本）
func (b *TraceBuffer) GetHostFunctionCalls() []HostFunctionCall {
	b.hostCallsMutex.RLock()
	defer b.hostCallsMutex.RUnlock()
	
	// 返回副本，避免外部修改
	result := make([]HostFunctionCall, len(b.hostFunctionCalls))
	copy(result, b.hostFunctionCalls)
	return result
}

// AddStateChange 添加状态变更记录（线程安全）
func (b *TraceBuffer) AddStateChange(change StateChange) {
	b.stateMutex.Lock()
	defer b.stateMutex.Unlock()
	
	b.stateChanges = append(b.stateChanges, change)
}

// AddStateChanges 批量添加状态变更记录（线程安全）
func (b *TraceBuffer) AddStateChanges(changes []StateChange) {
	if len(changes) == 0 {
		return
	}
	
	b.stateMutex.Lock()
	defer b.stateMutex.Unlock()
	
	b.stateChanges = append(b.stateChanges, changes...)
}

// GetStateChanges 获取状态变更记录（线程安全，返回副本）
func (b *TraceBuffer) GetStateChanges() []StateChange {
	b.stateMutex.RLock()
	defer b.stateMutex.RUnlock()
	
	// 返回副本，避免外部修改
	result := make([]StateChange, len(b.stateChanges))
	copy(result, b.stateChanges)
	return result
}

// AddExecutionEvent 添加执行事件记录（线程安全）
func (b *TraceBuffer) AddExecutionEvent(event ExecutionEvent) {
	b.eventsMutex.Lock()
	defer b.eventsMutex.Unlock()
	
	b.executionEvents = append(b.executionEvents, event)
}

// AddExecutionEvents 批量添加执行事件记录（线程安全）
func (b *TraceBuffer) AddExecutionEvents(events []ExecutionEvent) {
	if len(events) == 0 {
		return
	}
	
	b.eventsMutex.Lock()
	defer b.eventsMutex.Unlock()
	
	b.executionEvents = append(b.executionEvents, events...)
}

// GetExecutionEvents 获取执行事件记录（线程安全，返回副本）
func (b *TraceBuffer) GetExecutionEvents() []ExecutionEvent {
	b.eventsMutex.RLock()
	defer b.eventsMutex.RUnlock()
	
	// 返回副本，避免外部修改
	result := make([]ExecutionEvent, len(b.executionEvents))
	copy(result, b.executionEvents)
	return result
}

// Clear 清空所有记录（线程安全）
func (b *TraceBuffer) Clear() {
	b.hostCallsMutex.Lock()
	b.hostFunctionCalls = b.hostFunctionCalls[:0] // 保留容量，只重置长度
	b.hostCallsMutex.Unlock()
	
	b.stateMutex.Lock()
	b.stateChanges = b.stateChanges[:0]
	b.stateMutex.Unlock()
	
	b.eventsMutex.Lock()
	b.executionEvents = b.executionEvents[:0]
	b.eventsMutex.Unlock()
}

// Reset 重置缓冲区（清空并恢复初始容量）
func (b *TraceBuffer) Reset() {
	b.hostCallsMutex.Lock()
	b.hostFunctionCalls = make([]HostFunctionCall, 0, b.initialHostCallsCapacity)
	b.hostCallsMutex.Unlock()
	
	b.stateMutex.Lock()
	b.stateChanges = make([]StateChange, 0, b.initialStateCapacity)
	b.stateMutex.Unlock()
	
	b.eventsMutex.Lock()
	b.executionEvents = make([]ExecutionEvent, 0, b.initialEventsCapacity)
	b.eventsMutex.Unlock()
}

// GetStats 获取缓冲区统计信息
func (b *TraceBuffer) GetStats() map[string]interface{} {
	b.hostCallsMutex.RLock()
	hostCallsLen := len(b.hostFunctionCalls)
	hostCallsCap := cap(b.hostFunctionCalls)
	b.hostCallsMutex.RUnlock()
	
	b.stateMutex.RLock()
	stateLen := len(b.stateChanges)
	stateCap := cap(b.stateChanges)
	b.stateMutex.RUnlock()
	
	b.eventsMutex.RLock()
	eventsLen := len(b.executionEvents)
	eventsCap := cap(b.executionEvents)
	b.eventsMutex.RUnlock()
	
	return map[string]interface{}{
		"host_function_calls": map[string]interface{}{
			"count":    hostCallsLen,
			"capacity": hostCallsCap,
		},
		"state_changes": map[string]interface{}{
			"count":    stateLen,
			"capacity": stateCap,
		},
		"execution_events": map[string]interface{}{
			"count":    eventsLen,
			"capacity": eventsCap,
		},
	}
}

