package keepalive

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	libnetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/weisyn/v1/internal/core/p2p/discovery"
	"github.com/weisyn/v1/pkg/constants/events"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/p2p"
	"github.com/weisyn/v1/pkg/types"
)

// 为避免循环导入，定义所需的接口别名
type RendezvousRouting = p2p.Routing

// KeyPeerMonitor 关键peer监控器
// 负责周期性探测关键peer集合，失败时触发自愈
type KeyPeerMonitor struct {
	host          host.Host
	routing       RendezvousRouting
	addrManager   *discovery.AddrManager
	keyPeerSet    *KeyPeerSet
	logger        log.Logger
	eventBus      event.EventBus
	
	// 探测状态
	lastProbeAt   map[peer.ID]time.Time
	probeFailures map[peer.ID]int
	stateMu       sync.RWMutex
	
	// 配置
	probeInterval      time.Duration  // 探测周期（默认60s）
	perPeerMinInterval time.Duration  // 单个peer最小探测间隔（默认30s）
	probeTimeout       time.Duration  // 探测超时（默认5s）
	failThreshold      int            // 失败阈值（默认3）
	maxConcurrent      int            // 最大并发探测数（默认5）
	
	probeSem      chan struct{}      // 并发控制信号量
	
	// 运行控制
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	running    bool
	runningMu  sync.RWMutex
}

// NewKeyPeerMonitor 创建KeyPeerMonitor
func NewKeyPeerMonitor(
	host host.Host,
	routing RendezvousRouting,
	addrManager *discovery.AddrManager,
	keyPeerSet *KeyPeerSet,
	logger log.Logger,
	eventBus event.EventBus,
	probeInterval time.Duration,
	perPeerMinInterval time.Duration,
	probeTimeout time.Duration,
	failThreshold int,
	maxConcurrent int,
) *KeyPeerMonitor {
	// 设置默认值
	if probeInterval <= 0 {
		probeInterval = 60 * time.Second
	}
	if perPeerMinInterval <= 0 {
		perPeerMinInterval = 30 * time.Second
	}
	if probeTimeout <= 0 {
		probeTimeout = 5 * time.Second
	}
	if failThreshold <= 0 {
		failThreshold = 3
	}
	if maxConcurrent <= 0 {
		maxConcurrent = 5
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	return &KeyPeerMonitor{
		host:               host,
		routing:            routing,
		addrManager:        addrManager,
		keyPeerSet:         keyPeerSet,
		logger:             logger,
		eventBus:           eventBus,
		lastProbeAt:        make(map[peer.ID]time.Time),
		probeFailures:      make(map[peer.ID]int),
		probeInterval:      probeInterval,
		perPeerMinInterval: perPeerMinInterval,
		probeTimeout:       probeTimeout,
		failThreshold:      failThreshold,
		maxConcurrent:      maxConcurrent,
		probeSem:           make(chan struct{}, maxConcurrent),
		ctx:                ctx,
		cancel:             cancel,
	}
}

// Start 启动监控器
func (kpm *KeyPeerMonitor) Start() error {
	kpm.runningMu.Lock()
	defer kpm.runningMu.Unlock()
	
	if kpm.running {
		return fmt.Errorf("monitor already running")
	}
	
	kpm.running = true
	kpm.wg.Add(1)
	go kpm.probeLoop()
	
	if kpm.logger != nil {
		kpm.logger.Infof("✅ KeyPeerMonitor已启动: interval=%s per_peer_min=%s timeout=%s threshold=%d concurrent=%d",
			kpm.probeInterval, kpm.perPeerMinInterval, kpm.probeTimeout, kpm.failThreshold, kpm.maxConcurrent)
	}
	
	return nil
}

// Stop 停止监控器
func (kpm *KeyPeerMonitor) Stop() error {
	kpm.runningMu.Lock()
	defer kpm.runningMu.Unlock()
	
	if !kpm.running {
		return nil
	}
	
	kpm.cancel()
	kpm.wg.Wait()
	kpm.running = false
	
	if kpm.logger != nil {
		kpm.logger.Info("KeyPeerMonitor已停止")
	}
	
	return nil
}

// probeLoop 探测循环
func (kpm *KeyPeerMonitor) probeLoop() {
	defer kpm.wg.Done()
	
	ticker := time.NewTicker(kpm.probeInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-kpm.ctx.Done():
			return
		case <-ticker.C:
			kpm.runProbeRound()
		}
	}
}

// runProbeRound 执行一轮探测
func (kpm *KeyPeerMonitor) runProbeRound() {
	// 允许在“未注入真实host”的测试/降级模式下运行：直接跳过探测，避免空指针崩溃
	if kpm == nil || kpm.host == nil || kpm.keyPeerSet == nil {
		if kpm != nil && kpm.logger != nil {
			kpm.logger.Debug("KeyPeerMonitor未就绪（host/keyPeerSet为空），跳过本轮探测")
		}
		return
	}

	// 清理KeyPeerSet中过期的recentlyUseful记录
	kpm.keyPeerSet.Cleanup()
	
	// 获取所有关键peer
	keyPeers := kpm.keyPeerSet.GetAllKeyPeers()
	if len(keyPeers) == 0 {
		if kpm.logger != nil {
			kpm.logger.Debug("KeyPeerSet为空，跳过本轮探测")
		}
		return
	}
	
	if kpm.logger != nil {
		kpm.logger.Debugf("开始KeyPeer探测轮次: key_peers=%d", len(keyPeers))
	}
	
	now := time.Now()
	probeCount := 0
	skippedCount := 0
	
	for _, p := range keyPeers {
		// 检查是否满足per-peer最小间隔
		kpm.stateMu.RLock()
		lastProbe, exists := kpm.lastProbeAt[p]
		kpm.stateMu.RUnlock()
		
		if exists && now.Sub(lastProbe) < kpm.perPeerMinInterval {
			skippedCount++
			continue
		}
		
		// 检查连接状态
		connectedness := kpm.host.Network().Connectedness(p)
		if connectedness == libnetwork.Connected {
			// 已连接，重置失败计数
			kpm.stateMu.Lock()
			kpm.probeFailures[p] = 0
			kpm.lastProbeAt[p] = now
			kpm.stateMu.Unlock()
			continue
		}
		
		// 需要探测
		probeCount++
		kpm.wg.Add(1)
		go func(peerID peer.ID) {
			defer kpm.wg.Done()
			
			// 获取信号量
			select {
			case kpm.probeSem <- struct{}{}:
				defer func() { <-kpm.probeSem }()
			case <-kpm.ctx.Done():
				return
			}
			
			kpm.probePeer(peerID)
		}(p)
	}
	
	if kpm.logger != nil {
		kpm.logger.Debugf("KeyPeer探测轮次完成: probed=%d skipped=%d total=%d", probeCount, skippedCount, len(keyPeers))
	}
}

// probePeer 探测单个peer
func (kpm *KeyPeerMonitor) probePeer(p peer.ID) {
	if kpm == nil || kpm.host == nil {
		// 测试/降级模式：无真实host时不探测
		return
	}

	if kpm.logger != nil {
		kpm.logger.Debugf("探测peer: %s", p)
	}
	
	ctx, cancel := context.WithTimeout(kpm.ctx, kpm.probeTimeout)
	defer cancel()
	
	// 获取peer的地址信息
	addrs := kpm.host.Peerstore().Addrs(p)
	if len(addrs) == 0 {
		if kpm.logger != nil {
			kpm.logger.Debugf("peer %s 无地址，跳过探测", p)
		}
		return
	}
	
	// 尝试连接
	addrInfo := peer.AddrInfo{ID: p, Addrs: addrs}
	err := kpm.host.Connect(ctx, addrInfo)
	
	kpm.stateMu.Lock()
	kpm.lastProbeAt[p] = time.Now()
	
	if err != nil {
		// 探测失败
		kpm.probeFailures[p]++
		failCount := kpm.probeFailures[p]
		kpm.stateMu.Unlock()
		
		if kpm.logger != nil {
			kpm.logger.Warnf("探测peer失败: %s, 失败次数=%d/%d, 错误: %v", p, failCount, kpm.failThreshold, err)
		}
		
		// 达到失败阈值，触发自愈
		if failCount >= kpm.failThreshold {
			kpm.repairPeer(p)
		}
	} else {
		// 探测成功
		kpm.probeFailures[p] = 0
		kpm.stateMu.Unlock()
		
		if kpm.logger != nil {
			kpm.logger.Debugf("探测peer成功: %s", p)
		}
	}
}

// repairPeer 修复peer连接
func (kpm *KeyPeerMonitor) repairPeer(p peer.ID) {
	if kpm.logger != nil {
		kpm.logger.Infof("🔧 开始修复peer连接: %s", p)
	}
	
	// 1. 快速重连（使用当前地址）
	ctx, cancel := context.WithTimeout(kpm.ctx, kpm.probeTimeout)
	addrs := kpm.host.Peerstore().Addrs(p)
	if len(addrs) > 0 {
		addrInfo := peer.AddrInfo{ID: p, Addrs: addrs}
		err := kpm.host.Connect(ctx, addrInfo)
		cancel()
		
		if err == nil {
			// 重连成功
			kpm.stateMu.Lock()
			kpm.probeFailures[p] = 0
			kpm.stateMu.Unlock()
			
			if kpm.logger != nil {
				kpm.logger.Infof("✅ 快速重连成功: %s", p)
			}
			return
		}
		
		if kpm.logger != nil {
			kpm.logger.Warnf("快速重连失败: %s, 错误: %v", p, err)
		}
	} else {
		cancel()
	}
	
	// 2. DHT补地址
	if kpm.routing != nil {
		ctx, cancel = context.WithTimeout(kpm.ctx, 30*time.Second)
		newAddrInfo, err := kpm.routing.FindPeer(ctx, p)
		cancel()
		
		if err != nil {
			if kpm.logger != nil {
				kpm.logger.Warnf("DHT FindPeer失败: %s, 错误: %v", p, err)
			}
		} else if len(newAddrInfo.Addrs) > 0 {
			if kpm.logger != nil {
				kpm.logger.Infof("通过DHT找到新地址: %s, addrs=%d", p, len(newAddrInfo.Addrs))
			}
			
			// 3. 使用新地址二次重连
			ctx, cancel = context.WithTimeout(kpm.ctx, kpm.probeTimeout)
			err = kpm.host.Connect(ctx, newAddrInfo)
			cancel()
			
			if err == nil {
				kpm.stateMu.Lock()
				kpm.probeFailures[p] = 0
				kpm.stateMu.Unlock()
				
				if kpm.logger != nil {
					kpm.logger.Infof("✅ 使用新地址重连成功: %s", p)
				}
				return
			}
			
			if kpm.logger != nil {
				kpm.logger.Warnf("使用新地址重连失败: %s, 错误: %v", p, err)
			}
		}
	}
	
	// 4. 发布Discovery间隔重置事件
	if kpm.eventBus != nil {
		resetData := &types.DiscoveryResetEventData{
			Reason:    "peer_disconnected",
			Trigger:   "keypeer_monitor",
			PeerID:    p.String(),
			Timestamp: time.Now().Unix(),
		}
		kpm.eventBus.Publish(events.EventTypeDiscoveryIntervalReset, resetData)
		
		if kpm.logger != nil {
			kpm.logger.Infof("🔄 关键peer修复失败，已触发Discovery间隔重置: %s", p)
		}
	}
}

