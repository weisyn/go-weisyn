# 历史交易查询模块

## 概述

历史交易查询模块 (`internal/core/persistence/query/history`) 提供按资源/UTXO查询所有相关交易的能力。

## 功能

### 1. 资源历史查询

- **GetResourceHistory**: 查询资源的所有历史交易（部署、引用、升级）
- **GetResourceHistoryTotal**: 获取资源的历史交易总数
- **GetLastUpdateHeight**: 获取资源历史索引最后更新的区块高度

### 2. UTXO历史查询

- **GetUTXOHistory**: 查询UTXO的所有历史交易（引用、消费）

## 索引设计

### 资源历史索引

- **键格式**: `indices:resource:history:{contentHash}`
- **值格式**: 交易哈希列表（变长，每32字节一个交易哈希）+ 最后更新高度（8字节）
- **写入时机**: 在 `writeResourceHistoryIndices` 中，当交易引用或消费资源UTXO时

### UTXO历史索引

- **键格式**: `indices:utxo:history:{txId}:{outputIndex}`
- **值格式**: 交易哈希列表（变长，每32字节一个交易哈希）+ 最后更新高度（8字节）
- **写入时机**: 在 `writeUTXOHistoryIndices` 中，当交易引用UTXO时

## 使用示例

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
    tx, err := txQuery.GetTransaction(ctx, entry.TxHash)
    // ...
}
```

## 注意事项

1. **索引写入顺序**: 历史索引写入必须在 `writeUTXOChanges` 之后，因为需要从UTXO中提取资源信息
2. **去重机制**: `appendToHistoryIndex` 会自动去重，避免重复记录同一交易
3. **分页支持**: 查询方法支持 `offset` 和 `limit` 参数，用于分页查询
4. **性能考虑**: 索引只存储交易哈希，不重复存储交易数据，交易数据从区块中提取

## 未来扩展

1. **交易类型过滤**: 支持按交易类型（引用/消费/升级）过滤
2. **时间范围过滤**: 支持按时间范围过滤历史交易
3. **排序优化**: 支持按区块高度或时间戳排序
4. **批量查询**: 支持批量查询多个资源的历史交易

