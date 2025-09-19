package errors

import (
	"context"
	"fmt"
	"strings"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// PermissionErrorHandler 权限错误处理器
type PermissionErrorHandler struct {
	logger log.Logger
}

// NewPermissionErrorHandler 创建权限错误处理器
func NewPermissionErrorHandler(logger log.Logger) ErrorHandler {
	return &PermissionErrorHandler{
		logger: logger,
	}
}

// CanHandle 检查是否能处理此类型的错误
func (peh *PermissionErrorHandler) CanHandle(err error) bool {
	if cliErr, ok := err.(*CLIError); ok {
		return cliErr.Type == PermissionError
	}

	// 检查错误消息
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "permission") ||
		strings.Contains(message, "unauthorized") ||
		strings.Contains(message, "access denied") ||
		strings.Contains(message, "权限") ||
		strings.Contains(message, "未授权")
}

// Handle 处理错误
func (peh *PermissionErrorHandler) Handle(ctx context.Context, err error) (*ErrorHandleResult, error) {
	peh.logger.Info("处理权限错误")

	var cliErr *CLIError
	if ce, ok := err.(*CLIError); ok {
		cliErr = ce
	} else {
		// 转换为CLI错误
		cliErr = &CLIError{
			Type:     PermissionError,
			Severity: ErrorSeverityLevel,
			Message:  err.Error(),
			Cause:    err,
		}
	}

	// 分析权限错误的具体原因
	specificCause := peh.analyzePermissionError(cliErr)

	// 构建处理结果
	result := &ErrorHandleResult{
		Handled:      true,
		UserMessage:  peh.buildUserMessage(specificCause),
		TechnicalMsg: cliErr.Error(),
		Severity:     ErrorSeverityLevel,
		ShouldRetry:  false,
		Metadata: map[string]interface{}{
			"permission_cause": specificCause,
			"handler":          "PermissionErrorHandler",
		},
	}

	// 添加具体的建议和动作
	result.Suggestions, result.Actions = peh.buildSuggestionsAndActions(specificCause)

	return result, nil
}

// GetHandlerInfo 获取处理器信息
func (peh *PermissionErrorHandler) GetHandlerInfo() ErrorHandlerInfo {
	return ErrorHandlerInfo{
		Name:           "PermissionErrorHandler",
		SupportedTypes: []ErrorType{PermissionError},
		Priority:       1,
		Description:    "处理权限相关错误",
	}
}

// analyzePermissionError 分析权限错误的具体原因
func (peh *PermissionErrorHandler) analyzePermissionError(cliErr *CLIError) string {
	message := strings.ToLower(cliErr.Message)

	if strings.Contains(message, "wallet") || strings.Contains(message, "钱包") {
		if strings.Contains(message, "unlock") || strings.Contains(message, "解锁") {
			return "wallet_locked"
		}
		if strings.Contains(message, "not found") || strings.Contains(message, "不存在") {
			return "wallet_not_found"
		}
		return "wallet_permission"
	}

	if strings.Contains(message, "user") || strings.Contains(message, "用户") {
		return "user_permission"
	}

	if strings.Contains(message, "admin") || strings.Contains(message, "管理员") {
		return "admin_required"
	}

	return "general_permission"
}

// buildUserMessage 构建用户消息
func (peh *PermissionErrorHandler) buildUserMessage(cause string) string {
	switch cause {
	case "wallet_locked":
		return "🔐 钱包已锁定，无法执行此操作。请先解锁钱包后重试。"
	case "wallet_not_found":
		return "💳 未找到可用的钱包。请先创建或导入钱包。"
	case "wallet_permission":
		return "🔒 您没有操作此钱包的权限。请确认钱包是否正确解锁。"
	case "user_permission":
		return "👤 您的用户权限不足以执行此操作。请联系管理员获取相应权限。"
	case "admin_required":
		return "👑 此操作需要管理员权限。请使用管理员账户登录。"
	default:
		return "🚫 权限不足，无法执行此操作。请检查您的权限设置。"
	}
}

// buildSuggestionsAndActions 构建建议和动作
func (peh *PermissionErrorHandler) buildSuggestionsAndActions(cause string) ([]string, []ErrorAction) {
	switch cause {
	case "wallet_locked":
		return []string{
				"解锁需要的钱包",
				"检查钱包密码是否正确",
				"确认钱包文件未损坏",
			}, []ErrorAction{
				{
					ID:          "unlock_wallet",
					Title:       "解锁钱包",
					Description: "打开钱包解锁界面",
					Handler: func(ctx context.Context) error {
						// 这里应该调用钱包解锁功能
						return fmt.Errorf("钱包解锁功能需要集成")
					},
				},
			}

	case "wallet_not_found":
		return []string{
				"创建新的钱包",
				"导入现有钱包",
				"检查钱包文件路径",
			}, []ErrorAction{
				{
					ID:          "create_wallet",
					Title:       "创建钱包",
					Description: "创建一个新的钱包",
					Handler: func(ctx context.Context) error {
						return fmt.Errorf("钱包创建功能需要集成")
					},
				},
				{
					ID:          "import_wallet",
					Title:       "导入钱包",
					Description: "从私钥或文件导入钱包",
					Handler: func(ctx context.Context) error {
						return fmt.Errorf("钱包导入功能需要集成")
					},
				},
			}

	default:
		return []string{
			"检查用户权限设置",
			"确认账户状态正常",
			"联系系统管理员",
		}, []ErrorAction{}
	}
}

// NetworkErrorHandler 网络错误处理器
type NetworkErrorHandler struct {
	logger log.Logger
}

// NewNetworkErrorHandler 创建网络错误处理器
func NewNetworkErrorHandler(logger log.Logger) ErrorHandler {
	return &NetworkErrorHandler{
		logger: logger,
	}
}

// CanHandle 检查是否能处理此类型的错误
func (neh *NetworkErrorHandler) CanHandle(err error) bool {
	if cliErr, ok := err.(*CLIError); ok {
		return cliErr.Type == NetworkError
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "connection") ||
		strings.Contains(message, "network") ||
		strings.Contains(message, "timeout") ||
		strings.Contains(message, "unreachable") ||
		strings.Contains(message, "连接") ||
		strings.Contains(message, "网络") ||
		strings.Contains(message, "超时")
}

// Handle 处理错误
func (neh *NetworkErrorHandler) Handle(ctx context.Context, err error) (*ErrorHandleResult, error) {
	neh.logger.Info("处理网络错误")

	var cliErr *CLIError
	if ce, ok := err.(*CLIError); ok {
		cliErr = ce
	} else {
		cliErr = &CLIError{
			Type:     NetworkError,
			Severity: WarningSeverity,
			Message:  err.Error(),
			Cause:    err,
		}
	}

	// 分析网络错误类型
	networkIssue := neh.analyzeNetworkError(cliErr)

	result := &ErrorHandleResult{
		Handled:      true,
		UserMessage:  neh.buildNetworkMessage(networkIssue),
		TechnicalMsg: cliErr.Error(),
		Severity:     WarningSeverity,
		ShouldRetry:  true,
		Metadata: map[string]interface{}{
			"network_issue": networkIssue,
			"handler":       "NetworkErrorHandler",
		},
	}

	result.Suggestions, result.Actions = neh.buildNetworkSuggestionsAndActions(networkIssue)

	return result, nil
}

// GetHandlerInfo 获取处理器信息
func (neh *NetworkErrorHandler) GetHandlerInfo() ErrorHandlerInfo {
	return ErrorHandlerInfo{
		Name:           "NetworkErrorHandler",
		SupportedTypes: []ErrorType{NetworkError},
		Priority:       1,
		Description:    "处理网络连接相关错误",
	}
}

// analyzeNetworkError 分析网络错误类型
func (neh *NetworkErrorHandler) analyzeNetworkError(cliErr *CLIError) string {
	message := strings.ToLower(cliErr.Message)

	if strings.Contains(message, "timeout") || strings.Contains(message, "超时") {
		return "timeout"
	}

	if strings.Contains(message, "connection refused") || strings.Contains(message, "拒绝连接") {
		return "connection_refused"
	}

	if strings.Contains(message, "unreachable") || strings.Contains(message, "不可达") {
		return "unreachable"
	}

	if strings.Contains(message, "dns") || strings.Contains(message, "域名") {
		return "dns_error"
	}

	return "general_network"
}

// buildNetworkMessage 构建网络消息
func (neh *NetworkErrorHandler) buildNetworkMessage(issue string) string {
	switch issue {
	case "timeout":
		return "⏰ 网络请求超时，服务器响应时间过长。"
	case "connection_refused":
		return "🚫 连接被拒绝，服务器可能未运行或端口被占用。"
	case "unreachable":
		return "🌐 目标服务器不可达，请检查网络连接和服务器地址。"
	case "dns_error":
		return "🔍 域名解析失败，请检查DNS设置或使用IP地址。"
	default:
		return "📡 网络连接出现问题，请检查网络设置。"
	}
}

// buildNetworkSuggestionsAndActions 构建网络建议和动作
func (neh *NetworkErrorHandler) buildNetworkSuggestionsAndActions(issue string) ([]string, []ErrorAction) {
	suggestions := []string{
		"检查网络连接状态",
		"确认服务器地址和端口正确",
		"稍后重试操作",
	}

	actions := []ErrorAction{
		{
			ID:          "retry_connection",
			Title:       "重试连接",
			Description: "立即重试网络连接",
			Handler: func(ctx context.Context) error {
				// 这里可以实现重连逻辑
				return fmt.Errorf("重连功能需要集成")
			},
		},
		{
			ID:          "check_network",
			Title:       "网络诊断",
			Description: "检查网络连接状态",
			Handler: func(ctx context.Context) error {
				// 这里可以实现网络诊断
				return fmt.Errorf("网络诊断功能需要集成")
			},
		},
	}

	switch issue {
	case "timeout":
		suggestions = append(suggestions, "增加超时时间设置", "检查服务器负载状况")
	case "connection_refused":
		suggestions = append(suggestions, "确认服务器正在运行", "检查防火墙设置")
	case "unreachable":
		suggestions = append(suggestions, "检查路由设置", "确认目标地址可达")
	case "dns_error":
		suggestions = append(suggestions, "更换DNS服务器", "使用IP地址直接连接")
	}

	return suggestions, actions
}

// ValidationErrorHandler 验证错误处理器
type ValidationErrorHandler struct {
	logger log.Logger
}

// NewValidationErrorHandler 创建验证错误处理器
func NewValidationErrorHandler(logger log.Logger) ErrorHandler {
	return &ValidationErrorHandler{
		logger: logger,
	}
}

// CanHandle 检查是否能处理此类型的错误
func (veh *ValidationErrorHandler) CanHandle(err error) bool {
	if cliErr, ok := err.(*CLIError); ok {
		return cliErr.Type == ValidationError
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "invalid") ||
		strings.Contains(message, "validation") ||
		strings.Contains(message, "format") ||
		strings.Contains(message, "验证") ||
		strings.Contains(message, "格式") ||
		strings.Contains(message, "无效")
}

// Handle 处理错误
func (veh *ValidationErrorHandler) Handle(ctx context.Context, err error) (*ErrorHandleResult, error) {
	veh.logger.Info("处理验证错误")

	var cliErr *CLIError
	if ce, ok := err.(*CLIError); ok {
		cliErr = ce
	} else {
		cliErr = &CLIError{
			Type:     ValidationError,
			Severity: WarningSeverity,
			Message:  err.Error(),
			Cause:    err,
		}
	}

	validationIssue := veh.analyzeValidationError(cliErr)

	result := &ErrorHandleResult{
		Handled:      true,
		UserMessage:  veh.buildValidationMessage(validationIssue),
		TechnicalMsg: cliErr.Error(),
		Severity:     WarningSeverity,
		ShouldRetry:  false,
		Metadata: map[string]interface{}{
			"validation_issue": validationIssue,
			"handler":          "ValidationErrorHandler",
		},
	}

	result.Suggestions, result.Actions = veh.buildValidationSuggestionsAndActions(validationIssue, cliErr)

	return result, nil
}

// GetHandlerInfo 获取处理器信息
func (veh *ValidationErrorHandler) GetHandlerInfo() ErrorHandlerInfo {
	return ErrorHandlerInfo{
		Name:           "ValidationErrorHandler",
		SupportedTypes: []ErrorType{ValidationError},
		Priority:       1,
		Description:    "处理数据验证相关错误",
	}
}

// analyzeValidationError 分析验证错误类型
func (veh *ValidationErrorHandler) analyzeValidationError(cliErr *CLIError) string {
	message := strings.ToLower(cliErr.Message)

	if strings.Contains(message, "address") || strings.Contains(message, "地址") {
		return "invalid_address"
	}

	if strings.Contains(message, "amount") || strings.Contains(message, "金额") {
		return "invalid_amount"
	}

	if strings.Contains(message, "password") || strings.Contains(message, "密码") {
		return "invalid_password"
	}

	if strings.Contains(message, "private key") || strings.Contains(message, "私钥") {
		return "invalid_private_key"
	}

	if strings.Contains(message, "format") || strings.Contains(message, "格式") {
		return "invalid_format"
	}

	return "general_validation"
}

// buildValidationMessage 构建验证消息
func (veh *ValidationErrorHandler) buildValidationMessage(issue string) string {
	switch issue {
	case "invalid_address":
		return "📮 地址格式不正确，请检查地址格式和长度。"
	case "invalid_amount":
		return "💰 金额格式不正确，请输入有效的数字金额。"
	case "invalid_password":
		return "🔑 密码格式不符合要求，请检查密码长度和复杂度。"
	case "invalid_private_key":
		return "🔐 私钥格式不正确，请确认私钥为64位十六进制字符串。"
	case "invalid_format":
		return "📝 数据格式不正确，请按照指定格式输入。"
	default:
		return "⚠️ 输入数据验证失败，请检查输入格式。"
	}
}

// buildValidationSuggestionsAndActions 构建验证建议和动作
func (veh *ValidationErrorHandler) buildValidationSuggestionsAndActions(issue string, cliErr *CLIError) ([]string, []ErrorAction) {
	suggestions := []string{
		"仔细检查输入格式",
		"参考帮助文档了解正确格式",
	}

	actions := []ErrorAction{
		{
			ID:          "show_format_help",
			Title:       "查看格式帮助",
			Description: "显示正确的输入格式说明",
			Handler: func(ctx context.Context) error {
				// 这里可以显示格式帮助
				return fmt.Errorf("格式帮助功能需要集成")
			},
		},
	}

	switch issue {
	case "invalid_address":
		suggestions = append(suggestions,
			"地址应以大写字母开头",
			"确认地址长度在20-50位之间",
			"检查地址是否包含无效字符")
	case "invalid_amount":
		suggestions = append(suggestions,
			"金额必须大于0",
			"小数位数不能超过8位",
			"不能包含非数字字符")
	case "invalid_password":
		suggestions = append(suggestions,
			"密码长度至少8位",
			"包含大小写字母、数字和特殊字符",
			"避免使用常见密码")
	case "invalid_private_key":
		suggestions = append(suggestions,
			"私钥必须是64位十六进制字符串",
			"只能包含0-9和a-f字符",
			"检查是否有额外的空格或换行符")
	}

	return suggestions, actions
}

// registerDefaultHandlers 在错误管理器中注册默认处理器
func (em *errorManager) registerDefaultHandlers() {
	// 注册权限错误处理器
	if err := em.RegisterHandler(NewPermissionErrorHandler(em.logger)); err != nil {
		em.logger.Error(fmt.Sprintf("注册权限错误处理器失败: %v", err))
	}

	// 注册网络错误处理器
	if err := em.RegisterHandler(NewNetworkErrorHandler(em.logger)); err != nil {
		em.logger.Error(fmt.Sprintf("注册网络错误处理器失败: %v", err))
	}

	// 注册验证错误处理器
	if err := em.RegisterHandler(NewValidationErrorHandler(em.logger)); err != nil {
		em.logger.Error(fmt.Sprintf("注册验证错误处理器失败: %v", err))
	}
}
