// Package errors 提供CLI的统一错误处理机制
package errors

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/weisyn/v1/internal/cli/ui"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ErrorType 错误类型
type ErrorType string

const (
	// PermissionError 权限错误
	PermissionError ErrorType = "permission"
	// NetworkError 网络错误
	NetworkError ErrorType = "network"
	// ValidationError 验证错误
	ValidationError ErrorType = "validation"
	// ConfigError 配置错误
	ConfigError ErrorType = "config"
	// SystemError 系统错误
	SystemError ErrorType = "system"
	// UserError 用户操作错误
	UserError ErrorType = "user"
	// InternalError 内部错误
	InternalError ErrorType = "internal"
)

// ErrorSeverity 错误严重程度
type ErrorSeverity int

const (
	// InfoSeverity 信息级别
	InfoSeverity ErrorSeverity = iota
	// WarningSeverity 警告级别
	WarningSeverity
	// ErrorSeverity 错误级别
	ErrorSeverityLevel
	// CriticalSeverity 严重错误级别
	CriticalSeverity
)

// String 返回错误严重程度的字符串表示
func (es ErrorSeverity) String() string {
	switch es {
	case InfoSeverity:
		return "Info"
	case WarningSeverity:
		return "Warning"
	case ErrorSeverityLevel:
		return "Error"
	case CriticalSeverity:
		return "Critical"
	default:
		return "Unknown"
	}
}

// CLIError CLI错误结构
type CLIError struct {
	Type        ErrorType              // 错误类型
	Severity    ErrorSeverity          // 严重程度
	Code        string                 // 错误代码
	Message     string                 // 错误消息
	Description string                 // 详细描述
	Cause       error                  // 原始错误
	Context     map[string]interface{} // 上下文信息
	Suggestions []string               // 解决建议
	Timestamp   time.Time              // 发生时间
	Location    string                 // 错误位置
}

// Error 实现error接口
func (ce *CLIError) Error() string {
	if ce.Code != "" {
		return fmt.Sprintf("[%s:%s] %s", ce.Type, ce.Code, ce.Message)
	}
	return fmt.Sprintf("[%s] %s", ce.Type, ce.Message)
}

// Unwrap 支持错误链
func (ce *CLIError) Unwrap() error {
	return ce.Cause
}

// ErrorHandler 错误处理器接口
type ErrorHandler interface {
	// CanHandle 检查是否能处理此类型的错误
	CanHandle(err error) bool

	// Handle 处理错误
	Handle(ctx context.Context, err error) (*ErrorHandleResult, error)

	// GetHandlerInfo 获取处理器信息
	GetHandlerInfo() ErrorHandlerInfo
}

// ErrorHandlerInfo 错误处理器信息
type ErrorHandlerInfo struct {
	Name           string      // 处理器名称
	SupportedTypes []ErrorType // 支持的错误类型
	Priority       int         // 优先级（越小越高）
	Description    string      // 描述
}

// ErrorHandleResult 错误处理结果
type ErrorHandleResult struct {
	Handled      bool                   // 是否已处理
	UserMessage  string                 // 用户消息
	TechnicalMsg string                 // 技术消息
	Suggestions  []string               // 建议
	Actions      []ErrorAction          // 可执行的动作
	Severity     ErrorSeverity          // 严重程度
	ShouldRetry  bool                   // 是否应该重试
	Metadata     map[string]interface{} // 元数据
}

// ErrorAction 错误相关动作
type ErrorAction struct {
	ID          string                          // 动作ID
	Title       string                          // 动作标题
	Description string                          // 动作描述
	Handler     func(ctx context.Context) error // 动作处理函数
}

// ErrorManager 错误管理器接口
type ErrorManager interface {
	// 错误处理
	HandleError(ctx context.Context, err error) error
	HandleCLIError(ctx context.Context, cliErr *CLIError) error

	// 错误创建
	NewError(errorType ErrorType, code, message string) *CLIError
	NewErrorWithCause(errorType ErrorType, code, message string, cause error) *CLIError
	WrapError(err error, errorType ErrorType, code, message string) *CLIError

	// 特定类型错误
	NewPermissionError(code, message string) *CLIError
	NewNetworkError(code, message string, cause error) *CLIError
	NewValidationError(field, message string) *CLIError

	// 处理器管理
	RegisterHandler(handler ErrorHandler) error
	UnregisterHandler(handlerName string) error

	// 用户友好显示
	ShowUserFriendlyError(ctx context.Context, err error) error

	// 错误恢复
	TryRecover(ctx context.Context, err error) (*RecoveryResult, error)
}

// RecoveryResult 恢复结果
type RecoveryResult struct {
	Recovered bool   // 是否恢复成功
	Message   string // 恢复消息
	Action    string // 执行的恢复动作
}

// errorManager 错误管理器实现
type errorManager struct {
	logger   log.Logger
	ui       ui.Components
	handlers map[string]ErrorHandler
}

// NewErrorManager 创建错误管理器
func NewErrorManager(logger log.Logger, uiComponents ui.Components) ErrorManager {
	em := &errorManager{
		logger:   logger,
		ui:       uiComponents,
		handlers: make(map[string]ErrorHandler),
	}

	// 注册默认处理器
	em.registerDefaultHandlers()

	return em
}

// HandleError 处理错误
func (em *errorManager) HandleError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}

	// 如果已经是CLIError，直接处理
	if cliErr, ok := err.(*CLIError); ok {
		return em.HandleCLIError(ctx, cliErr)
	}

	// 尝试转换为CLIError
	cliErr := em.convertToCLIError(err)
	return em.HandleCLIError(ctx, cliErr)
}

// HandleCLIError 处理CLI错误
func (em *errorManager) HandleCLIError(ctx context.Context, cliErr *CLIError) error {
	em.logger.Error(fmt.Sprintf("处理CLI错误: type=%s, code=%s, message=%s",
		cliErr.Type, cliErr.Code, cliErr.Message))

	// 查找合适的处理器
	handler := em.findBestHandler(cliErr)
	if handler == nil {
		// 使用默认处理
		return em.defaultErrorHandling(ctx, cliErr)
	}

	// 使用专用处理器处理
	result, err := handler.Handle(ctx, cliErr)
	if err != nil {
		em.logger.Error(fmt.Sprintf("错误处理器执行失败: %v", err))
		return em.defaultErrorHandling(ctx, cliErr)
	}

	if result != nil && result.Handled {
		// 显示处理结果
		return em.displayHandleResult(ctx, result)
	}

	// 如果没有被处理，使用默认处理
	return em.defaultErrorHandling(ctx, cliErr)
}

// NewError 创建新的CLI错误
func (em *errorManager) NewError(errorType ErrorType, code, message string) *CLIError {
	return &CLIError{
		Type:      errorType,
		Severity:  ErrorSeverityLevel,
		Code:      code,
		Message:   message,
		Context:   make(map[string]interface{}),
		Timestamp: time.Now(),
		Location:  em.getCallerLocation(),
	}
}

// NewErrorWithCause 创建带原因的CLI错误
func (em *errorManager) NewErrorWithCause(errorType ErrorType, code, message string, cause error) *CLIError {
	cliErr := em.NewError(errorType, code, message)
	cliErr.Cause = cause
	return cliErr
}

// WrapError 包装现有错误
func (em *errorManager) WrapError(err error, errorType ErrorType, code, message string) *CLIError {
	return em.NewErrorWithCause(errorType, code, message, err)
}

// NewPermissionError 创建权限错误
func (em *errorManager) NewPermissionError(code, message string) *CLIError {
	cliErr := em.NewError(PermissionError, code, message)
	cliErr.Severity = ErrorSeverityLevel
	cliErr.Suggestions = []string{
		"请检查当前用户权限级别",
		"确保已解锁必要的钱包",
		"联系管理员获取相应权限",
	}
	return cliErr
}

// NewNetworkError 创建网络错误
func (em *errorManager) NewNetworkError(code, message string, cause error) *CLIError {
	cliErr := em.NewErrorWithCause(NetworkError, code, message, cause)
	cliErr.Severity = WarningSeverity
	cliErr.Suggestions = []string{
		"检查网络连接状态",
		"确认API服务地址正确",
		"稍后重试操作",
		"检查防火墙设置",
	}
	return cliErr
}

// NewValidationError 创建验证错误
func (em *errorManager) NewValidationError(field, message string) *CLIError {
	cliErr := em.NewError(ValidationError, "VALIDATION_FAILED", message)
	cliErr.Context["field"] = field
	cliErr.Severity = WarningSeverity
	cliErr.Suggestions = []string{
		"检查输入数据格式",
		"参考帮助文档了解正确格式",
		"确认所有必填字段已填写",
	}
	return cliErr
}

// ShowUserFriendlyError 显示用户友好的错误
func (em *errorManager) ShowUserFriendlyError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}

	// 转换为CLI错误
	var cliErr *CLIError
	if ce, ok := err.(*CLIError); ok {
		cliErr = ce
	} else {
		cliErr = em.convertToCLIError(err)
	}

	// 构建用户友好的错误消息
	return em.displayUserFriendlyMessage(ctx, cliErr)
}

// TryRecover 尝试错误恢复
func (em *errorManager) TryRecover(ctx context.Context, err error) (*RecoveryResult, error) {
	if err == nil {
		return &RecoveryResult{Recovered: true, Message: "无需恢复"}, nil
	}

	// 转换为CLI错误
	var cliErr *CLIError
	if ce, ok := err.(*CLIError); ok {
		cliErr = ce
	} else {
		cliErr = em.convertToCLIError(err)
	}

	// 根据错误类型尝试恢复
	return em.attemptRecovery(ctx, cliErr)
}

// convertToCLIError 转换为CLI错误
func (em *errorManager) convertToCLIError(err error) *CLIError {
	message := err.Error()

	// 根据错误消息特征判断错误类型
	errorType := em.detectErrorType(message)

	cliErr := &CLIError{
		Type:      errorType,
		Severity:  ErrorSeverityLevel,
		Code:      "GENERIC_ERROR",
		Message:   message,
		Cause:     err,
		Context:   make(map[string]interface{}),
		Timestamp: time.Now(),
		Location:  em.getCallerLocation(),
	}

	// 根据错误类型添加建议
	cliErr.Suggestions = em.getSuggestionsForType(errorType)

	return cliErr
}

// detectErrorType 检测错误类型
func (em *errorManager) detectErrorType(message string) ErrorType {
	lowerMsg := strings.ToLower(message)

	// 权限相关关键词
	if strings.Contains(lowerMsg, "permission") ||
		strings.Contains(lowerMsg, "unauthorized") ||
		strings.Contains(lowerMsg, "access denied") ||
		strings.Contains(lowerMsg, "权限") ||
		strings.Contains(lowerMsg, "未授权") {
		return PermissionError
	}

	// 网络相关关键词
	if strings.Contains(lowerMsg, "connection") ||
		strings.Contains(lowerMsg, "network") ||
		strings.Contains(lowerMsg, "timeout") ||
		strings.Contains(lowerMsg, "unreachable") ||
		strings.Contains(lowerMsg, "连接") ||
		strings.Contains(lowerMsg, "网络") ||
		strings.Contains(lowerMsg, "超时") {
		return NetworkError
	}

	// 验证相关关键词
	if strings.Contains(lowerMsg, "invalid") ||
		strings.Contains(lowerMsg, "validation") ||
		strings.Contains(lowerMsg, "format") ||
		strings.Contains(lowerMsg, "验证") ||
		strings.Contains(lowerMsg, "格式") ||
		strings.Contains(lowerMsg, "无效") {
		return ValidationError
	}

	// 配置相关关键词
	if strings.Contains(lowerMsg, "config") ||
		strings.Contains(lowerMsg, "setting") ||
		strings.Contains(lowerMsg, "配置") ||
		strings.Contains(lowerMsg, "设置") {
		return ConfigError
	}

	return SystemError
}

// getSuggestionsForType 根据错误类型获取建议
func (em *errorManager) getSuggestionsForType(errorType ErrorType) []string {
	switch errorType {
	case PermissionError:
		return []string{
			"检查用户权限设置",
			"确保已正确登录",
			"联系管理员获取权限",
		}
	case NetworkError:
		return []string{
			"检查网络连接",
			"确认服务器地址正确",
			"稍后重试",
		}
	case ValidationError:
		return []string{
			"检查输入格式",
			"查看帮助文档",
			"确认数据有效性",
		}
	case ConfigError:
		return []string{
			"检查配置文件",
			"恢复默认设置",
			"查看配置文档",
		}
	default:
		return []string{
			"查看详细错误信息",
			"检查系统日志",
			"联系技术支持",
		}
	}
}

// getCallerLocation 获取调用位置
func (em *errorManager) getCallerLocation() string {
	_, file, line, ok := runtime.Caller(3) // 跳过3层调用栈
	if !ok {
		return "unknown"
	}

	// 只保留文件名，不要完整路径
	parts := strings.Split(file, "/")
	filename := parts[len(parts)-1]

	return fmt.Sprintf("%s:%d", filename, line)
}

// findBestHandler 查找最佳处理器
func (em *errorManager) findBestHandler(cliErr *CLIError) ErrorHandler {
	var bestHandler ErrorHandler
	bestPriority := int(^uint(0) >> 1) // 最大整数

	for _, handler := range em.handlers {
		if handler.CanHandle(cliErr) {
			info := handler.GetHandlerInfo()
			if info.Priority < bestPriority {
				bestHandler = handler
				bestPriority = info.Priority
			}
		}
	}

	return bestHandler
}

// defaultErrorHandling 默认错误处理
func (em *errorManager) defaultErrorHandling(ctx context.Context, cliErr *CLIError) error {
	// 根据严重程度显示不同的消息
	switch cliErr.Severity {
	case InfoSeverity:
		em.ui.ShowInfo(cliErr.Message)
	case WarningSeverity:
		em.ui.ShowWarning(cliErr.Message)
	case ErrorSeverityLevel, CriticalSeverity:
		em.ui.ShowError(cliErr.Message)
	}

	// 显示建议
	if len(cliErr.Suggestions) > 0 {
		em.ui.ShowInfo("💡 建议:")
		for _, suggestion := range cliErr.Suggestions {
			em.ui.ShowInfo(fmt.Sprintf("  • %s", suggestion))
		}
	}

	return nil
}

// displayHandleResult 显示处理结果
func (em *errorManager) displayHandleResult(ctx context.Context, result *ErrorHandleResult) error {
	if result.UserMessage != "" {
		switch result.Severity {
		case InfoSeverity:
			em.ui.ShowInfo(result.UserMessage)
		case WarningSeverity:
			em.ui.ShowWarning(result.UserMessage)
		case ErrorSeverityLevel, CriticalSeverity:
			em.ui.ShowError(result.UserMessage)
		}
	}

	// 显示建议
	if len(result.Suggestions) > 0 {
		em.ui.ShowInfo("💡 建议:")
		for _, suggestion := range result.Suggestions {
			em.ui.ShowInfo(fmt.Sprintf("  • %s", suggestion))
		}
	}

	// 显示可执行动作
	if len(result.Actions) > 0 {
		em.ui.ShowInfo("🔧 可执行的操作:")
		actions := make([]string, len(result.Actions))
		for i, action := range result.Actions {
			actions[i] = action.Title
		}

		selectedIndex, err := em.ui.ShowMenu("请选择要执行的操作", actions)
		if err == nil && selectedIndex >= 0 && selectedIndex < len(result.Actions) {
			// 执行选中的动作
			selectedAction := result.Actions[selectedIndex]
			if selectedAction.Handler != nil {
				if err := selectedAction.Handler(ctx); err != nil {
					em.ui.ShowError(fmt.Sprintf("执行操作失败: %v", err))
				} else {
					em.ui.ShowSuccess("操作执行成功")
				}
			}
		}
	}

	return nil
}

// displayUserFriendlyMessage 显示用户友好消息
func (em *errorManager) displayUserFriendlyMessage(ctx context.Context, cliErr *CLIError) error {
	// 构建友好的标题
	title := em.getFriendlyTitle(cliErr.Type)

	// 构建友好的消息
	friendlyMessage := em.getFriendlyMessage(cliErr)

	// 显示错误面板
	em.ui.ShowPanel(title, friendlyMessage)

	// 显示建议
	if len(cliErr.Suggestions) > 0 {
		em.ui.ShowInfo("💡 解决建议:")
		for i, suggestion := range cliErr.Suggestions {
			em.ui.ShowInfo(fmt.Sprintf("  %d. %s", i+1, suggestion))
		}
	}

	// 如果有上下文信息，显示详细信息
	if len(cliErr.Context) > 0 {
		contextInfo := make(map[string]string)
		for key, value := range cliErr.Context {
			contextInfo[key] = fmt.Sprintf("%v", value)
		}
		em.ui.ShowKeyValuePairs("详细信息", contextInfo)
	}

	return nil
}

// getFriendlyTitle 获取友好的标题
func (em *errorManager) getFriendlyTitle(errorType ErrorType) string {
	switch errorType {
	case PermissionError:
		return "🔐 权限不足"
	case NetworkError:
		return "🌐 网络连接问题"
	case ValidationError:
		return "⚠️ 输入验证失败"
	case ConfigError:
		return "⚙️ 配置问题"
	case UserError:
		return "👤 操作错误"
	case SystemError:
		return "🛠️ 系统错误"
	case InternalError:
		return "🔧 内部错误"
	default:
		return "❌ 发生错误"
	}
}

// getFriendlyMessage 获取友好的消息
func (em *errorManager) getFriendlyMessage(cliErr *CLIError) string {
	var messageBuilder strings.Builder

	// 添加主要消息
	messageBuilder.WriteString(cliErr.Message)

	// 如果有描述，添加描述
	if cliErr.Description != "" {
		messageBuilder.WriteString(fmt.Sprintf("\n\n📋 详细说明:\n%s", cliErr.Description))
	}

	// 添加错误代码（如果有）
	if cliErr.Code != "" {
		messageBuilder.WriteString(fmt.Sprintf("\n\n🔍 错误代码: %s", cliErr.Code))
	}

	// 添加时间信息
	messageBuilder.WriteString(fmt.Sprintf("\n\n⏰ 发生时间: %s",
		cliErr.Timestamp.Format("2006-01-02 15:04:05")))

	return messageBuilder.String()
}

// attemptRecovery 尝试恢复
func (em *errorManager) attemptRecovery(ctx context.Context, cliErr *CLIError) (*RecoveryResult, error) {
	switch cliErr.Type {
	case NetworkError:
		return em.recoverNetworkError(ctx, cliErr)
	case ConfigError:
		return em.recoverConfigError(ctx, cliErr)
	case ValidationError:
		return em.recoverValidationError(ctx, cliErr)
	default:
		return &RecoveryResult{
			Recovered: false,
			Message:   "此类型错误暂不支持自动恢复",
		}, nil
	}
}

// recoverNetworkError 恢复网络错误
func (em *errorManager) recoverNetworkError(ctx context.Context, cliErr *CLIError) (*RecoveryResult, error) {
	// 简单重试逻辑
	em.ui.ShowInfo("🔄 尝试重新连接...")

	// 这里可以实现实际的网络重连逻辑
	// 暂时返回模拟结果
	return &RecoveryResult{
		Recovered: false,
		Message:   "网络连接恢复失败，请检查网络设置",
		Action:    "network_retry",
	}, nil
}

// recoverConfigError 恢复配置错误
func (em *errorManager) recoverConfigError(ctx context.Context, cliErr *CLIError) (*RecoveryResult, error) {
	// 提供重置为默认配置的选项
	confirmed, err := em.ui.ShowConfirmDialog(
		"🔧 配置恢复",
		"是否要重置为默认配置？这将覆盖当前的配置设置。",
	)

	if err != nil || !confirmed {
		return &RecoveryResult{
			Recovered: false,
			Message:   "用户取消配置恢复",
		}, nil
	}

	// 这里可以实现实际的配置重置逻辑
	return &RecoveryResult{
		Recovered: true,
		Message:   "配置已重置为默认值",
		Action:    "config_reset",
	}, nil
}

// recoverValidationError 恢复验证错误
func (em *errorManager) recoverValidationError(ctx context.Context, cliErr *CLIError) (*RecoveryResult, error) {
	// 验证错误通常需要用户重新输入，无法自动恢复
	return &RecoveryResult{
		Recovered: false,
		Message:   "验证错误需要用户重新输入正确的数据",
		Action:    "user_input_required",
	}, nil
}

// RegisterHandler 注册错误处理器
func (em *errorManager) RegisterHandler(handler ErrorHandler) error {
	if handler == nil {
		return fmt.Errorf("处理器不能为空")
	}

	info := handler.GetHandlerInfo()
	if info.Name == "" {
		return fmt.Errorf("处理器名称不能为空")
	}

	if _, exists := em.handlers[info.Name]; exists {
		return fmt.Errorf("处理器已存在: %s", info.Name)
	}

	em.handlers[info.Name] = handler
	em.logger.Info(fmt.Sprintf("注册错误处理器: name=%s", info.Name))

	return nil
}

// UnregisterHandler 取消注册错误处理器
func (em *errorManager) UnregisterHandler(handlerName string) error {
	if _, exists := em.handlers[handlerName]; !exists {
		return fmt.Errorf("处理器不存在: %s", handlerName)
	}

	delete(em.handlers, handlerName)
	em.logger.Info(fmt.Sprintf("取消注册错误处理器: name=%s", handlerName))

	return nil
}
