package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/weisyn/v1/internal/core/infrastructure/metrics"
)

// MemoryHandler 内存监控端点处理器
//
// 📊 **内存监控接口**
//
// 提供内存状态查询端点：
// - GET /system/memory: 获取当前内存状态（runtime + 各模块统计）
//
// 实现细节：
// - 接入 MemoryDoctor 获取当前内存采样数据
// - 返回 runtime.MemStats 和所有模块的内存统计
type MemoryHandler struct {
	logger       *zap.Logger
	memoryDoctor *metrics.MemoryDoctor
}

// NewMemoryHandler 创建内存监控处理器
//
// 参数：
//   - logger: 日志记录器
//   - memoryDoctor: 内存监控组件
//
// 返回：内存监控处理器实例
func NewMemoryHandler(
	logger *zap.Logger,
	memoryDoctor *metrics.MemoryDoctor,
) *MemoryHandler {
	return &MemoryHandler{
		logger:       logger,
		memoryDoctor: memoryDoctor,
	}
}

// RegisterRoutes 注册内存监控路由
//
// 注册内存监控端点：
// - GET /system/memory: 获取当前内存状态
func (h *MemoryHandler) RegisterRoutes(r *gin.RouterGroup) {
	system := r.Group("/system")
	{
		system.GET("/memory", h.GetMemory) // 获取当前内存状态
	}
}

// GetMemory 获取当前内存状态
//
// GET /api/v1/system/memory
//
// 返回当前内存状态，包括：
// - runtime: Go runtime 内存统计和进程真实物理内存（RSS）
//   - rss_mb: 进程真实物理内存（RSS，MB）- **这是真实内存占用**
//   - heap_alloc: Go runtime 堆分配（bytes）- **仅作趋势参考，非物理内存**
//   - heap_inuse: Go runtime 堆使用（bytes）- **仅作趋势参考，非物理内存**
//   - num_gc: GC 次数
//   - num_goroutine: Goroutine 数量
// - modules: 各模块的内存统计（module, layer, objects, approx_bytes, cache_items, queue_length）
//
// 响应格式：
// {
//   "runtime": {
//     "rss_mb": 512,
//     "rss_bytes": 536870912,
//     "heap_alloc": 123456789,
//     "heap_inuse": 22334455,
//     "num_gc": 42,
//     "num_goroutine": 321
//   },
//   "modules": [
//     {
//       "module": "mempool.txpool",
//       "layer": "L3-Coordination",
//       "objects": 1024,
//       "approx_bytes": 8388608,
//       "cache_items": 0,
//       "queue_length": 1024
//     }
//   ]
// }
func (h *MemoryHandler) GetMemory(c *gin.Context) {
	if h.memoryDoctor == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":     "memory doctor service is not available",
			"error_cn":  "内存监控服务不可用",
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	// 获取当前内存状态
	stats := h.memoryDoctor.GetCurrentStats()

	// 构建响应
	response := gin.H{
		"runtime": gin.H{
			"rss_mb":        stats.RSSMB,        // 真实物理内存（MB）
			"rss_bytes":     stats.RSSBytes,     // 真实物理内存（bytes）
			"heap_alloc":    stats.HeapAlloc,     // Go runtime 指标（仅作趋势参考）
			"heap_inuse":    stats.HeapInuse,    // Go runtime 指标（仅作趋势参考）
			"num_gc":        stats.NumGC,
			"num_goroutine": stats.NumGoroutine,
		},
		"modules": stats.Modules,
	}

	c.JSON(http.StatusOK, response)
}

