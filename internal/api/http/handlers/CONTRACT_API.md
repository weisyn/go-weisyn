# 🚀 WES智能合约HTTP API文档

## 📋 API概览

WES智能合约系统提供完整的HTTP REST API，支持合约部署、调用、查询等全生命周期操作。所有API端点都在 `/api/v1/contract` 路径下。

### 🎯 **核心功能**
- ✅ 智能合约部署
- ✅ 合约函数调用
- ✅ 合约状态查询
- ✅ 代币余额管理
- ✅ 合约信息查询
- ✅ 执行统计监控

### 🔧 **技术特性**
- RESTful API设计
- JSON请求/响应格式
- 统一的错误处理
- 执行费用计量和限制
- 事件发射记录

## 📊 **API端点总览**

| 方法 | 端点 | 功能描述 | 状态 |
|------|------|----------|------|
| POST | `/api/v1/contract/deploy` | 部署智能合约 | ✅ |
| POST | `/api/v1/contract/call` | 调用合约函数 | ✅ |
| GET | `/api/v1/contract/query` | 查询合约状态 | ✅ |
| GET | `/api/v1/contract/info/:hash` | 获取合约信息 | ✅ |
| GET | `/api/v1/contract/balance` | 查询代币余额 | ✅ |
| GET | `/api/v1/contract/token/info/:hash` | 获取代币信息 | ✅ |
| GET | `/api/v1/contract/stats` | 获取执行统计 | ✅ |

## 🔗 **详细API文档**

### 1. 部署智能合约

**端点**: `POST /api/v1/contract/deploy`

**功能**: 将WASM智能合约部署到区块链上

**请求体**:
```json
{
  "wasm_code": "0061736d0100000001...",  // 十六进制WASM字节码 (必需)
  "owner": "alice",                      // 部署者地址 (必需)
  "init_params": "",                     // 初始化参数 (可选)
  "执行费用_limit": 1000000,                  // 执行费用限制 (可选，默认1M)
  "metadata": {                          // 合约元数据 (可选)
    "name": "WES Token",
    "symbol": "WES",
    "description": "WES区块链原生代币"
  }
}
```

**响应示例**:
```json
{
  "success": true,
  "message": "合约部署成功",
  "data": {
    "hash": "a1b2c3d4e5f6...",
    "owner": "alice",
    "deploy_time": 1703404800,
    "code_size": 1024,
    "version": "1.0.0",
    "metadata": {
      "name": "WES Token",
      "symbol": "WES"
    }
  },
  "timestamp": 1703404800
}
```

**错误响应**:
```json
{
  "success": false,
  "message": "合约部署失败",
  "error": "WASM代码解析失败: invalid magic number",
  "timestamp": 1703404800
}
```

### 2. 调用合约函数

**端点**: `POST /api/v1/contract/call`

**功能**: 调用已部署智能合约的函数（状态变更操作）

**请求体**:
```json
{
  "contract_hash": "a1b2c3d4e5f6...",    // 合约哈希 (必需)
  "function": "transfer",                // 函数名 (必需)
  "parameters": "bob,1000",              // 函数参数 (可选)
  "caller": "alice",                     // 调用者地址 (必需)
  "执行费用_limit": 100000                    // 执行费用限制 (可选，默认10万)
}
```

**响应示例**:
```json
{
  "success": true,
  "message": "合约调用成功",
  "data": {
    "return_data": "01",
    "success": true
  },
  "执行费用_used": 85000,
  "events": [
    {
      "name": "Transfer",
      "data": "616c69636500000000000000000000000000000000000000000000000000000000626f6200000000000000000000000000000000000000000000000000000000e803000000000000"
    }
  ],
  "timestamp": 1703404800
}
```

### 3. 查询合约状态

**端点**: `GET /api/v1/contract/query`

**功能**: 查询合约状态（只读操作，不消耗执行费用上链）

**查询参数**:
- `contract_hash`: 合约哈希 (必需)
- `function`: 查询函数名 (必需)
- `parameters`: 函数参数 (可选)

**请求示例**:
```
GET /api/v1/contract/query?contract_hash=a1b2c3d4e5f6...&function=balance_of&parameters=alice
```

**响应示例**:
```json
{
  "success": true,
  "message": "合约查询成功",
  "data": {
    "return_data": "00c9f2c9cd04000000",
    "value": 999000000
  },
  "执行费用_used": 50,
  "timestamp": 1703404800
}
```

### 4. 获取合约信息

**端点**: `GET /api/v1/contract/info/:hash`

**功能**: 获取已部署合约的基本信息

**路径参数**:
- `hash`: 合约哈希

**请求示例**:
```
GET /api/v1/contract/info/a1b2c3d4e5f6...
```

**响应示例**:
```json
{
  "success": true,
  "message": "获取合约信息成功",
  "data": {
    "hash": "a1b2c3d4e5f6...",
    "owner": "616c69636500000000000000000000000000000000000000000000000000000000",
    "deploy_time": 1703404800,
    "code_size": 1024,
    "version": "1.0.0"
  },
  "timestamp": 1703404800
}
```

### 5. 查询代币余额

**端点**: `GET /api/v1/contract/balance`

**功能**: 查询指定地址的代币余额（专用于ERC20风格代币）

**查询参数**:
- `contract_hash`: 代币合约哈希 (必需)
- `address`: 查询地址 (必需)

**请求示例**:
```
GET /api/v1/contract/balance?contract_hash=a1b2c3d4e5f6...&address=alice
```

**响应示例**:
```json
{
  "success": true,
  "message": "余额查询成功",
  "data": {
    "address": "alice",
    "balance": 999000000,
    "contract_hash": "a1b2c3d4e5f6..."
  },
  "timestamp": 1703404800
}
```

### 6. 获取代币信息

**端点**: `GET /api/v1/contract/token/info/:hash`

**功能**: 获取ERC20代币的详细信息

**路径参数**:
- `hash`: 代币合约哈希

**请求示例**:
```
GET /api/v1/contract/token/info/a1b2c3d4e5f6...
```

**响应示例**:
```json
{
  "success": true,
  "message": "代币信息查询成功",
  "data": {
    "name": "WES Token",
    "symbol": "WES",
    "decimals": 18,
    "total_supply": 1000000000
  },
  "timestamp": 1703404800
}
```

### 7. 获取执行统计

**端点**: `GET /api/v1/contract/stats`

**功能**: 获取智能合约执行引擎的统计信息

**响应示例**:
```json
{
  "success": true,
  "message": "执行统计获取成功",
  "data": {
    "total_executions": 1250,
    "total_执行费用_used": 12500000,
    "average_执行费用_used": 10000,
    "total_time": "2m30.5s",
    "average_time": "120ms"
  },
  "timestamp": 1703404800
}
```

## 🔧 **参数格式说明**

### 地址格式
- **别名**: `alice`, `bob`, `charlie`
- **十六进制**: `0x1234567890abcdef...` (32字节)
- **自动填充**: 短地址会自动填充到32字节

### 参数编码
函数参数使用逗号分隔，支持以下格式：
- **数字**: `1000` (自动编码为8字节小端序)
- **地址**: `alice` 或 `0x1234...`
- **十六进制**: `0xabcdef...`

### WASM代码格式
- **十六进制字符串**: `0061736d01000000...`
- **无0x前缀**: `61736d01000000...`
- **文件路径**: `./token.wasm` (仅限本地测试)

## 📊 **响应格式标准**

### 成功响应
```json
{
  "success": true,
  "message": "操作描述",
  "data": {...},              // 响应数据
  "执行费用_used": 50000,          // 执行费用消耗 (可选)
  "events": [...],            // 事件列表 (可选)
  "timestamp": 1703404800
}
```

### 错误响应
```json
{
  "success": false,
  "message": "错误描述",
  "error": "详细错误信息",
  "timestamp": 1703404800
}
```

## 🧪 **API测试示例**

### 使用curl测试

#### 1. 部署WES Token合约
```bash
curl -X POST http://localhost:8080/api/v1/contract/deploy \
  -H "Content-Type: application/json" \
  -d '{
    "wasm_code": "0061736d0100000001...",
    "owner": "alice",
    "执行费用_limit": 1000000,
    "metadata": {
      "name": "WES Token",
      "symbol": "WES"
    }
  }'
```

#### 2. 查询代币总供应量
```bash
curl -X GET "http://localhost:8080/api/v1/contract/query?contract_hash=a1b2c3d4...&function=total_supply"
```

#### 3. 转账操作
```bash
curl -X POST http://localhost:8080/api/v1/contract/call \
  -H "Content-Type: application/json" \
  -d '{
    "contract_hash": "a1b2c3d4...",
    "function": "transfer",
    "parameters": "bob,1000",
    "caller": "alice",
    "执行费用_limit": 100000
  }'
```

#### 4. 查询余额
```bash
curl -X GET "http://localhost:8080/api/v1/contract/balance?contract_hash=a1b2c3d4...&address=alice"
```

### 使用JavaScript测试

```javascript
// 部署合约
const deployContract = async () => {
  const response = await fetch('http://localhost:8080/api/v1/contract/deploy', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      wasm_code: '0061736d0100000001...',
      owner: 'alice',
      执行费用_limit: 1000000
    })
  });
  
  const result = await response.json();
  console.log('部署结果:', result);
  return result.data.hash;
};

// 查询余额
const queryBalance = async (contractHash, address) => {
  const response = await fetch(`http://localhost:8080/api/v1/contract/balance?contract_hash=${contractHash}&address=${address}`);
  const result = await response.json();
  console.log('余额查询:', result);
  return result.data.balance;
};

// 转账
const transfer = async (contractHash, to, amount) => {
  const response = await fetch('http://localhost:8080/api/v1/contract/call', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      contract_hash: contractHash,
      function: 'transfer',
      parameters: `${to},${amount}`,
      caller: 'alice',
      执行费用_limit: 100000
    })
  });
  
  const result = await response.json();
  console.log('转账结果:', result);
  return result.success;
};
```

### 使用Python测试

```python
import requests
import json

# 部署合约
def deploy_contract():
    url = "http://localhost:8080/api/v1/contract/deploy"
    data = {
        "wasm_code": "0061736d0100000001...",
        "owner": "alice",
        "执行费用_limit": 1000000,
        "metadata": {
            "name": "WES Token",
            "symbol": "WES"
        }
    }
    
    response = requests.post(url, json=data)
    result = response.json()
    print("部署结果:", result)
    return result["data"]["hash"]

# 查询余额
def query_balance(contract_hash, address):
    url = f"http://localhost:8080/api/v1/contract/balance"
    params = {
        "contract_hash": contract_hash,
        "address": address
    }
    
    response = requests.get(url, params=params)
    result = response.json()
    print("余额查询:", result)
    return result["data"]["balance"]

# 转账
def transfer(contract_hash, to, amount):
    url = "http://localhost:8080/api/v1/contract/call"
    data = {
        "contract_hash": contract_hash,
        "function": "transfer",
        "parameters": f"{to},{amount}",
        "caller": "alice",
        "执行费用_limit": 100000
    }
    
    response = requests.post(url, json=data)
    result = response.json()
    print("转账结果:", result)
    return result["success"]

# 使用示例
if __name__ == "__main__":
    # 部署合约
    contract_hash = deploy_contract()
    
    # 查询初始余额
    alice_balance = query_balance(contract_hash, "alice")
    print(f"Alice初始余额: {alice_balance}")
    
    # 转账给Bob
    transfer_success = transfer(contract_hash, "bob", 1000)
    
    if transfer_success:
        # 查询转账后余额
        alice_balance = query_balance(contract_hash, "alice")
        bob_balance = query_balance(contract_hash, "bob")
        print(f"转账后 - Alice: {alice_balance}, Bob: {bob_balance}")
```

## ⚡ **性能指标**

### 响应时间基准
- **合约查询**: < 50ms
- **合约调用**: < 200ms
- **合约部署**: < 1000ms
- **余额查询**: < 30ms

### 执行费用消耗参考
- **状态读取**: 100 执行费用
- **状态写入**: 200 执行费用 + 2*字节数
- **代币转账**: ~85,000 执行费用
- **合约部署**: 100,000 - 1,000,000 执行费用

### 并发能力
- **最大并发**: 1000+ 请求/秒
- **查询操作**: 无锁并发
- **状态变更**: 串行化处理

## 🛡️ **安全注意事项**

### 输入验证
- 所有输入参数都经过严格验证
- WASM代码魔数检查
- 执行费用限制防止DoS攻击
- 地址格式标准化

### 权限控制
- 只有合约所有者可以升级合约
- 转账需要足够余额验证
- 授权额度检查

### 错误处理
- 统一的错误响应格式
- 详细的错误信息记录
- 优雅的异常恢复

## 📚 **相关文档**

- [WES架构设计](../../../docs/ARCHITECTURE.md)
- [智能合约开发指南](../../../contracts/README.md)
- [WASM虚拟机文档](../../../internal/core/blockchain/domains/execution/README.md)
- [测试用例说明](../../../test/integration/README.md)

---

*🎉 现在你可以通过HTTP API完全控制WES智能合约系统了！*
