package discovery

import (
	"context"
	"time"

	eventiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
)

// controls 封装与外部信号的轻量交互：
// - 订阅网络质量等 Hint，触发一次短促发现；
// - 避免复杂控制面逻辑进入 discovery，保持边界清晰。

// subscribeHints 订阅业务Hint并触发一次短促发现
func (r *Runtime) subscribeHints(ctx context.Context, bus eventiface.EventBus, peers []string) {
	if bus == nil || r == nil || r.hostHandle == nil {
		return
	}
	if r.log != nil {
		r.log.Infof("p2p.discovery.hints subscribe event=%s peers=%d", eventiface.EventTypeNetworkQualityChanged, len(peers))
	}
	_ = bus.Subscribe(eventiface.EventTypeNetworkQualityChanged, func(_ eventiface.Event) error {
		if r.log != nil {
			r.log.Debugf("p2p.discovery.hints trigger event=%s", eventiface.EventTypeNetworkQualityChanged)
		}
		go func() {
			host := r.hostHandle.Host()
			if host == nil {
				if r.log != nil {
					r.log.Warnf("p2p.discovery.hints host=nil, skip")
				}
				return
			}
			// 轻量短促尝试：最多一次退避
			if ok, _ := r.tryDialOnce(ctx, peers, host); !ok {
				if r.log != nil {
					r.log.Debugf("p2p.discovery.hints first_try_failed, retry_after=2s")
				}
				time.Sleep(2 * time.Second)
				_, _ = r.tryDialOnce(ctx, peers, host)
			}
		}()
		return nil
	})
}
