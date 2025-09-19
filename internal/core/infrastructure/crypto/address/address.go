package address

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	cryptointf "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/btcsuite/btcutil/base58"
	"golang.org/x/crypto/ripemd160"
)

// WES地址系统配置常量
const (
	// WESP2PKHVersionWES P2PKH地址版本字节
	WESP2PKHVersion = 0x1C
	// WESP2SHVersionWES P2SH地址版本字节（多重签名）
	WESP2SHVersion = 0x9C
	// AddressHashLength 地址哈希长度（20字节）
	AddressHashLength = 20
	// CompressedPublicKeyLength 压缩公钥长度（33字节）
	CompressedPublicKeyLength = 33
	// UncompressedPublicKeyLength 未压缩公钥长度（64字节）
	UncompressedPublicKeyLength = 64
)

var (
	// ErrInvalidPublicKey 无效的公钥
	ErrInvalidPublicKey = errors.New("invalid public key format")
	// ErrInvalidAddress 无效的地址格式
	ErrInvalidAddress = errors.New("invalid address format")
	// ErrInvalidAddressLength 无效的地址长度
	ErrInvalidAddressLength = errors.New("invalid address length")
	// ErrInvalidVersion 无效的版本字节
	ErrInvalidVersion = errors.New("invalid address version")
	// ErrInvalidChecksum 校验和错误
	ErrInvalidChecksum = errors.New("invalid checksum")
)

// AddressServiceWES区块链地址管理服务
//
// 专注于Bitcoin风格的地址生成和管理：
// - 使用secp256k1椭圆曲线
// - SHA256+RIPEMD160哈希算法
// - Base58Check编码
// -WES专用版本字节
type AddressService struct {
	// KeyManager用于私钥到公钥的转换
	keyManager cryptointf.KeyManager
}

// 确保AddressService实现了AddressManager接口
var _ cryptointf.AddressManager = (*AddressService)(nil)

// NewAddressService 创建新的地址服务实例
//
// 参数：
//   - keyManager: 密钥管理器，用于私钥到公钥的转换（可为nil，此时PrivateKeyToAddress方法不可用）
//
// 返回：
//   - *AddressService: 地址服务实例
//
// 注意：
//   - 如果keyManager为nil，则PrivateKeyToAddress方法将返回错误
//   - 其他方法（PublicKeyToAddress、StringToAddress等）正常工作
func NewAddressService(keyManager cryptointf.KeyManager) *AddressService {
	return &AddressService{
		keyManager: keyManager,
	}
}

// PrivateKeyToAddress 从私钥直接生成标准地址
//
// 无状态设计的核心方法，实现完整的私钥到地址推导流程：
// 私钥(32字节) → 公钥(secp256k1) → SHA256 → RIPEMD160 → Base58Check → 标准地址
//
// 推导算法：
//   1. 使用secp256k1椭圆曲线从私钥导出压缩公钥
//   2. 对公钥进行Hash160运算（SHA256 + RIPEMD160）
//   3. 添加WES版本字节和校验和
//   4. 进行Base58Check编码生成最终地址
//
// 参数：
//   - privateKey: 32字节secp256k1私钥
//
// 返回：
//   - string: 标准地址 (Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn)
//   - error: 私钥无效或生成失败
func (s *AddressService) PrivateKeyToAddress(privateKey []byte) (string, error) {
	// 检查KeyManager是否可用
	if s.keyManager == nil {
		return "", fmt.Errorf("私钥转地址功能不可用：未提供KeyManager依赖")
	}

	// 验证私钥有效性
	if err := s.keyManager.ValidatePrivateKey(privateKey); err != nil {
		return "", fmt.Errorf("私钥验证失败: %w", err)
	}

	// 从私钥导出公钥
	publicKey, err := s.keyManager.DerivePublicKey(privateKey)
	if err != nil {
		return "", fmt.Errorf("从私钥导出公钥失败: %w", err)
	}

	// 使用现有的PublicKeyToAddress方法生成地址
	address, err := s.PublicKeyToAddress(publicKey)
	if err != nil {
		return "", fmt.Errorf("从公钥生成地址失败: %w", err)
	}

	return address, nil
}

// PublicKeyToAddress 从公钥生成标准地址
//
// 实现Bitcoin风格的地址推导算法：
// 公钥 → SHA256 → RIPEMD160 → 版本字节+校验和 → Base58编码
func (s *AddressService) PublicKeyToAddress(publicKey []byte) (string, error) {
	// 验证公钥长度
	if len(publicKey) != CompressedPublicKeyLength && len(publicKey) != UncompressedPublicKeyLength {
		return "", fmt.Errorf("%w: expected %d or %d bytes, got %d",
			ErrInvalidPublicKey, CompressedPublicKeyLength, UncompressedPublicKeyLength, len(publicKey))
	}

	// 执行Hash160：SHA256 + RIPEMD160
	addressHash := hash160(publicKey)

	// 使用Base58Check编码生成最终地址
	address := base58CheckEncode(addressHash, WESP2PKHVersion)

	return address, nil
}

// hash160 执行Bitcoin风格的Hash160操作：RIPEMD160(SHA256(data))
func hash160(data []byte) []byte {
	// 第一步：SHA256哈希
	sha256Hash := sha256.Sum256(data)

	// 第二步：RIPEMD160哈希
	ripemd160Hasher := ripemd160.New()
	ripemd160Hasher.Write(sha256Hash[:])

	return ripemd160Hasher.Sum(nil)
}

// base58CheckEncode 使用版本字节和校验和编码数据（Base58Check）
func base58CheckEncode(data []byte, version byte) string {
	// 构建载荷：版本字节 + 数据
	payload := make([]byte, 1+len(data))
	payload[0] = version
	copy(payload[1:], data)

	// 计算校验和：双SHA256的前4字节
	checksum := doubleSHA256(payload)[:4]

	// 构建完整数据：载荷 + 校验和
	fullData := make([]byte, len(payload)+4)
	copy(fullData, payload)
	copy(fullData[len(payload):], checksum)

	return base58.Encode(fullData)
}

// base58CheckDecode 解码Base58Check编码的数据
func base58CheckDecode(encoded string) ([]byte, byte, error) {
	decoded := base58.Decode(encoded)
	if len(decoded) < 5 {
		return nil, 0, ErrInvalidAddressLength
	}

	// 分离载荷和校验和
	payloadLen := len(decoded) - 4
	payload := decoded[:payloadLen]
	checksum := decoded[payloadLen:]

	// 验证校验和
	expectedChecksum := doubleSHA256(payload)[:4]
	for i := 0; i < 4; i++ {
		if checksum[i] != expectedChecksum[i] {
			return nil, 0, ErrInvalidChecksum
		}
	}

	// 返回数据（不含版本字节）和版本字节
	if len(payload) == 0 {
		return nil, 0, ErrInvalidAddressLength
	}

	return payload[1:], payload[0], nil
}

// doubleSHA256 执行双SHA256哈希
func doubleSHA256(data []byte) []byte {
	first := sha256.Sum256(data)
	second := sha256.Sum256(first[:])
	return second[:]
}

// StringToAddress 解析字符串为标准地址格式
func (s *AddressService) StringToAddress(addressStr string) (string, error) {
	if addressStr == "" {
		return "", ErrInvalidAddress
	}

	// 只接受 Bitcoin风格地址
	valid, err := s.ValidateAddress(addressStr)
	if !valid || err != nil {
		return "", fmt.Errorf("invalidWES address: %w", err)
	}

	return addressStr, nil
}

// ValidateAddress 验证地址格式和校验和
func (s *AddressService) ValidateAddress(address string) (bool, error) {
	if address == "" {
		return false, ErrInvalidAddress
	}

	// 只验证Bitcoin风格的Base58Check地址
	return s.validateWESAddress(address)
}

// validateWESAddress 验证 Bitcoin风格地址
func (s *AddressService) validateWESAddress(address string) (bool, error) {
	// 检查Base58字符
	if !isValidBase58(address) {
		return false, ErrInvalidAddress
	}

	// 检查长度范围
	if len(address) < 25 || len(address) > 34 {
		return false, ErrInvalidAddressLength
	}

	// Base58Check解码
	data, version, err := base58CheckDecode(address)
	if err != nil {
		return false, fmt.Errorf("base58check decode failed: %w", err)
	}

	// 验证版本字节
	if version != WESP2PKHVersion && version != WESP2SHVersion {
		return false, fmt.Errorf("%w: got 0x%02x", ErrInvalidVersion, version)
	}

	// 验证数据长度
	if len(data) != AddressHashLength {
		return false, fmt.Errorf("%w: got %d bytes", ErrInvalidAddressLength, len(data))
	}

	return true, nil
}

// AddressToBytes 将地址转换为原始字节数组
func (s *AddressService) AddressToBytes(address string) ([]byte, error) {
	// 只处理 Bitcoin风格地址
	data, _, err := base58CheckDecode(address)
	if err != nil {
		return nil, fmt.Errorf("invalidWES address: %w", err)
	}
	return data, nil
}

// BytesToAddress 将字节数组转换为标准地址
func (s *AddressService) BytesToAddress(addressBytes []byte) (string, error) {
	if len(addressBytes) != AddressHashLength {
		return "", fmt.Errorf("%w: expected %d bytes, got %d",
			ErrInvalidAddressLength, AddressHashLength, len(addressBytes))
	}

	// 使用Base58Check编码生成地址
	address := base58CheckEncode(addressBytes, WESP2PKHVersion)
	return address, nil
}

// AddressToHexString 将地址转换为十六进制字符串（调试用）
func (s *AddressService) AddressToHexString(address string) (string, error) {
	// 获取原始字节
	bytes, err := s.AddressToBytes(address)
	if err != nil {
		return "", err
	}

	// 转换为十六进制（仅用于调试）
	return fmt.Sprintf("%x", bytes), nil
}

// HexStringToAddress 将十六进制字符串转换为地址
func (s *AddressService) HexStringToAddress(hexStr string) (string, error) {
	// 解析十六进制字符串
	if len(hexStr) != 40 {
		return "", fmt.Errorf("invalid hex length: expected 40 chars, got %d", len(hexStr))
	}

	bytes := make([]byte, 20)
	for i := 0; i < 20; i++ {
		high := hexToByte(hexStr[i*2])
		low := hexToByte(hexStr[i*2+1])
		if high == 255 || low == 255 {
			return "", ErrInvalidAddress
		}
		bytes[i] = (high << 4) | low
	}

	// 转换为标准地址
	return s.BytesToAddress(bytes)
}

// GetAddressType 获取地址类型（仅返回Bitcoin风格）
func (s *AddressService) GetAddressType(address string) (cryptointf.AddressType, error) {
	if address == "" {
		return cryptointf.AddressTypeInvalid, ErrInvalidAddress
	}

	// 只检查 Bitcoin风格地址
	valid, err := s.validateWESAddress(address)
	if valid && err == nil {
		return cryptointf.AddressTypeBitcoin, nil
	}

	return cryptointf.AddressTypeInvalid, ErrInvalidAddress
}

// isValidBase58 检查字符串是否为有效的Base58编码
func isValidBase58(s string) bool {
	for _, char := range s {
		if !strings.ContainsRune("123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz", char) {
			return false
		}
	}
	return true
}

// CompareAddresses 比较两个地址是否相等
func (s *AddressService) CompareAddresses(addr1, addr2 string) (bool, error) {
	// 转换为字节数组进行比较
	bytes1, err := s.AddressToBytes(addr1)
	if err != nil {
		return false, fmt.Errorf("invalid address1: %w", err)
	}

	bytes2, err := s.AddressToBytes(addr2)
	if err != nil {
		return false, fmt.Errorf("invalid address2: %w", err)
	}

	// 比较字节数组
	if len(bytes1) != len(bytes2) {
		return false, nil
	}

	for i := range bytes1 {
		if bytes1[i] != bytes2[i] {
			return false, nil
		}
	}

	return true, nil
}

// IsZeroAddress 检查是否为零地址
func (s *AddressService) IsZeroAddress(address string) bool {
	bytes, err := s.AddressToBytes(address)
	if err != nil {
		return false
	}

	// 检查是否所有字节都为0
	for _, b := range bytes {
		if b != 0 {
			return false
		}
	}

	return true
}

// hexToByte 将十六进制字符转换为字节值
func hexToByte(c byte) byte {
	switch {
	case '0' <= c && c <= '9':
		return c - '0'
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10
	default:
		return 255 // 无效字符
	}
}
