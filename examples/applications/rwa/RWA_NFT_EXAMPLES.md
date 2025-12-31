# 🏛️ WES RWA 和 NFT 应用示例

⚠️ **重要说明**：本文档为**概念演示**和**设计参考**，展示 RWA 和 NFT 的应用场景与合约交互模式。

**当前状态**：
- 示例代码使用旧版 API 参数格式，**需要根据实际 API 更新后才能运行**
- 实际 API 文档请参考：`internal/api/http/handlers/contract.go` 和 `pkg/interfaces/tx/`
- 建议先运行 `examples/basic/hello-world` 了解实际 API 使用方式

---

## 📋 **目录**
- [房地产代币化](#房地产代币化)
- [艺术品 NFT](#艺术品-nft)
- [股票代币化](#股票代币化)
- [商品代币化](#商品代币化)
- [身份证明 NFT](#身份证明-nft)
- [实施指南](#实施指南)

---

## 🏠 **房地产代币化**

### 场景描述
将价值 5000 万的北京商业地产代币化，分割成 1000 个代币，每个代币代表 0.1% 的所有权。

### 1. 部署房地产代币合约（概念示例）

⚠️ **API 更新说明**：实际部署请使用以下参数格式：
- `deployer_private_key`（私钥 hex）
- `contract_file_path`（WASM 文件路径）
- `config`（包含 `abi_version` 和 `exported_functions`）
- `name`、`description`

```bash
# 概念示例（需要更新为实际 API 格式）
curl -X POST http://localhost:28680/api/v1/contract/deploy 
  -H "Content-Type: application/json" 
  -d '{
    "deployer_private_key": "your_private_key_hex",
    "contract_file_path": "/path/to/real_estate.wasm",
    "config": {
      "abi_version": "v1",
      "exported_functions": ["mint_property_tokens", "transfer", "get_property_info"]
    },
    "name": "BeijingCommercialRealEstate",
    "description": "北京朝阳区CBD核心商业地产代币化合约"
  }'
```

### 2. 铸造房地产代币（概念示例）

⚠️ **API 更新说明**：实际调用请使用：
- `caller_private_key`（私钥 hex）
- `contract_address`（content_hash）
- `method_name`（方法名）
- `parameters`（map 格式）
- `execution_fee_limit`（执行费用限制）

```bash
# 概念示例（需要更新为实际 API 格式）
curl -X POST http://localhost:28680/api/v1/contract/call 
  -H "Content-Type: application/json" 
  -d '{
    "caller_private_key": "your_private_key_hex",
    "contract_address": "content_hash_from_deploy_response",
    "method_name": "mint_property_tokens",
    "parameters": {
      "amount": 1000
    },
    "execution_fee_limit": 500000
  }'
```

### 3. 查询房地产代币信息

⚠️ **注意**：当前 `/contract/query` 端点未实现，请使用 `/contract/call` 进行查询操作。

**预期响应**：
```json
{
  "success": true,
  "data": {
    "property_name": "北京朝阳区CBD商业地产A座",
    "total_value": "50,000,000 CNY",
    "total_tokens": "1,000",
    "remaining_tokens": "850",
    "token_price": "50,000 CNY per token",
    "property_address": "北京市朝阳区建国门外大街1号",
    "certification": "京房权证朝字第123456号"
  }
}
```

---

## 🎨 **艺术品 NFT**

### 场景描述
著名画家的数字艺术作品铸造为 NFT，包含完整的创作信息和所有权证明。

### 概念说明
本节展示 NFT 合约的设计模式和交互流程。实际实现时需要：
1. 参考 `examples/basic/hello-world` 的实际 API 调用方式
2. 使用 `deployer_private_key`、`contract_file_path`、`config` 等实际参数
3. 合约调用使用 `caller_private_key`、`method_name`、`parameters`、`execution_fee_limit`

---

## 📈 **股票代币化**

### 场景描述
将传统股票转换为区块链代币，实现 24/7 交易和分割所有权。

**注意**：股票代币化涉及证券合规要求，实际应用需要满足相关法规。

---

## 🏭 **商品代币化**

### 场景描述
将实物商品（如黄金、石油）代币化，实现更灵活的交易和存储。

---

## 🆔 **身份证明 NFT**

### 场景描述
将身份证明、学历证书等凭证铸造为 NFT，实现可验证、不可篡改的数字凭证。

---

## 📚 **实施指南**

### 实际开发步骤

1. **学习基础**
   - 先完成 `examples/basic/hello-world`，了解实际 API 使用
   - 理解合约部署与调用的完整流程

2. **合约开发**
   - 参考 `contracts/templates/` 开发合约
   - 使用 TinyGo 编译为 WASM

3. **API 对接**
   - 使用实际的 API 参数格式
   - 部署：`POST /api/v1/contract/deploy`
   - 调用：`POST /api/v1/contract/call`

4. **测试验证**
   - 在测试环境完整验证
   - 确保所有功能符合预期

### 参考资源
- 实际 API 文档：`internal/api/http/handlers/contract.go`
- 公共接口：`pkg/interfaces/tx/`
- 可运行示例：`examples/basic/hello-world/`
- 合约模板：`contracts/templates/`

---

**文档状态**：🚧 概念演示，需要根据实际 API 更新
**最后更新**：2025-01

---
