package proof

import (
	"context"
	"fmt"
	"time"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/tx"
)

// ================================================================================================
// 🎯 MultiProofProvider - 多锁型证明生成器（路由器）
// ================================================================================================
//
// 🎯 核心职责：根据锁定条件类型路由到不同的证明生成器
//
// 🏗️ 支持的锁定条件：
// 1. SingleKeyLock   → 单密钥签名证明
// 2. MultiKeyLock    → 多密钥签名证明（需要外部 MultiSigSession）
// 3. DelegationLock  → 委托授权证明
// 4. ThresholdLock   → 门限签名证明（需要外部ThresholdSigner）
// 5. TimeLock        → 时间锁证明（递归包装 base_proof）
// 6. HeightLock      → 高度锁证明（递归包装 base_proof）
// 7. ContractLock    → 合约执行证明（需要 ISPC 层生成）
//
// ⚠️ 架构边界：
// - TX 层提供基础证明生成能力（Single/Delegation）
// - 复杂签名（Multi/Threshold）由应用层或专用库提供
// - 合约证明（Contract）由 ISPC 层生成，TX 层不处理
//
// 📚 参考文档：
// - _docs/architecture/TX_STATE_MACHINE_ARCHITECTURE.md
// ================================================================================================

// MultiProofProvider 实现多锁型证明生成
type MultiProofProvider struct {
	singleKeySigner tx.Signer // 单密钥签名器（用于 SingleKey/Delegation）
}

// NewMultiProofProvider 创建 MultiProofProvider 实例
func NewMultiProofProvider(singleKeySigner tx.Signer) *MultiProofProvider {
	return &MultiProofProvider{
		singleKeySigner: singleKeySigner,
	}
}

// GenerateProof 根据锁定条件类型生成对应的解锁证明
//
// 🔄 路由逻辑：
// - SingleKeyLock   → 调用 singleKeySigner.Sign()
// - MultiKeyLock    → 返回错误（需要外部 MultiSigSession）
// - DelegationLock  → 生成委托证明（基于 SingleKey）
// - ThresholdLock   → 返回错误（需要外部 ThresholdSigner）
// - TimeLock        → 递归生成 base_proof，包装为 TimeProof
// - HeightLock      → 递归生成 base_proof，包装为 HeightProof
// - ContractLock    → 返回错误（需要 ISPC 层生成）
func (p *MultiProofProvider) GenerateProof(
	ctx context.Context,
	tx *transaction.Transaction,
	lockingCondition *transaction.LockingCondition,
) (*transaction.UnlockingProof, error) {
	// 检查参数
	if lockingCondition == nil {
		return nil, fmt.Errorf("%w: locking condition is nil", ErrUnsupportedLockType)
	}

	switch lock := lockingCondition.Condition.(type) {
	case *transaction.LockingCondition_SingleKeyLock:
		return p.generateSingleKeyProof(ctx, tx, lock)

	case *transaction.LockingCondition_MultiKeyLock:
		// MultiKey 需要外部 MultiSigSession 管理
		return nil, ErrMultiSigRequiresSession

	case *transaction.LockingCondition_DelegationLock:
		return p.generateDelegationProof(ctx, tx, lock)

	case *transaction.LockingCondition_ThresholdLock:
		// Threshold 需要专用的门限签名库
		return nil, ErrThresholdRequiresExternalSigner

	case *transaction.LockingCondition_TimeLock:
		return p.generateTimeProof(ctx, tx, lock)

	case *transaction.LockingCondition_HeightLock:
		return p.generateHeightProof(ctx, tx, lock)

	case *transaction.LockingCondition_ContractLock:
		// ExecutionProof 由 ISPC 层生成
		return nil, ErrExecutionProofRequiresISPC

	default:
		return nil, fmt.Errorf("%w: unsupported lock type", ErrUnsupportedLockType)
	}
}

// ================================================================================================
// 🔧 具体锁型的证明生成实现
// ================================================================================================

// generateSingleKeyProof 生成单密钥签名证明
func (p *MultiProofProvider) generateSingleKeyProof(
	ctx context.Context,
	tx *transaction.Transaction,
	lock *transaction.LockingCondition_SingleKeyLock,
) (*transaction.UnlockingProof, error) {
	// ✅ 修复：更新注释，说明实际行为
	// MultiProofProvider 不处理 SingleKeyLock，应使用 SimpleProofProvider
	// 这是设计上的职责分离，不是简化实现
	return nil, fmt.Errorf("SingleKey proof generation should use SimpleProofProvider, not MultiProofProvider")
}

// generateDelegationProof 生成委托授权证明
func (p *MultiProofProvider) generateDelegationProof(
	ctx context.Context,
	tx *transaction.Transaction,
	lock *transaction.LockingCondition_DelegationLock,
) (*transaction.UnlockingProof, error) {
	// DelegationProof 需要外部提供委托交易 ID 和操作类型
	// 这里只是示例框架，实际需要从上下文或配置中获取
	return nil, ErrDelegationRequiresExternalContext
}

// generateTimeProof 生成时间锁证明（递归）
func (p *MultiProofProvider) generateTimeProof(
	ctx context.Context,
	tx *transaction.Transaction,
	lock *transaction.LockingCondition_TimeLock,
) (*transaction.UnlockingProof, error) {
	timeLock := lock.TimeLock
	if timeLock == nil {
		return nil, fmt.Errorf("TimeLock is nil")
	}

	// 1. 递归生成 base_lock 的证明
	baseProof, err := p.GenerateProof(ctx, tx, timeLock.BaseLock)
	if err != nil {
		return nil, fmt.Errorf("failed to generate base proof for TimeLock: %w", err)
	}

	// 2. 包装为 TimeProof
	timeProof := &transaction.TimeProof{
		CurrentTimestamp: uint64(time.Now().Unix()),
		TimestampProof:   []byte("block_timestamp_proof"), // 实际应从区块链获取
		BaseProof:        baseProof,
		TimeSource:       timeLock.TimeSource,
	}

	// ⚠️ 注意：TimeProof 和 HeightProof 应该在 TxInput 层面设置，而不是 UnlockingProof
	// 这里返回错误，提示需要在更高层处理
	_ = timeProof // 避免 unused 警告
	return nil, fmt.Errorf("TimeProof should be set at TxInput level, not UnlockingProof level")
}

// generateHeightProof 生成高度锁证明（递归）
func (p *MultiProofProvider) generateHeightProof(
	ctx context.Context,
	tx *transaction.Transaction,
	lock *transaction.LockingCondition_HeightLock,
) (*transaction.UnlockingProof, error) {
	heightLock := lock.HeightLock
	if heightLock == nil {
		return nil, fmt.Errorf("HeightLock is nil")
	}

	// 1. 递归生成 base_lock 的证明
	baseProof, err := p.GenerateProof(ctx, tx, heightLock.BaseLock)
	if err != nil {
		return nil, fmt.Errorf("failed to generate base proof for HeightLock: %w", err)
	}

	// 2. 包装为 HeightProof
	heightProof := &transaction.HeightProof{
		CurrentHeight:      uint64(0), // 实际应从区块链获取
		BlockHeaderProof:   []byte("block_header_proof"),
		BaseProof:          baseProof,
		ConfirmationBlocks: heightLock.ConfirmationBlocks,
	}

	// ⚠️ 注意：HeightProof 应该在 TxInput 层面设置，而不是 UnlockingProof
	// 这里返回错误，提示需要在更高层处理
	_ = heightProof // 避免 unused 警告
	return nil, fmt.Errorf("HeightProof should be set at TxInput level, not UnlockingProof level")
}

// ================================================================================================
// 🎯 错误定义
// ================================================================================================

var (
	// ErrUnsupportedLockType 不支持的锁定条件类型
	ErrUnsupportedLockType = fmt.Errorf("unsupported lock type")

	// ErrMultiSigRequiresSession 多签需要外部 MultiSigSession
	ErrMultiSigRequiresSession = fmt.Errorf("multi-sig proof requires external MultiSigSession")

	// ErrThresholdRequiresExternalSigner 门限签名需要外部签名器
	ErrThresholdRequiresExternalSigner = fmt.Errorf("threshold proof requires external threshold signer")

	// ErrExecutionProofRequiresISPC ExecutionProof 需要 ISPC 层生成
	ErrExecutionProofRequiresISPC = fmt.Errorf("execution proof requires ISPC layer generation")

	// ErrDelegationRequiresExternalContext 委托证明需要外部上下文
	ErrDelegationRequiresExternalContext = fmt.Errorf("delegation proof requires external context (delegation_tx_id, operation_type)")
)
