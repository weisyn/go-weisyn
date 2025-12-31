package builder

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/weisyn/v1/client/core/transport"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/address"
)

// TransferBuilder 转账Draft构建器
// 负责构建简单的1对1转账交易
type TransferBuilder struct {
	client         transport.Client
	addressManager *address.AddressService
}

// NewTransferBuilder 创建转账构建器
func NewTransferBuilder(client transport.Client, addrMgr *address.AddressService) *TransferBuilder {
	return &TransferBuilder{
		client:         client,
		addressManager: addrMgr,
	}
}

// TransferRequest 转账请求参数
type TransferRequest struct {
	From   string  // 发送方地址
	To     string  // 接收方地址
	Amount *Amount // 转账金额
	Memo   string  // 备注（可选）
}

// Build 构建转账交易Draft
//
// 流程：
//  1. 查询发送方的所有UTXO
//  2. 估算交易费用
//  3. 选择UTXO（满足金额+费用）
//  4. 构建输入
//  5. 构建输出（接收方+找零）
//  6. 返回Draft
func (tb *TransferBuilder) Build(ctx context.Context, req *TransferRequest) (*DraftTx, error) {
	// 参数验证
	if err := tb.validateRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// 直接传递 Base58 地址给服务端（服务端要求 Base58 格式）
	// 1. 查询UTXO
	rawUTXOs, err := tb.client.GetUTXOs(ctx, req.From, nil)
	if err != nil {
		return nil, fmt.Errorf("get utxos: %w", err)
	}

	if len(rawUTXOs) == 0 {
		return nil, fmt.Errorf("no available UTXOs for address %s", req.From)
	}

	// 转换为内部UTXO类型
	utxos := tb.convertUTXOs(rawUTXOs)

	// 2. 估算费用（使用简化估算）
	estimatedFee := tb.estimateFee(ctx, len(utxos), 2) // 假设2个输出（接收+找零）

	// 3. 计算需要的总金额（转账金额 + 费用）
	targetAmount := req.Amount.Add(estimatedFee)

	// 4. 选择UTXO（使用FirstFit策略）
	selector := NewFirstFitSelector()
	selectedUTXOs, totalSelected, err := selector.Select(utxos, targetAmount)
	if err != nil {
		return nil, fmt.Errorf("select utxos: %w (need %s, available %s)",
			err,
			targetAmount.String(),
			CalculateTotalAmount(utxos).String(),
		)
	}

	// 5. 计算找零
	change, err := totalSelected.Sub(req.Amount)
	if err != nil {
		return nil, fmt.Errorf("calculate change: %w", err)
	}

	change, err = change.Sub(estimatedFee)
	if err != nil {
		return nil, fmt.Errorf("insufficient for fee: %w", err)
	}

	// 6. 创建Draft
	draft := tb.buildDraft(req, selectedUTXOs, change, estimatedFee)

	return draft, nil
}

// buildDraft 构建Draft交易
func (tb *TransferBuilder) buildDraft(
	req *TransferRequest,
	selectedUTXOs []UTXO,
	change *Amount,
	fee *Amount,
) *DraftTx {
	// 创建基础Builder
	builder := NewTxBuilder(tb.client).(*DefaultTxBuilder)
	draft := builder.CreateDraft()

	// 添加输入
	for _, utxo := range selectedUTXOs {
		draft.AddInput(Input{
			TxHash:      utxo.TxHash,
			OutputIndex: utxo.Vout,
			Amount:      utxo.Amount.StringUnits(),
			Address:     utxo.Address,
			LockScript:  string(utxo.ScriptPub),
		})
	}

	// 添加接收方输出
	draft.AddOutput(Output{
		Address:    req.To,
		Amount:     req.Amount.StringUnits(),
		Type:       OutputTypeTransfer,
		LockScript: tb.generateLockScript(req.To),
	})

	// 添加找零输出（如果有找零）
	if change.IsPositive() {
		draft.AddOutput(Output{
			Address:    req.From,
			Amount:     change.StringUnits(),
			Type:       OutputTypeTransfer,
			LockScript: tb.generateLockScript(req.From),
		})
	}

	// 设置参数
	if req.Memo != "" {
		draft.SetMemo(req.Memo)
	}

	// 记录费用信息到Extra
	draft.params.Extra = map[string]interface{}{
		"estimated_fee": fee.StringUnits(),
		"change":        change.StringUnits(),
	}

	return draft
}

// estimateFee 估算交易费用
//
// 简化估算：
//   - 每个输入 ~148字节
//   - 每个输出 ~34字节
//   - 基础费率 10 sat/byte
func (tb *TransferBuilder) estimateFee(ctx context.Context, numInputs, numOutputs int) *Amount {
	// 估算交易大小（字节）
	txSize := EstimateTransactionSize(numInputs, numOutputs)

	// 基础费率：10 sat/byte
	feePerByte := int64(10)

	// 计算总费用
	totalFee := int64(txSize) * feePerByte

	return NewAmountFromUnits(uint64(totalFee))
}

// convertUTXOs 转换transport.UTXO为builder.UTXO
func (tb *TransferBuilder) convertUTXOs(rawUTXOs []*transport.UTXO) []UTXO {
	utxos := make([]UTXO, 0, len(rawUTXOs))

	for _, raw := range rawUTXOs {
		// 解析金额
		amount, err := NewAmountFromString(raw.Amount)
		if err != nil {
			// 跳过无效的UTXO
			continue
		}

		utxos = append(utxos, UTXO{
			TxHash:    raw.TxHash,
			Vout:      raw.OutputIndex,
			Amount:    amount,
			Address:   raw.Address,
			ScriptPub: []byte(raw.LockScript),
		})
	}

	return utxos
}

// generateLockScript 生成锁定脚本
//
// 简化实现：P2PKH格式
// 实际应该根据地址类型生成对应的锁定脚本
func (tb *TransferBuilder) generateLockScript(address string) string {
	return fmt.Sprintf("OP_DUP OP_HASH160 %s OP_EQUALVERIFY OP_CHECKSIG", address)
}

// validateRequest 验证转账请求
func (tb *TransferBuilder) validateRequest(req *TransferRequest) error {
	if req == nil {
		return fmt.Errorf("request is nil")
	}

	if req.From == "" {
		return fmt.Errorf("from address is empty")
	}

	if req.To == "" {
		return fmt.Errorf("to address is empty")
	}

	if req.Amount == nil || !req.Amount.IsPositive() {
		return fmt.Errorf("invalid amount")
	}

	return nil
}

// EstimateFeeForTransfer 为转账估算费用（公开方法，用于UI显示）
func (tb *TransferBuilder) EstimateFeeForTransfer(ctx context.Context, from string, amount *Amount) (*Amount, error) {
	// 直接传递 Base58 地址给服务端（服务端要求 Base58 格式）
	// 查询UTXO数量
	rawUTXOs, err := tb.client.GetUTXOs(ctx, from, nil)
	if err != nil {
		return nil, fmt.Errorf("get utxos: %w", err)
	}

	if len(rawUTXOs) == 0 {
		return nil, fmt.Errorf("no available UTXOs")
	}

	utxos := tb.convertUTXOs(rawUTXOs)

	// 使用选择器估算需要多少个输入
	estimatedFee := tb.estimateFee(ctx, len(utxos), 2)
	targetAmount := amount.Add(estimatedFee)

	selector := NewFirstFitSelector()
	selectedUTXOs, _, err := selector.Select(utxos, targetAmount)
	if err != nil {
		return nil, fmt.Errorf("cannot select enough UTXOs: %w", err)
	}

	// 重新计算准确的费用（基于实际选中的输入数量）
	actualFee := tb.estimateFee(ctx, len(selectedUTXOs), 2)

	return actualFee, nil
}

// convertAddressToHex 将Base58地址转换为十六进制格式
func (tb *TransferBuilder) convertAddressToHex(addr string) (string, error) {
	if tb.addressManager == nil {
		// 降级：假设已经是十六进制格式
		return addr, nil
	}

	// 使用 AddressManager 将 Base58 地址转为字节数组
	addressBytes, err := tb.addressManager.AddressToBytes(addr)
	if err != nil {
		return "", fmt.Errorf("convert address to bytes: %w", err)
	}

	// 转为十六进制并添加 0x 前缀
	addressHex := "0x" + hex.EncodeToString(addressBytes)
	return addressHex, nil
}
