// Package threshold 提供门限签名验证实现
//
// frost.go: FROST Schnorr 门限签名验证器包装
//
// 🎯 **设计目的**：
// 提供threshold包内的FROST验证器，内部使用frost封装层
// 保持threshold包的接口稳定，同时隔离dcrd依赖
package threshold

import (
	"github.com/weisyn/v1/internal/core/infrastructure/crypto/frost"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	cryptointf "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
)

// FROSTThresholdVerifier FROST 门限签名验证器包装
//
// 🎯 **设计目的**：
// 包装frost封装层的验证器，对外提供threshold包的接口
// 内部使用frost包封装dcrd依赖
type FROSTThresholdVerifier struct {
	frostVerifier *frost.FROSTVerifier
}

// NewFROSTThresholdVerifier 创建 FROST 门限签名验证器
func NewFROSTThresholdVerifier() *FROSTThresholdVerifier {
	return &FROSTThresholdVerifier{
		frostVerifier: frost.NewFROSTVerifier(),
	}
}

// VerifyThresholdSignature 验证组合门限签名
//
// 实现 cryptointf.ThresholdSignatureVerifier 接口
//
// 委托给frost封装层进行实际验证
func (v *FROSTThresholdVerifier) VerifyThresholdSignature(
	dataHash []byte,
	combinedSignature []byte,
	shares []*transaction.ThresholdProof_ThresholdSignatureShare,
	groupPublicKey []byte,
	threshold uint32,
	totalParties uint32,
	scheme string,
) (bool, error) {
	return v.frostVerifier.VerifyThresholdSignature(
		dataHash,
		combinedSignature,
		shares,
		groupPublicKey,
		threshold,
		totalParties,
		scheme,
	)
	}


// VerifySignatureShare 验证单个签名份额的有效性
//
// 实现 cryptointf.ThresholdSignatureVerifier 接口
//
// 委托给frost封装层进行实际验证
func (v *FROSTThresholdVerifier) VerifySignatureShare(
	message []byte,
	share *transaction.ThresholdProof_ThresholdSignatureShare,
	partyPublicKey []byte,
	scheme string,
) (bool, error) {
	return v.frostVerifier.VerifySignatureShare(message, share, partyPublicKey, scheme)
}

// 编译期检查：确保 FROSTThresholdVerifier 实现了 cryptointf.ThresholdSignatureVerifier 接口
var _ cryptointf.ThresholdSignatureVerifier = (*FROSTThresholdVerifier)(nil)

