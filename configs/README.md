# WES 配置文件说明

## 📁 目录结构

```
configs/
├── development/                 # 开发环境配置
│   ├── single/
│   │   └── config.json         # 单节点开发配置
│   └── cluster/
│       ├── node1.json          # 集群节点1配置
│       └── node2.json          # 集群节点2配置
├── testing/
│   └── config.json             # 测试环境配置
├── production/
│   └── config.json             # 生产环境配置
└── README.md                   # 本文件
```

## 🎯 统一配置设计 (v0.0.1+)

### **配置统一原则**
从v0.0.1开始，所有配置文件采用统一结构设计，区分用户可配置和系统内部化设置：

| 配置类别 | 状态 | 配置项数量 | 说明 |
|---------|------|------------|------|
| **用户交互配置** | ✅ 保留 | ~25项 | 网络连接、端口、挖矿、引导节点等用户需要调整的设置 |
| **内部技术细节** | ❌ 内部化 | ~68项 | 存储引擎参数、协议细节、性能调优、安全策略 |

### **统一配置结构**
所有配置文件（development、testing、production、cluster）都采用以下标准结构：

#### **✅ 保留的用户配置**

**📡 网络身份配置**
- **network**: `chain_id`, `network_name` - 区块链网络标识

**👤 创世配置**
- **genesis**: 初始账户列表和余额分配

**🌐 API服务配置**
- **api**: HTTP/gRPC/WebSocket启用状态和端口配置
- **enable_mining_api**: 挖矿API开关

**⛏️ 挖矿配置**
- **mining**: 出块时间、聚合器开关、挖矿线程数

**🔗 网络节点配置**
- **node**: P2P监听地址、引导节点列表、网络发现开关、身份配置
  - `listen_addresses`: 用户可配置不同端口避免冲突
  - `bootstrap_peers`: 用户可添加私有引导节点
  - `enable_mdns`, `enable_dht`, `enable_nat_port`: 不同环境需要不同网络发现策略
  - `host.identity`: P2P网络身份配置（可选，自动生成）

**📝 运维配置**
- **log**: 日志级别和文件路径
- **storage**: 数据存储路径

#### **❌ 内部化的技术配置**
- **存储引擎细节**: value_log_file_size, sync_writes, auto_compaction, cache_size
- **协议传输配置**: enable_tcp, enable_quic, enable_tls, enable_noise
- **性能调优参数**: 超时时间、批处理配置、缓存策略
- **安全策略配置**: CORS设置、限流策略、消息大小限制
- **交易池细节**: RBF配置、队列管理、冲突解决策略
- **执行引擎参数**: 资源限制、WASM优化配置

### **配置参数统一对照表**

| 配置类别 | 旧结构字段 | 新统一结构 | 用户配置 |
|---------|-----------|-----------|----------|
| **网络标识** | `genesis.chain_id` + `blockchain.chain_id` | `network.chain_id` | ✅ |
| **网络名称** | `genesis.network_id` | `network.network_name` | ✅ |
| **API端口** | `api.http.port` + `api.grpc.port` | `api.http_port` + `api.grpc_port` | ✅ |
| **API开关** | `api.http.enabled` | `api.http_enabled` | ✅ |
| **挖矿设置** | `consensus.miner.*` + `consensus.aggregator.*` | `mining.*` | ✅ |
| **P2P监听** | `node.listen_addresses` | `node.listen_addresses` | ✅ |
| **引导节点** | `node.bootstrap_peers` | `node.bootstrap_peers` | ✅ |
| **网络发现** | `node.enable_mdns/dht` | `node.enable_mdns/dht` | ✅ |
| **日志配置** | `log.level` + `log.file_path` | `log.level` + `log.file_path` | ✅ |
| **存储路径** | `storage.badger.path` | `storage.data_path` | ✅ |
| **存储引擎** | `storage.type` + `storage.badger.*` | *内部化* | ❌ |
| **传输协议** | `node.transport.*` + `node.security.*` | *内部化* | ❌ |
| **性能参数** | `storage.cache_size` + 各种timeout | *内部化* | ❌ |
| **RBF策略** | `txpool.rbf.*` | *内部化* | ❌ |

### **统一配置文件结构示例**

```json
{
  "_comment": "WES配置文件 - 统一版本",
  "_environment": "环境标识",
  "_version": "0.0.1",
  
  "network": {
    "chain_id": 网络链ID,
    "network_name": "网络名称"
  },
  
  "genesis": {
    "accounts": [
      {
        "name": "账户名称",
        "private_key": "私钥",
        "address": "地址",
        "initial_balance": "初始余额"
      }
    ]
  },
  
  "api": {
    "http_enabled": true,
    "http_port": 8080,
    "grpc_enabled": true,
    "grpc_port": 9090,
    "websocket_enabled": true,
    "websocket_port": 8081,
    "enable_mining_api": true
  },
  
  "mining": {
    "target_block_time": "出块时间",
    "enable_aggregator": true,
    "max_mining_threads": 挖矿线程数
  },
  
  "node": {
    "listen_addresses": [
      "P2P监听地址列表"
    ],
    "bootstrap_peers": [
      "引导节点列表"
    ],
    "host": {
      "identity": {
        "key_file": "P2P身份密钥文件路径（可选）",
        "private_key": "base64编码的libp2p私钥（可选）"
      }
    },
    "enable_mdns": true,
    "enable_dht": true,
    "enable_nat_port": true,
    "enable_dcutr": true,
    "enable_auto_relay": true
  },
  
  "log": {
    "level": "日志级别",
    "file_path": "日志文件路径"
  },
  
  "storage": {
    "data_path": "数据存储路径"
  }
}
```

### **环境隔离**
- **development**: 开发环境（内测），使用快速出块、低难度，供开发团队内部测试使用
- **testing**: 测试环境（公测），模拟生产环境参数，供外部用户公开测试体验
- **production**: 生产环境（主网），严格的安全和性能设置，正式运行的区块链网络

## 🚀 使用方法

### **WES双层接口设计**

WES提供双层用户接口，满足不同用户需求：

| **接口层** | **目标用户** | **主要功能** | **使用场景** |
|-----------|-------------|-------------|-------------|
| **API层** | 服务器/开发者 | 无状态区块链接口 | 企业后端、第三方集成、批量处理 |
| **CLI层** | 个人用户 | 本地钱包+交互界面 | 个人管理、开发测试、本地操作 |

### **🎯 环境专用启动架构**

#### **🔧 开发环境 - ./bin/development**
```bash
# 完整功能启动（CLI交互 + API服务）
./bin/development                           # 自动加载 configs/development/single/config.json

# 仅API服务（后端开发）
./bin/development --api-only                # 纯API服务，适合脚本对接

# 仅CLI交互（个人用户）  
./bin/development --cli-only                # 本地钱包管理，个人操作

# 源码调试
go run ./cmd/development                    # 开发阶段调试
```

#### **🧪 测试环境 - ./bin/testing**
```bash
# 测试环境启动
./bin/testing                               # 自动加载 configs/testing/config.json

# 测试API服务（CI/CD推荐）
./bin/testing --api-only                    # 自动化测试、集成验证

# 源码测试
go run ./cmd/testing                        # 测试阶段调试
```

#### **🚀 生产环境 - ./bin/production**
```bash
# 生产环境启动
./bin/production                            # 自动加载 configs/production/config.json

# 生产API服务（企业部署推荐）
./bin/production --api-only                 # 服务器后台运行，企业部署

# 源码生产（不推荐）
go run ./cmd/production                     # 仅用于生产环境调试
```

### **✨ 架构优势对比**

| **传统方式** | **环境专用方式** | **核心优势** |
|-------------|-----------------|-------------|
| `./bin/node -env=development` | `./bin/development` | ✅ 零参数启动，消除环境参数混乱 |
| 运行时环境切换 | 编译时环境绑定 | ✅ 避免生产环境误用开发配置 |
| 配置文件参数化 | 配置路径硬编码 | ✅ 消除配置文件路径错误 |
| 单一二进制文件 | 环境专用二进制 | ✅ 部署简化，仅需拷贝对应文件 |

### **🔧 典型使用组合**

#### **个人用户推荐**
```bash
# 1. 首次使用 - 创建钱包
go run ./cmd/node -env=development
> 选择: 钱包管理 → 创建钱包

# 2. 日常使用 - 快速查余额  
go run ./cmd/node --cli balance

# 3. 转账操作 - 交互式界面
go run ./cmd/node --cli transfer
```

#### **开发者推荐**
```bash
# 1. 开发测试 - 单节点环境
go run ./cmd/node -env=development

# 2. 多节点测试 - 集群环境
# 终端1
go run ./cmd/node -config=configs/development/cluster/node1.json
# 终端2  
go run ./cmd/node -config=configs/development/cluster/node2.json

# 3. 自动化测试 - 脚本模式
go run ./cmd/node --cli status > status.txt
```

#### **企业部署推荐**
```bash
# 1. 测试环境部署
go run ./cmd/node --daemon -env=testing

# 2. 生产环境部署
go run ./cmd/node --daemon -env=production  

# 3. 企业应用集成（通过API）
curl -X POST "http://localhost:8080/api/v1/transactions/transfer" \
  -H "Content-Type: application/json" \
  -d '{
    "sender_private_key": "企业密钥管理系统提供",
    "to_address": "recipient_address", 
    "amount": "1000.0"
  }'
```

### **📱 CLI工具使用提示**

当使用交互模式时，CLI提供以下功能：

- **💳 钱包管理**：创建、导入、删除本地钱包
- **💰 余额查询**：查看所有钱包余额
- **🔄 转账操作**：安全的私钥管理+转账执行  
- **📊 状态监控**：节点状态、挖矿状态、网络状态
- **⛏️ 挖矿控制**：启动/停止挖矿，查看算力
- **🌐 网络管理**：查看连接节点，网络信息

### **🔑 私钥管理说明**

**API层使用**（企业/开发者）：
- 需要自行管理私钥（企业KMS、代理托管等）
- 调用时传入私钥：`TransferAsset(privateKey, ...)`

**CLI层使用**（个人用户）：
- CLI帮助管理本地钱包和私钥  
- 密码保护、加密存储
- 用户只需记住钱包密码

### **环境配置选择**
```bash
# 指定环境启动
go run ./cmd/node -env=development    # 开发单节点
go run ./cmd/node -env=testing        # 测试环境
go run ./cmd/node -env=production     # 生产环境

# 指定具体配置文件
go run ./cmd/node -config=configs/development/single/config.json
go run ./cmd/node -config=configs/development/cluster/node1.json
```

## ⚙️ 配置详解

### **开发环境配置特点（内测）**
- **单节点模式**:
  - 端口: HTTP(8080), gRPC(9090), WebSocket(8081)
  - 快速出块: 5秒
  - 低挖矿难度: 1-1000
  - 本地访问: 127.0.0.1
  - 供开发团队内部快速迭代和调试

- **集群模式**:
  - 节点1: 端口8080/9090/8081, P2P端口4001
  - 节点2: 端口8082/9091/8083, P2P端口4002
  - 支持聚合器功能测试
  - 适用于多节点功能验证

### **测试环境配置特点（公测）**
- 模拟生产环境参数
- 出块时间: 15秒
- 中等挖矿难度: 100-10M
- 支持外部访问: 0.0.0.0
- 启用完整P2P功能
- 供外部用户公开测试体验和反馈

### **生产环境配置特点（主网）**
- 严格安全设置
- 出块时间: 10分钟
- 高挖矿难度: 1M-100B
- 仅本地API访问: 127.0.0.1
- 禁用内网P2P连接
- 关闭挖矿API
- 正式运行的区块链网络，具有真实经济价值

## 🛡️ 合规系统配置

### **环境感知安全控制**

WES合规系统采用**环境感知**设计，根据 `blockchain.network_type` 自动控制启用状态：

| 环境类型 | network_type | 合规状态 | 说明 |
|---------|-------------|----------|------|
| 开发环境（内测） | `"development"` | 🔴 **自动禁用** | 供开发团队内部测试，便于开发调试，无合规限制 |
| 测试环境（公测） | `"testnet"` | 🔴 **自动禁用** | 供外部用户公开测试，便于功能验证，无合规限制 |
| 生产环境（主网） | `"mainnet"` | 🛡️ **强制启用** | 正式运行的区块链网络，严格合规控制，包含16个禁用国家+22个禁用操作 |

### **设计原则**

1. **系统级安全控制**：
   - 合规启用/禁用完全由系统根据环境决定
   - 用户配置文件无法绕过系统级安全限制
   - 开发和生产环境安全策略完全分离

2. **硬编码安全规则**：
   - 16个不可绕过的禁用国家（基于UN/OFAC/FATF制裁清单）
   - 22个不可绕过的禁用操作（高风险DeFi、隐私、跨链等）
   - 详细清单见：`internal/config/compliance/defaults.go`

3. **无需用户配置**：
   - 开发/测试：零配置，自动适配
   - 生产环境：自动启用，仅需配置外部服务（可选）

### **外部服务配置**

仅在**生产环境**需要配置外部合规服务（可选）：

```json
"_compliance_external_services": {
  "identity_provider": {
    "url": "https://your-identity-provider.com",
    "public_key_pem": "-----BEGIN PUBLIC KEY-----\n...\n-----END PUBLIC KEY-----"
  },
  "geoip": {
    "database_path": "./data/production/compliance/GeoLite2-Country.mmdb",
    "update_url": "https://download.maxmind.com/..."
  }
}
```

## 📊 数据目录映射

每个配置文件对应的数据存储位置：

| 配置文件 | 数据目录 | 说明 |
|---------|----------|------|
| `development/single/config.json` | `./data/development/single/` | 开发单节点数据 |
| `development/cluster/node1.json` | `./data/development/cluster/node1/` | 开发集群节点1数据 |
| `development/cluster/node2.json` | `./data/development/cluster/node2/` | 开发集群节点2数据 |
| `testing/config.json` | `./data/testing/` | 测试环境数据 |
| `production/config.json` | `./data/production/` | 生产环境数据 |

## 🔧 配置修改指南

### **网络配置**
- HTTP API端口: `api.http.port`
- gRPC API端口: `api.grpc.port`  
- WebSocket端口: `api.websocket.port`
- P2P监听地址: `node.listen_addresses`
- 引导节点: `node.bootstrap_peers`

### **挖矿配置**
- 出块时间: `consensus.target_block_time`
- 挖矿难度: `consensus.pow.initial_difficulty`
- 区块奖励: `consensus.miner.base_block_reward`

### **存储配置**
- 数据路径: `storage.data_path`
- 日志路径: `log.file_path`

### **身份与密钥管理**
- P2P身份密钥: `node.host.identity.key_file` 或 `node.host.identity.private_key`
- CLI钱包路径: `cli.wallet_storage_path` (相对于storage.data_path)

## 🔐 身份与密钥管理指南

### **P2P网络身份**

WES节点使用libp2p网络身份系统，每个节点需要唯一的身份密钥用于网络通信：

**🎯 身份配置优先级：**
1. **配置中的私钥** (`node.host.identity.private_key`): base64编码的libp2p私钥
2. **密钥文件** (`node.host.identity.key_file`): 私钥文件路径
3. **自动生成**: 基于数据目录自动生成并持久化

**📁 默认路径规则：**
- 单节点: `<storage.data_path>/p2p/identity.key`
- 集群节点: 各节点使用独立的数据目录，确保身份隔离

**示例配置：**
```json
{
  "node": {
    "host": {
      "identity": {
        "key_file": "./data/development/cluster/node1/p2p/identity.key"
      }
    }
  },
  "storage": {
    "data_path": "./data/development/cluster/node1"
  }
}
```

### **链上账户与钱包**

链上账户用于交易签名和挖矿奖励接收，与P2P身份完全独立：

**🏦 钱包存储：**
- CLI钱包路径: `<storage.data_path>/wallets/` (基于配置中的`cli.wallet_storage_path`)
- 每个节点的钱包完全隔离，避免共享

**⛏️ 挖矿地址使用：**
- 挖矿API要求显式提供矿工地址，不使用默认地址
- 开发环境可使用genesis配置中的测试地址
- 生产环境必须使用独立生成的钱包地址

**示例挖矿请求：**
```bash
curl -X POST http://localhost:8080/api/v1/mining/start \
  -H "Content-Type: application/json" \
  -d '{"miner_address": "CUQ3g6P5WmFN289pPn7AAhnQ3T2cZRv2BR"}'
```

### **安全最佳实践**

1. **身份隔离**：
   - 每个节点使用独立的P2P身份密钥
   - 不同环境使用不同的数据目录

2. **密钥管理**：
   - P2P身份密钥自动生成，无需手动管理
   - 链上钱包私钥通过CLI安全创建和管理
   - 生产环境不在配置文件中存储私钥

3. **路径配置**：
   - 使用相对路径配置，基于`storage.data_path`自动解析
   - 确保不同节点使用不同的数据目录

## ⚠️ 注意事项

1. **生产环境安全**：
   - 生产环境配置包含占位符，部署前必须替换真实值
   - 私钥信息需要单独管理，不要写入配置文件

2. **网络隔离**：
   - 不同环境使用不同的chain_id，防止网络混合
   - 开发环境使用本地网络，避免连接外部节点

3. **数据目录**：
   - 数据目录由代码自动创建，无需手动创建
   - 不同环境数据完全隔离，可安全清理开发数据

4. **配置修改**：
   - 修改配置后需要重启节点
   - 核心参数（如chain_id）修改可能导致数据不兼容

## 🌐 网络发现配置

### **引导节点列表 (8个节点)**

所有环境都配置了相同的引导节点，确保最佳的网络连接性：

**官方DNS引导节点 (4个):**
- `/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN`
- `/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa`
- `/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb`
- `/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt`

**美国节点 (1个):**
- `/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ`

**亚洲节点 (3个):**
- `/ip4/8.130.32.119/tcp/4001/p2p/QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5N` (阿里云)
- `/ip4/47.245.56.181/tcp/4001/p2p/QmZa1sAxajnQjVM8WjWXoMbmPd7NsWhfKsPkErzpm9wGkp` (阿里云)
- `/ip4/47.103.15.12/tcp/4001/p2p/QmUbootABQNHKeyjHdrheq1oVzGdFZxB1oTHMZAcD5iWdH` (阿里云)

### **网络发现机制**

- **mDNS**: 局域网自动发现 (development环境启用)
- **DHT**: 分布式哈希表发现 (所有环境启用)  
- **AutoRelay**: 自动中继穿越NAT (production/testing环境启用)
- **Bootstrap**: 引导节点发现 (所有环境使用相同节点列表)

### **连接性优化**

**开发环境:**
- mDNS启用，支持局域网零配置发现
- 监听0.0.0.0，允许外部连接

**测试/生产环境:**
- AutoRelay启用，改善NAT穿越
- 亚洲节点提供更低延迟连接
- 多地理分布确保连接可靠性