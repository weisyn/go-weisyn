// Package hash 提供区块哈希计算服务
//
// 🎯 核心职责：
// 1. 提供确定性的区块哈希计算服务
// 2. 实现gRPC BlockHashService接口
// 3. 支持调试信息和性能监控
// 4. 确保跨平台哈希计算一致性
package hash

import (
	"context"
	"fmt"
	"time"

	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"google.golang.org/protobuf/proto"
)

// BlockHashService 实现区块哈希服务
type BlockHashService struct {
	core.UnimplementedBlockHashServiceServer
	hashManager crypto.HashManager
	logger      log.Logger
}

// NewBlockHashService 创建区块哈希服务实例
func NewBlockHashService(hashManager crypto.HashManager, logger log.Logger) *BlockHashService {
	return &BlockHashService{
		hashManager: hashManager,
		logger:      logger,
	}
}

// ComputeBlockHash 计算区块哈希（确定性）
func (s *BlockHashService) ComputeBlockHash(ctx context.Context, req *core.ComputeBlockHashRequest) (*core.ComputeBlockHashResponse, error) {
	startTime := time.Now()

	if req.Block == nil || req.Block.Header == nil {
		return &core.ComputeBlockHashResponse{
			Hash:    nil,
			IsValid: false,
		}, fmt.Errorf("区块或区块头为空")
	}

	// 序列化区块头进行哈希计算
	headerBytes, err := proto.Marshal(req.Block.Header)
	if err != nil {
		return &core.ComputeBlockHashResponse{
			Hash:    nil,
			IsValid: false,
		}, fmt.Errorf("序列化区块头失败: %w", err)
	}

	// 使用HashManager接口的DoubleSHA-256算法（确定性，与挖矿保持一致）
	// 注意：挖矿使用DoubleSHA256，验证也必须使用DoubleSHA256，确保一致性
	hash := s.hashManager.DoubleSHA256(headerBytes)

	response := &core.ComputeBlockHashResponse{
		Hash:    hash,
		IsValid: true,
	}

	// 如果需要调试信息
	if req.IncludeDebugInfo {
		response.DebugInfo = &core.BlockHashDebugInfo{
			CanonicalBytes:      headerBytes,
			CanonicalLength:     uint64(len(headerBytes)),
			SerializationMethod: "protobuf",
			IncludedFields: []string{
				"version", "previous_hash", "merkle_root", "timestamp",
				"height", "nonce", "difficulty",
			},
			ExcludedFields:       []string{},
			ComputationTimeNanos: uint64(time.Since(startTime).Nanoseconds()),
		}
	}

	return response, nil
}

// ValidateBlockHash 验证区块哈希（确定性）
func (s *BlockHashService) ValidateBlockHash(ctx context.Context, req *core.ValidateBlockHashRequest) (*core.ValidateBlockHashResponse, error) {
	if req == nil {
		return &core.ValidateBlockHashResponse{
			IsValid: false,
		}, fmt.Errorf("验证请求不能为空")
	}

	// 计算实际哈希
	computeReq := &core.ComputeBlockHashRequest{
		Block:            req.Block,
		IncludeDebugInfo: false, // 验证时不需要调试信息
	}

	computeResp, err := s.ComputeBlockHash(ctx, computeReq)
	if err != nil {
		return &core.ValidateBlockHashResponse{
			IsValid: false,
		}, fmt.Errorf("计算区块哈希失败: %w", err)
	}

	// 比较哈希值
	isValid := string(computeResp.Hash) == string(req.ExpectedHash)

	response := &core.ValidateBlockHashResponse{
		IsValid:      isValid,
		ComputedHash: computeResp.Hash,
		ExpectedHash: req.ExpectedHash,
	}

	if !isValid {
		errorMsg := "区块哈希验证失败：计算的哈希与期望值不匹配"
		response.ErrorMessage = &errorMsg
	}

	return response, nil
}

// ComputeBlockHeaderHash 计算区块头哈希的简化接口
// 用于不需要gRPC接口的场景
func (s *BlockHashService) ComputeBlockHeaderHash(header *core.BlockHeader) ([]byte, error) {
	if header == nil {
		return nil, fmt.Errorf("区块头不能为空")
	}

	// 序列化区块头
	headerBytes, err := proto.Marshal(header)
	if err != nil {
		return nil, fmt.Errorf("序列化区块头失败: %w", err)
	}

	// 计算DoubleSHA-256哈希（与挖矿保持一致）
	return s.hashManager.DoubleSHA256(headerBytes), nil
}
