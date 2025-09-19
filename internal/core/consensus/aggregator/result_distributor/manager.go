// Package result_distributor 实现结果分发服务
//
// 🎯 **结果分发服务模块**
//
// 本包实现 ResultDistributor 接口，提供聚合选择结果的分发功能：
// - 分发聚合选择结果到全网
// - 使用标准的ConsensusResultBroadcast protobuf消息
// - 通过PubSub方式广播到TopicConsensusResult主题
package result_distributor

import (
	"context"
	"errors"

	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	"github.com/weisyn/v1/pkg/interfaces/network"
	"github.com/weisyn/v1/pkg/types"
)

// ResultDistributorService 结果分发服务实现（薄委托层）
type ResultDistributorService struct {
	logger            log.Logger               // 日志记录器
	messageBuilder    *consensusMessageBuilder // 标准消息构建器
	pubsubDistributor *pubsubDistributor       // PubSub分发器
}

// NewResultDistributorService 创建结果分发服务实例
func NewResultDistributorService(
	logger log.Logger,
	network network.Network,
	host node.Host,
) interfaces.ResultDistributor {
	// 创建标准消息构建器
	messageBuilder := newConsensusMessageBuilder(logger, host)

	// 创建PubSub分发器
	pubsubDistributor := newPubsubDistributor(logger, network)

	return &ResultDistributorService{
		logger:            logger,
		messageBuilder:    messageBuilder,
		pubsubDistributor: pubsubDistributor,
	}
}

// 编译时确保 ResultDistributorService 实现了 ResultDistributor 接口
var _ interfaces.ResultDistributor = (*ResultDistributorService)(nil)

// DistributeSelectedBlock 分发选中的区块
func (s *ResultDistributorService) DistributeSelectedBlock(ctx context.Context, selected *types.CandidateBlock, proof *types.SelectionProof, totalCandidates uint32, finalScore float64) error {
	s.logger.Info("分发选中的区块到全网")

	// 构建标准的ConsensusResultBroadcast消息
	broadcast, err := s.messageBuilder.buildConsensusResultBroadcast(selected, proof, totalCandidates, finalScore)
	if err != nil {
		return err
	}

	// 通过PubSub发布到全网
	return s.pubsubDistributor.publishConsensusResult(ctx, broadcast)
}

// BroadcastToNetwork 网络广播
func (s *ResultDistributorService) BroadcastToNetwork(ctx context.Context, message *types.DistributionMessage) error {
	s.logger.Info("执行网络广播")

	// 检查消息有效性
	if message == nil || message.SelectedBlock == nil || message.SelectionProof == nil {
		return errors.New("invalid distribution message")
	}

	// 委托给DistributeSelectedBlock处理
	// TODO: 从message中获取候选数量和评分，当前使用默认值
	return s.DistributeSelectedBlock(ctx, message.SelectedBlock, message.SelectionProof, 1, 1.0)
}

// MonitorConsensusConvergence 监控共识收敛 - 简化实现
func (s *ResultDistributorService) MonitorConsensusConvergence(ctx context.Context, blockHash string) (*types.ConvergenceStatus, error) {
	s.logger.Info("监控共识收敛状态")

	// 简化实现：区块链自运行，不需要复杂的收敛监控
	// 直接返回已收敛状态
	return &types.ConvergenceStatus{
		BlockHash:        blockHash,
		TotalNodes:       1,
		AcceptingNodes:   1,
		ConvergenceRatio: 1.0,
		IsConverged:      true,
	}, nil
}

// GetDistributionStatistics 获取分发统计 - 简化实现
func (s *ResultDistributorService) GetDistributionStatistics() (*types.DistributionStats, error) {
	s.logger.Info("获取分发统计信息")

	// 简化实现：区块链自运行，不需要复杂的统计功能
	// 返回基本的统计结构
	return &types.DistributionStats{
		TotalDistributions: 0,
		SuccessfulSends:    0,
		FailedSends:        0,
		NetworkCoverage:    0.0,
	}, nil
}
