// Package screens - MainMenuScreen实现
package screens

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pterm/pterm"
	"golang.org/x/term"

	"github.com/weisyn/v1/internal/cli/commands"
	"github.com/weisyn/v1/internal/cli/layout"
	"github.com/weisyn/v1/internal/cli/ui"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// MainMenuScreen 主菜单屏幕
type MainMenuScreen struct {
	*layout.BaseScreen
	logger        log.Logger
	uiComponents  ui.Components
	accountCmd    *commands.AccountCommands
	transferCmd   *commands.TransferCommands
	blockchainCmd *commands.BlockchainCommands
	miningCmd     *commands.MiningCommands
	nodeCmd       *commands.NodeCommands
}

// NewMainMenuScreen 创建主菜单屏幕
func NewMainMenuScreen(
	logger log.Logger,
	uiComponents ui.Components,
	accountCmd *commands.AccountCommands,
	transferCmd *commands.TransferCommands,
	blockchainCmd *commands.BlockchainCommands,
	miningCmd *commands.MiningCommands,
	nodeCmd *commands.NodeCommands,
) *MainMenuScreen {
	config := layout.ScreenConfig{
		ShowTopBar:    true, // 主菜单显示状态栏
		ShowFooterTip: true,
		FooterTipType: "menu",
		AutoClear:     true,
		Timeout:       0, // 主菜单不设置超时
	}

	return &MainMenuScreen{
		BaseScreen:    layout.NewBaseScreen("main_menu", config),
		logger:        logger,
		uiComponents:  uiComponents,
		accountCmd:    accountCmd,
		transferCmd:   transferCmd,
		blockchainCmd: blockchainCmd,
		miningCmd:     miningCmd,
		nodeCmd:       nodeCmd,
	}
}

// Render 渲染主菜单屏幕
func (s *MainMenuScreen) Render(ctx context.Context) (*layout.ScreenResult, error) {
	for {
		// 检查context取消信号
		select {
		case <-ctx.Done():
			s.logger.Info("收到退出信号，正在停止菜单...")
			return &layout.ScreenResult{Action: "exit"}, ctx.Err()
		default:
			// 继续执行
		}

		// 使用channel处理菜单选择，支持context取消
		type menuResult struct {
			input string
			err   error
		}

		resultChan := make(chan menuResult, 1)

		go func() {
			// 检查是否在TTY环境中
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				// 非TTY环境，使用简单的文本菜单
				input, err := s.showSimpleTextMenu()
				resultChan <- menuResult{input: input, err: err}
				return
			}

			// TTY环境，使用交互式菜单
			input, err := s.showInteractiveMenu()
			resultChan <- menuResult{input: input, err: err}
		}()

		// 等待菜单选择结果或context取消
		var input string
		var err error
		select {
		case result := <-resultChan:
			input = result.input
			err = result.err
		case <-ctx.Done():
			s.logger.Info("菜单选择被中断（Ctrl+C），正在退出...")
			return &layout.ScreenResult{Action: "exit"}, ctx.Err()
		}

		if err != nil {
			s.logger.Errorf("菜单选择失败: %v", err)
			continue // 重新显示菜单
		}

		// 解析选择的功能
		selectedIndex := s.parseMenuSelection(input)

		// 执行选中的功能
		if err := s.executeMenuItem(ctx, selectedIndex); err != nil {
			if err.Error() == "exit" {
				return &layout.ScreenResult{Action: "exit"}, nil
			}
			// 如果是context取消导致的错误，退出循环
			if ctx.Err() != nil {
				s.logger.Info("菜单项执行被中断，正在退出...")
				return &layout.ScreenResult{Action: "exit"}, ctx.Err()
			}
			// 用户友好的错误处理
			s.handleMenuError(err)
		}

		// 继续循环，重新显示菜单
	}
}

// showInteractiveMenu 显示交互式菜单
func (s *MainMenuScreen) showInteractiveMenu() (string, error) {
	// 显示欢迎框，但不重复显示操作提示（ShowMenu会显示）
	pterm.DefaultBox.WithTitle("WES 区块链控制台").WithTitleTopCenter().Println(
		"欢迎使用微迅区块链系统！\n" +
			"选择下方功能开始您的区块链之旅",
	)
	pterm.Println()

	// 使用统一的UI组件渲染菜单
	menuOptions := []string{
		"账户管理    - 查看余额、创建和管理钱包账户",
		"转账操作    - 发送和接收数字资产",
		"挖矿控制    - 参与网络挖矿获得奖励",
		"资源管理    - 部署和管理区块链资源",
		"区块信息    - 查看区块链数据和交易记录",
		"系统中心    - 节点状态和系统设置",
		"使用帮助    - 获取功能说明和操作指南",
		"退出程序    - 安全退出控制台",
	}

	idx, err := s.uiComponents.ShowMenu("", menuOptions) // 不重复显示标题
	if err != nil {
		return "", err
	}
	if idx < 0 || idx >= len(menuOptions) {
		return "", fmt.Errorf("无效的菜单索引")
	}
	return menuOptions[idx], nil
}

// showSimpleTextMenu 显示简单文本菜单（非TTY环境） - 新架构
func (s *MainMenuScreen) showSimpleTextMenu() (string, error) {
	pterm.DefaultBox.WithTitle("WES 区块链控制台").WithTitleTopCenter().Println(
		"欢迎使用微迅区块链系统！\n" +
			"选择下方功能开始您的区块链之旅",
	)
	pterm.Println()

	pterm.Println("功能菜单：")
	pterm.Println("1. 账户管理    - 查看余额、创建和管理钱包账户")
	pterm.Println("2. 转账操作    - 发送和接收数字资产")
	pterm.Println("3. 挖矿控制    - 参与网络挖矿获得奖励")
	pterm.Println("4. 资源管理    - 部署和管理区块链资源")
	pterm.Println("5. 区块信息    - 查看区块链数据和交易记录")
	pterm.Println("6. 系统中心    - 节点状态和系统设置")
	pterm.Println("7. 使用帮助    - 获取功能说明和操作指南")
	pterm.Println("8. 退出程序    - 安全退出控制台")
	pterm.Println()
	pterm.Info.Println("💡 输入对应数字编号并按回车键确认")
	pterm.Print("请输入选项编号 (1-8): ")

	var choice string
	_, err := fmt.Scanf("%s", &choice)
	if err != nil {
		return "", err
	}

	// 将数字选择转换为完整的菜单选项格式
	switch strings.TrimSpace(choice) {
	case "1":
		return "账户管理    - 查看余额、创建和管理钱包账户", nil
	case "2":
		return "转账操作    - 发送和接收数字资产", nil
	case "3":
		return "挖矿控制    - 参与网络挖矿获得奖励", nil
	case "4":
		return "资源管理    - 部署和管理区块链资源", nil
	case "5":
		return "区块信息    - 查看区块链数据和交易记录", nil
	case "6":
		return "系统中心    - 节点状态和系统设置", nil
	case "7":
		return "使用帮助    - 获取功能说明和操作指南", nil
	case "8":
		return "退出程序    - 安全退出控制台", nil
	default:
		return "", fmt.Errorf("无效的选择: %s，请输入1-8", choice)
	}
}

// parseMenuSelection 解析菜单选择 - 新架构
func (s *MainMenuScreen) parseMenuSelection(input string) int {
	// 根据新的菜单架构映射选项 - 支持带描述的菜单项
	menuMap := map[string]int{
		"账户管理    - 查看余额、创建和管理钱包账户": 0,
		"转账操作    - 发送和接收数字资产":      1,
		"挖矿控制    - 参与网络挖矿获得奖励":     2,
		"资源管理    - 部署和管理区块链资源":     3,
		"区块信息    - 查看区块链数据和交易记录":   4,
		"系统中心    - 节点状态和系统设置":      5,
		"使用帮助    - 获取功能说明和操作指南":    6,
		"退出程序    - 安全退出控制台":        7,
	}

	if index, ok := menuMap[input]; ok {
		return index
	}
	// 如果映射失败，记录错误但仍返回默认选项
	s.logger.Warnf("未识别的菜单选项: %s，默认使用应用能力", input)
	return 0 // 默认返回应用能力
}

// executeMenuItem 执行菜单项 - 新架构
func (s *MainMenuScreen) executeMenuItem(ctx context.Context, selectedIndex int) error {
	s.logger.Debugf("执行菜单项: %d", selectedIndex)

	switch selectedIndex {
	case 0: // 账户管理
		return s.accountCmd.ShowAccountMenu(ctx)
	case 1: // 转账操作
		return s.transferCmd.ShowTransferMenu(ctx)
	case 2: // 挖矿控制
		return s.miningCmd.ShowMiningMenu(ctx)
	case 3: // 资源管理
		return s.showResourceManagementMenu(ctx)
	case 4: // 区块信息
		return s.blockchainCmd.ShowBlockchainMenu(ctx)
	case 5: // 系统中心
		return s.showSystemCenterMenu(ctx)
	case 6: // 使用帮助
		s.uiComponents.ShowInfo("使用帮助功能正在开发中...")
		ui.ShowStandardWaitPrompt("return_menu")
		return nil
	case 7: // 退出程序
		s.showExitMessage()
		return fmt.Errorf("exit")
	default:
		return fmt.Errorf("无效的菜单选择: %d", selectedIndex)
	}
}

// handleMenuError 处理菜单错误
func (s *MainMenuScreen) handleMenuError(err error) {
	s.logger.Errorf("菜单项执行失败: %v", err)
	pterm.Error.Printf("操作失败: %v\n", err)
	pterm.Println()
	ui.ShowStandardWaitPrompt("return_menu")
}

// showExitMessage 显示退出消息
func (s *MainMenuScreen) showExitMessage() {
	pterm.DefaultHeader.WithBackgroundStyle(pterm.NewStyle(pterm.FgYellow)).
		WithMargin(1).
		Println("👋 感谢使用WES")

	exitMessage := `
🌟 感谢您使用微迅(weisyn)区块链节点！

📊 本次会话统计：
• 系统运行正常
• 所有服务已安全关闭

💡 下次启动：
运行 './weisyn development' 重新启动开发环境

🔗 社区支持：
• 官方网站: https://weisyn.org
• 技术文档: https://docs.weisyn.org
• 问题反馈: https://github.com/weisyn/issues

祝您使用愉快！ 🚀
	`

	pterm.DefaultBox.
		WithTitle("再见！").
		WithTitleTopCenter().
		WithBoxStyle(pterm.NewStyle(pterm.FgYellow)).
		Println(exitMessage)
}

// OnEnter 进入主菜单屏幕时的准备工作
func (s *MainMenuScreen) OnEnter(ctx context.Context) error {
	s.logger.Info("进入主菜单屏幕")
	return nil
}

// OnExit 退出主菜单屏幕时的清理工作
func (s *MainMenuScreen) OnExit(ctx context.Context) error {
	s.logger.Info("退出主菜单屏幕")
	return nil
}

// showApplicationsMenu 显示应用能力菜单 - 核心业务功能
func (s *MainMenuScreen) showApplicationsMenu(ctx context.Context) error {
	options := []string{
		"账户管理 - 查看余额、创建账户",
		"转账操作 - 发送和接收代币",
		"区块信息 - 查看区块链数据",
		"挖矿控制 - 参与网络获得奖励",
		"返回主菜单",
	}

	selectedIndex, err := s.uiComponents.ShowMenu("应用能力", options)
	if err != nil {
		return err
	}

	switch selectedIndex {
	case 0:
		return s.accountCmd.ShowAccountMenu(ctx)
	case 1:
		return s.transferCmd.InteractiveTransfer(ctx)
	case 2:
		return s.blockchainCmd.ShowLatestBlocks(ctx)
	case 3:
		return s.miningCmd.ShowMiningMenu(ctx)
	case 4:
		return nil
	default:
		return fmt.Errorf("无效选择")
	}
}

// showSystemCenterMenu 显示系统中心菜单 - 系统管理功能
func (s *MainMenuScreen) showSystemCenterMenu(ctx context.Context) error {
	options := []string{
		"节点信息 - 网络连接状态",
		"系统状态 - 运行状态监控",
		"系统设置 - 配置管理",
		"返回主菜单",
	}

	selectedIndex, err := s.uiComponents.ShowMenu("系统中心", options)
	if err != nil {
		return err
	}

	switch selectedIndex {
	case 0:
		return s.nodeCmd.ShowNodeMenu(ctx)
	case 1:
		return s.nodeCmd.ShowStatus(ctx)
	case 2:
		s.uiComponents.ShowInfo("系统设置功能正在开发中...")
		ui.ShowStandardWaitPrompt("return_menu")
		return nil
	case 3:
		return nil
	default:
		return fmt.Errorf("无效选择")
	}
}

// showResourceManagementMenu 显示资源管理菜单 - 资源相关功能
func (s *MainMenuScreen) showResourceManagementMenu(ctx context.Context) error {
	options := []string{
		"部署静态资源 - 上传文件到区块链",
		"下载静态资源 - 获取已上传的文件",
		"部署智能合约 - 发布合约程序",
		"调用智能合约 - 执行合约功能",
		"部署AI模型 - 上传机器学习模型",
		"执行AI推理 - 运行AI计算",
		"返回主菜单",
	}

	selectedIndex, err := s.uiComponents.ShowMenu("资源管理", options)
	if err != nil {
		return err
	}

	switch selectedIndex {
	case 0:
		return s.deployStaticResource(ctx)
	case 1:
		return s.fetchStaticResource(ctx)
	case 2:
		return s.deployContract(ctx)
	case 3:
		return s.callContract(ctx)
	case 4:
		return s.deployAIModel(ctx)
	case 5:
		return s.executeAIInference(ctx)
	case 6:
		return nil
	default:
		return fmt.Errorf("无效选择")
	}
}

// deployStaticResource 部署静态资源 - 真实交互功能
func (s *MainMenuScreen) deployStaticResource(ctx context.Context) error {
	s.uiComponents.ShowInfo("静态资源部署功能")

	// 收集部署参数
	privateKeyStr, err := s.uiComponents.ShowInputDialog("输入密码", "部署者私钥 (64位十六进制):", true)
	if err != nil {
		return err
	}

	filePath, err := s.uiComponents.ShowInputDialog("输入", "本地文件路径:", false)
	if err != nil {
		return err
	}

	resourceName, err := s.uiComponents.ShowInputDialog("输入", "资源显示名称:", false)
	if err != nil {
		return err
	}

	s.uiComponents.ShowInfo(fmt.Sprintf("✅ 模拟部署成功\n私钥: %s\n文件: %s\n名称: %s\n\n💡 功能正在完善中，敬请期待",
		privateKeyStr, filePath, resourceName))
	ui.ShowStandardWaitPrompt("return_menu")
	return nil
}

// fetchStaticResource 获取静态资源 - 真实交互功能
func (s *MainMenuScreen) fetchStaticResource(ctx context.Context) error {
	s.uiComponents.ShowInfo("静态资源获取功能")

	contentHashStr, err := s.uiComponents.ShowInputDialog("输入", "资源内容哈希:", false)
	if err != nil {
		return err
	}

	targetDir, err := s.uiComponents.ShowInputDialog("输入", "保存目录 (留空使用默认):", false)
	if err != nil {
		return err
	}

	s.uiComponents.ShowInfo(fmt.Sprintf("✅ 模拟获取成功\n哈希: %s\n保存: %s\n\n💡 功能正在完善中，敬请期待",
		contentHashStr, targetDir))
	ui.ShowStandardWaitPrompt("return_menu")
	return nil
}

// deployContract 部署智能合约 - 真实交互功能
func (s *MainMenuScreen) deployContract(ctx context.Context) error {
	s.uiComponents.ShowInfo("智能合约部署功能")

	wasmPath, err := s.uiComponents.ShowInputDialog("输入", "WASM合约文件路径:", false)
	if err != nil {
		return err
	}

	contractName, err := s.uiComponents.ShowInputDialog("输入", "合约名称:", false)
	if err != nil {
		return err
	}

	s.uiComponents.ShowInfo(fmt.Sprintf("✅ 模拟部署成功\n合约文件: %s\n名称: %s\n\n💡 功能正在完善中，敬请期待",
		wasmPath, contractName))
	ui.ShowStandardWaitPrompt("return_menu")
	return nil
}

// callContract 调用智能合约 - 真实交互功能
func (s *MainMenuScreen) callContract(ctx context.Context) error {
	s.uiComponents.ShowInfo("智能合约调用功能")

	contractAddress, err := s.uiComponents.ShowInputDialog("输入", "合约地址:", false)
	if err != nil {
		return err
	}

	methodName, err := s.uiComponents.ShowInputDialog("输入", "方法名:", false)
	if err != nil {
		return err
	}

	s.uiComponents.ShowInfo(fmt.Sprintf("✅ 模拟调用成功\n合约地址: %s\n方法名: %s\n\n💡 功能正在完善中，敬请期待",
		contractAddress, methodName))
	ui.ShowStandardWaitPrompt("return_menu")
	return nil
}

// deployAIModel 部署AI模型 - 真实交互功能
func (s *MainMenuScreen) deployAIModel(ctx context.Context) error {
	s.uiComponents.ShowInfo("AI模型部署功能")

	modelPath, err := s.uiComponents.ShowInputDialog("输入", "ONNX模型文件路径:", false)
	if err != nil {
		return err
	}

	modelName, err := s.uiComponents.ShowInputDialog("输入", "模型名称:", false)
	if err != nil {
		return err
	}

	s.uiComponents.ShowInfo(fmt.Sprintf("✅ 模拟部署成功\n模型文件: %s\n名称: %s\n\n💡 功能正在完善中，敬请期待",
		modelPath, modelName))
	ui.ShowStandardWaitPrompt("return_menu")
	return nil
}

// executeAIInference 执行AI推理 - 真实交互功能
func (s *MainMenuScreen) executeAIInference(ctx context.Context) error {
	s.uiComponents.ShowInfo("AI推理执行功能")

	modelHash, err := s.uiComponents.ShowInputDialog("输入", "模型内容哈希:", false)
	if err != nil {
		return err
	}

	inputData, err := s.uiComponents.ShowInputDialog("输入", "输入数据 (JSON格式):", false)
	if err != nil {
		return err
	}

	s.uiComponents.ShowInfo(fmt.Sprintf("✅ 模拟推理成功\n模型标识: %s\n输入数据: %s\n计算结果: [模拟推理结果]\n\n💡 功能正在完善中，敬请期待",
		modelHash, inputData))
	ui.ShowStandardWaitPrompt("return_menu")
	return nil
}
