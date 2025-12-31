// Package multisig 提供多重签名验证实现
//
// 🎯 **职责**：实现MultiSignatureVerifier接口，提供M-of-N多重签名验证能力
//
// **设计原则**：
// - 专注于密码学验证，不涉及业务规则
// - 依赖SignatureManager进行单签名验证
// - 支持多种签名算法和哈希类型
package multisig

import (
	"fmt"

	cryptointf "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
)

// MultiSignatureVerifierImpl MultiSignatureVerifier接口的实现
type MultiSignatureVerifierImpl struct {
	signatureManager cryptointf.SignatureManager
}

// NewMultiSignatureVerifier 创建新的多重签名验证器
//
// 参数：
//   - signatureManager: 签名管理器，用于单签名验证
//
// 返回：
//   - *MultiSignatureVerifierImpl: 多重签名验证器实例
func NewMultiSignatureVerifier(signatureManager cryptointf.SignatureManager) *MultiSignatureVerifierImpl {
	return &MultiSignatureVerifierImpl{
		signatureManager: signatureManager,
	}
}

// VerifyMultiSignature 验证M-of-N多重签名
//
// 实现 cryptointf.MultiSignatureVerifier 接口
//
// **验证流程**：
// 1. 验证签名数量
// 2. 验证索引有效性
// 3. 验证索引唯一性
// 4. 逐个验证签名
// 5. 验证算法一致性
//
// 参数：
//   - message: 被签名的消息（通常是交易哈希）
//   - signatures: 签名列表
//   - publicKeys: 授权公钥列表
//   - requiredSignatures: 需要的最少签名数（M）
//   - algorithm: 期望的签名算法（如果为0则不强制检查）
//
// 返回：
//   - bool: 验证是否通过
//   - error: 验证失败时的错误信息
func (v *MultiSignatureVerifierImpl) VerifyMultiSignature(
	message []byte,
	signatures []cryptointf.MultiSignatureEntry,
	publicKeys []cryptointf.PublicKey,
	requiredSignatures uint32,
	algorithm cryptointf.SignatureAlgorithm,
) (bool, error) {
	// 1. 验证签名数量
	if uint32(len(signatures)) < requiredSignatures {
		return false, fmt.Errorf(
			"签名数量不足: 需要 %d 个，实际 %d 个",
			requiredSignatures, len(signatures),
		)
	}

	// 2. 验证索引有效性和唯一性
	usedIndices := make(map[uint32]bool)
	for i, sig := range signatures {
		keyIndex := sig.KeyIndex

		// 检查索引范围
		if keyIndex >= uint32(len(publicKeys)) {
			return false, fmt.Errorf(
				"无效的key_index: signatures[%d].key_index=%d >= 公钥数量=%d",
				i, keyIndex, len(publicKeys),
			)
		}

		// 检查索引唯一性
		if usedIndices[keyIndex] {
			return false, fmt.Errorf(
				"重复的key_index: signatures[%d].key_index=%d 已被使用",
				i, keyIndex,
			)
		}
		usedIndices[keyIndex] = true
	}

	// 3. 验证算法一致性
	if len(signatures) > 0 {
		firstAlgo := signatures[0].Algorithm
		for i, sig := range signatures {
			if sig.Algorithm != firstAlgo {
				return false, fmt.Errorf(
					"签名算法不一致: signatures[0].algorithm=%v, signatures[%d].algorithm=%v",
					firstAlgo, i, sig.Algorithm,
				)
			}
		}

		// 检查算法是否符合要求
		if algorithm != 0 && firstAlgo != algorithm {
			return false, fmt.Errorf(
				"签名算法不匹配: 期望 %v，实际 %v",
				algorithm, firstAlgo,
			)
		}
	}

	// 4. 逐个验证签名
	validCount := 0
	for i, sig := range signatures {
		pubKey := publicKeys[sig.KeyIndex]

		// 使用SignatureManager验证签名
		// 注意：这里使用VerifyTransactionSignature，因为它支持SighashType
		// 转换SighashType：transaction.SignatureHashType -> crypto.SignatureHashType
		sigHashType := cryptointf.SignatureHashType(sig.SighashType)
		valid := v.signatureManager.VerifyTransactionSignature(
			message,
			sig.Signature,
			pubKey.Value,
			sigHashType,
		)

		if !valid {
			return false, fmt.Errorf(
				"签名验证失败: signatures[%d] (key_index=%d) 验证不通过",
				i, sig.KeyIndex,
			)
		}
		validCount++
	}

	// 5. 验证有效签名数是否满足要求
	if uint32(validCount) < requiredSignatures {
		return false, fmt.Errorf(
			"有效签名数不足: 需要 %d 个，实际 %d 个",
			requiredSignatures, validCount,
		)
	}

	return true, nil
}

// 编译期检查：确保实现了接口
var _ cryptointf.MultiSignatureVerifier = (*MultiSignatureVerifierImpl)(nil)

