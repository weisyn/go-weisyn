// Package orchestrator 提供区块提交功能
//
// 🎯 **共识模式感知的区块提交实现**：
//   - 分布式共识模式: 提交给聚合器进行网络共识
//   - 单节点开发模式: 直接本地处理，立即确认
//
// 根据配置的 enable_aggregator 标志自动选择正确的提交路径。
package orchestrator

import (
	"context"
	"fmt"

	blocktypes "github.com/weisyn/v1/pb/blockchain/block"
)

// ==================== 区块提交方法（共识模式感知） ====================

// submitBlockToAggregator 提交挖出的区块（根据共识模式自动分支）
//
// 🎯 **共识模式分支处理**：
//   - 分布式共识模式: 提交给聚合器，等待网络共识
//   - 单节点开发模式: 直接本地处理，立即确认
//
// @param ctx 上下文对象
// @param minedBlock 已挖出的完整区块
// @return error 提交过程中的错误
func (s *MiningOrchestratorService) submitBlockToAggregator(ctx context.Context, minedBlock *blocktypes.Block) error {
	if s.logger != nil {
		s.logger.Info("开始提交挖出的区块")
	}

	// ⚠️ 系统内不存在“单节点共识模式”，统一走聚合器共识入口：
	// - 无其它节点/首个节点启动时，聚合器自身也会“本地应用最终区块”，并在有 peers 时广播；
	// - 这样保证系统语义一致，不引入分叉的双路径。
	return s.submitToDistributedConsensus(ctx, minedBlock)
}

// submitToDistributedConsensus 分布式共识模式：提交给聚合器（V2 新增弃权重选）
//
// 🎯 **生产环境标准路径**：
//   - 通过聚合器控制器提交区块
//   - 聚合器会判断本节点是否为当前高度的聚合节点
//   - 如果是聚合节点，则执行聚合选择流程
//   - 如果不是，则转发给正确的聚合节点
//
// 🔄 **V2 新增弃权重选机制**：
//   - 检测弃权响应（waived=true）
//   - 记录弃权节点并重选下一个聚合器
//   - 回环兜底：所有候选都弃权时，由原始矿工处理
//
// @param ctx 上下文对象
// @param minedBlock 已挖出的完整区块
// @return error 提交过程中的错误
func (s *MiningOrchestratorService) submitToDistributedConsensus(ctx context.Context, minedBlock *blocktypes.Block) error {
	// 检查挖出的区块是否为 nil
	if minedBlock == nil {
		return fmt.Errorf("挖出的区块不能为空")
	}

	// 检查区块头是否为 nil
	if minedBlock.Header == nil {
		return fmt.Errorf("挖出的区块头不能为空")
	}

	if s.logger != nil {
		s.logger.Info("使用分布式聚合器共识模式提交区块")
	}

	// V2 新增：带弃权重选的提交逻辑
	height := minedBlock.Header.Height

	// 通过聚合器控制器接口提交（内部包含选举、转发、弃权重选等完整逻辑）
	// ProcessAggregationRound 内部会：
	// 1. 检查本节点是否为聚合器
	// 2. 如果不是，调用 forwardBlockToCorrectAggregator（支持重选）
	// 3. 如果是，执行聚合流程
	err := s.aggregatorController.ProcessAggregationRound(ctx, minedBlock)
	if err != nil {
		if s.logger != nil {
			s.logger.Infof("聚合器处理失败: %v", err)
		}
		return fmt.Errorf("聚合器处理失败: %v", err)
	}

	if s.logger != nil {
		s.logger.Infof("✅ 成功提交区块给聚合器，区块高度: %d", height)
	}

	return nil
}
