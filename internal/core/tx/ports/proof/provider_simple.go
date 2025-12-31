// Package proof 提供 ProofProvider 端口的实现
//
// 本包实现 Hexagonal Architecture 中的适配器层，负责生成交易解锁证明。
package proof

import (
	"context"
	"fmt"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/interfaces/tx"
)

// SimpleProofProvider 简单证明提供者
//
// 🎯 **核心职责**：为 SingleKeyLock 生成对应的 SingleKeyProof
//
// 💡 **设计理念**：
// 在 P1 MVP 阶段，只支持最简单的单签场景（SingleKeyLock），为交易的所有输入
// 使用同一个私钥签名。更复杂的场景（多签、合约锁等）在后续阶段实现。
//
// ⚠️ **P1 约束**：
// - 只处理 SingleKeyLock，其他锁定条件返回错误
// - 为所有输入使用同一个 Signer（相同密钥）
// - 假设所有输入都属于同一个所有者
//
// 📞 **调用方**：
// - ComposedTx.WithProofs(): Type-state 转换时使用
type SimpleProofProvider struct {
	signer  tx.Signer
	utxoMgr persistence.UTXOQuery
}

// NewSimpleProofProvider 创建新的 SimpleProofProvider
//
// 参数：
//   - signer: 签名服务（提供私钥签名能力）
//   - utxoMgr: UTXO 管理器（查询输入引用的 UTXO）
//
// 返回：
//   - *SimpleProofProvider: 新创建的实例
func NewSimpleProofProvider(
	signer tx.Signer,
	utxoMgr persistence.UTXOQuery,
) *SimpleProofProvider {
	return &SimpleProofProvider{
		signer:  signer,
		utxoMgr: utxoMgr,
	}
}

// ProvideProofs 为交易的所有输入生成解锁证明
//
// 🎯 **核心逻辑**：
// 1. 遍历交易的所有输入
// 2. 通过 UTXOManager 获取每个输入引用的 UTXO
// 3. 检查 UTXO 的 LockingCondition 类型
// 4. 如果是 SingleKeyLock，生成 SingleKeyProof
// 5. 将生成的 proof 填充到输入的 unlocking_proof 字段
//
// ⚠️ **P1 约束**：
// - 只处理 SingleKeyLock → SingleKeyProof
// - 其他锁定条件返回 unsupported 错误
// - 所有输入必须使用同一个 Signer
//
// ⚠️ **副作用**：
// - 会修改 tx.Inputs[i].UnlockingProof（填充证明）
// - 这是唯一允许修改交易的地方
//
// 参数：
//   - ctx: 上下文对象
//   - tx: 待生成证明的交易
//
// 返回：
//   - error: 证明生成失败
//   - nil: 所有 proof 生成成功
//   - non-nil: 某个 proof 生成失败
func (p *SimpleProofProvider) ProvideProofs(ctx context.Context, tx *transaction.Transaction) error {
	// 0. 检查参数
	if tx == nil {
		return fmt.Errorf("交易不能为空")
	}
	if len(tx.Inputs) == 0 {
		// 没有输入的交易（如 Coinbase）不需要生成证明
		return nil
	}

	// 1. 为每个输入生成证明
	for i, input := range tx.Inputs {
		if err := p.generateProofForInput(ctx, tx, i, input); err != nil {
			return fmt.Errorf("为输入 %d 生成证明失败: %w", i, err)
		}
	}

	return nil
}

// generateProofForInput 为单个输入生成证明
//
// 参数：
//   - ctx: 上下文对象
//   - tx: 当前交易
//   - index: 输入索引
//   - input: 待生成证明的输入
//
// 返回：
//   - error: 生成失败
func (p *SimpleProofProvider) generateProofForInput(
	ctx context.Context,
	tx *transaction.Transaction,
	index int,
	input *transaction.TxInput,
) error {
	// 1. 获取输入引用的 UTXO
	utxo, err := p.utxoMgr.GetUTXO(ctx, input.PreviousOutput)
	if err != nil {
		return fmt.Errorf("获取 UTXO 失败: %w", err)
	}

	// 2. 提取 TxOutput（使用 CachedOutput）
	txOutput := utxo.GetCachedOutput()
	if txOutput == nil {
		return fmt.Errorf("UTXO 没有缓存的 TxOutput（仅支持 CachedOutput 策略）")
	}
	if len(txOutput.LockingConditions) == 0 {
		return fmt.Errorf("TxOutput 没有任何锁定条件")
	}

	// 3. 获取第一个锁定条件（P1 只处理单条件）
	lockingCondition := txOutput.LockingConditions[0]

	// 4. 根据锁定条件类型生成对应的证明并填充
	if err := p.generateAndFillProof(ctx, tx, index, lockingCondition); err != nil {
		return err
	}

	return nil
}

// generateAndFillProof 根据锁定条件类型生成对应的证明并填充到输入
//
// 参数：
//   - ctx: 上下文对象
//   - tx: 当前交易
//   - index: 输入索引
//   - lock: 锁定条件
//
// 返回：
//   - error: 生成或填充失败
func (p *SimpleProofProvider) generateAndFillProof(
	ctx context.Context,
	tx *transaction.Transaction,
	index int,
	lock *transaction.LockingCondition,
) error {
	// 检查锁定条件类型
	switch lock.Condition.(type) {
	case *transaction.LockingCondition_SingleKeyLock:
		// 生成 SingleKeyProof 并直接填充
		proof, err := p.generateSingleKeyProof(ctx, tx, lock.GetSingleKeyLock())
		if err != nil {
			return err
		}
		tx.Inputs[index].UnlockingProof = proof
		return nil

	default:
		// P1 阶段不支持其他类型
		return fmt.Errorf("P1 阶段不支持的锁定条件类型: %T", lock.Condition)
	}
}

// generateSingleKeyProof 生成 SingleKeyProof
//
// 🎯 **核心逻辑**：
// 1. 使用 Signer 对交易签名（Signer 内部使用 HashCanonicalizer 计算规范化哈希）
// 2. 获取公钥和算法
// 3. 构建 SingleKeyProof
//
// ⚠️ **重要**：
// - Signer.Sign() 内部已使用 HashCanonicalizer.ComputeTransactionHash()
// - 这确保了签名哈希正确排除了 unlocking_proof 中的 signature 字段
// - SIGHASH 类型默认使用 SIGHASH_ALL（签名所有输入和输出）
//
// 参数：
//   - ctx: 上下文对象
//   - tx: 待签名的交易
//   - singleKeyLock: SingleKeyLock 配置
//
// 返回：
//   - *transaction.TxInput_SingleKeyProof: 生成的 SingleKeyProof（实现 isTxInput_UnlockingProof）
//   - error: 生成失败
func (p *SimpleProofProvider) generateSingleKeyProof(
	ctx context.Context,
	tx *transaction.Transaction,
	singleKeyLock *transaction.SingleKeyLock,
) (*transaction.TxInput_SingleKeyProof, error) {
	// 1. 使用 Signer 对交易签名（内部使用规范化哈希）
	signature, err := p.signer.Sign(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("签名失败: %w", err)
	}

	// 2. 获取公钥
	pubKey, err := p.signer.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("获取公钥失败: %w", err)
	}

	// 3. 获取签名算法
	algorithm := p.signer.Algorithm()

	// 4. 构建 SingleKeyProof
	singleKeyProof := &transaction.SingleKeyProof{
		Signature: signature,
		PublicKey: pubKey,
		Algorithm: algorithm,
		// SighashType 使用默认值 SIGHASH_ALL
		SighashType: transaction.SignatureHashType_SIGHASH_ALL,
	}

	// 5. 包装为 TxInput_SingleKeyProof（实现 isTxInput_UnlockingProof 接口）
	return &transaction.TxInput_SingleKeyProof{
		SingleKeyProof: singleKeyProof,
	}, nil
}

// 编译期检查：确保 SimpleProofProvider 实现了 tx.ProofProvider 接口
var _ tx.ProofProvider = (*SimpleProofProvider)(nil)
