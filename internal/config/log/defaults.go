package log

import (
	"go.uber.org/zap/zapcore"
)

// 日志配置默认值
// 这些默认值基于生产环境的最佳实践和常见的日志配置
const (
	// === 基础日志配置 ===

	// defaultLogLevel 默认日志级别设为"info"
	// 原因：info级别平衡了信息量和性能，记录重要事件但不过于详细
	// 生产环境中info级别既能提供足够的诊断信息，又不会产生过多日志
	defaultLogLevel = "info"

	// defaultToConsole 默认启用控制台输出
	// 原因：开发和调试时需要实时查看日志，控制台输出提供即时反馈
	// 生产环境可通过配置禁用以提高性能
	defaultToConsole = true

	// defaultFilePath 默认日志文件路径为"./data/logs/weisyn.log"
	// 原因：统一的日志目录便于管理，使用小写文件名符合Unix惯例
	// data/logs目录结构清晰，便于日志轮转和备份
	defaultFilePath = "./data/logs/weisyn.log"

	// === 日志轮转配置 ===

	// defaultMaxSize 单个日志文件最大大小设为100MB
	// 原因：100MB足够记录大量日志信息，同时文件不会过大影响性能
	// 适中的文件大小便于日志分析工具处理和传输
	defaultMaxSize = 100

	// defaultMaxBackups 最大备份文件数设为10
	// 原因：保留10个备份文件提供足够的历史记录用于问题排查
	// 平衡磁盘空间使用和历史数据保留需求
	defaultMaxBackups = 10

	// defaultMaxAge 日志文件最大保留天数设为30天
	// 原因：30天覆盖了大多数问题排查的时间窗口
	// 符合一般的数据保留策略，平衡存储成本和数据价值
	defaultMaxAge = 30

	// defaultCompress 默认启用历史日志压缩
	// 原因：压缩可以显著减少磁盘空间占用，特别是对于大量日志
	// 现代系统的CPU资源相对充足，压缩的计算成本可以接受
	defaultCompress = true

	// === 调试配置 ===

	// defaultEnableCaller 默认启用调用者信息
	// 原因：调用者信息对于定位问题非常重要，特别是在复杂的代码库中
	// 虽然有轻微的性能开销，但诊断价值远大于成本
	defaultEnableCaller = true

	// defaultEnableStacktrace 默认对Error级别启用堆栈跟踪
	// 原因：堆栈跟踪对于错误诊断至关重要，但只在Error级别启用避免过度开销
	// 提供足够的错误上下文信息用于问题定位
	defaultEnableStacktrace = true
)

// 默认的日志级别映射
var defaultLevelMap = map[string]zapcore.Level{
	"debug": zapcore.DebugLevel,
	"info":  zapcore.InfoLevel,
	"warn":  zapcore.WarnLevel,
	"error": zapcore.ErrorLevel,
	"panic": zapcore.PanicLevel,
	"fatal": zapcore.FatalLevel,
}
