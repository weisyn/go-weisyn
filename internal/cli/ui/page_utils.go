package ui

import (
	"fmt"

	"github.com/pterm/pterm"
	"github.com/weisyn/v1/internal/cli/status"
)

// ShowPageHeader 统一的页面头部显示（清屏+简洁提示）
// 所有Commands的子功能页面都应该调用此函数开始页面显示
var globalStatusManager *status.StatusManager

// SetStatusManager 设置全局状态管理器，供页面头部渲染使用
func SetStatusManager(sm *status.StatusManager) { globalStatusManager = sm }

func ShowPageHeader() {
	// 清屏
	pterm.Print("\033[2J\033[H")

	// 显示顶部状态栏（如可用）
	if globalStatusManager != nil {
		statusBar := globalStatusManager.RenderStatusBar()
		pterm.Println(statusBar)
		pterm.Println()
	}
}

// ShowStandardTip 显示标准化的操作提示 - 左对齐
func ShowStandardTip(tipType string) {
	prefixText := pterm.NewStyle(pterm.FgLightBlue, pterm.Bold).Sprint("操作提示  ")

	switch tipType {
	case "menu":
		pterm.Println(prefixText + "💡 使用 ↑↓ 方向键选择选项，Enter 回车键确认，Ctrl+C 退出")
	case "confirm":
		pterm.Println(prefixText + "💡 使用 ←→ 左右键选择 是/否，Enter 回车键确认")
	case "input":
		pterm.Println(prefixText + "✏️ 请输入内容，完成后按 Enter 确认，Ctrl+C 取消")
	case "password":
		pterm.Println(prefixText + "🔒 密码输入将被隐藏显示，输入完成后按 Enter 确认")
	default:
		pterm.Println(prefixText + "💡 使用方向键选择，Enter 确认，Ctrl+C 退出")
	}
	pterm.Println()
}

// ShowStandardWaitPrompt 显示标准化的等待提示
func ShowStandardWaitPrompt(promptType string) {
	var message string
	switch promptType {
	case "continue":
		message = "按 Enter 键继续..."
	case "return":
		message = "按 Enter 键返回..."
	case "return_menu":
		message = "🔄 按 Enter 键返回主菜单..."
	default:
		message = "按 Enter 键继续..."
	}

	pterm.DefaultInteractiveTextInput.
		WithDefaultText("").
		Show(message)

	// 用户按Enter后立即清屏，避免返回时内容堆叠
	pterm.Print("\033[2J\033[H")
}

// StandardErrorFormat 标准化错误消息格式
func StandardErrorFormat(operation, details string, err error) string {
	if err != nil {
		return fmt.Sprintf("%s失败 - %s: %v", operation, details, err)
	}
	return fmt.Sprintf("%s失败 - %s", operation, details)
}

// StandardSuccessFormat 标准化成功消息格式
func StandardSuccessFormat(operation, details string) string {
	if details == "" {
		return fmt.Sprintf("✅ %s成功", operation)
	}
	return fmt.Sprintf("✅ %s成功 - %s", operation, details)
}

// StandardWarningFormat 标准化警告消息格式
func StandardWarningFormat(message, suggestion string) string {
	if suggestion == "" {
		return fmt.Sprintf("⚠️ %s", message)
	}
	return fmt.Sprintf("⚠️ %s\n💡 建议: %s", message, suggestion)
}

// StandardInfoFormat 标准化信息消息格式
func StandardInfoFormat(title, content string) string {
	if title == "" {
		return fmt.Sprintf("💡 %s", content)
	}
	return fmt.Sprintf("💡 %s: %s", title, content)
}

// ShowStandardSpinner 标准化的加载指示器管理
type StandardSpinner struct {
	spinner *pterm.SpinnerPrinter
	message string
}

// StartSpinner 启动标准化加载指示器
func StartSpinner(message string) *StandardSpinner {
	spinner, err := pterm.DefaultSpinner.WithText(message).Start()
	if err != nil {
		// 如果启动失败，显示静态消息作为备选
		pterm.Info.Println(message)
		return &StandardSpinner{spinner: nil, message: message}
	}
	return &StandardSpinner{spinner: spinner, message: message}
}

// UpdateMessage 更新加载指示器消息
func (s *StandardSpinner) UpdateMessage(newMessage string) {
	if s.spinner != nil {
		s.spinner.Text = newMessage
		s.message = newMessage
	} else {
		pterm.Info.Println(newMessage)
	}
}

// Stop 停止加载指示器并清理
func (s *StandardSpinner) Stop() {
	if s.spinner != nil {
		if err := s.spinner.Stop(); err != nil {
			// 记录错误但不阻断流程
		}
		// 极强清理 - 完全清除spinner痕迹
		pterm.Print("\033[2K\r")        // 清除当前行
		pterm.Print("\033[1A\033[2K\r") // 上移一行并清除
		pterm.Print("\033[1A\033[2K\r") // 再上移一行并清除
		pterm.Print("\033[1A\033[2K\r") // 再上移一行并清除
		pterm.Print("\033[2K\r")        // 清除当前行
		pterm.Print("\033[0m\033[?25h") // 重置样式+显示光标
		// 强制清空并刷新缓冲区
		pterm.Print("")
		pterm.Print("")
	}
}

// Success 以成功状态结束加载指示器
func (s *StandardSpinner) Success(message string) {
	if s.spinner != nil {
		s.spinner.Success(message)
		pterm.Print("\033[2K\r")
	} else {
		pterm.Success.Println(message)
	}
}

// Fail 以失败状态结束加载指示器
func (s *StandardSpinner) Fail(message string) {
	if s.spinner != nil {
		s.spinner.Fail(message)
		pterm.Print("\033[2K\r")
	} else {
		pterm.Error.Println(message)
	}
}
