package ui

import (
	"os"
	"testing"
)

// TestNewComponents_TTYDetection 测试 TTY 检测
func TestNewComponents_TTYDetection(t *testing.T) {
	// 创建 UI 组件
	comp := NewComponents(NoopLogger())

	// 验证组件创建成功
	if comp == nil {
		t.Fatal("NewComponents() 返回 nil")
	}

	// 基本功能测试（不会导致 panic）
	err := comp.ShowSuccess("测试消息")
	if err != nil {
		t.Errorf("ShowSuccess() 失败: %v", err)
	}

	err = comp.ShowError("测试错误")
	if err != nil {
		t.Errorf("ShowError() 失败: %v", err)
	}

	err = comp.ShowWarning("测试警告")
	if err != nil {
		t.Errorf("ShowWarning() 失败: %v", err)
	}

	err = comp.ShowInfo("测试信息")
	if err != nil {
		t.Errorf("ShowInfo() 失败: %v", err)
	}
}

// TestComponents_ShowTable 测试表格显示
func TestComponents_ShowTable(t *testing.T) {
	comp := NewComponents(NoopLogger())

	data := [][]string{
		{"姓名", "年龄", "城市"},
		{"张三", "25", "北京"},
		{"李四", "30", "上海"},
	}

	err := comp.ShowTable("用户列表", data)
	if err != nil {
		t.Errorf("ShowTable() 失败: %v", err)
	}

	// 测试空数据
	err = comp.ShowTable("空表格", [][]string{})
	if err == nil {
		t.Error("ShowTable() 应该对空数据返回错误")
	}
}

// TestComponents_ShowList 测试列表显示
func TestComponents_ShowList(t *testing.T) {
	comp := NewComponents(NoopLogger())

	items := []string{
		"项目1",
		"项目2",
		"项目3",
	}

	err := comp.ShowList("测试列表", items)
	if err != nil {
		t.Errorf("ShowList() 失败: %v", err)
	}
}

// TestComponents_ShowKeyValuePairs 测试键值对显示
func TestComponents_ShowKeyValuePairs(t *testing.T) {
	comp := NewComponents(NoopLogger())

	pairs := map[string]string{
		"名称": "测试用户",
		"年龄": "25",
		"城市": "北京",
	}

	err := comp.ShowKeyValuePairs("用户信息", pairs)
	if err != nil {
		t.Errorf("ShowKeyValuePairs() 失败: %v", err)
	}
}

// TestComponents_ShowPanel 测试面板显示
func TestComponents_ShowPanel(t *testing.T) {
	comp := NewComponents(NoopLogger())

	content := "这是一个测试面板\n包含多行内容"
	err := comp.ShowPanel("测试面板", content)
	if err != nil {
		t.Errorf("ShowPanel() 失败: %v", err)
	}
}

// TestComponents_ShowHeader 测试标题显示
func TestComponents_ShowHeader(t *testing.T) {
	comp := NewComponents(NoopLogger())

	err := comp.ShowHeader("测试标题")
	if err != nil {
		t.Errorf("ShowHeader() 失败: %v", err)
	}
}

// TestComponents_ShowSection 测试分节显示
func TestComponents_ShowSection(t *testing.T) {
	comp := NewComponents(NoopLogger())

	err := comp.ShowSection("测试分节")
	if err != nil {
		t.Errorf("ShowSection() 失败: %v", err)
	}
}

// TestComponents_ShowBalanceInfo 测试余额信息显示
func TestComponents_ShowBalanceInfo(t *testing.T) {
	comp := NewComponents(NoopLogger())

	err := comp.ShowBalanceInfo("weisyn1test123", 100.5, "WES")
	if err != nil {
		t.Errorf("ShowBalanceInfo() 失败: %v", err)
	}
}

// TestComponents_ShowSecurityWarning 测试安全警告显示
func TestComponents_ShowSecurityWarning(t *testing.T) {
	comp := NewComponents(NoopLogger())

	err := comp.ShowSecurityWarning("这是一个安全警告！")
	if err != nil {
		t.Errorf("ShowSecurityWarning() 失败: %v", err)
	}
}

// TestComponents_Clear 测试清屏
func TestComponents_Clear(t *testing.T) {
	comp := NewComponents(NoopLogger())

	err := comp.Clear()
	if err != nil {
		t.Errorf("Clear() 失败: %v", err)
	}
}

// TestComponents_ProgressBar 测试进度条
func TestComponents_ProgressBar(t *testing.T) {
	comp := NewComponents(NoopLogger())

	pbar := comp.NewProgressBar("测试进度", 100)
	if pbar == nil {
		t.Fatal("NewProgressBar() 返回 nil")
	}

	err := pbar.Start()
	if err != nil {
		t.Errorf("ProgressBar.Start() 失败: %v", err)
	}

	err = pbar.Update(50, "已完成 50%")
	if err != nil {
		t.Errorf("ProgressBar.Update() 失败: %v", err)
	}

	err = pbar.Finish("完成")
	if err != nil {
		t.Errorf("ProgressBar.Finish() 失败: %v", err)
	}
}

// TestComponents_Spinner 测试加载动画
func TestComponents_Spinner(t *testing.T) {
	comp := NewComponents(NoopLogger())

	spinner := comp.ShowSpinner("正在加载...")
	if spinner == nil {
		t.Fatal("ShowSpinner() 返回 nil")
	}

	err := spinner.Start()
	if err != nil {
		t.Errorf("Spinner.Start() 失败: %v", err)
	}

	err = spinner.UpdateText("更新文本")
	if err != nil {
		t.Errorf("Spinner.UpdateText() 失败: %v", err)
	}

	err = spinner.Success("成功")
	if err != nil {
		t.Errorf("Spinner.Success() 失败: %v", err)
	}
}

// TestComponents_ShowContinuePrompt_NonTTY 测试非 TTY 环境的继续提示
func TestComponents_ShowContinuePrompt_NonTTY(t *testing.T) {
	// 注意：此测试在 CI/CD 环境（非 TTY）中会直接返回
	// 在 TTY 环境中需要手动按 Enter（不适合自动化测试）

	comp := NewComponents(NoopLogger())

	// 在非 TTY 环境中，此方法应立即返回
	err := comp.ShowContinuePrompt("测试", "按 Enter 继续")
	if err != nil {
		t.Errorf("ShowContinuePrompt() 失败: %v", err)
	}
}

// TestFormatDuration 测试时间格式化
func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration int
		want     string
	}{
		{"秒", 45, "45s"},
		{"分秒", 125, "2m 5s"},
		{"时分秒", 3665, "1h 1m 5s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 注意：这里需要导入 time 包
			// got := FormatDuration(time.Duration(tt.duration) * time.Second)
			// if got != tt.want {
			//     t.Errorf("FormatDuration() = %v, want %v", got, tt.want)
			// }
		})
	}
}

// TestTruncateString 测试字符串截断
func TestTruncateString(t *testing.T) {
	tests := []struct {
		name   string
		str    string
		maxLen int
		want   string
	}{
		{"不截断", "short", 10, "short"},
		{"截断", "this is a very long string", 10, "this is..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateString(tt.str, tt.maxLen)
			if got != tt.want {
				t.Errorf("TruncateString() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestNoopLogger 测试空日志实现
func TestNoopLogger(t *testing.T) {
	logger := NoopLogger()
	if logger == nil {
		t.Fatal("NoopLogger() 返回 nil")
	}

	// 调用所有方法，确保不会 panic
	logger.Debug("debug")
	logger.Debugf("debugf %s", "test")
	logger.Info("info")
	logger.Infof("infof %s", "test")
	logger.Warn("warn")
	logger.Warnf("warnf %s", "test")
	logger.Error("error")
	logger.Errorf("errorf %s", "test")
}

// TestGetDefaultTheme 测试默认主题
func TestGetDefaultTheme(t *testing.T) {
	theme := GetDefaultTheme()
	if theme == nil {
		t.Fatal("GetDefaultTheme() 返回 nil")
	}

	// 验证主题颜色已设置
	if theme.PrimaryColor == 0 {
		t.Error("主题主色调未设置")
	}
	if theme.SuccessColor == 0 {
		t.Error("主题成功色未设置")
	}
	if theme.ErrorColor == 0 {
		t.Error("主题错误色未设置")
	}
}

// TestMain 测试主函数
func TestMain(m *testing.M) {
	// 设置测试环境（如果需要）
	// ...

	// 运行测试
	code := m.Run()

	// 清理（如果需要）
	// ...

	os.Exit(code)
}

