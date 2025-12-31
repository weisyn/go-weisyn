package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	lphost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/metrics"
	autonat "github.com/libp2p/go-libp2p/p2p/host/autonat"

	p2pcfg "github.com/weisyn/v1/internal/config/p2p"
	"github.com/weisyn/v1/internal/core/p2p/connectivity"
	"github.com/weisyn/v1/internal/core/p2p/diagnostics"
	"github.com/weisyn/v1/internal/core/p2p/discovery"
	p2phost "github.com/weisyn/v1/internal/core/p2p/host"
	p2phostpkg "github.com/weisyn/v1/internal/core/p2p/host"
	"github.com/weisyn/v1/internal/core/p2p/interfaces"
	"github.com/weisyn/v1/internal/core/p2p/keepalive"
	"github.com/weisyn/v1/internal/core/p2p/routing"
	"github.com/weisyn/v1/internal/core/p2p/swarm"
	cfgprovider "github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	"github.com/libp2p/go-libp2p/core/peer"
)

// Runtime P2P 运行时实现
//
// 组合所有子系统（Swarm / Routing / Discovery / Connectivity / Diagnostics / Keepalive）
// 实现 InternalP2P 接口
type Runtime struct {
	host         lphost.Host
	hostRuntime  *p2phost.HostRuntime // 保存 host.Runtime 引用，用于访问 ConnectionProtector 等
	swarm        p2pi.Swarm
	routing      p2pi.Routing
	discovery    p2pi.Discovery
	connectivity p2pi.Connectivity
	diagnostics  p2pi.Diagnostics
	keepalive    *keepalive.KeyPeerMonitor // KeyPeer监控保活

	logger         logiface.Logger
	opts           *p2pcfg.Options
	eventBus       event.EventBus
	configProvider cfgprovider.Provider // 配置提供者，用于获取 network_id
}

var _ interfaces.InternalP2P = (*Runtime)(nil)

// Options 暴露运行时加载的 P2P 配置（仅供内部模块使用，例如 network 注入 forceConnect 配置）
func (r *Runtime) Options() *p2pcfg.Options {
	if r == nil {
		return nil
	}
	return r.opts
}

// NewRuntime 创建 P2P 运行时
func NewRuntime(opts *p2pcfg.Options, logger logiface.Logger, eb event.EventBus) (*Runtime, error) {
	return NewRuntimeWithConfig(opts, logger, eb, nil)
}

// NewRuntimeWithConfig 创建 P2P 运行时（带配置提供者）
func NewRuntimeWithConfig(opts *p2pcfg.Options, logger logiface.Logger, eb event.EventBus, configProvider cfgprovider.Provider) (*Runtime, error) {
	rt := &Runtime{
		logger:         logger,
		opts:           opts,
		eventBus:       eb,
		configProvider: configProvider,
	}

	return rt, nil
}

// InitHost 确保底层 libp2p Host 已经构建
//
// - 在 Fx 构造阶段（ProvideService）会调用一次，以便 Network 模块可以立即获得 Host
// - 在 Runtime.Start 中也会调用（幂等），确保在生命周期启动阶段 Host 已就绪
func (r *Runtime) InitHost(ctx context.Context) error {
	if r.host != nil {
		// 已经初始化过，直接返回
		return nil
	}

	// 1. 构建 libp2p Host
	hr, err := p2phost.BuildHostWithRuntime(ctx, r.opts)
	if err != nil {
		return fmt.Errorf("build host: %w", err)
	}
	r.host = hr.Host
	r.hostRuntime = hr

	if r.logger != nil {
		r.logger.Infof("P2P host started: id=%s addrs=%v", hr.Host.ID().String(), hr.Host.Addrs())

		// 打印一份关键 P2P 配置快照，便于排障与对比环境
		if r.opts != nil {
			r.logger.Infof(
				"p2p.runtime.config profile=%s dht_mode=%s enable_dht=%t enable_mdns=%t bootstrap_peers=%d min_peers=%d max_peers=%d discovery_interval=%s advertise_interval=%s discovery_namespace=%s enable_relay=%t enable_relay_service=%t enable_dcutr=%t enable_autorelay=%t static_relay_peers=%d autorelay_dynamic_candidates=%d enable_nat_port=%t force_reachability=%s enable_autonat_client=%t enable_autonat_service=%t",
				r.opts.Profile,
				r.opts.DHTMode,
				r.opts.EnableDHT,
				r.opts.EnableMDNS,
				len(r.opts.BootstrapPeers),
				r.opts.MinPeers,
				r.opts.MaxPeers,
				r.opts.DiscoveryInterval,
				r.opts.AdvertiseInterval,
				r.opts.DiscoveryNamespace,
				r.opts.EnableRelay,
				r.opts.EnableRelayService,
				r.opts.EnableDCUTR,
				r.opts.EnableAutoRelay,
				len(r.opts.StaticRelayPeers),
				r.opts.AutoRelayDynamicCandidates,
				r.opts.EnableNATPortMap,
				r.opts.ForceReachability,
				r.opts.EnableAutoNATClient,
				r.opts.EnableAutoNATService,
			)
		}
	}

	return nil
}

// Start 启动 P2P 运行时
//
// 供 Fx lifecycle 调用
func (r *Runtime) Start(ctx context.Context) error {
	if r.logger != nil {
		r.logger.Info("🚀 P2P runtime starting")
	}

	// 1. 确保 libp2p Host 已经构建
	if err := r.InitHost(ctx); err != nil {
		return err
	}

	// 2. 初始化各个子系统
	// Swarm：注入 BandwidthProvider（通过 hostRuntime.Runtime）
	var bwProvider interfaces.BandwidthProvider
	if r.hostRuntime != nil && r.hostRuntime.Runtime != nil {
		// host.Runtime 实现了 BandwidthProvider 接口
		bwProvider = r.hostRuntime.Runtime
	}
	r.swarm = swarm.NewService(r.host, bwProvider)

	// Routing
	dhtMode := p2pi.DHTMode(r.opts.DHTMode)
	if dhtMode == "" {
		dhtMode = p2pi.DHTModeAuto
	}
	routingSvc := routing.NewService(dhtMode)
	if err := routingSvc.Initialize(r.host, r.opts, r.logger); err != nil {
		if r.logger != nil {
			r.logger.Warnf("p2p.routing initialize failed: %v", err)
		}
		// Routing 初始化失败不阻断其他服务
	} else {
		// 执行 DHT Bootstrap
		if err := routingSvc.Bootstrap(ctx); err != nil {
			if r.logger != nil {
				r.logger.Warnf("p2p.routing bootstrap failed: %v", err)
			}
		}
	}
	r.routing = routingSvc

	// Discovery
	discoverySvc := discovery.NewService()
	// 设置实例数据目录（用于 AddrManager 存储路径）
	if r.configProvider != nil {
		instanceDataDir := r.configProvider.GetInstanceDataDir()
		discoverySvc.SetInstanceDataDir(instanceDataDir)
	}
	if err := discoverySvc.Initialize(r.host, r.opts, r.logger, r.eventBus); err != nil {
		return fmt.Errorf("initialize discovery: %w", err)
	}
	// 将 RendezvousRouting 能力注入到 Discovery（如果可用）
	if routingSvc != nil {
		// routing.Service 实现了 RendezvousRouting 接口
		discoverySvc.SetRendezvousRouting(routingSvc)
	}
	r.discovery = discoverySvc

	// Connectivity
	profile := p2pi.Profile(r.opts.Profile)
	if profile == "" {
		profile = p2pi.ProfileServer
	}
	connectivitySvc := connectivity.NewService(profile)
	connectivitySvc.Initialize(r.host, r.opts, r.logger)
	// 设置 ConnectionProtector（如果可用）
	if r.hostRuntime != nil && r.hostRuntime.Runtime != nil {
		if protector := r.hostRuntime.Runtime.GetConnectionProtector(); protector != nil {
			connectivitySvc.SetConnectionProtector(protector)
		}
	}
	r.connectivity = connectivitySvc

	// 启动 Connectivity 管理器（确保 Relay / DCUTR / AutoRelay 等高级连通能力真正生效）
	if connectivityStarter, ok := r.connectivity.(interface {
		Start(context.Context) error
	}); ok {
		if err := connectivityStarter.Start(ctx); err != nil {
			if r.logger != nil {
				r.logger.Warnf("p2p.connectivity start failed: %v", err)
			}
		}
	}

	// 启动 AutoNAT 客户端（如果启用）
	if r.opts != nil && r.opts.EnableAutoNATClient {
		autonatClient, err := startAutoNAT(r.host, r.opts)
		if err != nil {
			if r.logger != nil {
				r.logger.Warnf("p2p.connectivity start_autonat_client failed: %v", err)
			}
			// AutoNAT 客户端启动失败不阻断其他服务
		} else if autonatClient != nil {
			// 将 AutoNAT 客户端实例注入到 Connectivity Manager
			if connectivitySvc, ok := r.connectivity.(interface {
				SetAutoNATClient(client autonat.AutoNAT)
			}); ok {
				connectivitySvc.SetAutoNATClient(autonatClient)
			}
			if r.logger != nil {
				r.logger.Infof("p2p.connectivity autonat_client started")
			}
		}
	}

	// Diagnostics
	var diagnosticsSvc *diagnostics.Service
	if r.opts.DiagnosticsEnabled {
		diagnosticsSvc = diagnostics.NewService(r.opts.DiagnosticsAddr)
		// 获取共享带宽计数器（通过 BandwidthProvider 接口）
		var bwReporter metrics.Reporter
		if r.hostRuntime != nil && r.hostRuntime.Runtime != nil {
			// host.Runtime 实现了 BandwidthProvider 接口
			bwReporter = r.hostRuntime.Runtime.BandwidthReporter()
		} else {
			// 如果 hostRuntime 不可用，回退到全局函数（向后兼容）
			bwReporter = p2phost.GetBandwidthCounter()
		}
		diagnosticsSvc.Initialize(r.host, r.logger, bwReporter)
		// 设置配置提供者（用于获取 network_id）
		if r.configProvider != nil {
			diagnosticsSvc.SetConfigProvider(r.configProvider)
		}
		// 设置 P2P 配置选项（用于获取 Announce/Gater 规则）
		if r.opts != nil {
			diagnosticsSvc.SetP2POptions(r.opts)
		}
		// 设置子系统引用（用于健康检查和路由信息）
		diagnosticsSvc.SetSubsystems(routingSvc, connectivitySvc)
		// 注入 ResourceManagerInspector（通过接口，避免直接依赖 host 包）
		if r.hostRuntime != nil && r.hostRuntime.Runtime != nil {
			// host.Runtime 实现了 ResourceManagerInspector 接口
			diagnosticsSvc.SetResourceManagerInspector(r.hostRuntime.Runtime)
		}
		// 订阅 K桶摘要事件（用于 /debug/p2p/routing 输出“空桶风险/最近入桶原因”）
		if r.eventBus != nil {
			diagnosticsSvc.SubscribeKBucketSummary(r.eventBus)
			// 订阅自愈事件（用于 /debug/repair 输出“最近一次自愈动作/原因/结果”）
			diagnosticsSvc.SubscribeRepairEvents(r.eventBus)
		}
		// 启动诊断服务
		if err := diagnosticsSvc.Start(ctx); err != nil {
			if r.logger != nil {
				r.logger.Warnf("p2p.diagnostics start failed: %v", err)
			}
		}
	} else {
		diagnosticsSvc = diagnostics.NewService("")
	}
	r.diagnostics = diagnosticsSvc

	// 将 Diagnostics 回调注入到 Discovery（如果启用诊断）
	if diagnosticsSvc != nil && r.opts.DiagnosticsEnabled {
		discoverySvc.SetDiagnosticsCallbacks(
			diagnosticsSvc.RecordDiscoveryBootstrapAttempt,
			diagnosticsSvc.RecordDiscoveryBootstrapSuccess,
			diagnosticsSvc.RecordDiscoveryMDNSPeerFound,
			diagnosticsSvc.RecordDiscoveryMDNSConnectSuccess,
			diagnosticsSvc.RecordDiscoveryMDNSConnectFail,
			diagnosticsSvc.UpdateDiscoveryLastBootstrapTS,
			diagnosticsSvc.UpdateDiscoveryLastMDNSTS,
		)
	}

	// 3. 注册网络事件通知器（将 libp2p 网络事件桥接到 EventBus）
	if r.eventBus != nil && r.host != nil {
		p2phostpkg.RegisterNetworkEventNotifiee(r.host.Network(), r.eventBus, r.logger)
	}

	// 3.1 🆕 注册 WES 连接通知器（非 WES 节点降权/断开）
	// 背景：阿里云节点 Goroutine 峰值 34,832（19x 本地），核心原因是大量非 WES 节点涌入
	// 策略：
	// - WES 业务节点：设置正权重（+20），保护连接
	// - 非 WES 入站节点：设置负权重（-20），60 秒后断开
	// - 非 WES 出站节点：设置负权重（-10）
	// 参考：LIBP2P_GOROUTINE_ANALYSIS.md
	if r.host != nil {
		wesCfg := p2phostpkg.DefaultWESConnNotifeeConfig()
		// 可通过配置调整非 WES 节点超时时间
		// wesCfg.NonWESTimeout = 60 * time.Second
		p2phostpkg.RegisterWESConnNotifee(r.host, r.logger, wesCfg)
	}

	// 4. 启动 Discovery 服务
	if err := r.discovery.Start(ctx); err != nil {
		return fmt.Errorf("start discovery: %w", err)
	}

	// 5. 订阅 Hint 事件（网络质量变化时触发短促引导拨号）
	if r.eventBus != nil {
		if dsvc, ok := r.discovery.(interface {
			SubscribeHints(ctx context.Context, bus event.EventBus)
		}); ok {
			dsvc.SubscribeHints(ctx, r.eventBus)
		}
	}

	// 6. 启动 KeyPeerMonitor（关键peer保活）
	if r.opts.EnableKeyPeerMonitor {
		// 创建KeyPeerSet
		keyPeerSet := keepalive.NewKeyPeerSet(
			r.opts.KeyPeerSetMaxSize,
			10*time.Second, // usefulWindow可以配置
		)

		// 注入业务关键节点（Tier0）：用于“连接质量保活”，避免只靠公网海量libp2p节点维持连接数
		if r.opts != nil && len(r.opts.BusinessCriticalPeerIDs) > 0 {
			added := 0
			for _, s := range r.opts.BusinessCriticalPeerIDs {
				pid, err := peer.Decode(strings.TrimSpace(s))
				if err != nil || pid == "" {
					if r.logger != nil {
						r.logger.Warnf("invalid business critical peer id: %s", s)
					}
					continue
				}
				keyPeerSet.AddBusinessCritical(pid)
				added++
			}
			if added > 0 && r.logger != nil {
				r.logger.Infof("KeyPeerSet business critical peers loaded count=%d", added)
			}
		}
		
		// 获取AddrManager (从Discovery service)
		var addrManager *discovery.AddrManager
		if dsvc, ok := r.discovery.(*discovery.Service); ok {
			// 需要Discovery暴露GetAddrManager方法，暂时为nil
			_ = dsvc // 避免unused警告
			addrManager = nil
		}
		
		// 创建KeyPeerMonitor
		r.keepalive = keepalive.NewKeyPeerMonitor(
			r.host,
			r.routing,      // 实现了RendezvousRouting接口
			addrManager,    // AddrManager引用
			keyPeerSet,
			r.logger,
			r.eventBus,
			r.opts.KeyPeerProbeInterval,
			r.opts.PerPeerMinProbeInterval,
			r.opts.ProbeTimeout,
			r.opts.ProbeFailThreshold,
			r.opts.ProbeMaxConcurrent,
		)
		
		// 启动KeyPeerMonitor
		if err := r.keepalive.Start(); err != nil {
			if r.logger != nil {
				r.logger.Warnf("KeyPeerMonitor start failed: %v", err)
			}
			// 保活失败不阻断主服务
		} else {
			if r.logger != nil {
				r.logger.Info("✅ KeyPeerMonitor started")
			}
		}
	}

	if r.logger != nil {
		r.logger.Info("✅ P2P runtime started successfully")
	}

	return nil
}

// Stop 停止 P2P 运行时
//
// 优雅关闭 host 与子服务
func (r *Runtime) Stop(ctx context.Context) error {
	if r.logger != nil {
		r.logger.Info("🛑 P2P runtime stopping")
	}
	
	// 停止KeyPeerMonitor
	if r.keepalive != nil {
		if err := r.keepalive.Stop(); err != nil {
			if r.logger != nil {
				r.logger.Warnf("KeyPeerMonitor stop failed: %v", err)
			}
		}
	}

	// 停止 Diagnostics
	if r.diagnostics != nil {
		if diagSvc, ok := r.diagnostics.(interface{ Stop(context.Context) error }); ok {
			_ = diagSvc.Stop(ctx)
		}
	}

	// 停止 Discovery
	if r.discovery != nil {
		_ = r.discovery.Stop(ctx)
	}

	// 停止 Connectivity
	if r.connectivity != nil {
		if connectivitySvc, ok := r.connectivity.(interface{ Stop() error }); ok {
			_ = connectivitySvc.Stop()
		}
	}

	// 关闭 Host
	if r.host != nil {
		if err := r.host.Close(); err != nil {
			if r.logger != nil {
				r.logger.Warnf("close host error: %v", err)
			}
		}
		r.host = nil
	}

	if r.logger != nil {
		r.logger.Info("✅ P2P runtime stopped")
	}

	return nil
}

// ============= p2p.Service 实现 =============

func (r *Runtime) Host() lphost.Host {
	return r.host
}

func (r *Runtime) Swarm() p2pi.Swarm {
	return r.swarm
}

func (r *Runtime) Routing() p2pi.Routing {
	return r.routing
}

func (r *Runtime) Discovery() p2pi.Discovery {
	return r.discovery
}

func (r *Runtime) Connectivity() p2pi.Connectivity {
	return r.connectivity
}

func (r *Runtime) Diagnostics() p2pi.Diagnostics {
	return r.diagnostics
}

// startAutoNAT 在 Host 启动后启动 AutoNAT（按配置）
// 直接使用 p2pcfg.Options，不再依赖 nodeconfig.NodeOptions
func startAutoNAT(h lphost.Host, opts *p2pcfg.Options) (autonat.AutoNAT, error) {
	if h == nil {
		return nil, nil
	}
	// 仅当显式启用客户端时启动
	if opts != nil && opts.EnableAutoNATClient {
		an, err := autonat.New(h)
		return an, err
	}
	return nil, nil
}
