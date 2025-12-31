// Package authz_test 提供 DelegationPlugin 的单元测试
//
// 🧪 **测试规范遵循**：
// - 每个源文件对应一个测试文件（delegation.go → delegation_test.go）
// - 遵循测试规范：docs/system/standards/principles/testing-standards.md
package authz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/core/tx/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	txiface "github.com/weisyn/v1/pkg/interfaces/tx"
)

// TestNewDelegationPlugin 测试创建 DelegationPlugin
func TestNewDelegationPlugin(t *testing.T) {
	plugin := NewDelegationPlugin()

	assert.NotNil(t, plugin)
}

// TestDelegationPlugin_Name 测试插件名称
func TestDelegationPlugin_Name(t *testing.T) {
	plugin := NewDelegationPlugin()

	assert.Equal(t, "delegation", plugin.Name())
}

// TestDelegationPlugin_Match_NotDelegationLock 测试不匹配其他锁类型
func TestDelegationPlugin_Match_NotDelegationLock(t *testing.T) {
	plugin := NewDelegationPlugin()

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_SingleKeyLock{
			SingleKeyLock: &transaction.SingleKeyLock{},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_DelegationProof{
			DelegationProof: &transaction.DelegationProof{},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.NoError(t, err)
	assert.False(t, matched)
}

// TestDelegationPlugin_Match_MissingProof 测试缺少 proof
func TestDelegationPlugin_Match_MissingProof(t *testing.T) {
	plugin := NewDelegationPlugin()

	expiryBlocks := uint64(1000)
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_DelegationLock{
			DelegationLock: &transaction.DelegationLock{
				ExpiryDurationBlocks: &expiryBlocks,
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: nil,
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "missing delegation proof")
}

// TestDelegationPlugin_Match_EmptyDelegationTransactionId 测试空委托交易ID
func TestDelegationPlugin_Match_EmptyDelegationTransactionId(t *testing.T) {
	plugin := NewDelegationPlugin()

	expiryBlocks := uint64(1000)
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_DelegationLock{
			DelegationLock: &transaction.DelegationLock{
				ExpiryDurationBlocks: &expiryBlocks,
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_DelegationProof{
			DelegationProof: &transaction.DelegationProof{
				DelegationTransactionId: nil, // 空交易ID
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "invalid delegation_transaction_id")
}

// TestDelegationPlugin_Match_OperationTypeNotAuthorized 测试操作类型未授权
func TestDelegationPlugin_Match_OperationTypeNotAuthorized(t *testing.T) {
	plugin := NewDelegationPlugin()

	ctx := txiface.WithVerifierEnvironment(context.Background(), &MockVerifierEnvironment{blockHeight: 100})

	// 不设置过期时间，避免过期检查先于操作类型检查
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_DelegationLock{
			DelegationLock: &transaction.DelegationLock{
				ExpiryDurationBlocks: nil, // 无过期限制
				AuthorizedOperations: []string{"transfer", "approve"},
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_DelegationProof{
			DelegationProof: &transaction.DelegationProof{
				DelegationTransactionId: testutil.RandomTxID(),
				OperationType:           "burn", // 未授权的操作类型
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(ctx, lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "operation type not authorized")
}

// TestDelegationPlugin_Match_ValueAmountExceedsLimit 测试价值金额超过限制
func TestDelegationPlugin_Match_ValueAmountExceedsLimit(t *testing.T) {
	plugin := NewDelegationPlugin()

	ctx := txiface.WithVerifierEnvironment(context.Background(), &MockVerifierEnvironment{blockHeight: 100})

	// 不设置过期时间，避免过期检查先于价值检查
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_DelegationLock{
			DelegationLock: &transaction.DelegationLock{
				ExpiryDurationBlocks: nil, // 无过期限制
				MaxValuePerOperation: 1000,
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_DelegationProof{
			DelegationProof: &transaction.DelegationProof{
				DelegationTransactionId: testutil.RandomTxID(),
				ValueAmount:            2000, // 超过限制
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(ctx, lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "operation value exceeds max limit")
}

// TestDelegationPlugin_Match_DelegateNotAllowed 测试被委托方不在允许列表中
func TestDelegationPlugin_Match_DelegateNotAllowed(t *testing.T) {
	plugin := NewDelegationPlugin()

	ctx := txiface.WithVerifierEnvironment(context.Background(), &MockVerifierEnvironment{blockHeight: 100})

	// 不设置过期时间，避免过期检查先于委托方检查
	allowedDelegate := testutil.RandomAddress()
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_DelegationLock{
			DelegationLock: &transaction.DelegationLock{
				ExpiryDurationBlocks: nil, // 无过期限制
				AllowedDelegates:     [][]byte{allowedDelegate},
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_DelegationProof{
			DelegationProof: &transaction.DelegationProof{
				DelegationTransactionId: testutil.RandomTxID(),
				DelegateAddress:         testutil.RandomAddress(), // 不同的地址
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(ctx, lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "delegate not allowed")
}

// TestDelegationPlugin_Match_Success 测试成功匹配
func TestDelegationPlugin_Match_Success(t *testing.T) {
	plugin := NewDelegationPlugin()

	ctx := txiface.WithVerifierEnvironment(context.Background(), &MockVerifierEnvironment{blockHeight: 100})

	// 不设置过期时间，避免过期检查
	allowedDelegate := testutil.RandomAddress()
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_DelegationLock{
			DelegationLock: &transaction.DelegationLock{
				ExpiryDurationBlocks: nil, // 无过期限制
				AuthorizedOperations: []string{"transfer"},
				MaxValuePerOperation: 1000,
				AllowedDelegates:     [][]byte{allowedDelegate},
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_DelegationProof{
			DelegationProof: &transaction.DelegationProof{
				DelegationTransactionId: testutil.RandomTxID(),
				OperationType:           "transfer",
				ValueAmount:             500,
				DelegateAddress:          allowedDelegate, // 使用允许的地址
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(ctx, lock, proof, tx)

	assert.NoError(t, err)
	assert.True(t, matched)
}

// TestDelegationPlugin_Match_Success_NoRestrictions 测试成功匹配（无限制）
func TestDelegationPlugin_Match_Success_NoRestrictions(t *testing.T) {
	plugin := NewDelegationPlugin()

	ctx := txiface.WithVerifierEnvironment(context.Background(), &MockVerifierEnvironment{blockHeight: 100})

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_DelegationLock{
			DelegationLock: &transaction.DelegationLock{
				ExpiryDurationBlocks: nil, // 无过期限制
				AuthorizedOperations: nil, // 无操作限制
				MaxValuePerOperation: 0,   // 0 表示无限制（需要检查实现）
				AllowedDelegates:     nil, // 无委托方限制
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_DelegationProof{
			DelegationProof: &transaction.DelegationProof{
				DelegationTransactionId: testutil.RandomTxID(),
				OperationType:           "any_operation",
				ValueAmount:             0, // 使用0避免价值检查
				DelegateAddress:         testutil.RandomAddress(),
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(ctx, lock, proof, tx)

	assert.NoError(t, err)
	assert.True(t, matched)
}

// TestBytesEqual 测试 bytesEqual 辅助函数
func TestBytesEqual(t *testing.T) {
	a := []byte{1, 2, 3}
	b := []byte{1, 2, 3}
	c := []byte{1, 2, 4}
	d := []byte{1, 2}

	assert.True(t, bytesEqual(a, b))
	assert.False(t, bytesEqual(a, c))
	assert.False(t, bytesEqual(a, d))
	assert.False(t, bytesEqual(d, a))
	assert.True(t, bytesEqual(nil, nil))
	assert.False(t, bytesEqual(nil, a))
	assert.False(t, bytesEqual(a, nil))
}

