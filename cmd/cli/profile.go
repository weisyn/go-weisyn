package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/weisyn/v1/client/core/config"
)

// profileCmd Profile管理命令
var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Profile管理",
	Long:  "管理配置Profile,支持多环境切换(local/testnet/mainnet)",
}

// profileListCmd 列出所有profiles
var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出所有profiles",
	Long:  "列出所有可用的配置Profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		profiles := profileMgr.ListProfiles()
		currentProfile, _ := profileMgr.GetCurrentProfile()

		var result []map[string]interface{}
		for _, name := range profiles {
			profile, err := profileMgr.GetProfile(name)
			if err != nil {
				continue
			}

			isCurrent := (currentProfile != nil && currentProfile.Name == name)

			result = append(result, map[string]interface{}{
				"name":     name,
				"chain_id": profile.ChainID,
				"current":  isCurrent,
			})
		}

		return formatter.Print(result)
	},
}

// profileShowCmd 显示profile详情
var profileShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "显示profile详情",
	Long:  "显示指定profile的详细配置(不指定则显示当前profile)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var profile *config.Profile
		var err error

		if len(args) > 0 {
			profile, err = profileMgr.GetProfile(args[0])
		} else {
			profile, err = profileMgr.GetCurrentProfile()
		}

		if err != nil {
			formatter.PrintError(err)
			return err
		}

		return formatter.Print(profile)
	},
}

// profileSwitchCmd 切换profile
var profileSwitchCmd = &cobra.Command{
	Use:   "switch <name>",
	Short: "切换profile",
	Long:  "切换到指定的配置Profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if err := profileMgr.SwitchProfile(name); err != nil {
			formatter.PrintError(err)
			return err
		}

		formatter.PrintSuccess(fmt.Sprintf("已切换到 profile '%s'", name))

		profile, _ := profileMgr.GetProfile(name)
		return formatter.Print(map[string]interface{}{
			"name":     name,
			"chain_id": profile.ChainID,
		})
	},
}

// profileCurrentCmd 显示当前profile
var profileCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "显示当前profile",
	Long:  "显示当前使用的配置Profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		profile, err := profileMgr.GetCurrentProfile()
		if err != nil {
			formatter.PrintError(err)
			return err
		}

		return formatter.Print(map[string]interface{}{
			"name":     profile.Name,
			"chain_id": profile.ChainID,
		})
	},
}

// profileCreateCmd 创建新profile
var profileCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "创建新profile",
	Long:  "创建一个新的配置Profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// 检查是否已存在
		if _, err := profileMgr.GetProfile(name); err == nil {
			return fmt.Errorf("profile '%s' 已存在", name)
		}

		// 提示输入配置
		fmt.Printf("创建 profile '%s'\n", name)
		fmt.Print("Chain ID: ")
		var chainID string
		if _, err := fmt.Scanln(&chainID); err != nil {
			return fmt.Errorf("读取 Chain ID 失败: %w", err)
		}

		fmt.Print("JSON-RPC URL: ")
		var jsonrpcURL string
		if _, err := fmt.Scanln(&jsonrpcURL); err != nil {
			return fmt.Errorf("读取 JSON-RPC URL 失败: %w", err)
		}

		// 创建profile
		profile := &config.Profile{
			Name:    name,
			ChainID: chainID,
			Endpoints: []config.EndpointConfig{
				{
					Name:     name + "-primary",
					Priority: 1,
					JSONRPC:  jsonrpcURL,
				},
			},
			Timeout:             config.Duration(30 * 1000000000), // 30s
			RetryAttempts:       3,
			RetryBackoff:        config.Duration(1 * 1000000000),  // 1s
			HealthCheckInterval: config.Duration(30 * 1000000000), // 30s
		}

		// 保存profile
		if err := profileMgr.SaveProfile(profile); err != nil {
			return fmt.Errorf("保存 profile 失败: %w", err)
		}

		formatter.PrintSuccess(fmt.Sprintf("Profile '%s' 创建成功", name))

		return formatter.Print(map[string]interface{}{
			"name":     name,
			"chain_id": chainID,
		})
	},
}

// profileImportCmd 导入profile
var profileImportCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "导入profile",
	Long:  "从JSON文件导入配置Profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]

		// 读取文件
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("读取文件失败: %w", err)
		}

		// 解析JSON
		var profile config.Profile
		if err := json.Unmarshal(data, &profile); err != nil {
			return fmt.Errorf("解析JSON失败: %w", err)
		}

		// 检查是否已存在
		if _, err := profileMgr.GetProfile(profile.Name); err == nil {
			return fmt.Errorf("profile '%s' 已存在", profile.Name)
		}

		// 保存profile
		if err := profileMgr.SaveProfile(&profile); err != nil {
			return fmt.Errorf("保存 profile 失败: %w", err)
		}

		formatter.PrintSuccess(fmt.Sprintf("Profile '%s' 导入成功", profile.Name))

		return formatter.Print(map[string]interface{}{
			"name":     profile.Name,
			"chain_id": profile.ChainID,
		})
	},
}

// profileExportCmd 导出profile
var profileExportCmd = &cobra.Command{
	Use:   "export <name> [file]",
	Short: "导出profile",
	Long:  "将配置Profile导出为JSON文件",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// 获取profile
		profile, err := profileMgr.GetProfile(name)
		if err != nil {
			return fmt.Errorf("获取 profile 失败: %w", err)
		}

		// 序列化JSON
		data, err := json.MarshalIndent(profile, "", "  ")
		if err != nil {
			return fmt.Errorf("序列化JSON失败: %w", err)
		}

		// 确定输出文件
		outputFile := name + "-profile.json"
		if len(args) > 1 {
			outputFile = args[1]
		}

		// 写入文件
		if err := os.WriteFile(outputFile, data, 0600); err != nil {
			return fmt.Errorf("写入文件失败: %w", err)
		}

		formatter.PrintSuccess(fmt.Sprintf("Profile '%s' 已导出到 %s", name, outputFile))

		return formatter.Print(map[string]interface{}{
			"profile": name,
			"file":    outputFile,
		})
	},
}

// profileDeleteCmd 删除profile
var profileDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "删除profile",
	Long:  "删除指定的配置Profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// 获取当前profile
		currentProfile, _ := profileMgr.GetCurrentProfile()
		if currentProfile != nil && currentProfile.Name == name {
			return fmt.Errorf("不能删除当前正在使用的 profile")
		}

		// 确认删除
		fmt.Printf("确认删除 profile '%s'? (yes/no): ", name)
		var confirm string
		if _, err := fmt.Scanln(&confirm); err != nil {
			return fmt.Errorf("读取输入失败: %w", err)
		}
		if strings.ToLower(confirm) != "yes" {
			formatter.PrintInfo("取消删除")
			return nil
		}

		// 删除profile
		if err := profileMgr.DeleteProfile(name); err != nil {
			return fmt.Errorf("删除 profile 失败: %w", err)
		}

		formatter.PrintSuccess(fmt.Sprintf("Profile '%s' 已删除", name))
		return nil
	},
}

func init() {
	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileShowCmd)
	profileCmd.AddCommand(profileSwitchCmd)
	profileCmd.AddCommand(profileCurrentCmd)
	profileCmd.AddCommand(profileCreateCmd)
	profileCmd.AddCommand(profileImportCmd)
	profileCmd.AddCommand(profileExportCmd)
	profileCmd.AddCommand(profileDeleteCmd)
}
