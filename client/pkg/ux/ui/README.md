# UI 组件库

提供可复用的 TTY 交互组件，基于 pterm 库构建。

## 📋 功能特性

- **自动 TTY 检测**：自动检测是否为 TTY 环境，非 TTY 环境禁用交互功能
- **丰富的组件**：表格、列表、菜单、输入框、进度条、加载动画等
- **主题支持**：内置默认主题，可自定义颜色
- **日志适配**：通过接口适配任意日志实现

## 🚀 快速开始

### 创建 UI 组件实例

```go
package main

import (
    "github.com/weisyn/v1/client/pkg/ux/ui"
)

func main() {
    // 创建 UI 组件（传入 nil 或 NoopLogger() 表示不输出日志）
    uiComponents := ui.NewComponents(ui.NoopLogger())
    
    // 或者传入自定义日志实现
    // uiComponents := ui.NewComponents(myLogger)
}
```

### 基本使用示例

#### 1. 显示消息

```go
// 成功消息
uiComponents.ShowSuccess("操作成功！")

// 错误消息
uiComponents.ShowError("操作失败：连接超时")

// 警告消息
uiComponents.ShowWarning("注意：余额不足")

// 信息消息
uiComponents.ShowInfo("正在处理...")
```

#### 2. 显示表格

```go
data := [][]string{
    {"姓名", "年龄", "城市"},        // 表头
    {"张三", "25", "北京"},
    {"李四", "30", "上海"},
}

uiComponents.ShowTable("用户列表", data)
```

#### 3. 显示菜单（交互式选择）

```go
options := []string{
    "查询余额",
    "创建钱包",
    "转账",
    "退出",
}

selectedIndex, err := uiComponents.ShowMenu("主菜单", options)
if err != nil {
    // 处理错误
}

switch selectedIndex {
case 0:
    // 查询余额
case 1:
    // 创建钱包
// ...
}
```

#### 4. 输入对话框

```go
// 普通输入
name, err := uiComponents.ShowInputDialog("输入姓名", "请输入您的姓名", false)

// 密码输入（隐藏显示）
password, err := uiComponents.ShowInputDialog("输入密码", "请输入密码", true)
```

#### 5. 确认对话框

```go
// 默认值为 No
confirmed, err := uiComponents.ShowConfirmDialog("确认删除", "确定要删除此钱包吗？")

// 指定默认值
confirmed, err := uiComponents.ShowConfirmDialogWithDefault("确认", "继续吗？", true)
```

#### 6. 进度条

```go
// 创建进度条
progressBar := uiComponents.NewProgressBar("下载文件", 100)
progressBar.Start()

for i := 0; i < 100; i++ {
    time.Sleep(10 * time.Millisecond)
    progressBar.Update(i, fmt.Sprintf("已完成 %d%%", i))
}

progressBar.Finish("下载完成")
```

#### 7. 加载动画

```go
spinner := uiComponents.ShowSpinner("正在连接...")
spinner.Start()

// 执行耗时操作
time.Sleep(2 * time.Second)

// 成功停止
spinner.Success("连接成功！")

// 或失败停止
// spinner.Fail("连接失败")
```

#### 8. 显示面板

```go
content := "钱包地址: weisyn1abc...\n余额: 100.5 WES"
uiComponents.ShowPanel("钱包信息", content)
```

## 📚 组件列表

### 数据展示组件

- `ShowTable(title, data)` - 显示表格
- `ShowList(title, items)` - 显示列表
- `ShowKeyValuePairs(title, pairs)` - 显示键值对

### 交互选择组件

- `ShowMenu(title, options)` - 显示菜单（返回选中索引）
- `ShowConfirmDialog(title, message)` - 确认对话框
- `ShowConfirmDialogWithDefault(title, message, defaultValue)` - 确认对话框（指定默认值）
- `ShowInputDialog(title, prompt, isPassword)` - 输入对话框
- `ShowContinuePrompt(title, message)` - "按 Enter 继续"提示（非 TTY 直接返回）

### 进度反馈组件

- `NewProgressBar(title, total)` - 创建进度条
- `ShowSpinner(message)` - 显示加载动画
- `ShowLoadingMessage(message)` - 显示加载消息

### 状态显示组件

- `ShowSuccess(message)` - 显示成功消息
- `ShowError(message)` - 显示错误消息
- `ShowWarning(message)` - 显示警告消息
- `ShowInfo(message)` - 显示信息消息

### 面板和布局组件

- `ShowPanel(title, content)` - 显示面板
- `ShowSideBySidePanels(left, right)` - 显示并排面板
- `ShowHeader(text)` - 显示标题
- `ShowSection(text)` - 显示分区标题

### 特殊组件

- `ShowPermissionStatus(level, status)` - 显示权限状态
- `ShowSecurityWarning(message)` - 显示安全警告
- `ShowWalletSelector(wallets)` - 显示钱包选择器
- `ShowBalanceInfo(address, balance, tokenSymbol)` - 显示余额信息

### 屏幕控制组件

- `Clear()` - 清屏

## 🔌 Logger 适配器

UI 组件接受一个 `Logger` 接口，可以适配任意日志实现：

```go
type Logger interface {
    Debug(msg string)
    Debugf(format string, args ...interface{})
    Info(msg string)
    Infof(format string, args ...interface{})
    Warn(msg string)
    Warnf(format string, args ...interface{})
    Error(msg string)
    Errorf(format string, args ...interface{})
}
```

如果不需要日志，可以使用内置的 `NoopLogger()`：

```go
uiComponents := ui.NewComponents(ui.NoopLogger())
```

## 🎨 主题定制

可以通过 `GetDefaultTheme()` 获取默认主题，或创建自定义主题：

```go
theme := ui.GetDefaultTheme()
// 修改颜色
theme.PrimaryColor = pterm.FgBlue
theme.SuccessColor = pterm.FgGreen
```

## ⚠️ TTY 环境检测

UI 组件会自动检测是否为 TTY 环境：

- **TTY 环境**：启用完整交互功能（颜色、进度条、菜单等）
- **非 TTY 环境**：自动禁用交互功能，适用于管道、重定向、CI/CD 等场景

```bash
# TTY 环境（直接运行）
./wes account list

# 非 TTY 环境（管道）
./wes account list | grep "default"

# 非 TTY 环境（重定向）
./wes account list > output.txt
```

## 📝 工具函数

### FormatDuration

格式化时间段：

```go
duration := 3665 * time.Second
formatted := ui.FormatDuration(duration) // "1h 1m 5s"
```

### TruncateString

截断字符串：

```go
text := "很长的字符串内容..."
truncated := ui.TruncateString(text, 20) // "很长的字符串内容..."
```

## 📖 完整示例

```go
package main

import (
    "context"
    "fmt"
    
    "github.com/weisyn/v1/client/pkg/ux/ui"
)

func main() {
    // 1. 创建 UI 组件
    uiComponents := ui.NewComponents(ui.NoopLogger())
    
    // 2. 显示标题
    uiComponents.ShowHeader("WES 钱包管理器")
    
    // 3. 显示菜单
    options := []string{
        "查询余额",
        "创建钱包",
        "转账",
        "退出",
    }
    
    selectedIndex, err := uiComponents.ShowMenu("请选择操作", options)
    if err != nil {
        uiComponents.ShowError(fmt.Sprintf("选择失败: %v", err))
        return
    }
    
    // 4. 根据选择执行操作
    switch selectedIndex {
    case 0:
        // 输入地址
        address, err := uiComponents.ShowInputDialog("查询余额", "请输入地址", false)
        if err != nil {
            uiComponents.ShowError(fmt.Sprintf("输入失败: %v", err))
            return
        }
        
        // 显示加载动画
        spinner := uiComponents.ShowSpinner("正在查询余额...")
        spinner.Start()
        
        // 模拟查询
        // balance := queryBalance(address)
        
        spinner.Success("查询成功！")
        
        // 显示结果
        uiComponents.ShowBalanceInfo(address, 100.5, "WES")
        
    case 1:
        uiComponents.ShowInfo("创建钱包功能开发中...")
        
    case 2:
        uiComponents.ShowInfo("转账功能开发中...")
        
    case 3:
        uiComponents.ShowInfo("再见！")
        return
    }
}
```

## 🔗 相关链接

- [pterm 文档](https://github.com/pterm/pterm)
- [ux/flows 包文档](../flows/README.md)

