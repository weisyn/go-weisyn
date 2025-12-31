package handlers

import (
	"context"
	"net/http"
	"net/http/pprof"
	"runtime"
	"sort"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/weisyn/v1/internal/core/infrastructure/metrics"
	metricsiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/metrics"
	"github.com/weisyn/v1/pkg/interfaces/network"
	p2piface "github.com/weisyn/v1/pkg/interfaces/p2p"
)

// DiagnosticsHandler 统一诊断入口处理器
//
// 🎯 **统一诊断入口**
//
// 提供统一的诊断汇总端点：
// - GET /system/diagnostics/summary: 获取节点诊断汇总（health + runtime + modules + P2P）
//
// 实现细节：
// - 合并 health、runtime、modules、P2P 等关键信息
// - 返回 top N 模块的内存占用
// - 提供 P2P 简要信息（peers、connections）
type DiagnosticsHandler struct {
	logger        *zap.Logger
	healthHandler *HealthHandler
	memoryDoctor  *metrics.MemoryDoctor
	p2pService    network.Network
}

// NewDiagnosticsHandler 创建统一诊断入口处理器
//
// 参数：
//   - logger: 日志记录器
//   - healthHandler: 健康检查处理器（用于获取 health 状态）
//   - memoryDoctor: 内存监控组件（用于获取 runtime 和 modules）
//   - p2pService: P2P 网络服务（用于获取 P2P 简要信息）
//
// 返回：统一诊断入口处理器实例
func NewDiagnosticsHandler(
	logger *zap.Logger,
	healthHandler *HealthHandler,
	memoryDoctor *metrics.MemoryDoctor,
	p2pService network.Network,
) *DiagnosticsHandler {
	return &DiagnosticsHandler{
		logger:        logger,
		healthHandler: healthHandler,
		memoryDoctor:  memoryDoctor,
		p2pService:    p2pService,
	}
}

// RegisterRoutes 注册统一诊断入口路由
//
// 注册端点：
// - GET /system/diagnostics/summary: 获取节点诊断汇总
// - GET /system/diagnostics/pprof/*: pprof 性能分析端点（生产环境 Goroutine 诊断）
func (h *DiagnosticsHandler) RegisterRoutes(r *gin.RouterGroup) {
	system := r.Group("/system")
	{
		diagnostics := system.Group("/diagnostics")
		{
			diagnostics.GET("/summary", h.GetSummary) // 获取诊断汇总

			// pprof 端点（用于生产环境 Goroutine 泄漏排查）
			// - /system/diagnostics/pprof/: 索引页
			// - /system/diagnostics/pprof/goroutine: Goroutine 堆栈（关键）
			// - /system/diagnostics/pprof/heap: 堆内存
			// - /system/diagnostics/pprof/profile: CPU 分析
			// - /system/diagnostics/pprof/trace: 执行追踪
			pprofGroup := diagnostics.Group("/pprof")
			{
				pprofGroup.GET("/", gin.WrapF(pprof.Index))
				pprofGroup.GET("/cmdline", gin.WrapF(pprof.Cmdline))
				pprofGroup.GET("/profile", gin.WrapF(pprof.Profile))
				pprofGroup.GET("/symbol", gin.WrapF(pprof.Symbol))
				pprofGroup.POST("/symbol", gin.WrapF(pprof.Symbol))
				pprofGroup.GET("/trace", gin.WrapF(pprof.Trace))
				// 主要诊断端点
				pprofGroup.GET("/goroutine", gin.WrapH(pprof.Handler("goroutine")))
				pprofGroup.GET("/heap", gin.WrapH(pprof.Handler("heap")))
				pprofGroup.GET("/allocs", gin.WrapH(pprof.Handler("allocs")))
				pprofGroup.GET("/block", gin.WrapH(pprof.Handler("block")))
				pprofGroup.GET("/mutex", gin.WrapH(pprof.Handler("mutex")))
				pprofGroup.GET("/threadcreate", gin.WrapH(pprof.Handler("threadcreate")))
			}

			// 快捷 Goroutine 诊断端点（带 debug 参数）
			diagnostics.GET("/goroutines", h.GetGoroutines)
		}
	}
}

// GetGoroutines 获取 Goroutine 详细信息
//
// GET /api/v1/system/diagnostics/goroutines?debug=1|2
//
// 参数：
//   - debug: 1=简要信息（默认），2=完整堆栈
//
// 返回 Goroutine 诊断信息，用于快速排查 Goroutine 泄漏
func (h *DiagnosticsHandler) GetGoroutines(c *gin.Context) {
	debug := c.DefaultQuery("debug", "1")

	count := runtime.NumGoroutine()

	// 返回 JSON 汇总信息
	if debug == "0" {
		c.JSON(http.StatusOK, gin.H{
			"goroutine_count": count,
			"warning":         count > 5000,
			"critical":        count > 10000,
		})
		return
	}

	// debug=1 或 debug=2 时，转发到 pprof handler
	c.Request.URL.RawQuery = "debug=" + debug
	pprof.Handler("goroutine").ServeHTTP(c.Writer, c.Request)
}

// GetSummary 获取节点诊断汇总
//
// GET /api/v1/system/diagnostics/summary
//
// 返回节点诊断汇总，包括：
// - health: 健康检查状态（live、ready）
// - runtime: 运行时资源统计（RSS、heap、goroutines、FD）
// - modules_top: Top N 模块的内存占用（按 approx_bytes 排序）
// - p2p_brief: P2P 简要信息（peers、connections）
//
// 响应格式：
//
//	{
//	  "health": {
//	    "live": true,
//	    "ready": true
//	  },
//	  "runtime": {
//	    "rss_mb": 512,
//	    "heap_alloc": 123456789,
//	    "num_goroutine": 321,
//	    "open_fds": 200,
//	    "fd_limit": 4096
//	  },
//	  "modules_top": [
//	    {
//	      "module": "mempool.txpool",
//	      "approx_bytes": 8388608,
//	      "objects": 1024
//	    }
//	  ],
//	  "p2p_brief": {
//	    "peers": 5,
//	    "connections": 7
//	  }
//	}
func (h *DiagnosticsHandler) GetSummary(c *gin.Context) {
	ctx := c.Request.Context()
	response := gin.H{}

	// 1. 获取 health 状态
	health := h.getHealthStatus(ctx)
	response["health"] = health

	// 2. 获取 runtime 统计
	if h.memoryDoctor != nil {
		stats := h.memoryDoctor.GetCurrentStats()
		runtime := gin.H{
			"rss_mb":        stats.RSSMB,
			"heap_alloc":    stats.HeapAlloc,
			"num_goroutine": stats.NumGoroutine,
			"open_fds":      stats.OpenFDs,
			"fd_limit":      stats.FDLimit,
		}
		response["runtime"] = runtime

		// 3. 获取 top N 模块（按 approx_bytes 排序，取前 3 个）
		modulesTop := h.getTopModules(stats.Modules, 3)
		response["modules_top"] = modulesTop
	} else {
		response["runtime"] = gin.H{
			"error": "memory doctor not available",
		}
		response["modules_top"] = []interface{}{}
	}

	// 4. 获取 P2P 简要信息
	p2pBrief := h.getP2PBrief(ctx)
	response["p2p_brief"] = p2pBrief

	c.JSON(http.StatusOK, response)
}

// getHealthStatus 获取健康检查状态
func (h *DiagnosticsHandler) getHealthStatus(ctx context.Context) gin.H {
	if h.healthHandler == nil {
		return gin.H{
			"live":  false,
			"ready": false,
		}
	}

	// 简单检查：如果能响应，认为 live = true
	// ready 状态需要检查各组件
	live := true
	ready := h.healthHandler.isDatabaseReady(ctx) &&
		h.healthHandler.isP2PReady(ctx) &&
		h.healthHandler.isSyncComplete(ctx) &&
		h.healthHandler.isMempoolReady(ctx)

	return gin.H{
		"live":  live,
		"ready": ready,
	}
}

// getTopModules 获取 Top N 模块（按 approx_bytes 排序）
func (h *DiagnosticsHandler) getTopModules(modules []metricsiface.ModuleMemoryStats, topN int) []gin.H {
	// 按 approx_bytes 降序排序
	sort.Slice(modules, func(i, j int) bool {
		return modules[i].ApproxBytes > modules[j].ApproxBytes
	})

	// 取前 topN 个
	result := make([]gin.H, 0)
	for i := 0; i < topN && i < len(modules); i++ {
		result = append(result, gin.H{
			"module":       modules[i].Module,
			"approx_bytes": modules[i].ApproxBytes,
			"objects":      modules[i].Objects,
		})
	}

	return result
}

// getP2PBrief 获取 P2P 简要信息
func (h *DiagnosticsHandler) getP2PBrief(ctx context.Context) gin.H {
	peers := 0
	connections := 0

	if h.p2pService == nil {
		return gin.H{
			"peers":       peers,
			"connections": connections,
		}
	}

	// 通过 P2P Service 接口获取诊断信息
	if p2pSvc, ok := h.p2pService.(interface{ P2P() p2piface.Service }); ok {
		if diag := p2pSvc.P2P().Diagnostics(); diag != nil {
			// 通过 Diagnostics 接口获取真实的 peers 和 connections 数量
			peers = diag.GetPeersCount()
			connections = diag.GetConnectionsCount()
		}
	}

	return gin.H{
		"peers":       peers,
		"connections": connections,
	}
}
