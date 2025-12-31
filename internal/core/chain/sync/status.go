// Package sync 实现同步状态查询功能
//
// 🎯 **状态查询实现**
//
// 本文件实现 CheckSync 方法的具体逻辑，提供真实的同步状态查询功能：
// - 查询本地链高度和网络最新高度
// - 计算同步进度和状态判断
// - 构建完整的同步状态信息
package sync

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/network"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/types"
)

// 状态查询静默窗口缓存（用于抑制频繁网络高度查询/日志抖动）
var (
	statusQueryMu              sync.Mutex
	lastNetworkHeightQueryTime time.Time
	lastNetworkHeightValue     uint64
	lastWasUpToDate            bool
)

// ============================================================================
//                           同步状态查询实现
// ============================================================================

// checkSyncImpl 查询当前同步状态的具体实现
//
// 🎯 **真实状态查询逻辑**：
// 1. 查询本地链高度
// 2. 查询网络高度（通过K桶节点采样）
// 3. 计算同步进度和状态
// 4. 构建完整状态信息
//
// 🎯 **适配新的依赖注入架构**：
// - chainQuery: 使用persistence.ChainQuery替代ChainService（读操作）
//
// 参数：
//   - ctx: 上下文对象
//   - chainQuery: 链状态查询服务（读操作）
//   - kBucketManager: K桶管理器，用于选择节点查询网络高度
//   - network: 网络服务，用于与远程节点通信
//   - logger: 日志记录器
//
// 返回：
//   - *types.SystemSyncStatus: 同步状态信息
//   - error: 查询错误
func checkSyncImpl(
	ctx context.Context,
	chainQuery persistence.ChainQuery,
	routingManager kademlia.RoutingTableManager,
	network network.Network,
	p2pService p2pi.Service,
	configProvider config.Provider,
	runtimeState p2pi.RuntimeState,
	logger log.Logger,
) (*types.SystemSyncStatus, error) {
	// 查询本地链状态
	chainInfo, err := chainQuery.GetChainInfo(ctx)
	if err != nil {
		if logger != nil {
			logger.Errorf("查询本地链状态失败: %v", err)
		}
		return &types.SystemSyncStatus{
			Status:        types.SyncStatusError,
			CurrentHeight: 0,
			NetworkHeight: 0,
			SyncProgress:  0.0,
			LastSyncTime:  types.RFC3339Time(time.Now()),
			ErrorMessage:  fmt.Sprintf("查询本地链状态失败: %v", err),
		}, nil
	}

	// 检查链信息是否为 nil
	if chainInfo == nil {
		if logger != nil {
			logger.Errorf("链信息为空")
		}
		return &types.SystemSyncStatus{
			Status:        types.SyncStatusError,
			CurrentHeight: 0,
			NetworkHeight: 0,
			SyncProgress:  0.0,
			LastSyncTime:  types.RFC3339Time(time.Now()),
			ErrorMessage:  "链信息为空",
		}, nil
	}

	localHeight := chainInfo.Height
	if logger != nil {
		logger.Debugf("本地区块链高度: %d", localHeight)
	}

	// 查询网络高度（使用与trigger.go一致的节点筛选和查询逻辑）
	var networkHeight uint64 = localHeight // 默认使用本地高度

	// 1. K桶节点选择（与trigger.go一致）
	selectedPeers, err := selectKBucketPeersForSync(ctx, routingManager, p2pService, configProvider, chainInfo, logger)
	if err != nil {
		if logger != nil {
			logger.Debugf("状态查询-K桶节点选择失败: %v，使用本地高度", err)
		}
	} else if len(selectedPeers) == 0 {
		if logger != nil {
			logger.Debug("状态查询-没有可用节点，使用本地高度")
		}
	} else {
		// 2. ✅ UpToDate 静默窗口：当最近一次查询已确认“已同步”，在窗口期内不重复查询网络高度
		silenceMins := 5
		if configProvider != nil && configProvider.GetBlockchain() != nil {
			if m := configProvider.GetBlockchain().Sync.Advanced.UpToDateSilenceWindowMins; m > 0 {
				silenceMins = m
			}
		}
		silenceWindow := time.Duration(silenceMins) * time.Minute

		// 检查上次状态查询是否在静默窗口内
		var shouldQuery bool = true
		var cachedHeight uint64
		var useCache bool
		statusQueryMu.Lock()
		if !lastNetworkHeightQueryTime.IsZero() &&
			time.Since(lastNetworkHeightQueryTime) < silenceWindow &&
			lastWasUpToDate {
			// 只有在“上次确认已同步”的情况下才使用缓存，避免掩盖落后状态
			useCache = true
			cachedHeight = lastNetworkHeightValue
			shouldQuery = false
		}
		statusQueryMu.Unlock()

		activeSyncMutex.RLock()
		if activeSyncTask != nil {
			// 如果有活跃同步任务，则不重复查询
			shouldQuery = false
			if logger != nil {
				logger.Debug("状态查询-存在活跃同步任务，跳过网络高度查询")
			}
		}
		activeSyncMutex.RUnlock()

		if useCache {
			networkHeight = cachedHeight
			if logger != nil {
				logger.Debugf("状态查询-命中UpToDate静默窗口缓存: network_height=%d window=%s", networkHeight, silenceWindow)
			}
		} else if shouldQuery {
			// 3. 使用新的网络高度查询函数（与trigger.go一致）
			height, _, queryErr := queryNetworkHeightFromCandidates(ctx, selectedPeers, network, p2pService, chainInfo, configProvider, logger)
			if queryErr != nil {
				if logger != nil {
					logger.Debugf("状态查询-网络高度查询失败: %v，使用本地高度", queryErr)
				}
			} else {
				networkHeight = height
				if logger != nil {
					logger.Debugf("状态查询-网络高度查询成功: %d", networkHeight)
				}
				// 更新静默窗口缓存：仅当确认“已同步”时启用缓存
				statusQueryMu.Lock()
				lastNetworkHeightQueryTime = time.Now()
				lastNetworkHeightValue = networkHeight
				lastWasUpToDate = (networkHeight <= localHeight)
				statusQueryMu.Unlock()
			}
		}
	}

	if logger != nil {
		logger.Debugf("网络区块链高度: %d", networkHeight)
	}

	// 计算同步进度和状态
	status, progress := calculateSyncStatus(localHeight, networkHeight)

	// 特殊处理：根据环境、sync.startup_mode、node_role 与受信任检查点，对冷启动场景进行语义化判断
	// 目的：
	//   - 在 dev + from_genesis + miner/validator 场景下，单节点高度为0时仍可被视为“已同步”，以便开发环境挖矿与调试；
	//   - 在 test/prod 或 from_network 场景下，或节点角色为 full/light 时，高度为0则保持 Bootstrapping/Degraded 语义，禁止参与出块；
	//   - 当 require_trusted_checkpoint=true 但未配置 trusted_checkpoint 时，及时暴露为配置错误，避免“假同步”。
	if configProvider != nil {
		appCfg := configProvider.GetAppConfig()

		// 1. 读取 sync.startup_mode
		startupMode := ""
		if appCfg != nil && appCfg.Sync != nil && appCfg.Sync.StartupMode != nil {
			startupMode = strings.ToLower(strings.TrimSpace(*appCfg.Sync.StartupMode))
		}

		// 2. 未显式配置时，按环境推导默认模式：dev → from_genesis，其它 → from_network
		if startupMode == "" {
			env := strings.ToLower(configProvider.GetEnvironment())
			if env == "dev" {
				startupMode = "from_genesis"
			} else {
				startupMode = "from_network"
			}
		}

		// 3. 读取节点角色（可选）
		nodeRole := ""
		if appCfg != nil && appCfg.NodeRole != nil {
			nodeRole = strings.ToLower(strings.TrimSpace(*appCfg.NodeRole))
		}

		isConsensusNode := nodeRole == "" || nodeRole == "miner" || nodeRole == "validator"

		// 4. 读取受信任检查点配置（如有）
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

		// 5. 当 require_trusted_checkpoint=true 且 startup_mode=from_network 时，如果未配置完整检查点，则视为配置错误
		if startupMode == "from_network" && requireTrusted {
			if trustedHeight == 0 || trustedHash == "" {
				// 注意：这里只是状态计算逻辑，真正的校验/拒绝启动应由配置验证层补充。
				// 这里将状态标记为 Error，以便对外暴露为“配置不正确”的同步错误，而不是误判为已同步。
				status = types.SyncStatusError
				progress = 0.0
			}
		}

		// 6. 对 dev + from_genesis + 共识角色（miner/validator）的冷启动场景，且未强制检查点要求，直接视为已同步，方便单节点开发/测试
		if localHeight == 0 && networkHeight == 0 && startupMode == "from_genesis" && isConsensusNode && !requireTrusted {
			status = types.SyncStatusSynced
			progress = 100.0
		}

		// 7. from_network 场景下 local=0 && network=0 的统一语义
		//
		// 🎯 架构原则：不区分 dev/test/prod 环境，所有环境统一行为
		// - 当 local=0 && network=0 时，表示当前节点是首个节点或网络中无其他节点
		// - 统一保留 Bootstrapping 语义，由共识层的单节点特判决定是否允许挖矿
		// - 这样确保 dev/test/prod 环境行为一致，避免测试通过但生产失败的情况
		// 注意：这里不再根据环境强制标记为 Degraded，保持 Bootstrapping 状态
		// （calculateSyncStatus 已经会根据 local=0 && network=0 返回 Bootstrapping，这里无需额外处理）
	}

	if logger != nil {
		logger.Debugf("同步状态: %s, 进度: %.2f%%", status.String(), progress)
	}

	// 更新 RuntimeState（如果可用）
	if runtimeState != nil {
		// 获取同步滞后阈值（使用默认值，配置中暂无此字段）
		var syncLagThreshold uint64 = 10 // 默认10个区块

		// 判断是否正在同步
		isSyncing := status == types.SyncStatusSyncing

		// 更新 RuntimeState
		runtimeState.UpdateSyncStatusFromSyncService(
			localHeight,
			networkHeight,
			syncLagThreshold,
			isSyncing,
		)
	}

	// 🔧 **增强状态信息**：包含活跃同步任务详情
	syncStatus := &types.SystemSyncStatus{
		Status:        status,
		CurrentHeight: localHeight,
		NetworkHeight: networkHeight,
		SyncProgress:  progress,
		LastSyncTime:  types.RFC3339Time(time.Now()),
		ErrorMessage:  "",
	}

	// 如果有活跃同步任务，添加任务详情
	activeSyncMutex.RLock()
	currentTask := activeSyncTask
	activeSyncMutex.RUnlock()

	if currentTask != nil {
		syncStatus.LastSyncTime = types.RFC3339Time(currentTask.StartTime)

		if status == types.SyncStatusSyncing {
			// 📊 **增强活跃任务统计信息**
			elapsed := time.Since(currentTask.StartTime).Seconds()

			// 计算同步速度
			var syncSpeed float64
			if elapsed > 0 {
				syncSpeed = float64(currentTask.ProcessedBlocks) / elapsed
			}

			// 计算预计剩余时间
			var estimatedRemainingSeconds float64
			if progress > 0 && progress < 100 && syncSpeed > 0 {
				remainingBlocks := networkHeight - localHeight
				estimatedRemainingSeconds = float64(remainingBlocks) / syncSpeed
			}

			if logger != nil {
				logger.Debugf("📈 活跃同步任务详情: RequestID=%s, 目标高度=%d, "+
					"运行时长=%s, 已处理区块=%d, 同步速度=%.2f区块/秒, "+
					"预计剩余时间=%.1f秒, 数据源=%s",
					currentTask.RequestID, currentTask.TargetHeight,
					time.Since(currentTask.StartTime), currentTask.ProcessedBlocks,
					syncSpeed, estimatedRemainingSeconds, safeShortPeerID(currentTask.SourcePeerID))
			}
		} else {
			// 非同步状态但有任务时的信息
			if logger != nil {
				logger.Debugf("💤 同步任务状态: RequestID=%s, 状态=%s, 开始时间=%s",
					currentTask.RequestID, status.String(), currentTask.StartTime.Format("15:04:05"))
			}
		}
	} else {
		// 没有活跃任务的状态信息
		if logger != nil {
			logger.Debugf("ℹ️ 同步状态概览: 状态=%s, 本地高度=%d, 网络高度=%d, 进度=%.1f%%",
				status.String(), localHeight, networkHeight, progress)
		}
	}

	// 更新 Prometheus 同步指标（慢路径调用）
	observeSyncMetrics(syncStatus)

	return syncStatus, nil
}

// ============================================================================
//                           网络高度查询（已移除套壳方法）
// ============================================================================

// 注意：原 queryNetworkHeight 方法已被移除，现在直接使用
// queryNetworkHeightWithKBucket 以避免不必要的套壳调用

// ============================================================================
//                           状态计算逻辑
// ============================================================================

func calculateSyncStatus(localHeight, networkHeight uint64) (types.SystemSyncStatusType, float64) {
	// 🔍 统一进度计算辅助函数
	calcProgress := func(local, network uint64) float64 {
		if network == 0 {
			return 0.0
		}
		p := float64(local) / float64(network) * 100.0
		if p > 100.0 {
			return 100.0
		}
		return p
	}

	// 特殊场景 1：无法获取网络高度（networkHeight == 0）
	if networkHeight == 0 {
		// 本地也没有高度：典型的冷启动/创世场景，视为 Bootstrapping
		if localHeight == 0 {
			return types.SyncStatusBootstrapping, 0.0
		}
		// 本地有高度但看不到网络：降级状态，无法判断是否已同步
		return types.SyncStatusDegraded, 100.0
	}

	// 特殊场景 2：本地高度已不低于网络高度 → 认为已同步
	if localHeight >= networkHeight {
		return types.SyncStatusSynced, 100.0
	}

	// 🔧 **检查是否有活跃同步任务**
	activeSyncMutex.RLock()
	currentTask := activeSyncTask
	activeSyncMutex.RUnlock()

	if currentTask != nil {
		// 有活跃同步任务，返回 syncing 状态和实时进度
		var target uint64 = networkHeight
		if currentTask.TargetHeight > 0 {
			target = currentTask.TargetHeight
		}
		return types.SyncStatusSyncing, calcProgress(localHeight, target)
	}

	// 本地高度低于网络高度，但没有活跃同步任务
	// 区分“冷启动引导中”和“一般降级”两种场景
	if localHeight == 0 {
		// 冷启动，尚未开始同步
		return types.SyncStatusBootstrapping, 0.0
	}

	// 一般情况：本地明显落后网络，但没有触发同步 → 降级状态
	return types.SyncStatusDegraded, calcProgress(localHeight, networkHeight)
}

// helper: safeShortPeerID 返回安全的短PeerID（最多8字符）
func safeShortPeerID(id fmt.Stringer) string {
	var idStr string
	if id != nil {
		idStr = id.String()
	}
	if len(idStr) >= 8 {
		return idStr[:8]
	}
	return idStr
}
