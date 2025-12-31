# WES 资源元数据标准化规范

> **版本**: v1.0.0  
> **更新日期**: 2025-11-XX  
> **状态**: ✅ 已实现

---

## 📋 概述

本文档定义了 WES 链上资源元数据字段的标准化规范，确保节点、SDK 和前端应用对元数据字段的理解和使用保持一致。

## 🎯 设计原则

1. **严格链上数据**：所有元数据字段必须来自链上存储，不推导、不默认
2. **向后兼容**：支持旧版本资源（可能缺少部分元数据字段）
3. **可扩展性**：通过 `custom_attributes` 支持业务层扩展

---

## 📊 标准元数据字段

### 核心字段（Protocol Buffer 定义）

根据 `pb/blockchain/block/transaction/resource/resource.proto`，标准元数据字段包括：

| 字段名 | 类型 | 必填 | 说明 | 来源 |
|--------|------|------|------|------|
| `name` | `string` | 否 | 资源显示名称（用户友好） | `Resource.name` |
| `version` | `string` | 否 | 版本标识（如 "1.0.0", "v2.1.3"） | `Resource.version` |
| `description` | `string` | 否 | 资源描述 | `Resource.description` |
| `creator_address` | `string` | 否 | 创建者地址（Base58 格式） | `Resource.creator_address` |
| `created_timestamp` | `uint64` | 是 | 创建时间戳（Unix 秒） | `Resource.created_timestamp` |
| `original_filename` | `string` | 否 | 原始文件名（含扩展名） | `Resource.original_filename` |
| `file_extension` | `string` | 否 | 文件扩展名（如 ".wasm", ".onnx"） | `Resource.file_extension` |
| `custom_attributes` | `map<string, string>` | 否 | 自定义属性（业务层扩展） | `Resource.custom_attributes` |

### 扩展字段（通过 custom_attributes）

| 键名 | 值类型 | 说明 | 示例 |
|------|--------|------|------|
| `tags` | `string` (逗号分隔) | 标签列表 | `"deFi,smart-contract,wasm"` |
| `category` | `string` | 业务分类 | `"finance"`, `"gaming"` |
| `license` | `string` | 许可证 | `"MIT"`, `"Apache-2.0"` |
| `homepage` | `string` | 项目主页 URL | `"https://example.com"` |
| `repository` | `string` | 代码仓库 URL | `"https://github.com/..."` |

---

## 🔌 JSON-RPC 方法规范

### `wes_getResourceByContentHash`

**用途**：根据 content_hash 查询资源元数据

**参数**：
```json
{
  "content_hash": "0xabc123..."
}
```

**返回**：
```json
{
  "content_hash": "0xabc123...",
  "name": "My Contract",
  "version": "1.0.0",
  "description": "A smart contract for...",
  "creator_address": "WES1...",
  "created_timestamp": 1234567890,
  "original_filename": "contract.wasm",
  "file_extension": ".wasm",
  "custom_attributes": {
    "tags": "deFi,smart-contract",
    "license": "MIT"
  },
  "category": "EXECUTABLE",
  "executable_type": "CONTRACT",
  "mime_type": "application/wasm",
  "size": 12345
}
```

**字段说明**：
- 所有字段都是可选的（除了 `content_hash` 和 `created_timestamp`）
- 如果链上没有某个字段，返回时该字段为 `null` 或不存在
- `custom_attributes` 中的 `tags` 如果是逗号分隔字符串，SDK 应解析为数组

---

## 📦 SDK 映射规范

### client-sdk-js

```typescript
interface ResourceInfo {
  // 核心字段
  resourceId: Uint8Array;
  resourceType: 'contract' | 'model' | 'static';
  contentHash: Uint8Array;
  size: number;
  mimeType?: string;
  lockingConditions: LockingCondition[];
  createdAt: Date;
  
  // 标准元数据字段（严格来自链上）
  name?: string;              // 来自 Resource.name
  version?: string;           // 来自 Resource.version
  description?: string;       // 来自 Resource.description
  creatorAddress?: string;    // 来自 Resource.creator_address
  tags?: string[];            // 来自 Resource.custom_attributes["tags"]（解析为数组）
  customAttributes?: Record<string, string>; // 来自 Resource.custom_attributes
}
```

### 字段提取逻辑

```typescript
// 1. 从 Resource 对象提取标准字段
const name = resource.name || undefined;  // 空字符串视为不存在
const version = resource.version || undefined;
const description = resource.description || undefined;
const creatorAddress = resource.creator_address || undefined;

// 2. 从 custom_attributes 提取 tags
const tags = resource.custom_attributes?.["tags"]
  ? resource.custom_attributes["tags"].split(',').map(t => t.trim()).filter(t => t)
  : undefined;

// 3. 如果字段为空字符串，视为不存在
if (name === '') name = undefined;
if (version === '') version = undefined;
```

---

## 🔄 版本管理数据结构

### 版本关系存储

版本关系通过 `custom_attributes` 存储：

| 键名 | 值 | 说明 |
|------|-----|------|
| `parent_version` | `content_hash` | 父版本（升级来源）的 content_hash |
| `version_chain` | `content_hash1,content_hash2,...` | 版本链（从初始版本到当前版本） |
| `is_deprecated` | `"true"` / `"false"` | 是否已弃用 |

### 版本查询方法

**新增 RPC 方法**：`wes_getResourceVersions`

**参数**：
```json
{
  "content_hash": "0xabc123..."
}
```

**返回**：
```json
{
  "current_version": {
    "content_hash": "0xabc123...",
    "version": "2.0.0",
    "deployed_at": 1234567890,
    "deployer": "WES1...",
    "tx_hash": "0xtx123..."
  },
  "versions": [
    {
      "content_hash": "0xabc123...",
      "version": "2.0.0",
      "deployed_at": 1234567890,
      "deployer": "WES1...",
      "tx_hash": "0xtx123...",
      "status": "active"
    },
    {
      "content_hash": "0xdef456...",
      "version": "1.0.0",
      "deployed_at": 1234567800,
      "deployer": "WES1...",
      "tx_hash": "0xtx456...",
      "status": "deprecated"
    }
  ]
}
```

---

## 🔧 代码/ABI 查询规范

### `wes_getResourceCode`

**用途**：获取资源的代码/字节码

**参数**：
```json
{
  "resource_id": "txId:outputIndex",
  "code_type": "wasm" | "source"
}
```

**返回**：
```json
{
  "code_type": "wasm",
  "content": "0x0061736d01000000...",  // 十六进制编码的字节码
  "size": 12345
}
```

**说明**：
- `code_type="wasm"`: 返回 WASM 字节码（十六进制）
- `code_type="source"`: 如果链上存储了源码，返回源码；否则返回错误

### `wes_getResourceABI`

**用途**：获取资源的 ABI（应用二进制接口）

**参数**：
```json
{
  "resource_id": "txId:outputIndex"
}
```

**返回**：
```json
{
  "abi_version": "v1",
  "methods": [
    {
      "name": "transfer",
      "type": "write",
      "parameters": [
        {"name": "to", "type": "string"},
        {"name": "amount", "type": "uint64"}
      ],
      "return_type": "void"
    }
  ]
}
```

---

## ✅ 实施检查清单

### 节点层面
- [x] `Resource` protobuf 已定义标准元数据字段
- [x] `wes_getResourceByContentHash` 返回完整元数据
- [ ] `wes_getResourceCode` 实现（待实现）
- [ ] `wes_getResourceABI` 实现（待实现）
- [ ] `wes_getResourceVersions` 实现（待实现）

### SDK 层面
- [x] `ResourceInfo` 接口定义标准元数据字段
- [ ] 元数据字段提取逻辑标准化（待实现）
- [ ] 支持新的 RPC 方法（待实现）

### 前端层面
- [x] UI 层条件显示逻辑（已完成）
- [x] 移除推导和默认值（已完成）

---

## 📝 更新日志

- **2025-11-XX**: 创建标准化规范文档
  - 定义标准元数据字段
  - 设计版本管理数据结构
  - 设计代码/ABI 查询方法

---

## 🔗 相关文档

- [WES JSON-RPC API 规范](./jsonrpc_spec.md)
- [资源分类协议](../../../_docs/design/protobuf/RESOURCE_CLASSIFICATION_PROTOCOL.md)
- [资源详情页链上数据合规性](../../../../workbench/contract-workbench.git/_dev/CHAIN_DATA_COMPLIANCE.md)

