package draftstore

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/tx"
	"github.com/weisyn/v1/pkg/types"
)

// redisClient Redis 客户端接口（用于依赖注入和测试）
//
// 🎯 **设计理念**：
// 定义最小化的 Redis 操作接口，支持多种 Redis 客户端实现。
// 生产环境可以使用 go-redis，测试环境可以使用 mock。
//
// ⚠️ **可见性**：此接口为包内私有接口，仅用于实现细节，不对外暴露。
type redisClient interface {
	// Set 设置键值对
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	// Get 获取键对应的值
	Get(ctx context.Context, key string) ([]byte, error)
	// Del 删除键
	Del(ctx context.Context, keys ...string) (int64, error)
	// Keys 查找匹配模式的所有键
	Keys(ctx context.Context, pattern string) ([]string, error)
	// Exists 检查键是否存在
	Exists(ctx context.Context, keys ...string) (int64, error)
	// TTL 获取键的剩余生存时间
	TTL(ctx context.Context, key string) (time.Duration, error)
	// Expire 设置键的过期时间
	Expire(ctx context.Context, key string, expiration time.Duration) (bool, error)
	// Ping 测试连接
	Ping(ctx context.Context) error
	// Close 关闭连接
	Close() error
}

// RedisStore Redis 版本的 DraftStore 实现
//
// 📋 **职责**：
//   - 在 Redis 中存储和检索交易草稿
//   - 提供分布式、持久化的草稿存储
//   - 支持 TTL 自动过期清理
//
// 🔒 **并发安全**：
//   - Redis 本身提供原子性操作
//   - 支持多个进程/节点并发访问
//
// 📚 **使用场景**：
//   - Off-chain 场景：CLI/API 跨会话草稿保存
//   - 分布式场景：多节点共享草稿状态
//   - 长期存储：支持持久化和恢复
//
// ⚠️ **核心优势**：
//   - 持久化：进程重启后数据不丢失
//   - 分布式：支持跨进程/跨节点共享
//   - TTL 支持：自动清理过期草稿
//   - 高性能：基于内存的快速读写
//
// 🎯 **设计理念**：
//   - Key 格式：draft:{draftID}
//   - Value 格式：JSON 序列化的 DraftTx
//   - TTL：使用 Redis EXPIRE 实现自动过期
type RedisStore struct {
	// Redis 客户端（使用接口以支持依赖注入和测试）
	client redisClient
	// Key 前缀（用于命名空间隔离）
	keyPrefix string
	// 默认 TTL（秒）
	defaultTTL time.Duration
}

// 确保实现接口
var _ tx.DraftStore = (*RedisStore)(nil)

// Config Redis DraftStore 配置
type Config struct {
	// Redis 服务器地址（如 "localhost:28791"）
	Addr string
	// Redis 密码（可选）
	Password string
	// Redis 数据库编号（0-15）
	DB int
	// Key 前缀（用于命名空间隔离）
	KeyPrefix string
	// 默认 TTL（秒，0 表示永不过期）
	DefaultTTL int
	// 连接池大小
	PoolSize int
	// 最小空闲连接数
	MinIdleConns int
	// 连接超时（秒）
	DialTimeout int
	// 读超时（秒）
	ReadTimeout int
	// 写超时（秒）
	WriteTimeout int
}

// DefaultConfig 返回默认配置
//
// ⚠️ **已废弃**：此函数保留仅为向后兼容，生产代码应使用配置系统。
// 请使用 internal/config/tx/draftstore 配置模块提供的配置。
//
// 🔧 **修复说明**：硬编码的Redis地址已移除，请通过配置系统管理。
func DefaultConfig() *Config {
	// 🔧 修复：移除硬编码，返回空配置，强制使用配置系统
	// 如果调用方需要默认值，应从配置模块获取
	return &Config{
		Addr:         "", // 必须通过配置提供
		Password:     "",
		DB:           0,
		KeyPrefix:    "weisyn:draft:",
		DefaultTTL:   3600,
		PoolSize:     10,
		MinIdleConns: 5,
		DialTimeout:  5,
		ReadTimeout:  3,
		WriteTimeout: 3,
	}
}

// NewRedisStoreFromConfig 从配置创建 Redis 版 DraftStore 实例
//
// 🎯 **使用场景**：从配置系统创建 Redis DraftStore
//
// 参数：
//   - cfg: Redis 配置
//
// 返回值：
//   - tx.DraftStore: 服务实例
//   - error: 创建失败的原因
func NewRedisStoreFromConfig(cfg *Config) (tx.DraftStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("redis config cannot be nil")
	}

	// 创建 go-redis 客户端
	client, err := newGoRedisClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Redis client: %w", err)
	}

	// 使用配置中的 keyPrefix 和 defaultTTL
	keyPrefix := cfg.KeyPrefix
	if keyPrefix == "" {
		keyPrefix = "weisyn:draft:"
	}

	defaultTTL := cfg.DefaultTTL
	if defaultTTL <= 0 {
		defaultTTL = 3600 // 默认1小时
	}

	return NewRedisStore(client, keyPrefix, defaultTTL)
}

// NewRedisStore 创建 Redis 版 DraftStore 实例
//
// 参数：
//   - client: Redis 客户端（需实现 redisClient 接口）
//   - keyPrefix: Key 前缀
//   - defaultTTL: 默认 TTL（秒）
//
// 返回值：
//   - tx.DraftStore: 服务实例
//   - error: 创建失败的原因
func NewRedisStore(client redisClient, keyPrefix string, defaultTTL int) (tx.DraftStore, error) {
	if client == nil {
		return nil, fmt.Errorf("redis client cannot be nil")
	}

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisStore{
		client:     client,
		keyPrefix:  keyPrefix,
		defaultTTL: time.Duration(defaultTTL) * time.Second,
	}, nil
}

// Save 保存交易草稿到 Redis
//
// 实现 tx.DraftStore 接口
//
// 参数：
//   - ctx: 上下文对象
//   - draft: 待保存的交易草稿
//
// 返回值：
//   - string: 草稿唯一 ID
//   - error: 保存失败的原因
func (s *RedisStore) Save(ctx context.Context, draft *types.DraftTx) (string, error) {
	if draft == nil {
		return "", fmt.Errorf("draft cannot be nil")
	}

	draftID := draft.DraftID
	if draftID == "" {
		return "", fmt.Errorf("draft ID cannot be empty")
	}

	// 序列化草稿为 JSON
	data, err := json.Marshal(draft)
	if err != nil {
		return "", fmt.Errorf("failed to marshal draft: %w", err)
	}

	// 构建 Redis key
	key := s.buildKey(draftID)

	// 保存到 Redis（使用默认 TTL）
	err = s.client.Set(ctx, key, data, s.defaultTTL)
	if err != nil {
		return "", fmt.Errorf("failed to save draft to Redis: %w", err)
	}

	return draftID, nil
}

// Get 从 Redis 检索交易草稿
//
// 实现 tx.DraftStore 接口
//
// 参数：
//   - ctx: 上下文对象
//   - draftID: 草稿唯一标识
//
// 返回值：
//   - *types.DraftTx: 草稿对象
//   - error: 检索失败的原因（如草稿不存在）
func (s *RedisStore) Get(ctx context.Context, draftID string) (*types.DraftTx, error) {
	if draftID == "" {
		return nil, fmt.Errorf("draft ID cannot be empty")
	}

	// 构建 Redis key
	key := s.buildKey(draftID)

	// 从 Redis 读取
	data, err := s.client.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("draft not found: %s (error: %w)", draftID, err)
	}

	// 反序列化
	var draft types.DraftTx
	if err := json.Unmarshal(data, &draft); err != nil {
		return nil, fmt.Errorf("failed to unmarshal draft: %w", err)
	}

	return &draft, nil
}

// Delete 从 Redis 删除交易草稿
//
// 实现 tx.DraftStore 接口
//
// 参数：
//   - ctx: 上下文对象
//   - draftID: 草稿唯一标识
//
// 返回值：
//   - error: 删除失败的原因
func (s *RedisStore) Delete(ctx context.Context, draftID string) error {
	if draftID == "" {
		return fmt.Errorf("draft ID cannot be empty")
	}

	// 构建 Redis key
	key := s.buildKey(draftID)

	// 从 Redis 删除
	result, err := s.client.Del(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to delete draft from Redis: %w", err)
	}

	if result == 0 {
		// 草稿不存在，返回错误
		return fmt.Errorf("draft not found: %s", draftID)
	}

	return nil
}

// List 列出交易草稿（分页）
//
// 实现 tx.DraftStore 接口
//
// 参数：
//   - ctx: 上下文对象
//   - owner: 所有者地址（用于过滤，Redis实现暂不支持）
//   - offset: 偏移量
//   - limit: 限制数量
//
// 返回值：
//   - []*types.DraftTx: 草稿列表
//   - error: 列出失败的原因
func (s *RedisStore) List(ctx context.Context, owner []byte, limit, offset int) ([]*types.DraftTx, error) {
	// 使用 KEYS 命令查找所有匹配的 key
	// 注意：生产环境应使用 SCAN 而非 KEYS，避免阻塞
	pattern := s.keyPrefix + "*"
	keys, err := s.client.Keys(ctx, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to list drafts from Redis: %w", err)
	}

	// 应用分页
	start := offset
	end := offset + limit
	if start > len(keys) {
		return []*types.DraftTx{}, nil
	}
	if end > len(keys) {
		end = len(keys)
	}

	// 批量获取草稿
	drafts := make([]*types.DraftTx, 0, end-start)
	for i := start; i < end; i++ {
		// 提取 draftID
		key := keys[i]
		if len(key) <= len(s.keyPrefix) {
			continue
		}
		draftID := key[len(s.keyPrefix):]

		// 获取草稿
		draft, err := s.Get(ctx, draftID)
		if err != nil {
			// 跳过获取失败的草稿
			continue
		}

		drafts = append(drafts, draft)
	}

	return drafts, nil
}

// Exists 检查交易草稿是否存在
//
// 扩展方法（非 DraftStore 接口定义）
//
// 参数：
//   - ctx: 上下文对象
//   - draftID: 草稿唯一标识
//
// 返回值：
//   - bool: true 表示存在，false 表示不存在
//   - error: 检查失败的原因
func (s *RedisStore) Exists(ctx context.Context, draftID string) (bool, error) {
	if draftID == "" {
		return false, fmt.Errorf("draft ID cannot be empty")
	}

	// 构建 Redis key
	key := s.buildKey(draftID)

	// 使用 EXISTS 命令
	result, err := s.client.Exists(ctx, key)
	if err != nil {
		return false, fmt.Errorf("failed to check draft existence in Redis: %w", err)
	}

	return result > 0, nil
}

// GetTTL 获取草稿的剩余 TTL
//
// 扩展方法（非 DraftStore 接口定义）
//
// 参数：
//   - ctx: 上下文对象
//   - draftID: 草稿唯一标识
//
// 返回值：
//   - time.Duration: 剩余 TTL（-1 表示永不过期，-2 表示不存在）
//   - error: 获取失败的原因
func (s *RedisStore) GetTTL(ctx context.Context, draftID string) (time.Duration, error) {
	if draftID == "" {
		return 0, fmt.Errorf("draft ID cannot be empty")
	}

	// 构建 Redis key
	key := s.buildKey(draftID)

	// 使用 TTL 命令
	ttl, err := s.client.TTL(ctx, key)
	if err != nil {
		return 0, fmt.Errorf("failed to get TTL from Redis: %w", err)
	}

	return ttl, nil
}

// SetTTL 设置草稿的 TTL
//
// 实现 tx.DraftStore 接口
//
// 参数：
//   - ctx: 上下文对象
//   - draftID: 草稿唯一标识
//   - ttlSeconds: TTL（秒）
//
// 返回值：
//   - error: 设置失败的原因
func (s *RedisStore) SetTTL(ctx context.Context, draftID string, ttlSeconds int) error {
	if draftID == "" {
		return fmt.Errorf("draft ID cannot be empty")
	}

	// 构建 Redis key
	key := s.buildKey(draftID)

	// 使用 EXPIRE 命令
	ok, err := s.client.Expire(ctx, key, time.Duration(ttlSeconds)*time.Second)
	if err != nil {
		return fmt.Errorf("failed to set TTL in Redis: %w", err)
	}

	if !ok {
		return fmt.Errorf("draft not found: %s", draftID)
	}

	return nil
}

// Close 关闭 Redis 连接
//
// 扩展方法（非 DraftStore 接口定义）
//
// 返回值：
//   - error: 关闭失败的原因
func (s *RedisStore) Close() error {
	return s.client.Close()
}

// buildKey 构建 Redis key
//
// 参数：
//   - draftID: 草稿唯一标识
//
// 返回值：
//   - string: Redis key（格式：keyPrefix + draftID）
func (s *RedisStore) buildKey(draftID string) string {
	return s.keyPrefix + draftID
}

// Ping 测试 Redis 连接
//
// 扩展方法（非 DraftStore 接口定义）
//
// 参数：
//   - ctx: 上下文对象
//
// 返回值：
//   - error: 测试失败的原因
func (s *RedisStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx)
}
