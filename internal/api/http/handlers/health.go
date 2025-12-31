package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/weisyn/v1/pkg/interfaces/mempool"
	"github.com/weisyn/v1/pkg/interfaces/network"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"go.uber.org/zap"
)

// HealthHandler 健康检查端点处理器
//
// 🏥 **Kubernetes风格健康检查**
//
// 提供三层健康检查端点：
// - /health: 完整健康报告（所有组件状态）
// - /health/live: 存活检查（进程是否响应）
// - /health/ready: 就绪检查（是否可对外服务）
//
// 实现细节：
// - 接入 ChainService 检查同步状态
// - 接入 Network 检查 P2P 连接
// - 接入 TxPool 检查内存池状态
// - 接入 Repository 检查数据库连接
type HealthHandler struct {
	logger       *zap.Logger
	startTime    time.Time
	chainQuery   persistence.ChainQuery   // 链查询服务（用于查询操作）
	blockQuery   persistence.BlockQuery    // 区块查询服务（用于获取最高块）
	p2pService   network.Network
	mempool      mempool.TxPool
	eutxoService persistence.UTXOQuery
	uresService  persistence.ResourceQuery
}

// NewHealthHandler 创建健康检查处理器
//
// 参数：
//   - logger: 日志记录器
//   - chainQuery: 链查询服务（用于查询链状态）
//   - p2pService: P2P网络服务（对等节点检查）
//   - mempool: 内存池服务（交易池状态检查）
//   - repo: 数据库管理器（数据库连接检查）
//
// 返回：健康检查处理器实例
func NewHealthHandler(
	logger *zap.Logger,
	chainQuery persistence.ChainQuery,
	blockQuery persistence.BlockQuery,
	p2pService network.Network,
	mempool mempool.TxPool,
	eutxoService persistence.UTXOQuery,
	uresService persistence.ResourceQuery,
) *HealthHandler {
	return &HealthHandler{
		logger:       logger,
		startTime:    time.Now(),
		chainQuery:   chainQuery,
		blockQuery:   blockQuery,
		p2pService:   p2pService,
		mempool:      mempool,
		eutxoService: eutxoService,
		uresService:  uresService,
	}
}

// RegisterRoutes 注册健康检查路由
//
// 注册三个健康检查端点：
// - GET /health: 完整健康报告
// - GET /health/live: Kubernetes liveness probe
// - GET /health/ready: Kubernetes readiness probe
func (h *HealthHandler) RegisterRoutes(r *gin.RouterGroup) {
	health := r.Group("/health")
	{
		health.GET("", h.GetHealth)          // 完整健康报告
		health.GET("/live", h.GetLiveness)   // 存活检查
		health.GET("/ready", h.GetReadiness) // 就绪检查
	}
}

// GetHealth 获取完整健康状态
//
// GET /api/v1/health
//
// 返回完整的健康报告，包括：
// - 整体状态（healthy/degraded/unhealthy）
// - 各组件状态（数据库、区块链、P2P、内存池）
// - 性能指标（延迟、吞吐量等）
//
// 实现细节：
// - 检查数据库连接（Repository.GetHighestBlock）
// - 检查区块链同步状态（ChainService.IsDataFresh）
// - 检查P2P连接（Network.GetPeerCount）
// - 检查内存池状态（TxPool.GetPendingTransactions）
func (h *HealthHandler) GetHealth(c *gin.Context) {
	ctx := c.Request.Context()
	uptime := time.Since(h.startTime)

	// 检查各组件状态
	components := make(map[string]interface{})
	overallHealthy := true

	// 1. 检查数据库连接
	dbStatus := h.checkDatabase(ctx)
	components["database"] = dbStatus
	if status, ok := dbStatus["status"].(string); ok && status != "healthy" {
		overallHealthy = false
	}

	// 2. 检查区块链状态
	chainStatus := h.checkBlockchain(ctx)
	components["blockchain"] = chainStatus
	if status, ok := chainStatus["status"].(string); ok && status != "healthy" {
		overallHealthy = false
	}

	// 3. 检查P2P网络
	p2pStatus := h.checkP2P(ctx)
	components["p2p"] = p2pStatus
	if status, ok := p2pStatus["status"].(string); ok && status != "healthy" {
		overallHealthy = false
	}

	// 4. 检查内存池
	mempoolStatus := h.checkMempool(ctx)
	components["mempool"] = mempoolStatus
	if status, ok := mempoolStatus["status"].(string); ok && status != "healthy" {
		overallHealthy = false
	}

	// 确定整体状态
	overallStatus := "healthy"
	if !overallHealthy {
		overallStatus = "degraded"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     overallStatus,
		"liveness":   "ok",
		"readiness":  h.determineReadiness(components),
		"version":    "v1.0.0",
		"timestamp":  time.Now().Format(time.RFC3339),
		"uptime":     uptime.String(),
		"components": components,
	})
}

// GetLiveness 存活检查（Kubernetes Liveness Probe）
//
// GET /api/v1/health/live
//
// **Kubernetes Liveness Probe**
//
// 仅检查进程是否响应，不检查业务状态。
// 失败时 Kubernetes 将重启 Pod。
//
// 实现细节：
// - 检查进程是否能响应（能执行到这里就表示存活）
// - 不检查依赖服务（避免因依赖故障导致重启）
// - 总是返回 200 OK（除非进程死锁）
func (h *HealthHandler) GetLiveness(c *gin.Context) {
	// 简单响应表示进程存活
	// 如果能执行到这里，说明进程没有死锁或崩溃
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// GetReadiness 就绪检查（Kubernetes Readiness Probe）
//
// GET /api/v1/health/ready
//
// **Kubernetes Readiness Probe**
//
// 检查节点是否已同步且可对外服务。
// 失败时 Kubernetes 将从 Service 中移除 Pod。
//
// 实现细节：
// - 检查数据库连接（Repository.GetHighestBlock）
// - 检查P2P连接（至少1个对等节点）
// - 检查同步状态（ChainService.IsDataFresh）
// - 检查内存池运行状态（TxPool可用性）
//
// 返回：
// - 200 OK：节点就绪，可对外服务
// - 503 Service Unavailable：节点未就绪
func (h *HealthHandler) GetReadiness(c *gin.Context) {
	ctx := c.Request.Context()

	// 执行所有就绪检查
	checks := make(map[string]bool)

	// 1. 检查数据库连接
	checks["database"] = h.isDatabaseReady(ctx)

	// 2. 检查P2P连接（至少1个对等节点）
	checks["p2p_connected"] = h.isP2PReady(ctx)

	// 3. 检查同步状态（是否完成同步）
	checks["sync_complete"] = h.isSyncComplete(ctx)

	// 4. 检查内存池运行状态
	checks["mempool_running"] = h.isMempoolReady(ctx)

	// 判断是否全部就绪
	allReady := true
	for _, ready := range checks {
		if !ready {
			allReady = false
			break
		}
	}

	if allReady {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ready",
			"checks":    checks,
			"timestamp": time.Now().Format(time.RFC3339),
		})
	} else {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":    "not_ready",
			"checks":    checks,
			"timestamp": time.Now().Format(time.RFC3339),
		})
	}
}
