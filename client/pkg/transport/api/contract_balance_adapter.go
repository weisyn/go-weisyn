package api

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/weisyn/v1/client/core/transport"
	"github.com/weisyn/v1/client/pkg/ux/flows"
)

// ContractBalanceAdapter 基于 transport.Client 的合约代币余额查询实现
type ContractBalanceAdapter struct {
	transportClient transport.Client
}

// NewContractBalanceAdapter 创建合约代币余额查询适配器
func NewContractBalanceAdapter(client transport.Client) *ContractBalanceAdapter {
	return &ContractBalanceAdapter{
		transportClient: client,
	}
}

// FetchBalances 查询指定账户在配置合约下的代币余额
func (a *ContractBalanceAdapter) FetchBalances(
	ctx context.Context,
	ownerAddress string,
	specs []flows.ContractTokenSpec,
) ([]flows.TokenBalance, error) {
	if len(specs) == 0 {
		return nil, nil
	}

	results := make([]flows.TokenBalance, 0, len(specs))

	for _, spec := range specs {
		req := &transport.ContractTokenBalanceRequest{
			Address:     strings.TrimSpace(ownerAddress),
			ContentHash: strings.TrimPrefix(strings.TrimSpace(spec.ContentHash), "0x"),
			TokenID:     strings.TrimSpace(spec.TokenID),
		}

		resp, err := a.transportClient.GetContractTokenBalance(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("调用合约 %s 失败: %w", spec.Label, err)
		}

		var balanceValue uint64
		if resp.BalanceUint64 > 0 {
			balanceValue = resp.BalanceUint64
		} else if resp.Balance != "" {
			parsed, err := strconv.ParseUint(resp.Balance, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("解析合约 %s 余额失败: %w", spec.Label, err)
			}
			balanceValue = parsed
		}

		results = append(results, flows.TokenBalance{
			TokenID:   spec.TokenID,
			TokenName: spec.Label,
			Amount:    balanceValue,
		})
	}

	return results, nil
}
