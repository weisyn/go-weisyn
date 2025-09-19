package config

import (
	apiconfig "github.com/weisyn/v1/internal/config/api"
	blockchainconfig "github.com/weisyn/v1/internal/config/blockchain"
	candidatepoolconfig "github.com/weisyn/v1/internal/config/candidatepool"
	cliconfig "github.com/weisyn/v1/internal/config/cli"
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
	txpoolconfig "github.com/weisyn/v1/internal/config/txpool"
)

// Provider 配置提供者接口
type Provider interface {
	// === 核心配置 ===

	// GetNode 获取节点网络配置（NodeOptions）
	GetNode() *nodeconfig.NodeOptions

	// GetAPI 获取API服务配置
	GetAPI() *apiconfig.APIOptions

	// GetBlockchain 获取区块链配置
	GetBlockchain() *blockchainconfig.BlockchainOptions

	// GetConsensus 获取共识配置
	GetConsensus() *consensusconfig.ConsensusOptions

	// GetTxPool 获取交易池配置
	GetTxPool() *txpoolconfig.TxPoolOptions

	// GetCandidatePool 获取候选池配置
	GetCandidatePool() *candidatepoolconfig.CandidatePoolOptions

	// GetNetwork 获取网络配置
	GetNetwork() *networkconfig.NetworkOptions

	// GetSync 获取同步配置
	GetSync() *syncconfig.SyncOptions

	// GetLog 获取日志配置
	GetLog() *logconfig.LogOptions

	// GetEvent 获取事件配置
	GetEvent() *eventconfig.EventOptions

	// GetRepository 获取资源仓库配置
	GetRepository() *repositoryconfig.RepositoryOptions

	// GetCompliance 获取合规配置
	GetCompliance() *complianceconfig.ComplianceOptions

	// GetCLI 获取CLI配置
	GetCLI() *cliconfig.CLIOptions

	// === 网络命名空间配置 ===

	// GetNetworkNamespace 获取网络命名空间
	// 返回用于网络隔离的命名空间字符串，如"mainnet", "testnet", "dev"
	GetNetworkNamespace() string

	// === 存储引擎配置 ===

	// GetBadger 获取BadgerDB存储配置
	GetBadger() *badgerconfig.BadgerOptions

	// GetMemory 获取内存存储配置
	GetMemory() *memoryconfig.MemoryOptions

	// GetFile 获取文件存储配置
	GetFile() *fileconfig.FileOptions

	// GetSQLite 获取SQLite存储配置
	GetSQLite() *sqliteconfig.SQLiteOptions

	// GetTemporary 获取临时存储配置
	GetTemporary() *temporaryconfig.TempOptions
}
