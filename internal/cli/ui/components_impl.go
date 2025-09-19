package ui

import (
	"fmt"

	"github.com/pterm/pterm"

	"github.com/weisyn/v1/internal/cli/client"
)

// ShowTable 显示表格
func (c *components) ShowTable(title string, data [][]string) error {
	if len(data) == 0 {
		return fmt.Errorf("表格数据为空")
	}

	// 创建带标题的表格
	table := pterm.DefaultTable.WithHasHeader().WithHeaderRowSeparator("-")

	if title != "" {
		pterm.DefaultHeader.WithBackgroundStyle(pterm.NewStyle(c.theme.PrimaryColor)).
			Println(title)
	}

	return table.WithData(data).Render()
}

// ShowList 显示列表
func (c *components) ShowList(title string, items []string) error {
	if title != "" {
		pterm.DefaultHeader.WithBackgroundStyle(pterm.NewStyle(c.theme.PrimaryColor)).
			Println(title)
	}

	// 转换string切片为BulletListItem切片
	listItems := make([]pterm.BulletListItem, len(items))
	for i, item := range items {
		listItems[i] = pterm.BulletListItem{Text: item}
	}
	list := pterm.DefaultBulletList.WithItems(listItems)
	return list.Render()
}

// ShowKeyValuePairs 显示键值对
func (c *components) ShowKeyValuePairs(title string, pairs map[string]string) error {
	if title != "" {
		pterm.DefaultHeader.WithBackgroundStyle(pterm.NewStyle(c.theme.PrimaryColor)).
			Println(title)
	}

	// 转换为表格数据
	data := [][]string{{"项目", "值"}}
	for key, value := range pairs {
		data = append(data, []string{key, value})
	}

	table := pterm.DefaultTable.WithHasHeader().WithHeaderRowSeparator("-")
	return table.WithData(data).Render()
}

// ShowMenu 显示菜单选择
func (c *components) ShowMenu(title string, options []string) (int, error) {
	if title != "" {
		pterm.DefaultHeader.WithBackgroundStyle(pterm.NewStyle(c.theme.PrimaryColor)).
			Println(title)
		pterm.Println()
	}

	// 显示标准化操作提示
	ShowStandardTip("menu")

	result, err := pterm.DefaultInteractiveSelect.
		WithOptions(options).
		WithDefaultText("请选择一个选项").
		WithMaxHeight(10). // 确保选项能够完全显示
		WithFilter(false). // 禁用过滤以避免混乱
		Show()

	if err != nil {
		// 改善错误处理
		if err.Error() == "interrupt" {
			return -1, fmt.Errorf("用户取消操作")
		}
		return -1, fmt.Errorf("菜单选择失败: %v", err)
	}

	// 查找选中项的索引
	for i, option := range options {
		if option == result {
			return i, nil
		}
	}

	return -1, fmt.Errorf("未找到选中的选项: %s", result)
}

// ShowConfirmDialog 显示确认对话框
func (c *components) ShowConfirmDialog(title, message string) (bool, error) {
	if title != "" {
		pterm.DefaultHeader.WithBackgroundStyle(pterm.NewStyle(c.theme.WarningColor)).
			Println(title)
		pterm.Println()
	}

	pterm.Info.Println(message)
	pterm.Println()

	// 显示标准化操作提示
	ShowStandardTip("confirm")

	result, err := pterm.DefaultInteractiveConfirm.
		WithDefaultText("确认继续吗？").
		WithDefaultValue(false).
		Show()

	if err != nil {
		return false, fmt.Errorf("确认对话框失败: %v", err)
	}

	return result, nil
}

// ShowInputDialog 显示输入对话框
func (c *components) ShowInputDialog(title, prompt string, isPassword bool) (string, error) {
	if title != "" {
		pterm.DefaultHeader.WithBackgroundStyle(pterm.NewStyle(c.theme.InfoColor)).
			Println(title)
		pterm.Println()
	}

	// 显示标准化操作提示
	if isPassword {
		ShowStandardTip("password")
	} else {
		ShowStandardTip("input")
	}

	var result string
	var err error

	if isPassword {
		result, err = pterm.DefaultInteractiveTextInput.
			WithMask("*").
			WithDefaultText(prompt).
			Show()
	} else {
		result, err = pterm.DefaultInteractiveTextInput.
			WithDefaultText(prompt).
			Show()
	}

	if err != nil {
		// 改进错误处理
		if err.Error() == "interrupt" {
			return "", fmt.Errorf("用户取消输入")
		}
		return "", fmt.Errorf("输入对话框失败: %v", err)
	}

	return result, nil
}

// NewProgressBar 创建进度条
func (c *components) NewProgressBar(title string, total int) ProgressBar {
	return &progressBarImpl{
		title: title,
		total: total,
		theme: c.theme,
	}
}

// ShowSpinner 显示加载动画
func (c *components) ShowSpinner(message string) Spinner {
	return &spinnerImpl{
		message: message,
		theme:   c.theme,
	}
}

// ShowLoadingMessage 显示加载消息
func (c *components) ShowLoadingMessage(message string) error {
	pterm.Info.WithPrefix(pterm.Prefix{
		Text:  "LOADING",
		Style: pterm.NewStyle(c.theme.InfoColor),
	}).Println(message)
	return nil
}

// ShowSuccess 显示成功消息
func (c *components) ShowSuccess(message string) error {
	pterm.Success.WithPrefix(pterm.Prefix{
		Text:  "SUCCESS",
		Style: pterm.NewStyle(c.theme.SuccessColor),
	}).Println(message)
	return nil
}

// ShowError 显示错误消息
func (c *components) ShowError(message string) error {
	pterm.Error.WithPrefix(pterm.Prefix{
		Text:  "ERROR",
		Style: pterm.NewStyle(c.theme.ErrorColor),
	}).Println(message)
	return nil
}

// ShowWarning 显示警告消息
func (c *components) ShowWarning(message string) error {
	pterm.Warning.WithPrefix(pterm.Prefix{
		Text:  "WARNING",
		Style: pterm.NewStyle(c.theme.WarningColor),
	}).Println(message)
	return nil
}

// ShowInfo 显示信息消息
func (c *components) ShowInfo(message string) error {
	pterm.Info.WithPrefix(pterm.Prefix{
		Text:  "INFO",
		Style: pterm.NewStyle(c.theme.InfoColor),
	}).Println(message)
	return nil
}

// ShowPanel 显示面板
func (c *components) ShowPanel(title, content string) error {
	panel := pterm.DefaultBox.
		WithTitle(title).
		WithTitleTopCenter().
		WithBoxStyle(pterm.NewStyle(c.theme.PrimaryColor))

	panel.Println(content)
	return nil
}

// ShowSideBySidePanels 显示并排面板
func (c *components) ShowSideBySidePanels(left, right PanelData) error {
	// 创建左侧面板
	leftPanel := pterm.DefaultBox.
		WithTitle(left.Title).
		WithTitleTopCenter().
		WithBoxStyle(pterm.NewStyle(c.theme.PrimaryColor)).
		Sprint(left.Content)

	// 创建右侧面板
	rightPanel := pterm.DefaultBox.
		WithTitle(right.Title).
		WithTitleTopCenter().
		WithBoxStyle(pterm.NewStyle(c.theme.SecondaryColor)).
		Sprint(right.Content)

	// 并排显示
	panels, err := pterm.DefaultPanel.WithPanels([][]pterm.Panel{
		{pterm.Panel{Data: leftPanel}},
		{pterm.Panel{Data: rightPanel}},
	}).Srender()

	if err != nil {
		return fmt.Errorf("渲染并排面板失败: %v", err)
	}

	fmt.Println(panels)
	return nil
}

// ShowHeader 显示标题
func (c *components) ShowHeader(text string) error {
	pterm.DefaultHeader.WithBackgroundStyle(pterm.NewStyle(c.theme.PrimaryColor)).
		WithMargin(2).
		Println(text)
	return nil
}

// ShowSection 显示分节
func (c *components) ShowSection(text string) error {
	pterm.DefaultSection.WithStyle(pterm.NewStyle(c.theme.PrimaryColor)).
		Println(text)
	return nil
}

// ShowPermissionStatus 显示权限状态
func (c *components) ShowPermissionStatus(level, status string) error {
	statusBox := pterm.DefaultBox.
		WithTitle("🔐 权限状态").
		WithTitleTopCenter().
		WithBoxStyle(pterm.NewStyle(c.theme.InfoColor))

	content := fmt.Sprintf("权限级别: %s\n状态描述: %s", level, status)
	statusBox.Println(content)
	return nil
}

// ShowSecurityWarning 显示安全警告
func (c *components) ShowSecurityWarning(message string) error {
	warningBox := pterm.DefaultBox.
		WithTitle("🛡️ 安全警告").
		WithTitleTopCenter().
		WithBoxStyle(pterm.NewStyle(c.theme.WarningColor))

	warningBox.Println(message)
	return nil
}

// ShowWalletSelector 显示钱包选择器
func (c *components) ShowWalletSelector(wallets []WalletDisplayInfo) (int, error) {
	if len(wallets) == 0 {
		return -1, fmt.Errorf("没有可用的钱包")
	}

	pterm.DefaultHeader.WithBackgroundStyle(pterm.NewStyle(c.theme.PrimaryColor)).
		Println("选择钱包")

	// 构建选项列表，显示完整地址
	options := make([]string, len(wallets))
	for i, wallet := range wallets {
		lockStatus := "🔓"
		if wallet.IsLocked {
			lockStatus = "🔒"
		}
		options[i] = fmt.Sprintf("%s %s (%s) - %s", lockStatus, wallet.Name, wallet.Address, wallet.Balance)
	}

	selectedIndex, err := c.ShowMenu("", options)
	if err != nil {
		return -1, fmt.Errorf("钱包选择失败: %v", err)
	}

	return selectedIndex, nil
}

// ShowNodeStatus 显示节点状态
func (c *components) ShowNodeStatus(nodeInfo *client.NodeInfo, miningStatus *client.MiningStatus) error {
	if nodeInfo == nil {
		return fmt.Errorf("节点信息为空")
	}

	// 创建节点状态面板
	statusBox := pterm.DefaultBox.
		WithTitle("🔗 节点状态").
		WithTitleTopCenter().
		WithBoxStyle(pterm.NewStyle(c.theme.InfoColor))

	var content string
	content += fmt.Sprintf("节点ID: %s\n", nodeInfo.NodeID)
	content += fmt.Sprintf("版本: %s\n", nodeInfo.Version)
	content += fmt.Sprintf("连接数: %d\n", nodeInfo.PeerCount)
	content += fmt.Sprintf("区块高度: %d\n", nodeInfo.BlockHeight)
	content += fmt.Sprintf("挖矿状态: %s\n", func() string {
		if nodeInfo.IsMining {
			return "🔄 挖矿中"
		}
		return "⏸️ 未挖矿"
	}())

	if miningStatus != nil {
		content += fmt.Sprintf("挖矿状态: %s\n", getConsensusStatusText(*miningStatus))
	}

	statusBox.Println(content)
	return nil
}

// ShowBalanceInfo 显示余额信息
func (c *components) ShowBalanceInfo(address string, balance float64, tokenSymbol string) error {
	balanceBox := pterm.DefaultBox.
		WithTitle("💰 余额信息").
		WithTitleTopCenter().
		WithBoxStyle(pterm.NewStyle(c.theme.SuccessColor))

	content := fmt.Sprintf("地址: %s\n余额: %.6f %s", address, balance, tokenSymbol)
	balanceBox.Println(content)
	return nil
}

// getDefaultTheme 获取默认主题配置
func getDefaultTheme() *ThemeConfig {
	return &ThemeConfig{
		PrimaryColor:   pterm.FgBlue,
		SecondaryColor: pterm.FgCyan,
		SuccessColor:   pterm.FgGreen,
		WarningColor:   pterm.FgYellow,
		ErrorColor:     pterm.FgRed,
		InfoColor:      pterm.FgLightBlue,
	}
}

// progressBarImpl 进度条实现
type progressBarImpl struct {
	title   string
	total   int
	current int
	pbar    *pterm.ProgressbarPrinter
	theme   *ThemeConfig
}

func (p *progressBarImpl) Start() error {
	p.pbar, _ = pterm.DefaultProgressbar.
		WithTitle(p.title).
		WithTotal(p.total).
		WithBarStyle(pterm.NewStyle(p.theme.PrimaryColor)).
		Start()
	return nil
}

func (p *progressBarImpl) Update(current int, message string) error {
	if p.pbar == nil {
		return fmt.Errorf("进度条未启动")
	}
	p.current = current
	p.pbar.UpdateTitle(fmt.Sprintf("%s - %s", p.title, message))
	p.pbar.Current = current
	return nil
}

func (p *progressBarImpl) Increment(message string) error {
	if p.pbar == nil {
		return fmt.Errorf("进度条未启动")
	}
	p.current++
	p.pbar.UpdateTitle(fmt.Sprintf("%s - %s", p.title, message))
	p.pbar.Increment()
	return nil
}

func (p *progressBarImpl) Finish(message string) error {
	if p.pbar == nil {
		return fmt.Errorf("进度条未启动")
	}
	if message != "" {
		p.pbar.UpdateTitle(fmt.Sprintf("%s - %s", p.title, message))
	}
	_, err := p.pbar.Stop()
	return err
}

func (p *progressBarImpl) Stop() error {
	if p.pbar == nil {
		return nil
	}
	_, err := p.pbar.Stop()
	return err
}

// spinnerImpl 加载动画实现
type spinnerImpl struct {
	message string
	spinner *pterm.SpinnerPrinter
	theme   *ThemeConfig
}

func (s *spinnerImpl) Start() error {
	var err error
	s.spinner, err = pterm.DefaultSpinner.
		WithText(s.message).
		WithStyle(pterm.NewStyle(s.theme.PrimaryColor)).
		Start()
	return err
}

func (s *spinnerImpl) UpdateText(text string) error {
	if s.spinner == nil {
		return fmt.Errorf("加载动画未启动")
	}
	s.message = text
	s.spinner.UpdateText(text)
	return nil
}

func (s *spinnerImpl) Stop() error {
	if s.spinner == nil {
		return nil
	}
	return s.spinner.Stop()
}

func (s *spinnerImpl) Success(message string) error {
	if s.spinner == nil {
		return fmt.Errorf("加载动画未启动")
	}
	s.spinner.Success(message)
	return nil
}

// getConsensusStatusText 获取共识状态文本
func getConsensusStatusText(status client.MiningStatus) string {
	if status.IsActive {
		return fmt.Sprintf("🟢 ✅ 正在参与共识 (最后区块: %s)", status.LastBlock)
	}
	return "🔴 ❌ 未参与共识"
}

func (s *spinnerImpl) Fail(message string) error {
	if s.spinner == nil {
		return fmt.Errorf("加载动画未启动")
	}
	s.spinner.Fail(message)
	return nil
}
