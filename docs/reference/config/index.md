# WES 配置参考

---

## 🎯 概述

WES 节点采用模块化配置系统，每个模块都有独立的配置文件和默认值。

**配置方式**：
- JSON 配置文件
- 环境变量（部分支持）
- 代码默认值

**配置优先级**：用户配置 > 环境变量 > 默认值

---

## 📚 配置模块

WES 配置系统按功能模块组织：

| 模块 | 路径 | 说明 |
|------|------|------|
| **API** | `internal/config/api/` | API 服务配置（HTTP/gRPC/WebSocket） |
| **Node** | `internal/config/node/` | P2P 网络节点配置 |
| **Blockchain** | `internal/config/blockchain/` | 区块链核心配置 |
| **Consensus** | `internal/config/consensus/` | 共识机制配置 |
| **Storage** | `internal/config/storage/*/` | 存储后端配置（Badger/SQLite/File/Memory） |
| **Network** | `internal/config/network/` | 网络层配置 |
| **Event** | `internal/config/event/` | 事件系统配置 |
| **Log** | `internal/config/log/` | 日志配置 |
| **TX** | `internal/config/tx/*/` | 交易相关配置 |
| **TXPool** | `internal/config/txpool/` | 交易池配置 |
| **Sync** | `internal/config/sync/` | 同步配置 |
| **Compliance** | `internal/config/compliance/` | 合规配置 |

---

## 🔧 API 配置

### HTTP API

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `http.enabled` | bool | `true` | 是否启用 HTTP 服务 |
| `http.host` | string | `"0.0.0.0"` | 监听地址 |
| `http.port` | int | `8080` | 监听端口 |
| `http.enable_rest` | bool | `true` | 是否启用 REST 端点（/api/v1/*） |
| `http.enable_jsonrpc` | bool | `true` | 是否启用 JSON-RPC（/jsonrpc） |
| `http.enable_websocket` | bool | `true` | 是否启用 WebSocket（/ws） |
| `http.timeout` | duration | `30s` | 请求超时时间 |
| `http.read_timeout` | duration | `15s` | 读取超时时间 |
| `http.write_timeout` | duration | `15s` | 写入超时时间 |
| `http.cors_enabled` | bool | `true` | 是否启用 CORS |
| `http.cors_origins` | []string | `["*"]` | 允许的 CORS 源 |
| `http.rate_limit_requests_per_minute` | int | `600` | 每分钟最大请求数 |
| `http.max_request_size` | int | `4194304` | 最大请求大小（4MB） |

### gRPC API

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `grpc.enabled` | bool | `true` | 是否启用 gRPC |
| `grpc.host` | string | `"0.0.0.0"` | 监听地址 |
| `grpc.port` | int | `9090` | 监听端口 |
| `grpc.max_message_size` | int | `4194304` | 最大消息大小（4MB） |
| `grpc.keepalive_time` | duration | `30s` | 连接保活时间 |
| `grpc.keepalive_timeout` | duration | `5s` | 保活超时 |

### WebSocket

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `websocket.enabled` | bool | `true` | 是否启用 WebSocket |
| `websocket.host` | string | `"0.0.0.0"` | 监听地址 |
| `websocket.port` | int | `8081` | 监听端口 |
| `websocket.max_connections` | int | `100` | 最大连接数 |
| `websocket.read_buffer_size` | int | `1024` | 读缓冲区大小（字节） |
| `websocket.write_buffer_size` | int | `1024` | 写缓冲区大小（字节） |

---

## 🌐 Node 配置（P2P 网络）

### 连接管理

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `node.connectivity.min_peers` | int | `8` | 最小连接节点数 |
| `node.connectivity.max_peers` | int | `50` | 最大连接节点数 |
| `node.connectivity.low_water` | int | `10` | 连接管理低水位 |
| `node.connectivity.high_water` | int | `25` | 连接管理高水位 |
| `node.connectivity.grace_period` | duration | `20s` | 连接优雅关闭期 |
| `node.connectivity.enable_nat_port` | bool | `true` | 启用 NAT 端口映射 |
| `node.connectivity.enable_dcutr` | bool | `true` | 启用 DCUtR 打洞 |
| `node.connectivity.enable_auto_relay` | bool | `false` | 启用自动中继 |

### 节点发现

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `node.discovery.bootstrap_peers` | []string | `[...]` | 引导节点列表 |
| `node.discovery.mdns.enabled` | bool | `true` | 启用 mDNS 发现 |
| `node.discovery.mdns.service_name` | string | `"weisyn-node"` | mDNS 服务名称 |
| `node.discovery.dht.enabled` | bool | `true` | 启用 DHT 发现 |
| `node.discovery.dht.mode` | string | `"auto"` | DHT 模式（client/server/auto） |
| `node.discovery.discovery_interval` | duration | `20s` | 发现间隔 |
| `node.discovery.advertise_interval` | duration | `300s` | 广播间隔 |

### 主机配置

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `node.host.listen_addresses` | []string | `["/ip4/0.0.0.0/tcp/4001", ...]` | 监听地址列表 |
| `node.host.transport.enable_tcp` | bool | `true` | 启用 TCP 传输 |
| `node.host.transport.enable_quic` | bool | `true` | 启用 QUIC 传输 |
| `node.host.transport.enable_websocket` | bool | `false` | 启用 WebSocket 传输 |
| `node.host.security.enable_tls` | bool | `true` | 启用 TLS |
| `node.host.security.enable_noise` | bool | `true` | 启用 Noise 协议 |

---

## ⛓️ Blockchain 配置

### 基础链配置

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `blockchain.chain_id` | uint64 | `2` | 链ID（测试网） |
| `blockchain.network_id` | uint64 | `2` | 网络ID |
| `blockchain.node_mode` | string | `"full"` | 节点模式（light/full） |

### 区块配置

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `blockchain.block.max_block_size` | uint64 | `2097152` | 最大区块大小（2MB） |
| `blockchain.block.max_transactions` | int | `1000` | 最大交易数 |
| `blockchain.block.block_time_target` | int | `10` | 目标出块时间（秒） |
| `blockchain.block.min_block_interval` | int | `10` | 最小区块间隔（秒） |
| `blockchain.block.min_difficulty` | uint64 | `1` | 最小难度 |
| `blockchain.block.max_time_drift` | int | `300` | 最大时间偏差（秒） |
| `blockchain.block.validation_timeout` | duration | `30s` | 验证超时 |
| `blockchain.block.cache_size` | int | `1000` | 区块缓存数量 |

### 交易配置

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `blockchain.transaction.max_transaction_size` | uint64 | `65536` | 最大交易大小（64KB） |
| `blockchain.transaction.base_fee_per_byte` | uint64 | `10` | 基础字节费率 |
| `blockchain.transaction.minimum_fee` | uint64 | `1000` | 最低费用 |
| `blockchain.transaction.maximum_fee` | uint64 | `1000000` | 最高费用 |
| `blockchain.transaction.dust_threshold` | float64 | `0.00001` | 粉尘阈值 |
| `blockchain.transaction.cache_size` | int | `10000` | 交易缓存数量 |
| `blockchain.transaction.congestion_multiplier` | float64 | `1.5` | 拥堵系数 |

### 执行配置（ISPC）

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `blockchain.execution.ispc.resource_limits.execution_timeout_seconds` | int | `60` | 执行超时时间（秒） |
| `blockchain.execution.ispc.resource_limits.max_memory_mb` | int | `512` | 最大内存限制（MB） |
| `blockchain.execution.ispc.resource_limits.max_trace_size_mb` | int | `10` | 最大执行轨迹大小（MB） |
| `blockchain.execution.ispc.resource_limits.max_temp_storage_mb` | int | `100` | 最大临时存储（MB） |
| `blockchain.execution.ispc.resource_limits.max_host_function_calls` | uint32 | `10000` | 最大宿主函数调用次数 |
| `blockchain.execution.ispc.resource_limits.max_utxo_queries` | uint32 | `1000` | 最大UTXO查询次数 |
| `blockchain.execution.ispc.resource_limits.max_resource_queries` | uint32 | `1000` | 最大资源查询次数 |
| `blockchain.execution.ispc.resource_limits.max_concurrent_executions` | int | `100` | 最大并发执行数 |

---

## ⚙️ Consensus 配置

### 基础共识配置

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `consensus.consensus_type` | string | `"pow"` | 共识类型（pow/pos/poa/pbft） |
| `consensus.target_block_time` | duration | `10s` | 目标出块时间 |
| `consensus.block_size_limit` | uint64 | `2097152` | 区块大小限制（2MB） |

### 矿工配置

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `consensus.miner.mining_timeout` | duration | `5m` | 挖矿超时时间 |
| `consensus.miner.loop_interval` | duration | `100ms` | 挖矿循环间隔 |
| `consensus.miner.max_transactions` | uint32 | `1000` | 每个区块最大交易数 |
| `consensus.miner.min_transactions` | uint32 | `0` | 每个区块最小交易数 |
| `consensus.miner.tx_selection_mode` | string | `"priority"` | 交易选择模式 |
| `consensus.miner.max_cpu_usage` | float64 | `0.8` | 最大CPU使用率（80%） |
| `consensus.miner.max_memory_usage` | uint64 | `1073741824` | 最大内存使用量（1GB） |
| `consensus.miner.max_goroutines` | int | `8` | 最大协程数 |

### 聚合器配置

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `consensus.aggregator.enable_aggregator` | bool | `true` | 启用聚合器功能（生产环境必须） |
| `consensus.aggregator.min_peer_threshold` | int | `3` | 最小节点阈值（生产环境 >= 3） |
| `consensus.aggregator.max_candidates` | int | `10` | 最大候选区块数量 |
| `consensus.aggregator.collection_timeout` | duration | `60s` | 收集超时时间 |
| `consensus.aggregator.selection_interval` | duration | `5s` | 选择间隔时间 |

### PoW 配置

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `consensus.pow.initial_difficulty` | uint64 | `1000` | 初始难度 |
| `consensus.pow.min_difficulty` | uint64 | `1` | 最小难度 |
| `consensus.pow.max_difficulty` | uint64 | `0x1d00ffff` | 最大难度 |
| `consensus.pow.difficulty_window` | int | `100` | 难度调整窗口（区块数） |
| `consensus.pow.difficulty_adjustment_factor` | float64 | `4.0` | 难度调整因子 |
| `consensus.pow.worker_count` | int | `4` | 挖矿线程数 |
| `consensus.pow.enable_parallel` | bool | `true` | 启用并行挖矿 |

---

## 💾 Storage 配置

### BadgerDB（默认）

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `storage.badger.path` | string | `"./data/badger"` | 数据库存储路径 |
| `storage.badger.sync_writes` | bool | `true` | 同步写入（数据安全性） |
| `storage.badger.mem_table_size` | int64 | `134217728` | 内存表大小（128MB） |
| `storage.badger.enable_auto_compaction` | bool | `true` | 启用自动压缩 |

### SQLite

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `storage.sqlite.path` | string | `"./data/sqlite"` | 数据库文件路径 |
| `storage.sqlite.enable_wal` | bool | `true` | 启用 WAL 模式 |

### File Storage

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `storage.file.path` | string | `"./data/files"` | 文件存储路径 |
| `storage.file.max_file_size` | uint64 | `104857600` | 最大文件大小（100MB） |

---

## 📋 配置示例

### 完整配置示例（JSON）

```json
{
  "api": {
    "http": {
      "enabled": true,
      "port": 8080,
      "enable_rest": true,
      "enable_jsonrpc": true,
      "enable_websocket": true
    },
    "grpc": {
      "enabled": true,
      "port": 9090
    }
  },
  "node": {
    "connectivity": {
      "min_peers": 8,
      "max_peers": 50
    },
    "discovery": {
      "mdns": {
        "enabled": true
      },
      "dht": {
        "enabled": true,
        "mode": "auto"
      }
    }
  },
  "blockchain": {
    "chain_id": 2,
    "block": {
      "max_block_size": 2097152,
      "block_time_target": 10
    },
    "transaction": {
      "max_transaction_size": 65536,
      "base_fee_per_byte": 10
    }
  },
  "consensus": {
    "consensus_type": "pow",
    "miner": {
      "max_transactions": 1000
    },
    "aggregator": {
      "enable_aggregator": true,
      "min_peer_threshold": 3
    }
  },
  "storage": {
    "badger": {
      "path": "./data/badger",
      "sync_writes": true
    }
  }
}
```

---

## 📚 相关文档

- [API 参考](../api/index.md) - API 接口文档
- [CLI 参考](../cli/index.md) - 命令行工具文档
- [配置源码](../../../internal/config/) - 配置模块源码

---

**相关文档**：
- [产品总览](../../overview.md) - 了解 WES 是什么、核心价值、应用场景
- [部署指南](../../tutorials/deployment/) - 了解部署配置
