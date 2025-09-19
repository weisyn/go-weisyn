package main

import (
	"flag"
	"fmt"

	"github.com/weisyn/v1/cmd/common"
	"github.com/weisyn/v1/configs"
)

func main() {
	// 环境配置（嵌入式）
	envConfig := &common.EnvironmentConfig{
		Name:           "production",
		DisplayName:    "生产环境",
		Icon:           "🚀",
		ConfigPath:     "configs/production/config.json",
		EmbeddedConfig: configs.GetProductionConfig(),
		Features: []string{
			"生产级优化",
			"高性能配置",
			"安全加固",
			"监控集成",
			"零配置启动",
		},
		RecommendedMode: "api-only",
		Warnings: []string{
			"推荐使用 --api-only 模式部署",
			"确保二进制文件安全性和访问权限",
			"建议配置系统服务和监控",
			"CLI模式仅用于紧急调试",
			"CLI模式不适合生产环境长期运行",
		},
	}

	// 命令行参数定义
	var (
		apiOnly     = flag.Bool("api-only", false, "仅启动API服务（生产环境推荐）")
		cliOnly     = flag.Bool("cli-only", false, "仅启动CLI交互（不推荐生产使用）")
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

	// 生产环境特殊警告
	if !*apiOnly {
		fmt.Println("⚠️  警告: 生产环境建议使用 --api-only 模式")
		fmt.Println("💡 提示: ./bin/production --api-only")
	}

	// 使用嵌入配置启动
	common.StartWithEmbeddedConfig(envConfig, *apiOnly, *cliOnly)
}
