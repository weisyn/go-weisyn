package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	p2piface "github.com/weisyn/v1/pkg/interfaces/p2p"
	"go.uber.org/zap"
)

// NodeStatusHandler 节点状态处理器
//
// 🎯 **节点运行时状态 API**
//
// 提供节点运行时状态查询和控制端点：
// - GET /node/status: 查询节点状态
// - POST /node/sync_mode: 设置同步模式
// - POST /node/mining: 设置挖矿开关
type NodeStatusHandler struct {
	logger           *zap.Logger
	nodeRuntimeState p2piface.RuntimeState
}

// NewNodeStatusHandler 创建节点状态处理器
func NewNodeStatusHandler(
	logger *zap.Logger,
	nodeRuntimeState p2piface.RuntimeState,
) *NodeStatusHandler {
	return &NodeStatusHandler{
		logger:           logger,
		nodeRuntimeState: nodeRuntimeState,
	}
}

// RegisterRoutes 注册节点状态路由
func (h *NodeStatusHandler) RegisterRoutes(r *gin.RouterGroup) {
	node := r.Group("/node")
	{
		node.GET("/status", h.GetNodeStatus)     // 查询节点状态
		node.POST("/sync_mode", h.SetSyncMode)   // 设置同步模式
		node.POST("/mining", h.SetMiningEnabled) // 设置挖矿开关
	}
}

// GetNodeStatus 获取节点状态
//
// GET /api/v1/node/status
//
// 返回节点运行时状态快照，包括：
// - sync_mode: 同步模式（full/light/archive/pruned）
// - sync_status: 同步状态（syncing/synced/lagging/error）
// - is_fully_synced: 是否已完全同步
// - is_online: 是否在线
// - mining_enabled: 是否开启挖矿
// - is_consensus_eligible: 是否具备共识资格
// - is_voter_in_round: 当前轮次是否参与投票
// - is_proposer_candidate: 当前轮次是否可作为出块候选者
func (h *NodeStatusHandler) GetNodeStatus(c *gin.Context) {
	snapshot := h.nodeRuntimeState.GetSnapshot()

	c.JSON(http.StatusOK, gin.H{
		"sync_mode":             string(snapshot.SyncMode),
		"sync_status":           string(snapshot.SyncStatus),
		"is_fully_synced":       snapshot.IsFullySynced,
		"is_online":             snapshot.IsOnline,
		"mining_enabled":        snapshot.MiningEnabled,
		"is_consensus_eligible": snapshot.IsConsensusEligible,
		"is_voter_in_round":     snapshot.IsVoterInRound,
		"is_proposer_candidate": snapshot.IsProposerCandidate,
	})
}

// SetSyncModeRequest 设置同步模式请求
type SetSyncModeRequest struct {
	Mode string `json:"mode" binding:"required"` // 同步模式：full | light | archive | pruned
}

// SetSyncMode 设置同步模式
//
// POST /api/v1/node/sync_mode
//
// 请求体：
//
//	{
//	  "mode": "full" | "light" | "archive" | "pruned"
//	}
//
// 行为：
// - 更新 sync.mode
// - 检查不变式 I6（同步模式切换约束）
// - 如果从 full → light，自动停止挖矿
func (h *NodeStatusHandler) SetSyncMode(c *gin.Context) {
	var req SetSyncModeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证同步模式
	mode := p2piface.SyncMode(req.Mode)
	switch mode {
	case p2piface.SyncModeFull, p2piface.SyncModeLight, p2piface.SyncModeArchive, p2piface.SyncModePruned:
		// 有效模式
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid sync mode, must be one of: full, light, archive, pruned",
		})
		return
	}

	// 更新同步模式
	ctx := c.Request.Context()
	if err := h.nodeRuntimeState.SetSyncMode(ctx, mode); err != nil {
		h.logger.Error("failed to set sync mode", zap.Error(err), zap.String("mode", string(mode)))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "sync mode updated successfully",
		"mode":    string(mode),
	})
}

// SetMiningEnabledRequest 设置挖矿开关请求
type SetMiningEnabledRequest struct {
	Enabled bool `json:"enabled" binding:"required"` // 是否开启挖矿
}

// SetMiningEnabled 设置挖矿开关
//
// POST /api/v1/node/mining
//
// 请求体：
//
//	{
//	  "enabled": true | false
//	}
//
// 行为：
// - 检查不变式 I4（挖矿前置条件）
// - 更新 mining.enabled
// - 记录日志
func (h *NodeStatusHandler) SetMiningEnabled(c *gin.Context) {
	var req SetMiningEnabledRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// V2 约束：开启挖矿必须走 miner.StartMining（需要矿工地址 + 门闸检查）。
	// /node/mining 作为“状态开关”接口不具备这些必要输入，因此只允许关闭。
	if req.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "V2 挖矿开启被拒绝：请通过 JSON-RPC `wes_startMining` 提供矿工地址启动挖矿（包含门闸检查）；该接口仅支持关闭挖矿",
			"enabled": false,
		})
		return
	}

	// 更新挖矿开关
	ctx := c.Request.Context()
	if err := h.nodeRuntimeState.SetMiningEnabled(ctx, req.Enabled); err != nil {
		h.logger.Error("failed to set mining enabled", zap.Error(err), zap.Bool("enabled", req.Enabled))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp := gin.H{
		"message": "mining status updated successfully",
		"enabled": req.Enabled,
	}

	c.JSON(http.StatusOK, resp)
}
