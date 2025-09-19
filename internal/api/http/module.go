package http

import (
	"io"
	"os"

	"github.com/gin-gonic/gin"
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/consensus"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	nodeiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	"github.com/weisyn/v1/pkg/interfaces/network"
	"github.com/weisyn/v1/pkg/interfaces/repository"
	"go.uber.org/fx"
)

// 初始化HTTP模块（用于记录模块加载）
func debugHTTPModule(logger log.Logger) {
	logger.Info("HTTP API模块加载")
}

// 检查HTTP服务器依赖项
func debugHTTPDependencies(
	lifecycle fx.Lifecycle,
	config config.Provider,
	logger log.Logger,
	blockchainService blockchain.ChainService,
	transactionService blockchain.TransactionService,
	accountService blockchain.AccountService,
	blockService blockchain.BlockService,
	// 🆕 新增：智能合约和AI模型服务现在可用
	repositoryManager repository.RepositoryManager, // 仓储管理器
	resourceManager repository.ResourceManager, // 资源管理器
	consensusService consensus.MinerService,
	addressManager crypto.AddressManager,
	hashManager crypto.HashManager,
	blockHashClient core.BlockHashServiceClient,
	transactionHashClient transaction.TransactionHashServiceClient,
	networkService nodeiface.Host,
	networkInterface network.Network,
	storage storage.BadgerStore,
	txPool mempool.TxPool,
	routingTable kademlia.RoutingTableManager,
	contractService blockchain.ContractService, // 🆕 新增：合约服务
	aiModelService blockchain.AIModelService, // 🆕 新增：AI模型服务
) {
	logger.Info("HTTP服务器依赖检查")

	if lifecycle == nil {
		logger.Error("HTTP服务器缺少依赖: lifecycle为nil")
	} else {
		logger.Info("HTTP服务器依赖正常: lifecycle已注入")
	}

	if config == nil {
		logger.Error("HTTP服务器缺少依赖: config为nil")
	} else {
		logger.Infof("HTTP服务器依赖正常: config已注入，类型: %T", config)

		apiOptions := config.GetAPI()
		if apiOptions == nil {
			logger.Error("HTTP服务器配置异常: apiOptions为nil")
		} else {
			logger.Infof("HTTP服务器配置正常: apiOptions已注入，类型: %T", apiOptions)

			// 直接读取HTTP选项
			httpEnabled := apiOptions.HTTP.Enabled
			httpHost := apiOptions.HTTP.Host
			httpPort := apiOptions.HTTP.Port
			logger.Infof("HTTP服务器配置: enabled=%v host=%s port=%d", httpEnabled, httpHost, httpPort)
		}
	}

	// logger已在前面使用，如果为nil早就panic了，这里无需检查
	logger.Info("HTTP服务器依赖正常: logger已注入")

	// 检查区块链系统服务
	if blockchainService == nil {
		logger.Error("HTTP服务器缺少依赖: blockchainService为nil")
	} else {
		logger.Infof("HTTP服务器依赖正常: blockchainService已注入，类型: %T", blockchainService)
	}

	// 检查交易服务
	if transactionService == nil {
		logger.Error("HTTP服务器缺少依赖: transactionService为nil")
	} else {
		logger.Infof("HTTP服务器依赖正常: transactionService已注入，类型: %T", transactionService)
	}

	// 检查账户服务
	if accountService == nil {
		logger.Error("HTTP服务器缺少依赖: accountService为nil")
	} else {
		logger.Infof("HTTP服务器依赖正常: accountService已注入，类型: %T", accountService)
	}

	// 检查区块服务
	if blockService == nil {
		logger.Error("HTTP服务器缺少依赖: blockService为nil")
	} else {
		logger.Infof("HTTP服务器依赖正常: blockService已注入，类型: %T", blockService)
	}

	// 注意：已移除交易管理器、合约服务、AI模型服务的检查，这些服务尚未实现

	// 检查仓储管理器
	if repositoryManager == nil {
		logger.Error("HTTP服务器缺少依赖: repositoryManager为nil")
	} else {
		logger.Infof("HTTP服务器依赖正常: repositoryManager已注入，类型: %T", repositoryManager)
	}

	// 检查资源管理器
	if resourceManager == nil {
		logger.Error("HTTP服务器缺少依赖: resourceManager为nil")
	} else {
		logger.Infof("HTTP服务器依赖正常: resourceManager已注入，类型: %T", resourceManager)
	}

	// 检查共识服务
	if consensusService == nil {
		logger.Error("HTTP服务器缺少依赖: consensusService为nil")
	} else {
		logger.Infof("HTTP服务器依赖正常: consensusService已注入，类型: %T", consensusService)
	}

	// 检查地址管理器
	if addressManager == nil {
		logger.Error("HTTP服务器缺少依赖: addressManager为nil")
	} else {
		logger.Infof("HTTP服务器依赖正常: addressManager已注入，类型: %T", addressManager)
	}

	// 检查P2P网络服务
	if networkService == nil {
		logger.Error("HTTP服务器缺少依赖: networkService为nil")
	} else {
		logger.Infof("HTTP服务器依赖正常: networkService已注入，类型: %T", networkService)
	}

	// 🆕 检查存储服务
	if storage == nil {
		logger.Error("HTTP服务器缺少依赖: storage为nil")
	} else {
		logger.Infof("HTTP服务器依赖正常: storage已注入，类型: %T", storage)
	}

	// 🆕 检查交易池服务
	if txPool == nil {
		logger.Error("HTTP服务器缺少依赖: txPool为nil")
	} else {
		logger.Infof("HTTP服务器依赖正常: txPool已注入，类型: %T", txPool)
	}

	// 🆕 检查智能合约服务
	if contractService == nil {
		logger.Error("HTTP服务器缺少依赖: contractService为nil")
	} else {
		logger.Infof("HTTP服务器依赖正常: contractService已注入，类型: %T", contractService)
	}

	// 🆕 检查AI模型服务
	if aiModelService == nil {
		logger.Error("HTTP服务器缺少依赖: aiModelService为nil")
	} else {
		logger.Infof("HTTP服务器依赖正常: aiModelService已注入，类型: %T", aiModelService)
	}

	logger.Info("HTTP服务器依赖检查完成")
}

// initializeGinMode 在模块加载时初始化GIN模式
func initializeGinMode() {
	if os.Getenv("WES_CLI_MODE") == "true" {
		// CLI模式下设置为Release模式，减少调试输出
		gin.SetMode(gin.ReleaseMode)
		// 重定向GIN的默认输出到空设备，抑制控制台输出
		gin.DefaultWriter = io.Discard
		gin.DefaultErrorWriter = io.Discard
	}
}

// Module 返回HTTP服务模块
func Module() fx.Option {
	return fx.Options(
		// 首先初始化GIN模式
		fx.Invoke(initializeGinMode),

		// 增加调试日志
		fx.Invoke(debugHTTPModule),

		// 检查依赖项 - 使用 fx.Annotate 处理命名依赖
		// 检查依赖项 - 使用正确的命名标签
		fx.Invoke(
			fx.Annotate(
				debugHTTPDependencies,
				fx.ParamTags(``, ``, ``, `name:"chain_service"`, `name:"transaction_service"`, `name:"blockchain_account_service"`, `name:"block_service"`, ``, `name:"public_resource_manager"`, `name:"consensus_miner_service"`, ``, ``, ``, ``, `name:"node_host"`, `name:"network_service"`, ``, `name:"tx_pool"`, `name:"routing_table_manager"`, `name:"contract_service"`, `name:"ai_model_service"`),
			),
		),

		// 提供HTTP服务器实例 - 使用正确的命名标签
		fx.Provide(
			fx.Annotate(
				NewServer,
				fx.ParamTags(``, ``, ``, `name:"chain_service"`, `name:"transaction_service"`, `name:"blockchain_account_service"`, `name:"block_service"`, ``, `name:"public_resource_manager"`, `name:"consensus_miner_service"`, ``, ``, ``, ``, `name:"node_host"`, `name:"network_service"`, ``, `name:"tx_pool"`, `name:"routing_table_manager"`, `name:"contract_service"`, `name:"ai_model_service"`),
			),
		),

		// 🆕 提供内部管理服务器实例（仅供开发使用）
		// 🚨 重要：此服务器仅供内部开发使用，不对外暴露
		fx.Provide(
			fx.Annotate(
				NewInternalManagementServer,
				fx.ParamTags(``, ``, ``, `name:"chain_service"`, ``, `name:"node_host"`, `name:"network_service"`),
			),
		),

		// 启动HTTP服务器
		fx.Invoke(func(server *Server, logger log.Logger) {
			logger.Info("调用HTTP服务器启动函数，确保它实际被启动")
		}),

		// 🆕 启动内部管理服务器
		// 🚨 重要：此服务器仅供内部开发使用，不对外暴露
		fx.Invoke(func(internalServer *InternalManagementServer, logger log.Logger) {
			logger.Info("调用内部管理服务器启动函数，确保它实际被启动")
		}),
	)
}
