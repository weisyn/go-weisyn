package commands

import (
	"context"
	"fmt"

	"github.com/pterm/pterm"

	"github.com/weisyn/v1/internal/cli/client"
	"github.com/weisyn/v1/internal/cli/ui"
	walletpkg "github.com/weisyn/v1/internal/cli/wallet"
	blockchainintf "github.com/weisyn/v1/pkg/interfaces/blockchain"
	consensusintf "github.com/weisyn/v1/pkg/interfaces/consensus"
	cryptointf "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// MiningCommands 挖矿控制命令处理器 - 基于真实接口
type MiningCommands struct {
	logger         log.Logger
	apiClient      *client.Client
	ui             ui.Components
	minerService   consensusintf.MinerService  // 💎 挖矿服务（真实接口）
	chainService   blockchainintf.ChainService // 🔗 区块链服务（真实接口）
	addressManager cryptointf.AddressManager   // 🏠 地址管理（真实接口）
	walletManager  walletpkg.WalletManager     // 🔐 本地钱包管理（用于选择矿工地址）
}

// NewMiningCommands 创建挖矿命令处理器 - 直接接收真实接口
func NewMiningCommands(
	logger log.Logger,
	apiClient *client.Client,
	ui ui.Components,
	minerService consensusintf.MinerService,
	chainService blockchainintf.ChainService,
	addressManager cryptointf.AddressManager,
	walletManager walletpkg.WalletManager,
) *MiningCommands {
	return &MiningCommands{
		logger:         logger,
		apiClient:      apiClient,
		ui:             ui,
		minerService:   minerService,
		chainService:   chainService,
		addressManager: addressManager,
		walletManager:  walletManager,
	}
}

// ShowMiningMenu 显示挖矿控制菜单 - 基于真实接口
func (m *MiningCommands) ShowMiningMenu(ctx context.Context) error {
	for {
		// 统一页面头部显示
		ui.ShowPageHeader()

		pterm.DefaultSection.Println("⛏️ 挖矿控制")
		pterm.Println()

		// 显示菜单选项 - 基于真实接口功能
		options := []string{
			"查看挖矿状态",
			"开始挖矿",
			"停止挖矿",
			"挖矿功能说明",
			"返回主菜单",
		}

		selectedIndex, err := m.ui.ShowMenu("请选择挖矿操作:", options)
		if err != nil {
			m.logger.Errorf("菜单选择失败: %v", err)
			m.ui.ShowError(fmt.Sprintf("菜单操作失败: %v", err))
			m.waitForContinue()
			continue
		}

		switch selectedIndex {
		case 0: // 查看挖矿状态
			if err := m.ShowMiningStatus(ctx); err != nil {
				m.logger.Errorf("查看挖矿状态失败: %v", err)
				m.ui.ShowError(fmt.Sprintf("查看挖矿状态失败: %v", err))
				m.waitForContinue()
			}
		case 1: // 开始挖矿
			if err := m.StartMining(ctx); err != nil {
				m.logger.Errorf("开始挖矿失败: %v", err)
				m.ui.ShowError(fmt.Sprintf("开始挖矿失败: %v", err))
				m.waitForContinue()
			}
		case 2: // 停止挖矿
			if err := m.StopMining(ctx); err != nil {
				m.logger.Errorf("停止挖矿失败: %v", err)
				m.ui.ShowError(fmt.Sprintf("停止挖矿失败: %v", err))
				m.waitForContinue()
			}
		case 3: // 挖矿功能说明
			if err := m.ShowMiningInfo(ctx); err != nil {
				m.logger.Errorf("显示功能说明失败: %v", err)
				m.ui.ShowError(fmt.Sprintf("显示功能说明失败: %v", err))
				m.waitForContinue()
			}
		case 4: // 返回主菜单
			return nil
		default:
			m.ui.ShowWarning("无效的选择，请重新选择")
			m.waitForContinue()
			continue
		}
	}
}

// ShowMiningStatus 显示挖矿状态 - 基于真实MinerService.GetMiningStatus
func (m *MiningCommands) ShowMiningStatus(ctx context.Context) error {
	ui.SwitchToResultPage("⛏️ 挖矿状态")

	// 检查挖矿服务是否可用
	if m.minerService == nil {
		ui.ShowServiceUnavailableState("挖矿服务")
		m.waitForContinue()
		return nil
	}

	// 获取真实的挖矿状态信息
	progress := ui.StartSpinner("正在获取挖矿状态...")
	isRunning, minerAddress, err := m.minerService.GetMiningStatus(ctx)
	progress.Stop()

	if err != nil {
		ui.ShowNetworkErrorState("获取挖矿状态", err.Error())
		m.waitForContinue()
		return nil
	}

	// 显示真实的挖矿状态信息
	statusData := [][]string{
		{"挖矿状态", m.getMiningStatusText(isRunning)},
		{"矿工地址", m.formatMinerAddress(minerAddress)},
	}

	if !isRunning {
		statusData = append(statusData, []string{"状态说明", "挖矿未运行"})
	} else {
		statusData = append(statusData, []string{"状态说明", "挖矿正在运行中"})
	}

	pterm.DefaultBox.WithTitle("⛏️ 挖矿状态").WithTitleTopCenter().Println("")
	pterm.DefaultTable.
		WithHasHeader().
		WithData(append([][]string{{"状态项目", "当前值"}}, statusData...)).
		Render()

	pterm.Printf("\n💡 系统说明:\n")
	pterm.Printf("   • 挖矿状态信息实时更新\n")
	pterm.Printf("   • 支持启动、停止和状态查询功能\n")
	pterm.Printf("   • 不提供收益统计和历史记录功能\n")

	m.waitForContinue()
	return nil
}

// StartMining 开始挖矿 - 基于真实MinerService.StartMining
func (m *MiningCommands) StartMining(ctx context.Context) error {
	ui.SwitchToResultPage("⛏️ 开始挖矿")

	// 检查挖矿服务是否可用
	if m.minerService == nil {
		ui.ShowServiceUnavailableState("挖矿服务")
		m.waitForContinue()
		return nil
	}

	// 获取当前状态
	progress := ui.StartSpinner("正在检查挖矿状态...")
	isRunning, currentMinerAddress, err := m.minerService.GetMiningStatus(ctx)
	progress.Stop()

	if err != nil {
		ui.ShowNetworkErrorState("检查挖矿状态", err.Error())
		m.waitForContinue()
		return nil
	}

	if isRunning {
		pterm.DefaultBox.WithTitle("ℹ️ 挖矿状态").WithTitleTopCenter().Println(
			"挖矿已在运行中\n" +
				fmt.Sprintf("矿工地址: %s\n\n", m.formatMinerAddress(currentMinerAddress)) +
				"如需更换矿工地址，请先停止挖矿",
		)
		m.waitForContinue()
		return nil
	}

	// 获取矿工地址（智能选择：钱包列表 -> 手动输入）
	addressStr, minerAddressBytes, err := m.getMinerAddress(ctx)
	if err != nil {
		return err
	}
	if addressStr == "" {
		m.ui.ShowInfo("挖矿操作已取消")
		m.waitForContinue()
		return nil
	}

	// 确认开始挖矿
	confirmed, err := m.ui.ShowConfirmDialog("确认开始挖矿", "确认开始挖矿操作?")
	if err != nil || !confirmed {
		m.ui.ShowInfo("挖矿操作已取消")
		m.waitForContinue()
		return nil
	}

	// 开始挖矿（直接调用真实接口）
	progress = ui.StartSpinner("正在开始挖矿...")
	err = m.minerService.StartMining(ctx, minerAddressBytes)
	progress.Stop()

	if err != nil {
		ui.ShowNetworkErrorState("开始挖矿", err.Error())
		m.waitForContinue()
		return nil
	}

	// 显示成功信息
	pterm.DefaultBox.WithTitle("✅ 挖矿启动成功").WithTitleTopCenter().Println(
		fmt.Sprintf("矿工地址: %s\n", addressStr) +
			"挖矿已成功启动\n\n" +
			"💡 提示:\n" +
			"• 挖矿将在后台持续运行\n" +
			"• 可以通过「查看挖矿状态」确认运行状态\n" +
			"• 使用「停止挖矿」可以停止挖矿进程",
	)

	m.waitForContinue()
	return nil
}

// StopMining 停止挖矿 - 基于真实MinerService.StopMining
func (m *MiningCommands) StopMining(ctx context.Context) error {
	ui.SwitchToResultPage("⛏️ 停止挖矿")

	// 检查挖矿服务是否可用
	if m.minerService == nil {
		ui.ShowServiceUnavailableState("挖矿服务")
		m.waitForContinue()
		return nil
	}

	// 获取当前状态
	progress := ui.StartSpinner("正在检查挖矿状态...")
	isRunning, minerAddress, err := m.minerService.GetMiningStatus(ctx)
	progress.Stop()

	if err != nil {
		ui.ShowNetworkErrorState("检查挖矿状态", err.Error())
		m.waitForContinue()
		return nil
	}

	if !isRunning {
		pterm.DefaultBox.WithTitle("ℹ️ 挖矿状态").WithTitleTopCenter().Println(
			"挖矿当前未运行\n\n" +
				"💡 提示: 使用「开始挖矿」可以启动挖矿进程",
		)
		m.waitForContinue()
		return nil
	}

	// 确认停止挖矿
	pterm.DefaultBox.WithTitle("📋 当前挖矿信息").WithTitleTopCenter().Println(
		fmt.Sprintf("矿工地址: %s\n", m.formatMinerAddress(minerAddress)) +
			"挖矿状态: 正在运行",
	)

	confirmed, err := m.ui.ShowConfirmDialog("确认停止挖矿", "确认停止挖矿操作?")
	if err != nil || !confirmed {
		m.ui.ShowInfo("操作已取消")
		m.waitForContinue()
		return nil
	}

	// 停止挖矿（直接调用真实接口）
	progress = ui.StartSpinner("正在停止挖矿...")
	err = m.minerService.StopMining(ctx)
	progress.Stop()

	if err != nil {
		ui.ShowNetworkErrorState("停止挖矿", err.Error())
		m.waitForContinue()
		return nil
	}

	// 显示成功信息
	pterm.DefaultBox.WithTitle("✅ 挖矿已停止").WithTitleTopCenter().Println(
		"挖矿进程已成功停止\n\n" +
			"💡 提示:\n" +
			"• 使用「查看挖矿状态」可以确认状态\n" +
			"• 使用「开始挖矿」可以重新启动挖矿",
	)

	m.waitForContinue()
	return nil
}

// ShowMiningInfo 挖矿功能说明 - 基于真实接口的功能介绍
func (m *MiningCommands) ShowMiningInfo(ctx context.Context) error {
	ui.SwitchToResultPage("⛏️ 挖矿功能说明")

	pterm.DefaultBox.WithTitle("💡 挖矿功能说明").WithTitleTopCenter().Println(
		"基于真实MinerService接口的挖矿功能:\n\n" +
			"🎯 支持的操作:\n" +
			"• StartMining(ctx, minerAddress) - 开始挖矿\n" +
			"• StopMining(ctx) - 停止挖矿\n" +
			"• GetMiningStatus(ctx) - 获取挖矿状态\n\n" +
			"❌ 不支持的功能:\n" +
			"• 收益统计和历史记录\n" +
			"• 挖矿难度配置\n" +
			"• 线程数配置\n" +
			"• 其他高级配置选项\n\n" +
			"💡 设计原则:\n" +
			"• CLI直接调用pkg/interfaces中定义的真实接口\n" +
			"• 不创建任何抽象层或虚假功能\n" +
			"• 专注于核心挖矿控制功能",
	)

	m.waitForContinue()
	return nil
}

// getMiningStatusText 获取挖矿状态文本
func (m *MiningCommands) getMiningStatusText(isRunning bool) string {
	if isRunning {
		return "🟢 正在运行"
	}
	return "🔴 已停止"
}

// formatMinerAddress 格式化矿工地址显示
func (m *MiningCommands) formatMinerAddress(minerAddress []byte) string {
	if len(minerAddress) == 0 {
		return "未设置"
	}

	// 简化显示：显示前6位和后4位
	addressStr := string(minerAddress)
	if len(addressStr) > 10 {
		return fmt.Sprintf("%s...%s", addressStr[:6], addressStr[len(addressStr)-4:])
	}
	return addressStr
}

// getMinerAddress 智能获取矿工地址 - 优先钱包选择，支持手动输入降级
func (m *MiningCommands) getMinerAddress(ctx context.Context) (string, []byte, error) {
	// 策略1：优先尝试从钱包列表选择
	if m.walletManager != nil {
		wallets, err := m.walletManager.ListWallets(ctx)
		if err == nil && len(wallets) > 0 {
			// 有钱包可用，显示钱包选择器
			return m.selectFromWallets(ctx, wallets)
		}
	}

	// 策略2：钱包不可用或无钱包 - 降级处理
	return m.handleNoWalletScenario(ctx)
}

// selectFromWallets 从钱包列表中选择矿工地址
func (m *MiningCommands) selectFromWallets(ctx context.Context, wallets []*walletpkg.WalletInfo) (string, []byte, error) {
	pterm.DefaultBox.WithTitle("💡 智能地址选择").WithTitleTopCenter().Println(
		fmt.Sprintf("检测到 %d 个钱包，建议从现有钱包中选择矿工地址\n\n", len(wallets)) +
			"优势：\n" +
			"• 无需手动输入，避免地址错误\n" +
			"• 直接使用您拥有的地址进行挖矿\n" +
			"• 挖矿收益将发送到您的钱包",
	)

	// 构建钱包显示列表
	displayList := make([]ui.WalletDisplayInfo, 0, len(wallets))
	for _, w := range wallets {
		displayList = append(displayList, ui.WalletDisplayInfo{
			ID:       w.ID,
			Name:     w.Name,
			Address:  w.Address,
			Balance:  "--", // 挖矿不需要显示余额
			IsLocked: !w.IsUnlocked,
		})
	}

	// 添加手动输入选项
	options := []string{
		"从钱包列表选择地址 (推荐)",
		"手动输入其他地址",
		"取消挖矿操作",
	}

	selectedOption, err := m.ui.ShowMenu("选择地址获取方式:", options)
	if err != nil {
		return "", nil, err
	}

	switch selectedOption {
	case 0: // 从钱包选择
		idx, err := m.ui.ShowWalletSelector(displayList)
		if err != nil {
			return "", nil, err
		}

		selectedWallet := wallets[idx]
		addressStr := selectedWallet.Address

		// 验证并转换地址
		minerAddressBytes, err := m.validateAndConvertAddress(addressStr)
		if err != nil {
			return "", nil, err
		}

		return addressStr, minerAddressBytes, nil

	case 1: // 手动输入
		return m.handleManualAddressInput(ctx)

	case 2: // 取消
		return "", nil, nil

	default:
		return "", nil, fmt.Errorf("无效选择")
	}
}

// handleNoWalletScenario 处理无钱包情况 - 提供引导和降级选项
func (m *MiningCommands) handleNoWalletScenario(ctx context.Context) (string, []byte, error) {
	pterm.DefaultBox.WithTitle("⚠️ 未找到钱包").WithTitleTopCenter().Println(
		"当前系统中没有可用的钱包\n\n" +
			"💡 建议解决方案:\n" +
			"1. 先创建钱包 (推荐) - 通过「💰 账户管理」→「钱包管理」\n" +
			"2. 手动输入地址 (高级用户) - 需要您确保地址正确性\n\n" +
			"🎯 使用钱包的好处:\n" +
			"• 挖矿收益自动进入您的钱包\n" +
			"• 避免地址输入错误\n" +
			"• 便于后续资金管理",
	)

	options := []string{
		"现在去创建钱包 (推荐)",
		"手动输入矿工地址",
		"取消挖矿操作",
	}

	selectedOption, err := m.ui.ShowMenu("选择解决方案:", options)
	if err != nil {
		return "", nil, err
	}

	switch selectedOption {
	case 0: // 引导创建钱包
		pterm.DefaultBox.WithTitle("📋 创建钱包指引").WithTitleTopCenter().Println(
			"请按以下步骤创建钱包:\n\n" +
				"1. 退出当前挖矿操作\n" +
				"2. 选择「💰 账户管理」\n" +
				"3. 选择「钱包管理」\n" +
				"4. 选择「创建钱包」\n" +
				"5. 创建成功后返回挖矿菜单\n\n" +
				"💡 钱包创建后，系统会自动识别并提供钱包地址选择",
		)
		m.waitForContinue()
		return "", nil, nil

	case 1: // 手动输入
		return m.handleManualAddressInput(ctx)

	case 2: // 取消
		return "", nil, nil

	default:
		return "", nil, fmt.Errorf("无效选择")
	}
}

// handleManualAddressInput 处理手动地址输入
func (m *MiningCommands) handleManualAddressInput(ctx context.Context) (string, []byte, error) {
	pterm.DefaultBox.WithTitle("⚠️ 手动输入地址").WithTitleTopCenter().Println(
		"请仔细输入矿工地址\n\n" +
			"重要提示:\n" +
			"• 地址必须完全正确，否则挖矿收益将丢失\n" +
			"• 建议复制粘贴，避免手动输入错误\n" +
			"• 确保该地址属于您，否则收益将发送给他人",
	)

	addressStr, err := m.ui.ShowInputDialog("输入", "矿工地址:", false)
	if err != nil {
		return "", nil, err
	}

	if addressStr == "" {
		m.ui.ShowWarning("地址不能为空")
		m.waitForContinue()
		return "", nil, nil
	}

	// 验证并转换地址
	minerAddressBytes, err := m.validateAndConvertAddress(addressStr)
	if err != nil {
		return "", nil, err
	}

	return addressStr, minerAddressBytes, nil
}

// validateAndConvertAddress 验证并转换地址格式
func (m *MiningCommands) validateAndConvertAddress(addressStr string) ([]byte, error) {
	if m.addressManager == nil {
		m.ui.ShowError("地址管理器不可用")
		m.waitForContinue()
		return nil, fmt.Errorf("地址管理器不可用")
	}

	parsed, parseErr := m.addressManager.StringToAddress(addressStr)
	if parseErr != nil {
		m.ui.ShowError(fmt.Sprintf("地址格式无效: %v", parseErr))
		m.waitForContinue()
		return nil, parseErr
	}

	minerAddressBytes, convErr := m.addressManager.AddressToBytes(parsed)
	if convErr != nil {
		m.ui.ShowError(fmt.Sprintf("地址转换失败: %v", convErr))
		m.waitForContinue()
		return nil, convErr
	}

	return minerAddressBytes, nil
}

// waitForContinue 等待用户按任意键继续
func (m *MiningCommands) waitForContinue() {
	pterm.Println()
	ui.ShowStandardWaitPrompt("continue")
}
