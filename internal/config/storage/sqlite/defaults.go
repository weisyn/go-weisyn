// Package sqlite provides default configuration values for SQLite storage.
package sqlite

import "time"

// SQLite存储配置默认值
const (
	// 数据库文件路径
	// defaultDBPath 默认数据库路径
	// 原因：使用相对路径便于部署
	defaultDBPath = "./data/sqlite/storage.db"

	// 连接池配置
	// defaultMaxOpenConns 默认最大打开连接数设为25
	// 原因：25个连接平衡并发性能和资源消耗
	defaultMaxOpenConns = 25

	// defaultMaxIdleConns 默认最大空闲连接数设为10
	// 原因：10个空闲连接提供足够的连接复用
	defaultMaxIdleConns = 10

	// defaultConnMaxLifetime 默认连接最大生存时间设为5分钟
	// 原因：5分钟生存时间平衡连接复用和资源释放
	defaultConnMaxLifetime = 5 * time.Minute

	// 事务配置
	// defaultTxTimeout 默认事务超时时间设为30秒
	// 原因：30秒足够完成大多数事务操作
	defaultTxTimeout = 30 * time.Second

	// defaultBusyTimeout 默认忙等待超时时间设为10秒
	// 原因：10秒超时避免长时间等待锁
	defaultBusyTimeout = 10 * time.Second

	// 性能配置
	// defaultCacheSize 默认缓存大小设为-64000（64MB）
	// 原因：合理的缓存大小提高查询性能
	defaultCacheSize = -64000 // 64MB缓存

	// defaultPageSize 默认页大小设为4KB
	// 原因：4KB页大小是SQLite的标准配置
	defaultPageSize = 4096 // 4KB页大小

	// defaultSyncMode 默认同步模式设为"NORMAL"
	// 原因：NORMAL模式平衡性能和安全性
	defaultSyncMode = "NORMAL" // 同步模式

	// defaultJournalMode 默认日志模式设为"WAL"
	// 原因：WAL模式提供更好的并发性能
	defaultJournalMode = "WAL" // 日志模式

	// 其他配置
	// defaultVacuumInterval 默认清理间隔设为24小时
	// 原因：24小时清理间隔保持数据库性能
	defaultVacuumInterval = 24 * time.Hour

	// defaultEnableWAL 默认启用WAL模式
	// 原因：WAL模式提供更好的并发性能
	defaultEnableWAL = true

	// defaultEnableForeignKeys 默认启用外键约束
	// 原因：外键约束确保数据完整性
	defaultEnableForeignKeys = true
)
