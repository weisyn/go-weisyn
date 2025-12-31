// Package condition_test 提供 Condition 插件的测试 Mock 对象
//
// 🧪 **测试规范遵循**：
// - 统一管理 Mock 对象，避免重复定义
// - 遵循测试规范：docs/system/standards/principles/testing-standards.md
package condition

import (
	"context"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/core/tx/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	txiface "github.com/weisyn/v1/pkg/interfaces/tx"
	utxopb "github.com/weisyn/v1/pb/blockchain/utxo"
)

// MockVerifierEnvironment 模拟验证环境
type MockVerifierEnvironment struct {
	blockHeight  uint64
	blockTime    uint64
	minerAddress []byte
	chainID      []byte
	utxoQuery    *testutil.MockUTXOQuery
	nonceMap     map[string]uint64 // address -> nonce
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
	return nil
}

func (m *MockVerifierEnvironment) IsCoinbase(tx *transaction.Transaction) bool {
	return len(tx.Inputs) == 0
}

func (m *MockVerifierEnvironment) GetNonce(ctx context.Context, address []byte) (uint64, error) {
	if m.nonceMap == nil {
		return 0, nil
	}
	addrStr := string(address)
	nonce, ok := m.nonceMap[addrStr]
	if !ok {
		return 0, nil // 默认 nonce 为 0
	}
	return nonce, nil
}

func (m *MockVerifierEnvironment) GetPublicKey(ctx context.Context, address []byte) ([]byte, error) {
	return nil, nil
}

func (m *MockVerifierEnvironment) GetTxBlockHeight(ctx context.Context, txID []byte) (uint64, error) {
	return 0, nil
}

func (m *MockVerifierEnvironment) IsSponsorClaim(tx *transaction.Transaction) bool {
	return false
}

