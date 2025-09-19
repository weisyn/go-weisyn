package discovery

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	lpdisc "github.com/libp2p/go-libp2p/core/discovery"
	lphost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	routdisc "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	ma "github.com/multiformats/go-multiaddr"
	nodeconfig "github.com/weisyn/v1/internal/config/node"
	hostrt "github.com/weisyn/v1/internal/core/infrastructure/node/impl/host"
	eventiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	storageiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// Runtime 聚合 mDNS 与 DHT，并负责：
// 1) 在启动期按配置启动 mDNS/DHT；
// 2) 基于引导节点进行主动拨号与保活；
// 3) 根据连接稳定度自适应调节拨号间隔，降低对网络的扰动；
// 4) 通过事件总线发布轻量网络事件。
type Runtime struct {
	mdns       *mdnsRuntime
	dht        *dhtRuntime
	cfg        *nodeconfig.NodeOptions
	log        logiface.Logger
	hostHandle interface{ Host() lphost.Host }
	cancel     context.CancelFunc
	bus        eventiface.EventBus
	store      storageiface.Provider
	rd         lpdisc.Discovery
}

func NewRuntime(cfg *nodeconfig.NodeOptions, logger logiface.Logger, hostHandle interface{ Host() lphost.Host }, eb eventiface.EventBus, sp storageiface.Provider) (*Runtime, error) {
	dr := &Runtime{cfg: cfg, log: logger, hostHandle: hostHandle, bus: eb, store: sp}
	// 注意：不在此处获取host，因为host可能还没有启动
	// mdns和dht的初始化会在Start()时进行
	return dr, nil
}

func (r *Runtime) Start(ctx context.Context) error {
	if r.log != nil {
		r.log.Infof("p2p.discovery.runtime starting")
	}

	// 在Start时获取host，确保host已经启动
	host := r.hostHandle.Host()
	if host == nil {
		return fmt.Errorf("host is nil, cannot start discovery")
	}
	if r.log != nil {
		r.log.Infof("p2p.discovery.runtime host id=%s addrs=%v", host.ID().String(), host.Addrs())
		// 配置快照：帮助线上排障快速定位差异
		if r.cfg != nil {
			// 配置日志（简化）
			bootstrapCount := len(r.cfg.Discovery.BootstrapPeers)
			minPeers := r.cfg.Connectivity.MinPeers
			r.log.Infof("p2p.discovery.config mdns=%t dht=%t bootstrap=%d min_peers=%d",
				r.cfg.Discovery.MDNS.Enabled, r.cfg.Discovery.DHT.Enabled, bootstrapCount, minPeers,
			)
		} else {
			// 零配置快照
			r.log.Infof("p2p.discovery.config zero_config=true")
		}
	}

	// 初始化mdns和dht runtime
	r.mdns = newMDNSRuntime(r.cfg, r.log, host, r.bus)
	r.dht = newDHTRuntime(r.cfg, r.log, host)

	// 若 host 侧启用了 diagnostics，则将轻量事件回调与指标桥接
	if hh, ok := r.hostHandle.(interface {
		GetDiagnosticsManager() *hostrt.DiagnosticsManager
	}); ok {
		if dm := hh.GetDiagnosticsManager(); dm != nil {
			// mDNS 事件回调
			r.mdns.onPeerFound = func() { dm.RecordDiscoveryMDNSPeerFound(); dm.UpdateDiscoveryLastMDNSTS() }
			r.mdns.onConnOK = func() { dm.RecordDiscoveryMDNSConnectOK() }
			r.mdns.onConnFail = func() { dm.RecordDiscoveryMDNSConnectFail() }
		}
	}

	// mDNS/DHT 启动策略：始终尝试启动（失败仅告警，不阻断）
	if r.log != nil {
		r.log.Infof("p2p.discovery.runtime start_mdns policy=always_on")
	}
	_ = r.mdns.Start(ctx)
	if r.log != nil {
		r.log.Infof("p2p.discovery.runtime start_dht policy=always_on")
	}
	_ = r.dht.Start(ctx)
	// 启用 DHT rendezvous：广播自身并查找对端
	if r.dht != nil && r.dht.ContentRouting() != nil {
		// ns := "weisyn-weisgn111"
		ns := rendezvousString(r.cfg)

		if r.log != nil {
			r.log.Infof("p2p.discovery.rendezvous starting_findPeersLoop ns=%s", ns)
		}
		go r.findPeersLoop(ctx, r.rd, ns, host)
	} else {
		if r.log != nil {
			r.log.Warnf("p2p.discovery.rendezvous disabled dht=%v content_routing=%v",
				r.dht != nil, r.dht != nil && r.dht.ContentRouting() != nil)
		}
	}

	// 引导节点拨号（退避 + 动态调度）
	if r.cfg != nil {
		peers := r.cfg.Discovery.BootstrapPeers
		if r.log != nil {
			r.log.Infof("p2p.discovery.bootstrap start peers=%d", len(peers))
		}

		// 立即尝试一次同步拨号
		if len(peers) > 0 {
			if r.log != nil {
				r.log.Infof("p2p.discovery.bootstrap sync_dial begin")
			}
			// 记录一次尝试
			if hh, ok := r.hostHandle.(interface {
				GetDiagnosticsManager() *hostrt.DiagnosticsManager
			}); ok {
				if dm := hh.GetDiagnosticsManager(); dm != nil {
					dm.RecordDiscoveryBootstrapAttempt()
				}
			}
			success, _ := r.tryDialOnce(ctx, peers, host)
			if success {
				if hh, ok := r.hostHandle.(interface {
					GetDiagnosticsManager() *hostrt.DiagnosticsManager
				}); ok {
					if dm := hh.GetDiagnosticsManager(); dm != nil {
						dm.RecordDiscoveryBootstrapSuccess()
						dm.UpdateDiscoveryLastBootstrapTS()
					}
				}
			}
		}

		cctx, cancel := context.WithCancel(ctx)
		r.cancel = cancel
		go r.schedulerLoop(cctx, peers, host)
		// 订阅Hint触发短促发现
		r.subscribeHints(cctx, r.bus, peers)
	}

	if r.log != nil {
		r.log.Infof("p2p.discovery.runtime started")
	}
	return nil
}

// Stop 关闭 Host 和所有相关服务。
func (r *Runtime) Stop(ctx context.Context) error {
	if r.cancel != nil {
		r.cancel()
	}
	if r.dht != nil {
		_ = r.dht.Stop(ctx)
	}
	if r.mdns != nil {
		_ = r.mdns.Stop(ctx)
	}
	return nil
}

func (r *Runtime) schedulerLoop(ctx context.Context, peers []string, host lphost.Host) {
	if len(peers) == 0 || host == nil {
		return
	}
	if r.log != nil {
		r.log.Infof("p2p.discovery.scheduler start peers=%d connected=%d", len(peers), len(host.Network().Peers()))
	}
	// 初始快速退避尝试 - 优化退避策略，增加成功率
	b := NewBackoff(2*time.Second, 60*time.Second, 1.5, 0.1)
	for i := 0; i < 5; i++ {
		success, roundConn := r.tryDialOnce(ctx, peers, host)
		if r.log != nil {
			r.log.Infof("p2p.discovery.bootstrap_fast attempt=%d success=%t connected_round=%d", i+1, success, roundConn)
		}
		if success {
			break // 已连上引导，跳出快速尝试进入周期检测维持
		}
		d := b.Next()
		if r.log != nil {
			r.log.Infof("p2p.discovery.backoff sleep=%s", d)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(d):
		}
	}
	// 动态周期：空闲降频（连接稳定则指数增加间隔）+ 稳定后延后（使用 advertise_interval）
	// 使用配置的 discovery_interval 作为 baseInterval；advertise_interval 作为稳定延后上限
	baseInterval := 5 * time.Minute
	maxInterval := 15 * time.Minute
	if r.cfg != nil {
		if r.cfg.Discovery.DiscoveryInterval > 0 {
			baseInterval = r.cfg.Discovery.DiscoveryInterval
		}
		if r.cfg.Discovery.AdvertiseInterval > 0 {
			maxInterval = r.cfg.Discovery.AdvertiseInterval
		}
	}
	dynamic := baseInterval
	stableTarget := r.cfg.Connectivity.MinPeers
	if stableTarget <= 0 {
		stableTarget = 8
	}
	stableCount := 0
	stableThreshold := 3
	if r.log != nil {
		r.log.Infof("p2p.discovery.scheduler_config base_interval=%s max_interval=%s stable_target=%d threshold=%d", baseInterval, maxInterval, stableTarget, stableThreshold)
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		// 尝试一次拨号
		success, roundConn := r.tryDialOnce(ctx, peers, host)
		connected := len(host.Network().Peers())
		if r.log != nil {
			r.log.Infof("p2p.discovery.cycle interval=%s connected=%d success=%t connected_round=%d stableCount=%d target=%d", dynamic, connected, success, roundConn, stableCount, stableTarget)
		}
		if success {
			// 网络稳定延后：使用最大间隔等待一段时间，避免刚连上又立即打扰
			d := jitter(maxInterval, 0.1)
			if r.log != nil {
				r.log.Infof("p2p.discovery.stable_delay sleep=%s", d)
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
				if r.log != nil {
					r.log.Infof("p2p.discovery.interval_update from=%s to=%s reason=stable", old, dynamic)
				}
			}
		} else {
			// 不稳定则恢复为基础间隔
			if dynamic != baseInterval {
				old := dynamic
				dynamic = baseInterval
				if r.log != nil {
					r.log.Infof("p2p.discovery.interval_update from=%s to=%s reason=unstable", old, dynamic)
				}
			}
			stableCount = 0
		}
		// 等待下个周期，加入轻微抖动避免同步风暴
		d := jitter(dynamic, 0.1)
		if r.log != nil {
			r.log.Infof("p2p.discovery.sleep sleep=%s", d)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(d):
		}
	}
}

func jitter(d time.Duration, frac float64) time.Duration {
	if frac <= 0 {
		return d
	}
	f := 1 + (rand.Float64()*2-1)*frac
	return time.Duration(float64(d) * f)
}

// tryDialOnce 进行一轮引导拨号，返回是否至少连接成功一个节点，以及本轮成功数量
func (r *Runtime) tryDialOnce(ctx context.Context, peers []string, host lphost.Host) (bool, int) {
	var connected int
	roundStart := time.Now()
	if r.log != nil {
		r.log.Debugf("p2p.discovery.dial_round begin peers=%d", len(peers))
	}
	for _, s := range peers {
		if r.log != nil {
			r.log.Debugf("p2p.discovery.dial_peer start addr=%s", s)
		}
		m, err := ma.NewMultiaddr(s)
		if err != nil {
			if r.log != nil {
				r.log.Errorf("无效的multiaddr: %s, error: %v", s, err)
			}
			continue
		}
		info, err := peer.AddrInfoFromP2pAddr(m)
		if err != nil {
			if r.log != nil {
				r.log.Errorf("无法解析peer地址: %s, error: %v", s, err)
			}
			continue
		}
		cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		perStart := time.Now()
		err = host.Connect(cctx, *info)
		if err == nil {
			connected++
			if r.log != nil {
				r.log.Infof("成功连接到peer: %s (%s) duration=%s", info.ID, s, time.Since(perStart))
			}
		} else {
			if r.log != nil {
				r.log.Errorf("连接peer失败: %s (%s), error: %v duration=%s", info.ID, s, err, time.Since(perStart))
			}
		}
		cancel()
	}
	if r.log != nil {
		r.log.Debugf("p2p.discovery.dial_round end success=%d duration=%s", connected, time.Since(roundStart))
	}
	return connected > 0, connected
}

// findPeersLoop：通过 DHT rendezvous 持续发现对端并尝试连接
func (r *Runtime) findPeersLoop(ctx context.Context, d lpdisc.Discovery, ns string, host lphost.Host) {
	if host == nil {
		if r.log != nil {
			r.log.Warnf("p2p.discovery.dht_loop discovery=%v host=%v", d != nil, host != nil)
		}
		return
	}
	if r.log != nil {
		r.log.Infof("p2p.discovery.dht_loop starting ns=%s host_id=%s", ns, host.ID().String())
	}

	// 主循环：持续重启DHT发现
	for {
		select {
		case <-ctx.Done():
			if r.log != nil {
				r.log.Infof("p2p.discovery.dht_loop context_cancelled_main ns=%s", ns)
			}
			return
		default:
		}

		// 启动一轮DHT发现
		shouldRestart := r.runDHTDiscoveryRound(ctx, ns, host)
		if !shouldRestart {
			// 如果不需要重启（例如context取消），则退出主循环
			return
		}

		// 等待5秒后重启下一轮

		if r.log != nil {
			r.log.Infof("p2p.discovery.dht_loop starting_5s_wait ns=%s", ns)
		}
		select {
		case <-ctx.Done():
			if r.log != nil {
				r.log.Infof("p2p.discovery.dht_loop context_cancelled_during_wait ns=%s", ns)
			}
			return
		case <-time.After(5 * time.Second):
			if r.log != nil {
				r.log.Infof("p2p.discovery.dht_loop restarting_after_close ns=%s", ns)
			}
			// 继续下一轮循环
		}
	}
}

// runDHTDiscoveryRound 运行一轮DHT发现，返回是否需要重启
func (r *Runtime) runDHTDiscoveryRound(ctx context.Context, ns string, host lphost.Host) bool {
	// 使用logger而不是fmt.Println，避免干扰CLI界面
	if r.log != nil {
		r.log.Infof("🔄 DHT重启循环开始")
		r.log.Infof("p2p.discovery.dht_loop calling_FindPeers ns=%s", ns)
	}

	routingDiscovery := routdisc.NewRoutingDiscovery(r.dht.kdht)
	// Advertise 是一个实用函数，它通过 Advertiser 持久地为服务做广播。
	dutil.Advertise(ctx, routingDiscovery, ns)

	pch, err := routingDiscovery.FindPeers(ctx, ns)
	if err != nil {

		if r.log != nil {
			r.log.Warnf("p2p.discovery.rendezvous find_peers_error ns=%s err=%v", ns, err)
		}
		return false // 出错时不重启
	}

	if r.log != nil {
		r.log.Infof("p2p.discovery.dht_loop peer_channel_ready ns=%s, waiting_for_peers", ns)
		// 检查DHT状态
		if r.dht != nil {
			r.log.Infof("p2p.discovery.dht_loop dht_rt_size=%d connected_peers=%d bootstrap_peers=%d",
				r.dht.GetRoutingTableSize(), len(host.Network().Peers()), len(host.Network().ConnsToPeer(host.ID())))
			// 显示连接的引导节点
			for _, peerID := range host.Network().Peers() {
				r.log.Debugf("p2p.discovery.dht_loop connected_peer id=%s", peerID.String())
			}
		}
	}

	for {

		select {
		case <-ctx.Done():

			if r.log != nil {
				r.log.Infof("p2p.discovery.dht_loop context_cancelled ns=%s", ns)
			}
			return false // context取消时不重启
		case info, ok := <-pch:
			if !ok {

				if r.log != nil {
					r.log.Warnf("p2p.discovery.dht_loop channel_closed_unexpectedly ns=%s, should_restart=true", ns)
					// 检查DHT状态
					if r.dht != nil {
						r.log.Infof("p2p.discovery.dht_loop final_dht_rt_size=%d connected_peers=%d",
							r.dht.GetRoutingTableSize(), len(host.Network().Peers()))
					}
				}
				return true // 通道关闭时需要重启
			}

			// 处理发现的peer
			r.handleDiscoveredPeer(ctx, info, host, ns)
		}
	}
}

// handleDiscoveredPeer 处理发现的peer
func (r *Runtime) handleDiscoveredPeer(ctx context.Context, info peer.AddrInfo, host lphost.Host, ns string) {
	// 使用logger而不是fmt.Printf，避免干扰CLI界面
	if r.log != nil {
		r.log.Infof("🎉 发现新peer: %s", info.ID.String())
		r.log.Infof("p2p.discovery.dht_loop peer_discovered id=%s addrs=%d ns=%s", info.ID.String(), len(info.Addrs), ns)
	}

	// Debug: 检查发现的节点ID
	if r.log != nil {
		r.log.Debugf("p2p.discovery.dht_loop peer_check discovered_id=%s self_id=%s", info.ID.String(), host.ID().String())
	}

	if info.ID == "" || info.ID == host.ID() {
		// 使用logger而不是fmt.Printf
		if r.log != nil {
			reason := func() string {
				if info.ID == "" {
					return "empty_id"
				}
				return "self_id"
			}()
			r.log.Infof("⏩ 跳过peer (原因: %s): %s", reason, info.ID.String())
		}
		if r.log != nil {
			r.log.Debugf("p2p.discovery.dht_loop skip_peer reason=%s id=%s",
				func() string {
					if info.ID == "" {
						return "empty_id"
					}
					return "self_id"
				}(), info.ID.String())
		}
		return
	}

	if r.log != nil {
		r.log.Infof("p2p.discovery.dht_loop connecting_to_peer id=%s addrs=%v", info.ID.String(), info.Addrs)
	}

	cctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	host.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)

	err := host.Connect(cctx, info)
	if err == nil {
		if r.log != nil {
			r.log.Infof("p2p.discovery.dht_loop connect_success id=%s", info.ID.String())
		}

		// 发布peer连接成功事件（使用标准事件类型）
		if r.bus != nil {
			// EventBus.Publish方法签名：Publish(eventType EventType, args ...interface{})
			// K桶管理器的handler期望：func(ctx context.Context, data interface{}) error
			// 因此发布时传递context.Background()和peer.ID作为两个参数
			r.bus.Publish("network.peer.connected", context.Background(), info.ID)
			if r.log != nil {
				r.log.Infof("📡 发布peer连接事件: %s", info.ID.String()[:12]+"...")
			}
		} else if r.log != nil {
			r.log.Warnf("⚠️ 事件总线为nil，无法发布peer连接事件: %s", info.ID.String()[:12]+"...")
		}

	} else {
		if r.log != nil {
			r.log.Warnf("p2p.discovery.dht_loop connect_failed id=%s error=%v", info.ID.String(), err)
		}
	}
}
