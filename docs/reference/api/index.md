# WES API 参考

---

## 🎯 概述

WES 节点提供多种 API 接口供外部调用：

- **RESTful API**：基于 HTTP，提供区块查询、交易提交、状态查询等功能
- **JSON-RPC API**：遵循 JSON-RPC 2.0 规范，与以太坊生态兼容
- **WebSocket API**：支持实时事件订阅

**设计目标**：
- ✅ web3.js/ethers.js 直接可用
- ✅ 对标 Geth/Bitcoin Core
- ✅ 支持客户端签名模式
- ✅ 支持状态锚定查询
- ✅ 支持重组安全订阅

---

## 📍 连接方式

### RESTful API

**基础 URL**：
- 本地开发：`http://localhost:8080/api/v1`
- 生产环境：`https://api.weisyn.io/api/v1`

**示例**：
```bash
curl http://localhost:8080/api/v1/blocks/12345
```

### JSON-RPC API

**端点**：`http://localhost:8080/jsonrpc` 或 `http://localhost:8545`

**HTTP 请求示例**：
```bash
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "wes_blockNumber",
    "params": [],
    "id": 1
  }'
```

**WebSocket 订阅示例**：
```javascript
const ws = new WebSocket('ws://localhost:8545');

ws.send(JSON.stringify({
  jsonrpc: "2.0",
  method: "wes_subscribe",
  params: ["newHeads"],
  id: 1
}));
```

---

## 📚 RESTful API

### 区块查询

#### `GET /api/v1/blocks/{height}`

按区块高度查询区块详情。

**参数**：
- `height` (path, uint64) - 区块高度

**响应**：
- `200` - 成功返回区块信息
- `404` - 区块不存在

**状态锚定**：此接口支持状态锚定查询，响应中包含查询时的状态锚点信息。

### 交易操作

#### `POST /api/v1/transactions`

提交已签名交易到内存池。

**⚠️ 安全模型**：此接口仅接受已签名交易，不接受私钥。节点验证签名后加入内存池。

**请求体**：
```json
{
  "signedTx": "0xf86c808504a817c800825208..."
}
```

**响应**：
- `200` - 交易已接受
  ```json
  {
    "txHash": "0xabc123...",
    "status": "pending"
  }
  ```
- `400` - 交易被拒绝（费用过低、签名无效等）

### SPV 轻客户端支持

#### `GET /api/v1/spv/tx/{hash}/proof`

获取交易的 SPV Merkle 证明，用于轻客户端验证交易是否包含在区块中。

**参数**：
- `hash` (path, string) - 交易哈希

**响应**：
```json
{
  "txHash": "0xabc123...",
  "blockHash": "0xdef456...",
  "blockHeight": 12345,
  "merkleRoot": "0x...",
  "merkleProof": ["0x...", "0x..."],
  "index": 0
}
```

### 交易池策略

#### `GET /api/v1/txpool/policy`

查询节点的交易池策略参数，用于客户端估算交易费用和了解提交要求。

**响应**：
```json
{
  "minRelayFee": "1000",
  "minTip": "100",
  "maxTxSize": 1048576,
  "maxTxCount": 10000,
  "evictionPolicy": "fee_rate"
}
```

### 健康检查

#### `GET /api/v1/health`

完整健康检查，返回节点的完整健康状态。

#### `GET /api/v1/health/live`

存活检查（Liveness），仅检查进程是否响应。

#### `GET /api/v1/health/ready`

就绪检查（Readiness），检查节点是否已同步且可对外服务。

---

## 📚 JSON-RPC API

### 链信息

#### `net_version`

返回网络ID。

**参数**：无

**返回**：
```json
{
  "jsonrpc": "2.0",
  "result": "1",
  "id": 1
}
```

#### `wes_chainId`

返回链ID（十六进制）。

**参数**：无

**返回**：
```json
{
  "jsonrpc": "2.0",
  "result": "0x1",
  "id": 1
}
```

#### `wes_blockNumber`

返回最新区块高度。

**参数**：无

**返回**：
```json
{
  "jsonrpc": "2.0",
  "result": "0x1234",
  "id": 1
}
```

#### `wes_syncing`

返回同步状态。

**参数**：无

**返回（未同步）**：
```json
{
  "jsonrpc": "2.0",
  "result": false,
  "id": 1
}
```

**返回（同步中）**：
```json
{
  "jsonrpc": "2.0",
  "result": {
    "startingBlock": "0x0",
    "currentBlock": "0x1234",
    "highestBlock": "0x5678"
  },
  "id": 1
}
```

### 区块查询

#### `wes_getBlockByHeight`

按高度查询区块。

**参数**：
1. `height` (string) - 区块高度（十六进制）
2. `fullTx` (boolean) - 是否返回完整交易（否则仅返回哈希）

**示例**：
```json
{
  "jsonrpc": "2.0",
  "method": "wes_getBlockByHeight",
  "params": ["0x1234", false],
  "id": 1
}
```

#### `wes_getBlockByHash`

按哈希查询区块。

**参数**：
1. `hash` (string) - 区块哈希
2. `fullTx` (boolean) - 是否返回完整交易

### 交易操作

#### `wes_sendRawTransaction`

提交已签名交易。

**⚠️ 安全**：仅接受已签名交易，不接受私钥！

**参数**：
1. `signedTx` (string) - 十六进制编码的已签名交易

**返回（成功）**：
```json
{
  "jsonrpc": "2.0",
  "result": "0xtxhash...",
  "id": 1
}
```

**返回（失败）**：
```json
{
  "jsonrpc": "2.0",
  "error": {
    "code": -32003,
    "message": "Transaction fee too low",
    "data": {
      "providedFeeRate": "500",
      "minRequiredFeeRate": "1000"
    }
  },
  "id": 1
}
```

### 资源查询

#### `wes_getResourceByContentHash`

根据内容哈希查询资源元数据。

**参数**：
1. `content_hash` (string) - 资源内容哈希（十六进制）

**返回**：
```json
{
  "jsonrpc": "2.0",
  "result": {
    "content_hash": "0xabc123...",
    "name": "My Contract",
    "version": "1.0.0",
    "category": "RESOURCE_CATEGORY_EXECUTABLE",
    "executable_type": "EXECUTABLE_TYPE_CONTRACT",
    "resourceType": "contract",
    "owner": "abc123...",
    "size": 12345,
    "success": true
  },
  "id": 1
}
```

### 订阅（WebSocket）

#### `wes_subscribe`

订阅事件。

**参数**：
1. `subscriptionType` (string) - 订阅类型
   - `"newHeads"` - 新区块头
   - `"newPendingTxs"` - 新待处理交易
   - `"logs"` - 合约日志
2. `filters` (object, 可选) - 过滤器

**返回**：
```json
{
  "jsonrpc": "2.0",
  "result": "0xsubscription123",
  "id": 1
}
```

**事件推送（含重组标记）**：
```json
{
  "jsonrpc": "2.0",
  "method": "wes_subscription",
  "params": {
    "subscription": "0xsubscription123",
    "result": {
      "type": "newHead",
      "height": 12345,
      "hash": "0xabc...",
      "removed": false,
      "reorgId": "r123",
      "resumeToken": "tok789"
    }
  }
}
```

#### `wes_unsubscribe`

取消订阅。

**参数**：
1. `subscriptionId` (string) - 订阅ID

---

## ⚠️ 错误码

### 标准错误码

| 代码 | 消息 | 含义 |
|------|------|------|
| -32700 | Parse error | JSON解析错误 |
| -32600 | Invalid Request | 无效请求 |
| -32601 | Method not found | 方法不存在 |
| -32602 | Invalid params | 无效参数 |
| -32603 | Internal error | 内部错误 |

### WES自定义错误码

| 代码 | 消息 | 含义 |
|------|------|------|
| -32000 | Node is syncing | 节点正在同步 |
| -32001 | Block not found | 区块不存在 |
| -32002 | Invalid block param | 无效的区块参数 |
| -32003 | Transaction fee too low | 交易费过低 |
| -32004 | Transaction already known | 交易已存在 |
| -32005 | Transaction conflicts | 交易冲突 |
| -32006 | Invalid transaction signature | 无效签名 |
| -32008 | Mempool full | 内存池已满 |
| -32010 | Chain reorganized | 链重组 |

---

## 📋 兼容性说明

| 项目 | 兼容性 | 说明 |
|------|-------|------|
| **web3.js** | ✅ 兼容 | 可直接使用 |
| **ethers.js** | ✅ 兼容 | 可直接使用 |
| **Geth** | ⚠️ 部分 | 方法名不同(`wes_`前缀) |
| **Bitcoin Core** | ❌ 不兼容 | 协议差异过大 |

---

## 📚 相关文档

- [CLI 参考](../cli/index.md) - 命令行工具文档
- [配置参考](../config/index.md) - 配置字段说明
- [Schema 参考](../schema/index.md) - 数据格式规范

**完整 API 规范**：
- [OpenAPI 规范](../../../internal/api/docs/openapi.yaml) - RESTful API 完整定义
- [JSON-RPC 规范](../../../internal/api/docs/jsonrpc_spec.md) - JSON-RPC 方法完整列表

---

**相关文档**：
- [产品总览](../../overview.md) - 了解 WES 是什么、核心价值、应用场景
- [组件能力视图](../../components/) - 了解各组件能力
