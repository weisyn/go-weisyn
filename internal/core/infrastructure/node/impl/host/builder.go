package host

// 本文件负责构建生产可用的 libp2p Host，聚合传输、安全、复用、连接与资源策略等选项。
// 仅做装配，不包含业务逻辑，遵循高内聚低耦合，便于测试与维护。
//
// 设计说明：
// - 装配顺序遵循底座优先：Transports → Security → Muxers → Conn/Resource/Bandwidth → Identity → Gater/Addrs → ListenAddrs → Extra；
//   这样能确保后续选项能读取到前置能力的上下文（如带宽计数器、ResourceManager 等）。
// - enrichListenAddresses 会在启用 QUIC/WS 时自动补全对应监听地址，减少配置负担，同时保持兼容 TCP。

import (
	"context"

	libp2p "github.com/libp2p/go-libp2p"
	lphost "github.com/libp2p/go-libp2p/core/host"
	ma "github.com/multiformats/go-multiaddr"

	nodeconfig "github.com/weisyn/v1/internal/config/node"
	connpkg "github.com/weisyn/v1/internal/core/infrastructure/node/impl/connectivity"
	relaypkg "github.com/weisyn/v1/internal/core/infrastructure/node/impl/connectivity/relay"
)

// newHost 根据配置装配 libp2p Host。
// 装配顺序：传输 → 安全 → 复用 → 连接管理/资源管理/带宽 → 身份 → 地址过滤 → 监听地址 → 额外选项。
func newHost(ctx context.Context, cfg *nodeconfig.NodeOptions, extra ...libp2p.Option) (lphost.Host, error) {
	var opts []libp2p.Option
	// DNS 解析：使用 libp2p 默认解析器，避免未初始化的自定义解析器引发崩溃
	// 传输/安全/复用
	opts = append(opts, withTransportOptions(cfg)...)
	opts = append(opts, withSecurityOptions(cfg)...)
	opts = append(opts, withMuxerOptions(cfg)...)
	// 私有网络（PSK）
	opts = append(opts, withPrivateNetworkOptions(cfg)...)
	// 连接/资源/带宽/身份
	opts = append(opts, withConnectionManagerOptions(cfg)...)
	opts = append(opts, withResourceManagerOptions(cfg)...)
	opts = append(opts, withBandwidthLimiterOptions(cfg)...)
	opts = append(opts, withIdentityOptions(cfg)...)
	// 地址过滤
	opts = append(opts, withAdvancedAddressFiltering(cfg)...)
	// AutoNAT 服务（限速 + v2）在构建期统一注入
	opts = append(opts, withAutoNATServiceOptions(cfg)...)
	// Connectivity 选项：NATPortMap / Reachability / Relay / AutoRelay / HolePunching / RelayService
	opts = append(opts, connpkg.WithNATPortMapOptions(cfg)...)
	opts = append(opts, connpkg.WithReachabilityOptions(cfg)...)
	opts = append(opts, relaypkg.WithRelayTransportOptions(cfg)...)
	// 动态 AutoRelay（零配置启用）优先于静态
	opts = append(opts, relaypkg.WithAutoRelayDynamicOptions(cfg)...)
	opts = append(opts, relaypkg.WithAutoRelayStaticOptions(cfg)...)
	opts = append(opts, relaypkg.WithHolePunchingOptions(cfg)...)
	opts = append(opts, relaypkg.WithRelayServiceOptions(cfg)...)

	// 监听地址：零配置回退；存在配置时按配置
	if cfg == nil || len(cfg.Host.ListenAddresses) == 0 {
		fallback := []string{
			"/ip4/0.0.0.0/tcp/0",
			"/ip6/::/tcp/0",
			"/ip4/0.0.0.0/udp/0/quic-v1",
			"/ip6/::/udp/0/quic-v1",
		}
		opts = append(opts, libp2p.ListenAddrStrings(fallback...))
	} else {
		addrs := cfg.Host.ListenAddresses
		addrs = enrichListenAddresses(addrs, cfg)
		opts = append(opts, libp2p.ListenAddrStrings(addrs...))
	}

	opts = append(opts, extra...)
	return libp2p.New(opts...)
}

// enrichListenAddresses 在启用 QUIC/WS 时自动补全监听 multiaddrs
func enrichListenAddresses(base []string, cfg *nodeconfig.NodeOptions) []string {
	hasQUIC := false
	hasWS := false
	if cfg != nil {
		hasQUIC = cfg.Host.Transport.EnableQUIC
		hasWS = cfg.Host.Transport.EnableWebSocket
	}
	if !hasQUIC && !hasWS {
		return base
	}

	existing := make(map[string]struct{}, len(base))
	for _, s := range base {
		existing[s] = struct{}{}
	}

	for _, s := range base {
		m, err := ma.NewMultiaddr(s)
		if err != nil {
			continue
		}
		if hasQUIC {
			if _, err := m.ValueForProtocol(ma.P_TCP); err == nil {
				if ip4, err := m.ValueForProtocol(ma.P_IP4); err == nil && ip4 != "" {
					port, _ := m.ValueForProtocol(ma.P_TCP)
					quicStr := "/ip4/" + ip4 + "/udp/" + port + "/quic-v1"
					if _, ok := existing[quicStr]; !ok {
						base = append(base, quicStr)
						existing[quicStr] = struct{}{}
					}
				}
				if ip6, err := m.ValueForProtocol(ma.P_IP6); err == nil && ip6 != "" {
					port, _ := m.ValueForProtocol(ma.P_TCP)
					quicStr := "/ip6/" + ip6 + "/udp/" + port + "/quic-v1"
					if _, ok := existing[quicStr]; !ok {
						base = append(base, quicStr)
						existing[quicStr] = struct{}{}
					}
				}
			}
		}
		if hasWS {
			if _, err := m.ValueForProtocol(ma.P_TCP); err == nil {
				wsStr := s + "/ws"
				if _, ok := existing[wsStr]; !ok {
					base = append(base, wsStr)
					existing[wsStr] = struct{}{}
				}
			}
		}
	}
	return base
}
