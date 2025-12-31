package transfer

import (
	"context"
	"fmt"

	"github.com/weisyn/v1/client/core/builder"
	"github.com/weisyn/v1/client/core/transport"
	"github.com/weisyn/v1/client/core/wallet"
)

// BatchTransferService 批量转账业务服务
type BatchTransferService struct {
	transport transport.Client
	signer    *wallet.Signer
	selector  builder.UTXOSelector
}

// NewBatchTransferService 创建批量转账业务服务
func NewBatchTransferService(
	client transport.Client,
	signer *wallet.Signer,
) *BatchTransferService {
	return &BatchTransferService{
		transport: client,
		signer:    signer,
		selector:  builder.NewGreedySelector(), // 批量转账使用贪心策略
	}
}

// BatchRecipient 批量转账收款人
type BatchRecipient struct {
	Address string // 收款地址
	Amount  string // 转账金额（WES单位）
}

// BatchTransferRequest 批量转账请求
type BatchTransferRequest struct {
	FromAddress string           // 发送方地址
	Recipients  []BatchRecipient // 收款人列表
	PrivateKey  []byte           // 发送方私钥
	Memo        string           // 备注（可选）
}

// BatchTransferResult 批量转账结果
type BatchTransferResult struct {
	TxID        string                    // 交易ID
	TxHash      string                    // 交易哈希
	Success     bool                      // 是否成功
	Message     string                    // 结果消息
	TotalAmount string                    // 总转账金额
	Fee         string                    // 实际手续费
	Change      string                    // 找零金额
	Recipients  int                       // 收款人数量
	FailedItems []BatchTransferFailedItem // 失败的转账项
	BlockHeight uint64                    // 区块高度（待确认时为0）
}

// BatchTransferFailedItem 失败的转账项
type BatchTransferFailedItem struct {
	Address string // 收款地址
	Amount  string // 金额
	Reason  string // 失败原因
}

// ExecuteBatchTransfer 执行批量转账
//
// 完整流程：
//  1. 验证所有收款人参数
//  2. 计算总金额
//  3. 余额检查
//  4. UTXO选择（贪心策略，最小化输入数量）
//  5. 构建Draft（1个或多个输入，N个接收输出+1个找零输出）
//  6. Seal、Sign、Broadcast
func (bs *BatchTransferService) ExecuteBatchTransfer(ctx context.Context, req *BatchTransferRequest) (*BatchTransferResult, error) {
	// 1. 参数验证
	if err := bs.validateBatchRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// 2. 验证并解析所有收款人金额
	totalAmount := builder.Zero()
	validRecipients := make([]BatchRecipient, 0, len(req.Recipients))
	failedItems := make([]BatchTransferFailedItem, 0)

	for _, recipient := range req.Recipients {
		// 解析金额
		amount, err := builder.NewAmountFromString(recipient.Amount)
		if err != nil {
			failedItems = append(failedItems, BatchTransferFailedItem{
				Address: recipient.Address,
				Amount:  recipient.Amount,
				Reason:  fmt.Sprintf("invalid amount: %v", err),
			})
			continue
		}

		// 验证金额为正
		if !amount.IsPositive() {
			failedItems = append(failedItems, BatchTransferFailedItem{
				Address: recipient.Address,
				Amount:  recipient.Amount,
				Reason:  "amount must be positive",
			})
			continue
		}

		// 累加总金额
		totalAmount = totalAmount.Add(amount)
		validRecipients = append(validRecipients, recipient)
	}

	// 如果没有有效的收款人，返回错误
	if len(validRecipients) == 0 {
		return nil, fmt.Errorf("no valid recipients")
	}

	// 3. 查询发送方的UTXOs
	utxos, err := bs.transport.GetUTXOs(ctx, req.FromAddress, nil)
	if err != nil {
		return nil, fmt.Errorf("get utxos: %w", err)
	}

	if len(utxos) == 0 {
		return nil, fmt.Errorf("no available UTXOs")
	}

	// 转换UTXO
	convertedUTXOs := bs.convertUTXOs(utxos)

	// 4. 估算费用（基于输入输出数量）
	estimatedFee := bs.estimateBatchFee(len(convertedUTXOs), len(validRecipients)+1)

	// 5. 计算需要的总金额（转账总额 + 费用）
	targetAmount := totalAmount.Add(estimatedFee)

	// 6. 选择UTXO
	selectedUTXOs, totalSelected, err := bs.selector.Select(convertedUTXOs, targetAmount)
	if err != nil {
		return nil, fmt.Errorf("select utxos: %w (need %s, available %s)",
			err,
			targetAmount.String(),
			builder.CalculateTotalAmount(convertedUTXOs).String(),
		)
	}

	// 7. 重新估算费用（基于实际选中的输入数量）
	actualFee := bs.estimateBatchFee(len(selectedUTXOs), len(validRecipients)+1)

	// 8. 计算找零
	change, err := totalSelected.Sub(totalAmount)
	if err != nil {
		return nil, fmt.Errorf("calculate change: %w", err)
	}

	change, err = change.Sub(actualFee)
	if err != nil {
		return nil, fmt.Errorf("insufficient for fee: %w", err)
	}

	// 9. 构建Draft交易
	draft, err := bs.buildBatchDraft(req, selectedUTXOs, validRecipients, change, actualFee)
	if err != nil {
		return nil, fmt.Errorf("build draft: %w", err)
	}

	// 10. Seal
	composed, err := draft.Seal()
	if err != nil {
		return nil, fmt.Errorf("seal transaction: %w", err)
	}

	// 11. 添加解锁证明
	proofs := bs.generateProofs(composed)
	proven, err := composed.WithProofs(proofs)
	if err != nil {
		return nil, fmt.Errorf("add proofs: %w", err)
	}

	// 12. 签名
	signed, err := bs.signTransaction(ctx, proven, req.FromAddress, req.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("sign transaction: %w", err)
	}

	// 13. 广播
	txResult, err := bs.transport.SendRawTransaction(ctx, signed.RawHex())
	if err != nil {
		return nil, fmt.Errorf("broadcast transaction: %w", err)
	}

	return &BatchTransferResult{
		TxID:        composed.TxID(),
		TxHash:      txResult.TxHash,
		Success:     true,
		Message:     fmt.Sprintf("批量转账交易已提交（%d个收款人）", len(validRecipients)),
		TotalAmount: totalAmount.String(),
		Fee:         actualFee.String(),
		Change:      change.String(),
		Recipients:  len(validRecipients),
		FailedItems: failedItems,
		BlockHeight: 0,
	}, nil
}

// buildBatchDraft 构建批量转账Draft
func (bs *BatchTransferService) buildBatchDraft(
	req *BatchTransferRequest,
	selectedUTXOs []builder.UTXO,
	recipients []BatchRecipient,
	change *builder.Amount,
	fee *builder.Amount,
) (*builder.DraftTx, error) {
	// 创建基础Builder
	txBuilder := builder.NewTxBuilder(bs.transport).(*builder.DefaultTxBuilder)
	draft := txBuilder.CreateDraft()

	// 添加输入
	for _, utxo := range selectedUTXOs {
		draft.AddInput(builder.Input{
			TxHash:      utxo.TxHash,
			OutputIndex: utxo.Vout,
			Amount:      utxo.Amount.StringUnits(),
			Address:     utxo.Address,
			LockScript:  string(utxo.ScriptPub),
		})
	}

	// 添加所有接收方输出
	for _, recipient := range recipients {
		amount, _ := builder.NewAmountFromString(recipient.Amount)
		draft.AddOutput(builder.Output{
			Address:    recipient.Address,
			Amount:     amount.StringUnits(),
			Type:       builder.OutputTypeTransfer,
			LockScript: bs.generateLockScript(recipient.Address),
		})
	}

	// 添加找零输出（如果有找零）
	if change.IsPositive() {
		draft.AddOutput(builder.Output{
			Address:    req.FromAddress,
			Amount:     change.StringUnits(),
			Type:       builder.OutputTypeTransfer,
			LockScript: bs.generateLockScript(req.FromAddress),
		})
	}

	// 设置参数
	if req.Memo != "" {
		draft.SetMemo(req.Memo)
	}

	draft.SetParams(builder.TxParams{
		Extra: map[string]interface{}{
			"batch_transfer": true,
			"recipients":     len(recipients),
			"estimated_fee":  fee.StringUnits(),
			"change":         change.StringUnits(),
		},
	})

	return draft, nil
}

// estimateBatchFee 估算批量转账费用
func (bs *BatchTransferService) estimateBatchFee(numInputs, numOutputs int) *builder.Amount {
	// 估算交易大小
	txSize := builder.EstimateTransactionSize(numInputs, numOutputs)

	// 基础费率：10 sat/byte
	feePerByte := int64(10)

	// 计算总费用
	totalFee := int64(txSize) * feePerByte

	return builder.NewAmountFromUnits(uint64(totalFee))
}

// convertUTXOs 转换transport.UTXO为builder.UTXO
func (bs *BatchTransferService) convertUTXOs(rawUTXOs []*transport.UTXO) []builder.UTXO {
	utxos := make([]builder.UTXO, 0, len(rawUTXOs))

	for _, raw := range rawUTXOs {
		amount, err := builder.NewAmountFromString(raw.Amount)
		if err != nil {
			continue
		}

		utxos = append(utxos, builder.UTXO{
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
func (bs *BatchTransferService) generateLockScript(address string) string {
	return fmt.Sprintf("OP_DUP OP_HASH160 %s OP_EQUALVERIFY OP_CHECKSIG", address)
}

// generateProofs 生成解锁证明
func (bs *BatchTransferService) generateProofs(composed *builder.ComposedTx) []builder.UnlockingProof {
	inputs := composed.Inputs()
	proofs := make([]builder.UnlockingProof, len(inputs))

	for i := range inputs {
		proofs[i] = builder.UnlockingProof{
			InputIndex: i,
			Type:       "signature",
			Data:       []byte{},
		}
	}

	return proofs
}

// signTransaction 签名交易
func (bs *BatchTransferService) signTransaction(
	ctx context.Context,
	proven *builder.ProvenTx,
	fromAddress string,
	privateKey []byte,
) (*builder.SignedTx, error) {
	signers := make(map[string]string)
	txID := proven.TxID()

	signature, err := (*bs.signer).SignHash([]byte(txID), fromAddress)
	if err != nil {
		return nil, fmt.Errorf("sign hash with address %s: %w", fromAddress, err)
	}

	signers[fromAddress] = string(signature)

	signed, err := proven.Sign(bs.transport, signers)
	if err != nil {
		return nil, fmt.Errorf("create signed tx: %w", err)
	}

	return signed, nil
}

// validateBatchRequest 验证批量转账请求
func (bs *BatchTransferService) validateBatchRequest(req *BatchTransferRequest) error {
	if req == nil {
		return fmt.Errorf("request is nil")
	}

	if req.FromAddress == "" {
		return fmt.Errorf("from address is empty")
	}

	if len(req.Recipients) == 0 {
		return fmt.Errorf("recipients list is empty")
	}

	if len(req.PrivateKey) == 0 {
		return fmt.Errorf("private key is empty")
	}

	return nil
}
