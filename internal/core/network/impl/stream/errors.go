package stream

import "errors"

// errors.go
// 传输层错误模型与错误码归纳（方法框架）：
// - 采用哨兵错误用于上层分支处理
// - 细节错误由实现层包装

var (
	ErrNoRoute          = errors.New("no route")          // 路由不可达
	ErrRateLimited      = errors.New("rate limited")      // 被限流
	ErrBackpressureFull = errors.New("backpressure full") // 背压饱和
	ErrTimeout          = errors.New("timeout")           // 超时
	ErrCodec            = errors.New("codec error")       // 编解码错误
	ErrConnection       = errors.New("connection error")  // 连接错误

	// ErrRetryable 标记可重试错误的占位，实际实现可用 wrap 增强上下文
	ErrRetryable = errors.New("retryable")
)

// IsRetryable 判断错误是否可重试（方法框架）
// 说明：
// - 实现阶段可通过 errors.Is(err, ErrRetryable) 或错误分类映射来决定
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	var ce *CodecError
	if errors.As(err, &ce) {
		return ce.IsRetryable()
	}
	return errors.Is(err, ErrRetryable)
}
