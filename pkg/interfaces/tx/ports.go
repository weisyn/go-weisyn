// Package tx provides port interfaces for transaction operations.
package tx

import (
	"context"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/types"
)

// ================================================================================================
// ✍️ Signer（签名服务端口）
// ================================================================================================

// Signer 签名服务接口
//
// 🎯 **核心职责**：对交易进行数字签名
//
// 💡 **设计理念**：
// 通过端口接口抽象签名服务，支持多种签名源（Local、KMS、HSM）的灵活替换。
// 符合六边形架构的"端口/适配器"模式。
//
// 🔌 **适配器实现**：
// 1. LocalSigner: 使用本地私钥签名（开发/测试环境）
// 2. KMSSigner: 使用 AWS KMS 签名（云环境）
// 3. HSMSigner: 使用硬件安全模块签名（企业环境）
//
// ⚠️ **核心约束**：
// - 不能修改交易内容
// - 签名必须可验证（与 LockingCondition 匹配）
// - 签名算法必须符合系统要求
//
// 📞 **调用方**：
// - ProvenTx.Sign(): Type-state 转换时签名
// - SignedTx 创建时使用
type Signer interface {
	// Sign 对交易签名
	//
	// 🎯 **核心逻辑**：
	// 1. 计算交易哈希（Canonical Serialization）
	// 2. 使用私钥对哈希签名
	// 3. 返回签名数据
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - tx: 待签名的交易（ProvenTx 的底层对象）
	//
	// 返回：
	//   - *transaction.SignatureData: 签名数据
	//   - error: 签名失败
	//
	// ⚠️ 约束：
	// - 不能修改 tx
	// - 签名必须对 tx 的 Canonical 序列化结果签名
	// - 签名算法必须与 LockingCondition 要求的算法一致
	//
	// 📝 **典型实现**：
	//
	//	func (s *LocalSigner) Sign(ctx context.Context, tx *transaction.Transaction) (*transaction.SignatureData, error) {
	//	    // 1. 计算交易哈希
	//	    txHash := ComputeTxHash(tx)
	//
	//	    // 2. 使用私钥签名
	//	    signature := ecdsa.Sign(s.privateKey, txHash)
	//
	//	    // 3. 返回签名数据
	//	    return &transaction.SignatureData{Value: signature}, nil
	//	}
	Sign(ctx context.Context, tx *transaction.Transaction) (*transaction.SignatureData, error)

	// PublicKey 获取对应的公钥
	//
	// 返回：
	//   - *transaction.PublicKey: 公钥数据
	//   - error: 获取失败
	//
	// 用途：
	// - 构建 UnlockingProof 时需要公钥
	// - 验证签名时需要公钥
	PublicKey() (*transaction.PublicKey, error)

	// Algorithm 返回签名算法
	//
	// 返回：签名算法标识（ECDSA_SECP256K1、ED25519 等）
	//
	// 用途：确保签名算法与 LockingCondition 要求一致
	Algorithm() transaction.SignatureAlgorithm

	// SignBytes 对任意数据签名
	//
	// 🎯 **核心逻辑**：
	// 对任意字节数据进行签名，而不仅限于完整交易。
	// 用于特殊场景如 DelegationProof 签名、消息签名等。
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - data: 待签名的原始数据
	//   - sigHashType: 签名哈希类型（通常使用 SigHashAll）
	//
	// 返回：
	//   - []byte: 签名字节数组
	//   - error: 签名失败
	//
	// ⚠️ 约束：
	//   - data 应该已经是最终的待签名数据（通常是哈希值）
	//   - 签名算法与 Algorithm() 返回的算法一致
	//
	// 📝 **典型实现**：
	//
	//	func (s *LocalSigner) SignBytes(ctx context.Context, data []byte, sigHashType transaction.SighashType) ([]byte, error) {
	//	    // 根据签名算法选择实现
	//	    switch s.algorithm {
	//	    case transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1:
	//	        // 使用 ECDSA 签名
	//	        signature, err := ecdsa.SignASN1(rand.Reader, s.privateKey, data)
	//	        if err != nil {
	//	            return nil, fmt.Errorf("ECDSA签名失败: %w", err)
	//	        }
	//	        return signature, nil
	//	    case transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ED25519:
	//	        // 使用 Ed25519 签名
	//	        signature := ed25519.Sign(s.privateKey, data)
	//	        return signature, nil
	//	    default:
	//	        return nil, fmt.Errorf("不支持的签名算法: %v", s.algorithm)
	//	    }
	//	}
	//
	// 📝 **使用示例**：
	//
	//	// 签名 DelegationProof
	//	proofData := buildDelegationProofData(proof)
	//	signature, err := signer.SignBytes(ctx, proofData, 0)  // 0 = SigHashAll
	SignBytes(ctx context.Context, data []byte) ([]byte, error)
}

// ================================================================================================
// 💰 FeeEstimator（费用估算端口）
// ================================================================================================

// FeeEstimator 费用估算接口
//
// 🎯 **核心职责**：估算交易所需的费用
//
// 💡 **设计理念**：
// 将费用估算逻辑抽象为端口接口，支持多种估算策略的灵活替换。
//
// 🔌 **适配器实现**：
// 1. StaticFeeEstimator: 固定费率（最简单）
// 2. DynamicFeeEstimator: 根据网络拥堵动态调整
// 3. PriorityFeeEstimator: 支持优先级加速
//
// ⚠️ **核心约束**：
// - 不能修改交易
// - 估算结果只是建议，不强制执行
// - 实际费用由 Verifier 检查
//
// 📞 **调用方**：
// - SDK Helper: 转账前估算费用
// - CLI: 显示预估费用
// - Wallet: 余额检查
type FeeEstimator interface {
	// EstimateFee 估算交易费用
	//
	// 🎯 **核心逻辑**：
	// 根据交易大小、网络拥堵、优先级等因素估算合理的费用。
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - tx: 待估算的交易
	//
	// 返回：
	//   - uint64: 建议费用（以 wei 为单位）
	//   - error: 估算失败
	//
	// ⚠️ 注意：
	// - 返回值只是建议，不保证交易一定被接受
	// - 用户可以选择支付更高或更低的费用
	// - 实际费用由 UTXO 差额或 fee_mechanism 决定
	//
	// 📝 **典型实现**：
	//
	//	func (e *StaticFeeEstimator) EstimateFee(ctx context.Context, tx *transaction.Transaction) (uint64, error) {
	//	    // 1. 计算交易大小
	//	    txSize := proto.Size(tx)
	//
	//	    // 2. 根据费率计算费用
	//	    fee := uint64(txSize) * e.feePerByte
	//
	//	    return fee, nil
	//	}
	EstimateFee(ctx context.Context, tx *transaction.Transaction) (uint64, error)

	// EstimateFeeWithPriority 估算带优先级的费用（可选）
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - tx: 待估算的交易
	//   - priority: 优先级（0=normal, 1=high, 2=urgent）
	//
	// 返回：
	//   - uint64: 建议费用
	//   - error: 估算失败
	//
	// 用途：支持用户选择不同的确认速度
	EstimateFeeWithPriority(ctx context.Context, tx *transaction.Transaction, priority uint8) (uint64, error)
}

// ================================================================================================
// 🔑 ProofProvider（证明提供者端口）
// ================================================================================================

// ProofProvider 证明提供者接口
//
// 🎯 **核心职责**：为交易输入生成解锁证明（UnlockingProof）
//
// 💡 **设计理念**：
// 将证明生成逻辑抽象为端口接口，支持多种证明策略的灵活实现。
// 协调多种证明类型（7 种）的生成。
//
// 🔌 **适配器实现**：
// 1. SimpleProofProvider: 为所有 input 使用相同的签名
// 2. MultiProofProvider: 为不同 input 使用不同的签名源
// 3. DelegatedProofProvider: 支持委托授权证明
//
// ⚠️ **核心约束**：
// - 必须为所有 input 生成对应的 proof
// - 生成的 proof 必须匹配 UTXO 的 LockingCondition
// - 不能修改交易的其他部分
//
// 📞 **调用方**：
// - ComposedTx.WithProofs(): Type-state 转换时生成证明
type ProofProvider interface {
	// ProvideProofs 为交易生成所有输入的解锁证明
	//
	// 🎯 **核心逻辑**：
	// 1. 遍历交易的所有 input
	// 2. 获取每个 input 引用的 UTXO
	// 3. 根据 UTXO 的 LockingCondition 类型生成对应的 UnlockingProof
	// 4. 将 proof 填充到 input 的 unlocking_proof 字段
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - tx: 待生成证明的交易（ComposedTx 的底层对象）
	//
	// 返回：
	//   - error: 证明生成失败
	//     • nil: 所有 proof 生成成功
	//     • non-nil: 某个 proof 生成失败
	//
	// ⚠️ 约束：
	// - 必须为所有 input 生成 proof（不能跳过）
	// - 生成的 proof 必须是正确的类型（与 LockingCondition 匹配）
	// - 不能修改 tx 的 inputs/outputs 列表
	//
	// ⚠️ 副作用：
	// - 会修改 tx.inputs[i].unlocking_proof（填充证明）
	// - 这是唯一允许修改交易的地方
	//
	// 📝 **典型实现**：
	//
	//	func (p *SimpleProofProvider) ProvideProofs(ctx context.Context, tx *transaction.Transaction) error {
	//	    for i, input := range tx.Inputs {
	//	        // 1. 获取 UTXO
	//	        utxo, err := p.utxoManager.GetUTXO(ctx, input.PreviousOutput)
	//	        if err != nil {
	//	            return err
	//	        }
	//
	//	        // 2. 根据 LockingCondition 类型生成对应的 proof
	//	        lock := utxo.LockingConditions[0]
	//	        if lock.GetSingleKeyLock() != nil {
	//	            // 生成 SingleKeyProof
	//	            proof := &transaction.SingleKeyProof{
	//	                PublicKey: p.signer.PublicKey(),
	//	                Signature: p.signer.Sign(ctx, tx),
	//	                Algorithm: p.signer.Algorithm(),
	//	            }
	//	            tx.Inputs[i].UnlockingProof = &transaction.UnlockingProof{
	//	                Proof: &transaction.UnlockingProof_SingleKeyProof{
	//	                    SingleKeyProof: proof,
	//	                },
	//	            }
	//	        }
	//	        // ... 处理其他类型的 lock
	//	    }
	//	    return nil
	//	}
	ProvideProofs(ctx context.Context, tx *transaction.Transaction) error
}

// ================================================================================================
// 🎯 端口设计说明
// ================================================================================================

// 设计权衡 1: Signer 是否包含私钥管理
//
// 背景：Signer 接口是否应该负责私钥管理
//
// 备选方案：
// 1. 只签名：Signer 只提供 Sign() - 优势：职责单一 - 劣势：需要额外的密钥管理接口
// 2. 包含管理：Signer 提供 GenerateKey()、ExportKey() 等 - 优势：完整 - 劣势：职责混乱
//
// 选择：只签名
//
// 理由：
// - Signer 是"签名服务"，不是"密钥管理服务"
// - 密钥管理应该由专门的 KeyManager 接口负责
// - 保持接口简洁，遵循单一职责原则
//
// 代价：
// - 密钥管理需要单独的接口
// - 但这是正确的职责分离

// 设计权衡 2: FeeEstimator 是否强制执行
//
// 背景：估算的费用是否应该强制要求
//
// 备选方案：
// 1. 只建议：估算结果只是建议，用户可以自行决定 - 优势：灵活 - 劣势：可能费用不足
// 2. 强制执行：估算结果必须满足，否则拒绝交易 - 优势：安全 - 劣势：不够灵活
//
// 选择：只建议
//
// 理由：
// - 费用估算是"辅助工具"，不是"验证规则"
// - 用户可能有特殊需求（如愿意支付更高费用加速）
// - 实际费用检查由 Verifier 的 Conservation 插件负责
//
// 代价：
// - 用户可能设置过低的费用导致交易被拒绝
// - 但这是用户的选择权

// 设计权衡 3: ProofProvider 是否支持部分证明
//
// 背景：是否允许只为部分 input 生成 proof
//
// 备选方案：
// 1. 全部或无：必须为所有 input 生成 proof - 优势：简单 - 劣势：不够灵活
// 2. 支持部分：可以只为部分 input 生成 proof - 优势：灵活 - 劣势：复杂
//
// 选择：全部或无
//
// 理由：
// - 交易要么所有 input 都有 proof（可以提交），要么没有（不能提交）
// - 部分 proof 没有意义（无法通过验证）
// - 保持简单，避免中间状态
//
// 代价：
// - 如果某个 input 的 proof 生成失败，整个交易失败
// - 但这是合理的（无法提交不完整的交易）

// ================================================================================================
// 🎯 使用示例
// ================================================================================================

// Example_LocalSigner 展示如何实现 LocalSigner
//
// 说明：此函数只是示例，不会被编译运行
func Example_LocalSigner() {
	// type LocalSigner struct {
	// 	privateKey []byte
	// }
	//
	// func (s *LocalSigner) Sign(ctx context.Context, tx *transaction.Transaction) (*transaction.SignatureData, error) {
	// 	// 1. 计算交易哈希
	// 	txHash := ComputeTxHash(tx)
	//
	// 	// 2. 使用私钥签名
	// 	signature := ecdsa.Sign(s.privateKey, txHash)
	//
	// 	// 3. 返回签名数据
	// 	return &transaction.SignatureData{Value: signature}, nil
	// }
	//
	// func (s *LocalSigner) PublicKey() (*transaction.PublicKey, error) {
	// 	pubKey := ecdsa.PublicKeyFromPrivateKey(s.privateKey)
	// 	return &transaction.PublicKey{Value: pubKey}, nil
	// }
	//
	// func (s *LocalSigner) Algorithm() transaction.SignatureAlgorithm {
	// 	return transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1
	// }
}

// Example_SimpleProofProvider 展示如何实现 SimpleProofProvider
//
// 说明：此函数只是示例，不会被编译运行
func Example_SimpleProofProvider() {
	// type SimpleProofProvider struct {
	// 	signer      Signer
	// 	utxoManager repository.UTXOManager
	// }
	//
	// func (p *SimpleProofProvider) ProvideProofs(ctx context.Context, tx *transaction.Transaction) error {
	// 	for i, input := range tx.Inputs {
	// 		// 1. 获取 UTXO
	// 		utxo, err := p.utxoManager.GetUTXO(ctx, input.PreviousOutput)
	// 		if err != nil {
	// 			return fmt.Errorf("failed to get UTXO: %w", err)
	// 		}
	//
	// 		// 2. 根据 LockingCondition 类型生成对应的 proof
	// 		lock := utxo.LockingConditions[0]
	// 		if lock.GetSingleKeyLock() != nil {
	// 			// 生成 SingleKeyProof
	// 			signature, err := p.signer.Sign(ctx, tx)
	// 			if err != nil {
	// 				return fmt.Errorf("failed to sign: %w", err)
	// 			}
	// 			pubKey, err := p.signer.PublicKey()
	// 			if err != nil {
	// 				return fmt.Errorf("failed to get public key: %w", err)
	// 			}
	//
	// 			proof := &transaction.SingleKeyProof{
	// 				PublicKey: pubKey,
	// 				Signature: signature,
	// 				Algorithm: p.signer.Algorithm(),
	// 			}
	// 			tx.Inputs[i].UnlockingProof = &transaction.UnlockingProof{
	// 				Proof: &transaction.UnlockingProof_SingleKeyProof{
	// 					SingleKeyProof: proof,
	// 				},
	// 			}
	// 		}
	// 		// ... 处理其他类型的 lock
	// 	}
	// 	return nil
	// }
}

// ================================================================================================
// 📝 DraftStore（草稿存储端口）
// ================================================================================================

// DraftStore 草稿存储接口
//
// 🎯 **核心职责**：存储和检索交易草稿
//
// 💡 **设计理念**：
// Draft 是 Builder 的辅助工具，用于支持渐进式构建和延迟签名。
// DraftStore 提供草稿的持久化能力，符合六边形架构的"端口/适配器"模式。
//
// 🔌 **适配器实现**：
// 1. MemoryDraftStore: 内存存储（快速，但不持久）
// 2. RedisDraftStore: Redis 存储（分布式，支持 TTL）
// 3. DBDraftStore: 数据库存储（持久化，支持查询）
//
// 🔄 **使用场景**：
//
// **场景 1：ISPC 场景（可选存储）**
//
//	// ISPC 通常不需要持久化草稿，直接在内存中构建
//	draft := builder.CreateDraft(ctx)
//	// ... 渐进式构建 ...
//	composed := draft.Seal()
//
// **场景 2：Off-chain 场景（需要存储）**
//
//	// 创建草稿
//	draft := builder.CreateDraft(ctx)
//	draft.AddInput(...).AddOutput(...)
//
//	// 保存草稿
//	draftID, _ := draftStore.Save(ctx, draft)
//
//	// ... 用户确认 ...
//
//	// 检索草稿
//	draft, _ = draftStore.Get(ctx, draftID)
//	composed := draft.Seal()
//
// ⚠️ **核心约束**：
// - Save() 返回 draftID，用于后续检索
// - Get() 返回的草稿可以继续修改（如果未封闭）
// - Delete() 删除草稿，释放存储空间
// - TTL（可选）：草稿可以设置过期时间
//
// 📞 **调用方**：
// - TxBuilder: CreateDraft()/LoadDraft() 时使用
// - CLI/API: 用户交互式构建交易时使用
type DraftStore interface {
	// Save 保存交易草稿
	//
	// 🎯 **用途**：将草稿持久化，返回唯一 ID
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - draft: 待保存的草稿
	//
	// 返回：
	//   - string: 草稿唯一 ID（用于后续检索）
	//   - error: 保存失败
	//
	// ⚠️ 约束：
	// - draftID 必须全局唯一
	// - 已保存的草稿可以被覆盖（如果 draftID 相同）
	// - 实现应支持并发安全
	//
	// 📝 **典型实现**：
	//
	//	func (s *MemoryDraftStore) Save(ctx context.Context, draft *types.DraftTx) (string, error) {
	//	    draftID := draft.GetDraftID()
	//	    s.mu.Lock()
	//	    defer s.mu.Unlock()
	//	    s.drafts[draftID] = draft
	//	    return draftID, nil
	//	}
	Save(ctx context.Context, draft *types.DraftTx) (string, error)

	// Get 获取交易草稿
	//
	// 🎯 **用途**：通过 draftID 检索已保存的草稿
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - draftID: 草稿唯一 ID
	//
	// 返回：
	//   - *types.DraftTx: 检索到的草稿
	//   - error: 检索失败（如草稿不存在）
	//
	// ⚠️ 约束：
	// - 如果 draftID 不存在，返回 ErrDraftNotFound
	// - 返回的草稿可以继续修改（如果未封闭）
	// - 实现应支持并发安全
	//
	// 📝 **典型实现**：
	//
	//	func (s *MemoryDraftStore) Get(ctx context.Context, draftID string) (*types.DraftTx, error) {
	//	    s.mu.RLock()
	//	    defer s.mu.RUnlock()
	//	    draft, ok := s.drafts[draftID]
	//	    if !ok {
	//	        return nil, ErrDraftNotFound
	//	    }
	//	    return draft, nil
	//	}
	Get(ctx context.Context, draftID string) (*types.DraftTx, error)

	// Delete 删除交易草稿
	//
	// 🎯 **用途**：删除已保存的草稿，释放存储空间
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - draftID: 草稿唯一 ID
	//
	// 返回：
	//   - error: 删除失败
	//
	// ⚠️ 约束：
	// - 如果 draftID 不存在，不报错（幂等操作）
	// - 删除后无法再检索
	// - 实现应支持并发安全
	//
	// 📝 **典型实现**：
	//
	//	func (s *MemoryDraftStore) Delete(ctx context.Context, draftID string) error {
	//	    s.mu.Lock()
	//	    defer s.mu.Unlock()
	//	    delete(s.drafts, draftID)
	//	    return nil
	//	}
	Delete(ctx context.Context, draftID string) error

	// List 列出所有草稿（可选，用于管理界面）
	//
	// 🎯 **用途**：列出指定用户的所有草稿
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - ownerAddress: 所有者地址（可选，nil 表示列出所有）
	//   - limit: 最大返回数量（0 表示无限制）
	//   - offset: 偏移量（分页用）
	//
	// 返回：
	//   - []*types.DraftTx: 草稿列表
	//   - error: 列出失败
	//
	// ⚠️ 约束：
	// - 此方法是可选的，简单实现可以不支持
	// - 返回的草稿按创建时间倒序排列
	// - 实现应支持并发安全
	List(ctx context.Context, ownerAddress []byte, limit, offset int) ([]*types.DraftTx, error)

	// SetTTL 设置草稿过期时间（可选，用于自动清理）
	//
	// 🎯 **用途**：为草稿设置生存时间，过期后自动删除
	//
	// 参数：
	//   - ctx: 上下文对象
	//   - draftID: 草稿唯一 ID
	//   - ttlSeconds: 生存时间（秒）
	//
	// 返回：
	//   - error: 设置失败
	//
	// ⚠️ 约束：
	// - 此方法是可选的，简单实现可以不支持
	// - ttlSeconds=0 表示永不过期
	// - 适用于 Redis 等支持 TTL 的存储
	SetTTL(ctx context.Context, draftID string, ttlSeconds int) error
}

// ================================================================================================
// 🎯 Draft 相关错误定义
// ================================================================================================

// ErrDraftNotFound 草稿未找到错误
//
// 当 DraftStore.Get() 找不到指定的草稿时返回此错误
var ErrDraftNotFound = &DraftError{
	Code:    "DRAFT_NOT_FOUND",
	Message: "draft not found",
}

// ErrDraftAlreadySealed 草稿已封闭错误
//
// 当尝试修改已封闭的草稿时返回此错误
var ErrDraftAlreadySealed = &DraftError{
	Code:    "DRAFT_ALREADY_SEALED",
	Message: "draft is already sealed, cannot modify",
}

// DraftError 草稿相关错误类型
type DraftError struct {
	Code    string // 错误代码
	Message string // 错误消息
	DraftID string // 草稿 ID（可选）
}

// Error 实现 error 接口
func (e *DraftError) Error() string {
	if e.DraftID != "" {
		return e.Code + ": " + e.Message + " (draftID=" + e.DraftID + ")"
	}
	return e.Code + ": " + e.Message
}

// Is 实现 errors.Is 接口
func (e *DraftError) Is(target error) bool {
	t, ok := target.(*DraftError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}
