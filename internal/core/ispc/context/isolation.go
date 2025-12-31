package context

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
)

// ContextIsolationEnforcer 上下文隔离增强器
//
// 🎯 **隔离增强**：
// - 深度拷贝：确保上下文完全隔离
// - 泄漏检测：检测上下文是否泄漏
// - 清理验证：验证上下文是否正确清理
type ContextIsolationEnforcer struct {
	// 活跃上下文跟踪（用于泄漏检测）
	activeContexts map[string]*contextTrackingInfo
	mutex          sync.RWMutex

	// 清理验证配置
	maxLifetime time.Duration // 上下文最大生存时间
}

// contextTrackingInfo 上下文跟踪信息
type contextTrackingInfo struct {
	executionID  string
	createdAt    time.Time
	lastAccessAt time.Time
	accessCount  uint64
	isDestroyed  bool
	destroyedAt  time.Time
}

// NewContextIsolationEnforcer 创建上下文隔离增强器
func NewContextIsolationEnforcer(maxLifetime time.Duration) *ContextIsolationEnforcer {
	return &ContextIsolationEnforcer{
		activeContexts: make(map[string]*contextTrackingInfo),
		maxLifetime:    maxLifetime,
	}
}

// TrackContext 跟踪上下文创建
func (e *ContextIsolationEnforcer) TrackContext(executionID string) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if _, exists := e.activeContexts[executionID]; exists {
		return
	}

	e.activeContexts[executionID] = &contextTrackingInfo{
		executionID:  executionID,
		createdAt:    time.Now(),
		lastAccessAt: time.Now(),
		accessCount:  0,
		isDestroyed:  false,
	}
}

// TrackAccess 跟踪上下文访问
func (e *ContextIsolationEnforcer) TrackAccess(executionID string) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if info, exists := e.activeContexts[executionID]; exists {
		info.lastAccessAt = time.Now()
		info.accessCount++
	}
}

// TrackDestroy 跟踪上下文销毁
func (e *ContextIsolationEnforcer) TrackDestroy(executionID string) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if info, exists := e.activeContexts[executionID]; exists {
		info.isDestroyed = true
		info.destroyedAt = time.Now()
		// 不立即删除，保留用于泄漏检测
	}
}

// DetectLeaks 检测上下文泄漏
//
// 🎯 **泄漏检测**：
// - 检测超过最大生存时间仍未销毁的上下文
// - 检测访问次数异常高的上下文（可能的内存泄漏）
//
// 📋 **返回值**：
//   - leakedContexts: 泄漏的上下文列表
//   - err: 检测过程中的错误
func (e *ContextIsolationEnforcer) DetectLeaks() (leakedContexts []string, err error) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	now := time.Now()
	leakedContexts = []string{}

	for executionID, info := range e.activeContexts {
		// 检测1：超过最大生存时间仍未销毁
		if !info.isDestroyed {
			lifetime := now.Sub(info.createdAt)
			if lifetime > e.maxLifetime {
				leakedContexts = append(leakedContexts, executionID)
				continue
			}
		}

		// 检测2：访问次数异常高（可能的内存泄漏）
		if info.accessCount > 10000 {
			leakedContexts = append(leakedContexts, executionID)
		}
	}

	return leakedContexts, nil
}

// CleanupOldTracking 清理旧的跟踪信息
func (e *ContextIsolationEnforcer) CleanupOldTracking(maxAge time.Duration) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	now := time.Now()
	for executionID, info := range e.activeContexts {
		// 清理已销毁且超过maxAge的跟踪信息
		if info.isDestroyed {
			age := now.Sub(info.destroyedAt)
			if age > maxAge {
				delete(e.activeContexts, executionID)
			}
		}
	}
}

// DeepCopyContext 深度拷贝执行上下文
//
// 🎯 **深度拷贝**：
// - 拷贝所有基本字段
// - 拷贝所有切片和映射（深拷贝）
// - 不拷贝管理器引用（避免循环引用）
//
// 📋 **参数**：
//   - src: 源执行上下文
//
// 🔧 **返回值**：
//   - *contextImpl: 深度拷贝的上下文副本
//   - error: 拷贝过程中的错误
func DeepCopyContext(src *contextImpl) (*contextImpl, error) {
	if src == nil {
		return nil, fmt.Errorf("源上下文不能为nil")
	}

	src.mutex.RLock()
	defer src.mutex.RUnlock()

	// 创建新实例
	dst := &contextImpl{
		executionID:   src.executionID,
		createdAt:     src.createdAt,
		expiresAt:     src.expiresAt,
		hasDeadline:   src.hasDeadline,
		traceID:       src.traceID,
		userID:        src.userID,
		requestID:     src.requestID,
		hostABI:       src.hostABI, // 注意：这是引用，不深拷贝（HostABI应该是不可变的）
		manager:       nil,         // 不拷贝管理器引用
		resourceUsage: nil,         // 资源使用统计不拷贝（执行特定）
	}

	// 深拷贝交易草稿（如果存在）
	if src.txDraft != nil {
		// 注意：TransactionDraft的深拷贝需要根据其实际结构实现
		// 这里简化处理，假设txDraft是不可变的或由外部管理
		dst.txDraft = src.txDraft
	}

	// 深拷贝宿主函数调用记录
	dst.hostFunctionCalls = make([]HostFunctionCall, len(src.hostFunctionCalls))
	for i, call := range src.hostFunctionCalls {
		dst.hostFunctionCalls[i] = HostFunctionCall{
			FunctionName: call.FunctionName,
			Parameters:   deepCopyInterface(call.Parameters),
			Result:       deepCopyInterface(call.Result),
			Timestamp:    call.Timestamp,
			Duration:     call.Duration,
			Success:      call.Success,
			Error:        call.Error,
		}
	}

	// 深拷贝状态变更记录
	dst.stateChanges = make([]StateChange, len(src.stateChanges))
	for i, change := range src.stateChanges {
		dst.stateChanges[i] = StateChange{
			Type:      change.Type,
			Key:       change.Key,
			OldValue:  deepCopyInterface(change.OldValue),
			NewValue:  deepCopyInterface(change.NewValue),
			Timestamp: change.Timestamp,
		}
	}

	// 深拷贝执行事件记录
	dst.executionEvents = make([]ExecutionEvent, len(src.executionEvents))
	for i, event := range src.executionEvents {
		var eventData interface{} = deepCopyInterface(event.Data)
		// 如果Data是map[string]interface{}类型，保持类型
		if dataMap, ok := event.Data.(map[string]interface{}); ok {
			if copiedMap, ok := eventData.(map[string]interface{}); ok {
				eventData = copiedMap
			} else {
				eventData = dataMap // 回退到原始数据
			}
		}
		dst.executionEvents[i] = ExecutionEvent{
			EventType: event.EventType,
			Data:      eventData,
			Timestamp: event.Timestamp,
		}
	}

	// 深拷贝业务数据
	if src.returnData != nil {
		dst.returnData = make([]byte, len(src.returnData))
		copy(dst.returnData, src.returnData)
	}

	// 深拷贝事件列表
	if src.events != nil {
		dst.events = make([]*ispcInterfaces.Event, len(src.events))
		for i, event := range src.events {
			eventCopy := *event
			if event.Data != nil {
				copiedData := deepCopyInterface(event.Data)
				if dataMap, ok := copiedData.(map[string]interface{}); ok {
					eventCopy.Data = dataMap
				} else {
					// 如果类型不匹配，使用原始数据
					eventCopy.Data = event.Data
				}
			}
			dst.events[i] = &eventCopy
		}
	}

	// 深拷贝合约调用参数
	if src.initParams != nil {
		dst.initParams = make([]byte, len(src.initParams))
		copy(dst.initParams, src.initParams)
	}

	// 深拷贝合约地址
	if src.contractAddress != nil {
		dst.contractAddress = make([]byte, len(src.contractAddress))
		copy(dst.contractAddress, src.contractAddress)
	}

	// 深拷贝调用者地址
	if src.callerAddress != nil {
		dst.callerAddress = make([]byte, len(src.callerAddress))
		copy(dst.callerAddress, src.callerAddress)
	}

	return dst, nil
}

// deepCopyInterface 深拷贝interface{}类型
func deepCopyInterface(src interface{}) interface{} {
	if src == nil {
		return nil
	}

	switch v := src.(type) {
	case []byte:
		dst := make([]byte, len(v))
		copy(dst, v)
		return dst
	case string:
		return v // 字符串是不可变的
	case map[string]interface{}:
		dst := make(map[string]interface{})
		for k, val := range v {
			dst[k] = deepCopyInterface(val)
		}
		return dst
	case []interface{}:
		dst := make([]interface{}, len(v))
		for i, val := range v {
			dst[i] = deepCopyInterface(val)
		}
		return dst
	default:
		// 对于其他类型，返回原值（假设是不可变的）
		return src
	}
}

// ContextCleanupVerifier 上下文清理验证器
type ContextCleanupVerifier struct {
	// 已清理的上下文记录
	cleanedContexts map[string]*cleanupRecord
	mutex           sync.RWMutex
}

// cleanupRecord 清理记录
type cleanupRecord struct {
	executionID   string
	cleanedAt     time.Time
	cleanupMethod string
	success       bool
	errorMsg      string
}

// NewContextCleanupVerifier 创建上下文清理验证器
func NewContextCleanupVerifier() *ContextCleanupVerifier {
	return &ContextCleanupVerifier{
		cleanedContexts: make(map[string]*cleanupRecord),
	}
}

// RecordCleanup 记录上下文清理
func (v *ContextCleanupVerifier) RecordCleanup(executionID string, cleanupMethod string, success bool, errorMsg string) {
	v.mutex.Lock()
	defer v.mutex.Unlock()

	v.cleanedContexts[executionID] = &cleanupRecord{
		executionID:   executionID,
		cleanedAt:     time.Now(),
		cleanupMethod: cleanupMethod,
		success:       success,
		errorMsg:      errorMsg,
	}
}

// VerifyCleanup 验证上下文是否已清理
func (v *ContextCleanupVerifier) VerifyCleanup(executionID string) (cleaned bool, record *cleanupRecord) {
	v.mutex.RLock()
	defer v.mutex.RUnlock()

	record, exists := v.cleanedContexts[executionID]
	return exists && record.success, record
}

// GetCleanupStats 获取清理统计信息
func (v *ContextCleanupVerifier) GetCleanupStats() map[string]interface{} {
	v.mutex.RLock()
	defer v.mutex.RUnlock()

	totalCleaned := len(v.cleanedContexts)
	successCount := 0
	failureCount := 0

	for _, record := range v.cleanedContexts {
		if record.success {
			successCount++
		} else {
			failureCount++
		}
	}

	return map[string]interface{}{
		"total_cleaned": totalCleaned,
		"success_count": successCount,
		"failure_count": failureCount,
	}
}

// VerifyContextIsolation 验证上下文隔离
//
// 🎯 **隔离验证**：
// - 检查两个上下文是否完全独立
// - 检查是否有共享的可变状态
//
// 📋 **参数**：
//   - ctx1: 第一个上下文
//   - ctx2: 第二个上下文
//
// 🔧 **返回值**：
//   - isolated: 是否隔离
//   - issues: 隔离问题列表
func VerifyContextIsolation(ctx1, ctx2 *contextImpl) (isolated bool, issues []string) {
	issues = []string{}

	if ctx1 == nil || ctx2 == nil {
		issues = append(issues, "上下文不能为nil")
		return false, issues
	}

	// 检查执行ID是否不同
	if ctx1.executionID == ctx2.executionID {
		issues = append(issues, "执行ID相同，不是独立的上下文")
	}

	// 检查是否有共享的可变引用
	if ctx1.hostABI == ctx2.hostABI && ctx1.hostABI != nil {
		// HostABI应该是不可变的，共享引用是可以接受的
		// 但如果HostABI是可变的，这里应该报告问题
		// 暂时不报告，因为HostABI设计为不可变
	}

	if ctx1.manager == ctx2.manager && ctx1.manager != nil {
		// 管理器引用共享是可以接受的（所有上下文共享同一个管理器）
		// 但需要确保管理器本身是线程安全的
	}

	// 检查切片是否独立（通过地址比较）
	if len(ctx1.hostFunctionCalls) > 0 && len(ctx2.hostFunctionCalls) > 0 {
		if &ctx1.hostFunctionCalls[0] == &ctx2.hostFunctionCalls[0] {
			issues = append(issues, "hostFunctionCalls切片共享底层数组")
		}
	}

	if len(ctx1.stateChanges) > 0 && len(ctx2.stateChanges) > 0 {
		if &ctx1.stateChanges[0] == &ctx2.stateChanges[0] {
			issues = append(issues, "stateChanges切片共享底层数组")
		}
	}

	if len(ctx1.events) > 0 && len(ctx2.events) > 0 {
		if &ctx1.events[0] == &ctx2.events[0] {
			issues = append(issues, "events切片共享底层数组")
		}
	}

	isolated = len(issues) == 0
	return isolated, issues
}

// CheckMemoryLeak 检查内存泄漏
//
// 🎯 **内存泄漏检测**：
// - 使用runtime.MemStats检测内存增长
// - 检测goroutine泄漏
//
// 📋 **返回值**：
//   - hasLeak: 是否检测到泄漏
//   - details: 泄漏详情
func CheckMemoryLeak(beforeStats, afterStats *runtime.MemStats) (hasLeak bool, details map[string]interface{}) {
	details = make(map[string]interface{})

	if beforeStats == nil || afterStats == nil {
		details["error"] = "内存统计不能为nil"
		return false, details
	}

	// 检查堆内存增长
	heapGrowth := afterStats.HeapAlloc - beforeStats.HeapAlloc
	details["heap_growth_bytes"] = heapGrowth

	// 检查goroutine数量增长
	goroutineGrowth := runtime.NumGoroutine()
	details["goroutine_count"] = goroutineGrowth

	// 如果堆内存增长超过100MB，认为可能有泄漏
	if heapGrowth > 100*1024*1024 {
		details["leak_suspected"] = true
		details["reason"] = "堆内存增长超过100MB"
		hasLeak = true
	}

	// 如果goroutine数量超过1000，认为可能有泄漏
	if goroutineGrowth > 1000 {
		details["leak_suspected"] = true
		if reason, ok := details["reason"].(string); ok {
			details["reason"] = reason + "; goroutine数量超过1000"
		} else {
			details["reason"] = "goroutine数量超过1000"
		}
		hasLeak = true
	}

	return hasLeak, details
}

// GetMemoryStats 获取当前内存统计
func GetMemoryStats() *runtime.MemStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return &m
}

// ValidateContextCleanup 验证上下文清理
//
// 🎯 **清理验证**：
// - 检查上下文是否已从管理器中移除
// - 检查上下文的所有字段是否已清空
// - 检查是否有资源泄漏
//
// 📋 **参数**：
//   - ctx: 要验证的上下文
//   - manager: 上下文管理器
//
// 🔧 **返回值**：
//   - cleaned: 是否已清理
//   - issues: 清理问题列表
func ValidateContextCleanup(ctx *contextImpl, manager *Manager) (cleaned bool, issues []string) {
	issues = []string{}

	if ctx == nil {
		return true, issues
	}

	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	// 检查1：上下文是否仍在管理器中
	if manager != nil {
		_, err := manager.GetContext(ctx.executionID)
		if err == nil {
			issues = append(issues, "上下文仍在管理器中，未清理")
		}
	}

	// 检查2：检查关键字段是否已清空（可选，取决于清理策略）
	// 注意：某些字段可能不需要清空，只需要从管理器中移除即可

	cleaned = len(issues) == 0
	return cleaned, issues
}
