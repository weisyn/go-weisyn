// Package memory 提供基于BigCache的内存缓存实现
package memory

import (
	"context"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/allegro/bigcache/v3"
	memoryconfig "github.com/weisyn/v1/internal/config/storage/memory"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	storage "github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// TTL前缀，用于在缓存键中存储TTL信息
const ttlPrefix = "_ttl_"

// Store 实现了MemoryStore接口，基于BigCache提供内存缓存功能
type Store struct {
	cache  *bigcache.BigCache
	logger log.Logger
	mutex  sync.RWMutex
	config *memoryconfig.Config
	closed bool            // 添加关闭状态标志
	keySet map[string]bool // 维护键集合以支持模式匹配
}

// New 创建一个新的BigCache内存存储实例
func New(config *memoryconfig.Config, logger log.Logger) storage.MemoryStore {
	// 解析配置的生命周期窗口
	lifeWindow, err := time.ParseDuration(config.GetLifeWindow())
	if err != nil {
		logger.Errorf("解析生命周期窗口失败: %v", err)
		lifeWindow = 10 * time.Minute // 默认值
	}

	// 解析清理窗口
	cleanWindow, err := time.ParseDuration(config.GetCleanWindow())
	if err != nil {
		logger.Errorf("解析清理窗口失败: %v", err)
		cleanWindow = 5 * time.Minute // 默认值
	}

	// 使用配置参数设置BigCache
	bigCacheConfig := bigcache.DefaultConfig(lifeWindow)
	bigCacheConfig.MaxEntriesInWindow = config.GetMaxEntriesInWindow()
	bigCacheConfig.MaxEntrySize = config.GetMaxEntrySize()
	bigCacheConfig.Shards = 1024 // 使用合理的默认分片数
	bigCacheConfig.CleanWindow = cleanWindow

	// 创建BigCache实例
	cache, err := bigcache.New(context.Background(), bigCacheConfig)
	if err != nil {
		logger.Errorf("创建BigCache实例失败: %v", err)
		return nil
	}

	return &Store{
		cache:  cache,
		logger: logger,
		config: config,
		keySet: make(map[string]bool),
	}
}

// Close 关闭缓存并释放资源
func (s *Store) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.closed {
		s.logger.Info("内存存储已关闭，跳过重复关闭")
		return nil
	}

	s.logger.Info("关闭内存存储")
	err := s.cache.Close()
	if err == nil {
		s.closed = true
	}
	return err
}

// Get 获取缓存值
func (s *Store) Get(ctx context.Context, key string) ([]byte, bool, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// 检查键是否过期
	if expired, err := s.isExpired(key); err != nil {
		if err == bigcache.ErrEntryNotFound {
			return nil, false, nil
		}
		return nil, false, err
	} else if expired {
		// 如果已过期，删除该键
		_ = s.cache.Delete(key)
		_ = s.cache.Delete(ttlPrefix + key)
		// 从键集合中移除（需要升级为写锁）
		s.mutex.RUnlock()
		s.mutex.Lock()
		delete(s.keySet, key)
		s.mutex.Unlock()
		s.mutex.RLock()
		return nil, false, nil
	}

	// 获取值
	value, err := s.cache.Get(key)
	if err != nil {
		if err == bigcache.ErrEntryNotFound {
			return nil, false, nil
		}
		s.logger.Warnf("获取缓存键[%s]失败: %v", key, err)
		return nil, false, err
	}

	return value, true, nil
}

// Set 设置缓存值，可指定过期时间
func (s *Store) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 设置键值对
	if err := s.cache.Set(key, value); err != nil {
		s.logger.Warnf("设置缓存键[%s]失败: %v", key, err)
		return err
	}

	// 添加到键集合
	s.keySet[key] = true

	// 如果指定了TTL，则设置过期时间
	if ttl > 0 {
		expirationTime := time.Now().Add(ttl).UnixNano()
		expirationBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(expirationBytes, uint64(expirationTime))

		if err := s.cache.Set(ttlPrefix+key, expirationBytes); err != nil {
			s.logger.Warnf("设置缓存键[%s]的TTL失败: %v", key, err)
			return err
		}
	} else {
		// 如果TTL为0（永不过期），删除可能存在的过期记录
		_ = s.cache.Delete(ttlPrefix + key)
	}

	return nil
}

// Delete 删除指定键的缓存
func (s *Store) Delete(ctx context.Context, key string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 删除键值对和对应的TTL记录
	if err := s.cache.Delete(key); err != nil && err != bigcache.ErrEntryNotFound {
		s.logger.Warnf("删除缓存键[%s]失败: %v", key, err)
		return err
	}

	// 从键集合中移除
	delete(s.keySet, key)

	// 删除TTL记录
	_ = s.cache.Delete(ttlPrefix + key)

	return nil
}

// Exists 检查键是否存在
func (s *Store) Exists(ctx context.Context, key string) (bool, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// 检查键是否过期
	if expired, err := s.isExpired(key); err != nil {
		if err == bigcache.ErrEntryNotFound {
			return false, nil
		}
		return false, err
	} else if expired {
		// 如果已过期，删除该键
		_ = s.cache.Delete(key)
		_ = s.cache.Delete(ttlPrefix + key)
		// 从键集合中移除（需要升级为写锁）
		s.mutex.RUnlock()
		s.mutex.Lock()
		delete(s.keySet, key)
		s.mutex.Unlock()
		s.mutex.RLock()
		return false, nil
	}

	// 检查键是否存在
	_, err := s.cache.Get(key)
	if err != nil {
		if err == bigcache.ErrEntryNotFound {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// GetMany 批量获取多个键的值
func (s *Store) GetMany(ctx context.Context, keys []string) (map[string][]byte, error) {
	s.mutex.Lock() // 使用写锁以便清理过期键
	defer s.mutex.Unlock()

	result := make(map[string][]byte)

	for _, key := range keys {
		// 检查键是否过期
		if expired, err := s.isExpired(key); err != nil {
			if err != bigcache.ErrEntryNotFound {
				s.logger.Warnf("检查缓存键[%s]过期状态失败: %v", key, err)
			}
			continue
		} else if expired {
			// 如果已过期，删除该键
			_ = s.cache.Delete(key)
			_ = s.cache.Delete(ttlPrefix + key)
			delete(s.keySet, key)
			continue
		}

		// 获取值
		value, err := s.cache.Get(key)
		if err != nil {
			if err != bigcache.ErrEntryNotFound {
				s.logger.Warnf("批量获取缓存键[%s]失败: %v", key, err)
			}
			continue
		}

		result[key] = value
	}

	return result, nil
}

// SetMany 批量设置多个键值对，使用相同的TTL
func (s *Store) SetMany(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for key, value := range items {
		// 设置键值对
		if err := s.cache.Set(key, value); err != nil {
			s.logger.Warnf("批量设置缓存键[%s]失败: %v", key, err)
			return err
		}

		// 添加到键集合
		s.keySet[key] = true

		// 如果指定了TTL，则设置过期时间
		if ttl > 0 {
			expirationTime := time.Now().Add(ttl).UnixNano()
			expirationBytes := make([]byte, 8)
			binary.LittleEndian.PutUint64(expirationBytes, uint64(expirationTime))

			if err := s.cache.Set(ttlPrefix+key, expirationBytes); err != nil {
				s.logger.Warnf("批量设置缓存键[%s]的TTL失败: %v", key, err)
				return err
			}
		} else {
			// 如果TTL为0（永不过期），删除可能存在的过期记录
			_ = s.cache.Delete(ttlPrefix + key)
		}
	}

	return nil
}

// DeleteMany 批量删除多个键
func (s *Store) DeleteMany(ctx context.Context, keys []string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for _, key := range keys {
		// 删除键值对和对应的TTL记录
		if err := s.cache.Delete(key); err != nil && err != bigcache.ErrEntryNotFound {
			s.logger.Warnf("批量删除缓存键[%s]失败: %v", key, err)
			// 继续处理其他键，不返回错误
		}

		// 从键集合中移除
		delete(s.keySet, key)

		// 删除TTL记录
		_ = s.cache.Delete(ttlPrefix + key)
	}

	return nil
}

// Clear 清空所有缓存
func (s *Store) Clear(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 重置缓存
	if err := s.cache.Reset(); err != nil {
		s.logger.Errorf("清空缓存失败: %v", err)
		return err
	}

	// 清空键集合
	s.keySet = make(map[string]bool)

	return nil
}

// GetTTL 获取键的剩余生存时间
func (s *Store) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// 首先检查键是否存在
	_, err := s.cache.Get(key)
	if err != nil {
		if err == bigcache.ErrEntryNotFound {
			return 0, fmt.Errorf("键不存在")
		}
		return 0, err
	}

	// 获取TTL信息
	ttlBytes, err := s.cache.Get(ttlPrefix + key)
	if err != nil {
		if err == bigcache.ErrEntryNotFound {
			// 键存在但没有TTL设置，表示永不过期
			return 0, nil
		}
		return 0, err
	}

	// 解析过期时间
	expirationTime := int64(binary.LittleEndian.Uint64(ttlBytes))
	remaining := time.Duration(expirationTime - time.Now().UnixNano())

	if remaining <= 0 {
		// 键已过期，删除它
		_ = s.cache.Delete(key)
		_ = s.cache.Delete(ttlPrefix + key)
		return 0, fmt.Errorf("键已过期")
	}

	return remaining, nil
}

// UpdateTTL 更新键的过期时间
func (s *Store) UpdateTTL(ctx context.Context, key string, ttl time.Duration) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 首先检查键是否存在
	_, err := s.cache.Get(key)
	if err != nil {
		if err == bigcache.ErrEntryNotFound {
			return fmt.Errorf("键不存在")
		}
		return err
	}

	// 设置新的TTL
	if ttl > 0 {
		expirationTime := time.Now().Add(ttl).UnixNano()
		expirationBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(expirationBytes, uint64(expirationTime))

		if err := s.cache.Set(ttlPrefix+key, expirationBytes); err != nil {
			s.logger.Warnf("更新缓存键[%s]的TTL失败: %v", key, err)
			return err
		}
	} else {
		// 如果TTL为0（永不过期），删除过期记录
		_ = s.cache.Delete(ttlPrefix + key)
	}

	return nil
}

// Count 获取当前缓存中的键数量
func (s *Store) Count(ctx context.Context) (int64, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// 获取所有缓存条目数
	total := s.cache.Len()

	// 如果没有数据，直接返回0
	if total == 0 {
		return 0, nil
	}

	// 估算实际有效键的数量（不包括TTL记录）
	// 注意：这是一个估计值，因为我们无法精确知道有多少键有TTL记录
	actual := total / 2

	return int64(actual), nil
}

// isExpired 检查键是否已过期
func (s *Store) isExpired(key string) (bool, error) {
	// 获取TTL信息
	ttlBytes, err := s.cache.Get(ttlPrefix + key)
	if err != nil {
		if err == bigcache.ErrEntryNotFound {
			// 没有TTL记录，表示永不过期
			return false, nil
		}
		return false, err
	}

	// 检查键是否存在
	_, err = s.cache.Get(key)
	if err != nil {
		return false, err
	}

	// 解析过期时间
	expirationTime := int64(binary.LittleEndian.Uint64(ttlBytes))

	// 如果当前时间超过过期时间，则键已过期
	return time.Now().UnixNano() > expirationTime, nil
}

// DeleteByPattern 根据模式删除缓存
func (s *Store) DeleteByPattern(ctx context.Context, pattern string) (int64, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 获取所有匹配且有效的键（此调用会清理过期的键）
	keys, err := s.getKeysInternal(pattern)
	if err != nil {
		return 0, err
	}

	// 删除匹配的键
	deletedCount := int64(0)
	for _, key := range keys {
		// 删除键值对和对应的TTL记录
		if err := s.cache.Delete(key); err != nil && err != bigcache.ErrEntryNotFound {
			s.logger.Warnf("删除匹配模式[%s]的缓存键[%s]失败: %v", pattern, key, err)
			continue
		}

		// 从键集合中移除
		delete(s.keySet, key)

		// 删除TTL记录
		_ = s.cache.Delete(ttlPrefix + key)
		deletedCount++
	}

	return deletedCount, nil
}

// GetKeys 获取匹配模式的所有键
func (s *Store) GetKeys(ctx context.Context, pattern string) ([]string, error) {
	s.mutex.Lock() // 改为写锁以便清理过期键
	defer s.mutex.Unlock()

	return s.getKeysInternal(pattern)
}

// getKeysInternal 内部方法，获取匹配模式的键（不加锁）
func (s *Store) getKeysInternal(pattern string) ([]string, error) {
	var matchedKeys []string
	var expiredKeys []string

	// 如果没有模式，返回所有键
	if pattern == "" || pattern == "*" {
		for key := range s.keySet {
			// 检查键是否过期，如果过期则收集起来清理
			if expired, _ := s.isExpired(key); expired {
				expiredKeys = append(expiredKeys, key)
			} else {
				matchedKeys = append(matchedKeys, key)
			}
		}
	} else {
		// 实现通配符匹配
		for key := range s.keySet {
			// 检查键是否过期，如果过期则收集起来清理
			if expired, _ := s.isExpired(key); expired {
				expiredKeys = append(expiredKeys, key)
				continue
			}

			matched, err := filepath.Match(pattern, key)
			if err != nil {
				// 如果模式无效，尝试简单的字符串匹配
				if strings.Contains(key, strings.ReplaceAll(pattern, "*", "")) {
					matchedKeys = append(matchedKeys, key)
				}
			} else if matched {
				matchedKeys = append(matchedKeys, key)
			}
		}
	}

	// 清理过期的键
	for _, expiredKey := range expiredKeys {
		_ = s.cache.Delete(expiredKey)
		_ = s.cache.Delete(ttlPrefix + expiredKey)
		delete(s.keySet, expiredKey)
	}

	return matchedKeys, nil
}
