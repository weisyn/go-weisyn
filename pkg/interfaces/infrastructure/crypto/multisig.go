// Package crypto 提供WES系统的多重签名接口定义
//
// ✍️ **多重签名服务 (Multi-Signature Service)**
//
// 本文件定义了WES区块链系统的多重签名（M-of-N）验证接口，专注于：
// - M-of-N多重签名验证：验证多个签名是否满足最低要求
// - 签名索引验证：确保签名与公钥的对应关系正确
// - 算法一致性：确保所有签名使用相同的算法
//
// 🎯 **核心功能**
// - MultiSignatureVerifier：多重签名验证器接口
// - 支持灵活的M-of-N验证策略
// - 支持多种签名算法（ECDSA、Ed25519等）
//
// 🏗️ **设计原则**
// - 密码学验证：专注于密码学层面的验证
// - 接口抽象：不依赖具体实现细节
// - 可测试性：支持Mock测试
//
// 🔗 **组件关系**
// - MultiSignatureVerifier：被TX模块的MultiKeyPlugin使用
// - 与SignatureManager：依赖签名服务进行单签名验证
package crypto

import (
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// SignatureAlgorithm 签名算法类型（从transaction包导入）
type SignatureAlgorithm = transaction.SignatureAlgorithm

// MultiSignatureEntry 多重签名条目
//
// 表示单个签名及其元信息
type MultiSignatureEntry struct {
	// KeyIndex 在PublicKeys中的索引
	// 范围：[0, len(PublicKeys)-1]
	KeyIndex uint32

	// Signature 签名数据（64字节，r+s格式）
	Signature []byte

	// Algorithm 签名算法类型
	Algorithm SignatureAlgorithm

	// SighashType 签名哈希类型
	SighashType transaction.SignatureHashType
}

// PublicKey 公钥数据
type PublicKey struct {
	// Value 公钥字节数据
	Value []byte
}

// MultiSignatureVerifier M-of-N多重签名验证器接口
//
// 🎯 **职责**：验证M-of-N多重签名
//
// **验证规则**：
// 1. 签名数量：len(signatures) >= requiredSignatures
// 2. 索引有效性：每个signature的KeyIndex < len(publicKeys)
// 3. 索引唯一性：signatures中的KeyIndex不重复
// 4. 签名有效性：每个signature对message的签名验证通过
// 5. 算法一致性：所有signature的Algorithm一致
//
// **参数说明**：
//   - message: 被签名的消息（通常是交易哈希）
//   - signatures: 签名列表
//   - publicKeys: 授权公钥列表（索引0对应KeyIndex=0）
//   - requiredSignatures: 需要的最少签名数（M）
//   - algorithm: 期望的签名算法（如果为0则不检查）
//
// **返回**：
//   - bool: 验证是否通过
//   - error: 验证失败的原因
//
// **使用示例**：
//
//	verifier := crypto.NewMultiSignatureVerifier(signatureManager)
//	valid, err := verifier.VerifyMultiSignature(
//	    txHash,
//	    []MultiSignatureEntry{
//	        {KeyIndex: 0, Signature: sig0, Algorithm: SigAlgoECDSA},
//	        {KeyIndex: 1, Signature: sig1, Algorithm: SigAlgoECDSA},
//	    },
//	    []PublicKey{pubKey0, pubKey1, pubKey2},
//	    2, // 需要2个签名
//	    SigAlgoECDSA,
//	)
type MultiSignatureVerifier interface {
	// VerifyMultiSignature 验证M-of-N多重签名
	//
	// 此方法负责密码学层面的验证，不涉及业务规则判断
	VerifyMultiSignature(
		message []byte,
		signatures []MultiSignatureEntry,
		publicKeys []PublicKey,
		requiredSignatures uint32,
		algorithm SignatureAlgorithm,
	) (bool, error)
}

