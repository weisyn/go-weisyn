// Package api provides default configuration values for API services.
package api

import "time"

// API服务默认配置值
const (
	// === HTTP API配置 ===

	// defaultHTTPEnabled 默认启用HTTP服务
	// 原因：HTTP 服务是 API 网关核心，承载 REST/JSON-RPC/WebSocket
	defaultHTTPEnabled = true

	// 协议细粒度开关默认值（v0.0.2+）
	// 原因：默认全开，开销低（空闲时 HTTP 复用一个监听）
	defaultHTTPEnableREST      = true // REST 端点（运维、调试）
	defaultHTTPEnableJSONRPC   = true // JSON-RPC（主协议，不应关闭）
	defaultHTTPEnableWebSocket = true // WebSocket（实时订阅）

	// defaultHTTPHost HTTP监听地址设为0.0.0.0
	// 原因：监听所有网络接口，允许来自任何IP的连接
	defaultHTTPHost = "0.0.0.0"

	// defaultHTTPPort HTTP端口设为28680（WES 端口规范）
	// 原因：避开常用端口冲突，并与全生态端口段保持一致（见 docs/PORTS_SPEC.md）
	defaultHTTPPort = 28680

	// defaultHTTPTimeout HTTP超时时间设为30秒
	// 原因：给复杂查询足够的处理时间，同时避免长时间占用连接
	defaultHTTPTimeout = 30 * time.Second

	// defaultHTTPReadTimeout HTTP读取超时设为15秒
	// 原因：防止慢客户端占用连接，确保服务器响应性
	defaultHTTPReadTimeout = 15 * time.Second

	// defaultHTTPWriteTimeout HTTP写入超时设为15秒
	// 原因：防止慢客户端影响响应写入，保证服务器性能
	defaultHTTPWriteTimeout = 15 * time.Second

	// defaultMaxRequestSize 最大请求大小设为4MB
	// 原因：允许较大的JSON请求，如批量操作，同时防止内存溢出
	defaultMaxRequestSize = 4 * 1024 * 1024

	// defaultCORSEnabled 默认启用CORS
	// 原因：现代Web应用经常需要跨域访问API
	defaultCORSEnabled = true

	// defaultRateLimitRPM 默认限流每分钟600请求
	// 原因：防止API滥用，保护服务器资源
	defaultRateLimitRPM = 600

	// === gRPC API配置 ===

	// defaultGRPCEnabled 默认启用gRPC
	// 原因：gRPC提供高性能的二进制协议，适合系统间通信
	defaultGRPCEnabled = true

	// defaultGRPCHost gRPC监听地址设为0.0.0.0
	// 原因：同HTTP，允许外部系统连接
	defaultGRPCHost = "0.0.0.0"

	// defaultGRPCPort gRPC端口设为28682（WES 端口规范）
	// 原因：避开常用端口冲突，并与 WES Node 端口段一致（见 docs/PORTS_SPEC.md）
	defaultGRPCPort = 28682

	// defaultGRPCMaxMessageSize gRPC最大消息大小设为4MB
	// 原因：支持大型交易批次和区块数据传输
	defaultGRPCMaxMessageSize = 4 * 1024 * 1024

	// defaultGRPCKeepaliveTime gRPC连接保活时间设为30秒
	// 原因：及时检测连接状态，避免死连接占用资源
	defaultGRPCKeepaliveTime = 30 * time.Second

	// defaultGRPCKeepaliveTimeout gRPC保活超时设为5秒
	// 原因：快速检测连接失效，及时清理资源
	defaultGRPCKeepaliveTimeout = 5 * time.Second

	// === WebSocket配置 ===

	// defaultWebSocketEnabled 默认启用WebSocket
	// 原因：为前端应用提供实时数据推送能力
	defaultWebSocketEnabled = true

	// defaultWebSocketHost WebSocket监听地址设为0.0.0.0
	// 原因：允许Web应用从任何域连接
	defaultWebSocketHost = "0.0.0.0"

	// defaultWebSocketPort WebSocket端口设为28681（WES 端口规范）
	// 原因：与 HTTP 端口分离，同时避免与常用端口冲突（见 docs/PORTS_SPEC.md）
	defaultWebSocketPort = 28681

	// defaultWebSocketMaxConnections WebSocket最大连接数设为100
	// 原因：限制并发连接，防止资源耗尽
	defaultWebSocketMaxConnections = 100

	// defaultWebSocketReadBufferSize WebSocket读缓冲区设为1024字节
	// 原因：足以处理常见的实时消息，节省内存
	defaultWebSocketReadBufferSize = 1024

	// defaultWebSocketWriteBufferSize WebSocket写缓冲区设为1024字节
	// 原因：与读缓冲区保持一致，优化内存使用
	defaultWebSocketWriteBufferSize = 1024
)

// defaultCORSOrigins 默认CORS允许源列表
// 开发环境允许所有源，生产环境应限制为特定域名
var defaultCORSOrigins = []string{
	"*", // 允许所有源，生产环境建议替换为具体域名
}
