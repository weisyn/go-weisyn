// Package event 提供事件管理功能
package event

import (
	"go.uber.org/fx"

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
				serviceInput := ServiceInput{
					Provider:  input.Provider,
					Logger:    input.Logger,
					Lifecycle: input.Lifecycle,
				}

				serviceOutput, err := CreateEventServices(serviceInput)
				if err != nil {
					return ModuleOutput{}, err
				}

				return ModuleOutput{
					EventBus: serviceOutput.EventBus,
				}, nil
			},
		),
	)
}
