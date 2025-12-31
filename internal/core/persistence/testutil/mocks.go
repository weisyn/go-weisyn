// Package testutil 提供 Persistence 模块测试的辅助工具
//
// 🧪 **测试辅助工具包**
//
// 本包提供测试所需的 Mock 对象、测试数据和辅助函数，用于简化测试代码编写。
// 遵循 docs/system/standards/principles/testing-standards.md 规范。
package testutil

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/weisyn/v1/internal/core/eutxo/testutil"
	"github.com/weisyn/v1/internal/core/persistence/query/interfaces"
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	pb_resource "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
	"github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/types"
	"google.golang.org/grpc"
)

// ==================== Mock 对象 ====================

// MockLogger 统一的日志Mock实现（复用 eutxo/testutil）
type MockLogger = testutil.MockLogger

// BehavioralMockLogger 行为Mock日志（复用 eutxo/testutil）
type BehavioralMockLogger = testutil.BehavioralMockLogger

// MockBadgerStore 内存键值存储Mock（复用 eutxo/testutil）
type MockBadgerStore = testutil.MockBadgerStore

// MockHashManager 哈希管理器Mock（复用 eutxo/testutil）
type MockHashManager = testutil.MockHashManager

// MockFileStore 文件存储Mock实现
//
// ✅ **设计原则**：内存文件系统，支持基本文件操作
// 📋 **使用场景**：所有需要文件存储的测试用例
type MockFileStore struct {
	files   map[string][]byte
	mutex   sync.RWMutex
	fileInfos map[string]types.FileInfo
}

// NewMockFileStore 创建新的 MockFileStore
func NewMockFileStore() *MockFileStore {
	return &MockFileStore{
		files:     make(map[string][]byte),
		fileInfos: make(map[string]types.FileInfo),
	}
}

// Save 保存数据到指定路径
func (m *MockFileStore) Save(ctx context.Context, path string, data []byte) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.files[path] = data
	m.fileInfos[path] = types.FileInfo{
		Size:      int64(len(data)),
		CreateTime: time.Now(),
		ModTime:    time.Now(),
		IsDir:     false,
	}

	return nil
}

// Load 从指定路径加载数据
func (m *MockFileStore) Load(ctx context.Context, path string) ([]byte, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	data, exists := m.files[path]
	if !exists {
		return nil, errors.New("file not found")
	}

	return data, nil
}

// Delete 删除指定路径的文件
func (m *MockFileStore) Delete(ctx context.Context, path string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.files[path]; !exists {
		return errors.New("file not found")
	}

	delete(m.files, path)
	delete(m.fileInfos, path)
	return nil
}

// Exists 检查指定路径的文件是否存在
func (m *MockFileStore) Exists(ctx context.Context, path string) (bool, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	_, exists := m.files[path]
	return exists, nil
}

// FileInfo 获取文件信息
func (m *MockFileStore) FileInfo(ctx context.Context, path string) (types.FileInfo, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	info, exists := m.fileInfos[path]
	if !exists {
		return types.FileInfo{}, errors.New("file not found")
	}

	return info, nil
}

// ListFiles 列出指定目录下的所有文件
func (m *MockFileStore) ListFiles(ctx context.Context, dirPath string, pattern string) ([]string, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var files []string
	for path := range m.files {
		// 简单的目录匹配（实际实现应该更复杂）
		if dirPath == "" || len(path) >= len(dirPath) && path[:len(dirPath)] == dirPath {
			files = append(files, path)
		}
	}

	return files, nil
}

// MakeDir 创建目录
func (m *MockFileStore) MakeDir(ctx context.Context, dirPath string, recursive bool) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.fileInfos[dirPath] = types.FileInfo{
		Size:      0,
		CreateTime: time.Now(),
		ModTime:    time.Now(),
		IsDir:     true,
	}

	return nil
}

// DeleteDir 删除目录
func (m *MockFileStore) DeleteDir(ctx context.Context, dirPath string, recursive bool) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 删除目录下的所有文件
	for path := range m.files {
		if len(path) >= len(dirPath) && path[:len(dirPath)] == dirPath {
			delete(m.files, path)
			delete(m.fileInfos, path)
		}
	}

	delete(m.fileInfos, dirPath)
	return nil
}

// Copy 复制文件
func (m *MockFileStore) Copy(ctx context.Context, srcPath, dstPath string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	data, exists := m.files[srcPath]
	if !exists {
		return errors.New("file not found")
	}

	m.files[dstPath] = data
	m.fileInfos[dstPath] = types.FileInfo{
		Size:      int64(len(data)),
		CreateTime: time.Now(),
		ModTime:    time.Now(),
		IsDir:     false,
	}

	return nil
}

// Move 移动文件
func (m *MockFileStore) Move(ctx context.Context, srcPath, dstPath string) error {
	if err := m.Copy(ctx, srcPath, dstPath); err != nil {
		return err
	}
	return m.Delete(ctx, srcPath)
}

// OpenReadStream 打开读取流
func (m *MockFileStore) OpenReadStream(ctx context.Context, path string) (io.ReadCloser, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	data, exists := m.files[path]
	if !exists {
		return nil, errors.New("file not found")
	}

	return &mockReadCloser{data: data}, nil
}

// OpenWriteStream 打开写入流
func (m *MockFileStore) OpenWriteStream(ctx context.Context, path string) (io.WriteCloser, error) {
	return &mockWriteCloser{
		store: m,
		path:  path,
		ctx:   ctx,
	}, nil
}

// mockReadCloser 模拟读取流
type mockReadCloser struct {
	data []byte
	pos  int
}

func (m *mockReadCloser) Read(p []byte) (n int, err error) {
	if m.pos >= len(m.data) {
		return 0, io.EOF
	}
	n = copy(p, m.data[m.pos:])
	m.pos += n
	return n, nil
}

func (m *mockReadCloser) Close() error {
	return nil
}

// mockWriteCloser 模拟写入流
type mockWriteCloser struct {
	store *MockFileStore
	path  string
	ctx   context.Context
	buf   []byte
}

func (m *mockWriteCloser) Write(p []byte) (n int, err error) {
	m.buf = append(m.buf, p...)
	return len(p), nil
}

func (m *mockWriteCloser) Close() error {
	return m.store.Save(m.ctx, m.path, m.buf)
}

// MockBlockHashServiceClient 区块哈希服务客户端Mock
//
// ✅ **设计原则**：使用 sha256 计算区块哈希
// 📋 **使用场景**：所有需要区块哈希服务的测试用例
type MockBlockHashServiceClient struct{}

// ComputeBlockHash 计算区块哈希
func (m *MockBlockHashServiceClient) ComputeBlockHash(ctx context.Context, in *core.ComputeBlockHashRequest, opts ...grpc.CallOption) (*core.ComputeBlockHashResponse, error) {
	// 简化实现：使用区块高度的哈希
	hasher := sha256.New()
	if in.Block != nil && in.Block.Header != nil {
		hasher.Write([]byte{byte(in.Block.Header.Height)})
		if in.Block.Header.PreviousHash != nil {
			hasher.Write(in.Block.Header.PreviousHash)
		}
	}
	hash := hasher.Sum(nil)
	return &core.ComputeBlockHashResponse{
		Hash:     hash,
		IsValid:  true,
	}, nil
}

// ValidateBlockHash 验证区块哈希
func (m *MockBlockHashServiceClient) ValidateBlockHash(ctx context.Context, in *core.ValidateBlockHashRequest, opts ...grpc.CallOption) (*core.ValidateBlockHashResponse, error) {
	resp, err := m.ComputeBlockHash(ctx, &core.ComputeBlockHashRequest{Block: in.Block})
	if err != nil {
		return nil, err
	}
	isValid := len(in.ExpectedHash) == len(resp.Hash)
	return &core.ValidateBlockHashResponse{
		IsValid:      isValid,
		ComputedHash: resp.Hash,
		ExpectedHash: in.ExpectedHash,
	}, nil
}

// MockTransactionHashServiceClient 交易哈希服务客户端Mock
//
// ✅ **设计原则**：使用 sha256 计算交易哈希
// 📋 **使用场景**：所有需要交易哈希服务的测试用例
type MockTransactionHashServiceClient struct{}

// ComputeHash 计算交易哈希
func (m *MockTransactionHashServiceClient) ComputeHash(ctx context.Context, in *transaction.ComputeHashRequest, opts ...grpc.CallOption) (*transaction.ComputeHashResponse, error) {
	// 简化实现：使用交易数据的哈希
	hasher := sha256.New()
	if in.Transaction != nil {
		hasher.Write([]byte("tx"))
	}
	hash := hasher.Sum(nil)
	return &transaction.ComputeHashResponse{
		Hash:    hash,
		IsValid: true,
	}, nil
}

// ValidateHash 验证交易哈希
func (m *MockTransactionHashServiceClient) ValidateHash(ctx context.Context, in *transaction.ValidateHashRequest, opts ...grpc.CallOption) (*transaction.ValidateHashResponse, error) {
	resp, err := m.ComputeHash(ctx, &transaction.ComputeHashRequest{Transaction: in.Transaction})
	if err != nil {
		return nil, err
	}
	isValid := len(in.ExpectedHash) == len(resp.Hash)
	return &transaction.ValidateHashResponse{
		IsValid:      isValid,
		ComputedHash: resp.Hash,
		ExpectedHash: in.ExpectedHash,
	}, nil
}

// ComputeSignatureHash 计算签名哈希
func (m *MockTransactionHashServiceClient) ComputeSignatureHash(ctx context.Context, in *transaction.ComputeSignatureHashRequest, opts ...grpc.CallOption) (*transaction.ComputeSignatureHashResponse, error) {
	// 简化实现：使用交易哈希
	resp, err := m.ComputeHash(ctx, &transaction.ComputeHashRequest{Transaction: in.Transaction})
	if err != nil {
		return nil, err
	}
	return &transaction.ComputeSignatureHashResponse{
		Hash:    resp.Hash,
		IsValid: true,
	}, nil
}

// ValidateSignatureHash 验证签名哈希
func (m *MockTransactionHashServiceClient) ValidateSignatureHash(ctx context.Context, in *transaction.ValidateSignatureHashRequest, opts ...grpc.CallOption) (*transaction.ValidateSignatureHashResponse, error) {
	resp, err := m.ComputeSignatureHash(ctx, &transaction.ComputeSignatureHashRequest{Transaction: in.Transaction})
	if err != nil {
		return nil, err
	}
	isValid := len(in.ExpectedHash) == len(resp.Hash)
	return &transaction.ValidateSignatureHashResponse{
		IsValid:      isValid,
		ComputedHash: resp.Hash,
		ExpectedHash: in.ExpectedHash,
	}, nil
}

// ==================== Mock 子查询服务 ====================

// MockInternalChainQuery 内部链查询服务Mock
type MockInternalChainQuery struct{}

func (m *MockInternalChainQuery) GetChainInfo(ctx context.Context) (*types.ChainInfo, error) {
	return &types.ChainInfo{
		Height:        0,
		BestBlockHash: make([]byte, 32),
		NodeMode:      types.NodeModeFull,
	}, nil
}

func (m *MockInternalChainQuery) GetCurrentHeight(ctx context.Context) (uint64, error) {
	return 0, nil
}

func (m *MockInternalChainQuery) GetBestBlockHash(ctx context.Context) ([]byte, error) {
	return make([]byte, 32), nil
}

func (m *MockInternalChainQuery) GetNodeMode(ctx context.Context) (types.NodeMode, error) {
	return types.NodeModeFull, nil
}

func (m *MockInternalChainQuery) IsDataFresh(ctx context.Context) (bool, error) {
	return true, nil
}

func (m *MockInternalChainQuery) IsReady(ctx context.Context) (bool, error) {
	return true, nil
}

func (m *MockInternalChainQuery) GetSyncStatus(ctx context.Context) (*types.SystemSyncStatus, error) {
	return &types.SystemSyncStatus{
		Status:        types.SyncStatusSyncing,
		CurrentHeight: 0,
		NetworkHeight: 0,
		SyncProgress:  0.0,
	}, nil
}

func (m *MockInternalChainQuery) GetQueryMetrics(ctx context.Context) (*interfaces.QueryMetrics, error) {
	return &interfaces.QueryMetrics{
		QueryCount:   0,
		SuccessCount: 0,
		FailureCount: 0,
		IsHealthy:    true,
	}, nil
}

// MockInternalBlockQuery 内部区块查询服务Mock
type MockInternalBlockQuery struct{}

func (m *MockInternalBlockQuery) GetBlockByHeight(ctx context.Context, height uint64) (*core.Block, error) {
	return &core.Block{
		Header: &core.BlockHeader{
			Height: height,
		},
	}, nil
}

func (m *MockInternalBlockQuery) GetBlockByHash(ctx context.Context, blockHash []byte) (*core.Block, error) {
	return &core.Block{
		Header: &core.BlockHeader{},
	}, nil
}

func (m *MockInternalBlockQuery) GetBlockHeader(ctx context.Context, blockHash []byte) (*core.BlockHeader, error) {
	return &core.BlockHeader{}, nil
}

func (m *MockInternalBlockQuery) GetBlockRange(ctx context.Context, startHeight, endHeight uint64) ([]*core.Block, error) {
	return []*core.Block{}, nil
}

func (m *MockInternalBlockQuery) GetHighestBlock(ctx context.Context) (height uint64, blockHash []byte, err error) {
	return 0, make([]byte, 32), nil
}

// MockInternalTxQuery 内部交易查询服务Mock
type MockInternalTxQuery struct{}

func (m *MockInternalTxQuery) GetTransaction(ctx context.Context, txHash []byte) (blockHash []byte, txIndex uint32, tx *transaction.Transaction, err error) {
	return make([]byte, 32), 0, &transaction.Transaction{}, nil
}

func (m *MockInternalTxQuery) GetTxBlockHeight(ctx context.Context, txHash []byte) (uint64, error) {
	return 0, nil
}

func (m *MockInternalTxQuery) GetBlockTimestamp(ctx context.Context, height uint64) (int64, error) {
	return time.Now().Unix(), nil
}

func (m *MockInternalTxQuery) GetAccountNonce(ctx context.Context, address []byte) (uint64, error) {
	return 0, nil
}

func (m *MockInternalTxQuery) GetTransactionsByBlock(ctx context.Context, blockHash []byte) ([]*transaction.Transaction, error) {
	return []*transaction.Transaction{}, nil
}

// MockInternalUTXOQuery 内部UTXO查询服务Mock
type MockInternalUTXOQuery struct{}

func (m *MockInternalUTXOQuery) GetUTXO(ctx context.Context, outpoint *transaction.OutPoint) (*utxo.UTXO, error) {
	return &utxo.UTXO{}, nil
}

func (m *MockInternalUTXOQuery) GetUTXOsByAddress(ctx context.Context, address []byte, category *utxo.UTXOCategory, onlyAvailable bool) ([]*utxo.UTXO, error) {
	return []*utxo.UTXO{}, nil
}

func (m *MockInternalUTXOQuery) GetSponsorPoolUTXOs(ctx context.Context, onlyAvailable bool) ([]*utxo.UTXO, error) {
	return []*utxo.UTXO{}, nil
}

func (m *MockInternalUTXOQuery) GetCurrentStateRoot(ctx context.Context) ([]byte, error) {
	return make([]byte, 32), nil
}

func (m *MockInternalUTXOQuery) CheckAssetUTXOConsistency(ctx context.Context) (bool, error) {
	return false, nil // 默认返回一致（inconsistent=false）
}

func (m *MockInternalUTXOQuery) RunAssetUTXORepair(ctx context.Context, dryRun bool) error {
	return nil // 默认成功
}

// MockInternalResourceQuery 内部资源查询服务Mock
type MockInternalResourceQuery struct{}

func (m *MockInternalResourceQuery) GetResourceByContentHash(ctx context.Context, contentHash []byte) (*pb_resource.Resource, error) {
	return &pb_resource.Resource{}, nil
}

func (m *MockInternalResourceQuery) GetResourceFromBlockchain(ctx context.Context, contentHash []byte) (*pb_resource.Resource, bool, error) {
	return &pb_resource.Resource{}, false, nil
}

func (m *MockInternalResourceQuery) GetResourceTransaction(ctx context.Context, contentHash []byte) (txHash, blockHash []byte, blockHeight uint64, err error) {
	return make([]byte, 32), make([]byte, 32), 0, nil
}

func (m *MockInternalResourceQuery) CheckFileExists(contentHash []byte) bool {
	return false
}

func (m *MockInternalResourceQuery) BuildFilePath(contentHash []byte) string {
	return ""
}

func (m *MockInternalResourceQuery) ListResourceHashes(ctx context.Context, offset int, limit int) ([][]byte, error) {
	return [][]byte{}, nil
}

func (m *MockInternalResourceQuery) GetResourceByInstance(ctx context.Context, txHash []byte, outputIndex uint32) (*pb_resource.Resource, bool, error) {
	return &pb_resource.Resource{}, false, nil
}

func (m *MockInternalResourceQuery) ListResourceInstancesByCode(ctx context.Context, contentHash []byte) ([]*transaction.OutPoint, error) {
	return []*transaction.OutPoint{}, nil
}

// MockInternalAccountQuery 内部账户查询服务Mock
type MockInternalAccountQuery struct{}

func (m *MockInternalAccountQuery) GetAccountBalance(ctx context.Context, address []byte, tokenID []byte) (*types.BalanceInfo, error) {
	return &types.BalanceInfo{
		Address: &transaction.Address{},
		TokenID: tokenID,
		Available: 0,
		Locked: 0,
		Pending: 0,
		Total: 0,
	}, nil
}

// MockInternalPricingQuery 内部定价查询服务Mock
type MockInternalPricingQuery struct{}

func (m *MockInternalPricingQuery) GetPricingState(ctx context.Context, resourceHash []byte) (*types.ResourcePricingState, error) {
	return nil, nil
}

