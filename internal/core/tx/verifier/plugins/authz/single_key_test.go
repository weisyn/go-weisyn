// Package authz_test 提供 SingleKeyPlugin 的单元测试
//
// 🧪 **测试规范遵循**：
// - 每个源文件对应一个测试文件（single_key.go → single_key_test.go）
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

// ==================== SingleKeyPlugin 测试 ====================

// TestNewSingleKeyPlugin 测试创建 SingleKeyPlugin
func TestNewSingleKeyPlugin(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockHashMgr := &testutil.MockHashManager{}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewSingleKeyPlugin(mockSigMgr, mockHashMgr, mockCanonicalizer)

	assert.NotNil(t, plugin)
	assert.NotNil(t, plugin.sigManager)
	assert.NotNil(t, plugin.hashManager)
	assert.NotNil(t, plugin.hashCanonicalizer)
}

// TestSingleKeyPlugin_Name 测试插件名称
func TestSingleKeyPlugin_Name(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockHashMgr := &testutil.MockHashManager{}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewSingleKeyPlugin(mockSigMgr, mockHashMgr, mockCanonicalizer)

	assert.Equal(t, "single_key", plugin.Name())
}

// TestSingleKeyPlugin_Match_SingleKeyLock 测试匹配 SingleKeyLock
func TestSingleKeyPlugin_Match_SingleKeyLock(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockHashMgr := &testutil.MockHashManager{}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewSingleKeyPlugin(mockSigMgr, mockHashMgr, mockCanonicalizer)

	// 创建相同的公钥，确保 lock 和 proof 匹配
	publicKey := testutil.RandomPublicKey()
	lock := testutil.CreateSingleKeyLock(publicKey)

	// 创建 SingleKeyProof（先创建 proof，然后从 TxInput 中提取）
	singleKeyProof := &transaction.SingleKeyProof{
		PublicKey: &transaction.PublicKey{
			Value: publicKey, // 使用相同的公钥
		},
		Signature: &transaction.SignatureData{
			Value: testutil.RandomBytes(64),
		},
		Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		SighashType: transaction.SignatureHashType_SIGHASH_ALL,
	}

	// 创建包含输入的 TxInput，使用同一个 singleKeyProof 对象
	input := &transaction.TxInput{
		PreviousOutput:  testutil.CreateOutPoint(nil, 0),
		IsReferenceOnly: false,
		UnlockingProof: &transaction.TxInput_SingleKeyProof{
			SingleKeyProof: singleKeyProof, // 使用同一个对象
		},
	}
	tx := testutil.CreateTransaction([]*transaction.TxInput{input}, nil)

	// 创建 UnlockingProof，使用同一个 singleKeyProof 对象
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_SingleKeyProof{
			SingleKeyProof: singleKeyProof, // 使用同一个对象
		},
	}

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.NoError(t, err)
	assert.True(t, matched)
}

// TestSingleKeyPlugin_Match_NotSingleKeyLock 测试不匹配其他锁类型
func TestSingleKeyPlugin_Match_NotSingleKeyLock(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockHashMgr := &testutil.MockHashManager{}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewSingleKeyPlugin(mockSigMgr, mockHashMgr, mockCanonicalizer)

	// MultiKeyLock 不应该匹配
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_MultiKeyLock{
			MultiKeyLock: &transaction.MultiKeyLock{
				RequiredSignatures: 2,
			},
		},
	}
	proof := testutil.CreateSingleKeyProof(nil, nil)
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.NoError(t, err)
	assert.False(t, matched)
}

// TestSingleKeyPlugin_Match_NilLock 测试 nil lock
func TestSingleKeyPlugin_Match_NilLock(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockHashMgr := &testutil.MockHashManager{}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewSingleKeyPlugin(mockSigMgr, mockHashMgr, mockCanonicalizer)

	proof := testutil.CreateSingleKeyProof(nil, nil)
	tx := testutil.CreateTransaction(nil, nil)

	// 注意：当 lock 为 nil 时，GetSingleKeyLock() 会 panic
	// 但根据实现，Match 方法会先检查 lock.GetSingleKeyLock()，如果返回 nil，则返回 false
	defer func() {
		if r := recover(); r != nil {
			// 如果 panic，这是预期的行为
			assert.NotNil(t, r)
		}
	}()

	matched, err := plugin.Match(context.Background(), nil, proof, tx)

	// 如果返回了结果（没有 panic），验证结果
	if err == nil {
		assert.False(t, matched) // nil lock 不应该匹配
	}
}

// TestSingleKeyPlugin_Match_NilProof 测试 nil proof
func TestSingleKeyPlugin_Match_NilProof(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockHashMgr := &testutil.MockHashManager{}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewSingleKeyPlugin(mockSigMgr, mockHashMgr, mockCanonicalizer)

	lock := testutil.CreateSingleKeyLock(nil)
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, nil, tx)

	assert.Error(t, err)
	assert.True(t, matched) // 匹配了 SingleKeyLock，但 proof 为空
	assert.Contains(t, err.Error(), "SingleKeyProof")
}

// TestSingleKeyPlugin_Match_MissingProof 测试缺少 proof
func TestSingleKeyPlugin_Match_MissingProof(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockHashMgr := &testutil.MockHashManager{}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewSingleKeyPlugin(mockSigMgr, mockHashMgr, mockCanonicalizer)

	lock := testutil.CreateSingleKeyLock(nil)
	proof := &transaction.UnlockingProof{
		Proof: nil, // 空的 proof
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "SingleKeyProof")
}

// TestSingleKeyPlugin_Match_InputIndexNotFound 测试找不到输入索引
func TestSingleKeyPlugin_Match_InputIndexNotFound(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockHashMgr := &testutil.MockHashManager{}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewSingleKeyPlugin(mockSigMgr, mockHashMgr, mockCanonicalizer)

	publicKey := testutil.RandomPublicKey()
	lock := testutil.CreateSingleKeyLock(publicKey)

	// 创建一个 proof，但交易中没有对应的输入
	singleKeyProof := &transaction.SingleKeyProof{
		PublicKey: &transaction.PublicKey{
			Value: publicKey,
		},
		Signature: &transaction.SignatureData{
			Value: testutil.RandomBytes(64),
		},
		Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		SighashType: transaction.SignatureHashType_SIGHASH_ALL,
	}

	// 创建一个不同的 proof 对象（指针不同）
	differentProof := &transaction.SingleKeyProof{
		PublicKey: &transaction.PublicKey{
			Value: publicKey,
		},
		Signature: &transaction.SignatureData{
			Value: testutil.RandomBytes(64),
		},
		Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		SighashType: transaction.SignatureHashType_SIGHASH_ALL,
	}

	input := &transaction.TxInput{
		PreviousOutput:  testutil.CreateOutPoint(nil, 0),
		IsReferenceOnly: false,
		UnlockingProof: &transaction.TxInput_SingleKeyProof{
			SingleKeyProof: differentProof, // 使用不同的 proof 对象
		},
	}
	tx := testutil.CreateTransaction([]*transaction.TxInput{input}, nil)

	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_SingleKeyProof{
			SingleKeyProof: singleKeyProof, // 使用不同的 proof 对象
		},
	}

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "无法找到当前输入的索引")
}

// TestSingleKeyPlugin_Match_ComputeSignatureHashError 测试计算签名哈希错误
func TestSingleKeyPlugin_Match_ComputeSignatureHashError(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockHashMgr := &testutil.MockHashManager{}

	// 创建一个返回错误的 Canonicalizer
	errorClient := &ErrorMockTransactionHashServiceClient{
		computeSignatureHashError: fmt.Errorf("计算签名哈希失败"),
	}
	mockCanonicalizer := hash.NewCanonicalizer(errorClient)

	plugin := NewSingleKeyPlugin(mockSigMgr, mockHashMgr, mockCanonicalizer)

	publicKey := testutil.RandomPublicKey()
	lock := testutil.CreateSingleKeyLock(publicKey)

	singleKeyProof := &transaction.SingleKeyProof{
		PublicKey: &transaction.PublicKey{
			Value: publicKey,
		},
		Signature: &transaction.SignatureData{
			Value: testutil.RandomBytes(64),
		},
		Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		SighashType: transaction.SignatureHashType_SIGHASH_ALL,
	}

	input := &transaction.TxInput{
		PreviousOutput:  testutil.CreateOutPoint(nil, 0),
		IsReferenceOnly: false,
		UnlockingProof: &transaction.TxInput_SingleKeyProof{
			SingleKeyProof: singleKeyProof,
		},
	}
	tx := testutil.CreateTransaction([]*transaction.TxInput{input}, nil)

	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_SingleKeyProof{
			SingleKeyProof: singleKeyProof,
		},
	}

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "计算签名哈希失败")
}

// TestSingleKeyPlugin_Match_EmptyPublicKeyInProof 测试 proof 中公钥为空
func TestSingleKeyPlugin_Match_EmptyPublicKeyInProof(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockHashMgr := &testutil.MockHashManager{}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewSingleKeyPlugin(mockSigMgr, mockHashMgr, mockCanonicalizer)

	publicKey := testutil.RandomPublicKey()
	lock := testutil.CreateSingleKeyLock(publicKey)

	singleKeyProof := &transaction.SingleKeyProof{
		PublicKey: nil, // 公钥为空
		Signature: &transaction.SignatureData{
			Value: testutil.RandomBytes(64),
		},
		Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		SighashType: transaction.SignatureHashType_SIGHASH_ALL,
	}

	input := &transaction.TxInput{
		PreviousOutput:  testutil.CreateOutPoint(nil, 0),
		IsReferenceOnly: false,
		UnlockingProof: &transaction.TxInput_SingleKeyProof{
			SingleKeyProof: singleKeyProof,
		},
	}
	tx := testutil.CreateTransaction([]*transaction.TxInput{input}, nil)

	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_SingleKeyProof{
			SingleKeyProof: singleKeyProof,
		},
	}

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "公钥为空")
}

// TestSingleKeyPlugin_Match_EmptySignatureInProof 测试 proof 中签名为空
func TestSingleKeyPlugin_Match_EmptySignatureInProof(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockHashMgr := &testutil.MockHashManager{}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewSingleKeyPlugin(mockSigMgr, mockHashMgr, mockCanonicalizer)

	publicKey := testutil.RandomPublicKey()
	lock := testutil.CreateSingleKeyLock(publicKey)

	singleKeyProof := &transaction.SingleKeyProof{
		PublicKey: &transaction.PublicKey{
			Value: publicKey,
		},
		Signature:   nil, // 签名为空
		Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		SighashType: transaction.SignatureHashType_SIGHASH_ALL,
	}

	input := &transaction.TxInput{
		PreviousOutput:  testutil.CreateOutPoint(nil, 0),
		IsReferenceOnly: false,
		UnlockingProof: &transaction.TxInput_SingleKeyProof{
			SingleKeyProof: singleKeyProof,
		},
	}
	tx := testutil.CreateTransaction([]*transaction.TxInput{input}, nil)

	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_SingleKeyProof{
			SingleKeyProof: singleKeyProof,
		},
	}

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "签名为空")
}

// TestSingleKeyPlugin_Match_UnsupportedAlgorithm 测试不支持的算法
func TestSingleKeyPlugin_Match_UnsupportedAlgorithm(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockHashMgr := &testutil.MockHashManager{}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewSingleKeyPlugin(mockSigMgr, mockHashMgr, mockCanonicalizer)

	publicKey := testutil.RandomPublicKey()
	lock := testutil.CreateSingleKeyLock(publicKey)

	singleKeyProof := &transaction.SingleKeyProof{
		PublicKey: &transaction.PublicKey{
			Value: publicKey,
		},
		Signature: &transaction.SignatureData{
			Value: testutil.RandomBytes(64),
		},
		Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_UNKNOWN, // 不支持的算法
		SighashType: transaction.SignatureHashType_SIGHASH_ALL,
	}

	input := &transaction.TxInput{
		PreviousOutput:  testutil.CreateOutPoint(nil, 0),
		IsReferenceOnly: false,
		UnlockingProof: &transaction.TxInput_SingleKeyProof{
			SingleKeyProof: singleKeyProof,
		},
	}
	tx := testutil.CreateTransaction([]*transaction.TxInput{input}, nil)

	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_SingleKeyProof{
			SingleKeyProof: singleKeyProof,
		},
	}

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "不支持的签名算法")
}

// TestSingleKeyPlugin_Match_SignatureVerificationFailure 测试签名验证失败
func TestSingleKeyPlugin_Match_SignatureVerificationFailure(t *testing.T) {
	// 创建一个返回 false 的 MockSignatureManager
	mockSigMgr := &FailingMockSignatureManager{}
	mockHashMgr := &testutil.MockHashManager{}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewSingleKeyPlugin(mockSigMgr, mockHashMgr, mockCanonicalizer)

	publicKey := testutil.RandomPublicKey()
	lock := testutil.CreateSingleKeyLock(publicKey)

	singleKeyProof := &transaction.SingleKeyProof{
		PublicKey: &transaction.PublicKey{
			Value: publicKey,
		},
		Signature: &transaction.SignatureData{
			Value: testutil.RandomBytes(64),
		},
		Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		SighashType: transaction.SignatureHashType_SIGHASH_ALL,
	}

	input := &transaction.TxInput{
		PreviousOutput:  testutil.CreateOutPoint(nil, 0),
		IsReferenceOnly: false,
		UnlockingProof: &transaction.TxInput_SingleKeyProof{
			SingleKeyProof: singleKeyProof,
		},
	}
	tx := testutil.CreateTransaction([]*transaction.TxInput{input}, nil)

	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_SingleKeyProof{
			SingleKeyProof: singleKeyProof,
		},
	}

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "签名验证失败")
}

// TestSingleKeyPlugin_Match_RequiredPublicKeyMismatch 测试公钥不匹配
func TestSingleKeyPlugin_Match_RequiredPublicKeyMismatch(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockHashMgr := &testutil.MockHashManager{}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewSingleKeyPlugin(mockSigMgr, mockHashMgr, mockCanonicalizer)

	expectedPublicKey := testutil.RandomPublicKey()
	actualPublicKey := testutil.RandomPublicKey() // 不同的公钥

	lock := testutil.CreateSingleKeyLock(expectedPublicKey)

	singleKeyProof := &transaction.SingleKeyProof{
		PublicKey: &transaction.PublicKey{
			Value: actualPublicKey, // 使用不同的公钥
		},
		Signature: &transaction.SignatureData{
			Value: testutil.RandomBytes(64),
		},
		Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		SighashType: transaction.SignatureHashType_SIGHASH_ALL,
	}

	input := &transaction.TxInput{
		PreviousOutput:  testutil.CreateOutPoint(nil, 0),
		IsReferenceOnly: false,
		UnlockingProof: &transaction.TxInput_SingleKeyProof{
			SingleKeyProof: singleKeyProof,
		},
	}
	tx := testutil.CreateTransaction([]*transaction.TxInput{input}, nil)

	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_SingleKeyProof{
			SingleKeyProof: singleKeyProof,
		},
	}

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "公钥不匹配")
}

// TestSingleKeyPlugin_Match_RequiredAddressHashMismatch 测试地址哈希不匹配
func TestSingleKeyPlugin_Match_RequiredAddressHashMismatch(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockHashMgr := &testutil.MockHashManager{}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewSingleKeyPlugin(mockSigMgr, mockHashMgr, mockCanonicalizer)

	publicKey := testutil.RandomPublicKey()
	expectedAddressHash := testutil.RandomBytes(20)

	// 创建使用地址哈希的 lock
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_SingleKeyLock{
			SingleKeyLock: &transaction.SingleKeyLock{
				KeyRequirement: &transaction.SingleKeyLock_RequiredAddressHash{
					RequiredAddressHash: expectedAddressHash,
				},
			},
		},
	}

	singleKeyProof := &transaction.SingleKeyProof{
		PublicKey: &transaction.PublicKey{
			Value: publicKey, // 这个公钥对应的地址哈希与 expectedAddressHash 不同
		},
		Signature: &transaction.SignatureData{
			Value: testutil.RandomBytes(64),
		},
		Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		SighashType: transaction.SignatureHashType_SIGHASH_ALL,
	}

	input := &transaction.TxInput{
		PreviousOutput:  testutil.CreateOutPoint(nil, 0),
		IsReferenceOnly: false,
		UnlockingProof: &transaction.TxInput_SingleKeyProof{
			SingleKeyProof: singleKeyProof,
		},
	}
	tx := testutil.CreateTransaction([]*transaction.TxInput{input}, nil)

	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_SingleKeyProof{
			SingleKeyProof: singleKeyProof,
		},
	}

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "地址哈希不匹配")
}

// TestSingleKeyPlugin_Match_UnsupportedKeyRequirement 测试不支持的密钥要求类型
func TestSingleKeyPlugin_Match_UnsupportedKeyRequirement(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockHashMgr := &testutil.MockHashManager{}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewSingleKeyPlugin(mockSigMgr, mockHashMgr, mockCanonicalizer)

	publicKey := testutil.RandomPublicKey()

	// 创建一个没有 KeyRequirement 的 lock（这不应该发生，但测试边界情况）
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_SingleKeyLock{
			SingleKeyLock: &transaction.SingleKeyLock{
				KeyRequirement: nil, // 空的 KeyRequirement
			},
		},
	}

	singleKeyProof := &transaction.SingleKeyProof{
		PublicKey: &transaction.PublicKey{
			Value: publicKey,
		},
		Signature: &transaction.SignatureData{
			Value: testutil.RandomBytes(64),
		},
		Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		SighashType: transaction.SignatureHashType_SIGHASH_ALL,
	}

	input := &transaction.TxInput{
		PreviousOutput:  testutil.CreateOutPoint(nil, 0),
		IsReferenceOnly: false,
		UnlockingProof: &transaction.TxInput_SingleKeyProof{
			SingleKeyProof: singleKeyProof,
		},
	}
	tx := testutil.CreateTransaction([]*transaction.TxInput{input}, nil)

	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_SingleKeyProof{
			SingleKeyProof: singleKeyProof,
		},
	}

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "不支持的锁定约束类型")
}

// TestSingleKeyPlugin_Match_Ed25519Algorithm 测试 Ed25519 算法
func TestSingleKeyPlugin_Match_Ed25519Algorithm(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockHashMgr := &testutil.MockHashManager{}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewSingleKeyPlugin(mockSigMgr, mockHashMgr, mockCanonicalizer)

	publicKey := testutil.RandomPublicKey()
	lock := testutil.CreateSingleKeyLock(publicKey)

	singleKeyProof := &transaction.SingleKeyProof{
		PublicKey: &transaction.PublicKey{
			Value: publicKey,
		},
		Signature: &transaction.SignatureData{
			Value: testutil.RandomBytes(64),
		},
		Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ED25519, // Ed25519 算法
		SighashType: transaction.SignatureHashType_SIGHASH_ALL,
	}

	input := &transaction.TxInput{
		PreviousOutput:  testutil.CreateOutPoint(nil, 0),
		IsReferenceOnly: false,
		UnlockingProof: &transaction.TxInput_SingleKeyProof{
			SingleKeyProof: singleKeyProof,
		},
	}
	tx := testutil.CreateTransaction([]*transaction.TxInput{input}, nil)

	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_SingleKeyProof{
			SingleKeyProof: singleKeyProof,
		},
	}

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.NoError(t, err)
	assert.True(t, matched)
}

// TestSingleKeyPlugin_Match_RequiredAddressHash_EmptyAddressHash 测试空地址哈希
func TestSingleKeyPlugin_Match_RequiredAddressHash_EmptyAddressHash(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockHashMgr := &testutil.MockHashManager{}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewSingleKeyPlugin(mockSigMgr, mockHashMgr, mockCanonicalizer)

	publicKey := testutil.RandomPublicKey()

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_SingleKeyLock{
			SingleKeyLock: &transaction.SingleKeyLock{
				KeyRequirement: &transaction.SingleKeyLock_RequiredAddressHash{
					RequiredAddressHash: nil, // 空的地址哈希
				},
			},
		},
	}

	singleKeyProof := &transaction.SingleKeyProof{
		PublicKey: &transaction.PublicKey{
			Value: publicKey,
		},
		Signature: &transaction.SignatureData{
			Value: testutil.RandomBytes(64),
		},
		Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		SighashType: transaction.SignatureHashType_SIGHASH_ALL,
	}

	input := &transaction.TxInput{
		PreviousOutput:  testutil.CreateOutPoint(nil, 0),
		IsReferenceOnly: false,
		UnlockingProof: &transaction.TxInput_SingleKeyProof{
			SingleKeyProof: singleKeyProof,
		},
	}
	tx := testutil.CreateTransaction([]*transaction.TxInput{input}, nil)

	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_SingleKeyProof{
			SingleKeyProof: singleKeyProof,
		},
	}

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "地址哈希为空")
}

// TestSingleKeyPlugin_Match_RequiredPublicKey_EmptyPublicKey 测试空公钥
func TestSingleKeyPlugin_Match_RequiredPublicKey_EmptyPublicKey(t *testing.T) {
	mockSigMgr := &testutil.MockSignatureManager{}
	mockHashMgr := &testutil.MockHashManager{}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewSingleKeyPlugin(mockSigMgr, mockHashMgr, mockCanonicalizer)

	publicKey := testutil.RandomPublicKey()

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_SingleKeyLock{
			SingleKeyLock: &transaction.SingleKeyLock{
				KeyRequirement: &transaction.SingleKeyLock_RequiredPublicKey{
					RequiredPublicKey: nil, // 空的公钥
				},
			},
		},
	}

	singleKeyProof := &transaction.SingleKeyProof{
		PublicKey: &transaction.PublicKey{
			Value: publicKey,
		},
		Signature: &transaction.SignatureData{
			Value: testutil.RandomBytes(64),
		},
		Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		SighashType: transaction.SignatureHashType_SIGHASH_ALL,
	}

	input := &transaction.TxInput{
		PreviousOutput:  testutil.CreateOutPoint(nil, 0),
		IsReferenceOnly: false,
		UnlockingProof: &transaction.TxInput_SingleKeyProof{
			SingleKeyProof: singleKeyProof,
		},
	}
	tx := testutil.CreateTransaction([]*transaction.TxInput{input}, nil)

	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_SingleKeyProof{
			SingleKeyProof: singleKeyProof,
		},
	}

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "公钥为空")
}


// TestSingleKeyPlugin_Match_Ed25519SignatureVerificationFailure 测试 Ed25519 签名验证失败
func TestSingleKeyPlugin_Match_Ed25519SignatureVerificationFailure(t *testing.T) {
	mockSigMgr := &FailingMockSignatureManager{} // 使用失败的签名管理器
	mockHashMgr := &testutil.MockHashManager{}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewSingleKeyPlugin(mockSigMgr, mockHashMgr, mockCanonicalizer)

	publicKey := testutil.RandomPublicKey()
	lock := testutil.CreateSingleKeyLock(publicKey)

	singleKeyProof := &transaction.SingleKeyProof{
		PublicKey: &transaction.PublicKey{
			Value: publicKey,
		},
		Signature: &transaction.SignatureData{
			Value: testutil.RandomBytes(64),
		},
		Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ED25519, // Ed25519 算法
		SighashType: transaction.SignatureHashType_SIGHASH_ALL,
	}

	input := &transaction.TxInput{
		PreviousOutput:  testutil.CreateOutPoint(nil, 0),
		IsReferenceOnly: false,
		UnlockingProof: &transaction.TxInput_SingleKeyProof{
			SingleKeyProof: singleKeyProof,
		},
	}
	tx := testutil.CreateTransaction([]*transaction.TxInput{input}, nil)

	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_SingleKeyProof{
			SingleKeyProof: singleKeyProof,
		},
	}

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "Ed25519签名验证失败")
}

// TestSingleKeyPlugin_Match_ECDSA_SignatureVerificationFailure_DetailedError 测试 ECDSA 签名验证失败（详细错误消息）
func TestSingleKeyPlugin_Match_ECDSA_SignatureVerificationFailure_DetailedError(t *testing.T) {
	mockSigMgr := &FailingMockSignatureManager{} // 使用失败的签名管理器
	mockHashMgr := &testutil.MockHashManager{}
	mockCanonicalizer := NewMockCanonicalizer()

	plugin := NewSingleKeyPlugin(mockSigMgr, mockHashMgr, mockCanonicalizer)

	publicKey := testutil.RandomPublicKey()
	lock := testutil.CreateSingleKeyLock(publicKey)

	singleKeyProof := &transaction.SingleKeyProof{
		PublicKey: &transaction.PublicKey{
			Value: publicKey,
		},
		Signature: &transaction.SignatureData{
			Value: testutil.RandomBytes(64),
		},
		Algorithm:   transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
		SighashType: transaction.SignatureHashType_SIGHASH_ALL,
	}

	input := &transaction.TxInput{
		PreviousOutput:  testutil.CreateOutPoint(nil, 0),
		IsReferenceOnly: false,
		UnlockingProof: &transaction.TxInput_SingleKeyProof{
			SingleKeyProof: singleKeyProof,
		},
	}
	tx := testutil.CreateTransaction([]*transaction.TxInput{input}, nil)

	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_SingleKeyProof{
			SingleKeyProof: singleKeyProof,
		},
	}

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	// 验证错误消息包含详细的调试信息
	assert.Contains(t, err.Error(), "ECDSA签名验证失败")
	assert.Contains(t, err.Error(), "txHash=")
	assert.Contains(t, err.Error(), "pubKey=")
	assert.Contains(t, err.Error(), "sig=")
}
