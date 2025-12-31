// Package badger 提供基于BadgerDB的存储实现
package badger

import (
	"fmt"
	"sync/atomic"
	"time"

	badgerdb "github.com/dgraph-io/badger/v3"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// 确保 Transaction 实现了 interfaces.BadgerTransaction 接口
var _ storage.BadgerTransaction = (*Transaction)(nil)

// TransactionState 定义事务的状态
type TransactionState int32

const (
	// TxActive 表示事务处于活动状态
	TxActive TransactionState = iota
	// TxCommitted 表示事务已提交
	TxCommitted
	// TxDiscarded 表示事务已丢弃
	TxDiscarded
)

// Transaction 实现BadgerTransaction接口
type Transaction struct {
	txn        *badgerdb.Txn
	state      int32            // 使用atomic操作管理状态
	operations int              // 记录操作次数
	sizeEst    *TxSizeEstimator // 事务大小估算器
}

// Get 获取指定键的值
func (t *Transaction) Get(key []byte) ([]byte, error) {
	if t.getState() != TxActive {
		return nil, fmt.Errorf("事务已关闭")
	}

	item, err := t.txn.Get(key)
	if err != nil {
		if err == badgerdb.ErrKeyNotFound {
			return nil, nil // 键不存在时返回nil值和nil错误
		}
		return nil, err
	}

	// 复制值
	val, err := item.ValueCopy(nil)
	if err != nil {
		return nil, fmt.Errorf("复制键值失败: %w", err)
	}

	t.operations++
	return val, nil
}

// Set 设置键值对
func (t *Transaction) Set(key, value []byte) error {
	if t.getState() != TxActive {
		return fmt.Errorf("事务已关闭")
	}

	if err := t.txn.Set(key, value); err != nil {
		return fmt.Errorf("设置键值失败: %w", err)
	}

	// 记录写入大小
	if t.sizeEst != nil {
		t.sizeEst.AddWrite(len(key), len(value))
	}

	t.operations++
	return nil
}

// SetWithTTL 设置键值对并指定过期时间
func (t *Transaction) SetWithTTL(key, value []byte, ttl time.Duration) error {
	if t.getState() != TxActive {
		return fmt.Errorf("事务已关闭")
	}

	entry := badgerdb.NewEntry(key, value).WithTTL(ttl)
	if err := t.txn.SetEntry(entry); err != nil {
		return fmt.Errorf("设置带TTL的键值失败: %w", err)
	}

	t.operations++
	return nil
}

// Delete 删除指定键的值
func (t *Transaction) Delete(key []byte) error {
	if t.getState() != TxActive {
		return fmt.Errorf("事务已关闭")
	}

	if err := t.txn.Delete(key); err != nil {
		return fmt.Errorf("删除键值失败: %w", err)
	}

	// 记录删除大小
	if t.sizeEst != nil {
		t.sizeEst.AddDelete(len(key))
	}

	t.operations++
	return nil
}

// Exists 检查键是否存在
func (t *Transaction) Exists(key []byte) (bool, error) {
	if t.getState() != TxActive {
		return false, fmt.Errorf("事务已关闭")
	}

	_, err := t.txn.Get(key)
	if err == badgerdb.ErrKeyNotFound {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("检查键存在性失败: %w", err)
	}

	t.operations++
	return true, nil
}

// Merge 原子性地合并键的现有值与新值
// 通过mergeFunc函数定义如何合并现有值与新值
func (t *Transaction) Merge(key, value []byte, mergeFunc func(existingVal, newVal []byte) []byte) error {
	if t.getState() != TxActive {
		return fmt.Errorf("事务已关闭")
	}

	// 获取现有值
	existingVal, err := t.Get(key)
	if err != nil {
		return fmt.Errorf("获取现有值失败: %w", err)
	}

	// 使用合并函数合并值
	mergedVal := mergeFunc(existingVal, value)

	// 设置合并后的值
	if err := t.Set(key, mergedVal); err != nil {
		return fmt.Errorf("设置合并值失败: %w", err)
	}

	return nil
}

// Commit 提交事务
// 将事务中的所有更改应用到数据库
func (t *Transaction) Commit() error {
	if !atomic.CompareAndSwapInt32(&t.state, int32(TxActive), int32(TxCommitted)) {
		if t.getState() == TxCommitted {
			return fmt.Errorf("事务已提交")
		}
		return fmt.Errorf("事务已丢弃，无法提交")
	}

	// 如果没有任何操作，不需要提交
	if t.operations == 0 {
		t.txn.Discard()
		return nil
	}

	// 提交事务
	if err := t.txn.Commit(); err != nil {
		// 提交失败，将状态设回活动状态
		atomic.StoreInt32(&t.state, int32(TxActive))
		return fmt.Errorf("事务提交失败: %w", err)
	}

	return nil
}

// Discard 丢弃事务
// 丢弃事务中的所有更改
func (t *Transaction) Discard() {
	// 只有活动状态的事务才能丢弃
	if atomic.CompareAndSwapInt32(&t.state, int32(TxActive), int32(TxDiscarded)) {
		t.txn.Discard()
	}
}

// getState 获取事务当前状态
func (t *Transaction) getState() TransactionState {
	return TransactionState(atomic.LoadInt32(&t.state))
}

// IsActive 检查事务是否处于活动状态
func (t *Transaction) IsActive() bool {
	return t.getState() == TxActive
}

// IsCommitted 检查事务是否已提交
func (t *Transaction) IsCommitted() bool {
	return t.getState() == TxCommitted
}

// IsDiscarded 检查事务是否已丢弃
func (t *Transaction) IsDiscarded() bool {
	return t.getState() == TxDiscarded
}

// GetSizeEstimator 获取事务大小估算器
//
// 返回：
//   - storage.TxSizeEstimator: 大小估算器接口，如果未启用则返回nil
func (t *Transaction) GetSizeEstimator() storage.TxSizeEstimator {
	return t.sizeEst
}
