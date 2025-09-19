package discovery

import (
	"context"

	eventiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/libp2p/go-libp2p/core/peer"
)

// 事件桥接（轻量）：
// - 仅在 discovery 内部把重要网络事件转发到通用 EventBus；
// - 不引入业务耦合；
// - 保持接口最小化，便于后期替换。

// emitPeerConnected 发送已连接事件（轻量桥接）
func emitPeerConnected(bus eventiface.EventBus, id peer.ID) {
	if bus == nil {
		return
	}
	// 事件处理器期望 (context.Context, interface{}) 参数
	bus.Publish(eventiface.EventTypeNetworkPeerConnected, context.Background(), id)
}

// emitPeerDisconnected 发送断开事件
func emitPeerDisconnected(bus eventiface.EventBus, id peer.ID) {
	if bus == nil {
		return
	}
	// 事件处理器期望 (context.Context, interface{}) 参数
	bus.Publish(eventiface.EventTypeNetworkPeerDisconnected, context.Background(), id)
}
