// Package authz 提供权限验证插件实现
//
// 本包实现 AuthZ 钩子的各种验证插件，负责验证 UnlockingProof 是否匹配 LockingCondition。
package authz

import (
	"bytes"
	"context"
	"fmt"

	"github.com/weisyn/v1/internal/core/tx/ports/hash"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/tx"
)

// SingleKeyPlugin 单密钥验证插件
//
// 🎯 **核心职责**：验证 SingleKeyProof 是否匹配 SingleKeyLock
//
// 💡 **设计理念**：
// 单密钥验证是最基础的权限验证方式，类似于 BTC 的 P2PKH（Pay-to-Public-Key-Hash）。
// 验证过程包括：签名验证 + 公钥/地址验证。
//
// ⚠️ **P1 MVP 约束**：
// - 支持 ECDSA_SECP256K1 和 ED25519 两种签名算法
// - 支持通过公钥或地址哈希进行验证
// - 插件无状态，可并行调用
//
// 📞 **调用方**：Verifier Kernel（通过 AuthZ Hook）
type SingleKeyPlugin struct {
	sigManager        crypto.SignatureManager
	hashManager       crypto.HashManager
	hashCanonicalizer *hash.Canonicalizer // 规范化哈希计算器（TX 内部工具）
}

// NewSingleKeyPlugin 创建新的 SingleKeyPlugin
//
// 参数：
//   - sigManager: 签名管理器（用于验证签名）
//   - hashManager: 哈希管理器（用于地址计算）
//   - hashCanonicalizer: 规范化哈希计算器（用于交易哈希）
//
// 返回：
//   - *SingleKeyPlugin: 新创建的实例
func NewSingleKeyPlugin(
	sigManager crypto.SignatureManager,
	hashManager crypto.HashManager,
	hashCanonicalizer *hash.Canonicalizer,
) *SingleKeyPlugin {
	return &SingleKeyPlugin{
		sigManager:        sigManager,
		hashManager:       hashManager,
		hashCanonicalizer: hashCanonicalizer,
	}
}

// Name 返回插件名称
//
// 实现 tx.AuthZPlugin 接口
//
// 返回：
//   - string: "single_key"
func (p *SingleKeyPlugin) Name() string {
	return "single_key"
}

// Match 验证 UnlockingProof 是否匹配 LockingCondition
//
// 实现 tx.AuthZPlugin 接口
//
// 🎯 **核心逻辑**：
// 1. 类型检查：lock 必须是 SingleKeyLock，proof 必须是 SingleKeyProof
// 2. 签名验证：验证 proof 中的签名是否对交易哈希有效
// 3. 公钥/地址验证：验证 proof 中的公钥是否与 lock 中要求的一致
//
// 参数：
//   - ctx: 上下文对象
//   - lock: UTXO 的锁定条件
//   - unlockingProof: input 的解锁证明（wrapped in UnlockingProof）
//   - tx: 完整的交易对象（用于签名验证）
//
// 返回：
//   - bool: 是否匹配此插件
//   - true: 此插件处理了验证（可能成功或失败）
//   - false: 此插件不处理此类型的 lock/proof
//   - error: 验证错误
//   - nil: 验证成功
//   - non-nil: 验证失败，描述失败原因
func (p *SingleKeyPlugin) Match(
	ctx context.Context,
	lock *transaction.LockingCondition,
	unlockingProof *transaction.UnlockingProof,
	tx *transaction.Transaction,
) (bool, error) {
	// 1. 类型检查：是否为 SingleKeyLock
	singleKeyLock := lock.GetSingleKeyLock()
	if singleKeyLock == nil {
		return false, nil // 不是 SingleKeyLock，让其他插件处理
	}

	// 2. 提取 SingleKeyProof
	singleKeyProof := unlockingProof.GetSingleKeyProof()
	if singleKeyProof == nil {
		return true, fmt.Errorf("SingleKeyLock 需要 SingleKeyProof，但proof为空或类型不匹配")
	}

	// 3. 找到当前 input 的索引
	//   注意：由于 AuthZ 验证是按输入顺序进行的，我们需要找到匹配的索引
	//   通过比较 proof 的指针地址来定位
	inputIndex := -1
	for i, input := range tx.Inputs {
		// 比较 SingleKeyProof 是否是同一个对象
		if input.GetSingleKeyProof() == singleKeyProof {
			inputIndex = i
			break
		}
	}
	if inputIndex == -1 {
		return true, fmt.Errorf("无法找到当前输入的索引")
	}

	// 4. 验证签名
	if err := p.verifySignature(ctx, tx, singleKeyProof, inputIndex); err != nil {
		return true, fmt.Errorf("签名验证失败: %w", err)
	}

	// 5. 验证公钥或地址
	if err := p.verifyPublicKey(ctx, singleKeyLock, singleKeyProof); err != nil {
		return true, fmt.Errorf("公钥/地址验证失败: %w", err)
	}

	return true, nil
}

// verifySignature 验证签名是否有效
//
// 参数：
//   - ctx: 上下文对象
//   - tx: 待验证的交易
//   - proof: SingleKeyProof（包含签名和公钥）
//   - inputIndex: 当前输入索引（用于计算签名哈希）
//
// 返回：
//   - error: 验证失败的原因
func (p *SingleKeyPlugin) verifySignature(
	ctx context.Context,
	tx *transaction.Transaction,
	proof *transaction.SingleKeyProof,
	inputIndex int,
) error {
	// 1. 使用 HashCanonicalizer 计算签名哈希（用于验证）
	txHash, err := p.hashCanonicalizer.ComputeSignatureHashForVerification(
		ctx,
		tx,
		inputIndex,
		proof.SighashType,
	)
	if err != nil {
		return fmt.Errorf("计算签名哈希失败: %w", err)
	}

	// 2. 提取公钥
	if proof.PublicKey == nil || len(proof.PublicKey.Value) == 0 {
		return fmt.Errorf("公钥为空")
	}
	pubKeyBytes := proof.PublicKey.Value

	// 3. 提取签名
	if proof.Signature == nil || len(proof.Signature.Value) == 0 {
		return fmt.Errorf("签名为空")
	}
	signatureBytes := proof.Signature.Value

	// 4. 根据算法验证签名
	switch proof.Algorithm {
	case transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1:
		// ECDSA secp256k1 签名验证
		// ⚠️ 直接使用verifyECDSA，因为txHash已经是规范化哈希，不需要再哈希
		valid := p.sigManager.VerifyTransactionSignature(txHash, signatureBytes, pubKeyBytes, crypto.SigHashAll)
		if !valid {
			return fmt.Errorf("ECDSA签名验证失败: txHash=%x, pubKey=%x, sig=%x",
				txHash[:8], pubKeyBytes[:8], signatureBytes[:8])
		}

	case transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ED25519:
		// Ed25519 签名验证
		valid := p.sigManager.VerifyTransactionSignature(txHash, signatureBytes, pubKeyBytes, crypto.SigHashAll)
		if !valid {
			return fmt.Errorf("Ed25519签名验证失败")
		}

	default:
		return fmt.Errorf("不支持的签名算法: %v", proof.Algorithm)
	}

	return nil
}

// verifyPublicKey 验证公钥或地址是否匹配
//
// 参数：
//   - ctx: 上下文对象
//   - lock: SingleKeyLock（包含预期的公钥或地址）
//   - proof: SingleKeyProof（包含实际的公钥）
//
// 返回：
//   - error: 验证失败的原因
func (p *SingleKeyPlugin) verifyPublicKey(
	ctx context.Context,
	lock *transaction.SingleKeyLock,
	proof *transaction.SingleKeyProof,
) error {
	// 提取 proof 中的公钥
	if proof.PublicKey == nil || len(proof.PublicKey.Value) == 0 {
		return fmt.Errorf("proof中的公钥为空")
	}
	actualPubKey := proof.PublicKey.Value

	// 检查 lock 中定义的约束类型
	switch keyReq := lock.KeyRequirement.(type) {
	case *transaction.SingleKeyLock_RequiredPublicKey:
		// 约束类型：直接验证公钥
		if keyReq.RequiredPublicKey == nil || len(keyReq.RequiredPublicKey.Value) == 0 {
			return fmt.Errorf("lock中的公钥为空")
		}
		expectedPubKey := keyReq.RequiredPublicKey.Value
		if !bytes.Equal(actualPubKey, expectedPubKey) {
			return fmt.Errorf("公钥不匹配")
		}
		return nil

	case *transaction.SingleKeyLock_RequiredAddressHash:
		// 约束类型：通过地址验证
		if len(keyReq.RequiredAddressHash) == 0 {
			return fmt.Errorf("lock中的地址哈希为空")
		}
		expectedAddressHash := keyReq.RequiredAddressHash

		// 从公钥计算地址哈希
		actualAddressHash, err := p.computeAddressFromPublicKey(actualPubKey)
		if err != nil {
			return fmt.Errorf("计算地址哈希失败: %w", err)
		}

		if !bytes.Equal(actualAddressHash, expectedAddressHash) {
			return fmt.Errorf("地址哈希不匹配")
		}
		return nil

	default:
		return fmt.Errorf("不支持的锁定约束类型: %T", lock.KeyRequirement)
	}
}

// computeAddressFromPublicKey 从公钥计算地址
//
// 参数：
//   - pubKey: 公钥字节
//
// 返回：
//   - []byte: 地址（20字节）
//   - error: 计算失败
func (p *SingleKeyPlugin) computeAddressFromPublicKey(pubKey []byte) ([]byte, error) {
	// 地址计算：address = RIPEMD160(SHA256(pubKey))
	// 这是类似 BTC 的地址生成方式

	// 1. SHA256
	sha256Hash := p.hashManager.SHA256(pubKey)

	// 2. RIPEMD160
	addressHash := p.hashManager.RIPEMD160(sha256Hash)

	return addressHash, nil
}

// 编译期检查：确保 SingleKeyPlugin 实现了 tx.AuthZPlugin 接口
var _ tx.AuthZPlugin = (*SingleKeyPlugin)(nil)
