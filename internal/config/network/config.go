package network

import "time"

// NetworkOptions 网络配置选项
// 只包含实际在网络实现中使用的核心配置参数
type NetworkOptions struct {
	// === 消息传输配置 ===
	MaxMessageSize int64         `json:"max_message_size"` // 最大消息大小(字节)
	MessageTimeout time.Duration `json:"message_timeout"`  // 消息超时时间

	// === 去重缓存配置 ===
	DeduplicationCacheTTL time.Duration `json:"deduplication_cache_ttl"` // 去重缓存TTL

	// === 重试配置 ===
	RetryAttempts    int           `json:"retry_attempts"`     // 重试次数
	RetryBackoffBase time.Duration `json:"retry_backoff_base"` // 重试退避基数
	RetryBackoffMax  time.Duration `json:"retry_backoff_max"`  // 最大退避时间

	// === 流传输配置 ===
	ConnectTimeout time.Duration `json:"connect_timeout"` // 连接超时
	WriteTimeout   time.Duration `json:"write_timeout"`   // 写入超时
	ReadTimeout    time.Duration `json:"read_timeout"`    // 读取超时
}

// Config 网络配置实现
type Config struct {
	options *NetworkOptions
}

// New 创建网络配置实现
func New(userConfig interface{}) *Config {
	// 1. 先创建完整的默认配置
	defaultOptions := createDefaultNetworkOptions()

	// 2. 暂时不处理用户配置，后续添加
	// TODO: 当有用户配置类型时，在这里进行转换和合并

	return &Config{
		options: defaultOptions,
	}
}

// createDefaultNetworkOptions 创建默认网络配置
func createDefaultNetworkOptions() *NetworkOptions {
	return &NetworkOptions{
		// 消息传输配置
		MaxMessageSize: defaultMaxMessageSize,
		MessageTimeout: defaultMessageTimeout,

		// 去重缓存配置
		DeduplicationCacheTTL: defaultDeduplicationCacheTTL,

		// 重试和容错配置
		RetryAttempts:    defaultRetryAttempts,
		RetryBackoffBase: defaultRetryBackoffBase,
		RetryBackoffMax:  defaultRetryBackoffMax,

		// 流传输配置
		ConnectTimeout: defaultConnectTimeout,
		WriteTimeout:   defaultWriteTimeout,
		ReadTimeout:    defaultReadTimeout,
	}
}

// GetOptions 获取完整的网络配置选项
func (c *Config) GetOptions() *NetworkOptions {
	return c.options
}

// === 消息传输配置访问方法 ===

// GetMaxMessageSize 获取最大消息大小
func (c *Config) GetMaxMessageSize() int64 {
	return c.options.MaxMessageSize
}

// GetMessageTimeout 获取消息超时时间
func (c *Config) GetMessageTimeout() time.Duration {
	return c.options.MessageTimeout
}

// === 去重缓存配置访问方法 ===

// GetDeduplicationCacheTTL 获取去重缓存TTL
func (c *Config) GetDeduplicationCacheTTL() time.Duration {
	return c.options.DeduplicationCacheTTL
}

// === 重试和容错配置访问方法 ===

// GetRetryAttempts 获取重试次数
func (c *Config) GetRetryAttempts() int {
	return c.options.RetryAttempts
}

// GetRetryBackoffBase 获取重试退避基数
func (c *Config) GetRetryBackoffBase() time.Duration {
	return c.options.RetryBackoffBase
}

// GetRetryBackoffMax 获取最大退避时间
func (c *Config) GetRetryBackoffMax() time.Duration {
	return c.options.RetryBackoffMax
}

// === 流传输配置访问方法 ===

// GetConnectTimeout 获取连接超时
func (c *Config) GetConnectTimeout() time.Duration {
	return c.options.ConnectTimeout
}

// GetWriteTimeout 获取写入超时
func (c *Config) GetWriteTimeout() time.Duration {
	return c.options.WriteTimeout
}

// GetReadTimeout 获取读取超时
func (c *Config) GetReadTimeout() time.Duration {
	return c.options.ReadTimeout
}
