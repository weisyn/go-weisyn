// Package writer 实现资源 UTXO 索引更新器
package writer

import (
	"context"
	"fmt"

	"crypto/sha256"

	"github.com/weisyn/v1/pkg/interfaces/eutxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"google.golang.org/protobuf/proto"
)

// ResourceUTXOIndexUpdater 资源 UTXO 索引更新器
//
// 🎯 **核心职责**：
// 在区块执行完成后，更新资源 UTXO 索引和引用计数。
//
// 💡 **设计理念**：
// - 增量更新：只处理本区块的交易
// - 原子性：在事务中批量更新
// - 幂等性：可以重复执行
type ResourceUTXOIndexUpdater struct {
	storage storage.BadgerStore
	logger  log.Logger
}

// NewResourceUTXOIndexUpdater 创建资源 UTXO 索引更新器
func NewResourceUTXOIndexUpdater(storage storage.BadgerStore, logger log.Logger) *ResourceUTXOIndexUpdater {
	return &ResourceUTXOIndexUpdater{
		storage: storage,
		logger:  logger,
	}
}

// UpdateBlock 更新区块的资源 UTXO 索引
//
// 🎯 **处理流程**：
// 1. 遍历区块中的所有交易
// 2. 处理 ResourceOutput：创建/更新 resource_utxo
// 3. 处理 TxInput.is_reference_only=true：更新引用计数
// 4. 处理 TxInput.is_reference_only=false：标记 UTXO 为 CONSUMED
func (u *ResourceUTXOIndexUpdater) UpdateBlock(ctx context.Context, block *core.Block) error {
	if block == nil {
		return fmt.Errorf("区块不能为空")
	}

	// 在事务中批量更新
	return u.storage.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		// 1. 遍历所有交易
		for _, txProto := range block.Body.Transactions {
			if err := u.processTransaction(ctx, tx, txProto, block.Header.Height, block.Header.Timestamp); err != nil {
				return fmt.Errorf("处理交易失败: %w", err)
			}
		}

		return nil
	})
}

// processTransaction 处理单个交易
func (u *ResourceUTXOIndexUpdater) processTransaction(
	ctx context.Context,
	tx storage.BadgerTransaction,
	txProto *transaction.Transaction,
	blockHeight uint64,
	blockTimestamp uint64,
) error {
	// 计算交易哈希：必须与共识层 TransactionHashService 的算法一致（确定性、排除签名字段）
	txHash, err := u.computeTxHash(txProto)
	if err != nil {
		return fmt.Errorf("计算交易哈希失败: %w", err)
	}

	// 1. 处理输出：创建/更新 ResourceUTXO
	for outputIndex, output := range txProto.Outputs {
		if resourceOutput := output.GetResource(); resourceOutput != nil {
			if err := u.processResourceOutput(ctx, tx, txHash, uint32(outputIndex), output, resourceOutput, blockHeight, blockTimestamp); err != nil {
				return fmt.Errorf("处理 ResourceOutput 失败: %w", err)
			}
		}
	}

	// 2. 处理输入：更新引用计数或标记为 CONSUMED
	for _, input := range txProto.Inputs {
		if err := u.processResourceInput(ctx, tx, input, blockHeight, blockTimestamp); err != nil {
			return fmt.Errorf("处理 ResourceInput 失败: %w", err)
		}
	}

	return nil
}

// processResourceOutput 处理 ResourceOutput
func (u *ResourceUTXOIndexUpdater) processResourceOutput(
	ctx context.Context,
	tx storage.BadgerTransaction,
	txHash []byte,
	outputIndex uint32,
	output *transaction.TxOutput,
	resourceOutput *transaction.ResourceOutput,
	blockHeight uint64,
	blockTimestamp uint64,
) error {
	// 1. 提取资源信息
	resource := resourceOutput.Resource
	if resource == nil {
		return fmt.Errorf("ResourceOutput.resource 不能为空")
	}

	contentHash := resource.ContentHash
	if len(contentHash) != 32 {
		return fmt.Errorf("contentHash 必须是 32 字节，实际: %d", len(contentHash))
	}

	// 2. 创建/更新 ResourceUTXORecord
	record := &eutxo.ResourceUTXORecord{
		ContentHash:       contentHash,
		TxId:              txHash,
		OutputIndex:       outputIndex,
		Owner:             output.Owner,
		Status:            eutxo.ResourceUTXOStatusActive,
		CreationTimestamp: resourceOutput.CreationTimestamp,
		IsImmutable:       resourceOutput.IsImmutable,
	}

	if resourceOutput.ExpiryTimestamp != nil && *resourceOutput.ExpiryTimestamp > 0 {
		expiry := *resourceOutput.ExpiryTimestamp
		record.ExpiryTimestamp = &expiry
		// 检查是否已过期
		if blockTimestamp >= expiry {
			record.Status = eutxo.ResourceUTXOStatusExpired
		}
	}

	// 3. Phase 4：不再写入基于 contentHash 的旧索引（resource:utxo:*, index:resource:owner:*, resource:counters:*）

	if u.logger != nil {
		u.logger.Debugf("✅ 已更新资源 UTXO 索引: contentHash=%x, txHash=%x, outputIndex=%d",
			contentHash[:8], txHash[:8], outputIndex)
	}

	return nil
}

// processResourceInput 处理 ResourceInput
func (u *ResourceUTXOIndexUpdater) processResourceInput(
	ctx context.Context,
	tx storage.BadgerTransaction,
	input *transaction.TxInput,
	blockHeight uint64,
	blockTimestamp uint64,
) error {
	// 1. 查询被引用的 UTXO
	outpoint := input.PreviousOutput
	if outpoint == nil {
		return nil // 跳过无效输入
	}

	// 2. Phase 4：引用计数和状态更新逻辑已迁移到基于实例的索引，不再依赖旧的 resource:utxo:* / resource:counters:* 键
	return nil
}

// computeTxHash 计算交易哈希（与 TransactionHashService.ComputeHash 对齐）
// - 排除输入 unlocking_proof（签名/证明），用于交易ID计算
// - 使用 Deterministic protobuf marshal，保证跨平台一致
func (u *ResourceUTXOIndexUpdater) computeTxHash(txn *transaction.Transaction) ([]byte, error) {
	if txn == nil {
		return nil, fmt.Errorf("transaction is nil")
	}
	// 创建交易副本，排除签名字段（与 TransactionHashService 一致）
	txCopy := proto.Clone(txn).(*transaction.Transaction)
	for _, in := range txCopy.Inputs {
		in.UnlockingProof = nil
	}
	mo := proto.MarshalOptions{Deterministic: true}
	data, err := mo.Marshal(txCopy)
	if err != nil {
		return nil, fmt.Errorf("marshal transaction: %w", err)
	}
	sum := sha256.Sum256(data)
	return sum[:], nil
}

