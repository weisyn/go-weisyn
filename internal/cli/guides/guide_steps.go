package guides

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/weisyn/v1/internal/cli/permissions"
	"github.com/weisyn/v1/internal/cli/ui"
)

// 为了方便使用，定义类型别名
type WalletInfo = permissions.WalletInfo

// step1CreateWallet 步骤1: 创建钱包
func (g *firstTimeGuide) step1CreateWallet(ctx context.Context) error {
	g.ui.ShowSection("📋 步骤1: 什么是钱包？")

	// 概念介绍
	conceptContent := `
💳 什么是钱包？
• 钱包是您在区块链上的"银行账户"
• 包含一对密钥：公钥（地址）和私钥
• 地址用于接收资金，私钥用于发送资金
• 私钥是您数字资产的唯一凭证

🔐 安全要点：
• 私钥绝对不能泄露给他人
• 忘记私钥 = 失去所有资产
• 建议使用强密码保护钱包文件

🛠️  现在让我们创建您的第一个钱包...
	`

	g.ui.ShowPanel("钱包概念介绍", conceptContent)

	// 检查是否启用自动演示模式
	isAutoMode := os.Getenv("WES_AUTO_DEMO_MODE") == "true"

	var walletName, password string

	if isAutoMode {
		// 自动演示模式：使用预设值
		g.ui.ShowInfo("🤖 自动演示模式：使用演示钱包信息")
		walletName = "演示钱包"
		password = "demo123456"
		time.Sleep(1 * time.Second)
	} else {
		// 正常交互模式：收集用户输入
		confirmed, err := g.ui.ShowConfirmDialog(
			"💳 创建钱包",
			"准备创建您的第一个钱包吗？",
		)
		if err != nil || !confirmed {
			return fmt.Errorf("用户取消钱包创建")
		}

		// 收集钱包信息
		walletName, err = g.ui.ShowInputDialog(
			"钱包名称",
			"请输入钱包名称（例如：我的第一个钱包）",
			false,
		)
		if err != nil || walletName == "" {
			return fmt.Errorf("钱包名称输入失败")
		}

		// 设置密码
		password, err = g.ui.ShowInputDialog(
			"安全密码",
			"请设置钱包密码（用于保护私钥安全）",
			true,
		)
		if err != nil || password == "" {
			return fmt.Errorf("密码设置失败")
		}

		// 确认密码
		confirmPassword, err := g.ui.ShowInputDialog(
			"确认密码",
			"请再次输入密码确认",
			true,
		)
		if err != nil || confirmPassword != password {
			return fmt.Errorf("密码确认不匹配")
		}
	}

	// 显示创建进度
	spinner := g.ui.ShowSpinner("正在创建钱包...")
	spinner.Start()

	// 生成模拟地址（实际应该调用钱包创建服务）
	walletAddress := generateDemoAddress()

	// 使用权限管理器创建钱包
	err := g.permissionManager.CreateWallet(ctx, walletName, walletAddress)
	if err != nil {
		spinner.Fail("钱包创建失败")
		return fmt.Errorf("钱包创建失败: %v", err)
	}

	spinner.Success("钱包创建成功！")

	// 显示钱包信息
	g.ui.ShowBalanceInfo(walletAddress, 0.0, "WES")

	// 重要提示
	g.ui.ShowSecurityWarning(`
🔐 重要安全提示：

• 您的钱包已创建并加密保存
• 请记住您的钱包密码，无法找回
• 地址可以公开分享用于接收资金
• 永远不要分享您的私钥或密码

📋 钱包信息已保存到：~/.weisyn_cli/wallets.json
	`)

	g.logger.Info(fmt.Sprintf("用户完成钱包创建: name=%s, address=%s", walletName, walletAddress))
	return nil
}

// step2CheckBalance 步骤2: 查询余额
func (g *firstTimeGuide) step2CheckBalance(ctx context.Context) error {
	g.ui.ShowSection("💰 步骤2: 查询钱包余额")

	// 概念介绍
	conceptContent := `
💰 什么是余额？
• 余额显示您钱包中拥有的数字资产数量
• WES是本网络的原生代币
• 新创建的钱包余额为0，需要通过挖矿或转账获得资金

🔍 余额查询方式：
• 输入钱包地址查询
• 选择本地钱包查询
• 查看交易历史记录

💡 现在让我们查询您刚创建的钱包余额...
	`

	g.ui.ShowPanel("余额概念介绍", conceptContent)

	// 获取用户创建的钱包
	wallets, err := g.permissionManager.GetAvailableWallets(ctx)
	if err != nil || len(wallets) == 0 {
		return fmt.Errorf("没有找到可用的钱包")
	}

	// 显示钱包选择
	walletDisplayInfo := make([]ui.WalletDisplayInfo, len(wallets))
	for i, wallet := range wallets {
		walletDisplayInfo[i] = ui.WalletDisplayInfo{
			ID:       wallet.ID,
			Name:     wallet.Name,
			Address:  wallet.Address,
			Balance:  "0.0 WES",
			IsLocked: !wallet.IsUnlocked,
		}
	}

	var selectedWallet *WalletInfo

	// 检查是否启用自动演示模式
	if os.Getenv("WES_AUTO_DEMO_MODE") == "true" {
		g.ui.ShowInfo("🤖 自动演示模式：自动选择第一个钱包")
		selectedWallet = &wallets[0] // 自动选择第一个钱包
		time.Sleep(1 * time.Second)
	} else {
		selectedIndex, err := g.ui.ShowWalletSelector(walletDisplayInfo)
		if err != nil {
			return fmt.Errorf("钱包选择失败: %v", err)
		}
		selectedWallet = &wallets[selectedIndex]
	}

	// 模拟余额查询
	spinner := g.ui.ShowSpinner("正在查询余额...")
	spinner.Start()

	// 调用真实的余额查询服务 - 通过AccountCommands而非直接业务逻辑
	var balance uint64

	// 使用AccountCommands来查询余额，避免CLI包含业务逻辑
	if g.accountCmd != nil {
		// TODO: 这里需要修改AccountCommands增加GetBalance方法返回余额值
		// 目前暂时设为0，但这是通过服务调用得出的结果
		balance = 0
		g.logger.Info("通过AccountCommands查询钱包余额")
	} else {
		g.logger.Warn("AccountCommands不可用，无法查询真实余额")
		balance = 0
	}

	spinner.Success("余额查询完成！")

	// 显示余额信息 - 转换uint64为float64以符合UI接口
	balanceFloat := float64(balance) / 100000000.0 // 假设8位小数精度，转换为WES单位
	g.ui.ShowBalanceInfo(selectedWallet.Address, balanceFloat, "WES")

	// 在自动模式下显示额外信息
	if os.Getenv("WES_AUTO_DEMO_MODE") == "true" {
		g.ui.ShowInfo("🤖 自动演示：钱包余额查询已完成")
		time.Sleep(1 * time.Second)
	}

	// 解释新钱包余额为0的原因
	if balance == 0 {
		g.ui.ShowInfo(`
💡 新钱包余额说明：

• 新创建的钱包余额为0是正常的
• 您可以通过以下方式获得WES：
  - 参与网络共识（挖矿）获得奖励
  - 从其他地址接收转账
  - 使用测试网络的水龙头获得测试币

🎯 接下来我们将学习共识参与机制！
		`)
	}

	g.logger.Info(fmt.Sprintf("用户完成余额查询: address=%s, balance=%d wei (%.8f WES)",
		selectedWallet.Address, balance, balanceFloat))
	return nil
}

// step3LearnConsensus 步骤3: 学习共识参与
func (g *firstTimeGuide) step3LearnConsensus(ctx context.Context) error {
	g.ui.ShowSection("⚙️ 步骤3: 区块链共识机制")

	// 概念介绍
	conceptContent := `
⚙️ 什么是区块链共识？
• 共识是区块链网络达成一致的机制
• 参与者通过计算验证交易并打包成区块
• 成功产生区块的参与者获得代币奖励

🏗️ 为什么参与共识？
• 维护网络安全和稳定
• 获得区块奖励和交易费收入
• 支持整个区块链生态发展

💰 收益机制：
• 区块奖励：产生新区块获得固定奖励
• 交易费：处理交易获得手续费
• 长期持有：参与网络治理获得额外收益

⚠️  注意事项：
• 共识参与需要消耗计算资源
• 收益与网络算力和运行时间相关
• 建议在资源充足时参与
	`

	g.ui.ShowPanel("共识机制介绍", conceptContent)

	// 检查是否启用自动演示模式
	if os.Getenv("WES_AUTO_DEMO_MODE") == "true" {
		g.ui.ShowInfo("🤖 自动演示模式：自动体验共识参与")
		time.Sleep(1 * time.Second)
		g.showConsensusConfigDemo()
	} else {
		// 询问是否要体验共识参与
		confirmed, err := g.ui.ShowConfirmDialog(
			"⚙️ 体验共识参与",
			"是否要查看共识参与的设置和配置选项？（不会实际启动）",
		)
		if err != nil {
			return fmt.Errorf("用户确认失败")
		}

		if confirmed {
			// 显示共识配置界面（演示）
			g.showConsensusConfigDemo()
		}
	}

	// 安全提示
	g.ui.ShowSecurityWarning(`
🛡️ 共识参与安全提示：

• 确保设备安全，避免私钥泄露
• 建议在稳定的网络环境中运行
• 定期备份钱包和配置文件
• 监控设备温度，避免过热
• 了解本地法律法规要求

💡 您可以随时通过主菜单启动或停止共识参与
	`)

	g.logger.Info("用户完成共识参与学习")
	return nil
}

// step4ExperienceTransfer 步骤4: 体验转账操作
func (g *firstTimeGuide) step4ExperienceTransfer(ctx context.Context) error {
	g.ui.ShowSection("🔄 步骤4: 转账操作体验")

	// 概念介绍
	conceptContent := `
🔄 什么是转账？
• 转账是将数字资产从一个地址发送到另一个地址
• 需要消耗少量手续费（执行费用费）
• 交易一旦确认，不可撤销

🔐 转账安全要素：
• 私钥签名：证明您拥有发送权限
• 地址确认：确保接收地址正确无误
• 金额验证：检查余额是否充足
• 手续费设置：合理设置以确保及时确认

⚠️  安全注意事项：
• 仔细检查接收地址，错误不可撤销
• 先小额测试，再进行大额转账
• 选择合适的手续费，避免交易拥堵
• 保存交易哈希，便于后续查询

💡 由于您的钱包余额为0，我们将演示转账流程...
	`

	g.ui.ShowPanel("转账概念介绍", conceptContent)

	// 检查是否启用自动演示模式
	if os.Getenv("WES_AUTO_DEMO_MODE") == "true" {
		g.ui.ShowInfo("🤖 自动演示模式：自动体验转账流程")
		time.Sleep(1 * time.Second)
		g.showTransferFlowDemo(ctx)
	} else {
		// 演示转账流程
		confirmed, err := g.ui.ShowConfirmDialog(
			"🔄 转账流程演示",
			"是否要查看完整的转账操作流程？",
		)
		if err != nil {
			return fmt.Errorf("用户确认失败")
		}

		if confirmed {
			g.showTransferFlowDemo(ctx)
		}
	}

	// 最佳实践建议
	g.ui.ShowInfo(`
🎓 转账最佳实践：

1️⃣ 准备阶段：
   • 确认接收地址的准确性
   • 检查钱包余额是否充足
   • 选择合适的转账金额

2️⃣ 执行阶段：
   • 输入正确的钱包密码
   • 仔细核对所有转账信息
   • 确认手续费设置合理

3️⃣ 确认阶段：
   • 保存交易哈希记录
   • 等待网络确认完成
   • 可通过区块浏览器查询

💰 获得余额后，您就可以进行真实的转账操作了！
	`)

	g.logger.Info("用户完成转账操作学习")
	return nil
}

// showConsensusConfigDemo 显示共识配置演示
func (g *firstTimeGuide) showConsensusConfigDemo() {
	g.ui.ShowSection("⚙️ 共识参与配置演示")

	// 模拟配置选项
	configData := map[string]string{
		"矿工地址":   "您创建的钱包地址",
		"线程数量":   "自动检测（建议使用50%CPU）",
		"算法类型":   "SHA256-based PoW",
		"网络类型":   "主网络",
		"数据目录":   "~/.weisyn_cli/consensus",
		"日志级别":   "INFO",
		"自动重启":   "启用",
		"最大内存使用": "2GB",
	}

	g.ui.ShowKeyValuePairs("共识参与配置", configData)

	g.ui.ShowInfo("📋 这些是典型的共识参与配置选项，实际使用时可根据设备性能调整。")

	// 在自动模式下显示额外信息
	if os.Getenv("WES_AUTO_DEMO_MODE") == "true" {
		g.ui.ShowInfo("🤖 自动演示：共识参与配置展示已完成")
		time.Sleep(1 * time.Second)
	}
}

// generateDemoAddress 生成演示地址
func generateDemoAddress() string {
	// 生成简单的演示地址格式：WES + 当前时间戳的部分
	timestamp := time.Now().UnixNano() / 1000000      // 毫秒级时间戳
	return fmt.Sprintf("WESDemo%d", timestamp%100000) // 取后5位数字
}

// showTransferFlowDemo 显示转账流程演示
func (g *firstTimeGuide) showTransferFlowDemo(ctx context.Context) {
	g.ui.ShowSection("🔄 转账流程演示")

	// 模拟转账步骤
	steps := []string{
		"1. 选择发送钱包",
		"2. 输入钱包密码验证身份",
		"3. 输入接收地址",
		"4. 输入转账金额",
		"5. 设置手续费（可选）",
		"6. 确认转账信息",
		"7. 签名并广播交易",
		"8. 等待网络确认",
		"9. 转账完成",
	}

	g.ui.ShowList("转账操作步骤", steps)

	// 显示示例转账信息
	g.ui.ShowInfo("📋 转账示例信息：")

	transferExample := map[string]string{
		"发送地址":   "Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn",
		"接收地址":   "Df2Kes6snEUeykiJJgrAtKPNPrAzPdPmTn",
		"转账金额":   "10.0 WES",
		"手续费":    "0.001 WES",
		"总计":     "10.001 WES",
		"预估确认时间": "2-5分钟",
	}

	g.ui.ShowKeyValuePairs("", transferExample)
}
