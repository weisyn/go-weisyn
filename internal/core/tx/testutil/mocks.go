// Package testutil 提供 TX 模块测试的辅助工具
//
// 🧪 **测试辅助工具包**
//
// 本包提供测试所需的 Mock 对象、测试数据和辅助函数，用于简化测试代码编写。
// 遵循 docs/system/standards/principles/testing-standards.md 规范。
package testutil

import (
	"context"
	"crypto/sha256"
	"fmt"
	"hash"
	"sync"

	"go.uber.org/zap"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxopb "github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/constants"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== Mock 对象 ====================

// MockLogger 统一的日志Mock实现
//
// ✅ **设计原则**：最小实现，所有方法返回空值，不记录日志
// 📋 **使用场景**：80%的测试用例，不需要验证日志调用
type MockLogger struct{}

func (m *MockLogger) Debug(msg string)                          {}
func (m *MockLogger) Debugf(format string, args ...interface{}) {}
func (m *MockLogger) Info(msg string)                           {}
func (m *MockLogger) Infof(format string, args ...interface{})  {}
func (m *MockLogger) Warn(msg string)                           {}
func (m *MockLogger) Warnf(format string, args ...interface{})  {}
func (m *MockLogger) Error(msg string)                          {}
func (m *MockLogger) Errorf(format string, args ...interface{}) {}
func (m *MockLogger) Fatal(msg string)                          {}
func (m *MockLogger) Fatalf(format string, args ...interface{}) {}
func (m *MockLogger) With(args ...interface{}) log.Logger       { return m }
func (m *MockLogger) Sync() error                               { return nil }
func (m *MockLogger) GetZapLogger() *zap.Logger                 { return zap.NewNop() }

// BehavioralMockLogger 行为Mock日志（记录调用）
//
// ✅ **设计原则**：记录所有日志调用，用于验证日志行为
// 📋 **使用场景**：需要验证日志调用的测试（5%的测试用例）
type BehavioralMockLogger struct {
	logs  []string
	mutex sync.Mutex
}

func (m *BehavioralMockLogger) Debug(msg string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logs = append(m.logs, "DEBUG: "+msg)
}

func (m *BehavioralMockLogger) Debugf(format string, args ...interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logs = append(m.logs, fmt.Sprintf("DEBUG: "+format, args...))
}

func (m *BehavioralMockLogger) Info(msg string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logs = append(m.logs, "INFO: "+msg)
}

func (m *BehavioralMockLogger) Infof(format string, args ...interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logs = append(m.logs, fmt.Sprintf("INFO: "+format, args...))
}

func (m *BehavioralMockLogger) Warn(msg string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logs = append(m.logs, "WARN: "+msg)
}

func (m *BehavioralMockLogger) Warnf(format string, args ...interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logs = append(m.logs, fmt.Sprintf("WARN: "+format, args...))
}

func (m *BehavioralMockLogger) Error(msg string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logs = append(m.logs, "ERROR: "+msg)
}

func (m *BehavioralMockLogger) Errorf(format string, args ...interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logs = append(m.logs, fmt.Sprintf("ERROR: "+format, args...))
}

func (m *BehavioralMockLogger) Fatal(msg string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logs = append(m.logs, "FATAL: "+msg)
}

func (m *BehavioralMockLogger) Fatalf(format string, args ...interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logs = append(m.logs, fmt.Sprintf("FATAL: "+format, args...))
}

func (m *BehavioralMockLogger) With(args ...interface{}) log.Logger { return m }
func (m *BehavioralMockLogger) Sync() error                         { return nil }
func (m *BehavioralMockLogger) GetZapLogger() *zap.Logger           { return zap.NewNop() }

// GetLogs 获取所有日志记录
func (m *BehavioralMockLogger) GetLogs() []string {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return append([]string{}, m.logs...)
}

// ClearLogs 清空日志记录
func (m *BehavioralMockLogger) ClearLogs() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logs = m.logs[:0]
}

// MockUTXOQuery 模拟 UTXO 查询服务
type MockUTXOQuery struct {
	utxos map[string]*utxopb.UTXO // key: txid:index
}

// NewMockUTXOQuery 创建模拟 UTXO 查询服务
func NewMockUTXOQuery() *MockUTXOQuery {
	return &MockUTXOQuery{
		utxos: make(map[string]*utxopb.UTXO),
	}
}

// AddUTXO 添加 UTXO 到模拟查询服务
func (m *MockUTXOQuery) AddUTXO(utxo *utxopb.UTXO) {
	key := fmt.Sprintf("%x:%d", utxo.Outpoint.TxId, utxo.Outpoint.OutputIndex)
	m.utxos[key] = utxo
}

// GetUTXO 实现 persistence.UTXOQuery 接口
func (m *MockUTXOQuery) GetUTXO(ctx context.Context, outpoint *transaction.OutPoint) (*utxopb.UTXO, error) {
	key := fmt.Sprintf("%x:%d", outpoint.TxId, outpoint.OutputIndex)
	utxo, ok := m.utxos[key]
	if !ok {
		return nil, fmt.Errorf("UTXO not found: %s", key)
	}
	return utxo, nil
}

// GetCurrentStateRoot 实现 persistence.UTXOQuery 接口
func (m *MockUTXOQuery) GetCurrentStateRoot(ctx context.Context) ([]byte, error) {
	// 简化实现：返回固定值
	return []byte("mock-state-root"), nil
}

// GetUTXOsByAddress 实现 persistence.UTXOQuery 接口
func (m *MockUTXOQuery) GetUTXOsByAddress(ctx context.Context, address []byte, category *utxopb.UTXOCategory, onlyAvailable bool) ([]*utxopb.UTXO, error) {
	var result []*utxopb.UTXO
	for _, utxo := range m.utxos {
		if len(utxo.OwnerAddress) > 0 && len(address) > 0 {
			if string(utxo.OwnerAddress) == string(address) {
				if onlyAvailable {
					if utxo.GetStatus() == utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE {
						result = append(result, utxo)
					}
				} else {
					result = append(result, utxo)
				}
			}
		}
	}
	return result, nil
}

// GetSponsorPoolUTXOs 实现 persistence.UTXOQuery 接口
func (m *MockUTXOQuery) GetSponsorPoolUTXOs(ctx context.Context, onlyAvailable bool) ([]*utxopb.UTXO, error) {
	var result []*utxopb.UTXO
	for _, utxo := range m.utxos {
		// 检查是否为赞助池 UTXO（OwnerAddress 为 SponsorPoolOwner）
		if len(utxo.OwnerAddress) > 0 && len(constants.SponsorPoolOwner[:]) > 0 {
			if string(utxo.OwnerAddress) == string(constants.SponsorPoolOwner[:]) {
				if onlyAvailable {
					if utxo.GetStatus() == utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE {
						result = append(result, utxo)
					}
				} else {
					result = append(result, utxo)
				}
			}
		}
	}
	return result, nil
}

// AddSponsorPoolUTXO 添加赞助池 UTXO（便利方法）
func (m *MockUTXOQuery) AddSponsorPoolUTXO(utxo *utxopb.UTXO) {
	m.AddUTXO(utxo)
}

// MockTxPool 模拟交易池
type MockTxPool struct {
	txs map[string]*transaction.Transaction // key: txid
}

// NewMockTxPool 创建模拟交易池
func NewMockTxPool() *MockTxPool {
	return &MockTxPool{
		txs: make(map[string]*transaction.Transaction),
	}
}

// SubmitTx 实现 mempool.TxPool 接口
func (m *MockTxPool) SubmitTx(tx *transaction.Transaction) ([]byte, error) {
	// 简单模拟：存储交易（注意：实际 Transaction 可能没有 Hash 字段，这里简化处理）
	// 使用交易的序列化作为 key
	txid := fmt.Sprintf("%x", tx.Inputs)
	if len(tx.Inputs) > 0 && tx.Inputs[0].PreviousOutput != nil {
		txid = fmt.Sprintf("%x", tx.Inputs[0].PreviousOutput.TxId)
	}
	m.txs[txid] = tx
	// 返回模拟的交易哈希
	return []byte(txid), nil
}

// SubmitTxs 实现 mempool.TxPool 接口
func (m *MockTxPool) SubmitTxs(txs []*transaction.Transaction) ([][]byte, error) {
	var txHashes [][]byte
	for _, tx := range txs {
		txHash, err := m.SubmitTx(tx)
		if err != nil {
			return nil, err
		}
		txHashes = append(txHashes, txHash)
	}
	return txHashes, nil
}

// GetTransactionsForMining 实现 mempool.TxPool 接口
func (m *MockTxPool) GetTransactionsForMining() ([]*transaction.Transaction, error) {
	var result []*transaction.Transaction
	for _, tx := range m.txs {
		result = append(result, tx)
	}
	return result, nil
}

// MarkTransactionsAsMining 实现 mempool.TxPool 接口
func (m *MockTxPool) MarkTransactionsAsMining(txIDs [][]byte) error {
	return nil
}

// ConfirmTransactions 实现 mempool.TxPool 接口
func (m *MockTxPool) ConfirmTransactions(txIDs [][]byte, blockHeight uint64) error {
	for _, txID := range txIDs {
		txid := fmt.Sprintf("%x", txID)
		delete(m.txs, txid)
	}
	return nil
}

// RejectTransactions 实现 mempool.TxPool 接口
func (m *MockTxPool) RejectTransactions(txIDs [][]byte) error {
	return nil
}

// MarkTransactionsAsPendingConfirm 实现 mempool.TxPool 接口
func (m *MockTxPool) MarkTransactionsAsPendingConfirm(txIDs [][]byte, blockHeight uint64) error {
	return nil
}

// SyncStatus 实现 mempool.TxPool 接口
func (m *MockTxPool) SyncStatus(height uint64, stateRoot []byte) error {
	return nil
}

// UpdateTransactionStatus 实现 mempool.TxPool 接口
func (m *MockTxPool) UpdateTransactionStatus(txID []byte, status types.TxStatus) error {
	return nil
}

// GetAllPendingTransactions 实现 mempool.TxPool 接口
func (m *MockTxPool) GetAllPendingTransactions() ([]*transaction.Transaction, error) {
	var result []*transaction.Transaction
	for _, tx := range m.txs {
		result = append(result, tx)
	}
	return result, nil
}

// GetTx 实现 mempool.TxPool 接口
func (m *MockTxPool) GetTx(txID []byte) (*transaction.Transaction, error) {
	txid := fmt.Sprintf("%x", txID)
	tx, ok := m.txs[txid]
	if !ok {
		return nil, fmt.Errorf("transaction not found")
	}
	return tx, nil
}

// GetTxStatus 实现 mempool.TxPool 接口
func (m *MockTxPool) GetTxStatus(txID []byte) (types.TxStatus, error) {
	txid := fmt.Sprintf("%x", txID)
	if _, ok := m.txs[txid]; ok {
		return types.TxStatusPending, nil
	}
	return types.TxStatusUnknown, fmt.Errorf("transaction not found")
}

// GetPoolStats 实现 mempool.TxPool 接口
func (m *MockTxPool) GetPoolStats() (map[string]interface{}, error) {
	return map[string]interface{}{
		"total_transactions": len(m.txs),
	}, nil
}

// GetTransactionsByStatus 实现 mempool.TxPool 接口
func (m *MockTxPool) GetTransactionsByStatus(status types.TxStatus) ([]*transaction.Transaction, error) {
	var result []*transaction.Transaction
	for _, tx := range m.txs {
		result = append(result, tx)
	}
	return result, nil
}

// GetPendingTransactions 实现 mempool.TxPool 接口
func (m *MockTxPool) GetPendingTransactions() ([]*transaction.Transaction, error) {
	return m.GetAllPendingTransactions()
}

// GetTransactionByID 实现 mempool.TxPool 接口
func (m *MockTxPool) GetTransactionByID(txID []byte) (*transaction.Transaction, error) {
	return m.GetTx(txID)
}

// MockDraftService 模拟草稿服务
type MockDraftService struct {
	drafts map[string]*types.DraftTx
}

// NewMockDraftService 创建模拟草稿服务
func NewMockDraftService() *MockDraftService {
	return &MockDraftService{
		drafts: make(map[string]*types.DraftTx),
	}
}

// CreateDraft 实现 tx.TransactionDraftService 接口
func (m *MockDraftService) CreateDraft(ctx context.Context) (*types.DraftTx, error) {
	draft := &types.DraftTx{
		Tx: &transaction.Transaction{
			Version: 1,
			Inputs:  make([]*transaction.TxInput, 0),
			Outputs: make([]*transaction.TxOutput, 0),
		},
	}
	// 简单模拟：生成一个 ID
	draftID := fmt.Sprintf("draft-%d", len(m.drafts))
	m.drafts[draftID] = draft
	return draft, nil
}

// LoadDraft 实现 tx.TransactionDraftService 接口
func (m *MockDraftService) LoadDraft(ctx context.Context, draftID string) (*types.DraftTx, error) {
	draft, ok := m.drafts[draftID]
	if !ok {
		return nil, fmt.Errorf("draft not found: %s", draftID)
	}
	return draft, nil
}

// SaveDraft 实现 tx.TransactionDraftService 接口
func (m *MockDraftService) SaveDraft(ctx context.Context, draft *types.DraftTx) error {
	// 简单模拟：存储草稿
	draftID := fmt.Sprintf("draft-%d", len(m.drafts))
	m.drafts[draftID] = draft
	return nil
}

// DeleteDraft 实现 tx.TransactionDraftService 接口
func (m *MockDraftService) DeleteDraft(ctx context.Context, draftID string) error {
	delete(m.drafts, draftID)
	return nil
}

// SealDraft 实现 tx.TransactionDraftService 接口
func (m *MockDraftService) SealDraft(ctx context.Context, draft *types.DraftTx) (*types.ComposedTx, error) {
	return &types.ComposedTx{
		Tx:     draft.Tx,
		Sealed: true,
	}, nil
}

// AddInput 实现 tx.TransactionDraftService 接口
func (m *MockDraftService) AddInput(ctx context.Context, draft *types.DraftTx, outpoint *transaction.OutPoint, isReferenceOnly bool, unlockingProof *transaction.UnlockingProof) (uint32, error) {
	input := &transaction.TxInput{
		PreviousOutput:  outpoint,
		IsReferenceOnly: isReferenceOnly,
	}
	if unlockingProof != nil {
		// 设置 UnlockingProof（使用 oneof）
		if singleKeyProof := unlockingProof.GetSingleKeyProof(); singleKeyProof != nil {
			input.UnlockingProof = &transaction.TxInput_SingleKeyProof{
				SingleKeyProof: singleKeyProof,
			}
		}
	}
	draft.Tx.Inputs = append(draft.Tx.Inputs, input)
	return uint32(len(draft.Tx.Inputs) - 1), nil
}

// AddAssetOutput 实现 tx.TransactionDraftService 接口
func (m *MockDraftService) AddAssetOutput(ctx context.Context, draft *types.DraftTx, owner []byte, amount string, tokenID []byte, lockingConditions []*transaction.LockingCondition) (uint32, error) {
	var assetOutput *transaction.AssetOutput
	if tokenID == nil {
		assetOutput = &transaction.AssetOutput{
			AssetContent: &transaction.AssetOutput_NativeCoin{
				NativeCoin: &transaction.NativeCoinAsset{
					Amount: amount,
				},
			},
		}
	} else {
		assetOutput = &transaction.AssetOutput{
			AssetContent: &transaction.AssetOutput_ContractToken{
				ContractToken: &transaction.ContractTokenAsset{
					ContractAddress: tokenID,
					TokenIdentifier: &transaction.ContractTokenAsset_FungibleClassId{
						FungibleClassId: []byte("default"),
					},
					Amount: amount,
				},
			},
		}
	}
	output := &transaction.TxOutput{
		Owner:             owner,
		LockingConditions: lockingConditions,
		OutputContent: &transaction.TxOutput_Asset{
			Asset: assetOutput,
		},
	}
	draft.Tx.Outputs = append(draft.Tx.Outputs, output)
	return uint32(len(draft.Tx.Outputs) - 1), nil
}

// AddResourceOutput 实现 tx.TransactionDraftService 接口
func (m *MockDraftService) AddResourceOutput(ctx context.Context, draft *types.DraftTx, contentHash []byte, category string, owner []byte, lockingConditions []*transaction.LockingCondition, metadata []byte) (uint32, error) {
	// 简化实现
	output := &transaction.TxOutput{
		Owner:             owner,
		LockingConditions: lockingConditions,
	}
	draft.Tx.Outputs = append(draft.Tx.Outputs, output)
	return uint32(len(draft.Tx.Outputs) - 1), nil
}

// AddStateOutput 实现 tx.TransactionDraftService 接口
func (m *MockDraftService) AddStateOutput(ctx context.Context, draft *types.DraftTx, stateID []byte, stateVersion uint64, executionResultHash []byte, publicInputs []byte, parentStateHash []byte) (uint32, error) {
	// 简化实现
	output := &transaction.TxOutput{
		Owner: make([]byte, 20),
	}
	draft.Tx.Outputs = append(draft.Tx.Outputs, output)
	return uint32(len(draft.Tx.Outputs) - 1), nil
}

// GetDraftByID 实现 tx.TransactionDraftService 接口
func (m *MockDraftService) GetDraftByID(ctx context.Context, draftID string) (*types.DraftTx, error) {
	return m.LoadDraft(ctx, draftID)
}

// ValidateDraft 实现 tx.TransactionDraftService 接口
func (m *MockDraftService) ValidateDraft(ctx context.Context, draft *types.DraftTx) error {
	if draft == nil {
		return fmt.Errorf("draft is nil")
	}
	if draft.Tx == nil {
		return fmt.Errorf("draft.Tx is nil")
	}
	return nil
}

// MockProofProvider 模拟证明提供者
type MockProofProvider struct {
	proofs map[int]*transaction.UnlockingProof // key: input index
}

// NewMockProofProvider 创建模拟证明提供者
func NewMockProofProvider() *MockProofProvider {
	return &MockProofProvider{
		proofs: make(map[int]*transaction.UnlockingProof),
	}
}

// SetProof 设置指定输入的证明
func (m *MockProofProvider) SetProof(inputIndex int, proof *transaction.UnlockingProof) {
	m.proofs[inputIndex] = proof
}

// ProvideProofs 实现 tx.ProofProvider 接口
func (m *MockProofProvider) ProvideProofs(
	ctx context.Context,
	tx *transaction.Transaction,
) error {
	for i, input := range tx.Inputs {
		proof, ok := m.proofs[i]
		if !ok {
			return fmt.Errorf("proof not found for input %d", i)
		}
		// 设置 UnlockingProof（使用 oneof）
		if singleKeyProof := proof.GetSingleKeyProof(); singleKeyProof != nil {
			input.UnlockingProof = &transaction.TxInput_SingleKeyProof{
				SingleKeyProof: singleKeyProof,
			}
		}
	}
	return nil
}

// MockSigner 模拟签名器
type MockSigner struct {
	publicKey []byte
}

// NewMockSigner 创建模拟签名器
func NewMockSigner(publicKey []byte) *MockSigner {
	if publicKey == nil {
		publicKey = RandomPublicKey()
	}
	return &MockSigner{
		publicKey: publicKey,
	}
}

// Sign 实现 tx.Signer 接口
func (m *MockSigner) Sign(ctx context.Context, tx *transaction.Transaction) (*transaction.SignatureData, error) {
	return &transaction.SignatureData{
		Value: []byte("mock-signature"),
	}, nil
}

// PublicKey 实现 tx.Signer 接口
func (m *MockSigner) PublicKey() (*transaction.PublicKey, error) {
	return &transaction.PublicKey{
		Value: m.publicKey,
	}, nil
}

// Algorithm 实现 tx.Signer 接口
func (m *MockSigner) Algorithm() transaction.SignatureAlgorithm {
	return transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1
}

// SignBytes 实现 tx.Signer 接口
func (m *MockSigner) SignBytes(ctx context.Context, data []byte) ([]byte, error) {
	return []byte("mock-signature-bytes"), nil
}

// ==================== Crypto Mock 对象（供其他测试使用）====================

// MockSignatureManager 模拟 SignatureManager（供其他测试使用）
type MockSignatureManager struct {
	signature []byte
}

func (m *MockSignatureManager) Sign(data []byte, privateKey []byte) ([]byte, error) {
	if m.signature != nil {
		return m.signature, nil
	}
	return []byte("mock-signature"), nil
}

func (m *MockSignatureManager) Verify(data, signature, publicKey []byte) bool {
	return true
}

func (m *MockSignatureManager) RecoverPublicKey(hash []byte, signature []byte) ([]byte, error) {
	return RandomPublicKey(), nil
}

func (m *MockSignatureManager) NormalizeSignature(signature []byte) ([]byte, error) {
	return signature, nil
}

func (m *MockSignatureManager) SignTransaction(txHash []byte, privateKey []byte, sigHashType crypto.SignatureHashType) ([]byte, error) {
	return m.Sign(txHash, privateKey)
}

func (m *MockSignatureManager) VerifyTransactionSignature(txHash []byte, signature []byte, publicKey []byte, sigHashType crypto.SignatureHashType) bool {
	return m.Verify(txHash, signature, publicKey)
}

func (m *MockSignatureManager) SignMessage(message []byte, privateKey []byte) ([]byte, error) {
	return m.Sign(message, privateKey)
}

func (m *MockSignatureManager) VerifyMessage(message []byte, signature []byte, publicKey []byte) bool {
	return m.Verify(message, signature, publicKey)
}

func (m *MockSignatureManager) ValidateSignature(signature []byte) error {
	if len(signature) == 0 {
		return fmt.Errorf("signature is empty")
	}
	return nil
}

func (m *MockSignatureManager) VerifyBatch(dataList [][]byte, signatureList [][]byte, publicKeyList [][]byte) ([]bool, error) {
	results := make([]bool, len(dataList))
	for i := range dataList {
		results[i] = m.Verify(dataList[i], signatureList[i], publicKeyList[i])
	}
	return results, nil
}

func (m *MockSignatureManager) RecoverAddress(hash []byte, signature []byte) (string, error) {
	pubKey, err := m.RecoverPublicKey(hash, signature)
	if err != nil {
		return "", err
	}
	// 简化实现：返回公钥的十六进制字符串作为地址
	return fmt.Sprintf("%x", pubKey), nil
}

func (m *MockSignatureManager) SignBatch(dataList [][]byte, privateKey []byte) ([][]byte, error) {
	signatures := make([][]byte, len(dataList))
	for i, data := range dataList {
		sig, err := m.Sign(data, privateKey)
		if err != nil {
			return nil, err
		}
		signatures[i] = sig
	}
	return signatures, nil
}

// MockHashManager 统一的哈希管理器Mock实现
//
// ✅ **设计原则**：使用真实的SHA256算法，确保哈希计算正确
// 📋 **使用场景**：所有需要哈希计算的测试
type MockHashManager struct{}

func (m *MockHashManager) SHA256(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

func (m *MockHashManager) Keccak256(data []byte) []byte {
	return m.SHA256(data) // 简化实现，使用SHA256
}

func (m *MockHashManager) RIPEMD160(data []byte) []byte {
	hash := make([]byte, 20)
	copy(hash, m.SHA256(data)[:20])
	return hash
}

func (m *MockHashManager) DoubleSHA256(data []byte) []byte {
	first := m.SHA256(data)
	return m.SHA256(first)
}

func (m *MockHashManager) NewSHA256Hasher() hash.Hash {
	return sha256.New()
}

func (m *MockHashManager) NewRIPEMD160Hasher() hash.Hash {
	return sha256.New() // 简化实现，返回SHA256的hasher
}

// MockCanonicalizer 模拟 Canonicalizer（供其他测试使用）
// 注意：这是一个简化的实现，实际应该使用 hash.Canonicalizer
type MockCanonicalizer struct {
	txHash  []byte
	sigHash []byte
}

func (m *MockCanonicalizer) ComputeTransactionHash(ctx context.Context, tx *transaction.Transaction) ([]byte, error) {
	if m.txHash != nil {
		return m.txHash, nil
	}
	return RandomTxID(), nil
}

func (m *MockCanonicalizer) ComputeSignatureHash(ctx context.Context, tx *transaction.Transaction, inputIndex int, sigHashType transaction.SignatureHashType) ([]byte, error) {
	if m.sigHash != nil {
		return m.sigHash, nil
	}
	return RandomTxID(), nil
}

// MockAddressManager 模拟 AddressManager（供测试使用）
type MockAddressManager struct{}

func (m *MockAddressManager) PrivateKeyToAddress(privateKey []byte) (string, error) {
	if len(privateKey) != 32 {
		return "", fmt.Errorf("invalid private key length")
	}
	// 简化实现：基于私钥哈希生成地址字符串
	hash := sha256.Sum256(privateKey)
	return fmt.Sprintf("Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPm%x", hash[:8]), nil
}

func (m *MockAddressManager) PublicKeyToAddress(publicKey []byte) (string, error) {
	if len(publicKey) != 33 && len(publicKey) != 64 {
		return "", fmt.Errorf("invalid public key length")
	}
	// 简化实现：基于公钥哈希生成地址字符串，确保一致性
	hash := sha256.Sum256(publicKey)
	return fmt.Sprintf("Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPm%x", hash[:8]), nil
}

func (m *MockAddressManager) StringToAddress(addressStr string) (string, error) {
	if addressStr == "" {
		return "", fmt.Errorf("empty address string")
	}
	return addressStr, nil
}

func (m *MockAddressManager) ValidateAddress(address string) (bool, error) {
	if address == "" {
		return false, fmt.Errorf("empty address")
	}
	return true, nil
}

func (m *MockAddressManager) AddressToBytes(address string) ([]byte, error) {
	if address == "" {
		return nil, fmt.Errorf("empty address")
	}
	// 简化实现：从地址字符串生成20字节哈希
	hash := sha256.Sum256([]byte(address))
	return hash[:20], nil
}

func (m *MockAddressManager) BytesToAddress(addressBytes []byte) (string, error) {
	if len(addressBytes) != 20 {
		return "", fmt.Errorf("invalid address bytes length")
	}
	// 简化实现：返回一个固定的测试地址
	return "Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn", nil
}

func (m *MockAddressManager) AddressToHexString(address string) (string, error) {
	if address == "" {
		return "", fmt.Errorf("empty address")
	}
	// 简化实现：返回40字符的十六进制字符串
	return "0000000000000000000000000000000000000000", nil
}

func (m *MockAddressManager) HexStringToAddress(hexStr string) (string, error) {
	if hexStr == "" {
		return "", fmt.Errorf("empty hex string")
	}
	// 简化实现：返回一个固定的测试地址
	return "Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn", nil
}

func (m *MockAddressManager) GetAddressType(address string) (types.AddressType, error) {
	if address == "" {
		return types.AddressTypeInvalid, fmt.Errorf("empty address")
	}
	return types.AddressTypeBitcoin, nil
}

func (m *MockAddressManager) CompareAddresses(addr1, addr2 string) (bool, error) {
	if addr1 == "" || addr2 == "" {
		return false, fmt.Errorf("empty address")
	}
	return addr1 == addr2, nil
}

func (m *MockAddressManager) IsZeroAddress(address string) bool {
	return address == "" || address == "0000000000000000000000000000000000000000"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
