package cli

// CLI配置默认值
const (
	// 钱包存储默认路径（相对于storage.data_path）
	defaultWalletStoragePath = "wallets"

	// UI主题
	defaultTheme = "default"

	// 颜色和进度条
	defaultEnableColors   = true
	defaultEnableProgress = true

	// 交互设置
	defaultMenuTimeout    = 30 // 30秒超时
	defaultConfirmOnExit  = true
	defaultAutoSaveConfig = true
)
