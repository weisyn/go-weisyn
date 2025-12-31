// Package temporary provides default configuration values for temporary storage.
package temporary

import "time"

// 临时存储配置默认值
const (
	// 大小限制
	// defaultMaxSize 默认最大大小设为1GB
	// 原因：1GB足够存储临时文件，同时控制磁盘使用
	defaultMaxSize = 1024 * 1024 * 1024 // 1GB

	// defaultMaxFiles 默认最大文件数设为10000
	// 原因：10000个文件能满足大多数临时存储需求
	defaultMaxFiles = 10000

	// 清理配置
	// defaultAutoCleanup 默认启用自动清理
	// 原因：自动清理防止临时文件堆积
	defaultAutoCleanup = true

	// defaultCleanupInterval 默认清理间隔设为1小时
	// 原因：定期清理临时文件，避免磁盘空间浪费
	defaultCleanupInterval = 1 * time.Hour

	// defaultMaxAge 默认最大文件年龄设为24小时
	// 原因：24小时足够完成大多数临时操作
	defaultMaxAge = 24 * time.Hour

	// 性能配置
	// defaultCacheSize 默认缓存大小设为64MB
	// 原因：64MB缓存提高临时文件访问性能
	defaultCacheSize = 64 * 1024 * 1024 // 64MB

	// defaultBufferSize 默认缓冲区大小设为32KB
	// 原因：32KB缓冲区平衡内存使用和I/O效率
	defaultBufferSize = 32 * 1024 // 32KB

	// 权限配置
	// defaultDirPerm 默认目录权限设为0755
	// 原因：0755权限允许所有者完全控制，其他用户可读可执行
	defaultDirPerm = 0755

	// defaultFilePerm 默认文件权限设为0644
	// 原因：0644权限允许所有者读写，其他用户只读
	defaultFilePerm = 0644
)
