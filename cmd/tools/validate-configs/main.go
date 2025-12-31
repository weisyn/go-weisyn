// Package main 提供验证链配置文件的命令行工具
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/weisyn/v1/internal/config"
	"github.com/weisyn/v1/pkg/types"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "用法: %s <config-file>...\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "示例: %s configs/chains/*.json\n", os.Args[0])
		os.Exit(1)
	}

	var hasError bool
	for _, configPath := range os.Args[1:] {
		if err := validateConfig(configPath); err != nil {
			fmt.Fprintf(os.Stderr, "❌ %s: %v\n", configPath, err)
			hasError = true
		} else {
			fmt.Printf("✅ %s: 验证通过\n", configPath)
		}
	}

	if hasError {
		os.Exit(1)
	}
}

func validateConfig(configPath string) error {
	// 读取配置文件
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析为 AppConfig
	var appConfig types.AppConfig
	if err := json.Unmarshal(configBytes, &appConfig); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 构建统一创世配置（简化版，仅用于验证）
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
		if acc.PublicKey != "" {
			genesisAccount.PublicKey = acc.PublicKey
		} else {
			// 如果没有 public_key，使用 address 作为标识（确保确定性）
			genesisAccount.PublicKey = acc.Address
		}
		genesisConfig.GenesisAccounts = append(genesisConfig.GenesisAccounts, genesisAccount)
	}

	// 执行验证
	if err := config.ValidateMandatoryConfig(&appConfig, genesisConfig); err != nil {
		return fmt.Errorf("配置验证失败: %w", err)
	}

	return nil
}


