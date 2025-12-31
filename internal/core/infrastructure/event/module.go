// Package event 提供事件管理功能
package event

import (
	"go.uber.org/fx"

	metricsiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/metrics"
	metricsutil "github.com/weisyn/v1/pkg/utils/metrics"
	"github.com/weisyn/v1/pkg/interfaces/config"
	eventInterface "github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ModuleInput 事件模块输入依赖
type ModuleInput struct {
	fx.In

	Provider  config.Provider // 配置提供者
	Logger    log.Logger      `optional:"true"` // 日志记录器（可选）
	Lifecycle fx.Lifecycle    // 生命周期管理
}

// ModuleOutput 事件模块输出服务
type ModuleOutput struct {
	fx.Out

	EventBus eventInterface.EventBus // 基础事件总线
}

// Module 返回事件模块
func Module() fx.Option {
	return fx.Module("event",
		fx.Provide(
			func(input ModuleInput) (ModuleOutput, error) {
				// 🎯 为事件模块添加 module 字段，日志将路由到 node-system.log
				var eventLogger log.Logger
				if input.Logger != nil {
					eventLogger = input.Logger.With("module", "event")
				}
				
				serviceInput := ServiceInput{
					Provider:  input.Provider,
					Logger:    eventLogger,
					Lifecycle: input.Lifecycle,
				}

				serviceOutput, err := CreateEventServices(serviceInput)
				if err != nil {
					return ModuleOutput{}, err
				}

				// 注册 EventBus 到内存监控系统
				if reporter, ok := serviceOutput.EventBus.(metricsiface.MemoryReporter); ok {
					metricsutil.RegisterMemoryReporter(reporter)
					if eventLogger != nil {
						eventLogger.Info("✅ EventBus 已注册到内存监控系统")
					}
				}

				return ModuleOutput{
					EventBus: serviceOutput.EventBus,
				}, nil
			},
		),
	)
}
