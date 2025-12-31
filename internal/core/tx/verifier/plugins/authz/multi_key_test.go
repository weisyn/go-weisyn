// Package authz_test 提供 MultiKeyPlugin 的单元测试
//
// 🧪 **测试规范遵循**：
// - 每个源文件对应一个测试文件（multi_key.go → multi_key_test.go）
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

// ==================== MultiKeyPlugin 测试 ====================

// TestNewMultiKeyPlugin 测试创建 MultiKeyPlugin
func TestNewMultiKeyPlugin(t *testing.T) {
	mockMultiSigVerifier := &MockMultiSignatureVerifier{}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewMultiKeyPlugin(mockMultiSigVerifier, mockCanonicalizer)

	assert.NotNil(t, plugin)
	assert.Equal(t, "multi_key", plugin.Name())
}

// TestMultiKeyPlugin_Match_Success 测试成功匹配
func TestMultiKeyPlugin_Match_Success(t *testing.T) {
	mockMultiSigVerifier := &MockMultiSignatureVerifier{shouldFail: false}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewMultiKeyPlugin(mockMultiSigVerifier, mockCanonicalizer)

	// 创建 3-of-5 多签锁
	publicKeys := [][]byte{
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
	}
	lock := testutil.CreateMultiKeyLock(publicKeys, 3)

	// 创建 MultiKeyProof（3 个签名）
	multiKeyProof := &transaction.MultiKeyProof{
		Signatures: []*transaction.MultiKeyProof_SignatureEntry{
			{
				KeyIndex:    0,
				Signature:   &transaction.SignatureData{Value: testutil.RandomBytes(64)},
				Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
				SighashType: transaction.SignatureHashType_SIGHASH_ALL,
			},
			{
				KeyIndex:    1,
				Signature:   &transaction.SignatureData{Value: testutil.RandomBytes(64)},
				Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
				SighashType: transaction.SignatureHashType_SIGHASH_ALL,
			},
			{
				KeyIndex:    2,
				Signature:   &transaction.SignatureData{Value: testutil.RandomBytes(64)},
				Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
				SighashType: transaction.SignatureHashType_SIGHASH_ALL,
			},
		},
	}

	input := &transaction.TxInput{
		PreviousOutput:  testutil.CreateOutPoint(nil, 0),
		IsReferenceOnly: false,
		UnlockingProof: &transaction.TxInput_MultiKeyProof{
			MultiKeyProof: multiKeyProof,
		},
	}
	tx := testutil.CreateTransaction([]*transaction.TxInput{input}, nil)

	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_MultiKeyProof{
			MultiKeyProof: multiKeyProof,
		},
	}

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.NoError(t, err)
	assert.True(t, matched)
}

// TestMultiKeyPlugin_Match_NilLock 测试 nil lock
func TestMultiKeyPlugin_Match_NilLock(t *testing.T) {
	mockMultiSigVerifier := &MockMultiSignatureVerifier{}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewMultiKeyPlugin(mockMultiSigVerifier, mockCanonicalizer)

	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_MultiKeyProof{
			MultiKeyProof: &transaction.MultiKeyProof{},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	defer func() {
		if r := recover(); r != nil {
			assert.NotNil(t, r)
		}
	}()

	matched, err := plugin.Match(context.Background(), nil, proof, tx)

	if err == nil {
		assert.False(t, matched)
	}
}

// TestMultiKeyPlugin_Match_MissingProof 测试缺少 proof
func TestMultiKeyPlugin_Match_MissingProof(t *testing.T) {
	mockMultiSigVerifier := &MockMultiSignatureVerifier{}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewMultiKeyPlugin(mockMultiSigVerifier, mockCanonicalizer)

	publicKeys := [][]byte{testutil.RandomPublicKey(), testutil.RandomPublicKey()}
	lock := testutil.CreateMultiKeyLock(publicKeys, 1)
	proof := &transaction.UnlockingProof{
		Proof: nil, // 空的 proof
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "MultiKeyProof")
}

// TestMultiKeyPlugin_Match_InsufficientSignatures 测试签名不足
func TestMultiKeyPlugin_Match_InsufficientSignatures(t *testing.T) {
	mockMultiSigVerifier := &MockMultiSignatureVerifier{}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewMultiKeyPlugin(mockMultiSigVerifier, mockCanonicalizer)

	// 创建 3-of-5 多签锁
	publicKeys := [][]byte{
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
	}
	lock := testutil.CreateMultiKeyLock(publicKeys, 3)

	// 只提供 2 个签名（需要 3 个）
	multiKeyProof := &transaction.MultiKeyProof{
		Signatures: []*transaction.MultiKeyProof_SignatureEntry{
			{
				KeyIndex:    0,
				Signature:   &transaction.SignatureData{Value: testutil.RandomBytes(64)},
				Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
				SighashType: transaction.SignatureHashType_SIGHASH_ALL,
			},
			{
				KeyIndex:    1,
				Signature:   &transaction.SignatureData{Value: testutil.RandomBytes(64)},
				Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
				SighashType: transaction.SignatureHashType_SIGHASH_ALL,
			},
		},
	}

	input := &transaction.TxInput{
		PreviousOutput:  testutil.CreateOutPoint(nil, 0),
		IsReferenceOnly: false,
		UnlockingProof: &transaction.TxInput_MultiKeyProof{
			MultiKeyProof: multiKeyProof,
		},
	}
	tx := testutil.CreateTransaction([]*transaction.TxInput{input}, nil)

	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_MultiKeyProof{
			MultiKeyProof: multiKeyProof,
		},
	}

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "insufficient signatures")
}

// TestMultiKeyPlugin_Match_InputIndexNotFound 测试找不到输入索引
func TestMultiKeyPlugin_Match_InputIndexNotFound(t *testing.T) {
	mockMultiSigVerifier := &MockMultiSignatureVerifier{}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewMultiKeyPlugin(mockMultiSigVerifier, mockCanonicalizer)

	publicKeys := [][]byte{testutil.RandomPublicKey(), testutil.RandomPublicKey()}
	lock := testutil.CreateMultiKeyLock(publicKeys, 1)

	multiKeyProof := &transaction.MultiKeyProof{
		Signatures: []*transaction.MultiKeyProof_SignatureEntry{
			{
				KeyIndex:    0,
				Signature:   &transaction.SignatureData{Value: testutil.RandomBytes(64)},
				Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
				SighashType: transaction.SignatureHashType_SIGHASH_ALL,
			},
		},
	}

	// 创建一个不同的 proof 对象
	differentProof := &transaction.MultiKeyProof{
		Signatures: []*transaction.MultiKeyProof_SignatureEntry{
			{
				KeyIndex:    0,
				Signature:   &transaction.SignatureData{Value: testutil.RandomBytes(64)},
				Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
				SighashType: transaction.SignatureHashType_SIGHASH_ALL,
			},
		},
	}

	input := &transaction.TxInput{
		PreviousOutput:  testutil.CreateOutPoint(nil, 0),
		IsReferenceOnly: false,
		UnlockingProof: &transaction.TxInput_MultiKeyProof{
			MultiKeyProof: differentProof, // 使用不同的 proof 对象
		},
	}
	tx := testutil.CreateTransaction([]*transaction.TxInput{input}, nil)

	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_MultiKeyProof{
			MultiKeyProof: multiKeyProof, // 使用不同的 proof 对象
		},
	}

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "无法找到当前输入的索引")
}

// TestMultiKeyPlugin_Match_ComputeSignatureHashError 测试计算签名哈希错误
func TestMultiKeyPlugin_Match_ComputeSignatureHashError(t *testing.T) {
	mockMultiSigVerifier := &MockMultiSignatureVerifier{}
	errorClient := &ErrorMockTransactionHashServiceClient{
		computeSignatureHashError: fmt.Errorf("计算签名哈希失败"),
	}
	mockCanonicalizer := hash.NewCanonicalizer(errorClient)

	plugin := NewMultiKeyPlugin(mockMultiSigVerifier, mockCanonicalizer)

	publicKeys := [][]byte{testutil.RandomPublicKey(), testutil.RandomPublicKey()}
	lock := testutil.CreateMultiKeyLock(publicKeys, 1)

	multiKeyProof := &transaction.MultiKeyProof{
		Signatures: []*transaction.MultiKeyProof_SignatureEntry{
			{
				KeyIndex:    0,
				Signature:   &transaction.SignatureData{Value: testutil.RandomBytes(64)},
				Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
				SighashType: transaction.SignatureHashType_SIGHASH_ALL,
			},
		},
	}

	input := &transaction.TxInput{
		PreviousOutput:  testutil.CreateOutPoint(nil, 0),
		IsReferenceOnly: false,
		UnlockingProof: &transaction.TxInput_MultiKeyProof{
			MultiKeyProof: multiKeyProof,
		},
	}
	tx := testutil.CreateTransaction([]*transaction.TxInput{input}, nil)

	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_MultiKeyProof{
			MultiKeyProof: multiKeyProof,
		},
	}

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "计算签名哈希失败")
}

// TestMultiKeyPlugin_Match_KeyIndexOutOfRange 测试密钥索引越界
func TestMultiKeyPlugin_Match_KeyIndexOutOfRange(t *testing.T) {
	mockMultiSigVerifier := &MockMultiSignatureVerifier{}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewMultiKeyPlugin(mockMultiSigVerifier, mockCanonicalizer)

	// 创建 2 个公钥的锁
	publicKeys := [][]byte{testutil.RandomPublicKey(), testutil.RandomPublicKey()}
	lock := testutil.CreateMultiKeyLock(publicKeys, 1)

	// 使用 key_index=2（超出范围，只有 0 和 1）
	multiKeyProof := &transaction.MultiKeyProof{
		Signatures: []*transaction.MultiKeyProof_SignatureEntry{
			{
				KeyIndex:    2, // 超出范围
				Signature:   &transaction.SignatureData{Value: testutil.RandomBytes(64)},
				Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
				SighashType: transaction.SignatureHashType_SIGHASH_ALL,
			},
		},
	}

	input := &transaction.TxInput{
		PreviousOutput:  testutil.CreateOutPoint(nil, 0),
		IsReferenceOnly: false,
		UnlockingProof: &transaction.TxInput_MultiKeyProof{
			MultiKeyProof: multiKeyProof,
		},
	}
	tx := testutil.CreateTransaction([]*transaction.TxInput{input}, nil)

	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_MultiKeyProof{
			MultiKeyProof: multiKeyProof,
		},
	}

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "超出范围")
}

// TestMultiKeyPlugin_Match_SignatureVerificationFailure 测试签名验证失败
func TestMultiKeyPlugin_Match_SignatureVerificationFailure(t *testing.T) {
	mockMultiSigVerifier := &MockMultiSignatureVerifier{shouldFail: true}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewMultiKeyPlugin(mockMultiSigVerifier, mockCanonicalizer)

	publicKeys := [][]byte{testutil.RandomPublicKey(), testutil.RandomPublicKey()}
	lock := testutil.CreateMultiKeyLock(publicKeys, 1)

	multiKeyProof := &transaction.MultiKeyProof{
		Signatures: []*transaction.MultiKeyProof_SignatureEntry{
			{
				KeyIndex:    0,
				Signature:   &transaction.SignatureData{Value: testutil.RandomBytes(64)},
				Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
				SighashType: transaction.SignatureHashType_SIGHASH_ALL,
			},
		},
	}

	input := &transaction.TxInput{
		PreviousOutput:  testutil.CreateOutPoint(nil, 0),
		IsReferenceOnly: false,
		UnlockingProof: &transaction.TxInput_MultiKeyProof{
			MultiKeyProof: multiKeyProof,
		},
	}
	tx := testutil.CreateTransaction([]*transaction.TxInput{input}, nil)

	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_MultiKeyProof{
			MultiKeyProof: multiKeyProof,
		},
	}

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "多重签名验证失败")
}

// TestMultiKeyPlugin_Match_DifferentSighashTypes_Success 测试不同 SighashType 成功
func TestMultiKeyPlugin_Match_DifferentSighashTypes_Success(t *testing.T) {
	mockMultiSigVerifier := &MockMultiSignatureVerifier{shouldFail: false}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewMultiKeyPlugin(mockMultiSigVerifier, mockCanonicalizer)

	publicKeys := [][]byte{
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
	}
	lock := testutil.CreateMultiKeyLock(publicKeys, 2)

	// 使用不同的 SighashType
	multiKeyProof := &transaction.MultiKeyProof{
		Signatures: []*transaction.MultiKeyProof_SignatureEntry{
			{
				KeyIndex:    0,
				Signature:   &transaction.SignatureData{Value: testutil.RandomBytes(64)},
				Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
				SighashType: transaction.SignatureHashType_SIGHASH_ALL,
			},
			{
				KeyIndex:    1,
				Signature:   &transaction.SignatureData{Value: testutil.RandomBytes(64)},
				Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
				SighashType: transaction.SignatureHashType_SIGHASH_NONE, // 不同的 SighashType
			},
		},
	}

	input := &transaction.TxInput{
		PreviousOutput:  testutil.CreateOutPoint(nil, 0),
		IsReferenceOnly: false,
		UnlockingProof: &transaction.TxInput_MultiKeyProof{
			MultiKeyProof: multiKeyProof,
		},
	}
	tx := testutil.CreateTransaction([]*transaction.TxInput{input}, nil)

	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_MultiKeyProof{
			MultiKeyProof: multiKeyProof,
		},
	}

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	// 由于 MockMultiSignatureVerifier 总是返回成功，这里应该通过
	assert.NoError(t, err)
	assert.True(t, matched)
}

// TestMultiKeyPlugin_Match_DifferentSighashTypes_Failure 测试不同 SighashType 失败
func TestMultiKeyPlugin_Match_DifferentSighashTypes_Failure(t *testing.T) {
	mockMultiSigVerifier := &MockMultiSignatureVerifier{shouldFail: true}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewMultiKeyPlugin(mockMultiSigVerifier, mockCanonicalizer)

	publicKeys := [][]byte{
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
		testutil.RandomPublicKey(),
	}
	lock := testutil.CreateMultiKeyLock(publicKeys, 2)

	// 使用不同的 SighashType
	multiKeyProof := &transaction.MultiKeyProof{
		Signatures: []*transaction.MultiKeyProof_SignatureEntry{
			{
				KeyIndex:    0,
				Signature:   &transaction.SignatureData{Value: testutil.RandomBytes(64)},
				Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
				SighashType: transaction.SignatureHashType_SIGHASH_ALL,
			},
			{
				KeyIndex:    1,
				Signature:   &transaction.SignatureData{Value: testutil.RandomBytes(64)},
				Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
				SighashType: transaction.SignatureHashType_SIGHASH_NONE, // 不同的 SighashType
			},
		},
	}

	input := &transaction.TxInput{
		PreviousOutput:  testutil.CreateOutPoint(nil, 0),
		IsReferenceOnly: false,
		UnlockingProof: &transaction.TxInput_MultiKeyProof{
			MultiKeyProof: multiKeyProof,
		},
	}
	tx := testutil.CreateTransaction([]*transaction.TxInput{input}, nil)

	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_MultiKeyProof{
			MultiKeyProof: multiKeyProof,
		},
	}

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	// 由于 MockMultiSignatureVerifier 返回失败，这里应该失败
	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "验证失败")
}

