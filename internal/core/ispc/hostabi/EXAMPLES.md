# HostABI 原语使用示例和最佳实践

> **📌 文档类型**：使用指南和示例

---

## 📍 文档定位

本文档提供 **ISPC HostABI 17个最小原语**的详细使用示例和最佳实践指南，帮助合约开发者正确使用这些原语。

**目标受众**：
- 智能合约开发者
- WASM合约开发者
- ONNX模型开发者

---

## 🎯 17个最小原语分类

### 类别 A：确定性区块视图（4个）
- `GetBlockHeight` - 获取当前区块高度
- `GetBlockTimestamp` - 获取当前区块时间戳
- `GetBlockHash` - 获取指定高度的区块哈希
- `GetChainID` - 获取链标识符

### 类别 B：执行上下文（3个）
- `GetCaller` - 获取调用者地址
- `GetContractAddress` - 获取当前合约地址
- `GetTransactionID` - 获取当前交易ID

### 类别 C：UTXO查询（2个）
- `UTXOLookup` - 查询UTXO详情
- `UTXOExists` - 检查UTXO是否存在

### 类别 D：资源查询（2个）
- `ResourceLookup` - 查询资源详情
- `ResourceExists` - 检查资源是否存在

### 类别 E：交易草稿构建（4个）
- `TxAddInput` - 添加交易输入
- `TxAddAssetOutput` - 添加资产输出
- `TxAddResourceOutput` - 添加资源输出
- `TxAddStateOutput` - 添加状态输出

### 类别 G：执行追踪（2个）
- `EmitEvent` - 发出事件
- `LogDebug` - 调试日志

---

## 📚 原语使用示例

### 1. GetBlockHeight - 获取当前区块高度

**用途**：获取当前执行时的区块高度

**WASM示例**：
```rust
// Rust/WASM合约代码
#[no_mangle]
pub extern "C" fn get_current_height() -> u64 {
    // 调用宿主函数
    let height = host_get_block_height();
    height
}
```

**使用场景**：
- 实现时间锁合约（基于区块高度）
- 实现定期支付合约
- 实现区块高度相关的业务逻辑

**最佳实践**：
- ✅ 使用区块高度而非时间戳实现时间锁（更可靠）
- ✅ 缓存区块高度值（在同一执行中多次使用）
- ⚠️ 不要依赖区块高度的精确值（可能有延迟）

---

### 2. GetBlockTimestamp - 获取当前区块时间戳

**用途**：获取当前执行时的区块时间戳（Unix秒）

**WASM示例**：
```rust
// Rust/WASM合约代码
#[no_mangle]
pub extern "C" fn get_current_time() -> u64 {
    let timestamp = host_get_block_timestamp();
    timestamp
}
```

**使用场景**：
- 实现基于时间的业务逻辑
- 实现过期检查
- 实现时间相关的计算

**最佳实践**：
- ✅ 使用时间戳实现业务逻辑（如过期检查）
- ⚠️ 注意时间戳的精度（秒级）
- ⚠️ 不要依赖时间戳的精确值（可能有延迟）

---

### 3. GetBlockHash - 获取指定高度的区块哈希

**用途**：获取指定高度的区块哈希（32字节）

**WASM示例**：
```rust
// Rust/WASM合约代码
#[no_mangle]
pub extern "C" fn get_block_hash(height: u64) -> *const u8 {
    let hash = host_get_block_hash(height);
    hash.as_ptr()
}
```

**使用场景**：
- 验证区块历史
- 实现区块哈希链
- 实现区块相关的验证逻辑

**最佳实践**：
- ✅ 验证区块哈希的有效性
- ⚠️ 注意查询历史区块的性能开销
- ⚠️ 不要频繁查询历史区块

---

### 4. GetChainID - 获取链标识符

**用途**：获取当前链的标识符（字符串）

**WASM示例**：
```rust
// Rust/WASM合约代码
#[no_mangle]
pub extern "C" fn get_chain_id() -> *const u8 {
    let chain_id = host_get_chain_id();
    chain_id.as_ptr()
}
```

**使用场景**：
- 实现跨链验证
- 实现链特定的业务逻辑
- 实现链标识符检查

**最佳实践**：
- ✅ 在跨链场景中使用链ID进行验证
- ✅ 缓存链ID值（在同一执行中多次使用）
- ⚠️ 不要硬编码链ID值

---

### 5. GetCaller - 获取调用者地址

**用途**：获取当前合约调用的调用者地址（20字节）

**WASM示例**：
```rust
// Rust/WASM合约代码
#[no_mangle]
pub extern "C" fn get_caller_address() -> *const u8 {
    let caller = host_get_caller();
    caller.as_ptr()
}
```

**使用场景**：
- 实现权限检查
- 实现身份验证
- 实现调用者相关的业务逻辑

**最佳实践**：
- ✅ 使用调用者地址进行权限检查
- ✅ 验证调用者地址的有效性
- ⚠️ 不要信任调用者地址（可能被伪造，需要通过签名验证）

---

### 6. GetContractAddress - 获取当前合约地址

**用途**：获取当前合约的地址（20字节）

**WASM示例**：
```rust
// Rust/WASM合约代码
#[no_mangle]
pub extern "C" fn get_contract_address() -> *const u8 {
    let address = host_get_contract_address();
    address.as_ptr()
}
```

**使用场景**：
- 实现合约自引用
- 实现合约地址相关的业务逻辑
- 实现合约身份标识

**最佳实践**：
- ✅ 使用合约地址进行自引用
- ✅ 缓存合约地址值（在同一执行中多次使用）
- ⚠️ 不要硬编码合约地址值

---

### 7. GetTransactionID - 获取当前交易ID

**用途**：获取当前交易的ID（32字节）

**WASM示例**：
```rust
// Rust/WASM合约代码
#[no_mangle]
pub extern "C" fn get_transaction_id() -> *const u8 {
    let tx_id = host_get_transaction_id();
    tx_id.as_ptr()
}
```

**使用场景**：
- 实现交易相关的业务逻辑
- 实现交易ID记录
- 实现交易追踪

**最佳实践**：
- ✅ 使用交易ID进行交易追踪
- ✅ 缓存交易ID值（在同一执行中多次使用）
- ⚠️ 注意交易ID的唯一性

---

### 8. UTXOLookup - 查询UTXO详情

**用途**：查询指定UTXO的详情

**WASM示例**：
```rust
// Rust/WASM合约代码
#[no_mangle]
pub extern "C" fn lookup_utxo(tx_id: *const u8, output_index: u32) -> *const UTXO {
    let utxo = host_utxo_lookup(tx_id, output_index);
    utxo.as_ptr()
}
```

**使用场景**：
- 查询UTXO余额
- 查询UTXO详情
- 实现UTXO相关的业务逻辑

**最佳实践**：
- ✅ 使用UTXOLookup查询UTXO详情
- ✅ 缓存查询结果（在同一执行中多次使用）
- ⚠️ 注意查询不存在的UTXO会返回错误

---

### 9. UTXOExists - 检查UTXO是否存在

**用途**：检查指定UTXO是否存在

**WASM示例**：
```rust
// Rust/WASM合约代码
#[no_mangle]
pub extern "C" fn check_utxo_exists(tx_id: *const u8, output_index: u32) -> bool {
    let exists = host_utxo_exists(tx_id, output_index);
    exists
}
```

**使用场景**：
- 快速检查UTXO是否存在
- 实现UTXO存在性验证
- 实现UTXO相关的条件检查

**最佳实践**：
- ✅ 使用UTXOExists进行快速存在性检查
- ✅ 如果只需要检查存在性，使用UTXOExists而非UTXOLookup（性能更好）
- ⚠️ 注意UTXOExists不返回UTXO详情

---

### 10. ResourceLookup - 查询资源详情

**用途**：查询指定资源的详情

**WASM示例**：
```rust
// Rust/WASM合约代码
#[no_mangle]
pub extern "C" fn lookup_resource(resource_hash: *const u8) -> *const Resource {
    let resource = host_resource_lookup(resource_hash);
    resource.as_ptr()
}
```

**使用场景**：
- 查询资源详情
- 实现资源相关的业务逻辑
- 实现资源验证

**最佳实践**：
- ✅ 使用ResourceLookup查询资源详情
- ✅ 缓存查询结果（在同一执行中多次使用）
- ⚠️ 注意查询不存在的资源会返回错误

---

### 11. ResourceExists - 检查资源是否存在

**用途**：检查指定资源是否存在

**WASM示例**：
```rust
// Rust/WASM合约代码
#[no_mangle]
pub extern "C" fn check_resource_exists(resource_hash: *const u8) -> bool {
    let exists = host_resource_exists(resource_hash);
    exists
}
```

**使用场景**：
- 快速检查资源是否存在
- 实现资源存在性验证
- 实现资源相关的条件检查

**最佳实践**：
- ✅ 使用ResourceExists进行快速存在性检查
- ✅ 如果只需要检查存在性，使用ResourceExists而非ResourceLookup（性能更好）
- ⚠️ 注意ResourceExists不返回资源详情

---

### 12. TxAddInput - 添加交易输入

**用途**：向当前交易草稿添加输入

**WASM示例**：
```rust
// Rust/WASM合约代码
#[no_mangle]
pub extern "C" fn add_input(tx_id: *const u8, output_index: u32, is_reference_only: bool) -> u32 {
    let input_index = host_tx_add_input(tx_id, output_index, is_reference_only);
    input_index
}
```

**使用场景**：
- 消费UTXO
- 引用UTXO（只读）
- 构建交易输入

**最佳实践**：
- ✅ 使用TxAddInput添加交易输入
- ✅ 区分消费引用和只读引用（is_reference_only参数）
- ⚠️ 注意输入的有效性（UTXO必须存在）
- ⚠️ 注意输入的权限（必须提供有效的解锁证明）

---

### 13. TxAddAssetOutput - 添加资产输出

**用途**：向当前交易草稿添加资产输出

**WASM示例**：
```rust
// Rust/WASM合约代码
#[no_mangle]
pub extern "C" fn add_asset_output(owner: *const u8, amount: u64, token_id: *const u8) -> u32 {
    let output_index = host_tx_add_asset_output(owner, amount, token_id);
    output_index
}
```

**使用场景**：
- 转账资产
- 创建资产输出
- 构建交易输出

**最佳实践**：
- ✅ 使用TxAddAssetOutput添加资产输出
- ✅ 验证输出参数的有效性（地址、金额等）
- ⚠️ 注意输出的金额必须大于0
- ⚠️ 注意输出的所有者地址必须有效

---

### 14. TxAddResourceOutput - 添加资源输出

**用途**：向当前交易草稿添加资源输出

**WASM示例**：
```rust
// Rust/WASM合约代码
#[no_mangle]
pub extern "C" fn add_resource_output(
    content_hash: *const u8,
    category: *const u8,
    owner: *const u8
) -> u32 {
    let output_index = host_tx_add_resource_output(content_hash, category, owner);
    output_index
}
```

**使用场景**：
- 部署合约
- 部署模型
- 创建资源输出

**最佳实践**：
- ✅ 使用TxAddResourceOutput添加资源输出
- ✅ 验证资源参数的有效性（内容哈希、类别等）
- ⚠️ 注意资源的内容哈希必须有效
- ⚠️ 注意资源的类别必须有效

---

### 15. TxAddStateOutput - 添加状态输出

**用途**：向当前交易草稿添加状态输出

**WASM示例**：
```rust
// Rust/WASM合约代码
#[no_mangle]
pub extern "C" fn add_state_output(
    state_id: *const u8,
    state_version: u64,
    execution_result_hash: *const u8
) -> u32 {
    let output_index = host_tx_add_state_output(
        state_id,
        state_version,
        execution_result_hash
    );
    output_index
}
```

**使用场景**：
- 记录执行结果
- 创建状态输出
- 构建状态证明

**最佳实践**：
- ✅ 使用TxAddStateOutput添加状态输出
- ✅ 验证状态参数的有效性（状态ID、版本等）
- ⚠️ 注意状态的执行结果哈希必须有效
- ⚠️ 注意状态的版本号必须递增

---

### 16. EmitEvent - 发出事件

**用途**：发出执行事件（用于链上事件日志）

**WASM示例**：
```rust
// Rust/WASM合约代码
#[no_mangle]
pub extern "C" fn emit_transfer_event(from: *const u8, to: *const u8, amount: u64) {
    let event_data = format!("Transfer: from={:?}, to={:?}, amount={}", from, to, amount);
    host_emit_event(event_data.as_ptr(), event_data.len());
}
```

**使用场景**：
- 发出业务事件
- 记录重要操作
- 实现事件驱动的业务逻辑

**最佳实践**：
- ✅ 使用EmitEvent发出重要业务事件
- ✅ 事件数据应该结构化（JSON格式）
- ⚠️ 不要发出过多事件（影响性能）
- ⚠️ 事件数据应该简洁明了

---

### 17. LogDebug - 调试日志

**用途**：输出调试日志（仅用于开发调试）

**WASM示例**：
```rust
// Rust/WASM合约代码
#[no_mangle]
pub extern "C" fn debug_log(message: *const u8, len: usize) {
    host_log_debug(message, len);
}
```

**使用场景**：
- 开发调试
- 问题排查
- 执行追踪

**最佳实践**：
- ✅ 使用LogDebug进行开发调试
- ⚠️ 生产环境应该禁用调试日志（影响性能）
- ⚠️ 调试日志不应该包含敏感信息
- ⚠️ 调试日志不应该影响业务逻辑

---

## 🎯 最佳实践指南

### 1. 原语调用性能优化

**缓存查询结果**：
```rust
// ❌ 错误：多次查询相同数据
let height1 = host_get_block_height();
let height2 = host_get_block_height(); // 重复查询

// ✅ 正确：缓存查询结果
let height = host_get_block_height();
// 使用缓存的height值
```

**批量查询优化**：
```rust
// ❌ 错误：逐个查询
for utxo in utxos {
    let exists = host_utxo_exists(utxo.tx_id, utxo.output_index);
}

// ✅ 正确：使用批量查询（如果支持）
let results = host_batch_utxo_exists(utxos);
```

### 2. 错误处理

**总是检查错误**：
```rust
// ❌ 错误：忽略错误
let utxo = host_utxo_lookup(tx_id, output_index);

// ✅ 正确：检查错误
match host_utxo_lookup(tx_id, output_index) {
    Ok(utxo) => {
        // 使用utxo
    }
    Err(e) => {
        // 处理错误
        return Err(e);
    }
}
```

### 3. 安全性

**验证输入参数**：
```rust
// ❌ 错误：不验证输入
let output_index = host_tx_add_asset_output(owner, amount, token_id);

// ✅ 正确：验证输入
if amount == 0 {
    return Err("amount must be greater than 0");
}
if owner.len() != 20 {
    return Err("owner address must be 20 bytes");
}
let output_index = host_tx_add_asset_output(owner, amount, token_id);
```

**权限检查**：
```rust
// ✅ 正确：检查调用者权限
let caller = host_get_caller();
if caller != authorized_address {
    return Err("unauthorized caller");
}
```

### 4. 资源管理

**及时清理资源**：
```rust
// ✅ 正确：及时清理资源
{
    let resource = host_resource_lookup(resource_hash)?;
    // 使用resource
} // resource自动清理
```

### 5. 事件和日志

**结构化事件数据**：
```rust
// ❌ 错误：非结构化事件
host_emit_event("Transfer happened");

// ✅ 正确：结构化事件
let event = json!({
    "type": "Transfer",
    "from": from_address,
    "to": to_address,
    "amount": amount
});
host_emit_event(event.to_string().as_ptr(), event.len());
```

**生产环境禁用调试日志**：
```rust
// ✅ 正确：条件编译
#[cfg(debug_assertions)]
host_log_debug(message.as_ptr(), message.len());
```

---

## 🔧 原语调用追踪工具

### 使用统计功能

**获取原语使用统计**：
```go
// Go代码示例
stats := hostProvider.GetUsageStats()
for primitive, count := range stats.CallCount {
    fmt.Printf("Primitive %s called %d times\n", primitive, count)
}
```

**检查原语完整性**：
```go
// Go代码示例
missing, err := hostProvider.CheckCompleteness()
if err != nil {
    log.Fatal(err)
}
if len(missing) > 0 {
    fmt.Printf("Missing primitives: %v\n", missing)
}
```

---

## 📚 相关文档

- **HostABI接口定义**：[pkg/interfaces/ispc/hostabi.go](../../../pkg/interfaces/ispc/hostabi.go)
- **HostABI实现**：[internal/core/ispc/hostabi/README.md](./README.md)
- **原语完整性测试**：[internal/core/ispc/hostabi/completeness_test.go](./completeness_test.go)

---

## ⚠️ 注意事项

1. **确定性**：所有原语调用都是确定性的，相同输入总是产生相同输出
2. **执行上下文**：所有原语调用都基于当前执行上下文（固定的区块高度视图）
3. **错误处理**：总是检查原语调用的错误返回值
4. **性能**：缓存查询结果，避免重复查询
5. **安全性**：验证输入参数，检查权限

---

## 🎓 学习资源

- **WASM合约开发指南**：[docs/components/core/ispc/capabilities/unified-engines.md](../../../docs/components/core/ispc/capabilities/unified-engines.md)
- **ISPC概念文档**：[docs/components/core/ispc/concept.md](../../../docs/components/core/ispc/concept.md)

