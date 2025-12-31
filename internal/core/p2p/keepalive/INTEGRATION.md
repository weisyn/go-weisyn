# KeyPeerMonitor Runtime集成指南

## 1. 在主应用中添加Module

在应用的依赖注入配置中（通常是 `cmd/node/main.go` 或 `internal/app/app.go`），添加keepalive模块：

```go
import (
    "github.com/weisyn/v1/internal/core/p2p/keepalive"
)

func NewApp() *fx.App {
    return fx.New(
        // ... 其他模块 ...
        
        // P2P相关模块
        p2phost.Module(),      // P2P host模块
        discovery.Module(),    // Discovery模块
        kademlia.Module(),     // Kademlia路由表模块
        
        // 🔧 新增：KeyPeerMonitor模块
        keepalive.Module(),    // KeyPeer监控与保活模块
        
        // ... 其他模块 ...
    )
}
```

## 2. 确保依赖顺序

KeyPeerMonitor依赖以下组件，确保它们在之前初始化：
- `host.Host` - libp2p主机（由p2phost模块提供）
- `p2pi.RendezvousRouting` - DHT路由（由routing模块提供）
- `p2pi.Discovery` - Discovery服务（由discovery模块提供）
- `event.EventBus` - 事件总线（由event模块提供）
- `*p2pcfg.Options` - P2P配置

## 3. 配置启用

在配置文件中启用KeyPeerMonitor（默认已启用）：

```yaml
node:
  p2p:
    # KeyPeer监控配置
    enable_key_peer_monitor: true
    key_peer_probe_interval: 60s
    per_peer_min_probe_interval: 30s
    probe_timeout: 5s
    probe_fail_threshold: 3
    probe_max_concurrent: 5
    key_peer_set_max_size: 128
    
    # Discovery间隔收敛配置
    discovery_max_interval_cap: 2m
    dht_steady_interval_cap: 2m
    discovery_reset_min_interval: 30s
    discovery_reset_cool_down: 10s
```

## 4. 运行时行为

启动后，KeyPeerMonitor会：
1. 每60s扫描KeyPeerSet
2. 对未连接的关键peer执行探测
3. 失败达阈值后触发自愈链路
4. 发布Discovery间隔重置事件

日志示例：
```
[INFO] 🚀 正在启动KeyPeerMonitor...
[INFO] ✅ KeyPeerMonitor已启动: interval=60s per_peer_min=30s timeout=5s threshold=3 concurrent=5
[DEBUG] 开始KeyPeer探测轮次: key_peers=15
[WARN] 探测peer失败: 12D3K..., 失败次数=2/3, 错误: context deadline exceeded
[INFO] 🔧 开始修复peer连接: 12D3K...
[INFO] ✅ 使用新地址重连成功: 12D3K...
[INFO] 🔄 关键peer修复失败，已触发Discovery间隔重置: 12D3K...
```

## 5. 指标监控

通过诊断接口查看指标：
```bash
curl http://localhost:28686/debug/p2p/keepalive/metrics
```

返回示例：
```json
{
  "probe_attempts": 150,
  "probe_success": 140,
  "probe_fail": 10,
  "reconnect_attempts": 10,
  "reconnect_success": 8,
  "repair_triggered": 3,
  "repair_success": 2,
  "reset_events_published": 3
}
```

## 6. 与其他组件的交互

### 6.1 与Discovery的协同
- KeyPeerMonitor发布`EventTypeDiscoveryIntervalReset`事件
- Discovery.Service订阅该事件并重置scheduler和DHT循环

### 6.2 与Kademlia的协同
- Kademlia在`FindClosestPeers`失败时也会发布重置事件
- KeyPeerSet可从Kademlia获取Active+Suspect节点列表

### 6.3 与AddrManager的协同
- KeyPeerMonitor通过AddrManager的`triggerAddrLookup`补充地址
- AddrManager在地址大规模过期时可触发重置事件

## 7. 故障排查

### 7.1 KeyPeerMonitor未启动
检查：
- 配置中`enable_key_peer_monitor`是否为true
- 日志中是否有"缺少libp2p host"警告
- 依赖组件是否正确初始化

### 7.2 探测失败率过高
调整配置：
- 增大`probe_timeout`（如改为10s）
- 增大`probe_fail_threshold`（如改为5）
- 增大`per_peer_min_probe_interval`避免频繁探测

### 7.3 事件风暴
检查：
- `discovery_reset_cool_down`是否过小
- 是否有大量peer同时断连
- 考虑增大冷却时间或降低探测频率

## 8. 性能考虑

- **CPU开销**：探测循环每60s运行一次，对≤128个peer执行轻量级连接检查
- **网络开销**：仅对断连的关键peer发起连接尝试，有并发限制（默认5）
- **内存开销**：KeyPeerSet + 探测状态映射，约几KB到几十KB

## 9. 生产环境最佳实践

1. **监控关键指标**：
   - `probe_fail`持续上升 → 网络质量问题
   - `repair_fail`频繁 → DHT或地址管理问题
   - `reset_events_published`过多 → 考虑调整阈值

2. **告警阈值**：
   - `repair_fail / repair_triggered > 0.5` 持续5分钟
   - `reset_events_published` 在1分钟内>10次

3. **容量规划**：
   - KeyPeerSet建议≤128个（默认）
   - Bootstrap节点建议3-5个
   - 并发探测数建议5-10个

## 10. 与原有代码的兼容性

✅ **完全兼容**：
- 不影响现有Discovery逻辑（只是加速响应）
- 不影响现有连接管理（ConnMgr仍正常工作）
- 可通过配置开关随时启用/禁用

❌ **不向后兼容的改变**：
- `AdvertiseInterval`不再用于Discovery/DHT上限
- 新增配置项需要更新配置文件/环境变量

