package badger

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	badgerconfig "github.com/weisyn/v1/internal/config/storage/badger"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	interfaces "github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// 模拟BadgerConfig接口
type mockBadgerConfig struct {
	path             string
	valueLogFileSize int64
	valueThreshold   int64
	syncWrites       bool
	autoCompaction   bool
}

func (m *mockBadgerConfig) GetPath() string               { return m.path }
func (m *mockBadgerConfig) GetValueLogFileSize() int64    { return m.valueLogFileSize }
func (m *mockBadgerConfig) GetValueThreshold() int64      { return m.valueThreshold }
func (m *mockBadgerConfig) IsSyncWritesEnabled() bool     { return m.syncWrites }
func (m *mockBadgerConfig) IsAutoCompactionEnabled() bool { return m.autoCompaction }

// 模拟Logger接口
type mockLogger struct{}

func (m *mockLogger) Debug(msg string)                          {}
func (m *mockLogger) Debugf(format string, args ...interface{}) {}
func (m *mockLogger) Info(msg string)                           {}
func (m *mockLogger) Infof(format string, args ...interface{})  {}
func (m *mockLogger) Warn(msg string)                           {}
func (m *mockLogger) Warnf(format string, args ...interface{})  {}
func (m *mockLogger) Error(msg string)                          {}
func (m *mockLogger) Errorf(format string, args ...interface{}) {}
func (m *mockLogger) Fatal(msg string)                          {}
func (m *mockLogger) Fatalf(format string, args ...interface{}) {}
func (m *mockLogger) With(args ...interface{}) log.Logger       { return m }
func (m *mockLogger) Sync() error                               { return nil }
func (m *mockLogger) Close() error                              { return nil }
func (m *mockLogger) GetZapLogger() *zap.Logger                 { return nil }

// 初始化测试环境
func setupTestStore(t *testing.T) (*Store, string, func()) {
	// 创建临时测试目录
	tempDir, err := os.MkdirTemp("", "badger-test")
	require.NoError(t, err)

	// 创建测试配置
	// 创建配置 - 使用新的配置系统
	options := &badgerconfig.BadgerOptions{
		Path:                 tempDir,
		SyncWrites:           false,
		MemTableSize:         1 << 20, // 1MB
		EnableAutoCompaction: false,
	}
	cfg := badgerconfig.New(options)

	// 创建测试日志
	logger := &mockLogger{}

	// 创建存储实例
	store := New(cfg, logger)
	require.NotNil(t, store)

	// 返回清理函数
	cleanup := func() {
		// 注意：新的存储接口已移除Close()方法，资源由DI容器自动管理
		os.RemoveAll(tempDir)
	}

	return store.(*Store), tempDir, cleanup
}

// 测试基本的键值操作
func TestBasicKeyValueOperations(t *testing.T) {
	store, _, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// 测试键值
	key := []byte("test-key")
	value := []byte("test-value")

	// 1. 测试不存在的键
	exists, err := store.Exists(ctx, key)
	assert.NoError(t, err)
	assert.False(t, exists)

	val, err := store.Get(ctx, key)
	assert.NoError(t, err)
	assert.Nil(t, val)

	// 2. 测试设置键值
	err = store.Set(ctx, key, value)
	assert.NoError(t, err)

	// 3. 测试键存在
	exists, err = store.Exists(ctx, key)
	assert.NoError(t, err)
	assert.True(t, exists)

	// 4. 测试获取值
	val, err = store.Get(ctx, key)
	assert.NoError(t, err)
	assert.Equal(t, value, val)

	// 5. 测试更新值
	newValue := []byte("updated-value")
	err = store.Set(ctx, key, newValue)
	assert.NoError(t, err)

	val, err = store.Get(ctx, key)
	assert.NoError(t, err)
	assert.Equal(t, newValue, val)

	// 6. 测试删除键
	err = store.Delete(ctx, key)
	assert.NoError(t, err)

	exists, err = store.Exists(ctx, key)
	assert.NoError(t, err)
	assert.False(t, exists)
}

// 测试键值TTL
func TestKeyValueTTL(t *testing.T) {
	store, _, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// 测试键值
	key := []byte("ttl-key")
	value := []byte("ttl-value")

	// 设置带过期时间的键值
	err := store.SetWithTTL(ctx, key, value, 1*time.Second)
	assert.NoError(t, err)

	// 立即检查，应该存在
	exists, err := store.Exists(ctx, key)
	assert.NoError(t, err)
	assert.True(t, exists)

	// 等待过期
	time.Sleep(1500 * time.Millisecond)

	// 再次检查，应该已过期
	exists, err = store.Exists(ctx, key)
	assert.NoError(t, err)
	assert.False(t, exists)
}

// 测试批量操作
func TestBatchOperations(t *testing.T) {
	store, _, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// 1. 测试批量设置
	entries := map[string][]byte{
		"key1": []byte("value1"),
		"key2": []byte("value2"),
		"key3": []byte("value3"),
	}

	err := store.SetMany(ctx, entries)
	assert.NoError(t, err)

	// 2. 测试批量获取
	keys := [][]byte{
		[]byte("key1"),
		[]byte("key2"),
		[]byte("key3"),
		[]byte("key4"), // 不存在的键
	}

	values, err := store.GetMany(ctx, keys)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(values))
	assert.Equal(t, []byte("value1"), values["key1"])
	assert.Equal(t, []byte("value2"), values["key2"])
	assert.Equal(t, []byte("value3"), values["key3"])
	assert.Nil(t, values["key4"])

	// 3. 测试批量删除
	deleteKeys := [][]byte{
		[]byte("key1"),
		[]byte("key3"),
	}

	err = store.DeleteMany(ctx, deleteKeys)
	assert.NoError(t, err)

	// 验证删除结果
	exists, err := store.Exists(ctx, []byte("key1"))
	assert.NoError(t, err)
	assert.False(t, exists)

	exists, err = store.Exists(ctx, []byte("key2"))
	assert.NoError(t, err)
	assert.True(t, exists)

	exists, err = store.Exists(ctx, []byte("key3"))
	assert.NoError(t, err)
	assert.False(t, exists)
}

// 测试前缀和范围扫描
func TestScan(t *testing.T) {
	store, _, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// 插入测试数据
	keyValues := map[string][]byte{
		"user:1": []byte("Alice"),
		"user:2": []byte("Bob"),
		"user:3": []byte("Charlie"),
		"post:1": []byte("Post 1"),
		"post:2": []byte("Post 2"),
	}

	err := store.SetMany(ctx, keyValues)
	assert.NoError(t, err)

	// 1. 测试前缀扫描
	userPrefix := []byte("user:")
	users, err := store.PrefixScan(ctx, userPrefix)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(users))
	assert.Equal(t, []byte("Alice"), users["user:1"])
	assert.Equal(t, []byte("Bob"), users["user:2"])
	assert.Equal(t, []byte("Charlie"), users["user:3"])

	// 2. 测试范围扫描
	startKey := []byte("user:1")
	endKey := []byte("user:3")
	userRange, err := store.RangeScan(ctx, startKey, endKey)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(userRange))
	assert.Equal(t, []byte("Alice"), userRange["user:1"])
	assert.Equal(t, []byte("Bob"), userRange["user:2"])
	assert.Nil(t, userRange["user:3"]) // 不包含endKey
}

// 测试事务操作
func TestTransaction(t *testing.T) {
	store, _, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// 1. 测试事务提交
	err := store.RunInTransaction(ctx, func(tx interfaces.BadgerTransaction) error {
		// 写入数据
		if err := tx.Set([]byte("tx-key1"), []byte("tx-value1")); err != nil {
			return err
		}
		if err := tx.Set([]byte("tx-key2"), []byte("tx-value2")); err != nil {
			return err
		}
		return nil
	})
	assert.NoError(t, err)

	// 验证提交的数据
	val1, err := store.Get(ctx, []byte("tx-key1"))
	assert.NoError(t, err)
	assert.Equal(t, []byte("tx-value1"), val1)

	val2, err := store.Get(ctx, []byte("tx-key2"))
	assert.NoError(t, err)
	assert.Equal(t, []byte("tx-value2"), val2)

	// 2. 测试事务回滚
	err = store.RunInTransaction(ctx, func(tx interfaces.BadgerTransaction) error {
		// 写入数据
		if err := tx.Set([]byte("tx-key3"), []byte("tx-value3")); err != nil {
			return err
		}
		// 故意返回错误以触发回滚
		return fmt.Errorf("事务回滚测试")
	})
	assert.Error(t, err)

	// 验证回滚的数据不存在
	exists, err := store.Exists(ctx, []byte("tx-key3"))
	assert.NoError(t, err)
	assert.False(t, exists)
}
