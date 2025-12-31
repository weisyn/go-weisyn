package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/weisyn/v1/client/core/transport"
	"github.com/weisyn/v1/client/pkg/transport/api"
	"github.com/weisyn/v1/client/pkg/ux/flows"
	"github.com/weisyn/v1/pkg/utils"
)

var contractBalanceSpecs []string

var accountContractBalancesCmd = &cobra.Command{
	Use:   "contract-balances <address>",
	Short: "查询账户的原生币与合约代币余额",
	Long: `查询指定地址的主币余额，并按配置的合约列表依次调用 BalanceOf() 汇总合约代币余额。

示例：
  wes account contract-balances CZcgQm... \
    --contract simple-token=dded4d5f563f5c59c21cc3c39a8fae9fb2e9a75f27ee9e98e6775167fc249395:default`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		address := strings.TrimSpace(args[0])
		if address == "" {
			return fmt.Errorf("地址不能为空")
		}

		if len(contractBalanceSpecs) == 0 {
			return fmt.Errorf("至少指定一个 --contract 选项，格式 label=contentHash[:tokenID]")
		}

		specs, err := parseContractBalanceSpecs(contractBalanceSpecs)
		if err != nil {
			return err
		}

		// 建立客户端
		client, err := getClient()
		if err != nil {
			return err
		}
		defer func() {
			if cerr := client.Close(); cerr != nil {
				formatter.PrintWarning(fmt.Sprintf("关闭客户端失败: %v", cerr))
			}
		}()

		ctx := context.Background()

		// 查询原生币余额
		nativeInfo, err := queryNativeBalance(ctx, client, address)
		if err != nil {
			return err
		}

		// 查询合约代币余额
		balanceAdapter := api.NewContractBalanceAdapter(client)
		tokenBalanceResults, err := balanceAdapter.FetchBalances(ctx, address, specs)
		if err != nil {
			return err
		}

		tokenOutputs := make([]map[string]interface{}, 0, len(tokenBalanceResults))
		for i, tb := range tokenBalanceResults {
			spec := specs[i]
			tokenOutputs = append(tokenOutputs, map[string]interface{}{
				"label":         spec.Label,
				"content_hash":  spec.ContentHash,
				"token_id":      spec.TokenID,
				"balance":       tb.Amount,
				"balance_human": utils.FormatWeiToDecimal(tb.Amount),
			})
		}

		output := map[string]interface{}{
			"address": address,
			"native":  nativeInfo,
			"tokens":  tokenOutputs,
		}

		return formatter.Print(output)
	},
}

func parseContractBalanceSpecs(specStrings []string) ([]flows.ContractTokenSpec, error) {
	configs := make([]flows.ContractTokenSpec, 0, len(specStrings))
	for _, s := range specStrings {
		spec := strings.TrimSpace(s)
		if spec == "" {
			continue
		}

		parts := strings.SplitN(spec, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("无效的 --contract 参数: %q，期望格式 label=contentHash[:tokenID]", spec)
		}

		label := strings.TrimSpace(parts[0])
		if label == "" {
			return nil, fmt.Errorf("无效的 --contract 参数: %q，标签不能为空", spec)
		}

		value := strings.TrimSpace(parts[1])
		valueParts := strings.Split(value, ":")
		contentHash := strings.TrimSpace(valueParts[0])
		if len(contentHash) != 64 || !isHexString(contentHash) {
			return nil, fmt.Errorf("无效的 contentHash: %q，应为64位十六进制字符串", contentHash)
		}

		tokenID := ""
		if len(valueParts) > 1 {
			tokenID = strings.TrimSpace(valueParts[1])
		}
		if tokenID == "" {
			tokenID = "default"
		}

		configs = append(configs, flows.ContractTokenSpec{
			Label:       label,
			ContentHash: strings.ToLower(contentHash),
			TokenID:     tokenID,
		})
	}

	return configs, nil
}

func isHexString(v string) bool {
	for _, r := range v {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
			continue
		}
		return false
	}
	return true
}

func queryNativeBalance(ctx context.Context, client transport.Client, address string) (map[string]interface{}, error) {
	balance, err := client.GetBalance(ctx, address, nil)
	if err != nil {
		return nil, fmt.Errorf("查询原生币余额失败: %w", err)
	}

	raw := strings.TrimPrefix(balance.Balance, "0x")
	if raw == "" {
		raw = "0"
	}

	value, err := strconv.ParseUint(raw, 16, 64)
	if err != nil {
		return nil, fmt.Errorf("解析原生币余额失败: %w", err)
	}

	return map[string]interface{}{
		"raw":       value,
		"formatted": utils.FormatWeiToDecimal(value) + " WES",
		"height":    balance.Height,
		"hash":      balance.Hash,
	}, nil
}

func init() {
	accountContractBalancesCmd.Flags().StringArrayVar(&contractBalanceSpecs, "contract", nil, "合约配置，格式 label=contentHash[:tokenID]，可重复指定")

	accountCmd.AddCommand(accountContractBalancesCmd)
}
