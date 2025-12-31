package discovery

import (
	"context"
	"strings"
	"sync"
	"time"

	libevent "github.com/libp2p/go-libp2p/core/event"
	lphost "github.com/libp2p/go-libp2p/core/host"
	libnetwork "github.com/libp2p/go-libp2p/core/network"
	libpeer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"sort"

	"github.com/weisyn/v1/internal/core/p2p/interfaces"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	metricsiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/metrics"
)

// AddrTTL 地址生命周期配置
type AddrTTL struct {
	DHT       time.Duration // DHT发现的地址
	Connected time.Duration // 连接成功的地址
	Bootstrap time.Duration // Bootstrap节点地址
	Failed    time.Duration // 连接失败的地址
}

// DefaultAddrTTL 默认地址TTL配置
var DefaultAddrTTL = AddrTTL{
	// 🆕 P0-009: DHT 地址 TTL 过短会导致地址在短时间内过期，DHT 再次发现时出现 addrs=0。
	// 统一默认值为 2h（与 discovery/service.go 的内置配置保持一致）。
	DHT:       2 * time.Hour,
	Connected: 24 * time.Hour,
	Bootstrap: peerstore.PermanentAddrTTL,
	Failed:    5 * time.Minute,
}

// PeerRediscoveryInfo peer重发现信息
type PeerRediscoveryInfo struct {
	PeerID        libpeer.ID
	LastAttemptAt time.Time
	FailCount     int
	Priority      int // 0=normal, 1=high (from recent connections)
}

// AddrManager 地址管理器
//
// 负责主动管理peer地址的生命周期，解决libp2p Peerstore地址24小时自动过期问题。
// 核心功能：
// - 分级TTL管理：不同来源的地址使用不同的生命周期
// - 主动刷新：定期检查并刷新即将过期的地址
// - 事件驱动：根据连接状态升级/降级地址TTL
// - 故障自愈：地址失效时自动触发重新发现
// - 主动重发现：无地址peer加入队列，周期重试
type AddrManager struct {
	host      lphost.Host
	peerstore peerstore.Peerstore
	routing   interfaces.RendezvousRouting // 用于DHT查询
	ttl       AddrTTL
	logger    logiface.Logger

	// 地址刷新状态
	mu             sync.RWMutex
	lastRefreshAt  map[libpeer.ID]time.Time // 记录每个peer的最后刷新时间
	lastSeenAt     map[libpeer.ID]time.Time // 记录每个peer的最后“看见”时间（用于淘汰/有界化）
	lastConnectedAt map[libpeer.ID]time.Time // 记录每个peer的最近连接时间（用于刷新策略精细化）
	pendingLookups map[libpeer.ID]bool      // 正在查询的peer，防止重复查询
	refreshCursor  int                      // refreshAllPeers 的分片遍历游标（避免每次全量扫描）

	// 🆕 重发现队列
	rediscoveryQueue map[libpeer.ID]*PeerRediscoveryInfo
	rediscoveryMu    sync.RWMutex

	// 配置参数
	maxConcurrentLookups int           // 最大并发查询数
	lookupTimeout        time.Duration // 查询超时时间
	refreshInterval      time.Duration // 刷新周期
	refreshThreshold     time.Duration // 刷新阈值
	maxTrackedPeers      int           // 最大跟踪 peer 数（超限则淘汰）
	refreshBudget        int           // 每次 refresh 周期最多处理的 peer 数（避免全量遍历引发资源风暴）
	maxAddrsPerPeer      int           // 每个 peer 最多保留的地址数量（控制 peerstore 占用）
	maxPendingLookups    int           // pendingLookups 上限（避免 map 无界增长）
	maxRediscoveryQueue  int           // rediscoveryQueue 上限（避免队列无界增长）

	// 🆕 重发现配置
	rediscoveryInterval    time.Duration // 重发现扫描间隔（默认30s）
	rediscoveryMaxRetries  int           // 最大重试次数（默认10）
	rediscoveryBackoffBase time.Duration // 退避基础时间（默认1m）

	// 持久化配置
	enablePersistence bool
	persistenceBackend string // "badger" | "json"
	badgerDir          string
	namespacePrefix    string
	pruneInterval      time.Duration
	recordTTL          time.Duration

	// lookup 并发限流（避免 DHT 风暴）
	lookupSem chan struct{}
	// rediscovery 并发限流（避免 goroutine 风暴）
	rediscoverySem chan struct{}

	ctx    context.Context
	cancel context.CancelFunc

	// 持久化存储（Badger/JSON）
	store AddrStore

	// bootstrap peer 集合（用于淘汰保护）
	bootstrapPeers map[libpeer.ID]struct{}
}

// AddrManagerConfig 地址管理器配置
type AddrManagerConfig struct {
	TTL                  AddrTTL
	MaxConcurrentLookups int
	LookupTimeout        time.Duration
	RefreshInterval      time.Duration
	RefreshThreshold     time.Duration
	// === 有界化（防 OOM）===
	MaxTrackedPeers     int // 最大可跟踪 peer 数（超限淘汰，bootstrap/connected/recent 优先保留）
	RefreshBudget       int // 每次 refresh 周期最多处理的 peer 数
	MaxAddrsPerPeer     int // 每个 peer 最多保留地址数量
	MaxPendingLookups   int // pendingLookups map 上限
	MaxRediscoveryQueue int // rediscoveryQueue 上限
	// 🆕 重发现配置
	RediscoveryInterval    time.Duration // 重发现扫描间隔（默认30s）
	RediscoveryMaxRetries  int           // 最大重试次数（默认10）
	RediscoveryBackoffBase time.Duration // 退避基础时间（默认1m）
	// 持久化
	EnablePersistence  bool
	PersistenceBackend string        // "badger" | "json"
	BadgerDir          string        // 例如 data/p2p/<hostID>/badger
	NamespacePrefix    string        // 例如 peer_addrs/v1/
	PruneInterval      time.Duration // 例如 1h
	RecordTTL          time.Duration // 例如 7d
}

// NewAddrManager 创建地址管理器
func NewAddrManager(h lphost.Host, routing interfaces.RendezvousRouting, cfg AddrManagerConfig, logger logiface.Logger) *AddrManager {
	ctx, cancel := context.WithCancel(context.Background())

	am := &AddrManager{
		host:                   h,
		peerstore:              h.Peerstore(),
		routing:                routing,
		ttl:                    cfg.TTL,
		logger:                 logger,
		lastRefreshAt:          make(map[libpeer.ID]time.Time),
		lastSeenAt:             make(map[libpeer.ID]time.Time),
		lastConnectedAt:        make(map[libpeer.ID]time.Time),
		pendingLookups:         make(map[libpeer.ID]bool),
		rediscoveryQueue:       make(map[libpeer.ID]*PeerRediscoveryInfo),
		maxConcurrentLookups:   cfg.MaxConcurrentLookups,
		lookupTimeout:          cfg.LookupTimeout,
		refreshInterval:        cfg.RefreshInterval,
		refreshThreshold:       cfg.RefreshThreshold,
		maxTrackedPeers:        cfg.MaxTrackedPeers,
		refreshBudget:          cfg.RefreshBudget,
		maxAddrsPerPeer:        cfg.MaxAddrsPerPeer,
		maxPendingLookups:      cfg.MaxPendingLookups,
		maxRediscoveryQueue:    cfg.MaxRediscoveryQueue,
		rediscoveryInterval:    cfg.RediscoveryInterval,
		rediscoveryMaxRetries:  cfg.RediscoveryMaxRetries,
		rediscoveryBackoffBase: cfg.RediscoveryBackoffBase,
		enablePersistence:      cfg.EnablePersistence,
		persistenceBackend:     cfg.PersistenceBackend,
		badgerDir:              cfg.BadgerDir,
		namespacePrefix:        cfg.NamespacePrefix,
		pruneInterval:          cfg.PruneInterval,
		recordTTL:              cfg.RecordTTL,
		ctx:                    ctx,
		cancel:                 cancel,
		bootstrapPeers:         make(map[libpeer.ID]struct{}),
	}
	
	// 设置重发现默认值
	if am.rediscoveryInterval == 0 {
		am.rediscoveryInterval = 30 * time.Second
	}
	if am.rediscoveryMaxRetries == 0 {
		am.rediscoveryMaxRetries = 10
	}
	if am.rediscoveryBackoffBase == 0 {
		am.rediscoveryBackoffBase = 1 * time.Minute
	}

	// 初始化 lookup semaphore
	if am.maxConcurrentLookups <= 0 {
		am.maxConcurrentLookups = 10
	}
	am.lookupSem = make(chan struct{}, am.maxConcurrentLookups)

	// 🆕 初始化 rediscovery semaphore（独立的并发限制，默认5，避免 goroutine 风暴）
	rediscoveryMaxConcurrent := 5
	if cfg.MaxConcurrentLookups > 0 && cfg.MaxConcurrentLookups < 5 {
		rediscoveryMaxConcurrent = cfg.MaxConcurrentLookups
	}
	am.rediscoverySem = make(chan struct{}, rediscoveryMaxConcurrent)

	// 🆕 P1 修复：优化有界化默认值，防止内存泄漏
	// 根据阿里云节点分析报告（20,087 对象 / 41.1MB），将默认值调整为更保守的值
	// 参考：本地稳定节点约 6,000 对象 / 11.8MB
	if am.maxTrackedPeers <= 0 {
		am.maxTrackedPeers = 5000 // 原 20000 → 5000，减少 75%
	}
	if am.refreshBudget <= 0 {
		am.refreshBudget = 500 // 原 1000 → 500，减少每次 refresh 的 peer 数
	}
	if am.maxAddrsPerPeer <= 0 {
		am.maxAddrsPerPeer = 10 // 原 8 → 10，每个 peer 最多 10 个地址
	}
	if am.maxPendingLookups <= 0 {
		am.maxPendingLookups = 5000 // 原 20000 → 5000，减少 75%
	}
	// 🆕 优化：将队列大小从10000降低到50，防止内存泄漏
	if am.maxRediscoveryQueue <= 0 {
		am.maxRediscoveryQueue = 50
	}

	// 初始化持久化 store（专用 BadgerDB / JSON）
	if am.enablePersistence {
		switch strings.TrimSpace(strings.ToLower(am.persistenceBackend)) {
		case "", "badger":
			s, err := newBadgerAddrStore(badgerAddrStoreConfig{
				Dir:             am.badgerDir,
				NamespacePrefix: am.namespacePrefix,
			}, logger)
			if err != nil {
				if logger != nil {
					logger.Errorf("addr_manager badger store init failed: %v", err)
				}
			} else {
				am.store = s
				am.loadPersistedRecords()
			}
		case "json":
			// TODO: 如确需 JSON 后端，可实现 json store 适配器
			if logger != nil {
				logger.Warnf("addr_manager persistence backend=json not implemented, skipping persistence")
			}
		default:
			if logger != nil {
				logger.Warnf("addr_manager unknown persistence backend=%s, skipping persistence", am.persistenceBackend)
			}
		}
	}

	return am
}

// ModuleName 实现 MemoryReporter（用于 MemoryDoctor 采样）
func (am *AddrManager) ModuleName() string {
	return "p2p.addr_manager"
}

// CollectMemoryStats 实现 MemoryReporter（用于 MemoryDoctor 采样）
func (am *AddrManager) CollectMemoryStats() metricsiface.ModuleMemoryStats {
	// 估算：peerstore 规模与地址数量是主要驱动。
	//
	// 注意：
	// - 这里不使用“每 peer 固定 X KB”的拍脑袋常数（会误导分析），而是对 peerstore 做小样本采样，
	//   用真实的 peerID / addr 字符串长度得到每 peer 的平均占用，再按总 peer 数放大。
	// - 估算只用于趋势观察，不追求绝对精确。
	peerCount := 0
	if am != nil && am.peerstore != nil {
		peerCount = len(am.peerstore.Peers())
	}
	pending := 0
	if am != nil {
		am.mu.RLock()
		pending = len(am.pendingLookups)
		am.mu.RUnlock()
	}
	rediscovery := 0
	if am != nil {
		rediscovery = am.GetRediscoveryQueueSize()
	}

	approx := int64(0)
	if am != nil && am.peerstore != nil && peerCount > 0 {
		peers := am.peerstore.Peers()
		// 采样上限：避免 MemoryDoctor 采样时对超大 peerstore 造成明显开销
		sampleN := peerCount
		if sampleN > 50 {
			sampleN = 50
		}
		var totalBytes int64
		for i := 0; i < sampleN; i++ {
			pid := peers[i]
			// peerID 字符串长度（近似表示其在内存中的 payload）
			totalBytes += int64(len(pid))
			// 地址字符串长度（addr payload）
			for _, a := range am.peerstore.Addrs(pid) {
				if a == nil {
					continue
				}
				totalBytes += int64(len(a.String()))
			}
		}
		avgBytesPerPeer := float64(totalBytes) / float64(sampleN)
		approx = int64(avgBytesPerPeer * float64(peerCount))
	}
	return metricsiface.ModuleMemoryStats{
		Module:      "p2p.addr_manager",
		Layer:       "L2-Infrastructure",
		Objects:     int64(peerCount),
		ApproxBytes: approx,
		CacheItems:  int64(pending),
		QueueLength: int64(rediscovery),
	}
}

// capAddrs 限制单个 peer 的地址数量，避免 peerstore 占用无界增长
func (am *AddrManager) capAddrs(addrs []ma.Multiaddr) []ma.Multiaddr {
	if am == nil || am.maxAddrsPerPeer <= 0 || len(addrs) <= am.maxAddrsPerPeer {
		return addrs
	}
	// 去重并按“可拨号优先”简单排序：public/relay 优先，其次 private，最后其他
	type bucket struct {
		pub   []ma.Multiaddr
		priv  []ma.Multiaddr
		other []ma.Multiaddr
	}
	seen := make(map[string]struct{}, len(addrs))
	var b bucket
	for _, a := range addrs {
		if a == nil {
			continue
		}
		s := a.String()
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		isRelay := false
		for _, p := range a.Protocols() {
			if p.Name == "p2p-circuit" {
				isRelay = true
				break
			}
		}
		if isRelay {
			b.pub = append(b.pub, a)
			continue
		}
		if ip, err := manet.ToIP(a); err == nil && ip != nil {
			if ip.IsPrivate() {
				b.priv = append(b.priv, a)
			} else {
				b.pub = append(b.pub, a)
			}
			continue
		}
		b.other = append(b.other, a)
	}
	out := make([]ma.Multiaddr, 0, am.maxAddrsPerPeer)
	appendUpTo := func(src []ma.Multiaddr) {
		for _, a := range src {
			if len(out) >= am.maxAddrsPerPeer {
				return
			}
			out = append(out, a)
		}
	}
	appendUpTo(b.pub)
	appendUpTo(b.priv)
	appendUpTo(b.other)
	return out
}

func (am *AddrManager) markSeenLocked(id libpeer.ID, now time.Time) {
	am.lastSeenAt[id] = now
	am.lastRefreshAt[id] = now
}

// enforceBounds 在 refresh 周期内做轻量有界化：超限淘汰 + 关键 map 清理
func (am *AddrManager) enforceBounds() {
	if am == nil || am.peerstore == nil {
		return
	}
	if am.maxTrackedPeers <= 0 {
		return
	}
	peers := am.peerstore.Peers()
	if len(peers) <= am.maxTrackedPeers {
		return
	}

	type cand struct {
		id        libpeer.ID
		seenAt    time.Time
		connected bool
	}
	now := time.Now()
	cands := make([]cand, 0, len(peers))
	for _, p := range peers {
		if p == "" || p == am.host.ID() {
			continue
		}
		if am.isBootstrapPeer(p) {
			continue
		}
		connected := false
		if am.host != nil && am.host.Network().Connectedness(p) == libnetwork.Connected {
			connected = true
		}
		if connected {
			continue
		}
		am.mu.RLock()
		seenAt := am.lastSeenAt[p]
		lastConn := am.lastConnectedAt[p]
		am.mu.RUnlock()
		// 近期连接过的优先保留
		if !lastConn.IsZero() && now.Sub(lastConn) < am.ttl.Connected {
			continue
		}
		cands = append(cands, cand{id: p, seenAt: seenAt, connected: connected})
	}

	needEvict := len(peers) - am.maxTrackedPeers
	if needEvict <= 0 || len(cands) == 0 {
		return
	}

	// seenAt 越早越先淘汰；没有 seenAt 的认为最老
	sort.Slice(cands, func(i, j int) bool {
		a := cands[i].seenAt
		b := cands[j].seenAt
		if a.IsZero() && !b.IsZero() {
			return true
		}
		if !a.IsZero() && b.IsZero() {
			return false
		}
		return a.Before(b)
	})

	if needEvict > len(cands) {
		needEvict = len(cands)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	evicted := 0
	for i := 0; i < needEvict; i++ {
		id := cands[i].id
		// 清理 peerstore（尽可能释放地址与元数据）
		am.peerstore.ClearAddrs(id)
		am.peerstore.RemovePeer(id)

		am.mu.Lock()
		delete(am.lastRefreshAt, id)
		delete(am.lastSeenAt, id)
		delete(am.lastConnectedAt, id)
		delete(am.pendingLookups, id)
		am.mu.Unlock()

		am.rediscoveryMu.Lock()
		delete(am.rediscoveryQueue, id)
		am.rediscoveryMu.Unlock()

		if am.store != nil {
			_ = am.store.Delete(ctx, id.String())
		}
		evicted++
	}

	if evicted > 0 && am.logger != nil {
		am.logger.Warnf("addr_manager bounded_peerstore evicted=%d current_peers=%d max=%d", evicted, len(peers)-evicted, am.maxTrackedPeers)
	}
}

func (am *AddrManager) isBootstrapPeer(id libpeer.ID) bool {
	if am == nil {
		return false
	}
	am.mu.RLock()
	_, ok := am.bootstrapPeers[id]
	am.mu.RUnlock()
	return ok
}

// Start 启动地址管理器
func (am *AddrManager) Start() {
	if am.logger != nil {
		am.logger.Infof("addr_manager starting")
	}

	// 启动主动刷新goroutine
	go am.refreshLoop()

	// 监听连接事件
	go am.handleConnectionEvents()

	// 启动 prune 循环（all_discovered 场景必须）
	if am.store != nil && am.pruneInterval > 0 && am.recordTTL > 0 {
		go am.pruneLoop()
	}

	// 🆕 启动重发现循环
	go am.rediscoveryLoop()

	if am.logger != nil {
		am.logger.Infof("addr_manager started with rediscovery enabled")
	}
}

// Stop 停止地址管理器
func (am *AddrManager) Stop() {
	if am.logger != nil {
		am.logger.Infof("addr_manager stopping")
	}

	am.cancel()

	// 关闭持久化 store
	if am.store != nil {
		_ = am.store.Close()
		am.store = nil
	}

	if am.logger != nil {
		am.logger.Infof("addr_manager stopped")
	}
}

// AddDHTAddr 添加DHT发现的地址
func (am *AddrManager) AddDHTAddr(id libpeer.ID, addrs []ma.Multiaddr) {
	if len(addrs) == 0 {
		return
	}

	addrs = am.capAddrs(addrs)
	am.peerstore.AddAddrs(id, addrs, am.ttl.DHT)

	now := time.Now()
	am.mu.Lock()
	am.markSeenLocked(id, now)
	am.mu.Unlock()

	if am.logger != nil {
		am.logger.Debugf("addr_manager add_dht_addr peer=%s addrs=%d ttl=%s",
			id.String(), len(addrs), am.ttl.DHT)
	}

	am.upsertPeerRecord(id.String(), func(r *PeerAddrRecord) {
		r.LastSeenAt = now
		r.Addrs = mergeStringAddrs(r.Addrs, addrs)
	})
}

// AddConnectedAddr 添加连接成功的地址（升级TTL）
func (am *AddrManager) AddConnectedAddr(id libpeer.ID, addrs []ma.Multiaddr) {
	if len(addrs) == 0 {
		return
	}

	addrs = am.capAddrs(addrs)
	// 连接成功，升级TTL到24小时
	am.peerstore.AddAddrs(id, addrs, am.ttl.Connected)

	now := time.Now()
	am.mu.Lock()
	am.markSeenLocked(id, now)
	am.lastConnectedAt[id] = now
	am.mu.Unlock()

	if am.logger != nil {
		am.logger.Debugf("addr_manager add_connected_addr peer=%s addrs=%d ttl=%s",
			id.String(), len(addrs), am.ttl.Connected)
	}

	am.upsertPeerRecord(id.String(), func(r *PeerAddrRecord) {
		r.LastSeenAt = now
		r.LastConnectedAt = now
		r.SuccessCount++
		r.Addrs = mergeStringAddrs(r.Addrs, addrs)
	})
}

// AddBootstrapAddr 添加Bootstrap节点地址（永久保存）
func (am *AddrManager) AddBootstrapAddr(id libpeer.ID, addrs []ma.Multiaddr) {
	if len(addrs) == 0 {
		return
	}

	addrs = am.capAddrs(addrs)
	// Bootstrap节点使用永久TTL
	am.peerstore.AddAddrs(id, addrs, am.ttl.Bootstrap)

	if am.logger != nil {
		am.logger.Debugf("addr_manager add_bootstrap_addr peer=%s addrs=%d",
			id.String(), len(addrs))
	}

	now := time.Now()
	am.mu.Lock()
	am.bootstrapPeers[id] = struct{}{}
	am.markSeenLocked(id, now)
	am.mu.Unlock()
	am.upsertPeerRecord(id.String(), func(r *PeerAddrRecord) {
		r.IsBootstrap = true
		r.LastSeenAt = now
		r.Addrs = mergeStringAddrs(r.Addrs, addrs)
	})
}

// MarkAddrFailed 标记地址连接失败（降级TTL）
func (am *AddrManager) MarkAddrFailed(id libpeer.ID) {
	// 获取现有地址
	addrs := am.peerstore.Addrs(id)
	if len(addrs) == 0 {
		return
	}

	// 降低TTL到5分钟
	am.peerstore.AddAddrs(id, addrs, am.ttl.Failed)

	if am.logger != nil {
		am.logger.Debugf("addr_manager mark_failed peer=%s ttl=%s",
			id.String(), am.ttl.Failed)
	}

	now := time.Now()
	am.mu.Lock()
	am.lastSeenAt[id] = now
	am.mu.Unlock()
	am.upsertPeerRecord(id.String(), func(r *PeerAddrRecord) {
		r.LastSeenAt = now
		r.LastFailedAt = now
		r.FailCount++
	})
}

// GetAddrs 获取peer地址（如果无地址，触发重发现）
func (am *AddrManager) GetAddrs(id libpeer.ID) []ma.Multiaddr {
	addrs := am.peerstore.Addrs(id)

	if len(addrs) == 0 {
		// 🆕 优化：无地址时，除了触发查询，还加入重发现队列
		am.triggerAddrLookup(id)
		am.TriggerRediscovery(id, false) // normal priority
	}

	return addrs
}

// triggerAddrLookup 触发地址查询（异步，防重复）
func (am *AddrManager) triggerAddrLookup(id libpeer.ID) {
	am.mu.Lock()
	// pendingLookups 有界化：防止极端情况下 map 无界增长（比如 refresh 风暴）
	if am.maxPendingLookups > 0 && len(am.pendingLookups) >= am.maxPendingLookups {
		am.mu.Unlock()
		if am.logger != nil {
			am.logger.Warnf("addr_manager pending_lookups_full drop peer=%s size=%d max=%d",
				id.String(), len(am.pendingLookups), am.maxPendingLookups)
		}
		return
	}

	// 检查是否已在查询中
	if am.pendingLookups[id] {
		am.mu.Unlock()
		return
	}

	am.pendingLookups[id] = true
	am.mu.Unlock()

	if am.logger != nil {
		am.logger.Warnf("addr_manager trigger_lookup peer=%s", id.String())
	}

	// 并发限流：避免 refresh/all_discovered 场景把 DHT 打爆
	select {
	case am.lookupSem <- struct{}{}:
		// acquired
	default:
		if am.logger != nil {
			am.logger.Warnf("addr_manager lookup_throttled peer=%s max_concurrent=%d", id.String(), am.maxConcurrentLookups)
		}
		am.mu.Lock()
		delete(am.pendingLookups, id)
		am.mu.Unlock()
		return
	}

	// 异步查询
	go func() {
		defer func() {
			am.mu.Lock()
			delete(am.pendingLookups, id)
			am.mu.Unlock()
			<-am.lookupSem
		}()

		// 检查是否有routing可用
		if am.routing == nil {
			if am.logger != nil {
				am.logger.Warnf("addr_manager lookup_skipped peer=%s reason=no_routing", id.String())
			}
			return
		}

		ctx, cancel := context.WithTimeout(am.ctx, am.lookupTimeout)
		defer cancel()

		// 通过DHT查询peer地址
		if am.logger != nil {
			am.logger.Debugf("addr_manager lookup_start peer=%s", id.String())
		}

	info, err := am.routing.FindPeer(ctx, id)
	if err != nil {
		// 🆕 P0-009: 容错——即使 FindPeer 失败，只要当前 peerstore 仍有地址，也给予“宽限期”续期，
		// 避免地址按原 TTL 直接过期，导致后续出现 addrs=0 -> 无法重连 -> 网络孤岛。
		if existing := am.peerstore.Addrs(id); len(existing) > 0 {
			graceTTL := 30 * time.Minute
			existing = am.capAddrs(existing)
			am.peerstore.AddAddrs(id, existing, graceTTL)
			if am.logger != nil {
				am.logger.Debugf("addr_manager lookup_failed_grace_extended peer=%s grace_ttl=%s addrs=%d",
					id.String(), graceTTL, len(existing))
			}
		}

		if am.logger != nil {
			// "routing: not found" 是正常的 P2P 网络行为（节点离线/未广播），降级为 DEBUG
			// 其他错误（如网络故障、超时等）仍记录为 WARN
			if err.Error() == "routing: not found" {
				am.logger.Debugf("addr_manager lookup_not_in_dht peer=%s", id.String())
			} else {
				am.logger.Warnf("addr_manager lookup_failed peer=%s err=%v", id.String(), err)
			}
		}
		return
	}

		if len(info.Addrs) > 0 {
			am.AddDHTAddr(info.ID, info.Addrs)
			if am.logger != nil {
				am.logger.Infof("addr_manager lookup_success peer=%s addrs=%d", id.String(), len(info.Addrs))
			}
		} else {
			if am.logger != nil {
				am.logger.Warnf("addr_manager lookup_no_addrs peer=%s", id.String())
			}
		}
	}()
}

// loadPersistedRecords 启动时加载持久化记录并回填到 peerstore
func (am *AddrManager) loadPersistedRecords() {
	if am.store == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	recs, err := am.store.LoadAll(ctx)
	if err != nil {
		if am.logger != nil {
			am.logger.Warnf("addr_manager load_persisted_records failed: %v", err)
		}
		return
	}

	now := time.Now()
	loaded := 0
	for _, r := range recs {
		if r == nil || strings.TrimSpace(r.PeerID) == "" {
			continue
		}
		// TTL 过期记录跳过（pruneLoop 会清理）
		if !r.IsBootstrap && am.recordTTL > 0 && !r.LastSeenAt.IsZero() && now.Sub(r.LastSeenAt) > am.recordTTL {
			continue
		}
		id, err := libpeer.Decode(r.PeerID)
		if err != nil || id == "" {
			continue
		}
		addrs := make([]ma.Multiaddr, 0, len(r.Addrs))
		for _, s := range r.Addrs {
			a, err := ma.NewMultiaddr(s)
			if err != nil {
				continue
			}
			addrs = append(addrs, a)
		}
		if len(addrs) == 0 {
			continue
		}

		// 回填
		addrs = am.capAddrs(addrs)
		if r.IsBootstrap {
			am.peerstore.AddAddrs(id, addrs, am.ttl.Bootstrap)
			am.mu.Lock()
			am.bootstrapPeers[id] = struct{}{}
			am.mu.Unlock()
		} else if !r.LastConnectedAt.IsZero() {
			am.peerstore.AddAddrs(id, addrs, am.ttl.Connected)
			am.mu.Lock()
			am.lastConnectedAt[id] = r.LastConnectedAt
			am.mu.Unlock()
		} else {
			am.peerstore.AddAddrs(id, addrs, am.ttl.DHT)
		}
		am.mu.Lock()
		if !r.LastSeenAt.IsZero() {
			am.lastRefreshAt[id] = r.LastSeenAt
			am.lastSeenAt[id] = r.LastSeenAt
		} else {
			am.lastRefreshAt[id] = now
			am.lastSeenAt[id] = now
		}
		am.mu.Unlock()
		loaded++
	}

	if am.logger != nil {
		am.logger.Infof("addr_manager loaded_persisted_records count=%d", loaded)
	}
}

func (am *AddrManager) upsertPeerRecord(peerID string, mutate func(r *PeerAddrRecord)) {
	if am.store == nil || strings.TrimSpace(peerID) == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	rec, ok, err := am.store.Get(ctx, peerID)
	if err != nil {
		if am.logger != nil {
			am.logger.Warnf("addr_manager store_get failed peer=%s err=%v", peerID, err)
		}
		return
	}
	if !ok || rec == nil {
		rec = &PeerAddrRecord{Version: PeerAddrRecordVersion, PeerID: peerID}
	}
	if rec.Version == 0 {
		rec.Version = PeerAddrRecordVersion
	}
	mutate(rec)
	_ = am.store.Upsert(ctx, rec)
}

func mergeStringAddrs(existing []string, addrs []ma.Multiaddr) []string {
	if len(addrs) == 0 {
		return existing
	}
	seen := make(map[string]struct{}, len(existing)+len(addrs))
	out := make([]string, 0, len(existing)+len(addrs))
	for _, s := range existing {
		if strings.TrimSpace(s) == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	for _, a := range addrs {
		if a == nil {
			continue
		}
		s := a.String()
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// pruneLoop 定期清理过期/劣质记录（all_discovered 场景必须）
func (am *AddrManager) pruneLoop() {
	ticker := time.NewTicker(am.pruneInterval)
	defer ticker.Stop()

	if am.logger != nil {
		am.logger.Infof("addr_manager prune_loop started interval=%s ttl=%s", am.pruneInterval, am.recordTTL)
	}

	for {
		select {
		case <-am.ctx.Done():
			if am.logger != nil {
				am.logger.Infof("addr_manager prune_loop stopped")
			}
			return
		case <-ticker.C:
			am.pruneOnce()
		}
	}
}

// MaxAddrManagerMemoryBytes 地址管理器最大内存占用（15MB）
// 超过此值会触发强制淘汰
const MaxAddrManagerMemoryBytes = 15 * 1024 * 1024

func (am *AddrManager) pruneOnce() {
	if am.store == nil || am.recordTTL <= 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	recs, err := am.store.LoadAll(ctx)
	if err != nil {
		if am.logger != nil {
			am.logger.Warnf("addr_manager prune_load_all failed: %v", err)
		}
		return
	}

	now := time.Now()
	deleted := 0

	// 🆕 P1 修复：阶段1 - 清理过期和失败记录
	for _, r := range recs {
		if r == nil || r.IsBootstrap {
			continue
		}
		expired := !r.LastSeenAt.IsZero() && now.Sub(r.LastSeenAt) > am.recordTTL
		tooManyFails := r.FailCount >= 50 && (r.LastConnectedAt.IsZero() || now.Sub(r.LastConnectedAt) > 24*time.Hour)
		if expired || tooManyFails {
			_ = am.store.Delete(ctx, r.PeerID)
			deleted++
		}
	}

	// 🆕 P1 修复：阶段2 - 内存上限检查和 LRU 淘汰
	// 重新加载剩余记录
	recs, err = am.store.LoadAll(ctx)
	if err == nil {
		am.pruneByMemoryLimit(ctx, recs, now)
	}

	if deleted > 0 && am.logger != nil {
		am.logger.Infof("addr_manager prune_done deleted=%d total=%d", deleted, len(recs))
	}
}

// pruneByMemoryLimit 根据内存上限淘汰记录
// 当记录数超过 maxTrackedPeers 或估算内存超过 MaxAddrManagerMemoryBytes 时触发 LRU 淘汰
func (am *AddrManager) pruneByMemoryLimit(ctx context.Context, recs []*PeerAddrRecord, now time.Time) {
	// maxTrackedPeers 未配置时使用默认值，避免测试/非标准构造导致“所有记录被误删”
	effectiveMaxTrackedPeers := am.maxTrackedPeers
	if effectiveMaxTrackedPeers <= 0 {
		effectiveMaxTrackedPeers = 5000
	}

	// 统计非 bootstrap 记录数量
	nonBootstrapCount := 0
	for _, r := range recs {
		if r != nil && !r.IsBootstrap {
			nonBootstrapCount++
		}
	}

	// 估算当前内存占用（每个记录约 2KB）
	estimatedMemory := int64(nonBootstrapCount) * 2 * 1024

	// 判断是否需要淘汰
	needPrune := false
	var pruneReason string

	if nonBootstrapCount > effectiveMaxTrackedPeers {
		needPrune = true
		pruneReason = "records_exceed_max"
	} else if estimatedMemory > MaxAddrManagerMemoryBytes {
		needPrune = true
		pruneReason = "memory_exceed_limit"
	}

	if !needPrune {
		return
	}

	// 计算需要淘汰的数量
	// 目标：降到 maxTrackedPeers 的 80% 或内存上限的 80%
	targetCount := int(float64(effectiveMaxTrackedPeers) * 0.8)
	targetMemory := int64(float64(MaxAddrManagerMemoryBytes) * 0.8)
	targetByMemory := int(targetMemory / (2 * 1024))

	if targetByMemory < targetCount {
		targetCount = targetByMemory
	}

	needEvict := nonBootstrapCount - targetCount
	if needEvict <= 0 {
		return
	}

	// 🆕 LRU 淘汰：按 LastSeenAt 排序，淘汰最久未见的
	type candidate struct {
		peerID   string
		lastSeen time.Time
	}

	candidates := make([]candidate, 0, nonBootstrapCount)
	for _, r := range recs {
		if r == nil || r.IsBootstrap {
			continue
		}
		// 跳过近期连接过的
		if !r.LastConnectedAt.IsZero() && now.Sub(r.LastConnectedAt) < 24*time.Hour {
			continue
		}
		candidates = append(candidates, candidate{
			peerID:   r.PeerID,
			lastSeen: r.LastSeenAt,
		})
	}

	// 按 LastSeenAt 升序排序（最老的在前面）
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].lastSeen.IsZero() && !candidates[j].lastSeen.IsZero() {
			return true
		}
		if !candidates[i].lastSeen.IsZero() && candidates[j].lastSeen.IsZero() {
			return false
		}
		return candidates[i].lastSeen.Before(candidates[j].lastSeen)
	})

	// 执行淘汰
	evicted := 0
	for i := 0; i < needEvict && i < len(candidates); i++ {
		_ = am.store.Delete(ctx, candidates[i].peerID)
		evicted++
	}

	if evicted > 0 && am.logger != nil {
		am.logger.Warnf("addr_manager prune_by_memory_limit reason=%s evicted=%d target_count=%d estimated_memory_mb=%.1f",
			pruneReason, evicted, targetCount, float64(estimatedMemory)/(1024*1024))
	}
}

// handleConnectionEvents 处理连接事件（在addr_refresh.go中实现）
func (am *AddrManager) handleConnectionEvents() {
	// 订阅libp2p连接事件
	sub, err := am.host.EventBus().Subscribe(new(libevent.EvtPeerConnectednessChanged))
	if err != nil {
		if am.logger != nil {
			am.logger.Errorf("addr_manager subscribe_events failed: %v", err)
		}
		return
	}
	defer sub.Close()

	if am.logger != nil {
		am.logger.Infof("addr_manager event_handler started")
	}

	for {
		select {
		case <-am.ctx.Done():
			if am.logger != nil {
				am.logger.Infof("addr_manager event_handler stopped")
			}
			return

		case e := <-sub.Out():
			evt, ok := e.(libevent.EvtPeerConnectednessChanged)
			if !ok {
				continue
			}
			am.handleConnectednessChange(evt)
		}
	}
}

// handleConnectednessChange 处理连接状态变化
func (am *AddrManager) handleConnectednessChange(evt libevent.EvtPeerConnectednessChanged) {
	switch evt.Connectedness {
	case libnetwork.Connected:
		// 连接成功，升级地址TTL
		addrs := am.peerstore.Addrs(evt.Peer)
		if len(addrs) > 0 {
			am.AddConnectedAddr(evt.Peer, addrs)
		}

	case libnetwork.NotConnected:
		// 连接断开，保持现有TTL（不降级，允许重连）
		if am.logger != nil {
			am.logger.Debugf("addr_manager peer_disconnected peer=%s", evt.Peer.String())
		}

	case libnetwork.CannotConnect:
		// 无法连接，降级TTL
		am.MarkAddrFailed(evt.Peer)
	}
}

