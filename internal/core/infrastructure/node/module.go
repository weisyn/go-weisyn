package node

import (
	"context"

	"go.uber.org/fx"

	discpkg "github.com/weisyn/v1/internal/core/infrastructure/node/impl/discovery"
	hostpkg "github.com/weisyn/v1/internal/core/infrastructure/node/impl/host"
	cfgprovider "github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	eventiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	nodeiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	storageiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// ModuleParams 定义节点网络模块统一依赖
type ModuleParams struct {
	fx.In

	Provider cfgprovider.Provider  `optional:"true"`
	Logger   logiface.Logger       `optional:"true"`
	Event    eventiface.EventBus   `optional:"true"`
	Storage  storageiface.Provider `optional:"true"`
}

// ModuleOutput 定义节点网络模块输出（内部运行时句柄）
type ModuleOutput struct {
	fx.Out

	HostRuntime *hostpkg.Runtime
	DiscRuntime *discpkg.Runtime
	Host        nodeiface.Host `name:"node_host"`
}

// ProvideServices 装配 host 与 discovery 运行时
func ProvideServices(p ModuleParams) (ModuleOutput, error) {
	serviceInput := ServiceInput{
		Provider: p.Provider,
		Logger:   p.Logger,
		Event:    p.Event,
		Storage:  p.Storage,
	}

	serviceOutput, err := CreateNodeServices(serviceInput)
	if err != nil {
		return ModuleOutput{}, err
	}

	return ModuleOutput{
		HostRuntime: serviceOutput.HostRuntime,
		DiscRuntime: serviceOutput.DiscRuntime,
		Host:        serviceOutput.Host,
	}, nil
}

// Module 返回节点网络模块（仅依赖注入与生命周期绑定）
func Module() fx.Option {
	return fx.Module("node",
		fx.Provide(ProvideServices),
		fx.Invoke(
			// 绑定生命周期：先启 host，再启 discovery；停止反向。
			func(params struct {
				fx.In
				Lifecycle   fx.Lifecycle
				HostRuntime *hostpkg.Runtime
				DiscRuntime *discpkg.Runtime
				HostService nodeiface.Host `name:"node_host"`
				Logger      logiface.Logger
				EventBus    eventiface.EventBus `optional:"true"`
			}) {
				lc := params.Lifecycle
				hostRuntime := params.HostRuntime
				discRuntime := params.DiscRuntime
				hostService := params.HostService
				logger := params.Logger
				eventBus := params.EventBus
				// 创建长期运行的context，不受启动流程影响
				discCtx, discCancel := context.WithCancel(context.Background())

				lc.Append(fx.Hook{
					OnStart: func(ctx context.Context) error {
						if logger != nil {
							logger.Info("🌐 启动节点模块: host → discovery")
						}
						if err := hostRuntime.Start(ctx); err != nil {
							if logger != nil {
								logger.Errorf("节点 host 启动失败: %v", err)
							}
							return err
						}

						// Host启动完成后，注册延迟的协议处理器
						if logger != nil {
							logger.Info("Host启动完成，开始注册延迟的协议处理器")
						}
						hostService.RegisterPendingHandlers()

						// 发布Host启动完成事件，通知网络模块初始化GossipSub
						if eventBus != nil {
							eventBus.Publish(event.EventTypeHostStarted, map[string]interface{}{
								"host_id":   hostService.ID(),
								"addresses": hostService.AnnounceAddrs(),
							})
							if logger != nil {
								logger.Info("📢 发布Host启动完成事件")
							}
						}

						// 使用独立的长期上下文启动发现服务
						if err := discRuntime.Start(discCtx); err != nil {
							if logger != nil {
								logger.Errorf("节点 discovery 启动失败: %v", err)
							}
							return err
						}
						return nil
					},
					OnStop: func(ctx context.Context) error {
						if logger != nil {
							logger.Info("🛑 停止节点模块: discovery → host")
						}
						// 取消discovery的长期上下文
						discCancel()
						_ = discRuntime.Stop(ctx)
						_ = hostRuntime.Stop(ctx)
						return nil
					},
				})
			},
		),
	)
}
