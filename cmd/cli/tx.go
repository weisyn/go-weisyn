package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/weisyn/v1/client/core/builder"
	"github.com/weisyn/v1/client/core/wallet"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/address"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/key"
)

var (
	// tx build 标志
	txFrom   string
	txTo     string
	txAmount string
	txOutput string

	// tx sign 标志
	txFile     string
	txPassword string

	// tx send 标志
	txWait          bool
	txConfirmations int
)

// txCmd 交易相关命令
var txCmd = &cobra.Command{
	Use:   "tx",
	Short: "交易管理",
	Long:  "构建、签名、发送和查询交易",
}

// txBuildCmd 构建交易
var txBuildCmd = &cobra.Command{
	Use:   "build",
	Short: "构建交易",
	Long:  "构建交易草稿,可以是转账、合约部署或合约调用",
}

// txBuildTransferCmd 构建转账交易
var txBuildTransferCmd = &cobra.Command{
	Use:   "transfer",
	Short: "构建转账交易",
	Long:  "构建普通转账交易,自动选择UTXO并计算找零",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 验证参数
		if txFrom == "" || txTo == "" || txAmount == "" {
			return fmt.Errorf("必须指定 --from, --to 和 --amount")
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

		// 创建地址服务
		keyManager := key.NewKeyManager()
		addressManager := address.NewAddressService(keyManager)

		// 创建转账构建器
		tb := builder.NewTransferBuilder(client, addressManager)

		// 解析金额
		amount, err := builder.NewAmountFromString(txAmount)
		if err != nil {
			formatter.PrintError(fmt.Errorf("invalid amount: %w", err))
			return err
		}

		// 构建交易草稿
		draft, err := tb.Build(ctx, &builder.TransferRequest{
			From:   txFrom,
			To:     txTo,
			Amount: amount,
			Memo:   "",
		})
		if err != nil {
			formatter.PrintError(err)
			return err
		}

		// 保存草稿
		if txOutput == "" {
			txOutput = "draft.json"
		}

		if err := draft.Save(txOutput); err != nil {
			formatter.PrintError(err)
			return err
		}

		formatter.PrintSuccess(fmt.Sprintf("交易草稿已保存到 %s", txOutput))

		return formatter.Print(map[string]interface{}{
			"draft_file": txOutput,
			"from":       txFrom,
			"to":         txTo,
			"amount":     txAmount,
		})
	},
}

// txSealCmd 密封交易
var txSealCmd = &cobra.Command{
	Use:   "seal",
	Short: "密封交易",
	Long:  "密封交易草稿,使其不可再修改并计算TxID",
	RunE: func(cmd *cobra.Command, args []string) error {
		if txFile == "" {
			return fmt.Errorf("必须指定 --tx 参数")
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

		// 加载草稿
		txBuilder := builder.NewTxBuilder(client)
		draft, err := txBuilder.LoadDraft(txFile)
		if err != nil {
			formatter.PrintError(err)
			return err
		}

		// 密封
		composed, err := draft.Seal()
		if err != nil {
			formatter.PrintError(err)
			return err
		}

		// 保存组合交易
		composedFile := "composed.json"
		if err := composed.Save(composedFile); err != nil {
			formatter.PrintError(err)
			return err
		}

		formatter.PrintSuccess(fmt.Sprintf("交易已密封,TxID: %s", composed.TxID()))

		return formatter.Print(map[string]interface{}{
			"tx_id":         composed.TxID(),
			"composed_file": composedFile,
		})
	},
}

// txGetCmd 查询交易
var txGetCmd = &cobra.Command{
	Use:   "get <tx_hash>",
	Short: "查询交易",
	Long:  "根据交易哈希查询交易信息",
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
		txHash := args[0]

		// 查询交易
		tx, err := client.GetTransaction(ctx, txHash)
		if err != nil {
			formatter.PrintError(err)
			return err
		}

		return formatter.Print(tx)
	},
}

// txReceiptCmd 查询交易回执
var txReceiptCmd = &cobra.Command{
	Use:   "receipt <tx_hash>",
	Short: "查询交易回执",
	Long:  "根据交易哈希查询交易执行回执",
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
		txHash := args[0]

		// 查询回执
		receipt, err := client.GetTransactionReceipt(ctx, txHash)
		if err != nil {
			formatter.PrintError(err)
			return err
		}

		return formatter.Print(receipt)
	},
}

// txSendCmd 发送交易
var txSendCmd = &cobra.Command{
	Use:   "send",
	Short: "发送交易",
	Long:  "发送已签名的交易到节点",
	RunE: func(cmd *cobra.Command, args []string) error {
		if txFile == "" {
			return fmt.Errorf("必须指定 --file 参数")
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

		// 读取签名交易文件
		data, err := os.ReadFile(txFile)
		if err != nil {
			return fmt.Errorf("读取交易文件: %w", err)
		}

		// 简化处理:假设文件中是原始交易十六进制
		rawTxHex := string(data)

		// 发送交易
		result, err := client.SendRawTransaction(ctx, rawTxHex)
		if err != nil {
			formatter.PrintError(err)
			return err
		}

		if !result.Accepted {
			formatter.PrintWarning(fmt.Sprintf("交易被拒绝: %s", result.Reason))
			return formatter.Print(result)
		}

		formatter.PrintSuccess(fmt.Sprintf("交易已提交: %s", result.TxHash))

		return formatter.Print(result)
	},
}

// txSignCmd 签名交易
var txSignCmd = &cobra.Command{
	Use:   "sign <composed-tx-file>",
	Short: "签名交易",
	Long:  "使用keystore中的私钥对交易进行签名",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		composedFile := args[0]

		// 获取当前profile
		profile, err := profileMgr.GetCurrentProfile()
		if err != nil {
			return err
		}

		// 读取交易文件
		data, err := os.ReadFile(composedFile)
		if err != nil {
			return fmt.Errorf("读取交易文件: %w", err)
		}

		// 解析交易（简化：假设是JSON格式的ComposedTx）
		var composedData struct {
			TxID    string `json:"tx_id"`
			From    string `json:"from"`
			RawData string `json:"raw_data"` // 待签名的原始数据
		}
		if err := json.Unmarshal(data, &composedData); err != nil {
			return fmt.Errorf("解析交易文件: %w", err)
		}

		// 获取发送方地址
		fromAddress := composedData.From
		if fromAddress == "" {
			return fmt.Errorf("交易中未找到发送方地址")
		}

		// 创建账户管理器（使用标准的 AddressManager 生成 Base58Check 地址）
		keyManager := key.NewKeyManager()
		addressManager := address.NewAddressService(keyManager)
		am, err := wallet.NewAccountManager(profile.KeystorePath, addressManager)
		if err != nil {
			return fmt.Errorf("初始化账户管理器: %w", err)
		}

		// 获取账户信息
		account, err := am.GetAccount(fromAddress)
		if err != nil {
			return fmt.Errorf("获取账户失败: %w (请确保该地址的keystore存在于当前profile)", err)
		}

		// 提示输入密码
		password, err := promptPassword("请输入keystore密码")
		if err != nil {
			return err
		}

		// 创建签名器
		signer, err := wallet.NewKeystoreSigner(account.KeystorePath, fromAddress)
		if err != nil {
			return fmt.Errorf("创建签名器失败: %w", err)
		}

		// 解锁keystore
		if err := signer.Unlock(password, 0); err != nil {
			return fmt.Errorf("解锁keystore失败: %w (密码错误?)", err)
		}
		defer signer.Lock()

		// 签名交易（将raw_data从hex解码为字节数组）
		// 这里简化处理，实际需要根据交易格式解析
		txBytes := []byte(composedData.RawData)
		signature, err := signer.Sign(txBytes, fromAddress)
		if err != nil {
			return fmt.Errorf("签名失败: %w", err)
		}

		// 构建签名交易输出
		signedTx := map[string]interface{}{
			"tx_id":     composedData.TxID,
			"from":      fromAddress,
			"signature": fmt.Sprintf("0x%x", signature),
			"raw_hex":   fmt.Sprintf("0x%x", txBytes), // 简化：实际应该组合原始交易+签名
		}

		// 保存签名交易
		signedFile := "signed.json"
		if txOutput != "" {
			signedFile = txOutput
		}

		signedData, err := json.MarshalIndent(signedTx, "", "  ")
		if err != nil {
			return fmt.Errorf("序列化签名交易: %w", err)
		}

		if err := os.WriteFile(signedFile, signedData, 0600); err != nil {
			return fmt.Errorf("保存签名交易: %w", err)
		}

		formatter.PrintSuccess(fmt.Sprintf("交易已签名，已保存到: %s", signedFile))

		return formatter.Print(map[string]interface{}{
			"tx_id":       composedData.TxID,
			"from":        fromAddress,
			"signed_file": signedFile,
		})
	},
}

func init() {
	// 构建命令
	txCmd.AddCommand(txBuildCmd)
	txBuildCmd.AddCommand(txBuildTransferCmd)
	txBuildTransferCmd.Flags().StringVar(&txFrom, "from", "", "发送方地址 (必需)")
	txBuildTransferCmd.Flags().StringVar(&txTo, "to", "", "接收方地址 (必需)")
	txBuildTransferCmd.Flags().StringVar(&txAmount, "amount", "", "转账金额 (必需)")
	txBuildTransferCmd.Flags().StringVarP(&txOutput, "output", "o", "", "输出文件 (默认: draft.json)")

	// 密封命令
	txCmd.AddCommand(txSealCmd)
	txSealCmd.Flags().StringVar(&txFile, "tx", "", "交易草稿文件 (必需)")

	// 查询命令
	txCmd.AddCommand(txGetCmd)
	txCmd.AddCommand(txReceiptCmd)

	// 签名命令
	txCmd.AddCommand(txSignCmd)
	txSignCmd.Flags().StringVarP(&txOutput, "output", "o", "", "输出文件 (默认: signed.json)")

	// 发送命令
	txCmd.AddCommand(txSendCmd)
	txSendCmd.Flags().StringVar(&txFile, "file", "", "签名交易文件 (必需)")
	txSendCmd.Flags().BoolVar(&txWait, "wait", false, "等待交易确认")
	txSendCmd.Flags().IntVar(&txConfirmations, "confirmations", 1, "等待的确认数")
}
