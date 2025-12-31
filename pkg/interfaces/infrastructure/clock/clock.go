// Package clock provides clock synchronization interfaces.
package clock

import "time"

// Clock 提供统一的时间源接口（基础设施层接口）
//
// 设计目标：
// - 确定性：在同一执行上下文中，保证时间可控且一致
// - 可测试：支持可替换与Mock实现
// - 可扩展：可切换为NTP/Roughtime等时间源
type Clock interface {
	// Now 获取当前时间
	Now() time.Time

	// Since 计算从指定时间到现在的持续时间
	Since(t time.Time) time.Duration

	// Unix 获取当前Unix时间戳（秒）
	Unix() int64

	// UnixNano 获取当前Unix时间戳（纳秒）
	UnixNano() int64
}
