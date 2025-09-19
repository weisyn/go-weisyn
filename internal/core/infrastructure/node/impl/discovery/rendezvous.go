package discovery

import (
	nodeconfig "github.com/weisyn/v1/internal/config/node"
)

// rendezvous 字符串用于不同发现机制的“会合点”前缀：
// - 以通用前缀开头，避免硬编码具体网络/链 ID；
// - 调用方可在必要时拼接 network_type/chain_id 等后缀，形成隔离的命名空间。
func rendezvousString(cfg *nodeconfig.NodeOptions) string {
	if cfg != nil && cfg.Discovery.RendezvousNamespace != "" {
		return cfg.Discovery.RendezvousNamespace
	}
	// 简化为通用前缀，避免硬编码具体网络/链ID；后续可在调用方拼接 network_type/chain_id
	return "weisyn"
}
