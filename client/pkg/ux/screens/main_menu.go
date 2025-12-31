package screens

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/weisyn/v1/client/core/contract"
	"github.com/weisyn/v1/client/core/mining"
	"github.com/weisyn/v1/client/core/resource"
	"github.com/weisyn/v1/client/core/transfer"
	"github.com/weisyn/v1/client/core/transport"
	"github.com/weisyn/v1/client/core/wallet"
	"github.com/weisyn/v1/client/pkg/ux/flows"
	"github.com/weisyn/v1/client/pkg/ux/ui"
)

// MainMenuScreen 主菜单屏幕
//
// 迁移自 _archived/old-internal-cli/internal/cli/presentation/screens/main_menu.go
// 对接新的 client/core 业务层
type MainMenuScreen struct {
	transport       transport.Client
	walletManager   *wallet.AccountManager
	transferService *transfer.TransferService
	miningService   *mining.MiningService
	contractService *contract.ContractService
	resourceService *resource.ResourceService
	contractFlow    *flows.ContractFlow // 合约交互式流程
	reader          *bufio.Reader
}

// NewMainMenuScreen 创建主菜单屏幕
func NewMainMenuScreen(
	client transport.Client,
	walletMgr *wallet.AccountManager,
	transferSvc *transfer.TransferService,
	miningSvc *mining.MiningService,
	contractSvc *contract.ContractService,
	resourceSvc *resource.ResourceService,
	uiComponents ui.Components,
) *MainMenuScreen {
	// 创建钱包适配器
	walletAdapter := NewWalletServiceAdapter(walletMgr)
	// 创建合约适配器（使用transport.Client和walletService）
	contractAdapter := NewContractServiceAdapter(client, walletAdapter)

	return &MainMenuScreen{
		transport:       client,
		walletManager:   walletMgr,
		transferService: transferSvc,
		miningService:   miningSvc,
		contractService: contractSvc,
		resourceService: resourceSvc,
		contractFlow:    flows.NewContractFlow(uiComponents, contractAdapter, walletAdapter),
		reader:          bufio.NewReader(os.Stdin),
	}
}

// Render 渲染主菜单并处理用户选择
func (s *MainMenuScreen) Render(ctx context.Context) error {
	for {
		// 清屏
		fmt.Print("\033[H\033[2J")

		// 显示欢迎信息
		fmt.Println("╔════════════════════════════════════════════════╗")
		fmt.Println("║          WES 区块链控制台                      ║")
		fmt.Println("║      欢迎使用微迅区块链系统！                  ║")
		fmt.Println("╚════════════════════════════════════════════════╝")
		fmt.Println()

		// 菜单选项
		fmt.Println("【主菜单】")
		fmt.Println()
		fmt.Println("  1. 账户管理    - 创建账户、查看余额、解锁/锁定账户")
		fmt.Println("  2. 转账操作    - 简单转账（用于测试节点功能）")
		fmt.Println("  3. 挖矿控制    - 启动/停止挖矿、查看算力和奖励")
		fmt.Println("  4. 区块信息    - 查看链信息、区块和交易详情")
		fmt.Println("  5. 节点状态    - 查看节点运行状态和同步情况")
		fmt.Println("  6. 使用帮助    - 获取功能说明和操作指南")
		fmt.Println("  0. 退出程序    - 安全退出控制台")
		fmt.Println()
		fmt.Print("请选择功能（输入数字）: ")

		// 读取用户输入
		var choice int
		_, err := fmt.Scanf("%d\n", &choice)
		if err != nil {
			fmt.Println("输入无效，请输入数字")
			s.waitForEnter()
			continue
		}

		// 检查context取消信号
		select {
		case <-ctx.Done():
			fmt.Println("\n收到退出信号，程序终止")
			return ctx.Err()
		default:
		}

		// 处理菜单选择
		if err := s.handleMenuSelection(ctx, choice); err != nil {
			if err.Error() == "exit" {
				return nil
			}
			fmt.Printf("\n操作失败: %v\n", err)
			s.waitForEnter()
		}
	}
}

// handleMenuSelection 处理菜单选择
func (s *MainMenuScreen) handleMenuSelection(ctx context.Context, choice int) error {
	switch choice {
	case 1:
		return s.handleAccountMenu(ctx)
	case 2:
		return s.handleTransferMenu(ctx)
	case 3:
		return s.handleMiningMenu(ctx)
	case 4:
		return s.handleBlockchainMenu(ctx)
	case 5:
		return s.handleSystemMenu(ctx)
	case 6:
		return s.handleHelpMenu(ctx)
	case 0:
		return s.handleExit()
	default:
		fmt.Println("\n无效选择，请重新输入")
		s.waitForEnter()
		return nil
	}
}

// handleAccountMenu 账户管理子菜单
func (s *MainMenuScreen) handleAccountMenu(ctx context.Context) error {
	for {
		fmt.Print("\033[H\033[2J")
		fmt.Println("【账户管理】")
		fmt.Println()
		fmt.Println("  1. 查看账户列表")
		fmt.Println("  2. 创建新账户")
		fmt.Println("  3. 查询余额")
		fmt.Println("  4. 解锁账户")
		fmt.Println("  5. 锁定账户")
		fmt.Println("  0. 返回主菜单")
		fmt.Println()
		fmt.Print("请选择: ")

		var choice int
		fmt.Scanf("%d\n", &choice)

		switch choice {
		case 1:
			s.showAccountList(ctx)
		case 2:
			s.createAccount(ctx)
		case 3:
			s.queryBalance(ctx)
		case 4:
			s.unlockAccount(ctx)
		case 5:
			s.lockAccount(ctx)
		case 0:
			return nil
		default:
			fmt.Println("无效选择")
			s.waitForEnter()
		}
	}
}

// handleTransferMenu 转账操作子菜单
func (s *MainMenuScreen) handleTransferMenu(ctx context.Context) error {
	// 直接执行简单转账，不再显示子菜单
	s.simpleTransfer(ctx)
	return nil
}

// handleMiningMenu 挖矿控制子菜单
func (s *MainMenuScreen) handleMiningMenu(ctx context.Context) error {
	for {
		fmt.Print("\033[H\033[2J")
		fmt.Println("【挖矿控制】")
		fmt.Println()
		fmt.Println("  1. 查看挖矿状态")
		fmt.Println("  2. 启动挖矿")
		fmt.Println("  3. 停止挖矿")
		fmt.Println("  4. 查看算力")
		fmt.Println("  5. 查询挖矿奖励")
		fmt.Println("  0. 返回主菜单")
		fmt.Println()
		fmt.Print("请选择: ")

		var choice int
		fmt.Scanf("%d\n", &choice)

		switch choice {
		case 1:
			s.showMiningStatus(ctx)
		case 2:
			s.startMining(ctx)
		case 3:
			s.stopMining(ctx)
		case 4:
			s.showHashrate(ctx)
		case 5:
			s.queryMiningRewards(ctx)
		case 0:
			return nil
		default:
			fmt.Println("无效选择")
			s.waitForEnter()
		}
	}
}

// handleResourceMenu 资源管理子菜单
func (s *MainMenuScreen) handleResourceMenu(ctx context.Context) error {
	for {
		fmt.Print("\033[H\033[2J")
		fmt.Println("【资源管理】")
		fmt.Println()
		fmt.Println("  1. 部署资源文件")
		fmt.Println("  2. 获取资源文件")
		fmt.Println("  3. 查询资源列表")
		fmt.Println("  0. 返回主菜单")
		fmt.Println()
		fmt.Print("请选择: ")

		var choice int
		fmt.Scanf("%d\n", &choice)

		switch choice {
		case 1:
			s.deployResource(ctx)
		case 2:
			s.fetchResource(ctx)
		case 3:
			s.queryResourceList(ctx)
		case 0:
			return nil
		default:
			fmt.Println("无效选择")
			s.waitForEnter()
		}
	}
}

// handleContractMenu 合约管理子菜单
func (s *MainMenuScreen) handleContractMenu(ctx context.Context) error {
	for {
		fmt.Print("\033[H\033[2J")
		fmt.Println("【合约管理】")
		fmt.Println()
		fmt.Println("  1. 部署合约")
		fmt.Println("  2. 调用合约")
		fmt.Println("  3. 查询合约状态")
		fmt.Println("  0. 返回主菜单")
		fmt.Println()
		fmt.Print("请选择: ")

		var choice int
		fmt.Scanf("%d\n", &choice)

		switch choice {
		case 1:
			s.deployContract(ctx)
		case 2:
			s.callContract(ctx)
		case 3:
			s.queryContractStatus(ctx)
		case 0:
			return nil
		default:
			fmt.Println("无效选择")
			s.waitForEnter()
		}
	}
}

// handleBlockchainMenu 区块信息子菜单
func (s *MainMenuScreen) handleBlockchainMenu(ctx context.Context) error {
	for {
		fmt.Print("\033[H\033[2J")
		fmt.Println("【区块信息】")
		fmt.Println()

		// 显示当前链尖信息（快速预览）
		currentHeight, err := s.transport.BlockNumber(ctx)
		if err == nil {
			fmt.Printf("📊 当前链尖高度: %d\n", currentHeight)
			fmt.Println()
		}

		fmt.Println("  1. 查询链信息")
		fmt.Println("  2. 查询区块详情")
		fmt.Println("  3. 查询交易详情")
		fmt.Println("  4. 查询交易池状态")
		fmt.Println("  0. 返回主菜单")
		fmt.Println()
		fmt.Print("请选择: ")

		var choice int
		fmt.Scanf("%d\n", &choice)

		switch choice {
		case 1:
			s.queryChainInfo(ctx)
		case 2:
			s.queryBlockInfo(ctx)
		case 3:
			s.queryTxInfo(ctx)
		case 4:
			s.queryTxPoolStatus(ctx)
		case 0:
			return nil
		default:
			fmt.Println("无效选择")
			s.waitForEnter()
		}
	}
}

// handleSystemMenu 节点状态（原系统中心，精简为只显示节点状态）
func (s *MainMenuScreen) handleSystemMenu(ctx context.Context) error {
	s.showNodeStatus(ctx)
	return nil
}

// handleHelpMenu 使用帮助
func (s *MainMenuScreen) handleHelpMenu(ctx context.Context) error {
	fmt.Print("\033[H\033[2J")
	fmt.Println("【使用帮助】")
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("WES 节点控制台 - 功能说明")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println("本控制台主要用于：")
	fmt.Println("  • 节点功能测试和验证")
	fmt.Println("  • 挖矿操作和控制")
	fmt.Println("  • 基础链信息查询")
	fmt.Println()
	fmt.Println("⚠️  重要提示：")
	fmt.Println("  本控制台不是生产钱包，仅提供基础功能用于节点测试。")
	fmt.Println("  如需高级钱包功能（批量转账、时间锁、账户导入导出等），")
	fmt.Println("  请使用 cmd/cli 或其他专业钱包工具。")
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println("功能列表：")
	fmt.Println()
	fmt.Println("1. 账户管理")
	fmt.Println("   - 创建账户、查看账户列表")
	fmt.Println("   - 查询余额")
	fmt.Println("   - 解锁/锁定账户（用于签名交易）")
	fmt.Println()
	fmt.Println("2. 转账操作")
	fmt.Println("   - 简单转账（1对1，用于测试节点交易功能）")
	fmt.Println()
	fmt.Println("3. 挖矿控制")
	fmt.Println("   - 查看挖矿状态、启动/停止挖矿")
	fmt.Println("   - 查看算力、查询挖矿奖励")
	fmt.Println()
	fmt.Println("4. 区块信息")
	fmt.Println("   - 查询链信息（链ID、高度、同步状态）")
	fmt.Println("   - 查询区块详情、交易详情")
	fmt.Println("   - 查询交易池状态")
	fmt.Println()
	fmt.Println("5. 节点状态")
	fmt.Println("   - 查看节点运行状态和同步情况")
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()
	s.waitForEnter()
	return nil
}

// handleExit 退出程序
func (s *MainMenuScreen) handleExit() error {
	fmt.Println("\n感谢使用WES区块链系统！")
	fmt.Println("再见！")
	return fmt.Errorf("exit")
}

// waitForEnter 等待用户按回车键
func (s *MainMenuScreen) waitForEnter() {
	fmt.Print("\n按回车键继续...")
	s.reader.ReadString('\n')
}

// ====== 账户管理功能实现 ======

func (s *MainMenuScreen) showAccountList(ctx context.Context) {
	fmt.Println("\n【账户列表】")
	accounts, err := s.walletManager.ListAccounts()
	if err != nil {
		fmt.Printf("查询失败: %v\n", err)
		s.waitForEnter()
		return
	}

	if len(accounts) == 0 {
		fmt.Println("暂无账户")
		s.waitForEnter()
		return
	}

	for i, acc := range accounts {
		fmt.Printf("%d. %s (标签: %s)\n", i+1, acc.Address, acc.Label)
	}

	s.waitForEnter()
}

func (s *MainMenuScreen) createAccount(ctx context.Context) {
	for {
		fmt.Print("\033[H\033[2J")
		fmt.Println("【创建钱包 / 账户】")
		fmt.Println()
		fmt.Println("请选择创建方式：")
		fmt.Println("  1. 助记词创建（推荐，可恢复）")
		fmt.Println("  2. 导入助记词（已有钱包恢复）")
		fmt.Println("  3. 高级：随机私钥创建（不推荐，不可恢复）")
		fmt.Println("  0. 返回上一级")
		fmt.Println()
		fmt.Print("请选择: ")

		var choice int
		if _, err := fmt.Scanf("%d\n", &choice); err != nil {
			fmt.Println("输入无效")
			s.waitForEnter()
			continue
		}

		switch choice {
		case 1:
			s.createAccountByMnemonic()
			return
		case 2:
			s.importAccountByMnemonic()
			return
		case 3:
			s.createAccountByRandomKey()
			return
		case 0:
			return
		default:
			fmt.Println("无效选择")
			s.waitForEnter()
		}
	}
}

func (s *MainMenuScreen) createAccountByMnemonic() {
	fmt.Print("\033[H\033[2J")
	fmt.Println("【助记词创建钱包（推荐）】")
	fmt.Println()

	label := s.readLine("请输入账户标签（可选）: ", true)

	password, ok := s.readPasswordWithConfirm()
	if !ok {
		return
	}

	// 1) 生成助记词（24词）
	mnemonic, err := s.walletManager.GenerateNewMnemonic(wallet.Mnemonic24Words)
	if err != nil {
		fmt.Printf("❌ 生成助记词失败: %v\n", err)
		s.waitForEnter()
		return
	}

	// 2) 展示助记词 + 强提示
	fmt.Println()
	fmt.Println("⚠️ 重要：请立即备份助记词（系统不会再次展示）")
	fmt.Println("   - 建议抄写在纸上离线保存")
	fmt.Println("   - 切勿截图/拍照/复制到聊天软件/云盘")
	fmt.Println("   - 切勿泄露给任何人")
	fmt.Println()
	fmt.Println("【助记词（24个单词）】")
	words := strings.Fields(mnemonic)
	for i, w := range words {
		fmt.Printf("%2d) %s\n", i+1, w)
	}
	fmt.Println()

	// 3) 抽查确认（防止“看一眼就过”）
	if !s.confirmMnemonicByChallenge(words) {
		fmt.Println("已取消创建（助记词未确认）")
		s.waitForEnter()
		return
	}

	// 4) 用助记词创建账户（不存储助记词，仅存 keystore 加密私钥）
	account, err := s.walletManager.CreateAccountFromMnemonic(mnemonic, "", password, label)
	if err != nil {
		fmt.Printf("❌ 创建失败: %v\n", err)
		s.waitForEnter()
		return
	}

	fmt.Println()
	fmt.Println("✅ 钱包创建成功！")
	fmt.Printf("地址: %s\n", account.Address)
	if account.Label != "" {
		fmt.Printf("标签: %s\n", account.Label)
	}
	fmt.Println()
	fmt.Println("提示：助记词不会被保存，丢失将无法恢复。")
	s.waitForEnter()
}

func (s *MainMenuScreen) importAccountByMnemonic() {
	fmt.Print("\033[H\033[2J")
	fmt.Println("【导入助记词（恢复钱包）】")
	fmt.Println()

	label := s.readLine("请输入账户标签（可选）: ", true)
	fmt.Println("请输入助记词（用空格分隔，例如 24 个单词）：")
	mnemonic := s.readLine("> ", false)

	// 1) 校验助记词格式
	ok, detail := s.walletManager.ValidateMnemonic(mnemonic)
	if !ok {
		fmt.Printf("❌ 助记词无效: %s\n", detail)
		s.waitForEnter()
		return
	}

	// 2) 预览地址（让用户确认没导错）
	addr, err := s.walletManager.DeriveAddressFromMnemonic(mnemonic, "", "")
	if err != nil {
		fmt.Printf("❌ 派生地址失败: %v\n", err)
		s.waitForEnter()
		return
	}
	fmt.Println()
	fmt.Println("将导入的钱包地址预览：")
	fmt.Printf("  %s\n", addr)
	fmt.Println()
	confirm := strings.ToLower(s.readLine("确认导入该助记词？输入 yes 继续，其它任意键取消: ", true))
	if confirm != "yes" {
		fmt.Println("已取消导入")
		s.waitForEnter()
		return
	}

	password, ok2 := s.readPasswordWithConfirm()
	if !ok2 {
		return
	}

	account, err := s.walletManager.CreateAccountFromMnemonic(mnemonic, "", password, label)
	if err != nil {
		fmt.Printf("❌ 导入失败: %v\n", err)
		s.waitForEnter()
		return
	}

	fmt.Println()
	fmt.Println("✅ 导入成功！")
	fmt.Printf("地址: %s\n", account.Address)
	if account.Label != "" {
		fmt.Printf("标签: %s\n", account.Label)
	}
	s.waitForEnter()
}

func (s *MainMenuScreen) createAccountByRandomKey() {
	fmt.Print("\033[H\033[2J")
	fmt.Println("【高级：随机私钥创建（不推荐）】")
	fmt.Println()
	fmt.Println("⚠️ 该方式不会生成助记词，一旦丢失 keystore 或密码，资金将无法恢复。")
	fmt.Println()

	label := s.readLine("请输入账户标签（可选）: ", true)

	password, ok := s.readPasswordWithConfirm()
	if !ok {
		return
	}

	// ✅ 修复参数顺序：CreateAccount(password, label)
	account, err := s.walletManager.CreateAccount(password, label)
	if err != nil {
		fmt.Printf("❌ 创建失败: %v\n", err)
		s.waitForEnter()
		return
	}

	fmt.Println()
	fmt.Println("✅ 账户创建成功！")
	fmt.Printf("地址: %s\n", account.Address)
	if account.Label != "" {
		fmt.Printf("标签: %s\n", account.Label)
	}
	s.waitForEnter()
}

func (s *MainMenuScreen) readLine(prompt string, allowEmpty bool) string {
	for {
		fmt.Print(prompt)
		line, _ := s.reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" && !allowEmpty {
			fmt.Println("输入不能为空，请重试")
			continue
		}
		return line
	}
}

func (s *MainMenuScreen) readPasswordWithConfirm() (string, bool) {
	password := s.readLine("请输入密码（至少8位）: ", false)
	if len(password) < 8 {
		fmt.Println("❌ 密码长度不能少于8位")
		s.waitForEnter()
		return "", false
	}
	confirm := s.readLine("请再次输入密码确认: ", false)
	if password != confirm {
		fmt.Println("❌ 两次输入的密码不一致")
		s.waitForEnter()
		return "", false
	}
	return password, true
}

func (s *MainMenuScreen) confirmMnemonicByChallenge(words []string) bool {
	if len(words) < 12 {
		// 非预期（理论上不会发生）
		return strings.ToLower(s.readLine("是否确认已备份助记词？输入 yes 继续: ", false)) == "yes"
	}

	fmt.Println("为了确认您已正确备份，请完成抽查：")
	fmt.Println("（提示：请按上面序号找到对应单词，输入单词本身）")
	fmt.Println()

	// 固定抽查 3 个位置（避免引入随机数/依赖），也能阻止“直接回车”
	challenges := []int{3, 12, 20}
	for _, idx := range challenges {
		if idx < 1 || idx > len(words) {
			continue
		}
		ans := strings.ToLower(s.readLine(fmt.Sprintf("请输入第 %d 个单词: ", idx), false))
		if ans != strings.ToLower(words[idx-1]) {
			fmt.Println("❌ 校验失败：助记词单词不匹配")
			return false
		}
	}

	fmt.Println("✅ 抽查通过")
	return true
}

func (s *MainMenuScreen) queryBalance(ctx context.Context) {
	fmt.Println("\n【查询余额】")
	fmt.Print("请输入地址（回车使用默认账户）: ")
	var address string
	fmt.Scanln(&address)

	if address == "" {
		accounts, _ := s.walletManager.ListAccounts()
		if len(accounts) > 0 {
			address = accounts[0].Address
		}
	}

	balance, err := s.transferService.GetBalance(ctx, address)
	if err != nil {
		fmt.Printf("❌ 查询失败: %v\n", err)
		fmt.Println()
		fmt.Println("💡 可能的原因：")
		fmt.Println("  - 节点未就绪或网络连接失败")
		fmt.Println("  - 地址格式错误")
		fmt.Println("  - 链尚未初始化（未挖出创世块）")
		fmt.Println("  - 该地址确实没有余额")
		s.waitForEnter()
		return
	}

	fmt.Printf("地址: %s\n", address)
	fmt.Printf("余额: %s WES\n", balance)
	s.waitForEnter()
}

func (s *MainMenuScreen) unlockAccount(ctx context.Context) {
	fmt.Print("\033[H\033[2J")
	fmt.Println("【解锁账户】")
	fmt.Println()

	// 获取账户列表
	accounts, err := s.walletManager.ListAccounts()
	if err != nil {
		fmt.Printf("❌ 获取账户列表失败: %v\n", err)
		s.waitForEnter()
		return
	}

	if len(accounts) == 0 {
		fmt.Println("❌ 没有可用账户")
		s.waitForEnter()
		return
	}

	// 显示账户列表（标注解锁状态）
	fmt.Println("可用账户：")
	for i, acc := range accounts {
		unlocked := s.walletManager.IsWalletUnlocked(acc.Address)
		status := "🔒 已锁定"
		if unlocked {
			status = "🔓 已解锁"
		}
		fmt.Printf("  %d. %s (标签: %s) %s\n", i+1, acc.Address, acc.Label, status)
	}
	fmt.Println()

	// 选择账户
	var accountIndex int
	fmt.Print("请选择要解锁的账户（输入序号）: ")
	var input string
	fmt.Scanln(&input)
	if input == "" {
		fmt.Println("❌ 无效选择")
		s.waitForEnter()
		return
	}
	if _, err := fmt.Sscanf(input, "%d", &accountIndex); err != nil || accountIndex < 1 || accountIndex > len(accounts) {
		fmt.Println("❌ 无效选择")
		s.waitForEnter()
		return
	}
	accountIndex-- // 转换为索引

	selectedAccount := accounts[accountIndex]

	// 检查是否已解锁
	if s.walletManager.IsWalletUnlocked(selectedAccount.Address) {
		fmt.Printf("✓ 账户 %s 已经解锁\n", selectedAccount.Address)
		s.waitForEnter()
		return
	}

	// 输入密码
	fmt.Print("请输入账户密码: ")
	var password string
	fmt.Scanln(&password)
	if password == "" {
		fmt.Println("❌ 密码不能为空")
		s.waitForEnter()
		return
	}
	fmt.Println()

	// 解锁账户
	fmt.Println("正在解锁账户...")
	if err := s.walletManager.UnlockWallet(selectedAccount.Address, password); err != nil {
		fmt.Printf("❌ 解锁失败: %v\n", err)
		fmt.Println("💡 提示: 请检查密码是否正确")
		s.waitForEnter()
		return
	}

	fmt.Printf("✅ 账户 %s 已成功解锁\n", selectedAccount.Address)
	fmt.Println("💡 提示: 解锁后的账户可用于签名交易，直到程序退出或手动锁定")
	s.waitForEnter()
}

func (s *MainMenuScreen) lockAccount(ctx context.Context) {
	fmt.Print("\033[H\033[2J")
	fmt.Println("【锁定账户】")
	fmt.Println()

	// 获取账户列表
	accounts, err := s.walletManager.ListAccounts()
	if err != nil {
		fmt.Printf("❌ 获取账户列表失败: %v\n", err)
		s.waitForEnter()
		return
	}

	if len(accounts) == 0 {
		fmt.Println("❌ 没有可用账户")
		s.waitForEnter()
		return
	}

	// 显示已解锁的账户
	unlockedAccounts := []*wallet.AccountInfo{}
	for _, acc := range accounts {
		if s.walletManager.IsWalletUnlocked(acc.Address) {
			unlockedAccounts = append(unlockedAccounts, acc)
		}
	}

	if len(unlockedAccounts) == 0 {
		fmt.Println("✓ 没有已解锁的账户")
		s.waitForEnter()
		return
	}

	fmt.Println("已解锁的账户：")
	for i, acc := range unlockedAccounts {
		fmt.Printf("  %d. %s (标签: %s)\n", i+1, acc.Address, acc.Label)
	}
	fmt.Println()

	// 选择账户
	var accountIndex int
	fmt.Print("请选择要锁定的账户（输入序号）: ")
	var input string
	fmt.Scanln(&input)
	if input == "" {
		fmt.Println("❌ 无效选择")
		s.waitForEnter()
		return
	}
	if _, err := fmt.Sscanf(input, "%d", &accountIndex); err != nil || accountIndex < 1 || accountIndex > len(unlockedAccounts) {
		fmt.Println("❌ 无效选择")
		s.waitForEnter()
		return
	}
	accountIndex-- // 转换为索引

	selectedAccount := unlockedAccounts[accountIndex]

	// 锁定账户
	if err := s.walletManager.LockWallet(selectedAccount.Address); err != nil {
		fmt.Printf("❌ 锁定失败: %v\n", err)
		s.waitForEnter()
		return
	}

	fmt.Printf("✅ 账户 %s 已成功锁定\n", selectedAccount.Address)
	fmt.Println("💡 提示: 锁定后的账户需要重新解锁才能用于签名交易")
	s.waitForEnter()
}

func (s *MainMenuScreen) exportAccount(ctx context.Context) {
	fmt.Print("\033[H\033[2J")
	fmt.Println("【导出账户】")
	fmt.Println()
	fmt.Println("⚠️ 警告: 导出私钥后，请妥善保管，不要泄露给他人！")
	fmt.Println()

	// 获取账户列表
	accounts, err := s.walletManager.ListAccounts()
	if err != nil {
		fmt.Printf("❌ 获取账户列表失败: %v\n", err)
		s.waitForEnter()
		return
	}

	if len(accounts) == 0 {
		fmt.Println("❌ 没有可用账户")
		s.waitForEnter()
		return
	}

	// 显示账户列表
	fmt.Println("可用账户：")
	for i, acc := range accounts {
		fmt.Printf("  %d. %s (标签: %s)\n", i+1, acc.Address, acc.Label)
	}
	fmt.Println()

	// 选择账户
	var accountIndex int
	fmt.Print("请选择要导出的账户（输入序号）: ")
	var input string
	fmt.Scanln(&input)
	if input == "" {
		fmt.Println("❌ 无效选择")
		s.waitForEnter()
		return
	}
	if _, err := fmt.Sscanf(input, "%d", &accountIndex); err != nil || accountIndex < 1 || accountIndex > len(accounts) {
		fmt.Println("❌ 无效选择")
		s.waitForEnter()
		return
	}
	accountIndex-- // 转换为索引

	selectedAccount := accounts[accountIndex]

	// 输入密码
	fmt.Print("请输入账户密码（用于解密私钥）: ")
	var password string
	fmt.Scanln(&password)
	if password == "" {
		fmt.Println("❌ 密码不能为空")
		s.waitForEnter()
		return
	}
	fmt.Println()

	// 导出私钥
	fmt.Println("正在导出私钥...")
	privateKeyHex, err := s.walletManager.ExportPrivateKey(selectedAccount.Address, password)
	if err != nil {
		fmt.Printf("❌ 导出失败: %v\n", err)
		fmt.Println("💡 提示: 请检查密码是否正确")
		s.waitForEnter()
		return
	}

	// 显示私钥
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("✅ 私钥导出成功！")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("账户地址: %s\n", selectedAccount.Address)
	fmt.Printf("账户标签: %s\n", selectedAccount.Label)
	fmt.Println()
	fmt.Println("私钥（十六进制）:")
	fmt.Printf("  %s\n", privateKeyHex)
	fmt.Println()
	fmt.Println("⚠️ 重要提示:")
	fmt.Println("  - 请妥善保管此私钥，不要泄露给他人")
	fmt.Println("  - 私钥丢失将无法恢复账户")
	fmt.Println("  - 建议将私钥保存在安全的地方（如密码管理器）")
	fmt.Println("  - 可以使用此私钥在其他钱包中导入账户")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()

	s.waitForEnter()
}

func (s *MainMenuScreen) importAccount(ctx context.Context) {
	fmt.Print("\033[H\033[2J")
	fmt.Println("【导入账户】")
	fmt.Println()
	fmt.Println("💡 提示: 导入私钥将创建一个新的账户，使用您提供的私钥")
	fmt.Println()

	// 输入私钥
	fmt.Println("请输入私钥（十六进制格式，支持 0x 或 Cf 前缀）:")
	fmt.Println("例如: abc123... 或 0xabc123... 或 Cfabc123...")
	fmt.Print("私钥: ")
	var privateKeyHex string
	fmt.Scanln(&privateKeyHex)
	if privateKeyHex == "" {
		fmt.Println("❌ 私钥不能为空")
		s.waitForEnter()
		return
	}
	fmt.Println()

	// 输入密码（用于加密新账户）
	fmt.Print("请输入密码（用于加密新账户）: ")
	var password string
	fmt.Scanln(&password)
	if password == "" {
		fmt.Println("❌ 密码不能为空")
		s.waitForEnter()
		return
	}
	fmt.Println()

	// 确认密码
	fmt.Print("请再次输入密码（确认）: ")
	var passwordConfirm string
	fmt.Scanln(&passwordConfirm)
	if password != passwordConfirm {
		fmt.Println("❌ 两次输入的密码不一致")
		s.waitForEnter()
		return
	}
	fmt.Println()

	// 输入标签
	fmt.Print("请输入账户标签（可选，直接回车跳过）: ")
	var label string
	fmt.Scanln(&label)
	if label == "" {
		label = "导入的账户"
	}
	fmt.Println()

	// 导入账户
	fmt.Println("正在导入账户...")
	account, err := s.walletManager.ImportPrivateKey(privateKeyHex, password, label)
	if err != nil {
		fmt.Printf("❌ 导入失败: %v\n", err)
		fmt.Println()
		fmt.Println("💡 可能的原因：")
		fmt.Println("  - 私钥格式错误（应为64字符十六进制）")
		fmt.Println("  - 私钥长度不正确（应为32字节）")
		fmt.Println("  - 该账户已存在")
		s.waitForEnter()
		return
	}

	// 显示结果
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("✅ 账户导入成功！")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("账户地址: %s\n", account.Address)
	fmt.Printf("账户标签: %s\n", account.Label)
	fmt.Printf("Keystore路径: %s\n", account.KeystorePath)
	fmt.Println()
	fmt.Println("💡 提示: 账户已成功导入，您现在可以使用此账户进行转账等操作")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()

	s.waitForEnter()
}

// ====== 转账功能实现 ======

func (s *MainMenuScreen) simpleTransfer(ctx context.Context) {
	fmt.Print("\033[H\033[2J")
	fmt.Println("【简单转账】")
	fmt.Println()

	// 1. 获取账户列表，让用户选择发送方账户
	accounts, err := s.walletManager.ListAccounts()
	if err != nil {
		fmt.Printf("❌ 获取账户列表失败: %v\n", err)
		s.waitForEnter()
		return
	}

	if len(accounts) == 0 {
		fmt.Println("❌ 没有可用账户，请先创建账户")
		s.waitForEnter()
		return
	}

	// 显示账户列表（标注解锁状态）
	fmt.Println("可用账户：")
	for i, acc := range accounts {
		unlocked := s.walletManager.IsWalletUnlocked(acc.Address)
		status := "🔒"
		if unlocked {
			status = "🔓"
		}
		fmt.Printf("  %d. %s %s (标签: %s)\n", i+1, status, acc.Address, acc.Label)
	}
	fmt.Println()

	// 2. 选择发送方账户
	var fromIndex int
	fmt.Print("请选择发送方账户（输入序号，回车使用第一个）: ")
	var fromInput string
	fmt.Scanln(&fromInput)
	if fromInput == "" {
		fromIndex = 0
	} else {
		if _, err := fmt.Sscanf(fromInput, "%d", &fromIndex); err != nil || fromIndex < 1 || fromIndex > len(accounts) {
			fmt.Printf("❌ 无效选择，使用第一个账户\n")
			fromIndex = 1
		}
		fromIndex-- // 转换为索引
	}

	fromAccount := accounts[fromIndex]
	fmt.Printf("✓ 已选择发送方: %s\n", fromAccount.Address)
	fmt.Println()

	// 3. 输入接收方地址
	fmt.Print("请输入接收方地址: ")
	var toAddress string
	fmt.Scanln(&toAddress)
	if toAddress == "" {
		fmt.Println("❌ 接收方地址不能为空")
		s.waitForEnter()
		return
	}
	fmt.Println()

	// 4. 输入转账金额
	fmt.Print("请输入转账金额（WES单位，例如: 100.5）: ")
	var amountStr string
	fmt.Scanln(&amountStr)
	if amountStr == "" {
		fmt.Println("❌ 转账金额不能为空")
		s.waitForEnter()
		return
	}
	fmt.Println()

	// 5. 检查账户是否已解锁，如果未解锁则要求输入密码
	var password string
	var privateKey []byte

	if s.walletManager.IsWalletUnlocked(fromAccount.Address) {
		fmt.Println("✓ 账户已解锁，直接使用")
		var err error
		privateKey, err = s.walletManager.GetPrivateKey(fromAccount.Address, "")
		if err != nil {
			fmt.Printf("❌ 获取私钥失败: %v\n", err)
			s.waitForEnter()
			return
		}
	} else {
		// 需要解锁
		fmt.Print("请输入账户密码（用于解锁签名）: ")
		fmt.Scanln(&password)
		if password == "" {
			fmt.Println("❌ 密码不能为空")
			s.waitForEnter()
			return
		}
		fmt.Println()

		fmt.Println("正在解锁账户...")
		if err := s.walletManager.UnlockWallet(fromAccount.Address, password); err != nil {
			fmt.Printf("❌ 解锁账户失败: %v\n", err)
			fmt.Println("💡 提示: 请检查密码是否正确")
			s.waitForEnter()
			return
		}
		fmt.Println("✓ 账户已解锁")

		var err error
		privateKey, err = s.walletManager.GetPrivateKey(fromAccount.Address, password)
		if err != nil {
			fmt.Printf("❌ 获取私钥失败: %v\n", err)
			s.waitForEnter()
			return
		}
	}

	defer func() {
		// 安全清零私钥
		for i := range privateKey {
			privateKey[i] = 0
		}
	}()

	// 6. 可选：备注
	fmt.Print("请输入备注（可选，直接回车跳过）: ")
	var memo string
	fmt.Scanln(&memo)
	fmt.Println()

	// 7. 构建转账请求
	req := &transfer.TransferRequest{
		FromAddress: fromAccount.Address,
		ToAddress:   toAddress,
		Amount:      amountStr,
		PrivateKey:  privateKey,
		Memo:        memo,
	}

	// 8. 执行转账
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("开始执行转账...")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()

	result, err := s.transferService.ExecuteTransfer(ctx, req)
	if err != nil {
		fmt.Printf("\n❌ 转账失败: %v\n", err)
		fmt.Println()
		fmt.Println("💡 可能的原因：")
		fmt.Println("  - 余额不足")
		fmt.Println("  - 接收方地址格式错误")
		fmt.Println("  - 节点未就绪或网络连接失败")
		fmt.Println("  - 交易构建或签名失败")
		s.waitForEnter()
		return
	}

	// 9. 显示结果
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("✅ 转账成功！")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("交易ID (TxID): %s\n", result.TxID)
	fmt.Printf("交易哈希 (TxHash): %s\n", result.TxHash)
	fmt.Printf("手续费: %s WES\n", result.Fee)
	if result.Change != "" && result.Change != "0" {
		fmt.Printf("找零: %s WES\n", result.Change)
	}
	if result.BlockHeight > 0 {
		fmt.Printf("区块高度: %d\n", result.BlockHeight)
	} else {
		fmt.Println("状态: 待确认（交易已提交到网络，等待打包）")
	}
	fmt.Println()

	s.waitForEnter()
}

func (s *MainMenuScreen) batchTransfer(ctx context.Context) {
	fmt.Print("\033[H\033[2J")
	fmt.Println("【批量转账】")
	fmt.Println()
	fmt.Println("💡 提示: 批量转账允许您一次向多个地址转账")
	fmt.Println()

	// 1. 获取账户列表，让用户选择发送方账户
	accounts, err := s.walletManager.ListAccounts()
	if err != nil {
		fmt.Printf("❌ 获取账户列表失败: %v\n", err)
		s.waitForEnter()
		return
	}

	if len(accounts) == 0 {
		fmt.Println("❌ 没有可用账户，请先创建账户")
		s.waitForEnter()
		return
	}

	// 显示账户列表（标注解锁状态）
	fmt.Println("可用账户：")
	for i, acc := range accounts {
		unlocked := s.walletManager.IsWalletUnlocked(acc.Address)
		status := "🔒"
		if unlocked {
			status = "🔓"
		}
		fmt.Printf("  %d. %s %s (标签: %s)\n", i+1, status, acc.Address, acc.Label)
	}
	fmt.Println()

	// 2. 选择发送方账户
	var fromIndex int
	fmt.Print("请选择发送方账户（输入序号，回车使用第一个）: ")
	var fromInput string
	fmt.Scanln(&fromInput)
	if fromInput == "" {
		fromIndex = 0
	} else {
		if _, err := fmt.Sscanf(fromInput, "%d", &fromIndex); err != nil || fromIndex < 1 || fromIndex > len(accounts) {
			fmt.Printf("❌ 无效选择，使用第一个账户\n")
			fromIndex = 1
		}
		fromIndex-- // 转换为索引
	}

	fromAccount := accounts[fromIndex]
	fmt.Printf("✓ 已选择发送方: %s\n", fromAccount.Address)
	fmt.Println()

	// 3. 检查账户是否已解锁，如果未解锁则要求输入密码
	var password string
	var privateKey []byte

	if s.walletManager.IsWalletUnlocked(fromAccount.Address) {
		fmt.Println("✓ 账户已解锁，直接使用")
		var err error
		privateKey, err = s.walletManager.GetPrivateKey(fromAccount.Address, "")
		if err != nil {
			fmt.Printf("❌ 获取私钥失败: %v\n", err)
			s.waitForEnter()
			return
		}
	} else {
		// 需要解锁
		fmt.Print("请输入账户密码（用于解锁签名）: ")
		fmt.Scanln(&password)
		if password == "" {
			fmt.Println("❌ 密码不能为空")
			s.waitForEnter()
			return
		}
		fmt.Println()

		fmt.Println("正在解锁账户...")
		if err := s.walletManager.UnlockWallet(fromAccount.Address, password); err != nil {
			fmt.Printf("❌ 解锁账户失败: %v\n", err)
			fmt.Println("💡 提示: 请检查密码是否正确")
			s.waitForEnter()
			return
		}
		fmt.Println("✓ 账户已解锁")

		var err error
		privateKey, err = s.walletManager.GetPrivateKey(fromAccount.Address, password)
		if err != nil {
			fmt.Printf("❌ 获取私钥失败: %v\n", err)
			s.waitForEnter()
			return
		}
	}

	defer func() {
		// 安全清零私钥
		for i := range privateKey {
			privateKey[i] = 0
		}
	}()

	// 4. 收集收款人信息
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("请输入收款人信息（每行一个，格式：地址,金额）")
	fmt.Println("例如：Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn,100.5")
	fmt.Println("输入空行结束输入")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()

	recipients := []transfer.BatchRecipient{}
	recipientNum := 1

	for {
		fmt.Printf("收款人 %d (地址,金额): ", recipientNum)
		var line string
		fmt.Scanln(&line)

		if line == "" {
			if recipientNum == 1 {
				fmt.Println("❌ 至少需要输入一个收款人")
				continue
			}
			break
		}

		// 解析输入：地址,金额
		parts := strings.Split(line, ",")
		if len(parts) != 2 {
			fmt.Println("❌ 格式错误，请使用：地址,金额")
			continue
		}

		address := strings.TrimSpace(parts[0])
		amount := strings.TrimSpace(parts[1])

		if address == "" || amount == "" {
			fmt.Println("❌ 地址和金额都不能为空")
			continue
		}

		recipients = append(recipients, transfer.BatchRecipient{
			Address: address,
			Amount:  amount,
		})

		fmt.Printf("✓ 已添加收款人 %d: %s, %s WES\n", recipientNum, address, amount)
		recipientNum++
	}

	if len(recipients) == 0 {
		fmt.Println("❌ 没有有效的收款人")
		s.waitForEnter()
		return
	}

	fmt.Println()
	fmt.Printf("✓ 共添加 %d 个收款人\n", len(recipients))
	fmt.Println()

	// 5. 可选：备注
	fmt.Print("请输入备注（可选，直接回车跳过）: ")
	var memo string
	fmt.Scanln(&memo)
	fmt.Println()

	// 6. 创建批量转账服务
	batchService := transfer.NewBatchTransferService(s.transport, nil)

	// 7. 构建批量转账请求
	req := &transfer.BatchTransferRequest{
		FromAddress: fromAccount.Address,
		Recipients:  recipients,
		PrivateKey:  privateKey,
		Memo:        memo,
	}

	// 8. 执行批量转账
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("开始执行批量转账...")
	fmt.Printf("发送方: %s\n", fromAccount.Address)
	fmt.Printf("收款人数量: %d\n", len(recipients))
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()

	result, err := batchService.ExecuteBatchTransfer(ctx, req)
	if err != nil {
		fmt.Printf("\n❌ 批量转账失败: %v\n", err)
		fmt.Println()
		fmt.Println("💡 可能的原因：")
		fmt.Println("  - 余额不足（总金额 + 手续费）")
		fmt.Println("  - 收款人地址格式错误")
		fmt.Println("  - 节点未就绪或网络连接失败")
		fmt.Println("  - 交易构建或签名失败")
		s.waitForEnter()
		return
	}

	// 9. 显示结果
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("✅ 批量转账成功！")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("交易ID (TxID): %s\n", result.TxID)
	fmt.Printf("交易哈希 (TxHash): %s\n", result.TxHash)
	fmt.Printf("收款人数量: %d\n", result.Recipients)
	fmt.Printf("总转账金额: %s WES\n", result.TotalAmount)
	fmt.Printf("手续费: %s WES\n", result.Fee)
	if result.Change != "" && result.Change != "0" {
		fmt.Printf("找零: %s WES\n", result.Change)
	}
	if result.BlockHeight > 0 {
		fmt.Printf("区块高度: %d\n", result.BlockHeight)
	} else {
		fmt.Println("状态: 待确认（交易已提交到网络，等待打包）")
	}

	// 显示失败的收款人（如果有）
	if len(result.FailedItems) > 0 {
		fmt.Println()
		fmt.Println("⚠️ 部分收款人验证失败：")
		for _, item := range result.FailedItems {
			fmt.Printf("  - %s (%s): %s\n", item.Address, item.Amount, item.Reason)
		}
	}

	fmt.Println()

	s.waitForEnter()
}

func (s *MainMenuScreen) timelockTransfer(ctx context.Context) {
	fmt.Print("\033[H\033[2J")
	fmt.Println("【时间锁转账】")
	fmt.Println()
	fmt.Println("💡 提示: 时间锁转账允许您设置一个解锁时间，接收方只能在此时间之后花费资金")
	fmt.Println()

	// 1. 获取账户列表，让用户选择发送方账户
	accounts, err := s.walletManager.ListAccounts()
	if err != nil {
		fmt.Printf("❌ 获取账户列表失败: %v\n", err)
		s.waitForEnter()
		return
	}

	if len(accounts) == 0 {
		fmt.Println("❌ 没有可用账户，请先创建账户")
		s.waitForEnter()
		return
	}

	// 显示账户列表（标注解锁状态）
	fmt.Println("可用账户：")
	for i, acc := range accounts {
		unlocked := s.walletManager.IsWalletUnlocked(acc.Address)
		status := "🔒"
		if unlocked {
			status = "🔓"
		}
		fmt.Printf("  %d. %s %s (标签: %s)\n", i+1, status, acc.Address, acc.Label)
	}
	fmt.Println()

	// 2. 选择发送方账户
	var fromIndex int
	fmt.Print("请选择发送方账户（输入序号，回车使用第一个）: ")
	var fromInput string
	fmt.Scanln(&fromInput)
	if fromInput == "" {
		fromIndex = 0
	} else {
		if _, err := fmt.Sscanf(fromInput, "%d", &fromIndex); err != nil || fromIndex < 1 || fromIndex > len(accounts) {
			fmt.Printf("❌ 无效选择，使用第一个账户\n")
			fromIndex = 1
		}
		fromIndex-- // 转换为索引
	}

	fromAccount := accounts[fromIndex]
	fmt.Printf("✓ 已选择发送方: %s\n", fromAccount.Address)
	fmt.Println()

	// 3. 输入接收方地址
	fmt.Print("请输入接收方地址: ")
	var toAddress string
	fmt.Scanln(&toAddress)
	if toAddress == "" {
		fmt.Println("❌ 接收方地址不能为空")
		s.waitForEnter()
		return
	}
	fmt.Println()

	// 4. 输入转账金额
	fmt.Print("请输入转账金额（WES单位，例如: 100.5）: ")
	var amountStr string
	fmt.Scanln(&amountStr)
	if amountStr == "" {
		fmt.Println("❌ 转账金额不能为空")
		s.waitForEnter()
		return
	}
	fmt.Println()

	// 5. 输入解锁时间
	fmt.Println("请输入解锁时间（接收方可以花费资金的时间）")
	fmt.Println("格式1: 日期时间（例如: 2024-12-31 23:59:59）")
	fmt.Println("格式2: 相对时间（例如: 30天 或 720小时）")
	fmt.Print("解锁时间: ")
	var timeInput string
	fmt.Scanln(&timeInput)
	if timeInput == "" {
		fmt.Println("❌ 解锁时间不能为空")
		s.waitForEnter()
		return
	}

	var unlockTime time.Time
	var err2 error

	// 尝试解析为日期时间格式
	unlockTime, err2 = time.Parse("2006-01-02 15:04:05", timeInput)
	if err2 != nil {
		// 尝试解析为日期格式
		unlockTime, err2 = time.Parse("2006-01-02", timeInput)
		if err2 != nil {
			// 尝试解析为相对时间（例如: 30天）
			unlockTime, err2 = parseRelativeTime(timeInput)
			if err2 != nil {
				fmt.Printf("❌ 时间格式错误: %v\n", err2)
				fmt.Println("💡 支持的格式:")
				fmt.Println("  - 日期时间: 2024-12-31 23:59:59")
				fmt.Println("  - 日期: 2024-12-31")
				fmt.Println("  - 相对时间: 30天 或 720小时")
				s.waitForEnter()
				return
			}
		}
	}

	// 验证解锁时间必须在未来
	if !unlockTime.After(time.Now()) {
		fmt.Println("❌ 解锁时间必须在未来")
		s.waitForEnter()
		return
	}

	fmt.Printf("✓ 解锁时间: %s\n", unlockTime.Format("2006-01-02 15:04:05"))
	fmt.Println()

	// 6. 检查账户是否已解锁，如果未解锁则要求输入密码
	var password string
	var privateKey []byte

	if s.walletManager.IsWalletUnlocked(fromAccount.Address) {
		fmt.Println("✓ 账户已解锁，直接使用")
		privateKey, err2 = s.walletManager.GetPrivateKey(fromAccount.Address, "")
		if err2 != nil {
			fmt.Printf("❌ 获取私钥失败: %v\n", err2)
			s.waitForEnter()
			return
		}
	} else {
		// 需要解锁
		fmt.Print("请输入账户密码（用于解锁签名）: ")
		fmt.Scanln(&password)
		if password == "" {
			fmt.Println("❌ 密码不能为空")
			s.waitForEnter()
			return
		}
		fmt.Println()

		fmt.Println("正在解锁账户...")
		if err2 = s.walletManager.UnlockWallet(fromAccount.Address, password); err2 != nil {
			fmt.Printf("❌ 解锁账户失败: %v\n", err2)
			fmt.Println("💡 提示: 请检查密码是否正确")
			s.waitForEnter()
			return
		}
		fmt.Println("✓ 账户已解锁")

		privateKey, err2 = s.walletManager.GetPrivateKey(fromAccount.Address, password)
		if err2 != nil {
			fmt.Printf("❌ 获取私钥失败: %v\n", err2)
			s.waitForEnter()
			return
		}
	}

	defer func() {
		// 安全清零私钥
		for i := range privateKey {
			privateKey[i] = 0
		}
	}()

	// 7. 可选：备注
	fmt.Print("请输入备注（可选，直接回车跳过）: ")
	var memo string
	fmt.Scanln(&memo)
	fmt.Println()

	// 8. 创建时间锁转账服务
	timeLockService := transfer.NewTimeLockTransferService(s.transport, nil)

	// 9. 构建时间锁转账请求
	req := &transfer.TimeLockTransferRequest{
		FromAddress: fromAccount.Address,
		ToAddress:   toAddress,
		Amount:      amountStr,
		PrivateKey:  privateKey,
		UnlockTime:  unlockTime,
		Memo:        memo,
	}

	// 10. 执行时间锁转账
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("开始执行时间锁转账...")
	fmt.Printf("发送方: %s\n", fromAccount.Address)
	fmt.Printf("接收方: %s\n", toAddress)
	fmt.Printf("金额: %s WES\n", amountStr)
	fmt.Printf("解锁时间: %s\n", unlockTime.Format("2006-01-02 15:04:05"))
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()

	result, err := timeLockService.ExecuteTimeLockTransfer(ctx, req)
	if err != nil {
		fmt.Printf("\n❌ 时间锁转账失败: %v\n", err)
		fmt.Println()
		fmt.Println("💡 可能的原因：")
		fmt.Println("  - 余额不足（金额 + 手续费）")
		fmt.Println("  - 接收方地址格式错误")
		fmt.Println("  - 解锁时间格式错误或不在未来")
		fmt.Println("  - 节点未就绪或网络连接失败")
		fmt.Println("  - 交易构建或签名失败")
		s.waitForEnter()
		return
	}

	// 11. 显示结果
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("✅ 时间锁转账成功！")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("交易ID (TxID): %s\n", result.TxID)
	fmt.Printf("交易哈希 (TxHash): %s\n", result.TxHash)
	fmt.Printf("转账金额: %s WES\n", result.Amount)
	fmt.Printf("手续费: %s WES\n", result.Fee)
	if result.Change != "" && result.Change != "0" {
		fmt.Printf("找零: %s WES\n", result.Change)
	}
	fmt.Printf("解锁时间: %s\n", result.UnlockTime.Format("2006-01-02 15:04:05"))
	if result.BlockHeight > 0 {
		fmt.Printf("区块高度: %d\n", result.BlockHeight)
	} else {
		fmt.Println("状态: 待确认（交易已提交到网络，等待打包）")
	}
	fmt.Println()
	fmt.Println("💡 提示: 接收方只能在解锁时间之后花费这笔资金")
	fmt.Println()

	s.waitForEnter()
}

// parseRelativeTime 解析相对时间（例如: "30天" 或 "720小时"）
func parseRelativeTime(input string) (time.Time, error) {
	input = strings.TrimSpace(input)
	now := time.Now()

	// 尝试解析 "数字+单位" 格式
	var value int
	var unit string
	if _, err := fmt.Sscanf(input, "%d%s", &value, &unit); err != nil {
		return time.Time{}, fmt.Errorf("invalid relative time format")
	}

	unit = strings.ToLower(strings.TrimSpace(unit))

	switch {
	case strings.HasPrefix(unit, "天") || strings.HasPrefix(unit, "day") || unit == "d":
		return now.AddDate(0, 0, value), nil
	case strings.HasPrefix(unit, "小时") || strings.HasPrefix(unit, "hour") || unit == "h":
		return now.Add(time.Duration(value) * time.Hour), nil
	case strings.HasPrefix(unit, "分钟") || strings.HasPrefix(unit, "minute") || unit == "m":
		return now.Add(time.Duration(value) * time.Minute), nil
	case strings.HasPrefix(unit, "周") || strings.HasPrefix(unit, "week") || unit == "w":
		return now.AddDate(0, 0, value*7), nil
	case strings.HasPrefix(unit, "月") || strings.HasPrefix(unit, "month"):
		return now.AddDate(0, value, 0), nil
	case strings.HasPrefix(unit, "年") || strings.HasPrefix(unit, "year") || unit == "y":
		return now.AddDate(value, 0, 0), nil
	default:
		return time.Time{}, fmt.Errorf("unknown time unit: %s", unit)
	}
}

func (s *MainMenuScreen) queryTransferHistory(ctx context.Context) {
	fmt.Print("\033[H\033[2J")
	fmt.Println("【转账记录查询】")
	fmt.Println()

	// 1. 选择查询方式
	fmt.Println("请选择查询方式：")
	fmt.Println("  1. 按交易哈希查询")
	fmt.Println("  2. 按资源ID查询")
	fmt.Println()
	fmt.Print("请选择（输入数字）: ")

	var choice int
	fmt.Scanf("%d\n", &choice)

	var txID, resourceID string

	switch choice {
	case 1:
		fmt.Print("请输入交易哈希: ")
		fmt.Scanln(&txID)
		if txID == "" {
			fmt.Println("❌ 交易哈希不能为空")
			s.waitForEnter()
			return
		}
	case 2:
		fmt.Print("请输入资源ID（ContentHash）: ")
		fmt.Scanln(&resourceID)
		if resourceID == "" {
			fmt.Println("❌ 资源ID不能为空")
			s.waitForEnter()
			return
		}
	default:
		fmt.Println("❌ 无效选择")
		s.waitForEnter()
		return
	}

	// 2. 输入分页参数
	fmt.Print("请输入每页数量（默认10，直接回车使用默认）: ")
	var limitInput string
	fmt.Scanln(&limitInput)
	limit := 10
	if limitInput != "" {
		if _, err := fmt.Sscanf(limitInput, "%d", &limit); err != nil || limit <= 0 {
			limit = 10
		}
	}

	fmt.Print("请输入偏移量（默认0，直接回车使用默认）: ")
	var offsetInput string
	fmt.Scanln(&offsetInput)
	offset := 0
	if offsetInput != "" {
		if _, err := fmt.Sscanf(offsetInput, "%d", &offset); err != nil || offset < 0 {
			offset = 0
		}
	}

	// 3. 执行查询
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("正在查询交易历史...")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()

	transactions, err := s.transport.GetTransactionHistory(ctx, txID, resourceID, limit, offset)
	if err != nil {
		fmt.Printf("❌ 查询失败: %v\n", err)
		s.waitForEnter()
		return
	}

	// 4. 显示结果
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	if len(transactions) == 0 {
		fmt.Println("未找到相关交易")
	} else {
		fmt.Printf("找到 %d 笔交易：\n", len(transactions))
		fmt.Println("═══════════════════════════════════════════════════════════════")
		for i, tx := range transactions {
			fmt.Printf("\n交易 %d:\n", i+1)
			fmt.Printf("  哈希: %s\n", tx.Hash)
			fmt.Printf("  发送方: %s\n", tx.From)
			fmt.Printf("  接收方: %s\n", tx.To)
			fmt.Printf("  金额: %s\n", tx.Value)
			fmt.Printf("  手续费: %s\n", tx.Fee)
			fmt.Printf("  状态: %s\n", tx.Status)
			if tx.BlockHeight > 0 {
				fmt.Printf("  区块高度: %d\n", tx.BlockHeight)
				fmt.Printf("  区块哈希: %s\n", tx.BlockHash)
			} else {
				fmt.Println("  状态: 待确认")
			}
			if !tx.Timestamp.IsZero() {
				fmt.Printf("  时间: %s\n", tx.Timestamp.Format("2006-01-02 15:04:05"))
			}
		}
	}
	fmt.Println()

	s.waitForEnter()
}

// ====== 挖矿功能实现 ======

func (s *MainMenuScreen) showMiningStatus(ctx context.Context) {
	fmt.Println("\n【挖矿状态】")
	status, err := s.miningService.GetMiningStatus(ctx)
	if err != nil {
		fmt.Printf("查询失败: %v\n", err)
		s.waitForEnter()
		return
	}

	fmt.Printf("正在挖矿: %v\n", status.IsMining)
	fmt.Printf("算力: %.2f H/s\n", status.HashRate)
	fmt.Printf("矿工地址: %s\n", status.MinerAddress)
	fmt.Printf("已挖区块: %d\n", status.BlocksMined)
	fmt.Printf("当前高度: %d\n", status.CurrentHeight)
	s.waitForEnter()
}

func (s *MainMenuScreen) startMining(ctx context.Context) {
	fmt.Println("\n【启动挖矿】")
	fmt.Print("请输入矿工地址: ")
	var address string
	fmt.Scanln(&address)

	result, err := s.miningService.StartMining(ctx, &mining.StartMiningRequest{
		MinerAddress: address,
		Threads:      1,
	})
	if err != nil {
		fmt.Printf("启动失败: %v\n", err)
		s.waitForEnter()
		return
	}

	fmt.Printf("✅ %s\n", result.Message)
	s.waitForEnter()
}

func (s *MainMenuScreen) stopMining(ctx context.Context) {
	fmt.Println("\n【停止挖矿】")
	result, err := s.miningService.StopMining(ctx)
	if err != nil {
		fmt.Printf("停止失败: %v\n", err)
		s.waitForEnter()
		return
	}

	fmt.Printf("✅ %s\n", result.Message)
	fmt.Printf("已挖区块: %d\n", result.BlocksMined)
	fmt.Printf("总奖励: %s\n", result.TotalRewards)
	s.waitForEnter()
}

func (s *MainMenuScreen) showHashrate(ctx context.Context) {
	fmt.Println("\n【查看算力】")
	hashrate, err := s.miningService.GetHashRate(ctx)
	if err != nil {
		fmt.Printf("查询失败: %v\n", err)
		s.waitForEnter()
		return
	}

	fmt.Printf("当前算力: %.2f H/s\n", hashrate)
	s.waitForEnter()
}

func (s *MainMenuScreen) queryMiningRewards(ctx context.Context) {
	fmt.Println("\n【查询挖矿奖励】")
	fmt.Print("请输入矿工地址: ")
	var address string
	fmt.Scanln(&address)

	rewards, err := s.miningService.GetPendingRewards(ctx, address)
	if err != nil {
		fmt.Printf("查询失败: %v\n", err)
		s.waitForEnter()
		return
	}

	fmt.Printf("待领取奖励: %s\n", rewards)
	s.waitForEnter()
}

// ====== 资源管理功能 ======

func (s *MainMenuScreen) deployResource(ctx context.Context) {
	fmt.Print("\033[H\033[2J")
	fmt.Println("【部署资源】")
	fmt.Println()

	// 1. 获取账户列表
	accounts, err := s.walletManager.ListAccounts()
	if err != nil {
		fmt.Printf("❌ 获取账户列表失败: %v\n", err)
		s.waitForEnter()
		return
	}

	if len(accounts) == 0 {
		fmt.Println("❌ 没有可用账户，请先创建账户")
		s.waitForEnter()
		return
	}

	// 显示账户列表
	fmt.Println("可用账户：")
	for i, acc := range accounts {
		fmt.Printf("  %d. %s (标签: %s)\n", i+1, acc.Address, acc.Label)
	}
	fmt.Println()

	// 2. 选择账户
	var accountIndex int
	fmt.Print("请选择部署账户（输入序号，回车使用第一个）: ")
	var input string
	fmt.Scanln(&input)
	if input == "" {
		accountIndex = 0
	} else {
		if _, err := fmt.Sscanf(input, "%d", &accountIndex); err != nil || accountIndex < 1 || accountIndex > len(accounts) {
			fmt.Printf("❌ 无效选择，使用第一个账户\n")
			accountIndex = 1
		}
		accountIndex-- // 转换为索引
	}

	selectedAccount := accounts[accountIndex]
	fmt.Printf("✓ 已选择账户: %s\n", selectedAccount.Address)
	fmt.Println()

	// 3. 输入密码（如果账户未解锁）
	var password string
	var privateKey []byte

	if s.walletManager.IsWalletUnlocked(selectedAccount.Address) {
		fmt.Println("✓ 账户已解锁")
		var err error
		privateKeyHex, err := s.walletManager.ExportPrivateKey(selectedAccount.Address, "")
		if err != nil {
			fmt.Printf("❌ 获取私钥失败: %v\n", err)
			s.waitForEnter()
			return
		}
		// 转换十六进制字符串为字节数组
		privateKey, err = hex.DecodeString(privateKeyHex)
		if err != nil {
			fmt.Printf("❌ 私钥格式错误: %v\n", err)
			s.waitForEnter()
			return
		}
	} else {
		fmt.Print("请输入账户密码: ")
		fmt.Scanln(&password)
		if password == "" {
			fmt.Println("❌ 密码不能为空")
			s.waitForEnter()
			return
		}

		if err := s.walletManager.UnlockWallet(selectedAccount.Address, password); err != nil {
			fmt.Printf("❌ 解锁失败: %v\n", err)
			s.waitForEnter()
			return
		}

		privateKeyHex, err := s.walletManager.ExportPrivateKey(selectedAccount.Address, password)
		if err != nil {
			fmt.Printf("❌ 获取私钥失败: %v\n", err)
			s.waitForEnter()
			return
		}
		privateKey, err = hex.DecodeString(privateKeyHex)
		if err != nil {
			fmt.Printf("❌ 私钥格式错误: %v\n", err)
			s.waitForEnter()
			return
		}
	}

	defer func() {
		// 安全清零私钥
		for i := range privateKey {
			privateKey[i] = 0
		}
	}()

	// 4. 输入文件路径
	fmt.Print("请输入资源文件路径: ")
	var filePath string
	fmt.Scanln(&filePath)
	if filePath == "" {
		fmt.Println("❌ 文件路径不能为空")
		s.waitForEnter()
		return
	}

	// 5. 输入资源名称（可选）
	fmt.Print("请输入资源名称（可选，直接回车跳过）: ")
	var resourceName string
	fmt.Scanln(&resourceName)

	// 6. 输入资源类型（可选）
	fmt.Print("请输入资源类型（可选，直接回车跳过）: ")
	var resourceType string
	fmt.Scanln(&resourceType)

	// 7. 输入备注（可选）
	fmt.Print("请输入备注（可选，直接回车跳过）: ")
	var memo string
	fmt.Scanln(&memo)

	// 8. 执行部署
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("开始部署资源...")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()

	deployReq := &resource.DeployRequest{
		Deployer:     selectedAccount.Address,
		FilePath:     filePath,
		ResourceName: resourceName,
		ResourceType: resourceType,
		Memo:         memo,
		PrivateKey:   privateKey,
	}

	result, err := s.resourceService.DeployResource(ctx, deployReq)
	if err != nil {
		fmt.Printf("❌ 部署失败: %v\n", err)
		s.waitForEnter()
		return
	}

	// 9. 显示结果
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("✅ 资源部署成功！")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("交易哈希: %s\n", result.TxHash)
	fmt.Printf("资源地址: %s\n", result.ResourceAddress)
	fmt.Printf("手续费: %s\n", result.Fee)
	if result.BlockHeight > 0 {
		fmt.Printf("区块高度: %d\n", result.BlockHeight)
	} else {
		fmt.Println("状态: 待确认（交易已提交到网络，等待打包）")
	}
	fmt.Println()

	s.waitForEnter()
}

func (s *MainMenuScreen) fetchResource(ctx context.Context) {
	fmt.Print("\033[H\033[2J")
	fmt.Println("【获取资源】")
	fmt.Println()

	// 1. 输入资源地址
	fmt.Print("请输入资源地址: ")
	var resourceAddress string
	fmt.Scanln(&resourceAddress)
	if resourceAddress == "" {
		fmt.Println("❌ 资源地址不能为空")
		s.waitForEnter()
		return
	}

	// 2. 执行获取
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("正在获取资源...")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()

	fetchReq := &resource.FetchRequest{
		ResourceAddress: resourceAddress,
	}

	result, err := s.resourceService.FetchResource(ctx, fetchReq)
	if err != nil {
		fmt.Printf("❌ 获取失败: %v\n", err)
		s.waitForEnter()
		return
	}

	// 3. 显示结果
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	if result.Success {
		fmt.Println("✅ 资源获取成功！")
		fmt.Println("═══════════════════════════════════════════════════════════════")
		if result.ResourceName != "" {
			fmt.Printf("资源名称: %s\n", result.ResourceName)
		}
		if result.ResourceType != "" {
			fmt.Printf("资源类型: %s\n", result.ResourceType)
		}
		fmt.Printf("数据大小: %d 字节\n", len(result.Data))
		fmt.Printf("消息: %s\n", result.Message)
	} else {
		fmt.Println("❌ 资源获取失败")
		fmt.Println("═══════════════════════════════════════════════════════════════")
		fmt.Printf("消息: %s\n", result.Message)
	}
	fmt.Println()

	s.waitForEnter()
}

func (s *MainMenuScreen) queryResourceList(ctx context.Context) {
	fmt.Println("\n【查询资源列表】")
	fmt.Println("功能开发中...")
	s.waitForEnter()
}

// ====== 合约管理功能 ======

func (s *MainMenuScreen) deployContract(ctx context.Context) {
	if err := s.contractFlow.ShowDeployContract(ctx); err != nil {
		fmt.Printf("\n部署合约失败: %v\n", err)
	}
	s.waitForEnter()
}

func (s *MainMenuScreen) callContract(ctx context.Context) {
	if err := s.contractFlow.ShowCallContract(ctx); err != nil {
		fmt.Printf("\n调用合约失败: %v\n", err)
	}
	s.waitForEnter()
}

func (s *MainMenuScreen) queryContractStatus(ctx context.Context) {
	if err := s.contractFlow.ShowQueryContract(ctx); err != nil {
		fmt.Printf("\n查询合约失败: %v\n", err)
	}
	s.waitForEnter()
}

// ====== 区块信息功能 ======

func (s *MainMenuScreen) queryChainInfo(ctx context.Context) {
	fmt.Print("\033[H\033[2J")
	fmt.Println("【查询链信息】")
	fmt.Println()

	// 获取链ID
	chainID, err := s.transport.ChainID(ctx)
	if err != nil {
		fmt.Printf("❌ 查询链ID失败: %v\n", err)
		fmt.Println()
		fmt.Println("💡 可能的原因：")
		fmt.Println("  - 节点未就绪或网络连接失败")
		fmt.Println("  - 链尚未初始化（未挖出创世块）")
		fmt.Println("  - JSON-RPC 服务未启动")
		s.waitForEnter()
		return
	}

	// 获取最新区块高度
	height, err := s.transport.BlockNumber(ctx)
	if err != nil {
		fmt.Printf("❌ 查询区块高度失败: %v\n", err)
		fmt.Println()
		fmt.Println("💡 可能的原因：")
		fmt.Println("  - 节点未就绪或网络连接失败")
		fmt.Println("  - 链尚未初始化（未挖出创世块）")
		fmt.Println("  - JSON-RPC 服务未启动")
		s.waitForEnter()
		return
	}

	// 获取同步状态
	syncStatus, err := s.transport.Syncing(ctx)
	if err != nil {
		fmt.Printf("⚠️ 查询同步状态失败: %v（继续显示其他信息）\n", err)
		fmt.Println()
	}

	fmt.Printf("链ID: %s\n", chainID)
	fmt.Printf("当前高度: %d\n", height)
	if syncStatus.Syncing {
		progress := float64(syncStatus.CurrentBlock-syncStatus.StartingBlock) /
			float64(syncStatus.HighestBlock-syncStatus.StartingBlock) * 100
		fmt.Printf("同步状态: 同步中 (%.2f%%)\n", progress)
		fmt.Printf("  当前区块: %d / %d\n", syncStatus.CurrentBlock, syncStatus.HighestBlock)
	} else {
		fmt.Println("同步状态: 已同步")
	}

	s.waitForEnter()
}

func (s *MainMenuScreen) queryBlockInfo(ctx context.Context) {
	fmt.Print("\033[H\033[2J")
	fmt.Println("【查询区块详情】")
	fmt.Println()

	// 先显示当前链尖信息
	currentHeight, heightErr := s.transport.BlockNumber(ctx)
	if heightErr == nil {
		fmt.Printf("📊 当前链尖高度: %d\n", currentHeight)
		fmt.Println()
	}

	fmt.Print("请输入区块高度: ")
	var height uint64
	fmt.Scanf("%d\n", &height)

	// 获取区块信息
	block, err := s.transport.GetBlockByHeight(ctx, height, false, nil)
	if err != nil {
		fmt.Printf("查询失败: %v\n", err)
		s.waitForEnter()
		return
	}

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("区块详情")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()

	fmt.Printf("区块哈希: %s\n", block.Hash)
	fmt.Printf("父区块哈希: %s\n", block.ParentHash)
	fmt.Printf("高度: %d", block.Height)
	// 标注是否为最新区块
	if heightErr == nil && height == currentHeight {
		fmt.Printf(" ⭐ (最新区块)")
	}
	fmt.Println()
	fmt.Printf("时间戳: %s\n", block.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("交易数: %d\n", block.TxCount)
	if block.Miner != "" {
		fmt.Printf("矿工: %s\n", block.Miner)
	}
	if block.Difficulty != "" {
		fmt.Printf("难度: %s\n", block.Difficulty)
	}
	fmt.Printf("状态根: %s\n", block.StateRoot)

	// 显示交易哈希列表
	if len(block.TxHashes) > 0 {
		fmt.Println()
		fmt.Println("═══════════════════════════════════════════════════════════════")
		fmt.Println("交易哈希列表")
		fmt.Println("═══════════════════════════════════════════════════════════════")
		fmt.Println()
		maxShow := 20 // 最多显示20个交易哈希
		if len(block.TxHashes) < maxShow {
			maxShow = len(block.TxHashes)
		}
		for i := 0; i < maxShow; i++ {
			fmt.Printf("  %d. %s\n", i+1, block.TxHashes[i])
		}
		if len(block.TxHashes) > maxShow {
			fmt.Printf("  ... 还有 %d 笔交易未显示\n", len(block.TxHashes)-maxShow)
		}
		fmt.Println()
		fmt.Println("💡 提示: 可以使用交易哈希查询交易详情")
	} else if block.TxCount > 0 {
		// 如果 TxHashes 为空但 TxCount > 0，说明可能有交易但哈希未返回
		fmt.Println()
		fmt.Println("⚠️  注意: 区块包含交易，但交易哈希列表为空")
	}

	fmt.Println()
	s.waitForEnter()
}

func (s *MainMenuScreen) queryTxInfo(ctx context.Context) {
	fmt.Print("\033[H\033[2J")
	fmt.Println("【查询交易详情】")
	fmt.Println()
	fmt.Print("请输入交易哈希: ")
	var txHash string
	fmt.Scanln(&txHash)

	fmt.Println()
	fmt.Println("正在查询交易...")

	tx, err := s.transport.GetTransaction(ctx, txHash)
	if err != nil {
		fmt.Printf("❌ 查询失败: %v\n", err)
		s.waitForEnter()
		return
	}

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("                        交易详情")
	fmt.Println("═══════════════════════════════════════════════════════════════")

	// 基础信息
	fmt.Println()
	fmt.Println("📋 基础信息")
	fmt.Println("───────────────────────────────────────────────────────────────")
	fmt.Printf("  交易哈希:   %s\n", tx.Hash)
	fmt.Printf("  版本:       %d\n", tx.Version)
	fmt.Printf("  Nonce:      %d\n", tx.Nonce)
	if !tx.Timestamp.IsZero() {
		fmt.Printf("  时间戳:     %s\n", tx.Timestamp.Format("2006-01-02 15:04:05"))
	}
	if tx.ChainID != "" {
		// 尝试将 base64 编码的 chain_id 转换为十六进制格式
		chainIDDisplay := tx.ChainID
		if decoded, err := base64.StdEncoding.DecodeString(tx.ChainID); err == nil && len(decoded) > 0 {
			// 移除前导零字节
			trimmed := decoded
			for len(trimmed) > 1 && trimmed[0] == 0 {
				trimmed = trimmed[1:]
			}
			chainIDDisplay = "0x" + hex.EncodeToString(trimmed)
		}
		fmt.Printf("  链 ID:      %s\n", chainIDDisplay)
	}
	if tx.Status != "" {
		fmt.Printf("  状态:       %s\n", tx.Status)
	}

	// 区块信息
	fmt.Println()
	fmt.Println("📦 区块信息")
	fmt.Println("───────────────────────────────────────────────────────────────")
	fmt.Printf("  区块高度:   #%d\n", tx.BlockHeight)
	if tx.BlockHash != "" {
		fmt.Printf("  区块哈希:   %s\n", tx.BlockHash)
	}
	fmt.Printf("  交易索引:   %d\n", tx.TxIndex)

	// 输入列表
	if len(tx.Inputs) > 0 {
		fmt.Println()
		fmt.Printf("📥 交易输入 (%d 个)\n", len(tx.Inputs))
		fmt.Println("───────────────────────────────────────────────────────────────")
		for i, input := range tx.Inputs {
			fmt.Printf("  [%d] ", i)
			if input.PreviousOutput != nil {
				fmt.Printf("引用: %s:%d", truncateHash(input.PreviousOutput.TxID), input.PreviousOutput.OutputIndex)
			}
			if input.IsReferenceOnly {
				fmt.Printf(" (只读引用)")
			} else {
				fmt.Printf(" (消费)")
			}
			if input.UnlockingProofType != "" && input.UnlockingProofType != "unknown" {
				fmt.Printf(" [%s]", input.UnlockingProofType)
			}
			fmt.Println()
		}
	} else {
		fmt.Println()
		fmt.Println("📥 交易输入: 无 (可能是 Coinbase 交易)")
	}

	// 输出列表
	if len(tx.Outputs) > 0 {
		fmt.Println()
		fmt.Printf("📤 交易输出 (%d 个)\n", len(tx.Outputs))
		fmt.Println("───────────────────────────────────────────────────────────────")
		for i, output := range tx.Outputs {
			fmt.Printf("  [%d] ", i)

			switch output.OutputType {
			case "asset":
				fmt.Printf("💰 资产输出")
				if output.Asset != nil {
					if output.Asset.NativeCoin != nil && output.Asset.NativeCoin.Amount != "" {
						fmt.Printf(": %s (原生币)", output.Asset.NativeCoin.Amount)
					} else if output.Asset.ContractToken != nil {
						fmt.Printf(": %s (合约代币)", output.Asset.ContractToken.Amount)
					}
				}
			case "resource":
				fmt.Printf("📦 资源输出")
				if output.Resource != nil {
					if output.Resource.Category != "" {
						fmt.Printf(" [%s", output.Resource.Category)
						if output.Resource.ExecutableType != "" {
							fmt.Printf("/%s", output.Resource.ExecutableType)
						}
						fmt.Printf("]")
					}
					if output.Resource.ContentHash != "" {
						fmt.Printf("\n      内容哈希: %s", truncateHash(output.Resource.ContentHash))
					}
					if output.Resource.MimeType != "" {
						fmt.Printf("\n      MIME类型: %s", output.Resource.MimeType)
					}
				}
			case "state":
				fmt.Printf("📊 状态输出")
				if output.State != nil {
					if output.State.StateID != "" {
						fmt.Printf("\n      状态ID: %s", truncateHash(output.State.StateID))
					}
					if output.State.StateVersion > 0 {
						fmt.Printf(" (v%d)", output.State.StateVersion)
					}
					if output.State.ExecutionResultHash != "" {
						fmt.Printf("\n      执行结果: %s", truncateHash(output.State.ExecutionResultHash))
					}
				}
			default:
				fmt.Printf("❓ 未知类型")
			}

			if output.Owner != "" {
				fmt.Printf("\n      所有者: %s", formatOwnerAddress(output.Owner))
			}
			fmt.Println()
		}
	}

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	s.waitForEnter()
}

// truncateHash 截断哈希显示（保留前后各 8 个字符）
func truncateHash(hash string) string {
	if len(hash) <= 20 {
		return hash
	}
	return hash[:8] + "..." + hash[len(hash)-8:]
}

// formatOwnerAddress 格式化所有者地址（将 base64 转换为十六进制）
func formatOwnerAddress(owner string) string {
	// 尝试将 base64 编码的地址转换为十六进制格式
	if decoded, err := base64.StdEncoding.DecodeString(owner); err == nil && len(decoded) > 0 {
		hexAddr := "0x" + hex.EncodeToString(decoded)
		return truncateHash(hexAddr)
	}
	// 如果已经是十六进制格式，直接截断显示
	return truncateHash(owner)
}

func (s *MainMenuScreen) queryTxPoolStatus(ctx context.Context) {
	fmt.Println("\n【查询交易池状态】")

	status, err := s.transport.TxPoolStatus(ctx)
	if err != nil {
		fmt.Printf("查询失败: %v\n", err)
		s.waitForEnter()
		return
	}

	fmt.Printf("待处理交易: %d\n", status.Pending)
	fmt.Printf("已排队交易: %d\n", status.Queued)
	fmt.Printf("总计: %d\n", status.Total)

	s.waitForEnter()
}

// ====== 系统功能 ======

func (s *MainMenuScreen) showNodeStatus(ctx context.Context) {
	fmt.Println("\n【节点状态】")

	// 获取链ID
	chainID, err := s.transport.ChainID(ctx)
	if err != nil {
		fmt.Printf("查询失败: %v\n", err)
		s.waitForEnter()
		return
	}

	// 获取最新区块高度
	height, err := s.transport.BlockNumber(ctx)
	if err != nil {
		fmt.Printf("查询失败: %v\n", err)
		s.waitForEnter()
		return
	}

	// 获取同步状态
	syncStatus, err := s.transport.Syncing(ctx)
	if err != nil {
		fmt.Printf("查询失败: %v\n", err)
		s.waitForEnter()
		return
	}

	fmt.Printf("链ID: %s\n", chainID)
	fmt.Printf("当前高度: %d\n", height)

	if syncStatus.Syncing {
		fmt.Println("节点状态: 同步中")
	} else {
		fmt.Println("节点状态: 运行中（已同步）")
	}

	// 尝试获取最新区块以检测节点活性
	block, err := s.transport.GetBlockByHeight(ctx, height, false, nil)
	if err == nil {
		fmt.Printf("最新区块时间: %s\n", block.Timestamp.Format("2006-01-02 15:04:05"))
	}

	s.waitForEnter()
}

func (s *MainMenuScreen) showNetworkInfo(ctx context.Context) {
	fmt.Println("\n【网络连接】")
	fmt.Println("功能开发中...")
	s.waitForEnter()
}

func (s *MainMenuScreen) showSystemSettings(ctx context.Context) {
	fmt.Println("\n【系统设置】")
	fmt.Println("功能开发中...")
	s.waitForEnter()
}

// ============================================================================
// 适配器层 - 将 core 层服务适配为 flows 层接口
// ============================================================================

// ContractServiceAdapter 合约服务适配器
// 将 transport.Client 和 walletService 适配为 flows.ContractService 接口
type ContractServiceAdapter struct {
	transport     transport.Client
	walletService flows.WalletService
}

func NewContractServiceAdapter(transportClient transport.Client, walletService flows.WalletService) *ContractServiceAdapter {
	return &ContractServiceAdapter{
		transport:     transportClient,
		walletService: walletService,
	}
}

func (a *ContractServiceAdapter) DeployContract(ctx context.Context, req *flows.ContractDeployRequest) (*flows.ContractDeployResult, error) {
	// 1. 获取私钥
	privateKeyHex, err := a.walletService.ExportPrivateKey(ctx, req.WalletName, req.Password)
	if err != nil {
		return &flows.ContractDeployResult{
			Success: false,
			Message: fmt.Sprintf("获取私钥失败: %v", err),
		}, fmt.Errorf("获取私钥失败: %w", err)
	}

	// 2. 读取WASM文件
	wasmBytes, err := os.ReadFile(req.FilePath)
	if err != nil {
		return &flows.ContractDeployResult{
			Success: false,
			Message: fmt.Sprintf("读取WASM文件失败: %v", err),
		}, fmt.Errorf("读取WASM文件失败: %w", err)
	}

	// 3. Base64编码WASM内容
	wasmBase64 := base64.StdEncoding.EncodeToString(wasmBytes)

	// 4. 确定ABI版本
	abiVersion := "v1"
	if req.Config != nil && req.Config.AbiVersion != "" {
		abiVersion = req.Config.AbiVersion
	}

	// 5. 调用transport.DeployContract
	transportReq := &transport.DeployContractRequest{
		PrivateKey:        privateKeyHex,
		WasmContentBase64: wasmBase64,
		AbiVersion:        abiVersion,
		Name:              req.Name,
		Description:       req.Description,
	}

	transportResult, err := a.transport.DeployContract(ctx, transportReq)
	if err != nil {
		return &flows.ContractDeployResult{
			Success: false,
			Message: fmt.Sprintf("部署失败: %v", err),
		}, fmt.Errorf("部署失败: %w", err)
	}

	return &flows.ContractDeployResult{
		ContentHash: transportResult.ContentHash,
		TxHash:      transportResult.TxHash,
		Success:     transportResult.Success,
		Message:     transportResult.Message,
	}, nil
}

func (a *ContractServiceAdapter) CallContract(ctx context.Context, req *flows.ContractCallRequest) (*flows.ContractCallResult, error) {
	// 1. 获取私钥
	privateKeyHex, err := a.walletService.ExportPrivateKey(ctx, req.WalletName, req.Password)
	if err != nil {
		return &flows.ContractCallResult{
			Success: false,
			Message: fmt.Sprintf("获取私钥失败: %v", err),
		}, fmt.Errorf("获取私钥失败: %w", err)
	}

	// 2. 转换ContentHash为十六进制字符串
	contentHashHex := hex.EncodeToString(req.ContentHash)

	// 3. Base64编码Payload（如果有）
	payloadBase64 := ""
	if len(req.Payload) > 0 {
		payloadBase64 = base64.StdEncoding.EncodeToString(req.Payload)
	}

	// 4. 调用transport.CallContract
	transportReq := &transport.CallContractRequest{
		PrivateKey:    privateKeyHex,
		ContentHash:   contentHashHex,
		Method:        req.Method,
		Params:        req.Params,
		PayloadBase64: payloadBase64,
	}

	transportResult, err := a.transport.CallContract(ctx, transportReq)
	if err != nil {
		return &flows.ContractCallResult{
			Success: false,
			Message: fmt.Sprintf("调用失败: %v", err),
		}, fmt.Errorf("调用失败: %w", err)
	}

	// 5. 转换返回数据
	returnData := []byte{}
	if transportResult.ReturnData != "" {
		returnData, err = base64.StdEncoding.DecodeString(transportResult.ReturnData)
		if err != nil {
			// 如果解码失败，保留原始字符串
			returnData = []byte(transportResult.ReturnData)
		}
	}

	// 6. 转换事件列表
	events := make([]flows.EventInfo, 0, len(transportResult.Events))
	for _, evt := range transportResult.Events {
		eventInfo := flows.EventInfo{
			Type:      "",
			Timestamp: 0,
			Data:      evt,
		}
		if t, ok := evt["type"].(string); ok {
			eventInfo.Type = t
		}
		if ts, ok := evt["timestamp"].(float64); ok {
			eventInfo.Timestamp = int64(ts)
		}
		events = append(events, eventInfo)
	}

	return &flows.ContractCallResult{
		TxHash:     transportResult.TxHash,
		Results:    transportResult.Results,
		ReturnData: returnData,
		Events:     events,
		Success:    transportResult.Success,
		Message:    transportResult.Message,
	}, nil
}

func (a *ContractServiceAdapter) QueryContract(ctx context.Context, req *flows.ContractQueryRequest) (*flows.ContractQueryResult, error) {
	// 1. 转换ContentHash（移除0x前缀如果存在）
	contentHash := strings.TrimPrefix(req.ContentHash, "0x")
	contentHash = strings.TrimPrefix(contentHash, "0X")

	// 2. 如果没有提供方法名，则只查询合约元数据
	if req.Method == "" {
		metadata, err := a.transport.GetContract(ctx, contentHash)
		if err != nil {
			return &flows.ContractQueryResult{
				Success: false,
				Message: fmt.Sprintf("查询失败: %v", err),
			}, fmt.Errorf("查询失败: %w", err)
		}

		return &flows.ContractQueryResult{
			Results:    []uint64{},
			ReturnData: []byte{},
			Success:    true,
			Message:    "查询成功",
			Metadata: map[string]interface{}{
				"content_hash":       metadata.ContentHash,
				"name":               metadata.Name,
				"version":            metadata.Version,
				"abi_version":        metadata.AbiVersion,
				"exported_functions": metadata.ExportedFunctions,
				"description":        metadata.Description,
				"size":               metadata.Size,
				"mime_type":          metadata.MimeType,
				"creation_time":      metadata.CreationTime,
				"owner":              metadata.Owner,
			},
		}, nil
	}

	// 3. 组装 callData 用于 wes_call（只读调用）
	callSpec := map[string]interface{}{
		"method": req.Method,
		"params": req.Params,
	}
	callSpecJSON, err := json.Marshal(callSpec)
	if err != nil {
		return &flows.ContractQueryResult{
			Success: false,
			Message: fmt.Sprintf("组装调用参数失败: %v", err),
		}, fmt.Errorf("组装调用参数失败: %w", err)
	}

	callData := map[string]interface{}{
		"to":   contentHash, // wes_call 要求 to 为 content_hash (32字节hex)
		"data": string(callSpecJSON),
	}

	// 4. 调用 transport.Call（wes_call）
	callReq := &transport.CallRequest{
		To:   contentHash,
		Data: string(callSpecJSON),
	}
	callResult, err := a.transport.Call(ctx, callReq, nil)
	if err != nil {
		return &flows.ContractQueryResult{
			Success: false,
			Message: fmt.Sprintf("合约调用失败: %v", err),
		}, fmt.Errorf("合约调用失败: %w", err)
	}

	// 5. 如果 transport.Call 返回的是 CallResult，需要进一步调用 CallRaw 获取完整结果
	// 因为节点 wes_call 返回的结构包含 return_values, return_data, events 等
	var resultMap map[string]interface{}
	if callResult != nil && callResult.Success {
		// transport.Call 可能只返回了部分信息，我们需要直接调用 CallRaw 获取完整结果
		rawResult, rawErr := a.transport.CallRaw(ctx, "wes_call", []interface{}{callData})
		if rawErr == nil {
			if rawMap, ok := rawResult.(map[string]interface{}); ok {
				resultMap = rawMap
			}
		}
	}

	if resultMap == nil {
		// 回退：直接使用 CallRaw
		rawResult, rawErr := a.transport.CallRaw(ctx, "wes_call", []interface{}{callData})
		if rawErr != nil {
			return &flows.ContractQueryResult{
				Success: false,
				Message: fmt.Sprintf("合约调用失败: %v", rawErr),
			}, fmt.Errorf("合约调用失败: %w", rawErr)
		}
		if rawMap, ok := rawResult.(map[string]interface{}); ok {
			resultMap = rawMap
		} else {
			return &flows.ContractQueryResult{
				Success: false,
				Message: fmt.Sprintf("返回格式错误: %T", rawResult),
			}, fmt.Errorf("返回格式错误: %T", rawResult)
		}
	}

	// 6. 解析返回结果
	success := true
	if successVal, ok := resultMap["success"].(bool); ok {
		success = successVal
	}

	// 提取 return_values (u64 数组)
	var results []uint64
	if returnValues, ok := resultMap["return_values"].([]interface{}); ok {
		results = make([]uint64, 0, len(returnValues))
		for _, v := range returnValues {
			switch val := v.(type) {
			case float64:
				results = append(results, uint64(val))
			case uint64:
				results = append(results, val)
			case int64:
				results = append(results, uint64(val))
			}
		}
	}

	// 提取 return_data (hex 字符串)
	var returnData []byte
	if returnDataHex, ok := resultMap["return_data"].(string); ok {
		returnDataHex = strings.TrimPrefix(strings.TrimPrefix(returnDataHex, "0x"), "0X")
		if data, err := hex.DecodeString(returnDataHex); err == nil {
			returnData = data
		}
	}

	// 提取 events
	var events []map[string]interface{}
	if eventsVal, ok := resultMap["events"].([]interface{}); ok {
		events = make([]map[string]interface{}, 0, len(eventsVal))
		for _, evt := range eventsVal {
			if evtMap, ok := evt.(map[string]interface{}); ok {
				events = append(events, evtMap)
			}
		}
	}

	// 构建元数据（包含所有返回字段）
	metadata := make(map[string]interface{})
	for k, v := range resultMap {
		metadata[k] = v
	}

	message := "查询成功"
	if !success {
		message = "查询失败"
		if msgVal, ok := resultMap["error"].(string); ok {
			message = msgVal
		}
	}

	return &flows.ContractQueryResult{
		Results:    results,
		ReturnData: returnData,
		Success:    success,
		Message:    message,
		Metadata:   metadata,
	}, nil
}

// WalletServiceAdapter 钱包服务适配器
// 将 wallet.AccountManager 适配为 flows.WalletService 接口
type WalletServiceAdapter struct {
	manager *wallet.AccountManager
}

func NewWalletServiceAdapter(manager *wallet.AccountManager) *WalletServiceAdapter {
	return &WalletServiceAdapter{manager: manager}
}

func (a *WalletServiceAdapter) ListWallets(ctx context.Context) ([]flows.WalletInfo, error) {
	accounts, err := a.manager.ListAccounts()
	if err != nil {
		return nil, err
	}

	wallets := make([]flows.WalletInfo, len(accounts))
	for i, acc := range accounts {
		wallets[i] = flows.WalletInfo{
			Name:      acc.Label,
			Address:   acc.Address,
			IsDefault: false, // AccountManager 暂不支持默认账户标记
		}
	}

	return wallets, nil
}

func (a *WalletServiceAdapter) CreateWallet(ctx context.Context, name, password string) (*flows.WalletInfo, error) {
	// 生成助记词（24 个单词，256 bits 熵）
	mnemonic, err := a.manager.GenerateNewMnemonic(wallet.Mnemonic24Words)
	if err != nil {
		return nil, fmt.Errorf("生成助记词失败: %w", err)
	}

	// 从助记词创建账户
	account, err := a.manager.CreateAccountFromMnemonic(mnemonic, "", password, name)
	if err != nil {
		return nil, fmt.Errorf("创建钱包失败: %w", err)
	}

	return &flows.WalletInfo{
		Name:      name,
		Address:   account.Address,
		IsDefault: false,
		Mnemonic:  mnemonic, // 返回助记词供用户备份
	}, nil
}

func (a *WalletServiceAdapter) ImportWallet(ctx context.Context, name, privateKey, password string) (*flows.WalletInfo, error) {
	// AccountManager.ImportPrivateKey(privateKeyHex, password, label)
	account, err := a.manager.ImportPrivateKey(privateKey, password, name)
	if err != nil {
		return nil, err
	}

	return &flows.WalletInfo{
		Name:      name,
		Address:   account.Address,
		IsDefault: false,
	}, nil
}

func (a *WalletServiceAdapter) DeleteWallet(ctx context.Context, name string) error {
	// DeleteAccount 需要地址，但我们只有name（label）
	// 需要先通过ListAccounts找到对应的地址
	accounts, err := a.manager.ListAccounts()
	if err != nil {
		return err
	}

	for _, acc := range accounts {
		if acc.Label == name {
			return a.manager.DeleteAccount(acc.Address)
		}
	}

	return fmt.Errorf("钱包 %s 不存在", name)
}

func (a *WalletServiceAdapter) ExportPrivateKey(ctx context.Context, name, password string) (string, error) {
	// ExportPrivateKey 需要地址，但我们只有name（label）
	accounts, err := a.manager.ListAccounts()
	if err != nil {
		return "", err
	}

	for _, acc := range accounts {
		if acc.Label == name {
			return a.manager.ExportPrivateKey(acc.Address, password)
		}
	}

	return "", fmt.Errorf("钱包 %s 不存在", name)
}

func (a *WalletServiceAdapter) UnlockWallet(ctx context.Context, name, password string) error {
	// UnlockWallet 需要地址
	accounts, err := a.manager.ListAccounts()
	if err != nil {
		return err
	}

	for _, acc := range accounts {
		if acc.Label == name {
			return a.manager.UnlockWallet(acc.Address, password)
		}
	}

	return fmt.Errorf("钱包 %s 不存在", name)
}

func (a *WalletServiceAdapter) SetDefaultWallet(ctx context.Context, name string) error {
	// SetDefaultWallet 需要地址
	accounts, err := a.manager.ListAccounts()
	if err != nil {
		return err
	}

	for _, acc := range accounts {
		if acc.Label == name {
			return a.manager.SetDefaultWallet(acc.Address)
		}
	}

	return fmt.Errorf("钱包 %s 不存在", name)
}

func (a *WalletServiceAdapter) ChangePassword(ctx context.Context, name, oldPassword, newPassword string) error {
	// ChangePassword 需要地址
	accounts, err := a.manager.ListAccounts()
	if err != nil {
		return err
	}

	for _, acc := range accounts {
		if acc.Label == name {
			return a.manager.ChangePassword(acc.Address, oldPassword, newPassword)
		}
	}

	return fmt.Errorf("钱包 %s 不存在", name)
}

func (a *WalletServiceAdapter) ValidatePassword(ctx context.Context, name, password string) (bool, error) {
	// ValidatePassword 需要地址
	accounts, err := a.manager.ListAccounts()
	if err != nil {
		return false, err
	}

	for _, acc := range accounts {
		if acc.Label == name {
			return a.manager.ValidatePassword(acc.Address, password)
		}
	}

	return false, fmt.Errorf("钱包 %s 不存在", name)
}
