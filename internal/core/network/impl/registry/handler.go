package registry

import (
	"context"
	"time"

	iface "github.com/weisyn/v1/pkg/interfaces/network"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

// handler.go
// 处理器适配与包装（方法框架）：
// - 统一签名与异常恢复（panic recover）
// - 可选注入上下文元信息（如 correlationId）
// - 可选加入超时控制/拦截器链（如前置验证/后置统计）

// HandlerWrapper 对上层 handler 进行包装以增强健壮性
type HandlerWrapper struct {
	defaultTimeout time.Duration
}

// NewHandlerWrapper 创建处理器包装器
func NewHandlerWrapper() *HandlerWrapper { return &HandlerWrapper{defaultTimeout: 0} }

// WithTimeout 设置默认超时（占位）
func (w *HandlerWrapper) WithTimeout(d time.Duration) *HandlerWrapper { w.defaultTimeout = d; return w }

// Wrap 返回具有标准签名与保护（recover）的处理器函数
func (w *HandlerWrapper) Wrap(h iface.MessageHandler) iface.MessageHandler {
	return func(ctx context.Context, from peer.ID, req []byte) (resp []byte, err error) {
		defer func() {
			if r := recover(); r != nil {
				resp, err = nil, ctx.Err()
			}
		}()
		// 可选超时
		if w.defaultTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, w.defaultTimeout)
			defer cancel()
		}
		return h(ctx, from, req)
	}
}

// Invoke 统一的处理器调用入口（可用于内部调度）
func (w *HandlerWrapper) Invoke(ctx context.Context, h iface.MessageHandler, fromPeerID interface{}, protocol string, data []byte) error {
	pid, _ := fromPeerID.(peer.ID)
	_, err := h(ctx, pid, data)
	return err
}
