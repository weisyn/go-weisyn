# Query Module - 统一查询服务

## 📋 模块概述

`internal/core/query` 实现了 WES 系统的统一查询服务，提供只读的 CQRS 读路径。

### 🎯 核心职责

- 实现 `pkg/interfaces/query.QueryService` 接口
- 聚合所有领域的查询服务
- 提供统一的只读查询入口
- 避免循环依赖

### 🏗️ 架构设计

```
QueryService (统一查询服务)
    ├─ ChainQuery (链状态查询)
    ├─ BlockQuery (区块查询)
    ├─ TxQuery (交易查询)
    ├─ UTXOQuery (EUTXO查询)
    ├─ ResourceQuery (资源查询)
    └─ AccountQuery (账户查询)
```

### 三层架构规范

本模块严格遵循 `docs/system/standards/principles/code-organization.md` 定义的三层架构规范：

```
┌─────────────────────────────────────────────────────────────────┐
│ pkg/interfaces/persistence/                                      │
│ 📦 公共接口层 (Public Interfaces)                                 │
│                                                                 │
│  ├─ query.go       ➜ QueryService (统一查询服务接口)             │
│  ├─ chain.go       ➜ ChainQuery                                │
│  ├─ block.go       ➜ BlockQuery                                │
│  ├─ tx.go          ➜ TxQuery                                   │
│  ├─ eutxo.go       ➜ UTXOQuery                                │
│  ├─ resource.go    ➜ ResourceQuery                             │
│  └─ account.go     ➜ AccountQuery                              │
└─────────────────────────────────────────────────────────────────┘
                            ↑ 嵌入/继承
┌─────────────────────────────────────────────────────────────────┐
│ internal/core/persistence/query/interfaces/                    │
│ 🔧 内部接口层 (Internal Interfaces) - 必需                       │
│                                                                 │
│  ├─ query.go       ➜ InternalQueryService  ✅                  │
│  ├─ chain.go       ➜ InternalChainQuery    ✅                  │
│  ├─ block.go       ➜ InternalBlockQuery    ✅                  │
│  ├─ tx.go          ➜ InternalTxQuery       ✅                  │
│  ├─ eutxo.go       ➜ InternalUTXOQuery     ✅                  │
│  ├─ resource.go    ➜ InternalResourceQuery ✅                  │
│  └─ account.go     ➜ InternalAccountQuery   ✅                  │
└─────────────────────────────────────────────────────────────────┘
                            ↑ 实现
┌─────────────────────────────────────────────────────────────────┐
│ internal/core/persistence/query/                                │
│ 📄 实现层 (Implementation Layer) - 所有实现都在子目录中          │
│                                                                 │
│  ├─ aggregator/service.go ➜ Service (implements InternalQueryService) ✅│
│  ├─ chain/service.go      ➜ Service (implements InternalChainQuery)   ✅│
│  ├─ block/service.go      ➜ Service (implements InternalBlockQuery)   ✅│
│  ├─ tx/service.go         ➜ Service (implements InternalTxQuery)      ✅│
│  ├─ eutxo/service.go      ➜ Service (implements InternalUTXOQuery)    ✅│
│  ├─ resource/service.go   ➜ Service (implements InternalResourceQuery)✅│
│  └─ account/service.go    ➜ Service (implements InternalAccountQuery) ✅│
└─────────────────────────────────────────────────────────────────┘
                            ↑ 装配/绑定
┌─────────────────────────────────────────────────────────────────┐
│ internal/core/persistence/query/module.go                       │
│ 🔌 依赖注入配置 (Dependency Injection)                          │
└─────────────────────────────────────────────────────────────────┘
```

**关键原则**：
- ✅ 公共接口层：定义外部使用的接口
- ✅ 内部接口层：定义实现层之间的协作接口
- ✅ 实现层：实现内部接口，通过 fx 导出公共接口
- ✅ 依赖注入：通过 fx 统一管理依赖关系

## 📦 模块结构

```
internal/core/query/
├── service.go              # 统一查询服务主实现
├── module.go               # fx 模块配置
├── README.md               # 本文档
├── chain/
│   └── service.go          # 链状态查询实现
├── block/
│   └── service.go          # 区块查询实现
├── tx/
│   └── service.go          # 交易查询实现
├── eutxo/
│   └── service.go          # EUTXO查询实现
├── resource/
│   └── service.go          # 资源查询实现
└── account/
    └── service.go          # 账户查询实现
```

## 🔧 设计原则

### 1. CQRS 架构

- **只读操作**：所有方法都是查询操作，不修改状态
- **读写分离**：查询服务独立于写服务
- **性能优化**：支持缓存和索引优化

### 2. 避免循环依赖

- **统一入口**：所有模块通过 QueryService 查询
- **接口隔离**：通过接口隔离模块依赖
- **单向依赖**：业务模块 → QueryService（不反向）

### 3. 聚合模式

- **领域分离**：每个领域查询服务独立实现
- **组合聚合**：通过组合提供完整查询能力
- **接口委托**：统一服务委托到领域服务

## 📝 使用方式

### 1. 依赖注入

```go
type MyService struct {
    query query.QueryService
}

func NewMyService(queryService query.QueryService) *MyService {
    return &MyService{query: queryService}
}
```

### 2. 查询示例

```go
// 查询链信息
chainInfo, err := s.query.GetChainInfo(ctx)

// 查询区块
block, err := s.query.GetBlockByHeight(ctx, height)

// 查询交易
_, _, tx, err := s.query.GetTransaction(ctx, txHash)

// 查询UTXO
utxo, err := s.query.GetUTXO(ctx, outpoint)

// 查询资源
resource, err := s.query.GetResourceByContentHash(ctx, contentHash)

// 查询账户余额
balance, err := s.query.GetAccountBalance(ctx, address, tokenID)
```

## 🔍 接口说明

### ChainQuery - 链状态查询

- `GetChainInfo`: 获取链基础信息
- `GetCurrentHeight`: 获取当前链高度
- `GetBestBlockHash`: 获取最佳区块哈希
- `GetNodeMode`: 获取节点模式
- `IsDataFresh`: 检查数据新鲜度
- `IsReady`: 检查系统就绪状态

### BlockQuery - 区块查询

- `GetBlockByHeight`: 按高度获取区块
- `GetBlockByHash`: 按哈希获取区块
- `GetBlockHeader`: 获取区块头
- `GetBlockRange`: 获取区块范围
- `GetHighestBlock`: 获取最高区块信息

### TxQuery - 交易查询

- `GetTransaction`: 根据交易哈希获取完整交易
- `GetTxBlockHeight`: 获取交易所在的区块高度
- `GetBlockTimestamp`: 获取指定高度的区块时间戳
- `GetAccountNonce`: 获取账户当前nonce
- `GetTransactionsByBlock`: 获取区块中的所有交易

### UTXOQuery - EUTXO查询

- `GetUTXO`: 根据OutPoint精确获取UTXO
- `GetUTXOsByAddress`: 获取地址拥有的UTXO列表
- `GetSponsorPoolUTXOs`: 获取赞助池UTXO列表
- `GetCurrentStateRoot`: 获取当前UTXO状态根

### ResourceQuery - 资源查询

- `GetResourceByContentHash`: 根据内容哈希查询完整资源
- `GetResourceFromBlockchain`: 从区块链获取资源元信息
- `GetResourceTransaction`: 获取资源关联的交易信息
- `CheckFileExists`: 检查本地文件是否存在
- `BuildFilePath`: 构建本地文件路径
- `ListResourceHashes`: 列出所有资源哈希

### AccountQuery - 账户查询（聚合视图）

- `GetAccountBalance`: 获取账户余额（聚合所有UTXO）

## 🔄 数据流

```
业务模块
    ↓ 查询请求
QueryService
    ↓ 委托
领域查询服务 (Chain/Block/Tx/UTXO/Resource/Account)
    ↓ 读取
存储层 (BadgerStore)
    ↓ 返回
业务模块
```

## 🗂️ 存储键规范

本模块的查询键空间以**当前实现**为准（历史文档与实现可能存在偏差）。派生数据全量盘点见：
`_dev/02-架构设计-architecture/10-数据与存储架构-data-and-storage/04-DERIVED_DATA_INVENTORY.md`。

### 1. 链状态 (ChainQuery)

| 功能 | 键格式 | 值格式 | 说明 |
|------|--------|--------|------|
| 链尖状态 | `state:chain:tip` | `height(8字节) + blockHash(32字节)` | 存储当前链的最高区块信息 |
| 链状态根 | `state:chain:root` | `stateRoot(32字节)` | 链视角的状态根（通常与 UTXO 相关） |

### 2. 区块索引 (BlockQuery)

| 功能 | 键格式 | 值格式 | 说明 |
|------|--------|--------|------|
| 高度→区块定位信息 | `indices:height:{height}` | `blockHash(32) + filePathLen(1) + filePath(N) + fileSize(8)` | 用于定位 `blocks/...` 文件读取区块原文 |
| 哈希→高度 | `indices:hash:{blockHashHex}` | `height(8字节)` | 根据区块哈希反查高度 |
| 区块原文 | 文件：`blocks/{segment}/{height}.bin` | protobuf `Block` bytes | 区块原文落在 FileStore（文件系统），Badger 仅存索引 |

### 3. 交易索引 (TxQuery)

| 功能 | 键格式 | 值格式 | 说明 |
|------|--------|--------|------|
| 交易哈希→位置 | `indices:tx:{txHashHex}` | `blockHeight(8) + blockHash(32) + txIndex(4)` | 定位交易所在区块及索引 |
| 地址→Nonce | `indices:nonce:{address}` | `uint64(8字节)` | 账户的当前nonce值 |

### 4. UTXO 索引 (UTXOQuery)

| 功能 | 键格式 | 值格式 | 说明 |
|------|--------|--------|------|
| OutPoint→UTXO | `utxo:set:{txIdHex}:{outputIndex}` | protobuf: `UTXO` | UTXO 主集合（执行状态真相） |
| 地址→UTXO列表 | `index:address:{ownerHex}` | outpointList（实现当前为 36 bytes/entry：`txId(32)+outputIndex(4)`） | 地址索引（可重建） |
| 资产→UTXO列表 | `index:asset:{assetIdHex}` | outpointList（见 EUTXO index 编码） | 资产索引（可重建） |
| 高度→UTXO列表 | `index:height:{height}` | outpointList（见 EUTXO index 编码） | 高度索引（可重建） |
| UTXO 状态根 | `utxo_state_root` | `stateRoot(32字节)` | UTXO 视角状态根（与 `state:chain:root` 在快照恢复时同步） |

### 5. 资源索引 (ResourceQuery)

| 功能 | 键格式 | 值格式 | 说明 |
|------|--------|--------|------|
| 资源实例索引 | `indices:resource-instance:{instanceID}` | `blockHash(32) + blockHeight(8) + codeID(32)` | 实例维度索引（可重建） |
| 资源实例记录 | `resource:utxo-instance:{instanceID}` | JSON `ResourceUTXORecord` | 实例维度记录（可重建） |
| code→instances | `indices:resource-code:{codeIDHex}` | JSON `[]instanceID` | 代码到实例列表（可重建） |
| owner→instances | `index:resource:owner-instance:{ownerHex}:{instanceID}` | `instanceID` bytes | owner 反向索引（可重建，键数量大） |
| 资源历史 | `indices:resource:history:{contentHashHex}` | `txHashList(32*n) + lastUpdatedHeight(8)` | 资源历史交易索引（可重建） |

### 6. 账户聚合 (AccountQuery)

AccountQuery 不直接访问存储，而是通过聚合 `UTXOQuery` 提供账户余额视图。

## ⚠️ 注意事项

### 1. 只读原则

- 查询服务不修改任何状态
- 所有方法都是幂等的
- 支持并发调用

### 2. 性能考虑

- 高频查询方法需要优化
- 支持索引和缓存
- 避免全表扫描

### 3. 错误处理

- 查询不到返回错误，不返回nil
- 错误信息要明确
- 支持错误链追踪

## 🚀 后续优化

### 1. 索引优化

- 地址UTXO索引
- 资源哈希索引
- 交易位置索引

### 2. 缓存策略

- 热点数据缓存
- LRU缓存策略
- 缓存失效机制

### 3. 只读副本

- 支持路由到只读副本
- 读写分离
- 负载均衡

## 📚 相关文档

- [Public Interface Design](../../../docs/system/designs/interfaces/public-interface-design.md)
- [Query Interface Specification](../../../pkg/interfaces/query/README.md)
- [CQRS Architecture](../../../docs/system/designs/architecture/cqrs.md)

