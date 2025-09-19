package temporary

import "time"

// 临时存储配置默认值
const (
	// 临时存储路径
	defaultTempPath = "./data/temp"

	// 大小限制
	defaultMaxSize  = 1024 * 1024 * 1024 // 1GB
	defaultMaxFiles = 10000

	// 清理配置
	defaultAutoCleanup     = true
	defaultCleanupInterval = 1 * time.Hour
	defaultMaxAge          = 24 * time.Hour

	// 性能配置
	defaultCacheSize  = 64 * 1024 * 1024 // 64MB
	defaultBufferSize = 32 * 1024        // 32KB

	// 权限配置
	defaultDirPerm  = 0755
	defaultFilePerm = 0644
)
