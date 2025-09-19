package kbucket

import (
	"container/list"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// PeerInfo 包含了K-Bucket中一个对等节点的所有相关信息
// 严格按照defs-back/kbucket/bucket.go的PeerInfo结构
type PeerInfo struct {
	Id                            peer.ID   // 对等节点的唯一标识符
	Mode                          int       // 对等节点的运行模式(如DHT服务器/客户端)
	LastUsefulAt                  time.Time // 对等节点上次对我们有用的时间点
	LastSuccessfulOutboundQueryAt time.Time // 我们最后一次从该对等节点获得成功查询响应的时间点
	AddedAt                       time.Time // 该对等节点被添加到路由表的时间点
	dhtId                         []byte    // 对等节点在DHT XOR密钥空间中的ID
	replaceable                   bool      // 当桶已满时,该对等节点是否可以被替换以容纳新节点
}

// Bucket 是一个对等节点列表
// 所有对bucket的访问都在路由表的锁保护下进行同步,
// 因此bucket本身不需要任何锁
// 这与defs-back/kbucket/bucket.go的设计完全一致
type Bucket struct {
	list *list.List // 存储对等节点信息的双向链表
}

// newBucket 创建并返回一个新的空bucket
// 遵循defs-back/kbucket/bucket.go的newBucket函数
func newBucket() *Bucket {
	return &Bucket{
		list: list.New(),
	}
}

// getPeers 返回桶中所有对等节点的信息列表
// 返回的是一个防御性副本,调用者可以安全地修改返回的切片
// 对应defs-back/kbucket/bucket.go的peers()方法
func (b *Bucket) getPeers() []*PeerInfo {
	peers := make([]*PeerInfo, 0, b.list.Len())
	for e := b.list.Front(); e != nil; e = e.Next() {
		p := e.Value.(*PeerInfo)
		// 返回副本以保证安全
		peerCopy := *p
		peers = append(peers, &peerCopy)
	}
	return peers
}

// len 返回桶中对等节点的数量
func (b *Bucket) len() int {
	return b.list.Len()
}

// getPeerIds 返回桶中所有对等节点的ID列表
// 对应defs-back/kbucket/bucket.go的peerIds()方法
func (b *Bucket) getPeerIds() []peer.ID {
	ids := make([]peer.ID, 0, b.list.Len())
	for e := b.list.Front(); e != nil; e = e.Next() {
		p := e.Value.(*PeerInfo)
		ids = append(ids, p.Id)
	}
	return ids
}

// find 在桶中查找指定ID的对等节点
// 对应defs-back/kbucket/bucket.go的find()方法
func (b *Bucket) find(id peer.ID) *list.Element {
	for e := b.list.Front(); e != nil; e = e.Next() {
		p := e.Value.(*PeerInfo)
		if p.Id == id {
			return e
		}
	}
	return nil
}

// moveToFront 将指定的元素移动到链表前端
// 实现LRU机制,最近使用的节点移到前端
func (b *Bucket) moveToFront(e *list.Element) {
	b.list.MoveToFront(e)
}

// pushFront 在桶前端添加新的对等节点
func (b *Bucket) pushFront(p *PeerInfo) {
	b.list.PushFront(p)
}

// remove 从桶中移除指定的元素
func (b *Bucket) remove(e *list.Element) {
	b.list.Remove(e)
}

// split 分割桶
// 对应defs-back/kbucket/bucket.go的split()方法
// 这是Kademlia算法中的关键操作
func (b *Bucket) split(cpl int, target []byte) *Bucket {
	newBucket := newBucket()

	// 遍历当前桶中的所有节点
	for e := b.list.Front(); e != nil; {
		p := e.Value.(*PeerInfo)
		next := e.Next()

		// 计算节点与目标的公共前缀长度
		// 如果节点应该移动到新桶中
		if commonPrefixLength(p.dhtId, target) > cpl {
			b.list.Remove(e)
			newBucket.list.PushBack(p)
		}

		e = next
	}

	return newBucket
}

// min 根据给定的比较函数返回桶中的"最小"对等节点
// 对应defs-back/kbucket/bucket.go的min()方法
func (b *Bucket) min(lessThan func(p1 *PeerInfo, p2 *PeerInfo) bool) *PeerInfo {
	if b.list.Len() == 0 {
		return nil
	}

	minVal := b.list.Front().Value.(*PeerInfo)

	for e := b.list.Front().Next(); e != nil; e = e.Next() {
		val := e.Value.(*PeerInfo)
		if lessThan(val, minVal) {
			minVal = val
		}
	}

	return minVal
}

// updateAllWith 使用给定的更新函数更新桶中的所有对等节点
// 对应defs-back/kbucket/bucket.go的updateAllWith()方法
func (b *Bucket) updateAllWith(updateFnc func(p *PeerInfo)) {
	for e := b.list.Front(); e != nil; e = e.Next() {
		val := e.Value.(*PeerInfo)
		updateFnc(val)
	}
}

// commonPrefixLength 计算两个字节数组的公共前缀长度
// 这是Kademlia距离计算的核心算法
func commonPrefixLength(a, b []byte) int {
	if len(a) != len(b) {
		return 0
	}

	for i := 0; i < len(a); i++ {
		xor := a[i] ^ b[i]
		if xor == 0 {
			continue
		}

		// 找到第一个不同的位
		for j := 7; j >= 0; j-- {
			if (xor>>j)&1 == 1 {
				return i*8 + (7 - j)
			}
		}
	}

	return len(a) * 8
}
