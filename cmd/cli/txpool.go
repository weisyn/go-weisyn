package main

import (
	"context"
	"fmt"
	"log"

	"github.com/spf13/cobra"
)

// txpoolCmd 交易池相关命令
var txpoolCmd = &cobra.Command{
	Use:   "txpool",
	Short: "交易池管理",
	Long:  "查询和管理交易池状态",
}

// txpoolStatusCmd 查询交易池状态
var txpoolStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "查询交易池状态",
	Long:  "查询交易池中待处理和排队的交易数量",
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

		// 查询状态
		status, err := client.TxPoolStatus(ctx)
		if err != nil {
			formatter.PrintError(err)
			return err
		}

		formatter.PrintInfo("交易池状态:")

		return formatter.Print(map[string]interface{}{
			"pending": status.Pending,
			"queued":  status.Queued,
			"total":   status.Total,
		})
	},
}

// txpoolContentCmd 查询交易池内容
var txpoolContentCmd = &cobra.Command{
	Use:   "content",
	Short: "查询交易池内容",
	Long:  "查询交易池中的详细交易内容",
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

		// 查询内容
		content, err := client.TxPoolContent(ctx)
		if err != nil {
			formatter.PrintError(err)
			return err
		}

		// 统计信息
		pendingCount := 0
		queuedCount := 0
		for _, txs := range content.Pending {
			pendingCount += len(txs)
		}
		for _, txs := range content.Queued {
			queuedCount += len(txs)
		}

		formatter.PrintInfo(fmt.Sprintf("待处理交易: %d 个地址, %d 笔交易", len(content.Pending), pendingCount))
		formatter.PrintInfo(fmt.Sprintf("排队交易: %d 个地址, %d 笔交易", len(content.Queued), queuedCount))

		return formatter.Print(map[string]interface{}{
			"pending": content.Pending,
			"queued":  content.Queued,
		})
	},
}

func init() {
	txpoolCmd.AddCommand(txpoolStatusCmd)
	txpoolCmd.AddCommand(txpoolContentCmd)

	// 添加到root命令
	rootCmd.AddCommand(txpoolCmd)
}
