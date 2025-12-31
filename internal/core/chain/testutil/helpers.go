package testutil

import (
	"github.com/weisyn/v1/internal/core/block/testutil"
	"github.com/weisyn/v1/internal/core/chain/fork"
	"github.com/weisyn/v1/internal/core/chain/sync"
	_ "github.com/weisyn/v1/internal/core/infrastructure/writegate" // 导入实现包以触发 init()
)

// NewTestForkHandler 创建测试用的 ForkHandler
func NewTestForkHandler() (*fork.Service, error) {
	queryService := testutil.NewMockQueryService()
	hashManager := &testutil.MockHashManager{}
	blockHashClient := testutil.NewMockBlockHashClient()
	txHashClient := testutil.NewMockTransactionHashClient()
	configProvider := &MockConfigProvider{}
	eventBus := testutil.NewMockEventBus()
	logger := &testutil.MockLogger{}

	service, err := fork.NewService(
		queryService,
		hashManager,
		blockHashClient,
		txHashClient,
		nil, // store（可选）
		configProvider,
		eventBus,
		logger,
	)
	if err != nil {
		return nil, err
	}

	return service.(*fork.Service), nil
}

// NewTestSyncManager 创建测试用的 SyncManager
func NewTestSyncManager() (*sync.Manager, error) {
	queryService := testutil.NewMockQueryService()
	blockValidator := testutil.NewMockBlockValidator()
	blockProcessor, err := testutil.NewTestBlockProcessor()
	if err != nil {
		return nil, err
	}
	blockHashClient := testutil.NewMockBlockHashClient()
	networkService := NewMockNetwork()
	kBucketManager := NewMockRoutingTableManager()
	p2pService := NewMockP2PService()
	configProvider := &MockConfigProvider{}
	tempStore := NewMockTempStore()
	runtimeState := NewMockRuntimeState()
	logger := &testutil.MockLogger{}
	eventBus := testutil.NewMockEventBus()

	manager := sync.NewManager(
		queryService,
		blockValidator,
		blockProcessor,
		queryService,
		networkService,
		kBucketManager,
		p2pService,
		configProvider,
		tempStore,
		runtimeState,
		blockHashClient,
		nil, // forkHandler
		nil, // recoveryMgr
		logger,
		eventBus,
	)

	return manager.(*sync.Manager), nil
}
