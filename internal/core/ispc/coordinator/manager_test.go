package coordinator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ctxmgr "github.com/weisyn/v1/internal/core/ispc/context"
	"github.com/weisyn/v1/internal/core/ispc/hostabi"
	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	"github.com/weisyn/v1/internal/core/ispc/testutil"
	"github.com/weisyn/v1/internal/core/ispc/zkproof"
	"google.golang.org/grpc"

	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/interfaces/tx"
	"github.com/weisyn/v1/pkg/types"
	core "github.com/weisyn/v1/pb/blockchain/block"
	utxo "github.com/weisyn/v1/pb/blockchain/utxo"
	ures "github.com/weisyn/v1/pkg/interfaces/ures"
	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	pb_resource "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
)

// ============================================================================
// Manager 核心功能测试
// ============================================================================
//
// 🎯 **测试目的**：发现代码缺陷和BUG，确保实现正确性
//
// ✅ **重构说明**：使用testutil包中的统一Mock对象，遵循测试规范
//
// ============================================================================

// mockInternalEngineManager Mock的引擎管理器
type mockInternalEngineManager struct{}

func (m *mockInternalEngineManager) ExecuteWASM(ctx context.Context, hash []byte, method string, params []uint64) ([]uint64, error) {
	return []uint64{1, 2, 3}, nil
}

func (m *mockInternalEngineManager) ExecuteONNX(ctx context.Context, hash []byte, tensorInputs []ispcInterfaces.TensorInput) ([]ispcInterfaces.TensorOutput, error) {
	// 简化的Mock实现：返回一个固定的张量输出
	return []ispcInterfaces.TensorOutput{
		{
			Name:    "output_0",
			DType:   "float64",
			Shape:   []int64{2},
			Layout:  "",
			Values:  []float64{1.0, 2.0},
			RawData: nil,
		},
	}, nil
}

func (m *mockInternalEngineManager) Shutdown(ctx context.Context) error {
	return nil
}

// createTestManager 创建测试用的Manager
//
// ✅ **重构说明**：使用testutil包中的统一Mock对象，遵循测试规范
func createTestManager(t *testing.T) *Manager {
	logger := testutil.NewTestLogger()
	configProvider := testutil.NewTestConfigProvider()
	clock := testutil.NewTestClock()

	// 创建contextManager
	contextManager := ctxmgr.NewManager(logger, configProvider, clock)

	// 创建zkproofManager
	hashManager := testutil.NewTestHashManager()
	signatureManager := testutil.NewTestSignatureManager()
	zkproofManager := zkproof.NewManager(hashManager, signatureManager, logger, configProvider)

	// 创建hostProvider（需要Mock所有依赖）
	hostProvider := createMockHostProvider(t, logger)

	// 创建engineManager
	engineManager := &mockInternalEngineManager{}

	return NewManager(
		engineManager,
		contextManager,
		zkproofManager,
		hostProvider,
		logger,
		configProvider,
	)
}

// TestNewManager 测试创建Manager
func TestNewManager(t *testing.T) {
	logger := testutil.NewTestLogger()
	configProvider := testutil.NewTestConfigProvider()
	clock := testutil.NewTestClock()

	// 创建依赖
	contextManager := ctxmgr.NewManager(logger, configProvider, clock)
	hashManager := testutil.NewTestHashManager()
	signatureManager := testutil.NewTestSignatureManager()
	zkproofManager := zkproof.NewManager(hashManager, signatureManager, logger, configProvider)
	hostProvider := createMockHostProvider(t, logger)
	engineManager := &mockInternalEngineManager{}

	// 创建Manager
	manager := NewManager(
		engineManager,
		contextManager,
		zkproofManager,
		hostProvider,
		logger,
		configProvider,
	)

	// 验证Manager已正确创建
	require.NotNil(t, manager)
	assert.NotNil(t, manager.engineManager)
	assert.NotNil(t, manager.contextManager)
	assert.NotNil(t, manager.zkproofManager)
	assert.NotNil(t, manager.hostProvider)
	assert.NotNil(t, manager.logger)
	assert.NotNil(t, manager.configProvider)
	assert.False(t, manager.asyncZKProofEnabled, "异步ZK证明应该默认禁用")
	assert.NotNil(t, manager.zkProofTaskStore, "任务存储应该已初始化")
}

// TestNewManager_NilDependencies 测试创建Manager时nil依赖的处理
// 🐛 **BUG检测**：测试nil依赖是否会导致panic或错误
func TestNewManager_NilDependencies(t *testing.T) {
	logger := testutil.NewTestLogger()
	configProvider := testutil.NewTestConfigProvider()
	clock := testutil.NewTestClock()

	contextManager := ctxmgr.NewManager(logger, configProvider, clock)
	hashManager := testutil.NewTestHashManager()
	signatureManager := testutil.NewTestSignatureManager()
	zkproofManager := zkproof.NewManager(hashManager, signatureManager, logger, configProvider)
	hostProvider := createMockHostProvider(t, logger)
	engineManager := &mockInternalEngineManager{}

	// ⚠️ **BUG检测**：测试nil依赖
	// 注意：NewManager不检查nil，这可能是设计决策，但应该测试
	tests := []struct {
		name           string
		engineManager  ispcInterfaces.InternalEngineManager
		contextManager *ctxmgr.Manager
		zkproofManager *zkproof.Manager
		hostProvider   *hostabi.HostFunctionProvider
		logger         interface{}
		configProvider interface{}
		expectPanic    bool
	}{
		{
			name:           "nil engineManager",
			engineManager:  nil,
			contextManager: contextManager,
			zkproofManager: zkproofManager,
			hostProvider:   hostProvider,
			logger:         logger,
			configProvider: configProvider,
			expectPanic:    false, // NewManager不检查nil
		},
		{
			name:           "nil contextManager",
			engineManager:  engineManager,
			contextManager: nil,
			zkproofManager: zkproofManager,
			hostProvider:   hostProvider,
			logger:         logger,
			configProvider: configProvider,
			expectPanic:    false, // NewManager不检查nil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectPanic {
				assert.Panics(t, func() {
					loggerVal, _ := tt.logger.(log.Logger)
					configVal, _ := tt.configProvider.(config.Provider)
					_ = loggerVal
					_ = configVal
					NewManager(
						tt.engineManager,
						tt.contextManager,
						tt.zkproofManager,
						tt.hostProvider,
						loggerVal,
						configVal,
					)
				}, "应该panic")
			} else {
				// 不panic，但可能创建了无效的Manager
				loggerVal, _ := tt.logger.(log.Logger)
				configVal, _ := tt.configProvider.(config.Provider)
				manager := NewManager(
					tt.engineManager,
					tt.contextManager,
					tt.zkproofManager,
					tt.hostProvider,
					loggerVal,
					configVal,
				)
				// ⚠️ **潜在BUG**：如果依赖为nil，后续调用可能panic
				if manager != nil {
					t.Logf("⚠️ 警告：Manager已创建，但依赖为nil，后续调用可能失败")
				}
			}
		})
	}
}

// TestManager_SetRuntimeDependencies 测试运行时依赖注入
func TestManager_SetRuntimeDependencies(t *testing.T) {
	manager := createTestManager(t)

	// ⚠️ **BUG检测**：测试nil依赖的处理
	tests := []struct {
		name         string
		queryService interface{}
		uresCAS      interface{}
		draftSvc     interface{}
		hashMgr      interface{}
		expectError  bool
		errorMsg     string
	}{
		{
			name:         "all nil",
			queryService: nil,
			uresCAS:      nil,
			draftSvc:     nil,
			hashMgr:      nil,
			expectError:  true,
			errorMsg:     "queryService cannot be nil",
		},
		{
			name:         "nil queryService",
			queryService: nil,
			uresCAS:      &mockCASStorage{},
			draftSvc:     &mockDraftService{},
			hashMgr:      testutil.NewTestHashManager(),
			expectError:  true,
			errorMsg:     "queryService cannot be nil",
		},
		{
			name:         "nil uresCAS",
			queryService: &mockQueryService{},
			uresCAS:      nil,
			draftSvc:     &mockDraftService{},
			hashMgr:      testutil.NewTestHashManager(),
			expectError:  true,
			errorMsg:     "uresCAS cannot be nil",
		},
		{
			name:         "nil draftService",
			queryService: &mockQueryService{},
			uresCAS:      &mockCASStorage{},
			draftSvc:     nil,
			hashMgr:      testutil.NewTestHashManager(),
			expectError:  true,
			errorMsg:     "draftService cannot be nil",
		},
		{
			name:         "nil hashManager",
			queryService: &mockQueryService{},
			uresCAS:      &mockCASStorage{},
			draftSvc:     &mockDraftService{},
			hashMgr:      nil,
			expectError:  true,
			errorMsg:     "hashManager cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var queryService persistence.QueryService
			var uresCAS ures.CASStorage
			var draftSvc tx.TransactionDraftService
			var hashMgr crypto.HashManager
			
			if qs, ok := tt.queryService.(persistence.QueryService); ok {
				queryService = qs
			}
			if uc, ok := tt.uresCAS.(ures.CASStorage); ok {
				uresCAS = uc
			}
			if ds, ok := tt.draftSvc.(tx.TransactionDraftService); ok {
				draftSvc = ds
			}
			if hm, ok := tt.hashMgr.(crypto.HashManager); ok {
				hashMgr = hm
			}
			
			err := manager.SetRuntimeDependencies(
				queryService,
				uresCAS,
				draftSvc,
				hashMgr,
			)

			if tt.expectError {
				assert.Error(t, err, "应该返回错误")
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg, "错误信息应该包含预期内容")
				}
			} else {
				assert.NoError(t, err, "不应该返回错误")
			}
		})
	}
}

// TestManager_SetRuntimeDependencies_Success 测试成功的运行时依赖注入
func TestManager_SetRuntimeDependencies_Success(t *testing.T) {
	manager := createTestManager(t)

	queryService := &mockQueryService{}
	uresCAS := &mockCASStorage{}
	draftSvc := &mockDraftService{}
	hashMgr := testutil.NewTestHashManager()

	err := manager.SetRuntimeDependencies(queryService, uresCAS, draftSvc, hashMgr)
	require.NoError(t, err)

	// 验证依赖已注入
	manager.runtimeMutex.RLock()
	assert.NotNil(t, manager.eutxoQuery, "eutxoQuery应该已注入")
	assert.NotNil(t, manager.uresCAS, "uresCAS应该已注入")
	assert.NotNil(t, manager.draftService, "draftService应该已注入")
	assert.NotNil(t, manager.hashManager, "hashManager应该已注入")
	manager.runtimeMutex.RUnlock()
}

// TestManager_SetRuntimeDependencies_NilHostProvider 测试hostProvider为nil的情况
// 🐛 **BUG检测**：如果hostProvider为nil，SetRuntimeDependencies应该返回错误
func TestManager_SetRuntimeDependencies_NilHostProvider(t *testing.T) {
	logger := testutil.NewTestLogger()
	configProvider := testutil.NewTestConfigProvider()
	clock := testutil.NewTestClock()

	contextManager := ctxmgr.NewManager(logger, configProvider, clock)
	hashManager := testutil.NewTestHashManager()
	signatureManager := testutil.NewTestSignatureManager()
	zkproofManager := zkproof.NewManager(hashManager, signatureManager, logger, configProvider)
	engineManager := &mockInternalEngineManager{}

	// 创建Manager，但hostProvider为nil
	manager := &Manager{
		engineManager:  engineManager,
		contextManager: contextManager,
		zkproofManager: zkproofManager,
		hostProvider:   nil, // nil hostProvider
		logger:         logger,
		configProvider: configProvider,
		zkProofTaskStore: make(map[string]*zkproof.ZKProofTask),
	}

	queryService := &mockQueryService{}
	uresCAS := &mockCASStorage{}
	draftSvc := &mockDraftService{}
	hashMgr := testutil.NewTestHashManager()

	// ⚠️ **BUG检测**：hostProvider为nil时应该返回错误
	err := manager.SetRuntimeDependencies(queryService, uresCAS, draftSvc, hashMgr)
	assert.Error(t, err, "hostProvider为nil时应该返回错误")
	assert.Contains(t, err.Error(), "hostProvider cannot be nil")
}

// TestManager_SetRuntimeDependencies_Concurrent 测试并发设置运行时依赖
// 🐛 **BUG检测**：测试并发安全性
func TestManager_SetRuntimeDependencies_Concurrent(t *testing.T) {
	manager := createTestManager(t)

	queryService := &mockQueryService{}
	uresCAS := &mockCASStorage{}
	draftSvc := &mockDraftService{}
	hashMgr := testutil.NewTestHashManager()

	// 并发设置运行时依赖
	concurrency := 10
	done := make(chan bool, concurrency)
	errors := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					errors <- &panicError{panic: r}
				}
				done <- true
			}()

			err := manager.SetRuntimeDependencies(queryService, uresCAS, draftSvc, hashMgr)
			if err != nil {
				errors <- err
			}
		}()
	}

	// 等待所有goroutine完成
	for i := 0; i < concurrency; i++ {
		<-done
	}

	// 检查是否有panic或错误
	select {
	case err := <-errors:
		if _, ok := err.(*panicError); ok {
			t.Errorf("❌ BUG发现：并发设置运行时依赖时发生panic：%v", err)
		} else {
			t.Logf("⚠️ 警告：并发设置运行时依赖时发生错误（可能是幂等问题）：%v", err)
		}
	default:
		t.Logf("✅ 并发设置运行时依赖没有发生panic或错误")
	}

	// 验证最终状态
	manager.runtimeMutex.RLock()
	assert.NotNil(t, manager.eutxoQuery, "eutxoQuery应该已注入")
	assert.NotNil(t, manager.uresCAS, "uresCAS应该已注入")
	assert.NotNil(t, manager.draftService, "draftService应该已注入")
	assert.NotNil(t, manager.hashManager, "hashManager应该已注入")
	manager.runtimeMutex.RUnlock()
}


// ============================================================================
// Mock对象定义
// ============================================================================

// createMockHostProvider 创建Mock的HostFunctionProvider
// ⚠️ **注意**：由于hostabi.NewHostFunctionProvider需要很多依赖，这里创建最小化的Mock
func createMockHostProvider(t *testing.T, logger interface{}) *hostabi.HostFunctionProvider {
	t.Helper()
	
	// 创建Mock依赖
	mockUTXOQuery := &mockUTXOQuery{}
	mockCASStorage := &mockCASStorage{}
	mockDraftService := &mockDraftService{}
	mockTxAdapter := &mockTxAdapter{}
	mockTxHashClient := &mockTxHashServiceClient{}
	mockAddressManager := &mockAddressManager{}
	
	// 创建HostFunctionProvider
	loggerVal, ok := logger.(log.Logger)
	if !ok {
		t.Fatal("logger类型转换失败")
	}
	hostProvider := hostabi.NewHostFunctionProvider(
		loggerVal,
		mockUTXOQuery,
		mockCASStorage,
		mockDraftService,
		mockTxAdapter,
		mockTxHashClient,
		mockAddressManager,
	)
	
	return hostProvider
}

// ============================================================================
// Mock对象定义（实现所需接口）
// ============================================================================

// mockUTXOQuery Mock的UTXO查询服务
type mockUTXOQuery struct{}

func (m *mockUTXOQuery) GetUTXO(ctx context.Context, outpoint *pb.OutPoint) (*utxo.UTXO, error) {
	return nil, nil
}

func (m *mockUTXOQuery) GetUTXOsByAddress(ctx context.Context, address []byte, category *utxo.UTXOCategory, onlyAvailable bool) ([]*utxo.UTXO, error) {
	return nil, nil
}

func (m *mockUTXOQuery) GetSponsorPoolUTXOs(ctx context.Context, onlyAvailable bool) ([]*utxo.UTXO, error) {
	return nil, nil
}

func (m *mockUTXOQuery) GetCurrentStateRoot(ctx context.Context) ([]byte, error) {
	return nil, nil
}

// mockChainQuery Mock的链查询服务
type mockChainQuery struct{}

func (m *mockChainQuery) GetChainInfo(ctx context.Context) (*types.ChainInfo, error) {
	return nil, nil
}

func (m *mockChainQuery) GetCurrentHeight(ctx context.Context) (uint64, error) {
	return 0, nil
}

func (m *mockChainQuery) GetBestBlockHash(ctx context.Context) ([]byte, error) {
	return nil, nil
}

func (m *mockChainQuery) GetNodeMode(ctx context.Context) (types.NodeMode, error) {
	return types.NodeModeFull, nil
}

func (m *mockChainQuery) IsDataFresh(ctx context.Context) (bool, error) {
	return true, nil
}

func (m *mockChainQuery) IsReady(ctx context.Context) (bool, error) {
	return true, nil
}

func (m *mockChainQuery) GetSyncStatus(ctx context.Context) (*types.SystemSyncStatus, error) {
	return nil, nil
}

// mockBlockQuery Mock的区块查询服务
type mockBlockQuery struct{}

func (m *mockBlockQuery) GetBlockByHeight(ctx context.Context, height uint64) (*core.Block, error) {
	return nil, nil
}

func (m *mockBlockQuery) GetBlockByHash(ctx context.Context, blockHash []byte) (*core.Block, error) {
	return nil, nil
}

func (m *mockBlockQuery) GetBlockHeader(ctx context.Context, blockHash []byte) (*core.BlockHeader, error) {
	return nil, nil
}

func (m *mockBlockQuery) GetBlockRange(ctx context.Context, startHeight, endHeight uint64) ([]*core.Block, error) {
	return nil, nil
}

func (m *mockBlockQuery) GetHighestBlock(ctx context.Context) (height uint64, blockHash []byte, err error) {
	return 0, nil, nil
}

// mockTxQuery Mock的交易查询服务
type mockTxQuery struct{}

func (m *mockTxQuery) GetTransaction(ctx context.Context, txHash []byte) (blockHash []byte, txIndex uint32, tx *pb.Transaction, err error) {
	return nil, 0, nil, nil
}

func (m *mockTxQuery) GetTxBlockHeight(ctx context.Context, txHash []byte) (uint64, error) {
	return 0, nil
}

func (m *mockTxQuery) GetBlockTimestamp(ctx context.Context, height uint64) (int64, error) {
	return 0, nil
}

func (m *mockTxQuery) GetAccountNonce(ctx context.Context, address []byte) (uint64, error) {
	return 0, nil
}

func (m *mockTxQuery) GetTransactionsByBlock(ctx context.Context, blockHash []byte) ([]*pb.Transaction, error) {
	return nil, nil
}

// mockResourceQuery Mock的资源查询服务
type mockResourceQuery struct{}

func (m *mockResourceQuery) GetResourceByContentHash(ctx context.Context, contentHash []byte) (*pb_resource.Resource, error) {
	return nil, nil
}

func (m *mockResourceQuery) GetResourceFromBlockchain(ctx context.Context, contentHash []byte) (*pb_resource.Resource, bool, error) {
	return nil, false, nil
}

func (m *mockResourceQuery) GetResourceTransaction(ctx context.Context, contentHash []byte) (txHash, blockHash []byte, blockHeight uint64, err error) {
	return nil, nil, 0, nil
}

func (m *mockResourceQuery) CheckFileExists(contentHash []byte) bool {
	return false
}

func (m *mockResourceQuery) BuildFilePath(contentHash []byte) string {
	return ""
}

func (m *mockResourceQuery) ListResourceHashes(ctx context.Context, offset int, limit int) ([][]byte, error) {
	return nil, nil
}

// mockAccountQuery Mock的账户查询服务
type mockAccountQuery struct{}

func (m *mockAccountQuery) GetAccountBalance(ctx context.Context, address []byte, tokenID []byte) (*types.BalanceInfo, error) {
	return nil, nil
}

// mockPricingQuery Mock的定价查询服务
type mockPricingQuery struct{}

func (m *mockPricingQuery) GetPricingState(ctx context.Context, resourceHash []byte) (*types.ResourcePricingState, error) {
	return nil, nil
}

// mockQueryService Mock的查询服务（实现QueryService接口）
type mockQueryService struct {
	mockChainQuery
	mockBlockQuery
	mockTxQuery
	mockUTXOQuery
	mockResourceQuery
	mockAccountQuery
	mockPricingQuery
}

// mockCASStorage Mock的CAS存储（实现CASStorage接口）
type mockCASStorage struct{}

func (m *mockCASStorage) BuildFilePath(contentHash []byte) string {
	return ""
}

func (m *mockCASStorage) StoreFile(ctx context.Context, contentHash []byte, data []byte) error {
	return nil
}

func (m *mockCASStorage) ReadFile(ctx context.Context, contentHash []byte) ([]byte, error) {
	return nil, nil
}

func (m *mockCASStorage) FileExists(contentHash []byte) bool {
	return false
}

// mockDraftService Mock的交易草稿服务（实现TransactionDraftService接口）
type mockDraftService struct{}

func (m *mockDraftService) CreateDraft(ctx context.Context) (*types.DraftTx, error) {
	return nil, nil
}

func (m *mockDraftService) LoadDraft(ctx context.Context, draftID string) (*types.DraftTx, error) {
	return nil, nil
}

func (m *mockDraftService) SaveDraft(ctx context.Context, draft *types.DraftTx) error {
	return nil
}

func (m *mockDraftService) DeleteDraft(ctx context.Context, draftID string) error {
	return nil
}

func (m *mockDraftService) SealDraft(ctx context.Context, draft *types.DraftTx) (*types.ComposedTx, error) {
	return nil, nil
}

func (m *mockDraftService) AddInput(ctx context.Context, draft *types.DraftTx, outpoint *pb.OutPoint, isReferenceOnly bool, unlockingProof *pb.UnlockingProof) (uint32, error) {
	return 0, nil
}

func (m *mockDraftService) AddAssetOutput(ctx context.Context, draft *types.DraftTx, owner []byte, amount string, tokenID []byte, lockingConditions []*pb.LockingCondition) (uint32, error) {
	return 0, nil
}

func (m *mockDraftService) AddResourceOutput(ctx context.Context, draft *types.DraftTx, contentHash []byte, category string, owner []byte, lockingConditions []*pb.LockingCondition, metadata []byte) (uint32, error) {
	return 0, nil
}

func (m *mockDraftService) AddStateOutput(ctx context.Context, draft *types.DraftTx, stateID []byte, stateVersion uint64, executionResultHash []byte, publicInputs []byte, parentStateHash []byte) (uint32, error) {
	return 0, nil
}

func (m *mockDraftService) GetDraftByID(ctx context.Context, draftID string) (*types.DraftTx, error) {
	return nil, nil
}

func (m *mockDraftService) ValidateDraft(ctx context.Context, draft *types.DraftTx) error {
	return nil
}

// mockTxAdapter Mock的TX适配器（实现hostabi.TxAdapter接口）
type mockTxAdapter struct{}

func (m *mockTxAdapter) BeginTransaction(ctx context.Context, blockHeight uint64, blockTimestamp uint64) (int32, error) {
	return 0, nil
}

func (m *mockTxAdapter) AddTransfer(ctx context.Context, draftHandle int32, from []byte, to []byte, amount string, tokenID []byte) (int32, error) {
	return 0, nil
}

func (m *mockTxAdapter) AddCustomInput(ctx context.Context, draftHandle int32, outpoint *pb.OutPoint, isReferenceOnly bool) (int32, error) {
	return 0, nil
}

func (m *mockTxAdapter) AddCustomOutput(ctx context.Context, draftHandle int32, output *pb.TxOutput) (int32, error) {
	return 0, nil
}

func (m *mockTxAdapter) GetDraft(ctx context.Context, draftHandle int32) (*types.DraftTx, error) {
	return nil, nil
}

func (m *mockTxAdapter) FinalizeTransaction(ctx context.Context, draftHandle int32) (*pb.Transaction, error) {
	return nil, nil
}

func (m *mockTxAdapter) CleanupDraft(ctx context.Context, draftHandle int32) error {
	return nil
}

// mockTxHashServiceClient Mock的交易哈希服务客户端
type mockTxHashServiceClient struct{}

func (m *mockTxHashServiceClient) ComputeTransactionHash(ctx context.Context, tx *pb.Transaction) ([]byte, error) {
	return nil, nil
}

func (m *mockTxHashServiceClient) ComputeHash(ctx context.Context, in *pb.ComputeHashRequest, opts ...grpc.CallOption) (*pb.ComputeHashResponse, error) {
	return &pb.ComputeHashResponse{Hash: nil}, nil
}

func (m *mockTxHashServiceClient) ValidateHash(ctx context.Context, in *pb.ValidateHashRequest, opts ...grpc.CallOption) (*pb.ValidateHashResponse, error) {
	return &pb.ValidateHashResponse{IsValid: true}, nil
}

func (m *mockTxHashServiceClient) ComputeSignatureHash(ctx context.Context, in *pb.ComputeSignatureHashRequest, opts ...grpc.CallOption) (*pb.ComputeSignatureHashResponse, error) {
	return &pb.ComputeSignatureHashResponse{Hash: nil}, nil
}

func (m *mockTxHashServiceClient) ValidateSignatureHash(ctx context.Context, in *pb.ValidateSignatureHashRequest, opts ...grpc.CallOption) (*pb.ValidateSignatureHashResponse, error) {
	return &pb.ValidateSignatureHashResponse{IsValid: true}, nil
}

// mockAddressManager Mock的地址管理器
type mockAddressManager struct{}

func (m *mockAddressManager) EncodeAddress(address []byte) string {
	return ""
}

func (m *mockAddressManager) DecodeAddress(encoded string) ([]byte, error) {
	return nil, nil
}

func (m *mockAddressManager) ValidateAddress(address string) (bool, error) {
	return true, nil
}

func (m *mockAddressManager) AddressToBytes(address string) ([]byte, error) {
	return nil, nil
}

func (m *mockAddressManager) AddressToHexString(address string) (string, error) {
	return "", nil
}

func (m *mockAddressManager) HexStringToAddress(hexString string) (string, error) {
	return "", nil
}

func (m *mockAddressManager) BytesToAddress(addressBytes []byte) (string, error) {
	return "", nil
}

func (m *mockAddressManager) PrivateKeyToAddress(privateKey []byte) (string, error) {
	return "", nil
}

func (m *mockAddressManager) PublicKeyToAddress(publicKey []byte) (string, error) {
	return "", nil
}

func (m *mockAddressManager) CompareAddresses(addr1, addr2 string) (bool, error) {
	return false, nil
}

func (m *mockAddressManager) GetAddressType(address string) (crypto.AddressType, error) {
	return crypto.AddressTypeBitcoin, nil
}

func (m *mockAddressManager) IsZeroAddress(address string) bool {
	return false
}

func (m *mockAddressManager) StringToAddress(addressStr string) (string, error) {
	return "", nil
}

// panicError 用于捕获panic错误
type panicError struct {
	panic interface{}
}

func (e *panicError) Error() string {
	return "panic occurred"
}

