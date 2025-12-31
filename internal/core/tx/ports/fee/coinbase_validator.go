package fee

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"sort"

	transaction_pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	txiface "github.com/weisyn/v1/pkg/interfaces/tx"
)

// CoinbaseValidator Coinbase交易验证器（零增发）
//
// 🎯 **零增发验证核心**
//
// 验证规则:
//   - len(Inputs) == 0（Coinbase无输入）
//   - 所有输出Owner == minerAddr（归矿工所有）
//   - Sum(Outputs by token) == expectedFees（费用守恒）
//   - 无额外Token（增发检测）
//
// 费用守恒验证:
//  1. 提取Coinbase所有输出的Token和金额
//  2. 按TokenKey分组累加
//  3. 与expectedFees逐个Token对比
//  4. Token数量必须一致（防止增发）
type CoinbaseValidator struct {
	calculator *Calculator // 复用Calculator的Token提取逻辑
}

// NewCoinbaseValidator 创建Coinbase验证器
func NewCoinbaseValidator() *CoinbaseValidator {
	// 注意：这里不需要utxoFetcher，因为Coinbase没有输入
	// 仅复用Calculator的extractTokenInfo方法
	return &CoinbaseValidator{
		calculator: &Calculator{},
	}
}

// Validate 验证Coinbase交易
//
// 参数:
//
//	ctx: 上下文对象
//	coinbase: Coinbase交易
//	expectedFees: 预期的手续费（从普通交易计算得出）
//	minerAddr: 矿工地址（20字节）
//
// 返回:
//
//	error: 验证错误
func (v *CoinbaseValidator) Validate(
	ctx context.Context,
	coinbase *transaction_pb.Transaction,
	expectedFees *txiface.AggregatedFees,
	minerAddr []byte,
) error {
	// 1. 验证无输入
	if len(coinbase.Inputs) != 0 {
		return fmt.Errorf("Coinbase交易不能有输入，实际输入数: %d", len(coinbase.Inputs))
	}

	// ✅ 特殊情况：零增发模式下，如果没有交易（无手续费），Coinbase可以没有输出
	// 这是合法的，直接返回验证通过
	if len(coinbase.Outputs) == 0 {
		if len(expectedFees.ByToken) == 0 {
			// 期望手续费也为空，验证通过
			return nil
		}
		// 期望有手续费但Coinbase没有输出，验证失败
		return fmt.Errorf("Coinbase缺少期望的手续费输出")
	}

	// 2. 验证所有输出Owner == minerAddr
	for i, output := range coinbase.Outputs {
		if !bytes.Equal(output.Owner, minerAddr) {
			return fmt.Errorf("Coinbase输出[%d]的Owner不是矿工地址: 期望=%x, 实际=%x",
				i, minerAddr, output.Owner)
		}
	}

	// 3. 提取Coinbase所有输出的Token和金额
	actualFees := make(map[txiface.TokenKey]*big.Int)
	for i, output := range coinbase.Outputs {
		assetOutput := output.GetAsset()
		if assetOutput == nil {
			return fmt.Errorf("Coinbase输出[%d]不是资产输出", i)
		}

		tokenKey, amount, err := v.calculator.extractTokenInfo(assetOutput)
		if err != nil {
			return fmt.Errorf("Coinbase输出[%d]: 提取Token信息失败: %w", i, err)
		}

		// ✅ 修复：允许金额为0（零增发机制下，无交易时手续费为0是合法的）
		// 但不允许负数（防止金额字段错误）
		if amount.Sign() < 0 {
			return fmt.Errorf("Coinbase输出[%d]: 金额不能为负数, 实际=%s", i, amount.String())
		}

		// ✅ 特殊处理：金额为0的输出（通常是矿工地址标识）不参与费用守恒验证
		// 仅金额>0的输出参与验证
		if amount.Sign() > 0 {
			// 累加同类Token（理论上不应该有重复，但做防御性检查）
			if existing, ok := actualFees[tokenKey]; ok {
				actualFees[tokenKey] = new(big.Int).Add(existing, amount)
			} else {
				actualFees[tokenKey] = new(big.Int).Set(amount)
			}
		}
	}

	// 4. 验证费用守恒：严格零增发（按 Token 严格相等且无额外 Token）
	if err := v.validateFeeConservation(actualFees, expectedFees.ByToken); err != nil {
		return fmt.Errorf("费用守恒验证失败: %w", err)
	}

	return nil
}

// validateFeeConservation 验证费用守恒（按Token）
//
// 验证逻辑:
//  1. Token种类数量必须一致
//  2. 每种Token的金额必须完全相等
//  3. 不能有额外的Token（增发检测）
func (v *CoinbaseValidator) validateFeeConservation(
	actual map[txiface.TokenKey]*big.Int,
	expected map[txiface.TokenKey]*big.Int,
) error {
	// 验证Token数量一致
	if len(actual) != len(expected) {
		return fmt.Errorf("Token种类数量不一致: 期望=%d, 实际=%d", len(expected), len(actual))
	}

	// 按确定性顺序验证（避免map遍历顺序不确定）
	expectedKeys := v.sortTokenKeys(expected)

	for _, tokenKey := range expectedKeys {
		expectedAmount := expected[tokenKey]
		actualAmount, ok := actual[tokenKey]

		if !ok {
			return fmt.Errorf("Token [%s]: Coinbase缺少此Token输出", tokenKey)
		}

		if actualAmount.Cmp(expectedAmount) != 0 {
			return fmt.Errorf("Token [%s]: 金额不一致, 期望=%s, 实际=%s",
				tokenKey, expectedAmount.String(), actualAmount.String())
		}
	}

	// 检查是否有额外的Token（增发检测）
	actualKeys := v.sortTokenKeys(actual)
	for _, tokenKey := range actualKeys {
		if _, ok := expected[tokenKey]; !ok {
			return fmt.Errorf("Token [%s]: Coinbase包含额外Token（增发检测）", tokenKey)
		}
	}

	return nil
}

// sortTokenKeys 对TokenKey进行字典序排序（确定性）
func (v *CoinbaseValidator) sortTokenKeys(tokenMap map[txiface.TokenKey]*big.Int) []txiface.TokenKey {
	keys := make([]txiface.TokenKey, 0, len(tokenMap))
	for k := range tokenMap {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return string(keys[i]) < string(keys[j])
	})
	return keys
}
