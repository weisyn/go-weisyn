// Package flows 提供可复用的交互流程
package flows

import (
	"context"
	"fmt"
	"strings"

	"github.com/weisyn/v1/client/pkg/tools/format"
	"github.com/weisyn/v1/client/pkg/ux/ui"
	"github.com/weisyn/v1/pkg/utils"
)

// AccountFlow 账户管理交互流程
//
// 功能：
//   - 提供账户相关的完整UI交互流程
//   - 解耦UI交互与后端实现
//   - 支持查询余额、创建/导入钱包、管理钱包等操作
//
// 依赖：
//   - ui.Components: UI组件接口
//   - AccountService: 账户服务端口
//   - WalletService: 钱包服务端口
//   - AddressValidator: 地址验证器端口
type AccountFlow struct {
	ui               ui.Components
	accountService   AccountService
	walletService    WalletService
	addressValidator AddressValidator
	contractBalance  ContractBalanceService
	tokenSpecs       []ContractTokenSpec
}

// NewAccountFlow 创建账户流程实例
func NewAccountFlow(
	uiComponents ui.Components,
	accountService AccountService,
	walletService WalletService,
	addressValidator AddressValidator,
	contractBalance ContractBalanceService,
	tokenSpecs []ContractTokenSpec,
) *AccountFlow {
	return &AccountFlow{
		ui:               uiComponents,
		accountService:   accountService,
		walletService:    walletService,
		addressValidator: addressValidator,
		contractBalance:  contractBalance,
		tokenSpecs:       tokenSpecs,
	}
}

// ============================================================================
// 查询余额流程
// ============================================================================

// ShowBalance 展示账户余额（交互式）
//
// 功能（对齐旧CLI）：
//   - 获取本地钱包列表
//   - 让用户选择一个钱包
//   - 查询该钱包的余额并展示
//   - 支持主币和代币余额展示
func (f *AccountFlow) ShowBalance(ctx context.Context) error {
	f.ui.ShowHeader("查询账户余额")

	// 1. 获取本地钱包列表
	wallets, err := f.walletService.ListWallets(ctx)
	if err != nil {
		f.ui.ShowError("获取钱包列表失败: " + err.Error())
		fmt.Println()
		f.ui.ShowInfo("💡 可能的原因：")
		f.ui.ShowInfo("   • 您还没有创建任何钱包")
		f.ui.ShowInfo("   • 钱包文件不存在或损坏")
		fmt.Println()
		f.ui.ShowInfo("📝 建议操作：")
		f.ui.ShowInfo("   1. 返回上一级菜单")
		f.ui.ShowInfo("   2. 选择 '创建账户'")
		f.ui.ShowInfo("   3. 按照提示完成钱包创建")
		return fmt.Errorf("获取钱包列表失败: %w", err)
	}

	if len(wallets) == 0 {
		f.ui.ShowWarning("暂无钱包，无法查看余额")
		fmt.Println()
		f.ui.ShowInfo("💡 提示：请先创建一个钱包")
		f.ui.ShowInfo("   返回上一级菜单 → 选择 '创建账户'")
		return fmt.Errorf("暂无钱包")
	}

WalletSelection:
	for {
		// 2. 构建钱包选项（追加“返回上一级”）
		walletNames := make([]string, len(wallets)+1)
		for i, w := range wallets {
			walletNames[i] = fmt.Sprintf("%s (%s)", w.Name, w.Address)
		}
		walletNames[len(wallets)] = "返回上一级"

		// 3. 让用户选择钱包
		selectedIdx, err := f.ui.ShowMenu("选择要查询的钱包", walletNames)
		if err != nil {
			f.ui.ShowError("选择失败: " + err.Error())
			return fmt.Errorf("选择钱包失败: %w", err)
		}

		// 返回上一层菜单
		if selectedIdx == len(wallets) {
			return nil
		}

		selectedWallet := wallets[selectedIdx]

		options := []string{
			"查询原生币余额",
			"查询合约代币余额",
			"返回钱包列表",
		}

		selectedAction, err := f.ui.ShowMenu("请选择查询类型", options)
		if err != nil {
			return fmt.Errorf("选择查询类型失败: %w", err)
		}

		switch selectedAction {
		case 0:
			if err := f.showNativeBalance(ctx, selectedWallet); err != nil {
				f.ui.ShowWarning(err.Error())
			}
			f.ui.ShowContinuePrompt("", "")
		case 1:
			if f.contractBalance == nil {
				f.ui.ShowWarning("当前环境暂不支持合约代币查询")
				f.ui.ShowContinuePrompt("", "")
				continue
			}
			if err := f.showContractBalance(ctx, selectedWallet); err != nil {
				f.ui.ShowWarning(err.Error())
			}
			f.ui.ShowContinuePrompt("", "")
		case 2:
			continue WalletSelection
		default:
			continue WalletSelection
		}
	}
}

func (f *AccountFlow) showNativeBalance(ctx context.Context, wallet WalletInfo) error {
	spinner := f.ui.ShowSpinner(fmt.Sprintf("正在查询 %s 的原生币余额...", wallet.Name))
	spinner.Start()

	balance, _, err := f.accountService.GetBalance(ctx, wallet.Address)
	spinner.Stop()

	if err != nil {
		return fmt.Errorf("查询原生币余额失败: %w", err)
	}

	f.ui.ShowHeader(fmt.Sprintf("原生币余额 - %s", wallet.Name))
	fmt.Println()
	f.ui.ShowInfo(fmt.Sprintf("  地址: %s", wallet.Address))
	// balance 为最小单位（BaseUnit），对用户展示时转换为 WES
	f.ui.ShowInfo(fmt.Sprintf("  余额: %s WES", utils.FormatWeiToDecimal(balance)))
	fmt.Println()

	return nil
}

func (f *AccountFlow) showContractBalance(ctx context.Context, wallet WalletInfo) error {
	contentHash, err := f.ui.ShowInputDialog("合约地址", "请输入合约 Content Hash（64 位十六进制）", false)
	if err != nil {
		return fmt.Errorf("读取合约地址失败: %w", err)
	}
	contentHash = strings.TrimSpace(contentHash)
	if contentHash == "" {
		return fmt.Errorf("合约地址不能为空")
	}

	tokenID, err := f.ui.ShowInputDialog("代币标识", "请输入代币 Token ID（可留空）", false)
	if err != nil {
		return fmt.Errorf("读取 Token ID 失败: %w", err)
	}
	tokenID = strings.TrimSpace(tokenID)

	label, err := f.ui.ShowInputDialog("展示名称", "请输入代币展示名称（可留空使用合约哈希前缀）", false)
	if err != nil {
		return fmt.Errorf("读取展示名称失败: %w", err)
	}
	label = strings.TrimSpace(label)

	sanitizedHash, err := sanitizeContentHash(contentHash)
	if err != nil {
		return fmt.Errorf("合约地址无效: %w", err)
	}

	if tokenID == "" {
		tokenID = "default"
	}

	if label == "" {
		label = generateDefaultLabel(sanitizedHash, tokenID)
	}

	spec := ContractTokenSpec{
		Label:       label,
		ContentHash: sanitizedHash,
		TokenID:     tokenID,
	}

	spinner := f.ui.ShowSpinner(fmt.Sprintf("正在查询 %s 的合约代币余额...", label))
	spinner.Start()

	balances, err := f.contractBalance.FetchBalances(ctx, wallet.Address, []ContractTokenSpec{spec})
	spinner.Stop()

	if err != nil {
		return fmt.Errorf("查询合约代币余额失败: %w", err)
	}

	var amount uint64
	if len(balances) > 0 {
		amount = balances[0].Amount
	}

	f.ui.ShowHeader("合约代币余额")
	fmt.Println()
	f.ui.ShowInfo(fmt.Sprintf("  钱包: %s (%s)", wallet.Name, wallet.Address))
	f.ui.ShowInfo(fmt.Sprintf("  合约: %s", sanitizedHash))
	if tokenID != "" {
		f.ui.ShowInfo(fmt.Sprintf("  Token ID: %s", tokenID))
	}
	// amount 为最小单位（BaseUnit），对用户展示时转换为 WES
	f.ui.ShowInfo(fmt.Sprintf("  余额: %s WES", utils.FormatWeiToDecimal(amount)))
	fmt.Println()

	return nil
}

func sanitizeContentHash(hash string) (string, error) {
	sanitized := strings.TrimSpace(strings.TrimPrefix(hash, "0x"))
	if len(sanitized) != 64 {
		return "", fmt.Errorf("长度必须是 64 个十六进制字符")
	}
	for _, r := range sanitized {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F') {
			return "", fmt.Errorf("包含非十六进制字符")
		}
	}
	return strings.ToLower(sanitized), nil
}

func generateDefaultLabel(contentHash, tokenID string) string {
	shortHash := contentHash
	if len(shortHash) > 8 {
		shortHash = shortHash[:8]
	}
	if tokenID != "" {
		return fmt.Sprintf("%s (%s)", shortHash, tokenID)
	}
	return shortHash
}

// GetBalanceByAddress 获取指定地址的余额（编程式调用）
//
// 功能：
//   - 直接查询指定地址余额
//   - 不包含UI交互
//   - 适用于命令行参数传入地址的场景
func (f *AccountFlow) GetBalanceByAddress(ctx context.Context, address string) (*BalanceInfo, error) {
	// 1. 验证地址
	valid, err := f.addressValidator.ValidateAddress(address)
	if !valid || err != nil {
		return nil, fmt.Errorf("地址无效: %w", err)
	}

	// 2. 查询余额
	balance, tokenBalances, err := f.accountService.GetBalance(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("查询余额失败: %w", err)
	}

	// 3. 返回结果
	return &BalanceInfo{
		Address:          address,
		Balance:          balance,
		BalanceFormatted: utils.FormatWeiToDecimal(balance),
		TokenBalances:    tokenBalances,
	}, nil
}

// ============================================================================
// 钱包列表流程
// ============================================================================

// ShowWalletList 展示钱包列表
//
// 功能：
//   - 查询所有钱包
//   - 格式化展示钱包信息（名称、地址、创建时间）
func (f *AccountFlow) ShowWalletList(ctx context.Context) error {
	f.ui.ShowHeader("钱包列表")

	// 1. 查询钱包列表
	wallets, err := f.walletService.ListWallets(ctx)
	if err != nil {
		f.ui.ShowError(fmt.Sprintf("查询钱包列表失败: %v", err))
		return fmt.Errorf("查询钱包列表失败: %w", err)
	}

	// 2. 检查是否为空
	if len(wallets) == 0 {
		f.ui.ShowInfo("暂无钱包，请先创建钱包")
		return nil
	}

	// 3. 格式化展示
	data := [][]string{{"钱包名称", "地址", "默认", "状态", "创建时间"}}
	for _, wallet := range wallets {
		defaultMark := ""
		if wallet.IsDefault {
			defaultMark = "✓"
		}
		lockStatus := "🔓 已解锁"
		if wallet.IsLocked {
			lockStatus = "🔒 已锁定"
		}
		data = append(data, []string{
			wallet.Name,
			format.FormatAddress(wallet.Address, 10, 8),
			defaultMark,
			lockStatus,
			wallet.CreatedAt.Format("2006-01-02 15:04"),
		})
	}

	f.ui.ShowTable("", data)

	return nil
}

// ============================================================================
// 创建钱包流程
// ============================================================================

// CreateWallet 创建新钱包（交互式）
//
// 功能：
//   - 提示用户输入钱包名称和密码
//   - 验证密码强度
//   - 创建钱包并展示结果
func (f *AccountFlow) CreateWallet(ctx context.Context) (*CreateWalletResult, error) {
	f.ui.ShowHeader("创建新钱包")

	// 1. 输入钱包名称
	name, err := f.ui.ShowInputDialog("钱包名称", "请输入钱包名称", false)
	if err != nil {
		return nil, fmt.Errorf("输入钱包名称失败: %w", err)
	}

	if name == "" {
		f.ui.ShowError("钱包名称不能为空")
		return nil, fmt.Errorf("钱包名称不能为空")
	}

	// 2. 输入密码
	password, err := f.ui.ShowInputDialog("密码", "请输入钱包密码（至少8位）", true)
	if err != nil {
		return nil, fmt.Errorf("输入密码失败: %w", err)
	}

	// 3. 验证密码强度
	if len(password) < 8 {
		f.ui.ShowError("密码长度不能少于8位")
		return nil, fmt.Errorf("密码强度不足")
	}

	// 4. 确认密码
	confirmPassword, err := f.ui.ShowInputDialog("确认密码", "请再次输入密码", true)
	if err != nil {
		return nil, fmt.Errorf("确认密码失败: %w", err)
	}

	if password != confirmPassword {
		f.ui.ShowError("两次输入的密码不一致")
		return nil, fmt.Errorf("密码不一致")
	}

	// 5. 显示加载动画
	spinner := f.ui.ShowSpinner("正在创建钱包...")
	spinner.Start()

	// 6. 创建钱包
	walletInfo, err := f.walletService.CreateWallet(ctx, name, password)
	spinner.Stop()

	if err != nil {
		f.ui.ShowError(fmt.Sprintf("创建钱包失败: %v", err))
		return nil, fmt.Errorf("创建钱包失败: %w", err)
	}

	// 7. 展示成功结果
	f.ui.ShowSuccess("钱包创建成功！")

	// 8. 显示助记词（关键步骤）
	if walletInfo.Mnemonic != "" {
		f.ui.ShowSecurityWarning("⚠️ 重要：请立即备份以下助记词！")
		fmt.Println()
		f.ui.ShowPanel("🔑 助记词（24个单词）", walletInfo.Mnemonic)
		fmt.Println()
		f.ui.ShowWarning("⚠️ 安全提示：")
		f.ui.ShowInfo("   • 助记词是恢复钱包的唯一方式")
		f.ui.ShowInfo("   • 请将助记词抄写在纸上，存放在安全的地方")
		f.ui.ShowInfo("   • 切勿截图、拍照或以电子方式存储")
		f.ui.ShowInfo("   • 切勿将助记词告诉任何人")
		f.ui.ShowInfo("   • 助记词丢失后，钱包将无法恢复！")
		fmt.Println()

		// 要求用户确认已备份
		f.ui.ShowContinuePrompt("确认已备份助记词", "按回车键继续...")
	}

	// 9. 显示钱包信息
	f.ui.ShowPanel("钱包信息", fmt.Sprintf(
		"钱包名称: %s\n地址: %s",
		walletInfo.Name,
		walletInfo.Address,
	))

	return &CreateWalletResult{
		WalletName: walletInfo.Name,
		Address:    walletInfo.Address,
		Success:    true,
		Message:    "钱包创建成功",
	}, nil
}

// ============================================================================
// 导入钱包流程
// ============================================================================

// ImportWallet 导入已有钱包（交互式）
//
// 功能：
//   - 提示用户输入钱包名称、私钥和密码
//   - 验证私钥格式
//   - 导入钱包并展示结果
func (f *AccountFlow) ImportWallet(ctx context.Context) (*ImportWalletResult, error) {
	f.ui.ShowHeader("导入钱包")

	// 1. 输入钱包名称
	name, err := f.ui.ShowInputDialog("钱包名称", "请输入钱包名称", false)
	if err != nil {
		return nil, fmt.Errorf("输入钱包名称失败: %w", err)
	}

	// 2. 输入私钥
	f.ui.ShowSecurityWarning("请确保在安全的环境中输入私钥！")
	privateKey, err := f.ui.ShowInputDialog("私钥", "请输入私钥（十六进制，64位）", true)
	if err != nil {
		return nil, fmt.Errorf("输入私钥失败: %w", err)
	}

	// 3. 验证私钥格式
	if len(privateKey) != 64 {
		f.ui.ShowError("私钥长度无效，应为64位十六进制字符")
		return nil, fmt.Errorf("私钥格式无效")
	}

	// 4. 输入密码
	password, err := f.ui.ShowInputDialog("密码", "请输入钱包密码（至少8位）", true)
	if err != nil {
		return nil, fmt.Errorf("输入密码失败: %w", err)
	}

	if len(password) < 8 {
		f.ui.ShowError("密码长度不能少于8位")
		return nil, fmt.Errorf("密码强度不足")
	}

	// 5. 显示加载动画
	spinner := f.ui.ShowSpinner("正在导入钱包...")
	spinner.Start()

	// 6. 导入钱包
	walletInfo, err := f.walletService.ImportWallet(ctx, name, privateKey, password)
	spinner.Stop()

	if err != nil {
		f.ui.ShowError(fmt.Sprintf("导入钱包失败: %v", err))
		return nil, fmt.Errorf("导入钱包失败: %w", err)
	}

	// 7. 展示成功结果
	f.ui.ShowSuccess("钱包导入成功！")
	f.ui.ShowPanel("钱包信息", fmt.Sprintf(
		"钱包名称: %s\n地址: %s",
		walletInfo.Name,
		walletInfo.Address,
	))

	return &ImportWalletResult{
		WalletName: walletInfo.Name,
		Address:    walletInfo.Address,
		Success:    true,
		Message:    "钱包导入成功",
	}, nil
}

// ============================================================================
// 删除钱包流程
// ============================================================================

// DeleteWallet 删除钱包（交互式）
//
// 功能：
//   - 列出所有钱包供用户选择
//   - 确认删除操作
//   - 删除钱包
func (f *AccountFlow) DeleteWallet(ctx context.Context) error {
	f.ui.ShowHeader("删除钱包")

	// 1. 查询钱包列表
	wallets, err := f.walletService.ListWallets(ctx)
	if err != nil {
		f.ui.ShowError(fmt.Sprintf("查询钱包列表失败: %v", err))
		return fmt.Errorf("查询钱包列表失败: %w", err)
	}

	if len(wallets) == 0 {
		f.ui.ShowInfo("暂无钱包")
		return nil
	}

	// 2. 选择钱包
	options := make([]string, len(wallets))
	for i, wallet := range wallets {
		options[i] = fmt.Sprintf("%s (%s)", wallet.Name, format.FormatAddress(wallet.Address, 10, 8))
	}

	selectedIndex, err := f.ui.ShowMenu("选择要删除的钱包", options)
	if err != nil {
		return fmt.Errorf("选择钱包失败: %w", err)
	}

	selectedWallet := wallets[selectedIndex]

	// 3. 安全警告
	f.ui.ShowSecurityWarning(fmt.Sprintf(
		"您即将删除钱包：%s\n地址：%s\n\n删除后无法恢复，请确保已备份私钥！",
		selectedWallet.Name,
		selectedWallet.Address,
	))

	// 4. 确认删除
	confirm, err := f.ui.ShowConfirmDialog("确认删除", "确定要删除此钱包吗？")
	if err != nil {
		return fmt.Errorf("确认失败: %w", err)
	}

	if !confirm {
		f.ui.ShowInfo("已取消删除操作")
		return nil
	}

	// 5. 删除钱包
	spinner := f.ui.ShowSpinner("正在删除钱包...")
	spinner.Start()

	err = f.walletService.DeleteWallet(ctx, selectedWallet.Name)
	spinner.Stop()

	if err != nil {
		f.ui.ShowError(fmt.Sprintf("删除钱包失败: %v", err))
		return fmt.Errorf("删除钱包失败: %w", err)
	}

	f.ui.ShowSuccess("钱包已删除")

	return nil
}

// ============================================================================
// 导出私钥流程
// ============================================================================

// ExportPrivateKey 导出私钥（交互式）
//
// 功能：
//   - 列出所有钱包供用户选择
//   - 验证密码
//   - 导出私钥并展示（含安全警告）
func (f *AccountFlow) ExportPrivateKey(ctx context.Context) (*ExportPrivateKeyResult, error) {
	f.ui.ShowHeader("导出私钥")

	// 1. 安全警告
	f.ui.ShowSecurityWarning("导出私钥存在极高安全风险！\n请确保在安全的环境中操作！\n切勿将私钥泄露给他人！")

	confirm, err := f.ui.ShowConfirmDialog("安全确认", "您确定要导出私钥吗？")
	if err != nil || !confirm {
		f.ui.ShowInfo("已取消操作")
		return nil, fmt.Errorf("用户取消操作")
	}

	// 2. 查询钱包列表
	wallets, err := f.walletService.ListWallets(ctx)
	if err != nil {
		f.ui.ShowError(fmt.Sprintf("查询钱包列表失败: %v", err))
		return nil, fmt.Errorf("查询钱包列表失败: %w", err)
	}

	if len(wallets) == 0 {
		f.ui.ShowInfo("暂无钱包")
		return nil, fmt.Errorf("暂无钱包")
	}

	// 3. 选择钱包
	options := make([]string, len(wallets))
	for i, wallet := range wallets {
		options[i] = fmt.Sprintf("%s (%s)", wallet.Name, format.FormatAddress(wallet.Address, 10, 8))
	}

	selectedIndex, err := f.ui.ShowMenu("选择钱包", options)
	if err != nil {
		return nil, fmt.Errorf("选择钱包失败: %w", err)
	}

	selectedWallet := wallets[selectedIndex]

	// 4. 输入密码
	password, err := f.ui.ShowInputDialog("密码验证", "请输入钱包密码", true)
	if err != nil {
		return nil, fmt.Errorf("输入密码失败: %w", err)
	}

	// 5. 导出私钥
	spinner := f.ui.ShowSpinner("正在导出私钥...")
	spinner.Start()

	privateKey, err := f.walletService.ExportPrivateKey(ctx, selectedWallet.Name, password)
	spinner.Stop()

	if err != nil {
		f.ui.ShowError(fmt.Sprintf("导出私钥失败: %v", err))
		return nil, fmt.Errorf("导出私钥失败: %w", err)
	}

	// 6. 展示私钥（含警告）
	f.ui.ShowPanel("私钥信息", fmt.Sprintf(
		"钱包名称: %s\n地址: %s\n私钥: %s\n\n⚠️⚠️⚠️ 严重安全警告 ⚠️⚠️⚠️\n私钥控制资产所有权！\n请立即备份并删除屏幕记录！\n切勿通过网络传输或截图分享！",
		selectedWallet.Name,
		selectedWallet.Address,
		privateKey,
	))

	return &ExportPrivateKeyResult{
		WalletName: selectedWallet.Name,
		Address:    selectedWallet.Address,
		PrivateKey: privateKey,
		Warning:    "⚠️ 私钥导出存在安全风险，请妥善保管！切勿泄露给他人！",
	}, nil
}

// ============================================================================
// 修改密码流程
// ============================================================================

// ChangePassword 修改钱包密码（交互式）
//
// 功能：
//   - 列出所有钱包供用户选择
//   - 验证旧密码
//   - 输入新密码并确认
//   - 修改密码
func (f *AccountFlow) ChangePassword(ctx context.Context) error {
	f.ui.ShowHeader("修改钱包密码")

	// 1. 查询钱包列表
	wallets, err := f.walletService.ListWallets(ctx)
	if err != nil {
		f.ui.ShowError(fmt.Sprintf("查询钱包列表失败: %v", err))
		return fmt.Errorf("查询钱包列表失败: %w", err)
	}

	if len(wallets) == 0 {
		f.ui.ShowInfo("暂无钱包")
		return nil
	}

	// 2. 选择钱包
	options := make([]string, len(wallets))
	for i, wallet := range wallets {
		options[i] = fmt.Sprintf("%s (%s)", wallet.Name, format.FormatAddress(wallet.Address, 10, 8))
	}

	selectedIndex, err := f.ui.ShowMenu("选择钱包", options)
	if err != nil {
		return fmt.Errorf("选择钱包失败: %w", err)
	}

	selectedWallet := wallets[selectedIndex]

	// 3. 输入旧密码
	oldPassword, err := f.ui.ShowInputDialog("验证身份", "请输入当前密码", true)
	if err != nil {
		return fmt.Errorf("输入旧密码失败: %w", err)
	}

	// 4. 输入新密码
	newPassword, err := f.ui.ShowInputDialog("新密码", "请输入新密码（至少8位）", true)
	if err != nil {
		return fmt.Errorf("输入新密码失败: %w", err)
	}

	if len(newPassword) < 8 {
		f.ui.ShowError("密码长度不能少于8位")
		return fmt.Errorf("密码强度不足")
	}

	// 5. 确认新密码
	confirmPassword, err := f.ui.ShowInputDialog("确认密码", "请再次输入新密码", true)
	if err != nil {
		return fmt.Errorf("确认密码失败: %w", err)
	}

	if newPassword != confirmPassword {
		f.ui.ShowError("两次输入的密码不一致")
		return fmt.Errorf("密码不一致")
	}

	// 6. 修改密码
	spinner := f.ui.ShowSpinner("正在修改密码...")
	spinner.Start()

	err = f.walletService.ChangePassword(ctx, selectedWallet.Name, oldPassword, newPassword)
	spinner.Stop()

	if err != nil {
		f.ui.ShowError(fmt.Sprintf("修改密码失败: %v", err))
		return fmt.Errorf("修改密码失败: %w", err)
	}

	f.ui.ShowSuccess("密码修改成功！")

	return nil
}

// ============================================================================
// 辅助函数
// ============================================================================

// convertToFloat 将格式化后的余额字符串转换为float64（用于UI展示）
func convertToFloat(balanceStr string) float64 {
	var balance float64
	fmt.Sscanf(balanceStr, "%f", &balance)
	return balance
}
