package transfer

import (
	"context"
	"fmt"
	"time"

	"github.com/weisyn/v1/client/core/builder"
	"github.com/weisyn/v1/client/core/transport"
	"github.com/weisyn/v1/client/core/wallet"
)

// TimeLockTransferService 时间锁转账业务服务
type TimeLockTransferService struct {
	transport transport.Client
	signer    *wallet.Signer
	selector  builder.UTXOSelector
}

// NewTimeLockTransferService 创建时间锁转账业务服务
func NewTimeLockTransferService(
	client transport.Client,
	signer *wallet.Signer,
) *TimeLockTransferService {
	return &TimeLockTransferService{
		transport: client,
		signer:    signer,
		selector:  builder.NewFirstFitSelector(),
	}
}

// TimeLockTransferRequest 时间锁转账请求
type TimeLockTransferRequest struct {
	FromAddress string    // 发送方地址
	ToAddress   string    // 接收方地址
	Amount      string    // 转账金额（WES单位）
	PrivateKey  []byte    // 发送方私钥
	UnlockTime  time.Time // 解锁时间（接收方可以花费的时间）
	Memo        string    // 备注（可选）
}

// TimeLockTransferResult 时间锁转账结果
type TimeLockTransferResult struct {
	TxID        string    // 交易ID
	TxHash      string    // 交易哈希
	Success     bool      // 是否成功
	Message     string    // 结果消息
	Amount      string    // 转账金额
	Fee         string    // 实际手续费
	Change      string    // 找零金额
	UnlockTime  time.Time // 解锁时间
	BlockHeight uint64    // 区块高度（待确认时为0）
}

// ExecuteTimeLockTransfer 执行时间锁转账
//
// 完整流程：
//  1. 验证参数（包括时间锁时间）
//  2. 余额检查
//  3. UTXO选择
//  4. 构建Draft（带时间锁的输出）
//  5. Seal、Sign、Broadcast
//
// 时间锁机制：
//   - 接收方只能在UnlockTime之后花费这笔UTXO
//   - 锁定脚本包含时间锁条件：OP_CHECKLOCKTIMEVERIFY
func (ts *TimeLockTransferService) ExecuteTimeLockTransfer(ctx context.Context, req *TimeLockTransferRequest) (*TimeLockTransferResult, error) {
	// 1. 参数验证
	if err := ts.validateRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// 2. 验证解锁时间（必须在未来）
	if !req.UnlockTime.After(time.Now()) {
		return nil, fmt.Errorf("unlock time must be in the future")
	}

	// 3. 解析金额
	amount, err := builder.NewAmountFromString(req.Amount)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %w", err)
	}

	// 4. 查询发送方的UTXOs
	utxos, err := ts.transport.GetUTXOs(ctx, req.FromAddress, nil)
	if err != nil {
		return nil, fmt.Errorf("get utxos: %w", err)
	}

	if len(utxos) == 0 {
		return nil, fmt.Errorf("no available UTXOs")
	}

	// 转换UTXO
	convertedUTXOs := ts.convertUTXOs(utxos)

	// 5. 估算费用
	estimatedFee := ts.estimateFee(len(convertedUTXOs), 2) // 1个时间锁输出 + 1个找零输出

	// 6. 计算需要的总金额（转账金额 + 费用）
	targetAmount := amount.Add(estimatedFee)

	// 7. 选择UTXO
	selectedUTXOs, totalSelected, err := ts.selector.Select(convertedUTXOs, targetAmount)
	if err != nil {
		return nil, fmt.Errorf("select utxos: %w (need %s, available %s)",
			err,
			targetAmount.String(),
			builder.CalculateTotalAmount(convertedUTXOs).String(),
		)
	}

	// 8. 重新估算费用（基于实际选中的输入数量）
	actualFee := ts.estimateFee(len(selectedUTXOs), 2)

	// 9. 计算找零
	change, err := totalSelected.Sub(amount)
	if err != nil {
		return nil, fmt.Errorf("calculate change: %w", err)
	}

	change, err = change.Sub(actualFee)
	if err != nil {
		return nil, fmt.Errorf("insufficient for fee: %w", err)
	}

	// 10. 构建Draft交易
	draft, err := ts.buildTimeLockDraft(req, selectedUTXOs, amount, change, actualFee)
	if err != nil {
		return nil, fmt.Errorf("build draft: %w", err)
	}

	// 11. Seal
	composed, err := draft.Seal()
	if err != nil {
		return nil, fmt.Errorf("seal transaction: %w", err)
	}

	// 12. 添加解锁证明
	proofs := ts.generateProofs(composed)
	proven, err := composed.WithProofs(proofs)
	if err != nil {
		return nil, fmt.Errorf("add proofs: %w", err)
	}

	// 13. 签名
	signed, err := ts.signTransaction(ctx, proven, req.FromAddress, req.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("sign transaction: %w", err)
	}

	// 14. 广播
	txResult, err := ts.transport.SendRawTransaction(ctx, signed.RawHex())
	if err != nil {
		return nil, fmt.Errorf("broadcast transaction: %w", err)
	}

	return &TimeLockTransferResult{
		TxID:        composed.TxID(),
		TxHash:      txResult.TxHash,
		Success:     true,
		Message:     fmt.Sprintf("时间锁转账交易已提交，解锁时间：%s", req.UnlockTime.Format(time.RFC3339)),
		Amount:      amount.String(),
		Fee:         actualFee.String(),
		Change:      change.String(),
		UnlockTime:  req.UnlockTime,
		BlockHeight: 0,
	}, nil
}

// buildTimeLockDraft 构建时间锁转账Draft
func (ts *TimeLockTransferService) buildTimeLockDraft(
	req *TimeLockTransferRequest,
	selectedUTXOs []builder.UTXO,
	amount *builder.Amount,
	change *builder.Amount,
	fee *builder.Amount,
) (*builder.DraftTx, error) {
	// 创建基础Builder
	txBuilder := builder.NewTxBuilder(ts.transport).(*builder.DefaultTxBuilder)
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

	// 添加时间锁输出（接收方）
	unlockTimestamp := uint64(req.UnlockTime.Unix())
	draft.AddOutput(builder.Output{
		Address:    req.ToAddress,
		Amount:     amount.StringUnits(),
		Type:       builder.OutputTypeTransfer,
		LockScript: ts.generateTimeLockScript(req.ToAddress, unlockTimestamp),
		Data: map[string]interface{}{
			"timelock":    true,
			"unlock_time": unlockTimestamp,
			"unlock_date": req.UnlockTime.Format(time.RFC3339),
		},
	})

	// 添加找零输出（如果有找零）
	if change.IsPositive() {
		draft.AddOutput(builder.Output{
			Address:    req.FromAddress,
			Amount:     change.StringUnits(),
			Type:       builder.OutputTypeTransfer,
			LockScript: ts.generateLockScript(req.FromAddress),
		})
	}

	// 设置交易参数（包括时间锁）
	draft.SetParams(builder.TxParams{
		LockTime: unlockTimestamp,
		Memo:     req.Memo,
		Extra: map[string]interface{}{
			"timelock_transfer": true,
			"unlock_time":       unlockTimestamp,
			"unlock_date":       req.UnlockTime.Format(time.RFC3339),
			"estimated_fee":     fee.StringUnits(),
			"change":            change.StringUnits(),
		},
	})

	return draft, nil
}

// generateTimeLockScript 生成时间锁脚本
//
// 格式：<unlock_time> OP_CHECKLOCKTIMEVERIFY OP_DROP OP_DUP OP_HASH160 <address_hash> OP_EQUALVERIFY OP_CHECKSIG
//
// 这样的脚本确保：
//  1. 当前区块时间必须 >= unlock_time（通过OP_CHECKLOCKTIMEVERIFY）
//  2. 必须提供正确的签名（通过OP_CHECKSIG）
func (ts *TimeLockTransferService) generateTimeLockScript(address string, unlockTime uint64) string {
	return fmt.Sprintf("%d OP_CHECKLOCKTIMEVERIFY OP_DROP OP_DUP OP_HASH160 %s OP_EQUALVERIFY OP_CHECKSIG",
		unlockTime,
		address,
	)
}

// generateLockScript 生成普通锁定脚本
func (ts *TimeLockTransferService) generateLockScript(address string) string {
	return fmt.Sprintf("OP_DUP OP_HASH160 %s OP_EQUALVERIFY OP_CHECKSIG", address)
}

// estimateFee 估算交易费用
func (ts *TimeLockTransferService) estimateFee(numInputs, numOutputs int) *builder.Amount {
	// 时间锁交易的脚本更长，增加额外开销
	txSize := builder.EstimateTransactionSize(numInputs, numOutputs)
	timeLockOverhead := 20 // 时间锁脚本额外字节数

	// 基础费率：10 sat/byte
	feePerByte := int64(10)

	// 计算总费用
	totalFee := int64(txSize+timeLockOverhead) * feePerByte

	return builder.NewAmountFromUnits(uint64(totalFee))
}

// convertUTXOs 转换transport.UTXO为builder.UTXO
func (ts *TimeLockTransferService) convertUTXOs(rawUTXOs []*transport.UTXO) []builder.UTXO {
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

// generateProofs 生成解锁证明
func (ts *TimeLockTransferService) generateProofs(composed *builder.ComposedTx) []builder.UnlockingProof {
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
func (ts *TimeLockTransferService) signTransaction(
	ctx context.Context,
	proven *builder.ProvenTx,
	fromAddress string,
	privateKey []byte,
) (*builder.SignedTx, error) {
	signers := make(map[string]string)
	txID := proven.TxID()

	signature, err := (*ts.signer).SignHash([]byte(txID), fromAddress)
	if err != nil {
		return nil, fmt.Errorf("sign hash with address %s: %w", fromAddress, err)
	}

	signers[fromAddress] = string(signature)

	signed, err := proven.Sign(ts.transport, signers)
	if err != nil {
		return nil, fmt.Errorf("create signed tx: %w", err)
	}

	return signed, nil
}

// validateRequest 验证时间锁转账请求
func (ts *TimeLockTransferService) validateRequest(req *TimeLockTransferRequest) error {
	if req == nil {
		return fmt.Errorf("request is nil")
	}

	if req.FromAddress == "" {
		return fmt.Errorf("from address is empty")
	}

	if req.ToAddress == "" {
		return fmt.Errorf("to address is empty")
	}

	if req.Amount == "" {
		return fmt.Errorf("amount is empty")
	}

	if len(req.PrivateKey) == 0 {
		return fmt.Errorf("private key is empty")
	}

	if req.UnlockTime.IsZero() {
		return fmt.Errorf("unlock time is zero")
	}

	return nil
}
