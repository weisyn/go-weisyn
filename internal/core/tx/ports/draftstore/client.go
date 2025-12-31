package draftstore

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// goRedisClient go-redis 客户端实现
//
// 🎯 **职责**：实现 redisClient 接口，封装 go-redis 客户端
//
// 📋 **实现说明**：
//   - 使用 github.com/redis/go-redis/v9 作为底层客户端
//   - 提供完整的 Redis 操作接口实现
//   - 支持连接池、超时等配置
//
// 🔒 **并发安全**：
//   - go-redis 客户端本身是并发安全的
//   - 可以安全地在多个 goroutine 中使用
type goRedisClient struct {
	client *redis.Client
}

// 确保实现接口
var _ redisClient = (*goRedisClient)(nil)

// newGoRedisClient 创建 go-redis 客户端实现
//
// 参数：
//   - cfg: Redis 配置
//
// 返回值：
//   - redisClient: Redis 客户端接口实现
//   - error: 创建失败的原因
func newGoRedisClient(cfg *Config) (redisClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("redis config cannot be nil")
	}

	if cfg.Addr == "" {
		return nil, fmt.Errorf("redis address cannot be empty")
	}

	// 构建 go-redis 选项
	opts := &redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
	}

	// 设置超时（如果配置了）
	if cfg.DialTimeout > 0 {
		opts.DialTimeout = time.Duration(cfg.DialTimeout) * time.Second
	}
	if cfg.ReadTimeout > 0 {
		opts.ReadTimeout = time.Duration(cfg.ReadTimeout) * time.Second
	}
	if cfg.WriteTimeout > 0 {
		opts.WriteTimeout = time.Duration(cfg.WriteTimeout) * time.Second
	}

	// 创建 go-redis 客户端
	client := redis.NewClient(opts)

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &goRedisClient{
		client: client,
	}, nil
}

// Set 设置键值对
func (c *goRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return c.client.Set(ctx, key, value, expiration).Err()
}

// Get 获取键对应的值
func (c *goRedisClient) Get(ctx context.Context, key string) ([]byte, error) {
	result := c.client.Get(ctx, key)
	if err := result.Err(); err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("key not found: %s", key)
		}
		return nil, err
	}
	return []byte(result.Val()), nil
}

// Del 删除键
func (c *goRedisClient) Del(ctx context.Context, keys ...string) (int64, error) {
	if len(keys) == 0 {
		return 0, nil
	}
	return c.client.Del(ctx, keys...).Result()
}

// Keys 查找匹配模式的所有键
func (c *goRedisClient) Keys(ctx context.Context, pattern string) ([]string, error) {
	return c.client.Keys(ctx, pattern).Result()
}

// Exists 检查键是否存在
func (c *goRedisClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	if len(keys) == 0 {
		return 0, nil
	}
	return c.client.Exists(ctx, keys...).Result()
}

// TTL 获取键的剩余生存时间
func (c *goRedisClient) TTL(ctx context.Context, key string) (time.Duration, error) {
	return c.client.TTL(ctx, key).Result()
}

// Expire 设置键的过期时间
func (c *goRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	return c.client.Expire(ctx, key, expiration).Result()
}

// Ping 测试连接
func (c *goRedisClient) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Close 关闭连接
func (c *goRedisClient) Close() error {
	return c.client.Close()
}

