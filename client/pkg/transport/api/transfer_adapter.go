package api

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/weisyn/v1/client/core/transfer"
	"github.com/weisyn/v1/client/core/transport"
	"github.com/weisyn/v1/client/core/wallet"
	"github.com/weisyn/v1/client/pkg/ux/flows"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/address"
)

// TransferAdapter 转账服务适配器（通过 JSON-RPC 连接到节点）
type TransferAdapter struct {
	transportClient transport.Client
	transferSvc     *transfer.TransferService
	addressManager  *address.AddressService
}

// NewTransferAdapter 创建转账服务适配器
func NewTransferAdapter(transportClient transport.Client, addrMgr *address.AddressService) *TransferAdapter {
	// 创建空的签名器指针（实际签名在节点侧完成）
	var signer *wallet.Signer = nil

	// 创建TransferService（传入addressManager用于地址转换）
	transferSvc := transfer.NewTransferService(transportClient, signer, addrMgr)

	return &TransferAdapter{
		transportClient: transportClient,
		transferSvc:     transferSvc,
		addressManager:  addrMgr,
	}
}

// Transfer 执行单笔转账
// 🎯 调用节点的 wes_sendTransaction 接口（节点内部完成三步流程）
func (t *TransferAdapter) Transfer(ctx context.Context, req *flows.TransferRequest) (string, error) {
	fmt.Printf("\n========== 转账流程开始 ==========\n")
	fmt.Printf("📤 发送方地址(Base58): %s\n", req.FromAddress)
	fmt.Printf("📥 接收方地址(Base58): %s\n", req.ToAddress)
	fmt.Printf("💰 转账金额: %d\n", req.Amount)

	// 转换地址为hex格式
	fromAddressHex, err := t.convertAddressToHex(req.FromAddress)
	if err != nil {
		return "", fmt.Errorf("转换发送地址失败: %w", err)
	}
	toAddressHex, err := t.convertAddressToHex(req.ToAddress)
	if err != nil {
		return "", fmt.Errorf("转换接收地址失败: %w", err)
	}

	fmt.Printf("📍 发送方地址(Hex): %s\n", fromAddressHex)
	fmt.Printf("📍 接收方地址(Hex): %s\n", toAddressHex)

	// 调用节点的 wes_sendTransaction（节点内部完成：构建→签名→提交）
	fmt.Printf("\n[调用] wes_sendTransaction\n")
	// ⚠️ 注意：节点要求 Base58 地址，此处保持原始地址格式
	result, err := t.transportClient.SendTransaction(ctx, req.FromAddress, req.ToAddress, req.Amount, req.PrivateKey)
	if err != nil {
		fmt.Printf("❌ 转账失败: %v\n", err)
		return "", fmt.Errorf("转账失败: %w", err)
	}

	if !result.Accepted {
		fmt.Printf("❌ 交易被拒绝: %s\n", result.Reason)
		return "", fmt.Errorf("交易被拒绝: %s", result.Reason)
	}

	fmt.Printf("✅ 转账成功，TxHash: %s\n", result.TxHash)
	fmt.Printf("\n========== 转账流程完成 ==========\n\n")
	return result.TxHash, nil
}

// convertAddressToHex 辅助方法：将 Base58 地址转换为十六进制格式
func (t *TransferAdapter) convertAddressToHex(base58Addr string) (string, error) {
	if t.addressManager == nil {
		return "", fmt.Errorf("addressManager not available")
	}
	hexAddr, err := t.addressManager.AddressToHexString(base58Addr)
	if err != nil {
		return "", fmt.Errorf("地址转换失败: %w", err)
	}
	return hexAddr, nil
}

// BatchTransfer 批量转账（暂不支持，返回错误）
func (t *TransferAdapter) BatchTransfer(ctx context.Context, req *flows.BatchTransferRequest) (string, error) {
	return "", fmt.Errorf("BatchTransfer not yet implemented - use single Transfer for now")
}

// TimeLockTransfer 时间锁定转账（暂不支持，返回错误）
func (t *TransferAdapter) TimeLockTransfer(ctx context.Context, req *flows.TimeLockTransferRequest) (string, error) {
	return "", fmt.Errorf("TimeLockTransfer not yet implemented - use single Transfer for now")
}

// EstimateFee 估算手续费
func (t *TransferAdapter) EstimateFee(ctx context.Context, from, to string, amount uint64) (uint64, error) {
	// 组装最小交易对象（节点 wes_estimateFee 主要读取 amount 字段）
	txObj := map[string]interface{}{
		"amount": fmt.Sprintf("%d", amount),
	}

	// 调用节点的 wes_estimateFee 接口
	result, err := t.transportClient.CallRaw(ctx, "wes_estimateFee", []interface{}{txObj})
	if err != nil {
		return 0, fmt.Errorf("调用 wes_estimateFee 失败: %w", err)
	}

	// 解析返回结果
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("wes_estimateFee 返回格式错误: %T", result)
	}

	// 提取 estimated_fee 字段
	estimatedFeeVal, ok := resultMap["estimated_fee"]
	if !ok {
		return 0, fmt.Errorf("wes_estimateFee 返回缺少 estimated_fee 字段")
	}

	// 转换为 uint64
	var estimatedFee uint64
	switch v := estimatedFeeVal.(type) {
	case float64:
		estimatedFee = uint64(v)
	case uint64:
		estimatedFee = v
	case string:
		var parseErr error
		estimatedFee, parseErr = parseUint64FromString(v)
		if parseErr != nil {
			return 0, fmt.Errorf("解析 estimated_fee 失败: %w", parseErr)
		}
	default:
		return 0, fmt.Errorf("estimated_fee 类型不支持: %T", v)
	}

	return estimatedFee, nil
}

// parseUint64FromString 从字符串解析 uint64（支持十进制和十六进制）
func parseUint64FromString(s string) (uint64, error) {
	// 移除 0x 前缀（如果有）
	s = strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	
	// 尝试解析为十进制
	val, err := strconv.ParseUint(s, 10, 64)
	if err == nil {
		return val, nil
	}
	
	// 尝试解析为十六进制
	val, err = strconv.ParseUint(s, 16, 64)
	if err == nil {
		return val, nil
	}
	
	return 0, fmt.Errorf("无法解析为 uint64: %s", s)
}

// GetBalance 查询余额
func (t *TransferAdapter) GetBalance(ctx context.Context, address string) (uint64, error) {
	// 直接传递 Base58 地址给服务端（服务端要求 Base58 格式，拒绝 0x 前缀）
	// 调用transport client查询余额
	balance, err := t.transportClient.GetBalance(ctx, address, nil)
	if err != nil {
		return 0, fmt.Errorf("查询余额失败: %w", err)
	}

	// 解析余额字符串
	var balanceUint64 uint64
	fmt.Sscanf(balance.Balance, "%d", &balanceUint64)
	return balanceUint64, nil
}
