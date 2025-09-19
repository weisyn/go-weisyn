// Package log 提供了一个通用的日志接口和基于zap的实现
// 它支持不同级别的日志记录、结构化日志、日志旋转等功能
package log

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	logconfig "github.com/weisyn/v1/internal/config/log"
	logInterface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// 日志级别定义
const (
	DebugLevel = string(logInterface.DebugLevel)
	InfoLevel  = string(logInterface.InfoLevel)
	WarnLevel  = string(logInterface.WarnLevel)
	ErrorLevel = string(logInterface.ErrorLevel)
	FatalLevel = string(logInterface.FatalLevel)
)

var (
	// 全局日志实例，使用接口类型
	globalLogger logInterface.Logger
	// 用于保护全局日志实例的互斥锁
	mu sync.RWMutex
)

// Logger 是日志记录器的结构体，实现了log.Logger接口
type Logger struct {
	zapLogger *zap.Logger
	sugar     *zap.SugaredLogger
}

// 初始化全局日志记录器
func init() {
	ResetDefault()
}

// ResetDefault 重置全局日志记录器为默认配置
func ResetDefault() {
	// 获取默认配置
	defaultConfig := logconfig.New(nil)

	logger, err := New(defaultConfig)
	if err != nil {
		// 在初始化日志器失败时使用控制台输出错误
		fmt.Fprintf(os.Stderr, "Failed to initialize default logger: %v\n", err)
		return
	}

	// 设置为全局记录器
	SetLogger(logger)
}

// NewLogger 根据配置创建新的日志记录器
func New(config *logconfig.Config) (logInterface.Logger, error) {
	level := config.GetZapLevel()

	// 使用配置提供的编码器，明确类型操作
	consoleEncoder := config.CreateConsoleEncoder()
	fileEncoder := config.CreateFileEncoder()

	// 设置输出
	var cores []zapcore.Core

	// 1. 如果配置了控制台输出
	outputPath := config.GetFilePath()
	if outputPath == "stdout" || outputPath == "stderr" || config.IsConsoleEnabled() {
		var output zapcore.WriteSyncer
		if outputPath == "stderr" {
			output = zapcore.AddSync(os.Stderr)
		} else {
			output = zapcore.AddSync(os.Stdout)
		}
		cores = append(cores, zapcore.NewCore(consoleEncoder, output, zap.NewAtomicLevelAt(level)))
	}

	// 2. 如果配置了文件输出
	if outputPath != "stdout" && outputPath != "stderr" {
		var logPath string

		// 检查是否已经是绝对路径
		if filepath.IsAbs(outputPath) {
			// 如果已经是绝对路径，直接使用
			logPath = outputPath
		} else {
			// 如果是相对路径，需要基于当前工作目录处理
			currentDir, err := os.Getwd()
			if err != nil {
				return nil, fmt.Errorf("获取当前工作目录失败: %w", err)
			}

			// 如果当前在cmd/node目录下，需要回到项目根目录
			if strings.HasSuffix(currentDir, "cmd/node") {
				currentDir = filepath.Dir(filepath.Dir(currentDir))
			}

			// 构建完整的日志文件路径
			logPath = filepath.Join(currentDir, outputPath)
		}

		// 将路径转换为绝对路径（确保路径规范化）
		absPath, err := filepath.Abs(logPath)
		if err != nil {
			return nil, fmt.Errorf("获取日志文件绝对路径失败: %w", err)
		}

		// 确保日志目录存在
		logDir := filepath.Dir(absPath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return nil, fmt.Errorf("创建日志目录失败: %w", err)
		}

		// 打印日志文件路径，方便调试（CLI模式下抑制输出）
		if os.Getenv("WES_CLI_MODE") != "true" {
			fmt.Printf("日志文件将创建在: %s\n", absPath)
		}

		// 配置日志轮转
		fileWriter := zapcore.AddSync(&lumberjack.Logger{
			Filename:   absPath,
			MaxSize:    config.GetMaxSize(),           // megabytes
			MaxBackups: config.GetMaxBackups(),        // 最多保留文件数
			MaxAge:     config.GetMaxAge(),            // days
			Compress:   config.IsCompressionEnabled(), // 是否压缩
		})

		cores = append(cores, zapcore.NewCore(fileEncoder, fileWriter, zap.NewAtomicLevelAt(level)))
	}

	// 合并所有的Cores
	core := zapcore.NewTee(cores...)

	// 创建日志记录器
	zapOptions := []zap.Option{}

	// 添加调用者信息
	if config.IsCallerEnabled() {
		zapOptions = append(zapOptions, zap.AddCaller())
	}

	// 添加堆栈跟踪
	if config.IsStacktraceEnabled() {
		zapOptions = append(zapOptions, zap.AddStacktrace(zapcore.ErrorLevel))
	}

	// 创建zap Logger
	zapLogger := zap.New(core, zapOptions...)
	sugar := zapLogger.Sugar()

	return &Logger{
		zapLogger: zapLogger,
		sugar:     sugar,
	}, nil
}

// NewLoggerFromConfig 从系统配置创建日志记录器
// 根据提供的参数创建配置并返回对应的日志记录器实例
func NewLoggerFromConfig(level string, outputPath string, encoding string, enableCaller bool, enableStacktrace bool) (logInterface.Logger, error) {
	// 创建日志选项并应用传入的参数
	options := &logconfig.LogOptions{
		Level:            level,
		FilePath:         outputPath,
		EnableCaller:     enableCaller,
		EnableStacktrace: enableStacktrace,
		ToConsole:        outputPath == "stdout" || outputPath == "stderr",
	}

	// 使用自定义选项创建配置
	logConfig := logconfig.New(options)

	return New(logConfig)
}

// GetZapLogger 获取底层的zap日志记录器
func (l *Logger) GetZapLogger() *zap.Logger {
	return l.zapLogger
}

// SetLogger 设置全局日志记录器
func SetLogger(logger logInterface.Logger) {
	if logger == nil {
		return
	}
	mu.Lock()
	globalLogger = logger
	mu.Unlock()
}

// GetLogger 获取全局日志记录器
func GetLogger() logInterface.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return globalLogger
}

// 以下是全局日志函数

// Debug 记录调试级别的日志
func Debug(msg string) {
	mu.RLock()
	defer mu.RUnlock()
	if globalLogger != nil {
		globalLogger.Debug(msg)
	}
}

// Debugf 使用格式化字符串记录调试级别的日志
func Debugf(format string, args ...interface{}) {
	mu.RLock()
	defer mu.RUnlock()
	if globalLogger != nil {
		globalLogger.Debugf(format, args...)
	}
}

// Info 记录信息级别的日志
func Info(msg string) {
	mu.RLock()
	defer mu.RUnlock()
	if globalLogger != nil {
		globalLogger.Info(msg)
	}
}

// Infof 使用格式化字符串记录信息级别的日志
func Infof(format string, args ...interface{}) {
	mu.RLock()
	defer mu.RUnlock()
	if globalLogger != nil {
		globalLogger.Infof(format, args...)
	}
}

// Warn 记录警告级别的日志
func Warn(msg string) {
	mu.RLock()
	defer mu.RUnlock()
	if globalLogger != nil {
		globalLogger.Warn(msg)
	}
}

// Warnf 使用格式化字符串记录警告级别的日志
func Warnf(format string, args ...interface{}) {
	mu.RLock()
	defer mu.RUnlock()
	if globalLogger != nil {
		globalLogger.Warnf(format, args...)
	}
}

// Error 记录错误级别的日志
func Error(msg string) {
	mu.RLock()
	defer mu.RUnlock()
	if globalLogger != nil {
		globalLogger.Error(msg)
	}
}

// Errorf 使用格式化字符串记录错误级别的日志
func Errorf(format string, args ...interface{}) {
	mu.RLock()
	defer mu.RUnlock()
	if globalLogger != nil {
		globalLogger.Errorf(format, args...)
	}
}

// Fatal 记录致命级别的日志，然后退出程序
func Fatal(msg string) {
	mu.RLock()
	defer mu.RUnlock()
	if globalLogger != nil {
		globalLogger.Fatal(msg)
	}
}

// Fatalf 使用格式化字符串记录致命级别的日志，然后退出程序
func Fatalf(format string, args ...interface{}) {
	mu.RLock()
	defer mu.RUnlock()
	if globalLogger != nil {
		globalLogger.Fatalf(format, args...)
	}
}

// With 创建带有额外字段的日志记录器
func With(args ...interface{}) logInterface.Logger {
	mu.RLock()
	defer mu.RUnlock()
	if globalLogger == nil {
		// 如果全局日志记录器不存在，初始化它
		ResetDefault()
	}

	// 使用接口的 With 方法返回新的日志记录器
	return globalLogger.With(args...)
}

// 将可变参数转换为zap字段
// 参数必须是偶数个，按键值对形式提供：key1, value1, key2, value2, ...
func toZapFields(args ...interface{}) []zap.Field {
	if len(args)%2 != 0 {
		// 参数不是偶数个，忽略最后一个参数以确保键值对的完整性
		// 这是严格的类型安全处理，不进行自动补充
		args = args[:len(args)-1]
	}

	fields := make([]zap.Field, 0, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		// 确保key是字符串类型
		key, ok := args[i].(string)
		if !ok {
			key = fmt.Sprint(args[i])
		}
		fields = append(fields, zap.Any(key, args[i+1]))
	}
	return fields
}

// Debug 记录调试级别的日志
func (l *Logger) Debug(msg string) {
	l.sugar.Debug(msg)
}

// Debugf 使用格式化字符串记录调试级别的日志
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.sugar.Debugf(format, args...)
}

// Info 记录信息级别的日志
func (l *Logger) Info(msg string) {
	l.sugar.Info(msg)
}

// Infof 使用格式化字符串记录信息级别的日志
func (l *Logger) Infof(format string, args ...interface{}) {
	l.sugar.Infof(format, args...)
}

// Warn 记录警告级别的日志
func (l *Logger) Warn(msg string) {
	l.sugar.Warn(msg)
}

// Warnf 使用格式化字符串记录警告级别的日志
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.sugar.Warnf(format, args...)
}

// Error 记录错误级别的日志
func (l *Logger) Error(msg string) {
	l.sugar.Error(msg)
}

// Errorf 使用格式化字符串记录错误级别的日志
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.sugar.Errorf(format, args...)
}

// Fatal 记录致命级别的日志，然后退出程序
func (l *Logger) Fatal(msg string) {
	l.sugar.Fatal(msg)
}

// Fatalf 使用格式化字符串记录致命级别的日志，然后退出程序
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.sugar.Fatalf(format, args...)
}

// With 返回一个带有额外字段的Logger
func (l *Logger) With(args ...interface{}) logInterface.Logger {
	return &Logger{
		zapLogger: l.zapLogger.With(toZapFields(args...)...),
		sugar:     l.sugar.With(args...),
	}
}

// Sync 同步日志缓冲区到输出
func (l *Logger) Sync() error {
	return l.zapLogger.Sync()
}

// Close 关闭日志记录器
func (l *Logger) Close() error {
	return l.zapLogger.Sync()
}
