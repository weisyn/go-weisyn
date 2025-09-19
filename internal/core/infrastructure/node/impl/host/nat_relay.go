package host

import (
	libp2p "github.com/libp2p/go-libp2p"

	nodeconfig "github.com/weisyn/v1/internal/config/node"
)

// Connectivity/NAT & Address Filtering：
// - withAdvancedAddressFiltering：将 ConnectionGater 应用为 Host 选项。
// - 其它连通性（NATPortMap、AutoNAT 等）已迁移至 connectivity 包并在 builder/lifecycle 中装配。

func withAdvancedAddressFiltering(cfg *nodeconfig.NodeOptions) []libp2p.Option {
	// 使用支持 CIDR + 前缀的高级过滤器，减少苛刻配置，提高准确性
	g := newAdvancedConnectionGaterFromConfig(cfg)
	return []libp2p.Option{libp2p.ConnectionGater(g)}
}
