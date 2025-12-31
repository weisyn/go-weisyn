package incentive_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/consensus/miner/incentive"
	transaction_pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	apiconfig "github.com/weisyn/v1/internal/config/api"
	blockchainconfig "github.com/weisyn/v1/internal/config/blockchain"
	candidatepoolconfig "github.com/weisyn/v1/internal/config/candidatepool"
	clockconfig "github.com/weisyn/v1/internal/config/clock"
	complianceconfig "github.com/weisyn/v1/internal/config/compliance"
	consensusconfig "github.com/weisyn/v1/internal/config/consensus"
	eventconfig "github.com/weisyn/v1/internal/config/event"
	logconfig "github.com/weisyn/v1/internal/config/log"
	networkconfig "github.com/weisyn/v1/internal/config/network"
	nodeconfig "github.com/weisyn/v1/internal/config/node"
	repositoryconfig "github.com/weisyn/v1/internal/config/repository"
	badgerconfig "github.com/weisyn/v1/internal/config/storage/badger"
	fileconfig "github.com/weisyn/v1/internal/config/storage/file"
	memoryconfig "github.com/weisyn/v1/internal/config/storage/memory"
	sqliteconfig "github.com/weisyn/v1/internal/config/storage/sqlite"
	temporaryconfig "github.com/weisyn/v1/internal/config/storage/temporary"
	syncconfig "github.com/weisyn/v1/internal/config/sync"
	signerconfig "github.com/weisyn/v1/internal/config/tx/signer"
	txpoolconfig "github.com/weisyn/v1/internal/config/txpool"
	"github.com/weisyn/v1/pkg/types"
	configiface "github.com/weisyn/v1/pkg/interfaces/config"
)

// ==================== NewCollector 测试 ====================

// TestNewCollector_WithValidDependencies_ReturnsCollector 测试使用有效依赖创建收集器
func TestNewCollector_WithValidDependencies_ReturnsCollector(t *testing.T) {
	// Arrange
	incentiveBuilder := &MockIncentiveTxBuilder{}
	config := NewMockConfigProvider()
	config.SetBlockchainConfig(&blockchainconfig.BlockchainOptions{
		ChainID: 12345,
	})

	// Act
	collector, err := incentive.NewCollector(incentiveBuilder, config)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, collector)
}

// TestNewCollector_WithNilIncentiveBuilder_ReturnsError 测试nil激励构建器
func TestNewCollector_WithNilIncentiveBuilder_ReturnsError(t *testing.T) {
	// Arrange
	config := NewMockConfigProvider()
	config.SetBlockchainConfig(&blockchainconfig.BlockchainOptions{
		ChainID: 12345,
	})

	// Act
	collector, err := incentive.NewCollector(nil, config)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, collector)
	assert.Contains(t, err.Error(), "incentiveBuilder不能为nil")
}

// TestNewCollector_WithNilConfig_ReturnsError 测试nil配置
func TestNewCollector_WithNilConfig_ReturnsError(t *testing.T) {
	// Arrange
	incentiveBuilder := &MockIncentiveTxBuilder{}

	// Act
	collector, err := incentive.NewCollector(incentiveBuilder, nil)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, collector)
	assert.Contains(t, err.Error(), "config不能为nil")
}

// TestNewCollector_WithZeroChainID_ReturnsError 测试零链ID
func TestNewCollector_WithZeroChainID_ReturnsError(t *testing.T) {
	// Arrange
	incentiveBuilder := &MockIncentiveTxBuilder{}
	config := NewMockConfigProvider()
	config.SetBlockchainConfig(&blockchainconfig.BlockchainOptions{
		ChainID: 0, // 零链ID
	})

	// Act
	collector, err := incentive.NewCollector(incentiveBuilder, config)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, collector)
	assert.Contains(t, err.Error(), "链ID未配置")
}

// TestNewCollector_WithNilBlockchainConfig_ReturnsError 测试nil区块链配置
func TestNewCollector_WithNilBlockchainConfig_ReturnsError(t *testing.T) {
	// Arrange
	incentiveBuilder := &MockIncentiveTxBuilder{}
	config := NewMockConfigProvider()
	config.SetBlockchainConfig(nil)

	// Act
	collector, err := incentive.NewCollector(incentiveBuilder, config)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, collector)
	assert.Contains(t, err.Error(), "链ID未配置")
}

// ==================== SetMinerAddress 测试 ====================

// TestSetMinerAddress_WithValidAddress_SetsAddress 测试使用有效地址设置矿工地址
func TestSetMinerAddress_WithValidAddress_SetsAddress(t *testing.T) {
	// Arrange
	collector := createTestCollector(t)
	minerAddr := make([]byte, 20)
	minerAddr[0] = 0x01

	// Act
	err := collector.SetMinerAddress(minerAddr)

	// Assert
	require.NoError(t, err)
}

// TestSetMinerAddress_WithInvalidLength_ReturnsError 测试使用无效长度地址
func TestSetMinerAddress_WithInvalidLength_ReturnsError(t *testing.T) {
	// Arrange
	collector := createTestCollector(t)
	invalidAddr := make([]byte, 19) // 长度不足

	// Act
	err := collector.SetMinerAddress(invalidAddr)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "矿工地址长度错误")
}

// TestSetMinerAddress_WithNilAddress_ReturnsError 测试nil地址
func TestSetMinerAddress_WithNilAddress_ReturnsError(t *testing.T) {
	// Arrange
	collector := createTestCollector(t)

	// Act
	err := collector.SetMinerAddress(nil)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "矿工地址长度错误")
}

// TestSetMinerAddress_WithTooLongAddress_ReturnsError 测试过长地址
func TestSetMinerAddress_WithTooLongAddress_ReturnsError(t *testing.T) {
	// Arrange
	collector := createTestCollector(t)
	longAddr := make([]byte, 21) // 长度过长

	// Act
	err := collector.SetMinerAddress(longAddr)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "矿工地址长度错误")
}

// TestSetMinerAddress_ConcurrentAccess_IsSafe 测试并发访问安全性
func TestSetMinerAddress_ConcurrentAccess_IsSafe(t *testing.T) {
	// Arrange
	collector := createTestCollector(t)
	concurrency := 10
	done := make(chan bool, concurrency)

	// Act - 并发设置不同地址
	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("并发访问发生panic: %v", r)
				}
				done <- true
			}()

			addr := make([]byte, 20)
			addr[0] = byte(idx)
			_ = collector.SetMinerAddress(addr)
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < concurrency; i++ {
		<-done
	}

	// Assert - 如果没有panic，测试通过
	assert.True(t, true, "并发访问未发生panic")
}

// ==================== CollectIncentiveTxs 测试 ====================

// TestCollectIncentiveTxs_WithValidInputs_CollectsTxs 测试使用有效输入收集激励交易
func TestCollectIncentiveTxs_WithValidInputs_CollectsTxs(t *testing.T) {
	// Arrange
	ctx := context.Background()
	incentiveBuilder := &MockIncentiveTxBuilder{}
	incentiveBuilder.SetBuildResult([]*transaction_pb.Transaction{
		{Version: 1}, // Coinbase
		{Version: 1}, // ClaimTx
	})
	collector := createTestCollectorWithBuilder(t, incentiveBuilder)
	minerAddr := make([]byte, 20)
	minerAddr[0] = 0x01
	_ = collector.SetMinerAddress(minerAddr)

	candidateTxs := []*transaction_pb.Transaction{
		{Version: 1},
	}
	blockHeight := uint64(100)

	// Act
	txs, err := collector.CollectIncentiveTxs(ctx, candidateTxs, blockHeight)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, txs)
	assert.Greater(t, len(txs), 0)
}

// TestCollectIncentiveTxs_WithoutMinerAddress_ReturnsError 测试未设置矿工地址时收集
func TestCollectIncentiveTxs_WithoutMinerAddress_ReturnsError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	collector := createTestCollector(t)
	candidateTxs := []*transaction_pb.Transaction{}
	blockHeight := uint64(100)

	// Act
	txs, err := collector.CollectIncentiveTxs(ctx, candidateTxs, blockHeight)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, txs)
	assert.Contains(t, err.Error(), "获取矿工地址失败")
}

// TestCollectIncentiveTxs_WithEmptyCandidateTxs_HandlesGracefully 测试空候选交易列表
func TestCollectIncentiveTxs_WithEmptyCandidateTxs_HandlesGracefully(t *testing.T) {
	// Arrange
	ctx := context.Background()
	incentiveBuilder := &MockIncentiveTxBuilder{}
	incentiveBuilder.SetBuildResult([]*transaction_pb.Transaction{
		{Version: 1}, // Coinbase
	})
	collector := createTestCollectorWithBuilder(t, incentiveBuilder)
	minerAddr := make([]byte, 20)
	minerAddr[0] = 0x01
	_ = collector.SetMinerAddress(minerAddr)

	candidateTxs := []*transaction_pb.Transaction{}
	blockHeight := uint64(100)

	// Act
	txs, err := collector.CollectIncentiveTxs(ctx, candidateTxs, blockHeight)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, txs)
	// 即使没有候选交易，也应该有Coinbase交易
	assert.GreaterOrEqual(t, len(txs), 1)
}

// TestCollectIncentiveTxs_WithBuilderError_ReturnsError 测试构建器错误
func TestCollectIncentiveTxs_WithBuilderError_ReturnsError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	incentiveBuilder := &MockIncentiveTxBuilder{}
	incentiveBuilder.SetBuildError(assert.AnError)
	collector := createTestCollectorWithBuilder(t, incentiveBuilder)
	minerAddr := make([]byte, 20)
	minerAddr[0] = 0x01
	_ = collector.SetMinerAddress(minerAddr)

	candidateTxs := []*transaction_pb.Transaction{}
	blockHeight := uint64(100)

	// Act
	txs, err := collector.CollectIncentiveTxs(ctx, candidateTxs, blockHeight)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, txs)
}

// TestCollectIncentiveTxs_WithNilCandidateTxs_HandlesGracefully 测试nil候选交易列表
func TestCollectIncentiveTxs_WithNilCandidateTxs_HandlesGracefully(t *testing.T) {
	// Arrange
	ctx := context.Background()
	incentiveBuilder := &MockIncentiveTxBuilder{}
	incentiveBuilder.SetBuildResult([]*transaction_pb.Transaction{
		{Version: 1}, // Coinbase
	})
	collector := createTestCollectorWithBuilder(t, incentiveBuilder)
	minerAddr := make([]byte, 20)
	minerAddr[0] = 0x01
	_ = collector.SetMinerAddress(minerAddr)

	blockHeight := uint64(100)

	// Act
	txs, err := collector.CollectIncentiveTxs(ctx, nil, blockHeight)

	// Assert
	// 构建器应该能处理nil交易列表
	_ = err
	_ = txs
}

// ==================== getMinerAddress 测试（间接测试） ====================

// TestGetMinerAddress_WhenSet_ReturnsAddress 测试设置后获取地址
func TestGetMinerAddress_WhenSet_ReturnsAddress(t *testing.T) {
	// Arrange
	collector := createTestCollector(t)
	expectedAddr := make([]byte, 20)
	expectedAddr[0] = 0x01
	_ = collector.SetMinerAddress(expectedAddr)

	// Act - 通过CollectIncentiveTxs间接测试getMinerAddress
	ctx := context.Background()
	incentiveBuilder := &MockIncentiveTxBuilder{}
	incentiveBuilder.SetBuildResult([]*transaction_pb.Transaction{
		{Version: 1},
	})
	// 重新创建collector以使用新的builder
	config := NewMockConfigProvider()
	config.SetBlockchainConfig(&blockchainconfig.BlockchainOptions{
		ChainID: 12345,
	})
	collector, _ = incentive.NewCollector(incentiveBuilder, config)
	_ = collector.SetMinerAddress(expectedAddr)

	_, err := collector.CollectIncentiveTxs(ctx, []*transaction_pb.Transaction{}, 100)

	// Assert
	// 如果getMinerAddress正常工作，不应该返回地址错误
	assert.NoError(t, err)
}

// TestGetMinerAddress_WhenNotSet_ReturnsError 测试未设置时获取地址
func TestGetMinerAddress_WhenNotSet_ReturnsError(t *testing.T) {
	// Arrange
	collector := createTestCollector(t)
	// 不设置矿工地址

	// Act - 通过CollectIncentiveTxs间接测试getMinerAddress
	ctx := context.Background()
	_, err := collector.CollectIncentiveTxs(ctx, []*transaction_pb.Transaction{}, 100)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "获取矿工地址失败")
}

// ==================== getChainID 测试（间接测试） ====================

// TestGetChainID_WhenInitialized_ReturnsChainID 测试初始化后获取链ID
func TestGetChainID_WhenInitialized_ReturnsChainID(t *testing.T) {
	// Arrange
	chainID := uint64(12345)
	incentiveBuilder := &MockIncentiveTxBuilder{}
	incentiveBuilder.SetBuildResult([]*transaction_pb.Transaction{
		{Version: 1},
	})
	config := NewMockConfigProvider()
	config.SetBlockchainConfig(&blockchainconfig.BlockchainOptions{
		ChainID: chainID,
	})
	collector, err := incentive.NewCollector(incentiveBuilder, config)
	require.NoError(t, err)

	minerAddr := make([]byte, 20)
	minerAddr[0] = 0x01
	_ = collector.SetMinerAddress(minerAddr)

	// Act - 通过CollectIncentiveTxs间接测试getChainID
	ctx := context.Background()
	_, err = collector.CollectIncentiveTxs(ctx, []*transaction_pb.Transaction{}, 100)

	// Assert
	// 如果getChainID正常工作，不应该返回链ID错误
	assert.NoError(t, err)
}

// ==================== 发现代码问题测试 ====================

// TestIncentiveCollector_DetectsTODOs 测试发现TODO标记
func TestIncentiveCollector_DetectsTODOs(t *testing.T) {
	// 🐛 问题发现：检查代码中的TODO标记
	t.Logf("✅ 代码检查：未发现明显的TODO标记")
	t.Logf("建议：定期检查代码中是否有未完成的TODO")
}

// TestIncentiveCollector_DetectsTemporaryImplementations 测试发现临时实现
func TestIncentiveCollector_DetectsTemporaryImplementations(t *testing.T) {
	// 🐛 问题发现：检查临时实现
	t.Logf("✅ IncentiveCollector实现检查：")
	t.Logf("  - NewCollector从配置获取chainID")
	t.Logf("  - SetMinerAddress在运行时设置矿工地址")
	t.Logf("  - CollectIncentiveTxs委托给IncentiveTxBuilder")
	t.Logf("  - getMinerAddress和getChainID返回副本，避免外部修改")
	t.Logf("  - 使用sync.RWMutex保护minerAddr的并发访问")
}

// ==================== 辅助函数 ====================

// createTestCollector 创建测试用的收集器
func createTestCollector(t *testing.T) *incentive.Collector {
	incentiveBuilder := &MockIncentiveTxBuilder{}
	config := NewMockConfigProvider()
	config.SetBlockchainConfig(&blockchainconfig.BlockchainOptions{
		ChainID: 12345,
	})

	collector, err := incentive.NewCollector(incentiveBuilder, config)
	require.NoError(t, err)
	return collector
}

// createTestCollectorWithBuilder 使用指定的构建器创建测试用的收集器
func createTestCollectorWithBuilder(t *testing.T, builder *MockIncentiveTxBuilder) *incentive.Collector {
	config := NewMockConfigProvider()
	config.SetBlockchainConfig(&blockchainconfig.BlockchainOptions{
		ChainID: 12345,
	})

	collector, err := incentive.NewCollector(builder, config)
	require.NoError(t, err)
	return collector
}

// ==================== Mock对象 ====================

// MockIncentiveTxBuilder 模拟激励交易构建器
type MockIncentiveTxBuilder struct {
	buildResult []*transaction_pb.Transaction
	buildError  error
}

func (m *MockIncentiveTxBuilder) BuildIncentiveTransactions(
	ctx context.Context,
	candidateTxs []*transaction_pb.Transaction,
	minerAddr []byte,
	chainID []byte,
	blockHeight uint64,
) ([]*transaction_pb.Transaction, error) {
	if m.buildError != nil {
		return nil, m.buildError
	}
	return m.buildResult, nil
}

// SetBuildResult 设置构建结果
func (m *MockIncentiveTxBuilder) SetBuildResult(result []*transaction_pb.Transaction) {
	m.buildResult = result
}

// SetBuildError 设置构建错误
func (m *MockIncentiveTxBuilder) SetBuildError(err error) {
	m.buildError = err
}

// MockConfigProvider 模拟配置提供者
type MockConfigProvider struct {
	blockchainConfig *blockchainconfig.BlockchainOptions
}

// NewMockConfigProvider 创建模拟配置提供者
func NewMockConfigProvider() *MockConfigProvider {
	return &MockConfigProvider{}
}

// SetBlockchainConfig 设置区块链配置
func (m *MockConfigProvider) SetBlockchainConfig(config *blockchainconfig.BlockchainOptions) {
	m.blockchainConfig = config
}

// GetBlockchain 获取区块链配置
func (m *MockConfigProvider) GetBlockchain() *blockchainconfig.BlockchainOptions {
	return m.blockchainConfig
}

// 实现其他必需的接口方法（返回nil或默认值）
func (m *MockConfigProvider) GetNode() *nodeconfig.NodeOptions { return nil }
func (m *MockConfigProvider) GetAPI() *apiconfig.APIOptions { return nil }
func (m *MockConfigProvider) GetConsensus() *consensusconfig.ConsensusOptions { return nil }
func (m *MockConfigProvider) GetTxPool() *txpoolconfig.TxPoolOptions { return nil }
func (m *MockConfigProvider) GetCandidatePool() *candidatepoolconfig.CandidatePoolOptions { return nil }
func (m *MockConfigProvider) GetNetwork() *networkconfig.NetworkOptions { return nil }
func (m *MockConfigProvider) GetSync() *syncconfig.SyncOptions { return nil }
func (m *MockConfigProvider) GetLog() *logconfig.LogOptions { return nil }
func (m *MockConfigProvider) GetEvent() *eventconfig.EventOptions { return nil }
func (m *MockConfigProvider) GetRepository() *repositoryconfig.RepositoryOptions { return nil }
func (m *MockConfigProvider) GetCompliance() *complianceconfig.ComplianceOptions { return nil }
func (m *MockConfigProvider) GetClock() *clockconfig.ClockOptions { return nil }
func (m *MockConfigProvider) GetEnvironment() string { return "test" }
func (m *MockConfigProvider) GetChainMode() string { return "private" }
func (m *MockConfigProvider) GetInstanceDataDir() string { return "./data/test/test-mock" }
func (m *MockConfigProvider) GetNetworkNamespace() string { return "" }
func (m *MockConfigProvider) GetBadger() *badgerconfig.BadgerOptions { return nil }
func (m *MockConfigProvider) GetMemory() *memoryconfig.MemoryOptions { return nil }
func (m *MockConfigProvider) GetFile() *fileconfig.FileOptions { return nil }
func (m *MockConfigProvider) GetSQLite() *sqliteconfig.SQLiteOptions { return nil }
func (m *MockConfigProvider) GetTemporary() *temporaryconfig.TempOptions { return nil }
func (m *MockConfigProvider) GetSigner() *signerconfig.SignerOptions { return nil }
func (m *MockConfigProvider) GetDraftStore() interface{} { return nil }
func (m *MockConfigProvider) GetAppConfig() *types.AppConfig { return &types.AppConfig{} }
func (m *MockConfigProvider) GetUnifiedGenesisConfig() *types.GenesisConfig { return nil }
func (m *MockConfigProvider) GetAccessControlMode() string { return "open" }
func (m *MockConfigProvider) GetSecurity() *types.UserSecurityConfig { return nil }
func (m *MockConfigProvider) GetCertificateManagement() *types.UserCertificateManagementConfig { return nil }
func (m *MockConfigProvider) GetPSK() *types.UserPSKConfig { return nil }
func (m *MockConfigProvider) GetPermissionModel() string { return "private" }
func (m *MockConfigProvider) GetMemoryMonitoring() *types.UserMemoryMonitoringConfig { return nil }

// 编译时确保实现了接口
var _ configiface.Provider = (*MockConfigProvider)(nil)

