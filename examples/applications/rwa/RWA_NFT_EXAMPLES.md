# 🏛️ WES RWA 和 NFT 应用示例

基于 WES 智能合约系统和完整的资产类型支持，以下是 RWA（现实世界资产）和 NFT 的实际应用示例。

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

### 1. 部署房地产代币合约

```bash
curl -X POST http://localhost:8080/api/v1/contract/deploy 
  -H "Content-Type: application/json" 
  -d '{
    "wasm_code": "0x...", // 房地产合约的 WASM 代码
    "owner": "CRealEstateCompany...",
    "owner_public_key": "02...",
    "init_params": "0x7b226e616d65223a22e58c97e4baace69c9de998b3e59cb0e4baa7....", // JSON参数的hex编码
    "执行费用_limit": 2000000,
    "metadata": {
      "name": "BeijingCommercialRealEstate",
      "version": "1.0.0",
      "description": "北京朝阳区CBD核心商业地产代币化合约",
      "property_type": "commercial_real_estate",
      "location": "Beijing_Chaoyang_CBD",
      "total_value": "50000000",
      "currency": "CNY",
      "total_tokens": "1000",
      "token_type": "fungible"
    }
  }'
```

### 2. 铸造房地产代币

```bash
curl -X POST http://localhost:8080/api/v1/contract/call 
  -H "Content-Type: application/json" 
  -d '{
    "contract_hash": "property_contract_hash_...",
    "function": "mint_property_tokens",
    "parameters": "0x000003e8", // 1000 tokens in hex
    "caller": "CRealEstateCompany...",
    "caller_public_key": "02...",
    "执行费用_limit": 500000
  }'
```

### 3. 查询房地产代币信息

```bash
curl -X GET "http://localhost:8080/api/v1/contract/query?contract_hash=property_contract_hash_...&function=get_property_info"
```

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

### 1. 部署艺术品 NFT 合约

```bash
curl -X POST http://localhost:8080/api/v1/contract/deploy 
  -H "Content-Type: application/json" 
  -d '{
    "wasm_code": "0x...", // NFT 合约的 WASM 代码
    "owner": "CArtistAddress...",
    "owner_public_key": "03...",
    "init_params": "0x7b226e616d65223a224469676974616c4172745f436f6c6c656374696f6e....",
    "执行费用_limit": 1500000,
    "metadata": {
      "name": "DigitalArtCollection",
      "version": "1.0.0",
      "description": "知名数字艺术家作品集合",
      "artist": "Zhang Wei",
      "collection_size": "100",
      "token_standard": "NFT"
    }
  }'
```

### 2. 铸造艺术品 NFT

```bash
curl -X POST http://localhost:8080/api/v1/contract/call 
  -H "Content-Type: application/json" 
  -d '{
    "contract_hash": "art_nft_contract_hash_...",
    "function": "mint_artwork_nft",
    "parameters": "0x7b22746f6b656e5f6964223a2241525457305f3030312f2f", // JSON参数编码
    "caller": "CArtistAddress...",
    "caller_public_key": "03...",
    "执行费用_limit": 300000
  }'
```

**参数解码后的内容**：
```json
{
  "token_id": "ARTW_001",
  "title": "数字梦境",
  "description": "一幅融合了传统山水与现代数字技术的艺术作品",
  "image_url": "https://ipfs.io/ipfs/QmYx7...",
  "artist": "Zhang Wei",
  "creation_date": "2024-01-15",
  "medium": "Digital Mixed Media",
  "dimensions": "3840x2160",
  "edition": "1/1",
  "provenance": "Artist Studio → First Owner",
  "certificate_url": "https://ipfs.io/ipfs/QmAb8..."
}
```

### 3. 查询 NFT 详情

```bash
curl -X GET "http://localhost:8080/api/v1/contract/query?contract_hash=art_nft_contract_hash_...&function=get_nft_metadata&parameters=0x4152545720303031" // "ARTW_001" in hex
```

---

## 📈 **股票代币化**

### 场景描述
将传统股票转换为区块链代币，实现 24/7 交易和分割所有权。

### 1. 部署股票代币合约

```bash
curl -X POST http://localhost:8080/api/v1/contract/deploy 
  -H "Content-Type: application/json" 
  -d '{
    "wasm_code": "0x...",
    "owner": "CSecuritiesCompany...",
    "owner_public_key": "02...",
    "metadata": {
      "name": "TokenizedStocks",
      "description": "传统股票的区块链代币化",
      "compliance": "SEC_Regulated",
      "custodian": "ABC Securities Co."
    }
  }'
```

### 2. 代币化苹果公司股票

```bash
curl -X POST http://localhost:8080/api/v1/contract/call 
  -H "Content-Type: application/json" 
  -d '{
    "contract_hash": "stock_contract_hash_...",
    "function": "tokenize_stock",
    "parameters": "0x7b2273746f636b5f73796d626f6c223a224141504c22...", // 编码后的股票信息
    "caller": "CSecuritiesCompany...",
    "执行费用_limit": 400000
  }'
```

### 3. 查询股票代币余额

```bash
curl -X GET "http://localhost:8080/api/v1/contract/query?contract_hash=stock_contract_hash_...&function=balance_of&parameters=0x..." // 用户地址的hex编码
```

---

## 🏭 **商品代币化**

### 场景描述
将实物商品（如黄金、石油）代币化，实现更灵活的交易和存储。

### 1. 黄金代币化合约

```bash
curl -X POST http://localhost:8080/api/v1/contract/deploy 
  -H "Content-Type: application/json" 
  -d '{
    "wasm_code": "0x...",
    "owner": "CCommodityVault...",
    "metadata": {
      "name": "GoldTokenization",
      "description": "实物黄金的数字化代币",
      "commodity_type": "Gold",
      "purity": "99.99%",
      "vault_location": "Shanghai Gold Exchange Vault",
      "audit_firm": "SGS Precious Metals"
    }
  }'
```

### 2. 铸造黄金代币（代表 100 盎司黄金）

```bash
curl -X POST http://localhost:8080/api/v1/contract/call 
  -H "Content-Type: application/json" 
  -d '{
    "contract_hash": "gold_contract_hash_...",
    "function": "mint_gold_tokens",
    "parameters": "0x0000000000000064", // 100 ounces in hex
    "caller": "CCommodityVault...",
    "执行费用_limit": 300000
  }'
```

---

## 🆔 **身份证明 NFT**

### 场景描述
将学历证书、职业证书等身份信息铸造为 NFT，防止伪造并便于验证。

### 1. 部署教育证书 NFT 合约

```bash
curl -X POST http://localhost:8080/api/v1/contract/deploy 
  -H "Content-Type: application/json" 
  -d '{
    "wasm_code": "0x...",
    "owner": "CUniversityAddress...",
    "metadata": {
      "name": "EducationalCertificates",
      "description": "大学学历证书 NFT 系统",
      "institution": "Tsinghua University",
      "authority": "Ministry of Education",
      "verification_standard": "ISO 21001"
    }
  }'
```

### 2. 颁发学位证书 NFT

```bash
curl -X POST http://localhost:8080/api/v1/contract/call 
  -H "Content-Type: application/json" 
  -d '{
    "contract_hash": "certificate_contract_hash_...",
    "function": "issue_degree_certificate",
    "parameters": "0x7b2273747564656e745f6964223a22323032313030313233....", // 学生信息编码
    "caller": "CUniversityAddress...",
    "执行费用_limit": 250000
  }'
```

**证书信息**：
```json
{
  "student_id": "2021001234",
  "student_name": "Li Ming",
  "degree": "Master of Computer Science",
  "graduation_date": "2024-06-30",
  "gpa": "3.8/4.0",
  "thesis_title": "Blockchain Applications in Education",
  "certificate_id": "THU_CS_2024_001234",
  "issuer": "Tsinghua University",
  "verification_url": "https://verify.tsinghua.edu.cn/cert/001234"
}
```

---

## 🛠️ **实施指南**

### 合约开发步骤

1. **设计资产模型**
   - 确定资产类型（FT/NFT/SFT）
   - 定义元数据结构
   - 设计业务逻辑

2. **编写 WASM 合约**
   ```rust
   // 示例：房地产代币合约片段
   #[derive(Serialize, Deserialize)]
   pub struct PropertyInfo {
       pub name: String,
       pub location: String,
       pub total_value: u64,
       pub total_tokens: u64,
       pub remaining_tokens: u64,
   }
   
   #[no_mangle]
   pub extern "C" fn mint_property_tokens() {
       // 铸造房地产代币逻辑
   }
   ```

3. **部署和测试**
   - 使用合约 API 部署
   - 进行功能测试
   - 安全审计

4. **集成应用**
   - 前端界面开发
   - 用户体验优化
   - 监管合规

### 最佳实践

1. **安全考虑**
   - 实施访问控制
   - 添加暂停机制
   - 定期安全审计

2. **合规要求**
   - 了解当地法规
   - 实施 KYC/AML
   - 数据隐私保护

3. **用户体验**
   - 简化操作流程
   - 提供清晰的状态反馈
   - 多语言支持

---

## 🎯 **结论**

WES 系统的设计为 RWA 和 NFT 提供了：

- ✅ **完整的资产类型支持**：FT/NFT/SFT 全覆盖
- ✅ **强大的合约能力**：复杂业务逻辑支持
- ✅ **高性能架构**：EUTXO 模型的并发优势
- ✅ **企业级特性**：合规、安全、可审计

**你可以基于这个系统构建几乎所有类型的 RWA 和 NFT 应用！** 🚀
