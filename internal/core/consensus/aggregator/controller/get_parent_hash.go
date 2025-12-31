// get_parent_hash.go
// 获取父区块哈希的辅助函数
//
// 主要功能：
// 1. 根据指定高度获取父区块哈希
// 2. 作为距离计算的基准
// 3. 处理创世区块的特殊情况
//
// 作者：WES开发团队
// 创建时间：2025-09-14

package controller

import (
	"context"
	"fmt"
	"time"

	core "github.com/weisyn/v1/pb/blockchain/block"
)

// getParentBlockHash 获取父区块哈希
//
// 参数：
// - ctx: 上下文（用于QueryService和BlockHashServiceClient调用）
// - height: 当前区块高度
//
// 返回：
// - []byte: 父区块哈希（来自真实链状态）
// - error: 获取错误
func (s *aggregationStarter) getParentBlockHash(ctx context.Context, height uint64) ([]byte, error) {
	consensusAggregatorParentHashRequests.Inc()
	start := time.Now()
	defer observeParentHashDuration(start)

	// 高度 0：创世块的“父哈希”约定为全零哈希
	if height == 0 {
		zeroHash := make([]byte, 32)
		return zeroHash, nil
	}

	// 聚合轮次针对的是高度 height 的候选区块，其父块高度应为 height-1
	parentHeight := height - 1

	// 必须通过统一 QueryService 获取真实父块
	if s.chainQuery == nil {
		consensusAggregatorParentHashErrors.Inc()
		return nil, fmt.Errorf("chain query service is not available for parent block hash lookup")
	}

	parentBlock, err := s.chainQuery.GetBlockByHeight(ctx, parentHeight)
	if err != nil {
		consensusAggregatorParentHashErrors.Inc()
		return nil, fmt.Errorf("failed to get parent block at height %d: %w", parentHeight, err)
	}
	if parentBlock == nil || parentBlock.Header == nil {
		consensusAggregatorParentHashErrors.Inc()
		return nil, fmt.Errorf("parent block or header is nil at height %d", parentHeight)
	}

	// 通过 BlockHashServiceClient 计算父块哈希，遵守统一哈希抽象
	if s.blockHashClient == nil {
		consensusAggregatorParentHashErrors.Inc()
		return nil, fmt.Errorf("block hash service client is not available for parent block hash calculation")
	}

	req := &core.ComputeBlockHashRequest{
		Block: parentBlock,
	}
	resp, err := s.blockHashClient.ComputeBlockHash(ctx, req)
	if err != nil {
		consensusAggregatorParentHashErrors.Inc()
		return nil, fmt.Errorf("failed to compute parent block hash at height %d: %w", parentHeight, err)
	}
	if resp == nil || !resp.IsValid || len(resp.Hash) == 0 {
		consensusAggregatorParentHashErrors.Inc()
		return nil, fmt.Errorf("invalid block hash response for parent height %d", parentHeight)
	}

	s.logger.Infof("获取父区块哈希完成: parent_height=%d, hash_prefix=%x", parentHeight, resp.Hash[:8])
	return resp.Hash, nil
}
