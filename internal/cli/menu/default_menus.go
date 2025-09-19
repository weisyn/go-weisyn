package menu

import (
	"context"
	"fmt"

	"github.com/pterm/pterm"

	"github.com/weisyn/v1/internal/app/version"
)

// initializeDefaultMenus 初始化默认菜单结构
func (dms *dualMenuSystem) initializeDefaultMenus() {
	dms.logger.Info("初始化默认菜单结构")

	// 创建主菜单
	dms.createMainMenu()

	// 创建系统级菜单
	dms.createSystemMenus()

	// 创建用户级菜单
	dms.createUserMenus()

	// 创建管理员菜单
	dms.createAdminMenus()

	dms.logger.Info("默认菜单结构初始化完成")
}

// createMainMenu 创建主菜单
func (dms *dualMenuSystem) createMainMenu() {
	mainMenu := &Menu{
		ID:          "main",
		Title:       "WES 区块链系统",
		Description: "欢迎使用WES - 下一代区块链操作系统",
		Level:       SystemLevel,
		Items: []*MenuItem{
			// 系统级功能
			{
				ID:          "system_info",
				Title:       "系统信息",
				Description: "查看系统状态和网络信息",
				Icon:        "📊",
				Type:        SubMenuItem,
				Level:       SystemLevel,
				Enabled:     true,
				Visible:     true,
				Order:       10,
			},

			// 区块链浏览
			{
				ID:          "blockchain_explorer",
				Title:       "区块链浏览器",
				Description: "浏览区块、交易和地址信息",
				Icon:        "🔍",
				Type:        SubMenuItem,
				Level:       SystemLevel,
				Enabled:     true,
				Visible:     true,
				Order:       20,
			},

			// 分隔符
			{
				ID:      "separator_1",
				Type:    SeparatorItem,
				Visible: true,
			},

			// 钱包管理
			{
				ID:          "wallet_management",
				Title:       "钱包管理",
				Description: "创建、导入和管理您的钱包",
				Icon:        "💳",
				Type:        SubMenuItem,
				Level:       SystemLevel, // 钱包创建不需要现有钱包
				Enabled:     true,
				Visible:     true,
				Order:       30,
			},

			// 资产管理
			{
				ID:          "asset_management",
				Title:       "资产管理",
				Description: "查看余额与转账",
				Icon:        "💰",
				Type:        SubMenuItem,
				Level:       UserLevel,
				Enabled:     true,
				Visible:     true,
				Order:       40,
			},

			// 共识参与
			{
				ID:          "consensus_participation",
				Title:       "共识参与",
				Description: "参与网络共识获得奖励",
				Icon:        "⚙️",
				Type:        SubMenuItem,
				Level:       UserLevel,
				Enabled:     true,
				Visible:     true,
				Order:       50,
			},

			// 分隔符
			{
				ID:      "separator_2",
				Type:    SeparatorItem,
				Visible: true,
			},

			// 开发者工具
			{
				ID:          "developer_tools",
				Title:       "开发者工具",
				Description: "智能合约部署和调试工具",
				Icon:        "🛠️",
				Type:        SubMenuItem,
				Level:       UserLevel,
				Enabled:     true,
				Visible:     true,
				Order:       60,
			},

			// 系统设置
			{
				ID:          "system_settings",
				Title:       "系统设置",
				Description: "配置系统参数和偏好设置",
				Icon:        "⚙️",
				Type:        SubMenuItem,
				Level:       SystemLevel,
				Enabled:     true,
				Visible:     true,
				Order:       70,
			},

			// 帮助支持
			{
				ID:          "help_support",
				Title:       "帮助与支持",
				Description: "获取帮助和支持信息",
				Icon:        "❓",
				Type:        SubMenuItem,
				Level:       SystemLevel,
				Enabled:     true,
				Visible:     true,
				Order:       80,
			},
		},
	}

	dms.mainMenu = mainMenu
	dms.menus["main"] = mainMenu

	// 创建子菜单引用
	dms.linkSubMenus(mainMenu)
}

// createSystemMenus 创建系统级菜单
func (dms *dualMenuSystem) createSystemMenus() {
	// 系统信息菜单
	systemInfoMenu := &Menu{
		ID:          "system_info",
		Title:       "系统信息",
		Description: "查看系统运行状态和网络信息",
		Level:       SystemLevel,
		Items: []*MenuItem{
			{
				ID:          "node_status",
				Title:       "节点状态",
				Description: "查看当前节点运行状态",
				Icon:        "🌐",
				Type:        ActionItem,
				Level:       SystemLevel,
				Enabled:     true,
				Visible:     true,
				Action:      dms.createPlaceholderAction("查看节点状态"),
			},
			{
				ID:          "network_info",
				Title:       "网络信息",
				Description: "查看网络连接和同步状态",
				Icon:        "📡",
				Type:        ActionItem,
				Level:       SystemLevel,
				Enabled:     true,
				Visible:     true,
				Action:      dms.createPlaceholderAction("查看网络信息"),
			},
			{
				ID:          "blockchain_stats",
				Title:       "区块链统计",
				Description: "查看区块链基本统计信息",
				Icon:        "📈",
				Type:        ActionItem,
				Level:       SystemLevel,
				Enabled:     true,
				Visible:     true,
				Action:      dms.createPlaceholderAction("查看区块链统计"),
			},
		},
	}

	// 区块链浏览器菜单
	blockchainExplorerMenu := &Menu{
		ID:          "blockchain_explorer",
		Title:       "区块链浏览器",
		Description: "浏览和查询区块链数据",
		Level:       SystemLevel,
		Items: []*MenuItem{
			{
				ID:          "latest_blocks",
				Title:       "最新区块",
				Description: "查看最近产生的区块",
				Icon:        "🧱",
				Type:        ActionItem,
				Level:       SystemLevel,
				Enabled:     true,
				Visible:     true,
				Action:      dms.createPlaceholderAction("查看最新区块"),
			},
			{
				ID:          "search_block",
				Title:       "搜索区块",
				Description: "根据高度或哈希搜索区块",
				Icon:        "🔍",
				Type:        ActionItem,
				Level:       SystemLevel,
				Enabled:     true,
				Visible:     true,
				Action:      dms.createPlaceholderAction("搜索区块"),
			},
			{
				ID:          "search_transaction",
				Title:       "搜索交易",
				Description: "根据哈希搜索交易信息",
				Icon:        "🔎",
				Type:        ActionItem,
				Level:       SystemLevel,
				Enabled:     true,
				Visible:     true,
				Action:      dms.createPlaceholderAction("搜索交易"),
			},
			{
				ID:          "address_info",
				Title:       "地址信息",
				Description: "查看地址余额和交易记录",
				Icon:        "📋",
				Type:        ActionItem,
				Level:       SystemLevel,
				Enabled:     true,
				Visible:     true,
				Action:      dms.createPlaceholderAction("查看地址信息"),
			},
		},
	}

	// 系统设置菜单
	systemSettingsMenu := &Menu{
		ID:          "system_settings",
		Title:       "系统设置",
		Description: "配置系统参数和用户偏好",
		Level:       SystemLevel,
		Items: []*MenuItem{
			{
				ID:          "display_settings",
				Title:       "显示设置",
				Description: "配置界面主题和显示选项",
				Icon:        "🎨",
				Type:        ActionItem,
				Level:       SystemLevel,
				Enabled:     true,
				Visible:     true,
				Action:      dms.createPlaceholderAction("配置显示设置"),
			},
			{
				ID:          "language_settings",
				Title:       "语言设置",
				Description: "选择界面语言",
				Icon:        "🌍",
				Type:        ActionItem,
				Level:       SystemLevel,
				Enabled:     true,
				Visible:     true,
				Action:      dms.createPlaceholderAction("设置界面语言"),
			},
			{
				ID:          "network_settings",
				Title:       "网络设置",
				Description: "配置网络连接参数",
				Icon:        "🌐",
				Type:        ActionItem,
				Level:       SystemLevel,
				Enabled:     true,
				Visible:     true,
				Action:      dms.createPlaceholderAction("配置网络设置"),
			},
		},
	}

	// 帮助支持菜单
	helpSupportMenu := &Menu{
		ID:          "help_support",
		Title:       "帮助与支持",
		Description: "获取使用帮助和技术支持",
		Level:       SystemLevel,
		Items: []*MenuItem{
			{
				ID:          "user_guide",
				Title:       "用户指南",
				Description: "查看详细的使用说明",
				Icon:        "📖",
				Type:        ActionItem,
				Level:       SystemLevel,
				Enabled:     true,
				Visible:     true,
				Action:      dms.createPlaceholderAction("查看用户指南"),
			},
			{
				ID:          "first_time_guide",
				Title:       "新手引导",
				Description: "重新运行首次使用引导",
				Icon:        "🎯",
				Type:        ActionItem,
				Level:       SystemLevel,
				Enabled:     true,
				Visible:     true,
				Action:      dms.createPlaceholderAction("运行新手引导"),
			},
			{
				ID:          "about_system",
				Title:       "关于系统",
				Description: "查看版本信息和致谢",
				Icon:        "ℹ️",
				Type:        ActionItem,
				Level:       SystemLevel,
				Enabled:     true,
				Visible:     true,
				Action:      dms.createAboutSystemAction(),
			},
		},
	}

	// 注册菜单
	dms.menus["system_info"] = systemInfoMenu
	dms.menus["blockchain_explorer"] = blockchainExplorerMenu
	dms.menus["system_settings"] = systemSettingsMenu
	dms.menus["help_support"] = helpSupportMenu
}

// createUserMenus 创建用户级菜单
func (dms *dualMenuSystem) createUserMenus() {
	// 钱包管理菜单
	walletManagementMenu := &Menu{
		ID:          "wallet_management",
		Title:       "钱包管理",
		Description: "管理您的数字钱包",
		Level:       SystemLevel,
		Items: []*MenuItem{
			{
				ID:          "create_wallet",
				Title:       "创建新钱包",
				Description: "创建一个新的数字钱包",
				Icon:        "➕",
				Type:        ActionItem,
				Level:       SystemLevel,
				Enabled:     true,
				Visible:     true,
				Action:      dms.createPlaceholderAction("创建新钱包"),
			},
			{
				ID:          "import_wallet",
				Title:       "导入钱包",
				Description: "从私钥或文件导入钱包",
				Icon:        "📥",
				Type:        ActionItem,
				Level:       SystemLevel,
				Enabled:     true,
				Visible:     true,
				Action:      dms.createPlaceholderAction("导入钱包"),
			},
			{
				ID:          "list_wallets",
				Title:       "钱包列表",
				Description: "查看所有已创建的钱包",
				Icon:        "📋",
				Type:        ActionItem,
				Level:       SystemLevel,
				Enabled:     true,
				Visible:     true,
				Action:      dms.createPlaceholderAction("查看钱包列表"),
			},
			{
				ID:          "unlock_wallet",
				Title:       "解锁钱包",
				Description: "解锁钱包以进行交易操作",
				Icon:        "🔓",
				Type:        ActionItem,
				Level:       SystemLevel,
				Enabled:     true,
				Visible:     true,
				Action:      dms.createPlaceholderAction("解锁钱包"),
			},
			{
				ID:          "backup_wallet",
				Title:       "备份钱包",
				Description: "导出钱包备份文件",
				Icon:        "💾",
				Type:        ActionItem,
				Level:       UserLevel,
				Enabled:     true,
				Visible:     true,
				Action:      dms.createPlaceholderAction("备份钱包"),
			},
		},
	}

	// 资产管理菜单
	assetManagementMenu := &Menu{
		ID:          "asset_management",
		Title:       "资产管理",
		Description: "管理您的数字资产",
		Level:       UserLevel,
		Items: []*MenuItem{
			{
				ID:          "check_balance",
				Title:       "查看余额",
				Description: "查看钱包余额和资产分布",
				Icon:        "💰",
				Type:        ActionItem,
				Level:       UserLevel,
				Enabled:     true,
				Visible:     true,
				Action:      dms.createPlaceholderAction("查看余额"),
			},
			{
				ID:          "send_transfer",
				Title:       "发送转账",
				Description: "向其他地址发送WES代币",
				Icon:        "📤",
				Type:        ActionItem,
				Level:       UserLevel,
				Enabled:     true,
				Visible:     true,
				Action:      dms.createPlaceholderAction("发送转账"),
			},
			{
				ID:          "batch_transfer",
				Title:       "批量转账",
				Description: "同时向多个地址发送代币",
				Icon:        "📦",
				Type:        ActionItem,
				Level:       UserLevel,
				Enabled:     true,
				Visible:     true,
				Action:      dms.createPlaceholderAction("批量转账"),
			},

			{
				ID:          "timelock_transfer",
				Title:       "时间锁转账",
				Description: "创建定时解锁的转账",
				Icon:        "⏰",
				Type:        ActionItem,
				Level:       UserLevel,
				Enabled:     true,
				Visible:     true,
				Action:      dms.createPlaceholderAction("时间锁转账"),
			},
		},
	}

	// 共识参与菜单
	consensusParticipationMenu := &Menu{
		ID:          "consensus_participation",
		Title:       "共识参与",
		Description: "参与网络共识，获得奖励",
		Level:       UserLevel,
		Items: []*MenuItem{
			{
				ID:          "mining_status",
				Title:       "共识状态",
				Description: "查看当前共识参与状态",
				Icon:        "📊",
				Type:        ActionItem,
				Level:       UserLevel,
				Enabled:     true,
				Visible:     true,
				Action:      dms.createPlaceholderAction("查看共识状态"),
			},
			{
				ID:          "start_mining",
				Title:       "开始共识",
				Description: "开始参与网络共识",
				Icon:        "▶️",
				Type:        ActionItem,
				Level:       UserLevel,
				Enabled:     true,
				Visible:     true,
				Action:      dms.createPlaceholderAction("开始共识参与"),
			},
			{
				ID:          "stop_mining",
				Title:       "停止共识",
				Description: "停止参与网络共识",
				Icon:        "⏹️",
				Type:        ActionItem,
				Level:       UserLevel,
				Enabled:     true,
				Visible:     true,
				Action:      dms.createPlaceholderAction("停止共识参与"),
			},
			{
				ID:          "mining_rewards",
				Title:       "共识奖励",
				Description: "查看共识奖励记录",
				Icon:        "🏆",
				Type:        ActionItem,
				Level:       UserLevel,
				Enabled:     true,
				Visible:     true,
				Action:      dms.createPlaceholderAction("查看共识奖励"),
			},
			{
				ID:          "mining_settings",
				Title:       "共识设置",
				Description: "配置共识参与参数",
				Icon:        "⚙️",
				Type:        ActionItem,
				Level:       UserLevel,
				Enabled:     true,
				Visible:     true,
				Action:      dms.createPlaceholderAction("配置共识设置"),
			},
		},
	}

	// 注册菜单
	dms.menus["wallet_management"] = walletManagementMenu
	dms.menus["asset_management"] = assetManagementMenu
	dms.menus["consensus_participation"] = consensusParticipationMenu
}

// createAdminMenus 创建管理员菜单
func (dms *dualMenuSystem) createAdminMenus() {
	// 开发者工具菜单
	developerToolsMenu := &Menu{
		ID:          "developer_tools",
		Title:       "开发者工具",
		Description: "智能合约和开发相关工具",
		Level:       UserLevel,
		Items: []*MenuItem{
			{
				ID:          "deploy_contract",
				Title:       "部署合约",
				Description: "部署智能合约到区块链",
				Icon:        "🚀",
				Type:        ActionItem,
				Level:       UserLevel,
				Enabled:     false, // 暂不可用
				Visible:     true,
				Action:      dms.createPlaceholderAction("部署智能合约"),
			},
			{
				ID:          "call_contract",
				Title:       "调用合约",
				Description: "调用已部署的智能合约",
				Icon:        "📞",
				Type:        ActionItem,
				Level:       UserLevel,
				Enabled:     false, // 暂不可用
				Visible:     true,
				Action:      dms.createPlaceholderAction("调用智能合约"),
			},
			{
				ID:          "contract_events",
				Title:       "合约事件",
				Description: "监听和查看合约事件",
				Icon:        "👁️",
				Type:        ActionItem,
				Level:       UserLevel,
				Enabled:     false, // 暂不可用
				Visible:     true,
				Action:      dms.createPlaceholderAction("查看合约事件"),
			},
		},
	}

	dms.menus["developer_tools"] = developerToolsMenu
}

// linkSubMenus 链接子菜单引用
func (dms *dualMenuSystem) linkSubMenus(menu *Menu) {
	for _, item := range menu.Items {
		if item.Type == SubMenuItem && item.SubMenu == nil {
			// 根据ID查找对应的子菜单
			if subMenu, exists := dms.menus[item.ID]; exists {
				item.SubMenu = subMenu
				subMenu.Parent = menu
			}
		}
	}
}

// createPlaceholderAction 创建功能信息页动作（替换"开发中"占位符）
func (dms *dualMenuSystem) createPlaceholderAction(actionName string) MenuAction {
	return func(ctx context.Context) error {
		// 显示功能规划信息页，而不是简单的"开发中"消息
		pterm.DefaultSection.Println(fmt.Sprintf("%s - 功能说明", actionName))

		pterm.DefaultBox.WithTitle("📋 功能规划").Println(
			fmt.Sprintf("功能名称: %s\n\n", actionName) +
				"🔧 当前状态: 规划阶段\n\n" +
				"📝 功能说明:\n" +
				"此功能正在进行需求分析和技术设计。\n" +
				"我们致力于提供稳定、高性能的区块链操作体验。\n\n" +
				"💡 替代方案:\n" +
				"• 使用API接口直接操作\n" +
				"• 通过其他菜单项实现相关功能\n" +
				"• 查看文档了解命令行操作\n\n" +
				"📞 反馈渠道:\n" +
				"如果您需要此功能，请通过以下方式反馈:\n" +
				"• GitHub Issues: 提交功能请求\n" +
				"• 开发文档: 查看技术规范\n" +
				"• 社区讨论: 参与功能设计讨论",
		)

		dms.ui.ShowInfo("提示: 您可以通过其他已实现的功能达到类似效果")
		return nil
	}
}

// createAboutSystemAction 创建关于系统的动作
func (dms *dualMenuSystem) createAboutSystemAction() MenuAction {
	return func(ctx context.Context) error {
		// 显示区块链系统信息（去除违背区块链理念的中心化信息）
		aboutInfo := map[string]string{
			"系统名称":  "WES 区块链系统",
			"CLI版本": version.GetVersion(),
			"共识机制":  "EUTXO + PoW",
			"架构特点":  "双层权限架构",
			"技术栈":   "Go + libp2p",
			"UI框架":  "Pterm终端界面",
			"许可证":   "MIT License",
		}

		dms.ui.ShowKeyValuePairs("关于WES系统", aboutInfo)

		dms.ui.ShowInfo(`
🙏 特别致谢：

• Go开发团队 - 提供了优秀的编程语言
• Pterm项目 - 提供了美观的终端UI库
• 所有开源贡献者 - 让这个项目成为可能

💡 如果您在使用过程中遇到问题或有建议，请：
• 访问官网获取最新文档
• 在GitHub上提交Issue
• 加入社区讨论交流

感谢您使用WES区块链系统！
		`)

		return nil
	}
}
