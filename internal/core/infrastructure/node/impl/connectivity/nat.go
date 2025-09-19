package connectivity

import (
	libp2p "github.com/libp2p/go-libp2p"

	nodeconfig "github.com/weisyn/v1/internal/config/node"
)

// WithNATPortMapOptions 根据配置构建 NAT 端口映射选项
func WithNATPortMapOptions(cfg *nodeconfig.NodeOptions) []libp2p.Option {
	var opts []libp2p.Option
	// 连接优先：缺省启用；配置简化后直接检查配置字段
	if cfg == nil || cfg.Connectivity.EnableNATPort {
		opts = append(opts, libp2p.NATPortMap())
	}
	return opts
}
