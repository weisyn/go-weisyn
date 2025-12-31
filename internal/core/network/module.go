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
// 说明：本目录提供 Fx Module 绑定关系与实现骨架；具体实现位于各个功能域目录（facade/, pubsub/, registry/ 等）
// Package network provides network communication functionality for P2P operations.
package network

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/fx"
	"go.uber.org/zap"

	p2pcfg "github.com/weisyn/v1/internal/config/p2p"
	networkconfig "github.com/weisyn/v1/internal/config/network"
	"github.com/weisyn/v1/internal/core/network/facade"
	"github.com/weisyn/v1/pkg/interfaces/config"
	cryptoi "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	iface "github.com/weisyn/v1/pkg/interfaces/network"
	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	metricsutil "github.com/weisyn/v1/pkg/utils/metrics"
	ma "github.com/multiformats/go-multiaddr"
	libpeer "github.com/libp2p/go-libp2p/core/peer"
)

// ModuleInput 定义 network 模块的输入依赖
//
// 🎯 **依赖组织**：
// 本结构体使用fx.In标签，通过依赖注入自动提供所有必需的组件依赖。
type ModuleInput struct {
	fx.In

	// ========== 配置依赖 ==========
	ConfigProvider config.Provider `optional:"false"` // 配置提供者

	// ========== 基础设施依赖 ==========
	P2P         p2pi.Service             `name:"p2p_service"` // P2P运行时服务
	Logger      logiface.Logger          `optional:"true"`    // 日志记录器
	EventBus    event.EventBus           `optional:"true"`    // 事件总线
	HashManager cryptoi.HashManager      `optional:"true"`    // 哈希管理器
	SigManager  cryptoi.SignatureManager `optional:"true"`    // 签名管理器
}

// ModuleOutput Network 模块输出
type ModuleOutput struct {
	fx.Out

	// ========== 对外公共接口（命名依赖）==========
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
				func(lc fx.Lifecycle, logger logiface.Logger, networkService iface.Network) {
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
							// 停止 Facade 及其安全组件
							if f, ok := networkService.(*facade.Facade); ok {
								f.Stop()
							}
							return nil
						},
					})
				},
				fx.ParamTags(``, `optional:"true"`, `name:"network_service"`),
			),
		),
	)
}

// ProvideServices 提供网络服务
func ProvideServices(input ModuleInput) (ModuleOutput, error) {
	// 初始化Logger（处理可选Logger）
	var logger logiface.Logger
	if input.Logger != nil {
		// 🎯 为网络模块添加 module 字段，日志将路由到 node-system.log
		logger = input.Logger.With("module", "network")
	} else {
		// 创建no-op logger作为回退
		logger = &noopLogger{}
	}

	// 验证必需的依赖
	if input.ConfigProvider == nil {
		return ModuleOutput{}, fmt.Errorf("配置提供者不能为空")
	}
	if input.P2P == nil {
		return ModuleOutput{}, fmt.Errorf("P2P运行时服务不能为空")
	}

	// 从 P2P Service 获取 Host
	libp2pHost := input.P2P.Host()
	if libp2pHost == nil {
		return ModuleOutput{}, fmt.Errorf("P2P Host 不能为空")
	}

	// 获取配置 - 配置提供者已经返回了完整的配置选项
	networkOptions := input.ConfigProvider.GetNetwork()
	if networkOptions == nil {
		return ModuleOutput{}, fmt.Errorf("网络配置不能为空")
	}

	// 创建网络配置实例（使用获取到的配置）
	networkConfig := networkconfig.New(networkOptions)

	// 获取网络命名空间（用于自动为协议 ID 和 Topic 添加 namespace）
	networkNamespace := input.ConfigProvider.GetNetworkNamespace()

	// 创建网络门面实例（直接使用 libp2p Host，并传入 namespace）
	f := facade.NewFacadeWithNamespace(
		libp2pHost,
		logger,
		networkConfig,
		input.HashManager,
		input.SigManager,
		networkNamespace,
	)

	// 注入 forceConnect 配置（从 P2P Runtime 读取 Options）
	if f != nil && input.P2P != nil {
		var opts *p2pcfg.Options
		if getter, ok := input.P2P.(interface{ Options() *p2pcfg.Options }); ok {
			opts = getter.Options()
		}
		if opts != nil {
			bizPeers := make([]libpeer.ID, 0, len(opts.BusinessCriticalPeerIDs))
			for _, s := range opts.BusinessCriticalPeerIDs {
				id, err := libpeer.Decode(strings.TrimSpace(s))
				if err == nil && id != "" {
					bizPeers = append(bizPeers, id)
				}
			}

			bootstrapPeers := make([]libpeer.ID, 0, len(opts.BootstrapPeers))
			for _, addrStr := range opts.BootstrapPeers {
				m, err := ma.NewMultiaddr(addrStr)
				if err != nil {
					continue
				}
				info, err := libpeer.AddrInfoFromP2pAddr(m)
				if err == nil && info != nil && info.ID != "" {
					bootstrapPeers = append(bootstrapPeers, info.ID)
				}
			}

			f.SetForceConnectConfig(facade.ForceConnectConfig{
				Enabled:           opts.ForceConnectEnabled,
				Cooldown:          opts.ForceConnectCooldown,
				Concurrency:       opts.ForceConnectConcurrency,
				BudgetPerRound:    opts.ForceConnectBudgetPerRound,
				Tier2SampleBudget: opts.ForceConnectTier2SampleBudget,
				Timeout:           opts.ForceConnectTimeout,
				BusinessPeers:     bizPeers,
				BootstrapPeers:    bootstrapPeers,
			})
		} else {
			// 没有拿到 opts，不阻断网络模块
			if logger != nil {
				logger.Debug("p2p options not available, skipping forceConnect config injection")
			}
		}
	}

	logger.Info("网络模块创建完成，等待Host启动后初始化GossipSub")

	// 🔧 监听Host启动事件并初始化GossipSub
	if input.EventBus != nil {
		logger.Info("开始订阅Host启动事件")

		// 定义事件处理器
		eventHandler := func(args ...interface{}) {
			logger.Info("收到Host启动事件，初始化GossipSub")
			f.ForceInitializeGossipSub()
		}

		// 订阅Host启动事件
		if err := input.EventBus.Subscribe(event.EventTypeHostStarted, eventHandler); err != nil {
			logger.Errorf("订阅Host启动事件失败: %v", err)

		} else {
			logger.Info("Host启动事件订阅成功")

		}
	} else {
		logger.Warn("事件总线不可用，使用超时机制")

	}

	// 注册 Network Facade 到内存监控系统
	if f != nil {
		metricsutil.RegisterMemoryReporter(f)
		if logger != nil {
			logger.Info("✅ Network Facade 已注册到内存监控系统")
		}
	}

	return ModuleOutput{
		NetworkService: f,
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
