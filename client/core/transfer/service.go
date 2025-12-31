package transfer

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/weisyn/v1/client/core/builder"
	"github.com/weisyn/v1/client/core/transport"
	"github.com/weisyn/v1/client/core/wallet"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/address"
)

// TransferService 转账业务服务
// 等价于旧TX的AssetService，提供完整的转账业务逻辑
type TransferService struct {
	builder        *builder.TransferBuilder
	transport      transport.Client
	signer         *wallet.Signer
	addressManager *address.AddressService
}

// NewTransferService 创建转账业务服务
func NewTransferService(
	client transport.Client,
	signer *wallet.Signer,
	addressManager *address.AddressService,
) *TransferService {
	return &TransferService{
		builder:        builder.NewTransferBuilder(client, addressManager),
		transport:      client,
		signer:         signer,
		addressManager: addressManager,
	}
}

// TransferRequest 转账请求
type TransferRequest struct {
	FromAddress string // 发送方地址
	ToAddress   string // 接收方地址
	Amount      string // 转账金额（WES单位）
	PrivateKey  []byte // 发送方私钥
	Memo        string // 备注（可选）
}

// TransferResult 转账结果
type TransferResult struct {
	TxID        string // 交易ID
	TxHash      string // 交易哈希
	Success     bool   // 是否成功
	Message     string // 结果消息
	Fee         string // 实际手续费
	Change      string // 找零金额
	BlockHeight uint64 // 区块高度（待确认时为0）
}

// ExecuteTransfer 执行单笔转账
//
// 完整流程：
//  1. 余额检查 - 查询UTXO并验证余额是否充足
//  2. UTXO选择 - 选择足够支付金额+费用的UTXO
//  3. 构建Draft - 创建转账交易草稿
//  4. Seal - 密封交易，计算TxID
//  5. Sign - 签名交易
//  6. Broadcast - 广播到网络
//
// 这是旧TX的AssetService.TransferAsset()的等价实现
func (s *TransferService) ExecuteTransfer(ctx context.Context, req *TransferRequest) (*TransferResult, error) {
	fmt.Printf("\n========== 转账流程开始 ==========\n")

	// 1. 参数验证
	fmt.Printf("[步骤0] 参数验证\n")
	if err := s.validateTransferRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// 2. 解析金额
	fmt.Printf("[步骤0] 解析金额: %s\n", req.Amount)
	amount, err := builder.NewAmountFromString(req.Amount)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %w", err)
	}

	// 3. 余额检查
	fmt.Printf("[步骤0] 余额检查\n")
	if err := s.checkBalance(ctx, req.FromAddress, amount); err != nil {
		return nil, fmt.Errorf("balance check failed: %w", err)
	}

	// ========== 步骤1：构建交易 ==========
	fmt.Printf("\n[步骤1] 开始构建交易\n")
	fmt.Printf("  - From: %s\n", req.FromAddress)
	fmt.Printf("  - To: %s\n", req.ToAddress)
	fmt.Printf("  - Amount: %s\n", amount.String())

	draft, err := s.builder.Build(ctx, &builder.TransferRequest{
		From:   req.FromAddress,
		To:     req.ToAddress,
		Amount: amount,
		Memo:   req.Memo,
	})
	if err != nil {
		return nil, fmt.Errorf("build draft: %w", err)
	}
	fmt.Printf("[步骤1] ✅ Draft构建成功\n")

	// 5. Seal - 密封交易，计算TxID
	fmt.Printf("[步骤1] 密封交易，计算TxID\n")
	composed, err := draft.Seal()
	if err != nil {
		return nil, fmt.Errorf("seal transaction: %w", err)
	}
	fmt.Printf("[步骤1] ✅ 交易已密封，TxID: %s\n", composed.TxID())

	// 6. 添加解锁证明（占位，实际需要根据输入生成证明）
	proofs := s.generateProofs(composed)
	proven, err := composed.WithProofs(proofs)
	if err != nil {
		return nil, fmt.Errorf("add proofs: %w", err)
	}
	fmt.Printf("[步骤1] ✅ 添加解锁证明\n")

	// ========== 步骤2：签名交易 ==========
	fmt.Printf("\n[步骤2] 开始签名交易\n")
	signed, err := s.signTransaction(ctx, proven, req.FromAddress, req.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("sign transaction: %w", err)
	}
	fmt.Printf("[步骤2] ✅ 交易签名完成\n")

	// ========== 步骤3：提交交易 ==========
	fmt.Printf("\n[步骤3] 开始提交交易到节点\n")
	rawHex := signed.RawHex()
	fmt.Printf("[步骤3] Transaction raw hex (前100字符): %s...\n", rawHex[:min(100, len(rawHex))])
	fmt.Printf("[步骤3] Transaction raw hex 总长度: %d\n", len(rawHex))

	txResult, err := s.transport.SendRawTransaction(ctx, rawHex)
	if err != nil {
		fmt.Printf("[步骤3] ❌ 提交失败: %v\n", err)
		return nil, fmt.Errorf("broadcast transaction: %w", err)
	}
	fmt.Printf("[步骤3] ✅ 交易已提交到网络，TxHash: %s\n", txResult.TxHash)
	fmt.Printf("\n========== 转账流程完成 ==========\n\n")

	// 9. 提取费用和找零信息
	fee, change := s.extractFeeAndChange(draft)

	return &TransferResult{
		TxID:        composed.TxID(),
		TxHash:      txResult.TxHash,
		Success:     true,
		Message:     "转账交易已提交",
		Fee:         fee,
		Change:      change,
		BlockHeight: 0, // 待确认
	}, nil
}

// checkBalance 检查余额是否充足
func (s *TransferService) checkBalance(ctx context.Context, address string, amount *builder.Amount) error {
	// 直接传递 Base58 地址给服务端（服务端要求 Base58 格式）
	// 查询UTXOs
	utxos, err := s.transport.GetUTXOs(ctx, address, nil)
	if err != nil {
		return fmt.Errorf("get utxos: %w", err)
	}

	if len(utxos) == 0 {
		return fmt.Errorf("no available UTXOs (balance is 0)")
	}

	// 计算总余额
	totalBalance := builder.Zero()
	for _, utxo := range utxos {
		utxoAmount, err := builder.NewAmountFromString(utxo.Amount)
		if err != nil {
			continue // 跳过无效UTXO
		}
		totalBalance = totalBalance.Add(utxoAmount)
	}

	// 估算费用
	estimatedFee, err := s.builder.EstimateFeeForTransfer(ctx, address, amount)
	if err != nil {
		estimatedFee = builder.NewAmountFromUnits(10000) // 降级：使用固定费用
	}

	// 检查余额是否充足（金额 + 费用）
	required := amount.Add(estimatedFee)
	if totalBalance.LessThan(required) {
		return fmt.Errorf("insufficient balance: have %s, need %s (amount: %s, fee: %s)",
			totalBalance.String(),
			required.String(),
			amount.String(),
			estimatedFee.String(),
		)
	}

	return nil
}

// generateProofs 生成解锁证明
// 简化实现：为每个输入生成占位证明
func (s *TransferService) generateProofs(composed *builder.ComposedTx) []builder.UnlockingProof {
	inputs := composed.Inputs()
	proofs := make([]builder.UnlockingProof, len(inputs))

	for i := range inputs {
		proofs[i] = builder.UnlockingProof{
			InputIndex: i,
			Type:       "signature",
			Data:       []byte{}, // 实际签名在Sign步骤填充
		}
	}

	return proofs
}

// signTransaction 签名交易
func (s *TransferService) signTransaction(
	ctx context.Context,
	proven *builder.ProvenTx,
	fromAddress string,
	privateKey []byte,
) (*builder.SignedTx, error) {
	// 构建签名者映射
	signers := make(map[string]string)

	// 获取交易TxID作为待签名数据
	txID := proven.TxID()

	// 🔥 直接使用传入的私钥签名（不再依赖signer内部的keystore）
	// 这种方式类似于旧版本的实现，直接将私钥用于签名
	var signature []byte
	var err error

	if privateKey != nil && len(privateKey) > 0 {
		// 使用提供的私钥签名
		// txID是十六进制字符串(0x...)，需要解码为字节
		var txHash []byte
		if len(txID) > 2 && txID[:2] == "0x" {
			var err error
			txHash, err = hex.DecodeString(txID[2:])
			if err != nil {
				return nil, fmt.Errorf("decode txID: %w", err)
			}
		} else {
			txHash = []byte(txID)
		}

		signature, err = signWithPrivateKey(txHash, privateKey)
		if err != nil {
			return nil, fmt.Errorf("sign with private key: %w", err)
		}
	} else {
		// 降级：尝试使用signer内部的keystore
		var txHash []byte
		if len(txID) > 2 && txID[:2] == "0x" {
			var err error
			txHash, err = hex.DecodeString(txID[2:])
			if err != nil {
				return nil, fmt.Errorf("decode txID: %w", err)
			}
		} else {
			txHash = []byte(txID)
		}

		signature, err = (*s.signer).SignHash(txHash, fromAddress)
		if err != nil {
			return nil, fmt.Errorf("sign hash with address %s: %w", fromAddress, err)
		}
	}

	// 添加签名者
	// 这里简化处理，实际应该为每个输入生成对应的签名
	signers[fromAddress] = string(signature)

	// 调用ProvenTx.Sign
	signed, err := proven.Sign(s.transport, signers)
	if err != nil {
		return nil, fmt.Errorf("create signed tx: %w", err)
	}

	return signed, nil
}

// extractFeeAndChange 从Draft中提取费用和找零信息
func (s *TransferService) extractFeeAndChange(draft *builder.DraftTx) (fee, change string) {
	// 从Extra参数中提取（在TransferBuilder.Build中设置）
	if draft.GetParams().Extra != nil {
		if feeVal, ok := draft.GetParams().Extra["estimated_fee"].(string); ok {
			fee = feeVal
		}
		if changeVal, ok := draft.GetParams().Extra["change"].(string); ok {
			change = changeVal
		}
	}

	return fee, change
}

// signWithPrivateKey 使用私钥签名哈希值
func signWithPrivateKey(hash []byte, privateKeyBytes []byte) ([]byte, error) {
	// 将私钥字节转换为ECDSA私钥
	privateKey, err := crypto.ToECDSA(privateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	// 使用私钥签名哈希
	signature, err := crypto.Sign(hash, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign: %w", err)
	}

	return signature, nil
}

// validateTransferRequest 验证转账请求
func (s *TransferService) validateTransferRequest(req *TransferRequest) error {
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

	return nil
}

// EstimateFee 估算转账手续费（供UI显示）
func (s *TransferService) EstimateFee(ctx context.Context, from, to string, amount string) (string, error) {
	// 解析金额
	amt, err := builder.NewAmountFromString(amount)
	if err != nil {
		return "", fmt.Errorf("invalid amount: %w", err)
	}

	// 使用builder估算费用
	estimatedFee, err := s.builder.EstimateFeeForTransfer(ctx, from, amt)
	if err != nil {
		return "", fmt.Errorf("estimate fee: %w", err)
	}

	return estimatedFee.String(), nil
}

// GetBalance 获取地址余额（供UI显示）
func (s *TransferService) GetBalance(ctx context.Context, address string) (string, error) {
	// 直接传递 Base58 地址给服务端（服务端要求 Base58 格式，拒绝 0x 前缀）
	// 查询UTXOs
	utxos, err := s.transport.GetUTXOs(ctx, address, nil)
	if err != nil {
		return "", fmt.Errorf("get utxos: %w", err)
	}

	// 计算总余额
	totalBalance := builder.Zero()
	for _, utxo := range utxos {
		utxoAmount, err := builder.NewAmountFromString(utxo.Amount)
		if err != nil {
			continue
		}
		totalBalance = totalBalance.Add(utxoAmount)
	}

	return totalBalance.String(), nil
}

// convertAddressToHex 将Base58地址转换为十六进制格式
func (s *TransferService) convertAddressToHex(addr string) (string, error) {
	if s.addressManager == nil {
		// 降级：假设已经是十六进制格式
		return addr, nil
	}

	// 使用 AddressManager 将 Base58 地址转为字节数组
	addressBytes, err := s.addressManager.AddressToBytes(addr)
	if err != nil {
		return "", fmt.Errorf("convert address to bytes: %w", err)
	}

	// 转为十六进制并添加 0x 前缀
	addressHex := "0x" + hex.EncodeToString(addressBytes)
	// 临时调试：验证转换是否正确
	fmt.Printf("[DEBUG] Address conversion: %s -> %s\n", addr, addressHex)
	return addressHex, nil
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
