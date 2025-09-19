package runtime

import (
	"fmt"
	"strings"
	"time"
)

// ErrorType WASM错误类型枚举
type ErrorType string

const (
	// 执行错误
	ErrorTypeExecution         ErrorType = "execution"
	ErrorTypeMemory            ErrorType = "memory"
	ErrorTypeTimeout           ErrorType = "timeout"
	ErrorTypeResourceExhausted ErrorType = "resource_exhausted"
	ErrorTypeStackOverflow     ErrorType = "stack_overflow"

	// 编译错误
	ErrorTypeCompilation  ErrorType = "compilation"
	ErrorTypeValidation   ErrorType = "validation"
	ErrorTypeOptimization ErrorType = "optimization"

	// 模块错误
	ErrorTypeModuleNotFound   ErrorType = "module_not_found"
	ErrorTypeFunctionNotFound ErrorType = "function_not_found"
	ErrorTypeImportError      ErrorType = "import_error"
	ErrorTypeExportError      ErrorType = "export_error"

	// 宿主错误
	ErrorTypeHostFunction   ErrorType = "host_function"
	ErrorTypeHostPermission ErrorType = "host_permission"
	ErrorTypeHostResource   ErrorType = "host_resource"

	// 系统错误
	ErrorTypeSystem        ErrorType = "system"
	ErrorTypeConfiguration ErrorType = "configuration"
	ErrorTypeInternal      ErrorType = "internal"
)

// ErrorSeverity 错误严重性等级
type ErrorSeverity int

const (
	SeverityLow      ErrorSeverity = 1
	SeverityMedium   ErrorSeverity = 2
	SeverityHigh     ErrorSeverity = 3
	SeverityCritical ErrorSeverity = 4
)

// WASMError WASM统一错误类型
// 提供标准化的错误信息格式和处理机制
type WASMError struct {
	// 错误类型
	Type ErrorType `json:"type"`

	// 严重性等级
	Severity ErrorSeverity `json:"severity"`

	// 错误代码
	Code string `json:"code"`

	// 错误消息
	Message string `json:"message"`

	// 详细描述
	Details string `json:"details,omitempty"`

	// 根本原因错误
	Cause error `json:"-"`

	// 错误上下文
	Context map[string]interface{} `json:"context,omitempty"`

	// 发生时间
	Timestamp time.Time `json:"timestamp"`

	// 调用栈
	StackTrace []string `json:"stackTrace,omitempty"`

	// 恢复建议
	RecoveryHints []string `json:"recoveryHints,omitempty"`
}

// Error 实现error接口
func (we *WASMError) Error() string {
	var parts []string

	parts = append(parts, fmt.Sprintf("[%s]", we.Type))

	if we.Code != "" {
		parts = append(parts, fmt.Sprintf("(%s)", we.Code))
	}

	parts = append(parts, we.Message)

	if we.Details != "" {
		parts = append(parts, fmt.Sprintf("- %s", we.Details))
	}

	if we.Cause != nil {
		parts = append(parts, fmt.Sprintf("caused by: %v", we.Cause))
	}

	return strings.Join(parts, " ")
}

// Unwrap 返回根本原因错误
func (we *WASMError) Unwrap() error {
	return we.Cause
}

// Is 检查错误类型
func (we *WASMError) Is(target error) bool {
	if target == nil {
		return false
	}

	if targetWASMError, ok := target.(*WASMError); ok {
		return we.Type == targetWASMError.Type && we.Code == targetWASMError.Code
	}

	return false
}

// GetType 获取错误类型
func (we *WASMError) GetType() ErrorType {
	return we.Type
}

// GetSeverity 获取严重性等级
func (we *WASMError) GetSeverity() ErrorSeverity {
	return we.Severity
}

// GetCode 获取错误代码
func (we *WASMError) GetCode() string {
	return we.Code
}

// IsRecoverable 判断错误是否可恢复
func (we *WASMError) IsRecoverable() bool {
	switch we.Type {
	case ErrorTypeTimeout, ErrorTypeResourceExhausted:
		return true
	case ErrorTypeMemory, ErrorTypeStackOverflow:
		return we.Severity <= SeverityMedium
	case ErrorTypeHostFunction, ErrorTypeHostPermission:
		return true
	default:
		return false
	}
}

// AddContext 添加错误上下文
func (we *WASMError) AddContext(key string, value interface{}) *WASMError {
	if we.Context == nil {
		we.Context = make(map[string]interface{})
	}
	we.Context[key] = value
	return we
}

// AddRecoveryHint 添加恢复建议
func (we *WASMError) AddRecoveryHint(hint string) *WASMError {
	we.RecoveryHints = append(we.RecoveryHints, hint)
	return we
}

// ErrorBuilder 错误构建器
type ErrorBuilder struct {
	error *WASMError
}

// NewError 创建新的WASM错误
func NewError(errorType ErrorType, code string, message string) *ErrorBuilder {
	return &ErrorBuilder{
		error: &WASMError{
			Type:      errorType,
			Code:      code,
			Message:   message,
			Timestamp: time.Now(),
			Context:   make(map[string]interface{}),
		},
	}
}

// WithSeverity 设置严重性等级
func (eb *ErrorBuilder) WithSeverity(severity ErrorSeverity) *ErrorBuilder {
	eb.error.Severity = severity
	return eb
}

// WithDetails 设置详细描述
func (eb *ErrorBuilder) WithDetails(details string) *ErrorBuilder {
	eb.error.Details = details
	return eb
}

// WithCause 设置根本原因错误
func (eb *ErrorBuilder) WithCause(cause error) *ErrorBuilder {
	eb.error.Cause = cause
	return eb
}

// WithContext 添加上下文信息
func (eb *ErrorBuilder) WithContext(key string, value interface{}) *ErrorBuilder {
	if eb.error.Context == nil {
		eb.error.Context = make(map[string]interface{})
	}
	eb.error.Context[key] = value
	return eb
}

// WithStackTrace 设置调用栈
func (eb *ErrorBuilder) WithStackTrace(trace []string) *ErrorBuilder {
	eb.error.StackTrace = trace
	return eb
}

// WithRecoveryHint 添加恢复建议
func (eb *ErrorBuilder) WithRecoveryHint(hint string) *ErrorBuilder {
	eb.error.RecoveryHints = append(eb.error.RecoveryHints, hint)
	return eb
}

// Build 构建错误对象
func (eb *ErrorBuilder) Build() *WASMError {
	// 设置默认严重性
	if eb.error.Severity == 0 {
		eb.error.Severity = eb.getDefaultSeverity()
	}

	// 添加默认恢复建议
	if len(eb.error.RecoveryHints) == 0 {
		eb.addDefaultRecoveryHints()
	}

	return eb.error
}

// getDefaultSeverity 获取默认严重性等级
func (eb *ErrorBuilder) getDefaultSeverity() ErrorSeverity {
	switch eb.error.Type {
	case ErrorTypeTimeout, ErrorTypeResourceExhausted:
		return SeverityMedium
	case ErrorTypeMemory, ErrorTypeStackOverflow:
		return SeverityHigh
	case ErrorTypeSystem, ErrorTypeInternal:
		return SeverityCritical
	default:
		return SeverityLow
	}
}

// addDefaultRecoveryHints 添加默认恢复建议
func (eb *ErrorBuilder) addDefaultRecoveryHints() {
	switch eb.error.Type {
	case ErrorTypeTimeout:
		eb.error.RecoveryHints = append(eb.error.RecoveryHints, "增加执行超时时间", "优化合约代码逻辑")
	case ErrorTypeResourceExhausted:
		eb.error.RecoveryHints = append(eb.error.RecoveryHints, "增加资源限制", "优化合约资源使用")
	case ErrorTypeMemory:
		eb.error.RecoveryHints = append(eb.error.RecoveryHints, "增加内存限制", "优化内存使用")
	case ErrorTypeCompilation:
		eb.error.RecoveryHints = append(eb.error.RecoveryHints, "检查WASM字节码格式", "验证模块导入导出")
	case ErrorTypeHostFunction:
		eb.error.RecoveryHints = append(eb.error.RecoveryHints, "检查宿主函数权限", "验证参数格式")
	}
}

// ==================== 预定义错误 ====================

// NewExecutionError 创建执行错误
func NewExecutionError(code string, message string, cause error) *WASMError {
	return NewError(ErrorTypeExecution, code, message).
		WithCause(cause).
		WithSeverity(SeverityHigh).
		Build()
}

// NewMemoryError 创建内存错误
func NewMemoryError(code string, message string) *WASMError {
	return NewError(ErrorTypeMemory, code, message).
		WithSeverity(SeverityHigh).
		WithRecoveryHint("检查内存使用情况").
		WithRecoveryHint("增加内存限制").
		Build()
}

// NewTimeoutError 创建超时错误
func NewTimeoutError(timeout time.Duration) *WASMError {
	return NewError(ErrorTypeTimeout, "EXECUTION_TIMEOUT", "执行超时").
		WithDetails(fmt.Sprintf("执行时间超过限制: %v", timeout)).
		WithSeverity(SeverityMedium).
		WithRecoveryHint("增加执行超时时间").
		WithRecoveryHint("优化合约执行逻辑").
		Build()
}

// NewResourceExhaustedError 创建资源耗尽错误
func NewResourceExhaustedError(resourceUsed, ExecutionFeeLimit uint64) *WASMError {
	return NewError(ErrorTypeResourceExhausted, "GAS_EXHAUSTED", "资源耗尽").
		WithDetails(fmt.Sprintf("资源使用: %d, 限制: %d", resourceUsed, ExecutionFeeLimit)).
		WithSeverity(SeverityMedium).
		WithContext("resourceUsed", resourceUsed).
		WithContext("ExecutionFeeLimit", ExecutionFeeLimit).
		WithRecoveryHint("增加资源限制").
		WithRecoveryHint("优化合约资源使用").
		Build()
}

// NewCompilationError 创建编译错误
func NewCompilationError(code string, message string, cause error) *WASMError {
	return NewError(ErrorTypeCompilation, code, message).
		WithCause(cause).
		WithSeverity(SeverityHigh).
		WithRecoveryHint("检查WASM字节码格式").
		Build()
}

// NewValidationError 创建验证错误
func NewValidationError(code string, message string) *WASMError {
	return NewError(ErrorTypeValidation, code, message).
		WithSeverity(SeverityHigh).
		WithRecoveryHint("检查模块格式").
		WithRecoveryHint("验证导入导出签名").
		Build()
}

// NewFunctionNotFoundError 创建函数未找到错误
func NewFunctionNotFoundError(functionName string) *WASMError {
	return NewError(ErrorTypeFunctionNotFound, "FUNCTION_NOT_FOUND", "函数未找到").
		WithDetails(fmt.Sprintf("函数名: %s", functionName)).
		WithSeverity(SeverityMedium).
		WithContext("functionName", functionName).
		WithRecoveryHint("检查函数名称拼写").
		WithRecoveryHint("验证模块导出函数").
		Build()
}

// NewHostFunctionError 创建宿主函数错误
func NewHostFunctionError(code string, message string, functionName string) *WASMError {
	return NewError(ErrorTypeHostFunction, code, message).
		WithDetails(fmt.Sprintf("宿主函数: %s", functionName)).
		WithSeverity(SeverityMedium).
		WithContext("hostFunction", functionName).
		WithRecoveryHint("检查宿主函数权限").
		WithRecoveryHint("验证参数格式").
		Build()
}

// NewHostPermissionError 创建宿主权限错误
func NewHostPermissionError(operation string, permission string) *WASMError {
	return NewError(ErrorTypeHostPermission, "HOST_PERMISSION_DENIED", "宿主权限拒绝").
		WithDetails(fmt.Sprintf("操作: %s, 权限: %s", operation, permission)).
		WithSeverity(SeverityMedium).
		WithContext("operation", operation).
		WithContext("permission", permission).
		WithRecoveryHint("检查操作权限配置").
		WithRecoveryHint("验证安全策略").
		Build()
}

// NewSystemError 创建系统错误
func NewSystemError(code string, message string, cause error) *WASMError {
	return NewError(ErrorTypeSystem, code, message).
		WithCause(cause).
		WithSeverity(SeverityCritical).
		WithRecoveryHint("检查系统资源").
		WithRecoveryHint("重启引擎服务").
		Build()
}

// NewConfigurationError 创建配置错误
func NewConfigurationError(code string, message string, configKey string) *WASMError {
	return NewError(ErrorTypeConfiguration, code, message).
		WithDetails(fmt.Sprintf("配置项: %s", configKey)).
		WithSeverity(SeverityHigh).
		WithContext("configKey", configKey).
		WithRecoveryHint("检查配置文件格式").
		WithRecoveryHint("验证配置参数范围").
		Build()
}

// ==================== 错误处理器 ====================

// ErrorHandler 错误处理器
type ErrorHandler struct {
	// 错误统计
	stats *ErrorStats

	// 错误回调
	callbacks []ErrorCallback
}

// ErrorStats 错误统计
type ErrorStats struct {
	// 总错误数
	TotalErrors uint64 `json:"totalErrors"`

	// 按类型分组的错误数
	ErrorsByType map[ErrorType]uint64 `json:"errorsByType"`

	// 按严重性分组的错误数
	ErrorsBySeverity map[ErrorSeverity]uint64 `json:"errorsBySeverity"`

	// 可恢复错误数
	RecoverableErrors uint64 `json:"recoverableErrors"`

	// 最后错误时间
	LastErrorTime time.Time `json:"lastErrorTime"`
}

// ErrorCallback 错误处理回调函数类型
type ErrorCallback func(*WASMError) error

// NewErrorHandler 创建错误处理器
func NewErrorHandler() *ErrorHandler {
	return &ErrorHandler{
		stats: &ErrorStats{
			ErrorsByType:     make(map[ErrorType]uint64),
			ErrorsBySeverity: make(map[ErrorSeverity]uint64),
		},
		callbacks: make([]ErrorCallback, 0),
	}
}

// HandleError 处理错误
func (eh *ErrorHandler) HandleError(err *WASMError) error {
	// 更新统计
	eh.updateStats(err)

	// 调用错误回调
	for _, callback := range eh.callbacks {
		if callbackErr := callback(err); callbackErr != nil {
			// 回调执行失败，记录但不影响主流程
			continue
		}
	}

	return nil
}

// updateStats 更新错误统计
func (eh *ErrorHandler) updateStats(err *WASMError) {
	eh.stats.TotalErrors++
	eh.stats.ErrorsByType[err.Type]++
	eh.stats.ErrorsBySeverity[err.Severity]++
	eh.stats.LastErrorTime = err.Timestamp

	if err.IsRecoverable() {
		eh.stats.RecoverableErrors++
	}
}

// RegisterCallback 注册错误回调
func (eh *ErrorHandler) RegisterCallback(callback ErrorCallback) {
	eh.callbacks = append(eh.callbacks, callback)
}

// GetStats 获取错误统计
func (eh *ErrorHandler) GetStats() *ErrorStats {
	return eh.stats
}

// ResetStats 重置错误统计
func (eh *ErrorHandler) ResetStats() {
	eh.stats = &ErrorStats{
		ErrorsByType:     make(map[ErrorType]uint64),
		ErrorsBySeverity: make(map[ErrorSeverity]uint64),
	}
}

// ==================== 错误工具函数 ====================

// IsWASMError 检查是否为WASM错误
func IsWASMError(err error) bool {
	_, ok := err.(*WASMError)
	return ok
}

// AsWASMError 转换为WASM错误
func AsWASMError(err error) (*WASMError, bool) {
	wasmErr, ok := err.(*WASMError)
	return wasmErr, ok
}

// WrapError 包装普通错误为WASM错误
func WrapError(err error, errorType ErrorType, code string, message string) *WASMError {
	return NewError(errorType, code, message).
		WithCause(err).
		Build()
}

// JoinErrors 合并多个错误
func JoinErrors(errors ...*WASMError) *WASMError {
	if len(errors) == 0 {
		return nil
	}

	if len(errors) == 1 {
		return errors[0]
	}

	// 选择最高严重性作为主错误
	mainError := errors[0]
	for _, err := range errors[1:] {
		if err.Severity > mainError.Severity {
			mainError = err
		}
	}

	// 添加其他错误作为上下文
	for i, err := range errors {
		if err != mainError {
			mainError.AddContext(fmt.Sprintf("relatedError_%d", i), err.Error())
		}
	}

	return mainError
}
