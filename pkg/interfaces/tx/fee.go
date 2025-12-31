// Package tx 提供交易处理的公共接口定义
//
// 📋 **fee.go - 费用管理接口**
//
// 本文件定义交易费用计算、聚合和Coinbase构建的公共接口。
// 遵循零增发激励机制设计，Coinbase仅聚合手续费，无区块奖励。
package tx

import (
	"context"
	"math/big"

	transaction_pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ============================================================================
//                            费用数据结构
// ============================================================================

// TokenKey Token唯一标识
//
// 格式规范:
//   - 原生币: "native"
//   - 合约代币: "contract:{contractAddress}:{tokenClassId}"
//
// 示例:
//   - "native"
//   - "contract:0x1234abcd:token-001"
type TokenKey string

// AggregatedFees 聚合费用（按Token分组）
//
// 🎯 **零增发费用模型**
//
// 用于表示多笔交易的手续费总和，支持多种代币类型。
// 每种代币类型独立聚合，用于构建Coinbase交易的多个输出。
//
// 使用场景:
//   1. 矿工构建Coinbase: 聚合候选交易的所有手续费
//   2. 验证者验证Coinbase: 重新计算并对比费用守恒
//
// 示例:
//
//	fees := &AggregatedFees{
//	    ByToken: map[TokenKey]*big.Int{
//	        "native":                      big.NewInt(1000000),  // 1 WES
//	        "contract:0xabc:token-class-1": big.NewInt(500),      // 500 tokens
//	    },
//	}
type AggregatedFees struct {
	ByToken map[TokenKey]*big.Int // TokenKey -> 总金额
}

// ============================================================================
//                            费用管理接口
// ============================================================================

// FeeManager 费用管理器接口
//
// 🎯 **核心职责**: 交易费用计算、聚合、Coinbase构建与验证
//
// 设计原则:
//   - 零增发: Coinbase仅聚合手续费，BlockReward = 0
//   - 多代币支持: 支持原生币和合约代币作为手续费
//   - 费用守恒: 输入总值 = 输出总值 + 手续费（按Token类型分别守恒）
//
// 依赖模块:
//   - UTXOManager: 获取交易输入引用的UTXO（计算输入总值）
//   - Calculator: 计算单笔交易的输入输出差额
//   - CoinbaseBuilder: 构建零增发Coinbase交易
//
// 实现位置:
//   - internal/core/tx/ports/fee/manager.go
type FeeManager interface {
	// CalculateTransactionFee 计算单笔交易的费用（输入-输出差额）
	//
	// 🎯 **费用计算核心逻辑**
	//
	// 计算公式（按Token类型）:
	//   Fee(token) = Sum(Inputs(token)) - Sum(Outputs(token))
	//
	// 步骤:
	//   1. 遍历交易输入，通过OutPoint查询UTXO，提取金额并按Token分组
	//   2. 遍历交易输出，提取金额并按Token分组
	//   3. 计算差额: inputs - outputs
	//
	// 参数:
	//   ctx: 上下文对象
	//   tx: 待计算的交易
	//
	// 返回:
	//   *AggregatedFees: 按Token分组的费用
	//   error: 计算错误（如UTXO不存在、金额溢出等）
	//
	// 注意:
	//   - Coinbase交易（无输入）费用为0
	//   - 如果输出总值 > 输入总值，返回错误（费用不能为负）
	CalculateTransactionFee(ctx context.Context, tx *transaction_pb.Transaction) (*AggregatedFees, error)

	// AggregateFees 聚合多笔交易的费用
	//
	// 🎯 **费用聚合逻辑**
	//
	// 将多笔交易的费用按Token类型合并，生成总费用。
	// 用于矿工构建Coinbase或验证者验证区块手续费。
	//
	// 参数:
	//   fees: 多笔交易的费用列表
	//
	// 返回:
	//   *AggregatedFees: 聚合后的总费用
	//
	// 示例:
	//
	//	fee1 := &AggregatedFees{ByToken: map[TokenKey]*big.Int{"native": big.NewInt(100)}}
	//	fee2 := &AggregatedFees{ByToken: map[TokenKey]*big.Int{"native": big.NewInt(200)}}
	//	total := feeManager.AggregateFees([]*AggregatedFees{fee1, fee2})
	//	// total.ByToken["native"] == 300
	AggregateFees(fees []*AggregatedFees) *AggregatedFees

	// BuildCoinbase 构建Coinbase交易（零增发：仅聚合手续费）
	//
	// 🎯 **零增发Coinbase构建**
	//
	// Coinbase特征:
	//   - 无输入（len(Inputs) == 0）
	//   - 输出 = 手续费聚合（按Token类型分别创建输出）
	//   - 所有输出Owner = minerAddr
	//   - 无BlockReward（零增发核心）
	//
	// 参数:
	//   aggregatedFees: 聚合后的手续费
	//   minerAddr: 矿工地址（所有输出的所有者）
	//   chainID: 链ID（交易所属链）
	//
	// 返回:
	//   *Transaction: 构建的Coinbase交易
	//   error: 构建错误
	//
	// 输出结构:
	//   - 每种Token类型生成一个输出
	//   - 锁定条件: SingleKeyLock(minerAddr)
	//   - 金额 = aggregatedFees.ByToken[token]
	//
	// 示例:
	//
	//	fees := &AggregatedFees{
	//	    ByToken: map[TokenKey]*big.Int{
	//	        "native": big.NewInt(1000),
	//	        "contract:0xabc:token1": big.NewInt(500),
	//	    },
	//	}
	//	coinbase, err := feeManager.BuildCoinbase(fees, minerAddr, chainID)
	//	// coinbase.Inputs == []
	//	// coinbase.Outputs == [
	//	//     {Owner: minerAddr, Asset: NativeCoin(1000)},
	//	//     {Owner: minerAddr, Asset: ContractToken(0xabc, token1, 500)},
	//	// ]
	BuildCoinbase(
		aggregatedFees *AggregatedFees,
		minerAddr []byte,
		chainID []byte,
	) (*transaction_pb.Transaction, error)

	// ValidateCoinbase 验证Coinbase交易（费用守恒）
	//
	// 🎯 **零增发Coinbase验证**
	//
	// 验证项:
	//   1. 结构检查: len(Inputs) == 0
	//   2. 所有者检查: 所有输出Owner == minerAddr
	//   3. 费用守恒: Sum(Outputs by token) == expectedFees.ByToken[token]
	//   4. 无额外增发: 不存在 expectedFees 中没有的Token输出
	//
	// 参数:
	//   ctx: 上下文对象
	//   coinbase: 待验证的Coinbase交易
	//   expectedFees: 期望的手续费（从区块内交易重新计算）
	//   minerAddr: 矿工地址
	//
	// 返回:
	//   error: 验证失败的原因，nil表示验证通过
	//
	// 验证失败场景:
	//   - Coinbase有输入
	//   - 输出Owner不是minerAddr
	//   - 输出金额 != expectedFees（任一Token类型）
	//   - 存在未预期的Token输出（增发检测）
	ValidateCoinbase(
		ctx context.Context,
		coinbase *transaction_pb.Transaction,
		expectedFees *AggregatedFees,
		minerAddr []byte,
	) error
}

// ============================================================================
//                            辅助接口
// ============================================================================

// UTXOFetcher UTXO查询函数类型
//
// 用于费用计算时查询交易输入引用的UTXO。
// 通常由 UTXOManager.GetUTXO 提供。
type UTXOFetcher func(ctx context.Context, outpoint *transaction_pb.OutPoint) (*transaction_pb.TxOutput, error)

