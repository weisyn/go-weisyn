// Package authz_test 提供 DelegationLockPlugin 的单元测试
//
// 🧪 **测试规范遵循**：
// - 每个源文件对应一个测试文件
// - 遵循测试规范：docs/system/standards/principles/testing-standards.md
package authz

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/core/tx/ports/hash"
	"github.com/weisyn/v1/internal/core/tx/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	txiface "github.com/weisyn/v1/pkg/interfaces/tx"
)

// ==================== DelegationLockPlugin 测试 ====================

// TestNewDelegationLockPlugin 测试创建 DelegationLockPlugin
func TestNewDelegationLockPlugin(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewDelegationLockPlugin(mockSigMgr, mockCanonicalizer)

	assert.NotNil(t, plugin)
	assert.Equal(t, "DelegationLock", plugin.Name())
}

// TestDelegationLockPlugin_Match_NotDelegationLock 测试不匹配其他锁类型
func TestDelegationLockPlugin_Match_NotDelegationLock(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockCanonicalizer := NewMockCanonicalizer()
	plugin := NewDelegationLockPlugin(mockSigMgr, mockCanonicalizer)

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

// TestDelegationLockPlugin_Match_MissingProof 测试缺少 proof
func TestDelegationLockPlugin_Match_MissingProof(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockCanonicalizer := NewMockCanonicalizer()
	plugin := NewDelegationLockPlugin(mockSigMgr, mockCanonicalizer)

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_DelegationLock{
			DelegationLock: &transaction.DelegationLock{},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: nil,
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "DelegationProof")
}

// TestDelegationLockPlugin_Match_VerifierEnvironmentNotProvided 测试未提供 VerifierEnvironment
func TestDelegationLockPlugin_Match_VerifierEnvironmentNotProvided(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockCanonicalizer := NewMockCanonicalizer()
	plugin := NewDelegationLockPlugin(mockSigMgr, mockCanonicalizer)

	expiryBlocks := uint64(1000)
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_DelegationLock{
			DelegationLock: &transaction.DelegationLock{
				ExpiryDurationBlocks: &expiryBlocks,
				AuthorizedOperations:  []string{"transfer"},
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_DelegationProof{
			DelegationProof: &transaction.DelegationProof{
				DelegationTransactionId: testutil.RandomTxID(),
				OperationType:           "transfer",
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "VerifierEnvironment")
}

// TestDelegationLockPlugin_Match_GetTxBlockHeightError 测试 GetTxBlockHeight 错误
func TestDelegationLockPlugin_Match_GetTxBlockHeightError(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockCanonicalizer := NewMockCanonicalizer()
	plugin := NewDelegationLockPlugin(mockSigMgr, mockCanonicalizer)

	expiryBlocks := uint64(1000)
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_DelegationLock{
			DelegationLock: &transaction.DelegationLock{
				ExpiryDurationBlocks: &expiryBlocks,
				AuthorizedOperations:  []string{"transfer"},
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_DelegationProof{
			DelegationProof: &transaction.DelegationProof{
				DelegationTransactionId: testutil.RandomTxID(),
				OperationType:           "transfer",
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	mockEnv := &MockVerifierEnvironment{
		blockHeight:      1000,
		txBlockHeightErr: fmt.Errorf("查询失败"),
	}
	ctx := txiface.WithVerifierEnvironment(context.Background(), mockEnv)

	matched, err := plugin.Match(ctx, lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "查询委托交易区块高度失败")
}

// TestDelegationLockPlugin_Match_ExpiredDelegation 测试委托已过期
func TestDelegationLockPlugin_Match_ExpiredDelegation(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockCanonicalizer := NewMockCanonicalizer()
	plugin := NewDelegationLockPlugin(mockSigMgr, mockCanonicalizer)

	expiryBlocks := uint64(1000)
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_DelegationLock{
			DelegationLock: &transaction.DelegationLock{
				ExpiryDurationBlocks: &expiryBlocks,
				AuthorizedOperations: []string{"transfer"},
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_DelegationProof{
			DelegationProof: &transaction.DelegationProof{
				DelegationTransactionId: testutil.RandomTxID(),
				OperationType:           "transfer",
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	// 委托高度5000，当前高度10000，过期高度6000，已过期
	mockEnv := &MockVerifierEnvironment{
		blockHeight:   10000,
		txBlockHeight: 5000,
	}
	ctx := txiface.WithVerifierEnvironment(context.Background(), mockEnv)

	matched, err := plugin.Match(ctx, lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "委托已过期")
}

// TestDelegationLockPlugin_Match_OperationTypeNotAuthorized 测试操作类型未授权
func TestDelegationLockPlugin_Match_OperationTypeNotAuthorized(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockCanonicalizer := NewMockCanonicalizer()
	plugin := NewDelegationLockPlugin(mockSigMgr, mockCanonicalizer)

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_DelegationLock{
			DelegationLock: &transaction.DelegationLock{
				ExpiryDurationBlocks: nil, // 无过期限制
				AuthorizedOperations:  []string{"transfer", "approve"},
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

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "不在授权范围内")
}

// TestDelegationLockPlugin_Match_DelegateNotInAllowedList 测试被委托方不在允许列表中
func TestDelegationLockPlugin_Match_DelegateNotInAllowedList(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockCanonicalizer := NewMockCanonicalizer()
	plugin := NewDelegationLockPlugin(mockSigMgr, mockCanonicalizer)

	allowedDelegate := testutil.RandomAddress()
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_DelegationLock{
			DelegationLock: &transaction.DelegationLock{
				ExpiryDurationBlocks: nil, // 无过期限制
				AuthorizedOperations: []string{"transfer"},
				AllowedDelegates:     [][]byte{allowedDelegate},
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_DelegationProof{
			DelegationProof: &transaction.DelegationProof{
				DelegationTransactionId: testutil.RandomTxID(),
				OperationType:           "transfer",
				DelegateAddress:         testutil.RandomAddress(), // 不同的地址
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "不在允许列表中")
}

// TestDelegationLockPlugin_Match_ValueAmountExceedsLimit 测试价值金额超过限制
func TestDelegationLockPlugin_Match_ValueAmountExceedsLimit(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockCanonicalizer := NewMockCanonicalizer()
	plugin := NewDelegationLockPlugin(mockSigMgr, mockCanonicalizer)

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_DelegationLock{
			DelegationLock: &transaction.DelegationLock{
				ExpiryDurationBlocks: nil, // 无过期限制
				AuthorizedOperations: []string{"transfer"},
				MaxValuePerOperation: 1000,
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_DelegationProof{
			DelegationProof: &transaction.DelegationProof{
				DelegationTransactionId: testutil.RandomTxID(),
				OperationType:           "transfer",
				ValueAmount:            2000, // 超过限制
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "超过限制")
}

// TestDelegationLockPlugin_Match_InputIndexNotFound 测试找不到输入索引
func TestDelegationLockPlugin_Match_InputIndexNotFound(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockCanonicalizer := NewMockCanonicalizer()
	plugin := NewDelegationLockPlugin(mockSigMgr, mockCanonicalizer)

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_DelegationLock{
			DelegationLock: &transaction.DelegationLock{
				ExpiryDurationBlocks: nil,
				AuthorizedOperations: []string{"transfer"},
			},
		},
	}
	delegationProof := &transaction.DelegationProof{
		DelegationTransactionId: testutil.RandomTxID(),
		OperationType:           "transfer",
		DelegateSignature: &transaction.SignatureData{
			Value: testutil.RandomBytes(64),
		},
	}
	// 创建一个不同的 proof 对象
	differentProof := &transaction.DelegationProof{
		DelegationTransactionId: testutil.RandomTxID(),
		OperationType:           "transfer",
	}
	input := &transaction.TxInput{
		PreviousOutput:  testutil.CreateOutPoint(nil, 0),
		IsReferenceOnly: false,
		UnlockingProof: &transaction.TxInput_DelegationProof{
			DelegationProof: differentProof, // 使用不同的 proof 对象
		},
	}
	tx := testutil.CreateTransaction([]*transaction.TxInput{input}, nil)

	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_DelegationProof{
			DelegationProof: delegationProof, // 使用不同的 proof 对象
		},
	}

	mockEnv := &MockVerifierEnvironment{
		blockHeight:   1000,
		txBlockHeight: 500,
		publicKey:     testutil.RandomPublicKey(),
	}
	ctx := txiface.WithVerifierEnvironment(context.Background(), mockEnv)

	matched, err := plugin.Match(ctx, lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "无法找到当前输入的索引")
}

// TestDelegationLockPlugin_Match_ComputeSignatureHashError 测试计算签名哈希错误
func TestDelegationLockPlugin_Match_ComputeSignatureHashError(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	errorClient := &ErrorMockTransactionHashServiceClient{
		computeSignatureHashError: fmt.Errorf("计算签名哈希失败"),
	}
	mockCanonicalizer := hash.NewCanonicalizer(errorClient)
	plugin := NewDelegationLockPlugin(mockSigMgr, mockCanonicalizer)

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_DelegationLock{
			DelegationLock: &transaction.DelegationLock{
				ExpiryDurationBlocks: nil,
				AuthorizedOperations: []string{"transfer"},
			},
		},
	}
	delegationProof := &transaction.DelegationProof{
		DelegationTransactionId: testutil.RandomTxID(),
		OperationType:           "transfer",
		DelegateSignature: &transaction.SignatureData{
			Value: testutil.RandomBytes(64),
		},
	}
	input := &transaction.TxInput{
		PreviousOutput:  testutil.CreateOutPoint(nil, 0),
		IsReferenceOnly: false,
		UnlockingProof: &transaction.TxInput_DelegationProof{
			DelegationProof: delegationProof,
		},
	}
	tx := testutil.CreateTransaction([]*transaction.TxInput{input}, nil)

	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_DelegationProof{
			DelegationProof: delegationProof,
		},
	}

	mockEnv := &MockVerifierEnvironment{
		blockHeight:   1000,
		txBlockHeight: 500,
		publicKey:     testutil.RandomPublicKey(),
	}
	ctx := txiface.WithVerifierEnvironment(context.Background(), mockEnv)

	matched, err := plugin.Match(ctx, lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "计算交易签名哈希失败")
}

// TestDelegationLockPlugin_Match_GetPublicKeyError 测试 GetPublicKey 错误
func TestDelegationLockPlugin_Match_GetPublicKeyError(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockCanonicalizer := NewMockCanonicalizer()
	plugin := NewDelegationLockPlugin(mockSigMgr, mockCanonicalizer)

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_DelegationLock{
			DelegationLock: &transaction.DelegationLock{
				ExpiryDurationBlocks: nil,
				AuthorizedOperations: []string{"transfer"},
			},
		},
	}
	delegationProof := &transaction.DelegationProof{
		DelegationTransactionId: testutil.RandomTxID(),
		OperationType:           "transfer",
		DelegateSignature: &transaction.SignatureData{
			Value: testutil.RandomBytes(64),
		},
	}
	input := &transaction.TxInput{
		PreviousOutput:  testutil.CreateOutPoint(nil, 0),
		IsReferenceOnly: false,
		UnlockingProof: &transaction.TxInput_DelegationProof{
			DelegationProof: delegationProof,
		},
	}
	tx := testutil.CreateTransaction([]*transaction.TxInput{input}, nil)

	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_DelegationProof{
			DelegationProof: delegationProof,
		},
	}

	mockEnv := &MockVerifierEnvironment{
		blockHeight:   1000,
		txBlockHeight: 500,
		publicKeyErr:  fmt.Errorf("获取公钥失败"),
	}
	ctx := txiface.WithVerifierEnvironment(context.Background(), mockEnv)

	matched, err := plugin.Match(ctx, lock, proof, tx)

	// 注意：根据实现，GetPublicKey 错误不会阻止验证通过（向后兼容）
	// 所以这里应该成功
	assert.NoError(t, err)
	assert.True(t, matched)
}

// TestDelegationLockPlugin_Match_DelegateSignatureVerificationFailure 测试被委托方签名验证失败
func TestDelegationLockPlugin_Match_DelegateSignatureVerificationFailure(t *testing.T) {
	mockSigMgr := &FailingMockSignatureManager{} // 使用失败的签名管理器
	mockCanonicalizer := NewMockCanonicalizer()
	plugin := NewDelegationLockPlugin(mockSigMgr, mockCanonicalizer)

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_DelegationLock{
			DelegationLock: &transaction.DelegationLock{
				ExpiryDurationBlocks: nil,
				AuthorizedOperations: []string{"transfer"},
			},
		},
	}
	delegationProof := &transaction.DelegationProof{
		DelegationTransactionId: testutil.RandomTxID(),
		OperationType:           "transfer",
		DelegateSignature: &transaction.SignatureData{
			Value: testutil.RandomBytes(64),
		},
	}
	input := &transaction.TxInput{
		PreviousOutput:  testutil.CreateOutPoint(nil, 0),
		IsReferenceOnly: false,
		UnlockingProof: &transaction.TxInput_DelegationProof{
			DelegationProof: delegationProof,
		},
	}
	tx := testutil.CreateTransaction([]*transaction.TxInput{input}, nil)

	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_DelegationProof{
			DelegationProof: delegationProof,
		},
	}

	mockEnv := &MockVerifierEnvironment{
		blockHeight:   1000,
		txBlockHeight: 500,
		publicKey:     testutil.RandomPublicKey(),
	}
	ctx := txiface.WithVerifierEnvironment(context.Background(), mockEnv)

	matched, err := plugin.Match(ctx, lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "签名验证失败")
}

// TestDelegationLockPlugin_Match_NoSignatureProvided 测试未提供签名
func TestDelegationLockPlugin_Match_NoSignatureProvided(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockCanonicalizer := NewMockCanonicalizer()
	plugin := NewDelegationLockPlugin(mockSigMgr, mockCanonicalizer)

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_DelegationLock{
			DelegationLock: &transaction.DelegationLock{
				ExpiryDurationBlocks: nil,
				AuthorizedOperations: []string{"transfer"},
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_DelegationProof{
			DelegationProof: &transaction.DelegationProof{
				DelegationTransactionId: testutil.RandomTxID(),
				OperationType:           "transfer",
				DelegateSignature:       nil, // 未提供签名
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	// 未提供签名时，应该跳过签名验证，验证通过
	assert.NoError(t, err)
	assert.True(t, matched)
}

// TestDelegationLockPlugin_Match_Success 测试成功匹配
func TestDelegationLockPlugin_Match_Success(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockCanonicalizer := NewMockCanonicalizer()
	plugin := NewDelegationLockPlugin(mockSigMgr, mockCanonicalizer)

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
				DelegateAddress:          allowedDelegate,
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.NoError(t, err)
	assert.True(t, matched)
}

