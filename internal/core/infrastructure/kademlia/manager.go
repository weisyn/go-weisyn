package kbucket

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	"github.com/weisyn/v1/pkg/types"
	"github.com/libp2p/go-libp2p/core/peer"
)

// RoutingTableManager 实现路由表管理器
// 基于defs-back/kbucket的原始算法，确保Kademlia算法的准确性
type RoutingTableManager struct {
	// 配置和依赖
	config kademlia.KBucketConfig
	logger log.Logger
	host   node.Host // 新增：用于WES节点验证

	// 核心数据（来自defs-back/kbucket/table.go的结构）
	ctx        context.Context
	ctxCancel  context.CancelFunc
	localID    []byte        // 本地节点ID
	buckets    []*Bucket     // K桶数组
	bucketSize int           // 桶大小
	maxLatency time.Duration // 最大延迟

	// 锁管理
	tabLock        sync.RWMutex       // 总体锁
	cplRefreshLk   sync.RWMutex       // CPL刷新锁
	cplRefreshedAt map[uint]time.Time // CPL刷新时间

	// 回调函数
	peerAdded   func(peer.ID)
	peerRemoved func(peer.ID)

	// 宽限期（来自原始算法）
	usefulnessGracePeriod time.Duration

	// 运行状态
	running  bool
	runMutex sync.RWMutex
}

// NewRoutingTableManager 创建新的路由表管理器
// 严格按照defs-back/kbucket/table.go的NewRoutingTable逻辑
func NewRoutingTableManager(
	config kademlia.KBucketConfig,
	logger log.Logger,
	host node.Host,
) kademlia.RoutingTableManager {

	logger.Info("创建K桶路由表管理器")

	// 创建初始桶（来自原始算法）
	initialBucket := newBucket()

	manager := &RoutingTableManager{
		config:                config,
		logger:                logger,
		host:                  host,
		buckets:               []*Bucket{initialBucket},
		bucketSize:            config.GetBucketSize(),
		maxLatency:            config.GetMaxLatency(),
		cplRefreshedAt:        make(map[uint]time.Time),
		usefulnessGracePeriod: config.GetUsefulnessGracePeriod(),

		// 默认空回调
		peerAdded:   func(peer.ID) {},
		peerRemoved: func(peer.ID) {},
	}

	// 创建上下文（来自原始算法）
	manager.ctx, manager.ctxCancel = context.WithCancel(context.Background())

	logger.Info("K桶路由表管理器创建完成")
	return manager
}

// Start 启动管理器
func (rtm *RoutingTableManager) Start(ctx context.Context) error {
	rtm.runMutex.Lock()
	defer rtm.runMutex.Unlock()

	if rtm.running {
		return fmt.Errorf("routing table manager already running")
	}

	rtm.logger.Info("启动K桶路由表管理器")
	rtm.running = true
	return nil
}

// Stop 停止管理器
func (rtm *RoutingTableManager) Stop(ctx context.Context) error {
	rtm.runMutex.Lock()
	defer rtm.runMutex.Unlock()

	if !rtm.running {
		return nil
	}

	rtm.logger.Info("停止K桶路由表管理器")
	rtm.ctxCancel()
	rtm.running = false
	return nil
}

// IsRunning 检查运行状态
func (rtm *RoutingTableManager) IsRunning() bool {
	rtm.runMutex.RLock()
	defer rtm.runMutex.RUnlock()
	return rtm.running
}

// AddPeer 添加节点
// 基于defs-back/kbucket/table.go的TryAddPeer逻辑实现
func (rtm *RoutingTableManager) AddPeer(ctx context.Context, addrInfo peer.AddrInfo) (bool, error) {
	if !rtm.IsRunning() {
		return false, fmt.Errorf("manager not running")
	}

	rtm.logger.Debugf("尝试添加节点: %s", addrInfo.ID)

	// 🔒 WES节点验证：只允许业务节点进入K桶
	if rtm.host != nil {
		if isValidWES, err := rtm.host.ValidateWESPeer(ctx, addrInfo.ID); err != nil {
			rtm.logger.Debugf("节点 %s 验证失败: %v", addrInfo.ID, err)
			return false, nil // 静默拒绝，不返回错误
		} else if !isValidWES {
			rtm.logger.Debugf("拒绝外部节点进入K桶: %s", addrInfo.ID)
			return false, nil // 静默拒绝外部节点
		}
		// ✅ WES节点验证通过，继续添加
		rtm.logger.Debugf("WES节点验证通过: %s", addrInfo.ID)
	}

	// 将peer.ID转换为DHT ID
	dhtID := ConvertPeerID(addrInfo.ID)

	// 计算公共前缀长度来确定桶索引
	cpl := CommonPrefixLen(rtm.localID, dhtID)
	bucketIndex := cpl
	if bucketIndex >= len(rtm.buckets) {
		bucketIndex = len(rtm.buckets) - 1
	}

	rtm.tabLock.Lock()
	defer rtm.tabLock.Unlock()

	// 确保桶存在
	rtm.ensureBucket(bucketIndex)

	bucket := rtm.buckets[bucketIndex]

	// 检查节点是否已存在
	if elem := bucket.find(addrInfo.ID); elem != nil {
		// 节点已存在，移到前端（LRU更新）
		bucket.moveToFront(elem)
		rtm.logger.Debugf("节点已存在，更新LRU: %s", addrInfo.ID)
		return true, nil
	}

	// 检查桶是否已满
	if bucket.len() >= rtm.bucketSize {
		// 桶已满，检查最后一个节点是否可替换
		lastPeer := bucket.getPeers()[bucket.len()-1]
		if time.Since(lastPeer.LastUsefulAt) > rtm.usefulnessGracePeriod {
			// 最后一个节点太久未使用，可以替换
			bucket.remove(bucket.list.Back())
			rtm.logger.Debugf("替换最久未使用的节点: %s -> %s", lastPeer.Id, addrInfo.ID)
		} else {
			rtm.logger.Debugf("桶已满且无法替换节点: %s", addrInfo.ID)
			return false, nil
		}
	}

	// 添加新节点
	now := time.Now()
	peerInfo := &PeerInfo{
		Id:                            addrInfo.ID,
		Mode:                          0, // 默认模式
		LastUsefulAt:                  now,
		LastSuccessfulOutboundQueryAt: now,
		AddedAt:                       now,
		dhtId:                         dhtID,
		replaceable:                   false,
	}

	bucket.pushFront(peerInfo)

	// 触发回调
	rtm.peerAdded(addrInfo.ID)

	rtm.logger.Debugf("成功添加节点到桶 %d: %s", bucketIndex, addrInfo.ID)
	return true, nil
}

// RemovePeer 移除节点
func (rtm *RoutingTableManager) RemovePeer(peerID peer.ID) error {
	if !rtm.IsRunning() {
		return fmt.Errorf("manager not running")
	}

	rtm.logger.Debugf("移除节点: %s", peerID)

	rtm.tabLock.Lock()
	defer rtm.tabLock.Unlock()

	// 遍历所有桶查找并移除节点
	for i, bucket := range rtm.buckets {
		if elem := bucket.find(peerID); elem != nil {
			bucket.remove(elem)
			rtm.peerRemoved(peerID)
			rtm.logger.Debugf("从桶 %d 移除节点: %s", i, peerID)
			return nil
		}
	}

	return fmt.Errorf("peer not found: %s", peerID)
}

// FindClosestPeers 查找最近节点
// 基于defs-back/kbucket/table.go的NearestPeers算法实现
func (rtm *RoutingTableManager) FindClosestPeers(target []byte, count int) []peer.ID {
	if !rtm.IsRunning() {
		rtm.logger.Warn("管理器未运行")
		return nil
	}

	if count <= 0 {
		return nil
	}

	rtm.logger.Debugf("查找距离目标最近的%d个节点", count)

	rtm.tabLock.RLock()
	defer rtm.tabLock.RUnlock()

	// 计算目标的公共前缀长度
	cpl := CommonPrefixLen(rtm.localID, target)

	// 收集候选节点
	var candidates []peer.ID

	// 从目标桶开始，向外扩展搜索
	bucketIndex := cpl
	if bucketIndex >= len(rtm.buckets) {
		bucketIndex = len(rtm.buckets) - 1
	}

	// 搜索策略：从目标桶开始，然后向两侧扩展
	visited := make(map[int]bool)

	for len(candidates) < count*2 && len(visited) < len(rtm.buckets) {
		// 搜索当前桶
		if bucketIndex >= 0 && bucketIndex < len(rtm.buckets) && !visited[bucketIndex] {
			visited[bucketIndex] = true
			bucket := rtm.buckets[bucketIndex]
			peers := bucket.getPeers()

			for _, p := range peers {
				candidates = append(candidates, p.Id)
			}
		}

		// 交替向两侧扩展
		if bucketIndex > 0 {
			bucketIndex--
		} else if bucketIndex < len(rtm.buckets)-1 {
			bucketIndex++
		} else {
			break
		}
	}

	// 如果候选节点不够，从所有桶收集
	if len(candidates) < count {
		for i, bucket := range rtm.buckets {
			if !visited[i] {
				peers := bucket.getPeers()
				for _, p := range peers {
					candidates = append(candidates, p.Id)
				}
			}
		}
	}

	// 使用节点选择器按距离排序并选择最近的
	closest := SelectClosestPeers(candidates, target, count, rtm.logger)

	rtm.logger.Debugf("找到 %d 个最近节点", len(closest))
	return closest
}

// GetRoutingTable 获取路由表快照
func (rtm *RoutingTableManager) GetRoutingTable() *kademlia.RoutingTable {
	rtm.tabLock.RLock()
	defer rtm.tabLock.RUnlock()

	buckets := make([]*kademlia.Bucket, len(rtm.buckets))
	totalPeers := 0

	for i, bucket := range rtm.buckets {
		peers := bucket.getPeers()
		totalPeers += len(peers)

		kbucketPeers := make([]*kademlia.PeerInfo, len(peers))
		for j, peer := range peers {
			kbucketPeers[j] = &kademlia.PeerInfo{
				ID:                peer.Id.String(),
				LastSeen:          types.Timestamp(peer.LastUsefulAt),
				LastUsefulAt:      types.Timestamp(peer.LastUsefulAt),
				AddedAt:           types.Timestamp(peer.AddedAt),
				ConnectionLatency: time.Duration(0), // 实际应从连接监控获取
				IsReplaceable:     peer.replaceable,
				DHTId:             peer.dhtId,
				Mode:              peer.Mode,
			}
		}

		buckets[i] = &kademlia.Bucket{
			Index: i,
			Peers: kbucketPeers,
		}
	}

	return &kademlia.RoutingTable{
		LocalID:    string(rtm.localID),
		Buckets:    buckets,
		BucketSize: rtm.bucketSize,
		TableSize:  totalPeers,
		UpdatedAt:  types.Timestamp(time.Now()),
	}
}

// GetPeerCounts 获取节点统计信息
func (rtm *RoutingTableManager) GetPeerCounts() (totalPeers, healthyPeers int) {
	rtm.tabLock.RLock()
	defer rtm.tabLock.RUnlock()

	now := time.Now()

	// 统计所有桶中的节点
	for _, bucket := range rtm.buckets {
		peers := bucket.getPeers()
		totalPeers += len(peers)

		// 健康检查：最近活跃时间在宽限期内的节点认为是健康的
		for _, peer := range peers {
			if now.Sub(peer.LastUsefulAt) <= rtm.usefulnessGracePeriod {
				healthyPeers++
			}
		}
	}

	return totalPeers, healthyPeers
}

// SetPeerAddedCallback 设置节点添加回调
func (rtm *RoutingTableManager) SetPeerAddedCallback(callback func(peer.ID)) {
	rtm.tabLock.Lock()
	defer rtm.tabLock.Unlock()
	rtm.peerAdded = callback
}

// SetPeerRemovedCallback 设置节点移除回调
func (rtm *RoutingTableManager) SetPeerRemovedCallback(callback func(peer.ID)) {
	rtm.tabLock.Lock()
	defer rtm.tabLock.Unlock()
	rtm.peerRemoved = callback
}

// ensureBucket 确保指定索引的桶存在
func (rtm *RoutingTableManager) ensureBucket(index int) {
	for len(rtm.buckets) <= index {
		newBucket := newBucket()
		rtm.buckets = append(rtm.buckets, newBucket)
		rtm.logger.Debugf("创建新桶，索引: %d", len(rtm.buckets)-1)
	}
}
