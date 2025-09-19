package types

import "time"

// Eventing 与网络层使用的通用数据结构（从 interfaces 迁移）

// ===== 归并自 network_service.go =====

// RegisterConfig 协议注册配置
type RegisterConfig struct {
	MaxConcurrency     int  // 最大并发处理数
	EnableBackpressure bool // 是否启用背压
}

// SubscribeConfig 订阅配置
type SubscribeConfig struct {
	MaxConcurrency              int  // 最大并发处理数
	EnableBackpressure          bool // 是否启用背压
	MaxMessageSize              int  // 最大消息大小
	EnableSignatureVerification bool // 是否启用签名验证
	EnableRateLimit             bool // 是否启用速率限制
}

// TransportOptions 传输选项
type TransportOptions struct {
	// 超时配置
	ConnectTimeout time.Duration // 连接超时
	WriteTimeout   time.Duration // 写入超时
	ReadTimeout    time.Duration // 读取超时

	// 重试配置
	MaxRetries    int           // 最大重试次数
	RetryDelay    time.Duration // 重试延迟
	BackoffFactor float64       // 退避因子

	// 传输配置
	MaxConcurrency       int  // 最大并发数
	EnableCompression    bool // 是否启用压缩
	EnableEncryption     bool // 是否启用传输层加密
	EnableIntegrityCheck bool // 是否启用完整性校验
}

// PublishOptions 发布选项
type PublishOptions struct {
	Topic              string        // 指定发布主题
	Timeout            time.Duration // 发送超时
	RetryCount         int           // 重试次数
	RequireAck         bool          // 是否要求确认
	Priority           int           // 发布优先级
	BatchSize          int           // 批量大小
	DelayBetweenSend   time.Duration // 发送间隔
	MaxMessageSize     int           // 最大消息大小（兼容使用）
	SignatureEnabled   bool          // 是否启用签名（兼容使用）
	CompressionEnabled bool          // 是否启用压缩（兼容使用）
}

// ProtocolInfo 协议信息
type ProtocolInfo struct {
	Name              string            // 协议名称
	Version           string            // 协议版本
	SupportedFeatures []string          // 支持的特性列表
	ID                string            // 协议ID（兼容使用）
	RegisteredAt      time.Time         // 注册时间（兼容使用）
	Metadata          map[string]string // 元数据（兼容使用）
}
