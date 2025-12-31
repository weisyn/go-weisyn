package diagnostics

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/metrics"
	"github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestHost 创建一个用于测试的 libp2p Host
func createTestHost(t *testing.T) host.Host {
	mn := mocknet.New()
	h, err := mn.GenPeer()
	require.NoError(t, err)
	return h
}

// TestService_RegisterMetrics 测试指标注册
func TestService_RegisterMetrics(t *testing.T) {
	// Arrange
	svc := NewService("127.0.0.1:0")
	bwReporter := metrics.NewBandwidthCounter()
	testHost := createTestHost(t)

	// Act
	svc.Initialize(testHost, nil, bwReporter)

	// Assert: 验证所有指标都已注册
	metrics, err := svc.registry.Gather()
	require.NoError(t, err)

	metricNames := make(map[string]bool)
	for _, mf := range metrics {
		metricNames[mf.GetName()] = true
	}

	// 验证基础指标
	assert.True(t, metricNames["p2p_connections_total"], "p2p_connections_total should be registered")
	assert.True(t, metricNames["p2p_peers_total"], "p2p_peers_total should be registered")
	assert.True(t, metricNames["p2p_bandwidth_in_rate_bytes_per_sec"], "p2p_bandwidth_in_rate_bytes_per_sec should be registered")
	assert.True(t, metricNames["p2p_bandwidth_out_rate_bytes_per_sec"], "p2p_bandwidth_out_rate_bytes_per_sec should be registered")
	assert.True(t, metricNames["p2p_bandwidth_in_total_bytes"], "p2p_bandwidth_in_total_bytes should be registered")
	assert.True(t, metricNames["p2p_bandwidth_out_total_bytes"], "p2p_bandwidth_out_total_bytes should be registered")

	// 验证 Discovery 指标
	assert.True(t, metricNames["p2p_discovery_bootstrap_attempt_total"], "p2p_discovery_bootstrap_attempt_total should be registered")
	assert.True(t, metricNames["p2p_discovery_bootstrap_success_total"], "p2p_discovery_bootstrap_success_total should be registered")
	assert.True(t, metricNames["p2p_discovery_mdns_peer_found_total"], "p2p_discovery_mdns_peer_found_total should be registered")
	assert.True(t, metricNames["p2p_discovery_mdns_connect_success_total"], "p2p_discovery_mdns_connect_success_total should be registered")
	assert.True(t, metricNames["p2p_discovery_mdns_connect_fail_total"], "p2p_discovery_mdns_connect_fail_total should be registered")
	assert.True(t, metricNames["p2p_discovery_last_bootstrap_unixtime"], "p2p_discovery_last_bootstrap_unixtime should be registered")
	assert.True(t, metricNames["p2p_discovery_last_mdns_found_unixtime"], "p2p_discovery_last_mdns_found_unixtime should be registered")
}

// TestService_RecordDiscoveryMetrics 测试 Discovery 指标记录
func TestService_RecordDiscoveryMetrics(t *testing.T) {
	// Arrange
	svc := NewService("127.0.0.1:0")
	bwReporter := metrics.NewBandwidthCounter()
	testHost := createTestHost(t)
	svc.Initialize(testHost, nil, bwReporter)

	// Act: 记录各种 Discovery 事件
	svc.RecordDiscoveryBootstrapAttempt()
	svc.RecordDiscoveryBootstrapSuccess()
	svc.RecordDiscoveryMDNSPeerFound()
	svc.RecordDiscoveryMDNSConnectSuccess()
	svc.RecordDiscoveryMDNSConnectFail()
	svc.UpdateDiscoveryLastBootstrapTS()
	svc.UpdateDiscoveryLastMDNSTS()

	// Assert: 验证指标值已更新
	metrics, err := svc.registry.Gather()
	require.NoError(t, err)

	for _, mf := range metrics {
		switch mf.GetName() {
		case "p2p_discovery_bootstrap_attempt_total":
			assert.Equal(t, 1.0, mf.GetMetric()[0].GetCounter().GetValue(), "bootstrap attempt should be 1")
		case "p2p_discovery_bootstrap_success_total":
			assert.Equal(t, 1.0, mf.GetMetric()[0].GetCounter().GetValue(), "bootstrap success should be 1")
		case "p2p_discovery_mdns_peer_found_total":
			assert.Equal(t, 1.0, mf.GetMetric()[0].GetCounter().GetValue(), "mdns peer found should be 1")
		case "p2p_discovery_mdns_connect_success_total":
			assert.Equal(t, 1.0, mf.GetMetric()[0].GetCounter().GetValue(), "mdns connect success should be 1")
		case "p2p_discovery_mdns_connect_fail_total":
			assert.Equal(t, 1.0, mf.GetMetric()[0].GetCounter().GetValue(), "mdns connect fail should be 1")
		case "p2p_discovery_last_bootstrap_unixtime":
			assert.Greater(t, mf.GetMetric()[0].GetGauge().GetValue(), float64(0), "last bootstrap TS should be > 0")
		case "p2p_discovery_last_mdns_found_unixtime":
			assert.Greater(t, mf.GetMetric()[0].GetGauge().GetValue(), float64(0), "last mdns TS should be > 0")
		}
	}
}

// TestService_HTTPEndpoints 测试 HTTP 端点响应
func TestService_HTTPEndpoints(t *testing.T) {
	// Arrange
	svc := NewService("")
	bwReporter := metrics.NewBandwidthCounter()
	testHost := createTestHost(t)
	svc.Initialize(testHost, nil, bwReporter)

	// 创建测试 HTTP 服务器
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(svc.registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/debug/p2p/peers", svc.handlePeers)
	mux.HandleFunc("/debug/p2p/connections", svc.handleConnections)
	mux.HandleFunc("/debug/p2p/stats", svc.handleStats)
	mux.HandleFunc("/debug/p2p/health", svc.handleHealth)
	mux.HandleFunc("/debug/p2p/routing", svc.handleRouting)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	baseURL := ts.URL

	// Test /metrics endpoint
	t.Run("GET /metrics", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/metrics")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "metrics endpoint should return 200")
		contentType := resp.Header.Get("Content-Type")
		assert.Contains(t, contentType, "text/plain", "metrics should return text/plain")
		assert.Contains(t, contentType, "version=0.0.4", "metrics should return prometheus format")
	})

	// Test /debug/p2p/peers endpoint
	t.Run("GET /debug/p2p/peers", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/debug/p2p/peers")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "peers endpoint should return 200")
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"), "peers should return JSON")

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.Contains(t, result, "peers", "response should contain peers field")
	})

	// Test /debug/p2p/connections endpoint
	t.Run("GET /debug/p2p/connections", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/debug/p2p/connections")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "connections endpoint should return 200")
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"), "connections should return JSON")

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.Contains(t, result, "connections", "response should contain connections field")
	})

	// Test /debug/p2p/stats endpoint
	t.Run("GET /debug/p2p/stats", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/debug/p2p/stats")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "stats endpoint should return 200")
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"), "stats should return JSON")

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.Contains(t, result, "peers", "response should contain peers field")
		assert.Contains(t, result, "connections", "response should contain connections field")
		assert.Contains(t, result, "host_id", "response should contain host_id field")
		assert.Contains(t, result, "bandwidth", "response should contain bandwidth field")
	})

	// Test /debug/p2p/health endpoint
	t.Run("GET /debug/p2p/health", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/debug/p2p/health")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "health endpoint should return 200")
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"), "health should return JSON")

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.Contains(t, result, "host_id", "response should contain host_id field")
		assert.Contains(t, result, "num_peers", "response should contain num_peers field")
		assert.Contains(t, result, "num_conns", "response should contain num_conns field")
		assert.Contains(t, result, "reachability", "response should contain reachability field")
		assert.Contains(t, result, "autoNAT_status", "response should contain autoNAT_status field")
		assert.Contains(t, result, "relay_stats", "response should contain relay_stats field")
	})

	// Test /debug/p2p/routing endpoint
	t.Run("GET /debug/p2p/routing", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/debug/p2p/routing")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "routing endpoint should return 200")
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"), "routing should return JSON")

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.Contains(t, result, "routing_table_size", "response should contain routing_table_size field")
		assert.Contains(t, result, "mode", "response should contain mode field")
		assert.Contains(t, result, "num_bootstrap_peers", "response should contain num_bootstrap_peers field")
	})
}

// TestService_MetricsEndpoint_Content 测试 /metrics 端点内容
func TestService_MetricsEndpoint_Content(t *testing.T) {
	// Arrange
	svc := NewService("")
	bwReporter := metrics.NewBandwidthCounter()
	testHost := createTestHost(t)
	svc.Initialize(testHost, nil, bwReporter)

	// 记录一些指标
	svc.RecordDiscoveryBootstrapAttempt()
	svc.RecordDiscoveryBootstrapSuccess()

	// 创建测试 HTTP 服务器
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(svc.registry, promhttp.HandlerOpts{}))
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Act: 获取 metrics
	resp, err := http.Get(ts.URL + "/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()

	// Assert: 验证 metrics 内容包含预期的指标
	body := make([]byte, 10000)
	n, _ := resp.Body.Read(body)
	metricsContent := string(body[:n])

	assert.Contains(t, metricsContent, "p2p_connections_total", "metrics should contain p2p_connections_total")
	assert.Contains(t, metricsContent, "p2p_peers_total", "metrics should contain p2p_peers_total")
	assert.Contains(t, metricsContent, "p2p_discovery_bootstrap_attempt_total", "metrics should contain bootstrap attempt")
	assert.Contains(t, metricsContent, "p2p_discovery_bootstrap_success_total", "metrics should contain bootstrap success")
}

