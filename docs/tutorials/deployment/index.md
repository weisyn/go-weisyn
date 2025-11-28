# WES 部署指南

---

## 🎯 部署概览

本指南介绍如何部署 WES 节点到生产环境、测试环境和常见拓扑。

---

## 📋 部署前准备

### 系统要求

**生产环境**：
- **操作系统**: Linux（推荐 Ubuntu 22.04+）
- **内存**: 至少 8GB RAM（推荐 16GB+）
- **存储**: 至少 100GB 可用空间（推荐 SSD）
- **网络**: 稳定的互联网连接，开放端口 8080（API）、8081（P2P）

**测试环境**：
- **操作系统**: Linux/macOS/Windows
- **内存**: 至少 4GB RAM
- **存储**: 至少 10GB 可用空间
- **网络**: 稳定的互联网连接

---

## 🚀 部署方式

### 方式 1：本地单节点部署

**适用场景**：开发、测试、单节点运行

**步骤**：
1. 下载并安装 WES 节点
2. 初始化节点配置
3. 启动节点

**详细步骤**：参见 [快速开始](../quickstart/)

---

### 方式 2：Docker 容器部署

**适用场景**：隔离环境、易于管理、跨平台

**步骤**：
1. 安装 Docker
2. 拉取 WES 镜像
3. 运行容器

**示例**：
```bash
# 1. 构建 Docker 镜像
docker build -t weisyn:latest .

# 2. 运行容器
docker run -d \
  --name wes-node \
  -p 8080:8080 \
  -p 9090:9090 \
  -v ./data:/app/data \
  -v ./configs:/app/configs \
  weisyn:latest \
  go run cmd/weisyn/main.go --env production --daemon
```

**注意**：当前 Dockerfile 为基础镜像，需要根据实际需求构建完整的 WES 节点镜像。

---

### 方式 3：云环境部署

**适用场景**：生产环境、高可用、弹性扩展

**支持平台**：
- AWS EC2
- Google Cloud Compute Engine
- Azure Virtual Machines
- 阿里云 ECS

**步骤**：
1. 创建云服务器实例
2. 安装 WES 节点
3. 配置安全组和防火墙
4. 启动节点

---

## 🏗️ 常见拓扑

### 拓扑 1：单节点

**适用场景**：开发、测试

```
┌─────────────┐
│  WES Node   │
│  (本地)     │
└─────────────┘
```

---

### 拓扑 2：多节点网络

**适用场景**：测试网络、私有网络

```
┌─────────┐     ┌─────────┐     ┌─────────┐
│ Node 1  │─────│ Node 2  │─────│ Node 3  │
└─────────┘     └─────────┘     └─────────┘
```

---

### 拓扑 3：生产集群

**适用场景**：生产环境、高可用

```
        ┌─────────────┐
        │  Load Balancer │
        └───────┬───────┘
                │
    ┌───────────┼───────────┐
    │           │           │
┌─────────┐ ┌─────────┐ ┌─────────┐
│ Node 1  │ │ Node 2  │ │ Node 3  │
└─────────┘ └─────────┘ └─────────┘
```

---

## ⚙️ 配置说明

### 生产环境配置

**节点配置**（JSON 格式）：
```json
{
  "api": {
    "http_enabled": true,
    "http_port": 8080,
    "http_enable_rest": true,
    "http_enable_jsonrpc": true,
    "http_enable_websocket": true,
    "grpc_enabled": true,
    "grpc_port": 9090
  },
  "network": {
    "chain_id": 1
  },
  "storage": {
    "badger": {
      "path": "/var/lib/wes/data/badger",
      "sync_writes": true
    }
  },
  "blockchain": {
    "execution": {
      "ispc": {
        "resource_limits": {
          "execution_timeout_seconds": 60,
          "max_memory_mb": 512
        }
      }
    }
  }
}
```

### 测试环境配置

**节点配置**（JSON 格式）：
```json
{
  "api": {
    "http_enabled": true,
    "http_port": 8080,
    "grpc_enabled": true,
    "grpc_port": 9090
  },
  "network": {
    "chain_id": 20001
  },
  "storage": {
    "badger": {
      "path": "./data/badger"
    }
  }
}
```

**配置文件位置**：
- 开发环境：`configs/development/single/config.json`
- 测试环境：`configs/testing/config.json`
- 生产环境：`configs/production/config.json`

---

## 🔒 安全配置

### 防火墙配置

**开放端口**：
- `8080` - HTTP API 端口（承载 REST/JSON-RPC/WebSocket）
- `9090` - gRPC API 端口
- `4001` - P2P 网络端口（libp2p）

**限制访问**：
- API 端口（8080/9090）：仅允许内网访问
- P2P 端口（4001）：允许公网访问

### 认证配置

**API 密钥**：
- 为管理 API 配置 API 密钥
- 定期轮换 API 密钥

---

## 📊 监控与运维

### 监控指标

**关键指标**：
- 节点状态
- 区块高度
- 交易吞吐量
- 网络连接数
- 存储使用率

### 日志管理

**日志位置**：
- `./logs/wes.log` - 主日志文件
- `./logs/error.log` - 错误日志

**日志级别**：
- `DEBUG` - 调试信息
- `INFO` - 一般信息
- `WARN` - 警告信息
- `ERROR` - 错误信息

---

## 📚 相关文档

- [快速开始](../quickstart/) - 快速上手 WES
- [配置参考](../../reference/config/) - 配置字段说明
- [故障排查](../../troubleshooting/operations.md) - 运维问题排查

---

**相关文档**：
- [产品总览](../../overview.md) - 了解 WES 是什么、核心价值、应用场景
- [快速开始](../quickstart/) - 快速上手 WES

