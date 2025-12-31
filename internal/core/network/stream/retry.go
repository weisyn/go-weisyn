// Package stream provides retry and backoff strategies for network streams.
package stream

import "time"

// retry.go
// 重试/退避策略定义（方法框架）：
// - 提供指数退避与抖动参数
// - 由 client 调用以决定下一次重试延迟

// RetryPolicy 重试策略（方法框架）
type RetryPolicy struct {
	MaxRetries    int           // 最大重试次数
	BaseDelay     time.Duration // 基础延迟
	BackoffFactor float64       // 退避因子
	JitterRatio   float64       // 抖动比例（0-1）
}

// NextDelay 计算第 n 次重试的延迟
// 参数：尝试次数（从1开始）
// 返回：建议延迟
func (p *RetryPolicy) NextDelay(_attempt int) time.Duration { return 0 }
