package connectivity

import (
	libp2p "github.com/libp2p/go-libp2p"

	nodeconfig "github.com/weisyn/v1/internal/config/node"
)

// WithReachabilityOptions 将配置映射为 libp2p 可达性选项
func WithReachabilityOptions(cfg *nodeconfig.NodeOptions) []libp2p.Option {
	if cfg == nil {
		return nil
	}
	switch cfg.Connectivity.ForceReachability {
	case "public":
		return []libp2p.Option{libp2p.ForceReachabilityPublic()}
	case "private":
		return []libp2p.Option{libp2p.ForceReachabilityPrivate()}
	default:
		return nil
	}
}
