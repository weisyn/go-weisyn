// trigger.go - 同步触发主入口
// 负责协调3阶段同步流程：同步触发与节点选择、K桶智能同步、分页补齐同步
// - 使用K桶算法选择最近邻节点
// - 查询网络高度并执行智能同步
// - 处理区块验证和应用流程
package sync

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	peer "github.com/libp2p/go-libp2p/core/peer"

	"github.com/weisyn/v1/internal/core/chain/interfaces"
	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/constants/events"
	"github.com/weisyn/v1/pkg/interfaces/block"
	"github.com/weisyn/v1/pkg/interfaces/config"
	eventiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/network"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/types"
	corruptutil "github.com/weisyn/v1/pkg/utils/corruption"
)

// ============================================================================
//                              主要同步入口
// ============================================================================
//
// 注意：
// - 同步状态管理已移至 sync_state.go
// - K桶节点选择已移至 node_selection.go
// - 网络高度查询已移至 height_query.go
// - 区块同步逻辑已移至 block_sync.go
// - 数据结构使用pb/blockchain/block/block.proto统一格式

// ============================================================================
//                           重试策略辅助函数（SYNC-203修复）
// ============================================================================

// retryCountKey 用于在上下文中存储重试计数
type retryCountKey struct{}

// withRetryCount 将重试计数添加到上下文
func withRetryCount(ctx context.Context, count int) context.Context {
	return context.WithValue(ctx, retryCountKey{}, count)
}

// getRetryCount 从上下文中获取重试计数
func getRetryCount(ctx context.Context) int {
	if v := ctx.Value(retryCountKey{}); v != nil {
		if count, ok := v.(int); ok {
			return count
		}
	}
	return 0
}

// ============================================================================
//                           同步触发实现
// ============================================================================

// triggerSyncImpl 手动触发同步的具体实现
//
// 🎯 **3阶段K桶智能同步策略**：
// 1. 同步触发与节点选择：检查系统状态，使用K桶选择最优节点
// 2. K桶智能同步：获取初始区块批次和网络高度
// 3. 分页补齐同步：使用分页方式同步剩余区块
//
// 🎯 **适配新的依赖注入架构**：
// - chainQuery: 使用persistence.ChainQuery替代ChainService（读操作）
// - blockValidator: 使用block.BlockValidator替代BlockService.ValidateBlock
// - blockProcessor: 使用block.BlockProcessor替代BlockService.ProcessBlock
//
// ⚠️ **同步状态管理**：
// - 同步状态不再持久化，查询时实时计算
// - 同步过程中的状态仅在内存中维护（sync_state.go）
//
// 参数：
//   - ctx: 上下文对象
//   - chainQuery: 链状态查询服务（读操作）
//   - blockValidator: 区块验证服务（读操作）
//   - blockProcessor: 区块处理服务（写操作）
//   - routingManager: K桶管理器，用于节点选择
//   - networkService: 网络服务，用于P2P通信
//   - host: 主机服务，用于节点ID获取和验证
//   - configProvider: 配置提供者，用于获取链ID等配置
//   - tempStore: 临时存储服务（用于存储乱序区块，支持分页补齐时的连续性检测）
//   - logger: 日志记录器
//
// 返回：
//   - error: 同步错误，nil表示成功
func triggerSyncImpl(
	ctx context.Context,
	chainQuery persistence.ChainQuery,
	queryService persistence.QueryService,
	blockValidator block.BlockValidator,
	blockProcessor block.BlockProcessor,
	routingManager kademlia.RoutingTableManager,
	networkService network.Network,
	p2pService p2pi.Service,
	configProvider config.Provider,
	tempStore storage.TempStore, // 通过管理器向下传递的临时存储（可选）
	blockHashClient core.BlockHashServiceClient, // 用于构造 locator / fork 判定
	forkHandler interfaces.InternalForkHandler, // 用于 fork-aware 自动 reorg
	logger log.Logger,
	eventBus eventiface.EventBus, // 可选：发布corruption.detected（同步/应用失败）
	recoveryMgr interface{}, // 派生数据恢复管理器（用于Tip不一致修复，暂用interface{}避免循环依赖）
) error {
	urgent, urgentReason := urgentSyncFromContext(ctx)
	if logger != nil {
		if urgent {
			if urgentReason != "" {
				logger.Infof("[TriggerSync] 🚀 启动标准K桶3阶段同步流程（URGENT: %s）", urgentReason)
			} else {
				logger.Info("[TriggerSync] 🚀 启动标准K桶3阶段同步流程（URGENT）")
			}
		} else {
			logger.Info("[TriggerSync] 🚀 启动标准K桶3阶段同步流程")
		}
	}

	// ✅ 触发幂等化：如果已经在同步中，则“视为已触发/无需重复触发”，直接返回 nil。
	// 目的：避免多源触发（订阅/定时/候选验证）堆积大量重复任务或失败日志。
	if hasActiveSyncTask() {
		if logger != nil {
			logger.Debug("[TriggerSync] ⏩ skip: already syncing")
		}
		return nil
	}

	// ✅ 全局触发去抖：在短时间内把多次触发合并掉（返回 nil 语义同上）。
	// ⚠️ 紧急同步不受去抖影响（但仍受同步锁/singleflight 约束）
	if !urgent && shouldSkipTriggerByMinInterval(configProvider, logger) {
		return nil
	}

	// ✅ 无上游退避：当路由表长期为空/无可用上游节点时，避免每次触发都等待 selectionTimeout 造成固定周期空跑。
	// ⚠️ 紧急同步不受该退避影响（紧急触发通常来自“缺块/分叉/一致性风险”，应尽快尝试）
	if !urgent && shouldSkipTriggerByNoUpstreamBackoff(logger) {
		return nil
	}

	// 生成请求ID
	requestID := fmt.Sprintf("sync-%d", time.Now().UnixNano())

	// ================================
	// 阶段0: 同步冲突检查和锁获取
	// ================================
	if !tryAcquireSyncLock(requestID, logger) {
		// 与上面的 hasActiveSyncTask 一样：冲突时不返回 error，避免上层重复告警/刷屏。
		return nil
	}
	defer releaseSyncLock(logger)

	// 创建可取消的同步上下文
	syncCtx, cancelFunc := context.WithCancel(ctx)
	defer cancelFunc()

	// 🧹 **内存监控**: 同步开始前记录内存状态
	if logger != nil {
		snapshot := GetMemorySnapshot()
		logger.Info(snapshot.FormatMemoryLog("🧹 同步开始前内存状态"))
	}

	reorgAttempted := false

restartFromStage1:
	// ================================
	// 阶段1: 同步触发与节点选择
	// ================================
	if logger != nil {
		logger.Info("[TriggerSync] 📍 阶段1: 同步触发与节点选择")
	}

	// 1.1 系统就绪性检查
	ready, err := chainQuery.IsReady(syncCtx)
	if err != nil {
		return fmt.Errorf("系统就绪检查失败: %w", err)
	}
	if !ready {
		return fmt.Errorf("系统尚未就绪，无法启动同步")
	}

	// 1.2 获取本地链信息
	localChainInfo, err := chainQuery.GetChainInfo(syncCtx)
	if err != nil {
		return fmt.Errorf("获取本地区块链信息失败: %w", err)
	}

	// 检查链信息是否为 nil
	if localChainInfo == nil {
		return fmt.Errorf("链信息为空")
	}

	localHeight := localChainInfo.Height

	// 1.2.5 同步就绪门闸（Readiness Gate）
	// 目的：解决 P2P 渐进式连接与同步瞬态流程的时序错配。
	// - 启动早期 often connected=0，若直接进入阶段1.5会因为“未连接/协议缓存为空”导致候选被瞬态过滤；
	// - 这里先等待（并对配置的 WES bootstrap 做 best-effort dial）直到至少一个 WES 候选可用。
	// 超时后可恢复返回：避免产生“硬错误”导致上层反复刷屏/节点进入错误态。
	readinessTimeout := 20 * time.Second
	if configProvider != nil {
		if bc := configProvider.GetBlockchain(); bc != nil && bc.Sync.Advanced.SyncTriggerTimeout > 0 {
			// 复用触发超时作为上限（避免引入新配置项）
			readinessTimeout = bc.Sync.Advanced.SyncTriggerTimeout
		}
	}
	if !waitForSyncReadiness(syncCtx, p2pService, configProvider, logger, readinessTimeout) {
		// 可恢复返回：定时调度器 / 下一次触发会在网络就绪后继续推进。
		return nil
	}

	// 1.3 解析 sync.startup_mode / node_role / 受信任检查点配置
	//    这里仅做语义校验与前置约束，具体“如何从检查点开始同步”的细节留给后续实现。
	if configProvider != nil {
		appCfg := configProvider.GetAppConfig()

		// 1.3.1 读取 startup_mode，按环境推导默认值
		startupMode := ""
		if appCfg != nil && appCfg.Sync != nil && appCfg.Sync.StartupMode != nil {
			startupMode = strings.ToLower(strings.TrimSpace(*appCfg.Sync.StartupMode))
		}
		if startupMode == "" {
			env := strings.ToLower(configProvider.GetEnvironment())
			if env == "dev" {
				startupMode = "from_genesis"
			} else {
				startupMode = "from_network"
			}
		}

		// 1.3.2 读取受信任检查点配置
		var requireTrusted bool
		var trustedHeight uint64
		var trustedHash string
		if appCfg != nil && appCfg.Sync != nil {
			if appCfg.Sync.RequireTrustedCheckpoint != nil {
				requireTrusted = *appCfg.Sync.RequireTrustedCheckpoint
			}
			if appCfg.Sync.TrustedCheckpoint != nil {
				if appCfg.Sync.TrustedCheckpoint.Height != nil {
					trustedHeight = *appCfg.Sync.TrustedCheckpoint.Height
				}
				if appCfg.Sync.TrustedCheckpoint.BlockHash != nil {
					trustedHash = strings.TrimSpace(*appCfg.Sync.TrustedCheckpoint.BlockHash)
				}
			}
		}

		// 1.3.3 当 require_trusted_checkpoint=true 且 startup_mode=from_network 时，检查 trusted_checkpoint 是否完整
		if startupMode == "from_network" && requireTrusted {
			if trustedHeight == 0 || trustedHash == "" {
				if logger != nil {
					logger.Errorf("[TriggerSync] ❌ 配置错误: sync.startup_mode=from_network 且 require_trusted_checkpoint=true, 但 trusted_checkpoint.height 或 block_hash 未正确配置")
				}
				return fmt.Errorf("sync 配置错误: require_trusted_checkpoint=true 但 trusted_checkpoint 未完整配置")
			}

			if logger != nil {
				logger.Infof("[TriggerSync] 🔐 受信任检查点启用: height=%d, hash=%s", trustedHeight, trustedHash)
			}
		}
	}

	// 1.4 K桶节点选择（基于Kademlia距离算法）
	//
	// ✅ 重要修复（你日志里“没处理同步”的根因）：
	// - 在启动阶段，Chain 模块可能先触发 TriggerSync，但 Kademlia/Discovery 还未完成启动与入桶；
	// - 旧逻辑会立刻把“选不到节点”当成 no-op 返回，导致启动同步几乎必然“什么都没做”；
	// - 这里加入一个短暂的等待+重试窗口（默认 30s，可通过 blockchain.sync.advanced.sync_trigger_timeout 调整），
	//   让同步在 peer/路由表就绪后自动进入 SyncHelloV2/SyncBlocksV2。
	selectionTimeout := 30 * time.Second
	if configProvider != nil {
		if bc := configProvider.GetBlockchain(); bc != nil && bc.Sync.Advanced.SyncTriggerTimeout > 0 {
			selectionTimeout = bc.Sync.Advanced.SyncTriggerTimeout
		}
	}
	selectionDeadline := time.Now().Add(selectionTimeout)

	var selectedPeers []peer.ID
	var selectErr error
	backoff := 500 * time.Millisecond

	for {
		// 🔥 使用带降级策略的节点选择（K桶 → DHT → Bootstrap）
		selectedPeers, selectErr = selectCandidatePeersWithFallback(syncCtx, routingManager, p2pService, configProvider, localChainInfo, logger)
		if selectErr == nil && len(selectedPeers) > 0 {
			break
		}
		if time.Now().After(selectionDeadline) {
			break
		}
		// 不中断启动：短暂等待后重试（避免刷屏，用 debug 级别）
		if logger != nil {
			if selectErr != nil {
				logger.Debugf("[TriggerSync] 等待上游节点就绪：节点选择失败（含降级策略），将重试（deadline=%s, err=%v）",
					selectionDeadline.Format(time.RFC3339), selectErr)
			} else {
				logger.Debugf("[TriggerSync] 等待上游节点就绪：节点选择为空（含降级策略），将重试（deadline=%s）",
					selectionDeadline.Format(time.RFC3339))
			}
		}
		time.Sleep(backoff)
		// 简单退避，最高 2s
		if backoff < 2*time.Second {
			backoff *= 2
			if backoff > 2*time.Second {
				backoff = 2 * time.Second
			}
		}
	}

	if selectErr != nil {
		// 设计语义：
		// - 仍然把“无可用上游”视为 no-op（避免启动失败）；
		// - 但现在会先等待一段时间，极大降低冷启动时“同步没做事”的概率。
		if logger != nil {
			logger.Warnf("[TriggerSync] ⚠️ K桶节点选择失败（无可用上游，跳过本次同步）: %v", selectErr)
		}
		// 进入无上游退避，避免外层（共识/运维/订阅）持续触发导致固定周期空跑
		markNoUpstream(logger)
		return nil
	}

	if len(selectedPeers) == 0 {
		// 没有任何可用的同步节点（包括K桶为空、全部被过滤等）：
		// - 这同样表示“当前网络中没有可作为上游的WES节点”，属于正常状态；
		// - 不应被视为“同步失败”，而是“无需同步”的情形。
		if logger != nil {
			logger.Info("[TriggerSync] ℹ️ 没有找到可用的同步节点，可能网络尚未连接或当前仅有本地节点，跳过本次同步")
		}
		// 同样进入无上游退避
		markNoUpstream(logger)
		return nil
	}

	// 一旦出现可用上游，立即清空无上游退避
	resetNoUpstreamBackoff()

	// 1.4.1 过滤最近已同步过的节点（避免重复同步）
	// 从配置获取节点同步缓存过期时间
	syncCacheExpiry := 5 * time.Minute // 默认5分钟
	if configProvider != nil {
		if blockchainConfig := configProvider.GetBlockchain(); blockchainConfig != nil {
			if blockchainConfig.Sync.Advanced.PeerSyncCacheExpiryMins > 0 {
				syncCacheExpiry = time.Duration(blockchainConfig.Sync.Advanced.PeerSyncCacheExpiryMins) * time.Minute
			}
		}
	}
	// ⚠️ 紧急同步：必须绕过 recently-synced 过滤，确保缺块/分叉能立即补齐/收敛
	var filteredPeers []peer.ID
	if !urgent {
		for _, peerID := range selectedPeers {
			if !checkIfPeerRecentlySynced(peerID, localHeight, syncCacheExpiry) {
				filteredPeers = append(filteredPeers, peerID)
			} else {
				if logger != nil {
					logger.Infof("[TriggerSync] ⏩ skip: recently-synced peer (expiry=%.0fm), peer=%s",
						syncCacheExpiry.Minutes(), peerID.String()[:12]+"...")
				}
			}
		}

		if len(filteredPeers) == 0 {
			if logger != nil {
				logger.Info("[TriggerSync] ⏩ skip: no new WES peers after filtering (all recently-synced)")
			}
			return nil
		}
	} else {
		filteredPeers = selectedPeers
		if logger != nil {
			logger.Warnf("[TriggerSync] 🚨 urgent: bypass peer-sync cache filtering (peers=%d, local_height=%d)", len(selectedPeers), localHeight)
		}
	}

	// 记录原始候选数（修复：此前 selectedPeers 会被覆盖，导致日志重复计数/误导）
	origSelectedCount := len(selectedPeers)
	selectedPeers = filteredPeers

	if logger != nil {
		logger.Infof("[TriggerSync] ✅ 阶段1完成: 本地高度=%d, 候选节点=%d (过滤后=%d)",
			localHeight, origSelectedCount, len(selectedPeers))
		for i, peerID := range selectedPeers {
			if i < 3 { // 只显示前3个节点以避免日志过长
				logger.Debugf("[TriggerSync] 候选节点[%d]: %s", i+1, peerID.String()[:12]+"...")
			}
		}
	}

	// ================================
	// 阶段1.5: 网络高度查询（获取真实目标高度）
	// ================================
	if logger != nil {
		logger.Info("[TriggerSync] 📍 阶段1.5: 网络高度查询")
	}

	// 1.5.1 显式查询网络最新高度（使用已筛选的候选节点）
	networkHeight, networkSourcePeer, err := queryNetworkHeightFromCandidates(
		syncCtx, selectedPeers, networkService, p2pService, localChainInfo, configProvider, logger,
	)
	if err != nil {
		if logger != nil {
			logger.Warnf("[TriggerSync] ⚠️ 网络高度查询失败: %v", err)
		}
		return fmt.Errorf("网络高度查询失败: %w", err)
	}

	if logger != nil {
		logger.Infof("[TriggerSync] ✅ 阶段1.5完成: 网络真实高度=%d, 来源节点=%s",
			networkHeight, networkSourcePeer.String()[:8])
	}

	// ✅ SYNC-002修复：保存权威网络高度（阶段1.5查询结果），阶段2/3不可降低
	authoritativeNetworkHeight := networkHeight
	maxObservedNetworkHeight := networkHeight
	if logger != nil {
		logger.Infof("🔐 权威网络高度已锁定: %d (来源: 阶段1.5)", authoritativeNetworkHeight)
	}

	// ✅ SYNC-104修复：记录阶段1.5的网络高度
	recordNetworkHeight(networkHeight, networkSourcePeer, "height_query")

	// ✅ SYNC-202修复：更新诊断信息（阶段1.5完成）
	UpdateSyncDiagnostics(func(d *SyncDiagnostics) {
		d.CurrentNetworkHeight = networkHeight
		d.NetworkHeightSourcePeer = networkSourcePeer.String()
		d.NetworkHeightQueriedAt = time.Now()
		d.CurrentLocalHeight = localHeight
		d.CurrentSyncStage = "stage1.5"
	})

	// 1.5.2 记录同步查询结果到缓存
	recordPeerSyncResult(networkSourcePeer, localHeight, networkHeight)

	// 1.5.3 判断是否需要同步
	// ⚠️ 重要：这里不再基于“网络高度 <= 本地高度”直接提前退出。
	// 原因：
	// - 网络高度查询只是采样/估计，可能命中低高度节点导致误判（例如返回 28）；
	// - 即便高度一致，也可能存在“同高度 hash 不一致”的分叉，需要进入 SyncHelloV2 做 fork 判定与自动 reorg。
	// ✅ 真实判定统一交给阶段2的 SyncHelloV2（fork-aware）。

	// ================================
	// 阶段2: K桶智能同步请求（获取初始区块批次）
	// ================================
	if logger != nil {
		logger.Info("[TriggerSync] 📍 阶段2: SyncHelloV2 + SyncBlocksV2（fork-aware）")
	}

	// 2.1 先执行 SyncHelloV2：请求携带 (localTipHeight, localTipHash, locator) 以判定同链/分叉
	var initialBlocks []*core.Block // 使用proto统一格式
	var sourcePeer peer.ID
	var hello *helloV2Info
	var lastErr error
	var anyHelloSucceeded bool
	var maxRemoteTip uint64

	// 2.1.1 计算 localTipHash（优先以 localHeight 对应区块的真实 hash 为准，避免 BestBlockHash 与高度不一致导致误判 fork）
	localTipHash := localChainInfo.BestBlockHash
	if queryService != nil && blockHashClient != nil {
		if blk, err := queryService.GetBlockByHeight(syncCtx, localHeight); err == nil && blk != nil && blk.Header != nil {
			if resp, err := blockHashClient.ComputeBlockHash(syncCtx, &core.ComputeBlockHashRequest{Block: blk}); err == nil && resp != nil && resp.IsValid && len(resp.Hash) == 32 {
				localTipHash = resp.Hash
				// 若与 chainInfo 中的 BestBlockHash 不一致，立即修复
			if len(localChainInfo.BestBlockHash) == 32 && string(localChainInfo.BestBlockHash) != string(resp.Hash) {
				if logger != nil {
					logger.Errorf("❌ 检测到 tip_hash 不一致，触发索引自动修复: height=%d", localHeight)
					logger.Errorf("   stored_hash=%x, actual_hash=%x", localChainInfo.BestBlockHash[:6], resp.Hash[:6])
				}
				
				// TODO: 立即触发索引修复
				// 注意：这里需要注入 recoveryManager，当前作为TODO标记
				// 实际使用时需要在sync.Manager中添加recoveryManager依赖
				// 详见: _dev/14-实施任务-implementation-tasks/20251217-sync-kbucket-critical-defects/PENDING_FX_INTEGRATION.md
				if logger != nil {
					logger.Warn("⚠️ 索引修复功能需要注入 recoveryManager（待完成fx集成）")
				}
				
				// 更新本地变量
				localChainInfo.BestBlockHash = resp.Hash
				localTipHash = resp.Hash
				}
			} else if logger != nil {
				logger.Debugf("[TriggerSync] 计算本地 tip_hash 失败（回退到 chainInfo.BestBlockHash）：height=%d err=%v", localHeight, err)
			}
		} else if logger != nil {
			logger.Debugf("[TriggerSync] 读取本地 tip 区块失败（回退到 chainInfo.BestBlockHash）：height=%d err=%v", localHeight, err)
		}
	}

	// 2.1.2 构造 locator（二进制），用于共同祖先快速定位（必须使用 QueryService）
	var locatorBytes []byte
	if queryService != nil && blockHashClient != nil {
		if b, err := BuildBlockLocatorBinary(syncCtx, queryService, blockHashClient, localHeight, 32, configProvider); err == nil {
			locatorBytes = b
		} else if logger != nil {
			logger.Debugf("[TriggerSync] locator 构造失败（降级为无 locator hello）: %v", err)
		}
	}
	if logger != nil {
		entries := 0
		if len(locatorBytes)%(8+32) == 0 {
			entries = len(locatorBytes) / (8 + 32)
		}
		if len(localTipHash) == 32 {
			logger.Debugf("[TriggerSync] SyncHelloV2 payload: local_tip=%d tip_hash=%x locator_len=%d locator_entries=%d",
				localHeight, localTipHash[:6], len(locatorBytes), entries)
		} else {
			logger.Debugf("[TriggerSync] SyncHelloV2 payload: local_tip=%d tip_hash_len=%d locator_len=%d locator_entries=%d",
				localHeight, len(localTipHash), len(locatorBytes), entries)
		}
	}

	// ✅ SYNC-001修复：优先使用阶段1.5选择的高度源节点
	var candidatesWithPriority []peer.ID
	if networkSourcePeer != "" {
		candidatesWithPriority = append(candidatesWithPriority, networkSourcePeer)
		if logger != nil {
			logger.Infof("📌 优先使用阶段1.5高度源节点: %s (height=%d)", 
				networkSourcePeer.String()[:12]+"...", authoritativeNetworkHeight)
		}
	}
	// 其他节点作为备选（排除已添加的高度源节点）
	for _, p := range selectedPeers {
		if p != networkSourcePeer {
			candidatesWithPriority = append(candidatesWithPriority, p)
		}
	}

	// ✅ SYNC-101修复 + SYNC-202增强：过滤低高度节点和坏节点（紧急模式放宽）
	var filteredByHeight []peer.ID
	var skippedLowHeight, skippedBad int
	for _, p := range candidatesWithPriority {
		// 🆕 紧急模式下放宽过滤：只过滤明确的坏节点，不过滤低高度节点
		if !urgent && isLowHeightPeer(p) {
			if logger != nil {
				logger.Debugf("⏩ 跳过低高度节点: %s", p.String()[:12]+"...")
			}
			skippedLowHeight++
			continue
		}
		// 🆕 紧急模式下：检查坏节点是否即将过期（剩余时间 < 10分钟），如果是则放行
		if IsBadPeer(p) {
			if urgent && isBadPeerNearExpiry(p, 10*time.Minute) {
				if logger != nil {
					logger.Debugf("🔄 紧急模式：坏节点即将过期，放行: %s", p.String()[:12]+"...")
				}
			} else {
				if logger != nil {
					logger.Debugf("⏩ 跳过坏节点: %s", p.String()[:12]+"...")
				}
				skippedBad++
				continue
			}
		}
		filteredByHeight = append(filteredByHeight, p)
	}
	candidatesWithPriority = filteredByHeight

	// 🆕 SYNC-HIGH002修复：过滤后无节点时使用三级降级策略
	if len(candidatesWithPriority) == 0 {
		if logger != nil {
			logger.Warnf("[TriggerSync] ⚠️ 过滤低高度(%d)和坏节点(%d)后，无可用候选节点，启动降级策略",
				skippedLowHeight, skippedBad)
		}

		// 尝试使用带降级策略的节点选择
		fallbackPeers, fallbackErr := selectCandidatePeersWithFallback(
			syncCtx,
			routingManager,
			p2pService,
			configProvider,
			localChainInfo,
			logger,
		)
		if fallbackErr != nil || len(fallbackPeers) == 0 {
			// 触发发现加速，快速恢复网络连接
			triggerDiscoveryAcceleration(eventBus, "sync_no_candidates", logger)

			// 🆕 最后尝试：清理过期的低高度节点记录，重新尝试原始候选
			clearExpiredLowHeightPeers()
			var retryPeers []peer.ID
			for _, p := range selectedPeers {
				if !IsBadPeer(p) {
					retryPeers = append(retryPeers, p)
				}
			}
			if len(retryPeers) > 0 {
				if logger != nil {
					logger.Infof("🔄 清理过期记录后重试: %d 个候选节点", len(retryPeers))
				}
				candidatesWithPriority = retryPeers
			} else {
				return fmt.Errorf("过滤低高度和坏节点后，无可用候选节点（降级策略也失败）")
			}
		} else {
			if logger != nil {
				logger.Infof("✅ 降级策略成功: %d 个备用节点", len(fallbackPeers))
			}
			candidatesWithPriority = fallbackPeers
		}
	}

	// ✅ SYNC-202修复：更新诊断信息（阶段2开始，节点已过滤）
	UpdateSyncDiagnostics(func(d *SyncDiagnostics) {
		d.CurrentSyncStage = "stage2"
		d.AvailablePeers = len(candidatesWithPriority)
	})

	for _, peerID := range candidatesWithPriority {
		hello, err = performSyncHelloV2(
			syncCtx,
			peerID,
			localHeight,
			localTipHash,
			locatorBytes,
			localChainInfo,
			networkService,
			p2pService,
			configProvider,
			logger,
		)
		if err != nil || hello == nil {
			lastErr = err
			// ✅ SYNC-003修复：记录hello失败原因
			errMsg := "hello returned nil"
			if err != nil {
				errMsg = err.Error()
			}
			recordSyncFailure(peerID, "hello", "network_error", errMsg, logger)
			if logger != nil {
				logger.Warnf("[TriggerSync] 节点 %s SyncHelloV2 失败: %v", peerID.String()[:8], err)
			}
			continue // 尝试下一个节点
		}
		anyHelloSucceeded = true
		if hello.remoteTipHeight > maxRemoteTip {
			maxRemoteTip = hello.remoteTipHeight
		}

		// ✅ SYNC-002修复：只升不降，只有更高的高度才更新观察高度
		if hello.remoteTipHeight > 0 {
			// ✅ SYNC-104修复：记录hello返回的高度
			recordNetworkHeight(hello.remoteTipHeight, peerID, "hello")
			
			if hello.remoteTipHeight > maxObservedNetworkHeight {
				maxObservedNetworkHeight = hello.remoteTipHeight
				if logger != nil {
					logger.Infof("🔼 更新观察网络高度: %d -> %d (数据源: %s)", 
						networkHeight, hello.remoteTipHeight, peerID.String()[:12]+"...")
				}
				// 更新当前工作高度
			networkHeight = hello.remoteTipHeight
			} else if hello.remoteTipHeight < authoritativeNetworkHeight {
				if logger != nil {
					logger.Warnf("⚠️ 忽略低高度节点: remote_height=%d < authoritative=%d (节点: %s)",
						hello.remoteTipHeight, authoritativeNetworkHeight, peerID.String()[:12]+"...")
				}
			}
		}

		switch hello.relationship {
		case "UP_TO_DATE":
			sourcePeer = peerID
			// 不需要同步
			initialBlocks = nil
		case "REMOTE_AHEAD_SAME_CHAIN":
			sourcePeer = peerID
			
			// ✅ 更新诊断信息：设置数据源节点
			UpdateSyncDiagnostics(func(d *SyncDiagnostics) {
				d.CurrentDataSourcePeer = sourcePeer.String()
			})
			
			// 拉取一个初始批次（响应大小受限；后续分页补齐继续）
			if networkHeight > localHeight {
				initialBlocks, err = fetchBlockRange(
					syncCtx,
					sourcePeer,
					localHeight+1,
					networkHeight,
					networkService,
					p2pService,
					configProvider,
					logger,
				)
				if err != nil {
					lastErr = err
					// ✅ SYNC-003修复：记录blocks失败原因（细化分类）
					recordSyncFailure(peerID, "blocks", ClassifyError(err), err.Error(), logger)
					if logger != nil {
						logger.Warnf("[TriggerSync] 节点 %s SyncBlocksV2 初始批次失败: %v", peerID.String()[:8], err)
					}
					sourcePeer = peer.ID("")
					continue
				}
				
				// ✅ 更新诊断信息：记录拉取的区块数
				UpdateSyncDiagnostics(func(d *SyncDiagnostics) {
					d.BlocksFetched += uint64(len(initialBlocks))
				})
			}
		case "REMOTE_BEHIND":
			// ✅ SYNC-005修复：记录低高度节点，短期内不再选择
			recordLowHeightPeer(peerID, hello.remoteTipHeight, logger)
			if logger != nil {
				logger.Warnf("⚠️ 对端落后（REMOTE_BEHIND）: remote_height=%d < local_height=%d, 切换到下一个节点",
					hello.remoteTipHeight, localHeight)
			}
			continue
		case "UNKNOWN":
			// ✅ SYNC-005修复：无法判定链关系，切换到下一个节点
			if logger != nil {
				logger.Warnf("⚠️ 无法判定链关系（UNKNOWN）: peer=%s, 切换到下一个节点", 
					peerID.String()[:12]+"...")
			}
			continue
		case "FORK_DETECTED":
			// ✅ SYNC-102修复：确认已使用locator进行fork-aware判定
			if logger != nil {
				ah := ""
				if len(hello.commonAncestorHash) == 32 {
					ah = fmt.Sprintf("%x", hello.commonAncestorHash[:6])
				}
				logger.Warnf("[TriggerSync] ⚠️ 检测到分叉（基于locator）: peer=%s remote_tip=%d local_tip=%d ancestor=%d ancestor_hash=%s locator_len=%d",
					peerID.String()[:8], hello.remoteTipHeight, localHeight, 
					hello.commonAncestorHeight, ah, len(locatorBytes))
			}
			
			// 🆕 降级策略：如果本地高度为0且无法定位共同祖先，降级为普通同步
			if localHeight == 0 && hello.commonAncestorHeight == 0 && len(hello.commonAncestorHash) != 32 {
				if logger != nil {
					logger.Warnf("[TriggerSync] 🔄 空链场景降级：本地高度为0且无共同祖先，切换为普通同步模式")
				}
				// 直接执行普通同步逻辑（与REMOTE_AHEAD_SAME_CHAIN相同）
				sourcePeer = peerID
				
				// 更新诊断信息：设置数据源节点
				UpdateSyncDiagnostics(func(d *SyncDiagnostics) {
					d.CurrentDataSourcePeer = sourcePeer.String()
				})
				
				// 拉取初始批次
				if networkHeight > localHeight {
					initialBlocks, err = fetchBlockRange(
						syncCtx,
						sourcePeer,
						localHeight+1,
						networkHeight,
						networkService,
						p2pService,
						configProvider,
						logger,
					)
					if err != nil {
						lastErr = err
						recordSyncFailure(peerID, "blocks", ClassifyError(err), err.Error(), logger)
						if logger != nil {
							logger.Warnf("[TriggerSync] 节点 %s SyncBlocksV2 初始批次失败: %v", peerID.String()[:8], err)
						}
						sourcePeer = peer.ID("")
						continue
					}
					
					// 更新诊断信息：记录拉取的区块数
					UpdateSyncDiagnostics(func(d *SyncDiagnostics) {
						d.BlocksFetched += uint64(len(initialBlocks))
					})
				}
				// 空链降级成功，跳出循环进入阶段3
				break
			}
			
			if reorgAttempted {
				h := localHeight
				publishSyncCorruption(eventBus, fmt.Errorf("fork detected (already attempted reorg): peer=%s ancestor=%d", peerID, hello.commonAncestorHeight), &h)
				return fmt.Errorf("同步握手检测到分叉，且已尝试过一次 reorg 仍未收敛：peer=%s", peerID)
			}

			if err := tryAutoReorgFromHello(
				syncCtx,
				peerID,
				hello,
				chainQuery,
				blockHashClient,
				forkHandler,
				networkService,
				p2pService,
				configProvider,
				logger,
			); err != nil {
				if logger != nil {
					logger.Errorf("[TriggerSync] ❌ 自动reorg失败: peer=%s remote_tip=%d ancestor=%d err=%v",
						peerID.String()[:8], hello.remoteTipHeight, hello.commonAncestorHeight, err)
				}
				h := localHeight
				publishSyncCorruption(eventBus, fmt.Errorf("auto reorg failed: %w", err), &h)
				
				// 🆕 降级策略：如果reorg失败且ancestor信息缺失，尝试切换下一个节点而不是直接失败
				if hello.commonAncestorHeight == 0 && len(hello.commonAncestorHash) != 32 {
					if logger != nil {
						logger.Warnf("[TriggerSync] 🔄 自动reorg失败但祖先信息缺失，尝试切换到下一个节点")
					}
					// 记录失败原因
					recordSyncFailure(peerID, "reorg", "missing_ancestor", 
						"auto reorg failed due to missing ancestor info", logger)
					continue // 尝试下一个节点
				}
				
				return fmt.Errorf("自动 reorg 失败: %w", err)
			}

			reorgAttempted = true
			if logger != nil {
				logger.Warn("[TriggerSync] 🔁 自动reorg完成，重启同步流程以收敛到同一链尖")
			}
			goto restartFromStage1
		default:
			// ✅ SYNC-005修复：非预期的 relationship，切换到下一个节点
			if logger != nil {
				logger.Warnf("⚠️ 非预期的 relationship: %v, peer=%s, 切换到下一个节点", 
					hello.relationship, peerID.String()[:12]+"...")
			}
			continue
		}

		sourcePeer = peerID
		break
	}

	if sourcePeer == "" {
		// ✅ 重要语义：如果握手成功但所有对端都"落后于本地链尖"，则无需同步（不应被视为失败）。
		// 典型场景：本节点从磁盘恢复到更高高度，但当前连接的上游节点处于较低高度/不同步。
		if anyHelloSucceeded && maxRemoteTip <= localHeight {
			if logger != nil {
				logger.Infof("[TriggerSync] ✅ 无需同步：所有候选节点均落后于本地链尖（local=%d max_remote=%d）", localHeight, maxRemoteTip)
			}
			return nil
		}
		
		// ✅ SYNC-203修复：实现同步重试策略
		retryCount := getRetryCount(ctx)
		maxRetries := 3
		if retryCount < maxRetries {
			retryDelay := time.Duration(retryCount+1) * 5 * time.Second
			if logger != nil {
				logger.Infof("🔄 阶段2同步失败，将在 %v 后重试 (第 %d/%d 次)", 
					retryDelay, retryCount+1, maxRetries)
			}
			
			time.Sleep(retryDelay)
			
			// 递归调用，增加重试计数
			return triggerSyncImpl(
				withRetryCount(ctx, retryCount+1),
				chainQuery, queryService, blockValidator, blockProcessor,
				routingManager, networkService, p2pService, configProvider,
				tempStore, blockHashClient, forkHandler, logger, eventBus, recoveryMgr,
			)
		}
		
		if logger != nil {
			// 该分支会返回 error 给上层（触发 corruption 事件），语义为“同步失败”，必须使用 ERROR。
			logger.Errorf("[TriggerSync] ❌ 所有候选节点的SyncHelloV2/初始批次均失败，已达最大重试次数")
		}
		h := localHeight
		if lastErr == nil {
			lastErr = fmt.Errorf("no eligible peer")
		}
		publishSyncCorruption(eventBus, lastErr, &h)
		return fmt.Errorf("阶段2同步失败，已达最大重试次数(%d): %w", maxRetries, lastErr)
	}

	if logger != nil {
		logger.Infof("[TriggerSync] ✅ 同步握手完成，获得初始区块批次: %d个区块, 数据源: %s, relationship=%s",
			len(initialBlocks), sourcePeer.String()[:12]+"...", hello.relationship)
	}

	// 2.3 设置活跃同步状态（使用观察到的最高网络高度）
	setActiveSyncTask(&activeSyncContext{
		RequestID:       requestID,
		StartTime:       time.Now(),
		TargetHeight:    maxObservedNetworkHeight,
		SourcePeerID:    sourcePeer,
		CancelFunc:      cancelFunc,
		ProcessedBlocks: 0,
	})

	// ✅ 同步状态不再持久化，仅在内存中维护（sync_state.go）
	// 查询时通过 sync/status.go 实时计算同步状态

	if logger != nil {
		logger.Infof("[TriggerSync] ✅ 阶段2完成: 观察网络高度=%d, 初始区块=%d, 数据源=%s",
			maxObservedNetworkHeight, len(initialBlocks), sourcePeer.String()[:8])
	}

	// ================================
	// 阶段3: 分页补齐同步
	// ================================
	
	// ✅ SYNC-002/SYNC-004修复：使用阶段1.5查询的权威网络高度（不会被阶段2覆盖）
	finalAuthoritativeHeight := authoritativeNetworkHeight
	if maxObservedNetworkHeight > finalAuthoritativeHeight {
		// 如果阶段2观察到更高的高度，更新权威高度
		finalAuthoritativeHeight = maxObservedNetworkHeight
		if logger != nil {
			logger.Infof("🔼 更新最终权威高度: %d -> %d", 
				authoritativeNetworkHeight, finalAuthoritativeHeight)
		}
	}
	
	if logger != nil {
		logger.Infof("[TriggerSync] 📍 阶段3: 分页补齐同步 (local=%d, authoritative=%d, gap=%d)",
			localHeight, finalAuthoritativeHeight, finalAuthoritativeHeight-localHeight)
	}

	// ✅ SYNC-202修复：更新诊断信息（阶段3开始）
	UpdateSyncDiagnostics(func(d *SyncDiagnostics) {
		d.CurrentSyncStage = "stage3"
		d.CurrentNetworkHeight = finalAuthoritativeHeight
	})

	// 3.1 处理初始区块（来自K桶智能同步）
	if len(initialBlocks) > 0 {
		if logger != nil {
			logger.Infof("[TriggerSync] 📦 开始处理初始区块批次: %d个区块", len(initialBlocks))
		}
		err = processBlockBatch(syncCtx, initialBlocks, blockValidator, blockProcessor, logger)
		if err != nil {
			if logger != nil {
				logger.Errorf("[TriggerSync] ❌ 初始区块批次处理失败: %v", err)
			}
			h := localHeight
			publishSyncCorruption(eventBus, err, &h)
			return fmt.Errorf("初始区块批次处理失败: %w", err)
		}
		updateSyncProgress(uint64(len(initialBlocks)))
		
		// ✅ 更新诊断信息：记录处理的区块数
		UpdateSyncDiagnostics(func(d *SyncDiagnostics) {
			d.BlocksProcessed += uint64(len(initialBlocks))
		})

		// ✅ 同步进度在内存中更新（sync_state.go），不再持久化

		if logger != nil {
			logger.Infof("[TriggerSync] ✅ 初始区块批次处理完成: %d个区块已应用", len(initialBlocks))
		}
	} else {
		if logger != nil {
			logger.Info("[TriggerSync] 📦 K桶同步未返回初始区块，继续分页同步")
		}
	}

	// 3.2 计算剩余需要同步的高度范围（使用权威高度）
	currentHeight := localHeight + uint64(len(initialBlocks))
	if finalAuthoritativeHeight > currentHeight {
		missingBlocks := finalAuthoritativeHeight - currentHeight
		if logger != nil {
			logger.Infof("[TriggerSync] 📏 需要分页同步剩余区块: %d个 (从高度%d到%d)",
				missingBlocks, currentHeight+1, finalAuthoritativeHeight)
		}

		// ✅ SYNC-103修复：构造阶段3的可用节点列表，支持多节点容错
		// 优先使用阶段2成功的节点，其他候选节点作为备选
		availablePeersForStage3 := []peer.ID{}
		if sourcePeer != "" {
			availablePeersForStage3 = append(availablePeersForStage3, sourcePeer)
			if logger != nil {
				logger.Infof("📌 阶段3优先节点: %s (阶段2成功)", 
					sourcePeer.String()[:12]+"...")
			}
		}
		// 添加其他候选节点作为备选（排除sourcePeer，过滤低高度和坏节点）
		for _, p := range candidatesWithPriority {
			if p != sourcePeer && !isLowHeightPeer(p) && !IsBadPeer(p) {
				availablePeersForStage3 = append(availablePeersForStage3, p)
			}
		}

		if logger != nil {
			logger.Infof("📊 阶段3可用节点数: %d (主节点=%s, 备选=%d)", 
				len(availablePeersForStage3), 
				sourcePeer.String()[:12]+"...",
				len(availablePeersForStage3)-1)
		}

		err = performRangePaginatedSync(
			syncCtx, availablePeersForStage3, currentHeight, finalAuthoritativeHeight,
			networkService, p2pService, blockValidator, blockProcessor, tempStore, configProvider, logger,
		)
		if err != nil {
			if logger != nil {
				logger.Errorf("[TriggerSync] ❌ 分页补齐同步失败: %v", err)
			}
			h := currentHeight
			publishSyncCorruption(eventBus, err, &h)
			return fmt.Errorf("分页补齐同步失败: %w", err)
		}

		if logger != nil {
			logger.Infof("[TriggerSync] ✅ 分页补齐同步完成: 已同步到高度%d", finalAuthoritativeHeight)
		}
	} else {
		if logger != nil {
			logger.Info("[TriggerSync] 📏 无需分页同步，初始批次已包含所有缺失区块")
		}
	}

	// ✅ 同步完成，状态查询时将实时计算（sync/status.go）

	// ✅ SYNC-202修复：更新诊断信息（同步完成）
	UpdateSyncDiagnostics(func(d *SyncDiagnostics) {
		d.CurrentSyncStage = "completed"
		d.SyncProgress = 1.0
	})

	if logger != nil {
		logger.Info("[TriggerSync] 🎉 标准3阶段同步流程完成！")

		// 🧹 **内存优化**: 同步完成后进行内存清理
		snapshotBefore := GetMemorySnapshot()

		// 强制垃圾回收
		runtime.GC()
		runtime.GC() // 执行两次GC确保彻底清理

		snapshotAfter := GetMemorySnapshot()

		logger.Infof("🧹 同步完成后内存优化: "+
			"heap_alloc=%dMB->%dMB (节省=%dMB) "+
			"rss=%dMB->%dMB (节省=%dMB) "+
			"gc_count=%d->%d",
			snapshotBefore.HeapAllocMB, snapshotAfter.HeapAllocMB,
			snapshotBefore.HeapAllocMB-snapshotAfter.HeapAllocMB,
			snapshotBefore.RSSMB, snapshotAfter.RSSMB,
			int64(snapshotBefore.RSSMB)-int64(snapshotAfter.RSSMB),
			snapshotBefore.NumGC, snapshotAfter.NumGC)
	}
	return nil
}

func publishSyncCorruption(eventBus eventiface.EventBus, err error, height *uint64) {
	if eventBus == nil || err == nil {
		return
	}
	data := types.CorruptionEventData{
		Component: types.CorruptionComponentSync,
		Phase:     types.CorruptionPhaseApply,
		Severity:  types.CorruptionSeverityCritical,
		Height:    height,
		ErrClass:  corruptutil.ClassifyErr(err),
		Error:     err.Error(),
		At:        types.RFC3339Time(time.Now()),
	}
	eventBus.Publish(eventiface.EventTypeCorruptionDetected, context.Background(), data)
}

// triggerDiscoveryAcceleration 触发发现加速
// 🆕 SYNC-HIGH002修复：当同步无可用节点时，触发发现机制快速恢复网络连接
func triggerDiscoveryAcceleration(eventBus eventiface.EventBus, reason string, logger log.Logger) {
	if eventBus == nil {
		return
	}

	resetData := &types.DiscoveryResetEventData{
		Reason:           reason,
		Trigger:          "sync_no_candidates",
		RoutingTableSize: 0,
		Timestamp:        time.Now().Unix(),
	}

	eventBus.Publish(events.EventTypeDiscoveryIntervalReset, resetData)

	if logger != nil {
		logger.Infof("🔄 触发发现加速: reason=%s", reason)
	}
}

// ============================================================================
//                           配置获取工具函数
// ============================================================================
