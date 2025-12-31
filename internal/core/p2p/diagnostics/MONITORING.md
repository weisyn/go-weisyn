# P2P 监控集成文档

## 概述

P2P 模块提供了完整的 Prometheus 指标和 HTTP 诊断端点，用于监控和调试 P2P 网络状态。

## Prometheus 指标

### 连接和 Peer 指标

| 指标名称 | 类型 | 说明 |
|---------|------|------|
| `p2p_connections_total` | Gauge | 当前 P2P 连接数 |
| `p2p_peers_total` | Gauge | 当前连接的 Peer 数量 |

### 带宽指标

| 指标名称 | 类型 | 说明 |
|---------|------|------|
| `p2p_bandwidth_in_rate_bytes_per_sec` | Gauge | 入站带宽速率（字节/秒） |
| `p2p_bandwidth_out_rate_bytes_per_sec` | Gauge | 出站带宽速率（字节/秒） |
| `p2p_bandwidth_in_total_bytes` | Gauge | 入站总字节数 |
| `p2p_bandwidth_out_total_bytes` | Gauge | 出站总字节数 |

### Discovery 指标

| 指标名称 | 类型 | 说明 |
|---------|------|------|
| `p2p_discovery_bootstrap_attempt_total` | Counter | Bootstrap 尝试总次数 |
| `p2p_discovery_bootstrap_success_total` | Counter | Bootstrap 成功总次数 |
| `p2p_discovery_mdns_peer_found_total` | Counter | mDNS 发现的 Peer 总数 |
| `p2p_discovery_mdns_connect_success_total` | Counter | mDNS 连接成功总次数 |
| `p2p_discovery_mdns_connect_fail_total` | Counter | mDNS 连接失败总次数 |
| `p2p_discovery_last_bootstrap_unixtime` | Gauge | 最后 Bootstrap 时间戳（Unix 时间） |
| `p2p_discovery_last_mdns_found_unixtime` | Gauge | 最后 mDNS 发现时间戳（Unix 时间） |

## HTTP 诊断端点

### `/metrics`

**方法**: GET  
**Content-Type**: `text/plain; version=0.0.4; charset=utf-8`

返回 Prometheus 格式的指标数据。

**示例**:
```bash
curl http://127.0.0.1:28686/metrics
```

**响应示例**:
```
# HELP p2p_connections_total Current number of P2P connections
# TYPE p2p_connections_total gauge
p2p_connections_total 5

# HELP p2p_peers_total Current number of connected peers
# TYPE p2p_peers_total gauge
p2p_peers_total 3
...
```

### `/debug/p2p/peers`

**方法**: GET  
**Content-Type**: `application/json`

返回当前连接的 Peer 列表。

**响应示例**:
```json
{
  "peers": 3,
  "peer_ids": ["peer1", "peer2", "peer3"]
}
```

### `/debug/p2p/connections`

**方法**: GET  
**Content-Type**: `application/json`

返回当前连接数。

**响应示例**:
```json
{
  "connections": 5
}
```

### `/debug/p2p/stats`

**方法**: GET  
**Content-Type**: `application/json`

返回 P2P 统计信息，包括 Peer 数、连接数、带宽统计等。

**响应示例**:
```json
{
  "peers": 3,
  "connections": 5,
  "host_id": "12D3KooW...",
  "bandwidth": {
    "in_rate_bps": 1024.5,
    "out_rate_bps": 2048.3,
    "in_total_bytes": 1048576,
    "out_total_bytes": 2097152
  }
}
```

### `/debug/p2p/health`

**方法**: GET  
**Content-Type**: `application/json`

返回 P2P 健康检查信息，包括可达性状态、AutoNAT 状态、Relay 状态等。

**响应示例**:
```json
{
  "host_id": "12D3KooW...",
  "num_peers": 3,
  "num_conns": 5,
  "reachability": "public",
  "autoNAT_status": "public",
  "relay_stats": {
    "enabled": true
  }
}
```

### `/debug/p2p/routing`

**方法**: GET  
**Content-Type**: `application/json`

返回 DHT 路由表信息。

**响应示例**:
```json
{
  "routing_table_size": 20,
  "mode": "auto",
  "num_bootstrap_peers": 3
}
```

## 配置

诊断服务通过 `internal/config/p2p.Options` 配置：

```go
opts := &p2p.Options{
    DiagnosticsEnabled: true,
    DiagnosticsAddr:    "127.0.0.1:28686",
    // ... 其他配置
}
```

## 集成 Prometheus

### 1. 配置 Prometheus scrape 目标

在 `prometheus.yml` 中添加：

```yaml
scrape_configs:
  - job_name: 'wes-p2p'
    static_configs:
      - targets: ['127.0.0.1:28686']
    metrics_path: '/metrics'
    scrape_interval: 15s
```

### 2. 验证指标收集

启动 Prometheus 后，可以在 Prometheus UI 中查询指标：

```promql
# 查询当前连接数
p2p_connections_total

# 查询 Bootstrap 成功率
rate(p2p_discovery_bootstrap_success_total[5m]) / rate(p2p_discovery_bootstrap_attempt_total[5m])

# 查询带宽使用
p2p_bandwidth_in_rate_bytes_per_sec
p2p_bandwidth_out_rate_bytes_per_sec
```

## Grafana 仪表板示例

### 连接和 Peer 监控

```json
{
  "panels": [
    {
      "title": "P2P Connections",
      "targets": [
        {
          "expr": "p2p_connections_total"
        }
      ]
    },
    {
      "title": "P2P Peers",
      "targets": [
        {
          "expr": "p2p_peers_total"
        }
      ]
    }
  ]
}
```

### 带宽监控

```json
{
  "panels": [
    {
      "title": "Inbound Bandwidth",
      "targets": [
        {
          "expr": "p2p_bandwidth_in_rate_bytes_per_sec"
        }
      ]
    },
    {
      "title": "Outbound Bandwidth",
      "targets": [
        {
          "expr": "p2p_bandwidth_out_rate_bytes_per_sec"
        }
      ]
    }
  ]
}
```

### Discovery 监控

```json
{
  "panels": [
    {
      "title": "Bootstrap Success Rate",
      "targets": [
        {
          "expr": "rate(p2p_discovery_bootstrap_success_total[5m]) / rate(p2p_discovery_bootstrap_attempt_total[5m])"
        }
      ]
    },
    {
      "title": "mDNS Peers Found",
      "targets": [
        {
          "expr": "rate(p2p_discovery_mdns_peer_found_total[5m])"
        }
      ]
    }
  ]
}
```

## 验证脚本

使用提供的验证脚本检查监控系统：

```bash
./scripts/verify_p2p_monitoring.sh
```

该脚本会：
1. 检查代码编译
2. 运行单元测试
3. 验证 Prometheus 指标注册
4. 验证 HTTP 端点可用性
5. 验证 Prometheus 指标内容

## 故障排查

### 指标未更新

1. 检查诊断服务是否启动：
   ```bash
   curl http://127.0.0.1:28686/debug/p2p/health
   ```

2. 检查指标是否注册：
   ```bash
   curl http://127.0.0.1:28686/metrics | grep p2p_
   ```

3. 检查 Discovery 回调是否设置：
   - 确保 `discovery.Service.SetDiagnosticsCallbacks()` 被调用
   - 检查 Runtime 初始化代码

### HTTP 端点返回 503

1. 检查 Host 是否初始化
2. 检查诊断服务是否启动
3. 查看日志中的错误信息

### Prometheus 无法抓取指标

1. 检查网络连接
2. 检查防火墙设置
3. 验证 Prometheus 配置中的 targets 地址正确
4. 检查诊断服务日志

## 最佳实践

1. **监控关键指标**：
   - 连接数和 Peer 数（反映网络健康度）
   - Bootstrap 成功率（反映网络连通性）
   - 带宽使用（反映网络负载）

2. **设置告警规则**：
   ```yaml
   groups:
     - name: p2p_alerts
       rules:
         - alert: P2PConnectionsLow
           expr: p2p_connections_total < 2
           for: 5m
           annotations:
             summary: "P2P connections are low"
         
         - alert: BootstrapFailureRateHigh
           expr: rate(p2p_discovery_bootstrap_success_total[5m]) / rate(p2p_discovery_bootstrap_attempt_total[5m]) < 0.5
           for: 10m
           annotations:
             summary: "Bootstrap failure rate is high"
   ```

3. **定期检查**：
   - 每周检查指标趋势
   - 监控异常峰值
   - 分析 Discovery 成功率变化

