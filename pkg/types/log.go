package types

// LogLevel 日志级别类型（从 pkg/interfaces/infrastructure/log/level.go 迁移）
type LogLevel string

const (
	DebugLevel LogLevel = "debug"
	InfoLevel  LogLevel = "info"
	WarnLevel  LogLevel = "warn"
	ErrorLevel LogLevel = "error"
	FatalLevel LogLevel = "fatal"
)
