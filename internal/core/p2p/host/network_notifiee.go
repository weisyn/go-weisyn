package host

import (
	"context"

	"github.com/libp2p/go-libp2p/core/network"
	ma "github.com/multiformats/go-multiaddr"
	eventiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// networkEventNotifiee 实现network.Notifiee接口，将网络事件转发到事件总线
// 主要用于监听节点连接和断连事件，供K桶管理器和其他组件订阅
type networkEventNotifiee struct {
	eventBus eventiface.EventBus
	logger   logiface.Logger
}

// newNetworkEventNotifiee 创建网络事件通知器
func newNetworkEventNotifiee(eventBus eventiface.EventBus, logger logiface.Logger) *networkEventNotifiee {
	return &networkEventNotifiee{
		eventBus: eventBus,
		logger:   logger,
	}
}

// Listen 监听地址变化（不处理）
func (n *networkEventNotifiee) Listen(_ network.Network, _ ma.Multiaddr) {}

// ListenClose 监听地址关闭（不处理）
func (n *networkEventNotifiee) ListenClose(_ network.Network, _ ma.Multiaddr) {}

// Connected 处理节点连接事件
func (n *networkEventNotifiee) Connected(_ network.Network, conn network.Conn) {
	peerID := conn.RemotePeer()

	if n.logger != nil {
		n.logger.Debugf("节点连接事件: %s, 方向=%s", peerID, conn.Stat().Direction)
	}

	// 发布连接事件到事件总线
	if n.eventBus != nil {
		// 发布连接事件（与断连事件保持一致）
		n.eventBus.Publish(eventiface.EventTypeNetworkPeerConnected, context.Background(), peerID)
		if n.logger != nil {
			n.logger.Debugf("📡 已发布节点连接事件: %s", peerID)
		}
	}
}

// Disconnected 处理节点断连事件
func (n *networkEventNotifiee) Disconnected(_ network.Network, conn network.Conn) {
	peerID := conn.RemotePeer()

	if n.logger != nil {
		// 降级为 Debug，避免在默认 info 级别刷屏
		n.logger.Debugf("节点断连事件: %s, 方向=%s", peerID, conn.Stat().Direction)
	}

	// 发布断连事件到事件总线
	// 注意：EventBus订阅者期望 func(ctx context.Context, data interface{}) error 签名
	// 所以Publish需要传递两个参数：context和data
	if n.eventBus != nil {
		n.eventBus.Publish(eventiface.EventTypeNetworkPeerDisconnected, context.Background(), peerID)
		if n.logger != nil {
			// 事件发布日志也降级为 Debug
			n.logger.Debugf("📡 已发布节点断连事件: %s", peerID)
		}
	}
}

// OpenedStream 处理流打开事件（不处理）
func (n *networkEventNotifiee) OpenedStream(_ network.Network, _ network.Stream) {}

// ClosedStream 处理流关闭事件（不处理）
func (n *networkEventNotifiee) ClosedStream(_ network.Network, _ network.Stream) {}

// RegisterNetworkEventNotifiee 注册网络事件通知器到libp2p host
// 应在host启动后调用
func RegisterNetworkEventNotifiee(h network.Network, eventBus eventiface.EventBus, logger logiface.Logger) {
	if h == nil {
		if logger != nil {
			logger.Warn("无法注册网络事件通知器：host为nil")
		}
		return
	}

	notifiee := newNetworkEventNotifiee(eventBus, logger)
	h.Notify(notifiee)

	if logger != nil {
		logger.Info("✅ 已注册网络事件通知器（监听连接和断连）")
	}
}

