// Package transaction 提供区块链交易管理的实现
//
// 🏗️ **统一交易管理器 - 模块化架构**
//
// 本文件实现了统一的交易管理器，作为各个业务模块的协调中心：
// - **架构角色**：薄管理器，委托具体业务实现给专业模块
// - **接口实现**：统一实现 4 个公共接口（TransactionService、ContractService、AIModelService、TransactionManager）
// - **模块协调**：协调 transfer/、resource/、contract/、aimodel/、lifecycle/ 等业务模块
// - **依赖注入**：作为各模块的依赖注入入口，管理全局依赖
//
// 🎯 **重构后职责**
// - **接口对齐**：确保与 pkg/interfaces/blockchain/ 中的接口完全对齐
// - **模块委托**：将具体业务逻辑委托给对应的业务模块实现
// - **依赖管理**：管理和注入各模块需要的公共依赖服务
// - **生命周期**：协调交易从构建到提交的完整生命周期
//
// ⚠️ **设计原则**
// - **薄管理器**：本文件不包含复杂业务逻辑，只做接口适配和模块调用
// - **模块化**：每个业务功能都有独立的模块实现
// - **类型统一**：使用 pkg/types 中的公共类型，不定义内部业务结构
// - **接口优先**：通过接口依赖，便于测试和模块替换
package transaction

import (
	"context"
	"fmt"
	"sync"
	"time"

	// 公共接口
	"github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/consensus"
	"github.com/weisyn/v1/pkg/interfaces/execution"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	netiface "github.com/weisyn/v1/pkg/interfaces/network"
	"github.com/weisyn/v1/pkg/interfaces/repository"
	"github.com/weisyn/v1/pkg/types"

	// 内部接口
	networkIntegration "github.com/weisyn/v1/internal/core/blockchain/integration/network"
	"github.com/weisyn/v1/internal/core/blockchain/interfaces"

	// 业务模块
	"github.com/weisyn/v1/internal/core/blockchain/transaction/aimodel"
	"github.com/weisyn/v1/internal/core/blockchain/transaction/contract"
	"github.com/weisyn/v1/internal/core/blockchain/transaction/fee"
	"github.com/weisyn/v1/internal/core/blockchain/transaction/genesis"
	"github.com/weisyn/v1/internal/core/blockchain/transaction/lifecycle"
	"github.com/weisyn/v1/internal/core/blockchain/transaction/mining"
	txNetworkHandler "github.com/weisyn/v1/internal/core/blockchain/transaction/network_handler"
	"github.com/weisyn/v1/internal/core/blockchain/transaction/resource"
	"github.com/weisyn/v1/internal/core/blockchain/transaction/transfer"
	"github.com/weisyn/v1/internal/core/blockchain/transaction/validation"

	// 协议定义
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"

	// libp2p依赖
	peer "github.com/libp2p/go-libp2p/core/peer"
	resourcePb "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
)

// ============================================================================
//                              管理器实现
// ============================================================================

// Manager 统一交易管理器
//
// 🎯 **新架构职责**：模块化交易管理协调中心
//
// 📋 **实现的公共接口**：
// - blockchain.TransactionService：统一交易服务（转账、静态资源部署）
// - blockchain.ContractService：智能合约服务（部署、调用）
// - blockchain.AIModelService：AI模型服务（部署、推理）
// - blockchain.TransactionManager：交易生命周期管理（签名、提交、查询、多签）
//
// 🏗️ **模块化架构**：
// - **业务模块委托**：具体业务逻辑委托给专业模块实现
// - **依赖注入协调**：管理所有模块的公共依赖
// - **接口适配层**：确保与公共接口的完美对齐
// - **生命周期协调**：协调交易的完整生命周期
//
// 🔧 **依赖管理**：
// - **基础设施依赖**：repository、txPool、crypto services等
// - **业务模块实例**：transfer、contract、aimodel、lifecycle等
// - **缓存和配置**：内存缓存、配置管理等
type Manager struct {
	// ========== 基础设施依赖 ==========
	repo                repository.RepositoryManager             // 数据存储访问层
	txPool              mempool.TxPool                           // 交易池访问
	utxoManager         repository.UTXOManager                   // UTXO管理服务
	minerService        consensus.MinerService                   // 矿工服务（用于获取矿工地址）
	configManager       config.Provider                          // 配置管理器（用于获取链ID等配置）
	txHashServiceClient transaction.TransactionHashServiceClient // 交易哈希服务客户端
	hashManager         crypto.HashManager                       // 哈希计算服务
	signatureManager    crypto.SignatureManager                  // 数字签名服务
	keyManager          crypto.KeyManager                        // 密钥管理服务
	addressManager      crypto.AddressManager                    // 地址管理服务
	cacheStore          storage.MemoryStore                      // 内存缓存服务
	feeManager          *fee.Manager                             // 费用系统管理器
	networkService      netiface.Network                         // 网络层服务
	logger              log.Logger                               // 日志记录器（可选）

	// ========== 业务模块实例 ==========
	assetTransferService          *transfer.AssetTransferService             // 资产转账服务
	batchTransferService          *transfer.BatchTransferService             // 批量转账服务
	staticDeployService           *resource.StaticResourceDeployService      // 静态资源部署服务
	contractDeployService         *contract.ContractDeployService            // 合约部署服务
	contractCallService           *contract.ContractCallService              // 合约调用服务
	aiModelDeployService          *aimodel.AIModelDeployService              // AI模型部署服务
	aiModelInferService           *aimodel.AIModelInferService               // AI模型推理服务
	transactionSignService        *lifecycle.TransactionSignService          // 交易签名服务
	transactionSubmitService      *lifecycle.TransactionSubmitService        // 交易提交服务
	transactionQueryService       *lifecycle.TransactionQueryService         // 交易查询服务
	transactionStatusService      *lifecycle.TransactionStatusService        // 交易状态服务
	transactionFeeEstimateService *lifecycle.TransactionFeeEstimationService // 交易费用估算服务
	transactionValidateService    *lifecycle.TransactionValidationService    // 交易验证服务
	multiSigService               *lifecycle.MultiSigService                 // 多重签名服务
	miningTemplateService         *mining.MiningTemplateService              // 挖矿模板服务

	// ========== 网络集成模块 ==========
	networkHandlerService interfaces.NetworkProtocolHandler // 网络协议处理服务

	// ========== 会话管理 ==========
	sessionMutex sync.RWMutex                      // 会话缓存读写锁
	sessionCache map[string]*types.MultiSigSession // 多签会话缓存（使用公共类型）
}

// NewManager 创建新的交易管理器实例
//
// 🏗️ **构造函数 - 依赖注入模式**
//
// 参数说明：
//   - repo: 仓储管理器，提供底层数据访问能力
//   - txPool: 交易池，用于交易广播和管理
//   - utxoManager: UTXO管理器，用于UTXO选择和管理
//   - minerService: 矿工服务，提供矿工地址等信息
//   - configManager: 配置管理器，提供链ID等配置信息
//   - txHashServiceClient: 交易哈希服务客户端
//   - hashManager: 哈希管理器，用于计算交易哈希
//   - signatureManager: 签名管理器，用于交易签名
//   - keyManager: 密钥管理器，用于密钥操作
//   - addressManager: 地址管理器，用于地址转换
//   - cacheStore: 内存缓存服务，用于缓存管理
//   - logger: 日志记录器，用于记录操作日志（可选）
//   - assetTransferService: 资产转账服务实例
//   - batchTransferService: 批量转账服务实例
//   - staticDeployService: 静态资源部署服务实例
//   - contractDeployService: 合约部署服务实例
//   - contractCallService: 合约调用服务实例
//   - aiModelDeployService: AI模型部署服务实例
//   - aiModelInferService: AI模型推理服务实例
//   - transactionSignService: 交易签名服务实例
//
// 返回：
//   - *Manager: 交易管理器实例
//
// 设计说明：
// - 使用依赖注入模式，便于测试和扩展
// - 管理器作为薄实现层，协调各个专门的服务模块
// - 支持模块化架构，每个业务功能由独立服务实现
// - 初始化内存缓存，支持哈希+缓存架构
//
// 使用示例：
//
//	```go
//	manager := NewManager(repo, txPool, utxoMgr, consensus, config, txHashClient,
//	                      hashMgr, sigMgr, keyMgr, addrMgr, cache, logger,
//	                      assetService, batchService, staticService,
//	                      contractDeployService, contractCallService,
//	                      aiModelDeployService, aiModelInferService, signService)
//	txService := manager.(blockchain.TransactionService)
//	```
func NewManager(
	repo repository.RepositoryManager,
	txPool mempool.TxPool,
	utxoManager repository.UTXOManager,
	resourceManager repository.ResourceManager,
	minerService consensus.MinerService,
	configManager config.Provider,
	txHashServiceClient transaction.TransactionHashServiceClient,
	hashManager crypto.HashManager,
	signatureManager crypto.SignatureManager,
	keyManager crypto.KeyManager,
	addressManager crypto.AddressManager,
	cacheStore storage.MemoryStore,
	networkService netiface.Network,
	// 🎯 execution接口依赖
	engineManager execution.EngineManager,
	hostCapabilityRegistry execution.HostCapabilityRegistry,
	executionCoordinator execution.ExecutionCoordinator,
	// 🎯 网络基础设施依赖
	host node.Host,
	kbucketManager kademlia.RoutingTableManager,
	logger log.Logger,
) *Manager {
	if repo == nil {
		panic("交易管理器初始化失败：仓储管理器不能为空")
	}
	if txPool == nil {
		panic("交易管理器初始化失败：交易池不能为空")
	}
	if utxoManager == nil {
		panic("交易管理器初始化失败：UTXO管理器不能为空")
	}
	if resourceManager == nil {
		panic("交易管理器初始化失败：资源管理器不能为空")
	}
	// 矿工服务允许为nil，在共识模块启动后再注入
	// if minerService == nil {
	//     panic("交易管理器初始化失败：矿工服务不能为空")
	// }
	if configManager == nil {
		panic("交易管理器初始化失败：配置管理器不能为空")
	}
	if txHashServiceClient == nil {
		panic("交易管理器初始化失败：交易哈希服务客户端不能为空")
	}
	if hashManager == nil {
		panic("交易管理器初始化失败：哈希管理器不能为空")
	}
	if cacheStore == nil {
		panic("交易管理器初始化失败：内存缓存服务不能为空")
	}
	if networkService == nil {
		panic("交易管理器初始化失败：网络服务不能为空")
	}
	if host == nil {
		panic("交易管理器初始化失败：节点Host不能为空")
	}
	if kbucketManager == nil {
		panic("交易管理器初始化失败：K-bucket管理器不能为空")
	}
	// 创建业务服务实例（直接使用log.Logger公共接口，符合架构原则）

	// 6. 初始化费用系统
	feeManager := fee.NewManager(txHashServiceClient)

	assetTransferService := transfer.NewAssetTransferService(utxoManager, cacheStore, keyManager, addressManager, configManager, txHashServiceClient, feeManager, logger)
	batchTransferService := transfer.NewBatchTransferService(utxoManager, cacheStore, keyManager, addressManager, configManager, txHashServiceClient, logger)
	staticDeployService := resource.NewStaticResourceDeployService(utxoManager, resourceManager, hashManager, keyManager, addressManager, cacheStore, configManager, logger)
	// ✅ 使用真实的ResourceManager和execution接口依赖注入
	contractDeployService := contract.NewContractDeployService(
		utxoManager,
		keyManager,
		addressManager,
		cacheStore,
		logger,
		resourceManager,
		txHashServiceClient,
		configManager,
	)
	// ✅ 使用真实的execution接口依赖
	contractCallService := contract.NewContractCallService(
		utxoManager,
		signatureManager,
		hashManager,
		keyManager,
		addressManager,
		txHashServiceClient, // 统一交易哈希服务
		cacheStore,
		engineManager,
		hostCapabilityRegistry,
		executionCoordinator,
		configManager,
		logger,
	)
	aiModelDeployService := aimodel.NewAIModelDeployService(utxoManager, resourceManager, hashManager, keyManager, addressManager, cacheStore, logger)
	aiModelInferService := aimodel.NewAIModelInferService(utxoManager, hashManager, keyManager, addressManager, cacheStore, logger)
	transactionSignService := lifecycle.NewTransactionSignService(signatureManager, keyManager, addressManager, utxoManager, txHashServiceClient, cacheStore, logger)
	transactionQueryService := lifecycle.NewTransactionQueryService(logger, cacheStore, txPool, repo)
	transactionStatusService := lifecycle.NewTransactionStatusService(logger, cacheStore, txPool, repo)
	transactionSubmitService := lifecycle.NewTransactionSubmitService(logger, cacheStore, txPool, networkService, repo, txHashServiceClient, utxoManager, host, kbucketManager)

	transactionFeeEstimateService := lifecycle.NewTransactionFeeEstimationService(logger, feeManager, cacheStore, utxoManager, repo)

	// 7. 初始化验证系统（包含跨网防护）
	var localChainID uint64 = 0
	if configManager != nil {
		if blockchainConfig := configManager.GetBlockchain(); blockchainConfig != nil {
			localChainID = blockchainConfig.ChainID
		}
	}
	transactionValidateService := lifecycle.NewTransactionValidationService(logger, cacheStore, utxoManager, txHashServiceClient, localChainID)

	// 8. 初始化多重签名系统
	multiSigService := lifecycle.NewMultiSigService(logger)

	// 9. 初始化挖矿模板服务
	miningTemplateService := mining.NewMiningTemplateService(
		repo, txPool, utxoManager, minerService, configManager,
		txHashServiceClient, hashManager, addressManager, cacheStore, logger)

	// 10. 初始化网络集成模块
	networkHandlerService := txNetworkHandler.NewTxNetworkProtocolHandlerService(txPool, transactionValidateService, logger)

	if logger != nil {
		logger.Info("✅ 交易管理器业务服务初始化完成 - 15个子服务已创建")
	}

	manager := &Manager{
		repo:                repo,
		txPool:              txPool,
		utxoManager:         utxoManager,
		minerService:        minerService,
		configManager:       configManager,
		txHashServiceClient: txHashServiceClient,
		hashManager:         hashManager,
		signatureManager:    signatureManager,
		keyManager:          keyManager,
		addressManager:      addressManager,
		cacheStore:          cacheStore,
		feeManager:          feeManager,
		networkService:      networkService,
		logger:              logger,

		// ========== 业务模块实例 ==========
		assetTransferService:          assetTransferService,
		batchTransferService:          batchTransferService,
		staticDeployService:           staticDeployService,
		contractDeployService:         contractDeployService,
		contractCallService:           contractCallService,
		aiModelDeployService:          aiModelDeployService,
		aiModelInferService:           aiModelInferService,
		transactionSignService:        transactionSignService,
		transactionSubmitService:      transactionSubmitService,
		transactionQueryService:       transactionQueryService,
		transactionStatusService:      transactionStatusService,
		transactionFeeEstimateService: transactionFeeEstimateService,
		transactionValidateService:    transactionValidateService,
		multiSigService:               multiSigService,
		miningTemplateService:         miningTemplateService,

		// ========== 网络集成模块 ==========
		networkHandlerService: networkHandlerService,

		// ========== 会话管理 ==========
		sessionCache: make(map[string]*types.MultiSigSession),
	}

	// 记录初始化日志
	if logger != nil {
		logger.Infof("✅ 交易管理器初始化完成 - component: TransactionManager, cacheEnabled: true, multiSigEnabled: true")
	}

	return manager
}

// SetMinerService 设置矿工服务（用于延迟注入，解决循环依赖）
func (m *Manager) SetMinerService(minerService consensus.MinerService) {
	m.minerService = minerService
	if m.miningTemplateService != nil {
		// 直接调用挖矿模板服务的SetMinerService方法
		m.miningTemplateService.SetMinerService(minerService)
	}
	if m.logger != nil {
		m.logger.Info("🔗 交易管理器已注入矿工服务")
	}
}

// GetNetworkHandler 获取网络处理器
//
// 🎯 **网络集成委托方法**
//
// 为 blockchain/module.go 装配层提供网络处理器实例，
// 用于注册到 integration/network.RegisterSubscribeHandlers
//
// 返回值:
//
//	networkIntegration.TxAnnounceRouter: 交易公告路由器接口实现
func (m *Manager) GetNetworkHandler() networkIntegration.TxAnnounceRouter {
	return m.networkHandlerService
}

// ████████████████████████████████████████████████████████████████████████████████████████████████
// █                                                                                              █
// █                         💰  TRANSACTION SERVICE INTERFACE                                 █
// █                                                                                              █
// █   统一交易服务：处理所有类型的区块链交易操作（价值转移、资源部署、合约执行）         █
// █                                                                                              █
// ████████████████████████████████████████████████████████████████████████████████████████████████

// TransferAsset 转账操作（支持基础和高级模式）
//
// 🎯 **功能说明**：
//   - 基础模式（options=nil）：个人日常转账，系统自动处理
//   - 高级模式（options!=nil）：企业级转账，支持复杂业务场景
//
// 📋 **实现状态**：已完成，委托给专门的资产转账服务
// - 🗋️ **具体实现在**： internal/core/blockchain/transaction/transfer/asset_transfer.go
// - 🔄 **高级功能支持**：7种锁定机制、企业多签、时间控制、委托授权
// - 📊 **自动处理特性**：UTXO智能选择、找零计算、手续费估算、余额验证
// - 🔐 **锁定机制映射**：业务策略自动选择对应的protobuf锁定机制
//
// 📝 **参数说明**：
//   - toAddress: 接收方地址（十六进制字符串）
//   - amount: 转账金额（字符串，支持小数，如"1.23456789"）
//   - tokenID: 代币标识（""=原生代币，其他=合约地址）
//   - memo: 转账备注（可选，显示在区块浏览器）
//   - options: 高级控制选项（可变参数，省略=基础转账，传入=企业级高级功能）
//
// 💡 **返回值说明**：
//   - []byte: 未签名交易哈希（用于SignTransaction）
//   - error: 构建错误
//
// 💡 **调用示例**：
//   - 基础转账：TransferAsset(ctx, addr, "100.0", "", "转账备注")
//   - 高级转账：TransferAsset(ctx, addr, "100.0", "", "转账备注", &transferOptions)
func (m *Manager) TransferAsset(ctx context.Context,
	senderPrivateKey []byte,
	toAddress string,
	amount string,
	tokenID string,
	memo string,
	options ...*types.TransferOptions,
) ([]byte, error) {
	// 薄实现：纯参数透传，不做业务逻辑处理
	if m.assetTransferService == nil {
		return nil, fmt.Errorf("资产转账服务未初始化")
	}

	// 直接透传所有参数给具体服务
	return m.assetTransferService.TransferAsset(ctx, senderPrivateKey, toAddress, amount, tokenID, memo, options...)
}

// BatchTransfer 批量转账操作
//
// 🎯 **效率优化**：一次性处理多笔转账，降低手续费
//
// 📋 **实现状态**：已完成，委托给专门的批量转账服务
// - 🗋️ **具体实现在**： internal/core/blockchain/transaction/transfer/batch_transfer.go
// - 📊 **优化特性**：UTXO批量选择优化、手续费分摊计算、原子性保证、失败全部回滚
//
// 📝 **适用场景**：
//   - 工资发放、红包分发、空投发放
//   - 批量退款、分润结算
//
// 📝 **参数说明**：
//   - transfers: 转账参数列表（最多1000笔）
//
// 💡 **返回值说明**：
//   - []byte: 未签名批量交易哈希
//   - error: 构建错误
func (m *Manager) BatchTransfer(ctx context.Context,
	senderPrivateKey []byte,
	transfers []types.TransferParams,
) ([]byte, error) {
	// 薄实现：纯参数透传，不做业务逻辑处理
	if m.batchTransferService == nil {
		return nil, fmt.Errorf("批量转账服务未初始化")
	}

	// 直接透传所有参数给具体服务
	return m.batchTransferService.BatchTransfer(ctx, senderPrivateKey, transfers)
}

// DeployStaticResource 静态资源部署（支持基础和高级模式）
//
// 🎯 **功能说明**：
//   - 基础模式（options=nil）：个人文件上传，isPublic控制访问
//   - 高级模式（options!=nil）：企业级资源管理，支持复杂业务场景
//
// 📋 **实现状态**：薄实现层，等待后续细化
// - 🗋️ **具体实现将在**： internal/core/blockchain/transaction/resource_deploy.go
// - 📊 **自动处理特性**：文件哈希计算、存储成本估算、重复检测、格式验证
// - 🔐 **访问控制模式**：personal、shared、commercial、enterprise
//
// 📝 **基础模式典型应用**：
//   - 个人照片备份、重要文档存证
//   - 创作作品版权保护、学历证书存储
//
// 📝 **高级模式支持的业务场景**：
//   - 企业机密文档：多重签名访问控制
//   - 付费数字内容：按次付费下载模式
//   - 团队协作文档：部门内共享访问
//   - 定时发布内容：预设时间自动公开
//
// 📝 **参数说明**：
//   - filePath: 本地文件路径（如："/path/to/document.pdf"）
//   - name: 资源显示名称（如："我的毕业证书"）
//   - description: 资源描述信息（如："清华大学计算机学士学位证书"）
//   - isPublic: 是否公开访问（基础模式：true=任何人可访问，false=仅上传者）
//   - tags: 资源分类标签（如：["\u8bc1\u4e66", "\u6559\u80b2", "\u4e2a\u4eba"]）
//   - options: 高级部署选项（可变参数，省略=基础模式，传入=企业级高级功能）
//
// 💡 **返回值说明**：
//   - []byte: 未签名交易哈希
//   - error: 部署错误
//
// 💡 **调用示例**：
//   - 基础部署：DeployStaticResource(ctx, "/path/file.pdf", "证书", "学位证书", true, []string{"教育"})
//   - 高级部署：DeployStaticResource(ctx, "/path/file.pdf", "证书", "学位证书", true, []string{"教育"}, &deployOptions)
func (m *Manager) DeployStaticResource(ctx context.Context,
	deployerPrivateKey []byte,
	filePath string,
	name string,
	description string,
	tags []string,
	options ...*types.ResourceDeployOptions,
) ([]byte, error) {
	// 薄实现：纯参数透传，不做业务逻辑处理
	if m.staticDeployService == nil {
		return nil, fmt.Errorf("静态资源部署服务未初始化")
	}

	// 直接透传所有参数给具体服务
	return m.staticDeployService.DeployStaticResource(ctx, deployerPrivateKey, filePath, name, description, tags, options...)
}

// FetchStaticResourceFile 获取静态资源文件
//
// 🎯 **功能说明**：
//   - 根据内容哈希获取已部署的静态资源文件
//   - 验证请求者权限（仅资源部署者可获取）
//   - 支持自定义保存目录或使用默认目录
//   - 自动处理文件名冲突（iOS风格递增）
//
// 📝 **实现状态**：薄实现层，委托给具体服务处理
func (m *Manager) FetchStaticResourceFile(ctx context.Context,
	contentHash []byte,
	requesterPrivateKey []byte,
	targetDir string,
) (string, error) {
	// 薄实现：纯参数透传，不做业务逻辑处理
	if m.staticDeployService == nil {
		return "", fmt.Errorf("静态资源部署服务未初始化")
	}

	// 委托给具体服务处理
	return m.staticDeployService.FetchStaticResourceFile(ctx, contentHash, requesterPrivateKey, targetDir)
}

// ████████████████████████████████████████████████████████████████████████████████████████████████
// █                                                                                              █
// █                           🔗  CONTRACT SERVICE INTERFACE                                       █
// █                                                                                              █
// █   智能合约服务：处理WASM合约的部署、调用和管理（分离独立服务）                      █
// █                                                                                              █
// ████████████████████████████████████████████████████████████████████████████████████████████████

// DeployContract 智能合约部署（支持基础和高级模式）
//
// 🎯 **功能说明**：
//   - 基础模式（options=nil）：开发者上传合约到区块链，公开可调用
//   - 高级模式（options!=nil）：企业级合约部署，支持复杂访问控制和商业化
//
// 📋 **实现状态**：薄实现层，等待后续细化
// - 🗋️ **具体实现将在**： internal/core/blockchain/transaction/contract_deploy.go
// - 📊 **自动处理特性**：WASM格式验证、执行费用消耗预估、安全性检查、依赖关系分析
// - 🔐 **访问控制模式**：personal、shared、commercial、enterprise
//
// 📝 **基础模式典型应用**：
//   - DeFi协议部署、游戏逻辑合约
//   - 投票治理、资产管理合约
//
// 📝 **高级模式支持的业务场景**：
//   - 私有合约：企业内部业务逻辑（仅授权人员可调用）
//   - 付费服务：按调用次数收费的合约服务
//   - 多签治理：需要多方签名才能升级的关键合约
//   - 定时上线：预设时间自动激活的合约功能
//
// 📝 **参数说明**：
//   - contractBytes: 合约WASM字节码文件
//   - config: 执行配置（执行费用限制、权限等）
//   - name: 合约显示名称（如："去中心化投票系统"）
//   - description: 合约功能描述
//   - options: 高级部署选项（可变参数，省略=基础部署，传入=企业级高级功能）
//
// 💡 **返回值说明**：
//   - []byte: 未签名交易哈希
//   - error: 部署错误
//
// 💡 **调用示例**：
//   - 基础部署：DeployContract(ctx, wasmBytes, config, "投票合约", "去中心化投票系统")
//   - 高级部署：DeployContract(ctx, wasmBytes, config, "投票合约", "去中心化投票系统", &deployOptions)
func (m *Manager) DeployContract(ctx context.Context,
	deployerPrivateKey []byte,
	contractFilePath string,
	config *resourcePb.ContractExecutionConfig,
	name string,
	description string,
	options ...*types.ResourceDeployOptions,
) ([]byte, error) {
	// 薄实现：纯参数透传，不做业务逻辑处理
	if m.contractDeployService == nil {
		return nil, fmt.Errorf("合约部署服务未初始化")
	}

	// 直接透传所有参数给具体服务
	return m.contractDeployService.DeployContract(ctx, deployerPrivateKey, contractFilePath, config, name, description, options...)
}

// CallContract 智能合约调用（支持基础和高级模式）
//
// 🎯 **功能说明**：
//   - 基础模式（options=nil）：用户直接调用合约方法执行业务逻辑
//   - 高级模式（options!=nil）：企业级合约调用，支持委托、多签等控制
//
// 📋 **实现状态**：薄实现层，等待后续细化
// - 🗋️ **具体实现将在**： internal/core/blockchain/transaction/contract_call.go
// - 📊 **自动处理特性**：参数类型转换、执行费用费用计算、状态一致性、异常处理
//
// 📝 **基础模式典型应用**：
//   - 代币转账、NFT交易、投票参与
//   - 查询余额、获取状态信息
//
// 📝 **高级模式支持的调用场景**：
//   - 委托调用：代理其他用户执行合约方法
//   - 多签调用：需要多方授权的重要操作
//   - 定时调用：延迟执行的合约调用
//   - 批量调用：优化执行费用费用的批量操作
//
// 📝 **参数说明**：
//   - contractAddress: 合约地址（部署后返回的地址）
//   - methodName: 方法名（如："transfer", "vote", "query"）
//   - parameters: 方法参数（JSON格式，如：{"to": "0x123", "amount": "100"}）
//   - 执行费用Limit: 执行费用限制（防止无限循环）
//   - value: 发送的代币数量（可选，如："1.5"）
//   - options: 高级调用选项（可变参数，省略=基础调用，传入=企业级高级功能）
//
// 💡 **返回值说明**：
//   - []byte: 未签名交易哈希
//   - error: 调用错误
//
// 💡 **调用示例**：
//   - 基础调用：CallContract(ctx, contractAddr, "transfer", params, 100000, "0")
//   - 高级调用：CallContract(ctx, contractAddr, "transfer", params, 100000, "0", &callOptions)
func (m *Manager) CallContract(ctx context.Context,
	callerPrivateKey []byte,
	contractAddress string,
	methodName string,
	parameters map[string]interface{},
	执行费用Limit uint64,
	value string,
	options ...*types.TransferOptions,
) ([]byte, error) {
	// 薄实现：纯参数透传，不做业务逻辑处理
	if m.contractCallService == nil {
		return nil, fmt.Errorf("合约调用服务未初始化")
	}

	// 直接透传所有参数给具体服务
	return m.contractCallService.CallContract(ctx, callerPrivateKey, contractAddress, methodName, parameters, 执行费用Limit, value, options...)
}

// ████████████████████████████████████████████████████████████████████████████████████████████████
// █                                                                                              █
// █                           🤖  AI MODEL SERVICE INTERFACE                                     █
// █                                                                                              █
// █   AI模型服务：处理AI模型的部署、推理和商业化管理（分离独立服务）                    █
// █                                                                                              █
// ████████████████████████████████████████████████████████████████████████████████████████████████

// DeployAIModel AI模型部署（支持基础和商业化模式）
//
// 🎯 **功能说明**：
//   - 基础模式（options=nil）：AI开发者上传模型到区块链，公开可用
//   - 商业化模式（options!=nil）：企业级AI模型部署和商业化，支持复杂商业模式
//
// 📋 **实现状态**：薄实现层，等待后续细化
// - 🗋️ **具体实现将在**： internal/core/blockchain/transaction/aimodel_deploy.go
// - 📊 **自动处理特性**：模型格式验证、推理性能评估、存储优化、版本管理
// - 💰 **收入分成模式**：开发者获得80%收入，平台获得20%手续费
//
// 📝 **基础模式典型应用**：
//   - 图像识别、文本分析、语音识别模型
//   - 预测模型、推荐算法、决策树模型
//
// 📝 **商业化模式支持的场景**：
//   - 按次付费：每次推理收费（如：图片识别0.01原生币/次）
//   - 订阅模式：月费制无限使用（如：文本分析99原生币/月）
//   - 分层定价：不同用户等级不同价格
//   - 企业授权：内部团队共享使用高价值模型
//
// 📝 **参数说明**：
//   - modelBytes: AI模型文件（如：PyTorch、ONNX格式）
//   - config: AI推理配置（GPU需求、内存限制等）
//   - name: 模型显示名称（如："ResNet50图像分类器"）
//   - description: 模型功能描述
//   - options: 高级部署选项（可变参数，省略=基础部署，传入=商业化模式）
//
// 💡 **返回值说明**：
//   - []byte: 未签名交易哈希
//   - error: 部署错误
//
// 💡 **调用示例**：
//   - 基础部署：DeployAIModel(ctx, modelBytes, config, "图像识别", "ResNet50模型")
//   - 商业化部署：DeployAIModel(ctx, modelBytes, config, "图像识别", "ResNet50模型", &deployOptions)
func (m *Manager) DeployAIModel(ctx context.Context,
	deployerPrivateKey []byte,
	modelFilePath string,
	config *resourcePb.AIModelExecutionConfig,
	name string,
	description string,
	options ...*types.ResourceDeployOptions,
) ([]byte, error) {
	// 薄实现：纯参数透传，不做业务逻辑处理
	if m.aiModelDeployService == nil {
		return nil, fmt.Errorf("AI模型部署服务未初始化")
	}

	// 直接透传所有参数给具体服务
	return m.aiModelDeployService.DeployAIModel(ctx, deployerPrivateKey, modelFilePath, config, name, description, options...)
}

// InferAIModel AI推理执行（支持基础和高级模式）
//
// 🎯 **功能说明**：
//   - 基础模式（options=nil）：用户使用AI模型进行推理计算
//   - 高级模式（options!=nil）：企业级推理管理，支持委托、批量、付费等
//
// 📋 **实现状态**：薄实现层，等待后续细化
// - 🗋️ **具体实现将在**： internal/core/blockchain/transaction/aimodel_infer.go
// - 📊 **自动处理特性**：输入数据预处理、推理结果后处理、性能监控、错误恢复
//
// 📝 **基础模式典型应用**：
//   - 上传图片进行识别、输入文本进行分析
//   - 实时预测、数据处理
//
// 📝 **高级模式支持的推理场景**：
//   - 批量推理：一次处理多个输入，优化费用
//   - 委托推理：代理其他用户执行推理
//   - 定时推理：延迟执行的推理任务
//   - 付费推理：自动处理费用支付和结算
//
// 📝 **参数说明**：
//   - modelAddress: 模型地址（部署后返回的地址）
//   - inputData: 输入数据（基础模式：map[string]interface{}；高级模式：支持批量interface{}）
//   - parameters: 推理参数（如：{"temperature": 0.7, "max_tokens": 100}）
//   - options: 高级推理选项（可变参数，省略=基础推理，传入=企业级高级功能）
//
// 💡 **返回值说明**：
//   - []byte: 未签名交易哈希
//   - error: 推理错误
//
// 💡 **调用示例**：
//   - 基础推理：InferAIModel(ctx, modelAddr, inputData, params)
//   - 批量推理：InferAIModel(ctx, modelAddr, batchInputData, params, &inferOptions)
func (m *Manager) InferAIModel(ctx context.Context,
	callerPrivateKey []byte,
	modelAddress string,
	inputData interface{},
	parameters map[string]interface{},
	options ...*types.TransferOptions,
) ([]byte, error) {
	// 薄实现：纯参数透传，不做业务逻辑处理
	if m.aiModelInferService == nil {
		return nil, fmt.Errorf("AI模型推理服务未初始化")
	}

	// 直接透传所有参数给具体服务
	return m.aiModelInferService.InferAIModel(ctx, callerPrivateKey, modelAddress, inputData, parameters, options...)
}

// ████████████████████████████████████████████████████████████████████████████████████████████████
// █                                                                                              █
// █                       📋  TRANSACTION MANAGER INTERFACE                                      █
// █                                                                                              █
// █   交易管理器：处理交易生命周期管理（签名、提交、状态查询、多签协作）                 █
// █                                                                                              █
// ████████████████████████████████████████████████████████████████████████████████████████████████

// ╬══════════════════════════════════════════════════════════════════════════════════════════════╖
// ║                         ✍️  交易签名和提交                                                 ║
// ╚══════════════════════════════════════════════════════════════════════════════════════════════╝

// SignTransaction 签名交易
//
// 🎯 **最关键操作**：用户对交易进行数字签名授权
//
// 📋 **实现状态**：薄实现层，等待后续细化
// - 🗋️ **具体实现将在**： internal/core/blockchain/transaction/sign.go
// - 🔐 **安全特性**：私钥本地处理、签名算法验证、交易完整性检查、防重放攻击
//
// 📝 **业务流程**：
//
//	用户确认交易详情 → 私钥签名 → 生成可提交交易
//
// 📝 **参数说明**：
//   - txHash: 未签名交易哈希（由各Service接口生成）
//   - privateKey: 用户私钥（ECDSA secp256k1格式）
//
// 💡 **返回值说明**：
//   - []byte: 已签名交易哈希（用于SubmitTransaction）
//   - error: 签名错误
func (m *Manager) SignTransaction(ctx context.Context,
	txHash []byte,
	privateKey []byte,
) ([]byte, error) {
	if m.logger != nil {
		m.logger.Debug("开始签名交易 - method: SignTransaction")
	}

	// 委托给专门的交易签名服务处理
	if m.transactionSignService == nil {
		if m.logger != nil {
			m.logger.Warn("交易签名服务未初始化")
		}
		return nil, fmt.Errorf("交易签名服务未初始化")
	}

	// 调用交易签名服务
	signedTxHash, err := m.transactionSignService.SignTransaction(ctx, txHash, privateKey)
	if err != nil {
		if m.logger != nil {
			m.logger.Error(fmt.Sprintf("交易签名失败: %v", err))
		}
		return nil, fmt.Errorf("交易签名失败: %w", err)
	}

	if m.logger != nil {
		m.logger.Info(fmt.Sprintf("✅ 交易签名完成 - signedTxHash: %x", signedTxHash))
	}

	return signedTxHash, nil
}

// SubmitTransaction 提交交易到网络
//
// 🎯 **网络广播**：将已签名交易提交到区块链网络
//
// 📋 **实现状态**：薄实现层，等待后续细化
// - 🗋️ **具体实现将在**： internal/core/blockchain/transaction/submit.go
// - 📊 **自动处理**：网络连接重试、交易格式验证、手续费检查、重复提交防护
//
// 📝 **网络流程**：
//
//	交易验证 → P2P网络广播 → 内存池排队 → 等待打包
//
// 📝 **参数说明**：
//   - signedTxHash: 已签名交易哈希（由SignTransaction生成）
//
// 💡 **返回值说明**：
//   - error: 提交错误，nil表示成功
func (m *Manager) SubmitTransaction(ctx context.Context,
	signedTxHash []byte,
) error {
	if m.logger != nil {
		m.logger.Debug("开始提交交易 - method: SubmitTransaction")
	}

	// 委托给专门的提交服务
	if m.transactionSubmitService == nil {
		if m.logger != nil {
			m.logger.Warn("交易提交服务未初始化")
		}
		return fmt.Errorf("交易提交服务未初始化")
	}

	return m.transactionSubmitService.SubmitTransaction(ctx, signedTxHash)
}

// ╬══════════════════════════════════════════════════════════════════════════════════════════════╖
// ║                         📊  交易状态查询                                                   ║
// ╚══════════════════════════════════════════════════════════════════════════════════════════════╝

// GetTransactionStatus 查询交易状态
//
// 🎯 **状态跟踪**：查询交易在区块链中的确认状态
//
// 📋 **实现状态**：薄实现层，等待后续细化
// - 🗋️ **具体实现将在**： internal/core/blockchain/transaction/status.go
//
// 📝 **状态类型**：
//   - pending：在内存池中等待确认
//   - confirmed：已被打包到区块
//   - failed：执行失败（执行费用不足等）
//
// 📝 **参数说明**：
//   - txHash: 交易哈希（签名前后均可）
//
// 💡 **返回值说明**：
//   - types.TransactionStatusEnum: 交易状态（pending/confirmed/failed）
//   - error: 查询错误
func (m *Manager) GetTransactionStatus(ctx context.Context,
	txHash []byte,
) (types.TransactionStatusEnum, error) {
	if m.logger != nil {
		m.logger.Debug("查询交易状态 - method: GetTransactionStatus")
	}

	// 委托给专门的状态服务
	if m.transactionStatusService == nil {
		if m.logger != nil {
			m.logger.Warn("交易状态服务未初始化")
		}
		return "", fmt.Errorf("交易状态服务未初始化")
	}

	return m.transactionStatusService.GetTransactionStatus(ctx, txHash)
}

// GetTransaction 查询完整交易信息
//
// 🎯 **详细查询**：获取交易的完整原始数据
//
// 📋 **实现状态**：薄实现层，等待后续细化
// - 🗋️ **具体实现将在**： internal/core/blockchain/transaction/query.go
//
// 📝 **返回信息**：
//   - 交易输入输出详情、锁定条件和解锁证明
//   - 执行结果和执行费用消耗
//
// 📝 **参数说明**：
//   - txHash: 交易哈希（签名前后均可）
//
// 💡 **返回值说明**：
//   - *transaction.Transaction: 完整的protobuf交易结构
//   - error: 查询错误
func (m *Manager) GetTransaction(ctx context.Context,
	txHash []byte,
) (*transaction.Transaction, error) {
	if m.logger != nil {
		m.logger.Debug("获取交易详情 - method: GetTransaction")
	}

	// 委托给专门的查询服务
	if m.transactionQueryService == nil {
		if m.logger != nil {
			m.logger.Warn("交易查询服务未初始化")
		}
		return nil, fmt.Errorf("交易查询服务未初始化")
	}

	return m.transactionQueryService.GetTransaction(ctx, txHash)
}

// ╬══════════════════════════════════════════════════════════════════════════════════════════════╖
// ║                         💰  费用估算和验证                                                 ║
// ╚══════════════════════════════════════════════════════════════════════════════════════════════╝

// EstimateTransactionFee 费用估算
//
// 🎯 **简单实用**：估算交易所需的基本费用
//
// 📋 **实现状态**：薄实现层，等待后续细化
// - 🗋️ **具体实现将在**： internal/core/blockchain/transaction/fee_estimation.go
//
// 📝 **参数说明**：
//   - txHash: 未签名交易哈希（用于大小计算）
//
// 💡 **返回值说明**：
//   - uint64: 预估费用（以最小单位计算）
//   - error: 估算错误
func (m *Manager) EstimateTransactionFee(ctx context.Context,
	txHash []byte,
) (uint64, error) {
	if m.logger != nil {
		m.logger.Debug("估算交易费用 - method: EstimateTransactionFee")
	}

	// 委托给费用估算服务进行处理
	if m.transactionFeeEstimateService == nil {
		if m.logger != nil {
			m.logger.Warn("费用估算服务未初始化，返回基础费用")
		}
		return 21000, nil // 返回基础费用作为fallback
	}

	// 使用费用估算服务进行精确估算
	estimatedFee, err := m.transactionFeeEstimateService.EstimateTransactionFee(ctx, txHash)
	if err != nil {
		if m.logger != nil {
			m.logger.Error(fmt.Sprintf("费用估算失败: %v", err))
		}
		return 0, fmt.Errorf("费用估算失败: %w", err)
	}

	if m.logger != nil {
		m.logger.Debug(fmt.Sprintf("费用估算完成 - 哈希: %x, 费用: %d", txHash[:8], estimatedFee))
	}

	return estimatedFee, nil
}

// ValidateTransaction 交易验证
//
// 🎯 **简单验证**：验证交易是否有效
//
// 📋 **实现状态**：薄实现层，等待后续细化
// - 🗋️ **具体实现将在**： internal/core/blockchain/transaction/validation.go
//
// 📝 **验证内容**：
//   - 交易格式正确性 - 签名有效性 - 余额充足性 - 基本规则检查
//
// 📝 **参数说明**：
//   - txHash: 交易哈希（签名前后均可）
//
// 💡 **返回值说明**：
//   - bool: 验证结果（true=通过，false=不通过）
//   - error: 验证过程中的错误
func (m *Manager) ValidateTransaction(ctx context.Context,
	txHash []byte,
) (bool, error) {
	if m.logger != nil {
		m.logger.Debug("验证交易 - method: ValidateTransaction")
	}

	// 委托给验证服务进行处理
	if m.transactionValidateService == nil {
		if m.logger != nil {
			m.logger.Warn("验证服务未初始化，无法进行交易验证")
		}
		return false, fmt.Errorf("验证服务未初始化")
	}

	// 使用验证服务进行完整验证
	isValid, err := m.transactionValidateService.ValidateTransaction(ctx, txHash)
	if err != nil {
		if m.logger != nil {
			m.logger.Error(fmt.Sprintf("交易验证失败: %v", err))
		}
		return false, fmt.Errorf("交易验证失败: %w", err)
	}

	if m.logger != nil {
		m.logger.Debug(fmt.Sprintf("交易验证完成 - 哈希: %x, 结果: %v", txHash[:8], isValid))
	}

	return isValid, nil
}

// ╬══════════════════════════════════════════════════════════════════════════════════════════════╖
// ║                         🤝  企业级多签协作                                                 ║
// ╚══════════════════════════════════════════════════════════════════════════════════════════════╝

// StartMultiSigSession 创建多签会话
//
// 🎯 **企业协作**：启动企业级多重签名工作流
//
// 📋 **实现状态**：薄实现层，等待后续细化
// - 🗋️ **具体实现将在**： internal/core/blockchain/transaction/multisig.go
//
// 📝 **典型场景**：
//   - 大额资金转移需要3-of-5高管签名
//   - 重要合约部署需要技术+法务+财务签名
//
// 📝 **参数说明**：
//   - requiredSignatures: 需要的签名数量（M，如：3）
//   - authorizedSigners: 授权签名者地址列表（N个，如：5个地址）
//   - expiryDuration: 会话过期时间（如：7天）
//   - description: 会话描述（如："Q4季度资金划拨"）
//
// 💡 **返回值说明**：
//   - string: 多签会话 ID
//   - error: 创建错误
func (m *Manager) StartMultiSigSession(ctx context.Context,
	requiredSignatures uint32,
	authorizedSigners []string,
	expiryDuration time.Duration,
	description string,
) (string, error) {
	if m.logger != nil {
		m.logger.Debug("创建多签会话 - method: StartMultiSigSession")
	}

	// 委托给多签服务进行处理
	if m.multiSigService == nil {
		if m.logger != nil {
			m.logger.Warn("多签服务未初始化，无法创建多签会话")
		}
		return "", fmt.Errorf("多签服务未初始化")
	}

	// 使用多签服务创建会话
	sessionID, err := m.multiSigService.StartMultiSigSession(ctx, requiredSignatures, authorizedSigners, expiryDuration, description)
	if err != nil {
		if m.logger != nil {
			m.logger.Error(fmt.Sprintf("多签会话创建失败: %v", err))
		}
		return "", fmt.Errorf("多签会话创建失败: %w", err)
	}

	// 将会话添加到缓存中
	session := &types.MultiSigSession{
		SessionID:          sessionID,
		RequiredSignatures: requiredSignatures,
		CurrentSignatures:  0,
		Status:             "active",
		ExpiryTime:         time.Now().Add(expiryDuration),
	}

	// 安全地添加会话到缓存
	m.sessionMutex.Lock()
	m.sessionCache[sessionID] = session
	m.sessionMutex.Unlock()

	if m.logger != nil {
		m.logger.Debug(fmt.Sprintf("多签会话创建成功 - 会话ID: %s", sessionID))
	}

	return sessionID, nil
}

// AddSignatureToMultiSigSession 添加签名到多签会话
//
// 🎯 **异步签名**：参与者异步贡献签名
//
// 📋 **实现状态**：薄实现层，等待后续细化
// - 🗋️ **具体实现将在**： internal/core/blockchain/transaction/multisig.go
//
// 📝 **工作流程**：
//
//	签名者收到通知 → 审查交易详情 → 提供数字签名 → 系统记录状态
//
// 📝 **参数说明**：
//   - sessionID: 多签会话ID（由StartMultiSigSession返回）
//   - signature: 签名数据（包含签名者身份）
//
// 💡 **返回值说明**：
//   - error: 添加签名错误，nil表示成功
func (m *Manager) AddSignatureToMultiSigSession(ctx context.Context,
	sessionID string,
	signature *types.MultiSigSignature,
) error {
	if m.logger != nil {
		m.logger.Debug("添加多签签名 - method: AddSignatureToMultiSigSession")
	}

	// 委托给多签服务进行处理
	if m.multiSigService == nil {
		if m.logger != nil {
			m.logger.Warn("多签服务未初始化，无法添加签名")
		}
		return fmt.Errorf("多签服务未初始化")
	}

	// 使用多签服务添加签名
	err := m.multiSigService.AddSignatureToMultiSigSession(ctx, sessionID, signature)
	if err != nil {
		if m.logger != nil {
			m.logger.Error(fmt.Sprintf("添加多签签名失败: %v", err))
		}
		return fmt.Errorf("添加签名失败: %w", err)
	}

	// 更新缓存中的会话状态
	m.sessionMutex.Lock()
	if cachedSession, exists := m.sessionCache[sessionID]; exists {
		cachedSession.CurrentSignatures++
		if m.logger != nil {
			m.logger.Debug(fmt.Sprintf("多签签名添加成功 - 会话ID: %s, 当前签名数: %d/%d",
				sessionID, cachedSession.CurrentSignatures, cachedSession.RequiredSignatures))
		}
	}
	m.sessionMutex.Unlock()

	return nil
}

// GetMultiSigSessionStatus 查询多签会话状态
//
// 🎯 **进度跟踪**：查询多签会话的进展状态
//
// 📋 **实现状态**：薄实现层，等待后续细化
// - 🗋️ **具体实现将在**： internal/core/blockchain/transaction/multisig.go
//
// 📝 **状态信息**：
//   - 已收集/需要签名数 - 会话状态 - 剩余有效时间
//
// 📝 **参数说明**：
//   - sessionID: 多签会话ID
//
// 💡 **返回值说明**：
//   - *types.MultiSigSession: 简化的会话状态信息
//   - error: 查询错误
func (m *Manager) GetMultiSigSessionStatus(ctx context.Context,
	sessionID string,
) (*types.MultiSigSession, error) {
	if m.logger != nil {
		m.logger.Debug("查询多签会话状态 - method: GetMultiSigSessionStatus")
	}

	// 优先从缓存获取会话状态
	m.sessionMutex.RLock()
	cachedSession, exists := m.sessionCache[sessionID]
	m.sessionMutex.RUnlock()

	if exists {
		if m.logger != nil {
			m.logger.Debug("缓存命中，返回缓存的会话状态")
		}
		return cachedSession, nil
	}

	// 委托给多签服务进行查询
	if m.multiSigService == nil {
		if m.logger != nil {
			m.logger.Warn("多签服务未初始化，无法查询会话状态")
		}
		return nil, fmt.Errorf("多签服务未初始化")
	}

	// 使用多签服务查询会话状态
	session, err := m.multiSigService.GetMultiSigSessionStatus(ctx, sessionID)
	if err != nil {
		if m.logger != nil {
			m.logger.Error(fmt.Sprintf("查询多签会话状态失败: %v", err))
		}
		return nil, fmt.Errorf("查询状态失败: %w", err)
	}

	// 更新缓存
	if session != nil {
		m.sessionMutex.Lock()
		m.sessionCache[sessionID] = session
		m.sessionMutex.Unlock()
	}

	if m.logger != nil {
		m.logger.Debug(fmt.Sprintf("多签会话状态查询成功 - 会话ID: %s, 状态: %s", sessionID, session.Status))
	}

	return session, nil
}

// FinalizeMultiSigSession 完成多签会话
//
// 🎯 **会话完成**：达到签名门限后，生成最终交易
//
// 📋 **实现状态**：薄实现层，等待后续细化
// - 🗋️ **具体实现将在**： internal/core/blockchain/transaction/multisig.go
//
// 📝 **完成条件**：
//   - 收集到足够数量的有效签名 - 所有签名验证通过 - 会话在有效期内
//
// 📝 **参数说明**：
//   - sessionID: 多签会话ID
//
// 💡 **返回值说明**：
//   - []byte: 最终交易哈希（可用于SubmitTransaction）
//   - error: 完成错误
func (m *Manager) FinalizeMultiSigSession(ctx context.Context,
	sessionID string,
) ([]byte, error) {
	if m.logger != nil {
		m.logger.Debug("完成多签会话 - method: FinalizeMultiSigSession")
	}

	// 委托给多签服务进行处理
	if m.multiSigService == nil {
		if m.logger != nil {
			m.logger.Warn("多签服务未初始化，无法完成会话")
		}
		return nil, fmt.Errorf("多签服务未初始化")
	}

	// 使用多签服务完成会话
	finalTxHash, err := m.multiSigService.FinalizeMultiSigSession(ctx, sessionID)
	if err != nil {
		if m.logger != nil {
			m.logger.Error(fmt.Sprintf("完成多签会话失败: %v", err))
		}
		return nil, fmt.Errorf("完成会话失败: %w", err)
	}

	// 更新缓存中的会话状态
	m.sessionMutex.Lock()
	if cachedSession, exists := m.sessionCache[sessionID]; exists {
		cachedSession.Status = "completed"
		cachedSession.FinalTransactionHash = finalTxHash
		if m.logger != nil {
			m.logger.Debug(fmt.Sprintf("多签会话完成成功 - 会话ID: %s, 交易哈希: %x", sessionID, finalTxHash[:8]))
		}
	}
	m.sessionMutex.Unlock()

	return finalTxHash, nil
}

// ============================================================================
//                              内部服务接口实现
// ============================================================================

// 注意：以下方法是InternalTransactionService接口的正式实现
// 这些方法是区块链内部组件的核心业务需求，不是为了兼容旧代码

// ValidateTransactionsInBlock 批量验证区块中的交易
//
// 🎯 **区块交易批量验证**：内部服务接口，供区块验证组件调用
// - 使用专业的批量验证器进行高性能验证
// - 确保区块中所有交易都符合有效性要求
//
// 📊 **性能优化**：
// - 使用专业的批量验证器，支持并行验证
// - 避免重复的哈希查找开销
// - 批量UTXO状态检查优化
//
// 参数:
//   - ctx: 上下文对象
//   - transactions: 需要验证的交易列表
//
// 返回值:
//   - bool: 是否所有交易都有效
//   - error: 验证过程中的错误
func (m *Manager) ValidateTransactionsInBlock(ctx context.Context, transactions []*transaction.Transaction) (bool, error) {
	if m.logger != nil {
		m.logger.Debugf("开始批量验证区块交易 - 数量: %d", len(transactions))
	}

	// 创建专业的区块验证器并委托验证
	validator := validation.NewBlockTransactionValidator(
		m.utxoManager,         // UTXO管理器（用于验证UTXO存在性）
		m.txHashServiceClient, // 哈希服务客户端（用于哈希验证）
		m.logger,              // 日志记录器
	)

	// 委托给专业验证器执行完整验证
	return validator.ValidateTransactionsInBlock(ctx, transactions)
}

// GetMiningTemplate 获取包含 Coinbase 在首位的完整挖矿交易模板
//
// 📝 **内部服务方法**：为内部矿工服务提供交易模板
// - 🗋️ **具体实现在**： internal/core/blockchain/transaction/mining/mining_template.go
//
// ⚠️ **使用说明**：此方法主要供内部矿工组件调用，不是公共接口的一部分
func (m *Manager) GetMiningTemplate(ctx context.Context) ([]*transaction.Transaction, error) {
	if m.logger != nil {
		m.logger.Debug("开始生成挖矿模板 - method: GetMiningTemplate")
	}

	// 薄实现：委托给专门的挖矿模板服务
	if m.miningTemplateService == nil {
		if m.logger != nil {
			m.logger.Error("挖矿模板服务未初始化")
		}
		return nil, fmt.Errorf("挖矿模板服务未初始化")
	}

	// 调用挖矿模板服务获取模板
	miningTransactions, err := m.miningTemplateService.GetMiningTemplate(ctx)
	if err != nil {
		if m.logger != nil {
			m.logger.Error(fmt.Sprintf("挖矿模板生成失败: %v", err))
		}
		return nil, fmt.Errorf("挖矿模板生成失败: %w", err)
	}

	if m.logger != nil {
		m.logger.Info(fmt.Sprintf("✅ 挖矿模板生成完成 - 交易数量: %d", len(miningTransactions)))
	}

	return miningTransactions, nil
}

// ============================================================================
//                              UTXO选择设计哲学
// ============================================================================
//
// 🎯 **UTXO选择的极简设计原则**
//
// 在Transaction Manager中，我们直接实现UTXO选择逻辑，遵循以下设计哲学：
//
// 💡 **核心理念**：
// "UTXO选择就像从购物车中选择几件商品，不需要专门创建一个'商品选择服务'"
//
// ✅ **正确的做法**：
// • 在需要UTXO的地方直接实现选择逻辑（如各模块的内部方法）
// • 使用简单有效的首次适应算法遍历选择
// • 直接调用UTXOManager.GetUTXOsByAddress()获取数据
// • 返回简单明确的结果：选中的UTXO + 找零金额
//
// ❌ **过度设计的错误**：
// • 创建UTXOBusinessService、UTXOSelectionService等独立服务
// • 使用策略模式、工厂模式等复杂设计模式
// • 封装UTXOSelectionParams、UTXOSelectionDependencies等参数对象
// • 添加健康度报告、优化建议、复杂度评分等无实际使用场景的功能
//
// 🔍 **判断标准**：
// 当考虑添加新的UTXO相关组件时，问自己：
// 1. 这个组件解决了什么**具体**问题？
// 2. 有人会**真正使用**这个功能吗？
// 3. 不添加这个组件，系统会**无法工作**吗？
//
// **如果答案不够肯定，答案就是"不需要"。**
//
// 📝 **代码示例**：
// ```go
// // 简单直接的UTXO选择实现
// func (service *SomeService) selectUTXOsForAmount(ctx context.Context, address []byte, amountStr string) {
//     // 1. 获取可用UTXO
//     allUTXOs, err := service.utxoManager.GetUTXOsByAddress(ctx, address, &assetCategory, true)
//
//     // 2. 遍历选择（首次适应）
//     for _, utxo := range allUTXOs {
//         if totalSelected >= targetAmount {
//             break
//         }
//         selectedInputs = append(selectedInputs, createTxInput(utxo))
//         totalSelected += extractAmount(utxo)
//     }
//
//     // 3. 返回结果
//     return selectedInputs, calculateChange(totalSelected, targetAmount), nil
// }
// ```
//
// ⚠️ **重构教训**：
// 本设计原则源于2024年UTXO架构重构经验，删除了多个过度设计的组件。
// 牢记：**简单的算法比复杂的架构更有价值。**

// ==================== 创世区块交易服务 ====================

// CreateGenesisTransactions 创建创世区块交易
//
// 📁 **实现模块**: genesis/creator.go
//
// 🎯 **薄实现委托模式**
//
// 委托给genesis子模块的CreateTransactions函数实现具体业务逻辑
func (m *Manager) CreateGenesisTransactions(ctx context.Context, genesisConfig interface{}) ([]*transaction.Transaction, error) {
	return m.createGenesisTransactions(ctx, genesisConfig)
}

// ValidateGenesisTransactions 验证创世交易有效性
//
// 📁 **实现模块**: genesis/validator.go
//
// 🎯 **薄实现委托模式**
//
// 委托给genesis子模块的ValidateTransactions函数实现具体业务逻辑
func (m *Manager) ValidateGenesisTransactions(ctx context.Context, transactions []*transaction.Transaction) (bool, error) {
	return m.validateGenesisTransactions(ctx, transactions)
}

// ============================================================================
//                           NetworkProtocolHandler 接口实现（委托模式）
// ============================================================================

// HandleTransactionDirect 处理交易直连传播请求
//
// 🎯 **委托给网络处理器服务**
//
// 委托给networkHandlerService子模块实现具体的网络协议处理逻辑
func (m *Manager) HandleTransactionDirect(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
	if m.networkHandlerService == nil {
		return nil, fmt.Errorf("network handler service not initialized")
	}
	return m.networkHandlerService.HandleTransactionDirect(ctx, from, reqBytes)
}

// HandleTransactionAnnounce 处理交易公告
//
// 🎯 **委托给网络处理器服务**
//
// 委托给networkHandlerService子模块实现具体的网络协议处理逻辑
func (m *Manager) HandleTransactionAnnounce(ctx context.Context, from peer.ID, topic string, data []byte) error {
	if m.networkHandlerService == nil {
		return fmt.Errorf("network handler service not initialized")
	}
	return m.networkHandlerService.HandleTransactionAnnounce(ctx, from, topic, data)
}

// ==================== 创世交易内部委托实现 ====================

// createGenesisTransactions 内部方法：委托给genesis子模块创建创世交易
func (m *Manager) createGenesisTransactions(ctx context.Context, genesisConfig interface{}) ([]*transaction.Transaction, error) {
	return genesis.CreateTransactions(
		ctx,
		genesisConfig,
		m.keyManager,
		m.addressManager,
		m.logger,
	)
}

// validateGenesisTransactions 内部方法：委托给genesis子模块验证创世交易
func (m *Manager) validateGenesisTransactions(ctx context.Context, transactions []*transaction.Transaction) (bool, error) {
	return genesis.ValidateTransactions(ctx, transactions, m.logger)
}

// ============================================================================
//                              编译时接口合规检查
// ============================================================================

// 编译时检查接口实现
// 确保Manager结构体实现了所有公共服务接口和内部服务接口
//
// 📝 **实现的公共接口**：
// - blockchain.TransactionService：统一交易服务（转账、静态资源部署）
// - blockchain.ContractService：智能合约服务（部署、调用）
// - blockchain.AIModelService：AI模型服务（部署、推理）
// - blockchain.TransactionManager：交易生命周期管理（签名、提交、查询、多签）
//
// 📝 **实现的内部接口**：
// - interfaces.InternalTransactionService：内部交易服务接口（包括挖矿模板、批量验证）
//
// ⚠️ **重要提示**：如果编译失败，说明接口实现不完整，需要添加缺失的方法
var (
	// 确保实现内部服务接口
	_ interfaces.InternalTransactionService = (*Manager)(nil)

	// 确保实现所有公共服务接口
	_ blockchain.TransactionService = (*Manager)(nil) // 统一交易服务
	_ blockchain.ContractService    = (*Manager)(nil) // 智能合约服务
	_ blockchain.AIModelService     = (*Manager)(nil) // AI模型服务
	_ blockchain.TransactionManager = (*Manager)(nil) // 交易生命周期管理器
)
