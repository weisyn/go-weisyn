package relay

import (
	libp2p "github.com/libp2p/go-libp2p"

	nodeconfig "github.com/weisyn/v1/internal/config/node"
)

// WithRelayTransportOptions 基于配置返回中继传输开关
func WithRelayTransportOptions(cfg *nodeconfig.NodeOptions) []libp2p.Option {
	var opts []libp2p.Option
	if cfg == nil {
		return []libp2p.Option{libp2p.EnableRelay()}
	}
	if cfg.Connectivity.EnableAutoRelay || cfg.Connectivity.ForceReachability == "private" || cfg.Connectivity.EnableRelayTransport {
		opts = append(opts, libp2p.EnableRelay())
	}
	return opts
}
