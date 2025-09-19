package host

// Package host 提供 P2P Host 运行期所需的装配选项与组件实现。
// 本文件实现地址公告过滤（AddrsFactory），确保不会对外通告无效/不期望的地址。
//
// 过滤规则：
// - 剔除未指定地址（0.0.0.0 / ::）与回环地址（127.0.0.1 / ::1）；
// - 私网地址仅在配置允许时通告；
// - 公网全局单播地址保留；
// - 非 IP 地址（如 /p2p-circuit）原样保留；
// - 若过滤后为空，回退为原始地址集合，避免“无地址可用”导致的连通性问题。

import (
	libp2p "github.com/libp2p/go-libp2p"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	mamask "github.com/whyrusleeping/multiaddr-filter"

	nodeconfig "github.com/weisyn/v1/internal/config/node"
)

// withAddressFactory 返回用于过滤公告地址的 libp2p 选项。
// - 过滤未指定地址（如 0.0.0.0）
// - 忽略回环地址，优先保留私有/公网可路由地址
func withAddressFactoryByConfig(cfg *nodeconfig.NodeOptions) libp2p.Option {
	advertisePrivate := false
	var announce, appendAnnounce, noAnnounce []string
	if cfg != nil {
		advertisePrivate = cfg.Host.AdvertisePrivateAddrs
		announce = append([]string{}, cfg.Host.Announce...)
		appendAnnounce = append([]string{}, cfg.Host.AppendAnnounce...)
		noAnnounce = append([]string{}, cfg.Host.NoAnnounce...)
	}
	return libp2p.AddrsFactory(func(in []ma.Multiaddr) []ma.Multiaddr {
		// 1) 基础集合
		base := in
		if len(announce) > 0 {
			base = make([]ma.Multiaddr, 0, len(announce))
			for _, s := range announce {
				if m, err := ma.NewMultiaddr(s); err == nil {
					base = append(base, m)
				}
			}
		}
		// 2) 追加地址
		if len(appendAnnounce) > 0 {
			seen := make(map[string]struct{}, len(base))
			for _, m := range base {
				seen[string(m.Bytes())] = struct{}{}
			}
			for _, s := range appendAnnounce {
				if m, err := ma.NewMultiaddr(s); err == nil {
					if _, ok := seen[string(m.Bytes())]; !ok {
						base = append(base, m)
						seen[string(m.Bytes())] = struct{}{}
					}
				}
			}
		}
		// 3) NoAnnounce 规则：CIDR 与精确地址
		filters := ma.NewFilters()
		exact := map[string]bool{}
		for _, s := range noAnnounce {
			if f, err := mamask.NewMask(s); err == nil {
				filters.AddFilter(*f, ma.ActionDeny)
				continue
			}
			if m, err := ma.NewMultiaddr(s); err == nil {
				exact[string(m.Bytes())] = true
			}
		}
		// 4) IP 规则过滤与生成输出
		out := make([]ma.Multiaddr, 0, len(base))
		for _, a := range base {
			if manet.IsIPUnspecified(a) {
				continue
			}
			if exact[string(a.Bytes())] {
				continue
			}
			if filters.AddrBlocked(a) {
				continue
			}
			if ip, err := manet.ToIP(a); err == nil {
				if ip.IsLoopback() {
					continue
				}
				if ip.IsPrivate() && !advertisePrivate {
					continue
				}
			}
			out = append(out, a)
		}
		if len(out) == 0 {
			// 兜底策略：若全部被过滤，保留非回环地址以保证局域网可发现（例如 mDNS 情况）
			fallback := make([]ma.Multiaddr, 0, len(in))
			for _, a := range in {
				if manet.IsIPUnspecified(a) {
					continue
				}
				if ip, err := manet.ToIP(a); err == nil {
					if ip.IsLoopback() {
						continue
					}
				}
				fallback = append(fallback, a)
			}
			if len(fallback) == 0 {
				return in
			}
			return fallback
		}
		return out
	})
}
