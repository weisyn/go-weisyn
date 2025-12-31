package routing

import (
	"context"
	"fmt"
	"strings"

	ds "github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	lphost "github.com/libp2p/go-libp2p/core/host"
	libpeer "github.com/libp2p/go-libp2p/core/peer"
	routdisc "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"

	p2pcfg "github.com/weisyn/v1/internal/config/p2p"
	"github.com/weisyn/v1/internal/core/p2p/interfaces"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
)

// Service Routing 服务实现
//
// 对标 Kubo Routing：基于 DHT 的 Peer 路由与发现
type Service struct {
	host lphost.Host
	kdht *dht.IpfsDHT

	mode p2pi.DHTMode
	opts *p2pcfg.Options

	logger logiface.Logger
	ctx    context.Context

	// offline 标记当前 Routing 是否处于“离线模式”
	// 场景：
	// - 显式关闭 DHT（EnableDHT=false）
	// - DHT 初始化失败（例如配置错误或网络异常）
	offline bool
}

var _ p2pi.Routing = (*Service)(nil)
var _ interfaces.RendezvousRouting = (*Service)(nil)

// NewService 创建 Routing 服务
func NewService(mode p2pi.DHTMode) *Service {
	return &Service{
		mode: mode,
	}
}

// Initialize 初始化 DHT（需要 Host 和配置）
func (s *Service) Initialize(host lphost.Host, opts *p2pcfg.Options, logger logiface.Logger) error {
	if host == nil {
		return fmt.Errorf("host is required")
	}

	s.host = host
	s.opts = opts
	s.logger = logger
	s.ctx = context.Background()

	// 如果未启用 DHT，则进入“离线模式”（不进行任何 DHT 网络交互）
	if opts != nil && !opts.EnableDHT {
		s.offline = true
		if logger != nil {
			logger.Infof("p2p.routing.dht disabled by config, routing offline")
		}
		return nil
	}

	// 转换 DHT 模式
	mode := dht.ModeAuto
	switch s.mode {
	case p2pi.DHTModeClient:
		mode = dht.ModeClient
	case p2pi.DHTModeServer:
		mode = dht.ModeServer
	case p2pi.DHTModeLAN:
		mode = dht.ModeClient // LAN 模式使用 client 模式
	default:
		mode = dht.ModeAuto
	}

	// 创建 DHT 选项
	//
	// 🆕 libp2p 资源控制：使用 WES 专属 DHT 协议前缀
	// 背景：WES 节点连接到全球公共 libp2p DHT，导致大量非 WES 节点涌入
	// 解决方案：
	// - 使用 "/wes" 协议前缀，创建 WES 专属 DHT 网络
	// - 与公共 DHT（IPFS/kubo 等）隔离，减少非业务连接
	// - 使用 RoutingTableFilter 过滤非 WES 节点
	// 参考：LIBP2P_GOROUTINE_ANALYSIS.md
	dhtOpts := []dht.Option{
		dht.Mode(mode),
		dht.Datastore(dsync.MutexWrap(ds.NewMapDatastore())),
		// 🆕 WES 专属 DHT 协议前缀
		// 使得 WES 节点只与其他 WES 节点进行 DHT 交互
		dht.ProtocolPrefix("/wes"),
		dht.V1ProtocolOverride("/wes/kad/1.0.0"),
		// 🆕 降低 DHT 路由表桶大小，减少内存占用
		// 默认值为 20，降低到 10 可减少约 50% 的路由表内存
		dht.BucketSize(10),
		// 🆕 DHT 路由表 WES 过滤器
		// 只允许支持 /weisyn/ 协议的节点进入 DHT 路由表
		// 这样可以进一步减少非 WES 节点占用的资源
		dht.RoutingTableFilter(s.wesRoutingTableFilter(host)),
	}

	// 创建 DHT 实例
	kdht, err := dht.New(s.ctx, host, dhtOpts...)
	if err != nil {
		// 初始化失败时标记为离线模式，避免影响整体 P2P 运行时
		s.offline = true
		if logger != nil {
			logger.Warnf("p2p.routing.dht init failed, routing offline: %v", err)
		}
		return fmt.Errorf("create dht: %w", err)
	}

	s.kdht = kdht

	if logger != nil {
		logger.Infof("p2p.routing.dht initialized mode=%v", mode)
	}

	return nil
}

// FindPeer 查找指定 PeerID 的地址信息
func (s *Service) FindPeer(ctx context.Context, id libpeer.ID) (libpeer.AddrInfo, error) {
	if s.kdht == nil {
		return libpeer.AddrInfo{}, fmt.Errorf("dht not initialized")
	}

	return s.kdht.FindPeer(ctx, id)
}

// FindClosestPeers 查找最接近指定 key 的 Peer 列表
func (s *Service) FindClosestPeers(ctx context.Context, key []byte, count int) (<-chan libpeer.AddrInfo, error) {
	if s.kdht == nil {
		return nil, fmt.Errorf("dht not initialized")
	}

	// 使用 DHT 路由表查找最接近的 Peer
	// 注意：GetClosestPeers 的 API 可能不同，这里使用路由表方法
	rt := s.kdht.RoutingTable()
	if rt == nil {
		return nil, fmt.Errorf("routing table not available")
	}

	// 从路由表获取最接近的 Peer
	peerIDs := rt.NearestPeers(key, count)
	if len(peerIDs) == 0 {
		// 返回空 channel
		peerChan := make(chan libpeer.AddrInfo)
		close(peerChan)
		return peerChan, nil
	}

	// 转换为 AddrInfo channel
	peerChan := make(chan libpeer.AddrInfo, len(peerIDs))
	go func() {
		defer close(peerChan)
		for _, peerID := range peerIDs {
			// 从 peerstore 获取地址
			addrs := s.host.Peerstore().Addrs(peerID)
			peerChan <- libpeer.AddrInfo{
				ID:    peerID,
				Addrs: addrs,
			}
		}
	}()

	return peerChan, nil
}

// Bootstrap 执行 DHT Bootstrap
func (s *Service) Bootstrap(ctx context.Context) error {
	if s.kdht == nil {
		// 在离线模式下，Bootstrap 视为 no-op，避免上层反复报错
		if s.offline {
			if s.logger != nil {
				s.logger.Infof("p2p.routing.dht bootstrap skipped (offline mode)")
			}
			return nil
		}
		return fmt.Errorf("dht not initialized")
	}

	if err := s.kdht.Bootstrap(ctx); err != nil {
		if s.logger != nil {
			s.logger.Warnf("p2p.routing.dht bootstrap failed: %v", err)
		}
		return fmt.Errorf("dht bootstrap: %w", err)
	}

	if s.logger != nil {
		if s.kdht.RoutingTable() != nil {
			s.logger.Infof("p2p.routing.dht bootstrap ok rt_size=%d", s.kdht.RoutingTable().Size())
		} else {
			s.logger.Infof("p2p.routing.dht bootstrap ok")
		}
	}

	return nil
}

// Mode 返回当前 DHT 模式
func (s *Service) Mode() p2pi.DHTMode {
	return s.mode
}

// GetDHT 返回底层 DHT 实例（供内部使用）
// TODO: 后续可以考虑收紧使用范围，仅保留给极少数诊断场景使用。
func (s *Service) GetDHT() *dht.IpfsDHT {
	return s.kdht
}

// Offline 返回当前 Routing 是否处于离线模式
//
// 离线模式定义：
// - 配置显式关闭 DHT（EnableDHT=false），或
// - DHT 初始化失败导致 kdht 为空。
//
// 该方法不在 p2pi.Routing 接口中，只用于 Diagnostics 等内部观测。
func (s *Service) Offline() bool {
	// 优先使用显式标记，其次根据 DHT 实例是否存在进行推断
	return s.offline || s.kdht == nil
}

// ============= RendezvousRouting 实现（供 Discovery 使用） =============

// AdvertiseAndFindPeers 在指定命名空间下执行广告与发现，返回对端 AddrInfo channel
//
// - 若处于离线模式（offline=true 或 kdht 为空），返回已关闭的 channel，避免阻塞调用方；
// - 若 DHT 未初始化且非离线模式，返回错误。
func (s *Service) AdvertiseAndFindPeers(ctx context.Context, ns string) (<-chan libpeer.AddrInfo, error) {
	if s.kdht == nil {
		if s.offline {
			// 离线模式下：直接返回错误，让上层 Discovery 停止 DHT rendezvous 循环，
			// 避免“空 channel 立刻关闭 → Discovery 误判为异常并无限重启”的假故障/刷屏。
			if s.logger != nil {
				s.logger.Infof("p2p.routing.rendezvous skipped (offline) ns=%s", ns)
			}
			return nil, fmt.Errorf("routing offline")
		}
		return nil, fmt.Errorf("dht not initialized")
	}

	rd := routdisc.NewRoutingDiscovery(s.kdht)
	// Advertise 是一个实用函数，它通过 Advertiser 持久地为服务做广播。
	dutil.Advertise(ctx, rd, ns)

	ch, err := rd.FindPeers(ctx, ns)
	if err != nil {
		if s.logger != nil {
			s.logger.Warnf("p2p.routing.rendezvous find_peers_error ns=%s err=%v", ns, err)
		}
		return nil, fmt.Errorf("rendezvous find_peers: %w", err)
	}

	return ch, nil
}

// RoutingTableSize 返回当前 DHT 路由表大小（不可用时返回 0）
func (s *Service) RoutingTableSize() int {
	if s.kdht == nil || s.kdht.RoutingTable() == nil {
		return 0
	}
	return s.kdht.RoutingTable().Size()
}

// wesRoutingTableFilter 返回 WES 节点路由表过滤器
//
// 🆕 DHT 路由表 WES 过滤：只允许支持 /weisyn/ 协议的节点进入 DHT 路由表
//
// 背景：
// - 阿里云节点 Goroutine 峰值 34,832（19x 本地），大量非 WES 节点涌入
// - DHT 路由表存储大量非 WES 节点，占用内存和 Goroutine
//
// 策略：
// - 只有支持 /weisyn/ 协议的节点才能进入 DHT 路由表
// - 非 WES 节点（如 IPFS/kubo）将被过滤掉
//
// 参考：LIBP2P_GOROUTINE_ANALYSIS.md
func (s *Service) wesRoutingTableFilter(host lphost.Host) dht.RouteTableFilterFunc {
	// 注意：RouteTableFilterFunc 的签名是 func(dht interface{}, p peer.ID) bool
	return func(_ interface{}, peerID libpeer.ID) bool {
		// 如果 host 不可用，允许所有节点（降级策略）
		if host == nil {
			return true
		}

		// 获取节点支持的协议
		protos, err := host.Peerstore().GetProtocols(peerID)
		if err != nil {
			// 无法获取协议信息，使用降级策略：允许入桶
			// 后续健康检查会清理无效节点
			if s.logger != nil {
				s.logger.Debugf("p2p.routing.dht_filter peer=%s get_protos_err=%v (allowing)", peerID.String()[:12], err)
			}
			return true
		}

		// 检查是否支持 /weisyn/ 协议
		for _, p := range protos {
			if strings.Contains(string(p), "/weisyn/") {
				return true // WES 业务节点，允许入桶
			}
		}

		// 非 WES 节点：拒绝入桶
		if s.logger != nil {
			s.logger.Debugf("p2p.routing.dht_filter peer=%s rejected (no /weisyn/ protocol)", peerID.String()[:12])
		}
		return false
	}
}
