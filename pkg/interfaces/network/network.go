package network

// Package network 定义 WES 系统网络服务层对外接口
//
// 🌐 **网络服务层 (Network Service Layer)**
//
// 本接口聚焦于网络消息的编解码与分发，专注于：
// - 协议注册与路由：基于协议 ID 注册流式 handler、订阅 handler
// - 两类消息范式：流式（请求-响应/长流）与订阅（发布-订阅）
// - 编解码与校验：长度前缀、压缩/签名/校验、版本协商（应用层）
// - 可靠性控制：超时、并发、背压、重试策略（应用层粒度）
//
// 🎯 **设计原则**
// - 不负责：Host 构建、NAT/Relay、连接/资源管理、发现（mDNS/DHT）、拨号策略
// - 不负责：Peer 路由/DHT 维护与引导调度
// - 不负责：任何业务语义（交易/区块等）与指标对外暴露
// - 不包含：启动/停止方法；生命周期由实现层自行管理
//
// 🔗 **与 P2P 的边界**
// - 仅依赖 p2p.Host 提供的 EnsureConnected/NewStream/RegisterStreamHandler
// - 不主动发现/拨号；发送前若需要连接，由 P2P 保障连通性
// - 不读取 P2P 配置；仅消费 Host 与可选的 EventBus

import (
	"context"
	"io"
	"time"

	"github.com/weisyn/v1/pkg/types"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

// Network 统一门面接口：聚合协议注册、流式发送与订阅发布能力
// - 不包含启动/停止方法；生命周期由实现层自行管理
// - 不暴露指标；仅面向消息编解码与分发
// - 业务协议由各领域模块自行注册，Network 不维护业务协议清单
type Network interface {
	// ==================== 协议注册（流式） ====================

	// RegisterStreamHandler 注册流式协议处理器
	// 参数：
	//   - protoID: 协议标识符（建议含版本），例如 "/weisyn/block/sync/v1"
	//   - handler: 流式消息处理器（应用层解包/处理/回包）
	//   - opts: 注册选项（可选：并发限制、背压、校验等）
	// 返回：
	//   - error: 注册失败时返回错误（如协议ID非法/重复注册）
	RegisterStreamHandler(protoID string, handler MessageHandler, opts ...RegisterOption) error

	// UnregisterStreamHandler 注销流式协议处理器
	// 参数：
	//   - protoID: 协议标识符
	// 返回：
	//   - error: 注销失败时返回错误（如协议不存在）
	UnregisterStreamHandler(protoID string) error

	// ==================== 订阅注册（PubSub） ====================

	// Subscribe 订阅指定主题
	// 参数：
	//   - topic: 主题名称（建议遵循命名规范），例如 "weisyn.block.announce.v1"
	//   - handler: 订阅消息处理器
	//   - opts: 订阅选项（可选：并发/背压、消息大小限制、签名校验等）
	// 返回：
	//   - unsubscribe: 取消订阅函数
	//   - error: 订阅失败时的错误
	Subscribe(topic string, handler SubscribeHandler, opts ...SubscribeOption) (unsubscribe func() error, err error)

	// ==================== 发送 API ====================

	// Call 流式请求-响应（点对点）
	// 参数：
	//   - ctx: 上下文（用于超时控制和取消）
	//   - to: 目标节点ID
	//   - protoID: 协议标识符（含版本）
	//   - req: 请求载荷（编码前）
	//   - opts: 传输选项（超时/重试/压缩等）
	// 返回：
	//   - []byte: 响应载荷（编码前）
	//   - error: 失败原因
	Call(ctx context.Context, to peer.ID, protoID string, req []byte, opts *types.TransportOptions) ([]byte, error)

	// OpenStream 打开长流（用于大体量数据传输等少量场景）
	// 参数：
	//   - ctx: 上下文（用于超时控制和取消）
	//   - to: 目标节点ID
	//   - protoID: 协议标识符（含版本）
	//   - opts: 传输选项
	// 返回：
	//   - StreamHandle: 流句柄（支持读写、半关闭、Reset等）
	//   - error: 失败原因
	OpenStream(ctx context.Context, to peer.ID, protoID string, opts *types.TransportOptions) (StreamHandle, error)

	// Publish 发布消息到指定主题（发布-订阅）
	// 参数：
	//   - ctx: 上下文（用于超时控制和取消）
	//   - topic: 主题名称（遵循命名规范）
	//   - data: 消息载荷
	//   - opts: 发布选项（压缩、签名等）
	// 返回：
	//   - error: 发布失败时的错误
	Publish(ctx context.Context, topic string, data []byte, opts *types.PublishOptions) error

	// ==================== 自检/诊断（非指标） ====================

	// ListProtocols 列出已注册的协议信息（用于诊断）
	// 返回：
	//   - []ProtocolInfo: 协议信息列表
	ListProtocols() []types.ProtocolInfo

	// GetProtocolInfo 获取指定协议的详细信息（用于诊断）
	// 参数：
	//   - protoID: 协议标识符
	// 返回：
	//   - *ProtocolInfo: 协议信息（不存在时返回 nil）
	GetProtocolInfo(protoID string) *types.ProtocolInfo

	// GetTopicPeers 获取指定主题连接的节点列表（用于诊断GossipSub mesh）
	// 参数：
	//   - topic: 主题名称
	// 返回：
	//   - []peer.ID: 连接到该主题的节点ID列表
	GetTopicPeers(topic string) []peer.ID

	// IsSubscribed 检查是否已订阅指定主题
	// 参数：
	//   - topic: 主题名称
	// 返回：
	//   - bool: 是否已订阅该主题
	IsSubscribed(topic string) bool

	// ==================== 协议能力检查 ====================

	// CheckProtocolSupport 检查对等节点是否支持指定协议
	// 参数：
	//   - ctx: 上下文（用于超时控制）
	//   - peerID: 对等节点ID
	//   - protocol: 协议标识符
	// 返回：
	//   - bool: 是否支持该协议
	//   - error: 检查失败时的错误
	CheckProtocolSupport(ctx context.Context, peerID peer.ID, protocol string) (bool, error)
}

// MessageHandler 流式消息处理器签名（应用层解包/处理/回包）
// 参数：
//   - ctx: 处理上下文（取消/超时）
//   - from: 发送方节点ID
//   - req: 请求载荷（需要应用层解码）
//
// 返回：
//   - resp: 响应载荷（需要应用层编码）
//   - error: 处理失败时的错误
type MessageHandler func(ctx context.Context, from peer.ID, req []byte) (resp []byte, err error)

// SubscribeHandler 订阅消息处理器签名
// 参数：
//   - ctx: 处理上下文（取消/超时）
//   - from: 发送方节点ID
//   - topic: 主题名称
//   - data: 消息数据
//
// 返回：
//   - error: 处理失败时的错误
type SubscribeHandler func(ctx context.Context, from peer.ID, topic string, data []byte) error

// StreamHandle 长流句柄（抽象，不暴露底层 libp2p 类型）
// 提供读写、半关闭、Reset、超时等流操作能力
type StreamHandle interface {
	io.Reader
	io.Writer
	Close() error
	CloseWrite() error
	Reset() error
	SetDeadline(t time.Time) error
}

// ==================== 选项与配置 ====================

// RegisterOption 协议注册选项（可选参数模式）
type RegisterOption func(*types.RegisterConfig)

// 兼容别名
type RegisterConfig = types.RegisterConfig

// SubscribeOption 订阅选项（可选参数模式）
type SubscribeOption func(*types.SubscribeConfig)

// 兼容别名
type SubscribeConfig = types.SubscribeConfig

// 兼容别名
type TransportOptions = types.TransportOptions

// 兼容别名
type PublishOptions = types.PublishOptions

// 兼容别名
type ProtocolInfo = types.ProtocolInfo
