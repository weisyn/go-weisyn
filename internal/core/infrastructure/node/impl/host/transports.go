package host

import (
	libp2p "github.com/libp2p/go-libp2p"
	libp2pquic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/libp2p/go-libp2p/p2p/transport/websocket"

	nodeconfig "github.com/weisyn/v1/internal/config/node"
)

// 传输层：
// - 默认使用 libp2p 稳定组合（DefaultTransports）；
// - 显式启用 TCP/QUIC/WebSocket 时，按需追加，并尽量保留指标/可观测性；
// - 部分底层细节（如 TCPKeepAlive/QUIC stream 限制）上游构造器未暴露，保持默认以确保稳定性。

// withTransportOptions 根据配置构建传输层选项
func withTransportOptions(cfg *nodeconfig.NodeOptions) []libp2p.Option {
	if cfg == nil {
		return []libp2p.Option{libp2p.DefaultTransports}
	}
	tr := cfg.Host.Transport
	var opts []libp2p.Option

	// TCP
	if tr.EnableTCP {
		opts = append(opts, libp2p.Transport(tcp.NewTCPTransport, tcp.WithMetrics()))
	}
	// QUIC
	if tr.EnableQUIC {
		opts = append(opts, libp2p.Transport(libp2pquic.NewTransport))
	}
	// WebSocket
	if tr.EnableWebSocket {
		opts = append(opts, libp2p.Transport(websocket.New))
	}

	if len(opts) == 0 {
		return []libp2p.Option{libp2p.DefaultTransports}
	}
	return opts
}
