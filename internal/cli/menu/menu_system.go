// Package menu 提供CLI的双层菜单系统
package menu

import (
	"context"
	"fmt"
	"strings"

	"github.com/weisyn/v1/internal/cli/permissions"
	"github.com/weisyn/v1/internal/cli/ui"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// MenuLevel 菜单层级
type MenuLevel int

const (
	// SystemLevel 系统级菜单（公开功能）
	SystemLevel MenuLevel = iota
	// UserLevel 用户级菜单（需要私钥）
	UserLevel
	// AdminLevel 管理员级菜单
	AdminLevel
)

// String 返回菜单层级的字符串表示
func (ml MenuLevel) String() string {
	switch ml {
	case SystemLevel:
		return "System"
	case UserLevel:
		return "User"
	case AdminLevel:
		return "Admin"
	default:
		return "Unknown"
	}
}

// MenuItemType 菜单项类型
type MenuItemType int

const (
	// ActionItem 动作菜单项（执行功能）
	ActionItem MenuItemType = iota
	// SubMenuItem 子菜单项（进入子菜单）
	SubMenuItem
	// SeparatorItem 分隔符（仅显示）
	SeparatorItem
	// InfoItem 信息项（仅显示信息）
	InfoItem
)

// MenuAction 菜单动作函数
type MenuAction func(ctx context.Context) error

// MenuItem 菜单项
type MenuItem struct {
	ID          string       // 菜单项唯一标识
	Title       string       // 显示标题
	Description string       // 描述信息
	Icon        string       // 图标（emoji）
	Type        MenuItemType // 菜单项类型
	Level       MenuLevel    // 所需权限级别
	Action      MenuAction   // 执行动作（ActionItem类型）
	SubMenu     *Menu        // 子菜单（SubMenuItem类型）
	Enabled     bool         // 是否启用
	Visible     bool         // 是否可见
	Order       int          // 排序权重
}

// Menu 菜单定义
type Menu struct {
	ID          string      // 菜单唯一标识
	Title       string      // 菜单标题
	Description string      // 菜单描述
	Items       []*MenuItem // 菜单项列表
	Parent      *Menu       // 父菜单
	Level       MenuLevel   // 菜单级别
}

// MenuContext 菜单上下文
type MenuContext struct {
	CurrentMenu       *Menu                       // 当前菜单
	MenuStack         []*Menu                     // 菜单栈（用于导航）
	UserPermissions   permissions.PermissionLevel // 用户权限级别
	PermissionManager *permissions.Manager        // 权限管理器
	AdditionalData    map[string]interface{}      // 额外数据
}

// DualMenuSystem 双层菜单系统接口
type DualMenuSystem interface {
	// 菜单管理
	RegisterMenu(menu *Menu) error
	GetMenu(menuID string) *Menu
	GetMainMenu() *Menu

	// 菜单显示和导航
	ShowMainMenu(ctx context.Context) error
	ShowMenu(ctx context.Context, menuID string) error
	NavigateToMenu(ctx context.Context, menuID string) error
	NavigateBack(ctx context.Context) error

	// 权限管理
	FilterMenuByPermissions(menu *Menu, userLevel permissions.PermissionLevel) *Menu
	UpdatePermissions(userLevel permissions.PermissionLevel)

	// 菜单定制
	SetMenuCustomizer(customizer MenuCustomizer)
	GetMenuContext() *MenuContext
}

// MenuCustomizer 菜单定制器接口
type MenuCustomizer interface {
	// 定制菜单显示
	CustomizeMenu(menu *Menu, context *MenuContext) *Menu

	// 定制菜单项显示
	CustomizeMenuItem(item *MenuItem, context *MenuContext) *MenuItem

	// 定制菜单标题
	CustomizeMenuTitle(menu *Menu, context *MenuContext) string
}

// dualMenuSystem 双层菜单系统实现
type dualMenuSystem struct {
	logger            log.Logger
	ui                ui.Components
	permissionManager *permissions.Manager

	// 菜单注册表
	menus    map[string]*Menu
	mainMenu *Menu

	// 菜单状态
	context    *MenuContext
	customizer MenuCustomizer
}

// NewDualMenuSystem 创建双层菜单系统
func NewDualMenuSystem(
	logger log.Logger,
	uiComponents ui.Components,
	permissionManager *permissions.Manager,
) DualMenuSystem {
	system := &dualMenuSystem{
		logger:            logger,
		ui:                uiComponents,
		permissionManager: permissionManager,
		menus:             make(map[string]*Menu),
		context: &MenuContext{
			MenuStack:      make([]*Menu, 0),
			AdditionalData: make(map[string]interface{}),
		},
	}

	// 初始化默认菜单
	system.initializeDefaultMenus()

	return system
}

// RegisterMenu 注册菜单
func (dms *dualMenuSystem) RegisterMenu(menu *Menu) error {
	if menu.ID == "" {
		return fmt.Errorf("菜单ID不能为空")
	}

	if _, exists := dms.menus[menu.ID]; exists {
		return fmt.Errorf("菜单已存在: %s", menu.ID)
	}

	dms.menus[menu.ID] = menu
	dms.logger.Info(fmt.Sprintf("菜单已注册: id=%s, title=%s", menu.ID, menu.Title))

	return nil
}

// GetMenu 获取菜单
func (dms *dualMenuSystem) GetMenu(menuID string) *Menu {
	return dms.menus[menuID]
}

// GetMainMenu 获取主菜单
func (dms *dualMenuSystem) GetMainMenu() *Menu {
	return dms.mainMenu
}

// ShowMainMenu 显示主菜单
func (dms *dualMenuSystem) ShowMainMenu(ctx context.Context) error {
	// 更新权限状态
	dms.updateContextPermissions()

	if dms.mainMenu == nil {
		return fmt.Errorf("主菜单未初始化")
	}

	// 重置菜单栈
	dms.context.MenuStack = []*Menu{}
	dms.context.CurrentMenu = dms.mainMenu

	return dms.showCurrentMenu(ctx)
}

// ShowMenu 显示指定菜单
func (dms *dualMenuSystem) ShowMenu(ctx context.Context, menuID string) error {
	menu := dms.GetMenu(menuID)
	if menu == nil {
		return fmt.Errorf("菜单不存在: %s", menuID)
	}

	// 更新权限状态
	dms.updateContextPermissions()

	dms.context.CurrentMenu = menu
	return dms.showCurrentMenu(ctx)
}

// showCurrentMenu 显示当前菜单
func (dms *dualMenuSystem) showCurrentMenu(ctx context.Context) error {
	menu := dms.context.CurrentMenu
	if menu == nil {
		return fmt.Errorf("当前菜单为空")
	}

	// 根据权限过滤菜单
	filteredMenu := dms.FilterMenuByPermissions(menu, dms.context.UserPermissions)

	// 应用定制器
	if dms.customizer != nil {
		filteredMenu = dms.customizer.CustomizeMenu(filteredMenu, dms.context)
	}

	// 显示菜单标题和状态
	dms.showMenuHeader(filteredMenu)

	// 准备菜单选项
	options := dms.prepareMenuOptions(filteredMenu)
	if len(options) == 0 {
		dms.ui.ShowWarning("当前权限级别下没有可用的菜单项")
		return nil
	}

	// 添加导航选项
	if len(dms.context.MenuStack) > 0 {
		options = append(options, "🔙 返回上级菜单")
	}
	options = append(options, "❌ 退出")

	// 显示菜单并获取用户选择
	menuTitle := dms.getMenuDisplayTitle(filteredMenu)
	selectedIndex, err := dms.ui.ShowMenu(menuTitle, options)
	if err != nil {
		return fmt.Errorf("菜单选择失败: %v", err)
	}

	// 处理用户选择
	return dms.handleMenuSelection(ctx, filteredMenu, selectedIndex)
}

// prepareMenuOptions 准备菜单选项
func (dms *dualMenuSystem) prepareMenuOptions(menu *Menu) []string {
	visibleItems := dms.getVisibleItems(menu)
	options := make([]string, 0, len(visibleItems))

	for _, item := range visibleItems {
		if item.Type == SeparatorItem {
			continue // 分隔符不作为选项
		}

		optionText := dms.formatMenuOption(item)
		options = append(options, optionText)
	}

	return options
}

// getVisibleItems 获取可见的菜单项
func (dms *dualMenuSystem) getVisibleItems(menu *Menu) []*MenuItem {
	visibleItems := make([]*MenuItem, 0)

	for _, item := range menu.Items {
		if !item.Visible {
			continue
		}

		// 检查权限级别
		if !dms.hasPermissionForItem(item) {
			continue
		}

		// 应用定制器
		if dms.customizer != nil {
			item = dms.customizer.CustomizeMenuItem(item, dms.context)
			if item == nil {
				continue
			}
		}

		visibleItems = append(visibleItems, item)
	}

	return visibleItems
}

// hasPermissionForItem 检查是否有权限访问菜单项
func (dms *dualMenuSystem) hasPermissionForItem(item *MenuItem) bool {
	switch item.Level {
	case SystemLevel:
		return dms.context.UserPermissions >= permissions.SystemOnly
	case UserLevel:
		return dms.context.UserPermissions >= permissions.FullAccess
	case AdminLevel:
		// 简化实现：管理员权限检查
		return dms.context.UserPermissions >= permissions.FullAccess
	default:
		return false
	}
}

// formatMenuOption 格式化菜单选项
func (dms *dualMenuSystem) formatMenuOption(item *MenuItem) string {
	var parts []string

	// 添加图标
	if item.Icon != "" {
		parts = append(parts, item.Icon)
	}

	// 添加标题
	parts = append(parts, item.Title)

	// 添加状态指示
	if !item.Enabled {
		parts = append(parts, "(禁用)")
	}

	// 添加子菜单指示
	if item.Type == SubMenuItem {
		parts = append(parts, "→")
	}

	return strings.Join(parts, " ")
}

// showMenuHeader 显示菜单头部信息
func (dms *dualMenuSystem) showMenuHeader(menu *Menu) {
	// 显示菜单标题
	title := dms.getMenuDisplayTitle(menu)
	dms.ui.ShowHeader(title)

	// 显示权限状态
	userContext := dms.permissionManager.GetUserContext()
	dms.ui.ShowPermissionStatus("用户状态", userContext.GetDisplayStatus())

	// 显示菜单描述
	if menu.Description != "" {
		dms.ui.ShowInfo(menu.Description)
	}

	// 显示导航路径
	if len(dms.context.MenuStack) > 0 {
		path := dms.buildNavigationPath()
		dms.ui.ShowInfo(fmt.Sprintf("📍 当前位置: %s", path))
	}
}

// getMenuDisplayTitle 获取菜单显示标题
func (dms *dualMenuSystem) getMenuDisplayTitle(menu *Menu) string {
	if dms.customizer != nil {
		customTitle := dms.customizer.CustomizeMenuTitle(menu, dms.context)
		if customTitle != "" {
			return customTitle
		}
	}

	return menu.Title
}

// buildNavigationPath 构建导航路径
func (dms *dualMenuSystem) buildNavigationPath() string {
	pathParts := make([]string, 0, len(dms.context.MenuStack)+1)

	for _, menu := range dms.context.MenuStack {
		pathParts = append(pathParts, menu.Title)
	}

	if dms.context.CurrentMenu != nil {
		pathParts = append(pathParts, dms.context.CurrentMenu.Title)
	}

	return strings.Join(pathParts, " → ")
}

// handleMenuSelection 处理菜单选择
func (dms *dualMenuSystem) handleMenuSelection(ctx context.Context, menu *Menu, selectedIndex int) error {
	visibleItems := dms.getVisibleItems(menu)

	// 计算实际选择的项目（排除分隔符）
	actualItemIndex := -1
	itemCount := 0

	for i, item := range visibleItems {
		if item.Type != SeparatorItem {
			if itemCount == selectedIndex {
				actualItemIndex = i
				break
			}
			itemCount++
		}
	}

	// 检查是否选择了导航选项
	navigationOptionsStart := len(visibleItems)
	for _, item := range visibleItems {
		if item.Type == SeparatorItem {
			navigationOptionsStart--
		}
	}

	if selectedIndex >= navigationOptionsStart {
		return dms.handleNavigationSelection(ctx, selectedIndex-navigationOptionsStart)
	}

	// 检查选择是否有效
	if actualItemIndex < 0 || actualItemIndex >= len(visibleItems) {
		return fmt.Errorf("无效的菜单选择")
	}

	selectedItem := visibleItems[actualItemIndex]

	// 检查菜单项是否启用
	if !selectedItem.Enabled {
		dms.ui.ShowWarning("该功能暂时不可用")
		return dms.showCurrentMenu(ctx) // 重新显示菜单
	}

	// 根据菜单项类型处理
	switch selectedItem.Type {
	case ActionItem:
		return dms.executeAction(ctx, selectedItem)
	case SubMenuItem:
		return dms.navigateToSubMenu(ctx, selectedItem)
	case InfoItem:
		dms.ui.ShowInfo(selectedItem.Description)
		return dms.showCurrentMenu(ctx) // 重新显示菜单
	default:
		return fmt.Errorf("不支持的菜单项类型: %d", selectedItem.Type)
	}
}

// handleNavigationSelection 处理导航选择
func (dms *dualMenuSystem) handleNavigationSelection(ctx context.Context, navIndex int) error {
	switch navIndex {
	case 0: // 返回上级菜单（如果存在）
		if len(dms.context.MenuStack) > 0 {
			return dms.NavigateBack(ctx)
		}
		fallthrough
	case 1: // 退出
		return fmt.Errorf("用户选择退出")
	default:
		return fmt.Errorf("无效的导航选择")
	}
}

// executeAction 执行动作
func (dms *dualMenuSystem) executeAction(ctx context.Context, item *MenuItem) error {
	if item.Action == nil {
		dms.ui.ShowWarning("该功能尚未实现")
		return dms.showCurrentMenu(ctx)
	}

	dms.logger.Info(fmt.Sprintf("执行菜单动作: item=%s", item.ID))

	// 执行动作
	err := item.Action(ctx)
	if err != nil {
		dms.ui.ShowError(fmt.Sprintf("操作执行失败: %v", err))
	}

	// 重新显示菜单（除非是退出操作）
	if err == nil || !strings.Contains(err.Error(), "退出") {
		return dms.showCurrentMenu(ctx)
	}

	return err
}

// navigateToSubMenu 导航到子菜单
func (dms *dualMenuSystem) navigateToSubMenu(ctx context.Context, item *MenuItem) error {
	if item.SubMenu == nil {
		return fmt.Errorf("子菜单未定义")
	}

	// 将当前菜单推入栈
	dms.context.MenuStack = append(dms.context.MenuStack, dms.context.CurrentMenu)
	dms.context.CurrentMenu = item.SubMenu

	return dms.showCurrentMenu(ctx)
}

// NavigateToMenu 导航到指定菜单
func (dms *dualMenuSystem) NavigateToMenu(ctx context.Context, menuID string) error {
	menu := dms.GetMenu(menuID)
	if menu == nil {
		return fmt.Errorf("菜单不存在: %s", menuID)
	}

	// 将当前菜单推入栈
	if dms.context.CurrentMenu != nil {
		dms.context.MenuStack = append(dms.context.MenuStack, dms.context.CurrentMenu)
	}

	dms.context.CurrentMenu = menu
	return dms.showCurrentMenu(ctx)
}

// NavigateBack 返回上级菜单
func (dms *dualMenuSystem) NavigateBack(ctx context.Context) error {
	if len(dms.context.MenuStack) == 0 {
		return fmt.Errorf("已经在顶级菜单")
	}

	// 从栈中弹出上级菜单
	lastIndex := len(dms.context.MenuStack) - 1
	dms.context.CurrentMenu = dms.context.MenuStack[lastIndex]
	dms.context.MenuStack = dms.context.MenuStack[:lastIndex]

	return dms.showCurrentMenu(ctx)
}

// updateContextPermissions 更新上下文权限信息
func (dms *dualMenuSystem) updateContextPermissions() {
	userContext := dms.permissionManager.GetUserContext()
	dms.context.UserPermissions = userContext.PermissionLevel
	dms.context.PermissionManager = dms.permissionManager
}

// FilterMenuByPermissions 根据权限过滤菜单
func (dms *dualMenuSystem) FilterMenuByPermissions(menu *Menu, userLevel permissions.PermissionLevel) *Menu {
	// 创建菜单副本
	filteredMenu := &Menu{
		ID:          menu.ID,
		Title:       menu.Title,
		Description: menu.Description,
		Parent:      menu.Parent,
		Level:       menu.Level,
		Items:       make([]*MenuItem, 0),
	}

	// 过滤菜单项
	for _, item := range menu.Items {
		if dms.shouldIncludeItem(item, userLevel) {
			filteredMenu.Items = append(filteredMenu.Items, item)
		}
	}

	return filteredMenu
}

// shouldIncludeItem 判断是否应该包含菜单项
func (dms *dualMenuSystem) shouldIncludeItem(item *MenuItem, userLevel permissions.PermissionLevel) bool {
	// 检查可见性
	if !item.Visible {
		return false
	}

	// 检查权限级别
	switch item.Level {
	case SystemLevel:
		return userLevel >= permissions.SystemOnly
	case UserLevel:
		return userLevel >= permissions.FullAccess
	case AdminLevel:
		return userLevel >= permissions.FullAccess
	default:
		return false
	}
}

// UpdatePermissions 更新权限
func (dms *dualMenuSystem) UpdatePermissions(userLevel permissions.PermissionLevel) {
	dms.context.UserPermissions = userLevel
	dms.logger.Info(fmt.Sprintf("菜单系统权限已更新: level=%s", userLevel.String()))
}

// SetMenuCustomizer 设置菜单定制器
func (dms *dualMenuSystem) SetMenuCustomizer(customizer MenuCustomizer) {
	dms.customizer = customizer
	dms.logger.Info("菜单定制器已设置")
}

// GetMenuContext 获取菜单上下文
func (dms *dualMenuSystem) GetMenuContext() *MenuContext {
	return dms.context
}
