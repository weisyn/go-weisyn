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
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
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
	p2pService p2pi.Service,
	minPeerThreshold int,
) interfaces.ResultDistributor {
	// 创建标准消息构建器
	messageBuilder := newConsensusMessageBuilder(logger, p2pService)

	// 创建PubSub分发器（v2：引入网络健康门槛，避免阈值与网络现实不匹配）
	pubsubDistributor := newPubsubDistributor(logger, network, minPeerThreshold)

	return &ResultDistributorService{
		logger:            logger,
		messageBuilder:    messageBuilder,
		pubsubDistributor: pubsubDistributor,
	}
}

// 编译时确保 ResultDistributorService 实现了 ResultDistributor 接口
var _ interfaces.ResultDistributor = (*ResultDistributorService)(nil)

// DistributeSelectedBlock 分发选中的区块
func (s *ResultDistributorService) DistributeSelectedBlock(ctx context.Context, selected *types.CandidateBlock, proof *types.DistanceSelectionProof, totalCandidates uint32) error {
	s.logger.Info("分发选中的区块到全网")

	// 构建标准的ConsensusResultBroadcast消息
	broadcast, err := s.messageBuilder.buildConsensusResultBroadcast(selected, proof, totalCandidates)
	if err != nil {
		return err
	}

	// 通过PubSub发布到全网
	return s.pubsubDistributor.publishConsensusResult(ctx, broadcast)
}

// BroadcastToNetwork 网络广播
func (s *ResultDistributorService) BroadcastToNetwork(ctx context.Context, message *types.DistanceDistributionMessage) error {
	s.logger.Info("执行网络广播")

	// 检查消息有效性
	if message == nil || message.SelectedBlock == nil || message.SelectionProof == nil {
		return errors.New("invalid distribution message")
	}

	// 从消息中提取信息并分发
	return s.DistributeSelectedBlock(ctx, message.SelectedBlock, message.SelectionProof, message.SelectionProof.TotalCandidates)
}
