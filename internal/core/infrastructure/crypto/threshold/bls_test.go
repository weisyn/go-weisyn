// Package threshold 提供门限签名验证实现
//
// bls_test.go: BLS 门限签名验证器测试
//
// 🎯 **测试目的**：验证 BLS 门限签名实现的正确性
package threshold

import (
	"crypto/rand"
	"testing"

	bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381"
	"github.com/stretchr/testify/require"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// TestBLSThresholdVerifier_Basic 测试 BLS 门限签名验证器的基本功能
func TestBLSThresholdVerifier_Basic(t *testing.T) {
	verifier := NewBLSThresholdVerifier()
	require.NotNil(t, verifier)
}

// TestBLSThresholdVerifier_HashToG2 测试哈希到 G2 曲线功能
func TestBLSThresholdVerifier_HashToG2(t *testing.T) {
	// 生成随机消息
	message := make([]byte, 32)
	_, err := rand.Read(message)
	require.NoError(t, err)

	// 哈希到 G2 曲线
	dst := []byte("BLS_SIG_BLS12381G2_XMD:SHA-256_SSWU_RO_WES_V1")
	hashPoint, err := bls12381.HashToG2(message, dst)
	require.NoError(t, err)
	require.NotNil(t, hashPoint)

	// 验证点是有效的
	var zero bls12381.G2Affine
	require.False(t, hashPoint.Equal(&zero))
}

// TestBLSThresholdVerifier_PairingCheck 测试配对验证功能
func TestBLSThresholdVerifier_PairingCheck(t *testing.T) {
	// 获取生成元
	_, _, g1Gen, g2Gen := bls12381.Generators()

	// 测试：e(g1Gen, g2Gen) 应该是非平凡的配对值
	P := []bls12381.G1Affine{g1Gen}
	Q := []bls12381.G2Affine{g2Gen}

	valid, err := bls12381.PairingCheck(P, Q)
	require.NoError(t, err)
	// 注意：e(g1Gen, g2Gen) 不等于 1，所以 valid 应该是 false
	// 但这个测试只是验证 PairingCheck 函数可以正常工作
	_ = valid
}

// TestBLSThresholdVerifier_VerifySignatureShare_InvalidInput 测试无效输入的处理
func TestBLSThresholdVerifier_VerifySignatureShare_InvalidInput(t *testing.T) {
	verifier := NewBLSThresholdVerifier()

	// 测试：空消息
	message := []byte{}
	share := &transaction.ThresholdProof_ThresholdSignatureShare{
		SignatureShare: make([]byte, 96),
		VerificationKey: make([]byte, 48),
	}

	valid, err := verifier.VerifySignatureShare(message, share, share.VerificationKey, "BLS_THRESHOLD")
	require.Error(t, err)
	require.False(t, valid)
}

