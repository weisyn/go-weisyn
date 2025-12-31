// Package condition_test 提供 TimeLockPlugin 的单元测试
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

// ==================== TimeLockPlugin 测试 ====================

// TestNewTimeLockPlugin 测试创建 TimeLockPlugin
func TestNewTimeLockPlugin(t *testing.T) {
	plugin := NewTimeLockPlugin()

	assert.NotNil(t, plugin)
}

// TestTimeLockPlugin_Name 测试插件名称
func TestTimeLockPlugin_Name(t *testing.T) {
	plugin := NewTimeLockPlugin()

	assert.Equal(t, "TimeLock", plugin.Name())
}

// TestTimeLockPlugin_Check_Success 测试时间锁验证成功
func TestTimeLockPlugin_Check_Success(t *testing.T) {
	plugin := NewTimeLockPlugin()
	utxoQuery := testutil.NewMockUTXOQuery()

	// 创建时间锁 UTXO（解锁时间已过）
	unlockTime := uint64(time.Now().Unix() - 3600) // 1小时前
	outpoint := testutil.CreateOutPoint(nil, 0)
	timeLock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_TimeLock{
			TimeLock: &transaction.TimeLock{
				UnlockTimestamp: unlockTime,
				BaseLock:        testutil.CreateSingleKeyLock(nil),
				TimeSource:      transaction.TimeLock_TIME_SOURCE_BLOCK_TIMESTAMP,
			},
		},
	}
	output := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", timeLock)
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	// 创建交易（包含 TimeProof）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_TimeProof{
					TimeProof: &transaction.TimeProof{
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
		blockHeight: 100,
		blockTime:   uint64(time.Now().Unix()),
		utxoQuery:   utxoQuery,
	}

	// 将环境注入到 context
	ctx := txiface.WithVerifierEnvironment(context.Background(), env)

	// 验证应该成功
	err := plugin.Check(ctx, tx, env.GetBlockHeight(), env.GetBlockTime())

	assert.NoError(t, err)
}

// TestTimeLockPlugin_Check_NotUnlocked 测试时间锁未解锁
func TestTimeLockPlugin_Check_NotUnlocked(t *testing.T) {
	plugin := NewTimeLockPlugin()
	utxoQuery := testutil.NewMockUTXOQuery()

	// 创建时间锁 UTXO（解锁时间未到）
	unlockTime := uint64(time.Now().Unix() + 3600) // 1小时后
	outpoint := testutil.CreateOutPoint(nil, 0)
	timeLock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_TimeLock{
			TimeLock: &transaction.TimeLock{
				UnlockTimestamp: unlockTime,
				BaseLock:        testutil.CreateSingleKeyLock(nil),
				TimeSource:      transaction.TimeLock_TIME_SOURCE_BLOCK_TIMESTAMP,
			},
		},
	}
	output := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", timeLock)
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	// 创建交易
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_TimeProof{
					TimeProof: &transaction.TimeProof{
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
		blockHeight: 100,
		blockTime:   uint64(time.Now().Unix()),
		utxoQuery:   utxoQuery,
	}

	// 将环境注入到 context
	ctx := txiface.WithVerifierEnvironment(context.Background(), env)

	// 验证应该失败
	err := plugin.Check(ctx, tx, env.GetBlockHeight(), env.GetBlockTime())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "时间锁未解锁")
}

// TestTimeLockPlugin_Check_NoTimeProof 测试没有 TimeProof
func TestTimeLockPlugin_Check_NoTimeProof(t *testing.T) {
	plugin := NewTimeLockPlugin()

	// 创建交易（没有 TimeProof）
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
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "900", testutil.CreateSingleKeyLock(nil)),
		},
	)

	// 验证应该通过（没有 TimeProof 的输入不需要时间锁验证）
	err := plugin.Check(context.Background(), tx, 100, uint64(time.Now().Unix()))

	assert.NoError(t, err)
}

// TestTimeLockPlugin_Check_NoVerifierEnvironment 测试没有 VerifierEnvironment
func TestTimeLockPlugin_Check_NoVerifierEnvironment(t *testing.T) {
	plugin := NewTimeLockPlugin()

	// 创建交易（包含 TimeProof）
	currentTimestamp := uint64(time.Now().Unix())
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_TimeProof{
					TimeProof: &transaction.TimeProof{
						CurrentTimestamp: currentTimestamp,
						BaseProof:        testutil.CreateSingleKeyProof(nil, nil),
					},
				},
			},
		},
		nil,
	)

	// 不提供 VerifierEnvironment，使用简化验证
	err := plugin.Check(context.Background(), tx, 100, currentTimestamp)
	assert.NoError(t, err)

	// 当前时间 < currentTimestamp，应该失败
	err = plugin.Check(context.Background(), tx, 100, currentTimestamp-1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "current_timestamp")
}

// TestTimeLockPlugin_Check_GetUTXOError 测试获取 UTXO 失败
func TestTimeLockPlugin_Check_GetUTXOError(t *testing.T) {
	plugin := NewTimeLockPlugin()

	// 创建交易
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_TimeProof{
					TimeProof: &transaction.TimeProof{
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

// TestTimeLockPlugin_Check_NoOutputInUTXO 测试 UTXO 中没有 Output
func TestTimeLockPlugin_Check_NoOutputInUTXO(t *testing.T) {
	plugin := NewTimeLockPlugin()
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
				UnlockingProof: &transaction.TxInput_TimeProof{
					TimeProof: &transaction.TimeProof{
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

// TestTimeLockPlugin_Check_NoTimeLockInUTXO 测试 UTXO 中没有 TimeLock
func TestTimeLockPlugin_Check_NoTimeLockInUTXO(t *testing.T) {
	plugin := NewTimeLockPlugin()
	utxoQuery := testutil.NewMockUTXOQuery()

	// 创建没有 TimeLock 的 UTXO
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	// 创建交易（包含 TimeProof）
	currentTimestamp := uint64(time.Now().Unix())
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_TimeProof{
					TimeProof: &transaction.TimeProof{
						CurrentTimestamp: currentTimestamp,
						BaseProof:        testutil.CreateSingleKeyProof(nil, nil),
					},
				},
			},
		},
		nil,
	)

	env := &MockVerifierEnvironment{
		blockHeight: 100,
		blockTime:   currentTimestamp,
		utxoQuery:   utxoQuery,
	}
	ctx := txiface.WithVerifierEnvironment(context.Background(), env)

	// 应该使用 TimeProof 中的 current_timestamp 进行验证
	err := plugin.Check(ctx, tx, 100, currentTimestamp)
	assert.NoError(t, err)

	// 当前时间 < currentTimestamp，应该失败
	err = plugin.Check(ctx, tx, 100, currentTimestamp-1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "current_timestamp")
}

// TestTimeLockPlugin_Check_DifferentTimeSources 测试不同的时间来源
func TestTimeLockPlugin_Check_DifferentTimeSources(t *testing.T) {
	plugin := NewTimeLockPlugin()
	utxoQuery := testutil.NewMockUTXOQuery()

	unlockTime := uint64(time.Now().Unix() - 3600)
	outpoint := testutil.CreateOutPoint(nil, 0)

	// 测试 BLOCK_TIMESTAMP
	timeLock1 := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_TimeLock{
			TimeLock: &transaction.TimeLock{
				UnlockTimestamp: unlockTime,
				BaseLock:        testutil.CreateSingleKeyLock(nil),
				TimeSource:      transaction.TimeLock_TIME_SOURCE_BLOCK_TIMESTAMP,
			},
		},
	}
	output1 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", timeLock1)
	utxo1 := testutil.CreateUTXO(outpoint, output1, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo1)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_TimeProof{
					TimeProof: &transaction.TimeProof{
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
	assert.NoError(t, err)

	// 测试 MEDIAN_TIME（当前实现使用 blockTime 作为近似值）
	utxoQuery = testutil.NewMockUTXOQuery()
	timeLock2 := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_TimeLock{
			TimeLock: &transaction.TimeLock{
				UnlockTimestamp: unlockTime,
				BaseLock:        testutil.CreateSingleKeyLock(nil),
				TimeSource:      transaction.TimeLock_TIME_SOURCE_MEDIAN_TIME,
			},
		},
	}
	output2 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", timeLock2)
	utxo2 := testutil.CreateUTXO(outpoint, output2, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo2)

	env.utxoQuery = utxoQuery
	err = plugin.Check(ctx, tx, 100, uint64(time.Now().Unix()))
	assert.NoError(t, err)

	// 测试 ORACLE（当前实现使用 blockTime 作为近似值）
	utxoQuery = testutil.NewMockUTXOQuery()
	timeLock3 := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_TimeLock{
			TimeLock: &transaction.TimeLock{
				UnlockTimestamp: unlockTime,
				BaseLock:        testutil.CreateSingleKeyLock(nil),
				TimeSource:      transaction.TimeLock_TIME_SOURCE_ORACLE,
			},
		},
	}
	output3 := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", timeLock3)
	utxo3 := testutil.CreateUTXO(outpoint, output3, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo3)

	env.utxoQuery = utxoQuery
	err = plugin.Check(ctx, tx, 100, uint64(time.Now().Unix()))
	assert.NoError(t, err)
}

