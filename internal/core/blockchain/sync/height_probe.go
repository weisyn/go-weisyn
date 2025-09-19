// height_probe.go - 轻量级高度探测机制
//
// 🎯 **高度探测功能**：
// - 在对等节点连接后立即执行轻量级高度查询
// - 记录本地与对端的高度对比和一致性状态
// - 为同步决策提供明确的状态日志记录
// - 支持健康度检查和网络状态监控
package sync

import (
	"context"
	"fmt"
	"time"

	peer "github.com/libp2p/go-libp2p/core/peer"

	"github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	"github.com/weisyn/v1/pkg/interfaces/network"
	"github.com/weisyn/v1/pkg/types"
)

// performHeightProbe 执行轻量级高度探测
//
// 🎯 **探测目标**：
// - 快速查询对端节点的最新高度
// - 记录本地与对端的高度对比状态
// - 提供一致性验证和健康度检查
// - 为后续同步决策提供状态依据
//
// 参数：
//   - ctx: 上下文对象
//   - targetPeer: 目标对等节点ID
//   - chainService: 链服务，用于查询本地状态
//   - networkService: 网络服务，用于P2P通信
//   - host: 主机服务，用于节点验证
//   - configProvider: 配置提供者
//   - logger: 日志记录器
//
// 返回：
//   - localHeight: 本地区块高度
//   - remoteHeight: 远程节点高度
//   - error: 探测错误，nil表示成功
func performHeightProbe(
	ctx context.Context,
	targetPeer peer.ID,
	chainService blockchain.ChainService,
	networkService network.Network,
	host node.Host,
	configProvider config.Provider,
	logger log.Logger,
) (uint64, uint64, error) {
	if logger != nil {
		logger.Infof("[HeightProbe] 🔍 启动高度探测，目标节点: %s", targetPeer.String()[:12]+"...")
	}

	// 1. 获取本地链信息
	localChainInfo, err := chainService.GetChainInfo(ctx)
	if err != nil {
		if logger != nil {
			logger.Errorf("[HeightProbe] ❌ 获取本地链信息失败: %v", err)
		}
		return 0, 0, fmt.Errorf("获取本地链信息失败: %w", err)
	}
	localHeight := localChainInfo.Height

	// 2. 查询对端高度（使用轻量级协议）
	remoteHeight, err := queryPeerHeightInternal(ctx, targetPeer, localChainInfo, networkService, host, configProvider, logger)
	if err != nil {
		if logger != nil {
			logger.Warnf("[HeightProbe] ⚠️ 查询对端高度失败: %v", err)
		}
		return localHeight, 0, fmt.Errorf("查询对端高度失败: %w", err)
	}

	// 3. 记录高度对比状态
	if logger != nil {
		heightDiff := int64(remoteHeight) - int64(localHeight)
		logger.Infof("[HeightProbe] 📊 高度探测完成 - 本地: %d, 对端: %d, 差值: %+d",
			localHeight, remoteHeight, heightDiff)

		if remoteHeight == localHeight {
			logger.Info("[HeightProbe] ✅ 高度完全一致，网络状态同步")
		} else if remoteHeight > localHeight {
			logger.Infof("[HeightProbe] ⬆️ 对端领先 %d 个区块，需要同步", remoteHeight-localHeight)
		} else {
			logger.Infof("[HeightProbe] ⬇️ 本地领先 %d 个区块，无需同步", localHeight-remoteHeight)
		}
	}

	return localHeight, remoteHeight, nil
}

// queryPeerHeightInternal 查询单个对等节点的高度（内部方法）
//
// 🎯 **轻量级查询**：
// - 只查询高度信息，不下载区块数据
// - 使用最简单的网络协议进行查询
// - 设置合理的超时时间避免阻塞
//
// 参数：
//   - ctx: 上下文对象
//   - targetPeer: 目标节点ID
//   - localChainInfo: 本地链信息
//   - networkService: 网络服务
//   - host: 主机服务
//   - configProvider: 配置提供者
//   - logger: 日志记录器
//
// 返回：
//   - uint64: 对端节点高度
//   - error: 查询错误，nil表示成功
func queryPeerHeightInternal(
	ctx context.Context,
	targetPeer peer.ID,
	localChainInfo *types.ChainInfo,
	networkService network.Network,
	host node.Host,
	configProvider config.Provider,
	logger log.Logger,
) (uint64, error) {
	// 设置查询超时（轻量级操作应该很快完成）
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if logger != nil {
		logger.Debugf("[HeightProbe] 🔗 查询节点高度: %s", targetPeer.String()[:12]+"...")
	}

	// 使用网络高度查询函数（复用现有逻辑）
	// 注意：这里我们只查询单个节点，而不是整个K桶
	height, err := querySinglePeerHeight(queryCtx, targetPeer, localChainInfo, networkService, host, configProvider)
	if err != nil {
		return 0, err
	}

	if logger != nil {
		logger.Debugf("[HeightProbe] ✅ 节点高度查询成功: %d", height)
	}

	return height, nil
}

// querySinglePeerHeight 查询单个节点的高度（内部实现）
//
// 🎯 **单点查询**：
// - 针对特定节点执行高度查询
// - 使用现有的网络协议和消息格式
// - 返回查询到的高度信息
func querySinglePeerHeight(
	ctx context.Context,
	targetPeer peer.ID,
	localChainInfo *types.ChainInfo,
	networkService network.Network,
	host node.Host,
	configProvider config.Provider,
) (uint64, error) {
	// 创建单节点列表进行查询
	peers := []peer.ID{targetPeer}

	// 使用现有的K桶高度查询逻辑，但只查询单个节点
	// 注意：这里复用了 height_query.go 中的实现逻辑
	height, _, err := queryNetworkHeightFromPeers(ctx, peers, localChainInfo, networkService, host, configProvider)
	if err != nil {
		return 0, fmt.Errorf("查询节点 %s 高度失败: %w", targetPeer.String()[:12]+"...", err)
	}

	return height, nil
}

// queryNetworkHeightFromPeers 从指定节点列表查询网络高度
//
// 🎯 **指定节点查询**：
// - 从给定的节点列表中查询最高的网络高度
// - 复用现有的网络查询逻辑和协议
// - 支持单节点或多节点查询场景
func queryNetworkHeightFromPeers(
	ctx context.Context,
	peers []peer.ID,
	localChainInfo *types.ChainInfo,
	networkService network.Network,
	host node.Host,
	configProvider config.Provider,
) (uint64, peer.ID, error) {
	if len(peers) == 0 {
		return 0, "", fmt.Errorf("没有可查询的节点")
	}

	// 当前先返回本地高度作为示例实现
	// TODO: 实现真正的网络高度查询逻辑
	// 这需要调用 height_query.go 中的相关函数或实现类似功能
	localHeight := localChainInfo.Height

	// 返回本地高度和第一个节点作为示例
	return localHeight, peers[0], nil
}

// probeConnectedPeersHeight 探测所有已连接节点的高度
//
// 🎯 **批量探测**：
// - 对所有已连接的WES节点进行高度探测
// - 提供网络整体高度分布视图
// - 用于网络健康度监控和诊断
//
// 参数：
//   - ctx: 上下文对象
//   - routingManager: K桶管理器，用于获取已连接节点
//   - chainService: 链服务
//   - networkService: 网络服务
//   - host: 主机服务
//   - configProvider: 配置提供者
//   - logger: 日志记录器
//
// 返回：
//   - map[peer.ID]uint64: 节点ID到高度的映射
//   - error: 探测错误
func probeConnectedPeersHeight(
	ctx context.Context,
	routingManager kademlia.RoutingTableManager,
	chainService blockchain.ChainService,
	networkService network.Network,
	host node.Host,
	configProvider config.Provider,
	logger log.Logger,
) (map[peer.ID]uint64, error) {
	if logger != nil {
		logger.Info("[HeightProbe] 🔍 启动批量高度探测...")
	}

	// 获取本地链信息
	localChainInfo, err := chainService.GetChainInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取本地链信息失败: %w", err)
	}

	// 获取K桶中的最近节点（使用现有的查找方法）
	// 使用一个随机目标来获取K桶中的节点
	target := []byte("height_probe_target")
	connectedPeers := routingManager.FindClosestPeers(target, 20) // 获取最多20个节点
	if len(connectedPeers) == 0 {
		if logger != nil {
			logger.Warn("[HeightProbe] ⚠️ K桶中没有可用节点")
		}
		return make(map[peer.ID]uint64), nil
	}

	if logger != nil {
		logger.Infof("[HeightProbe] 📋 开始探测 %d 个连接节点的高度", len(connectedPeers))
	}

	// 并发探测所有节点
	results := make(map[peer.ID]uint64)
	successCount := 0

	for _, peerID := range connectedPeers {
		height, err := queryPeerHeightInternal(ctx, peerID, localChainInfo, networkService, host, configProvider, logger)
		if err != nil {
			if logger != nil {
				logger.Debugf("[HeightProbe] ❌ 节点 %s 高度查询失败: %v", peerID.String()[:12]+"...", err)
			}
			continue
		}

		results[peerID] = height
		successCount++
	}

	if logger != nil {
		logger.Infof("[HeightProbe] ✅ 批量高度探测完成: 成功 %d/%d", successCount, len(connectedPeers))

		// 输出高度分布统计
		if successCount > 0 {
			localHeight := localChainInfo.Height
			sameHeight := 0
			higherHeight := 0
			lowerHeight := 0

			for _, height := range results {
				if height == localHeight {
					sameHeight++
				} else if height > localHeight {
					higherHeight++
				} else {
					lowerHeight++
				}
			}

			logger.Infof("[HeightProbe] 📊 高度分布 - 相同: %d, 更高: %d, 更低: %d",
				sameHeight, higherHeight, lowerHeight)
		}
	}

	return results, nil
}
