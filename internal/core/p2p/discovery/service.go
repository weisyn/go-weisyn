package discovery

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	lphost "github.com/libp2p/go-libp2p/core/host"
	libnetwork "github.com/libp2p/go-libp2p/core/network"
	libpeer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	mdns "github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"

	p2pcfg "github.com/weisyn/v1/internal/config/p2p"
	"github.com/weisyn/v1/internal/core/p2p/interfaces"
	"github.com/weisyn/v1/pkg/constants/events"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	metricsutil "github.com/weisyn/v1/pkg/utils/metrics"
	"github.com/weisyn/v1/pkg/types"
)

// Service Discovery 服务实现
//
// 统一调度 Bootstrap / mDNS / Rendezvous 等发现插件
type Service struct {
	host     lphost.Host
	opts     *p2pcfg.Options
	logger   logiface.Logger
	eventBus event.EventBus
	mdnsSvc  mdns.Service
	// rendezvousRouting 通过内部接口协作的 Rendezvous 路由能力（由 Routing 子系统注入）
	rendezvousRouting interfaces.RendezvousRouting
	ctx               context.Context
	cancel            context.CancelFunc

	// 调度器相关
	schedulerCancel context.CancelFunc
	dhtLoopCancel   context.CancelFunc
	mu              sync.RWMutex

	// 诊断指标回调（可选）
	recordBootstrapAttempt   func()
	recordBootstrapSuccess   func()
	recordMDNSPeerFound      func()
	recordMDNSConnectSuccess func()
	recordMDNSConnectFail    func()
	updateLastBootstrapTS    func()
	updateLastMDNSTS         func()

	// 地址管理器
	addrManager *AddrManager

	// 实例数据目录（用于构建存储路径）
	instanceDataDir string

	// Phase 3: 间隔重置机制
	schedulerResetChan chan struct{} // bootstrap调度器重置通道
	dhtResetChan       chan struct{} // DHT rendezvous重置通道
	lastResetAt        time.Time     // 最后一次重置时间（用于冷却）
	resetMu            sync.Mutex    // 重置操作保护锁

	// 🆕 2025-12-18: Peer ID 不匹配治愈缓存
	// 避免对同一 (expected, addr) 组合重复输出 WARN 日志
	peerMismatchCache   map[string]time.Time // key: "expected:addr" -> 首次治愈时间
	peerMismatchMu      sync.RWMutex
	peerMismatchTotal   int64 // 总治愈次数（用于统计）
	peerMismatchUnique  int64 // 唯一组合次数
}

var _ p2pi.Discovery = (*Service)(nil)

// healPeerIDMismatch 尝试对 "peer id mismatch" 做本地自愈：
// - 从 expected peer 的 peerstore 中移除该 addr（避免继续连错人/污染选举&同步候选）
// - 将该 addr 归档到 actual peer（从错误文本中解析出的 remote key matches）
//
// 说明：
// - 这是 "addr->peer 映射纠错" 的系统路径修复点，属于生产级闭环；否则会长期出现"拨号到某地址但对端 peerID 不一致"。
// - 这里使用 TempAddrTTL 写入 actual peer，避免把错误的 DHT addr 永久固化。
//
// 🆕 2025-12-18 优化：
// - 添加缓存避免对同一 (expected, addr) 组合重复输出 WARN 日志
// - 首次发现: WARN，后续发现: DEBUG
// - 添加统计计数
func (s *Service) healPeerIDMismatch(expected libpeer.ID, addr ma.Multiaddr, dialErr error) bool {
	if s == nil || s.host == nil || expected == "" || addr == nil || dialErr == nil {
		return false
	}
	msg := dialErr.Error()
	if !strings.Contains(msg, "peer id mismatch") || !strings.Contains(msg, "remote key matches") {
		return false
	}

	// 尝试解析 actual peer："... remote key matches <peerID>"
	actualStr := ""
	if idx := strings.Index(msg, "remote key matches"); idx >= 0 {
		rest := strings.TrimSpace(msg[idx+len("remote key matches"):])
		// 截断到第一个空白/逗号/括号/方括号
		for i, r := range rest {
			if r == ' ' || r == ',' || r == ')' || r == ']' || r == '\n' || r == '\r' || r == '\t' {
				rest = rest[:i]
				break
			}
		}
		actualStr = strings.TrimSpace(rest)
	}
	if actualStr == "" {
		return false
	}
	actual, err := libpeer.Decode(actualStr)
	if err != nil || actual == "" {
		return false
	}
	if actual == expected {
		// 理论上不会出现，但防御一下避免误删
		return false
	}

	// 🆕 检查缓存：是否已经处理过这个 (expected, addr) 组合
	cacheKey := expected.String() + ":" + addr.String()
	isFirstTime := false

	s.peerMismatchMu.Lock()
	if s.peerMismatchCache == nil {
		s.peerMismatchCache = make(map[string]time.Time)
	}
	if _, exists := s.peerMismatchCache[cacheKey]; !exists {
		// 首次发现
		s.peerMismatchCache[cacheKey] = time.Now()
		s.peerMismatchUnique++
		isFirstTime = true
	}
	s.peerMismatchTotal++
	totalCount := s.peerMismatchTotal
	uniqueCount := s.peerMismatchUnique
	s.peerMismatchMu.Unlock()

	// 1) 从 expected 的地址集中移除该 addr
	current := s.host.Peerstore().Addrs(expected)
	filtered := make([]ma.Multiaddr, 0, len(current))
	for _, a := range current {
		if a == nil {
			continue
		}
		if a.Equal(addr) {
			continue
		}
		filtered = append(filtered, a)
	}
	// 清空再回填（libp2p 没有 "remove single addr" 的通用接口）
	s.host.Peerstore().ClearAddrs(expected)
	if len(filtered) > 0 {
		s.host.Peerstore().AddAddrs(expected, filtered, peerstore.PermanentAddrTTL)
	}

	// 2) 将该 addr 归档到 actual peer（临时 TTL，等待后续健康探测/握手校验再次确认）
	s.host.Peerstore().AddAddrs(actual, []ma.Multiaddr{addr}, peerstore.TempAddrTTL)

	// 🆕 区分首次和重复发现的日志级别
	if s.logger != nil {
		if isFirstTime {
			// 首次发现：WARN（便于运维关注）
			s.logger.Warnf(
				"p2p.discovery.peer_id_mismatch_healed expected=%s actual=%s addr=%s (first_time=true, total=%d, unique=%d)",
				expected.String()[:12], actual.String()[:12], addr.String(), totalCount, uniqueCount,
			)
		} else {
			// 重复发现：DEBUG（避免刷屏）
			s.logger.Debugf(
				"p2p.discovery.peer_id_mismatch_healed expected=%s actual=%s addr=%s (first_time=false, total=%d)",
				expected.String()[:12], actual.String()[:12], addr.String(), totalCount,
			)
		}
	}
	return true
}

// GetPeerMismatchStats 返回 peer ID 不匹配治愈的统计信息
//
// 🆕 2025-12-18：用于监控和诊断
func (s *Service) GetPeerMismatchStats() (total int64, unique int64) {
	s.peerMismatchMu.RLock()
	defer s.peerMismatchMu.RUnlock()
	return s.peerMismatchTotal, s.peerMismatchUnique
}

// CleanupPeerMismatchCache 清理过期的 peer ID 不匹配缓存条目
//
// 🆕 2025-12-18：定期清理，避免缓存无限增长
// 保留最近 1 小时内的条目
func (s *Service) CleanupPeerMismatchCache() {
	s.peerMismatchMu.Lock()
	defer s.peerMismatchMu.Unlock()

	if s.peerMismatchCache == nil {
		return
	}

	cutoff := time.Now().Add(-1 * time.Hour)
	for key, ts := range s.peerMismatchCache {
		if ts.Before(cutoff) {
			delete(s.peerMismatchCache, key)
		}
	}
}

// healPeerIDMismatchFromAggregateError 尝试从 “all dials failed” 的聚合错误文本中提取 addr 并做纠错。
// 典型行格式：
//   - [/ip4/.../tcp/28683] failed to negotiate security protocol: peer id mismatch: expected <A>, but remote key matches <B>
func (s *Service) healPeerIDMismatchFromAggregateError(expected libpeer.ID, dialErr error) {
	if s == nil || s.host == nil || expected == "" || dialErr == nil {
		return
	}
	msg := dialErr.Error()
	if !strings.Contains(msg, "peer id mismatch") || !strings.Contains(msg, "remote key matches") {
		return
	}
	lines := strings.Split(msg, "\n")
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if !strings.Contains(ln, "peer id mismatch") || !strings.Contains(ln, "remote key matches") {
			continue
		}
		// 提取 "* [<addr>]" 段
		lb := strings.Index(ln, "[")
		rb := strings.Index(ln, "]")
		if lb < 0 || rb <= lb {
			continue
		}
		addrStr := strings.TrimSpace(ln[lb+1 : rb])
		if addrStr == "" {
			continue
		}
		maddr, err := ma.NewMultiaddr(addrStr)
		if err != nil {
			continue
		}
		_ = s.healPeerIDMismatch(expected, maddr, fmt.Errorf("%s", ln))
	}
}

// NewService 创建 Discovery 服务
func NewService() *Service {
	return &Service{
		// Phase 3: 初始化重置通道
		schedulerResetChan: make(chan struct{}, 1), // 带缓冲避免阻塞
		dhtResetChan:       make(chan struct{}, 1),
	}
}

// Initialize 初始化 Discovery 服务（需要 Host 和配置）
func (s *Service) Initialize(host lphost.Host, opts *p2pcfg.Options, logger logiface.Logger, eb event.EventBus) error {
	if host == nil {
		return fmt.Errorf("host is required")
	}

	s.host = host
	s.opts = opts
	s.logger = logger
	s.eventBus = eb
	s.ctx, s.cancel = context.WithCancel(context.Background())

	// 初始化地址管理器（P2P 基础设施，内部实现，用户无需配置）
	//
	// 使用经过生产验证的默认配置：
	// - DHT 地址 TTL: 30分钟（频繁刷新，保证可达性）
	// - 连接成功地址 TTL: 24小时（稳定节点长期保留）
	// - 失败地址 TTL: 5分钟（快速淘汰不可达节点）
	// - 持久化到: {instanceDataDir}/p2p/addrs/（自动创建）
	if host != nil {
		// 构建存储路径：优先使用实例数据目录，回退到工作区根目录
		var badgerDir string
		if s.instanceDataDir != "" {
			// 使用链专属数据目录：data/test/test-public-xxx/p2p/addrs
			badgerDir = fmt.Sprintf("%s/p2p/addrs", s.instanceDataDir)
		} else {
			// 回退方案（兼容旧行为）：data/p2p/<hostID>/addrs
			hostID := host.ID().String()
			badgerDir = fmt.Sprintf("data/p2p/%s/addrs", hostID)
			if logger != nil {
				logger.Warnf("⚠️ instanceDataDir 未设置，AddrManager 使用回退路径: %s", badgerDir)
			}
		}

		// 内部默认配置（无需用户手工 JSON，按节点角色自动推导）
		// - 对 bootstrap/DHT server：允许更大的 peer 上限，但仍然必须有界（避免 4GB 容器 OOM）
		// - 对普通节点：上限更小
		isBootLike := false
		if s.opts != nil {
			if strings.ToLower(strings.TrimSpace(s.opts.DHTMode)) == "server" {
				isBootLike = true
			}
			if s.opts.Profile == p2pcfg.ProfileServer {
				isBootLike = true
			}
		}
		maxTrackedPeers := 5000
		refreshBudget := 500
		maxAddrsPerPeer := 8
		// 🆕 优化：大幅降低队列上限，防止内存泄漏（从5000降到50）
		maxRediscoveryQueue := 50
		if isBootLike {
			maxTrackedPeers = 20000
			refreshBudget = 1500
			// bootstrap节点稍大但也要控制（从10000降到100）
			maxRediscoveryQueue = 100
		}

		// 内部固定配置（经过生产验证的最佳实践）+ 有界化参数
		amCfg := AddrManagerConfig{
			TTL: AddrTTL{
				// 🆕 P0-009: DHT 地址 TTL 过短会导致 refresh 窗口过小（FindPeer 连续失败即过期 -> addrs=0 -> 网络孤岛）
				// 将 DHT TTL 拉长到 2h，为 refresh/rediscovery 提供更宽的容错窗口。
				DHT:       2 * time.Hour,
				Connected: 24 * time.Hour,
				Bootstrap: peerstore.PermanentAddrTTL,
				Failed:    5 * time.Minute,
			},
			MaxConcurrentLookups:   10,
			LookupTimeout:          30 * time.Second,
			RefreshInterval:        10 * time.Minute,
			// 🆕 P0-009: 提前刷新，避免接近过期时再查询导致“只剩 1-2 次机会”
			RefreshThreshold:       30 * time.Minute,
			MaxTrackedPeers:        maxTrackedPeers,
			RefreshBudget:          refreshBudget,
			MaxAddrsPerPeer:        maxAddrsPerPeer,
			MaxPendingLookups:      maxTrackedPeers, // 与 peer 上限同量级即可
			MaxRediscoveryQueue:    maxRediscoveryQueue,
			EnablePersistence:      true,
			PersistenceBackend:     "badger",
			BadgerDir:              badgerDir,
			NamespacePrefix:        "peer_addrs/v1/",
			PruneInterval:          1 * time.Hour,
			RecordTTL:              7 * 24 * time.Hour,
			RediscoveryInterval:    30 * time.Second,
			RediscoveryMaxRetries:  10,
			RediscoveryBackoffBase: 1 * time.Minute,
		}

		// 注意：这里rendezvousRouting还未注入，会在SetRendezvousRouting时可用
		s.addrManager = NewAddrManager(host, nil, amCfg, logger)

		if logger != nil {
			logger.Infof(
				"✅ AddrManager 已初始化（内部实现，自动管理节点地址），存储路径: %s (maxTrackedPeers=%d refreshBudget=%d maxAddrsPerPeer=%d)",
				badgerDir, maxTrackedPeers, refreshBudget, maxAddrsPerPeer,
			)
		}

		// ✅ 将 bootstrap peers 标记为永久保留（避免有界化误淘汰关键节点）
		if s.opts != nil && len(s.opts.BootstrapPeers) > 0 {
			peers := s.filterBootstrapPeers(s.opts.BootstrapPeers)
			for _, p := range peers {
				m, err := ma.NewMultiaddr(p)
				if err != nil {
					continue
				}
				ai, err := libpeer.AddrInfoFromP2pAddr(m)
				if err != nil || ai == nil || ai.ID == "" || len(ai.Addrs) == 0 {
					continue
				}
				s.addrManager.AddBootstrapAddr(ai.ID, ai.Addrs)
			}
		}

		// ✅ 注册到 MemoryDoctor（用于采样 peerstore/队列规模）
		metricsutil.RegisterMemoryReporter(s.addrManager)
	}

	// 🔧 Phase 3: 订阅Discovery间隔重置事件
	if eb != nil {
		err := eb.Subscribe(events.EventTypeDiscoveryIntervalReset, func(data interface{}) {
			// 触发scheduler和DHT循环重置
			select {
			case s.schedulerResetChan <- struct{}{}:
			default: // 如果通道已满，忽略（防止阻塞）
			}

			select {
			case s.dhtResetChan <- struct{}{}:
			default: // 如果通道已满，忽略
			}

			if s.logger != nil {
				if resetData, ok := data.(*types.DiscoveryResetEventData); ok {
					s.logger.Infof("🔄 收到Discovery间隔重置事件: reason=%s trigger=%s", resetData.Reason, resetData.Trigger)
				} else {
					s.logger.Info("🔄 收到Discovery间隔重置事件")
				}
			}
		})

		if err != nil && logger != nil {
			logger.Warnf("订阅Discovery间隔重置事件失败: %v", err)
		} else if logger != nil {
			logger.Debug("✅ 已订阅Discovery间隔重置事件")
		}
	}

	return nil
}

// SetRendezvousRouting 设置 Rendezvous 路由实现（由 Runtime 在初始化 Routing 后调用）
func (s *Service) SetRendezvousRouting(r interfaces.RendezvousRouting) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rendezvousRouting = r

	// 更新地址管理器的routing引用
	if s.addrManager != nil {
		s.addrManager.routing = r
	}
}

// SetInstanceDataDir 设置实例数据目录（用于构建 AddrManager 存储路径）
//
// 应该在 Initialize 之前调用，以便 AddrManager 使用正确的路径。
// 如果在 Initialize 之后调用，需要重新初始化 AddrManager。
func (s *Service) SetInstanceDataDir(dataDir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.instanceDataDir = dataDir
}

// SetDiagnosticsCallbacks 设置诊断指标回调（可选）
func (s *Service) SetDiagnosticsCallbacks(
	recordBootstrapAttempt func(),
	recordBootstrapSuccess func(),
	recordMDNSPeerFound func(),
	recordMDNSConnectSuccess func(),
	recordMDNSConnectFail func(),
	updateLastBootstrapTS func(),
	updateLastMDNSTS func(),
) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recordBootstrapAttempt = recordBootstrapAttempt
	s.recordBootstrapSuccess = recordBootstrapSuccess
	s.recordMDNSPeerFound = recordMDNSPeerFound
	s.recordMDNSConnectSuccess = recordMDNSConnectSuccess
	s.recordMDNSConnectFail = recordMDNSConnectFail
	s.updateLastBootstrapTS = updateLastBootstrapTS
	s.updateLastMDNSTS = updateLastMDNSTS
}

// Start 启动发现服务
func (s *Service) Start(ctx context.Context) error {
	if s.host == nil {
		return fmt.Errorf("discovery service not initialized")
	}

	// 预过滤 bootstrap peers：避免无效 multiaddr/占位符导致 discovery 循环刷屏，
	// 并确保后续调度器仅对“可解析”的地址进行拨号。
	var validBootstrapPeers []string
	if s.opts != nil && len(s.opts.BootstrapPeers) > 0 {
		validBootstrapPeers = s.filterBootstrapPeers(s.opts.BootstrapPeers)
	}

	// 打印 Discovery 关键配置快照，便于现场排障
	if s.logger != nil && s.opts != nil {
		s.logger.Infof(
			"p2p.discovery.config enable_mdns=%t enable_dht=%t bootstrap_peers=%d discovery_interval=%s advertise_interval=%s rendezvous_ns=%s min_peers=%d max_peers=%d",
			s.opts.EnableMDNS,
			s.opts.EnableDHT,
			len(validBootstrapPeers),
			s.opts.DiscoveryInterval,
			s.opts.AdvertiseInterval,
			s.getRendezvousNamespace(),
			s.opts.MinPeers,
			s.opts.MaxPeers,
		)
	}

	// 启动 mDNS（如果启用）
	if s.opts != nil && s.opts.EnableMDNS {
		if err := s.startMDNS(); err != nil {
			if s.logger != nil {
				s.logger.Warnf("p2p.discovery.mdns start failed: %v", err)
			}
			// mDNS 失败不阻断其他发现机制
		}
	}

	// 启动 Bootstrap 调度器循环（带退避策略）
	if s.opts != nil && len(validBootstrapPeers) > 0 {
		schedulerCtx, schedulerCancel := context.WithCancel(s.ctx)
		s.schedulerCancel = schedulerCancel
		go s.schedulerLoop(schedulerCtx, validBootstrapPeers)
	} else if s.logger != nil && s.opts != nil && len(s.opts.BootstrapPeers) > 0 {
		// 配置里声明了 bootstrap peers，但全部无效/占位符：给出一次性、可操作的告警。
		s.logger.Warnf(
			"p2p.discovery.bootstrap disabled: all configured bootstrap_peers are invalid/placeholder (configured=%d, valid=0). "+
				"this node will likely stay isolated unless you enable mDNS (enable_mdns=true) or manually connect via wes_admin_connectPeer / POST /api/v1/admin/p2p/connect",
			len(s.opts.BootstrapPeers),
		)
	}

	// 启动 DHT Rendezvous 发现循环（如果启用 DHT）
	if s.opts != nil && s.opts.EnableDHT {
		// 单节点 / 孤立网络模式：显式关闭 DHT rendezvous 循环，避免在明知只有一个节点的环境下空跑
		if s.opts.DiscoverySingleNodeMode || s.opts.DiscoveryExpectedMinPeers == 0 {
			if s.logger != nil {
				s.logger.Infof("p2p.discovery.dht_rendezvous skipped: single_node_mode=%t expected_min_peers=%d",
					s.opts.DiscoverySingleNodeMode, s.opts.DiscoveryExpectedMinPeers)
			}
		} else {
			s.mu.RLock()
			rendezvous := s.rendezvousRouting
			s.mu.RUnlock()

			if rendezvous != nil {
				ns := s.getRendezvousNamespace()
				if ns != "" {
					dhtLoopCtx, dhtLoopCancel := context.WithCancel(s.ctx)
					s.dhtLoopCancel = dhtLoopCancel
					go s.findPeersLoop(dhtLoopCtx, ns)
				}
			} else if s.logger != nil {
				s.logger.Warnf("p2p.discovery.dht_rendezvous disabled: rendezvous routing not available")
			}
		}
	}

	// 启动地址管理器
	if s.addrManager != nil {
		s.addrManager.Start()
	}

	// 🆕 2025-12-18: 启动 peer mismatch 缓存清理协程
	go s.peerMismatchCacheCleanupLoop(s.ctx)

	if s.logger != nil {
		s.logger.Infof("p2p.discovery service started")
	}

	return nil
}

// peerMismatchCacheCleanupLoop 定期清理 peer mismatch 缓存
//
// 🆕 2025-12-18: 每 30 分钟清理一次过期条目，避免缓存无限增长
func (s *Service) peerMismatchCacheCleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.CleanupPeerMismatchCache()
			if s.logger != nil {
				total, unique := s.GetPeerMismatchStats()
				s.logger.Debugf("p2p.discovery.peer_mismatch_cache_cleanup total_healed=%d unique_combinations=%d", total, unique)
			}
		}
	}
}

// Stop 停止发现服务
func (s *Service) Stop(ctx context.Context) error {
	// 停止调度器循环
	if s.schedulerCancel != nil {
		s.schedulerCancel()
		s.schedulerCancel = nil
	}

	// 停止 DHT Rendezvous 循环
	if s.dhtLoopCancel != nil {
		s.dhtLoopCancel()
		s.dhtLoopCancel = nil
	}

	// 停止主 context
	if s.cancel != nil {
		s.cancel()
	}

	if s.mdnsSvc != nil {
		if err := s.mdnsSvc.Close(); err != nil {
			if s.logger != nil {
				s.logger.Warnf("p2p.discovery.mdns close failed: %v", err)
			}
		}
		s.mdnsSvc = nil
	}

	// 停止地址管理器
	if s.addrManager != nil {
		s.addrManager.Stop()
	}

	if s.logger != nil {
		s.logger.Infof("p2p.discovery service stopped")
	}

	return nil
}

// Trigger 触发一次发现（reason 用于日志）
func (s *Service) Trigger(reason string) {
	if s.logger != nil {
		s.logger.Infof("p2p.discovery trigger: %s", reason)
	}

	// 重新连接到 Bootstrap Peers（一次性）
	if s.opts != nil && len(s.opts.BootstrapPeers) > 0 {
		peers := s.filterBootstrapPeers(s.opts.BootstrapPeers)
		if len(peers) > 0 {
			go s.tryDialOnce(context.Background(), peers)
		} else if s.logger != nil {
			s.logger.Debugf("p2p.discovery.trigger skipped: no valid bootstrap peers (reason=%s)", reason)
		}
	}
}

// filterBootstrapPeers 过滤无效/占位符 bootstrap peers，并在检测到问题时输出一次性诊断信息。
//
// 目标：
// - 避免 schedulerLoop 对无效地址进行无限重试，产生大量 error 噪音；
// - 给出可操作的修复建议（替换真实 multiaddr / 开启 mDNS / 使用 admin connect）。
func (s *Service) filterBootstrapPeers(peers []string) []string {
	if len(peers) == 0 {
		return nil
	}

	valid := make([]string, 0, len(peers))
	var invalid []string
	var placeholder []string

	for _, p := range peers {
		// 明确识别“文档占位符”，避免每次都走 multiaddr 解析再报错
		if strings.Contains(p, "ExampleBootstrapPeerReplaceMe") {
			placeholder = append(placeholder, p)
			continue
		}

		m, err := ma.NewMultiaddr(p)
		if err != nil {
			invalid = append(invalid, p)
			continue
		}
		if _, err := libpeer.AddrInfoFromP2pAddr(m); err != nil {
			invalid = append(invalid, p)
			continue
		}
		valid = append(valid, p)
	}

	if s.logger != nil {
		// 仅在发现问题时输出告警，避免正常场景刷屏
		if len(placeholder) > 0 {
			s.logger.Warnf(
				"p2p.discovery.bootstrap_peers_placeholder detected=%d (example=%s). "+
					"please replace with real multiaddr (/ip4/<ip>/tcp/28683/p2p/<peerId>) for this chain, or enable mDNS for LAN testing",
				len(placeholder),
				placeholder[0],
			)
		}
		if len(invalid) > 0 {
			s.logger.Warnf(
				"p2p.discovery.bootstrap_peers_invalid detected=%d (example=%s). "+
					"invalid peers will be ignored",
				len(invalid),
				invalid[0],
			)
		}
	}

	return valid
}

// SubscribeHints 订阅网络质量/业务 Hint，触发一次短促引导拨号
//
// 当收到 EventTypeNetworkQualityChanged 事件时，会触发一次轻量引导拨号尝试，
// 用于在网络质量变化或业务层异常时快速修复连接，而不需要等待下一个 discovery 周期。
//
// - ctx: 生命周期由 Runtime 管理，Stop 时 cancel
// - bus: EventBus 实例，允许为 nil（nil 时直接返回）
func (s *Service) SubscribeHints(ctx context.Context, bus event.EventBus) {
	if bus == nil || s == nil || s.host == nil {
		return
	}
	if s.opts == nil || len(s.opts.BootstrapPeers) == 0 {
		if s.logger != nil {
			s.logger.Debugf("p2p.discovery.hints skip: no bootstrap peers configured")
		}
		return
	}

	if s.logger != nil {
		s.logger.Infof("p2p.discovery.hints subscribe event=%s peers=%d", event.EventTypeNetworkQualityChanged, len(s.opts.BootstrapPeers))
	}

	_ = bus.Subscribe(event.EventTypeNetworkQualityChanged, func(_ event.Event) error {
		if s.logger != nil {
			s.logger.Debugf("p2p.discovery.hints trigger event=%s", event.EventTypeNetworkQualityChanged)
		}

		go func() {
			// 使用短生命周期的 context（30秒超时），避免与 Runtime 的大 ctx 混在一起
			localCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			// 轻量短促尝试：复用现有的拨号逻辑
			ok, _ := s.tryDialOnce(localCtx, s.opts.BootstrapPeers)
			if !ok {
				if s.logger != nil {
					s.logger.Debugf("p2p.discovery.hints first_try_failed retry_after=2s")
				}
				// 如同旧实现，再轻尝试一次（2s 延迟）
				time.Sleep(2 * time.Second)
				_, _ = s.tryDialOnce(localCtx, s.opts.BootstrapPeers)
			}
		}()

		return nil
	})
}

// startMDNS 启动 mDNS 服务
func (s *Service) startMDNS() error {
	// 注意：mDNS 的 service name 必须在同一局域网内保持一致，否则节点互相“看不见”。
	// 之前这里硬编码为 "weisyn-p2p"，会导致与配置系统（node.discovery.mdns.service_name，通常为 weisyn-node-<networkNamespace>）
	// 不一致，从而出现“局域网节点无法发现”的问题。
	serviceName := "weisyn-node"
	if s.opts != nil && strings.TrimSpace(s.opts.MDNSServiceName) != "" {
		serviceName = strings.TrimSpace(s.opts.MDNSServiceName)
	}

	s.mu.RLock()
	recordMDNSPeerFound := s.recordMDNSPeerFound
	recordMDNSConnectSuccess := s.recordMDNSConnectSuccess
	recordMDNSConnectFail := s.recordMDNSConnectFail
	updateLastMDNSTS := s.updateLastMDNSTS
	s.mu.RUnlock()

	notifee := &mdnsNotifee{
		host:                     s.host,
		logger:                   s.logger,
		eventBus:                 s.eventBus,
		recordMDNSPeerFound:      recordMDNSPeerFound,
		recordMDNSConnectSuccess: recordMDNSConnectSuccess,
		recordMDNSConnectFail:    recordMDNSConnectFail,
		updateLastMDNSTS:         updateLastMDNSTS,
	}

	s.mdnsSvc = mdns.NewMdnsService(s.host, serviceName, notifee)
	if err := s.mdnsSvc.Start(); err != nil {
		return fmt.Errorf("start mdns: %w", err)
	}

	if s.logger != nil {
		s.logger.Infof("p2p.discovery.mdns started service=%s", serviceName)
	}

	return nil
}

// getRendezvousNamespace 获取 Rendezvous 命名空间
func (s *Service) getRendezvousNamespace() string {
	if s.opts != nil && s.opts.DiscoveryNamespace != "" {
		return s.opts.DiscoveryNamespace
	}
	// 理论上 opts 由 internal/config/p2p 统一生成并带有默认值，这里返回空表示不启用 DHT rendezvous
	return ""
}

// tryDialOnce 进行一轮引导拨号，返回是否至少连接成功一个节点，以及本轮成功数量
func (s *Service) tryDialOnce(ctx context.Context, peers []string) (bool, int) {
	var connected int
	roundStart := time.Now()
	if s.logger != nil {
		s.logger.Debugf("p2p.discovery.dial_round begin peers=%d", len(peers))
	}

	// 记录尝试
	s.mu.RLock()
	recordAttempt := s.recordBootstrapAttempt
	s.mu.RUnlock()
	if recordAttempt != nil {
		recordAttempt()
	}
	// 始终通过 EventBus 发布引导尝试事件，便于统一观测
	if s.eventBus != nil {
		s.eventBus.Publish("p2p.discovery.bootstrap.attempt", nil)
	}

	for _, peerAddr := range peers {
		if s.logger != nil {
			s.logger.Debugf("p2p.discovery.dial_peer start addr=%s", peerAddr)
		}
		m, err := ma.NewMultiaddr(peerAddr)
		if err != nil {
			if s.logger != nil {
				s.logger.Errorf("无效的multiaddr: %s, error: %v", peerAddr, err)
			}
			continue
		}
		info, err := libpeer.AddrInfoFromP2pAddr(m)
		if err != nil {
			if s.logger != nil {
				s.logger.Errorf("无法解析peer地址: %s, error: %v", peerAddr, err)
			}
			continue
		}
		cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		perStart := time.Now()
		err = s.host.Connect(cctx, *info)
		if err == nil {
			connected++
			if s.logger != nil {
				s.logger.Infof("成功连接到peer: %s (%s) duration=%s", info.ID, peerAddr, time.Since(perStart))
			}

			// 发布事件
			if s.eventBus != nil {
				s.eventBus.Publish("p2p.peer.connected", map[string]interface{}{
					"peer_id": info.ID.String(),
					"source":  "bootstrap",
				})
			}
		} else {
			if s.logger != nil {
				// 将引导节点连接失败降级为 Debug 级别日志，避免在公网环境下产生大量 Error 噪音
				s.logger.Debugf("连接peer失败: %s (%s), error: %v duration=%s", info.ID, peerAddr, err, time.Since(perStart))
			}
		}
		cancel()
	}
	if s.logger != nil {
		s.logger.Debugf("p2p.discovery.dial_round end success=%d duration=%s", connected, time.Since(roundStart))
	}

	// 记录成功
	if connected > 0 {
		s.mu.RLock()
		recordSuccess := s.recordBootstrapSuccess
		updateTS := s.updateLastBootstrapTS
		s.mu.RUnlock()
		if recordSuccess != nil {
			recordSuccess()
		}
		if updateTS != nil {
			updateTS()
		}
		// 无论是否设置 Prometheus 回调，统一通过 EventBus 发布一次成功事件
		if s.eventBus != nil {
			s.eventBus.Publish("p2p.discovery.bootstrap.success", map[string]interface{}{
				"connected": connected,
			})
		}
	}

	return connected > 0, connected
}

// mdnsNotifee 实现 mdns.Notifee 接口
type mdnsNotifee struct {
	host                     lphost.Host
	logger                   logiface.Logger
	eventBus                 event.EventBus
	recordMDNSPeerFound      func()
	recordMDNSConnectSuccess func()
	recordMDNSConnectFail    func()
	updateLastMDNSTS         func()
}

func (n *mdnsNotifee) HandlePeerFound(info libpeer.AddrInfo) {
	if n.host == nil {
		return
	}

	if n.logger != nil {
		n.logger.Debugf("p2p.discovery.mdns peer found id=%s addrs=%d", info.ID.String(), len(info.Addrs))
	}

	// 记录 mDNS peer found
	if n.recordMDNSPeerFound != nil {
		n.recordMDNSPeerFound()
	}
	if n.updateLastMDNSTS != nil {
		n.updateLastMDNSTS()
	}

	// 忽略自己
	if info.ID == n.host.ID() {
		return
	}

	// 如果已连接，跳过
	if n.host.Network().Connectedness(info.ID) == libnetwork.Connected {
		return
	}

	// === mDNS 逐地址拨号（TCP 优先）===
	//
	// 背景：
	// - mDNS 发现通常发生在 LAN，但 libp2p 对 AddrInfo 的拨号会并发/择优，错误经常被聚合，最终只看到 “dial backoff/…skipping N errors”，
	//   导致“发现了却连不上”的根因无法定位。
	// - 因此这里按地址逐个尝试，并优先 TCP，再 QUIC，输出每个 addr 的原始错误。
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	addrs := info.Addrs
	if len(addrs) == 0 {
		// 没地址直接视为失败（mDNS 理论上不应出现）
		if n.logger != nil {
			n.logger.Debugf("p2p.discovery.mdns connect failed id=%s: no_addrs", info.ID)
		}
		if n.recordMDNSConnectFail != nil {
			n.recordMDNSConnectFail()
		}
		return
	}

	// 私网优先（LAN 场景）
	var privateAddrs []ma.Multiaddr
	for _, a := range addrs {
		if ip, e := manet.ToIP(a); e == nil && ip != nil && ip.IsPrivate() {
			privateAddrs = append(privateAddrs, a)
		}
	}
	if len(privateAddrs) > 0 {
		addrs = privateAddrs
	}

	var tcpAddrs, quicAddrs, otherAddrs []ma.Multiaddr
	for _, a := range addrs {
		if _, e := a.ValueForProtocol(ma.P_TCP); e == nil {
			tcpAddrs = append(tcpAddrs, a)
			continue
		}
		if _, e := a.ValueForProtocol(ma.P_QUIC_V1); e == nil {
			quicAddrs = append(quicAddrs, a)
			continue
		}
		otherAddrs = append(otherAddrs, a)
	}
	ordered := append(append(append([]ma.Multiaddr{}, tcpAddrs...), quicAddrs...), otherAddrs...)

	var lastErr error
	for _, a := range ordered {
		// 每个地址给一个小超时，避免单个坏地址把 mDNS 连接窗口拖死
		perCtx, perCancel := context.WithTimeout(ctx, 4*time.Second)
		err := n.host.Connect(perCtx, libpeer.AddrInfo{ID: info.ID, Addrs: []ma.Multiaddr{a}})
		perCancel()
		if err == nil {
			lastErr = nil
			break
		}
		lastErr = err
		if n.logger != nil {
			n.logger.Debugf("p2p.discovery.mdns connect failed id=%s addr=%s err=%v", info.ID, a.String(), err)
		}
	}

	if lastErr != nil {
		// 记录失败
		if n.recordMDNSConnectFail != nil {
			n.recordMDNSConnectFail()
		}
		return
	}

	{
		if n.logger != nil {
			n.logger.Infof("p2p.discovery.mdns connected to %s", info.ID)
		}

		// 记录成功
		if n.recordMDNSConnectSuccess != nil {
			n.recordMDNSConnectSuccess()
		}

		// 发布事件
		if n.eventBus != nil {
			n.eventBus.Publish("p2p.peer.connected", map[string]interface{}{
				"peer_id": info.ID.String(),
				"source":  "mdns",
			})
		}
	}
}

// schedulerLoop 引导节点调度器循环（带退避和动态间隔）
func (s *Service) schedulerLoop(ctx context.Context, peers []string) {
	if len(peers) == 0 || s.host == nil {
		return
	}
	if s.logger != nil {
		s.logger.Infof("p2p.discovery.scheduler start peers=%d connected=%d", len(peers), len(s.host.Network().Peers()))
	}

	// 初始快速退避尝试 - 优化退避策略，增加成功率
	b := NewBackoff(2*time.Second, 60*time.Second, 1.5, 0.1)
	for i := 0; i < 5; i++ {
		success, roundConn := s.tryDialOnce(ctx, peers)
		if s.logger != nil {
			s.logger.Infof("p2p.discovery.bootstrap_fast attempt=%d success=%t connected_round=%d", i+1, success, roundConn)
		}
		if success {
			break // 已连上引导，跳出快速尝试进入周期检测维持
		}
		d := b.Next()
		if s.logger != nil {
			s.logger.Infof("p2p.discovery.backoff sleep=%s", d)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(d):
		}
	}

	// 🔧 Phase 3: 动态周期改造 - 使用新上限DiscoveryMaxIntervalCap（默认2m，不再15m）
	baseInterval := s.opts.DiscoveryInterval
	if baseInterval == 0 {
		baseInterval = 5 * time.Minute
	}

	// 使用新配置的上限（默认2m）代替AdvertiseInterval（15m）
	maxInterval := s.opts.DiscoveryMaxIntervalCap
	if maxInterval == 0 {
		maxInterval = 2 * time.Minute
	}

	dynamic := baseInterval
	stableTarget := s.opts.MinPeers
	if stableTarget <= 0 {
		stableTarget = 8
	}
	stableCount := 0
	stableThreshold := 3

	// 重置冷却时间
	resetCoolDown := s.opts.DiscoveryResetCoolDown
	if resetCoolDown == 0 {
		resetCoolDown = 10 * time.Second
	}

	if s.logger != nil {
		// 配置快照保留 Info，便于排障
		s.logger.Infof("p2p.discovery.scheduler_config base_interval=%s max_interval=%s stable_target=%d threshold=%d reset_cooldown=%s",
			baseInterval, maxInterval, stableTarget, stableThreshold, resetCoolDown)
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		// 尝试一次拨号
		success, roundConn := s.tryDialOnce(ctx, peers)
		connected := len(s.host.Network().Peers())
		if s.logger != nil {
			// 周期性调度为高频事件，降级为 Debug，避免在公网环境刷屏
			s.logger.Debugf("p2p.discovery.cycle interval=%s connected=%d success=%t connected_round=%d stableCount=%d target=%d", dynamic, connected, success, roundConn, stableCount, stableTarget)
		}
		if success {
			// 网络稳定延后：使用最大间隔等待一段时间，避免刚连上又立即打扰
			d := jitter(maxInterval, 0.1)
			if s.logger != nil {
				// 稳定延迟属于内部自调度细节，使用 Debug 级别
				s.logger.Debugf("p2p.discovery.stable_delay sleep=%s", d)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(d):
			}
			continue
		}
		// 根据当前连接数自适应调整间隔
		if connected >= stableTarget {
			stableCount++
			if stableCount >= stableThreshold {
				old := dynamic
				dynamic = dynamic * 2
				if dynamic > maxInterval {
					dynamic = maxInterval
				}
				if s.logger != nil {
					// 间隔调整事件保留 Info，便于观察自适应行为
					s.logger.Infof("p2p.discovery.interval_update from=%s to=%s reason=stable", old, dynamic)
				}
			}
		} else {
			// 不稳定则恢复为基础间隔
			if dynamic != baseInterval {
				old := dynamic
				dynamic = baseInterval
				if s.logger != nil {
					// 间隔调整事件保留 Info
					s.logger.Infof("p2p.discovery.interval_update from=%s to=%s reason=unstable", old, dynamic)
				}
			}
			stableCount = 0
		}
		// 🔧 Phase 3: 等待下个周期，支持重置事件
		d := jitter(dynamic, 0.1)
		if s.logger != nil {
			// 周期 sleep 为高频事件，降级为 Debug
			s.logger.Debugf("p2p.discovery.sleep sleep=%s", d)
		}
		select {
		case <-ctx.Done():
			return
		case <-s.schedulerResetChan:
			// 收到重置事件：检查冷却期，通过则重置间隔并立即触发一轮拨号
			s.resetMu.Lock()
			now := time.Now()
			if now.Sub(s.lastResetAt) < resetCoolDown {
				// 冷却期内，忽略重置
				if s.logger != nil {
					s.logger.Debugf("p2p.discovery.scheduler_reset ignored reason=cooldown elapsed=%s", now.Sub(s.lastResetAt))
				}
				s.resetMu.Unlock()
				continue
			}
			s.lastResetAt = now
			s.resetMu.Unlock()

			// 重置间隔到基础值
			old := dynamic
			dynamic = baseInterval
			stableCount = 0
			if s.logger != nil {
				s.logger.Infof("p2p.discovery.scheduler_reset from=%s to=%s", old, dynamic)
			}

			// 立即触发一轮拨号（不等待）
			continue
		case <-time.After(d):
		}
	}
}

// ===================== DHT Rendezvous 发现状态机 =====================

type dhtDiscoveryMode string

const (
	dhtModeBootstrap dhtDiscoveryMode = "bootstrap"
	dhtModeSteady    dhtDiscoveryMode = "steady"
	dhtModeIsolated  dhtDiscoveryMode = "isolated"

	// 在 Bootstrap 阶段期望的最小 DHT 路由表规模/连接数的默认值
	dhtBootstrapMinPeers = 3
	// Bootstrap 阶段最长持续时间，超出后进入 Isolated 模式
	dhtBootstrapMaxDuration = 5 * time.Minute
	// Bootstrap 阶段的基础轮询间隔
	dhtBootstrapInterval = 5 * time.Second

	// Steady 阶段的默认轮询间隔（若未从配置中获取到更合适的值）
	dhtSteadyIntervalDefault = 60 * time.Second

	// Isolated 阶段的退避参数（指数退避）
	dhtIsolatedInitialInterval = 5 * time.Second
	dhtIsolatedMaxInterval     = 10 * time.Minute

	// 每轮 DHT 发现的超时时间（不同模式可区分，但保持保守上限）
	dhtBootstrapRoundTimeout = 60 * time.Second
	dhtSteadyRoundTimeout    = 60 * time.Second
	dhtIsolatedRoundTimeout  = 30 * time.Second
)

// dhtDiscoveryState 记录某个 rendezvous namespace 下的 DHT 发现状态
type dhtDiscoveryState struct {
	mode            dhtDiscoveryMode
	lastSuccessTime time.Time
	successCount    int
	failureCount    int
	currentInterval time.Duration // 下一轮 sleep 的基础间隔
	bootstrapStart  time.Time     // 进入 bootstrap 模式的时间
}

// getDHTExpectedMinPeers 返回当前环境下 DHT 期望的最小 peers 数量
// 优先使用配置（Options.DiscoveryExpectedMinPeers），否则退回默认值。
func (s *Service) getDHTExpectedMinPeers() int {
	if s != nil && s.opts != nil && s.opts.DiscoveryExpectedMinPeers > 0 {
		return s.opts.DiscoveryExpectedMinPeers
	}
	return dhtBootstrapMinPeers
}

// findPeersLoop 通过 DHT rendezvous 持续发现对端并尝试连接
func (s *Service) findPeersLoop(ctx context.Context, ns string) {
	if s.host == nil {
		if s.logger != nil {
			s.logger.Warnf("p2p.discovery.dht_loop host=nil")
		}
		return
	}
	if s.logger != nil {
		s.logger.Infof("p2p.discovery.dht_loop starting ns=%s host_id=%s", ns, s.host.ID().String())
	}

	// 初始化当前 namespace 的 DHT 状态机
	state := &dhtDiscoveryState{
		mode:            dhtModeBootstrap,
		currentInterval: dhtBootstrapInterval,
		bootstrapStart:  time.Now(),
	}

	// 主循环：持续重启DHT发现
	for {
		select {
		case <-ctx.Done():
			if s.logger != nil {
				s.logger.Infof("p2p.discovery.dht_loop context_cancelled_main ns=%s", ns)
			}
			return
		default:
		}

		// 为本轮 DHT 发现创建短生命周期的 ctx，防止内部 goroutine 长期挂住
		var roundTimeout time.Duration
		switch state.mode {
		case dhtModeIsolated:
			roundTimeout = dhtIsolatedRoundTimeout
		case dhtModeSteady:
			roundTimeout = dhtSteadyRoundTimeout
		default:
			roundTimeout = dhtBootstrapRoundTimeout
		}

		roundCtx, cancel := context.WithTimeout(ctx, roundTimeout)
		// 启动一轮DHT发现
		shouldRestart, discovered, rtSize := s.runDHTDiscoveryRound(roundCtx, ns)
		cancel() // 显式结束本轮，释放 libp2p 内部资源

		now := time.Now()

		// 更新状态机统计
		if discovered {
			state.successCount++
			state.lastSuccessTime = now
			state.failureCount = 0
		} else {
			state.failureCount++
		}

		// 估算“是否足够健康”：基于 DHT 路由表大小与当前连接数
		minPeers := s.getDHTExpectedMinPeers()
		enoughPeers := rtSize >= minPeers
		if !enoughPeers && s.host != nil {
			if len(s.host.Network().Peers()) >= minPeers {
				enoughPeers = true
			}
		}

		// 模式迁移
		switch state.mode {
		case dhtModeBootstrap:
			if discovered && enoughPeers {
				// 🔧 Phase 3: 切换到稳定阶段，使用新配置DHTSteadyIntervalCap（默认2m）
				state.mode = dhtModeSteady
				// 使用新的上限配置，不再使用AdvertiseInterval（15m）
				steadyInterval := dhtSteadyIntervalDefault
				if s.opts != nil && s.opts.DHTSteadyIntervalCap > 0 {
					steadyInterval = s.opts.DHTSteadyIntervalCap
				}
				state.currentInterval = steadyInterval
				state.bootstrapStart = time.Time{}
				if s.logger != nil {
					s.logger.Infof("p2p.discovery.dht_loop mode_transition ns=%s from=%s to=%s reason=enough_peers rt_size=%d interval=%s",
						ns, dhtModeBootstrap, dhtModeSteady, rtSize, steadyInterval)
				}
			} else {
				// Bootstrap 长时间无任何成功发现，视为孤立环境
				if state.bootstrapStart.IsZero() {
					state.bootstrapStart = now
				}
				if state.successCount == 0 && now.Sub(state.bootstrapStart) >= dhtBootstrapMaxDuration {
					state.mode = dhtModeIsolated
					state.currentInterval = dhtIsolatedInitialInterval
					if s.logger != nil {
						s.logger.Warnf("p2p.discovery.dht_loop mode_transition ns=%s from=%s to=%s reason=bootstrap_timeout",
							ns, dhtModeBootstrap, dhtModeIsolated)
					}
				}
			}
		case dhtModeSteady:
			// 稳定阶段如果路由表完全清空，回退到 Bootstrap 重新积极发现
			if rtSize == 0 {
				state.mode = dhtModeBootstrap
				state.successCount = 0
				state.failureCount = 0
				state.currentInterval = dhtBootstrapInterval
				state.bootstrapStart = now
				if s.logger != nil {
					s.logger.Warnf("p2p.discovery.dht_loop mode_transition ns=%s from=%s to=%s reason=rt_empty",
						ns, dhtModeSteady, dhtModeBootstrap)
				}
			}
		case dhtModeIsolated:
			if discovered && enoughPeers {
				// 🔧 Phase 3: 从孤立恢复，直接进入稳定阶段，使用新配置
				state.mode = dhtModeSteady
				steadyInterval := dhtSteadyIntervalDefault
				if s.opts != nil && s.opts.DHTSteadyIntervalCap > 0 {
					steadyInterval = s.opts.DHTSteadyIntervalCap
				}
				state.currentInterval = steadyInterval
				state.successCount = 1
				state.failureCount = 0
				state.bootstrapStart = time.Time{}
				if s.logger != nil {
					s.logger.Infof("p2p.discovery.dht_loop mode_transition ns=%s from=%s to=%s reason=recovered",
						ns, dhtModeIsolated, dhtModeSteady)
				}
			} else {
				// 在孤立模式下使用指数退避，逐步拉长轮询间隔，避免空跑
				if state.currentInterval <= 0 {
					state.currentInterval = dhtIsolatedInitialInterval
				} else {
					next := state.currentInterval * 2
					if next > dhtIsolatedMaxInterval {
						next = dhtIsolatedMaxInterval
					}
					state.currentInterval = next
				}
			}
		}
		if !shouldRestart {
			// 如果不需要重启（例如context取消或 rendezvous 不可用），则退出主循环
			return
		}

		// 根据当前模式选择下一轮的等待间隔并加入轻微抖动
		sleepBase := state.currentInterval
		if sleepBase <= 0 {
			// 各模式的兜底间隔
			switch state.mode {
			case dhtModeIsolated:
				sleepBase = dhtIsolatedInitialInterval
			case dhtModeSteady:
				sleepBase = dhtSteadyIntervalDefault
			default:
				sleepBase = dhtBootstrapInterval
			}
			state.currentInterval = sleepBase
		}
		d := jitter(sleepBase, 0.1)
		if s.logger != nil {
			s.logger.Debugf("p2p.discovery.dht_loop sleep_before_next_round ns=%s mode=%s base=%s sleep=%s",
				ns, state.mode, sleepBase, d)
		}

		// 🔧 Phase 3: 支持重置事件
		resetCoolDown := s.opts.DiscoveryResetCoolDown
		if resetCoolDown == 0 {
			resetCoolDown = 10 * time.Second
		}

		select {
		case <-ctx.Done():
			if s.logger != nil {
				s.logger.Infof("p2p.discovery.dht_loop context_cancelled_during_wait ns=%s", ns)
			}
			return
		case <-s.dhtResetChan:
			// 收到重置事件：检查冷却期，通过则立即触发下一轮
			s.resetMu.Lock()
			now := time.Now()
			if now.Sub(s.lastResetAt) < resetCoolDown {
				// 冷却期内，忽略重置
				if s.logger != nil {
					s.logger.Debugf("p2p.discovery.dht_reset ignored reason=cooldown elapsed=%s", now.Sub(s.lastResetAt))
				}
				s.resetMu.Unlock()
				continue
			}
			s.lastResetAt = now
			s.resetMu.Unlock()

			if s.logger != nil {
				s.logger.Infof("p2p.discovery.dht_reset triggered ns=%s mode=%s", ns, state.mode)
			}

			// 立即触发下一轮（不等待）
			continue
		case <-time.After(d):
			// 继续下一轮循环
		}
	}
}

// runDHTDiscoveryRound 运行一轮DHT发现
// 返回值：
//   - bool: 是否需要在通道关闭后重启下一轮
//   - bool: 本轮是否至少发现过一个“有效”peer（非自身且带地址）
//   - int:  本轮结束时的 DHT 路由表规模快照
func (s *Service) runDHTDiscoveryRound(ctx context.Context, ns string) (bool, bool, int) {
	discovered := false
	rtSize := 0
	if s.logger != nil {
		s.logger.Infof("🔄 DHT重启循环开始 ns=%s", ns)
		s.logger.Infof("p2p.discovery.dht_loop calling_FindPeers ns=%s", ns)
	}

	s.mu.RLock()
	rendezvous := s.rendezvousRouting
	s.mu.RUnlock()

	if rendezvous == nil {
		if s.logger != nil {
			s.logger.Warnf("p2p.discovery.dht_loop rendezvous_not_available ns=%s", ns)
		}
		return false, false, 0
	}

	pch, err := rendezvous.AdvertiseAndFindPeers(ctx, ns)
	if err != nil {
		if s.logger != nil {
			s.logger.Warnf("p2p.discovery.rendezvous find_peers_error ns=%s err=%v", ns, err)
		}
		return false, false, rendezvous.RoutingTableSize() // 出错时不重启
	}

	if s.logger != nil {
		s.logger.Infof("p2p.discovery.dht_loop peer_channel_ready ns=%s, waiting_for_peers", ns)
		// 检查 DHT 状态（通过接口获取路由表大小）
		rtSize = rendezvous.RoutingTableSize()
		if rtSize > 0 {
			s.logger.Infof("p2p.discovery.dht_loop dht_rt_size=%d connected_peers=%d",
				rtSize, len(s.host.Network().Peers()))
		}
	}

	for {
		select {
		case <-ctx.Done():
			if s.logger != nil {
				s.logger.Infof("p2p.discovery.dht_loop round_done ns=%s reason=context_done err=%v", ns, ctx.Err())
			}
			// 这里的 ctx 是“本轮 roundCtx”，超时/取消应当视为“结束本轮并进入下一轮”，
			// 否则会导致 DHT 发现循环只运行一次：A 先启动、B 后启动时，A 很可能永远发现不到 B。
			//
			// 真正的退出由 findPeersLoop 外层 ctx.Done() 控制。
			rtSize = rendezvous.RoutingTableSize()
			return true, discovered, rtSize
		case info, ok := <-pch:
			if !ok {
				if s.logger != nil {
					// DHT/Rendezvous 在“本轮无 peer 可返回”时关闭 channel 属于常见行为，不应按异常 Warn 刷屏。
					// 仍返回 should_restart=true 以进入下一轮发现。
					s.logger.Debugf("p2p.discovery.dht_loop peer_channel_closed ns=%s, should_restart=true", ns)
					// 检查 DHT 状态（通过接口获取路由表大小）
					rtSize = rendezvous.RoutingTableSize()
					if rtSize > 0 {
						s.logger.Infof("p2p.discovery.dht_loop final_dht_rt_size=%d connected_peers=%d",
							rtSize, len(s.host.Network().Peers()))
					}
				}
				return true, discovered, rtSize // 通道关闭时需要重启
			}

			// 处理发现的peer
			if s.handleDiscoveredPeer(ctx, info, ns) {
				discovered = true
			}
		}
	}
}

// handleDiscoveredPeer 处理发现的peer
// 返回值：
//   - bool: 是否为一个“有效”peer（非自身且带地址），用于上层统计发现成功次数
func (s *Service) handleDiscoveredPeer(ctx context.Context, info libpeer.AddrInfo, ns string) bool {
	if s.logger != nil {
		// DHT 发现 peer 在主网环境下会非常频繁：
		// - 保留一条精简的 Info 日志，便于确认发现行为；
		// - 详细信息（addrs/self_id 对比）降级为 Debug，避免刷屏。
		s.logger.Infof("p2p.discovery.dht_loop peer_discovered id=%s addrs=%d ns=%s",
			info.ID.String(), len(info.Addrs), ns)
		s.logger.Debugf("p2p.discovery.dht_loop peer_check discovered_id=%s self_id=%s", info.ID.String(), s.host.ID().String())
	}

	if info.ID == "" || info.ID == s.host.ID() {
		if s.logger != nil {
			reason := func() string {
				if info.ID == "" {
					return "empty_id"
				}
				return "self_id"
			}()
			// 自身/空ID跳过为预期行为，使用 Debug 级别
			s.logger.Debugf("⏩ 跳过peer (原因: %s): %s", reason, info.ID.String())
		}
		return false
	}

	// 如果 DHT 返回的节点没有任何地址（仅有 ID），尝试通过地址管理器获取
	if len(info.Addrs) == 0 {
		if s.addrManager != nil {
			// 尝试从地址管理器获取地址（会触发异步查询+重发现队列）
			addrs := s.addrManager.GetAddrs(info.ID)
			if len(addrs) == 0 {
				// 🆕 优化：如果该peer最近有连接记录，标记为高优先级重发现
				if s.wasRecentlyConnected(info.ID) {
					s.addrManager.TriggerRediscovery(info.ID, true) // high priority
					if s.logger != nil {
						s.logger.Infof("p2p.discovery.dht_loop peer_no_addrs id=%s ns=%s, high_priority_rediscovery_triggered",
							info.ID.String(), ns)
					}
				} else {
					if s.logger != nil {
						s.logger.Warnf("p2p.discovery.dht_loop peer_no_addrs id=%s ns=%s, rediscovery_triggered",
							info.ID.String(), ns)
					}
				}
				return false
			}
			// 使用地址管理器返回的地址
			info.Addrs = addrs
		} else {
			// 不应该发生：AddrManager 现在是强制启用的基础设施
			if s.logger != nil {
				s.logger.Errorf("p2p.discovery.dht_loop addr_manager_nil (unexpected) peer=%s ns=%s",
					info.ID.String(), ns)
			}
			return false
		}
	}

	// 走到这里说明是一个“有效”peer
	validPeer := true

	if s.logger != nil {
		// 每次连接尝试为高频事件，使用 Debug 级别，避免 Info 噪音
		s.logger.Debugf("p2p.discovery.dht_loop connecting_to_peer id=%s addrs=%v", info.ID.String(), info.Addrs)
	}

	// === LAN 优先拨号策略（关键修复）===
	// 目标：即使没有 mDNS，只要接入同一 DHT/同一批 bootstrap，也应尽量“间接发现并直连”局域网节点。
	// 现实问题：很多网络不支持 NAT hairpin；若对方只公告公网地址，即使在同一 LAN 内也可能拨不通。
	//
	// 策略：
	// - 若我们自身“看起来处于 LAN”（启用 mDNS 或本机 Host 地址包含私网 IP），并且对方 AddrInfo 中包含私网地址，
	//   则先仅用私网地址尝试一次 Connect；失败后再回退到全量地址（含公网/relay）。
	isLANMode := s.opts != nil && s.opts.EnableMDNS
	if !isLANMode && s.host != nil {
		for _, a := range s.host.Addrs() {
			if ip, e := manet.ToIP(a); e == nil && ip != nil && ip.IsPrivate() {
				isLANMode = true
				break
			}
		}
	}

	var privateAddrs []ma.Multiaddr
	for _, a := range info.Addrs {
		if ip, e := manet.ToIP(a); e == nil && ip != nil && ip.IsPrivate() {
			privateAddrs = append(privateAddrs, a)
		}
	}

	cctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// 使用地址管理器添加DHT发现的地址（分级TTL管理）
	// AddrManager 现在是强制启用的基础设施，应该始终可用
	if s.addrManager != nil {
		s.addrManager.AddDHTAddr(info.ID, info.Addrs)
	}

	var err error
	if isLANMode && len(privateAddrs) > 0 {
		// 关键改进：
		// 1) 私网场景下，libp2p 可能会在 AddrInfo 内部做地址选择/并发拨号；一旦命中坏地址（尤其是 relay/QUIC）
		//    错误会被聚合成 “...skipping N errors”，导致我们难以看到真正的失败原因。
		// 2) 实际上 LAN 内最稳的是 TCP，其次才是 QUIC；因此这里按传输做优先级并逐个地址尝试拨号，
		//    每个 addr 的真实错误都会被记录下来（debug 级别）。
		var tcpAddrs, quicAddrs, otherAddrs []ma.Multiaddr
		for _, a := range privateAddrs {
			if _, e := a.ValueForProtocol(ma.P_TCP); e == nil {
				tcpAddrs = append(tcpAddrs, a)
				continue
			}
			if _, e := a.ValueForProtocol(ma.P_QUIC_V1); e == nil {
				quicAddrs = append(quicAddrs, a)
				continue
			}
			otherAddrs = append(otherAddrs, a)
		}
		ordered := append(append(append([]ma.Multiaddr{}, tcpAddrs...), quicAddrs...), otherAddrs...)

		if s.logger != nil {
			s.logger.Debugf(
				"p2p.discovery.dht_loop dialing_private_first id=%s private_addrs=%d tcp=%d quic=%d other=%d",
				info.ID.String(), len(privateAddrs), len(tcpAddrs), len(quicAddrs), len(otherAddrs),
			)
		}

		// 写入私网地址（确保 peerstore 有可拨号的 LAN 地址）
		// AddrManager 现在是强制启用的基础设施，应该始终可用
		if s.addrManager != nil {
			s.addrManager.AddDHTAddr(info.ID, privateAddrs)
		}

		for _, a := range ordered {
			tmp := libpeer.AddrInfo{ID: info.ID, Addrs: []ma.Multiaddr{a}}
			perCtx, perCancel := context.WithTimeout(cctx, 10*time.Second)
			perErr := s.host.Connect(perCtx, tmp)
			perCancel()
			if perErr == nil {
				err = nil
				break
			}
			// ✅ 自愈：如果该地址对应的 remote peerID 与预期不一致，立即纠错 addr->peer 映射，避免后续持续连错人。
			_ = s.healPeerIDMismatch(info.ID, a, perErr)
			// 这里保留每个地址的原始错误，便于直接定位“是被 gater 拦了 / 无 transport / 握手失败 / 连接被复位”等。
			if s.logger != nil {
				s.logger.Debugf("p2p.discovery.dht_loop private_dial_failed id=%s addr=%s err=%v", info.ID.String(), a.String(), perErr)
			}
			err = perErr
		}
	}
	if err != nil {
		// 回退：全量地址（可能包含公网/relay）
		err = s.host.Connect(cctx, info)
	}
	if err == nil {
		if s.logger != nil {
			// 成功连接保留 Info，便于观测网络连通性
			s.logger.Infof("p2p.discovery.dht_loop connect_success id=%s", info.ID.String())
		}

		// 发布事件
		if s.eventBus != nil {
			s.eventBus.Publish("p2p.peer.connected", map[string]interface{}{
				"peer_id": info.ID.String(),
				"source":  "dht",
			})
		}
	} else {
		// ✅ 自愈（兜底）：fallback Connect() 返回的聚合错误中可能包含 peer id mismatch 的 addr 列表，尝试批量纠错。
		s.healPeerIDMismatchFromAggregateError(info.ID, err)
		if s.logger != nil {
			// DHT 发现阶段在公网环境下连接失败很常见（噪声大），但在 LAN/私网互联场景下，
			// "发现到了却连不上"是必须被看见的关键故障信号。
			//
			// 优化后的判定策略：
			// 1. 检查是否是dial backoff（预期的失败，不应该警告）
			// 2. 检查是否是跨网段私网地址（不可达，不应该警告）
			// 3. 只有在mDNS模式下且是同一LAN内的连接失败才警告
			errMsg := err.Error()
			isDialBackoff := strings.Contains(errMsg, "dial backoff") || strings.Contains(errMsg, "backoff")
			
			// 检查是否有同网段的私网地址失败
			isLANMode := s.opts != nil && s.opts.EnableMDNS
			hasSameLANAddr := false
			if isLANMode && s.host != nil {
				// 获取本机私网IP段
				hostPrivateNets := make(map[string]bool)
				for _, a := range s.host.Addrs() {
					if ip, e := manet.ToIP(a); e == nil && ip != nil && ip.IsPrivate() {
						// 提取网段（如192.168.0.x -> 192.168.0）
						ipStr := ip.String()
						if idx := strings.LastIndex(ipStr, "."); idx > 0 {
							hostPrivateNets[ipStr[:idx]] = true
						}
					}
				}
				// 检查对方地址是否在同一网段
				for _, a := range info.Addrs {
					if ip, e := manet.ToIP(a); e == nil && ip != nil && ip.IsPrivate() {
						ipStr := ip.String()
						if idx := strings.LastIndex(ipStr, "."); idx > 0 {
							if hostPrivateNets[ipStr[:idx]] {
								hasSameLANAddr = true
								break
							}
						}
					}
				}
			}
			
			// 只在以下情况警告：mDNS模式 && 同网段 && 非backoff
			if isLANMode && hasSameLANAddr && !isDialBackoff {
				s.logger.Warnf("p2p.discovery.dht_loop connect_failed id=%s addrs=%v error=%v", info.ID.String(), info.Addrs, err)
			} else {
				// 其他情况降级为Debug，避免刷屏
				s.logger.Debugf("p2p.discovery.dht_loop connect_failed id=%s error=%v", info.ID.String(), err)
			}
		}
	}

	return validPeer
}
