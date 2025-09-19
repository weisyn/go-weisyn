// Package config 提供CLI的配置管理功能 - 工厂函数
package config

import (
	"context"

	"github.com/weisyn/v1/internal/cli/ui"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ConfigManagerOptions 配置管理器选项
type ConfigManagerOptions struct {
	ConfigDir   string       // 配置目录
	Format      ConfigFormat // 配置格式
	EnableAudit bool         // 启用审计
	AuditFile   string       // 审计文件路径
}

// NewConfigManagerWithDefaults 创建带默认配置的配置管理器
func NewConfigManagerWithDefaults(logger log.Logger, uiComponents ui.Components) ConfigManager {
	return NewConfigManager(logger, uiComponents, "")
}

// NewConfigManagerWithOptions 创建带选项的配置管理器
func NewConfigManagerWithOptions(logger log.Logger, uiComponents ui.Components, opts ConfigManagerOptions) ConfigManager {
	cm := NewConfigManager(logger, uiComponents, opts.ConfigDir).(*configManager)

	// 应用选项
	if opts.Format != "" {
		cm.format = opts.Format
	}

	// 注册默认验证器
	defaultValidator := NewDefaultValidator()
	cm.RegisterValidator(defaultValidator)

	// 注册默认监听器
	cm.registerDefaultListeners(uiComponents, opts)

	return cm
}

// CreateFullyConfiguredManager 创建完整配置的配置管理器
func CreateFullyConfiguredManager(logger log.Logger, uiComponents ui.Components, configDir string) (ConfigManager, error) {
	opts := ConfigManagerOptions{
		ConfigDir:   configDir,
		Format:      JSONFormat,
		EnableAudit: true,
		AuditFile:   "config_audit.log",
	}

	cm := NewConfigManagerWithOptions(logger, uiComponents, opts)

	// 初始化配置管理器
	ctx := context.Background()
	if err := cm.Initialize(ctx); err != nil {
		return nil, err
	}

	logger.Info("配置管理器已完全初始化")
	return cm, nil
}

// registerDefaultListeners 注册默认监听器
func (cm *configManager) registerDefaultListeners(uiComponents ui.Components, opts ConfigManagerOptions) {
	// UI主题监听器
	uiThemeListener := NewUIThemeListener(cm.logger, uiComponents)
	cm.RegisterListener(uiThemeListener)

	// 安全配置监听器
	securityListener := NewSecurityListener(cm.logger, uiComponents)
	cm.RegisterListener(securityListener)

	// 网络配置监听器
	networkListener := NewNetworkListener(cm.logger, uiComponents)
	cm.RegisterListener(networkListener)

	// 钱包配置监听器
	walletListener := NewWalletListener(cm.logger, uiComponents)
	cm.RegisterListener(walletListener)

	// 审计监听器
	if opts.EnableAudit {
		auditListener := NewAuditListener(cm.logger, opts.AuditFile)
		cm.RegisterListener(auditListener)
	}

	cm.logger.Info("默认配置监听器已注册")
}

// GetDefaultConfigKeys 获取默认配置键列表
func GetDefaultConfigKeys() []string {
	return []string{
		// UI相关配置
		"ui.theme",
		"ui.language",
		"ui.show_hints",
		"ui.animation_enabled",
		"ui.page_size",

		// 网络相关配置
		"network.api_url",
		"network.timeout",
		"network.max_retries",
		"network.enable_tls",

		// 安全相关配置
		"security.require_confirmation",
		"security.session_timeout",
		"security.mask_sensitive_data",
		"security.audit_logging",

		// 系统相关配置
		"system.log_level",
		"system.auto_save",
		"system.backup_enabled",
		"system.cleanup_interval",

		// 钱包相关配置
		"wallet.auto_lock_timeout",
		"wallet.backup_on_create",
		"wallet.encryption_enabled",
	}
}

// GetConfigDescription 获取配置项描述
func GetConfigDescription(key string) string {
	descriptions := map[string]string{
		// UI相关配置
		"ui.theme":             "界面主题设置",
		"ui.language":          "界面语言设置",
		"ui.show_hints":        "是否显示操作提示",
		"ui.animation_enabled": "是否启用动画效果",
		"ui.page_size":         "分页显示条数",

		// 网络相关配置
		"network.api_url":     "API服务器地址",
		"network.timeout":     "网络请求超时时间（秒）",
		"network.max_retries": "最大重试次数",
		"network.enable_tls":  "是否启用TLS加密",

		// 安全相关配置
		"security.require_confirmation": "敏感操作是否需要确认",
		"security.session_timeout":      "会话超时时间（秒）",
		"security.mask_sensitive_data":  "是否遮罩敏感数据",
		"security.audit_logging":        "是否启用审计日志",

		// 系统相关配置
		"system.log_level":        "日志记录级别",
		"system.auto_save":        "是否自动保存配置",
		"system.backup_enabled":   "是否启用配置备份",
		"system.cleanup_interval": "清理间隔时间（秒）",

		// 钱包相关配置
		"wallet.auto_lock_timeout":  "钱包自动锁定超时（秒）",
		"wallet.backup_on_create":   "创建钱包时是否自动备份",
		"wallet.encryption_enabled": "是否启用钱包加密",
	}

	if desc, exists := descriptions[key]; exists {
		return desc
	}

	return "未知配置项"
}

// ExportConfigTemplate 导出配置模板
func ExportConfigTemplate() map[string]interface{} {
	template := make(map[string]interface{})

	// 使用默认值填充模板
	defaultValues := map[string]interface{}{
		// UI相关默认配置
		"ui.theme":             "default",
		"ui.language":          "zh-CN",
		"ui.show_hints":        true,
		"ui.animation_enabled": true,
		"ui.page_size":         20,

		// 网络相关默认配置
		"network.api_url":     "http://localhost:8080",
		"network.timeout":     30,
		"network.max_retries": 3,
		"network.enable_tls":  false,

		// 安全相关默认配置
		"security.require_confirmation": true,
		"security.session_timeout":      1800,
		"security.mask_sensitive_data":  true,
		"security.audit_logging":        true,

		// 系统相关默认配置
		"system.log_level":        "info",
		"system.auto_save":        true,
		"system.backup_enabled":   true,
		"system.cleanup_interval": 3600,

		// 钱包相关默认配置
		"wallet.auto_lock_timeout":  600,
		"wallet.backup_on_create":   true,
		"wallet.encryption_enabled": true,
	}

	for key, value := range defaultValues {
		template[key] = value
	}

	return template
}
