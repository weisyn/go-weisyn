package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/pterm/pterm"

	"github.com/weisyn/v1/internal/cli/client"
	"github.com/weisyn/v1/internal/cli/ui"
	walletpkg "github.com/weisyn/v1/internal/cli/wallet"
	blockchainintf "github.com/weisyn/v1/pkg/interfaces/blockchain"
	cryptointf "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// TransferCommands 转账操作命令处理器 - 直接使用真实接口
type TransferCommands struct {
	logger             log.Logger
	apiClient          *client.Client
	ui                 ui.Components
	transactionService blockchainintf.TransactionService // 💸 交易服务（真实接口）
	transactionManager blockchainintf.TransactionManager // 🔄 交易管理（真实接口）
	addressManager     cryptointf.AddressManager         // 🏠 地址管理（真实接口）
	signatureManager   cryptointf.SignatureManager       // ✍️ 签名管理（真实接口）
	walletManager      walletpkg.WalletManager           // 🔐 本地钱包管理（用于选择地址与解锁）
}

// NewTransferCommands 创建转账命令处理器 - 直接接收真实接口
func NewTransferCommands(
	logger log.Logger,
	apiClient *client.Client,
	ui ui.Components,
	transactionService blockchainintf.TransactionService,
	transactionManager blockchainintf.TransactionManager,
	addressManager cryptointf.AddressManager,
	signatureManager cryptointf.SignatureManager,
	walletManager walletpkg.WalletManager,
) *TransferCommands {
	return &TransferCommands{
		logger:             logger,
		apiClient:          apiClient,
		ui:                 ui,
		transactionService: transactionService,
		transactionManager: transactionManager,
		addressManager:     addressManager,
		signatureManager:   signatureManager,
		walletManager:      walletManager,
	}
}

// InteractiveTransfer 交互式转账 - 基于真实接口
func (t *TransferCommands) InteractiveTransfer(ctx context.Context) error {
	// 生成CLI请求ID用于跨终端日志追踪
	requestID := fmt.Sprintf("CLI-%d", time.Now().UnixNano())
	t.logger.Infof("💻 [%s] 开始交互式转账流程", requestID)

	// 检查交易服务是否可用
	if t.transactionService == nil || t.transactionManager == nil || t.addressManager == nil {
		t.logger.Errorf("💻 [%s] ❌ 交易服务不可用", requestID)
		ui.ShowServiceUnavailableState("交易管理")
		t.waitForContinue()
		return nil
	}

	t.logger.Infof("💻 [%s] ✅ 交易服务检查通过", requestID)

	// 阶段1：转账交易页面
	ui.SwitchToResultPage("💸 转账交易")

	pterm.Println("📝 转账功能")
	pterm.Println("请选择用于发送的本地钱包，并输入密码解锁")
	pterm.Println()

	// 阶段2：选择钱包并解锁获取私钥
	t.logger.Infof("💻 [%s] 📱 开始钱包选择和解锁流程", requestID)
	privateKeyBytes, fromAddress, err := t.SelectWalletAndGetPrivateKey(ctx)
	if err != nil {
		t.logger.Errorf("💻 [%s] ❌ 钱包解锁失败: %v", requestID, err)
		return err
	}
	t.logger.Infof("💻 [%s] ✅ 钱包解锁成功，发送地址: %s", requestID, fromAddress)

	toAddress, err := t.ui.ShowInputDialog("输入", "接收方地址:", false)
	if err != nil {
		return err
	}

	if toAddress == "" {
		t.logger.Warnf("💻 [%s] ⚠️ 用户未输入接收方地址", requestID)
		t.ui.ShowError("接收方地址不能为空")
		t.waitForContinue()
		return nil
	}
	t.logger.Infof("💻 [%s] 📍 接收方地址: %s", requestID, toAddress)

	amount, err := t.ui.ShowInputDialog("输入", "转账金额 (WES):", false)
	if err != nil {
		return err
	}

	if amount == "" {
		t.logger.Warnf("💻 [%s] ⚠️ 用户未输入转账金额", requestID)
		t.ui.ShowError("转账金额不能为空")
		t.waitForContinue()
		return nil
	}
	t.logger.Infof("💻 [%s] 💰 转账金额: %s WES", requestID, amount)

	memo, err := t.ui.ShowInputDialog("输入", "转账备注 (可选):", false)
	if err != nil {
		return err
	}
	t.logger.Infof("💻 [%s] 📝 转账备注: %s", requestID, memo)

	// 阶段3：确认转账信息
	ui.SwitchToResultPage("💸 转账确认")

	pterm.DefaultBox.WithTitle("📋 转账确认").WithTitleTopCenter().Println(
		fmt.Sprintf("从地址: %s\n", fromAddress) +
			fmt.Sprintf("到地址: %s\n", toAddress) +
			fmt.Sprintf("金额: %s WES\n", amount) +
			fmt.Sprintf("备注: %s\n\n", memo) +
			"⚠️ 系统将执行真实的转账操作",
	)

	confirmed, err := t.ui.ShowConfirmDialog("确认转账", "确认执行转账操作?")
	if err != nil || !confirmed {
		t.logger.Infof("💻 [%s] ❌ 用户取消转账操作", requestID)
		t.ui.ShowInfo("转账操作已取消")
		t.waitForContinue()
		return nil
	}

	t.logger.Infof("💻 [%s] ✅ 用户确认转账，开始执行", requestID)

	// 阶段4：执行转账 - 使用真实接口
	progress := ui.StartSpinner("正在构建交易...")

	// 步骤1：构建交易（直接调用TransactionService真实接口）
	t.logger.Infof("💻 [%s] 🔄 步骤1: 开始构建转账交易", requestID)
	txHash, err := t.transactionService.TransferAsset(ctx,
		privateKeyBytes, // 发送方私钥
		toAddress,       // 接收方地址
		amount,          // 转账金额
		"",              // 空字符串表示原生代币
		memo,            // 转账备注
	)

	if err != nil {
		t.logger.Errorf("💻 [%s] ❌ 步骤1: 构建交易失败: %v", requestID, err)
		progress.Stop()
		ui.SwitchToResultPage("💸 转账失败")
		t.ui.ShowError(fmt.Sprintf("构建交易失败: %v", err))
		t.waitForContinue()
		return nil
	}

	t.logger.Infof("💻 [%s] ✅ 步骤1: 交易构建成功，TxHash: %x", requestID, txHash)
	progress.UpdateMessage("正在签名交易...")

	// 步骤2：签名交易（直接调用TransactionManager真实接口）
	t.logger.Infof("💻 [%s] 🔄 步骤2: 开始签名交易", requestID)
	signedTxHash, err := t.transactionManager.SignTransaction(ctx, txHash, privateKeyBytes)
	if err != nil {
		t.logger.Errorf("💻 [%s] ❌ 步骤2: 签名交易失败: %v", requestID, err)
		progress.Stop()
		ui.SwitchToResultPage("💸 转账失败")
		t.ui.ShowError(fmt.Sprintf("签名交易失败: %v", err))
		t.waitForContinue()
		return nil
	}

	t.logger.Infof("💻 [%s] ✅ 步骤2: 交易签名成功，SignedTxHash: %x", requestID, signedTxHash)
	progress.UpdateMessage("正在提交到网络...")

	// 步骤3：提交交易（直接调用TransactionManager真实接口）
	t.logger.Infof("💻 [%s] 🔄 步骤3: 开始提交交易到网络", requestID)
	err = t.transactionManager.SubmitTransaction(ctx, signedTxHash)
	if err != nil {
		t.logger.Errorf("💻 [%s] ❌ 步骤3: 提交交易失败: %v", requestID, err)
		progress.Stop()
		ui.SwitchToResultPage("💸 转账失败")
		t.ui.ShowError(fmt.Sprintf("提交交易失败: %v", err))
		t.waitForContinue()
		return nil
	}

	t.logger.Infof("💻 [%s] ✅ 步骤3: 交易提交成功", requestID)
	progress.Stop()

	// 阶段5：转账成功页面
	ui.SwitchToResultPage("💸 转账成功")

	pterm.DefaultBox.WithTitle("✅ 转账提交成功").WithTitleTopCenter().Println(
		fmt.Sprintf("交易哈希: %x\n", signedTxHash) +
			fmt.Sprintf("接收地址: %s\n", toAddress) +
			fmt.Sprintf("转账金额: %s WES\n", amount) +
			fmt.Sprintf("转账备注: %s\n\n", memo) +
			"💡 交易已提交到区块链网络，等待确认\n" +
			"💡 可以使用「区块链信息」菜单查看交易状态",
	)

	t.logger.Infof("💻 [%s] 🎉 转账交易提交成功: txHash=%x, to=%s, amount=%s",
		requestID, signedTxHash, toAddress, amount)

	t.logger.Infof("💻 [%s] 📋 转账完成汇总: From=%s, To=%s, Amount=%s WES, Memo=%s",
		requestID, fromAddress, toAddress, amount, memo)

	t.waitForContinue()
	t.logger.Infof("💻 [%s] 🏁 交互式转账流程结束", requestID)
	return nil
}

// BatchTransfer 批量转账 - 基于真实接口
func (t *TransferCommands) BatchTransfer(ctx context.Context) error {
	ui.SwitchToResultPage("📦 批量转账")

	// 检查服务是否可用
	if t.transactionService == nil || t.transactionManager == nil {
		ui.ShowServiceUnavailableState("批量转账")
		t.waitForContinue()
		return nil
	}

	pterm.Println("📝 批量转账功能")
	pterm.Println("请选择用于发送的本地钱包，并输入密码解锁")
	pterm.Println()

	// 选择钱包并解锁
	_, fromAddress, err := t.SelectWalletAndGetPrivateKey(ctx)
	if err != nil {
		return err
	}

	// 收集收款信息
	pterm.Println("请逐个添加收款信息（输入空地址结束）:")

	var transfers []struct {
		ToAddress string
		Amount    string
		Memo      string
	}

	for i := 1; ; i++ {
		pterm.Printf("\n第 %d 个收款人:\n", i)

		address, err := t.ui.ShowInputDialog("输入", "收款地址 (留空结束):", false)
		if err != nil {
			return err
		}
		if address == "" {
			break
		}

		amount, err := t.ui.ShowInputDialog("输入", "转账金额 (WES):", false)
		if err != nil {
			return err
		}
		if amount == "" {
			pterm.Warning.Println("金额不能为空，跳过此条记录")
			continue
		}

		memo, err := t.ui.ShowInputDialog("输入", "备注 (可选):", false)
		if err != nil {
			return err
		}

		transfers = append(transfers, struct {
			ToAddress string
			Amount    string
			Memo      string
		}{
			ToAddress: address,
			Amount:    amount,
			Memo:      memo,
		})
	}

	if len(transfers) == 0 {
		t.ui.ShowInfo("没有添加任何收款人，批量转账取消")
		t.waitForContinue()
		return nil
	}

	// 确认批量转账信息
	ui.SwitchToResultPage("📦 批量转账确认")

	pterm.DefaultBox.WithTitle("📋 批量转账确认").WithTitleTopCenter().Println(
		fmt.Sprintf("发送地址: %s\n", fromAddress) +
			fmt.Sprintf("收款人数量: %d\n\n", len(transfers)) +
			"收款明细:",
	)

	for i, transfer := range transfers {
		pterm.Printf("  %d. %s -> %s WES (%s)\n",
			i+1, transfer.ToAddress, transfer.Amount, transfer.Memo)
	}
	pterm.Println()

	confirmed, err := t.ui.ShowConfirmDialog("确认批量转账", "确认执行批量转账操作?")
	if err != nil || !confirmed {
		t.ui.ShowInfo("批量转账操作已取消")
		t.waitForContinue()
		return nil
	}

	// 执行批量转账 - 注意：这里需要types.TransferParams，暂时用简化实现
	ui.SwitchToResultPage("📦 批量转账说明")

	ui.ShowEmptyState(
		"💡 批量转账接口说明",
		"真实BatchTransfer接口需要types.TransferParams类型",
		[]string{
			"返回转账菜单",
			"批量转账功能需要进一步完善",
			"当前展示基本的交互流程",
			"实际实现需要完整的types.TransferParams结构",
		},
	)

	t.waitForContinue()
	return nil
}

// TimeLockTransfer 时间锁转账 - 基于真实接口的高级选项说明
func (t *TransferCommands) TimeLockTransfer(ctx context.Context) error {
	ui.SwitchToResultPage("⏰ 时间锁转账")

	// 真实流程：选择钱包 → 解锁 → 输入对端与金额 → 指定时间锁参数 → 确认
	pterm.Println("📝 时间锁转账说明：选择钱包并解锁后，设置解锁条件进行转账")
	pterm.Println()

	t.waitForContinue()
	return nil
}

// ShowTransferHistory 转账历史说明 - 真实接口不提供历史查询
//

// ShowTransferMenu 显示转账菜单
func (t *TransferCommands) ShowTransferMenu(ctx context.Context) error {
	options := []string{
		"普通转账",
		"批量转账",
		"返回主菜单",
	}

	selectedIndex, err := t.ui.ShowMenu("转账操作", options)
	if err != nil {
		return err
	}

	switch selectedIndex {
	case 0:
		return t.InteractiveTransfer(ctx)
	case 1:
		return t.BatchTransfer(ctx)
	case 2:
		return nil
	default:
		return fmt.Errorf("无效选择")
	}
}

// waitForContinue 等待用户按任意键继续
func (t *TransferCommands) waitForContinue() {
	pterm.Println()
	ui.ShowStandardWaitPrompt("continue")
}

// SelectWalletAndGetPrivateKey 选择一个本地钱包并通过密码解锁以获取私钥
// 返回：私钥字节、钱包地址
func (t *TransferCommands) SelectWalletAndGetPrivateKey(ctx context.Context) ([]byte, string, error) {
	if t.walletManager == nil {
		t.ui.ShowError("钱包管理器不可用")
		t.waitForContinue()
		return nil, "", fmt.Errorf("钱包管理器不可用")
	}

	// 读取钱包列表
	wallets, err := t.walletManager.ListWallets(ctx)
	if err != nil {
		t.ui.ShowError(fmt.Sprintf("加载钱包失败: %v", err))
		t.waitForContinue()
		return nil, "", err
	}
	if len(wallets) == 0 {
		t.ui.ShowError("未找到本地钱包，请先在账户管理中创建或导入钱包")
		t.waitForContinue()
		return nil, "", fmt.Errorf("无钱包")
	}

	// 构建选择列表
	displayList := make([]ui.WalletDisplayInfo, 0, len(wallets))
	for _, w := range wallets {
		displayList = append(displayList, ui.WalletDisplayInfo{
			ID:       w.ID,
			Name:     w.Name,
			Address:  w.Address,
			Balance:  "--",
			IsLocked: !w.IsUnlocked,
		})
	}

	idx, err := t.ui.ShowWalletSelector(displayList)
	if err != nil {
		return nil, "", err
	}
	selected := wallets[idx]

	// 如果未解锁，提示输入密码并解锁
	if !selected.IsUnlocked {
		password, err := t.ui.ShowInputDialog("输入密码", "钱包密码:", true)
		if err != nil {
			return nil, "", err
		}
		if err := t.walletManager.UnlockWallet(ctx, selected.ID, password); err != nil {
			t.ui.ShowError(fmt.Sprintf("解锁失败: %v", err))
			t.waitForContinue()
			return nil, "", err
		}
	}

	// 读取私钥
	// 为避免重复输入密码，这里尝试空密码获取；若实现需要密码，按上一步密码重用
	// 简化：再次提示密码用于导出私钥
	password, err := t.ui.ShowInputDialog("输入密码", "确认钱包密码以签名:", true)
	if err != nil {
		return nil, "", err
	}

	priv, err := t.walletManager.GetPrivateKey(ctx, selected.ID, password)
	if err != nil {
		t.ui.ShowError(fmt.Sprintf("获取私钥失败: %v", err))
		t.waitForContinue()
		return nil, "", err
	}
	return priv, selected.Address, nil
}
