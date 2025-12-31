// Package testutil 提供 Mempool 模块测试的辅助工具
//
// 🧪 **测试辅助工具包**
//
// 本包提供测试所需的 Mock 对象、测试数据和辅助函数，用于简化测试代码编写。
// 遵循 docs/system/standards/principles/testing-standards.md 规范。
package testutil

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	complianceIfaces "github.com/weisyn/v1/pkg/interfaces/compliance"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
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

// MockEventBus 统一的事件总线Mock实现
//
// ✅ **设计原则**：最小实现，不实际发布事件
// 📋 **使用场景**：80%的测试用例，不需要验证事件发布
type MockEventBus struct {
	mu     sync.RWMutex
	events []event.Event
}

// NewMockEventBus 创建新的Mock事件总线
func NewMockEventBus() *MockEventBus {
	return &MockEventBus{
		events: make([]event.Event, 0),
	}
}

// Subscribe 订阅事件
func (m *MockEventBus) Subscribe(eventType event.EventType, handler interface{}) error {
	return nil
}

// SubscribeAsync 异步订阅事件
func (m *MockEventBus) SubscribeAsync(eventType event.EventType, handler interface{}, transactional bool) error {
	return nil
}

// SubscribeOnce 一次性订阅事件
func (m *MockEventBus) SubscribeOnce(eventType event.EventType, handler interface{}) error {
	return nil
}

// SubscribeOnceAsync 异步一次性订阅事件
func (m *MockEventBus) SubscribeOnceAsync(eventType event.EventType, handler interface{}, transactional bool) error {
	return nil
}

// Publish 发布事件
func (m *MockEventBus) Publish(eventType event.EventType, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Mock实现：不实际发布事件
}

// PublishEvent 发布Event接口类型事件
func (m *MockEventBus) PublishEvent(evt event.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, evt)
}

// Unsubscribe 取消订阅
func (m *MockEventBus) Unsubscribe(eventType event.EventType, handler interface{}) error {
	return nil
}

// WaitAsync 等待所有异步处理完成
func (m *MockEventBus) WaitAsync() {}

// HasCallback 检查是否有回调函数
func (m *MockEventBus) HasCallback(eventType event.EventType) bool {
	return false
}

// GetEventHistory 获取指定事件类型的历史记录
func (m *MockEventBus) GetEventHistory(eventType event.EventType) []interface{} {
	return nil
}

// PublishWESEvent 发布WES事件
func (m *MockEventBus) PublishWESEvent(event *types.WESEvent) error {
	return nil
}

// SubscribeWithFilter 带过滤器的订阅
func (m *MockEventBus) SubscribeWithFilter(eventType event.EventType, filter event.EventFilter, handler event.EventHandler) (types.SubscriptionID, error) {
	return "", nil
}

// SubscribeWESEvents 订阅WES消息事件
func (m *MockEventBus) SubscribeWESEvents(protocols []event.ProtocolType, handler event.WESEventHandler) (types.SubscriptionID, error) {
	return "", nil
}

// UnsubscribeByID 通过订阅ID取消订阅
func (m *MockEventBus) UnsubscribeByID(id types.SubscriptionID) error {
	return nil
}

// EnableEventHistory 启用事件历史记录
func (m *MockEventBus) EnableEventHistory(eventType event.EventType, maxSize int) error {
	return nil
}

// DisableEventHistory 禁用事件历史记录
func (m *MockEventBus) DisableEventHistory(eventType event.EventType) error {
	return nil
}

// GetActiveSubscriptions 获取活跃订阅列表
func (m *MockEventBus) GetActiveSubscriptions() ([]*types.SubscriptionInfo, error) {
	return nil, nil
}

// UpdateConfig 更新事件总线配置
func (m *MockEventBus) UpdateConfig(config *types.EventBusConfig) error {
	return nil
}

// GetConfig 获取当前配置
func (m *MockEventBus) GetConfig() (*types.EventBusConfig, error) {
	return nil, nil
}

// RegisterEventInterceptor 注册事件拦截器
func (m *MockEventBus) RegisterEventInterceptor(interceptor event.EventInterceptor) error {
	return nil
}

// UnregisterEventInterceptor 注销事件拦截器
func (m *MockEventBus) UnregisterEventInterceptor(interceptorID string) error {
	return nil
}

// GetEvents 获取所有发布的事件（用于测试）
func (m *MockEventBus) GetEvents() []event.Event {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]event.Event{}, m.events...)
}

// ClearEvents 清空事件（用于测试）
func (m *MockEventBus) ClearEvents() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = m.events[:0]
}

// MockMemoryStore 统一的内存存储Mock实现
//
// ✅ **设计原则**：内存存储，支持基本的键值操作
// 📋 **使用场景**：Mempool测试，需要模拟内存存储
type MockMemoryStore struct {
	mu    sync.RWMutex
	store map[string][]byte
	ttl   map[string]time.Time
}

// NewMockMemoryStore 创建新的Mock内存存储
func NewMockMemoryStore() *MockMemoryStore {
	return &MockMemoryStore{
		store: make(map[string][]byte),
		ttl:   make(map[string]time.Time),
	}
}

// Get 获取值
func (m *MockMemoryStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, exists := m.store[key]
	if !exists {
		return nil, false, nil
	}
	// 检查TTL
	if expireTime, hasTTL := m.ttl[key]; hasTTL {
		if time.Now().After(expireTime) {
			delete(m.store, key)
			delete(m.ttl, key)
			return nil, false, nil
		}
	}
	return value, true, nil
}

// Set 设置键值
func (m *MockMemoryStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[key] = value
	if ttl > 0 {
		m.ttl[key] = time.Now().Add(ttl)
	} else {
		delete(m.ttl, key)
	}
	return nil
}

// Delete 删除键
func (m *MockMemoryStore) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.store, key)
	delete(m.ttl, key)
	return nil
}

// Exists 检查键是否存在
func (m *MockMemoryStore) Exists(ctx context.Context, key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.store[key]
	if exists {
		// 检查TTL
		if expireTime, hasTTL := m.ttl[key]; hasTTL {
			if time.Now().After(expireTime) {
				return false, nil
			}
		}
	}
	return exists, nil
}

// GetMany 批量获取
func (m *MockMemoryStore) GetMany(ctx context.Context, keys []string) (map[string][]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string][]byte)
	for _, key := range keys {
		if value, exists := m.store[key]; exists {
			// 检查TTL
			if expireTime, hasTTL := m.ttl[key]; hasTTL {
				if time.Now().After(expireTime) {
					continue
				}
			}
			result[key] = value
		}
	}
	return result, nil
}

// SetMany 批量设置
func (m *MockMemoryStore) SetMany(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, value := range items {
		m.store[key] = value
		if ttl > 0 {
			m.ttl[key] = time.Now().Add(ttl)
		} else {
			delete(m.ttl, key)
		}
	}
	return nil
}

// DeleteMany 批量删除
func (m *MockMemoryStore) DeleteMany(ctx context.Context, keys []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, key := range keys {
		delete(m.store, key)
		delete(m.ttl, key)
	}
	return nil
}

// Clear 清空所有数据
func (m *MockMemoryStore) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store = make(map[string][]byte)
	m.ttl = make(map[string]time.Time)
	return nil
}

// DeleteByPattern 根据模式删除
func (m *MockMemoryStore) DeleteByPattern(ctx context.Context, pattern string) (int64, error) {
	// Mock实现：简单实现，不支持通配符
	return 0, nil
}

// GetKeys 获取匹配模式的所有键
func (m *MockMemoryStore) GetKeys(ctx context.Context, pattern string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	keys := make([]string, 0, len(m.store))
	for key := range m.store {
		keys = append(keys, key)
	}
	return keys, nil
}

// GetTTL 获取键的剩余生存时间
func (m *MockMemoryStore) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	expireTime, exists := m.ttl[key]
	if !exists {
		return 0, errors.New("键不存在或没有TTL")
	}
	remaining := time.Until(expireTime)
	if remaining < 0 {
		return 0, errors.New("键已过期")
	}
	return remaining, nil
}

// UpdateTTL 更新键的过期时间
func (m *MockMemoryStore) UpdateTTL(ctx context.Context, key string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.store[key]; !exists {
		return errors.New("键不存在")
	}
	if ttl > 0 {
		m.ttl[key] = time.Now().Add(ttl)
	} else {
		delete(m.ttl, key)
	}
	return nil
}

// Count 获取当前缓存中的键数量
func (m *MockMemoryStore) Count(ctx context.Context) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return int64(len(m.store)), nil
}

// MockTransactionHashService 统一的交易哈希服务Mock实现
//
// ✅ **设计原则**：返回固定哈希值
// 📋 **使用场景**：Mempool测试，需要模拟交易哈希计算
type MockTransactionHashService struct{}

// ComputeHash 计算交易哈希
func (m *MockTransactionHashService) ComputeHash(ctx context.Context, in *transaction.ComputeHashRequest, opts ...grpc.CallOption) (*transaction.ComputeHashResponse, error) {
	// Mock实现：基于交易内容生成32字节哈希
	hash := make([]byte, 32)
	if in != nil && in.Transaction != nil {
		// 使用交易的Nonce和CreationTimestamp生成哈希
		nonce := in.Transaction.Nonce
		timestamp := in.Transaction.CreationTimestamp
		for i := 0; i < 32; i++ {
			if i < 8 {
				hash[i] = byte(nonce >> (i * 8))
			} else if i < 16 {
				hash[i] = byte(timestamp >> ((i - 8) * 8))
			} else {
				hash[i] = byte(i)
			}
		}
	} else {
		// 默认哈希值
		copy(hash, []byte("mock_tx_hash_32_bytes_12345678"))
	}
	return &transaction.ComputeHashResponse{
		Hash:    hash,
		IsValid: true,
	}, nil
}

// ValidateHash 验证交易哈希
func (m *MockTransactionHashService) ValidateHash(ctx context.Context, in *transaction.ValidateHashRequest, opts ...grpc.CallOption) (*transaction.ValidateHashResponse, error) {
	// Mock实现：总是返回true
	return &transaction.ValidateHashResponse{
		IsValid: true,
	}, nil
}

// ComputeSignatureHash 计算签名哈希
func (m *MockTransactionHashService) ComputeSignatureHash(ctx context.Context, in *transaction.ComputeSignatureHashRequest, opts ...grpc.CallOption) (*transaction.ComputeSignatureHashResponse, error) {
	// Mock实现：返回固定哈希值
	return &transaction.ComputeSignatureHashResponse{
		Hash:    []byte("mock_sig_hash_32_bytes_12345678"),
		IsValid: true,
	}, nil
}

// ValidateSignatureHash 验证签名哈希
func (m *MockTransactionHashService) ValidateSignatureHash(ctx context.Context, in *transaction.ValidateSignatureHashRequest, opts ...grpc.CallOption) (*transaction.ValidateSignatureHashResponse, error) {
	// Mock实现：总是返回true
	return &transaction.ValidateSignatureHashResponse{
		IsValid: true,
	}, nil
}

// MockBlockHashService 统一的区块哈希服务Mock实现
//
// ✅ **设计原则**：可重复、低成本、且避免测试内碰撞
// 📋 **使用场景**：Mempool测试，需要模拟区块哈希计算
type MockBlockHashService struct {
}

// ComputeBlockHash 计算区块哈希
func (m *MockBlockHashService) ComputeBlockHash(ctx context.Context, in *core.ComputeBlockHashRequest, opts ...grpc.CallOption) (*core.ComputeBlockHashResponse, error) {
	// Mock实现：根据区块高度生成不同的32字节哈希值
	hash := make([]byte, 32)
	if in.Block != nil && in.Block.Header != nil {
		// 使用区块高度和时间戳生成确定性哈希（相同区块重复计算应得到相同结果）
		height := in.Block.Header.Height
		timestamp := in.Block.Header.Timestamp
		// 填充哈希：前8字节为高度，接下来8字节为时间戳，剩余16字节为固定值
		for i := 0; i < 8; i++ {
			hash[i] = byte(height >> (i * 8))
		}
		for i := 0; i < 8; i++ {
			hash[8+i] = byte(timestamp >> (i * 8))
		}
		copy(hash[16:], []byte("mock_hash_16bytes"))
	} else {
		// 如果区块为nil，返回固定哈希
		copy(hash, []byte("mock_block_hash_32_bytes_12345678"))
	}
	return &core.ComputeBlockHashResponse{
		Hash:    hash,
		IsValid: true, // 必须设置为true，否则会返回"区块结构无效"错误
	}, nil
}

// ValidateBlockHash 验证区块哈希
func (m *MockBlockHashService) ValidateBlockHash(ctx context.Context, in *core.ValidateBlockHashRequest, opts ...grpc.CallOption) (*core.ValidateBlockHashResponse, error) {
	// Mock实现：总是返回true
	return &core.ValidateBlockHashResponse{
		IsValid: true,
	}, nil
}

// MockCompliancePolicy 统一的合规策略Mock实现
//
// ✅ **设计原则**：可配置的Mock，支持允许/拒绝决策
// 📋 **使用场景**：Mempool测试，需要模拟合规检查
type MockCompliancePolicy struct {
	shouldAllow bool
	decision    *complianceIfaces.Decision
	err         error
}

// NewMockCompliancePolicy 创建新的Mock合规策略
func NewMockCompliancePolicy(shouldAllow bool) *MockCompliancePolicy {
	return &MockCompliancePolicy{
		shouldAllow: shouldAllow,
		decision: &complianceIfaces.Decision{
			Allowed:   shouldAllow,
			Reason:    "",
			Source:    complianceIfaces.DecisionSourceConfig, // 使用Config作为Mock源
			Timestamp: time.Now(),
		},
	}
}

// NewMockCompliancePolicyWithDecision 创建带自定义决策的Mock合规策略
func NewMockCompliancePolicyWithDecision(decision *complianceIfaces.Decision) *MockCompliancePolicy {
	return &MockCompliancePolicy{
		shouldAllow: decision.Allowed,
		decision:    decision,
	}
}

// NewMockCompliancePolicyWithError 创建返回错误的Mock合规策略
func NewMockCompliancePolicyWithError(err error) *MockCompliancePolicy {
	return &MockCompliancePolicy{
		err: err,
	}
}

// CheckTransaction 检查交易的合规性
func (m *MockCompliancePolicy) CheckTransaction(ctx context.Context, tx *transaction.Transaction, source *complianceIfaces.TransactionSource) (*complianceIfaces.Decision, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.decision, nil
}

// CheckOperation 检查特定操作的合规性
func (m *MockCompliancePolicy) CheckOperation(ctx context.Context, operation string, address string, source *complianceIfaces.TransactionSource) (*complianceIfaces.Decision, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.decision, nil
}
