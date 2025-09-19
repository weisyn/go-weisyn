// Package security 提供CLI的安全功能实现
package security

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/weisyn/v1/internal/cli/ui"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// SecurityLevel 安全级别
type SecurityLevel int

const (
	// LowSecurity 低安全级别
	LowSecurity SecurityLevel = iota
	// MediumSecurity 中等安全级别
	MediumSecurity
	// HighSecurity 高安全级别
	HighSecurity
	// CriticalSecurity 关键安全级别
	CriticalSecurity
)

// String 返回安全级别的字符串表示
func (sl SecurityLevel) String() string {
	switch sl {
	case LowSecurity:
		return "Low"
	case MediumSecurity:
		return "Medium"
	case HighSecurity:
		return "High"
	case CriticalSecurity:
		return "Critical"
	default:
		return "Unknown"
	}
}

// OperationType 操作类型
type OperationType string

const (
	// WalletOperation 钱包相关操作
	WalletOperation OperationType = "wallet"
	// TransferOperation 转账操作
	TransferOperation OperationType = "transfer"
	// ConsensusOperation 共识操作
	ConsensusOperation OperationType = "consensus"
	// SystemOperation 系统操作
	SystemOperation OperationType = "system"
	// SettingsOperation 设置操作
	SettingsOperation OperationType = "settings"
)

// SecurityContext 安全上下文
type SecurityContext struct {
	UserID        string                 // 用户标识
	SessionID     string                 // 会话标识
	Operation     OperationType          // 操作类型
	SecurityLevel SecurityLevel          // 安全级别
	Timestamp     time.Time              // 时间戳
	IPAddress     string                 // IP地址
	UserAgent     string                 // 用户代理
	Metadata      map[string]interface{} // 额外元数据
}

// ConfirmationRequest 确认请求
type ConfirmationRequest struct {
	Title       string        // 标题
	Message     string        // 消息内容
	Operation   OperationType // 操作类型
	Level       SecurityLevel // 安全级别
	Details     []string      // 详细信息
	Warnings    []string      // 警告信息
	Timeout     time.Duration // 超时时间
	RequireAuth bool          // 是否需要身份验证
}

// SecurityPolicy 安全策略
type SecurityPolicy struct {
	// 确认策略
	RequireConfirmation   map[OperationType]bool // 需要确认的操作类型
	DoubleConfirmation    map[OperationType]bool // 需要双重确认的操作类型
	RequireAuthentication map[OperationType]bool // 需要身份验证的操作类型

	// 敏感信息保护
	MaskSensitiveData      bool     // 是否掩码敏感数据
	SensitiveDataPatterns  []string // 敏感数据模式
	LogSensitiveOperations bool     // 是否记录敏感操作

	// 超时设置
	ConfirmationTimeout   time.Duration // 确认超时时间
	AuthenticationTimeout time.Duration // 身份验证超时时间
	SessionTimeout        time.Duration // 会话超时时间

	// 安全提示
	ShowSecurityWarnings bool // 是否显示安全警告
	ShowOperationSummary bool // 是否显示操作摘要
	WarnUnsafeOperations bool // 是否警告不安全操作
}

// SecurityManager 安全管理器接口
type SecurityManager interface {
	// 操作确认
	RequestConfirmation(ctx context.Context, request ConfirmationRequest) (bool, error)
	RequestDoubleConfirmation(ctx context.Context, request ConfirmationRequest) (bool, error)

	// 身份验证
	AuthenticateUser(ctx context.Context, securityContext SecurityContext) (bool, error)
	ValidatePassword(ctx context.Context, password string) (bool, error)

	// 敏感信息保护
	MaskSensitiveData(data string) string
	ValidateSensitiveOperation(ctx context.Context, operation OperationType) error

	// 安全提示和警告
	ShowSecurityWarning(ctx context.Context, warning SecurityWarning) error
	ShowOperationSummary(ctx context.Context, summary OperationSummary) error

	// 策略管理
	SetSecurityPolicy(policy SecurityPolicy)
	GetSecurityPolicy() SecurityPolicy

	// 审计日志
	LogSecurityEvent(ctx context.Context, event SecurityEvent)
	GetSecurityAuditLog(ctx context.Context, limit int) ([]SecurityEvent, error)
}

// SecurityWarning 安全警告
type SecurityWarning struct {
	Level       SecurityLevel // 警告级别
	Title       string        // 警告标题
	Message     string        // 警告消息
	Suggestions []string      // 安全建议
	LearnMore   string        // 了解更多链接
}

// OperationSummary 操作摘要
type OperationSummary struct {
	Operation   OperationType          // 操作类型
	Description string                 // 操作描述
	Impact      string                 // 影响说明
	Parameters  map[string]interface{} // 操作参数
	Warnings    []string               // 相关警告
	Timestamp   time.Time              // 时间戳
}

// SecurityEvent 安全事件
type SecurityEvent struct {
	ID        string                 // 事件ID
	Timestamp time.Time              // 时间戳
	UserID    string                 // 用户ID
	SessionID string                 // 会话ID
	EventType string                 // 事件类型
	Operation OperationType          // 操作类型
	Level     SecurityLevel          // 安全级别
	Success   bool                   // 是否成功
	Message   string                 // 事件消息
	Metadata  map[string]interface{} // 事件元数据
	IPAddress string                 // IP地址
}

// securityManager 安全管理器实现
type securityManager struct {
	logger log.Logger
	ui     ui.Components
	policy SecurityPolicy

	// 会话管理
	activeSessions map[string]*SecurityContext
	eventLog       []SecurityEvent

	// 敏感数据模式
	sensitivePatterns []*regexp.Regexp
}

// NewSecurityManager 创建安全管理器
func NewSecurityManager(logger log.Logger, uiComponents ui.Components) SecurityManager {
	sm := &securityManager{
		logger:         logger,
		ui:             uiComponents,
		policy:         getDefaultSecurityPolicy(),
		activeSessions: make(map[string]*SecurityContext),
		eventLog:       make([]SecurityEvent, 0),
	}

	// 编译敏感数据模式
	sm.compileSensitivePatterns()

	return sm
}

// RequestConfirmation 请求操作确认
func (sm *securityManager) RequestConfirmation(ctx context.Context, request ConfirmationRequest) (bool, error) {
	sm.logger.Info(fmt.Sprintf("请求操作确认: operation=%s, level=%s", request.Operation, request.Level.String()))

	// 检查是否需要确认
	if !sm.policy.RequireConfirmation[request.Operation] {
		return true, nil // 不需要确认，直接通过
	}

	// 显示确认对话框
	return sm.showConfirmationDialog(ctx, request)
}

// RequestDoubleConfirmation 请求双重确认
func (sm *securityManager) RequestDoubleConfirmation(ctx context.Context, request ConfirmationRequest) (bool, error) {
	sm.logger.Info(fmt.Sprintf("请求双重确认: operation=%s, level=%s", request.Operation, request.Level.String()))

	// 检查是否需要双重确认
	if !sm.policy.DoubleConfirmation[request.Operation] {
		return sm.RequestConfirmation(ctx, request)
	}

	// 第一次确认
	firstConfirmed, err := sm.showConfirmationDialog(ctx, request)
	if err != nil || !firstConfirmed {
		return false, err
	}

	// 显示额外的安全警告
	sm.showDoubleConfirmationWarning(request)

	// 第二次确认
	secondRequest := request
	secondRequest.Title = "⚠️ 二次确认"
	secondRequest.Message = fmt.Sprintf("您即将执行高风险操作：%s\n\n请再次确认您的选择。", request.Message)

	secondConfirmed, err := sm.showConfirmationDialog(ctx, secondRequest)
	if err != nil {
		return false, err
	}

	// 记录安全事件
	sm.LogSecurityEvent(ctx, SecurityEvent{
		ID:        generateEventID(),
		Timestamp: time.Now(),
		EventType: "double_confirmation",
		Operation: request.Operation,
		Level:     request.Level,
		Success:   secondConfirmed,
		Message:   fmt.Sprintf("双重确认操作: %s", request.Operation),
	})

	return secondConfirmed, nil
}

// showConfirmationDialog 显示确认对话框
func (sm *securityManager) showConfirmationDialog(ctx context.Context, request ConfirmationRequest) (bool, error) {
	// 构建确认消息
	var messageBuilder strings.Builder
	messageBuilder.WriteString(request.Message)

	// 添加详细信息
	if len(request.Details) > 0 {
		messageBuilder.WriteString("\n\n📋 操作详情：")
		for _, detail := range request.Details {
			messageBuilder.WriteString(fmt.Sprintf("\n• %s", detail))
		}
	}

	// 添加警告信息
	if len(request.Warnings) > 0 {
		messageBuilder.WriteString("\n\n⚠️ 重要警告：")
		for _, warning := range request.Warnings {
			messageBuilder.WriteString(fmt.Sprintf("\n• %s", warning))
		}
	}

	// 添加安全级别提示
	securityLevelHint := sm.getSecurityLevelHint(request.Level)
	if securityLevelHint != "" {
		messageBuilder.WriteString(fmt.Sprintf("\n\n🛡️ %s", securityLevelHint))
	}

	// 显示确认对话框
	confirmed, err := sm.ui.ShowConfirmDialog(request.Title, messageBuilder.String())

	// 记录确认结果
	sm.LogSecurityEvent(ctx, SecurityEvent{
		ID:        generateEventID(),
		Timestamp: time.Now(),
		EventType: "confirmation_request",
		Operation: request.Operation,
		Level:     request.Level,
		Success:   confirmed && err == nil,
		Message:   fmt.Sprintf("用户确认操作: %s, 结果: %v", request.Operation, confirmed),
	})

	return confirmed, err
}

// showDoubleConfirmationWarning 显示双重确认警告
func (sm *securityManager) showDoubleConfirmationWarning(request ConfirmationRequest) {
	warning := SecurityWarning{
		Level: CriticalSecurity,
		Title: "🔐 高风险操作警告",
		Message: fmt.Sprintf(`
您即将执行高风险操作：%s

此操作可能会：
• 影响您的资产安全
• 无法撤销或回滚
• 产生不可预期的结果

请仔细确认操作内容，并确保您完全理解操作后果。
`, request.Message),
		Suggestions: []string{
			"仔细检查所有操作参数",
			"确保在安全的网络环境中操作",
			"建议先进行小额测试",
			"保持钱包和私钥的安全",
		},
	}

	sm.ShowSecurityWarning(context.Background(), warning)
}

// getSecurityLevelHint 获取安全级别提示
func (sm *securityManager) getSecurityLevelHint(level SecurityLevel) string {
	switch level {
	case LowSecurity:
		return "安全级别：低 - 常规操作"
	case MediumSecurity:
		return "安全级别：中 - 请谨慎操作"
	case HighSecurity:
		return "安全级别：高 - 请仔细确认"
	case CriticalSecurity:
		return "安全级别：关键 - 极度危险操作，请三思而后行"
	default:
		return ""
	}
}

// AuthenticateUser 用户身份验证
func (sm *securityManager) AuthenticateUser(ctx context.Context, securityContext SecurityContext) (bool, error) {
	sm.logger.Info(fmt.Sprintf("用户身份验证: user_id=%s, operation=%s", securityContext.UserID, securityContext.Operation))

	// 检查是否需要身份验证
	if !sm.policy.RequireAuthentication[securityContext.Operation] {
		return true, nil // 不需要身份验证
	}

	// 简化实现：通过UI获取密码
	password, err := sm.ui.ShowInputDialog(
		"身份验证",
		"请输入钱包密码以验证身份",
		true,
	)

	if err != nil {
		return false, fmt.Errorf("身份验证取消: %v", err)
	}

	// 验证密码
	isValid, err := sm.ValidatePassword(ctx, password)
	if err != nil {
		return false, fmt.Errorf("密码验证失败: %v", err)
	}

	// 记录身份验证事件
	sm.LogSecurityEvent(ctx, SecurityEvent{
		ID:        generateEventID(),
		Timestamp: time.Now(),
		UserID:    securityContext.UserID,
		SessionID: securityContext.SessionID,
		EventType: "authentication",
		Operation: securityContext.Operation,
		Level:     securityContext.SecurityLevel,
		Success:   isValid,
		Message:   "用户身份验证",
		IPAddress: securityContext.IPAddress,
	})

	return isValid, nil
}

// ValidatePassword 验证密码
func (sm *securityManager) ValidatePassword(ctx context.Context, password string) (bool, error) {
	// 简化实现：基本密码强度检查
	if len(password) < 6 {
		return false, fmt.Errorf("密码长度不足")
	}

	// 这里应该与实际的钱包密码验证集成
	// 目前返回true作为演示
	return true, nil
}

// MaskSensitiveData 掩码敏感数据
func (sm *securityManager) MaskSensitiveData(data string) string {
	if !sm.policy.MaskSensitiveData {
		return data
	}

	maskedData := data

	// 应用敏感数据模式
	for _, pattern := range sm.sensitivePatterns {
		maskedData = pattern.ReplaceAllStringFunc(maskedData, func(match string) string {
			if len(match) <= 6 {
				return strings.Repeat("*", len(match))
			}
			// 保留前2位和后2位，中间用*替换
			return match[:2] + strings.Repeat("*", len(match)-4) + match[len(match)-2:]
		})
	}

	return maskedData
}

// ValidateSensitiveOperation 验证敏感操作
func (sm *securityManager) ValidateSensitiveOperation(ctx context.Context, operation OperationType) error {
	// 检查操作是否被允许
	switch operation {
	case TransferOperation:
		return sm.validateTransferOperation(ctx)
	case WalletOperation:
		return sm.validateWalletOperation(ctx)
	case ConsensusOperation:
		return sm.validateConsensusOperation(ctx)
	default:
		return nil // 其他操作默认允许
	}
}

// validateTransferOperation 验证转账操作
func (sm *securityManager) validateTransferOperation(ctx context.Context) error {
	// 检查是否有活跃的转账会话
	// 检查转账限额
	// 检查目标地址是否在黑名单
	// 简化实现，返回nil
	return nil
}

// validateWalletOperation 验证钱包操作
func (sm *securityManager) validateWalletOperation(ctx context.Context) error {
	// 检查钱包操作频率
	// 检查是否有可疑活动
	// 简化实现，返回nil
	return nil
}

// validateConsensusOperation 验证共识操作
func (sm *securityManager) validateConsensusOperation(ctx context.Context) error {
	// 检查共识参与权限
	// 检查系统资源状况
	// 简化实现，返回nil
	return nil
}

// ShowSecurityWarning 显示安全警告
func (sm *securityManager) ShowSecurityWarning(ctx context.Context, warning SecurityWarning) error {
	if !sm.policy.ShowSecurityWarnings {
		return nil // 不显示安全警告
	}

	// 构建警告消息
	var messageBuilder strings.Builder
	messageBuilder.WriteString(warning.Message)

	// 添加安全建议
	if len(warning.Suggestions) > 0 {
		messageBuilder.WriteString("\n\n💡 安全建议：")
		for _, suggestion := range warning.Suggestions {
			messageBuilder.WriteString(fmt.Sprintf("\n• %s", suggestion))
		}
	}

	// 添加了解更多链接
	if warning.LearnMore != "" {
		messageBuilder.WriteString(fmt.Sprintf("\n\n🔗 了解更多：%s", warning.LearnMore))
	}

	// 显示警告
	sm.ui.ShowSecurityWarning(messageBuilder.String())

	// 记录警告事件
	sm.LogSecurityEvent(ctx, SecurityEvent{
		ID:        generateEventID(),
		Timestamp: time.Now(),
		EventType: "security_warning",
		Level:     warning.Level,
		Success:   true,
		Message:   fmt.Sprintf("显示安全警告: %s", warning.Title),
	})

	return nil
}

// ShowOperationSummary 显示操作摘要
func (sm *securityManager) ShowOperationSummary(ctx context.Context, summary OperationSummary) error {
	if !sm.policy.ShowOperationSummary {
		return nil // 不显示操作摘要
	}

	sm.ui.ShowSection(fmt.Sprintf("📋 操作摘要 - %s", summary.Operation))

	// 显示基本信息
	basicInfo := map[string]string{
		"操作类型": string(summary.Operation),
		"操作描述": summary.Description,
		"影响说明": summary.Impact,
		"执行时间": summary.Timestamp.Format("2006-01-02 15:04:05"),
	}

	sm.ui.ShowKeyValuePairs("基本信息", basicInfo)

	// 显示操作参数
	if len(summary.Parameters) > 0 {
		paramInfo := make(map[string]string)
		for key, value := range summary.Parameters {
			paramInfo[key] = sm.MaskSensitiveData(fmt.Sprintf("%v", value))
		}
		sm.ui.ShowKeyValuePairs("操作参数", paramInfo)
	}

	// 显示警告信息
	if len(summary.Warnings) > 0 {
		sm.ui.ShowWarning("⚠️ 相关警告：")
		for _, warning := range summary.Warnings {
			sm.ui.ShowWarning(fmt.Sprintf("• %s", warning))
		}
	}

	return nil
}

// compileSensitivePatterns 编译敏感数据模式
func (sm *securityManager) compileSensitivePatterns() {
	patterns := append(sm.policy.SensitiveDataPatterns, getDefaultSensitivePatterns()...)

	sm.sensitivePatterns = make([]*regexp.Regexp, 0, len(patterns))

	for _, pattern := range patterns {
		if compiled, err := regexp.Compile(pattern); err == nil {
			sm.sensitivePatterns = append(sm.sensitivePatterns, compiled)
		} else {
			sm.logger.Info(fmt.Sprintf("编译敏感数据模式失败: pattern=%s, error=%v", pattern, err))
		}
	}
}

// LogSecurityEvent 记录安全事件
func (sm *securityManager) LogSecurityEvent(ctx context.Context, event SecurityEvent) {
	// 设置事件ID和时间戳（如果未设置）
	if event.ID == "" {
		event.ID = generateEventID()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// 添加到事件日志
	sm.eventLog = append(sm.eventLog, event)

	// 保持日志大小限制（最多保留1000条记录）
	if len(sm.eventLog) > 1000 {
		sm.eventLog = sm.eventLog[1:]
	}

	// 记录到系统日志
	if sm.policy.LogSensitiveOperations || event.Level <= MediumSecurity {
		sm.logger.Info(fmt.Sprintf("安全事件: type=%s, operation=%s, level=%s, success=%v",
			event.EventType, event.Operation, event.Level.String(), event.Success))
	}
}

// GetSecurityAuditLog 获取安全审计日志
func (sm *securityManager) GetSecurityAuditLog(ctx context.Context, limit int) ([]SecurityEvent, error) {
	if limit <= 0 || limit > len(sm.eventLog) {
		limit = len(sm.eventLog)
	}

	// 返回最近的事件（倒序）
	result := make([]SecurityEvent, limit)
	startIndex := len(sm.eventLog) - limit

	for i := 0; i < limit; i++ {
		result[i] = sm.eventLog[startIndex+i]
	}

	return result, nil
}

// SetSecurityPolicy 设置安全策略
func (sm *securityManager) SetSecurityPolicy(policy SecurityPolicy) {
	sm.policy = policy
	sm.compileSensitivePatterns() // 重新编译敏感数据模式
	sm.logger.Info("安全策略已更新")
}

// GetSecurityPolicy 获取当前安全策略
func (sm *securityManager) GetSecurityPolicy() SecurityPolicy {
	return sm.policy
}

// 辅助函数

// generateEventID 生成事件ID
func generateEventID() string {
	return fmt.Sprintf("sec_%d", time.Now().UnixNano())
}

// getDefaultSecurityPolicy 获取默认安全策略
func getDefaultSecurityPolicy() SecurityPolicy {
	return SecurityPolicy{
		RequireConfirmation: map[OperationType]bool{
			WalletOperation:    true,
			TransferOperation:  true,
			ConsensusOperation: true,
			SystemOperation:    false,
			SettingsOperation:  false,
		},
		DoubleConfirmation: map[OperationType]bool{
			TransferOperation: true,
		},
		RequireAuthentication: map[OperationType]bool{
			WalletOperation:   true,
			TransferOperation: true,
		},
		MaskSensitiveData:      true,
		LogSensitiveOperations: true,
		ConfirmationTimeout:    30 * time.Second,
		AuthenticationTimeout:  60 * time.Second,
		SessionTimeout:         30 * time.Minute,
		ShowSecurityWarnings:   true,
		ShowOperationSummary:   true,
		WarnUnsafeOperations:   true,
	}
}

// getDefaultSensitivePatterns 获取默认敏感数据模式
func getDefaultSensitivePatterns() []string {
	return []string{
		`[0-9a-fA-F]{64}`, // 私钥模式
		`[0-9a-fA-F]{40}`, // 地址模式
		`password=\S+`,    // 密码参数
		`privatekey=\S+`,  // 私钥参数
		`seed=\S+`,        // 种子参数
	}
}
