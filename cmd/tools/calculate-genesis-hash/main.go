// Package main 提供计算创世哈希的命令行工具
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/weisyn/v1/internal/config/node"
	"github.com/weisyn/v1/pkg/types"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "用法: %s <config-file>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "示例: %s configs/chains/test-public-demo.json\n", os.Args[0])
		os.Exit(1)
	}

	configPath := os.Args[1]
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取配置文件失败: %v\n", err)
		os.Exit(1)
	}

	var appConfig types.AppConfig
	if err := json.Unmarshal(configBytes, &appConfig); err != nil {
		fmt.Fprintf(os.Stderr, "解析配置文件失败: %v\n", err)
		os.Exit(1)
	}

	// 构建统一创世配置
	genesisConfig := &types.GenesisConfig{
		NetworkID: "",
		ChainID:   0,
		Timestamp: appConfig.Genesis.Timestamp,
	}

	if appConfig.Network != nil {
		if appConfig.Network.NetworkID != nil {
			genesisConfig.NetworkID = *appConfig.Network.NetworkID
		}
		if appConfig.Network.ChainID != nil {
			genesisConfig.ChainID = *appConfig.Network.ChainID
		}
	}

	for _, acc := range appConfig.Genesis.Accounts {
		genesisAccount := types.GenesisAccount{
			Address:        acc.Address,
			InitialBalance: acc.InitialBalance,
		}
		// 优先使用 public_key，如果没有则使用 address 作为标识（CalculateGenesisHash 需要 PublicKey）
		if acc.PublicKey != "" {
			genesisAccount.PublicKey = acc.PublicKey
		} else {
			// 如果没有 public_key，使用 address 作为标识（确保确定性）
			// 注意：这要求配置文件中要么有 public_key，要么所有账户的 address 唯一且稳定
			genesisAccount.PublicKey = acc.Address
		}
		genesisConfig.GenesisAccounts = append(genesisConfig.GenesisAccounts, genesisAccount)
	}

	// 计算 genesis hash
	genesisHash, err := node.CalculateGenesisHash(genesisConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "计算创世哈希失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("配置文件: %s\n", configPath)
	fmt.Printf("链ID: %d\n", genesisConfig.ChainID)
	fmt.Printf("网络ID: %s\n", genesisConfig.NetworkID)
	fmt.Printf("创世时间戳: %d\n", genesisConfig.Timestamp)
	fmt.Printf("创世账户数: %d\n", len(genesisConfig.GenesisAccounts))
	fmt.Printf("\n计算得到的 genesis_hash: %s\n", genesisHash)
	fmt.Printf("\n请在配置文件的 genesis 段添加:\n")
	fmt.Printf("  \"expected_genesis_hash\": \"%s\"\n", genesisHash)
}

