package main

import (
	"context"
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"github.com/weisyn/v1/client/core/wallet"
	"github.com/weisyn/v1/client/pkg/transport/api"
	"github.com/weisyn/v1/client/pkg/transport/jsonrpc"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/address"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/key"
)

var (
	miningAddress string // 矿工地址
)

// miningCmd 挖矿相关命令
var miningCmd = &cobra.Command{
	Use:   "mining",
	Short: "挖矿控制",
	Long:  "启动、停止挖矿，查看挖矿状态",
}

// miningStartCmd 启动挖矿
var miningStartCmd = &cobra.Command{
	Use:   "start",
	Short: "启动挖矿",
	Long: `启动挖矿进程

示例:
  wes mining start --address CUQ3g6P5WmFN289pPn7AAhnQ3T2cZRv2BR
  wes mining start  # 使用默认钱包地址`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getClient()
		if err != nil {
			return err
		}
		defer func() {
			if err := client.Close(); err != nil {
				log.Printf("Failed to close client: %v", err)
			}
		}()

		ctx := context.Background()

		// 如果未指定地址，尝试使用默认钱包
		minerAddr := miningAddress
		if minerAddr == "" {
			// 尝试获取默认钱包地址
			profile, err := profileMgr.GetCurrentProfile()
			if err != nil {
				return fmt.Errorf("获取Profile失败: %w", err)
			}

			// 使用标准的 AddressManager 生成 Base58Check 地址
			keyMgr := key.NewKeyManager()
			addrMgr := address.NewAddressService(keyMgr)
			am, err := wallet.NewAccountManager(profile.KeystorePath, addrMgr)
			if err != nil {
				return fmt.Errorf("初始化账户管理器: %w", err)
			}

			defaultAccount, err := am.GetDefaultWallet()
			if err != nil {
				return fmt.Errorf("获取默认钱包失败: %w\n提示: 使用 --address 指定矿工地址", err)
			}

			minerAddr = defaultAccount.Address
			formatter.PrintInfo(fmt.Sprintf("使用默认钱包地址: %s", minerAddr))
		}

		// 创建挖矿适配器
		profile, _ := profileMgr.GetCurrentProfile()
		rpcClient := jsonrpc.NewClient(profile.Endpoints[0].JSONRPC)

		// 需要 addressManager
		keyManager := key.NewKeyManager()
		addressManager := address.NewAddressService(keyManager)

		miningAdapter := api.NewMiningAdapter(rpcClient, addressManager)

		// 启动挖矿
		if err := miningAdapter.StartMining(ctx, minerAddr); err != nil {
			formatter.PrintError(err)
			return err
		}

		formatter.PrintSuccess("✅ 挖矿已启动")
		formatter.PrintInfo(fmt.Sprintf("矿工地址: %s", minerAddr))

		return nil
	},
}

// miningStopCmd 停止挖矿
var miningStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "停止挖矿",
	Long:  "停止挖矿进程",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getClient()
		if err != nil {
			return err
		}
		defer func() {
			if err := client.Close(); err != nil {
				log.Printf("Failed to close client: %v", err)
			}
		}()

		ctx := context.Background()

		// 创建挖矿适配器
		profile, _ := profileMgr.GetCurrentProfile()
		rpcClient := jsonrpc.NewClient(profile.Endpoints[0].JSONRPC)

		keyManager := key.NewKeyManager()
		addressManager := address.NewAddressService(keyManager)

		miningAdapter := api.NewMiningAdapter(rpcClient, addressManager)

		// 停止挖矿
		if err := miningAdapter.StopMining(ctx); err != nil {
			formatter.PrintError(err)
			return err
		}

		formatter.PrintSuccess("✅ 挖矿已停止")

		return nil
	},
}

// miningStatusCmd 查询挖矿状态
var miningStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "查询挖矿状态",
	Long:  "查询当前挖矿状态和矿工地址",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getClient()
		if err != nil {
			return err
		}
		defer func() {
			if err := client.Close(); err != nil {
				log.Printf("Failed to close client: %v", err)
			}
		}()

		ctx := context.Background()

		// 创建挖矿适配器
		profile, _ := profileMgr.GetCurrentProfile()
		rpcClient := jsonrpc.NewClient(profile.Endpoints[0].JSONRPC)

		keyManager := key.NewKeyManager()
		addressManager := address.NewAddressService(keyManager)

		miningAdapter := api.NewMiningAdapter(rpcClient, addressManager)

		// 获取挖矿状态
		status, err := miningAdapter.GetMiningStatus(ctx)
		if err != nil {
			formatter.PrintError(err)
			return err
		}

		// 格式化输出
		result := map[string]interface{}{
			"is_running":    status.IsRunning,
			"miner_address": status.MinerAddress,
		}

		if status.IsRunning {
			formatter.PrintSuccess("⛏️  挖矿运行中")
		} else {
			formatter.PrintInfo("⏸️  挖矿已停止")
		}

		return formatter.Print(result)
	},
}

func init() {
	// 添加子命令
	miningCmd.AddCommand(miningStartCmd)
	miningCmd.AddCommand(miningStopCmd)
	miningCmd.AddCommand(miningStatusCmd)

	// 添加标志
	miningStartCmd.Flags().StringVar(&miningAddress, "address", "", "矿工地址（接收挖矿奖励）")
}
