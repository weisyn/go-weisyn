# 配置更新指南 - DHT间隔与关键连接保活

## 更新日期
2025-12-16

## 背景
为解决"DHT发现间隔过长"和"连接缺乏保活"问题，新增了以下P2P配置项。

---

## 新增配置项列表

### 1. Discovery间隔收敛配置

在 `node` 配置节中添加以下字段：

```json
{
  "node": {
    "discovery_max_interval_cap": "2m",
    "_comment_discovery_max_interval_cap": "Discovery调度器最大间隔上限：时间字符串（如2m），默认2m，取代旧的15m上限，加快节点发现响应",
    
    "dht_steady_interval_cap": "2m",
    "_comment_dht_steady_interval_cap": "DHT稳定模式最大间隔上限：时间字符串（如2m），默认2m，确保DHT路由表定期刷新",
    
    "discovery_reset_min_interval": "30s",
    "_comment_discovery_reset_min_interval": "Discovery重置后最小间隔：时间字符串（如30s），默认30s，避免重置到过小值",
    
    "discovery_reset_cool_down": "10s",
    "_comment_discovery_reset_cool_down": "Discovery重置冷却时间：时间字符串（如10s），默认10s，防止重置事件风暴"
  }
}
```

### 2. KeyPeer监控保活配置

在 `node` 配置节中添加以下字段：

```json
{
  "node": {
    "enable_key_peer_monitor": true,
    "_comment_enable_key_peer_monitor": "是否启用关键peer监控保活：布尔值，true启用KeyPeerMonitor，false禁用，默认true",
    
    "key_peer_probe_interval": "60s",
    "_comment_key_peer_probe_interval": "关键peer探测轮次间隔：时间字符串（如60s），默认60s，每60秒扫描一次关键peer集合",
    
    "per_peer_min_probe_interval": "30s",
    "_comment_per_peer_min_probe_interval": "单个peer最小探测间隔：时间字符串（如30s），默认30s，避免频繁探测同一peer",
    
    "probe_timeout": "5s",
    "_comment_probe_timeout": "探测超时时间：时间字符串（如5s），默认5s，单次探测连接的超时时间",
    
    "probe_fail_threshold": 3,
    "_comment_probe_fail_threshold": "探测失败阈值：整数，默认3，连续失败达到此阈值后触发自愈",
    
    "probe_max_concurrent": 5,
    "_comment_probe_max_concurrent": "最大并发探测数：整数，默认5，限制同时进行的探测连接数，避免网络风暴",
    
    "key_peer_set_max_size": 128,
    "_comment_key_peer_set_max_size": "关键peer集合最大大小：整数，默认128，限制KeyPeerSet的peer数量"
  }
}
```

### 3. forceConnect（GossipSub Mesh 拉活，业务节点优先）

背景：WES 网络连接了大量“非业务的公网 libp2p 节点”。如果对 peerstore 做全量主动连接，容易出现 goroutine/内存突刺。\n+本配置提供“业务节点优先 + Tier2 抽样辅助公网发现/mesh形成”的可控拉活机制，并通过并发/预算/cooldown 强约束节流。\n+
在 `node.discovery` 配置节中添加以下字段：

```json
{
  "node": {
    "discovery": {
      "business_critical_peer_ids": [
        "12D3KooW..."
      ],
      "_comment_business_critical_peer_ids": "业务关键节点PeerID列表（个位数），forceConnect/保活优先级最高",

      "force_connect": {
        "enabled": true,
        "_comment_enabled": "是否启用GossipSub拉活（默认true）。如排障可临时关闭以观察网络自然收敛行为",

        "cooldown": "2m",
        "_comment_cooldown": "触发冷却时间（默认2m），2分钟内最多执行一轮拉活，避免多处触发叠加导致风暴",

        "concurrency": 15,
        "_comment_concurrency": "并发拨号上限（默认15），建议10~20",

        "budget_per_round": 50,
        "_comment_budget_per_round": "每轮总拨号预算（默认50），优先覆盖业务节点/bootstraps/topic peers",

        "tier2_sample_budget": 20,
        "_comment_tier2_sample_budget": "Tier2（非业务海量libp2p节点）抽样预算（默认20），仅用于公网发现/mesh拉活",

        "timeout": "10s",
        "_comment_timeout": "单peer拨号超时（默认10s）"
      }
    }
  }
}
```

---

## 完整示例（仅展示node部分的修改）

```json
{
  "node": {
    "listen_addresses": [
      "/ip4/0.0.0.0/tcp/28683",
      "/ip6/::/tcp/28683",
      "/ip4/0.0.0.0/udp/28683/quic-v1",
      "/ip6/::/udp/28683/quic-v1"
    ],
    
    "host": {
      "diagnostics_enabled": true,
      "diagnostics_port": 28686,
      "identity": {
        "key_file": "./p2p/identity.key"
      },
      "advertise_private_addrs": false,
      "gater": {
        "mode": "open",
        "allow_cidrs": [],
        "allow_prefixes": [],
        "deny_cidrs": [],
        "deny_prefixes": []
      }
    },
    
    "bootstrap_peers": [
      "/ip4/101.37.245.124/tcp/28683/p2p/12D3KooWKP9yJbstwT3mYpvNc5CpmiVjpdqAcja3JLMeifroreBz"
    ],
    
    "enable_mdns": true,
    "enable_dht": true,
    "enable_nat_port": true,
    "enable_dcutr": true,
    "enable_auto_relay": true,
    "enable_autonat_client": true,
    
    "_comment_new_configs": "🆕 以下为2025-12-16新增的DHT间隔与保活配置",
    
    "discovery_max_interval_cap": "2m",
    "_comment_discovery_max_interval_cap": "Discovery调度器最大间隔上限：时间字符串（如2m），默认2m，取代旧的15m上限",
    
    "dht_steady_interval_cap": "2m",
    "_comment_dht_steady_interval_cap": "DHT稳定模式最大间隔上限：时间字符串（如2m），默认2m",
    
    "discovery_reset_min_interval": "30s",
    "_comment_discovery_reset_min_interval": "Discovery重置后最小间隔：时间字符串（如30s），默认30s",
    
    "discovery_reset_cool_down": "10s",
    "_comment_discovery_reset_cool_down": "Discovery重置冷却时间：时间字符串（如10s），默认10s",
    
    "enable_key_peer_monitor": true,
    "_comment_enable_key_peer_monitor": "是否启用关键peer监控保活：布尔值，true启用，false禁用，默认true",
    
    "key_peer_probe_interval": "60s",
    "_comment_key_peer_probe_interval": "关键peer探测轮次间隔：时间字符串（如60s），默认60s",
    
    "per_peer_min_probe_interval": "30s",
    "_comment_per_peer_min_probe_interval": "单个peer最小探测间隔：时间字符串（如30s），默认30s",
    
    "probe_timeout": "5s",
    "_comment_probe_timeout": "探测超时时间：时间字符串（如5s），默认5s",
    
    "probe_fail_threshold": 3,
    "_comment_probe_fail_threshold": "探测失败阈值：整数，默认3",
    
    "probe_max_concurrent": 5,
    "_comment_probe_max_concurrent": "最大并发探测数：整数，默认5",
    
    "key_peer_set_max_size": 128,
    "_comment_key_peer_set_max_size": "关键peer集合最大大小：整数，默认128"
  }
}
```

---

## 更新步骤

### 步骤1：更新现有配置文件

对于以下配置文件，添加新的配置项：

1. `configs/chains/test-public-demo.json`
2. `configs/chains/dev-public-local.json`
3. 其他自定义的链配置文件

### 步骤2：验证配置

运行配置验证工具（如果有的话）：

```bash
# 验证配置文件格式
jq empty configs/chains/test-public-demo.json
```

### 步骤3：重启节点

使用更新后的配置重启节点：

```bash
# 使用测试网配置
weisyn-node --chain public

# 或使用自定义配置
weisyn-node --chain public --config ./configs/chains/my-config.json
```

---

## 配置说明

### Discovery间隔收敛

**作用**：
- 将Discovery/DHT的最大间隔从15分钟大幅降低到2分钟
- 通过事件驱动机制在关键情况下立即重置间隔
- 加快节点发现和地址刷新的响应速度

**推荐值**：
- `discovery_max_interval_cap`: 2m (公有链/联盟链)
- `dht_steady_interval_cap`: 2m (公有链/联盟链)
- `discovery_reset_min_interval`: 30s
- `discovery_reset_cool_down`: 10s

**调优建议**：
- 网络节点较少（<10个）：可适当增大间隔到5m
- 网络节点众多（>100个）：保持2m
- 本地开发/测试：可降低到1m以加快测试

### KeyPeer监控保活

**作用**：
- 定期探测关键peer（bootstrap、K桶核心、最近有用、业务关键）的连接状态
- 失败时自动触发重连和DHT地址查询
- 通过自愈链路确保关键连接的可用性

**推荐值**：
- `key_peer_probe_interval`: 60s (生产环境)
- `per_peer_min_probe_interval`: 30s
- `probe_timeout`: 5s
- `probe_fail_threshold`: 3
- `probe_max_concurrent`: 5
- `key_peer_set_max_size`: 128

**调优建议**：
- 网络质量差：增大`probe_timeout`到10s，`probe_fail_threshold`到5
- 节点频繁断连：减小`key_peer_probe_interval`到30s
- 网络风暴告警：增大`per_peer_min_probe_interval`到60s
- 资源受限环境：减小`probe_max_concurrent`到3

---

## 向后兼容性

**完全向后兼容**：
- 未配置新字段时，使用代码中的默认值
- 不会影响现有的Discovery和连接管理逻辑
- 可通过 `enable_key_peer_monitor=false` 快速禁用新功能

**不向后兼容的改变**：
- `AdvertiseInterval`不再用于Discovery/DHT的上限计算
- 新节点将使用更激进的发现策略（2m vs 15m）

---

## 监控与告警

### 关键指标

通过诊断接口查看：

```bash
# KeyPeer监控指标
curl http://localhost:28686/debug/p2p/keepalive/metrics

# Discovery状态
curl http://localhost:28686/debug/p2p/discovery
```

### 告警阈值建议

1. **探测失败率过高**：`probe_fail / probe_attempts > 0.5` 持续5分钟
2. **修复失败率过高**：`repair_fail / repair_triggered > 0.5` 持续5分钟
3. **重置事件风暴**：`reset_events_published` 在1分钟内>10次

---

## 故障排查

### KeyPeerMonitor未启动

**检查**：
- 配置中 `enable_key_peer_monitor` 是否为true
- 日志中是否有"KeyPeerMonitor已启动"消息
- 是否有"缺少libp2p host"警告

### 探测失败率过高

**调整配置**：
```json
{
  "probe_timeout": "10s",       // 从5s增加到10s
  "probe_fail_threshold": 5     // 从3增加到5
}
```

### 重置事件风暴

**调整配置**：
```json
{
  "discovery_reset_cool_down": "30s"  // 从10s增加到30s
}
```

---

## 参考文档

- 设计文档: `_dev/14-实施任务-implementation-tasks/20251216-network-degradation-root-cause-analysis/DHT_INTERVAL_KEEPALIVE_FIX.md`
- 实施报告: `_dev/14-实施任务-implementation-tasks/20251216-network-degradation-root-cause-analysis/DHT_INTERVAL_KEEPALIVE_IMPLEMENTATION_COMPLETE.md`
- 集成指南: `internal/core/p2p/keepalive/INTEGRATION.md`
- 模块文档: `internal/core/p2p/keepalive/README.md`

---

## 更新历史

- **2025-12-16**: 初始版本，新增11个配置项（4个Discovery + 7个KeyPeerMonitor）

