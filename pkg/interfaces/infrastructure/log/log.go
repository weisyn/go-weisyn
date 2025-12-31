// Package log 提供WES系统的核心日志记录接口定义
//
// 📋 **日志系统核心接口 (Core Logging System Interface)**
//
// 本文件定义了WES系统的核心日志接口，专注于：
// - 统一的日志记录接口
// - 结构化日志和上下文支持
// - 日志输出的性能优化
// - 多级别日志的统一管理
//
// 🎯 **设计原则**
// - 统一接口：为所有模块提供统一的日志接口
// - 结构化：支持结构化日志和元数据附加
// - 高性能：优化日志处理性能，支持异步输出
// - 灵活配置：支持灵活的日志级别和输出配置
package log

import "go.uber.org/zap"

// Logger 定义日志记录器接口
type Logger interface {
	// Debug 记录调试级别的日志
	Debug(msg string)

	// Debugf 使用格式化字符串记录调试级别的日志
	Debugf(format string, args ...interface{})

	// Info 记录信息级别的日志
	Info(msg string)

	// Infof 使用格式化字符串记录信息级别的日志
	Infof(format string, args ...interface{})

	// Warn 记录警告级别的日志
	Warn(msg string)

	// Warnf 使用格式化字符串记录警告级别的日志
	Warnf(format string, args ...interface{})

	// Error 记录错误级别的日志
	Error(msg string)

	// Errorf 使用格式化字符串记录错误级别的日志
	Errorf(format string, args ...interface{})

	// Fatal 记录致命级别的日志，然后退出程序
	Fatal(msg string)

	// Fatalf 使用格式化字符串记录致命级别的日志，然后退出程序
	Fatalf(format string, args ...interface{})

	// With 返回一个带有额外字段的Logger
	With(args ...interface{}) Logger

	// Sync 同步日志缓冲区到输出
	Sync() error

	// 注意：日志记录器由DI容器自动管理资源，无需手动Close()

	// GetZapLogger 获取原始的zap日志记录器
	GetZapLogger() *zap.Logger
}
