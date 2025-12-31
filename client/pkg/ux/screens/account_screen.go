package screens

import (
	"context"
	"fmt"
	"strconv"

	"github.com/weisyn/v1/client/core/transport"
	"github.com/weisyn/v1/client/core/wallet"
	"github.com/weisyn/v1/pkg/utils"
)

// AccountScreen 账户管理屏幕
type AccountScreen struct {
	accountManager *wallet.AccountManager
	transport      transport.Client
}

// NewAccountScreen 创建账户管理屏幕
func NewAccountScreen(
	accountManager *wallet.AccountManager,
	transport transport.Client,
) *AccountScreen {
	return &AccountScreen{
		accountManager: accountManager,
		transport:      transport,
	}
}

// Render 渲染账户管理屏幕
func (s *AccountScreen) Render(ctx context.Context) {
	for {
		fmt.Println("\n========================================")
		fmt.Println("           账户管理")
		fmt.Println("========================================")
		fmt.Println("1. 创建账户")
		fmt.Println("2. 导出私钥")
		fmt.Println("3. 查看账户列表")
		fmt.Println("4. 查看账户余额")
		fmt.Println("0. 返回主菜单")
		fmt.Println("========================================")
		fmt.Print("请选择操作: ")

		var choice int
		fmt.Scanf("%d\n", &choice)

		switch choice {
		case 1:
			s.createAccount(ctx)
		case 2:
			s.exportPrivateKey(ctx)
		case 3:
			s.listAccounts(ctx)
		case 4:
			s.viewBalance(ctx)
		case 0:
			return
		default:
			fmt.Println("无效的选择，请重试")
		}
	}
}

// createAccount 创建账户
func (s *AccountScreen) createAccount(ctx context.Context) {
	fmt.Println("\n【创建账户】")

	fmt.Print("请输入账户标签: ")
	var label string
	fmt.Scanf("%s\n", &label)

	fmt.Print("请输入密码: ")
	var password string
	fmt.Scanf("%s\n", &password)

	// ✅ 修复参数顺序：CreateAccount(password, label)
	account, err := s.accountManager.CreateAccount(password, label)
	if err != nil {
		fmt.Printf("创建账户失败: %v\n", err)
		s.waitForEnter()
		return
	}

	fmt.Printf("账户创建成功！\n")
	fmt.Printf("地址: %s\n", account.Address)
	fmt.Printf("标签: %s\n", account.Label)
	s.waitForEnter()
}

// exportPrivateKey 导出私钥
func (s *AccountScreen) exportPrivateKey(ctx context.Context) {
	fmt.Println("\n【导出私钥】")

	accounts, err := s.accountManager.ListAccounts()
	if err != nil {
		fmt.Printf("获取账户列表失败: %v\n", err)
		s.waitForEnter()
		return
	}

	if len(accounts) == 0 {
		fmt.Println("当前没有账户")
		s.waitForEnter()
		return
	}

	fmt.Println("账户列表：")
	for i, acc := range accounts {
		label := acc.Label
		if label == "" {
			label = "(无标签)"
		}
		fmt.Printf("%d. %s (%s)\n", i+1, label, acc.Address)
	}

	fmt.Print("请选择要导出的账户序号: ")
	var choice int
	fmt.Scanf("%d\n", &choice)

	if choice < 1 || choice > len(accounts) {
		fmt.Println("无效的选择")
		s.waitForEnter()
		return
	}

	account := accounts[choice-1]

	fmt.Print("请输入密码: ")
	var password string
	fmt.Scanf("%s\n", &password)

	privateKey, err := s.accountManager.ExportPrivateKey(account.Address, password)
	if err != nil {
		fmt.Printf("导出私钥失败: %v\n", err)
	} else {
		fmt.Printf("私钥: %s\n", privateKey)
		fmt.Println("⚠️  请妥善保管私钥，切勿泄露！")
	}

	s.waitForEnter()
}

// listAccounts 列出所有账户
func (s *AccountScreen) listAccounts(ctx context.Context) {
	fmt.Println("\n【账户列表】")

	accounts, err := s.accountManager.ListAccounts()
	if err != nil {
		fmt.Printf("获取账户列表失败: %v\n", err)
		s.waitForEnter()
		return
	}

	if len(accounts) == 0 {
		fmt.Println("当前没有账户")
		s.waitForEnter()
		return
	}

	fmt.Printf("共有 %d 个账户：\n\n", len(accounts))
	for i, acc := range accounts {
		label := acc.Label
		if label == "" {
			label = "(无标签)"
		}
		fmt.Printf("%d. 标签: %s\n", i+1, label)
		fmt.Printf("   地址: %s\n", acc.Address)
		fmt.Printf("   创建时间: %s\n", acc.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Println()
	}

	s.waitForEnter()
}

// viewBalance 查看账户余额
func (s *AccountScreen) viewBalance(ctx context.Context) {
	fmt.Println("\n【查看余额】")

	accounts, err := s.accountManager.ListAccounts()
	if err != nil {
		fmt.Printf("获取账户列表失败: %v\n", err)
		s.waitForEnter()
		return
	}

	if len(accounts) == 0 {
		fmt.Println("当前没有账户")
		s.waitForEnter()
		return
	}

	fmt.Println("账户列表：")
	for i, acc := range accounts {
		label := acc.Label
		if label == "" {
			label = "(无标签)"
		}
		fmt.Printf("%d. %s (%s)\n", i+1, label, acc.Address)
	}

	fmt.Print("请选择要查看余额的账户序号（0查看全部）: ")
	var choice int
	fmt.Scanf("%d\n", &choice)

	if choice == 0 {
		// 查看所有账户余额
		for _, acc := range accounts {
			label := acc.Label
			if label == "" {
				label = "(无标签)"
			}
			s.displayAccountBalance(ctx, acc.Address, label)
		}
	} else if choice >= 1 && choice <= len(accounts) {
		// 查看指定账户余额
		account := accounts[choice-1]
		label := account.Label
		if label == "" {
			label = "(无标签)"
		}
		s.displayAccountBalance(ctx, account.Address, label)
	} else {
		fmt.Println("无效的选择")
	}

	s.waitForEnter()
}

// displayAccountBalance 显示账户余额
func (s *AccountScreen) displayAccountBalance(ctx context.Context, address, label string) {
	fmt.Printf("\n【%s】(%s)\n", label, address)

	// 查询UTXO
	utxos, err := s.transport.GetUTXOs(ctx, address, nil)
	if err != nil {
		fmt.Printf("查询余额失败: %v\n", err)
		return
	}

	if len(utxos) == 0 {
		fmt.Println("余额: 0 WES")
		return
	}

	// 计算总余额 - Amount是string类型
	var total uint64
	for _, utxo := range utxos {
		amount, err := strconv.ParseUint(utxo.Amount, 10, 64)
		if err != nil {
			fmt.Printf("解析金额失败: %v\n", err)
			continue
		}
		total += amount
	}

	// 注意：utxo.Amount 是最小单位（BaseUnit），展示给用户时必须换算为 WES
	fmt.Printf("余额: %s WES\n", utils.FormatWeiToDecimal(total))
	fmt.Printf("UTXO数量: %d\n", len(utxos))
}

// waitForEnter 等待用户按回车
func (s *AccountScreen) waitForEnter() {
	fmt.Print("\n按回车键继续...")
	fmt.Scanln()
}
