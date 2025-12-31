// Package authz_test 提供 ThresholdLockPlugin 的单元测试
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
)

// ==================== ThresholdLockPlugin 测试 ====================

// TestNewThresholdLockPlugin 测试创建 ThresholdLockPlugin
func TestNewThresholdLockPlugin(t *testing.T) {
	mockVerifier := &MockThresholdSignatureVerifier{}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewThresholdLockPlugin(mockVerifier, mockCanonicalizer)

	assert.NotNil(t, plugin)
	assert.Equal(t, "ThresholdLock", plugin.Name())
}

// TestThresholdLockPlugin_Match_NotThresholdLock 测试不匹配其他锁类型
func TestThresholdLockPlugin_Match_NotThresholdLock(t *testing.T) {
	mockVerifier := &MockThresholdSignatureVerifier{}
	mockCanonicalizer := NewMockCanonicalizer()
	plugin := NewThresholdLockPlugin(mockVerifier, mockCanonicalizer)

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_SingleKeyLock{
			SingleKeyLock: &transaction.SingleKeyLock{},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ThresholdProof{
			ThresholdProof: &transaction.ThresholdProof{},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.NoError(t, err)
	assert.False(t, matched)
}

// TestThresholdLockPlugin_Match_MissingProof 测试缺少 proof
func TestThresholdLockPlugin_Match_MissingProof(t *testing.T) {
	mockVerifier := &MockThresholdSignatureVerifier{}
	mockCanonicalizer := NewMockCanonicalizer()
	plugin := NewThresholdLockPlugin(mockVerifier, mockCanonicalizer)

	groupKey := testutil.RandomPublicKey()
	partyKeys := [][]byte{groupKey, testutil.RandomPublicKey(), testutil.RandomPublicKey(), testutil.RandomPublicKey()}
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ThresholdLock{
			ThresholdLock: &transaction.ThresholdLock{
				Threshold:             2,
				TotalParties:          3,
				PartyVerificationKeys: partyKeys,
				SignatureScheme:       "BLS_THRESHOLD",
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
	assert.Contains(t, err.Error(), "ThresholdProof")
}

// TestThresholdLockPlugin_Match_InsufficientShares 测试份额不足
func TestThresholdLockPlugin_Match_InsufficientShares(t *testing.T) {
	mockVerifier := &MockThresholdSignatureVerifier{}
	mockCanonicalizer := NewMockCanonicalizer()
	plugin := NewThresholdLockPlugin(mockVerifier, mockCanonicalizer)

	groupKey := testutil.RandomPublicKey()
	partyKeys := [][]byte{groupKey, testutil.RandomPublicKey(), testutil.RandomPublicKey(), testutil.RandomPublicKey()}
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ThresholdLock{
			ThresholdLock: &transaction.ThresholdLock{
				Threshold:             3,
				TotalParties:          3,
				PartyVerificationKeys: partyKeys,
				SignatureScheme:       "BLS_THRESHOLD",
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ThresholdProof{
			ThresholdProof: &transaction.ThresholdProof{
				Shares: []*transaction.ThresholdProof_ThresholdSignatureShare{
					{PartyId: 1, SignatureShare: testutil.RandomBytes(96), VerificationKey: partyKeys[1]},
					{PartyId: 2, SignatureShare: testutil.RandomBytes(96), VerificationKey: partyKeys[2]},
					// 只有2个份额，需要3个
				},
				SignatureScheme: "BLS_THRESHOLD",
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "签名份额数量")
}

// TestThresholdLockPlugin_Match_DuplicatePartyID 测试重复的 party_id
func TestThresholdLockPlugin_Match_DuplicatePartyID(t *testing.T) {
	mockVerifier := &MockThresholdSignatureVerifier{}
	mockCanonicalizer := NewMockCanonicalizer()
	plugin := NewThresholdLockPlugin(mockVerifier, mockCanonicalizer)

	groupKey := testutil.RandomPublicKey()
	partyKeys := [][]byte{groupKey, testutil.RandomPublicKey(), testutil.RandomPublicKey(), testutil.RandomPublicKey()}
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ThresholdLock{
			ThresholdLock: &transaction.ThresholdLock{
				Threshold:             2,
				TotalParties:          3,
				PartyVerificationKeys: partyKeys,
				SignatureScheme:       "BLS_THRESHOLD",
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ThresholdProof{
			ThresholdProof: &transaction.ThresholdProof{
				Shares: []*transaction.ThresholdProof_ThresholdSignatureShare{
					{PartyId: 1, SignatureShare: testutil.RandomBytes(96), VerificationKey: partyKeys[1]},
					{PartyId: 1, SignatureShare: testutil.RandomBytes(96), VerificationKey: partyKeys[1]}, // 重复的 party_id
				},
				SignatureScheme: "BLS_THRESHOLD",
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "参与方")
}

// TestThresholdLockPlugin_Match_PartyIDOutOfRange 测试 party_id 超出范围
func TestThresholdLockPlugin_Match_PartyIDOutOfRange(t *testing.T) {
	mockVerifier := &MockThresholdSignatureVerifier{}
	mockCanonicalizer := NewMockCanonicalizer()
	plugin := NewThresholdLockPlugin(mockVerifier, mockCanonicalizer)

	groupKey := testutil.RandomPublicKey()
	partyKeys := [][]byte{groupKey, testutil.RandomPublicKey(), testutil.RandomPublicKey(), testutil.RandomPublicKey()}
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ThresholdLock{
			ThresholdLock: &transaction.ThresholdLock{
				Threshold:             2,
				TotalParties:          3,
				PartyVerificationKeys: partyKeys,
				SignatureScheme:       "BLS_THRESHOLD",
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ThresholdProof{
			ThresholdProof: &transaction.ThresholdProof{
				Shares: []*transaction.ThresholdProof_ThresholdSignatureShare{
					{PartyId: 1, SignatureShare: testutil.RandomBytes(96), VerificationKey: partyKeys[1]},
					{PartyId: 4, SignatureShare: testutil.RandomBytes(96), VerificationKey: partyKeys[1]}, // 超出范围（期望 1..3）
				},
				SignatureScheme: "BLS_THRESHOLD",
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	// 错误信息应明确指出 party_id 的合法区间（1..TotalParties）
	assert.Contains(t, err.Error(), "期望 1..3")
}

// TestThresholdLockPlugin_Match_VerificationKeyMismatch 测试验证密钥不匹配
func TestThresholdLockPlugin_Match_VerificationKeyMismatch(t *testing.T) {
	mockVerifier := &MockThresholdSignatureVerifier{}
	mockCanonicalizer := NewMockCanonicalizer()
	plugin := NewThresholdLockPlugin(mockVerifier, mockCanonicalizer)

	groupKey := testutil.RandomPublicKey()
	partyKeys := [][]byte{groupKey, testutil.RandomPublicKey(), testutil.RandomPublicKey(), testutil.RandomPublicKey()}
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ThresholdLock{
			ThresholdLock: &transaction.ThresholdLock{
				Threshold:             2,
				TotalParties:          3,
				PartyVerificationKeys: partyKeys,
				SignatureScheme:       "BLS_THRESHOLD",
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ThresholdProof{
			ThresholdProof: &transaction.ThresholdProof{
				Shares: []*transaction.ThresholdProof_ThresholdSignatureShare{
					{PartyId: 1, SignatureShare: testutil.RandomBytes(96), VerificationKey: partyKeys[1]},
					{PartyId: 2, SignatureShare: testutil.RandomBytes(96), VerificationKey: testutil.RandomPublicKey()}, // 错误的密钥
				},
				SignatureScheme: "BLS_THRESHOLD",
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "验证密钥不匹配")
}

// TestThresholdLockPlugin_Match_ComputeSignatureHashError 测试计算签名哈希错误
func TestThresholdLockPlugin_Match_ComputeSignatureHashError(t *testing.T) {
	mockVerifier := &MockThresholdSignatureVerifier{}
	errorClient := &ErrorMockTransactionHashServiceClient{
		computeSignatureHashError: fmt.Errorf("计算签名哈希失败"),
	}
	mockCanonicalizer := hash.NewCanonicalizer(errorClient)
	plugin := NewThresholdLockPlugin(mockVerifier, mockCanonicalizer)

	groupKey := testutil.RandomPublicKey()
	partyKeys := [][]byte{groupKey, testutil.RandomPublicKey(), testutil.RandomPublicKey(), testutil.RandomPublicKey()}
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ThresholdLock{
			ThresholdLock: &transaction.ThresholdLock{
				Threshold:             2,
				TotalParties:          3,
				PartyVerificationKeys: partyKeys,
				SignatureScheme:       "BLS_THRESHOLD",
			},
		},
	}
	thresholdProof := &transaction.ThresholdProof{
		Shares: []*transaction.ThresholdProof_ThresholdSignatureShare{
			{PartyId: 1, SignatureShare: testutil.RandomBytes(96), VerificationKey: partyKeys[1]},
			{PartyId: 2, SignatureShare: testutil.RandomBytes(96), VerificationKey: partyKeys[2]},
		},
		CombinedSignature: testutil.RandomBytes(96),
		SignatureScheme:   "BLS_THRESHOLD",
	}
	input := &transaction.TxInput{
		PreviousOutput:  testutil.CreateOutPoint(nil, 0),
		IsReferenceOnly: false,
		UnlockingProof: &transaction.TxInput_ThresholdProof{
			ThresholdProof: thresholdProof,
		},
	}
	tx := testutil.CreateTransaction([]*transaction.TxInput{input}, nil)

	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ThresholdProof{
			ThresholdProof: thresholdProof,
		},
	}

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "计算交易签名哈希失败")
}

// TestThresholdLockPlugin_Match_ThresholdSignatureVerificationFailure 测试门限签名验证失败
func TestThresholdLockPlugin_Match_ThresholdSignatureVerificationFailure(t *testing.T) {
	mockVerifier := &MockThresholdSignatureVerifier{shouldFail: true} // 使用失败的验证器
	mockCanonicalizer := NewMockCanonicalizer()
	plugin := NewThresholdLockPlugin(mockVerifier, mockCanonicalizer)

	groupKey := testutil.RandomPublicKey()
	partyKeys := [][]byte{groupKey, testutil.RandomPublicKey(), testutil.RandomPublicKey(), testutil.RandomPublicKey()}
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ThresholdLock{
			ThresholdLock: &transaction.ThresholdLock{
				Threshold:             2,
				TotalParties:          3,
				PartyVerificationKeys: partyKeys,
				SignatureScheme:       "BLS_THRESHOLD",
			},
		},
	}
	thresholdProof := &transaction.ThresholdProof{
		Shares: []*transaction.ThresholdProof_ThresholdSignatureShare{
			{PartyId: 1, SignatureShare: testutil.RandomBytes(96), VerificationKey: partyKeys[1]},
			{PartyId: 2, SignatureShare: testutil.RandomBytes(96), VerificationKey: partyKeys[2]},
		},
		CombinedSignature: testutil.RandomBytes(96),
		SignatureScheme:   "BLS_THRESHOLD",
	}
	input := &transaction.TxInput{
		PreviousOutput:  testutil.CreateOutPoint(nil, 0),
		IsReferenceOnly: false,
		UnlockingProof: &transaction.TxInput_ThresholdProof{
			ThresholdProof: thresholdProof,
		},
	}
	tx := testutil.CreateTransaction([]*transaction.TxInput{input}, nil)

	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ThresholdProof{
			ThresholdProof: thresholdProof,
		},
	}

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "门限签名验证失败")
}

// TestThresholdLockPlugin_Match_Success 测试成功匹配
func TestThresholdLockPlugin_Match_Success(t *testing.T) {
	mockVerifier := &MockThresholdSignatureVerifier{}
	mockCanonicalizer := NewMockCanonicalizer()
	plugin := NewThresholdLockPlugin(mockVerifier, mockCanonicalizer)

	groupKey := testutil.RandomPublicKey()
	partyKeys := [][]byte{
		groupKey, // [0] group public key
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
	}
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ThresholdLock{
			ThresholdLock: &transaction.ThresholdLock{
				Threshold:             3,
				TotalParties:          5,
				PartyVerificationKeys: partyKeys,
				SignatureScheme:       "BLS_THRESHOLD",
			},
		},
	}
	thresholdProof := &transaction.ThresholdProof{
		Shares: []*transaction.ThresholdProof_ThresholdSignatureShare{
			{PartyId: 1, SignatureShare: testutil.RandomBytes(96), VerificationKey: partyKeys[1]},
			{PartyId: 2, SignatureShare: testutil.RandomBytes(96), VerificationKey: partyKeys[2]},
			{PartyId: 3, SignatureShare: testutil.RandomBytes(96), VerificationKey: partyKeys[3]},
		},
		CombinedSignature: testutil.RandomBytes(96),
		SignatureScheme:   "BLS_THRESHOLD",
	}
	input := &transaction.TxInput{
		PreviousOutput:  testutil.CreateOutPoint(nil, 0),
		IsReferenceOnly: false,
		UnlockingProof: &transaction.TxInput_ThresholdProof{
			ThresholdProof: thresholdProof,
		},
	}
	tx := testutil.CreateTransaction([]*transaction.TxInput{input}, nil)

	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ThresholdProof{
			ThresholdProof: thresholdProof,
		},
	}

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.NoError(t, err)
	assert.True(t, matched)
}
