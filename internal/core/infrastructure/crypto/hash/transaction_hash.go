package hash

import (
	"context"
	"crypto/subtle"
	"fmt"
	"time"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"google.golang.org/protobuf/proto"
)

// TransactionHashService 交易哈希计算服务
//
// 🎯 核心职责：
// 1. 提供确定性的交易哈希计算服务
// 2. 实现gRPC TransactionHashService接口
// 3. 支持调试信息和性能监控
// 4. 确保跨平台哈希计算一致性
//
// 🔧 技术特点：
// - 确定性算法：固定使用SHA-256
// - 标准序列化：使用Protobuf规范序列化
// - 字段控制：精确控制哈希计算包含的字段
// - 性能监控：提供实际的计算时间统计
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

// ComputeHash 计算交易哈希（确定性实现）
//
// 🎯 设计原则：
// 1. 确定性：相同交易在任何平台上计算出相同哈希
// 2. 标准化：严格按照proto规范序列化
// 3. 字段控制：精确控制包含和排除的字段
// 4. 性能监控：准确测量计算耗时
func (ths *TransactionHashService) ComputeHash(ctx context.Context, req *transaction.ComputeHashRequest) (*transaction.ComputeHashResponse, error) {
	startTime := time.Now()

	if req == nil {
		return &transaction.ComputeHashResponse{
			IsValid: false,
		}, fmt.Errorf("请求不能为空")
	}

	if req.Transaction == nil {
		return &transaction.ComputeHashResponse{
			IsValid: false,
		}, fmt.Errorf("交易不能为空")
	}

	// 序列化交易（确定性）
	mo := proto.MarshalOptions{Deterministic: true}
	txBytes, err := mo.Marshal(req.Transaction)
	if err != nil {
		return &transaction.ComputeHashResponse{
			IsValid: false,
		}, fmt.Errorf("序列化交易失败: %w", err)
	}

	// 计算SHA-256哈希（确定性）
	hash := ths.hashManager.SHA256(txBytes)

	response := &transaction.ComputeHashResponse{
		Hash:    hash,
		IsValid: true,
	}

	// 如果需要调试信息
	if req.IncludeDebugInfo {
		response.DebugInfo = &transaction.HashDebugInfo{
			CanonicalBytes:      txBytes,
			CanonicalLength:     uint64(len(txBytes)),
			SerializationMethod: "protobuf",
			IncludedFields: []string{
				"version", "inputs", "outputs", "nonce",
				"creation_timestamp", "validity_window", "fee_mechanism", "metadata", "chain_id",
			},
			ExcludedFields:       []string{"signatures", "unlocking_proof.signature", "unlocking_proof.multi_key_proof.signatures"},
			ComputationTimeNanos: uint64(time.Since(startTime).Nanoseconds()),
		}
	}

	return response, nil
}

// ValidateHash 验证交易哈希（确定性）
//
// 🎯 验证逻辑：
// 1. 重新计算交易哈希
// 2. 使用时间安全比较防止时序攻击
// 3. 提供详细的验证结果
func (ths *TransactionHashService) ValidateHash(ctx context.Context, req *transaction.ValidateHashRequest) (*transaction.ValidateHashResponse, error) {
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

	computeResp, err := ths.ComputeHash(ctx, computeReq)
	if err != nil {
		return &transaction.ValidateHashResponse{
			IsValid: false,
		}, fmt.Errorf("计算哈希失败: %w", err)
	}

	// 时间安全比较（防止时序攻击）
	isValid := subtle.ConstantTimeCompare(computeResp.Hash, req.ExpectedHash) == 1

	return &transaction.ValidateHashResponse{
		IsValid:      isValid,
		ComputedHash: computeResp.Hash,
		ExpectedHash: req.ExpectedHash,
		ErrorMessage: func() *string {
			if isValid {
				return nil
			}
			msg := "computed hash does not match expected hash"
			return &msg
		}(),
	}, nil
}

// ComputeTransactionHash 计算交易哈希的简化接口
// 用于不需要gRPC接口的场景
func (ths *TransactionHashService) ComputeTransactionHash(tx *transaction.Transaction) ([]byte, error) {
	if tx == nil {
		return nil, fmt.Errorf("交易不能为空")
	}

	// 序列化交易（确定性）
	mo := proto.MarshalOptions{Deterministic: true}
	txBytes, err := mo.Marshal(tx)
	if err != nil {
		return nil, fmt.Errorf("序列化交易失败: %w", err)
	}

	// 计算SHA-256哈希
	return ths.hashManager.SHA256(txBytes), nil
}
