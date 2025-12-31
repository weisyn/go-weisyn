package wallet

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/base58"
	"github.com/btcsuite/btcd/chaincfg"
)

// AddressManager 地址管理器接口
//
// 用于生成标准 Base58Check 编码的 WES 地址。
// 根据 WES 地址规范，所有地址必须使用 Base58Check 格式。
type AddressManager interface {
	// PrivateKeyToAddress 从私钥生成标准 Base58Check 地址
	PrivateKeyToAddress(privateKey []byte) (string, error)
}

// unlockedAccountData 解锁账户的运行时数据（内存缓存）
type unlockedAccountData struct {
	PrivateKey   []byte    // 解密后的私钥（32字节）
	UnlockedAt   time.Time // 解锁时间
	LastAccessAt time.Time // 最后访问时间
}

// AccountManager 账户管理器
type AccountManager struct {
	keystoreDir      string
	addressManager   AddressManager                  // 地址管理器（可选，用于标准地址推导）
	unlockedAccounts map[string]*unlockedAccountData // 内存解锁缓存: address -> data
}

// NewAccountManager 创建账户管理器
func NewAccountManager(keystoreDir string, addrMgr AddressManager) (*AccountManager, error) {
	if err := os.MkdirAll(keystoreDir, 0700); err != nil {
		return nil, fmt.Errorf("create keystore dir: %w", err)
	}

	return &AccountManager{
		keystoreDir:      keystoreDir,
		addressManager:   addrMgr,
		unlockedAccounts: make(map[string]*unlockedAccountData),
	}, nil
}

// CreateAccount 创建新账户（使用旧版方式：32字节随机数 + Cf前缀）
func (am *AccountManager) CreateAccount(password string, label string) (*AccountInfo, error) {
	// ✅ 生成32字节随机私钥（旧版方式）
	privateKeyBytes := make([]byte, 32)
	if _, err := rand.Read(privateKeyBytes); err != nil {
		return nil, fmt.Errorf("generate private key: %w", err)
	}

	privateKeyHex := hex.EncodeToString(privateKeyBytes)

	// ✅ 使用AddressManager生成Cf前缀地址（旧版方式）
	address, err := am.deriveAddressCf(privateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("derive address: %w", err)
	}

	// 保存到keystore
	keystorePath, err := am.savePrivateKeyBytes(privateKeyBytes, address, password, label)
	if err != nil {
		return nil, fmt.Errorf("save keystore: %w", err)
	}

	now := time.Now()

	// 检查是否是第一个账户，如果是则设为默认
	accounts, _ := am.ListAccounts()
	isDefault := len(accounts) == 0

	return &AccountInfo{
		ID:            address, // 使用地址作为ID
		Name:          label,
		Address:       address,
		Description:   "",
		PrivateKeyHex: privateKeyHex, // 用于调试
		KeystorePath:  keystorePath,
		Label:         label,
		CreatedAt:     now,
		UpdatedAt:     now,
		IsDefault:     isDefault,
		IsUnlocked:    false,
	}, nil
}

// ListAccounts 列出所有账户
func (am *AccountManager) ListAccounts() ([]*AccountInfo, error) {
	entries, err := os.ReadDir(am.keystoreDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*AccountInfo{}, nil
		}
		return nil, fmt.Errorf("read keystore dir: %w", err)
	}

	var accounts []*AccountInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// 只处理 keystore 文件（UTC--* 格式）
		if !strings.HasPrefix(entry.Name(), "UTC--") {
			continue
		}

		filePath := filepath.Join(am.keystoreDir, entry.Name())
		info, err := am.loadKeystoreInfo(filePath)
		if err != nil {
			// 记录错误但继续
			fmt.Fprintf(os.Stderr, "Warning: failed to load %s: %v\n", entry.Name(), err)
			continue
		}

		accounts = append(accounts, info)
	}

	return accounts, nil
}

// GetAccount 获取账户信息
func (am *AccountManager) GetAccount(address string) (*AccountInfo, error) {
	address = normalizeAddress(address)

	// 遍历keystore目录
	accounts, err := am.ListAccounts()
	if err != nil {
		return nil, err
	}

	for _, account := range accounts {
		if normalizeAddress(account.Address) == address {
			return account, nil
		}
	}

	return nil, fmt.Errorf("account not found: %s", address)
}

// ImportPrivateKey 导入私钥（支持Cf前缀地址）
func (am *AccountManager) ImportPrivateKey(privateKeyHex string, password string, label string) (*AccountInfo, error) {
	// 解码私钥（移除0x或Cf前缀）
	privateKeyHex = strings.TrimPrefix(privateKeyHex, "0x")
	privateKeyHex = strings.TrimPrefix(privateKeyHex, "Cf")

	privateKeyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("decode private key: %w", err)
	}

	if len(privateKeyBytes) != 32 {
		return nil, fmt.Errorf("invalid private key length: expected 32 bytes, got %d", len(privateKeyBytes))
	}

	// ✅ 使用AddressManager生成Cf前缀地址
	address, err := am.deriveAddressCf(privateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("derive address: %w", err)
	}

	// 检查是否已存在
	if _, err := am.GetAccount(address); err == nil {
		return nil, fmt.Errorf("account already exists: %s", address)
	}

	// 保存
	keystorePath, err := am.savePrivateKeyBytes(privateKeyBytes, address, password, label)
	if err != nil {
		return nil, fmt.Errorf("save keystore: %w", err)
	}

	return &AccountInfo{
		Address:      address,
		KeystorePath: keystorePath,
		Label:        label,
		CreatedAt:    time.Now(),
	}, nil
}

// ExportPrivateKey 导出私钥（支持Cf前缀）
func (am *AccountManager) ExportPrivateKey(address string, password string) (string, error) {
	account, err := am.GetAccount(address)
	if err != nil {
		return "", err
	}

	// 读取keystore
	data, err := os.ReadFile(account.KeystorePath)
	if err != nil {
		return "", fmt.Errorf("read keystore: %w", err)
	}

	var keystore KeystoreV1
	if err := json.Unmarshal(data, &keystore); err != nil {
		return "", fmt.Errorf("parse keystore: %w", err)
	}

	// 从密码派生密钥并解密
	privateKeyBytes, err := decryptWithPassword(keystore.Crypto, password)
	if err != nil {
		return "", fmt.Errorf("decrypt private key: %w", err)
	}

	// 返回十六进制格式
	privateKeyHex := hex.EncodeToString(privateKeyBytes)
	return privateKeyHex, nil
}

// decryptWithPassword 使用密码解密（内部辅助函数）
func decryptWithPassword(crypto CryptoV1, password string) ([]byte, error) {
	// 从密码派生密钥
	key, err := deriveKey(password, crypto)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	// 解密
	return decrypt(crypto, key)
}

// DeleteAccount 删除账户
func (am *AccountManager) DeleteAccount(address string) error {
	account, err := am.GetAccount(address)
	if err != nil {
		return err
	}

	// 删除 keystore 文件
	if err := os.Remove(account.KeystorePath); err != nil {
		return fmt.Errorf("delete keystore: %w", err)
	}

	return nil
}

// UpdateLabel 更新账户标签
func (am *AccountManager) UpdateLabel(address string, newLabel string) error {
	account, err := am.GetAccount(address)
	if err != nil {
		return err
	}

	// 读取keystore
	data, err := os.ReadFile(account.KeystorePath)
	if err != nil {
		return fmt.Errorf("read keystore: %w", err)
	}

	var keystore KeystoreV1
	if err := json.Unmarshal(data, &keystore); err != nil {
		return fmt.Errorf("parse keystore: %w", err)
	}

	// 更新标签
	keystore.Label = newLabel

	// 保存
	data, err = json.MarshalIndent(keystore, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal keystore: %w", err)
	}

	if err := os.WriteFile(account.KeystorePath, data, 0600); err != nil {
		return fmt.Errorf("write keystore: %w", err)
	}

	return nil
}

// loadKeystoreInfo 加载keystore信息（不解密）
func (am *AccountManager) loadKeystoreInfo(filePath string) (*AccountInfo, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read keystore: %w", err)
	}

	var keystore KeystoreV1
	if err := json.Unmarshal(data, &keystore); err != nil {
		return nil, fmt.Errorf("parse keystore: %w", err)
	}

	createdAt, _ := time.Parse(time.RFC3339, keystore.CreatedAt)
	address := normalizeAddress(keystore.Address)

	// 检查是否为默认账户（通过Label前缀判断）
	isDefault := strings.HasPrefix(keystore.Label, "[DEFAULT] ")
	label := strings.TrimPrefix(keystore.Label, "[DEFAULT] ")

	// 检查是否已解锁（内存状态）
	isUnlocked := am.IsWalletUnlocked(address)

	return &AccountInfo{
		ID:           address,
		Name:         label,
		Address:      address,
		Description:  "", // keystore v1不存储description
		KeystorePath: filePath,
		Label:        label,
		CreatedAt:    createdAt,
		UpdatedAt:    createdAt, // keystore v1不存储updatedat，使用createdat
		IsDefault:    isDefault,
		IsUnlocked:   isUnlocked,
	}, nil
}

// AccountInfo 账户信息（完整版，对标旧版WalletInfo）
type AccountInfo struct {
	ID            string    `json:"id"`              // 账户唯一标识（使用地址）
	Name          string    `json:"name"`            // 账户名称（同Label）
	Address       string    `json:"address"`         // 账户地址
	Description   string    `json:"description"`     // 账户描述
	PrivateKeyHex string    `json:"-"`               // 不序列化，仅用于创建时返回
	KeystorePath  string    `json:"keystore_path"`   // Keystore文件路径
	Label         string    `json:"label,omitempty"` // 标签
	CreatedAt     time.Time `json:"created_at"`      // 创建时间
	UpdatedAt     time.Time `json:"updated_at"`      // 更新时间
	IsDefault     bool      `json:"is_default"`      // 是否为默认账户
	IsUnlocked    bool      `json:"is_unlocked"`     // 是否已解锁（运行时状态）
}

// ===== 地址生成辅助函数 =====

// deriveAddress 使用 AddressManager 生成标准 Base58Check 地址
//
// 根据 WES 地址规范，所有地址必须使用 Base58Check 编码格式。
// 如果未提供 AddressManager，将返回错误而不是使用旧版格式。
func (am *AccountManager) deriveAddressCf(privateKey []byte) (string, error) {
	// 必须使用 AddressManager 生成标准 Base58Check 地址
	if am.addressManager == nil {
		return "", fmt.Errorf("AddressManager is required for generating standard Base58Check addresses; please provide AddressManager when creating AccountManager")
	}

	address, err := am.addressManager.PrivateKeyToAddress(privateKey)
	if err != nil {
		return "", fmt.Errorf("address manager failed: %w", err)
	}
	return address, nil
}

// savePrivateKeyBytes 保存私钥字节到keystore
func (am *AccountManager) savePrivateKeyBytes(privateKeyBytes []byte, address string, password string, label string) (string, error) {
	// 加密
	crypto, err := encrypt(privateKeyBytes, password)
	if err != nil {
		return "", fmt.Errorf("encrypt: %w", err)
	}

	// 构建keystore
	keystore := KeystoreV1{
		Version:   "1.0.0",
		ID:        generateUUID(),
		Address:   address,
		Crypto:    crypto,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Label:     label,
	}

	// 序列化
	data, err := json.MarshalIndent(keystore, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal keystore: %w", err)
	}

	// 生成文件名: UTC--<timestamp>--<address>
	filename := fmt.Sprintf("UTC--%s--%s",
		time.Now().UTC().Format("2006-01-02T15-04-05.000000000Z"),
		strings.TrimPrefix(strings.ToLower(address), "Cf"),
	)
	filePath := filepath.Join(am.keystoreDir, filename)

	// 保存
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return "", fmt.Errorf("write keystore: %w", err)
	}

	return filePath, nil
}

// normalizeAddress 规范化地址（保持大小写，Base58地址区分大小写）
func normalizeAddress(address string) string {
	// ✅ 只去除首尾空格，保持原始大小写
	// Base58编码的地址（CU..., CW..., Cf...）是大小写敏感的
	return strings.TrimSpace(address)
}

// ============================================================================
// 钱包解锁/锁定功能（从旧版迁移）
// ============================================================================

// UnlockWallet 解锁账户（缓存到内存）
func (am *AccountManager) UnlockWallet(address, password string) error {
	address = normalizeAddress(address)

	// 检查是否已解锁
	if _, exists := am.unlockedAccounts[address]; exists {
		// 更新最后访问时间
		am.unlockedAccounts[address].LastAccessAt = time.Now()
		return nil
	}

	// 获取账户
	account, err := am.GetAccount(address)
	if err != nil {
		return fmt.Errorf("get account: %w", err)
	}

	// 读取keystore
	data, err := os.ReadFile(account.KeystorePath)
	if err != nil {
		return fmt.Errorf("read keystore: %w", err)
	}

	var keystore KeystoreV1
	if err := json.Unmarshal(data, &keystore); err != nil {
		return fmt.Errorf("parse keystore: %w", err)
	}

	// 解密私钥
	privateKeyBytes, err := decryptWithPassword(keystore.Crypto, password)
	if err != nil {
		return fmt.Errorf("decrypt private key: %w (wrong password?)", err)
	}

	// 缓存到内存
	now := time.Now()
	am.unlockedAccounts[address] = &unlockedAccountData{
		PrivateKey:   privateKeyBytes,
		UnlockedAt:   now,
		LastAccessAt: now,
	}

	return nil
}

// LockWallet 锁定账户（从内存中清除）
func (am *AccountManager) LockWallet(address string) error {
	address = normalizeAddress(address)

	// 从内存中删除
	if data, exists := am.unlockedAccounts[address]; exists {
		// 安全清零私钥
		for i := range data.PrivateKey {
			data.PrivateKey[i] = 0
		}
		delete(am.unlockedAccounts, address)
	}

	return nil
}

// IsWalletUnlocked 检查账户是否已解锁
func (am *AccountManager) IsWalletUnlocked(address string) bool {
	address = normalizeAddress(address)
	_, exists := am.unlockedAccounts[address]
	return exists
}

// GetPrivateKey 获取私钥（需要密码或已解锁）
func (am *AccountManager) GetPrivateKey(address, password string) ([]byte, error) {
	address = normalizeAddress(address)

	// 先检查是否已解锁
	if data, exists := am.unlockedAccounts[address]; exists {
		// 更新最后访问时间
		data.LastAccessAt = time.Now()
		// 返回副本，避免外部修改
		privateKeyCopy := make([]byte, len(data.PrivateKey))
		copy(privateKeyCopy, data.PrivateKey)
		return privateKeyCopy, nil
	}

	// 未解锁，需要密码
	if password == "" {
		return nil, fmt.Errorf("wallet is locked, password required")
	}

	// 获取账户
	account, err := am.GetAccount(address)
	if err != nil {
		return nil, fmt.Errorf("get account: %w", err)
	}

	// 读取keystore
	data, err := os.ReadFile(account.KeystorePath)
	if err != nil {
		return nil, fmt.Errorf("read keystore: %w", err)
	}

	var keystore KeystoreV1
	if err := json.Unmarshal(data, &keystore); err != nil {
		return nil, fmt.Errorf("parse keystore: %w", err)
	}

	// 解密私钥
	privateKeyBytes, err := decryptWithPassword(keystore.Crypto, password)
	if err != nil {
		return nil, fmt.Errorf("decrypt private key: %w (wrong password?)", err)
	}

	return privateKeyBytes, nil
}

// CleanupExpiredSessions 清理过期的解锁会话（应定期调用）
func (am *AccountManager) CleanupExpiredSessions(timeout time.Duration) {
	if timeout == 0 {
		timeout = 30 * time.Minute // 默认30分钟超时
	}

	now := time.Now()
	for address, data := range am.unlockedAccounts {
		if now.Sub(data.LastAccessAt) > timeout {
			// 安全清零私钥
			for i := range data.PrivateKey {
				data.PrivateKey[i] = 0
			}
			delete(am.unlockedAccounts, address)
		}
	}
}

// ============================================================================
// 密码管理功能（从旧版迁移）
// ============================================================================

// ChangePassword 修改账户密码
func (am *AccountManager) ChangePassword(address, oldPassword, newPassword string) error {
	address = normalizeAddress(address)

	// 获取账户
	account, err := am.GetAccount(address)
	if err != nil {
		return fmt.Errorf("get account: %w", err)
	}

	// 读取keystore
	data, err := os.ReadFile(account.KeystorePath)
	if err != nil {
		return fmt.Errorf("read keystore: %w", err)
	}

	var keystore KeystoreV1
	if err := json.Unmarshal(data, &keystore); err != nil {
		return fmt.Errorf("parse keystore: %w", err)
	}

	// 用旧密码解密私钥
	privateKeyBytes, err := decryptWithPassword(keystore.Crypto, oldPassword)
	if err != nil {
		return fmt.Errorf("old password incorrect: %w", err)
	}

	// 用新密码重新加密
	newCrypto, err := encrypt(privateKeyBytes, newPassword)
	if err != nil {
		return fmt.Errorf("encrypt with new password: %w", err)
	}

	// 更新keystore
	keystore.Crypto = newCrypto

	// 保存回文件
	data, err = json.MarshalIndent(keystore, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal keystore: %w", err)
	}

	if err := os.WriteFile(account.KeystorePath, data, 0600); err != nil {
		return fmt.Errorf("write keystore: %w", err)
	}

	// 如果账户已解锁，更新内存中的私钥
	if unlocked, exists := am.unlockedAccounts[address]; exists {
		copy(unlocked.PrivateKey, privateKeyBytes)
		unlocked.LastAccessAt = time.Now()
	}

	return nil
}

// ValidatePassword 验证账户密码是否正确
func (am *AccountManager) ValidatePassword(address, password string) (bool, error) {
	address = normalizeAddress(address)

	// 尝试用密码解密私钥
	_, err := am.GetPrivateKey(address, password)
	if err != nil {
		if strings.Contains(err.Error(), "wrong password") {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// ValidatePrivateKey 验证私钥格式并返回对应地址
func (am *AccountManager) ValidatePrivateKey(privateKeyHex string) (bool, string, error) {
	// 移除前缀
	privateKeyHex = strings.TrimPrefix(privateKeyHex, "0x")
	privateKeyHex = strings.TrimPrefix(privateKeyHex, "Cf")

	// 解码
	privateKeyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return false, "", fmt.Errorf("invalid hex format: %w", err)
	}

	// 检查长度
	if len(privateKeyBytes) != 32 {
		return false, "", fmt.Errorf("invalid private key length: expected 32 bytes, got %d", len(privateKeyBytes))
	}

	// 推导地址
	address, err := am.deriveAddressCf(privateKeyBytes)
	if err != nil {
		return false, "", fmt.Errorf("derive address: %w", err)
	}

	return true, address, nil
}

// ============================================================================
// 默认钱包管理功能（从旧版迁移）
// ============================================================================

// SetDefaultWallet 设置默认账户
func (am *AccountManager) SetDefaultWallet(address string) error {
	address = normalizeAddress(address)

	// 获取所有账户
	accounts, err := am.ListAccounts()
	if err != nil {
		return fmt.Errorf("list accounts: %w", err)
	}

	// 更新所有账户的默认标记
	found := false
	for _, account := range accounts {
		// 读取keystore
		data, err := os.ReadFile(account.KeystorePath)
		if err != nil {
			continue
		}

		var keystore KeystoreV1
		if err := json.Unmarshal(data, &keystore); err != nil {
			continue
		}

		// 更新默认标记（通过Label字段存储，添加前缀）
		accountAddress := normalizeAddress(account.Address)
		if accountAddress == address {
			// 在Label中添加[DEFAULT]标记
			if !strings.HasPrefix(keystore.Label, "[DEFAULT] ") {
				keystore.Label = "[DEFAULT] " + keystore.Label
			}
			found = true
		} else {
			// 移除[DEFAULT]标记
			keystore.Label = strings.TrimPrefix(keystore.Label, "[DEFAULT] ")
		}

		// 保存回文件
		data, err = json.MarshalIndent(keystore, "", "  ")
		if err != nil {
			continue
		}

		if err := os.WriteFile(account.KeystorePath, data, 0600); err != nil {
			continue
		}
	}

	if !found {
		return fmt.Errorf("account not found: %s", address)
	}

	return nil
}

// GetDefaultWallet 获取默认账户
func (am *AccountManager) GetDefaultWallet() (*AccountInfo, error) {
	accounts, err := am.ListAccounts()
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}

	if len(accounts) == 0 {
		return nil, fmt.Errorf("no accounts available")
	}

	// 查找带[DEFAULT]标记的账户
	for _, account := range accounts {
		if account.IsDefault {
			return account, nil
		}
	}

	// 如果没有默认账户，返回第一个
	return accounts[0], nil
}

// ============================================================================
// 从文件导入功能（从旧版迁移）
// ============================================================================

// ImportWalletFromFile 从文件导入账户
func (am *AccountManager) ImportWalletFromFile(name, filePath, password string) (*AccountInfo, error) {
	// 读取文件
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	// 尝试解析为keystore格式
	var keystore KeystoreV1
	if err := json.Unmarshal(data, &keystore); err == nil {
		// 这是一个keystore文件，尝试解密
		privateKeyBytes, err := decryptWithPassword(keystore.Crypto, password)
		if err != nil {
			return nil, fmt.Errorf("decrypt keystore: %w (wrong password?)", err)
		}

		// 使用解密的私钥导入
		privateKeyHex := hex.EncodeToString(privateKeyBytes)
		return am.ImportPrivateKey(privateKeyHex, password, name)
	}

	// 尝试作为私钥文件（纯文本十六进制）
	privateKeyHex := strings.TrimSpace(string(data))
	return am.ImportPrivateKey(privateKeyHex, password, name)
}

// ============================================================================
// WIF 格式支持（Bitcoin Wallet Import Format）
// ============================================================================

// WESWIFPrivateKeyID WES 私钥 WIF 前缀版本号
// 使用 0x9C (156) 对应 WES 的 P2PKH 版本号 0x1C (28)
// 规则: WIF版本 = 地址版本 + 0x80
const WESWIFPrivateKeyID = 0x9C

// ImportWIF 从 WIF 格式导入私钥
// WIF (Wallet Import Format) 是 Bitcoin 标准的私钥编码格式
// WES 使用自定义的 WIF 版本号 (0x9C) 以区分 Bitcoin
func (am *AccountManager) ImportWIF(wifString, password, label string) (*AccountInfo, error) {
	wifString = strings.TrimSpace(wifString)
	if wifString == "" {
		return nil, fmt.Errorf("WIF string is empty")
	}

	// 尝试使用 WES 网络参数解码
	// 创建自定义 WES 网络参数
	wesNetParams := &chaincfg.Params{
		Name:             "wes-mainnet",
		PrivateKeyID:     WESWIFPrivateKeyID, // 0x9C
		HDPrivateKeyID:   [4]byte{0x04, 0x88, 0xAD, 0xE4},
		HDPublicKeyID:    [4]byte{0x04, 0x88, 0xB2, 0x1E},
	}

	wif, err := btcutil.DecodeWIF(wifString)
	if err != nil {
		// 尝试使用 Bitcoin mainnet 参数解码（兼容 BTC 私钥）
		wif, err = btcutil.DecodeWIF(wifString)
		if err != nil {
			return nil, fmt.Errorf("invalid WIF format: %w", err)
		}
	}

	// 验证 WIF 是否有效
	if !wif.IsForNet(wesNetParams) && !wif.IsForNet(&chaincfg.MainNetParams) {
		return nil, fmt.Errorf("WIF is not for WES or Bitcoin mainnet")
	}

	// 获取私钥字节
	privateKeyBytes := wif.PrivKey.Serialize()

	// 使用 AddressManager 生成地址
	address, err := am.deriveAddressCf(privateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("derive address: %w", err)
	}

	// 检查是否已存在
	if _, err := am.GetAccount(address); err == nil {
		return nil, fmt.Errorf("account already exists: %s", address)
	}

	// 保存到 keystore
	keystorePath, err := am.savePrivateKeyBytes(privateKeyBytes, address, password, label)
	if err != nil {
		return nil, fmt.Errorf("save keystore: %w", err)
	}

	return &AccountInfo{
		Address:      address,
		KeystorePath: keystorePath,
		Label:        label,
		CreatedAt:    time.Now(),
	}, nil
}

// ExportWIF 导出为 WIF 格式（WES 专用格式）
// compressed: 是否使用压缩公钥格式（推荐 true）
func (am *AccountManager) ExportWIF(address, password string, compressed bool) (string, error) {
	// 获取私钥
	privateKeyBytes, err := am.GetPrivateKey(address, password)
	if err != nil {
		return "", fmt.Errorf("get private key: %w", err)
	}

	// 手动构建 WIF（使用 WES 版本号）
	// WIF 格式: 版本(1) + 私钥(32) + [压缩标记(1)] + 校验和(4)
	if compressed {
		payload := make([]byte, 33)
		copy(payload[0:32], privateKeyBytes)
		payload[32] = 0x01 // 压缩公钥标记
		return base58.CheckEncode(payload, WESWIFPrivateKeyID), nil
	}
	
	return base58.CheckEncode(privateKeyBytes, WESWIFPrivateKeyID), nil
}

// ExportWIFWithBTCFormat 导出为 Bitcoin 兼容的 WIF 格式
// 使用 Bitcoin mainnet 的 WIF 版本号 (0x80)
// 注意：这会生成 Bitcoin 格式的 WIF，但地址在两条链上不同
func (am *AccountManager) ExportWIFWithBTCFormat(address, password string, compressed bool) (string, error) {
	// 获取私钥
	privateKeyBytes, err := am.GetPrivateKey(address, password)
	if err != nil {
		return "", fmt.Errorf("get private key: %w", err)
	}

	// 解析为 btcec 私钥
	btcecPrivKey, err := parsePrivateKey(privateKeyBytes)
	if err != nil {
		return "", fmt.Errorf("parse private key: %w", err)
	}
	_ = btcecPrivKey

	// 手动构建 WIF（避免类型转换问题）
	// WIF 格式: 版本(1) + 私钥(32) + [压缩标记(1)] + 校验和(4)
	var payload []byte
	if compressed {
		payload = make([]byte, 34)
		payload[0] = 0x80 // Bitcoin mainnet WIF 版本
		copy(payload[1:33], privateKeyBytes)
		payload[33] = 0x01 // 压缩公钥标记
	} else {
		payload = make([]byte, 33)
		payload[0] = 0x80 // Bitcoin mainnet WIF 版本
		copy(payload[1:33], privateKeyBytes)
	}

	// Base58Check 编码
	return base58.CheckEncode(payload[1:], payload[0]), nil
}

// ============================================================================
// 助记词钱包支持
// ============================================================================

// CreateAccountFromMnemonic 从助记词创建账户
// 这是一个快捷方法，内部使用 MnemonicSigner
func (am *AccountManager) CreateAccountFromMnemonic(mnemonic, passphrase, password, label string) (*AccountInfo, error) {
	if am.addressManager == nil {
		return nil, fmt.Errorf("address manager is required for mnemonic accounts")
	}

	// 创建助记词签名器
	signer, err := NewMnemonicSigner(MnemonicSignerConfig{
		Mnemonic:       mnemonic,
		Passphrase:     passphrase,
		AddressManager: am.addressManager,
	})
	if err != nil {
		return nil, fmt.Errorf("create mnemonic signer: %w", err)
	}

	// 解锁以派生密钥
	if err := signer.Unlock("", 0); err != nil {
		return nil, fmt.Errorf("unlock signer: %w", err)
	}
	defer signer.Lock()

	// 获取默认地址
	address, err := signer.GetAddress("")
	if err != nil {
		return nil, fmt.Errorf("get address: %w", err)
	}

	// 获取私钥字节
	privateKeyBytes, err := signer.GetPrivateKeyBytes(address)
	if err != nil {
		return nil, fmt.Errorf("get private key: %w", err)
	}

	// 检查是否已存在
	if _, err := am.GetAccount(address); err == nil {
		return nil, fmt.Errorf("account already exists: %s", address)
	}

	// 保存到 keystore（添加助记词派生路径信息到 label）
	derivationPath := signer.GetDerivationPath()
	fullLabel := label
	if label != "" {
		fullLabel = fmt.Sprintf("%s [HD:%s]", label, derivationPath)
	} else {
		fullLabel = fmt.Sprintf("[HD:%s]", derivationPath)
	}

	keystorePath, err := am.savePrivateKeyBytes(privateKeyBytes, address, password, fullLabel)
	if err != nil {
		return nil, fmt.Errorf("save keystore: %w", err)
	}

	return &AccountInfo{
		Address:      address,
		KeystorePath: keystorePath,
		Label:        fullLabel,
		CreatedAt:    time.Now(),
	}, nil
}

// GenerateNewMnemonic 生成新的助记词
// 返回助记词字符串，调用方应安全存储
func (am *AccountManager) GenerateNewMnemonic(strength MnemonicStrength) (string, error) {
	mm := NewMnemonicManager()
	return mm.GenerateMnemonic(strength)
}

// ValidateMnemonic 验证助记词是否有效
func (am *AccountManager) ValidateMnemonic(mnemonic string) (bool, string) {
	mm := NewMnemonicManager()
	return mm.ValidateMnemonicWithDetails(mnemonic)
}

// DeriveAddressFromMnemonic 从助记词派生指定路径的地址（不保存）
// 用于预览地址或验证助记词
func (am *AccountManager) DeriveAddressFromMnemonic(mnemonic, passphrase, path string) (string, error) {
	if am.addressManager == nil {
		return "", fmt.Errorf("address manager is required")
	}

	signer, err := NewMnemonicSigner(MnemonicSignerConfig{
		Mnemonic:       mnemonic,
		Passphrase:     passphrase,
		AddressManager: am.addressManager,
	})
	if err != nil {
		return "", err
	}

	if err := signer.Unlock("", 0); err != nil {
		return "", err
	}
	defer signer.Lock()

	if path == "" {
		path = DefaultDerivationPath().String()
	}

	return signer.DeriveAddress(path)
}
