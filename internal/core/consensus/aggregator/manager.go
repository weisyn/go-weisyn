// Package aggregator 聚合节点管理器（薄实现层）
//
// 🎯 **聚合器薄实现层**
//
// 本包实现 InternalAggregatorService 接口，集成所有聚合器子组件：
// - 薄实现原则：只做接口方法委托，不包含业务逻辑
// - 组件集成：集成所有子组件提供完整ABS聚合服务
// - 架构一致性：与miner重构模式保持完全一致
package aggregator

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/weisyn/v1/internal/config/consensus"
	"github.com/weisyn/v1/internal/core/consensus/aggregator/block_selector"
	"github.com/weisyn/v1/internal/core/consensus/aggregator/candidate_collector"
	"github.com/weisyn/v1/internal/core/consensus/aggregator/controller"
	"github.com/weisyn/v1/internal/core/consensus/aggregator/decision_calculator"
	"github.com/weisyn/v1/internal/core/consensus/aggregator/distance_selector"
	"github.com/weisyn/v1/internal/core/consensus/aggregator/election"
	"github.com/weisyn/v1/internal/core/consensus/aggregator/event_handler"
	"github.com/weisyn/v1/internal/core/consensus/aggregator/network_handler"
	"github.com/weisyn/v1/internal/core/consensus/aggregator/result_distributor"
	"github.com/weisyn/v1/internal/core/consensus/aggregator/state_manager"
	networkintegration "github.com/weisyn/v1/internal/core/consensus/integration/network"
	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	"github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	"github.com/weisyn/v1/pkg/interfaces/network"
	"github.com/weisyn/v1/pkg/types"
)

// Manager 聚合器薄管理层
//
// 🎯 **设计原则**：严格的薄实现层
// - 只实现接口方法委托，所有具体逻辑都在子组件中
// - 集成所有子组件，提供完整的ABS聚合服务能力
// - 支持网络协议注册和统一路由
type Manager struct {
	// ========== 核心依赖 ==========
	logger       log.Logger              // 日志记录器
	blockService blockchain.BlockService // 区块服务依赖（用于处理共识结果）

	// ========== 业务子组件实例 ==========
	controllerService     interfaces.AggregatorController   // 控制器服务
	electionService       interfaces.AggregatorElection     // 选举服务
	networkHandlerService interfaces.NetworkProtocolHandler // 网络处理服务
	candidateCollector    interfaces.CandidateCollector     // 候选收集服务
	decisionCalculator    interfaces.DecisionCalculator     // 决策计算服务
	blockSelector         interfaces.BlockSelector          // 区块选择服务
	distanceSelector      interfaces.DistanceSelector       // 距离选择服务
	resultDistributor     interfaces.ResultDistributor      // 结果分发服务
	stateManagerService   interfaces.AggregatorStateManager // 状态管理服务
	eventHandlerService   interfaces.AggregatorEventHandler // 事件处理服务
}

// NewManager 创建聚合器薄管理器
func NewManager(
	logger log.Logger,
	eventBus event.EventBus,
	candidatePool mempool.CandidatePool,
	hashManager crypto.HashManager,
	signatureManager crypto.SignatureManager,
	keyManager crypto.KeyManager,
	powEngine crypto.POWEngine,
	host node.Host,
	network network.Network,
	chainService blockchain.ChainService,
	distanceCalculator kademlia.DistanceCalculator,
	config *consensus.ConsensusOptions,
	syncService blockchain.SystemSyncService,
	routingTableManager kademlia.RoutingTableManager,
	blockService blockchain.BlockService,
) interfaces.InternalAggregatorService {
	// 创建所有子组件服务实例
	electionService := election.NewAggregatorElectionService(
		logger,
		chainService,
		hashManager,
		distanceCalculator,
		host,
		network,
		routingTableManager,
	)
	candidateCollector := candidate_collector.NewCandidateCollectorService(
		logger,
		candidatePool,
		chainService,
		hashManager,
		host,
		powEngine,
		config,
	)
	decisionCalculator := decision_calculator.NewDecisionCalculatorService(
		logger,
		chainService,
		hashManager,
		host,
		config,
	)
	blockSelector := block_selector.NewBlockSelectorService(
		logger,
		hashManager,
		signatureManager,
		keyManager,
		host,
	)
	distanceSelector := distance_selector.New(
		logger,
		hashManager,
	)
	resultDistributor := result_distributor.NewResultDistributorService(
		logger,
		network,
		host,
	)
	stateManagerService := state_manager.NewAggregatorStateManagerService(
		logger,
	)

	// 控制器需要访问所有子组件来实现真正的聚合编排
	controllerService := controller.NewAggregatorControllerService(
		logger,
		stateManagerService,
		electionService,
		candidateCollector,
		decisionCalculator,
		blockSelector,
		distanceSelector,
		resultDistributor,
		candidatePool,
		network,
		host,
		routingTableManager,
	)

	// 网络处理器需要控制器来触发聚合流程
	networkHandlerService := network_handler.NewNetworkProtocolHandlerService(
		logger,
		electionService,
		chainService,
		candidatePool,
		host,
		network,
		controllerService,
		syncService,
		blockService,
	)

	// 事件处理器需要状态管理器来处理系统事件
	eventHandlerService := event_handler.NewAggregatorEventHandlerService(
		logger,
		stateManagerService,
	)

	// 创建Manager实例
	return &Manager{
		logger:                logger,
		blockService:          blockService,
		controllerService:     controllerService,
		electionService:       electionService,
		networkHandlerService: networkHandlerService,
		candidateCollector:    candidateCollector,
		decisionCalculator:    decisionCalculator,
		blockSelector:         blockSelector,
		distanceSelector:      distanceSelector,
		resultDistributor:     resultDistributor,
		stateManagerService:   stateManagerService,
		eventHandlerService:   eventHandlerService,
	}
}

// ============================================================================
//                           编译时接口检查
// ============================================================================

// 确保 Manager 实现了所有聚合器接口
var _ interfaces.InternalAggregatorService = (*Manager)(nil)                // 1. 聚合服务总接口
var _ interfaces.AggregatorController = (*Manager)(nil)                     // 2. 聚合器控制器
var _ interfaces.AggregatorElection = (*Manager)(nil)                       // 3. 聚合器选举
var _ interfaces.NetworkProtocolHandler = (*Manager)(nil)                   // 4. 网络协议处理器
var _ interfaces.CandidateCollector = (*Manager)(nil)                       // 5. 候选收集器
var _ interfaces.DecisionCalculator = (*Manager)(nil)                       // 6. 决策计算器
var _ interfaces.BlockSelector = (*Manager)(nil)                            // 7. 区块选择器
var _ interfaces.DistanceSelector = (*Manager)(nil)                         // 8. 距离选择器
var _ interfaces.ResultDistributor = (*Manager)(nil)                        // 9. 结果分发器
var _ interfaces.AggregatorStateManager = (*Manager)(nil)                   // 10. 状态管理器
var _ interfaces.AggregatorEventHandler = (*Manager)(nil)                   // 11. 事件处理器
var _ networkintegration.UnifiedAggregatorSubscribeRouter = (*Manager)(nil) // 网络订阅路由

// ============================================================================
//                           AggregatorController 接口实现（薄委托）
// ============================================================================

// ProcessAggregationRound 处理聚合轮次
func (m *Manager) ProcessAggregationRound(ctx context.Context, candidateBlock *block.Block) error {
	return m.controllerService.ProcessAggregationRound(ctx, candidateBlock)
}

// StartAggregatorService 启动聚合器服务
func (m *Manager) StartAggregatorService(ctx context.Context) error {
	return m.controllerService.StartAggregatorService(ctx)
}

// StopAggregatorService 停止聚合器服务
func (m *Manager) StopAggregatorService(ctx context.Context) error {
	return m.controllerService.StopAggregatorService(ctx)
}

// ============================================================================
//                           AggregatorElection 接口实现（薄委托）
// ============================================================================

// IsAggregatorForHeight 判断当前节点是否为指定高度的聚合节点
func (m *Manager) IsAggregatorForHeight(height uint64) (bool, error) {
	return m.electionService.IsAggregatorForHeight(height)
}

// GetAggregatorForHeight 获取指定高度的聚合节点ID
func (m *Manager) GetAggregatorForHeight(height uint64) (peer.ID, error) {
	return m.electionService.GetAggregatorForHeight(height)
}

// ValidateAggregatorEligibility 验证聚合节点资格
func (m *Manager) ValidateAggregatorEligibility(peerID peer.ID) (bool, error) {
	return m.electionService.ValidateAggregatorEligibility(peerID)
}

// ============================================================================
//                           NetworkProtocolHandler 接口实现（薄委托）
// ============================================================================

// HandleMinerBlockSubmission 处理矿工区块提交
func (m *Manager) HandleMinerBlockSubmission(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
	return m.networkHandlerService.HandleMinerBlockSubmission(ctx, from, reqBytes)
}

// HandleConsensusHeartbeat 处理共识心跳协议
func (m *Manager) HandleConsensusHeartbeat(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
	return m.networkHandlerService.HandleConsensusHeartbeat(ctx, from, reqBytes)
}

// HandleConsensusResultBroadcast 处理共识结果广播
func (m *Manager) HandleConsensusResultBroadcast(ctx context.Context, from peer.ID, topic string, data []byte) error {
	return m.networkHandlerService.HandleConsensusResultBroadcast(ctx, from, topic, data)
}

// ============================================================================
//                           CandidateCollector 接口实现（薄委托）
// ============================================================================

// StartCollectionWindow 启动候选收集窗口
func (m *Manager) StartCollectionWindow(height uint64, duration time.Duration) error {
	return m.candidateCollector.StartCollectionWindow(height, duration)
}

// CloseCollectionWindow 关闭收集窗口
func (m *Manager) CloseCollectionWindow(height uint64) ([]types.CandidateBlock, error) {
	return m.candidateCollector.CloseCollectionWindow(height)
}

// IsCollectionActive 检查收集窗口是否活跃
func (m *Manager) IsCollectionActive(height uint64) bool {
	return m.candidateCollector.IsCollectionActive(height)
}

// GetCollectionProgress 获取收集进度
func (m *Manager) GetCollectionProgress(height uint64) (*types.CollectionProgress, error) {
	return m.candidateCollector.GetCollectionProgress(height)
}

// ClearCandidatePool 清空候选区块内存池
func (m *Manager) ClearCandidatePool() (int, error) {
	return m.candidateCollector.ClearCandidatePool()
}

// ============================================================================
//                           DecisionCalculator 接口实现（薄委托）
// ============================================================================

// EvaluateAllCandidates 批量评估所有候选区块
func (m *Manager) EvaluateAllCandidates(candidates []types.CandidateBlock) ([]types.ScoredCandidate, error) {
	return m.decisionCalculator.EvaluateAllCandidates(candidates)
}

// CalculateABSScore 计算候选区块的ABS综合评分
func (m *Manager) CalculateABSScore(candidate *types.CandidateBlock) (*types.ABSScore, error) {
	return m.decisionCalculator.CalculateABSScore(candidate)
}

// ValidateEvaluationResult 验证评估结果
func (m *Manager) ValidateEvaluationResult(scores []types.ScoredCandidate) error {
	return m.decisionCalculator.ValidateEvaluationResult(scores)
}

// GetEvaluationStatistics 获取评估统计信息
func (m *Manager) GetEvaluationStatistics() (*types.EvaluationStats, error) {
	return m.decisionCalculator.GetEvaluationStatistics()
}

// ============================================================================
//                           BlockSelector 接口实现（薄委托）
// ============================================================================

// SelectBestCandidate 选择最优候选区块
func (m *Manager) SelectBestCandidate(scores []types.ScoredCandidate) (*types.CandidateBlock, error) {
	return m.blockSelector.SelectBestCandidate(scores)
}

// ApplyTieBreaking 处理平局情况
func (m *Manager) ApplyTieBreaking(tiedCandidates []types.ScoredCandidate) (*types.CandidateBlock, error) {
	return m.blockSelector.ApplyTieBreaking(tiedCandidates)
}

// GenerateSelectionProof 生成选择证明
func (m *Manager) GenerateSelectionProof(selected *types.CandidateBlock, scores []types.ScoredCandidate) (*types.SelectionProof, error) {
	return m.blockSelector.GenerateSelectionProof(selected, scores)
}

// ============================================================================
//                           DistanceSelector 接口实现（薄委托）
// ============================================================================

// CalculateDistances 计算所有候选区块与父区块的XOR距离
func (m *Manager) CalculateDistances(ctx context.Context, candidates []types.CandidateBlock, parentBlockHash []byte) ([]types.DistanceResult, error) {
	return m.distanceSelector.CalculateDistances(ctx, candidates, parentBlockHash)
}

// SelectClosestBlock 选择距离最近的区块
func (m *Manager) SelectClosestBlock(ctx context.Context, distanceResults []types.DistanceResult) (*types.CandidateBlock, error) {
	return m.distanceSelector.SelectClosestBlock(ctx, distanceResults)
}

// GenerateDistanceProof 生成距离选择证明
func (m *Manager) GenerateDistanceProof(ctx context.Context, selected *types.CandidateBlock, allResults []types.DistanceResult, parentBlockHash []byte) (*types.DistanceSelectionProof, error) {
	return m.distanceSelector.GenerateDistanceProof(ctx, selected, allResults, parentBlockHash)
}

// VerifyDistanceSelection 验证距离选择的正确性
func (m *Manager) VerifyDistanceSelection(ctx context.Context, selected *types.CandidateBlock, proof *types.DistanceSelectionProof) error {
	return m.distanceSelector.VerifyDistanceSelection(ctx, selected, proof)
}

// GetDistanceStatistics 获取距离选择统计信息
func (m *Manager) GetDistanceStatistics() *types.DistanceStatistics {
	return m.distanceSelector.GetDistanceStatistics()
}

// ============================================================================
//                           ResultDistributor 接口实现（薄委托）
// ============================================================================

// DistributeSelectedBlock 分发选中的区块
func (m *Manager) DistributeSelectedBlock(ctx context.Context, selected *types.CandidateBlock, proof *types.SelectionProof, totalCandidates uint32, finalScore float64) error {
	return m.resultDistributor.DistributeSelectedBlock(ctx, selected, proof, totalCandidates, finalScore)
}

// BroadcastToNetwork 网络广播
func (m *Manager) BroadcastToNetwork(ctx context.Context, message *types.DistributionMessage) error {
	return m.resultDistributor.BroadcastToNetwork(ctx, message)
}

// MonitorConsensusConvergence 监控共识收敛
func (m *Manager) MonitorConsensusConvergence(ctx context.Context, blockHash string) (*types.ConvergenceStatus, error) {
	return m.resultDistributor.MonitorConsensusConvergence(ctx, blockHash)
}

// GetDistributionStatistics 获取分发统计
func (m *Manager) GetDistributionStatistics() (*types.DistributionStats, error) {
	return m.resultDistributor.GetDistributionStatistics()
}

// ============================================================================
//                           AggregatorStateManager 接口实现（薄委托）
// ============================================================================

// GetCurrentState 获取当前聚合状态
func (m *Manager) GetCurrentState() interfaces.AggregationState {
	return m.stateManagerService.GetCurrentState()
}

// TransitionTo 转换到目标状态
func (m *Manager) TransitionTo(newState interfaces.AggregationState) error {
	return m.stateManagerService.TransitionTo(newState)
}

// IsValidTransition 验证状态转换
func (m *Manager) IsValidTransition(from, to interfaces.AggregationState) bool {
	return m.stateManagerService.IsValidTransition(from, to)
}

// GetStateHistory 获取状态转换历史
func (m *Manager) GetStateHistory(limit int) ([]types.StateTransition, error) {
	return m.stateManagerService.GetStateHistory(limit)
}

// GetCurrentHeight 获取当前聚合高度
func (m *Manager) GetCurrentHeight() uint64 {
	return m.stateManagerService.GetCurrentHeight()
}

// SetCurrentHeight 设置当前聚合高度
func (m *Manager) SetCurrentHeight(height uint64) error {
	return m.stateManagerService.SetCurrentHeight(height)
}

// ============================================================================
//                           AggregatorEventHandler接口实现（薄委托）
// ============================================================================

// HandleChainReorganized 处理链重组事件
func (m *Manager) HandleChainReorganized(ctx context.Context, eventData *types.ChainReorganizedEventData) error {
	return m.eventHandlerService.HandleChainReorganized(ctx, eventData)
}

// HandleNetworkQualityChanged 处理网络质量变化事件
func (m *Manager) HandleNetworkQualityChanged(ctx context.Context, eventData *types.NetworkQualityChangedEventData) error {
	return m.eventHandlerService.HandleNetworkQualityChanged(ctx, eventData)
}
