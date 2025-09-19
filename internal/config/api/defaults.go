package api

import "time"

// API服务默认配置值
// 这些默认值基于生产环境的最佳实践和常见API服务配置
const (
	// === HTTP API配置 ===

	// defaultHTTPEnabled 默认启用HTTP API
	// 原因：HTTP API是最常用的接口，为用户提供RESTful访问方式
	// 大多数应用都需要HTTP接口进行交互
	defaultHTTPEnabled = true

	// defaultHTTPHost HTTP监听地址设为0.0.0.0
	// 原因：监听所有网络接口，允许来自任何IP的连接
	// 生产环境中通常需要外部访问，使用0.0.0.0最为灵活
	defaultHTTPHost = "0.0.0.0"

	// defaultHTTPPort HTTP端口设为8080
	// 原因：8080是常用的HTTP替代端口，避免与系统端口冲突
	// 不需要root权限，便于开发和部署
	defaultHTTPPort = 8080

	// defaultHTTPTimeout HTTP超时时间设为30秒
	// 原因：给复杂查询足够的处理时间，同时避免长时间占用连接
	// 30秒平衡了用户体验和系统资源
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
	// 启用CORS提供更好的前端支持
	defaultCORSEnabled = true

	// defaultRateLimitRPM 默认限流每分钟600请求
	// 原因：防止API滥用，保护服务器资源
	// 600请求/分钟 = 10请求/秒，对正常使用足够，同时限制恶意访问
	defaultRateLimitRPM = 600

	// === gRPC API配置 ===

	// defaultGRPCEnabled 默认启用gRPC
	// 原因：gRPC提供高性能的二进制协议，适合系统间通信
	// 对于区块链节点间的高频通信很重要
	defaultGRPCEnabled = true

	// defaultGRPCHost gRPC监听地址设为0.0.0.0
	// 原因：同HTTP，允许外部系统连接
	defaultGRPCHost = "0.0.0.0"

	// defaultGRPCPort gRPC端口设为9090
	// 原因：9090是gRPC服务的常用端口
	// 与HTTP端口分离，避免协议冲突
	defaultGRPCPort = 9090

	// defaultGRPCMaxMessageSize gRPC最大消息大小设为4MB
	// 原因：支持大型交易批次和区块数据传输
	// 与HTTP请求大小保持一致
	defaultGRPCMaxMessageSize = 4 * 1024 * 1024

	// defaultGRPCKeepaliveTime gRPC连接保活时间设为30秒
	// 原因：及时检测连接状态，避免死连接占用资源
	// 30秒适合大多数网络环境
	defaultGRPCKeepaliveTime = 30 * time.Second

	// defaultGRPCKeepaliveTimeout gRPC保活超时设为5秒
	// 原因：快速检测连接失效，及时清理资源
	defaultGRPCKeepaliveTimeout = 5 * time.Second

	// === WebSocket配置 ===

	// defaultWebSocketEnabled 默认启用WebSocket
	// 原因：为前端应用提供实时数据推送能力
	// 区块链应用经常需要实时更新
	defaultWebSocketEnabled = true

	// defaultWebSocketHost WebSocket监听地址设为0.0.0.0
	// 原因：允许Web应用从任何域连接
	defaultWebSocketHost = "0.0.0.0"

	// defaultWebSocketPort WebSocket端口设为8081
	// 原因：与HTTP端口分离，避免协议混淆
	// 8081是常用的WebSocket端口
	defaultWebSocketPort = 8081

	// defaultWebSocketMaxConnections WebSocket最大连接数设为100
	// 原因：限制并发连接，防止资源耗尽
	// 100个连接足以支持中等规模的实时应用
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
