package file

import (
	"path/filepath"

	configtypes "github.com/weisyn/v1/pkg/types"
	"github.com/weisyn/v1/pkg/utils"
)

// FileOptions 文件存储配置选项
type FileOptions struct {
	RootPath                string `json:"root_path"`
	MaxFileSize             int64  `json:"max_file_size"`
	DirectoryIndexEnabled   bool   `json:"directory_index_enabled"`
	FileVerificationEnabled bool   `json:"file_verification_enabled"`
	FilePermissions         int    `json:"file_permissions"`
	DirectoryPermissions    int    `json:"directory_permissions"`
}

// Config 文件存储配置实现
type Config struct {
	options *FileOptions
}

// New 创建文件存储配置实现
func New(userConfig interface{}) *Config {
	defaultOptions := createDefaultFileOptions()

	// 如果有用户配置，应用用户配置覆盖默认值
	if userConfig != nil {
		applyUserConfig(defaultOptions, userConfig)
	}

	return &Config{
		options: defaultOptions,
	}
}

// NewFromOptions 从FileOptions创建配置实现
// 用于直接使用已构建的配置选项（例如从Provider获取的选项）
func NewFromOptions(options *FileOptions) *Config {
	if options == nil {
		// 如果选项为空，使用默认配置
		return New(nil)
	}
	return &Config{
		options: options,
	}
}

// createDefaultFileOptions 创建默认文件存储配置
func createDefaultFileOptions() *FileOptions {
	return &FileOptions{
		RootPath:                getDefaultPath(),
		MaxFileSize:             defaultMaxFileSize,
		DirectoryIndexEnabled:   defaultDirectoryIndexEnabled,
		FileVerificationEnabled: defaultFileVerificationEnabled,
		FilePermissions:         defaultFilePermissions,
		DirectoryPermissions:    defaultDirectoryPermissions,
	}
}

// getDefaultPath 获取默认文件存储路径（使用路径解析工具）
func getDefaultPath() string {
	return utils.ResolveDataPath("./data/files")
}

// applyUserConfig 应用用户配置覆盖默认值
// 
// 路径构建规则（遵循 data-architecture.md 标准）：
// - 如果配置了 storage.data_root，使用 {data_root}/files/
// - 如果未配置，使用默认值 ./data/files/（作为默认环境或测试环境）
func applyUserConfig(options *FileOptions, userConfig interface{}) {
	// 处理用户存储配置，只使用JSON中实际存在的字段
	if storageConfig, ok := userConfig.(*configtypes.UserStorageConfig); ok && storageConfig != nil {
		// 只处理 DataRoot 字段，其他字段使用 defaults.go 中的默认值
		if storageConfig.DataRoot != nil {
			// 使用配置的存储路径 + files子目录，并解析为绝对路径
			// 遵循统一标准：{data_root}/files/
			filesPath := filepath.Join(*storageConfig.DataRoot, "files")
			options.RootPath = utils.ResolveDataPath(filesPath)
		}
		// 如果 DataRoot 为 nil，使用 getDefaultPath() 返回的默认路径（./data/files）
	}
}

// GetOptions 获取完整的文件存储配置选项
func (c *Config) GetOptions() *FileOptions {
	return c.options
}

// GetRootPath 获取根目录路径
func (c *Config) GetRootPath() string {
	return c.options.RootPath
}

// GetMaxFileSize 获取最大文件大小限制(MB)
func (c *Config) GetMaxFileSize() int64 {
	return c.options.MaxFileSize
}

// IsDirectoryIndexEnabled 是否启用目录索引
func (c *Config) IsDirectoryIndexEnabled() bool {
	return c.options.DirectoryIndexEnabled
}

// IsFileVerificationEnabled 是否启用文件校验
func (c *Config) IsFileVerificationEnabled() bool {
	return c.options.FileVerificationEnabled
}

// GetFilePermissions 获取文件权限设置
func (c *Config) GetFilePermissions() int {
	return c.options.FilePermissions
}

// GetDirectoryPermissions 获取目录权限设置
func (c *Config) GetDirectoryPermissions() int {
	return c.options.DirectoryPermissions
}
