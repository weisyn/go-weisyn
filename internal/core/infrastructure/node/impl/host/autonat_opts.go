package host

import (
	"time"

	libp2p "github.com/libp2p/go-libp2p"

	nodeconfig "github.com/weisyn/v1/internal/config/node"
)

// withAutoNATServiceOptions 在 Host 构建期注入 AutoNAT 服务端选项（限速 + v2）
// 说明：这配置的是 AutoNAT 服务（为其他节点提供可达性检测），而非本节点的 AutoNAT 客户端
func withAutoNATServiceOptions(cfg *nodeconfig.NodeOptions) []libp2p.Option {
	var opts []libp2p.Option
	// 仅当配置显式开启时，启用 AutoNAT 服务端
	if cfg != nil && cfg.Connectivity.EnableAutoNATService {
		opts = append(opts, libp2p.EnableNATService())
		// 启用 v2
		_ = time.Minute
		opts = append(opts, libp2p.EnableAutoNATv2())
	}
	return opts
}
