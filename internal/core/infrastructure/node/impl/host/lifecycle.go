package host

// 本文件提供 Host 运行时生命周期管理：
// - Start：装配并启动 libp2p Host，随后启动诊断、连通性等服务
// - Stop：优雅关闭 Host 和所有相关服务
// 保持职责单一，仅负责生命周期，不参与业务逻辑。

import (
	"context"
	"fmt"

	lphost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/metrics"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"

	nodeconfig "github.com/weisyn/v1/internal/config/node"
	relaydyn "github.com/weisyn/v1/internal/core/infrastructure/node/impl/connectivity/relay"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// Runtime 负责 Host 的创建与关闭
type Runtime struct {
	cfg                 *nodeconfig.NodeOptions
	logger              logiface.Logger
	host                lphost.Host
	diagnosticsManager  *DiagnosticsManager
	connectivityManager *AdvancedConnectivityManager
	connectionProtector *ConnectionProtector
}

func NewRuntime(cfg *nodeconfig.NodeOptions, logger logiface.Logger) (*Runtime, error) {
	return &Runtime{
		cfg:                 cfg,
		logger:              logger,
		connectionProtector: NewConnectionProtector(),
	}, nil
}

// Start 装配选项并启动 Host，随后启动所有相关服务。
func (r *Runtime) Start(ctx context.Context) error {
	if r.host != nil {
		return nil
	}

	// 构建连接选项
	opts := withAddressFactoryByConfig(r.cfg)

	// 创建主机（注意：opts 为单个 libp2p.Option，不要展开）
	h, err := newHost(ctx, r.cfg, opts)
	if err != nil {
		return fmt.Errorf("create host: %w", err)
	}
	r.host = h

	// 向动态 AutoRelay 提供 Host 访问器
	relaydyn.SetHostProvider(func() lphost.Host { return r.host })

	if r.logger != nil {
		// 动态 AutoRelay 标志与默认策略保持一致（默认关闭，显式开启后为true）
		autorelayDyn := r.cfg != nil && r.cfg.Connectivity.EnableAutoRelay
		// 打印更详细的配置摘要，便于生产排障
		if r.cfg != nil {
			r.logger.Infof("node host started: id=%s addrs=%v min_peers=%d max_peers=%d nat_portmap=%t diagnostics=%t autorelay=%t autorelay_dynamic=%t", h.ID().String(), h.Addrs(), r.cfg.Connectivity.MinPeers, r.cfg.Connectivity.MaxPeers, r.cfg.Connectivity.EnableNATPort, r.cfg.Host.DiagnosticsEnabled, r.cfg.Connectivity.EnableAutoRelay, autorelayDyn)
		} else {
			r.logger.Infof("node host started: id=%s addrs=%v (zero-config: listen=fallback tcp/quic, autorelay_dynamic=%t)", h.ID().String(), h.Addrs(), autorelayDyn)
		}
	}

	// 保护引导/核心 peers，避免被连接管理器修剪
	if r.cfg != nil {
		for _, s := range r.cfg.Discovery.BootstrapPeers {
			m, err := ma.NewMultiaddr(s)
			if err != nil {
				continue
			}
			if info, err := peer.AddrInfoFromP2pAddr(m); err == nil {
				if cm := r.host.ConnManager(); cm != nil {
					cm.Protect(info.ID, "bootstrap")
				}
			}
		}
	}

	// AutoNAT 服务已在 builder 阶段注入（限速/v2），无需运行期启动

	// 启动诊断管理器（配置化）
	if r.cfg != nil && r.cfg.Host.DiagnosticsEnabled {
		port := 8080
		if r.cfg.Host.DiagnosticsPort > 0 {
			port = r.cfg.Host.DiagnosticsPort
		}
		bwReporter := metrics.NewBandwidthCounter()
		dm := NewDiagnosticsManager(r.host, bwReporter, port)
		// 复用与 Host 相同的带宽计数器
		dm.bw = sharedBandwidthCounter
		r.diagnosticsManager = dm
		if err := r.diagnosticsManager.Start(); err != nil {
			if r.logger != nil {
				r.logger.Warnf("diagnostics manager start failed: %v", err)
			}
		} else if r.logger != nil {
			r.logger.Infof("diagnostics server started on :%d", port)
		}
	}

	// 已通过 builder 的 libp2p 选项启用 Relay/HolePunch/AutoRelay/RelayService；
	// 不再启动自管理的连接性管理器以避免重复装配。

	return nil
}

// Stop 关闭 Host 和所有相关服务。
func (r *Runtime) Stop(ctx context.Context) error {
	// 停止高级连通性管理器
	if r.connectivityManager != nil {
		if err := r.connectivityManager.Stop(); err != nil && r.logger != nil {
			r.logger.Warnf("connectivity manager stop error: %v", err)
		}
		r.connectivityManager = nil
	}

	// 停止诊断管理器
	if r.diagnosticsManager != nil {
		if err := r.diagnosticsManager.Stop(); err != nil && r.logger != nil {
			r.logger.Warnf("diagnostics manager stop error: %v", err)
		}
		r.diagnosticsManager = nil
	}

	// 关闭主机
	if r.host != nil {
		_ = r.host.Close()
		r.host = nil
		if r.logger != nil {
			r.logger.Infof("node host stopped")
		}
	}

	return nil
}

// Host 返回内部 host，供 discovery 使用。
func (r *Runtime) Host() lphost.Host {
	return r.host
}

// GetDiagnosticsManager 返回诊断管理器
func (r *Runtime) GetDiagnosticsManager() *DiagnosticsManager {
	return r.diagnosticsManager
}

// GetConnectivityManager 返回连通性管理器
func (r *Runtime) GetConnectivityManager() *AdvancedConnectivityManager {
	return r.connectivityManager
}

// GetConnectionProtector 返回连接保护器
func (r *Runtime) GetConnectionProtector() *ConnectionProtector {
	return r.connectionProtector
}

// GetLogger 返回logger实例
func (r *Runtime) GetLogger() logiface.Logger {
	return r.logger
}

// GetStats 获取运行时统计信息
func (r *Runtime) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"host_active": r.host != nil,
	}

	if r.host != nil {
		stats["peer_id"] = r.host.ID().String()
		stats["addresses"] = r.host.Addrs()
		stats["connected_peers"] = len(r.host.Network().Peers())
		stats["connections"] = len(r.host.Network().Conns())
	}

	if r.diagnosticsManager != nil {
		stats["diagnostics_enabled"] = true
	}

	if r.connectivityManager != nil {
		relayStats := r.connectivityManager.GetRelayStats()
		stats["connectivity"] = relayStats
	}

	if r.connectionProtector != nil {
		protectorStats := r.connectionProtector.GetStats()
		stats["protection"] = protectorStats
	}

	return stats
}
