package signature

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"

	"github.com/btcsuite/btcd/btcec/v2"
	btcec_ecdsa "github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/key"
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/secp256k1"
	cryptointf "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
)

// 确保SignatureService实现了cryptointf.SignatureManager接口
var _ cryptointf.SignatureManager = (*SignatureService)(nil)

// 错误定义
var (
	ErrInvalidSignature       = errors.New("无效的签名")
	ErrInvalidKeyLength       = errors.New("无效的密钥长度")
	ErrInvalidRecoveryID      = errors.New("无效的恢复ID")
	ErrSignatureBatchMismatch = errors.New("签名和数据数量不匹配")
	ErrInvalidHashLength      = errors.New("无效的哈希长度")
	ErrInvalidSignatureFormat = errors.New("无效的签名格式")
	ErrInvalidPublicKey       = errors.New("无效的公钥")
)

// WES签名系统常量
const (
	// 签名组件长度
	SignatureLength            = 64 // r+s (标准)
	RecoverableSignatureLength = 65 // r+s+v (可恢复签名)
	HashLength                 = 32 // SHA256哈希长度

	//WES消息签名前缀
	WESMessagePrefix = "\x18 Signed Message:\n"
)

// SignatureService 提供原生的数字签名功能
//
// 🎯 **设计原则**：
// - 使用Go标准库实现ECDSA签名
// - 使用secp256k1椭圆曲线（通过btcd封装层获取）
// - 双SHA256哈希（标准）
// - 自己实现签名规范化和恢复算法
// - 使用通用密码学库，避免区块链特定依赖
type SignatureService struct {
	keyManager     *key.KeyManager
	addressManager cryptointf.AddressManager
	secp256k1Curve *secp256k1.Curve // secp256k1曲线封装
}

// NewSignatureService 创建新的签名服务
func NewSignatureService(keyManager *key.KeyManager, addressManager cryptointf.AddressManager) *SignatureService {
	return &SignatureService{
		keyManager:     keyManager,
		addressManager: addressManager,
		secp256k1Curve: secp256k1.NewCurve(), // 初始化secp256k1曲线
	}
}

// SignTransaction 签名交易数据（标准实现）
func (ss *SignatureService) SignTransaction(txHash []byte, privateKey []byte, sigHashType cryptointf.SignatureHashType) ([]byte, error) {
	if len(txHash) != HashLength {
		return nil, ErrInvalidHashLength
	}
	if len(privateKey) != 32 {
		return nil, ErrInvalidKeyLength
	}

	// 使用Go标准库ECDSA签名
	signature, err := ss.signECDSA(txHash, privateKey)
	if err != nil {
		return nil, fmt.Errorf("交易签名失败: %w", err)
	}

	//WES标准：规范化签名（确保低S值）
	return ss.NormalizeSignature(signature)
}

// VerifyTransactionSignature 验证交易签名
func (ss *SignatureService) VerifyTransactionSignature(txHash []byte, signature []byte, publicKey []byte, sigHashType cryptointf.SignatureHashType) bool {
	if len(txHash) != HashLength || len(signature) != SignatureLength {
		return false
	}

	return ss.verifyECDSA(txHash, signature, publicKey)
}

// Sign 签名任意数据
func (ss *SignatureService) Sign(data []byte, privateKey []byte) ([]byte, error) {
	if len(privateKey) != 32 {
		return nil, ErrInvalidKeyLength
	}

	//WES标准：双SHA256哈希
	hash := ss.doubleSHA256(data)

	// 使用Go标准库签名
	signature, err := ss.signECDSA(hash, privateKey)
	if err != nil {
		return nil, err
	}

	return ss.NormalizeSignature(signature)
}

// Verify 验证数据签名
func (ss *SignatureService) Verify(data, signature, publicKey []byte) bool {
	if len(signature) != SignatureLength {
		return false
	}

	//WES标准：双SHA256哈希
	hash := ss.doubleSHA256(data)

	return ss.verifyECDSA(hash, signature, publicKey)
}

// SignMessage 签名消息（带前缀）
func (ss *SignatureService) SignMessage(message []byte, privateKey []byte) ([]byte, error) {
	if len(privateKey) != 32 {
		return nil, ErrInvalidKeyLength
	}

	// 构建风格的消息前缀
	prefixedMessage := ss.buildPrefixedMessage(message)

	//WES标准：双SHA256哈希
	hash := ss.doubleSHA256(prefixedMessage)

	// 签名并生成可恢复签名
	recoverableSig, err := ss.signRecoverable(hash, privateKey)
	if err != nil {
		return nil, err
	}

	return recoverableSig, nil
}

// VerifyMessage 验证消息签名
func (ss *SignatureService) VerifyMessage(message []byte, signature []byte, publicKey []byte) bool {
	if len(signature) != RecoverableSignatureLength {
		return false
	}

	// 构建带前缀的消息
	prefixedMessage := ss.buildPrefixedMessage(message)
	hash := ss.doubleSHA256(prefixedMessage)

	// 使用前64字节验证签名
	return ss.verifyECDSA(hash, signature[0:64], publicKey)
}

// RecoverPublicKey 从签名恢复公钥（自己实现）
func (ss *SignatureService) RecoverPublicKey(hash []byte, signature []byte) ([]byte, error) {
	if len(hash) != HashLength {
		return nil, ErrInvalidHashLength
	}
	if len(signature) != RecoverableSignatureLength {
		return nil, fmt.Errorf("可恢复签名长度错误: %d, 期望%d字节", len(signature), RecoverableSignatureLength)
	}

	// 提取恢复ID
	recoveryID := signature[64]
	if recoveryID >= 4 {
		return nil, ErrInvalidRecoveryID
	}

	// 公钥恢复（返回压缩公钥 33 字节）
	publicKey, err := ss.recoverPublicKeyFromSignature(hash, signature[0:64], recoveryID)
	if err != nil {
		return nil, fmt.Errorf("公钥恢复失败: %w", err)
	}

	return publicKey, nil
}

// RecoverAddress 从签名恢复地址
func (ss *SignatureService) RecoverAddress(hash []byte, signature []byte) (string, error) {
	publicKey, err := ss.RecoverPublicKey(hash, signature)
	if err != nil {
		return "", fmt.Errorf("地址恢复失败: %w", err)
	}

	address, err := ss.addressManager.PublicKeyToAddress(publicKey)
	if err != nil {
		return "", fmt.Errorf("公钥转地址失败: %w", err)
	}

	return address, nil
}

// SignBatch 批量签名
func (ss *SignatureService) SignBatch(dataList [][]byte, privateKey []byte) ([][]byte, error) {
	if len(privateKey) != 32 {
		return nil, ErrInvalidKeyLength
	}

	signatures := make([][]byte, len(dataList))
	for i, data := range dataList {
		sig, err := ss.Sign(data, privateKey)
		if err != nil {
			return nil, fmt.Errorf("批量签名失败 [%d]: %w", i, err)
		}
		signatures[i] = sig
	}

	return signatures, nil
}

// VerifyBatch 批量验证签名
func (ss *SignatureService) VerifyBatch(dataList [][]byte, signatureList [][]byte, publicKeyList [][]byte) ([]bool, error) {
	if len(dataList) != len(signatureList) || len(dataList) != len(publicKeyList) {
		return nil, ErrSignatureBatchMismatch
	}

	results := make([]bool, len(dataList))
	for i := range dataList {
		results[i] = ss.Verify(dataList[i], signatureList[i], publicKeyList[i])
	}

	return results, nil
}

// NormalizeSignature 规范化签名（标准：确保低S值）
func (ss *SignatureService) NormalizeSignature(signature []byte) ([]byte, error) {
	if len(signature) != SignatureLength {
		return nil, ErrInvalidSignatureFormat
	}

	// 提取r和s
	r := new(big.Int).SetBytes(signature[0:32])
	s := new(big.Int).SetBytes(signature[32:64])

	// 获取secp256k1曲线参数
	curve := ss.secp256k1Curve.S256()
	halfOrder := new(big.Int).Div(curve.Params().N, big.NewInt(2))

	//WES标准：如果s > N/2，则使用 s = N - s
	if s.Cmp(halfOrder) > 0 {
		s.Sub(curve.Params().N, s)
	}

	// 重新构建规范化签名
	normalizedSig := make([]byte, SignatureLength)
	r.FillBytes(normalizedSig[0:32])
	s.FillBytes(normalizedSig[32:64])

	return normalizedSig, nil
}

// ValidateSignature 验证签名格式（标准）
func (ss *SignatureService) ValidateSignature(signature []byte) error {
	if len(signature) != SignatureLength && len(signature) != RecoverableSignatureLength {
		return fmt.Errorf("签名长度错误: %d, 期望%d或%d字节", len(signature), SignatureLength, RecoverableSignatureLength)
	}

	// 验证r和s的范围
	r := new(big.Int).SetBytes(signature[0:32])
	s := new(big.Int).SetBytes(signature[32:64])

	curve := ss.secp256k1Curve.S256()

	// r不能为0且小于曲线阶数
	if r.Cmp(big.NewInt(0)) == 0 || r.Cmp(curve.Params().N) >= 0 {
		return fmt.Errorf("签名r值无效")
	}

	// s不能为0且小于曲线阶数
	if s.Cmp(big.NewInt(0)) == 0 || s.Cmp(curve.Params().N) >= 0 {
		return fmt.Errorf("签名s值无效")
	}

	//WES标准：检查是否为低S值
	halfOrder := new(big.Int).Div(curve.Params().N, big.NewInt(2))
	if s.Cmp(halfOrder) > 0 {
		return fmt.Errorf("签名s值过高，违反低S值标准")
	}

	return nil
}

// ================================================================================
// 🔧 内部实现方法 -WES自定义签名算法
// ================================================================================

// doubleSHA256WES标准：双SHA256哈希
func (ss *SignatureService) doubleSHA256(data []byte) []byte {
	first := sha256.Sum256(data)
	second := sha256.Sum256(first[:])
	return second[:]
}

// buildPrefixedMessage 构建风格的带前缀消息
func (ss *SignatureService) buildPrefixedMessage(message []byte) []byte {
	prefix := []byte(WESMessagePrefix)
	lengthBytes := []byte{byte(len(message))}

	result := make([]byte, 0, len(prefix)+len(lengthBytes)+len(message))
	result = append(result, prefix...)
	result = append(result, lengthBytes...)
	result = append(result, message...)

	return result
}

// signECDSAWES核心签名算法（使用Go标准库）
func (ss *SignatureService) signECDSA(hash []byte, privateKey []byte) ([]byte, error) {
	// 使用secp256k1曲线
	curve := ss.secp256k1Curve.S256()

	// 创建私钥对象
	privKey := new(big.Int).SetBytes(privateKey)

	// 创建ECDSA私钥
	ecdsaPrivKey := &ecdsa.PrivateKey{
		D: privKey,
		PublicKey: ecdsa.PublicKey{
			Curve: curve,
		},
	}

	// 计算公钥点
	ecdsaPrivKey.X, ecdsaPrivKey.Y = curve.ScalarBaseMult(privKey.Bytes())

	// 使用Go标准库签名
	r, s, err := ecdsa.Sign(rand.Reader, ecdsaPrivKey, hash)
	if err != nil {
		return nil, err
	}

	// 构建64字节签名
	signature := make([]byte, SignatureLength)
	r.FillBytes(signature[0:32])
	s.FillBytes(signature[32:64])

	return signature, nil
}

// verifyECDSAWES核心验证算法（使用Go标准库）
func (ss *SignatureService) verifyECDSA(hash []byte, signature []byte, publicKey []byte) bool {
	// 解析签名
	r := new(big.Int).SetBytes(signature[0:32])
	s := new(big.Int).SetBytes(signature[32:64])

	// 解析公钥
	curve := ss.secp256k1Curve.S256()
	var x, y *big.Int

	switch len(publicKey) {
	case 33:
		// 压缩公钥，需要解压缩
		uncompressed, err := ss.keyManager.DecompressPublicKey(publicKey)
		if err != nil {
			return false
		}
		x = new(big.Int).SetBytes(uncompressed[1:33])
		y = new(big.Int).SetBytes(uncompressed[33:65])
	case 65:
		// 未压缩公钥
		if publicKey[0] != 0x04 {
			return false
		}
		x = new(big.Int).SetBytes(publicKey[1:33])
		y = new(big.Int).SetBytes(publicKey[33:65])
	case 64:
		// 64字节格式（无前缀）
		x = new(big.Int).SetBytes(publicKey[0:32])
		y = new(big.Int).SetBytes(publicKey[32:64])
	default:
		return false
	}

	// 创建ECDSA公钥
	ecdsaPubKey := &ecdsa.PublicKey{
		Curve: curve,
		X:     x,
		Y:     y,
	}

	// 使用Go标准库验证签名
	return ecdsa.Verify(ecdsaPubKey, hash, r, s)
}

// signRecoverableWES可恢复签名实现
//
// 生成包含恢复ID的65字节签名，确保能够正确恢复公钥
func (ss *SignatureService) signRecoverable(hash []byte, privateKey []byte) ([]byte, error) {
	// 使用 btcec 直接生成 compact signature（包含 recovery id），避免自行猜测 recoveryID 导致不稳定/失败
	priv, _ := btcec.PrivKeyFromBytes(privateKey)
	compact := btcec_ecdsa.SignCompact(priv, hash, true) // header + r + s
	if len(compact) != 65 {
		return nil, fmt.Errorf("生成可恢复签名失败: compact 签名长度异常=%d", len(compact))
	}

	// compact[0] = 27 + recID (+4 表示压缩)
	recID := (compact[0] - 27) & 0x03
	if recID >= 4 {
		return nil, fmt.Errorf("生成可恢复签名失败: recovery id 无效=%d", recID)
	}

	// 转换为本仓库约定的 r(32)+s(32)+recID(1) 格式
	out := make([]byte, 65)
	copy(out[:64], compact[1:])
	out[64] = recID

	// 防御性自检：确保可恢复签名确实能恢复到当前私钥对应的公钥
	expectedPublicKey, err := ss.keyManager.DerivePublicKey(privateKey) // 33字节压缩
	if err != nil {
		return nil, fmt.Errorf("推导公钥失败: %w", err)
	}
	recovered, err := ss.secp256k1Curve.RecoverPubkey(hash, out)
	if err != nil {
		return nil, fmt.Errorf("可恢复签名自检失败: %w", err)
	}
	if !bytes.Equal(expectedPublicKey, recovered) {
		return nil, fmt.Errorf("可恢复签名自检失败: recovered pubkey mismatch")
	}

	return out, nil
}

// recoverPublicKeyFromSignatureWES公钥恢复算法
//
// 使用ECDSA签名恢复公钥，支持标准的secp256k1恢复算法
//
// 参数：
//   - hash: 32字节消息哈希
//   - signature: 64字节ECDSA签名 (r+s)
//   - recoveryID: 恢复ID (0-3)
//
// 返回：
//   - []byte: 恢复的公钥（65字节未压缩格式）
//   - error: 恢复失败时的错误
func (ss *SignatureService) recoverPublicKeyFromSignature(hash []byte, signature []byte, recoveryID byte) ([]byte, error) {
	if len(hash) != 32 {
		return nil, fmt.Errorf("哈希长度错误: %d, 期望32字节", len(hash))
	}
	if len(signature) != 64 {
		return nil, fmt.Errorf("签名长度错误: %d, 期望64字节", len(signature))
	}
	if recoveryID >= 4 {
		return nil, fmt.Errorf("恢复ID无效: %d, 期望0-3", recoveryID)
	}

	// 构建65字节可恢复签名格式 (r+s+recoveryID)
	recoverableSig := make([]byte, 65)
	copy(recoverableSig[0:64], signature)
	recoverableSig[64] = recoveryID

	// 使用secp256k1库恢复压缩公钥（33字节）
	recoveredPubKey, err := ss.secp256k1Curve.RecoverPubkey(hash, recoverableSig)
	if err != nil {
		return nil, fmt.Errorf("secp256k1公钥恢复失败: %w", err)
	}

	if len(recoveredPubKey) != 33 {
		return nil, fmt.Errorf("恢复的公钥长度异常: %d, 期望33字节(压缩)", len(recoveredPubKey))
	}

	// ParsePubKey 会进行曲线/格式校验
	if _, err := btcec.ParsePubKey(recoveredPubKey); err != nil {
		return nil, fmt.Errorf("恢复的公钥不合法: %w", err)
	}

	return recoveredPubKey, nil
}

// comparePublicKeys 比较两个公钥是否相同
//
// 统一转换为压缩格式进行比较，确保格式一致性
func (ss *SignatureService) comparePublicKeys(pubKey1, pubKey2 []byte) bool {
	// 统一转换为压缩格式进行比较
	compressed1 := ss.normalizeToCompressed(pubKey1)
	compressed2 := ss.normalizeToCompressed(pubKey2)

	if len(compressed1) != len(compressed2) {
		return false
	}

	for i := range compressed1 {
		if compressed1[i] != compressed2[i] {
			return false
		}
	}

	return true
}

// normalizeToCompressed 将公钥标准化为压缩格式
func (ss *SignatureService) normalizeToCompressed(publicKey []byte) []byte {
	switch len(publicKey) {
	case 33:
		// 已经是压缩公钥
		return publicKey
	case 65:
		// 未压缩公钥，转换为压缩格式
		compressed, err := ss.keyManager.CompressPublicKey(publicKey)
		if err != nil {
			return publicKey // 出错时返回原始值
		}
		return compressed
	case 64:
		// 64字节格式，先添加前缀再压缩
		uncompressed := make([]byte, 65)
		uncompressed[0] = 0x04
		copy(uncompressed[1:], publicKey)
		compressed, err := ss.keyManager.CompressPublicKey(uncompressed)
		if err != nil {
			return publicKey // 出错时返回原始值
		}
		return compressed
	default:
		return publicKey
	}
}
