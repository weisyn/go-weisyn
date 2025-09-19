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
	"time"

	"github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	"github.com/weisyn/v1/pkg/interfaces/network"
	"github.com/weisyn/v1/pkg/types"
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
// 参数：
//   - ctx: 上下文对象
//   - chainService: 链服务，用于查询本地高度
//   - kBucketManager: K桶管理器，用于选择节点查询网络高度
//   - network: 网络服务，用于与远程节点通信
//   - logger: 日志记录器
//
// 返回：
//   - *types.SystemSyncStatus: 同步状态信息
//   - error: 查询错误
func checkSyncImpl(
	ctx context.Context,
	chainService blockchain.ChainService,
	routingManager kademlia.RoutingTableManager,
	network network.Network,
	host node.Host,
	configProvider config.Provider,
	logger log.Logger,
) (*types.SystemSyncStatus, error) {
	// 查询本地链状态
	chainInfo, err := chainService.GetChainInfo(ctx)
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

	localHeight := chainInfo.Height
	if logger != nil {
		logger.Debugf("本地区块链高度: %d", localHeight)
	}

	// 查询网络高度（使用与trigger.go一致的节点筛选和查询逻辑）
	var networkHeight uint64 = localHeight // 默认使用本地高度

	// 1. K桶节点选择（与trigger.go一致）
	selectedPeers, err := selectKBucketPeersForSync(ctx, routingManager, host, chainInfo, logger)
	if err != nil {
		if logger != nil {
			logger.Debugf("状态查询-K桶节点选择失败: %v，使用本地高度", err)
		}
	} else if len(selectedPeers) == 0 {
		if logger != nil {
			logger.Debug("状态查询-没有可用节点，使用本地高度")
		}
	} else {
		// 2. 应用UpToDate静默窗口机制，避免频繁查询
		// 注意：目前状态查询时仅检查活跃同步任务状态，未实现完整的时间窗口机制
		// TODO: 如需实现完整的静默窗口，可基于上次查询时间和UpToDateSilenceWindowMins配置

		// 检查上次状态查询是否在静默窗口内
		var shouldQuery bool = true
		activeSyncMutex.RLock()
		if activeSyncTask != nil {
			// 如果有活跃同步任务，则不重复查询
			shouldQuery = false
			if logger != nil {
				logger.Debug("状态查询-存在活跃同步任务，跳过网络高度查询")
			}
		}
		activeSyncMutex.RUnlock()

		if shouldQuery {
			// 3. 使用新的网络高度查询函数（与trigger.go一致）
			height, _, queryErr := queryNetworkHeightFromCandidates(ctx, selectedPeers, network, host, chainInfo, configProvider, logger)
			if queryErr != nil {
				if logger != nil {
					logger.Debugf("状态查询-网络高度查询失败: %v，使用本地高度", queryErr)
				}
			} else {
				networkHeight = height
				if logger != nil {
					logger.Debugf("状态查询-网络高度查询成功: %d", networkHeight)
				}
			}
		}
	}

	if logger != nil {
		logger.Debugf("网络区块链高度: %d", networkHeight)
	}

	// 计算同步进度和状态
	status, progress := calculateSyncStatus(localHeight, networkHeight)

	if logger != nil {
		logger.Debugf("同步状态: %s, 进度: %.2f%%", status.String(), progress)
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
					syncSpeed, estimatedRemainingSeconds, currentTask.SourcePeerID.String()[:8])
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

// calculateSyncStatus 计算同步状态和进度
//
// 🎯 **状态判断逻辑**：
// - 本地高度 == 网络高度：已同步（synced）
// - 本地高度 < 网络高度：需要同步，但状态为空闲（idle）
// - 高度差过大：可能存在问题，但仍标记为空闲
//
// 参数：
//   - localHeight: 本地区块高度
//   - networkHeight: 网络区块高度
//
// 返回：
//   - types.SystemSyncStatusType: 同步状态
//   - float64: 同步进度百分比
func calculateSyncStatus(localHeight, networkHeight uint64) (types.SystemSyncStatusType, float64) {
	// 如果网络高度为0或查询失败，认为已同步
	if networkHeight == 0 {
		return types.SyncStatusSynced, 100.0
	}

	// 计算高度差
	if localHeight >= networkHeight {
		// 本地高度不低于网络高度，认为已同步
		return types.SyncStatusSynced, 100.0
	}

	// 🔧 **检查是否有活跃同步任务**
	activeSyncMutex.RLock()
	currentTask := activeSyncTask
	activeSyncMutex.RUnlock()

	if currentTask != nil {
		// 有活跃同步任务，返回syncing状态和实时进度
		var syncProgress float64
		if currentTask.TargetHeight > 0 {
			// 基于同步任务的目标高度计算进度
			syncProgress = float64(localHeight) / float64(currentTask.TargetHeight) * 100.0
		} else {
			// 如果没有目标高度信息，使用网络高度
			syncProgress = float64(localHeight) / float64(networkHeight) * 100.0
		}

		if syncProgress > 100.0 {
			syncProgress = 100.0
		}

		return types.SyncStatusSyncing, syncProgress
	}

	// 本地高度低于网络高度，但没有活跃同步任务
	progress := float64(localHeight) / float64(networkHeight) * 100.0
	return types.SyncStatusIdle, progress
}
