package quorum

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/protobuf/proto"

	"github.com/weisyn/v1/internal/config/node"
	chainsync "github.com/weisyn/v1/internal/core/chain/sync"
	netpb "github.com/weisyn/v1/pb/network/protocol"
	"github.com/weisyn/v1/pkg/constants/protocols"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/network"
	"github.com/weisyn/v1/pkg/interfaces/p2p"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/types"
)

// Checker 执行 V2 挖矿稳定性门闸检查（网络法定人数 + 高度一致性 + 链尖前置）。
type Checker interface {
	Check(ctx context.Context) (*Result, error)
}

type checker struct {
	cfgProvider config.Provider
	minerCfg    MinerConfigView

	chainQuery persistence.ChainQuery
	querySvc   persistence.QueryService

	routing    kademlia.RoutingTableManager
	p2pService p2p.Service
	netService network.Network

	logger log.Logger

	mu              sync.Mutex
	discoveryStart  time.Time
	quorumReachedAt time.Time
	
	helloSemaphore chan struct{} // 限制并发 hello 数量（worker pool）
}

// NewChecker 创建一个 QuorumChecker（作为 miner 的子组件）。
func NewChecker(
	cfgProvider config.Provider,
	minerCfg MinerConfigView,
	chainQuery persistence.ChainQuery,
	querySvc persistence.QueryService,
	routing kademlia.RoutingTableManager,
	p2pService p2p.Service,
	netService network.Network,
	logger log.Logger,
) Checker {
	return &checker{
		cfgProvider:    cfgProvider,
		minerCfg:       minerCfg,
		chainQuery:     chainQuery,
		querySvc:       querySvc,
		routing:        routing,
		p2pService:     p2pService,
		netService:     netService,
		logger:         logger,
		helloSemaphore: make(chan struct{}, 10), // 默认 10 并发度
	}
}

func (c *checker) ensureStartTimesLocked(now time.Time) {
	if c.discoveryStart.IsZero() {
		c.discoveryStart = now
	}
}

func (c *checker) collectChainTipPrereq(ctx context.Context, localHeight uint64) ChainTipPrerequisite {
	pr := ChainTipPrerequisite{
		TipReadable:            true,
		TipTimestamp:           0,
		TipAge:                 0,
		TipFresh:               true,
		TipHealthyForHandshake: true,
	}

	// height=0：创世不做新鲜度门闸
	if localHeight == 0 {
		return pr
	}

	// tip 可读性：需要能读取到 tip block
	if c.querySvc == nil {
		pr.TipReadable = false
		pr.TipHealthyForHandshake = false
		return pr
	}
	blk, err := c.querySvc.GetBlockByHeight(ctx, localHeight)
	if err != nil || blk == nil || blk.Header == nil {
		pr.TipReadable = false
		pr.TipHealthyForHandshake = false
		return pr
	}

	pr.TipTimestamp = blk.Header.Timestamp
	now := time.Now()
	if pr.TipTimestamp > 0 {
		pr.TipAge = now.Sub(time.Unix(int64(pr.TipTimestamp), 0))
	}

	if c.minerCfg != nil && c.minerCfg.GetEnableTipFreshnessCheck() {
		maxStale := time.Duration(c.minerCfg.GetMaxTipStalenessSeconds()) * time.Second
		if maxStale > 0 && pr.TipAge > maxStale {
			pr.TipFresh = false
		}
	}

	pr.TipHealthyForHandshake = pr.TipReadable && (!c.minerCfg.GetEnableTipFreshnessCheck() || pr.TipFresh)
	return pr
}

func (c *checker) getConnectedPeers() []peer.ID {
	if c.p2pService == nil || c.p2pService.Host() == nil || c.p2pService.Host().Network() == nil {
		return nil
	}
	return c.p2pService.Host().Network().Peers()
}

func (c *checker) getDiscoveredPeersCount() int {
	if c.routing == nil {
		return 0
	}
	rt := c.routing.GetRoutingTable()
	if rt == nil {
		return 0
	}
	return rt.TableSize
}

func parseUint64(s string) (uint64, error) {
	var v uint64
	_, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &v)
	return v, err
}

func median(values []uint64) uint64 {
	if len(values) == 0 {
		return 0
	}
	tmp := make([]uint64, 0, len(values))
	tmp = append(tmp, values...)
	sort.Slice(tmp, func(i, j int) bool { return tmp[i] < tmp[j] })
	return tmp[len(tmp)/2]
}

func (c *checker) helloV2Height(ctx context.Context, target peer.ID, localTipHeight uint64, localTipHash []byte, locatorBytes []byte) (uint64, error) {
	if c.netService == nil {
		return 0, fmt.Errorf("network service unavailable")
	}
	if len(localTipHash) != 32 {
		return 0, fmt.Errorf("local tip hash invalid (len=%d)", len(localTipHash))
	}

	localIdentity, err := chainsync.GetLocalChainIdentity(ctx, c.cfgProvider, c.querySvc)
	if err != nil {
		return 0, fmt.Errorf("get local chain identity failed: %w", err)
	}
	if !localIdentity.IsValid() {
		return 0, fmt.Errorf("local chain identity invalid")
	}

	req := &netpb.KBucketSyncRequest{
		RequestId:       fmt.Sprintf("mining-quorum-hello-%d", time.Now().UnixNano()),
		LocalHeight:     localTipHeight,
		RoutingKey:      localTipHash,
		MaxResponseSize: 256 * 1024,
		RequesterPeerId: locatorBytes,
		TargetHeight:    nil,
	}
	req.ChainIdentity = node.ToProtoChainIdentity(localIdentity)
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return 0, fmt.Errorf("marshal hello request failed: %w", err)
	}

	ok, _ := c.netService.CheckProtocolSupport(ctx, target, protocols.ProtocolSyncHelloV2)
	if !ok {
		return 0, fmt.Errorf("peer does not support %s", protocols.ProtocolSyncHelloV2)
	}

	respBytes, err := c.netService.Call(ctx, target, protocols.ProtocolSyncHelloV2, reqBytes, &types.TransportOptions{
		ConnectTimeout: 8 * time.Second,
		WriteTimeout:   8 * time.Second,
		ReadTimeout:    12 * time.Second,
		MaxRetries:     0,
	})
	if err != nil {
		return 0, fmt.Errorf("hello v2 call failed: %w", err)
	}

	resp := &netpb.IntelligentPaginationResponse{}
	if err := proto.Unmarshal(respBytes, resp); err != nil {
		return 0, fmt.Errorf("unmarshal hello v2 response failed: %w", err)
	}
	if !resp.Success {
		msg := ""
		if resp.ErrorMessage != nil {
			msg = *resp.ErrorMessage
		}
		return 0, fmt.Errorf("hello v2 rejected: %s", msg)
	}
	if resp.ChainIdentity == nil {
		return 0, fmt.Errorf("hello v2 missing chain_identity")
	}
	remoteIdentity := node.FromProtoChainIdentity(resp.ChainIdentity)
	if !remoteIdentity.IsValid() || !localIdentity.IsSameChain(remoteIdentity) {
		return 0, fmt.Errorf("hello v2 chain identity mismatch: remote=%v local=%v", remoteIdentity, localIdentity)
	}

	remoteTip := resp.NextHeight
	if strings.Contains(resp.PaginationReason, "remote_tip=") {
		if idx := strings.Index(resp.PaginationReason, "remote_tip="); idx >= 0 {
			sub := resp.PaginationReason[idx+len("remote_tip="):]
			if end := strings.IndexByte(sub, ' '); end > 0 {
				sub = sub[:end]
			}
			if v, perr := parseUint64(sub); perr == nil && v > 0 {
				remoteTip = v
			}
		}
	}
	if remoteTip == 0 {
		return 0, fmt.Errorf("hello v2 returned remote tip height=0")
	}
	return remoteTip, nil
}

func (c *checker) Check(ctx context.Context) (*Result, error) {
	// V2 配置开关：如果禁用了网络对齐检查，直接允许挖矿（向后兼容）
	if c.minerCfg != nil && !c.minerCfg.GetEnableNetworkAlignmentCheck() {
		return &Result{
			State:           StateHeightAligned,
			AllowMining:     true,
			Reason:          "网络对齐检查已禁用（enable_network_alignment_check=false）",
			SuggestedAction: "",
		}, nil
	}

	now := time.Now()
	c.mu.Lock()
	c.ensureStartTimesLocked(now)
	discoveryStart := c.discoveryStart
	quorumReachedAt := c.quorumReachedAt
	c.mu.Unlock()

	if ctx == nil {
		ctx = context.Background()
	}

	// 1) 本地高度与 tip hash
	if c.chainQuery == nil {
		return nil, fmt.Errorf("chainQuery is nil")
	}
	chainInfo, err := c.chainQuery.GetChainInfo(ctx)
	if err != nil || chainInfo == nil {
		return nil, fmt.Errorf("get chain info failed: %w", err)
	}
	localHeight := chainInfo.Height
	localTipHash := chainInfo.BestBlockHash
	if len(localTipHash) != 32 {
		res := &Result{
			State:           StateNotStarted,
			AllowMining:     false,
			Reason:          "链尖哈希不可用，拒绝网络确认/挖矿",
			SuggestedAction: "repair",
		}
		res.Metrics.LocalHeight = localHeight
		res.ChainTip = ChainTipPrerequisite{TipReadable: false, TipHealthyForHandshake: false}
		c.updatePrometheusMetrics(res)
		return res, nil
	}

	// 2) 链尖前置条件
	// V2 设计：tip_stale 不应直接阻止挖矿，除非同时存在 HeightConflict
	// - tip_readable=false：必须阻止（链尖损坏）
	// - tip_stale：仅作为“触发同步检查”的信号，不直接阻止挖矿
	tipPrereq := c.collectChainTipPrereq(ctx, localHeight)
	if !tipPrereq.TipReadable {
		res := &Result{
			State:           StateNotStarted,
			AllowMining:     false,
			Reason:          "链尖不可读，禁止挖矿（请先修复）",
			SuggestedAction: "repair",
			ChainTip:        tipPrereq,
			Metrics: Metrics{
				LocalHeight:        localHeight,
				DiscoveryStartedAt: discoveryStart,
			},
		}
		c.updatePrometheusMetrics(res)
		return res, nil
	}
	// tip_stale 不在这里阻止挖矿，将在后续高度一致性检查中处理

	// 3) peers 指标
	connected := c.getConnectedPeers()
	connectedPeers := len(connected)
	discoveredPeers := c.getDiscoveredPeersCount()

	requiredTotal := 2
	allowSingle := false
	discoveryTimeout := 120 * time.Second
	quorumRecoveryTimeout := 300 * time.Second
	maxHeightSkew := uint64(5)
	if c.minerCfg != nil {
		if v := c.minerCfg.GetMinNetworkQuorumTotal(); v > 0 {
			requiredTotal = v
		}
		allowSingle = c.minerCfg.GetAllowSingleNodeMining()
		if s := c.minerCfg.GetNetworkDiscoveryTimeoutSeconds(); s > 0 {
			discoveryTimeout = time.Duration(s) * time.Second
		}
		if s := c.minerCfg.GetQuorumRecoveryTimeoutSeconds(); s > 0 {
			quorumRecoveryTimeout = time.Duration(s) * time.Second
		}
		if v := c.minerCfg.GetMaxHeightSkew(); v > 0 {
			maxHeightSkew = v
		}
	}

	peerHeights := make(map[peer.ID]uint64)
	// 4) 高度交换：对 connected peers 并发进行 hello v2（使用 worker pool + semaphore）
	var peerHeightsMu sync.Mutex
	var wg sync.WaitGroup
	
	for _, pid := range connected {
		if pid == "" {
			continue
		}
		
		pid := pid // 捕获循环变量
		wg.Add(1)
		
		go func() {
			defer wg.Done()
			
			// 获取 semaphore 令牌（限制并发度）
			select {
			case c.helloSemaphore <- struct{}{}:
				defer func() { <-c.helloSemaphore }()
			case <-ctx.Done():
				return
			}
			
			// 执行 hello v2
			peerCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			h, herr := c.helloV2Height(peerCtx, pid, localHeight, localTipHash, nil)
			cancel()
			
			if herr != nil {
				return
			}
			
			// 安全地写入 peerHeights map
			peerHeightsMu.Lock()
			peerHeights[pid] = h
			peerHeightsMu.Unlock()
		}()
	}
	
	// 等待所有 hello 完成
	wg.Wait()

	qualified := len(peerHeights)
	currentTotal := qualified + 1 // 含本机

	res := &Result{
		State:           StateDiscovering,
		AllowMining:     false,
		Reason:          "",
		SuggestedAction: "",
		ChainTip:        tipPrereq,
		Metrics: Metrics{
			DiscoveredPeers:     discoveredPeers,
			ConnectedPeers:      connectedPeers,
			QualifiedPeers:      qualified,
			RequiredQuorumTotal: requiredTotal,
			CurrentQuorumTotal:  currentTotal,
			QuorumReached:       currentTotal >= requiredTotal,
			LocalHeight:         localHeight,
			PeerHeights:         peerHeights,
			DiscoveryStartedAt:  discoveryStart,
			QuorumReachedAt:     quorumReachedAt,
		},
	}

	// 5) 状态机/决策
	if currentTotal < requiredTotal {
		if now.Sub(discoveryStart) > discoveryTimeout {
			res.State = StateIsolated
			if allowSingle {
			res.AllowMining = true
			res.Reason = "⚠️ 单节点测试模式（allow_single_node_mining=true），允许挖矿"
			res.SuggestedAction = "single_node_warning"
			c.updatePrometheusMetrics(res)
			return res, nil
			}
		res.AllowMining = false
		res.Reason = fmt.Sprintf("网络法定人数不足（当前=%d 需要=%d），且发现超时进入孤岛状态", currentTotal, requiredTotal)
		res.SuggestedAction = "check_network"
		c.updatePrometheusMetrics(res)
		return res, nil
		}
		if discoveredPeers > 0 || connectedPeers > 0 {
			res.State = StateQuorumPending
			res.Reason = fmt.Sprintf("网络法定人数不足（当前=%d 需要=%d），等待更多节点加入/完成握手", currentTotal, requiredTotal)
		} else {
			res.State = StateDiscovering
			res.Reason = "正在进行网络发现"
		}
		res.SuggestedAction = "wait"
		c.updatePrometheusMetrics(res)
		return res, nil
	}

	// 达到法定人数：记录达标时间
	if quorumReachedAt.IsZero() {
		c.mu.Lock()
		if c.quorumReachedAt.IsZero() {
			c.quorumReachedAt = now
		}
		res.Metrics.QuorumReachedAt = c.quorumReachedAt
		c.mu.Unlock()
	} else {
		res.Metrics.QuorumReachedAt = quorumReachedAt
	}

	// 6) 高度一致性
	values := make([]uint64, 0, len(peerHeights))
	for _, h := range peerHeights {
		values = append(values, h)
	}
	med := median(values)
	
	// ✅ 防御逻辑：处理 median=0 的异常情况
	// 当 peerHeights 为空或所有peer高度都是0时，median会返回0
	// 这会导致挖矿被永久阻止（skew = localHeight - 0 会非常大）
	if med == 0 && localHeight > 0 {
		if len(values) == 0 {
			// 情况1：无有效peer高度（K桶为空或所有握手失败）
			if c.logger != nil {
				c.logger.Warnf("⚠️ 挖矿门闸：无有效peer高度，median=0，使用本地高度作为fallback (local=%d)", localHeight)
			}
			med = localHeight
		} else {
			// 情况2：所有peer高度都是0但本地不是0（peer可能刚启动或同步中）
			if c.logger != nil {
				c.logger.Warnf("⚠️ 挖矿门闸：所有peer高度为0，本地高度=%d，使用本地高度作为median", localHeight)
			}
			med = localHeight
		}
	}
	
	// ✅ 告警：本地高度远高于median（可能是网络分区）
	if med > 0 && localHeight > med && (localHeight - med) > 100 {
		if c.logger != nil {
			c.logger.Warnf("⚠️ 挖矿门闸：本地高度(%d)远高于median(%d)，可能是网络分区或本地链孤立", 
				localHeight, med)
		}
		// 仍使用median进行后续判断，但记录告警
	}
	
	res.Metrics.MedianPeerHeight = med
	res.Metrics.HeightSkew = int64(localHeight) - int64(med)

	threshold := maxHeightSkew

	if localHeight == 0 {
		for _, h := range peerHeights {
			if h > 1 {
				res.State = StateHeightConflict
				res.AllowMining = false
				res.Reason = fmt.Sprintf("本地处于创世高度，但发现 peer 高度>1（peer_height=%d），禁止挖矿并应优先同步", h)
				res.SuggestedAction = "sync"
				c.updatePrometheusMetrics(res)
				return res, nil
			}
		}
		res.State = StateHeightAligned
		res.AllowMining = true
		res.Reason = fmt.Sprintf("网络法定人数达标(%d/%d)且创世高度一致", currentTotal, requiredTotal)
		c.updatePrometheusMetrics(res)
		return res, nil
	}

	absSkew := res.Metrics.HeightSkew
	if absSkew < 0 {
		absSkew = -absSkew
	}
	
	// V2 设计：tip_stale + HeightConflict 时才阻止挖矿
	// 如果 tip_stale 但高度一致，允许挖矿（避免全网自锁）
	if uint64(absSkew) <= threshold {
		// 高度一致：允许挖矿
		// 如果 tip_stale，在 reason 中提示但不阻止
		reason := fmt.Sprintf("网络法定人数达标(%d/%d)且高度一致（local=%d median=%d skew=%d max_height_skew=%d）",
			currentTotal, requiredTotal, localHeight, med, res.Metrics.HeightSkew, threshold)
		if !tipPrereq.TipFresh && c.minerCfg != nil && c.minerCfg.GetEnableTipFreshnessCheck() {
			reason += fmt.Sprintf("（链尖已过期 %v，建议同步检查）", tipPrereq.TipAge)
		}
		res.State = StateHeightAligned
		res.AllowMining = true
		res.Reason = reason
		c.updatePrometheusMetrics(res)
		return res, nil
	}

	if now.Sub(res.Metrics.QuorumReachedAt) < quorumRecoveryTimeout {
		res.State = StateQuorumReached
		res.AllowMining = false
		res.Reason = fmt.Sprintf("法定人数达标但高度尚未对齐（local=%d median=%d skew=%d max_height_skew=%d），等待同步收敛",
			localHeight, med, res.Metrics.HeightSkew, threshold)
		res.SuggestedAction = "sync"
		c.updatePrometheusMetrics(res)
		return res, nil
	}

	// 高度不一致：根据冲突持续时间决定是否启用降级策略
	reason := fmt.Sprintf("高度冲突（local=%d median=%d skew=%d max_height_skew=%d）",
		localHeight, med, res.Metrics.HeightSkew, threshold)
	if !tipPrereq.TipFresh && c.minerCfg != nil && c.minerCfg.GetEnableTipFreshnessCheck() {
		reason += fmt.Sprintf("（同时链尖已过期 %v）", tipPrereq.TipAge)
	}
	
	// ✅ 降级策略：如果高度冲突持续超过30分钟，允许挖矿但标记需要人工检查
	// 目的：避免在网络分区或同步问题时永久禁止挖矿，导致节点完全停摆
	conflictDuration := now.Sub(res.Metrics.QuorumReachedAt)
	const degradationThreshold = 30 * time.Minute
	
	if conflictDuration > degradationThreshold {
		// 启用降级策略：允许挖矿但发出严重警告
		if c.logger != nil {
			c.logger.Warnf("⚠️ 挖矿门闸：高度冲突已持续%v（超过阈值%v），启用降级策略允许挖矿", 
				conflictDuration, degradationThreshold)
			c.logger.Warnf("⚠️ 建议人工检查：1) 网络连接状态 2) peer同步进度 3) 是否存在分叉")
		}
		res.State = StateHeightAligned // 使用 Aligned 状态但在 reason 中说明是降级
		res.AllowMining = true
		res.Reason = reason + fmt.Sprintf("（冲突持续%v，降级允许挖矿，需人工检查）", conflictDuration)
		res.SuggestedAction = "manual_check_required"
	} else {
		// 正常策略：禁止挖矿并触发同步
		res.State = StateHeightConflict
		res.AllowMining = false
		res.Reason = reason + "，禁止挖矿并触发同步/排查"
		res.SuggestedAction = "sync"
	}
	
	// 更新 Prometheus 指标
	c.updatePrometheusMetrics(res)
	
	return res, nil
}

// updatePrometheusMetrics 更新 Prometheus 指标
func (c *checker) updatePrometheusMetrics(res *Result) {
	if res == nil {
		return
	}
	
	// 获取 nodeID（用于 Prometheus label）
	nodeID := "unknown"
	if c.p2pService != nil && c.p2pService.Host() != nil {
		nodeID = c.p2pService.Host().ID().String()
	}
	
	updateMetrics(nodeID, res)
}


