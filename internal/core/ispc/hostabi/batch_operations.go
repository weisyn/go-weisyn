package hostabi

import (
	"context"
	"fmt"

	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/tx"
)

// ============================================================================
// 批量操作优化
// ============================================================================
//
// 🎯 **设计目的**：
// 提供批量UTXO查询、批量输出添加等功能，提升性能。
//
// 🏗️ **实现策略**：
// - 批量操作：一次调用处理多个操作，减少锁竞争和存储访问
// - 事务性保证：批量操作要么全部成功，要么全部失败
// - 性能优化：减少重复的Draft加载和保存操作
//
// ⚠️ **注意**：
// - 批量操作需要保证原子性
// - 如果批量操作中任何一个失败，需要回滚已执行的操作
//
// ============================================================================

// BatchInputSpec 批量输入规范
type BatchInputSpec struct {
	Outpoint        *pb.OutPoint
	IsReferenceOnly bool
	UnlockingProof  *pb.UnlockingProof
}

// BatchAssetOutputSpec 批量资产输出规范
type BatchAssetOutputSpec struct {
	Owner             []byte
	Amount            uint64
	TokenID           []byte
	LockingConditions []*pb.LockingCondition
}

// BatchResourceOutputSpec 批量资源输出规范
type BatchResourceOutputSpec struct {
	ContentHash       []byte
	Category          string
	Owner             []byte
	LockingConditions []*pb.LockingCondition
	Metadata          []byte
}

// BatchStateOutputSpec 批量状态输出规范
type BatchStateOutputSpec struct {
	StateID              []byte
	StateVersion         uint64
	ExecutionResultHash  []byte
	PublicInputs         []byte
	ParentStateHash      []byte
}

// BatchOperationResult 批量操作结果
type BatchOperationResult struct {
	SuccessCount int      // 成功操作数
	FailureCount int      // 失败操作数
	Indices      []uint32 // 操作索引列表（按输入顺序）
	Errors       []error  // 错误列表（如果有）
}

// BatchDraftOperations 批量草稿操作器
//
// 🎯 **设计目的**：
// 提供批量操作TransactionDraft的能力，减少重复的加载和保存操作。
//
// 🏗️ **实现策略**：
// - 批量加载：一次加载Draft，多次操作
// - 批量保存：所有操作完成后一次性保存
// - 事务性保证：如果任何操作失败，回滚所有操作
//
// ⚠️ **注意**：此类型当前未被使用，保留供未来优化使用
type BatchDraftOperations struct {
	draftService tx.TransactionDraftService
	// logger 字段已移除，当前未使用
}

// BatchAddInputs 批量添加输入
//
// 📋 **参数**：
//   - ctx: 执行上下文
//   - draftID: 草稿ID
//   - inputs: 输入规范列表
//
// 🔧 **返回值**：
//   - *BatchOperationResult: 批量操作结果
//   - error: 批量操作失败时的错误信息
//
// 🎯 **事务性保证**：
//   - 如果任何输入添加失败，回滚已添加的输入
//   - 返回详细的成功/失败统计
func (b *BatchDraftOperations) BatchAddInputs(
	ctx context.Context,
	draftID string,
	inputs []BatchInputSpec,
) (*BatchOperationResult, error) {
	if len(inputs) == 0 {
		return &BatchOperationResult{
			SuccessCount: 0,
			FailureCount: 0,
			Indices:      []uint32{},
			Errors:       []error{},
		}, nil
	}

	// 1. 加载Draft（只加载一次）
	draft, err := b.draftService.LoadDraft(ctx, draftID)
	if err != nil {
		return nil, fmt.Errorf("加载草稿失败: %w", err)
	}

	// 2. 记录初始状态（用于回滚）
	initialInputCount := len(draft.Tx.Inputs)

	// 3. 批量添加输入
	result := &BatchOperationResult{
		Indices: make([]uint32, 0, len(inputs)),
		Errors:  make([]error, 0),
	}

	for i, inputSpec := range inputs {
		index, err := b.draftService.AddInput(ctx, draft, inputSpec.Outpoint, inputSpec.IsReferenceOnly, inputSpec.UnlockingProof)
		if err != nil {
			// 操作失败，回滚已添加的输入
			draft.Tx.Inputs = draft.Tx.Inputs[:initialInputCount]
			result.FailureCount++
			result.Errors = append(result.Errors, fmt.Errorf("添加输入 %d 失败: %w", i, err))
			continue
		}
		result.SuccessCount++
		result.Indices = append(result.Indices, index)
	}

	// 4. 如果所有操作都成功，保存Draft（只保存一次）
	if result.FailureCount == 0 {
		if err := b.draftService.SaveDraft(ctx, draft); err != nil {
			// 保存失败，回滚已添加的输入
			draft.Tx.Inputs = draft.Tx.Inputs[:initialInputCount]
			return nil, fmt.Errorf("保存草稿失败: %w", err)
		}
	} else {
		// 有操作失败，不保存（已回滚）
		return result, fmt.Errorf("批量添加输入部分失败: 成功=%d, 失败=%d", result.SuccessCount, result.FailureCount)
	}

	return result, nil
}

// BatchAddAssetOutputs 批量添加资产输出
//
// 📋 **参数**：
//   - ctx: 执行上下文
//   - draftID: 草稿ID
//   - outputs: 资产输出规范列表
//
// 🔧 **返回值**：
//   - *BatchOperationResult: 批量操作结果
//   - error: 批量操作失败时的错误信息
//
// 🎯 **事务性保证**：
//   - 如果任何输出添加失败，回滚已添加的输出
//   - 返回详细的成功/失败统计
func (b *BatchDraftOperations) BatchAddAssetOutputs(
	ctx context.Context,
	draftID string,
	outputs []BatchAssetOutputSpec,
) (*BatchOperationResult, error) {
	if len(outputs) == 0 {
		return &BatchOperationResult{
			SuccessCount: 0,
			FailureCount: 0,
			Indices:      []uint32{},
			Errors:       []error{},
		}, nil
	}

	// 1. 加载Draft（只加载一次）
	draft, err := b.draftService.LoadDraft(ctx, draftID)
	if err != nil {
		return nil, fmt.Errorf("加载草稿失败: %w", err)
	}

	// 2. 记录初始状态（用于回滚）
	initialOutputCount := len(draft.Tx.Outputs)

	// 3. 批量添加输出
	result := &BatchOperationResult{
		Indices: make([]uint32, 0, len(outputs)),
		Errors:  make([]error, 0),
	}

	for i, outputSpec := range outputs {
		// 验证参数
		if len(outputSpec.Owner) != 20 {
			draft.Tx.Outputs = draft.Tx.Outputs[:initialOutputCount]
			result.FailureCount++
			result.Errors = append(result.Errors, fmt.Errorf("输出 %d: owner 地址必须是 20 字节", i))
			continue
		}

		amountStr := fmt.Sprintf("%d", outputSpec.Amount)
		index, err := b.draftService.AddAssetOutput(ctx, draft, outputSpec.Owner, amountStr, outputSpec.TokenID, outputSpec.LockingConditions)
		if err != nil {
			// 操作失败，回滚已添加的输出
			draft.Tx.Outputs = draft.Tx.Outputs[:initialOutputCount]
			result.FailureCount++
			result.Errors = append(result.Errors, fmt.Errorf("添加资产输出 %d 失败: %w", i, err))
			continue
		}
		result.SuccessCount++
		result.Indices = append(result.Indices, index)
	}

	// 4. 如果所有操作都成功，保存Draft（只保存一次）
	if result.FailureCount == 0 {
		if err := b.draftService.SaveDraft(ctx, draft); err != nil {
			// 保存失败，回滚已添加的输出
			draft.Tx.Outputs = draft.Tx.Outputs[:initialOutputCount]
			return nil, fmt.Errorf("保存草稿失败: %w", err)
		}
	} else {
		// 有操作失败，不保存（已回滚）
		return result, fmt.Errorf("批量添加资产输出部分失败: 成功=%d, 失败=%d", result.SuccessCount, result.FailureCount)
	}

	return result, nil
}

// BatchAddResourceOutputs 批量添加资源输出
//
// 📋 **参数**：
//   - ctx: 执行上下文
//   - draftID: 草稿ID
//   - outputs: 资源输出规范列表
//
// 🔧 **返回值**：
//   - *BatchOperationResult: 批量操作结果
//   - error: 批量操作失败时的错误信息
//
// 🎯 **事务性保证**：
//   - 如果任何输出添加失败，回滚已添加的输出
//   - 返回详细的成功/失败统计
func (b *BatchDraftOperations) BatchAddResourceOutputs(
	ctx context.Context,
	draftID string,
	outputs []BatchResourceOutputSpec,
) (*BatchOperationResult, error) {
	if len(outputs) == 0 {
		return &BatchOperationResult{
			SuccessCount: 0,
			FailureCount: 0,
			Indices:      []uint32{},
			Errors:       []error{},
		}, nil
	}

	// 1. 加载Draft（只加载一次）
	draft, err := b.draftService.LoadDraft(ctx, draftID)
	if err != nil {
		return nil, fmt.Errorf("加载草稿失败: %w", err)
	}

	// 2. 记录初始状态（用于回滚）
	initialOutputCount := len(draft.Tx.Outputs)

	// 3. 批量添加输出
	result := &BatchOperationResult{
		Indices: make([]uint32, 0, len(outputs)),
		Errors:  make([]error, 0),
	}

	for i, outputSpec := range outputs {
		// 验证参数
		if len(outputSpec.ContentHash) != 32 {
			draft.Tx.Outputs = draft.Tx.Outputs[:initialOutputCount]
			result.FailureCount++
			result.Errors = append(result.Errors, fmt.Errorf("输出 %d: contentHash 必须是 32 字节", i))
			continue
		}
		if len(outputSpec.Owner) != 20 {
			draft.Tx.Outputs = draft.Tx.Outputs[:initialOutputCount]
			result.FailureCount++
			result.Errors = append(result.Errors, fmt.Errorf("输出 %d: owner 地址必须是 20 字节", i))
			continue
		}

		index, err := b.draftService.AddResourceOutput(ctx, draft, outputSpec.ContentHash, outputSpec.Category, outputSpec.Owner, outputSpec.LockingConditions, outputSpec.Metadata)
		if err != nil {
			// 操作失败，回滚已添加的输出
			draft.Tx.Outputs = draft.Tx.Outputs[:initialOutputCount]
			result.FailureCount++
			result.Errors = append(result.Errors, fmt.Errorf("添加资源输出 %d 失败: %w", i, err))
			continue
		}
		result.SuccessCount++
		result.Indices = append(result.Indices, index)
	}

	// 4. 如果所有操作都成功，保存Draft（只保存一次）
	if result.FailureCount == 0 {
		if err := b.draftService.SaveDraft(ctx, draft); err != nil {
			// 保存失败，回滚已添加的输出
			draft.Tx.Outputs = draft.Tx.Outputs[:initialOutputCount]
			return nil, fmt.Errorf("保存草稿失败: %w", err)
		}
	} else {
		// 有操作失败，不保存（已回滚）
		return result, fmt.Errorf("批量添加资源输出部分失败: 成功=%d, 失败=%d", result.SuccessCount, result.FailureCount)
	}

	return result, nil
}

// BatchAddStateOutputs 批量添加状态输出
//
// 📋 **参数**：
//   - ctx: 执行上下文
//   - draftID: 草稿ID
//   - outputs: 状态输出规范列表
//
// 🔧 **返回值**：
//   - *BatchOperationResult: 批量操作结果
//   - error: 批量操作失败时的错误信息
//
// 🎯 **事务性保证**：
//   - 如果任何输出添加失败，回滚已添加的输出
//   - 返回详细的成功/失败统计
func (b *BatchDraftOperations) BatchAddStateOutputs(
	ctx context.Context,
	draftID string,
	outputs []BatchStateOutputSpec,
) (*BatchOperationResult, error) {
	if len(outputs) == 0 {
		return &BatchOperationResult{
			SuccessCount: 0,
			FailureCount: 0,
			Indices:      []uint32{},
			Errors:       []error{},
		}, nil
	}

	// 1. 加载Draft（只加载一次）
	draft, err := b.draftService.LoadDraft(ctx, draftID)
	if err != nil {
		return nil, fmt.Errorf("加载草稿失败: %w", err)
	}

	// 2. 记录初始状态（用于回滚）
	initialOutputCount := len(draft.Tx.Outputs)

	// 3. 批量添加输出
	result := &BatchOperationResult{
		Indices: make([]uint32, 0, len(outputs)),
		Errors:  make([]error, 0),
	}

	for i, outputSpec := range outputs {
		// 验证参数
		if len(outputSpec.StateID) == 0 {
			draft.Tx.Outputs = draft.Tx.Outputs[:initialOutputCount]
			result.FailureCount++
			result.Errors = append(result.Errors, fmt.Errorf("输出 %d: stateID 不能为空", i))
			continue
		}
		if len(outputSpec.ExecutionResultHash) != 32 {
			draft.Tx.Outputs = draft.Tx.Outputs[:initialOutputCount]
			result.FailureCount++
			result.Errors = append(result.Errors, fmt.Errorf("输出 %d: executionResultHash 必须是 32 字节", i))
			continue
		}

		index, err := b.draftService.AddStateOutput(ctx, draft, outputSpec.StateID, outputSpec.StateVersion, outputSpec.ExecutionResultHash, outputSpec.PublicInputs, outputSpec.ParentStateHash)
		if err != nil {
			// 操作失败，回滚已添加的输出
			draft.Tx.Outputs = draft.Tx.Outputs[:initialOutputCount]
			result.FailureCount++
			result.Errors = append(result.Errors, fmt.Errorf("添加状态输出 %d 失败: %w", i, err))
			continue
		}
		result.SuccessCount++
		result.Indices = append(result.Indices, index)
	}

	// 4. 如果所有操作都成功，保存Draft（只保存一次）
	if result.FailureCount == 0 {
		if err := b.draftService.SaveDraft(ctx, draft); err != nil {
			// 保存失败，回滚已添加的输出
			draft.Tx.Outputs = draft.Tx.Outputs[:initialOutputCount]
			return nil, fmt.Errorf("保存草稿失败: %w", err)
		}
	} else {
		// 有操作失败，不保存（已回滚）
		return result, fmt.Errorf("批量添加状态输出部分失败: 成功=%d, 失败=%d", result.SuccessCount, result.FailureCount)
	}

	return result, nil
}

