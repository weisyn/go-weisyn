// Package crypto 提供WES系统的门限签名接口定义
//
// 🔐 **门限签名服务 (Threshold Signature Service)**
//
// 本文件定义了WES区块链系统的门限签名接口，专注于：
// - 门限签名验证：BLS Threshold、FROST Schnorr等主流方案
// - 组合签名验证：验证多个签名分片的组合签名
// - 多方安全：支持M-of-N门限签名验证
//
// 🎯 **核心功能**
// - ThresholdSignatureVerifier：门限签名验证器接口
// - 多方案支持：BLS、FROST等主流门限签名方案
// - 安全验证：密码学级别的签名验证
//
// 🏗️ **设计原则**
// - 算法标准：支持业界标准的门限签名方案
// - 安全可靠：使用成熟的密码学库和算法实现
// - 接口抽象：支持多种门限签名库实现
//
// 🔗 **组件关系**
// - ThresholdSignatureVerifier：被 ThresholdLockPlugin 使用
// - 与SignatureManager：配合进行签名验证
package crypto

import (
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ThresholdSignatureVerifier 门限签名验证器接口
//
// 🎯 **核心职责**：验证门限签名的有效性
//
// 💡 **设计理念**：
// 门限签名是一种高级多签方案，允许 M-of-N 的参与方生成有效签名，
// 而不需要收集所有私钥。常见的方案包括：
// - BLS Threshold：基于双线性配对的门限签名
// - FROST Schnorr：基于Schnorr签名的门限方案
//
// ⚠️ **核心约束**：
// - ✅ 只读验证：不修改签名或数据
// - ✅ 无状态：不存储验证结果
// - ✅ 并发安全：可并行调用
//
// 📝 **典型使用场景**：
// - ThresholdLock验证：验证门限签名解锁UTXO
// - 多方授权：企业级多签场景
// - 银行级安全：大额资产管理
type ThresholdSignatureVerifier interface {
	// VerifyThresholdSignature 验证门限签名
	//
	// 🎯 **核心逻辑**：
	// 1. 根据签名方案选择验证算法
	// 2. 验证签名分片的有效性
	// 3. 验证组合签名的有效性
	// 4. 验证签名分片数量 >= threshold
	//
	// 参数：
	//   - message: 待验证的消息（通常是交易哈希）
	//   - combinedSignature: 组合签名（由签名分片组合而成）
	//   - shares: 签名分片列表（至少 threshold 个）
	//   - groupPublicKey: 门限组的公钥
	//   - threshold: 门限值（至少需要 threshold 个签名分片）
	//   - totalParties: 总参与方数量
	//   - scheme: 签名方案（如 "BLS_THRESHOLD", "FROST_SCHNORR"）
	//
	// 返回：
	//   - bool: 签名是否有效
	//   - error: 验证错误（如签名方案不支持、参数无效等）
	//
	// 📝 **使用示例**：
	//
	//	// BLS 门限签名验证
	//	valid, err := verifier.VerifyThresholdSignature(
	//	    txHash,
	//	    combinedSig,
	//	    shares,
	//	    groupPubKey,
	//	    5,  // threshold
	//	    7,  // totalParties
	//	    "BLS_THRESHOLD",
	//	)
	//	if err != nil {
	//	    return fmt.Errorf("门限签名验证失败: %w", err)
	//	}
	//	if !valid {
	//	    return fmt.Errorf("门限签名无效")
	//	}
	VerifyThresholdSignature(
		message []byte,
		combinedSignature []byte,
		shares []*transaction.ThresholdProof_ThresholdSignatureShare,
		groupPublicKey []byte,
		threshold uint32,
		totalParties uint32,
		scheme string,
	) (bool, error)

	// VerifySignatureShare 验证单个签名分片
	//
	// 🎯 **用途**：在组合签名前验证每个分片的有效性
	//
	// 参数：
	//   - message: 待验证的消息
	//   - share: 签名分片
	//   - partyPublicKey: 参与方的公钥
	//   - scheme: 签名方案
	//
	// 返回：
	//   - bool: 分片是否有效
	//   - error: 验证错误
	//
	// ⚠️ **注意**：
	// - 此方法用于早期验证，不是必须的
	// - 如果 VerifyThresholdSignature 已验证组合签名，此方法可跳过
	VerifySignatureShare(
		message []byte,
		share *transaction.ThresholdProof_ThresholdSignatureShare,
		partyPublicKey []byte,
		scheme string,
	) (bool, error)
}

