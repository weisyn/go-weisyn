// Package wallet 提供钱包管理功能
package wallet

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/pbkdf2"

	cryptointf "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// WalletManager 钱包管理器接口
type WalletManager interface {
	// 钱包创建
	CreateWallet(ctx context.Context, req *CreateWalletRequest) (*WalletInfo, error)

	// 钱包导入
	ImportWallet(ctx context.Context, req *ImportWalletRequest) (*WalletInfo, error)
	ImportWalletFromFile(ctx context.Context, name, filePath, password string) (*WalletInfo, error)

	// 钱包列表管理
	ListWallets(ctx context.Context) ([]*WalletInfo, error)
	GetWallet(ctx context.Context, walletID string) (*WalletInfo, error)
	DeleteWallet(ctx context.Context, walletID string) error

	// 钱包解锁/锁定
	UnlockWallet(ctx context.Context, walletID, password string) error
	LockWallet(ctx context.Context, walletID string) error
	IsWalletUnlocked(ctx context.Context, walletID string) (bool, error)

	// 私钥管理
	GetPrivateKey(ctx context.Context, walletID, password string) ([]byte, error)
	ChangePassword(ctx context.Context, walletID, oldPassword, newPassword string) error

	// 配置管理
	SetDefaultWallet(ctx context.Context, walletID string) error
	GetDefaultWallet(ctx context.Context) (*WalletInfo, error)

	// 钱包验证
	ValidatePassword(ctx context.Context, walletID, password string) (bool, error)
	ValidatePrivateKey(privateKey string) (bool, string, error)
}

// WalletInfo 钱包信息
type WalletInfo struct {
	ID           string    `json:"id"`            // 钱包唯一标识
	Name         string    `json:"name"`          // 钱包名称
	Address      string    `json:"address"`       // 钱包地址
	Description  string    `json:"description"`   // 钱包描述
	CreatedAt    time.Time `json:"created_at"`    // 创建时间
	UpdatedAt    time.Time `json:"updated_at"`    // 更新时间
	IsDefault    bool      `json:"is_default"`    // 是否为默认钱包
	IsUnlocked   bool      `json:"is_unlocked"`   // 是否已解锁
	KeystorePath string    `json:"keystore_path"` // 密钥文件路径
}

// CreateWalletRequest 创建钱包请求参数
type CreateWalletRequest struct {
	Name        string `json:"name"`        // 钱包名称（必填）
	Password    string `json:"password"`    // 用户密码（用于加密私钥）
	Description string `json:"description"` // 钱包描述（可选）
}

// ImportWalletRequest 导入钱包请求参数
type ImportWalletRequest struct {
	Name        string `json:"name"`        // 钱包名称（必填）
	Password    string `json:"password"`    // 用户密码（用于加密私钥）
	PrivateKey  string `json:"private_key"` // 私钥（十六进制，二选一）
	Mnemonic    string `json:"mnemonic"`    // 助记词（二选一）
	Description string `json:"description"` // 钱包描述（可选）
}

// EncryptedWallet 加密钱包存储结构
type EncryptedWallet struct {
	ID             string `json:"id"`              // 钱包ID
	Name           string `json:"name"`            // 钱包名称
	Address        string `json:"address"`         // 钱包地址
	EncryptedKey   string `json:"encrypted_key"`   // 加密的私钥
	Salt           string `json:"salt"`            // 加密盐值
	CreatedAt      int64  `json:"created_at"`      // 创建时间戳
	UpdatedAt      int64  `json:"updated_at"`      // 更新时间戳
	IsDefault      bool   `json:"is_default"`      // 是否为默认钱包
	PasswordHint   string `json:"password_hint"`   // 密码提示（可选）
	KeyDerivation  string `json:"key_derivation"`  // 密钥派生算法标识
	EncryptionType string `json:"encryption_type"` // 加密算法类型
}

// WalletStorage 钱包存储文件结构
type WalletStorage struct {
	Version   string             `json:"version"`    // 存储格式版本
	Wallets   []*EncryptedWallet `json:"wallets"`    // 钱包列表
	CreatedAt int64              `json:"created_at"` // 存储文件创建时间
	UpdatedAt int64              `json:"updated_at"` // 存储文件更新时间
}

// walletManager 钱包管理器实现
type walletManager struct {
	logger     log.Logger
	storageDir string

	// 运行时状态
	unlockedWallets map[string]*unlockedWalletData // walletID -> 解锁的钱包数据
	addressManager  cryptointf.AddressManager
}

// unlockedWalletData 解锁钱包的运行时数据
type unlockedWalletData struct {
	PrivateKey   string    // 解密后的私钥
	UnlockedAt   time.Time // 解锁时间
	LastAccessAt time.Time // 最后访问时间
}

// NewWalletManager 创建钱包管理器
func NewWalletManager(logger log.Logger, storageDir string, addressManager cryptointf.AddressManager) WalletManager {
	if storageDir == "" {
		homeDir, _ := os.UserHomeDir()
		storageDir = filepath.Join(homeDir, ".weisyn_cli", "wallets")
	}

	// 确保存储目录存在
	os.MkdirAll(storageDir, 0700)

	return &walletManager{
		logger:          logger,
		storageDir:      storageDir,
		unlockedWallets: make(map[string]*unlockedWalletData),
		addressManager:  addressManager,
	}
}

// CreateWallet 创建新钱包
func (w *walletManager) CreateWallet(ctx context.Context, req *CreateWalletRequest) (*WalletInfo, error) {
	w.logger.Info(fmt.Sprintf("开始创建钱包: name=%s", req.Name))

	// 生成私钥
	privateKey, address, err := w.generateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("生成密钥对失败: %v", err)
	}

	// 创建钱包信息
	walletInfo := &WalletInfo{
		ID:          generateWalletID(),
		Name:        req.Name,
		Address:     address,
		Description: req.Description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		IsDefault:   false,
		IsUnlocked:  false,
	}

	// 保存钱包（加密私钥并存储）
	err = w.saveWalletSecurely(walletInfo, privateKey, req.Password)
	if err != nil {
		return nil, fmt.Errorf("保存钱包失败: %v", err)
	}

	return walletInfo, nil
}

// ImportWallet 从私钥导入钱包
func (w *walletManager) ImportWallet(ctx context.Context, req *ImportWalletRequest) (*WalletInfo, error) {
	w.logger.Info(fmt.Sprintf("开始导入钱包: name=%s", req.Name))

	// 选择导入方式：私钥或助记词
	var privateKey string
	var address string

	if req.PrivateKey != "" {
		// 使用私钥导入
		isValid, addr, validErr := w.ValidatePrivateKey(req.PrivateKey)
		if validErr != nil {
			return nil, fmt.Errorf("私钥验证失败: %v", validErr)
		}
		if !isValid {
			return nil, fmt.Errorf("私钥格式不正确")
		}
		privateKey = req.PrivateKey
		address = addr
	} else if req.Mnemonic != "" {
		// 使用助记词导入（简化实现）
		return nil, fmt.Errorf("助记词导入功能暂未实现")
	} else {
		return nil, fmt.Errorf("必须提供私钥或助记词")
	}

	// 创建钱包信息
	walletInfo := &WalletInfo{
		ID:          generateWalletID(),
		Name:        req.Name,
		Address:     address,
		Description: req.Description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		IsDefault:   false,
		IsUnlocked:  false,
	}

	// 保存钱包（加密私钥并存储）
	err := w.saveWalletSecurely(walletInfo, privateKey, req.Password)
	if err != nil {
		return nil, fmt.Errorf("保存钱包失败: %v", err)
	}

	return walletInfo, nil
}

// ImportWalletFromFile 从文件导入钱包
func (w *walletManager) ImportWalletFromFile(ctx context.Context, name, filePath, password string) (*WalletInfo, error) {
	w.logger.Info(fmt.Sprintf("从文件导入钱包: name=%s, file=%s", name, filePath))

	// 读取文件内容
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取钱包文件失败: %v", err)
	}

	// 尝试解析为加密钱包文件
	var encryptedWallet EncryptedWallet
	if err := json.Unmarshal(data, &encryptedWallet); err == nil {
		// 这是一个加密的钱包文件，需要解密
		privateKey, err := w.decryptPrivateKey(encryptedWallet.EncryptedKey, encryptedWallet.Salt, password)
		if err != nil {
			return nil, fmt.Errorf("解密私钥失败: %v", err)
		}

		req := &ImportWalletRequest{
			Name:        name,
			Password:    password,
			PrivateKey:  privateKey,
			Mnemonic:    "",
			Description: "",
		}
		return w.ImportWallet(ctx, req)
	}

	// 尝试将文件内容作为私钥处理
	privateKey := string(data)
	req := &ImportWalletRequest{
		Name:        name,
		Password:    password,
		PrivateKey:  privateKey,
		Mnemonic:    "",
		Description: "",
	}
	return w.ImportWallet(ctx, req)
}

// createWalletWithKey 使用给定的私钥创建钱包
func (w *walletManager) createWalletWithKey(ctx context.Context, name, privateKey, address, password string) (*WalletInfo, error) {
	// 生成钱包ID
	walletID := w.generateWalletID(name, address)

	// 检查钱包是否已存在
	storage, err := w.loadStorage()
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("加载钱包存储失败: %v", err)
	}
	if storage == nil {
		storage = w.createNewStorage()
	}

	// 检查是否有重复的地址
	for _, wallet := range storage.Wallets {
		if wallet.Address == address {
			return nil, fmt.Errorf("该地址的钱包已存在: %s", address)
		}
	}

	// 加密私钥
	salt := w.generateSalt()
	encryptedKey, err := w.encryptPrivateKey(privateKey, salt, password)
	if err != nil {
		return nil, fmt.Errorf("加密私钥失败: %v", err)
	}

	// 创建加密钱包记录
	now := time.Now().Unix()
	encryptedWallet := &EncryptedWallet{
		ID:             walletID,
		Name:           name,
		Address:        address,
		EncryptedKey:   encryptedKey,
		Salt:           salt,
		CreatedAt:      now,
		UpdatedAt:      now,
		IsDefault:      len(storage.Wallets) == 0, // 第一个钱包设为默认
		KeyDerivation:  "pbkdf2",
		EncryptionType: "aes-256-gcm",
	}

	// 添加到存储
	storage.Wallets = append(storage.Wallets, encryptedWallet)
	storage.UpdatedAt = now

	// 保存到文件
	if err := w.saveStorage(storage); err != nil {
		return nil, fmt.Errorf("保存钱包存储失败: %v", err)
	}

	// 创建钱包信息
	walletInfo := &WalletInfo{
		ID:           walletID,
		Name:         name,
		Address:      address,
		CreatedAt:    time.Unix(now, 0),
		UpdatedAt:    time.Unix(now, 0),
		IsDefault:    encryptedWallet.IsDefault,
		IsUnlocked:   false,
		KeystorePath: w.getStorageFilePath(),
	}

	w.logger.Info(fmt.Sprintf("钱包创建成功: id=%s, address=%s", walletID, address))
	return walletInfo, nil
}

// generateKeyPair 生成密钥对（简化版本，实际应该使用更安全的密钥生成）
func (w *walletManager) generateKeyPair() (privateKey, address string, err error) {
	// 生成32字节随机私钥
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", "", fmt.Errorf("生成随机私钥失败: %v", err)
	}

	privateKeyHex := hex.EncodeToString(key)

	// 使用地址管理器从私钥推导标准地址
	if w.addressManager == nil {
		// 退化处理（不应发生）：保留旧逻辑，避免崩溃
		hash := sha256.Sum256(key)
		address = "Cf" + hex.EncodeToString(hash[:16])
	} else {
		addr, derr := w.addressManager.PrivateKeyToAddress(key)
		if derr != nil {
			return "", "", fmt.Errorf("从私钥推导地址失败: %v", derr)
		}
		address = addr
	}

	return privateKeyHex, address, nil
}

// generateWalletID 生成钱包ID
func (w *walletManager) generateWalletID(name, address string) string {
	data := fmt.Sprintf("%s-%s-%d", name, address, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8]) // 使用前8字节作为ID
}

// generateSalt 生成加密盐值
func (w *walletManager) generateSalt() string {
	salt := make([]byte, 16)
	rand.Read(salt)
	return hex.EncodeToString(salt)
}

// encryptPrivateKey 加密私钥
func (w *walletManager) encryptPrivateKey(privateKey, salt, password string) (string, error) {
	// 使用PBKDF2派生密钥
	saltBytes, _ := hex.DecodeString(salt)
	key := pbkdf2.Key([]byte(password), saltBytes, 10000, 32, sha256.New)

	// 使用AES-256-GCM加密
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// 生成随机nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// 加密
	ciphertext := gcm.Seal(nonce, nonce, []byte(privateKey), nil)
	return hex.EncodeToString(ciphertext), nil
}

// decryptPrivateKey 解密私钥
func (w *walletManager) decryptPrivateKey(encryptedKey, salt, password string) (string, error) {
	// 派生密钥
	saltBytes, _ := hex.DecodeString(salt)
	key := pbkdf2.Key([]byte(password), saltBytes, 10000, 32, sha256.New)

	// 解码密文
	ciphertext, err := hex.DecodeString(encryptedKey)
	if err != nil {
		return "", err
	}

	// 创建解密器
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// 提取nonce
	if len(ciphertext) < gcm.NonceSize() {
		return "", fmt.Errorf("密文长度不足")
	}

	nonce := ciphertext[:gcm.NonceSize()]
	ciphertext = ciphertext[gcm.NonceSize():]

	// 解密
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// getStorageFilePath 获取存储文件路径
func (w *walletManager) getStorageFilePath() string {
	return filepath.Join(w.storageDir, "wallets.json")
}

// createNewStorage 创建新的存储结构
func (w *walletManager) createNewStorage() *WalletStorage {
	now := time.Now().Unix()
	return &WalletStorage{
		Version:   "1.0",
		Wallets:   []*EncryptedWallet{},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// loadStorage 加载钱包存储
func (w *walletManager) loadStorage() (*WalletStorage, error) {
	filePath := w.getStorageFilePath()

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var storage WalletStorage
	if err := json.Unmarshal(data, &storage); err != nil {
		return nil, fmt.Errorf("解析钱包存储文件失败: %v", err)
	}

	return &storage, nil
}

// saveStorage 保存钱包存储
func (w *walletManager) saveStorage(storage *WalletStorage) error {
	// 确保目录存在
	if err := os.MkdirAll(w.storageDir, 0700); err != nil {
		return fmt.Errorf("创建存储目录失败: %v", err)
	}

	filePath := w.getStorageFilePath()

	data, err := json.MarshalIndent(storage, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化钱包存储失败: %v", err)
	}

	// 使用临时文件确保原子性写入
	tempFile := filePath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0600); err != nil {
		return fmt.Errorf("写入临时文件失败: %v", err)
	}

	if err := os.Rename(tempFile, filePath); err != nil {
		os.Remove(tempFile) // 清理临时文件
		return fmt.Errorf("替换存储文件失败: %v", err)
	}

	return nil
}

// generateWalletID 生成钱包ID
func generateWalletID() string {
	return fmt.Sprintf("wallet_%d_%06d", time.Now().Unix(), time.Now().Nanosecond()%1000000)
}

// saveWalletSecurely 安全保存钱包（加密私钥）
func (w *walletManager) saveWalletSecurely(walletInfo *WalletInfo, privateKey, password string) error {
	// 加载现有存储或创建新存储
	storage, err := w.loadStorage()
	if err != nil {
		if os.IsNotExist(err) {
			storage = w.createNewStorage()
		} else {
			return fmt.Errorf("加载钱包存储失败: %v", err)
		}
	}

	// 检查钱包是否已存在
	for _, existing := range storage.Wallets {
		if existing.ID == walletInfo.ID {
			return fmt.Errorf("钱包ID已存在: %s", walletInfo.ID)
		}
		if existing.Name == walletInfo.Name {
			return fmt.Errorf("钱包名称已存在: %s", walletInfo.Name)
		}
	}

	// 生成盐值并加密私钥
	salt := hex.EncodeToString([]byte(fmt.Sprintf("salt_%d", time.Now().UnixNano())))
	encryptedKey, err := w.encryptPrivateKey(privateKey, salt, password)
	if err != nil {
		return fmt.Errorf("加密私钥失败: %v", err)
	}

	// 创建加密钱包记录
	encryptedWallet := &EncryptedWallet{
		ID:             walletInfo.ID,
		Name:           walletInfo.Name,
		Address:        walletInfo.Address,
		EncryptedKey:   encryptedKey,
		Salt:           salt,
		CreatedAt:      walletInfo.CreatedAt.Unix(),
		UpdatedAt:      walletInfo.UpdatedAt.Unix(),
		IsDefault:      walletInfo.IsDefault,
		PasswordHint:   "",
		KeyDerivation:  "pbkdf2",
		EncryptionType: "aes-256-gcm",
	}

	// 如果这是第一个钱包，设为默认钱包
	if len(storage.Wallets) == 0 {
		encryptedWallet.IsDefault = true
		walletInfo.IsDefault = true
	}

	// 添加到存储
	storage.Wallets = append(storage.Wallets, encryptedWallet)
	storage.UpdatedAt = time.Now().Unix()

	// 保存存储
	return w.saveStorage(storage)
}
