package context

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/weisyn/v1/internal/core/ispc/interfaces"
	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== 内部操作函数（不是接口方法）====================
// 这些函数处理 Manager 内部的复杂逻辑，保持 Manager 的薄实现原则

// generateExecutionID 生成执行ID（内部函数）
// 注意：这是一个全局函数，无法访问Manager的clock，但只在executionID为空时调用
// 在实际使用中，应该尽量传递非空的executionID以确保确定性
func generateExecutionID() string {
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("exec_%d", timestamp)
}

// cleanupExpiredContexts 清理过期上下文（内部函数）
//
// 📋 **参数**：无
// 🔧 **返回值**：
//   - error: 清理过程中的错误信息（当前总是返回nil）
//
// 🔒 **并发安全**：使用写锁保护，确保清理过程中不会有并发修改
// 🎯 **用途**：定期扫描并清理过期的执行上下文，防止内存泄漏
// ⏰ **调用时机**：由后台定时任务自动调用，间隔由CleanupIntervalMs配置
// ⚠️ **性能考虑**：会遍历所有活跃上下文，在高并发时可能影响性能
func (m *Manager) cleanupExpiredContexts() error {
	m.logger.Debug("开始清理过期执行上下文")

	m.mutex.Lock()
	defer m.mutex.Unlock()

	now := m.clock.Now()
	expiredContexts := make([]string, 0)

	// 查找过期的上下文
	for executionID, context := range m.contexts {
		if contextImpl, ok := context.(*contextImpl); ok {
			// ⚠️ **BUG修复**：只有设置了deadline的上下文才应该检查过期
			// 如果hasDeadline为false，即使expiresAt被设置，也不应该被清理
			if contextImpl.hasDeadline && now.After(contextImpl.expiresAt) {
				expiredContexts = append(expiredContexts, executionID)
			}
		}
	}

	// 删除过期的上下文
	for _, executionID := range expiredContexts {
		delete(m.contexts, executionID)
		m.logger.Debugf("清理过期上下文: executionID=%s", executionID)
	}

	if len(expiredContexts) > 0 {
		m.logger.Debugf("清理完成，共清理 %d 个过期上下文", len(expiredContexts))
	}

	return nil
}

// startCleanupTask 启动后台清理任务（内部函数）
//
// 启动后台任务定期清理过期上下文
func (m *Manager) startCleanupTask() {
	ticker := time.NewTicker(time.Duration(m.config.CleanupIntervalMs) * time.Millisecond)
	go func() {
		defer ticker.Stop()
		for range ticker.C {
			if err := m.cleanupExpiredContexts(); err != nil {
				m.logger.Debugf("清理过期上下文时发生错误: %v", err)
			}
		}
	}()
}

// ==================== 接口方法的内部实现（委托逻辑） ====================

// createContextInternal 负责创建执行上下文的完整逻辑
//
// 📋 **参数**：
//   - m: *Manager - 管理器实例，提供依赖服务和配置
//   - ctx: context.Context - 外部调用上下文，用于继承超时、链路追踪等信息
//   - request: interface{} - 执行请求对象，必须为*interfaces.ExecutionRequest类型
//
// 🔧 **返回值**：
//   - interfaces.ExecutionContext: 新创建的执行上下文实例
//   - error: 创建失败时的错误信息
//
// 🔒 **并发安全**：使用写锁保护contexts映射的写入操作
// 🎯 **核心功能**：
//   - 生成唯一执行ID
//   - 继承外部上下文信息（超时、链路追踪、用户身份等）
//   - 创建contextImpl实例并注册到管理器
//
// ⚠️ **上下文继承**：会提取ctx中的deadline、trace_id、user_id、request_id等信息
func (m *Manager) createContextInternal(ctx context.Context, executionID string, callerAddress string) (interfaces.ExecutionContext, error) {
	m.logger.Debug("开始创建执行上下文")

	if executionID == "" {
		executionID = generateExecutionID()
	}

	// 从外部 ctx 继承信息
	now := m.clock.Now()
	var deadline time.Time
	var hasDeadline bool

	if d, ok := ctx.Deadline(); ok {
		deadline = d
		hasDeadline = true
		m.logger.Debugf("继承外部超时时间: %v", deadline)
	} else {
		deadline = now.Add(time.Duration(m.config.MaxContextLifetime) * time.Millisecond)
		hasDeadline = false
	}

	// 提取链路追踪信息
	var traceID string
	if tid := ctx.Value("trace_id"); tid != nil {
		if tidStr, ok := tid.(string); ok {
			traceID = tidStr
			m.logger.Debugf("继承链路追踪ID: %s", traceID)
		}
	}

	// 提取用户身份信息
	var userID string
	if uid := ctx.Value("user_id"); uid != nil {
		if uidStr, ok := uid.(string); ok {
			userID = uidStr
			m.logger.Debugf("继承用户ID: %s", userID)
		}
	}

	// 提取请求ID
	var requestID string
	if rid := ctx.Value("request_id"); rid != nil {
		if ridStr, ok := rid.(string); ok {
			requestID = ridStr
			m.logger.Debugf("继承请求ID: %s", requestID)
		}
	}

	// 解码调用者地址（hex字符串 -> 字节）
	var callerAddrBytes []byte
	if callerAddress != "" {
		var err error
		callerAddrBytes, err = hex.DecodeString(callerAddress)
		if err != nil {
			m.logger.Warnf("解码调用者地址失败: %v，使用空地址", err)
			callerAddrBytes = make([]byte, 20) // 全0地址
		}
	}

	// 创建上下文实例，包含继承的信息
	// P1: 性能优化 - 预分配轨迹记录切片容量，减少内存重新分配
	contextInstance := &contextImpl{
		executionID:   executionID,
		createdAt:     now,
		expiresAt:     deadline,
		hasDeadline:   hasDeadline,
		traceID:       traceID,
		userID:        userID,
		requestID:     requestID,
		callerAddress: callerAddrBytes, // 设置调用者地址（字节）
		txDraft:       nil,
		manager:       m,
		mutex:         sync.RWMutex{},
		lastCallTime:  time.Time{}, // P1: 初始化为零值，第一次调用时会从createdAt计算
		// P1: 性能优化 - 预分配轨迹记录切片容量
		hostFunctionCalls: make([]HostFunctionCall, 0, 100), // 预分配100个容量
		stateChanges:      make([]StateChange, 0, 50),        // 预分配50个容量
		executionEvents:   make([]ExecutionEvent, 0, 50),     // 预分配50个容量
		resourceUsage: &types.ResourceUsage{
			StartTime: now,
		}, // P0: 初始化资源使用统计
		// P0: 初始化确定性增强器（使用固定时间戳）
		deterministicEnforcer: m.CreateDeterministicEnforcer(executionID, nil, &now),
		randomSource:          nil, // 延迟初始化，在需要时创建
	}

	// 如果提供了调用者地址，创建初始交易草稿
	if callerAddress != "" {
		// 生成DraftID（使用executionID + 时间戳）
		draftID := fmt.Sprintf("draft_%s_%d", executionID, now.UnixNano())

		initialDraft := &interfaces.TransactionDraft{
			DraftID:       draftID,     // ✅ 设置DraftID
			ExecutionID:   executionID, // ✅ 设置ExecutionID
			CallerAddress: callerAddress,
			CreatedAt:     now,
			Tx:            &pb.Transaction{Inputs: []*pb.TxInput{}, Outputs: []*pb.TxOutput{}},
			Outputs:       []*pb.TxOutput{},
		}
		contextInstance.txDraft = initialDraft
		m.logger.Debugf("为上下文创建初始交易草稿: draftID=%s, callerAddress=%s", draftID, callerAddress)
	}

	// 存储上下文
	// ⚠️ **BUG修复**：检查executionID是否已存在，防止覆盖
	m.mutex.Lock()
	if _, exists := m.contexts[executionID]; exists {
		m.mutex.Unlock()
		return nil, WrapContextAlreadyExistsError(executionID)
	}
	m.contexts[executionID] = contextInstance
	m.mutex.Unlock()

	// P0: 跟踪上下文创建（用于泄漏检测）
	if m.isolationEnforcer != nil {
		m.isolationEnforcer.TrackContext(executionID)
	}

	// 使用公共接口记录上下文创建（结构化日志）
	m.logger.With(
		"execution_id", executionID,
		"trace_id", traceID,
		"request_id", requestID,
		"user_id", userID,
		"caller_address", callerAddress,
		"action", "context_creation",
	).Info("执行上下文已创建")
	
	// P1: 使用调试器记录上下文创建（如果启用）
	if m.debugger != nil {
		m.debugger.LogContextCreation(executionID, traceID, requestID, userID)
	}
	
	// P0: 注册ExecutionContext到工作线程池（如果启用异步轨迹记录）
	if m.asyncTraceEnabled && m.traceWorkerPool != nil {
		m.traceWorkerPool.RegisterContext(executionID, contextInstance)
	}
	
	return contextInstance, nil
}

// destroyContextInternal 负责销毁执行上下文（幂等）
//
// 📋 **参数**：
//   - m: *Manager - 管理器实例
//   - ctx: context.Context - 外部调用上下文（当前未使用，为接口兼容性保留）
//   - executionID: string - 要销毁的执行上下文ID
//
// 🔧 **返回值**：
//   - error: 销毁失败时的错误信息，幂等设计下通常返回nil
//
// 🔒 **并发安全**：使用写锁保护contexts映射的删除操作
// 🎯 **核心功能**：从管理器中移除指定的执行上下文，释放内存资源
// ✅ **幂等设计**：如果上下文不存在，会记录日志但不返回错误
// ⚠️ **最佳实践**：应在执行完成或异常时调用，确保资源及时释放
func (m *Manager) destroyContextInternal(ctx context.Context, executionID string) error {
	m.logger.Debugf("开始销毁执行上下文: executionID=%s", executionID)

	m.mutex.Lock()
	defer m.mutex.Unlock()

	contextInstance, exists := m.contexts[executionID]
	if !exists {
		m.logger.Debugf("DestroyContext 幂等: 上下文不存在或已销毁 executionID=%s", executionID)
		return nil
	}

	// 获取上下文信息用于日志
	var traceID, requestID, userID string
	var duration time.Duration
	if ctxImpl, ok := contextInstance.(*contextImpl); ok {
		traceID = ctxImpl.traceID
		requestID = ctxImpl.requestID
		userID = ctxImpl.userID
		duration = m.clock.Now().Sub(ctxImpl.createdAt)
	}

	delete(m.contexts, executionID)
	
	// P0: 跟踪上下文销毁（用于清理验证）
	if m.isolationEnforcer != nil {
		m.isolationEnforcer.TrackDestroy(executionID)
	}
	if m.cleanupVerifier != nil {
		m.cleanupVerifier.RecordCleanup(executionID, "DestroyContext", true, "")
	}
	
	// 使用公共接口记录上下文销毁（结构化日志）
	m.logger.With(
		"execution_id", executionID,
		"trace_id", traceID,
		"request_id", requestID,
		"user_id", userID,
		"duration", duration.String(),
		"action", "context_destruction",
	).Info("执行上下文已销毁")
	
	// P1: 使用调试器记录上下文销毁（如果启用）
	if m.debugger != nil {
		m.debugger.LogContextDestruction(executionID, duration, "正常销毁")
	}
	
	// P0: 从工作线程池注销ExecutionContext（如果启用异步轨迹记录）
	if m.asyncTraceEnabled && m.traceWorkerPool != nil {
		m.traceWorkerPool.UnregisterContext(executionID)
	}
	
	return nil
}

// getContextInternal 负责获取执行上下文并校验过期
//
// 📋 **参数**：
//   - m: *Manager - 管理器实例
//   - executionID: string - 执行上下文的唯一标识符
//
// 🔧 **返回值**：
//   - interfaces.ExecutionContext: 找到的执行上下文实例
//   - error: 未找到或已过期时的错误信息
//
// 🔒 **并发安全**：使用读锁保护contexts映射的读取操作
// 🎯 **核心功能**：
//   - 从管理器中查找指定的执行上下文
//   - 自动检查上下文是否已过期
//
// ⏰ **过期检查**：会检查当前时间是否超过上下文的expiresAt时间
// ⚠️ **错误处理**：不存在或已过期都会返回相应的错误信息
func (m *Manager) getContextInternal(executionID string) (interfaces.ExecutionContext, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	context, exists := m.contexts[executionID]
	if !exists {
		return nil, fmt.Errorf("execution context not found: %s", executionID)
	}

	// ⚠️ **BUG修复**：只有设置了deadline的上下文才应该检查过期
	// 如果hasDeadline为false，即使expiresAt被设置，也不应该被视为过期
	if ctxImpl, ok := context.(*contextImpl); ok {
		if ctxImpl.hasDeadline && m.clock.Now().After(ctxImpl.expiresAt) {
			return nil, fmt.Errorf("execution context expired: %s", executionID)
		}
	}

	// P0: 跟踪上下文访问（用于泄漏检测）
	if m.isolationEnforcer != nil {
		m.isolationEnforcer.TrackAccess(executionID)
	}

	// P1: 使用调试器记录上下文访问（如果启用详细模式）
	if m.debugger != nil {
		m.debugger.LogContextAccess(executionID, "get")
	}

	return context, nil
}
