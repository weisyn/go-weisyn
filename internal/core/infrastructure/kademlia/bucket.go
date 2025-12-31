package kbucket

import (
	"container/list"
	"math"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// PeerState 节点健康状态枚举
type PeerState int

const (
	// PeerStateActive 活跃状态：节点可用且健康
	PeerStateActive PeerState = iota
	// PeerStateSuspect 怀疑状态：累计失败达到阈值，但仍可选用（低优先级）
	PeerStateSuspect
	// PeerStateQuarantined 隔离状态：持续失败或断连，不参与选择，保留探测
	PeerStateQuarantined
	// PeerStateEvicted 驱逐状态：长期不可达且桶有余量，待清理
	PeerStateEvicted
)

// String 返回状态的字符串表示
func (s PeerState) String() string {
	switch s {
	case PeerStateActive:
		return "Active"
	case PeerStateSuspect:
		return "Suspect"
	case PeerStateQuarantined:
		return "Quarantined"
	case PeerStateEvicted:
		return "Evicted"
	default:
		return "Unknown"
	}
}

// PeerProbeStatus 探测状态枚举（用于清理前探测机制）
type PeerProbeStatus int

const (
	// ProbeNotNeeded 不需要探测
	ProbeNotNeeded PeerProbeStatus = iota
	// ProbePending 待探测（标记为待清理，需要探测确认）
	ProbePending
	// ProbeSuccess 探测成功（取消清理，恢复Active）
	ProbeSuccess
	// ProbeFailed 探测失败（确认清理）
	ProbeFailed
)

// String 返回探测状态的字符串表示
func (s PeerProbeStatus) String() string {
	switch s {
	case ProbeNotNeeded:
		return "NotNeeded"
	case ProbePending:
		return "Pending"
	case ProbeSuccess:
		return "Success"
	case ProbeFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}

// PeerInfo 包含了K-Bucket中一个对等节点的所有相关信息
// 严格按照defs-back/kbucket/bucket.go的PeerInfo结构，并扩展健康管理字段
type PeerInfo struct {
	// 原始Kademlia字段
	Id                            peer.ID   // 对等节点的唯一标识符
	Mode                          int       // 对等节点的运行模式(如DHT服务器/客户端)
	LastUsefulAt                  time.Time // 对等节点上次对我们有用的时间点
	LastSuccessfulOutboundQueryAt time.Time // 我们最后一次从该对等节点获得成功查询响应的时间点
	AddedAt                       time.Time // 该对等节点被添加到路由表的时间点
	dhtId                         []byte    // 对等节点在DHT XOR密钥空间中的ID
	replaceable                   bool      // 当桶已满时,该对等节点是否可以被替换以容纳新节点

	// 健康管理扩展字段
	peerState          PeerState // 节点健康状态
	failureCount       int       // 失败计数（滑动窗口内）
	healthScore        float64   // 健康分（0-100，指数衰减）
	lastFailureAt      time.Time // 最近一次失败时间
	lastHealthUpdateAt time.Time // 上次健康分更新时间（用于Δt维护）
	quarantinedUntil   time.Time // 隔离截止时间（零值表示未隔离）
	
	// 清理前探测字段（Phase 2）
	probeStatus    PeerProbeStatus // 探测状态
	lastProbeAt    time.Time       // 上次探测时间
	probeFailCount int             // 连续探测失败次数
	
	stateLock sync.RWMutex
}

// GetState 获取节点状态（线程安全）
func (p *PeerInfo) GetState() PeerState {
	p.stateLock.RLock()
	defer p.stateLock.RUnlock()
	return p.peerState
}

// SetState 设置节点状态（线程安全）
func (p *PeerInfo) SetState(state PeerState) {
	p.stateLock.Lock()
	defer p.stateLock.Unlock()
	p.peerState = state
}

// GetHealthScore 获取健康分（线程安全）
func (p *PeerInfo) GetHealthScore() float64 {
	p.stateLock.RLock()
	defer p.stateLock.RUnlock()
	return p.healthScore
}

// IsQuarantined 检查是否在隔离期内（线程安全）
func (p *PeerInfo) IsQuarantined() bool {
	p.stateLock.RLock()
	defer p.stateLock.RUnlock()
	return p.peerState == PeerStateQuarantined && time.Now().Before(p.quarantinedUntil)
}

// RecordFailure 记录失败并更新状态
func (p *PeerInfo) RecordFailure(failureThreshold int, quarantineDuration time.Duration) {
	p.stateLock.Lock()
	defer p.stateLock.Unlock()

	p.failureCount++
	p.lastFailureAt = time.Now()
	p.healthScore = max(0, p.healthScore-10) // 每次失败扣10分

	// 状态转换逻辑
	switch p.peerState {
	case PeerStateActive:
		if p.failureCount >= failureThreshold {
			p.peerState = PeerStateSuspect
		}
	case PeerStateSuspect:
		if p.failureCount >= failureThreshold*2 {
			p.peerState = PeerStateQuarantined
			p.quarantinedUntil = time.Now().Add(quarantineDuration)
		}
	case PeerStateQuarantined:
		// 延长隔离期
		p.quarantinedUntil = time.Now().Add(quarantineDuration)
	}
}

// RecordSuccess 记录成功并恢复健康状态
func (p *PeerInfo) RecordSuccess() {
	p.stateLock.Lock()
	defer p.stateLock.Unlock()

	// 强制恢复健康分并重置失败计数
	p.healthScore = 100
	p.failureCount = 0
	p.LastUsefulAt = time.Now()
	p.LastSuccessfulOutboundQueryAt = time.Now()

	// 任何成功都直接恢复Active状态
	p.peerState = PeerStateActive
	p.quarantinedUntil = time.Time{}
}

// DecayHealth 指数衰减健康分（半衰期机制，基于Δt）
// 使用标准指数衰减公式：failure(t) = failure_0 * 0.5^(t/halfLife)
// now: 当前时间，用于计算Δt = now - lastHealthUpdateAt
func (p *PeerInfo) DecayHealth(now time.Time, halfLife time.Duration) {
	p.stateLock.Lock()
	defer p.stateLock.Unlock()

	// 初始化lastHealthUpdateAt
	if p.lastHealthUpdateAt.IsZero() {
		p.lastHealthUpdateAt = now
		return
	}

	// 如果从未失败过，无需衰减
	if p.lastFailureAt.IsZero() {
		p.lastHealthUpdateAt = now
		return
	}

	// 计算距上次健康更新的时间差（Δt）
	elapsed := now.Sub(p.lastHealthUpdateAt)
	if elapsed <= 0 {
		return // 时间未前进，无需更新
	}

	periods := float64(elapsed) / float64(halfLife)

	// 使用正确的指数衰减公式：0.5^periods
	decayFactor := math.Pow(0.5, periods)

	// 计算衰减后的失败分，并恢复健康分
	failureScore := 100 - p.healthScore
	decayedFailure := failureScore * decayFactor
	p.healthScore = min(100, p.healthScore+decayedFailure)

	// 更新健康分更新时间
	p.lastHealthUpdateAt = now

	// 如果健康分恢复到阈值以上，降级状态
	if p.healthScore >= 70 && p.peerState == PeerStateSuspect {
		p.peerState = PeerStateActive
	}
}

// CheckQuarantineExpired 检查隔离是否过期并自动解除
func (p *PeerInfo) CheckQuarantineExpired() bool {
	p.stateLock.Lock()
	defer p.stateLock.Unlock()

	if p.peerState == PeerStateQuarantined && time.Now().After(p.quarantinedUntil) {
		// 隔离期过期，降级为Suspect等待探测
		p.peerState = PeerStateSuspect
		p.quarantinedUntil = time.Time{}
		return true
	}
	return false
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// Bucket 是一个对等节点列表
// 所有对bucket的访问都在路由表的锁保护下进行同步,
// 因此bucket本身不需要任何锁
// 这与defs-back/kbucket/bucket.go的设计完全一致
type Bucket struct {
	list             *list.List // 存储对等节点信息的双向链表
	replacementCache *list.List // 替换缓存：存储备选节点
	maxReplacement   int        // 替换缓存最大容量
}

// newBucket 创建并返回一个新的空bucket
// 遵循defs-back/kbucket/bucket.go的newBucket函数
func newBucket() *Bucket {
	return &Bucket{
		list:             list.New(),
		replacementCache: list.New(),
		maxReplacement:   5, // 默认每桶5个替换位
	}
}

// addToReplacementCache 添加节点到替换缓存
func (b *Bucket) addToReplacementCache(p *PeerInfo) {
	// 检查是否已在替换缓存中
	for e := b.replacementCache.Front(); e != nil; e = e.Next() {
		cached := e.Value.(*PeerInfo)
		if cached.Id == p.Id {
			// 已存在，移到前端
			b.replacementCache.MoveToFront(e)
			return
		}
	}

	// 添加到替换缓存前端
	b.replacementCache.PushFront(p)

	// 如果超出容量，移除最后一个
	if b.replacementCache.Len() > b.maxReplacement {
		b.replacementCache.Remove(b.replacementCache.Back())
	}
}

// promoteFromReplacementCache 从替换缓存提升一个节点到主桶
func (b *Bucket) promoteFromReplacementCache() *PeerInfo {
	if b.replacementCache.Len() == 0 {
		return nil
	}

	// 取出最前面的节点（最近添加的）
	elem := b.replacementCache.Front()
	p := elem.Value.(*PeerInfo)
	b.replacementCache.Remove(elem)

	return p
}

// getPeers 返回桶中所有对等节点的信息列表
// 返回的是指针列表，调用者需要注意并发安全
// 对应defs-back/kbucket/bucket.go的peers()方法
func (b *Bucket) getPeers() []*PeerInfo {
	peers := make([]*PeerInfo, 0, b.list.Len())
	for e := b.list.Front(); e != nil; e = e.Next() {
		p := e.Value.(*PeerInfo)
		// 直接返回指针，不做深拷贝以避免锁拷贝问题
		peers = append(peers, p)
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
