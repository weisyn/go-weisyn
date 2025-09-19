// trigger.go - 同步触发主入口
// 负责协调3阶段同步流程：同步触发与节点选择、K桶智能同步、分页补齐同步
// - 使用K桶算法选择最近邻节点
// - 查询网络高度并执行智能同步
// - 处理区块验证和应用流程
package sync

import (
	"context"
	"fmt"
	"time"

	peer "github.com/libp2p/go-libp2p/core/peer"

	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	"github.com/weisyn/v1/pkg/interfaces/network"
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
//                           同步触发实现
// ============================================================================

// triggerSyncImpl 手动触发同步的具体实现
//
// 🎯 **3阶段K桶智能同步策略**：
// 1. 同步触发与节点选择：检查系统状态，使用K桶选择最优节点
// 2. K桶智能同步：获取初始区块批次和网络高度
// 3. 分页补齐同步：使用分页方式同步剩余区块
//
// 参数：
//   - ctx: 上下文对象
//   - chainService: 链服务，用于查询本地状态
//   - blockService: 区块服务，用于验证和处理区块
//   - kBucketManager: K桶管理器，用于节点选择
//   - networkService: 网络服务，用于P2P通信
//   - host: 主机服务，用于节点ID获取和验证
//   - configProvider: 配置提供者，用于获取链ID等配置
//   - logger: 日志记录器
//
// 返回：
//   - error: 同步错误，nil表示成功
func triggerSyncImpl(
	ctx context.Context,
	chainService blockchain.ChainService,
	blockService blockchain.BlockService,
	routingManager kademlia.RoutingTableManager,
	networkService network.Network,
	host node.Host,
	configProvider config.Provider,
	logger log.Logger,
) error {
	if logger != nil {
		logger.Info("[TriggerSync] 🚀 启动标准K桶3阶段同步流程")
	}

	// 生成请求ID
	requestID := fmt.Sprintf("sync-%d", time.Now().UnixNano())

	// ================================
	// 阶段0: 同步冲突检查和锁获取
	// ================================
	if !tryAcquireSyncLock(requestID, logger) {
		return fmt.Errorf("同步任务已在进行中，请等待当前任务完成")
	}
	defer releaseSyncLock(logger)

	// 创建可取消的同步上下文
	syncCtx, cancelFunc := context.WithCancel(ctx)
	defer cancelFunc()

	// ================================
	// 阶段1: 同步触发与节点选择
	// ================================
	if logger != nil {
		logger.Info("[TriggerSync] 📍 阶段1: 同步触发与节点选择")
	}

	// 1.1 系统就绪性检查
	ready, err := chainService.IsReady(syncCtx)
	if err != nil {
		return fmt.Errorf("系统就绪检查失败: %w", err)
	}
	if !ready {
		return fmt.Errorf("系统尚未就绪，无法启动同步")
	}

	// 1.2 获取本地链信息
	localChainInfo, err := chainService.GetChainInfo(syncCtx)
	if err != nil {
		return fmt.Errorf("获取本地区块链信息失败: %w", err)
	}
	localHeight := localChainInfo.Height

	// 1.4 K桶节点选择（基于Kademlia距离算法）
	selectedPeers, err := selectKBucketPeersForSync(syncCtx, routingManager, host, localChainInfo, logger)
	if err != nil {
		if logger != nil {
			logger.Warnf("[TriggerSync] ⚠️ K桶节点选择失败: %v", err)
		}
		return fmt.Errorf("K桶节点选择失败: %w", err)
	}

	if len(selectedPeers) == 0 {
		if logger != nil {
			logger.Warn("[TriggerSync] ⚠️ 没有找到可用的同步节点，可能网络尚未连接")
		}
		return fmt.Errorf("没有可用的同步节点")
	}

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
	var filteredPeers []peer.ID
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

	selectedPeers = filteredPeers

	if logger != nil {
		logger.Infof("[TriggerSync] ✅ 阶段1完成: 本地高度=%d, 候选节点=%d (过滤后=%d)",
			localHeight, len(selectedPeers)+len(filteredPeers), len(selectedPeers))
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
		syncCtx, selectedPeers, networkService, host, localChainInfo, configProvider, logger,
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

	// 1.5.2 记录同步查询结果到缓存
	recordPeerSyncResult(networkSourcePeer, localHeight, networkHeight)

	// 1.5.3 判断是否需要同步
	if networkHeight <= localHeight {
		if logger != nil {
			logger.Info("[TriggerSync] 🎉 节点已与网络同步，无需进一步同步")
			logger.Infof("[TriggerSync] 📊 Sync state: up-to-date (local=%d, remote=%d), no action needed", localHeight, networkHeight)
			if networkHeight == localHeight {
				logger.Info("[TriggerSync] ✅ 高度完全一致，节点与网络保持同步状态")
			} else {
				logger.Info("[TriggerSync] ✅ 本地高度领先，无需同步下载")
			}
		}
		return nil
	}

	// ================================
	// 阶段2: K桶智能同步请求（获取初始区块批次）
	// ================================
	if logger != nil {
		logger.Info("[TriggerSync] 📍 阶段2: K桶智能同步请求")
	}

	// 2.1 执行K桶智能同步（仅获取初始区块批次，不再返回"网络高度"）
	var initialBlocks []*core.Block // 使用proto统一格式
	var sourcePeer peer.ID

	for _, peerID := range selectedPeers {
		initialBlocks, err = performKBucketSmartSync(
			syncCtx, peerID, localHeight, localChainInfo,
			networkService, host, configProvider, logger,
		)
		if err != nil {
			if logger != nil {
				logger.Warnf("[TriggerSync] 节点 %s K桶同步失败: %v", peerID.String()[:8], err)
			}
			continue // 尝试下一个节点
		}

		sourcePeer = peerID
		break
	}

	if err != nil {
		if logger != nil {
			logger.Warnf("[TriggerSync] ❌ 所有候选节点的K桶同步均失败，同步中止")
		}
		return fmt.Errorf("所有K桶节点同步均失败: %w", err)
	}

	if logger != nil {
		logger.Infof("[TriggerSync] ✅ K桶智能同步成功，获得初始区块批次: %d个区块, 数据源: %s",
			len(initialBlocks), sourcePeer.String()[:12]+"...")
	}

	// 2.3 设置活跃同步状态
	setActiveSyncTask(&activeSyncContext{
		RequestID:       requestID,
		StartTime:       time.Now(),
		TargetHeight:    networkHeight,
		SourcePeerID:    sourcePeer,
		CancelFunc:      cancelFunc,
		ProcessedBlocks: 0,
	})

	if logger != nil {
		logger.Infof("[TriggerSync] ✅ 阶段2完成: 网络高度=%d, 初始区块=%d, 数据源=%s",
			networkHeight, len(initialBlocks), sourcePeer.String()[:8])
	}

	// ================================
	// 阶段3: 分页补齐同步
	// ================================
	if logger != nil {
		logger.Info("[TriggerSync] 📍 阶段3: 分页补齐同步")
	}

	// 3.1 处理初始区块（来自K桶智能同步）
	if len(initialBlocks) > 0 {
		if logger != nil {
			logger.Infof("[TriggerSync] 📦 开始处理初始区块批次: %d个区块", len(initialBlocks))
		}
		err = processBlockBatch(syncCtx, initialBlocks, blockService, logger)
		if err != nil {
			if logger != nil {
				logger.Errorf("[TriggerSync] ❌ 初始区块批次处理失败: %v", err)
			}
			return fmt.Errorf("初始区块批次处理失败: %w", err)
		}
		updateSyncProgress(uint64(len(initialBlocks)))
		if logger != nil {
			logger.Infof("[TriggerSync] ✅ 初始区块批次处理完成: %d个区块已应用", len(initialBlocks))
		}
	} else {
		if logger != nil {
			logger.Info("[TriggerSync] 📦 K桶同步未返回初始区块，继续分页同步")
		}
	}

	// 3.2 计算剩余需要同步的高度范围
	currentHeight := localHeight + uint64(len(initialBlocks))
	if networkHeight > currentHeight {
		missingBlocks := networkHeight - currentHeight
		if logger != nil {
			logger.Infof("[TriggerSync] 📏 需要分页同步剩余区块: %d个 (从高度%d到%d)",
				missingBlocks, currentHeight+1, networkHeight)
		}

		// 3.3 执行分页补齐同步（使用所有可用节点进行故障转移）
		availablePeers := []peer.ID{sourcePeer}
		// 添加其他备用节点（排除已使用的sourcePeer）
		for _, peer := range selectedPeers {
			if peer != sourcePeer {
				availablePeers = append(availablePeers, peer)
			}
		}

		if logger != nil {
			logger.Infof("[TriggerSync] 🔄 启动分页补齐同步，可用节点: %d个", len(availablePeers))
		}

		err = performRangePaginatedSync(
			syncCtx, availablePeers, currentHeight, networkHeight,
			networkService, host, blockService, configProvider, logger,
		)
		if err != nil {
			if logger != nil {
				logger.Errorf("[TriggerSync] ❌ 分页补齐同步失败: %v", err)
			}
			return fmt.Errorf("分页补齐同步失败: %w", err)
		}

		if logger != nil {
			logger.Infof("[TriggerSync] ✅ 分页补齐同步完成: 已同步到高度%d", networkHeight)
		}
	} else {
		if logger != nil {
			logger.Info("[TriggerSync] 📏 无需分页同步，初始批次已包含所有缺失区块")
		}
	}

	if logger != nil {
		logger.Info("[TriggerSync] 🎉 标准3阶段同步流程完成！")
	}
	return nil
}

// ============================================================================
//                           配置获取工具函数
// ============================================================================
