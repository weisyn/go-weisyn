# 数据存储应用API使用指南

## 📖 概述

本文档详细介绍数据存储应用的API接口，包括使用方法、参数说明、示例代码和最佳实践。

## 🚀 快速开始

### 初始化客户端

```go
import (
    "github.com/weisyn/v1/pkg/interfaces/blockchain"
    // 其他必要的导入
)

// 创建存储客户端
client := NewStorageClient(blockchainInstance, "storage_contract_address")
```

## 📋 API接口列表

### 1. 数据存储接口

#### StoreData - 存储数据

**功能**: 将数据安全存储到区块链上

**方法签名**:
```go
func (client *StorageClient) StoreData(request StorageRequest) (*StorageResult, error)
```

**参数说明**:
```go
type StorageRequest struct {
    Title       string                 `json:"title"`       // 数据标题
    Content     string                 `json:"content"`     // 数据内容
    DataType    string                 `json:"data_type"`   // 数据类型
    Tags        []string               `json:"tags"`        // 标签列表
    Metadata    map[string]interface{} `json:"metadata"`    // 元数据
    Encrypt     bool                   `json:"encrypt"`     // 是否加密
    Owner       string                 `json:"owner"`       // 所有者地址
}
```

**返回结果**:
```go
type StorageResult struct {
    RecordID  string    `json:"record_id"`  // 记录ID
    Hash      string    `json:"hash"`       // 数据哈希
    TxHash    string    `json:"tx_hash"`    // 交易哈希
    Success   bool      `json:"success"`    // 是否成功
    Message   string    `json:"message"`    // 结果消息
    Timestamp time.Time `json:"timestamp"`  // 时间戳
}
```

**使用示例**:
```go
request := StorageRequest{
    Title:    "项目计划文档",
    Content:  "这是项目计划的详细内容...",
    DataType: "document",
    Tags:     []string{"项目", "计划"},
    Metadata: map[string]interface{}{
        "version": "1.0",
        "author":  "Alice",
    },
    Encrypt: true,
    Owner:   "alice_address",
}

result, err := client.StoreData(request)
if err != nil {
    log.Printf("存储失败: %v", err)
    return
}

fmt.Printf("存储成功! 记录ID: %s\n", result.RecordID)
```

#### RetrieveData - 检索数据

**功能**: 根据记录ID检索特定数据

**方法签名**:
```go
func (client *StorageClient) RetrieveData(recordID string, requester string) (*DataRecord, error)
```

**参数说明**:
- `recordID`: 数据记录的唯一标识
- `requester`: 请求者的地址

**返回结果**:
```go
type DataRecord struct {
    ID          string                 `json:"id"`
    Title       string                 `json:"title"`
    Content     string                 `json:"content"`
    DataType    string                 `json:"data_type"`
    Owner       string                 `json:"owner"`
    Tags        []string               `json:"tags"`
    Metadata    map[string]interface{} `json:"metadata"`
    Hash        string                 `json:"hash"`
    Timestamp   time.Time              `json:"timestamp"`
    Version     int                    `json:"version"`
    IsEncrypted bool                   `json:"is_encrypted"`
}
```

**使用示例**:
```go
record, err := client.RetrieveData("record_id_123", "alice_address")
if err != nil {
    log.Printf("检索失败: %v", err)
    return
}

fmt.Printf("标题: %s\n", record.Title)
fmt.Printf("内容: %s\n", record.Content)
```

### 2. 数据查询接口

#### QueryData - 条件查询

**功能**: 根据多种条件查询数据

**方法签名**:
```go
func (client *StorageClient) QueryData(request QueryRequest) ([]DataRecord, error)
```

**参数说明**:
```go
type QueryRequest struct {
    ID       string            `json:"id"`         // 按ID查询
    Title    string            `json:"title"`      // 按标题查询
    Tags     []string          `json:"tags"`       // 按标签查询
    Owner    string            `json:"owner"`      // 按所有者查询
    DataType string            `json:"data_type"`  // 按类型查询
    Metadata map[string]string `json:"metadata"`   // 按元数据查询
    TimeFrom time.Time         `json:"time_from"`  // 时间范围开始
    TimeTo   time.Time         `json:"time_to"`    // 时间范围结束
    Limit    int               `json:"limit"`      // 结果数量限制
}
```

**使用示例**:
```go
// 简单查询 - 按标签
queryReq := QueryRequest{
    Tags:  []string{"项目"},
    Limit: 10,
}

records, err := client.QueryData(queryReq)
if err != nil {
    log.Printf("查询失败: %v", err)
    return
}

fmt.Printf("找到 %d 个匹配记录\n", len(records))

// 复合查询 - 多条件
complexQuery := QueryRequest{
    Owner:    "alice_address",
    DataType: "document",
    Tags:     []string{"项目", "计划"},
    TimeFrom: time.Now().AddDate(0, 0, -7), // 最近7天
    Limit:    20,
}

records, err = client.QueryData(complexQuery)
```

### 3. 数据管理接口

#### UpdateData - 更新数据

**功能**: 更新已有数据，创建新版本

**方法签名**:
```go
func (client *StorageClient) UpdateData(recordID string, updateRequest StorageRequest) (*StorageResult, error)
```

**使用示例**:
```go
updateReq := StorageRequest{
    Title:   "项目计划文档 v2.0",
    Content: "更新后的项目计划内容...",
    Tags:    []string{"项目", "计划", "更新"},
    Owner:   "alice_address",
}

result, err := client.UpdateData("record_id_123", updateReq)
if err != nil {
    log.Printf("更新失败: %v", err)
    return
}

fmt.Printf("更新成功! 新版本: %s\n", result.Message)
```

#### DeleteData - 删除数据

**功能**: 标记删除数据（保留历史记录）

**方法签名**:
```go
func (client *StorageClient) DeleteData(recordID string, requester string) (*StorageResult, error)
```

**使用示例**:
```go
result, err := client.DeleteData("record_id_123", "alice_address")
if err != nil {
    log.Printf("删除失败: %v", err)
    return
}

fmt.Printf("删除成功: %s\n", result.Message)
```

#### GetDataHistory - 获取版本历史

**功能**: 获取数据的所有版本历史

**方法签名**:
```go
func (client *StorageClient) GetDataHistory(recordID string, requester string) ([]DataRecord, error)
```

**使用示例**:
```go
history, err := client.GetDataHistory("record_id_123", "alice_address")
if err != nil {
    log.Printf("获取历史失败: %v", err)
    return
}

fmt.Printf("数据有 %d 个版本\n", len(history))
for i, record := range history {
    fmt.Printf("版本 %d: %s (创建于 %s)\n", 
        record.Version, record.Title, record.Timestamp.Format("2006-01-02 15:04:05"))
}
```

## 🔧 数据管理器API

### DataManager - 数据处理

#### ProcessContent - 内容处理

**功能**: 处理内容（加密、压缩等）

```go
dm := NewDataManager()

// 加密处理
processedContent, err := dm.ProcessContent(originalContent, true)
if err != nil {
    log.Printf("处理失败: %v", err)
    return
}

// 解密处理
decryptedContent, err := dm.DecryptContent(processedContent, "user_address")
```

#### ValidateIntegrity - 完整性验证

**功能**: 验证数据完整性

```go
isValid, err := dm.ValidateIntegrity(content, expectedHash)
if err != nil {
    log.Printf("验证失败: %v", err)
    return
}

if isValid {
    fmt.Println("数据完整性验证通过")
} else {
    fmt.Println("数据可能已被篡改")
}
```

#### ChunkData - 数据分片

**功能**: 将大数据分成小片

```go
chunks, err := dm.ChunkData(largeContent, 1024*1024) // 1MB分片
if err != nil {
    log.Printf("分片失败: %v", err)
    return
}

fmt.Printf("数据分为 %d 个片段\n", len(chunks))

// 重组数据
reassembled, err := dm.ReassembleChunks(chunks)
```

## 🔍 查询引擎API

### QueryEngine - 高级查询

#### SearchIndex - 索引搜索

**功能**: 在索引中快速搜索

```go
qe := NewQueryEngine()

// 添加数据到索引
err := qe.AddToIndex(dataRecord)
if err != nil {
    log.Printf("添加索引失败: %v", err)
    return
}

// 搜索
queryReq := QueryRequest{
    Title: "项目",
    Tags:  []string{"计划"},
}

results, err := qe.SearchIndex(queryReq)
if err != nil {
    log.Printf("搜索失败: %v", err)
    return
}

fmt.Printf("搜索到 %d 个结果\n", len(results))
```

#### GetIndexStats - 索引统计

**功能**: 获取索引统计信息

```go
stats := qe.GetIndexStats()
fmt.Printf("总记录数: %d\n", stats.TotalRecords)
fmt.Printf("标题索引项: %d\n", stats.TitleEntries)
fmt.Printf("标签索引项: %d\n", stats.TagEntries)
```

## 🛡️ 完整性检查API

### IntegrityChecker - 数据完整性

#### VerifyDataIntegrity - 验证单个数据

```go
ic := NewIntegrityChecker()

result := ic.VerifyDataIntegrity(dataRecord)
if result.IsValid {
    fmt.Println("数据完整性正常")
} else {
    fmt.Printf("完整性验证失败: %s\n", result.ErrorMessage)
}
```

#### BatchVerifyIntegrity - 批量验证

```go
batchResult := ic.BatchVerifyIntegrity(dataRecords)
fmt.Printf("验证 %d 个记录，有效 %d 个，无效 %d 个\n",
    batchResult.TotalChecked, batchResult.ValidCount, batchResult.InvalidCount)
```

#### GenerateIntegrityReport - 生成报告

```go
report := ic.GenerateIntegrityReport(dataRecords)
fmt.Printf("整体质量评分: %.2f%%\n", report["quality_scores"].(map[string]interface{})["overall_score"])
```

## ⚡ 性能优化

### 批量操作

```go
// 批量存储
var requests []StorageRequest
for _, data := range dataList {
    requests = append(requests, StorageRequest{
        Title:   data.Title,
        Content: data.Content,
        // ... 其他字段
    })
}

// 并发处理
var wg sync.WaitGroup
for _, req := range requests {
    wg.Add(1)
    go func(request StorageRequest) {
        defer wg.Done()
        client.StoreData(request)
    }(req)
}
wg.Wait()
```

### 缓存机制

```go
// 缓存热点数据
cache := make(map[string]*DataRecord)

// 检查缓存
if cachedRecord, exists := cache[recordID]; exists {
    return cachedRecord, nil
}

// 从区块链获取并缓存
record, err := client.RetrieveData(recordID, requester)
if err == nil {
    cache[recordID] = record
}
```

## 🔒 安全最佳实践

### 权限验证

```go
// 检查操作权限
func checkPermission(requester, owner string, operation string) bool {
    if requester == owner {
        return true // 所有者拥有所有权限
    }
    
    // 检查其他权限...
    return false
}

// 在操作前验证
if !checkPermission(requester, record.Owner, "read") {
    return nil, fmt.Errorf("权限不足")
}
```

### 敏感数据处理

```go
// 敏感数据必须加密
if containsSensitiveInfo(content) {
    request.Encrypt = true
}

// 记录访问日志
logAccess(requester, recordID, operation, time.Now())
```

## 📊 监控和日志

### 性能监控

```go
import "time"

func monitorPerformance(operation string, fn func() error) error {
    start := time.Now()
    err := fn()
    duration := time.Since(start)
    
    log.Printf("操作 %s 耗时: %v", operation, duration)
    return err
}

// 使用示例
err := monitorPerformance("store_data", func() error {
    _, err := client.StoreData(request)
    return err
})
```

### 错误处理

```go
func handleStorageError(err error) {
    switch {
    case strings.Contains(err.Error(), "permission"):
        log.Printf("权限错误: %v", err)
        // 处理权限错误
    case strings.Contains(err.Error(), "network"):
        log.Printf("网络错误: %v", err)
        // 处理网络错误
    default:
        log.Printf("未知错误: %v", err)
        // 处理其他错误
    }
}
```

## ❓ 常见问题和解决方案

### Q: 如何处理大文件存储？

```go
// 对于大文件，使用分片存储
dm := NewDataManager()
chunks, err := dm.ChunkData(largeContent, 1024*1024) // 1MB分片

var chunkIDs []string
for i, chunk := range chunks {
    chunkReq := StorageRequest{
        Title:   fmt.Sprintf("chunk_%d_of_%s", i, originalTitle),
        Content: chunk,
        Tags:    []string{"chunk", "large_file"},
        Owner:   owner,
    }
    
    result, err := client.StoreData(chunkReq)
    if err != nil {
        return err
    }
    chunkIDs = append(chunkIDs, result.RecordID)
}

// 存储分片索引
indexReq := StorageRequest{
    Title:   originalTitle,
    Content: strings.Join(chunkIDs, ","),
    Tags:    []string{"index", "large_file"},
    Metadata: map[string]interface{}{
        "total_chunks": len(chunks),
        "chunk_size":   1024*1024,
    },
    Owner: owner,
}
```

### Q: 如何实现数据版本比较？

```go
func compareVersions(recordID string, version1, version2 int) (map[string]interface{}, error) {
    history, err := client.GetDataHistory(recordID, requester)
    if err != nil {
        return nil, err
    }
    
    var v1, v2 *DataRecord
    for _, record := range history {
        if record.Version == version1 {
            v1 = &record
        }
        if record.Version == version2 {
            v2 = &record
        }
    }
    
    comparison := map[string]interface{}{
        "title_changed":    v1.Title != v2.Title,
        "content_changed":  v1.Content != v2.Content,
        "tags_changed":     !equalStringSlices(v1.Tags, v2.Tags),
        "size_change":      len(v2.Content) - len(v1.Content),
    }
    
    return comparison, nil
}
```

---

🎯 通过本API指南，您应该能够熟练使用数据存储应用的所有功能。如果遇到问题，请参考[故障排除指南](../token-transfer/docs/TROUBLESHOOTING.md)或联系技术支持。
