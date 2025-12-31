# 系统合约（contracts/system）

---

## ⚠️ 状态说明

- **状态**：**DEPRECATED（已废弃）**
- **废弃日期**：2025-11-15
- **原因**：这些合约使用旧的 HostABI（`state_get`/`state_set`），不符合当前 WES 架构（HostABI v1.1）

---

## 📌 当前内容

本目录包含以下内容：

### 实际存在的合约

| 合约名称 | 源文件 | WASM 文件 | 状态 | 说明 |
|---------|--------|-----------|------|------|
| **block_query_contract** | `block_query_contract.go` | `block_query_contract.wasm` (123KB) | ⚠️ 已废弃 | 使用旧的 HostABI，未实际使用 |
| **simple_contract** | `simple_contract.go` | `simple_contract.wasm` (121KB) | ⚠️ 仅测试用 | 仅用于测试代码，使用旧的 HostABI |

### 已删除的占位符

以下文件已被删除（仅为文本占位符，不是真正的 WASM 文件）：
- `debug_contract.wasm` (30B)
- `governance_contract.wasm` (30B)
- `staking_contract.wasm` (30B)
- `transfer_contract.wasm` (30B)
- `faucet_contract.wasm` (33B)

---

## 🎯 废弃原因

1. **使用旧的 HostABI**：
   - 这些合约使用 `state_get`/`state_set`/`state_exists` 等旧的 HostABI
   - 当前 WES 架构使用 HostABI v1.1，不再支持这些旧接口

2. **没有实际使用场景**：
   - `simple_contract` 仅用于测试代码（`service_test.go`）
   - `block_query_contract` 没有找到实际使用场景
   - 其他合约只是占位符，从未实现

3. **架构不匹配**：
   - WES 当前架构不依赖"系统合约"的概念
   - 平台功能通过 HostABI 直接提供，不需要通过合约实现

---

## 📋 历史说明

**原始设计意图**（已废弃）：
- 提供区块链网络运行所需的基础功能和服务
- 作为"系统级合约"直接集成到区块链节点中
- 提供网络治理、区块查询、调试和基础服务

**当前 WES 架构**：
- WES 平台通过 HostABI v1.1 直接提供平台能力
- 不需要通过"系统合约"来实现平台功能
- 所有合约都是用户部署的普通合约

---

## 🔄 迁移建议

### 对于测试用途

如果 `simple_contract` 仍需要用于测试，建议：

1. **移动到 `contracts/examples/`**：
   ```bash
   mv contracts/system/simple_contract.* contracts/examples/basic/simple-contract-test/
   ```

2. **更新为使用 HostABI v1.1**：
   - 移除 `state_get`/`state_set` 调用
   - 使用新的 HostABI 接口（如 `AppendStateOutput`）

3. **更新测试代码**：
   - 更新 `service_test.go` 中的路径引用

### 对于功能需求

如果需要区块查询、治理等功能：

1. **使用 HostABI v1.1**：
   - 直接通过 HostABI 调用平台能力
   - 不需要通过合约实现

2. **使用 SDK 模板**：
   - 参考 **合约 SDK 模块**（**@go.mod `github.com/weisyn/contract-sdk-go`**）的模板
   - 使用 SDK 提供的业务语义接口

---

## 📚 相关文档

- [合约平台总览](../README.md)
- [资源级示例](../examples/README.md)
- **合约 SDK 模块**（**@go.mod `github.com/weisyn/contract-sdk-go`**）：SDK 模板库

---

## ⚠️ 下一步行动

**建议**：
1. ✅ 已删除空的占位符文件
2. ⚠️ 评估 `simple_contract` 是否仍需要用于测试
3. ⚠️ 如果不需要，建议删除整个 `contracts/system/` 目录
4. ⚠️ 如果需要保留测试合约，移动到 `contracts/examples/` 并更新为 HostABI v1.1

---

**维护者**：合约平台组  
**最后更新**：2025-11-15
