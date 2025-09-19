package network

import (
	"context"

	"github.com/weisyn/v1/pkg/constants/protocols"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	netiface "github.com/weisyn/v1/pkg/interfaces/network"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

// 订阅协议常量已迁移至 protocols.go 统一管理
// 使用 protocols.go 中定义的主题常量，与Proto定义严格对齐

// TxAnnounceRouter 交易公告路由器接口——主要传播路径
// 由 transaction/network/handler.go 提供具体实现，基于pb/network/protocol/transaction.proto
// 注意：只有全节点才需要订阅和处理交易公告
type TxAnnounceRouter interface {
	// HandleTransactionAnnounce 交易广播通告处理（主要传播路径）
	// 输入: TransactionAnnouncement (序列化后的字节数组)
	// 特性: GossipSub订阅模式，fire-and-forget全网交易广播
	HandleTransactionAnnounce(ctx context.Context, from peer.ID, topic string, data []byte) error
}

// RegisterSubscribeHandlers 注册订阅式协议处理器
// 纯粹的integration层：仅负责订阅注册和路由转发，实现双重保障传播的主要路径
func RegisterSubscribeHandlers(
	network netiface.Network,
	txRouter TxAnnounceRouter,
	logger log.Logger,
) error {
	if network == nil {
		if logger != nil {
			logger.Warn("网络服务未提供，跳过订阅协议注册")
		}
		return nil
	}

	// 交易广播通告订阅（主要传播路径） - 转发给transaction/network/handler.go
	if txRouter != nil {
		// 注册交易广播通告订阅处理器，实现GossipSub主要传播路径
		if _, err := network.Subscribe(protocols.TopicTransactionAnnounce, func(ctx context.Context, from peer.ID, topic string, data []byte) error {
			if logger != nil {
				logger.Debugf("💰 [交易集成] 接收交易广播通告: topic=%s, from=%s, size=%d", topic, from.String(), len(data))
			}
			return txRouter.HandleTransactionAnnounce(ctx, from, topic, data)
		}); err != nil {
			if logger != nil {
				logger.Errorf("❗ 交易广播通告主题订阅失败: %v", err)
			}
			return err
		}
		if logger != nil {
			logger.Infof("✅ 交易广播通告订阅成功: %s", protocols.TopicTransactionAnnounce)
		}
	} else {
		if logger != nil {
			logger.Info("交易路由器未提供，跳过交易广播订阅")
		}
	}

	if logger != nil {
		logger.Info("✅ 订阅式协议处理器注册完成：交易广播通告")
	}
	return nil
}
