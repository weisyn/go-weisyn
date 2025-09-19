package common

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/weisyn/v1/internal/app"
	"github.com/weisyn/v1/internal/core/infrastructure/log"
)

// EnvironmentConfig 环境配置信息
type EnvironmentConfig struct {
	Name           string // 环境名称（development/testing/production）
	DisplayName    string // 显示名称
	Icon           string // 环境图标
	ConfigPath     string // 配置文件路径（用于显示）
	EmbeddedConfig []byte // 嵌入的配置内容

	// 环境特点描述
	Features []string

	// 推荐的使用模式
	RecommendedMode string

	// 特殊提示信息
	Warnings []string
}

// CreateTempConfigFile 创建临时配置文件
func (config *EnvironmentConfig) CreateTempConfigFile() (string, error) {
	// 尝试多个目录创建临时文件，优先使用可访问的目录
	tempDirs := []string{
		"./config-temp", // 启动配置临时目录
		".",             // 当前目录
		os.TempDir(),    // 系统临时目录
	}

	var tmpfile *os.File
	var err error

	for _, dir := range tempDirs {
		// 确保目录存在
		if dir == "./config-temp" {
			os.MkdirAll(dir, 0755)
		}

		// 尝试在该目录创建临时文件
		tmpfile, err = os.CreateTemp(dir, fmt.Sprintf("weisyn-%s-config-*.json", config.Name))
		if err == nil {
			break // 成功创建，退出循环
		}
	}

	if tmpfile == nil {
		return "", fmt.Errorf("创建临时配置文件失败，尝试了多个目录: %v", err)
	}

	// 写入嵌入的配置内容
	if _, err := tmpfile.Write(config.EmbeddedConfig); err != nil {
		tmpfile.Close()
		os.Remove(tmpfile.Name())
		return "", fmt.Errorf("写入临时配置文件失败: %v", err)
	}

	if err := tmpfile.Close(); err != nil {
		os.Remove(tmpfile.Name())
		return "", fmt.Errorf("关闭临时配置文件失败: %v", err)
	}

	return tmpfile.Name(), nil
}

// CleanupOldTempConfigFiles 清理遗留的临时配置文件
func (config *EnvironmentConfig) CleanupOldTempConfigFiles() {
	// 清理tmp目录中的旧临时配置文件
	pattern := fmt.Sprintf("./config-temp/weisyn-%s-config-*.json", config.Name)
	if matches, err := filepath.Glob(pattern); err == nil {
		for _, match := range matches {
			if err := os.Remove(match); err == nil {
				fmt.Printf("🧹 清理遗留临时配置文件: %s\n", match)
			}
		}
	}
}

// CleanupTempConfigFile 清理临时配置文件
func (config *EnvironmentConfig) CleanupTempConfigFile(tempPath string) {
	if tempPath != "" {
		if err := os.Remove(tempPath); err != nil && !os.IsNotExist(err) {
			// 只有文件存在但删除失败时才报告错误
			fmt.Printf("⚠️  临时配置文件清理失败: %s, 错误: %v\n", tempPath, err)
		}
	}
}

// StartupMode 启动模式
type StartupMode int

const (
	ModeAPIOnly StartupMode = iota // 仅API服务
	ModeCLIOnly                    // 仅CLI交互
	ModeFull                       // 全功能模式
)

// RunAPIOnlyMode 运行仅API服务模式
func RunAPIOnlyMode(config *EnvironmentConfig, startOptions []app.Option) {
	fmt.Printf("🌐 启动模式: 仅API服务（%s）\n", config.DisplayName)

	// 启动应用程序（仅启用API模块，禁用CLI）
	startOptions = append(startOptions, app.WithAPI()) // API默认已启用，这里显式说明
	nodeApp, err := app.Start(startOptions...)
	if err != nil {
		fmt.Printf("❌ 应用程序启动失败: %v\n", err)
		os.Exit(1)
	}

	// 打印启动成功信息
	fmt.Printf("✅ WES%s API服务启动成功！\n", config.DisplayName)
	fmt.Printf("🔗 API服务地址: http://localhost:8080\n")

	// 根据环境显示不同的特色信息
	switch config.Name {
	case "development":
		fmt.Printf("📊 管理界面: http://localhost:3000\n")
		fmt.Println("🔄 开发服务运行中，支持热重载")
	case "testing":
		fmt.Printf("🧪 适合集成测试、自动化验证\n")
	case "production":
		fmt.Printf("🚀 生产级服务，7x24小时运行")
		fmt.Println("📊 监控: 请配置相应的监控和日志收集")
	}

	fmt.Println("🔄 服务正在后台运行，按 Ctrl+C 停止...")

	// 记录日志
	log.Info(fmt.Sprintf("WES%sAPI服务启动成功", config.DisplayName))

	// 等待终止信号
	nodeApp.Wait()
	fmt.Printf("✅ WES%sAPI服务已停止\n", config.DisplayName)
}

// RunCLIOnlyMode 运行仅CLI交互模式
func RunCLIOnlyMode(config *EnvironmentConfig, startOptions []app.Option) {
	// 设置CLI模式环境变量，抑制非CLI相关的输出
	os.Setenv("WES_CLI_MODE", "true")

	fmt.Printf("💻 启动模式: 仅CLI交互（%s）\n", config.DisplayName)

	// 显示环境特定的警告
	for _, warning := range config.Warnings {
		if config.Name == "production" && warning != "" {
			fmt.Printf("⚠️  警告: %s\n", warning)
		}
	}

	// 启动应用程序（仅启用CLI模块，禁用API）
	startOptions = append(startOptions, app.WithCLI(), app.WithoutAPI())
	nodeApp, err := app.Start(startOptions...)
	if err != nil {
		fmt.Printf("❌ 应用程序启动失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ WES%sCLI已启动！\n", config.DisplayName)

	// 根据环境显示不同的功能描述
	switch config.Name {
	case "development":
		fmt.Println("💳 功能: 钱包管理、转账操作、状态查询")
	case "testing":
		fmt.Println("🧪 功能: 测试验证、功能确认、状态检查")
	case "production":
		fmt.Println("🔧 功能: 紧急调试、状态检查、问题排查")
	}

	fmt.Println("🔄 进入交互模式，按 Ctrl+C 退出...")

	// 记录日志
	logMsg := fmt.Sprintf("WES%sCLI启动成功", config.DisplayName)
	if config.Name == "production" {
		logMsg += " - 调试模式"
	}
	log.Info(logMsg)

	// 创建上下文和信号处理
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 监听中断信号
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		fmt.Println("\n🛑 正在优雅退出...")
		cancel()
		nodeApp.Stop()
	}()

	// 运行CLI交互界面
	cliApp := nodeApp.GetCLIApp()
	if cliApp != nil {
		if err := cliApp.Run(ctx); err != nil && err != context.Canceled {
			fmt.Printf("⚠️  CLI运行错误: %v\n", err)
		}
		fmt.Println("🛑 CLI已退出，正在停止服务...")
		nodeApp.Stop()
	} else {
		fmt.Println("⚠️  CLI服务未启用")
		<-ctx.Done()
	}

	fmt.Printf("✅ WES%sCLI已停止\n", config.DisplayName)
}

// RunFullMode 运行全功能模式（默认）
func RunFullMode(config *EnvironmentConfig, startOptions []app.Option) {
	// 设置CLI模式环境变量，因为全功能模式也包含CLI界面
	// 这样可以保持一致的用户体验，避免技术日志干扰
	os.Setenv("WES_CLI_MODE", "true")

	fmt.Printf("%s 启动模式: 全功能（%s）\n", config.Icon, config.DisplayName)

	// 生产环境的警告
	if config.Name == "production" {
		fmt.Println("⚠️  警告: 全功能模式不推荐用于生产环境")
		fmt.Println("💡 建议: 使用 --api-only 模式进行生产部署")
	}

	// 启动应用程序（同时启用API和CLI模块）
	startOptions = append(startOptions, app.WithAPI(), app.WithCLI())
	nodeApp, err := app.Start(startOptions...)
	if err != nil {
		fmt.Printf("❌ 应用程序启动失败: %v\n", err)
		os.Exit(1)
	}

	// 打印启动成功信息
	fmt.Printf("✅ WES%s启动成功！\n", config.DisplayName)
	fmt.Printf("🔗 API服务: http://localhost:8080\n")

	// 环境特定信息
	switch config.Name {
	case "development":
		fmt.Printf("📊 管理界面: http://localhost:3000\n")
	case "testing":
		fmt.Printf("🧪 测试验证: 完整功能可用\n")
	case "production":
		fmt.Printf("⚠️  生产提醒: CLI界面占用额外资源\n")
	}

	fmt.Println("💻 CLI交互界面已就绪")
	fmt.Println("🔄 完整功能运行中，按 Ctrl+C 停止...")

	// 记录日志
	logMsg := fmt.Sprintf("WES%s全功能模式启动成功", config.DisplayName)
	if config.Name == "production" {
		logMsg += " - 不推荐配置"
	}
	log.Info(logMsg)

	// 创建上下文和信号处理
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 监听中断信号
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		fmt.Println("\n🛑 正在优雅退出...")
		cancel()
		nodeApp.Stop()
	}()

	// 运行CLI交互界面
	cliApp := nodeApp.GetCLIApp()
	if cliApp != nil {
		if err := cliApp.Run(ctx); err != nil && err != context.Canceled {
			fmt.Printf("⚠️  交互界面运行错误: %v\n", err)
		}
		fmt.Println("🛑 CLI已退出，正在停止整个服务...")
		cancel()
		nodeApp.Stop()
	} else {
		fmt.Println("⚠️  CLI服务未启用，使用基本模式")
		fmt.Println("📖 API服务已启动，按 Ctrl+C 停止...")
		<-ctx.Done()
	}

	fmt.Printf("✅ WES%s已停止\n", config.DisplayName)
}

// ShowEnvironmentHelp 显示环境特定的帮助信息
func ShowEnvironmentHelp(config *EnvironmentConfig) {
	fmt.Printf("%s WES %s节点\n", config.Icon, config.DisplayName)
	fmt.Println()
	fmt.Println("用法:")
	fmt.Printf("  go run ./cmd/%s [选项]\n", config.Name)
	fmt.Printf("  ./bin/%s [选项]\n", config.Name)
	fmt.Println()

	fmt.Println("启动模式:")
	if config.Name == "production" {
		fmt.Printf("  ./bin/%s --api-only          # 仅API服务（生产推荐⭐）\n", config.Name)
		fmt.Printf("  ./bin/%s                     # 完整功能（不推荐生产）\n", config.Name)
		fmt.Printf("  ./bin/%s --cli-only          # 仅CLI交互（仅调试用）\n", config.Name)
	} else {
		fmt.Printf("  ./bin/%s                     # 完整功能（CLI + API）\n", config.Name)
		if config.Name == "development" {
			fmt.Printf("  ./bin/%s --api-only         # 仅API服务\n", config.Name)
			fmt.Printf("  ./bin/%s --cli-only         # 仅CLI交互\n", config.Name)
		} else if config.Name == "testing" {
			fmt.Printf("  ./bin/%s --api-only         # 仅API服务（推荐CI/CD）\n", config.Name)
			fmt.Printf("  ./bin/%s --cli-only         # 仅CLI交互（功能验证）\n", config.Name)
		}
	}

	fmt.Println()
	fmt.Println("配置文件:")
	fmt.Printf("  自动加载: %s\n", config.ConfigPath)
	fmt.Println()

	// 显示环境特有的警告
	if len(config.Warnings) > 0 && config.Name == "production" {
		fmt.Println("⚠️  生产环境注意事项:")
		for _, warning := range config.Warnings {
			if warning != "" {
				fmt.Printf("  • %s\n", warning)
			}
		}
		fmt.Println()
	}

	fmt.Println("环境特点:")
	for _, feature := range config.Features {
		fmt.Printf("  ✓ %s\n", feature)
	}
}

// ShowEnvironmentVersion 显示环境特定的版本信息
func ShowEnvironmentVersion(config *EnvironmentConfig) {
	fmt.Printf("WES %s节点 v1.0.0\n", config.DisplayName)
	fmt.Printf("环境: %s\n", config.Name)
	fmt.Printf("配置: %s (嵌入式)\n", config.ConfigPath)
	fmt.Printf("构建时间: 2025-01-26\n")
	fmt.Printf("Go版本: 1.21+\n")
}

// StartWithEmbeddedConfig 使用嵌入配置启动应用
func StartWithEmbeddedConfig(config *EnvironmentConfig, apiOnly, cliOnly bool) {
	fmt.Printf("%s 正在启动WES%s节点...\n", config.Icon, config.DisplayName)
	fmt.Printf("📁 配置: %s (嵌入式配置)\n", config.ConfigPath)

	// 简化启动提示
	fmt.Println("🔧 正在启动WES开发环境节点...")

	// 清理遗留的临时配置文件
	config.CleanupOldTempConfigFiles()

	// 验证嵌入配置
	if len(config.EmbeddedConfig) == 0 {
		fmt.Printf("❌ 错误: 未找到嵌入的配置内容\n")
		fmt.Println("💡 这可能是构建过程中的问题")
		os.Exit(1)
	}

	// 创建临时配置文件
	tempConfigPath, err := config.CreateTempConfigFile()
	if err != nil {
		fmt.Printf("❌ 错误: %v\n", err)
		os.Exit(1)
	}

	// 设置信号处理，确保程序被中断时能正确清理临时文件
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Printf("\n🛑 收到中断信号，正在清理资源...\n")
		config.CleanupTempConfigFile(tempConfigPath)
		os.Exit(0)
	}()

	// 确保临时文件被清理
	defer config.CleanupTempConfigFile(tempConfigPath)

	// 设置启动选项，使用临时配置文件
	var startOptions []app.Option
	startOptions = append(startOptions, app.WithConfigFile(tempConfigPath))

	// 判断启动模式并使用共享逻辑
	if apiOnly {
		RunAPIOnlyMode(config, startOptions)
	} else if cliOnly {
		RunCLIOnlyMode(config, startOptions)
	} else {
		RunFullMode(config, startOptions)
	}
}
