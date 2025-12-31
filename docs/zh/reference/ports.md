# 端口规范

---

## 概述

本文档定义了 WES 系统使用的网络端口规范。

---

## 默认端口

| 端口 | 协议 | 服务 | 说明 |
|------|------|------|------|
| 30303 | TCP/UDP | P2P | 节点间通信 |
| 8545 | HTTP | API | REST/JSON-RPC API |
| 8546 | WebSocket | API | WebSocket API |
| 6060 | HTTP | Metrics | Prometheus 指标 |
| 9090 | HTTP | Admin | 管理接口 |

---

## 端口说明

### P2P 端口 (30303)

**用途**：节点间 P2P 通信

**协议**：
- TCP：节点发现、区块同步
- UDP：节点发现（Kademlia）

**配置**：
```yaml
network:
  listen_addr: "0.0.0.0:30303"
```

**防火墙**：
```bash
# 入站和出站都需要开放
sudo ufw allow 30303/tcp
sudo ufw allow 30303/udp
```

### API 端口 (8545)

**用途**：对外提供 REST 和 JSON-RPC API

**协议**：HTTP

**配置**：
```yaml
api:
  http:
    enabled: true
    addr: "0.0.0.0:8545"
```

**安全建议**：
- 生产环境应限制访问来源
- 启用 HTTPS
- 配置认证

### WebSocket 端口 (8546)

**用途**：实时事件订阅

**协议**：WebSocket

**配置**：
```yaml
api:
  ws:
    enabled: true
    addr: "0.0.0.0:8546"
```

### 指标端口 (6060)

**用途**：Prometheus 指标收集

**协议**：HTTP

**配置**：
```yaml
metrics:
  enabled: true
  addr: "0.0.0.0:6060"
```

### 管理端口 (9090)

**用途**：节点管理和诊断

**协议**：HTTP

**配置**：
```yaml
admin:
  enabled: true
  addr: "127.0.0.1:9090"
```

**安全建议**：
- 仅绑定本地地址
- 生产环境禁用或严格限制

---

## 自定义端口

### 配置示例

```yaml
network:
  listen_addr: "0.0.0.0:31303"  # 自定义 P2P 端口

api:
  http:
    addr: "0.0.0.0:18545"  # 自定义 API 端口
  ws:
    addr: "0.0.0.0:18546"  # 自定义 WebSocket 端口

metrics:
  addr: "0.0.0.0:16060"  # 自定义指标端口

admin:
  addr: "127.0.0.1:19090"  # 自定义管理端口
```

---

## 多节点部署

在同一台机器上部署多个节点时，需要使用不同的端口：

| 节点 | P2P | API | WebSocket | Metrics |
|------|-----|-----|-----------|---------|
| 节点 1 | 30303 | 8545 | 8546 | 6060 |
| 节点 2 | 30304 | 8547 | 8548 | 6061 |
| 节点 3 | 30305 | 8549 | 8550 | 6062 |

---

## 相关文档

- [配置参考](./config/) - 完整配置说明
- [部署操作](../how-to/deploy/) - 部署指南
- [网络与拓扑](../concepts/network-and-topology.md) - 网络架构

