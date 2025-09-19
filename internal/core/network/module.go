// Package network 提供WES系统的网络服务层统一管理
//
// 🌐 **网络服务层 (Network Service Layer)**
//
// 本模块是WES七层架构中的第二层：网络服务层，负责：
// - 统一网络服务：整合协议、路由、传输和统一网络服务
// - 高级网络功能：消息路由、去重、流量控制、负载均衡
// - 网络协议管理：处理多种网络协议和消息类型
// - 为应用层和内存池层提供网络服务
//
// 🎯 **设计原则**
// - 统一管理：将所有网络服务相关组件统一管理
// - 层次清晰：严格遵循七层架构的层级关系
// - 接口标准：统一使用 pkg/interfaces/network 标准接口
// - 依赖注入：通过fx框架输出接口，内部自管理生命周期；不在接口暴露Start/Stop
// - 高内聚低耦合：遵循依赖倒置原则，与 P2P 边界清晰（仅消费 P2P Host）
//
// 说明：本目录提供 Fx Module 绑定关系与实现骨架；具体实现位于 internal/core/network/impl/*
package network

import (
	"context"
	"fmt"

	"go.uber.org/fx"
	"go.uber.org/zap"

	networkconfig "github.com/weisyn/v1/internal/config/network"
	impl "github.com/weisyn/v1/internal/core/network/impl"
	"github.com/weisyn/v1/pkg/interfaces/config"
	cryptoi "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	nodeiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
	iface "github.com/weisyn/v1/pkg/interfaces/network"
)

// ModuleParams Network 模块依赖
type ModuleParams struct {
	fx.In

	// ========== 配置依赖 ==========
	ConfigProvider config.Provider `optional:"false"` // 配置提供者

	// ========== 基础设施依赖 ==========
	Host        nodeiface.Host           `name:"node_host"` // P2P主机服务
	Logger      logiface.Logger          `optional:"true"`  // 日志记录器
	EventBus    event.EventBus           `optional:"true"`  // 事件总线
	HashManager cryptoi.HashManager      `optional:"true"`  // 哈希管理器
	SigManager  cryptoi.SignatureManager `optional:"true"`  // 签名管理器
}

// ModuleOutput Network 模块输出
type ModuleOutput struct {
	fx.Out

	// ========== 对外公共接口 ==========
	NetworkService iface.Network `name:"network_service"` // 统一的网络服务接口
}

// Module 返回统一的网络模块
func Module() fx.Option {
	return fx.Module("network",
		// 提供网络服务
		fx.Provide(ProvideServices),

		// 生命周期管理
		fx.Invoke(
			fx.Annotate(
				func(lc fx.Lifecycle, logger logiface.Logger) {
					// 处理可选Logger
					if logger == nil {
						logger = &noopLogger{}
					}
					lc.Append(fx.Hook{
						OnStart: func(ctx context.Context) error {
							logger.Info("🌐 网络模块启动")
							return nil
						},
						OnStop: func(ctx context.Context) error {
							logger.Info("🌐 网络模块停止")
							return nil
						},
					})
				},
				fx.ParamTags(``, `optional:"true"`),
			),
		),
	)
}

// ProvideServices 提供网络服务
func ProvideServices(params ModuleParams) (ModuleOutput, error) {
	// 初始化Logger（处理可选Logger）
	var logger logiface.Logger
	if params.Logger != nil {
		logger = params.Logger
	} else {
		// 创建no-op logger作为回退
		logger = &noopLogger{}
	}

	// 验证必需的依赖
	if params.ConfigProvider == nil {
		return ModuleOutput{}, fmt.Errorf("配置提供者不能为空")
	}
	if params.Host == nil {
		return ModuleOutput{}, fmt.Errorf("P2P主机服务不能为空")
	}

	// 获取配置 - 配置提供者已经返回了完整的配置选项
	networkOptions := params.ConfigProvider.GetNetwork()
	if networkOptions == nil {
		return ModuleOutput{}, fmt.Errorf("网络配置不能为空")
	}

	// 创建网络配置实例（使用获取到的配置）
	networkConfig := networkconfig.New(networkOptions)

	// 创建网络门面实例
	facade := impl.NewFacade(
		params.Host,
		logger,
		networkConfig,
		params.HashManager,
		params.SigManager,
	)

	logger.Info("网络模块创建完成，等待Host启动后初始化GossipSub")

	// 🔧 监听Host启动事件并初始化GossipSub
	if params.EventBus != nil {
		logger.Info("开始订阅Host启动事件")

		// 定义事件处理器
		eventHandler := func(args ...interface{}) {
			logger.Info("收到Host启动事件，初始化GossipSub")
			facade.ForceInitializeGossipSub()
		}

		// 订阅Host启动事件
		if err := params.EventBus.Subscribe(event.EventTypeHostStarted, eventHandler); err != nil {
			logger.Errorf("订阅Host启动事件失败: %v", err)

		} else {
			logger.Info("Host启动事件订阅成功")

		}
	} else {
		logger.Warn("事件总线不可用，使用超时机制")

	}

	return ModuleOutput{
		NetworkService: facade,
	}, nil
}

// noopLogger 是一个无操作的Logger实现，用于可选Logger为nil时的回退
type noopLogger struct{}

func (l *noopLogger) Debug(msg string)                            {}
func (l *noopLogger) Debugf(format string, args ...interface{})   {}
func (l *noopLogger) Info(msg string)                             {}
func (l *noopLogger) Infof(format string, args ...interface{})    {}
func (l *noopLogger) Warn(msg string)                             {}
func (l *noopLogger) Warnf(format string, args ...interface{})    {}
func (l *noopLogger) Error(msg string)                            {}
func (l *noopLogger) Errorf(format string, args ...interface{})   {}
func (l *noopLogger) Fatal(msg string)                            {}
func (l *noopLogger) Fatalf(format string, args ...interface{})   {}
func (l *noopLogger) With(keyvals ...interface{}) logiface.Logger { return l }
func (l *noopLogger) Sync() error                                 { return nil }
func (l *noopLogger) GetZapLogger() *zap.Logger                   { return nil }
