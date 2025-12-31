# 合约加载器服务（internal/core/engines/wasm/loader）

## 📋 模块定位

本模块是WES系统中**WASM智能合约字节码加载器的基础实现**，负责**从内容寻址存储加载合约字节码并进行基础验证**。通过**确定性路径构建**和**标准化地址解析**，提供**可靠的合约加载能力**，支撑**WASM引擎的合约执行需求**。

## 🎯 设计原则

- **确定性路径构建**：基于配置 + 内容哈希的确定性路径，无歧义
- **内容寻址优先**：严格遵循内容寻址原则，路径由配置和哈希决定
- **职责单一**：专注字节码加载和基础验证，不涉及区块链状态管理
- **架构边界清晰**：engines层只负责"加载字节码 → 执行"，UTXO验证由TX层负责

## ✅ 核心职责（当前实现）

1. **合约地址解析**：解析64位十六进制字符串为32字节内容哈希
2. **字节码加载**：从确定性路径读取WASM字节码文件
3. **格式验证**：验证WASM魔数和版本号
4. **错误处理**：提供详细的调试信息和错误提示

## 📁 模块文件结构

```
internal/core/engines/wasm/loader/
├── 📦 contract_loader.go          # ContractLoader 核心实现
│   ├── NewContractLoader()        # 构造函数（接受logger和fileStoreRootPath）
│   ├── LoadContract()             # 核心加载方法
│   ├── parseContractAddress()     # 地址解析（64位hex → 32字节）
│   ├── readBytecodeFromStorage()  # 确定性路径构建 + 文件读取
│   └── validateWASMFormat()       # WASM格式验证（魔数和版本号）
└── ⚠️ errors.go                   # 错误定义（预留，当前未使用）
```

## 🔄 合约加载流程

```
1. 接收合约地址（64位hex）
   ↓
2. 解析为内容哈希（32字节）
   ↓
3. 构建确定性路径：
   fileStoreRootPath + hashHex[:2] + hashHex
   示例：data/files/d2/d2ef233ef664052a...
   ↓
4. 读取WASM字节码文件
   ↓
5. 验证WASM格式（魔数 0x00 0x61 0x73 0x6D + 版本号）
   ↓
6. 返回 WASMContract 对象
```

## 🔑 关键方法说明

### LoadContract(ctx, contractAddress)
- **功能**：加载合约字节码
- **输入**：64位十六进制字符串（不带0x前缀）
- **输出**：`*types.WASMContract` 或错误
- **性能**：取决于文件系统I/O，通常 10-100ms

### parseContractAddress(address)
- **功能**：解析合约地址为内容哈希
- **验证**：严格64位hex，拒绝0x前缀
- **输出**：32字节内容哈希

### readBytecodeFromStorage(contentHash)
- **功能**：从确定性路径读取字节码
- **路径公式**：`fileStoreRootPath / hashHex[:2] / hashHex`
- **错误处理**：详细的调试信息（路径、哈希、建议操作）

### validateWASMFormat(bytecode)
- **功能**：验证WASM字节码格式
- **检查项**：
  - 最小长度（≥8字节）
  - 魔数（0x00 0x61 0x73 0x6D）
  - 版本号（0x01 0x00 0x00 0x00）

## 🎯 架构边界

### ✅ 本模块负责
- 字节码加载和基础格式验证
- 地址解析和路径构建
- 文件系统I/O操作

### ❌ 本模块不负责
- 区块链UTXO状态验证（由TX层负责）
- 合约执行（由runtime子模块负责）
- 缓存管理（简化设计，暂不实现）
- 资源元信息查询（由repository层负责）

## 🚧 未来扩展规划

以下功能为**未来扩展方向**，当前**尚未实现**：

- ⏳ **多层缓存策略**（L1内存缓存 + L2磁盘缓存）
- ⏳ **版本管理支持**（`LoadContractVersion` 方法）
- ⏳ **热点检测和预加载**（智能缓存优化）
- ⏳ **多存储后端支持**（IPFS、CDN等）
- ⏳ **性能监控和指标**（缓存命中率、加载时间等）

> 📝 **说明**：当前实现遵循"最小可用原则"，优先保证功能正确性和架构清晰性。性能优化（如缓存）将根据实际需求逐步添加。

## 🔗 依赖关系

```
ContractLoader
├── logger (log.Logger)              # 日志记录
└── fileStoreRootPath (string)       # 文件存储根路径（从配置注入）
```

**注意**：重构后的实现**不再依赖** `ResourceManager`，直接通过确定性路径构建读取文件。

---

> 📝 **设计哲学**：简单、确定、可靠。通过确定性路径构建避免复杂的元数据管理，通过严格的格式验证确保字节码安全。
