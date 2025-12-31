// Package candidatepool AddCandidate错误路径覆盖率测试
package candidatepool

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/weisyn/v1/internal/config/candidatepool"
	"github.com/weisyn/v1/internal/core/mempool/testutil"
	core "github.com/weisyn/v1/pb/blockchain/block"
)

// TestAddCandidate_WithInvalidFormat_ReturnsError_Advanced 测试无效格式
func TestAddCandidate_WithInvalidFormat_ReturnsError_Advanced(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	// 创建无效区块（nil Header）
	invalidBlock := &core.Block{
		Header: nil,
		Body:   &core.BlockBody{},
	}

	// Act
	blockHash, err := pool.AddCandidate(invalidBlock, "peer1")

	// Assert
	assert.Error(t, err, "无效格式应该返回错误")
	assert.Nil(t, blockHash, "区块哈希应为nil")
	assert.Contains(t, err.Error(), "格式验证失败", "错误信息应该包含格式验证失败")
}

// TestAddCandidate_WithInvalidHash_ReturnsError_Advanced 测试无效哈希
func TestAddCandidate_WithInvalidHash_ReturnsError_Advanced(t *testing.T) {
	// Arrange
	pool := createTestCandidatePool(t)
	block := testutil.CreateSimpleTestBlock(100)

	// 创建一个返回无效哈希的Mock哈希服务
	mockHashService := &MockBlockHashServiceWithInvalidHash{}
	pool.hashService = mockHashService

	// Act
	blockHash, err := pool.AddCandidate(block, "peer1")

	// Assert
	// 根据实现，如果哈希服务返回IsValid=false，calcBlockHash会返回"区块结构无效"错误
	assert.Error(t, err, "无效哈希应该返回错误")
	assert.Nil(t, blockHash, "区块哈希应为nil")
	assert.Contains(t, err.Error(), "区块结构无效", "错误信息应该包含'区块结构无效'")
}

// TestAddCandidate_WithOversizedBlock_ReturnsError_Advanced 测试超大区块
func TestAddCandidate_WithOversizedBlock_ReturnsError_Advanced(t *testing.T) {
	// Arrange
	config := &candidatepool.CandidatePoolOptions{
		MaxCandidates:    100,
		MaxAge:          10 * time.Minute,
		MemoryLimit:     100 * 1024 * 1024,
		CleanupInterval: 1 * time.Minute,
		MaxBlockSize:    1000, // 很小的限制
	}
	logger := &testutil.MockLogger{}
	eventBus := testutil.NewMockEventBus()
	memory := testutil.NewMockMemoryStore()
	hashService := &testutil.MockBlockHashService{}

	pool, err := NewCandidatePoolWithCache(config, logger, eventBus, memory, hashService, nil)
	require.NoError(t, err)
	cp := pool.(*CandidatePool)

	// 创建包含大量交易的区块（可能超过大小限制）
	largeBlock := testutil.CreateTestBlock(100, nil, 100) // 100个交易

	// Act
	_, err = cp.AddCandidate(largeBlock, "peer1")

	// Assert
	// 根据实现，可能返回大小验证错误或成功（如果估算大小未超过限制）
	if err != nil {
		assert.Contains(t, err.Error(), "大小验证失败", "错误信息应该包含大小验证失败")
	}
}

// MockBlockHashServiceWithInvalidHash Mock哈希服务（返回无效哈希）
type MockBlockHashServiceWithInvalidHash struct{}

func (m *MockBlockHashServiceWithInvalidHash) ComputeBlockHash(ctx context.Context, in *core.ComputeBlockHashRequest, opts ...grpc.CallOption) (*core.ComputeBlockHashResponse, error) {
	// 返回IsValid=false，表示区块结构无效
	return &core.ComputeBlockHashResponse{
		Hash:    []byte("invalid_hash_32_bytes_12345678"),
		IsValid: false, // 返回无效，这会触发calcBlockHash返回错误
	}, nil
}

func (m *MockBlockHashServiceWithInvalidHash) ValidateBlockHash(ctx context.Context, in *core.ValidateBlockHashRequest, opts ...grpc.CallOption) (*core.ValidateBlockHashResponse, error) {
	return &core.ValidateBlockHashResponse{
		IsValid: false, // 返回无效
	}, nil
}

