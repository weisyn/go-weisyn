package manager

import (
	"context"
	"os"
	"strings"

	"github.com/pterm/pterm"

	"github.com/weisyn/v1/internal/app/version"
	"github.com/weisyn/v1/internal/cli/commands"
	"github.com/weisyn/v1/internal/cli/guides"
	"github.com/weisyn/v1/internal/cli/interactive"
	"github.com/weisyn/v1/internal/cli/layout"
	"github.com/weisyn/v1/internal/cli/layout/screens"
	"github.com/weisyn/v1/internal/cli/permissions"
	"github.com/weisyn/v1/internal/cli/status"
	"github.com/weisyn/v1/internal/cli/ui"
	blockchainintf "github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// Controller CLI控制器，协调各个CLI组件 - 使用新的LayoutManager架构
type Controller struct {
	logger         log.Logger
	layoutManager  *layout.LayoutManager
	statusManager  *status.StatusManager
	menu           *interactive.Menu      // 保留备用
	dashboard      *interactive.Dashboard // 保留备用
	account        *commands.AccountCommands
	transfer       *commands.TransferCommands
	blockchain     *commands.BlockchainCommands
	mining         *commands.MiningCommands
	node           *commands.NodeCommands
	firstTimeGuide guides.FirstTimeGuide
	permissionMgr  *permissions.Manager
	uiComponents   ui.Components
}

// NewController 创建CLI控制器实例 - 使用新的LayoutManager架构
func NewController(
	logger log.Logger,
	statusManager *status.StatusManager,
	menu *interactive.Menu,
	dashboard *interactive.Dashboard,
	account *commands.AccountCommands,
	transfer *commands.TransferCommands,
	blockchain *commands.BlockchainCommands,
	mining *commands.MiningCommands,
	node *commands.NodeCommands,
	accountService blockchainintf.AccountService,
	permissionManager *permissions.Manager,
	uiComponents ui.Components,
) *Controller {
	// 创建首次用户引导
	firstTimeGuide := guides.NewFirstTimeGuide(
		logger,
		account,
		transfer,
		mining,
		blockchain,
		accountService,
		permissionManager,
		uiComponents,
	)

	// 创建LayoutManager
	layoutManager := layout.NewLayoutManager(logger, statusManager, uiComponents)

	// 创建并注册所有屏幕
	welcomeScreen := screens.NewWelcomeScreen(logger)
	firstGuideScreen := screens.NewFirstTimeGuideScreen(logger, firstTimeGuide)
	mainMenuScreen := screens.NewMainMenuScreen(
		logger,
		uiComponents,
		account,
		transfer,
		blockchain,
		mining,
		node,
	)

	// 注册屏幕到LayoutManager
	layoutManager.RegisterScreen(welcomeScreen)
	layoutManager.RegisterScreen(firstGuideScreen)
	layoutManager.RegisterScreen(mainMenuScreen)

	return &Controller{
		logger:         logger,
		layoutManager:  layoutManager,
		statusManager:  statusManager,
		menu:           menu,      // 保留备用
		dashboard:      dashboard, // 保留备用
		account:        account,
		transfer:       transfer,
		blockchain:     blockchain,
		mining:         mining,
		node:           node,
		firstTimeGuide: firstTimeGuide,
		permissionMgr:  permissionManager,
		uiComponents:   uiComponents,
	}
}

// Run 启动CLI应用 - 使用新的LayoutManager架构
func (c *Controller) Run(ctx context.Context) error {
	c.logger.Info("🚀 启动WES CLI应用...")

	// 启动StatusManager
	if c.statusManager != nil {
		if err := c.statusManager.Start(ctx); err != nil {
			c.logger.Errorf("启动状态管理器失败: %v", err)
		}
		defer c.statusManager.Stop()
	}

	// 检查用户是否为首次用户
	userContext := c.permissionMgr.GetUserContext()
	isFirstTimeUser := userContext.IsFirstTimeUser

	// 简化启动日志
	c.logger.Info("🚀 CLI系统就绪")

	// 根据用户类型选择起始屏幕
	var startScreen string
	if isFirstTimeUser {
		c.logger.Info("检测到首次用户，启动引导流程")
		startScreen = "first_time_guide"
	} else {
		c.logger.Info("常规用户，显示欢迎界面")
		startScreen = "welcome"
	}

	// 使用LayoutManager显示起始屏幕
	if err := c.layoutManager.Show(ctx, startScreen); err != nil {
		if err.Error() == "exit" {
			c.logger.Info("用户选择退出")
			return nil
		}
		c.logger.Errorf("显示屏幕失败: %v", err)

		// 出现错误时回退到旧的菜单系统
		c.logger.Warn("回退到传统菜单系统")
		return c.menu.Run(ctx)
	}

	c.logger.Info("CLI应用正常结束")
	return nil
}

// ExecuteCommand 执行单个命令
func (c *Controller) ExecuteCommand(ctx context.Context, command string) error {
	switch strings.ToLower(command) {
	case "balance":
		return c.account.ShowBalance(ctx)
	case "transfer":
		return c.transfer.InteractiveTransfer(ctx)
	case "status":
		return c.node.ShowStatus(ctx)
	case "mining":
		return c.mining.ShowMiningStatus(ctx)
	case "peers":
		return c.node.ShowPeers(ctx)
	case "blocks":
		return c.blockchain.ShowLatestBlocks(ctx)
	default:
		return c.showCommandHelp(command)
	}
}

// showWelcome 显示欢迎信息
func (c *Controller) showWelcome() {
	// 清屏交由统一页面工具处理
	ui.ShowPageHeader()

	// 添加顶部空行，让界面不那么拥挤
	pterm.Println()

	// 创建欢迎横幅 - 左对齐显示，更整齐
	asciiArt := `██╗    ██╗███████╗███████╗
██║    ██║██╔════╝██╔════╝
██║ █╗ ██║█████╗  ███████╗
██║███╗██║██╔══╝  ╚════██║
╚███╔███╔╝███████╗███████║
 ╚══╝╚══╝ ╚══════╝╚══════╝`

	// 左对齐显示ASCII艺术
	lines := strings.Split(asciiArt, "\n")
	for _, line := range lines {
		// 应用样式但不添加居中padding
		styledLine := pterm.NewStyle(pterm.FgLightBlue, pterm.Bold).Sprint(line)
		pterm.Println(styledLine)
	}

	// ASCII艺术后添加空行
	pterm.Println()

	// 显示版本和状态信息 - 左对齐
	pterm.Println(pterm.LightGreen("🌟 微迅 (weisyn) 区块链节点 CLI " + version.GetVersion()))
	pterm.Println(pterm.Gray("基于EUTXO模型的下一代区块链平台"))
	pterm.Println() // 标题后添加换行
}

// showCommandHelp 显示命令帮助
func (c *Controller) showCommandHelp(command string) error {
	pterm.Error.Printf("未知命令: %s\n\n", command)

	pterm.DefaultHeader.Println("可用命令")

	commands := [][]string{
		{"balance", "查看账户余额"},
		{"transfer", "执行转账操作"},
		{"status", "显示节点状态"},
		{"mining", "查看挖矿状态"},
		{"peers", "显示连接的节点"},
		{"blocks", "查看最新区块"},
	}

	pterm.DefaultTable.WithHasHeader().WithData(append([][]string{
		{"命令", "描述"},
	}, commands...)).Render()

	pterm.Println()
	pterm.Info.Println("使用示例:")
	pterm.Printf("  %s --cli balance   # 查看余额\n", os.Args[0])
	pterm.Printf("  %s --cli transfer  # 执行转账\n", os.Args[0])
	pterm.Printf("  %s --daemon        # 后台运行\n", os.Args[0])
	pterm.Printf("  %s                 # 交互模式\n", os.Args[0])

	return nil
}

// 删除了interceptSystemLogs函数，避免定时日志干扰界面显示
