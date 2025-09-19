package ui

import (
	"fmt"
	"time"

	"github.com/pterm/pterm"

	"github.com/weisyn/v1/internal/cli/client"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// Components UI组件接口，定义所有可用的UI组件
type Components interface {
	// 数据展示组件
	ShowTable(title string, data [][]string) error
	ShowList(title string, items []string) error
	ShowKeyValuePairs(title string, pairs map[string]string) error

	// 交互选择组件
	ShowMenu(title string, options []string) (int, error)
	ShowConfirmDialog(title, message string) (bool, error)
	ShowInputDialog(title, prompt string, isPassword bool) (string, error)

	// 进度反馈组件
	NewProgressBar(title string, total int) ProgressBar
	ShowSpinner(message string) Spinner
	ShowLoadingMessage(message string) error

	// 状态显示组件
	ShowSuccess(message string) error
	ShowError(message string) error
	ShowWarning(message string) error
	ShowInfo(message string) error

	// 面板和布局组件
	ShowPanel(title, content string) error
	ShowSideBySidePanels(left, right PanelData) error
	ShowHeader(text string) error
	ShowSection(text string) error

	// 权限和安全相关组件
	ShowPermissionStatus(level, status string) error
	ShowSecurityWarning(message string) error
	ShowWalletSelector(wallets []WalletDisplayInfo) (int, error)

	// 特殊组件
	ShowNodeStatus(nodeInfo *client.NodeInfo, miningStatus *client.MiningStatus) error
	ShowBalanceInfo(address string, balance float64, tokenSymbol string) error
}

// ProgressBar 进度条接口
type ProgressBar interface {
	Start() error
	Update(current int, message string) error
	Increment(message string) error
	Finish(message string) error
	Stop() error
}

// Spinner 加载动画接口
type Spinner interface {
	Start() error
	UpdateText(text string) error
	Stop() error
	Success(message string) error
	Fail(message string) error
}

// PanelData 面板数据结构
type PanelData struct {
	Title   string
	Content string
	Width   int
}

// WalletDisplayInfo 钱包显示信息
type WalletDisplayInfo struct {
	ID       string
	Name     string
	Address  string
	Balance  string
	IsLocked bool
}

// components UI组件集合的具体实现
type components struct {
	logger log.Logger
	theme  *ThemeConfig
}

// ThemeConfig 主题配置
type ThemeConfig struct {
	PrimaryColor   pterm.Color
	SecondaryColor pterm.Color
	SuccessColor   pterm.Color
	WarningColor   pterm.Color
	ErrorColor     pterm.Color
	InfoColor      pterm.Color
}

// NewComponents 创建UI组件实例
func NewComponents(logger log.Logger) Components {
	return &components{
		logger: logger,
		theme:  getDefaultTheme(),
	}
}

// 辅助函数

// truncateString 截断字符串到指定长度
func truncateString(str string, maxLen int) string {
	if len(str) <= maxLen {
		return str
	}
	return str[:maxLen-3] + "..."
}

// formatDuration 格式化持续时间
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// getMiningStatusText 获取共识参与状态文本
func (c *components) getMiningStatusText(isActive bool) string {
	if isActive {
		return pterm.Green("🟢 ⛏️ 共识参与中")
	}
	return pterm.Red("🔴 ❌ 未参与共识")
}
