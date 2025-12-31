// Package conservation 提供价值守恒验证插件实现
//
// utxo_diff.go: 默认UTXO差额验证插件
package conservation

import (
	"context"
	"fmt"
	"math/big"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/interfaces/tx"
)

// DefaultUTXODiffPlugin 默认UTXO差额验证插件
//
// 🎯 **核心职责**：验证基础价值守恒 - Σ(输入) ≥ Σ(输出) + 费用
//
// 💡 **设计理念**：
// 这是最基础的价值守恒验证,确保交易不会凭空创造价值(除非是 Coinbase 交易)。
// 默认情况下,交易费用 = Σ(输入) - Σ(输出),即 UTXO 差额。
//
// ⚠️ **验证规则**：
// 1. 对于每种代币类型,分别计算输入输出总和
// 2. 原生代币: Σ(输入) ≥ Σ(输出) (差额作为交易费)
// 3. 合约代币: Σ(输入) ≥ Σ(输出) (差额作为销毁或费用)
// 4. Coinbase交易(0输入): 跳过验证(由共识层控制)
//
// 🔒 **核心约束**：
// - 插件无状态：不存储验证结果
// - 插件只读：不修改交易
// - 并发安全：多个 goroutine 可以同时调用
//
// 📞 **调用方**：Verifier Kernel（通过 Conservation Hook）
type DefaultUTXODiffPlugin struct{}

// NewDefaultUTXODiffPlugin 创建新的 DefaultUTXODiffPlugin
//
// 返回：
//   - *DefaultUTXODiffPlugin: 新创建的实例
func NewDefaultUTXODiffPlugin() *DefaultUTXODiffPlugin {
	return &DefaultUTXODiffPlugin{}
}

// Name 返回插件名称
//
// 实现 tx.ConservationPlugin 接口
//
// 返回：
//   - string: "default_utxo_diff"
func (p *DefaultUTXODiffPlugin) Name() string {
	return "default_utxo_diff"
}

// Check 检查价值守恒
//
// 实现 tx.ConservationPlugin 接口
//
// 🎯 **核心逻辑**：
// 1. 如果是 Coinbase 交易(0输入),跳过验证
// 2. 从 inputs UTXO 列表计算每种代币的输入总和
// 3. 从 outputs 列表计算每种代币的输出总和
// 4. 对每种代币: Σ(输入) ≥ Σ(输出)
//
// ⚠️ **重要约束**：
// - 本插件不检查费用机制约束(由其他插件处理)
// - 本插件只验证基础守恒: 输入 >= 输出
// - 差额(输入 - 输出)即为交易费,由矿工获得
//
// 参数：
//   - ctx: 上下文对象
//   - inputs: 输入 UTXO 列表（已通过 UTXOManager 获取）
//   - outputs: 输出列表（从 Transaction 中获取）
//   - tx: 完整的交易对象
//
// 返回：
//   - error: 验证失败的原因
//   - nil: 验证通过
//   - non-nil: 价值不守恒
//
// 📝 **使用场景**：
//
//	// 正常转账: 100输入 = 70输出 + 25找零 + 5费用
//	inputs:  [{native_coin: 100}]
//	outputs: [{native_coin: 70}, {native_coin: 25}]
//	fee:     5 (隐含在差额中)
//	err := plugin.Check(ctx, inputs, outputs, tx)  // nil（验证通过）
//
//	// 价值不守恒: 100输入 < 150输出（非法）
//	inputs:  [{native_coin: 100}]
//	outputs: [{native_coin: 150}]
//	err := plugin.Check(ctx, inputs, outputs, tx)  // error（凭空创造了50）
func (p *DefaultUTXODiffPlugin) Check(
	ctx context.Context,
	inputs []*utxo.UTXO,
	outputs []*transaction.TxOutput,
	tx *transaction.Transaction,
) error {
	// 1. Coinbase 交易检查(0输入)
	if len(inputs) == 0 {
		// Coinbase 交易由共识层验证,跳过价值守恒检查
		return nil
	}

	// 2. 计算输入总和(按代币类型分组)
	inputSums := p.calculateInputSumsFromUTXOs(inputs)

	// 3. 计算输出总和(按代币类型分组)
	outputSums := p.calculateOutputSumsFromList(outputs)

	// 4. 验证每种代币的守恒: Σ(输入) ≥ Σ(输出)
	for tokenKey, outputSum := range outputSums {
		inputSum, exists := inputSums[tokenKey]
		if !exists {
			return fmt.Errorf(
				"代币 %s: 输出总和=%s,但没有对应的输入",
				tokenKey,
				outputSum,
			)
		}

		// 比较输入输出(字符串数值比较)
		if !p.isGreaterOrEqual(inputSum, outputSum) {
			return fmt.Errorf(
				"代币 %s: 价值不守恒 - 输入总和=%s < 输出总和=%s",
				tokenKey,
				inputSum,
				outputSum,
			)
		}
	}

	return nil
}

// calculateInputSumsFromUTXOs 从 UTXO 列表计算输入总和(按代币类型分组)
//
// 参数：
//   - inputs: 输入 UTXO 列表
//
// 返回：
//   - map[string]string: 代币类型 -> 输入总和
func (p *DefaultUTXODiffPlugin) calculateInputSumsFromUTXOs(
	inputs []*utxo.UTXO,
) map[string]string {
	sums := make(map[string]string)

	for _, utxo := range inputs {
		// 提取 TxOutput
		txOutput := utxo.GetCachedOutput()
		if txOutput == nil {
			// UTXO 没有缓存的 TxOutput,跳过
			continue
		}

		// 提取资产信息
		assetOutput := txOutput.GetAsset()
		if assetOutput == nil {
			// 非资产输出(如 Resource/State),跳过价值计算
			continue
		}

		// 获取代币类型和数量
		tokenKey, amount := p.extractAssetInfo(assetOutput)
		if tokenKey == "" {
			continue
		}

		// 累加到 sums
		currentSum, exists := sums[tokenKey]
		if !exists {
			sums[tokenKey] = amount
		} else {
			sums[tokenKey] = p.addAmounts(currentSum, amount)
		}
	}

	return sums
}

// calculateOutputSumsFromList 从输出列表计算输出总和(按代币类型分组)
//
// 参数：
//   - outputs: 输出列表
//
// 返回：
//   - map[string]string: 代币类型 -> 输出总和
func (p *DefaultUTXODiffPlugin) calculateOutputSumsFromList(
	outputs []*transaction.TxOutput,
) map[string]string {
	sums := make(map[string]string)

	for _, output := range outputs {
		// 提取资产信息
		assetOutput := output.GetAsset()
		if assetOutput == nil {
			// 非资产输出(如 Resource/State),跳过价值计算
			continue
		}

		// 获取代币类型和数量
		tokenKey, amount := p.extractAssetInfo(assetOutput)
		if tokenKey == "" {
			continue
		}

		// 累加到 sums
		currentSum, exists := sums[tokenKey]
		if !exists {
			sums[tokenKey] = amount
		} else {
			sums[tokenKey] = p.addAmounts(currentSum, amount)
		}
	}

	return sums
}

// extractAssetInfo 从 AssetOutput 提取代币类型和数量
//
// 参数：
//   - assetOutput: 资产输出
//
// 返回：
//   - string: 代币类型标识符
//   - string: 数量(字符串表示)
func (p *DefaultUTXODiffPlugin) extractAssetInfo(
	assetOutput *transaction.AssetOutput,
) (string, string) {
	// 原生代币
	if nativeCoin := assetOutput.GetNativeCoin(); nativeCoin != nil {
		return "native", nativeCoin.Amount
	}

	// 合约代币
	if contractToken := assetOutput.GetContractToken(); contractToken != nil {
		// 使用合约地址作为代币类型标识
		tokenKey := fmt.Sprintf("contract:%x", contractToken.ContractAddress)
		return tokenKey, contractToken.Amount
	}

	return "", ""
}

// addAmounts 字符串数值相加
//
// 参数：
//   - a: 数值1(字符串)
//   - b: 数值2(字符串)
//
// 返回：
//   - string: a + b(字符串)
//
// 使用 big.Int 进行精确计算，支持任意大小的整数
func (p *DefaultUTXODiffPlugin) addAmounts(a, b string) string {
	// 使用 big.Int 进行精确计算
	aVal := new(big.Int)
	bVal := new(big.Int)
	
	// 解析字符串为 big.Int
	aVal.SetString(a, 10)
	bVal.SetString(b, 10)
	
	// 执行加法
	result := new(big.Int).Add(aVal, bVal)
	
	return result.String()
}

// isGreaterOrEqual 比较字符串数值: a >= b
//
// 参数：
//   - a: 数值1(字符串)
//   - b: 数值2(字符串)
//
// 返回：
//   - bool: a >= b
//
// 使用 big.Int 进行精确比较，支持任意大小的整数
func (p *DefaultUTXODiffPlugin) isGreaterOrEqual(a, b string) bool {
	// 使用 big.Int 进行精确比较
	aVal := new(big.Int)
	bVal := new(big.Int)
	
	// 解析字符串为 big.Int
	aVal.SetString(a, 10)
	bVal.SetString(b, 10)
	
	// 比较：返回 -1 (a < b), 0 (a == b), 1 (a > b)
	cmp := aVal.Cmp(bVal)
	return cmp >= 0 // a >= b
}

// 编译期检查：确保 DefaultUTXODiffPlugin 实现了 tx.ConservationPlugin 接口
var _ tx.ConservationPlugin = (*DefaultUTXODiffPlugin)(nil)
