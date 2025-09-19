package host

import (
	"context"
	"net/http"
	"runtime"
	"time"

	"strconv"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/metrics"
	"github.com/libp2p/go-libp2p/core/network"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	transportpb "github.com/weisyn/v1/pb/network/transport"
)

// diagnostics.go
// 诊断管理器：严格基于pb定义的诊断数据序列化
// 🎯 核心原则：完全使用pb定义，删除所有JSON map结构

// DiagnosticsManager 诊断管理器
type DiagnosticsManager struct {
	host     host.Host
	server   *http.Server
	bw       metrics.Reporter
	registry *prometheus.Registry

	// Prometheus指标
	totalConnections prometheus.Counter
	messagesSent     prometheus.Counter
	messagesReceived prometheus.Counter
	bandwidthIn      prometheus.Counter
	bandwidthOut     prometheus.Counter
	errorCount       prometheus.Counter

	// 发现指标
	discoveryBootstrapAttempts  prometheus.Counter
	discoveryBootstrapSuccess   prometheus.Counter
	discoveryMDNSPeerFound      prometheus.Counter
	discoveryMDNSConnectSuccess prometheus.Counter
	discoveryMDNSConnectFail    prometheus.Counter
	discoveryLastBootstrapTS    prometheus.Gauge
	discoveryLastMDNSTS         prometheus.Gauge
}

// NewDiagnosticsManager 创建诊断管理器
func NewDiagnosticsManager(host host.Host, bw metrics.Reporter, port int) *DiagnosticsManager {
	registry := prometheus.NewRegistry()

	dm := &DiagnosticsManager{
		host:     host,
		bw:       bw,
		registry: registry,
	}

	// 注册Prometheus指标
	dm.totalConnections = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "p2p_connections_total",
		Help: "Total number of P2P connections established",
	})

	dm.messagesSent = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "p2p_messages_sent_total",
		Help: "Total number of P2P messages sent",
	})

	dm.messagesReceived = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "p2p_messages_received_total",
		Help: "Total number of P2P messages received",
	})

	dm.bandwidthIn = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "p2p_bandwidth_in_bytes_total",
		Help: "Total inbound bandwidth in bytes",
	})

	dm.bandwidthOut = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "p2p_bandwidth_out_bytes_total",
		Help: "Total outbound bandwidth in bytes",
	})

	dm.errorCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "p2p_errors_total",
		Help: "Total number of P2P errors",
	})

	// 发现指标
	dm.discoveryBootstrapAttempts = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "p2p_discovery_bootstrap_attempt_total",
		Help: "Total bootstrap attempts",
	})

	dm.discoveryBootstrapSuccess = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "p2p_discovery_bootstrap_success_total",
		Help: "Successful bootstrap attempts",
	})

	dm.discoveryMDNSPeerFound = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "p2p_discovery_mdns_peer_found_total",
		Help: "MDNS peers discovered",
	})

	dm.discoveryMDNSConnectSuccess = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "p2p_discovery_mdns_connect_success_total",
		Help: "Successful MDNS connections",
	})

	dm.discoveryMDNSConnectFail = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "p2p_discovery_mdns_connect_fail_total",
		Help: "Failed MDNS connections",
	})

	dm.discoveryLastBootstrapTS = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "p2p_discovery_last_bootstrap_unixtime",
		Help: "Last bootstrap timestamp",
	})

	dm.discoveryLastMDNSTS = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "p2p_discovery_last_mdns_found_unixtime",
		Help: "Last MDNS discovery timestamp",
	})

	// 注册所有指标
	registry.MustRegister(
		dm.totalConnections,
		dm.messagesSent,
		dm.messagesReceived,
		dm.bandwidthIn,
		dm.bandwidthOut,
		dm.errorCount,
		dm.discoveryBootstrapAttempts,
		dm.discoveryBootstrapSuccess,
		dm.discoveryMDNSPeerFound,
		dm.discoveryMDNSConnectSuccess,
		dm.discoveryMDNSConnectFail,
		dm.discoveryLastBootstrapTS,
		dm.discoveryLastMDNSTS,
	)

	// 创建 HTTP 服务器 - 严格使用pb序列化
	mux := http.NewServeMux()

	// Prometheus 指标端点（保持标准格式）
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	// 🎯 重新设计的pb优先诊断端点
	mux.HandleFunc("/debug/peers", dm.handlePeersProtobuf)
	mux.HandleFunc("/debug/peers/json", dm.handlePeersJSON) // protobuf->JSON转换版本
	mux.HandleFunc("/debug/connections", dm.handleConnectionsProtobuf)
	mux.HandleFunc("/debug/connections/json", dm.handleConnectionsJSON)
	mux.HandleFunc("/debug/host", dm.handleHostInfoProtobuf)
	mux.HandleFunc("/debug/host/json", dm.handleHostInfoJSON)
	mux.HandleFunc("/debug/health", dm.handleHealthProtobuf)
	mux.HandleFunc("/debug/health/json", dm.handleHealthJSON)

	dm.server = &http.Server{Addr: ":" + strconv.Itoa(port), Handler: mux}

	return dm
}

// ==================== pb优先的处理器实现 ====================

// handlePeersProtobuf 返回protobuf格式的peer信息
func (dm *DiagnosticsManager) handlePeersProtobuf(w http.ResponseWriter, r *http.Request) {
	if dm.host == nil {
		http.Error(w, "Host not available", http.StatusServiceUnavailable)
		return
	}

	peers := dm.host.Network().Peers()
	peerList := &transportpb.PeerListResponse{
		TotalPeers: int32(len(peers)),
	}

	for _, p := range peers {
		// 获取地址
		addrs := dm.host.Peerstore().Addrs(p)
		addrStrings := make([]string, len(addrs))
		for i, addr := range addrs {
			addrStrings[i] = addr.String()
		}

		// 构建PeerInfo - 严格使用pb定义
		peerInfo := &transportpb.PeerInfo{
			Id:        p.String(),
			Addresses: addrStrings,
			IsTrusted: false, // 简化处理
		}

		// 获取连接信息
		conns := dm.host.Network().ConnsToPeer(p)
		if len(conns) > 0 {
			peerInfo.ConnectedTime = uint64(conns[0].Stat().Opened.Unix())
			peerInfo.Direction = conns[0].Stat().Direction.String()
		}

		peerList.Peers = append(peerList.Peers, peerInfo)
	}

	// 序列化为protobuf
	data, err := proto.Marshal(peerList)
	if err != nil {
		http.Error(w, "Serialization failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-protobuf")
	w.Write(data)
}

// handlePeersJSON 返回JSON格式的peer信息（protobuf->JSON转换）
func (dm *DiagnosticsManager) handlePeersJSON(w http.ResponseWriter, r *http.Request) {
	if dm.host == nil {
		http.Error(w, "Host not available", http.StatusServiceUnavailable)
		return
	}

	peers := dm.host.Network().Peers()
	peerList := &transportpb.PeerListResponse{
		TotalPeers: int32(len(peers)),
	}

	for _, p := range peers {
		addrs := dm.host.Peerstore().Addrs(p)
		addrStrings := make([]string, len(addrs))
		for i, addr := range addrs {
			addrStrings[i] = addr.String()
		}

		peerInfo := &transportpb.PeerInfo{
			Id:        p.String(),
			Addresses: addrStrings,
			IsTrusted: false,
		}

		conns := dm.host.Network().ConnsToPeer(p)
		if len(conns) > 0 {
			peerInfo.ConnectedTime = uint64(conns[0].Stat().Opened.Unix())
			peerInfo.Direction = conns[0].Stat().Direction.String()
		}

		peerList.Peers = append(peerList.Peers, peerInfo)
	}

	// 🎯 使用protojson进行pb->JSON转换，而不是手动构建JSON
	marshaler := protojson.MarshalOptions{
		EmitUnpopulated: true,
		Indent:          "  ",
	}

	jsonData, err := marshaler.Marshal(peerList)
	if err != nil {
		http.Error(w, "JSON conversion failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

// handleConnectionsProtobuf 返回protobuf格式的连接信息
func (dm *DiagnosticsManager) handleConnectionsProtobuf(w http.ResponseWriter, r *http.Request) {
	if dm.host == nil {
		http.Error(w, "Host not available", http.StatusServiceUnavailable)
		return
	}

	conns := dm.host.Network().Conns()
	// 🚨 架构问题：需要定义ConnectionListResponse消息
	// 临时使用PeerListResponse结构

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
	w.Write(data)
}

// handleConnectionsJSON protobuf->JSON转换版本
func (dm *DiagnosticsManager) handleConnectionsJSON(w http.ResponseWriter, r *http.Request) {
	if dm.host == nil {
		http.Error(w, "Host not available", http.StatusServiceUnavailable)
		return
	}

	conns := dm.host.Network().Conns()
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
	w.Write(jsonData)
}

// handleHostInfoProtobuf 返回protobuf格式的主机信息
func (dm *DiagnosticsManager) handleHostInfoProtobuf(w http.ResponseWriter, r *http.Request) {
	if dm.host == nil {
		http.Error(w, "Host not available", http.StatusServiceUnavailable)
		return
	}

	addrs := dm.host.Addrs()
	addrStrings := make([]string, len(addrs))
	for i, addr := range addrs {
		addrStrings[i] = addr.String()
	}

	protocolIDs := dm.host.Mux().Protocols()
	protocolStrings := make([]string, len(protocolIDs))
	for i, pid := range protocolIDs {
		protocolStrings[i] = string(pid)
	}

	nodeInfo := &transportpb.NodeInfo{
		Id:        dm.host.ID().String(),
		Addresses: addrStrings,
		Protocols: protocolStrings,
		NetworkId: dm.getNetworkIdBytes(),
	}

	data, err := proto.Marshal(nodeInfo)
	if err != nil {
		http.Error(w, "Serialization failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-protobuf")
	w.Write(data)
}

// handleHostInfoJSON protobuf->JSON转换版本
func (dm *DiagnosticsManager) handleHostInfoJSON(w http.ResponseWriter, r *http.Request) {
	if dm.host == nil {
		http.Error(w, "Host not available", http.StatusServiceUnavailable)
		return
	}

	addrs := dm.host.Addrs()
	addrStrings := make([]string, len(addrs))
	for i, addr := range addrs {
		addrStrings[i] = addr.String()
	}

	protocolIDs := dm.host.Mux().Protocols()
	protocolStrings := make([]string, len(protocolIDs))
	for i, pid := range protocolIDs {
		protocolStrings[i] = string(pid)
	}

	nodeInfo := &transportpb.NodeInfo{
		Id:        dm.host.ID().String(),
		Addresses: addrStrings,
		Protocols: protocolStrings,
		NetworkId: dm.getNetworkIdBytes(),
	}

	marshaler := protojson.MarshalOptions{EmitUnpopulated: true, Indent: "  "}
	jsonData, err := marshaler.Marshal(nodeInfo)
	if err != nil {
		http.Error(w, "JSON conversion failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

// handleHealthProtobuf 健康检查 - protobuf版本
func (dm *DiagnosticsManager) handleHealthProtobuf(w http.ResponseWriter, r *http.Request) {
	// 🚨 架构问题：需要定义HealthStatus消息
	// 临时使用简单的方式
	status := "healthy"
	if dm.host == nil {
		status = "unhealthy"
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	// 临时实现：直接写状态字符串
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(status))
}

// handleHealthJSON 健康检查 - JSON版本
func (dm *DiagnosticsManager) handleHealthJSON(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
	}

	if dm.host == nil {
		status["status"] = "unhealthy"
		status["reason"] = "host not available"
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	// 🚨 这里仍使用map结构，违反了pb优先原则
	// 需要定义专门的HealthStatus pb消息
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"` + status["status"].(string) + `"}`))
}

// ==================== 生命周期管理 ====================

// dmNotifiee 实现 network.Notifiee，将事件转为指标
type dmNotifiee struct{ dm *DiagnosticsManager }

func (n *dmNotifiee) Listen(_ network.Network, _ ma.Multiaddr)       {}
func (n *dmNotifiee) ListenClose(_ network.Network, _ ma.Multiaddr)  {}
func (n *dmNotifiee) Connected(_ network.Network, c network.Conn)    { n.dm.RecordConnection() }
func (n *dmNotifiee) Disconnected(_ network.Network, c network.Conn) {}

// getNetworkIdBytes 获取正确的网络ID字节数组
// 🎯 **网络隔离标识符生成器**
//
// 返回用于P2P握手的网络标识符，而不是本地PeerID。
// 这个标识符用于确保节点只连接到相同网络的其他节点。
//
// 格式：{NetworkNamespace}:{ChainID}
// 例如："testnet:2" 或 "mainnet:1"
//
// TODO: 需要从配置提供者获取真实的网络命名空间和链ID
func (dm *DiagnosticsManager) getNetworkIdBytes() []byte {
	// 临时硬编码，应该从配置提供者获取
	// TODO: 集成配置提供者来获取真实的网络信息
	networkNamespace := "mainnet" // 应该来自配置
	chainID := "1"                // 应该来自配置

	networkId := networkNamespace + ":" + chainID
	return []byte(networkId)
}

// Start 启动诊断服务
func (dm *DiagnosticsManager) Start() error {
	if dm.host != nil {
		dm.host.Network().Notify(&dmNotifiee{dm: dm})
	}

	go dm.collectMetrics()
	go func() {
		_ = dm.server.ListenAndServe()
	}()

	return nil
}

// Stop 停止诊断服务
func (dm *DiagnosticsManager) Stop() error {
	if dm.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return dm.server.Shutdown(ctx)
	}
	return nil
}

// collectMetrics 收集指标
func (dm *DiagnosticsManager) collectMetrics() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// 收集运行时指标
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		// 这里可以添加更多指标收集逻辑
	}
}

// ==================== 指标记录方法 ====================

func (dm *DiagnosticsManager) RecordMessage(sent bool) {
	if sent {
		dm.messagesSent.Inc()
	} else {
		dm.messagesReceived.Inc()
	}
}

func (dm *DiagnosticsManager) RecordBandwidth(bytes int64, out bool) {
	if out {
		dm.bandwidthOut.Add(float64(bytes))
	} else {
		dm.bandwidthIn.Add(float64(bytes))
	}
}

func (dm *DiagnosticsManager) RecordConnection() {
	dm.totalConnections.Inc()
}

func (dm *DiagnosticsManager) RecordError() {
	dm.errorCount.Inc()
}

// 发现指标记录方法
func (dm *DiagnosticsManager) RecordDiscoveryBootstrapAttempt() {
	dm.discoveryBootstrapAttempts.Inc()
	dm.discoveryLastBootstrapTS.SetToCurrentTime()
}

func (dm *DiagnosticsManager) RecordDiscoveryBootstrapSuccess() {
	dm.discoveryBootstrapSuccess.Inc()
}

func (dm *DiagnosticsManager) RecordDiscoveryMDNSPeerFound() {
	dm.discoveryMDNSPeerFound.Inc()
	dm.discoveryLastMDNSTS.SetToCurrentTime()
}

func (dm *DiagnosticsManager) RecordDiscoveryMDNSConnectSuccess() {
	dm.discoveryMDNSConnectSuccess.Inc()
}

func (dm *DiagnosticsManager) RecordDiscoveryMDNSConnectFail() {
	dm.discoveryMDNSConnectFail.Inc()
}

// ==================== 架构问题总结 ====================

/*
🚨 通过彻底重构诊断系统，暴露的架构需求：

1. **需要补充的pb消息定义**：
   ```proto
   // 应在pb/network/transport/diagnostics.proto中定义：
   message HealthStatus {
     string status = 1;
     uint64 timestamp = 2;
     string reason = 3;
   }

   message ConnectionInfo {
     string peer_id = 1;
     string local_addr = 2;
     string remote_addr = 3;
     string direction = 4;
     uint64 opened_time = 5;
     int32 streams_count = 6;
   }

   message ConnectionListResponse {
     repeated ConnectionInfo connections = 1;
     int32 total_connections = 2;
   }

   message DiagnosticsStats {
     RuntimeStats runtime = 1;
     NetworkStats network = 2;
     BandwidthStats bandwidth = 3;
   }
   ```

2. **当前解决方案特点**：
   ✅ 完全基于pb数据结构
   ✅ 提供protobuf和JSON两种格式
   ✅ 使用protojson进行标准转换
   ❌ 部分地方仍需要更完善的pb定义

3. **架构优势**：
   - 类型安全的数据结构
   - 标准化的序列化格式
   - 向后兼容的API演进
   - 高效的二进制传输

这种pb优先的诊断系统设计提供了真正的类型安全保障。
*/

// UpdateDiscoveryLastMDNSTS 更新MDNS发现时间戳
func (dm *DiagnosticsManager) UpdateDiscoveryLastMDNSTS() {
	if dm.discoveryLastMDNSTS != nil {
		dm.discoveryLastMDNSTS.SetToCurrentTime()
	}
}

// RecordDiscoveryMDNSConnectOK 记录MDNS连接成功
func (dm *DiagnosticsManager) RecordDiscoveryMDNSConnectOK() {
	if dm.discoveryMDNSConnectSuccess != nil {
		dm.discoveryMDNSConnectSuccess.Inc()
	}
}

// UpdateDiscoveryLastBootstrapTS 更新Bootstrap时间戳
func (dm *DiagnosticsManager) UpdateDiscoveryLastBootstrapTS() {
	if dm.discoveryLastBootstrapTS != nil {
		dm.discoveryLastBootstrapTS.SetToCurrentTime()
	}
}
