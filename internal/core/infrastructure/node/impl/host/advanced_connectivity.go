package host

import (
	"context"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"

	nodeconfig "github.com/weisyn/v1/internal/config/node"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// AdvancedConnectivityManager 高级连通性管理器
type AdvancedConnectivityManager struct {
	host               host.Host
	relayService       *relayv2.Relay
	relayClientEnabled bool
	holepunchEnabled   bool
	autorelayEnabled   bool
	logger             logiface.Logger
}

// NewAdvancedConnectivityManager 创建高级连通性管理器
func NewAdvancedConnectivityManager(h host.Host, cfg *nodeconfig.NodeOptions) *AdvancedConnectivityManager {
	acm := &AdvancedConnectivityManager{host: h}
	if cfg != nil {
		acm.relayClientEnabled = cfg.Connectivity.EnableAutoRelay
		acm.holepunchEnabled = cfg.Connectivity.EnableDCUtR
		acm.autorelayEnabled = cfg.Connectivity.EnableAutoRelay
	}
	return acm
}

// NewAdvancedConnectivityManagerWithLogger 创建带日志的高级连通性管理器
func NewAdvancedConnectivityManagerWithLogger(h host.Host, cfg *nodeconfig.NodeOptions, logger logiface.Logger) *AdvancedConnectivityManager {
	acm := NewAdvancedConnectivityManager(h, cfg)
	acm.logger = logger
	return acm
}

// Start 启动高级连通性服务
func (acm *AdvancedConnectivityManager) Start(ctx context.Context) error {
	if acm.logger != nil {
		acm.logger.Infof("p2p.host.connectivity start relay_client=%t dcutr=%t autorelay=%t", acm.relayClientEnabled, acm.holepunchEnabled, acm.autorelayEnabled)
	}
	// 启动 Circuit Relay v2 服务（服务端，仅在外层按配置启用时调用 Start）
	if err := acm.startRelayService(ctx); err != nil {
		if acm.logger != nil {
			acm.logger.Warnf("p2p.host.connectivity relay_service_start_failed error=%v", err)
		}
		return err
	}

	// 启动 DCUtR (Hole Punching) 服务（通过 Host 选项启用，这里占位）
	if err := acm.startHolePunchService(ctx); err != nil {
		if acm.logger != nil {
			acm.logger.Warnf("p2p.host.connectivity dcutr_start_failed error=%v", err)
		}
		return err
	}

	// 启动 AutoRelay 服务（通过 Host 选项启用，这里占位）
	if err := acm.startAutoRelayService(ctx); err != nil {
		if acm.logger != nil {
			acm.logger.Warnf("p2p.host.connectivity autorelay_start_failed error=%v", err)
		}
		return err
	}

	if acm.logger != nil {
		acm.logger.Infof("p2p.host.connectivity started")
	}
	return nil
}

// Stop 停止高级连通性服务
func (acm *AdvancedConnectivityManager) Stop() error {
	// 停止中继服务
	if acm.relayService != nil {
		if err := acm.relayService.Close(); err != nil {
			if acm.logger != nil {
				acm.logger.Warnf("p2p.host.connectivity relay_service_stop_failed error=%v", err)
			}
			return err
		}
		acm.relayService = nil
	}

	if acm.logger != nil {
		acm.logger.Infof("p2p.host.connectivity stopped")
	}

	return nil
}

// startRelayService 启动 Circuit Relay v2 服务
func (acm *AdvancedConnectivityManager) startRelayService(ctx context.Context) error {
	// 配置 Circuit Relay v2 服务
	relayOptions := []relayv2.Option{
		relayv2.WithResources(relayv2.Resources{
			Limit: &relayv2.RelayLimit{
				Duration: 2 * time.Minute,
				Data:     1 << 17, // 128KB
			},
			ReservationTTL:  1 * time.Hour,
			MaxReservations: 128,
			MaxCircuits:     16,
			BufferSize:      2048,
		}),
	}

	relay, err := relayv2.New(acm.host, relayOptions...)
	if err != nil {
		return err
	}

	acm.relayService = relay
	if acm.logger != nil {
		acm.logger.Infof("p2p.host.connectivity relay_service_started")
	}
	return nil
}

// startHolePunchService 启动 DCUtR (Hole Punching) 服务
func (acm *AdvancedConnectivityManager) startHolePunchService(ctx context.Context) error {
	// Hole Punch 服务通过 libp2p.EnableHolePunching() 选项自动启用
	// 这里不需要手动创建服务实例
	return nil
}

// startAutoRelayService 启动 AutoRelay 服务
func (acm *AdvancedConnectivityManager) startAutoRelayService(ctx context.Context) error {
	// AutoRelay 服务通过 libp2p.EnableAutoRelayWithStaticRelays() 选项自动启用
	// 这里不需要手动创建服务实例
	return nil
}

// ConnectViaRelay 通过中继连接到对等节点
func (acm *AdvancedConnectivityManager) ConnectViaRelay(ctx context.Context, relayPeer, targetPeer peer.ID) error {
	// Circuit Relay v2 连接通过标准 libp2p 连接 API 自动处理
	// 当直接连接失败时，libp2p 会自动尝试通过中继连接
	return acm.host.Connect(ctx, peer.AddrInfo{ID: targetPeer})
}

// RequestReservation 请求中继预留
func (acm *AdvancedConnectivityManager) RequestReservation(ctx context.Context, relayPeer peer.ID) error {
	// 中继预留通过 AutoRelay 服务自动管理
	// 这里可以添加手动预留的逻辑，但当前版本使用自动管理
	return nil
}

// GetRelayStats 获取中继统计信息
func (acm *AdvancedConnectivityManager) GetRelayStats() map[string]interface{} {
	stats := map[string]interface{}{
		"relay_enabled":     acm.relayService != nil,
		"holepunch_enabled": acm.holepunchEnabled,
		"autorelay_enabled": acm.autorelayEnabled,
		"relay_client":      acm.relayClientEnabled,
	}

	if acm.relayService != nil {
		stats["relay_active"] = true
	}

	return stats
}

// withAdvancedConnectivityOptions 已迁移：装配改由 connectivity 包提供
func withAdvancedConnectivityOptions(cfg *nodeconfig.NodeOptions) []libp2p.Option { return nil }

// ConnectionProtector 连接保护器
type ConnectionProtector struct {
	allowedPeers map[peer.ID]bool
	blockedPeers map[peer.ID]bool
}

// NewConnectionProtector 创建连接保护器
func NewConnectionProtector() *ConnectionProtector {
	return &ConnectionProtector{
		allowedPeers: make(map[peer.ID]bool),
		blockedPeers: make(map[peer.ID]bool),
	}
}

// AllowPeer 允许特定节点
func (cp *ConnectionProtector) AllowPeer(p peer.ID) {
	cp.allowedPeers[p] = true
	delete(cp.blockedPeers, p)
}

// BlockPeer 阻止特定节点
func (cp *ConnectionProtector) BlockPeer(p peer.ID) {
	cp.blockedPeers[p] = true
	delete(cp.allowedPeers, p)
}

// IsAllowed 检查节点是否被允许
func (cp *ConnectionProtector) IsAllowed(p peer.ID) bool {
	// 如果在阻止列表中，直接拒绝
	if cp.blockedPeers[p] {
		return false
	}

	// 如果有允许列表且节点不在其中，拒绝
	if len(cp.allowedPeers) > 0 && !cp.allowedPeers[p] {
		return false
	}

	return true
}

// GetStats 获取保护器统计信息
func (cp *ConnectionProtector) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"allowed_peers": len(cp.allowedPeers),
		"blocked_peers": len(cp.blockedPeers),
	}
}
