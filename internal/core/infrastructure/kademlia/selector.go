package kbucket

import (
	"crypto/sha256"
	"sort"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/libp2p/go-libp2p/core/peer"
)

// KademliaPeerSelector Kademlia节点选择器
// 实现标准的Kademlia节点选择算法
type KademliaPeerSelector struct {
	logger     log.Logger
	calculator kademlia.DistanceCalculator
}

// NewKademliaPeerSelector 创建Kademlia节点选择器
func NewKademliaPeerSelector(logger log.Logger) kademlia.PeerSelector {
	return &KademliaPeerSelector{
		logger:     logger,
		calculator: NewXORDistanceCalculator(logger),
	}
}

// peerDistance 用于排序的节点距离信息
type peerDistance struct {
	peer     peer.ID
	distance []byte
}

// SelectPeers 选择节点，支持基于criteria的筛选
func (ps *KademliaPeerSelector) SelectPeers(candidates []peer.ID, count int, criteria *kademlia.SelectionCriteria) []peer.ID {
	if count <= 0 || len(candidates) == 0 {
		return nil
	}

	if count >= len(candidates) {
		return candidates
	}

	// 如果有selection criteria，先进行预筛选
	filteredCandidates := candidates
	if criteria != nil && len(criteria.TargetKey) > 0 {
		// 基于距离排序候选节点
		filteredCandidates = ps.RankPeers(candidates, criteria.TargetKey)
	}

	// 选择最佳的count个节点
	if count > len(filteredCandidates) {
		count = len(filteredCandidates)
	}

	result := make([]peer.ID, count)
	copy(result, filteredCandidates[:count])

	ps.logger.Debugf("从 %d 个候选节点中选择了 %d 个节点", len(candidates), count)
	return result
}

// RankPeers 根据距离目标的远近对节点进行排序
func (ps *KademliaPeerSelector) RankPeers(peers []peer.ID, targetKey []byte) []peer.ID {
	if len(peers) <= 1 {
		return peers
	}

	ps.logger.Debugf("对 %d 个节点进行距离排序", len(peers))

	// 计算每个节点到目标的距离
	distances := make([]peerDistance, len(peers))
	for i, p := range peers {
		distance := ps.calculator.DistanceToKey(p, targetKey)
		distances[i] = peerDistance{
			peer:     p,
			distance: distance,
		}
	}

	// 按距离排序
	sort.Slice(distances, func(i, j int) bool {
		return ps.calculator.Compare(distances[i].distance, distances[j].distance) < 0
	})

	// 提取排序后的peer.ID列表
	result := make([]peer.ID, len(distances))
	for i, pd := range distances {
		result[i] = pd.peer
	}

	ps.logger.Debugf("节点排序完成")
	return result
}

// FilterPeers 使用给定的过滤器过滤节点
func (ps *KademliaPeerSelector) FilterPeers(peers []peer.ID, filter kademlia.PeerFilter) []peer.ID {
	if filter == nil {
		return peers
	}

	var filtered []peer.ID
	for _, p := range peers {
		if filter(p) {
			filtered = append(filtered, p)
		}
	}

	ps.logger.Debugf("从 %d 个节点中过滤出 %d 个节点", len(peers), len(filtered))
	return filtered
}

// SelectClosestPeers 选择距离目标最近的节点
func SelectClosestPeers(peers []peer.ID, targetKey []byte, count int, logger log.Logger) []peer.ID {
	selector := NewKademliaPeerSelector(logger)
	ranked := selector.RankPeers(peers, targetKey)

	if count > len(ranked) {
		count = len(ranked)
	}

	return ranked[:count]
}

// ComputePeerDistance 计算节点到目标的距离
func ComputePeerDistance(peerID peer.ID, targetKey []byte) []byte {
	// 将peer.ID转换为DHT ID
	peerHash := sha256.Sum256([]byte(peerID))

	// 确保target长度为32字节
	var targetHash [32]byte
	if len(targetKey) == 32 {
		copy(targetHash[:], targetKey)
	} else {
		targetHash = sha256.Sum256(targetKey)
	}

	// 计算XOR距离
	distance := make([]byte, 32)
	for i := range distance {
		distance[i] = peerHash[i] ^ targetHash[i]
	}

	return distance
}
