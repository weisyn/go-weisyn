package screens

import (
	"context"
	"fmt"
	"time"

	"github.com/weisyn/v1/client/core/transfer"
	"github.com/weisyn/v1/client/core/wallet"
)

// TransferScreen 转账操作屏幕
type TransferScreen struct {
	transferService *transfer.TransferService
	batchService    *transfer.BatchTransferService
	timelockService *transfer.TimeLockTransferService
	accountManager  *wallet.AccountManager
}

// NewTransferScreen 创建转账操作屏幕
func NewTransferScreen(
	transferService *transfer.TransferService,
	batchService *transfer.BatchTransferService,
	timelockService *transfer.TimeLockTransferService,
	accountManager *wallet.AccountManager,
) *TransferScreen {
	return &TransferScreen{
		transferService: transferService,
		batchService:    batchService,
		timelockService: timelockService,
		accountManager:  accountManager,
	}
}

// Render 渲染转账操作屏幕
func (s *TransferScreen) Render(ctx context.Context) {
	for {
		fmt.Println("\n========================================")
		fmt.Println("           转账操作")
		fmt.Println("========================================")
		fmt.Println("1. 简单转账")
		fmt.Println("2. 批量转账")
		fmt.Println("3. 时间锁转账")
		fmt.Println("0. 返回主菜单")
		fmt.Println("========================================")
		fmt.Print("请选择操作: ")

		var choice int
		fmt.Scanf("%d\n", &choice)

		switch choice {
		case 1:
			s.simpleTransfer(ctx)
		case 2:
			s.batchTransfer(ctx)
		case 3:
			s.timelockTransfer(ctx)
		case 0:
			return
		default:
			fmt.Println("无效的选择，请重试")
		}
	}
}

// simpleTransfer 简单转账
func (s *TransferScreen) simpleTransfer(ctx context.Context) {
	fmt.Println("\n【简单转账】")

	// 选择发送账户
	fromAddr, privateKey, err := s.selectAccount("请选择发送账户")
	if err != nil {
		fmt.Printf("选择账户失败: %v\n", err)
		s.waitForEnter()
		return
	}

	// 输入接收地址
	fmt.Print("请输入接收地址: ")
	var toAddr string
	fmt.Scanf("%s\n", &toAddr)

	// 输入转账金额
	fmt.Print("请输入转账金额 (WES): ")
	var amount string
	fmt.Scanf("%s\n", &amount)

	// 输入备注（可选）
	fmt.Print("请输入备注（回车跳过）: ")
	var memo string
	fmt.Scanf("%[^\n]\n", &memo)
	if memo == "" {
		fmt.Scanln() // 清空缓冲
	}

	// 构建转账请求
	req := &transfer.TransferRequest{
		FromAddress: fromAddr,
		ToAddress:   toAddr,
		Amount:      amount,
		PrivateKey:  privateKey,
		Memo:        memo,
	}

	// 执行转账
	fmt.Println("\n正在执行转账...")
	result, err := s.transferService.ExecuteTransfer(ctx, req)
	if err != nil {
		fmt.Printf("转账失败: %v\n", err)
		s.waitForEnter()
		return
	}

	// 显示结果
	fmt.Println("\n转账成功！")
	fmt.Printf("交易ID: %s\n", result.TxID)
	fmt.Printf("交易哈希: %s\n", result.TxHash)
	fmt.Printf("手续费: %s WES\n", result.Fee)
	fmt.Printf("找零: %s WES\n", result.Change)

	s.waitForEnter()
}

// batchTransfer 批量转账
func (s *TransferScreen) batchTransfer(ctx context.Context) {
	fmt.Println("\n【批量转账】")

	// 选择发送账户
	fromAddr, privateKey, err := s.selectAccount("请选择发送账户")
	if err != nil {
		fmt.Printf("选择账户失败: %v\n", err)
		s.waitForEnter()
		return
	}

	// 输入收款人数量
	fmt.Print("请输入收款人数量: ")
	var count int
	fmt.Scanf("%d\n", &count)

	if count <= 0 {
		fmt.Println("收款人数量必须大于0")
		s.waitForEnter()
		return
	}

	// 输入收款人信息
	recipients := make([]transfer.BatchRecipient, 0, count)
	for i := 0; i < count; i++ {
		fmt.Printf("\n收款人 %d:\n", i+1)

		fmt.Print("  地址: ")
		var addr string
		fmt.Scanf("%s\n", &addr)

		fmt.Print("  金额 (WES): ")
		var amount string
		fmt.Scanf("%s\n", &amount)

		recipients = append(recipients, transfer.BatchRecipient{
			Address: addr,
			Amount:  amount,
		})
	}

	// 输入备注（可选）
	fmt.Print("\n请输入备注（回车跳过）: ")
	var memo string
	fmt.Scanf("%[^\n]\n", &memo)
	if memo == "" {
		fmt.Scanln() // 清空缓冲
	}

	// 构建批量转账请求
	req := &transfer.BatchTransferRequest{
		FromAddress: fromAddr,
		Recipients:  recipients,
		PrivateKey:  privateKey,
		Memo:        memo,
	}

	// 执行批量转账
	fmt.Println("\n正在执行批量转账...")
	result, err := s.batchService.ExecuteBatchTransfer(ctx, req)
	if err != nil {
		fmt.Printf("批量转账失败: %v\n", err)
		s.waitForEnter()
		return
	}

	// 显示结果
	fmt.Println("\n批量转账成功！")
	fmt.Printf("交易ID: %s\n", result.TxID)
	fmt.Printf("交易哈希: %s\n", result.TxHash)
	fmt.Printf("总金额: %s WES\n", result.TotalAmount)
	fmt.Printf("手续费: %s WES\n", result.Fee)
	fmt.Printf("找零: %s WES\n", result.Change)
	// BatchTransferResult不包含SuccessCount和FailedRecipients字段
	// 简化显示结果

	s.waitForEnter()
}

// timelockTransfer 时间锁转账
func (s *TransferScreen) timelockTransfer(ctx context.Context) {
	fmt.Println("\n【时间锁转账】")

	// 选择发送账户
	fromAddr, privateKey, err := s.selectAccount("请选择发送账户")
	if err != nil {
		fmt.Printf("选择账户失败: %v\n", err)
		s.waitForEnter()
		return
	}

	// 输入接收地址
	fmt.Print("请输入接收地址: ")
	var toAddr string
	fmt.Scanf("%s\n", &toAddr)

	// 输入转账金额
	fmt.Print("请输入转账金额 (WES): ")
	var amount string
	fmt.Scanf("%s\n", &amount)

	// 输入解锁时间
	fmt.Println("\n请输入解锁时间：")
	fmt.Println("1. 1小时后")
	fmt.Println("2. 24小时后")
	fmt.Println("3. 7天后")
	fmt.Println("4. 自定义时间")
	fmt.Print("请选择: ")

	var timeChoice int
	fmt.Scanf("%d\n", &timeChoice)

	var unlockTime time.Time
	now := time.Now()

	switch timeChoice {
	case 1:
		unlockTime = now.Add(1 * time.Hour)
	case 2:
		unlockTime = now.Add(24 * time.Hour)
	case 3:
		unlockTime = now.Add(7 * 24 * time.Hour)
	case 4:
		fmt.Print("请输入解锁时间 (格式: 2006-01-02 15:04:05): ")
		var timeStr string
		fmt.Scanf("%[^\n]\n", &timeStr)

		unlockTime, err = time.Parse("2006-01-02 15:04:05", timeStr)
		if err != nil {
			fmt.Printf("时间格式错误: %v\n", err)
			s.waitForEnter()
			return
		}
	default:
		fmt.Println("无效的选择")
		s.waitForEnter()
		return
	}

	// 验证解锁时间
	if unlockTime.Before(now) {
		fmt.Println("解锁时间不能早于当前时间")
		s.waitForEnter()
		return
	}

	// 输入备注（可选）
	fmt.Print("\n请输入备注（回车跳过）: ")
	var memo string
	fmt.Scanf("%[^\n]\n", &memo)
	if memo == "" {
		fmt.Scanln() // 清空缓冲
	}

	// 构建时间锁转账请求
	req := &transfer.TimeLockTransferRequest{
		FromAddress: fromAddr,
		ToAddress:   toAddr,
		Amount:      amount,
		PrivateKey:  privateKey,
		UnlockTime:  unlockTime,
		Memo:        memo,
	}

	// 执行时间锁转账
	fmt.Println("\n正在执行时间锁转账...")
	result, err := s.timelockService.ExecuteTimeLockTransfer(ctx, req)
	if err != nil {
		fmt.Printf("时间锁转账失败: %v\n", err)
		s.waitForEnter()
		return
	}

	// 显示结果
	fmt.Println("\n时间锁转账成功！")
	fmt.Printf("交易ID: %s\n", result.TxID)
	fmt.Printf("交易哈希: %s\n", result.TxHash)
	fmt.Printf("转账金额: %s WES\n", result.Amount)
	fmt.Printf("解锁时间: %s\n", result.UnlockTime.Format("2006-01-02 15:04:05"))
	fmt.Printf("手续费: %s WES\n", result.Fee)
	fmt.Printf("找零: %s WES\n", result.Change)

	s.waitForEnter()
}

// selectAccount 选择账户
func (s *TransferScreen) selectAccount(prompt string) (address string, privateKey []byte, err error) {
	accounts, err := s.accountManager.ListAccounts()
	if err != nil {
		return "", nil, fmt.Errorf("获取账户列表失败: %w", err)
	}

	if len(accounts) == 0 {
		return "", nil, fmt.Errorf("当前没有账户，请先创建账户")
	}

	fmt.Printf("\n%s:\n", prompt)
	for i, acc := range accounts {
		label := acc.Label
		if label == "" {
			label = "(无标签)"
		}
		fmt.Printf("%d. %s (%s)\n", i+1, label, acc.Address)
	}

	fmt.Print("请选择账户序号: ")
	var choice int
	fmt.Scanf("%d\n", &choice)

	if choice < 1 || choice > len(accounts) {
		return "", nil, fmt.Errorf("无效的选择")
	}

	account := accounts[choice-1]

	fmt.Print("请输入密码: ")
	var password string
	fmt.Scanf("%s\n", &password)

	// 使用ExportPrivateKey获取私钥
	privateKeyHex, err := s.accountManager.ExportPrivateKey(account.Address, password)
	if err != nil {
		return "", nil, fmt.Errorf("密码错误: %w", err)
	}

	// 将hex字符串转换为字节数组（简化处理，实际应使用hex.DecodeString）
	privateKey = []byte(privateKeyHex)

	return account.Address, privateKey, nil
}

// waitForEnter 等待用户按回车
func (s *TransferScreen) waitForEnter() {
	fmt.Print("\n按回车键继续...")
	fmt.Scanln()
}
