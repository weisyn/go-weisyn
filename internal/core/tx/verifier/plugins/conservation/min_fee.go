// Package conservation 提供价值守恒验证插件实现
//
// min_fee.go: 最低费用验证插件
package conservation

import (
	"context"
	"fmt"
	"math/big"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxopb "github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/interfaces/tx"
)

// MinFeePlugin 最低费用验证插件
//
// 🎯 **核心职责**：验证交易费用是否满足最低要求
//
// 💡 **设计理念**：
// 防止垃圾交易攻击，确保每笔交易支付足够的费用来激励矿工/验证者打包。
//
// 🔒 **验证规则**：
// 1. 如果交易设置了 minimum_fee：实际费用 >= minimum_amount
// 2. 如果未设置 minimum_fee：直接通过（使用默认差额机制）
// 3. 实际费用 = Σ(输入) - Σ(输出)
//
// ⚠️ **核心约束**：
// - ❌ 插件无状态：不存储验证结果
// - ❌ 插件只读：不修改交易
// - ✅ 并发安全：多个 goroutine 可以同时调用
//
// 📞 **调用方**：Verifier Kernel（通过 Conservation Hook）
type MinFeePlugin struct{}

// NewMinFeePlugin 创建新的 MinFeePlugin
//
// 返回：
//   - *MinFeePlugin: 新创建的实例
func NewMinFeePlugin() *MinFeePlugin {
	return &MinFeePlugin{}
}

// Name 返回插件名称
//
// 实现 tx.ConservationPlugin 接口
//
// 返回：
//   - string: "min_fee"
func (p *MinFeePlugin) Name() string {
	return "min_fee"
}

// Check 检查费用是否满足最低要求
//
// 实现 tx.ConservationPlugin 接口
//
// 🎯 **核心逻辑**：
// 1. 检查交易是否设置了 minimum_fee
// 2. 如果未设置，直接通过（使用默认差额机制）
// 3. 如果设置了，计算实际费用并验证
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
//	// 场景：防垃圾交易，设置最低费用 0.001 原生币
//	minimum_fee {
//	    minimum_amount: "1000000"  // 1000000 wei = 0.001 原生币
//	    fee_token: {native_token: true}
//	}
func (p *MinFeePlugin) Check(
	ctx context.Context,
	inputs []*utxopb.UTXO,
	outputs []*transaction.TxOutput,
	tx *transaction.Transaction,
) error {
	// 1. 检查是否设置了 minimum_fee
	minimumFee := tx.GetMinimumFee()
	if minimumFee == nil {
		// 未设置最低费用，直接通过（使用默认差额机制）
		return nil
	}

	// 2. 解析最低费用金额
	minFeeAmount, ok := new(big.Int).SetString(minimumFee.MinimumAmount, 10)
	if !ok || minFeeAmount.Sign() < 0 {
		return fmt.Errorf("invalid minimum_fee.minimum_amount: %s", minimumFee.MinimumAmount)
	}

	// 3. 计算实际费用（输入总和 - 输出总和）
	actualFee, err := p.calculateActualFee(inputs, outputs, tx)
	if err != nil {
		return fmt.Errorf("failed to calculate actual fee: %w", err)
	}

	// 4. 验证：实际费用 >= 最低费用
	if actualFee.Cmp(minFeeAmount) < 0 {
		return fmt.Errorf(
			"insufficient fee: actual=%s, minimum=%s, shortfall=%s",
			actualFee.String(),
			minFeeAmount.String(),
			new(big.Int).Sub(minFeeAmount, actualFee).String(),
		)
	}

	// 5. 验证通过
	return nil
}

// calculateActualFee 计算交易的实际费用
//
// 🎯 **核心逻辑**：
// 实际费用 = Σ(输入资产金额) - Σ(输出资产金额)
//
// ✅ **P6 完整实现**：支持原生代币和合约代币
//
// 参数：
//   - inputs: 输入 UTXO 列表
//   - outputs: 输出列表
//   - tx: 完整的交易对象
//
// 返回：
//   - *big.Int: 实际费用
//   - error: 计算失败的原因
func (p *MinFeePlugin) calculateActualFee(
	inputs []*utxopb.UTXO,
	outputs []*transaction.TxOutput,
	tx *transaction.Transaction,
) (*big.Int, error) {
	// 获取费用代币类型（从 minimum_fee 中获取）
	minimumFee := tx.GetMinimumFee()
	if minimumFee == nil || minimumFee.FeeToken == nil {
		// 默认使用原生代币
		return p.calculateFeeForNativeToken(inputs, outputs, tx)
	}

	// 根据 fee_token 类型计算费用
	switch tokenType := minimumFee.FeeToken.TokenType.(type) {
	case *transaction.TokenReference_NativeToken:
		// 原生代币费用
		return p.calculateFeeForNativeToken(inputs, outputs, tx)

	case *transaction.TokenReference_ContractAddress:
		// 合约代币费用
		return p.calculateFeeForContractToken(inputs, outputs, tx, tokenType.ContractAddress)

	default:
		return nil, fmt.Errorf("unknown fee_token type: %T", tokenType)
	}
}

// calculateFeeForNativeToken 计算原生代币费用
func (p *MinFeePlugin) calculateFeeForNativeToken(
	inputs []*utxopb.UTXO,
	outputs []*transaction.TxOutput,
	tx *transaction.Transaction,
) (*big.Int, error) {
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
						return nil, fmt.Errorf("invalid input native coin amount: %s", nativeCoin.NativeCoin.Amount)
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
						return nil, fmt.Errorf("invalid output native coin amount: %s", nativeCoin.NativeCoin.Amount)
					}
					totalOutputs.Add(totalOutputs, amount)
				}
			}
		}
	}

	// 3. 实际费用 = 输入总和 - 输出总和
	actualFee := new(big.Int).Sub(totalInputs, totalOutputs)

	// 4. 验证费用不为负（价值守恒的基本要求）
	if actualFee.Sign() < 0 {
		return nil, fmt.Errorf("negative fee: inputs=%s, outputs=%s", totalInputs.String(), totalOutputs.String())
	}

	return actualFee, nil
}

// calculateFeeForContractToken 计算合约代币费用
//
// 只计算指定合约地址的代币差额作为费用
//
// 参数：
//   - inputs: 输入 UTXO 列表
//   - outputs: 输出列表
//   - tx: 完整的交易对象
//   - contractAddress: 合约地址
//
// 返回：
//   - *big.Int: 实际费用
//   - error: 计算失败的原因
func (p *MinFeePlugin) calculateFeeForContractToken(
	inputs []*utxopb.UTXO,
	outputs []*transaction.TxOutput,
	tx *transaction.Transaction,
	contractAddress []byte,
) (*big.Int, error) {
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
							return nil, fmt.Errorf("invalid input contract token amount: %s", contractToken.ContractToken.Amount)
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
							return nil, fmt.Errorf("invalid output contract token amount: %s", contractToken.ContractToken.Amount)
						}
						totalOutputs.Add(totalOutputs, amount)
					}
				}
			}
		}
	}

	// 3. 实际费用 = 输入总和 - 输出总和
	actualFee := new(big.Int).Sub(totalInputs, totalOutputs)

	// 4. 验证费用不为负
	if actualFee.Sign() < 0 {
		return nil, fmt.Errorf("negative contract token fee: inputs=%s, outputs=%s, contract=%x",
			totalInputs.String(), totalOutputs.String(), contractAddress)
	}

	return actualFee, nil
}

// bytesEqual 比较两个字节数组是否相等
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// 编译期检查：确保 MinFeePlugin 实现了 tx.ConservationPlugin 接口
var _ tx.ConservationPlugin = (*MinFeePlugin)(nil)
