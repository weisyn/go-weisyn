// Package interfaces 定义 chain 模块的内部接口
//
// 🎯 **设计理念**：
// - 继承公共接口，确保 API 一致性
// - 扩展集成层接口，支持网络和事件适配
// - 提供内部管理方法，支持系统内部协调
package interfaces

import (
	"context"

	peer "github.com/libp2p/go-libp2p/core/peer"
	chainif "github.com/weisyn/v1/pkg/interfaces/chain"
	"github.com/weisyn/v1/pkg/types"
)

// InternalSyncService 内部同步服务接口
//
// 🎯 **扩展公共同步服务**
//
// 继承公共接口，添加网络协议处理和事件订阅能力。
// 注意：为了避免循环依赖，这里直接定义接口方法，而不是导入integration包。
//
// 接口组合：
// - 完全继承公共同步服务接口的所有方法
// - 添加网络协议处理方法（对应integration/network.SyncProtocolRouter）
// - 添加事件订阅处理方法（对应integration/event.SyncEventSubscriber）
//
// 🔗 **使用场景**：
// - 模块内部：sync子模块实现此接口
// - 依赖注入：通过fx框架注入到其他模块
// - 网络注册：通过integration/network注册网络协议处理器
// - 事件订阅：通过integration/event注册事件订阅
//
// 📋 **实现要求**：
// - 必须实现chainif.SystemSyncService的所有方法
// - 必须实现网络协议处理方法（对应SyncProtocolRouter接口）
// - 必须实现事件订阅处理方法（对应SyncEventSubscriber接口）
type InternalSyncService interface {
	// 继承公共接口（在chain包内）
	chainif.SystemSyncService

	// ==================== 网络协议处理方法 ====================
	// 对应 integration/network.SyncProtocolRouter 接口

	// HandleKBucketSync K-bucket智能同步请求处理
	HandleKBucketSync(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error)

	// HandleRangePaginated 智能分页区块范围同步处理
	HandleRangePaginated(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error)

	// HandleSyncHelloV2 Sync v2 握手：判定链关系与共同祖先
	HandleSyncHelloV2(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error)

	// HandleSyncBlocksV2 Sync v2 区块批量同步：按范围返回 blocks
	HandleSyncBlocksV2(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error)

	// ==================== 事件订阅处理方法 ====================
	// 对应 integration/event.SyncEventSubscriber 接口

	// HandleForkDetected 处理分叉检测事件
	HandleForkDetected(eventData *types.ForkDetectedEventData) error

	// HandleForkProcessing 处理分叉处理中事件
	HandleForkProcessing(eventData *types.ForkProcessingEventData) error

	// HandleForkCompleted 处理分叉完成事件
	HandleForkCompleted(eventData *types.ForkCompletedEventData) error

	// HandleNetworkQualityChanged 处理网络质量变化事件
	HandleNetworkQualityChanged(eventData *types.NetworkQualityChangedEventData) error
}

// 编译时检查接口实现
var _ chainif.SystemSyncService = (InternalSyncService)(nil)
