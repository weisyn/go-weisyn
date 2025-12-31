// Package config provides configuration provider interfaces.
package config

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
	"github.com/weisyn/v1/pkg/types"
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

	// GetMemoryMonitoring 获取内存监控配置
	GetMemoryMonitoring() *types.UserMemoryMonitoringConfig

	// GetEvent 获取事件配置
	GetEvent() *eventconfig.EventOptions

	// GetRepository 获取资源仓库配置
	GetRepository() *repositoryconfig.RepositoryOptions

	// GetCompliance 获取合规配置
	GetCompliance() *complianceconfig.ComplianceOptions

	// GetClock 获取时钟配置
	GetClock() *clockconfig.ClockOptions

	// === 环境与链模式配置 ===

	// GetEnvironment 获取运行环境
	// 返回运行环境字符串：dev | test | prod
	// 未配置时默认为 "prod"（安全优先）
	GetEnvironment() string

	// GetChainMode 获取链治理模式
	// 返回链模式字符串：public | consortium | private
	// 未配置时会 panic（fail-fast）
	GetChainMode() string

	// GetInstanceDataDir 获取链实例的数据目录
	// 返回链实例专属的数据目录路径，用于隔离不同链实例的数据
	// 路径格式：{data_root}/{environment}/{instance_slug}
	GetInstanceDataDir() string

	// === 网络命名空间配置 ===

	// GetNetworkNamespace 获取网络命名空间
	// 返回用于网络隔离的命名空间字符串，如"mainnet", "testnet", "dev"
	// 未配置时会 panic（fail-fast）
	GetNetworkNamespace() string

	// === 安全配置 ===

	// GetSecurity 获取安全配置
	// 返回安全配置对象，包含 access_control、certificate_management、psk、permission_model
	// 如果未配置，返回 nil
	GetSecurity() *types.UserSecurityConfig

	// GetAccessControlMode 获取接入控制模式
	// 返回接入控制模式字符串：open | allowlist | psk
	// 如果未配置，根据 chain_mode 返回默认值
	GetAccessControlMode() string

	// GetCertificateManagement 获取证书管理配置（仅联盟链）
	// 返回证书管理配置对象，包含 ca_bundle_path
	// 如果未配置或不是联盟链，返回 nil
	GetCertificateManagement() *types.UserCertificateManagementConfig

	// GetPSK 获取 PSK 配置（仅私有链）
	// 返回 PSK 配置对象，包含 file 路径
	// 如果未配置或不是私有链，返回 nil
	GetPSK() *types.UserPSKConfig

	// GetPermissionModel 获取权限模型
	// 返回权限模型字符串：public | consortium | private
	// 如果未配置，根据 chain_mode 返回默认值
	GetPermissionModel() string

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

	// === 签名器和费用估算器配置 ===

	// GetSigner 获取签名器配置
	GetSigner() *signerconfig.SignerOptions

	// GetDraftStore 获取草稿存储配置
	GetDraftStore() interface{} // 返回 *internal/config/tx/draftstore.DraftStoreOptions

	// === 原始配置访问 ===

	// GetAppConfig 获取原始应用配置（用于验证等场景）
	GetAppConfig() *types.AppConfig

	// GetUnifiedGenesisConfig 获取统一格式的创世配置
	// 返回合并后的创世配置（优先级：外部genesis.json > 主配置文件中的genesis）
	GetUnifiedGenesisConfig() *types.GenesisConfig
}
