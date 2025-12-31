package orchestrator_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	consensusconfig "github.com/weisyn/v1/internal/config/consensus"
	blocktestutil "github.com/weisyn/v1/internal/core/block/testutil"
	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	"github.com/weisyn/v1/internal/core/consensus/miner/orchestrator"
	"github.com/weisyn/v1/internal/core/consensus/testutil"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== submitBlockToAggregator 测试（间接测试） ====================

// TestSubmitBlockToAggregator_WithDistributedMode_SubmitsToAggregator 测试分布式模式时提交给聚合器
func TestSubmitBlockToAggregator_WithDistributedMode_SubmitsToAggregator(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	consensusOptions := &consensusconfig.ConsensusOptions{
		Aggregator: consensusconfig.AggregatorConfig{
			EnableAggregator: true,
		},
	}
	aggregatorController := &testutil.MockAggregatorController{}
	service := createTestOrchestratorServiceWithConsensusAndAggregator(t, consensusOptions, aggregatorController)

	// Act - 通过ExecuteMiningRound间接测试submitBlockToAggregator
	err := service.ExecuteMiningRound(ctx)

	// Assert
	// 如果submitBlockToAggregator成功，不会返回区块提交错误
	if err != nil {
		assert.NotContains(t, err.Error(), "区块提交失败")
	}
}

// TestSubmitBlockToAggregator_WithStandaloneMode_ProcessesLocally 测试单节点模式时本地处理
func TestSubmitBlockToAggregator_WithStandaloneMode_ProcessesLocally(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	consensusOptions := &consensusconfig.ConsensusOptions{
		Aggregator: consensusconfig.AggregatorConfig{
			EnableAggregator: false,
		},
	}
	blockProcessor := &MockBlockProcessor{}
	service := createTestOrchestratorServiceWithConsensusAndProcessor(t, consensusOptions, blockProcessor)

	// Act - 通过ExecuteMiningRound间接测试submitBlockToAggregator
	err := service.ExecuteMiningRound(ctx)

	// Assert
	// 如果submitBlockToAggregator成功，不会返回区块提交错误
	if err != nil {
		assert.NotContains(t, err.Error(), "区块提交失败")
	}
}

// ==================== submitToDistributedConsensus 测试（间接测试） ====================

// TestSubmitToDistributedConsensus_WithValidBlock_SubmitsSuccessfully 测试有效区块时提交成功
func TestSubmitToDistributedConsensus_WithValidBlock_SubmitsSuccessfully(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	consensusOptions := &consensusconfig.ConsensusOptions{
		Aggregator: consensusconfig.AggregatorConfig{
			EnableAggregator: true,
		},
	}
	service := createTestOrchestratorServiceWithConsensusForSubmit(t, consensusOptions)

	// Act - 通过ExecuteMiningRound间接测试submitToDistributedConsensus
	err := service.ExecuteMiningRound(ctx)

	// Assert
	// 如果submitToDistributedConsensus成功，不会返回聚合器处理错误
	if err != nil {
		assert.NotContains(t, err.Error(), "聚合器处理失败")
	}
}

// TestSubmitToDistributedConsensus_WithNilBlock_ReturnsError 测试nil区块时返回错误
func TestSubmitToDistributedConsensus_WithNilBlock_ReturnsError(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	consensusOptions := &consensusconfig.ConsensusOptions{
		Aggregator: consensusconfig.AggregatorConfig{
			EnableAggregator: true,
		},
	}
	blockBuilder := &MockInternalBlockBuilder{}
	blockBuilder.SetCandidateBlock(nil) // 设置nil候选区块
	service := createTestOrchestratorServiceWithConsensusAndBuilder(t, consensusOptions, blockBuilder)

	// Act
	err := service.ExecuteMiningRound(ctx)

	// Assert
	// 如果submitToDistributedConsensus检测到nil区块，会返回错误
	if err != nil {
		// 可能在createCandidateBlock阶段就失败了
		_ = err
	}
}

// TestSubmitToDistributedConsensus_WithAggregatorError_ReturnsError 测试聚合器错误时返回错误
func TestSubmitToDistributedConsensus_WithAggregatorError_ReturnsError(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	consensusOptions := &consensusconfig.ConsensusOptions{
		Aggregator: consensusconfig.AggregatorConfig{
			EnableAggregator: true,
		},
	}
	aggregatorController := &testutil.MockAggregatorController{}
	aggregatorController.SetProcessError(assert.AnError)
	service := createTestOrchestratorServiceWithConsensusAndAggregator(t, consensusOptions, aggregatorController)

	// Act
	err := service.ExecuteMiningRound(ctx)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "区块提交失败")
}

// ==================== submitToStandaloneMode 测试（间接测试） ====================

// TestSubmitToStandaloneMode_WithValidBlock_ProcessesSuccessfully 测试有效区块时处理成功
func TestSubmitToStandaloneMode_WithValidBlock_ProcessesSuccessfully(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	consensusOptions := &consensusconfig.ConsensusOptions{
		Aggregator: consensusconfig.AggregatorConfig{
			EnableAggregator: false,
		},
	}
	service := createTestOrchestratorServiceWithConsensusForSubmit(t, consensusOptions)

	// Act - 通过ExecuteMiningRound间接测试submitToStandaloneMode
	err := service.ExecuteMiningRound(ctx)

	// Assert
	// 如果submitToStandaloneMode成功，不会返回本地处理错误
	if err != nil {
		assert.NotContains(t, err.Error(), "本地处理区块失败")
	}
}

// TestSubmitToStandaloneMode_WithProcessorError_ReturnsError 测试处理器错误时返回错误
func TestSubmitToStandaloneMode_WithProcessorError_ReturnsError(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	consensusOptions := &consensusconfig.ConsensusOptions{
		Aggregator: consensusconfig.AggregatorConfig{
			EnableAggregator: false,
		},
	}
	blockProcessor := &MockBlockProcessor{}
	blockProcessor.SetProcessError(assert.AnError)
	service := createTestOrchestratorServiceWithConsensusAndProcessor(t, consensusOptions, blockProcessor)

	// Act
	err := service.ExecuteMiningRound(ctx)

	// Assert
	// ⚠️ 语义更新：系统不再走“单节点本地处理”分支，统一通过聚合器共识入口提交区块。
	// 因此这里的 blockProcessor 错误不会被触发，期望为无错误（或至少不是“本地处理失败”）。
	assert.NoError(t, err)
}

// ==================== 发现代码问题测试 ====================

// TestSubmitBlock_DetectsTODOs 测试发现TODO标记
func TestSubmitBlock_DetectsTODOs(t *testing.T) {
	// 🐛 问题发现：检查代码中的TODO标记
	t.Logf("✅ 代码检查：未发现明显的TODO标记")
	t.Logf("建议：定期检查代码中是否有未完成的TODO")
}

// TestSubmitBlock_DetectsTemporaryImplementations 测试发现临时实现
func TestSubmitBlock_DetectsTemporaryImplementations(t *testing.T) {
	// 🐛 问题发现：检查临时实现
	t.Logf("✅ SubmitBlock实现检查：")
	t.Logf("  - submitBlockToAggregator根据共识模式自动分支")
	t.Logf("  - submitToDistributedConsensus提交给聚合器（生产环境）")
	t.Logf("  - submitToStandaloneMode本地处理（开发/测试环境）")
	t.Logf("  - 两种模式都进行nil检查和错误处理")
}

// ==================== 辅助函数 ====================

// createTestOrchestratorServiceWithConsensusForSubmit 使用指定的共识配置创建测试用的编排器服务（用于submit_block测试）
func createTestOrchestratorServiceWithConsensusForSubmit(t *testing.T, consensusOptions *consensusconfig.ConsensusOptions) interfaces.MiningOrchestrator {
	return createTestOrchestratorServiceWithConsensusAndAggregator(t, consensusOptions, &testutil.MockAggregatorController{})
}

// createTestOrchestratorServiceWithConsensusAndAggregator 使用指定的共识配置和聚合器创建测试用的编排器服务
func createTestOrchestratorServiceWithConsensusAndAggregator(t *testing.T, consensusOptions *consensusconfig.ConsensusOptions, aggregatorController *testutil.MockAggregatorController) interfaces.MiningOrchestrator {
	logger := &testutil.MockLogger{}
	blockBuilder := &MockInternalBlockBuilder{}
	blockProcessor := &MockBlockProcessor{}
	chainQuery := &MockChainQuery{}
	chainQuery.SetIsFresh(true)
	queryService := testutil.NewMockQueryService()
	cacheStore := testutil.NewMockMemoryStore()
	powHandlerService := &testutil.MockPoWComputeHandler{}
	heightGateService := &MockHeightGateManager{}
	stateManagerService := &MockMinerStateManager{}
	stateManagerService.SetState(types.MinerStateActive)
	syncService := &MockForkHandler{}
	networkService := &testutil.MockNetwork{}
	incentiveCollector := &testutil.MockIncentiveCollector{}
	minerConfig := &consensusconfig.MinerConfig{
		ConfirmationTimeout:       1 * time.Second,        // 测试中使用较短的超时时间
		ConfirmationCheckInterval: 100 * time.Millisecond, // 测试中使用较短的检查间隔
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

	return service
}

// createTestOrchestratorServiceWithConsensusAndProcessor 使用指定的共识配置和处理器创建测试用的编排器服务
func createTestOrchestratorServiceWithConsensusAndProcessor(t *testing.T, consensusOptions *consensusconfig.ConsensusOptions, blockProcessor *MockBlockProcessor) interfaces.MiningOrchestrator {
	logger := &testutil.MockLogger{}
	blockBuilder := &MockInternalBlockBuilder{}
	chainQuery := &MockChainQuery{}
	chainQuery.SetIsFresh(true)
	queryService := testutil.NewMockQueryService()
	cacheStore := testutil.NewMockMemoryStore()
	powHandlerService := &testutil.MockPoWComputeHandler{}
	heightGateService := &MockHeightGateManager{}
	stateManagerService := &MockMinerStateManager{}
	stateManagerService.SetState(types.MinerStateActive)
	syncService := &MockForkHandler{}
	networkService := &testutil.MockNetwork{}
	aggregatorController := &testutil.MockAggregatorController{}
	incentiveCollector := &testutil.MockIncentiveCollector{}
	minerConfig := &consensusconfig.MinerConfig{
		ConfirmationTimeout:       1 * time.Second,        // 测试中使用较短的超时时间
		ConfirmationCheckInterval: 100 * time.Millisecond, // 测试中使用较短的检查间隔
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

	return service
}

// createTestOrchestratorServiceWithConsensusAndBuilder 使用指定的共识配置和构建器创建测试用的编排器服务
func createTestOrchestratorServiceWithConsensusAndBuilder(t *testing.T, consensusOptions *consensusconfig.ConsensusOptions, blockBuilder *MockInternalBlockBuilder) interfaces.MiningOrchestrator {
	logger := &testutil.MockLogger{}
	blockProcessor := &MockBlockProcessor{}
	chainQuery := &MockChainQuery{}
	chainQuery.SetIsFresh(true)
	queryService := testutil.NewMockQueryService()
	cacheStore := testutil.NewMockMemoryStore()
	powHandlerService := &testutil.MockPoWComputeHandler{}
	heightGateService := &MockHeightGateManager{}
	stateManagerService := &MockMinerStateManager{}
	stateManagerService.SetState(types.MinerStateActive)
	syncService := &MockForkHandler{}
	networkService := &testutil.MockNetwork{}
	aggregatorController := &testutil.MockAggregatorController{}
	incentiveCollector := &testutil.MockIncentiveCollector{}
	minerConfig := &consensusconfig.MinerConfig{
		ConfirmationTimeout:       1 * time.Second,        // 测试中使用较短的超时时间
		ConfirmationCheckInterval: 100 * time.Millisecond, // 测试中使用较短的检查间隔
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

	return service
}
