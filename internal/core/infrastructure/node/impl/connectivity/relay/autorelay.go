package relay

import (
	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"

	nodeconfig "github.com/weisyn/v1/internal/config/node"
)

// WithAutoRelayStaticOptions 若配置包含静态中继清单，则返回对应 AutoRelay 选项
func WithAutoRelayStaticOptions(cfg *nodeconfig.NodeOptions) []libp2p.Option {
	var opts []libp2p.Option
	if cfg == nil || !cfg.Connectivity.EnableAutoRelay {
		return opts
	}
	static := cfg.Discovery.StaticRelayPeers
	if len(static) == 0 {
		static = cfg.Discovery.BootstrapPeers
	}
	if len(static) == 0 {
		return opts
	}
	var infos []peer.AddrInfo
	for _, s := range static {
		m, err := ma.NewMultiaddr(s)
		if err != nil {
			continue
		}
		if info, err := peer.AddrInfoFromP2pAddr(m); err == nil {
			infos = append(infos, *info)
		}
	}
	if len(infos) > 0 {
		opts = append(opts, libp2p.EnableAutoRelayWithStaticRelays(infos))
	}
	return opts
}
