package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	runtimectx "github.com/weisyn/v1/internal/core/infrastructure/runtime"
)

// RuntimeModeHandler 提供节点运行模式与 UTXO 健康状态的 HTTP 接口
//
// 路径前缀：/api/v1/runtime
// - GET  /api/v1/runtime/mode       获取当前运行模式与 UTXO 健康状态
// - POST /api/v1/runtime/mode       设置运行模式（需在受控环境下使用）
type RuntimeModeHandler struct {
	logger *zap.Logger
}

// NewRuntimeModeHandler 创建 RuntimeModeHandler
func NewRuntimeModeHandler(logger *zap.Logger) *RuntimeModeHandler {
	return &RuntimeModeHandler{
		logger: logger,
	}
}

// RegisterRoutes 注册运行模式相关路由
func (h *RuntimeModeHandler) RegisterRoutes(r *gin.RouterGroup) {
	group := r.Group("/runtime")
	{
		group.GET("/mode", h.GetMode)
		group.POST("/mode", h.SetMode)
	}
}

// GetMode 获取当前节点运行模式与各类 UTXO 健康状态
//
// GET /api/v1/runtime/mode
func (h *RuntimeModeHandler) GetMode(c *gin.Context) {
	mode := runtimectx.GetNodeMode()

	resp := gin.H{
		"mode": mode.String(),
		"utxo": gin.H{
			"asset": gin.H{
				"health": runtimectx.GetUTXOHealth(runtimectx.UTXOTypeAsset),
			},
			"resource": gin.H{
				"health": runtimectx.GetUTXOHealth(runtimectx.UTXOTypeResource),
			},
		},
	}

	c.JSON(http.StatusOK, resp)
}

// SetMode 设置节点运行模式
//
// POST /api/v1/runtime/mode?value=RepairingUTXO
//
// 注意：
// - 仅建议在受控的内网/运维环境中开放此接口
// - 需要在上层通过网关/认证机制限制访问
func (h *RuntimeModeHandler) SetMode(c *gin.Context) {
	value := c.Query("value")
	if value == "" {
		value = c.PostForm("value")
	}

	if value == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":    "missing mode value",
			"error_cn": "缺少 mode 参数（例如 Normal/Degraded/RepairingUTXO/ReadOnly）",
		})
		return
	}

	var mode runtimectx.NodeMode
	switch value {
	case "Normal":
		mode = runtimectx.NodeModeNormal
	case "Degraded":
		mode = runtimectx.NodeModeDegraded
	case "RepairingUTXO":
		mode = runtimectx.NodeModeRepairingUTXO
	case "ReadOnly":
		mode = runtimectx.NodeModeReadOnly
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error":    "invalid mode value",
			"error_cn": "非法的 mode 值，仅支持 Normal/Degraded/RepairingUTXO/ReadOnly",
		})
		return
	}

	runtimectx.SetNodeMode(mode)

	if h.logger != nil {
		h.logger.Warn("runtime mode changed via HTTP API",
			zap.String("mode", mode.String()),
			zap.String("remote_addr", c.Request.RemoteAddr))
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"mode":     mode.String(),
		"message":  "runtime mode updated",
		"message_cn": "运行模式已更新",
	})
}


