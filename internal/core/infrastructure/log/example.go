// Package log 示例文件演示了如何使用日志包
package log

import (
	logconfig "github.com/weisyn/v1/internal/config/log"
)

// Example 演示了如何使用日志包
func Example() {
	// 使用默认日志记录器
	Info("这是一条信息日志")
	Warn("这是一条警告日志")
	Error("这是一条错误日志")

	// 使用格式化日志
	Infof("用户 %s 登录成功，IP: %s", "admin", "192.168.1.1")

	// 带有结构化字段的日志
	With("userId", 12345, "action", "login").Info("用户登录")

	// 自定义日志记录器 - 使用新的配置系统
	options := &logconfig.LogOptions{
		Level:     "debug",
		FilePath:  "app.log",
		ToConsole: true,
	}
	logConfig := logconfig.New(options)
	// 注意：新的配置系统不支持动态设置，这些应该在创建配置时设置

	logger, err := New(logConfig)
	if err != nil {
		Fatal("无法创建日志记录器")
	}

	// 使用自定义日志记录器
	logger.Debug("这是一条调试日志")
	logger.With("requestId", "abc-123").Info("处理请求")

	// 注意：日志记录器资源由DI容器自动管理，无需手动关闭
}

// ExampleFileOutput 演示了如何将日志输出到文件
func ExampleFileOutput() {
	// 创建一个输出到文件的日志记录器
	options := &logconfig.LogOptions{
		Level:     "info",
		FilePath:  "logs/app.log",
		ToConsole: false,
	}
	logConfig := logconfig.New(options)

	logger, err := New(logConfig)
	if err != nil {
		Fatal("无法创建文件日志记录器")
	}

	// 使用日志记录器
	logger.Info("应用启动")
	logger.With("module", "api").Info("API服务启动")

	// 注意：日志记录器资源由DI容器自动管理，无需手动关闭
}

// ExampleMultipleLoggers 演示了如何使用多个日志记录器
func ExampleMultipleLoggers() {
	// 应用日志
	appOptions := &logconfig.LogOptions{
		Level:     InfoLevel,
		FilePath:  "logs/app.log",
		ToConsole: false,
		// FileEncoding不再支持: "json",
	}
	appConfig := logconfig.New(appOptions)
	appLogger, _ := New(appConfig)

	// 访问日志
	accessOptions := &logconfig.LogOptions{
		Level:     InfoLevel,
		FilePath:  "logs/access.log",
		ToConsole: false,
		// FileEncoding不再支持: "json",
	}
	accessConfig := logconfig.New(accessOptions)
	accessLogger, _ := New(accessConfig)

	// 错误日志
	errorOptions := &logconfig.LogOptions{
		Level:     "error",
		FilePath:  "logs/error.log",
		ToConsole: false,
		// FileEncoding不再支持: "json",
	}
	errorConfig := logconfig.New(errorOptions)
	// 注意：新的配置系统不支持动态设置，这些应该在创建配置时设置
	errorLogger, _ := New(errorConfig)

	// 使用不同的日志记录器
	appLogger.Info("应用启动")
	// 使用结构化日志代替多参数
	accessLogger.With("method", "GET", "path", "/api/users").Info("收到用户请求")
	errorLogger.With("error", "connection refused").Error("数据库连接失败")

	// 注意：日志记录器资源由DI容器自动管理，无需手动关闭
}
