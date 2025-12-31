// Package hash 提供交易哈希计算服务
//
// 🎯 核心职责：
// 1. 提供确定性的交易哈希计算服务
// 2. 实现gRPC TransactionHashService接口
// 3. 支持调试信息和性能监控
// 4. 确保跨平台哈希计算一致性
package hash

import (
	"context"
	"crypto/subtle"
	"fmt"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"google.golang.org/protobuf/proto"
)

// TransactionHashService 实现交易哈希服务
type TransactionHashService struct {
	transaction.UnimplementedTransactionHashServiceServer
	hashManager crypto.HashManager
	logger      log.Logger
}

// NewTransactionHashService 创建交易哈希服务实例
func NewTransactionHashService(hashManager crypto.HashManager, logger log.Logger) *TransactionHashService {
	return &TransactionHashService{
		hashManager: hashManager,
		logger:      logger,
	}
}

// ComputeHash 计算交易哈希（确定性）
// 排除签名字段，用于交易ID计算
func (s *TransactionHashService) ComputeHash(ctx context.Context, req *transaction.ComputeHashRequest) (*transaction.ComputeHashResponse, error) {
	if req.Transaction == nil {
		return &transaction.ComputeHashResponse{
			Hash:    nil,
			IsValid: false,
		}, fmt.Errorf("交易不能为空")
	}

	// 创建交易副本，排除签名字段
	txCopy := proto.Clone(req.Transaction).(*transaction.Transaction)
	// 清空所有输入的解锁证明（包含签名）
	for _, input := range txCopy.Inputs {
		input.UnlockingProof = nil
	}

	// 序列化交易（已排除签名）进行哈希计算
	mo := proto.MarshalOptions{Deterministic: true}
	txBytes, err := mo.Marshal(txCopy)
	if err != nil {
		return &transaction.ComputeHashResponse{
			Hash:    nil,
			IsValid: false,
		}, fmt.Errorf("序列化交易失败: %w", err)
	}

	// 使用HashManager接口的SHA-256算法（确定性）
	hash := s.hashManager.SHA256(txBytes)

	response := &transaction.ComputeHashResponse{
		Hash:    hash,
		IsValid: true,
	}

	return response, nil
}

// ValidateHash 验证交易哈希（确定性）
func (s *TransactionHashService) ValidateHash(ctx context.Context, req *transaction.ValidateHashRequest) (*transaction.ValidateHashResponse, error) {
	if req == nil {
		return &transaction.ValidateHashResponse{
			IsValid: false,
		}, fmt.Errorf("验证请求不能为空")
	}

	// 计算实际哈希
	computeReq := &transaction.ComputeHashRequest{
		Transaction:      req.Transaction,
		IncludeDebugInfo: false, // 验证时不需要调试信息
	}

	computeResp, err := s.ComputeHash(ctx, computeReq)
	if err != nil {
		return &transaction.ValidateHashResponse{
			IsValid: false,
		}, fmt.Errorf("计算交易哈希失败: %w", err)
	}

	// 比较哈希值
	isValid := len(computeResp.Hash) == len(req.ExpectedHash) &&
		subtle.ConstantTimeCompare(computeResp.Hash, req.ExpectedHash) == 1

	response := &transaction.ValidateHashResponse{
		IsValid:      isValid,
		ComputedHash: computeResp.Hash,
		ExpectedHash: req.ExpectedHash,
	}

	if !isValid {
		errorMsg := "交易哈希验证失败：计算的哈希与期望值不匹配"
		response.ErrorMessage = &errorMsg
	}

	return response, nil
}

// ComputeSignatureHash 计算签名哈希（用于签名和验证）
// 支持 SIGHASH 类型处理
func (s *TransactionHashService) ComputeSignatureHash(ctx context.Context, req *transaction.ComputeSignatureHashRequest) (*transaction.ComputeSignatureHashResponse, error) {
	if req.Transaction == nil {
		return &transaction.ComputeSignatureHashResponse{
			Hash:    nil,
			IsValid: false,
		}, fmt.Errorf("交易不能为空")
	}

	if int(req.InputIndex) >= len(req.Transaction.Inputs) {
		return &transaction.ComputeSignatureHashResponse{
			Hash:    nil,
			IsValid: false,
		}, fmt.Errorf("输入索引超出范围: %d", req.InputIndex)
	}

	// 创建交易副本，排除签名字段
	txCopy := proto.Clone(req.Transaction).(*transaction.Transaction)
	// 清空所有输入的解锁证明（包含签名）
	for _, input := range txCopy.Inputs {
		input.UnlockingProof = nil
	}

	// 根据 SIGHASH 类型处理交易结构
	// 简化实现：当前只支持 SIGHASH_ALL（包含所有输入和输出）
	// TODO: 实现完整的 SIGHASH 类型支持（SIGHASH_NONE, SIGHASH_SINGLE, ANYONECANPAY等）
	if req.SighashType != transaction.SignatureHashType_SIGHASH_ALL {
		if s.logger != nil {
			s.logger.Warnf("⚠️ 不支持的 SIGHASH 类型: %v，使用 SIGHASH_ALL", req.SighashType)
		}
	}

	// 序列化交易进行哈希计算
	mo := proto.MarshalOptions{Deterministic: true}
	txBytes, err := mo.Marshal(txCopy)
	if err != nil {
		return &transaction.ComputeSignatureHashResponse{
			Hash:    nil,
			IsValid: false,
		}, fmt.Errorf("序列化交易失败: %w", err)
	}

	// 添加输入索引和 SIGHASH 类型到哈希计算
	// 这确保了不同输入和不同 SIGHASH 类型会产生不同的哈希
	hasher := s.hashManager.NewSHA256Hasher()
	hasher.Write(txBytes)
	hasher.Write([]byte{byte(req.InputIndex), byte(req.SighashType)})
	hash := hasher.Sum(nil)

	// 🔍 调试：记录签名哈希计算的关键数据（使用 logger 确保输出到日志）
	txID := s.hashManager.SHA256(txBytes)
	var txIDPrefix, hashPrefix string
	if len(txID) >= 8 {
		txIDPrefix = fmt.Sprintf("%x", txID[:8])
	} else {
		txIDPrefix = fmt.Sprintf("%x", txID)
	}
	if len(hash) >= 8 {
		hashPrefix = fmt.Sprintf("%x", hash[:8])
	} else {
		hashPrefix = fmt.Sprintf("%x", hash)
	}
	// 调试日志：仅在 Debug 级别输出，避免生产环境产生过多日志
	if s.logger != nil {
		s.logger.Debugf("🔐 [TxHashService.ComputeSignatureHash] txID=%s inputIndex=%d sighashType=%v sigHash=%s",
			txIDPrefix, req.InputIndex, req.SighashType, hashPrefix)
	}

	response := &transaction.ComputeSignatureHashResponse{
		Hash:    hash,
		IsValid: true,
	}

	return response, nil
}

// ValidateSignatureHash 验证签名哈希（用于签名验证）
func (s *TransactionHashService) ValidateSignatureHash(ctx context.Context, req *transaction.ValidateSignatureHashRequest) (*transaction.ValidateSignatureHashResponse, error) {
	if req == nil {
		return &transaction.ValidateSignatureHashResponse{
			IsValid: false,
		}, fmt.Errorf("验证请求不能为空")
	}

	// 计算实际签名哈希
	computeReq := &transaction.ComputeSignatureHashRequest{
		Transaction:      req.Transaction,
		InputIndex:       req.InputIndex,
		SighashType:      req.SighashType,
		IncludeDebugInfo: false, // 验证时不需要调试信息
	}

	computeResp, err := s.ComputeSignatureHash(ctx, computeReq)
	if err != nil {
		return &transaction.ValidateSignatureHashResponse{
			IsValid: false,
		}, fmt.Errorf("计算签名哈希失败: %w", err)
	}

	// 比较哈希值
	isValid := len(computeResp.Hash) == len(req.ExpectedHash) &&
		subtle.ConstantTimeCompare(computeResp.Hash, req.ExpectedHash) == 1

	response := &transaction.ValidateSignatureHashResponse{
		IsValid:      isValid,
		ComputedHash: computeResp.Hash,
		ExpectedHash: req.ExpectedHash,
	}

	if !isValid {
		errorMsg := "签名哈希验证失败：计算的哈希与期望值不匹配"
		response.ErrorMessage = &errorMsg
	}

	return response, nil
}
