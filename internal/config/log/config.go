package log

import (
	configtypes "github.com/weisyn/v1/pkg/types"
	"go.uber.org/zap/zapcore"
)

// LogOptions 日志配置选项
// 专注于基础设施核心功能的简化配置
type LogOptions struct {
	// === 基础配置 ===
	Level     string `json:"level"`      // 日志级别 (debug, info, warn, error, fatal)
	ToConsole bool   `json:"to_console"` // 是否输出到控制台
	FilePath  string `json:"file_path"`  // 日志文件路径

	// === 基础轮转配置 ===
	MaxSize    int  `json:"max_size"`    // 单个日志文件最大大小(MB)
	MaxBackups int  `json:"max_backups"` // 最大备份文件数
	MaxAge     int  `json:"max_age"`     // 日志文件最大保留天数
	Compress   bool `json:"compress"`    // 是否压缩历史日志文件

	// === 调试配置 ===
	EnableCaller     bool `json:"enable_caller"`     // 是否启用调用者信息
	EnableStacktrace bool `json:"enable_stacktrace"` // 是否启用堆栈跟踪

	// === 内部配置（不对外暴露） ===
	LevelMap map[string]zapcore.Level `json:"-"` // 级别映射
}

// Config 日志配置实现
type Config struct {
	options *LogOptions
}

// New 创建日志配置实现
func New(userConfig interface{}) *Config {
	// 1. 先创建完整的默认配置
	defaultOptions := createDefaultLogOptions()

	// 2. 如果有用户配置，应用用户配置覆盖默认值
	if userConfig != nil {
		applyUserLogConfig(defaultOptions, userConfig)
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
		return &Config{
			options: p.GetLog(),
		}
	}

	// 如果类型断言失败，回退到默认配置
	return New(nil)
}

// createDefaultLogOptions 创建默认日志配置
func createDefaultLogOptions() *LogOptions {
	return &LogOptions{
		// 基础配置
		Level:     defaultLogLevel,
		ToConsole: defaultToConsole,
		FilePath:  defaultFilePath,

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
	}
}

// applyUserLogConfig 应用用户日志配置覆盖默认值
func applyUserLogConfig(options *LogOptions, userConfig interface{}) {
	// 需要导入用户配置的类型
	if logConfig, ok := userConfig.(*configtypes.UserLogConfig); ok && logConfig != nil {
		// 只处理JSON配置文件中实际出现的字段
		if logConfig.Level != nil {
			options.Level = *logConfig.Level
		}
		if logConfig.FilePath != nil {
			options.FilePath = *logConfig.FilePath
			options.ToConsole = false // 指定文件路径时默认不输出到控制台
		}
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
