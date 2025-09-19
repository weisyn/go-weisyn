package menu

import (
	"fmt"
	"strings"

	"github.com/weisyn/v1/internal/cli/permissions"
)

// DefaultMenuCustomizer 默认菜单定制器实现
type DefaultMenuCustomizer struct {
	showPermissionHints bool
	showShortcuts       bool
	compactMode         bool
}

// NewDefaultMenuCustomizer 创建默认菜单定制器
func NewDefaultMenuCustomizer() MenuCustomizer {
	return &DefaultMenuCustomizer{
		showPermissionHints: true,
		showShortcuts:       false,
		compactMode:         false,
	}
}

// CustomizeMenu 定制菜单显示
func (dmc *DefaultMenuCustomizer) CustomizeMenu(menu *Menu, context *MenuContext) *Menu {
	// 创建菜单副本
	customizedMenu := *menu
	customizedItems := make([]*MenuItem, 0, len(menu.Items))

	for _, item := range menu.Items {
		customizedItem := dmc.CustomizeMenuItem(item, context)
		if customizedItem != nil {
			customizedItems = append(customizedItems, customizedItem)
		}
	}

	customizedMenu.Items = customizedItems

	// 如果是紧凑模式，移除分隔符
	if dmc.compactMode {
		customizedMenu.Items = dmc.removeSeparators(customizedMenu.Items)
	}

	return &customizedMenu
}

// CustomizeMenuItem 定制菜单项显示
func (dmc *DefaultMenuCustomizer) CustomizeMenuItem(item *MenuItem, context *MenuContext) *MenuItem {
	if item == nil {
		return nil
	}

	// 创建菜单项副本
	customizedItem := *item

	// 添加权限提示
	if dmc.showPermissionHints {
		customizedItem.Title = dmc.addPermissionHint(item, context)
	}

	// 添加快捷键提示
	if dmc.showShortcuts {
		customizedItem.Title = dmc.addShortcutHint(customizedItem.Title, item)
	}

	// 根据用户状态调整可用性
	customizedItem.Enabled = dmc.isItemEnabled(item, context)

	return &customizedItem
}

// CustomizeMenuTitle 定制菜单标题
func (dmc *DefaultMenuCustomizer) CustomizeMenuTitle(menu *Menu, context *MenuContext) string {
	title := menu.Title

	// 添加权限级别指示
	if context.UserPermissions != permissions.UNKNOWN {
		switch context.UserPermissions {
		case permissions.SystemOnly:
			title += " (系统级)"
		case permissions.FullAccess:
			title += " (完全访问)"
		}
	}

	// 添加钱包状态指示
	if context.PermissionManager != nil {
		userContext := context.PermissionManager.GetUserContext()
		if userContext.HasWallets {
			if userContext.IsWalletUnlocked {
				title += " 🔓"
			} else {
				title += " 🔐"
			}
		} else {
			title += " 💳❌"
		}
	}

	return title
}

// addPermissionHint 添加权限提示
func (dmc *DefaultMenuCustomizer) addPermissionHint(item *MenuItem, context *MenuContext) string {
	title := item.Title

	// 根据权限级别添加提示
	switch item.Level {
	case SystemLevel:
		// 系统级功能不需要特殊提示
		break
	case UserLevel:
		if context.UserPermissions < permissions.FullAccess {
			title += " (需要解锁钱包)"
		}
	case AdminLevel:
		if context.UserPermissions < permissions.FullAccess {
			title += " (需要管理员权限)"
		}
	}

	return title
}

// addShortcutHint 添加快捷键提示
func (dmc *DefaultMenuCustomizer) addShortcutHint(title string, item *MenuItem) string {
	// 简化实现：为常用功能添加快捷键提示
	shortcuts := map[string]string{
		"check_balance": "[Ctrl+B]",
		"send_transfer": "[Ctrl+T]",
		"mining_status": "[Ctrl+M]",
		"create_wallet": "[Ctrl+W]",
		"node_status":   "[Ctrl+N]",
		"latest_blocks": "[Ctrl+L]",
	}

	if shortcut, exists := shortcuts[item.ID]; exists {
		return fmt.Sprintf("%s %s", title, shortcut)
	}

	return title
}

// isItemEnabled 判断菜单项是否应该启用
func (dmc *DefaultMenuCustomizer) isItemEnabled(item *MenuItem, context *MenuContext) bool {
	// 如果原本就禁用，保持禁用
	if !item.Enabled {
		return false
	}

	// 检查权限要求
	switch item.Level {
	case SystemLevel:
		return context.UserPermissions >= permissions.SystemOnly
	case UserLevel:
		return context.UserPermissions >= permissions.FullAccess
	case AdminLevel:
		// 管理员权限检查
		return context.UserPermissions >= permissions.FullAccess
	default:
		return false
	}
}

// removeSeparators 移除分隔符项
func (dmc *DefaultMenuCustomizer) removeSeparators(items []*MenuItem) []*MenuItem {
	filtered := make([]*MenuItem, 0, len(items))

	for _, item := range items {
		if item.Type != SeparatorItem {
			filtered = append(filtered, item)
		}
	}

	return filtered
}

// SetShowPermissionHints 设置是否显示权限提示
func (dmc *DefaultMenuCustomizer) SetShowPermissionHints(show bool) {
	dmc.showPermissionHints = show
}

// SetShowShortcuts 设置是否显示快捷键
func (dmc *DefaultMenuCustomizer) SetShowShortcuts(show bool) {
	dmc.showShortcuts = show
}

// SetCompactMode 设置紧凑模式
func (dmc *DefaultMenuCustomizer) SetCompactMode(compact bool) {
	dmc.compactMode = compact
}

// PermissionAwareCustomizer 权限感知定制器
type PermissionAwareCustomizer struct {
	*DefaultMenuCustomizer
	enhanceUnavailable bool
}

// NewPermissionAwareCustomizer 创建权限感知定制器
func NewPermissionAwareCustomizer() MenuCustomizer {
	return &PermissionAwareCustomizer{
		DefaultMenuCustomizer: &DefaultMenuCustomizer{
			showPermissionHints: true,
			showShortcuts:       false,
			compactMode:         false,
		},
		enhanceUnavailable: true,
	}
}

// CustomizeMenuItem 定制菜单项显示（权限感知版本）
func (pac *PermissionAwareCustomizer) CustomizeMenuItem(item *MenuItem, context *MenuContext) *MenuItem {
	// 先使用基础定制
	customizedItem := pac.DefaultMenuCustomizer.CustomizeMenuItem(item, context)
	if customizedItem == nil {
		return nil
	}

	// 增强不可用项的显示
	if pac.enhanceUnavailable && !customizedItem.Enabled {
		customizedItem.Title = pac.enhanceUnavailableItem(customizedItem.Title, item, context)
	}

	return customizedItem
}

// enhanceUnavailableItem 增强不可用项的显示
func (pac *PermissionAwareCustomizer) enhanceUnavailableItem(title string, item *MenuItem, context *MenuContext) string {
	// 添加不可用原因说明
	switch item.Level {
	case UserLevel:
		if context.UserPermissions < permissions.FullAccess {
			title += " ⚠️"
		}
	case AdminLevel:
		if context.UserPermissions < permissions.FullAccess {
			title += " 🚫"
		}
	}

	// 如果是功能未实现
	if strings.Contains(item.Description, "开发中") || !item.Enabled {
		title += " 🚧"
	}

	return title
}

// SetEnhanceUnavailable 设置是否增强不可用项显示
func (pac *PermissionAwareCustomizer) SetEnhanceUnavailable(enhance bool) {
	pac.enhanceUnavailable = enhance
}

// ThemeCustomizer 主题定制器
type ThemeCustomizer struct {
	*DefaultMenuCustomizer
	theme string
}

// NewThemeCustomizer 创建主题定制器
func NewThemeCustomizer(theme string) MenuCustomizer {
	return &ThemeCustomizer{
		DefaultMenuCustomizer: &DefaultMenuCustomizer{
			showPermissionHints: true,
			showShortcuts:       false,
			compactMode:         false,
		},
		theme: theme,
	}
}

// CustomizeMenuTitle 定制菜单标题（主题版本）
func (tc *ThemeCustomizer) CustomizeMenuTitle(menu *Menu, context *MenuContext) string {
	title := tc.DefaultMenuCustomizer.CustomizeMenuTitle(menu, context)

	// 根据主题添加装饰
	switch tc.theme {
	case "minimal":
		// 最简主题，只保留基本信息
		return menu.Title
	case "colorful":
		// 彩色主题，添加更多emoji
		return fmt.Sprintf("🌈 %s 🌈", title)
	case "professional":
		// 专业主题，添加边框
		return fmt.Sprintf("▎%s", title)
	default:
		return title
	}
}

// SetTheme 设置主题
func (tc *ThemeCustomizer) SetTheme(theme string) {
	tc.theme = theme
}
