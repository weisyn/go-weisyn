package sqlite

import "time"

// SQLite存储配置默认值
const (
	// 数据库文件路径
	defaultDBPath = "./data/sqlite/storage.db"

	// 连接池配置
	defaultMaxOpenConns    = 25
	defaultMaxIdleConns    = 10
	defaultConnMaxLifetime = 5 * time.Minute

	// 事务配置
	defaultTxTimeout   = 30 * time.Second
	defaultBusyTimeout = 10 * time.Second

	// 性能配置
	defaultCacheSize   = -64000   // 64MB缓存
	defaultPageSize    = 4096     // 4KB页大小
	defaultSyncMode    = "NORMAL" // 同步模式
	defaultJournalMode = "WAL"    // 日志模式

	// 其他配置
	defaultVacuumInterval    = 24 * time.Hour
	defaultEnableWAL         = true
	defaultEnableForeignKeys = true
)
