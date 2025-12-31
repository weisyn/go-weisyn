package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

/*
🎯 数据存储客户端应用

这是一个完整的数据存储应用示例，展示如何：
1. 在区块链上安全存储数据
2. 建立高效的数据索引系统
3. 实现灵活的数据查询机制
4. 确保数据完整性和可追溯性

💡 学习重点：
- 去中心化存储的原理和实现
- 数据加密和安全存储
- 索引构建和查询优化
- 版本控制和数据审计
*/

// Transaction 简化的交易结构
type Transaction struct {
	From              string `json:"from"`
	To                string `json:"to"`
	Amount            uint64 `json:"amount"`
	ExecutionFeeLimit uint64 `json:"execution_fee_limit"`
	ExecutionFeePrice uint64 `json:"execution_fee_price"`
	Data              string `json:"data"`
	Timestamp         int64  `json:"timestamp"`
	ContractMethod    string `json:"contract_method"`
}

// StorageClient 数据存储客户端
type StorageClient struct {
	storageContract string       // 存储合约地址
	dataManager     *DataManager // 数据管理器
	queryEngine     *QueryEngine // 查询引擎
}

// DataRecord 数据记录结构
type DataRecord struct {
	ID          string                 `json:"id"`           // 数据唯一标识
	Title       string                 `json:"title"`        // 数据标题
	Content     string                 `json:"content"`      // 数据内容
	DataType    string                 `json:"data_type"`    // 数据类型
	Owner       string                 `json:"owner"`        // 数据所有者
	Tags        []string               `json:"tags"`         // 标签列表
	Metadata    map[string]interface{} `json:"metadata"`     // 元数据
	Hash        string                 `json:"hash"`         // 内容哈希
	Timestamp   time.Time              `json:"timestamp"`    // 创建时间
	Version     int                    `json:"version"`      // 版本号
	IsEncrypted bool                   `json:"is_encrypted"` // 是否加密
}

// StorageRequest 存储请求
type StorageRequest struct {
	Title    string                 `json:"title"`     // 标题
	Content  string                 `json:"content"`   // 内容
	DataType string                 `json:"data_type"` // 数据类型
	Tags     []string               `json:"tags"`      // 标签
	Metadata map[string]interface{} `json:"metadata"`  // 元数据
	Encrypt  bool                   `json:"encrypt"`   // 是否加密
	Owner    string                 `json:"owner"`     // 所有者
}

// StorageResult 存储结果
type StorageResult struct {
	RecordID  string    `json:"record_id"` // 记录ID
	Hash      string    `json:"hash"`      // 数据哈希
	TxHash    string    `json:"tx_hash"`   // 交易哈希
	Success   bool      `json:"success"`   // 是否成功
	Message   string    `json:"message"`   // 结果消息
	Timestamp time.Time `json:"timestamp"` // 时间戳
}

// QueryRequest 查询请求
type QueryRequest struct {
	ID       string            `json:"id"`        // 按ID查询
	Title    string            `json:"title"`     // 按标题查询
	Tags     []string          `json:"tags"`      // 按标签查询
	Owner    string            `json:"owner"`     // 按所有者查询
	DataType string            `json:"data_type"` // 按类型查询
	Metadata map[string]string `json:"metadata"`  // 按元数据查询
	TimeFrom time.Time         `json:"time_from"` // 时间范围开始
	TimeTo   time.Time         `json:"time_to"`   // 时间范围结束
	Limit    int               `json:"limit"`     // 结果数量限制
}

// NewStorageClient 创建新的存储客户端
func NewStorageClient(storageContract string) *StorageClient {
	return &StorageClient{
		storageContract: storageContract,
		dataManager:     NewDataManager(),
		queryEngine:     NewQueryEngine(),
	}
}

// StoreData 存储数据到区块链
// 🎯 核心功能：将数据安全存储到区块链并建立索引
func (client *StorageClient) StoreData(request StorageRequest) (*StorageResult, error) {
	// 📋 步骤1：验证存储请求
	if err := client.validateStorageRequest(request); err != nil {
		return &StorageResult{
			Success:   false,
			Message:   fmt.Sprintf("请求验证失败: %v", err),
			Timestamp: time.Now(),
		}, err
	}

	// 📋 步骤2：生成唯一记录ID
	recordID := client.generateRecordID(request)

	// 📋 步骤3：处理数据内容（加密/压缩）
	processedContent, err := client.dataManager.ProcessContent(request.Content, request.Encrypt)
	if err != nil {
		return &StorageResult{
			Success:   false,
			Message:   fmt.Sprintf("内容处理失败: %v", err),
			Timestamp: time.Now(),
		}, err
	}

	// 📋 步骤4：计算数据哈希
	contentHash := client.calculateContentHash(processedContent)

	// 📋 步骤5：构建数据记录
	record := DataRecord{
		ID:          recordID,
		Title:       request.Title,
		Content:     processedContent,
		DataType:    request.DataType,
		Owner:       request.Owner,
		Tags:        request.Tags,
		Metadata:    request.Metadata,
		Hash:        contentHash,
		Timestamp:   time.Now(),
		Version:     1,
		IsEncrypted: request.Encrypt,
	}

	// 📋 步骤6：构建存储交易
	params := map[string]interface{}{
		"record": record,
	}

	transaction, err := client.buildStorageTransaction(request.Owner, params)
	if err != nil {
		return &StorageResult{
			Success:   false,
			Message:   fmt.Sprintf("构建交易失败: %v", err),
			Timestamp: time.Now(),
		}, err
	}

	// 📋 步骤7：提交交易到区块链
	// 在实际应用中，这里会调用区块链接口
	// txHash, err := client.blockchain.SubmitTransaction(transaction)
	// 这里使用模拟实现
	txHash := client.simulateTransactionSubmission(transaction)
	if txHash == "" {
		return &StorageResult{
			Success:   false,
			Message:   "提交交易失败: 模拟错误",
			Timestamp: time.Now(),
		}, fmt.Errorf("transaction submission failed")
	}

	// 📋 步骤8：建立本地索引（可选）
	if err := client.queryEngine.AddToIndex(record); err != nil {
		log.Printf("警告: 索引建立失败: %v", err)
	}

	// 💡 生活化理解：
	// 存储数据就像把文件放入保险箱
	// - 数据加密 = 文件密码保护
	// - 哈希值 = 文件指纹，确保完整性
	// - 区块链 = 不可篡改的保险箱
	// - 索引 = 档案管理系统，便于查找

	// ✅ 返回存储结果
	return &StorageResult{
		RecordID:  recordID,
		Hash:      contentHash,
		TxHash:    txHash,
		Success:   true,
		Message:   "数据存储成功",
		Timestamp: time.Now(),
	}, nil
}

// RetrieveData 根据ID检索数据
// 🎯 功能：从区块链检索指定的数据记录
func (client *StorageClient) RetrieveData(recordID string, requester string) (*DataRecord, error) {
	// 构建查询参数
	params := map[string]interface{}{
		"record_id": recordID,
		"requester": requester,
	}

	// 调用存储合约的检索方法
	// 在实际应用中，这里会调用区块链接口
	// result, err := client.blockchain.CallContract(client.storageContract, "RetrieveData", params)
	// 这里使用模拟实现
	result, err := client.simulateContractCall("RetrieveData", params)
	if err != nil {
		return nil, fmt.Errorf("合约调用失败: %v", err)
	}

	// 解析返回的数据记录
	var record DataRecord
	if err := json.Unmarshal(result, &record); err != nil {
		return nil, fmt.Errorf("解析数据失败: %v", err)
	}

	// 如果数据是加密的，需要解密
	if record.IsEncrypted {
		decryptedContent, err := client.dataManager.DecryptContent(record.Content, requester)
		if err != nil {
			return nil, fmt.Errorf("解密失败: %v", err)
		}
		record.Content = decryptedContent
	}

	return &record, nil
}

// QueryData 根据条件查询数据
// 🎯 功能：支持多维度的数据查询和筛选
func (client *StorageClient) QueryData(request QueryRequest) ([]DataRecord, error) {
	// 📋 步骤1：本地索引查询（快速筛选）
	candidateIDs, err := client.queryEngine.SearchIndex(request)
	if err != nil {
		log.Printf("本地索引查询失败，降级为链上查询: %v", err)
	}

	// 📋 步骤2：构建链上查询参数
	params := map[string]interface{}{
		"query":         request,
		"candidate_ids": candidateIDs,
	}

	// 📋 步骤3：调用合约查询方法
	// 在实际应用中，这里会调用区块链接口
	// result, err := client.blockchain.CallContract(client.storageContract, "QueryData", params)
	// 这里使用模拟实现
	result, err := client.simulateContractCall("QueryData", params)
	if err != nil {
		return nil, fmt.Errorf("查询失败: %v", err)
	}

	// 📋 步骤4：解析查询结果
	var records []DataRecord
	if err := json.Unmarshal(result, &records); err != nil {
		return nil, fmt.Errorf("解析结果失败: %v", err)
	}

	// 📋 步骤5：后处理（解密、排序、过滤）
	processedRecords, err := client.postProcessResults(records, request)
	if err != nil {
		return nil, fmt.Errorf("结果处理失败: %v", err)
	}

	return processedRecords, nil
}

// UpdateData 更新已有数据
// 🎯 功能：创建数据的新版本，保持历史记录
func (client *StorageClient) UpdateData(recordID string, updateRequest StorageRequest) (*StorageResult, error) {
	// 获取原始记录
	originalRecord, err := client.RetrieveData(recordID, updateRequest.Owner)
	if err != nil {
		return &StorageResult{
			Success:   false,
			Message:   fmt.Sprintf("获取原始记录失败: %v", err),
			Timestamp: time.Now(),
		}, err
	}

	// 检查权限
	if originalRecord.Owner != updateRequest.Owner {
		return &StorageResult{
			Success:   false,
			Message:   "无权限更新此记录",
			Timestamp: time.Now(),
		}, fmt.Errorf("permission denied")
	}

	// 处理更新内容
	processedContent, err := client.dataManager.ProcessContent(updateRequest.Content, updateRequest.Encrypt)
	if err != nil {
		return &StorageResult{
			Success:   false,
			Message:   fmt.Sprintf("内容处理失败: %v", err),
			Timestamp: time.Now(),
		}, err
	}

	// 创建新版本记录
	updatedRecord := *originalRecord
	updatedRecord.Content = processedContent
	updatedRecord.Title = updateRequest.Title
	updatedRecord.Tags = updateRequest.Tags
	updatedRecord.Metadata = updateRequest.Metadata
	updatedRecord.Hash = client.calculateContentHash(processedContent)
	updatedRecord.Timestamp = time.Now()
	updatedRecord.Version = originalRecord.Version + 1
	updatedRecord.IsEncrypted = updateRequest.Encrypt

	// 提交更新交易
	params := map[string]interface{}{
		"record_id":      recordID,
		"updated_record": updatedRecord,
	}

	transaction, err := client.buildStorageTransaction(updateRequest.Owner, params)
	if err != nil {
		return &StorageResult{
			Success:   false,
			Message:   fmt.Sprintf("构建更新交易失败: %v", err),
			Timestamp: time.Now(),
		}, err
	}

	// 在实际应用中，这里会调用区块链接口
	// txHash, err := client.blockchain.SubmitTransaction(transaction)
	// 这里使用模拟实现
	txHash := client.simulateTransactionSubmission(transaction)
	if txHash == "" {
		return &StorageResult{
			Success:   false,
			Message:   "提交更新失败: 模拟错误",
			Timestamp: time.Now(),
		}, fmt.Errorf("update submission failed")
	}

	// 更新索引
	if err := client.queryEngine.UpdateIndex(updatedRecord); err != nil {
		log.Printf("警告: 索引更新失败: %v", err)
	}

	return &StorageResult{
		RecordID:  recordID,
		Hash:      updatedRecord.Hash,
		TxHash:    txHash,
		Success:   true,
		Message:   fmt.Sprintf("数据更新成功，版本: %d", updatedRecord.Version),
		Timestamp: time.Now(),
	}, nil
}

// DeleteData 删除数据（标记删除，保留历史）
func (client *StorageClient) DeleteData(recordID string, requester string) (*StorageResult, error) {
	// 获取原始记录检查权限
	originalRecord, err := client.RetrieveData(recordID, requester)
	if err != nil {
		return &StorageResult{
			Success:   false,
			Message:   fmt.Sprintf("获取记录失败: %v", err),
			Timestamp: time.Now(),
		}, err
	}

	if originalRecord.Owner != requester {
		return &StorageResult{
			Success:   false,
			Message:   "无权限删除此记录",
			Timestamp: time.Now(),
		}, fmt.Errorf("permission denied")
	}

	// 构建删除交易（标记删除）
	params := map[string]interface{}{
		"record_id":        recordID,
		"requester":        requester,
		"delete_timestamp": time.Now(),
	}

	transaction, err := client.buildStorageTransaction(requester, params)
	if err != nil {
		return &StorageResult{
			Success:   false,
			Message:   fmt.Sprintf("构建删除交易失败: %v", err),
			Timestamp: time.Now(),
		}, err
	}

	// 在实际应用中，这里会调用区块链接口
	// txHash, err := client.blockchain.SubmitTransaction(transaction)
	// 这里使用模拟实现
	txHash := client.simulateTransactionSubmission(transaction)
	if txHash == "" {
		return &StorageResult{
			Success:   false,
			Message:   "提交删除失败: 模拟错误",
			Timestamp: time.Now(),
		}, fmt.Errorf("delete submission failed")
	}

	// 从索引中移除
	if err := client.queryEngine.RemoveFromIndex(recordID); err != nil {
		log.Printf("警告: 索引移除失败: %v", err)
	}

	return &StorageResult{
		RecordID:  recordID,
		TxHash:    txHash,
		Success:   true,
		Message:   "数据删除成功",
		Timestamp: time.Now(),
	}, nil
}

// GetDataHistory 获取数据的版本历史
func (client *StorageClient) GetDataHistory(recordID string, requester string) ([]DataRecord, error) {
	params := map[string]interface{}{
		"record_id": recordID,
		"requester": requester,
	}

	// 在实际应用中，这里会调用区块链接口
	// result, err := client.blockchain.CallContract(client.storageContract, "GetDataHistory", params)
	// 这里使用模拟实现
	result, err := client.simulateContractCall("GetDataHistory", params)
	if err != nil {
		return nil, fmt.Errorf("获取历史失败: %v", err)
	}

	var history []DataRecord
	if err := json.Unmarshal(result, &history); err != nil {
		return nil, fmt.Errorf("解析历史失败: %v", err)
	}

	return history, nil
}

// 私有方法：验证存储请求
func (client *StorageClient) validateStorageRequest(request StorageRequest) error {
	if request.Title == "" {
		return fmt.Errorf("标题不能为空")
	}
	if request.Content == "" {
		return fmt.Errorf("内容不能为空")
	}
	if request.Owner == "" {
		return fmt.Errorf("所有者不能为空")
	}
	if request.DataType == "" {
		request.DataType = "text" // 默认类型
	}
	return nil
}

// 私有方法：生成记录ID
func (client *StorageClient) generateRecordID(request StorageRequest) string {
	data := fmt.Sprintf("%s_%s_%d", request.Owner, request.Title, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16]) // 使用前16字节作为ID
}

// 私有方法：计算内容哈希
func (client *StorageClient) calculateContentHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// 私有方法：构建存储交易
func (client *StorageClient) buildStorageTransaction(owner string, params map[string]interface{}) (*Transaction, error) {
	paramsData, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	return &Transaction{
		From:              owner,
		To:                client.storageContract,
		Amount:            0,
		ExecutionFeeLimit: 2000000,
		ExecutionFeePrice: 1,
		Data:              string(paramsData),
		Timestamp:         time.Now().Unix(),
		ContractMethod:    "StoreData",
	}, nil
}

// 私有方法：后处理查询结果
func (client *StorageClient) postProcessResults(records []DataRecord, request QueryRequest) ([]DataRecord, error) {
	var processedRecords []DataRecord

	for _, record := range records {
		// 解密数据（如果需要且有权限）
		if record.IsEncrypted {
			// 这里应该检查权限
			// 为了演示，暂时跳过解密
		}

		processedRecords = append(processedRecords, record)

		// 应用限制
		if request.Limit > 0 && len(processedRecords) >= request.Limit {
			break
		}
	}

	return processedRecords, nil
}

// 私有方法：模拟交易提交
func (client *StorageClient) simulateTransactionSubmission(tx *Transaction) string {
	// 生成模拟交易哈希
	hashData := fmt.Sprintf("%s_%s_%d_%d", tx.From, tx.To, tx.Amount, tx.Timestamp)
	hash := sha256.Sum256([]byte(hashData))
	return hex.EncodeToString(hash[:16]) // 返回前16字节作为交易哈希
}

// 私有方法：模拟合约调用
func (client *StorageClient) simulateContractCall(method string, params map[string]interface{}) ([]byte, error) {
	// 根据方法返回模拟数据
	switch method {
	case "RetrieveData":
		// 返回模拟的数据记录
		record := DataRecord{
			ID:        "demo_record_123",
			Title:     "演示文档",
			Content:   "这是一个演示文档的内容",
			DataType:  "document",
			Owner:     "demo_owner",
			Tags:      []string{"演示", "文档"},
			Hash:      "demo_hash_abc123",
			Timestamp: time.Now(),
			Version:   1,
		}
		return json.Marshal(record)

	case "QueryData":
		// 返回模拟的查询结果
		records := []DataRecord{
			{
				ID:        "demo_record_123",
				Title:     "演示文档",
				Content:   "这是一个演示文档的内容",
				DataType:  "document",
				Owner:     "demo_owner",
				Tags:      []string{"演示", "文档"},
				Hash:      "demo_hash_abc123",
				Timestamp: time.Now(),
				Version:   1,
			},
		}
		return json.Marshal(records)

	case "GetDataHistory":
		// 返回模拟的历史记录
		history := []DataRecord{
			{
				ID:        "demo_record_123",
				Title:     "演示文档 v1",
				Content:   "这是第一版的内容",
				Version:   1,
				Timestamp: time.Now().Add(-time.Hour),
			},
			{
				ID:        "demo_record_123",
				Title:     "演示文档 v2",
				Content:   "这是第二版的内容",
				Version:   2,
				Timestamp: time.Now(),
			},
		}
		return json.Marshal(history)

	default:
		return nil, fmt.Errorf("未知的方法: %s", method)
	}
}

// 演示函数：展示数据存储应用流程
func DemoStorageFlow() {
	fmt.Println("🎮 数据存储应用演示")
	fmt.Println("==================")

	// 注意：这里的代码是演示性质的
	// 实际使用时需要替换为真实的区块链实例和合约地址

	fmt.Println("1. 初始化存储客户端...")
	client := NewStorageClient("demo_storage_contract_address")

	fmt.Println("2. 存储数据...")
	request := StorageRequest{
		Title:    "我的第一个文档",
		Content:  "这是一个示例文档内容",
		DataType: "document",
		Tags:     []string{"示例", "测试"},
		Owner:    "user_address",
		Encrypt:  false,
	}

	fmt.Printf("存储请求: %+v\n", request)
	result, err := client.StoreData(request)
	if err != nil {
		fmt.Printf("存储失败: %v\n", err)
		return
	}
	fmt.Printf("存储结果: %+v\n", result)

	fmt.Println("3. 查询数据...")
	queryReq := QueryRequest{
		Tags:  []string{"示例"},
		Owner: "user_address",
		Limit: 10,
	}

	fmt.Printf("查询请求: %+v\n", queryReq)
	records, err := client.QueryData(queryReq)
	if err != nil {
		fmt.Printf("查询失败: %v\n", err)
		return
	}
	fmt.Printf("查询结果: %d 条记录\n", len(records))

	fmt.Println("4. 检索特定数据...")
	record, err := client.RetrieveData("demo_record_123", "demo_owner")
	if err != nil {
		fmt.Printf("检索失败: %v\n", err)
		return
	}
	fmt.Printf("检索结果: %+v\n", record)

	fmt.Println("✅ 演示完成")
}

func main() {
	DemoStorageFlow()
}
