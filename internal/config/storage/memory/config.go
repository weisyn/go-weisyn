package memory

import "time"

// MemoryOptions 内存存储配置选项
// 专注于基础设施核心功能的简化配置
type MemoryOptions struct {
	// === 基础配置 ===
	MaxMemory  int64         `json:"max_memory"`  // 最大内存使用量
	MaxEntries int           `json:"max_entries"` // 最大条目数
	DefaultTTL time.Duration `json:"default_ttl"` // 默认TTL

	// === 清理配置 ===
	CleanupInterval time.Duration `json:"cleanup_interval"` // 清理间隔
}

// Config 内存存储配置实现
type Config struct {
	options *MemoryOptions
}

// New 创建内存存储配置实现
func New(userConfig interface{}) *Config {
	defaultOptions := createDefaultMemoryOptions()
	return &Config{
		options: defaultOptions,
	}
}

// createDefaultMemoryOptions 创建默认内存存储配置
func createDefaultMemoryOptions() *MemoryOptions {
	return &MemoryOptions{
		MaxMemory:       defaultMaxMemory,
		MaxEntries:      defaultMaxEntries,
		DefaultTTL:      defaultDefaultTTL,
		CleanupInterval: defaultCleanupInterval,
	}
}

// GetOptions 获取完整的内存存储配置选项
func (c *Config) GetOptions() *MemoryOptions {
	return c.options
}

// === 基础配置访问方法 ===

// GetMaxMemory 获取最大内存使用量
func (c *Config) GetMaxMemory() int64 {
	return c.options.MaxMemory
}

// GetMaxEntries 获取最大条目数
func (c *Config) GetMaxEntries() int {
	return c.options.MaxEntries
}

// GetDefaultTTL 获取默认TTL
func (c *Config) GetDefaultTTL() time.Duration {
	return c.options.DefaultTTL
}

// GetCleanupInterval 获取清理间隔
func (c *Config) GetCleanupInterval() time.Duration {
	return c.options.CleanupInterval
}

// === 兼容方法（与现有代码兼容） ===

// GetLifeWindow 获取生命周期窗口（兼容方法）
func (c *Config) GetLifeWindow() string {
	return c.options.DefaultTTL.String()
}

// GetCleanWindow 获取清理窗口（兼容方法）
func (c *Config) GetCleanWindow() string {
	return c.options.CleanupInterval.String()
}

// GetMaxEntriesInWindow 获取窗口内最大条目数（兼容方法）
// 优化：降低默认值以减少 BigCache 预分配内存
// 从 100000 降到 10000，可大幅减少内存占用
func (c *Config) GetMaxEntriesInWindow() int {
	// 如果配置的值太大，使用更合理的默认值
	if c.options.MaxEntries > 10000 {
		return 10000 // 限制最大条目数为 10000，减少预分配
	}
	return c.options.MaxEntries
}

// GetMaxEntrySize 获取最大条目大小（兼容方法）
// 优化：从 1MB 降低到 64KB，大幅减少 BigCache 预分配内存
// 64KB 足够大多数缓存条目使用，可减少约 93% 的内存占用
func (c *Config) GetMaxEntrySize() int {
	// 从 1MB 降低到 64KB，减少预分配内存
	// 预期效果：内存从 97GB 降到约 6.4GB（如果 MaxEntriesInWindow=100000）
	// 或降到约 640MB（如果 MaxEntriesInWindow=10000）
	return 64 * 1024 // 64KB
}
