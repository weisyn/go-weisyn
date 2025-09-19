// calculate_aggregator.go
// 确定性聚合节点选举算法实现
//
// 主要功能：
// 1. 基于Hash(height || SEED) + KademliaClosestPeer的确定性选举算法
// 2. Kademlia距离计算实现
// 3. 确定性routing_key生成
// 4. 最近节点查找算法
//
// 核心算法：
//   routing_key = Hash(height || SEED)  // SEED = 上一确定区块哈希
//   aggregator = KademliaClosestPeer(routing_key)
//
// 设计原则：
// - 确保全网节点计算结果一致性
// - 每个区块高度只有唯一聚合节点
// - 去中心化分布式选举机制
// - 毫秒级高性能选举判断
//
// 作者：WES开发团队
// 创建时间：2025-09-13

package election

import (
	"context"
	"encoding/binary"
	"errors"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/weisyn/v1/pkg/constants/protocols"
	"github.com/weisyn/v1/pkg/interfaces/blockchain"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	netiface "github.com/weisyn/v1/pkg/interfaces/network"
)

// aggregatorCalculator 聚合节点计算器
type aggregatorCalculator struct {
	chainService        blockchain.ChainService
	hashManager         crypto.HashManager
	kbucket             kademlia.DistanceCalculator
	host                node.Host
	networkService      netiface.Network             // 新增：网络服务，用于协议能力检查
	routingTableManager kademlia.RoutingTableManager // 新增：路由表管理器，用于清理外部节点
	logger              log.Logger                   // 新增：日志记录器
}

// newAggregatorCalculator 创建聚合节点计算器
func newAggregatorCalculator(
	chainService blockchain.ChainService,
	hashManager crypto.HashManager,
	kbucket kademlia.DistanceCalculator,
	host node.Host,
	networkService netiface.Network,
	routingTableManager kademlia.RoutingTableManager,
	logger log.Logger,
) *aggregatorCalculator {
	return &aggregatorCalculator{
		chainService:        chainService,
		hashManager:         hashManager,
		kbucket:             kbucket,
		host:                host,
		networkService:      networkService,
		routingTableManager: routingTableManager,
		logger:              logger,
	}
}

// generateRoutingKey 生成确定性路由键
// routing_key = Hash(height || SEED)  // SEED = 上一确定区块哈希
func (calc *aggregatorCalculator) generateRoutingKey(ctx context.Context, height uint64) ([]byte, error) {
	// 获取链信息以获得上一区块哈希作为SEED
	chainInfo, err := calc.chainService.GetChainInfo(ctx)
	if err != nil {
		return nil, errors.New("failed to get chain info")
	}

	// 如果是创世块，使用零哈希作为种子
	seed := chainInfo.BestBlockHash
	if height == 0 {
		seed = make([]byte, 32) // 32字节零哈希
	}

	calc.logger.Debugf("🔑 生成路由键: height=%d, seed=%x", height, seed[:8])

	// 构造 height || SEED
	heightBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(heightBytes, height)

	// 拼接高度和种子
	data := append(heightBytes, seed...)

	// 计算SHA256哈希作为路由键
	routingKey := calc.hashManager.SHA256(data)

	calc.logger.Debugf("🎯 路由键生成完成: routing_key=%x", routingKey[:8])

	return routingKey, nil
}

// getAggregatorForHeight 获取指定高度的聚合节点
func (calc *aggregatorCalculator) getAggregatorForHeight(ctx context.Context, height uint64) (peer.ID, error) {
	// 生成路由键
	routingKey, err := calc.generateRoutingKey(ctx, height)
	if err != nil {
		return "", err
	}

	// 查找最近的节点
	aggregatorID, err := calc.selectClosestPeer(ctx, routingKey)
	if err != nil {
		return "", err
	}

	return aggregatorID, nil
}

// isAggregatorForHeight 判断当前节点是否为指定高度的聚合节点
func (calc *aggregatorCalculator) isAggregatorForHeight(ctx context.Context, height uint64) (bool, error) {
	// 获取当前节点ID
	localPeerID := calc.host.ID()

	calc.logger.Debugf("🤔 判断节点 %s 是否为高度 %d 的聚合器", localPeerID, height)

	// 获取该高度的聚合节点
	aggregatorID, err := calc.getAggregatorForHeight(ctx, height)
	if err != nil {
		calc.logger.Errorf("❌ 获取高度 %d 的聚合器失败: %v", height, err)
		return false, err
	}

	// 判断是否为当前节点
	isAggregator := localPeerID == aggregatorID

	if isAggregator {
		calc.logger.Infof("✅ 确认本节点 %s 是高度 %d 的聚合器", localPeerID, height)
	} else {
		calc.logger.Infof("❌ 本节点 %s 不是高度 %d 的聚合器，实际聚合器: %s", localPeerID, height, aggregatorID)
	}

	return isAggregator, nil
}

// selectClosestPeer 基于Kademlia距离选择最近的节点（使用K桶系统）
func (calc *aggregatorCalculator) selectClosestPeer(ctx context.Context, routingKey []byte) (peer.ID, error) {
	// 获取当前节点ID
	localPeerID := calc.host.ID()

	// 🎯 使用K桶管理器获取节点列表（标准化的网络拓扑）
	var kBucketPeers []peer.ID

	if calc.routingTableManager != nil {
		// 使用RoutingTableManager的FindClosestPeers方法
		kBucketPeers = calc.routingTableManager.FindClosestPeers(routingKey, 20)
		calc.logger.Infof("🗂️  从K桶获取到 %d 个候选节点", len(kBucketPeers))
	} else {
		calc.logger.Warn("⚠️  K桶管理器不可用，将只考虑当前节点")
	}

	// 🔒 关键安全过滤：只考虑支持WES协议的节点
	validPeers := []peer.ID{}
	removedExternalNodes := []peer.ID{}

	calc.logger.Debugf("🔍 开始过滤K桶节点，总数: %d", len(kBucketPeers))

	// 🔧 首先将自己添加到候选节点（确保算法一致性）
	allCandidates := []peer.ID{localPeerID}
	allCandidates = append(allCandidates, kBucketPeers...)

	for _, peerID := range allCandidates {
		// 🔒 协议能力检查：验证节点是否支持共识协议
		// 注意：自己节点总是支持协议，无需检查
		var supported bool
		var err error

		if peerID == localPeerID {
			supported = true // 自己节点总是支持
			calc.logger.Debugf("✅ 节点 %s 是本地节点，自动通过协议检查", peerID)
		} else {
			supported, err = calc.networkService.CheckProtocolSupport(ctx, peerID, protocols.ProtocolBlockSubmission)
		}

		if err != nil {
			calc.logger.Warnf("⚠️  节点 %s 协议检查失败，跳过: %v", peerID, err)
			continue
		}

		if !supported {
			calc.logger.Warnf("🚫 发现外部节点 %s（不支持WES协议），跳过聚合器选择", peerID)

			// 🧹 从K桶中移除外部节点（如果路由表管理器可用）
			if calc.routingTableManager != nil {
				if err := calc.routingTableManager.RemovePeer(peerID); err != nil {
					calc.logger.Errorf("从K桶移除外部节点 %s 失败: %v", peerID, err)
				} else {
					calc.logger.Infof("✅ 成功从K桶移除外部节点: %s", peerID)
					removedExternalNodes = append(removedExternalNodes, peerID)
				}
			}
			continue
		}

		// ✅ 节点通过协议检查，加入候选列表
		validPeers = append(validPeers, peerID)
	}

	calc.logger.Infof("🔒 聚合器候选节点过滤完成: K桶=%d, 包含自己后=%d, 有效=%d, 移除外部节点=%d",
		len(kBucketPeers), len(allCandidates), len(validPeers), len(removedExternalNodes))

	// 🎯 对所有有效节点（包括自己）进行距离计算
	var closestPeer peer.ID
	var closestDistance []byte

	calc.logger.Debugf("🧮 开始计算所有候选节点到routing_key的距离，候选数: %d", len(validPeers))

	for i, peerID := range validPeers {
		// 计算该节点到routing_key的距离
		distance := calc.kbucket.DistanceToKey(peerID, routingKey)

		calc.logger.Debugf("📏 节点 %s 距离计算: %x", peerID, distance[:8]) // 显示前8字节

		// 第一个节点或找到更近的节点时更新
		if i == 0 || calc.kbucket.Compare(distance, closestDistance) < 0 {
			if closestPeer != "" {
				calc.logger.Debugf("🎯 找到更近的聚合器候选: %s (替换 %s)", peerID, closestPeer)
			} else {
				calc.logger.Debugf("🎯 初始聚合器候选: %s", peerID)
			}
			closestPeer = peerID
			closestDistance = distance
		}
	}

	if closestPeer == localPeerID {
		calc.logger.Infof("🏆 最终选择的聚合器: %s (本地节点) - 从%d个候选节点中选出", closestPeer, len(validPeers))
	} else {
		calc.logger.Infof("🏆 最终选择的聚合器: %s (远程节点) - 从%d个候选节点中选出", closestPeer, len(validPeers))
	}
	return closestPeer, nil
}
