# 历史交易索引设计文档

## 概述

本文档描述了WES区块链系统中历史交易索引的设计和实现，用于支持高效的历史交易查询。

## 设计目标

1. **高效查询**：支持按资源/UTXO快速查询所有相关交易
2. **索引优化**：只存储交易哈希，不重复存储交易数据
3. **增量更新**：支持追加模式，新交易自动追加到历史索引
4. **去重机制**：自动去重，避免重复记录同一交易

## 索引结构

### 1. 资源历史索引

**键格式**: `indices:resource:history:{contentHash}`

**值格式**: 
- 交易哈希列表（变长，每32字节一个交易哈希）
- 最后更新高度（8字节，大端序）

**示例**:
```
键: indices:resource:history:9ab5c433a8167e44569377d9cddaab24b769087fdb95c3daa91e87ce03c50e40
值: [txHash1(32字节)][txHash2(32字节)]...[lastHeight(8字节)]
```

**写入时机**:
- 在 `writeResourceHistoryIndices` 中
- 当交易引用或消费资源UTXO时
- 必须在 `writeUTXOChanges` 之后调用（需要从UTXO中提取资源信息）

### 2. UTXO历史索引

**键格式**: `indices:utxo:history:{txId}:{outputIndex}`

**值格式**:
- 交易哈希列表（变长，每32字节一个交易哈希）
- 最后更新高度（8字节，大端序）

**示例**:
```
键: indices:utxo:history:61ab489ec00b4d667911ee88ce32f6998e34a6d948c6fe20e607d1ad104a839c:0
值: [txHash1(32字节)][txHash2(32字节)]...[lastHeight(8字节)]
```

**写入时机**:
- 在 `writeUTXOHistoryIndices` 中
- 当交易引用UTXO时（包括引用型和消费型）

## 实现细节

### 写入流程

1. **区块处理流程**:
   ```
   WriteBlocks
   ├─ writeBlockData          # 存储区块数据
   ├─ writeTransactionIndices # 更新交易索引
   ├─ writeUTXOChanges        # 处理UTXO变更
   ├─ writeChainState         # 更新链状态
   ├─ writeResourceIndices    # 更新资源索引
   ├─ writeResourceHistoryIndices  # ✅ 新增：写入资源历史索引
   └─ writeUTXOHistoryIndices      # ✅ 新增：写入UTXO历史索引
   ```

2. **资源历史索引写入**:
   - 遍历区块中的所有交易
   - 检查交易输入：如果引用了资源UTXO，从UTXO的cached_output中提取contentHash
   - 追加交易哈希到资源历史索引

3. **UTXO历史索引写入**:
   - 遍历区块中的所有交易
   - 检查交易输入：如果引用了UTXO，追加交易哈希到UTXO历史索引

### 查询流程

1. **资源历史查询**:
   ```
   GetResourceHistory(contentHash, offset, limit)
   ├─ 构建索引键: indices:resource:history:{contentHash}
   ├─ 读取索引值
   ├─ 解析交易哈希列表（排除最后8字节的高度信息）
   ├─ 应用分页（offset, limit）
   └─ 返回交易历史条目列表
   ```

2. **UTXO历史查询**:
   ```
   GetUTXOHistory(outpoint, offset, limit)
   ├─ 构建索引键: indices:utxo:history:{txId}:{outputIndex}
   ├─ 读取索引值
   ├─ 解析交易哈希列表（排除最后8字节的高度信息）
   ├─ 应用分页（offset, limit）
   └─ 返回交易历史条目列表
   ```

## 使用示例

### 查询资源历史交易

```go
// 创建历史查询服务
historyService, err := history.NewService(storage, logger)
if err != nil {
    return err
}

// 查询资源历史交易
contentHash := []byte{...} // 32字节
entries, err := historyService.GetResourceHistory(ctx, contentHash, 0, 10)
if err != nil {
    return err
}

// 遍历交易历史条目
for _, entry := range entries {
    // 通过交易哈希查询完整交易信息
    _, _, tx, err := txQuery.GetTransaction(ctx, entry.TxHash)
    if err != nil {
        continue
    }
    // 处理交易...
}
```

### 查询UTXO历史交易

```go
// 查询UTXO历史交易
outpoint := &transaction.OutPoint{
    TxId:        []byte{...}, // 32字节
    OutputIndex: 0,
}
entries, err := historyService.GetUTXOHistory(ctx, outpoint, 0, 10)
if err != nil {
    return err
}

// 遍历交易历史条目
for _, entry := range entries {
    // 通过交易哈希查询完整交易信息
    _, _, tx, err := txQuery.GetTransaction(ctx, entry.TxHash)
    if err != nil {
        continue
    }
    // 处理交易...
}
```

## 性能考虑

1. **索引大小**: 每个交易哈希32字节，加上8字节高度信息，索引大小 = (交易数量 × 32) + 8 字节
2. **查询性能**: O(1) 索引查找 + O(n) 列表解析，n为交易数量
3. **写入性能**: O(1) 追加操作，去重检查O(n)，n为现有交易数量
4. **存储优化**: 只存储交易哈希，不重复存储交易数据，交易数据从区块中提取

## 未来扩展

1. **交易类型过滤**: 支持按交易类型（引用/消费/升级）过滤
2. **时间范围过滤**: 支持按时间范围过滤历史交易
3. **排序优化**: 支持按区块高度或时间戳排序
4. **批量查询**: 支持批量查询多个资源的历史交易
5. **压缩优化**: 对于历史数据，可以考虑压缩存储

## 注意事项

1. **索引写入顺序**: 历史索引写入必须在 `writeUTXOChanges` 之后，因为需要从UTXO中提取资源信息
2. **去重机制**: `appendToHistoryIndex` 会自动去重，避免重复记录同一交易
3. **分页支持**: 查询方法支持 `offset` 和 `limit` 参数，用于分页查询
4. **错误处理**: 索引不存在时返回空列表，不返回错误

