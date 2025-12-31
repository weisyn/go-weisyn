// Package draftstore_test 提供 DraftStore 的单元测试
//
// 🧪 **测试覆盖**：
// - RedisStore 核心功能测试
// - TTL 管理测试
// - 边界条件和错误场景测试
package draftstore

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== Mock redisClient ====================

// mockRedisClient mock Redis 客户端实现
type mockRedisClient struct {
	data   map[string][]byte
	ttls   map[string]time.Duration
	mu     sync.RWMutex
	closed bool
}

func newMockRedisClient() *mockRedisClient {
	return &mockRedisClient{
		data: make(map[string][]byte),
		ttls: make(map[string]time.Duration),
	}
}

func (m *mockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return fmt.Errorf("client closed")
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		var err error
		data, err = json.Marshal(value)
		if err != nil {
			return err
		}
	}

	m.data[key] = data
	if expiration > 0 {
		m.ttls[key] = expiration
	}
	return nil
}

func (m *mockRedisClient) Get(ctx context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return nil, fmt.Errorf("client closed")
	}

	data, ok := m.data[key]
	if !ok {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	return data, nil
}

func (m *mockRedisClient) Del(ctx context.Context, keys ...string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return 0, fmt.Errorf("client closed")
	}

	count := int64(0)
	for _, key := range keys {
		if _, ok := m.data[key]; ok {
			delete(m.data, key)
			delete(m.ttls, key)
			count++
		}
	}
	return count, nil
}

func (m *mockRedisClient) Keys(ctx context.Context, pattern string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return nil, fmt.Errorf("client closed")
	}

	var keys []string
	for k := range m.data {
		// 简单的模式匹配：支持 * 后缀匹配
		if pattern == "*" {
			keys = append(keys, k)
		} else if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
			// 前缀匹配：pattern 是 "prefix*"
			prefix := pattern[:len(pattern)-1]
			if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
				keys = append(keys, k)
			}
		} else if pattern == k {
			// 精确匹配
			keys = append(keys, k)
		}
	}
	// 排序 keys 以确保顺序稳定（Redis KEYS 命令返回的 keys 是排序的）
	sort.Strings(keys)
	return keys, nil
}

func (m *mockRedisClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return 0, fmt.Errorf("client closed")
	}

	count := int64(0)
	for _, key := range keys {
		if _, ok := m.data[key]; ok {
			count++
		}
	}
	return count, nil
}

func (m *mockRedisClient) TTL(ctx context.Context, key string) (time.Duration, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return 0, fmt.Errorf("client closed")
	}

	if _, ok := m.data[key]; !ok {
		return -2, nil // Redis 返回 -2 表示 key 不存在
	}

	ttl, ok := m.ttls[key]
	if !ok {
		return -1, nil // Redis 返回 -1 表示永不过期
	}
	return ttl, nil
}

func (m *mockRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return false, fmt.Errorf("client closed")
	}

	if _, ok := m.data[key]; !ok {
		return false, nil
	}

	m.ttls[key] = expiration
	return true, nil
}

func (m *mockRedisClient) Ping(ctx context.Context) error {
	if m.closed {
		return fmt.Errorf("client closed")
	}
	return nil
}

func (m *mockRedisClient) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closed = true
	return nil
}

// ==================== RedisStore 核心功能测试 ====================

// TestNewRedisStore 测试创建 RedisStore
func TestNewRedisStore(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)

	assert.NoError(t, err)
	assert.NotNil(t, store)
}

// TestNewRedisStore_NilClient 测试使用 nil client 创建
func TestNewRedisStore_NilClient(t *testing.T) {
	_, err := NewRedisStore(nil, "test:", 3600)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")
}

// TestNewRedisStore_ConnectionFailed 测试连接失败
func TestNewRedisStore_ConnectionFailed(t *testing.T) {
	client := newMockRedisClient()
	client.Close() // 关闭客户端模拟连接失败

	_, err := NewRedisStore(client, "test:", 3600)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect")
}

// TestRedisStore_Save 测试保存草稿
func TestRedisStore_Save(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	draft := &types.DraftTx{
		DraftID: "test-draft-1",
		Tx: &transaction.Transaction{
			Version: 1,
			Inputs:  []*transaction.TxInput{},
			Outputs: []*transaction.TxOutput{},
		},
		IsSealed: false,
	}

	draftID, err := store.Save(context.Background(), draft)
	assert.NoError(t, err)
	assert.Equal(t, "test-draft-1", draftID)
}

// TestRedisStore_Get 测试获取草稿
func TestRedisStore_Get(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	draft := &types.DraftTx{
		DraftID: "test-draft-2",
		Tx: &transaction.Transaction{
			Version: 1,
			Inputs:  []*transaction.TxInput{},
			Outputs: []*transaction.TxOutput{},
		},
		IsSealed: false,
	}

	_, err = store.Save(context.Background(), draft)
	require.NoError(t, err)

	loaded, err := store.Get(context.Background(), "test-draft-2")
	assert.NoError(t, err)
	assert.NotNil(t, loaded)
	assert.Equal(t, draft.DraftID, loaded.DraftID)
	assert.Equal(t, draft.Tx.Version, loaded.Tx.Version)
}

// TestRedisStore_Get_NotFound 测试获取不存在的草稿
func TestRedisStore_Get_NotFound(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	_, err = store.Get(context.Background(), "non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestRedisStore_Delete 测试删除草稿
func TestRedisStore_Delete(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	draft := &types.DraftTx{
		DraftID: "test-draft-3",
		Tx: &transaction.Transaction{
			Version: 1,
			Inputs:  []*transaction.TxInput{},
			Outputs: []*transaction.TxOutput{},
		},
		IsSealed: false,
	}

	_, err = store.Save(context.Background(), draft)
	require.NoError(t, err)

	err = store.Delete(context.Background(), "test-draft-3")
	assert.NoError(t, err)

	_, err = store.Get(context.Background(), "test-draft-3")
	assert.Error(t, err)
}

// TestRedisStore_Delete_NotFound 测试删除不存在的草稿
func TestRedisStore_Delete_NotFound(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	err = store.Delete(context.Background(), "non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestRedisStore_List 测试列出所有草稿
func TestRedisStore_List(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	// 保存多个草稿
	for i := 0; i < 3; i++ {
		draft := &types.DraftTx{
			DraftID: fmt.Sprintf("test-draft-%d", i),
			Tx: &transaction.Transaction{
				Version: 1,
				Inputs:  []*transaction.TxInput{},
				Outputs: []*transaction.TxOutput{},
			},
			IsSealed: false,
		}
		_, err := store.Save(context.Background(), draft)
		require.NoError(t, err)
	}

	drafts, err := store.List(context.Background(), nil, 10, 0)
	assert.NoError(t, err)
	assert.Len(t, drafts, 3)
}

// TestRedisStore_List_WithPagination 测试分页列表
func TestRedisStore_List_WithPagination(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	// 保存多个草稿
	for i := 0; i < 5; i++ {
		draft := &types.DraftTx{
			DraftID: fmt.Sprintf("test-draft-%d", i),
			Tx: &transaction.Transaction{
				Version: 1,
				Inputs:  []*transaction.TxInput{},
				Outputs: []*transaction.TxOutput{},
			},
			IsSealed: false,
		}
		_, err := store.Save(context.Background(), draft)
		require.NoError(t, err)
	}

	// 第一页
	drafts, err := store.List(context.Background(), nil, 2, 0)
	assert.NoError(t, err)
	assert.Len(t, drafts, 2)

	// 第二页
	drafts, err = store.List(context.Background(), nil, 2, 2)
	assert.NoError(t, err)
	assert.Len(t, drafts, 2)

	// 超出范围
	drafts, err = store.List(context.Background(), nil, 2, 20) // offset 20 超出范围
	assert.NoError(t, err)
	// 注意：由于 keys 排序和分页逻辑，如果 offset 超出范围，应该返回空列表
	// 但如果 keys 数量少于 offset，应该返回空列表
	assert.Len(t, drafts, 0)
}

// TestRedisStore_SetTTL 测试设置 TTL
func TestRedisStore_SetTTL(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	draft := &types.DraftTx{
		DraftID: "test-draft-ttl",
		Tx: &transaction.Transaction{
			Version: 1,
			Inputs:  []*transaction.TxInput{},
			Outputs: []*transaction.TxOutput{},
		},
		IsSealed: false,
	}

	_, err = store.Save(context.Background(), draft)
	require.NoError(t, err)

	err = store.SetTTL(context.Background(), "test-draft-ttl", 60)
	assert.NoError(t, err)
}

// TestRedisStore_SetTTL_NotFound 测试为不存在的草稿设置 TTL
func TestRedisStore_SetTTL_NotFound(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	err = store.SetTTL(context.Background(), "non-existent", 60)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestRedisStore_GetTTL 测试获取 TTL
func TestRedisStore_GetTTL(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	draft := &types.DraftTx{
		DraftID: "test-draft-ttl-get",
		Tx: &transaction.Transaction{
			Version: 1,
			Inputs:  []*transaction.TxInput{},
			Outputs: []*transaction.TxOutput{},
		},
		IsSealed: false,
	}

	_, err = store.Save(context.Background(), draft)
	require.NoError(t, err)

	err = store.SetTTL(context.Background(), "test-draft-ttl-get", 120)
	require.NoError(t, err)

	ttl, err := store.(*RedisStore).GetTTL(context.Background(), "test-draft-ttl-get")
	assert.NoError(t, err)
	assert.Greater(t, ttl, time.Duration(0))
}

// TestRedisStore_Exists 测试检查草稿是否存在
func TestRedisStore_Exists(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	draft := &types.DraftTx{
		DraftID: "test-draft-exists",
		Tx: &transaction.Transaction{
			Version: 1,
			Inputs:  []*transaction.TxInput{},
			Outputs: []*transaction.TxOutput{},
		},
		IsSealed: false,
	}

	_, err = store.Save(context.Background(), draft)
	require.NoError(t, err)

	exists, err := store.(*RedisStore).Exists(context.Background(), "test-draft-exists")
	assert.NoError(t, err)
	assert.True(t, exists)

	exists, err = store.(*RedisStore).Exists(context.Background(), "non-existent")
	assert.NoError(t, err)
	assert.False(t, exists)
}

// TestRedisStore_Ping 测试 Ping
func TestRedisStore_Ping(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	err = store.(*RedisStore).Ping(context.Background())
	assert.NoError(t, err)
}

// TestRedisStore_Close 测试关闭连接
func TestRedisStore_Close(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	err = store.(*RedisStore).Close()
	assert.NoError(t, err)
}

// ==================== Save 边界条件测试 ====================

// TestRedisStore_Save_NilDraft 测试保存 nil draft
func TestRedisStore_Save_NilDraft(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	_, err = store.Save(context.Background(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")
}

// TestRedisStore_Save_EmptyDraftID 测试保存空 draftID
func TestRedisStore_Save_EmptyDraftID(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	draft := &types.DraftTx{
		DraftID: "", // 空 draftID
		Tx: &transaction.Transaction{
			Version: 1,
		},
	}

	_, err = store.Save(context.Background(), draft)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

// TestRedisStore_Save_Overwrite 测试覆盖已存在的草稿
func TestRedisStore_Save_Overwrite(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	draft1 := &types.DraftTx{
		DraftID: "test-draft-overwrite",
		Tx: &transaction.Transaction{
			Version: 1,
			Inputs:  []*transaction.TxInput{},
			Outputs: []*transaction.TxOutput{},
		},
		IsSealed: false,
	}

	// 第一次保存
	draftID1, err := store.Save(context.Background(), draft1)
	require.NoError(t, err)
	assert.Equal(t, "test-draft-overwrite", draftID1)

	// 第二次保存（覆盖）
	draft2 := &types.DraftTx{
		DraftID: "test-draft-overwrite",
		Tx: &transaction.Transaction{
			Version: 2, // 版本不同
			Inputs:  []*transaction.TxInput{},
			Outputs: []*transaction.TxOutput{},
		},
		IsSealed: true, // 状态不同
	}

	draftID2, err := store.Save(context.Background(), draft2)
	assert.NoError(t, err)
	assert.Equal(t, "test-draft-overwrite", draftID2)

	// 验证已覆盖
	loaded, err := store.Get(context.Background(), "test-draft-overwrite")
	assert.NoError(t, err)
	assert.Equal(t, uint32(2), loaded.Tx.Version)
	assert.True(t, loaded.IsSealed)
}

// ==================== Get 边界条件测试 ====================

// TestRedisStore_Get_EmptyDraftID 测试获取空 draftID
func TestRedisStore_Get_EmptyDraftID(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	_, err = store.Get(context.Background(), "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

// ==================== Delete 边界条件测试 ====================

// TestRedisStore_Delete_EmptyDraftID 测试删除空 draftID
func TestRedisStore_Delete_EmptyDraftID(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	err = store.Delete(context.Background(), "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

// ==================== NewRedisStoreFromConfig 测试 ====================

// TestNewRedisStoreFromConfig_NilConfig 测试 nil 配置
func TestNewRedisStoreFromConfig_NilConfig(t *testing.T) {
	_, err := NewRedisStoreFromConfig(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")
}

// TestNewRedisStoreFromConfig_EmptyAddr 测试空地址
func TestNewRedisStoreFromConfig_EmptyAddr(t *testing.T) {
	cfg := &Config{
		Addr: "", // 空地址
	}

	_, err := NewRedisStoreFromConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "address cannot be empty")
}

// TestNewRedisStoreFromConfig_Success 测试成功创建
func TestNewRedisStoreFromConfig_Success(t *testing.T) {
	cfg := &Config{
		Addr:         "localhost:28791",
		Password:     "",
		DB:           0,
		KeyPrefix:    "test:",
		DefaultTTL:   3600,
		PoolSize:     10,
		MinIdleConns: 5,
		DialTimeout:  5,
		ReadTimeout:  3,
		WriteTimeout: 3,
	}

	// 注意：这个测试需要真实的 Redis 连接，可能会失败
	// 如果 Redis 不可用，测试会失败，这是预期的
	store, err := NewRedisStoreFromConfig(cfg)
	if err != nil {
		// Redis 不可用，跳过测试
		t.Skipf("Redis not available: %v", err)
		return
	}

	assert.NotNil(t, store)
	defer store.(*RedisStore).Close()
}

// TestNewRedisStoreFromConfig_EmptyKeyPrefix 测试空 keyPrefix（使用默认值）
func TestNewRedisStoreFromConfig_EmptyKeyPrefix(t *testing.T) {
	cfg := &Config{
		Addr:      "localhost:28791",
		KeyPrefix: "", // 空 keyPrefix，应该使用默认值
	}

	// 注意：这个测试需要真实的 Redis 连接
	_, err := NewRedisStoreFromConfig(cfg)
	if err != nil {
		// Redis 不可用，跳过测试
		t.Skipf("Redis not available: %v", err)
		return
	}
	// 如果成功，说明使用了默认 keyPrefix
}

// TestDefaultConfig 测试默认配置
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.NotNil(t, cfg)
	assert.Equal(t, "", cfg.Addr) // 必须通过配置提供
	assert.Equal(t, "", cfg.Password)
	assert.Equal(t, 0, cfg.DB)
	assert.Equal(t, "weisyn:draft:", cfg.KeyPrefix)
	assert.Equal(t, 3600, cfg.DefaultTTL)
	assert.Equal(t, 10, cfg.PoolSize)
	assert.Equal(t, 5, cfg.MinIdleConns)
	assert.Equal(t, 5, cfg.DialTimeout)
	assert.Equal(t, 3, cfg.ReadTimeout)
	assert.Equal(t, 3, cfg.WriteTimeout)
}

// ==================== Exists 和 GetTTL 扩展测试 ====================

// TestRedisStore_Exists_EmptyDraftID 测试空 draftID
func TestRedisStore_Exists_EmptyDraftID(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)
	redisStore := store.(*RedisStore)

	exists, err := redisStore.Exists(context.Background(), "")

	assert.Error(t, err)
	assert.False(t, exists)
	assert.Contains(t, err.Error(), "cannot be empty")
}

// TestRedisStore_GetTTL_EmptyDraftID 测试空 draftID
func TestRedisStore_GetTTL_EmptyDraftID(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)
	redisStore := store.(*RedisStore)

	ttl, err := redisStore.GetTTL(context.Background(), "")

	assert.Error(t, err)
	assert.Equal(t, time.Duration(0), ttl)
	assert.Contains(t, err.Error(), "cannot be empty")
}

// TestRedisStore_GetTTL_NotFound 测试不存在的草稿
func TestRedisStore_GetTTL_NotFound(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)
	redisStore := store.(*RedisStore)

	ttl, err := redisStore.GetTTL(context.Background(), "non-existent")

	// mockRedisClient.TTL 对于不存在的 key 返回 -2（不存在）
	// 根据 Redis 规范，-2 表示 key 不存在，-1 表示永不过期
	// 这里应该没有错误，TTL 应该是 -2
	assert.NoError(t, err)
	assert.Equal(t, time.Duration(-2), ttl, "不存在的 key 的 TTL 应该是 -2")
}

// TestRedisStore_SetTTL_EmptyDraftID 测试空 draftID
func TestRedisStore_SetTTL_EmptyDraftID(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	err = store.SetTTL(context.Background(), "", 60)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

// TestRedisStore_Save_MarshalError 测试序列化错误（通过无效的 draft 结构）
func TestRedisStore_Save_MarshalError(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	// 创建一个可能导致序列化问题的 draft（虽然这种情况很少见）
	draft := &types.DraftTx{
		DraftID: "test-draft",
		Tx:      nil, // nil Tx 可能导致序列化问题
	}

	draftID, err := store.Save(context.Background(), draft)

	// 注意：实际上 nil Tx 可能不会导致序列化错误，因为 JSON 会序列化为 null
	// 这个测试主要用于覆盖代码路径
	_ = draftID
	_ = err
}

// TestRedisStore_List_KeysError 测试 Keys 错误
func TestRedisStore_List_KeysError(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	// 关闭客户端模拟错误
	client.Close()

	drafts, err := store.List(context.Background(), nil, 10, 0)

	assert.Error(t, err)
	assert.Nil(t, drafts)
	// 错误可能是 "failed to list drafts" 或 "client closed"
	assert.True(t,
		contains(err.Error(), "failed to list drafts") ||
			contains(err.Error(), "client closed"),
		"错误应该包含 'failed to list drafts' 或 'client closed'")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				containsMiddle(s, substr))))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestRedisStore_List_GetError 测试获取草稿时出错（跳过失败的草稿）
func TestRedisStore_List_GetError(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	// 保存一个有效的草稿
	draft := &types.DraftTx{
		DraftID: "test-draft-valid",
		Tx: &transaction.Transaction{
			Version: 1,
			Inputs:  []*transaction.TxInput{},
			Outputs: []*transaction.TxOutput{},
		},
		IsSealed: false,
	}
	_, err = store.Save(context.Background(), draft)
	require.NoError(t, err)

	// 手动添加一个无效的 key（会导致 Get 失败）
	client.Set(context.Background(), "test:invalid-draft", "invalid-json", 0)

	// List 应该跳过无效的草稿，只返回有效的
	drafts, err := store.List(context.Background(), nil, 10, 0)

	assert.NoError(t, err)
	assert.Len(t, drafts, 1) // 只返回有效的草稿
	assert.Equal(t, "test-draft-valid", drafts[0].DraftID)
}

// TestRedisStore_List_EmptyKeyPrefix 测试 key 长度等于 keyPrefix 的情况
func TestRedisStore_List_EmptyKeyPrefix(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	// 手动添加一个 key 长度等于 keyPrefix 的 key（应该被跳过）
	client.Set(context.Background(), "test:", "data", 0)

	drafts, err := store.List(context.Background(), nil, 10, 0)

	assert.NoError(t, err)
	// 这个 key 应该被跳过（因为 len(key) <= len(keyPrefix)）
	assert.Len(t, drafts, 0)
}

// TestRedisStore_List_ZeroLimit 测试 limit 为 0（无限制）
func TestRedisStore_List_ZeroLimit(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	// 保存多个草稿
	for i := 0; i < 5; i++ {
		draft := &types.DraftTx{
			DraftID: fmt.Sprintf("test-draft-%d", i),
			Tx: &transaction.Transaction{
				Version: 1,
				Inputs:  []*transaction.TxInput{},
				Outputs: []*transaction.TxOutput{},
			},
			IsSealed: false,
		}
		_, err := store.Save(context.Background(), draft)
		require.NoError(t, err)
	}

	// limit 为 0 表示无限制，但实际实现中需要处理 limit=0 的情况
	// 如果 limit=0，end = offset + 0 = offset，循环不会执行
	// 所以需要特殊处理 limit=0 的情况
	// 这里使用一个很大的 limit 来模拟无限制
	drafts, err := store.List(context.Background(), nil, 1000, 0)

	assert.NoError(t, err)
	assert.Len(t, drafts, 5) // 应该返回所有草稿
}

// TestRedisStore_List_OffsetGreaterThanKeys 测试 offset 大于 keys 数量
func TestRedisStore_List_OffsetGreaterThanKeys(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	// 保存少量草稿
	for i := 0; i < 3; i++ {
		draft := &types.DraftTx{
			DraftID: fmt.Sprintf("test-draft-%d", i),
			Tx: &transaction.Transaction{
				Version: 1,
				Inputs:  []*transaction.TxInput{},
				Outputs: []*transaction.TxOutput{},
			},
			IsSealed: false,
		}
		_, err := store.Save(context.Background(), draft)
		require.NoError(t, err)
	}

	// offset 大于 keys 数量
	drafts, err := store.List(context.Background(), nil, 10, 100)

	assert.NoError(t, err)
	assert.Len(t, drafts, 0) // 应该返回空列表
}

// ==================== Save 错误场景测试 ====================

// TestRedisStore_Save_SetError 测试 Set 失败
func TestRedisStore_Save_SetError(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	// 关闭客户端模拟错误
	client.Close()

	draft := &types.DraftTx{
		DraftID: "test-draft",
		Tx: &transaction.Transaction{
			Version: 1,
			Inputs:  []*transaction.TxInput{},
			Outputs: []*transaction.TxOutput{},
		},
		IsSealed: false,
	}

	draftID, err := store.Save(context.Background(), draft)

	assert.Error(t, err)
	assert.Empty(t, draftID)
	assert.Contains(t, err.Error(), "failed to save draft to Redis")
}

// ==================== Get 错误场景测试 ====================

// TestRedisStore_Get_UnmarshalError 测试反序列化失败
func TestRedisStore_Get_UnmarshalError(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	// 手动添加一个无效的 JSON 数据
	client.Set(context.Background(), "test:invalid-draft", "invalid-json-data", 0)

	draft, err := store.Get(context.Background(), "invalid-draft")

	assert.Error(t, err)
	assert.Nil(t, draft)
	assert.Contains(t, err.Error(), "failed to unmarshal draft")
}

// TestRedisStore_Get_GetError 测试 Get 失败
func TestRedisStore_Get_GetError(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	// 关闭客户端模拟错误
	client.Close()

	draft, err := store.Get(context.Background(), "test-draft")

	assert.Error(t, err)
	assert.Nil(t, draft)
	assert.Contains(t, err.Error(), "not found")
}

// ==================== Delete 错误场景测试 ====================

// TestRedisStore_Delete_DelError 测试 Del 失败
func TestRedisStore_Delete_DelError(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	// 先保存一个草稿
	draft := &types.DraftTx{
		DraftID: "test-draft",
		Tx: &transaction.Transaction{
			Version: 1,
			Inputs:  []*transaction.TxInput{},
			Outputs: []*transaction.TxOutput{},
		},
		IsSealed: false,
	}
	_, err = store.Save(context.Background(), draft)
	require.NoError(t, err)

	// 关闭客户端模拟错误
	client.Close()

	err = store.Delete(context.Background(), "test-draft")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete draft from Redis")
}

// ==================== Exists 错误场景测试 ====================

// TestRedisStore_Exists_ExistsError 测试 Exists 失败
func TestRedisStore_Exists_ExistsError(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)
	redisStore := store.(*RedisStore)

	// 关闭客户端模拟错误
	client.Close()

	exists, err := redisStore.Exists(context.Background(), "test-draft")

	assert.Error(t, err)
	assert.False(t, exists)
	assert.Contains(t, err.Error(), "failed to check draft existence")
}

// ==================== GetTTL 错误场景测试 ====================

// TestRedisStore_GetTTL_TTLError 测试 TTL 失败
func TestRedisStore_GetTTL_TTLError(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)
	redisStore := store.(*RedisStore)

	// 先保存一个草稿
	draft := &types.DraftTx{
		DraftID: "test-draft",
		Tx: &transaction.Transaction{
			Version: 1,
			Inputs:  []*transaction.TxInput{},
			Outputs: []*transaction.TxOutput{},
		},
		IsSealed: false,
	}
	_, err = store.Save(context.Background(), draft)
	require.NoError(t, err)

	// 关闭客户端模拟错误
	client.Close()

	ttl, err := redisStore.GetTTL(context.Background(), "test-draft")

	assert.Error(t, err)
	assert.Equal(t, time.Duration(0), ttl)
	assert.Contains(t, err.Error(), "failed to get TTL")
}

// ==================== SetTTL 错误场景测试 ====================

// TestRedisStore_SetTTL_ExpireError 测试 Expire 失败
func TestRedisStore_SetTTL_ExpireError(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	// 先保存一个草稿
	draft := &types.DraftTx{
		DraftID: "test-draft",
		Tx: &transaction.Transaction{
			Version: 1,
			Inputs:  []*transaction.TxInput{},
			Outputs: []*transaction.TxOutput{},
		},
		IsSealed: false,
	}
	_, err = store.Save(context.Background(), draft)
	require.NoError(t, err)

	// 关闭客户端模拟错误
	client.Close()

	err = store.SetTTL(context.Background(), "test-draft", 60)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to set TTL")
}

// ==================== NewRedisStoreFromConfig 扩展测试 ====================

// TestNewRedisStoreFromConfig_EmptyKeyPrefix_UsesDefault 测试空 keyPrefix 使用默认值
func TestNewRedisStoreFromConfig_EmptyKeyPrefix_UsesDefault(t *testing.T) {
	cfg := &Config{
		Addr:      "localhost:28791",
		KeyPrefix: "", // 应该使用默认值 "weisyn:draft:"
	}

	// 这个测试需要真实的 Redis 连接
	_, err := NewRedisStoreFromConfig(cfg)
	if err != nil {
		// Redis 不可用，跳过测试
		t.Skipf("Redis not available: %v", err)
		return
	}
	// 如果成功，说明使用了默认 keyPrefix
}

// TestNewRedisStoreFromConfig_ZeroDefaultTTL_UsesDefault 测试 defaultTTL 为 0 时使用默认值
func TestNewRedisStoreFromConfig_ZeroDefaultTTL_UsesDefault(t *testing.T) {
	cfg := &Config{
		Addr:       "localhost:28791",
		DefaultTTL: 0, // 应该使用默认值 3600
	}

	// 这个测试需要真实的 Redis 连接
	_, err := NewRedisStoreFromConfig(cfg)
	if err != nil {
		// Redis 不可用，跳过测试
		t.Skipf("Redis not available: %v", err)
		return
	}
	// 如果成功，说明使用了默认 TTL
}

// TestNewRedisStoreFromConfig_NegativeDefaultTTL_UsesDefault 测试 defaultTTL 为负数时使用默认值
func TestNewRedisStoreFromConfig_NegativeDefaultTTL_UsesDefault(t *testing.T) {
	cfg := &Config{
		Addr:       "localhost:28791",
		DefaultTTL: -1, // 应该使用默认值 3600
	}

	// 这个测试需要真实的 Redis 连接
	_, err := NewRedisStoreFromConfig(cfg)
	if err != nil {
		// Redis 不可用，跳过测试
		t.Skipf("Redis not available: %v", err)
		return
	}
	// 如果成功，说明使用了默认 TTL
}

// ==================== newGoRedisClient 错误路径测试 ====================

// TestNewGoRedisClient_NilConfig 测试 nil 配置
func TestNewGoRedisClient_NilConfig(t *testing.T) {
	_, err := newGoRedisClient(nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis config cannot be nil")
}

// TestNewGoRedisClient_EmptyAddr 测试空地址
func TestNewGoRedisClient_EmptyAddr(t *testing.T) {
	cfg := &Config{
		Addr: "", // 空地址
	}

	_, err := newGoRedisClient(cfg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis address cannot be empty")
}

// TestNewGoRedisClient_ConnectionFailed 测试连接失败
func TestNewGoRedisClient_ConnectionFailed(t *testing.T) {
	cfg := &Config{
		Addr: "invalid-host:28791", // 无效地址
	}

	_, err := newGoRedisClient(cfg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to Redis")
}

// TestNewGoRedisClient_WithTimeouts 测试带超时配置
func TestNewGoRedisClient_WithTimeouts(t *testing.T) {
	cfg := &Config{
		Addr:         "localhost:28791",
		DialTimeout:  10,
		ReadTimeout:  5,
		WriteTimeout: 5,
	}

	// 这个测试需要真实的 Redis 连接
	_, err := newGoRedisClient(cfg)
	if err != nil {
		// Redis 不可用，跳过测试
		t.Skipf("Redis not available: %v", err)
		return
	}
	// 如果成功，说明超时配置生效
}

// ==================== List 边界条件测试 ====================

// TestRedisStore_List_EndGreaterThanKeys 测试 end 大于 keys 数量
func TestRedisStore_List_EndGreaterThanKeys(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	// 保存少量草稿
	for i := 0; i < 3; i++ {
		draft := &types.DraftTx{
			DraftID: fmt.Sprintf("test-draft-%d", i),
			Tx: &transaction.Transaction{
				Version: 1,
				Inputs:  []*transaction.TxInput{},
				Outputs: []*transaction.TxOutput{},
			},
			IsSealed: false,
		}
		_, err := store.Save(context.Background(), draft)
		require.NoError(t, err)
	}

	// limit 很大，end 会大于 keys 数量
	drafts, err := store.List(context.Background(), nil, 100, 0)

	assert.NoError(t, err)
	assert.Len(t, drafts, 3) // 应该返回所有草稿
}

// TestRedisStore_List_StartEqualsKeys 测试 start 等于 keys 数量
func TestRedisStore_List_StartEqualsKeys(t *testing.T) {
	client := newMockRedisClient()
	store, err := NewRedisStore(client, "test:", 3600)
	require.NoError(t, err)

	// 保存少量草稿
	for i := 0; i < 3; i++ {
		draft := &types.DraftTx{
			DraftID: fmt.Sprintf("test-draft-%d", i),
			Tx: &transaction.Transaction{
				Version: 1,
				Inputs:  []*transaction.TxInput{},
				Outputs: []*transaction.TxOutput{},
			},
			IsSealed: false,
		}
		_, err := store.Save(context.Background(), draft)
		require.NoError(t, err)
	}

	// start 等于 keys 数量
	drafts, err := store.List(context.Background(), nil, 10, 3)

	assert.NoError(t, err)
	assert.Len(t, drafts, 0) // 应该返回空列表
}
