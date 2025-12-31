// Package incentive_test 提供 Incentive 插件的测试 Mock 对象
//
// 🧪 **测试规范遵循**：
// - 统一管理 Mock 对象，避免重复定义
// - 遵循测试规范：docs/system/standards/principles/testing-standards.md
package incentive

import (
	"context"
	"math/big"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/core/tx/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	txiface "github.com/weisyn/v1/pkg/interfaces/tx"
	utxopb "github.com/weisyn/v1/pb/blockchain/utxo"
)

// MockFeeManager 模拟 FeeManager
type MockFeeManager struct {
	calculateFeeFunc func(ctx context.Context, tx *transaction.Transaction) (*txiface.AggregatedFees, error)
	aggregateFeesFunc func(fees []*txiface.AggregatedFees) *txiface.AggregatedFees
	buildCoinbaseFunc func(aggregatedFees *txiface.AggregatedFees, minerAddr []byte, chainID []byte) (*transaction.Transaction, error)
	validateCoinbaseFunc func(ctx context.Context, coinbase *transaction.Transaction, expectedFees *txiface.AggregatedFees, minerAddr []byte) error
}

func (m *MockFeeManager) CalculateTransactionFee(ctx context.Context, tx *transaction.Transaction) (*txiface.AggregatedFees, error) {
	if m.calculateFeeFunc != nil {
		return m.calculateFeeFunc(ctx, tx)
	}
	return &txiface.AggregatedFees{ByToken: make(map[txiface.TokenKey]*big.Int)}, nil
}

func (m *MockFeeManager) AggregateFees(fees []*txiface.AggregatedFees) *txiface.AggregatedFees {
	if m.aggregateFeesFunc != nil {
		return m.aggregateFeesFunc(fees)
	}
	return &txiface.AggregatedFees{ByToken: make(map[txiface.TokenKey]*big.Int)}
}

func (m *MockFeeManager) BuildCoinbase(aggregatedFees *txiface.AggregatedFees, minerAddr []byte, chainID []byte) (*transaction.Transaction, error) {
	if m.buildCoinbaseFunc != nil {
		return m.buildCoinbaseFunc(aggregatedFees, minerAddr, chainID)
	}
	return &transaction.Transaction{}, nil
}

func (m *MockFeeManager) ValidateCoinbase(ctx context.Context, coinbase *transaction.Transaction, expectedFees *txiface.AggregatedFees, minerAddr []byte) error {
	if m.validateCoinbaseFunc != nil {
		return m.validateCoinbaseFunc(ctx, coinbase, expectedFees, minerAddr)
	}
	return nil
}

// MockVerifierEnvironment 模拟验证环境
type MockVerifierEnvironment struct {
	blockHeight      uint64
	blockTime        uint64
	minerAddress     []byte
	chainID          []byte
	utxoQuery        *testutil.MockUTXOQuery
	expectedFees     *txiface.AggregatedFees
	nonce            uint64
	nonceError       error
	publicKey        []byte
	publicKeyError   error
	txBlockHeight    uint64
	txBlockHeightError error
}

func (m *MockVerifierEnvironment) GetBlockHeight() uint64 {
	return m.blockHeight
}

func (m *MockVerifierEnvironment) GetBlockTime() uint64 {
	return m.blockTime
}

func (m *MockVerifierEnvironment) GetMinerAddress() []byte {
	return m.minerAddress
}

func (m *MockVerifierEnvironment) GetChainID() []byte {
	return m.chainID
}

func (m *MockVerifierEnvironment) GetUTXO(ctx context.Context, outpoint *transaction.OutPoint) (*utxopb.UTXO, error) {
	if m.utxoQuery == nil {
		return nil, assert.AnError
	}
	return m.utxoQuery.GetUTXO(ctx, outpoint)
}

func (m *MockVerifierEnvironment) GetOutput(ctx context.Context, outpoint *transaction.OutPoint) (*transaction.TxOutput, error) {
	utxo, err := m.GetUTXO(ctx, outpoint)
	if err != nil {
		return nil, err
	}
	output := utxo.GetCachedOutput()
	if output == nil {
		return nil, assert.AnError
	}
	return output, nil
}

func (m *MockVerifierEnvironment) GetExpectedFees() *txiface.AggregatedFees {
	return m.expectedFees
}

func (m *MockVerifierEnvironment) IsCoinbase(tx *transaction.Transaction) bool {
	return len(tx.Inputs) == 0
}

func (m *MockVerifierEnvironment) GetNonce(ctx context.Context, address []byte) (uint64, error) {
	return m.nonce, m.nonceError
}

func (m *MockVerifierEnvironment) GetPublicKey(ctx context.Context, address []byte) ([]byte, error) {
	return m.publicKey, m.publicKeyError
}

func (m *MockVerifierEnvironment) GetTxBlockHeight(ctx context.Context, txID []byte) (uint64, error) {
	return m.txBlockHeight, m.txBlockHeightError
}

func (m *MockVerifierEnvironment) IsSponsorClaim(tx *transaction.Transaction) bool {
	return len(tx.Inputs) == 1 && tx.Inputs[0].GetDelegationProof() != nil
}

// Compile-time check to ensure MockVerifierEnvironment implements txiface.VerifierEnvironment
var _ txiface.VerifierEnvironment = (*MockVerifierEnvironment)(nil)

