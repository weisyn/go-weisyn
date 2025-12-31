// Package authz_test 提供 AuthZ 插件的测试 Mock 对象
//
// 🧪 **测试规范遵循**：
// - 统一管理 Mock 对象，避免重复定义
// - 遵循测试规范：docs/system/standards/principles/testing-standards.md
package authz

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"github.com/weisyn/v1/internal/core/tx/ports/hash"
	"github.com/weisyn/v1/internal/core/tx/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxopb "github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	txiface "github.com/weisyn/v1/pkg/interfaces/tx"
)

// NewMockCanonicalizer 创建模拟 Canonicalizer（返回 *hash.Canonicalizer）
func NewMockCanonicalizer() *hash.Canonicalizer {
	mockClient := &MockTransactionHashServiceClient{
		txHash:  testutil.RandomTxID(),
		sigHash: testutil.RandomTxID(),
	}
	return hash.NewCanonicalizer(mockClient)
}

// MockTransactionHashServiceClient 模拟 TransactionHashServiceClient
type MockTransactionHashServiceClient struct {
	txHash  []byte
	sigHash []byte
}

func (m *MockTransactionHashServiceClient) ComputeHash(ctx context.Context, req *transaction.ComputeHashRequest, opts ...grpc.CallOption) (*transaction.ComputeHashResponse, error) {
	if m.txHash != nil {
		return &transaction.ComputeHashResponse{
			Hash:    m.txHash,
			IsValid: true,
		}, nil
	}
	return &transaction.ComputeHashResponse{
		Hash:    testutil.RandomTxID(),
		IsValid: true,
	}, nil
}

func (m *MockTransactionHashServiceClient) ValidateHash(ctx context.Context, req *transaction.ValidateHashRequest, opts ...grpc.CallOption) (*transaction.ValidateHashResponse, error) {
	return &transaction.ValidateHashResponse{
		IsValid: true,
	}, nil
}

func (m *MockTransactionHashServiceClient) ComputeSignatureHash(ctx context.Context, req *transaction.ComputeSignatureHashRequest, opts ...grpc.CallOption) (*transaction.ComputeSignatureHashResponse, error) {
	if m.sigHash != nil {
		return &transaction.ComputeSignatureHashResponse{
			Hash:    m.sigHash,
			IsValid: true,
		}, nil
	}
	return &transaction.ComputeSignatureHashResponse{
		Hash:    testutil.RandomTxID(),
		IsValid: true,
	}, nil
}

func (m *MockTransactionHashServiceClient) ValidateSignatureHash(ctx context.Context, req *transaction.ValidateSignatureHashRequest, opts ...grpc.CallOption) (*transaction.ValidateSignatureHashResponse, error) {
	return &transaction.ValidateSignatureHashResponse{
		IsValid: true,
	}, nil
}

// ErrorMockTransactionHashServiceClient 模拟返回错误的 TransactionHashServiceClient
type ErrorMockTransactionHashServiceClient struct {
	computeHashError           error
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

// FailingMockSignatureManager 模拟签名验证失败的 SignatureManager
type FailingMockSignatureManager struct{}

func (m *FailingMockSignatureManager) Sign(data []byte, privateKey []byte) ([]byte, error) {
	return testutil.RandomBytes(64), nil
}

func (m *FailingMockSignatureManager) Verify(data, signature, publicKey []byte) bool {
	return false // 总是返回 false
}

func (m *FailingMockSignatureManager) RecoverPublicKey(hash []byte, signature []byte) ([]byte, error) {
	return testutil.RandomPublicKey(), nil
}

func (m *FailingMockSignatureManager) NormalizeSignature(signature []byte) ([]byte, error) {
	return signature, nil
}

func (m *FailingMockSignatureManager) SignTransaction(txHash []byte, privateKey []byte, sigHashType crypto.SignatureHashType) ([]byte, error) {
	return testutil.RandomBytes(64), nil
}

func (m *FailingMockSignatureManager) VerifyTransactionSignature(txHash []byte, signature []byte, publicKey []byte, sigHashType crypto.SignatureHashType) bool {
	return false // 总是返回 false
}

func (m *FailingMockSignatureManager) SignMessage(message []byte, privateKey []byte) ([]byte, error) {
	return testutil.RandomBytes(64), nil
}

func (m *FailingMockSignatureManager) VerifyMessage(message []byte, signature []byte, publicKey []byte) bool {
	return false // 总是返回 false
}

func (m *FailingMockSignatureManager) RecoverAddress(message []byte, signature []byte) (string, error) {
	return string(testutil.RandomAddress()), nil
}

func (m *FailingMockSignatureManager) SignBatch(messages [][]byte, privateKey []byte) ([][]byte, error) {
	result := make([][]byte, len(messages))
	for i := range messages {
		result[i] = testutil.RandomBytes(64)
	}
	return result, nil
}

func (m *FailingMockSignatureManager) VerifyBatch(dataList [][]byte, signatureList [][]byte, publicKeyList [][]byte) ([]bool, error) {
	result := make([]bool, len(dataList))
	// 全部返回 false
	return result, nil
}

func (m *FailingMockSignatureManager) ValidateSignature(signature []byte) error {
	return nil
}

// MockMultiSignatureVerifier 模拟 MultiSignatureVerifier
type MockMultiSignatureVerifier struct {
	shouldFail bool
}

func (m *MockMultiSignatureVerifier) VerifyMultiSignature(
	message []byte,
	signatures []crypto.MultiSignatureEntry,
	publicKeys []crypto.PublicKey,
	requiredSignatures uint32,
	algorithm crypto.SignatureAlgorithm,
) (bool, error) {
	if m.shouldFail {
		return false, fmt.Errorf("多重签名验证失败")
	}
	// 检查 key_index 是否在范围内
	for _, sig := range signatures {
		if sig.KeyIndex >= uint32(len(publicKeys)) {
			return false, fmt.Errorf("key_index %d 超出范围", sig.KeyIndex)
		}
	}
	return len(signatures) >= int(requiredSignatures), nil
}

// MockThresholdSignatureVerifier 模拟 ThresholdSignatureVerifier
type MockThresholdSignatureVerifier struct {
	shouldFail bool
}

func (m *MockThresholdSignatureVerifier) VerifyThresholdSignature(
	message []byte,
	combinedSignature []byte,
	shares []*transaction.ThresholdProof_ThresholdSignatureShare,
	partyPublicKeys []byte,
	threshold uint32,
	totalParties uint32,
	scheme string,
) (bool, error) {
	if m.shouldFail {
		return false, fmt.Errorf("门限签名验证失败")
	}
	return true, nil
}

func (m *MockThresholdSignatureVerifier) VerifySignatureShare(
	message []byte,
	share *transaction.ThresholdProof_ThresholdSignatureShare,
	partyPublicKey []byte,
	scheme string,
) (bool, error) {
	if m.shouldFail {
		return false, fmt.Errorf("签名份额验证失败")
	}
	return true, nil
}

// MockVerifierEnvironment 模拟 VerifierEnvironment
type MockVerifierEnvironment struct {
	blockHeight      uint64
	txBlockHeight    uint64
	txBlockHeightErr error
	publicKey        []byte
	publicKeyErr     error
}

func (m *MockVerifierEnvironment) GetBlockHeight() uint64 {
	return m.blockHeight
}

func (m *MockVerifierEnvironment) GetBlockTime() uint64 {
	return 0
}

func (m *MockVerifierEnvironment) GetMinerAddress() []byte {
	return nil
}

func (m *MockVerifierEnvironment) GetChainID() []byte {
	return nil
}

func (m *MockVerifierEnvironment) GetExpectedFees() *txiface.AggregatedFees {
	return nil
}

func (m *MockVerifierEnvironment) GetUTXO(ctx context.Context, outpoint *transaction.OutPoint) (*utxopb.UTXO, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockVerifierEnvironment) IsCoinbase(tx *transaction.Transaction) bool {
	return false
}

func (m *MockVerifierEnvironment) GetNonce(ctx context.Context, address []byte) (uint64, error) {
	return 0, fmt.Errorf("not implemented")
}

func (m *MockVerifierEnvironment) GetPublicKey(ctx context.Context, address []byte) ([]byte, error) {
	if m.publicKeyErr != nil {
		return nil, m.publicKeyErr
	}
	return m.publicKey, nil
}

func (m *MockVerifierEnvironment) GetTxBlockHeight(ctx context.Context, txID []byte) (uint64, error) {
	if m.txBlockHeightErr != nil {
		return 0, m.txBlockHeightErr
	}
	return m.txBlockHeight, nil
}

func (m *MockVerifierEnvironment) GetOutput(ctx context.Context, outpoint *transaction.OutPoint) (*transaction.TxOutput, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockVerifierEnvironment) IsSponsorClaim(tx *transaction.Transaction) bool {
	return false
}

