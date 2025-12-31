package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/weisyn/v1/client/core/config"
	"github.com/weisyn/v1/client/core/transport"
	"github.com/weisyn/v1/client/core/wallet"
	addresspkg "github.com/weisyn/v1/internal/core/infrastructure/crypto/address"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/key"
)

// wizardCmd 首次启动向导
var wizardCmd = &cobra.Command{
	Use:   "wizard",
	Short: "首次启动向导",
	Long:  "引导首次使用的用户完成初始配置：创建Profile、检测连通性、创建账户",
	RunE:  runWizard,
}

func runWizard(cmd *cobra.Command, args []string) error {
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║          欢迎使用 WES 区块链命令行工具                          ║")
	fmt.Println("║                 首次启动向导                                    ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 步骤1：检测配置目录
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("无法获取用户目录: %w", err)
	}

	configDir := filepath.Join(homeDir, ".wes")
	fmt.Printf("✓ 配置目录: %s\n", configDir)
	fmt.Println()

	// 步骤2：选择环境
	fmt.Println("【步骤 1/4】选择要连接的网络环境")
	fmt.Println()
	fmt.Println("  1. 本地开发环境 (localhost:28680)")
	fmt.Println("  2. 测试网络 (testnet)")
	fmt.Println("  3. 主网络 (mainnet)")
	fmt.Println("  4. 自定义节点")
	fmt.Println()

	var choice string
	fmt.Print("请选择 [1-4]: ")
	if _, err := fmt.Scanln(&choice); err != nil {
		return fmt.Errorf("读取输入失败: %w", err)
	}
	fmt.Println()

	var profileName, chainID, nodeURL string

	switch choice {
	case "1":
		profileName = "dev-private-local" // 使用完整 Profile 名称
		chainID = "wes-local-1"
		nodeURL = "http://localhost:28680/jsonrpc"
	case "2":
		profileName = "test-public-testnet" // 使用完整 Profile 名称
		chainID = "wes-testnet-1"
		nodeURL = "https://testnet-rpc.wes.io"
	case "3":
		profileName = "prod-public-mainnet" // 使用完整 Profile 名称
		chainID = "wes-mainnet-1"
		nodeURL = "https://mainnet-rpc.wes.io"
	case "4":
		fmt.Print("Profile 名称: ")
		if _, err := fmt.Scanln(&profileName); err != nil {
			return fmt.Errorf("读取 Profile 名称失败: %w", err)
		}
		fmt.Print("Chain ID: ")
		if _, err := fmt.Scanln(&chainID); err != nil {
			return fmt.Errorf("读取 Chain ID 失败: %w", err)
		}
		fmt.Print("节点 JSON-RPC URL: ")
		if _, err := fmt.Scanln(&nodeURL); err != nil {
			return fmt.Errorf("读取节点 URL 失败: %w", err)
		}
		fmt.Println()
	default:
		return fmt.Errorf("无效的选择")
	}

	fmt.Printf("✓ 将创建 Profile: %s\n", profileName)
	fmt.Printf("✓ Chain ID: %s\n", chainID)
	fmt.Printf("✓ 节点 URL: %s\n", nodeURL)
	fmt.Println()

	// 步骤3：测试节点连通性
	fmt.Println("【步骤 2/4】测试节点连通性")
	fmt.Println()

	if err := testNodeConnectivity(nodeURL); err != nil {
		fmt.Printf("⚠️  警告: 无法连接到节点: %v\n", err)
		fmt.Println()
		fmt.Print("是否继续？(yes/no): ")
		var confirm string
		if _, err := fmt.Scanln(&confirm); err != nil {
			return fmt.Errorf("读取输入失败: %w", err)
		}
		if strings.ToLower(confirm) != "yes" {
			return fmt.Errorf("用户取消")
		}
		fmt.Println()
	} else {
		fmt.Println("✓ 节点连接成功")
		fmt.Println()
	}

	// 步骤4：创建 Profile
	fmt.Println("【步骤 3/4】创建配置 Profile")
	fmt.Println()

	pm, err := config.NewProfileManager(configDir)
	if err != nil {
		return fmt.Errorf("初始化配置管理器失败: %w", err)
	}

	// 检查 Profile 是否已存在
	if _, err := pm.GetProfile(profileName); err == nil {
		fmt.Printf("⚠️  Profile '%s' 已存在\n", profileName)
		fmt.Print("是否覆盖？(yes/no): ")
		var confirm string
		if _, err := fmt.Scanln(&confirm); err != nil {
			return fmt.Errorf("读取输入失败: %w", err)
		}
		if strings.ToLower(confirm) != "yes" {
			fmt.Println("✓ 使用现有 Profile")
		} else {
			if err := createProfile(pm, profileName, chainID, nodeURL); err != nil {
				return err
			}
		}
	} else {
		if err := createProfile(pm, profileName, chainID, nodeURL); err != nil {
			return err
		}
	}

	// 切换到新创建的 Profile
	if err := pm.SwitchProfile(profileName); err != nil {
		return fmt.Errorf("切换 Profile 失败: %w", err)
	}

	fmt.Printf("✓ 已切换到 Profile '%s'\n", profileName)
	fmt.Println()

	// 步骤5：创建第一个账户
	fmt.Println("【步骤 4/4】创建您的第一个账户")
	fmt.Println()
	fmt.Print("是否创建新账户？(yes/no): ")
	var createAccount string
	if _, err := fmt.Scanln(&createAccount); err != nil {
		return fmt.Errorf("读取输入失败: %w", err)
	}
	fmt.Println()

	if strings.ToLower(createAccount) == "yes" {
		profile, _ := pm.GetProfile(profileName)

		fmt.Print("账户标签（可选）: ")
		var label string
		if _, err := fmt.Scanln(&label); err != nil {
			// 标签是可选的，忽略错误
			label = ""
		}

		password, err := promptPassword("请设置账户密码")
		if err != nil {
			return err
		}

		confirmPassword, err := promptPassword("请确认密码")
		if err != nil {
			return err
		}

		if password != confirmPassword {
			return fmt.Errorf("密码不匹配")
		}
		fmt.Println()

		// 创建账户管理器（使用标准的 AddressManager 生成 Base58Check 地址）
		keyManager := key.NewKeyManager()
		addressManager := addresspkg.NewAddressService(keyManager)
		am, err := wallet.NewAccountManager(profile.KeystorePath, addressManager)
		if err != nil {
			return fmt.Errorf("初始化账户管理器失败: %w", err)
		}

		// 创建账户
		account, err := am.CreateAccount(password, label)
		if err != nil {
			return fmt.Errorf("创建账户失败: %w", err)
		}

		fmt.Println("✓ 账户创建成功")
		fmt.Printf("  地址: %s\n", account.Address)
		fmt.Printf("  Keystore: %s\n", account.KeystorePath)
		if label != "" {
			fmt.Printf("  标签: %s\n", label)
		}
		fmt.Println()
	} else {
		fmt.Println("✓ 跳过账户创建（您可以稍后使用 'wes account new' 创建）")
		fmt.Println()
	}

	// 完成
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    配置完成！                                   ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("接下来您可以：")
	fmt.Println()
	fmt.Println("  查看配置:")
	fmt.Printf("    wes profile show %s\n", profileName)
	fmt.Println()
	fmt.Println("  创建账户:")
	fmt.Println("    wes account new --label \"my-wallet\"")
	fmt.Println()
	fmt.Println("  查看链信息:")
	fmt.Println("    wes chain info")
	fmt.Println()
	fmt.Println("  查看所有命令:")
	fmt.Println("    wes --help")
	fmt.Println()

	return nil
}

// testNodeConnectivity 测试节点连通性
func testNodeConnectivity(nodeURL string) error {
	fmt.Printf("正在连接到 %s ...\n", nodeURL)

	// 创建临时客户端
	clientConfig := transport.ClientConfig{
		Endpoints: []transport.EndpointConfig{
			{
				Name:     "test",
				Priority: 1,
				JSONRPC:  nodeURL,
			},
		},
		Timeout:       30 * time.Second,
		RetryAttempts: 1,
	}

	client, err := transport.NewFallbackClient(clientConfig)
	if err != nil {
		return fmt.Errorf("创建客户端失败: %w", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("Failed to close client: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Ping 测试
	if err := client.Ping(ctx); err != nil {
		return fmt.Errorf("Ping 失败: %w", err)
	}

	// 获取链 ID
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("获取链 ID 失败: %w", err)
	}

	// 获取区块高度
	blockNumber, err := client.BlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("获取区块高度失败: %w", err)
	}

	fmt.Printf("  Chain ID: %s\n", chainID)
	fmt.Printf("  当前区块高度: %d\n", blockNumber)

	return nil
}

// createProfile 创建 Profile
func createProfile(pm *config.ProfileManager, name, chainID, nodeURL string) error {
	profile := &config.Profile{
		Name:    name,
		ChainID: chainID,
		Endpoints: []config.EndpointConfig{
			{
				Name:     name + "-primary",
				Priority: 1,
				JSONRPC:  nodeURL,
			},
		},
		Timeout:             config.Duration(30 * time.Second),
		RetryAttempts:       3,
		RetryBackoff:        config.Duration(time.Second),
		HealthCheckInterval: config.Duration(30 * time.Second),
	}

	if err := pm.SaveProfile(profile); err != nil {
		return fmt.Errorf("保存 Profile 失败: %w", err)
	}

	fmt.Printf("✓ Profile '%s' 创建成功\n", name)
	return nil
}

func init() {
	rootCmd.AddCommand(wizardCmd)
}
