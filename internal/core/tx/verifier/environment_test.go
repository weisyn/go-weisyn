// Package verifier_test 提供 VerifierEnvironment 的单元测试
//
// 🧪 **测试覆盖**：
// - StaticVerifierEnvironment 基础功能测试
// - 区块上下文查询测试
// - UTXO 查询测试
// - 错误场景测试
package verifier

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/core/tx/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxopb "github.com/weisyn/v1/pb/blockchain/utxo"
)

// ==================== StaticVerifierEnvironment 基础功能测试 ====================

// TestNewStaticVerifierEnvironment 测试创建验证环境
func TestNewStaticVerifierEnvironment(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	config := &VerifierEnvironmentConfig{
		BlockHeight:  100,
		BlockTime:    1234567890,
		MinerAddress: testutil.RandomAddress(),
		ChainID:      []byte("test-chain"),
		UTXOQuery:    utxoQuery,
	}

	env := NewStaticVerifierEnvironment(config)

	assert.NotNil(t, env)
	assert.Equal(t, uint64(100), env.GetBlockHeight())
	assert.Equal(t, uint64(1234567890), env.GetBlockTime())
	assert.Equal(t, config.MinerAddress, env.GetMinerAddress())
	assert.Equal(t, config.ChainID, env.GetChainID())
}

// TestStaticVerifierEnvironment_GetUTXO 测试获取 UTXO
func TestStaticVerifierEnvironment_GetUTXO(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	config := &VerifierEnvironmentConfig{
		BlockHeight:  100,
		BlockTime:    1234567890,
		MinerAddress: testutil.RandomAddress(),
		ChainID:      []byte("test-chain"),
		UTXOQuery:    utxoQuery,
	}
	env := NewStaticVerifierEnvironment(config)

	// 添加 UTXO
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	// 获取 UTXO
	result, err := env.GetUTXO(context.Background(), outpoint)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, utxo.Outpoint.TxId, result.Outpoint.TxId)
}

// TestStaticVerifierEnvironment_GetUTXO_NotFound 测试 UTXO 不存在
func TestStaticVerifierEnvironment_GetUTXO_NotFound(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	config := &VerifierEnvironmentConfig{
		BlockHeight:  100,
		BlockTime:    1234567890,
		MinerAddress: testutil.RandomAddress(),
		ChainID:      []byte("test-chain"),
		UTXOQuery:    utxoQuery,
	}
	env := NewStaticVerifierEnvironment(config)

	// 查询不存在的 UTXO
	outpoint := testutil.CreateOutPoint(nil, 0)
	_, err := env.GetUTXO(context.Background(), outpoint)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "UTXO not found")
}

// TestStaticVerifierEnvironment_GetOutput 测试获取 Output
func TestStaticVerifierEnvironment_GetOutput(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	config := &VerifierEnvironmentConfig{
		BlockHeight:  100,
		BlockTime:    1234567890,
		MinerAddress: testutil.RandomAddress(),
		ChainID:      []byte("test-chain"),
		UTXOQuery:    utxoQuery,
	}
	env := NewStaticVerifierEnvironment(config)

	// 添加 UTXO
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	// 获取 Output
	result, err := env.GetOutput(context.Background(), outpoint)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, output.Owner, result.Owner)
}

// TestStaticVerifierEnvironment_GetOutput_NotFound 测试 Output 不存在
func TestStaticVerifierEnvironment_GetOutput_NotFound(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	config := &VerifierEnvironmentConfig{
		BlockHeight:  100,
		BlockTime:    1234567890,
		MinerAddress: testutil.RandomAddress(),
		ChainID:      []byte("test-chain"),
		UTXOQuery:    utxoQuery,
	}
	env := NewStaticVerifierEnvironment(config)

	// 查询不存在的 Output
	outpoint := testutil.CreateOutPoint(nil, 0)
	_, err := env.GetOutput(context.Background(), outpoint)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "UTXO not found")
}

// TestStaticVerifierEnvironment_GetBlockHeight 测试获取区块高度
func TestStaticVerifierEnvironment_GetBlockHeight(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	config := &VerifierEnvironmentConfig{
		BlockHeight:  200,
		BlockTime:    1234567890,
		MinerAddress: testutil.RandomAddress(),
		ChainID:      []byte("test-chain"),
		UTXOQuery:    utxoQuery,
	}
	env := NewStaticVerifierEnvironment(config)

	height := env.GetBlockHeight()

	assert.Equal(t, uint64(200), height)
}

// TestStaticVerifierEnvironment_GetBlockTime 测试获取区块时间
func TestStaticVerifierEnvironment_GetBlockTime(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	config := &VerifierEnvironmentConfig{
		BlockHeight:  100,
		BlockTime:    9876543210,
		MinerAddress: testutil.RandomAddress(),
		ChainID:      []byte("test-chain"),
		UTXOQuery:    utxoQuery,
	}
	env := NewStaticVerifierEnvironment(config)

	blockTime := env.GetBlockTime()

	assert.Equal(t, uint64(9876543210), blockTime)
}

// TestStaticVerifierEnvironment_GetMinerAddress 测试获取矿工地址
func TestStaticVerifierEnvironment_GetMinerAddress(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	minerAddr := testutil.RandomAddress()
	config := &VerifierEnvironmentConfig{
		BlockHeight:  100,
		BlockTime:    1234567890,
		MinerAddress: minerAddr,
		ChainID:      []byte("test-chain"),
		UTXOQuery:    utxoQuery,
	}
	env := NewStaticVerifierEnvironment(config)

	address := env.GetMinerAddress()

	assert.Equal(t, minerAddr, address)
}

// TestStaticVerifierEnvironment_GetChainID 测试获取链 ID
func TestStaticVerifierEnvironment_GetChainID(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	chainID := []byte("mainnet")
	config := &VerifierEnvironmentConfig{
		BlockHeight:  100,
		BlockTime:    1234567890,
		MinerAddress: testutil.RandomAddress(),
		ChainID:      chainID,
		UTXOQuery:    utxoQuery,
	}
	env := NewStaticVerifierEnvironment(config)

	id := env.GetChainID()

	assert.Equal(t, chainID, id)
}

// TestStaticVerifierEnvironment_GetExpectedFees 测试获取期望费用
func TestStaticVerifierEnvironment_GetExpectedFees(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	config := &VerifierEnvironmentConfig{
		BlockHeight:  100,
		BlockTime:    1234567890,
		MinerAddress: testutil.RandomAddress(),
		ChainID:      []byte("test-chain"),
		UTXOQuery:    utxoQuery,
	}
	env := NewStaticVerifierEnvironment(config)

	// 获取期望费用（当前实现返回 nil）
	fees := env.GetExpectedFees()

	// 注意：当前实现返回 nil（仅在验证Coinbase时需要）
	// 这里主要测试接口调用不报错
	assert.Nil(t, fees)
}

// TestStaticVerifierEnvironment_IsCoinbase 测试判断是否为 Coinbase
func TestStaticVerifierEnvironment_IsCoinbase(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	config := &VerifierEnvironmentConfig{
		BlockHeight:  100,
		BlockTime:    1234567890,
		MinerAddress: testutil.RandomAddress(),
		ChainID:      []byte("test-chain"),
		UTXOQuery:    utxoQuery,
	}
	env := NewStaticVerifierEnvironment(config)

	// Coinbase 交易（无输入）
	coinbaseTx := &transaction.Transaction{
		Version: 1,
		Inputs:  []*transaction.TxInput{},
		Outputs: []*transaction.TxOutput{},
	}
	assert.True(t, env.IsCoinbase(coinbaseTx))

	// 非 Coinbase 交易（有输入）
	normalTx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil)),
		},
	)
	assert.False(t, env.IsCoinbase(normalTx))
}


// TestStaticVerifierEnvironment_GetNonce_NoQueryService 测试获取 Nonce（无 QueryService）
func TestStaticVerifierEnvironment_GetNonce_NoQueryService(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	config := &VerifierEnvironmentConfig{
		BlockHeight:  100,
		BlockTime:    1234567890,
		MinerAddress: testutil.RandomAddress(),
		ChainID:      []byte("test-chain"),
		UTXOQuery:    utxoQuery,
		// QueryService 为 nil
	}
	env := NewStaticVerifierEnvironment(config)

	address := testutil.RandomAddress()
	_, err := env.GetNonce(context.Background(), address)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "QueryService未提供")
}

// TestStaticVerifierEnvironment_GetPublicKey_Success 测试获取公钥（成功）
func TestStaticVerifierEnvironment_GetPublicKey_Success(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	address := testutil.RandomAddress()
	pubKey := testutil.RandomPublicKey()

	// 创建包含 SingleKeyLock 的 UTXO
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(address, "1000", testutil.CreateSingleKeyLock(pubKey))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	config := &VerifierEnvironmentConfig{
		BlockHeight:  100,
		BlockTime:    1234567890,
		MinerAddress: testutil.RandomAddress(),
		ChainID:      []byte("test-chain"),
		UTXOQuery:    utxoQuery,
	}
	env := NewStaticVerifierEnvironment(config)

	result, err := env.GetPublicKey(context.Background(), address)
	assert.NoError(t, err)
	assert.Equal(t, pubKey, result)
}

// TestStaticVerifierEnvironment_GetPublicKey_EmptyAddress 测试获取公钥（空地址）
func TestStaticVerifierEnvironment_GetPublicKey_EmptyAddress(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	config := &VerifierEnvironmentConfig{
		BlockHeight:  100,
		BlockTime:    1234567890,
		MinerAddress: testutil.RandomAddress(),
		ChainID:      []byte("test-chain"),
		UTXOQuery:    utxoQuery,
	}
	env := NewStaticVerifierEnvironment(config)

	_, err := env.GetPublicKey(context.Background(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "地址为空")
}

// TestStaticVerifierEnvironment_GetPublicKey_NotFound 测试获取公钥（未找到）
func TestStaticVerifierEnvironment_GetPublicKey_NotFound(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	config := &VerifierEnvironmentConfig{
		BlockHeight:  100,
		BlockTime:    1234567890,
		MinerAddress: testutil.RandomAddress(),
		ChainID:      []byte("test-chain"),
		UTXOQuery:    utxoQuery,
	}
	env := NewStaticVerifierEnvironment(config)

	address := testutil.RandomAddress()
	_, err := env.GetPublicKey(context.Background(), address)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无法获取地址")
}

// TestStaticVerifierEnvironment_GetTxBlockHeight_NoQueryService 测试获取交易区块高度（无 QueryService）
func TestStaticVerifierEnvironment_GetTxBlockHeight_NoQueryService(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	config := &VerifierEnvironmentConfig{
		BlockHeight:  100,
		BlockTime:    1234567890,
		MinerAddress: testutil.RandomAddress(),
		ChainID:      []byte("test-chain"),
		UTXOQuery:    utxoQuery,
		// QueryService 为 nil
	}
	env := NewStaticVerifierEnvironment(config)

	txID := []byte("test-tx-id")
	_, err := env.GetTxBlockHeight(context.Background(), txID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "QueryService未提供")
}

// TestStaticVerifierEnvironment_IsSponsorClaim 测试判断是否为赞助领取交易
func TestStaticVerifierEnvironment_IsSponsorClaim(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	config := &VerifierEnvironmentConfig{
		BlockHeight:  100,
		BlockTime:    1234567890,
		MinerAddress: testutil.RandomAddress(),
		ChainID:      []byte("test-chain"),
		UTXOQuery:    utxoQuery,
	}
	env := NewStaticVerifierEnvironment(config)

	// 赞助领取交易（单个输入，使用 DelegationProof）
	sponsorClaimTx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_DelegationProof{
					DelegationProof: &transaction.DelegationProof{
						OperationType:     "consume",
						DelegateAddress:   testutil.RandomAddress(),
						ValueAmount:       500,
						DelegateSignature: nil,
					},
				},
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "500", testutil.CreateSingleKeyLock(nil)),
		},
	)
	assert.True(t, env.IsSponsorClaim(sponsorClaimTx))

	// 非赞助领取交易（多个输入）
	normalTx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
			},
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 1),
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil)),
		},
	)
	assert.False(t, env.IsSponsorClaim(normalTx))
}

