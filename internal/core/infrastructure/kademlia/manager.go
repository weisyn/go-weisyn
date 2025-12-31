package kbucket

import (
	"container/list"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	lphost "github.com/libp2p/go-libp2p/core/host"
	libnetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/weisyn/v1/internal/config/node"
	"github.com/weisyn/v1/pkg/constants"
	"github.com/weisyn/v1/pkg/constants/events"
	"github.com/weisyn/v1/pkg/constants/protocols"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	"github.com/weisyn/v1/pkg/types"
)

// RoutingTableManager 实现路由表管理器
// 基于defs-back/kbucket的原始算法，确保Kademlia算法的准确性
type RoutingTableManager struct {
	// 配置和依赖
	config         kademlia.KBucketConfig
	logger         log.Logger
	p2pService     p2pi.Service    // 新增：用于WES节点验证和连接状态检查
	configProvider config.Provider // 新增：用于获取本地链身份进行比对
	eventBus       event.EventBus  // 🔧 Phase 3: 事件总线，用于发布重置事件
	eventBusMu     sync.RWMutex    // eventBus字段保护锁

	// 核心数据（来自defs-back/kbucket/table.go的结构）
	ctx        context.Context
	ctxCancel  context.CancelFunc
	localID    []byte        // 本地节点ID
	buckets    []*Bucket     // K桶数组
	bucketSize int           // 桶大小
	maxLatency time.Duration // 最大延迟

	// 锁管理
	tabLock        sync.RWMutex       // 总体锁
	cplRefreshedAt map[uint]time.Time // CPL刷新时间

	// 回调函数
	peerAdded   func(peer.ID)
	peerRemoved func(peer.ID)

	// 诊断信息：最近一次入桶尝试结果（用于 /debug/p2p/routing）
	lastAddMu sync.RWMutex
	lastAdd   *types.KBucketLastAdd

	// 宽限期（来自原始算法）
	usefulnessGracePeriod time.Duration

	// 运行状态
	running  bool
	runMutex sync.RWMutex

	// 🆕 就绪状态（Start完成且localID已初始化）
	ready      bool
	readyMutex sync.RWMutex

	// 可观测性指标
	metrics *KBucketMetrics

	// 探测并发控制（Phase 2）
	probeSemaphore chan struct{}
}

// NewRoutingTableManager 创建新的路由表管理器
// 严格按照defs-back/kbucket/table.go的NewRoutingTable逻辑
func NewRoutingTableManager(
	config kademlia.KBucketConfig,
	logger log.Logger,
	p2pService p2pi.Service,
	configProvider config.Provider, // 新增：用于获取本地链身份进行比对
) kademlia.RoutingTableManager {

	logger.Info("创建K桶路由表管理器")

	// 创建初始桶（来自原始算法）
	initialBucket := newBucket()

	manager := &RoutingTableManager{
		config:                config,
		logger:                logger,
		p2pService:            p2pService,
		configProvider:        configProvider,
		buckets:               []*Bucket{initialBucket},
		bucketSize:            config.GetBucketSize(),
		maxLatency:            config.GetMaxLatency(),
		cplRefreshedAt:        make(map[uint]time.Time),
		usefulnessGracePeriod: config.GetUsefulnessGracePeriod(),
		metrics:               &KBucketMetrics{},
		probeSemaphore:        make(chan struct{}, 5), // 最多5个并发探测

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

	// 初始化本地 DHT ID（32 bytes），用于正确的 CPL/bucket 计算。
	//
	// 说明：
	// - 之前 localID 为空会导致 CommonPrefixLen 恒为 0（所有节点落在 bucket 0），不会直接阻塞功能，
	//   但会降低 Kademlia 选择的质量，影响同步/选举等上层策略。
	// - 若 P2P Host 尚未就绪，则退化为随机 ID（仍保证长度正确）。
	if rtm.p2pService != nil && rtm.p2pService.Host() != nil {
		rtm.localID = ConvertPeerID(rtm.p2pService.Host().ID())
	} else {
		rtm.localID = GenerateRandomID()
	}

	rtm.running = true

	// 启动维护协程
	go rtm.maintenanceLoop()

	// 🔧 Phase 2：启动探测工作协程
	go rtm.probeWorker()

	// 🆕 标记就绪状态
	rtm.readyMutex.Lock()
	rtm.ready = true
	rtm.readyMutex.Unlock()

	rtm.logger.Info("✅ K桶路由表管理器已就绪")

	return nil
}

func (rtm *RoutingTableManager) setLastAdd(peerID peer.ID, result, reason string, err error) {
	if peerID == "" {
		return
	}
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	la := types.NewKBucketLastAdd(peerID.String(), time.Now(), result, reason, errStr)
	rtm.lastAddMu.Lock()
	rtm.lastAdd = &la
	rtm.lastAddMu.Unlock()
}

// GetDiagnosticsSummary 返回 K桶摘要（总量/健康量/最近入桶原因），供线上快速判断“空桶风险”。
func (rtm *RoutingTableManager) GetDiagnosticsSummary() types.KBucketSummary {
	total, healthy := rtm.GetPeerCounts()
	var last *types.KBucketLastAdd
	rtm.lastAddMu.RLock()
	if rtm.lastAdd != nil {
		cp := *rtm.lastAdd
		last = &cp
	}
	rtm.lastAddMu.RUnlock()
	return types.KBucketSummary{
		TotalPeers:   total,
		HealthyPeers: healthy,
		LastAdd:      last,
	}
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

	// 🆕 清除就绪状态
	rtm.readyMutex.Lock()
	rtm.ready = false
	rtm.readyMutex.Unlock()

	return nil
}

// IsRunning 检查运行状态
func (rtm *RoutingTableManager) IsRunning() bool {
	rtm.runMutex.RLock()
	defer rtm.runMutex.RUnlock()
	return rtm.running
}

// 🆕 IsReady 检查就绪状态（运行中且已初始化）
func (rtm *RoutingTableManager) IsReady() bool {
	rtm.readyMutex.RLock()
	defer rtm.readyMutex.RUnlock()
	return rtm.ready && rtm.running
}

// 🔧 Phase 3: SetEventBus 设置事件总线（由lifecycle注入）
func (rtm *RoutingTableManager) SetEventBus(eb event.EventBus) {
	rtm.eventBusMu.Lock()
	defer rtm.eventBusMu.Unlock()
	rtm.eventBus = eb
}

// AddPeer 添加节点
// 基于defs-back/kbucket/table.go的TryAddPeer逻辑实现
func (rtm *RoutingTableManager) AddPeer(ctx context.Context, addrInfo peer.AddrInfo) (bool, error) {
	if !rtm.IsRunning() {
		return false, fmt.Errorf("manager not running")
	}

	rtm.logger.Debugf("尝试添加节点: %s", addrInfo.ID)

	// 🔒 WES节点验证：只允许业务节点进入K桶
	if rtm.p2pService != nil {
		if isValidWES, err := rtm.validateWESPeer(ctx, addrInfo.ID); err != nil {
			// 这里必须返回 error：
			// - 该错误通常表示 Identify/Peerstore 尚未就绪、Host 未就绪等“可恢复”问题；
			// - 返回 error 能触发上层（module.go 的延迟重试/周期 reconcile）继续尝试入桶；
			// - 同时也能让日志明确暴露根因，避免“永远不入桶”但看不出原因。
			rtm.logger.Debugf("节点 %s 验证失败（可恢复，稍后重试）: %v", addrInfo.ID, err)
			rtm.setLastAdd(addrInfo.ID, "error", "wes_check_error", err)
			return false, err
		} else if !isValidWES {
			rtm.logger.Debugf("拒绝外部节点进入K桶: %s", addrInfo.ID)
			rtm.setLastAdd(addrInfo.ID, "rejected", "not_wes", nil)
			return false, nil // 静默拒绝外部节点
		}
		// ✅ WES节点验证通过，继续添加
		rtm.logger.Debugf("WES节点验证通过: %s", addrInfo.ID)
		rtm.setLastAdd(addrInfo.ID, "rejected", "weisyn_proto", nil) // 先标记“通过WES识别”，成功入桶会覆盖为 added
	}

	// 🔒 链身份验证：检查 peer 的链身份是否匹配
	if rtm.configProvider != nil && rtm.p2pService != nil {
		chainOK, reason, err := rtm.validatePeerChainIdentity(ctx, addrInfo.ID)
		if err != nil {
			// 视为“可恢复错误”：通常是 Identify/peerstore 尚未就绪，交给上层重试/reconcile。
			rtm.logger.Debugf("policy.chain_identity_error: peer=%s err=%v", addrInfo.ID, err)
			rtm.setLastAdd(addrInfo.ID, "error", "chain_identity_error", err)
			return false, err
		}
		if !chainOK {
			rtm.logger.Debugf("policy.reject_sync_peer: 链身份不匹配/缺失，拒绝加入K桶: peer=%s reason=%s", addrInfo.ID, reason)
			rtm.setLastAdd(addrInfo.ID, "rejected", reason, nil)
			return false, nil // 静默拒绝，不返回错误
		}
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
		rtm.setLastAdd(addrInfo.ID, "already_exists", "unknown", nil)
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
			rtm.setLastAdd(addrInfo.ID, "bucket_full", "bucket_full", nil)
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
		peerState:                     PeerStateActive, // 初始状态为Active
		healthScore:                   100,             // 初始健康分100
		failureCount:                  0,
	}

	bucket.pushFront(peerInfo)

	// 触发回调
	rtm.peerAdded(addrInfo.ID)

	rtm.logger.Debugf("成功添加节点到桶 %d: %s", bucketIndex, addrInfo.ID)
	rtm.setLastAdd(addrInfo.ID, "added", "unknown", nil)
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
			
			// ✅ 修复缺陷M：取消保护已删除的peer连接
			// 当peer从K桶删除时，应取消连接保护，允许连接管理器根据需要淘汰这些连接
			if rtm.p2pService != nil && rtm.p2pService.Host() != nil {
				if cm := rtm.p2pService.Host().ConnManager(); cm != nil {
					cm.Unprotect(peerID, "kbucket")
					rtm.logger.Debugf("🔓 已取消保护K桶peer连接: %s", peerID)
				}
			}
			
			return nil
		}
	}

	return fmt.Errorf("peer not found: %s", peerID)
}

// FindClosestPeers 查找最近节点（带健康过滤）
// 基于defs-back/kbucket/table.go的NearestPeers算法实现
func (rtm *RoutingTableManager) FindClosestPeers(target []byte, count int) []peer.ID {
	if !rtm.IsRunning() {
		rtm.logger.Warn("管理器未运行")
		return nil
	}

	if count <= 0 {
		return nil
	}

	rtm.logger.Debugf("查找距离目标最近的%d个健康节点", count)

	rtm.tabLock.RLock()
	defer rtm.tabLock.RUnlock()

	// 获取libp2p host用于连接状态检查
	var libp2pHost interface{}
	if rtm.p2pService != nil {
		libp2pHost = rtm.p2pService.Host()
	}

	// 计算目标的公共前缀长度
	cpl := CommonPrefixLen(rtm.localID, target)

	// 收集候选节点（仅收集健康的）
	var candidates []peer.ID
	var suspectCandidates []peer.ID // Suspect节点单独收集，作为灰度探测候选

	// 从目标桶开始，向外扩展搜索
	bucketIndex := cpl
	if bucketIndex >= len(rtm.buckets) {
		bucketIndex = len(rtm.buckets) - 1
	}

	// 搜索策略：从目标桶开始，然后向两侧扩展
	visited := make(map[int]bool)

	for len(candidates)+len(suspectCandidates) < count*3 && len(visited) < len(rtm.buckets) {
		// 搜索当前桶
		if bucketIndex >= 0 && bucketIndex < len(rtm.buckets) && !visited[bucketIndex] {
			visited[bucketIndex] = true
			bucket := rtm.buckets[bucketIndex]
			peers := bucket.getPeers()

			for _, p := range peers {
				// 健康过滤
				if rtm.isPeerHealthy(p, libp2pHost) {
					candidates = append(candidates, p.Id)
				} else if p.GetState() == PeerStateSuspect {
					// Suspect节点保留作为灰度探测候选
					suspectCandidates = append(suspectCandidates, p.Id)
				}
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
	if len(candidates)+len(suspectCandidates) < count {
		for i, bucket := range rtm.buckets {
			if !visited[i] {
				peers := bucket.getPeers()
				for _, p := range peers {
					if rtm.isPeerHealthy(p, libp2pHost) {
						candidates = append(candidates, p.Id)
					} else if p.GetState() == PeerStateSuspect {
						suspectCandidates = append(suspectCandidates, p.Id)
					}
				}
			}
		}
	}

	// 使用节点选择器按距离排序并选择最近的
	closest := SelectClosestPeers(candidates, target, count, rtm.logger)

	// 如果健康节点不够，适量添加Suspect节点作为灰度探测
	if len(closest) < count && len(suspectCandidates) > 0 {
		remaining := count - len(closest)
		if remaining > len(suspectCandidates)/2 {
			remaining = len(suspectCandidates) / 2 // 最多添加一半Suspect节点
		}
		suspectClosest := SelectClosestPeers(suspectCandidates, target, remaining, rtm.logger)
		closest = append(closest, suspectClosest...)
		rtm.logger.Debugf("添加%d个Suspect节点作为灰度探测候选", len(suspectClosest))
	}

	rtm.logger.Debugf("找到 %d 个健康节点（包含%d个Suspect灰度）", len(closest), len(closest)-len(candidates))

	// 🔧 Phase 3: 记录FindClosestPeers失败事件并触发Discovery间隔重置
	if len(closest) == 0 {
		if rtm.metrics != nil {
			rtm.metrics.RecordNoClosestPeers()
		}
		
		// 发布Discovery间隔重置事件，让发现循环立即加速
		rtm.eventBusMu.RLock()
		eb := rtm.eventBus
		rtm.eventBusMu.RUnlock()
		
		if eb != nil {
			resetData := &types.DiscoveryResetEventData{
				Reason:           "kbucket_degraded",
				Trigger:          "kademlia",
				RoutingTableSize: 0,
				Timestamp:        time.Now().Unix(),
			}
			eb.Publish(events.EventTypeDiscoveryIntervalReset, resetData)
			if rtm.logger != nil {
				rtm.logger.Infof("🔄 K桶退化：FindClosestPeers找不到节点，已触发Discovery间隔重置")
			}
		}
	}

	return closest
}

// isPeerHealthy 检查节点是否健康（可被选用）
func (rtm *RoutingTableManager) isPeerHealthy(p *PeerInfo, libp2pHostInterface interface{}) bool {
	// 1. 检查健康分
	if p.GetHealthScore() < 50 {
		return false
	}

	// 2. 检查是否被隔离
	if p.IsQuarantined() {
		return false
	}

	// 3. 检查连接状态（如果有host）
	if libp2pHostInterface != nil {
		// 尝试从rtm.p2pService获取连接状态（更直接的方式）
		if rtm.p2pService != nil {
			if h := rtm.p2pService.Host(); h != nil {
				// 简化连接检查：通过查询peerstore地址是否存在
				addrs := h.Peerstore().Addrs(p.Id)
				if len(addrs) == 0 {
					return false // 无地址信息，认为不可达
				}
				// 进一步检查是否有活跃连接
				conns := h.Network().ConnsToPeer(p.Id)
				if len(conns) == 0 {
					return false // 无活跃连接
				}
			}
		}
	}

	// 4. Active或Suspect状态节点可以被选用
	return p.GetState() == PeerStateActive || p.GetState() == PeerStateSuspect
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

// RecordPeerFailure 记录节点失败
func (rtm *RoutingTableManager) RecordPeerFailure(peerID peer.ID) {
	rtm.tabLock.RLock()
	defer rtm.tabLock.RUnlock()

	// 查找节点并记录失败
	for _, bucket := range rtm.buckets {
		if elem := bucket.find(peerID); elem != nil {
			p := elem.Value.(*PeerInfo)
			p.RecordFailure(rtm.config.GetFailureThreshold(), rtm.config.GetQuarantineDuration())
			rtm.logger.Debugf("记录节点失败: %s, 状态=%s, 健康分=%.1f, 失败次数=%d",
				peerID, p.GetState(), p.GetHealthScore(), p.failureCount)
			return
		}
	}
}

// RecordPeerSuccess 记录节点成功
func (rtm *RoutingTableManager) RecordPeerSuccess(peerID peer.ID) {
	rtm.tabLock.RLock()
	defer rtm.tabLock.RUnlock()

	// 查找节点并记录成功
	for _, bucket := range rtm.buckets {
		if elem := bucket.find(peerID); elem != nil {
			p := elem.Value.(*PeerInfo)
			p.RecordSuccess()
			rtm.logger.Debugf("记录节点成功: %s, 状态=%s, 健康分=%.1f",
				peerID, p.GetState(), p.GetHealthScore())
			// 成功后移到桶前端（LRU更新）
			bucket.moveToFront(elem)
			return
		}
	}
}

// QuarantineIncompatiblePeer 直接隔离不兼容的节点
//
// 🆕 2025-12-18：用于处理明确不支持 WES 协议的节点
//
// 与 RecordPeerFailure 的区别：
// - RecordPeerFailure: 需要多次失败才会进入隔离状态（渐进式降级）
// - QuarantineIncompatiblePeer: 直接进入隔离状态（协议不兼容是明确的不兼容，无需渐进）
//
// 隔离效果：
// - 节点状态设置为 Quarantined
// - 健康分设置为 0
// - 隔离时间为配置的隔离期（默认 1 小时）
// - 隔离期间节点不会被选为聚合器/同步上游
//
// 参数：
// - peerID: 要隔离的节点 ID
// - reason: 隔离原因（用于日志）
func (rtm *RoutingTableManager) QuarantineIncompatiblePeer(peerID peer.ID, reason string) {
	rtm.tabLock.RLock()
	defer rtm.tabLock.RUnlock()

	quarantineDuration := rtm.config.GetQuarantineDuration()

	// 查找节点并直接隔离
	for _, bucket := range rtm.buckets {
		if elem := bucket.find(peerID); elem != nil {
			p := elem.Value.(*PeerInfo)

			p.stateLock.Lock()
			p.peerState = PeerStateQuarantined
			p.healthScore = 0
			p.quarantinedUntil = time.Now().Add(quarantineDuration)
			p.stateLock.Unlock()

			rtm.logger.Infof("🔒 隔离不兼容节点: peer=%s reason=%s duration=%s",
				peerID.String()[:12], reason, quarantineDuration)
			return
		}
	}

	// 节点不在 K 桶中，记录日志（可能已被清理）
	rtm.logger.Debugf("尝试隔离不存在的节点: peer=%s reason=%s", peerID.String()[:12], reason)
}

// maintenanceLoop 维护协程：周期性执行健康管理任务
func (rtm *RoutingTableManager) maintenanceLoop() {
	ticker := time.NewTicker(rtm.config.GetMaintainInterval())
	defer ticker.Stop()

	for {
		select {
		case <-rtm.ctx.Done():
			rtm.logger.Info("维护协程收到停止信号")
			return
		case <-ticker.C:
			rtm.runMaintenance()
		}
	}
}

// runMaintenance 执行维护任务
func (rtm *RoutingTableManager) runMaintenance() {
	if !rtm.IsRunning() {
		return
	}

	rtm.tabLock.Lock()
	defer rtm.tabLock.Unlock()

	// 记录维护执行
	if rtm.metrics != nil {
		rtm.metrics.RecordMaintenanceRun()
	}

	now := time.Now()
	halfLife := rtm.config.GetHealthDecayHalfLife()
	minPeers := rtm.config.GetMinPeersPerBucket()
	gracePeriod := rtm.usefulnessGracePeriod

	// 🔧 为所有已连接peer更新LastUsefulAt（自动续期）
	if rtm.p2pService != nil && rtm.p2pService.Host() != nil {
		host := rtm.p2pService.Host()
		for _, bucket := range rtm.buckets {
			bucket.updateAllWith(func(p *PeerInfo) {
				if host.Network().Connectedness(p.Id) == libnetwork.Connected {
					// 连接中的peer，自动续期LastUsefulAt
					p.LastUsefulAt = now
				}
			})
		}
	}

	for bucketIdx, bucket := range rtm.buckets {
		if bucket.len() == 0 {
			continue
		}

		// 1. 健康分衰减（基于Δt）
		bucket.updateAllWith(func(p *PeerInfo) {
			p.DecayHealth(now, halfLife)
		})

		// 2. 检查并解除过期的隔离
		bucket.updateAllWith(func(p *PeerInfo) {
			if p.CheckQuarantineExpired() {
				rtm.logger.Debugf("节点隔离期过期，降级为Suspect: %s", p.Id)
			}
		})

		// 3. 🆕 主动清理Suspect节点（修复内存泄漏）
		rtm.cleanupSuspectPeers(bucket, bucketIdx)

		// 4. 清理长期不可达且不健康的节点（仅当桶有余量时）
		if bucket.len() > minPeers {
			rtm.cleanupUnhealthyPeers(bucket, bucketIdx, gracePeriod)
		}

		// 5. 🔧 Phase 2：最终清理（只删除探测确认失败的peer）
		rtm.finalCleanup(bucket, bucketIdx)
	}

	// 6. 🆕 检查总peer数量，如果过多则强制清理
	totalPeers := rtm.sizeNoLock()
	if totalPeers > 500 {
		rtm.logger.Warnf("Peer总数过多(%d)，执行强制清理", totalPeers)
		rtm.forceCleanupOldestSuspect(50)
	}

	// 7. 更新状态分布指标
	if rtm.metrics != nil {
		var active, suspect, quarantined, evicted int64
		for _, bucket := range rtm.buckets {
			for e := bucket.list.Front(); e != nil; e = e.Next() {
				p := e.Value.(*PeerInfo)
				switch p.GetState() {
				case PeerStateActive:
					active++
				case PeerStateSuspect:
					suspect++
				case PeerStateQuarantined:
					quarantined++
				case PeerStateEvicted:
					evicted++
				}
			}
		}
		rtm.metrics.UpdateStateDistribution(active, suspect, quarantined, evicted)
	}
}

// cleanupUnhealthyPeers 清理不健康的节点
func (rtm *RoutingTableManager) cleanupUnhealthyPeers(bucket *Bucket, bucketIdx int, gracePeriod time.Duration) {
	now := time.Now()
	minPeers := rtm.config.GetMinPeersPerBucket()
	cleanupGracePeriod := rtm.config.GetCleanupGracePeriod()
	lowHealthThreshold := rtm.config.GetLowHealthThreshold()
	addrProtectionGracePeriod := rtm.config.GetAddrProtectionGracePeriod()

	// 获取host用于检查连接状态
	var host lphost.Host
	if rtm.p2pService != nil {
		host = rtm.p2pService.Host()
	}

	// 🔧 Phase 2：标记待清理peer（不立即删除）
	for e := bucket.list.Front(); e != nil; e = e.Next() {
		p := e.Value.(*PeerInfo)

		// 🔧 硬约束：必须先检查连接状态
		isConnected := false
		if host != nil {
			connectedness := host.Network().Connectedness(p.Id)
			isConnected = (connectedness == libnetwork.Connected)
			if isConnected {
				// 仍连接的peer，跳过清理
				rtm.logger.Debugf("跳过清理已连接peer: bucket=%d, peer=%s, state=%s, health=%.1f",
					bucketIdx, p.Id, p.GetState(), p.GetHealthScore())
				continue
			}
		}

		// === P0-010：清理条件保守化 ===
		// 1) “长期无用”不再用 gracePeriod*3（默认约3分钟，过于激进），改为独立的 CleanupGracePeriod（默认10分钟）
		// 2) 低健康阈值从 20 降到 10（更保守）
		// 3) 若 peerstore 中仍有地址，给予更长的保护窗口（30分钟），减少误清理导致网络孤岛
		//
		// 注意：LastUsefulAt 可能为零值（历史/构造测试），此时回退到 AddedAt，避免被误判为“很久以前”
		lastUsefulRef := p.LastUsefulAt
		if lastUsefulRef.IsZero() {
			lastUsefulRef = p.AddedAt
		}
		if lastUsefulRef.IsZero() {
			lastUsefulRef = now
		}

		// 断连 peer 的“重连宽限期”：在 cleanupGracePeriod 内不进入清理流程
		if now.Sub(lastUsefulRef) < cleanupGracePeriod {
			continue
		}

		// 若仍有地址，额外保护（由配置项控制）
		if host != nil {
			if addrs := host.Peerstore().Addrs(p.Id); len(addrs) > 0 {
				if now.Sub(lastUsefulRef) < addrProtectionGracePeriod {
					continue
				}
			}
		}

		// 清理条件：断连 + (长期无用 + 低健康) OR Evicted状态 + 桶有余量
		longTimeUnused := now.Sub(lastUsefulRef) > cleanupGracePeriod
		lowHealth := p.GetHealthScore() < lowHealthThreshold
		isEvicted := p.GetState() == PeerStateEvicted

		// 🔧 Phase 2：不立即删除，标记为待探测
		if (longTimeUnused && lowHealth) || isEvicted {
			if bucket.len() > minPeers {
				// 标记为待探测（而非立即删除）
				p.stateLock.Lock()
				if p.probeStatus == ProbeNotNeeded || p.probeStatus == ProbeSuccess {
					p.probeStatus = ProbePending
					p.lastProbeAt = time.Time{} // 重置探测时间
					p.probeFailCount = 0        // 重置失败计数
					rtm.logger.Debugf("标记peer待探测清理: bucket=%d, peer=%s, state=%s, health=%.1f",
						bucketIdx, p.Id, p.GetState(), p.GetHealthScore())
				}
				p.stateLock.Unlock()
			}
		}
	}
}

// cleanupSuspectPeers 清理断连的Suspect/Quarantined节点
func (rtm *RoutingTableManager) cleanupSuspectPeers(bucket *Bucket, bucketIdx int) {
	// 获取host用于检查连接状态
	var host lphost.Host
	if rtm.p2pService != nil {
		host = rtm.p2pService.Host()
	}

	if host == nil {
		return // 无法检查连接状态，跳过清理
	}

	now := time.Now()

	// 🔧 Phase 2：标记待探测（不立即删除）
	for e := bucket.list.Front(); e != nil; e = e.Next() {
		p := e.Value.(*PeerInfo)

		// 🔧 硬约束：只清理已断连的peer
		connectedness := host.Network().Connectedness(p.Id)
		if connectedness == libnetwork.Connected {
			continue
		}

		// 🔧 Phase 2：不立即删除，标记为待探测
		// Suspect断连且长期无用（更保守的阈值：10分钟）
		if p.GetState() == PeerStateSuspect {
			if now.Sub(p.LastUsefulAt) > 10*time.Minute {
				p.stateLock.Lock()
				if p.probeStatus == ProbeNotNeeded || p.probeStatus == ProbeSuccess {
					p.probeStatus = ProbePending
					p.lastProbeAt = time.Time{}
					p.probeFailCount = 0
					rtm.logger.Debugf("标记Suspect peer待探测清理: bucket=%d, peer=%s",
						bucketIdx, p.Id)
				}
				p.stateLock.Unlock()
			}
		}

		// Quarantined断连且隔离期过期超过10分钟
		if p.GetState() == PeerStateQuarantined {
			if now.Sub(p.LastUsefulAt) > 10*time.Minute {
				p.stateLock.Lock()
				if p.probeStatus == ProbeNotNeeded || p.probeStatus == ProbeSuccess {
					p.probeStatus = ProbePending
					p.lastProbeAt = time.Time{}
					p.probeFailCount = 0
					rtm.logger.Debugf("标记Quarantined peer待探测清理: bucket=%d, peer=%s",
						bucketIdx, p.Id)
				}
				p.stateLock.Unlock()
			}
		}
	}

	// Phase 2：不再在此处执行清理，改由finalCleanup处理
}

// forceCleanupOldestSuspect 🆕 强制清理最老的Suspect节点（修复内存泄漏）
func (rtm *RoutingTableManager) forceCleanupOldestSuspect(count int) {
	type suspectPeer struct {
		bucket *Bucket
		elem   *list.Element
		peer   *PeerInfo
	}

	var suspects []suspectPeer

	// 收集所有Suspect和Quarantined节点
	for _, bucket := range rtm.buckets {
		for e := bucket.list.Front(); e != nil; e = e.Next() {
			p := e.Value.(*PeerInfo)
			if p.GetState() == PeerStateSuspect || p.GetState() == PeerStateQuarantined {
				suspects = append(suspects, suspectPeer{
					bucket: bucket,
					elem:   e,
					peer:   p,
				})
			}
		}
	}

	// 按LastUsefulAt排序（最老的在前面）
	// 简化实现：清理前N个
	cleanCount := count
	if cleanCount > len(suspects) {
		cleanCount = len(suspects)
	}

	for i := 0; i < cleanCount; i++ {
		sp := suspects[i]
		sp.bucket.remove(sp.elem)
		rtm.peerRemoved(sp.peer.Id)
		rtm.logger.Infof("强制清理Suspect节点: peer=%s", sp.peer.Id)
	}
}

// sizeNoLock 🆕 获取peer总数（不加锁版本，在已加锁的情况下调用）
func (rtm *RoutingTableManager) sizeNoLock() int {
	count := 0
	for _, bucket := range rtm.buckets {
		count += bucket.len()
	}
	return count
}

// validateWESPeer 验证节点是否为WES业务节点
// 基于协议能力检查实现简单的节点分类
func (rtm *RoutingTableManager) validateWESPeer(ctx context.Context, peerID peer.ID) (bool, error) {
	if rtm.p2pService == nil {
		return false, fmt.Errorf("p2p service not available")
	}

	host := rtm.p2pService.Host()
	if host == nil {
		return false, fmt.Errorf("libp2p host not available")
	}

	// 获取节点支持的协议（先获取，用于后续连接状态判定）
	peerProtocols, err := host.Peerstore().GetProtocols(peerID)
	if err != nil {
		return false, fmt.Errorf("failed to get protocols for peer %s: %v", peerID, err)
	}

	// 检查节点连接状态
	//
	// ✅ 修复缺陷K：连接状态时序竞态
	// - libp2p 连接状态在短时间内可能从 Connected 抖动到 CanConnect/NotConnected；
	// - 若只允许 Connected，会在 "connected 事件触发入桶" 的延迟窗口内误拒绝业务节点，导致 K桶长期为空。
	// - 放宽为 Connected 或 CanConnect：允许"可建立连接但当前未连接"的 peer 进入候选集合，
	//   后续依赖健康探测/两阶段清理机制淘汰长期失联节点。
	//
	// ✅ 修复缺陷L：连接管理器淘汰导致的K桶入表失败
	// - 当连接数超过 HighWater 时，连接管理器会主动淘汰连接，导致 peer 从 Connected 变为 NotConnected
	// - 如果 peer 有协议缓存（说明之前成功 Identify），即使当前 NotConnected 也允许入桶
	// - 理由：依赖后续健康探测机制淘汰长期失联节点，避免因暂时断连而错失业务节点
	connectedness := host.Network().Connectedness(peerID)
	if connectedness == libnetwork.Connected || connectedness == libnetwork.CanConnect {
		// 已连接或可连接，直接通过连接状态检查
	} else if connectedness == libnetwork.NotConnected && len(peerProtocols) > 0 {
		// NotConnected 但有协议缓存：允许入桶（可能是连接管理器淘汰或临时断连）
		// 后续健康探测会淘汰长期失联的节点
	} else {
		// 完全无连接且无协议信息：拒绝
		return false, nil
	}

	// ✅ 最高优先级：只要对端宣告过任意 "/weisyn/" 协议，即认为是 WES 业务节点。
	// 这样可以避免“协议枚举不全/新协议未列入 baseCandidates”导致的误判。
	for _, p := range peerProtocols {
		sp := string(p)
		if strings.Contains(sp, "/weisyn/") {
			return true, nil
		}
	}

	// 检查是否支持 WES 业务协议（用于判断是否“本链业务节点”，可进入 K 桶参与同步/路由/选举）。
	//
	// ⚠️ 关键修复：
	// 之前仅用 ProtocolBlockSubmission 作为“WES节点识别”条件，但该协议通常只会在聚合器/特定共识角色上注册
	//（矿工/普通 full 节点可能不会注册 handler，因此 Peerstore 协议列表里不包含它），会导致：
	// - 同一套 weisyn 节点互联成功，但被误判为“非 WES 节点” -> 不入桶 -> 无法同步
	//
	// 正确策略：只要对端支持任一 weisyn 的基础/同步协议，即可认为是 WES 业务节点。
	// 共识侧如果需要严格协议能力（如 block_submission），应在共识模块内单独校验（已有）。
	baseCandidates := []string{
		// 基础管理/发现（只要是 weisyn 节点通常都会有）
		protocols.ProtocolNodeInfo,
		protocols.ProtocolHeartbeat,

		// 同步相关（必须）
		protocols.ProtocolBlockSync,
		protocols.ProtocolHeaderSync,
		protocols.ProtocolStateSync,
		protocols.ProtocolKBucketSync,
		protocols.ProtocolRangePaginated,

		// 交易直连（可选）
		protocols.ProtocolTransactionDirect,

		// 共识提交（可选：聚合器/特定角色）
		protocols.ProtocolBlockSubmission,
	}

	ns := ""
	if rtm.configProvider != nil {
		func() {
			defer func() { _ = recover() }()
			ns = rtm.configProvider.GetNetworkNamespace()
		}()
	}

	match := func(sp, base string) bool {
		if sp == base {
			return true
		}
		if ns != "" {
			q := protocols.QualifyProtocol(base, ns)
			return sp == q
		}
		return false
	}

	for _, p := range peerProtocols {
		sp := string(p)
		for _, base := range baseCandidates {
			if match(sp, base) {
				return true, nil
			}
		}
	}

	// 不支持WES核心协议，认为是外部节点
	return false, nil
}

// FindClosestPeersForProtocol 返回“距离 target 最近且支持指定协议”的候选节点集合。
//
// 设计约束：
// - 该方法必须是“纯本地快路径”，不得 DialPeer，不得做任何网络探测；
// - 协议支持判断仅基于 peerstore 中已缓存的协议列表（Identify 结果）。
//
// 用途：
// - 共识选举/转发：避免在热路径调用 CheckProtocolSupport -> DialPeer；
// - 同步上游选择：优先选择确实支持同步协议的 peer。
func (rtm *RoutingTableManager) FindClosestPeersForProtocol(target []byte, count int, requiredProto string) []peer.ID {
	if count <= 0 {
		return nil
	}
	// 先取更多候选，再按协议过滤，避免过滤后不足
	candidates := rtm.FindClosestPeers(target, count*3)
	if len(candidates) == 0 {
		return nil
	}
	out := make([]peer.ID, 0, count)
	for _, pid := range candidates {
		if pid == "" {
			continue
		}
		ok, err := rtm.peerSupportsProtocolFromPeerstore(pid, requiredProto)
		if err != nil || !ok {
			continue
		}
		out = append(out, pid)
		if len(out) >= count {
			break
		}
	}
	return out
}

// SupportsProtocol 返回 peer 是否支持指定协议（纯本地快路径，不拨号）。
func (rtm *RoutingTableManager) SupportsProtocol(peerID peer.ID, protoID string) (bool, error) {
	return rtm.peerSupportsProtocolFromPeerstore(peerID, protoID)
}

// SupportsProtocolWithRefresh 检查 peer 是否支持协议，如果 peerstore 中没有缓存则尝试刷新
// 🆕 2025-12-19 新增：解决 peerstore 协议列表未及时更新导致的误判
//
// 策略：
// 1. 首先检查 peerstore 缓存（快路径）
// 2. 如果缓存为空且 peer 已连接，尝试从 identify 服务刷新协议列表
// 3. 再次检查刷新后的协议列表
//
// 注意：此方法可能触发网络操作，仅在必要时使用（如聚合器选择失败后的重试）
func (rtm *RoutingTableManager) SupportsProtocolWithRefresh(ctx context.Context, peerID peer.ID, protoID string) (bool, error) {
	// 1. 快路径：先检查 peerstore 缓存
	supported, err := rtm.peerSupportsProtocolFromPeerstore(peerID, protoID)
	if err == nil && supported {
		return true, nil
	}

	// 2. 检查是否需要刷新
	if rtm.p2pService == nil || rtm.p2pService.Host() == nil {
		return false, fmt.Errorf("p2p service not available for protocol refresh")
	}

	h := rtm.p2pService.Host()

	// 检查 peer 是否已连接
	if h.Network().Connectedness(peerID) != libnetwork.Connected {
		// 未连接，无法刷新
		if rtm.logger != nil {
			rtm.logger.Debugf("无法刷新协议列表：peer %s 未连接", peerID.String()[:12])
		}
		return false, nil
	}

	// 3. 尝试触发 identify 刷新（如果有 identify 服务）
	// 注意：libp2p 的 identify 服务会自动在连接时交换协议信息
	// 这里我们只是等待一小段时间让 identify 完成
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-time.After(100 * time.Millisecond):
		// 等待 identify 可能的更新
	}

	// 4. 再次检查协议支持
	supported, err = rtm.peerSupportsProtocolFromPeerstore(peerID, protoID)
	if err != nil {
		return false, err
	}

	if supported {
		if rtm.logger != nil {
			rtm.logger.Debugf("协议列表刷新后，peer %s 支持协议 %s", peerID.String()[:12], protoID)
		}
	}

	return supported, nil
}

// GetPeerProtocols 获取 peer 支持的所有协议列表（调试用）
func (rtm *RoutingTableManager) GetPeerProtocols(peerID peer.ID) ([]string, error) {
	if rtm.p2pService == nil || rtm.p2pService.Host() == nil {
		return nil, fmt.Errorf("host not available")
	}

	h := rtm.p2pService.Host()
	ps, err := h.Peerstore().GetProtocols(peerID)
	if err != nil {
		return nil, err
	}

	result := make([]string, len(ps))
	for i, p := range ps {
		result[i] = string(p)
	}
	return result, nil
}

// IsWESNode 检查 peer 是否是 WES 节点
// 通过检查是否支持 WES 核心协议来判断
func (rtm *RoutingTableManager) IsWESNode(peerID peer.ID) bool {
	// WES 节点必须支持以下核心协议之一
	coreProtocols := []string{
		protocols.ProtocolBlockSubmission,
		protocols.ProtocolSyncHelloV2,
		protocols.ProtocolKBucketSync,
	}

	for _, proto := range coreProtocols {
		supported, err := rtm.peerSupportsProtocolFromPeerstore(peerID, proto)
		if err == nil && supported {
			return true
		}
	}
	return false
}

// PeerType 节点类型枚举
type PeerType string

const (
	// PeerTypeWESFull WES 完整节点（支持所有核心协议）
	PeerTypeWESFull PeerType = "wes_full"
	// PeerTypeWESPartial WES 部分节点（支持部分核心协议，可能版本不同）
	PeerTypeWESPartial PeerType = "wes_partial"
	// PeerTypeWESIncompatible WES 节点但版本不兼容
	PeerTypeWESIncompatible PeerType = "wes_incompatible"
	// PeerTypeExternalLibp2p 外部 libp2p 节点（非 WES）
	PeerTypeExternalLibp2p PeerType = "external_libp2p"
	// PeerTypeUnknown 未知类型（无法确定）
	PeerTypeUnknown PeerType = "unknown"
)

// PeerCompatibilityInfo 节点兼容性信息
type PeerCompatibilityInfo struct {
	PeerID             peer.ID
	Type               PeerType
	SupportedProtocols []string          // 支持的协议列表
	MissingProtocols   []string          // 缺失的核心协议
	VersionMismatch    map[string]string // 版本不匹配的协议: 期望版本 -> 实际版本
	IncompatibleReason string            // 不兼容原因
	IsCompatible       bool              // 是否兼容
}

// AnalyzePeerCompatibility 分析节点兼容性
// 返回详细的节点类型识别和兼容性信息
func (rtm *RoutingTableManager) AnalyzePeerCompatibility(peerID peer.ID) *PeerCompatibilityInfo {
	info := &PeerCompatibilityInfo{
		PeerID:          peerID,
		Type:            PeerTypeUnknown,
		VersionMismatch: make(map[string]string),
		IsCompatible:    false,
	}

	// 1. 获取 peer 支持的所有协议
	peerProtocols, err := rtm.GetPeerProtocols(peerID)
	if err != nil {
		info.IncompatibleReason = fmt.Sprintf("无法获取协议列表: %v", err)
		return info
	}
	info.SupportedProtocols = peerProtocols

	if len(peerProtocols) == 0 {
		info.Type = PeerTypeUnknown
		info.IncompatibleReason = "协议列表为空（可能 identify 未完成）"
		return info
	}

	// 2. 检查 WES 核心协议支持情况
	coreProtocols := []string{
		protocols.ProtocolBlockSubmission,
		protocols.ProtocolSyncHelloV2,
		protocols.ProtocolKBucketSync,
	}

	supportedCoreCount := 0
	for _, coreProto := range coreProtocols {
		supported, _ := rtm.peerSupportsProtocolFromPeerstore(peerID, coreProto)
		if supported {
			supportedCoreCount++
		} else {
			info.MissingProtocols = append(info.MissingProtocols, coreProto)

			// 检查是否有版本不匹配的情况
			for _, peerProto := range peerProtocols {
				basePath := protocols.ExtractProtocolBasePath(coreProto)
				peerBasePath := protocols.ExtractProtocolBasePath(peerProto)
				if basePath != "" && basePath == peerBasePath {
					// 同一协议但版本不同
					expectedVersion := protocols.GetProtocolVersion(coreProto)
					actualVersion := protocols.GetProtocolVersion(peerProto)
					info.VersionMismatch[coreProto] = fmt.Sprintf("期望 %s, 实际 %s", expectedVersion, actualVersion)
				}
			}
		}
	}

	// 3. 判断节点类型
	switch {
	case supportedCoreCount == len(coreProtocols):
		info.Type = PeerTypeWESFull
		info.IsCompatible = true
	case supportedCoreCount > 0:
		if len(info.VersionMismatch) > 0 {
			info.Type = PeerTypeWESIncompatible
			info.IncompatibleReason = fmt.Sprintf("协议版本不匹配: %v", info.VersionMismatch)
		} else {
			info.Type = PeerTypeWESPartial
			info.IsCompatible = true // 部分兼容也算兼容
		}
	default:
		// 检查是否是 libp2p 节点（通过查找常见 libp2p 协议）
		libp2pProtocols := []string{"/ipfs/", "/libp2p/", "/meshsub/", "/floodsub/"}
		isLibp2p := false
		for _, peerProto := range peerProtocols {
			for _, libp2pPrefix := range libp2pProtocols {
				if len(peerProto) > len(libp2pPrefix) && peerProto[:len(libp2pPrefix)] == libp2pPrefix {
					isLibp2p = true
					break
				}
			}
			if isLibp2p {
				break
			}
		}
		if isLibp2p {
			info.Type = PeerTypeExternalLibp2p
			info.IncompatibleReason = "外部 libp2p 节点，不支持 WES 协议"
		} else {
			info.Type = PeerTypeUnknown
			info.IncompatibleReason = "未知节点类型"
		}
	}

	return info
}

// QuarantineWithAnalysis 带分析的隔离
// 根据节点类型采取不同的隔离策略
func (rtm *RoutingTableManager) QuarantineWithAnalysis(peerID peer.ID, requiredProto string) *PeerCompatibilityInfo {
	// 1. 分析节点兼容性
	info := rtm.AnalyzePeerCompatibility(peerID)

	// 2. 根据节点类型决定隔离策略
	var reason string
	var quarantineDuration time.Duration

	defaultQuarantineDuration := rtm.config.GetQuarantineDuration()

	switch info.Type {
	case PeerTypeExternalLibp2p:
		// 外部 libp2p 节点：长期隔离（这些节点几乎不可能变成 WES 节点）
		quarantineDuration = defaultQuarantineDuration * 2
		reason = fmt.Sprintf("external_libp2p_node:missing_%s", requiredProto)
	case PeerTypeWESIncompatible:
		// WES 版本不兼容：中等时间隔离（可能需要升级）
		quarantineDuration = defaultQuarantineDuration
		reason = fmt.Sprintf("wes_version_incompatible:%s", requiredProto)
	case PeerTypeUnknown:
		// 未知类型：短期隔离（可能是暂时问题）
		quarantineDuration = defaultQuarantineDuration / 2
		if quarantineDuration < time.Minute*5 {
			quarantineDuration = time.Minute * 5
		}
		reason = fmt.Sprintf("unknown_peer_type:missing_%s", requiredProto)
	default:
		// WES 节点但缺失特定协议：标准隔离
		quarantineDuration = defaultQuarantineDuration
		reason = fmt.Sprintf("wes_partial:missing_%s", requiredProto)
	}

	// 3. 执行隔离
	rtm.tabLock.RLock()
	defer rtm.tabLock.RUnlock()

	for _, bucket := range rtm.buckets {
		if elem := bucket.find(peerID); elem != nil {
			p := elem.Value.(*PeerInfo)

			p.stateLock.Lock()
			p.peerState = PeerStateQuarantined
			p.healthScore = 0
			p.quarantinedUntil = time.Now().Add(quarantineDuration)
			p.stateLock.Unlock()

			if rtm.logger != nil {
				rtm.logger.Infof("🔒 分析后隔离节点: peer=%s type=%s reason=%s duration=%s",
					peerID.String()[:12], info.Type, reason, quarantineDuration)
			}
			break
		}
	}

	return info
}

func (rtm *RoutingTableManager) peerSupportsProtocolFromPeerstore(peerID peer.ID, protoID string) (bool, error) {
	if protoID == "" || rtm.p2pService == nil || rtm.p2pService.Host() == nil {
		return false, fmt.Errorf("host/proto not available")
	}
	h := rtm.p2pService.Host()
	ps, err := h.Peerstore().GetProtocols(peerID)
	if err != nil {
		return false, err
	}

	// 🆕 2025-12-19 优化：使用协议变体进行更全面的匹配
	// 支持：原始协议ID、带命名空间的协议ID、不同版本的协议ID
	ns := ""
	if rtm.configProvider != nil {
		ns = rtm.configProvider.GetNetworkNamespace()
	}

	// 获取协议的所有变体（原始、带命名空间、不同版本）
	candidates := protocols.GetProtocolVariants(protoID, ns)

	// 转换为 map 以便快速查找
	candidateSet := make(map[string]struct{}, len(candidates))
	for _, c := range candidates {
		candidateSet[c] = struct{}{}
	}

	for _, p := range ps {
		if _, ok := candidateSet[string(p)]; ok {
			return true, nil
		}
	}
	return false, nil
}

// validatePeerChainIdentity 验证 peer 的链身份是否与本地匹配（vNext：不向后兼容）。
//
// 从 peer 的 UserAgent 中解析链身份信息，与本地链身份比对。
// 如果不匹配，说明是外链节点，不应加入 K 桶。
//
// 返回：
// - chainOK：是否同链
// - reason：拒绝原因（用于诊断与 lastAdd）
// - err：可恢复错误（例如 peerstore/identify 未就绪），需要上层重试
func (rtm *RoutingTableManager) validatePeerChainIdentity(ctx context.Context, peerID peer.ID) (chainOK bool, reason string, err error) {
	if rtm.configProvider == nil || rtm.p2pService == nil {
		// 缺少本地链身份来源/host：属于系统配置问题，不应“放行”导致跨链污染。
		return false, "chain_identity_unavailable", fmt.Errorf("configProvider or p2pService is nil")
	}

	host := rtm.p2pService.Host()
	if host == nil {
		return false, "chain_identity_unavailable", fmt.Errorf("libp2p host not available")
	}

	// 获取本地链身份
	appCfg := rtm.configProvider.GetAppConfig()
	if appCfg == nil {
		return false, "chain_identity_unavailable", fmt.Errorf("app config not available")
	}

	unifiedGenesis := rtm.configProvider.GetUnifiedGenesisConfig()
	if unifiedGenesis == nil {
		return false, "chain_identity_unavailable", fmt.Errorf("genesis config not available")
	}

	genesisHash, err := node.CalculateGenesisHash(unifiedGenesis)
	if err != nil {
		return false, "chain_identity_unavailable", fmt.Errorf("calculate genesis hash failed: %w", err)
	}

	localIdentity := node.BuildLocalChainIdentity(appCfg, genesisHash)

	// ✅ 优先使用“系统路径缓存”的链身份（来自 SyncHelloV2 / KBucketSync 响应），避免依赖 UserAgent
	if host != nil {
		if v, err := host.Peerstore().Get(peerID, constants.PeerstoreKeyChainIdentity); err == nil {
			if s, ok := v.(string); ok && s != "" {
				var cached types.ChainIdentity
				if uerr := json.Unmarshal([]byte(s), &cached); uerr == nil && cached.IsValid() {
					if localIdentity.IsSameChain(cached) {
						return true, "ok_cached_chain_identity", nil
					}
					return false, "chain_mismatch_cached_identity", nil
				}
			}
		}
	}

	// 环境：dev/test/prod（默认 dev）
	// - dev/test：允许一定的迁移期兼容（避免因为历史 UserAgent/启动时序导致“永远不入桶”）
	// - prod：坚持 fail-closed（UserAgent 必须携带完整链身份），防止跨链污染 K桶 影响共识/同步选路
	env := "dev"
	if appCfg != nil {
		func() {
			defer func() { _ = recover() }()
			if e := strings.ToLower(string(appCfg.GetEnvironment())); e != "" {
				env = e
			}
		}()
	}

	// 兼容策略（迁移期）：
	// 某些历史版本的 weisyn 节点 UserAgent 只包含代码版本（如 "github.com/weisyn/v1@xxxx"），不包含链身份段。
	// 但它们的协议 ID 往往已经是命名空间化的（如 "/weisyn/<ns>/sync/hello/2.0.0"），可用于“同 namespace”级别的链归属判断。
	//
	// 安全边界：
	// - 仅在 dev/test 环境允许该兜底（prod 必须严格链身份）；
	// - 仅当本地 ns 非空时才允许通过“命名空间化协议”推断同链；
	// - 一旦 UserAgent 中携带了可解析链身份，则走严格校验（chain_id / ns / mode / genesisHash8）。
	allowByNamespaceProtocol := func() (bool, error) {
		if env == "prod" {
			return false, nil
		}
		ns := localIdentity.NetworkNamespace
		if ns == "" {
			return false, nil
		}
		ps, err := host.Peerstore().GetProtocols(peerID)
		if err != nil {
			// Identify/peerstore 可能尚未就绪：交给上层重试
			return false, fmt.Errorf("get peer protocols failed: %w", err)
		}
		want := "/weisyn/" + ns + "/"
		for _, p := range ps {
			if strings.Contains(string(p), want) {
				return true, nil
			}
		}
		return false, nil
	}

	// 尝试从 UserAgent 解析 peer 的链身份
	// libp2p 的 UserAgent 存储在 peerstore 中
	userAgent, err := host.Peerstore().Get(peerID, "AgentVersion")
	if err != nil {
		// Identify/peerstore 可能尚未就绪：交给上层重试
		return false, "chain_identity_not_ready", fmt.Errorf("get AgentVersion failed: %w", err)
	}

	userAgentStr, ok := userAgent.(string)
	if !ok || userAgentStr == "" {
		// vNext：prod 必须拒绝“未携带链身份”的节点；dev/test 允许用“命名空间化协议”推断同链（仅同 ns）。
		okByNS, nsErr := allowByNamespaceProtocol()
		if nsErr != nil {
			return false, "chain_identity_not_ready", nsErr
		}
		if okByNS {
			return true, "ok_by_ns_proto", nil
		}
		return false, "chain_identity_missing", nil
	}

	// 解析 UserAgent 中的链身份（严格格式才校验，不严格则向后兼容放行）
	//
	// 期望格式（由 p2p.Options.UserAgent 生成）：
	//   <version>/<ns>/<mode>/<chainID>@<genesisHash8>
	// 示例：
	//   "github.com/weisyn/v1@98ef22e/public-testnet-demo/public/12001@fc536d38"
	//
	// 常见“非严格”格式（历史版本/外部节点）：
	//   "github.com/weisyn/v1@98ef22e"   —— 仅包含代码版本，不包含链身份
	//
	// ⚠️ 关键修复：
	// 旧逻辑会把上述非严格格式中的 "@98ef22e" 误当作 genesis hash8，从而错误拒绝同链节点（chain_ok=false）。
	parts := strings.Split(userAgentStr, "/")
	if len(parts) < 2 {
		okByNS, nsErr := allowByNamespaceProtocol()
		if nsErr != nil {
			return false, "chain_identity_not_ready", nsErr
		}
		if okByNS {
			return true, "ok_by_ns_proto", nil
		}
		return false, "chain_identity_missing", nil
	}

	// 链身份通常出现在末尾三段：ns / mode / (chainID@hash8)
	if len(parts) < 4 {
		// vNext：没有足够段数承载链身份，视为“未携带链身份”，拒绝
		okByNS, nsErr := allowByNamespaceProtocol()
		if nsErr != nil {
			return false, "chain_identity_not_ready", nsErr
		}
		if okByNS {
			return true, "ok_by_ns_proto", nil
		}
		return false, "chain_identity_missing", nil
	}

	identityStr := parts[len(parts)-1] // "12001@fc536d38" or "v1@98ef22e"
	identityParts := strings.Split(identityStr, "@")
	if len(identityParts) != 2 {
		okByNS, nsErr := allowByNamespaceProtocol()
		if nsErr != nil {
			return false, "chain_identity_not_ready", nsErr
		}
		if okByNS {
			return true, "ok_by_ns_proto", nil
		}
		return false, "chain_identity_missing", nil
	}

	remoteChainID := identityParts[0]
	remoteHash8 := identityParts[1]

	// vNext：必须携带可解析的 chain_id（数字串），否则拒绝
	isDigits := func(s string) bool {
		if s == "" {
			return false
		}
		for i := 0; i < len(s); i++ {
			if s[i] < '0' || s[i] > '9' {
				return false
			}
		}
		return true
	}
	if !isDigits(remoteChainID) {
		okByNS, nsErr := allowByNamespaceProtocol()
		if nsErr != nil {
			return false, "chain_identity_not_ready", nsErr
		}
		if okByNS {
			return true, "ok_by_ns_proto", nil
		}
		return false, "chain_identity_missing", nil
	}

	// 严格校验：chain_id 必须一致
	if remoteChainID != localIdentity.ChainID {
		rtm.logger.Debugf("policy.reject_sync_peer: 链身份不匹配 (chain_id), peer=%s remote_chain_id=%s local_chain_id=%s",
			peerID.String()[:8], remoteChainID, localIdentity.ChainID)
		return false, "chain_mismatch_chain_id", nil
	}

	// 严格校验：namespace/mode（从末尾倒数第3/第2段取）
	remoteNamespace := parts[len(parts)-3]
	if remoteNamespace != "" && remoteNamespace != localIdentity.NetworkNamespace {
		rtm.logger.Debugf("policy.reject_sync_peer: 链身份不匹配 (namespace), peer=%s remote_ns=%s local_ns=%s",
			peerID.String()[:8], remoteNamespace, localIdentity.NetworkNamespace)
		return false, "chain_mismatch_namespace", nil
	}

	remoteMode := parts[len(parts)-2]
	localMode := string(localIdentity.ChainMode)
	if remoteMode != "" && localMode != "" && remoteMode != localMode {
		rtm.logger.Debugf("policy.reject_sync_peer: 链身份不匹配 (mode), peer=%s remote_mode=%s local_mode=%s",
			peerID.String()[:8], remoteMode, localMode)
		return false, "chain_mismatch_mode", nil
	}

	// 严格校验：genesis hash 前8位
	if len(remoteHash8) >= 8 && len(localIdentity.GenesisHash) >= 8 {
		if remoteHash8[:8] != localIdentity.GenesisHash[:8] {
			rtm.logger.Debugf("policy.reject_sync_peer: 链身份不匹配 (genesis hash 前8位), peer=%s remote_hash8=%s local_hash8=%s",
				peerID.String()[:8], remoteHash8[:8], localIdentity.GenesisHash[:8])
			return false, "chain_mismatch_genesis", nil
		}
	}

	return true, "ok", nil
}

// ============================================================================
// Phase 2：清理前探测机制
// ============================================================================

// probeWorker 探测工作协程，定期扫描待探测peer并执行主动连接验证
func (rtm *RoutingTableManager) probeWorker() {
	ticker := time.NewTicker(10 * time.Second) // 每10秒扫描一次
	defer ticker.Stop()

	rtm.logger.Info("启动探测工作协程")

	for {
		select {
		case <-rtm.ctx.Done():
			rtm.logger.Info("探测工作协程收到停止信号")
			return
		case <-ticker.C:
			rtm.executePendingProbes()
		}
	}
}

// executePendingProbes 执行待探测peer的扫描和探测
func (rtm *RoutingTableManager) executePendingProbes() {
	if !rtm.IsRunning() {
		return
	}

	rtm.tabLock.RLock()
	defer rtm.tabLock.RUnlock()

	now := time.Now()
	probeIntervalMin := 30 * time.Second // 最小探测间隔

	// 收集需要探测的peers
	var pendingProbes []*PeerInfo

	for _, bucket := range rtm.buckets {
		for e := bucket.list.Front(); e != nil; e = e.Next() {
			p := e.Value.(*PeerInfo)

			p.stateLock.RLock()
			status := p.probeStatus
			lastProbe := p.lastProbeAt
			p.stateLock.RUnlock()

			if status != ProbePending {
				continue
			}

			// 限制探测频率（至少间隔30秒）
			if !lastProbe.IsZero() && now.Sub(lastProbe) < probeIntervalMin {
				continue
			}

			pendingProbes = append(pendingProbes, p)
		}
	}

	if len(pendingProbes) == 0 {
		return
	}

	rtm.logger.Debugf("发现%d个待探测peer", len(pendingProbes))

	// 并发探测（异步执行，不阻塞）
	for _, p := range pendingProbes {
		go rtm.probePeer(p)
	}
}

// probePeer 对单个peer执行主动连接探测
func (rtm *RoutingTableManager) probePeer(p *PeerInfo) {
	// 并发控制：获取信号量
	select {
	case rtm.probeSemaphore <- struct{}{}:
		defer func() { <-rtm.probeSemaphore }()
	case <-rtm.ctx.Done():
		return
	}

	// 更新最后探测时间
	p.stateLock.Lock()
	p.lastProbeAt = time.Now()
	p.stateLock.Unlock()

	// 获取peerstore中的地址
	if rtm.p2pService == nil || rtm.p2pService.Host() == nil {
		rtm.logger.Debugf("探测失败：host未就绪, peer=%s", p.Id)
		rtm.recordProbeFailure(p)
		return
	}

	host := rtm.p2pService.Host()
	addrs := host.Peerstore().Addrs(p.Id)
	if len(addrs) == 0 {
		rtm.logger.Debugf("探测失败：无地址信息, peer=%s", p.Id)
		rtm.recordProbeFailure(p)
		return
	}

	// 创建带超时的上下文（5秒超时）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 尝试重新连接
	addrInfo := peer.AddrInfo{
		ID:    p.Id,
		Addrs: addrs,
	}

	err := host.Connect(ctx, addrInfo)
	if err == nil {
		// 🎯 连接成功！取消清理，恢复Active状态
		rtm.recordProbeSuccess(p)
		rtm.logger.Infof("✅ 探测成功，peer恢复: peer=%s", p.Id)

		// 记录指标
		if rtm.metrics != nil {
			rtm.metrics.ProbeSuccessCount++
			rtm.metrics.ProbePreventedCleanup++ // 防止了一次清理
		}
	} else {
		// ❌ 连接失败
		rtm.recordProbeFailure(p)
		rtm.logger.Debugf("探测失败: peer=%s, fail_count=%d, err=%v",
			p.Id, p.probeFailCount, err)

		// 记录指标
		if rtm.metrics != nil {
			rtm.metrics.ProbeFailCount++
		}
	}

	// 记录探测尝试
	if rtm.metrics != nil {
		rtm.metrics.ProbeAttempts++
	}
}

// recordProbeSuccess 记录探测成功
func (rtm *RoutingTableManager) recordProbeSuccess(p *PeerInfo) {
	p.stateLock.Lock()
	defer p.stateLock.Unlock()

	p.probeStatus = ProbeSuccess
	p.probeFailCount = 0

	// 恢复健康状态
	p.healthScore = 100
	p.failureCount = 0
	p.LastUsefulAt = time.Now()
	p.LastSuccessfulOutboundQueryAt = time.Now()
	p.peerState = PeerStateActive
	p.quarantinedUntil = time.Time{}
}

// recordProbeFailure 记录探测失败
func (rtm *RoutingTableManager) recordProbeFailure(p *PeerInfo) {
	p.stateLock.Lock()
	defer p.stateLock.Unlock()

	p.probeFailCount++

	// 连续3次失败才标记为ProbeFailed
	if p.probeFailCount >= 3 {
		p.probeStatus = ProbeFailed
		rtm.logger.Warnf("探测连续失败3次，确认清理: peer=%s", p.Id)
	}
}

// finalCleanup 最终清理：只删除探测确认失败的peer
func (rtm *RoutingTableManager) finalCleanup(bucket *Bucket, bucketIdx int) {
	var toRemove []*list.Element

	for e := bucket.list.Front(); e != nil; e = e.Next() {
		p := e.Value.(*PeerInfo)

		p.stateLock.RLock()
		status := p.probeStatus
		p.stateLock.RUnlock()

		// 只清理探测确认失败的peer
		if status == ProbeFailed {
			toRemove = append(toRemove, e)
		}
	}

	if len(toRemove) == 0 {
		return
	}

	// 执行清理
	for _, elem := range toRemove {
		p := elem.Value.(*PeerInfo)

		bucket.remove(elem)
		rtm.peerRemoved(p.Id)

		// 记录指标
		if rtm.metrics != nil {
			rtm.metrics.RecordCleanup("probe_failed", false)
		}

		rtm.logger.Infof("最终清理探测失败peer: bucket=%d, peer=%s, fail_count=%d",
			bucketIdx, p.Id, p.probeFailCount)

		// 尝试从替换缓存提升节点
		if replacement := bucket.promoteFromReplacementCache(); replacement != nil {
			bucket.pushFront(replacement)
			rtm.logger.Infof("从替换缓存提升节点: bucket=%d, peer=%s", bucketIdx, replacement.Id)
		}
	}
}
