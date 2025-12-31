package main

import (
	"context"
	"log"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/weisyn/v1/client/core/transport"
)

var (
	blockFullTx   bool   // 是否包含完整交易
	blockAtHeight uint64 // 状态锚定高度
)

// blockCmd 区块相关命令
var blockCmd = &cobra.Command{
	Use:   "block",
	Short: "查询区块信息",
	Long:  "查询区块信息,支持按高度或哈希查询",
}

// blockGetCmd 获取区块
var blockGetCmd = &cobra.Command{
	Use:   "get <height|hash>",
	Short: "获取区块",
	Long:  "根据高度或哈希获取区块信息",
	Args:  cobra.ExactArgs(1),
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

		blockID := args[0]
		var block interface{}

		// 判断是高度还是哈希
		if height, err := strconv.ParseUint(blockID, 10, 64); err == nil {
			// 按高度查询
			var anchor *transport.StateAnchor
			if blockAtHeight != 0 {
				h := blockAtHeight
				anchor = &transport.StateAnchor{Height: &h}
			}

			block, err = client.GetBlockByHeight(ctx, height, blockFullTx, anchor)
			if err != nil {
				formatter.PrintError(err)
				return err
			}
		} else {
			// 按哈希查询
			block, err = client.GetBlockByHash(ctx, blockID, blockFullTx)
			if err != nil {
				formatter.PrintError(err)
				return err
			}
		}

		return formatter.Print(block)
	},
}

func init() {
	blockCmd.AddCommand(blockGetCmd)

	// 添加标志
	blockGetCmd.Flags().BoolVar(&blockFullTx, "full-tx", false, "包含完整交易信息")
	blockGetCmd.Flags().Uint64Var(&blockAtHeight, "at-height", 0, "在指定高度查询(状态锚定)")
}
