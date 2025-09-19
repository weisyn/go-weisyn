// Package layout 提供统一的CLI屏幕管理和布局架构
package layout

import (
	"context"
	"time"
)

// ScreenConfig 屏幕配置
type ScreenConfig struct {
	ShowTopBar    bool          // 是否显示顶部状态栏
	ShowFooterTip bool          // 是否显示底部操作提示
	FooterTipType string        // 底部提示类型 (menu, confirm, input等)
	AutoClear     bool          // 进入时是否自动清屏
	Timeout       time.Duration // 屏幕超时时间（0表示无超时）
}

// ScreenResult 屏幕执行结果
type ScreenResult struct {
	Action     string                 // 执行的动作 (next, back, exit, etc.)
	Data       map[string]interface{} // 返回的数据
	NextScreen string                 // 下一个要显示的屏幕名称（可选）
	Error      error                  // 执行错误
}

// Screen 屏幕接口 - 每个"页面"的最小单位
type Screen interface {
	// GetName 获取屏幕名称（用于导航和调试）
	GetName() string

	// GetConfig 获取屏幕配置
	GetConfig() ScreenConfig

	// Render 渲染屏幕内容（不负责清屏和TopBar，由LayoutManager处理）
	Render(ctx context.Context) (*ScreenResult, error)

	// OnEnter 屏幕进入时的回调（可选的准备工作）
	OnEnter(ctx context.Context) error

	// OnExit 屏幕退出时的回调（可选的清理工作）
	OnExit(ctx context.Context) error

	// CanExit 检查是否可以退出屏幕（用于确认对话框等）
	CanExit(ctx context.Context) (bool, error)
}

// BaseScreen 基础屏幕实现，提供默认行为
type BaseScreen struct {
	name   string
	config ScreenConfig
}

// NewBaseScreen 创建基础屏幕
func NewBaseScreen(name string, config ScreenConfig) *BaseScreen {
	return &BaseScreen{
		name:   name,
		config: config,
	}
}

// GetName 获取屏幕名称
func (s *BaseScreen) GetName() string {
	return s.name
}

// GetConfig 获取屏幕配置
func (s *BaseScreen) GetConfig() ScreenConfig {
	return s.config
}

// OnEnter 默认进入回调（空实现）
func (s *BaseScreen) OnEnter(ctx context.Context) error {
	return nil
}

// OnExit 默认退出回调（空实现）
func (s *BaseScreen) OnExit(ctx context.Context) error {
	return nil
}

// CanExit 默认可以退出
func (s *BaseScreen) CanExit(ctx context.Context) (bool, error) {
	return true, nil
}

// Render 需要具体屏幕实现
func (s *BaseScreen) Render(ctx context.Context) (*ScreenResult, error) {
	return &ScreenResult{
		Action: "next",
		Data:   make(map[string]interface{}),
	}, nil
}

// ScreenRegistry 屏幕注册表
type ScreenRegistry struct {
	screens map[string]Screen
}

// NewScreenRegistry 创建屏幕注册表
func NewScreenRegistry() *ScreenRegistry {
	return &ScreenRegistry{
		screens: make(map[string]Screen),
	}
}

// Register 注册屏幕
func (r *ScreenRegistry) Register(screen Screen) {
	r.screens[screen.GetName()] = screen
}

// Get 获取屏幕
func (r *ScreenRegistry) Get(name string) (Screen, bool) {
	screen, exists := r.screens[name]
	return screen, exists
}

// List 列出所有已注册的屏幕
func (r *ScreenRegistry) List() []string {
	names := make([]string, 0, len(r.screens))
	for name := range r.screens {
		names = append(names, name)
	}
	return names
}
