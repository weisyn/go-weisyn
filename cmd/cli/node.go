package main

import (
	"context"
	"fmt"
	"log"

	"github.com/spf13/cobra"
)

// nodeCmd 节点相关命令
var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "节点管理",
	Long:  "查询和管理节点状态、连接信息",
}

// nodeInfoCmd 查询节点信息
var nodeInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "查询节点信息",
	Long:  "查询节点的基本信息：版本、网络ID、同步状态等",
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

		// 获取链ID
		chainID, err := client.ChainID(ctx)
		if err != nil {
			return fmt.Errorf("获取链ID失败: %w", err)
		}

		// 获取最新区块高度
		blockNumber, err := client.BlockNumber(ctx)
		if err != nil {
			return fmt.Errorf("获取区块高度失败: %w", err)
		}

		// 获取同步状态
		syncStatus, err := client.Syncing(ctx)
		if err != nil {
			return fmt.Errorf("获取同步状态失败: %w", err)
		}

		// Ping 检查连通性
		if err := client.Ping(ctx); err != nil {
			formatter.PrintWarning("节点连接异常")
		}

		formatter.PrintInfo("节点信息:")

		nodeInfo := map[string]interface{}{
			"chain_id":     chainID,
			"block_height": blockNumber,
			"syncing":      syncStatus.Syncing,
		}

		if syncStatus.Syncing {
			progress := float64(syncStatus.CurrentBlock-syncStatus.StartingBlock) /
				float64(syncStatus.HighestBlock-syncStatus.StartingBlock) * 100

			nodeInfo["sync_progress"] = fmt.Sprintf("%.2f%%", progress)
			nodeInfo["current_block"] = syncStatus.CurrentBlock
			nodeInfo["highest_block"] = syncStatus.HighestBlock
		}

		return formatter.Print(nodeInfo)
	},
}

// nodeHealthCmd 检查节点健康状态
var nodeHealthCmd = &cobra.Command{
	Use:   "health",
	Short: "检查节点健康状态",
	Long:  "检查节点是否可达及基本健康状况",
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

		// Ping 检查
		if err := client.Ping(ctx); err != nil {
			formatter.PrintError(fmt.Errorf("节点不可达: %w", err))
			return formatter.Print(map[string]interface{}{
				"status": "unhealthy",
				"error":  err.Error(),
			})
		}

		// 检查同步状态
		syncStatus, err := client.Syncing(ctx)
		if err != nil {
			formatter.PrintWarning("无法获取同步状态")
		}

		// 检查交易池
		txPoolStatus, err := client.TxPoolStatus(ctx)
		if err != nil {
			formatter.PrintWarning("无法获取交易池状态")
		}

		formatter.PrintSuccess("节点健康")

		health := map[string]interface{}{
			"status": "healthy",
			"ping":   "ok",
		}

		if syncStatus != nil {
			health["syncing"] = syncStatus.Syncing
			if syncStatus.Syncing {
				progress := float64(syncStatus.CurrentBlock-syncStatus.StartingBlock) /
					float64(syncStatus.HighestBlock-syncStatus.StartingBlock) * 100
				health["sync_progress"] = fmt.Sprintf("%.2f%%", progress)
			}
		}

		if txPoolStatus != nil {
			health["txpool_pending"] = txPoolStatus.Pending
			health["txpool_queued"] = txPoolStatus.Queued
		}

		return formatter.Print(health)
	},
}

// nodePeersCmd 查询节点连接的对等节点
var nodePeersCmd = &cobra.Command{
	Use:   "peers",
	Short: "查询对等节点",
	Long:  "查询节点当前连接的对等节点信息",
	RunE: func(cmd *cobra.Command, args []string) error {
		formatter.PrintInfo("对等节点查询功能需要节点提供专用的 peers API")
		formatter.PrintInfo("当前版本通过 chain syncing 命令查看网络状态")

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

		// 获取同步状态（包含网络信息）
		syncStatus, err := client.Syncing(ctx)
		if err != nil {
			return fmt.Errorf("获取网络状态失败: %w", err)
		}

		networkInfo := map[string]interface{}{
			"syncing":       syncStatus.Syncing,
			"current_block": syncStatus.CurrentBlock,
			"highest_block": syncStatus.HighestBlock,
		}

		if !syncStatus.Syncing {
			formatter.PrintSuccess("节点已完全同步")
		} else {
			progress := float64(syncStatus.CurrentBlock-syncStatus.StartingBlock) /
				float64(syncStatus.HighestBlock-syncStatus.StartingBlock) * 100
			formatter.PrintInfo(fmt.Sprintf("同步进度: %.2f%%", progress))
		}

		return formatter.Print(networkInfo)
	},
}

// nodeConnectCmd 主动连接指定 peer
var nodeConnectCmd = &cobra.Command{
	Use:   "connect",
	Short: "主动连接指定 P2P 节点",
	Long: `主动连接指定的 P2P 节点（peerId）。

该命令通过管理面 JSON-RPC 方法 wes_admin_connectPeer 通知节点发起拨号，
适用于公有链/联盟链中已知节点的快速连通性诊断和拓扑增强。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if nodeConnectPeerID == "" {
			return fmt.Errorf("必须指定 --peer-id")
		}

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

		// 直接通过底层 transport 客户端调用管理面 JSON-RPC
		type adminConnectParams struct {
			Multiaddrs []string `json:"multiaddrs,omitempty"`
			TimeoutMs  int      `json:"timeoutMs,omitempty"`
		}

		params := []interface{}{nodeConnectPeerID}
		if len(nodeConnectAddrs) > 0 || nodeConnectTimeout > 0 {
			opts := adminConnectParams{
				Multiaddrs: nodeConnectAddrs,
				TimeoutMs:  nodeConnectTimeout,
			}
			params = append(params, opts)
		}

		// client 是 transport.Client，直接使用 CallRaw
		result, err := client.CallRaw(ctx, "wes_admin_connectPeer", params)
		if err != nil {
			return fmt.Errorf("调用 wes_admin_connectPeer 失败: %w", err)
		}

		return formatter.Print(result)
	},
}

var (
	nodeConnectPeerID string
	nodeConnectAddrs  []string
	nodeConnectTimeout int
)

func init() {
	nodeCmd.AddCommand(nodeInfoCmd)
	nodeCmd.AddCommand(nodeHealthCmd)
	nodeCmd.AddCommand(nodePeersCmd)
	nodeCmd.AddCommand(nodeConnectCmd)

	// node connect flags
	nodeConnectCmd.Flags().StringVar(&nodeConnectPeerID, "peer-id", "", "目标节点的 libp2p PeerID（必填）")
	nodeConnectCmd.Flags().StringSliceVar(&nodeConnectAddrs, "addr", nil, "可选的 multiaddr 地址（可多次指定）")
	nodeConnectCmd.Flags().IntVar(&nodeConnectTimeout, "timeout", 10000, "拨号超时时间（毫秒），默认 10000")

	// 添加到root命令
	rootCmd.AddCommand(nodeCmd)
}
