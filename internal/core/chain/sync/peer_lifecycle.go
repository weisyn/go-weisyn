package sync

import (
	"context"
	"strings"
	"sync"
	"time"

	libnetwork "github.com/libp2p/go-libp2p/core/network"
	peer "github.com/libp2p/go-libp2p/core/peer"

	"github.com/weisyn/v1/pkg/constants/protocols"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
)

// ======================= Peer lifecycle（最小实现） =======================
//
// 目的：
// - 将 P2P 渐进式时序（discover→dial→identify→protocols）与同步瞬态流程解耦；
// - 在启动阶段，避免因为“未连接/协议缓存暂为空”就把业务节点判死；
// - 通过后台/轻量拨号让节点状态收敛，而不是在业务路径里做硬过滤。

type peerLifecycleState string

const (
	peerStateUnknown    peerLifecycleState = "unknown"
	peerStateDialing    peerLifecycleState = "dialing"
	peerStateConnected  peerLifecycleState = "connected"
	peerStateIdentified peerLifecycleState = "identified"
)

type peerCandidateState struct {
	PeerID        peer.ID
	State         peerLifecycleState
	LastUpdatedAt time.Time
	LastErr       string
}

var (
	peerStateMu  sync.RWMutex
	peerStateMap = make(map[peer.ID]*peerCandidateState)
)

func setPeerState(pid peer.ID, st peerLifecycleState, err error) {
	if pid == "" {
		return
	}
	peerStateMu.Lock()
	defer peerStateMu.Unlock()
	s := peerStateMap[pid]
	if s == nil {
		s = &peerCandidateState{PeerID: pid}
		peerStateMap[pid] = s
	}
	s.State = st
	s.LastUpdatedAt = time.Now()
	if err != nil {
		s.LastErr = err.Error()
	} else {
		s.LastErr = ""
	}
}

// isPublicBootstrapMultiaddr 判断 bootstrap_peers 条目是否为公网 libp2p/bootstrap/IPFS 节点
// 注意：该函数用于“同步候选剔除”，不会影响 discovery 连接行为。
func isPublicBootstrapMultiaddr(addr string) bool {
	a := strings.ToLower(addr)
	return strings.Contains(a, "bootstrap.libp2p.io") || strings.Contains(a, "ipfs") || strings.Contains(a, "go-ipfs")
}

// getConfiguredWESBootstrapPeerIDs 返回配置中的“WES业务 bootstrap peers”（剔除公网 libp2p bootstraps）
func getConfiguredWESBootstrapPeerIDs(cfg config.Provider) []peer.ID {
	if cfg == nil {
		return nil
	}
	nc := cfg.GetNode()
	if nc == nil {
		return nil
	}
	var out []peer.ID
	for _, s := range nc.Discovery.BootstrapPeers {
		if isPublicBootstrapMultiaddr(s) {
			continue
		}
		parts := strings.Split(s, "/p2p/")
		if len(parts) != 2 {
			continue
		}
		if pid, err := peer.Decode(parts[1]); err == nil && pid != "" {
			out = append(out, pid)
		}
	}
	return out
}

// bestEffortEnsureConnected 尝试对 peer 做一次“轻量拨号”，以便尽快完成 identify/协议缓存填充
// - 不会无限重试；由上层门闸循环控制频率。
func bestEffortEnsureConnected(ctx context.Context, p2pService p2pi.Service, pid peer.ID) {
	if pid == "" || p2pService == nil || p2pService.Host() == nil {
		return
	}
	h := p2pService.Host()
	if h.Network() == nil {
		return
	}
	if h.Network().Connectedness(pid) == libnetwork.Connected {
		setPeerState(pid, peerStateConnected, nil)
		return
	}
	ai := peer.AddrInfo{ID: pid, Addrs: h.Peerstore().Addrs(pid)}
	// AddrInfo 为空时 Connect 可能失败，这里仍允许失败并记录（由发现模块持续补全地址）
	setPeerState(pid, peerStateDialing, nil)
	_ = h.Connect(ctx, ai)
}

// hasWESSyncProtocolCached 判断 peerstore 是否已缓存 weisyn 同步协议（用于 readiness gate）
func hasWESSyncProtocolCached(p2pService p2pi.Service, cfg config.Provider, pid peer.ID) bool {
	if pid == "" || p2pService == nil || p2pService.Host() == nil {
		return false
	}
	h := p2pService.Host()
	ps, err := h.Peerstore().GetProtocols(pid)
	if err != nil || len(ps) == 0 {
		return false
	}
	ns := ""
	if cfg != nil {
		func() {
			defer func() { _ = recover() }()
			ns = cfg.GetNetworkNamespace()
		}()
	}
	want := map[string]struct{}{
		protocols.ProtocolKBucketSync:    {},
		protocols.ProtocolSyncHelloV2:    {},
		protocols.ProtocolRangePaginated: {},
		protocols.ProtocolBlockSync:      {},
	}
	if ns != "" {
		want[protocols.QualifyProtocol(protocols.ProtocolKBucketSync, ns)] = struct{}{}
		want[protocols.QualifyProtocol(protocols.ProtocolSyncHelloV2, ns)] = struct{}{}
		want[protocols.QualifyProtocol(protocols.ProtocolRangePaginated, ns)] = struct{}{}
		want[protocols.QualifyProtocol(protocols.ProtocolBlockSync, ns)] = struct{}{}
	}
	for _, p := range ps {
		if _, ok := want[string(p)]; ok {
			return true
		}
	}
	return false
}

// waitForSyncReadiness 启动同步门闸：在进入阶段1.5前，等待至少一个 WES 候选具备可用性（已连接或协议缓存已就绪）
//
// 返回：
// - true：已就绪，允许继续执行同步
// - false：超时/上下文结束（上层应“可恢复返回”，避免阶段1.5硬失败）
func waitForSyncReadiness(ctx context.Context, p2pService p2pi.Service, cfg config.Provider, logger log.Logger, timeout time.Duration) bool {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	deadline := time.Now().Add(timeout)

	if logger != nil {
		logger.Debugf("[TriggerSync] readiness_gate: waiting up to %s for WES peer readiness", timeout)
	}

	for {
		if ctx.Err() != nil {
			return false
		}
		if time.Now().After(deadline) {
			if logger != nil {
				logger.Warnf("[TriggerSync] readiness_gate: timeout after %s (no WES peer ready yet)", timeout)
			}
			return false
		}

		// 1) 优先：配置的 WES bootstrap peers（排除 public bootstrap）
		boots := getConfiguredWESBootstrapPeerIDs(cfg)
		for _, pid := range boots {
			bestEffortEnsureConnected(ctx, p2pService, pid)
			if p2pService != nil && p2pService.Host() != nil && p2pService.Host().Network() != nil {
				if p2pService.Host().Network().Connectedness(pid) == libnetwork.Connected {
					setPeerState(pid, peerStateConnected, nil)
					return true
				}
			}
			// 即使尚未 connected，只要协议缓存已经就绪，也允许继续（后续阶段会真正发起请求）
			if hasWESSyncProtocolCached(p2pService, cfg, pid) {
				setPeerState(pid, peerStateIdentified, nil)
				return true
			}
		}

		// 2) 次优：任意已连接 peer 中，已有 weisyn 同步协议缓存
		if p2pService != nil && p2pService.Host() != nil && p2pService.Host().Network() != nil {
			for _, pid := range p2pService.Host().Network().Peers() {
				if hasWESSyncProtocolCached(p2pService, cfg, pid) {
					setPeerState(pid, peerStateIdentified, nil)
					return true
				}
			}
		}

		time.Sleep(500 * time.Millisecond)
	}
}


