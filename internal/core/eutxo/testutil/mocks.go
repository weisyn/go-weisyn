// Package testutil 提供 EUTXO 模块测试的辅助工具
//
// 🧪 **测试辅助工具包**
//
// 本包提供测试所需的 Mock 对象、测试数据和辅助函数，用于简化测试代码编写。
// 遵循 docs/system/standards/principles/testing-standards.md 规范。
package testutil

import (
	"context"
	"crypto/sha256"
	"hash"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
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
func (m *MockLogger) Warnf(format string, args ...interface{}) {}
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
	m.logs = append(m.logs, "DEBUG: "+format)
}

func (m *BehavioralMockLogger) Info(msg string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logs = append(m.logs, "INFO: "+msg)
}

func (m *BehavioralMockLogger) Infof(format string, args ...interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logs = append(m.logs, "INFO: "+format)
}

func (m *BehavioralMockLogger) Warn(msg string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logs = append(m.logs, "WARN: "+msg)
}

func (m *BehavioralMockLogger) Warnf(format string, args ...interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logs = append(m.logs, "WARN: "+format)
}

func (m *BehavioralMockLogger) Error(msg string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logs = append(m.logs, "ERROR: "+msg)
}

func (m *BehavioralMockLogger) Errorf(format string, args ...interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logs = append(m.logs, "ERROR: "+format)
}

func (m *BehavioralMockLogger) Fatal(msg string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logs = append(m.logs, "FATAL: "+msg)
}

func (m *BehavioralMockLogger) Fatalf(format string, args ...interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.logs = append(m.logs, "FATAL: "+format)
}

func (m *BehavioralMockLogger) With(args ...interface{}) log.Logger { return m }
func (m *BehavioralMockLogger) Sync() error                           { return nil }
func (m *BehavioralMockLogger) GetZapLogger() *zap.Logger            { return zap.NewNop() }

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

// MockBadgerStore 模拟 BadgerDB 存储服务
//
// ✅ **设计原则**：内存存储，支持基本操作
// 📋 **使用场景**：单元测试，不需要真实数据库
type MockBadgerStore struct {
	data  map[string][]byte
	mutex sync.RWMutex
}

// NewMockBadgerStore 创建模拟 BadgerDB 存储服务
func NewMockBadgerStore() *MockBadgerStore {
	return &MockBadgerStore{
		data: make(map[string][]byte),
	}
}

// Get 实现 storage.BadgerStore 接口
func (m *MockBadgerStore) Get(ctx context.Context, key []byte) ([]byte, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	value, ok := m.data[string(key)]
	if !ok {
		return nil, nil
	}
	return value, nil
}

// Set 实现 storage.BadgerStore 接口
func (m *MockBadgerStore) Set(ctx context.Context, key, value []byte) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.data[string(key)] = value
	return nil
}

// SetWithTTL 实现 storage.BadgerStore 接口（简化实现，忽略TTL）
func (m *MockBadgerStore) SetWithTTL(ctx context.Context, key, value []byte, ttl time.Duration) error {
	return m.Set(ctx, key, value)
}

// Delete 实现 storage.BadgerStore 接口
func (m *MockBadgerStore) Delete(ctx context.Context, key []byte) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.data, string(key))
	return nil
}

// Exists 实现 storage.BadgerStore 接口
func (m *MockBadgerStore) Exists(ctx context.Context, key []byte) (bool, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	_, ok := m.data[string(key)]
	return ok, nil
}

// GetMany 实现 storage.BadgerStore 接口
func (m *MockBadgerStore) GetMany(ctx context.Context, keys [][]byte) (map[string][]byte, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	result := make(map[string][]byte)
	for _, key := range keys {
		if value, ok := m.data[string(key)]; ok {
			result[string(key)] = value
		}
	}
	return result, nil
}

// SetMany 实现 storage.BadgerStore 接口
func (m *MockBadgerStore) SetMany(ctx context.Context, entries map[string][]byte) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for k, v := range entries {
		m.data[k] = v
	}
	return nil
}

// DeleteMany 实现 storage.BadgerStore 接口
func (m *MockBadgerStore) DeleteMany(ctx context.Context, keys [][]byte) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for _, key := range keys {
		delete(m.data, string(key))
	}
	return nil
}

// PrefixScan 实现 storage.BadgerStore 接口
func (m *MockBadgerStore) PrefixScan(ctx context.Context, prefix []byte) (map[string][]byte, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	result := make(map[string][]byte)
	prefixStr := string(prefix)
	for k, v := range m.data {
		if len(k) >= len(prefixStr) && k[:len(prefixStr)] == prefixStr {
			result[k] = v
		}
	}
	return result, nil
}

// RangeScan 实现 storage.BadgerStore 接口
func (m *MockBadgerStore) RangeScan(ctx context.Context, start, end []byte) (map[string][]byte, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	result := make(map[string][]byte)
	startStr := string(start)
	endStr := string(end)
	for k, v := range m.data {
		if k >= startStr && k < endStr {
			result[k] = v
		}
	}
	return result, nil
}

// RunInTransaction 实现 storage.BadgerStore 接口
func (m *MockBadgerStore) RunInTransaction(ctx context.Context, fn func(txn storage.BadgerTransaction) error) error {
	// 简化实现：创建一个模拟事务并执行函数
	mockTxn := &MockBadgerTransaction{store: m}
	return fn(mockTxn)
}

// MockBadgerTransaction 模拟 BadgerDB 事务
type MockBadgerTransaction struct {
	store *MockBadgerStore
}

// Get 实现 storage.BadgerTransaction 接口
func (m *MockBadgerTransaction) Get(key []byte) ([]byte, error) {
	return m.store.Get(context.Background(), key)
}

// Set 实现 storage.BadgerTransaction 接口
func (m *MockBadgerTransaction) Set(key, value []byte) error {
	return m.store.Set(context.Background(), key, value)
}

// SetWithTTL 实现 storage.BadgerTransaction 接口
func (m *MockBadgerTransaction) SetWithTTL(key, value []byte, ttl time.Duration) error {
	return m.store.SetWithTTL(context.Background(), key, value, ttl)
}

// Delete 实现 storage.BadgerTransaction 接口
func (m *MockBadgerTransaction) Delete(key []byte) error {
	return m.store.Delete(context.Background(), key)
}

// Exists 实现 storage.BadgerTransaction 接口
func (m *MockBadgerTransaction) Exists(key []byte) (bool, error) {
	return m.store.Exists(context.Background(), key)
}

// Merge 实现 storage.BadgerTransaction 接口
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

// Close 实现 storage.BadgerStore 接口
func (m *MockBadgerStore) Close() error {
	return nil
}

// MockHashManager 模拟哈希管理器
//
// ✅ **设计原则**：使用标准库实现，支持基本哈希操作
// 📋 **使用场景**：单元测试，不需要真实哈希服务
type MockHashManager struct{}

// SHA256 实现 crypto.HashManager 接口
func (m *MockHashManager) SHA256(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

// DoubleSHA256 实现 crypto.HashManager 接口
func (m *MockHashManager) DoubleSHA256(data []byte) []byte {
	first := sha256.Sum256(data)
	second := sha256.Sum256(first[:])
	return second[:]
}

// SHA3_256 实现 crypto.HashManager 接口（简化实现，使用SHA256）
func (m *MockHashManager) SHA3_256(data []byte) []byte {
	return m.SHA256(data)
}

// Keccak256 实现 crypto.HashManager 接口（简化实现，使用SHA256）
func (m *MockHashManager) Keccak256(data []byte) []byte {
	return m.SHA256(data)
}

// RIPEMD160 实现 crypto.HashManager 接口（简化实现，使用SHA256的前20字节）
func (m *MockHashManager) RIPEMD160(data []byte) []byte {
	hash := m.SHA256(data)
	if len(hash) >= 20 {
		return hash[:20]
	}
	return hash
}

// NewSHA256Hasher 实现 crypto.HashManager 接口
func (m *MockHashManager) NewSHA256Hasher() hash.Hash {
	return sha256.New()
}

// NewSHA3_256Hasher 实现 crypto.HashManager 接口（简化实现，使用SHA256）
func (m *MockHashManager) NewSHA3_256Hasher() hash.Hash {
	return sha256.New()
}

// NewKeccak256Hasher 实现 crypto.HashManager 接口（简化实现，使用SHA256）
func (m *MockHashManager) NewKeccak256Hasher() hash.Hash {
	return sha256.New()
}

// NewRIPEMD160Hasher 实现 crypto.HashManager 接口（简化实现，使用SHA256的前20字节）
func (m *MockHashManager) NewRIPEMD160Hasher() hash.Hash {
	// 简化实现：返回一个包装的哈希器
	return &mockRIPEMD160Hasher{hasher: sha256.New()}
}

// mockRIPEMD160Hasher 模拟 RIPEMD160 哈希器
type mockRIPEMD160Hasher struct {
	hasher hash.Hash
}

func (m *mockRIPEMD160Hasher) Write(p []byte) (n int, err error) {
	return m.hasher.Write(p)
}

func (m *mockRIPEMD160Hasher) Sum(b []byte) []byte {
	hash := m.hasher.Sum(nil)
	if len(hash) >= 20 {
		return append(b, hash[:20]...)
	}
	return append(b, hash...)
}

func (m *mockRIPEMD160Hasher) Reset() {
	m.hasher.Reset()
}

func (m *mockRIPEMD160Hasher) Size() int {
	return 20
}

func (m *mockRIPEMD160Hasher) BlockSize() int {
	return m.hasher.BlockSize()
}

// MockEventBus 模拟事件总线
//
// ✅ **设计原则**：记录发布的事件，用于验证
// 📋 **使用场景**：需要验证事件发布的测试
type MockEventBus struct {
	events []interface{}
	mutex  sync.RWMutex
}

// NewMockEventBus 创建模拟事件总线
func NewMockEventBus() *MockEventBus {
	return &MockEventBus{
		events: make([]interface{}, 0),
	}
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

// Publish 实现 event.EventBus 接口
func (m *MockEventBus) Publish(eventType event.EventType, args ...interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.events = append(m.events, args...)
}

// PublishEvent 实现 event.EventBus 接口
func (m *MockEventBus) PublishEvent(evt event.Event) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.events = append(m.events, evt)
}

// Unsubscribe 实现 event.EventBus 接口
func (m *MockEventBus) Unsubscribe(eventType event.EventType, handler interface{}) error {
	return nil
}

// WaitAsync 实现 event.EventBus 接口
func (m *MockEventBus) WaitAsync() {}

// HasCallback 实现 event.EventBus 接口
func (m *MockEventBus) HasCallback(eventType event.EventType) bool {
	return false
}

// GetEventHistory 实现 event.EventBus 接口
func (m *MockEventBus) GetEventHistory(eventType event.EventType) []interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return append([]interface{}{}, m.events...)
}

// PublishWESEvent 实现 event.EventBus 接口
func (m *MockEventBus) PublishWESEvent(evt *types.WESEvent) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.events = append(m.events, evt)
	return nil
}

// SubscribeWithFilter 实现 event.EventBus 接口
func (m *MockEventBus) SubscribeWithFilter(eventType event.EventType, filter event.EventFilter, handler event.EventHandler) (types.SubscriptionID, error) {
	return types.SubscriptionID("mock-subscription"), nil
}

// SubscribeWESEvents 实现 event.EventBus 接口
func (m *MockEventBus) SubscribeWESEvents(protocols []event.ProtocolType, handler event.WESEventHandler) (types.SubscriptionID, error) {
	return types.SubscriptionID("mock-subscription"), nil
}

// UnsubscribeByID 实现 event.EventBus 接口
func (m *MockEventBus) UnsubscribeByID(id types.SubscriptionID) error {
	return nil
}

// EnableEventHistory 实现 event.EventBus 接口
func (m *MockEventBus) EnableEventHistory(eventType event.EventType, maxSize int) error {
	return nil
}

// DisableEventHistory 实现 event.EventBus 接口
func (m *MockEventBus) DisableEventHistory(eventType event.EventType) error {
	return nil
}

// GetActiveSubscriptions 实现 event.EventBus 接口
func (m *MockEventBus) GetActiveSubscriptions() ([]*types.SubscriptionInfo, error) {
	return nil, nil
}

// UpdateConfig 实现 event.EventBus 接口
func (m *MockEventBus) UpdateConfig(config *types.EventBusConfig) error {
	return nil
}

// GetConfig 实现 event.EventBus 接口
func (m *MockEventBus) GetConfig() (*types.EventBusConfig, error) {
	return nil, nil
}

// RegisterEventInterceptor 实现 event.EventBus 接口
func (m *MockEventBus) RegisterEventInterceptor(interceptor event.EventInterceptor) error {
	return nil
}

// UnregisterEventInterceptor 实现 event.EventBus 接口
func (m *MockEventBus) UnregisterEventInterceptor(interceptorID string) error {
	return nil
}

// GetEvents 获取所有发布的事件
func (m *MockEventBus) GetEvents() []interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return append([]interface{}{}, m.events...)
}

// ClearEvents 清空事件记录
func (m *MockEventBus) ClearEvents() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.events = m.events[:0]
}


