package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/weisyn/v1/internal/api/http/handlers"
	"github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	nodeiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	"github.com/weisyn/v1/pkg/interfaces/network"
	"github.com/weisyn/v1/pkg/interfaces/repository"
)

// InternalManagementServer 内部管理服务器
// 🚨 重要提醒：此服务器仅供内部使用，不对外暴露
// 提供测试网络管理、数据清理、网络重置等内部功能的API接口
type InternalManagementServer struct {
	router     *gin.Engine     // Gin路由引擎
	httpServer *http.Server    // HTTP服务器
	logger     log.Logger      // 日志记录器
	config     config.Provider // 配置提供者

	// 内部管理处理器
	managementHandler *handlers.InternalManagementHandler // 内部管理处理器

	// 核心服务依赖
	blockchainService blockchain.ChainService      // 区块链服务
	repositoryManager repository.RepositoryManager // 仓储管理器
	networkService    nodeiface.Host               // 网络服务
	networkInterface  network.Network              // 网络接口

	// 服务器状态
	isRunning bool      // 服务器运行状态
	startTime time.Time // 启动时间
}

// NewInternalManagementServer 创建内部管理服务器
// 🔒 安全注意：此服务器默认监听内部端口，不应对外暴露
func NewInternalManagementServer(
	lifecycle fx.Lifecycle,
	logger log.Logger,
	config config.Provider,
	blockchainService blockchain.ChainService,
	repositoryManager repository.RepositoryManager,
	networkService nodeiface.Host,
	networkInterface network.Network,
) *InternalManagementServer {

	// 设置Gin为发布模式，减少输出
	gin.SetMode(gin.ReleaseMode)

	// 创建路由引擎
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	// 创建内部管理处理器
	managementHandler := handlers.NewInternalManagementHandler(
		blockchainService,
		repositoryManager,
		networkService,
		networkInterface,
		config,
		logger,
	)

	// 创建服务器实例
	server := &InternalManagementServer{
		router:            router,
		logger:            logger,
		config:            config,
		managementHandler: managementHandler,
		blockchainService: blockchainService,
		repositoryManager: repositoryManager,
		networkService:    networkService,
		networkInterface:  networkInterface,
		isRunning:         false,
	}

	// 设置路由
	server.setupInternalRoutes()

	// 注册生命周期钩子
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return server.Start()
		},
		OnStop: func(ctx context.Context) error {
			return server.Stop(ctx)
		},
	})

	return server
}

// setupInternalRoutes 设置内部管理路由
// 🚨 重要：这些路由仅供内部使用，不应暴露给外部用户
func (s *InternalManagementServer) setupInternalRoutes() {
	s.logger.Info("[内部管理] 设置内部管理路由...")

	// 健康检查端点
	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"service":   "internal-management",
			"uptime":    time.Since(s.startTime).String(),
			"timestamp": time.Now(),
		})
	})

	// 内部管理API路由组
	internal := s.router.Group("/internal")

	// 测试网络管理路由组
	testNetwork := internal.Group("/test-network")
	{
		// ================================================================
		//                    🚨 阶段1：快速响应机制
		// ================================================================

		// 网络状态检查
		testNetwork.GET("/status", s.managementHandler.GetTestNetworkStatus)

		// 网络清理
		testNetwork.POST("/clean", s.managementHandler.CleanTestNetwork)

		// 测试会话管理
		testNetwork.POST("/session/start", s.managementHandler.StartTestSession)

		// 网络节点发现
		testNetwork.GET("/nodes/discover", s.managementHandler.DiscoverNetworkNodes)

		// 网络拓扑信息
		testNetwork.GET("/topology", s.managementHandler.GetNetworkTopology)

		// ================================================================
		//                    🚨 阶段2：协议增强（智能重置机制）
		// ================================================================

		// 广播网络重置
		testNetwork.POST("/broadcast-reset", s.managementHandler.BroadcastNetworkReset)

		// 网络一致性检查
		testNetwork.GET("/consistency-check", s.managementHandler.CheckNetworkConsistency)

		// 强制网络重新同步
		testNetwork.POST("/force-resync", s.managementHandler.ForceNetworkResync)

		// ================================================================
		//                    🔍 阶段3：高级网络管理功能
		// ================================================================

		// 高级网络指标
		testNetwork.GET("/metrics/advanced", s.managementHandler.GetAdvancedNetworkMetrics)

		// 导出网络状态
		testNetwork.GET("/export-state", s.managementHandler.ExportNetworkState)
	}

	// 系统管理路由组
	system := internal.Group("/system")
	{
		// 系统信息
		system.GET("/info", s.getSystemInfo)

		// 配置信息（脱敏）
		system.GET("/config", s.getSystemConfig)

		// 服务状态
		system.GET("/services", s.getServiceStatus)

		// 性能指标
		system.GET("/metrics", s.getSystemMetrics)
	}

	// 调试路由组
	debug := internal.Group("/debug")
	{
		// 调试信息
		debug.GET("/info", s.getDebugInfo)

		// 日志级别控制
		debug.POST("/log-level", s.setLogLevel)

		// 内存分析
		debug.GET("/memory", s.getMemoryInfo)

		// Goroutine 信息
		debug.GET("/goroutines", s.getGoroutineInfo)
	}

	s.logger.Info("[内部管理] 内部管理路由设置完成")
	s.logger.Info("[内部管理] 可用端点:")
	s.logger.Info("[内部管理]   GET  /health - 健康检查")
	s.logger.Info("[内部管理]   GET  /internal/test-network/status - 网络状态")
	s.logger.Info("[内部管理]   POST /internal/test-network/clean - 网络清理")
	s.logger.Info("[内部管理]   POST /internal/test-network/broadcast-reset - 广播重置")
	s.logger.Info("[内部管理]   GET  /internal/test-network/consistency-check - 一致性检查")
	s.logger.Info("[内部管理]   GET  /internal/system/info - 系统信息")
}

// Start 启动内部管理服务器
func (s *InternalManagementServer) Start() error {
	// 获取内部管理端口配置
	port := s.getInternalManagementPort()
	host := "127.0.0.1" // 仅监听本地回环地址，确保不对外暴露

	addr := fmt.Sprintf("%s:%d", host, port)

	s.logger.Infof("[内部管理] 启动内部管理服务器: %s", addr)

	// 创建HTTP服务器
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	// 记录启动时间
	s.startTime = time.Now()
	s.isRunning = true

	// 在goroutine中启动服务器
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Errorf("[内部管理] 内部管理服务器启动失败: %v", err)
			s.isRunning = false
		}
	}()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	s.logger.Infof("[内部管理] ✅ 内部管理服务器启动成功")
	s.logger.Infof("[内部管理] 🔒 仅限内部访问: http://%s", addr)
	s.logger.Warnf("[内部管理] 🚨 警告：此服务器包含敏感管理功能，请勿对外暴露")

	return nil
}

// Stop 停止内部管理服务器
func (s *InternalManagementServer) Stop(ctx context.Context) error {
	if !s.isRunning {
		return nil
	}

	s.logger.Info("[内部管理] 正在关闭内部管理服务器...")

	// 创建带超时的上下文
	stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// 优雅关闭服务器
	if err := s.httpServer.Shutdown(stopCtx); err != nil {
		s.logger.Errorf("[内部管理] 服务器关闭失败: %v", err)
		return err
	}

	s.isRunning = false
	s.logger.Info("[内部管理] ✅ 内部管理服务器已关闭")

	return nil
}

// getInternalManagementPort 获取内部管理端口
func (s *InternalManagementServer) getInternalManagementPort() int {
	// 默认内部管理端口
	defaultPort := 8090

	// TODO: 从配置文件读取内部管理端口
	// 这里可以扩展为从配置文件中读取

	return defaultPort
}

// ================================================================
//                        系统管理端点实现
// ================================================================

// getSystemInfo 获取系统信息
func (s *InternalManagementServer) getSystemInfo(c *gin.Context) {
	systemInfo := map[string]interface{}{
		"service":    "WES Internal Management",
		"version":    "1.0.0",
		"uptime":     time.Since(s.startTime).String(),
		"start_time": s.startTime,
		"status":     "running",
		"endpoints": map[string]interface{}{
			"health_check":      "/health",
			"network_status":    "/internal/test-network/status",
			"network_clean":     "/internal/test-network/clean",
			"broadcast_reset":   "/internal/test-network/broadcast-reset",
			"consistency_check": "/internal/test-network/consistency-check",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    systemInfo,
	})
}

// getSystemConfig 获取系统配置（脱敏）
func (s *InternalManagementServer) getSystemConfig(c *gin.Context) {
	configInfo := map[string]interface{}{
		"sanitized": true,
		"note":      "敏感信息已移除",
		"available": s.config != nil,
	}

	// 添加一些非敏感的配置信息
	if s.config != nil {
		// TODO: 从配置中提取非敏感信息
		configInfo["has_config"] = true
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    configInfo,
	})
}

// getServiceStatus 获取服务状态
func (s *InternalManagementServer) getServiceStatus(c *gin.Context) {
	serviceStatus := map[string]interface{}{
		"internal_management": map[string]interface{}{
			"status":     "running",
			"uptime":     time.Since(s.startTime).String(),
			"start_time": s.startTime,
		},
		"blockchain": map[string]interface{}{
			"available": s.blockchainService != nil,
			"status":    "unknown",
		},
		"network": map[string]interface{}{
			"available": s.networkService != nil,
			"status":    "unknown",
		},
		"repository": map[string]interface{}{
			"available": s.repositoryManager != nil,
			"status":    "unknown",
		},
	}

	// 获取区块链状态
	if s.blockchainService != nil {
		if chainInfo, err := s.blockchainService.GetChainInfo(context.Background()); err == nil && chainInfo != nil {
			serviceStatus["blockchain"].(map[string]interface{})["current_height"] = chainInfo.Height
			serviceStatus["blockchain"].(map[string]interface{})["status"] = "active"
		}
	}

	// 获取网络状态
	if s.networkService != nil {
		libp2pHost := s.networkService.Libp2pHost()
		if libp2pHost != nil {
			peers := libp2pHost.Network().Peers()
			serviceStatus["network"].(map[string]interface{})["connected_peers"] = len(peers)
			serviceStatus["network"].(map[string]interface{})["status"] = "active"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    serviceStatus,
	})
}

// getSystemMetrics 获取系统指标
func (s *InternalManagementServer) getSystemMetrics(c *gin.Context) {
	metrics := map[string]interface{}{
		"timestamp": time.Now(),
		"uptime":    time.Since(s.startTime).String(),
		"system": map[string]interface{}{
			"memory_usage": "unknown",
			"cpu_usage":    "unknown",
			"disk_usage":   "unknown",
		},
		"application": map[string]interface{}{
			"goroutines": "unknown",
			"gc_stats":   "unknown",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    metrics,
	})
}

// ================================================================
//                        调试端点实现
// ================================================================

// getDebugInfo 获取调试信息
func (s *InternalManagementServer) getDebugInfo(c *gin.Context) {
	debugInfo := map[string]interface{}{
		"server_status": map[string]interface{}{
			"running":    s.isRunning,
			"start_time": s.startTime,
			"uptime":     time.Since(s.startTime).String(),
		},
		"dependencies": map[string]interface{}{
			"blockchain_service": s.blockchainService != nil,
			"repository_manager": s.repositoryManager != nil,
			"network_service":    s.networkService != nil,
			"network_interface":  s.networkInterface != nil,
		},
		"handlers": map[string]interface{}{
			"management_handler": s.managementHandler != nil,
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    debugInfo,
	})
}

// setLogLevel 设置日志级别
func (s *InternalManagementServer) setLogLevel(c *gin.Context) {
	var request struct {
		Level string `json:"level"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数格式错误",
		})
		return
	}

	// TODO: 实现实际的日志级别设置逻辑
	s.logger.Infof("[内部管理] 日志级别设置请求: %s", request.Level)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "日志级别设置功能待实现",
		"data": gin.H{
			"requested_level": request.Level,
		},
	})
}

// getMemoryInfo 获取内存信息
func (s *InternalManagementServer) getMemoryInfo(c *gin.Context) {
	// TODO: 实现内存信息收集
	memoryInfo := map[string]interface{}{
		"note":      "内存信息收集功能待实现",
		"timestamp": time.Now(),
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    memoryInfo,
	})
}

// getGoroutineInfo 获取协程信息
func (s *InternalManagementServer) getGoroutineInfo(c *gin.Context) {
	// TODO: 实现协程信息收集
	goroutineInfo := map[string]interface{}{
		"note":      "协程信息收集功能待实现",
		"timestamp": time.Now(),
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    goroutineInfo,
	})
}
