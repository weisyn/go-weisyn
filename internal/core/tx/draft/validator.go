package draft

import (
	"context"
	"fmt"
	"math/big"

	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/types"
)

// ============================================================================
// 验证增强
// ============================================================================
//
// 🎯 **设计目的**：
// 增强输入参数验证和边界检查，实现防御性编程。
//
// 🏗️ **实现策略**：
// - 输入参数验证：验证所有输入参数的有效性
// - 边界条件检查：检查数值边界、长度限制等
// - 防御性编程：在关键操作前进行验证
//
// ⚠️ **注意**：
// - 验证不包含业务逻辑验证（由验证层负责）
// - 验证专注于数据格式和边界检查
//
// ============================================================================

// ValidationError 验证错误
type ValidationError struct {
	Field   string // 字段名
	Message string // 错误消息
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("验证失败 [%s]: %s", e.Field, e.Message)
}

// ValidationResult 验证结果
type ValidationResult struct {
	Valid   bool            // 是否有效
	Errors  []ValidationError // 错误列表
	Warnings []string       // 警告列表（非致命问题）
}

// NewValidationResult 创建验证结果
func NewValidationResult() *ValidationResult {
	return &ValidationResult{
		Valid:    true,
		Errors:   make([]ValidationError, 0),
		Warnings: make([]string, 0),
	}
}

// AddError 添加错误
func (r *ValidationResult) AddError(field, message string) {
	r.Valid = false
	r.Errors = append(r.Errors, ValidationError{
		Field:   field,
		Message: message,
	})
}

// AddWarning 添加警告
func (r *ValidationResult) AddWarning(message string) {
	r.Warnings = append(r.Warnings, message)
}

// Error 返回错误消息
func (r *ValidationResult) Error() string {
	if r.Valid {
		return ""
	}
	msg := "验证失败:\n"
	for _, err := range r.Errors {
		msg += fmt.Sprintf("  - %s: %s\n", err.Field, err.Message)
	}
	return msg
}

// DraftValidator Draft验证器
type DraftValidator struct {
	// 配置参数
	maxInputs      int // 最大输入数（0表示无限制）
	maxOutputs     int // 最大输出数（0表示无限制）
	maxDraftSize   int // 最大草稿大小（字节，0表示无限制）
	enableWarnings bool // 是否启用警告
}

// NewDraftValidator 创建Draft验证器
func NewDraftValidator() *DraftValidator {
	return &DraftValidator{
		maxInputs:      1000, // 默认最大1000个输入
		maxOutputs:     1000, // 默认最大1000个输出
		maxDraftSize:   1024 * 1024, // 默认最大1MB
		enableWarnings: true,
	}
}

// NewDraftValidatorWithConfig 创建带配置的Draft验证器
func NewDraftValidatorWithConfig(maxInputs, maxOutputs, maxDraftSize int, enableWarnings bool) *DraftValidator {
	return &DraftValidator{
		maxInputs:      maxInputs,
		maxOutputs:     maxOutputs,
		maxDraftSize:   maxDraftSize,
		enableWarnings: enableWarnings,
	}
}

// ValidateDraft 验证草稿的基本有效性（增强版）
func (v *DraftValidator) ValidateDraft(ctx context.Context, draft *types.DraftTx) *ValidationResult {
	result := NewValidationResult()

	// 1. 基本空值检查
	if draft == nil {
		result.AddError("draft", "草稿不能为 nil")
		return result
	}

	if draft.Tx == nil {
		result.AddError("draft.Tx", "草稿的 Tx 不能为 nil")
		return result
	}

	// 2. DraftID验证
	if draft.DraftID == "" {
		result.AddError("draft.DraftID", "草稿 ID 不能为空")
	}

	// 3. Nonce验证
	if draft.Tx.Nonce == 0 {
		result.AddError("draft.Tx.Nonce", "交易 Nonce 不能为 0")
	}

	// 4. 输入数量边界检查
	if v.maxInputs > 0 && len(draft.Tx.Inputs) > v.maxInputs {
		result.AddError("draft.Tx.Inputs", fmt.Sprintf("输入数量超过限制: %d > %d", len(draft.Tx.Inputs), v.maxInputs))
	}

	// 5. 输出数量边界检查
	if v.maxOutputs > 0 && len(draft.Tx.Outputs) > v.maxOutputs {
		result.AddError("draft.Tx.Outputs", fmt.Sprintf("输出数量超过限制: %d > %d", len(draft.Tx.Outputs), v.maxOutputs))
	}

	// 6. 输入验证
	for i, input := range draft.Tx.Inputs {
		if err := v.validateInput(input, i); err != nil {
			result.AddError(fmt.Sprintf("draft.Tx.Inputs[%d]", i), err.Error())
		}
	}

	// 7. 输出验证
	for i, output := range draft.Tx.Outputs {
		if err := v.validateOutput(output, i); err != nil {
			result.AddError(fmt.Sprintf("draft.Tx.Outputs[%d]", i), err.Error())
		}
	}

	// 8. 警告检查（非致命）
	if v.enableWarnings {
		if len(draft.Tx.Inputs) == 0 && len(draft.Tx.Outputs) == 0 {
			result.AddWarning("草稿为空：没有输入和输出")
		}
		if len(draft.Tx.Inputs) > 100 {
			result.AddWarning(fmt.Sprintf("输入数量较多: %d，可能影响性能", len(draft.Tx.Inputs)))
		}
		if len(draft.Tx.Outputs) > 100 {
			result.AddWarning(fmt.Sprintf("输出数量较多: %d，可能影响性能", len(draft.Tx.Outputs)))
		}
	}

	return result
}

// validateInput 验证输入
func (v *DraftValidator) validateInput(input *pb.TxInput, index int) error {
	if input == nil {
		return fmt.Errorf("输入不能为 nil")
	}

	if input.PreviousOutput == nil {
		return fmt.Errorf("PreviousOutput 不能为 nil")
	}

	// PreviousOutput验证
	if len(input.PreviousOutput.TxId) == 0 {
		return fmt.Errorf("PreviousOutput.TxId 不能为空")
	}

	if len(input.PreviousOutput.TxId) != 32 {
		return fmt.Errorf("PreviousOutput.TxId 必须是 32 字节，实际: %d 字节", len(input.PreviousOutput.TxId))
	}

	// UnlockingProof验证（如果存在）
	if input.UnlockingProof != nil {
		// UnlockingProof是oneof类型，至少需要有一个字段被设置
		// 这里只检查是否设置了，具体验证由验证层负责
	}

	return nil
}

// validateOutput 验证输出
func (v *DraftValidator) validateOutput(output *pb.TxOutput, index int) error {
	if output == nil {
		return fmt.Errorf("输出不能为 nil")
	}

	// 检查输出类型（oneof output_content）
	hasAsset := output.GetAsset() != nil
	hasResource := output.GetResource() != nil
	hasState := output.GetState() != nil

	count := 0
	if hasAsset {
		count++
	}
	if hasResource {
		count++
	}
	if hasState {
		count++
	}

	if count == 0 {
		return fmt.Errorf("输出必须包含 asset、resource 或 state 之一")
	}

	if count > 1 {
		return fmt.Errorf("输出只能包含 asset、resource 或 state 之一，不能同时包含多个")
	}

	// 根据类型验证
	if hasAsset {
		return v.validateAssetOutput(output.GetAsset(), index)
	}
	if hasResource {
		return v.validateResourceOutput(output.GetResource(), index)
	}
	if hasState {
		return v.validateStateOutput(output.GetState(), index)
	}

	return nil
}

// validateAssetOutput 验证资产输出
func (v *DraftValidator) validateAssetOutput(output *pb.AssetOutput, index int) error {
	if output == nil {
		return fmt.Errorf("AssetOutput 不能为 nil")
	}

	// 检查AssetContent（oneof）
	hasNativeCoin := output.GetNativeCoin() != nil
	hasContractToken := output.GetContractToken() != nil

	if !hasNativeCoin && !hasContractToken {
		return fmt.Errorf("AssetOutput 必须包含 NativeCoin 或 ContractToken")
	}

	if hasNativeCoin && hasContractToken {
		return fmt.Errorf("AssetOutput 不能同时包含 NativeCoin 和 ContractToken")
	}

	// 验证NativeCoin
	if hasNativeCoin {
		nativeCoin := output.GetNativeCoin()
		if nativeCoin.Amount == "" {
			return fmt.Errorf("NativeCoin.Amount 不能为空")
		}
		amountBig, ok := new(big.Int).SetString(nativeCoin.Amount, 10)
		if !ok {
			return fmt.Errorf("NativeCoin.Amount 不是有效的数字: %s", nativeCoin.Amount)
		}
		if amountBig.Sign() <= 0 {
			return fmt.Errorf("NativeCoin.Amount 必须大于 0，实际: %s", nativeCoin.Amount)
		}
	}

	// 验证ContractToken
	if hasContractToken {
		contractToken := output.GetContractToken()
		if len(contractToken.ContractAddress) == 0 {
			return fmt.Errorf("contractToken.contractAddress 不能为空")
		}
		if len(contractToken.ContractAddress) != 20 {
			return fmt.Errorf("contractToken.contractAddress 必须是 20 字节，实际: %d 字节", len(contractToken.ContractAddress))
		}
		if contractToken.Amount == "" {
			return fmt.Errorf("contractToken.amount 不能为空")
		}
		amountBig, ok := new(big.Int).SetString(contractToken.Amount, 10)
		if !ok {
			return fmt.Errorf("contractToken.amount 不是有效的数字: %s", contractToken.Amount)
		}
		if amountBig.Sign() <= 0 {
			return fmt.Errorf("contractToken.amount 必须大于 0，实际: %s", contractToken.Amount)
		}
	}

	return nil
}

// validateResourceOutput 验证资源输出
func (v *DraftValidator) validateResourceOutput(output *pb.ResourceOutput, index int) error {
	if output == nil {
		return fmt.Errorf("ResourceOutput 不能为 nil")
	}

	// Resource验证
	if output.Resource == nil {
		return fmt.Errorf("ResourceOutput.Resource 不能为 nil")
	}

	// ContentHash验证（从Resource中获取）
	if len(output.Resource.ContentHash) == 0 {
		return fmt.Errorf("Resource.ContentHash 不能为空")
	}

	if len(output.Resource.ContentHash) != 32 {
		return fmt.Errorf("Resource.ContentHash 必须是 32 字节，实际: %d 字节", len(output.Resource.ContentHash))
	}

	// Category验证（从Resource中获取，是枚举类型）
	if output.Resource.Category == 0 {
		return fmt.Errorf("Resource.Category 不能为 UNKNOWN")
	}

	return nil
}

// validateStateOutput 验证状态输出
func (v *DraftValidator) validateStateOutput(output *pb.StateOutput, index int) error {
	if output == nil {
		return fmt.Errorf("StateOutput 不能为 nil")
	}

	// StateId验证
	if len(output.StateId) == 0 {
		return fmt.Errorf("StateId 不能为空")
	}

	if len(output.StateId) > 256 {
		return fmt.Errorf("StateId 长度不能超过 256 字节，实际: %d 字节", len(output.StateId))
	}

	// ExecutionResultHash验证
	if len(output.ExecutionResultHash) == 0 {
		return fmt.Errorf("ExecutionResultHash 不能为空")
	}

	if len(output.ExecutionResultHash) != 32 {
		return fmt.Errorf("ExecutionResultHash 必须是 32 字节，实际: %d 字节", len(output.ExecutionResultHash))
	}

	// PublicInputs验证（可选，从ZKStateProof中获取）
	if output.ZkProof != nil && output.ZkProof.PublicInputs != nil {
		for i, publicInput := range output.ZkProof.PublicInputs {
			if len(publicInput) > 1024*1024 {
				return fmt.Errorf("PublicInputs[%d] 大小不能超过 1MB，实际: %d 字节", i, len(publicInput))
			}
		}
	}

	// ParentStateHash验证（可选）
	if output.ParentStateHash != nil && len(output.ParentStateHash) != 32 {
		return fmt.Errorf("ParentStateHash 必须是 32 字节（如果提供），实际: %d 字节", len(output.ParentStateHash))
	}

	return nil
}

// ValidateOutpoint 验证Outpoint
func (v *DraftValidator) ValidateOutpoint(outpoint *pb.OutPoint) error {
	if outpoint == nil {
		return fmt.Errorf("outpoint 不能为 nil")
	}

	if len(outpoint.TxId) == 0 {
		return fmt.Errorf("outpoint.txId 不能为空")
	}

	if len(outpoint.TxId) != 32 {
		return fmt.Errorf("outpoint.txId 必须是 32 字节，实际: %d 字节", len(outpoint.TxId))
	}

	return nil
}

// ValidateOwnerAddress 验证Owner地址
func (v *DraftValidator) ValidateOwnerAddress(owner []byte) error {
	if len(owner) == 0 {
		return fmt.Errorf("owner 地址不能为空")
	}

	if len(owner) != 20 {
		return fmt.Errorf("owner 地址必须是 20 字节，实际: %d 字节", len(owner))
	}

	return nil
}

// ValidateAmount 验证金额字符串
func (v *DraftValidator) ValidateAmount(amount string) error {
	if amount == "" {
		return fmt.Errorf("amount 不能为空")
	}

	amountBig, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return fmt.Errorf("amount 不是有效的数字: %s", amount)
	}

	if amountBig.Sign() <= 0 {
		return fmt.Errorf("amount 必须大于 0，实际: %s", amount)
	}

	return nil
}

// ValidateContentHash 验证内容哈希
func (v *DraftValidator) ValidateContentHash(contentHash []byte) error {
	if len(contentHash) == 0 {
		return fmt.Errorf("contentHash 不能为空")
	}

	if len(contentHash) != 32 {
		return fmt.Errorf("contentHash 必须是 32 字节，实际: %d 字节", len(contentHash))
	}

	return nil
}

// ValidateStateID 验证状态ID
func (v *DraftValidator) ValidateStateID(stateID []byte) error {
	if len(stateID) == 0 {
		return fmt.Errorf("stateId 不能为空")
	}

	if len(stateID) > 256 {
		return fmt.Errorf("stateId 长度不能超过 256 字节，实际: %d 字节", len(stateID))
	}

	return nil
}

// ValidateExecutionResultHash 验证执行结果哈希
func (v *DraftValidator) ValidateExecutionResultHash(hash []byte) error {
	if len(hash) == 0 {
		return fmt.Errorf("executionResultHash 不能为空")
	}

	if len(hash) != 32 {
		return fmt.Errorf("executionResultHash 必须是 32 字节，实际: %d 字节", len(hash))
	}

	return nil
}

