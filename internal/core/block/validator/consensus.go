// Package validator 实现区块验证服务
package validator

import (
	"context"
	"fmt"
	"math/big"

	"github.com/weisyn/v1/internal/core/block/difficulty"
	core "github.com/weisyn/v1/pb/blockchain/block"
)

// mtpCache 全局MTP缓存实例
//
// 使用全局缓存可以在多个验证调用之间共享MTP计算结果，
// 显著减少同步过程中的数据库查询次数。
//
// 缓存容量为10000，足够覆盖大多数同步场景。
// 链重组时需要调用 InvalidateAbove 清除受影响的缓存。
var mtpCache = difficulty.GlobalMTPCache

// validateConsensus 验证共识规则
//
// 🎯 **共识验证检查项**：
// 1. PoW验证（WES 使用 PoW+XOR 混合共识，PoW 是基础层）
// 2. 难度验证
// 3. 区块哈希验证（Hash < Target）
//
// ⚠️ **创世区块处理**：
// - 创世区块（高度=0）也需要通过PoW验证
// - 创世区块构建后需要进行挖矿来找到满足难度要求的Nonce
//
// 参数：
//   - ctx: 上下文
//   - block: 待验证区块
//
// 返回：
//   - error: 验证错误（nil表示通过）
func (s *Service) validateConsensus(ctx context.Context, block *core.Block) error {
	if block == nil || block.Header == nil {
		return fmt.Errorf("区块或区块头为空")
	}

	// 0. 获取 v2 共识参数（非向后兼容：强制存在）
	if s.configProvider == nil {
		return fmt.Errorf("configProvider 未注入（v2 规则要求必需）")
	}
	consensusOpts := s.configProvider.GetConsensus()
	if consensusOpts == nil {
		return fmt.Errorf("无法获取共识配置（GetConsensus 返回 nil）")
	}
	chainOpts := s.configProvider.GetBlockchain()
	if chainOpts == nil {
		return fmt.Errorf("无法获取区块链配置（GetBlockchain 返回 nil）")
	}

	// 目标出块时间（秒，至少 1s）
	targetSec := uint64(consensusOpts.TargetBlockTime.Seconds())
	if targetSec == 0 {
		targetSec = 1
	}

	params := difficulty.Params{
		TargetBlockTimeSeconds:             targetSec,
		DifficultyWindow:                   consensusOpts.POW.DifficultyWindow,
		MaxAdjustUpPPM:                     consensusOpts.POW.MaxAdjustUpPPM,
		MaxAdjustDownPPM:                   consensusOpts.POW.MaxAdjustDownPPM,
		EMAAlphaPPM:                        consensusOpts.POW.EMAAlphaPPM,
		MinDifficulty:                      consensusOpts.POW.MinDifficulty,
		MaxDifficulty:                      consensusOpts.POW.MaxDifficulty,
		MTPWindow:                          consensusOpts.POW.MTPWindow,
		MinBlockIntervalSeconds:            uint64(chainOpts.Block.MinBlockInterval),
		MaxFutureDriftSeconds:              consensusOpts.POW.MaxFutureDriftSeconds,
		EmergencyDownshiftThresholdSeconds: consensusOpts.POW.EmergencyDownshiftThresholdSeconds,
		MaxEmergencyDownshiftBits:          consensusOpts.POW.MaxEmergencyDownshiftBits,
	}

	// 1. 验证难度字段非 0
	if block.Header.Difficulty == 0 {
		return fmt.Errorf("区块难度不能为0")
	}

	// 2. v2 时间戳有效性规则（MTP + 最小间隔 + future drift）
	// 使用带缓存的版本减少IO压力，避免MTP计算超时
	if err := difficulty.ValidateTimestampRulesWithCache(ctx, s.queryService, block.Header, params, mtpCache); err != nil {
		return fmt.Errorf("时间戳规则校验失败: %w", err)
	}

	// 3. v2 难度正确性校验（expectedDifficulty 必须匹配）
	if block.Header.Height == 0 {
		// 创世区块：难度必须等于配置的 initial_difficulty
		if block.Header.Difficulty != consensusOpts.POW.InitialDifficulty {
			return fmt.Errorf("创世区块难度不匹配: got=%d expected=%d",
				block.Header.Difficulty, consensusOpts.POW.InitialDifficulty)
		}
	} else {
		parentBlock, err := s.queryService.GetBlockByHash(ctx, block.Header.PreviousHash)
		if err != nil || parentBlock == nil || parentBlock.Header == nil {
			return fmt.Errorf("获取父区块失败，无法校验难度: %w", err)
		}
		expected, err := difficulty.NextDifficultyForTimestamp(ctx, s.queryService, parentBlock.Header, block.Header.Timestamp, params)
		if err != nil {
			return fmt.Errorf("计算 expectedDifficulty 失败: %w", err)
		}
		if block.Header.Difficulty != expected {
			return fmt.Errorf("区块难度不匹配: got=%d expected=%d height=%d",
				block.Header.Difficulty, expected, block.Header.Height)
		}
	}

	// 4. 计算区块哈希（使用 gRPC 服务）
	if s.blockHashClient == nil {
		return fmt.Errorf("blockHashClient 未初始化")
	}

	req := &core.ComputeBlockHashRequest{
		Block: block,
	}
	resp, err := s.blockHashClient.ComputeBlockHash(ctx, req)
	if err != nil {
		return fmt.Errorf("调用区块哈希服务失败: %w", err)
	}

	if !resp.IsValid {
		return fmt.Errorf("区块结构无效")
	}

	blockHash := resp.Hash

	// 5. 验证 PoW（区块哈希必须小于目标值）
	// Target = 2^(256 - Difficulty)
	// 哈希值必须小于 Target 才满足 PoW 要求
	target := s.calculateTarget(block.Header.Difficulty)
	hashInt := new(big.Int).SetBytes(blockHash)

	if hashInt.Cmp(target) >= 0 {
		if s.logger != nil {
			s.logger.Warnf("⚠️ PoW验证失败: 区块哈希 %x >= 目标值（难度=%d）",
				blockHash[:min(8, len(blockHash))], block.Header.Difficulty)
		}
		return fmt.Errorf("PoW验证失败: 区块哈希不满足难度要求（难度=%d）", block.Header.Difficulty)
	}

	if s.logger != nil {
		if block.Header.Height == 0 {
			s.logger.Debugf("✅ 创世区块PoW验证通过: 难度=%d, Nonce=%x", block.Header.Difficulty, block.Header.Nonce)
		} else {
			s.logger.Debugf("✅ PoW验证通过: 难度=%d", block.Header.Difficulty)
		}
	}

	return nil
}

// calculateTarget 计算 PoW 目标值
//
// 🎯 **PoW 目标值计算**
//
// WES 使用标准 PoW 难度计算：
// Target = 2^(256 - Difficulty)
//
// 区块哈希必须小于 Target 才满足 PoW 要求
//
// 参数：
//   - difficulty: 区块难度值
//
// 返回：
//   - *big.Int: PoW 目标值
func (s *Service) calculateTarget(difficulty uint64) *big.Int {
	// 标准 PoW 目标值计算
	// Target = 2^(256 - Difficulty)
	// 难度越大，目标值越小，挖矿越难

	// 最大难度：256（目标值为1）
	// 最小难度：0（目标值为 2^256）
	maxDifficulty := uint64(256)
	if difficulty > maxDifficulty {
		difficulty = maxDifficulty
	}

	// 计算目标值
	// Target = 2^(256 - Difficulty)
	exp := uint64(256) - difficulty
	target := new(big.Int)
	target.Exp(big.NewInt(2), big.NewInt(int64(exp)), nil)

	return target
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// InvalidateMTPCacheAbove 使指定高度以上的MTP缓存失效
//
// 在链重组时调用此函数，确保缓存不会返回过期的MTP值。
//
// 参数：
//   - height: 分叉点高度，该高度以上的所有缓存都会被清除
func InvalidateMTPCacheAbove(height uint64) {
	if mtpCache != nil {
		mtpCache.InvalidateAbove(height)
	}
}

// GetMTPCacheStats 获取MTP缓存统计信息
//
// 用于监控和调试缓存性能。
//
// 返回：
//   - size: 当前缓存大小
//   - capacity: 缓存容量
//   - hits: 缓存命中次数
//   - misses: 缓存未命中次数
//   - hitRate: 缓存命中率
func GetMTPCacheStats() (size int, capacity int, hits uint64, misses uint64, hitRate float64) {
	if mtpCache != nil {
		return mtpCache.Stats()
	}
	return 0, 0, 0, 0, 0
}
