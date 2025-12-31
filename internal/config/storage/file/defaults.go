// Package file provides default configuration values for file storage.
package file

// 文件存储配置默认值
const (
	// defaultMaxFileSize 默认最大文件大小设为1GB (in MB)
	// 原因：1GB足够存储大型文件，同时避免内存压力
	defaultMaxFileSize = int64(1024) // 1GB (in MB)

	// defaultDirectoryIndexEnabled 默认启用目录索引
	// 原因：目录索引提高文件查找效率
	defaultDirectoryIndexEnabled = true

	// defaultFileVerificationEnabled 默认启用文件验证
	// 原因：文件验证确保数据完整性
	defaultFileVerificationEnabled = true

	// defaultFilePermissions 默认文件权限设为0644
	// 原因：0644权限允许所有者读写，其他用户只读
	defaultFilePermissions = 0644

	// defaultDirectoryPermissions 默认目录权限设为0755
	// 原因：0755权限允许所有者完全控制，其他用户可读可执行
	defaultDirectoryPermissions = 0755
)
