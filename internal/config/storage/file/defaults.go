// Package file 提供文件存储的默认配置
package file

// 默认配置值
const (
	defaultRootPath                = "./data/files"
	defaultMaxFileSize             = int64(1024) // 1GB (in MB)
	defaultDirectoryIndexEnabled   = true
	defaultFileVerificationEnabled = true
	defaultFilePermissions         = 0644
	defaultDirectoryPermissions    = 0755
)
