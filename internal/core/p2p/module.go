package p2p

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/fx"

	p2pcfg "github.com/weisyn/v1/internal/config/p2p"
	"github.com/weisyn/v1/internal/core/p2p/runtime"
	p2pservice "github.com/weisyn/v1/internal/core/p2p/service"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
)

// ModuleInput 定义 P2P 模块统一依赖
type ModuleInput struct {
	fx.In

	ConfigProvider config.Provider
	Logger         logiface.Logger `optional:"true"`
	EventBus       event.EventBus  `optional:"true"`
}

// ModuleOutput 定义 P2P 模块输出
type ModuleOutput struct {
	fx.Out

	P2PService        p2pi.Service        `name:"p2p_service"`
	P2PNetworkService p2pi.NetworkService `name:"p2p_network_service"`
	NodeRuntimeState  p2pi.RuntimeState   `name:"node_runtime_state"` // 节点运行时状态（由 P2P 模块管理）
}

// ProvideService 装配 P2P 运行时
func ProvideService(in ModuleInput) (ModuleOutput, error) {
	if in.ConfigProvider == nil {
		return ModuleOutput{}, fmt.Errorf("ConfigProvider is required for p2p module")
	}

	// 从配置提供者生成 P2P 配置
	opts, err := p2pcfg.NewFromChainConfig(in.ConfigProvider)
	if err != nil {
		return ModuleOutput{}, fmt.Errorf("failed to create p2p config: %w", err)
	}

	logger := in.Logger
	// Logger 可以为 nil，各组件会处理 nil logger 的情况
	rt, err := runtime.NewRuntimeWithConfig(opts, logger, in.EventBus, in.ConfigProvider)
	if err != nil {
		return ModuleOutput{}, fmt.Errorf("failed to create p2p runtime: %w", err)
	}

	// 在 DI 构造阶段预先初始化 Host，确保 Network 模块可以立即获取到非空 Host
	// 使用 Background Context 即可，真正的生命周期由 Fx Lifecycle 在 Start/Stop 中管理
	if err := rt.InitHost(context.Background()); err != nil {
		return ModuleOutput{}, fmt.Errorf("failed to init p2p host: %w", err)
	}

	host := rt.Host()
	if host == nil {
		return ModuleOutput{}, fmt.Errorf("p2p host is nil after InitHost")
	}

	networkSvc := p2pservice.NewNetworkService(host, logger)

	// 创建节点运行时状态实例（由 P2P 模块管理）
	nodeRuntimeState := NewRuntimeState(logger)

	// 注册 P2P 连接状态更新回调，自动更新 is_online 状态
	// 注意：这里需要从 runtime 获取连接状态更新事件
	// 暂时先创建实例，后续可以在 hookLifecycle 中注册回调

	return ModuleOutput{
		P2PService:        rt,
		P2PNetworkService: networkSvc,
		NodeRuntimeState:  nodeRuntimeState,
	}, nil
}

// Module 返回 P2P 模块（仅依赖注入与生命周期绑定）
func Module() fx.Option {
	return fx.Module("p2p",
		fx.Provide(ProvideService),
		// 绑定 P2P Runtime 的生命周期，使用命名依赖 `p2p_service`
		fx.Invoke(
			fx.Annotate(
				hookLifecycle,
				fx.ParamTags(``, `optional:"true"`, `name:"p2p_service"`, `name:"node_runtime_state"`),
			),
		),
	)
}

// hookLifecycle 绑定生命周期
func hookLifecycle(lc fx.Lifecycle, logger logiface.Logger, p2pSvc p2pi.Service, runtimeState p2pi.RuntimeState) {
	// Logger 可以为 nil，各组件会处理 nil logger 的情况

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if logger != nil {
				logger.Info("🚀 P2P runtime starting")
			}

			// 如果 runtime 需要 Start，可以在这里转型调用
			if starter, ok := p2pSvc.(interface{ Start(context.Context) error }); ok {
				if err := starter.Start(ctx); err != nil {
					return err
				}
			}

			// 启动连接状态监控 goroutine，定期更新 is_online 状态
			if runtimeState != nil {
				go monitorConnectionStatus(ctx, p2pSvc, runtimeState, logger)
			}

			return nil
		},
		OnStop: func(ctx context.Context) error {
			if logger != nil {
				logger.Info("🛑 P2P runtime stopping")
			}

			// 在停止时设置 is_online 为 false
			if runtimeState != nil {
				runtimeState.SetIsOnline(false)
			}

			if stopper, ok := p2pSvc.(interface{ Stop(context.Context) error }); ok {
				return stopper.Stop(ctx)
			}

			return nil
		},
	})
}

// monitorConnectionStatus 监控连接状态并更新 RuntimeState
func monitorConnectionStatus(ctx context.Context, p2pSvc p2pi.Service, runtimeState p2pi.RuntimeState, logger logiface.Logger) {
	ticker := time.NewTicker(5 * time.Second) // 每5秒检查一次
	defer ticker.Stop()

	// 初始检查延迟，等待 P2P 服务完全启动
	time.Sleep(2 * time.Second)

	for {
		select {
		case <-ctx.Done():
			if logger != nil {
				logger.Debug("P2P 连接状态监控已停止")
			}
			return
		case <-ticker.C:
			// 检查 Swarm 统计信息
			if swarm := p2pSvc.Swarm(); swarm != nil {
				stats := swarm.Stats()
				isOnline := stats.NumPeers > 0 // 至少有一个 peer 连接则认为在线

				// 更新 RuntimeState
				currentOnline := runtimeState.IsOnline()
				if currentOnline != isOnline {
					runtimeState.SetIsOnline(isOnline)
					if logger != nil {
						if isOnline {
							logger.Infof("P2P 节点已上线 (peers=%d)", stats.NumPeers)
						} else {
							logger.Infof("P2P 节点已下线 (peers=0)")
						}
					}
				}
			}
		}
	}
}
