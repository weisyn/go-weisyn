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

// moduleRoutingCore 基于 module 字段的路由 Core
// 根据日志中的 module 字段决定写入 system.log 还是 business.log
type moduleRoutingCore struct {
	systemCore   zapcore.Core
	businessCore zapcore.Core
	fallbackCore zapcore.Core // 没有 module 字段时的默认 core
}

// Enabled 实现 zapcore.Core 接口
func (c *moduleRoutingCore) Enabled(level zapcore.Level) bool {
	// 只要任一 core 启用，就返回 true
	return c.systemCore.Enabled(level) || c.businessCore.Enabled(level) || c.fallbackCore.Enabled(level)
}

// With 实现 zapcore.Core 接口
func (c *moduleRoutingCore) With(fields []zapcore.Field) zapcore.Core {
	return &moduleRoutingCore{
		systemCore:   c.systemCore.With(fields),
		businessCore: c.businessCore.With(fields),
		fallbackCore: c.fallbackCore.With(fields),
	}
}

// Check 实现 zapcore.Core 接口
// 注意：在 Check 阶段无法获取字段信息，所以我们需要让所有 core 都通过 Check
// 然后在 Write 阶段根据字段信息进行路由
func (c *moduleRoutingCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	// 让所有 core 都通过 Check，实际路由在 Write 中进行
	if c.systemCore.Enabled(entry.Level) || c.businessCore.Enabled(entry.Level) {
		return checked.AddCore(entry, c)
	}
	return checked
}

// Write 实现 zapcore.Core 接口
// 在这里根据字段中的 module 信息进行路由
func (c *moduleRoutingCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	// 检查字段中的 module 信息
	var module string
	for _, field := range fields {
		if field.Key == "module" {
			// zap.String("module", "x") 会写入 field.String（Type=StringType），不是 field.Interface
			switch field.Type {
			case zapcore.StringType:
				module = field.String
			case zapcore.StringerType:
				if s, ok := field.Interface.(fmt.Stringer); ok && s != nil {
					module = s.String()
				}
			case zapcore.ReflectType:
				// 兜底：部分 zap.Any 可能把 string 放在 Interface 中
				if str, ok := field.Interface.(string); ok {
					module = str
				}
			default:
				// 兜底：保持兼容旧实现
				if str, ok := field.Interface.(string); ok {
					module = str
				}
			}
			if module != "" {
				break
			}
		}
	}

	// 根据 module 字段决定写入哪个文件
	if isSystemModule(module) {
		return c.systemCore.Write(entry, fields)
	} else if isBusinessModule(module) {
		return c.businessCore.Write(entry, fields)
	} else {
		// 没有 module 字段或未知 module，写入两个文件
		var errs []error
		if err := c.systemCore.Write(entry, fields); err != nil {
			errs = append(errs, err)
		}
		if err := c.businessCore.Write(entry, fields); err != nil {
			errs = append(errs, err)
		}
		if len(errs) > 0 {
			return fmt.Errorf("写入日志失败: %v", errs)
		}
		return nil
	}
}

// Sync 实现 zapcore.Core 接口
func (c *moduleRoutingCore) Sync() error {
	var errs []error
	if err := c.systemCore.Sync(); err != nil {
		errs = append(errs, err)
	}
	if err := c.businessCore.Sync(); err != nil {
		errs = append(errs, err)
	}
	if err := c.fallbackCore.Sync(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("同步日志文件失败: %v", errs)
	}
	return nil
}

// isSystemModule 判断是否为系统模块
// 系统模块包括：p2p, consensus, storage, network, sync 等基础设施模块
func isSystemModule(module string) bool {
	systemModules := map[string]bool{
		"p2p":        true, // P2P 网络主机和发现
		"consensus":  true, // 共识算法和区块生成
		"storage":   true, // 存储子系统
		"persistence": true, // 持久化查询服务
		"network":    true, // 网络层（GossipSub、消息路由）
		"chain":      true, // 链状态管理和同步
		"block":      true, // 区块构建、验证和处理
		"event":      true, // 事件总线
		"kademlia":  true, // Kademlia 路由表
		"compliance": true, // 合规服务
		"crypto":     true, // 加密模块
		"sync":       true, // 同步服务（兼容旧代码）
		"infra":      true, // 基础设施模块（通用）
		"system":     true, // 系统模块（通用）
	}
	return systemModules[module]
}

// isBusinessModule 判断是否为业务模块
// 业务模块包括：api, executor, contract, workbench, tx 等业务逻辑模块
func isBusinessModule(module string) bool {
	businessModules := map[string]bool{
		"api":       true, // HTTP/JSON-RPC/gRPC API
		"executor":  true, // 合约执行器（ISPC）
		"tx":        true, // 交易处理
		"mempool":   true, // 内存池（交易池和候选区块池）
		"ures":      true, // URES 资源存储
		"eutxo":     true, // EUTXO 模型
		"contract":  true, // 智能合约相关（兼容旧代码）
		"workbench": true, // Workbench 交互（兼容旧代码）
		"business":  true, // 业务逻辑模块（通用）
		"app":       true, // 应用层模块（通用）
	}
	return businessModules[module]
}

// createFileWriter 创建日志文件写入器
func createFileWriter(logPath string, config *logconfig.Config) zapcore.WriteSyncer {
	// 确保日志目录存在
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0700); err != nil {
		// 如果创建目录失败，输出到 stderr
		fmt.Fprintf(os.Stderr, "创建日志目录失败 %s: %v\n", logDir, err)
		return zapcore.AddSync(os.Stderr)
	}

	// 配置日志轮转
	return zapcore.AddSync(&lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    config.GetMaxSize(),           // megabytes
		MaxBackups: config.GetMaxBackups(),        // 最多保留文件数
		MaxAge:     config.GetMaxAge(),            // days
		Compress:   config.IsCompressionEnabled(), // 是否压缩
	})
}

// NewLogger 根据配置创建新的日志记录器
func New(config *logconfig.Config) (logInterface.Logger, error) {
	level := config.GetZapLevel()

	// 使用配置提供的编码器
	consoleEncoder := config.CreateConsoleEncoder()
	fileEncoder := config.CreateFileEncoder()

	// 设置输出
	var cores []zapcore.Core

	// 1. 如果配置了控制台输出
	outputPath := config.GetFilePath()
	// ✅ CLI模式：强制禁用控制台输出（即使配置中启用了）
	shouldOutputToConsole := os.Getenv("WES_CLI_MODE") != "true" && (outputPath == "stdout" || outputPath == "stderr" || config.IsConsoleEnabled())
	if shouldOutputToConsole {
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
		// 检查是否是默认路径（./data/logs/weisyn.log），如果是则跳过文件输出
		isDefaultPath := false
		if outputPath == "./data/logs/weisyn.log" || strings.HasSuffix(outputPath, "/data/logs/weisyn.log") || strings.HasSuffix(outputPath, "data/logs/weisyn.log") {
			isDefaultPath = true
		} else {
			// 检查绝对路径是否指向默认位置
			if filepath.IsAbs(outputPath) {
				currentDir, err := os.Getwd()
				if err == nil {
					defaultPath := filepath.Join(currentDir, "data", "logs", "weisyn.log")
					defaultAbs, _ := filepath.Abs(defaultPath)
					outputAbs, _ := filepath.Abs(outputPath)
					if defaultAbs == outputAbs {
						isDefaultPath = true
					}
				}
			}
		}

		if isDefaultPath {
			// 跳过默认路径，使用控制台输出（init() 时的临时方案）
			var output zapcore.WriteSyncer
			if config.IsConsoleEnabled() {
				output = zapcore.AddSync(os.Stdout)
			} else {
				output = zapcore.AddSync(os.Stderr)
			}
			cores = append(cores, zapcore.NewCore(consoleEncoder, output, zap.NewAtomicLevelAt(level)))
		} else {
			// 🎯 多文件日志架构：根据配置决定使用单文件还是多文件
			if config.IsMultiFileEnabled() {
				// 多文件模式：system.log + business.log
				logDir := config.GetLogDir()
				if logDir == "" {
					logDir = filepath.Dir(outputPath)
				}

				systemLogPath := filepath.Join(logDir, config.GetSystemLogFile())
				businessLogPath := filepath.Join(logDir, config.GetBusinessLogFile())

				// 确保路径是绝对路径
				if !filepath.IsAbs(systemLogPath) {
					currentDir, err := os.Getwd()
					if err != nil {
						return nil, fmt.Errorf("获取当前工作目录失败: %w", err)
					}
					systemLogPath = filepath.Join(currentDir, systemLogPath)
					businessLogPath = filepath.Join(currentDir, businessLogPath)
				}

				systemLogPath, _ = filepath.Abs(systemLogPath)
				businessLogPath, _ = filepath.Abs(businessLogPath)

				// 打印日志文件路径，方便调试（CLI模式下抑制输出）
				if os.Getenv("WES_CLI_MODE") != "true" {
					fmt.Printf("系统日志文件: %s\n", systemLogPath)
					fmt.Printf("业务日志文件: %s\n", businessLogPath)
				}

				// 创建文件写入器
				systemWriter := createFileWriter(systemLogPath, config)
				businessWriter := createFileWriter(businessLogPath, config)

				// 创建 system 和 business 的 core
				systemCore := zapcore.NewCore(fileEncoder, systemWriter, zap.NewAtomicLevelAt(level))
				businessCore := zapcore.NewCore(fileEncoder, businessWriter, zap.NewAtomicLevelAt(level))

				// 创建路由 core，根据 module 字段路由日志
				routingCore := &moduleRoutingCore{
					systemCore:   systemCore,
					businessCore: businessCore,
					fallbackCore: zapcore.NewTee(systemCore, businessCore), // 没有 module 字段时写入两个文件
				}

				cores = append(cores, routingCore)
			} else {
				// 单文件模式：使用原来的逻辑
				var logPath string

				// 检查是否已经是绝对路径
				if filepath.IsAbs(outputPath) {
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

				// 打印日志文件路径，方便调试（CLI模式下抑制输出）
				if os.Getenv("WES_CLI_MODE") != "true" {
					fmt.Printf("日志文件将创建在: %s\n", absPath)
				}

				// 配置日志轮转
				fileWriter := createFileWriter(absPath, config)
				cores = append(cores, zapcore.NewCore(fileEncoder, fileWriter, zap.NewAtomicLevelAt(level)))
			}
		}
	}

	// 合并所有的Cores
	core := zapcore.NewTee(cores...)

	// 创建日志记录器
	zapOptions := []zap.Option{}

	// 添加调用者信息
	if config.IsCallerEnabled() {
		zapOptions = append(zapOptions, zap.AddCaller())
		// 跳过一层日志封装，使调用位置指向真实业务代码位置（而非本文件）
		zapOptions = append(zapOptions, zap.AddCallerSkip(1))
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
