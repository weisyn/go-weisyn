// Package testutil 提供 Consensus 模块测试的辅助工具
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
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	peer "github.com/libp2p/go-libp2p/core/peer"

	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	pb_resource "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
	utxo "github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/network"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== Mock 对象 ====================

// MockLogger 统一的日志Mock实现
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

func (m *MockEventBus) Publish(eventType event.EventType, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, args...)
}

func (m *MockEventBus) PublishEvent(event event.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
}

func (m *MockEventBus) Subscribe(eventType event.EventType, handler interface{}) error {
	return nil
}

func (m *MockEventBus) SubscribeAsync(eventType event.EventType, handler interface{}, transactional bool) error {
	return nil
}

func (m *MockEventBus) SubscribeOnce(eventType event.EventType, handler interface{}) error {
	return nil
}

func (m *MockEventBus) SubscribeOnceAsync(eventType event.EventType, handler interface{}, transactional bool) error {
	return nil
}

func (m *MockEventBus) Unsubscribe(eventType event.EventType, handler interface{}) error {
	return nil
}

func (m *MockEventBus) WaitAsync() {}

func (m *MockEventBus) PublishWESEvent(event *types.WESEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	return nil
}

func (m *MockEventBus) SubscribeWithFilter(eventType event.EventType, filter event.EventFilter, handler event.EventHandler) (types.SubscriptionID, error) {
	return types.SubscriptionID("mock-subscription-0"), nil
}

func (m *MockEventBus) SubscribeWESEvents(protocols []event.ProtocolType, handler event.WESEventHandler) (types.SubscriptionID, error) {
	return types.SubscriptionID("mock-subscription-1"), nil
}

func (m *MockEventBus) UnsubscribeByID(id types.SubscriptionID) error {
	return nil
}

func (m *MockEventBus) UpdateConfig(config *types.EventBusConfig) error {
	return nil
}

func (m *MockEventBus) RegisterEventInterceptor(interceptor event.EventInterceptor) error {
	return nil
}

func (m *MockEventBus) UnregisterEventInterceptor(interceptorID string) error {
	return nil
}

func (m *MockEventBus) EnableEventHistory(eventType event.EventType, maxSize int) error {
	return nil
}

func (m *MockEventBus) GetConfig() (*types.EventBusConfig, error) {
	return &types.EventBusConfig{}, nil
}

func (m *MockEventBus) HasCallback(eventType event.EventType) bool {
	return false
}

func (m *MockEventBus) GetEventHistory(eventType event.EventType) []interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]interface{}{}, m.events...)
}

func (m *MockEventBus) GetActiveSubscriptions() ([]*types.SubscriptionInfo, error) {
	return nil, nil
}

func (m *MockEventBus) DisableEventHistory(eventType event.EventType) error {
	return nil
}

// MockHashManager 模拟哈希管理器
type MockHashManager struct{}

func (m *MockHashManager) SHA256(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:]
}

func (m *MockHashManager) Keccak256(data []byte) []byte {
	return m.SHA256(data)
}

func (m *MockHashManager) RIPEMD160(data []byte) []byte {
	result := make([]byte, 20)
	copy(result, data)
	if len(result) > 20 {
		result = result[:20]
	}
	return result
}

func (m *MockHashManager) DoubleSHA256(data []byte) []byte {
	first := sha256.Sum256(data)
	second := sha256.Sum256(first[:])
	return second[:]
}

func (m *MockHashManager) NewSHA256Hasher() hash.Hash {
	return sha256.New()
}

func (m *MockHashManager) NewRIPEMD160Hasher() hash.Hash {
	return sha256.New()
}

// MockPOWEngine 模拟POW引擎
type MockPOWEngine struct {
	mineError    error
	verifyError  error
	verifyResult bool
}

// NewMockPOWEngine 创建模拟POW引擎
func NewMockPOWEngine() *MockPOWEngine {
	return &MockPOWEngine{
		verifyResult: true,
	}
}

// SetMineError 设置挖矿错误
func (m *MockPOWEngine) SetMineError(err error) {
	m.mineError = err
}

// SetVerifyError 设置验证错误
func (m *MockPOWEngine) SetVerifyError(err error) {
	m.verifyError = err
}

// SetVerifyResult 设置验证结果
func (m *MockPOWEngine) SetVerifyResult(result bool) {
	m.verifyResult = result
}

func (m *MockPOWEngine) MineBlockHeader(ctx context.Context, header *core.BlockHeader) (*core.BlockHeader, error) {
	if m.mineError != nil {
		return nil, m.mineError
	}
	if header == nil {
		return nil, nil
	}
	minedHeader := *header
	minedHeader.Nonce = []byte{0x01, 0x02, 0x03, 0x04}
	return &minedHeader, nil
}

func (m *MockPOWEngine) VerifyBlockHeader(header *core.BlockHeader) (bool, error) {
	if m.verifyError != nil {
		return false, m.verifyError
	}
	return m.verifyResult, nil
}

// MockMemoryStore 模拟内存存储
type MockMemoryStore struct {
	data map[string][]byte
	mu   sync.RWMutex
}

// NewMockMemoryStore 创建模拟内存存储
func NewMockMemoryStore() *MockMemoryStore {
	return &MockMemoryStore{
		data: make(map[string][]byte),
	}
}

func (m *MockMemoryStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.data[key]
	return val, ok, nil
}

func (m *MockMemoryStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return nil
}

func (m *MockMemoryStore) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

func (m *MockMemoryStore) Exists(ctx context.Context, key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.data[key]
	return ok, nil
}

func (m *MockMemoryStore) GetMany(ctx context.Context, keys []string) (map[string][]byte, error) {
	result := make(map[string][]byte)
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, key := range keys {
		if val, ok := m.data[key]; ok {
			result[key] = val
		}
	}
	return result, nil
}

func (m *MockMemoryStore) SetMany(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range items {
		m.data[k] = v
	}
	return nil
}

func (m *MockMemoryStore) DeleteMany(ctx context.Context, keys []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, key := range keys {
		delete(m.data, key)
	}
	return nil
}

func (m *MockMemoryStore) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string][]byte)
	return nil
}

func (m *MockMemoryStore) DeleteByPattern(ctx context.Context, pattern string) (int64, error) {
	return 0, nil
}

func (m *MockMemoryStore) GetKeys(ctx context.Context, pattern string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var keys []string
	for k := range m.data {
		keys = append(keys, k)
	}
	return keys, nil
}

func (m *MockMemoryStore) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	return 0, nil
}

func (m *MockMemoryStore) UpdateTTL(ctx context.Context, key string, ttl time.Duration) error {
	return nil
}

func (m *MockMemoryStore) Count(ctx context.Context) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return int64(len(m.data)), nil
}

// MockNetwork 模拟网络服务
type MockNetwork struct{}

func (m *MockNetwork) RegisterStreamHandler(protoID string, handler network.MessageHandler, opts ...network.RegisterOption) error {
	return nil
}

func (m *MockNetwork) UnregisterStreamHandler(protoID string) error {
	return nil
}

func (m *MockNetwork) Subscribe(topic string, handler network.SubscribeHandler, opts ...network.SubscribeOption) (func() error, error) {
	return func() error { return nil }, nil
}

func (m *MockNetwork) Call(ctx context.Context, to peer.ID, protoID string, req []byte, opts *types.TransportOptions) ([]byte, error) {
	return nil, nil
}

func (m *MockNetwork) OpenStream(ctx context.Context, to peer.ID, protoID string, opts *types.TransportOptions) (network.StreamHandle, error) {
	return nil, nil
}

func (m *MockNetwork) Publish(ctx context.Context, topic string, data []byte, opts *types.PublishOptions) error {
	return nil
}

func (m *MockNetwork) ListProtocols() []types.ProtocolInfo {
	return nil
}

func (m *MockNetwork) GetProtocolInfo(protoID string) *types.ProtocolInfo {
	return nil
}

func (m *MockNetwork) GetTopicPeers(topic string) []peer.ID {
	return nil
}

func (m *MockNetwork) IsSubscribed(topic string) bool {
	return false
}

func (m *MockNetwork) CheckProtocolSupport(ctx context.Context, peerID peer.ID, protocol string) (bool, error) {
	return false, nil
}

// MockQueryService 模拟查询服务
type MockQueryService struct {
	blocks map[string]*core.Block
	mu     sync.RWMutex
	err    error
	errMu  sync.RWMutex
}

// NewMockQueryService 创建模拟查询服务
func NewMockQueryService() *MockQueryService {
	return &MockQueryService{
		blocks: make(map[string]*core.Block),
	}
}

func (m *MockQueryService) GetBlockByHash(ctx context.Context, hash []byte) (*core.Block, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	block, ok := m.blocks[string(hash)]
	if !ok {
		return nil, nil
	}
	return block, nil
}

func (m *MockQueryService) GetBlockByHeight(ctx context.Context, height uint64) (*core.Block, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, block := range m.blocks {
		if block.Header != nil && block.Header.Height == height {
			return block, nil
		}
	}
	return nil, nil
}

func (m *MockQueryService) GetCurrentHeight(ctx context.Context) (uint64, error) {
	m.errMu.RLock()
	err := m.err
	m.errMu.RUnlock()
	if err != nil {
		return 0, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var maxHeight uint64
	for _, block := range m.blocks {
		if block.Header != nil && block.Header.Height > maxHeight {
			maxHeight = block.Header.Height
		}
	}
	return maxHeight, nil
}

func (m *MockQueryService) GetBestBlockHash(ctx context.Context) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var bestBlock *core.Block
	var maxHeight uint64
	for _, block := range m.blocks {
		if block.Header != nil && block.Header.Height > maxHeight {
			maxHeight = block.Header.Height
			bestBlock = block
		}
	}
	if bestBlock == nil {
		return nil, nil
	}
	return make([]byte, 32), nil
}

func (m *MockQueryService) GetChainInfo(ctx context.Context) (*types.ChainInfo, error) {
	height, err := m.GetCurrentHeight(ctx)
	if err != nil {
		return nil, err
	}
	hash, _ := m.GetBestBlockHash(ctx)
	return &types.ChainInfo{
		Height:        height,
		BestBlockHash: hash,
		IsReady:       true,
		Status:        "normal",
	}, nil
}

func (m *MockQueryService) SetError(err error) {
	m.errMu.Lock()
	defer m.errMu.Unlock()
	m.err = err
}

func (m *MockQueryService) SetBlock(hash []byte, block *core.Block) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blocks[string(hash)] = block
}

func (m *MockQueryService) BuildFilePath(contentHash []byte) string {
	// Mock实现：返回一个简单的测试路径
	return fmt.Sprintf("/mock/path/%x", contentHash)
}

func (m *MockQueryService) CheckFileExists(contentHash []byte) bool {
	// Mock实现：默认返回false（文件不存在）
	return false
}

func (m *MockQueryService) ListResourceHashes(ctx context.Context, offset int, limit int) ([][]byte, error) {
	// Mock实现：返回空列表
	return [][]byte{}, nil
}

// ChainQuery 接口方法
func (m *MockQueryService) GetNodeMode(ctx context.Context) (types.NodeMode, error) {
	return types.NodeModeFull, nil
}

func (m *MockQueryService) IsDataFresh(ctx context.Context) (bool, error) {
	return true, nil
}

func (m *MockQueryService) IsReady(ctx context.Context) (bool, error) {
	return true, nil
}

func (m *MockQueryService) GetSyncStatus(ctx context.Context) (*types.SystemSyncStatus, error) {
	height, _ := m.GetCurrentHeight(ctx)
	return &types.SystemSyncStatus{
		CurrentHeight: height,
		NetworkHeight: height,
		Status:        types.SyncStatusSynced,
		SyncProgress:  1.0,
	}, nil
}

// BlockQuery 接口方法
func (m *MockQueryService) GetBlockHeader(ctx context.Context, blockHash []byte) (*core.BlockHeader, error) {
	block, err := m.GetBlockByHash(ctx, blockHash)
	if err != nil || block == nil {
		return nil, err
	}
	return block.Header, nil
}

func (m *MockQueryService) GetBlockRange(ctx context.Context, startHeight, endHeight uint64) ([]*core.Block, error) {
	var blocks []*core.Block
	for h := startHeight; h <= endHeight; h++ {
		block, err := m.GetBlockByHeight(ctx, h)
		if err != nil {
			return nil, err
		}
		if block != nil {
			blocks = append(blocks, block)
		}
	}
	return blocks, nil
}

func (m *MockQueryService) GetHighestBlock(ctx context.Context) (uint64, []byte, error) {
	height, err := m.GetCurrentHeight(ctx)
	if err != nil {
		return 0, nil, err
	}
	hash, err := m.GetBestBlockHash(ctx)
	return height, hash, err
}

// TxQuery 接口方法
func (m *MockQueryService) GetTransaction(ctx context.Context, txHash []byte) ([]byte, uint32, *transaction.Transaction, error) {
	// Mock实现：返回nil
	return nil, 0, nil, nil
}

func (m *MockQueryService) GetTxBlockHeight(ctx context.Context, txHash []byte) (uint64, error) {
	// Mock实现：返回0
	return 0, nil
}

func (m *MockQueryService) GetBlockTimestamp(ctx context.Context, height uint64) (int64, error) {
	block, err := m.GetBlockByHeight(ctx, height)
	if err != nil || block == nil || block.Header == nil {
		return 0, err
	}
	return int64(block.Header.Timestamp), nil
}

func (m *MockQueryService) GetAccountNonce(ctx context.Context, address []byte) (uint64, error) {
	// Mock实现：返回0
	return 0, nil
}

func (m *MockQueryService) GetTransactionsByBlock(ctx context.Context, blockHash []byte) ([]*transaction.Transaction, error) {
	block, err := m.GetBlockByHash(ctx, blockHash)
	if err != nil || block == nil || block.Body == nil {
		return nil, err
	}
	return block.Body.Transactions, nil
}

// UTXOQuery 接口方法
func (m *MockQueryService) GetUTXO(ctx context.Context, outpoint *transaction.OutPoint) (*utxo.UTXO, error) {
	// Mock实现：返回nil
	return nil, nil
}

func (m *MockQueryService) GetUTXOsByAddress(ctx context.Context, address []byte, category *utxo.UTXOCategory, onlyAvailable bool) ([]*utxo.UTXO, error) {
	// Mock实现：返回空列表
	return []*utxo.UTXO{}, nil
}

func (m *MockQueryService) GetSponsorPoolUTXOs(ctx context.Context, onlyAvailable bool) ([]*utxo.UTXO, error) {
	// Mock实现：返回空列表
	return []*utxo.UTXO{}, nil
}

func (m *MockQueryService) GetCurrentStateRoot(ctx context.Context) ([]byte, error) {
	// Mock实现：返回空哈希
	return make([]byte, 32), nil
}

// ResourceQuery 接口方法
func (m *MockQueryService) GetResourceByContentHash(ctx context.Context, contentHash []byte) (*pb_resource.Resource, error) {
	// Mock实现：返回nil
	return nil, nil
}

func (m *MockQueryService) GetResourceFromBlockchain(ctx context.Context, contentHash []byte) (*pb_resource.Resource, bool, error) {
	// Mock实现：返回nil和false
	return nil, false, nil
}

func (m *MockQueryService) GetResourceTransaction(ctx context.Context, contentHash []byte) ([]byte, []byte, uint64, error) {
	// Mock实现：返回nil和0
	return nil, nil, 0, nil
}

// AccountQuery 接口方法
func (m *MockQueryService) GetAccountBalance(ctx context.Context, address []byte, tokenID []byte) (*types.BalanceInfo, error) {
	// Mock实现：返回零余额
	addr := &transaction.Address{
		RawHash: address,
	}
	return &types.BalanceInfo{
		Address:            addr,
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

// PricingQuery 接口方法
func (m *MockQueryService) GetPricingState(ctx context.Context, resourceHash []byte) (*types.ResourcePricingState, error) {
	// Mock实现：返回nil
	return nil, nil
}

// MockCandidatePool 模拟候选池
type MockCandidatePool struct {
	candidates []*core.Block
	mu         sync.RWMutex
}

// NewMockCandidatePool 创建模拟候选池
func NewMockCandidatePool() *MockCandidatePool {
	return &MockCandidatePool{
		candidates: make([]*core.Block, 0),
	}
}

func (m *MockCandidatePool) SubmitCandidate(ctx context.Context, block *core.Block) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.candidates = append(m.candidates, block)
	return nil
}

func (m *MockCandidatePool) GetCandidates(ctx context.Context, height uint64) ([]*core.Block, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*core.Block
	for _, block := range m.candidates {
		if block.Header != nil && block.Header.Height == height {
			result = append(result, block)
		}
	}
	return result, nil
}

func (m *MockCandidatePool) RemoveCandidate(ctx context.Context, blockHash []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, block := range m.candidates {
		if block.Header != nil {
			// 简化实现：假设匹配
			m.candidates = append(m.candidates[:i], m.candidates[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *MockCandidatePool) ClearCandidates(ctx context.Context, height uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var filtered []*core.Block
	for _, block := range m.candidates {
		if block.Header == nil || block.Header.Height != height {
			filtered = append(filtered, block)
		}
	}
	m.candidates = filtered
	return nil
}

// MockRoutingTableManager 模拟路由表管理器
type MockRoutingTableManager struct{}

func (m *MockRoutingTableManager) GetRoutingTable() *types.RoutingTable {
	return &types.RoutingTable{
		Buckets:    []*types.Bucket{},
		LocalID:    "",
		BucketSize: 20,
		TableSize:  0,
		UpdatedAt:  types.Timestamp(time.Time{}),
	}
}

func (m *MockRoutingTableManager) AddPeer(ctx context.Context, addrInfo peer.AddrInfo) (bool, error) {
	return true, nil
}

func (m *MockRoutingTableManager) RemovePeer(peerID peer.ID) error {
	return nil
}

func (m *MockRoutingTableManager) FindClosestPeers(target []byte, count int) []peer.ID {
	return nil
}

func (m *MockRoutingTableManager) RecordPeerSuccess(peerID peer.ID) {
	// 无操作
}

func (m *MockRoutingTableManager) RecordPeerFailure(peerID peer.ID) {
	// 无操作
}

// MockDistanceCalculator 模拟距离计算器
type MockDistanceCalculator struct{}

func (m *MockDistanceCalculator) CalculateDistance(peer1, peer2 string) ([]byte, error) {
	return make([]byte, 32), nil
}

// MockSignatureManager 模拟签名管理器
type MockSignatureManager struct{}

func (m *MockSignatureManager) Sign(data []byte, privateKey []byte) ([]byte, error) {
	return make([]byte, 64), nil
}

func (m *MockSignatureManager) Verify(data, signature, publicKey []byte) (bool, error) {
	return true, nil
}

// MockKeyManager 模拟密钥管理器
type MockKeyManager struct{}

func (m *MockKeyManager) GenerateKeyPair() ([]byte, []byte, error) {
	return make([]byte, 32), make([]byte, 33), nil
}

func (m *MockKeyManager) GetPublicKey(privateKey []byte) ([]byte, error) {
	return make([]byte, 33), nil
}

// MockMerkleTreeManager 模拟Merkle树管理器
type MockMerkleTreeManager struct{}

func (m *MockMerkleTreeManager) NewMerkleTree(data [][]byte) (crypto.MerkleTree, error) {
	return &MockMerkleTree{}, nil
}

func (m *MockMerkleTreeManager) Verify(tree crypto.MerkleTree, data []byte) bool {
	return true
}

func (m *MockMerkleTreeManager) VerifyProof(tree crypto.MerkleTree, data []byte, proof [][]byte, rootHash []byte) bool {
	return true
}

func (m *MockMerkleTreeManager) GetProof(tree crypto.MerkleTree, data []byte) ([][]byte, error) {
	return nil, nil
}

// MockMerkleTree 模拟Merkle树
type MockMerkleTree struct{}

func (m *MockMerkleTree) GetRoot() []byte {
	return make([]byte, 32)
}

func (m *MockMerkleTree) GetLeaves() [][]byte {
	return nil
}

func (m *MockMerkleTree) Verify(data []byte) bool {
	return true
}

func (m *MockMerkleTree) VerifyProof(data []byte, proof [][]byte, rootHash []byte) bool {
	return true
}

func (m *MockMerkleTree) GetProof(data []byte) ([][]byte, error) {
	return nil, nil
}

// MockTransactionHashClient 模拟交易哈希客户端
type MockTransactionHashClient struct{}

func (m *MockTransactionHashClient) ComputeHash(ctx context.Context, in *transaction.ComputeHashRequest, opts ...grpc.CallOption) (*transaction.ComputeHashResponse, error) {
	return &transaction.ComputeHashResponse{
		IsValid: true,
		Hash:    make([]byte, 32),
	}, nil
}

func (m *MockTransactionHashClient) ValidateHash(ctx context.Context, in *transaction.ValidateHashRequest, opts ...grpc.CallOption) (*transaction.ValidateHashResponse, error) {
	return &transaction.ValidateHashResponse{
		IsValid: true,
	}, nil
}

func (m *MockTransactionHashClient) ComputeSignatureHash(ctx context.Context, in *transaction.ComputeSignatureHashRequest, opts ...grpc.CallOption) (*transaction.ComputeSignatureHashResponse, error) {
	return &transaction.ComputeSignatureHashResponse{
		IsValid: true,
		Hash:    make([]byte, 32),
	}, nil
}

func (m *MockTransactionHashClient) ValidateSignatureHash(ctx context.Context, in *transaction.ValidateSignatureHashRequest, opts ...grpc.CallOption) (*transaction.ValidateSignatureHashResponse, error) {
	return &transaction.ValidateSignatureHashResponse{
		IsValid: true,
	}, nil
}

// NewMockTransactionHashClient 创建模拟交易哈希客户端
func NewMockTransactionHashClient() *MockTransactionHashClient {
	return &MockTransactionHashClient{}
}

// MockPoWComputeHandler 模拟PoW计算处理器
type MockPoWComputeHandler struct{}

func (m *MockPoWComputeHandler) MineBlockHeader(ctx context.Context, header *core.BlockHeader) (*core.BlockHeader, error) {
	if header == nil {
		return nil, nil
	}
	minedHeader := *header
	minedHeader.Nonce = []byte{0x01, 0x02, 0x03, 0x04}
	return &minedHeader, nil
}

func (m *MockPoWComputeHandler) VerifyBlockHeader(header *core.BlockHeader) (bool, error) {
	return true, nil
}

func (m *MockPoWComputeHandler) ProduceBlockFromTemplate(ctx context.Context, candidateBlock interface{}) (interface{}, error) {
	return candidateBlock, nil
}

func (m *MockPoWComputeHandler) StartPoWEngine(ctx context.Context, params types.MiningParameters) error {
	return nil
}

func (m *MockPoWComputeHandler) StopPoWEngine(ctx context.Context) error {
	return nil
}

func (m *MockPoWComputeHandler) IsRunning() bool {
	return true
}

// ==================== MockQuorumChecker ====================

// MockQuorumChecker 模拟门槛检查器
// 注意：这个 mock 需要保持足够通用，不依赖具体的 quorum.Result 类型
// 因此我们直接在测试代码中控制其行为
type MockQuorumChecker struct {
	AllowMining     bool
	Reason          string
	SuggestedAction string
	CheckError      error
}

// Check 执行门槛检查（返回兼容 quorum.Checker 接口的结果）
// 注意：实际返回的类型需要与 quorum.Result 兼容
func (m *MockQuorumChecker) Check(ctx context.Context) (interface{}, error) {
	if m.CheckError != nil {
		return nil, m.CheckError
	}
	
	// 返回一个简单的结构体，模拟 quorum.Result
	result := struct {
		AllowMining     bool
		Reason          string
		SuggestedAction string
	}{
		AllowMining:     m.AllowMining,
		Reason:          m.Reason,
		SuggestedAction: m.SuggestedAction,
	}
	
	return &result, nil
}
