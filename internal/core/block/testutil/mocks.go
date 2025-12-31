// Package testutil 提供 Block 模块测试的辅助工具
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
	"math/big"
	"sync"
	"time"

	"google.golang.org/grpc"

	"go.uber.org/zap"

	"github.com/weisyn/v1/internal/core/block/interfaces"
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	pb_resource "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
	utxopb "github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/ispc"
	txiface "github.com/weisyn/v1/pkg/interfaces/tx"
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

// MockBadgerStore 模拟 BadgerStore
type MockBadgerStore struct {
	data  map[string][]byte
	mu    sync.RWMutex
	err   error
	errMu sync.RWMutex
}

// NewMockBadgerStore 创建模拟 BadgerStore
func NewMockBadgerStore() *MockBadgerStore {
	return &MockBadgerStore{
		data: make(map[string][]byte),
	}
}

// Close 实现 storage.BadgerStore 接口
func (m *MockBadgerStore) Close() error {
	return nil
}

// Get 实现 storage.BadgerStore 接口
func (m *MockBadgerStore) Get(ctx context.Context, key []byte) ([]byte, error) {
	if err := m.getError(); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.data[string(key)]
	if !ok {
		return nil, fmt.Errorf("key not found: %x", key)
	}
	return val, nil
}

// Set 实现 storage.BadgerStore 接口
func (m *MockBadgerStore) Set(ctx context.Context, key, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[string(key)] = value
	return nil
}

// SetWithTTL 实现 storage.BadgerStore 接口
func (m *MockBadgerStore) SetWithTTL(ctx context.Context, key, value []byte, ttl time.Duration) error {
	return m.Set(ctx, key, value)
}

// Delete 实现 storage.BadgerStore 接口
func (m *MockBadgerStore) Delete(ctx context.Context, key []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, string(key))
	return nil
}

// Exists 实现 storage.BadgerStore 接口
func (m *MockBadgerStore) Exists(ctx context.Context, key []byte) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.data[string(key)]
	return ok, nil
}

// GetMany 实现 storage.BadgerStore 接口
func (m *MockBadgerStore) GetMany(ctx context.Context, keys [][]byte) (map[string][]byte, error) {
	result := make(map[string][]byte)
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, key := range keys {
		if val, ok := m.data[string(key)]; ok {
			result[string(key)] = val
		}
	}
	return result, nil
}

// SetMany 实现 storage.BadgerStore 接口
func (m *MockBadgerStore) SetMany(ctx context.Context, entries map[string][]byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range entries {
		m.data[k] = v
	}
	return nil
}

// DeleteMany 实现 storage.BadgerStore 接口
func (m *MockBadgerStore) DeleteMany(ctx context.Context, keys [][]byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, key := range keys {
		delete(m.data, string(key))
	}
	return nil
}

// PrefixScan 实现 storage.BadgerStore 接口
func (m *MockBadgerStore) PrefixScan(ctx context.Context, prefix []byte) (map[string][]byte, error) {
	result := make(map[string][]byte)
	prefixStr := string(prefix)
	m.mu.RLock()
	defer m.mu.RUnlock()
	for k, v := range m.data {
		if len(k) >= len(prefixStr) && k[:len(prefixStr)] == prefixStr {
			result[k] = v
		}
	}
	return result, nil
}

// RangeScan 实现 storage.BadgerStore 接口
func (m *MockBadgerStore) RangeScan(ctx context.Context, startKey, endKey []byte) (map[string][]byte, error) {
	result := make(map[string][]byte)
	startStr := string(startKey)
	endStr := string(endKey)
	m.mu.RLock()
	defer m.mu.RUnlock()
	for k, v := range m.data {
		if k >= startStr && k < endStr {
			result[k] = v
		}
	}
	return result, nil
}

// RunInTransaction 实现 storage.BadgerStore 接口
func (m *MockBadgerStore) RunInTransaction(ctx context.Context, fn func(storage.BadgerTransaction) error) error {
	// 简化实现：直接执行，不实现真正的事务
	return fn(&MockBadgerTransaction{store: m})
}

// MockBadgerTransaction 模拟 BadgerTransaction
type MockBadgerTransaction struct {
	store *MockBadgerStore
}

func (m *MockBadgerTransaction) Get(key []byte) ([]byte, error) {
	return m.store.Get(context.Background(), key)
}

func (m *MockBadgerTransaction) Set(key, value []byte) error {
	return m.store.Set(context.Background(), key, value)
}

func (m *MockBadgerTransaction) Delete(key []byte) error {
	return m.store.Delete(context.Background(), key)
}

func (m *MockBadgerTransaction) Exists(key []byte) (bool, error) {
	return m.store.Exists(context.Background(), key)
}

func (m *MockBadgerTransaction) SetWithTTL(key, value []byte, ttl time.Duration) error {
	return m.store.SetWithTTL(context.Background(), key, value, ttl)
}

func (m *MockBadgerTransaction) Merge(key, value []byte, mergeFunc func(existingVal, newVal []byte) []byte) error {
	existing, _ := m.Get(key)
	merged := mergeFunc(existing, value)
	return m.Set(key, merged)
}

// GetSizeEstimator 实现 storage.BadgerTransaction 接口
func (m *MockBadgerTransaction) GetSizeEstimator() storage.TxSizeEstimator {
	// Mock 实现返回 nil（测试中不需要实际的大小估算）
	return nil
}

// SetData 设置测试数据
func (m *MockBadgerStore) SetData(key []byte, value []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[string(key)] = value
}

// SetError 设置错误（用于测试错误场景）
func (m *MockBadgerStore) SetError(err error) {
	m.errMu.Lock()
	defer m.errMu.Unlock()
	m.err = err
}

// getError 获取错误
func (m *MockBadgerStore) getError() error {
	m.errMu.RLock()
	defer m.errMu.RUnlock()
	return m.err
}

// MockTxPool 模拟交易池
type MockTxPool struct {
	txs   []*transaction.Transaction
	mu    sync.RWMutex
	err   error
	errMu sync.RWMutex
}

// NewMockTxPool 创建模拟交易池
func NewMockTxPool() *MockTxPool {
	return &MockTxPool{
		txs: make([]*transaction.Transaction, 0),
	}
}

// SubmitTx 实现 mempool.TxPool 接口
func (m *MockTxPool) SubmitTx(tx *transaction.Transaction) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.txs = append(m.txs, tx)
	return []byte(fmt.Sprintf("tx-%d", len(m.txs))), nil
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
	m.errMu.RLock()
	err := m.err
	m.errMu.RUnlock()
	if err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.txs, nil
}

// SetError 设置错误（用于测试错误场景）
func (m *MockTxPool) SetError(err error) {
	m.errMu.Lock()
	defer m.errMu.Unlock()
	m.err = err
}

// MarkTransactionsAsMining 实现 mempool.TxPool 接口
func (m *MockTxPool) MarkTransactionsAsMining(txIDs [][]byte) error {
	return nil
}

// ConfirmTransactions 实现 mempool.TxPool 接口
func (m *MockTxPool) ConfirmTransactions(txIDs [][]byte, blockHeight uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// 简化实现：移除所有交易
	m.txs = make([]*transaction.Transaction, 0)
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
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.txs, nil
}

// GetTx 实现 mempool.TxPool 接口
func (m *MockTxPool) GetTx(txID []byte) (*transaction.Transaction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.txs) > 0 {
		return m.txs[0], nil
	}
	return nil, fmt.Errorf("transaction not found")
}

// GetTxStatus 实现 mempool.TxPool 接口
func (m *MockTxPool) GetTxStatus(txID []byte) (types.TxStatus, error) {
	return types.TxStatusPending, nil
}

// GetTransactionsByStatus 实现 mempool.TxPool 接口
func (m *MockTxPool) GetTransactionsByStatus(status types.TxStatus) ([]*transaction.Transaction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.txs, nil
}

// GetTransactionByID 实现 mempool.TxPool 接口
func (m *MockTxPool) GetTransactionByID(txID []byte) (*transaction.Transaction, error) {
	return m.GetTx(txID)
}

// GetPendingTransactions 实现 mempool.TxPool 接口
func (m *MockTxPool) GetPendingTransactions() ([]*transaction.Transaction, error) {
	return m.GetAllPendingTransactions()
}

// AddTransaction 添加交易到池中（辅助方法）
func (m *MockTxPool) AddTransaction(tx *transaction.Transaction) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.txs = append(m.txs, tx)
}

// RemoveTransaction 从池中移除交易（辅助方法）
func (m *MockTxPool) RemoveTransaction(txHash []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// 简化实现：移除所有交易
	m.txs = make([]*transaction.Transaction, 0)
	return nil
}

// MockTxProcessor 模拟交易处理器
type MockTxProcessor struct{}

// ProcessTransaction 实现 txiface.TxProcessor 接口
func (m *MockTxProcessor) ProcessTransaction(ctx context.Context, tx *transaction.Transaction) error {
	return nil
}

// SubmitTx 实现 txiface.TxProcessor 接口
func (m *MockTxProcessor) SubmitTx(ctx context.Context, signedTx *types.SignedTx) (*types.SubmittedTx, error) {
	return &types.SubmittedTx{
		Tx: signedTx.Tx,
	}, nil
}

// GetTxStatus 实现 txiface.TxProcessor 接口
func (m *MockTxProcessor) GetTxStatus(ctx context.Context, txHash []byte) (*types.TxBroadcastState, error) {
	return &types.TxBroadcastState{
		Status:      types.BroadcastStatusLocalSubmitted,
		SubmittedAt: time.Now(),
	}, nil
}

// MockZKProofService 模拟ZK证明服务
// 实现 ispc.ZKProofService 接口
type MockZKProofService struct {
	verifyResult bool
	verifyError  error
	mu           sync.RWMutex
}

// 确保MockZKProofService实现了ispc.ZKProofService接口
var _ ispc.ZKProofService = (*MockZKProofService)(nil)

// NewMockZKProofService 创建模拟ZK证明服务
func NewMockZKProofService() *MockZKProofService {
	return &MockZKProofService{
		verifyResult: true, // 默认验证通过
		verifyError:  nil,
	}
}

// GenerateStateProof 实现 ispc.ZKProofService 接口
func (m *MockZKProofService) GenerateStateProof(
	ctx context.Context,
	executionResultHash []byte,
	publicInputs [][]byte,
	circuitID string,
) (*transaction.ZKStateProof, error) {
	// 返回一个模拟的ZK证明
	return &transaction.ZKStateProof{
		Proof:               []byte("mock-proof"),
		PublicInputs:        publicInputs,
		ProvingScheme:       "groth16",
		Curve:               "bn254",
		VerificationKeyHash: make([]byte, 32),
		CircuitId:           circuitID,
		CircuitVersion:      1,
	}, nil
}

// VerifyStateProof 实现 ispc.ZKProofService 接口
func (m *MockZKProofService) VerifyStateProof(
	ctx context.Context,
	proof *transaction.ZKStateProof,
) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.verifyResult, m.verifyError
}

// SetVerifyResult 设置验证结果（用于测试）
func (m *MockZKProofService) SetVerifyResult(result bool, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.verifyResult = result
	m.verifyError = err
}

// MockHashManager 模拟哈希管理器
type MockHashManager struct{}

// SHA256 实现 crypto.HashManager 接口
func (m *MockHashManager) SHA256(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:]
}

// Keccak256 实现 crypto.HashManager 接口
func (m *MockHashManager) Keccak256(data []byte) []byte {
	// 简化实现：使用SHA256代替
	return m.SHA256(data)
}

// RIPEMD160 实现 crypto.HashManager 接口
func (m *MockHashManager) RIPEMD160(data []byte) []byte {
	// 简化实现：返回20字节
	result := make([]byte, 20)
	copy(result, data)
	if len(result) > 20 {
		result = result[:20]
	}
	return result
}

// DoubleSHA256 实现 crypto.HashManager 接口
func (m *MockHashManager) DoubleSHA256(data []byte) []byte {
	first := sha256.Sum256(data)
	second := sha256.Sum256(first[:])
	return second[:]
}

// NewSHA256Hasher 实现 crypto.HashManager 接口
func (m *MockHashManager) NewSHA256Hasher() hash.Hash {
	return sha256.New()
}

// NewRIPEMD160Hasher 实现 crypto.HashManager 接口
func (m *MockHashManager) NewRIPEMD160Hasher() hash.Hash {
	// 简化实现：返回SHA256代替
	return sha256.New()
}

// MockBlockHashClient 模拟区块哈希服务客户端
type MockBlockHashClient struct {
	hashFunc func(*core.Block) ([]byte, error)
	err      error
	errMu    sync.RWMutex
}

// NewMockBlockHashClient 创建模拟区块哈希客户端
func NewMockBlockHashClient() *MockBlockHashClient {
	return &MockBlockHashClient{
		hashFunc: func(block *core.Block) ([]byte, error) {
			// 默认实现：返回固定哈希
			hash := make([]byte, 32)
			if block != nil && block.Header != nil {
				copy(hash, fmt.Sprintf("block-%d", block.Header.Height))
			}
			return hash, nil
		},
	}
}

// ComputeBlockHash 实现 core.BlockHashServiceClient 接口
func (m *MockBlockHashClient) ComputeBlockHash(ctx context.Context, req *core.ComputeBlockHashRequest, opts ...grpc.CallOption) (*core.ComputeBlockHashResponse, error) {
	m.errMu.RLock()
	err := m.err
	m.errMu.RUnlock()
	if err != nil {
		return nil, err
	}
	if m.hashFunc != nil {
		hash, err := m.hashFunc(req.Block)
		if err != nil {
			return nil, err
		}
		return &core.ComputeBlockHashResponse{
			IsValid: true,
			Hash:    hash,
		}, nil
	}
	return &core.ComputeBlockHashResponse{
		IsValid: true,
		Hash:    make([]byte, 32),
	}, nil
}

// SetError 设置错误（用于测试错误场景）
func (m *MockBlockHashClient) SetError(err error) {
	m.errMu.Lock()
	defer m.errMu.Unlock()
	m.err = err
}

// ValidateBlockHash 实现 core.BlockHashServiceClient 接口
func (m *MockBlockHashClient) ValidateBlockHash(ctx context.Context, req *core.ValidateBlockHashRequest, opts ...grpc.CallOption) (*core.ValidateBlockHashResponse, error) {
	if m.hashFunc != nil {
		hash, err := m.hashFunc(req.Block)
		if err != nil {
			return &core.ValidateBlockHashResponse{
				IsValid: false,
			}, nil
		}
		isValid := len(hash) == len(req.ExpectedHash)
		if isValid {
			for i := range hash {
				if hash[i] != req.ExpectedHash[i] {
					isValid = false
					break
				}
			}
		}
		return &core.ValidateBlockHashResponse{
			IsValid:      isValid,
			ComputedHash: hash,
		}, nil
	}
	return &core.ValidateBlockHashResponse{
		IsValid: true,
	}, nil
}

// MockTransactionHashClient 模拟交易哈希服务客户端
type MockTransactionHashClient struct {
	hashFunc func(*transaction.Transaction) ([]byte, error)
	err      error
	errMu    sync.RWMutex
}

// NewMockTransactionHashClient 创建模拟交易哈希客户端
func NewMockTransactionHashClient() *MockTransactionHashClient {
	return &MockTransactionHashClient{
		hashFunc: func(tx *transaction.Transaction) ([]byte, error) {
			// 默认实现：返回固定哈希
			hash := make([]byte, 32)
			if tx != nil {
				copy(hash, fmt.Sprintf("tx-%d", tx.Nonce))
			}
			return hash, nil
		},
	}
}

// ComputeHash 实现 transaction.TransactionHashServiceClient 接口
func (m *MockTransactionHashClient) ComputeHash(ctx context.Context, req *transaction.ComputeHashRequest, opts ...grpc.CallOption) (*transaction.ComputeHashResponse, error) {
	m.errMu.RLock()
	err := m.err
	m.errMu.RUnlock()
	if err != nil {
		return nil, err
	}
	if m.hashFunc != nil {
		hash, err := m.hashFunc(req.Transaction)
		if err != nil {
			return nil, err
		}
		return &transaction.ComputeHashResponse{
			IsValid: true,
			Hash:    hash,
		}, nil
	}
	return &transaction.ComputeHashResponse{
		IsValid: true,
		Hash:    make([]byte, 32),
	}, nil
}

// SetError 设置错误（用于测试错误场景）
func (m *MockTransactionHashClient) SetError(err error) {
	m.errMu.Lock()
	defer m.errMu.Unlock()
	m.err = err
}

// ComputeSignatureHash 实现 transaction.TransactionHashServiceClient 接口
func (m *MockTransactionHashClient) ComputeSignatureHash(ctx context.Context, req *transaction.ComputeSignatureHashRequest, opts ...grpc.CallOption) (*transaction.ComputeSignatureHashResponse, error) {
	hash := make([]byte, 32)
	if req.Transaction != nil {
		copy(hash, fmt.Sprintf("sig-%d-%d", req.InputIndex, req.Transaction.Nonce))
	}
	return &transaction.ComputeSignatureHashResponse{
		IsValid: true,
		Hash:    hash,
	}, nil
}

// ValidateHash 实现 transaction.TransactionHashServiceClient 接口
func (m *MockTransactionHashClient) ValidateHash(ctx context.Context, req *transaction.ValidateHashRequest, opts ...grpc.CallOption) (*transaction.ValidateHashResponse, error) {
	if m.hashFunc != nil {
		hash, err := m.hashFunc(req.Transaction)
		if err != nil {
			return &transaction.ValidateHashResponse{
				IsValid: false,
			}, nil
		}
		isValid := len(hash) == len(req.ExpectedHash)
		if isValid {
			for i := range hash {
				if hash[i] != req.ExpectedHash[i] {
					isValid = false
					break
				}
			}
		}
		return &transaction.ValidateHashResponse{
			IsValid:      isValid,
			ComputedHash: hash,
		}, nil
	}
	return &transaction.ValidateHashResponse{
		IsValid: true,
	}, nil
}

// ValidateSignatureHash 实现 transaction.TransactionHashServiceClient 接口
func (m *MockTransactionHashClient) ValidateSignatureHash(ctx context.Context, req *transaction.ValidateSignatureHashRequest, opts ...grpc.CallOption) (*transaction.ValidateSignatureHashResponse, error) {
	hash := make([]byte, 32)
	if req.Transaction != nil {
		copy(hash, fmt.Sprintf("sig-%d-%d", req.InputIndex, req.Transaction.Nonce))
	}
	isValid := len(hash) == len(req.ExpectedHash)
	if isValid {
		for i := range hash {
			if hash[i] != req.ExpectedHash[i] {
				isValid = false
				break
			}
		}
	}
	return &transaction.ValidateSignatureHashResponse{
		IsValid:      isValid,
		ComputedHash: hash,
	}, nil
}

// MockQueryService 模拟查询服务
type MockQueryService struct {
	blocks map[string]*core.Block
	// blocksByHeight 维护“主链视角”的 canonical 区块映射：
	// - 用于在测试中出现“同高度多个区块”（分叉场景）时，仍能确定性地返回主链块
	// - 不影响只设置单块/单高度的既有测试用例
	blocksByHeight map[uint64]*core.Block
	hashByHeight   map[uint64][]byte
	mu             sync.RWMutex
	err            error
	errMu          sync.RWMutex
}

// NewMockQueryService 创建模拟查询服务
func NewMockQueryService() *MockQueryService {
	return &MockQueryService{
		blocks:         make(map[string]*core.Block),
		blocksByHeight: make(map[uint64]*core.Block),
		hashByHeight:   make(map[uint64][]byte),
	}
}

// GetBlockByHash 实现 persistence.BlockQuery 接口
func (m *MockQueryService) GetBlockByHash(ctx context.Context, hash []byte) (*core.Block, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	block, ok := m.blocks[string(hash)]
	if !ok {
		return nil, fmt.Errorf("block not found")
	}
	return block, nil
}

// GetBlockByHeight 实现 persistence.BlockQuery 接口
func (m *MockQueryService) GetBlockByHeight(ctx context.Context, height uint64) (*core.Block, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// 优先返回 canonical 主链块（用于分叉测试的确定性）
	if b, ok := m.blocksByHeight[height]; ok && b != nil {
		return b, nil
	}
	for _, block := range m.blocks {
		if block.Header != nil && block.Header.Height == height {
			return block, nil
		}
	}
	return nil, fmt.Errorf("block not found at height %d", height)
}

// GetBlockHeader 实现 persistence.BlockQuery 接口
func (m *MockQueryService) GetBlockHeader(ctx context.Context, hash []byte) (*core.BlockHeader, error) {
	block, err := m.GetBlockByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	return block.Header, nil
}

// GetBlockRange 实现 persistence.BlockQuery 接口
func (m *MockQueryService) GetBlockRange(ctx context.Context, startHeight, endHeight uint64) ([]*core.Block, error) {
	var result []*core.Block
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, block := range m.blocks {
		if block.Header != nil && block.Header.Height >= startHeight && block.Header.Height <= endHeight {
			result = append(result, block)
		}
	}
	return result, nil
}

// GetHighestBlock 实现 persistence.BlockQuery 接口
func (m *MockQueryService) GetHighestBlock(ctx context.Context) (height uint64, blockHash []byte, err error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// 若存在 canonical 映射，优先用它来确定性返回主链最高块
	var highestHeight uint64
	for h := range m.blocksByHeight {
		if h > highestHeight {
			highestHeight = h
		}
	}
	if highestHeight > 0 || (highestHeight == 0 && m.blocksByHeight[0] != nil) {
		h := highestHeight
		hash := m.hashByHeight[h]
		if len(hash) == 0 {
			// 兜底：保持旧行为，避免部分测试未设置 hashByHeight
			fallback := make([]byte, 32)
			copy(fallback, fmt.Sprintf("block-%d", h))
			hash = fallback
		}
		return h, hash, nil
	}

	// 兼容旧逻辑：未设置 canonical 时，从 blocks 扫描
	var highestBlock *core.Block
	for _, block := range m.blocks {
		if block.Header != nil && (highestBlock == nil || block.Header.Height > highestBlock.Header.Height) {
			highestBlock = block
		}
	}
	if highestBlock == nil || highestBlock.Header == nil {
		return 0, nil, fmt.Errorf("no blocks found")
	}
	fallback := make([]byte, 32)
	copy(fallback, fmt.Sprintf("block-%d", highestBlock.Header.Height))
	return highestBlock.Header.Height, fallback, nil
}

// BuildFilePath 实现 persistence.QueryService 接口
func (m *MockQueryService) BuildFilePath(path []byte) string {
	return string(path)
}

// CheckFileExists 实现 persistence.QueryService 接口
func (m *MockQueryService) CheckFileExists(contentHash []byte) bool {
	return false
}

// GetAccountBalance 实现 persistence.QueryService 接口
func (m *MockQueryService) GetAccountBalance(ctx context.Context, address []byte, tokenID []byte) (*types.BalanceInfo, error) {
	return &types.BalanceInfo{
		Address: &transaction.Address{
			RawHash: address,
		},
		TokenID:            tokenID,
		Available:          0,
		Locked:             0,
		Pending:            0,
		Total:              0,
		AvailableFormatted: "0",
		LockedFormatted:    "0",
		PendingFormatted:   "0",
		TotalFormatted:     "0",
		UTXOCount:          0,
	}, nil
}

// GetAccountNonce 实现 persistence.QueryService 接口
func (m *MockQueryService) GetAccountNonce(ctx context.Context, address []byte) (uint64, error) {
	return 0, nil
}

// GetBestBlockHash 实现 persistence.QueryService 接口
func (m *MockQueryService) GetBestBlockHash(ctx context.Context) ([]byte, error) {
	height, hash, err := m.GetHighestBlock(ctx)
	if err != nil {
		return nil, err
	}
	_ = height
	return hash, nil
}

// GetBlockTimestamp 实现 persistence.QueryService 接口
func (m *MockQueryService) GetBlockTimestamp(ctx context.Context, height uint64) (int64, error) {
	block, err := m.GetBlockByHeight(ctx, height)
	if err != nil {
		return 0, err
	}
	if block.Header != nil {
		return int64(block.Header.Timestamp), nil
	}
	return 0, nil
}

// GetCurrentHeight 实现 persistence.QueryService 接口
func (m *MockQueryService) GetCurrentHeight(ctx context.Context) (uint64, error) {
	height, _, err := m.GetHighestBlock(ctx)
	if err != nil {
		return 0, err
	}
	return height, nil
}

// GetChainInfo 实现 persistence.QueryService 接口
func (m *MockQueryService) GetChainInfo(ctx context.Context) (*types.ChainInfo, error) {
	height, hash, err := m.GetHighestBlock(ctx)
	if err != nil {
		return &types.ChainInfo{
			Height:        0,
			BestBlockHash: nil,
			IsReady:       false,
			Status:        "error",
		}, nil
	}
	return &types.ChainInfo{
		Height:        height,
		BestBlockHash: hash,
		IsReady:       true,
		Status:        "normal",
	}, nil
}

// GetNodeMode 实现 persistence.QueryService 接口
func (m *MockQueryService) GetNodeMode(ctx context.Context) (types.NodeMode, error) {
	return types.NodeModeFull, nil
}

// GetResourceByContentHash 实现 persistence.QueryService 接口
func (m *MockQueryService) GetResourceByContentHash(ctx context.Context, contentHash []byte) (*pb_resource.Resource, error) {
	return nil, fmt.Errorf("not implemented")
}

// GetResourceFromBlockchain 实现 persistence.QueryService 接口
func (m *MockQueryService) GetResourceFromBlockchain(ctx context.Context, contentHash []byte) (*pb_resource.Resource, bool, error) {
	return nil, false, fmt.Errorf("not implemented")
}

// GetResourceTransaction 实现 persistence.QueryService 接口
func (m *MockQueryService) GetResourceTransaction(ctx context.Context, contentHash []byte) (txHash, blockHash []byte, blockHeight uint64, err error) {
	return nil, nil, 0, fmt.Errorf("not implemented")
}

// GetSyncStatus 实现 persistence.QueryService 接口
func (m *MockQueryService) GetSyncStatus(ctx context.Context) (*types.SystemSyncStatus, error) {
	return &types.SystemSyncStatus{
		Status: types.SyncStatusSynced,
	}, nil
}

// GetTransaction 实现 persistence.QueryService 接口
func (m *MockQueryService) GetTransaction(ctx context.Context, txHash []byte) (blockHash []byte, txIndex uint32, transaction *transaction.Transaction, err error) {
	return nil, 0, nil, fmt.Errorf("not implemented")
}

// GetTransactionsByBlock 实现 persistence.QueryService 接口
func (m *MockQueryService) GetTransactionsByBlock(ctx context.Context, blockHash []byte) ([]*transaction.Transaction, error) {
	block, err := m.GetBlockByHash(ctx, blockHash)
	if err != nil {
		return nil, err
	}
	return block.Body.Transactions, nil
}

// GetTxBlockHeight 实现 persistence.QueryService 接口
func (m *MockQueryService) GetTxBlockHeight(ctx context.Context, txHash []byte) (uint64, error) {
	_, _, tx, err := m.GetTransaction(ctx, txHash)
	if err != nil {
		return 0, err
	}
	_ = tx
	// 简化实现：返回0
	return 0, nil
}

// IsDataFresh 实现 persistence.QueryService 接口
func (m *MockQueryService) IsDataFresh(ctx context.Context) (bool, error) {
	return true, nil
}

// IsReady 实现 persistence.QueryService 接口
func (m *MockQueryService) IsReady(ctx context.Context) (bool, error) {
	return true, nil
}

// ListResourceHashes 实现 persistence.QueryService 接口
func (m *MockQueryService) ListResourceHashes(ctx context.Context, offset int, limit int) ([][]byte, error) {
	return nil, nil
}

// GetCurrentStateRoot 实现 persistence.UTXOQuery 接口
func (m *MockQueryService) GetCurrentStateRoot(ctx context.Context) ([]byte, error) {
	m.errMu.RLock()
	err := m.err
	m.errMu.RUnlock()
	if err != nil {
		return nil, err
	}
	return make([]byte, 32), nil
}

// SetError 设置错误（用于测试错误场景）
func (m *MockQueryService) SetError(err error) {
	m.errMu.Lock()
	defer m.errMu.Unlock()
	m.err = err
}

// GetUTXO 实现 persistence.UTXOQuery 接口
func (m *MockQueryService) GetUTXO(ctx context.Context, outpoint *transaction.OutPoint) (*utxopb.UTXO, error) {
	return nil, fmt.Errorf("not implemented")
}

// GetUTXOsByAddress 实现 persistence.UTXOQuery 接口
func (m *MockQueryService) GetUTXOsByAddress(ctx context.Context, address []byte, category *utxopb.UTXOCategory, onlyAvailable bool) ([]*utxopb.UTXO, error) {
	return nil, nil
}

// GetSponsorPoolUTXOs 实现 persistence.UTXOQuery 接口
func (m *MockQueryService) GetSponsorPoolUTXOs(ctx context.Context, onlyAvailable bool) ([]*utxopb.UTXO, error) {
	return nil, nil
}

// GetPricingState 实现 persistence.PricingQuery 接口
func (m *MockQueryService) GetPricingState(ctx context.Context, resourceHash []byte) (*types.ResourcePricingState, error) {
	return nil, fmt.Errorf("not implemented")
}

// SetBlock 设置测试区块
func (m *MockQueryService) SetBlock(hash []byte, block *core.Block) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blocks[string(hash)] = block
	// 自动填充 canonical 主链块映射：仅在该高度尚未设置时记录
	if block != nil && block.Header != nil {
		h := block.Header.Height
		if _, ok := m.blocksByHeight[h]; !ok {
			m.blocksByHeight[h] = block
			// 记录 canonical hash（供 GetHighestBlock/GetBestBlockHash 使用）
			if len(hash) > 0 {
				cpy := make([]byte, len(hash))
				copy(cpy, hash)
				m.hashByHeight[h] = cpy
			}
		}
	}
}

// MockDataWriter 模拟数据写入服务
type MockDataWriter struct {
	blocks          map[string]*core.Block
	mu              sync.RWMutex
	writeBlockErr   error
	writeBlockErrMu sync.RWMutex
}

// NewMockDataWriter 创建模拟数据写入服务
func NewMockDataWriter() *MockDataWriter {
	return &MockDataWriter{
		blocks: make(map[string]*core.Block),
	}
}

// WriteBlock 实现 persistence.DataWriter 接口
func (m *MockDataWriter) WriteBlock(ctx context.Context, block *core.Block) error {
	m.writeBlockErrMu.RLock()
	err := m.writeBlockErr
	m.writeBlockErrMu.RUnlock()
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	// 简化实现：使用高度作为key
	if block.Header != nil {
		key := fmt.Sprintf("block-%d", block.Header.Height)
		m.blocks[key] = block
	}
	return nil
}

// SetWriteBlockError 设置写入错误（用于测试）
func (m *MockDataWriter) SetWriteBlockError(err error) {
	m.writeBlockErrMu.Lock()
	defer m.writeBlockErrMu.Unlock()
	m.writeBlockErr = err
}

// WriteBlocks 实现 persistence.DataWriter 接口
func (m *MockDataWriter) WriteBlocks(ctx context.Context, blocks []*core.Block) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, block := range blocks {
		if block.Header != nil {
			key := fmt.Sprintf("block-%d", block.Header.Height)
			m.blocks[key] = block
		}
	}
	return nil
}

// DeleteBlockTransactionIndices 实现 persistence.DataWriter 接口
func (m *MockDataWriter) DeleteBlockTransactionIndices(ctx context.Context, block *core.Block) error {
	return nil
}

// GetBlock 获取写入的区块（用于测试验证）
func (m *MockDataWriter) GetBlock(height uint64) *core.Block {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := fmt.Sprintf("block-%d", height)
	return m.blocks[key]
}

// MockTxVerifier 模拟交易验证器
type MockTxVerifier struct {
	verifyFunc func(*transaction.Transaction) error
}

// NewMockTxVerifier 创建模拟交易验证器
func NewMockTxVerifier() *MockTxVerifier {
	return &MockTxVerifier{
		verifyFunc: func(tx *transaction.Transaction) error {
			return nil
		},
	}
}

// Verify 实现 txiface.TxVerifier 接口
func (m *MockTxVerifier) Verify(ctx context.Context, tx *transaction.Transaction) error {
	if m.verifyFunc != nil {
		return m.verifyFunc(tx)
	}
	return nil
}

// RegisterAuthZPlugin 实现 txiface.TxVerifier 接口
func (m *MockTxVerifier) RegisterAuthZPlugin(plugin txiface.AuthZPlugin) {
	// 简化实现：不做任何操作
}

// RegisterConservationPlugin 实现 txiface.TxVerifier 接口
func (m *MockTxVerifier) RegisterConservationPlugin(plugin txiface.ConservationPlugin) {
	// 简化实现：不做任何操作
}

// RegisterConditionPlugin 实现 txiface.TxVerifier 接口
func (m *MockTxVerifier) RegisterConditionPlugin(plugin txiface.ConditionPlugin) {
	// 简化实现：不做任何操作
}

// MockFeeManager 模拟费用管理器
type MockFeeManager struct{}

// CalculateTransactionFee 实现 txiface.FeeManager 接口
func (m *MockFeeManager) CalculateTransactionFee(ctx context.Context, tx *transaction.Transaction) (*txiface.AggregatedFees, error) {
	return &txiface.AggregatedFees{
		ByToken: make(map[txiface.TokenKey]*big.Int),
	}, nil
}

// AggregateFees 实现 txiface.FeeManager 接口
func (m *MockFeeManager) AggregateFees(fees []*txiface.AggregatedFees) *txiface.AggregatedFees {
	result := &txiface.AggregatedFees{
		ByToken: make(map[txiface.TokenKey]*big.Int),
	}
	for _, fee := range fees {
		if fee != nil {
			for token, amount := range fee.ByToken {
				if result.ByToken[token] == nil {
					result.ByToken[token] = big.NewInt(0)
				}
				result.ByToken[token].Add(result.ByToken[token], amount)
			}
		}
	}
	return result
}

// BuildCoinbase 实现 txiface.FeeManager 接口
func (m *MockFeeManager) BuildCoinbase(aggregatedFees *txiface.AggregatedFees, minerAddress []byte, chainID []byte) (*transaction.Transaction, error) {
	return &transaction.Transaction{
		Version:           1,
		Inputs:            []*transaction.TxInput{},
		Outputs:           []*transaction.TxOutput{},
		Nonce:             0,
		CreationTimestamp: uint64(time.Now().Unix()),
		ChainId:           chainID,
	}, nil
}

// ValidateCoinbase 实现 txiface.FeeManager 接口
func (m *MockFeeManager) ValidateCoinbase(ctx context.Context, coinbase *transaction.Transaction, expectedFees *txiface.AggregatedFees, minerAddr []byte) error {
	return nil
}

// MockUTXOWriter 模拟UTXO写入器
type MockUTXOWriter struct{}

// CreateUTXO 实现 eutxo.UTXOWriter 接口
func (m *MockUTXOWriter) CreateUTXO(ctx context.Context, utxo *utxopb.UTXO) error {
	return nil
}

// CreateUTXOInTransaction 实现 eutxo.UTXOWriter 接口
func (m *MockUTXOWriter) CreateUTXOInTransaction(ctx context.Context, tx storage.BadgerTransaction, utxoObj *utxopb.UTXO) error {
	return nil
}

// DeleteUTXO 实现 eutxo.UTXOWriter 接口
func (m *MockUTXOWriter) DeleteUTXO(ctx context.Context, outpoint *transaction.OutPoint) error {
	return nil
}

// DeleteUTXOInTransaction 实现 eutxo.UTXOWriter 接口
func (m *MockUTXOWriter) DeleteUTXOInTransaction(ctx context.Context, tx storage.BadgerTransaction, outpoint *transaction.OutPoint) error {
	return nil
}

// ReferenceUTXO 实现 eutxo.UTXOWriter 接口
func (m *MockUTXOWriter) ReferenceUTXO(ctx context.Context, outpoint *transaction.OutPoint) error {
	return nil
}

// UnreferenceUTXO 实现 eutxo.UTXOWriter 接口
func (m *MockUTXOWriter) UnreferenceUTXO(ctx context.Context, outpoint *transaction.OutPoint) error {
	return nil
}

// UpdateStateRoot 实现 eutxo.UTXOWriter 接口
func (m *MockUTXOWriter) UpdateStateRoot(ctx context.Context, stateRoot []byte) error {
	return nil
}

// UpdateStateRootInTransaction 实现 eutxo.UTXOWriter 接口
func (m *MockUTXOWriter) UpdateStateRootInTransaction(ctx context.Context, tx storage.BadgerTransaction, stateRoot []byte) error {
	return nil
}

// WriteUTXO 实现 eutxo.UTXOWriter 接口
func (m *MockUTXOWriter) WriteUTXO(ctx context.Context, utxo interface{}) error {
	return nil
}

// MockEventBus 模拟事件总线
type MockEventBus struct {
	events []interface{}
	mu     sync.RWMutex
}

// NewMockEventBus 创建模拟事件总线
func NewMockEventBus() *MockEventBus {
	return &MockEventBus{
		events: make([]interface{}, 0),
	}
}

// Publish 实现 event.EventBus 接口
func (m *MockEventBus) Publish(eventType event.EventType, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, args...)
}

// PublishEvent 实现 event.EventBus 接口
func (m *MockEventBus) PublishEvent(event event.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
}

// Subscribe 实现 event.EventBus 接口
func (m *MockEventBus) Subscribe(eventType event.EventType, handler interface{}) error {
	return nil
}

// SubscribeAsync 实现 event.EventBus 接口
func (m *MockEventBus) SubscribeAsync(eventType event.EventType, handler interface{}, transactional bool) error {
	return nil
}

// SubscribeOnce 实现 event.EventBus 接口
func (m *MockEventBus) SubscribeOnce(eventType event.EventType, handler interface{}) error {
	return nil
}

// SubscribeOnceAsync 实现 event.EventBus 接口
func (m *MockEventBus) SubscribeOnceAsync(eventType event.EventType, handler interface{}, transactional bool) error {
	return nil
}

// Unsubscribe 实现 event.EventBus 接口
func (m *MockEventBus) Unsubscribe(eventType event.EventType, handler interface{}) error {
	return nil
}

// WaitAsync 实现 event.EventBus 接口
func (m *MockEventBus) WaitAsync() {
	// 简化实现：不做任何操作
}

// PublishWESEvent 实现 event.EventBus 接口
func (m *MockEventBus) PublishWESEvent(event *types.WESEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	return nil
}

// SubscribeWithFilter 实现 event.EventBus 接口
func (m *MockEventBus) SubscribeWithFilter(eventType event.EventType, filter event.EventFilter, handler event.EventHandler) (types.SubscriptionID, error) {
	return types.SubscriptionID("mock-subscription-0"), nil
}

// SubscribeWESEvents 实现 event.EventBus 接口
func (m *MockEventBus) SubscribeWESEvents(protocols []event.ProtocolType, handler event.WESEventHandler) (types.SubscriptionID, error) {
	return types.SubscriptionID("mock-subscription-1"), nil
}

// UnsubscribeByID 实现 event.EventBus 接口
func (m *MockEventBus) UnsubscribeByID(id types.SubscriptionID) error {
	return nil
}

// UpdateConfig 实现 event.EventBus 接口
func (m *MockEventBus) UpdateConfig(config *types.EventBusConfig) error {
	return nil
}

// RegisterEventInterceptor 实现 event.EventBus 接口
func (m *MockEventBus) RegisterEventInterceptor(interceptor event.EventInterceptor) error {
	return nil
}

// UnregisterEventInterceptor 实现 event.EventBus 接口
func (m *MockEventBus) UnregisterEventInterceptor(interceptorID string) error {
	return nil
}

// EnableEventHistory 实现 event.EventBus 接口
func (m *MockEventBus) EnableEventHistory(eventType event.EventType, maxSize int) error {
	return nil
}

// GetConfig 实现 event.EventBus 接口
func (m *MockEventBus) GetConfig() (*types.EventBusConfig, error) {
	return &types.EventBusConfig{}, nil
}

// HasCallback 实现 event.EventBus 接口
func (m *MockEventBus) HasCallback(eventType event.EventType) bool {
	return false
}

// GetEventHistory 实现 event.EventBus 接口
func (m *MockEventBus) GetEventHistory(eventType event.EventType) []interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]interface{}{}, m.events...)
}

// GetActiveSubscriptions 实现 event.EventBus 接口
func (m *MockEventBus) GetActiveSubscriptions() ([]*types.SubscriptionInfo, error) {
	return nil, nil
}

// DisableEventHistory 实现 event.EventBus 接口
func (m *MockEventBus) DisableEventHistory(eventType event.EventType) error {
	return nil
}

// GetEvents 获取发布的事件（用于测试验证）
func (m *MockEventBus) GetEvents() []interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]interface{}{}, m.events...)
}

// ClearEvents 清空事件（用于测试清理）
func (m *MockEventBus) ClearEvents() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = m.events[:0]
}

// MockBlockValidator 模拟区块验证器
type MockBlockValidator struct {
	validateFunc func(context.Context, *core.Block) (bool, error)
	mu           sync.RWMutex
}

// NewMockBlockValidator 创建模拟区块验证器
func NewMockBlockValidator() *MockBlockValidator {
	return &MockBlockValidator{
		validateFunc: func(ctx context.Context, block *core.Block) (bool, error) {
			return true, nil
		},
	}
}

// SetValidateResult 设置验证结果（用于测试）
func (m *MockBlockValidator) SetValidateResult(valid bool, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.validateFunc = func(ctx context.Context, block *core.Block) (bool, error) {
		return valid, err
	}
}

// ValidateBlock 实现 interfaces.InternalBlockValidator 接口
func (m *MockBlockValidator) ValidateBlock(ctx context.Context, block *core.Block) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.validateFunc != nil {
		return m.validateFunc(ctx, block)
	}
	return true, nil
}

// GetValidatorMetrics 实现 interfaces.InternalBlockValidator 接口
func (m *MockBlockValidator) GetValidatorMetrics(ctx context.Context) (*interfaces.ValidatorMetrics, error) {
	return &interfaces.ValidatorMetrics{}, nil
}

// ValidateStructure 实现 interfaces.InternalBlockValidator 接口
func (m *MockBlockValidator) ValidateStructure(ctx context.Context, block *core.Block) error {
	valid, err := m.ValidateBlock(ctx, block)
	if err != nil {
		return err
	}
	if !valid {
		return fmt.Errorf("结构验证失败")
	}
	return nil
}

// ValidateConsensus 实现 interfaces.InternalBlockValidator 接口
func (m *MockBlockValidator) ValidateConsensus(ctx context.Context, block *core.Block) error {
	valid, err := m.ValidateBlock(ctx, block)
	if err != nil {
		return err
	}
	if !valid {
		return fmt.Errorf("共识验证失败")
	}
	return nil
}
