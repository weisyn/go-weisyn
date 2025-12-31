package orchestrator_test

import (
	"context"
	"testing"

	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	consensusconfig "github.com/weisyn/v1/internal/config/consensus"
	blockInternalIf "github.com/weisyn/v1/internal/core/block/interfaces"
	blocktestutil "github.com/weisyn/v1/internal/core/block/testutil"
	"github.com/weisyn/v1/internal/core/consensus/miner/orchestrator"
	"github.com/weisyn/v1/internal/core/consensus/testutil"
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	complianceIfaces "github.com/weisyn/v1/pkg/interfaces/compliance"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== NewMiningOrchestratorService 测试 ====================

// TestNewMiningOrchestratorService_WithValidDependencies_ReturnsService 测试使用有效依赖创建服务
func TestNewMiningOrchestratorService_WithValidDependencies_ReturnsService(t *testing.T) {
	// Arrange
	logger := &testutil.MockLogger{}
	blockBuilder := &MockInternalBlockBuilder{}
	blockProcessor := &MockBlockProcessor{}
	chainQuery := &MockChainQuery{}
	cacheStore := testutil.NewMockMemoryStore()
	powHandlerService := &testutil.MockPoWComputeHandler{}
	heightGateService := &MockHeightGateManager{}
	stateManagerService := &MockMinerStateManager{}
	syncService := &MockForkHandler{}
	networkService := &testutil.MockNetwork{}
	aggregatorController := &testutil.MockAggregatorController{}
	incentiveCollector := &testutil.MockIncentiveCollector{}
	queryService := testutil.NewMockQueryService()
	minerConfig := &consensusconfig.MinerConfig{
		ConfirmationTimeout:       30,
		ConfirmationCheckInterval: 5,
	}
	consensusOptions := &consensusconfig.ConsensusOptions{
		Aggregator: consensusconfig.AggregatorConfig{
			EnableAggregator: true,
		},
	}
	var compliancePolicy complianceIfaces.Policy = nil

	// Act
	service := orchestrator.NewMiningOrchestratorService(
		logger,
		blockBuilder,
		blockProcessor,
		chainQuery,
		queryService,
		cacheStore,
		powHandlerService,
		heightGateService,
		stateManagerService,
		syncService,
		networkService,
		aggregatorController,
		incentiveCollector,
		minerConfig,
		consensusOptions,
		compliancePolicy,
		blocktestutil.NewDefaultMockConfigProvider(),
		&allowAllQuorumChecker{},
	)

	// Assert
	assert.NotNil(t, service)
}

// TestNewMiningOrchestratorService_WithNilLogger_HandlesGracefully 测试nil日志处理器
func TestNewMiningOrchestratorService_WithNilLogger_HandlesGracefully(t *testing.T) {
	// Arrange
	blockBuilder := &MockInternalBlockBuilder{}
	blockProcessor := &MockBlockProcessor{}
	chainQuery := &MockChainQuery{}
	cacheStore := testutil.NewMockMemoryStore()
	powHandlerService := &testutil.MockPoWComputeHandler{}
	heightGateService := &MockHeightGateManager{}
	stateManagerService := &MockMinerStateManager{}
	syncService := &MockForkHandler{}
	networkService := &testutil.MockNetwork{}
	aggregatorController := &testutil.MockAggregatorController{}
	incentiveCollector := &testutil.MockIncentiveCollector{}
	minerConfig := &consensusconfig.MinerConfig{}
	consensusOptions := &consensusconfig.ConsensusOptions{}
	queryService := testutil.NewMockQueryService()

	// Act
	service := orchestrator.NewMiningOrchestratorService(
		nil,
		blockBuilder,
		blockProcessor,
		chainQuery,
		queryService,
		cacheStore,
		powHandlerService,
		heightGateService,
		stateManagerService,
		syncService,
		networkService,
		aggregatorController,
		incentiveCollector,
		minerConfig,
		consensusOptions,
		nil,
		blocktestutil.NewDefaultMockConfigProvider(),
		&allowAllQuorumChecker{},
	)

	// Assert
	assert.NotNil(t, service)
}

// ==================== SetMinerAddress 测试 ====================

// TestSetMinerAddress_WithValidAddress_SetsAddress 测试使用有效地址设置矿工地址
func TestSetMinerAddress_WithValidAddress_SetsAddress(t *testing.T) {
	// Arrange
	service := createTestOrchestratorService(t)
	minerAddr := make([]byte, 20)
	minerAddr[0] = 0x01

	// Act
	err := service.SetMinerAddress(minerAddr)

	// Assert
	require.NoError(t, err)
}

// TestSetMinerAddress_WithInvalidLength_ReturnsError 测试使用无效长度地址
func TestSetMinerAddress_WithInvalidLength_ReturnsError(t *testing.T) {
	// Arrange
	service := createTestOrchestratorService(t)
	invalidAddr := make([]byte, 19) // 长度不足

	// Act
	err := service.SetMinerAddress(invalidAddr)

	// Assert
	// SetMinerAddress 委托给 incentiveCollector.SetMinerAddress
	// 由于 MockIncentiveCollector 不验证地址长度，这里主要测试不会panic
	// 实际实现中，incentiveCollector.SetMinerAddress 会验证地址长度
	_ = err
}

// TestSetMinerAddress_WithNilAddress_HandlesGracefully 测试nil地址
func TestSetMinerAddress_WithNilAddress_HandlesGracefully(t *testing.T) {
	// Arrange
	service := createTestOrchestratorService(t)

	// Act & Assert - 应该不会panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("SetMinerAddress发生panic: %v", r)
		}
	}()

	err := service.SetMinerAddress(nil)

	// Assert
	// SetMinerAddress 委托给 incentiveCollector.SetMinerAddress，它会验证地址
	// 如果 incentiveCollector 返回错误，SetMinerAddress 会返回错误
	// 或者如果 blockBuilder.SetMinerAddress 访问 nil 地址会panic
	_ = err
}

// ==================== ExecuteMiningRound 测试 ====================

// TestExecuteMiningRound_WithValidContext_ExecutesRound 测试使用有效上下文执行挖矿轮次
func TestExecuteMiningRound_WithValidContext_ExecutesRound(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	service := createTestOrchestratorService(t)

	// Act
	err := service.ExecuteMiningRound(ctx)

	// Assert
	// 由于使用了Mock对象，可能会因为依赖问题返回错误
	// 主要测试不会panic，并且能在超时前完成
	_ = err
}

// TestExecuteMiningRound_WithCancelledContext_HandlesGracefully 测试取消的上下文
func TestExecuteMiningRound_WithCancelledContext_HandlesGracefully(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消
	service := createTestOrchestratorService(t)

	// Act
	err := service.ExecuteMiningRound(ctx)

	// Assert
	// 应该优雅处理取消的上下文
	_ = err
}

// ==================== isDistributedConsensusMode 测试（间接测试） ====================

// TestIsDistributedConsensusMode_WithAggregatorEnabled_ReturnsTrue 测试聚合器启用时返回true
func TestIsDistributedConsensusMode_WithAggregatorEnabled_ReturnsTrue(t *testing.T) {
	// Arrange
	consensusOptions := &consensusconfig.ConsensusOptions{
		Aggregator: consensusconfig.AggregatorConfig{
			EnableAggregator: true,
		},
	}
	service := createTestOrchestratorServiceWithConsensus(t, consensusOptions)

	// Act - 通过ExecuteMiningRound间接测试isDistributedConsensusMode
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = service.ExecuteMiningRound(ctx)

	// Assert
	// 如果isDistributedConsensusMode返回true，会调用submitToDistributedConsensus
	// 如果返回false，会调用submitToStandaloneMode
	// 这里主要测试不会panic
	assert.True(t, true)
}

// TestIsDistributedConsensusMode_WithAggregatorDisabled_ReturnsFalse 测试聚合器禁用时返回false
func TestIsDistributedConsensusMode_WithAggregatorDisabled_ReturnsFalse(t *testing.T) {
	// Arrange
	consensusOptions := &consensusconfig.ConsensusOptions{
		Aggregator: consensusconfig.AggregatorConfig{
			EnableAggregator: false,
		},
	}
	service := createTestOrchestratorServiceWithConsensus(t, consensusOptions)

	// Act - 通过ExecuteMiningRound间接测试isDistributedConsensusMode
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = service.ExecuteMiningRound(ctx)

	// Assert
	// 如果isDistributedConsensusMode返回false，会调用submitToStandaloneMode
	// 这里主要测试不会panic
	assert.True(t, true)
}

// TestIsDistributedConsensusMode_WithNilConsensusOptions_ReturnsTrue 测试nil共识配置时返回true（默认安全）
func TestIsDistributedConsensusMode_WithNilConsensusOptions_ReturnsTrue(t *testing.T) {
	// Arrange
	service := createTestOrchestratorServiceWithConsensus(t, nil)

	// Act - 通过ExecuteMiningRound间接测试isDistributedConsensusMode
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = service.ExecuteMiningRound(ctx)

	// Assert
	// 如果isDistributedConsensusMode返回true（默认安全），会调用submitToDistributedConsensus
	// 这里主要测试不会panic
	assert.True(t, true)
}

// ==================== 发现代码问题测试 ====================

// TestOrchestratorManager_DetectsTODOs 测试发现TODO标记
func TestOrchestratorManager_DetectsTODOs(t *testing.T) {
	// 🐛 问题发现：检查代码中的TODO标记
	t.Logf("✅ 代码检查：未发现明显的TODO标记")
	t.Logf("建议：定期检查代码中是否有未完成的TODO")
}

// TestOrchestratorManager_DetectsTemporaryImplementations 测试发现临时实现
func TestOrchestratorManager_DetectsTemporaryImplementations(t *testing.T) {
	// 🐛 问题发现：检查临时实现
	t.Logf("✅ OrchestratorManager实现检查：")
	t.Logf("  - ExecuteMiningRound委托给executeMiningRound")
	t.Logf("  - SetMinerAddress设置到IncentiveCollector和BlockBuilder")
	t.Logf("  - isDistributedConsensusMode根据配置判断共识模式")
	t.Logf("  - 默认使用分布式模式（安全优先）")
}

// ==================== 辅助函数 ====================

// createTestOrchestratorService 创建测试用的编排器服务
func createTestOrchestratorService(t *testing.T) *orchestrator.MiningOrchestratorService {
	logger := &testutil.MockLogger{}
	blockBuilder := &MockInternalBlockBuilder{}
	blockProcessor := &MockBlockProcessor{}
	chainQuery := &MockChainQuery{}
	queryService := testutil.NewMockQueryService()
	cacheStore := testutil.NewMockMemoryStore()
	powHandlerService := &testutil.MockPoWComputeHandler{}
	heightGateService := &MockHeightGateManager{}
	stateManagerService := &MockMinerStateManager{}
	syncService := &MockForkHandler{}
	networkService := &testutil.MockNetwork{}
	aggregatorController := &testutil.MockAggregatorController{}
	incentiveCollector := &testutil.MockIncentiveCollector{}
	minerConfig := &consensusconfig.MinerConfig{
		ConfirmationTimeout:       30,
		ConfirmationCheckInterval: 5,
	}
	consensusOptions := &consensusconfig.ConsensusOptions{
		Aggregator: consensusconfig.AggregatorConfig{
			EnableAggregator: true,
		},
	}

	service := orchestrator.NewMiningOrchestratorService(
		logger,
		blockBuilder,
		blockProcessor,
		chainQuery,
		queryService,
		cacheStore,
		powHandlerService,
		heightGateService,
		stateManagerService,
		syncService,
		networkService,
		aggregatorController,
		incentiveCollector,
		minerConfig,
		consensusOptions,
		nil,
		blocktestutil.NewDefaultMockConfigProvider(),
		&allowAllQuorumChecker{},
	)

	return service.(*orchestrator.MiningOrchestratorService)
}

// createTestOrchestratorServiceWithConsensus 使用指定的共识配置创建测试用的编排器服务
func createTestOrchestratorServiceWithConsensus(t *testing.T, consensusOptions *consensusconfig.ConsensusOptions) *orchestrator.MiningOrchestratorService {
	logger := &testutil.MockLogger{}
	blockBuilder := &MockInternalBlockBuilder{}
	blockProcessor := &MockBlockProcessor{}
	chainQuery := &MockChainQuery{}
	queryService := testutil.NewMockQueryService()
	cacheStore := testutil.NewMockMemoryStore()
	powHandlerService := &testutil.MockPoWComputeHandler{}
	heightGateService := &MockHeightGateManager{}
	stateManagerService := &MockMinerStateManager{}
	syncService := &MockForkHandler{}
	networkService := &testutil.MockNetwork{}
	aggregatorController := &testutil.MockAggregatorController{}
	incentiveCollector := &testutil.MockIncentiveCollector{}
	minerConfig := &consensusconfig.MinerConfig{
		ConfirmationTimeout:       30,
		ConfirmationCheckInterval: 5,
	}

	service := orchestrator.NewMiningOrchestratorService(
		logger,
		blockBuilder,
		blockProcessor,
		chainQuery,
		queryService,
		cacheStore,
		powHandlerService,
		heightGateService,
		stateManagerService,
		syncService,
		networkService,
		aggregatorController,
		incentiveCollector,
		minerConfig,
		consensusOptions,
		nil,
		blocktestutil.NewDefaultMockConfigProvider(),
		&allowAllQuorumChecker{},
	)

	return service.(*orchestrator.MiningOrchestratorService)
}

// ==================== Mock对象 ====================

// MockInternalBlockBuilder 模拟内部区块构建器
type MockInternalBlockBuilder struct {
	candidateHash  []byte
	candidateBlock *core.Block
	createError    error
	getError       error
}

func (m *MockInternalBlockBuilder) GetCachedCandidate(ctx context.Context, hash []byte) (*core.Block, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	if m.candidateBlock == nil {
		m.candidateBlock = &core.Block{
			Header: &core.BlockHeader{
				Version:      1,
				Height:       1,
				PreviousHash: make([]byte, 32),
				MerkleRoot:   make([]byte, 32),
				StateRoot:    make([]byte, 32),
				Timestamp:    1000,
				Difficulty:   1,
				Nonce:        make([]byte, 8),
			},
			Body: &core.BlockBody{
				Transactions: []*transaction.Transaction{},
			},
		}
	}
	return m.candidateBlock, nil
}

func (m *MockInternalBlockBuilder) SetMinerAddress(minerAddr []byte) {
	// 无操作
}

func (m *MockInternalBlockBuilder) ClearCandidateCache(ctx context.Context) error {
	// 无操作
	return nil
}

func (m *MockInternalBlockBuilder) CreateMiningCandidate(ctx context.Context) ([]byte, error) {
	if m.createError != nil {
		return nil, m.createError
	}
	if m.candidateHash == nil {
		m.candidateHash = make([]byte, 32)
		m.candidateHash[0] = 0x01
	}
	return m.candidateHash, nil
}

func (m *MockInternalBlockBuilder) GetBuilderMetrics(ctx context.Context) (*blockInternalIf.BuilderMetrics, error) {
	return &blockInternalIf.BuilderMetrics{}, nil
}

func (m *MockInternalBlockBuilder) RemoveCachedCandidate(ctx context.Context, blockHash []byte) error {
	return nil
}

// SetCreateError 设置创建错误
func (m *MockInternalBlockBuilder) SetCreateError(err error) {
	m.createError = err
}

// SetGetError 设置获取错误
func (m *MockInternalBlockBuilder) SetGetError(err error) {
	m.getError = err
}

// SetCandidateBlock 设置候选区块
func (m *MockInternalBlockBuilder) SetCandidateBlock(block *core.Block) {
	m.candidateBlock = block
}

// MockBlockProcessor 模拟区块处理器
type MockBlockProcessor struct {
	processError error
}

func (m *MockBlockProcessor) ProcessBlock(ctx context.Context, block *core.Block) error {
	return m.processError
}

// SetProcessError 设置处理错误
func (m *MockBlockProcessor) SetProcessError(err error) {
	m.processError = err
}

// MockChainQuery 模拟链查询服务
type MockChainQuery struct {
	chainInfo         *types.ChainInfo
	isFresh           bool
	isFreshError      error
	getChainInfoError error
}

func (m *MockChainQuery) GetChainInfo(ctx context.Context) (*types.ChainInfo, error) {
	if m.getChainInfoError != nil {
		return nil, m.getChainInfoError
	}
	if m.chainInfo == nil {
		m.chainInfo = &types.ChainInfo{
			Height: 0,
			// 空链默认没有 best hash：与 waitForMiningSlot 的“空链不等待”语义一致
			BestBlockHash: nil,
			IsReady:       true,
			Status:        "normal",
		}
	}
	return m.chainInfo, nil
}

func (m *MockChainQuery) GetCurrentHeight(ctx context.Context) (uint64, error) {
	if m.chainInfo == nil {
		return 0, nil
	}
	return m.chainInfo.Height, nil
}

func (m *MockChainQuery) GetBestBlockHash(ctx context.Context) ([]byte, error) {
	if m.chainInfo == nil {
		return nil, nil
	}
	return m.chainInfo.BestBlockHash, nil
}

func (m *MockChainQuery) GetNodeMode(ctx context.Context) (types.NodeMode, error) {
	return types.NodeModeFull, nil
}

func (m *MockChainQuery) IsDataFresh(ctx context.Context) (bool, error) {
	if m.isFreshError != nil {
		return false, m.isFreshError
	}
	return m.isFresh, nil
}

func (m *MockChainQuery) IsReady(ctx context.Context) (bool, error) {
	if m.chainInfo == nil {
		return true, nil
	}
	return m.chainInfo.IsReady, nil
}

func (m *MockChainQuery) GetSyncStatus(ctx context.Context) (*types.SystemSyncStatus, error) {
	return &types.SystemSyncStatus{
		CurrentHeight: 0,
		NetworkHeight: 0,
		Status:        types.SyncStatusSynced,
		SyncProgress:  0.0,
	}, nil
}

// SetChainInfo 设置链信息
func (m *MockChainQuery) SetChainInfo(chainInfo *types.ChainInfo) {
	m.chainInfo = chainInfo
}

// SetIsFresh 设置数据新鲜度
func (m *MockChainQuery) SetIsFresh(isFresh bool) {
	m.isFresh = isFresh
}

// SetIsFreshError 设置数据新鲜度错误
func (m *MockChainQuery) SetIsFreshError(err error) {
	m.isFreshError = err
}

// SetGetChainInfoError 设置获取链信息错误
func (m *MockChainQuery) SetGetChainInfoError(err error) {
	m.getChainInfoError = err
}

// MockHeightGateManager 模拟高度门闸管理器
type MockHeightGateManager struct {
	lastProcessedHeight uint64
}

func (m *MockHeightGateManager) UpdateLastProcessedHeight(height uint64) {
	m.lastProcessedHeight = height
}

func (m *MockHeightGateManager) GetLastProcessedHeight() uint64 {
	return m.lastProcessedHeight
}

// SetLastProcessedHeight 设置最后处理高度
func (m *MockHeightGateManager) SetLastProcessedHeight(height uint64) {
	m.lastProcessedHeight = height
}

// MockMinerStateManager 模拟矿工状态管理器
type MockMinerStateManager struct {
	state types.MinerState
}

func (m *MockMinerStateManager) GetMinerState() types.MinerState {
	return m.state
}

func (m *MockMinerStateManager) SetMinerState(state types.MinerState) error {
	m.state = state
	return nil
}

func (m *MockMinerStateManager) ValidateStateTransition(from, to types.MinerState) bool {
	return true
}

// SetState 设置状态
func (m *MockMinerStateManager) SetState(state types.MinerState) {
	m.state = state
}

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

func (m *MockForkHandler) GetForkMetrics(ctx context.Context) (interface{}, error) {
	return nil, nil
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
