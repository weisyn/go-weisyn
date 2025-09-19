package badger

import (
	"context"
	"testing"
	"time"

	interfaces "github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/dgraph-io/badger/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 创建测试事务
func createTestTransaction(t *testing.T) (*Transaction, *badger.Txn) {
	// 创建测试事务
	store, _, cleanup := setupTestStore(t)
	defer cleanup()

	txn := store.db.NewTransaction(true)
	tx := &Transaction{txn: txn}

	return tx, txn
}

// 测试事务基本操作
func TestTransactionCRUD(t *testing.T) {
	tx, rawTxn := createTestTransaction(t)
	defer rawTxn.Discard()

	// 测试键值
	key := []byte("tx-test-key")
	value := []byte("tx-test-value")

	// 测试设置键值
	err := tx.Set(key, value)
	require.NoError(t, err)

	// 测试获取值
	val, err := tx.Get(key)
	assert.NoError(t, err)
	assert.Equal(t, value, val)

	// 测试键存在
	exists, err := tx.Exists(key)
	assert.NoError(t, err)
	assert.True(t, exists)

	// 测试删除键
	err = tx.Delete(key)
	assert.NoError(t, err)

	// 验证键已删除
	exists, err = tx.Exists(key)
	assert.NoError(t, err)
	assert.False(t, exists)
}

// 测试事务TTL设置
func TestTransactionTTL(t *testing.T) {
	// 创建完整的存储实例而不是单独的事务
	store, _, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// 使用存储的事务API而不是直接操作事务
	key := []byte("tx-ttl-key")
	value := []byte("tx-ttl-value")

	// 设置带TTL的键值
	err := store.SetWithTTL(ctx, key, value, 1*time.Second)
	assert.NoError(t, err)

	// 立即检查键是否存在
	exists, err := store.Exists(ctx, key)
	assert.NoError(t, err)
	assert.True(t, exists)

	// 等待键过期
	time.Sleep(1500 * time.Millisecond)

	// 验证键已过期
	exists, err = store.Exists(ctx, key)
	assert.NoError(t, err)
	assert.False(t, exists)
}

// 测试事务提交
func TestTransactionCommit(t *testing.T) {
	// 创建存储和事务
	store, _, cleanup := setupTestStore(t)
	defer cleanup()

	// 准备两个事务，一个提交一个丢弃
	txn1 := store.db.NewTransaction(true)
	tx1 := &Transaction{txn: txn1}

	txn2 := store.db.NewTransaction(true)
	tx2 := &Transaction{txn: txn2}

	// 在两个事务中设置不同的键值
	key1 := []byte("commit-key")
	value1 := []byte("commit-value")
	key2 := []byte("discard-key")
	value2 := []byte("discard-value")

	// 在事务1中设置键值
	err := tx1.Set(key1, value1)
	require.NoError(t, err)

	// 在事务2中设置键值
	err = tx2.Set(key2, value2)
	require.NoError(t, err)

	// 提交事务1
	err = tx1.Commit()
	assert.NoError(t, err)

	// 丢弃事务2
	tx2.Discard()

	// 验证事务1的键值已提交
	val, err := store.Get(nil, key1)
	assert.NoError(t, err)
	assert.Equal(t, value1, val)

	// 验证事务2的键值未提交
	exists, err := store.Exists(nil, key2)
	assert.NoError(t, err)
	assert.False(t, exists)
}

// 测试事务隔离性
func TestTransactionIsolation(t *testing.T) {
	// 创建存储
	store, _, cleanup := setupTestStore(t)
	defer cleanup()

	// 准备两个并发事务
	txn1 := store.db.NewTransaction(true)
	tx1 := &Transaction{txn: txn1}
	defer txn1.Discard()

	txn2 := store.db.NewTransaction(true)
	tx2 := &Transaction{txn: txn2}
	defer txn2.Discard()

	// 在事务1中设置键值
	key := []byte("isolation-key")
	value1 := []byte("isolation-value-1")
	err := tx1.Set(key, value1)
	require.NoError(t, err)

	// 在事务1提交前，事务2不应该能看到事务1的修改
	exists, err := tx2.Exists(key)
	assert.NoError(t, err)
	assert.False(t, exists)

	// 提交事务1
	err = tx1.Commit()
	assert.NoError(t, err)

	// 创建新事务，应该能看到事务1的修改
	txn3 := store.db.NewTransaction(true)
	tx3 := &Transaction{txn: txn3}
	defer txn3.Discard()

	exists, err = tx3.Exists(key)
	assert.NoError(t, err)
	assert.True(t, exists)

	val, err := tx3.Get(key)
	assert.NoError(t, err)
	assert.Equal(t, value1, val)
}

// 测试Merge方法
func TestTransactionMerge(t *testing.T) {
	// 创建存储和事务
	store, _, cleanup := setupTestStore(t)
	defer cleanup()

	// 使用事务API
	txn := store.db.NewTransaction(true)
	tx := &Transaction{
		txn:   txn,
		state: int32(TxActive),
	}

	// 确保函数结束时丢弃事务
	defer func() {
		if tx.IsActive() {
			tx.Discard()
		}
	}()

	// 测试键值
	key := []byte("merge-test-key")
	initialValue := []byte("hello")

	// 先设置初始值
	err := tx.Set(key, initialValue)
	require.NoError(t, err)

	// 定义合并函数：将两个值连接起来
	mergeFunc := func(existingVal, newVal []byte) []byte {
		if existingVal == nil {
			return newVal
		}
		result := make([]byte, len(existingVal)+len(newVal))
		copy(result, existingVal)
		copy(result[len(existingVal):], newVal)
		return result
	}

	// 执行合并操作
	err = tx.Merge(key, []byte(" world"), mergeFunc)
	require.NoError(t, err)

	// 验证合并结果
	val, err := tx.Get(key)
	assert.NoError(t, err)
	assert.Equal(t, []byte("hello world"), val)

	// 测试合并不存在的键
	newKey := []byte("new-merge-key")
	err = tx.Merge(newKey, []byte("new value"), mergeFunc)
	require.NoError(t, err)

	// 验证结果
	val, err = tx.Get(newKey)
	assert.NoError(t, err)
	assert.Equal(t, []byte("new value"), val)
}

// 测试事务实现接口
func TestTransactionInterface(t *testing.T) {
	tx, rawTxn := createTestTransaction(t)
	defer rawTxn.Discard()

	// 验证Transaction实现了BadgerTransaction接口
	var _ interfaces.BadgerTransaction = tx
}
