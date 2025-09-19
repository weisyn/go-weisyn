package stream

// client.go
// 能力：
// - Call/OpenStream：面向协议ID的点对点请求-响应与长流创建
// - 半关闭/超时：支持在写完后关闭写端、读端超时控制
// - 选项：解析 TransportOptions（超时/重试/压缩阈值/优先级）
// 依赖/公共接口：
// - p2p.Host：EnsureConnected + NewStream（打开 libp2p 流）
// - pkg/interfaces/infrastructure/log.Logger：结构化日志
// - pkg/interfaces/infrastructure/event.EventBus：必要时上报错误事件（内部）
// - impl/stream/codec.go：长度前缀帧/压缩协商
// 目的：
// - 将“打开流/收发/关闭”的细节与业务解耦，统一错误模型与重试入口
// 非目标：
// - 不做发现/拨号（交由 P2P 层）
// - 不做路由选择（交由 router 模块）

// Call 契约（方法框架）：
// - 写入请求帧后半关闭写端（若协议要求），随后读取响应帧
// - 超时分层：连接/写入/读取分别受控于 TransportOptions
// - 重试边界：仅在幂等请求且错误可重试时触发；默认不重试
// - 错误模型：超时/编解码/连接错误区分，必要时包装为 ErrRetryable
