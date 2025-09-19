package guides

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pterm/pterm"
)

// initializeSteps 初始化引导步骤
func (g *firstTimeGuide) initializeSteps() {
	g.progress = &GuideProgress{
		TotalSteps:     4,
		CompletedSteps: 0,
		CurrentStep:    1,
		Steps: []*GuideStep{
			{
				ID:          1,
				Title:       "步骤1: 创建您的第一个钱包",
				Description: "学习什么是钱包以及如何安全地创建和管理您的第一个钱包",
				Action:      g.step1CreateWallet,
				IsCompleted: false,
			},
			{
				ID:          2,
				Title:       "步骤2: 查询钱包余额",
				Description: "了解如何查询钱包余额和交易记录",
				Action:      g.step2CheckBalance,
				IsCompleted: false,
			},
			{
				ID:          3,
				Title:       "步骤3: 学习共识参与",
				Description: "了解区块链共识机制以及如何参与网络维护",
				Action:      g.step3LearnConsensus,
				IsCompleted: false,
			},
			{
				ID:          4,
				Title:       "步骤4: 体验转账操作",
				Description: "学习如何安全地进行转账操作",
				Action:      g.step4ExperienceTransfer,
				IsCompleted: false,
			},
		},
	}
}

// CheckAndRunFirstTimeSetup 检查并运行首次设置
func (g *firstTimeGuide) CheckAndRunFirstTimeSetup(ctx context.Context) (bool, error) {
	// 使用权限管理器检查是否为首次用户
	userContext := g.permissionManager.GetUserContext()

	if !userContext.IsFirstTimeUser {
		return false, nil // 不是首次用户，直接返回
	}

	// 使用统一页面工具显示欢迎界面
	g.ui.ShowHeader("")
	g.showSimpleWelcome()

	// 检查是否启用自动演示模式
	isAutoMode := os.Getenv("WES_AUTO_DEMO_MODE") == "true"

	if isAutoMode {
		g.ui.ShowInfo("🤖 自动演示模式：跳过引导")
		time.Sleep(1 * time.Second)
		return false, nil
	}

	// 询问是否开始引导
	pterm.Info.Println("欢迎首次使用WES！是否需要4步新手引导？")
	pterm.Println()

	confirmed, err := pterm.DefaultInteractiveConfirm.
		WithDefaultText("开始新手引导？").
		WithDefaultValue(false).
		Show()

	if err != nil {
		return false, fmt.Errorf("用户确认失败: %v", err)
	}

	if !confirmed {
		g.logger.Info("用户跳过了首次引导")
		return false, nil
	}

	// 运行完整引导
	if err := g.RunFullGuide(ctx); err != nil {
		return false, fmt.Errorf("引导执行失败: %v", err)
	}

	return true, nil
}

// RunFullGuide 运行完整的4步引导流程
func (g *firstTimeGuide) RunFullGuide(ctx context.Context) error {
	g.logger.Info("开始执行完整引导流程")

	// 切换到引导概览页面
	g.ui.ShowHeader("📋 引导流程概览")
	g.showGuideOverview()

	// 逐步执行引导
	for i, step := range g.progress.Steps {
		if step.IsCompleted {
			continue // 跳过已完成的步骤
		}

		g.progress.CurrentStep = i + 1

		// 每个步骤开始前显示步骤标题
		g.ui.ShowHeader(step.Title)

		// 显示当前步骤信息
		g.showStepInfo(step)

		// 执行步骤
		if err := step.Action(ctx); err != nil {
			g.logger.Error(fmt.Sprintf("引导步骤执行失败: step=%d, title=%s, error=%v",
				step.ID, step.Title, err))

			// 询问用户是否继续
			if !g.askToContinue(step) {
				return fmt.Errorf("引导被用户中断")
			}
			continue
		}

		// 标记步骤完成
		step.IsCompleted = true
		g.progress.CompletedSteps++

		g.ui.ShowSuccess(fmt.Sprintf("✅ %s 完成！", step.Title))
	}

	// 显示引导完成消息
	g.ui.ShowHeader("🎉 引导完成")
	g.showCompletionMessage()

	// 标记首次用户引导完成
	g.permissionManager.GetUserContext().IsFirstTimeUser = false

	g.logger.Info("首次用户引导流程完成")
	return nil
}

// GetProgress 获取引导进度
func (g *firstTimeGuide) GetProgress() *GuideProgress {
	return g.progress
}

// IsCompleted 检查引导是否完成
func (g *firstTimeGuide) IsCompleted() bool {
	return g.progress.CompletedSteps >= g.progress.TotalSteps
}

// ResetGuide 重置引导状态
func (g *firstTimeGuide) ResetGuide(ctx context.Context) error {
	g.initializeSteps()
	g.logger.Info("引导状态已重置")
	return nil
}

// showSimpleWelcome 显示简化的欢迎消息
func (g *firstTimeGuide) showSimpleWelcome() {
	// 显示WES ASCII艺术字
	asciiArt := `██╗    ██╗███████╗███████╗
██║    ██║██╔════╝██╔════╝
██║ █╗ ██║█████╗  ███████╗
██║███╗██║██╔══╝  ╚════██║
╚███╔███╔╝███████╗███████║
 ╚══╝╚══╝ ╚══════╝╚══════╝`

	pterm.Println()
	lines := pterm.NewStyle(pterm.FgLightBlue, pterm.Bold).Sprint(asciiArt)
	pterm.Println(lines)
	pterm.Println()

	// 显示版本信息
	pterm.Println(pterm.LightGreen("🌟 微迅 (weisyn) 区块链节点 CLI v0.0.1"))
	pterm.Println(pterm.Gray("基于EUTXO模型的下一代区块链平台"))
	pterm.Println()
}

// showGuideOverview 显示引导概览
func (g *firstTimeGuide) showGuideOverview() {
	g.ui.ShowSection("📋 引导流程概览")

	// 显示所有步骤
	stepList := make([]string, len(g.progress.Steps))
	for i, step := range g.progress.Steps {
		status := "⏳ 待完成"
		if step.IsCompleted {
			status = "✅ 已完成"
		} else if i+1 == g.progress.CurrentStep {
			status = "🔄 进行中"
		}

		stepList[i] = fmt.Sprintf("%s - %s", step.Title, status)
	}

	g.ui.ShowList("", stepList)
}

// showStepInfo 显示步骤信息
func (g *firstTimeGuide) showStepInfo(step *GuideStep) {
	g.ui.ShowHeader(fmt.Sprintf("🎯 %s", step.Title))
	g.ui.ShowInfo(step.Description)

	// 显示进度
	progressText := fmt.Sprintf("进度: %d/%d", g.progress.CurrentStep, g.progress.TotalSteps)
	g.ui.ShowInfo(progressText)
}

// askToContinue 询问是否继续
func (g *firstTimeGuide) askToContinue(step *GuideStep) bool {
	// 检查是否启用自动演示模式
	if os.Getenv("WES_AUTO_DEMO_MODE") == "true" {
		g.ui.ShowInfo("🤖 自动演示模式：继续执行下一步骤")
		time.Sleep(500 * time.Millisecond)
		return true
	}

	// Note: 这个方法通常在错误处理中调用，暂时不添加context支持
	// 如需支持可以参考上面的模式
	confirmed, _ := g.ui.ShowConfirmDialog(
		"⚠️ 步骤执行遇到问题",
		fmt.Sprintf("步骤 '%s' 执行时遇到问题，是否继续下一步骤？", step.Title),
	)
	return confirmed
}

// showCompletionMessage 显示完成消息
func (g *firstTimeGuide) showCompletionMessage() {
	g.ui.ShowHeader("🎉 恭喜！引导流程已完成")

	completionContent := `
✅ 您已经成功完成了WES新用户引导！

🎓 您现在已经掌握了：
• 钱包的创建和管理
• 余额查询的基本方法
• 区块链共识机制的基础知识
• 安全转账的操作流程

🚀 接下来您可以：
• 探索更多高级功能
• 参与网络共识获得收益
• 开发或部署智能合约
• 加入WES社区交流

💡 小提示：您随时可以通过主菜单的"帮助"功能回顾这些指导内容。
	`

	g.ui.ShowPanel("引导完成", completionContent)
}

// isFirstTimeUser 检查是否为首次用户（兼容旧代码）
func (g *firstTimeGuide) isFirstTimeUser(ctx context.Context) (bool, error) {
	return g.permissionManager.GetUserContext().IsFirstTimeUser, nil
}

// showFirstTimeWelcome 显示首次欢迎界面（兼容旧代码）
func (g *firstTimeGuide) showFirstTimeWelcome() {
	g.showSimpleWelcome()
}
