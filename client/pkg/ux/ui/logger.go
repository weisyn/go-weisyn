// Package ui 提供基础 UI 组件库
package ui

// Logger 日志接口（适配器，适用于各种日志实现）
//
// 目的：
//   - 解耦 UI 组件与具体的日志实现（zap/zerolog/标准库等）
//   - 允许客户端传入自己的日志器
//   - 如果不需要日志，可以传入 nil
type Logger interface {
	// Debug 输出调试级别日志
	Debug(msg string)
	Debugf(format string, args ...interface{})

	// Info 输出信息级别日志
	Info(msg string)
	Infof(format string, args ...interface{})

	// Warn 输出警告级别日志
	Warn(msg string)
	Warnf(format string, args ...interface{})

	// Error 输出错误级别日志
	Error(msg string)
	Errorf(format string, args ...interface{})
}

// noopLogger 空日志实现（不输出任何日志）
type noopLogger struct{}

func (l *noopLogger) Debug(_msg string)                       {}
func (l *noopLogger) Debugf(_format string, args ...interface{}) {}
func (l *noopLogger) Info(_msg string)                        {}
func (l *noopLogger) Infof(_format string, args ...interface{})  {}
func (l *noopLogger) Warn(_msg string)                        {}
func (l *noopLogger) Warnf(_format string, args ...interface{})  {}
func (l *noopLogger) Error(_msg string)                       {}
func (l *noopLogger) Errorf(_format string, args ...interface{}) {}

// NoopLogger 返回一个空日志实例（不输出任何日志）
func NoopLogger() Logger {
	return &noopLogger{}
}

