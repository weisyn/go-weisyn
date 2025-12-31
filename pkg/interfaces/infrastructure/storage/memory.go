// Package storage 提供WES系统的内存存储接口定义
//
// 🧠 **内存存储服务 (Memory Storage Service)**
//
// 本文件定义了WES区块链系统的内存存储接口，专注于：
// - 高速缓存：基于内存的高速数据缓存和临时存储
// - 热数据管理：频繁访问数据的内存缓存优化
// - 生命周期控制：支持TTL过期和自动清理机制
// - 多引擎支持：可基于Redis、Memcached、BigCache等实现
//
// 🎯 **核心功能**
// - MemoryService：内存存储服务接口，提供完整的内存数据管理
// - 缓存策略：支持LRU、LFU等多种缓存淘汰策略
// - 过期管理：灵活的TTL设置和自动过期清理
// - 批量操作：高效的批量读写和删除操作
//
// 🏗️ **设计原则**
// - 性能优先：充分利用内存的高速访问特性
// - 内存高效：合理的内存使用和垃圾回收策略
// - 并发安全：支持高并发的读写操作
// - 易用性：简洁统一的API设计和错误处理
//
// 🔗 **组件关系**
// - MemoryService：被缓存、会话、临时数据等模块使用
// - 与StorageProvider：作为存储提供者的高速缓存层
// - 与其他存储：配合BadgerDB、SQLite提供分层存储
package storage

import (
	"context"
	"time"
)

// ============================================================================= //nolint:gocritic // commentFormatting: 分隔线格式已修复
// MemoryStore 接口定义
// =============================================================================

// MemoryStore 定义了通用的内存缓存接口
//
// 提供WES区块链系统的高速内存存储服务：
// - 快速缓存：频繁访问数据的内存级缓存存储
// - 会话管理：用户会话和临时状态的内存存储
// - 热数据缓存：计算结果和查询结果的高速缓存
// - 生命周期管理：支持TTL过期和自动清理机制
type MemoryStore interface {
	//-------------------------------------------------------------------------
	// 生命周期管理
	//-------------------------------------------------------------------------

	//-------------------------------------------------------------------------
	// 基本操作
	//-------------------------------------------------------------------------
	// 注意：内存存储资源由DI容器自动管理，无需手动Close()

	// Get 获取缓存值，返回值、是否存在及可能的错误
	// value: 缓存的二进制数据
	// exists: true表示键存在，false表示键不存在
	// err: 操作过程中发生的错误，nil表示无错误
	Get(ctx context.Context, key string) (value []byte, exists bool, err error)

	// Set 设置缓存值，可指定过期时间
	// key: 缓存键名
	// value: 要缓存的二进制数据
	// ttl: 生存时间，0表示永不过期
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Delete 删除指定键的缓存
	// 如果键不存在，通常不会返回错误
	Delete(ctx context.Context, key string) error

	// Exists 检查键是否存在
	// 返回true表示键存在，false表示键不存在
	Exists(ctx context.Context, key string) (bool, error)

	//-------------------------------------------------------------------------
	// 批量操作
	//-------------------------------------------------------------------------

	// GetMany 批量获取多个键的值
	// keys: 要获取的键名列表
	// 返回的map仅包含存在的键值对，不存在的键不会出现在结果中
	GetMany(ctx context.Context, keys []string) (map[string][]byte, error)

	// SetMany 批量设置多个键值对，使用相同的TTL
	// items: 键值对集合，键为缓存键名，值为要缓存的二进制数据
	// ttl: 所有键值对的统一生存时间，0表示永不过期
	SetMany(ctx context.Context, items map[string][]byte, ttl time.Duration) error

	// DeleteMany 批量删除多个键
	// keys: 要删除的键名列表
	// 对于不存在的键，通常不会返回错误
	DeleteMany(ctx context.Context, keys []string) error

	//-------------------------------------------------------------------------
	// 缓存管理
	//-------------------------------------------------------------------------

	// Clear 清空所有缓存
	// 删除缓存中的所有键值对
	Clear(ctx context.Context) error

	// DeleteByPattern 根据模式删除缓存
	// pattern: 支持通配符的模式字符串，如 "user:*", "cache:?:data"
	// 返回删除的键数量和可能的错误
	DeleteByPattern(ctx context.Context, pattern string) (int64, error)

	// GetKeys 获取匹配模式的所有键
	// pattern: 支持通配符的模式字符串，如 "user:*", "*:cache"
	// 返回匹配的键列表，空模式表示获取所有键
	GetKeys(ctx context.Context, pattern string) ([]string, error)

	// GetTTL 获取键的剩余生存时间
	// 返回键的剩余生存时间
	// 若键不存在或已过期，返回错误
	GetTTL(ctx context.Context, key string) (time.Duration, error)

	// UpdateTTL 更新键的过期时间
	// 为已存在的键设置新的过期时间
	// 若键不存在，返回错误
	UpdateTTL(ctx context.Context, key string, ttl time.Duration) error

	// Count 获取当前缓存中的键数量
	// 返回缓存中当前有效的键数量
	Count(ctx context.Context) (int64, error)
}
