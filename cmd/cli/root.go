package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/weisyn/v1/client/core/config"
	"github.com/weisyn/v1/client/core/output"
	"github.com/weisyn/v1/client/core/transport"
)

// GlobalFlags 全局标志
type GlobalFlags struct {
	Profile      string // Profile名称
	ConfigDir    string // 配置目录
	OutputFormat string // 输出格式
	Silent       bool   // 静默模式
	Verbose      bool   // 详细模式
}

var (
	globalFlags GlobalFlags
	profileMgr  *config.ProfileManager
	formatter   *output.Formatter
)

// rootCmd 根命令
var rootCmd = &cobra.Command{
	Use:   "wes", // 命令名：wes（二进制名为 weisyn-cli）
	Short: "WES 区块链命令行客户端",
	Long: `WES CLI - 去中心化节点的薄客户端

二进制名: weisyn-cli
命令名: wes

使用方式:
  weisyn-cli <command>     # 直接使用二进制名
  wes <command>            # 使用别名（推荐）

WES CLI 是 WES 区块链的官方命令行工具,提供完整的区块链交互能力:
- 查询链状态、区块、交易
- 管理账户和密钥
- 构建、签名、发送交易
- 部署和调用智能合约
- 订阅实时事件

支持离线签名、多环境配置、冷钱包工作流等高级特性。`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// 初始化配置管理器
		var err error
		profileMgr, err = config.NewProfileManager(globalFlags.ConfigDir)
		if err != nil {
			return fmt.Errorf("初始化配置: %w", err)
		}

		// 初始化输出格式化器
		format := output.Format(globalFlags.OutputFormat)
		formatter = output.NewFormatter(format, os.Stdout)
		formatter.SetSilent(globalFlags.Silent)

		return nil
	},
}

// Execute 执行根命令
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	// 全局标志
	rootCmd.PersistentFlags().StringVar(&globalFlags.Profile, "profile", "", "使用指定的Profile (默认使用当前Profile)")
	rootCmd.PersistentFlags().StringVar(&globalFlags.ConfigDir, "config-dir", "", "配置目录 (默认: ~/.wes)")
	rootCmd.PersistentFlags().StringVarP(&globalFlags.OutputFormat, "output", "o", "json", "输出格式: json|pretty|table|text")
	rootCmd.PersistentFlags().BoolVar(&globalFlags.Silent, "silent", false, "静默模式 (仅输出结果)")
	rootCmd.PersistentFlags().BoolVarP(&globalFlags.Verbose, "verbose", "v", false, "详细输出")

	// 添加子命令
	rootCmd.AddCommand(chainCmd)
	rootCmd.AddCommand(blockCmd)
	rootCmd.AddCommand(txCmd)
	rootCmd.AddCommand(accountCmd)
	rootCmd.AddCommand(contractCmd)
	rootCmd.AddCommand(aiCmd)
	rootCmd.AddCommand(miningCmd)
	rootCmd.AddCommand(profileCmd)
}

// getClient 获取传输客户端
func getClient() (transport.Client, error) {
	// 获取当前profile
	var profile *config.Profile
	var err error

	if globalFlags.Profile != "" {
		profile, err = profileMgr.GetProfile(globalFlags.Profile)
	} else {
		profile, err = profileMgr.GetCurrentProfile()
	}

	if err != nil {
		return nil, fmt.Errorf("获取Profile: %w", err)
	}

	// 创建故障转移客户端
	clientConfig := profileToTransportConfig(profile)
	return transport.NewFallbackClient(clientConfig)
}

// profileToTransportConfig 将config.Profile转换为transport.ClientConfig
func profileToTransportConfig(p *config.Profile) transport.ClientConfig {
	// 转换Endpoints
	eps := make([]transport.EndpointConfig, 0, len(p.Endpoints))
	for _, e := range p.Endpoints {
		eps = append(eps, transport.EndpointConfig{
			Name:     e.Name,
			Priority: e.Priority,
			JSONRPC:  e.JSONRPC,
			REST:     e.REST,
			WS:       e.WS,
			GRPC:     e.GRPC,
		})
	}

	return transport.ClientConfig{
		Endpoints:           eps,
		Timeout:             time.Duration(p.Timeout),
		RetryAttempts:       p.RetryAttempts,
		RetryBackoff:        time.Duration(p.RetryBackoff),
		HealthCheckInterval: time.Duration(p.HealthCheckInterval),
	}
}
