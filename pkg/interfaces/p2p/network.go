package p2p

import (
	"context"
	"time"

	libpeer "github.com/libp2p/go-libp2p/core/peer"
)

// StreamHandler 流处理器函数类型
type StreamHandler func(ctx context.Context, s Stream)

// Stream 流接口
type Stream interface {
	ID() string
	Peer() libpeer.ID
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Close() error
}

// NetworkService 网络服务抽象层
//
// 提供与旧 node.NodeService 等价/近似的接口，但底层使用 p2p.Service
// 用于平滑迁移，让上层代码可以逐步从 node 切换到 p2p
type NetworkService interface {
	// EnsureConnected 确保与目标节点连通（幂等）
	EnsureConnected(ctx context.Context, peerID libpeer.ID, deadline time.Time) error

	// NewStream 打开出站流
	NewStream(ctx context.Context, to libpeer.ID, protocolID string) (Stream, error)

	// RegisterHandler 注册入站协议处理器
	RegisterHandler(protocolID string, handler StreamHandler)

	// UnregisterHandler 注销协议处理器
	UnregisterHandler(protocolID string)
}

