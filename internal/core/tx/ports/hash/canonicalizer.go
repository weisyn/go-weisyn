// Package hash 提供 HashCanonicalizer 端口的实现
//
// canonicalizer.go: 规范化交易哈希计算实现
package hash

import (
	"context"
	"fmt"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// Canonicalizer 规范化哈希计算器
//
// 🎯 **核心职责**：实现规范化交易哈希计算，排除签名字段
//
// 💡 **设计理念**：
// 交易哈希必须排除签名字段，否则会导致签名验证失败（循环依赖）。
// 本实现通过 gRPC TransactionHashService 进行哈希计算，确保一致性。
//
// ⚠️ **关键实现**：
// - 通过 gRPC 服务计算交易哈希和签名哈希
// - 确保所有哈希计算统一通过 TransactionHashService
// - 支持 SIGHASH 类型处理
//
// 📞 **调用方**：
// - Signer 实现
// - ProofProvider 实现
// - AuthZ 验证插件
type Canonicalizer struct {
	txHashClient transaction.TransactionHashServiceClient
}

// NewCanonicalizer 创建新的规范化哈希计算器
//
// 参数：
//   - txHashClient: 交易哈希服务客户端（用于通过 gRPC 计算哈希）
//
// 返回：
//   - *Canonicalizer: 新创建的实例
func NewCanonicalizer(txHashClient transaction.TransactionHashServiceClient) *Canonicalizer {
	return &Canonicalizer{
		txHashClient: txHashClient,
	}
}

// ComputeTransactionHash 计算交易哈希（用于交易ID）
//
// 实现 tx.HashCanonicalizer 接口
//
// 🎯 **规范化规则**：
// 通过 gRPC TransactionHashService.ComputeHash 计算交易哈希，确保一致性。
//
// 参数：
//   - ctx: 上下文对象
//   - tx: 待计算哈希的交易
//
// 返回：
//   - []byte: 交易哈希（32字节）
//   - error: 计算失败
func (c *Canonicalizer) ComputeTransactionHash(
	ctx context.Context,
	tx *transaction.Transaction,
) ([]byte, error) {
	// 1. 参数校验
	if tx == nil {
		return nil, ErrInvalidTransaction
	}

	if c.txHashClient == nil {
		return nil, fmt.Errorf("transaction hash client is not initialized")
	}

	// 2. 使用 gRPC 服务计算交易哈希
	req := &transaction.ComputeHashRequest{
		Transaction:     tx,
		IncludeDebugInfo: false,
	}
	resp, err := c.txHashClient.ComputeHash(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCanonicalSerializationFailed, err)
	}

	if !resp.IsValid {
		return nil, ErrInvalidTransaction
	}

	return resp.Hash, nil
}

// ComputeSignatureHash 计算签名哈希（用于签名和验证）
//
// 实现 tx.HashCanonicalizer 接口
//
// 🎯 **SIGHASH 类型处理**：
// 通过 gRPC TransactionHashService.ComputeSignatureHash 计算签名哈希，支持所有 SIGHASH 类型。
//
// 参数：
//   - ctx: 上下文对象
//   - tx: 待计算哈希的交易
//   - inputIndex: 当前输入索引
//   - sighashType: 签名哈希类型
//
// 返回：
//   - []byte: 签名哈希（32字节）
//   - error: 计算失败
func (c *Canonicalizer) ComputeSignatureHash(
	ctx context.Context,
	tx *transaction.Transaction,
	inputIndex int,
	sighashType transaction.SignatureHashType,
) ([]byte, error) {
	// 1. 参数校验
	if tx == nil {
		return nil, ErrInvalidTransaction
	}
	if inputIndex < 0 || inputIndex >= len(tx.Inputs) {
		return nil, ErrInputIndexOutOfRange
	}

	if c.txHashClient == nil {
		return nil, fmt.Errorf("transaction hash client is not initialized")
	}

	// 2. 使用 gRPC 服务计算签名哈希
	req := &transaction.ComputeSignatureHashRequest{
		Transaction:     tx,
		InputIndex:      uint32(inputIndex),
		SighashType:     sighashType,
		IncludeDebugInfo: false,
	}
	resp, err := c.txHashClient.ComputeSignatureHash(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCanonicalSerializationFailed, err)
	}

	if !resp.IsValid {
		return nil, ErrInvalidTransaction
	}

	return resp.Hash, nil
}

// ComputeSignatureHashForVerification 计算签名哈希（用于验证）
//
// 实现 tx.HashCanonicalizer 接口
//
// 💡 **设计理念**：
// 验证时的哈希计算逻辑与签名时完全相同，只是语义上更明确。
//
// 参数：
//   - ctx: 上下文对象
//   - tx: 待验证的交易（已包含签名）
//   - inputIndex: 当前输入索引
//   - sighashType: 签名哈希类型
//
// 返回：
//   - []byte: 签名哈希（32字节）
//   - error: 计算失败
func (c *Canonicalizer) ComputeSignatureHashForVerification(
	ctx context.Context,
	tx *transaction.Transaction,
	inputIndex int,
	sighashType transaction.SignatureHashType,
) ([]byte, error) {
	// 验证时的哈希计算逻辑与签名时完全相同
	return c.ComputeSignatureHash(ctx, tx, inputIndex, sighashType)
}

// ================================================================================================
// 🎯 错误定义（TX 内部错误,不暴露为公共接口）
// ================================================================================================

var (
	// ErrInvalidTransaction 交易结构无效
	ErrInvalidTransaction = fmt.Errorf("invalid transaction structure")

	// ErrCanonicalSerializationFailed 规范化序列化失败
	ErrCanonicalSerializationFailed = fmt.Errorf("canonical serialization failed")

	// ErrInputIndexOutOfRange 输入索引超出范围
	ErrInputIndexOutOfRange = fmt.Errorf("input index out of range")
)
