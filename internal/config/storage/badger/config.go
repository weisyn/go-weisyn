package badger

import (
	"path/filepath"

	configtypes "github.com/weisyn/v1/pkg/types"
	"github.com/weisyn/v1/pkg/utils"
)

// BadgerOptions BadgerDB存储配置选项
// 专注于基础设施核心功能的简化配置
type BadgerOptions struct {
	// === 基础配置 ===
	Path       string `json:"path"`        // 数据库存储路径
	SyncWrites bool   `json:"sync_writes"` // 是否同步写入（数据安全性）

	// === 基础性能配置 ===
	MemTableSize int64 `json:"mem_table_size"` // 内存表大小

	// === 维护配置 ===
	EnableAutoCompaction bool `json:"enable_auto_compaction"` // 是否启用自动压缩
}

// Config BadgerDB配置实现
type Config struct {
	options *BadgerOptions
}

// New 创建BadgerDB配置实现
func New(userConfig interface{}) *Config {
	defaultOptions := createDefaultBadgerOptions()

	// 如果有用户配置，应用用户配置覆盖默认值
	if userConfig != nil {
		applyUserConfig(defaultOptions, userConfig)
	}

	return &Config{
		options: defaultOptions,
	}
}

// NewFromOptions 从BadgerOptions创建配置实现
func NewFromOptions(options *BadgerOptions) *Config {
	return &Config{
		options: options,
	}
}

// createDefaultBadgerOptions 创建默认BadgerDB配置
func createDefaultBadgerOptions() *BadgerOptions {
	return &BadgerOptions{
		Path:                 getDefaultPath(), // 使用函数获取路径，确保正确解析
		SyncWrites:           defaultSyncWrites,
		MemTableSize:         defaultMemTableSize,
		EnableAutoCompaction: defaultEnableAutoCompaction,
	}
}

// applyUserConfig 应用用户配置覆盖默认值
// 
// 路径构建规则（遵循 data-architecture.md 标准）：
// - 如果配置了 storage.data_root，使用 {data_root}/badger/
// - 如果未配置，使用默认值 ./data/badger/（作为默认环境或测试环境）
func applyUserConfig(options *BadgerOptions, userConfig interface{}) {
	// 处理用户存储配置，只使用JSON中实际存在的字段
	if storageConfig, ok := userConfig.(*configtypes.UserStorageConfig); ok && storageConfig != nil {
		// 只处理 DataRoot 字段，其他字段使用 defaults.go 中的默认值
		if storageConfig.DataRoot != nil {
			// 使用配置的存储路径 + badger子目录，并解析为绝对路径
			// 遵循统一标准：{data_root}/badger/
			badgerPath := filepath.Join(*storageConfig.DataRoot, "badger")
			options.Path = utils.ResolveDataPath(badgerPath)
		}
		// 如果 DataRoot 为 nil，使用 getDefaultPath() 返回的默认路径（./data/badger）
	}
}

// GetOptions 获取完整的BadgerDB配置选项
func (c *Config) GetOptions() *BadgerOptions {
	return c.options
}

// === 基础配置访问方法 ===

// GetPath 获取数据库路径
func (c *Config) GetPath() string {
	return c.options.Path
}

// IsSyncWritesEnabled 是否启用同步写入
func (c *Config) IsSyncWritesEnabled() bool {
	return c.options.SyncWrites
}

// GetMemTableSize 获取内存表大小
func (c *Config) GetMemTableSize() int64 {
	return c.options.MemTableSize
}

// IsAutoCompactionEnabled 是否启用自动压缩
func (c *Config) IsAutoCompactionEnabled() bool {
	return c.options.EnableAutoCompaction
}
