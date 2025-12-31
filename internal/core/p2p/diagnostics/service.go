package diagnostics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof" // pprof 性能分析端点
	"sync"
	"syscall"
	"time"

	lphost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	p2pcfg "github.com/weisyn/v1/internal/config/p2p"
	"github.com/weisyn/v1/internal/core/diagnostics"
	"github.com/weisyn/v1/internal/core/p2p/interfaces"
	transportpb "github.com/weisyn/v1/pb/network/transport"
	cfgprovider "github.com/weisyn/v1/pkg/interfaces/config"
	eventiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	"github.com/weisyn/v1/pkg/types"
)

// Service Diagnostics 服务实现
//
// 暴露 HTTP 诊断端点与 Prometheus 指标
type Service struct {
	httpAddr       string
	host           lphost.Host
	logger         logiface.Logger
	server         *http.Server
	registry       *prometheus.Registry
	bwReporter     metrics.Reporter
	configProvider cfgprovider.Provider // 配置提供者，用于获取 network_id
	p2pOpts        interface {          // P2P 配置选项（用于获取 Announce/Gater/Bootstrap 规则）
		GetBootstrapPeers() []string
		GetAnnounce() []string
		GetAppendAnnounce() []string
		GetNoAnnounce() []string
		GetGaterAllowedPrefixes() []string
		GetGaterBlockedPrefixes() []string
	} `optional:"true"` // 可选，如果未设置则返回空配置

	// 子系统引用（用于健康检查和路由信息）
	routing      p2pi.Routing
	connectivity p2pi.Connectivity

	// ResourceManager 检查器（通过接口注入，避免直接依赖 host 包）
	rmInspector interfaces.ResourceManagerInspector

	// K桶摘要（由 KBucket 模块通过 EventBus 推送，供 /debug/p2p/routing 展示）
	kbucketMu      sync.RWMutex
	kbucketSummary *types.KBucketSummary

	// 自愈/损坏事件摘要（由各模块通过 EventBus 推送，供 /debug/repair 展示）
	repairMu            sync.RWMutex
	lastCorruption      *types.CorruptionEventData
	lastRepairResult    *types.CorruptionRepairEventData
	recentCorruptions   []types.CorruptionEventData
	recentRepairResults []types.CorruptionRepairEventData

	// Discovery 指标
	discoveryBootstrapAttempts  prometheus.Counter
	discoveryBootstrapSuccess   prometheus.Counter
	discoveryMDNSPeerFound      prometheus.Counter
	discoveryMDNSConnectSuccess prometheus.Counter
	discoveryMDNSConnectFail    prometheus.Counter
	discoveryLastBootstrapTS    prometheus.Gauge
	discoveryLastMDNSTS         prometheus.Gauge

	// P3-005: 新增关键监控指标
	kbucketHealthScore      prometheus.GaugeFunc // K桶健康评分 (0-100)
	connectionQualityScore  prometheus.GaugeFunc // 连接质量评分 (0-100)
}

var _ p2pi.Diagnostics = (*Service)(nil)

// NewService 创建 Diagnostics 服务
func NewService(httpAddr string) *Service {
	return &Service{
		httpAddr: httpAddr,
		registry: prometheus.NewRegistry(),
	}
}

// Initialize 初始化 Diagnostics 服务
func (s *Service) Initialize(host lphost.Host, logger logiface.Logger, bwReporter metrics.Reporter) {
	s.host = host
	s.logger = logger
	s.bwReporter = bwReporter

	// 注册 Prometheus 指标
	s.registerMetrics()
}

// SetConfigProvider 设置配置提供者（用于获取 network_id）
func (s *Service) SetConfigProvider(provider cfgprovider.Provider) {
	s.configProvider = provider
}

// SetP2POptions 设置 P2P 配置选项（用于获取 Announce/Gater/Bootstrap 规则）
func (s *Service) SetP2POptions(opts interface {
	GetBootstrapPeers() []string
	GetAnnounce() []string
	GetAppendAnnounce() []string
	GetNoAnnounce() []string
	GetGaterAllowedPrefixes() []string
	GetGaterBlockedPrefixes() []string
}) {
	s.p2pOpts = opts
}

// SetSubsystems 设置子系统引用（由 Runtime 调用）
func (s *Service) SetSubsystems(routing p2pi.Routing, connectivity p2pi.Connectivity) {
	s.routing = routing
	s.connectivity = connectivity
}

// SetResourceManagerInspector 设置 ResourceManager 检查器（由 Runtime 调用）
//
// 通过接口注入，避免直接依赖 host 包
func (s *Service) SetResourceManagerInspector(inspector interfaces.ResourceManagerInspector) {
	s.rmInspector = inspector
}

// SubscribeKBucketSummary 订阅 K桶摘要事件（由 Runtime 调用）
func (s *Service) SubscribeKBucketSummary(bus eventiface.EventBus) {
	if bus == nil {
		return
	}
	_ = bus.Subscribe(eventiface.EventTypeKBucketSummaryUpdated, func(ctx context.Context, data interface{}) error {
		summary, ok := data.(types.KBucketSummary)
		if !ok {
			return nil
		}
		s.kbucketMu.Lock()
		s.kbucketSummary = &summary
		s.kbucketMu.Unlock()
		return nil
	})
}

// SubscribeRepairEvents 订阅自愈/损坏事件（由 Runtime 调用）
func (s *Service) SubscribeRepairEvents(bus eventiface.EventBus) {
	if bus == nil {
		return
	}
	_ = bus.Subscribe(eventiface.EventTypeCorruptionDetected, func(ctx context.Context, data interface{}) error {
		ev, ok := data.(types.CorruptionEventData)
		if !ok {
			if p, ok2 := data.(*types.CorruptionEventData); ok2 && p != nil {
				ev = *p
				ok = true
			}
		}
		if !ok {
			return nil
		}
		s.repairMu.Lock()
		s.lastCorruption = &ev
		s.recentCorruptions = append(s.recentCorruptions, ev)
		if len(s.recentCorruptions) > 50 {
			s.recentCorruptions = s.recentCorruptions[len(s.recentCorruptions)-50:]
		}
		s.repairMu.Unlock()
		return nil
	})
	onRepair := func(ctx context.Context, data interface{}) error {
		ev, ok := data.(types.CorruptionRepairEventData)
		if !ok {
			if p, ok2 := data.(*types.CorruptionRepairEventData); ok2 && p != nil {
				ev = *p
				ok = true
			}
		}
		if !ok {
			return nil
		}
		s.repairMu.Lock()
		s.lastRepairResult = &ev
		s.recentRepairResults = append(s.recentRepairResults, ev)
		if len(s.recentRepairResults) > 50 {
			s.recentRepairResults = s.recentRepairResults[len(s.recentRepairResults)-50:]
		}
		s.repairMu.Unlock()
		return nil
	}
	_ = bus.Subscribe(eventiface.EventTypeCorruptionRepaired, onRepair)
	_ = bus.Subscribe(eventiface.EventTypeCorruptionRepairFailed, onRepair)
}

// Start 启动诊断 HTTP 服务
func (s *Service) Start(ctx context.Context) error {
	if s.httpAddr == "" || s.host == nil {
		// 未启用诊断服务
		return nil
	}

	// 先创建 listener，避免 ListenAndServe 在 goroutine 中失败却仍输出“已启动”日志
	listener, err := net.Listen("tcp", s.httpAddr)
	if err != nil {
		if s.logger != nil {
			// diagnostics 不是关键路径：端口被占用时降级为禁用诊断服务，避免影响节点启动
			if errors.Is(err, syscall.EADDRINUSE) {
				s.logger.Warnf("diagnostics server disabled (addr already in use): %s", s.httpAddr)
			} else {
				s.logger.Warnf("diagnostics server disabled (failed to listen on %s): %v", s.httpAddr, err)
			}
		}
		return nil
	}

	mux := http.NewServeMux()

	// Prometheus 指标端点
	mux.Handle("/metrics", promhttp.HandlerFor(s.registry, promhttp.HandlerOpts{}))

	// pprof 性能分析端点（L4 能力：代码级分析）
	// 使用标准库 net/http/pprof 提供的处理器
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	// 支持通过 URL 参数访问不同类型的 profile（如 /debug/pprof/heap, /debug/pprof/goroutine）
	mux.HandleFunc("/debug/pprof/heap", pprof.Handler("heap").ServeHTTP)
	mux.HandleFunc("/debug/pprof/goroutine", pprof.Handler("goroutine").ServeHTTP)
	mux.HandleFunc("/debug/pprof/allocs", pprof.Handler("allocs").ServeHTTP)
	mux.HandleFunc("/debug/pprof/block", pprof.Handler("block").ServeHTTP)
	mux.HandleFunc("/debug/pprof/mutex", pprof.Handler("mutex").ServeHTTP)

	// 诊断端点（JSON）
	mux.HandleFunc("/debug/p2p/peers", s.handlePeers)
	mux.HandleFunc("/debug/p2p/connections", s.handleConnections)
	mux.HandleFunc("/debug/p2p/stats", s.handleStats)
	mux.HandleFunc("/debug/p2p/health", s.handleHealth)
	mux.HandleFunc("/debug/p2p/routing", s.handleRouting)
	mux.HandleFunc("/debug/p2p/host", s.handleHost)
	// 自愈摘要
	mux.HandleFunc("/debug/repair", s.handleRepair)

	// 🆕 内存分析端点（来自 diagnostics 包）
	diagnostics.RegisterMemoryHandlers(mux)

	// PB 诊断端点
	mux.HandleFunc("/debug/p2p/host.pb", s.handleHostProtobuf)
	mux.HandleFunc("/debug/p2p/host.json", s.handleHostJSON)
	mux.HandleFunc("/debug/p2p/peers.pb", s.handlePeersProtobuf)
	mux.HandleFunc("/debug/p2p/peers.json", s.handlePeersJSON)
	mux.HandleFunc("/debug/p2p/connections.pb", s.handleConnectionsProtobuf)
	mux.HandleFunc("/debug/p2p/connections.json", s.handleConnectionsJSON)

	s.server = &http.Server{
		Addr:         s.httpAddr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			if s.logger != nil {
				s.logger.Errorf("diagnostics server error: %v", err)
			}
		}
	}()

	if s.logger != nil {
		s.logger.Infof("p2p.diagnostics server started on %s", s.httpAddr)
		s.logger.Infof("pprof endpoints available at http://%s/debug/pprof/", s.httpAddr)
	}

	return nil
}

// handleRepair 处理 /debug/repair 端点（自运行系统“最近一次自愈动作/原因/结果”一眼可见）
func (s *Service) handleRepair(w http.ResponseWriter, r *http.Request) {
	s.repairMu.RLock()
	lastCorruption := s.lastCorruption
	lastRepair := s.lastRepairResult
	recentCorruptions := append([]types.CorruptionEventData(nil), s.recentCorruptions...)
	recentRepairs := append([]types.CorruptionRepairEventData(nil), s.recentRepairResults...)
	s.repairMu.RUnlock()

	resp := map[string]interface{}{
		"last_corruption": lastCorruption,
		"last_repair":     lastRepair,
		"recent": map[string]interface{}{
			"corruptions": recentCorruptions,
			"repairs":     recentRepairs,
		},
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(resp)
}

// Stop 停止诊断 HTTP 服务
func (s *Service) Stop(ctx context.Context) error {
	if s.server != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := s.server.Shutdown(shutdownCtx); err != nil {
			if s.logger != nil {
				s.logger.Warnf("diagnostics server shutdown error: %v", err)
			}
			return err
		}
		s.server = nil
	}

	if s.logger != nil {
		s.logger.Infof("p2p.diagnostics server stopped")
	}

	return nil
}

// HTTPAddr 返回诊断 HTTP 服务地址
func (s *Service) HTTPAddr() string {
	return s.httpAddr
}

// GetPeersCount 返回当前连接的 peers 数量
func (s *Service) GetPeersCount() int {
	if s.host == nil {
		return 0
	}
	return len(s.host.Network().Peers())
}

// GetConnectionsCount 返回当前活跃连接数
func (s *Service) GetConnectionsCount() int {
	if s.host == nil {
		return 0
	}
	return len(s.host.Network().Conns())
}

// registerMetrics 注册 Prometheus 指标
func (s *Service) registerMetrics() {
	// 连接数指标
	connectionsTotal := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "p2p_connections_total",
		Help: "Current number of P2P connections",
	}, func() float64 {
		if s.host == nil {
			return 0
		}
		return float64(len(s.host.Network().Conns()))
	})

	peersTotal := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "p2p_peers_total",
		Help: "Current number of connected peers",
	}, func() float64 {
		if s.host == nil {
			return 0
		}
		return float64(len(s.host.Network().Peers()))
	})

	// 带宽指标
	bandwidthIn := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "p2p_bandwidth_in_rate_bytes_per_sec",
		Help: "Inbound bandwidth rate in bytes per second",
	}, func() float64 {
		if s.bwReporter != nil {
			if bwCounter, ok := s.bwReporter.(*metrics.BandwidthCounter); ok {
				totals := bwCounter.GetBandwidthTotals()
				return float64(totals.RateIn)
			}
		}
		return 0
	})

	bandwidthOut := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "p2p_bandwidth_out_rate_bytes_per_sec",
		Help: "Outbound bandwidth rate in bytes per second",
	}, func() float64 {
		if s.bwReporter != nil {
			if bwCounter, ok := s.bwReporter.(*metrics.BandwidthCounter); ok {
				totals := bwCounter.GetBandwidthTotals()
				return float64(totals.RateOut)
			}
		}
		return 0
	})

	bandwidthInTotal := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "p2p_bandwidth_in_total_bytes",
		Help: "Total inbound bandwidth in bytes",
	}, func() float64 {
		if s.bwReporter != nil {
			if bwCounter, ok := s.bwReporter.(*metrics.BandwidthCounter); ok {
				totals := bwCounter.GetBandwidthTotals()
				return float64(totals.TotalIn)
			}
		}
		return 0
	})

	bandwidthOutTotal := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "p2p_bandwidth_out_total_bytes",
		Help: "Total outbound bandwidth in bytes",
	}, func() float64 {
		if s.bwReporter != nil {
			if bwCounter, ok := s.bwReporter.(*metrics.BandwidthCounter); ok {
				totals := bwCounter.GetBandwidthTotals()
				return float64(totals.TotalOut)
			}
		}
		return 0
	})

	// Discovery 指标
	s.discoveryBootstrapAttempts = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "p2p_discovery_bootstrap_attempt_total",
		Help: "Total bootstrap attempts",
	})

	s.discoveryBootstrapSuccess = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "p2p_discovery_bootstrap_success_total",
		Help: "Successful bootstrap attempts",
	})

	s.discoveryMDNSPeerFound = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "p2p_discovery_mdns_peer_found_total",
		Help: "MDNS peers discovered",
	})

	s.discoveryMDNSConnectSuccess = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "p2p_discovery_mdns_connect_success_total",
		Help: "Successful MDNS connections",
	})

	s.discoveryMDNSConnectFail = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "p2p_discovery_mdns_connect_fail_total",
		Help: "Failed MDNS connections",
	})

	s.discoveryLastBootstrapTS = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "p2p_discovery_last_bootstrap_unixtime",
		Help: "Last bootstrap timestamp",
	})

	s.discoveryLastMDNSTS = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "p2p_discovery_last_mdns_found_unixtime",
		Help: "Last MDNS discovery timestamp",
	})

	// P3-005: K桶健康评分 (0-100)
	// 计算公式: (healthyPeers / totalPeers) * 100，如果 totalPeers=0 则返回 0
	s.kbucketHealthScore = prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "wes",
		Subsystem: "p2p",
		Name:      "kbucket_health_score",
		Help:      "K-bucket routing table health score (0-100), calculated as (healthy_peers / total_peers) * 100.",
	}, func() float64 {
		s.kbucketMu.RLock()
		defer s.kbucketMu.RUnlock()
		if s.kbucketSummary == nil || s.kbucketSummary.TotalPeers == 0 {
			return 0
		}
		return float64(s.kbucketSummary.HealthyPeers) / float64(s.kbucketSummary.TotalPeers) * 100
	})

	// P3-005: 连接质量评分 (0-100)
	// 基于当前连接数与 peers 数的比例，满分为每个 peer 至少 1 个连接
	s.connectionQualityScore = prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "wes",
		Subsystem: "p2p",
		Name:      "connection_quality_score",
		Help:      "Connection quality score (0-100), calculated based on connection/peer ratio and bandwidth availability.",
	}, func() float64 {
		if s.host == nil {
			return 0
		}
		peers := len(s.host.Network().Peers())
		conns := len(s.host.Network().Conns())
		if peers == 0 {
			return 0
		}
		// 基础分: 连接数/peer数 比例（上限100）
		ratio := float64(conns) / float64(peers)
		if ratio > 1 {
			ratio = 1
		}
		baseScore := ratio * 80 // 基础分占 80%

		// 带宽加分: 如果有带宽数据则加分
		bandwidthBonus := 0.0
		if s.bwReporter != nil {
			bandwidthBonus = 20 // 有带宽监控则加 20 分
		}

		return baseScore + bandwidthBonus
	})

	s.registry.MustRegister(
		connectionsTotal,
		peersTotal,
		bandwidthIn,
		bandwidthOut,
		bandwidthInTotal,
		bandwidthOutTotal,
		s.discoveryBootstrapAttempts,
		s.discoveryBootstrapSuccess,
		s.discoveryMDNSPeerFound,
		s.discoveryMDNSConnectSuccess,
		s.discoveryMDNSConnectFail,
		s.discoveryLastBootstrapTS,
		s.discoveryLastMDNSTS,
		s.kbucketHealthScore,
		s.connectionQualityScore,
	)
}

// RecordDiscoveryBootstrapAttempt 记录 Bootstrap 尝试
func (s *Service) RecordDiscoveryBootstrapAttempt() {
	if s.discoveryBootstrapAttempts != nil {
		s.discoveryBootstrapAttempts.Inc()
	}
}

// RecordDiscoveryBootstrapSuccess 记录 Bootstrap 成功
func (s *Service) RecordDiscoveryBootstrapSuccess() {
	if s.discoveryBootstrapSuccess != nil {
		s.discoveryBootstrapSuccess.Inc()
	}
}

// RecordDiscoveryMDNSPeerFound 记录 mDNS 发现的 Peer
func (s *Service) RecordDiscoveryMDNSPeerFound() {
	if s.discoveryMDNSPeerFound != nil {
		s.discoveryMDNSPeerFound.Inc()
	}
}

// RecordDiscoveryMDNSConnectSuccess 记录 mDNS 连接成功
func (s *Service) RecordDiscoveryMDNSConnectSuccess() {
	if s.discoveryMDNSConnectSuccess != nil {
		s.discoveryMDNSConnectSuccess.Inc()
	}
}

// RecordDiscoveryMDNSConnectFail 记录 mDNS 连接失败
func (s *Service) RecordDiscoveryMDNSConnectFail() {
	if s.discoveryMDNSConnectFail != nil {
		s.discoveryMDNSConnectFail.Inc()
	}
}

// UpdateDiscoveryLastBootstrapTS 更新最后 Bootstrap 时间戳
func (s *Service) UpdateDiscoveryLastBootstrapTS() {
	if s.discoveryLastBootstrapTS != nil {
		s.discoveryLastBootstrapTS.Set(float64(time.Now().Unix()))
	}
}

// UpdateDiscoveryLastMDNSTS 更新最后 mDNS 发现时间戳
func (s *Service) UpdateDiscoveryLastMDNSTS() {
	if s.discoveryLastMDNSTS != nil {
		s.discoveryLastMDNSTS.Set(float64(time.Now().Unix()))
	}
}

// handlePeers 处理 /debug/p2p/peers 端点
func (s *Service) handlePeers(w http.ResponseWriter, r *http.Request) {
	if s.host == nil {
		http.Error(w, "host not available", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"peers": %d, "peer_ids": [`, len(s.host.Network().Peers()))

	first := true
	for _, peerID := range s.host.Network().Peers() {
		if !first {
			fmt.Fprint(w, ", ")
		}
		fmt.Fprintf(w, `"%s"`, peerID.String())
		first = false
	}
	fmt.Fprint(w, "]}")
}

// handleConnections 处理 /debug/p2p/connections 端点
func (s *Service) handleConnections(w http.ResponseWriter, r *http.Request) {
	if s.host == nil {
		http.Error(w, "host not available", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"connections": %d}`, len(s.host.Network().Conns()))
}

// handleStats 处理 /debug/p2p/stats 端点
func (s *Service) handleStats(w http.ResponseWriter, r *http.Request) {
	if s.host == nil {
		http.Error(w, "host not available", http.StatusServiceUnavailable)
		return
	}

	network := s.host.Network()
	peers := len(network.Peers())
	connections := len(network.Conns())
	hostID := s.host.ID().String()

	// 获取带宽统计
	var bandwidthInRate, bandwidthOutRate, bandwidthInTotal, bandwidthOutTotal float64
	if s.bwReporter != nil {
		if bwCounter, ok := s.bwReporter.(*metrics.BandwidthCounter); ok {
			totals := bwCounter.GetBandwidthTotals()
			bandwidthInRate = float64(totals.RateIn)
			bandwidthOutRate = float64(totals.RateOut)
			bandwidthInTotal = float64(totals.TotalIn)
			bandwidthOutTotal = float64(totals.TotalOut)
		}
	}

	// 获取 ResourceManager 限额
	rcmgrLimits := s.getResourceManagerLimits()

	// 获取 network_id
	networkID := s.getNetworkID()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{
		"peers": %d,
		"connections": %d,
		"host_id": "%s",
		"network_id": "%s",
		"bandwidth": {
			"in_rate_bps": %.2f,
			"out_rate_bps": %.2f,
			"in_total_bytes": %.0f,
			"out_total_bytes": %.0f
		},
		"resource_limits": %s
	}`,
		peers, connections, hostID, networkID,
		bandwidthInRate, bandwidthOutRate,
		bandwidthInTotal, bandwidthOutTotal,
		rcmgrLimits)
}

// handleHealth 处理 /debug/p2p/health 端点
func (s *Service) handleHealth(w http.ResponseWriter, r *http.Request) {
	if s.host == nil {
		http.Error(w, "host not available", http.StatusServiceUnavailable)
		return
	}

	network := s.host.Network()
	peers := len(network.Peers())
	connections := len(network.Conns())
	hostID := s.host.ID().String()
	networkID := s.getNetworkID()

	// 获取连通性状态
	reachability := "unknown"
	autoNATStatus := "unknown"
	relayStats := map[string]interface{}{
		"enabled": false,
	}
	protectionStats := map[string]interface{}{}

	if s.connectivity != nil {
		reachability = string(s.connectivity.Reachability())
		// 从 Connectivity Service 获取完整 Stats
		if connectivitySvc, ok := s.connectivity.(interface{ StatsMap() map[string]interface{} }); ok {
			stats := connectivitySvc.StatsMap()
			if relay, ok := stats["relay_enabled"].(bool); ok {
				relayStats["enabled"] = relay
			}
			if relayActive, ok := stats["relay_active"].(bool); ok {
				relayStats["active"] = relayActive
			}
			if holepunch, ok := stats["holepunch_enabled"].(bool); ok {
				relayStats["holepunch_enabled"] = holepunch
			}
			if autorelay, ok := stats["autorelay_enabled"].(bool); ok {
				relayStats["autorelay_enabled"] = autorelay
			}
			if autonat, ok := stats["autoNAT_status"].(string); ok {
				autoNATStatus = autonat
			}
			if allowedPeers, ok := stats["allowed_peers"].(int); ok {
				protectionStats["allowed_peers"] = allowedPeers
			}
			if blockedPeers, ok := stats["blocked_peers"].(int); ok {
				protectionStats["blocked_peers"] = blockedPeers
			}
		}
	}

	// 格式化 JSON 输出
	relayJSON := fmt.Sprintf(`{"enabled": %t`, relayStats["enabled"].(bool))
	if active, ok := relayStats["active"].(bool); ok {
		relayJSON += fmt.Sprintf(`, "active": %t`, active)
	}
	if holepunch, ok := relayStats["holepunch_enabled"].(bool); ok {
		relayJSON += fmt.Sprintf(`, "holepunch_enabled": %t`, holepunch)
	}
	if autorelay, ok := relayStats["autorelay_enabled"].(bool); ok {
		relayJSON += fmt.Sprintf(`, "autorelay_enabled": %t`, autorelay)
	}
	relayJSON += "}"

	protectionJSON := "{}"
	if len(protectionStats) > 0 {
		protectionJSON = fmt.Sprintf(`{"allowed_peers": %d, "blocked_peers": %d}`,
			protectionStats["allowed_peers"], protectionStats["blocked_peers"])
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{
		"host_id": "%s",
		"network_id": "%s",
		"num_peers": %d,
		"num_conns": %d,
		"reachability": "%s",
		"autoNAT_status": "%s",
		"relay_stats": %s,
		"protection": %s
	}`,
		hostID, networkID, peers, connections,
		reachability, autoNATStatus,
		relayJSON, protectionJSON)
}

// handleRouting 处理 /debug/p2p/routing 端点
func (s *Service) handleRouting(w http.ResponseWriter, r *http.Request) {
	if s.host == nil {
		http.Error(w, "host not available", http.StatusServiceUnavailable)
		return
	}

	// 路由表信息
	routingTableSize := 0
	mode := "unknown"
	numBootstrapPeers := 0
	offline := false

	if s.routing != nil {
		mode = string(s.routing.Mode())

		// 通过 RendezvousRouting 接口获取离线状态和 DHT 路由表大小（如果 Routing Service 支持）
		if rr, ok := s.routing.(interfaces.RendezvousRouting); ok {
			offline = rr.Offline()
			routingTableSize = rr.RoutingTableSize()
		}
	}

	// 从 P2P 配置中获取 BootstrapPeers 数量（如果可用）
	if s.p2pOpts != nil {
		if peers := s.p2pOpts.GetBootstrapPeers(); len(peers) > 0 {
			numBootstrapPeers = len(peers)
		}
	}

	resp := map[string]interface{}{
		"routing_table_size":    routingTableSize,
		"mode":                 mode,
		"offline":              offline,
		"num_bootstrap_peers":  numBootstrapPeers,
	}

	// 附带 K桶摘要（如果已收到）
	s.kbucketMu.RLock()
	if s.kbucketSummary != nil {
		resp["kbucket"] = s.kbucketSummary
		resp["kbucket_empty_risk"] = (s.kbucketSummary.HealthyPeers == 0)
	}
	s.kbucketMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// getNetworkID 获取网络 ID（格式：networkNamespace:chainID）
func (s *Service) getNetworkID() string {
	var networkNamespace string = "mainnet" // 默认值
	var chainID string = "1"                // 默认值

	if s.configProvider != nil {
		// 获取网络命名空间
		networkNamespace = s.configProvider.GetNetworkNamespace()

		// 从 AppConfig 获取链 ID
		appConfig := s.configProvider.GetAppConfig()
		if appConfig != nil && appConfig.Network != nil && appConfig.Network.ChainID != nil {
			chainID = fmt.Sprintf("%d", *appConfig.Network.ChainID)
		}
	}

	return networkNamespace + ":" + chainID
}

// getResourceManagerLimits 获取 ResourceManager 限额信息（JSON 字符串）
//
// 通过 ResourceManagerInspector 接口获取，避免直接依赖 host 包
func (s *Service) getResourceManagerLimits() string {
	if s.rmInspector == nil {
		return "{}"
	}

	data := s.rmInspector.ResourceManagerLimits()
	if data == nil {
		return "{}"
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return `{"enabled": true, "error": "failed to marshal limits"}`
	}
	return string(jsonBytes)
}

// handleHost 处理 /debug/p2p/host 端点（展示 Host 配置摘要）
func (s *Service) handleHost(w http.ResponseWriter, r *http.Request) {
	if s.host == nil {
		http.Error(w, "host not available", http.StatusServiceUnavailable)
		return
	}

	hostID := s.host.ID().String()
	networkID := s.getNetworkID()

	// 获取地址信息
	addrs := s.host.Addrs()
	addrStrings := make([]string, len(addrs))
	for i, addr := range addrs {
		addrStrings[i] = addr.String()
	}

	// 获取协议列表
	protocolIDs := s.host.Mux().Protocols()
	protocolStrings := make([]string, len(protocolIDs))
	for i, pid := range protocolIDs {
		protocolStrings[i] = string(pid)
	}

	// 获取配置摘要（Announce/Gater 规则）
	configSummary := s.getConfigSummary()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// 构建 JSON 响应
	addrJSON, _ := json.Marshal(addrStrings)
	protocolJSON, _ := json.Marshal(protocolStrings)
	configJSON, _ := json.Marshal(configSummary)

	fmt.Fprintf(w, `{
		"id": "%s",
		"network_id": "%s",
		"addresses": %s,
		"protocols": %s,
		"config": %s
	}`,
		hostID, networkID,
		string(addrJSON), string(protocolJSON), string(configJSON))
}

// getConfigSummary 获取配置摘要（Announce/Gater/NAT/Reachability/AutoNAT 规则）
func (s *Service) getConfigSummary() map[string]interface{} {
	summary := map[string]interface{}{
		"announce":               []string{},
		"append_announce":        []string{},
		"no_announce":            []string{},
		"gater_allowed_prefixes": []string{},
		"gater_blocked_prefixes": []string{},
		"nat_port_map":           false,
		"force_reachability":     "",
		"autonat_client":         false,
		"autonat_service":        false,
	}

	// 从 p2pOpts 获取配置
	if s.p2pOpts != nil {
		if announce := s.p2pOpts.GetAnnounce(); len(announce) > 0 {
			summary["announce"] = announce
		}
		if appendAnnounce := s.p2pOpts.GetAppendAnnounce(); len(appendAnnounce) > 0 {
			summary["append_announce"] = appendAnnounce
		}
		if noAnnounce := s.p2pOpts.GetNoAnnounce(); len(noAnnounce) > 0 {
			summary["no_announce"] = noAnnounce
		}
		if allowedPrefixes := s.p2pOpts.GetGaterAllowedPrefixes(); len(allowedPrefixes) > 0 {
			summary["gater_allowed_prefixes"] = allowedPrefixes
		}
		if blockedPrefixes := s.p2pOpts.GetGaterBlockedPrefixes(); len(blockedPrefixes) > 0 {
			summary["gater_blocked_prefixes"] = blockedPrefixes
		}

		// 尝试类型断言获取 NAT/Reachability/AutoNAT 配置
		if opts, ok := s.p2pOpts.(*p2pcfg.Options); ok {
			summary["nat_port_map"] = opts.EnableNATPortMap
			summary["force_reachability"] = opts.ForceReachability
			summary["autonat_client"] = opts.EnableAutoNATClient
			summary["autonat_service"] = opts.EnableAutoNATService
		}
	}

	return summary
}

// handleHostProtobuf 处理 /debug/p2p/host.pb 端点（PB 格式）
func (s *Service) handleHostProtobuf(w http.ResponseWriter, r *http.Request) {
	if s.host == nil {
		http.Error(w, "host not available", http.StatusServiceUnavailable)
		return
	}

	addrs := s.host.Addrs()
	addrStrings := make([]string, len(addrs))
	for i, addr := range addrs {
		addrStrings[i] = addr.String()
	}

	protocolIDs := s.host.Mux().Protocols()
	protocolStrings := make([]string, len(protocolIDs))
	for i, pid := range protocolIDs {
		protocolStrings[i] = string(pid)
	}

	nodeInfo := &transportpb.NodeInfo{
		Id:        s.host.ID().String(),
		Addresses: addrStrings,
		Protocols: protocolStrings,
		NetworkId: []byte(s.getNetworkID()),
	}

	data, err := proto.Marshal(nodeInfo)
	if err != nil {
		http.Error(w, "Serialization failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-protobuf")
	if _, err := w.Write(data); err != nil {
		return
	}
}

// handleHostJSON 处理 /debug/p2p/host.json 端点（PB->JSON 格式）
func (s *Service) handleHostJSON(w http.ResponseWriter, r *http.Request) {
	if s.host == nil {
		http.Error(w, "host not available", http.StatusServiceUnavailable)
		return
	}

	addrs := s.host.Addrs()
	addrStrings := make([]string, len(addrs))
	for i, addr := range addrs {
		addrStrings[i] = addr.String()
	}

	protocolIDs := s.host.Mux().Protocols()
	protocolStrings := make([]string, len(protocolIDs))
	for i, pid := range protocolIDs {
		protocolStrings[i] = string(pid)
	}

	nodeInfo := &transportpb.NodeInfo{
		Id:        s.host.ID().String(),
		Addresses: addrStrings,
		Protocols: protocolStrings,
		NetworkId: []byte(s.getNetworkID()),
	}

	marshaler := protojson.MarshalOptions{EmitUnpopulated: true, Indent: "  "}
	jsonData, err := marshaler.Marshal(nodeInfo)
	if err != nil {
		http.Error(w, "JSON conversion failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(jsonData); err != nil {
		return
	}
}

// handlePeersProtobuf 处理 /debug/p2p/peers.pb 端点（PB 格式）
func (s *Service) handlePeersProtobuf(w http.ResponseWriter, r *http.Request) {
	if s.host == nil {
		http.Error(w, "host not available", http.StatusServiceUnavailable)
		return
	}

	peerList := s.buildPeerListResponse()

	data, err := proto.Marshal(peerList)
	if err != nil {
		http.Error(w, "Serialization failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-protobuf")
	if _, err := w.Write(data); err != nil {
		return
	}
}

// handlePeersJSON 处理 /debug/p2p/peers.json 端点（PB->JSON 格式）
func (s *Service) handlePeersJSON(w http.ResponseWriter, r *http.Request) {
	if s.host == nil {
		http.Error(w, "host not available", http.StatusServiceUnavailable)
		return
	}

	peerList := s.buildPeerListResponse()

	marshaler := protojson.MarshalOptions{EmitUnpopulated: true, Indent: "  "}
	jsonData, err := marshaler.Marshal(peerList)
	if err != nil {
		http.Error(w, "JSON conversion failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(jsonData); err != nil {
		return
	}
}

// handleConnectionsProtobuf 处理 /debug/p2p/connections.pb 端点（PB 格式）
func (s *Service) handleConnectionsProtobuf(w http.ResponseWriter, r *http.Request) {
	if s.host == nil {
		http.Error(w, "host not available", http.StatusServiceUnavailable)
		return
	}

	conns := s.host.Network().Conns()
	peerList := &transportpb.PeerListResponse{
		TotalPeers: int32(len(conns)),
	}

	for _, conn := range conns {
		peerInfo := &transportpb.PeerInfo{
			Id:            conn.RemotePeer().String(),
			Addresses:     []string{conn.RemoteMultiaddr().String()},
			Direction:     conn.Stat().Direction.String(),
			ConnectedTime: uint64(conn.Stat().Opened.Unix()),
		}
		peerList.Peers = append(peerList.Peers, peerInfo)
	}

	data, err := proto.Marshal(peerList)
	if err != nil {
		http.Error(w, "Serialization failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-protobuf")
	if _, err := w.Write(data); err != nil {
		return
	}
}

// handleConnectionsJSON 处理 /debug/p2p/connections.json 端点（PB->JSON 格式）
func (s *Service) handleConnectionsJSON(w http.ResponseWriter, r *http.Request) {
	if s.host == nil {
		http.Error(w, "host not available", http.StatusServiceUnavailable)
		return
	}

	conns := s.host.Network().Conns()
	peerList := &transportpb.PeerListResponse{
		TotalPeers: int32(len(conns)),
	}

	for _, conn := range conns {
		peerInfo := &transportpb.PeerInfo{
			Id:            conn.RemotePeer().String(),
			Addresses:     []string{conn.RemoteMultiaddr().String()},
			Direction:     conn.Stat().Direction.String(),
			ConnectedTime: uint64(conn.Stat().Opened.Unix()),
		}
		peerList.Peers = append(peerList.Peers, peerInfo)
	}

	marshaler := protojson.MarshalOptions{EmitUnpopulated: true, Indent: "  "}
	jsonData, err := marshaler.Marshal(peerList)
	if err != nil {
		http.Error(w, "JSON conversion failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(jsonData); err != nil {
		return
	}
}

// buildPeerListResponse 构建 PeerListResponse（用于 PB 端点）
func (s *Service) buildPeerListResponse() *transportpb.PeerListResponse {
	peers := s.host.Network().Peers()
	peerList := &transportpb.PeerListResponse{
		TotalPeers: int32(len(peers)),
		Self: &transportpb.NodeInfo{
			Id: s.host.ID().String(),
			Addresses: func() []string {
				addrs := s.host.Addrs()
				addrStrings := make([]string, len(addrs))
				for i, addr := range addrs {
					addrStrings[i] = addr.String()
				}
				return addrStrings
			}(),
			Protocols: func() []string {
				protocolIDs := s.host.Mux().Protocols()
				protocolStrings := make([]string, len(protocolIDs))
				for i, pid := range protocolIDs {
					protocolStrings[i] = string(pid)
				}
				return protocolStrings
			}(),
			NetworkId: []byte(s.getNetworkID()),
		},
	}

	for _, p := range peers {
		conns := s.host.Network().ConnsToPeer(p)
		peerInfo := &transportpb.PeerInfo{
			Id: p.String(),
		}

		if len(conns) > 0 {
			conn := conns[0]
			peerInfo.Addresses = []string{conn.RemoteMultiaddr().String()}
			peerInfo.Direction = conn.Stat().Direction.String()
			peerInfo.ConnectedTime = uint64(conn.Stat().Opened.Unix())
		}

		peerList.Peers = append(peerList.Peers, peerInfo)
	}

	return peerList
}
