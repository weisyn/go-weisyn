// Package fee_test 提供 FeeManager 的单元测试
//
// 🧪 **测试覆盖**：
// - Manager 核心功能测试
// - AggregateFees 测试
// - ValidateCoinbase 测试
package fee

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/core/tx/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	txiface "github.com/weisyn/v1/pkg/interfaces/tx"
)

// ==================== Manager 核心功能测试 ====================

// TestNewManager 测试创建 Manager
func TestNewManager(t *testing.T) {
	utxoFetcher := newMockUTXOFetcher()
	manager := NewManager(utxoFetcher)

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.calculator)
	assert.NotNil(t, manager.builder)
	assert.NotNil(t, manager.validator)
}

// TestManager_CalculateTransactionFee 测试计算交易费用
func TestManager_CalculateTransactionFee(t *testing.T) {
	utxos := make(map[string]*transaction.TxOutput)
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "2000", testutil.CreateSingleKeyLock(nil))
	utxos[fmt.Sprintf("%x:%d", outpoint.TxId, outpoint.OutputIndex)] = output

	utxoFetcher := func(ctx context.Context, op *transaction.OutPoint) (*transaction.TxOutput, error) {
		key := fmt.Sprintf("%x:%d", op.TxId, op.OutputIndex)
		if output, ok := utxos[key]; ok {
			return output, nil
		}
		return nil, fmt.Errorf("UTXO not found")
	}

	manager := NewManager(utxoFetcher)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{PreviousOutput: outpoint, IsReferenceOnly: false},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil)),
		},
	)

	fees, err := manager.CalculateTransactionFee(context.Background(), tx)

	assert.NoError(t, err)
	assert.NotNil(t, fees)
	assert.NotEmpty(t, fees.ByToken)
}

// TestManager_AggregateFees 测试聚合费用
func TestManager_AggregateFees(t *testing.T) {
	utxoFetcher := newMockUTXOFetcher()
	manager := NewManager(utxoFetcher)

	fee1 := &txiface.AggregatedFees{
		ByToken: map[txiface.TokenKey]*big.Int{
			"native": big.NewInt(100),
		},
	}

	fee2 := &txiface.AggregatedFees{
		ByToken: map[txiface.TokenKey]*big.Int{
			"native": big.NewInt(200),
		},
	}

	fee3 := &txiface.AggregatedFees{
		ByToken: map[txiface.TokenKey]*big.Int{
			"native":                    big.NewInt(50),
			"contract:0x1234:0x5678": big.NewInt(300),
		},
	}

	aggregated := manager.AggregateFees([]*txiface.AggregatedFees{fee1, fee2, fee3})

	assert.NotNil(t, aggregated)
	assert.Equal(t, "350", aggregated.ByToken["native"].String()) // 100 + 200 + 50
	assert.Equal(t, "300", aggregated.ByToken["contract:0x1234:0x5678"].String())
}

// TestManager_AggregateFees_Empty 测试空费用列表
func TestManager_AggregateFees_Empty(t *testing.T) {
	utxoFetcher := newMockUTXOFetcher()
	manager := NewManager(utxoFetcher)

	aggregated := manager.AggregateFees([]*txiface.AggregatedFees{})

	assert.NotNil(t, aggregated)
	assert.Empty(t, aggregated.ByToken)
}

// TestManager_BuildCoinbase 测试构建 Coinbase
func TestManager_BuildCoinbase(t *testing.T) {
	utxoFetcher := newMockUTXOFetcher()
	manager := NewManager(utxoFetcher)

	aggregatedFees := &txiface.AggregatedFees{
		ByToken: map[txiface.TokenKey]*big.Int{
			"native": big.NewInt(1000),
		},
	}
	minerAddr := testutil.RandomAddress()
	chainID := []byte("test-chain")

	coinbase, err := manager.BuildCoinbase(aggregatedFees, minerAddr, chainID)

	assert.NoError(t, err)
	assert.NotNil(t, coinbase)
	assert.Len(t, coinbase.Inputs, 0)
	assert.GreaterOrEqual(t, len(coinbase.Outputs), 1)
}

// TestManager_ValidateCoinbase_Success 测试验证 Coinbase 成功
func TestManager_ValidateCoinbase_Success(t *testing.T) {
	utxoFetcher := newMockUTXOFetcher()
	manager := NewManager(utxoFetcher)

	expectedFees := &txiface.AggregatedFees{
		ByToken: map[txiface.TokenKey]*big.Int{
			"native": big.NewInt(1000),
		},
	}
	minerAddr := testutil.RandomAddress()
	chainID := []byte("test-chain")

	coinbase, err := manager.BuildCoinbase(expectedFees, minerAddr, chainID)
	assert.NoError(t, err)

	// 验证 Coinbase
	err = manager.ValidateCoinbase(context.Background(), coinbase, expectedFees, minerAddr)

	assert.NoError(t, err)
}

// TestManager_ValidateCoinbase_WithInputs 测试验证有输入的 Coinbase（应该失败）
func TestManager_ValidateCoinbase_WithInputs(t *testing.T) {
	utxoFetcher := newMockUTXOFetcher()
	manager := NewManager(utxoFetcher)

	expectedFees := &txiface.AggregatedFees{
		ByToken: map[txiface.TokenKey]*big.Int{
			"native": big.NewInt(1000),
		},
	}
	minerAddr := testutil.RandomAddress()

	// 创建有输入的 Coinbase（无效）
	coinbase := &transaction.Transaction{
		Version: 1,
		Inputs: []*transaction.TxInput{
			{PreviousOutput: testutil.CreateOutPoint(nil, 0), IsReferenceOnly: false},
		},
		Outputs: []*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(minerAddr, "1000", testutil.CreateSingleKeyLock(nil)),
		},
	}

	err := manager.ValidateCoinbase(context.Background(), coinbase, expectedFees, minerAddr)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不能有输入")
}

// TestManager_ValidateCoinbase_WrongOwner 测试验证 Owner 不匹配的 Coinbase
func TestManager_ValidateCoinbase_WrongOwner(t *testing.T) {
	utxoFetcher := newMockUTXOFetcher()
	manager := NewManager(utxoFetcher)

	expectedFees := &txiface.AggregatedFees{
		ByToken: map[txiface.TokenKey]*big.Int{
			"native": big.NewInt(1000),
		},
	}
	minerAddr := testutil.RandomAddress()
	wrongOwner := testutil.RandomAddress()

	// 创建 Owner 不匹配的 Coinbase
	coinbase := &transaction.Transaction{
		Version: 1,
		Inputs:  []*transaction.TxInput{},
		Outputs: []*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(wrongOwner, "1000", testutil.CreateSingleKeyLock(nil)),
		},
	}

	err := manager.ValidateCoinbase(context.Background(), coinbase, expectedFees, minerAddr)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Owner不是矿工地址")
}

