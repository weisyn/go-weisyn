package service

import (
	"context"
	"fmt"
	"time"

	lphost "github.com/libp2p/go-libp2p/core/host"
	libnetwork "github.com/libp2p/go-libp2p/core/network"
	libpeer "github.com/libp2p/go-libp2p/core/peer"
	libprotocol "github.com/libp2p/go-libp2p/core/protocol"

	p2pi "github.com/weisyn/v1/pkg/interfaces/p2p"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// streamAdapter 将 libp2p Stream 适配为 p2p.Stream
type streamAdapter struct {
	stream libnetwork.Stream
}

func (a *streamAdapter) ID() string {
	return fmt.Sprintf("%s", a.stream.ID())
}

func (a *streamAdapter) Peer() libpeer.ID {
	return a.stream.Conn().RemotePeer()
}

func (a *streamAdapter) Read(p []byte) (int, error) {
	return a.stream.Read(p)
}

func (a *streamAdapter) Write(p []byte) (int, error) {
	return a.stream.Write(p)
}

func (a *streamAdapter) Close() error {
	return a.stream.Close()
}

// networkService 实现 p2p.NetworkService 接口
type networkService struct {
	host   lphost.Host
	logger logiface.Logger
}

// NewNetworkService 创建网络服务
func NewNetworkService(host lphost.Host, logger logiface.Logger) p2pi.NetworkService {
	return &networkService{
		host:   host,
		logger: logger,
	}
}

// EnsureConnected 确保与目标节点连通（幂等）
func (n *networkService) EnsureConnected(ctx context.Context, peerID libpeer.ID, deadline time.Time) error {
	if n.host == nil {
		return fmt.Errorf("host not available")
	}

	network := n.host.Network()
	if network == nil {
		return fmt.Errorf("network not available")
	}

	// 已连接则直接返回
	if network.Connectedness(peerID) == libnetwork.Connected {
		return nil
	}

	// 尝试拨号
	// 先尝试从 peerstore 获取地址信息
	addrs := n.host.Peerstore().Addrs(peerID)
	if len(addrs) == 0 {
		// 如果没有地址，尝试通过 Routing 查找
		// 这里暂时返回错误，上层可以通过 p2p.Service.Routing() 查找
		return fmt.Errorf("no addresses for peer %s", peerID)
	}

	peerInfo := libpeer.AddrInfo{
		ID:    peerID,
		Addrs: addrs,
	}

	// 使用 deadline 设置超时
	if !deadline.IsZero() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
	}

	return n.host.Connect(ctx, peerInfo)
}

// NewStream 打开出站流
func (n *networkService) NewStream(ctx context.Context, to libpeer.ID, protocolID string) (p2pi.Stream, error) {
	if n.host == nil {
		return nil, fmt.Errorf("host not available")
	}

	stream, err := n.host.NewStream(ctx, to, libprotocol.ID(protocolID))
	if err != nil {
		return nil, err
	}

	return &streamAdapter{stream: stream}, nil
}

// RegisterHandler 注册入站协议处理器
func (n *networkService) RegisterHandler(protocolID string, handler p2pi.StreamHandler) {
	if n.host == nil {
		if n.logger != nil {
			n.logger.Warnf("p2p.network.register_handler failed: host not available")
		}
		return
	}

	if n.logger != nil {
		n.logger.Debugf("p2p.network.register_handler protocol=%s", protocolID)
	}

	// 注册协议处理器
	n.host.SetStreamHandler(libprotocol.ID(protocolID), func(s libnetwork.Stream) {
		if n.logger != nil {
			n.logger.Debugf("p2p.network.stream_received protocol=%s peer=%s", protocolID, s.Conn().RemotePeer())
		}
		// 使用无派生的上下文；上层可在 handler 内部再行管理超时/取消
		handler(context.Background(), &streamAdapter{stream: s})
	})
}

// UnregisterHandler 注销协议处理器
func (n *networkService) UnregisterHandler(protocolID string) {
	if n.host == nil {
		return
	}

	if n.logger != nil {
		n.logger.Debugf("p2p.network.unregister_handler protocol=%s", protocolID)
	}

	n.host.RemoveStreamHandler(libprotocol.ID(protocolID))
}

