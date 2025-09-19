// Package ui - 屏幕切换工具函数
package ui

import (
	"github.com/pterm/pterm"
)

// SwitchToResultPage 切换到结果页面
// 用于"过程页 → 结果页"分屏模式，在加载完成后清屏并重新绘制页面
func SwitchToResultPage(title string) {
	// 重新显示页面头部（内含清屏+状态栏）
	ShowPageHeader()

	// 显示页面标题
	pterm.DefaultSection.Println(title)
	pterm.Println()
}

// ShowEmptyState 显示标准化的空状态
// 所有空状态使用统一的盒子设计
func ShowEmptyState(title, description string, suggestions []string) {
	content := description + "\n\n💡 建议操作：\n"

	for i, suggestion := range suggestions {
		content += pterm.Sprintf("  %d. %s\n", i+1, suggestion)
	}

	// 移除最后的换行符
	content = content[:len(content)-1]

	pterm.DefaultBox.WithTitle(title).WithTitleTopCenter().Println(content)
	pterm.Println()
}

// ShowDataNotFoundState 显示数据未找到的标准状态
func ShowDataNotFoundState(itemType, returnMenu string) {
	ShowEmptyState(
		"📝 "+itemType+"状态",
		"当前没有找到任何"+itemType,
		[]string{
			"返回" + returnMenu,
			"刷新重试",
			"检查系统配置",
		},
	)
}

// ShowNetworkErrorState 显示网络错误的标准状态
func ShowNetworkErrorState(operation, error string) {
	ShowEmptyState(
		"⚠️ 网络错误",
		"无法完成"+operation+"操作\n错误信息: "+error,
		[]string{
			"检查网络连接",
			"重试操作",
			"联系系统管理员",
		},
	)
}

// ShowServiceUnavailableState 显示服务不可用的标准状态
func ShowServiceUnavailableState(serviceName string) {
	ShowEmptyState(
		"🚧 服务不可用",
		serviceName+"服务当前不可用",
		[]string{
			"稍后重试",
			"检查服务状态",
			"查看系统日志",
		},
	)
}
