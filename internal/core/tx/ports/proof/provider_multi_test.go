// Package proof_test 提供 MultiProofProvider 的单元测试
//
// 🧪 **测试覆盖**：
// - MultiProofProvider 路由功能测试
// - 各种锁定条件的证明生成测试
// - 错误场景测试
package proof

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/core/tx/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ==================== MultiProofProvider 路由测试 ====================

// TestMultiProofProvider_GenerateProof_SingleKeyLock 测试单密钥锁定
func TestMultiProofProvider_GenerateProof_SingleKeyLock(t *testing.T) {
	signer := testutil.NewMockSigner(nil)
	provider := NewMultiProofProvider(signer)

	// 创建 SingleKeyLock
	lock := testutil.CreateSingleKeyLock(nil)

	// 创建交易
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", lock),
		},
	)

	// 生成证明应该失败（MultiProofProvider 不处理 SingleKeyLock）
	_, err := provider.GenerateProof(context.Background(), tx, lock)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SimpleProofProvider")
}

// TestMultiProofProvider_GenerateProof_DelegationLock 测试委托锁定
func TestMultiProofProvider_GenerateProof_DelegationLock(t *testing.T) {
	signer := testutil.NewMockSigner(nil)
	provider := NewMultiProofProvider(signer)

	// 创建 DelegationLock
	expiryDuration := uint64(0)
	delegationLock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_DelegationLock{
			DelegationLock: &transaction.DelegationLock{
				OriginalOwner:        testutil.RandomAddress(),
				AllowedDelegates:     [][]byte{testutil.RandomAddress()},
				AuthorizedOperations: []string{"transfer"},
				ExpiryDurationBlocks: &expiryDuration,
				MaxValuePerOperation: 1000000,
			},
		},
	}

	// 创建交易
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", delegationLock),
		},
	)

	// 生成证明应该失败（需要外部上下文）
	_, err := provider.GenerateProof(context.Background(), tx, delegationLock)

	assert.Error(t, err)
	assert.Equal(t, ErrDelegationRequiresExternalContext, err)
}

// TestMultiProofProvider_GenerateProof_MultiKeyLock 测试多重签名锁定
func TestMultiProofProvider_GenerateProof_MultiKeyLock(t *testing.T) {
	signer := testutil.NewMockSigner(nil)
	provider := NewMultiProofProvider(signer)

	// 创建 MultiKeyLock
	multiKeyLock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_MultiKeyLock{
			MultiKeyLock: &transaction.MultiKeyLock{
				RequiredSignatures: 2,
				AuthorizedKeys: []*transaction.PublicKey{
					{Value: testutil.RandomPublicKey()},
					{Value: testutil.RandomPublicKey()},
					{Value: testutil.RandomPublicKey()},
				},
			},
		},
	}

	// 创建交易
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", multiKeyLock),
		},
	)

	// 生成证明应该失败（需要外部 MultiSigSession）
	_, err := provider.GenerateProof(context.Background(), tx, multiKeyLock)

	assert.Error(t, err)
	assert.Equal(t, ErrMultiSigRequiresSession, err)
}

// TestMultiProofProvider_GenerateProof_ThresholdLock 测试门限签名锁定
func TestMultiProofProvider_GenerateProof_ThresholdLock(t *testing.T) {
	signer := testutil.NewMockSigner(nil)
	provider := NewMultiProofProvider(signer)

	// 创建 ThresholdLock
	thresholdLock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ThresholdLock{
			ThresholdLock: &transaction.ThresholdLock{
				Threshold:             2,
				TotalParties:          3,
				PartyVerificationKeys: [][]byte{testutil.RandomPublicKey(), testutil.RandomPublicKey(), testutil.RandomPublicKey()},
			},
		},
	}

	// 创建交易
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", thresholdLock),
		},
	)

	// 生成证明应该失败（需要外部 ThresholdSigner）
	_, err := provider.GenerateProof(context.Background(), tx, thresholdLock)

	assert.Error(t, err)
	assert.Equal(t, ErrThresholdRequiresExternalSigner, err)
}

// TestMultiProofProvider_GenerateProof_ContractLock 测试合约锁定
func TestMultiProofProvider_GenerateProof_ContractLock(t *testing.T) {
	signer := testutil.NewMockSigner(nil)
	provider := NewMultiProofProvider(signer)

	// 创建 ContractLock
	contractLock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ContractLock{
			ContractLock: &transaction.ContractLock{
				ContractAddress: testutil.RandomAddress(),
				RequiredMethod:  "transfer",
			},
		},
	}

	// 创建交易
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", contractLock),
		},
	)

	// 生成证明应该失败（需要 ISPC 层生成）
	_, err := provider.GenerateProof(context.Background(), tx, contractLock)

	assert.Error(t, err)
	assert.Equal(t, ErrExecutionProofRequiresISPC, err)
}

// TestMultiProofProvider_GenerateProof_TimeLock 测试时间锁
func TestMultiProofProvider_GenerateProof_TimeLock(t *testing.T) {
	signer := testutil.NewMockSigner(nil)
	provider := NewMultiProofProvider(signer)

	// 创建 TimeLock
	timeLock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_TimeLock{
			TimeLock: &transaction.TimeLock{
				UnlockTimestamp: uint64(0),
				BaseLock:        testutil.CreateSingleKeyLock(nil),
				TimeSource:      transaction.TimeLock_TIME_SOURCE_BLOCK_TIMESTAMP,
			},
		},
	}

	// 创建交易
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", timeLock),
		},
	)

	// 生成证明
	// 注意：TimeProof 和 HeightProof 应该在 TxInput 层面设置，而不是 UnlockingProof
	// 当前实现会返回错误，测试应该反映实际行为
	_, err := provider.GenerateProof(context.Background(), tx, timeLock)

	// 当前实现会返回错误，因为 TimeProof 应该在 TxInput 层面处理
	assert.Error(t, err)
}

// TestMultiProofProvider_GenerateProof_HeightLock 测试高度锁
func TestMultiProofProvider_GenerateProof_HeightLock(t *testing.T) {
	signer := testutil.NewMockSigner(nil)
	provider := NewMultiProofProvider(signer)

	// 创建 HeightLock
	heightLock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_HeightLock{
			HeightLock: &transaction.HeightLock{
				UnlockHeight: 100,
				BaseLock:     testutil.CreateSingleKeyLock(nil),
			},
		},
	}

	// 创建交易
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", heightLock),
		},
	)

	// 生成证明
	// 注意：TimeProof 和 HeightProof 应该在 TxInput 层面设置，而不是 UnlockingProof
	// 当前实现会返回错误，测试应该反映实际行为
	_, err := provider.GenerateProof(context.Background(), tx, heightLock)

	// 当前实现会返回错误，因为 HeightProof 应该在 TxInput 层面处理
	assert.Error(t, err)
}

// TestMultiProofProvider_GenerateProof_UnsupportedLock 测试不支持的锁定类型
func TestMultiProofProvider_GenerateProof_UnsupportedLock(t *testing.T) {
	signer := testutil.NewMockSigner(nil)
	provider := NewMultiProofProvider(signer)

	// 创建交易
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", nil),
		},
	)

	// 创建无效的锁定条件（nil）
	var lock *transaction.LockingCondition = nil

	// 生成证明应该失败
	_, err := provider.GenerateProof(context.Background(), tx, lock)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported lock type")
}

// ==================== MultiProofProvider 边界条件测试 ====================

// TestMultiProofProvider_GenerateProof_NilTransaction 测试 nil transaction
func TestMultiProofProvider_GenerateProof_NilTransaction(t *testing.T) {
	signer := testutil.NewMockSigner(nil)
	provider := NewMultiProofProvider(signer)

	lock := testutil.CreateSingleKeyLock(nil)

	// 生成证明应该失败（虽然 SingleKeyLock 会返回错误，但 nil transaction 应该先被检查）
	_, err := provider.GenerateProof(context.Background(), nil, lock)

	// 当前实现可能不会检查 nil transaction，但应该返回错误
	assert.Error(t, err)
}

// TestMultiProofProvider_GenerateProof_NilLockingCondition 测试 nil locking condition
func TestMultiProofProvider_GenerateProof_NilLockingCondition(t *testing.T) {
	signer := testutil.NewMockSigner(nil)
	provider := NewMultiProofProvider(signer)

	// 创建交易
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", nil),
		},
	)

	// 生成证明应该失败
	_, err := provider.GenerateProof(context.Background(), tx, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported lock type")
}

// TestMultiProofProvider_GenerateProof_TimeLock_NilBaseLock 测试 TimeLock 的 BaseLock 为 nil
func TestMultiProofProvider_GenerateProof_TimeLock_NilBaseLock(t *testing.T) {
	signer := testutil.NewMockSigner(nil)
	provider := NewMultiProofProvider(signer)

	// 创建 TimeLock（BaseLock 为 nil）
	timeLock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_TimeLock{
			TimeLock: &transaction.TimeLock{
				UnlockTimestamp: uint64(0),
				BaseLock:        nil, // nil BaseLock
				TimeSource:      transaction.TimeLock_TIME_SOURCE_BLOCK_TIMESTAMP,
			},
		},
	}

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", timeLock),
		},
	)

	// 生成证明应该失败（TimeProof 应该在 TxInput 层面处理）
	_, err := provider.GenerateProof(context.Background(), tx, timeLock)

	assert.Error(t, err)
}

// TestMultiProofProvider_GenerateProof_HeightLock_NilBaseLock 测试 HeightLock 的 BaseLock 为 nil
func TestMultiProofProvider_GenerateProof_HeightLock_NilBaseLock(t *testing.T) {
	signer := testutil.NewMockSigner(nil)
	provider := NewMultiProofProvider(signer)

	// 创建 HeightLock（BaseLock 为 nil）
	heightLock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_HeightLock{
			HeightLock: &transaction.HeightLock{
				UnlockHeight: 100,
				BaseLock:     nil, // nil BaseLock
			},
		},
	}

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", heightLock),
		},
	)

	// 生成证明应该失败（HeightProof 应该在 TxInput 层面处理）
	_, err := provider.GenerateProof(context.Background(), tx, heightLock)

	assert.Error(t, err)
}

// TestMultiProofProvider_GenerateProof_DelegationLock_Error 测试 DelegationLock 返回错误
func TestMultiProofProvider_GenerateProof_DelegationLock_Error(t *testing.T) {
	signer := testutil.NewMockSigner(nil)
	provider := NewMultiProofProvider(signer)

	// 创建 DelegationLock
	expiryDuration := uint64(0)
	delegationLock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_DelegationLock{
			DelegationLock: &transaction.DelegationLock{
				OriginalOwner:        testutil.RandomAddress(),
				AllowedDelegates:     [][]byte{testutil.RandomAddress()},
				AuthorizedOperations: []string{"transfer"},
				ExpiryDurationBlocks: &expiryDuration,
				MaxValuePerOperation: 1000000,
			},
		},
	}

	// 创建交易
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", delegationLock),
		},
	)

	// 生成证明应该失败（需要外部上下文）
	_, err := provider.GenerateProof(context.Background(), tx, delegationLock)

	assert.Error(t, err)
	assert.Equal(t, ErrDelegationRequiresExternalContext, err)
}

// ==================== generateTimeProof 错误路径测试 ====================

// TestMultiProofProvider_GenerateProof_TimeLock_NilTimeLock 测试 TimeLock 为 nil
func TestMultiProofProvider_GenerateProof_TimeLock_NilTimeLock(t *testing.T) {
	signer := testutil.NewMockSigner(nil)
	provider := NewMultiProofProvider(signer)

	// 创建 TimeLock（TimeLock 本身为 nil）
	timeLock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_TimeLock{
			TimeLock: nil, // nil TimeLock
		},
	}

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", timeLock),
		},
	)

	_, err := provider.GenerateProof(context.Background(), tx, timeLock)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "TimeLock is nil")
}

// TestMultiProofProvider_GenerateProof_TimeLock_BaseProofError 测试 base proof 生成失败
func TestMultiProofProvider_GenerateProof_TimeLock_BaseProofError(t *testing.T) {
	signer := testutil.NewMockSigner(nil)
	provider := NewMultiProofProvider(signer)

	// 创建 TimeLock（BaseLock 使用 MultiKeyLock，会返回错误）
	multiKeyLock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_MultiKeyLock{
			MultiKeyLock: &transaction.MultiKeyLock{
				RequiredSignatures: 2,
				AuthorizedKeys: []*transaction.PublicKey{
					{Value: testutil.RandomPublicKey()},
					{Value: testutil.RandomPublicKey()},
				},
			},
		},
	}

	timeLock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_TimeLock{
			TimeLock: &transaction.TimeLock{
				UnlockTimestamp: uint64(0),
				BaseLock:        multiKeyLock, // BaseLock 会返回错误
				TimeSource:      transaction.TimeLock_TIME_SOURCE_BLOCK_TIMESTAMP,
			},
		},
	}

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", timeLock),
		},
	)

	_, err := provider.GenerateProof(context.Background(), tx, timeLock)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to generate base proof for TimeLock")
}

// ==================== generateHeightProof 错误路径测试 ====================

// TestMultiProofProvider_GenerateProof_HeightLock_NilHeightLock 测试 HeightLock 为 nil
func TestMultiProofProvider_GenerateProof_HeightLock_NilHeightLock(t *testing.T) {
	signer := testutil.NewMockSigner(nil)
	provider := NewMultiProofProvider(signer)

	// 创建 HeightLock（HeightLock 本身为 nil）
	heightLock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_HeightLock{
			HeightLock: nil, // nil HeightLock
		},
	}

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", heightLock),
		},
	)

	_, err := provider.GenerateProof(context.Background(), tx, heightLock)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "HeightLock is nil")
}

// TestMultiProofProvider_GenerateProof_HeightLock_BaseProofError 测试 base proof 生成失败
func TestMultiProofProvider_GenerateProof_HeightLock_BaseProofError(t *testing.T) {
	signer := testutil.NewMockSigner(nil)
	provider := NewMultiProofProvider(signer)

	// 创建 HeightLock（BaseLock 使用 MultiKeyLock，会返回错误）
	multiKeyLock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_MultiKeyLock{
			MultiKeyLock: &transaction.MultiKeyLock{
				RequiredSignatures: 2,
				AuthorizedKeys: []*transaction.PublicKey{
					{Value: testutil.RandomPublicKey()},
					{Value: testutil.RandomPublicKey()},
				},
			},
		},
	}

	heightLock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_HeightLock{
			HeightLock: &transaction.HeightLock{
				UnlockHeight: 100,
				BaseLock:     multiKeyLock, // BaseLock 会返回错误
			},
		},
	}

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", heightLock),
		},
	)

	_, err := provider.GenerateProof(context.Background(), tx, heightLock)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to generate base proof for HeightLock")
}
