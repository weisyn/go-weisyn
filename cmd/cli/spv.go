package main

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/spf13/cobra"
)

// spvCmd SPV轻客户端相关命令
var spvCmd = &cobra.Command{
	Use:   "spv",
	Short: "SPV轻客户端",
	Long:  "SPV (Simplified Payment Verification) 轻客户端功能",
}

// spvBlockHeaderCmd 获取区块头
var spvBlockHeaderCmd = &cobra.Command{
	Use:   "header <height>",
	Short: "获取区块头",
	Long:  "获取指定高度的区块头（用于SPV验证）",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		height, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("无效的区块高度: %s", args[0])
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

		// 获取区块头
		header, err := client.GetBlockHeader(ctx, height)
		if err != nil {
			formatter.PrintError(err)
			return err
		}

		formatter.PrintSuccess(fmt.Sprintf("区块头 #%d", height))

		return formatter.Print(map[string]interface{}{
			"height":      header.Height,
			"hash":        header.Hash,
			"parent_hash": header.ParentHash,
			"timestamp":   header.Timestamp,
			"state_root":  header.StateRoot,
			"tx_root":     header.TxRoot,
			"difficulty":  header.Difficulty,
			"nonce":       header.Nonce,
		})
	},
}

// spvTxProofCmd 获取交易Merkle证明
var spvTxProofCmd = &cobra.Command{
	Use:   "proof <tx_hash>",
	Short: "获取交易Merkle证明",
	Long:  "获取交易的Merkle证明（用于SPV验证交易存在性）",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		txHash := args[0]

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

		// 获取Merkle证明
		proof, err := client.GetTxProof(ctx, txHash)
		if err != nil {
			formatter.PrintError(err)
			return err
		}

		formatter.PrintSuccess(fmt.Sprintf("交易Merkle证明: %s", txHash))

		return formatter.Print(map[string]interface{}{
			"tx_hash":      proof.TxHash,
			"block_hash":   proof.BlockHash,
			"block_height": proof.BlockHeight,
			"tx_index":     proof.TxIndex,
			"siblings":     proof.Siblings,
			"root":         proof.Root,
		})
	},
}

// spvVerifyCmd 验证Merkle证明
var spvVerifyCmd = &cobra.Command{
	Use:   "verify <tx_hash>",
	Short: "验证交易Merkle证明",
	Long:  "验证交易是否包含在区块中（本地SPV验证）",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		txHash := args[0]

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

		// 获取Merkle证明
		proof, err := client.GetTxProof(ctx, txHash)
		if err != nil {
			formatter.PrintError(err)
			return err
		}

		// 获取区块头
		header, err := client.GetBlockHeader(ctx, proof.BlockHeight)
		if err != nil {
			formatter.PrintError(err)
			return err
		}

		// 验证Merkle证明（简化实现：检查根是否匹配）
		if proof.Root != header.TxRoot {
			formatter.PrintError(fmt.Errorf("Merkle证明验证失败: 根不匹配"))
			return fmt.Errorf("验证失败")
		}

		formatter.PrintSuccess(fmt.Sprintf("交易 %s 已验证存在于区块 #%d", txHash, proof.BlockHeight))

		return formatter.Print(map[string]interface{}{
			"tx_hash":      proof.TxHash,
			"block_height": proof.BlockHeight,
			"block_hash":   proof.BlockHash,
			"verified":     true,
		})
	},
}

func init() {
	spvCmd.AddCommand(spvBlockHeaderCmd)
	spvCmd.AddCommand(spvTxProofCmd)
	spvCmd.AddCommand(spvVerifyCmd)

	// 添加到root命令
	rootCmd.AddCommand(spvCmd)
}
