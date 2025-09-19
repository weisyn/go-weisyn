package file

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
	return &Config{
		options: &FileOptions{
			RootPath:                defaultRootPath,
			MaxFileSize:             defaultMaxFileSize,
			DirectoryIndexEnabled:   defaultDirectoryIndexEnabled,
			FileVerificationEnabled: defaultFileVerificationEnabled,
			FilePermissions:         defaultFilePermissions,
			DirectoryPermissions:    defaultDirectoryPermissions,
		},
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
