// Package validator 提供聚合节点验证功能
//
// 本包实现聚合节点对区块的验证逻辑，包括激励交易的区块级验证。
package validator

import (
	"context"
	"fmt"
    "bytes"

	configiface "github.com/weisyn/v1/pkg/interfaces/config"
	txiface "github.com/weisyn/v1/pkg/interfaces/tx"
	block_pb "github.com/weisyn/v1/pb/blockchain/block"
	transaction_pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/constants"
)

// IncentiveValidator 激励交易验证器（聚合节点侧）
//
// 🎯 **聚合节点区块级验证**
//
// 职责:
//   - 验证区块激励区（Coinbase + 赞助区）
//   - 确保费用守恒和结构正确
type IncentiveValidator struct {
	feeManager txiface.FeeManager      // 费用管理器（包含Coinbase验证功能）
	config     configiface.Provider    // 配置提供者（中优先级-2：用于获取赞助策略）
	eutxoQuery persistence.UTXOQuery        // 用于收紧激励区识别（必须引用赞助池UTXO）
}

// NewIncentiveValidator 创建激励验证器
//
// 参数:
//   feeManager: 费用管理器
//   config: 配置提供者（用于获取赞助策略约束）
func NewIncentiveValidator(feeManager txiface.FeeManager, config configiface.Provider, eutxoQuery persistence.UTXOQuery) *IncentiveValidator {
	return &IncentiveValidator{
		feeManager: feeManager,
		config:     config,
		eutxoQuery: eutxoQuery,
	}
}

// ValidateIncentiveTxs 验证激励交易区
//
// 在 BlockManager.ValidateBlock() 中调用。
//
// 参数:
//   ctx: 上下文
//   block: 待验证的区块
//   minerAddr: 矿工地址
//
// 返回:
//   error: 验证失败原因，nil表示通过
func (v *IncentiveValidator) ValidateIncentiveTxs(
	ctx context.Context,
	block *block_pb.Block,
	minerAddr []byte,
) error {
	txs := block.Body.Transactions
	if len(txs) == 0 {
		return fmt.Errorf("区块交易列表为空")
	}

	// 1. 验证Coinbase（必须是首笔）
	coinbase := txs[0]
	if len(coinbase.Inputs) != 0 {
		return fmt.Errorf("首笔交易不是Coinbase（输入数=%d）", len(coinbase.Inputs))
	}

	// 1.1 识别激励区：Coinbase + 后续的赞助领取交易
    incentiveEndIndex := v.findIncentiveZoneEnd(ctx, txs)
	normalTxs := txs[incentiveEndIndex:]

	// 1.2 计算期望费用（从普通交易）
	expectedFees, err := v.calculateExpectedFees(ctx, normalTxs)
	if err != nil {
		return fmt.Errorf("计算期望费用失败: %w", err)
	}

	// 1.3 验证Coinbase费用守恒（通过FeeManager接口）
	if err := v.feeManager.ValidateCoinbase(ctx, coinbase, expectedFees, minerAddr); err != nil {
		return fmt.Errorf("Coinbase验证失败: %w", err)
	}

	// 2. 验证赞助领取交易（如有）
	sponsorClaimTxs := txs[1:incentiveEndIndex]
	if len(sponsorClaimTxs) > 0 {
		// 2.1 验证赞助领取交易数量上限（中优先级-2）
		if err := v.validateSponsorClaimCount(sponsorClaimTxs); err != nil {
			return fmt.Errorf("赞助领取交易数量验证失败: %w", err)
		}

		// 2.2 简单验证：赞助领取交易必须有1个输入（引用赞助池UTXO）
		// 详细验证应由TxVerifier的SponsorClaimPlugin处理
        for i, tx := range sponsorClaimTxs {
			if len(tx.Inputs) != 1 {
				return fmt.Errorf("赞助领取交易[%d]必须有且仅有1个输入", i+1)
			}
			if tx.Inputs[0].GetDelegationProof() == nil {
				return fmt.Errorf("赞助领取交易[%d]必须使用DelegationProof", i+1)
			}

            // 2.3 强制：引用赞助池UTXO（Owner=SponsorPoolOwner）
            if v.eutxoQuery != nil {
                utxo, err := v.eutxoQuery.GetUTXO(ctx, tx.Inputs[0].PreviousOutput)
                if err != nil {
                    return fmt.Errorf("赞助领取交易[%d] 查询UTXO失败: %w", i+1, err)
                }
                if utxo == nil || utxo.GetCachedOutput() == nil {
                    return fmt.Errorf("赞助领取交易[%d] 引用的UTXO不存在", i+1)
                }
                if !bytes.Equal(utxo.GetCachedOutput().Owner, constants.SponsorPoolOwner[:]) {
                    return fmt.Errorf("赞助领取交易[%d] 未引用赞助池UTXO", i+1)
                }
            }
		}
	}

	return nil
}

// findIncentiveZoneEnd 查找激励区结束位置
//
// 规则:
//   - index 0 是 Coinbase（无输入）
//   - index 1..N 是赞助领取交易（有DelegationProof）
//   - 后续是普通交易
//
// 返回值: 第一个普通交易的索引
func (v *IncentiveValidator) findIncentiveZoneEnd(ctx context.Context, txs []*transaction_pb.Transaction) int {
    for i := 1; i < len(txs); i++ {
        tx := txs[i]
        // 赞助领取交易特征：1个输入 + DelegationProof
        if len(tx.Inputs) == 1 && tx.Inputs[0].GetDelegationProof() != nil {
            if v.eutxoQuery != nil {
                utxo, err := v.eutxoQuery.GetUTXO(ctx, tx.Inputs[0].PreviousOutput)
                if err == nil && utxo != nil && utxo.GetCachedOutput() != nil && bytes.Equal(utxo.GetCachedOutput().Owner, constants.SponsorPoolOwner[:]) {
                    continue // 属于激励区
                }
            } else {
                // 无法查询时，保持弱识别，交由Tx插件进一步严格校验
                continue
            }
        }
        // 遇到第一个不符合赞助特征的交易，激励区结束
        return i
    }
    // 所有交易都是激励交易（极端情况）
    return len(txs)
}

// calculateExpectedFees 计算期望手续费
func (v *IncentiveValidator) calculateExpectedFees(
	ctx context.Context,
	normalTxs []*transaction_pb.Transaction,
) (*txiface.AggregatedFees, error) {
	var allFees []*txiface.AggregatedFees
	for _, tx := range normalTxs {
		fee, err := v.feeManager.CalculateTransactionFee(ctx, tx)
		if err != nil {
			return nil, err
		}
		allFees = append(allFees, fee)
	}
	return v.feeManager.AggregateFees(allFees), nil
}

// validateSponsorClaimCount 验证赞助领取交易数量（中优先级-2）
//
// 验证内容:
//   - 赞助领取交易数量不超过配置上限（MaxPerBlock）
//
// 参数:
//   sponsorClaimTxs: 赞助领取交易列表
//
// 返回:
//   error: 验证错误，nil表示通过
func (v *IncentiveValidator) validateSponsorClaimCount(
	sponsorClaimTxs []*transaction_pb.Transaction,
) error {
	// 获取配置中的上限
	if v.config == nil {
		// 配置未提供，跳过检查（向后兼容）
		return nil
	}

	consensusCfg := v.config.GetConsensus()
	if consensusCfg == nil || !consensusCfg.Miner.SponsorIncentive.Enabled {
		// 赞助激励未启用，跳过检查
		return nil
	}

	maxPerBlock := consensusCfg.Miner.SponsorIncentive.MaxPerBlock
	if maxPerBlock == 0 {
		// 无上限
		return nil
	}

	// 检查数量是否超过上限
	actualCount := len(sponsorClaimTxs)
	if actualCount > maxPerBlock {
		return fmt.Errorf("赞助领取交易数量超过上限：实际=%d，上限=%d", actualCount, maxPerBlock)
	}

	return nil
}

