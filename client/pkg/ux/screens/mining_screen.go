package screens

import (
	"context"
	"fmt"

	"github.com/weisyn/v1/client/core/wallet"
	"github.com/weisyn/v1/client/pkg/transport/api"
	"github.com/weisyn/v1/client/pkg/ux/ui"
)

// MiningScreen 挖矿控制屏幕
type MiningScreen struct {
	ui             ui.Components
	miningAdapter  *api.MiningAdapter
	accountManager *wallet.AccountManager
}

// NewMiningScreen 创建挖矿控制屏幕
func NewMiningScreen(
	components ui.Components,
	miningAdapter *api.MiningAdapter,
	accountManager *wallet.AccountManager,
) *MiningScreen {
	return &MiningScreen{
		ui:             components,
		miningAdapter:  miningAdapter,
		accountManager: accountManager,
	}
}

// Show 显示挖矿控制菜单
func (s *MiningScreen) Show(ctx context.Context) error {
	for {
		s.ui.Clear()
		s.ui.ShowHeader("⛏️  挖矿控制")

		// 先查询当前挖矿状态
		status, err := s.miningAdapter.GetMiningStatus(ctx)
		if err != nil {
			s.ui.ShowWarning(fmt.Sprintf("获取挖矿状态失败: %v", err))
		} else {
			// 显示当前状态
			if status.IsRunning {
				s.ui.ShowSuccess(fmt.Sprintf("✅ 挖矿运行中 - 矿工地址: %s", status.MinerAddress))
			} else {
				s.ui.ShowInfo("⏸️  挖矿已停止")
			}
		}

		s.ui.ShowInfo("")

		// 菜单选项（根据状态动态调整）
		var options []string
		if status != nil && status.IsRunning {
			options = []string{
				"停止挖矿",
				"查看挖矿状态",
				"返回上一级",
			}
		} else {
			options = []string{
				"启动挖矿",
				"查看挖矿状态",
				"返回上一级",
			}
		}

		choice, err := s.ui.ShowMenu("请选择操作", options)
		if err != nil {
			return err
		}

		// 根据当前状态处理选择
		if status != nil && status.IsRunning {
			// 挖矿运行中的菜单
			switch choice {
			case 0: // 停止挖矿
				if err := s.stopMining(ctx); err != nil {
					s.ui.ShowError(fmt.Sprintf("停止挖矿失败: %v", err))
				}
				s.ui.ShowContinuePrompt("", "")
			case 1: // 查看状态
				if err := s.showMiningStatus(ctx); err != nil {
					s.ui.ShowError(fmt.Sprintf("查询失败: %v", err))
				}
				s.ui.ShowContinuePrompt("", "")
			case 2: // 返回
				return nil
			}
		} else {
			// 挖矿已停止的菜单
			switch choice {
			case 0: // 启动挖矿
				if err := s.startMining(ctx); err != nil {
					s.ui.ShowError(fmt.Sprintf("启动挖矿失败: %v", err))
				}
				s.ui.ShowContinuePrompt("", "")
			case 1: // 查看状态
				if err := s.showMiningStatus(ctx); err != nil {
					s.ui.ShowError(fmt.Sprintf("查询失败: %v", err))
				}
				s.ui.ShowContinuePrompt("", "")
			case 2: // 返回
				return nil
			}
		}
	}
}

// startMining 启动挖矿
func (s *MiningScreen) startMining(ctx context.Context) error {
	s.ui.ShowHeader("🚀 启动挖矿")

	// 步骤1: 选择矿工钱包
	accounts, err := s.accountManager.ListAccounts()
	if err != nil {
		return fmt.Errorf("获取账户列表失败: %w", err)
	}

	if len(accounts) == 0 {
		s.ui.ShowWarning("您还没有钱包，请先创建钱包")
		return fmt.Errorf("no wallets available")
	}

	// 构建钱包选项列表
	walletOptions := make([]string, 0, len(accounts))
	for _, account := range accounts {
		label := fmt.Sprintf("%s (%s)", account.Name, account.Address)
		if account.IsDefault {
			label = "[默认] " + label
		}
		walletOptions = append(walletOptions, label)
	}

	// 让用户选择钱包
	selectedIndex, err := s.ui.ShowMenu("选择接收挖矿奖励的钱包", walletOptions)
	if err != nil {
		return err
	}

	selectedAccount := accounts[selectedIndex]
	minerAddress := selectedAccount.Address

	// 步骤2: 确认启动
	s.ui.ShowInfo("")
	s.ui.ShowInfo(fmt.Sprintf("矿工地址: %s", minerAddress))

	confirmed, err := s.ui.ShowConfirmDialog("确认启动", "是否开始挖矿？")
	if err != nil || !confirmed {
		s.ui.ShowInfo("已取消")
		return nil
	}

	// 步骤3: 启动挖矿
	s.ui.ShowInfo("正在启动挖矿...")

	if err := s.miningAdapter.StartMining(ctx, minerAddress); err != nil {
		return fmt.Errorf("启动挖矿失败: %w", err)
	}

	s.ui.ShowSuccess("✅ 挖矿已启动！")
	s.ui.ShowInfo(fmt.Sprintf("矿工地址: %s", minerAddress))
	s.ui.ShowInfo("💰 挖矿奖励将发送到此地址")

	return nil
}

// stopMining 停止挖矿
func (s *MiningScreen) stopMining(ctx context.Context) error {
	s.ui.ShowHeader("⏹️  停止挖矿")

	// 确认停止
	confirmed, err := s.ui.ShowConfirmDialog("确认停止", "是否停止挖矿？")
	if err != nil || !confirmed {
		s.ui.ShowInfo("已取消")
		return nil
	}

	// 停止挖矿
	s.ui.ShowInfo("正在停止挖矿...")

	if err := s.miningAdapter.StopMining(ctx); err != nil {
		return fmt.Errorf("停止挖矿失败: %w", err)
	}

	s.ui.ShowSuccess("✅ 挖矿已停止")

	return nil
}

// showMiningStatus 显示挖矿状态
func (s *MiningScreen) showMiningStatus(ctx context.Context) error {
	s.ui.ShowHeader("📊 挖矿状态")
	s.ui.ShowInfo("正在查询...")

	// 获取挖矿状态
	status, err := s.miningAdapter.GetMiningStatus(ctx)
	if err != nil {
		return fmt.Errorf("获取挖矿状态失败: %w", err)
	}

	s.ui.ShowInfo("")

	if status.IsRunning {
		s.ui.ShowSuccess("✅ 挖矿运行中")
		s.ui.ShowInfo(fmt.Sprintf("⛏️  矿工地址: %s", status.MinerAddress))
		s.ui.ShowInfo("💰 挖矿奖励将发送到此地址")
	} else {
		s.ui.ShowInfo("⏸️  挖矿已停止")
		s.ui.ShowInfo("💡 提示: 选择'启动挖矿'开始获得区块奖励")
	}

	return nil
}
