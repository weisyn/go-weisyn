package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/weisyn/v1/internal/core/consensus/miner/quorum"
)

// MiningHandler 挖矿相关调试端点处理器
//
// 🎯 **挖矿调试接口**
//
// 提供挖矿门闸/网络法定人数状态查询端点：
// - GET /debug/mining/quorum: 获取挖矿门闸状态（网络法定人数 + 高度一致性 + 链尖前置）
//
// 实现细节：
// - 调用 quorum.Checker.Check() 获取完整门闸状态
// - 返回人类可读的 JSON 响应（便于运维诊断）
type MiningHandler struct {
	logger        *zap.Logger
	quorumChecker quorum.Checker
}

// NewMiningHandler 创建挖矿调试处理器
//
// 参数：
//   - logger: 日志记录器
//   - quorumChecker: 挖矿门闸检查器（可选，如果为 nil 则端点返回错误）
//
// 返回：挖矿调试处理器实例
func NewMiningHandler(
	logger *zap.Logger,
	quorumChecker quorum.Checker,
) *MiningHandler {
	return &MiningHandler{
		logger:        logger,
		quorumChecker: quorumChecker,
	}
}

// RegisterRoutes 注册挖矿调试路由
//
// 注册端点：
// - GET /debug/mining/quorum: 获取挖矿门闸状态
func (h *MiningHandler) RegisterRoutes(r *gin.RouterGroup) {
	debug := r.Group("/debug")
	{
		mining := debug.Group("/mining")
		{
			mining.GET("/quorum", h.GetQuorumStatus) // 获取挖矿门闸状态
		}
	}
}

// GetQuorumStatus 获取挖矿门闸/网络法定人数状态
//
// GET /debug/mining/quorum
//
// 返回挖矿门闸的完整状态，包括：
// - allow_mining: 是否允许挖矿
// - state: 网络法定人数状态（NotStarted, Discovering, QuorumPending, QuorumReached, HeightAligned, HeightConflict, Isolated）
// - reason: 决策原因
// - suggested_action: 建议动作
// - metrics: 网络指标（peers、高度、法定人数等）
// - chain_tip: 链尖前置条件（可读性、新鲜度等）
//
// 响应格式：
//
//	{
//	  "allow_mining": false,
//	  "state": "QuorumPending",
//	  "reason": "网络法定人数不足（当前=1 需要=2），等待更多节点加入/完成握手",
//	  "suggested_action": "wait",
//	  "metrics": {
//	    "discovered_peers": 2,
//	    "connected_peers": 1,
//	    "qualified_peers": 1,
//	    "required_quorum_total": 2,
//	    "current_quorum_total": 2,
//	    "quorum_reached": true,
//	    "local_height": 100,
//	    "median_peer_height": 100,
//	    "height_skew": 0,
//	    "peer_heights": {
//	      "12D3KooW...": 100
//	    },
//	    "discovery_started_at": 1704067200,
//	    "quorum_reached_at": 1704067210
//	  },
//	  "chain_tip": {
//	    "tip_readable": true,
//	    "tip_timestamp": 1704067200,
//	    "tip_age_seconds": 60,
//	    "tip_fresh": true,
//	    "ready_for_network_handshake": true
//	  }
//	}
func (h *MiningHandler) GetQuorumStatus(c *gin.Context) {
	if h.quorumChecker == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "mining quorum checker not available",
		})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	res, err := h.quorumChecker.Check(ctx)
	if err != nil {
		h.logger.Error("failed to check mining quorum status", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to check mining quorum status",
			"details": err.Error(),
		})
		return
	}

	if res == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "mining quorum status is nil",
		})
		return
	}

	// 转换 peer.ID -> string
	peerHeights := make(map[string]uint64)
	for pid, h := range res.Metrics.PeerHeights {
		peerHeights[pid.String()] = h
	}

	// 转换时间戳
	discoveryStartedAt := int64(0)
	if !res.Metrics.DiscoveryStartedAt.IsZero() {
		discoveryStartedAt = res.Metrics.DiscoveryStartedAt.Unix()
	}
	quorumReachedAt := int64(0)
	if !res.Metrics.QuorumReachedAt.IsZero() {
		quorumReachedAt = res.Metrics.QuorumReachedAt.Unix()
	}

	c.JSON(http.StatusOK, gin.H{
		"allow_mining":     res.AllowMining,
		"state":            string(res.State),
		"reason":           res.Reason,
		"suggested_action": res.SuggestedAction,
		"metrics": gin.H{
			"discovered_peers":      res.Metrics.DiscoveredPeers,
			"connected_peers":       res.Metrics.ConnectedPeers,
			"qualified_peers":       res.Metrics.QualifiedPeers,
			"required_quorum_total": res.Metrics.RequiredQuorumTotal,
			"current_quorum_total":  res.Metrics.CurrentQuorumTotal,
			"quorum_reached":        res.Metrics.QuorumReached,
			"local_height":          res.Metrics.LocalHeight,
			"median_peer_height":    res.Metrics.MedianPeerHeight,
			"height_skew":           res.Metrics.HeightSkew,
			"peer_heights":          peerHeights,
			"discovery_started_at":  discoveryStartedAt,
			"quorum_reached_at":     quorumReachedAt,
		},
		"chain_tip": gin.H{
			"tip_readable":              res.ChainTip.TipReadable,
			"tip_timestamp":             res.ChainTip.TipTimestamp,
			"tip_age_seconds":           int64(res.ChainTip.TipAge / time.Second),
			"tip_fresh":                 res.ChainTip.TipFresh,
			"tip_healthy_for_handshake": res.ChainTip.TipHealthyForHandshake,
		},
	})
}
