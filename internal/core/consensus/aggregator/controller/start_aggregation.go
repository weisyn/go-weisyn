// start_aggregation.go
// 启动聚合轮次的业务逻辑实现
//
// 核心业务功能：
// 1. 启动指定高度的聚合轮次处理
// 2. 检查聚合节点资格
// 3. 初始化聚合流程状态
//
// 作者：WES开发团队
// 创建时间：2025-09-13

package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	"github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pb/network/protocol"
	"github.com/weisyn/v1/pkg/constants/protocols"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	netiface "github.com/weisyn/v1/pkg/interfaces/network"
	"github.com/weisyn/v1/pkg/types"
	"google.golang.org/protobuf/proto"
)

// aggregationStarter 聚合轮次启动器
type aggregationStarter struct {
	logger       log.Logger
	stateManager interfaces.AggregatorStateManager
	// 添加编排所需的子组件
	election           interfaces.AggregatorElection
	candidateCollector interfaces.CandidateCollector
	decisionCalculator interfaces.DecisionCalculator
	blockSelector      interfaces.BlockSelector
	distanceSelector   interfaces.DistanceSelector // 距离选择器
	resultDistributor  interfaces.ResultDistributor
	// 新增网络和候选池依赖
	candidatePool  mempool.CandidatePool
	networkService netiface.Network
	host           node.Host
	// 新增K桶管理器依赖，用于清理不兼容的外部节点
	routingTableManager kademlia.RoutingTableManager
}

// newAggregationStarter 创建聚合轮次启动器
func newAggregationStarter(
	logger log.Logger,
	stateManager interfaces.AggregatorStateManager,
	election interfaces.AggregatorElection,
	candidateCollector interfaces.CandidateCollector,
	decisionCalculator interfaces.DecisionCalculator,
	blockSelector interfaces.BlockSelector,
	distanceSelector interfaces.DistanceSelector,
	resultDistributor interfaces.ResultDistributor,
	candidatePool mempool.CandidatePool,
	networkService netiface.Network,
	host node.Host,
	routingTableManager kademlia.RoutingTableManager,
) *aggregationStarter {
	return &aggregationStarter{
		logger:              logger,
		stateManager:        stateManager,
		election:            election,
		candidateCollector:  candidateCollector,
		decisionCalculator:  decisionCalculator,
		blockSelector:       blockSelector,
		distanceSelector:    distanceSelector,
		resultDistributor:   resultDistributor,
		candidatePool:       candidatePool,
		networkService:      networkService,
		host:                host,
		routingTableManager: routingTableManager,
	}
}

// processAggregationRound 处理区块聚合轮次（新的统一入口）
//
// 🎯 **新的统一处理逻辑**：
// 1. 聚合节点选举判断
// 2. 非聚合节点：转发给正确的聚合节点
// 3. 聚合节点：添加到候选池并触发聚合流程
func (s *aggregationStarter) processAggregationRound(ctx context.Context, candidateBlock *block.Block) error {
	s.logger.Info("开始处理区块聚合轮次")

	// 1. 聚合节点选举判断
	height := candidateBlock.Header.Height
	s.logger.Infof("🔍 开始聚合器选举判断，区块高度: %d", height)

	isAggregator, err := s.election.IsAggregatorForHeight(height)
	if err != nil {
		s.logger.Errorf("❌ 聚合器选举失败: %v", err)
		return fmt.Errorf("aggregator election failed: %v", err)
	}

	if !isAggregator {
		// 2. 不是聚合节点，转发给正确的聚合节点
		s.logger.Infof("❌ 当前节点不是高度 %d 的聚合节点，进行转发", height)
		return s.forwardBlockToCorrectAggregator(ctx, candidateBlock)
	}

	// 3. 是聚合节点，添加到候选池并触发聚合流程
	s.logger.Infof("✅ 确认为高度 %d 的聚合节点，开始本地处理候选区块", height)

	// 添加到候选池
	blockHash, err := s.candidatePool.AddCandidate(candidateBlock, string(s.host.ID()))
	if err != nil {
		return fmt.Errorf("failed to add candidate to pool: %v", err)
	}
	s.logger.Infof("候选区块已添加到候选池，哈希: %s", blockHash[:8])

	// 触发ABS聚合流程
	return s.executeABSAggregationFlow(ctx, height)

}

// forwardBlockToCorrectAggregator 转发区块给正确的聚合节点
func (s *aggregationStarter) forwardBlockToCorrectAggregator(ctx context.Context, candidateBlock *block.Block) error {
	height := candidateBlock.Header.Height

	// 获取该高度的正确聚合节点
	targetAggregator, err := s.election.GetAggregatorForHeight(height)
	if err != nil {
		return fmt.Errorf("failed to get aggregator for height %d: %v", height, err)
	}

	// 🔒 严格安全检查：验证目标聚合器是否支持区块提交协议
	supported, err := s.networkService.CheckProtocolSupport(ctx, targetAggregator, protocols.ProtocolBlockSubmission)

	// ❌ 协议检查失败 - 拒绝发送，可能是外部节点或网络问题
	if err != nil {
		s.logger.Errorf("🚫 协议检查失败，拒绝向节点 %s 发送区块数据: %v", targetAggregator, err)
		return fmt.Errorf("protocol check failed for aggregator %s: %v - refusing to send block data to potentially incompatible node", targetAggregator, err)
	}

	// ❌ 节点不支持协议 - 这是外部节点，需要清理并拒绝
	if !supported {
		s.logger.Errorf("🚫 节点 %s 不支持协议 %s，这是外部节点！正在从K桶中移除...",
			targetAggregator, protocols.ProtocolBlockSubmission)

		// 🧹 从K桶中移除不兼容的外部节点
		if err := s.routingTableManager.RemovePeer(targetAggregator); err != nil {
			s.logger.Warnf("从K桶移除外部节点 %s 失败: %v", targetAggregator, err)
		} else {
			s.logger.Infof("✅ 成功从K桶移除外部节点: %s", targetAggregator)
		}

		return fmt.Errorf("external node %s does not support protocol %s - removed from routing table to prevent future selection",
			targetAggregator, protocols.ProtocolBlockSubmission)
	}

	// ✅ 协议检查通过
	s.logger.Debugf("✅ 已验证聚合器 %s 支持协议: %s", targetAggregator, protocols.ProtocolBlockSubmission)

	// 构建 MinerBlockSubmission 消息
	submission := &protocol.MinerBlockSubmission{
		Base: &protocol.BaseMessage{
			MessageId:     generateMessageID(),
			SenderId:      []byte(s.host.ID()),
			TimestampUnix: time.Now().Unix(),
		},
		CandidateBlock:   candidateBlock,
		MinerPeerId:      []byte(s.host.ID()),
		MiningDifficulty: candidateBlock.Header.Difficulty,
		ParentHash:       candidateBlock.Header.PreviousHash,
		RelayHopLimit:    1,
	}

	// 序列化消息
	reqBytes, err := proto.Marshal(submission)
	if err != nil {
		// 🔍 序列化失败调试信息
		s.logger.Errorf("🚫 MinerBlockSubmission序列化失败 - height=%d, error=%v", height, err)
		return fmt.Errorf("failed to serialize submission: %v", err)
	}

	// 🔍 序列化成功调试信息
	s.logger.Debugf("✅ MinerBlockSubmission序列化成功 - height=%d, size=%d, target=%s", height, len(reqBytes), targetAggregator)

	// 发送给正确的聚合节点
	_, err = s.networkService.Call(ctx, targetAggregator, protocols.ProtocolBlockSubmission, reqBytes, nil)
	if err != nil {
		return fmt.Errorf("network call failed to %s: %v", targetAggregator, err)
	}

	s.logger.Infof("成功转发区块给聚合节点: %s", targetAggregator)
	return nil
}

// executeABSAggregationFlow 执行ABS聚合流程
func (s *aggregationStarter) executeABSAggregationFlow(ctx context.Context, height uint64) error {
	// 2. 状态转换：Listening
	if err := s.stateManager.TransitionTo(types.AggregationStateListening); err != nil {
		return err
	}
	if err := s.stateManager.SetCurrentHeight(height); err != nil {
		return err
	}

	// 3. 状态转换：Collecting - 启动固定收集窗口
	//
	// 🎯 **固定收集窗口策略**：
	// - 从接收第一个候选区块开始，启动固定时间窗口
	// - 窗口期间收集所有到达的候选区块
	// - 窗口结束后立即进行选择，不等待更多候选
	// - 目标：给足够时间让各矿工的候选区块到达聚合器
	if err := s.stateManager.TransitionTo(types.AggregationStateCollecting); err != nil {
		return err
	}

	// 固定收集窗口时间（可配置）
	collectionDuration := 10 * time.Second // 默认10秒收集窗口
	// TODO: 从配置中获取 collectionDuration = s.config.Aggregator.CollectionWindowDuration

	err := s.candidateCollector.StartCollectionWindow(height, collectionDuration)
	if err != nil {
		return err
	}

	s.logger.Infof("🕐 固定收集窗口已启动：%v，高度: %d", collectionDuration, height)

	// 4. 等待收集窗口结束并获取所有候选区块
	candidates, err := s.candidateCollector.CloseCollectionWindow(height)
	if err != nil {
		return err
	}

	s.logger.Infof("✅ 收集窗口结束，共收集到 %d 个候选区块", len(candidates))

	// 5. 状态转换：Evaluating - XOR距离计算
	if err := s.stateManager.TransitionTo(types.AggregationStateEvaluating); err != nil {
		return err
	}

	// 获取父区块哈希作为距离计算基准
	parentBlockHash, err := s.getParentBlockHash(height)
	if err != nil {
		return fmt.Errorf("failed to get parent block hash: %v", err)
	}

	// 计算所有候选区块的XOR距离
	distanceResults, err := s.distanceSelector.CalculateDistances(ctx, candidates, parentBlockHash)
	if err != nil {
		return fmt.Errorf("failed to calculate distances: %v", err)
	}

	s.logger.Info("候选区块距离计算完成")

	// 6. 状态转换：Selecting - 选择距离最近的区块
	if err := s.stateManager.TransitionTo(types.AggregationStateSelecting); err != nil {
		return err
	}

	selected, err := s.distanceSelector.SelectClosestBlock(ctx, distanceResults)
	if err != nil {
		return fmt.Errorf("failed to select closest block: %v", err)
	}

	s.logger.Info("最优区块选择完成")

	// 7. 生成距离选择证明（给全网其他节点验证用）
	distanceProof, err := s.distanceSelector.GenerateDistanceProof(ctx, selected, distanceResults, parentBlockHash)
	if err != nil {
		return fmt.Errorf("failed to generate distance proof: %v", err)
	}

	s.logger.Info("距离选择证明生成完成")

	// 8. 状态转换：Distributing - 立即分发结果
	//
	// 🎯 **固定分发时机策略**：
	// - 收集窗口结束后立即选择最优区块并分发
	// - 不基于区块时间戳进行任何等待
	// - 不考虑最小区块间隔（由矿工侧难度调整控制）
	// - 目标：确保网络及时获得聚合结果，保持链的活跃性
	if err := s.stateManager.TransitionTo(types.AggregationStateDistributing); err != nil {
		return err
	}

	// 计算真实的候选数量和距离值作为评分
	totalCandidates := uint32(len(distanceResults))
	finalScore := 1.0 // 距离选择不需要复杂评分，使用固定值

	// 创建标准格式的选择证明
	selectionProof := &types.SelectionProof{
		SelectedCandidate:   selected,
		SelectionReason:     "XOR距离选择",
		SelectionTimestamp:  distanceProof.GeneratedAt,
		AllCandidatesHash:   fmt.Sprintf("%x", distanceProof.DistanceSummary),
		ScoresHash:          fmt.Sprintf("%x", distanceProof.ProofHash),
		AggregatorSignature: []byte{}, // 暂时留空，等待签名系统集成
		AggregatorID:        s.host.ID(),
		BlockHeight:         height,
		ProofHash:           fmt.Sprintf("%x", distanceProof.ProofHash),
	}

	// 立即分发选择结果，不等待时间戳
	err = s.resultDistributor.DistributeSelectedBlock(ctx, selected, selectionProof, totalCandidates, finalScore)
	if err != nil {
		return fmt.Errorf("failed to distribute selected block: %v", err)
	}

	s.logger.Info("结果分发完成")

	// 9. 状态转换：Idle - 聚合完成，回到空闲状态
	if err := s.stateManager.TransitionTo(types.AggregationStateIdle); err != nil {
		return err
	}

	s.logger.Info("ABS聚合流程完成")
	return nil
}

// generateMessageID 生成唯一消息ID
func generateMessageID() string {
	return fmt.Sprintf("msg_%d_%s", time.Now().UnixNano(), "aggregator")
}

// startAggregatorService 启动聚合器服务
func (s *aggregationStarter) startAggregatorService(ctx context.Context) error {
	s.logger.Info("启动聚合器服务")

	// 检查当前状态
	currentState := s.stateManager.GetCurrentState()
	if currentState != types.AggregationStateIdle {
		return errors.New("聚合器服务已在运行或处于异常状态")
	}

	// 保持在空闲状态，等待聚合轮次触发
	s.logger.Info("聚合器服务已启动，等待聚合轮次")
	return nil
}
