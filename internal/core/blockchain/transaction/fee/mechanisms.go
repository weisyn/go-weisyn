package fee

import (
	"context"
	"fmt"
	"math/big"

	pbtx "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ===============================
// 费用机制验证器
// ===============================

// MechanismValidator 费用机制验证器
type MechanismValidator struct {
	calculator *Calculator
}

// NewMechanismValidator 创建新的费用机制验证器
func NewMechanismValidator(calculator *Calculator) *MechanismValidator {
	if calculator == nil {
		panic("计算器不能为nil")
	}
	return &MechanismValidator{
		calculator: calculator,
	}
}

// ValidateFee 验证交易费用是否符合指定机制
func (mv *MechanismValidator) ValidateFee(ctx context.Context, tx *pbtx.Transaction, fetchUTXO UTXOFetcher) error {
	if tx == nil {
		return NewValidationError("transaction", nil, "交易不能为nil")
	}

	// 计算实际的UTXO差额费用
	actualFees, err := mv.calculator.CalculateUTXODifference(ctx, tx, fetchUTXO)
	if err != nil {
		return WrapError(err, "计算UTXO差额失败")
	}

	// 如果没有设置费用机制，使用默认的UTXO差额机制（任何差额都有效）
	if tx.FeeMechanism == nil {
		return nil
	}

	// 根据具体的费用机制进行验证
	switch mechanism := tx.FeeMechanism.(type) {
	case *pbtx.Transaction_MinimumFee:
		return mv.validateMinimumFee(actualFees, mechanism.MinimumFee)

	case *pbtx.Transaction_ProportionalFee:
		return mv.validateProportionalFee(ctx, actualFees, mechanism.ProportionalFee, tx, fetchUTXO)

	case *pbtx.Transaction_ContractFee:
		return mv.validateContractExecutionFee(actualFees, mechanism.ContractFee)

	case *pbtx.Transaction_PriorityFee:
		return mv.validatePriorityFee(actualFees, mechanism.PriorityFee)

	default:
		return fmt.Errorf("未知的费用机制类型: %T", mechanism)
	}
}

// ===============================
// 最低费用机制验证
// ===============================

// validateMinimumFee 验证最低费用机制
func (mv *MechanismValidator) validateMinimumFee(actualFees *TransactionFee, mechanism *pbtx.MinimumFee) error {
	if mechanism == nil {
		return NewValidationError("minimum_fee", nil, "最低费用机制不能为nil")
	}

	// 解析最低费用金额
	minimumAmount := ParseAmountString(mechanism.MinimumAmount)
	if err := ValidateAmount(minimumAmount); err != nil {
		return WrapError(err, "最低费用金额无效")
	}

	// 获取费用Token类型
	feeTokenKey := mv.getTokenKeyFromReference(mechanism.FeeToken)

	// 检查实际费用是否满足最低费用要求
	actualAmount := actualFees.Fees[feeTokenKey]
	if actualAmount == nil {
		actualAmount = big.NewInt(0)
	}

	if actualAmount.Cmp(minimumAmount) < 0 {
		return fmt.Errorf("费用不足: 最低要求 %s %s, 实际 %s %s",
			FormatAmount(minimumAmount), feeTokenKey,
			FormatAmount(actualAmount), feeTokenKey)
	}

	return nil
}

// ===============================
// 比例费用机制验证
// ===============================

// validateProportionalFee 验证比例费用机制（最复杂）
func (mv *MechanismValidator) validateProportionalFee(ctx context.Context, actualFees *TransactionFee, mechanism *pbtx.ProportionalFee, tx *pbtx.Transaction, fetchUTXO UTXOFetcher) error {
	if mechanism == nil {
		return NewValidationError("proportional_fee", nil, "比例费用机制不能为nil")
	}

	// 获取费用Token类型
	feeTokenKey := mv.getTokenKeyFromReference(mechanism.FeeToken)

	// 分析交易，识别找零和真实转账
	changeAnalysis, err := mv.analyzeChangeOutputs(ctx, tx, fetchUTXO)
	if err != nil {
		return WrapError(err, "找零分析失败")
	}

	// 计算真实转账金额（排除找零）
	transferAmount := changeAnalysis.TransferAmounts[feeTokenKey]
	if transferAmount == nil || transferAmount.Sign() <= 0 {
		// 如果没有该Token的转账，检查是否有任何转账
		totalTransfer := big.NewInt(0)
		for _, amount := range changeAnalysis.TransferAmounts {
			totalTransfer = SafeAdd(totalTransfer, amount)
		}

		if totalTransfer.Sign() <= 0 {
			return fmt.Errorf("没有有效的转账金额用于比例费用计算")
		}

		// 使用总转账金额的等价值（简化处理）
		transferAmount = totalTransfer
	}

	// 计算所需费用：转账金额 × 费率 / 10000
	rateBig := big.NewInt(int64(mechanism.RateBasisPoints))
	requiredFee := SafeMul(transferAmount, rateBig)
	requiredFee = SafeDiv(requiredFee, GetBasisPointsDivisor())

	// 检查最大费用限制
	if mechanism.MaxFeeAmount != nil && *mechanism.MaxFeeAmount != "" {
		maxFee := ParseAmountString(*mechanism.MaxFeeAmount)
		if requiredFee.Cmp(maxFee) > 0 {
			requiredFee = maxFee
		}
	}

	// 验证实际费用是否充足
	actualAmount := actualFees.Fees[feeTokenKey]
	if actualAmount == nil {
		actualAmount = big.NewInt(0)
	}

	if actualAmount.Cmp(requiredFee) < 0 {
		return fmt.Errorf("比例费用不足: 转账金额 %s, 费率 %d基点, 需要 %s %s, 实际 %s %s",
			FormatAmount(transferAmount),
			mechanism.RateBasisPoints,
			FormatAmount(requiredFee), feeTokenKey,
			FormatAmount(actualAmount), feeTokenKey)
	}

	return nil
}

// ChangeAnalysis 找零分析结果
type ChangeAnalysis struct {
	ChangeAmounts   map[TokenKey]*big.Int // 找零金额
	TransferAmounts map[TokenKey]*big.Int // 转账金额
	InputOwners     map[string]bool       // 输入所有者地址集合
	IsComplexTx     bool                  // 是否为复杂交易
}

// analyzeChangeOutputs 分析找零输出
func (mv *MechanismValidator) analyzeChangeOutputs(ctx context.Context, tx *pbtx.Transaction, fetchUTXO UTXOFetcher) (*ChangeAnalysis, error) {
	analysis := &ChangeAnalysis{
		ChangeAmounts:   make(map[TokenKey]*big.Int),
		TransferAmounts: make(map[TokenKey]*big.Int),
		InputOwners:     make(map[string]bool),
	}

	// 1. 收集所有输入的所有者地址
	for _, input := range tx.Inputs {
		prevOutput, err := fetchUTXO(ctx, input.PreviousOutput)
		if err != nil {
			continue // 跳过获取失败的输入
		}

		ownerKey := string(prevOutput.Owner)
		analysis.InputOwners[ownerKey] = true
	}

	// 检查是否为复杂交易
	analysis.IsComplexTx = len(analysis.InputOwners) > 1

	// 2. 分析输出：区分找零和真实转账
	for _, output := range tx.Outputs {
		tokenKey, amount, err := ExtractTokenInfo(output)
		if err != nil {
			continue // 跳过非资产输出
		}

		ownerKey := string(output.Owner)
		if analysis.InputOwners[ownerKey] {
			// 输出地址在输入地址集合中 → 找零
			if existing, exists := analysis.ChangeAmounts[tokenKey]; exists {
				analysis.ChangeAmounts[tokenKey] = SafeAdd(existing, amount)
			} else {
				analysis.ChangeAmounts[tokenKey] = new(big.Int).Set(amount)
			}
		} else {
			// 输出地址不在输入地址集合中 → 真实转账
			if existing, exists := analysis.TransferAmounts[tokenKey]; exists {
				analysis.TransferAmounts[tokenKey] = SafeAdd(existing, amount)
			} else {
				analysis.TransferAmounts[tokenKey] = new(big.Int).Set(amount)
			}
		}
	}

	return analysis, nil
}

// ===============================
// 合约执行费用机制验证
// ===============================

// validateContractExecutionFee 验证合约执行费用机制
func (mv *MechanismValidator) validateContractExecutionFee(actualFees *TransactionFee, mechanism *pbtx.ContractExecutionFee) error {
	if mechanism == nil {
		return NewValidationError("contract_execution_fee", nil, "合约执行费用机制不能为nil")
	}

	// 获取费用Token类型
	feeTokenKey := mv.getTokenKeyFromReference(mechanism.FeeToken)

	// 计算所需费用：base_fee + (执行费用_limit × 执行费用_price)
	baseFee := ParseAmountString(mechanism.BaseFee)
	if err := ValidateAmount(baseFee); err != nil {
		return WrapError(err, "基础费用无效")
	}

	feePrice := ParseAmountString(mechanism.ExecutionFee)
	if err := ValidateAmount(feePrice); err != nil {
		return WrapError(err, "执行费用价格无效")
	}

	// 对于合约执行，执行费用是固定的
	executionFeeAmount := ParseAmountString(mechanism.ExecutionFee)
	if err := ValidateAmount(executionFeeAmount); err != nil {
		return WrapError(err, "执行费用无效")
	}

	执行费用Cost := executionFeeAmount
	totalRequired := SafeAdd(baseFee, 执行费用Cost)

	// 验证实际费用
	actualAmount := actualFees.Fees[feeTokenKey]
	if actualAmount == nil {
		actualAmount = big.NewInt(0)
	}

	if actualAmount.Cmp(totalRequired) < 0 {
		return fmt.Errorf("合约执行费用不足: 需要 %s %s (基础费用 %s + 执行费用费用 %s), 实际 %s %s",
			FormatAmount(totalRequired), feeTokenKey,
			FormatAmount(baseFee), FormatAmount(执行费用Cost),
			FormatAmount(actualAmount), feeTokenKey)
	}

	return nil
}

// ===============================
// 优先级费用机制验证
// ===============================

// validatePriorityFee 验证优先级费用机制
func (mv *MechanismValidator) validatePriorityFee(actualFees *TransactionFee, mechanism *pbtx.PriorityFee) error {
	if mechanism == nil {
		return NewValidationError("priority_fee", nil, "优先级费用机制不能为nil")
	}

	// 获取费用Token类型
	feeTokenKey := mv.getTokenKeyFromReference(mechanism.FeeToken)

	// 解析基础费用和优先级倍率
	baseFee := ParseAmountString(mechanism.BaseFee)
	if err := ValidateAmount(baseFee); err != nil {
		return WrapError(err, "基础费用无效")
	}

	priorityRate := ParseAmountString(mechanism.PriorityRate)
	if err := ValidateAmount(priorityRate); err != nil {
		return WrapError(err, "优先级倍率无效")
	}

	if priorityRate.Cmp(big.NewInt(1)) < 0 {
		return NewValidationError("priority_rate", mechanism.PriorityRate, "优先级倍率不能小于1")
	}

	// 计算所需费用：base_fee × priority_rate
	requiredFee := SafeMul(baseFee, priorityRate)

	// 验证实际费用
	actualAmount := actualFees.Fees[feeTokenKey]
	if actualAmount == nil {
		actualAmount = big.NewInt(0)
	}

	if actualAmount.Cmp(requiredFee) < 0 {
		return fmt.Errorf("优先级费用不足: 需要 %s %s (基础费用 %s × 倍率 %s), 实际 %s %s",
			FormatAmount(requiredFee), feeTokenKey,
			FormatAmount(baseFee), FormatAmount(priorityRate),
			FormatAmount(actualAmount), feeTokenKey)
	}

	return nil
}

// ===============================
// 工具方法
// ===============================

// getTokenKeyFromReference 从TokenReference获取TokenKey
func (mv *MechanismValidator) getTokenKeyFromReference(tokenRef *pbtx.TokenReference) TokenKey {
	if tokenRef == nil {
		return GenerateNativeTokenKey()
	}

	switch tokenType := tokenRef.TokenType.(type) {
	case *pbtx.TokenReference_NativeToken:
		return GenerateNativeTokenKey()

	case *pbtx.TokenReference_ContractAddress:
		return GenerateContractTokenKey(tokenType.ContractAddress, "ft", []byte{})

	default:
		return GenerateNativeTokenKey()
	}
}

// ===============================
// 费用估算器
// ===============================

// FeeEstimator 费用估算器
type FeeEstimator struct {
	calculator *Calculator
	validator  *MechanismValidator
}

// NewFeeEstimator 创建新的费用估算器
func NewFeeEstimator(calculator *Calculator) *FeeEstimator {
	if calculator == nil {
		panic("计算器不能为nil")
	}
	return &FeeEstimator{
		calculator: calculator,
		validator:  NewMechanismValidator(calculator),
	}
}

// FeeEstimate 费用估算结果
type FeeEstimate struct {
	Conservative *big.Int // 保守估算
	Standard     *big.Int // 标准估算
	Fast         *big.Int // 快速估算
	TokenKey     TokenKey // 费用Token类型
	Mechanism    string   // 使用的费用机制
	Details      string   // 估算详情
}

// EstimateFee 估算交易费用
func (fe *FeeEstimator) EstimateFee(ctx context.Context, tx *pbtx.Transaction, fetchUTXO UTXOFetcher) (*FeeEstimate, error) {
	if tx == nil {
		return nil, NewValidationError("transaction", nil, "交易不能为nil")
	}

	// 计算当前UTXO差额
	actualFees, err := fe.calculator.CalculateUTXODifference(ctx, tx, fetchUTXO)
	if err != nil {
		return nil, WrapError(err, "计算UTXO差额失败")
	}

	// 如果没有设置费用机制，使用默认估算
	if tx.FeeMechanism == nil {
		return fe.estimateDefaultFee(actualFees), nil
	}

	// 根据具体机制进行估算
	switch mechanism := tx.FeeMechanism.(type) {
	case *pbtx.Transaction_MinimumFee:
		return fe.estimateMinimumFee(actualFees, mechanism.MinimumFee), nil

	case *pbtx.Transaction_ProportionalFee:
		return fe.estimateProportionalFee(ctx, actualFees, mechanism.ProportionalFee, tx, fetchUTXO)

	case *pbtx.Transaction_ContractFee:
		return fe.estimateContractFee(actualFees, mechanism.ContractFee), nil

	case *pbtx.Transaction_PriorityFee:
		return fe.estimatePriorityFee(actualFees, mechanism.PriorityFee), nil

	default:
		return fe.estimateDefaultFee(actualFees), nil
	}
}

// estimateDefaultFee 估算默认费用（UTXO差额）
func (fe *FeeEstimator) estimateDefaultFee(actualFees *TransactionFee) *FeeEstimate {
	// 找到最大的费用作为主要费用
	var maxAmount *big.Int
	var maxTokenKey TokenKey

	for tokenKey, amount := range actualFees.Fees {
		if maxAmount == nil || amount.Cmp(maxAmount) > 0 {
			maxAmount = amount
			maxTokenKey = tokenKey
		}
	}

	if maxAmount == nil {
		maxAmount = big.NewInt(0)
		maxTokenKey = GenerateNativeTokenKey()
	}

	return &FeeEstimate{
		Conservative: new(big.Int).Set(maxAmount),
		Standard:     new(big.Int).Set(maxAmount),
		Fast:         new(big.Int).Set(maxAmount),
		TokenKey:     maxTokenKey,
		Mechanism:    "默认UTXO差额",
		Details:      fmt.Sprintf("当前差额: %s %s", FormatAmount(maxAmount), maxTokenKey),
	}
}

// estimateMinimumFee 估算最低费用
func (fe *FeeEstimator) estimateMinimumFee(actualFees *TransactionFee, mechanism *pbtx.MinimumFee) *FeeEstimate {
	minimumAmount := ParseAmountString(mechanism.MinimumAmount)
	tokenKey := fe.validator.getTokenKeyFromReference(mechanism.FeeToken)

	// 实际费用
	actualAmount := actualFees.Fees[tokenKey]
	if actualAmount == nil {
		actualAmount = big.NewInt(0)
	}

	// 建议费用
	conservative := Max(minimumAmount, actualAmount)
	standard := SafeAdd(conservative, SafeDiv(conservative, big.NewInt(10))) // +10%
	fast := SafeAdd(conservative, SafeDiv(conservative, big.NewInt(5)))      // +20%

	return &FeeEstimate{
		Conservative: conservative,
		Standard:     standard,
		Fast:         fast,
		TokenKey:     tokenKey,
		Mechanism:    "最低费用",
		Details:      fmt.Sprintf("最低要求: %s %s, 当前: %s %s", FormatAmount(minimumAmount), tokenKey, FormatAmount(actualAmount), tokenKey),
	}
}

// estimateProportionalFee 估算比例费用
func (fe *FeeEstimator) estimateProportionalFee(ctx context.Context, actualFees *TransactionFee, mechanism *pbtx.ProportionalFee, tx *pbtx.Transaction, fetchUTXO UTXOFetcher) (*FeeEstimate, error) {
	tokenKey := fe.validator.getTokenKeyFromReference(mechanism.FeeToken)

	// 分析找零
	changeAnalysis, err := fe.validator.analyzeChangeOutputs(ctx, tx, fetchUTXO)
	if err != nil {
		return nil, WrapError(err, "找零分析失败")
	}

	// 计算转账金额
	transferAmount := changeAnalysis.TransferAmounts[tokenKey]
	if transferAmount == nil {
		transferAmount = big.NewInt(0)
	}

	// 计算比例费用
	rateBig := big.NewInt(int64(mechanism.RateBasisPoints))
	baseFee := SafeDiv(SafeMul(transferAmount, rateBig), GetBasisPointsDivisor())

	// 检查最大费用限制
	if mechanism.MaxFeeAmount != nil && *mechanism.MaxFeeAmount != "" {
		maxFee := ParseAmountString(*mechanism.MaxFeeAmount)
		if baseFee.Cmp(maxFee) > 0 {
			baseFee = maxFee
		}
	}

	conservative := baseFee
	standard := SafeAdd(baseFee, SafeDiv(baseFee, big.NewInt(20))) // +5%
	fast := SafeAdd(baseFee, SafeDiv(baseFee, big.NewInt(10)))     // +10%

	return &FeeEstimate{
		Conservative: conservative,
		Standard:     standard,
		Fast:         fast,
		TokenKey:     tokenKey,
		Mechanism:    "比例费用",
		Details:      fmt.Sprintf("转账金额: %s, 费率: %d基点", FormatAmount(transferAmount), mechanism.RateBasisPoints),
	}, nil
}

// estimateContractFee 估算合约执行费用
func (fe *FeeEstimator) estimateContractFee(actualFees *TransactionFee, mechanism *pbtx.ContractExecutionFee) *FeeEstimate {
	tokenKey := fe.validator.getTokenKeyFromReference(mechanism.FeeToken)

	baseFee := ParseAmountString(mechanism.BaseFee)
	executionFee := ParseAmountString(mechanism.ExecutionFee)

	执行费用Cost := executionFee
	totalRequired := SafeAdd(baseFee, 执行费用Cost)

	conservative := totalRequired
	standard := SafeAdd(totalRequired, SafeDiv(totalRequired, big.NewInt(20))) // +5%
	fast := SafeAdd(totalRequired, SafeDiv(totalRequired, big.NewInt(10)))     // +10%

	return &FeeEstimate{
		Conservative: conservative,
		Standard:     standard,
		Fast:         fast,
		TokenKey:     tokenKey,
		Mechanism:    "合约执行费用",
		Details:      fmt.Sprintf("基础费用: %s, 执行费用费用: %s", FormatAmount(baseFee), FormatAmount(执行费用Cost)),
	}
}

// estimatePriorityFee 估算优先级费用
func (fe *FeeEstimator) estimatePriorityFee(actualFees *TransactionFee, mechanism *pbtx.PriorityFee) *FeeEstimate {
	tokenKey := fe.validator.getTokenKeyFromReference(mechanism.FeeToken)

	baseFee := ParseAmountString(mechanism.BaseFee)
	priorityRate := ParseAmountString(mechanism.PriorityRate)

	requiredFee := SafeMul(baseFee, priorityRate)

	conservative := requiredFee
	standard := SafeAdd(requiredFee, SafeDiv(requiredFee, big.NewInt(20))) // +5%
	fast := SafeAdd(requiredFee, SafeDiv(requiredFee, big.NewInt(10)))     // +10%

	return &FeeEstimate{
		Conservative: conservative,
		Standard:     standard,
		Fast:         fast,
		TokenKey:     tokenKey,
		Mechanism:    "优先级费用",
		Details:      fmt.Sprintf("基础费用: %s, 优先级倍率: %s", FormatAmount(baseFee), FormatAmount(priorityRate)),
	}
}

// FormatFeeEstimate 格式化费用估算结果
func (fe *FeeEstimator) FormatFeeEstimate(estimate *FeeEstimate) string {
	if estimate == nil {
		return "nil"
	}

	return fmt.Sprintf("机制: %s, Token: %s, 保守: %s, 标准: %s, 快速: %s, 详情: %s",
		estimate.Mechanism,
		estimate.TokenKey,
		FormatAmount(estimate.Conservative),
		FormatAmount(estimate.Standard),
		FormatAmount(estimate.Fast),
		estimate.Details,
	)
}
