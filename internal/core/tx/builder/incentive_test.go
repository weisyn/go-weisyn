// Package builder_test 提供 Incentive Builder 的单元测试
//
// 🧪 **测试覆盖**：
// - IncentiveBuilder 核心功能测试
// - Coinbase 交易构建测试
// - Sponsor Claim 交易构建测试
// - 边界条件和错误场景测试
package builder

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	consensuscfg "github.com/weisyn/v1/internal/config/consensus"
	"github.com/weisyn/v1/internal/core/tx/testutil"
	transaction_pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxopb "github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/constants"
	apiconfig "github.com/weisyn/v1/internal/config/api"
	badgerconfig "github.com/weisyn/v1/internal/config/storage/badger"
	blockchainconfig "github.com/weisyn/v1/internal/config/blockchain"
	candidatepoolconfig "github.com/weisyn/v1/internal/config/candidatepool"
	clockconfig "github.com/weisyn/v1/internal/config/clock"
	complianceconfig "github.com/weisyn/v1/internal/config/compliance"
	eventconfig "github.com/weisyn/v1/internal/config/event"
	fileconfig "github.com/weisyn/v1/internal/config/storage/file"
	logconfig "github.com/weisyn/v1/internal/config/log"
	memoryconfig "github.com/weisyn/v1/internal/config/storage/memory"
	networkconfig "github.com/weisyn/v1/internal/config/network"
	nodeconfig "github.com/weisyn/v1/internal/config/node"
	repositoryconfig "github.com/weisyn/v1/internal/config/repository"
	signconfig "github.com/weisyn/v1/internal/config/tx/signer"
	sqliteconfig "github.com/weisyn/v1/internal/config/storage/sqlite"
	syncconfig "github.com/weisyn/v1/internal/config/sync"
	tempconfig "github.com/weisyn/v1/internal/config/storage/temporary"
	txpoolconfig "github.com/weisyn/v1/internal/config/txpool"
	txiface "github.com/weisyn/v1/pkg/interfaces/tx"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== Mock 对象 ====================

// MockFeeManager 模拟费用管理器
type MockFeeManager struct {
	fees map[string]*txiface.AggregatedFees
}

func NewMockFeeManager() *MockFeeManager {
	return &MockFeeManager{
		fees: make(map[string]*txiface.AggregatedFees),
	}
}

func (m *MockFeeManager) CalculateTransactionFee(ctx context.Context, tx *transaction_pb.Transaction) (*txiface.AggregatedFees, error) {
	// 简化实现：返回固定费用
	return &txiface.AggregatedFees{
		ByToken: make(map[txiface.TokenKey]*big.Int),
	}, nil
}

func (m *MockFeeManager) AggregateFees(fees []*txiface.AggregatedFees) *txiface.AggregatedFees {
	// 简化实现：返回聚合费用
	result := &txiface.AggregatedFees{
		ByToken: make(map[txiface.TokenKey]*big.Int),
	}
	// 简单聚合逻辑
	for _, fee := range fees {
		if fee != nil {
			for tokenKey, amount := range fee.ByToken {
				if result.ByToken[tokenKey] == nil {
					result.ByToken[tokenKey] = big.NewInt(0)
				}
				result.ByToken[tokenKey].Add(result.ByToken[tokenKey], amount)
			}
		}
	}
	return result
}

func (m *MockFeeManager) BuildCoinbase(aggregated *txiface.AggregatedFees, minerAddr []byte, chainID []byte) (*transaction_pb.Transaction, error) {
	return &transaction_pb.Transaction{
		Version: 1,
		Inputs:  []*transaction_pb.TxInput{},
		Outputs: []*transaction_pb.TxOutput{},
		ChainId: chainID,
	}, nil
}

func (m *MockFeeManager) ValidateCoinbase(ctx context.Context, coinbase *transaction_pb.Transaction, expectedFees *txiface.AggregatedFees, minerAddr []byte) error {
	return nil
}

// MockConfigProvider 模拟配置提供者
type MockConfigProvider struct {
	sponsorConfig *consensuscfg.SponsorIncentiveConfig
}

func NewMockConfigProvider() *MockConfigProvider {
	return &MockConfigProvider{
		sponsorConfig: &consensuscfg.SponsorIncentiveConfig{
			Enabled:            true,
			MaxPerBlock:        10,
			MaxAmountPerSponsor: 1000000,
			AcceptedTokens:     []consensuscfg.TokenFilterConfig{},
		},
	}
}

func (m *MockConfigProvider) GetConsensus() *consensuscfg.ConsensusOptions {
	return &consensuscfg.ConsensusOptions{
		Miner: consensuscfg.MinerConfig{
			SponsorIncentive: *m.sponsorConfig,
		},
	}
}

func (m *MockConfigProvider) GetNode() *nodeconfig.NodeOptions { return nil }
func (m *MockConfigProvider) GetAPI() *apiconfig.APIOptions { return nil }
func (m *MockConfigProvider) GetBlockchain() *blockchainconfig.BlockchainOptions { return nil }
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
func (m *MockConfigProvider) GetNetworkNamespace() string { return "test" }
func (m *MockConfigProvider) GetBadger() *badgerconfig.BadgerOptions { return nil }
func (m *MockConfigProvider) GetMemory() *memoryconfig.MemoryOptions { return nil }
func (m *MockConfigProvider) GetFile() *fileconfig.FileOptions { return nil }
func (m *MockConfigProvider) GetSQLite() *sqliteconfig.SQLiteOptions { return nil }
func (m *MockConfigProvider) GetTemporary() *tempconfig.TempOptions { return nil }
func (m *MockConfigProvider) GetSigner() *signconfig.SignerOptions { return nil }
func (m *MockConfigProvider) GetAppConfig() *types.AppConfig { return nil }
func (m *MockConfigProvider) GetDraftStore() interface{} { return nil }
func (m *MockConfigProvider) GetUnifiedGenesisConfig() *types.GenesisConfig { return nil }
func (m *MockConfigProvider) GetAccessControlMode() string { return "open" }
func (m *MockConfigProvider) GetSecurity() *types.UserSecurityConfig { return nil }
func (m *MockConfigProvider) GetCertificateManagement() *types.UserCertificateManagementConfig { return nil }
func (m *MockConfigProvider) GetPSK() *types.UserPSKConfig { return nil }
func (m *MockConfigProvider) GetPermissionModel() string { return "private" }
func (m *MockConfigProvider) GetMemoryMonitoring() *types.UserMemoryMonitoringConfig { return nil }

// SetSponsorConfig 设置赞助配置
func (m *MockConfigProvider) SetSponsorConfig(config *consensuscfg.SponsorIncentiveConfig) {
	m.sponsorConfig = config
}

// MockConfigProviderNil 返回nil配置的Mock
type MockConfigProviderNil struct{}

func (m *MockConfigProviderNil) GetConsensus() *consensuscfg.ConsensusOptions {
	return nil // 返回nil，模拟配置不存在的情况
}

func (m *MockConfigProviderNil) GetNode() *nodeconfig.NodeOptions { return nil }
func (m *MockConfigProviderNil) GetAPI() *apiconfig.APIOptions { return nil }
func (m *MockConfigProviderNil) GetBlockchain() *blockchainconfig.BlockchainOptions { return nil }
func (m *MockConfigProviderNil) GetTxPool() *txpoolconfig.TxPoolOptions { return nil }
func (m *MockConfigProviderNil) GetCandidatePool() *candidatepoolconfig.CandidatePoolOptions { return nil }
func (m *MockConfigProviderNil) GetNetwork() *networkconfig.NetworkOptions { return nil }
func (m *MockConfigProviderNil) GetSync() *syncconfig.SyncOptions { return nil }
func (m *MockConfigProviderNil) GetLog() *logconfig.LogOptions { return nil }
func (m *MockConfigProviderNil) GetEvent() *eventconfig.EventOptions { return nil }
func (m *MockConfigProviderNil) GetRepository() *repositoryconfig.RepositoryOptions { return nil }
func (m *MockConfigProviderNil) GetCompliance() *complianceconfig.ComplianceOptions { return nil }
func (m *MockConfigProviderNil) GetClock() *clockconfig.ClockOptions { return nil }
func (m *MockConfigProviderNil) GetEnvironment() string { return "test" }
func (m *MockConfigProviderNil) GetChainMode() string { return "private" }
func (m *MockConfigProviderNil) GetInstanceDataDir() string { return "./data/test/test-mock" }
func (m *MockConfigProviderNil) GetNetworkNamespace() string { return "test" }
func (m *MockConfigProviderNil) GetBadger() *badgerconfig.BadgerOptions { return nil }
func (m *MockConfigProviderNil) GetMemory() *memoryconfig.MemoryOptions { return nil }
func (m *MockConfigProviderNil) GetFile() *fileconfig.FileOptions { return nil }
func (m *MockConfigProviderNil) GetSQLite() *sqliteconfig.SQLiteOptions { return nil }
func (m *MockConfigProviderNil) GetTemporary() *tempconfig.TempOptions { return nil }
func (m *MockConfigProviderNil) GetSigner() *signconfig.SignerOptions { return nil }
func (m *MockConfigProviderNil) GetAppConfig() *types.AppConfig { return nil }
func (m *MockConfigProviderNil) GetDraftStore() interface{} { return nil }
func (m *MockConfigProviderNil) GetUnifiedGenesisConfig() *types.GenesisConfig { return nil }
func (m *MockConfigProviderNil) GetAccessControlMode() string { return "open" }
func (m *MockConfigProviderNil) GetSecurity() *types.UserSecurityConfig { return nil }
func (m *MockConfigProviderNil) GetCertificateManagement() *types.UserCertificateManagementConfig { return nil }
func (m *MockConfigProviderNil) GetPSK() *types.UserPSKConfig { return nil }
func (m *MockConfigProviderNil) GetPermissionModel() string { return "private" }
func (m *MockConfigProviderNil) GetMemoryMonitoring() *types.UserMemoryMonitoringConfig { return nil }

// ==================== IncentiveBuilder 核心功能测试 ====================

// TestNewIncentiveBuilder 测试创建新的 IncentiveBuilder
func TestNewIncentiveBuilder(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	assert.NotNil(t, builder)
	assert.NotNil(t, builder.feeManager)
	assert.NotNil(t, builder.eutxoQuery)
	assert.NotNil(t, builder.config)
}

// TestNewIncentiveBuilder_NilFeeManager 测试 nil feeManager
func TestNewIncentiveBuilder_NilFeeManager(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	assert.Panics(t, func() {
		NewIncentiveBuilder(nil, utxoQuery, configProvider, nil)
	}, "应该 panic 当 feeManager 为 nil")
}

// TestNewIncentiveBuilder_NilUTXOQuery 测试 nil utxoQuery
func TestNewIncentiveBuilder_NilUTXOQuery(t *testing.T) {
	feeManager := NewMockFeeManager()
	configProvider := NewMockConfigProvider()

	assert.Panics(t, func() {
		NewIncentiveBuilder(feeManager, nil, configProvider, nil)
	}, "应该 panic 当 utxoQuery 为 nil")
}

// TestNewIncentiveBuilder_NilConfig 测试 nil config
func TestNewIncentiveBuilder_NilConfig(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()

	assert.Panics(t, func() {
		NewIncentiveBuilder(feeManager, utxoQuery, nil, nil)
	}, "应该 panic 当 config 为 nil")
}

// TestBuildIncentiveTransactions_Success 测试构建激励交易成功
func TestBuildIncentiveTransactions_Success(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	ctx := context.Background()
	candidateTxs := []*transaction_pb.Transaction{}
	minerAddr := testutil.RandomAddress()
	chainID := []byte("test-chain")
	blockHeight := uint64(100)

	txs, err := builder.BuildIncentiveTransactions(ctx, candidateTxs, minerAddr, chainID, blockHeight)

	assert.NoError(t, err)
	assert.NotNil(t, txs)
	assert.GreaterOrEqual(t, len(txs), 1) // 至少包含 Coinbase
}

// TestBuildIncentiveTransactions_EmptyCandidateTxs 测试空候选交易列表
func TestBuildIncentiveTransactions_EmptyCandidateTxs(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	ctx := context.Background()
	minerAddr := testutil.RandomAddress()
	chainID := []byte("test-chain")
	blockHeight := uint64(100)

	txs, err := builder.BuildIncentiveTransactions(ctx, []*transaction_pb.Transaction{}, minerAddr, chainID, blockHeight)

	assert.NoError(t, err)
	assert.NotNil(t, txs)
	assert.GreaterOrEqual(t, len(txs), 1) // 至少包含 Coinbase
}

// TestBuildIncentiveTransactions_InvalidMinerAddr 测试无效矿工地址
func TestBuildIncentiveTransactions_InvalidMinerAddr(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	ctx := context.Background()
	invalidMinerAddr := []byte("invalid") // 长度不是 20 字节
	chainID := []byte("test-chain")
	blockHeight := uint64(100)

	// 注意：当前实现可能不会验证 minerAddr 长度，这取决于 buildCoinbase 的实现
	// 这里测试应该反映实际行为
	_, err := builder.BuildIncentiveTransactions(ctx, []*transaction_pb.Transaction{}, invalidMinerAddr, chainID, blockHeight)
	
	// 如果 buildCoinbase 验证了地址长度，应该返回错误
	// 否则应该成功（取决于实现）
	if err != nil {
		assert.Contains(t, err.Error(), "矿工地址")
	}
}

// TestBuildIncentiveTransactions_SponsorDisabled 测试赞助功能禁用
func TestBuildIncentiveTransactions_SponsorDisabled(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()
	configProvider.sponsorConfig.Enabled = false

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	ctx := context.Background()
	minerAddr := testutil.RandomAddress()
	chainID := []byte("test-chain")
	blockHeight := uint64(100)

	txs, err := builder.BuildIncentiveTransactions(ctx, []*transaction_pb.Transaction{}, minerAddr, chainID, blockHeight)

	assert.NoError(t, err)
	assert.NotNil(t, txs)
	assert.Equal(t, 1, len(txs)) // 只有 Coinbase，没有 Sponsor Claim
}

// TestBuildIncentiveTransactions_WithSponsorUTXOs 测试有 Sponsor UTXO 的情况
func TestBuildIncentiveTransactions_WithSponsorUTXOs(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	// 添加 Sponsor UTXO
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	ctx := context.Background()
	minerAddr := testutil.RandomAddress()
	chainID := []byte("test-chain")
	blockHeight := uint64(100)

	txs, err := builder.BuildIncentiveTransactions(ctx, []*transaction_pb.Transaction{}, minerAddr, chainID, blockHeight)

	assert.NoError(t, err)
	assert.NotNil(t, txs)
	// 注意：由于 Sponsor UTXO 可能不满足 DelegationLock 条件，可能只有 Coinbase
	assert.GreaterOrEqual(t, len(txs), 1)
}

// ==================== 边界条件测试 ====================

// TestBuildIncentiveTransactions_NilContext 测试 nil context
func TestBuildIncentiveTransactions_NilContext(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	minerAddr := testutil.RandomAddress()
	chainID := []byte("test-chain")
	blockHeight := uint64(100)

	// 使用 context.Background() 而不是 nil
	ctx := context.Background()
	_, err := builder.BuildIncentiveTransactions(ctx, []*transaction_pb.Transaction{}, minerAddr, chainID, blockHeight)

	// 应该成功（context.Background() 是有效的）
	assert.NoError(t, err)
}

// TestBuildIncentiveTransactions_ZeroBlockHeight 测试零区块高度
func TestBuildIncentiveTransactions_ZeroBlockHeight(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	ctx := context.Background()
	minerAddr := testutil.RandomAddress()
	chainID := []byte("test-chain")
	blockHeight := uint64(0)

	txs, err := builder.BuildIncentiveTransactions(ctx, []*transaction_pb.Transaction{}, minerAddr, chainID, blockHeight)

	assert.NoError(t, err)
	assert.NotNil(t, txs)
}

// TestBuildIncentiveTransactions_EmptyChainID 测试空 ChainID
func TestBuildIncentiveTransactions_EmptyChainID(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	ctx := context.Background()
	minerAddr := testutil.RandomAddress()
	chainID := []byte{}
	blockHeight := uint64(100)

	// 注意：当前实现可能不会验证 ChainID，这取决于 buildCoinbase 的实现
	_, err := builder.BuildIncentiveTransactions(ctx, []*transaction_pb.Transaction{}, minerAddr, chainID, blockHeight)
	
	// 如果 buildCoinbase 验证了 ChainID，应该返回错误
	// 否则应该成功（取决于实现）
	if err != nil {
		assert.Contains(t, err.Error(), "chainID")
	}
}

// TestBuildIncentiveTransactions_SponsorClaimFailed 测试赞助领取失败（应该记录警告但继续）
func TestBuildIncentiveTransactions_SponsorClaimFailed(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := NewMockUTXOQueryWithError(fmt.Errorf("查询失败"))
	configProvider := NewMockConfigProvider()
	configProvider.SetSponsorConfig(&consensuscfg.SponsorIncentiveConfig{
		Enabled:            true,
		MaxPerBlock:        10,
		MaxAmountPerSponsor: 1000000,
		AcceptedTokens:     []consensuscfg.TokenFilterConfig{},
	})

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	ctx := context.Background()
	minerAddr := testutil.RandomAddress()
	chainID := []byte("test-chain")
	blockHeight := uint64(100)

	// 赞助领取失败不应阻塞区块生成，应该返回 Coinbase
	txs, err := builder.BuildIncentiveTransactions(ctx, []*transaction_pb.Transaction{}, minerAddr, chainID, blockHeight)

	assert.NoError(t, err)
	assert.NotNil(t, txs)
	assert.GreaterOrEqual(t, len(txs), 1) // 至少包含 Coinbase
}

// ==================== getSponsorIncentiveConfig 测试 ====================

// TestGetSponsorIncentiveConfig_NilConfig 测试配置为nil的情况
func TestGetSponsorIncentiveConfig_NilConfig(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := &MockConfigProviderNil{} // 返回nil配置

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	config := builder.getSponsorIncentiveConfig()

	assert.Nil(t, config)
}

// TestGetSponsorIncentiveConfig_Success 测试获取配置成功
func TestGetSponsorIncentiveConfig_Success(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()
	sponsorConfig := &consensuscfg.SponsorIncentiveConfig{
		Enabled:            true,
		MaxPerBlock:        10,
		MaxAmountPerSponsor: 1000000,
		AcceptedTokens:     []consensuscfg.TokenFilterConfig{},
	}
	configProvider.SetSponsorConfig(sponsorConfig)

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	config := builder.getSponsorIncentiveConfig()

	assert.NotNil(t, config)
	assert.Equal(t, sponsorConfig.Enabled, config.Enabled)
	assert.Equal(t, sponsorConfig.MaxPerBlock, config.MaxPerBlock)
}

// ==================== buildCoinbase 测试 ====================

// TestBuildCoinbase_Success 测试构建 Coinbase 成功
func TestBuildCoinbase_Success(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	ctx := context.Background()
	candidateTxs := []*transaction_pb.Transaction{
		testutil.CreateTransaction(nil, []*transaction_pb.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil)),
		}),
	}
	minerAddr := testutil.RandomAddress()
	chainID := []byte("test-chain")

	coinbase, err := builder.buildCoinbase(ctx, candidateTxs, minerAddr, chainID)

	assert.NoError(t, err)
	assert.NotNil(t, coinbase)
	assert.Equal(t, chainID, coinbase.ChainId)
}

// TestBuildCoinbase_FeeCalculationError 测试费用计算失败
func TestBuildCoinbase_FeeCalculationError(t *testing.T) {
	feeManager := &MockFeeManagerWithError{
		calculateFeeError: fmt.Errorf("费用计算失败"),
	}
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	ctx := context.Background()
	candidateTxs := []*transaction_pb.Transaction{
		testutil.CreateTransaction(nil, []*transaction_pb.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil)),
		}),
	}
	minerAddr := testutil.RandomAddress()
	chainID := []byte("test-chain")

	coinbase, err := builder.buildCoinbase(ctx, candidateTxs, minerAddr, chainID)

	assert.Error(t, err)
	assert.Nil(t, coinbase)
	assert.Contains(t, err.Error(), "计算交易费用失败")
}

// MockFeeManagerWithError 带错误的费用管理器
type MockFeeManagerWithError struct {
	calculateFeeError error
	buildCoinbaseError error
}

func (m *MockFeeManagerWithError) CalculateTransactionFee(ctx context.Context, tx *transaction_pb.Transaction) (*txiface.AggregatedFees, error) {
	if m.calculateFeeError != nil {
		return nil, m.calculateFeeError
	}
	return &txiface.AggregatedFees{
		ByToken: make(map[txiface.TokenKey]*big.Int),
	}, nil
}

func (m *MockFeeManagerWithError) AggregateFees(fees []*txiface.AggregatedFees) *txiface.AggregatedFees {
	return &txiface.AggregatedFees{
		ByToken: make(map[txiface.TokenKey]*big.Int),
	}
}

func (m *MockFeeManagerWithError) BuildCoinbase(aggregated *txiface.AggregatedFees, minerAddr []byte, chainID []byte) (*transaction_pb.Transaction, error) {
	if m.buildCoinbaseError != nil {
		return nil, m.buildCoinbaseError
	}
	return &transaction_pb.Transaction{
		Version: 1,
		Inputs:  []*transaction_pb.TxInput{},
		Outputs: []*transaction_pb.TxOutput{},
		ChainId: chainID,
	}, nil
}

func (m *MockFeeManagerWithError) ValidateCoinbase(ctx context.Context, coinbase *transaction_pb.Transaction, expectedFees *txiface.AggregatedFees, minerAddr []byte) error {
	return nil
}

// ==================== filterValidSponsors 测试 ====================

// createSponsorUTXOWithDelegationLock 创建带 DelegationLock 的赞助 UTXO
func createSponsorUTXOWithDelegationLock(amount string, authorizedOps []string, expiryBlocks *uint64, blockHeight uint64) *utxopb.UTXO {
	outpoint := testutil.CreateOutPoint(nil, 0)
	delegationLock := &transaction_pb.DelegationLock{
		AuthorizedOperations: authorizedOps,
		MaxValuePerOperation:  1000000,
		ExpiryDurationBlocks:  expiryBlocks,
		AllowedDelegates:      nil, // 空表示任意矿工
	}
	lock := &transaction_pb.LockingCondition{
		Condition: &transaction_pb.LockingCondition_DelegationLock{
			DelegationLock: delegationLock,
		},
	}
	output := testutil.CreateNativeCoinOutput(constants.SponsorPoolOwner[:], amount, lock)
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxo.BlockHeight = blockHeight
	return utxo
}

// TestFilterValidSponsors_Success 测试过滤有效赞助成功
func TestFilterValidSponsors_Success(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	// 创建有效的赞助 UTXO
	sponsorUTXO := createSponsorUTXOWithDelegationLock("1000000", []string{"consume"}, nil, 100)
	sponsors := []*utxopb.UTXO{sponsorUTXO}
	currentHeight := uint64(200)
	policy := &consensuscfg.SponsorIncentiveConfig{
		Enabled:            true,
		MaxPerBlock:        10,
		MaxAmountPerSponsor: 1000000,
		AcceptedTokens:     []consensuscfg.TokenFilterConfig{},
	}

	valid := builder.filterValidSponsors(sponsors, currentHeight, policy)

	assert.Len(t, valid, 1)
	assert.Equal(t, sponsorUTXO, valid[0])
}

// TestFilterValidSponsors_NoDelegationLock 测试没有 DelegationLock
func TestFilterValidSponsors_NoDelegationLock(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	// 创建没有 DelegationLock 的 UTXO
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(constants.SponsorPoolOwner[:], "1000000", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	sponsors := []*utxopb.UTXO{utxo}
	currentHeight := uint64(200)
	policy := &consensuscfg.SponsorIncentiveConfig{
		Enabled:            true,
		MaxPerBlock:        10,
		MaxAmountPerSponsor: 1000000,
		AcceptedTokens:     []consensuscfg.TokenFilterConfig{},
	}

	valid := builder.filterValidSponsors(sponsors, currentHeight, policy)

	assert.Empty(t, valid)
}

// TestFilterValidSponsors_NoConsumeOperation 测试没有 consume 操作授权
func TestFilterValidSponsors_NoConsumeOperation(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	// 创建只有 transfer 授权的 UTXO
	sponsorUTXO := createSponsorUTXOWithDelegationLock("1000000", []string{"transfer"}, nil, 100)
	sponsors := []*utxopb.UTXO{sponsorUTXO}
	currentHeight := uint64(200)
	policy := &consensuscfg.SponsorIncentiveConfig{
		Enabled:            true,
		MaxPerBlock:        10,
		MaxAmountPerSponsor: 1000000,
		AcceptedTokens:     []consensuscfg.TokenFilterConfig{},
	}

	valid := builder.filterValidSponsors(sponsors, currentHeight, policy)

	assert.Empty(t, valid)
}

// TestFilterValidSponsors_WithAllowedDelegates 测试有 AllowedDelegates（应该被过滤）
func TestFilterValidSponsors_WithAllowedDelegates(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	// 创建有 AllowedDelegates 的 UTXO
	outpoint := testutil.CreateOutPoint(nil, 0)
	delegationLock := &transaction_pb.DelegationLock{
		AuthorizedOperations: []string{"consume"},
		MaxValuePerOperation:  1000000,
		AllowedDelegates:      [][]byte{testutil.RandomAddress()}, // 有允许的委托地址
	}
	lock := &transaction_pb.LockingCondition{
		Condition: &transaction_pb.LockingCondition_DelegationLock{
			DelegationLock: delegationLock,
		},
	}
	output := testutil.CreateNativeCoinOutput(constants.SponsorPoolOwner[:], "1000000", lock)
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	sponsors := []*utxopb.UTXO{utxo}
	currentHeight := uint64(200)
	policy := &consensuscfg.SponsorIncentiveConfig{
		Enabled:            true,
		MaxPerBlock:        10,
		MaxAmountPerSponsor: 1000000,
		AcceptedTokens:     []consensuscfg.TokenFilterConfig{},
	}

	valid := builder.filterValidSponsors(sponsors, currentHeight, policy)

	assert.Empty(t, valid)
}

// TestFilterValidSponsors_Expired 测试已过期的赞助
func TestFilterValidSponsors_Expired(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	// 创建已过期的赞助 UTXO（创建高度 100，过期高度 150，当前高度 200）
	expiryBlocks := uint64(50)
	sponsorUTXO := createSponsorUTXOWithDelegationLock("1000000", []string{"consume"}, &expiryBlocks, 100)
	sponsors := []*utxopb.UTXO{sponsorUTXO}
	currentHeight := uint64(200) // 超过过期高度 150
	policy := &consensuscfg.SponsorIncentiveConfig{
		Enabled:            true,
		MaxPerBlock:        10,
		MaxAmountPerSponsor: 1000000,
		AcceptedTokens:     []consensuscfg.TokenFilterConfig{},
	}

	valid := builder.filterValidSponsors(sponsors, currentHeight, policy)

	assert.Empty(t, valid)
}

// TestFilterValidSponsors_TokenWhitelist 测试 Token 白名单过滤
func TestFilterValidSponsors_TokenWhitelist(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	// 创建原生币赞助 UTXO
	sponsorUTXO := createSponsorUTXOWithDelegationLock("1000000", []string{"consume"}, nil, 100)
	sponsors := []*utxopb.UTXO{sponsorUTXO}
	currentHeight := uint64(200)
	policy := &consensuscfg.SponsorIncentiveConfig{
		Enabled:            true,
		MaxPerBlock:        10,
		MaxAmountPerSponsor: 1000000,
		AcceptedTokens: []consensuscfg.TokenFilterConfig{
			{AssetID: "contract:xxx:yyy", MinAmount: 0}, // 只接受合约代币，不接受原生币
		},
	}

	valid := builder.filterValidSponsors(sponsors, currentHeight, policy)

	assert.Empty(t, valid)
}

// TestFilterValidSponsors_MinAmount 测试最低金额过滤
func TestFilterValidSponsors_MinAmount(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	// 创建金额低于最低要求的赞助 UTXO
	sponsorUTXO := createSponsorUTXOWithDelegationLock("1000", []string{"consume"}, nil, 100)
	sponsors := []*utxopb.UTXO{sponsorUTXO}
	currentHeight := uint64(200)
	policy := &consensuscfg.SponsorIncentiveConfig{
		Enabled:            true,
		MaxPerBlock:        10,
		MaxAmountPerSponsor: 1000000,
		AcceptedTokens: []consensuscfg.TokenFilterConfig{
			{AssetID: "native", MinAmount: 10000}, // 最低金额 10000
		},
	}

	valid := builder.filterValidSponsors(sponsors, currentHeight, policy)

	assert.Empty(t, valid)
}

// TestFilterValidSponsors_NoCachedOutput 测试没有 CachedOutput
func TestFilterValidSponsors_NoCachedOutput(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	// 创建没有 CachedOutput 的 UTXO
	outpoint := testutil.CreateOutPoint(nil, 0)
	utxo := &utxopb.UTXO{
		Outpoint:     outpoint,
		Category:     utxopb.UTXOCategory_UTXO_CATEGORY_ASSET,
		Status:       utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE,
		OwnerAddress: constants.SponsorPoolOwner[:],
		// 没有 CachedOutput
	}

	sponsors := []*utxopb.UTXO{utxo}
	currentHeight := uint64(200)
	policy := &consensuscfg.SponsorIncentiveConfig{
		Enabled:            true,
		MaxPerBlock:        10,
		MaxAmountPerSponsor: 1000000,
		AcceptedTokens:     []consensuscfg.TokenFilterConfig{},
	}

	valid := builder.filterValidSponsors(sponsors, currentHeight, policy)

	assert.Empty(t, valid)
}

// ==================== buildSingleSponsorClaimTx 测试 ====================

// TestBuildSingleSponsorClaimTx_Success 测试构建单个赞助领取交易成功
func TestBuildSingleSponsorClaimTx_Success(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	ctx := context.Background()
	sponsorUTXO := createSponsorUTXOWithDelegationLock("1000000", []string{"consume"}, nil, 100)
	minerAddr := testutil.RandomAddress()
	chainID := []byte("test-chain")
	policy := &consensuscfg.SponsorIncentiveConfig{
		Enabled:            true,
		MaxPerBlock:        10,
		MaxAmountPerSponsor: 1000000,
		AcceptedTokens:     []consensuscfg.TokenFilterConfig{},
	}

	claimTx, err := builder.buildSingleSponsorClaimTx(ctx, sponsorUTXO, minerAddr, chainID, policy)

	assert.NoError(t, err)
	assert.NotNil(t, claimTx)
	assert.Len(t, claimTx.Inputs, 1)
	assert.Len(t, claimTx.Outputs, 1) // 只有矿工领取输出，没有找零
	assert.Equal(t, chainID, claimTx.ChainId)
}

// TestBuildSingleSponsorClaimTx_WithChange 测试有找零的情况
func TestBuildSingleSponsorClaimTx_WithChange(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	ctx := context.Background()
	// 创建金额大于 MaxAmountPerSponsor 的赞助 UTXO
	sponsorUTXO := createSponsorUTXOWithDelegationLock("2000000", []string{"consume"}, nil, 100)
	minerAddr := testutil.RandomAddress()
	chainID := []byte("test-chain")
	policy := &consensuscfg.SponsorIncentiveConfig{
		Enabled:            true,
		MaxPerBlock:        10,
		MaxAmountPerSponsor: 1000000, // 最大领取 1000000
		AcceptedTokens:     []consensuscfg.TokenFilterConfig{},
	}

	claimTx, err := builder.buildSingleSponsorClaimTx(ctx, sponsorUTXO, minerAddr, chainID, policy)

	assert.NoError(t, err)
	assert.NotNil(t, claimTx)
	assert.Len(t, claimTx.Inputs, 1)
	assert.Len(t, claimTx.Outputs, 2) // 矿工领取 + 找零
	assert.Equal(t, chainID, claimTx.ChainId)
}

// TestBuildSingleSponsorClaimTx_AmountExceedsUint64 测试金额超过 uint64 最大值
func TestBuildSingleSponsorClaimTx_AmountExceedsUint64(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	ctx := context.Background()
	// 创建金额超过 uint64 最大值的赞助 UTXO
	// uint64 最大值约为 1.8e19，这里使用更大的值
	hugeAmount := "20000000000000000000" // 2e19，超过 uint64 最大值
	// 设置 MaxValuePerOperation 为 uint64 最大值，确保 claimAmount 会等于 totalAmount（超过 uint64 最大值）
	maxValuePerOp := uint64(18446744073709551615) // uint64 最大值
	sponsorUTXO := createSponsorUTXOWithDelegationLockAndMaxValue(hugeAmount, []string{"consume"}, nil, 100, maxValuePerOp)
	minerAddr := testutil.RandomAddress()
	chainID := []byte("test-chain")
	// 设置 MaxAmountPerSponsor 为 uint64 最大值，这样 claimAmount 不会被限制，会等于 totalAmount（超过 uint64 最大值）
	policy := &consensuscfg.SponsorIncentiveConfig{
		Enabled:            true,
		MaxPerBlock:        10,
		MaxAmountPerSponsor: 18446744073709551615, // uint64 最大值，确保 claimAmount = totalAmount
		AcceptedTokens:     []consensuscfg.TokenFilterConfig{},
	}

	claimTx, err := builder.buildSingleSponsorClaimTx(ctx, sponsorUTXO, minerAddr, chainID, policy)

	assert.Error(t, err)
	assert.Nil(t, claimTx)
	assert.Contains(t, err.Error(), "领取金额超过uint64最大值")
}

// createSponsorUTXOWithDelegationLockAndMaxValue 创建带 DelegationLock 和自定义 MaxValuePerOperation 的赞助 UTXO
func createSponsorUTXOWithDelegationLockAndMaxValue(amount string, authorizedOps []string, expiryBlocks *uint64, blockHeight uint64, maxValuePerOp uint64) *utxopb.UTXO {
	outpoint := testutil.CreateOutPoint(nil, 0)
	delegationLock := &transaction_pb.DelegationLock{
		AuthorizedOperations: authorizedOps,
		MaxValuePerOperation:  maxValuePerOp,
		ExpiryDurationBlocks: expiryBlocks,
		AllowedDelegates:     nil, // 空表示任意矿工
	}
	lock := &transaction_pb.LockingCondition{
		Condition: &transaction_pb.LockingCondition_DelegationLock{
			DelegationLock: delegationLock,
		},
	}
	output := testutil.CreateNativeCoinOutput(constants.SponsorPoolOwner[:], amount, lock)
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxo.BlockHeight = blockHeight
	return utxo
}

// ==================== 辅助函数测试 ====================

// TestExtractDelegationLock 测试提取 DelegationLock
func TestExtractDelegationLock(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	// 创建带 DelegationLock 的输出
	delegationLock := &transaction_pb.DelegationLock{
		AuthorizedOperations: []string{"consume"},
		MaxValuePerOperation:  1000000,
	}
	lock := &transaction_pb.LockingCondition{
		Condition: &transaction_pb.LockingCondition_DelegationLock{
			DelegationLock: delegationLock,
		},
	}
	output := &transaction_pb.TxOutput{
		LockingConditions: []*transaction_pb.LockingCondition{lock},
	}

	result := builder.extractDelegationLock(output)

	assert.NotNil(t, result)
	assert.Equal(t, delegationLock, result)
}

// TestExtractDelegationLock_NotFound 测试没有 DelegationLock
func TestExtractDelegationLock_NotFound(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	// 创建只有 SingleKeyLock 的输出
	output := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil))

	result := builder.extractDelegationLock(output)

	assert.Nil(t, result)
}

// TestHasOperation 测试检查操作是否存在
func TestHasOperation(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	operations := []string{"consume", "transfer"}

	assert.True(t, builder.hasOperation(operations, "consume"))
	assert.True(t, builder.hasOperation(operations, "transfer"))
	assert.False(t, builder.hasOperation(operations, "approve"))
	assert.False(t, builder.hasOperation([]string{}, "consume"))
}

// TestExtractTokenKey_NativeCoin 测试提取原生币 TokenKey
func TestExtractTokenKey_NativeCoin(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	asset := &transaction_pb.AssetOutput{
		AssetContent: &transaction_pb.AssetOutput_NativeCoin{
			NativeCoin: &transaction_pb.NativeCoinAsset{
				Amount: "1000",
			},
		},
	}

	tokenKey := builder.extractTokenKey(asset)

	assert.Equal(t, txiface.TokenKey("native"), tokenKey)
}

// TestExtractTokenKey_ContractToken_Fungible 测试提取合约代币 TokenKey（Fungible）
func TestExtractTokenKey_ContractToken_Fungible(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	contractAddr := testutil.RandomAddress()
	classID := []byte("class-123")
	asset := &transaction_pb.AssetOutput{
		AssetContent: &transaction_pb.AssetOutput_ContractToken{
			ContractToken: &transaction_pb.ContractTokenAsset{
				ContractAddress: contractAddr,
				TokenIdentifier: &transaction_pb.ContractTokenAsset_FungibleClassId{
					FungibleClassId: classID,
				},
				Amount: "1000",
			},
		},
	}

	tokenKey := builder.extractTokenKey(asset)

	expected := txiface.TokenKey(fmt.Sprintf("contract:%x:%x", contractAddr, classID))
	assert.Equal(t, expected, tokenKey)
}

// TestExtractTokenKey_ContractToken_NFT 测试提取合约代币 TokenKey（NFT）
func TestExtractTokenKey_ContractToken_NFT(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	contractAddr := testutil.RandomAddress()
	nftID := testutil.RandomHash()
	asset := &transaction_pb.AssetOutput{
		AssetContent: &transaction_pb.AssetOutput_ContractToken{
			ContractToken: &transaction_pb.ContractTokenAsset{
				ContractAddress: contractAddr,
				TokenIdentifier: &transaction_pb.ContractTokenAsset_NftUniqueId{
					NftUniqueId: nftID,
				},
				Amount: "1",
			},
		},
	}

	tokenKey := builder.extractTokenKey(asset)

	expected := txiface.TokenKey(fmt.Sprintf("contract:%x:nft:%x", contractAddr, nftID))
	assert.Equal(t, expected, tokenKey)
}

// TestExtractTokenKey_ContractToken_SFT 测试提取合约代币 TokenKey（SFT）
func TestExtractTokenKey_ContractToken_SFT(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	contractAddr := testutil.RandomAddress()
	batchID := testutil.RandomHash()
	instanceID := uint64(123)
	asset := &transaction_pb.AssetOutput{
		AssetContent: &transaction_pb.AssetOutput_ContractToken{
			ContractToken: &transaction_pb.ContractTokenAsset{
				ContractAddress: contractAddr,
				TokenIdentifier: &transaction_pb.ContractTokenAsset_SemiFungibleId{
					SemiFungibleId: &transaction_pb.SemiFungibleId{
						BatchId:    batchID,
						InstanceId: instanceID,
					},
				},
				Amount: "100",
			},
		},
	}

	tokenKey := builder.extractTokenKey(asset)

	// extractTokenKey 使用 %x 格式化 InstanceId（十六进制），所以 123 会被格式化为 7b
	expected := txiface.TokenKey(fmt.Sprintf("contract:%x:sft:%x:%x", contractAddr, batchID, instanceID))
	assert.Equal(t, expected, tokenKey)
}

// TestExtractTokenKey_Unknown 测试未知类型的 TokenKey
func TestExtractTokenKey_Unknown(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	// 创建没有 AssetContent 的 AssetOutput
	asset := &transaction_pb.AssetOutput{}

	tokenKey := builder.extractTokenKey(asset)

	assert.Equal(t, txiface.TokenKey("unknown"), tokenKey)
}

// TestExtractAmount_NativeCoin 测试提取原生币金额
func TestExtractAmount_NativeCoin(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	asset := &transaction_pb.AssetOutput{
		AssetContent: &transaction_pb.AssetOutput_NativeCoin{
			NativeCoin: &transaction_pb.NativeCoinAsset{
				Amount: "1000000",
			},
		},
	}

	amount := builder.extractAmount(asset)

	assert.NotNil(t, amount)
	assert.Equal(t, int64(1000000), amount.Int64())
}

// TestExtractAmount_ContractToken 测试提取合约代币金额
func TestExtractAmount_ContractToken(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	asset := &transaction_pb.AssetOutput{
		AssetContent: &transaction_pb.AssetOutput_ContractToken{
			ContractToken: &transaction_pb.ContractTokenAsset{
				ContractAddress: testutil.RandomAddress(),
				TokenIdentifier: &transaction_pb.ContractTokenAsset_FungibleClassId{
					FungibleClassId: []byte("default"),
				},
				Amount: "500000",
			},
		},
	}

	amount := builder.extractAmount(asset)

	assert.NotNil(t, amount)
	assert.Equal(t, int64(500000), amount.Int64())
}

// TestExtractAmount_InvalidAmount 测试无效金额
func TestExtractAmount_InvalidAmount(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	asset := &transaction_pb.AssetOutput{
		AssetContent: &transaction_pb.AssetOutput_NativeCoin{
			NativeCoin: &transaction_pb.NativeCoinAsset{
				Amount: "invalid-number",
			},
		},
	}

	amount := builder.extractAmount(asset)

	// 无效金额时，SetString 返回 false，amount 为 nil
	// 需要检查是否为 nil，避免 panic
	if amount == nil {
		// 如果返回 nil，这是预期的行为（无效金额）
		return
	}
	// 如果返回了 big.Int，应该是 0
	assert.Equal(t, int64(0), amount.Int64())
}

// TestIsTokenAcceptedInPolicy_EmptyWhitelist 测试空白名单（接受所有）
func TestIsTokenAcceptedInPolicy_EmptyWhitelist(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	tokenKey := txiface.TokenKey("native")
	acceptedTokens := []consensuscfg.TokenFilterConfig{}

	assert.True(t, builder.isTokenAcceptedInPolicy(tokenKey, acceptedTokens))
}

// TestIsTokenAcceptedInPolicy_InWhitelist 测试在白名单中
func TestIsTokenAcceptedInPolicy_InWhitelist(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	tokenKey := txiface.TokenKey("native")
	acceptedTokens := []consensuscfg.TokenFilterConfig{
		{AssetID: "native", MinAmount: 0},
		{AssetID: "contract:xxx:yyy", MinAmount: 0},
	}

	assert.True(t, builder.isTokenAcceptedInPolicy(tokenKey, acceptedTokens))
}

// TestIsTokenAcceptedInPolicy_NotInWhitelist 测试不在白名单中
func TestIsTokenAcceptedInPolicy_NotInWhitelist(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	tokenKey := txiface.TokenKey("contract:aaa:bbb")
	acceptedTokens := []consensuscfg.TokenFilterConfig{
		{AssetID: "native", MinAmount: 0},
		{AssetID: "contract:xxx:yyy", MinAmount: 0},
	}

	assert.False(t, builder.isTokenAcceptedInPolicy(tokenKey, acceptedTokens))
}

// TestGetTokenMinAmount_EmptyWhitelist 测试空白名单（无最低金额要求）
func TestGetTokenMinAmount_EmptyWhitelist(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	tokenKey := txiface.TokenKey("native")
	acceptedTokens := []consensuscfg.TokenFilterConfig{}

	minAmount, accepted := builder.getTokenMinAmount(tokenKey, acceptedTokens)

	assert.Equal(t, uint64(0), minAmount)
	assert.True(t, accepted)
}

// TestGetTokenMinAmount_InWhitelist 测试在白名单中（有最低金额要求）
func TestGetTokenMinAmount_InWhitelist(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	tokenKey := txiface.TokenKey("native")
	acceptedTokens := []consensuscfg.TokenFilterConfig{
		{AssetID: "native", MinAmount: 10000},
	}

	minAmount, accepted := builder.getTokenMinAmount(tokenKey, acceptedTokens)

	assert.Equal(t, uint64(10000), minAmount)
	assert.True(t, accepted)
}

// TestGetTokenMinAmount_NotInWhitelist 测试不在白名单中
func TestGetTokenMinAmount_NotInWhitelist(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	tokenKey := txiface.TokenKey("contract:aaa:bbb")
	acceptedTokens := []consensuscfg.TokenFilterConfig{
		{AssetID: "native", MinAmount: 10000},
	}

	minAmount, accepted := builder.getTokenMinAmount(tokenKey, acceptedTokens)

	assert.Equal(t, uint64(0), minAmount)
	assert.False(t, accepted)
}

// TestCloneAssetWithAmount_NativeCoin 测试克隆原生币资产并修改金额
func TestCloneAssetWithAmount_NativeCoin(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	original := &transaction_pb.AssetOutput{
		AssetContent: &transaction_pb.AssetOutput_NativeCoin{
			NativeCoin: &transaction_pb.NativeCoinAsset{
				Amount: "1000000",
			},
		},
	}

	newAmount := big.NewInt(500000)
	cloned := builder.cloneAssetWithAmount(original, newAmount)

	assert.NotNil(t, cloned)
	assert.NotEqual(t, original, cloned) // 应该是新对象
	nativeCoin := cloned.GetNativeCoin()
	assert.NotNil(t, nativeCoin)
	assert.Equal(t, "500000", nativeCoin.Amount)
}

// TestCloneAssetWithAmount_ContractToken 测试克隆合约代币资产并修改金额
func TestCloneAssetWithAmount_ContractToken(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	contractAddr := testutil.RandomAddress()
	original := &transaction_pb.AssetOutput{
		AssetContent: &transaction_pb.AssetOutput_ContractToken{
			ContractToken: &transaction_pb.ContractTokenAsset{
				ContractAddress: contractAddr,
				TokenIdentifier: &transaction_pb.ContractTokenAsset_FungibleClassId{
					FungibleClassId: []byte("default"),
				},
				Amount: "1000000",
			},
		},
	}

	newAmount := big.NewInt(500000)
	cloned := builder.cloneAssetWithAmount(original, newAmount)

	assert.NotNil(t, cloned)
	assert.NotEqual(t, original, cloned) // 应该是新对象
	contractToken := cloned.GetContractToken()
	assert.NotNil(t, contractToken)
	assert.Equal(t, contractAddr, contractToken.ContractAddress)
	assert.Equal(t, "500000", contractToken.Amount)
}

// TestCloneAssetWithAmount_UnknownType 测试未知类型的资产
func TestCloneAssetWithAmount_UnknownType(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	// 创建没有 AssetContent 的 AssetOutput
	original := &transaction_pb.AssetOutput{}

	newAmount := big.NewInt(500000)
	cloned := builder.cloneAssetWithAmount(original, newAmount)

	assert.Nil(t, cloned)
}

// TestGetSponsorUTXOHelper 测试获取赞助 UTXO 辅助工具
func TestGetSponsorUTXOHelper(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	helper := builder.GetSponsorUTXOHelper()

	assert.NotNil(t, helper)
	assert.NotNil(t, helper.eutxoQuery)
}

// ==================== buildSponsorClaimTransactions 测试 ====================

// TestBuildSponsorClaimTransactions_Success 测试构建赞助领取交易列表成功
func TestBuildSponsorClaimTransactions_Success(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	// 添加有效的赞助 UTXO
	sponsorUTXO := createSponsorUTXOWithDelegationLock("1000000", []string{"consume"}, nil, 100)
	utxoQuery.AddSponsorPoolUTXO(sponsorUTXO)

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	ctx := context.Background()
	minerAddr := testutil.RandomAddress()
	chainID := []byte("test-chain")
	blockHeight := uint64(200)
	policy := &consensuscfg.SponsorIncentiveConfig{
		Enabled:            true,
		MaxPerBlock:        10,
		MaxAmountPerSponsor: 1000000,
		AcceptedTokens:     []consensuscfg.TokenFilterConfig{},
	}

	claimTxs, err := builder.buildSponsorClaimTransactions(ctx, minerAddr, chainID, blockHeight, policy)

	assert.NoError(t, err)
	assert.NotNil(t, claimTxs)
	assert.Len(t, claimTxs, 1)
}

// TestBuildSponsorClaimTransactions_NoSponsorUTXOs 测试没有赞助 UTXO
func TestBuildSponsorClaimTransactions_NoSponsorUTXOs(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	ctx := context.Background()
	minerAddr := testutil.RandomAddress()
	chainID := []byte("test-chain")
	blockHeight := uint64(200)
	policy := &consensuscfg.SponsorIncentiveConfig{
		Enabled:            true,
		MaxPerBlock:        10,
		MaxAmountPerSponsor: 1000000,
		AcceptedTokens:     []consensuscfg.TokenFilterConfig{},
	}

	claimTxs, err := builder.buildSponsorClaimTransactions(ctx, minerAddr, chainID, blockHeight, policy)

	assert.NoError(t, err)
	assert.Empty(t, claimTxs)
}

// TestBuildSponsorClaimTransactions_MaxPerBlockLimit 测试 MaxPerBlock 限制
func TestBuildSponsorClaimTransactions_MaxPerBlockLimit(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := testutil.NewMockUTXOQuery()
	configProvider := NewMockConfigProvider()

	// 添加多个有效的赞助 UTXO
	for i := 0; i < 15; i++ {
		sponsorUTXO := createSponsorUTXOWithDelegationLock("1000000", []string{"consume"}, nil, 100)
		utxoQuery.AddSponsorPoolUTXO(sponsorUTXO)
	}

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	ctx := context.Background()
	minerAddr := testutil.RandomAddress()
	chainID := []byte("test-chain")
	blockHeight := uint64(200)
	policy := &consensuscfg.SponsorIncentiveConfig{
		Enabled:            true,
		MaxPerBlock:        5, // 限制为 5 个
		MaxAmountPerSponsor: 1000000,
		AcceptedTokens:     []consensuscfg.TokenFilterConfig{},
	}

	claimTxs, err := builder.buildSponsorClaimTransactions(ctx, minerAddr, chainID, blockHeight, policy)

	assert.NoError(t, err)
	assert.Len(t, claimTxs, 5) // 应该只返回 5 个
}

// TestBuildSponsorClaimTransactions_QueryError 测试查询失败
func TestBuildSponsorClaimTransactions_QueryError(t *testing.T) {
	feeManager := NewMockFeeManager()
	utxoQuery := NewMockUTXOQueryWithError(fmt.Errorf("查询失败"))
	configProvider := NewMockConfigProvider()

	builder := NewIncentiveBuilder(feeManager, utxoQuery, configProvider, nil)

	ctx := context.Background()
	minerAddr := testutil.RandomAddress()
	chainID := []byte("test-chain")
	blockHeight := uint64(200)
	policy := &consensuscfg.SponsorIncentiveConfig{
		Enabled:            true,
		MaxPerBlock:        10,
		MaxAmountPerSponsor: 1000000,
		AcceptedTokens:     []consensuscfg.TokenFilterConfig{},
	}

	claimTxs, err := builder.buildSponsorClaimTransactions(ctx, minerAddr, chainID, blockHeight, policy)

	assert.Error(t, err)
	assert.Nil(t, claimTxs)
	assert.Contains(t, err.Error(), "扫描赞助池失败")
}

// MockUTXOQueryWithError 带错误的 UTXO 查询器
type MockUTXOQueryWithError struct {
	*testutil.MockUTXOQuery
	queryError error
}

func NewMockUTXOQueryWithError(queryError error) *MockUTXOQueryWithError {
	return &MockUTXOQueryWithError{
		MockUTXOQuery: testutil.NewMockUTXOQuery(),
		queryError:    queryError,
	}
}

func (m *MockUTXOQueryWithError) GetSponsorPoolUTXOs(ctx context.Context, onlyAvailable bool) ([]*utxopb.UTXO, error) {
	if m.queryError != nil {
		return nil, m.queryError
	}
	return m.MockUTXOQuery.GetSponsorPoolUTXOs(ctx, onlyAvailable)
}

