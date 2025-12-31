package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	resourceType     string
	resourceFile     string
	resourceMetadata string
)

// resourceCmd 资源相关命令
var resourceCmd = &cobra.Command{
	Use:   "resource",
	Short: "资源管理",
	Long:  "部署、获取和管理链上资源（静态文件、数据等）",
}

// resourceDeployCmd 部署资源
var resourceDeployCmd = &cobra.Command{
	Use:   "deploy <file>",
	Short: "部署资源",
	Long:  "部署静态资源到区块链存储",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		file := args[0]

		formatter.PrintInfo(fmt.Sprintf("准备部署资源: %s", file))
		formatter.PrintInfo(fmt.Sprintf("资源类型: %s", resourceType))

		// 资源部署功能需要WES的URES（统一资源系统）支持
		// 实际实现需要：
		// 1. 读取文件内容
		// 2. 计算内容哈希（CID）
		// 3. 构建资源部署交易
		// 4. 提交到链上

		formatter.PrintWarning("资源部署功能需要节点启用URES（统一资源系统）")

		return formatter.Print(map[string]interface{}{
			"action":        "deploy_resource",
			"file":          file,
			"type":          resourceType,
			"status":        "ures_required",
			"message":       "请确保节点已启用URES功能",
			"documentation": "_docs/specs/ures/",
		})
	},
}

// resourceFetchCmd 获取资源
var resourceFetchCmd = &cobra.Command{
	Use:   "fetch <resource-id>",
	Short: "获取资源",
	Long:  "从区块链存储获取资源内容",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		resourceID := args[0]

		formatter.PrintInfo(fmt.Sprintf("获取资源: %s", resourceID))

		// 资源获取功能需要：
		// 1. 查询资源元数据
		// 2. 获取资源内容
		// 3. 验证内容完整性（CID校验）
		// 4. 保存到本地

		formatter.PrintWarning("资源获取功能需要节点启用URES（统一资源系统）")

		return formatter.Print(map[string]interface{}{
			"action":        "fetch_resource",
			"resource_id":   resourceID,
			"status":        "ures_required",
			"message":       "请确保节点已启用URES功能",
			"documentation": "_docs/specs/ures/",
		})
	},
}

// resourceListCmd 列出资源
var resourceListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出资源",
	Long:  "列出账户部署的所有资源",
	RunE: func(cmd *cobra.Command, args []string) error {
		formatter.PrintInfo("资源列表功能需要节点启用URES（统一资源系统）")

		// 资源列表功能需要：
		// 1. 查询指定账户的资源索引
		// 2. 获取资源元数据列表
		// 3. 格式化展示

		formatter.PrintWarning("当前节点未提供URES资源查询API")

		return formatter.Print(map[string]interface{}{
			"action":        "list_resources",
			"status":        "ures_required",
			"message":       "URES（统一资源系统）功能开发中",
			"documentation": "_docs/specs/ures/",
		})
	},
}

func init() {
	resourceCmd.AddCommand(resourceDeployCmd)
	resourceCmd.AddCommand(resourceFetchCmd)
	resourceCmd.AddCommand(resourceListCmd)

	// deploy 标志
	resourceDeployCmd.Flags().StringVarP(&resourceType, "type", "t", "static", "资源类型 (static/dynamic/ai-model)")
	resourceDeployCmd.Flags().StringVar(&resourceMetadata, "metadata", "", "资源元数据 (JSON格式)")

	// fetch 标志
	resourceFetchCmd.Flags().StringVarP(&resourceFile, "output", "o", "", "输出文件路径")

	// 添加到root命令
	rootCmd.AddCommand(resourceCmd)
}
