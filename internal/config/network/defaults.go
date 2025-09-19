package network

import "time"

// 网络配置默认值
// 只包含实际使用的核心配置参数的默认值
const (
	// === 消息传输配置 ===

	// defaultMaxMessageSize 默认最大消息大小设为4MB
	// 原因：4MB足够传输大型区块或交易批次，同时避免内存压力
	defaultMaxMessageSize = 4 * 1024 * 1024

	// defaultMessageTimeout 默认消息超时时间设为30秒
	// 原因：30秒足够处理网络延迟和消息处理时间
	defaultMessageTimeout = 30 * time.Second

	// === 去重缓存配置 ===

	// defaultDeduplicationCacheTTL 默认去重缓存TTL设为10分钟
	// 原因：10分钟能覆盖消息在网络中传播的完整周期
	defaultDeduplicationCacheTTL = 10 * time.Minute

	// === 重试和容错配置 ===

	// defaultRetryAttempts 默认重试次数设为3
	// 原因：3次重试能应对大多数临时网络故障
	defaultRetryAttempts = 3

	// defaultRetryBackoffBase 默认重试退避基数设为1秒
	// 原因：1秒的基础退避时间适合网络通信的时间尺度
	defaultRetryBackoffBase = 1 * time.Second

	// defaultRetryBackoffMax 默认最大退避时间设为30秒
	// 原因：30秒的最大退避避免长时间等待
	defaultRetryBackoffMax = 30 * time.Second

	// === 流传输配置 ===

	// defaultConnectTimeout 默认连接超时时间设为10秒
	// 原因：10秒足够建立大多数P2P连接
	defaultConnectTimeout = 10 * time.Second

	// defaultWriteTimeout 默认写入超时时间设为5秒
	// 原因：5秒足够写入大多数消息
	defaultWriteTimeout = 5 * time.Second

	// defaultReadTimeout 默认读取超时时间设为10秒
	// 原因：10秒足够读取大多数响应消息
	defaultReadTimeout = 10 * time.Second
)
