package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/weisyn/v1/client/pkg/transport/jsonrpc"
	"github.com/weisyn/v1/client/pkg/ux/flows"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/address"
)

// AccountAdapter 账户服务适配器（通过 JSON-RPC 连接到节点）
type AccountAdapter struct {
	client         *jsonrpc.Client
	addressManager *address.AddressService // 用于地址转换
}

// NewAccountAdapter 创建账户服务适配器
func NewAccountAdapter(client *jsonrpc.Client, addrMgr *address.AddressService) *AccountAdapter {
	return &AccountAdapter{
		client:         client,
		addressManager: addrMgr,
	}
}

// GetBalance 获取余额
func (a *AccountAdapter) GetBalance(ctx context.Context, address string) (uint64, []flows.TokenBalance, error) {
	// ✅ 验证地址格式（WES使用Base58格式，不兼容ETH的0x前缀格式）
	if a.addressManager == nil {
		return 0, nil, fmt.Errorf("address manager not available")
	}
	
	// 拒绝0x前缀的ETH地址格式
	if len(address) > 2 && (address[:2] == "0x" || address[:2] == "0X") {
		return 0, nil, fmt.Errorf("WES地址必须使用Base58格式，不支持0x前缀的ETH地址格式")
	}
	
	// 验证Base58格式地址
	validAddress, err := a.addressManager.StringToAddress(address)
	if err != nil {
		return 0, nil, fmt.Errorf("invalid address format: %w", err)
	}
	
	// ✅ 调用 wes_getBalance（传递Base58格式地址）
	result, err := a.client.Call(ctx, "wes_getBalance", validAddress)
	if err != nil {
		return 0, nil, fmt.Errorf("calling wes_getBalance: %w", err)
	}

	// ✅ 步骤3：解析返回的对象格式（包含 balance, height, hash 等）
	var response struct {
		Balance   string `json:"balance"`             // "0x..."
		Height    string `json:"height"`              // "0x..."
		Hash      string `json:"hash,omitempty"`      // "0x..."
		StateRoot string `json:"stateRoot,omitempty"` // "0x..."
		Timestamp string `json:"timestamp,omitempty"` // "0x..."
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return 0, nil, fmt.Errorf("unmarshaling balance response: %w", err)
	}

	// ✅ 步骤4：提取 balance 字段并转换为 uint64
	balanceHex := response.Balance
	if len(balanceHex) < 2 || balanceHex[:2] != "0x" {
		return 0, nil, fmt.Errorf("invalid balance format: %s", balanceHex)
	}

	balance, err := strconv.ParseUint(balanceHex[2:], 16, 64)
	if err != nil {
		return 0, nil, fmt.Errorf("parsing balance: %w", err)
	}

	// TODO: 查询代币余额（需要根据实际 API 调整）
	tokens := []flows.TokenBalance{}

	return balance, tokens, nil
}
