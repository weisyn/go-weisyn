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
func (c *Config) GetMaxEntriesInWindow() int {
	return c.options.MaxEntries
}

// GetMaxEntrySize 获取最大条目大小（兼容方法）
func (c *Config) GetMaxEntrySize() int {
	// 返回一个合理的默认值
	return 1024 * 1024 // 1MB
}
