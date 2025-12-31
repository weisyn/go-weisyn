// Package txpool 压力测试
//
// P2-10: 交易池压力测试
//
// 🎯 **测试目标**：
// - 高并发场景测试（多个goroutine同时提交交易）
// - 性能基准测试（使用testing.B）
// - 内存压力测试（大量交易）
// - 性能指标收集
package txpool

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/weisyn/v1/internal/config/txpool"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	mempoolIfaces "github.com/weisyn/v1/pkg/interfaces/mempool"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

// mockHashService 模拟哈希服务
type mockHashService struct{}

func (m *mockHashService) ComputeHash(ctx context.Context, in *transaction.ComputeHashRequest, opts ...grpc.CallOption) (*transaction.ComputeHashResponse, error) {
	if in == nil || in.Transaction == nil {
		return nil, fmt.Errorf("无效的请求")
	}
	// 简单实现：使用交易序列化后的哈希
	data, _ := proto.Marshal(in.Transaction)
	hash := make([]byte, 32)
	copy(hash, data[:min(32, len(data))])
	// 填充不足32字节的部分
	for i := len(data); i < 32; i++ {
		hash[i] = byte(i % 256)
	}
	return &transaction.ComputeHashResponse{
		Hash:    hash,
		IsValid: true,
	}, nil
}

func (m *mockHashService) ValidateHash(ctx context.Context, in *transaction.ValidateHashRequest, opts ...grpc.CallOption) (*transaction.ValidateHashResponse, error) {
	if in == nil || in.Transaction == nil {
		return nil, fmt.Errorf("无效的请求")
	}
	// 简单实现：计算哈希并比较
	data, _ := proto.Marshal(in.Transaction)
	hash := make([]byte, 32)
	copy(hash, data[:min(32, len(data))])
	for i := len(data); i < 32; i++ {
		hash[i] = byte(i % 256)
	}
	isValid := len(in.ExpectedHash) == 32 && string(hash) == string(in.ExpectedHash)
	return &transaction.ValidateHashResponse{
		IsValid: isValid,
	}, nil
}

func (m *mockHashService) ComputeSignatureHash(ctx context.Context, in *transaction.ComputeSignatureHashRequest, opts ...grpc.CallOption) (*transaction.ComputeSignatureHashResponse, error) {
	if in == nil || in.Transaction == nil {
		return nil, fmt.Errorf("无效的请求")
	}
	// 简单实现：使用交易序列化后的哈希（不考虑 SIGHASH 类型）
	data, _ := proto.Marshal(in.Transaction)
	hash := make([]byte, 32)
	copy(hash, data[:min(32, len(data))])
	// 填充不足32字节的部分
	for i := len(data); i < 32; i++ {
		hash[i] = byte(i % 256)
	}
	return &transaction.ComputeSignatureHashResponse{
		Hash:    hash,
		IsValid: true,
	}, nil
}

func (m *mockHashService) ValidateSignatureHash(ctx context.Context, in *transaction.ValidateSignatureHashRequest, opts ...grpc.CallOption) (*transaction.ValidateSignatureHashResponse, error) {
	if in == nil || in.Transaction == nil {
		return nil, fmt.Errorf("无效的请求")
	}
	// 简单实现：计算签名哈希并比较
	data, _ := proto.Marshal(in.Transaction)
	hash := make([]byte, 32)
	copy(hash, data[:min(32, len(data))])
	for i := len(data); i < 32; i++ {
		hash[i] = byte(i % 256)
	}
	isValid := len(in.ExpectedHash) == 32 && string(hash) == string(in.ExpectedHash)
	return &transaction.ValidateSignatureHashResponse{
		IsValid: isValid,
	}, nil
}

// mockMemoryStore 模拟内存存储
type mockMemoryStore struct {
	data map[string][]byte
	mu   sync.RWMutex
}

func newMockMemoryStore() storage.MemoryStore {
	return &mockMemoryStore{
		data: make(map[string][]byte),
	}
}

func (m *mockMemoryStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.data[key]
	return val, ok, nil
}

func (m *mockMemoryStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return nil
}

func (m *mockMemoryStore) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

func (m *mockMemoryStore) Exists(ctx context.Context, key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.data[key]
	return ok, nil
}

func (m *mockMemoryStore) GetMany(ctx context.Context, keys []string) (map[string][]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string][]byte)
	for _, key := range keys {
		if val, ok := m.data[key]; ok {
			result[key] = val
		}
	}
	return result, nil
}

func (m *mockMemoryStore) SetMany(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range items {
		m.data[k] = v
	}
	return nil
}

func (m *mockMemoryStore) DeleteMany(ctx context.Context, keys []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, key := range keys {
		delete(m.data, key)
	}
	return nil
}

func (m *mockMemoryStore) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string][]byte)
	return nil
}

func (m *mockMemoryStore) DeleteByPattern(ctx context.Context, pattern string) (int64, error) {
	// 简化实现：不支持模式匹配
	return 0, nil
}

func (m *mockMemoryStore) GetKeys(ctx context.Context, pattern string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	keys := make([]string, 0, len(m.data))
	for k := range m.data {
		keys = append(keys, k)
	}
	return keys, nil
}

func (m *mockMemoryStore) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	return 0, nil
}

func (m *mockMemoryStore) UpdateTTL(ctx context.Context, key string, ttl time.Duration) error {
	return nil
}

func (m *mockMemoryStore) Count(ctx context.Context) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return int64(len(m.data)), nil
}

// createTestTransaction 创建测试交易
func createTestTransaction(txID int) *transaction.Transaction {
	txIDBytes := make([]byte, 32)
	txIDBytes[0] = byte(txID)
	txIDBytes[1] = byte(txID >> 8)
	txIDBytes[2] = byte(txID >> 16)
	txIDBytes[3] = byte(txID >> 24)

	return &transaction.Transaction{
		Version: 1,
		Inputs: []*transaction.TxInput{
			{
				PreviousOutput: &transaction.OutPoint{
					TxId:        txIDBytes,
					OutputIndex: 0,
				},
				IsReferenceOnly: false,
				Sequence:        0xFFFFFFFF,
			},
		},
		Outputs: []*transaction.TxOutput{
			{
				Owner: []byte(fmt.Sprintf("recipient_%d", txID)),
				LockingConditions: []*transaction.LockingCondition{
					{
						Condition: &transaction.LockingCondition_SingleKeyLock{
							SingleKeyLock: &transaction.SingleKeyLock{
								KeyRequirement: &transaction.SingleKeyLock_RequiredAddressHash{
									RequiredAddressHash: []byte(fmt.Sprintf("addr_hash_%d", txID)),
								},
								RequiredAlgorithm: transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
								SighashType:       transaction.SignatureHashType_SIGHASH_ALL,
							},
						},
					},
				},
				OutputContent: &transaction.TxOutput_Asset{
					Asset: &transaction.AssetOutput{
						AssetContent: &transaction.AssetOutput_NativeCoin{
							NativeCoin: &transaction.NativeCoinAsset{
								Amount: "100000000000", // 1000 WES
							},
						},
					},
				},
			},
		},
		Nonce:             uint64(txID),
		CreationTimestamp: uint64(time.Now().Unix()),
		ChainId:           []byte("weisyn-testnet"),
		FeeMechanism: &transaction.Transaction_MinimumFee{
			MinimumFee: &transaction.MinimumFee{
				MinimumAmount: "5000000000", // 50 WES
			},
		},
	}
}

// createTxPool 创建测试交易池
func createTxPool(t testing.TB) mempoolIfaces.TxPool {
	config := &txpool.TxPoolOptions{
		MaxSize:        10000,
		MemoryLimit:    100 * 1024 * 1024, // 100MB
		Lifetime:       time.Hour,
		MaxTxSize:      1024 * 1024, // 1MB
		MetricsEnabled: true,
		MetricsInterval: time.Minute,
	}

	memory := newMockMemoryStore()
	hashService := &mockHashService{}

	pool, err := NewTxPoolWithCache(config, nil, nil, memory, hashService, nil)
	if err != nil {
		t.Fatalf("创建交易池失败: %v", err)
	}

	return pool
}

// TestTxPool_ConcurrentSubmit 测试高并发提交交易
func TestTxPool_ConcurrentSubmit(t *testing.T) {
	pool := createTxPool(t)
	defer func() {
		if closer, ok := pool.(interface{ Close() }); ok {
			closer.Close()
		}
	}()

	const (
		numGoroutines = 100
		txsPerGoroutine = 100
		totalTxs = numGoroutines * txsPerGoroutine
	)

	var (
		successCount int64
		failCount    int64
		wg           sync.WaitGroup
	)

	start := time.Now()

	// 并发提交交易
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < txsPerGoroutine; j++ {
				txID := goroutineID*txsPerGoroutine + j
				tx := createTestTransaction(txID)
				
				_, err := pool.SubmitTx(tx)
				if err != nil {
					atomic.AddInt64(&failCount, 1)
				} else {
					atomic.AddInt64(&successCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	t.Logf("并发提交测试完成:")
	t.Logf("  - 总交易数: %d", totalTxs)
	t.Logf("  - 成功数: %d", successCount)
	t.Logf("  - 失败数: %d", failCount)
	t.Logf("  - 耗时: %v", duration)
	t.Logf("  - TPS: %.2f", float64(totalTxs)/duration.Seconds())

	// 验证至少有一些交易成功
	if successCount == 0 {
		t.Error("没有交易成功提交")
	}
}

// BenchmarkTxPool_SubmitTx 基准测试：单交易提交
func BenchmarkTxPool_SubmitTx(b *testing.B) {
	pool := createTxPool(b)
	defer func() {
		if closer, ok := pool.(interface{ Close() }); ok {
			closer.Close()
		}
	}()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		tx := createTestTransaction(i)
		_, _ = pool.SubmitTx(tx)
	}
}

// BenchmarkTxPool_GetPendingTxs 基准测试：获取待处理交易
func BenchmarkTxPool_GetPendingTxs(b *testing.B) {
	pool := createTxPool(b)
	defer func() {
		if closer, ok := pool.(interface{ Close() }); ok {
			closer.Close()
		}
	}()

	// 预先提交一些交易
	const preloadCount = 1000
	for i := 0; i < preloadCount; i++ {
		tx := createTestTransaction(i)
		_, _ = pool.SubmitTx(tx)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = pool.GetAllPendingTransactions()
	}
}

// BenchmarkTxPool_ConcurrentSubmit 基准测试：并发提交
func BenchmarkTxPool_ConcurrentSubmit(b *testing.B) {
	pool := createTxPool(b)
	defer func() {
		if closer, ok := pool.(interface{ Close() }); ok {
			closer.Close()
		}
	}()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		txID := 0
		for pb.Next() {
			txID++
			tx := createTestTransaction(txID)
			_, _ = pool.SubmitTx(tx)
		}
	})
}

// TestTxPool_MemoryPressure 内存压力测试
func TestTxPool_MemoryPressure(t *testing.T) {
	pool := createTxPool(t)
	defer func() {
		if closer, ok := pool.(interface{ Close() }); ok {
			closer.Close()
		}
	}()

	const numTxs = 5000

	start := time.Now()

	// 提交大量交易
	for i := 0; i < numTxs; i++ {
		tx := createTestTransaction(i)
		_, err := pool.SubmitTx(tx)
		if err != nil && err.Error() != "交易池已满" {
			// 除了池满错误，其他错误都记录
			t.Logf("提交交易 %d 失败: %v", i, err)
		}
	}

	duration := time.Since(start)

	// 获取待处理交易数量
	pendingTxs, _ := pool.GetAllPendingTransactions()

	t.Logf("内存压力测试完成:")
	t.Logf("  - 尝试提交交易数: %d", numTxs)
	t.Logf("  - 实际待处理交易数: %d", len(pendingTxs))
	t.Logf("  - 耗时: %v", duration)
	t.Logf("  - TPS: %.2f", float64(numTxs)/duration.Seconds())

	// 验证交易池仍然可用
	if len(pendingTxs) == 0 {
		t.Error("交易池中没有待处理交易")
	}
}

// TestTxPool_ConcurrentGetPending 并发获取待处理交易测试
func TestTxPool_ConcurrentGetPending(t *testing.T) {
	pool := createTxPool(t)
	defer func() {
		if closer, ok := pool.(interface{ Close() }); ok {
			closer.Close()
		}
	}()

	// 预先提交一些交易
	const preloadCount = 1000
	for i := 0; i < preloadCount; i++ {
		tx := createTestTransaction(i)
		_, _ = pool.SubmitTx(tx)
	}

	const numGoroutines = 50

	var wg sync.WaitGroup
	start := time.Now()

	// 并发获取待处理交易
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_, _ = pool.GetAllPendingTransactions()
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	t.Logf("并发获取待处理交易测试完成:")
	t.Logf("  - Goroutine数: %d", numGoroutines)
	t.Logf("  - 每个Goroutine请求数: 100")
	t.Logf("  - 总请求数: %d", numGoroutines*100)
	t.Logf("  - 耗时: %v", duration)
	t.Logf("  - QPS: %.2f", float64(numGoroutines*100)/duration.Seconds())
}

// TestTxPool_StressMix 混合压力测试：提交、获取、确认混合操作
func TestTxPool_StressMix(t *testing.T) {
	pool := createTxPool(t)
	defer func() {
		if closer, ok := pool.(interface{ Close() }); ok {
			closer.Close()
		}
	}()

	const (
		numGoroutines = 50
		opsPerGoroutine = 100
	)

	var (
		submitCount   int64
		getCount      int64
		confirmCount  int64
		wg            sync.WaitGroup
	)

	start := time.Now()

	// 启动多个goroutine执行混合操作
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				opType := j % 3
				switch opType {
				case 0: // 提交交易
					txID := goroutineID*opsPerGoroutine + j
					tx := createTestTransaction(txID)
					if _, err := pool.SubmitTx(tx); err == nil {
						atomic.AddInt64(&submitCount, 1)
					}
				case 1: // 获取待处理交易
					_, _ = pool.GetAllPendingTransactions()
					atomic.AddInt64(&getCount, 1)
				case 2: // 模拟确认交易（获取后确认）
					txs, _ := pool.GetAllPendingTransactions()
					if len(txs) > 0 {
						// 获取交易哈希（简化实现）
						txHash := make([]byte, 32)
						rand.Read(txHash)
						pool.ConfirmTransactions([][]byte{txHash}, uint64(j))
						atomic.AddInt64(&confirmCount, 1)
					}
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	t.Logf("混合压力测试完成:")
	t.Logf("  - 提交操作数: %d", submitCount)
	t.Logf("  - 获取操作数: %d", getCount)
	t.Logf("  - 确认操作数: %d", confirmCount)
	t.Logf("  - 耗时: %v", duration)
	t.Logf("  - OPS: %.2f", float64(submitCount+getCount+confirmCount)/duration.Seconds())
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

