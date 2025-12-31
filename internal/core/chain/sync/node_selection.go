package sync

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	libnetwork "github.com/libp2p/go-libp2p/core/network"
	peer "github.com/libp2p/go-libp2p/core/peer"

	kbucketimpl "github.com/weisyn/v1/internal/core/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/constants/protocols"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	"github.com/weisyn/v1/pkg/types"
)

func shufflePeersInPlace(peers []peer.ID) {
	if len(peers) <= 1 {
		return
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := len(peers) - 1; i > 0; i-- {
		j := r.Intn(i + 1)
		peers[i], peers[j] = peers[j], peers[i]
	}
}

// peerHintContextKey 用于在上下文中存储同步节点提示信息
type peerHintContextKey struct{}

// syncUrgentContextKey 用于在上下文中标记“紧急同步”触发
// - 紧急同步用于“缺块补齐/分叉处理”等必须立即执行的场景
// - 紧急同步必须绕过 TriggerSync 的去抖与 recently-synced 过滤（仍受 singleflight/锁约束）
type syncUrgentContextKey struct{}

// syncReasonContextKey 用于在上下文中携带“触发原因”（便于可观测性与诊断）
type syncReasonContextKey struct{}

// ======================= 路由表为空时的日志节流策略 =======================

// 说明：
// - 冷启动或网络不佳时，路由表可能长时间为空
// - 选择同步节点会频繁失败，如果每次都打印 warn 日志，会导致刷屏
// - 这里实现一个简单的“指数退避 + 最大间隔”的节流策略：
//     * 初始间隔：5s
//     * 每次打印后，间隔翻倍：5s -> 10s -> 20s -> 40s ...
//     * 上限：60s
//     * 一旦成功选到节点或使用到 peer hint，会立即重置为初始值

var (
	noPeerLogMu          sync.Mutex
	noPeerLastLog        time.Time
	noPeerCurrentBackoff = 5 * time.Second

	noPeerBackoffInitial = 5 * time.Second
	noPeerBackoffMax     = 60 * time.Second
)

// ======================= 上游节点“记忆”缓存（抗网络抖动） =======================
//
// 背景：
// - 在真实网络中，连接抖动、K桶落表时序、链身份/协议过滤等原因会导致“短时间内选不到上游”；
// - 如果每次都退化为 no-op，同步会被频繁打断，难以追平高度。
//
// 目标：
// - 一旦成功选到一个可用上游（K桶或 fallback），缓存为 lastGoodUpstream；
// - 当 K桶临时为空/选不到时，优先复用该上游（需仍处于 Connected 且未被标记 bad）。
var (
	lastUpstreamMu  sync.RWMutex
	lastUpstream    peer.ID
	lastUpstreamAt  time.Time
	lastUpstreamTTL = 10 * time.Minute

	lastUpstreamFailures               int
	lastUpstreamMaxConsecutiveFailures = 3
)

func applyUpstreamMemoryConfig(configProvider config.Provider) {
	// 默认值：与“旧实现”保持一致
	ttl := 10 * time.Minute
	maxFails := 3

	if configProvider != nil {
		if bc := configProvider.GetBlockchain(); bc != nil {
			if bc.Sync.Advanced.UpstreamMemoryTTLSeconds > 0 {
				ttl = time.Duration(bc.Sync.Advanced.UpstreamMemoryTTLSeconds) * time.Second
			}
			if bc.Sync.Advanced.UpstreamMaxConsecutiveFailures > 0 {
				maxFails = bc.Sync.Advanced.UpstreamMaxConsecutiveFailures
			}
		}
	}

	lastUpstreamMu.Lock()
	lastUpstreamTTL = ttl
	lastUpstreamMaxConsecutiveFailures = maxFails
	lastUpstreamMu.Unlock()
}

func setLastGoodUpstream(pid peer.ID) {
	if pid == "" {
		return
	}
	lastUpstreamMu.Lock()
	lastUpstream = pid
	lastUpstreamAt = time.Now()
	lastUpstreamFailures = 0
	lastUpstreamMu.Unlock()
}

func clearLastGoodUpstreamLocked() peer.ID {
	// caller must hold lastUpstreamMu (write)
	old := lastUpstream
	lastUpstream = ""
	lastUpstreamAt = time.Time{}
	lastUpstreamFailures = 0
	return old
}

func recordUpstreamSuccess(pid peer.ID) {
	if pid == "" {
		return
	}
	// 成功意味着该 peer 可用：更新 lastUpstream 并清零失败计数
	setLastGoodUpstream(pid)
}

func recordUpstreamFailure(pid peer.ID, logger log.Logger) {
	if pid == "" {
		return
	}
	lastUpstreamMu.Lock()
	defer lastUpstreamMu.Unlock()

	if lastUpstream == "" || pid != lastUpstream {
		return
	}

	lastUpstreamFailures++
	if lastUpstreamMaxConsecutiveFailures <= 0 {
		return
	}
	if lastUpstreamFailures < lastUpstreamMaxConsecutiveFailures {
		return
	}

	cleared := clearLastGoodUpstreamLocked()
	if logger != nil && cleared != "" {
		logger.Warnf("🧹 bad_upstream_fast_switch: 连续失败达到阈值，清除lastGoodUpstream并切换上游: peer=%s failures=%d threshold=%d",
			cleared.String(), lastUpstreamFailures, lastUpstreamMaxConsecutiveFailures)
	}
}

func getLastGoodUpstream(localPeerID peer.ID, p2pService p2pi.Service) (peer.ID, bool) {
	lastUpstreamMu.RLock()
	pid := lastUpstream
	ts := lastUpstreamAt
	lastUpstreamMu.RUnlock()

	if pid == "" || pid == localPeerID || IsBadPeer(pid) {
		return "", false
	}
	if !ts.IsZero() && time.Since(ts) > lastUpstreamTTL {
		return "", false
	}
	if p2pService == nil || p2pService.Host() == nil || p2pService.Host().Network() == nil {
		return "", false
	}
	if p2pService.Host().Network().Connectedness(pid) != libnetwork.Connected {
		return "", false
	}
	return pid, true
}

func resetNoPeerBackoff() {
	noPeerLogMu.Lock()
	defer noPeerLogMu.Unlock()

	noPeerLastLog = time.Time{}
	noPeerCurrentBackoff = noPeerBackoffInitial
}

func shouldLogNoPeer(now time.Time) bool {
	noPeerLogMu.Lock()
	defer noPeerLogMu.Unlock()

	if noPeerCurrentBackoff <= 0 {
		noPeerCurrentBackoff = noPeerBackoffInitial
	}

	// 第一次或已超过当前退避间隔，允许输出日志
	if noPeerLastLog.IsZero() || now.Sub(noPeerLastLog) >= noPeerCurrentBackoff {
		noPeerLastLog = now
		// 间隔翻倍，直至达到上限
		next := noPeerCurrentBackoff * 2
		if next > noPeerBackoffMax {
			next = noPeerBackoffMax
		}
		noPeerCurrentBackoff = next
		return true
	}
	return false
}

// ContextWithPeerHint 将指定的 peer ID 写入上下文，供同步节点选择阶段使用
func ContextWithPeerHint(ctx context.Context, hint peer.ID) context.Context {
	if hint == "" {
		return ctx
	}
	return context.WithValue(ctx, peerHintContextKey{}, hint)
}

// PeerHintFromContext 尝试从上下文中读取 peer 提示信息（对外导出）
func PeerHintFromContext(ctx context.Context) (peer.ID, bool) {
	return peerHintFromContext(ctx)
}

// ContextWithUrgentSync 标记本次 TriggerSync 为“紧急同步”（可选携带 reason）
func ContextWithUrgentSync(ctx context.Context, reason string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = context.WithValue(ctx, syncUrgentContextKey{}, true)
	if strings.TrimSpace(reason) != "" {
		ctx = context.WithValue(ctx, syncReasonContextKey{}, strings.TrimSpace(reason))
	}
	return ctx
}

// urgentSyncFromContext 读取“紧急同步”标记与原因
func urgentSyncFromContext(ctx context.Context) (urgent bool, reason string) {
	if ctx == nil {
		return false, ""
	}
	if v := ctx.Value(syncUrgentContextKey{}); v != nil {
		if b, ok := v.(bool); ok && b {
			urgent = true
		}
	}
	if v := ctx.Value(syncReasonContextKey{}); v != nil {
		if s, ok := v.(string); ok {
			reason = strings.TrimSpace(s)
		}
	}
	return urgent, reason
}

// peerHintFromContext 尝试从上下文中读取 peer 提示信息
func peerHintFromContext(ctx context.Context) (peer.ID, bool) {
	if ctx == nil {
		return "", false
	}

	val := ctx.Value(peerHintContextKey{})
	if val == nil {
		return "", false
	}

	switch v := val.(type) {
	case peer.ID:
		if v == "" {
			return "", false
		}
		return v, true
	case string:
		if v == "" {
			return "", false
		}
		return peer.ID(v), true
	default:
		return "", false
	}
}

// selectKBucketPeersForSync 基于K桶算法选择同步节点，必要时使用上下文中的 peer 提示兜底
func selectKBucketPeersForSync(
	ctx context.Context,
	routingManager kademlia.RoutingTableManager,
	p2pService p2pi.Service,
	configProvider config.Provider,
	chainInfo *types.ChainInfo,
	logger log.Logger,
) ([]peer.ID, error) {
	if logger != nil {
		logger.Debug("🔍 开始K桶节点选择")
	}

	// 每次选择前读取一次配置（支持运行时调整）
	applyUpstreamMemoryConfig(configProvider)

	// 🔒 防御式编程：RoutingTableManager 在 fx 中标记为 optional
	// 在某些单节点/测试场景下可能未注入，此时直接访问会导致 panic。
	localPeerID := peer.ID("")
	if p2pService != nil && p2pService.Host() != nil {
		localPeerID = p2pService.Host().ID()
	}

	if routingManager == nil {
		if logger != nil {
			logger.Warn("⚠️ RoutingTableManager 未注入，尝试使用上下文中的 peer hint 作为同步目标")
		}
		return buildPeersFromHint(ctx, localPeerID, p2pService, configProvider, logger)
	}

	if localPeerID == "" {
		// 单测/早期启动场景：Host 可能还没准备好，优先尝试 peer hint
		if logger != nil {
			logger.Warn("⚠️ P2P Host 未就绪，无法获取本地节点ID，尝试使用 peer hint 作为同步目标")
		}
		return buildPeersFromHint(ctx, localPeerID, p2pService, configProvider, logger)
	}

	routingTable := routingManager.GetRoutingTable()
	if routingTable == nil {
		if logger != nil {
			logger.Warnf("⚠️ routingManager.GetRoutingTable() 返回 nil：routingManager=%T localPeerID=%s，将回退到 peer hint/lastGoodUpstream/connected-peers",
				routingManager, localPeerID.String())
		}
		return buildPeersFromHint(ctx, localPeerID, p2pService, configProvider, logger)
	}

	target := []byte(localPeerID)
	// 按配置决定“最终返回的候选数”，同时从路由表拉一个更大的候选池以支持 random/mixed 策略。
	selectionCount := 8
	strategy := "mixed"
	if configProvider != nil {
		if bc := configProvider.GetBlockchain(); bc != nil {
			if bc.Sync.Advanced.KBucketSelectionCount > 0 {
				selectionCount = bc.Sync.Advanced.KBucketSelectionCount
			}
			if s := strings.ToLower(strings.TrimSpace(bc.Sync.Advanced.KBucketSelectionStrategy)); s != "" {
				strategy = s
			}
		}
	}
	if selectionCount <= 0 {
		selectionCount = 8
	}
	if selectionCount > 32 {
		selectionCount = 32
	}

	// 候选池大小：越大越利于随机性，但也要控制开销。
	candidatePool := selectionCount * 4
	if candidatePool < 16 {
		candidatePool = 16
	}
	if candidatePool > 64 {
		candidatePool = 64
	}

	// ✅ vNext：优先选择“明确支持 SyncHelloV2”的 peer，避免后续 hello 失败造成抖动。
	// 该过滤必须是纯本地快路径（只读 peerstore 协议缓存），不得触发 DialPeer。
	var selectedPeers []peer.ID
	if rm, ok := routingManager.(*kbucketimpl.RoutingTableManager); ok {
		selectedPeers = rm.FindClosestPeersForProtocol(target, candidatePool, protocols.ProtocolSyncHelloV2)
	} else {
		selectedPeers = routingManager.FindClosestPeers(target, candidatePool)
	}
	if len(selectedPeers) == 0 {
		return buildPeersFromHint(ctx, localPeerID, p2pService, configProvider, logger)
	}

	var filteredPeers []peer.ID
	for _, pid := range selectedPeers {
		if pid != localPeerID && !IsBadPeer(pid) {
			// 🔥 过滤掉不健康的节点（熔断中）
			if !IsHealthy(pid) {
				if logger != nil {
					logger.Warnf("⚠️ 节点已熔断，跳过: %s", pid.String()[:12]+"...")
				}
				continue
			}
			filteredPeers = append(filteredPeers, pid)
		}
	}

	if len(filteredPeers) == 0 {
		if logger != nil {
			logger.Warn("⚠️ 过滤后没有可用节点（已排除 bad peers 和熔断节点）")
		}
		return buildPeersFromHint(ctx, localPeerID, p2pService, configProvider, logger)
	}

	// 根据策略生成最终候选列表：
	// - distance: 保持距离排序（由 FindClosestPeers 返回顺序决定），取前 N
	// - random: 在候选池中随机取 N
	// - mixed: 前半取 closest，后半从剩余候选随机补齐
	final := make([]peer.ID, 0, selectionCount)
	switch strategy {
	case "distance":
		if len(filteredPeers) > selectionCount {
			final = append(final, filteredPeers[:selectionCount]...)
		} else {
			final = append(final, filteredPeers...)
		}
	case "random":
		shufflePeersInPlace(filteredPeers)
		if len(filteredPeers) > selectionCount {
			final = append(final, filteredPeers[:selectionCount]...)
		} else {
			final = append(final, filteredPeers...)
		}
	case "mixed":
		fallthrough
	default:
		closestN := selectionCount / 2
		if closestN < 1 {
			closestN = 1
		}
		if closestN > selectionCount {
			closestN = selectionCount
		}
		if len(filteredPeers) < closestN {
			closestN = len(filteredPeers)
		}
		final = append(final, filteredPeers[:closestN]...)

		rest := append([]peer.ID(nil), filteredPeers[closestN:]...)
		shufflePeersInPlace(rest)
		need := selectionCount - len(final)
		if need > 0 {
			if len(rest) > need {
				final = append(final, rest[:need]...)
			} else {
				final = append(final, rest...)
			}
		}
	}

	// 防御：去重（虽然理论上 FindClosestPeers 不会返回重复）
	uniq := make([]peer.ID, 0, len(final))
	seen := make(map[peer.ID]struct{}, len(final))
	for _, pid := range final {
		if _, ok := seen[pid]; ok {
			continue
		}
		seen[pid] = struct{}{}
		uniq = append(uniq, pid)
	}
	final = uniq

	// 能够成功选到可用节点，说明网络状态已恢复，重置退避策略
	resetNoPeerBackoff()
	// 记录一个“可用上游”用于抖动时复用
	if len(final) > 0 {
		recordUpstreamSuccess(final[0])
	}

	if logger != nil {
		logger.Debugf("✅ K桶节点选择完成: strategy=%s, 候选池=%d, 最终=%d", strategy, len(filteredPeers), len(final))
		for i, pid := range final {
			if i >= 3 {
				break
			}
			logger.Debugf("  节点[%d]: %s", i+1, pid.String())
		}
	}

	return final, nil
}

// buildPeersFromHint 根据上下文中的 peer 提示构造同步目标
func buildPeersFromHint(ctx context.Context, localPeerID peer.ID, p2pService p2pi.Service, configProvider config.Provider, logger log.Logger) ([]peer.ID, error) {
	// 兜底选择也需要使用最新的 TTL/阈值
	applyUpstreamMemoryConfig(configProvider)

	if hint, ok := peerHintFromContext(ctx); ok && hint != "" && hint != localPeerID {
		if logger != nil {
			logger.Infof("🪢 使用上下文中的peer hint作为同步目标: %s", hint.String())
		}
		// 成功使用 hint，视为网络可用，重置退避
		resetNoPeerBackoff()
		recordUpstreamSuccess(hint)
		return []peer.ID{hint}, nil
	}

	// === 抗抖动：优先复用上一次成功的上游节点 ===
	if pid, ok := getLastGoodUpstream(localPeerID, p2pService); ok {
		if logger != nil {
			logger.Infof("🧷 K桶为空/不可用：复用上一次可用上游节点: %s", pid.String())
		}
		resetNoPeerBackoff()
		return []peer.ID{pid}, nil
	}

	// === 兜底策略：当 K 桶为空时，从“已连接 peers”里挑选上游 ===
	//
	// 目标：我们的核心目的是区块同步。K 桶为空在冷启动/过滤误判/事件时序等场景会持续较久，
	// 如果此处直接返回“无可用节点”，同步将永久 no-op，链高度不会收敛。
	//
	// 策略：
	// - 仅考虑已连接的 peer（Connectedness=Connected），避免无意义拨号；
	// - 过滤 bad peers/self；
	// - 优先选择声明了 WES 同步相关协议的 peer（peerstore protocols 中包含 /weisyn/.../sync/... 或 /weisyn/.../blockchain/...）。
	if peers := selectConnectedPeersForSync(localPeerID, p2pService, configProvider, logger); len(peers) > 0 {
		resetNoPeerBackoff()
		recordUpstreamSuccess(peers[0])
		return peers, nil
	}

	if logger != nil {
		// 使用指数退避控制日志频率，避免冷启动/网络抖动时期的刷屏
		now := time.Now()
		if shouldLogNoPeer(now) {
			logger.Warn("⚠️ 路由表中没有可用节点，且上下文未提供有效peer hint")
		} else {
			logger.Debug("⚠️ 路由表中没有可用节点（日志已按退避策略节流）")
		}
	}
	return nil, fmt.Errorf("路由表中没有可用节点")
}

// selectConnectedPeersForSync 从 libp2p 已连接 peers 中选择可作为上游的候选节点（K桶为空时的兜底）。
//
// 返回值：
// - 若找不到任何候选，返回空切片。
func selectConnectedPeersForSync(localPeerID peer.ID, p2pService p2pi.Service, configProvider config.Provider, logger log.Logger) []peer.ID {
	if localPeerID == "" || p2pService == nil || p2pService.Host() == nil {
		return nil
	}
	host := p2pService.Host()
	net := host.Network()
	if net == nil {
		return nil
	}
	list := net.Peers()
	if len(list) == 0 {
		return nil
	}

	// 为每个 peer 计算“同步候选分数”：必须支持 SyncHelloV2 协议（迁移期同时兼容 original/qualified）。
	type scored struct {
		id    peer.ID
		score int
	}
	scoredPeers := make([]scored, 0, len(list))

	ns := ""
	if configProvider != nil {
		ns = configProvider.GetNetworkNamespace()
	}
	wantHello := map[string]struct{}{protocols.ProtocolSyncHelloV2: {}}
	if ns != "" {
		wantHello[protocols.QualifyProtocol(protocols.ProtocolSyncHelloV2, ns)] = struct{}{}
	}

	for _, pid := range list {
		if pid == "" || pid == localPeerID || IsBadPeer(pid) {
			continue
		}
		// 必须是已连接
		//（Peers() 理论上都是 connected，但这里防御式校验）
		if net.Connectedness(pid) != libnetwork.Connected {
			continue
		}
		score := 0
		// vNext：严格要求支持 SyncHelloV2；否则该 peer 不具备“作为同步上游”的最低能力。
		if ps, err := host.Peerstore().GetProtocols(pid); err == nil && len(ps) > 0 {
			for _, p := range ps {
				if _, ok := wantHello[string(p)]; ok {
					score = 100
					break
				}
			}
		}
		if score > 0 {
			scoredPeers = append(scoredPeers, scored{id: pid, score: score})
		}
	}

	if len(scoredPeers) == 0 {
		return nil
	}

	sort.Slice(scoredPeers, func(i, j int) bool {
		if scoredPeers[i].score != scoredPeers[j].score {
			return scoredPeers[i].score > scoredPeers[j].score
		}
		return scoredPeers[i].id.String() < scoredPeers[j].id.String()
	})

	const maxPeers = 4
	out := make([]peer.ID, 0, maxPeers)
	for _, sp := range scoredPeers {
		out = append(out, sp.id)
		if len(out) >= maxPeers {
			break
		}
	}

	if logger != nil {
		logger.Infof("🛟 K桶为空：使用已连接 peers 作为同步上游候选: %d", len(out))
		for i, pid := range out {
			if i >= 3 {
				break
			}
			logger.Debugf("  fallback_peer[%d]=%s", i+1, pid.String())
		}
	}
	return out
}

// ======================= 低高度节点记录（SYNC-005/SYNC-101修复） =======================
//
// 背景：
// - 在阶段2的 SyncHelloV2 中，如果对端返回 REMOTE_BEHIND（对端高度低于本地），
//   或者观察到对端高度远低于权威网络高度，则将该节点标记为"低高度节点"。
// - 短期内（默认10分钟）不再选择该节点作为同步上游，避免重复低效同步。

var (
	lowHeightPeersMu    sync.RWMutex
	lowHeightPeers      = make(map[peer.ID]lowHeightInfo)
	lowHeightPeerTTL    = 10 * time.Minute
)

type lowHeightInfo struct {
	Height     uint64
	RecordedAt time.Time
}

// recordLowHeightPeer 记录一个低高度节点
func recordLowHeightPeer(pid peer.ID, height uint64, logger log.Logger) {
	if pid == "" {
		return
	}
	lowHeightPeersMu.Lock()
	lowHeightPeers[pid] = lowHeightInfo{
		Height:     height,
		RecordedAt: time.Now(),
	}
	lowHeightPeersMu.Unlock()
	
	if logger != nil {
		logger.Debugf("📝 记录低高度节点: peer=%s height=%d", 
			pid.String()[:12]+"...", height)
	}
}

// isLowHeightPeer 检查节点是否为低高度节点（在TTL内）
func isLowHeightPeer(pid peer.ID) bool {
	lowHeightPeersMu.RLock()
	info, exists := lowHeightPeers[pid]
	lowHeightPeersMu.RUnlock()

	if !exists {
		return false
	}

	// 检查TTL：过期则在写锁下清理（避免在RLock下delete导致并发问题）
	if time.Since(info.RecordedAt) > lowHeightPeerTTL {
		lowHeightPeersMu.Lock()
		// 二次确认，避免竞态
		if info2, ok := lowHeightPeers[pid]; ok {
			if time.Since(info2.RecordedAt) > lowHeightPeerTTL {
				delete(lowHeightPeers, pid)
			}
		}
		lowHeightPeersMu.Unlock()
		return false
	}

	return true
}

// clearExpiredLowHeightPeers 清理所有过期的低高度节点记录
// 🆕 SYNC-HIGH002修复：在无候选节点时调用，给过期节点第二次机会
func clearExpiredLowHeightPeers() {
	lowHeightPeersMu.Lock()
	defer lowHeightPeersMu.Unlock()

	now := time.Now()
	for pid, info := range lowHeightPeers {
		if now.Sub(info.RecordedAt) > lowHeightPeerTTL {
			delete(lowHeightPeers, pid)
		}
	}
}

// getLowHeightPeersStats 获取低高度节点统计信息
func getLowHeightPeersStats() (total int, expired int) {
	lowHeightPeersMu.RLock()
	defer lowHeightPeersMu.RUnlock()

	now := time.Now()
	for _, info := range lowHeightPeers {
		total++
		if now.Sub(info.RecordedAt) > lowHeightPeerTTL {
			expired++
		}
	}
	return
}

// reduceLowHeightPeerTTL 临时缩短低高度节点 TTL（紧急恢复）
// 🆕 SYNC-HIGH002修复：在极端情况下加速节点恢复
func reduceLowHeightPeerTTL(factor float64) {
	if factor <= 0 || factor >= 1 {
		return
	}
	lowHeightPeersMu.Lock()
	defer lowHeightPeersMu.Unlock()

	// 临时缩短 TTL，让更多节点有机会被重试
	reducedTTL := time.Duration(float64(lowHeightPeerTTL) * factor)
	now := time.Now()

	for pid, info := range lowHeightPeers {
		if now.Sub(info.RecordedAt) > reducedTTL {
			delete(lowHeightPeers, pid)
		}
	}
}

// ======================= 带降级策略的节点选择（熔断+Fallback） =======================

// filterHealthyPeers 过滤健康的节点（排除熔断中的节点）
func filterHealthyPeers(peers []peer.ID, logger log.Logger) []peer.ID {
	healthy := make([]peer.ID, 0, len(peers))
	for _, pid := range peers {
		if IsHealthy(pid) {
			healthy = append(healthy, pid)
		} else {
			if logger != nil {
				logger.Debugf("⚠️ 节点已熔断，跳过: %s", pid.String()[:12]+"...")
			}
		}
	}
	return healthy
}

// selectRandomPeers 从列表中随机选择最多 n 个节点
func selectRandomPeers(peers []peer.ID, n int) []peer.ID {
	if len(peers) <= n {
		return peers
	}
	
	// 复制一份避免修改原始列表
	copied := make([]peer.ID, len(peers))
	copy(copied, peers)
	
	// 随机打乱
	shufflePeersInPlace(copied)
	
	return copied[:n]
}

// getBootstrapPeers 获取 Bootstrap 节点列表（从配置中读取）
func getBootstrapPeers(configProvider config.Provider) []peer.ID {
	if configProvider == nil {
		return nil
	}
	
	nodeConfig := configProvider.GetNode()
	if nodeConfig == nil || len(nodeConfig.Discovery.BootstrapPeers) == 0 {
		return nil
	}
	
	var bootstrapPeers []peer.ID
	for _, addrStr := range nodeConfig.Discovery.BootstrapPeers {
		// ✅ public bootstrap（bootstrap.libp2p.io 等）仅用于 discovery/连通性，不应作为“区块同步上游”
		// 保留在配置里，但从同步候选中剔除。
		if strings.Contains(addrStr, "bootstrap.libp2p.io") || strings.Contains(addrStr, "ipfs") {
			continue
		}
		// 尝试从 multiaddr 中提取 peer ID
		// 格式如: /ip4/1.2.3.4/tcp/5000/p2p/QmXXX
		parts := strings.Split(addrStr, "/p2p/")
		if len(parts) == 2 {
			if pid, err := peer.Decode(parts[1]); err == nil {
				bootstrapPeers = append(bootstrapPeers, pid)
			}
		}
	}
	
	return bootstrapPeers
}

// selectCandidatePeersWithFallback 带降级策略的节点选择
//
// 降级策略：
//   1. 优先使用 K桶节点（已过滤熔断节点）
//   2. 如果 K桶无可用节点，降级到 DHT 已连接节点
//   3. 如果 DHT 节点也不可用，尝试 Bootstrap 节点
func selectCandidatePeersWithFallback(
	ctx context.Context,
	routingManager kademlia.RoutingTableManager,
	p2pService p2pi.Service,
	configProvider config.Provider,
	chainInfo *types.ChainInfo,
	logger log.Logger,
) ([]peer.ID, error) {
	if logger != nil {
		logger.Debug("🔍 开始带降级策略的节点选择")
	}

	localPeerID := peer.ID("")
	if p2pService != nil && p2pService.Host() != nil {
		localPeerID = p2pService.Host().ID()
	}

	// 阶段1：尝试 K桶节点（最优）
	candidates, err := selectKBucketPeersForSync(ctx, routingManager, p2pService, configProvider, chainInfo, logger)
	if err == nil && len(candidates) > 0 {
		// K桶节点选择成功，已经过滤了熔断节点
		healthyCandidates := filterHealthyPeers(candidates, logger)
		if len(healthyCandidates) > 0 {
			if logger != nil {
				logger.Infof("✅ K桶节点可用: %d 个", len(healthyCandidates))
			}
			return healthyCandidates, nil
		}
	}

	// 阶段2：K桶节点全部不可用，降级到 DHT 已连接节点
	if logger != nil {
		logger.Warn("⚠️ K桶节点全部不可用，降级到DHT已连接节点")
	}

	if p2pService != nil && p2pService.Host() != nil {
		host := p2pService.Host()
		net := host.Network()
		if net != nil {
			connectedPeers := net.Peers()
			
			// 过滤：排除自己、bad peers、熔断节点
			var validPeers []peer.ID
			for _, pid := range connectedPeers {
				if pid == "" || pid == localPeerID || IsBadPeer(pid) {
					continue
				}
				if !IsHealthy(pid) {
					continue
				}
				validPeers = append(validPeers, pid)
			}

			if len(validPeers) > 0 {
				if logger != nil {
					logger.Infof("✅ DHT已连接节点可用: %d 个", len(validPeers))
				}
				// 随机选择最多 8 个节点
				k := 8
				if configProvider != nil {
					if bc := configProvider.GetBlockchain(); bc != nil {
						if bc.Sync.Advanced.KBucketSelectionCount > 0 {
							k = bc.Sync.Advanced.KBucketSelectionCount
						}
					}
				}
				return selectRandomPeers(validPeers, k), nil
			}
		}
	}

	// 阶段3：连接节点也不可用，尝试 Bootstrap 节点
	if logger != nil {
		logger.Warn("⚠️ DHT节点也不可用，尝试Bootstrap节点")
	}

	bootstrapPeers := getBootstrapPeers(configProvider)
	if len(bootstrapPeers) > 0 {
		// 过滤健康的 Bootstrap 节点
		healthyBootstrap := filterHealthyPeers(bootstrapPeers, logger)
		if len(healthyBootstrap) > 0 {
			if logger != nil {
				logger.Infof("✅ Bootstrap节点可用: %d 个", len(healthyBootstrap))
			}
			return healthyBootstrap, nil
		}
	}

	return nil, fmt.Errorf("没有可用的同步节点（K桶、DHT、Bootstrap均不可用）")
}
