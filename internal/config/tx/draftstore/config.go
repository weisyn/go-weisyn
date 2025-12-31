package draftstore

// DraftStoreOptions 草稿存储配置选项
//
// 🎯 **配置职责**：管理交易草稿存储相关的所有配置
//
// 📋 **存储后端**：
// - memory: 内存存储（默认，适用于单节点）
// - redis: Redis存储（适用于分布式场景）
type DraftStoreOptions struct {
	// 存储类型（memory, redis）
	Type string `json:"type"`

	// 内存存储配置
	Memory MemoryDraftStoreConfig `json:"memory"`

	// Redis存储配置
	Redis RedisDraftStoreConfig `json:"redis"`
}

// MemoryDraftStoreConfig 内存草稿存储配置
type MemoryDraftStoreConfig struct {
	// 最大草稿数
	MaxDrafts int `json:"max_drafts"`

	// 清理间隔（秒，0表示不自动清理）
	CleanupIntervalSeconds int `json:"cleanup_interval_seconds"`
}

// RedisDraftStoreConfig Redis草稿存储配置
type RedisDraftStoreConfig struct {
	// Redis服务器地址（格式：host:port）
	Addr string `json:"addr"`

	// Redis密码（可选）
	Password string `json:"password"`

	// Redis数据库编号（0-15）
	DB int `json:"db"`

	// Key前缀（用于命名空间隔离）
	KeyPrefix string `json:"key_prefix"`

	// 默认TTL（秒，0表示永不过期）
	DefaultTTL int `json:"default_ttl"`

	// 连接池大小
	PoolSize int `json:"pool_size"`

	// 最小空闲连接数
	MinIdleConns int `json:"min_idle_conns"`

	// 连接超时（秒）
	DialTimeout int `json:"dial_timeout"`

	// 读超时（秒）
	ReadTimeout int `json:"read_timeout"`

	// 写超时（秒）
	WriteTimeout int `json:"write_timeout"`
}

// UserDraftStoreConfig 用户草稿存储配置（从configs/*/config.json加载）
//
// 📋 **配置来源**：用户配置文件（可选，通常不暴露给用户）
type UserDraftStoreConfig struct {
	// 存储类型（memory, redis）
	Type string `json:"type"`

	// 内存存储配置
	Memory *MemoryDraftStoreConfig `json:"memory,omitempty"`

	// Redis存储配置
	Redis *RedisDraftStoreConfig `json:"redis,omitempty"`
}

// New 创建草稿存储配置选项
//
// 参数：
//   - userConfig: 用户配置（从configs/*/config.json加载，可为nil）
//
// 返回：
//   - *DraftStoreOptions: 草稿存储配置选项
func New(userConfig *UserDraftStoreConfig) *DraftStoreOptions {
	opts := &DraftStoreOptions{
		Type:   getDefaultStoreType(),
		Memory: getDefaultMemoryConfig(),
		Redis:  getDefaultRedisConfig(),
	}

	// 应用用户配置
	if userConfig != nil {
		applyUserConfig(opts, userConfig)
	}

	return opts
}

// applyUserConfig 应用用户配置
func applyUserConfig(opts *DraftStoreOptions, userConfig *UserDraftStoreConfig) {
	// 应用存储类型
	if userConfig.Type != "" {
		opts.Type = userConfig.Type
	}

	// 应用内存存储配置
	if userConfig.Memory != nil {
		if userConfig.Memory.MaxDrafts > 0 {
			opts.Memory.MaxDrafts = userConfig.Memory.MaxDrafts
		}
		if userConfig.Memory.CleanupIntervalSeconds > 0 {
			opts.Memory.CleanupIntervalSeconds = userConfig.Memory.CleanupIntervalSeconds
		}
	}

	// 应用Redis存储配置
	if userConfig.Redis != nil {
		if userConfig.Redis.Addr != "" {
			opts.Redis.Addr = userConfig.Redis.Addr
		}
		if userConfig.Redis.Password != "" {
			opts.Redis.Password = userConfig.Redis.Password
		}
		if userConfig.Redis.DB >= 0 {
			opts.Redis.DB = userConfig.Redis.DB
		}
		if userConfig.Redis.KeyPrefix != "" {
			opts.Redis.KeyPrefix = userConfig.Redis.KeyPrefix
		}
		if userConfig.Redis.DefaultTTL >= 0 {
			opts.Redis.DefaultTTL = userConfig.Redis.DefaultTTL
		}
		if userConfig.Redis.PoolSize > 0 {
			opts.Redis.PoolSize = userConfig.Redis.PoolSize
		}
		if userConfig.Redis.MinIdleConns > 0 {
			opts.Redis.MinIdleConns = userConfig.Redis.MinIdleConns
		}
		if userConfig.Redis.DialTimeout > 0 {
			opts.Redis.DialTimeout = userConfig.Redis.DialTimeout
		}
		if userConfig.Redis.ReadTimeout > 0 {
			opts.Redis.ReadTimeout = userConfig.Redis.ReadTimeout
		}
		if userConfig.Redis.WriteTimeout > 0 {
			opts.Redis.WriteTimeout = userConfig.Redis.WriteTimeout
		}
	}
}

// GetMemoryConfig 获取内存存储配置
func (o *DraftStoreOptions) GetMemoryConfig() *MemoryDraftStoreConfig {
	return &o.Memory
}

// GetRedisConfig 获取Redis存储配置
func (o *DraftStoreOptions) GetRedisConfig() *RedisDraftStoreConfig {
	return &o.Redis
}

