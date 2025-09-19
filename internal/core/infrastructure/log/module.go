// Package log 提供日志管理功能
package log

import (
	"fmt"

	logconfig "github.com/weisyn/v1/internal/config/log"
	"github.com/weisyn/v1/pkg/interfaces/config"
	logInterface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"go.uber.org/fx"
)

// ModuleParams 定义日志模块的依赖参数
type ModuleParams struct {
	fx.In

	Provider config.Provider // 配置提供者
}

// ModuleOutput 定义日志模块的输出结构
type ModuleOutput struct {
	fx.Out

	Logger logInterface.Logger // 日志记录器
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

	// 返回日志输出
	return ModuleOutput{
		Logger: logger,
	}, nil
}
