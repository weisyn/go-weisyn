# adapter 包代码问题报告

## 📋 概述

本文档记录通过测试发现的 `internal/core/ispc/hostabi/adapter` 包中的代码问题和潜在缺陷。

## 🐛 发现的问题

### 1. 错误处理的歧义性问题

#### 1.1 `get_block_height` 错误处理歧义

**问题描述**：
- 当 `hostABI.GetBlockHeight(ctx)` 返回错误时，函数返回 `0`
- 但 `0` 可能是有效的区块高度（区块0存在）
- 调用者无法区分"错误"和"区块0"

**代码位置**：
```go
// wasm_adapter.go:184-190
"get_block_height": func() uint64 {
    height, err := hostABI.GetBlockHeight(ctx)
    if err != nil {
        return 0  // ⚠️ 可能与区块0混淆
    }
    return height
}
```

**影响**：
- 调用者无法区分错误和有效值
- 可能导致错误的业务逻辑判断

**建议**：
- 考虑使用错误码（如 `ErrInternalError`）而不是返回 `0`
- 或者返回 `(height, error)` 而不是只返回 `height`
- 或者使用特殊值（如 `math.MaxUint64`）表示错误

#### 1.2 `get_block_timestamp` 错误处理歧义

**问题描述**：
- 当 `hostABI.GetBlockTimestamp(ctx)` 返回错误时，函数返回 `0`
- 但 `0` 是 Unix 纪元（1970-01-01 00:00:00 UTC）的有效时间戳
- 调用者无法区分"错误"和"Unix纪元"

**代码位置**：
```go
// wasm_adapter.go:192-198
"get_block_timestamp": func() uint64 {
    timestamp, err := hostABI.GetBlockTimestamp(ctx)
    if err != nil {
        return 0  // ⚠️ 可能与Unix纪元混淆
    }
    return timestamp
}
```

**影响**：
- 调用者无法区分错误和有效值
- 可能导致时间计算错误

**建议**：
- 考虑使用错误码（如 `ErrInternalError`）而不是返回 `0`
- 或者返回 `(timestamp, error)` 而不是只返回 `timestamp`
- 或者使用特殊值（如 `math.MaxUint64`）表示错误

#### 1.3 `get_caller` 错误处理歧义

**问题描述**：
- 多个错误路径都返回 `0`：
  - `nil ExecutionContext` → 返回 `0`
  - `nil memory` → 返回 `0`
  - 内存越界 → 返回 `0`
  - 地址长度错误 → 返回 `0`
  - 写入内存失败 → 返回 `0`
- 调用者无法区分不同的错误类型

**代码位置**：
```go
// wasm_adapter.go:203-245
"get_caller": func(ctx context.Context, m api.Module, addrPtr uint32) uint32 {
    // 多个错误路径都返回 0
    if currentExecCtx == nil {
        return 0  // ⚠️ 无法区分错误类型
    }
    if memory == nil {
        return 0  // ⚠️ 无法区分错误类型
    }
    // ...
}
```

**影响**：
- 调用者无法区分不同的错误类型
- 调试困难，无法定位具体问题

**建议**：
- 使用不同的错误码来区分错误类型：
  - `ErrContextNotFound` (5003) - ExecutionContext未找到
  - `ErrMemoryAccessFailed` (5004) - 内存访问失败
  - `ErrInvalidParameter` (1001) - 参数无效
  - `ErrInvalidAddress` (1010) - 地址无效

### 2. 错误处理不一致

**问题描述**：
- 不同宿主函数的错误处理方式不一致：
  - `get_block_height`: 返回 `0`（可能歧义）
  - `get_block_timestamp`: 返回 `0`（可能歧义）
  - `get_chain_id`: 返回错误码（更明确）
  - `get_caller`: 返回 `0`（可能歧义）

**影响**：
- API 不一致，增加使用复杂度
- 调用者需要记住不同函数的错误处理方式

**建议**：
- 统一错误处理方式，所有函数都使用错误码
- 或者统一使用 `(value, error)` 返回方式

### 3. nil facade 处理

**问题描述**：
- `NewSDKAdapter(nil)` 可以创建适配器，但 `facade` 为 `nil`
- 调用 `BuildTransaction` 时，如果 `facade` 为 `nil`，会调用 `nil.Compose()`，导致 panic

**代码位置**：
```go
// adapter.go:88-111
func (a *SDKAdapter) BuildTransaction(ctx context.Context, draftJSON []byte) (*types.DraftTx, error) {
    // ...
    draft, err := a.facade.Compose(ctx, intents)  // ⚠️ 如果 facade 为 nil，会 panic
    // ...
}
```

**影响**：
- 可能导致运行时 panic
- 错误应该在编译时或初始化时发现，而不是运行时

**建议**：
- 在 `BuildTransaction` 中添加 `nil` 检查：
  ```go
  if a.facade == nil {
      return nil, fmt.Errorf("facade未设置")
  }
  ```
- 或者在 `NewSDKAdapter` 中禁止 `nil` facade

## ✅ 已验证的正确行为

### 1. 空 draft 验证
- ✅ `convertToTxIntents` 正确验证空 draft
- ✅ 返回明确的错误信息："SDK draft必须包含至少一个输出或意图"

### 2. nil draft 验证
- ✅ `convertToTxIntents` 正确验证 nil draft
- ✅ 返回明确的错误信息："SDK draft不能为空"

### 3. 占位符代码
- ✅ `wasm_adapter.go` 中的占位符代码有明确的文档说明
- ✅ 占位符有明确的替换时机（同步/异步模式）
- ✅ 占位符有验证要求（如果Proof为空，交易验证将失败）

## 📊 问题统计

- **严重问题**：3个（错误处理歧义性）
- **中等问题**：1个（错误处理不一致）
- **潜在问题**：1个（nil facade 处理）
- **已验证正确**：3个

## 🔧 修复优先级

1. **高优先级**：修复 `get_caller` 的错误处理，使用错误码区分不同错误类型 ✅ **已修复**
2. **中优先级**：统一错误处理方式，所有函数都使用错误码 ✅ **已修复**
3. **低优先级**：考虑修复 `get_block_height` 和 `get_block_timestamp` 的歧义性（如果确实存在问题） ✅ **已修复**

## ✅ 修复状态

### 1. `get_block_height` 错误处理 ✅ **已修复**
- **修复方案**：使用 `math.MaxUint64` 表示错误，避免与区块0混淆
- **代码位置**：`wasm_adapter.go:184-194`
- **修复时间**：2024-12-XX

### 2. `get_block_timestamp` 错误处理 ✅ **已修复**
- **修复方案**：使用 `math.MaxUint64` 表示错误，避免与Unix纪元混淆
- **代码位置**：`wasm_adapter.go:196-206`
- **修复时间**：2024-12-XX

### 3. `get_caller` 错误处理 ✅ **已修复**
- **修复方案**：使用不同错误码区分错误类型：
  - `ErrContextNotFound` (5003) - ExecutionContext未找到
  - `ErrMemoryAccessFailed` (5004) - 内存访问失败
  - `ErrInvalidParameter` (1001) - 参数无效（内存越界）
  - `ErrInvalidAddress` (1010) - 地址无效（长度错误）
- **代码位置**：`wasm_adapter.go:208-260`
- **修复时间**：2024-12-XX

### 4. nil facade 处理 ✅ **已修复**
- **修复方案**：在 `BuildTransaction` 中添加 `nil` 检查
- **代码位置**：`adapter.go:92-95`
- **修复时间**：2024-12-XX

## 📝 备注

- 这些问题是通过测试发现的，符合"测试的目的是为了发现代码问题"的原则
- 所有问题已修复，测试已更新以验证修复
- 修复后的代码提高了错误处理的明确性和一致性

