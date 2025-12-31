// Package controller 实现聚合器控制服务
//
// 🎯 **聚合器控制模块**
//
// 本包实现 AggregatorController 接口，提供聚合器生命周期管理：
// - 启动和停止聚合器服务
// - 处理聚合轮次请求
// - 获取聚合状态信息
package controller

import (
	"context"

	"github.com/weisyn/v1/internal/config/consensus"
	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	"github.com/weisyn/v1/pb/blockchain/block"
	blockiface "github.com/weisyn/v1/pkg/interfaces/block"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	netiface "github.com/weisyn/v1/pkg/interfaces/network"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
)

// AggregatorControllerService 聚合器控制服务实现（薄委托层）
type AggregatorControllerService struct {
	logger         log.Logger                 // 日志记录器
	starter        *aggregationStarter        // 聚合启动器
	stopper        *aggregationStopper        // 聚合停止器
	statusProvider *aggregationStatusProvider // 状态提供器
}

// NewAggregatorControllerService 创建聚合器控制服务实例
func NewAggregatorControllerService(
	logger log.Logger,
	stateManager interfaces.AggregatorStateManager,
	// 添加编排所需的子组件依赖
	election interfaces.AggregatorElection,
	candidateCollector interfaces.CandidateCollector,
	decisionCalculator interfaces.DecisionCalculator,
	distanceSelector interfaces.DistanceSelector,
	resultDistributor interfaces.ResultDistributor,
	// 新增网络和候选池依赖
	candidatePool mempool.CandidatePool,
	networkService netiface.Network,
	p2pService p2pi.Service,
	routingTableManager kademlia.RoutingTableManager,
	config *consensus.ConsensusOptions, // 添加配置参数
	chainQuery persistence.QueryService,
	blockHashClient block.BlockHashServiceClient,
	blockProcessor blockiface.BlockProcessor, // 区块处理服务
) interfaces.AggregatorController {
	// 创建聚合启动器（传入编排所需的组件和配置）
	starter := newAggregationStarter(
		logger,
		stateManager,
		election,
		candidateCollector,
		decisionCalculator,
		distanceSelector,
		resultDistributor,
		candidatePool,
		networkService,
		p2pService,
		routingTableManager,
		config,
		chainQuery,
		blockHashClient,
		blockProcessor,
	)

	// 创建聚合停止器
	stopper := newAggregationStopper(logger, stateManager)

	// 创建状态提供器
	statusProvider := newAggregationStatusProvider(logger, stateManager)

	return &AggregatorControllerService{
		logger:         logger,
		starter:        starter,
		stopper:        stopper,
		statusProvider: statusProvider,
	}
}

// 编译时确保 AggregatorControllerService 实现了 AggregatorController 接口
var _ interfaces.AggregatorController = (*AggregatorControllerService)(nil)

// ProcessAggregationRound 处理聚合轮次
func (s *AggregatorControllerService) ProcessAggregationRound(ctx context.Context, candidateBlock *block.Block) error {
	s.logger.Info("收到区块聚合处理请求")

	// 委托给聚合启动器处理
	return s.starter.processAggregationRound(ctx, candidateBlock)
}

// StartAggregatorService 启动聚合器服务
func (s *AggregatorControllerService) StartAggregatorService(ctx context.Context) error {
	s.logger.Info("收到启动聚合器服务请求")

	// 委托给聚合启动器处理
	return s.starter.startAggregatorService(ctx)
}

// StopAggregatorService 停止聚合器服务
func (s *AggregatorControllerService) StopAggregatorService(ctx context.Context) error {
	s.logger.Info("收到停止聚合器服务请求")

	// 委托给聚合停止器处理
	return s.stopper.stopAggregatorService(ctx)
}
