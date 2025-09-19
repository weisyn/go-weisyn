// Package config 提供CLI的配置管理功能 - 监听器
package config

import (
	"fmt"
	"time"

	"github.com/weisyn/v1/internal/cli/ui"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// UIThemeListener UI主题变更监听器
type UIThemeListener struct {
	logger log.Logger
	ui     ui.Components
}

// NewUIThemeListener 创建UI主题监听器
func NewUIThemeListener(logger log.Logger, uiComponents ui.Components) ConfigListener {
	return &UIThemeListener{
		logger: logger,
		ui:     uiComponents,
	}
}

// OnConfigChanged 处理配置变更
func (l *UIThemeListener) OnConfigChanged(event ConfigChangeEvent) error {
	if event.Key != "ui.theme" {
		return nil // 只处理主题变更
	}

	theme, ok := event.NewValue.(string)
	if !ok {
		return fmt.Errorf("UI主题值类型错误: %T", event.NewValue)
	}

	l.logger.Info(fmt.Sprintf("UI主题变更: %s -> %s", event.OldValue, theme))

	// 这里可以触发UI组件的主题更新
	// 由于ui.Components接口还没有主题更新方法，这里先记录日志
	l.logger.Info(fmt.Sprintf("应用新主题: %s", theme))

	return nil
}

// SecurityListener 安全配置监听器
type SecurityListener struct {
	logger log.Logger
	ui     ui.Components
}

// NewSecurityListener 创建安全监听器
func NewSecurityListener(logger log.Logger, uiComponents ui.Components) ConfigListener {
	return &SecurityListener{
		logger: logger,
		ui:     uiComponents,
	}
}

// OnConfigChanged 处理配置变更
func (l *SecurityListener) OnConfigChanged(event ConfigChangeEvent) error {
	securityKeys := []string{
		"security.require_confirmation",
		"security.session_timeout",
		"security.mask_sensitive_data",
		"security.audit_logging",
	}

	// 检查是否是安全相关配置
	isSecurityConfig := false
	for _, key := range securityKeys {
		if event.Key == key {
			isSecurityConfig = true
			break
		}
	}

	if !isSecurityConfig {
		return nil
	}

	l.logger.Info(fmt.Sprintf("安全配置变更: key=%s, value=%v", event.Key, event.NewValue))

	// 根据不同的安全配置执行相应操作
	switch event.Key {
	case "security.require_confirmation":
		if confirmed, ok := event.NewValue.(bool); ok && confirmed {
			l.ui.ShowSecurityWarning("已启用操作确认，所有敏感操作将需要确认")
		}

	case "security.session_timeout":
		if timeout, ok := event.NewValue.(int); ok {
			l.ui.ShowInfo(fmt.Sprintf("会话超时设置已更新，新超时时间: %d 秒", timeout))
		}

	case "security.mask_sensitive_data":
		if mask, ok := event.NewValue.(bool); ok {
			status := "禁用"
			if mask {
				status = "启用"
			}
			l.ui.ShowInfo(fmt.Sprintf("敏感数据遮罩设置已更新，当前状态: %s", status))
		}

	case "security.audit_logging":
		if audit, ok := event.NewValue.(bool); ok {
			status := "禁用"
			if audit {
				status = "启用"
			}
			l.ui.ShowInfo(fmt.Sprintf("审计日志设置已更新，当前状态: %s", status))
		}
	}

	return nil
}

// NetworkListener 网络配置监听器
type NetworkListener struct {
	logger log.Logger
	ui     ui.Components
}

// NewNetworkListener 创建网络监听器
func NewNetworkListener(logger log.Logger, uiComponents ui.Components) ConfigListener {
	return &NetworkListener{
		logger: logger,
		ui:     uiComponents,
	}
}

// OnConfigChanged 处理配置变更
func (l *NetworkListener) OnConfigChanged(event ConfigChangeEvent) error {
	networkKeys := []string{
		"network.api_url",
		"network.timeout",
		"network.max_retries",
		"network.enable_tls",
	}

	// 检查是否是网络相关配置
	isNetworkConfig := false
	for _, key := range networkKeys {
		if event.Key == key {
			isNetworkConfig = true
			break
		}
	}

	if !isNetworkConfig {
		return nil
	}

	l.logger.Info(fmt.Sprintf("网络配置变更: key=%s, value=%v", event.Key, event.NewValue))

	// 根据不同的网络配置执行相应操作
	switch event.Key {
	case "network.api_url":
		if url, ok := event.NewValue.(string); ok {
			l.ui.ShowInfo(fmt.Sprintf("API地址已更新，新地址: %s", url))
		}

	case "network.timeout":
		if timeout, ok := event.NewValue.(int); ok {
			l.ui.ShowInfo(fmt.Sprintf("网络超时设置已更新，新超时时间: %d 秒", timeout))
		}

	case "network.max_retries":
		if retries, ok := event.NewValue.(int); ok {
			l.ui.ShowInfo(fmt.Sprintf("最大重试次数已更新，新重试次数: %d", retries))
		}

	case "network.enable_tls":
		if tls, ok := event.NewValue.(bool); ok {
			status := "禁用"
			if tls {
				status = "启用"
			}
			l.ui.ShowInfo(fmt.Sprintf("TLS设置已更新，当前状态: %s", status))
		}
	}

	return nil
}

// AuditListener 审计监听器（记录所有配置变更）
type AuditListener struct {
	logger    log.Logger
	auditFile string
}

// NewAuditListener 创建审计监听器
func NewAuditListener(logger log.Logger, auditFile string) ConfigListener {
	return &AuditListener{
		logger:    logger,
		auditFile: auditFile,
	}
}

// OnConfigChanged 处理配置变更
func (l *AuditListener) OnConfigChanged(event ConfigChangeEvent) error {
	// 记录审计日志
	auditEntry := fmt.Sprintf(
		"[%s] CONFIG_CHANGE: key=%s, old_value=%v, new_value=%v, scope=%s, source=%s",
		event.Timestamp.Format(time.RFC3339),
		event.Key,
		event.OldValue,
		event.NewValue,
		event.Scope,
		event.Source,
	)

	l.logger.Info(auditEntry)

	// 在实际实现中，这里可以写入到专门的审计文件
	// 为了简化，这里只记录到常规日志

	return nil
}

// ValidationListener 验证监听器（在配置变更前进行额外验证）
type ValidationListener struct {
	logger     log.Logger
	validators []ConfigValidator
}

// NewValidationListener 创建验证监听器
func NewValidationListener(logger log.Logger) ConfigListener {
	return &ValidationListener{
		logger:     logger,
		validators: make([]ConfigValidator, 0),
	}
}

// AddValidator 添加验证器
func (l *ValidationListener) AddValidator(validator ConfigValidator) {
	l.validators = append(l.validators, validator)
}

// OnConfigChanged 处理配置变更
func (l *ValidationListener) OnConfigChanged(event ConfigChangeEvent) error {
	// 执行额外的验证逻辑
	for _, validator := range l.validators {
		if err := validator.Validate(event.Key, event.NewValue); err != nil {
			l.logger.Error(fmt.Sprintf("配置验证失败: key=%s, error=%v", event.Key, err))
			return fmt.Errorf("配置验证失败: %v", err)
		}
	}

	l.logger.Info(fmt.Sprintf("配置验证通过: key=%s", event.Key))
	return nil
}

// WalletListener 钱包配置监听器
type WalletListener struct {
	logger log.Logger
	ui     ui.Components
}

// NewWalletListener 创建钱包监听器
func NewWalletListener(logger log.Logger, uiComponents ui.Components) ConfigListener {
	return &WalletListener{
		logger: logger,
		ui:     uiComponents,
	}
}

// OnConfigChanged 处理配置变更
func (l *WalletListener) OnConfigChanged(event ConfigChangeEvent) error {
	walletKeys := []string{
		"wallet.auto_lock_timeout",
		"wallet.backup_on_create",
		"wallet.encryption_enabled",
	}

	// 检查是否是钱包相关配置
	isWalletConfig := false
	for _, key := range walletKeys {
		if event.Key == key {
			isWalletConfig = true
			break
		}
	}

	if !isWalletConfig {
		return nil
	}

	l.logger.Info(fmt.Sprintf("钱包配置变更: key=%s, value=%v", event.Key, event.NewValue))

	// 根据不同的钱包配置执行相应操作
	switch event.Key {
	case "wallet.auto_lock_timeout":
		if timeout, ok := event.NewValue.(int); ok {
			l.ui.ShowInfo(fmt.Sprintf("钱包自动锁定时间已更新，新锁定时间: %d 秒", timeout))
		}

	case "wallet.backup_on_create":
		if backup, ok := event.NewValue.(bool); ok {
			status := "禁用"
			if backup {
				status = "启用"
			}
			l.ui.ShowInfo(fmt.Sprintf("钱包创建时备份设置已更新，当前状态: %s", status))
		}

	case "wallet.encryption_enabled":
		if encryption, ok := event.NewValue.(bool); ok {
			status := "禁用"
			if encryption {
				status = "启用"
			}
			l.ui.ShowInfo(fmt.Sprintf("钱包加密设置已更新，当前状态: %s", status))
		}
	}

	return nil
}
