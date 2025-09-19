# 🚀 WES 合约 API 使用指南

本指南展示了如何使用改进后的合约API，解决了硬编码公钥映射的问题。

## 📋 **目录**
- [部署合约](#部署合约)
- [调用合约](#调用合约)
- [查询合约](#查询合约)
- [错误处理](#错误处理)
- [最佳实践](#最佳实践)

---

## 🚀 **部署合约**

### 方式1：提供公钥（推荐）
```bash
curl -X POST http://localhost:8080/api/v1/contract/deploy \
  -H "Content-Type: application/json" \
  -d '{
    "wasm_code": "0x0061736d0100000001070160027f7f017f03020100070801046d61696e00000a09010700200020016a0b",
    "owner": "CUQ3g6P5WmFN289pPn7AAhnQ3T2cZRv2BR",
    "owner_public_key": "02349cb6a770701494eb716d0b430ebcff740a354b2ceaedb4d3a2b4bad2237896",
    "init_params": "0x",
    "执行费用_limit": 1000000,
    "metadata": {
      "name": "SimpleAdder",
      "version": "1.0.0",
      "description": "一个简单的加法合约"
    }
  }'
```

### 方式2：自动推导公钥（需要交易历史）
```bash
curl -X POST http://localhost:8080/api/v1/contract/deploy \
  -H "Content-Type: application/json" \
  -d '{
    "wasm_code": "0x0061736d0100000001070160027f7f017f03020100070801046d61696e00000a09010700200020016a0b",
    "owner": "CUQ3g6P5WmFN289pPn7AAhnQ3T2cZRv2BR",
    "init_params": "0x",
    "执行费用_limit": 1000000,
    "metadata": {
      "name": "SimpleAdder",
      "version": "1.0.0"
    }
  }'
```

### 成功响应示例
```json
{
  "success": true,
  "message": "合约部署交易已构建",
  "data": {
    "transaction_hash": "a1b2c3d4e5f6...",
    "deployment_type": "blockchain_level",
    "status": "built",
    "message": "合约部署交易已构建，请签名提交到区块链",
    "resource_type": "wasm_contract",
    "content_hash": "7f8a9b1c2d3e...",
    "code_size": 64,
    "metadata": {
      "name": "SimpleAdder",
      "version": "1.0.0",
      "description": "一个简单的加法合约"
    },
    "next_steps": [
      "使用 POST /api/v1/transactions/sign 签名交易",
      "签名成功后交易会自动提交到区块链",
      "合约将随交易永久存储在区块链账本上"
    ]
  },
  "timestamp": 1708123456
}
```

---

## 📞 **调用合约**

### 方式1：提供调用者公钥（推荐）
```bash
curl -X POST http://localhost:8080/api/v1/contract/call \
  -H "Content-Type: application/json" \
  -d '{
    "contract_hash": "7f8a9b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a",
    "function": "add",
    "parameters": "0x0000000a0000000b",
    "caller": "CUQ3g6P5WmFN289pPn7AAhnQ3T2cZRv2BR",
    "caller_public_key": "02349cb6a770701494eb716d0b430ebcff740a354b2ceaedb4d3a2b4bad2237896",
    "执行费用_limit": 100000,
    "reference_only": false,
    "expected_state_version": 0
  }'
```

### 方式2：自动推导调用者公钥
```bash
curl -X POST http://localhost:8080/api/v1/contract/call \
  -H "Content-Type: application/json" \
  -d '{
    "contract_hash": "7f8a9b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a",
    "function": "add",
    "parameters": "0x0000000a0000000b",
    "caller": "CUQ3g6P5WmFN289pPn7AAhnQ3T2cZRv2BR",
    "执行费用_limit": 100000
  }'
```

### 成功响应示例
```json
{
  "success": true,
  "message": "合约调用交易已构建",
  "data": {
    "transaction_hash": "b2c3d4e5f6a7...",
    "status": "built",
    "message": "合约调用交易已构建，请使用transaction_hash进行签名和提交",
    "function": "add",
    "contract": "7f8a9b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a",
    "执行费用_limit": 100000,
    "next_steps": [
      "使用 POST /api/v1/transactions/sign 签名交易",
      "签名成功后交易会自动提交到区块链"
    ]
  },
  "timestamp": 1708123456
}
```

---

## 🔍 **查询合约**

### 余额查询示例
```bash
curl -X GET "http://localhost:8080/api/v1/contract/query?contract_hash=7f8a9b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a&function=balance_of&parameters=0x1234567890abcdef"
```

### 响应示例（余额查询）
```json
{
  "success": true,
  "message": "合约查询成功",
  "data": {
    "balance": "2000000000",
    "amount": "2000000000",
    "formatted": "2,000,000,000",
    "raw_hex": "77359400"
  },
  "执行费用_used": 5000,
  "timestamp": 1708123456
}
```

### 合约信息查询
```bash
curl -X GET "http://localhost:8080/api/v1/contract/info/7f8a9b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a"
```

---

## ❌ **错误处理**

### 公钥与地址不匹配
```json
{
  "success": false,
  "message": "提供的公钥与地址不匹配",
  "error": "公钥与地址不匹配: 公钥生成的地址是 CSomeOtherAddress，但期望的地址是 CUQ3g6P5WmFN289pPn7AAhnQ3T2cZRv2BR",
  "timestamp": 1708123456
}
```

### 无法获取公钥（新地址）
```json
{
  "success": false,
  "message": "无法获取部署者公钥",
  "error": "无法获取地址 CNewAddress 的公钥。建议：\n1. 确保该地址已经进行过至少一笔交易（签名交易中包含公钥）\n2. 或者在请求中直接提供 owner_public_key 字段\n3. 如果是新地址，请先进行一笔简单转账来记录公钥",
  "timestamp": 1708123456
}
```

### 合约不存在
```json
{
  "success": false,
  "message": "合约不存在",
  "error": "合约不存在: 7f8a9b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a",
  "timestamp": 1708123456
}
```

---

## 💡 **最佳实践**

### 1. **优先提供公钥**
✅ **推荐做法**：
```json
{
  "owner": "CUQ3g6P5WmFN289pPn7AAhnQ3T2cZRv2BR",
  "owner_public_key": "02349cb6a770701494eb716d0b430ebcff740a354b2ceaedb4d3a2b4bad2237896"
}
```

### 2. **公钥格式支持**
- ✅ 压缩公钥（33字节）：`02349cb6a770701494eb716d0b430ebcff740a354b2ceaedb4d3a2b4bad2237896`
- ✅ 未压缩公钥（65字节）：`04349cb6a770701494eb716d0b430ebcff740a354b2ceaedb4d3a2b4bad2237896...`
- ✅ 带0x前缀：`0x02349cb6a770701494eb716d0b430ebcff740a354b2ceaedb4d3a2b4bad2237896`

### 3. **新地址处理流程**
1. **首次使用新地址**：必须提供公钥
2. **或者先进行一笔简单转账**：系统记录公钥后可自动推导
3. **企业用户**：建议始终提供公钥，提高可靠性

### 4. **错误重试策略**
```javascript
// JavaScript 示例
async function deployContract(contractData) {
  try {
    // 方式1：提供公钥
    return await apiCall({
      ...contractData,
      owner_public_key: userPublicKey
    });
  } catch (error) {
    if (error.message.includes('无法获取') && !contractData.owner_public_key) {
      throw new Error('请提供公钥: owner_public_key 字段');
    }
    throw error;
  }
}
```

### 5. **执行费用 限制建议**
- **合约部署**：`1,000,000` 执行费用
- **简单调用**：`100,000` 执行费用  
- **复杂计算**：`500,000` 执行费用
- **查询操作**：`50,000` 执行费用

---

## 🔧 **开发者注意事项**

### 开发环境默认地址
系统为开发测试提供了默认地址：
- 地址：`CUQ3g6P5WmFN289pPn7AAhnQ3T2cZRv2BR`
- 公钥：`02349cb6a770701494eb716d0b430ebcff740a354b2ceaedb4d3a2b4bad2237896`

### 生产环境使用
⚠️ **生产环境请务必**：
1. 使用真实的用户地址和公钥
2. 实现完整的交易历史查询功能
3. 添加必要的权限验证

---

**🎉 现在你可以使用更灵活、更安全的合约API了！**
