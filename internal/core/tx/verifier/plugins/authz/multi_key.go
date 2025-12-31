// Package authz 提供权限验证插件实现
//
// multi_key.go: 多密钥（M-of-N）权限验证插件
package authz

import (
	"context"
	"fmt"

	"github.com/weisyn/v1/internal/core/tx/ports/hash"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/tx"
)

// MultiKeyPlugin M-of-N 多密钥权限验证插件
//
// 🎯 **核心职责**：验证 MultiKeyLock 的解锁证明（MultiKeyProof）
//
// 💡 **设计理念**：
// 企业多签场景（如公司金库、董事会决策）需要 M-of-N 多重签名：
// - M: 需要的最少签名数
// - N: 授权公钥总数
// - 例如 3-of-5：5 个授权者中任意 3 个签名即可
//
// 🔒 **验证规则**：
// 1. 签名数量检查：signatures.length >= required_signatures
// 2. 索引有效性：每个 key_index 在 [0, N-1] 范围内
// 3. 索引唯一性：不允许重复使用相同的 key_index
// 4. 签名验证：每个签名对应正确的公钥（委托给MultiSignatureVerifier）
// 5. 算法一致性：所有签名算法一致且符合要求（委托给MultiSignatureVerifier）
//
// ⚠️ **核心约束**：
// - ❌ 插件无状态：不存储验证结果
// - ❌ 插件只读：不修改交易
// - ✅ 并发安全：多个 goroutine 可以同时调用
//
// 📞 **调用方**：Verifier Kernel（通过 AuthZ Hook）
type MultiKeyPlugin struct {
	multiSigVerifier  crypto.MultiSignatureVerifier // 多重签名验证器（Crypto层）
	hashCanonicalizer *hash.Canonicalizer           // 规范化哈希计算器（TX 内部工具）
}

// NewMultiKeyPlugin 创建新的 MultiKeyPlugin
//
// 参数：
//   - multiSigVerifier: 多重签名验证器（用于验证多重签名）
//   - hashCanonicalizer: 规范化哈希计算器（用于交易哈希）
//
// 返回：
//   - *MultiKeyPlugin: 新创建的实例
func NewMultiKeyPlugin(
	multiSigVerifier crypto.MultiSignatureVerifier,
	hashCanonicalizer *hash.Canonicalizer,
) *MultiKeyPlugin {
	return &MultiKeyPlugin{
		multiSigVerifier:  multiSigVerifier,
		hashCanonicalizer: hashCanonicalizer,
	}
}

// Name 返回插件名称
//
// 实现 tx.AuthZPlugin 接口
//
// 返回：
//   - string: "multi_key"
func (p *MultiKeyPlugin) Name() string {
	return "multi_key"
}

// Match 验证 UnlockingProof 是否匹配 LockingCondition
//
// 实现 tx.AuthZPlugin 接口
//
// 🎯 **核心逻辑**：
// 1. 类型检查：lock 必须是 MultiKeyLock，proof 必须是 MultiKeyProof
// 2. 签名数量验证：signatures.length >= required_signatures
// 3. 索引有效性和唯一性验证
// 4. 签名验证：每个签名对应正确的公钥
// 5. 算法一致性验证
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
//
// 📝 **使用场景**：
//
//	// 场景：公司金库 3-of-5 多签
//	multi_key_lock {
//	    required_signatures: 3
//	    authorized_keys: [CEO公钥, CFO公钥, CTO公钥, COO公钥, 董事公钥]
//	}
//
//	// 解锁证明：CEO + CFO + CTO 签名
//	multi_key_proof {
//	    signatures: [
//	        {key_index: 0, signature: CEO签名},
//	        {key_index: 1, signature: CFO签名},
//	        {key_index: 2, signature: CTO签名}
//	    ]
//	}
func (p *MultiKeyPlugin) Match(
	ctx context.Context,
	lock *transaction.LockingCondition,
	unlockingProof *transaction.UnlockingProof,
	tx *transaction.Transaction,
) (bool, error) {
	// 1. 类型检查：是否为 MultiKeyLock
	multiKeyLock := lock.GetMultiKeyLock()
	if multiKeyLock == nil {
		return false, nil // 不是 MultiKeyLock，让其他插件处理
	}

	// 2. 提取 MultiKeyProof
	multiKeyProof := unlockingProof.GetMultiKeyProof()
	if multiKeyProof == nil {
		return true, fmt.Errorf("proof is not MultiKeyProof")
	}

	// 3. 验证签名数量
	requiredSigs := multiKeyLock.RequiredSignatures
	providedSigs := uint32(len(multiKeyProof.Signatures))
	if providedSigs < requiredSigs {
		return true, fmt.Errorf(
			"insufficient signatures: required=%d, provided=%d",
			requiredSigs, providedSigs,
		)
	}

	// 4. 找到当前 input 的索引
	//   注意：由于 AuthZ 验证是按输入顺序进行的，我们需要找到匹配的索引
	//   通过比较 proof 的指针地址来定位
	inputIndex := -1
	for i, input := range tx.Inputs {
		// 比较 MultiKeyProof 是否是同一个对象
		if input.GetMultiKeyProof() == multiKeyProof {
			inputIndex = i
			break
		}
	}
	if inputIndex == -1 {
		return true, fmt.Errorf("无法找到当前输入的索引")
	}

	// 5. 准备多重签名验证所需的数据
	// ✅ **完整实现**：为每个签名单独计算签名哈希（如果SighashType不同）
	// 💡 **实现说明**：
	// - 如果所有签名使用相同的SighashType，使用统一的交易哈希
	// - 如果签名使用不同的SighashType，为每个签名单独计算交易哈希并单独验证
	
	// 转换签名和公钥格式
	authorizedKeys := multiKeyLock.AuthorizedKeys
	publicKeys := make([]crypto.PublicKey, 0, len(authorizedKeys))
	for _, pbKey := range authorizedKeys {
		publicKeys = append(publicKeys, crypto.PublicKey{
			Value: pbKey.Value,
		})
	}
	
	// 检查所有签名是否使用相同的SighashType
	allSameSighashType := true
	var firstSighashType transaction.SignatureHashType
	if len(multiKeyProof.Signatures) > 0 {
		firstSighashType = multiKeyProof.Signatures[0].SighashType
		for i := 1; i < len(multiKeyProof.Signatures); i++ {
			if multiKeyProof.Signatures[i].SighashType != firstSighashType {
				allSameSighashType = false
				break
			}
		}
	} else {
		firstSighashType = transaction.SignatureHashType_SIGHASH_ALL
	}

	if allSameSighashType {
		// ✅ 所有签名使用相同的SighashType，使用统一的交易哈希
		txHash, err := p.hashCanonicalizer.ComputeSignatureHashForVerification(
			ctx,
			tx,
			inputIndex,
			firstSighashType,
		)
		if err != nil {
			return true, fmt.Errorf("计算签名哈希失败: %w", err)
		}

		// 转换签名条目格式
		multiSigEntries := make([]crypto.MultiSignatureEntry, 0, len(multiKeyProof.Signatures))
	for _, sigEntry := range multiKeyProof.Signatures {
		multiSigEntries = append(multiSigEntries, crypto.MultiSignatureEntry{
			KeyIndex:   sigEntry.KeyIndex,
			Signature:  sigEntry.Signature.Value,
			Algorithm:  sigEntry.Algorithm,
				SighashType: sigEntry.SighashType,
		})
	}

		// 调用MultiSignatureVerifier进行密码学验证
	valid, err := p.multiSigVerifier.VerifyMultiSignature(
			txHash,
		multiSigEntries,
		publicKeys,
		multiKeyLock.RequiredSignatures,
		multiKeyLock.RequiredAlgorithm,
	)
	
	if err != nil {
		return true, fmt.Errorf("多重签名验证失败: %w", err)
	}
	
	if !valid {
		return true, fmt.Errorf("多重签名验证失败：签名验证不通过")
		}
	} else {
		// ✅ 签名使用不同的SighashType，为每个签名单独计算交易哈希并验证
		// 💡 **实现说明**：不同SighashType会产生不同的交易哈希，需要单独验证
		validCount := 0
		for i, sigEntry := range multiKeyProof.Signatures {
			// 为当前签名计算对应的交易哈希
			txHash, err := p.hashCanonicalizer.ComputeSignatureHashForVerification(
				ctx,
				tx,
				inputIndex,
				sigEntry.SighashType,
			)
			if err != nil {
				return true, fmt.Errorf("计算签名%d的哈希失败: %w", i, err)
			}

			// 验证key_index范围
			if sigEntry.KeyIndex >= uint32(len(publicKeys)) {
				return true, fmt.Errorf("签名%d的key_index=%d超出范围（公钥数量=%d）", i, sigEntry.KeyIndex, len(publicKeys))
			}

			// 使用MultiSignatureVerifier验证单个签名
			// 注意：通过MultiSignatureVerifier间接使用（创建一个单签名条目）
			singleSigEntry := []crypto.MultiSignatureEntry{
				{
					KeyIndex:   sigEntry.KeyIndex,
					Signature:  sigEntry.Signature.Value,
					Algorithm:  sigEntry.Algorithm,
					SighashType: sigEntry.SighashType,
				},
			}
			
			valid, err := p.multiSigVerifier.VerifyMultiSignature(
				txHash,
				singleSigEntry,
				publicKeys,
				1, // 只需要1个签名
				sigEntry.Algorithm,
			)
			
			if err != nil {
				return true, fmt.Errorf("签名%d验证失败: %w", i, err)
			}
			
			if !valid {
				return true, fmt.Errorf("签名%d验证失败：签名验证不通过（key_index=%d）", i, sigEntry.KeyIndex)
			}
			
			validCount++
		}

		// 验证有效签名数是否满足要求
		if uint32(validCount) < multiKeyLock.RequiredSignatures {
			return true, fmt.Errorf(
				"有效签名数不足: 需要 %d 个，实际 %d 个",
				multiKeyLock.RequiredSignatures, validCount,
			)
		}
	}

	// 7. 所有验证通过
	return true, nil
}

// 编译期检查：确保 MultiKeyPlugin 实现了 tx.AuthZPlugin 接口
var _ tx.AuthZPlugin = (*MultiKeyPlugin)(nil)
