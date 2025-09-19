package relay

import (
	libp2p "github.com/libp2p/go-libp2p"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"

	nodeconfig "github.com/weisyn/v1/internal/config/node"
)

// WithRelayServiceOptions 启用 Relay 服务端（使用默认资源配额）
// 后续如有更细资源配置，可在此扩展并映射到 relay.Resources
func WithRelayServiceOptions(cfg *nodeconfig.NodeOptions) []libp2p.Option {
	var opts []libp2p.Option
	if cfg == nil || !cfg.Connectivity.EnableRelayService {
		return opts
	}

	// 使用默认资源配置 - 可在 cfg.Connectivity.RelayService 中扩展
	res := relayv2.DefaultResources()
	opts = append(opts, libp2p.EnableRelayService(relayv2.WithResources(res)))
	return opts
}
