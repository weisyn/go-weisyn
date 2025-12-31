// Package ui 提供基础 UI 组件库
package ui

import (
	"time"

	"github.com/pterm/pterm"
)

// Components UI组件接口，定义所有可用的UI组件
type Components interface {
	// === 数据展示组件 ===

	// ShowTable 显示表格数据
	// title: 表格标题
	// data: 表格数据，第一行为表头
	ShowTable(title string, data [][]string) error

	// ShowList 显示列表
	// title: 列表标题
	// items: 列表项
	ShowList(title string, items []string) error

	// ShowKeyValuePairs 显示键值对
	// title: 标题
	// pairs: 键值对数据
	ShowKeyValuePairs(title string, pairs map[string]string) error

	// === 交互选择组件 ===

	// ShowMenu 显示菜单供用户选择
	// title: 菜单标题
	// options: 菜单选项
	// 返回: 选中的索引
	ShowMenu(title string, options []string) (int, error)

	// ShowConfirmDialog 显示确认对话框
	// title: 对话框标题
	// message: 提示消息
	// 返回: 用户是否确认
	ShowConfirmDialog(title, message string) (bool, error)

	// ShowConfirmDialogWithDefault 显示确认对话框（可指定默认值）
	// title: 对话框标题
	// message: 提示消息
	// defaultValue: 默认值（true=Yes, false=No）
	// 返回: 用户是否确认
	ShowConfirmDialogWithDefault(title, message string, defaultValue bool) (bool, error)

	// ShowInputDialog 显示输入对话框
	// title: 对话框标题
	// prompt: 输入提示
	// isPassword: 是否为密码输入（隐藏显示）
	// 返回: 用户输入的内容
	ShowInputDialog(title, prompt string, isPassword bool) (string, error)

	// ShowContinuePrompt 显示"按 Enter 键继续"的非确认提示（无 Y/n）
	// title: 标题
	// message: 提示消息
	// 行为: 在 TTY 环境下等待用户按 Enter 后返回；非 TTY 直接返回
	ShowContinuePrompt(title, message string) error

	// === 进度反馈组件 ===

	// NewProgressBar 创建进度条
	// title: 进度条标题
	// total: 总进度数
	NewProgressBar(title string, total int) ProgressBar

	// ShowSpinner 显示加载动画
	// message: 加载消息
	ShowSpinner(message string) Spinner

	// ShowLoadingMessage 显示加载消息
	// message: 消息内容
	ShowLoadingMessage(message string) error

	// === 状态显示组件 ===

	// ShowSuccess 显示成功消息
	ShowSuccess(message string) error

	// ShowError 显示错误消息
	ShowError(message string) error

	// ShowWarning 显示警告消息
	ShowWarning(message string) error

	// ShowInfo 显示信息消息
	ShowInfo(message string) error

	// === 面板和布局组件 ===

	// ShowPanel 显示面板
	// title: 面板标题
	// content: 面板内容
	ShowPanel(title, content string) error

	// ShowSideBySidePanels 显示并排面板
	// left: 左侧面板数据
	// right: 右侧面板数据
	ShowSideBySidePanels(left, right PanelData) error

	// ShowHeader 显示标题
	// text: 标题文本
	ShowHeader(text string) error

	// ShowSection 显示分区标题
	// text: 分区文本
	ShowSection(text string) error

	// === 特殊组件 ===

	// ShowPermissionStatus 显示权限状态
	// level: 权限级别
	// status: 状态描述
	ShowPermissionStatus(level, status string) error

	// ShowSecurityWarning 显示安全警告
	// message: 警告消息
	ShowSecurityWarning(message string) error

	// ShowWalletSelector 显示钱包选择器
	// wallets: 钱包信息列表
	// 返回: 选中的钱包索引
	ShowWalletSelector(wallets []WalletDisplayInfo) (int, error)

	// ShowBalanceInfo 显示余额信息
	// address: 地址
	// balance: 余额
	// tokenSymbol: 代币符号
	ShowBalanceInfo(address string, balance float64, tokenSymbol string) error

	// === 屏幕控制组件 ===

	// Clear 清屏
	Clear() error
}

// ProgressBar 进度条接口
type ProgressBar interface {
	// Start 开始进度条
	Start() error

	// Update 更新进度
	// current: 当前进度
	// message: 进度消息
	Update(current int, message string) error

	// Increment 增加进度
	// message: 进度消息
	Increment(message string) error

	// Finish 完成进度条
	// message: 完成消息
	Finish(message string) error

	// Stop 停止进度条
	Stop() error
}

// Spinner 加载动画接口
type Spinner interface {
	// Start 开始动画
	Start() error

	// UpdateText 更新文本
	// text: 新的文本
	UpdateText(text string) error

	// Stop 停止动画
	Stop() error

	// Success 以成功状态停止
	// message: 成功消息
	Success(message string) error

	// Fail 以失败状态停止
	// message: 失败消息
	Fail(message string) error
}

// PanelData 面板数据结构
type PanelData struct {
	Title   string // 面板标题
	Content string // 面板内容
	Width   int    // 面板宽度（0表示自动）
}

// WalletDisplayInfo 钱包显示信息
type WalletDisplayInfo struct {
	ID       string // 钱包ID
	Name     string // 钱包名称
	Address  string // 钱包地址
	Balance  string // 余额（格式化后的字符串）
	IsLocked bool   // 是否锁定
}

// ThemeConfig 主题配置
type ThemeConfig struct {
	PrimaryColor   pterm.Color // 主色调
	SecondaryColor pterm.Color // 辅助色
	SuccessColor   pterm.Color // 成功色
	WarningColor   pterm.Color // 警告色
	ErrorColor     pterm.Color // 错误色
	InfoColor      pterm.Color // 信息色
}

// GetDefaultTheme 获取默认主题配置
func GetDefaultTheme() *ThemeConfig {
	return &ThemeConfig{
		PrimaryColor:   pterm.FgLightBlue,
		SecondaryColor: pterm.FgLightCyan,
		SuccessColor:   pterm.FgGreen,
		WarningColor:   pterm.FgYellow,
		ErrorColor:     pterm.FgRed,
		InfoColor:      pterm.FgCyan,
	}
}

// FormatDuration 格式化时间段
func FormatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return pterm.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return pterm.Sprintf("%dm %ds", minutes, seconds)
	}
	return pterm.Sprintf("%ds", seconds)
}

// TruncateString 截断字符串
func TruncateString(str string, maxLen int) string {
	if len(str) <= maxLen {
		return str
	}
	return str[:maxLen-3] + "..."
}

