package keepalive

import (
	"context"
	"time"

	"go.uber.org/fx"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/weisyn/v1/internal/core/p2p/discovery"
	p2pcfg "github.com/weisyn/v1/internal/config/p2p"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/p2p"
)

// ModuleInput 定义KeyPeerMonitor模块的输入依赖
type ModuleInput struct {
	fx.In

	Lifecycle      fx.Lifecycle
	Host           host.Host                  `optional:"true"` // libp2p host
	Routing        p2p.Routing                `optional:"true"` // Routing service
	Discovery      p2p.Discovery              `optional:"true"` // Discovery service
	P2PConfig      *p2pcfg.Options            `optional:"true"` // P2P配置
	Logger         log.Logger                 `optional:"true"` // 日志记录器
	EventBus       event.EventBus             `optional:"true"` // 事件总线
}

// ModuleOutput 定义KeyPeerMonitor模块的输出
type ModuleOutput struct {
	fx.Out

	KeyPeerMonitor *KeyPeerMonitor `name:"key_peer_monitor" optional:"true"`
	KeyPeerSet     *KeyPeerSet     `name:"key_peer_set" optional:"true"`
}

// Module KeyPeerMonitor fx模块
func Module() fx.Option {
	return fx.Module("keepalive",
		fx.Provide(
			func(in ModuleInput) ModuleOutput {
				// 检查是否启用KeyPeerMonitor
				if in.P2PConfig == nil || !in.P2PConfig.EnableKeyPeerMonitor {
					if in.Logger != nil {
						in.Logger.Debug("KeyPeerMonitor已禁用，跳过初始化")
					}
					return ModuleOutput{}
				}

				// 检查必需依赖
				if in.Host == nil {
					if in.Logger != nil {
						in.Logger.Warn("KeyPeerMonitor初始化失败：缺少libp2p host")
					}
					return ModuleOutput{}
				}

				// 创建KeyPeerSet
				keyPeerSet := NewKeyPeerSet(
					in.P2PConfig.KeyPeerSetMaxSize,
					10*time.Minute, // usefulWindow
				)

				// 获取AddrManager（从Discovery service）
				var addrManager *discovery.AddrManager
				if in.Discovery != nil {
					if _, ok := in.Discovery.(*discovery.Service); ok {
						// 通过反射或类型断言获取addrManager
						// 注意：这需要discovery.Service暴露GetAddrManager方法
						// 暂时设置为nil，实际使用时需要discovery提供访问接口
						addrManager = nil
					}
				}

				// 创建KeyPeerMonitor
				monitor := NewKeyPeerMonitor(
					in.Host,
					in.Routing,
					addrManager,
					keyPeerSet,
					in.Logger,
					in.EventBus,
					in.P2PConfig.KeyPeerProbeInterval,
					in.P2PConfig.PerPeerMinProbeInterval,
					in.P2PConfig.ProbeTimeout,
					in.P2PConfig.ProbeFailThreshold,
					in.P2PConfig.ProbeMaxConcurrent,
				)

				return ModuleOutput{
					KeyPeerMonitor: monitor,
					KeyPeerSet:     keyPeerSet,
				}
			},
		),
		fx.Invoke(RegisterLifecycle),
	)
}

// LifecycleInput 生命周期管理输入
type LifecycleInput struct {
	fx.In

	Lifecycle      fx.Lifecycle
	KeyPeerMonitor *KeyPeerMonitor `name:"key_peer_monitor" optional:"true"`
	KeyPeerSet     *KeyPeerSet     `name:"key_peer_set" optional:"true"`
	P2PConfig      *p2pcfg.Options `optional:"true"`
	Logger         log.Logger      `optional:"true"`
}

// RegisterLifecycle 注册KeyPeerMonitor生命周期
func RegisterLifecycle(in LifecycleInput) {
	if in.KeyPeerMonitor == nil {
		if in.Logger != nil {
			in.Logger.Debug("KeyPeerMonitor未初始化，跳过生命周期注册")
		}
		return
	}

	in.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if in.Logger != nil {
				in.Logger.Info("🚀 正在启动KeyPeerMonitor...")
			}

			// TODO: 从配置中获取bootstrap peers并设置到KeyPeerSet
			if in.P2PConfig != nil && len(in.P2PConfig.BootstrapPeers) > 0 && in.KeyPeerSet != nil {
				// 解析bootstrap peer IDs
				// 注意：需要将string转换为peer.ID
				// bootstrapPeerIDs := parseBootstrapPeers(in.P2PConfig.BootstrapPeers)
				// in.KeyPeerSet.SetBootstrapPeers(bootstrapPeerIDs)
			}

			if err := in.KeyPeerMonitor.Start(); err != nil {
				if in.Logger != nil {
					in.Logger.Errorf("启动KeyPeerMonitor失败: %v", err)
				}
				return err
			}

			if in.Logger != nil {
				in.Logger.Info("✅ KeyPeerMonitor已启动")
			}
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if in.Logger != nil {
				in.Logger.Info("🛑 正在停止KeyPeerMonitor...")
			}

			if err := in.KeyPeerMonitor.Stop(); err != nil {
				if in.Logger != nil {
					in.Logger.Errorf("停止KeyPeerMonitor失败: %v", err)
				}
				return err
			}

			if in.Logger != nil {
				in.Logger.Info("✅ KeyPeerMonitor已停止")
			}
			return nil
		},
	})
}

