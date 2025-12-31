package wallet

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"golang.org/x/crypto/pbkdf2"
)

// KeystoreV1 Keystore文件格式(v1.0.0)
type KeystoreV1 struct {
	Version string   `json:"version"` // "1.0.0"
	ID      string   `json:"id"`      // UUID
	Address string   `json:"address"` // 0x...
	Crypto  CryptoV1 `json:"crypto"`

	// 元数据
	CreatedAt string `json:"created_at"`
	Label     string `json:"label,omitempty"`
}

// CryptoV1 加密参数
type CryptoV1 struct {
	Cipher       string       `json:"cipher"`     // "aes-256-gcm"
	Ciphertext   string       `json:"ciphertext"` // hex编码
	CipherParams CipherParams `json:"cipherparams"`
	KDF          string       `json:"kdf"` // "pbkdf2" | "argon2id"
	KDFParams    KDFParams    `json:"kdfparams"`
	MAC          string       `json:"mac"` // hex编码的MAC
}

// CipherParams 密码参数
type CipherParams struct {
	IV string `json:"iv"` // hex编码的初始化向量
}

// KDFParams 密钥派生参数
type KDFParams struct {
	// PBKDF2参数
	DKLen int    `json:"dklen"` // 派生密钥长度(32)
	Salt  string `json:"salt"`  // hex编码的盐值
	C     int    `json:"c"`     // 迭代次数
	PRF   string `json:"prf"`   // "hmac-sha256"
}

// KeystoreSigner Keystore签名器实现
type KeystoreSigner struct {
	keystorePath string
	address      string
	privateKey   *ecdsa.PrivateKey // 解锁后的私钥(内存中)
	mu           sync.RWMutex
	unlockUntil  time.Time
}

// NewKeystoreSigner 创建Keystore签名器
func NewKeystoreSigner(keystorePath string, address string) (*KeystoreSigner, error) {
	// 验证文件存在
	if _, err := os.Stat(keystorePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("keystore file not found: %s", keystorePath)
	}

	return &KeystoreSigner{
		keystorePath: keystorePath,
		address:      strings.TrimSpace(address), // ✅ 保持原始大小写（Base58区分大小写）
	}, nil
}

// Sign 签名交易
func (ks *KeystoreSigner) Sign(tx []byte, fromAddr string) ([]byte, error) {
	// 检查地址匹配（保持大小写敏感）
	if strings.TrimSpace(fromAddr) != ks.address {
		return nil, fmt.Errorf("address mismatch: expected %s, got %s", ks.address, fromAddr)
	}

	// 检查是否解锁
	if ks.IsLocked() {
		return nil, fmt.Errorf("keystore is locked, call Unlock first")
	}

	// 计算交易哈希
	hash := sha256.Sum256(tx)

	// 使用ECDSA签名
	return ks.SignHash(hash[:], fromAddr)
}

// SignHash 签名哈希值
func (ks *KeystoreSigner) SignHash(hash []byte, fromAddr string) ([]byte, error) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	if ks.privateKey == nil {
		return nil, fmt.Errorf("keystore is locked")
	}

	// ECDSA签名
	r, s, err := ecdsa.Sign(rand.Reader, ks.privateKey, hash)
	if err != nil {
		return nil, fmt.Errorf("ecdsa sign: %w", err)
	}

	// 序列化签名: r || s (64字节)
	signature := append(r.Bytes(), s.Bytes()...)
	return signature, nil
}

// GetAddress 获取地址
func (ks *KeystoreSigner) GetAddress(derivationPath string) (string, error) {
	// Keystore不支持派生路径
	if derivationPath != "" {
		return "", fmt.Errorf("keystore signer does not support derivation paths")
	}
	return ks.address, nil
}

// ListAddresses 列出地址
func (ks *KeystoreSigner) ListAddresses() ([]string, error) {
	return []string{ks.address}, nil
}

// Unlock 解锁keystore
func (ks *KeystoreSigner) Unlock(password string, duration time.Duration) error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	// 读取keystore文件
	data, err := os.ReadFile(ks.keystorePath)
	if err != nil {
		return fmt.Errorf("read keystore: %w", err)
	}

	// 解析JSON
	var keystoreData KeystoreV1
	if err := json.Unmarshal(data, &keystoreData); err != nil {
		return fmt.Errorf("parse keystore: %w", err)
	}

	// 派生解密密钥
	decryptKey, err := deriveKey(password, keystoreData.Crypto)
	if err != nil {
		return fmt.Errorf("derive key: %w", err)
	}

	// 解密私钥
	privateKeyBytes, err := decrypt(keystoreData.Crypto, decryptKey)
	if err != nil {
		return fmt.Errorf("decrypt: %w", err)
	}

	// 解析私钥(假设私钥存储为十六进制)
	privateKey, err := parsePrivateKey(privateKeyBytes)
	if err != nil {
		return fmt.Errorf("parse private key: %w", err)
	}

	// 保存到内存
	ks.privateKey = privateKey

	// 设置解锁时长
	if duration > 0 {
		ks.unlockUntil = time.Now().Add(duration)

		// 启动自动锁定
		go func() {
			time.Sleep(duration)
			ks.Lock()
		}()
	} else {
		// 永久解锁
		ks.unlockUntil = time.Time{}
	}

	return nil
}

// Lock 锁定keystore
func (ks *KeystoreSigner) Lock() {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	// 清除内存中的私钥
	if ks.privateKey != nil {
		// 覆盖私钥数据(安全清除)
		zeroKey(ks.privateKey)
		ks.privateKey = nil
	}

	ks.unlockUntil = time.Time{}
}

// IsLocked 检查是否锁定
func (ks *KeystoreSigner) IsLocked() bool {
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	if ks.privateKey == nil {
		return true
	}

	// 检查是否过期
	if !ks.unlockUntil.IsZero() && time.Now().After(ks.unlockUntil) {
		return true
	}

	return false
}

// Type 返回签名器类型
func (ks *KeystoreSigner) Type() SignerType {
	return SignerTypeKeystore
}

// 确保实现了Signer接口
var _ Signer = (*KeystoreSigner)(nil)

// ===== Keystore加密/解密辅助函数 =====

// deriveKey 派生解密密钥
func deriveKey(password string, crypto CryptoV1) ([]byte, error) {
	salt, err := hex.DecodeString(crypto.KDFParams.Salt)
	if err != nil {
		return nil, fmt.Errorf("decode salt: %w", err)
	}

	switch crypto.KDF {
	case "pbkdf2":
		// PBKDF2-HMAC-SHA256
		return pbkdf2.Key(
			[]byte(password),
			salt,
			crypto.KDFParams.C,
			crypto.KDFParams.DKLen,
			sha256.New,
		), nil

	default:
		return nil, fmt.Errorf("unsupported KDF: %s", crypto.KDF)
	}
}

// decrypt 解密密文
func decrypt(crypto CryptoV1, key []byte) ([]byte, error) {
	// 解码密文和IV
	ciphertext, err := hex.DecodeString(crypto.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}

	iv, err := hex.DecodeString(crypto.CipherParams.IV)
	if err != nil {
		return nil, fmt.Errorf("decode iv: %w", err)
	}

	switch crypto.Cipher {
	case "aes-256-gcm":
		// AES-256-GCM
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, fmt.Errorf("new cipher: %w", err)
		}

		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, fmt.Errorf("new gcm: %w", err)
		}

		plaintext, err := gcm.Open(nil, iv, ciphertext, nil)
		if err != nil {
			return nil, fmt.Errorf("gcm decrypt: %w (wrong password?)", err)
		}

		return plaintext, nil

	default:
		return nil, fmt.Errorf("unsupported cipher: %s", crypto.Cipher)
	}
}

// encrypt 加密明文
func encrypt(plaintext []byte, password string) (CryptoV1, error) {
	// 生成随机盐值
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return CryptoV1{}, fmt.Errorf("generate salt: %w", err)
	}

	// 派生密钥(PBKDF2)
	key := pbkdf2.Key([]byte(password), salt, 262144, 32, sha256.New)

	// AES-256-GCM加密
	block, err := aes.NewCipher(key)
	if err != nil {
		return CryptoV1{}, fmt.Errorf("new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return CryptoV1{}, fmt.Errorf("new gcm: %w", err)
	}

	// 生成随机IV
	iv := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return CryptoV1{}, fmt.Errorf("generate iv: %w", err)
	}

	// 加密
	ciphertext := gcm.Seal(nil, iv, plaintext, nil)

	// 计算MAC
	mac := sha256.Sum256(append(key[16:], ciphertext...))

	return CryptoV1{
		Cipher:     "aes-256-gcm",
		Ciphertext: hex.EncodeToString(ciphertext),
		CipherParams: CipherParams{
			IV: hex.EncodeToString(iv),
		},
		KDF: "pbkdf2",
		KDFParams: KDFParams{
			DKLen: 32,
			Salt:  hex.EncodeToString(salt),
			C:     262144, // 迭代262144次(推荐值)
			PRF:   "hmac-sha256",
		},
		MAC: hex.EncodeToString(mac[:]),
	}, nil
}

// parsePrivateKey 解析私钥
// 使用 secp256k1 曲线（与 Bitcoin 兼容）
func parsePrivateKey(data []byte) (*ecdsa.PrivateKey, error) {
	// 解析32字节的私钥
	if len(data) != 32 {
		return nil, fmt.Errorf("invalid private key length: expected 32 bytes, got %d", len(data))
	}

	// 使用 btcec 解析私钥（secp256k1 曲线）
	privKey, _ := btcec.PrivKeyFromBytes(data)

	// 转换为标准 ecdsa.PrivateKey
	return privKey.ToECDSA(), nil
}

// zeroKey 安全清除私钥
func zeroKey(key *ecdsa.PrivateKey) {
	if key != nil && key.D != nil {
		key.D.SetInt64(0)
	}
}

// SaveKeystore 保存keystore文件
// 已弃用：使用 AccountManager.CreateAccount 代替
func SaveKeystore(keystoreDir string, address string, privateKey *ecdsa.PrivateKey, password string, label string) (string, error) {
	// 创建目录
	if err := os.MkdirAll(keystoreDir, 0700); err != nil {
		return "", fmt.Errorf("create keystore dir: %w", err)
	}

	// 序列化私钥为32字节
	privateKeyBytes := paddedBigBytes(privateKey.D, 32)

	// 加密私钥
	crypto, err := encrypt(privateKeyBytes, password)
	if err != nil {
		return "", fmt.Errorf("encrypt: %w", err)
	}

	// 生成UUID
	id := generateUUID()

	// 构建keystore数据
	keystore := KeystoreV1{
		Version:   "1.0.0",
		ID:        id,
		Address:   address,
		Crypto:    crypto,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Label:     label,
	}

	// 序列化JSON
	data, err := json.MarshalIndent(keystore, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal json: %w", err)
	}

	// 保存文件: UTC--<timestamp>--<address>
	filename := fmt.Sprintf("UTC--%s--%s",
		time.Now().UTC().Format("2006-01-02T15-04-05.000000000Z"),
		strings.TrimPrefix(strings.ToLower(address), "0x"),
	)
	filePath := filepath.Join(keystoreDir, filename)

	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return "", fmt.Errorf("write keystore: %w", err)
	}

	return filePath, nil
}

// generateUUID 生成UUID(简化版)
func generateUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// 如果随机数生成失败，使用时间戳作为后备
		return fmt.Sprintf("%x-%x-%x-%x-%x", time.Now().Unix(), b[4:6], b[6:8], b[8:10], b[10:])
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// paddedBigBytes 将big.Int转换为固定长度的字节数组
func paddedBigBytes(bigInt *big.Int, length int) []byte {
	bytes := bigInt.Bytes()
	if len(bytes) >= length {
		return bytes[len(bytes)-length:]
	}

	padded := make([]byte, length)
	copy(padded[length-len(bytes):], bytes)
	return padded
}
