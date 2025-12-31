# WES JSON-RPC API 规范

> **版本**: v1.0.0  
> **协议**: JSON-RPC 2.0  
> **端口**: :28680

---

## 📍 **概述**

WES JSON-RPC API 是区块链节点的主协议接口,遵循 JSON-RPC 2.0 规范,与以太坊生态兼容。

**设计目标**:
- ✅ web3.js/ethers.js 直接可用
- ✅ 对标 Geth/Bitcoin Core
- ✅ 支持客户端签名模式
- ✅ 支持状态锚定查询
- ✅ 支持重组安全订阅

---

## 🔌 **连接方式**

### **HTTP**
```bash
curl -X POST http://localhost:28680 \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "wes_blockNumber",
    "params": [],
    "id": 1
  }'
```

### **WebSocket（订阅）**
```javascript
const ws = new WebSocket('ws://localhost:28680');

ws.send(JSON.stringify({
  jsonrpc: "2.0",
  method: "eth_subscribe",
  params: ["newHeads"],
  id: 1
}));
```

---

## 📚 **方法列表**

### **链信息**

#### `net_version`
返回网络ID。

**参数**: 无

**返回**:
```json
{
  "jsonrpc": "2.0",
  "result": "1",
  "id": 1
}
```

#### `wes_chainId`
返回链ID（十六进制）。

**参数**: 无

**返回**:
```json
{
  "jsonrpc": "2.0",
  "result": "0x1",
  "id": 1
}
```

#### `wes_syncing`
返回同步状态。

**参数**: 无

**返回（未同步）**:
```json
{
  "jsonrpc": "2.0",
  "result": false,
  "id": 1
}
```

**返回（同步中）**:
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

#### `wes_blockNumber`
返回最新区块高度。

**参数**: 无

**返回**:
```json
{
  "jsonrpc": "2.0",
  "result": "0x1234",
  "id": 1
}
```

---

### **区块查询**

#### `wes_getBlockByHeight`
按高度查询区块。

**参数**:
1. `height` (string) - 区块高度（十六进制）
2. `fullTx` (boolean) - 是否返回完整交易（否则仅返回哈希）

**示例**:
```json
{
  "jsonrpc": "2.0",
  "method": "wes_getBlockByHeight",
  "params": ["0x1234", false],
  "id": 1
}
```

**返回**:
```json
{
  "jsonrpc": "2.0",
  "result": {
    "height": "0x1234",
    "hash": "0xabc...",
    "parentHash": "0xdef...",
    "timestamp": "0x5f5e100",
    "stateRoot": "0x123...",
    "transactions": ["0xtx1...", "0xtx2..."]
  },
  "id": 1
}
```

#### `wes_getBlockByHash`
按哈希查询区块。

**参数**:
1. `hash` (string) - 区块哈希
2. `fullTx` (boolean) - 是否返回完整交易

**示例**:
```json
{
  "jsonrpc": "2.0",
  "method": "wes_getBlockByHash",
  "params": ["0xabc...", false],
  "id": 1
}
```

---

### **交易**

#### `wes_sendRawTransaction`
提交已签名交易。

**⚠️ 安全**: 仅接受已签名交易,不接受私钥！

**参数**:
1. `signedTx` (string) - 十六进制编码的已签名交易

**示例**:
```json
{
  "jsonrpc": "2.0",
  "method": "wes_sendRawTransaction",
  "params": ["0xf86c..."],
  "id": 1
}
```

**返回（成功）**:
```json
{
  "jsonrpc": "2.0",
  "result": "0xtxhash...",
  "id": 1
}
```

**返回（失败）**:
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

---

### **资源查询**

#### `wes_getResourceByContentHash`
根据内容哈希查询资源元数据。

**参数**:
1. `content_hash` (string) - 资源内容哈希（十六进制）

**示例**:
```json
{
  "jsonrpc": "2.0",
  "method": "wes_getResourceByContentHash",
  "params": ["0xabc123..."],
  "id": 1
}
```

**返回**:
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
- `owner` 过滤：按创建者地址（`creator_address`）过滤，支持 hex 格式（可带或不带 `0x` 前缀）
- 返回数组中的每个资源对象字段与 `wes_getResourceByContentHash` 一致

---

### **交易历史**

#### `wes_getTransactionHistory`
查询交易历史（支持按交易ID或资源ID查询）。

**参数**:
```json
{
  "filters": {
    "txId": "0x...",           // 可选：交易哈希（与 resourceId 至少提供一个）
    "resourceId": "0x...",     // 可选：资源内容哈希（与 txId 至少提供一个）
    "limit": 1,                 // 可选：返回数量限制（默认1）
    "offset": 0                 // 可选：偏移量（默认0）
  }
}
```

**示例（按交易ID）**:
```json
{
  "jsonrpc": "2.0",
  "method": "wes_getTransactionHistory",
  "params": [{
    "filters": {
      "txId": "0xabc123..."
    }
  }],
  "id": 1
}
```

**示例（按资源ID）**:
```json
{
  "jsonrpc": "2.0",
  "method": "wes_getTransactionHistory",
  "params": [{
    "filters": {
      "resourceId": "0xdef456..."
    }
  }],
  "id": 1
}
```

**返回**:
```json
{
  "jsonrpc": "2.0",
  "result": [
    {
      "hash": "0xabc123...",
      "blockHeight": "0x1234",
      "blockHash": "0xdef...",
      "transactionIndex": "0x0",
      "inputs": [...],
      "outputs": [...]
    }
  ],
  "id": 1
}
```

**说明**:
- 必须至少提供 `txId` 或 `resourceId` 之一
- 按 `txId` 查询：直接返回该交易的详细信息（数组形式）
- 按 `resourceId` 查询：返回该资源首次出现的部署交易信息（数组形式）
- 返回数组中的每个交易对象字段与 `wes_getTransactionByHash` 一致

---

### **订阅（WebSocket）**

#### `wes_subscribe`
订阅事件。

**参数**:
1. `subscriptionType` (string) - 订阅类型
   - `"newHeads"` - 新区块头
   - `"newPendingTxs"` - 新待处理交易
   - `"logs"` - 合约日志
2. `filters` (object, 可选) - 过滤器

**示例**:
```json
{
  "jsonrpc": "2.0",
  "method": "wes_subscribe",
  "params": ["newHeads"],
  "id": 1
}
```

**返回**:
```json
{
  "jsonrpc": "2.0",
  "result": "0xsubscription123",
  "id": 1
}
```

**事件推送（含重组标记）**:
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

**重组事件**:
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
      "removed": true,
      "reorgId": "r124"
    }
  }
}
```

#### `wes_unsubscribe`
取消订阅。

**参数**:
1. `subscriptionId` (string) - 订阅ID

**示例**:
```json
{
  "jsonrpc": "2.0",
  "method": "wes_unsubscribe",
  "params": ["0xsubscription123"],
  "id": 1
}
```

---

## ⚠️ **错误码**

### **标准错误码**
| 代码 | 消息 | 含义 |
|------|------|------|
| -32700 | Parse error | JSON解析错误 |
| -32600 | Invalid Request | 无效请求 |
| -32601 | Method not found | 方法不存在 |
| -32602 | Invalid params | 无效参数 |
| -32603 | Internal error | 内部错误 |

### **WES自定义错误码**
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

## 📋 **兼容性说明**

| 项目 | 兼容性 | 说明 |
|------|-------|------|
| **web3.js** | ✅ 兼容 | 可直接使用 |
| **ethers.js** | ✅ 兼容 | 可直接使用 |
| **Geth** | ⚠️ 部分 | 方法名不同(`wes_`前缀) |
| **Bitcoin Core** | ❌ 不兼容 | 协议差异过大 |

---

## 🧮 高级张量类型扩展

对于涉及推理结果、张量输出等高级数据类型的 JSON-RPC 方法（例如内部使用的推理调用接口），其 **float16 / bfloat16 / 量化张量** 等高级 dtype 的具体表达方式，不在本规范中展开，统一由《WES JSON-RPC 高级张量类型协议规范》进行约定。

- 本规范侧重于：**方法列表、请求/响应基本结构、错误码与兼容性**；
- 高级张量类型相关的字段（例如 `tensor_outputs`）、dtype 列表、能力协商机制等，请参考：
  - [`jsonrpc_advanced_tensor_types.md`](./jsonrpc_advanced_tensor_types.md)

---

## 🔗 相关文档

- [资源元数据标准化规范](./jsonrpc_resource_metadata.md) - 资源元数据字段和代码/ABI 查询方法

---

> 📝 **文档更新**  
> 最后更新: 2025-11-XX  
> 对标: Ethereum JSON-RPC, Geth, EIP-1898

