package temporary

import "time"

// TempOptions 临时存储配置选项
type TempOptions struct {
	TempPath        string        `json:"temp_path"`
	MaxSize         int64         `json:"max_size"`
	MaxFiles        int           `json:"max_files"`
	AutoCleanup     bool          `json:"auto_cleanup"`
	CleanupInterval time.Duration `json:"cleanup_interval"`
	MaxAge          time.Duration `json:"max_age"`
	CacheSize       int64         `json:"cache_size"`
	BufferSize      int           `json:"buffer_size"`
	DirPerm         int           `json:"dir_perm"`
	FilePerm        int           `json:"file_perm"`
}

// Config 临时存储配置实现
type Config struct {
	options *TempOptions
}

// New 创建临时存储配置实现
func New(userConfig interface{}) *Config {
	return &Config{
		options: &TempOptions{
			TempPath:        defaultTempPath,
			MaxSize:         defaultMaxSize,
			MaxFiles:        defaultMaxFiles,
			AutoCleanup:     defaultAutoCleanup,
			CleanupInterval: defaultCleanupInterval,
			MaxAge:          defaultMaxAge,
			CacheSize:       defaultCacheSize,
			BufferSize:      defaultBufferSize,
			DirPerm:         defaultDirPerm,
			FilePerm:        defaultFilePerm,
		},
	}
}

// GetOptions 获取完整的临时存储配置选项
func (c *Config) GetOptions() *TempOptions {
	return c.options
}

// GetTempPath 获取临时存储路径
func (c *Config) GetTempPath() string {
	return c.options.TempPath
}

// GetTempDir 获取临时文件根目录
func (c *Config) GetTempDir() string {
	return c.options.TempPath
}

// GetDefaultTTL 获取默认过期时间
func (c *Config) GetDefaultTTL() time.Duration {
	return c.options.MaxAge
}

// GetMaxTempFileSize 获取最大临时文件大小(MB)
func (c *Config) GetMaxTempFileSize() int64 {
	return c.options.MaxSize / (1024 * 1024) // 转换为MB
}

// GetCleanupInterval 获取清理任务间隔
func (c *Config) GetCleanupInterval() time.Duration {
	return c.options.CleanupInterval
}

// GetMaxTempFiles 获取最大临时文件数量
func (c *Config) GetMaxTempFiles() int {
	return c.options.MaxFiles
}

// IsAutoCleanupEnabled 是否启用自动清理
func (c *Config) IsAutoCleanupEnabled() bool {
	return c.options.AutoCleanup
}

// GetFilePermissions 获取文件权限设置
func (c *Config) GetFilePermissions() int {
	return c.options.FilePerm
}

// GetDirectoryPermissions 获取目录权限设置
func (c *Config) GetDirectoryPermissions() int {
	return c.options.DirPerm
}
