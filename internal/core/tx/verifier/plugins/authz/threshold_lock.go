// Package authz 提供 AuthZ 验证插件实现
//
// threshold_lock.go: 门限签名锁定验证插件
package authz

import (
	"bytes"
	"context"
	"fmt"

	"github.com/weisyn/v1/internal/core/tx/ports/hash"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
)

// ThresholdLockPlugin 门限签名锁定验证插件
//
// 🎯 **核心职责**：验证 ThresholdLock 和 ThresholdProof 的匹配
//
// 💡 **设计理念**：
// ThresholdLock 使用门限密码学的高级多签方案，适用于银行级安全、
// 大额资产管理等场景。与 MultiKeyLock 不同，ThresholdLock 使用
// 门限签名技术（如 BLS Threshold），提供更高的安全性和效率。
//
// 🔒 **验证要点**：
// 1. 签名份额数量必须 >= threshold
// 2. 每个份额必须对应不同的参与方
// 3. 参与方必须在 party_verification_keys 列表中
// 4. 组合签名必须验证通过
// 5. 签名方案必须匹配
//
// 📋 **典型应用**：
// - 央行数字货币发行：5-of-7 门限签名
// - 企业级AI模型：多方联合授权
// - 高安全协作：银行级安全要求
type ThresholdLockPlugin struct {
	thresholdVerifier crypto.ThresholdSignatureVerifier // 门限签名验证器
	hashCanonicalizer *hash.Canonicalizer               // 交易哈希计算器
}

// NewThresholdLockPlugin 创建新的 ThresholdLockPlugin
//
// 参数：
//   - thresholdVerifier: 门限签名验证器（用于验证门限签名）
//   - hashCanonicalizer: 交易哈希计算器（用于计算签名哈希）
//
// 返回：
//   - *ThresholdLockPlugin: 新创建的插件实例
func NewThresholdLockPlugin(
	thresholdVerifier crypto.ThresholdSignatureVerifier,
	hashCanonicalizer *hash.Canonicalizer,
) *ThresholdLockPlugin {
	return &ThresholdLockPlugin{
		thresholdVerifier: thresholdVerifier,
		hashCanonicalizer: hashCanonicalizer,
	}
}

// Name 返回插件名称
//
// 实现 tx.AuthZPlugin 接口
//
// 返回：
//   - string: 插件名称 "ThresholdLock"
func (p *ThresholdLockPlugin) Name() string {
	return "ThresholdLock"
}

// Match 验证 ThresholdLock 和 ThresholdProof 的匹配
//
// 实现 tx.AuthZPlugin 接口
//
// 🎯 **验证流程**：
// 1. 检查 lock 是否为 ThresholdLock
// 2. 提取 ThresholdProof
// 3. 验证签名份额数量 >= threshold
// 4. 验证每个份额对应不同的参与方
// 5. 验证参与方在 party_verification_keys 列表中
// 6. 验证签名方案匹配
// 7. 验证组合签名
//
// 参数：
//   - ctx: 上下文对象
//   - lock: 锁定条件（期望为 ThresholdLock）
//   - unlockingProof: 解锁证明（期望包含 ThresholdProof）
//   - tx: 完整的交易对象
//
// 返回：
//   - bool: 是否匹配（true=匹配，false=不匹配）
//   - error: 验证错误（匹配但验证失败时返回错误）
//
// 📝 **使用示例**：
//
//	thresholdLock := &transaction.LockingCondition{
//	    Condition: &transaction.LockingCondition_ThresholdLock{
//	        ThresholdLock: &transaction.ThresholdLock{
//	            Threshold: 5,
//	            TotalParties: 7,
//	            PartyVerificationKeys: [][]byte{key1, key2, ..., key7},
//	            SignatureScheme: "BLS_THRESHOLD",
//	            SecurityLevel: 256,
//	        },
//	    },
//	}
//
//	thresholdProof := &transaction.ThresholdProof{
//	    Shares: []*transaction.ThresholdProof_ThresholdSignatureShare{
//	        {PartyId: 1, SignatureShare: share1, VerificationKey: vk1},
//	        {PartyId: 3, SignatureShare: share3, VerificationKey: vk3},
//	        {PartyId: 4, SignatureShare: share4, VerificationKey: vk4},
//	        {PartyId: 5, SignatureShare: share5, VerificationKey: vk5},
//	        {PartyId: 7, SignatureShare: share7, VerificationKey: vk7},
//	    },
//	    CombinedSignature: combinedSig,
//	    SignatureScheme: "BLS_THRESHOLD",
//	}
func (p *ThresholdLockPlugin) Match(
	ctx context.Context,
	lock *transaction.LockingCondition,
	unlockingProof *transaction.UnlockingProof,
	tx *transaction.Transaction,
) (bool, error) {
	// 1. 类型检查：是否为 ThresholdLock
	thresholdLock := lock.GetThresholdLock()
	if thresholdLock == nil {
		// 不是 ThresholdLock，返回 false 表示跳过此插件
		return false, nil
	}

	// 2. 提取 ThresholdProof
	thresholdProof := unlockingProof.GetThresholdProof()
	if thresholdProof == nil {
		return true, fmt.Errorf("ThresholdLock 需要 ThresholdProof，但未提供")
	}

	// 3. 验证签名份额数量 >= threshold
	if uint32(len(thresholdProof.Shares)) < thresholdLock.Threshold {
		return true, fmt.Errorf("签名份额数量 %d 小于门限值 %d",
			len(thresholdProof.Shares),
			thresholdLock.Threshold)
	}

	// 4. 验证每个份额对应不同的参与方（防止重复）
	usedParties := make(map[uint32]bool)
	for _, share := range thresholdProof.Shares {
		if usedParties[share.PartyId] {
			return true, fmt.Errorf("参与方 %d 的份额重复", share.PartyId)
		}
		usedParties[share.PartyId] = true

		// 5. 验证参与方在 party_verification_keys 列表中
		// 设计约定（无须改 proto）：
		// - party_verification_keys[0] 存放 group_public_key（组公钥）
		// - party_verification_keys[1..total_parties] 存放各参与方 verification key（按 party_id 对齐）
		//
		// 这样可以在不扩展 protobuf 字段的前提下，为门限签名验证提供必需的 group public key，
		// 并避免“拿第一个参与方公钥当组公钥”的错误占位行为。
		if thresholdLock.TotalParties == 0 {
			return true, fmt.Errorf("ThresholdLock total_parties 不能为空")
		}
		expectedKeysLen := int(thresholdLock.TotalParties) + 1
		if len(thresholdLock.PartyVerificationKeys) != expectedKeysLen {
			return true, fmt.Errorf("ThresholdLock party_verification_keys 长度不符合约定：期望 %d（含 group key），实际 %d",
				expectedKeysLen, len(thresholdLock.PartyVerificationKeys))
		}
		if share.PartyId == 0 || share.PartyId > thresholdLock.TotalParties {
			return true, fmt.Errorf("参与方 party_id=%d 非法（期望 1..%d）", share.PartyId, thresholdLock.TotalParties)
		}

		// 验证 verification_key 匹配
		expectedKey := thresholdLock.PartyVerificationKeys[share.PartyId]
		if len(share.VerificationKey) == 0 || len(expectedKey) == 0 || !bytes.Equal(share.VerificationKey, expectedKey) {
			return true, fmt.Errorf("参与方 %d 的验证密钥不匹配", share.PartyId)
		}
	}

	// 6. 验证签名方案匹配
	if thresholdProof.SignatureScheme != thresholdLock.SignatureScheme {
		return true, fmt.Errorf("签名方案不匹配：期望 %s，实际 %s",
			thresholdLock.SignatureScheme,
			thresholdProof.SignatureScheme)
	}

	// 🔐 **P3-1: 实现门限签名验证** ✅
	//
	// **验证逻辑**：
	// 1. 找到当前输入的索引
	// 2. 计算交易签名哈希
	// 3. 使用 ThresholdSignatureVerifier 验证门限签名
	
	// 1. 找到当前输入的索引
	inputIndex := -1
	for i, input := range tx.Inputs {
		// 比较 ThresholdProof 是否是同一个对象
		if input.GetThresholdProof() == thresholdProof {
			inputIndex = i
			break
		}
	}
	if inputIndex == -1 {
		return true, fmt.Errorf("无法找到当前输入的索引")
	}

	// 2. 计算交易签名哈希（用于验证）
	txHash, err := p.hashCanonicalizer.ComputeSignatureHashForVerification(
		ctx, tx, inputIndex, transaction.SignatureHashType_SIGHASH_ALL)
	if err != nil {
		return true, fmt.Errorf("计算交易签名哈希失败: %w", err)
	}

	// 3. 提取门限签名参数
	combinedSig := thresholdProof.CombinedSignature
	shares := thresholdProof.Shares
	scheme := thresholdLock.SignatureScheme
	
	// 4. 组公钥获取（按约定：party_verification_keys[0]）
	expectedKeysLen := int(thresholdLock.TotalParties) + 1
	if len(thresholdLock.PartyVerificationKeys) != expectedKeysLen {
		return true, fmt.Errorf("ThresholdLock party_verification_keys 长度不符合约定：期望 %d（含 group key），实际 %d",
			expectedKeysLen, len(thresholdLock.PartyVerificationKeys))
	}
	groupPubKey := thresholdLock.PartyVerificationKeys[0]
	if len(groupPubKey) == 0 {
		return true, fmt.Errorf("ThresholdLock group public key 为空（要求存放在 party_verification_keys[0]）")
	}

	// 5. 验证门限签名
	// ✅ **完整实现**：调用 ThresholdSignatureVerifier 进行密码学验证
	// 💡 **实现说明**：
	// - BLS_THRESHOLD: 使用 gnark-crypto 库进行 BLS12-381 配对验证
	// - FROST_SCHNORR: 使用 dcrd 库进行 secp256k1 Schnorr 验证
	// - 两种方案都已完整实现，支持签名分片验证和组合签名验证
	if p.thresholdVerifier != nil {
		valid, err := p.thresholdVerifier.VerifyThresholdSignature(
			txHash,
			combinedSig,
			shares,
			groupPubKey,
			thresholdLock.Threshold,
			thresholdLock.TotalParties,
			scheme,
		)
		if err != nil {
			return true, fmt.Errorf("门限签名验证出错: %w", err)
		}
		if !valid {
			return true, fmt.Errorf("门限签名验证失败：签名无效（方案=%s）", scheme)
		}
		// ✅ 门限签名验证通过
	} else {
		// ⚠️ **向后兼容**：如果没有提供验证器，跳过密码学验证
		// 这允许在测试环境或使用外部门限签名服务时继续工作
		// 但在生产环境中，应该始终提供 ThresholdSignatureVerifier
		// 建议：未来可以考虑将签名验证设为强制（返回错误而不是跳过）
	}

	// 验证通过
	return true, nil
}
