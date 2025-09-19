package connectivity

import (
	lphost "github.com/libp2p/go-libp2p/core/host"
	autonat "github.com/libp2p/go-libp2p/p2p/host/autonat"

	nodeconfig "github.com/weisyn/v1/internal/config/node"
)

// StartAutoNAT 在 Host 启动后启动 AutoNAT（按配置）
// 等价迁移自 impl/host/nat_relay.go 的 startAutoNAT
func StartAutoNAT(h lphost.Host, cfg *nodeconfig.NodeOptions) error {
	if h == nil {
		return nil
	}
	// 仅当显式启用客户端时启动
	if cfg != nil && cfg.Connectivity.EnableAutoNATClient {
		_, err := autonat.New(h)
		return err
	}
	return nil
}
