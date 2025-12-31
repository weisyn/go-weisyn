package event

import (
	"context"
	"fmt"

	evbus "github.com/asaskevich/EventBus"
	eventconfig "github.com/weisyn/v1/internal/config/event"
	"github.com/weisyn/v1/pkg/interfaces/config"
	eventInterface "github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"

	"go.uber.org/fx"
)

// ServiceInput 事件服务工厂函数的输入参数
type ServiceInput struct {
	Provider  config.Provider // 配置提供者
	Logger    log.Logger      // 日志记录器（可选）
	Lifecycle fx.Lifecycle    // 生命周期管理
}

// ServiceOutput 事件服务工厂函数的输出结果
type ServiceOutput struct {
	EventBus eventInterface.EventBus // 基础事件总线
}

// CreateEventServices 创建事件服务
func CreateEventServices(input ServiceInput) (ServiceOutput, error) {
	// 获取事件配置选项
	eventOptions := input.Provider.GetEvent()

	// 创建事件配置
	eventCfg := eventconfig.New(eventOptions)

	// 初始化基础事件总线
	eventBus := New(eventCfg)

	// 记录日志
	if input.Logger != nil {
		input.Logger.Info("基础事件总线已初始化")
	}

	return ServiceOutput{
		EventBus: eventBus,
	}, nil
}

// CreateDomainRegistry 创建域注册中心
func CreateDomainRegistry(input ServiceInput) (*DomainRegistry, error) {
	// 如果没有日志器，NewDomainRegistry会处理nil的情况
	registry := NewDomainRegistry(input.Logger)

	if input.Logger != nil {
		input.Logger.Info("域注册中心已初始化")
	}

	return registry, nil
}

// CreateEventRouter 创建事件路由器
func CreateEventRouter(input ServiceInput) (*EventRouter, error) {
	// 如果没有日志器，NewEventRouter会处理nil的情况
	router := NewEventRouter(input.Logger)

	if input.Logger != nil {
		input.Logger.Info("事件路由器已初始化")
	}

	return router, nil
}

// CreateEventValidator 创建事件验证器
func CreateEventValidator(input ServiceInput) (EventValidator, error) {
	// 如果没有日志器，NewBasicEventValidator会处理nil的情况
	validator := NewBasicEventValidator(input.Logger, DefaultValidatorConfig())

	if input.Logger != nil {
		input.Logger.Info("事件验证器已初始化")
	}

	return validator, nil
}

// CreateEventCoordinator 创建事件协调器
func CreateEventCoordinator(
	input ServiceInput,
	domainRegistry *DomainRegistry,
	eventRouter *EventRouter,
	eventValidator EventValidator,
	basicEventBus eventInterface.EventBus,
) (EventCoordinator, error) {
	// 从基础EventBus中获取底层bus
	var underlyingBus evbus.Bus
	if basicEB, ok := basicEventBus.(*EventBus); ok {
		underlyingBus = basicEB.bus
	} else {
		return nil, fmt.Errorf("unsupported EventBus type")
	}

	coordinator := NewBasicEventCoordinator(
		input.Logger,
		DefaultCoordinatorConfig(),
		domainRegistry,
		eventRouter,
		eventValidator,
		underlyingBus,
	)

	// 注册生命周期钩子
	input.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if input.Logger != nil {
				input.Logger.Info("启动事件协调器...")
			}
			return coordinator.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			if input.Logger != nil {
				input.Logger.Info("停止事件协调器...")
			}
			return coordinator.Stop()
		},
	})

	if input.Logger != nil {
		input.Logger.Info("事件协调器已初始化")
	}

	return coordinator, nil
}

// CreateEnhancedEventServices 创建增强事件服务
func CreateEnhancedEventServices(
	input ServiceInput,
	domainRegistry *DomainRegistry,
	eventRouter *EventRouter,
	eventValidator EventValidator,
	coordinator EventCoordinator,
) (*EnhancedEventBus, error) {
	// 创建增强事件总线
	enhanced, err := NewEnhanced(input.Logger, DefaultEnhancedEventBusConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create enhanced event bus: %w", err)
	}

	// 注册生命周期钩子
	input.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if input.Logger != nil {
				input.Logger.Info("启动增强事件总线...")
			}
			return enhanced.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			if input.Logger != nil {
				input.Logger.Info("停止增强事件总线...")
			}
			return enhanced.Stop(ctx)
		},
	})

	if input.Logger != nil {
		input.Logger.Info("增强事件总线已初始化")
	}

	return enhanced, nil
}
