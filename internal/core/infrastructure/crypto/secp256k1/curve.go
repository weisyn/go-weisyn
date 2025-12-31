// Package secp256k1 提供 secp256k1 椭圆曲线封装
//
// 🎯 **设计目的**：
// 封装 btcd/btcec 的 secp256k1 实现，对外提供统一的 secp256k1 曲线接口。
// 通过封装层隔离第三方库依赖，便于未来替换底层实现。
//
// 🔒 **安全原则**：
// - 使用经过验证的密码学库（btcd是Bitcoin Core的Go实现）
// - 所有操作都遵循密码学最佳实践
package secp256k1

import (
	"crypto/elliptic"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
)

// Curve 封装 secp256k1 椭圆曲线
//
// 通过封装 btcd/btcec，提供统一的 secp256k1 曲线接口。
// 未来如果需要替换底层实现，只需修改此封装层。
type Curve struct{}

// NewCurve 创建新的 secp256k1 曲线实例
func NewCurve() *Curve {
	return &Curve{}
}

// S256 返回 secp256k1 椭圆曲线实例
//
// 返回：
//   - elliptic.Curve: secp256k1 曲线实例，可用于 ECDSA 签名
//
// 📝 **使用示例**：
//
//	curve := secp256k1.NewCurve().S256()
//	pubKey := &ecdsa.PublicKey{
//	    Curve: curve,
//	    X:     x,
//	    Y:     y,
//	}
func (c *Curve) S256() elliptic.Curve {
	return btcec.S256()
}

// RecoverPubkey 从签名恢复公钥
//
// 参数：
//   - hash: 消息哈希（32字节）
//   - signature: 65字节签名（r+s+recoveryID）
//
// 返回：
//   - []byte: 压缩公钥（33字节）
//   - error: 恢复失败时的错误
//
// 📝 **使用示例**：
//
//	pubKey, err := curve.RecoverPubkey(msgHash, sig)
//	if err != nil {
//	    return fmt.Errorf("公钥恢复失败: %w", err)
//	}
func (c *Curve) RecoverPubkey(hash, signature []byte) ([]byte, error) {
	// 验证签名长度（65字节：32+32+1）
	if len(signature) != 65 {
		return nil, &ErrInvalidSignatureLength{Expected: 65, Got: len(signature)}
	}

	// 验证哈希长度（32字节）
	if len(hash) != 32 {
		return nil, &ErrInvalidHashLength{Expected: 32, Got: len(hash)}
	}

	// btcd/btcec 的 RecoverCompact 期望“紧凑签名”格式：
	//   sig[0] = header = 27 + recID (+4 表示压缩公钥)
	//   sig[1:33] = r, sig[33:65] = s
	//
	// 本仓库上层更常用的格式是 r(32) + s(32) + recID(1)，即 recID 放在末尾（0-3）。
	// 这里做兼容：两种格式都接受。
	compactSig := signature
	if signature[0] < 27 || signature[0] > 34 {
		// 视为 r+s+recID（recID 在末尾）
		recID := signature[64]
		if recID >= 4 {
			return nil, &ErrRecoverPubkeyFailed{Err: fmt.Errorf("invalid recovery id: %d", recID)}
		}
		compactSig = make([]byte, 65)
		compactSig[0] = 27 + recID + 4 // +4 表示返回压缩公钥
		copy(compactSig[1:], signature[:64])
	}

	// 使用 btcd 的公钥恢复功能
	pubKey, _, err := ecdsa.RecoverCompact(compactSig, hash)
	if err != nil {
		return nil, &ErrRecoverPubkeyFailed{Err: err}
	}

	// 返回压缩公钥（33字节）
	return pubKey.SerializeCompressed(), nil
}

// VerifySignature 验证 secp256k1 签名
//
// 参数：
//   - pubKey: 公钥（33字节压缩或65字节未压缩）
//   - hash: 消息哈希（32字节）
//   - signature: 签名（64字节 r+s 或 65字节 r+s+recoveryID）
//
// 返回：
//   - bool: 签名是否有效
//
// 📝 **使用示例**：
//
//	valid := curve.VerifySignature(pubKey, msgHash, sig)
//	if !valid {
//	    return fmt.Errorf("签名无效")
//	}
func (c *Curve) VerifySignature(pubKey, hash, signature []byte) bool {
	// 验证哈希长度
	if len(hash) != 32 {
		return false
	}

	// 解析公钥
	pubKeyObj, err := btcec.ParsePubKey(pubKey)
	if err != nil {
		return false
	}

	// 处理签名格式：
	// - 64字节：r+s（标准格式）
	// - 65字节：r+s+recoveryID（前64字节是r+s）
	sigBytes := signature
	if len(signature) == 65 {
		sigBytes = signature[:64] // 使用前64字节
	} else if len(signature) != 64 {
		return false
	}

	// 解析签名
	sigObj, err := ecdsa.ParseSignature(sigBytes)
	if err != nil {
		return false
	}

	// 验证签名
	return sigObj.Verify(hash, pubKeyObj)
}

// 错误类型定义

// ErrInvalidSignatureLength 签名长度无效
type ErrInvalidSignatureLength struct {
	Expected int
	Got      int
}

func (e *ErrInvalidSignatureLength) Error() string {
	return fmt.Sprintf("无效的签名长度: 期望 %d 字节，实际 %d 字节", e.Expected, e.Got)
}

// ErrInvalidHashLength 哈希长度无效
type ErrInvalidHashLength struct {
	Expected int
	Got      int
}

func (e *ErrInvalidHashLength) Error() string {
	return fmt.Sprintf("无效的哈希长度: 期望 %d 字节，实际 %d 字节", e.Expected, e.Got)
}

// ErrRecoverPubkeyFailed 公钥恢复失败
type ErrRecoverPubkeyFailed struct {
	Err error
}

func (e *ErrRecoverPubkeyFailed) Error() string {
	return fmt.Sprintf("公钥恢复失败: %v", e.Err)
}

func (e *ErrRecoverPubkeyFailed) Unwrap() error {
	return e.Err
}
