package api

import (
	"context"
	"fmt"

	"github.com/weisyn/v1/internal/api/grpc"
	"github.com/weisyn/v1/internal/api/http"
	"github.com/weisyn/v1/internal/api/jsonrpc"
	"github.com/weisyn/v1/internal/api/jsonrpc/methods"
	"github.com/weisyn/v1/internal/api/websocket"
	"github.com/weisyn/v1/internal/core/consensus/miner/quorum"
	core "github.com/weisyn/v1/pb/blockchain/block"
	txpb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/consensus"
	cryptoInterface "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/ispc"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	"github.com/weisyn/v1/pkg/interfaces/network"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/interfaces/tx"
	ures "github.com/weisyn/v1/pkg/interfaces/ures"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Module 返回 API 网关模块的 fx.Option
// 🌐 区块链节点 API 网关模块
//
// 提供四协议栈接入：
// - JSON-RPC 2.0（主协议，DApp/钱包）
// - HTTP REST（运维/人类可读）
// - WebSocket（实时订阅，重组安全）
// - gRPC（高性能，暂为骨架）
//
// 依赖：
// - pkg/interfaces/blockchain（ChainService、AccountService）
// - pkg/interfaces/tx（TxVerifier）
// - pkg/interfaces/mempool（TxPool）
// - pkg/interfaces/repository（RepositoryManager、UTXOManager）
// - pkg/interfaces/ispc（ISPCCoordinator）
// - pkg/interfaces/infrastructure/crypto（MerkleTreeManager）
// - pkg/interfaces/infrastructure/event（EventBus）
// - pkg/interfaces/network（Network）
// - pb/blockchain（BlockHashService、TransactionHashService）
func Module() fx.Option {
	return fx.Module("api",
		// ========== API 模块专用 Logger ==========
		// 🎯 为 API 模块提供带 module 字段的 logger，日志将路由到 node-business.log
		fx.Provide(
			fx.Annotate(
				func(baseLogger *zap.Logger) *zap.Logger {
					if baseLogger == nil {
						return nil
					}
					return baseLogger.With(zap.String("module", "api"))
				},
				fx.ParamTags(``),                   // 从日志模块获取基础 logger（无标签）
				fx.ResultTags(`name:"api_logger"`), // 将结果标记为命名 logger，避免与全局 *zap.Logger 冲突
			),
		),
		// ========== JSON-RPC 方法处理器 ==========
		fx.Provide(
			// Chain 方法（需要命名的 ChainQuery、BlockQuery 和 SystemSyncService）
			fx.Annotate(
				methods.NewChainMethods,
				fx.ParamTags(
					`name:"api_logger"`,   // logger *zap.Logger（API 专用 logger）
					`name:"chain_query"`,  // persistence.ChainQuery
					`name:"block_query"`,  // persistence.BlockQuery
					`name:"sync_service"`, // chain.SystemSyncService
					``,                    // config.Provider
					``,                    // core.BlockHashServiceClient
					`optional:"true"`,     // resourcesvc.ResourceViewService（可选）
				),
			),
			// Block 方法（需要命名的 BlockQuery）
			fx.Annotate(
				methods.NewBlockMethods,
				fx.ParamTags(
					`name:"api_logger"`,  // logger *zap.Logger（API 专用 logger）
					`name:"block_query"`, // persistence.BlockQuery
					``,                   // core.BlockHashServiceClient
					``,                   // txpb.TransactionHashServiceClient
				),
			),
			methods.NewTxMethods, // Transaction 方法（使用 fx.In 结构体，标签在结构体字段上）
			// State 方法（需要命名的 AccountQuery, UTXOQuery, BlockQuery, ISPCCoordinator, AddressManager）
			fx.Annotate(
				methods.NewStateMethods,
				fx.ParamTags(
					`name:"api_logger"`,    // logger *zap.Logger（API 专用 logger）
					`name:"account_query"`, // persistence.AccountQuery
					`name:"utxo_query"`,    // persistence.UTXOQuery
					`name:"block_query"`,   // persistence.BlockQuery
					``,                     // ispc.ISPCCoordinator
					``,                     // cryptoInterface.AddressManager
				),
			),
			// TxPool 方法（需要命名的 TxPool 和 AddressManager）
			fx.Annotate(
				methods.NewTxPoolMethods,
				fx.ParamTags(
					`name:"api_logger"`, // logger *zap.Logger（API 专用 logger）
					`name:"tx_pool"`,    // mempool.TxPool
					``,                  // cryptoInterface.AddressManager
				),
			),
			// Mining 方法（需要 MinerService 依赖，可选）
			fx.Annotate(
				NewMiningMethodsProvider,
				fx.ParamTags(
					`name:"api_logger"`, // logger *zap.Logger（API 专用 logger）
					`name:"consensus_miner_service" optional:"true"`, // MinerService（可选）
					``,                          // cryptoInterface.AddressManager
					`name:"node_runtime_state"`, // p2p.RuntimeState（状态机模型，由 P2P 模块管理）
					`name:"mining_quorum_checker" optional:"true"`, // miner/quorum.Checker（可选，仅查询）
				),
			),
			// Subscribe 方法（暂时禁用，需要 SubscriptionManager 实现）
			func(logger *zap.Logger) *methods.SubscribeMethods {
				// 临时返回 nil SubscriptionManager 的实例
				return methods.NewSubscribeMethods(logger, nil)
			},
			// Admin P2P 管理方法（需要 P2P Service）
			fx.Annotate(
				methods.NewAdminP2PMethods,
				fx.ParamTags(
					`name:"api_logger"`,  // logger *zap.Logger（API 专用 logger）
					`name:"p2p_service"`, // p2p.Service
				),
			),
			// Sync 诊断方法（仅需要 logger）
			fx.Annotate(
				methods.NewSyncMethods,
				fx.ParamTags(
					`name:"api_logger"`, // logger *zap.Logger（API 专用 logger）
				),
			),
		),

		// ========== 协议服务器 ==========
		fx.Provide(
			NewJSONRPCServer, // JSON-RPC 服务器
			fx.Annotate(
				http.NewServer, // HTTP REST 服务器
				fx.ParamTags(
					`name:"api_logger"`,         // *zap.Logger（API 专用 logger）
					``,                          // config.Provider
					`name:"query_service"`,      // persistence.QueryService
					`name:"network_service"`,    // network.Network
					`name:"tx_pool"`,            // mempool.TxPool
					``,                          // crypto.MerkleTreeManager
					``,                          // txpb.TransactionHashServiceClient
					``,                          // core.BlockHashServiceClient
					`name:"tx_verifier"`,        // tx.TxVerifier
					``,                          // *jsonrpc.Server
					``,                          // *websocket.Server
					``,                          // *metrics.MemoryDoctor（可选）
					`name:"p2p_service"`,        // p2p.Service（P2P运行时）
					`name:"node_runtime_state"`, // p2p.RuntimeState（节点运行时状态）
				),
			),
			websocket.NewServer, // WebSocket 服务器
			grpc.NewServer,      // gRPC 服务器（已启用反射）
		),

		// ========== 生命周期管理 ==========
		fx.Invoke(
			fx.Annotate(
				registerAPIServers,
				fx.ParamTags(``, ``, ``, ``, ``, ``, ``, `name:"p2p_service"`, `name:"mining_quorum_checker" optional:"true"`),
			),
		),
	)
}

// NewJSONRPCServer 创建 JSON-RPC 服务器并注册所有方法
func NewJSONRPCServer(
	logger *zap.Logger,
	chainMethods *methods.ChainMethods,
	blockMethods *methods.BlockMethods,
	txMethods *methods.TxMethods,
	stateMethods *methods.StateMethods,
	txPoolMethods *methods.TxPoolMethods,
	miningMethods *methods.MiningMethods,
	subscribeMethods *methods.SubscribeMethods,
	adminP2PMethods *methods.AdminP2PMethods,
	syncMethods *methods.SyncMethods,
) *jsonrpc.Server {
	server := jsonrpc.NewServer(logger)

	// 注册 Chain 方法
	server.RegisterMethod("net_version", chainMethods.NetVersion)
	server.RegisterMethod("wes_chainId", chainMethods.ChainID)
	server.RegisterMethod("wes_syncing", chainMethods.Syncing)
	server.RegisterMethod("wes_getSyncStatus", chainMethods.GetSyncStatus)
	server.RegisterMethod("wes_getChainIdentity", chainMethods.GetChainIdentity)
	server.RegisterMethod("wes_blockNumber", chainMethods.BlockNumber)
	server.RegisterMethod("wes_getBlockHash", chainMethods.GetBlockHash)
	server.RegisterMethod("wes_getNetworkStats", chainMethods.GetNetworkStats)

	// 注册 Block 方法
	server.RegisterMethod("wes_getBlockByHeight", blockMethods.GetBlockByHeight)
	server.RegisterMethod("wes_getBlockByHash", blockMethods.GetBlockByHash)

	// 注册 Transaction 方法
	server.RegisterMethod("wes_getTransactionByHash", txMethods.GetTransactionByHash)
	server.RegisterMethod("wes_getTransactionReceipt", txMethods.GetTransactionReceipt)
	server.RegisterMethod("wes_getTransactionHistory", txMethods.GetTransactionHistory)
	server.RegisterMethod("wes_sendTransaction", txMethods.SendTransaction) // 完整转账接口（构建+签名+提交）
	server.RegisterMethod("wes_sendRawTransaction", txMethods.SendRawTransaction)
	server.RegisterMethod("wes_estimateFee", txMethods.EstimateFee)
	server.RegisterMethod("wes_buildTransaction", txMethods.BuildTransaction) // 通用交易构建 API
	// 通用交易签名辅助 API（供 SDK 使用）
	server.RegisterMethod("wes_computeSignatureHashFromDraft", txMethods.ComputeSignatureHashFromDraft)
	server.RegisterMethod("wes_finalizeTransactionFromDraft", txMethods.FinalizeTransactionFromDraft)

	// 注册 State 方法
	server.RegisterMethod("wes_getBalance", stateMethods.GetBalance)
	server.RegisterMethod("wes_getContractTokenBalance", stateMethods.GetContractTokenBalance)
	server.RegisterMethod("wes_getUTXO", stateMethods.GetUTXO)
	server.RegisterMethod("wes_call", stateMethods.Call)

	// 注册 TxPool 方法
	server.RegisterMethod("wes_txpool_status", txPoolMethods.TxPoolStatus)
	server.RegisterMethod("wes_txpool_content", txPoolMethods.TxPoolContent)
	server.RegisterMethod("wes_txpool_inspect", txPoolMethods.TxPoolInspect)

	// 注册 Mining 方法
	server.RegisterMethod("wes_startMining", miningMethods.StartMining)
	server.RegisterMethod("wes_stopMining", miningMethods.StopMining)
	server.RegisterMethod("wes_getMiningStatus", miningMethods.GetMiningStatus)
	server.RegisterMethod("wes_getMiningQuorumStatus", miningMethods.GetMiningQuorumStatus)

	// 注册 Contract 方法（智能合约）
	server.RegisterMethod("wes_deployContract", txMethods.DeployContract)
	server.RegisterMethod("wes_deployAIModel", txMethods.DeployAIModel)
	server.RegisterMethod("wes_callContract", txMethods.CallContract)
	server.RegisterMethod("wes_getContract", txMethods.GetContract)

	// 注册 AI Model 方法（AI模型）
	server.RegisterMethod("wes_callAIModel", txMethods.CallAIModel)

	// 注册 Resource 查询方法（基于 UTXO 视图的 ResourceViewService）
	server.RegisterMethod("wes_listResources", txMethods.ListResources)           // 资源列表（UTXO 视图）
	server.RegisterMethod("wes_getResource", txMethods.GetResource)               // 资源详情（UTXO 视图）
	server.RegisterMethod("wes_getResourceHistory", txMethods.GetResourceHistory) // 资源历史（UTXO 视图）
	server.RegisterMethod("wes_getResourceByContentHash", txMethods.GetResourceByContentHash)
	server.RegisterMethod("wes_getResourceTransaction", txMethods.GetResourceTransaction)
	server.RegisterMethod("wes_getResourceCode", txMethods.GetResourceCode)
	server.RegisterMethod("wes_getResourceABI", txMethods.GetResourceABI)

	// 注册 Pricing 查询方法（Phase 2: 定价状态查询）
	server.RegisterMethod("wes_getPricingState", txMethods.GetPricingState)

	// 注册费用预估方法（Phase 4: 费用预估）
	server.RegisterMethod("wes_estimateComputeFee", txMethods.EstimateComputeFee)

	// 注册 Subscribe 方法（仅 WebSocket 可用）
	server.RegisterMethod("wes_subscribe", subscribeMethods.Subscribe)
	server.RegisterMethod("wes_unsubscribe", subscribeMethods.Unsubscribe)

	// 注册 Admin P2P 管理方法（节点控制面）
	server.RegisterMethod("wes_admin_connectPeer", adminP2PMethods.ConnectPeer)
	server.RegisterMethod("wes_admin_getP2PStatus", adminP2PMethods.GetP2PStatus)

	// 注册 Sync 诊断方法（同步可观测性）
	server.RegisterMethod("wes_getSyncDiagnostics", syncMethods.GetSyncDiagnostics)
	server.RegisterMethod("wes_getSyncFailureHistory", syncMethods.GetSyncFailureHistory)
	server.RegisterMethod("wes_getNetworkHeightHistory", syncMethods.GetNetworkHeightHistory)

	logger.Info("JSON-RPC server initialized",
		zap.Int("registered_methods", 38)) // 新增3个sync诊断方法

	return server
}

// registerAPIServers 注册 API 服务器到生命周期
func registerAPIServers(
	lifecycle fx.Lifecycle,
	logger *zap.Logger,
	cfg config.Provider,
	httpServer *http.Server,
	wsServer *websocket.Server,
	jsonrpcServer *jsonrpc.Server,
	grpcServer *grpc.Server,
	p2pService p2pi.Service, // 从 P2P 模块获取服务
	quorumChecker quorum.Checker, // V2：挖矿门闸检查器（可选，用于 debug 端点）
) {
	// 注意：内存监控注册已移除，因为接口类型无法直接注册
	// 如果需要内存监控，应该在具体实现类型上实现 MemoryReporter 接口
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			apiConfig := cfg.GetAPI()
			logger.Info("🌐 Starting API Gateway servers...")

			// 启动 HTTP Server（包含 REST + JSON-RPC + WebSocket）
			if apiConfig.HTTP.Enabled {
				// 注册调试路由（需要额外依赖）
				if quorumChecker != nil {
					httpServer.RegisterDebugRoutes(quorumChecker)
				}

				if err := httpServer.Start(ctx, p2pService); err != nil {
					return fmt.Errorf("failed to start HTTP server on %s:%d: %w",
						apiConfig.HTTP.Host, apiConfig.HTTP.Port, err)
				}
				logger.Info("✅ HTTP Server started",
					zap.String("addr", fmt.Sprintf("%s:%d", apiConfig.HTTP.Host, apiConfig.HTTP.Port)),
					zap.Bool("rest", apiConfig.HTTP.EnableREST),
					zap.Bool("jsonrpc", apiConfig.HTTP.EnableJSONRPC),
					zap.Bool("websocket", apiConfig.HTTP.EnableWebSocket))
			} else {
				logger.Info("⏸️  HTTP Server disabled by config (http_enabled=false)")
			}

			// 启动 gRPC Server（含反射）
			if apiConfig.GRPC.Enabled {
				if err := grpcServer.Start(ctx); err != nil {
					return fmt.Errorf("failed to start gRPC server on %s:%d: %w",
						apiConfig.GRPC.Host, apiConfig.GRPC.Port, err)
				}
				actual := ""
				if grpcServer != nil {
					actual = grpcServer.Address()
				}
				logger.Info("✅ gRPC Server started",
					zap.String("addr", fmt.Sprintf("%s:%d", apiConfig.GRPC.Host, apiConfig.GRPC.Port)),
					zap.String("actual_addr", actual))
			} else {
				logger.Info("⏸️  gRPC Server disabled by config (grpc_enabled=false)")
			}

			logger.Info("✅ API Gateway initialization complete")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			apiConfig := cfg.GetAPI()
			logger.Info("🛑 Stopping API Gateway servers...")

			// 停止 gRPC Server
			if apiConfig.GRPC.Enabled {
				if err := grpcServer.Stop(ctx); err != nil {
					logger.Warn("Failed to stop gRPC server gracefully", zap.Error(err))
				} else {
					logger.Info("✅ gRPC Server stopped")
				}
			}

			// 停止 HTTP Server
			if apiConfig.HTTP.Enabled {
				if err := httpServer.Stop(ctx); err != nil {
					logger.Error("Failed to stop HTTP server", zap.Error(err))
					return err
				}
				logger.Info("✅ HTTP Server stopped")
			}

			logger.Info("✅ API Gateway shutdown complete")
			return nil
		},
	})
}

// ============================================================================
//                              模块输入依赖
// ============================================================================

// ModuleInput 定义 api 模块的输入依赖
//
// 🎯 **依赖组织**：
// 本结构体使用fx.In标签，通过依赖注入自动提供所有必需的组件依赖。
// 注意：API 模块的特性决定了它主要消费其他模块的服务，而不导出服务给其他模块。
// 因此，此结构体主要用于文档化和未来可能的统一化需求。
//
// ⚠️ **当前状态**：
// 此结构体目前未被直接使用，各个服务器创建函数直接使用 fx 注入的依赖。
// 保留此结构体是为了保持与其他模块的一致性，并便于未来可能的统一化。
type ModuleInput struct {
	fx.In

	// ========== 基础设施组件 ==========
	Logger *zap.Logger     `optional:"true"`  // 日志记录器
	Config config.Provider `optional:"false"` // 配置提供者

	// ========== 存储组件 ==========
	EventStore storage.BadgerStore `optional:"true"` // 事件存储（可选）

	// ========== 数据层依赖 ==========
	QueryService persistence.QueryService `optional:"false" name:"query_service"` // 统一查询服务

	// ========== 交易域依赖 ==========
	TxVerifier tx.TxVerifier `optional:"false" name:"tx_verifier"` // 交易验证器

	// ========== 内存池依赖 ==========
	Mempool mempool.TxPool `optional:"false" name:"tx_pool"` // 交易内存池

	// ========== URES 域依赖 ==========
	URESCAS ures.CASStorage `optional:"false" name:"cas_storage"` // CAS存储服务

	// ========== 执行引擎依赖 ==========
	ISPCCoordinator ispc.ISPCCoordinator `optional:"true"` // ISPC执行协调器（替代直接的WASM引擎）

	// ========== 密码学组件 ==========
	MerkleManager cryptoInterface.MerkleTreeManager `optional:"true"` // Merkle树管理器

	// ========== 事件总线 ==========
	EventBus event.EventBus `optional:"true"` // 事件总线（可选）

	// ========== 网络组件 ==========
	P2PService network.Network `optional:"true" name:"network_service"` // P2P网络服务

	// ========== 哈希服务客户端 ==========
	TxHashService    txpb.TransactionHashServiceClient `optional:"false"` // 交易哈希服务客户端
	BlockHashService core.BlockHashServiceClient       `optional:"false"` // 区块哈希服务客户端

	// ========== 节点运行时状态 ==========
	NodeRuntimeState p2pi.RuntimeState `optional:"false" name:"node_runtime_state"` // 节点运行时状态（状态机模型，由 P2P 模块管理）
}

// NewMiningMethodsProvider 创建 MiningMethods 提供者（处理可选的 MinerService）
func NewMiningMethodsProvider(
	logger *zap.Logger,
	minerService consensus.MinerService,
	addressManager cryptoInterface.AddressManager,
	nodeRuntimeState p2pi.RuntimeState, // ✅ Phase 2.4：使用状态机模型（由 P2P 模块管理）
	quorumChecker quorum.Checker, // V2：挖矿门闸状态查询（可选）
) *methods.MiningMethods {
	// MinerService 可能为 nil（如果共识模块未启用）
	if minerService == nil {
		logger.Warn("⚠️  MinerService 未提供，挖矿API将返回错误提示")
	}
	return methods.NewMiningMethods(logger, minerService, addressManager, nodeRuntimeState, quorumChecker)
}
