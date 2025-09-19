package cli

import (
	"path/filepath"
)

// CLIOptions CLI配置选项
type CLIOptions struct {
	// 钱包存储路径
	WalletStoragePath string `json:"wallet_storage_path"`

	// UI相关配置
	Theme          string `json:"theme"`           // UI主题
	EnableColors   bool   `json:"enable_colors"`   // 是否启用颜色
	EnableProgress bool   `json:"enable_progress"` // 是否显示进度条

	// 交互配置
	MenuTimeout    int  `json:"menu_timeout"`     // 菜单超时时间(秒)
	ConfirmOnExit  bool `json:"confirm_on_exit"`  // 退出时是否确认
	AutoSaveConfig bool `json:"auto_save_config"` // 是否自动保存配置
}

// Config CLI配置实现
type Config struct {
	options *CLIOptions
}

// New 创建CLI配置实现
func New(userConfig interface{}) *Config {
	defaultOptions := createDefaultCLIOptions()

	config := &Config{
		options: defaultOptions,
	}

	// 处理用户配置
	if userConfig != nil {
		config.applyUserConfig(userConfig)
	}

	return config
}

// createDefaultCLIOptions 创建默认CLI配置
func createDefaultCLIOptions() *CLIOptions {
	return &CLIOptions{
		WalletStoragePath: defaultWalletStoragePath,
		Theme:             defaultTheme,
		EnableColors:      defaultEnableColors,
		EnableProgress:    defaultEnableProgress,
		MenuTimeout:       defaultMenuTimeout,
		ConfirmOnExit:     defaultConfirmOnExit,
		AutoSaveConfig:    defaultAutoSaveConfig,
	}
}

// applyUserConfig 应用用户配置覆盖默认值
func (c *Config) applyUserConfig(userConfig interface{}) {
	// 这里可以根据需要处理用户配置
	// 目前暂时使用默认值
}

// GetOptions 获取完整的CLI配置选项
func (c *Config) GetOptions() *CLIOptions {
	return c.options
}

// GetWalletStoragePath 获取钱包存储路径
func (c *Config) GetWalletStoragePath() string {
	return c.options.WalletStoragePath
}

// GetTheme 获取UI主题
func (c *Config) GetTheme() string {
	return c.options.Theme
}

// IsColorsEnabled 是否启用颜色
func (c *Config) IsColorsEnabled() bool {
	return c.options.EnableColors
}

// IsProgressEnabled 是否显示进度条
func (c *Config) IsProgressEnabled() bool {
	return c.options.EnableProgress
}

// GetMenuTimeout 获取菜单超时时间
func (c *Config) GetMenuTimeout() int {
	return c.options.MenuTimeout
}

// IsConfirmOnExitEnabled 退出时是否确认
func (c *Config) IsConfirmOnExitEnabled() bool {
	return c.options.ConfirmOnExit
}

// IsAutoSaveConfigEnabled 是否自动保存配置
func (c *Config) IsAutoSaveConfigEnabled() bool {
	return c.options.AutoSaveConfig
}

// ResolveWalletStoragePath 解析钱包存储路径（支持相对路径和绝对路径）
func (c *Config) ResolveWalletStoragePath(baseDataPath string) string {
	walletPath := c.options.WalletStoragePath

	// 如果是绝对路径，直接返回
	if filepath.IsAbs(walletPath) {
		return walletPath
	}

	// 如果是相对路径，基于baseDataPath解析
	if baseDataPath != "" {
		return filepath.Join(baseDataPath, walletPath)
	}

	// 默认使用当前目录
	return walletPath
}
