package connectivity

import (
	"context"
	"sync"

	lphost "github.com/libp2p/go-libp2p/core/host"
	libnetwork "github.com/libp2p/go-libp2p/core/network"
	autonat "github.com/libp2p/go-libp2p/p2p/host/autonat"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"

	p2pcfg "github.com/weisyn/v1/internal/config/p2p"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
)

// ConnectivityStats 连通性统计信息
type ConnectivityStats struct {
	RelayEnabled     bool
	RelayActive      bool
	HolePunchEnabled bool
	AutoRelayEnabled bool
	RelayClient      bool
	NumRelays        int
	ActiveRelays     int
	AutoNATStatus    string // "public" | "private" | "unknown"
}

// Manager 高级连通性管理器
//
// 直接使用 p2pcfg.Options，提供 Relay、DCUTR、AutoNAT 等高级能力
type Manager struct {
	host                lphost.Host
	opts                *p2pcfg.Options
	logger              logiface.Logger
	relayService        *relayv2.Relay
	relayClientEnabled  bool
	holepunchEnabled    bool
	autorelayEnabled    bool
	connectionProtector interface{ GetStats() map[string]interface{} } // ConnectionProtector 接口
	autonatClient       autonat.AutoNAT                                  // AutoNAT 客户端实例（用于获取真实状态）
	mu                  sync.RWMutex
}

// NewManager 创建连通性管理器（直接使用 p2pcfg.Options）
func NewManager(host lphost.Host, opts *p2pcfg.Options, logger logiface.Logger) *Manager {
	m := &Manager{
		host:   host,
		opts:   opts,
		logger: logger,
	}

	if opts != nil {
		m.relayClientEnabled = opts.EnableRelay
		m.holepunchEnabled = opts.EnableDCUTR
		m.autorelayEnabled = opts.EnableAutoRelay
	}

	return m
}

// SetConnectionProtector 设置 ConnectionProtector（由 Runtime 调用）
func (m *Manager) SetConnectionProtector(protector interface{ GetStats() map[string]interface{} }) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connectionProtector = protector
}

// SetAutoNATClient 设置 AutoNAT 客户端实例（由 Runtime 调用）
func (m *Manager) SetAutoNATClient(client autonat.AutoNAT) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.autonatClient = client
}

// GetAutoNATReachability 返回 AutoNAT 客户端感知到的可达性状态（如可用）
func (m *Manager) GetAutoNATReachability() (p2pi.Reachability, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.autonatClient == nil {
		return p2pi.ReachabilityUnknown, false
	}

	status := m.autonatClient.Status()
	switch status {
	case libnetwork.ReachabilityPublic:
		return p2pi.ReachabilityPublic, true
	case libnetwork.ReachabilityPrivate:
		return p2pi.ReachabilityPrivate, true
	case libnetwork.ReachabilityUnknown:
		return p2pi.ReachabilityUnknown, true
	default:
		return p2pi.ReachabilityUnknown, true
	}
}

// Start 启动连通性管理器
func (m *Manager) Start(ctx context.Context) error {
	if m.logger != nil {
		m.logger.Infof("p2p.connectivity start relay_client=%t dcutr=%t autorelay=%t", m.relayClientEnabled, m.holepunchEnabled, m.autorelayEnabled)
	}

	// 启动 Circuit Relay v2 服务（服务端，仅当启用时）
	if m.opts != nil && m.opts.EnableRelayService {
		if err := m.startRelayService(ctx); err != nil {
			if m.logger != nil {
				m.logger.Warnf("p2p.connectivity relay_service_start_failed error=%v", err)
			}
			return err
		}
	}

	// DCUtR (Hole Punching) 和 AutoRelay 服务通过 Host 选项自动启用
	// 这里不需要手动创建服务实例

	if m.logger != nil {
		m.logger.Infof("p2p.connectivity started")
	}
	return nil
}

// Stop 停止连通性管理器
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 停止中继服务
	if m.relayService != nil {
		if err := m.relayService.Close(); err != nil {
			if m.logger != nil {
				m.logger.Warnf("p2p.connectivity relay_service_stop_failed error=%v", err)
			}
			return err
		}
		m.relayService = nil
	}

	if m.logger != nil {
		m.logger.Infof("p2p.connectivity stopped")
	}

	return nil
}

// startRelayService 启动 Circuit Relay v2 服务
func (m *Manager) startRelayService(ctx context.Context) error {
	// 构建资源配置
	res := relayv2.DefaultResources()

	// 如果配置了自定义资源，覆盖默认值
	if m.opts != nil {
		if m.opts.RelayMaxReservations > 0 {
			res.MaxReservations = m.opts.RelayMaxReservations
		}
		if m.opts.RelayMaxCircuits > 0 {
			res.MaxCircuits = m.opts.RelayMaxCircuits
		}
		if m.opts.RelayBufferSize > 0 {
			res.BufferSize = m.opts.RelayBufferSize
		}
	}

	relayOptions := []relayv2.Option{
		relayv2.WithResources(res),
	}

	relay, err := relayv2.New(m.host, relayOptions...)
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.relayService = relay
	m.mu.Unlock()

	if m.logger != nil {
		m.logger.Infof("p2p.connectivity relay_service_started")
	}
	return nil
}

// Stats 获取连通性统计信息
func (m *Manager) Stats() ConnectivityStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := ConnectivityStats{
		RelayEnabled:     m.opts != nil && m.opts.EnableRelay,
		HolePunchEnabled: m.opts != nil && m.opts.EnableDCUTR,
		// 是否启用 AutoRelay 由配置项 EnableAutoRelay 决定
		AutoRelayEnabled: m.opts != nil && m.opts.EnableAutoRelay,
		RelayClient:      m.opts != nil && m.opts.EnableRelay,
		AutoNATStatus:    "unknown",
	}

	// 检查中继服务状态
	m.mu.RLock()
	relayActive := m.relayService != nil
	m.mu.RUnlock()

	if relayActive {
		stats.RelayActive = true
		stats.NumRelays = 1
		stats.ActiveRelays = 1
	}

	// 尝试从 host 网络连接中统计中继连接数量
	// 检查是否有通过中继的连接（连接地址中包含 /p2p-circuit）
	if m.host != nil {
		conns := m.host.Network().Conns()
		relayConnCount := 0
		for _, conn := range conns {
			// 检查连接是否通过中继（通过检查 remote multiaddr 是否包含 /p2p-circuit）
			remoteAddr := conn.RemoteMultiaddr()
			if remoteAddr != nil {
				addrStr := remoteAddr.String()
				// 如果地址包含 /p2p-circuit，说明是通过中继的连接
				if contains(addrStr, "/p2p-circuit") {
					relayConnCount++
				}
			}
		}
		// 如果有中继连接，增加统计
		if relayConnCount > 0 {
			stats.ActiveRelays = relayConnCount
		}
	}

	// 优先使用 AutoNAT 客户端的真实状态
	if m.autonatClient != nil {
		status := m.autonatClient.Status()
		switch status {
		case libnetwork.ReachabilityPublic:
			stats.AutoNATStatus = "public"
		case libnetwork.ReachabilityPrivate:
			stats.AutoNATStatus = "private"
		case libnetwork.ReachabilityUnknown:
			stats.AutoNATStatus = "unknown"
		default:
			stats.AutoNATStatus = "unknown"
		}
	} else if m.opts != nil {
		// 如果没有 AutoNAT 客户端，使用强制可达性作为近似值
		switch m.opts.ForceReachability {
		case "public":
			stats.AutoNATStatus = "public"
		case "private":
			stats.AutoNATStatus = "private"
		}
	}

	return stats
}

// StatsMap 获取连通性统计信息（Map 格式，供 Diagnostics 使用）
func (m *Manager) StatsMap() map[string]interface{} {
	stats := m.Stats()
	result := map[string]interface{}{
		"relay_enabled":     stats.RelayEnabled,
		"relay_active":      stats.RelayActive,
		"holepunch_enabled": stats.HolePunchEnabled,
		"autorelay_enabled": stats.AutoRelayEnabled,
		"relay_client":      stats.RelayClient,
		"num_relays":        stats.NumRelays,
		"active_relays":     stats.ActiveRelays,
		"autoNAT_status":    stats.AutoNATStatus,
	}

	// 添加 ConnectionProtector 统计信息
	if m.connectionProtector != nil {
		protectorStats := m.connectionProtector.GetStats()
		if allowedPeers, ok := protectorStats["allowed_peers"].(int); ok {
			result["allowed_peers"] = allowedPeers
		}
		if blockedPeers, ok := protectorStats["blocked_peers"].(int); ok {
			result["blocked_peers"] = blockedPeers
		}
	}

	return result
}


// contains 检查字符串是否包含子串（不区分大小写）
func contains(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

