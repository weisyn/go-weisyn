package sqlite

import "time"

// SQLiteOptions SQLite存储配置选项
type SQLiteOptions struct {
	DBPath            string        `json:"db_path"`
	MaxOpenConns      int           `json:"max_open_conns"`
	MaxIdleConns      int           `json:"max_idle_conns"`
	ConnMaxLifetime   time.Duration `json:"conn_max_lifetime"`
	TxTimeout         time.Duration `json:"tx_timeout"`
	BusyTimeout       time.Duration `json:"busy_timeout"`
	CacheSize         int           `json:"cache_size"`
	PageSize          int           `json:"page_size"`
	SyncMode          string        `json:"sync_mode"`
	JournalMode       string        `json:"journal_mode"`
	VacuumInterval    time.Duration `json:"vacuum_interval"`
	EnableWAL         bool          `json:"enable_wal"`
	EnableForeignKeys bool          `json:"enable_foreign_keys"`
}

// Config SQLite配置实现
type Config struct {
	options *SQLiteOptions
}

// New 创建SQLite配置实现
func New(userConfig interface{}) *Config {
	return &Config{
		options: &SQLiteOptions{
			DBPath:            defaultDBPath,
			MaxOpenConns:      defaultMaxOpenConns,
			MaxIdleConns:      defaultMaxIdleConns,
			ConnMaxLifetime:   defaultConnMaxLifetime,
			TxTimeout:         defaultTxTimeout,
			BusyTimeout:       defaultBusyTimeout,
			CacheSize:         defaultCacheSize,
			PageSize:          defaultPageSize,
			SyncMode:          defaultSyncMode,
			JournalMode:       defaultJournalMode,
			VacuumInterval:    defaultVacuumInterval,
			EnableWAL:         defaultEnableWAL,
			EnableForeignKeys: defaultEnableForeignKeys,
		},
	}
}

// GetOptions 获取完整的SQLite配置选项
func (c *Config) GetOptions() *SQLiteOptions {
	return c.options
}

// GetDBPath 获取数据库路径
func (c *Config) GetDBPath() string {
	return c.options.DBPath
}
