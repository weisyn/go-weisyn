package log

import (
	"os"
	"path/filepath"

	configtypes "github.com/weisyn/v1/pkg/types"
	"github.com/weisyn/v1/pkg/utils"
	"go.uber.org/zap/zapcore"
)

// LogOptions 日志配置选项
// 专注于基础设施核心功能的简化配置
type LogOptions struct {
	// === 基础配置 ===
	Level     string `json:"level"`      // 日志级别 (debug, info, warn, error, fatal)
	ToConsole bool   `json:"to_console"` // 是否输出到控制台
	FilePath  string `json:"file_path"`  // 日志文件路径（已废弃，统一使用基于 storage.data_root / 实例数据目录的路径）

	// === 多文件日志配置 ===
	// 🎯 **多文件日志架构**：将日志按职责拆分为多个文件，提高可读性和可维护性
	EnableMultiFile bool   `json:"enable_multi_file"` // 是否启用多文件日志（默认true）
	SystemLogFile   string `json:"system_log_file"`   // 系统日志文件名（默认：node-system.log）
	BusinessLogFile string `json:"business_log_file"` // 业务日志文件名（默认：node-business.log）

	// === 基础轮转配置 ===
	MaxSize    int  `json:"max_size"`    // 单个日志文件最大大小(MB)
	MaxBackups int  `json:"max_backups"` // 最大备份文件数
	MaxAge     int  `json:"max_age"`     // 日志文件最大保留天数
	Compress   bool `json:"compress"`     // 是否压缩历史日志文件

	// === 调试配置 ===
	EnableCaller     bool `json:"enable_caller"`     // 是否启用调用者信息
	EnableStacktrace bool `json:"enable_stacktrace"` // 是否启用堆栈跟踪

	// === 内部配置（不对外暴露） ===
	LevelMap map[string]zapcore.Level `json:"-"` // 级别映射
	LogDir   string                   `json:"-"` // 日志目录（从 FilePath 推导）
}

// Config 日志配置实现
type Config struct {
	options *LogOptions
}

// UserLogConfigWithStorage 用户日志配置（包含存储配置用于路径解析）
type UserLogConfigWithStorage struct {
	Log     *configtypes.UserLogConfig
	Storage *configtypes.UserStorageConfig
}

// New 创建日志配置实现
func New(userConfig interface{}) *Config {
	// 1. 先创建完整的默认配置
	defaultOptions := createDefaultLogOptions()

	// 2. 如果有用户配置，应用用户配置覆盖默认值
	if userConfig != nil {
		applyUserLogConfig(defaultOptions, userConfig)
	}
	
	// ✅ CLI模式：强制禁用控制台输出（日志只写入文件，不干扰交互界面）
	// 注意：必须在最后检查，确保覆盖所有其他配置
	if os.Getenv("WES_CLI_MODE") == "true" {
		defaultOptions.ToConsole = false
	}

	return &Config{
		options: defaultOptions,
	}
}

// NewFromProvider 从配置提供者创建日志配置
func NewFromProvider(provider interface{}) *Config {
	// 类型断言获取配置提供者
	if p, ok := provider.(interface{ GetLog() *LogOptions }); ok {
		// 直接使用配置提供者返回的LogOptions
		options := p.GetLog()
		
		// ✅ CLI模式：强制禁用控制台输出（日志只写入文件，不干扰交互界面）
		// 注意：必须在日志配置创建时检查，因为后续不会调用 applyUserLogConfig
		if os.Getenv("WES_CLI_MODE") == "true" {
			options.ToConsole = false
		}
		
		return &Config{
			options: options,
		}
	}

	// 如果类型断言失败，回退到默认配置
	return New(nil)
}

// createDefaultLogOptions 创建默认日志配置
func createDefaultLogOptions() *LogOptions {
	defaultPath := getDefaultLogPath()
	logDir := filepath.Dir(defaultPath)
	
	return &LogOptions{
		// 基础配置
		Level:     defaultLogLevel,
		ToConsole: defaultToConsole,
		FilePath:  defaultPath,

		// 多文件日志配置
		EnableMultiFile: defaultEnableMultiFile,
		SystemLogFile:   defaultSystemLogFile,
		BusinessLogFile: defaultBusinessLogFile,

		// 基础轮转配置
		MaxSize:    defaultMaxSize,
		MaxBackups: defaultMaxBackups,
		MaxAge:     defaultMaxAge,
		Compress:   defaultCompress,

		// 调试配置
		EnableCaller:     defaultEnableCaller,
		EnableStacktrace: defaultEnableStacktrace,

		// 内部配置
		LevelMap: defaultLevelMap,
		LogDir:   logDir,
	}
}

// getDefaultLogPath 获取默认日志文件路径（使用路径解析工具）
func getDefaultLogPath() string {
	return utils.ResolveDataPath("./data/logs/weisyn.log")
}

// applyUserLogConfig 应用用户日志配置覆盖默认值
// 
// 路径构建规则（遵循 data-architecture.md 标准）：
// - 如果配置了 storage.data_root，优先使用 {data_root}/logs/weisyn.log（忽略显式的 log.file_path）
//   （在节点场景中，storage.data_root 由 Provider 设置为链实例数据目录 instance_data_dir）
// - 如果未配置 storage.data_root，使用默认值 ./data/logs/weisyn.log（作为默认环境或测试环境）
// 
// 🎯 **统一目录策略**：每个环境/链实例只有一个日志根目录 {instance_data_dir}/logs/
func applyUserLogConfig(options *LogOptions, userConfig interface{}) {
	// 优先处理 UserLogConfigWithStorage（包含 Storage 配置）
	if configWithStorage, ok := userConfig.(*UserLogConfigWithStorage); ok && configWithStorage != nil {
		// 🎯 关键：如果有 Storage 配置，优先使用 storage.data_root 构建日志路径
		// 即使配置文件中显式指定了 log.file_path，也统一使用 {data_root}/logs/weisyn.log
		// 在节点场景下，data_root 实际上等价于 instance_data_dir，
		// 这确保了每个链实例只有一个日志根目录
		if configWithStorage.Storage != nil && configWithStorage.Storage.DataRoot != nil {
			// 使用 storage.data_root + /logs/weisyn.log
			// 遵循统一标准：{data_root}/logs/weisyn.log
			logPath := filepath.Join(*configWithStorage.Storage.DataRoot, "logs", "weisyn.log")
			options.FilePath = utils.ResolveDataPath(logPath)
			// 更新日志目录
			options.LogDir = filepath.Dir(options.FilePath)
		}
		
		// 处理日志级别配置
		if configWithStorage.Log != nil {
			if configWithStorage.Log.Level != nil {
				options.Level = *configWithStorage.Log.Level
			}
			// ⚠️ 注意：不再处理 Log.FilePath，统一使用 storage.data_root / 实例数据目录推导的路径
			// 这确保了日志目录的统一性
		}
		return
	}

	// 向后兼容：处理 UserLogConfig（不包含 Storage 配置）
	// 这种情况通常发生在旧配置或测试场景中
	if logConfig, ok := userConfig.(*configtypes.UserLogConfig); ok && logConfig != nil {
		// 只处理JSON配置文件中实际出现的字段
		if logConfig.Level != nil {
			options.Level = *logConfig.Level
		}
		// ⚠️ 向后兼容：如果没有 Storage 配置，仍允许使用显式的 FilePath
		// 但建议迁移到使用 storage.data_root / 实例数据目录 的方式
		if logConfig.FilePath != nil {
			options.FilePath = utils.ResolveDataPath(*logConfig.FilePath)
			options.LogDir = filepath.Dir(options.FilePath)
			options.ToConsole = false // 指定文件路径时默认不输出到控制台
		}
	}
	
	// ✅ CLI模式：强制禁用控制台输出（日志只写入文件，不干扰交互界面）
	if os.Getenv("WES_CLI_MODE") == "true" {
		options.ToConsole = false
	}
}

// GetOptions 获取完整的日志配置选项
func (c *Config) GetOptions() *LogOptions {
	return c.options
}

// === 基础配置访问方法 ===

// GetLevel 获取日志级别
func (c *Config) GetLevel() string {
	return c.options.Level
}

// GetZapLevel 获取zap日志级别
func (c *Config) GetZapLevel() zapcore.Level {
	if level, exists := c.options.LevelMap[c.options.Level]; exists {
		return level
	}
	return zapcore.InfoLevel // 默认返回Info级别
}

// IsConsoleEnabled 是否启用控制台输出
func (c *Config) IsConsoleEnabled() bool {
	return c.options.ToConsole
}

// GetFilePath 获取日志文件路径
func (c *Config) GetFilePath() string {
	return c.options.FilePath
}

// GetLogDir 获取日志目录
func (c *Config) GetLogDir() string {
	return c.options.LogDir
}

// IsMultiFileEnabled 是否启用多文件日志
func (c *Config) IsMultiFileEnabled() bool {
	return c.options.EnableMultiFile
}

// GetSystemLogFile 获取系统日志文件名
func (c *Config) GetSystemLogFile() string {
	return c.options.SystemLogFile
}

// GetBusinessLogFile 获取业务日志文件名
func (c *Config) GetBusinessLogFile() string {
	return c.options.BusinessLogFile
}

// === 日志轮转配置访问方法 ===

// GetMaxSize 获取单个文件最大大小(MB)
func (c *Config) GetMaxSize() int {
	return c.options.MaxSize
}

// GetMaxBackups 获取最大备份文件数
func (c *Config) GetMaxBackups() int {
	return c.options.MaxBackups
}

// GetMaxAge 获取最大保留天数
func (c *Config) GetMaxAge() int {
	return c.options.MaxAge
}

// IsCompressionEnabled 是否启用压缩
func (c *Config) IsCompressionEnabled() bool {
	return c.options.Compress
}

// === 调试配置访问方法 ===

// IsCallerEnabled 是否启用调用者信息
func (c *Config) IsCallerEnabled() bool {
	return c.options.EnableCaller
}

// IsStacktraceEnabled 是否启用堆栈跟踪
func (c *Config) IsStacktraceEnabled() bool {
	return c.options.EnableStacktrace
}

// === 编码器创建方法 ===

// CreateFileEncoder 创建文件编码器 - 简化为JSON格式
func (c *Config) CreateFileEncoder() zapcore.Encoder {
	return zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
	})
}

// CreateConsoleEncoder 创建控制台编码器 - 简化为控制台格式
func (c *Config) CreateConsoleEncoder() zapcore.Encoder {
	return zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeTime:     zapcore.TimeEncoderOfLayout("15:04:05.000"),
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
	})
}
