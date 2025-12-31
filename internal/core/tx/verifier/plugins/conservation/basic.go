// Package conservation 提供价值守恒验证插件实现
//
// 本包实现 Conservation 钩子的各种验证插件，负责验证价值守恒（Σ输入 ≥ Σ输出 + 费用）。
package conservation

import (
	"context"
	"fmt"
	"strconv"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxopb "github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/interfaces/tx"
)

// BasicConservationPlugin 基础价值守恒验证插件
//
// 🎯 **核心职责**：验证交易的价值守恒（Σ输入 ≥ Σ输出）
//
// 💡 **设计理念**：
// 价值守恒是区块链的基础规则，确保没有凭空创造或消失的价值。
// 本插件实现最基础的价值守恒验证，支持原生币和合约代币。
//
// ⚠️ **P1 MVP 约束**：
// - 只处理 AssetOutput（忽略 Resource/State 输出）
// - 支持原生币（NativeCoinAsset）和合约代币（ContractTokenAsset）
// - 相同资产 ID 才能进行守恒验证
// - 差额即为费用，不验证最小费用（由 FeeEstimator 负责）
//
// 📞 **调用方**：Verifier Kernel（通过 Conservation Hook）
type BasicConservationPlugin struct {
	eutxoQuery persistence.UTXOQuery
}

// NewBasicConservationPlugin 创建新的 BasicConservationPlugin
//
// 参数：
//   - eutxoQuery: UTXO 管理器（用于查询输入引用的 UTXO）
//
// 返回：
//   - *BasicConservationPlugin: 新创建的实例
func NewBasicConservationPlugin(
	eutxoQuery persistence.UTXOQuery,
) *BasicConservationPlugin {
	return &BasicConservationPlugin{
		eutxoQuery: eutxoQuery,
	}
}

// Name 返回插件名称
//
// 实现 tx.ConservationPlugin 接口
//
// 返回：
//   - string: "basic_conservation"
func (p *BasicConservationPlugin) Name() string {
	return "basic_conservation"
}

// Check 检查价值守恒
//
// 实现 tx.ConservationPlugin 接口
//
// 🎯 **核心逻辑**：
// 1. 查询所有输入引用的 UTXO
// 2. 按资产 ID 分组计算输入总和（排除 is_reference_only 的输入）
// 3. 按资产 ID 分组计算输出总和
// 4. 验证：对于每种资产，Σ(输入) >= Σ(输出)
// 5. 差额即为该资产的费用
//
// ✅ **铸造场景支持**：
// - 如果交易是铸造场景（0消费型输入 + ExecutionProof + ContractTokenAsset输出），允许0输入+N输出
// - 铸造场景的合法性由AuthZ阶段验证（ExecutionProof的有效性）
//
// 参数：
//   - ctx: 上下文对象
//   - inputs: 输入 UTXO 列表（已通过 UTXOManager 获取）
//   - outputs: 输出列表（从 Transaction 中获取）
//   - tx: 完整的交易对象
//
// 返回：
//   - error: 价值守恒检查失败的原因
//   - nil: 检查通过
//   - non-nil: 检查失败，描述失败原因
func (p *BasicConservationPlugin) Check(
	ctx context.Context,
	inputs []*utxopb.UTXO,
	outputs []*transaction.TxOutput,
	tx *transaction.Transaction,
) error {
	// ✅ 检测是否为铸造场景
	isMinting := p.isMintingScenario(tx, outputs)
	if isMinting {
		// 铸造场景：允许0消费型输入+N输出
		// 合法性由AuthZ阶段验证（ExecutionProof的有效性）
		return nil
	}

	// 1. 计算输入总和（按资产分组）
	inputSums := make(map[string]uint64) // assetID -> amount
	for i, utxo := range inputs {
		// 检查是否为引用型输入（is_reference_only）
		if i < len(tx.Inputs) && tx.Inputs[i].IsReferenceOnly {
			// 引用型输入不计入价值守恒验证
			continue
		}

		// 提取 AssetOutput（只验证资产输出）
		txOutput := utxo.GetCachedOutput()
		if txOutput == nil {
			continue // 没有缓存输出，跳过
		}

		assetOutput := txOutput.GetAsset()
		if assetOutput == nil {
			continue // 非资产输出，跳过
		}

		// 提取资产 ID 和金额
		assetID, amount, err := p.extractAssetInfo(assetOutput)
		if err != nil {
			return fmt.Errorf("输入 %d: 提取资产信息失败: %w", i, err)
		}

		inputSums[assetID] += amount
	}

	// 2. 计算输出总和（按资产分组）
	outputSums := make(map[string]uint64) // assetID -> amount
	for i, output := range outputs {
		assetOutput := output.GetAsset()
		if assetOutput == nil {
			continue // 非资产输出，跳过
		}

		// 提取资产 ID 和金额
		assetID, amount, err := p.extractAssetInfo(assetOutput)
		if err != nil {
			return fmt.Errorf("输出 %d: 提取资产信息失败: %w", i, err)
		}

		outputSums[assetID] += amount
	}

	// 3. 验证价值守恒
	for assetID, outputSum := range outputSums {
		inputSum := inputSums[assetID]
		if inputSum < outputSum {
			return fmt.Errorf(
				"价值守恒验证失败，资产 %s: 输入总额=%d < 输出总额=%d",
				assetID, inputSum, outputSum,
			)
		}
		// 注意：差额（inputSum - outputSum）即为该资产的费用
	}

	return nil
}

// isMintingScenario 检测是否为铸造场景
//
// 🎯 **铸造场景判断条件**（必须同时满足）：
// 1. 消费型输入数量为0（可以有引用型输入）
// 2. 输出包含 ContractTokenAsset
// 3. 存在 ExecutionProof（在引用型输入的 UnlockingProof 中）
//
// 参数：
//   - tx: 完整的交易对象
//   - outputs: 输出列表
//
// 返回：
//   - bool: true 表示是铸造场景，false 表示不是
func (p *BasicConservationPlugin) isMintingScenario(
	tx *transaction.Transaction,
	outputs []*transaction.TxOutput,
) bool {
	// 1. 检查消费型输入数量是否为0
	consumingInputCount := 0
	hasExecutionProof := false

	for _, input := range tx.Inputs {
		if !input.IsReferenceOnly {
			// 消费型输入
			consumingInputCount++
		} else {
			// 引用型输入：检查是否有 ExecutionProof
			if input.UnlockingProof != nil {
				if _, ok := input.UnlockingProof.(*transaction.TxInput_ExecutionProof); ok {
					hasExecutionProof = true
				}
			}
		}
	}

	if consumingInputCount > 0 {
		// 有消费型输入，不是铸造场景
		return false
	}

	// 2. 检查输出是否包含 ContractTokenAsset
	hasContractTokenOutput := false
	for _, output := range outputs {
		if asset := output.GetAsset(); asset != nil {
			if contractToken := asset.GetContractToken(); contractToken != nil {
				hasContractTokenOutput = true
				break
			}
		}
	}

	if !hasContractTokenOutput {
		// 没有 ContractTokenAsset 输出，不是铸造场景
		return false
	}

	// 3. 检查是否存在 ExecutionProof
	if !hasExecutionProof {
		// 没有 ExecutionProof，不是铸造场景
		return false
	}

	// ✅ 同时满足三个条件：是铸造场景
	return true
}

// extractAssetInfo 从 AssetOutput 中提取资产 ID 和金额
//
// 参数：
//   - assetOutput: AssetOutput（可能是 NativeCoinAsset 或 ContractTokenAsset）
//
// 返回：
//   - string: 资产 ID
//   - uint64: 金额
//   - error: 提取失败
func (p *BasicConservationPlugin) extractAssetInfo(
	assetOutput *transaction.AssetOutput,
) (string, uint64, error) {
	switch asset := assetOutput.AssetContent.(type) {
	case *transaction.AssetOutput_NativeCoin:
		// 原生币
		if asset.NativeCoin == nil {
			return "", 0, fmt.Errorf("NativeCoin 为空")
		}

		// 资产 ID：原生币使用固定标识
		assetID := "native"

		// 金额：字符串转 uint64
		amount, err := strconv.ParseUint(asset.NativeCoin.Amount, 10, 64)
		if err != nil {
			return "", 0, fmt.Errorf("解析原生币金额失败: %w", err)
		}

		return assetID, amount, nil

	case *transaction.AssetOutput_ContractToken:
		// 合约代币
		if asset.ContractToken == nil {
			return "", 0, fmt.Errorf("ContractToken 为空")
		}

		// 资产 ID：使用 contract_address
		if len(asset.ContractToken.ContractAddress) == 0 {
			return "", 0, fmt.Errorf("合约地址为空")
		}
		assetID := fmt.Sprintf("contract:%x", asset.ContractToken.ContractAddress)

		// 金额：字符串转 uint64
		amount, err := strconv.ParseUint(asset.ContractToken.Amount, 10, 64)
		if err != nil {
			return "", 0, fmt.Errorf("解析合约代币金额失败: %w", err)
		}

		return assetID, amount, nil

	default:
		// P1 阶段不支持其他资产类型
		return "", 0, fmt.Errorf("不支持的资产类型: %T", assetOutput.AssetContent)
	}
}

// 编译期检查：确保 BasicConservationPlugin 实现了 tx.ConservationPlugin 接口
var _ tx.ConservationPlugin = (*BasicConservationPlugin)(nil)
