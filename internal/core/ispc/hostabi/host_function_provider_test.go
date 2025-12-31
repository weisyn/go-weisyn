package hostabi

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"github.com/weisyn/v1/internal/core/ispc/testutil"
	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	utxo "github.com/weisyn/v1/pb/blockchain/utxo"
	core "github.com/weisyn/v1/pb/blockchain/block"
	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	pb_resource "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/types"
)

// ============================================================================
// HostFunctionProvider 测试
// ============================================================================
//
// 🎯 **测试目的**：发现 HostFunctionProvider 的缺陷和BUG
//
// ============================================================================

// TestNewHostFunctionProvider 测试创建HostFunctionProvider
func TestNewHostFunctionProvider(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockUTXOQuery := &mockUTXOQueryForProvider{}
	mockCASStorage := &mockCASStorageForProvider{}
	mockDraftService := &mockDraftServiceForProvider{}
	mockTxAdapter := &mockTxAdapterForProvider{}
	mockTxHashClient := &mockTxHashServiceClientForProvider{}
	mockAddressManager := &mockAddressManagerForProvider{}

	provider := NewHostFunctionProvider(
		logger,
		mockUTXOQuery,
		mockCASStorage,
		mockDraftService,
		mockTxAdapter,
		mockTxHashClient,
		mockAddressManager,
	)

	assert.NotNil(t, provider, "HostFunctionProvider应该被创建")
	assert.Equal(t, logger, provider.logger, "logger应该被设置")
	assert.Equal(t, mockUTXOQuery, provider.eutxoQuery, "eutxoQuery应该被设置")
	assert.Equal(t, mockCASStorage, provider.uresCAS, "uresCAS应该被设置")
	assert.Equal(t, mockDraftService, provider.draftService, "draftService应该被设置")
	assert.Equal(t, mockTxAdapter, provider.txAdapter, "txAdapter应该被设置")
	assert.Equal(t, mockTxHashClient, provider.txHashClient, "txHashClient应该被设置")
	assert.Equal(t, mockAddressManager, provider.addressManager, "addressManager应该被设置")
	assert.True(t, provider.cacheEnabled, "缓存应该默认启用")
	assert.NotNil(t, provider.primitiveCache, "primitiveCache应该被创建")
}

// TestNewHostFunctionProviderWithCache 测试创建带缓存配置的HostFunctionProvider
func TestNewHostFunctionProviderWithCache(t *testing.T) {
	logger := testutil.NewTestLogger()
	mockUTXOQuery := &mockUTXOQueryForProvider{}
	mockCASStorage := &mockCASStorageForProvider{}
	mockDraftService := &mockDraftServiceForProvider{}
	mockTxAdapter := &mockTxAdapterForProvider{}
	mockTxHashClient := &mockTxHashServiceClientForProvider{}
	mockAddressManager := &mockAddressManagerForProvider{}

	tests := []struct {
		name        string
		enableCache bool
		cacheSize   int
		cacheTTL    time.Duration
	}{
		{"启用缓存", true, 100, 30 * time.Second},
		{"禁用缓存", false, 0, 0},
		{"大缓存", true, 1000, 5 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewHostFunctionProviderWithCache(
				logger,
				mockUTXOQuery,
				mockCASStorage,
				mockDraftService,
				mockTxAdapter,
				mockTxHashClient,
				mockAddressManager,
				tt.enableCache,
				tt.cacheSize,
				tt.cacheTTL,
			)

			assert.NotNil(t, provider, "HostFunctionProvider应该被创建")
			assert.Equal(t, tt.enableCache, provider.cacheEnabled, "cacheEnabled应该正确设置")
			if tt.enableCache {
				assert.NotNil(t, provider.primitiveCache, "primitiveCache应该被创建")
			} else {
				assert.Nil(t, provider.primitiveCache, "primitiveCache应该为nil")
			}
		})
	}
}

// TestHostFunctionProvider_SetMethods 测试各种Set方法
func TestHostFunctionProvider_SetMethods(t *testing.T) {
	provider := createTestHostFunctionProvider(t)

	mockChainQuery := &mockChainQueryForProvider{}
	mockBlockQuery := &mockBlockQueryForProvider{}
	mockTxQuery := &mockTxQueryForProvider{}
	mockResourceQuery := &mockResourceQueryForProvider{}
	mockHashManager := testutil.NewTestHashManager()
	mockTxAdapter := &mockTxAdapterForProvider{}

	provider.SetChainQuery(mockChainQuery)
	assert.Equal(t, mockChainQuery, provider.chainQuery, "chainQuery应该被设置")

	provider.SetBlockQuery(mockBlockQuery)
	assert.Equal(t, mockBlockQuery, provider.blockQuery, "blockQuery应该被设置")

	provider.SetTxQuery(mockTxQuery)
	assert.Equal(t, mockTxQuery, provider.txQuery, "txQuery应该被设置")

	provider.SetResourceQuery(mockResourceQuery)
	assert.Equal(t, mockResourceQuery, provider.resourceQuery, "resourceQuery应该被设置")

	provider.SetHashManager(mockHashManager)
	assert.Equal(t, mockHashManager, provider.hashManager, "hashManager应该被设置")

	provider.SetTxAdapter(mockTxAdapter)
	assert.Equal(t, mockTxAdapter, provider.txAdapter, "txAdapter应该被设置")
}

// TestWithExecutionContext 测试WithExecutionContext
func TestWithExecutionContext(t *testing.T) {
	ctx := context.Background()
	mockExecCtx := createMockExecutionContextForProvider()

	newCtx := WithExecutionContext(ctx, mockExecCtx)
	assert.NotEqual(t, ctx, newCtx, "应该返回新的context")

	// 验证可以从context中提取
	extracted := GetExecutionContext(newCtx)
	assert.Equal(t, mockExecCtx, extracted, "应该能正确提取ExecutionContext")
}

// TestGetExecutionContext 测试GetExecutionContext
func TestGetExecutionContext(t *testing.T) {
	tests := []struct {
		name     string
		setupCtx func() context.Context
		wantNil  bool
	}{
		{
			name: "包含ExecutionContext",
			setupCtx: func() context.Context {
				ctx := context.Background()
				mockExecCtx := createMockExecutionContextForProvider()
				return WithExecutionContext(ctx, mockExecCtx)
			},
			wantNil: false,
		},
		{
			name: "不包含ExecutionContext",
			setupCtx: func() context.Context {
				return context.Background()
			},
			wantNil: true,
		},
		{
			name: "错误的类型",
			setupCtx: func() context.Context {
				ctx := context.Background()
				return context.WithValue(ctx, executionContextKey, "invalid-type")
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupCtx()
			result := GetExecutionContext(ctx)
			if tt.wantNil {
				assert.Nil(t, result, "应该返回nil")
			} else {
				assert.NotNil(t, result, "应该返回ExecutionContext")
			}
		})
	}
}

// TestHostFunctionProvider_GetWASMHostFunctions_MissingExecutionContext 测试缺少ExecutionContext
func TestHostFunctionProvider_GetWASMHostFunctions_MissingExecutionContext(t *testing.T) {
	provider := createTestHostFunctionProvider(t)
	provider.SetChainQuery(&mockChainQueryForProvider{})

	ctx := context.Background() // 没有设置ExecutionContext
	_, err := provider.GetWASMHostFunctions(ctx, "exec-123")

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "ExecutionContext 未在 context 中设置", "错误信息应该正确")
}

// TestHostFunctionProvider_GetWASMHostFunctions_InvalidExecutionContextType 测试无效的ExecutionContext类型
func TestHostFunctionProvider_GetWASMHostFunctions_InvalidExecutionContextType(t *testing.T) {
	provider := createTestHostFunctionProvider(t)
	provider.SetChainQuery(&mockChainQueryForProvider{})

	ctx := context.WithValue(context.Background(), executionContextKey, "invalid-type")
	_, err := provider.GetWASMHostFunctions(ctx, "exec-123")

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "类型不正确", "错误信息应该正确")
}

// TestHostFunctionProvider_GetWASMHostFunctions_MissingChainQuery 测试缺少chainQuery
func TestHostFunctionProvider_GetWASMHostFunctions_MissingChainQuery(t *testing.T) {
	provider := createTestHostFunctionProvider(t)
	ctx := context.Background()
	mockExecCtx := createMockExecutionContextForProvider()
	ctx = WithExecutionContext(ctx, mockExecCtx)

	// 不设置chainQuery
	_, err := provider.GetWASMHostFunctions(ctx, "exec-123")

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "chainQuery 未设置", "错误信息应该正确")
}

// TestHostFunctionProvider_GetWASMHostFunctions_Success 测试成功获取WASM宿主函数
func TestHostFunctionProvider_GetWASMHostFunctions_Success(t *testing.T) {
	provider := createTestHostFunctionProvider(t)
	provider.SetChainQuery(&mockChainQueryForProvider{})
	provider.SetBlockQuery(&mockBlockQueryForProvider{})
	provider.SetTxQuery(&mockTxQueryForProvider{})
	provider.SetResourceQuery(&mockResourceQueryForProvider{})
	provider.SetHashManager(testutil.NewTestHashManager())

	ctx := context.Background()
	mockExecCtx := createMockExecutionContextForProvider()
	ctx = WithExecutionContext(ctx, mockExecCtx)

	functions, err := provider.GetWASMHostFunctions(ctx, "exec-123")

	assert.NoError(t, err, "应该成功")
	assert.NotNil(t, functions, "应该返回函数映射")
	// 注意：由于使用了真实的WASMAdapter，函数映射可能为空或包含函数
	// 这里主要测试错误路径，成功路径需要完整的依赖设置
	if err == nil {
		assert.NotNil(t, functions, "应该返回函数映射（可能为空）")
	}
}

// TestHostFunctionProvider_GetONNXHostFunctions_MissingExecutionContext 测试ONNX缺少ExecutionContext（应该返回空映射）
func TestHostFunctionProvider_GetONNXHostFunctions_MissingExecutionContext(t *testing.T) {
	provider := createTestHostFunctionProvider(t)
	provider.SetChainQuery(&mockChainQueryForProvider{})

	ctx := context.Background() // 没有设置ExecutionContext
	functions, err := provider.GetONNXHostFunctions(ctx, "exec-123")

	assert.NoError(t, err, "ONNX应该允许缺少ExecutionContext")
	assert.NotNil(t, functions, "应该返回函数映射")
	assert.Equal(t, 0, len(functions), "应该返回空映射")
}

// TestHostFunctionProvider_GetONNXHostFunctions_Success 测试成功获取ONNX宿主函数
func TestHostFunctionProvider_GetONNXHostFunctions_Success(t *testing.T) {
	provider := createTestHostFunctionProvider(t)
	provider.SetChainQuery(&mockChainQueryForProvider{})
	provider.SetBlockQuery(&mockBlockQueryForProvider{})
	provider.SetTxQuery(&mockTxQueryForProvider{})
	provider.SetResourceQuery(&mockResourceQueryForProvider{})
	provider.SetHashManager(testutil.NewTestHashManager())

	ctx := context.Background()
	mockExecCtx := createMockExecutionContextForProvider()
	ctx = WithExecutionContext(ctx, mockExecCtx)

	functions, err := provider.GetONNXHostFunctions(ctx, "exec-123")

	assert.NoError(t, err, "应该成功")
	assert.NotNil(t, functions, "应该返回函数映射")
	// 注意：ONNX宿主函数可能为空或包含函数
	// 这里主要测试错误路径，成功路径需要完整的依赖设置
	if err == nil {
		assert.NotNil(t, functions, "应该返回函数映射（可能为空）")
	}
}

// TestHostFunctionProvider_GetCacheStats 测试获取缓存统计
func TestHostFunctionProvider_GetCacheStats(t *testing.T) {
	tests := []struct {
		name        string
		enableCache bool
		wantNil     bool
	}{
		{"缓存启用", true, false},
		{"缓存禁用", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := createTestHostFunctionProviderWithCache(t, tt.enableCache, 100, time.Minute)
			stats := provider.GetCacheStats()

			if tt.wantNil {
				assert.Nil(t, stats, "应该返回nil")
			} else {
				assert.NotNil(t, stats, "应该返回统计信息")
			}
		})
	}
}

// TestHostFunctionProvider_ClearCache 测试清空缓存
func TestHostFunctionProvider_ClearCache(t *testing.T) {
	provider := createTestHostFunctionProvider(t)
	assert.NotNil(t, provider.primitiveCache, "缓存应该存在")

	// 清空缓存不应该panic
	assert.NotPanics(t, func() {
		provider.ClearCache()
	}, "清空缓存不应该panic")
}

// TestHostFunctionProvider_SetExecutionContext_Deprecated 测试废弃的SetExecutionContext方法
func TestHostFunctionProvider_SetExecutionContext_Deprecated(t *testing.T) {
	provider := createTestHostFunctionProvider(t)

	// 废弃方法应该不panic，也不做任何操作
	assert.NotPanics(t, func() {
		provider.SetExecutionContext("any-value")
	}, "废弃方法不应该panic")
}

// ============================================================================
// 辅助函数
// ============================================================================

// createTestHostFunctionProvider 创建测试用的HostFunctionProvider
func createTestHostFunctionProvider(t *testing.T) *HostFunctionProvider {
	t.Helper()

	logger := testutil.NewTestLogger()
	mockUTXOQuery := &mockUTXOQueryForProvider{}
	mockCASStorage := &mockCASStorageForProvider{}
	mockDraftService := &mockDraftServiceForProvider{}
	mockTxAdapter := &mockTxAdapterForProvider{}
	mockTxHashClient := &mockTxHashServiceClientForProvider{}
	mockAddressManager := &mockAddressManagerForProvider{}

	return NewHostFunctionProvider(
		logger,
		mockUTXOQuery,
		mockCASStorage,
		mockDraftService,
		mockTxAdapter,
		mockTxHashClient,
		mockAddressManager,
	)
}

// createTestHostFunctionProviderWithCache 创建带缓存配置的测试用HostFunctionProvider
func createTestHostFunctionProviderWithCache(t *testing.T, enableCache bool, cacheSize int, cacheTTL time.Duration) *HostFunctionProvider {
	t.Helper()

	logger := testutil.NewTestLogger()
	mockUTXOQuery := &mockUTXOQueryForProvider{}
	mockCASStorage := &mockCASStorageForProvider{}
	mockDraftService := &mockDraftServiceForProvider{}
	mockTxAdapter := &mockTxAdapterForProvider{}
	mockTxHashClient := &mockTxHashServiceClientForProvider{}
	mockAddressManager := &mockAddressManagerForProvider{}

	return NewHostFunctionProviderWithCache(
		logger,
		mockUTXOQuery,
		mockCASStorage,
		mockDraftService,
		mockTxAdapter,
		mockTxHashClient,
		mockAddressManager,
		enableCache,
		cacheSize,
		cacheTTL,
	)
}

// createMockExecutionContextForProvider 创建Mock的ExecutionContext
func createMockExecutionContextForProvider() ispcInterfaces.ExecutionContext {
	return &mockExecutionContextForProvider{
		executionID:      "exec-123",
		callerAddress:    make([]byte, 20),
		contractAddress:  make([]byte, 20),
		txID:             make([]byte, 32),
		chainID:          []byte("test-chain"),
		blockHeight:      100,
		blockTimestamp:   1234567890,
		draftID:          "draft-123",
	}
}

// ============================================================================
// Mock对象定义
// ============================================================================

// mockExecutionContextForProvider Mock的ExecutionContext
type mockExecutionContextForProvider struct {
	executionID     string
	callerAddress   []byte
	contractAddress []byte
	txID            []byte
	chainID         []byte
	blockHeight     uint64
	blockTimestamp  uint64
	draftID         string
}

func (m *mockExecutionContextForProvider) GetExecutionID() string { return m.executionID }
func (m *mockExecutionContextForProvider) GetDraftID() string { return m.draftID }
func (m *mockExecutionContextForProvider) GetBlockHeight() uint64 { return m.blockHeight }
func (m *mockExecutionContextForProvider) GetBlockTimestamp() uint64 { return m.blockTimestamp }
func (m *mockExecutionContextForProvider) GetChainID() []byte { return m.chainID }
func (m *mockExecutionContextForProvider) GetTransactionID() []byte { return m.txID }
func (m *mockExecutionContextForProvider) GetCallerAddress() []byte { return m.callerAddress }
func (m *mockExecutionContextForProvider) GetContractAddress() []byte { return m.contractAddress }
func (m *mockExecutionContextForProvider) HostABI() ispcInterfaces.HostABI { return nil }
func (m *mockExecutionContextForProvider) SetHostABI(hostABI ispcInterfaces.HostABI) error { return nil }
func (m *mockExecutionContextForProvider) GetTransactionDraft() (*ispcInterfaces.TransactionDraft, error) { return nil, nil }
func (m *mockExecutionContextForProvider) UpdateTransactionDraft(draft *ispcInterfaces.TransactionDraft) error { return nil }
func (m *mockExecutionContextForProvider) RecordHostFunctionCall(call *ispcInterfaces.HostFunctionCall) {}
func (m *mockExecutionContextForProvider) GetExecutionTrace() ([]*ispcInterfaces.HostFunctionCall, error) { return nil, nil }
func (m *mockExecutionContextForProvider) RecordStateChange(key string, oldValue interface{}, newValue interface{}) error { return nil }
func (m *mockExecutionContextForProvider) RecordTraceRecords(records []ispcInterfaces.TraceRecord) error { return nil }
func (m *mockExecutionContextForProvider) GetResourceUsage() *types.ResourceUsage { return nil }
func (m *mockExecutionContextForProvider) FinalizeResourceUsage() {}
func (m *mockExecutionContextForProvider) SetReturnData(data []byte) error { return nil }
func (m *mockExecutionContextForProvider) GetReturnData() ([]byte, error) { return nil, nil }
func (m *mockExecutionContextForProvider) AddEvent(event *ispcInterfaces.Event) error { return nil }
func (m *mockExecutionContextForProvider) GetEvents() ([]*ispcInterfaces.Event, error) { return nil, nil }
func (m *mockExecutionContextForProvider) SetInitParams(params []byte) error { return nil }
func (m *mockExecutionContextForProvider) GetInitParams() ([]byte, error) { return nil, nil }

// mockUTXOQueryForProvider Mock的UTXO查询服务
type mockUTXOQueryForProvider struct{}

func (m *mockUTXOQueryForProvider) GetUTXO(ctx context.Context, outpoint *pb.OutPoint) (*utxo.UTXO, error) { return nil, nil }
func (m *mockUTXOQueryForProvider) GetUTXOsByAddress(ctx context.Context, address []byte, category *utxo.UTXOCategory, onlyAvailable bool) ([]*utxo.UTXO, error) { return nil, nil }
func (m *mockUTXOQueryForProvider) GetSponsorPoolUTXOs(ctx context.Context, onlyAvailable bool) ([]*utxo.UTXO, error) { return nil, nil }
func (m *mockUTXOQueryForProvider) GetCurrentStateRoot(ctx context.Context) ([]byte, error) { return nil, nil }

// mockCASStorageForProvider Mock的CAS存储
type mockCASStorageForProvider struct{}

func (m *mockCASStorageForProvider) BuildFilePath(contentHash []byte) string { return "" }
func (m *mockCASStorageForProvider) StoreFile(ctx context.Context, contentHash []byte, data []byte) error { return nil }
func (m *mockCASStorageForProvider) ReadFile(ctx context.Context, contentHash []byte) ([]byte, error) { return nil, nil }
func (m *mockCASStorageForProvider) FileExists(contentHash []byte) bool { return false }

// 实现 ures.CASStorage 接口的其他方法（如果需要）
func (m *mockCASStorageForProvider) GetResourceByContentHash(ctx context.Context, contentHash []byte) (interface{}, error) { return nil, nil }
func (m *mockCASStorageForProvider) StoreResource(ctx context.Context, contentHash []byte, resource interface{}) error { return nil }

// mockDraftServiceForProvider Mock的交易草稿服务
type mockDraftServiceForProvider struct{}

func (m *mockDraftServiceForProvider) CreateDraft(ctx context.Context) (*types.DraftTx, error) { return nil, nil }
func (m *mockDraftServiceForProvider) LoadDraft(ctx context.Context, draftID string) (*types.DraftTx, error) { return nil, nil }
func (m *mockDraftServiceForProvider) SaveDraft(ctx context.Context, draft *types.DraftTx) error { return nil }
func (m *mockDraftServiceForProvider) GetDraftByID(ctx context.Context, draftID string) (*types.DraftTx, error) { return nil, nil }
func (m *mockDraftServiceForProvider) ValidateDraft(ctx context.Context, draft *types.DraftTx) error { return nil }
func (m *mockDraftServiceForProvider) SealDraft(ctx context.Context, draft *types.DraftTx) (*types.ComposedTx, error) { return nil, nil }
func (m *mockDraftServiceForProvider) DeleteDraft(ctx context.Context, draftID string) error { return nil }
func (m *mockDraftServiceForProvider) AddInput(ctx context.Context, draft *types.DraftTx, outpoint *pb.OutPoint, isReferenceOnly bool, unlockingProof *pb.UnlockingProof) (uint32, error) { return 0, nil }
func (m *mockDraftServiceForProvider) AddAssetOutput(ctx context.Context, draft *types.DraftTx, owner []byte, amount string, tokenID []byte, lockingConditions []*pb.LockingCondition) (uint32, error) { return 0, nil }
func (m *mockDraftServiceForProvider) AddResourceOutput(ctx context.Context, draft *types.DraftTx, contentHash []byte, category string, owner []byte, lockingConditions []*pb.LockingCondition, metadata []byte) (uint32, error) { return 0, nil }
func (m *mockDraftServiceForProvider) AddStateOutput(ctx context.Context, draft *types.DraftTx, stateID []byte, stateVersion uint64, executionResultHash []byte, publicInputs []byte, parentStateHash []byte) (uint32, error) { return 0, nil }

// mockTxAdapterForProvider Mock的TX适配器
type mockTxAdapterForProvider struct{}

func (m *mockTxAdapterForProvider) BeginTransaction(ctx context.Context, blockHeight uint64, blockTimestamp uint64) (int32, error) { return 0, nil }
func (m *mockTxAdapterForProvider) AddTransfer(ctx context.Context, draftHandle int32, from []byte, to []byte, amount string, tokenID []byte) (int32, error) { return 0, nil }
func (m *mockTxAdapterForProvider) AddCustomInput(ctx context.Context, draftHandle int32, outpoint *pb.OutPoint, isReferenceOnly bool) (int32, error) { return 0, nil }
func (m *mockTxAdapterForProvider) AddCustomOutput(ctx context.Context, draftHandle int32, output *pb.TxOutput) (int32, error) { return 0, nil }
func (m *mockTxAdapterForProvider) GetDraft(ctx context.Context, draftHandle int32) (*types.DraftTx, error) { return nil, nil }
func (m *mockTxAdapterForProvider) FinalizeTransaction(ctx context.Context, draftHandle int32) (*pb.Transaction, error) { return nil, nil }
func (m *mockTxAdapterForProvider) CleanupDraft(ctx context.Context, draftHandle int32) error { return nil }

// mockTxHashServiceClientForProvider Mock的交易哈希服务客户端
type mockTxHashServiceClientForProvider struct{}

func (m *mockTxHashServiceClientForProvider) ComputeHash(ctx context.Context, in *pb.ComputeHashRequest, opts ...grpc.CallOption) (*pb.ComputeHashResponse, error) {
	return &pb.ComputeHashResponse{Hash: make([]byte, 32)}, nil
}
func (m *mockTxHashServiceClientForProvider) ValidateHash(ctx context.Context, in *pb.ValidateHashRequest, opts ...grpc.CallOption) (*pb.ValidateHashResponse, error) {
	return &pb.ValidateHashResponse{IsValid: true}, nil
}
func (m *mockTxHashServiceClientForProvider) ComputeSignatureHash(ctx context.Context, in *pb.ComputeSignatureHashRequest, opts ...grpc.CallOption) (*pb.ComputeSignatureHashResponse, error) {
	return &pb.ComputeSignatureHashResponse{Hash: make([]byte, 32)}, nil
}
func (m *mockTxHashServiceClientForProvider) ValidateSignatureHash(ctx context.Context, in *pb.ValidateSignatureHashRequest, opts ...grpc.CallOption) (*pb.ValidateSignatureHashResponse, error) {
	return &pb.ValidateSignatureHashResponse{IsValid: true}, nil
}

// mockAddressManagerForProvider Mock的地址管理器
type mockAddressManagerForProvider struct{}

func (m *mockAddressManagerForProvider) PrivateKeyToAddress(privateKey []byte) (string, error) { return "test-address", nil }
func (m *mockAddressManagerForProvider) PublicKeyToAddress(publicKey []byte) (string, error) { return "test-address", nil }
func (m *mockAddressManagerForProvider) StringToAddress(addressStr string) (string, error) { return "test-address", nil }
func (m *mockAddressManagerForProvider) ValidateAddress(address string) (bool, error) { return true, nil }
func (m *mockAddressManagerForProvider) AddressToBytes(address string) ([]byte, error) { return make([]byte, 20), nil }
func (m *mockAddressManagerForProvider) BytesToAddress(addressBytes []byte) (string, error) { return "test-address", nil }
func (m *mockAddressManagerForProvider) AddressToHexString(address string) (string, error) { return "", nil }
func (m *mockAddressManagerForProvider) HexStringToAddress(hexStr string) (string, error) { return "", nil }
func (m *mockAddressManagerForProvider) GetAddressType(address string) (crypto.AddressType, error) { return crypto.AddressTypeBitcoin, nil }
func (m *mockAddressManagerForProvider) CompareAddresses(addr1, addr2 string) (bool, error) { return true, nil }
func (m *mockAddressManagerForProvider) IsZeroAddress(address string) bool { return false }

// mockChainQueryForProvider Mock的链查询服务
type mockChainQueryForProvider struct{}

func (m *mockChainQueryForProvider) GetChainInfo(ctx context.Context) (*types.ChainInfo, error) { return nil, nil }
func (m *mockChainQueryForProvider) GetCurrentHeight(ctx context.Context) (uint64, error) { return 100, nil }
func (m *mockChainQueryForProvider) GetBestBlockHash(ctx context.Context) ([]byte, error) { return make([]byte, 32), nil }
func (m *mockChainQueryForProvider) GetNodeMode(ctx context.Context) (types.NodeMode, error) { return types.NodeModeFull, nil }
func (m *mockChainQueryForProvider) IsDataFresh(ctx context.Context) (bool, error) { return true, nil }
func (m *mockChainQueryForProvider) IsReady(ctx context.Context) (bool, error) { return true, nil }
func (m *mockChainQueryForProvider) GetSyncStatus(ctx context.Context) (*types.SystemSyncStatus, error) { return nil, nil }

// mockBlockQueryForProvider Mock的区块查询服务
type mockBlockQueryForProvider struct{}

func (m *mockBlockQueryForProvider) GetBlockByHeight(ctx context.Context, height uint64) (*core.Block, error) { return nil, nil }
func (m *mockBlockQueryForProvider) GetBlockByHash(ctx context.Context, blockHash []byte) (*core.Block, error) { return nil, nil }
func (m *mockBlockQueryForProvider) GetBlockHeader(ctx context.Context, blockHash []byte) (*core.BlockHeader, error) { return nil, nil }
func (m *mockBlockQueryForProvider) GetBlockRange(ctx context.Context, startHeight, endHeight uint64) ([]*core.Block, error) { return nil, nil }
func (m *mockBlockQueryForProvider) GetHighestBlock(ctx context.Context) (height uint64, blockHash []byte, err error) { return 100, make([]byte, 32), nil }

// mockTxQueryForProvider Mock的交易查询服务
type mockTxQueryForProvider struct{}

func (m *mockTxQueryForProvider) GetTransaction(ctx context.Context, txHash []byte) (blockHash []byte, txIndex uint32, transaction *pb.Transaction, err error) { return nil, 0, nil, nil }
func (m *mockTxQueryForProvider) GetTxBlockHeight(ctx context.Context, txHash []byte) (uint64, error) { return 0, nil }
func (m *mockTxQueryForProvider) GetBlockTimestamp(ctx context.Context, height uint64) (int64, error) { return 0, nil }
func (m *mockTxQueryForProvider) GetAccountNonce(ctx context.Context, address []byte) (uint64, error) { return 0, nil }
func (m *mockTxQueryForProvider) GetTransactionsByBlock(ctx context.Context, blockHash []byte) ([]*pb.Transaction, error) { return nil, nil }

// mockResourceQueryForProvider Mock的资源查询服务
type mockResourceQueryForProvider struct{}

func (m *mockResourceQueryForProvider) GetResourceByContentHash(ctx context.Context, contentHash []byte) (*pb_resource.Resource, error) { return nil, nil }
func (m *mockResourceQueryForProvider) GetResourceFromBlockchain(ctx context.Context, contentHash []byte) (*pb_resource.Resource, bool, error) { return nil, false, nil }
func (m *mockResourceQueryForProvider) GetResourceTransaction(ctx context.Context, contentHash []byte) (txHash, blockHash []byte, blockHeight uint64, err error) { return nil, nil, 0, nil }
func (m *mockResourceQueryForProvider) CheckFileExists(contentHash []byte) bool { return false }
func (m *mockResourceQueryForProvider) BuildFilePath(contentHash []byte) string { return "" }
func (m *mockResourceQueryForProvider) ListResourceHashes(ctx context.Context, offset int, limit int) ([][]byte, error) { return nil, nil }

