package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

/*
🎯 钱包管理模块

这个模块展示如何在应用中管理用户钱包：
1. 创建新的钱包地址
2. 管理私钥和签名
3. 维护本地钱包状态
4. 处理钱包备份和恢复

💡 实际应用考虑：
- 私钥安全存储
- 助记词生成和验证
- 多重签名支持
- 硬件钱包集成
*/

// Wallet 钱包结构
type Wallet struct {
	Address    string    `json:"address"`     // 钱包地址
	PrivateKey string    `json:"private_key"` // 私钥（实际应用中需要加密存储）
	PublicKey  string    `json:"public_key"`  // 公钥
	Balance    uint64    `json:"balance"`     // 余额
	Nonce      uint64    `json:"nonce"`       // 交易计数器
	CreatedAt  time.Time `json:"created_at"`  // 创建时间
}

// WalletManager 钱包管理器
type WalletManager struct {
	wallets map[string]*Wallet // 钱包存储：address -> wallet
	mutex   sync.RWMutex       // 读写锁
}

// TransactionHistory 交易历史
type TransactionHistory struct {
	TxHash    string    `json:"tx_hash"`    // 交易哈希
	From      string    `json:"from"`       // 发送方
	To        string    `json:"to"`         // 接收方
	Amount    uint64    `json:"amount"`     // 金额
	Status    string    `json:"status"`     // 状态
	Timestamp time.Time `json:"timestamp"`  // 时间戳
	BlockHash string    `json:"block_hash"` // 区块哈希
}

// NewWalletManager 创建新的钱包管理器
func NewWalletManager() *WalletManager {
	return &WalletManager{
		wallets: make(map[string]*Wallet),
		mutex:   sync.RWMutex{},
	}
}

// CreateWallet 创建新钱包
// 🎯 功能：生成新的钱包地址和密钥对
func (wm *WalletManager) CreateWallet() (*Wallet, error) {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()

	// 📋 步骤1：生成私钥
	privateKey, err := wm.generatePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("生成私钥失败: %v", err)
	}

	// 📋 步骤2：从私钥推导公钥
	publicKey := wm.derivePublicKey(privateKey)

	// 📋 步骤3：从公钥生成地址
	address := wm.generateAddress(publicKey)

	// 📋 步骤4：创建钱包对象
	wallet := &Wallet{
		Address:    address,
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		Balance:    0,
		Nonce:      0,
		CreatedAt:  time.Now(),
	}

	// 📋 步骤5：存储钱包
	wm.wallets[address] = wallet

	// 💡 生活化理解：
	// 创建钱包就像办银行卡
	// - 私钥 = 银行卡密码（绝对保密）
	// - 公钥 = 银行卡号码的加密形式
	// - 地址 = 银行账户号码（可以公开）
	// - 余额 = 账户资金

	fmt.Printf("✅ 新钱包创建成功: %s\n", address[:16]+"...")

	return wallet, nil
}

// ImportWallet 导入已有钱包
func (wm *WalletManager) ImportWallet(privateKey string) (*Wallet, error) {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()

	// 验证私钥格式
	if !wm.validatePrivateKey(privateKey) {
		return nil, fmt.Errorf("无效的私钥格式")
	}

	// 从私钥推导公钥和地址
	publicKey := wm.derivePublicKey(privateKey)
	address := wm.generateAddress(publicKey)

	// 检查钱包是否已存在
	if _, exists := wm.wallets[address]; exists {
		return nil, fmt.Errorf("钱包已存在")
	}

	// 创建钱包对象
	wallet := &Wallet{
		Address:    address,
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		Balance:    0,
		Nonce:      0,
		CreatedAt:  time.Now(),
	}

	wm.wallets[address] = wallet
	return wallet, nil
}

// GetWallet 获取钱包信息
func (wm *WalletManager) GetWallet(address string) (*Wallet, error) {
	wm.mutex.RLock()
	defer wm.mutex.RUnlock()

	wallet, exists := wm.wallets[address]
	if !exists {
		return nil, fmt.Errorf("钱包不存在: %s", address)
	}

	// 返回钱包副本（不暴露私钥）
	safeCopy := *wallet
	safeCopy.PrivateKey = "***HIDDEN***"
	return &safeCopy, nil
}

// SignTransaction 对交易进行数字签名
// 🎯 功能：使用私钥对交易进行数字签名
func (wm *WalletManager) SignTransaction(address string, transaction *Transaction) (*Transaction, error) {
	wm.mutex.RLock()
	wallet, exists := wm.wallets[address]
	wm.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("钱包不存在: %s", address)
	}

	// 计算交易哈希
	txHash, err := wm.calculateTransactionHash(transaction)
	if err != nil {
		return nil, fmt.Errorf("计算交易哈希失败: %v", err)
	}

	// 使用私钥签名
	signature, err := wm.signHash(txHash, wallet.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("签名失败: %v", err)
	}

	// 创建签名后的交易副本
	signedTx := *transaction
	signedTx.Hash = hex.EncodeToString(txHash)
	signedTx.Signature = signature

	// 更新钱包nonce
	wm.mutex.Lock()
	wallet.Nonce++
	wm.mutex.Unlock()

	return &signedTx, nil
}

// VerifySignature 验证交易签名
func (wm *WalletManager) VerifySignature(transaction *Transaction) error {
	// 从交易中提取发送方地址
	senderAddress := transaction.From

	// 获取发送方钱包
	wm.mutex.RLock()
	wallet, exists := wm.wallets[senderAddress]
	wm.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("发送方钱包不存在")
	}

	// 重新计算交易哈希
	expectedHash, err := wm.calculateTransactionHash(transaction)
	if err != nil {
		return fmt.Errorf("计算交易哈希失败: %v", err)
	}

	// 验证哈希是否匹配
	actualHash, err := hex.DecodeString(transaction.Hash)
	if err != nil {
		return fmt.Errorf("解析交易哈希失败: %v", err)
	}

	if !wm.compareHashes(expectedHash, actualHash) {
		return fmt.Errorf("交易哈希不匹配")
	}

	// 验证签名
	if !wm.verifySignature(expectedHash, transaction.Signature, wallet.PublicKey) {
		return fmt.Errorf("签名验证失败")
	}

	return nil
}

// UpdateBalance 更新钱包余额
func (wm *WalletManager) UpdateBalance(address string, newBalance uint64) error {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()

	wallet, exists := wm.wallets[address]
	if !exists {
		return fmt.Errorf("钱包不存在: %s", address)
	}

	oldBalance := wallet.Balance
	wallet.Balance = newBalance

	fmt.Printf("💰 余额更新 %s: %d -> %d\n", address[:16]+"...", oldBalance, newBalance)
	return nil
}

// EstimateTransactionFee 估算交易费用
func (wm *WalletManager) EstimateTransactionFee(tx *Transaction) (uint64, error) {
	// 基础费用
	baseFee := uint64(1000)

	// 数据大小费用
	dataFee := uint64(len(tx.Data)) * 10

	// 执行费用费用
	执行费用Fee := tx.ExecutionFeePrice * tx.ExecutionFeeLimit

	totalFee := baseFee + dataFee + 执行费用Fee
	return totalFee, nil
}

// ExportWallet 导出钱包（返回私钥）
func (wm *WalletManager) ExportWallet(address string) (string, error) {
	wm.mutex.RLock()
	defer wm.mutex.RUnlock()

	wallet, exists := wm.wallets[address]
	if !exists {
		return "", fmt.Errorf("钱包不存在: %s", address)
	}

	// ⚠️ 警告：在实际应用中，导出私钥需要额外的安全验证
	return wallet.PrivateKey, nil
}

// ListWallets 列出所有钱包
func (wm *WalletManager) ListWallets() []*Wallet {
	wm.mutex.RLock()
	defer wm.mutex.RUnlock()

	var wallets []*Wallet
	for _, wallet := range wm.wallets {
		// 创建安全副本（隐藏私钥）
		safeCopy := *wallet
		safeCopy.PrivateKey = "***HIDDEN***"
		wallets = append(wallets, &safeCopy)
	}

	return wallets
}

// 私有方法：生成私钥
func (wm *WalletManager) generatePrivateKey() (string, error) {
	// 生成32字节随机数作为私钥
	privateKeyBytes := make([]byte, 32)
	_, err := rand.Read(privateKeyBytes)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(privateKeyBytes), nil
}

// 私有方法：从私钥推导公钥
func (wm *WalletManager) derivePublicKey(privateKey string) string {
	// 在实际应用中，这里会使用椭圆曲线密码学（ECC）
	// 这里使用简化的哈希实现
	privateKeyBytes, _ := hex.DecodeString(privateKey)
	publicKeyHash := sha256.Sum256(privateKeyBytes)
	return hex.EncodeToString(publicKeyHash[:])
}

// 私有方法：从公钥生成地址
func (wm *WalletManager) generateAddress(publicKey string) string {
	// 在实际应用中，地址生成会涉及多次哈希和校验和
	publicKeyBytes, _ := hex.DecodeString(publicKey)
	addressHash := sha256.Sum256(publicKeyBytes)

	// 取前20字节作为地址
	address := hex.EncodeToString(addressHash[:20])
	return "0x" + address
}

// 私有方法：验证私钥格式
func (wm *WalletManager) validatePrivateKey(privateKey string) bool {
	// 检查私钥长度（64个十六进制字符 = 32字节）
	if len(privateKey) != 64 {
		return false
	}

	// 检查是否为有效的十六进制字符串
	_, err := hex.DecodeString(privateKey)
	return err == nil
}

// 私有方法：计算交易哈希
func (wm *WalletManager) calculateTransactionHash(tx *Transaction) ([]byte, error) {
	// 构建交易数据字符串
	txData := fmt.Sprintf("%s_%s_%d_%d_%d_%d_%s_%d",
		tx.From, tx.To, tx.Amount, tx.ExecutionFeePrice, tx.ExecutionFeeLimit, tx.Nonce, tx.Data, tx.Timestamp)

	// 计算哈希
	hash := sha256.Sum256([]byte(txData))
	return hash[:], nil
}

// 私有方法：签名哈希
func (wm *WalletManager) signHash(hash []byte, privateKey string) (string, error) {
	// 在实际应用中，这里会使用ECDSA签名算法
	// 这里使用简化的模拟签名
	privateKeyBytes, _ := hex.DecodeString(privateKey)

	// 将私钥和哈希组合后再次哈希作为签名
	signatureData := append(privateKeyBytes, hash...)
	signature := sha256.Sum256(signatureData)

	return hex.EncodeToString(signature[:]), nil
}

// 私有方法：验证签名
func (wm *WalletManager) verifySignature(hash []byte, signature, publicKey string) bool {
	// 在实际应用中，这里会验证ECDSA签名
	// 这里使用简化的验证逻辑
	return len(signature) == 64 && len(publicKey) == 64
}

// 私有方法：比较哈希
func (wm *WalletManager) compareHashes(hash1, hash2 []byte) bool {
	if len(hash1) != len(hash2) {
		return false
	}

	for i := range hash1 {
		if hash1[i] != hash2[i] {
			return false
		}
	}

	return true
}

// 演示函数：展示钱包管理功能
func DemoWalletManager() {
	fmt.Println("🎮 钱包管理器演示")
	fmt.Println("===============")

	// 1. 创建钱包管理器
	fmt.Println("1. 创建钱包管理器...")
	wm := NewWalletManager()

	// 2. 创建新钱包
	fmt.Println("2. 创建新钱包...")
	wallet1, err := wm.CreateWallet()
	if err != nil {
		fmt.Printf("创建钱包失败: %v\n", err)
		return
	}
	fmt.Printf("钱包地址: %s\n", wallet1.Address)

	wallet2, err := wm.CreateWallet()
	if err != nil {
		fmt.Printf("创建钱包失败: %v\n", err)
		return
	}
	fmt.Printf("钱包地址: %s\n", wallet2.Address)

	// 3. 更新余额
	fmt.Println("3. 更新钱包余额...")
	wm.UpdateBalance(wallet1.Address, 1000000)
	wm.UpdateBalance(wallet2.Address, 500000)

	// 4. 创建并签名交易
	fmt.Println("4. 创建并签名交易...")
	tx := &Transaction{
		From:              wallet1.Address,
		To:                wallet2.Address,
		Amount:            100000,
		ExecutionFeePrice: 1,
		ExecutionFeeLimit: 21000,
		Nonce:             1,
		Data:              "transfer_data",
		Timestamp:         time.Now().Unix(),
	}

	signedTx, err := wm.SignTransaction(wallet1.Address, tx)
	if err != nil {
		fmt.Printf("签名交易失败: %v\n", err)
		return
	}

	fmt.Printf("交易哈希: %s\n", signedTx.Hash[:32]+"...")
	fmt.Printf("交易签名: %s\n", signedTx.Signature[:32]+"...")

	// 5. 验证签名
	fmt.Println("5. 验证交易签名...")
	if err := wm.VerifySignature(signedTx); err != nil {
		fmt.Printf("签名验证失败: %v\n", err)
	} else {
		fmt.Println("✅ 签名验证成功")
	}

	// 6. 列出所有钱包
	fmt.Println("6. 列出所有钱包...")
	wallets := wm.ListWallets()
	for i, wallet := range wallets {
		fmt.Printf("钱包 %d: %s (余额: %d)\n", i+1, wallet.Address[:16]+"...", wallet.Balance)
	}

	fmt.Println("✅ 钱包管理演示完成")
}

// 注意：main函数已移除，避免与其他文件冲突
// 要运行演示，请调用：DemoWalletManager()
