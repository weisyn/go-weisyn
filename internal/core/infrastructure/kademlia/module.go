package kbucket

import (
	"context"

	"go.uber.org/fx"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	nodeiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/node"
)

// ============================================================================
//                              输入输出定义
// ============================================================================

// ModuleInput 定义K桶模块的输入依赖
type ModuleInput struct {
	fx.In

	Config kademlia.KBucketConfig `name:"kbucket_config"`
	Logger log.Logger             // 日志记录器（必需）
	Host   nodeiface.Host         `name:"node_host"` // 新增：用于WES节点验证
}

// ModuleOutput 定义K桶模块的输出
type ModuleOutput struct {
	fx.Out

	RoutingTableManager kademlia.RoutingTableManager `name:"routing_table_manager"`
	DistanceCalculator  kademlia.DistanceCalculator  `name:"distance_calculator"`
	PeerSelector        kademlia.PeerSelector        `name:"peer_selector"`
}

// ============================================================================
//                              主模块定义
// ============================================================================

// Module K桶模块
// 采用fx依赖注入模式，提供完整的Kademlia路由表功能
func Module() fx.Option {
	return fx.Module("kbucket",
		// === 配置提供 ===
		fx.Provide(fx.Annotate(
			ProvideKBucketConfig,
			fx.ResultTags(`name:"kbucket_config"`),
		)),

		// === 核心组件提供 ===
		fx.Provide(
			func(in ModuleInput) ModuleOutput {
				// 创建核心组件
				routingTableManager := NewRoutingTableManager(in.Config, in.Logger, in.Host)
				distanceCalculator := NewXORDistanceCalculator(in.Logger)
				peerSelector := NewKademliaPeerSelector(in.Logger)

				return ModuleOutput{
					RoutingTableManager: routingTableManager,
					DistanceCalculator:  distanceCalculator,
					PeerSelector:        peerSelector,
				}
			},
		),

		// === 生命周期管理 ===
		fx.Invoke(RegisterKBucketLifecycle),
	)
}

// LifecycleInput 定义生命周期管理的输入依赖
type LifecycleInput struct {
	fx.In

	Lifecycle           fx.Lifecycle
	RoutingTableManager kademlia.RoutingTableManager `name:"routing_table_manager"`
	Logger              log.Logger
	EventBus            event.EventBus `optional:"true"`                  // 事件总线，用于订阅peer连接事件
	NodeHost            nodeiface.Host `name:"node_host" optional:"true"` // Node Host，用于获取已连接peers进行全量导入
}

// RegisterKBucketLifecycle 注册K桶生命周期管理
func RegisterKBucketLifecycle(in LifecycleInput) {
	in.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			in.Logger.Info("🗂️  正在启动K桶路由表管理器...")

			// 使用类型断言调用具体实现的Start方法
			if manager, ok := in.RoutingTableManager.(*RoutingTableManager); ok {
				if err := manager.Start(ctx); err != nil {
					in.Logger.Errorf("启动K桶路由表管理器失败: %v", err)
					return err
				}

				// 全量导入已连接的peers到K桶（避免订阅时序问题）
				if in.NodeHost != nil {
					// 获取底层libp2p host
					libp2pHost := in.NodeHost.Libp2pHost()
					if libp2pHost != nil {
						connectedPeers := libp2pHost.Network().Peers()
						in.Logger.Infof("🔒 开始全量导入已连接peers到K桶（含WES过滤）: 共%d个peer", len(connectedPeers))

						importedCount := 0
						rejectedCount := 0
						for _, peerID := range connectedPeers {
							// 跳过自己
							if peerID == libp2pHost.ID() {
								continue
							}

							addrInfo := peer.AddrInfo{ID: peerID}
							// 调用AddPeer，内部会进行WES节点验证
							if added, err := manager.AddPeer(ctx, addrInfo); err != nil {
								in.Logger.Warnf("全量导入peer失败: %s, 错误: %v", peerID, err)
							} else if added {
								importedCount++
								in.Logger.Debugf("WES节点导入成功: %s", peerID)
							} else {
								rejectedCount++
								// AddPeer返回false通常表示外部节点被过滤
							}
						}
						in.Logger.Infof("🔒 全量导入完成: WES节点=%d, 外部节点已过滤=%d, 总计=%d",
							importedCount, rejectedCount, len(connectedPeers)-1)
					} else {
						in.Logger.Warn("🗂️  无法获取libp2p Host，跳过全量导入已连接peers")
					}
				} else {
					in.Logger.Warn("🗂️  NodeHost为nil，跳过全量导入已连接peers")
				}

				// 订阅peer连接事件，自动添加到路由表
				if in.EventBus != nil {
					peerConnectedHandler := func(ctx context.Context, data interface{}) error {
						if peerID, ok := data.(peer.ID); ok {
							in.Logger.Debugf("收到peer连接事件，添加到路由表: %s", peerID)

							// 创建AddrInfo（地址留空，因为我们主要关心路由表的peer ID）
							addrInfo := peer.AddrInfo{ID: peerID}

							// 添加到路由表
							if added, err := manager.AddPeer(ctx, addrInfo); err != nil {
								in.Logger.Warnf("添加peer到路由表失败: %s, 错误: %v", peerID, err)
							} else if added {
								in.Logger.Debugf("成功添加peer到路由表: %s", peerID)
							}
						}
						return nil
					}

					// 订阅peer连接事件
					if err := in.EventBus.Subscribe(event.EventTypeNetworkPeerConnected, peerConnectedHandler); err != nil {
						in.Logger.Warnf("订阅peer连接事件失败: %v", err)
					} else {
						in.Logger.Info("🗂️  已订阅peer连接事件，将自动维护路由表")
					}
				}
			}

			in.Logger.Info("🗂️  K桶路由表管理器已启动")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			in.Logger.Info("🗂️  正在关闭K桶路由表管理器...")

			// 使用类型断言调用具体实现的Stop方法
			if manager, ok := in.RoutingTableManager.(*RoutingTableManager); ok {
				if err := manager.Stop(ctx); err != nil {
					in.Logger.Errorf("关闭K桶路由表管理器失败: %v", err)
					return err
				}
			}

			in.Logger.Info("🗂️  K桶路由表管理器已关闭")
			return nil
		},
	})
}
