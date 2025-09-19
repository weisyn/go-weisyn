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
		Name:           "testing",
		DisplayName:    "测试环境",
		Icon:           "🧪",
		ConfigPath:     "configs/testing/config.json",
		EmbeddedConfig: configs.GetTestingConfig(),
		Features: []string{
			"CI/CD优化",
			"稳定配置参数",
			"自动化测试友好",
			"快速启动停止",
			"零配置启动",
		},
		RecommendedMode: "api-only",
		Warnings:        []string{},
	}

	// 命令行参数定义
	var (
		apiOnly     = flag.Bool("api-only", false, "仅启动API服务（适合CI/CD）")
		cliOnly     = flag.Bool("cli-only", false, "仅启动CLI交互（测试验证）")
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

	// 使用嵌入配置启动
	common.StartWithEmbeddedConfig(envConfig, *apiOnly, *cliOnly)
}
