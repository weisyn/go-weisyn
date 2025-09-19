package discovery

import (
	"context"
	"time"

	lphost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	mdns "github.com/libp2p/go-libp2p/p2p/discovery/mdns"

	nodeconfig "github.com/weisyn/v1/internal/config/node"
	eventiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// mdnsRuntime 封装 mDNS 发现：
// - 使用服务名（ServiceName）在局域网内广播/发现；
// - 发现邻居后在配置的超时/重试限制内进行直连尝试；
// - 成功后通过事件总线发布轻量事件。
type mdnsRuntime struct {
	cfg    *nodeconfig.NodeOptions
	logger logiface.Logger
	host   lphost.Host
	svc    mdns.Service
	events eventiface.EventBus
	// 轻量发现指标钩子（可选）
	onPeerFound func()
	onConnOK    func()
	onConnFail  func()
}

func newMDNSRuntime(cfg *nodeconfig.NodeOptions, logger logiface.Logger, h lphost.Host, eb eventiface.EventBus) *mdnsRuntime {
	return &mdnsRuntime{cfg: cfg, logger: logger, host: h, events: eb}
}

func (r *mdnsRuntime) Start(ctx context.Context) error {
	if r.host == nil {
		return nil
	}
	// 开关控制：未启用则直接返回并记录日志
	if r.cfg != nil && !r.cfg.Discovery.MDNS.Enabled {
		if r.logger != nil {
			r.logger.Infof("p2p.discovery.mdns disabled by config")
		}
		return nil
	}
	serviceName := "weisyn-node"
	if r.cfg != nil && r.cfg.Discovery.MDNS.ServiceName != "" {
		serviceName = r.cfg.Discovery.MDNS.ServiceName
	}
	r.svc = mdns.NewMdnsService(r.host, serviceName, r)
	// 显式启动 mDNS 服务，避免仅创建未启动导致无法广播/接收
	if err := r.svc.Start(); err != nil {
		if r.logger != nil {
			r.logger.Warnf("p2p.discovery.mdns start failed: %v", err)
		}
		return err
	}
	if r.logger != nil {
		r.logger.Infof("p2p.discovery.mdns started service=%s host_id=%s", serviceName, r.host.ID().String())
	}
	return nil
}

func (r *mdnsRuntime) Stop(ctx context.Context) error {
	if r.svc != nil {
		_ = r.svc.Close()
		r.svc = nil
	}
	return nil
}

// mdns.Notifee
func (r *mdnsRuntime) HandlePeerFound(info peer.AddrInfo) {
	if r.host == nil {
		return
	}
	if r.logger != nil {
		r.logger.Debugf("p2p.discovery.mdns peer_found id=%s addrs=%d", info.ID.String(), len(info.Addrs))
	}
	if r.onPeerFound != nil {
		r.onPeerFound()
	}
	if info.ID == r.host.ID() {
		if r.logger != nil {
			r.logger.Debugf("p2p.discovery.mdns ignore self id=%s", info.ID.String())
		}
		return
	}
	if r.host.Network().Connectedness(info.ID) == network.Connected {
		if r.logger != nil {
			r.logger.Debugf("p2p.discovery.mdns already_connected id=%s", info.ID.String())
		}
		return
	}
	// 拨号参数
	to := 10 * time.Second
	maxRetries := 2
	if r.cfg != nil {
		if r.cfg.Discovery.MDNS.ConnectTimeout > 0 {
			to = r.cfg.Discovery.MDNS.ConnectTimeout
		}
		if r.cfg.Discovery.MDNS.RetryLimit > 0 {
			maxRetries = r.cfg.Discovery.MDNS.RetryLimit
		}
	}
	if r.logger != nil {
		r.logger.Debugf("p2p.discovery.mdns dial_policy timeout=%s max_retries=%d", to, maxRetries)
	}
	for attempt := 0; attempt < maxRetries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), to)
		err := r.host.Connect(ctx, info)
		cancel()
		if err == nil {
			if r.logger != nil {
				r.logger.Infof("p2p.discovery.mdns connect_success id=%s attempt=%d", info.ID.String(), attempt+1)
			}
			if r.onConnOK != nil {
				r.onConnOK()
			}
			break
		}
		if r.logger != nil {
			r.logger.Warnf("p2p.discovery.mdns connect_failed id=%s attempt=%d error=%v", info.ID.String(), attempt+1, err)
		}
		if r.onConnFail != nil {
			r.onConnFail()
		}
	}
	if r.events != nil {
		// 事件处理器期望 (context.Context, interface{}) 参数
		r.events.Publish(eventiface.EventTypeNetworkPeerConnected, context.Background(), info.ID)
		if r.logger != nil {
			r.logger.Debugf("p2p.discovery.mdns event_published type=%s id=%s", eventiface.EventTypeNetworkPeerConnected, info.ID.String())
		}
	}
}
