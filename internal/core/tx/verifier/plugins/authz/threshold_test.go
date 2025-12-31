// Package authz_test 提供 ThresholdPlugin 的单元测试
//
// 🧪 **测试规范遵循**：
// - 每个源文件对应一个测试文件（threshold.go → threshold_test.go）
// - 遵循测试规范：docs/system/standards/principles/testing-standards.md
package authz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/core/tx/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// TestNewThresholdPlugin 测试创建 ThresholdPlugin
func TestNewThresholdPlugin(t *testing.T) {
	plugin := NewThresholdPlugin()

	assert.NotNil(t, plugin)
}

// TestThresholdPlugin_Name 测试插件名称
func TestThresholdPlugin_Name(t *testing.T) {
	plugin := NewThresholdPlugin()

	assert.Equal(t, "threshold", plugin.Name())
}

// TestThresholdPlugin_Match_NotThresholdLock 测试不匹配其他锁类型
func TestThresholdPlugin_Match_NotThresholdLock(t *testing.T) {
	plugin := NewThresholdPlugin()

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

// TestThresholdPlugin_Match_MissingProof 测试缺少 proof
func TestThresholdPlugin_Match_MissingProof(t *testing.T) {
	plugin := NewThresholdPlugin()

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ThresholdLock{
			ThresholdLock: &transaction.ThresholdLock{
				Threshold:    3,
				TotalParties: 5,
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
	assert.Contains(t, err.Error(), "missing threshold proof")
}

// TestThresholdPlugin_Match_InsufficientShares 测试份额不足
func TestThresholdPlugin_Match_InsufficientShares(t *testing.T) {
	plugin := NewThresholdPlugin()

	partyKeys := [][]byte{
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
	}
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ThresholdLock{
			ThresholdLock: &transaction.ThresholdLock{
				Threshold:            3,
				TotalParties:         5,
				PartyVerificationKeys: partyKeys,
				SignatureScheme:      "BLS_THRESHOLD",
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ThresholdProof{
			ThresholdProof: &transaction.ThresholdProof{
				Shares: []*transaction.ThresholdProof_ThresholdSignatureShare{
					{PartyId: 0, SignatureShare: testutil.RandomBytes(96), VerificationKey: partyKeys[0]},
					{PartyId: 1, SignatureShare: testutil.RandomBytes(96), VerificationKey: partyKeys[1]},
					// 只有2个份额，需要3个
				},
				CombinedSignature: testutil.RandomBytes(96),
				SignatureScheme:   "BLS_THRESHOLD",
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "insufficient signature shares")
}

// TestThresholdPlugin_Match_DuplicatePartyID 测试重复的 party_id
func TestThresholdPlugin_Match_DuplicatePartyID(t *testing.T) {
	plugin := NewThresholdPlugin()

	partyKeys := [][]byte{
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
	}
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ThresholdLock{
			ThresholdLock: &transaction.ThresholdLock{
				Threshold:            2,
				TotalParties:         3,
				PartyVerificationKeys: partyKeys,
				SignatureScheme:      "BLS_THRESHOLD",
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ThresholdProof{
			ThresholdProof: &transaction.ThresholdProof{
				Shares: []*transaction.ThresholdProof_ThresholdSignatureShare{
					{PartyId: 0, SignatureShare: testutil.RandomBytes(96), VerificationKey: partyKeys[0]},
					{PartyId: 0, SignatureShare: testutil.RandomBytes(96), VerificationKey: partyKeys[0]}, // 重复的 party_id
				},
				CombinedSignature: testutil.RandomBytes(96),
				SignatureScheme:   "BLS_THRESHOLD",
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "duplicate party_id")
}

// TestThresholdPlugin_Match_PartyIDOutOfRange 测试 party_id 超出范围
func TestThresholdPlugin_Match_PartyIDOutOfRange(t *testing.T) {
	plugin := NewThresholdPlugin()

	partyKeys := [][]byte{
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
	}
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ThresholdLock{
			ThresholdLock: &transaction.ThresholdLock{
				Threshold:            2,
				TotalParties:         3,
				PartyVerificationKeys: partyKeys,
				SignatureScheme:      "BLS_THRESHOLD",
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ThresholdProof{
			ThresholdProof: &transaction.ThresholdProof{
				Shares: []*transaction.ThresholdProof_ThresholdSignatureShare{
					{PartyId: 0, SignatureShare: testutil.RandomBytes(96), VerificationKey: partyKeys[0]},
					{PartyId: 5, SignatureShare: testutil.RandomBytes(96), VerificationKey: partyKeys[0]}, // 超出范围
				},
				CombinedSignature: testutil.RandomBytes(96),
				SignatureScheme:   "BLS_THRESHOLD",
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "invalid party_id")
}

// TestThresholdPlugin_Match_VerificationKeyMismatch 测试验证密钥不匹配
func TestThresholdPlugin_Match_VerificationKeyMismatch(t *testing.T) {
	plugin := NewThresholdPlugin()

	partyKeys := [][]byte{
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
	}
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ThresholdLock{
			ThresholdLock: &transaction.ThresholdLock{
				Threshold:            2,
				TotalParties:         3,
				PartyVerificationKeys: partyKeys,
				SignatureScheme:      "BLS_THRESHOLD",
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ThresholdProof{
			ThresholdProof: &transaction.ThresholdProof{
				Shares: []*transaction.ThresholdProof_ThresholdSignatureShare{
					{PartyId: 0, SignatureShare: testutil.RandomBytes(96), VerificationKey: partyKeys[0]},
					{PartyId: 1, SignatureShare: testutil.RandomBytes(96), VerificationKey: testutil.RandomPublicKey()}, // 错误的密钥
				},
				CombinedSignature: testutil.RandomBytes(96),
				SignatureScheme:   "BLS_THRESHOLD",
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "verification key mismatch")
}

// TestThresholdPlugin_Match_EmptySignatureShare 测试空签名份额
func TestThresholdPlugin_Match_EmptySignatureShare(t *testing.T) {
	plugin := NewThresholdPlugin()

	partyKeys := [][]byte{
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
	}
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ThresholdLock{
			ThresholdLock: &transaction.ThresholdLock{
				Threshold:            2,
				TotalParties:         3,
				PartyVerificationKeys: partyKeys,
				SignatureScheme:      "BLS_THRESHOLD",
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ThresholdProof{
			ThresholdProof: &transaction.ThresholdProof{
				Shares: []*transaction.ThresholdProof_ThresholdSignatureShare{
					{PartyId: 0, SignatureShare: testutil.RandomBytes(96), VerificationKey: partyKeys[0]},
					{PartyId: 1, SignatureShare: nil, VerificationKey: partyKeys[1]}, // 空签名份额
				},
				CombinedSignature: testutil.RandomBytes(96),
				SignatureScheme:   "BLS_THRESHOLD",
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "empty signature share")
}

// TestThresholdPlugin_Match_EmptyCombinedSignature 测试空组合签名
func TestThresholdPlugin_Match_EmptyCombinedSignature(t *testing.T) {
	plugin := NewThresholdPlugin()

	partyKeys := [][]byte{
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
	}
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ThresholdLock{
			ThresholdLock: &transaction.ThresholdLock{
				Threshold:            2,
				TotalParties:         3,
				PartyVerificationKeys: partyKeys,
				SignatureScheme:      "BLS_THRESHOLD",
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ThresholdProof{
			ThresholdProof: &transaction.ThresholdProof{
				Shares: []*transaction.ThresholdProof_ThresholdSignatureShare{
					{PartyId: 0, SignatureShare: testutil.RandomBytes(96), VerificationKey: partyKeys[0]},
					{PartyId: 1, SignatureShare: testutil.RandomBytes(96), VerificationKey: partyKeys[1]},
				},
				CombinedSignature: nil, // 空组合签名
				SignatureScheme:   "BLS_THRESHOLD",
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "empty combined signature")
}

// TestThresholdPlugin_Match_SignatureSchemeMismatch 测试签名方案不匹配
func TestThresholdPlugin_Match_SignatureSchemeMismatch(t *testing.T) {
	plugin := NewThresholdPlugin()

	partyKeys := [][]byte{
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
	}
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ThresholdLock{
			ThresholdLock: &transaction.ThresholdLock{
				Threshold:            2,
				TotalParties:         3,
				PartyVerificationKeys: partyKeys,
				SignatureScheme:      "BLS_THRESHOLD",
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ThresholdProof{
			ThresholdProof: &transaction.ThresholdProof{
				Shares: []*transaction.ThresholdProof_ThresholdSignatureShare{
					{PartyId: 0, SignatureShare: testutil.RandomBytes(96), VerificationKey: partyKeys[0]},
					{PartyId: 1, SignatureShare: testutil.RandomBytes(96), VerificationKey: partyKeys[1]},
				},
				CombinedSignature: testutil.RandomBytes(96),
				SignatureScheme:   "ECDSA_TSS", // 不同的签名方案
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "signature scheme mismatch")
}

// TestThresholdPlugin_Match_Success 测试成功匹配
func TestThresholdPlugin_Match_Success(t *testing.T) {
	plugin := NewThresholdPlugin()

	partyKeys := [][]byte{
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
	}
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ThresholdLock{
			ThresholdLock: &transaction.ThresholdLock{
				Threshold:            3,
				TotalParties:         5,
				PartyVerificationKeys: partyKeys,
				SignatureScheme:      "BLS_THRESHOLD",
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ThresholdProof{
			ThresholdProof: &transaction.ThresholdProof{
				Shares: []*transaction.ThresholdProof_ThresholdSignatureShare{
					{PartyId: 0, SignatureShare: testutil.RandomBytes(96), VerificationKey: partyKeys[0]},
					{PartyId: 1, SignatureShare: testutil.RandomBytes(96), VerificationKey: partyKeys[1]},
					{PartyId: 2, SignatureShare: testutil.RandomBytes(96), VerificationKey: partyKeys[2]},
				},
				CombinedSignature: testutil.RandomBytes(96),
				SignatureScheme:   "BLS_THRESHOLD",
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.NoError(t, err)
	assert.True(t, matched)
}

