package sync

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	peer "github.com/libp2p/go-libp2p/core/peer"

	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/network"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
)

// ProbeDecision 表示“轻量探针”输出的决策结果：
// - 不下载区块、不申请同步锁；
// - 仅通过 hello/高度采样判断：是否需要进入 full sync，以及建议的上游 peer hint。
type ProbeDecision struct {
	ShouldFullSync bool
	Reason         string

	// NetworkTip 是探针观测到的“网络链尖高度”估计值（取样本中最大 remote_tip）。
	NetworkTip uint64

	// HintPeer 是建议作为上游的 peer（用于 ContextWithPeerHint），为空表示不建议绑定。
	HintPeer peer.ID

	// ForkDetected 表示探针发现了同高度 hash 不一致或 locator 反查共同祖先的分叉迹象。
	ForkDetected bool

	// Stats 用于观测/调试
	SampledPeers int
	HelloSuccess int
}

func pickRandomPeer(peers []peer.ID) peer.ID {
	if len(peers) == 0 {
		return ""
	}
	if len(peers) == 1 {
		return peers[0]
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return peers[r.Intn(len(peers))]
}

func computeLocalTipHashForProbe(
	ctx context.Context,
	queryService persistence.QueryService,
	blockHashClient core.BlockHashServiceClient,
	localHeight uint64,
	fallbackBestHash []byte,
	logger log.Logger,
) []byte {
	localTipHash := fallbackBestHash
	// 尽量按 “高度对应的真实区块 hash” 计算，避免 BestBlockHash 与高度不一致导致 hello 误判 fork。
	if queryService != nil && blockHashClient != nil {
		if blk, err := queryService.GetBlockByHeight(ctx, localHeight); err == nil && blk != nil && blk.Header != nil {
			if resp, err := blockHashClient.ComputeBlockHash(ctx, &core.ComputeBlockHashRequest{Block: blk}); err == nil && resp != nil && resp.IsValid && len(resp.Hash) == 32 {
				localTipHash = resp.Hash
			} else if logger != nil {
				logger.Debugf("[Probe] 计算本地 tip_hash 失败（回退到 chainInfo.BestBlockHash）：height=%d err=%v", localHeight, err)
			}
		} else if logger != nil {
			logger.Debugf("[Probe] 读取本地 tip 区块失败（回退到 chainInfo.BestBlockHash）：height=%d err=%v", localHeight, err)
		}
	}
	return localTipHash
}

// probeSyncImpl 执行轻量探针：
// - 使用 selectKBucketPeersForSync 选择候选；
// - 对候选执行 SyncHelloV2（可带 locator）；
// - 输出：是否需要 full sync、建议 peer hint、网络高度估计。
func probeSyncImpl(
	ctx context.Context,
	chainQuery persistence.ChainQuery,
	queryService persistence.QueryService,
	routingManager kademlia.RoutingTableManager,
	networkService network.Network,
	p2pService p2pi.Service,
	configProvider config.Provider,
	blockHashClient core.BlockHashServiceClient,
	logger log.Logger,
) (ProbeDecision, error) {
	// 同步进行中时：探针无意义且会增加网络噪音
	if hasActiveSyncTask() {
		return ProbeDecision{ShouldFullSync: false, Reason: "sync_in_progress"}, nil
	}

	chainInfo, err := chainQuery.GetChainInfo(ctx)
	if err != nil {
		return ProbeDecision{}, fmt.Errorf("probe: get chain info failed: %w", err)
	}
	if chainInfo == nil {
		return ProbeDecision{}, fmt.Errorf("probe: chain info is nil")
	}
	localHeight := chainInfo.Height

	candidates, err := selectKBucketPeersForSync(ctx, routingManager, p2pService, configProvider, chainInfo, logger)
	if err != nil || len(candidates) == 0 {
		// 没有上游：探针无法进行；这类场景 full sync 也无法做事，返回一个“无决策”的结果即可。
		return ProbeDecision{ShouldFullSync: false, Reason: "no_candidates"}, nil
	}

	// 限制 hello 探针的样本量，避免大网环境下做无谓扫描
	maxHello := 3
	if configProvider != nil {
		if bc := configProvider.GetBlockchain(); bc != nil {
			if bc.Sync.Advanced.MaxConcurrentRequests > 0 {
				// 复用该配置作为“探针并发/样本上限”的近似（但我们仍采用串行 hello，降低网络瞬时峰值）
				maxHello = bc.Sync.Advanced.MaxConcurrentRequests
			}
		}
	}
	if maxHello < 1 {
		maxHello = 1
	}
	if maxHello > 5 {
		maxHello = 5
	}
	if len(candidates) > maxHello {
		candidates = candidates[:maxHello]
	}

	// tip hash + locator：用于 fork-aware hello 判定
	localTipHash := computeLocalTipHashForProbe(ctx, queryService, blockHashClient, localHeight, chainInfo.BestBlockHash, logger)
	if len(localTipHash) != 32 {
		// 没有合法 tip_hash 时，探针降级为“只做高度采样”（不做 hello）
		h, _, e := queryNetworkHeightFromCandidates(ctx, candidates, networkService, p2pService, chainInfo, configProvider, logger)
		if e != nil {
			return ProbeDecision{ShouldFullSync: false, Reason: "no_tip_hash_and_height_query_failed"}, nil
		}
		// 高度领先才建议 full sync
		if h > localHeight {
			return ProbeDecision{ShouldFullSync: true, Reason: "network_ahead_height_only", NetworkTip: h}, nil
		}
		return ProbeDecision{ShouldFullSync: false, Reason: "up_to_date_height_only", NetworkTip: h}, nil
	}

	var locatorBytes []byte
	if queryService != nil && blockHashClient != nil {
		if b, err := BuildBlockLocatorBinary(ctx, queryService, blockHashClient, localHeight, 16, configProvider); err == nil {
			locatorBytes = b
		}
	}

	decision := ProbeDecision{
		ShouldFullSync: false,
		Reason:         "no_signal",
		NetworkTip:     localHeight,
		SampledPeers:   len(candidates),
		HelloSuccess:   0,
	}

	var aheadPeers []peer.ID
	var forkPeers []peer.ID
	var bestAheadPeer peer.ID
	var bestAheadHeight uint64

	for _, pid := range candidates {
		info, err := performSyncHelloV2(
			ctx,
			pid,
			localHeight,
			localTipHash,
			locatorBytes,
			chainInfo,
			networkService,
			p2pService,
			configProvider,
			logger,
		)
		if err != nil || info == nil {
			continue
		}
		decision.HelloSuccess++
		if info.remoteTipHeight > decision.NetworkTip {
			decision.NetworkTip = info.remoteTipHeight
		}

		switch info.relationship {
		case "REMOTE_AHEAD_SAME_CHAIN":
			aheadPeers = append(aheadPeers, pid)
			if info.remoteTipHeight > bestAheadHeight {
				bestAheadHeight = info.remoteTipHeight
				bestAheadPeer = pid
			}
		case "FORK_DETECTED":
			decision.ForkDetected = true
			forkPeers = append(forkPeers, pid)
		case "UP_TO_DATE":
			// no-op
		case "REMOTE_BEHIND":
			// no-op
		default:
			// UNKNOWN 等：不作为决策依据
		}
	}

	// 决策优先级：
	// 1) 发现网络领先（同链）→ 需要 full sync，优先选择“最高 tip”的 peer 作为 hint
	// 2) 发现 fork → 需要进入 full sync（触发 fork-aware reorg）；hint 从 forkPeers 中随机挑一个（避免绑定固定节点）
	// 3) 否则认为无需 full sync
	if len(aheadPeers) > 0 && bestAheadPeer != "" {
		decision.ShouldFullSync = true
		decision.Reason = "remote_ahead_same_chain"
		decision.HintPeer = bestAheadPeer
		return decision, nil
	}
	if decision.ForkDetected && len(forkPeers) > 0 {
		decision.ShouldFullSync = true
		decision.Reason = "fork_detected"
		decision.HintPeer = pickRandomPeer(forkPeers)
		return decision, nil
	}

	decision.ShouldFullSync = false
	decision.Reason = "up_to_date_or_no_actionable_signal"
	return decision, nil
}
