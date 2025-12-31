package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/weisyn/v1/client/pkg/transport/jsonrpc"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/address"
)

// MiningAdapter 挖矿服务适配器（通过 JSON-RPC 连接到节点）
type MiningAdapter struct {
	client         *jsonrpc.Client
	addressManager *address.AddressService
}

// NewMiningAdapter 创建挖矿服务适配器
func NewMiningAdapter(client *jsonrpc.Client, addrMgr *address.AddressService) *MiningAdapter {
	return &MiningAdapter{
		client:         client,
		addressManager: addrMgr,
	}
}

// MiningStatus 挖矿状态
type MiningStatus struct {
	IsRunning    bool   `json:"is_running"`
	MinerAddress string `json:"miner_address"` // Base58格式地址
}

// StartMining 启动挖矿
// minerAddress: Base58格式地址（如 CUQ3g6P5WmFN289pPn7AAhnQ3T2cZRv2BR）
func (m *MiningAdapter) StartMining(ctx context.Context, minerAddress string) error {
	// ✅ 验证地址格式（WES使用Base58格式，不兼容ETH的0x前缀格式）
	if m.addressManager == nil {
		return fmt.Errorf("address manager not available")
	}
	
	// 拒绝0x前缀的ETH地址格式
	if len(minerAddress) > 2 && (minerAddress[:2] == "0x" || minerAddress[:2] == "0X") {
		return fmt.Errorf("WES地址必须使用Base58格式，不支持0x前缀的ETH地址格式")
	}
	
	// 验证Base58格式地址
	validAddress, err := m.addressManager.StringToAddress(minerAddress)
	if err != nil {
		return fmt.Errorf("invalid address format: %w", err)
	}
	
	// ✅ 调用 wes_startMining（传递Base58格式地址）
	result, err := m.client.Call(ctx, "wes_startMining", validAddress)
	if err != nil {
		return fmt.Errorf("calling wes_startMining: %w", err)
	}

	// 解析响应
	var response struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		MinerAddress string `json:"miner_address"` // Base58格式地址
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return fmt.Errorf("unmarshaling response: %w", err)
	}

	if response.Status != "success" {
		return fmt.Errorf("mining start failed: %s", response.Message)
	}

	return nil
}

// StopMining 停止挖矿
func (m *MiningAdapter) StopMining(ctx context.Context) error {
	// 调用 wes_stopMining
	result, err := m.client.Call(ctx, "wes_stopMining", nil)
	if err != nil {
		return fmt.Errorf("calling wes_stopMining: %w", err)
	}

	// 解析响应
	var response struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return fmt.Errorf("unmarshaling response: %w", err)
	}

	if response.Status != "success" {
		return fmt.Errorf("mining stop failed: %s", response.Message)
	}

	return nil
}

// GetMiningStatus 获取挖矿状态
func (m *MiningAdapter) GetMiningStatus(ctx context.Context) (*MiningStatus, error) {
	// 调用 wes_getMiningStatus
	result, err := m.client.Call(ctx, "wes_getMiningStatus", nil)
	if err != nil {
		return nil, fmt.Errorf("calling wes_getMiningStatus: %w", err)
	}

	// 解析响应
	var response struct {
		IsRunning    bool   `json:"is_running"`
		MinerAddress string `json:"miner_address"` // Base58格式地址
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return nil, fmt.Errorf("unmarshaling response: %w", err)
	}

	// ✅ 直接返回Base58格式地址（不再需要转换）
	return &MiningStatus{
		IsRunning:    response.IsRunning,
		MinerAddress: response.MinerAddress,
	}, nil
}
