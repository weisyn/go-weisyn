// Package hash_test 提供 Hash Canonicalizer 的单元测试
//
// 🧪 **测试覆盖**：
// - Canonicalizer 核心功能测试
// - 交易哈希计算测试
// - 签名哈希计算测试
// - 边界条件和错误场景测试
package hash

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"github.com/weisyn/v1/internal/core/tx/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ==================== Canonicalizer 核心功能测试 ====================

// TestNewCanonicalizer 测试创建 Canonicalizer
func TestNewCanonicalizer(t *testing.T) {
	// 创建模拟的 TransactionHashServiceClient
	mockClient := &MockTransactionHashServiceClient{}

	canonicalizer := NewCanonicalizer(mockClient)

	assert.NotNil(t, canonicalizer)
	assert.NotNil(t, canonicalizer.txHashClient)
}

// TestCanonicalizer_ComputeTransactionHash 测试计算交易哈希
func TestCanonicalizer_ComputeTransactionHash(t *testing.T) {
	mockClient := &MockTransactionHashServiceClient{
		txHash: []byte("mock-tx-hash"),
	}

	canonicalizer := NewCanonicalizer(mockClient)

	tx := testutil.CreateTransaction(
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

	hash, err := canonicalizer.ComputeTransactionHash(context.Background(), tx)

	assert.NoError(t, err)
	assert.NotNil(t, hash)
	assert.Equal(t, []byte("mock-tx-hash"), hash)
}

// TestCanonicalizer_ComputeSignatureHash 测试计算签名哈希
func TestCanonicalizer_ComputeSignatureHash(t *testing.T) {
	mockClient := &MockTransactionHashServiceClient{
		sigHash: []byte("mock-sig-hash"),
	}

	canonicalizer := NewCanonicalizer(mockClient)

	tx := testutil.CreateTransaction(
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

	hash, err := canonicalizer.ComputeSignatureHash(
		context.Background(),
		tx,
		0, // inputIndex
		transaction.SignatureHashType_SIGHASH_ALL,
	)

	assert.NoError(t, err)
	assert.NotNil(t, hash)
	assert.Equal(t, []byte("mock-sig-hash"), hash)
}

// ==================== ComputeTransactionHash 边界条件测试 ====================

// TestCanonicalizer_ComputeTransactionHash_NilTransaction 测试 nil transaction
func TestCanonicalizer_ComputeTransactionHash_NilTransaction(t *testing.T) {
	mockClient := &MockTransactionHashServiceClient{}
	canonicalizer := NewCanonicalizer(mockClient)

	hash, err := canonicalizer.ComputeTransactionHash(context.Background(), nil)

	assert.Error(t, err)
	assert.Nil(t, hash)
	assert.Equal(t, ErrInvalidTransaction, err)
}

// TestCanonicalizer_ComputeTransactionHash_NilClient 测试 nil client
func TestCanonicalizer_ComputeTransactionHash_NilClient(t *testing.T) {
	canonicalizer := NewCanonicalizer(nil)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{},
		[]*transaction.TxOutput{},
	)

	hash, err := canonicalizer.ComputeTransactionHash(context.Background(), tx)

	assert.Error(t, err)
	assert.Nil(t, hash)
	assert.Contains(t, err.Error(), "not initialized")
}

// TestCanonicalizer_ComputeTransactionHash_ClientError 测试 gRPC 调用失败
func TestCanonicalizer_ComputeTransactionHash_ClientError(t *testing.T) {
	mockClient := &ErrorMockTransactionHashServiceClient{
		computeHashError: fmt.Errorf("gRPC error"),
	}
	canonicalizer := NewCanonicalizer(mockClient)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{},
		[]*transaction.TxOutput{},
	)

	hash, err := canonicalizer.ComputeTransactionHash(context.Background(), tx)

	assert.Error(t, err)
	assert.Nil(t, hash)
	assert.Contains(t, err.Error(), "canonical serialization failed")
}

// TestCanonicalizer_ComputeTransactionHash_InvalidResponse 测试 IsValid=false
func TestCanonicalizer_ComputeTransactionHash_InvalidResponse(t *testing.T) {
	mockClient := &InvalidResponseMockTransactionHashServiceClient{}
	canonicalizer := NewCanonicalizer(mockClient)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{},
		[]*transaction.TxOutput{},
	)

	hash, err := canonicalizer.ComputeTransactionHash(context.Background(), tx)

	assert.Error(t, err)
	assert.Nil(t, hash)
	assert.Equal(t, ErrInvalidTransaction, err)
}

// TestCanonicalizer_ComputeTransactionHash_EmptyTransaction 测试空交易
func TestCanonicalizer_ComputeTransactionHash_EmptyTransaction(t *testing.T) {
	mockClient := &MockTransactionHashServiceClient{
		txHash: testutil.RandomTxID(),
	}
	canonicalizer := NewCanonicalizer(mockClient)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{},
		[]*transaction.TxOutput{},
	)

	hash, err := canonicalizer.ComputeTransactionHash(context.Background(), tx)

	assert.NoError(t, err)
	assert.NotNil(t, hash)
}

// TestCanonicalizer_ComputeTransactionHash_ComplexTransaction 测试复杂交易
func TestCanonicalizer_ComputeTransactionHash_ComplexTransaction(t *testing.T) {
	mockClient := &MockTransactionHashServiceClient{
		txHash: testutil.RandomTxID(),
	}
	canonicalizer := NewCanonicalizer(mockClient)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{PreviousOutput: testutil.CreateOutPoint(nil, 0), IsReferenceOnly: false},
			{PreviousOutput: testutil.CreateOutPoint(nil, 1), IsReferenceOnly: false},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil)),
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "2000", testutil.CreateSingleKeyLock(nil)),
		},
	)

	hash, err := canonicalizer.ComputeTransactionHash(context.Background(), tx)

	assert.NoError(t, err)
	assert.NotNil(t, hash)
}

// ==================== ComputeSignatureHash 边界条件测试 ====================

// TestCanonicalizer_ComputeSignatureHash_NilTransaction 测试 nil transaction
func TestCanonicalizer_ComputeSignatureHash_NilTransaction(t *testing.T) {
	mockClient := &MockTransactionHashServiceClient{}
	canonicalizer := NewCanonicalizer(mockClient)

	hash, err := canonicalizer.ComputeSignatureHash(
		context.Background(),
		nil,
		0,
		transaction.SignatureHashType_SIGHASH_ALL,
	)

	assert.Error(t, err)
	assert.Nil(t, hash)
	assert.Equal(t, ErrInvalidTransaction, err)
}

// TestCanonicalizer_ComputeSignatureHash_NilClient 测试 nil client
func TestCanonicalizer_ComputeSignatureHash_NilClient(t *testing.T) {
	canonicalizer := NewCanonicalizer(nil)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{PreviousOutput: testutil.CreateOutPoint(nil, 0), IsReferenceOnly: false},
		},
		[]*transaction.TxOutput{},
	)

	hash, err := canonicalizer.ComputeSignatureHash(
		context.Background(),
		tx,
		0,
		transaction.SignatureHashType_SIGHASH_ALL,
	)

	assert.Error(t, err)
	assert.Nil(t, hash)
	assert.Contains(t, err.Error(), "not initialized")
}

// TestCanonicalizer_ComputeSignatureHash_NegativeInputIndex 测试负数 inputIndex
func TestCanonicalizer_ComputeSignatureHash_NegativeInputIndex(t *testing.T) {
	mockClient := &MockTransactionHashServiceClient{}
	canonicalizer := NewCanonicalizer(mockClient)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{PreviousOutput: testutil.CreateOutPoint(nil, 0), IsReferenceOnly: false},
		},
		[]*transaction.TxOutput{},
	)

	hash, err := canonicalizer.ComputeSignatureHash(
		context.Background(),
		tx,
		-1, // 负数索引
		transaction.SignatureHashType_SIGHASH_ALL,
	)

	assert.Error(t, err)
	assert.Nil(t, hash)
	assert.Equal(t, ErrInputIndexOutOfRange, err)
}

// TestCanonicalizer_ComputeSignatureHash_InputIndexOutOfRange 测试超出范围的 inputIndex
func TestCanonicalizer_ComputeSignatureHash_InputIndexOutOfRange(t *testing.T) {
	mockClient := &MockTransactionHashServiceClient{}
	canonicalizer := NewCanonicalizer(mockClient)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{PreviousOutput: testutil.CreateOutPoint(nil, 0), IsReferenceOnly: false},
		},
		[]*transaction.TxOutput{},
	)

	hash, err := canonicalizer.ComputeSignatureHash(
		context.Background(),
		tx,
		10, // 超出范围
		transaction.SignatureHashType_SIGHASH_ALL,
	)

	assert.Error(t, err)
	assert.Nil(t, hash)
	assert.Equal(t, ErrInputIndexOutOfRange, err)
}

// TestCanonicalizer_ComputeSignatureHash_ClientError 测试 gRPC 调用失败
func TestCanonicalizer_ComputeSignatureHash_ClientError(t *testing.T) {
	mockClient := &ErrorMockTransactionHashServiceClient{
		computeSignatureHashError: fmt.Errorf("gRPC error"),
	}
	canonicalizer := NewCanonicalizer(mockClient)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{PreviousOutput: testutil.CreateOutPoint(nil, 0), IsReferenceOnly: false},
		},
		[]*transaction.TxOutput{},
	)

	hash, err := canonicalizer.ComputeSignatureHash(
		context.Background(),
		tx,
		0,
		transaction.SignatureHashType_SIGHASH_ALL,
	)

	assert.Error(t, err)
	assert.Nil(t, hash)
	assert.Contains(t, err.Error(), "canonical serialization failed")
}

// TestCanonicalizer_ComputeSignatureHash_InvalidResponse 测试 IsValid=false
func TestCanonicalizer_ComputeSignatureHash_InvalidResponse(t *testing.T) {
	mockClient := &InvalidResponseMockTransactionHashServiceClient{}
	canonicalizer := NewCanonicalizer(mockClient)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{PreviousOutput: testutil.CreateOutPoint(nil, 0), IsReferenceOnly: false},
		},
		[]*transaction.TxOutput{},
	)

	hash, err := canonicalizer.ComputeSignatureHash(
		context.Background(),
		tx,
		0,
		transaction.SignatureHashType_SIGHASH_ALL,
	)

	assert.Error(t, err)
	assert.Nil(t, hash)
	assert.Equal(t, ErrInvalidTransaction, err)
}

// TestCanonicalizer_ComputeSignatureHash_DifferentSighashTypes 测试不同的 SIGHASH 类型
func TestCanonicalizer_ComputeSignatureHash_DifferentSighashTypes(t *testing.T) {
	mockClient := &MockTransactionHashServiceClient{
		sigHash: testutil.RandomTxID(),
	}
	canonicalizer := NewCanonicalizer(mockClient)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{PreviousOutput: testutil.CreateOutPoint(nil, 0), IsReferenceOnly: false},
		},
		[]*transaction.TxOutput{},
	)

	sighashTypes := []transaction.SignatureHashType{
		transaction.SignatureHashType_SIGHASH_ALL,
		transaction.SignatureHashType_SIGHASH_NONE,
		transaction.SignatureHashType_SIGHASH_SINGLE,
		transaction.SignatureHashType_SIGHASH_ALL_ANYONECANPAY,
		transaction.SignatureHashType_SIGHASH_NONE_ANYONECANPAY,
		transaction.SignatureHashType_SIGHASH_SINGLE_ANYONECANPAY,
	}

	for _, sighashType := range sighashTypes {
		hash, err := canonicalizer.ComputeSignatureHash(
			context.Background(),
			tx,
			0,
			sighashType,
		)

		assert.NoError(t, err, "SIGHASH type: %v", sighashType)
		assert.NotNil(t, hash, "SIGHASH type: %v", sighashType)
	}
}

// TestCanonicalizer_ComputeSignatureHash_MultipleInputs 测试多个输入
func TestCanonicalizer_ComputeSignatureHash_MultipleInputs(t *testing.T) {
	mockClient := &MockTransactionHashServiceClient{
		sigHash: testutil.RandomTxID(),
	}
	canonicalizer := NewCanonicalizer(mockClient)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{PreviousOutput: testutil.CreateOutPoint(nil, 0), IsReferenceOnly: false},
			{PreviousOutput: testutil.CreateOutPoint(nil, 1), IsReferenceOnly: false},
			{PreviousOutput: testutil.CreateOutPoint(nil, 2), IsReferenceOnly: false},
		},
		[]*transaction.TxOutput{},
	)

	// 测试每个输入的签名哈希
	for i := 0; i < len(tx.Inputs); i++ {
		hash, err := canonicalizer.ComputeSignatureHash(
			context.Background(),
			tx,
			i,
			transaction.SignatureHashType_SIGHASH_ALL,
		)

		assert.NoError(t, err, "Input index: %d", i)
		assert.NotNil(t, hash, "Input index: %d", i)
	}
}

// ==================== ComputeSignatureHashForVerification 测试 ====================

// TestCanonicalizer_ComputeSignatureHashForVerification 测试计算签名哈希（用于验证）
func TestCanonicalizer_ComputeSignatureHashForVerification(t *testing.T) {
	mockClient := &MockTransactionHashServiceClient{
		sigHash: []byte("mock-sig-hash"),
	}
	canonicalizer := NewCanonicalizer(mockClient)

	tx := testutil.CreateTransaction(
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

	hash, err := canonicalizer.ComputeSignatureHashForVerification(
		context.Background(),
		tx,
		0,
		transaction.SignatureHashType_SIGHASH_ALL,
	)

	assert.NoError(t, err)
	assert.NotNil(t, hash)
	assert.Equal(t, []byte("mock-sig-hash"), hash)
}

// TestCanonicalizer_ComputeSignatureHashForVerification_SameAsComputeSignatureHash 测试验证哈希与签名哈希相同
func TestCanonicalizer_ComputeSignatureHashForVerification_SameAsComputeSignatureHash(t *testing.T) {
	mockClient := &MockTransactionHashServiceClient{
		sigHash: testutil.RandomTxID(),
	}
	canonicalizer := NewCanonicalizer(mockClient)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{PreviousOutput: testutil.CreateOutPoint(nil, 0), IsReferenceOnly: false},
		},
		[]*transaction.TxOutput{},
	)

	sigHash, err1 := canonicalizer.ComputeSignatureHash(
		context.Background(),
		tx,
		0,
		transaction.SignatureHashType_SIGHASH_ALL,
	)

	verifyHash, err2 := canonicalizer.ComputeSignatureHashForVerification(
		context.Background(),
		tx,
		0,
		transaction.SignatureHashType_SIGHASH_ALL,
	)

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.Equal(t, sigHash, verifyHash) // 应该相同
}

// ==================== 上下文和并发测试 ====================

// TestCanonicalizer_ComputeTransactionHash_ContextCanceled 测试上下文取消
func TestCanonicalizer_ComputeTransactionHash_ContextCanceled(t *testing.T) {
	mockClient := &ErrorMockTransactionHashServiceClient{
		computeHashError: context.Canceled,
	}
	canonicalizer := NewCanonicalizer(mockClient)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{},
		[]*transaction.TxOutput{},
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	hash, err := canonicalizer.ComputeTransactionHash(ctx, tx)

	assert.Error(t, err)
	assert.Nil(t, hash)
}

// TestCanonicalizer_ComputeSignatureHash_ContextCanceled 测试上下文取消
func TestCanonicalizer_ComputeSignatureHash_ContextCanceled(t *testing.T) {
	mockClient := &ErrorMockTransactionHashServiceClient{
		computeSignatureHashError: context.Canceled,
	}
	canonicalizer := NewCanonicalizer(mockClient)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{PreviousOutput: testutil.CreateOutPoint(nil, 0), IsReferenceOnly: false},
		},
		[]*transaction.TxOutput{},
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	hash, err := canonicalizer.ComputeSignatureHash(
		ctx,
		tx,
		0,
		transaction.SignatureHashType_SIGHASH_ALL,
	)

	assert.Error(t, err)
	assert.Nil(t, hash)
}

// ==================== Mock 辅助类型 ====================

// MockTransactionHashServiceClient 模拟 TransactionHashServiceClient
type MockTransactionHashServiceClient struct {
	txHash  []byte
	sigHash []byte
}

func (m *MockTransactionHashServiceClient) ComputeHash(ctx context.Context, req *transaction.ComputeHashRequest, opts ...grpc.CallOption) (*transaction.ComputeHashResponse, error) {
	if m.txHash == nil {
		m.txHash = testutil.RandomTxID()
	}
	return &transaction.ComputeHashResponse{
		Hash:    m.txHash,
		IsValid: true, // 默认有效
	}, nil
}

func (m *MockTransactionHashServiceClient) ValidateHash(ctx context.Context, req *transaction.ValidateHashRequest, opts ...grpc.CallOption) (*transaction.ValidateHashResponse, error) {
	return &transaction.ValidateHashResponse{
		IsValid: true,
	}, nil
}

func (m *MockTransactionHashServiceClient) ComputeSignatureHash(ctx context.Context, req *transaction.ComputeSignatureHashRequest, opts ...grpc.CallOption) (*transaction.ComputeSignatureHashResponse, error) {
	if m.sigHash == nil {
		m.sigHash = testutil.RandomTxID()
	}
	return &transaction.ComputeSignatureHashResponse{
		Hash:    m.sigHash,
		IsValid: true, // 默认有效
	}, nil
}

func (m *MockTransactionHashServiceClient) ValidateSignatureHash(ctx context.Context, req *transaction.ValidateSignatureHashRequest, opts ...grpc.CallOption) (*transaction.ValidateSignatureHashResponse, error) {
	return &transaction.ValidateSignatureHashResponse{
		IsValid: true,
	}, nil
}

// ErrorMockTransactionHashServiceClient 返回错误的模拟客户端
type ErrorMockTransactionHashServiceClient struct {
	computeHashError          error
	computeSignatureHashError error
}

func (m *ErrorMockTransactionHashServiceClient) ComputeHash(ctx context.Context, req *transaction.ComputeHashRequest, opts ...grpc.CallOption) (*transaction.ComputeHashResponse, error) {
	if m.computeHashError != nil {
		return nil, m.computeHashError
	}
	return &transaction.ComputeHashResponse{
		Hash:    testutil.RandomTxID(),
		IsValid: true,
	}, nil
}

func (m *ErrorMockTransactionHashServiceClient) ValidateHash(ctx context.Context, req *transaction.ValidateHashRequest, opts ...grpc.CallOption) (*transaction.ValidateHashResponse, error) {
	return &transaction.ValidateHashResponse{
		IsValid: true,
	}, nil
}

func (m *ErrorMockTransactionHashServiceClient) ComputeSignatureHash(ctx context.Context, req *transaction.ComputeSignatureHashRequest, opts ...grpc.CallOption) (*transaction.ComputeSignatureHashResponse, error) {
	if m.computeSignatureHashError != nil {
		return nil, m.computeSignatureHashError
	}
	return &transaction.ComputeSignatureHashResponse{
		Hash:    testutil.RandomTxID(),
		IsValid: true,
	}, nil
}

func (m *ErrorMockTransactionHashServiceClient) ValidateSignatureHash(ctx context.Context, req *transaction.ValidateSignatureHashRequest, opts ...grpc.CallOption) (*transaction.ValidateSignatureHashResponse, error) {
	return &transaction.ValidateSignatureHashResponse{
		IsValid: true,
	}, nil
}

// InvalidResponseMockTransactionHashServiceClient 返回 IsValid=false 的模拟客户端
type InvalidResponseMockTransactionHashServiceClient struct{}

func (m *InvalidResponseMockTransactionHashServiceClient) ComputeHash(ctx context.Context, req *transaction.ComputeHashRequest, opts ...grpc.CallOption) (*transaction.ComputeHashResponse, error) {
	return &transaction.ComputeHashResponse{
		Hash:    []byte("mock-hash"),
		IsValid: false,
	}, nil
}

func (m *InvalidResponseMockTransactionHashServiceClient) ValidateHash(ctx context.Context, req *transaction.ValidateHashRequest, opts ...grpc.CallOption) (*transaction.ValidateHashResponse, error) {
	return &transaction.ValidateHashResponse{
		IsValid: true,
	}, nil
}

func (m *InvalidResponseMockTransactionHashServiceClient) ComputeSignatureHash(ctx context.Context, req *transaction.ComputeSignatureHashRequest, opts ...grpc.CallOption) (*transaction.ComputeSignatureHashResponse, error) {
	return &transaction.ComputeSignatureHashResponse{
		Hash:    []byte("mock-sig-hash"),
		IsValid: false,
	}, nil
}

func (m *InvalidResponseMockTransactionHashServiceClient) ValidateSignatureHash(ctx context.Context, req *transaction.ValidateSignatureHashRequest, opts ...grpc.CallOption) (*transaction.ValidateSignatureHashResponse, error) {
	return &transaction.ValidateSignatureHashResponse{
		IsValid: true,
	}, nil
}
