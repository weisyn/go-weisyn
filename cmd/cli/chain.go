package main

import (
	"context"
	"fmt"
	"log"

	"github.com/spf13/cobra"
)

// chainCmd 链相关命令
var chainCmd = &cobra.Command{
	Use:   "chain",
	Short: "查询区块链状态",
	Long:  "查询区块链状态信息,包括链ID、同步状态、最新区块高度等",
}

// chainInfoCmd 查询链信息
var chainInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "查询链信息",
	Long:  "查询链ID、链名称等基本信息",
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
			formatter.PrintError(err)
			return err
		}

		// 获取最新区块高度
		height, err := client.BlockNumber(ctx)
		if err != nil {
			formatter.PrintError(err)
			return err
		}

		// 构建输出
		info := map[string]interface{}{
			"chain_id":     chainID,
			"latest_block": height,
		}

		return formatter.Print(info)
	},
}

// chainSyncingCmd 查询同步状态
var chainSyncingCmd = &cobra.Command{
	Use:   "syncing",
	Short: "查询同步状态",
	Long:  "查询节点是否正在同步,以及同步进度",
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

		// 获取同步状态
		syncStatus, err := client.Syncing(ctx)
		if err != nil {
			formatter.PrintError(err)
			return err
		}

		if !syncStatus.Syncing {
			formatter.PrintInfo("节点已同步完成")
			return formatter.Print(map[string]interface{}{
				"syncing": false,
			})
		}

		// 计算同步进度
		progress := float64(syncStatus.CurrentBlock-syncStatus.StartingBlock) /
			float64(syncStatus.HighestBlock-syncStatus.StartingBlock) * 100

		result := map[string]interface{}{
			"syncing":        true,
			"starting_block": syncStatus.StartingBlock,
			"current_block":  syncStatus.CurrentBlock,
			"highest_block":  syncStatus.HighestBlock,
			"progress":       fmt.Sprintf("%.2f%%", progress),
		}

		return formatter.Print(result)
	},
}

// chainHeadCmd 查询链头
var chainHeadCmd = &cobra.Command{
	Use:   "head",
	Short: "查询链头区块",
	Long:  "查询最新区块的高度和哈希",
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

		// 获取最新区块高度
		height, err := client.BlockNumber(ctx)
		if err != nil {
			formatter.PrintError(err)
			return err
		}

		// 获取最新区块
		block, err := client.GetBlockByHeight(ctx, height, false, nil)
		if err != nil {
			formatter.PrintError(err)
			return err
		}

		result := map[string]interface{}{
			"height":    block.Height,
			"hash":      block.Hash,
			"timestamp": block.Timestamp,
			"tx_count":  block.TxCount,
		}

		return formatter.Print(result)
	},
}

func init() {
	chainCmd.AddCommand(chainInfoCmd)
	chainCmd.AddCommand(chainSyncingCmd)
	chainCmd.AddCommand(chainHeadCmd)
}
