// Package fork 链权重计算实现
package fork

import (
	"bytes"
	"context"
	"fmt"
	"math/big"

	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/types"
	"google.golang.org/protobuf/proto"
)

// ============================================================================
//                              链权重计算实现
// ============================================================================

// calculateChainWeight 计算链权重
//
// 🎯 **链权重计算核心逻辑**
//
// 权重计算方法：
// 1. 累积难度：所有区块难度之和
// 2. 区块数量：链的长度
// 3. 最后区块时间：用于平局时的决策
//
// 参数：
//   - fromHeight: 起始高度（包含）
//   - toHeight: 结束高度（包含）
//
// 返回：
//   - *types.ChainWeight: 链权重
//   - error: 计算错误
func (s *Service) calculateChainWeight(ctx context.Context, fromHeight, toHeight uint64) (*types.ChainWeight, error) {
	return s.calculateChainWeightWithProvider(ctx, fromHeight, toHeight, nil)
}

func (s *Service) calculateChainWeightWithProvider(
	ctx context.Context,
	fromHeight, toHeight uint64,
	provider func(height uint64) (*core.Block, bool),
) (*types.ChainWeight, error) {
	if fromHeight > toHeight {
		return nil, fmt.Errorf("起始高度 %d 大于结束高度 %d", fromHeight, toHeight)
	}

	if s.logger != nil {
		s.logger.Debugf("计算链权重: 高度范围 %d -> %d", fromHeight, toHeight)
	}

	// 初始化权重
	weight := &types.ChainWeight{
		CumulativeDifficulty: big.NewInt(0),
		BlockCount:           0,
		LastBlockTime:        0,
	}

	// 遍历指定高度范围内的所有区块
	for height := fromHeight; height <= toHeight; height++ {
		// 获取区块
		var blk *core.Block
		var ok bool
		if provider != nil {
			blk, ok = provider(height)
		}
		if !ok {
			var err error
			blk, err = s.queryService.GetBlockByHeight(ctx, height)
			if err != nil {
				return nil, fmt.Errorf("获取高度 %d 的区块失败: %w", height, err)
			}
		}

		if blk == nil || blk.Header == nil {
			return nil, fmt.Errorf("高度 %d 的区块无效", height)
		}

		// 累加难度
		// 注意：这里假设区块头包含难度字段
		// 如果没有，可以使用固定难度或从其他地方获取
		blockDifficulty := s.getBlockDifficulty(blk)
		weight.CumulativeDifficulty.Add(weight.CumulativeDifficulty, blockDifficulty)

		// 增加区块计数
		weight.BlockCount++

		// 更新最后区块时间
		if blk.Header.Timestamp > 0 {
			weight.LastBlockTime = int64(blk.Header.Timestamp)
		}

		// 记录链尖哈希（用于确定性 tie-break）
		if height == toHeight {
			tipHash, err := s.computeDeterministicBlockHash(ctx, blk)
			if err != nil {
				return nil, fmt.Errorf("计算链尖区块哈希失败(height=%d): %w", height, err)
			}
			weight.TipHash = tipHash
		}
	}

	if s.logger != nil {
		s.logger.Debugf("链权重计算完成: 累积难度=%s, 区块数=%d, 最后时间=%d",
			weight.CumulativeDifficulty.String(), weight.BlockCount, weight.LastBlockTime)
	}

	return weight, nil
}

// computeDeterministicBlockHash 计算区块哈希（确定性、与挖矿/验证一致）。
//
// 优先使用 BlockHashServiceClient（与系统路径一致）；
// 若不可用/失败，则回退到本地：DoubleSHA256(proto.Marshal(header))，保证可用性与确定性。
func (s *Service) computeDeterministicBlockHash(ctx context.Context, blk *core.Block) ([]byte, error) {
	if blk == nil || blk.Header == nil {
		return nil, fmt.Errorf("区块或区块头为空")
	}

	// 1) 优先走 blockHashClient（更“系统路径”）
	if s.blockHashClient != nil {
		resp, err := s.blockHashClient.ComputeBlockHash(ctx, &core.ComputeBlockHashRequest{
			Block:            blk,
			IncludeDebugInfo: false,
		})
		if err == nil && resp != nil && resp.IsValid && len(resp.Hash) > 0 {
			// 防御性拷贝：避免后续被复用修改
			return bytes.Clone(resp.Hash), nil
		}
	}

	// 2) 回退：本地计算（保持与 internal/core/block/hash/service.go 一致的 DoubleSHA256(headerBytes)）
	if s.hasher == nil {
		return nil, fmt.Errorf("hasher 未注入")
	}
	headerBytes, err := proto.Marshal(blk.Header)
	if err != nil {
		return nil, fmt.Errorf("序列化区块头失败: %w", err)
	}
	return s.hasher.DoubleSHA256(headerBytes), nil
}

// ============================================================================
//                              难度获取
// ============================================================================

// getBlockDifficulty 获取区块难度
//
// 🔢 **难度提取逻辑**
//
// 难度来源（按优先级）：
// 1. 区块头的难度字段
// 2. 从 POW 数据计算
// 3. 默认难度值（从配置系统获取）
func (s *Service) getBlockDifficulty(block *core.Block) *big.Int {
	// 方法1：从区块头获取难度（Difficulty是uint64类型）
	if block.Header != nil && block.Header.Difficulty > 0 {
		difficulty := new(big.Int)
		difficulty.SetUint64(block.Header.Difficulty)
		return difficulty
	}

	// 🔧 修复：从配置系统获取默认难度值，移除硬编码
	var defaultDifficultyValue uint64 = 1 // 默认最小难度
	if s.configProvider != nil {
		consensusOpts := s.configProvider.GetConsensus()
		if consensusOpts != nil {
			// 使用共识配置中的最小难度作为默认值
			// ConsensusOptions 包含 POW 配置，直接访问 MinDifficulty
			if consensusOpts.POW.MinDifficulty > 0 {
				defaultDifficultyValue = consensusOpts.POW.MinDifficulty
			}
		}
	}

	defaultDifficulty := big.NewInt(0).SetUint64(defaultDifficultyValue)

	if s.logger != nil {
		s.logger.Debugf("使用默认难度: %s (来自配置系统)", defaultDifficulty.String())
	}

	return defaultDifficulty
}

// ============================================================================
//                              权重比较
// ============================================================================

// CompareChainWeight 比较两条链的权重
//
// 🔍 **权重比较工具函数**
//
// 返回：
//   - 1: weight1 > weight2
//   - 0: weight1 == weight2
//   - -1: weight1 < weight2
func CompareChainWeight(weight1, weight2 *types.ChainWeight) int {
	// 检查权重参数是否为 nil
	if weight1 == nil && weight2 == nil {
		return 0 // 都为 nil，视为相等
	}
	if weight1 == nil {
		return -1 // weight1 为 nil，weight2 更大
	}
	if weight2 == nil {
		return 1 // weight2 为 nil，weight1 更大
	}

	// 检查累积难度是否为 nil
	if weight1.CumulativeDifficulty == nil && weight2.CumulativeDifficulty == nil {
		// 两者都为 nil，比较其他字段
	} else if weight1.CumulativeDifficulty == nil {
		return -1 // weight1 的累积难度为 nil，weight2 更大
	} else if weight2.CumulativeDifficulty == nil {
		return 1 // weight2 的累积难度为 nil，weight1 更大
	} else {
		// 1. 比较累积难度
		cmp := weight1.CumulativeDifficulty.Cmp(weight2.CumulativeDifficulty)
		if cmp != 0 {
			return cmp
		}
	}

	// 2. 累积难度相同，比较区块数量
	if weight1.BlockCount > weight2.BlockCount {
		return 1
	}
	if weight1.BlockCount < weight2.BlockCount {
		return -1
	}

	// 3. 区块数量相同，确定性 tie-break：tip hash 更小的优先（按固定字节序比较）
	// 说明：LastBlockTime 仅用于观测，不应作为 tie-break（可被操纵且可能导致不收敛）。
	if len(weight1.TipHash) > 0 || len(weight2.TipHash) > 0 {
		cmp := bytes.Compare(weight1.TipHash, weight2.TipHash)
		if cmp < 0 {
			return 1
		}
		if cmp > 0 {
			return -1
		}
	} else {
		// 向后兼容：若未提供 tip hash，退化为旧规则（更早时间戳优先）
	if weight1.LastBlockTime < weight2.LastBlockTime {
		return 1
	}
	if weight1.LastBlockTime > weight2.LastBlockTime {
		return -1
		}
	}

	// 4. 完全相同
	return 0
}
