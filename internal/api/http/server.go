package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/weisyn/v1/internal/api/http/handlers"
	"github.com/weisyn/v1/internal/api/http/middleware"
	"github.com/weisyn/v1/internal/api/jsonrpc"
	"github.com/weisyn/v1/internal/api/websocket"
	"github.com/weisyn/v1/internal/core/consensus/miner/quorum"
	"github.com/weisyn/v1/internal/core/infrastructure/metrics"
	core "github.com/weisyn/v1/pb/blockchain/block"
	txpb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	"github.com/weisyn/v1/pkg/interfaces/network"
	p2piface "github.com/weisyn/v1/pkg/interfaces/p2p"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/interfaces/tx"
)

// Server HTTP服务器
//
// 🎯 **最小化REST端点**
//
// 仅提供区块链节点的基础REST端点：
// - /health/*: 健康检查端点（Kubernetes风格）
// - /spv/*: SPV轻客户端端点（Merkle证明）
// - /txpool/*: 交易池查询端点
//
// 🛡️ **区块链化中间件**：
// - RequestID: 追踪
// - Metrics: 观测
// - RateLimit: 匿名限流
// - StateAnchor: 状态锚定（查询操作）
// - SignatureValidation: 签名验证（写操作，当前无写端点）
//
// 实现细节：
// - 使用 Gin 框架提供 REST API
// - 注册健康检查、SPV、交易池三类handler
// - 提供启动和停止方法
type Server struct {
	router     *gin.Engine
	httpServer *http.Server
	logger     *zap.Logger
	config     config.Provider
}

// NewServer 创建HTTP服务器
//
// 参数：
//   - logger: 日志记录器
//   - cfg: 配置提供者
//   - queryService: 查询服务（健康检查与SPV用）
//   - p2pService: P2P网络服务（健康检查用）
//   - mempool: 内存池服务（健康检查与交易池查询用）
//   - merkleManager: Merkle树管理器（SPV证明生成用）
//   - txHashCli: 交易哈希服务客户端（SPV证明用）
//   - blkHashCli: 区块哈希服务客户端（SPV证明用）
//   - txVerifier: 交易验证器（签名中间件用，当前无写端点）
//   - jsonrpcServer: JSON-RPC服务器（挂载到/rpc）
//   - wsServer: WebSocket服务器（挂载到/ws）
//   - memoryDoctor: 内存监控组件（可选，如果为 nil 则内存监控端点不可用）
//
// 返回：HTTP服务器实例（含JSON-RPC和WebSocket）
func NewServer(
	logger *zap.Logger,
	cfg config.Provider,
	queryService persistence.QueryService,
	p2pService network.Network,
	mempool mempool.TxPool,
	merkleManager crypto.MerkleTreeManager,
	txHashCli txpb.TransactionHashServiceClient,
	blkHashCli core.BlockHashServiceClient,
	txVerifier tx.TxVerifier,
	jsonrpcServer *jsonrpc.Server,
	wsServer *websocket.Server,
	memoryDoctor *metrics.MemoryDoctor,
	p2pRuntime p2piface.Service,
	nodeRuntimeState p2piface.RuntimeState,
) *Server {
	// 设置Gin模式（简化：默认使用Release模式）
	gin.SetMode(gin.ReleaseMode)

	// ✅ CLI模式：禁用Gin的默认日志输出（避免干扰CLI可视化界面）
	// Gin的日志会通过自定义中间件写入日志文件，而不是输出到终端
	if os.Getenv("WES_CLI_MODE") == "true" {
		gin.DefaultWriter = io.Discard
		gin.DefaultErrorWriter = io.Discard
	}

	// 创建Gin引擎
	router := gin.New()

	// ========== 基础中间件（框架内置） ==========
	router.Use(gin.Recovery())
	// 注意：不使用 gin.Logger()，因为：
	// 1. 它会输出到终端，干扰CLI可视化界面
	// 2. 日志已通过自定义日志中间件（middleware.Logger）写入日志文件
	// 如需HTTP请求日志，请使用自定义日志中间件

	// 错误处理中间件（必须在最后）
	router.Use(middleware.ErrorHandler(logger))

	// CORS 中间件（必须在其他中间件之前）
	apiConfig := cfg.GetAPI()
	if apiConfig.HTTP.CORSEnabled {
		router.Use(func(c *gin.Context) {
			origin := c.GetHeader("Origin")
			allowedOrigins := apiConfig.HTTP.CORSOrigins
			if len(allowedOrigins) == 0 {
				allowedOrigins = []string{"*"}
			}

			// 检查 Origin 是否允许
			allowOrigin := ""
			for _, allowed := range allowedOrigins {
				if allowed == "*" || allowed == origin {
					allowOrigin = allowed
					if allowed == "*" {
						allowOrigin = "*"
					} else {
						allowOrigin = origin
					}
					break
				}
			}

			if allowOrigin != "" {
				c.Writer.Header().Set("Access-Control-Allow-Origin", allowOrigin)
				c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
				c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-Id")
				c.Writer.Header().Set("Access-Control-Max-Age", "86400")
			}

			// 处理预检请求
			if c.Request.Method == "OPTIONS" {
				c.AbortWithStatus(http.StatusNoContent)
				return
			}

			c.Next()
		})
	}

	// ========== 区块链化中间件（可选启用） ==========
	// 注：当前 REST 端点无写操作，中间件暂保持轻量级
	// 若后续开放写端点（如 POST /api/v1/transactions），需启用完整中间件链

	// 1. RequestID 中间件（追踪）
	requestIDMiddleware := middleware.NewRequestID()
	router.Use(requestIDMiddleware.Middleware())

	// 2. Metrics 中间件（观测）- 可选
	// metricsMiddleware := middleware.NewMetrics(logger)
	// router.Use(metricsMiddleware.Middleware())

	// 3. RateLimit 中间件（匿名限流）
	// 读操作：100 QPS，写操作：10 QPS
	rateLimitMiddleware := middleware.NewRateLimit(logger, 100, 10)
	router.Use(rateLimitMiddleware.Middleware())

	// 4. StateAnchor 中间件（状态锚定，查询操作）
	stateAnchorMiddleware := middleware.NewStateAnchor(logger, queryService, queryService)
	router.Use(stateAnchorMiddleware.Middleware())

	// 5. SignatureValidation 中间件（签名验证，写操作）
	// 注：当前 REST 无写端点，此中间件暂不启用
	// 若后续开放写端点，取消注释以下代码：
	// signatureMiddleware := middleware.NewSignatureValidation(logger, txVerifier)
	// router.Use(signatureMiddleware.Middleware())
	_ = txVerifier // 避免未使用变量警告

	// 创建服务器实例
	server := &Server{
		router: router,
		logger: logger,
		config: cfg,
	}

	// 注册路由
	server.registerRoutes(queryService, p2pService, p2pRuntime, nodeRuntimeState, mempool, merkleManager, txHashCli, blkHashCli, jsonrpcServer, wsServer, memoryDoctor)

	logger.Info("HTTP server initialized with blockchain middleware",
		zap.Bool("state_anchor_enabled", true),
		zap.Bool("signature_validation_enabled", false), // 当前无写端点
		zap.Bool("jsonrpc_enabled", true),
		zap.Bool("websocket_enabled", true),
		zap.String("mode", gin.Mode()))

	return server
}

// RegisterDebugRoutes 注册调试路由（需要在生命周期中调用，因为需要额外的依赖）
//
// 参数：
//   - quorumChecker: 挖矿门闸检查器（可选）
func (s *Server) RegisterDebugRoutes(quorumChecker quorum.Checker) {
	s.registerDebugRoutes(quorumChecker)
}

// registerRoutes 注册所有路由
//
// 注册端点：
// - /rpc: JSON-RPC 2.0（主协议）
// - /ws: WebSocket订阅
// - /api/v1/health/*: 健康检查
// - /api/v1/spv/*: SPV轻客户端
// - /api/v1/txpool/*: 交易池查询
// - /api/v1/system/memory: 内存监控
func (s *Server) registerRoutes(
	queryService persistence.QueryService,
	p2pService network.Network,
	p2pRuntime p2piface.Service,
	nodeRuntimeState p2piface.RuntimeState,
	mempool mempool.TxPool,
	merkleManager crypto.MerkleTreeManager,
	txHashCli txpb.TransactionHashServiceClient,
	blkHashCli core.BlockHashServiceClient,
	jsonrpcServer *jsonrpc.Server,
	wsServer *websocket.Server,
	memoryDoctor *metrics.MemoryDoctor,
) {
	apiConfig := s.config.GetAPI()
	enabledEndpoints := []string{}

	// -1. Prometheus 指标端点（运维监控）
	// 使用默认 Registry 暴露所有已注册的指标（clock / API / 共识 / 同步 等）。
	s.router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	s.logger.Info("✅ Prometheus metrics endpoint registered", zap.String("path", "/metrics"))

	// 0. JSON-RPC 端点（主协议，DApp/钱包）
	if jsonrpcServer != nil && apiConfig.HTTP.EnableJSONRPC {
		s.router.POST("/jsonrpc", gin.WrapF(jsonrpcServer.ServeHTTP))
		s.logger.Info("✅ JSON-RPC endpoint registered", zap.String("path", "/jsonrpc"))
		enabledEndpoints = append(enabledEndpoints, "jsonrpc")

		// 兼容性别名：保留 /rpc 路径（已废弃）
		s.router.POST("/rpc", func(c *gin.Context) {
			s.logger.Warn("⚠️  /rpc endpoint is deprecated, use /jsonrpc instead")
			jsonrpcServer.ServeHTTP(c.Writer, c.Request)
		})
		s.logger.Info("⚠️  Legacy JSON-RPC endpoint registered (deprecated)", zap.String("path", "/rpc"))
	} else if !apiConfig.HTTP.EnableJSONRPC {
		s.logger.Info("⏸️  JSON-RPC endpoint disabled by config (http_enable_jsonrpc=false)")
	}

	// 0.1 WebSocket 端点（实时订阅）
	if wsServer != nil && apiConfig.HTTP.EnableWebSocket {
		s.router.GET("/ws", wsServer.HandleWebSocket)
		s.logger.Info("✅ WebSocket endpoint registered", zap.String("path", "/ws"))
		enabledEndpoints = append(enabledEndpoints, "websocket")
	} else if !apiConfig.HTTP.EnableWebSocket {
		s.logger.Info("⏸️  WebSocket endpoint disabled by config (http_enable_websocket=false)")
	}

	// API v1 路由组（REST 端点）
	if !apiConfig.HTTP.EnableREST {
		s.logger.Info("⏸️  REST endpoints disabled by config (http_enable_rest=false)")
		s.logger.Info("All routes registered",
			zap.Strings("enabled_endpoints", enabledEndpoints))
		return
	}

	// /api/v1 前缀
	apiV1 := s.router.Group("/api/v1")

	// 1. Health 端点
	// - /api/v1/health/liveness: 存活探针（进程是否存活）
	// - /api/v1/health/readiness: 就绪探针（依赖是否就绪）
	// - /api/v1/health/network: 网络状态探针（P2P连接情况）
	healthHandler := handlers.NewHealthHandler(
		s.logger,
		queryService, // ChainQuery
		queryService, // BlockQuery
		p2pService,
		mempool,
		queryService, // UTXOQuery
		queryService, // ResourceQuery
	)
	healthHandler.RegisterRoutes(apiV1)
	enabledEndpoints = append(enabledEndpoints, "health")

	// 2. SPV 端点
	// - /api/v1/spv/proof: 提交交易哈希，返回SPV证明
	// - /api/v1/spv/verify: 提交SPV证明，验证其有效性
	spvHandler := handlers.NewSPVHandler(
		s.logger,
		queryService, // BlockQuery
		queryService, // TxQuery
		merkleManager,
		txHashCli,
		blkHashCli,
	)
	spvHandler.RegisterRoutes(apiV1)
	enabledEndpoints = append(enabledEndpoints, "spv")

	// 3. TxPool 端点
	// - /api/v1/txpool/status: 交易池状态
	// - /api/v1/txpool/content: 交易池内容
	// - /api/v1/txpool/inspect: 交易池检查
	txPoolHandler := handlers.NewTxPoolHandler(s.logger, mempool)
	txPoolHandler.RegisterRoutes(apiV1)
	enabledEndpoints = append(enabledEndpoints, "txpool")

	// 3.5 Node Runtime 端点（运维控制面：sync_mode/mining/status）
	// - /api/v1/node/status
	// - /api/v1/node/sync_mode
	// - /api/v1/node/mining
	if nodeRuntimeState != nil {
		nodeStatusHandler := handlers.NewNodeStatusHandler(s.logger, nodeRuntimeState)
		nodeStatusHandler.RegisterRoutes(apiV1)
		enabledEndpoints = append(enabledEndpoints, "node")
	}

	// 3.6 Admin P2P 运维端点（仅控制面使用）
	// - /api/v1/admin/p2p/connect
	// - /api/v1/admin/p2p/status
	if p2pRuntime != nil {
		adminP2PHandler := handlers.NewAdminP2PHandler(s.logger, p2pRuntime)
		adminP2PHandler.RegisterRoutes(apiV1)
		enabledEndpoints = append(enabledEndpoints, "admin_p2p")
	}

	// 4. System Memory 端点
	// - /api/v1/system/memory: 内存使用情况（通过 MemoryDoctor 提供）
	if memoryDoctor != nil {
		memoryHandler := handlers.NewMemoryHandler(s.logger, memoryDoctor)
		memoryHandler.RegisterRoutes(apiV1)
		enabledEndpoints = append(enabledEndpoints, "system_memory")
	}

	s.logger.Info("All routes registered",
		zap.Strings("enabled_endpoints", enabledEndpoints))
}

// registerDebugRoutes 注册调试路由（需要额外依赖）
//
// 注册端点：
// - /api/v1/debug/mining/quorum: 挖矿门闸状态（需要 quorum.Checker）
func (s *Server) registerDebugRoutes(quorumChecker quorum.Checker) {
	apiV1 := s.router.Group("/api/v1")

	// Mining Debug 端点（需要 quorum.Checker）
	if quorumChecker != nil {
		miningHandler := handlers.NewMiningHandler(s.logger, quorumChecker)
		miningHandler.RegisterRoutes(apiV1)
		s.logger.Info("✅ Mining debug endpoint registered", zap.String("path", "/api/v1/debug/mining/quorum"))
	}
}

// Start 启动HTTP服务器
func (s *Server) Start(ctx context.Context, networkService p2piface.Service) error {
	if s.httpServer != nil {
		return fmt.Errorf("HTTP server already started")
	}

	apiConfig := s.config.GetAPI()
	configuredHost := apiConfig.HTTP.Host
	configuredPort := apiConfig.HTTP.Port
	addr := fmt.Sprintf("%s:%d", configuredHost, configuredPort)

	// 先创建 listener，确保端口可用（避免 ListenAndServe 在 goroutine 中失败却返回 nil）
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		// dev/test 环境下的友好行为：端口被占用时自动递增寻找可用端口，避免“多节点并跑”频繁失败。
		// prod 环境保持 fail-fast，避免静默变更服务端口。
		if errors.Is(err, syscall.EADDRINUSE) && s.config != nil && s.config.GetEnvironment() != "prod" {
			const maxPortTries = 20
			if s.logger != nil {
				s.logger.Warn("HTTP port already in use; auto-selecting another port (non-prod only)",
					zap.String("host", configuredHost),
					zap.Int("configured_port", configuredPort),
					zap.Int("max_tries", maxPortTries),
				)
			}
			found := false
			for i := 1; i <= maxPortTries; i++ {
				tryPort := configuredPort + i
				tryAddr := fmt.Sprintf("%s:%d", configuredHost, tryPort)
				l, listenErr := net.Listen("tcp", tryAddr)
				if listenErr == nil {
					listener = l
					addr = tryAddr
					found = true
					if s.logger != nil {
						s.logger.Warn("HTTP server port changed due to conflict",
							zap.Int("configured_port", configuredPort),
							zap.Int("actual_port", tryPort),
							zap.String("addr", addr),
						)
					}
					break
				}
			}
			if !found {
				return fmt.Errorf("port already in use: %s (hint: use --http-port or set api.http_port in config)", addr)
			}
		} else {
			if errors.Is(err, syscall.EADDRINUSE) {
				return fmt.Errorf("port already in use: %s (hint: use --http-port or set api.http_port in config)", addr)
			}
			return fmt.Errorf("failed to listen on %s: %w", addr, err)
		}
	}

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}

	// 在单独的goroutine中启动HTTP服务器
	go func() {
		if err := s.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP server ListenAndServe error", zap.Error(err))
		}
	}()

	s.logger.Info("HTTP server started", zap.String("addr", addr))
	return nil
}

// Stop 停止HTTP服务器
func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("HTTP server shutdown error: %w", err)
	}

	s.logger.Info("HTTP server stopped")
	return nil
}

// 注意：ToAPIServerConfig 和 NewMemoryDoctorProvider 已移除，这些功能应该在其他地方实现
