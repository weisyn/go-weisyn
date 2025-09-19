package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/weisyn/v1/internal/config/api"
	"github.com/weisyn/v1/internal/config/blockchain"
	"github.com/weisyn/v1/internal/config/candidatepool"
	"github.com/weisyn/v1/internal/config/cli"
	"github.com/weisyn/v1/internal/config/compliance"
	"github.com/weisyn/v1/internal/config/consensus"
	"github.com/weisyn/v1/internal/config/event"
	"github.com/weisyn/v1/internal/config/log"
	"github.com/weisyn/v1/internal/config/network"
	"github.com/weisyn/v1/internal/config/node"
	"github.com/weisyn/v1/internal/config/repository"
	"github.com/weisyn/v1/internal/config/storage/badger"
	"github.com/weisyn/v1/internal/config/storage/file"
	"github.com/weisyn/v1/internal/config/storage/memory"
	"github.com/weisyn/v1/internal/config/storage/sqlite"
	"github.com/weisyn/v1/internal/config/storage/temporary"
	"github.com/weisyn/v1/internal/config/sync"
	"github.com/weisyn/v1/internal/config/txpool"
	"github.com/weisyn/v1/pkg/constants/protocols"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/types"
	"github.com/weisyn/v1/pkg/utils"
)

// Provider 实现配置提供者接口
type Provider struct {
	appConfig *types.AppConfig
}

// NewProvider 创建配置提供者
func NewProvider(appConfig *types.AppConfig) config.Provider {
	return &Provider{
		appConfig: appConfig,
	}
}

// GetNode 获取节点网络配置
func (p *Provider) GetNode() *node.NodeOptions {
	// 直接传递用户Node配置给node.New，让它处理默认值和转换
	var userNodeConfig *types.UserNodeConfig
	if p.appConfig != nil && p.appConfig.Node != nil {
		userNodeConfig = p.appConfig.Node
	}

	// node.New会处理默认值应用和用户配置覆盖
	nodeOptions := node.New(userNodeConfig).GetOptions()

	// 应用默认身份密钥路径（基于存储路径）
	p.applyDefaultIdentityKeyPath(nodeOptions)

	// 应用网络命名空间隔离（跨网防护）
	p.applyNetworkNamespaceIsolation(nodeOptions)

	return nodeOptions
}

// GetAPI 获取API服务配置
func (p *Provider) GetAPI() *api.APIOptions {
	// 直接传递用户API配置给api.New，让它处理默认值和转换
	var userAPIConfig *types.UserAPIConfig
	if p.appConfig != nil && p.appConfig.API != nil {
		userAPIConfig = p.appConfig.API
	}

	// api.New会处理默认值应用和用户配置覆盖
	return api.New(userAPIConfig).GetOptions()
}

// GetBlockchain 获取区块链配置
func (p *Provider) GetBlockchain() *blockchain.BlockchainOptions {
	// 1. 尝试加载外部创世配置文件
	externalGenesisConfig, err := p.loadGenesisConfig()
	if err != nil {
		// 加载失败时使用内部默认配置
	}

	// 2. 处理用户区块链配置 - 支持新统一配置结构
	var userBlockchainConfig interface{}

	// 优先使用新的统一配置结构
	if p.appConfig != nil && (p.appConfig.Network != nil || p.appConfig.Genesis != nil) {
		// 构建区块链配置映射，兼容现有的解析逻辑
		blockchainConfigMap := make(map[string]interface{})

		// 处理网络配置
		if p.appConfig.Network != nil {
			if p.appConfig.Network.ChainID != nil {
				blockchainConfigMap["chain_id"] = *p.appConfig.Network.ChainID
			}
			if p.appConfig.Network.NetworkName != nil {
				blockchainConfigMap["network_id"] = *p.appConfig.Network.NetworkName
			}
		}

		// 处理创世配置
		if p.appConfig.Genesis != nil && len(p.appConfig.Genesis.Accounts) > 0 {
			genesisConfig := make(map[string]interface{})
			var genesisAccounts []map[string]interface{}

			for _, account := range p.appConfig.Genesis.Accounts {
				accountMap := make(map[string]interface{})
				if account.Name != "" {
					accountMap["name"] = account.Name
				}
				if account.PrivateKey != "" {
					accountMap["private_key"] = account.PrivateKey
				}
				if account.Address != "" {
					accountMap["address"] = account.Address
				}
				if account.InitialBalance != "" {
					accountMap["initial_balance"] = account.InitialBalance
				}
				genesisAccounts = append(genesisAccounts, accountMap)
			}

			genesisConfig["genesis_accounts"] = genesisAccounts
			blockchainConfigMap["genesis"] = genesisConfig
		}

		// 处理区块链配置（包括block配置）
		if p.appConfig.Blockchain != nil {
			// 将blockchain配置合并到blockchainConfigMap中
			if blockchainMap, ok := p.appConfig.Blockchain.(map[string]interface{}); ok {
				for key, value := range blockchainMap {
					blockchainConfigMap[key] = value
				}
			}
		}

		userBlockchainConfig = blockchainConfigMap

	} else if p.appConfig != nil && p.appConfig.Blockchain != nil {
		// 向后兼容：使用原有的区块链配置
		userBlockchainConfig = p.appConfig.Blockchain
	}

	// 3. 创建扩展配置结构，包含外部创世配置
	extendedConfig := &blockchain.UserBlockchainConfig{
		Genesis:               userBlockchainConfig,  // 原有的用户配置
		ExternalGenesisConfig: externalGenesisConfig, // 外部加载的创世配置（优先级更高）
	}

	// 4. 传递扩展配置给blockchain.New进行处理，并确保外部创世配置被正确传递
	blockchainConfig := blockchain.New(extendedConfig)

	// 🔧 关键修复：确保外部创世配置被正确应用到最终的BlockchainOptions中
	if externalGenesisConfig != nil && len(externalGenesisConfig.GenesisAccounts) > 0 {
		// 获取当前选项
		options := blockchainConfig.GetOptions()

		// 更新创世账户为外部配置的值
		for i, externalAccount := range externalGenesisConfig.GenesisAccounts {
			if i < len(options.GenesisConfig.Accounts) {
				// 解析外部配置的金额字符串
				if amount, err := strconv.ParseUint(externalAccount.InitialBalance, 10, 64); err == nil {
					options.GenesisConfig.Accounts[i].Amount = amount
					println("🔧 PROVIDER FIX: 更新账户", i, "金额:", externalAccount.InitialBalance, "->", amount)
				}
			}
		}

		return options
	}

	return blockchainConfig.GetOptions()
}

// GetConsensus 获取共识配置
func (p *Provider) GetConsensus() *consensus.ConsensusOptions {
	// 处理用户共识配置 - 支持新统一配置结构
	var userConsensusConfig interface{}

	// 优先使用新的Mining配置结构
	if p.appConfig != nil && p.appConfig.Mining != nil {
		// 构建共识配置映射，将Mining配置转换为Consensus模块期望的格式
		consensusConfigMap := make(map[string]interface{})

		// 处理目标出块时间
		if p.appConfig.Mining.TargetBlockTime != nil {
			consensusConfigMap["target_block_time"] = *p.appConfig.Mining.TargetBlockTime
		}

		// 处理聚合器配置
		if p.appConfig.Mining.EnableAggregator != nil || p.appConfig.Mining.MaxMiningThreads != nil {
			aggregatorConfig := make(map[string]interface{})

			if p.appConfig.Mining.EnableAggregator != nil {
				aggregatorConfig["enable_aggregator"] = *p.appConfig.Mining.EnableAggregator
			}

			consensusConfigMap["aggregator"] = aggregatorConfig
		}

		// 处理矿工配置
		if p.appConfig.Mining.MaxMiningThreads != nil {
			minerConfig := make(map[string]interface{})
			minerConfig["max_mining_threads"] = *p.appConfig.Mining.MaxMiningThreads
			consensusConfigMap["miner"] = minerConfig
		}

		userConsensusConfig = consensusConfigMap

	} else if p.appConfig != nil && p.appConfig.Consensus != nil {
		// 向后兼容：使用原有的共识配置
		userConsensusConfig = p.appConfig.Consensus
	}

	// consensus.New会处理默认值应用和用户配置覆盖
	return consensus.New(userConsensusConfig).GetOptions()
}

// GetTxPool 获取交易池配置
func (p *Provider) GetTxPool() *txpool.TxPoolOptions {
	return txpool.New(nil).GetOptions()
}

// GetCandidatePool 获取候选池配置
func (p *Provider) GetCandidatePool() *candidatepool.CandidatePoolOptions {
	return candidatepool.New(nil).GetOptions()
}

// GetNetwork 获取网络配置
func (p *Provider) GetNetwork() *network.NetworkOptions {
	// 处理用户网络配置 - 支持新统一配置结构
	var userNetworkConfig interface{}

	// 使用新的Network配置结构
	if p.appConfig != nil && p.appConfig.Network != nil {
		// 构建网络配置映射，转换为Network模块期望的格式
		networkConfigMap := make(map[string]interface{})

		if p.appConfig.Network.ChainID != nil {
			networkConfigMap["chain_id"] = *p.appConfig.Network.ChainID
		}
		if p.appConfig.Network.NetworkName != nil {
			networkConfigMap["network_name"] = *p.appConfig.Network.NetworkName
		}

		userNetworkConfig = networkConfigMap
	}

	return network.New(userNetworkConfig).GetOptions()
}

// GetSync 获取同步配置
func (p *Provider) GetSync() *sync.SyncOptions {
	return sync.New(nil).GetOptions()
}

// GetLog 获取日志配置
func (p *Provider) GetLog() *log.LogOptions {
	// 直接传递用户日志配置给log.New，让它处理默认值和转换
	var userLogConfig *types.UserLogConfig
	if p.appConfig != nil && p.appConfig.Log != nil {
		userLogConfig = p.appConfig.Log
	}

	// log.New会处理默认值应用和用户配置覆盖
	return log.New(userLogConfig).GetOptions()
}

// GetEvent 获取事件配置
func (p *Provider) GetEvent() *event.EventOptions {
	return event.New(nil).GetOptions()
}

// === 存储引擎配置方法 ===

// GetBadger 获取BadgerDB存储配置
func (p *Provider) GetBadger() *badger.BadgerOptions {
	// 从新的Storage配置结构中提取路径信息，转换为BadgerDB配置
	var userStorageConfig *types.UserStorageConfig
	if p.appConfig != nil && p.appConfig.Storage != nil {
		userStorageConfig = p.appConfig.Storage
	}

	// badger.New会处理默认值应用和用户配置覆盖
	return badger.New(userStorageConfig).GetOptions()
}

// GetMemory 获取内存存储配置
func (p *Provider) GetMemory() *memory.MemoryOptions {
	return memory.New(nil).GetOptions()
}

// GetFile 获取文件存储配置
func (p *Provider) GetFile() *file.FileOptions {
	return file.New(nil).GetOptions()
}

// GetSQLite 获取SQLite存储配置
func (p *Provider) GetSQLite() *sqlite.SQLiteOptions {
	return sqlite.New(nil).GetOptions()
}

// GetTemporary 获取临时存储配置
func (p *Provider) GetTemporary() *temporary.TempOptions {
	return temporary.New(nil).GetOptions()
}

// GetRepository 获取资源仓库配置
func (p *Provider) GetRepository() *repository.RepositoryOptions {
	// Repository配置已内部化，直接使用默认配置
	return repository.New(nil).GetOptions()
}

// GetCLI 获取CLI配置
func (p *Provider) GetCLI() *cli.CLIOptions {
	// CLI配置通常使用默认值，暂不支持用户自定义配置
	return cli.New(nil).GetOptions()
}

// GetCompliance 获取合规配置
func (p *Provider) GetCompliance() *compliance.ComplianceOptions {
	// 1. 获取网络类型（环境感知安全控制的关键）
	var networkType string = "production" // 默认为生产环境（安全优先）

	// 首先尝试从blockchain配置中获取network_type
	if p.appConfig != nil && p.appConfig.Blockchain != nil {
		if userBlockchain, ok := p.appConfig.Blockchain.(map[string]interface{}); ok {
			if nt, exists := userBlockchain["network_type"]; exists {
				if ntStr, ok := nt.(string); ok {
					networkType = ntStr
				}
			}
		}
	}

	// 如果没有明确的network_type，尝试从network配置推断
	if networkType == "production" && p.appConfig != nil && p.appConfig.Network != nil {
		if p.appConfig.Network.NetworkName != nil {
			networkName := *p.appConfig.Network.NetworkName
			// 根据网络名称推断环境类型
			if contains(networkName, "test") || contains(networkName, "dev") {
				if contains(networkName, "test") {
					networkType = "testing"
				} else {
					networkType = "development"
				}
			}
		}
	}

	// 最后尝试从配置文件的_environment字段推断（如果有的话）
	// 注意：这个字段通常在配置文件的注释中，但可能被解析器忽略

	// 2. 创建完全自包含的合规配置
	// 注意：合规系统完全自包含，只需要networkType即可，无需用户配置
	return compliance.New(nil, networkType).GetOptions()
}

// GetNetworkNamespace 获取网络命名空间
// 🎯 网络隔离命名空间提供者
//
// 提供用于网络层隔离的命名空间字符串，该命名空间将用于：
// - P2P协议ID前缀：/weisyn/{namespace}/protocol/version
// - GossipSub主题前缀：weisyn.{namespace}.topic.version
// - DHT协议前缀：/weisyn/{namespace}
// - mDNS服务名：weisyn-node-{namespace}
//
// 优先级：
// 1. network.network_namespace（显式指定）
// 2. blockchain.network_type（向后兼容）
// 3. network.network_name推断（部分兼容）
// 4. 默认值："mainnet"（安全优先）
func (p *Provider) GetNetworkNamespace() string {
	// 1. 优先使用显式指定的network_namespace
	if p.appConfig != nil && p.appConfig.Network != nil && p.appConfig.Network.NetworkNamespace != nil {
		return *p.appConfig.Network.NetworkNamespace
	}

	// 2. 尝试从blockchain配置的network_type获取（向后兼容）
	if p.appConfig != nil && p.appConfig.Blockchain != nil {
		if userBlockchain, ok := p.appConfig.Blockchain.(map[string]interface{}); ok {
			if nt, exists := userBlockchain["network_type"]; exists {
				if ntStr, ok := nt.(string); ok && ntStr != "" {
					// 标准化network_type到命名空间
					switch ntStr {
					case "testnet", "testing":
						return "testnet"
					case "devnet", "development", "dev":
						return "dev"
					case "mainnet", "production", "prod":
						return "mainnet"
					default:
						// 自定义网络类型直接使用
						return ntStr
					}
				}
			}
		}
	}

	// 3. 尝试从network_name推断（部分兼容）
	if p.appConfig != nil && p.appConfig.Network != nil && p.appConfig.Network.NetworkName != nil {
		networkName := strings.ToLower(*p.appConfig.Network.NetworkName)
		if contains(networkName, "test") {
			return "testnet"
		} else if contains(networkName, "dev") {
			return "dev"
		}
	}

	// 4. 默认值：主网（安全优先）
	return "mainnet"
}

// contains 检查字符串是否包含子串（不区分大小写）
func contains(s, substr string) bool {
	s = strings.ToLower(s)
	substr = strings.ToLower(substr)
	return strings.Contains(s, substr)
}

// ============================================================================
//                          创世配置文件加载器
// ============================================================================

// GenesisFileConfig 创世配置文件结构（匹配configs/genesis.json）
type GenesisFileConfig struct {
	NetworkID       string               `json:"network_id"`
	ChainID         uint64               `json:"chain_id"`
	GenesisAccounts []GenesisFileAccount `json:"genesis_accounts"`
}

// GenesisFileAccount 创世账户文件结构（匹配configs/genesis.json）
type GenesisFileAccount struct {
	Name           string `json:"name,omitempty"`
	PrivateKey     string `json:"private_key,omitempty"` // 仅用于测试环境
	PublicKey      string `json:"public_key"`
	Address        string `json:"address,omitempty"`
	InitialBalance string `json:"initial_balance"` // JSON中使用字符串存储大数
	AddressType    string `json:"address_type,omitempty"`
}

// loadGenesisConfig 加载创世配置文件
//
// 🎯 **创世配置加载器**
//
// 尝试加载专门的创世配置文件 configs/genesis.json，
// 作为对主配置文件 configs/config.json 的补充。
//
// 返回：
//   - *types.GenesisConfig: 创世配置，如果文件不存在返回 nil
//   - error: 文件读取或解析错误
func (p *Provider) loadGenesisConfig() (*types.GenesisConfig, error) {
	// 获取项目根目录
	projectRoot, err := p.getProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("无法确定项目根目录: %w", err)
	}

	// 构建genesis.json文件路径
	genesisFilePath := filepath.Join(projectRoot, "configs", "genesis.json")

	// 检查文件是否存在
	if _, err := os.Stat(genesisFilePath); os.IsNotExist(err) {
		// 文件不存在不是错误，返回nil让调用者使用其他配置源
		return nil, nil
	}

	// 读取文件内容
	data, err := os.ReadFile(genesisFilePath)
	if err != nil {
		return nil, fmt.Errorf("读取创世配置文件失败: %w", err)
	}

	// 解析JSON
	var fileConfig GenesisFileConfig
	if err := json.Unmarshal(data, &fileConfig); err != nil {
		return nil, fmt.Errorf("解析创世配置文件失败: %w", err)
	}

	// 转换为统一格式
	unifiedConfig := &types.GenesisConfig{
		NetworkID: fileConfig.NetworkID,
		ChainID:   fileConfig.ChainID,
		Timestamp: time.Now().Unix(), // 使用当前时间作为创世时间戳
	}

	// 转换创世账户
	for i, fileAccount := range fileConfig.GenesisAccounts {

		account := types.GenesisAccount{
			Name:           fileAccount.Name,
			PrivateKey:     fileAccount.PrivateKey, // 注意：生产环境中不应包含私钥
			PublicKey:      fileAccount.PublicKey,
			Address:        fileAccount.Address,
			InitialBalance: fileAccount.InitialBalance,
			AddressType:    fileAccount.AddressType,
		}

		// 验证必需字段
		if account.PublicKey == "" {
			return nil, fmt.Errorf("创世账户[%d]缺少public_key字段", i)
		}
		if account.InitialBalance == "" {
			return nil, fmt.Errorf("创世账户[%d]缺少initial_balance字段", i)
		}

		unifiedConfig.GenesisAccounts = append(unifiedConfig.GenesisAccounts, account)
	}

	return unifiedConfig, nil
}

// getProjectRoot 获取项目根目录路径
//
// 🎯 **项目根目录定位器**
//
// 通过查找go.mod文件来确定项目根目录。
func (p *Provider) getProjectRoot() (string, error) {
	// 从当前工作目录开始查找
	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("无法获取当前工作目录: %w", err)
	}

	// 向上查找go.mod文件
	for {
		goModPath := filepath.Join(currentDir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return currentDir, nil
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			// 已到达文件系统根目录，未找到go.mod
			break
		}
		currentDir = parentDir
	}

	return "", fmt.Errorf("未找到go.mod文件，无法确定项目根目录")
}

// applyDefaultIdentityKeyPath 应用默认身份密钥路径
//
// 🎯 **身份密钥路径默认值设置**
//
// 当用户未配置 Host.Identity.KeyFile 且未配置 PrivateKey 时，
// 基于 storage.data_path 自动设置默认的身份密钥文件路径。
//
// 默认规则：<storage.data_path>/p2p/identity.key
//
// 参数：
//   - nodeOptions: 节点配置选项（会被直接修改）
func (p *Provider) applyDefaultIdentityKeyPath(nodeOptions *node.NodeOptions) {
	if nodeOptions == nil {
		return
	}

	// 如果已经配置了私钥或密钥文件，不需要设置默认值
	if nodeOptions.Host.Identity.PrivateKey != "" || nodeOptions.Host.Identity.KeyFile != "" {
		return
	}

	// 获取存储配置中的数据路径
	var dataPath string
	if p.appConfig != nil && p.appConfig.Storage != nil && p.appConfig.Storage.DataPath != nil {
		dataPath = *p.appConfig.Storage.DataPath
	}

	if dataPath != "" {
		// 基于存储路径设置默认身份密钥路径
		identityKeyPath := filepath.Join(dataPath, "p2p", "identity.key")
		nodeOptions.Host.Identity.KeyFile = utils.ResolveDataPath(identityKeyPath)
	}
}

// applyNetworkNamespaceIsolation 应用网络命名空间隔离
// 🎯 **网络发现隔离核心实现**
//
// 基于网络命名空间动态设置网络发现相关的标识符，确保不同环境的节点
// 无法相互发现和连接，实现网络层面的完全隔离。
//
// 隔离范围：
// - mDNS服务名：从 "weisyn-node" → "weisyn-node-{namespace}"
// - DHT协议前缀：从 "/weisyn" → "/weisyn/{namespace}"
// - Rendezvous命名空间：从 "weisyn" → "weisyn-{namespace}"
//
// 参数：
//   - nodeOptions: 节点配置选项（会被直接修改）
func (p *Provider) applyNetworkNamespaceIsolation(nodeOptions *node.NodeOptions) {
	if nodeOptions == nil {
		return
	}

	// 获取网络命名空间
	networkNamespace := p.GetNetworkNamespace()

	// 应用mDNS服务名命名空间化
	if nodeOptions.Discovery.MDNS.ServiceName == "weisyn-node" {
		// 只有当前是默认值时才修改，避免覆盖用户自定义的服务名
		nodeOptions.Discovery.MDNS.ServiceName = protocols.QualifyMDNSService("weisyn-node", networkNamespace)
	}

	// 应用DHT协议前缀命名空间化
	if nodeOptions.Discovery.DHT.ProtocolPrefix == "/weisyn" {
		// 只有当前是默认值时才修改，避免覆盖用户自定义的前缀
		nodeOptions.Discovery.DHT.ProtocolPrefix = protocols.QualifyDHTPrefix("/weisyn", networkNamespace)
	}

	// 应用Rendezvous命名空间
	if nodeOptions.Discovery.RendezvousNamespace == "weisyn" || nodeOptions.Discovery.RendezvousNamespace == "" {
		// 只有当前是默认值或空时才修改
		nodeOptions.Discovery.RendezvousNamespace = "weisyn-" + networkNamespace
	}
}
