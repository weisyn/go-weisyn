# KeyPeer监控与保活模块

## 概述

KeyPeer监控模块负责主动探测和保活关键peer集合，确保网络核心节点的连通性。

## 核心组件

### 1. KeyPeerSet
关键peer集合管理，包含：
- **Bootstrap节点**：配置的种子节点
- **K桶核心节点**：Active+Suspect状态的路由节点
- **最近有用节点**：最近时间窗口内有用的peer
- **业务关键节点**：业务层显式标记的重要peer

### 2. KeyPeerMonitor
周期性探测器，功能：
- 按配置的间隔（默认60s）扫描KeyPeerSet
- 对每个peer执行连通性探测（受per-peer最小间隔限制）
- 失败达阈值后触发自愈链路
- 发布Discovery间隔重置事件

### 3. 自愈链路
探测失败后的修复流程：
1. 快速重连：使用peerstore当前地址
2. DHT补地址：通过FindPeer获取新地址
3. 二次重连：使用新地址重试
4. 事件重置：触发Discovery间隔加速

## 配置参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `EnableKeyPeerMonitor` | true | 启用关键peer监控 |
| `KeyPeerProbeInterval` | 60s | 探测周期 |
| `PerPeerMinProbeInterval` | 30s | 单个peer最小探测间隔 |
| `ProbeTimeout` | 5s | 探测超时 |
| `ProbeFailThreshold` | 3 | 失败阈值 |
| `ProbeMaxConcurrent` | 5 | 最大并发探测数 |
| `KeyPeerSetMaxSize` | 128 | 关键peer集合最大大小 |

## 指标观测

- `keypeer_probe_total`: 探测总数（按result分类）
- `keypeer_reconnect_total`: 重连总数
- `keypeer_repair_latency_seconds`: 修复延迟

## 与Discovery间隔收敛的协同

KeyPeerMonitor与Discovery间隔收敛机制协同工作：
- 关键peer断连时立即触发重置事件
- Discovery循环立即加速到baseInterval（不等15m）
- MTTR从分钟级降到几十秒级

## 实现状态

✅ 已完成核心实现
⏳ 单元测试框架已建立，待补充完整用例
⏳ 集成测试待实现（NAT断开、地址过期场景）

