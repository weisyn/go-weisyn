package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/weisyn/v1/internal/core/block/difficulty"
)

// waitForMiningSlot enforces v2 timestamp rules on the mining side:
// do not start building/mining a candidate until we are past the earliest allowed timestamp.
//
// This prevents "喷发式出块" where miners can immediately produce many blocks with identical/too-close timestamps.
func (s *MiningOrchestratorService) waitForMiningSlot(ctx context.Context) error {
	if s == nil {
		return fmt.Errorf("orchestrator is nil")
	}
	if s.configProvider == nil || s.queryService == nil || s.chainQuery == nil {
		// v2: hard requirement (non-compat)
		return fmt.Errorf("缺少 v2 必需依赖：configProvider/queryService/chainQuery")
	}

	consensusOpts := s.configProvider.GetConsensus()
	if consensusOpts == nil {
		return fmt.Errorf("无法获取共识配置（GetConsensus 返回 nil）")
	}
	chainOpts := s.configProvider.GetBlockchain()
	if chainOpts == nil {
		return fmt.Errorf("无法获取区块链配置（GetBlockchain 返回 nil）")
	}

	chainInfo, err := s.chainQuery.GetChainInfo(ctx)
	if err != nil {
		return fmt.Errorf("获取链信息失败: %w", err)
	}

	// Empty chain: no best hash, no need to wait.
	// If height==0 but BestBlockHash exists, it means genesis is present and we MUST enforce min interval vs genesis.
	if chainInfo == nil || len(chainInfo.BestBlockHash) != 32 {
		return nil
	}

	parentBlock, err := s.queryService.GetBlockByHash(ctx, chainInfo.BestBlockHash)
	if err != nil {
		return fmt.Errorf("获取父区块失败（用于等待挖矿窗口）: %w", err)
	}
	if parentBlock == nil || parentBlock.Header == nil {
		return fmt.Errorf("获取父区块失败（用于等待挖矿窗口）: parent block is nil/invalid")
	}

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

	// 使用带缓存的版本计算最早允许时间戳，减少IO压力
	earliest, err := difficulty.EarliestAllowedTimestampWithCache(ctx, s.queryService, parentBlock.Header, params, difficulty.GlobalMTPCache)
	if err != nil {
		return fmt.Errorf("计算 earliestAllowedTimestamp 失败: %w", err)
	}

	now := uint64(time.Now().Unix())
	if now >= earliest {
		return nil
	}

	waitSec := earliest - now
	waitDur := time.Duration(waitSec) * time.Second
	if s.logger != nil {
		s.logger.Infof("⏱️ v2 最小出块间隔生效：等待 %s 再进入挖矿（earliest=%d now=%d）",
			waitDur, earliest, now)
	}

	timer := time.NewTimer(waitDur)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
