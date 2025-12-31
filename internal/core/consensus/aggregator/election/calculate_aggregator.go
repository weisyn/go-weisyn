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
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	kbucketimpl "github.com/weisyn/v1/internal/core/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/constants/protocols"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	netiface "github.com/weisyn/v1/pkg/interfaces/network"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
)

// aggregatorCalculator 聚合节点计算器
type aggregatorCalculator struct {
	chainQuery          persistence.ChainQuery
	hashManager         crypto.HashManager
	kbucket             kademlia.DistanceCalculator
	p2pService          p2pi.Service
	networkService      netiface.Network             // 新增：网络服务，用于协议能力检查
	routingTableManager kademlia.RoutingTableManager // 新增：路由表管理器，用于清理外部节点
	logger              log.Logger                   // 新增：日志记录器
}

// newAggregatorCalculator 创建聚合节点计算器
func newAggregatorCalculator(
	chainQuery persistence.ChainQuery,
	hashManager crypto.HashManager,
	kbucket kademlia.DistanceCalculator,
	p2pService p2pi.Service,
	networkService netiface.Network,
	routingTableManager kademlia.RoutingTableManager,
	logger log.Logger,
) *aggregatorCalculator {
	return &aggregatorCalculator{
		chainQuery:          chainQuery,
		hashManager:         hashManager,
		kbucket:             kbucket,
		p2pService:          p2pService,
		networkService:      networkService,
		routingTableManager: routingTableManager,
		logger:              logger,
	}
}

// generateRoutingKey 生成确定性路由键
// routing_key = Hash(height || SEED)  // SEED = 固定零哈希（早期区块）或上一区块哈希
func (calc *aggregatorCalculator) generateRoutingKey(ctx context.Context, height uint64) ([]byte, error) {
	var seed []byte

	// 使用固定全零种子，确保无论节点当前高度如何都能得到一致的 routing_key。
	// 之前依赖本地链尖哈希，在节点不同步时会导致种子差异，从而选出不同的聚合器并互相转发。
	// 固定种子 + 高度哈希仍然能带来足够的轮换性，同时保证全网完全确定。
	seed = make([]byte, 32)
	calc.logger.Debugf("🔑 生成路由键: height=%d, 使用全局固定种子（zero-hash）", height)

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
	localPeerID := calc.p2pService.Host().ID()

	calc.logger.Infof("🤔 开始聚合器选举判断: 高度=%d, 本地节点=%s", height, localPeerID)

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
		calc.logger.Infof("ℹ️  本节点 %s 不是高度 %d 的聚合器，实际聚合器: %s", localPeerID, height, aggregatorID)
	}

	return isAggregator, nil
}

// selectClosestPeer 基于Kademlia距离选择最近的节点（使用K桶系统）
func (calc *aggregatorCalculator) selectClosestPeer(ctx context.Context, routingKey []byte) (peer.ID, error) {
	// 获取当前节点ID
	localPeerID := calc.p2pService.Host().ID()

	// 🎯 使用K桶管理器获取节点列表（标准化的网络拓扑）
	var kBucketPeers []peer.ID

	if calc.routingTableManager != nil {
		// ✅ 生产级：从候选集合中剔除“不支持区块提交协议”的 peer。
		// 关键约束：该过滤必须是纯本地快路径（只读 peerstore 协议缓存），不得 DialPeer。
		//
		// 说明：
		// - 之前使用 networkService.CheckProtocolSupport 会触发 DialPeer，导致选举热路径卡死；
		// - 这里改为由 kbucketimpl 在路由表侧提供“支持协议的最近邻”选择能力。
		if rm, ok := calc.routingTableManager.(*kbucketimpl.RoutingTableManager); ok {
			kBucketPeers = rm.FindClosestPeersForProtocol(routingKey, 20, protocols.ProtocolBlockSubmission)
		} else {
			// 防御：若不是我们自研实现，回退为原始 FindClosestPeers（但不做协议探测过滤）
			kBucketPeers = calc.routingTableManager.FindClosestPeers(routingKey, 20)
		}
		calc.logger.Infof("🗂️  从K桶获取到 %d 个候选节点（已按协议过滤）", len(kBucketPeers))
	} else {
		calc.logger.Warn("⚠️  K桶管理器不可用，将只考虑当前节点")
	}

	// 🔧 将自己添加到候选节点（确保算法一致性）
	validPeers := []peer.ID{localPeerID}
	validPeers = append(validPeers, kBucketPeers...)

	calc.logger.Infof("🔒 聚合器候选节点集合: K桶候选=%d, 包含自己后=%d",
		len(kBucketPeers), len(validPeers))

	// 打印所有候选节点的详细信息
	for i, peerID := range validPeers {
		distance := calc.kbucket.DistanceToKey(peerID, routingKey)
		isLocal := peerID == calc.p2pService.Host().ID()
		nodeType := "远程节点"
		if isLocal {
			nodeType = "本地节点"
		}
		calc.logger.Infof("📋 候选节点[%d]: %s (%s) - 距离=%x", i+1, peerID, nodeType, distance[:8])
	}

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
		calc.logger.Infof("📊 聚合器选举详情: 路由键=%x, 本地距离=%x, 候选节点数=%d",
			routingKey[:8], closestDistance[:8], len(validPeers))
	} else {
		calc.logger.Infof("🏆 最终选择的聚合器: %s (远程节点) - 从%d个候选节点中选出", closestPeer, len(validPeers))
		calc.logger.Infof("📊 聚合器选举详情: 路由键=%x, 远程距离=%x, 候选节点数=%d",
			routingKey[:8], closestDistance[:8], len(validPeers))
	}
	return closestPeer, nil
}

// getAggregatorForHeightWithWaivers 获取指定高度的聚合节点（排除弃权节点）
//
// V2 新增：支持弃权与重选机制
// 用于区块转发时确定目标聚合节点，排除已知弃权的节点，避免回环
func (calc *aggregatorCalculator) getAggregatorForHeightWithWaivers(
	ctx context.Context,
	height uint64,
	waivedAggregators []peer.ID,
) (peer.ID, error) {
	// 1. 生成确定性路由键（与原始选举一致）
	routingKey, err := calc.generateRoutingKey(ctx, height)
	if err != nil {
		return "", err
	}

	// 2. 从K桶获取候选节点（支持协议的最近邻）
	var kBucketPeers []peer.ID
	if calc.routingTableManager != nil {
		if rm, ok := calc.routingTableManager.(*kbucketimpl.RoutingTableManager); ok {
			kBucketPeers = rm.FindClosestPeersForProtocol(routingKey, 20, protocols.ProtocolBlockSubmission)
		} else {
			kBucketPeers = calc.routingTableManager.FindClosestPeers(routingKey, 20)
		}
		calc.logger.Infof("🗂️  从K桶获取到 %d 个候选节点（已按协议过滤）", len(kBucketPeers))
	} else {
		calc.logger.Warn("⚠️  K桶管理器不可用，将只考虑当前节点")
	}

	// 3. 过滤弃权节点
	waivedSet := make(map[peer.ID]bool)
	for _, waived := range waivedAggregators {
		waivedSet[waived] = true
	}

	validPeers := []peer.ID{}
	localPeerID := calc.p2pService.Host().ID()
	for _, peerID := range kBucketPeers {
		if !waivedSet[peerID] {
			validPeers = append(validPeers, peerID)
		}
	}

	calc.logger.Infof("🔒 过滤弃权节点后: 原始候选=%d, 弃权节点=%d, 有效候选=%d",
		len(kBucketPeers), len(waivedAggregators), len(validPeers))

	// 4. 如果所有K桶候选都已弃权，检查是否包含自己
	if len(validPeers) == 0 {
		// 如果自己不在弃权列表中，返回自己（回环兜底）
		if !waivedSet[localPeerID] {
			calc.logger.Infof("🔄 所有K桶候选都已弃权，回环到原始矿工: %s", localPeerID)
			return localPeerID, nil
		}
		// 如果自己也在弃权列表中，返回错误（理论上不应发生）
		return "", fmt.Errorf("all candidates waived including self")
	}

	// 5. 将自己添加到候选节点（如果自己不在弃权列表中）
	if !waivedSet[localPeerID] {
		validPeers = append([]peer.ID{localPeerID}, validPeers...)
	}

	// 6. 计算距离并选择最近邻（排除弃权节点后）
	var closestPeer peer.ID
	var closestDistance []byte

	for i, peerID := range validPeers {
		distance := calc.kbucket.DistanceToKey(peerID, routingKey)
		if i == 0 || calc.kbucket.Compare(distance, closestDistance) < 0 {
			closestPeer = peerID
			closestDistance = distance
		}
	}

	if closestPeer == localPeerID {
		calc.logger.Infof("🏆 重选后的聚合器: %s (本地节点) - 从%d个有效候选节点中选出", closestPeer, len(validPeers))
	} else {
		calc.logger.Infof("🏆 重选后的聚合器: %s (远程节点) - 从%d个有效候选节点中选出", closestPeer, len(validPeers))
	}

	return closestPeer, nil
}
