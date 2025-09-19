# WES 标准合约接口规范

## 📋 **规范概述**

　　本文档定义WES智能合约的标准接口规范，基于URES（统一资源执行状态）模型设计，确保合约的互操作性、标准化和开发一致性。

**版本**：v1.0  
**更新时间**：2024年12月  
**适用范围**：WES 4.0+ 执行域

---

## 🎯 **设计原则**

### **1. URES模型兼容**
- **无状态设计**：合约逻辑不依赖持久化状态存储
- **UTXO导向**：所有资产以UTXO形式管理和流转
- **纯函数计算**：合约执行为确定性的纯函数变换

### **2. 标准化接口**
- **统一签名**：标准化的函数签名和参数格式
- **一致错误码**：统一的错误处理和返回码体系
- **通用元数据**：标准化的合约元数据格式

### **3. 模块化设计**
- **功能分离**：将复杂功能拆分为独立模块
- **接口抽象**：通过接口实现功能的可插拔性
- **组合优于继承**：通过组合实现功能扩展

---

## 📚 **核心接口定义**

### **IContractBase - 基础合约接口**

所有WES合约必须实现的基础接口：

```go
// ==================== 基础合约接口 ====================

// Initialize 合约初始化
// 参数：initParams []byte - 初始化参数（JSON或其他格式）
// 返回：errorCode uint32 - 错误码（0=成功）
//export Initialize
func Initialize() uint32

// GetMetadata 获取合约元数据  
// 返回：通过set_return_data设置JSON格式的合约信息
//export GetMetadata
func GetMetadata() uint32

// GetVersion 获取合约版本
// 返回：通过set_return_data设置版本字符串
//export GetVersion  
func GetVersion() uint32
```

### **ITokenStandard - 代币标准接口**

代币类合约的标准接口（ERC20风格）：

```go
// ==================== 代币标准接口 ====================

// Transfer 转账代币
// 通过合约调用参数传递：to, amount, tokenId
// 返回：errorCode uint32
//export Transfer
func Transfer() uint32

// GetBalance 查询余额
// 通过合约调用参数传递：address, tokenId
// 返回：通过set_return_data设置余额信息
//export GetBalance
func GetBalance() uint32

// GetTotalSupply 获取总供应量
// 通过合约调用参数传递：tokenId（可选）
// 返回：通过set_return_data设置供应量信息
//export GetTotalSupply  
func GetTotalSupply() uint32

// Approve 授权代币使用权
// 通过合约调用参数传递：spender, amount, tokenId
// 返回：errorCode uint32
//export Approve
func Approve() uint32
```

### **INonFungibleToken - NFT标准接口**

NFT类合约的标准接口（ERC721风格）：

```go
// ==================== NFT标准接口 ====================

// MintNFT 铸造NFT
// 通过合约调用参数传递：to, tokenId, metadata
// 返回：errorCode uint32
//export MintNFT
func MintNFT() uint32

// TransferNFT 转移NFT
// 通过合约调用参数传递：from, to, tokenId
// 返回：errorCode uint32
//export TransferNFT
func TransferNFT() uint32

// GetTokenInfo 获取NFT信息
// 通过合约调用参数传递：tokenId
// 返回：通过set_return_data设置NFT详细信息
//export GetTokenInfo
func GetTokenInfo() uint32

// SetTokenURI 设置NFT元数据URI
// 通过合约调用参数传递：tokenId, uri
// 返回：errorCode uint32
//export SetTokenURI
func SetTokenURI() uint32
```

### **IGovernance - 治理合约接口**

治理类合约的标准接口：

```go
// ==================== 治理合约接口 ====================

// CreateProposal 创建提案
// 通过合约调用参数传递：title, description, actions
// 返回：errorCode uint32，通过set_return_data设置提案ID
//export CreateProposal
func CreateProposal() uint32

// Vote 投票
// 通过合约调用参数传递：proposalId, vote, votingPower
// 返回：errorCode uint32
//export Vote
func Vote() uint32

// ExecuteProposal 执行提案
// 通过合约调用参数传递：proposalId
// 返回：errorCode uint32
//export ExecuteProposal
func ExecuteProposal() uint32

// GetProposalInfo 获取提案信息
// 通过合约调用参数传递：proposalId
// 返回：通过set_return_data设置提案详细信息
//export GetProposalInfo
func GetProposalInfo() uint32
```

---

## 🔧 **宿主函数规范**

### **必需宿主函数**

所有合约必须能够访问的基础宿主函数：

```go
// ========== 基础环境函数 ==========
//go:wasmimport env get_caller           // 获取调用者地址
//go:wasmimport env get_contract_address // 获取合约地址
//go:wasmimport env set_return_data      // 设置返回数据
//go:wasmimport env emit_event           // 发出事件
//go:wasmimport env get_contract_init_params // 获取初始化参数

// ========== UTXO操作函数 ==========
//go:wasmimport env create_utxo_output    // 创建UTXO输出
//go:wasmimport env execute_utxo_transfer // 执行UTXO转移
//go:wasmimport env query_utxo_balance    // 查询UTXO余额

// ========== 内存管理函数 ==========
//go:wasmimport env malloc                // 内存分配
```

### **可选宿主函数**

根据合约功能需要可选择使用的宿主函数：

```go
// ========== 链信息查询函数 ==========
//go:wasmimport env get_block_height      // 获取区块高度
//go:wasmimport env get_timestamp         // 获取时间戳
//go:wasmimport env get_block_hash        // 获取区块哈希

// ========== 状态查询函数（特殊用途）==========
//go:wasmimport env state_get             // 状态查询（仅限只读）
//go:wasmimport env state_exists          // 状态存在性检查
```

---

## 📊 **数据格式规范**

### **错误码标准**

```go
const (
    SUCCESS           = 0   // 成功
    ERROR_INVALID_PARAMS = 1 // 无效参数
    ERROR_INSUFFICIENT_BALANCE = 2 // 余额不足
    ERROR_UNAUTHORIZED = 3   // 未授权操作
    ERROR_NOT_FOUND   = 4   // 资源不存在
    ERROR_ALREADY_EXISTS = 5 // 资源已存在
    ERROR_EXECUTION_FAILED = 6 // 执行失败
    ERROR_INVALID_STATE = 7  // 无效状态
    ERROR_TIMEOUT     = 8   // 操作超时
    ERROR_UNKNOWN     = 999 // 未知错误
)
```

### **元数据格式**

合约元数据必须为JSON格式：

```json
{
    "name": "合约名称",
    "symbol": "合约符号", 
    "version": "1.0.0",
    "description": "合约描述",
    "author": "开发者",
    "license": "许可证",
    "interfaces": ["IContractBase", "ITokenStandard"],
    "features": ["transfer", "mint", "burn"],
    "decimals": 18,
    "totalSupply": "1000000000"
}
```

### **事件格式**

事件必须包含标准字段：

```json
{
    "event": "事件名称",
    "contract": "合约地址", 
    "timestamp": 1640995200,
    "data": {
        // 事件具体数据
    }
}
```

---

## 🛠 **开发指南**

### **合约开发流程**

1. **选择合约模板**：根据功能需求选择对应的标准模板
2. **实现必需接口**：实现IContractBase及相关标准接口
3. **添加自定义逻辑**：在标准接口基础上添加特定业务逻辑
4. **测试验证**：使用WES测试框架进行功能和兼容性测试
5. **部署上链**：通过WES部署工具发布合约

### **最佳实践**

1. **错误处理**：所有函数必须返回明确的错误码
2. **参数验证**：对所有输入参数进行有效性检查
3. **日志记录**：重要操作必须发出事件日志
4. **安全检查**：验证调用者权限和操作合法性
5. **文档完整**：提供完整的合约说明和API文档

### **性能优化**

1. **内存管理**：合理使用malloc和内存回收
2. **计算优化**：避免不必要的复杂计算
3. **UTXO操作**：优化UTXO查询和创建逻辑
4. **数据压缩**：对大型数据使用适当的压缩格式

---

## 📋 **合规要求**

### **安全要求**

- ✅ 所有外部输入必须验证
- ✅ 关键操作必须权限检查
- ✅ 资产转移必须余额验证
- ✅ 错误信息不得泄露敏感信息

### **兼容性要求**

- ✅ 必须实现IContractBase接口
- ✅ 错误码必须符合标准定义
- ✅ 事件格式必须符合规范
- ✅ 元数据格式必须为标准JSON

### **性能要求**

- ✅ 单次调用执行费用消耗不得超过1,000,000
- ✅ 内存使用不得超过64MB
- ✅ 执行时间不得超过30秒
- ✅ 返回数据大小不得超过1MB

---

**📝 规范更新日志**

| 版本 | 日期 | 更新内容 |
|------|------|----------|
| v1.0 | 2024-12 | 初始版本，定义基础接口规范 |

---

*📄 本规范将随WES执行域的发展持续更新和完善。*
