// Package flows 提供可复用的交互流程
package flows

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/weisyn/v1/client/pkg/tools/format"
	"github.com/weisyn/v1/client/pkg/ux/ui"
	"github.com/weisyn/v1/pkg/utils"
)

// TransferFlow 转账交互流程
//
// 功能：
//   - 提供转账相关的完整UI交互流程
//   - 支持单笔转账、批量转账、时间锁转账
//   - 支持手续费估算
//
// 依赖：
//   - ui.Components: UI组件接口
//   - TransferService: 转账服务端口
//   - WalletService: 钱包服务端口（用于选择钱包和获取私钥）
//   - AddressValidator: 地址验证器端口
type TransferFlow struct {
	ui               ui.Components
	transferService  TransferService
	walletService    WalletService
	addressValidator AddressValidator
}

// NewTransferFlow 创建转账流程实例
func NewTransferFlow(
	uiComponents ui.Components,
	transferService TransferService,
	walletService WalletService,
	addressValidator AddressValidator,
) *TransferFlow {
	return &TransferFlow{
		ui:               uiComponents,
		transferService:  transferService,
		walletService:    walletService,
		addressValidator: addressValidator,
	}
}

// ============================================================================
// 单笔转账流程
// ============================================================================

// ExecuteTransfer 执行单笔转账（交互式）
//
// 功能：
//   - 选择发送方钱包
//   - 输入接收方地址和转账金额
//   - 验证输入
//   - 估算手续费
//   - 确认并执行转账
func (f *TransferFlow) ExecuteTransfer(ctx context.Context) (*TransferResult, error) {
	f.ui.ShowHeader("单笔转账")

	// 1. 选择发送方钱包
	fromWallet, privateKey, err := f.selectWalletWithPassword(ctx, "选择发送方钱包")
	if err != nil {
		return nil, err
	}

	// 2. 输入接收方地址
	toAddress, err := f.ui.ShowInputDialog("接收方地址", "请输入接收方地址", false)
	if err != nil {
		return nil, fmt.Errorf("输入接收方地址失败: %w", err)
	}

	// 3. 验证接收方地址
	valid, err := f.addressValidator.ValidateAddress(toAddress)
	if !valid || err != nil {
		f.ui.ShowError(fmt.Sprintf("接收方地址无效: %v", err))
		return nil, fmt.Errorf("接收方地址无效: %w", err)
	}

	// 4. 输入转账金额
	amountStr, err := f.ui.ShowInputDialog("转账金额", "请输入转账金额（WES）", false)
	if err != nil {
		return nil, fmt.Errorf("输入转账金额失败: %w", err)
	}

	// 解析金额
	amount, err := parseAmount(amountStr)
	if err != nil {
		f.ui.ShowError(fmt.Sprintf("金额格式无效: %v", err))
		return nil, fmt.Errorf("金额格式无效: %w", err)
	}

	// 5. 估算手续费
	spinner := f.ui.ShowSpinner("正在估算手续费...")
	spinner.Start()
	estimatedFee, err := f.transferService.EstimateFee(ctx, fromWallet.Address, toAddress, amount)
	spinner.Stop()

	if err != nil {
		f.ui.ShowWarning(fmt.Sprintf("无法估算手续费: %v", err))
		estimatedFee = 0 // 继续，但不显示手续费
	}

	// 6. 展示转账信息并确认
	confirmMsg := fmt.Sprintf(
		"发送方: %s (%s)\n接收方: %s\n转账金额: %s WES\n估算手续费: %s WES\n\n确认执行转账？",
		fromWallet.Name,
		format.FormatAddress(fromWallet.Address, 10, 8),
		format.FormatAddress(toAddress, 10, 8),
		utils.FormatWeiToDecimal(amount),
		utils.FormatWeiToDecimal(estimatedFee),
	)

	confirm, err := f.ui.ShowConfirmDialog("确认转账", confirmMsg)
	if err != nil || !confirm {
		f.ui.ShowInfo("已取消转账")
		return nil, fmt.Errorf("用户取消转账")
	}

	// 7. 执行转账
	spinner = f.ui.ShowSpinner("正在执行转账...")
	spinner.Start()

	req := &TransferRequest{
		FromAddress: fromWallet.Address,
		ToAddress:   toAddress,
		Amount:      amount,
		PrivateKey:  privateKey,
	}

	txHash, err := f.transferService.Transfer(ctx, req)
	spinner.Stop()

	if err != nil {
		f.ui.ShowError(fmt.Sprintf("转账失败: %v", err))
		return nil, fmt.Errorf("转账失败: %w", err)
	}

	// 8. 展示结果
	f.ui.ShowSuccess("转账成功！")
	f.ui.ShowPanel("交易信息", fmt.Sprintf(
		"交易哈希: %s\n发送方: %s\n接收方: %s\n金额: %s WES\n状态: 已提交，等待确认",
		format.FormatHashShort([]byte(txHash), 10, 10),
		format.FormatAddress(fromWallet.Address, 10, 8),
		format.FormatAddress(toAddress, 10, 8),
		utils.FormatWeiToDecimal(amount),
	))

	return &TransferResult{
		TxHash:      txHash,
		Success:     true,
		Message:     "转账交易已提交",
		BlockHeight: 0,
	}, nil
}

// ============================================================================
// 批量转账流程
// ============================================================================

// ExecuteBatchTransfer 执行批量转账（交互式）
//
// 功能：
//   - 选择发送方钱包
//   - 输入多个接收方地址和金额
//   - 验证输入
//   - 确认并执行批量转账
func (f *TransferFlow) ExecuteBatchTransfer(ctx context.Context) (*TransferResult, error) {
	f.ui.ShowHeader("批量转账")

	// 1. 选择发送方钱包
	fromWallet, privateKey, err := f.selectWalletWithPassword(ctx, "选择发送方钱包")
	if err != nil {
		return nil, err
	}

	// 2. 输入转账项数量
	countStr, err := f.ui.ShowInputDialog("转账数量", "请输入转账项数量", false)
	if err != nil {
		return nil, fmt.Errorf("输入转账数量失败: %w", err)
	}

	count, err := strconv.Atoi(countStr)
	if err != nil || count <= 0 || count > 100 {
		f.ui.ShowError("转账数量无效（范围：1-100）")
		return nil, fmt.Errorf("转账数量无效")
	}

	// 3. 逐项输入转账信息
	transfers := make([]TransferItem, 0, count)
	totalAmount := uint64(0)

	for i := 0; i < count; i++ {
		f.ui.ShowSection(fmt.Sprintf("转账项 %d/%d", i+1, count))

		// 输入接收方地址
		toAddress, err := f.ui.ShowInputDialog("接收方地址", fmt.Sprintf("请输入第%d个接收方地址", i+1), false)
		if err != nil {
			return nil, fmt.Errorf("输入接收方地址失败: %w", err)
		}

		// 验证地址
		valid, err := f.addressValidator.ValidateAddress(toAddress)
		if !valid || err != nil {
			f.ui.ShowError(fmt.Sprintf("地址无效: %v", err))
			return nil, fmt.Errorf("地址无效: %w", err)
		}

		// 输入转账金额
		amountStr, err := f.ui.ShowInputDialog("转账金额", fmt.Sprintf("请输入第%d个转账金额（WES）", i+1), false)
		if err != nil {
			return nil, fmt.Errorf("输入转账金额失败: %w", err)
		}

		amount, err := parseAmount(amountStr)
		if err != nil {
			f.ui.ShowError(fmt.Sprintf("金额格式无效: %v", err))
			return nil, fmt.Errorf("金额格式无效: %w", err)
		}

		transfers = append(transfers, TransferItem{
			ToAddress: toAddress,
			Amount:    amount,
		})
		totalAmount += amount
	}

	// 4. 展示批量转账清单
	data := [][]string{{"序号", "接收方地址", "金额（WES）"}}
	for i, transfer := range transfers {
		data = append(data, []string{
			fmt.Sprintf("%d", i+1),
			format.FormatAddress(transfer.ToAddress, 10, 8),
			utils.FormatWeiToDecimal(transfer.Amount),
		})
	}
	data = append(data, []string{"合计", "", utils.FormatWeiToDecimal(totalAmount)})

	f.ui.ShowTable("批量转账清单", data)

	// 5. 确认
	confirmMsg := fmt.Sprintf(
		"发送方: %s (%s)\n转账项数: %d\n总金额: %s WES\n\n确认执行批量转账？",
		fromWallet.Name,
		format.FormatAddress(fromWallet.Address, 10, 8),
		len(transfers),
		utils.FormatWeiToDecimal(totalAmount),
	)

	confirm, err := f.ui.ShowConfirmDialog("确认批量转账", confirmMsg)
	if err != nil || !confirm {
		f.ui.ShowInfo("已取消批量转账")
		return nil, fmt.Errorf("用户取消批量转账")
	}

	// 6. 执行批量转账
	spinner := f.ui.ShowSpinner("正在执行批量转账...")
	spinner.Start()

	req := &BatchTransferRequest{
		FromAddress: fromWallet.Address,
		Transfers:   transfers,
		PrivateKey:  privateKey,
	}

	txHash, err := f.transferService.BatchTransfer(ctx, req)
	spinner.Stop()

	if err != nil {
		f.ui.ShowError(fmt.Sprintf("批量转账失败: %v", err))
		return nil, fmt.Errorf("批量转账失败: %w", err)
	}

	// 7. 展示结果
	f.ui.ShowSuccess("批量转账成功！")
	f.ui.ShowPanel("交易信息", fmt.Sprintf(
		"交易哈希: %s\n发送方: %s\n转账项数: %d\n总金额: %s WES\n状态: 已提交，等待确认",
		format.FormatHashShort([]byte(txHash), 10, 10),
		format.FormatAddress(fromWallet.Address, 10, 8),
		len(transfers),
		utils.FormatWeiToDecimal(totalAmount),
	))

	return &TransferResult{
		TxHash:      txHash,
		Success:     true,
		Message:     "批量转账交易已提交",
		BlockHeight: 0,
	}, nil
}

// ============================================================================
// 时间锁转账流程
// ============================================================================

// ExecuteTimeLockTransfer 执行时间锁转账（交互式）
//
// 功能：
//   - 选择发送方钱包
//   - 输入接收方地址、金额和锁定时间
//   - 验证输入
//   - 确认并执行时间锁转账
func (f *TransferFlow) ExecuteTimeLockTransfer(ctx context.Context) (*TransferResult, error) {
	f.ui.ShowHeader("时间锁转账")

	// 1. 选择发送方钱包
	fromWallet, privateKey, err := f.selectWalletWithPassword(ctx, "选择发送方钱包")
	if err != nil {
		return nil, err
	}

	// 2. 输入接收方地址
	toAddress, err := f.ui.ShowInputDialog("接收方地址", "请输入接收方地址", false)
	if err != nil {
		return nil, fmt.Errorf("输入接收方地址失败: %w", err)
	}

	// 3. 验证接收方地址
	valid, err := f.addressValidator.ValidateAddress(toAddress)
	if !valid || err != nil {
		f.ui.ShowError(fmt.Sprintf("接收方地址无效: %v", err))
		return nil, fmt.Errorf("接收方地址无效: %w", err)
	}

	// 4. 输入转账金额
	amountStr, err := f.ui.ShowInputDialog("转账金额", "请输入转账金额（WES）", false)
	if err != nil {
		return nil, fmt.Errorf("输入转账金额失败: %w", err)
	}

	amount, err := parseAmount(amountStr)
	if err != nil {
		f.ui.ShowError(fmt.Sprintf("金额格式无效: %v", err))
		return nil, fmt.Errorf("金额格式无效: %w", err)
	}

	// 5. 输入锁定时间（小时数）
	f.ui.ShowInfo("锁定时间说明：资金将在指定时间后才能被接收方使用")
	lockHoursStr, err := f.ui.ShowInputDialog("锁定时间", "请输入锁定时长（小时）", false)
	if err != nil {
		return nil, fmt.Errorf("输入锁定时间失败: %w", err)
	}

	lockHours, err := strconv.Atoi(lockHoursStr)
	if err != nil || lockHours <= 0 {
		f.ui.ShowError("锁定时长无效")
		return nil, fmt.Errorf("锁定时长无效")
	}

	// 计算解锁时间（Unix时间戳）
	lockTime := uint64(time.Now().Add(time.Duration(lockHours) * time.Hour).Unix())
	unlockTimeStr := time.Unix(int64(lockTime), 0).Format("2006-01-02 15:04:05")

	// 6. 展示时间锁转账信息并确认
	confirmMsg := fmt.Sprintf(
		"发送方: %s (%s)\n接收方: %s\n转账金额: %s WES\n锁定时长: %d 小时\n解锁时间: %s\n\n⚠️ 时间锁转账在解锁时间前无法使用！\n确认执行时间锁转账？",
		fromWallet.Name,
		format.FormatAddress(fromWallet.Address, 10, 8),
		format.FormatAddress(toAddress, 10, 8),
		utils.FormatWeiToDecimal(amount),
		lockHours,
		unlockTimeStr,
	)

	confirm, err := f.ui.ShowConfirmDialog("确认时间锁转账", confirmMsg)
	if err != nil || !confirm {
		f.ui.ShowInfo("已取消时间锁转账")
		return nil, fmt.Errorf("用户取消时间锁转账")
	}

	// 7. 执行时间锁转账
	spinner := f.ui.ShowSpinner("正在执行时间锁转账...")
	spinner.Start()

	req := &TimeLockTransferRequest{
		FromAddress: fromWallet.Address,
		ToAddress:   toAddress,
		Amount:      amount,
		LockTime:    lockTime,
		PrivateKey:  privateKey,
	}

	txHash, err := f.transferService.TimeLockTransfer(ctx, req)
	spinner.Stop()

	if err != nil {
		f.ui.ShowError(fmt.Sprintf("时间锁转账失败: %v", err))
		return nil, fmt.Errorf("时间锁转账失败: %w", err)
	}

	// 8. 展示结果
	f.ui.ShowSuccess("时间锁转账成功！")
	f.ui.ShowPanel("交易信息", fmt.Sprintf(
		"交易哈希: %s\n发送方: %s\n接收方: %s\n金额: %s WES\n解锁时间: %s\n状态: 已提交，等待确认",
		format.FormatHashShort([]byte(txHash), 10, 10),
		format.FormatAddress(fromWallet.Address, 10, 8),
		format.FormatAddress(toAddress, 10, 8),
		utils.FormatWeiToDecimal(amount),
		unlockTimeStr,
	))

	return &TransferResult{
		TxHash:      txHash,
		Success:     true,
		Message:     fmt.Sprintf("时间锁转账交易已提交（锁定至 %s）", unlockTimeStr),
		BlockHeight: 0,
	}, nil
}

// ============================================================================
// 手续费估算流程
// ============================================================================

// EstimateFee 估算转账手续费（交互式）
//
// 功能：
//   - 输入发送方地址、接收方地址和金额
//   - 估算手续费并展示
func (f *TransferFlow) EstimateFee(ctx context.Context) (*FeeEstimate, error) {
	f.ui.ShowHeader("估算转账手续费")

	// 1. 输入发送方地址
	fromAddress, err := f.ui.ShowInputDialog("发送方地址", "请输入发送方地址", false)
	if err != nil {
		return nil, fmt.Errorf("输入发送方地址失败: %w", err)
	}

	// 2. 验证发送方地址
	valid, err := f.addressValidator.ValidateAddress(fromAddress)
	if !valid || err != nil {
		f.ui.ShowError(fmt.Sprintf("发送方地址无效: %v", err))
		return nil, fmt.Errorf("发送方地址无效: %w", err)
	}

	// 3. 输入接收方地址
	toAddress, err := f.ui.ShowInputDialog("接收方地址", "请输入接收方地址", false)
	if err != nil {
		return nil, fmt.Errorf("输入接收方地址失败: %w", err)
	}

	// 4. 验证接收方地址
	valid, err = f.addressValidator.ValidateAddress(toAddress)
	if !valid || err != nil {
		f.ui.ShowError(fmt.Sprintf("接收方地址无效: %v", err))
		return nil, fmt.Errorf("接收方地址无效: %w", err)
	}

	// 5. 输入转账金额
	amountStr, err := f.ui.ShowInputDialog("转账金额", "请输入转账金额（WES）", false)
	if err != nil {
		return nil, fmt.Errorf("输入转账金额失败: %w", err)
	}

	amount, err := parseAmount(amountStr)
	if err != nil {
		f.ui.ShowError(fmt.Sprintf("金额格式无效: %v", err))
		return nil, fmt.Errorf("金额格式无效: %w", err)
	}

	// 6. 估算手续费
	spinner := f.ui.ShowSpinner("正在估算手续费...")
	spinner.Start()

	estimatedFee, err := f.transferService.EstimateFee(ctx, fromAddress, toAddress, amount)
	spinner.Stop()

	if err != nil {
		f.ui.ShowError(fmt.Sprintf("估算手续费失败: %v", err))
		return nil, fmt.Errorf("估算手续费失败: %w", err)
	}

	// 7. 展示估算结果
	f.ui.ShowPanel("手续费估算结果", fmt.Sprintf(
		"发送方: %s\n接收方: %s\n转账金额: %s WES\n估算手续费: %s WES\n\n说明：实际手续费可能因网络状况而略有不同",
		format.FormatAddress(fromAddress, 10, 8),
		format.FormatAddress(toAddress, 10, 8),
		utils.FormatWeiToDecimal(amount),
		utils.FormatWeiToDecimal(estimatedFee),
	))

	return &FeeEstimate{
		EstimatedFee: estimatedFee,
		Unit:         "WES",
		Message:      "手续费估算完成",
	}, nil
}

// ============================================================================
// 辅助函数
// ============================================================================

// selectWalletWithPassword 选择钱包并验证密码
//
// 功能：
//   - 列出所有钱包供用户选择
//   - 提示输入密码
//   - 验证密码并获取私钥
//
// 返回：
//   - WalletInfo: 选中的钱包信息
//   - []byte: 钱包私钥
//   - error: 错误信息
func (f *TransferFlow) selectWalletWithPassword(ctx context.Context, title string) (*WalletInfo, []byte, error) {
	// 1. 查询钱包列表
	wallets, err := f.walletService.ListWallets(ctx)
	if err != nil {
		f.ui.ShowError(fmt.Sprintf("查询钱包列表失败: %v", err))
		return nil, nil, fmt.Errorf("查询钱包列表失败: %w", err)
	}

	if len(wallets) == 0 {
		f.ui.ShowInfo("暂无钱包，请先创建钱包")
		return nil, nil, fmt.Errorf("暂无钱包")
	}

	// 2. 选择钱包
	options := make([]string, len(wallets))
	for i, wallet := range wallets {
		defaultMark := ""
		if wallet.IsDefault {
			defaultMark = " [默认]"
		}
		options[i] = fmt.Sprintf("%s (%s)%s", wallet.Name, format.FormatAddress(wallet.Address, 10, 8), defaultMark)
	}

	selectedIndex, err := f.ui.ShowMenu(title, options)
	if err != nil {
		return nil, nil, fmt.Errorf("选择钱包失败: %w", err)
	}

	selectedWallet := wallets[selectedIndex]

	// 3. 输入密码
	password, err := f.ui.ShowInputDialog("密码验证", "请输入钱包密码", true)
	if err != nil {
		return nil, nil, fmt.Errorf("输入密码失败: %w", err)
	}

	// 4. 获取私钥
	privateKeyHex, err := f.walletService.ExportPrivateKey(ctx, selectedWallet.Name, password)
	if err != nil {
		f.ui.ShowError(fmt.Sprintf("密码错误或获取私钥失败: %v", err))
		return nil, nil, fmt.Errorf("获取私钥失败: %w", err)
	}

	// 5. 转换私钥为字节数组
	privateKey, err := format.ParseContentHash(privateKeyHex) // 复用ParseContentHash解析十六进制
	if err != nil {
		return nil, nil, fmt.Errorf("私钥格式无效: %w", err)
	}

	return &selectedWallet, privateKey, nil
}

// parseAmount 解析金额字符串为最小单位（Wei）
//
// 功能：
//   - 支持小数输入（如 "1.5"）
//   - 转换为最小单位（1 WES = 10^18 Wei）
//
// 参数：
//   - amountStr: 金额字符串（WES）
//
// 返回：
//   - uint64: 金额（Wei）
//   - error: 错误信息
func parseAmount(amountStr string) (uint64, error) {
	// 使用 utils.ParseDecimalToWei 解析金额
	amount, err := utils.ParseDecimalToWei(amountStr)
	if err != nil {
		return 0, fmt.Errorf("金额格式无效: %w", err)
	}

	if amount == 0 {
		return 0, fmt.Errorf("金额必须大于0")
	}

	return amount, nil
}

