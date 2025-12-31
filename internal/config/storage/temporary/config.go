package temporary

import (
	"path/filepath"
	"time"

	configtypes "github.com/weisyn/v1/pkg/types"
	"github.com/weisyn/v1/pkg/utils"
)

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
	defaultOptions := createDefaultTempOptions()

	// 如果有用户配置，应用用户配置覆盖默认值
	if userConfig != nil {
		applyUserConfig(defaultOptions, userConfig)
	}

	return &Config{
		options: defaultOptions,
	}
}

// NewFromOptions 从TempOptions创建配置实现
// 用于直接使用已构建的配置选项（例如从Provider获取的选项）
func NewFromOptions(options *TempOptions) *Config {
	if options == nil {
		// 如果选项为空，使用默认配置
		return New(nil)
	}
	return &Config{
		options: options,
	}
}

// createDefaultTempOptions 创建默认临时存储配置
func createDefaultTempOptions() *TempOptions {
	return &TempOptions{
		TempPath:        getDefaultPath(),
		MaxSize:         defaultMaxSize,
		MaxFiles:        defaultMaxFiles,
		AutoCleanup:     defaultAutoCleanup,
		CleanupInterval: defaultCleanupInterval,
		MaxAge:          defaultMaxAge,
		CacheSize:       defaultCacheSize,
		BufferSize:      defaultBufferSize,
		DirPerm:         defaultDirPerm,
		FilePerm:        defaultFilePerm,
	}
}

// getDefaultPath 获取默认临时存储路径（使用路径解析工具）
func getDefaultPath() string {
	return utils.ResolveDataPath("./data/temp")
}

// applyUserConfig 应用用户配置覆盖默认值
// 
// 路径构建规则（遵循 data-architecture.md 标准）：
// - 如果配置了 storage.data_root，使用 {data_root}/temp/
// - 如果未配置，使用默认值 ./data/temp/（作为默认环境或测试环境）
func applyUserConfig(options *TempOptions, userConfig interface{}) {
	// 处理用户存储配置，只使用JSON中实际存在的字段
	if storageConfig, ok := userConfig.(*configtypes.UserStorageConfig); ok && storageConfig != nil {
		// 只处理 DataRoot 字段，其他字段使用 defaults.go 中的默认值
		if storageConfig.DataRoot != nil {
			// 使用配置的存储路径 + temp子目录，并解析为绝对路径
			// 遵循统一标准：{data_root}/temp/
			tempPath := filepath.Join(*storageConfig.DataRoot, "temp")
			options.TempPath = utils.ResolveDataPath(tempPath)
		}
		// 如果 DataRoot 为 nil，使用 getDefaultPath() 返回的默认路径（./data/temp）
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
