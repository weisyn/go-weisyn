package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/ecies"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/crypto/scrypt"

	"github.com/weisyn/v1/internal/core/infrastructure/crypto/hash"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/key"
	cryptointf "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
)

// 错误定义
var (
	ErrInvalidKeyLength  = errors.New("无效的密钥长度")
	ErrInvalidCiphertext = errors.New("无效的密文格式")
	ErrDecryptionFailed  = errors.New("解密失败")
	ErrEmptyData         = errors.New("不能加密空数据")
)

// EncryptionService 提供加密和解密功能
type EncryptionService struct {
	hashService *hash.HashService
	keyManager  *key.KeyManager
}

// NewEncryptionService 创建新的加密服务
func NewEncryptionService(hashService *hash.HashService) *EncryptionService {
	return &EncryptionService{
		hashService: hashService,
		keyManager:  key.NewKeyManager(),
	}
}

// Encrypt 使用公钥加密数据
//
// 支持多种公钥格式：
//   - 33字节压缩公钥（Bitcoin标准）- 自动转换为未压缩格式
//   - 64字节未压缩公钥（无前缀）
//   - 65字节未压缩公钥（带0x04前缀）
func (s *EncryptionService) Encrypt(data []byte, publicKey []byte) ([]byte, error) {
	// 检查数据是否为空
	if len(data) == 0 {
		return nil, ErrEmptyData
	}

	// 智能转换公钥格式以支持ECIES加密
	processedPublicKey, err := s.preparePublicKeyForECIES(publicKey)
	if err != nil {
		return nil, fmt.Errorf("公钥格式处理失败: %w", err)
	}

	// 转换为ECDSA公钥（必须使用 geth 的 secp256k1 曲线实现；否则 ecies 会报 unsupported ECIES parameters）
	ecdsaPubKey, err := gethcrypto.UnmarshalPubkey(processedPublicKey)
	if err != nil {
		return nil, fmt.Errorf("转换为ECDSA公钥失败: %w", err)
	}

	// 转换为ECIES公钥
	eciesPubKey := ecies.ImportECDSAPublic(ecdsaPubKey)

	// 使用ECIES公钥加密
	return ecies.Encrypt(rand.Reader, eciesPubKey, data, nil, nil)
}

// preparePublicKeyForECIES 准备公钥格式以支持ECIES加密
//
// ECIES要求64字节或65字节未压缩公钥，此方法自动处理格式转换：
//   - 33字节压缩公钥 → 65字节未压缩公钥
//   - 64字节公钥 → 65字节公钥（添加前缀）
//   - 65字节公钥 → 直接使用
func (s *EncryptionService) preparePublicKeyForECIES(publicKey []byte) ([]byte, error) {
	switch len(publicKey) {
	case 33:
		// 压缩公钥 → 解压缩为65字节格式
		return s.keyManager.DecompressPublicKey(publicKey)
	case 64:
		// 64字节公钥 → 添加0x04前缀
		processedKey := make([]byte, 65)
		processedKey[0] = 0x04
		copy(processedKey[1:], publicKey)
		return processedKey, nil
	case 65:
		// 65字节公钥 → 验证前缀后直接使用
		if publicKey[0] != 0x04 {
			return nil, fmt.Errorf("无效的65字节公钥前缀: 0x%02x，期望0x04", publicKey[0])
		}
		return publicKey, nil
	default:
		return nil, fmt.Errorf("不支持的公钥长度: %d，期望33、64或65字节", len(publicKey))
	}
}

// Decrypt 使用私钥解密数据
func (s *EncryptionService) Decrypt(encryptedData []byte, privateKey []byte) ([]byte, error) {
	// 转换为ECDSA私钥（使用 geth 的 secp256k1 曲线实现，确保与 ECIES 兼容）
	ecdsaPrivKey, err := gethcrypto.ToECDSA(privateKey)
	if err != nil {
		return nil, err
	}

	// 转换为ECIES私钥
	eciesPrivKey := ecies.ImportECDSA(ecdsaPrivKey)

	// 使用ECIES私钥解密
	return eciesPrivKey.Decrypt(encryptedData, nil, nil)
}

// EncryptWithPassword 使用密码加密数据
func (s *EncryptionService) EncryptWithPassword(data []byte, password string) ([]byte, error) {
	// 生成随机盐
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}

	// 从密码和盐派生AES密钥
	key, err := s.deriveKey(password, salt)
	if err != nil {
		return nil, err
	}

	// 创建AES-GCM加密器
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// 生成随机nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// 加密数据
	ciphertext := gcm.Seal(nil, nonce, data, nil)

	// 拼接盐、nonce和密文
	result := make([]byte, 0, len(salt)+len(nonce)+len(ciphertext))
	result = append(result, salt...)
	result = append(result, nonce...)
	result = append(result, ciphertext...)

	return result, nil
}

// DecryptWithPassword 使用密码解密数据
func (s *EncryptionService) DecryptWithPassword(encryptedData []byte, password string) ([]byte, error) {
	if len(encryptedData) < 32 { // 至少需要盐(16字节) + nonce(12字节) + 4字节数据
		return nil, ErrInvalidCiphertext
	}

	// 提取盐
	salt := encryptedData[:16]

	// 从密码和盐派生AES密钥
	key, err := s.deriveKey(password, salt)
	if err != nil {
		return nil, err
	}

	// 创建AES-GCM解密器
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// 提取nonce
	nonceSize := gcm.NonceSize()
	if len(encryptedData) < 16+nonceSize {
		return nil, ErrInvalidCiphertext
	}

	nonce := encryptedData[16 : 16+nonceSize]
	ciphertext := encryptedData[16+nonceSize:]

	// 解密数据
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// deriveKey 从密码和盐派生密钥
func (s *EncryptionService) deriveKey(password string, salt []byte) ([]byte, error) {
	// 使用scrypt进行密钥派生
	key, err := scrypt.Key([]byte(password), salt, 32768, 8, 1, 32)
	if err != nil {
		// 降级到PBKDF2
		key = pbkdf2.Key([]byte(password), salt, 10000, 32, sha256.New)
	}

	return key, nil
}

// 确保EncryptionService实现了cryptointf.EncryptionManager接口
var _ cryptointf.EncryptionManager = (*EncryptionService)(nil)
