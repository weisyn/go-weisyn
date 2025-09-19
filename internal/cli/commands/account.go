package commands

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/pterm/pterm"

	"github.com/weisyn/v1/internal/cli/client"
	"github.com/weisyn/v1/internal/cli/ui"
	walletpkg "github.com/weisyn/v1/internal/cli/wallet"
	blockchainintf "github.com/weisyn/v1/pkg/interfaces/blockchain"
	cryptointf "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
	"github.com/weisyn/v1/pkg/utils"
)

// AccountCommands 账户管理命令处理器 - 直接使用真实接口
type AccountCommands struct {
	logger           log.Logger
	apiClient        *client.Client
	ui               ui.Components
	accountService   blockchainintf.AccountService // 📊 账户服务（真实接口）
	keyManager       cryptointf.KeyManager         // 🔑 密钥管理（真实接口）
	addressManager   cryptointf.AddressManager     // 🏠 地址管理（真实接口）
	signatureManager cryptointf.SignatureManager   // ✍️ 签名管理（真实接口）
	walletManager    walletpkg.WalletManager       // 💼 本地钱包管理
}

// NewAccountCommands 创建账户命令处理器 - 直接接收真实接口
func NewAccountCommands(
	logger log.Logger,
	apiClient *client.Client,
	ui ui.Components,
	accountService blockchainintf.AccountService,
	keyManager cryptointf.KeyManager,
	addressManager cryptointf.AddressManager,
	signatureManager cryptointf.SignatureManager,
	walletManager walletpkg.WalletManager,
) *AccountCommands {
	return &AccountCommands{
		logger:           logger,
		apiClient:        apiClient,
		ui:               ui,
		accountService:   accountService,
		keyManager:       keyManager,
		addressManager:   addressManager,
		signatureManager: signatureManager,
		walletManager:    walletManager,
	}
}

// ShowAccountMenu 显示账户管理菜单 - 统一子菜单入口
func (a *AccountCommands) ShowAccountMenu(ctx context.Context) error {
	for {
		// 清屏并显示统一页面头部（避免重复清屏导致的闪烁/黑屏）
		ui.ShowPageHeader()

		pterm.DefaultSection.Println("💰 账户管理")
		pterm.Println()

		// 首次进入时输出静态说明，避免用户看到空白
		pterm.DefaultBox.WithTitle("账户管理功能").WithTitleTopCenter().Println(
			"创建/导入钱包、解锁、设置默认、查询余额等",
		)
		pterm.Println()

		// 显示钱包状态摘要与解锁引导
		if a.walletManager != nil {
			if wallets, err := a.walletManager.ListWallets(ctx); err == nil {
				total := len(wallets)
				unlocked := 0
				defaultName := ""
				defaultStatus := ""
				for _, w := range wallets {
					if w.IsUnlocked {
						unlocked++
					}
					if w.IsDefault {
						defaultName = w.Name
						if w.IsUnlocked {
							defaultStatus = "已解锁"
						} else {
							defaultStatus = "已锁定"
						}
					}
				}

				info := fmt.Sprintf("钱包数: %d | 已解锁: %d", total, unlocked)
				if defaultName != "" {
					info = fmt.Sprintf("%s | 默认: %s (%s)", info, defaultName, defaultStatus)
				}
				_ = a.ui.ShowInfo(info)
				_ = a.ui.ShowInfo("🔓 提示: 如显示为已锁定，请通过 '钱包管理' → '解锁钱包' 解锁后再进行转账/合约等操作")
				pterm.Println()
			}
		}

		// 显示菜单选项 - 基于真实接口功能
		options := []string{
			"钱包管理",
			"查询账户余额",
			"返回主菜单",
		}

		selectedIndex, err := a.ui.ShowMenu("请选择账户操作:", options)
		if err != nil {
			a.logger.Errorf("菜单选择失败: %v", err)
			a.ui.ShowError(fmt.Sprintf("菜单操作失败: %v", err))
			a.waitForContinue()
			continue
		}

		switch selectedIndex {
		case 0: // 钱包管理
			if err := a.showWalletManagementMenu(ctx); err != nil {
				a.logger.Errorf("钱包管理失败: %v", err)
				a.ui.ShowError(fmt.Sprintf("钱包管理失败: %v", err))
				a.waitForContinue()
			}
		case 1: // 查询账户余额
			if err := a.ShowBalance(ctx); err != nil {
				a.logger.Errorf("查询余额失败: %v", err)
				a.ui.ShowError(fmt.Sprintf("查询余额失败: %v", err))
				a.waitForContinue()
			}
		case 2: // 返回主菜单
			return nil
		default:
			a.ui.ShowWarning("无效的选择，请重新选择")
			a.waitForContinue()
			continue
		}
	}
}

// ShowBalance 显示账户余额 - 修复界面版本
func (a *AccountCommands) ShowBalance(ctx context.Context) error {
	// 统一页面头部显示
	ui.ShowPageHeader()

	pterm.DefaultSection.Println("💰 查询账户余额")
	pterm.Println()

	// 检查密钥管理服务是否可用
	if a.keyManager == nil || a.addressManager == nil {
		ui.ShowServiceUnavailableState("密钥管理")
		a.waitForContinue()
		return nil
	}

	// 选择来源：从本地钱包选择 或 手动输入
	addrSourceIdx, err := a.ui.ShowMenu("选择地址来源", []string{"从本地钱包选择", "手动输入地址"})
	if err != nil {
		return err
	}

	var address string
	if addrSourceIdx == 0 {
		if a.walletManager == nil {
			a.ui.ShowError("钱包管理器不可用")
			return nil
		}
		wallets, wErr := a.walletManager.ListWallets(ctx)
		if wErr != nil {
			a.ui.ShowError(fmt.Sprintf("加载钱包失败: %v", wErr))
			return nil
		}
		if len(wallets) > 0 {
			display := make([]ui.WalletDisplayInfo, 0, len(wallets))
			for _, w := range wallets {
				display = append(display, ui.WalletDisplayInfo{ID: w.ID, Name: w.Name, Address: w.Address, Balance: "--", IsLocked: !w.IsUnlocked})
			}
			idx, selErr := a.ui.ShowWalletSelector(display)
			if selErr != nil {
				return selErr
			}
			address = wallets[idx].Address
		} else {
			a.ui.ShowWarning("未找到本地钱包，将切换为手动输入")
		}
	}

	if address == "" {
		// 手动输入
		addressInput, inputErr := a.ui.ShowInputDialog("输入地址", "请输入要查询的账户地址:", false)
		if inputErr != nil {
			return fmt.Errorf("获取地址输入失败: %w", inputErr)
		}
		if addressInput == "" {
			a.ui.ShowError("地址不能为空")
			return nil
		}
		address = addressInput
	}

	// 解析地址并转换为字节
	parsedAddress, parseErr := a.addressManager.StringToAddress(address)
	if parseErr != nil {
		a.ui.ShowError(fmt.Sprintf("地址格式无效: %v", parseErr))
		a.waitForContinue()
		return nil
	}
	addressBytes, convErr := a.addressManager.AddressToBytes(parsedAddress)
	if convErr != nil {
		a.ui.ShowError(fmt.Sprintf("地址转换失败: %v", convErr))
		a.waitForContinue()
		return nil
	}

	// 选择查询类型
	qIdx, qErr := a.ui.ShowMenu("选择查询类型", []string{
		"原生币（WES）",
		"所有合约代币（汇总）",
		"指定合约代币（输入TokenID）",
	})
	if qErr != nil {
		return qErr
	}

	switch qIdx {
	case 0:
		// 原生币余额
		progress := ui.StartSpinner("正在查询原生币余额...")
		balance, balanceErr := a.accountService.GetPlatformBalance(ctx, addressBytes)
		progress.Stop()

		ui.SwitchToResultPage("💰 原生币余额（WES）")
		if balanceErr != nil {
			ui.ShowNetworkErrorState("获取原生币余额", balanceErr.Error())
			a.waitForContinue()
			return nil
		}

		clientBalance := &client.BalanceInfo{
			Address: struct {
				RawHash string `json:"raw_hash"`
			}{RawHash: address},
			TokenID:   nil,
			Available: balance.Available,
			Locked:    balance.Locked,
			Total:     balance.Total,
		}
		// 使用标准格式化函数显示用户友好的WES单位
		formattedAmount := utils.FormatWeiToDecimal(balance.Available)
		a.ui.ShowBalanceInfo(address, clientBalance.ToFloat64(), "WES ("+formattedAmount+" WES)")
		a.waitForContinue()
		return nil

	case 1:
		// 所有合约代币余额
		progress := ui.StartSpinner("正在查询合约代币余额...")
		allBalances, allErr := a.accountService.GetAllTokenBalances(ctx, addressBytes)
		progress.Stop()

		ui.SwitchToResultPage("📦 合约代币余额（汇总）")
		if allErr != nil {
			ui.ShowNetworkErrorState("获取合约代币余额", allErr.Error())
			a.waitForContinue()
			return nil
		}
		if len(allBalances) == 0 {
			ui.ShowEmptyState("合约代币余额", "该地址暂无任何合约代币余额", []string{"返回账户管理菜单", "切换地址后重试"})
			a.waitForContinue()
			return nil
		}

		// 🔥 过滤：合约代币余额查询应该排除原生代币
		contractTokenBalances := make(map[string]*types.BalanceInfo)
		for tokenKey, b := range allBalances {
			// 跳过原生代币（tokenKey为空或TokenID为空）
			if tokenKey != "" && b.TokenID != nil {
				contractTokenBalances[tokenKey] = b
			}
		}

		if len(contractTokenBalances) == 0 {
			ui.ShowEmptyState("合约代币余额", "该地址暂无任何合约代币余额", []string{"返回账户管理菜单", "切换地址后重试"})
			a.waitForContinue()
			return nil
		}

		data := [][]string{{"TokenID", "可用余额", "锁定余额", "待确认余额", "总余额"}}
		for tokenKey, b := range contractTokenBalances {
			// 合约代币 - 显示TokenID和原始单位（因为不知道小数位数）
			tokenHex := tokenKey
			if tokenHex == "" && len(b.TokenID) > 0 {
				// TokenID是[]byte类型，直接使用
				tokenHex = hex.EncodeToString(b.TokenID)
			}
			// 安全地截短显示TokenID
			var tokenDisplay string
			if len(tokenHex) > 16 {
				tokenDisplay = tokenHex[:16] + "..."
			} else {
				tokenDisplay = tokenHex
			}

			data = append(data, []string{
				tokenDisplay,
				fmt.Sprintf("%d (原始)", b.Available),
				fmt.Sprintf("%d (原始)", b.Locked),
				fmt.Sprintf("%d (原始)", b.Pending),
				fmt.Sprintf("%d (原始)", b.Total),
			})
		}
		a.ui.ShowTable("代币余额", data)
		a.waitForContinue()
		return nil

	case 2:
		// 指定合约代币
		tokenHex, iErr := a.ui.ShowInputDialog("输入", "请输入代币TokenID（32字节十六进制）:", false)
		if iErr != nil {
			return iErr
		}
		tokenIDBytes, dErr := hex.DecodeString(strings.TrimSpace(tokenHex))
		if dErr != nil || len(tokenIDBytes) != 32 {
			a.ui.ShowError("TokenID格式错误，需32字节十六进制字符串")
			a.waitForContinue()
			return nil
		}

		progress := ui.StartSpinner("正在查询指定代币余额...")
		tb, tErr := a.accountService.GetTokenBalance(ctx, addressBytes, tokenIDBytes)
		progress.Stop()

		ui.SwitchToResultPage("📦 指定代币余额")
		if tErr != nil {
			ui.ShowNetworkErrorState("获取指定代币余额", tErr.Error())
			a.waitForContinue()
			return nil
		}

		pterm.DefaultBox.WithTitle("📦 合约代币余额").WithTitleTopCenter().Println(
			fmt.Sprintf("TokenID: %s\n可用: %d\n锁定: %d\n待确认: %d\n总额: %d", tokenHex, tb.Available, tb.Locked, tb.Pending, tb.Total),
		)
		a.waitForContinue()
		return nil
	}

	return nil
}

// ShowAllAccounts 显示所有账户 - 基于真实接口的简化版本
func (a *AccountCommands) ShowAllAccounts(ctx context.Context) error {
	// 显示本地钱包列表
	ui.SwitchToResultPage("💰 本地钱包列表")

	if a.walletManager == nil {
		ui.ShowServiceUnavailableState("钱包管理")
		a.waitForContinue()
		return nil
	}

	wallets, err := a.walletManager.ListWallets(ctx)
	if err != nil {
		a.ui.ShowError(fmt.Sprintf("加载钱包失败: %v", err))
		a.waitForContinue()
		return nil
	}

	if len(wallets) == 0 {
		ui.ShowEmptyState(
			"💡 钱包列表",
			"尚未创建或导入任何钱包",
			[]string{"返回账户管理菜单", "在钱包管理中创建/导入钱包"},
		)
		a.waitForContinue()
		return nil
	}

	data := [][]string{{"ID", "名称", "地址", "默认", "状态"}}
	for _, w := range wallets {
		status := "🔓 已解锁"
		if !w.IsUnlocked {
			status = "🔒 已锁定"
		}
		def := ""
		if w.IsDefault {
			def = "✅"
		}
		data = append(data, []string{w.ID, w.Name, w.Address, def, status})
	}
	a.ui.ShowTable("本地钱包", data)
	a.waitForContinue()
	return nil
}

// CreateAccount 创建新账户 - 基于真实KeyManager接口
func (a *AccountCommands) CreateAccount(ctx context.Context) error {
	// 基于钱包管理器创建钱包（加密存储私钥）
	ui.SwitchToResultPage("🆕 创建钱包")

	if a.walletManager == nil {
		ui.ShowServiceUnavailableState("钱包管理")
		a.waitForContinue()
		return nil
	}

	name, err := a.ui.ShowInputDialog("输入", "钱包名称:", false)
	if err != nil {
		return err
	}
	if name == "" {
		a.ui.ShowError("钱包名称不能为空")
		return nil
	}
	password, err := a.ui.ShowInputDialog("输入密码", "设置钱包密码:", true)
	if err != nil {
		return err
	}
	if password == "" {
		a.ui.ShowError("密码不能为空")
		return nil
	}

	// 确认密码
	confirm, err := a.ui.ShowInputDialog("输入密码", "重复输入钱包密码:", true)
	if err != nil {
		return err
	}
	if confirm != password {
		a.ui.ShowError("两次输入的密码不一致")
		a.waitForContinue()
		return nil
	}

	desc, _ := a.ui.ShowInputDialog("输入", "钱包描述(可选):", false)

	progress := ui.StartSpinner("正在创建钱包...")
	_, createErr := a.walletManager.CreateWallet(ctx, &walletpkg.CreateWalletRequest{
		Name:        name,
		Password:    password,
		Description: desc,
	})
	progress.Stop()

	if createErr != nil {
		a.ui.ShowError(fmt.Sprintf("创建钱包失败: %v", createErr))
		a.waitForContinue()
		return nil
	}

	a.ui.ShowSuccess("钱包创建成功")
	a.waitForContinue()
	return nil
}

// ImportAccount 导入账户 - 基于真实接口简化版本
func (a *AccountCommands) ImportAccount(ctx context.Context) error {
	// 基于钱包管理器导入（安全加密存储）
	ui.SwitchToResultPage("📥 导入钱包")

	if a.walletManager == nil {
		ui.ShowServiceUnavailableState("钱包管理")
		a.waitForContinue()
		return nil
	}

	name, err := a.ui.ShowInputDialog("输入", "钱包名称:", false)
	if err != nil {
		return err
	}
	if name == "" {
		a.ui.ShowError("钱包名称不能为空")
		return nil
	}
	password, err := a.ui.ShowInputDialog("输入密码", "设置钱包密码:", true)
	if err != nil {
		return err
	}
	if password == "" {
		a.ui.ShowError("密码不能为空")
		return nil
	}
	privateKey, err := a.ui.ShowInputDialog("输入密码", "导入私钥(64位十六进制):", true)
	if err != nil {
		return err
	}
	if privateKey == "" {
		a.ui.ShowError("私钥不能为空")
		return nil
	}
	desc, _ := a.ui.ShowInputDialog("输入", "钱包描述(可选):", false)

	progress := ui.StartSpinner("正在导入钱包...")
	_, impErr := a.walletManager.ImportWallet(ctx, &walletpkg.ImportWalletRequest{
		Name:        name,
		Password:    password,
		PrivateKey:  privateKey,
		Mnemonic:    "",
		Description: desc,
	})
	progress.Stop()

	if impErr != nil {
		a.ui.ShowError(fmt.Sprintf("导入钱包失败: %v", impErr))
		a.waitForContinue()
		return nil
	}
	a.ui.ShowSuccess("钱包导入成功")
	a.waitForContinue()
	return nil
}

// ListWallets 列出所有钱包 - 调用现有的ShowAllAccounts方法
func (a *AccountCommands) ListWallets(ctx context.Context) error {
	return a.ShowAllAccounts(ctx)
}

// ExportWallet 导出钱包信息 - 基于真实接口的简化版本
func (a *AccountCommands) ExportWallet(ctx context.Context) error {
	// 预留：根据需要实现导出（注意安全）
	a.ui.ShowInfo("导出功能未启用，为安全起见不导出明文私钥")
	a.waitForContinue()
	return nil
}

func (a *AccountCommands) showWalletManagementMenu(ctx context.Context) error {
	ui.SwitchToResultPage("💼 钱包管理")

	options := []string{
		"创建钱包",
		"导入钱包",
		"查看钱包列表",
		"解锁钱包",
		"设置默认钱包",
		"导出私钥",
		"删除钱包",
		"修改密码",
		"返回上一层",
	}

	idx, err := a.ui.ShowMenu("请选择钱包操作:", options)
	if err != nil {
		return err
	}

	switch idx {
	case 0:
		ui.SwitchToResultPage("🆕 创建钱包")
		return a.CreateAccount(ctx)
	case 1:
		ui.SwitchToResultPage("📥 导入钱包")
		return a.ImportAccount(ctx)
	case 2:
		ui.SwitchToResultPage("💰 本地钱包列表")
		return a.ShowAllAccounts(ctx)
	case 3:
		ui.SwitchToResultPage("🔓 解锁钱包")
		return a.unlockWalletFlow(ctx)
	case 4:
		ui.SwitchToResultPage("✅ 设置默认钱包")
		return a.setDefaultWalletFlow(ctx)
	case 5:
		ui.SwitchToResultPage("🔑 导出私钥")
		return a.exportWalletPrivateKeyFlow(ctx)
	case 6:
		ui.SwitchToResultPage("🗑️ 删除钱包")
		return a.deleteWalletFlow(ctx)
	case 7:
		ui.SwitchToResultPage("🔐 修改钱包密码")
		return a.changeWalletPasswordFlow(ctx)
	case 8:
		return nil
	default:
		return nil
	}
}

func (a *AccountCommands) unlockWalletFlow(ctx context.Context) error {
	if a.walletManager == nil {
		ui.ShowServiceUnavailableState("钱包管理")
		a.waitForContinue()
		return nil
	}
	wallets, err := a.walletManager.ListWallets(ctx)
	if err != nil || len(wallets) == 0 {
		a.ui.ShowWarning("未找到钱包")
		a.waitForContinue()
		return nil
	}
	display := make([]ui.WalletDisplayInfo, 0, len(wallets))
	for _, w := range wallets {
		display = append(display, ui.WalletDisplayInfo{ID: w.ID, Name: w.Name, Address: w.Address, Balance: "--", IsLocked: !w.IsUnlocked})
	}
	idx, err := a.ui.ShowWalletSelector(display)
	if err != nil {
		return err
	}
	password, err := a.ui.ShowInputDialog("输入密码", "钱包密码:", true)
	if err != nil {
		return err
	}
	if err := a.walletManager.UnlockWallet(ctx, wallets[idx].ID, password); err != nil {
		a.ui.ShowError("解锁失败: " + err.Error())
	} else {
		a.ui.ShowSuccess("钱包已解锁")
	}
	a.waitForContinue()
	return nil
}

func (a *AccountCommands) setDefaultWalletFlow(ctx context.Context) error {
	if a.walletManager == nil {
		ui.ShowServiceUnavailableState("钱包管理")
		a.waitForContinue()
		return nil
	}
	wallets, err := a.walletManager.ListWallets(ctx)
	if err != nil || len(wallets) == 0 {
		a.ui.ShowWarning("未找到钱包")
		a.waitForContinue()
		return nil
	}
	display := make([]ui.WalletDisplayInfo, 0, len(wallets))
	for _, w := range wallets {
		display = append(display, ui.WalletDisplayInfo{ID: w.ID, Name: w.Name, Address: w.Address, Balance: "--", IsLocked: !w.IsUnlocked})
	}
	idx, err := a.ui.ShowWalletSelector(display)
	if err != nil {
		return err
	}
	if err := a.walletManager.SetDefaultWallet(ctx, wallets[idx].ID); err != nil {
		a.ui.ShowError("设置默认钱包失败: " + err.Error())
	} else {
		a.ui.ShowSuccess("默认钱包已设置")
	}
	a.waitForContinue()
	return nil
}

func (a *AccountCommands) deleteWalletFlow(ctx context.Context) error {
	if a.walletManager == nil {
		ui.ShowServiceUnavailableState("钱包管理")
		a.waitForContinue()
		return nil
	}
	wallets, err := a.walletManager.ListWallets(ctx)
	if err != nil || len(wallets) == 0 {
		a.ui.ShowWarning("未找到钱包")
		a.waitForContinue()
		return nil
	}
	display := make([]ui.WalletDisplayInfo, 0, len(wallets))
	for _, w := range wallets {
		display = append(display, ui.WalletDisplayInfo{ID: w.ID, Name: w.Name, Address: w.Address, Balance: "--", IsLocked: !w.IsUnlocked})
	}
	idx, err := a.ui.ShowWalletSelector(display)
	if err != nil {
		return err
	}
	// 验证密码
	password, err := a.ui.ShowInputDialog("输入密码", "请输入钱包密码以确认删除:", true)
	if err != nil {
		return err
	}
	if ok, vErr := a.walletManager.ValidatePassword(ctx, wallets[idx].ID, password); vErr != nil || !ok {
		if vErr != nil {
			a.ui.ShowError("验证密码失败: " + vErr.Error())
		} else {
			a.ui.ShowError("密码不正确，无法删除")
		}
		a.waitForContinue()
		return nil
	}
	ok, err := a.ui.ShowConfirmDialog("确认删除", "此操作不可恢复，确认删除该钱包？")
	if err != nil || !ok {
		return nil
	}
	if err := a.walletManager.DeleteWallet(ctx, wallets[idx].ID); err != nil {
		a.ui.ShowError("删除失败: " + err.Error())
	} else {
		a.ui.ShowSuccess("钱包已删除")
	}
	a.waitForContinue()
	return nil
}

func (a *AccountCommands) exportWalletPrivateKeyFlow(ctx context.Context) error {
	if a.walletManager == nil {
		ui.ShowServiceUnavailableState("钱包管理")
		a.waitForContinue()
		return nil
	}
	wallets, err := a.walletManager.ListWallets(ctx)
	if err != nil || len(wallets) == 0 {
		a.ui.ShowWarning("未找到钱包")
		a.waitForContinue()
		return nil
	}
	display := make([]ui.WalletDisplayInfo, 0, len(wallets))
	for _, w := range wallets {
		display = append(display, ui.WalletDisplayInfo{ID: w.ID, Name: w.Name, Address: w.Address, Balance: "--", IsLocked: !w.IsUnlocked})
	}
	idx, err := a.ui.ShowWalletSelector(display)
	if err != nil {
		return err
	}
	// 输入密码获取私钥
	password, err := a.ui.ShowInputDialog("输入密码", "请输入钱包密码以导出私钥:", true)
	if err != nil {
		return err
	}
	priv, gErr := a.walletManager.GetPrivateKey(ctx, wallets[idx].ID, password)
	if gErr != nil {
		a.ui.ShowError("获取私钥失败: " + gErr.Error())
		a.waitForContinue()
		return nil
	}

	// 选择导出方式
	methodIdx, mErr := a.ui.ShowMenu("选择导出方式", []string{"在屏幕显示(高风险)", "保存到文件(推荐)", "取消"})
	if mErr != nil {
		return mErr
	}
	switch methodIdx {
	case 0:
		// 风险确认
		ok, cErr := a.ui.ShowConfirmDialog("高风险操作", "确认在屏幕显示明文私钥？")
		if cErr != nil || !ok {
			return nil
		}
		pterm.DefaultBox.WithTitle("🔑 私钥 (十六进制)").WithTitleTopCenter().Println(
			fmt.Sprintf("%x", priv),
		)
		a.ui.ShowWarning("请立即复制并妥善保存，窗口中显示存在泄露风险")
	case 1:
		path, iErr := a.ui.ShowInputDialog("保存路径", "请输入保存文件路径:", false)
		if iErr != nil {
			return iErr
		}
		if path == "" {
			a.ui.ShowError("保存路径不能为空")
			a.waitForContinue()
			return nil
		}
		if wErr := os.WriteFile(path, []byte(fmt.Sprintf("%x", priv)), 0600); wErr != nil {
			a.ui.ShowError("写入文件失败: " + wErr.Error())
			a.waitForContinue()
			return nil
		}
		a.ui.ShowSuccess("私钥已保存到文件 (0600)")
	default:
		return nil
	}
	a.waitForContinue()
	return nil
}

func (a *AccountCommands) changeWalletPasswordFlow(ctx context.Context) error {
	if a.walletManager == nil {
		ui.ShowServiceUnavailableState("钱包管理")
		a.waitForContinue()
		return nil
	}
	wallets, err := a.walletManager.ListWallets(ctx)
	if err != nil || len(wallets) == 0 {
		a.ui.ShowWarning("未找到钱包")
		a.waitForContinue()
		return nil
	}
	display := make([]ui.WalletDisplayInfo, 0, len(wallets))
	for _, w := range wallets {
		display = append(display, ui.WalletDisplayInfo{ID: w.ID, Name: w.Name, Address: w.Address, Balance: "--", IsLocked: !w.IsUnlocked})
	}
	idx, err := a.ui.ShowWalletSelector(display)
	if err != nil {
		return err
	}
	oldPwd, err := a.ui.ShowInputDialog("输入密码", "旧密码:", true)
	if err != nil {
		return err
	}
	newPwd, err := a.ui.ShowInputDialog("输入密码", "新密码:", true)
	if err != nil {
		return err
	}
	if err := a.walletManager.ChangePassword(ctx, wallets[idx].ID, oldPwd, newPwd); err != nil {
		a.ui.ShowError("修改失败: " + err.Error())
	} else {
		a.ui.ShowSuccess("密码已修改")
	}
	a.waitForContinue()
	return nil
}

// waitForContinue 等待用户按任意键继续
func (a *AccountCommands) waitForContinue() {
	pterm.Println()
	ui.ShowStandardWaitPrompt("continue")
}
