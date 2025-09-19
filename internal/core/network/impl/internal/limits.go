package internal

// limits.go
// 大小/并发/阈值常量集中定义（实施阶段补充）

// 传输相关默认阈值（方法框架）
const (
	DefaultMaxMessageSize       = 4 * 1024 * 1024 // 4MB
	DefaultMaxConcurrentIO      = 128
	DefaultCompressionThreshold = 1 * 1024 // 1KB
)
