package ui

import (
	"fmt"

	"github.com/pterm/pterm"

	"github.com/weisyn/v1/internal/cli/status"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// SimpleLayout 简洁布局 - 替代复杂的ASCII边框布局
type SimpleLayout struct {
	logger        log.Logger
	statusManager *status.StatusManager
}

// NewSimpleLayout 创建简洁布局
func NewSimpleLayout(
	logger log.Logger,
	statusManager *status.StatusManager,
) *SimpleLayout {
	return &SimpleLayout{
		logger:        logger,
		statusManager: statusManager,
	}
}

// ShowMainInterface 显示主界面（首次启动时使用）
func (sl *SimpleLayout) ShowMainInterface() {
	// 清屏
	pterm.Print("\033[2J\033[H")

	// 显示单行 TopBar（统一标准）
	if sl.statusManager != nil {
		statusBar := sl.statusManager.RenderStatusBar()
		pterm.Println(statusBar)
		pterm.Println()
	}
}

// ShowStatusOnly 只显示单行TopBar（菜单循环时使用，避免重复内容）
func (sl *SimpleLayout) ShowStatusOnly() {
	if sl.statusManager != nil {
		statusBar := sl.statusManager.RenderStatusBar()
		pterm.Println(statusBar)
		pterm.Println()
	}
}

// ShowPageHeader 统一的页面头部显示（清屏+TopBar，供所有子功能页面使用）
func (sl *SimpleLayout) ShowPageHeader() {
	// 清屏
	pterm.Print("\033[2J\033[H")

	// 显示统一TopBar
	if sl.statusManager != nil {
		statusBar := sl.statusManager.RenderStatusBar()
		pterm.Println(statusBar)
		pterm.Println()
	}
}

// showStatusInfo 显示状态信息
func (sl *SimpleLayout) showStatusInfo() {
	if sl.statusManager == nil {
		return
	}

	status := sl.statusManager.GetStatus()

	// 创建状态信息表格
	statusData := [][]string{
		{"版本", status.Version},
		{"节点ID", status.NodeID},
		{"区块高度", fmt.Sprintf("%d", status.BlockHeight)},
		{"连接节点", fmt.Sprintf("%d", status.ConnectedPeers)},
		{"挖矿状态", func() string {
			if status.IsMining {
				return "运行中"
			}
			return "已停止"
		}()},
	}

	// 显示状态表格
	pterm.DefaultTable.WithHasHeader(false).WithData(statusData).Render()
}

// showStatusBar 显示状态栏（兼容旧代码，暂时保留）
func (sl *SimpleLayout) showStatusBar() {
	if sl.statusManager != nil {
		statusBar := sl.statusManager.RenderStatusBar()
		pterm.Println(statusBar)
		pterm.Println()
	}
}

// ShowSystemStatus 显示系统状态页面
func (sl *SimpleLayout) ShowSystemStatus() {
	sl.ShowMainInterface()

	if sl.statusManager != nil {
		sl.statusManager.RenderDetailedStatus()
	}

	pterm.Println()
	ShowStandardWaitPrompt("return")
}

// ShowSuccessMessage 显示成功消息
func (sl *SimpleLayout) ShowSuccessMessage(message string) {
	pterm.Success.Println(message)
}

// ShowErrorMessage 显示错误消息
func (sl *SimpleLayout) ShowErrorMessage(message string) {
	pterm.Error.Println(message)
}

// ShowInfoMessage 显示信息消息
func (sl *SimpleLayout) ShowInfoMessage(message string) {
	pterm.Info.Println(message)
}

// ShowWarningMessage 显示警告消息
func (sl *SimpleLayout) ShowWarningMessage(message string) {
	pterm.Warning.Println(message)
}

// ShowLoadingSpinner 显示加载动画
func (sl *SimpleLayout) ShowLoadingSpinner(message string) (*pterm.SpinnerPrinter, error) {
	return pterm.DefaultSpinner.WithText(message).Start()
}

// ShowProgressBar 显示进度条
func (sl *SimpleLayout) ShowProgressBar(title string, total int) (*pterm.ProgressbarPrinter, error) {
	return pterm.DefaultProgressbar.WithTotal(total).WithTitle(title).Start()
}

// ShowTable 显示表格
func (sl *SimpleLayout) ShowTable(headers []string, data [][]string) {
	tableData := [][]string{headers}
	tableData = append(tableData, data...)
	pterm.DefaultTable.WithHasHeader(true).WithData(tableData).Render()
}

// ShowSimpleTable 显示简单表格（无表头）
func (sl *SimpleLayout) ShowSimpleTable(data [][]string) {
	pterm.DefaultTable.WithHasHeader(false).WithData(data).Render()
}

// ShowSection 显示分区标题
func (sl *SimpleLayout) ShowSection(title string) {
	pterm.DefaultSection.Println(title)
}

// ShowBox 显示信息框
func (sl *SimpleLayout) ShowBox(title, content string) {
	if title != "" {
		pterm.DefaultBox.WithTitle(title).WithTitleTopCenter().Println(content)
	} else {
		pterm.DefaultBox.Println(content)
	}
}

// ShowMenu 显示菜单选择
func (sl *SimpleLayout) ShowMenu(title string, options []string) (int, error) {
	if title != "" {
		pterm.DefaultSection.Println(title)
		pterm.Println()
	}

	// 显示标准化操作提示
	ShowStandardTip("menu")

	result, err := pterm.DefaultInteractiveSelect.
		WithOptions(options).
		WithDefaultOption(options[0]).
		WithMaxHeight(12). // 增加高度以确保所有选项都能显示
		WithFilter(false). // 禁用搜索过滤
		Show("📋 请选择:")

	if err != nil {
		// 改进错误处理
		if err.Error() == "interrupt" {
			return -1, fmt.Errorf("用户取消操作")
		}
		return -1, fmt.Errorf("菜单选择失败: %v", err)
	}

	// 找到选中项的索引
	for i, option := range options {
		if option == result {
			return i, nil
		}
	}

	return 0, nil
}

// ShowInputDialog 显示输入对话框
func (sl *SimpleLayout) ShowInputDialog(title string, prompt string, isSecret bool) (string, error) {
	if title != "" {
		pterm.DefaultSection.Println(title)
		pterm.Println()
	}

	if isSecret {
		return pterm.DefaultInteractiveTextInput.
			WithMask("*").
			Show(prompt)
	}

	return pterm.DefaultInteractiveTextInput.Show(prompt)
}

// ShowConfirmDialog 显示确认对话框
func (sl *SimpleLayout) ShowConfirmDialog(message string) (bool, error) {
	result, err := pterm.DefaultInteractiveConfirm.
		WithDefaultValue(false).
		Show(message)

	return result, err
}

// WaitForEnter 等待用户按回车键
func (sl *SimpleLayout) WaitForEnter(message string) {
	if message == "" {
		ShowStandardWaitPrompt("continue")
	} else {
		pterm.DefaultInteractiveTextInput.
			WithDefaultText("").
			Show(message)
	}
}
