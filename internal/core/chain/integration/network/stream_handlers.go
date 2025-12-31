package network

import (
	"context"

	peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/weisyn/v1/pkg/constants/protocols"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	netiface "github.com/weisyn/v1/pkg/interfaces/network"
)

// SyncProtocolRouter 同步协议路由器接口
//
// 🎯 **同步模块网络协议处理**：
// sync子模块专门处理与区块同步相关的网络协议：
// - K-bucket智能同步协议
// - 智能分页区块范围同步协议
//
// 由 sync/network_handler 包提供具体实现，基于pb/network/protocol/sync.proto
type SyncProtocolRouter interface {
	// HandleKBucketSync K-bucket智能同步请求处理
	// 输入: KBucketSyncRequest (序列化后的字节数组)
	// 输出: IntelligentPaginationResponse (序列化后的字节数组)
	HandleKBucketSync(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error)

	// HandleRangePaginated 智能分页区块范围同步处理
	// 输入: KBucketSyncRequest (序列化后的字节数组)
	// 输出: IntelligentPaginationResponse (序列化后的字节数组)
	HandleRangePaginated(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error)

	// HandleSyncHelloV2 Sync v2 握手：判定链关系与共同祖先
	// 输入: SyncHelloV2Request (序列化后的字节数组)
	// 输出: SyncHelloV2Response (序列化后的字节数组)
	HandleSyncHelloV2(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error)

	// HandleSyncBlocksV2 Sync v2 区块批量同步：按范围返回 blocks
	// 输入: SyncBlocksV2Request (序列化后的字节数组)
	// 输出: SyncBlocksV2Response (序列化后的字节数组)
	HandleSyncBlocksV2(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error)
}

// RegisterSyncStreamHandlers 注册区块同步流式协议处理器
//
// 🎯 **纯粹的integration层**：
// 仅负责协议注册和路由转发，基于Proto定义。
// 具体业务逻辑由 sync/network_handler 实现。
//
// 参数：
//   - network: 网络服务接口
//   - router: 同步协议路由器（实现SyncProtocolRouter接口）
//   - logger: 日志服务（可选）
//
// 返回：
//   - error: 注册失败时返回错误
func RegisterSyncStreamHandlers(
	network netiface.Network,
	router SyncProtocolRouter,
	logger log.Logger,
) error {
	if network == nil || router == nil {
		if logger != nil {
			logger.Warn("同步协议路由器未提供，跳过注册")
		}
		return nil
	}

	// 1) K-bucket智能同步协议 - 转发给sync/network_handler
	if err := network.RegisterStreamHandler(protocols.ProtocolKBucketSync, func(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
		if logger != nil {
			logger.Debugf("📚 [同步集成] 接收K-bucket同步请求，来自: %s, 数据大小: %d字节", from, len(reqBytes))
		}
		return router.HandleKBucketSync(ctx, from, reqBytes)
	}); err != nil {
		if logger != nil {
			logger.Errorf("❗ K-bucket同步协议注册失败: %v", err)
		}
		return err
	}

	// 2) 智能分页范围同步协议 - 转发给sync/network_handler
	if err := network.RegisterStreamHandler(protocols.ProtocolRangePaginated, func(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
		if logger != nil {
			logger.Debugf("📚 [同步集成] 接收分页范围同步请求，来自: %s, 数据大小: %d字节", from, len(reqBytes))
		}
		return router.HandleRangePaginated(ctx, from, reqBytes)
	}); err != nil {
		if logger != nil {
			logger.Errorf("❗ 分页范围同步协议注册失败: %v", err)
		}
		return err
	}

	// 3) Sync v2：握手协议 - 转发给sync/network_handler
	if err := network.RegisterStreamHandler(protocols.ProtocolSyncHelloV2, func(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
		if logger != nil {
			logger.Debugf("📚 [同步集成] 接收SyncHelloV2请求，来自: %s, 数据大小: %d字节", from, len(reqBytes))
		}
		return router.HandleSyncHelloV2(ctx, from, reqBytes)
	}); err != nil {
		if logger != nil {
			logger.Errorf("❗ SyncHelloV2协议注册失败: %v", err)
		}
		return err
	}

	// 4) Sync v2：区块批量同步协议 - 转发给sync/network_handler
	if err := network.RegisterStreamHandler(protocols.ProtocolSyncBlocksV2, func(ctx context.Context, from peer.ID, reqBytes []byte) ([]byte, error) {
		if logger != nil {
			logger.Debugf("📚 [同步集成] 接收SyncBlocksV2请求，来自: %s, 数据大小: %d字节", from, len(reqBytes))
		}
		return router.HandleSyncBlocksV2(ctx, from, reqBytes)
	}); err != nil {
		if logger != nil {
			logger.Errorf("❗ SyncBlocksV2协议注册失败: %v", err)
		}
		return err
	}

	if logger != nil {
		logger.Info("✅ 区块同步流式协议处理器注册完成：K-bucket同步 + 分页范围同步 + Sync v2(hello/blocks)")
	}
	return nil
}
