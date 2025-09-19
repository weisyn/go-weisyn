package relay

import (
	"context"

	libp2p "github.com/libp2p/go-libp2p"
	lphost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"

	nodeconfig "github.com/weisyn/v1/internal/config/node"
)

var hostProvider func() lphost.Host

// SetHostProvider 提供 host 访问器，供 AutoRelay PeerSource 在 Host 构建完成后读取运行时信息。
func SetHostProvider(f func() lphost.Host) { hostProvider = f }

// WithAutoRelayDynamicOptions 在零配置或显式启用时，注入基于 PeerSource 的 AutoRelay 选项。
// PeerSource 策略：
// 1) 优先使用当前已连接 peers（Network().Peers()），并附带已知地址；
// 2) 不足时从 Peerstore.PeersWithAddrs() 兜底；
// 3) 返回数量受 numPeers 限制。
func WithAutoRelayDynamicOptions(cfg *nodeconfig.NodeOptions) []libp2p.Option {
	// 若存在配置且显式关闭，则不注入
	if cfg != nil && !cfg.Connectivity.EnableAutoRelay {
		return nil
	}
	// 候选上限：优先使用配置
	limit := 16
	if cfg != nil && cfg.Connectivity.AutoRelayDynamicCandidates > 0 {
		limit = cfg.Connectivity.AutoRelayDynamicCandidates
	}
	ps := func(ctx context.Context, numPeers int) <-chan peer.AddrInfo {
		if numPeers <= 0 || numPeers > limit {
			numPeers = limit
		}
		ch := make(chan peer.AddrInfo, numPeers)
		go func() {
			defer close(ch)
			if hostProvider == nil {
				return
			}
			h := hostProvider()
			if h == nil {
				return
			}
			seen := make(map[peer.ID]struct{}, numPeers)
			// 1) 已连接 peers
			for _, pid := range h.Network().Peers() {
				if _, ok := seen[pid]; ok {
					continue
				}
				ai := peer.AddrInfo{ID: pid, Addrs: h.Peerstore().Addrs(pid)}
				if len(ai.Addrs) > 0 {
					ch <- ai
					seen[pid] = struct{}{}
					if len(seen) >= numPeers {
						return
					}
				}
			}
			// 2) Peerstore 兜底
			if len(seen) < numPeers {
				for _, pid := range h.Peerstore().PeersWithAddrs() {
					if _, ok := seen[pid]; ok {
						continue
					}
					ai := peer.AddrInfo{ID: pid, Addrs: h.Peerstore().Addrs(pid)}
					if len(ai.Addrs) == 0 {
						continue
					}
					ch <- ai
					seen[pid] = struct{}{}
					if len(seen) >= numPeers {
						return
					}
				}
			}
		}()
		return ch
	}
	return []libp2p.Option{libp2p.EnableAutoRelayWithPeerSource(ps)}
}
