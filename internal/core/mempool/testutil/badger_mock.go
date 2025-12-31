// Package testutil BadgerStore Mock实现
package testutil

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// MockBadgerStore Mock BadgerStore实现
//
// ✅ **设计原则**：内存存储，支持基本的键值操作
// 📋 **使用场景**：Mempool测试，需要模拟BadgerDB存储
type MockBadgerStore struct {
	mu    sync.RWMutex
	store map[string][]byte
	ttl   map[string]time.Time
	err   error // 可配置的错误，用于测试错误路径
}

// NewMockBadgerStore 创建新的Mock BadgerStore
func NewMockBadgerStore() *MockBadgerStore {
	return &MockBadgerStore{
		store: make(map[string][]byte),
		ttl:   make(map[string]time.Time),
	}
}

// NewMockBadgerStoreWithError 创建返回错误的Mock BadgerStore
func NewMockBadgerStoreWithError(err error) *MockBadgerStore {
	return &MockBadgerStore{
		store: make(map[string][]byte),
		ttl:   make(map[string]time.Time),
		err:   err,
	}
}

// SetError 设置错误（用于测试错误路径）
func (m *MockBadgerStore) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

// Get 获取指定键的值
func (m *MockBadgerStore) Get(ctx context.Context, key []byte) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	keyStr := string(key)
	value, exists := m.store[keyStr]
	if !exists {
		return nil, nil
	}
	// 检查TTL
	if expireTime, hasTTL := m.ttl[keyStr]; hasTTL {
		if time.Now().After(expireTime) {
			delete(m.store, keyStr)
			delete(m.ttl, keyStr)
			return nil, nil
		}
	}
	return value, nil
}

// Set 设置键值对
func (m *MockBadgerStore) Set(ctx context.Context, key, value []byte) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	keyStr := string(key)
	m.store[keyStr] = value
	return nil
}

// SetWithTTL 设置键值对并指定过期时间
func (m *MockBadgerStore) SetWithTTL(ctx context.Context, key, value []byte, ttl time.Duration) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	keyStr := string(key)
	m.store[keyStr] = value
	if ttl > 0 {
		m.ttl[keyStr] = time.Now().Add(ttl)
	} else {
		delete(m.ttl, keyStr)
	}
	return nil
}

// Delete 删除指定键的值
func (m *MockBadgerStore) Delete(ctx context.Context, key []byte) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	keyStr := string(key)
	delete(m.store, keyStr)
	delete(m.ttl, keyStr)
	return nil
}

// Exists 检查键是否存在
func (m *MockBadgerStore) Exists(ctx context.Context, key []byte) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	keyStr := string(key)
	_, exists := m.store[keyStr]
	if exists {
		// 检查TTL
		if expireTime, hasTTL := m.ttl[keyStr]; hasTTL {
			if time.Now().After(expireTime) {
				return false, nil
			}
		}
	}
	return exists, nil
}

// GetMany 批量获取多个键的值
func (m *MockBadgerStore) GetMany(ctx context.Context, keys [][]byte) (map[string][]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string][]byte)
	for _, key := range keys {
		keyStr := string(key)
		if value, exists := m.store[keyStr]; exists {
			// 检查TTL
			if expireTime, hasTTL := m.ttl[keyStr]; hasTTL {
				if time.Now().After(expireTime) {
					continue
				}
			}
			result[keyStr] = value
		}
	}
	return result, nil
}

// SetMany 批量设置多个键值对
func (m *MockBadgerStore) SetMany(ctx context.Context, items map[string][]byte) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, value := range items {
		m.store[key] = value
	}
	return nil
}

// DeleteMany 批量删除多个键
func (m *MockBadgerStore) DeleteMany(ctx context.Context, keys [][]byte) error {
	if m.err != nil {
		return m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, key := range keys {
		keyStr := string(key)
		delete(m.store, keyStr)
		delete(m.ttl, keyStr)
	}
	return nil
}

// Close 关闭BadgerDB数据库连接
func (m *MockBadgerStore) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store = make(map[string][]byte)
	m.ttl = make(map[string]time.Time)
	return nil
}

// NewTransaction 创建新事务
func (m *MockBadgerStore) NewTransaction(update bool) (storage.BadgerTransaction, error) {
	// Mock实现：返回nil，表示不支持事务
	return nil, errors.New("Mock BadgerStore不支持事务")
}

// View 执行只读事务
func (m *MockBadgerStore) View(fn func(txn storage.BadgerTransaction) error) error {
	// Mock实现：直接执行函数
	return fn(nil)
}

// Update 执行更新事务
func (m *MockBadgerStore) Update(fn func(txn storage.BadgerTransaction) error) error {
	// Mock实现：直接执行函数
	return fn(nil)
}

// PrefixScan 按前缀扫描键值对
func (m *MockBadgerStore) PrefixScan(ctx context.Context, prefix []byte) (map[string][]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string][]byte)
	prefixStr := string(prefix)
	for key, value := range m.store {
		if len(key) >= len(prefixStr) && key[:len(prefixStr)] == prefixStr {
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

// RangeScan 范围扫描键值对
func (m *MockBadgerStore) RangeScan(ctx context.Context, startKey, endKey []byte) (map[string][]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string][]byte)
	startStr := string(startKey)
	endStr := string(endKey)
	for key, value := range m.store {
		if key >= startStr && key < endStr {
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

// RunInTransaction 在事务中执行操作
func (m *MockBadgerStore) RunInTransaction(ctx context.Context, fn func(tx storage.BadgerTransaction) error) error {
	if m.err != nil {
		return m.err
	}
	return fn(nil)
}

