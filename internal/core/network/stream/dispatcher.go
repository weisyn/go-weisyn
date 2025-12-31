package stream

import (
	"context"

	iface "github.com/weisyn/v1/pkg/interfaces/network"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

// dispatcher.go
// 入站流分发（协议ID→处理器）
// 能力：
// - 基于协议ID将入站消息分发给相应的 MessageHandler
// - 可选维护本地临时 handler（优先于 registry 查找）
// 依赖：
// - registry 的 handler 查找（通过回调注入，避免直接耦合）
// 非目标：
// - 不做消息编解码/签名验证；由 impl/internal 或 codec 处理

// HandlerLookup 回调：按协议ID查找已注册的处理器
// 返回：处理器与是否存在标记
type HandlerLookup func(protocol string) (iface.MessageHandler, bool)

// Dispatcher 入站分发器（方法框架）
// 说明：仅定义结构与方法签名；内部逻辑后续补充
type Dispatcher struct {
	lookup        HandlerLookup
	localHandlers map[string]iface.MessageHandler
}

// NewDispatcher 创建分发器
// 参数：
//   - lookup: 从 registry 查询 handler 的回调
func NewDispatcher(lookup HandlerLookup) *Dispatcher {
	return &Dispatcher{lookup: lookup, localHandlers: make(map[string]iface.MessageHandler)}
}

// AddLocalHandler 为指定协议添加本地临时处理器
// 说明：本地处理器优先于回调查找，用于测试/临时覆盖
func (d *Dispatcher) AddLocalHandler(protocol string, h iface.MessageHandler) error { return nil }

// RemoveLocalHandler 移除本地临时处理器
func (d *Dispatcher) RemoveLocalHandler(protocol string) error { return nil }

// Dispatch 分发入站消息到对应处理器
// 参数：
//   - ctx: 上下文（取消/超时）
//   - from: 发送方节点ID
//   - protocol: 协议ID（含版本）
//   - data: 原始消息字节
//
// 返回：
//   - error: 处理器错误或未找到处理器
//
// 说明：查找优先级为 本地临时处理器 → Registry 回调查找；若仍未命中，返回未注册协议占位错误或丢弃
func (d *Dispatcher) Dispatch(ctx context.Context, from peer.ID, protocol string, data []byte) error {
	return nil
}
