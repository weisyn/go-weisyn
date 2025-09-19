package hash

import (
	"crypto/sha256"
	"crypto/subtle"
	"sync"

	cryptointf "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"golang.org/x/crypto/ripemd160"
	"golang.org/x/crypto/sha3"
)

// 确保HashService实现了cryptointf.HashManager接口
var _ cryptointf.HashManager = (*HashService)(nil)

// HashCache 简单的哈希缓存结构
type HashCache struct {
	cache map[string][]byte
	mu    sync.RWMutex
}

// NewHashCache 创建新的哈希缓存
func NewHashCache() *HashCache {
	return &HashCache{
		cache: make(map[string][]byte),
	}
}

// Get 从缓存获取哈希值
func (c *HashCache) Get(key string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	value, ok := c.cache[key]
	if ok {
		result := make([]byte, len(value))
		copy(result, value) // 返回副本而非引用
		return result, true
	}
	return nil, false
}

// Set 设置缓存中的哈希值
func (c *HashCache) Set(key string, value []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	valueCopy := make([]byte, len(value))
	copy(valueCopy, value) // 存储副本而非引用
	c.cache[key] = valueCopy
}

// HashService 提供哈希计算功能
type HashService struct {
	// 缓存最近的哈希结果，避免重复计算
	sha256Cache       *HashCache
	keccak256Cache    *HashCache
	doubleSHA256Cache *HashCache
	ripemd160Cache    *HashCache // 新增RIPEMD160缓存
}

// NewHashService 创建新的哈希服务
//
// 返回一个包含优化缓存的哈希服务实例
func NewHashService() *HashService {
	return &HashService{
		sha256Cache:       NewHashCache(),
		keccak256Cache:    NewHashCache(),
		doubleSHA256Cache: NewHashCache(),
		ripemd160Cache:    NewHashCache(), // 初始化RIPEMD160缓存
	}
}

// cacheKey 根据数据生成缓存键
// 🔥 修复：使用SHA256哈希作为缓存键，确保唯一性
func cacheKey(data []byte) string {
	// 对于任何大小的数据，都使用其SHA256哈希作为缓存键
	// 这确保了缓存键的唯一性，避免因数据截断导致的哈希冲突
	hasher := sha256.New()
	hasher.Write(data)
	keyHash := hasher.Sum(nil)
	return string(keyHash)
}

// SHA256 计算SHA-256哈希
//
// 参数:
//   - data: 要计算哈希的数据
//
// 返回:
//   - []byte: 32字节的SHA-256哈希结果
func (s *HashService) SHA256(data []byte) []byte {
	// 检查缓存
	key := cacheKey(data)
	if cachedHash, ok := s.sha256Cache.Get(key); ok {
		return cachedHash
	}

	// 计算哈希
	hash := sha256.Sum256(data)
	result := hash[:]

	// 存入缓存
	s.sha256Cache.Set(key, result)
	return result
}

// Keccak256 计算Keccak-256哈希
//
// 参数:
//   - data: 要计算哈希的数据
//
// 返回:
//   - []byte: 32字节的Keccak-256哈希结果
func (s *HashService) Keccak256(data []byte) []byte {
	// 检查缓存
	key := cacheKey(data)
	if cachedHash, ok := s.keccak256Cache.Get(key); ok {
		return cachedHash
	}

	// 计算哈希
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(data)
	result := hasher.Sum(nil)

	// 存入缓存
	s.keccak256Cache.Set(key, result)
	return result
}

// RIPEMD160 计算RIPEMD-160哈希
//
// 参数:
//   - data: 要计算哈希的数据
//
// 返回:
//   - []byte: 20字节的RIPEMD-160哈希结果
func (s *HashService) RIPEMD160(data []byte) []byte {
	// 检查缓存
	key := cacheKey(data)
	if cachedHash, ok := s.ripemd160Cache.Get(key); ok {
		return cachedHash
	}

	// 计算哈希
	hasher := ripemd160.New()
	hasher.Write(data)
	result := hasher.Sum(nil)

	// 存入缓存
	s.ripemd160Cache.Set(key, result)
	return result
}

// DoubleSHA256 计算双重SHA-256哈希
//
// 参数:
//   - data: 要计算哈希的数据
//
// 返回:
//   - []byte: 32字节的双重SHA-256哈希结果
func (s *HashService) DoubleSHA256(data []byte) []byte {
	// 检查缓存
	key := cacheKey(data)
	if cachedHash, ok := s.doubleSHA256Cache.Get(key); ok {
		return cachedHash
	}

	// 计算双重哈希
	first := sha256.Sum256(data)
	second := sha256.Sum256(first[:])
	result := second[:]

	// 存入缓存
	s.doubleSHA256Cache.Set(key, result)
	return result
}

// ConstantTimeCompare 在常量时间内比较两个哈希值是否相等
// 用于防止时序攻击，无论何时都会比较整个字节数组
//
// 参数:
//   - a: 第一个哈希值
//   - b: 第二个哈希值
//
// 返回:
//   - bool: 如果两个哈希值相等返回true，否则返回false
func ConstantTimeCompare(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}
