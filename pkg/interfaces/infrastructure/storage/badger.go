// Package storage 提供WES系统的BadgerDB存储接口定义
//
// 💾 **BadgerDB存储服务 (BadgerDB Storage Service)**
//
// 本文件定义了WES区块链系统的BadgerDB存储接口，专注于：
// - 高性能存储：BadgerDB的原生高性能键值存储服务
// - 事务支持：支持ACID事务和批量操作
// - 缓存优化：内置缓存机制和内存管理
// - 数据压缩：可选的数据压缩和存储优化
//
// 🎯 **核心功能**
// - BadgerService：BadgerDB存储服务接口，提供完整的数据存储能力
// - 高效读写：优化的读写操作和批量处理
// - 事务管理：支持原子性操作和数据一致性
// - 迭代器：高效的数据遍历和查询机制
//
// 🏧 **设计原则**
// - 性能优先：充分利用BadgerDB的性能优势
// - 内存高效：合理的内存使用和缓存策略
// - 数据安全：支持事务和数据完整性保障
// - 易用性：简洁的接口设计和错误处理
//
// 🔗 **组件关系**
// - BadgerService：被区块、交易、索引等模块使用
// - 与StorageProvider：作为存储提供者的一种实现
// - 与其他存储：与Memory、SQLite等存储引擎并存
package storage

import (
	"context"
	"time"
)

//=============================================================================
// BadgerStore 接口定义
//=============================================================================

// BadgerStore 定义了键值存储的应用接口
// 提供简单易用的键值存储操作，适用于需要高性能键值操作的场景
// 可用于实现缓存、配置存储、数据索引等功能
type BadgerStore interface {
	//-------------------------------------------------------------------------
	// 基础操作
	//-------------------------------------------------------------------------

	//-------------------------------------------------------------------------
	// 生命周期管理
	//-------------------------------------------------------------------------
	
	// Close 关闭BadgerDB数据库连接
	// 确保所有待处理的事务被提交，数据被正确写入磁盘
	// 应用关闭时必须调用此方法以避免数据损坏
	Close() error

	//-------------------------------------------------------------------------
	// 基本键值操作
	//-------------------------------------------------------------------------

	// Get 获取指定键的值
	// 如果键不存在，返回nil值和nil错误
	// 如果发生错误，返回nil值和相应错误
	Get(ctx context.Context, key []byte) ([]byte, error)

	// Set 设置键值对
	// 如果键已存在，将覆盖原有值
	// 如果键不存在，将创建新的键值对
	Set(ctx context.Context, key, value []byte) error

	// SetWithTTL 设置键值对并指定过期时间
	// ttl指定键值对的生存时间，过期后自动删除
	// ttl为0表示永不过期
	SetWithTTL(ctx context.Context, key, value []byte, ttl time.Duration) error

	// Delete 删除指定键的值
	// 如果键不存在，不会返回错误
	Delete(ctx context.Context, key []byte) error

	// Exists 检查键是否存在
	// 返回true表示键存在，false表示键不存在
	Exists(ctx context.Context, key []byte) (bool, error)

	//-------------------------------------------------------------------------
	// 批量操作
	//-------------------------------------------------------------------------

	// GetMany 批量获取多个键的值
	// 对于不存在的键，不会包含在返回结果中
	// 返回map的键为键的字符串表示
	GetMany(ctx context.Context, keys [][]byte) (map[string][]byte, error)

	// SetMany 批量设置多个键值对
	// map的键为键的字符串表示，值为要设置的二进制数据
	SetMany(ctx context.Context, entries map[string][]byte) error

	// DeleteMany 批量删除多个键
	// 对于不存在的键，不会返回错误
	DeleteMany(ctx context.Context, keys [][]byte) error

	//-------------------------------------------------------------------------
	// 扫描操作
	//-------------------------------------------------------------------------

	// PrefixScan 按前缀扫描键值对
	// 返回所有键以指定前缀开头的键值对
	// 返回map的键为键的字符串表示
	PrefixScan(ctx context.Context, prefix []byte) (map[string][]byte, error)

	// RangeScan 范围扫描键值对
	// 返回键在[startKey, endKey)范围内的所有键值对（包含startKey，不包含endKey）
	// 返回map的键为键的字符串表示
	RangeScan(ctx context.Context, startKey, endKey []byte) (map[string][]byte, error)

	//-------------------------------------------------------------------------
	// 事务操作
	//-------------------------------------------------------------------------

	// RunInTransaction 在事务中执行操作
	// fn函数在事务上下文中执行，可以执行多个原子操作
	// 如果fn返回错误，事务将被回滚
	// 如果fn成功执行，事务将被提交
	RunInTransaction(ctx context.Context, fn func(tx BadgerTransaction) error) error
}

//=============================================================================
// BadgerTransaction 接口定义
//=============================================================================

// BadgerTransaction 定义了键值存储事务操作接口
// 提供在单个事务中执行多个操作的能力
// 事务保证所有操作要么全部成功，要么全部失败
type BadgerTransaction interface {
	// Get 获取指定键的值
	// 如果键不存在，返回nil值和nil错误
	Get(key []byte) ([]byte, error)

	// Set 设置键值对
	// 如果键已存在，将覆盖原有值
	Set(key, value []byte) error

	// SetWithTTL 设置键值对并指定过期时间
	// ttl指定键值对的生存时间，过期后自动删除
	SetWithTTL(key, value []byte, ttl time.Duration) error

	// Delete 删除指定键的值
	// 如果键不存在，不会返回错误
	Delete(key []byte) error

	// Exists 检查键是否存在
	// 返回true表示键存在，false表示键不存在
	Exists(key []byte) (bool, error)

	// Merge 原子性地合并键的现有值与新值
	// 通过mergeFunc函数定义如何合并现有值与新值
	// 这是一个高效的原子更新操作，适用于计数器、列表追加等场景
	// mergeFunc接收现有值和新值作为参数，返回合并后的值
	// 如果键不存在，mergeFunc的existingVal参数将为nil
	Merge(key, value []byte, mergeFunc func(existingVal, newVal []byte) []byte) error
}
