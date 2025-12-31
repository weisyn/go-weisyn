package context

import (
	"encoding/json"
	"fmt"
	"runtime"
	"time"

	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ============================================================================
// 上下文调试工具（日志和调试工具）
// ============================================================================
//
// 🎯 **设计目的**：
// 提供上下文调试工具，包括生命周期日志、调试工具和状态导出功能。
//
// 🏗️ **实现策略**：
// - 上下文生命周期日志：记录创建、访问、销毁等关键生命周期事件
// - 上下文调试工具：提供调试命令和工具函数
// - 上下文状态导出：导出上下文状态为JSON格式，用于问题分析
//
// ⚠️ **注意**：
// - 调试工具主要用于开发调试阶段
// - 生产环境应该禁用详细调试日志（影响性能）
// - 状态导出可能包含敏感信息，需要谨慎处理
//
// ============================================================================

// DebugMode 调试模式
type DebugMode int

const (
	// DebugModeOff 关闭调试模式
	DebugModeOff DebugMode = iota
	// DebugModeBasic 基础调试模式（记录关键事件）
	DebugModeBasic
	// DebugModeVerbose 详细调试模式（记录所有事件）
	DebugModeVerbose
)

// String 返回调试模式字符串表示
func (m DebugMode) String() string {
	switch m {
	case DebugModeOff:
		return "off"
	case DebugModeBasic:
		return "basic"
	case DebugModeVerbose:
		return "verbose"
	default:
		return "unknown"
	}
}

// ContextDebugger 上下文调试器
type ContextDebugger struct {
	logger    log.Logger
	debugMode DebugMode
	enabled   bool
}

// NewContextDebugger 创建上下文调试器
//
// 📋 **参数**：
//   - logger: 日志记录器
//   - debugMode: 调试模式
func NewContextDebugger(logger log.Logger, debugMode DebugMode) *ContextDebugger {
	return &ContextDebugger{
		logger:    logger,
		debugMode: debugMode,
		enabled:   debugMode != DebugModeOff,
	}
}

// LogContextCreation 记录上下文创建日志
func (d *ContextDebugger) LogContextCreation(executionID string, traceID string, requestID string, userID string) {
	if !d.enabled || d.debugMode == DebugModeOff {
		return
	}

	if d.logger != nil {
		fields := []interface{}{
			"execution_id", executionID,
			"action", "context_creation",
		}
		if traceID != "" {
			fields = append(fields, "trace_id", traceID)
		}
		if requestID != "" {
			fields = append(fields, "request_id", requestID)
		}
		if userID != "" {
			fields = append(fields, "user_id", userID)
		}
		d.logger.With(fields...).Debugf("📝 创建执行上下文: %s", executionID)
	}
}

// LogContextAccess 记录上下文访问日志
func (d *ContextDebugger) LogContextAccess(executionID string, accessType string) {
	if !d.enabled || d.debugMode == DebugModeOff {
		return
	}

	if d.debugMode == DebugModeVerbose && d.logger != nil {
		d.logger.With(
			"execution_id", executionID,
			"access_type", accessType,
			"action", "context_access",
		).Debugf("🔍 访问执行上下文: %s (类型: %s)", executionID, accessType)
	}
}

// LogContextDestruction 记录上下文销毁日志
func (d *ContextDebugger) LogContextDestruction(executionID string, duration time.Duration, reason string) {
	if !d.enabled || d.debugMode == DebugModeOff {
		return
	}

	if d.logger != nil {
		fields := []interface{}{
			"execution_id", executionID,
			"duration", duration.String(),
			"action", "context_destruction",
		}
		if reason != "" {
			fields = append(fields, "reason", reason)
		}
		d.logger.With(fields...).Debugf("🗑️ 销毁执行上下文: %s (生存时间: %v)", executionID, duration)
	}
}

// LogHostFunctionCall 记录宿主函数调用日志
func (d *ContextDebugger) LogHostFunctionCall(executionID string, functionName string, duration time.Duration, success bool, err error) {
	if !d.enabled || d.debugMode == DebugModeVerbose {
		return
	}

	if d.logger != nil {
		fields := []interface{}{
			"execution_id", executionID,
			"function_name", functionName,
			"duration", duration.String(),
			"success", success,
			"action", "host_function_call",
		}
		if err != nil {
			fields = append(fields, "error", err.Error())
		}

		if success {
			d.logger.With(fields...).Debugf("🔧 宿主函数调用: %s (耗时: %v)", functionName, duration)
		} else {
			d.logger.With(fields...).Warnf("⚠️ 宿主函数调用失败: %s (错误: %v)", functionName, err)
		}
	}
}

// LogStateChange 记录状态变更日志
func (d *ContextDebugger) LogStateChange(executionID string, changeType string, key string) {
	if !d.enabled || d.debugMode == DebugModeVerbose {
		return
	}

	if d.logger != nil {
		d.logger.With(
			"execution_id", executionID,
			"change_type", changeType,
			"key", key,
			"action", "state_change",
		).Debugf("📊 状态变更: %s/%s", changeType, key)
	}
}

// SetDebugMode 设置调试模式
func (d *ContextDebugger) SetDebugMode(mode DebugMode) {
	d.debugMode = mode
	d.enabled = mode != DebugModeOff
}

// GetDebugMode 获取调试模式
func (d *ContextDebugger) GetDebugMode() DebugMode {
	return d.debugMode
}

// Enable 启用调试
func (d *ContextDebugger) Enable() {
	d.enabled = true
}

// Disable 禁用调试
func (d *ContextDebugger) Disable() {
	d.enabled = false
}

// IsEnabled 检查是否启用调试
func (d *ContextDebugger) IsEnabled() bool {
	return d.enabled
}

// ============================================================================
// 上下文状态导出
// ============================================================================

// ContextStateSnapshot 上下文状态快照
type ContextStateSnapshot struct {
	ExecutionID      string                 // 执行上下文ID
	TraceID          string                 // 追踪ID
	RequestID        string                 // 请求ID
	UserID           string                 // 用户ID
	CreatedAt        time.Time              // 创建时间
	LastAccessAt     time.Time              // 最后访问时间
	Duration         time.Duration          // 生存时间
	ContractAddress  []byte                 // 合约地址
	CallerAddress    []byte                 // 调用者地址
	TransactionID    []byte                 // 交易ID
	BlockHeight      uint64                 // 区块高度
	BlockTimestamp   uint64                 // 区块时间戳
	HostFunctionCalls int                   // 宿主函数调用次数
	StateChanges     int                    // 状态变更次数
	ExecutionEvents  int                    // 执行事件次数
	ResourceUsage    map[string]interface{} // 资源使用情况
	ReturnData       []byte                 // 返回数据
	Error            string                 // 错误信息（如果有）
	StackTrace       string                 // 堆栈跟踪（如果启用）
}

// ExportContextState 导出上下文状态
//
// 🎯 **状态导出**：
// - 导出上下文的完整状态信息
// - 用于问题分析和调试
//
// 📋 **参数**：
//   - ctx: 执行上下文
//   - includeStackTrace: 是否包含堆栈跟踪
//
// 📋 **返回值**：
//   - *ContextStateSnapshot: 上下文状态快照
//   - error: 导出错误
func ExportContextState(ctx ispcInterfaces.ExecutionContext, includeStackTrace bool) (*ContextStateSnapshot, error) {
	if ctx == nil {
		return nil, fmt.Errorf("上下文不能为 nil")
	}

	// 类型断言获取contextImpl以访问内部字段
	ctxImpl, ok := ctx.(*contextImpl)
	if !ok {
		return nil, fmt.Errorf("上下文类型错误，无法导出状态")
	}

	ctxImpl.mutex.RLock()
	defer ctxImpl.mutex.RUnlock()

	// 🎯 **修复**：使用确定性时钟获取最后访问时间，而不是 time.Now()
	var lastAccessAt time.Time
	if ctxImpl.manager != nil {
		lastAccessAt = ctxImpl.manager.GetDeterministicClock().Now()
	} else {
		// 如果 manager 不可用，使用创建时间作为后备
		lastAccessAt = ctxImpl.createdAt
	}

	snapshot := &ContextStateSnapshot{
		ExecutionID:      ctx.GetExecutionID(),
		TraceID:          ctxImpl.traceID,
		RequestID:        ctxImpl.requestID,
		UserID:           ctxImpl.userID,
		CreatedAt:        ctxImpl.createdAt,
		LastAccessAt:     lastAccessAt,
		Duration:         lastAccessAt.Sub(ctxImpl.createdAt),
		ContractAddress:  ctx.GetContractAddress(),
		CallerAddress:    ctx.GetCallerAddress(),
		TransactionID:    ctx.GetTransactionID(),
		BlockHeight:      ctx.GetBlockHeight(),
		BlockTimestamp:   ctx.GetBlockTimestamp(),
		HostFunctionCalls: len(ctxImpl.hostFunctionCalls),
		StateChanges:     len(ctxImpl.stateChanges),
		ExecutionEvents:  len(ctxImpl.executionEvents),
		ResourceUsage:    make(map[string]interface{}),
	}

	// 获取返回数据
	returnData, err := ctx.GetReturnData()
	if err == nil {
		snapshot.ReturnData = returnData
	}

	// 获取资源使用情况
	if resourceUsage := ctx.GetResourceUsage(); resourceUsage != nil {
		snapshot.ResourceUsage["execution_time_ms"] = resourceUsage.ExecutionTimeMs
		snapshot.ResourceUsage["peak_memory_bytes"] = resourceUsage.PeakMemoryBytes
		snapshot.ResourceUsage["host_function_calls"] = resourceUsage.HostFunctionCalls
		snapshot.ResourceUsage["utxo_queries"] = resourceUsage.UTXOQueries
		snapshot.ResourceUsage["resource_queries"] = resourceUsage.ResourceQueries
	}

	// 获取错误信息（如果有）
	// 注意：ExecutionContext接口没有GetError方法，这里简化处理

	// 包含堆栈跟踪（如果启用）
	if includeStackTrace {
		buf := make([]byte, 4096)
		n := runtime.Stack(buf, false)
		snapshot.StackTrace = string(buf[:n])
	}

	return snapshot, nil
}

// ExportContextStateJSON 导出上下文状态为JSON格式
//
// 📋 **参数**：
//   - ctx: 执行上下文
//   - includeStackTrace: 是否包含堆栈跟踪
//
// 📋 **返回值**：
//   - []byte: JSON格式的状态快照
//   - error: 导出错误
func ExportContextStateJSON(ctx ispcInterfaces.ExecutionContext, includeStackTrace bool) ([]byte, error) {
	snapshot, err := ExportContextState(ctx, includeStackTrace)
	if err != nil {
		return nil, err
	}

	return json.MarshalIndent(snapshot, "", "  ")
}

// ============================================================================
// 调试命令工具
// ============================================================================

// DebugCommand 调试命令类型
type DebugCommand string

const (
	// DebugCommandList 列出所有上下文
	DebugCommandList DebugCommand = "list"
	// DebugCommandShow 显示指定上下文的状态
	DebugCommandShow DebugCommand = "show"
	// DebugCommandStats 显示统计信息
	DebugCommandStats DebugCommand = "stats"
	// DebugCommandExport 导出上下文状态
	DebugCommandExport DebugCommand = "export"
	// DebugCommandLeaks 检测上下文泄漏
	DebugCommandLeaks DebugCommand = "leaks"
)

// DebugTool 调试工具
type DebugTool struct {
	manager *Manager
	logger  log.Logger
}

// NewDebugTool 创建调试工具
//
// 📋 **参数**：
//   - manager: 上下文管理器
//   - logger: 日志记录器
func NewDebugTool(manager *Manager, logger log.Logger) *DebugTool {
	return &DebugTool{
		manager: manager,
		logger:  logger,
	}
}

// ExecuteCommand 执行调试命令
//
// 📋 **参数**：
//   - command: 调试命令
//   - args: 命令参数
//
// 📋 **返回值**：
//   - interface{}: 命令执行结果
//   - error: 执行错误
func (dt *DebugTool) ExecuteCommand(command DebugCommand, args ...string) (interface{}, error) {
	switch command {
	case DebugCommandList:
		return dt.listContexts()
	case DebugCommandShow:
		if len(args) == 0 {
			return nil, fmt.Errorf("show命令需要executionID参数")
		}
		return dt.showContext(args[0])
	case DebugCommandStats:
		return dt.showStats()
	case DebugCommandExport:
		if len(args) == 0 {
			return nil, fmt.Errorf("export命令需要executionID参数")
		}
		return dt.exportContext(args[0])
	case DebugCommandLeaks:
		return dt.detectLeaks()
	default:
		return nil, fmt.Errorf("未知的调试命令: %s", command)
	}
}

// listContexts 列出所有上下文
func (dt *DebugTool) listContexts() (interface{}, error) {
	if dt.manager == nil {
		return nil, fmt.Errorf("上下文管理器未初始化")
	}

	executionIDs := dt.manager.ListContexts()
	return map[string]interface{}{
		"count":         len(executionIDs),
		"execution_ids": executionIDs,
	}, nil
}

// showContext 显示指定上下文的状态
func (dt *DebugTool) showContext(executionID string) (interface{}, error) {
	if dt.manager == nil {
		return nil, fmt.Errorf("上下文管理器未初始化")
	}

	ctx, err := dt.manager.GetContext(executionID)
	if err != nil {
		return nil, fmt.Errorf("获取上下文失败: %w", err)
	}

	return ExportContextState(ctx, false)
}

// showStats 显示统计信息
func (dt *DebugTool) showStats() (interface{}, error) {
	if dt.manager == nil {
		return nil, fmt.Errorf("上下文管理器未初始化")
	}

	return dt.manager.GetStats(), nil
}

// exportContext 导出上下文状态
func (dt *DebugTool) exportContext(executionID string) (interface{}, error) {
	if dt.manager == nil {
		return nil, fmt.Errorf("上下文管理器未初始化")
	}

	ctx, err := dt.manager.GetContext(executionID)
	if err != nil {
		return nil, fmt.Errorf("获取上下文失败: %w", err)
	}

	return ExportContextStateJSON(ctx, true)
}

// detectLeaks 检测上下文泄漏
func (dt *DebugTool) detectLeaks() (interface{}, error) {
	if dt.manager == nil {
		return nil, fmt.Errorf("上下文管理器未初始化")
	}

	leakedContexts, err := dt.manager.DetectContextLeaks()
	if err != nil {
		return nil, fmt.Errorf("检测上下文泄漏失败: %w", err)
	}

	return map[string]interface{}{
		"leaked_count": len(leakedContexts),
		"leaked_contexts": leakedContexts,
	}, nil
}

