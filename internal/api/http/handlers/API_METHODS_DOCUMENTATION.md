# API Methods Documentation

本文档详细说明了所有 API handlers 中的方法使用方式和返回值。

## 🏦 Transaction Handlers (transaction.go)

### 1. Transfer - 基础转账
**HTTP**: `POST /transactions/transfer`

**请求参数**：
```json
{
  "sender_private_key": "1234567890abcdef...",
  "to_address": "Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn", 
  "amount": "100.0",
  "token_id": "",
  "memo": "转账备注",
  "options": {...}
}
```

**成功响应**：
```json
{
  "success": true,
  "transaction_hash": "a1b2c3d4e5f6...",
  "message": "转账交易已成功创建"
}
```

### 2. BatchTransfer - 批量转账
**HTTP**: `POST /transactions/batch-transfer`

**请求参数**：
```json
{
  "sender_private_key": "1234567890abcdef...",
  "transfers": [
    {
      "to_address": "Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn",
      "amount": "100.0",
      "token_id": "",
      "memo": "工资发放"
    }
  ]
}
```

**成功响应**：
```json
{
  "success": true,
  "transaction_hash": "a1b2c3d4e5f6...",
  "message": "批量转账交易已成功创建，共 1 笔转账"
}
```

### 3. SignTransaction - 签名交易
**HTTP**: `POST /transactions/sign`

**请求参数**：
```json
{
  "transaction_hash": "a1b2c3d4e5f6...",
  "private_key": "1234567890abcdef..."
}
```

**成功响应**：
```json
{
  "success": true,
  "signed_tx_hash": "b2c3d4e5f6...",
  "message": "交易签名成功"
}
```

### 4. SubmitTransaction - 提交交易
**HTTP**: `POST /transactions/submit`

**请求参数**：
```json
{
  "signed_tx_hash": "b2c3d4e5f6..."
}
```

**成功响应**：
```json
{
  "success": true,
  "message": "交易已成功提交到网络"
}
```

### 5. GetTransactionStatus - 查询交易状态
**HTTP**: `GET /transactions/status/{txHash}`

**成功响应**：
```json
{
  "success": true,
  "status": "confirmed",
  "message": "交易状态: confirmed"
}
```

**状态值**：`pending`, `confirmed`, `failed`

### 6. GetTransactionDetails - 获取交易详情
**HTTP**: `GET /transactions/{txHash}`

**成功响应**：
```json
{
  "success": true,
  "transaction": {
    "hash": "a1b2c3d4e5f6...",
    "inputs": [...],
    "outputs": [...],
    "signatures": [...]
  },
  "message": "交易详情获取成功"
}
```

### 7. EstimateTransactionFee - 估算交易费用
**HTTP**: `POST /transactions/estimate-fee`

**请求参数**：
```json
{
  "transaction_hash": "a1b2c3d4e5f6..."
}
```

**成功响应**：
```json
{
  "success": true,
  "estimated_fee": 1000,
  "message": "预估费用: 1000"
}
```

### 8. ValidateTransaction - 验证交易
**HTTP**: `POST /transactions/validate`

**请求参数**：
```json
{
  "transaction_hash": "a1b2c3d4e5f6..."
}
```

**成功响应**：
```json
{
  "success": true,
  "valid": true,
  "message": "交易验证通过"
}
```

### 9. StartMultiSigSession - 开始多签会话
**HTTP**: `POST /transactions/multisig/start`

**请求参数**：
```json
{
  "required_signatures": 3,
  "authorized_signers": ["addr1", "addr2", "addr3", "addr4", "addr5"],
  "expiry_duration": "24h",
  "description": "Q4季度资金划拨"
}
```

**成功响应**：
```json
{
  "success": true,
  "session_id": "session123456",
  "message": "多签会话创建成功"
}
```

### 10. AddMultiSigSignature - 添加多签签名
**HTTP**: `POST /transactions/multisig/{sessionID}/sign`

**请求参数**：
```json
{
  "signature": {
    "signer_address": "addr1",
    "public_key": "...",
    "signature": "...",
    "signature_algorithm": "ECDSA_SECP256K1",
    "signed_at": "2024-01-15T10:30:00Z",
    "signer_role": "CFO"
  }
}
```

**成功响应**：
```json
{
  "success": true,
  "message": "签名已成功添加到多签会话"
}
```

## 🏢 Account Handlers (account.go)

### 1. GetPlatformBalance - 获取平台主币余额
**HTTP**: `GET /accounts/{address}/balance`

**成功响应**：
```json
{
  "success": true,
  "data": {
    "address": "Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn",
    "token_id": "",
    "available": 1500000000000000000,
    "locked": 0,
    "pending": 0,
    "total": 1500000000000000000,
    "last_updated": 1640995200
  },
  "message": "余额查询成功"
}
```

### 2. GetTokenBalance - 获取指定代币余额
**HTTP**: `GET /accounts/{address}/balance/{tokenId}`

**成功响应**：
```json
{
  "success": true,
  "data": {
    "address": "Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn",
    "token_id": "abcdef123456...",
    "available": 1000000000,
    "locked": 0,
    "pending": 0,
    "total": 1000000000,
    "last_updated": 1640995200
  }
}
```

### 3. GetAllTokenBalances - 获取所有代币余额
**HTTP**: `GET /accounts/{address}/balances`

**成功响应**：
```json
{
  "success": true,
  "data": {
    "": {
      "available": 1500000000000000000,
      "total": 1500000000000000000
    },
    "abcdef123456...": {
      "available": 1000000000,
      "total": 1000000000
    }
  }
}
```

### 4. GetLockedBalances - 获取锁定余额详情
**HTTP**: `GET /accounts/{address}/locked?tokenId=xxx`

### 5. GetPendingBalances - 获取待确认余额详情  
**HTTP**: `GET /accounts/{address}/pending?tokenId=xxx`

### 6. GetAccountInfo - 获取账户信息
**HTTP**: `GET /accounts/{address}/info`

## 📄 Resource Handlers (resource.go)

### 1. StoreResource - 存储资源
**HTTP**: `POST /resources/store`

**请求参数**：
```json
{
  "source_file_path": "/path/to/document.pdf",
  "metadata": {
    "type": "document",
    "author": "张三",
    "description": "重要合同文件"
  }
}
```

**成功响应**：
```json
{
  "success": true,
  "content_hash": "a1b2c3d4e5f6...",
  "message": "资源存储成功"
}
```

### 2. GetResource - 获取资源信息
**HTTP**: `GET /resources/{hash}`

**成功响应**：
```json
{
  "success": true,
  "resource": {
    "resource_path": "/contracts/token.wasm",
    "resource_type": "contract",
    "content_hash": "a1b2c3d4e5f6...",
    "size": 1024,
    "stored_at": 1640995200,
    "metadata": {...},
    "is_available": true
  },
  "message": "资源信息获取成功"
}
```

### 3. ListResources - 列出指定类型资源
**HTTP**: `GET /resources/list/{type}?offset=0&limit=50`

**成功响应**：
```json
{
  "success": true,
  "resources": [
    {
      "resource_type": "contract",
      "content_hash": "a1b2c3d4e5f6...",
      "size": 1024,
      "metadata": {...}
    }
  ],
  "message": "成功获取 1 个资源"
}
```

## ⛓️ Block Handlers (block.go)

### 1. GetChainInfo - 获取链信息
**HTTP**: `GET /blocks/chain-info`

**成功响应**：
```json
{
  "success": true,
  "chain_info": {
    "height": 12345,
    "best_block_hash": "a1b2c3d4e5f6...",
    "is_ready": true,
    "status": "normal",
    "network_height": 12345,
    "peer_count": 8,
    "last_block_time": 1640995200,
    "uptime": 86400,
    "node_mode": "full"
  },
  "message": "链信息获取成功"
}
```

### 2. GetBlockByHeight - 根据高度获取区块
**HTTP**: `GET /blocks/height/{height}`

**成功响应**：
```json
{
  "success": true,
  "block": {
    "header": {
      "height": 12345,
      "hash": "a1b2c3d4e5f6...",
      "previous_hash": "b2c3d4e5f6...",
      "timestamp": 1640995200
    },
    "transactions": [...]
  },
  "message": "区块获取成功"
}
```

### 3. GetBlockByHash - 根据哈希获取区块
**HTTP**: `GET /blocks/hash/{hash}`

### 4. GetLatestBlock - 获取最新区块
**HTTP**: `GET /blocks/latest`

## 🤖 Contract Handlers (contract.go)

### 1. DeployContract - 部署智能合约
**HTTP**: `POST /contracts/deploy`

**请求参数**：
```json
{
  "deployer_private_key": "1234567890abcdef...",
  "contract_file_path": "/path/to/contract.wasm",
  "config": {
    "max_执行费用_limit": 1000000,
    "max_memory_pages": 256,
    "timeout": 30
  },
  "name": "去中心化投票系统",
  "description": "基于区块链的透明投票合约"
}
```

**成功响应**：
```json
{
  "success": true,
  "transaction_hash": "a1b2c3d4e5f6...",
  "message": "合约部署交易已成功创建"
}
```

### 2. CallContract - 调用智能合约
**HTTP**: `POST /contracts/call`

**请求参数**：
```json
{
  "caller_private_key": "1234567890abcdef...",
  "contract_address": "0xabcdef123456...",
  "method_name": "transfer",
  "parameters": {
    "to": "0x123...",
    "amount": "100"
  },
  "执行费用_limit": 500000,
  "value": "0"
}
```

**成功响应**：
```json
{
  "success": true,
  "transaction_hash": "b2c3d4e5f6...",
  "message": "合约调用交易已成功创建"
}
```

### 3. DeployStaticResource - 部署静态资源
**HTTP**: `POST /contracts/deploy-resource`

**请求参数**：
```json
{
  "deployer_private_key": "1234567890abcdef...",
  "file_path": "/path/to/document.pdf",
  "name": "重要文档",
  "description": "合同文件",
  "tags": ["合同", "法律"],
  "options": {...}
}
```

**成功响应**：
```json
{
  "success": true,
  "transaction_hash": "c3d4e5f6...",
  "message": "静态资源部署交易已成功创建"
}
```

### 4. DeployAIModel - 部署AI模型
**HTTP**: `POST /ai/deploy`

**请求参数**：
```json
{
  "deployer_private_key": "1234567890abcdef...",
  "model_file_path": "/path/to/model.onnx",
  "config": {
    "format": "onnx",
    "framework": "onnxruntime",
    "max_batch_size": 32,
    "max_memory_mb": 2048
  },
  "name": "图像分类模型",
  "description": "ResNet50图像分类器"
}
```

**成功响应**：
```json
{
  "success": true,
  "transaction_hash": "d4e5f6...",
  "message": "AI模型部署交易已成功创建"
}
```

### 5. InferAIModel - AI模型推理
**HTTP**: `POST /ai/infer`

**请求参数**：
```json
{
  "caller_private_key": "1234567890abcdef...",
  "model_address": "0xabcdef123456...",
  "input_data": {
    "image": [0.485, 0.456, 0.406, ...]
  },
  "parameters": {
    "top_k": 5,
    "confidence": 0.1
  }
}
```

**成功响应**：
```json
{
  "success": true,
  "transaction_hash": "e5f6789...",
  "message": "AI模型推理交易已成功创建"
}
```

## ⛏️ Mining Handlers (mining.go)

### 1. StartMining - 启动挖矿
**HTTP**: `POST /mining/start`

**请求参数**：
```json
{
  "miner_address": "Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn",
  "threads": 4
}
```

**成功响应**：
```json
{
  "message": "挖矿启动成功",
  "status": "mining_started", 
  "miner_address": "1234567890abcdef..."
}
```

### 2. StopMining - 停止挖矿
**HTTP**: `POST /mining/stop`

**请求参数**：无需请求体

**成功响应**：
```json
{
  "message": "挖矿停止成功",
  "status": "mining_stopped"
}
```

### 3. GetMiningStatus - 获取挖矿状态
**HTTP**: `GET /mining/status`

**成功响应**：
```json
{
  "is_mining": true,
  "miner_address": "1234567890abcdef...",
  "start_time": "2024-01-15T10:30:00Z",
  "current_height": 12345
}
```

### 4. MineOnce - 单次挖矿
**HTTP**: `POST /mining/once`

**请求参数**：
```json
{
  "miner_address": "Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn",
  "max_txs": 1000
}
```

## 🌐 Node Handlers (node.go)

### 1. GetNodeInfo - 获取节点信息
**HTTP**: `GET /node/info`

**成功响应**：
```json
{
  "success": true,
  "node_id": "12D3KooW...",
  "addresses": [
    "/ip4/192.168.1.100/tcp/4001/p2p/12D3KooW..."
  ],
  "address_count": 2,
  "actual_listen_addrs": [...],
  "supported_protocols": ["kad-dht", "gossipsub"],
  "protocol_count": 2
}
```

### 2. GetNodeStatus - 获取节点状态
**HTTP**: `GET /node/status`

**成功响应**：
```json
{
  "success": true,
  "status": "running",
  "node_id": "12D3KooW...",
  "address_count": 2,
  "timestamp": 1640995200
}
```

### 3. GetPeers - 获取连接的节点列表
**HTTP**: `GET /node/peers?limit=100`

**成功响应**：
```json
{
  "success": true,
  "peers": [
    "12D3KooWAbc...",
    "12D3KooWDef..."
  ],
  "total_count": 15,
  "returned": 2
}
```

### 4. GetPeerByID - 获取特定节点信息
**HTTP**: `GET /node/peers/{peer_id}`

**成功响应**：
```json
{
  "success": true,
  "peer_id": "12D3KooW...",
  "connectedness": "Connected",
  "addresses": [...],
  "address_count": 3
}
```

### 5. Connect - 主动连接到指定节点
**HTTP**: `POST /node/connect`

**请求参数**：
```json
{
  "multiaddr": "/ip4/192.168.1.100/tcp/4001/p2p/12D3KooW..."
}
```

**成功响应**：
```json
{
  "success": true,
  "peer_id": "12D3KooW..."
}
```

### 6. GetTopicPeers - 获取主题连接节点
**HTTP**: `GET /node/topics/{topic}/peers`

**成功响应**：
```json
{
  "success": true,
  "topic": "weisyn.consensus.latest_block.v1",
  "peers": ["12D3KooW..."],
  "peer_count": 1
}
```

## 📊 使用流程示例

### 完整转账流程：
1. `POST /transactions/transfer` → 获得 `transaction_hash`
2. `POST /transactions/sign` → 获得 `signed_tx_hash`  
3. `POST /transactions/submit` → 提交到网络
4. `GET /transactions/status/{txHash}` → 查询确认状态

### 智能合约部署流程：
1. `POST /contracts/deploy` → 获得 `transaction_hash`
2. `POST /transactions/sign` → 签名交易
3. `POST /transactions/submit` → 提交部署
4. `POST /contracts/call` → 调用合约方法

### 资源管理流程：
1. `POST /resources/store` → 获得 `content_hash`
2. `GET /resources/{hash}` → 查询资源信息
3. `GET /resources/list/{type}` → 浏览同类型资源

## 🔧 通用错误响应格式

所有接口的错误响应都遵循统一格式：
```json
{
  "success": false,
  "message": "具体错误信息"
}
```

常见错误类型：
- 参数格式错误
- 服务暂时不可用  
- 资源不存在
- 权限不足
- 网络连接失败
