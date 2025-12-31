// Package threshold 提供门限签名验证实现
//
// bls.go: BLS 门限签名验证器
//
// 🎯 **核心职责**：验证 BLS（Boneh-Lynn-Shacham）门限签名
//
// 💡 **设计理念**：
// - 使用 gnark-crypto 库实现 BLS 签名验证
// - 支持门限签名聚合和验证
// - 兼容多种 BLS 曲线（BLS12-381）
package threshold

import (
	"fmt"

	bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
)

// BLSThresholdVerifier BLS 门限签名验证器
//
// 🎯 **核心功能**：验证 BLS 门限签名
type BLSThresholdVerifier struct{}

// NewBLSThresholdVerifier 创建 BLS 门限签名验证器
func NewBLSThresholdVerifier() *BLSThresholdVerifier {
	return &BLSThresholdVerifier{}
}

// VerifyThresholdSignature 验证组合门限签名
//
// 实现 crypto.ThresholdSignatureVerifier 接口
func (v *BLSThresholdVerifier) VerifyThresholdSignature(
	dataHash []byte,
	combinedSignature []byte,
	shares []*transaction.ThresholdProof_ThresholdSignatureShare,
	groupPublicKey []byte,
	threshold uint32,
	totalParties uint32,
	scheme string,
) (bool, error) {
	if scheme != "BLS_THRESHOLD" {
		return false, fmt.Errorf("不支持的签名方案: %s（仅支持 BLS_THRESHOLD）", scheme)
	}

	// BLS12-381 长度检查：
	// - G1 公钥：48 字节（压缩）或 96 字节（未压缩）
	// - G2 签名：96 字节（压缩）或 192 字节（未压缩）
	if len(combinedSignature) != bls12381.SizeOfG2AffineCompressed && len(combinedSignature) != bls12381.SizeOfG2AffineUncompressed {
		return false, fmt.Errorf("无效的BLS签名长度: %d（期望 %d 或 %d 字节）",
			len(combinedSignature), bls12381.SizeOfG2AffineCompressed, bls12381.SizeOfG2AffineUncompressed)
	}

	if len(groupPublicKey) != bls12381.SizeOfG1AffineCompressed && len(groupPublicKey) != bls12381.SizeOfG1AffineUncompressed {
		return false, fmt.Errorf("无效的BLS公钥长度: %d（期望 %d 或 %d 字节）",
			len(groupPublicKey), bls12381.SizeOfG1AffineCompressed, bls12381.SizeOfG1AffineUncompressed)
	}

	if len(shares) < int(threshold) {
		return false, fmt.Errorf("签名份额不足: %d < %d", len(shares), threshold)
	}

	// 1. 解析组合签名（G2，96字节压缩或192字节未压缩）
	var sig bls12381.G2Affine
	n, err := sig.SetBytes(combinedSignature)
	if err != nil {
		return false, fmt.Errorf("解析组合签名失败: %w", err)
	}
	if n != len(combinedSignature) {
		return false, fmt.Errorf("组合签名解析不完整：期望 %d 字节，实际解析 %d 字节", len(combinedSignature), n)
	}

	// 2. 解析组公钥（G1，48字节压缩或96字节未压缩）
	var pubKey bls12381.G1Affine
	n, err = pubKey.SetBytes(groupPublicKey)
	if err != nil {
		return false, fmt.Errorf("解析组公钥失败: %w", err)
	}
	if n != len(groupPublicKey) {
		return false, fmt.Errorf("组公钥解析不完整：期望 %d 字节，实际解析 %d 字节", len(groupPublicKey), n)
	}

	// 3. 验证签名份额（可选，用于早期验证）
	for i, share := range shares {
		if i >= int(threshold) {
			break // 只需验证 threshold 个份额
		}

		valid, err := v.VerifySignatureShare(dataHash, share, share.VerificationKey, scheme)
		if err != nil {
			return false, fmt.Errorf("验证签名份额 %d 失败: %w", i, err)
		}
		if !valid {
			return false, fmt.Errorf("签名份额 %d 无效", i)
		}
	}

	// 4. 验证组合签名
	// BLS 签名验证公式：e(pubKey, hash_to_g2(message)) == e(g1_gen, sig)
	// 等价于：e(pubKey, hash_to_g2(message)) * e(-g1_gen, sig) == 1

	// 4.1 将消息哈希到 G2 曲线
	// DST (Domain Separation Tag) 用于区分不同的应用场景
	dst := []byte("BLS_SIG_BLS12381G2_XMD:SHA-256_SSWU_RO_WES_V1")
	hashPoint, err := bls12381.HashToG2(dataHash, dst)
	if err != nil {
		return false, fmt.Errorf("哈希到G2曲线失败: %w", err)
	}

	// 4.2 获取 G1 生成元
	_, _, g1Gen, _ := bls12381.Generators()

	// 4.3 使用配对验证签名
	// BLS 签名验证公式：e(pubKey, hash_to_g2(message)) == e(g1Gen, sig)
	// 等价于：e(pubKey, hashPoint) * e(-g1Gen, sig) == 1
	// 使用 PairingCheck 验证：∏ e(P_i, Q_i) == 1

	// 计算 -g1Gen（G1 生成元的负元）
	var negG1Gen bls12381.G1Affine
	negG1Gen.Neg(&g1Gen)

	// 构造配对：e(pubKey, hashPoint) * e(negG1Gen, sig)
	// PairingCheck 计算 ∏ e(P_i, Q_i)，如果结果为 1 则返回 true
	P := []bls12381.G1Affine{pubKey, negG1Gen}
	Q := []bls12381.G2Affine{hashPoint, sig}

	valid, err := bls12381.PairingCheck(P, Q)
	if err != nil {
		return false, fmt.Errorf("配对验证失败: %w", err)
	}

	if !valid {
		return false, fmt.Errorf("BLS门限签名验证失败：配对不匹配")
	}

	return true, nil
}

// VerifySignatureShare 验证单个签名份额的有效性
//
// 实现 crypto.ThresholdSignatureVerifier 接口
func (v *BLSThresholdVerifier) VerifySignatureShare(
	message []byte,
	share *transaction.ThresholdProof_ThresholdSignatureShare,
	partyPublicKey []byte,
	scheme string,
) (bool, error) {
	if scheme != "BLS_THRESHOLD" {
		return false, fmt.Errorf("不支持的签名方案: %s", scheme)
	}

	// 长度检查
	if len(share.SignatureShare) != bls12381.SizeOfG2AffineCompressed && len(share.SignatureShare) != bls12381.SizeOfG2AffineUncompressed {
		return false, fmt.Errorf("无效的签名份额长度: %d（期望 %d 或 %d 字节）",
			len(share.SignatureShare), bls12381.SizeOfG2AffineCompressed, bls12381.SizeOfG2AffineUncompressed)
	}

	if len(partyPublicKey) != bls12381.SizeOfG1AffineCompressed && len(partyPublicKey) != bls12381.SizeOfG1AffineUncompressed {
		return false, fmt.Errorf("无效的验证密钥长度: %d（期望 %d 或 %d 字节）",
			len(partyPublicKey), bls12381.SizeOfG1AffineCompressed, bls12381.SizeOfG1AffineUncompressed)
	}

	// 1. 解析签名份额到 G2 群元素
	var sigShare bls12381.G2Affine
	n, err := sigShare.SetBytes(share.SignatureShare)
	if err != nil {
		return false, fmt.Errorf("解析签名份额失败: %w", err)
	}
	if n != len(share.SignatureShare) {
		return false, fmt.Errorf("签名份额解析不完整：期望 %d 字节，实际解析 %d 字节", len(share.SignatureShare), n)
	}

	// 2. 解析验证密钥到 G1 群元素
	var verKey bls12381.G1Affine
	n, err = verKey.SetBytes(partyPublicKey)
	if err != nil {
		return false, fmt.Errorf("解析验证密钥失败: %w", err)
	}
	if n != len(partyPublicKey) {
		return false, fmt.Errorf("验证密钥解析不完整：期望 %d 字节，实际解析 %d 字节", len(partyPublicKey), n)
	}

	// 3. 将消息哈希到 G2 曲线
	dst := []byte("BLS_SIG_BLS12381G2_XMD:SHA-256_SSWU_RO_WES_V1")
	hashPoint, err := bls12381.HashToG2(message, dst)
	if err != nil {
		return false, fmt.Errorf("哈希到G2曲线失败: %w", err)
	}

	// 4. 获取 G1 生成元
	_, _, g1Gen, _ := bls12381.Generators()

	// 5. 计算 -g1Gen（G1 生成元的负元）
	var negG1Gen bls12381.G1Affine
	negG1Gen.Neg(&g1Gen)

	// 6. 使用配对验证：e(verKey, hashPoint) == e(g1Gen, sigShare)
	// 等价于：e(verKey, hashPoint) * e(negG1Gen, sigShare) == 1
	P := []bls12381.G1Affine{verKey, negG1Gen}
	Q := []bls12381.G2Affine{hashPoint, sigShare}

	valid, err := bls12381.PairingCheck(P, Q)
	if err != nil {
		return false, fmt.Errorf("配对验证失败: %w", err)
	}

	return valid, nil
}

// 编译期检查：确保 BLSThresholdVerifier 实现了 crypto.ThresholdSignatureVerifier 接口
var _ crypto.ThresholdSignatureVerifier = (*BLSThresholdVerifier)(nil)
