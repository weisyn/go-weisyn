// Package src provides transaction building functionality for basic token transfer example.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

/*
🎯 交易构建模块

这个模块展示如何在应用中构建区块链交易：
1. 构建合约调用交易
2. 设置交易参数和手续费
3. 处理不同类型的合约操作
4. 优化交易性能和成本

💡 实际应用考虑：
- 动态执行费用费估算
- 交易打包优化
- 重发和加速机制
- 批量操作支持
*/

// Transaction 交易结构定义
type Transaction struct {
	From              string `json:"from"`                // 发送方地址
	To                string `json:"to"`                  // 接收方地址
	Amount            uint64 `json:"amount"`              // 转账金额
	ExecutionFeePrice uint64 `json:"execution_fee_price"` // 执行费用价格
	ExecutionFeeLimit uint64 `json:"execution_fee_limit"` // 执行费用限制
	Nonce             uint64 `json:"nonce"`               // 交易序号
	Data              string `json:"data"`                // 交易数据
	Timestamp         int64  `json:"timestamp"`           // 时间戳
	Hash              string `json:"hash"`                // 交易哈希
	Signature         string `json:"signature"`           // 数字签名
}

// TransactionBuilder 交易构建器
type TransactionBuilder struct {
	nonce             uint64            // 交易序号
	ExecutionFeePrice uint64            // 执行费用价格
	ExecutionFeeLimit uint64            // 执行费用限制
	metadata          map[string]string // 额外元数据
}

// TransferInfo 转账信息
type TransferInfo struct {
	To     string `json:"to"`     // 接收方地址
	Amount uint64 `json:"amount"` // 转账金额
	Memo   string `json:"memo"`   // 转账备注
}

// NewTransactionBuilder 创建新的交易构建器
func NewTransactionBuilder() *TransactionBuilder {
	return &TransactionBuilder{
		nonce:             1,
		ExecutionFeePrice: 1,
		ExecutionFeeLimit: 200000,
		metadata:          make(map[string]string),
	}
}

// BuildTransferTransaction 构建代币转账交易
// 🎯 功能：创建一个安全的代币转账交易
func (tb *TransactionBuilder) BuildTransferTransaction(from, to string, amount uint64) (*Transaction, error) {
	// 📋 步骤1：验证输入参数
	if err := tb.validateTransferParams(from, to, amount); err != nil {
		return nil, err
	}

	// 📋 步骤2：构建合约调用数据
	transferData := map[string]interface{}{
		"method": "Transfer",
		"params": map[string]interface{}{
			"from":   from,
			"to":     to,
			"amount": amount,
		},
	}

	jsonData, err := json.Marshal(transferData)
	if err != nil {
		return nil, fmt.Errorf("序列化转账数据失败: %v", err)
	}

	// 📋 步骤3：创建交易对象
	tx := &Transaction{
		From:              from,
		To:                to,
		Amount:            amount,
		ExecutionFeePrice: tb.ExecutionFeePrice,
		ExecutionFeeLimit: tb.ExecutionFeeLimit,
		Nonce:             tb.nonce,
		Data:              string(jsonData),
		Timestamp:         time.Now().Unix(),
	}

	// 📋 步骤4：计算交易哈希
	tx.Hash = tb.CalculateTransactionHash(tx)

	// 📋 步骤5：增加nonce（防止重放攻击）
	tb.nonce++

	return tx, nil
}

// BuildContractCallTransaction 构建合约调用交易
func (tb *TransactionBuilder) BuildContractCallTransaction(from, contractAddr, method string, params map[string]interface{}) (*Transaction, error) {
	// 构建合约调用数据
	callData := map[string]interface{}{
		"method": method,
		"params": params,
	}

	jsonData, err := json.Marshal(callData)
	if err != nil {
		return nil, fmt.Errorf("序列化合约调用数据失败: %v", err)
	}

	return &Transaction{
		From:              from,
		To:                contractAddr,
		Amount:            0, // 合约调用通常不转账
		ExecutionFeePrice: tb.ExecutionFeePrice,
		ExecutionFeeLimit: tb.ExecutionFeeLimit,
		Nonce:             tb.nonce,
		Data:              string(jsonData),
		Timestamp:         time.Now().Unix(),
	}, nil
}

// BuildBatchTransferTransaction 构建批量转账交易
func (tb *TransactionBuilder) BuildBatchTransferTransaction(from string, transfers []TransferInfo) (*Transaction, error) {
	if len(transfers) == 0 {
		return nil, fmt.Errorf("转账列表不能为空")
	}

	// 计算总转账金额
	var totalAmount uint64
	for _, transfer := range transfers {
		totalAmount += transfer.Amount
	}

	// 构建批量转账数据
	batchData := map[string]interface{}{
		"method": "BatchTransfer",
		"params": map[string]interface{}{
			"from":      from,
			"transfers": transfers,
		},
	}

	jsonData, err := json.Marshal(batchData)
	if err != nil {
		return nil, fmt.Errorf("序列化批量转账数据失败: %v", err)
	}

	return &Transaction{
		From:              from,
		To:                "batch_transfer_contract", // 批量转账合约地址
		Amount:            totalAmount,
		ExecutionFeePrice: tb.ExecutionFeePrice,
		ExecutionFeeLimit: tb.ExecutionFeeLimit * uint64(len(transfers)), // 根据转账数量调整执行费用
		Nonce:             tb.nonce,
		Data:              string(jsonData),
		Timestamp:         time.Now().Unix(),
	}, nil
}

// BuildTimeLockTransaction 构建时间锁定交易
func (tb *TransactionBuilder) BuildTimeLockTransaction(from, to string, amount uint64, unlockTime int64) (*Transaction, error) {
	if unlockTime <= time.Now().Unix() {
		return nil, fmt.Errorf("解锁时间必须在未来")
	}

	// 构建时间锁数据
	lockData := map[string]interface{}{
		"method": "TimeLock",
		"params": map[string]interface{}{
			"from":        from,
			"to":          to,
			"amount":      amount,
			"unlock_time": unlockTime,
		},
	}

	jsonData, err := json.Marshal(lockData)
	if err != nil {
		return nil, fmt.Errorf("序列化时间锁数据失败: %v", err)
	}

	return &Transaction{
		From:              from,
		To:                "timelock_contract", // 时间锁合约地址
		Amount:            amount,
		ExecutionFeePrice: tb.ExecutionFeePrice,
		ExecutionFeeLimit: tb.ExecutionFeeLimit,
		Nonce:             tb.nonce,
		Data:              string(jsonData),
		Timestamp:         time.Now().Unix(),
	}, nil
}

// CalculateTransactionHash 计算交易哈希
func (tb *TransactionBuilder) CalculateTransactionHash(tx *Transaction) string {
	// 构建用于哈希的数据字符串
	hashData := fmt.Sprintf("%s_%s_%d_%d_%d_%d_%s_%d",
		tx.From, tx.To, tx.Amount, tx.ExecutionFeePrice, tx.ExecutionFeeLimit, tx.Nonce, tx.Data, tx.Timestamp)

	// 计算SHA256哈希
	hash := sha256.Sum256([]byte(hashData))
	return hex.EncodeToString(hash[:])
}

// SignTransaction 对交易进行数字签名
func (tb *TransactionBuilder) SignTransaction(tx *Transaction, privateKey string) *Transaction {
	// 计算交易哈希
	hash := tb.CalculateTransactionHash(tx)

	// 💡 实际实现中，这里会使用真正的密码学签名
	// 这里使用简化的模拟签名
	signature := tb.generateMockSignature(hash, privateKey)

	// 返回签名后的交易
	signedTx := *tx
	signedTx.Hash = hash
	signedTx.Signature = signature

	return &signedTx
}

// Estimate执行费用 估算交易执行费用费用
func (tb *TransactionBuilder) Estimate执行费用(tx *Transaction) uint64 {
	// 基础执行费用费用
	base执行费用 := uint64(21000)

	// 数据执行费用费用（每字节4 执行费用）
	data执行费用 := uint64(len(tx.Data)) * 4

	// 合约调用额外费用
	contract执行费用 := uint64(0)
	if tx.To != tx.From {
		contract执行费用 = 50000
	}

	total执行费用 := base执行费用 + data执行费用 + contract执行费用

	// 添加10%的安全边际
	safetyMargin := total执行费用 / 10
	estimated执行费用 := total执行费用 + safetyMargin

	return estimated执行费用
}

// SetExecutionFeePrice 设置执行费用价格
func (tb *TransactionBuilder) SetExecutionFeePrice(executionFeePrice uint64) { //nolint:gocritic // captLocal: 参数名已修复为小写
	tb.ExecutionFeePrice = executionFeePrice
}

// SetExecutionFeeLimit 设置执行费用限制
func (tb *TransactionBuilder) SetExecutionFeeLimit(executionFeeLimit uint64) { //nolint:gocritic // captLocal: 参数名已修复为小写
	tb.ExecutionFeeLimit = executionFeeLimit
}

// AddMetadata 添加元数据
func (tb *TransactionBuilder) AddMetadata(key, value string) {
	tb.metadata[key] = value
}

// 私有方法：验证转账参数
func (tb *TransactionBuilder) validateTransferParams(from, to string, amount uint64) error {
	if from == "" {
		return fmt.Errorf("发送方地址不能为空")
	}
	if to == "" {
		return fmt.Errorf("接收方地址不能为空")
	}
	if amount == 0 {
		return fmt.Errorf("转账金额必须大于0")
	}
	if from == to {
		return fmt.Errorf("发送方和接收方不能相同")
	}
	return nil
}

// 私有方法：生成模拟签名
func (tb *TransactionBuilder) generateMockSignature(hash, privateKey string) string {
	// 在实际应用中，这里会使用椭圆曲线数字签名算法（ECDSA）
	// 这里使用简化的模拟签名
	signatureData := hash + privateKey + fmt.Sprintf("%d", time.Now().UnixNano())
	sigHash := sha256.Sum256([]byte(signatureData))
	return hex.EncodeToString(sigHash[:32]) // 返回前32字节作为模拟签名
}

// 演示函数：展示交易构建功能
func DemoTransactionBuilder() {
	fmt.Println("🎮 交易构建器演示")
	fmt.Println("===============")

	// 1. 创建交易构建器
	fmt.Println("1. 创建交易构建器...")
	builder := NewTransactionBuilder()

	// 2. 构建简单转账交易
	fmt.Println("2. 构建转账交易...")
	tx, err := builder.BuildTransferTransaction("alice", "bob", 1000)
	if err != nil {
		fmt.Printf("构建转账交易失败: %v\n", err)
		return
	}
	fmt.Printf("转账交易: %+v\n", tx)

	// 3. 构建批量转账交易
	fmt.Println("3. 构建批量转账交易...")
	transfers := []TransferInfo{
		{To: "bob", Amount: 100, Memo: "转账给Bob"},
		{To: "charlie", Amount: 200, Memo: "转账给Charlie"},
	}

	batchTx, err := builder.BuildBatchTransferTransaction("alice", transfers)
	if err != nil {
		fmt.Printf("构建批量转账失败: %v\n", err)
		return
	}
	fmt.Printf("批量转账交易: %+v\n", batchTx)

	// 4. 签名交易
	fmt.Println("4. 签名交易...")
	signedTx := builder.SignTransaction(tx, "alice_private_key")
	fmt.Printf("签名后交易哈希: %s\n", signedTx.Hash)
	fmt.Printf("交易签名: %s\n", signedTx.Signature[:32]+"...")

	// 5. 估算执行费用费用
	fmt.Println("5. 估算执行费用费用...")
	执行费用Estimate := builder.Estimate执行费用(tx)
	fmt.Printf("估算执行费用费用: %d\n", 执行费用Estimate)

	fmt.Println("✅ 交易构建演示完成")
}

// 注意：main函数已移除，避免与其他文件冲突
// 要运行演示，请调用：DemoTransactionBuilder()
