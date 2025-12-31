// Package condition_test 提供 HeightLockPlugin 的单元测试
//
// 🧪 **测试规范遵循**：
// - 每个源文件对应一个测试文件
// - 遵循测试规范：docs/system/standards/principles/testing-standards.md
package condition

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/core/tx/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	txiface "github.com/weisyn/v1/pkg/interfaces/tx"
	utxopb "github.com/weisyn/v1/pb/blockchain/utxo"
)

// ==================== HeightLockPlugin 测试 ====================

// TestNewHeightLockPlugin 测试创建 HeightLockPlugin
func TestNewHeightLockPlugin(t *testing.T) {
	plugin := NewHeightLockPlugin()

	assert.NotNil(t, plugin)
}

// TestHeightLockPlugin_Name 测试插件名称
func TestHeightLockPlugin_Name(t *testing.T) {
	plugin := NewHeightLockPlugin()

	assert.Equal(t, "HeightLock", plugin.Name())
}

// TestHeightLockPlugin_Check_Success 测试高度锁验证成功
func TestHeightLockPlugin_Check_Success(t *testing.T) {
	plugin := NewHeightLockPlugin()
	utxoQuery := testutil.NewMockUTXOQuery()

	// 创建高度锁 UTXO（解锁高度已过）
	unlockHeight := uint64(50)
	currentHeight := uint64(100)
	outpoint := testutil.CreateOutPoint(nil, 0)
	heightLock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_HeightLock{
			HeightLock: &transaction.HeightLock{
				UnlockHeight: unlockHeight,
				BaseLock:     testutil.CreateSingleKeyLock(nil),
			},
		},
	}
	output := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", heightLock)
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	// 创建交易（包含 HeightProof）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_HeightProof{
					HeightProof: &transaction.HeightProof{
						BaseProof: testutil.CreateSingleKeyProof(nil, nil),
					},
				},
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "900", testutil.CreateSingleKeyLock(nil)),
		},
	)

	// 创建模拟环境
	env := &MockVerifierEnvironment{
		blockHeight: currentHeight,
		blockTime:   uint64(time.Now().Unix()),
		utxoQuery:   utxoQuery,
	}

	// 将环境注入到 context
	ctx := txiface.WithVerifierEnvironment(context.Background(), env)

	// 验证应该成功
	err := plugin.Check(ctx, tx, currentHeight, uint64(time.Now().Unix()))

	assert.NoError(t, err)
}

// TestHeightLockPlugin_Check_NotUnlocked 测试高度锁未解锁
func TestHeightLockPlugin_Check_NotUnlocked(t *testing.T) {
	plugin := NewHeightLockPlugin()
	utxoQuery := testutil.NewMockUTXOQuery()

	// 创建高度锁 UTXO（解锁高度未到）
	unlockHeight := uint64(200)
	currentHeight := uint64(100)
	outpoint := testutil.CreateOutPoint(nil, 0)
	heightLock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_HeightLock{
			HeightLock: &transaction.HeightLock{
				UnlockHeight: unlockHeight,
				BaseLock:     testutil.CreateSingleKeyLock(nil),
			},
		},
	}
	output := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", heightLock)
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	// 创建交易
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_HeightProof{
					HeightProof: &transaction.HeightProof{
						BaseProof: testutil.CreateSingleKeyProof(nil, nil),
					},
				},
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "900", testutil.CreateSingleKeyLock(nil)),
		},
	)

	// 创建模拟环境
	env := &MockVerifierEnvironment{
		blockHeight: currentHeight,
		blockTime:   uint64(time.Now().Unix()),
		utxoQuery:   utxoQuery,
	}

	// 将环境注入到 context
	ctx := txiface.WithVerifierEnvironment(context.Background(), env)

	// 验证应该失败
	err := plugin.Check(ctx, tx, currentHeight, uint64(time.Now().Unix()))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "高度锁未解锁")
}

// TestHeightLockPlugin_Check_NoHeightProof 测试没有 HeightProof
func TestHeightLockPlugin_Check_NoHeightProof(t *testing.T) {
	plugin := NewHeightLockPlugin()

	// 创建交易（没有 HeightProof）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_SingleKeyProof{
					SingleKeyProof: &transaction.SingleKeyProof{},
				},
			},
		},
		nil,
	)

	// 验证应该通过（没有 HeightProof 的输入不需要高度锁验证）
	err := plugin.Check(context.Background(), tx, 100, uint64(time.Now().Unix()))

	assert.NoError(t, err)
}

// TestHeightLockPlugin_Check_NoVerifierEnvironment 测试没有 VerifierEnvironment
func TestHeightLockPlugin_Check_NoVerifierEnvironment(t *testing.T) {
	plugin := NewHeightLockPlugin()

	// 创建交易（包含 HeightProof）
	currentHeight := uint64(100)
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_HeightProof{
					HeightProof: &transaction.HeightProof{
						CurrentHeight: currentHeight,
						BaseProof:     testutil.CreateSingleKeyProof(nil, nil),
					},
				},
			},
		},
		nil,
	)

	// 不提供 VerifierEnvironment，使用简化验证
	err := plugin.Check(context.Background(), tx, currentHeight, uint64(time.Now().Unix()))
	assert.NoError(t, err)

	// 当前高度 < currentHeight，应该失败
	err = plugin.Check(context.Background(), tx, currentHeight-1, uint64(time.Now().Unix()))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "current_height")
}

// TestHeightLockPlugin_Check_GetUTXOError 测试获取 UTXO 失败
func TestHeightLockPlugin_Check_GetUTXOError(t *testing.T) {
	plugin := NewHeightLockPlugin()

	// 创建交易
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_HeightProof{
					HeightProof: &transaction.HeightProof{
						BaseProof: testutil.CreateSingleKeyProof(nil, nil),
					},
				},
			},
		},
		nil,
	)

	// 创建模拟环境（不提供 utxoQuery，导致 GetUTXO 失败）
	env := &MockVerifierEnvironment{
		blockHeight: 100,
		blockTime:   uint64(time.Now().Unix()),
		utxoQuery:   nil,
	}
	ctx := txiface.WithVerifierEnvironment(context.Background(), env)

	err := plugin.Check(ctx, tx, 100, uint64(time.Now().Unix()))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "查询UTXO失败")
}

// TestHeightLockPlugin_Check_NoOutputInUTXO 测试 UTXO 中没有 Output
func TestHeightLockPlugin_Check_NoOutputInUTXO(t *testing.T) {
	plugin := NewHeightLockPlugin()
	utxoQuery := testutil.NewMockUTXOQuery()

	// 创建没有 Output 的 UTXO
	outpoint := testutil.CreateOutPoint(nil, 0)
	utxo := &utxopb.UTXO{
		Outpoint: outpoint,
		// 不设置 CachedOutput
	}
	utxoQuery.AddUTXO(utxo)

	// 创建交易
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_HeightProof{
					HeightProof: &transaction.HeightProof{
						BaseProof: testutil.CreateSingleKeyProof(nil, nil),
					},
				},
			},
		},
		nil,
	)

	env := &MockVerifierEnvironment{
		blockHeight: 100,
		blockTime:   uint64(time.Now().Unix()),
		utxoQuery:   utxoQuery,
	}
	ctx := txiface.WithVerifierEnvironment(context.Background(), env)

	err := plugin.Check(ctx, tx, 100, uint64(time.Now().Unix()))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "UTXO未包含Output信息")
}

// TestHeightLockPlugin_Check_NoHeightLockInUTXO 测试 UTXO 中没有 HeightLock
func TestHeightLockPlugin_Check_NoHeightLockInUTXO(t *testing.T) {
	plugin := NewHeightLockPlugin()
	utxoQuery := testutil.NewMockUTXOQuery()

	// 创建没有 HeightLock 的 UTXO
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	// 创建交易（包含 HeightProof）
	currentHeight := uint64(100)
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_HeightProof{
					HeightProof: &transaction.HeightProof{
						CurrentHeight: currentHeight,
						BaseProof:     testutil.CreateSingleKeyProof(nil, nil),
					},
				},
			},
		},
		nil,
	)

	env := &MockVerifierEnvironment{
		blockHeight: currentHeight,
		blockTime:   uint64(time.Now().Unix()),
		utxoQuery:   utxoQuery,
	}
	ctx := txiface.WithVerifierEnvironment(context.Background(), env)

	// 应该使用 HeightProof 中的 current_height 进行验证
	err := plugin.Check(ctx, tx, currentHeight, uint64(time.Now().Unix()))
	assert.NoError(t, err)

	// 当前高度 < currentHeight，应该失败
	err = plugin.Check(ctx, tx, currentHeight-1, uint64(time.Now().Unix()))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "current_height")
}

// TestHeightLockPlugin_Check_ExactBoundary 测试边界值
func TestHeightLockPlugin_Check_ExactBoundary(t *testing.T) {
	plugin := NewHeightLockPlugin()
	utxoQuery := testutil.NewMockUTXOQuery()

	// 创建高度锁 UTXO（解锁高度正好等于当前高度）
	unlockHeight := uint64(100)
	currentHeight := uint64(100)
	outpoint := testutil.CreateOutPoint(nil, 0)
	heightLock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_HeightLock{
			HeightLock: &transaction.HeightLock{
				UnlockHeight: unlockHeight,
				BaseLock:     testutil.CreateSingleKeyLock(nil),
			},
		},
	}
	output := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", heightLock)
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_HeightProof{
					HeightProof: &transaction.HeightProof{
						BaseProof: testutil.CreateSingleKeyProof(nil, nil),
					},
				},
			},
		},
		nil,
	)

	env := &MockVerifierEnvironment{
		blockHeight: currentHeight,
		blockTime:   uint64(time.Now().Unix()),
		utxoQuery:   utxoQuery,
	}
	ctx := txiface.WithVerifierEnvironment(context.Background(), env)

	// 正好在边界上，应该通过
	err := plugin.Check(ctx, tx, currentHeight, uint64(time.Now().Unix()))
	assert.NoError(t, err)
}

