package relay

import (
	libp2p "github.com/libp2p/go-libp2p"

	nodeconfig "github.com/weisyn/v1/internal/config/node"
)

// WithHolePunchingOptions 基于配置启用 DCUtR（需具备中继客户端能力）
func WithHolePunchingOptions(cfg *nodeconfig.NodeOptions) []libp2p.Option {
	var opts []libp2p.Option
	// 连接优先：cfg 缺失时默认启用（若具备中继客户端能力则生效）
	if cfg == nil {
		return []libp2p.Option{libp2p.EnableHolePunching()}
	}
	if cfg.Connectivity.EnableDCUtR {
		opts = append(opts, libp2p.EnableHolePunching())
	}
	return opts
}
