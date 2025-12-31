package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	peer "github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/weisyn/v1/internal/api/http/middleware"
	apitypes "github.com/weisyn/v1/internal/api/types"
	"github.com/weisyn/v1/pkg/interfaces/p2p"
	"go.uber.org/zap"
)

// AdminP2PHandler 管理面 P2P 相关 REST 端点
//
// ⚠️ 仅建议用于节点控制面 / 运维面，不面向普通 DApp 暴露。
// 行为与 JSON-RPC 管理方法 wes_admin_connectPeer 保持一致。
type AdminP2PHandler struct {
	logger *zap.Logger
	p2p    p2p.Service
}

// NewAdminP2PHandler 创建管理面 P2P 处理器
func NewAdminP2PHandler(logger *zap.Logger, p2pSvc p2p.Service) *AdminP2PHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &AdminP2PHandler{
		logger: logger.With(zap.String("handler", "admin_p2p")),
		p2p:    p2pSvc,
	}
}

// adminConnectPeerRequest REST 管理端连接请求体
type adminConnectPeerRequest struct {
	PeerID     string   `json:"peerId"`
	Multiaddrs []string `json:"multiaddrs"`
	TimeoutMs  int      `json:"timeoutMs"`
}

// adminConnectPeerResponse 管理端连接响应体
type adminConnectPeerResponse struct {
	Connected  bool     `json:"connected"`
	PeerID     string   `json:"peerId"`
	TriedAddrs []string `json:"triedAddrs"`
	Error      string   `json:"error,omitempty"`
}

// adminP2PStatusResponse 管理面 P2P 状态响应体
type adminP2PStatusResponse struct {
	PeerID       string                 `json:"peerId"`
	ListenAddrs  []string               `json:"listenAddrs"`
	Connected    []peer.AddrInfo        `json:"connectedPeers"`
	Connections  []p2p.ConnInfo         `json:"connections"`
	SwarmStats   p2p.SwarmStats         `json:"swarmStats"`
	RoutingMode  string                 `json:"routingMode"`
	Reachability string                 `json:"reachability"`
	Profile      string                 `json:"profile"`
	Diagnostics  map[string]interface{} `json:"diagnostics"`
}

// RegisterRoutes 注册管理面 P2P 路由
//
// 路径前缀：/api/v1/admin/p2p
//
// 端点：
//   - POST /api/v1/admin/p2p/connect
//   - GET  /api/v1/admin/p2p/status
func (h *AdminP2PHandler) RegisterRoutes(r *gin.RouterGroup) {
	admin := r.Group("/admin")
	p2pGroup := admin.Group("/p2p")
	{
		p2pGroup.POST("/connect", h.Connect)
		p2pGroup.GET("/status", h.Status)
	}
}

// Status 返回 P2P 运行时状态（REST）
//
// GET /api/v1/admin/p2p/status
func (h *AdminP2PHandler) Status(c *gin.Context) {
	if h.p2p == nil || h.p2p.Host() == nil {
		middleware.WriteProblemDetails(c, apitypes.NewProblemDetails(
			apitypes.CodeCommonServiceUnavailable,
			apitypes.LayerBlockchainService,
			"P2P 服务不可用。",
			"p2p service is not available",
			http.StatusServiceUnavailable,
			nil,
		))
		return
	}

	host := h.p2p.Host()
	listenAddrs := make([]string, 0)
	for _, a := range host.Addrs() {
		listenAddrs = append(listenAddrs, a.String())
	}

	var routingMode string
	if r := h.p2p.Routing(); r != nil {
		routingMode = string(r.Mode())
	}

	var reachability string
	var profile string
	if conn := h.p2p.Connectivity(); conn != nil {
		reachability = string(conn.Reachability())
		profile = string(conn.Profile())
	}

	diag := map[string]interface{}{}
	if d := h.p2p.Diagnostics(); d != nil {
		diag["httpAddr"] = d.HTTPAddr()
		diag["peersCount"] = d.GetPeersCount()
		diag["connectionsCount"] = d.GetConnectionsCount()
	}

	var swarmStats p2p.SwarmStats
	var conns []p2p.ConnInfo
	var peers []peer.AddrInfo
	if s := h.p2p.Swarm(); s != nil {
		swarmStats = s.Stats()
		conns = s.Connections()
		peers = s.Peers()
	}

	c.JSON(http.StatusOK, adminP2PStatusResponse{
		PeerID:       host.ID().String(),
		ListenAddrs:  listenAddrs,
		Connected:    peers,
		Connections:  conns,
		SwarmStats:   swarmStats,
		RoutingMode:  routingMode,
		Reachability: reachability,
		Profile:      profile,
		Diagnostics:  diag,
	})
}

// Connect 主动连接指定 peer（REST）
//
// POST /api/v1/admin/p2p/connect
//
// 请求体：
//
//	{
//	  "peerId": "12D3KooW...",
//	  "multiaddrs": ["/ip4/101.37.245.124/tcp/28683"],
//	  "timeoutMs": 10000
//	}
//
// 返回：
//
//	{
//	  "connected": true,
//	  "peerId": "12D3KooW...",
//	  "triedAddrs": [...],
//	  "error": null
//	}
func (h *AdminP2PHandler) Connect(c *gin.Context) {
	if h.p2p == nil {
		middleware.WriteProblemDetails(c, apitypes.NewProblemDetails(
			apitypes.CodeCommonServiceUnavailable,
			apitypes.LayerBlockchainService,
			"P2P 服务不可用。",
			"p2p service is not available",
			http.StatusServiceUnavailable,
			nil,
		))
		return
	}

	var req adminConnectPeerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteProblemDetails(c, apitypes.NewProblemDetails(
			apitypes.CodeCommonValidationError,
			apitypes.LayerBlockchainService,
			"请求参数解析失败，请检查输入参数。",
			err.Error(),
			http.StatusBadRequest,
			nil,
		))
		return
	}

	if req.PeerID == "" {
		middleware.WriteProblemDetails(c, apitypes.NewProblemDetails(
			apitypes.CodeCommonValidationError,
			apitypes.LayerBlockchainService,
			"peerId 参数必填。",
			"peerId is required",
			http.StatusBadRequest,
			nil,
		))
		return
	}

	if req.TimeoutMs <= 0 {
		req.TimeoutMs = 10000
	}

	pid, err := peer.Decode(req.PeerID)
	if err != nil {
		middleware.WriteProblemDetails(c, apitypes.NewProblemDetails(
			apitypes.CodeCommonValidationError,
			apitypes.LayerBlockchainService,
			"无效的 peerId 格式。",
			err.Error(),
			http.StatusBadRequest,
			map[string]interface{}{
				"peerId": req.PeerID,
			},
		))
		return
	}

	var info peer.AddrInfo
	triedAddrs := make([]string, 0)

	if len(req.Multiaddrs) > 0 {
		for _, s := range req.Multiaddrs {
			maddr, err := ma.NewMultiaddr(s)
			if err != nil {
				middleware.WriteProblemDetails(c, apitypes.NewProblemDetails(
					apitypes.CodeCommonValidationError,
					apitypes.LayerBlockchainService,
					"无效的 multiaddr。",
					err.Error(),
					http.StatusBadRequest,
					map[string]interface{}{
						"multiaddr": s,
					},
				))
				return
			}
			info.Addrs = append(info.Addrs, maddr)
			triedAddrs = append(triedAddrs, s)
		}
		info.ID = pid
	} else {
		routing := h.p2p.Routing()
		if routing == nil {
			middleware.WriteProblemDetails(c, apitypes.NewProblemDetails(
				apitypes.CodeCommonServiceUnavailable,
				apitypes.LayerBlockchainService,
				"P2P 路由服务不可用。",
				"p2p routing is not available",
				http.StatusServiceUnavailable,
				nil,
			))
			return
		}
		ai, err := routing.FindPeer(c.Request.Context(), pid)
		if err != nil {
			h.logger.Warn("admin_connect_peer_findpeer_failed",
				zap.String("peer", req.PeerID),
				zap.Error(err),
			)
			c.JSON(http.StatusOK, adminConnectPeerResponse{
				Connected:  false,
				PeerID:     req.PeerID,
				TriedAddrs: triedAddrs,
				Error:      err.Error(),
			})
			return
		}
		info = ai
		for _, a := range ai.Addrs {
			triedAddrs = append(triedAddrs, a.String())
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(req.TimeoutMs)*time.Millisecond)
	defer cancel()

	h.logger.Info("admin_connect_peer_start",
		zap.String("peer", req.PeerID),
		zap.Strings("addrs", triedAddrs),
	)

	if err := h.p2p.Swarm().Dial(ctx, info); err != nil {
		h.logger.Warn("admin_connect_peer_failed",
			zap.String("peer", req.PeerID),
			zap.Strings("addrs", triedAddrs),
			zap.Error(err),
		)
		c.JSON(http.StatusOK, adminConnectPeerResponse{
			Connected:  false,
			PeerID:     req.PeerID,
			TriedAddrs: triedAddrs,
			Error:      err.Error(),
		})
		return
	}

	h.logger.Info("admin_connect_peer_success",
		zap.String("peer", req.PeerID),
		zap.Strings("addrs", triedAddrs),
	)

	c.JSON(http.StatusOK, adminConnectPeerResponse{
		Connected:  true,
		PeerID:     req.PeerID,
		TriedAddrs: triedAddrs,
	})
}
