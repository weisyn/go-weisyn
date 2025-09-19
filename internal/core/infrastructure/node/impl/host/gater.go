package host

// 本文件提供地址过滤（ConnectionGater）的实现，支持 CIDR 与前缀混合策略。
// 优先使用 CIDR（multiaddr-filter），保留前缀列表作为兜底，尽量减少苛刻配置。

import (
	"net"

	ccmgr "github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/control"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
	mamask "github.com/whyrusleeping/multiaddr-filter"

	nodeconfig "github.com/weisyn/v1/internal/config/node"
)

// advancedAddressGater 支持 CIDR + 前缀的混合过滤。
// 优先级：
// - 若 allowed 非空：优先使用 allow 的 CIDR 白名单；未命中再按前缀判定；仍未命中则拒绝；
// - 若 allowed 为空：先评估 CIDR deny；再评估 blocked 前缀；均未命中则放行。
//
// 注意：allowed 的 CIDR 仅对 IP 地址匹配有效；非 IP multiaddr 仅使用前缀判断。
// 空 allow/blocked 均表示不过滤（全放行）。

type advancedAddressGater struct {
	filters     *ma.Filters
	allowed     []string
	blocked     []string
	allowedCIDR []*net.IPNet
}

func newAdvancedAddressGater(allowed, blocked []string, filters *ma.Filters) *advancedAddressGater {
	return &advancedAddressGater{filters: filters, allowed: allowed, blocked: blocked, allowedCIDR: parseCIDRs(allowed)}
}

// InterceptPeerDial 允许所有 Peer 级拨号（地址级过滤在后续流程进行）。
func (g *advancedAddressGater) InterceptPeerDial(id peer.ID) (allow bool) { return true }

// InterceptAddrDial 对拨号地址进行校验。
func (g *advancedAddressGater) InterceptAddrDial(id peer.ID, addr ma.Multiaddr) (allow bool) {
	return g.allowAddr(addr)
}

// InterceptAccept 对入站连接的远端地址进行校验。
func (g *advancedAddressGater) InterceptAccept(conn network.ConnMultiaddrs) (allow bool) {
	return g.allowAddr(conn.RemoteMultiaddr())
}

// InterceptSecured 对已握手连接的远端地址进行校验。
func (g *advancedAddressGater) InterceptSecured(dir network.Direction, id peer.ID, conn network.ConnMultiaddrs) (allow bool) {
	return g.allowAddr(conn.RemoteMultiaddr())
}

// InterceptUpgraded 在连接升级后再次校验并可选择断开。
func (g *advancedAddressGater) InterceptUpgraded(conn network.Conn) (allow bool, reason control.DisconnectReason) {
	return g.allowAddr(conn.RemoteMultiaddr()), 0
}

func (g *advancedAddressGater) allowAddr(addr ma.Multiaddr) bool {
	addrStr := addr.String()
	// 1) allowed 优先：非空时使用白名单模式
	if len(g.allowed) > 0 {
		// 1.1 CIDR 白名单（仅 IP 地址）
		if ip := toIP(addr); ip != nil {
			for _, n := range g.allowedCIDR {
				if n.Contains(ip) {
					return true
				}
			}
		}
		// 1.2 前缀白名单
		for _, a := range g.allowed {
			if a != "" && hasPrefix(addrStr, a) {
				return true
			}
		}
		return false
	}
	// 2) 默认模式：先 CIDR deny，再 blocked 前缀 deny，最后放行
	if g.filters != nil && g.filters.AddrBlocked(addr) {
		return false
	}
	for _, b := range g.blocked {
		if b != "" && hasPrefix(addrStr, b) {
			return false
		}
	}
	return true
}

func hasPrefix(s, prefix string) bool {
	if len(prefix) == 0 {
		return true
	}
	if len(prefix) > len(s) {
		return false
	}
	return s[:len(prefix)] == prefix
}

// newAdvancedConnectionGaterFromConfig 根据配置构建支持 CIDR 的地址过滤器
func newAdvancedConnectionGaterFromConfig(cfg *nodeconfig.NodeOptions) ccmgr.ConnectionGater {
	var allowed, blocked []string
	if cfg != nil {
		allowed = cfg.Host.Gater.AllowedPrefixes
		blocked = cfg.Host.Gater.BlockedPrefixes
	}

	// 将 blocked 尝试解析为 CIDR 过滤器（兼容旧配置），失败则回退到前缀判断
	var filters *ma.Filters
	if len(blocked) > 0 {
		filters = ma.NewFilters()
		parsed := 0
		for _, rule := range blocked {
			if f, err := mamask.NewMask(rule); err == nil {
				filters.AddFilter(*f, ma.ActionDeny)
				parsed++
			}
		}
		if parsed == 0 {
			filters = nil // 保持为空，使用前缀兜底
		}
	}

	return newAdvancedAddressGater(allowed, blocked, filters)
}

var _ ccmgr.ConnectionGater = (*advancedAddressGater)(nil)

// parseCIDRs 解析允许列表中的 CIDR 规则
func parseCIDRs(rules []string) []*net.IPNet {
	var out []*net.IPNet
	for _, r := range rules {
		_, n, err := net.ParseCIDR(r)
		if err == nil && n != nil {
			out = append(out, n)
		}
	}
	return out
}

// toIP 从 multiaddr 提取 IP（若存在）
func toIP(addr ma.Multiaddr) net.IP {
	if v, err := addr.ValueForProtocol(ma.P_IP4); err == nil {
		return net.ParseIP(v)
	}
	if v, err := addr.ValueForProtocol(ma.P_IP6); err == nil {
		return net.ParseIP(v)
	}
	return nil
}
