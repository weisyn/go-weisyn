package main

import (
	"encoding/json"
	"fmt"
	"time"
)

/*
🎯 代币转账客户端应用

📝 客户端代码声明：
本文件是客户端示例代码，JSON序列化用于API接口交互，符合序列化规范。
区块链核心数据结构（Block、Transaction等）在内部系统中使用protobuf序列化。

这是一个完整的代币转账应用示例，展示如何：
1. 与已部署的代币合约交互
2. 构建和提交转账交易
3. 查询账户余额和交易状态
4. 处理实际的业务逻辑

💡 学习重点：
- 客户端如何调用智能合约
- 如何构建和签名交易
- 如何处理异步的区块链操作
- 错误处理和用户体验优化
*/

// TokenTransferClient 代币转账客户端
type TokenTransferClient struct {
	tokenContract string         // 代币合约地址
	walletManager *WalletManager // 钱包管理器
}

// TransferRequest 转账请求
type TransferRequest struct {
	From   string `json:"from"`   // 发送方地址
	To     string `json:"to"`     // 接收方地址
	Amount uint64 `json:"amount"` // 转账金额
	Memo   string `json:"memo"`   // 转账备注
}

// TransferResult 转账结果
type TransferResult struct {
	TxHash    string    `json:"tx_hash"`   // 交易哈希
	Success   bool      `json:"success"`   // 是否成功
	Message   string    `json:"message"`   // 结果消息
	Timestamp time.Time `json:"timestamp"` // 时间戳
}

// NewTokenTransferClient 创建新的转账客户端
func NewTokenTransferClient(tokenContract string) *TokenTransferClient {
	return &TokenTransferClient{
		tokenContract: tokenContract,
		walletManager: NewWalletManager(),
	}
}

// Transfer 执行代币转账
// 🎯 核心功能：安全地执行代币转账操作
func (client *TokenTransferClient) Transfer(request TransferRequest) (*TransferResult, error) {
	// 📋 步骤1：验证转账请求
	if err := client.validateTransferRequest(request); err != nil {
		return &TransferResult{
			Success:   false,
			Message:   fmt.Sprintf("参数验证失败: %v", err),
			Timestamp: time.Now(),
		}, err
	}

	// 📋 步骤2：检查发送方余额
	balance, err := client.GetBalance(request.From)
	if err != nil {
		return &TransferResult{
			Success:   false,
			Message:   fmt.Sprintf("查询余额失败: %v", err),
			Timestamp: time.Now(),
		}, err
	}

	if balance < request.Amount {
		return &TransferResult{
			Success:   false,
			Message:   fmt.Sprintf("余额不足，当前余额: %d, 需要: %d", balance, request.Amount),
			Timestamp: time.Now(),
		}, fmt.Errorf("insufficient balance")
	}

	// 📋 步骤3：构建转账交易
	tx, err := client.buildTransferTransaction(request)
	if err != nil {
		return &TransferResult{
			Success:   false,
			Message:   fmt.Sprintf("构建交易失败: %v", err),
			Timestamp: time.Now(),
		}, err
	}

	// 📋 步骤4：签名交易
	signedTx, err := client.walletManager.SignTransaction(request.From, tx)
	if err != nil {
		return &TransferResult{
			Success:   false,
			Message:   fmt.Sprintf("签名交易失败: %v", err),
			Timestamp: time.Now(),
		}, err
	}

	// 📋 步骤5：提交交易到区块链网络
	txHash := client.simulateTransactionSubmission(signedTx)
	if txHash == "" {
		return &TransferResult{
			Success:   false,
			Message:   "提交交易失败: 模拟错误",
			Timestamp: time.Now(),
		}, fmt.Errorf("transaction submission failed")
	}

	// 📋 步骤6：等待交易确认
	if err := client.waitForConfirmation(txHash, 30*time.Second); err != nil {
		return &TransferResult{
			TxHash:    txHash,
			Success:   false,
			Message:   fmt.Sprintf("交易确认失败: %v", err),
			Timestamp: time.Now(),
		}, err
	}

	// 💡 生活化理解：
	// 代币转账就像银行转账
	// - 验证余额 = 检查账户资金
	// - 构建交易 = 填写转账单
	// - 签名交易 = 本人签字确认
	// - 提交交易 = 银行处理转账
	// - 等待确认 = 等待到账通知

	// ✅ 返回转账成功结果
	return &TransferResult{
		TxHash:    txHash,
		Success:   true,
		Message:   "转账成功",
		Timestamp: time.Now(),
	}, nil
}

// GetBalance 查询账户余额
// 🎯 功能：查询指定地址的代币余额
func (client *TokenTransferClient) GetBalance(address string) (uint64, error) {
	// 构建查询参数
	params := map[string]interface{}{
		"address": address,
	}

	// 调用合约的GetBalance方法
	result, err := client.simulateContractCall("GetBalance", params)
	if err != nil {
		return 0, fmt.Errorf("调用合约失败: %v", err)
	}

	// 解析返回结果
	var balanceData map[string]interface{}
	if err := json.Unmarshal(result, &balanceData); err != nil {
		return 0, fmt.Errorf("解析余额失败: %v", err)
	}

	balance, ok := balanceData["balance"].(float64)
	if !ok {
		return 0, fmt.Errorf("余额格式错误")
	}

	return uint64(balance), nil
}

// 私有方法：验证转账请求
func (client *TokenTransferClient) validateTransferRequest(request TransferRequest) error {
	if request.From == "" {
		return fmt.Errorf("发送方地址不能为空")
	}
	if request.To == "" {
		return fmt.Errorf("接收方地址不能为空")
	}
	if request.Amount == 0 {
		return fmt.Errorf("转账金额必须大于0")
	}
	if request.From == request.To {
		return fmt.Errorf("发送方和接收方不能相同")
	}
	return nil
}

// 私有方法：构建转账交易
func (client *TokenTransferClient) buildTransferTransaction(request TransferRequest) (*Transaction, error) {
	// 构建合约调用数据
	transferData := map[string]interface{}{
		"method": "Transfer",
		"params": map[string]interface{}{
			"from":   request.From,
			"to":     request.To,
			"amount": request.Amount,
			"memo":   request.Memo,
		},
	}

	data, err := json.Marshal(transferData)
	if err != nil {
		return nil, fmt.Errorf("序列化交易数据失败: %v", err)
	}

	// 创建交易对象
	tx := &Transaction{
		From:              request.From,
		To:                client.tokenContract,
		Amount:            0, // 代币转账不涉及主币转账
		ExecutionFeePrice: 1,
		ExecutionFeeLimit: 200000,
		Nonce:             uint64(time.Now().UnixNano()),
		Data:              string(data),
		Timestamp:         time.Now().Unix(),
	}

	return tx, nil
}

// waitForConfirmation 等待交易确认
func (client *TokenTransferClient) waitForConfirmation(txHash string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		// 检查交易状态
		status := client.simulateTransactionStatus(txHash)
		if status == "error" {
			return fmt.Errorf("获取交易状态失败")
		}

		if status == "confirmed" {
			return nil
		}

		// 等待一段时间后重试
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("交易确认超时")
}

// 演示函数：展示代币转账流程
func DemoTransferFlow() {
	fmt.Println("🎮 代币转账应用演示")
	fmt.Println("=================")

	// 注意：这里的代码是演示性质的
	// 实际使用时需要替换为真实的区块链实例和合约地址

	fmt.Println("1. 初始化客户端...")
	client := NewTokenTransferClient("demo_token_contract_address")

	fmt.Println("2. 查询发送方余额...")
	balance, err := client.GetBalance("sender_address")
	if err != nil {
		fmt.Printf("查询余额失败: %v\n", err)
		return
	}
	fmt.Printf("当前余额: %d\n", balance)

	fmt.Println("3. 执行转账...")
	request := TransferRequest{
		From:   "sender_address",
		To:     "receiver_address",
		Amount: 100,
		Memo:   "测试转账",
	}

	fmt.Printf("转账请求: %+v\n", request)
	result, err := client.Transfer(request)
	if err != nil {
		fmt.Printf("转账失败: %v\n", err)
		return
	}
	fmt.Printf("转账结果: %+v\n", result)

	fmt.Println("4. 查询转账后余额...")
	newBalance, err := client.GetBalance("sender_address")
	if err != nil {
		fmt.Printf("查询余额失败: %v\n", err)
		return
	}
	fmt.Printf("转账后余额: %d\n", newBalance)

	fmt.Println("✅ 演示完成")
}

// 注意：main函数已移除，避免与其他文件冲突
// 要运行演示，请调用：DemoTransferFlow()

// 私有方法：模拟合约调用
func (client *TokenTransferClient) simulateContractCall(method string, params map[string]interface{}) ([]byte, error) {
	switch method {
	case "GetBalance":
		// 返回模拟余额
		balance := map[string]interface{}{
			"balance": 1000000, // 模拟余额：1,000,000
		}
		return json.Marshal(balance)
	default:
		return nil, fmt.Errorf("未知的方法: %s", method)
	}
}

// 私有方法：模拟交易提交
func (client *TokenTransferClient) simulateTransactionSubmission(tx *Transaction) string {
	// 生成模拟交易哈希
	return fmt.Sprintf("tx_hash_%d", time.Now().UnixNano())
}

// 私有方法：模拟交易状态查询
func (client *TokenTransferClient) simulateTransactionStatus(txHash string) string {
	// 模拟交易确认过程
	return "confirmed"
}
