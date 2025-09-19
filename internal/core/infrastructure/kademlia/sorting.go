package kbucket

import (
	"sort"

	"github.com/libp2p/go-libp2p/core/peer"
)

// peerDistanceSort 是一个辅助结构，用于按照与本地节点的距离对对等节点进行排序
// 基于defs-back/kbucket/sorting.go的设计
type peerDistanceSort struct {
	p        peer.ID // 对等节点的ID
	mode     int     // 对等节点的运行模式
	distance []byte  // 对等节点与本地节点的距离（XOR距离）
}

// peerDistanceSorter 实现sort.Interface接口，用于按照异或距离对对等节点进行排序
type peerDistanceSorter struct {
	peers  []peerDistanceSort // 待排序的对等节点列表
	target []byte             // 排序的目标节点ID，与目标距离最近的节点将排在前面
}

// Len 返回peerDistanceSorter中的节点数量
func (pds *peerDistanceSorter) Len() int {
	return len(pds.peers)
}

// Swap 交换peerDistanceSorter中两个位置的节点
func (pds *peerDistanceSorter) Swap(a, b int) {
	pds.peers[a], pds.peers[b] = pds.peers[b], pds.peers[a]
}

// Less 比较peerDistanceSorter中两个位置的节点的距离大小
func (pds *peerDistanceSorter) Less(a, b int) bool {
	return compareBytes(pds.peers[a].distance, pds.peers[b].distance) < 0
}

// appendPeer 将peer.ID添加到排序器的切片中
func (pds *peerDistanceSorter) appendPeer(p peer.ID, mode int, dhtID []byte) {
	// 计算与目标的距离
	distance := xorDistance(dhtID, pds.target)

	pds.peers = append(pds.peers, peerDistanceSort{
		p:        p,
		mode:     mode,
		distance: distance,
	})
}

// appendPeersFromList 从节点列表中添加节点到排序器
func (pds *peerDistanceSorter) appendPeersFromList(peers []*PeerInfo) {
	for _, p := range peers {
		pds.appendPeer(p.Id, p.Mode, p.dhtId)
	}
}

// sort 对节点列表进行排序
func (pds *peerDistanceSorter) sort() {
	sort.Sort(pds)
}

// SortClosestPeers 根据与目标的距离对节点进行排序
func SortClosestPeers(peers []peer.ID, target []byte) []peer.ID {
	// 创建排序器
	sorter := &peerDistanceSorter{
		target: target,
		peers:  make([]peerDistanceSort, 0, len(peers)),
	}

	// 添加所有节点并计算正确的DHT ID
	for _, p := range peers {
		// 使用与manager.go相同的转换方法
		dhtID := ConvertPeerID(p)
		sorter.appendPeer(p, 0, dhtID)
	}

	// 排序
	sorter.sort()

	// 提取排序后的peer.ID列表
	result := make([]peer.ID, len(sorter.peers))
	for i, pd := range sorter.peers {
		result[i] = pd.p
	}

	return result
}

// compareBytes 比较两个字节数组的大小
// 返回值：< 0 表示 a < b，0 表示 a == b，> 0 表示 a > b
func compareBytes(a, b []byte) int {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	for i := 0; i < minLen; i++ {
		if a[i] < b[i] {
			return -1
		} else if a[i] > b[i] {
			return 1
		}
	}

	// 前缀相同，比较长度
	return len(a) - len(b)
}

// xorDistance 计算两个字节数组的XOR距离
func xorDistance(a, b []byte) []byte {
	if len(a) != len(b) {
		// 如果长度不同，填充较短的数组
		maxLen := len(a)
		if len(b) > maxLen {
			maxLen = len(b)
		}

		paddedA := make([]byte, maxLen)
		paddedB := make([]byte, maxLen)

		copy(paddedA[maxLen-len(a):], a)
		copy(paddedB[maxLen-len(b):], b)

		a, b = paddedA, paddedB
	}

	distance := make([]byte, len(a))
	for i := range a {
		distance[i] = a[i] ^ b[i]
	}

	return distance
}
