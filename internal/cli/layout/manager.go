// Package layout - LayoutManager核心实现
package layout

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/pterm/pterm"

	"github.com/weisyn/v1/internal/cli/status"
	"github.com/weisyn/v1/internal/cli/ui"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// NavigationAction 导航动作类型
type NavigationAction string

const (
	ActionNext   NavigationAction = "next"
	ActionBack   NavigationAction = "back"
	ActionExit   NavigationAction = "exit"
	ActionReload NavigationAction = "reload"
)

// LayoutManager 屏幕布局管理器
type LayoutManager struct {
	logger          log.Logger
	statusManager   *status.StatusManager
	uiComponents    ui.Components
	registry        *ScreenRegistry
	navigationStack []string // 导航栈，用于Back操作
	currentScreen   string   // 当前屏幕名称
}

// NewLayoutManager 创建布局管理器
func NewLayoutManager(
	logger log.Logger,
	statusManager *status.StatusManager,
	uiComponents ui.Components,
) *LayoutManager {
	return &LayoutManager{
		logger:          logger,
		statusManager:   statusManager,
		uiComponents:    uiComponents,
		registry:        NewScreenRegistry(),
		navigationStack: make([]string, 0),
	}
}

// RegisterScreen 注册屏幕
func (m *LayoutManager) RegisterScreen(screen Screen) {
	m.registry.Register(screen)
	m.logger.Debugf("注册屏幕: %s", screen.GetName())
}

// Show 显示指定屏幕
func (m *LayoutManager) Show(ctx context.Context, screenName string) error {
	screen, exists := m.registry.Get(screenName)
	if !exists {
		return fmt.Errorf("屏幕不存在: %s", screenName)
	}

	// 压入导航栈
	if m.currentScreen != "" {
		m.navigationStack = append(m.navigationStack, m.currentScreen)
	}

	return m.displayScreen(ctx, screen)
}

// Replace 替换当前屏幕（不影响导航栈）
func (m *LayoutManager) Replace(ctx context.Context, screenName string) error {
	screen, exists := m.registry.Get(screenName)
	if !exists {
		return fmt.Errorf("屏幕不存在: %s", screenName)
	}

	return m.displayScreen(ctx, screen)
}

// Back 返回上一个屏幕
func (m *LayoutManager) Back(ctx context.Context) error {
	if len(m.navigationStack) == 0 {
		return errors.New("导航栈为空，无法返回")
	}

	// 弹出导航栈
	prevScreenName := m.navigationStack[len(m.navigationStack)-1]
	m.navigationStack = m.navigationStack[:len(m.navigationStack)-1]

	screen, exists := m.registry.Get(prevScreenName)
	if !exists {
		return fmt.Errorf("上一个屏幕不存在: %s", prevScreenName)
	}

	// 不再压入导航栈，因为这是返回操作
	m.currentScreen = prevScreenName
	return m.displayScreen(ctx, screen)
}

// displayScreen 显示屏幕的核心逻辑
func (m *LayoutManager) displayScreen(ctx context.Context, screen Screen) error {
	config := screen.GetConfig()

	// 1. 退出当前屏幕
	if m.currentScreen != "" {
		if currentScreen, exists := m.registry.Get(m.currentScreen); exists {
			if canExit, err := currentScreen.CanExit(ctx); err != nil || !canExit {
				if err != nil {
					return fmt.Errorf("检查屏幕退出条件失败: %v", err)
				}
				return errors.New("当前屏幕不允许退出")
			}

			if err := currentScreen.OnExit(ctx); err != nil {
				m.logger.Warnf("屏幕退出回调失败: %v", err)
			}
		}
	}

	// 2. 清屏（如果配置要求）
	if config.AutoClear {
		m.clearScreen()
	}

	// 3. 显示TopBar（如果配置要求）
	if config.ShowTopBar && m.statusManager != nil {
		statusBar := m.statusManager.RenderStatusBar()
		pterm.Println(statusBar)
		pterm.Println()
	}

	// 4. 进入新屏幕
	if err := screen.OnEnter(ctx); err != nil {
		return fmt.Errorf("屏幕进入回调失败: %v", err)
	}

	// 5. 渲染屏幕内容
	result, err := screen.Render(ctx)
	if err != nil {
		return fmt.Errorf("屏幕渲染失败: %v", err)
	}

	// 6. 显示底部提示（如果配置要求）
	if config.ShowFooterTip {
		ui.ShowStandardTip(config.FooterTipType)
	}

	// 7. 更新当前屏幕
	m.currentScreen = screen.GetName()
	m.logger.Debugf("切换到屏幕: %s", m.currentScreen)

	// 8. 处理屏幕结果
	return m.handleScreenResult(ctx, result)
}

// handleScreenResult 处理屏幕执行结果
func (m *LayoutManager) handleScreenResult(ctx context.Context, result *ScreenResult) error {
	if result == nil {
		return nil
	}

	if result.Error != nil {
		return result.Error
	}

	// 根据动作执行相应操作
	switch NavigationAction(result.Action) {
	case ActionNext:
		if result.NextScreen != "" {
			return m.Show(ctx, result.NextScreen)
		}
		return nil

	case ActionBack:
		return m.Back(ctx)

	case ActionExit:
		return errors.New("exit") // 特殊错误，表示用户要求退出

	case ActionReload:
		return m.Replace(ctx, m.currentScreen)

	default:
		m.logger.Warnf("未知的屏幕动作: %s", result.Action)
		return nil
	}
}

// clearScreen 清屏
func (m *LayoutManager) clearScreen() {
	pterm.Print("\033[2J\033[H")
}

// GetCurrentScreen 获取当前屏幕名称
func (m *LayoutManager) GetCurrentScreen() string {
	return m.currentScreen
}

// GetNavigationStack 获取导航栈（调试用）
func (m *LayoutManager) GetNavigationStack() []string {
	stack := make([]string, len(m.navigationStack))
	copy(stack, m.navigationStack)
	return stack
}

// RunScreenSequence 运行屏幕序列（用于引导流程等）
func (m *LayoutManager) RunScreenSequence(ctx context.Context, screenNames []string) error {
	if len(screenNames) == 0 {
		return errors.New("屏幕序列为空")
	}

	for _, screenName := range screenNames {
		if err := m.Replace(ctx, screenName); err != nil {
			if err.Error() == "exit" {
				return nil // 用户要求退出，正常结束
			}
			return err
		}

		// 检查context是否被取消
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// 继续下一个屏幕
		}
	}

	return nil
}

// SetTimeout 为当前屏幕设置超时（如果支持的话）
func (m *LayoutManager) SetTimeout(ctx context.Context, timeout time.Duration, timeoutAction func()) {
	if timeout <= 0 {
		return
	}

	go func() {
		timer := time.NewTimer(timeout)
		defer timer.Stop()

		select {
		case <-timer.C:
			if timeoutAction != nil {
				timeoutAction()
			}
		case <-ctx.Done():
			return
		}
	}()
}
