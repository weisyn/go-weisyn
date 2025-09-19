// Package screens - FirstTimeGuideScreen实现
package screens

import (
	"context"
	"time"

	"github.com/pterm/pterm"

	"github.com/weisyn/v1/internal/app/version"
	"github.com/weisyn/v1/internal/cli/guides"
	"github.com/weisyn/v1/internal/cli/layout"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// FirstTimeGuideScreen 首次使用引导屏幕
type FirstTimeGuideScreen struct {
	*layout.BaseScreen
	logger         log.Logger
	firstTimeGuide guides.FirstTimeGuide
}

// NewFirstTimeGuideScreen 创建首次使用引导屏幕
func NewFirstTimeGuideScreen(logger log.Logger, firstTimeGuide guides.FirstTimeGuide) *FirstTimeGuideScreen {
	config := layout.ScreenConfig{
		ShowTopBar:    false, // 引导期间不显示状态栏，避免干扰
		ShowFooterTip: false, // 引导会提供自己的提示
		FooterTipType: "",
		AutoClear:     true,
		Timeout:       0, // 引导过程不设置超时
	}

	return &FirstTimeGuideScreen{
		BaseScreen:     layout.NewBaseScreen("first_time_guide", config),
		logger:         logger,
		firstTimeGuide: firstTimeGuide,
	}
}

// Render 渲染首次引导屏幕
func (s *FirstTimeGuideScreen) Render(ctx context.Context) (*layout.ScreenResult, error) {
	s.logger.Info("开始首次用户引导流程")

	// 执行完整的首次引导流程
	success, err := s.firstTimeGuide.CheckAndRunFirstTimeSetup(ctx)
	if err != nil {
		s.logger.Errorf("首次引导执行失败: %v", err)

		// 引导失败，但不阻止进入主菜单
		return &layout.ScreenResult{
			Action:     "next",
			NextScreen: "main_menu",
			Data: map[string]interface{}{
				"guide_completed": false,
				"guide_error":     err.Error(),
			},
		}, nil
	}

	if success {
		s.logger.Info("首次引导流程完成")

		// 显示完成祝贺消息
		s.showCompletionMessage()

		// 引导完成，进入主菜单
		return &layout.ScreenResult{
			Action:     "next",
			NextScreen: "main_menu",
			Data: map[string]interface{}{
				"guide_completed": true,
			},
		}, nil
	} else {
		s.logger.Info("用户跳过首次引导")

		// 用户跳过引导，直接进入主菜单
		return &layout.ScreenResult{
			Action:     "next",
			NextScreen: "main_menu",
			Data: map[string]interface{}{
				"guide_completed": false,
				"guide_skipped":   true,
			},
		}, nil
	}
}

// showWelcomeMessage 显示欢迎消息（含ASCII艺术字）
func (s *FirstTimeGuideScreen) showWelcomeMessage() {
	// 添加顶部空行，让界面不那么拥挤
	pterm.Println()

	// 显示WES ASCII艺术字 - 与常规用户保持一致
	asciiArt := `██╗    ██╗███████╗███████╗
██║    ██║██╔════╝██╔════╝
██║ █╗ ██║█████╗  ███████╗
██║███╗██║██╔══╝  ╚════██║
╚███╔███╔╝███████╗███████║
 ╚══╝╚══╝ ╚══════╝╚══════╝`

	// 左对齐显示ASCII艺术
	lines := pterm.NewStyle(pterm.FgLightBlue, pterm.Bold).Sprint(asciiArt)
	pterm.Println(lines)

	// ASCII艺术后添加空行
	pterm.Println()

	// 显示版本和状态信息 - 左对齐
	pterm.Println(pterm.LightGreen("🌟 微迅 (weisyn) 区块链节点 CLI " + version.GetVersion()))
	pterm.Println(pterm.Gray("基于EUTXO模型的下一代区块链平台"))
	pterm.Println() // 标题后添加换行
}

// showCompletionMessage 显示引导完成消息
func (s *FirstTimeGuideScreen) showCompletionMessage() {
	// 使用统一页面工具显示完成消息
	// 清屏交由布局管理器完成

	// 显示完成横幅
	pterm.DefaultHeader.WithBackgroundStyle(pterm.NewStyle(pterm.FgGreen)).
		WithMargin(2).
		Println("🎉 恭喜！首次引导已完成")

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

准备进入主菜单...
	`

	pterm.DefaultBox.
		WithTitle("引导完成").
		WithTitleTopCenter().
		WithBoxStyle(pterm.NewStyle(pterm.FgGreen)).
		Println(completionContent)

	// 暂停几秒让用户阅读
	time.Sleep(3 * time.Second)
}

// OnEnter 进入首次引导屏幕时的准备工作
func (s *FirstTimeGuideScreen) OnEnter(ctx context.Context) error {
	s.logger.Info("进入首次引导屏幕")
	return nil
}

// OnExit 退出首次引导屏幕时的清理工作
func (s *FirstTimeGuideScreen) OnExit(ctx context.Context) error {
	s.logger.Info("退出首次引导屏幕")
	return nil
}

// CanExit 检查是否可以退出引导屏幕
func (s *FirstTimeGuideScreen) CanExit(ctx context.Context) (bool, error) {
	// 引导过程中允许用户通过Ctrl+C退出
	return true, nil
}
