// Package aggregator 聚合节点管理器（薄实现层）
//
// 🎯 **聚合器薄实现层**
//
// 本包实现 InternalAggregatorService 接口，集成所有聚合器子组件：
// - 薄实现原则：只做接口方法委托，不包含业务逻辑
// - 组件集成：集成所有子组件，提供完整的聚合服务（当前采用 PoW + XOR 距离选择）
// - 架构一致性：与miner重构模式保持完全一致
package aggregator

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/weisyn/v1/internal/config/consensus"
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
	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/interfaces/block"
	"github.com/weisyn/v1/pkg/interfaces/chain"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	metricsiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/metrics"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	"github.com/weisyn/v1/pkg/interfaces/network"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/types"
)

// Manager 聚合器薄管理层
//
// 🎯 **设计原则**：严格的薄实现层
// - 只实现接口方法委托，所有具体逻辑都在子组件中
// - 集成所有子组件，提供完整的聚合服务能力（统一 Aggregator + 距离选择）
// - 支持网络协议注册和统一路由
type Manager struct {
	// ========== 核心依赖 ==========
	logger       log.Logger           // 日志记录器
	blockService block.BlockProcessor // 区块服务依赖（用于处理共识结果）

	// ========== 节点运行时状态 ==========
	// 使用状态机模型（RuntimeState）进行共识能力 gating（由 P2P 模块管理）
	nodeRuntimeState p2pi.RuntimeState

	// ========== 同步服务（用于更新 RuntimeState 的同步状态）==========
	syncService chain.SystemSyncService // 同步服务（可选，用于在共识检查前更新同步状态）

	// ========== 业务子组件实例 ==========
	controllerService     interfaces.AggregatorController   // 控制器服务
	electionService       interfaces.AggregatorElection     // 选举服务
	networkHandlerService interfaces.NetworkProtocolHandler // 网络处理服务
	candidateCollector    interfaces.CandidateCollector     // 候选收集服务
	decisionCalculator    interfaces.DecisionCalculator     // 基础验证服务
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
	p2pService p2pi.Service,
	network network.Network,
	chainQuery persistence.QueryService,
	distanceCalculator kademlia.DistanceCalculator,
	config *consensus.ConsensusOptions,
	forkHandler chain.ForkHandler, // ✅ P1修复：重命名为 forkHandler，更清晰
	routingTableManager kademlia.RoutingTableManager,
	blockValidator block.BlockValidator,
	blockProcessor block.BlockProcessor,
	// ✅ P1修复：以下参数可选，暂时允许为 nil
	syncService chain.SystemSyncService, // ✅ P1修复：同步服务（可选）
	tempStore storage.TempStore, // ✅ P1修复：临时存储服务（可选）
	blockHashClient core.BlockHashServiceClient, // ✅ P1修复：区块哈希服务客户端（可选）
	configProvider config.Provider, // 配置提供者
	nodeRuntimeState p2pi.RuntimeState, // ✅ Phase 1.2：节点运行时状态（状态机模型，由 P2P 模块管理）
) interfaces.InternalAggregatorService {
	// ✅ Phase 1.2：使用节点运行时状态机，不再使用 NodeRole/策略矩阵
	if nodeRuntimeState == nil {
		if logger != nil {
			logger.Fatal("node runtime state is required (name=\"node_runtime_state\")")
		}
	}

	// 创建所有子组件服务实例
	electionService := election.NewAggregatorElectionService(
		logger,
		chainQuery,
		hashManager,
		distanceCalculator,
		p2pService,
		network,
		routingTableManager,
	)
	candidateCollector := candidate_collector.NewCandidateCollectorService(
		logger,
		candidatePool,
		chainQuery,
		hashManager,
		p2pService,
		powEngine,
		syncService,
		config,
		configProvider,
	)
	decisionCalculator := decision_calculator.NewDecisionCalculatorService(
		logger,
		hashManager,
		p2pService,
		config,
	)
	distanceSelector := distance_selector.New(
		logger,
		hashManager,
	)
	resultDistributor := result_distributor.NewResultDistributorService(
		logger,
		network,
		p2pService,
		config.Aggregator.MinPeerThreshold,
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
		distanceSelector,
		resultDistributor,
		candidatePool,
		network,
		p2pService,
		routingTableManager,
		config,          // 传递配置
		chainQuery,      // 传递统一查询服务（用于获取真实父块哈希等）
		blockHashClient, // 传递区块哈希服务客户端（用于通过统一接口计算区块哈希）
		blockProcessor,  // 传递区块处理服务（用于处理选中的区块）
	)

	// 网络处理器需要控制器来触发聚合流程
	// ✅ P1修复：添加 forkHandler, syncService, tempStore, blockHashClient 参数
	networkHandlerService := network_handler.NewNetworkProtocolHandlerService(
		logger,
		electionService,
		chainQuery,
		candidatePool,
		p2pService,
		network,
		controllerService,
		forkHandler, // ForkHandler
		syncService, // SystemSyncService（可选）
		blockValidator,
		blockProcessor,
		tempStore,           // TempStore（可选）
		blockHashClient,     // BlockHashServiceClient（可选）
		stateManagerService, // AggregatorStateManager（V2 新增）
	)

	// 事件处理器需要状态管理器来处理系统事件
	eventHandlerService := event_handler.NewAggregatorEventHandlerService(
		logger,
		stateManagerService,
	)

	// 创建Manager实例
	return &Manager{
		logger:                logger,
		blockService:          blockProcessor,
		nodeRuntimeState:      nodeRuntimeState,
		syncService:           syncService, // 保存同步服务引用，用于在共识检查前更新状态
		controllerService:     controllerService,
		electionService:       electionService,
		networkHandlerService: networkHandlerService,
		candidateCollector:    candidateCollector,
		decisionCalculator:    decisionCalculator,
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
var _ interfaces.DecisionCalculator = (*Manager)(nil)                       // 6. 基础验证器
var _ interfaces.DistanceSelector = (*Manager)(nil)                         // 7. 距离选择器
var _ interfaces.ResultDistributor = (*Manager)(nil)                        // 9. 结果分发器
var _ interfaces.AggregatorStateManager = (*Manager)(nil)                   // 10. 状态管理器
var _ interfaces.AggregatorEventHandler = (*Manager)(nil)                   // 11. 事件处理器
var _ networkintegration.UnifiedAggregatorSubscribeRouter = (*Manager)(nil) // 网络订阅路由

// ============================================================================
//                           AggregatorController 接口实现（薄委托）
// ============================================================================

// ProcessAggregationRound 处理聚合轮次
func (m *Manager) ProcessAggregationRound(ctx context.Context, candidateBlock *core.Block) error {
	// ✅ 新语义：不再用“是否已完全同步/是否有 peers”等网络观测来硬性阻断聚合流程。
	// - 对于 light 节点：依然禁止进入聚合（无法完整验证/执行）
	// - 对于未 fully synced 的 full/archive/pruned：允许继续本地链路（单节点/孤岛可出块），但应输出告警提示“确认语义降级/重组概率上升”
	if m.nodeRuntimeState != nil {
		snapshot := m.nodeRuntimeState.GetSnapshot()
		if snapshot.SyncMode == p2pi.SyncModeLight {
			return fmt.Errorf("轻节点不能参与聚合流程")
		}
		if !m.nodeRuntimeState.IsConsensusEligible() {
			// 尝试刷新一次同步状态（最佳努力），但不再作为硬门槛
			if m.syncService != nil {
				checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()
				_, _ = m.syncService.CheckSync(checkCtx)
			}
			snapshot = m.nodeRuntimeState.GetSnapshot()
			if m.logger != nil {
				m.logger.Warnf(
					"state.degrade_consensus: 节点不具备网络共识资格，将以本地模式继续聚合/出块（确认语义降级/重组概率上升） (sync_mode=%s, is_fully_synced=%v, is_online=%v)",
					snapshot.SyncMode, snapshot.IsFullySynced, snapshot.IsOnline,
				)
			}
		}
	}

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

// GetAggregatorForHeightWithWaivers 获取指定高度的聚合节点ID（排除弃权节点）
//
// V2 新增：支持弃权与重选机制
func (m *Manager) GetAggregatorForHeightWithWaivers(height uint64, waivedAggregators []peer.ID) (peer.ID, error) {
	return m.electionService.GetAggregatorForHeightWithWaivers(height, waivedAggregators)
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

// HandleAggregatorStatusQuery 处理聚合器状态查询协议（V2 新增）
func (m *Manager) HandleAggregatorStatusQuery(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
	return m.networkHandlerService.HandleAggregatorStatusQuery(ctx, from, reqBytes)
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

// EvaluateAllCandidates 批量验证所有候选区块
func (m *Manager) EvaluateAllCandidates(candidates []types.CandidateBlock) ([]types.CandidateBlock, error) {
	return m.decisionCalculator.EvaluateAllCandidates(candidates)
}

// ValidateCandidate 执行候选区块的基础验证
func (m *Manager) ValidateCandidate(candidate *types.CandidateBlock) (*types.CandidateValidationResult, error) {
	return m.decisionCalculator.ValidateCandidate(candidate)
}

// GetEvaluationStatistics 获取评估统计信息
func (m *Manager) GetEvaluationStatistics() (*types.EvaluationStats, error) {
	return m.decisionCalculator.GetEvaluationStatistics()
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
func (m *Manager) DistributeSelectedBlock(ctx context.Context, selected *types.CandidateBlock, proof *types.DistanceSelectionProof, totalCandidates uint32) error {
	// 🎯 语义修复：聚合节点自身必须“本地应用”最终区块，而不能只依赖 PubSub 广播
	//
	// 背景：
	// - 共识结果通过 PubSub 发布到 TopicConsensusResult；
	// - 其它节点通过 SubscribeTopic 收到广播后，会在 NetworkProtocolHandler 中：
	//   反序列化 → ValidateBlock → BlockProcessor.ProcessBlock → 更新本地链高度；
	// - 但当前实现中，广播消息默认不会“自发自收”，且 NetworkProtocolHandler 还显式跳过 from==self 的消息；
	// - 导致“本节点作为聚合器时，永远收不到自己的广播”，新区块只停留在共识层，无法写入本地链。
	//
	// 修复策略（不向后兼容旧语义）：
	// - 在聚合器选出最终区块后，先通过本地 BlockProcessor.ProcessBlock 直接将 FinalBlock 写入本地链；
	// - 然后再通过 ResultDistributor 将共识结果广播到网络，供其它节点消费；
	// - 对于多节点场景，后续收到来自网络的同一高度/同一哈希的区块时，链模块已有“重复区块幂等处理”逻辑，不会造成重复写入。

	// 1. 本地聚合器自用路径：直接将最终区块提交给区块处理器
	if m.blockService != nil && selected != nil && selected.Block != nil {
		if m.logger != nil && selected.Block.Header != nil {
			m.logger.Infof("🔗 [Aggregator] 本地应用最终区块: height=%d（先写入本地区块链，再广播共识结果）",
				selected.Block.Header.Height)
		}

		if err := m.blockService.ProcessBlock(ctx, selected.Block); err != nil {
			// 这里视为致命错误：本地都无法写入链状态，继续广播只会制造不一致
			if m.logger != nil {
				m.logger.Errorf("❌ [Aggregator] 本地应用最终区块失败: %v", err)
			}
			return fmt.Errorf("aggregator apply final block locally failed: %w", err)
		}

		if m.logger != nil && selected.Block.Header != nil {
			m.logger.Infof("✅ [Aggregator] 本地最终区块已写入链: height=%d", selected.Block.Header.Height)
		}
	} else if m.logger != nil {
		m.logger.Warn("⚠️ [Aggregator] 本地应用最终区块跳过：blockService/selected/block 为空，可能是依赖注入或调用路径异常")
	}

	// 2. 继续按原有语义，将结果广播到网络，让其他节点通过订阅路径更新各自链状态
	return m.resultDistributor.DistributeSelectedBlock(ctx, selected, proof, totalCandidates)
}

// BroadcastToNetwork 网络广播
func (m *Manager) BroadcastToNetwork(ctx context.Context, message *types.DistanceDistributionMessage) error {
	return m.resultDistributor.BroadcastToNetwork(ctx, message)
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

// EnsureState 确保处于目标状态（幂等操作）
func (m *Manager) EnsureState(targetState interfaces.AggregationState) error {
	return m.stateManagerService.EnsureState(targetState)
}

// EnsureIdle 确保处于 Idle 状态的便捷方法
func (m *Manager) EnsureIdle() error {
	return m.stateManagerService.EnsureIdle()
}

// IsValidTransition 验证状态转换
func (m *Manager) IsValidTransition(from, to interfaces.AggregationState) bool {
	return m.stateManagerService.IsValidTransition(from, to)
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

// ============================================================================
// 内存监控接口实现（MemoryReporter）
// ============================================================================

// ModuleName 返回模块名称（实现 MemoryReporter 接口）
func (m *Manager) ModuleName() string {
	return "consensus.aggregator"
}

// CollectMemoryStats 收集 Consensus Aggregator 模块的内存统计信息（实现 MemoryReporter 接口）
//
// 映射规则（根据 memory-standards.md）：
// - Objects: 当前活跃 round / vote 对象数量
// - ApproxBytes: 共识状态（包括暂存的 block header / vote / round state）估算 bytes
// - QueueLength: 共识消息队列长度（待处理消息、待广播块）
func (m *Manager) CollectMemoryStats() metricsiface.ModuleMemoryStats {
	// 统计活跃的聚合任务和候选区块
	// 📌 当前尚未对聚合任务 / 候选区块做精确对象计数，这里避免使用固定“1 个任务、10 个候选区块”的拍脑袋估算。
	objects := int64(0)

	// 📌 暂不对聚合状态做 bytes 级别估算。
	approxBytes := int64(0)

	// 缓存条目 / 队列长度暂不统计，交由其他 metrics 反映
	cacheItems := int64(0)
	queueLength := int64(0)

	return metricsiface.ModuleMemoryStats{
		Module:      "consensus.aggregator",
		Layer:       "L3-Coordination",
		Objects:     objects,
		ApproxBytes: approxBytes,
		CacheItems:  cacheItems,
		QueueLength: queueLength,
	}
}
