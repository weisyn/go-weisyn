// Package config provides configuration management functionality for client operations.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config CLI 配置
type Config struct {
	// 节点配置
	NodeEndpoint string `json:"node_endpoint"` // JSON-RPC 端点
	NodeRESTURL  string `json:"node_rest_url"` // REST API 端点

	// 钱包配置
	WalletDataDir string `json:"wallet_data_dir"` // 钱包数据目录
	DefaultWallet string `json:"default_wallet"`  // 默认钱包名称

	// CLI 配置
	FirstTimeSetup bool   `json:"first_time_setup"` // 是否首次设置
	Language       string `json:"language"`         // 语言设置

	// 合约代币查询配置
	ContractTokens []ContractToken `json:"contract_tokens,omitempty"` // 需要查询余额的合约代币
}

// ContractToken 合约代币配置
type ContractToken struct {
	Label       string `json:"label"`              // 展示名称
	ContentHash string `json:"content_hash"`       // 合约内容哈希（64位十六进制）
	TokenID     string `json:"token_id,omitempty"` // 代币标识（可选）
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	dataDir := filepath.Join(homeDir, ".wes-cli")

	return &Config{
		NodeEndpoint:   "http://localhost:28680/jsonrpc",
		NodeRESTURL:    "http://localhost:28680/api/v1",
		WalletDataDir:  filepath.Join(dataDir, "wallets"),
		DefaultWallet:  "",
		FirstTimeSetup: true,
		Language:       "zh-CN",
		ContractTokens: []ContractToken{},
	}
}

// Load 加载配置
func Load() (*Config, error) {
	configPath := getConfigPath()

	// 如果配置文件不存在，返回默认配置
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg := DefaultConfig()
		// 创建配置目录
		//nolint:gosec // G301: 配置目录需要用户可读权限，0755 是合理的
		if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
			return nil, fmt.Errorf("creating config directory: %w", err)
		}
		// 保存默认配置
		if err := cfg.Save(); err != nil {
			return nil, fmt.Errorf("saving default config: %w", err)
		}
		return cfg, nil
	}

	// 读取配置文件
	//nolint:gosec // G304: configPath 来自用户主目录，路径安全可控
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	// 解析配置
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	return &cfg, nil
}

// Save 保存配置
func (c *Config) Save() error {
	configPath := getConfigPath()

	// 确保目录存在
	//nolint:gosec // G301: 配置目录需要用户可读权限，0755 是合理的
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	// 序列化配置
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	// 写入文件
	//nolint:gosec // G304: configPath 来自用户主目录，路径安全可控
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

// getConfigPath 获取配置文件路径
func getConfigPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".wes-cli", "config.json")
}
