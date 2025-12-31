package testutil

import (
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
	configiface "github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/types"
)

// MockConfigProvider is a lightweight config.Provider implementation for tests.
// It only provides meaningful values for the config fields used by v2 consensus rules.
type MockConfigProvider struct {
	Node             *nodeconfig.NodeOptions
	API              *apiconfig.APIOptions
	Blockchain       *blockchainconfig.BlockchainOptions
	Consensus        *consensusconfig.ConsensusOptions
	TxPool           *txpoolconfig.TxPoolOptions
	CandidatePool    *candidatepoolconfig.CandidatePoolOptions
	Network          *networkconfig.NetworkOptions
	Sync             *syncconfig.SyncOptions
	Log              *logconfig.LogOptions
	MemoryMonitoring *types.UserMemoryMonitoringConfig
	Event            *eventconfig.EventOptions
	Repository       *repositoryconfig.RepositoryOptions
	Compliance       *complianceconfig.ComplianceOptions
	Clock            *clockconfig.ClockOptions

	Environment      string
	ChainMode        string
	NetworkNamespace string

	Security *types.UserSecurityConfig

	Badger    *badgerconfig.BadgerOptions
	Memory    *memoryconfig.MemoryOptions
	File      *fileconfig.FileOptions
	SQLite    *sqliteconfig.SQLiteOptions
	Temporary *temporaryconfig.TempOptions

	Signer     *signerconfig.SignerOptions
	DraftStore interface{}

	AppConfig     *types.AppConfig
	GenesisConfig *types.GenesisConfig
}

func NewDefaultMockConfigProvider() *MockConfigProvider {
	return &MockConfigProvider{
		Blockchain:       blockchainconfig.New(nil).GetOptions(),
		Consensus:        consensusconfig.New(nil).GetOptions(),
		Environment:      "test",
		ChainMode:        "public",
		NetworkNamespace: "testnet",
	}
}

var _ configiface.Provider = (*MockConfigProvider)(nil)

func (m *MockConfigProvider) GetNode() *nodeconfig.NodeOptions { return m.Node }
func (m *MockConfigProvider) GetAPI() *apiconfig.APIOptions    { return m.API }
func (m *MockConfigProvider) GetBlockchain() *blockchainconfig.BlockchainOptions {
	if m.Blockchain != nil {
		return m.Blockchain
	}
	return blockchainconfig.New(nil).GetOptions()
}
func (m *MockConfigProvider) GetConsensus() *consensusconfig.ConsensusOptions {
	if m.Consensus != nil {
		return m.Consensus
	}
	return consensusconfig.New(nil).GetOptions()
}
func (m *MockConfigProvider) GetTxPool() *txpoolconfig.TxPoolOptions { return m.TxPool }
func (m *MockConfigProvider) GetCandidatePool() *candidatepoolconfig.CandidatePoolOptions {
	return m.CandidatePool
}
func (m *MockConfigProvider) GetNetwork() *networkconfig.NetworkOptions { return m.Network }
func (m *MockConfigProvider) GetSync() *syncconfig.SyncOptions          { return m.Sync }
func (m *MockConfigProvider) GetLog() *logconfig.LogOptions             { return m.Log }
func (m *MockConfigProvider) GetMemoryMonitoring() *types.UserMemoryMonitoringConfig {
	return m.MemoryMonitoring
}
func (m *MockConfigProvider) GetEvent() *eventconfig.EventOptions                { return m.Event }
func (m *MockConfigProvider) GetRepository() *repositoryconfig.RepositoryOptions { return m.Repository }
func (m *MockConfigProvider) GetCompliance() *complianceconfig.ComplianceOptions { return m.Compliance }
func (m *MockConfigProvider) GetClock() *clockconfig.ClockOptions                { return m.Clock }

func (m *MockConfigProvider) GetEnvironment() string {
	if m.Environment == "" {
		return "test"
	}
	return m.Environment
}
func (m *MockConfigProvider) GetChainMode() string {
	if m.ChainMode == "" {
		return "public"
	}
	return m.ChainMode
}
func (m *MockConfigProvider) GetInstanceDataDir() string {
	return "./data/test/test-mock"
}
func (m *MockConfigProvider) GetNetworkNamespace() string {
	if m.NetworkNamespace == "" {
		return "testnet"
	}
	return m.NetworkNamespace
}

func (m *MockConfigProvider) GetSecurity() *types.UserSecurityConfig { return m.Security }
func (m *MockConfigProvider) GetAccessControlMode() string           { return "open" }
func (m *MockConfigProvider) GetCertificateManagement() *types.UserCertificateManagementConfig {
	return nil
}
func (m *MockConfigProvider) GetPSK() *types.UserPSKConfig { return nil }
func (m *MockConfigProvider) GetPermissionModel() string   { return m.GetChainMode() }

func (m *MockConfigProvider) GetBadger() *badgerconfig.BadgerOptions     { return m.Badger }
func (m *MockConfigProvider) GetMemory() *memoryconfig.MemoryOptions     { return m.Memory }
func (m *MockConfigProvider) GetFile() *fileconfig.FileOptions           { return m.File }
func (m *MockConfigProvider) GetSQLite() *sqliteconfig.SQLiteOptions     { return m.SQLite }
func (m *MockConfigProvider) GetTemporary() *temporaryconfig.TempOptions { return m.Temporary }

func (m *MockConfigProvider) GetSigner() *signerconfig.SignerOptions { return m.Signer }
func (m *MockConfigProvider) GetDraftStore() interface{}             { return m.DraftStore }

func (m *MockConfigProvider) GetAppConfig() *types.AppConfig                { return m.AppConfig }
func (m *MockConfigProvider) GetUnifiedGenesisConfig() *types.GenesisConfig { return m.GenesisConfig }
