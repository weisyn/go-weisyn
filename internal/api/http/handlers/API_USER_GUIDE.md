#WES HTTP API 用户指南

## 🚀 **快速开始**

### **基础信息**
- **服务地址**: `http://localhost:8080`
- **API版本**: `v1`
- **基础路径**: `/api/v1`
- **响应格式**: JSON

### **标准响应格式**
```json
{
  "success": true,           // 操作是否成功
  "data": { ... },          // 响应数据（成功时）
  "message": "操作成功",     // 成功消息
  "error": {                // 错误信息（失败时）
    "code": "ERROR_CODE",
    "message": "错误描述",
    "details": "详细信息"
  }
}
```

## 💰 **Transaction API - 交易管理**

### **🎯 用户友好的简化端点（推荐）**

#### **1. 简单转账 - SimpleTransfer**
适用于90%的日常转账场景

```bash
curl -X POST http://localhost:8080/api/v1/transactions/simple-transfer \
  -H "Content-Type: application/json" \
  -d '{
    "from_address": "0x1234567890abcdef1234567890abcdef12345678",
    "to_address": "0xabcdef1234567890abcdef1234567890abcdef12",
    "amount": "1000000000000000000",
    "fee_amount": "50000000000000000",
    "memo": "转账给Alice"
  }'
```

**响应示例**:
```json
{
  "success": true,
  "transaction_hash": "a1b2c3d4e5f6789...",
  "message": "简单转账构建成功"
}
```

#### **2. 批量转账 - BatchTransfer**
适用于薪资发放、批量付款

```bash
curl -X POST http://localhost:8080/api/v1/transactions/batch-transfer \
  -H "Content-Type: application/json" \
  -d '{
    "from_address": "0x1234567890abcdef1234567890abcdef12345678",
    "outputs": [
      {
        "to_address": "0xabcdef1234567890abcdef1234567890abcdef12",
        "amount": "500000000000000000"
      },
      {
        "to_address": "0x9876543210fedcba9876543210fedcba98765432",
        "amount": "300000000000000000"
      }
    ],
    "fee_amount": "100000000000000000",
    "memo": "2024年1月薪资"
  }'
```

#### **3. 时间锁转账 - TimeLockTransfer**
适用于员工期权、生日礼物、定期存款

```bash
curl -X POST http://localhost:8080/api/v1/transactions/time-lock \
  -H "Content-Type: application/json" \
  -d '{
    "from_address": "0x1234567890abcdef1234567890abcdef12345678",
    "to_address": "0xabcdef1234567890abcdef1234567890abcdef12",
    "amount": "1000000000000000000",
    "unlock_timestamp": 1735689600,
    "fee_amount": "75000000000000000",
    "memo": "2025年新年礼物"
  }'
```

#### **4. 多签转账 - MultiSigTransfer**
适用于企业级资金管理

```bash
curl -X POST http://localhost:8080/api/v1/transactions/multi-sig \
  -H "Content-Type: application/json" \
  -d '{
    "from_address": "0x1234567890abcdef1234567890abcdef12345678",
    "to_address": "0xabcdef1234567890abcdef1234567890abcdef12",
    "amount": "5000000000000000000",
    "required_signatures": 2,
    "authorized_addresses": [
      "0x1111111111111111111111111111111111111111",
      "0x2222222222222222222222222222222222222222",
      "0x3333333333333333333333333333333333333333"
    ],
    "fee_amount": "150000000000000000",
    "memo": "董事会批准的采购款项"
  }'
```

### **🔐 核心交易流程**

#### **5. 交易签名（包含提交）**
```bash
curl -X POST http://localhost:8080/api/v1/transactions/sign \
  -H "Content-Type: application/json" \
  -d '{
    "transaction_hash": "a1b2c3d4e5f6789...",
    "private_key": "your_private_key_here"
  }'
```

#### **6. 查询交易状态**
```bash
curl http://localhost:8080/api/v1/transactions/status/a1b2c3d4e5f6789...
```

**响应示例**:
```json
{
  "success": true,
  "data": {
    "status": "confirmed",
    "block_height": 12345,
    "confirmations": 6
  }
}
```

### **💡 完整转账流程示例**

```bash
# 第1步：构建简单转账
RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/transactions/simple-transfer \
  -H "Content-Type: application/json" \
  -d '{
    "from_address": "0x1234567890abcdef1234567890abcdef12345678",
    "to_address": "0xabcdef1234567890abcdef1234567890abcdef12",
    "amount": "1000000000000000000"
  }')

# 提取交易哈希
TX_HASH=$(echo $RESPONSE | jq -r '.transaction_hash')

# 第2步：签名并提交
curl -X POST http://localhost:8080/api/v1/transactions/sign \
  -H "Content-Type: application/json" \
  -d '{
    "transaction_hash": "'$TX_HASH'",
    "private_key": "your_private_key"
  }'

# 第3步：查询状态
curl http://localhost:8080/api/v1/transactions/status/$TX_HASH
```

## 💰 **Account API - 账户查询**

### **1. 查询平台币余额**
```bash
curl http://localhost:8080/api/v1/accounts/0x1234567890abcdef1234567890abcdef12345678/balance
```

### **2. 查询所有代币余额**
```bash
curl http://localhost:8080/api/v1/accounts/0x1234567890abcdef1234567890abcdef12345678/balances
```

### **3. 通过公钥查询余额**
```bash
curl http://localhost:8080/api/v1/accounts/by-public-key/04f123456789.../balance
```

### **4. 查询账户信息**
```bash
curl http://localhost:8080/api/v1/accounts/0x1234567890abcdef1234567890abcdef12345678/info
```

**响应示例**:
```json
{
  "success": true,
  "data": {
    "address": "0x1234567890abcdef1234567890abcdef12345678",
    "platform_balance": "1000000000000000000",
    "total_tokens": 5,
    "created_at": "2024-01-01T00:00:00Z"
  },
  "message": "账户信息查询成功"
}
```

## 🧱 **Block API - 区块查询**

### **1. 获取最新区块**
```bash
curl http://localhost:8080/api/v1/blocks/latest
```

### **2. 按高度查询区块**
```bash
curl http://localhost:8080/api/v1/blocks/height/12345
```

### **3. 按哈希查询区块**
```bash
curl http://localhost:8080/api/v1/blocks/hash/0xabc123...
```

### **4. 查询区块范围**
```bash
curl "http://localhost:8080/api/v1/blocks/range?start=100&end=200&limit=50"
```

### **5. 查询链信息**
```bash
curl http://localhost:8080/api/v1/blocks/info
```

**响应示例**:
```json
{
  "success": true,
  "data": {
    "current_height": 12345,
    "best_block_hash": "0xabc123...",
    "total_transactions": 98765,
    "network_id": "mainnet"
  },
  "message": "链信息查询成功"
}
```

## ⛏️ **Mining API - 挖矿控制**

### **1. 启动挖矿**
```bash
curl -X POST http://localhost:8080/api/v1/mining/start \
  -H "Content-Type: application/json" \
  -d '{
    "miner_address": "0x1234567890abcdef1234567890abcdef12345678",
    "threads": 4
  }'
```

### **2. 停止挖矿**
```bash
curl -X POST http://localhost:8080/api/v1/mining/stop
```

### **3. 查询挖矿状态**
```bash
curl http://localhost:8080/api/v1/mining/status
```

**响应示例**:
```json
{
  "success": true,
  "data": {
    "is_mining": true,
    "hash_rate": "1.23 MH/s",
    "threads": 4,
    "blocks_mined": 42
  },
  "message": "挖矿状态查询成功"
}
```

## 🛠️ **错误处理**

### **常见错误代码**
| 错误代码 | 描述 | 解决方案 |
|----------|------|----------|
| `INVALID_ADDRESS` | 地址格式无效 | 检查地址是否为42字符的0x开头格式 |
| `INVALID_AMOUNT` | 金额格式无效 | 确保金额为有效的数字字符串 |
| `INSUFFICIENT_BALANCE` | 余额不足 | 检查账户余额是否足够 |
| `TRANSACTION_NOT_FOUND` | 交易未找到 | 确认交易哈希正确 |
| `INTERNAL_ERROR` | 内部服务器错误 | 联系技术支持 |

### **错误响应示例**
```json
{
  "success": false,
  "error": {
    "code": "INVALID_ADDRESS",
    "message": "地址格式无效",
    "details": "地址必须是42字符的十六进制字符串，以0x开头"
  }
}
```

## 🔧 **开发工具**

### **Postman集合**
导入以下JSON到Postman以快速测试API：

```json
{
  "info": {
    "name": " API",
    "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
  },
  "item": [
    {
      "name": "简单转账",
      "request": {
        "method": "POST",
        "header": [{"key": "Content-Type", "value": "application/json"}],
        "url": "{{base_url}}/transactions/simple-transfer",
        "body": {
          "mode": "raw",
          "raw": "{\n  \"from_address\": \"{{from_address}}\",\n  \"to_address\": \"{{to_address}}\",\n  \"amount\": \"1000000000000000000\"\n}"
        }
      }
    }
  ],
  "variable": [
    {"key": "base_url", "value": "http://localhost:8080/api/v1"}
  ]
}
```

### **测试脚本**
```bash
#!/bin/bash
#WES API 测试脚本

BASE_URL="http://localhost:8080/api/v1"

# 测试健康检查
echo "测试区块链信息查询..."
curl -s $BASE_URL/blocks/info | jq .

# 测试账户余额查询
echo "测试账户余额查询..."
curl -s "$BASE_URL/accounts/0x1234567890abcdef1234567890abcdef12345678/balance" | jq .

# 测试简单转账
echo "测试简单转账..."
curl -s -X POST $BASE_URL/transactions/simple-transfer \
  -H "Content-Type: application/json" \
  -d '{
    "from_address": "0x1234567890abcdef1234567890abcdef12345678",
    "to_address": "0xabcdef1234567890abcdef1234567890abcdef12",
    "amount": "1000000000000000000"
  }' | jq .
```

## 📚 **更多资源**

- [区块链设计文档](../../docs/_COMPLETE_DESIGN_THEORY.md)
- [Transaction Proto定义](../../pb/blockchain/core/transaction.proto)
- [API架构说明](./SYSTEMATIC_API_ANALYSIS.md)
- [URL命名规范](./URL_NAMING_STANDARDS.md)

---

**文档版本**: v1.0  
**最后更新**: $(date)  
**维护团队**:WES开发团队 