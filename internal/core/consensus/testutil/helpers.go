package testutil

import (
	"context"
	"time"

	consensusconfig "github.com/weisyn/v1/internal/config/consensus"
	blocktestutil "github.com/weisyn/v1/internal/core/block/testutil"
	"github.com/weisyn/v1/internal/core/consensus/miner"
	"github.com/weisyn/v1/internal/core/consensus/miner/controller"
	"github.com/weisyn/v1/internal/core/consensus/miner/pow_handler"
	"github.com/weisyn/v1/internal/core/consensus/miner/state_manager"
	_ "github.com/weisyn/v1/internal/core/infrastructure/writegate" // 导入实现包以触发 init()
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	chainiface "github.com/weisyn/v1/pkg/interfaces/chain"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== 辅助函数 ====================

// NewTestConsensusOptions 创建测试用的共识配置
func NewTestConsensusOptions() *consensusconfig.ConsensusOptions {
	opts := consensusconfig.New(nil).GetOptions()
	// 使用秒级时间，避免纳秒级默认值导致测试行为与生产配置偏差
	opts.Miner.MiningTimeout = 30 * time.Second
	opts.Miner.LoopInterval = 1 * time.Second
	opts.Miner.MaxTransactions = 100
	opts.Miner.MinTransactions = 1
	opts.Miner.MaxForkDepth = 100
	opts.Miner.TxSelectionMode = "fee"
	// 让 v2 难度参数在测试环境也可用（避免 0 值）
	if opts.TargetBlockTime == 0 {
		opts.TargetBlockTime = 10 * time.Second
	}
	if opts.POW.DifficultyWindow == 0 {
		opts.POW.DifficultyWindow = 10
	}
	if opts.POW.MaxAdjustUpPPM == 0 {
		opts.POW.MaxAdjustUpPPM = 4_000_000
	}
	if opts.POW.MaxAdjustDownPPM == 0 {
		opts.POW.MaxAdjustDownPPM = 250_000
	}
	if opts.POW.MTPWindow == 0 {
		opts.POW.MTPWindow = 11
	}
	return opts
}

// NewTestMinerManager 创建测试用的矿工管理器
func NewTestMinerManager() (*miner.Manager, error) {
	logger := &MockLogger{}
	eventBus := NewMockEventBus()
	consensusOptions := NewTestConsensusOptions()

	// 创建依赖
	blockBuilder, err := blocktestutil.NewTestBlockBuilder()
	if err != nil {
		return nil, err
	}

	blockProcessor, err := blocktestutil.NewTestBlockProcessor()
	if err != nil {
		return nil, err
	}

	chainQuery := blocktestutil.NewMockQueryService()
	queryService := chainQuery
	syncService := &MockSystemSyncService{}
	cacheStore := NewMockMemoryStore()
	networkService := &MockNetwork{}
	var p2pService p2pi.Service = nil
	var routingManager kademlia.RoutingTableManager = nil
	powEngine := NewMockPOWEngine()
	hashManager := &MockHashManager{}
	merkleTreeManager := &MockMerkleTreeManager{}
	txHashClient := NewMockTransactionHashClient()
	aggregatorController := &MockAggregatorController{}
	incentiveCollector := &MockIncentiveCollector{}

	// blockBuilder已经是InternalBlockBuilder类型
	internalBlockBuilder := blockBuilder
	cfg := blocktestutil.NewDefaultMockConfigProvider()

	manager := miner.NewManager(
		logger,
		eventBus,
		consensusOptions,
		internalBlockBuilder,
		blockProcessor,
		chainQuery,
		queryService,
		syncService,
		cacheStore,
		networkService,
		p2pService,
		routingManager,
		powEngine,
		hashManager,
		merkleTreeManager,
		txHashClient,
		aggregatorController,
		incentiveCollector,
		nil, // compliancePolicy
		cfg, // configProvider
	)
	return manager.(*miner.Manager), nil
}

// NewTestMinerController 创建测试用的矿工控制器
func NewTestMinerController() (*controller.MinerControllerService, error) {
	logger := &MockLogger{}
	eventBus := NewMockEventBus()
	orchestratorService := &MockMiningOrchestrator{}
	stateManagerService := state_manager.NewMinerStateService(logger)
	chainQuery := blocktestutil.NewMockQueryService()
	powHandlerService := pow_handler.NewPoWComputeService(
		logger,
		NewMockPOWEngine(),
		&MockHashManager{},
		&MockMerkleTreeManager{},
		NewMockTransactionHashClient(),
	)
	minerConfig := &consensusconfig.MinerConfig{
		MiningTimeout:   30,
		LoopInterval:    1,
		MaxTransactions: 100,
		MinTransactions: 1,
		MaxForkDepth:    100,
		TxSelectionMode: "fee",
	}

	controllerService := controller.NewMinerControllerService(
		logger,
		eventBus,
		chainQuery,
		orchestratorService,
		stateManagerService,
		powHandlerService,
		minerConfig,
		nil, // quorumChecker（测试用：不做 v2 门闸强制）
	)
	return controllerService.(*controller.MinerControllerService), nil
}

// ==================== Mock 接口实现 ====================

// MockForkHandler 模拟分叉处理器
type MockForkHandler struct{}

func (m *MockForkHandler) HandleFork(ctx context.Context, block *core.Block) error {
	return nil
}

func (m *MockForkHandler) GetActiveChain(ctx context.Context) (*types.ChainInfo, error) {
	return &types.ChainInfo{
		Height:        0,
		BestBlockHash: make([]byte, 32),
		IsReady:       true,
		Status:        "normal",
	}, nil
}

func (m *MockForkHandler) DetectFork(ctx context.Context, block *core.Block) (bool, uint64, error) {
	return false, 0, nil
}

// MockSystemSyncService implements chain.SystemSyncService for tests.
type MockSystemSyncService struct{}

var _ chainiface.SystemSyncService = (*MockSystemSyncService)(nil)

func (m *MockSystemSyncService) TriggerSync(ctx context.Context) error { return nil }
func (m *MockSystemSyncService) CancelSync(ctx context.Context) error  { return nil }
func (m *MockSystemSyncService) CheckSync(ctx context.Context) (*types.SystemSyncStatus, error) {
	return &types.SystemSyncStatus{
		Status:        types.SyncStatusSynced,
		CurrentHeight: 0,
		NetworkHeight: 0,
	}, nil
}

func (m *MockForkHandler) GetForkMetrics(ctx context.Context) (interface{}, error) {
	return map[string]interface{}{
		"TotalForks": uint64(0),
	}, nil
}

func (m *MockForkHandler) CalculateChainWeight(ctx context.Context, fromHeight, toHeight uint64) (*types.ChainWeight, error) {
	return &types.ChainWeight{
		BlockCount: 0,
	}, nil
}

// SystemSyncService 方法（MockForkHandler 也实现 SystemSyncService）
func (m *MockForkHandler) TriggerSync(ctx context.Context) error {
	return nil
}

func (m *MockForkHandler) CancelSync(ctx context.Context) error {
	return nil
}

func (m *MockForkHandler) CheckSync(ctx context.Context) (*types.SystemSyncStatus, error) {
	return &types.SystemSyncStatus{
		Status: types.SyncStatusIdle,
	}, nil
}

// MockAggregatorController 模拟聚合器控制器
type MockAggregatorController struct {
	processError error
}

func (m *MockAggregatorController) ProcessAggregationRound(ctx context.Context, candidateBlock *core.Block) error {
	if m.processError != nil {
		return m.processError
	}
	return nil
}

func (m *MockAggregatorController) StartAggregatorService(ctx context.Context) error {
	return nil
}

func (m *MockAggregatorController) StopAggregatorService(ctx context.Context) error {
	return nil
}

// SetProcessError 设置处理错误
func (m *MockAggregatorController) SetProcessError(err error) {
	m.processError = err
}

// MockIncentiveCollector 模拟激励收集器
type MockIncentiveCollector struct{}

func (m *MockIncentiveCollector) CollectIncentives(ctx context.Context, block *core.Block) (*types.AggregatedFees, error) {
	return &types.AggregatedFees{}, nil
}

func (m *MockIncentiveCollector) CollectIncentiveTxs(ctx context.Context, candidateTxs []*transaction.Transaction, blockHeight uint64) ([]*transaction.Transaction, error) {
	return nil, nil
}

func (m *MockIncentiveCollector) SetMinerAddress(minerAddr []byte) error {
	return nil
}

// MockMiningOrchestrator 模拟挖矿编排器
type MockMiningOrchestrator struct{}

func (m *MockMiningOrchestrator) SetMinerAddress(minerAddr []byte) error {
	return nil
}

func (m *MockMiningOrchestrator) CheckMiningGate(ctx context.Context) error {
	return nil
}

func (m *MockMiningOrchestrator) ExecuteMiningRound(ctx context.Context) error {
	return nil
}
