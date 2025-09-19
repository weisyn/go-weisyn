package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/weisyn/v1/cmd/common"
	"github.com/weisyn/v1/configs"
)

func main() {
	// 环境配置（嵌入式）
	envConfig := &common.EnvironmentConfig{
		Name:           "development",
		DisplayName:    "开发环境",
		Icon:           "🔧",
		ConfigPath:     "configs/development/single/config.json",
		EmbeddedConfig: configs.GetDevelopmentConfig(),
		Features: []string{
			"开发调试优化",
			"详细日志输出",
			"快速启动配置",
			"本地钱包管理",
			"零配置启动",
		},
		RecommendedMode: "full",
		Warnings:        []string{},
	}

	// 命令行参数定义
	var (
		apiOnly     = flag.Bool("api-only", false, "仅启动API服务（后端开发）")
		cliOnly     = flag.Bool("cli-only", false, "仅启动CLI交互（个人用户）")
		autoDemo    = flag.Bool("auto-demo", false, "自动演示模式（跳过用户交互）")
		memoryOnly  = flag.Bool("memory-only", false, "强制使用内存数据库模式（数据不持久化）")
		showHelp    = flag.Bool("help", false, "显示帮助信息")
		showVersion = flag.Bool("version", false, "显示版本信息")
	)
	flag.Parse()

	// 显示帮助信息
	if *showHelp {
		common.ShowEnvironmentHelp(envConfig)
		fmt.Println()
		fmt.Println("选项:")
		flag.PrintDefaults()
		return
	}

	// 显示版本信息
	if *showVersion {
		common.ShowEnvironmentVersion(envConfig)
		return
	}

	// 参数冲突检查
	if *apiOnly && *cliOnly {
		fmt.Println("❌ 错误: --api-only 和 --cli-only 不能同时使用")
		fmt.Println("💡 提示: 使用 --help 查看详细用法")
		return
	}

	// 设置自动演示模式环境变量
	if *autoDemo {
		os.Setenv("WES_AUTO_DEMO_MODE", "true")
		fmt.Println("🤖 启用自动演示模式 - 将自动完成所有交互步骤")
	}

	// 设置内存数据库模式环境变量
	if *memoryOnly {
		os.Setenv("WES_MEMORY_ONLY_MODE", "true")
		fmt.Printf("\n")
		fmt.Printf("🧠 强制内存数据库模式已启用\n")
		fmt.Printf("⚠️  警告: 所有数据仅存储在内存中，程序退出后将丢失\n")
		fmt.Printf("💡 适用场景: 测试、演示、临时开发\n")
		fmt.Printf("\n")
	}

	// 使用嵌入配置启动
	common.StartWithEmbeddedConfig(envConfig, *apiOnly, *cliOnly)
}
