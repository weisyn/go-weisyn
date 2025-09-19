package node

// Package node 定义 Network 层所需的最小 节点网络 公共接口
// 设计目标：
// - 高内聚低耦合：仅暴露 Network 必需的能力（连通性保障、开流、入站流分派）
// - 无生命周期方法：不暴露 Start/Stop/IsReady 等，生命周期由实现内部管理
// - 无指标接口：不暴露监控/统计/质量评分等（与项目接口规范一致）
// - 稳定适配层：对 libp2p 等底层实现做最薄适配，避免实现细节泄漏

import (
	"context"
	"io"
	"time"

	libhost "github.com/libp2p/go-libp2p/core/host"
	peer "github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

// RawStream 最小流抽象（对底层 libp2p stream 的收敛）
// 说明：
// - 仅包含 Network 需要的读/写/半关闭/复位/截止时间设置能力
// - 不暴露实现细节（如多路复用器、窗口大小等）
type RawStream interface {
	io.Reader
	io.Writer
	Close() error
	CloseWrite() error
	Reset() error
	SetDeadline(t time.Time) error
}

// StreamHandler 入站流处理器签名（由 Network 的 dispatcher/registry 持有并注册）
// 参数：
//   - ctx: 处理上下文（取消/超时）
//   - remote: 对端 PeerID
//   - s: 入站 RawStream
type StreamHandler func(ctx context.Context, remote peer.ID, s RawStream)

// Host 面向 Network 的最小 节点网络 宿主机接口
// 仅提供三类能力：确保连通、开流、入站流注册；另提供可选观测方法
type Host interface {
	// EnsureConnected 确保与目标节点的连通性（幂等）
	// 说明：
	// - 由 节点网络 实现内部执行发现/拨号/策略/限流等；Network 不参与
	// - 应区分错误类型：超时/拒绝/背压/暂时性失败
	EnsureConnected(ctx context.Context, to peer.ID, deadline time.Time) error

	// NewStream 打开出站流
	// 说明：
	// - 协议ID由 Network 决定（含版本），节点网络 仅负责通道
	// - 要求支持半关闭（CloseWrite）与 Reset
	NewStream(ctx context.Context, to peer.ID, protocolID string) (RawStream, error)

	// RegisterStreamHandler 为给定协议注册入站处理器
	// 说明：
	// - 线程安全，可热更新；与内部协议命名应隔离避免冲突
	RegisterStreamHandler(protocolID string, h StreamHandler)

	// UnregisterStreamHandler 取消协议入站处理器
	UnregisterStreamHandler(protocolID string)

	// ===== 可选观测能力（非发送接收所必需） =====

	// ID 返回本地 PeerID（用于日志与追踪）
	ID() peer.ID

	// AnnounceAddrs 返回对外可达地址（已过 NAT/Relay 策略与过滤），用于诊断
	AnnounceAddrs() []ma.Multiaddr

	// Libp2pHost 返回底层 libp2p Host（仅供 Network 的 PubSub 适配使用）
	Libp2pHost() libhost.Host

	// RegisterPendingHandlers 注册延迟的协议处理器（内部使用）
	// 🔧 在P2P Host启动完成后调用，处理启动时无法注册的协议
	RegisterPendingHandlers()

	// ValidateWESPeer 验证节点是否为WES业务节点
	// 参数：
	//   - ctx: 上下文
	//   - peerID: 待验证的节点ID
	// 返回：
	//   - bool: 是否为WES节点
	//   - error: 验证过程中的错误
	// 说明：
	//   - 用于K桶过滤，只允许WES节点进入路由表
	//   - 基于协议能力检查实现简单的节点分类
	ValidateWESPeer(ctx context.Context, peerID peer.ID) (bool, error)
}
