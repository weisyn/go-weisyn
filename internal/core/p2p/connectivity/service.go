package connectivity

import (
	"context"
	"sync"

	lphost "github.com/libp2p/go-libp2p/core/host"
	libnetwork "github.com/libp2p/go-libp2p/core/network"
	autonat "github.com/libp2p/go-libp2p/p2p/host/autonat"
	ma "github.com/multiformats/go-multiaddr"

	p2pcfg "github.com/weisyn/v1/internal/config/p2p"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// Service Connectivity 服务实现
//
// 管理 NAT / AutoNAT / Relay / DCUTR 等连通性增强能力
type Service struct {
	host         lphost.Host
	reachability p2pi.Reachability
	profile      p2pi.Profile
	opts         *p2pcfg.Options
	logger       logiface.Logger
	manager      *Manager // 高级连通性管理器
	mu           sync.RWMutex
}

var _ p2pi.Connectivity = (*Service)(nil)

// NewService 创建 Connectivity 服务
func NewService(profile p2pi.Profile) *Service {
	return &Service{
		reachability: p2pi.ReachabilityUnknown,
		profile:      profile,
	}
}

// Initialize 初始化 Connectivity 服务
func (s *Service) Initialize(host lphost.Host, opts *p2pcfg.Options, logger logiface.Logger) {
	s.host = host
	s.opts = opts
	s.logger = logger

	// 根据 Profile 设置初始可达性
	switch s.profile {
	case p2pi.ProfileServer:
		s.reachability = p2pi.ReachabilityPublic
	case p2pi.ProfileClient:
		s.reachability = p2pi.ReachabilityPublic
	case p2pi.ProfileLAN:
		s.reachability = p2pi.ReachabilityPrivate
	default:
		s.reachability = p2pi.ReachabilityUnknown
	}

	// 创建高级连通性管理器（如果启用 Relay/DCUTR）
	if opts != nil && (opts.EnableRelay || opts.EnableRelayService || opts.EnableDCUTR) {
		s.manager = NewManager(host, opts, logger)
	}

	// 监听网络事件以更新可达性
	if host != nil {
		host.Network().Notify(&connectivityNotifiee{
			service: s,
		})
	}
}

// SetConnectionProtector 设置 ConnectionProtector（由 Runtime 调用）
func (s *Service) SetConnectionProtector(protector interface{ GetStats() map[string]interface{} }) {
	if s.manager != nil {
		s.manager.SetConnectionProtector(protector)
	}
}

// SetAutoNATClient 设置 AutoNAT 客户端实例（由 Runtime 调用）
func (s *Service) SetAutoNATClient(client autonat.AutoNAT) {
	if s.manager != nil && client != nil {
		s.manager.SetAutoNATClient(client)
	}
}

// Start 启动连通性管理器（由 Runtime 调用）
func (s *Service) Start(ctx context.Context) error {
	if s.manager != nil {
		return s.manager.Start(ctx)
	}
	return nil
}

// Stop 停止连通性管理器（由 Runtime 调用）
func (s *Service) Stop() error {
	if s.manager != nil {
		return s.manager.Stop()
	}
	return nil
}

// Stats 获取连通性统计信息（内部接口）
func (s *Service) Stats() ConnectivityStats {
	if s.manager != nil {
		return s.manager.Stats()
	}
	return ConnectivityStats{
		AutoNATStatus: "unknown",
	}
}

// StatsMap 获取连通性统计信息（Map 格式，供 Diagnostics 使用）
func (s *Service) StatsMap() map[string]interface{} {
	if s.manager != nil {
		return s.manager.StatsMap()
	}
	stats := s.Stats()
	return map[string]interface{}{
		"relay_enabled":     stats.RelayEnabled,
		"relay_active":      stats.RelayActive,
		"holepunch_enabled": stats.HolePunchEnabled,
		"autorelay_enabled": stats.AutoRelayEnabled,
		"relay_client":      stats.RelayClient,
		"num_relays":        stats.NumRelays,
		"active_relays":     stats.ActiveRelays,
		"autoNAT_status":    stats.AutoNATStatus,
	}
}

// Reachability 返回当前可达性状态
func (s *Service) Reachability() p2pi.Reachability {
	// 1. 优先尝试从 AutoNAT 客户端获取真实可达性状态
	if s.manager != nil {
		if r, ok := s.manager.GetAutoNATReachability(); ok && r != p2pi.ReachabilityUnknown {
			return r
		}
	}

	// 2. 回退到基于 Profile 的静态可达性推断
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.reachability
}

// Profile 返回当前 P2P Profile
func (s *Service) Profile() p2pi.Profile {
	return s.profile
}

// updateReachability 更新可达性状态
func (s *Service) updateReachability(reachability p2pi.Reachability) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reachability = reachability
}

// connectivityNotifiee 网络事件通知器
type connectivityNotifiee struct {
	service *Service
}

func (n *connectivityNotifiee) Listen(libnetwork.Network, ma.Multiaddr)      {}
func (n *connectivityNotifiee) ListenClose(libnetwork.Network, ma.Multiaddr) {}
func (n *connectivityNotifiee) Connected(libnetwork.Network, libnetwork.Conn)   {}
func (n *connectivityNotifiee) Disconnected(libnetwork.Network, libnetwork.Conn) {}
func (n *connectivityNotifiee) OpenedStream(libnetwork.Network, libnetwork.Stream) {}
func (n *connectivityNotifiee) ClosedStream(libnetwork.Network, libnetwork.Stream) {}

