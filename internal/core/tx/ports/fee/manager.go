package fee

import (
	"context"
	"math/big"

	txiface "github.com/weisyn/v1/pkg/interfaces/tx"
	transaction_pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// Manager FeeManager实现（组合模式）
//
// 🎯 **零增发费用管理器**
//
// 职责:
//   - 计算交易费用（委托给Calculator）
//   - 聚合多笔费用
//   - 构建Coinbase（委托给CoinbaseBuilder）
//   - 验证Coinbase（委托给CoinbaseValidator）
type Manager struct {
	calculator *Calculator
	builder    *CoinbaseBuilder
	validator  *CoinbaseValidator
}

// NewManager 创建FeeManager实例
func NewManager(utxoFetcher txiface.UTXOFetcher) *Manager {
	return &Manager{
		calculator: NewCalculator(utxoFetcher),
		builder:    NewCoinbaseBuilder(),
		validator:  NewCoinbaseValidator(),
	}
}

// 确保实现接口
var _ txiface.FeeManager = (*Manager)(nil)

// CalculateTransactionFee 实现 txiface.FeeManager
func (m *Manager) CalculateTransactionFee(
	ctx context.Context,
	tx *transaction_pb.Transaction,
) (*txiface.AggregatedFees, error) {
	return m.calculator.Calculate(ctx, tx)
}

// AggregateFees 实现 txiface.FeeManager
func (m *Manager) AggregateFees(fees []*txiface.AggregatedFees) *txiface.AggregatedFees {
	result := &txiface.AggregatedFees{
		ByToken: make(map[txiface.TokenKey]*big.Int),
	}

	for _, fee := range fees {
		for token, amount := range fee.ByToken {
			if existing, ok := result.ByToken[token]; ok {
				result.ByToken[token] = new(big.Int).Add(existing, amount)
			} else {
				result.ByToken[token] = new(big.Int).Set(amount)
			}
		}
	}

	return result
}

// BuildCoinbase 实现 txiface.FeeManager
func (m *Manager) BuildCoinbase(
	aggregatedFees *txiface.AggregatedFees,
	minerAddr []byte,
	chainID []byte,
) (*transaction_pb.Transaction, error) {
	return m.builder.Build(aggregatedFees, minerAddr, chainID)
}

// ValidateCoinbase 实现 txiface.FeeManager
func (m *Manager) ValidateCoinbase(
	ctx context.Context,
	coinbase *transaction_pb.Transaction,
	expectedFees *txiface.AggregatedFees,
	minerAddr []byte,
) error {
	return m.validator.Validate(ctx, coinbase, expectedFees, minerAddr)
}

