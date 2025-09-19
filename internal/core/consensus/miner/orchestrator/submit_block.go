// Package orchestrator 提供区块提交功能
//
// 实现矿工向Aggregator提交挖出区块的功能。
// 通过内部接口委托给Aggregator统一处理。
package orchestrator

import (
	"context"
	"fmt"

	blocktypes "github.com/weisyn/v1/pb/blockchain/block"
)

// ==================== 区块提交方法 ====================

// submitBlockToAggregator 向Aggregator提交挖出的区块
//
// 通过内部接口将挖出的区块提交给本地Aggregator进行处理。
//
// @param ctx 上下文对象
// @param minedBlock 已挖出的完整区块
// @return error 提交过程中的错误
func (s *MiningOrchestratorService) submitBlockToAggregator(ctx context.Context, minedBlock *blocktypes.Block) error {
	if s.logger != nil {
		s.logger.Info("开始向Aggregator提交挖出的区块")
	}

	// 通过聚合器控制器接口直接提交给本地Aggregator
	// Aggregator会自动判断是否为聚合节点，并处理相应的转发或聚合逻辑
	err := s.aggregatorController.ProcessAggregationRound(ctx, minedBlock)
	if err != nil {
		if s.logger != nil {
			s.logger.Infof("向Aggregator提交区块失败: %v", err)
		}
		return fmt.Errorf("aggregator processing failed: %v", err)
	}

	if s.logger != nil {
		s.logger.Infof("成功提交区块给Aggregator，区块高度: %d",
			minedBlock.Header.Height)
	}

	if s.logger != nil {
		s.logger.Info("区块已成功提交给Aggregator")
	}
	return nil
}
