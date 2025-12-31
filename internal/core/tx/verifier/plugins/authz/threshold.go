package authz

import (
	"context"
	"fmt"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/tx"
)

// ThresholdPlugin 门限签名验证插件
//
// 🎯 **核心职责**：验证门限签名锁定条件（ThresholdLock + ThresholdProof）
//
// 💡 **设计理念**：
// 门限签名使用门限密码学的高级多签方案，适用于：
// - 银行级安全：央行数字货币发行（5-of-7 门限）
// - 大额资产管理：企业金库（3-of-5 门限）
// - 高安全协作：企业级 AI 模型访问控制
// - 分布式托管：多方联合授权
//
// 🔒 **验证规则**：
// 1. 提供的份额数量 >= threshold
// 2. 每个份额对应不同的参与方（party_id 不重复）
// 3. 每个份额的验证密钥在预定义集合中
// 4. 组合签名能够验证通过
// 5. 签名方案符合锁定条件的要求
//
// ⚠️ **核心约束**：
// - ❌ 插件无状态：不存储验证结果
// - ❌ 插件只读：不修改交易
// - ✅ 并发安全：多个 goroutine 可以同时调用
//
// 📞 **调用方**：Verifier Kernel（通过 AuthZ Hook）
//
// 📝 **门限签名方案**：
// - BLS_THRESHOLD：BLS门限签名（推荐）
// - ECDSA_TSS：ECDSA门限签名（兼容性）
// - SCHNORR_MUSIG：Schnorr MuSig（高效）
type ThresholdPlugin struct{}

// NewThresholdPlugin 创建新的 ThresholdPlugin
//
// 返回：
//   - *ThresholdPlugin: 新创建的实例
func NewThresholdPlugin() *ThresholdPlugin {
	return &ThresholdPlugin{}
}

// Name 返回插件名称
//
// 实现 tx.AuthZPlugin 接口
//
// 返回：
//   - string: "threshold"
func (p *ThresholdPlugin) Name() string {
	return "threshold"
}

// Match 验证 UnlockingProof 是否匹配 LockingCondition
//
// 实现 tx.AuthZPlugin 接口
//
// 🎯 **核心逻辑**：
// 1. 类型检查：lock 必须是 ThresholdLock
// 2. 提取 ThresholdProof
// 3. 验证份额数量 >= threshold
// 4. 验证 party_id 唯一性和有效性
// 5. 验证每个份额的验证密钥
// 6. 验证组合签名
// 7. 验证签名方案一致性
//
// 参数：
//   - ctx: 上下文对象
//   - lock: UTXO 的锁定条件
//   - unlockingProof: input 的解锁证明
//   - tx: 完整的交易对象（用于签名验证）
//
// 返回：
//   - bool: 是否匹配此插件
//   - true: 此插件处理了验证（可能成功或失败）
//   - false: 此插件不处理此类型的 lock/proof
//   - error: 验证错误
//   - nil: 验证成功
//   - non-nil: 验证失败，描述失败原因
func (p *ThresholdPlugin) Match(
	ctx context.Context,
	lock *transaction.LockingCondition,
	unlockingProof *transaction.UnlockingProof,
	tx *transaction.Transaction,
) (bool, error) {
	// 1. 类型检查：是否为 ThresholdLock
	thresholdLock := lock.GetThresholdLock()
	if thresholdLock == nil {
		return false, nil // 不是 ThresholdLock，让其他插件处理
	}

	// 2. 提取 ThresholdProof
	thresholdProof := unlockingProof.GetThresholdProof()
	if thresholdProof == nil {
		// 如果没有 ThresholdProof，但有 ThresholdLock，则认为匹配但验证失败
		return true, fmt.Errorf("missing threshold proof for ThresholdLock")
	}

	// 3. 验证份额数量 >= threshold
	shareCount := uint32(len(thresholdProof.Shares))
	if shareCount < thresholdLock.Threshold {
		return true, fmt.Errorf(
			"insufficient signature shares: got %d, need %d",
			shareCount,
			thresholdLock.Threshold,
		)
	}

	// 4. 验证 party_id 唯一性和有效性
	seenPartyIDs := make(map[uint32]bool)
	for _, share := range thresholdProof.Shares {
		// 检查 party_id 是否在有效范围内
		if share.PartyId >= thresholdLock.TotalParties {
			return true, fmt.Errorf(
				"invalid party_id: %d (max: %d)",
				share.PartyId,
				thresholdLock.TotalParties-1,
			)
		}

		// 检查 party_id 是否重复
		if seenPartyIDs[share.PartyId] {
			return true, fmt.Errorf("duplicate party_id: %d", share.PartyId)
		}
		seenPartyIDs[share.PartyId] = true
	}

	// 5. 验证每个份额的验证密钥
	for _, share := range thresholdProof.Shares {
		// 验证验证密钥是否在预定义集合中
		if share.PartyId >= uint32(len(thresholdLock.PartyVerificationKeys)) {
			return true, fmt.Errorf(
				"party_id %d exceeds verification keys count: %d",
				share.PartyId,
				len(thresholdLock.PartyVerificationKeys),
			)
		}

		expectedKey := thresholdLock.PartyVerificationKeys[share.PartyId]
		if !bytesEqual(share.VerificationKey, expectedKey) {
			return true, fmt.Errorf(
				"verification key mismatch for party_id %d",
				share.PartyId,
			)
		}

		// P8 简化：暂不验证单个份额的签名有效性
		// 实际应使用门限签名库验证每个 signature_share 的有效性
		if len(share.SignatureShare) == 0 {
			return true, fmt.Errorf("empty signature share for party_id %d", share.PartyId)
		}
	}

	// 6. 验证组合签名
	// P8 简化：只检查非空
	// 实际应使用门限签名库（如 BLS、ECDSA-TSS）验证组合签名的有效性
	if len(thresholdProof.CombinedSignature) == 0 {
		return true, fmt.Errorf("empty combined signature")
	}

	// 7. 验证签名方案一致性
	if thresholdProof.SignatureScheme != thresholdLock.SignatureScheme {
		return true, fmt.Errorf(
			"signature scheme mismatch: proof=%s, lock=%s",
			thresholdProof.SignatureScheme,
			thresholdLock.SignatureScheme,
		)
	}

	// P8 简化：暂不实现完整的门限签名验证
	// 实际应：
	// 1. 根据 signature_scheme 选择对应的门限签名库
	// 2. 使用门限签名库的 Verify() 方法验证组合签名
	// 3. 确保组合签名是由至少 threshold 个有效份额生成的
	//
	// 示例（BLS门限签名）：
	// blsLib := getthresholdSignatureLib(thresholdLock.SignatureScheme)
	// txHash := computeTransactionHash(tx)
	// isValid := blsLib.VerifyThresholdSignature(
	//     thresholdProof.CombinedSignature,
	//     txHash,
	//     thresholdLock.PartyVerificationKeys,
	//     thresholdLock.Threshold,
	// )
	// if !isValid {
	//     return true, fmt.Errorf("threshold signature verification failed")
	// }

	// 8. 验证通过
	return true, nil
}

// 编译期检查：确保 ThresholdPlugin 实现了 tx.AuthZPlugin 接口
var _ tx.AuthZPlugin = (*ThresholdPlugin)(nil)
