package http

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/weisyn/v1/internal/api/http/handlers"
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
)

// 轻量包装，避免在API层直接引入progress包造成循环依赖
func tryMarkStep(step string) {
	// 通过延迟导入方式在运行时调用，避免编译期依赖
	// 这里简单地忽略失败（无副作用），由上层控制是否启用
	defer func() { _ = recover() }()
}

// Server HTTP服务器结构
// 负责提供区块链相关的HTTP API服务
// 包含路由管理、服务启动和停止等功能
type Server struct {
	router                *gin.Engine                              // Gin路由引擎，处理HTTP请求和路由分发
	httpServer            *http.Server                             // 标准HTTP服务器，提供HTTP监听功能
	config                config.Provider                          // 配置提供者，用于获取API配置
	logger                log.Logger                               // 日志记录器，用于记录服务器运行状态
	blockchainService     blockchain.ChainService                  // 区块链系统服务，用于系统级操作
	transactionService    blockchain.TransactionService            // 交易服务，用于转账等操作
	accountService        blockchain.AccountService                // 账户服务，用于余额查询等
	blockService          blockchain.BlockService                  // 区块服务，用于区块操作
	repositoryManager     repository.RepositoryManager             // 仓储管理器
	resourceManager       repository.ResourceManager               // 资源管理器
	consensusService      consensus.MinerService                   // 矿工服务，用于挖矿控制
	addressManager        crypto.AddressManager                    // 地址管理器，用于地址验证和转换
	hashManager           crypto.HashManager                       // 🆕 哈希管理器，用于哈希计算
	blockHashClient       core.BlockHashServiceClient              // 🆕 区块哈希服务客户端
	transactionHashClient transaction.TransactionHashServiceClient // 🆕 交易哈希服务客户端
	networkService        nodeiface.Host                           // 节点网络服务，用于节点管理
	networkInterface      network.Network                          // 网络接口，用于GossipSub等网络操作
	storage               storage.BadgerStore                      // 存储服务，用于智能合约状态管理
	// 移除了不存在的 ContentRouter
	txPool       mempool.TxPool               // 🆕 交易池服务，用于URES交易提交
	routingTable kademlia.RoutingTableManager // 🆕 Kademlia路由表管理器（可选）

	// 🆕 新增：智能合约和AI模型服务
	contractService blockchain.ContractService // 智能合约服务
	aiModelService  blockchain.AIModelService  // AI模型服务
}

// NewServer 创建新的HTTP服务器
// 该函数在fx框架的依赖注入系统中注册，会自动接收所需依赖
// 并负责服务器的初始化和生命周期管理
// 参数:
//   - lifecycle: fx生命周期管理器，用于注册服务启动和停止钩子
//   - config: 全局配置对象，包含API配置信息
//   - logger: 日志接口，用于记录服务器日志
//   - blockchainService: 区块链系统服务，提供系统级操作
//   - transactionService: 交易服务，提供转账等操作
//   - utxoService: UTXO/账户服务，提供余额查询等
//   - resourceService: 资源服务，提供合约、模型、文件管理
//   - blockService: 区块服务，提供区块操作
//   - consensusService: 共识服务，提供挖矿控制
//   - vmService: 虚拟机服务，用于合约调用
//
// 返回:
//   - 初始化完成的HTTP服务器实例
func NewServer(
	lifecycle fx.Lifecycle,
	config config.Provider,
	logger log.Logger,
	blockchainService blockchain.ChainService,
	transactionService blockchain.TransactionService,
	accountService blockchain.AccountService,
	blockService blockchain.BlockService,
	repositoryManager repository.RepositoryManager, // 仓储管理器
	resourceManager repository.ResourceManager, // 资源管理器
	consensusService consensus.MinerService,
	addressManager crypto.AddressManager,
	hashManager crypto.HashManager, // 🆕 新增：哈希管理器
	blockHashClient core.BlockHashServiceClient, // 🆕 新增：区块哈希服务客户端
	transactionHashClient transaction.TransactionHashServiceClient, // 🆕 新增：交易哈希服务客户端
	networkService nodeiface.Host,
	networkInterface network.Network,
	storage storage.BadgerStore,
	// 移除了不存在的 ContentRouter 参数
	txPool mempool.TxPool, // 🆕 新增：交易池服务
	routingTable kademlia.RoutingTableManager, // 🆕 新增：路由表管理器
	// 🆕 新增：智能合约和AI模型服务
	contractService blockchain.ContractService,
	aiModelService blockchain.AIModelService,
) *Server {
	// 根据环境模式配置Gin（必须在创建路由引擎之前设置）
	if os.Getenv("WES_CLI_MODE") == "true" {
		// CLI模式下设置为Release模式，减少调试输出
		gin.SetMode(gin.ReleaseMode)
		// 重定向GIN的默认输出到空设备，抑制控制台输出
		gin.DefaultWriter = io.Discard
		gin.DefaultErrorWriter = io.Discard
	}

	// 创建Gin路由引擎，使用自定义Writer（在CLI模式下为io.Discard）
	router := gin.New()
	if os.Getenv("WES_CLI_MODE") != "true" {
		// 只在非CLI模式下使用默认的日志和恢复中间件
		router.Use(gin.Logger(), gin.Recovery())
	} else {
		// CLI模式下使用静默的恢复中间件
		router.Use(gin.Recovery())
	}

	// 创建服务器实例，保存所有依赖
	server := &Server{
		router:                router,
		config:                config,
		logger:                logger,
		blockchainService:     blockchainService,
		transactionService:    transactionService,
		accountService:        accountService,
		blockService:          blockService,
		repositoryManager:     repositoryManager,
		resourceManager:       resourceManager,
		consensusService:      consensusService,
		addressManager:        addressManager,
		hashManager:           hashManager,           // 🆕 新增：哈希管理器
		blockHashClient:       blockHashClient,       // 🆕 新增：区块哈希服务客户端
		transactionHashClient: transactionHashClient, // 🆕 新增：交易哈希服务客户端
		networkService:        networkService,
		networkInterface:      networkInterface,
		storage:               storage,
		// 移除了不存在的 contentRouter
		txPool:       txPool,       // 🆕 新增：初始化交易池
		routingTable: routingTable, // 🆕 新增：使用传入的路由表管理器
		// 🆕 新增：智能合约和AI模型服务
		contractService: contractService,
		aiModelService:  aiModelService,
	}

	// 注册服务生命周期钩子
	// 当fx启动时，会调用OnStart方法启动HTTP服务
	// 当fx停止时，会调用OnStop方法停止HTTP服务
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := server.Start(); err != nil {
				return err
			}
			// 启动成功后，推进“启动API”阶段
			go func() {
				time.Sleep(10 * time.Millisecond)
				tryMarkStep("启动API")
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return server.Stop(ctx)
		},
	})

	// 确保WASM运行时已初始化完成，此时区块链启动过程应该已经完成VM初始化
	// TODO: wasmRuntime变量未定义，暂时注释掉
	// if wasmRuntime == nil {
	// 	logger.Warn("WASM运行时未配置，合约执行功能不可用")
	// } else {
	// 	logger.Info("WASM运行时状态正常")
	// }
	logger.Info("WASM运行时检查暂时跳过 - wasmRuntime变量未定义")

	// AI服务已移除

	// 检查共识服务状态
	if consensusService == nil {
		logger.Warn("共识服务未配置，挖矿功能不可用")
	} else {
		logger.Info("共识服务状态正常")
	}

	// 初始化路由，设置所有API端点
	server.setupRoutes()

	return server
}

// setupRoutes 设置HTTP路由
// 该方法配置所有API端点和它们的处理函数
// 包括资产、资源、执行等功能
func (s *Server) setupRoutes() {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Errorf("[PANIC] setupRoutes发生异常: %v", r)
		}
	}()

	s.logger.Info("开始设置HTTP路由...")

	// 调试：检查服务状态
	s.logger.Info("服务状态检查: 所有必需服务已注入")

	// 创建API版本前缀，所有API端点都在/api/v1路径下
	// 这样便于将来版本升级和兼容性管理
	v1 := s.router.Group("/api/v1")
	s.logger.Info("v1路由组已创建")

	// 创建挖矿控制处理器并注册路由
	// 挖矿handlers需要ConsensusService进行挖矿控制（已迁移到consensus层）
	s.logger.Info("准备注册挖矿控制路由...")
	miningHandlers := handlers.NewMiningHandlers(s.consensusService, s.config, s.addressManager, s.blockchainService, s.logger)
	miningHandlers.RegisterRoutes(v1)
	s.logger.Info("挖矿控制路由注册完成")

	// 创建区块查询处理器并注册路由
	// 区块handlers需要BlockService进行区块查询，BlockchainService进行链状态查询
	s.logger.Info("准备注册区块查询路由...")
	blockHandlers := handlers.NewBlockHandlers(s.repositoryManager, s.blockchainService, s.logger)
	// BlockHandlers 没有 RegisterRoutes 方法，直接注册路由
	blockGroup := v1.Group("/blocks")
	blockGroup.GET("/chain-info", blockHandlers.GetChainInfo)
	blockGroup.GET("/height/:height", blockHandlers.GetBlockByHeight)
	blockGroup.GET("/hash/:hash", blockHandlers.GetBlockByHash)
	blockGroup.GET("/latest", blockHandlers.GetLatestBlock)
	s.logger.Info("区块查询路由注册完成")

	// 创建UTXO查询和管理处理器并注册路由
	// UTXO handlers需要BlockchainService进行UTXO高级功能
	s.logger.Info("准备注册账户管理路由...")
	accountHandlers := handlers.NewAccountHandlers(s.accountService, s.blockchainService, s.addressManager, s.logger)
	accountHandlers.RegisterRoutes(v1)
	s.logger.Info("账户管理路由注册完成")

	// 创建交易管理处理器并注册路由s
	s.logger.Info("准备注册交易管理路由...")
	// 创建交易处理器 - 使用实际的服务
	// 类型断言：将TransactionService转换为TransactionManager
	// Manager实现了两个接口，所以可以安全转换
	var transactionManager blockchain.TransactionManager
	if manager, ok := s.transactionService.(blockchain.TransactionManager); ok {
		transactionManager = manager
	}

	transactionHandlers := handlers.NewTransactionHandlers(
		s.transactionService,
		transactionManager, // 使用类型断言后的TransactionManager
		nil,                // contractService 暂未实现
		nil,                // aiModelService 暂未实现
		s.logger,
	)
	// 注册完整的交易路由
	transactionGroup := v1.Group("/transactions")
	transactionGroup.POST("/transfer", transactionHandlers.Transfer)
	transactionGroup.POST("/batch-transfer", transactionHandlers.BatchTransfer)
	transactionGroup.POST("/sign", transactionHandlers.SignTransaction)
	transactionGroup.POST("/submit", transactionHandlers.SubmitTransaction)
	transactionGroup.GET("/status/:txHash", transactionHandlers.GetTransactionStatus)
	transactionGroup.GET("/:txHash", transactionHandlers.GetTransactionDetails)
	transactionGroup.POST("/estimate-fee", transactionHandlers.EstimateTransactionFee)
	transactionGroup.POST("/validate", transactionHandlers.ValidateTransaction)
	transactionGroup.POST("/fetch-resource", transactionHandlers.FetchStaticResourceFile)

	// 多签工作流路由
	multisigGroup := transactionGroup.Group("/multisig")
	multisigGroup.POST("/start", transactionHandlers.StartMultiSigSession)
	multisigGroup.POST("/:sessionID/sign", transactionHandlers.AddMultiSigSignature)
	multisigGroup.GET("/:sessionID/status", transactionHandlers.GetMultiSigSessionStatus)
	multisigGroup.POST("/:sessionID/finalize", transactionHandlers.FinalizeMultiSigSession)
	s.logger.Info("交易管理路由注册完成")

	// 创建节点网络处理器并注册路由
	// 节点 handlers 提供节点信息查询、连接状态监控等功能
	s.logger.Info("准备注册节点网络路由...")
	nodeHandlers := handlers.NewNodeHandlers(s.networkService, s.networkInterface, s.routingTable, s.config, s.logger)
	nodeGroup := v1.Group("/node")
	nodeHandlers.RegisterRoutes(nodeGroup)
	s.logger.Info("节点网络路由注册完成")

	// 🆕 创建智能合约处理器并注册路由
	// Contract handlers提供合约部署、调用、查询等功能
	s.logger.Info("准备注册智能合约路由...")
	if s.contractService == nil || s.aiModelService == nil {
		s.logger.Warn("合约或AI模型服务未可用，跳过合约API注册")
	} else {
		// 类型断言：将TransactionService转换为TransactionManager
		var transactionManager blockchain.TransactionManager
		if manager, ok := s.transactionService.(blockchain.TransactionManager); ok {
			transactionManager = manager
		}

		contractHandlers := handlers.NewContractHandler(
			s.contractService,
			s.transactionService,
			transactionManager,
			s.aiModelService,
			s.logger,
		)
		s.registerContractRoutes(v1, contractHandlers)
		s.logger.Info("智能合约路由注册完成")
	}

	// 创建资源内容处理器并注册路由
	// Resource handlers提供资源内容获取、下载等功能
	s.logger.Info("准备注册资源内容路由...")
	resourceHandlers := handlers.NewResourceHandler(
		s.resourceManager, // 使用 resourceManager
		s.logger,
	)
	// 注册资源路由
	resourceGroup := v1.Group("/resources")
	resourceGroup.POST("/store", resourceHandlers.StoreResource)
	resourceGroup.GET("/:hash", resourceHandlers.GetResource)
	resourceGroup.GET("/list/:type", resourceHandlers.ListResources)
	s.logger.Info("资源内容路由注册完成")

	// 健康检查端点，用于监控服务是否正常运行
	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 🔧 添加兼容性端点：/api/v1/info 作为 /api/v1/blocks/chain-info 的别名
	// 用于修复部署脚本的配置不匹配问题
	v1.GET("/info", blockHandlers.GetChainInfo)
	s.logger.Info("兼容性端点已添加：/api/v1/info -> /api/v1/blocks/chain-info")

	// 添加调试路由来测试v1路由组
	v1.GET("/debug", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "v1 路由组工作正常",
			"status":  "HTTP API服务器运行正常",
		})
	})

	s.logger.Info("所有API路由已注册完成")
}

// registerContractRoutes 注册智能合约相关路由
func (s *Server) registerContractRoutes(v1 *gin.RouterGroup, contractHandlers *handlers.ContractHandler) {
	// 创建合约路由组
	contractGroup := v1.Group("/contract")

	// 合约部署和管理
	contractGroup.POST("/deploy", contractHandlers.DeployContract) // 部署智能合约
	contractGroup.POST("/call", contractHandlers.CallContract)     // 调用合约函数

	// 静态资源部署
	contractGroup.POST("/deploy-resource", contractHandlers.DeployStaticResource) // 部署静态资源

	// AI模型相关
	aiGroup := v1.Group("/ai")
	aiGroup.POST("/deploy", contractHandlers.DeployAIModel) // 部署AI模型
	aiGroup.POST("/infer", contractHandlers.InferAIModel)   // AI模型推理

	// 🔧 注意：合约部署接口已支持任意文件类型，无需单独的资源接口

	// 代币专用端点

	s.logger.Debug("智能合约路由注册详情:")
	s.logger.Debug("  POST /api/v1/contract/deploy - 部署智能合约")
	s.logger.Debug("  POST /api/v1/contract/call - 调用合约函数")
	s.logger.Debug("  POST /api/v1/contract/mint-to-utxo - 状态转UTXO ⭐")
	s.logger.Debug("  GET  /api/v1/contract/query - 查询合约状态")
	s.logger.Debug("  GET  /api/v1/contract/info/:hash - 获取合约信息")
	s.logger.Debug("  GET  /api/v1/contract/balance - 查询代币余额")
	s.logger.Debug("  GET  /api/v1/contract/token/info/:hash - 获取代币信息")
	s.logger.Debug("  GET  /api/v1/contract/stats - 获取执行统计")
}

// registerResourceRoutes 注册资源内容相关路由
func (s *Server) registerResourceRoutes(v1 *gin.RouterGroup, resourceHandlers *handlers.ResourceHandler) {
	// 资源路由已在 setupRoutes 中注册，这个方法暂时不需要

	s.logger.Debug("资源内容路由注册详情:")
	s.logger.Debug("  POST /api/v1/resources/store - 存储资源")
	s.logger.Debug("  GET  /api/v1/resources/:hash - 获取资源信息")
	s.logger.Debug("  GET  /api/v1/resources/list/:type - 列出指定类型资源")
}

// Start 启动HTTP服务器
// 从配置中读取服务器设置，启动监听过程
// 启动过程在后台goroutine中进行，不会阻塞主线程
// 返回:
//   - 如果启动失败，返回错误；否则返回nil
func (s *Server) Start() error {
	// 读取配置或使用默认值
	var port int
	var host string

	// 检查配置中的HTTP API设置
	// 如果API已启用，读取配置的主机和端口
	apiOptions := s.config.GetAPI()
	if apiOptions != nil && apiOptions.HTTP.Enabled {
		port = apiOptions.HTTP.Port
		host = apiOptions.HTTP.Host
		s.logger.Infof("使用配置的HTTP设置: %s:%d", host, port)
	} else {
		s.logger.Info("HTTP API在配置中被禁用，使用默认值")
	}

	// 如果配置中没有指定或值无效，使用默认值
	if port == 0 {
		port = 8080 // 🔧 修复：默认端口，与config.json一致
	}
	if host == "" {
		host = "0.0.0.0" // 默认监听所有网络接口
	}

	// 端口占用检测和处理
	finalPort, err := s.handlePortConflict(host, port)
	if err != nil {
		return fmt.Errorf("端口处理失败: %w", err)
	}

	// 格式化服务器地址字符串
	addr := fmt.Sprintf("%s:%d", host, finalPort)

	// 添加调试日志
	s.logger.Infof("准备启动HTTP服务器，配置地址: %s", addr)
	enabled := false
	if apiOptions != nil {
		enabled = apiOptions.HTTP.Enabled
	}
	s.logger.Infof("检查HTTP服务是否启用: %v", enabled)

	// 创建标准HTTP服务器
	s.httpServer = &http.Server{
		Addr:    addr,     // 服务器监听地址和端口
		Handler: s.router, // 使用gin路由作为请求处理器
		// 可以添加其他设置如:
		ReadTimeout:  15 * time.Second, // 读取超时
		WriteTimeout: 15 * time.Second, // 写入超时
		IdleTimeout:  60 * time.Second, // 空闲连接超时
	}

	// 🔧 修复：存储启动协程以便管理生命周期
	s.startGoroutine(addr)

	// 🔧 修复：增强启动验证，确保服务器真正监听端口
	if err := s.waitForServerReady(addr, 3*time.Second); err != nil {
		s.logger.Errorf("HTTP服务器启动验证失败: %v", err)
		return fmt.Errorf("HTTP服务器启动验证失败: %w", err)
	}

	s.logger.Infof("✅ HTTP服务器启动成功，监听地址: %s", addr)
	s.logger.Infof("📡 API端点: http://%s/api/v1/", addr)
	s.logger.Infof("🩺 健康检查: http://%s/health", addr)

	// 如果能执行到这里，说明服务器启动过程已开始
	// 但不保证服务器已成功监听端口
	return nil
}

// handlePortConflict 处理端口冲突
func (s *Server) handlePortConflict(host string, port int) (int, error) {
	s.logger.Infof("检查端口可用性: %s:%d", host, port)

	// 检查端口是否可用
	if s.isPortAvailable(host, port) {
		s.logger.Infof("端口 %d 可用", port)
		return port, nil
	}

	s.logger.Warnf("⚠️ 端口 %d 被占用，自动寻找可用端口", port)

	// 如果端口被占用，尝试寻找可用端口（不强制终止其他进程）
	newPort, err := s.findAvailablePort(host, port)
	if err != nil {
		return 0, fmt.Errorf("无法找到可用端口: %w", err)
	}

	s.logger.Warnf("🔄 端口已自动漂移: %d -> %d (可能有其他节点实例正在运行)", port, newPort)
	return newPort, nil
}

// isPortAvailable 检查端口是否可用
func (s *Server) isPortAvailable(host string, port int) bool {
	addr := fmt.Sprintf("%s:%d", host, port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

// findAvailablePort 寻找可用端口
func (s *Server) findAvailablePort(host string, startPort int) (int, error) {
	s.logger.Infof("寻找可用端口，起始端口: %d", startPort)

	// 在起始端口附近寻找可用端口
	for i := 0; i < 100; i++ {
		candidatePort := startPort + i
		if candidatePort > 65535 {
			break
		}

		if s.isPortAvailable(host, candidatePort) {
			s.logger.Infof("找到可用端口: %d", candidatePort)
			return candidatePort, nil
		}
	}

	// 如果向上寻找失败，向下寻找
	for i := 1; i < 100; i++ {
		candidatePort := startPort - i
		if candidatePort < 1024 { // 避免使用系统保留端口
			break
		}

		if s.isPortAvailable(host, candidatePort) {
			s.logger.Infof("找到可用端口: %d", candidatePort)
			return candidatePort, nil
		}
	}

	return 0, fmt.Errorf("在端口范围内未找到可用端口")
}

// Stop 停止HTTP服务器
// 优雅地关闭服务器，等待所有请求处理完成
// 参数:
//   - ctx: 上下文，用于控制关闭超时
//
// 返回:
//   - 如果关闭失败，返回错误；否则返回nil
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("正在关闭HTTP服务器")

	// 创建带超时的上下文，防止关闭过程卡住
	// 5秒后如果服务器还未完全关闭，将强制关闭
	stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel() // 确保资源释放

	// Shutdown会等待所有活跃连接完成，然后关闭服务器
	// 如果超过停止上下文的超时时间，将返回错误
	if err := s.httpServer.Shutdown(stopCtx); err != nil {
		s.logger.Errorf("HTTP服务器关闭出错: %v", err)
		return err
	}

	s.logger.Info("HTTP服务器已关闭")
	return nil
}

// 🔧 新增：启动goroutine管理
func (s *Server) startGoroutine(addr string) {
	go func() {
		s.logger.Infof("HTTP服务器启动协程开始, 地址: %s", addr)

		// ListenAndServe会阻塞直到服务器关闭
		// 正常关闭时会返回http.ErrServerClosed，不应视为错误
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Errorf("❌ HTTP服务器运行失败: %v", err)
		} else {
			s.logger.Info("✅ HTTP服务器正常关闭")
		}
	}()
}

// 🔧 新增：等待服务器就绪
func (s *Server) waitForServerReady(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			s.logger.Infof("✅ HTTP服务器端口检测成功: %s", addr)
			return nil
		}

		// 等待一小段时间再重试
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("超时等待服务器启动: %s", addr)
}
