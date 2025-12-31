// Package log 提供日志管理功能
package log

import (
	"fmt"

	logconfig "github.com/weisyn/v1/internal/config/log"
	"github.com/weisyn/v1/pkg/interfaces/config"
	logInterface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// ModuleParams 定义日志模块的依赖参数
type ModuleParams struct {
	fx.In

	Provider config.Provider // 配置提供者
}

// ModuleOutput 定义日志模块的输出结构
type ModuleOutput struct {
	fx.Out

	Logger    logInterface.Logger // 日志记录器接口
	ZapLogger *zap.Logger         // zap.Logger 具体类型（供需要 zap 特性的模块使用）
}

// Module 返回日志模块
func Module() fx.Option {
	return fx.Module("log",
		// 提供日志服务
		fx.Provide(ProvideServices),
	)
}

// ProvideServices 提供日志服务
// 根据配置初始化日志记录器并返回
func ProvideServices(params ModuleParams) (ModuleOutput, error) {
	// 根据配置提供者创建日志配置
	userLogConfig := logconfig.NewFromProvider(params.Provider)

	// 用用户配置创建新的日志记录器
	logger, err := New(userLogConfig)
	if err != nil {
		return ModuleOutput{}, fmt.Errorf("根据用户配置创建日志记录器失败: %w", err)
	}

	// 设置为全局记录器，替换掉init()时用默认配置创建的日志器
	SetLogger(logger)

	// 类型断言获取具体的 Logger 实例，以便访问内部的 *zap.Logger
	var zapLogger *zap.Logger
	if concreteLogger, ok := logger.(*Logger); ok {
		zapLogger = concreteLogger.zapLogger
	} else {
		return ModuleOutput{}, fmt.Errorf("logger 类型断言失败，无法获取 *zap.Logger")
	}

	// 返回日志输出
	return ModuleOutput{
		Logger:    logger,
		ZapLogger: zapLogger,
	}, nil
}

// WithModule 为 logger 添加 module 字段
// 这是一个辅助函数，帮助各模块创建带 module 标识的 logger
//
// 参数：
//   - logger: 基础 logger（可以是 logInterface.Logger 或 *zap.Logger）
//   - module: 模块名称（如 "api", "p2p", "consensus" 等）
//
// 返回：
//   - logInterface.Logger: 带 module 字段的 logger（如果输入是 logInterface.Logger）
//   - *zap.Logger: 带 module 字段的 zap logger（如果输入是 *zap.Logger）
func WithModule(logger interface{}, module string) interface{} {
	switch l := logger.(type) {
	case logInterface.Logger:
		return l.With("module", module)
	case *zap.Logger:
		return l.With(zap.String("module", module))
	default:
		// 如果类型不匹配，返回原 logger
		return logger
	}
}

// NewModuleLogger 创建带 module 字段的 logger
// 这是一个便捷函数，用于在模块中创建带标识的 logger
//
// 参数：
//   - baseLogger: 基础 logger
//   - module: 模块名称
//
// 返回：
//   - logInterface.Logger: 带 module 字段的 logger
func NewModuleLogger(baseLogger logInterface.Logger, module string) logInterface.Logger {
	if baseLogger == nil {
		return nil
	}
	return baseLogger.With("module", module)
}

// NewModuleZapLogger 创建带 module 字段的 zap logger
// 这是一个便捷函数，用于在模块中创建带标识的 zap logger
//
// 参数：
//   - baseLogger: 基础 zap logger
//   - module: 模块名称
//
// 返回：
//   - *zap.Logger: 带 module 字段的 zap logger
func NewModuleZapLogger(baseLogger *zap.Logger, module string) *zap.Logger {
	if baseLogger == nil {
		return nil
	}
	return baseLogger.With(zap.String("module", module))
}
