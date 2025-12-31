package methods

import (
	"context"
	"encoding/json"
	"time"

	libhost "github.com/libp2p/go-libp2p/core/host"
	libpeer "github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
	p2piface "github.com/weisyn/v1/pkg/interfaces/p2p"
	"go.uber.org/zap"
)

// AdminP2PMethods 管理面 P2P 相关 JSON-RPC 方法
//
// ⚠️ 注意：仅用于节点控制面 / 运维面，不面向普通 DApp 暴露。
// 底层仍通过 p2p.Service 走完整的身份与安全校验，不绕过任何链级访问控制。
type AdminP2PMethods struct {
	logger *zap.Logger
	p2p    p2piface.Service
}

// NewAdminP2PMethods 创建管理面 P2P 方法处理器
//
// 参数：
//   - logger: API 专用 logger（可为 nil）
//   - p2pSvc: P2P 运行时服务（必须通过 name:"p2p_service" 注入）
func NewAdminP2PMethods(
	logger *zap.Logger,
	p2pSvc p2piface.Service,
) *AdminP2PMethods {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &AdminP2PMethods{
		logger: logger.With(zap.String("rpc", "admin_p2p")),
		p2p:    p2pSvc,
	}
}

// connectPeerOptions 管理面 ConnectPeer 选项
type connectPeerOptions struct {
	Multiaddrs []string `json:"multiaddrs"`
	TimeoutMs  int      `json:"timeoutMs"`
}

// AdminConnectPeerResult 管理面连接结果
type AdminConnectPeerResult struct {
	Connected  bool     `json:"connected"`
	PeerID     string   `json:"peerId"`
	TriedAddrs []string `json:"triedAddrs"`
	Error      string   `json:"error,omitempty"`
}

// AdminP2PStatusResult 管理面 P2P 运行时状态（用于排障）
type AdminP2PStatusResult struct {
	PeerID       string                 `json:"peerId"`
	ListenAddrs  []string               `json:"listenAddrs"`
	Connected    []libpeer.AddrInfo     `json:"connectedPeers"`
	Connections  []p2piface.ConnInfo    `json:"connections"`
	SwarmStats   p2piface.SwarmStats    `json:"swarmStats"`
	RoutingMode  string                 `json:"routingMode"`
	Reachability string                 `json:"reachability"`
	Profile      string                 `json:"profile"`
	Diagnostics  map[string]interface{} `json:"diagnostics"`
}

// GetP2PStatus 返回 P2P 运行时状态（JSON-RPC）
//
// Method: wes_admin_getP2PStatus
//
// 用途：
// - 生产排障：确认 peerId、监听地址、连接数、可达性、DHT 模式等关键事实；
// - 避免“看日志猜”。
func (m *AdminP2PMethods) GetP2PStatus(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if m.p2p == nil {
		return nil, NewInternalError("p2p service not available", nil)
	}

	var h libhost.Host = m.p2p.Host()
	if h == nil {
		return nil, NewInternalError("p2p host not available", nil)
	}

	listenAddrs := make([]string, 0)
	for _, a := range h.Addrs() {
		listenAddrs = append(listenAddrs, a.String())
	}

	var routingMode string
	if r := m.p2p.Routing(); r != nil {
		routingMode = string(r.Mode())
	}

	var reachability string
	var profile string
	if c := m.p2p.Connectivity(); c != nil {
		reachability = string(c.Reachability())
		profile = string(c.Profile())
	}

	diag := map[string]interface{}{}
	if d := m.p2p.Diagnostics(); d != nil {
		diag["httpAddr"] = d.HTTPAddr()
		diag["peersCount"] = d.GetPeersCount()
		diag["connectionsCount"] = d.GetConnectionsCount()
	}

	var swarmStats p2piface.SwarmStats
	var conns []p2piface.ConnInfo
	var peers []libpeer.AddrInfo
	if s := m.p2p.Swarm(); s != nil {
		swarmStats = s.Stats()
		conns = s.Connections()
		peers = s.Peers()
	}

	return &AdminP2PStatusResult{
		PeerID:       h.ID().String(),
		ListenAddrs:  listenAddrs,
		Connected:    peers,
		Connections:  conns,
		SwarmStats:   swarmStats,
		RoutingMode:  routingMode,
		Reachability: reachability,
		Profile:      profile,
		Diagnostics:  diag,
	}, nil
}

// ConnectPeer 主动连接指定 peer
//
// Method: wes_admin_connectPeer
//
// 参数：
//   - [0]: peerId (string, 必填) - libp2p PeerID（例如 "12D3KooW..."）
//   - [1]: options (object, 可选)：
//   - multiaddrs: []string - 指定要尝试的地址列表（multiaddr 字符串）
//   - timeoutMs: int       - 拨号超时时间（毫秒），默认 10000ms
//
// 行为：
//   - 如提供 multiaddrs，则优先使用该地址集进行拨号；
//   - 否则通过 DHT Routing.FindPeer 查找地址；
//   - 底层通过 p2p.Swarm().Dial() 发起连接；
//   - 不保证连接一定成功，结果通过返回值中的 connected/error 体现。
func (m *AdminP2PMethods) ConnectPeer(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var args []json.RawMessage
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, NewInvalidParamsError("invalid params format", map[string]interface{}{
			"error": err.Error(),
		})
	}
	if len(args) == 0 {
		return nil, NewInvalidParamsError("peerId is required", nil)
	}

	var peerIDStr string
	if err := json.Unmarshal(args[0], &peerIDStr); err != nil {
		return nil, NewInvalidParamsError("peerId must be string", map[string]interface{}{
			"error": err.Error(),
		})
	}

	var opts connectPeerOptions
	if len(args) > 1 {
		if err := json.Unmarshal(args[1], &opts); err != nil {
			return nil, NewInvalidParamsError("invalid options", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}
	if opts.TimeoutMs <= 0 {
		opts.TimeoutMs = 10000
	}

	if m.p2p == nil {
		return nil, NewInternalError("p2p service not available", nil)
	}

	pid, err := libpeer.Decode(peerIDStr)
	if err != nil {
		return nil, NewInvalidParamsError("invalid peerId", map[string]interface{}{
			"peerId": peerIDStr,
			"error":  err.Error(),
		})
	}

	// 准备地址信息
	var info libpeer.AddrInfo
	triedAddrs := make([]string, 0)

	if len(opts.Multiaddrs) > 0 {
		for _, s := range opts.Multiaddrs {
			maddr, err := ma.NewMultiaddr(s)
			if err != nil {
				return nil, NewInvalidParamsError("invalid multiaddr", map[string]interface{}{
					"multiaddr": s,
					"error":     err.Error(),
				})
			}
			info.Addrs = append(info.Addrs, maddr)
			triedAddrs = append(triedAddrs, s)
		}
		info.ID = pid
	} else {
		// 通过 DHT 路由查找 peer 地址
		routing := m.p2p.Routing()
		if routing == nil {
			return nil, NewInternalError("p2p routing not available", nil)
		}
		ai, err := routing.FindPeer(ctx, pid)
		if err != nil {
			m.logger.Warn("admin_connect_peer_findpeer_failed",
				zap.String("peer", peerIDStr),
				zap.Error(err),
			)
			return &AdminConnectPeerResult{
				Connected:  false,
				PeerID:     peerIDStr,
				TriedAddrs: triedAddrs,
				Error:      err.Error(),
			}, nil
		}
		info = ai
		for _, a := range ai.Addrs {
			triedAddrs = append(triedAddrs, a.String())
		}
	}

	// 拨号
	cctx, cancel := context.WithTimeout(ctx, time.Duration(opts.TimeoutMs)*time.Millisecond)
	defer cancel()

	m.logger.Info("admin_connect_peer_start",
		zap.String("peer", peerIDStr),
		zap.Strings("addrs", triedAddrs),
	)

	if err := m.p2p.Swarm().Dial(cctx, info); err != nil {
		m.logger.Warn("admin_connect_peer_failed",
			zap.String("peer", peerIDStr),
			zap.Strings("addrs", triedAddrs),
			zap.Error(err),
		)
		return &AdminConnectPeerResult{
			Connected:  false,
			PeerID:     peerIDStr,
			TriedAddrs: triedAddrs,
			Error:      err.Error(),
		}, nil
	}

	m.logger.Info("admin_connect_peer_success",
		zap.String("peer", peerIDStr),
		zap.Strings("addrs", triedAddrs),
	)

	return &AdminConnectPeerResult{
		Connected:  true,
		PeerID:     peerIDStr,
		TriedAddrs: triedAddrs,
	}, nil
}
