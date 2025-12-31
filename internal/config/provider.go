package config

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/weisyn/v1/internal/config/api"
	"github.com/weisyn/v1/internal/config/blockchain"
	"github.com/weisyn/v1/internal/config/candidatepool"
	clockconfig "github.com/weisyn/v1/internal/config/clock"
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
	syncconfig "github.com/weisyn/v1/internal/config/sync"
	"github.com/weisyn/v1/internal/config/tx/draftstore"
	"github.com/weisyn/v1/internal/config/tx/fee"
	"github.com/weisyn/v1/internal/config/tx/signer"
	"github.com/weisyn/v1/internal/config/txpool"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/constants/protocols"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/types"
	"github.com/weisyn/v1/pkg/utils"
)

// Provider 实现配置提供者接口
type Provider struct {
	appConfig            *types.AppConfig
	cachedUnifiedGenesis *types.GenesisConfig // 缓存统一创世配置
	cachedBlockchain     *blockchain.BlockchainOptions

	// 保护 cachedBlockchain/cachedUnifiedGenesis 的初始化，避免重复解析配置导致日志刷屏与性能浪费
	blockchainOnce sync.Once
}

// NewProvider 创建配置提供者
//
// 🔧 **配置验证**：在创建Provider时验证必填配置项
func NewProvider(appConfig *types.AppConfig) config.Provider {
	provider := &Provider{
		appConfig: appConfig,
	}

	// 🔧 验证必填配置项
	// 注意：这里先不加载统一创世配置，验证时使用appConfig中的配置
	// 统一创世配置会在GetBlockchain()时加载
	if err := ValidateMandatoryConfig(appConfig, nil); err != nil {
		// 配置验证失败，但不在NewProvider时panic，允许延迟验证
		// 在实际使用时（如GetBlockchain）会再次验证
		// 这样可以避免循环依赖问题
		_ = err // 暂时忽略，后续在启动时验证
	}

	return provider
}

// GetInstanceDataDir 计算链实例的数据目录（instance_data_dir）
// 规则：
//   - baseRoot: 来自 storage.data_root，未配置时为 "./data"
//   - environment: 来自顶层 environment（dev/test/prod，默认 dev）
//   - 实例 slug:
//   - 如果配置了 network_profile，则直接使用
//   - 否则按 {environment}-{chain_mode}-{network_name|network_namespace|chain_id} 推导
//   - 最终路径：{baseRoot}/{environment}/{instance_slug}
func (p *Provider) GetInstanceDataDir() string {
	// 1. 基础根目录（data_root）
	baseRoot := "./data"
	if p.appConfig != nil && p.appConfig.Storage != nil && p.appConfig.Storage.DataRoot != nil && *p.appConfig.Storage.DataRoot != "" {
		baseRoot = *p.appConfig.Storage.DataRoot
	}

	// 2. 环境（environment）
	env := "dev"
	if p.appConfig != nil && p.appConfig.Environment != nil && *p.appConfig.Environment != "" {
		env = strings.ToLower(*p.appConfig.Environment)
	}

	// 3. 实例 slug
	var slug string

	// 优先使用显式的 network_profile
	if p.appConfig != nil && p.appConfig.NetworkProfile != nil && *p.appConfig.NetworkProfile != "" {
		slug = *p.appConfig.NetworkProfile
	} else if p.appConfig != nil && p.appConfig.Network != nil {
		mode := ""
		if p.appConfig.Network.ChainMode != nil {
			mode = strings.ToLower(*p.appConfig.Network.ChainMode)
		}

		name := ""
		if p.appConfig.Network.NetworkName != nil && *p.appConfig.Network.NetworkName != "" {
			name = *p.appConfig.Network.NetworkName
		} else if p.appConfig.Network.NetworkNamespace != nil && *p.appConfig.Network.NetworkNamespace != "" {
			name = *p.appConfig.Network.NetworkNamespace
		} else if p.appConfig.Network.ChainID != nil {
			name = fmt.Sprintf("chain-%d", *p.appConfig.Network.ChainID)
		}

		// ⚠️ 兼容：配置尚未完整时（常见于单测/工具场景），不要 panic，退化为可用的默认 slug。
		// 必填项校验应由启动流程/ValidateMandatoryConfig 兜底，而不是在路径推导时硬崩。
		if mode == "" {
			mode = "unknown"
		}
		if name == "" {
			name = "unknown"
		}

		// 与文档示例保持一致：{environment}-{chainmode}-{network-name}
		slug = fmt.Sprintf("%s-%s-%s", env, mode, name)
	} else {
		// ⚠️ 兼容：没有 network 配置时仍返回一个确定的默认路径，避免 panic。
		slug = fmt.Sprintf("%s-%s-%s", env, "unknown", "unknown")
	}

	// 4. 组合得到实例数据目录，并解析为绝对路径
	instanceDir := filepath.Join(baseRoot, env, slug)
	return utils.ResolveDataPath(instanceDir)
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

	// 应用默认身份密钥路径（基于存储路径）并解析相对路径
	p.applyDefaultIdentityKeyPath(nodeOptions)
	p.resolveIdentityKeyPath(nodeOptions)

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
	// ✅ 配置读取应是幂等/稳定的：避免每次调用都重新解析并产生大量日志。
	// 说明：
	// - 当前代码路径会被很多模块频繁调用（例如同步/共识/启动流程）。
	// - 若每次都重新 new，会导致你看到的 “🔧 CONFIG DEBUG” 刷屏，且浪费 CPU。
	// - 配置热更新目前不在生产路径内；如未来需要热更新，应引入显式 Reload 机制而不是隐式重复解析。
	p.blockchainOnce.Do(func() {
		p.cachedBlockchain = p.buildBlockchainOptionsOnce()
	})
	return p.cachedBlockchain
}

func (p *Provider) buildBlockchainOptionsOnce() *blockchain.BlockchainOptions {
	// 1. 尝试加载外部创世配置文件
	externalGenesisConfig, err := p.loadGenesisConfig()
	if err != nil {
		// 加载失败时使用内部默认配置
	}

	// 🔧 **关键修复**：如果主配置文件中有创世配置，且没有外部创世配置，
	// 则将主配置文件的创世配置转换为 externalGenesisConfig 格式（types.GenesisConfig）
	if externalGenesisConfig == nil && p.appConfig != nil && p.appConfig.Genesis != nil && len(p.appConfig.Genesis.Accounts) > 0 {
		unifiedGenesis := &types.GenesisConfig{
			GenesisAccounts: []types.GenesisAccount{},
		}

		// 从网络配置中获取 ChainID 和 NetworkID
		if p.appConfig.Network != nil {
			if p.appConfig.Network.ChainID != nil {
				unifiedGenesis.ChainID = *p.appConfig.Network.ChainID
			}
			// ⚠️ NetworkID 必须来自 network.network_id（链身份关键字段），而不是 network_name（仅展示用途）。
			// 否则会导致：
			// - 运行时计算的 genesis_hash 与工具/文档不一致
			// - 修改 network_id 后 expected_genesis_hash 永远对不上（表现为“地址不行/配置不行”）
			if p.appConfig.Network.NetworkID != nil && *p.appConfig.Network.NetworkID != "" {
				unifiedGenesis.NetworkID = *p.appConfig.Network.NetworkID
			} else if p.appConfig.Network.NetworkName != nil && *p.appConfig.Network.NetworkName != "" {
				// 兼容兜底：如果历史配置缺失 network_id，退化使用 network_name
				unifiedGenesis.NetworkID = *p.appConfig.Network.NetworkName
			}
		}

		// 🔧 修复：从配置文件读取固定的创世时间戳，确保所有节点创世区块一致
		// 创世时间戳必须在配置中指定，不允许使用默认值
		if p.appConfig.Genesis.Timestamp == 0 {
			// 错误：未配置创世时间戳，必须显式指定
			panic("配置错误：genesis.timestamp 必须指定，不能为空或0。创世区块时间戳必须是固定值，确保所有节点创建相同的创世区块")
		}
		unifiedGenesis.Timestamp = p.appConfig.Genesis.Timestamp

		// 转换账户配置（统一范式）：要求配置中显式提供 public_key
		for i, account := range p.appConfig.Genesis.Accounts {
			genesisAccount := types.GenesisAccount{
				Name:           account.Name,
				Address:        account.Address,
				InitialBalance: account.InitialBalance,
				PrivateKey:     account.PrivateKey,
			}

			// 显式使用配置中的 public_key（所有环境统一要求提供）
			if account.PublicKey != "" {
				genesisAccount.PublicKey = account.PublicKey
			}

			// 验证必需字段
			if genesisAccount.Address == "" {
				// 仅在显式开启配置调试时打印（避免启动刷屏）
				if os.Getenv("WES_CONFIG_DEBUG") == "true" && os.Getenv("WES_CLI_MODE") != "true" {
					println("⚠️  创世账户[", i, "]缺少address字段，跳过")
				}
				continue
			}
			if genesisAccount.InitialBalance == "" {
				if os.Getenv("WES_CONFIG_DEBUG") == "true" && os.Getenv("WES_CLI_MODE") != "true" {
					println("⚠️  创世账户[", i, "]缺少initial_balance字段，跳过")
				}
				continue
			}

			// ✅ 确定性保障：如果未提供 public_key，用 address 作为排序键/标识，保证 genesis_hash 计算稳定。
			// 说明：CalculateGenesisHash 会按 PublicKey 排序；若 PublicKey 全为空，会导致排序结果不稳定（进而 expected_genesis_hash 对不上）。
			if genesisAccount.PublicKey == "" {
				genesisAccount.PublicKey = genesisAccount.Address
			}

			unifiedGenesis.GenesisAccounts = append(unifiedGenesis.GenesisAccounts, genesisAccount)
		}

		// 如果成功解析了账户，使用这个作为 externalGenesisConfig
		if len(unifiedGenesis.GenesisAccounts) > 0 {
			externalGenesisConfig = unifiedGenesis
			// println("✅ 主配置文件创世配置已转换为统一格式，账户数:", len(unifiedGenesis.GenesisAccounts))
		}
	}

	// 🔧 **缓存统一创世配置**，供 GetUnifiedGenesisConfig() 使用
	if externalGenesisConfig != nil {
		p.cachedUnifiedGenesis = externalGenesisConfig
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
			// 同上：network_id 必须使用 NetworkID 字段
			if p.appConfig.Network.NetworkID != nil && *p.appConfig.Network.NetworkID != "" {
				blockchainConfigMap["network_id"] = *p.appConfig.Network.NetworkID
			} else if p.appConfig.Network.NetworkName != nil && *p.appConfig.Network.NetworkName != "" {
				blockchainConfigMap["network_id"] = *p.appConfig.Network.NetworkName
			}
		}

		// 处理创世配置（仅用于向后兼容，已被 externalGenesisConfig 机制取代）
		// 注意：此逻辑现在主要用于调试和向后兼容，实际使用 externalGenesisConfig
		if p.appConfig.Genesis != nil && len(p.appConfig.Genesis.Accounts) > 0 && externalGenesisConfig == nil {
			if os.Getenv("WES_CONFIG_DEBUG") == "true" && os.Getenv("WES_CLI_MODE") != "true" {
				println("⚠️  使用向后兼容的创世配置解析（已废弃），推荐使用 externalGenesisConfig 机制")
			}
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
					// println("🔧 PROVIDER FIX: 更新账户", i, "金额:", externalAccount.InitialBalance, "->", amount)
				}
			}
		}

		return options
	}

	return blockchainConfig.GetOptions()
}

// GetUnifiedGenesisConfig 获取统一格式的创世配置
//
// 🎯 **统一创世配置获取器**
//
// 返回完整的创世配置（types.GenesisConfig），包含所有必需字段（Address, PublicKey, InitialBalance等）。
// 此方法应该在 GetBlockchain() 之后调用，以确保配置已被正确加载和缓存。
//
// 返回：
//   - *types.GenesisConfig: 统一格式的创世配置，包含完整的账户信息
func (p *Provider) GetUnifiedGenesisConfig() *types.GenesisConfig {
	// 如果已缓存，直接返回
	if p.cachedUnifiedGenesis != nil {
		return p.cachedUnifiedGenesis
	}

	// 如果没有缓存，触发 GetBlockchain() 来加载配置
	// 注意：这会触发配置解析并缓存结果
	_ = p.GetBlockchain()

	// 返回缓存的配置（可能为 nil）
	return p.cachedUnifiedGenesis
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

		// 处理矿工配置（含 v2 挖矿稳定性门闸配置）
		minerConfig := make(map[string]interface{})
		if p.appConfig.Mining.MaxMiningThreads != nil {
			minerConfig["max_mining_threads"] = *p.appConfig.Mining.MaxMiningThreads
		}
		if p.appConfig.Mining.MiningTimeout != nil {
			minerConfig["mining_timeout"] = *p.appConfig.Mining.MiningTimeout
		}
		if p.appConfig.Mining.PoWSlice != nil {
			minerConfig["pow_slice"] = *p.appConfig.Mining.PoWSlice
		}

		// ========== v2：挖矿稳定性门闸配置（门闸 + 配置 MVP） ==========
		if p.appConfig.Mining.MinNetworkQuorumTotal != nil {
			minerConfig["min_network_quorum_total"] = *p.appConfig.Mining.MinNetworkQuorumTotal
		}
		if p.appConfig.Mining.AllowSingleNodeMining != nil {
			minerConfig["allow_single_node_mining"] = *p.appConfig.Mining.AllowSingleNodeMining
		}
		if p.appConfig.Mining.NetworkDiscoveryTimeoutSeconds != nil {
			minerConfig["network_discovery_timeout_seconds"] = *p.appConfig.Mining.NetworkDiscoveryTimeoutSeconds
		}
		if p.appConfig.Mining.QuorumRecoveryTimeoutSeconds != nil {
			minerConfig["quorum_recovery_timeout_seconds"] = *p.appConfig.Mining.QuorumRecoveryTimeoutSeconds
		}
		if p.appConfig.Mining.MaxHeightSkew != nil {
			minerConfig["max_height_skew"] = *p.appConfig.Mining.MaxHeightSkew
		}
		if p.appConfig.Mining.MaxTipStalenessSeconds != nil {
			minerConfig["max_tip_staleness_seconds"] = *p.appConfig.Mining.MaxTipStalenessSeconds
		}
		if p.appConfig.Mining.EnableTipFreshnessCheck != nil {
			minerConfig["enable_tip_freshness_check"] = *p.appConfig.Mining.EnableTipFreshnessCheck
		}
		if p.appConfig.Mining.EnableNetworkAlignmentCheck != nil {
			minerConfig["enable_network_alignment_check"] = *p.appConfig.Mining.EnableNetworkAlignmentCheck
		}

		// 计算并注入默认值（当用户未显式提供时）
		// 注意：默认值定义与推导逻辑集中在 internal/config/consensus/defaults.go。
		{
			// 1) 推导 env
			env := strings.ToLower(strings.TrimSpace(p.GetEnvironment()))

			// 2) 先构造一次 options，用于读取 aggregator.min_peer_threshold 默认/用户覆盖值
			tmpOptions := consensus.New(consensusConfigMap).GetOptions()
			minPeerThreshold := 3
			if tmpOptions != nil && tmpOptions.Aggregator.MinPeerThreshold > 0 {
				minPeerThreshold = tmpOptions.Aggregator.MinPeerThreshold
			}

			// 3) min_network_quorum_total 默认值
			if _, exists := minerConfig["min_network_quorum_total"]; !exists {
				minerConfig["min_network_quorum_total"] = consensus.DefaultMinNetworkQuorumTotal(env, minPeerThreshold)
			}

			// 4) allow_single_node_mining 默认 false
			if _, exists := minerConfig["allow_single_node_mining"]; !exists {
				minerConfig["allow_single_node_mining"] = false
			}

			// 5) timeouts / skew 默认值
			if _, exists := minerConfig["network_discovery_timeout_seconds"]; !exists {
				minerConfig["network_discovery_timeout_seconds"] = 120
			}
			if _, exists := minerConfig["quorum_recovery_timeout_seconds"]; !exists {
				minerConfig["quorum_recovery_timeout_seconds"] = 300
			}
			if _, exists := minerConfig["max_height_skew"]; !exists {
				// 彻底简化：不区分 initial/runtime，统一一个阈值
				minerConfig["max_height_skew"] = uint64(5)
			}

			// 6) max_tip_staleness_seconds 默认：target_block_time * 10
			if _, exists := minerConfig["max_tip_staleness_seconds"]; !exists {
				tb := tmpOptions.TargetBlockTime
				if p.appConfig.Mining.TargetBlockTime != nil {
					if d, err := time.ParseDuration(strings.TrimSpace(*p.appConfig.Mining.TargetBlockTime)); err == nil && d > 0 {
						tb = d
					}
				}
				minerConfig["max_tip_staleness_seconds"] = consensus.DefaultMaxTipStalenessSeconds(tb)
			}

			// 7) enable_tip_freshness_check 默认 true
			if _, exists := minerConfig["enable_tip_freshness_check"]; !exists {
				minerConfig["enable_tip_freshness_check"] = true
			}
		}

		// 只有在 minerConfig 有值时才写入
		if len(minerConfig) > 0 {
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
func (p *Provider) GetSync() *syncconfig.SyncOptions {
	return syncconfig.New(nil).GetOptions()
}

// GetLog 获取日志配置
func (p *Provider) GetLog() *log.LogOptions {
	// 构建包含 Storage 配置的日志配置，支持按链实例隔离
	var userLogConfigWithStorage *log.UserLogConfigWithStorage
	if p.appConfig != nil {
		instanceDir := p.GetInstanceDataDir()
		userLogConfigWithStorage = &log.UserLogConfigWithStorage{
			Log: p.appConfig.Log,
			Storage: &types.UserStorageConfig{
				DataRoot: types.StringPtr(instanceDir),
			},
		}
	}

	// log.New 会处理默认值应用和用户配置覆盖（包括从 storage.data_root 构建日志路径）
	return log.New(userLogConfigWithStorage).GetOptions()
}

// GetMemoryMonitoring 获取内存监控配置
func (p *Provider) GetMemoryMonitoring() *types.UserMemoryMonitoringConfig {
	if p.appConfig != nil && p.appConfig.MemoryMonitoring != nil {
		return p.appConfig.MemoryMonitoring
	}
	return nil
}

// GetEvent 获取事件配置
func (p *Provider) GetEvent() *event.EventOptions {
	return event.New(nil).GetOptions()
}

// === 存储引擎配置方法 ===

// GetBadger 获取BadgerDB存储配置
func (p *Provider) GetBadger() *badger.BadgerOptions {
	// 所有链级存储统一基于“链实例数据目录（instance_data_dir）”构建
	instanceDir := p.GetInstanceDataDir()
	userStorageConfig := &types.UserStorageConfig{
		DataRoot: types.StringPtr(instanceDir),
	}

	// badger.New 会处理默认值应用和用户配置覆盖
	return badger.New(userStorageConfig).GetOptions()
}

// GetMemory 获取内存存储配置
func (p *Provider) GetMemory() *memory.MemoryOptions {
	return memory.New(nil).GetOptions()
}

// GetFile 获取文件存储配置
func (p *Provider) GetFile() *file.FileOptions {
	// 传递链实例数据目录以支持按链实例隔离
	instanceDir := p.GetInstanceDataDir()
	userStorageConfig := &types.UserStorageConfig{
		DataRoot: types.StringPtr(instanceDir),
	}

	// file.New 会处理默认值应用和用户配置覆盖（包括从 storage.data_root 构建文件路径）
	return file.New(userStorageConfig).GetOptions()
}

// GetSQLite 获取SQLite存储配置
func (p *Provider) GetSQLite() *sqlite.SQLiteOptions {
	return sqlite.New(nil).GetOptions()
}

// GetTemporary 获取临时存储配置
func (p *Provider) GetTemporary() *temporary.TempOptions {
	// 传递链实例数据目录以支持按链实例隔离
	instanceDir := p.GetInstanceDataDir()
	userStorageConfig := &types.UserStorageConfig{
		DataRoot: types.StringPtr(instanceDir),
	}

	// temporary.New 会处理默认值应用和用户配置覆盖（包括从 storage.data_root 构建临时路径）
	return temporary.New(userStorageConfig).GetOptions()
}

// GetRepository 获取资源仓库配置
func (p *Provider) GetRepository() *repository.RepositoryOptions {
	// Repository配置已内部化，直接使用默认配置
	return repository.New(nil).GetOptions()
}

// GetAppConfig 获取原始应用配置（用于验证等场景）
func (p *Provider) GetAppConfig() *types.AppConfig {
	return p.appConfig
}

// GetSigner 获取签名器配置
func (p *Provider) GetSigner() *signer.SignerOptions {
	// 构建签名器用户配置
	var userSignerConfig *signer.UserSignerConfig

	if p.appConfig != nil && p.appConfig.Signer != nil {
		// 转换types.UserSignerConfig到signer.UserSignerConfig
		userSignerConfig = &signer.UserSignerConfig{
			Type: p.appConfig.Signer.Type,
		}

		// 转换本地签名器配置
		if p.appConfig.Signer.Local != nil {
			userSignerConfig.Local = &signer.LocalSignerConfig{
				PrivateKeyHex: p.appConfig.Signer.Local.PrivateKeyHex,
				Environment:   p.appConfig.Signer.Local.Environment,
				// Algorithm 使用默认值（在signer.New中处理）
			}
		}

		// 转换KMS签名器配置
		if p.appConfig.Signer.KMS != nil {
			userSignerConfig.KMS = &signer.KMSSignerConfig{
				KeyID:         p.appConfig.Signer.KMS.KeyID,
				RetryCount:    p.appConfig.Signer.KMS.RetryCount,
				RetryDelayMs:  p.appConfig.Signer.KMS.RetryDelayMs,
				SignTimeoutMs: p.appConfig.Signer.KMS.SignTimeoutMs,
				Environment:   p.appConfig.Signer.KMS.Environment,
				// Algorithm 使用默认值（在signer.New中处理）
			}
		}

		// 转换HSM签名器配置
		if p.appConfig.Signer.HSM != nil {
			userSignerConfig.HSM = &signer.HSMSignerConfig{
				KeyID:           p.appConfig.Signer.HSM.KeyID,
				KeyLabel:        p.appConfig.Signer.HSM.KeyLabel,
				Algorithm:       transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_UNKNOWN, // 使用默认值
				LibraryPath:     p.appConfig.Signer.HSM.LibraryPath,
				EncryptedPIN:    p.appConfig.Signer.HSM.EncryptedPIN,
				KMSKeyID:        p.appConfig.Signer.HSM.KMSKeyID,
				KMSType:         p.appConfig.Signer.HSM.KMSType,
				VaultAddr:       p.appConfig.Signer.HSM.VaultAddr,
				VaultToken:      p.appConfig.Signer.HSM.VaultToken,
				VaultSecretPath: p.appConfig.Signer.HSM.VaultSecretPath,
				SessionPoolSize: p.appConfig.Signer.HSM.SessionPoolSize,
				Endpoint:        p.appConfig.Signer.HSM.Endpoint,
				Username:        p.appConfig.Signer.HSM.Username,
				Password:        p.appConfig.Signer.HSM.Password,
				Environment:     p.appConfig.Signer.HSM.Environment,
			}
		}
	}

	return signer.New(userSignerConfig)
}

// GetDraftStore 获取草稿存储配置
func (p *Provider) GetDraftStore() interface{} {
	// 构建草稿存储用户配置
	var userDraftStoreConfig *draftstore.UserDraftStoreConfig

	// 暂时没有用户配置支持，使用默认值
	// TODO: 如果将来需要在用户配置中添加draftstore配置，在pkg/types/config.go中添加UserDraftStoreConfig字段

	return draftstore.New(userDraftStoreConfig)
}

// GetFeeEstimator 获取费用估算器配置
func (p *Provider) GetFeeEstimator() *fee.FeeEstimatorOptions {
	// 构建费用估算器用户配置
	var userFeeEstimatorConfig *fee.UserFeeEstimatorConfig

	// 暂时没有用户配置支持，使用默认值
	// TODO: 如果将来需要在用户配置中添加fee估算器配置，在pkg/types/config.go中添加UserFeeEstimatorConfig字段

	return fee.New(userFeeEstimatorConfig)
}

// GetClock 获取时钟配置
func (p *Provider) GetClock() *clockconfig.ClockOptions {
	return clockconfig.New().GetOptions()
}

// GetCompliance 获取合规配置
// 🎯 基于 Environment × ChainMode 的合规配置提供者
//
// 重构后：基于显式的 Environment 和 ChainMode 生成合规 profile
// 不再使用推断逻辑，完全基于配置字段
func (p *Provider) GetCompliance() *compliance.ComplianceOptions {
	env := p.GetEnvironment()
	chainMode := p.GetChainMode()

	// 基于 (Environment, ChainMode) 组合生成合规 profile
	// 映射到合规系统支持的 networkType 字符串
	networkType := p.resolveComplianceProfile(env, chainMode)

	// 创建完全自包含的合规配置
	return compliance.New(nil, networkType).GetOptions()
}

// resolveComplianceProfile 解析合规配置 profile
// 将 (Environment, ChainMode) 组合映射到合规系统支持的 networkType
func (p *Provider) resolveComplianceProfile(env, chainMode string) string {
	// 映射规则：
	// - dev + * → "development"
	// - test + * → "testing"
	// - prod + * → "production"
	// ChainMode 不影响合规 profile 的基础级别，但未来可以扩展
	switch env {
	case "dev":
		return "development"
	case "test":
		return "testing"
	case "prod":
		return "production"
	default:
		// 安全优先：未知环境默认为生产环境
		return "production"
	}
}

// GetEnvironment 获取运行环境
// 🎯 运行环境提供者
//
// 返回配置的运行环境：dev | test | prod
// 如果未配置，返回 "prod"（安全优先），但建议配置中必须显式指定
func (p *Provider) GetEnvironment() string {
	if p.appConfig != nil && p.appConfig.Environment != nil {
		env := strings.ToLower(*p.appConfig.Environment)
		// 验证值有效性
		switch env {
		case "dev", "test", "prod":
			return env
		}
	}
	// 安全优先：未配置时默认为生产环境
	return "prod"
}

// GetChainMode 获取链模式
// 🎯 链模式提供者
//
// 返回配置的链模式：public | consortium | private
// 如果未配置，启动失败（fail-fast，不再推断）
func (p *Provider) GetChainMode() string {
	if p.appConfig != nil && p.appConfig.Network != nil && p.appConfig.Network.ChainMode != nil {
		mode := strings.ToLower(*p.appConfig.Network.ChainMode)
		// 验证值有效性
		switch mode {
		case "public", "consortium", "private":
			return mode
		}
	}
	// fail-fast: 链模式必须显式配置，不再推断
	panic("chain_mode must be explicitly configured in network.chain_mode (valid values: public, consortium, private)")
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
// 重构后：直接返回配置值，不再推断（fail-fast）
func (p *Provider) GetNetworkNamespace() string {
	if p.appConfig != nil && p.appConfig.Network != nil && p.appConfig.Network.NetworkNamespace != nil {
		return *p.appConfig.Network.NetworkNamespace
	}
	// fail-fast: 命名空间必须显式配置
	panic("network_namespace must be explicitly configured in network.network_namespace")
}

// ============================================================================
//                          安全配置提供者
// ============================================================================

// GetSecurity 获取安全配置
// 🎯 安全配置提供者
//
// 返回安全配置对象，包含 access_control、certificate_management、psk、permission_model
// 如果未配置，返回 nil
func (p *Provider) GetSecurity() *types.UserSecurityConfig {
	if p.appConfig != nil && p.appConfig.Security != nil {
		return p.appConfig.Security
	}
	return nil
}

// GetAccessControlMode 获取接入控制模式
// 🎯 接入控制模式提供者
//
// 返回接入控制模式字符串：open | allowlist | psk
// 如果未配置，根据 chain_mode 返回默认值：
// - public: "open"
// - consortium: "allowlist"
// - private: "psk"
func (p *Provider) GetAccessControlMode() string {
	security := p.GetSecurity()
	if security != nil && security.AccessControl != nil && security.AccessControl.Mode != nil {
		mode := strings.ToLower(*security.AccessControl.Mode)
		// 验证值有效性
		switch mode {
		case "open", "allowlist", "psk":
			return mode
		}
	}

	// 未配置时，根据 chain_mode 返回默认值
	chainMode := p.GetChainMode()
	switch chainMode {
	case "public":
		return "open"
	case "consortium":
		return "allowlist"
	case "private":
		return "psk"
	default:
		// 未知链模式，fail-fast
		panic(fmt.Sprintf("unknown chain_mode: %s, cannot determine default access_control.mode", chainMode))
	}
}

// GetCertificateManagement 获取证书管理配置（仅联盟链）
// 🎯 证书管理配置提供者
//
// 返回证书管理配置对象，包含 ca_bundle_path
// 如果未配置或不是联盟链，返回 nil
func (p *Provider) GetCertificateManagement() *types.UserCertificateManagementConfig {
	security := p.GetSecurity()
	if security != nil && security.CertificateManagement != nil {
		// 验证是否为联盟链
		chainMode := p.GetChainMode()
		if chainMode != "consortium" {
			// 非联盟链不应该有证书管理配置，但这里不报错，只返回 nil
			// 验证逻辑在配置验证阶段处理
			return nil
		}
		return security.CertificateManagement
	}
	return nil
}

// GetPSK 获取 PSK 配置（仅私有链）
// 🎯 PSK 配置提供者
//
// 返回 PSK 配置对象，包含 file 路径
// 如果未配置或不是私有链，返回 nil
func (p *Provider) GetPSK() *types.UserPSKConfig {
	security := p.GetSecurity()
	if security != nil && security.PSK != nil {
		// 验证是否为私有链
		chainMode := p.GetChainMode()
		if chainMode != "private" {
			// 非私有链不应该有 PSK 配置，但这里不报错，只返回 nil
			// 验证逻辑在配置验证阶段处理
			return nil
		}
		return security.PSK
	}
	return nil
}

// GetPermissionModel 获取权限模型
// 🎯 权限模型提供者
//
// 返回权限模型字符串：public | consortium | private
// 如果未配置，根据 chain_mode 返回默认值（与 chain_mode 保持一致）
func (p *Provider) GetPermissionModel() string {
	security := p.GetSecurity()
	if security != nil && security.PermissionModel != nil {
		model := strings.ToLower(*security.PermissionModel)
		// 验证值有效性
		switch model {
		case "public", "consortium", "private":
			return model
		}
	}

	// 未配置时，默认与 chain_mode 保持一致
	return p.GetChainMode()
}

// contains 检查字符串是否包含子串（不区分大小写）
// ⚠️ 已废弃：不再使用推断逻辑，此函数保留仅用于向后兼容（如有需要）
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
	Timestamp       int64                `json:"timestamp"` // 创世时间戳（必需字段）
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

	// 验证必需字段：创世时间戳必须在配置文件中指定
	if fileConfig.Timestamp == 0 {
		return nil, fmt.Errorf("创世配置文件缺少必需字段 timestamp，必须显式指定创世区块时间戳")
	}

	// 转换为统一格式
	unifiedConfig := &types.GenesisConfig{
		NetworkID: fileConfig.NetworkID,
		ChainID:   fileConfig.ChainID,
		Timestamp: fileConfig.Timestamp, // 使用配置文件中的时间戳
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
// 基于链实例数据目录（instance_data_dir）自动设置默认的身份密钥文件路径。
//
// 默认规则：{instance_data_dir}/p2p/identity.key
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

	// 使用链实例数据目录作为身份密钥默认根目录
	instanceDir := p.GetInstanceDataDir()
	if instanceDir == "" {
		return
	}

	// 基于链实例数据目录设置默认身份密钥路径：{instance_data_dir}/p2p/identity.key
	identityKeyPath := filepath.Join(instanceDir, "p2p", "identity.key")
	nodeOptions.Host.Identity.KeyFile = utils.ResolveDataPath(identityKeyPath)
}

// resolveIdentityKeyPath 解析用户配置的身份密钥文件路径
// 如果路径是相对路径，相对于实例数据目录解析为绝对路径
func (p *Provider) resolveIdentityKeyPath(nodeOptions *node.NodeOptions) {
	if nodeOptions == nil {
		return
	}

	keyFile := nodeOptions.Host.Identity.KeyFile
	if keyFile == "" {
		return
	}

	// 如果已经是绝对路径，直接返回
	if filepath.IsAbs(keyFile) {
		return
	}

	// 相对路径：相对于实例数据目录解析
	instanceDir := p.GetInstanceDataDir()
	if instanceDir == "" {
		// 如果没有实例数据目录，保持原路径（向后兼容）
		return
	}

	// 解析为相对于实例数据目录的绝对路径
	nodeOptions.Host.Identity.KeyFile = filepath.Join(instanceDir, keyFile)
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

// ============================================================================
//                       创世配置辅助方法
// ============================================================================

// derivePublicKeyFromPrivate 从私钥（十六进制字符串）推导公钥（十六进制字符串）
//
// 🎯 **公钥推导器**
//
// 使用 secp256k1 椭圆曲线，从私钥推导出对应的压缩公钥。
//
// 参数：
//   - privateKeyHex: 十六进制格式的私钥字符串
//
// 返回：
//   - string: 十六进制格式的压缩公钥（33字节，02或03前缀）
//   - error: 推导过程中的错误
func (p *Provider) derivePublicKeyFromPrivate(privateKeyHex string) (string, error) {
	// 1. 解码十六进制私钥
	privateKeyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return "", fmt.Errorf("私钥解码失败: %w", err)
	}

	// 2. 验证私钥长度（secp256k1私钥是32字节）
	if len(privateKeyBytes) != 32 {
		return "", fmt.Errorf("私钥长度无效: 期望32字节, 实际%d字节", len(privateKeyBytes))
	}

	// 3. 使用 go-ethereum/crypto 库从私钥创建 ECDSA 私钥对象
	privateKey, err := crypto.ToECDSA(privateKeyBytes)
	if err != nil {
		return "", fmt.Errorf("私钥转换失败: %w", err)
	}

	// 4. 获取压缩公钥（33字节）
	compressedPubKey := crypto.CompressPubkey(&privateKey.PublicKey)

	// 5. 编码为十六进制字符串
	publicKeyHex := hex.EncodeToString(compressedPubKey)

	return publicKeyHex, nil
}
