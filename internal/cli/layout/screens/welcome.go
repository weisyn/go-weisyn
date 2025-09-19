// Package screens 提供具体的屏幕实现
package screens

import (
	"context"
	"time"

	"github.com/pterm/pterm"

	"github.com/weisyn/v1/internal/app/version"
	"github.com/weisyn/v1/internal/cli/layout"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// WelcomeScreen 欢迎屏幕
type WelcomeScreen struct {
	*layout.BaseScreen
	logger log.Logger
}

// NewWelcomeScreen 创建欢迎屏幕
func NewWelcomeScreen(logger log.Logger) *WelcomeScreen {
	config := layout.ScreenConfig{
		ShowTopBar:    false, // 欢迎屏幕不显示状态栏
		ShowFooterTip: true,
		FooterTipType: "menu",
		AutoClear:     true,
		Timeout:       10 * time.Second, // 10秒后自动进入主菜单
	}

	return &WelcomeScreen{
		BaseScreen: layout.NewBaseScreen("welcome", config),
		logger:     logger,
	}
}

// Render 渲染欢迎屏幕
func (s *WelcomeScreen) Render(ctx context.Context) (*layout.ScreenResult, error) {
	// 添加顶部空行，让界面不那么拥挤
	pterm.Println()

	// 显示WES ASCII艺术字
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

	// 显示简要的使用提示
	pterm.Info.Println("系统准备就绪，即将进入主菜单...")
	pterm.Println()

	s.logger.Info("显示欢迎屏幕")

	// 暂停一下让用户看到欢迎信息
	select {
	case <-time.After(2 * time.Second):
		// 2秒后自动进入主菜单
		return &layout.ScreenResult{
			Action:     "next",
			NextScreen: "main_menu",
		}, nil
	case <-ctx.Done():
		return &layout.ScreenResult{
			Action: "exit",
		}, ctx.Err()
	}
}

// OnEnter 进入欢迎屏幕时的准备工作
func (s *WelcomeScreen) OnEnter(ctx context.Context) error {
	s.logger.Info("进入欢迎屏幕")
	return nil
}

// OnExit 退出欢迎屏幕时的清理工作
func (s *WelcomeScreen) OnExit(ctx context.Context) error {
	s.logger.Info("退出欢迎屏幕")
	return nil
}
