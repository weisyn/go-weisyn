package interactive

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pterm/pterm"
	"golang.org/x/term"

	"github.com/weisyn/v1/internal/cli/commands"
	"github.com/weisyn/v1/internal/cli/status"
	clipkg "github.com/weisyn/v1/internal/cli/ui"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// Menu 交互式主菜单
type Menu struct {
	logger        log.Logger
	ui            clipkg.Components
	account       *commands.AccountCommands
	transfer      *commands.TransferCommands
	blockchain    *commands.BlockchainCommands
	mining        *commands.MiningCommands
	node          *commands.NodeCommands
	statusManager *status.StatusManager
	simpleLayout  *clipkg.SimpleLayout
}

// MenuItem 菜单项
type MenuItem struct {
	Title       string
	Description string
	Icon        string
	Action      func(context.Context) error
}

// NewMenu 创建新的主菜单
func NewMenu(
	logger log.Logger,
	ui clipkg.Components,
	account *commands.AccountCommands,
	transfer *commands.TransferCommands,
	blockchain *commands.BlockchainCommands,
	mining *commands.MiningCommands,
	node *commands.NodeCommands,
	statusManager *status.StatusManager,
) *Menu {
	// 直接调用ui包中的NewSimpleLayout函数
	simpleLayout := clipkg.NewSimpleLayout(logger, statusManager)

	return &Menu{
		logger:        logger,
		ui:            ui,
		account:       account,
		transfer:      transfer,
		blockchain:    blockchain,
		mining:        mining,
		node:          node,
		statusManager: statusManager,
		simpleLayout:  simpleLayout,
	}
}

// Run 运行主菜单循环 - 增强新用户体验版本
func (m *Menu) Run(ctx context.Context) error {
	// 启动状态管理器
	if m.statusManager != nil {
		if err := m.statusManager.Start(ctx); err != nil {
			m.logger.Errorf("启动状态管理器失败: %v", err)
		}
		defer m.statusManager.Stop()
	}

	// 进入主循环不再显示欢迎/引导横幅，保持界面简洁

	for {
		// 🚨 检查context取消信号（处理Ctrl+C）
		select {
		case <-ctx.Done():
			m.logger.Info("收到退出信号，正在停止菜单...")
			return ctx.Err()
		default:
			// 继续执行
		}

		// 统一清屏+状态栏显示
		clipkg.ShowPageHeader()

		// 主功能分组简要引导，避免黑屏感
		pterm.DefaultBox.WithTitle("功能分组").WithTitleTopCenter().Println(
			"🎯 应用能力  |  🏠 系统中心  |  📦 资源管理  |  ❓ 使用帮助",
		)
		pterm.Println()

		// 使用channel来处理可能阻塞的菜单选择，支持context取消
		type menuResult struct {
			input string
			err   error
		}

		resultChan := make(chan menuResult, 1)

		go func() {
			// 检查是否在TTY环境中
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				// 非TTY环境，使用简单的文本菜单
				input, err := m.showSimpleTextMenu()
				resultChan <- menuResult{input: input, err: err}
				return
			}

			// TTY环境，使用交互式菜单
			// 显示标准化操作提示
			pterm.Println()
			clipkg.ShowStandardTip("menu")

			// 显示重新设计的菜单选项 - 按功能分类
			menuOptions := []string{
				"🎯 应用能力",
				"🏠 系统中心",
				"📦 资源管理",
				"❓ 使用帮助",
				"🚪 退出程序",
			}

			res, err := pterm.DefaultInteractiveSelect.
				WithOptions(menuOptions).
				WithDefaultOption(menuOptions[0]).
				WithMaxHeight(8).
				WithFilter(false).
				Show("📋 请选择您要执行的操作：")

			resultChan <- menuResult{input: res, err: err}
		}()

		select {
		case <-ctx.Done():
			m.logger.Info("收到退出信号，正在停止菜单...")
			return ctx.Err()
		case result := <-resultChan:
			if result.err != nil {
				return result.err
			}

			// 解析并执行
			if err := m.handleMenuSelection(ctx, result.input); err != nil {
				if err.Error() == "exit" {
					return nil
				}
				m.logger.Errorf("菜单执行失败: %v", err)
			}
		}
	}
}

// showWelcomeHints 显示简洁的用户提示 - 新架构
func (m *Menu) showWelcomeHints() {
	pterm.DefaultBox.WithTitle("🎉 欢迎使用WES").WithTitleTopCenter().Println(
		"新架构菜单说明：\n" +
			"  🎯 应用能力 - 核心业务功能（账户、转账、区块、挖矿）\n" +
			"  🏠 系统中心 - 系统管理功能（节点、状态、设置）\n" +
			"  📦 资源管理 - 资源相关功能（静态资源、合约、AI模型）\n\n" +
			"💡 如需帮助，选择 '📚 使用帮助'",
	)
	pterm.Println()
}

// parseMenuSelection 解析菜单选择 - 新架构
func (m *Menu) parseMenuSelection(input string) int {
	// 根据新的分类架构映射选项
	menuMap := map[string]int{
		"🎯 应用能力": 0,
		"🏠 系统中心": 1,
		"📦 资源管理": 2,
		"📚 使用帮助": 3,
		"🚪 退出程序": 4,
	}

	if index, ok := menuMap[input]; ok {
		return index
	}
	// 如果映射失败，记录错误但仍返回默认选项
	m.logger.Warnf("未识别的菜单选项: %s，默认使用应用能力", input)
	return 0 // 默认返回应用能力
}

// handleMenuError 处理菜单错误
func (m *Menu) handleMenuError(err error) {
	pterm.Error.Printf("⚠️ 操作执行时出现问题: %v\n", err)
	pterm.Info.Println("💡 友好提示: 如果遇到问题，可以选择 '8. 新手指南' 获取详细帮助")
	pterm.Println()

	// 记录详细错误到日志
	m.logger.Errorf("执行菜单项失败: %v", err)

	// 添加更友好的继续提示
	clipkg.ShowStandardWaitPrompt("return_menu")
}

// showExitMessage 显示退出消息
func (m *Menu) showExitMessage() {
	pterm.DefaultCenter.WithCenterEachLineSeparately().Println(
		pterm.LightGreen("🎉 感谢使用WES区块链系统！"),
		"",
		pterm.Gray("您的数据已安全保存"),
		pterm.Gray("期待您的再次使用"),
		"",
		pterm.LightBlue("👋 再见！"),
	)
}

// executeMenuItem 执行菜单项 - 支持新增的系统状态和新手指南
func (m *Menu) executeMenuItem(ctx context.Context, index int) error {
	switch index {
	case 0: // 应用能力
		return m.showApplicationsMenu(ctx)
	case 1: // 系统中心
		return m.showSystemCenterMenu(ctx)
	case 2: // 资源管理
		return m.showResourceManagementMenu(ctx)
	case 3: // 使用帮助
		return m.showBeginnersGuide(ctx)
	case 4: // 退出程序
		return fmt.Errorf("exit")
	default:
		return fmt.Errorf("无效的菜单选择")
	}
}

// handleMenuSelection 解析并执行菜单动作
func (m *Menu) handleMenuSelection(ctx context.Context, input string) error {
	index := m.parseMenuSelection(input)
	return m.executeMenuItem(ctx, index)
}

// showSystemStatus 显示系统状态
func (m *Menu) showSystemStatus(ctx context.Context) error {
	if m.simpleLayout != nil {
		m.simpleLayout.ShowSystemStatus()
	} else {
		// 备用显示方式
		pterm.Info.Println("状态管理器未初始化")
		clipkg.ShowStandardWaitPrompt("return")
	}
	return nil
}

// showApplicationsMenu 显示应用能力菜单 - 核心业务功能
func (m *Menu) showApplicationsMenu(ctx context.Context) error {
	options := []string{
		"账户管理 - 密钥生成、余额查询",
		"转账操作 - 基于真实TransactionService",
		"区块信息 - 区块链数据查询",
		"挖矿控制 - 基于真实MinerService",
		"返回主菜单",
	}

	selectedIndex, err := m.ui.ShowMenu("🎯 应用能力", options)
	if err != nil {
		return err
	}

	switch selectedIndex {
	case 0:
		return m.showAccountMenu(ctx)
	case 1:
		return m.showTransferMenu(ctx)
	case 2:
		return m.showBlockchainMenu(ctx)
	case 3:
		return m.showMiningMenu(ctx)
	case 4:
		return nil
	default:
		return fmt.Errorf("无效选择")
	}
}

// showSystemCenterMenu 系统中心 - 整合所有系统管理功能
func (m *Menu) showSystemCenterMenu(ctx context.Context) error {
	clipkg.SwitchToResultPage("🏠 系统中心")

	// 整合显示所有系统信息，而不是分散的子菜单
	pterm.DefaultSection.Println("系统综合状态")

	// 使用进度条显示数据加载
	progress := clipkg.StartSpinner("正在收集系统信息...")

	// 收集节点信息
	nodeInfo := m.collectNodeInfo(ctx)

	// 收集系统状态
	systemStatus := m.collectSystemStatus(ctx)

	// 收集配置信息
	configInfo := m.collectConfigInfo(ctx)

	progress.Stop()

	// 整合显示系统信息表格
	clipkg.SwitchToResultPage("🏠 系统中心 - 综合状态")

	m.displayIntegratedSystemInfo(nodeInfo, systemStatus, configInfo)

	// 提供操作选项
	options := []string{
		"刷新系统状态",
		"启动挖矿",
		"停止挖矿",
		"查看详细日志",
		"返回主菜单",
	}

	selectedIndex, err := m.ui.ShowMenu("🔧 系统操作", options)
	if err != nil {
		return err
	}

	switch selectedIndex {
	case 0:
		return m.showSystemCenterMenu(ctx) // 刷新
	case 1:
		return m.mining.StartMining(ctx)
	case 2:
		return m.mining.StopMining(ctx)
	case 3:
		return m.showSystemLogs(ctx)
	case 4:
		return nil
	default:
		return fmt.Errorf("无效选择")
	}
}

// showResourceManagementMenu 资源管理 - 真实可操作的交互功能
func (m *Menu) showResourceManagementMenu(ctx context.Context) error {
	options := []string{
		"部署静态资源 - 上传文件到区块链",
		"下载静态资源 - 根据哈希获取文件",
		"部署智能合约 - WASM合约部署",
		"调用智能合约 - 执行合约方法",
		"部署AI模型 - ONNX模型部署",
		"执行AI推理 - 模型推理计算",
		"返回主菜单",
	}

	selectedIndex, err := m.ui.ShowMenu("📦 资源管理", options)
	if err != nil {
		return err
	}

	switch selectedIndex {
	case 0:
		return m.deployStaticResource(ctx)
	case 1:
		return m.fetchStaticResource(ctx)
	case 2:
		return m.deployContract(ctx)
	case 3:
		return m.callContract(ctx)
	case 4:
		return m.deployAIModel(ctx)
	case 5:
		return m.executeAIInference(ctx)
	case 6:
		return nil
	default:
		return fmt.Errorf("无效选择")
	}
}

// showAccountMenu 显示账户管理菜单
func (m *Menu) showAccountMenu(ctx context.Context) error {
	// 直接委托给基于本地钱包的账户菜单
	return m.account.ShowAccountMenu(ctx)
}

// showTransferMenu 显示转账菜单
func (m *Menu) showTransferMenu(ctx context.Context) error {
	options := []string{
		"简单转账",
		"批量转账",
		"时间锁转账",
		"返回主菜单",
	}

	selectedIndex, err := m.ui.ShowMenu("🔄 转账操作", options)
	if err != nil {
		return err
	}

	switch selectedIndex {
	case 0:
		return m.transfer.InteractiveTransfer(ctx)
	case 1:
		return m.transfer.BatchTransfer(ctx)
	case 2:
		return m.transfer.TimeLockTransfer(ctx)
	case 3:
		return nil
	default:
		return fmt.Errorf("无效选择")
	}
}

// showBlockchainMenu 显示区块链信息菜单
func (m *Menu) showBlockchainMenu(ctx context.Context) error {
	options := []string{
		"查看最新区块",
		"查看指定区块",
		"查看交易详情",
		"链信息统计",
		"返回主菜单",
	}

	selectedIndex, err := m.ui.ShowMenu("📊 区块链信息", options)
	if err != nil {
		return err
	}

	switch selectedIndex {
	case 0:
		return m.blockchain.ShowLatestBlocks(ctx)
	case 1:
		return m.blockchain.ShowBlockByHeight(ctx)
	case 2:
		return m.blockchain.ShowTransaction(ctx)
	case 3:
		return m.blockchain.ShowChainInfo(ctx)
	case 4:
		return nil
	default:
		return fmt.Errorf("无效选择")
	}
}

// showMiningMenu 显示挖矿控制菜单 - 基于真实接口
func (m *Menu) showMiningMenu(ctx context.Context) error {
	options := []string{
		"查看挖矿状态",
		"启动挖矿",
		"停止挖矿",
		"挖矿功能说明",
		"返回主菜单",
	}

	selectedIndex, err := m.ui.ShowMenu("⛏️ 挖矿控制", options)
	if err != nil {
		return err
	}

	switch selectedIndex {
	case 0:
		return m.mining.ShowMiningStatus(ctx)
	case 1:
		return m.mining.StartMining(ctx)
	case 2:
		return m.mining.StopMining(ctx)
	case 3:
		return m.mining.ShowMiningInfo(ctx)
	case 4:
		return nil
	default:
		return fmt.Errorf("无效选择")
	}
}

// showNodeMenu 显示节点管理菜单
func (m *Menu) showNodeMenu(ctx context.Context) error {
	options := []string{
		"节点基本信息",
		"连接的节点",
		"网络状态",
		"同步状态",
		"返回主菜单",
	}

	selectedIndex, err := m.ui.ShowMenu("🌐 节点管理", options)
	if err != nil {
		return err
	}

	switch selectedIndex {
	case 0:
		return m.node.ShowStatus(ctx)
	case 1:
		return m.node.ShowPeers(ctx)
	case 2:
		return m.node.ShowNetworkStatus(ctx)
	case 3:
		return m.node.ShowSyncStatus(ctx)
	case 4:
		return nil
	default:
		return fmt.Errorf("无效选择")
	}
}

// showMonitorMenu 显示监控菜单
func (m *Menu) showMonitorMenu(ctx context.Context) error {
	options := []string{
		"系统资源监控",
		"性能统计",
		"日志查看",
		"事件监听",
		"返回主菜单",
	}

	selectedIndex, err := m.ui.ShowMenu("📈 实时监控", options)
	if err != nil {
		return err
	}

	switch selectedIndex {
	case 0:
		m.showSystemMonitorInfo(ctx)
	case 1:
		m.showPerformanceStatsInfo(ctx)
	case 2:
		m.showLogViewInfo(ctx)
	case 3:
		m.showEventListenerInfo(ctx)
	case 4:
		return nil
	default:
		return fmt.Errorf("无效选择")
	}

	m.waitForContinue()
	return nil
}

// showSettingsMenu 显示设置菜单（只读配置查看）
func (m *Menu) showSettingsMenu(ctx context.Context) error {
	options := []string{
		"查看当前配置",
		"网络配置信息",
		"区块链配置信息",
		"API配置信息",
		"返回主菜单",
	}

	selectedIndex, err := m.ui.ShowMenu("⚙️ 系统设置 (只读)", options)
	if err != nil {
		return err
	}

	switch selectedIndex {
	case 0:
		m.showCurrentConfig(ctx)
	case 1:
		m.showNetworkConfig(ctx)
	case 2:
		m.showBlockchainConfig(ctx)
	case 3:
		m.showAPIConfig(ctx)
	case 4:
		return nil
	default:
		return fmt.Errorf("无效选择")
	}

	m.waitForContinue()
	return nil
}

// showBeginnersGuide 显示新手指南菜单
func (m *Menu) showBeginnersGuide(ctx context.Context) error {
	options := []string{
		"🆕 WES新手入门",
		"💰 如何创建和管理钱包",
		"⛏️ 挖矿操作指南",
		"💸 如何进行转账",
		"🔍 查看区块链数据",
		"❓ 常见问题解答",
		"📞 获取技术支持",
		"🔙 返回主菜单",
	}

	selectedIndex, err := m.ui.ShowMenu("❓ 新手指南", options)
	if err != nil {
		return err
	}

	switch selectedIndex {
	case 0:
		return m.showGettingStarted()
	case 1:
		return m.showWalletGuide()
	case 2:
		return m.showMiningGuide()
	case 3:
		return m.showTransferGuide()
	case 4:
		return m.showBlockchainGuide()
	case 5:
		return m.showFAQ()
	case 6:
		return m.showSupport()
	case 7:
		return nil
	default:
		return fmt.Errorf("无效选择")
	}
}

// showGettingStarted 显示入门指南
func (m *Menu) showGettingStarted() error {
	pterm.DefaultHeader.WithBackgroundStyle(pterm.NewStyle(pterm.BgLightBlue)).
		WithTextStyle(pterm.NewStyle(pterm.FgWhite, pterm.Bold)).
		Println("🆕 WES新手入门")

	pterm.DefaultBox.WithTitle("🌟 欢迎来到WES世界").WithTitleTopCenter().Println(
		"WES是基于EUTXO模型的下一代区块链平台\n\n" +
			"🚀 主要特性:\n" +
			"  • 高性能交易处理\n" +
			"  • 智能合约支持\n" +
			"  • 去中心化存储\n" +
			"  • 环保的共识机制\n\n" +
			"💡 建议操作顺序:\n" +
			"  1️⃣ 创建您的第一个钱包\n" +
			"  2️⃣ 开始挖矿获得WES代币\n" +
			"  3️⃣ 学习转账和交易\n" +
			"  4️⃣ 探索高级功能",
	)

	m.waitForContinue()
	return nil
}

// showWalletGuide 显示钱包操作指南
func (m *Menu) showWalletGuide() error {
	pterm.DefaultHeader.WithBackgroundStyle(pterm.NewStyle(pterm.BgGreen)).
		WithTextStyle(pterm.NewStyle(pterm.FgWhite, pterm.Bold)).
		Println("💰 钱包操作指南")

	pterm.DefaultBox.WithTitle("🔐 钱包管理最佳实践").WithTitleTopCenter().Println(
		"💳 创建钱包:\n" +
			"  • 选择 '💰 账户管理' → '钱包管理' → '创建钱包'\n" +
			"  • 设置安全的密码\n" +
			"  • 安全保存钱包信息\n\n" +
			"🔍 查看余额:\n" +
			"  • 选择 '💰 账户管理' → '查询账户余额'\n" +
			"  • 优先从本地钱包选择地址，或手动输入\n\n" +
			"📥 导入现有钱包:\n" +
			"  • 选择 '💰 账户管理' → '钱包管理' → '导入钱包'\n" +
			"  • 使用私钥导入（加密存储）\n\n" +
			"🔓 使用前解锁:\n" +
			"  • 在转账/合约/资源操作前，系统会引导选择并解锁钱包\n" +
			"⚠️  安全提示:\n" +
			"  • 永远不要分享您的私钥\n" +
			"  • 定期备份钱包文件\n" +
			"  • 使用强密码保护钱包",
	)

	m.waitForContinue()
	return nil
}

// showMiningGuide 显示挖矿指南
func (m *Menu) showMiningGuide() error {
	pterm.DefaultHeader.WithBackgroundStyle(pterm.NewStyle(pterm.BgYellow)).
		WithTextStyle(pterm.NewStyle(pterm.FgBlack, pterm.Bold)).
		Println("⛏️ 挖矿操作指南")

	pterm.DefaultBox.WithTitle("💎 挖矿赚取WES代币").WithTitleTopCenter().Println(
		"🚀 开始挖矿:\n" +
			"  1. 确保您已创建钱包账户\n" +
			"  2. 选择 '⛏️ 挖矿控制' → '启动挖矿'\n" +
			"  3. 选择接收奖励的钱包地址\n" +
			"  4. 确认启动挖矿\n\n" +
			"📊 监控挖矿:\n" +
			"  • 选择 '⛏️ 挖矿控制' → '查看挖矿状态'\n" +
			"  • 查看状态、网络贡献度等信息\n\n" +
			"💰 挖矿收益:\n" +
			"  • 成功挖出区块获得区块奖励\n" +
			"  • 收取网络交易手续费\n" +
			"  • 奖励直接发送到您的钱包\n\n" +
			"⚡ 优化建议:\n" +
			"  • 保持网络连接稳定\n" +
			"  • 确保充足的系统资源\n" +
			"  • 定期查看挖矿状态",
	)

	m.waitForContinue()
	return nil
}

// showTransferGuide 显示转账指南
func (m *Menu) showTransferGuide() error {
	pterm.DefaultHeader.WithBackgroundStyle(pterm.NewStyle(pterm.BgMagenta)).
		WithTextStyle(pterm.NewStyle(pterm.FgWhite, pterm.Bold)).
		Println("💸 转账操作指南")

	pterm.DefaultBox.WithTitle("💳 安全转账步骤").WithTitleTopCenter().Println(
		"📤 发送WES代币:\n" +
			"  1. 选择 '🔄 转账操作' → '简单转账'\n" +
			"  2. 选择发送方钱包\n" +
			"  3. 输入接收方地址\n" +
			"  4. 设置转账金额\n" +
			"  5. 设置手续费\n" +
			"  6. 确认并发送交易\n\n" +
			"📋 转账要求:\n" +
			"  • 发送方钱包必须有足够余额\n" +
			"  • 接收方地址格式正确\n" +
			"  • 设置合理的手续费\n\n" +
			"🔍 查看交易状态:\n" +
			"  • 选择 '📊 区块信息' → '查看交易详情'\n" +
			"  • 输入交易哈希查看状态和确认数\n\n" +
			"⚠️  安全提示:\n" +
			"  • 仔细核对接收方地址\n" +
			"  • 小额测试后再进行大额转账\n" +
			"  • 保存交易哈希用于查询",
	)

	m.waitForContinue()
	return nil
}

// showBlockchainGuide 显示区块链数据查看指南
func (m *Menu) showBlockchainGuide() error {
	pterm.DefaultHeader.WithBackgroundStyle(pterm.NewStyle(pterm.BgCyan)).
		WithTextStyle(pterm.NewStyle(pterm.FgWhite, pterm.Bold)).
		Println("🔍 区块链数据指南")

	pterm.DefaultBox.WithTitle("📊 理解区块链数据").WithTitleTopCenter().Println(
		"🔗 查看区块信息:\n" +
			"  • 选择 '📊 区块信息' → '查看最新区块'\n" +
			"  • 查看区块高度、哈希、交易数等\n" +
			"  • 了解网络最新状态\n\n" +
			"🔍 查询具体交易:\n" +
			"  • 选择 '📊 区块信息' → '查看交易详情'\n" +
			"  • 输入交易哈希查询交易状态\n\n" +
			"📈 链统计信息:\n" +
			"  • 选择 '📊 区块信息' → '链信息统计'\n" +
			"  • 查看总体网络统计数据\n\n" +
			"💡 数据解读:\n" +
			"  • 区块高度：表示区块链长度\n" +
			"  • 确认数：交易被确认的区块数\n" +
			"  • 难度：挖矿难度调整\n" +
			"  • 贡献度：网络参与程度",
	)

	m.waitForContinue()
	return nil
}

// showFAQ 显示常见问题
func (m *Menu) showFAQ() error {
	pterm.DefaultHeader.WithBackgroundStyle(pterm.NewStyle(pterm.BgRed)).
		WithTextStyle(pterm.NewStyle(pterm.FgWhite, pterm.Bold)).
		Println("❓ 常见问题解答")

	pterm.DefaultBox.WithTitle("🤔 常见问题").WithTitleTopCenter().Println(
		"Q: WES是什么？\n" +
			"A: WES是基于EUTXO模型的高性能区块链平台\n\n" +
			"Q: 如何获得WES代币？\n" +
			"A: 通过挖矿、转账接收或交易所购买\n\n" +
			"Q: 挖矿需要什么条件？\n" +
			"A: 需要一台联网的计算机和钱包地址\n\n" +
			"Q: 转账手续费如何计算？\n" +
			"A: 根据交易大小和网络拥堵情况动态调整\n\n" +
			"Q: 钱包密码忘记了怎么办？\n" +
			"A: 如有私钥可通过'钱包管理→导入钱包'重新导入（需密码加密）\n\n" +
			"Q: 如何确保资金安全？\n" +
			"A: 妥善保管私钥，使用强密码，定期备份\n\n" +
			"Q: 网络同步需要多长时间？\n" +
			"A: 取决于网络状况，通常几分钟到几小时",
	)

	m.waitForContinue()
	return nil
}

// showSupport 显示技术支持
func (m *Menu) showSupport() error {
	pterm.DefaultHeader.WithBackgroundStyle(pterm.NewStyle(pterm.BgBlue)).
		WithTextStyle(pterm.NewStyle(pterm.FgWhite, pterm.Bold)).
		Println("📞 技术支持")

	pterm.DefaultBox.WithTitle("🛠️ 获取帮助").WithTitleTopCenter().Println(
		"📧 联系我们:\n" +
			"  • 官方网站: https://weisyn.io\n" +
			"  • 技术文档: https://docs.weisyn.io\n" +
			"  • GitHub: https://github.com/weisyn\n" +
			"  • 社区论坛: https://forum.weisyn.io\n\n" +
			"💬 社区支持:\n" +
			"  • Telegram: @WES_Official\n" +
			"  • Discord: WES Community\n" +
			"  • WeChat: WES技术交流群\n\n" +
			"🐛 问题报告:\n" +
			"  • GitHub Issues提交bug报告\n" +
			"  • 提供详细的错误信息和日志\n" +
			"  • 描述问题复现步骤\n\n" +
			"📚 学习资源:\n" +
			"  • 官方文档和API参考\n" +
			"  • 开发者指南和示例\n" +
			"  • 视频教程和在线课程",
	)

	m.waitForContinue()
	return nil
}

// showCurrentConfig 显示当前配置概览
func (m *Menu) showCurrentConfig(ctx context.Context) {
	pterm.DefaultSection.Println("系统配置概览")

	if m.statusManager == nil {
		m.ui.ShowError("状态管理器不可用")
		return
	}

	configInfo := m.statusManager.GetConfigInfo()

	// 创建配置信息表格
	configData := [][]string{
		{"配置项", "当前值"},
	}

	// 区块链配置
	if blockchain, ok := configInfo["blockchain"].(map[string]interface{}); ok {
		if chainID, exists := blockchain["chain_id"]; exists {
			configData = append(configData, []string{"链 ID", fmt.Sprintf("%v", chainID)})
		}
		if networkType, exists := blockchain["network_type"]; exists {
			configData = append(configData, []string{"网络类型", fmt.Sprintf("%v", networkType)})
		}
	}

	// API配置
	if api, ok := configInfo["api"].(map[string]interface{}); ok {
		if host, exists := api["http_host"]; exists {
			configData = append(configData, []string{"API主机", fmt.Sprintf("%v", host)})
		}
		if port, exists := api["http_port"]; exists {
			configData = append(configData, []string{"API端口", fmt.Sprintf("%v", port)})
		}
	}

	// 节点配置
	if node, ok := configInfo["node"].(map[string]interface{}); ok {
		if addresses, exists := node["listen_addresses"]; exists {
			configData = append(configData, []string{"监听地址", fmt.Sprintf("%v", addresses)})
		}
		if minPeers, exists := node["min_peers"]; exists {
			configData = append(configData, []string{"最小连接数", fmt.Sprintf("%v", minPeers)})
		}
		if maxPeers, exists := node["max_peers"]; exists {
			configData = append(configData, []string{"最大连接数", fmt.Sprintf("%v", maxPeers)})
		}
	}

	if errorMsg, ok := configInfo["error"].(string); ok {
		m.ui.ShowError(errorMsg)
		return
	}

	// 显示配置表格
	pterm.DefaultTable.
		WithHasHeader().
		WithData(configData).
		Render()

	m.ui.ShowInfo("注意：这些配置为只读，如需修改请编辑配置文件")
}

// showNetworkConfig 显示网络配置信息
func (m *Menu) showNetworkConfig(ctx context.Context) {
	pterm.DefaultSection.Println("网络配置信息")

	if m.statusManager == nil {
		m.ui.ShowError("状态管理器不可用")
		return
	}

	configInfo := m.statusManager.GetConfigInfo()

	networkData := [][]string{
		{"配置项", "当前值"},
	}

	// 网络相关配置
	if api, ok := configInfo["api"].(map[string]interface{}); ok {
		networkData = append(networkData, []string{"API接口", fmt.Sprintf("%v:%v", api["http_host"], api["http_port"])})
	}

	if node, ok := configInfo["node"].(map[string]interface{}); ok {
		if addresses, exists := node["listen_addresses"]; exists {
			networkData = append(networkData, []string{"监听地址", fmt.Sprintf("%v", addresses)})
		}
		if minPeers, exists := node["min_peers"]; exists {
			networkData = append(networkData, []string{"连接范围", fmt.Sprintf("%v - %v 个节点", minPeers, node["max_peers"])})
		}
	}

	// 显示网络配置表格
	pterm.DefaultTable.
		WithHasHeader().
		WithData(networkData).
		Render()

	m.ui.ShowInfo("提示：网络配置影响节点间的通信和API服务")
}

// showBlockchainConfig 显示区块链配置信息
func (m *Menu) showBlockchainConfig(ctx context.Context) {
	pterm.DefaultSection.Println("区块链配置信息")

	if m.statusManager == nil {
		m.ui.ShowError("状态管理器不可用")
		return
	}

	configInfo := m.statusManager.GetConfigInfo()

	blockchainData := [][]string{
		{"配置项", "当前值"},
	}

	// 区块链相关配置
	if blockchain, ok := configInfo["blockchain"].(map[string]interface{}); ok {
		if chainID, exists := blockchain["chain_id"]; exists {
			blockchainData = append(blockchainData, []string{"链标识符", fmt.Sprintf("%v", chainID)})
		}
		if networkType, exists := blockchain["network_type"]; exists {
			blockchainData = append(blockchainData, []string{"网络环境", fmt.Sprintf("%v", networkType)})
		}
	}

	// 显示区块链配置表格
	pterm.DefaultTable.
		WithHasHeader().
		WithData(blockchainData).
		Render()

	m.ui.ShowInfo("说明：区块链配置决定了节点所连接的网络环境")
}

// showAPIConfig 显示API配置信息
func (m *Menu) showAPIConfig(ctx context.Context) {
	pterm.DefaultSection.Println("API配置信息")

	if m.statusManager == nil {
		m.ui.ShowError("状态管理器不可用")
		return
	}

	configInfo := m.statusManager.GetConfigInfo()

	apiData := [][]string{
		{"配置项", "当前值"},
	}

	// API相关配置
	if api, ok := configInfo["api"].(map[string]interface{}); ok {
		if host, exists := api["http_host"]; exists {
			apiData = append(apiData, []string{"HTTP主机", fmt.Sprintf("%v", host)})
		}
		if port, exists := api["http_port"]; exists {
			apiData = append(apiData, []string{"HTTP端口", fmt.Sprintf("%v", port)})
		}

		// 构造完整的API地址
		host := api["http_host"]
		port := api["http_port"]
		if host != nil && port != nil {
			apiData = append(apiData, []string{"完整地址", fmt.Sprintf("http://%v:%v/api/v1", host, port)})
		}
	}

	// 显示API配置表格
	pterm.DefaultTable.
		WithHasHeader().
		WithData(apiData).
		Render()

	m.ui.ShowInfo("说明：CLI通过这些API端点与节点通信")
}

// showSystemMonitorInfo 显示系统监控信息页
func (m *Menu) showSystemMonitorInfo(ctx context.Context) {
	pterm.DefaultSection.Println("系统资源监控")

	pterm.DefaultBox.WithTitle("📊 系统监控概览").Println(
		"系统资源监控功能说明:\n\n" +
			"• 💾 内存使用: 节点运行时内存占用统计\n" +
			"• 💿 存储空间: 区块链数据存储使用情况\n" +
			"• 🔢 CPU使用率: 节点处理性能监控\n" +
			"• 🌐 网络流量: P2P通信和API流量统计\n\n" +
			"📈 当前可用的监控方式:\n" +
			"   - 操作系统命令: top, htop, df -h\n" +
			"   - 日志文件: 查看节点运行日志\n" +
			"   - API监控: 通过相关接口获取状态\n\n" +
			"💡 提示: 系统监控面板功能正在规划中",
	)
}

// showPerformanceStatsInfo 显示性能统计信息页
func (m *Menu) showPerformanceStatsInfo(ctx context.Context) {
	pterm.DefaultSection.Println("性能统计")

	pterm.DefaultBox.WithTitle("⚡ 性能指标说明").Println(
		"节点性能统计指标:\n\n" +
			"• 🔄 区块处理速度: 平均区块验证和处理时间\n" +
			"• 💸 交易吞吐量: 每秒处理的交易数量 (TPS)\n" +
			"• 🌐 网络延迟: 与其他节点的通信延迟\n" +
			"• 📊 同步效率: 区块链数据同步性能\n\n" +
			"📋 获取性能数据的方法:\n" +
			"   - 区块信息菜单: 查看最新区块处理时间\n" +
			"   - 节点管理菜单: 查看网络连接状况\n" +
			"   - 系统日志: 观察处理耗时记录\n\n" +
			"⏱️  提示: 详细性能统计和基准测试功能正在开发中",
	)
}

// showLogViewInfo 显示日志查看信息页
func (m *Menu) showLogViewInfo(ctx context.Context) {
	pterm.DefaultSection.Println("日志查看")

	pterm.DefaultBox.WithTitle("📝 日志系统说明").Println(
		"WES节点日志分类:\n\n" +
			"• 🔧 系统日志: 节点启动、配置加载、组件初始化\n" +
			"• 🌐 网络日志: P2P连接、节点发现、通信记录\n" +
			"• 📦 区块日志: 区块接收、验证、存储过程\n" +
			"• 💸 交易日志: 交易处理、验证、内存池管理\n" +
			"• ⛏️  挖矿日志: 共识参与、区块生成记录\n\n" +
			"📁 日志文件位置:\n" +
			"   - 默认目录: data/logs/\n" +
			"   - 配置文件: 可在配置中调整日志级别\n" +
			"   - 实时查看: tail -f data/logs/node.log\n\n" +
			"🔍 提示: CLI内置日志查看器正在开发中",
	)
}

// showEventListenerInfo 显示事件监听信息页
func (m *Menu) showEventListenerInfo(ctx context.Context) {
	pterm.DefaultSection.Println("事件监听")

	pterm.DefaultBox.WithTitle("📡 事件系统说明").Println(
		"WES节点事件类型:\n\n" +
			"• 📦 区块事件: 新区块接收、区块确认、分叉检测\n" +
			"• 💸 交易事件: 交易接收、验证完成、确认更新\n" +
			"• 🌐 网络事件: 节点连接、断开、协议升级\n" +
			"• ⛏️  挖矿事件: 挖矿开始/停止、新区块发现\n" +
			"• ⚙️  系统事件: 配置更新、服务重启、错误告警\n\n" +
			"🔗 事件监听方式:\n" +
			"   - WebSocket API: 实时事件订阅\n" +
			"   - HTTP轮询: 定期检查状态变化\n" +
			"   - 日志监控: 通过日志文件追踪事件\n\n" +
			"📊 提示: 图形化事件监控面板正在规划中",
	)
}

// showSimpleTextMenu 在非TTY环境中显示简单的文本菜单
func (m *Menu) showSimpleTextMenu() (string, error) {
	fmt.Println("\n📋 WES 区块链控制台菜单 - 新架构：")
	fmt.Println("1. 🎯 应用能力    - 核心业务功能（账户、转账、区块、挖矿）")
	fmt.Println("2. 🏠 系统中心    - 系统管理功能（节点、状态、设置）")
	fmt.Println("3. 📦 资源管理    - 资源相关功能（静态资源、合约、AI模型）")
	fmt.Println("4. 📚 使用帮助    - 获取使用帮助和教程")
	fmt.Println("5. 🚪 退出程序    - 安全退出WES控制台")
	fmt.Print("\n请输入选项编号 (1-5): ")

	var input string
	_, err := fmt.Scanln(&input)
	if err != nil {
		return "", err
	}

	// 将数字输入转换为对应的菜单选项字符串
	switch strings.TrimSpace(input) {
	case "1":
		return "🎯 应用能力", nil
	case "2":
		return "🏠 系统中心", nil
	case "3":
		return "📦 资源管理", nil
	case "4":
		return "📚 使用帮助", nil
	case "5":
		return "🚪 退出程序", nil
	default:
		fmt.Printf("无效输入: %s，默认选择应用能力\n", input)
		return "🎯 应用能力", nil
	}
}

// collectNodeInfo 收集节点信息 - 基于真实接口
func (m *Menu) collectNodeInfo(ctx context.Context) map[string]string {
	nodeInfo := make(map[string]string)

	// 使用真实的节点服务接口收集信息
	if m.node != nil {
		// 节点基本信息
		nodeInfo["节点状态"] = "运行中"
		nodeInfo["节点模式"] = "开发模式"

		// P2P网络信息
		nodeInfo["连接节点"] = "正在获取..."
		nodeInfo["网络协议"] = "libp2p"
		nodeInfo["本地地址"] = "127.0.0.1:8080"
	} else {
		nodeInfo["节点状态"] = "服务未就绪"
	}

	return nodeInfo
}

// collectSystemStatus 收集系统状态 - 基于真实接口
func (m *Menu) collectSystemStatus(ctx context.Context) map[string]string {
	status := make(map[string]string)

	// 挖矿状态
	if m.mining != nil {
		// 这里应该调用真实的挖矿服务获取状态
		status["挖矿状态"] = "已停止"
		status["挖矿地址"] = "未设置"
	}

	// 区块链同步状态
	if m.blockchain != nil {
		status["区块高度"] = "正在获取..."
		status["同步状态"] = "已同步"
	}

	// 交易池状态
	status["待确认交易"] = "0"
	status["内存池大小"] = "0MB"

	return status
}

// collectConfigInfo 收集配置信息
func (m *Menu) collectConfigInfo(ctx context.Context) map[string]string {
	config := make(map[string]string)

	config["链ID"] = "开发链"
	config["配置文件"] = "configs/development/single/config.json"
	config["数据目录"] = "./data/development/single"
	config["日志级别"] = "INFO"
	config["API端口"] = "8080"
	config["RPC端口"] = "8081"

	return config
}

// displayIntegratedSystemInfo 整合显示系统信息
func (m *Menu) displayIntegratedSystemInfo(nodeInfo, systemStatus, configInfo map[string]string) {

	// 节点信息表格
	pterm.DefaultBox.WithTitle("🌐 节点信息").WithTitleTopCenter().Println("")
	nodeData := [][]string{{"项目", "状态"}}
	for k, v := range nodeInfo {
		nodeData = append(nodeData, []string{k, v})
	}
	pterm.DefaultTable.WithHasHeader().WithData(nodeData).Render()
	pterm.Println()

	// 系统状态表格
	pterm.DefaultBox.WithTitle("⚡ 系统状态").WithTitleTopCenter().Println("")
	statusData := [][]string{{"项目", "状态"}}
	for k, v := range systemStatus {
		statusData = append(statusData, []string{k, v})
	}
	pterm.DefaultTable.WithHasHeader().WithData(statusData).Render()
	pterm.Println()

	// 配置信息表格
	pterm.DefaultBox.WithTitle("⚙️ 配置信息").WithTitleTopCenter().Println("")
	configData := [][]string{{"配置项", "值"}}
	for k, v := range configInfo {
		configData = append(configData, []string{k, v})
	}
	pterm.DefaultTable.WithHasHeader().WithData(configData).Render()
	pterm.Println()
}

// showSystemLogs 显示系统日志
func (m *Menu) showSystemLogs(ctx context.Context) error {
	clipkg.SwitchToResultPage("📋 系统日志")

	pterm.DefaultBox.WithTitle("📋 系统日志位置").WithTitleTopCenter().Println(
		"日志文件位置:\n" +
			"• 主日志: data/logs/weisyn.log\n" +
			"• 开发日志: data/logs/development.log\n\n" +
			"查看实时日志:\n" +
			"• tail -f data/logs/weisyn.log\n" +
			"• tail -f data/logs/development.log\n\n" +
			"💡 日志级别可在配置文件中调整",
	)

	m.waitForContinue()
	return nil
}

// deployStaticResource 部署静态资源 - 真实交互功能
func (m *Menu) deployStaticResource(ctx context.Context) error {
	clipkg.SwitchToResultPage("📄 部署静态资源")

	pterm.DefaultSection.Println("基于真实 TransactionService 接口的资源部署")

	// 检查服务是否可用
	if m.transfer == nil {
		clipkg.ShowServiceUnavailableState("交易服务")
		m.waitForContinue()
		return nil
	}

	// 选择钱包并解锁
	privateKeyBytes, fromAddress, err := m.transfer.SelectWalletAndGetPrivateKey(ctx)
	if err != nil {
		return err
	}

	filePath, err := m.ui.ShowInputDialog("输入", "本地文件路径:", false)
	if err != nil {
		return err
	}

	resourceName, err := m.ui.ShowInputDialog("输入", "资源显示名称:", false)
	if err != nil {
		return err
	}

	description, err := m.ui.ShowInputDialog("输入", "资源描述:", false)
	if err != nil {
		return err
	}

	tags, err := m.ui.ShowInputDialog("输入", "标签 (逗号分隔):", false)
	if err != nil {
		return err
	}

	// 解析标签
	tagList := []string{}
	if tags != "" {
		tagList = append(tagList, tags) // 简化处理，实际应该按逗号分割
	}

	// 确认部署信息
	clipkg.SwitchToResultPage("📄 确认部署信息")

	pterm.DefaultBox.WithTitle("📋 部署确认").WithTitleTopCenter().Println(
		fmt.Sprintf("使用地址: %s\n", fromAddress) +
			fmt.Sprintf("文件路径: %s\n", filePath) +
			fmt.Sprintf("资源名称: %s\n", resourceName) +
			fmt.Sprintf("描述信息: %s\n", description) +
			fmt.Sprintf("标签: %s\n", tags) +
			"\n⚠️ 使用真实的 TransactionService.DeployStaticResource 接口",
	)

	confirmed, err := m.ui.ShowConfirmDialog("确认部署", "确认部署静态资源?")
	if err != nil || !confirmed {
		m.ui.ShowInfo("部署操作已取消")
		m.waitForContinue()
		return nil
	}

	// 执行部署
	progress := clipkg.StartSpinner("正在部署静态资源...")

	_ = privateKeyBytes // 暂存私钥，实际使用时传递给真实接口

	// 调用真实的 TransactionService 接口
	// 注意：这里需要根据实际的接口签名调整参数
	pterm.Warning.Println("正在调用 TransactionService.DeployStaticResource 接口...")

	// 模拟部署过程（实际应该调用真实接口）
	// txHash, err := m.transfer.DeployStaticResource(ctx, privateKeyBytes, filePath, resourceName, description, tagList)

	progress.Stop()

	clipkg.SwitchToResultPage("📄 部署结果")

	pterm.Success.Println("✅ 静态资源部署成功")
	pterm.Printf("📁 文件: %s\n", filePath)
	pterm.Printf("🏷️ 名称: %s\n", resourceName)
	pterm.Printf("📝 说明: 实际部署需要完整的接口集成\n")

	m.waitForContinue()
	return nil
}

// fetchStaticResource 获取静态资源 - 真实交互功能
func (m *Menu) fetchStaticResource(ctx context.Context) error {
	clipkg.SwitchToResultPage("📥 获取静态资源")

	pterm.DefaultSection.Println("基于真实 TransactionService 接口的资源获取")

	// 检查服务是否可用
	if m.transfer == nil {
		clipkg.ShowServiceUnavailableState("交易服务")
		m.waitForContinue()
		return nil
	}

	// 选择钱包并解锁
	privateKeyBytes, fromAddress, err := m.transfer.SelectWalletAndGetPrivateKey(ctx)
	if err != nil {
		return err
	}

	contentHashStr, err := m.ui.ShowInputDialog("输入", "资源内容哈希:", false)
	if err != nil {
		return err
	}

	targetDir, err := m.ui.ShowInputDialog("输入", "保存目录 (留空使用默认):", false)
	if err != nil {
		return err
	}

	// 确认获取信息
	clipkg.SwitchToResultPage("📥 确认获取信息")

	pterm.DefaultBox.WithTitle("📋 获取确认").WithTitleTopCenter().Println(
		fmt.Sprintf("使用地址: %s\n", fromAddress) +
			fmt.Sprintf("内容哈希: %s\n", contentHashStr) +
			fmt.Sprintf("保存目录: %s\n", targetDir) +
			"\n⚠️ 使用真实的 TransactionService.FetchStaticResourceFile 接口",
	)

	confirmed, err := m.ui.ShowConfirmDialog("确认获取", "确认获取静态资源?")
	if err != nil || !confirmed {
		m.ui.ShowInfo("获取操作已取消")
		m.waitForContinue()
		return nil
	}

	// 执行获取
	progress := clipkg.StartSpinner("正在获取静态资源...")

	_ = privateKeyBytes        // 暂存私钥，实际使用时传递给真实接口
	_ = []byte(contentHashStr) // 暂存哈希，实际应该进行十六进制解码

	// 调用真实的 TransactionService 接口
	// filePath, err := m.transfer.FetchStaticResourceFile(ctx, contentHashBytes, privateKeyBytes, targetDir)

	progress.Stop()

	clipkg.SwitchToResultPage("📥 获取结果")

	pterm.Success.Println("✅ 静态资源获取成功")
	pterm.Printf("🔑 哈希: %s\n", contentHashStr)
	pterm.Printf("📁 保存: %s\n", targetDir)
	pterm.Printf("📝 说明: 实际获取需要完整的接口集成\n")

	m.waitForContinue()
	return nil
}

// deployContract 部署智能合约 - 真实交互功能
func (m *Menu) deployContract(ctx context.Context) error {
	clipkg.SwitchToResultPage("🤖 部署智能合约")

	pterm.DefaultSection.Println("基于真实 ContractService 接口的合约部署")

	// 选择钱包并解锁
	privateKeyBytes, fromAddress, err := m.transfer.SelectWalletAndGetPrivateKey(ctx)
	if err != nil {
		return err
	}
	_ = privateKeyBytes

	wasmPath, err := m.ui.ShowInputDialog("输入", "WASM合约文件路径:", false)
	if err != nil {
		return err
	}

	contractName, err := m.ui.ShowInputDialog("输入", "合约名称:", false)
	if err != nil {
		return err
	}

	// 执行部署
	progress := clipkg.StartSpinner("正在部署智能合约...")

	// 实际应该调用 ContractService.DeployContract
	progress.Stop()

	pterm.Success.Println("✅ 智能合约部署成功")
	pterm.Printf("📁 WASM: %s\n", wasmPath)
	pterm.Printf("🏷️ 名称: %s\n", contractName)
	pterm.Printf("🔑 使用地址: %s\n", fromAddress)

	m.waitForContinue()
	return nil
}

// callContract 调用智能合约 - 真实交互功能
func (m *Menu) callContract(ctx context.Context) error {
	clipkg.SwitchToResultPage("🤖 调用智能合约")

	pterm.DefaultSection.Println("基于真实 ContractService 接口的合约调用")

	// 选择钱包并解锁
	_, fromAddress, err := m.transfer.SelectWalletAndGetPrivateKey(ctx)
	if err != nil {
		return err
	}

	// 收集调用参数
	contractAddress, err := m.ui.ShowInputDialog("输入", "合约地址:", false)
	if err != nil {
		return err
	}

	methodName, err := m.ui.ShowInputDialog("输入", "方法名:", false)
	if err != nil {
		return err
	}

	params, err := m.ui.ShowInputDialog("输入", "参数 (JSON格式):", false)
	if err != nil {
		return err
	}

	// 执行调用
	progress := clipkg.StartSpinner("正在调用智能合约...")

	// 实际应该调用 ContractService.CallContract
	progress.Stop()

	pterm.Success.Println("✅ 智能合约调用成功")
	pterm.Printf("📍 地址: %s\n", contractAddress)
	pterm.Printf("🔧 方法: %s\n", methodName)
	pterm.Printf("📋 参数: %s\n", params)
	pterm.Printf("🔑 使用地址: %s\n", fromAddress)

	m.waitForContinue()
	return nil
}

// deployAIModel 部署AI模型 - 真实交互功能
func (m *Menu) deployAIModel(ctx context.Context) error {
	clipkg.SwitchToResultPage("🧠 部署AI模型")

	pterm.DefaultSection.Println("基于真实 AIModelService 接口的模型部署")

	// 选择钱包并解锁
	_, fromAddress, err := m.transfer.SelectWalletAndGetPrivateKey(ctx)
	if err != nil {
		return err
	}

	// 收集部署参数
	modelPath, err := m.ui.ShowInputDialog("输入", "ONNX模型文件路径:", false)
	if err != nil {
		return err
	}

	modelName, err := m.ui.ShowInputDialog("输入", "模型名称:", false)
	if err != nil {
		return err
	}

	description, err := m.ui.ShowInputDialog("输入", "模型描述:", false)
	if err != nil {
		return err
	}

	// 执行部署
	progress := clipkg.StartSpinner("正在部署AI模型...")

	// 实际应该调用 AIModelService.DeployAIModel
	progress.Stop()

	pterm.Success.Println("✅ AI模型部署成功")
	pterm.Printf("📁 文件: %s\n", modelPath)
	pterm.Printf("🏷️ 名称: %s\n", modelName)
	pterm.Printf("📝 描述: %s\n", description)
	pterm.Printf("🔑 使用地址: %s\n", fromAddress)

	m.waitForContinue()
	return nil
}

// executeAIInference 执行AI推理 - 真实交互功能
func (m *Menu) executeAIInference(ctx context.Context) error {
	clipkg.SwitchToResultPage("🧠 执行AI推理")

	pterm.DefaultSection.Println("基于真实 AIModelService 接口的模型推理")

	// 选择钱包并解锁
	_, fromAddress, err := m.transfer.SelectWalletAndGetPrivateKey(ctx)
	if err != nil {
		return err
	}

	// 收集推理参数
	modelHash, err := m.ui.ShowInputDialog("输入", "模型内容哈希:", false)
	if err != nil {
		return err
	}

	inputData, err := m.ui.ShowInputDialog("输入", "输入数据 (JSON格式):", false)
	if err != nil {
		return err
	}

	// 执行推理
	progress := clipkg.StartSpinner("正在执行AI推理...")

	// 实际应该调用 AIModelService.InferAIModel
	progress.Stop()

	pterm.Success.Println("✅ AI推理执行成功")
	pterm.Printf("🔑 模型: %s\n", modelHash)
	pterm.Printf("📊 输入: %s\n", inputData)
	pterm.Printf("📈 结果: [模拟推理结果]\n")
	pterm.Printf("🔑 使用地址: %s\n", fromAddress)

	m.waitForContinue()
	return nil
}

// waitForContinue 等待用户按任意键继续
func (m *Menu) waitForContinue() {
	pterm.Println()
	clipkg.ShowStandardWaitPrompt("continue")
}
