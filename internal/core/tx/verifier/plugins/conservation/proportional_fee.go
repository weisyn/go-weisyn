// Package conservation 提供价值守恒验证插件实现
//
// proportional_fee.go: 按比例收费验证插件
package conservation

import (
	"context"
	"fmt"
	"math/big"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxopb "github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/interfaces/tx"
)

// ProportionalFeePlugin 按比例收费验证插件
//
// 🎯 **核心职责**：验证交易费用是否符合按比例收费要求
//
// 💡 **设计理念**：
// 大额转账场景需要按比例收费，防止小额费用进行大额转账。
// 公式：费用 = 转账金额 × 费率（万分之X）
//
// 🔒 **验证规则**：
// 1. 如果交易设置了 proportional_fee：实际费用 >= 转账金额 × (rate_basis_points / 10000)
// 2. 如果设置了 max_fee_amount：实际费用 <= max_fee_amount
// 3. 如果未设置 proportional_fee：直接通过
//
// ⚠️ **核心约束**：
// - ❌ 插件无状态：不存储验证结果
// - ❌ 插件只读：不修改交易
// - ✅ 并发安全：多个 goroutine 可以同时调用
//
// 📞 **调用方**：Verifier Kernel（通过 Conservation Hook）
type ProportionalFeePlugin struct{}

// NewProportionalFeePlugin 创建新的 ProportionalFeePlugin
//
// 返回：
//   - *ProportionalFeePlugin: 新创建的实例
func NewProportionalFeePlugin() *ProportionalFeePlugin {
	return &ProportionalFeePlugin{}
}

// Name 返回插件名称
//
// 实现 tx.ConservationPlugin 接口
//
// 返回：
//   - string: "proportional_fee"
func (p *ProportionalFeePlugin) Name() string {
	return "proportional_fee"
}

// Check 检查费用是否符合按比例收费要求
//
// 实现 tx.ConservationPlugin 接口
//
// 🎯 **核心逻辑**：
// 1. 检查交易是否设置了 proportional_fee
// 2. 如果未设置，直接通过
// 3. 如果设置了，计算转账金额和实际费用
// 4. 验证：实际费用 >= 转账金额 × (rate / 10000)
// 5. 如果设置了 max_fee_amount，验证：实际费用 <= max_fee_amount
//
// 参数：
//   - ctx: 上下文对象
//   - inputs: 输入 UTXO 列表（已通过 UTXOManager 获取）
//   - outputs: 输出列表（从 Transaction 中获取）
//   - tx: 完整的交易对象
//
// 返回：
//   - error: 费用验证失败的原因
//   - nil: 验证通过
//
// 📝 **使用场景**：
//
//	// 场景：大额转账按比例收费 0.1%（10个基点）
//	proportional_fee {
//	    rate_basis_points: 10    // 0.1% = 10/10000
//	    max_fee_amount: "1000000000"  // 最高 1000 原生币
//	    fee_token: {native_token: true}
//	}
//
//	// 转账 100000 原生币，最低费用 = 100000 × 0.001 = 100
//	// 实际费用需要 >= 100 且 <= 1000000000
func (p *ProportionalFeePlugin) Check(
	ctx context.Context,
	inputs []*utxopb.UTXO,
	outputs []*transaction.TxOutput,
	tx *transaction.Transaction,
) error {
	// 1. 检查是否设置了 proportional_fee
	proportionalFee := tx.GetProportionalFee()
	if proportionalFee == nil {
		// 未设置按比例收费，直接通过
		return nil
	}

	// 2. 解析费率（万分之X）
	rateBasisPoints := proportionalFee.RateBasisPoints
	if rateBasisPoints == 0 {
		return fmt.Errorf("invalid proportional_fee.rate_basis_points: must be > 0")
	}

	// 3. 计算转账金额和实际费用
	transferAmount, actualFee, err := p.calculateAmounts(inputs, outputs, tx)
	if err != nil {
		return fmt.Errorf("failed to calculate amounts: %w", err)
	}

	// 4. 计算最低费用：转账金额 × (rate / 10000)
	// minFee = transferAmount × rateBasisPoints / 10000
	minFee := new(big.Int).Mul(transferAmount, big.NewInt(int64(rateBasisPoints)))
	minFee.Div(minFee, big.NewInt(10000))

	// 5. 验证：实际费用 >= 最低费用
	if actualFee.Cmp(minFee) < 0 {
		return fmt.Errorf(
			"insufficient proportional fee: actual=%s, required=%s (transfer=%s, rate=%d/10000), shortfall=%s",
			actualFee.String(),
			minFee.String(),
			transferAmount.String(),
			rateBasisPoints,
			new(big.Int).Sub(minFee, actualFee).String(),
		)
	}

	// 6. 如果设置了 max_fee_amount，验证：实际费用 <= 最大费用
	if proportionalFee.MaxFeeAmount != nil && *proportionalFee.MaxFeeAmount != "" {
		maxFee, ok := new(big.Int).SetString(*proportionalFee.MaxFeeAmount, 10)
		if !ok || maxFee.Sign() < 0 {
			return fmt.Errorf("invalid proportional_fee.max_fee_amount: %s", *proportionalFee.MaxFeeAmount)
		}

		if actualFee.Cmp(maxFee) > 0 {
			return fmt.Errorf(
				"excessive proportional fee: actual=%s, max=%s, overage=%s",
				actualFee.String(),
				maxFee.String(),
				new(big.Int).Sub(actualFee, maxFee).String(),
			)
		}
	}

	// 7. 验证通过
	return nil
}

// calculateAmounts 计算转账金额和实际费用
//
// 🎯 **核心逻辑**：
// - 转账金额 = Σ(输出资产金额)（不包括找零）
// - 实际费用 = Σ(输入资产金额) - Σ(输出资产金额)
//
// ✅ **P6 完整实现**：支持原生代币和合约代币
//
// 参数：
//   - inputs: 输入 UTXO 列表
//   - outputs: 输出列表
//   - tx: 完整的交易对象
//
// 返回：
//   - *big.Int: 转账金额
//   - *big.Int: 实际费用
//   - error: 计算失败的原因
func (p *ProportionalFeePlugin) calculateAmounts(
	inputs []*utxopb.UTXO,
	outputs []*transaction.TxOutput,
	tx *transaction.Transaction,
) (*big.Int, *big.Int, error) {
	// 获取费用代币类型（从 proportional_fee 中获取）
	proportionalFee := tx.GetProportionalFee()
	if proportionalFee == nil || proportionalFee.FeeToken == nil {
		// 默认使用原生代币
		return p.calculateAmountsForNativeToken(inputs, outputs, tx)
	}

	// 根据 fee_token 类型计算
	switch tokenType := proportionalFee.FeeToken.TokenType.(type) {
	case *transaction.TokenReference_NativeToken:
		// 原生代币
		return p.calculateAmountsForNativeToken(inputs, outputs, tx)

	case *transaction.TokenReference_ContractAddress:
		// 合约代币
		return p.calculateAmountsForContractToken(inputs, outputs, tx, tokenType.ContractAddress)

	default:
		return nil, nil, fmt.Errorf("unknown fee_token type: %T", tokenType)
	}
}

// calculateAmountsForNativeToken 计算原生代币的转账金额和费用
func (p *ProportionalFeePlugin) calculateAmountsForNativeToken(
	inputs []*utxopb.UTXO,
	outputs []*transaction.TxOutput,
	tx *transaction.Transaction,
) (*big.Int, *big.Int, error) {
	totalInputs := big.NewInt(0)
	totalOutputs := big.NewInt(0)

	// 1. 计算输入总和（只计算原生代币）
	for i, utxo := range inputs {
		// 只累加消费型输入（is_reference_only = false）
		if tx.Inputs[i].IsReferenceOnly {
			continue
		}

		// 获取 AssetOutput 的原生币
		if assetOutput := utxo.GetCachedOutput().GetAsset(); assetOutput != nil {
			if nativeCoin, ok := assetOutput.AssetContent.(*transaction.AssetOutput_NativeCoin); ok {
				if nativeCoin.NativeCoin != nil {
					amount, ok := new(big.Int).SetString(nativeCoin.NativeCoin.Amount, 10)
					if !ok {
						return nil, nil, fmt.Errorf("invalid input native coin amount: %s", nativeCoin.NativeCoin.Amount)
					}
					totalInputs.Add(totalInputs, amount)
				}
			}
		}
	}

	// 2. 计算输出总和（只计算原生代币）
	for _, output := range outputs {
		// 获取 AssetOutput 的原生币
		if assetOutput := output.GetAsset(); assetOutput != nil {
			if nativeCoin, ok := assetOutput.AssetContent.(*transaction.AssetOutput_NativeCoin); ok {
				if nativeCoin.NativeCoin != nil {
					amount, ok := new(big.Int).SetString(nativeCoin.NativeCoin.Amount, 10)
					if !ok {
						return nil, nil, fmt.Errorf("invalid output native coin amount: %s", nativeCoin.NativeCoin.Amount)
					}
					totalOutputs.Add(totalOutputs, amount)
				}
			}
		}
	}

	// 3. 计算费用
	actualFee := new(big.Int).Sub(totalInputs, totalOutputs)

	// 4. 验证费用不为负
	if actualFee.Sign() < 0 {
		return nil, nil, fmt.Errorf("negative fee: inputs=%s, outputs=%s", totalInputs.String(), totalOutputs.String())
	}

	// 5. 转账金额 = 输出总和（简化：不区分找零）
	transferAmount := totalOutputs

	return transferAmount, actualFee, nil
}

// calculateAmountsForContractToken 计算合约代币的转账金额和费用
//
// 只计算指定合约地址的代币
//
// 参数：
//   - inputs: 输入 UTXO 列表
//   - outputs: 输出列表
//   - tx: 完整的交易对象
//   - contractAddress: 合约地址
//
// 返回：
//   - *big.Int: 转账金额
//   - *big.Int: 实际费用
//   - error: 计算失败的原因
func (p *ProportionalFeePlugin) calculateAmountsForContractToken(
	inputs []*utxopb.UTXO,
	outputs []*transaction.TxOutput,
	tx *transaction.Transaction,
	contractAddress []byte,
) (*big.Int, *big.Int, error) {
	totalInputs := big.NewInt(0)
	totalOutputs := big.NewInt(0)

	// 1. 计算输入总和（只计算指定合约代币）
	for i, utxo := range inputs {
		// 只累加消费型输入
		if tx.Inputs[i].IsReferenceOnly {
			continue
		}

		// 获取 AssetOutput 的合约代币
		if assetOutput := utxo.GetCachedOutput().GetAsset(); assetOutput != nil {
			if contractToken, ok := assetOutput.AssetContent.(*transaction.AssetOutput_ContractToken); ok {
				if contractToken.ContractToken != nil {
					// 检查合约地址是否匹配
					if bytesEqual(contractToken.ContractToken.ContractAddress, contractAddress) {
						amount, ok := new(big.Int).SetString(contractToken.ContractToken.Amount, 10)
						if !ok {
							return nil, nil, fmt.Errorf("invalid input contract token amount: %s", contractToken.ContractToken.Amount)
						}
						totalInputs.Add(totalInputs, amount)
					}
				}
			}
		}
	}

	// 2. 计算输出总和（只计算指定合约代币）
	for _, output := range outputs {
		// 获取 AssetOutput 的合约代币
		if assetOutput := output.GetAsset(); assetOutput != nil {
			if contractToken, ok := assetOutput.AssetContent.(*transaction.AssetOutput_ContractToken); ok {
				if contractToken.ContractToken != nil {
					// 检查合约地址是否匹配
					if bytesEqual(contractToken.ContractToken.ContractAddress, contractAddress) {
						amount, ok := new(big.Int).SetString(contractToken.ContractToken.Amount, 10)
						if !ok {
							return nil, nil, fmt.Errorf("invalid output contract token amount: %s", contractToken.ContractToken.Amount)
						}
						totalOutputs.Add(totalOutputs, amount)
					}
				}
			}
		}
	}

	// 3. 计算费用
	actualFee := new(big.Int).Sub(totalInputs, totalOutputs)

	// 4. 验证费用不为负
	if actualFee.Sign() < 0 {
		return nil, nil, fmt.Errorf("negative contract token fee: inputs=%s, outputs=%s, contract=%x",
			totalInputs.String(), totalOutputs.String(), contractAddress)
	}

	// 5. 转账金额 = 输出总和（简化：不区分找零）
	transferAmount := totalOutputs

	return transferAmount, actualFee, nil
}

// 编译期检查：确保 ProportionalFeePlugin 实现了 tx.ConservationPlugin 接口
var _ tx.ConservationPlugin = (*ProportionalFeePlugin)(nil)
